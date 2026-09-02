# Fix Proposal: bf-65lsdu OOM Kill During Git GC (Repository Bloat)

**Proposal Date:** 2026-09-02
**Proposing Bead:** domchk-81e02aff (split-child, "Propose fix or mitigation for crash cause")
**Root Cause Source:** domchk-1b9940a3 (CLOSED — "Analyze root cause of agent signal -1 crash")
**Crash Bead:** bf-65lsdu
**Classification:** INFRASTRUCTURE EVENT — Repository Bloat OOM (Pattern 3, `docs/crash-response-guide.md`)
**Scope:** Mitigation proposal only. No domain-check application code changes are proposed or needed.

---

## 1. Root Cause Recap (from domchk-1b9940a3)

On 2026-08-13, agent processes performing `git gc` operations on this workspace
were killed with SIGKILL (exit code -1).

**Causal chain:**

1. Repository had silently grown to **~18GB** (36x the <500MB healthy target),
   with **17.20 GiB of loose objects** (4,515 objects) — 99% of the repository.
   Bloat source: automated beads committed 17+ identical ~237MB `.beads/*.jsonl`
   files before `.beads/` was gitignored.
2. `git gc --aggressive` on that object set requires several GB of RAM
   (window memory + delta cache + pack bookkeeping).
3. **Amplification:** multiple git gc operations ran **concurrently**, each
   consuming >4GB, on a 62GB box that also hosted the fleet.
4. The Linux OOM killer delivered SIGKILL to the gc processes → agent crashes:
   7 events on 2026-08-13 (21:30–23:56), 2 more on 2026-08-14 (00:14–00:20).
5. Resolution succeeded 2026-08-17 only after the task was **split into 3
   sequential child beads** (domchk-bdb1fedf, domchk-af4b5ef4, domchk-87be56d8)
   — task decomposition reduced per-run memory and complexity.

**Reproducibility:** not reproducible in the current state. Verified live on
2026-09-02:

```
$ git count-objects -vH
count: 0            # loose objects: 4,515 → 0
in-pack: 10349
packs: 1
size-pack: 90.14 MiB
garbage: 0

$ du -sh .git
92M                 # was ~18GB
```

The proposal below therefore has two parts: (a) confirmation that the
mitigation layers already implemented for this exact root cause are live and
correct, and (b) the specific gaps found during live verification, with
recommended fixes, implementation approach, and trade-offs.

---

## 2. Mitigation Strategy — Defense in Depth

Each layer below maps to a specific link in the causal chain. Type is the
implementation approach per the acceptance criteria (config change / code
change / process change). Status was verified by running the cited commands on
2026-09-02, not taken from documentation claims.

### L1 — Prevent bloat formation → `config`

| Control | Evidence (verified 2026-09-02) |
|---|---|
| `.gitignore` excludes `.beads/` | `.gitignore:66` = `.beads/` (covers the 237MB JSONL bloat source) |
| Pre-commit hook blocks files >10MB | `.git/hooks/pre-commit` installed 2026-09-01; source `scripts/pre-commit-repo-size-hook` |

**Trade-offs:** the hook is client-side and bypassable (`git commit -n`), and a
legitimate >10MB fixture requires an explicit override. Accepted: the threat
model here is *accidental* bulk data (bead stores, logs), not adversarial
commits, and CI-side `check-repo-health.sh` provides the second net.

### L2 — Bound per-operation memory → `config` (git) + `config` (systemd)

Even if bloat recurs, no single gc may be able to starve the box.

**Repo git config (verified via `git config -l --local`):**

```
pack.windowmemory=2g     # repack window memory — the dominant gc consumer
pack.deltacachesize=1g
pack.threads=1           # no per-thread memory multiplication
pack.window=5
pack.depth=20
```

**safe-git-gc.sh** (`scripts/safe-git-gc.sh`, committed at 4737327 plus
in-flight edits): runs gc inside a hard **systemd cgroup ceiling**
(`systemd-run --user --scope`, `MemoryMax` = `SAFE_GC_MEMORY_MAX`, default
2g), so a runaway gc is OOM-killed *inside its own cgroup* instead of
exhausting the system. The weekly full-gc service unit additionally sets
`MemoryMax=4G`, `MemorySwapMax=0`, `CPUQuota=300%`, `Nice=15`,
`IOSchedulingClass=idle`, `OOMScoreAdjust=500` (script:
`scripts/domain-check-git-gc-full.service`).

**Trade-offs:** single-threaded, small-window repacking is significantly
slower — the documented full-gc cost is ~1–2h vs minutes for uncapped
`--aggressive`. Accepted deliberately: wall-clock time is cheap; an OOM that
kills 7 agents in 2.5 hours is not. `OOMScoreAdjust=500` makes the gc itself
the preferred OOM victim, which is the correct failure direction.

### L3 — Serialize gc box-wide → `code`

The bf-65lsdu amplification factor was **concurrency**: several >4GB gc
processes at once. `safe-git-gc.sh` takes a box-wide lockfile
(`/tmp/domain-check-safe-git-gc.lock`, `SAFE_GC_LOCK_WAIT=1800s`) so at most
one gc runs on this box at any time.

**Trade-offs:** gc requests queue or skip; a skipped gc is fine because the
nightly timer (L5) is the actual guarantee, and ad-hoc runs are opportunistic.

### L4 — Detect bloat early → `process` + `config`

| Monitor | Cadence | Threshold | Verified state (2026-09-02) |
|---|---|---|---|
| `domain-check-repo-health.timer` | daily 02:00 | repo >1GB, loose >500MB | active, next fire 2026-09-03 02:00 (first fire pending — see GAP-1b) |
| `domain-check-resource-monitor.timer` | 5 min | mem <5GB, disk <15GB, load >10 | active |
| `domain-check-service-monitor.timer` | 2 min | gateway 503/502 | active |
| `domain-check-monitoring.timer` (crash pattern) | 10 min | 10+ crashes / 10 min | active |
| `scripts/preflight-health-check.sh` | per-task | memory/disk/repo gates | available for agent pre-task use |

**Trade-offs:** monitoring is cheap relative to the investigation cost it
replaces (the bloat period cost 100+ agent-hours of duplicate crash
investigations).

### L5 — Scheduled maintenance → `process`

| Operation | Schedule | Verified state (2026-09-02) |
|---|---|---|
| Standard gc (stages 1–2) | daily 03:00 | ✅ **fired today**, `LastTrigger=2026-09-02 03:00:10 EDT`, `Result=success` |
| Full gc (`safe-git-gc.sh --full`) | Sunday 04:00 | ⚠️ **enabled but never started — GAP-1 below** |

### L6 — Respond safely on recurrence → `process`

- `safe-git-gc.sh` stages its work with **checkpoint/resume**
  (`.git/safe-gc-checkpoint.json`, `--resume`), so an interrupted run resumes
  instead of restarting — the failure mode that produced repeated OOM attempts
  on 2026-08-13.
- **Task decomposition playbook:** bf-65lsdu itself was only resolved after the
  mega-task was split into 3 sequential child beads. Rule of thumb: any
  single-bead maintenance task expected to exceed ~30 minutes of gc/compression
  should be split (see `scripts/bead-split-recommender.sh`).
- Crash-response runbook: Pattern 3 in `docs/crash-response-guide.md`;
  `scripts/crash-classifier.sh` / `crash-alert-manager.sh` (6 fixes, incl.
  closed-bead filtering and dedup) prevent the false-positive investigation
  storm that accompanied earlier incidents.

---

## 3. Gaps Found During Live Verification (the actual proposals)

Verification found the layered defense **substantially implemented and
operational**. Three gaps remain; none require domain-check code changes.

### GAP-1 (P1, process) — Weekly full-gc timer is enabled but never started

**Evidence (2026-09-02):**

```
$ systemctl --user is-enabled domain-check-git-gc-full.timer
enabled
$ systemctl --user show domain-check-git-gc-full.timer \
    -p ActiveState,NextElapseUSecMonotonic,LastTriggerUSec
ActiveState=inactive
NextElapseUSecMonotonic=infinity     # will never fire in this session
LastTriggerUSec=                     # has never fired
```

`enable` only wires the timer into future session boots; the current user
session never *started* it. The unit definition itself is correct
(`OnCalendar=Sun *-*-* 04:00:00`, `Persistent=true`, `WantedBy=timers.target`)
and matches the installed copy. Documentation drift: CLAUDE.md states weekly
full gc is "✅ Scheduled" — it is scheduled for *boot*, not running now.

**Recommended fix (1 command, reversible with `systemctl --user stop`):**

```bash
systemctl --user daemon-reload
systemctl --user start domain-check-git-gc-full.timer
```

**Caveat — do not run it yet.** Because `Persistent=true` and `LastTriggerUSec`
is empty, starting the timer fires an immediate catch-up full gc. Another
worker currently has **uncommitted modifications to `scripts/safe-git-gc.sh`
and the gc service units** (verified in `git status` on 2026-09-02); a catch-up
run now would execute that in-flight script version. Sequence the fix:

1. Land/commit the in-flight safe-git-gc edits,
2. `systemctl --user daemon-reload && systemctl --user start domain-check-git-gc-full.timer`,
3. Confirm a successful catch-up run in `.beads/logs/git-gc-full.log`,
4. Update CLAUDE.md's "Scheduled" claim to reflect verified live state.

**Trade-offs:** the catch-up run costs one full gc (~1–2h, memory-capped at 4G,
niced) — acceptable one-time cost; deferring costs nothing since the repo is
at 92MB and the daily standard gc covers interim packing.

### GAP-1b (P3, process) — repo-health timer has never fired yet

`domain-check-repo-health.timer` is active with `NextElapse=2026-09-03 02:00`
but `LastTriggerUSec` is empty — it was installed/enabled recently and its
first daily run is still pending. **Action:** check
`.beads/logs/repo-health.log` after 2026-09-03 02:00 and confirm a first
successful run; if it did not fire, the `domain-check-repo-health.service`
unit (not the mitigation design) needs fixing. No action required before then.

### GAP-2 (P2, config) — `git gc --auto` bypasses the box-wide gc lock

The L3 lock only guards `safe-git-gc.sh` invocations. Routine git commands
(commit, merge, pull) trigger **auto-gc**, and this repo sets `gc.auto=100` —
~67x more aggressive than git's default (6700) — with `gc.autoDetach` at its
default (`true`), so multiple concurrent fleet workers can each spawn a
background auto-gc. Each is memory-bounded by `pack.windowmemory=2g` +
`pack.deltacachesize=1g` (~4GB worst case each), but they are **unserialized**:
the exact amplification mechanism that turned bf-65lsdu from one OOM into seven.

**Recommended fix (1 command):**

```bash
git config gc.auto 0    # rely on the nightly safe-git-gc timer instead
```

At 92MB and one pack, nightly scheduled gc fully covers packing needs;
loose objects accumulating for up to 24h is far below the 500MB warning
threshold, and `preflight-health-check.sh` gates on loose-object size anyway.

**Trade-offs:** losing opportunistic background packing means (a) loose objects
persist up to a day (cosmetic; well within thresholds) and (b) if a bloat event
*does* recur, auto-gc will not be silently fighting it either — which is
desirable, because the correct response to bloat is the serialized, capped,
resumable `safe-git-gc.sh`, not an unserialized background gc. If the team
prefers keeping auto-gc, the fallback is raising `gc.auto` to the git default
6700 and accepting a bounded-but-unserialized path; the cgroup caps (L2) make
this survivable but it remains the weaker option.

### GAP-3 (P3, config — operator action, box-level) — `needle.slice` has no memory ceiling

```
$ systemctl show needle.slice -p MemoryMax
MemoryMax=infinity
```

Fleet workers are uncapped at the slice level; during the 2026-08-16 fleet-wide
event, memory pressure hit 94.7% before anything throttled. **Recommendation:
`MemoryHigh=48G` (soft throttle), not `MemoryMax`** — a drop-in on
`needle.slice`:

```ini
# /etc/systemd/system/needle.slice.d/memory.conf
[Slice]
MemoryHigh=48G
```

**Trade-offs:** `MemoryHigh` only throttles allocations past 48G (of 62G),
preserving ~14G headroom for the system and non-fleet processes; a hard
`MemoryMax` was considered and **rejected** because OOM-killing a legitimate
build mid-flight is precisely the exit-code -1 false-positive crash class this
whole program exists to eliminate. This is an operator-level box change
(outside this repo); documented here because it is the last unbounded path.

---

## 4. Trade-off Summary

| Decision | Cost | Benefit | Verdict |
|---|---|---|---|
| Cgroup-capped gc (L2) | Full gc ~1–2h vs minutes | OOM impossible to propagate past the cgroup | Accept (implemented) |
| Single gc thread + small window (L2) | Slower repack | Bounded peak memory ~4GB | Accept (implemented) |
| Box-wide gc lock (L3) | Queueing/skipped ad-hoc gcs | Kills the concurrency amplification | Accept (implemented) |
| Client-side 10MB hook (L1) | Bypassable; manual override for big fixtures | Stops bloat at the source | Accept (implemented) |
| Start weekly full-gc timer (GAP-1) | One catch-up full gc run | Documented schedule becomes real | **Do after in-flight gc edits land** |
| `gc.auto=0` (GAP-2) | Up to 24h loose-object latency | Removes the unserialized gc path entirely | **Recommend** |
| `MemoryHigh=48G` on needle.slice (GAP-3) | Throttles extreme fleet builds | System headroom preserved fleet-wide | Recommend; soft cap, not hard kill |

---

## 5. Residual Risk and Tripwires

Risk that the bf-65lsdu pattern recurs is LOW but not zero (it requires L1 to
fail first — a large file series entering history). If it does:

| Signal | Source | Response |
|---|---|---|
| Repo >1GB or loose >500MB | `repo-health.log` (daily 02:00) | `./scripts/safe-git-gc.sh --full` (serialized, capped) |
| `oom-kill` in `dmesg`/journal | system journal | Run Pattern 3 runbook; verify with `git fsck --full` |
| gc lock wait timeouts | `.git/safe-gc.log` | Investigate concurrent gc source (GAP-2 path) |
| 10+ crashes / 10 min | `crash-monitor.log` | Infrastructure event per `docs/crash-response-guide.md` |

If a bloat recurrence *does* reach the OOM stage, the proven response is the
one that resolved bf-65lsdu: stop ad-hoc retries, run the capped staged
`safe-git-gc.sh`, and split the maintenance work into sequential child beads
rather than one mega-task.

---

## 6. Verification Evidence (2026-09-02)

| Check | Command | Result |
|---|---|---|
| Repository size | `git count-objects -vH`; `du -sh .git` | 90.14 MiB pack, 0 loose, 0 garbage; 92M |
| Gitignore | `grep -n beads .gitignore` | line 66: `.beads/` |
| Pre-commit hook | `ls -la .git/hooks/pre-commit` | installed 2026-09-01 (3.4KB) |
| Memory-bounded gc | `grep cgroup scripts/safe-git-gc.sh` | `systemd-run --user --scope` MemoryMax ceiling present |
| Box-wide lock | `grep LOCK_FILE scripts/safe-git-gc.sh` | `/tmp/domain-check-safe-git-gc.lock`, 1800s wait |
| Daily gc timer | `systemctl --user show domain-check-git-gc.timer` | active; last fired 2026-09-02 03:00:10, success |
| Weekly full timer | `systemctl --user show domain-check-git-gc-full.timer` | enabled, **inactive, never fired (GAP-1)** |
| Repo-health timer | `systemctl --user show domain-check-repo-health.timer` | active; next 2026-09-03 02:00, never fired yet (GAP-1b) |
| Repo gc config | `git config -l --local` | `pack.windowmemory=2g`, `gc.auto=100` (GAP-2) |
| Fleet cgroup | `systemctl show needle.slice -p MemoryMax` | `infinity` (GAP-3) |
| System memory | `free -g` | 49G available of 62G |

---

**Recommendation summary:** no code changes to domain-check (consistent with
the zero-defects finding across 200+ investigations). Ship three operational
items — start the weekly full-gc timer (after in-flight gc-script edits land),
set `gc.auto=0`, and add the `needle.slice` MemoryHigh drop-in — then this root
cause has no remaining unbounded or unscheduled path.

**Document Version:** 1.0
**Next Review:** with GAP-1/1b follow-up after 2026-09-03 02:00 repo-health fire
