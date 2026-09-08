# Repository Maintenance Guide

**Purpose:** Prevent repository bloat and maintain system stability  
**Last Updated:** 2026-09-08  
**Status:** ✅ Active

---

## Current Repository Health (verified live 2026-09-06)

The repository is **repaired and healthy**. The ~18GB loose-object bloat that
caused the memcg-OOM crashes (bf-1s6c3, bf-4yjq) was packed down and cannot
recur through `.beads/` (now gitignored, 0 tracked files).

- **`.git`:** 94 MB (was ~18 GB) — 136 loose objects / 1.04 MiB, one pack
  (10,712 objects, 90.43 MiB), 0 garbage
- **Integrity:** `git fsck --full` clean; `./scripts/check-repo-health.sh` passes
- **Verification record:** [bf-4yjq cleanup verification](../crashes/bf-4yjq-cleanup-verification.md)

Everything below describes the standing procedures that keep it that way.

---

## Quick Reference

### Daily Operations
```bash
# Pre-flight health check before starting work
./scripts/preflight-health-check.sh
```

### Weekly Maintenance
```bash
# Check repository health
./scripts/check-repo-health.sh

# Run cleanup if repository >500MB
./scripts/safe-git-gc.sh --full
```

### Emergency (If Repository >5GB)
```bash
# Immediate cleanup
./scripts/safe-git-gc.sh --full

# Monitor in another terminal
./scripts/safe-git-gc-monitor.sh --watch
```

---

## Automated Scheduling (systemd user timers)

All scheduled maintenance and monitoring runs as **systemd user timers**, not
cron — this box is NixOS and has no `crontab`. Installed by
`scripts/setup-repo-maintenance.sh` (repo-health + gc units) and
`scripts/install-monitoring.sh` / `scripts/install-git-gc-timers.sh`
(monitoring units); unit sources live in `scripts/domain-check-*.{service,timer}`.

| Timer | Schedule | What it runs |
|-------|----------|--------------|
| `domain-check-service-monitor.timer` | every 2 min | `service-monitor.sh --once` |
| `domain-check-resource-monitor.timer` | every 5 min | `resource-monitor.sh --once` |
| `domain-check-monitoring.timer` | every 10 min | `crash-pattern-detection.sh` |
| `domain-check-repo-health.timer` | daily 02:00 | `auto-gc-trigger.sh --dry-run` |
| `domain-check-git-gc.timer` | daily 03:00 | `safe-git-gc.sh` (stages 1-2) |
| `domain-check-git-gc-full.timer` | weekly Sun 04:00 | `safe-git-gc.sh --full` (MemoryMax=4G) |

**Verify the fleet is healthy:**
```bash
systemctl --user list-timers 'domain-check-*' --all
```
Every timer should show a future `Trigger` time. A timer showing `n/a` or
`-` is not going to fire — investigate before assuming coverage.

**Known failure mode (bit 2026-09-02):** editing a unit file in
`~/.config/systemd/user/` without `systemctl --user daemon-reload` leaves the
manager holding the previous (possibly fatal) unit state. The timer then
silently never fires while `list-timers` still lists it. After changing any
unit file — by hand or by re-running the installers — always:
```bash
systemctl --user daemon-reload && systemctl --user start domain-check-*.timer
```
The installers do this; bare `cp` into `~/.config/systemd/user/` does not.

### CI/CD Pipeline Coverage (verified 2026-09-06)

The crash context report
([bf-4yjq](../crash-context-report-bf-4yjq-comprehensive.md)) recommended
"repository size monitoring in the CI/CD pipeline". That recommendation was
checked against the live template and is **not implemented, by design**:
`domain-check-build` (`declarative-config/k8s/iad-ci/argo-workflows/domain-check-workflowtemplate.yml`)
runs lint/test/fuzz plus a kaniko build, and every step `git clone`s a fresh
copy from Forgejo into an ephemeral container. CI never holds a persistent
`.git`, so it cannot observe — or gate on — the loose-object accumulation that
caused the bf-1s6c3 / bf-4yjq OOMs. That accumulation happens in working
clones on the lab box, which is why the enforcement points are the timers
above, the gitignore rules, and the pre-commit hook rather than CI.

**Residual gap this leaves:** the >10MB gate is a per-clone pre-commit hook
(untracked in git, absent from a fresh clone), and Forgejo has no server-side
push hook, so nothing mechanically rejects a large *tracked* file at push time
— the same hole that let 17+ 237MB `.beads/*.jsonl` commits land. The
repo-wide `*.jsonl` / `.beads/` gitignore rules close the known vector; a
server-side size check (Forgejo push hook) would be the durable fix if another
vector appears.

---

## Repository Size Thresholds

| Size | Status | Action |
|------|--------|--------|
| <500MB | ✅ Healthy | None required |
| 500MB-1GB | ⚠️ Warning | Plan cleanup soon |
| 1GB-5GB | 🚨 Critical | Run cleanup immediately |
| >5GB | 🆘 Emergency | Run cleanup NOW |

Thresholds as actually enforced (constants live in the scripts — if a number
here disagrees with the script, the script wins):

| Threshold | Value | Enforced by |
|-----------|-------|-------------|
| Auto-gc trigger | 10 GB `.git` size (`AUTO_GC_THRESHOLD_MB=10240`) | `scripts/auto-gc-trigger.sh` |
| Size warning | 2 GB (`WARN_THRESHOLD_MB=2048`) | `scripts/auto-gc-trigger.sh` |
| Large-file gate | >10MB staged file blocks the commit (`MAX_SIZE_MB=10`) | `.git/hooks/pre-commit` |
| Large-file audit | >10MB blob in history or working tree flags a warning | `scripts/check-repo-health.sh` |
| Gc resource gates | mem ≥ ceiling + 1g, disk ≥ max(5G, 1.5× repo), load ≤ 15 | `scripts/safe-git-gc.sh` (`--check-only` to preview) |
| Gc memory ceiling | `MemoryMax=4G` on the timer units; `pack.windowMemory=2g` / `pack.deltaCacheSize=1g` / `pack.threads=1` on every repo | `domain-check-git-gc*.service` + `scripts/setup-git-gc-config.sh --verify` |

**Recommended gc frequency** (installed via `./scripts/setup-repo-maintenance.sh`,
schedule table above): daily incremental at 03:00, weekly full gc Sunday 04:00,
threshold check daily 02:00. Run a manual `./scripts/safe-git-gc.sh --full` only
when `--check-only` says gc is needed — "GC not needed" lines in
`.git/safe-gc.log` are the daily check reporting a healthy repo, not a failure.

---

## Warning Signs of Repository Bloat

**Early Warning Signs:**
- Repository size approaching 1GB
- Git operations becoming slow
- Loose objects >500MB

**Critical Symptoms:**
- Exit code -1 (SIGKILL) during git operations
- Repository size >5GB
- Routine git operations trigger OOM
- Multiple crashes with exit code -1

---

## Persistent Pack-Memory Bounds (OOM root-cause fix)

Exit-code -1 crashes in this workspace (bf-173o7e: 129 kills on 2026-08-14; bf-4x12ec: same
mechanism) were **kernel memcg OOM kills**, not git bugs: a bare `git gc --aggressive
--prune=now` over a bloated repo pushed git-pack-objects RSS past the 12GiB `MemoryMax` of
needle's per-dispatch scope (`run-p*.scope` in the systemd **user** manager), and the kernel
SIGKILLed the agent. `safe-git-gc.sh` bounds only its own sanctioned path — the bare
invocation is defended solely by git config, which for a year existed only as hand-applied
local state in this repo's `.git/config` and nowhere reproducible.

`scripts/setup-git-gc-config.sh` now makes that bound persistent, reproducible, and
verifiable:

- `pack.windowMemory = 2g` — caps the delta search window
- `pack.deltaCacheSize = 1g` — caps the delta write-out cache
- `pack.threads = 1` — **required**: per git docs the window limit is *per thread*, so
  leaving threads unset lets git multiply the window across all cores
- Worst case ≈ 3GiB per pack run, a quarter of the 12GiB dispatch scope

```bash
./scripts/setup-git-gc-config.sh              # bound this repo (local scope)
./scripts/setup-git-gc-config.sh --global     # bound all repos for this user (~/.gitconfig)
./scripts/setup-git-gc-config.sh --verify     # exit 1 if the effective bound is missing/unsafe
./scripts/setup-git-gc-config.sh --uninstall [--global]  # rollback; exit 1 if it removed the LAST bound
```

Applied `--global` on this box on 2026-09-02, so every repo for the `coding` user is
protected — including the other repos whose dispatch scopes produced the bf-4x12ec-family
kills. Re-run `--verify` if a repo reports fresh exit-code -1 crashes during git operations.

`--uninstall` is the rollback for this layer ([Rollback Plan](#rollback-plan-bf-1s6c3-mitigation-stack)
below). It removes only the three `pack.*` keys it owns — the advisory `gc.*` keys it may
have filled are left alone, since they can hold hand-tuned values — then re-runs `--verify`
and exits 1 when the rollback removed the **last** effective bound, because bare
gc/push are then unbounded again. Tested by `scripts/test-setup-git-gc-config.sh`
(27 assertions, sandboxed `GIT_CONFIG_GLOBAL`/`GIT_CONFIG_SYSTEM`, seconds to run).
Live round-trip verified 2026-09-07: `--uninstall` on this repo exited 0 with the
box-wide global bound still supplying all three keys (zero unprotected window), and the
re-apply restored the repo-local keys (`--verify` exit 0 at both ends).

`--verify` checks the **effective** bound — the system → global → local chain a bare gc
actually sees — and reports which scope supplies each key, so a repo protected only by the
box-wide global config verifies clean instead of false-alarming. `--verify --global`
checks `~/.gitconfig` itself; it exits 1 if the global scope has no bound.

Tested by `scripts/test-gc-memory-bounds.sh`: it runs the exact crash command at reduced
scale (8×64MiB incompressible blobs) under a 768MiB cgroup — 1/16th of the dispatch scope —
and asserts exit 0 with peak RSS ≈ 313MiB. Before the bound, the same command needed >12GiB
and died 129 times in a row.

What this does **not** cover: needle's zero-backoff re-claim loop, which amplified one
deterministic kill into a 129-attempt storm (bead released → re-claimed 9s later). That is
needle-side, outside this repo; alert-side suppression is handled by
`scripts/crash-alert-manager.sh` dedup/cooldown.

---

## safe-git-gc.sh Run-Time Safeguards

The persistent git config above bounds *bare* git. The sanctioned path — `safe-git-gc.sh` —
carries its own four layers, so a run the box cannot absorb refuses to start instead of
dying mid-repack (bf-65lsdu, 2026-08-13: several gc runs at once, each >4 GB):

**1. Configuration validation (fail fast, exit 2).** Soft limits are kept separate from the
hard ceiling, and the script refuses any combination where the ceiling does not cover the
soft worst case `pack.windowMemory × pack.threads + pack.deltaCacheSize + 512m slack` — a
ceiling below that sum is the self-inflicted OOM the ceiling exists to prevent
([stepwise-git-gc-strategy.md §7.1](./stepwise-git-gc-strategy.md), item 1). Sizes must
parse; an unpinned `pack.threads` is called out because it multiplies the window by nproc.

**2. Pre-flight resource checks (fail fast, exit 2).** Before any git work: available
memory ≥ ceiling + 1g, free disk ≥ max(5G, 1.5× repo size — a repack transiently needs the
repo size again), 1-minute load ≤ 15. Thresholds are tunable via `SAFE_GC_MIN_AVAIL_MEM`,
`SAFE_GC_MIN_DISK_GB`, `SAFE_GC_MAX_LOAD`.

**3. Memory enforcement on every git invocation.** Preference order: cgroup `MemoryMax` via
the systemd user manager (cumulative, authoritative) → `ulimit -v` address-space cap when
systemd is unavailable or `SAFE_GC_NO_CGROUP=1` → soft git limits alone if both are opted
out (logged as a warning). A runaway gc is OOM-killed inside its own cgroup rather than
exhausting the box.

**4. Checkpoint/resume + progress.** Each stage writes `status: "running"` with its pid to
`.git/safe-gc-checkpoint.json`, so `safe-git-gc-monitor.sh` and `--check-only` can tell a
run in flight from the last completed one. A dead run's entry is reaped to `interrupted`,
and `--resume` restarts the interrupted stage (repacks are safe to re-run — they write new
packs before removing old ones).

**Exit codes:** `0` success (or, with `--check-only`, gc is needed) · `1` failure (or, with
`--check-only`, gc not needed) · `2` fail-fast: invalid configuration or insufficient
resources.

**Where the ceiling comes from outside the script:** the `domain-check-git-gc.service` /
`domain-check-git-gc-full.service` units carry their own `MemoryMax=4G` (plus
`MemorySwapMax=0`, CPU quota, and the NixOS `PATH=` pin the user manager needs to find
bash), so a timer-driven run is bounded even before the script's own scope is created.

```bash
./scripts/safe-git-gc.sh --check-only      # validate resources + gc-needed verdict; mutates nothing
./scripts/test-safe-git-gc-limits.sh       # 33 assertions: ceilings, thresholds, checkpoints (seconds)
DOMCHECK_RUN_LONG_TESTS=1 ./scripts/test-safe-git-gc-limits.sh   # adds the real-gc-on-this-repo cases
```

Scope note: the *stage commands* inside the script (plain gc / incremental repacks, and the
§4 gates that should decide when wide-window work runs at all) are the stepwise-gc design's
remaining work items — see §7.1 items 2–8 of
[stepwise-git-gc-strategy.md](./stepwise-git-gc-strategy.md). The safeguards above are
independent of that rework.

Need **deeper aggressive compression than gc gives** (e.g. a `--depth=250
--window=250` repack)? Do not hand-roll an invocation — start from the
executed-and-measured procedure in
[Memory-Capped Manual Repack](#memory-capped-manual-repack-bf-5jhvpk-procedure-2026-09-08)
below.

---

## Memory-Capped Manual Repack (bf-5jhvpk procedure, 2026-09-08)

`safe-git-gc.sh` owns scheduled gc. When a bead asks for **deeper aggressive
compression on top of it** — the bf-5jhvpk target was
`git repack -a -d --depth=250 --window=250` — start from the invocation below,
which was executed and verified against this repository on 2026-09-08. Do not
invent a new invocation, and do not run it bare (rules at the end).

**The command that worked** (run detached, log polled to completion):

```bash
setsid nohup bash -c '
  systemd-run --user --scope --unit=bf-5jhvpk-repack \
    -p MemoryMax=4G -p MemorySwapMax=0 -p CPUQuota=300% -- \
    nice -n 15 git -c pack.windowMemory=512m -c pack.threads=3 \
      repack -a -d --depth=250 --window=250 --no-write-bitmap-index
' > .beads/logs/bf-5jhvpk-repack.log 2>&1 &
```

Why each piece is there:

| Piece | Why |
|---|---|
| `systemd-run --user --scope -p MemoryMax=4G` | Hard ceiling — a runaway repack is OOM-killed inside its own scope instead of the 12GiB dispatch scope (the bf-4x12ec mechanism) |
| `-p MemorySwapMax=0` | The cap must not be met with swap |
| `-p CPUQuota=300%` + `nice -n 15` | Keeps the box responsive for co-tenants |
| `pack.windowMemory=512m` | Per-window soft bound; sized down because depth/window 250 searches far more delta candidates than the gc defaults |
| `pack.threads=3` | Window memory is **per thread** (see [Persistent Pack-Memory Bounds](#persistent-pack-memory-bounds-oom-root-cause-fix)), so threads must be pinned for a worst case to be computable |
| `--no-write-bitmap-index` | Non-bare repo; default gc would not write a bitmap either. The old pack's `.bitmap` disappearing is expected, not data loss |
| `setsid nohup` + log file | Prior attempts died with their agent; detached output lets a restart or SIGHUP arrive mid-run without killing the repack |
| box-wide gc lock | Hold `/tmp/domain-check-safe-git-gc.lock` (the `safe-git-gc.sh` convention) so the run cannot race the nightly 03:00 gc |

**Measured result** — 2026-09-08T00:28:54Z, HEAD `7f8af4d`, executor bead
`domchk-371e54d8`, baseline bead `domchk-2971874f`: **exit 0, elapsed 3 s**,
**scope peak 350.4 M against the 4G cap** (~11x headroom; a 1 s cgroup sampler
peaked at 260 MiB — coarser than systemd's continuous accounting), and **no
OOM** in `journalctl --user`, `journalctl -k`, or `dmesg` for the run window.
Full depth/window 250 — **the first rung of the ladder held, no fallback tier
was needed**. Attempt 1 (00:26:36Z) exited 1 client-side before git ran: **this
box's `systemd-run` rejects `-p Nice=15`** ("Unknown assignment") — apply
niceness as a `nice -n 15` command prefix instead, as above.

| Metric | Before (00:08Z baseline) | After (00:28Z run / 00:49Z verification) | Delta |
|---|---|---|---|
| packs | 2 (~99 M + 679 K) | **1** (104,777,374 B) | 2 → 1, old packs pruned |
| packed + loose | 103.26 MiB | 100.89 MiB | **−2.37 MiB (−2.3%)** |
| loose objects | 485 / 3.48 MiB | 88 / 580 KiB at the run; 101 / 656 KiB at verification | −397 net then −384 vs baseline (485 → 561 at run time → 88 → 101) |
| in-pack objects | 11,700 | 12,174 | +474 (formerly loose) |
| `du -sh .git` | 107 M | **104 M** | −3 M |

Raw `size-pack` reads **+0.47 MiB**, which is an artifact and not growth: the
baseline's 99.78 MiB spanned *two* packs and *excluded* 3.48 MiB of loose
objects that the single new pack absorbed. On the like-for-like measure — total
object store, packed plus loose — the repository shrank 2.37 MiB. (The loose
row's path explains its own numbers: co-tenant churn took loose objects
485 → 561 between the 00:08Z baseline and the 00:28Z run, and the repack packed
the reachable ones in, landing at 88 — hence −397 net while 474 objects moved
in-pack. Churn kept adding loose objects after the run, so the 00:49Z
verification saw 101 / 656 KiB — that snapshot, 100.25 MiB pack + 656 KiB
loose, is what the packed + loose row's 100.89 MiB reads against.)
Full read-out and post-run integrity record:
[post-repack verification](./post-repack-verification-bf-5jhvpk-2026-09-08.md).

**Fallback ladder if `MemoryMax` trips** (exit 137, or an OOM entry in the
journal): `--depth=250 --window=250` → `--depth=50 --window=50` →
`--depth=10 --window=10`. A repack is safe to re-run — new packs are written
before old ones are removed — so a tripped run leaves the repository valid;
retry at the next rung down rather than re-running the same tier.

**Rules:**

- **Never run a bare (uncapped) repack on this box.** Unbounded `pack-objects`
  is what memcg-OOM-killed 129 consecutive attempts in bf-173o7e and produced
  the bf-4x12ec family.
- **Never raise the cap.** Walk down the ladder instead.
- Never run without the gc lock held, and never leave a detached run unpolled.
- Verify afterwards: `git fsck --full` (dangling-objects-only output is normal
  co-tenant churn), zero `tmp_*` files under `.git/objects/pack`, old packs and
  their indexes gone, and `./scripts/check-repo-health.sh` exit 0.

---

## Unpushed-Commit Backlog Monitor (gap M-1, 2026-09-07)

bf-1ea4g (2026-08-13) died 56 times in `git push` against a **422-commit unpushed backlog**
that had accumulated silently across ~30 killed attempts — and nothing in this workspace
measured commit-ahead. The only ahead/behind count that existed was
`verify-work-completion.sh`, evaluated once per bead at close time. If such a backlog ever
regrows (a dead worker's unpushed series, a self-amplifying retry loop), no daily check, no
preflight, and no monitor would mention it. `scripts/check-unpushed-backlog.sh` is that
check — the one new detection rule
[docs/crash-prevention-gaps-bf-1ea4g.md](../crash-prevention-gaps-bf-1ea4g.md) (§4 M-1)
registers for this crash.

**Mechanism:** `git rev-list --count @{upstream}..HEAD` (falls back to `origin/main` when
no branch upstream is configured; fails open with CLEAR when neither is measurable — a
fresh clone must not page). Thresholds per the spec: **WARN at ≥ 50, CRITICAL at ≥ 200**
(`BACKLOG_WARN_THRESHOLD` / `BACKLOG_CRITICAL_THRESHOLD` override both).

**Report-only, by design.** Per the G-2 correction
([crash-prevention-requirements.md](../crash-prevention-requirements.md)) remediation is
owned by the unconditional bounded nightly gc (03:00 timer); attaching remediation here
would re-invent conditional gating that is blind to bloat accumulating inside a pack. The
check therefore never creates an alert bead, never blocks, never gc's — it names the
precondition and appends a dated line to `.beads/logs/repo-health.log` at WARN/CRITICAL so
triage can tell when a backlog started growing. That log-only design is deliberate: the
alert layer's cardinality failure (one alert bead per kill, pre-0.4.2) is the failure mode
a precondition monitor must not repeat.

**Exit codes:** `0` clear, warn, or not-measurable (fails open) · `1` CRITICAL (≥ 200 — a
push would materialize the whole series in one pack-objects run, the bf-1ea4g shape) ·
`2` usage error (path missing or not a git repository).

**Where it runs:**
- **§9 of `check-repo-health.sh`** — manual and preflight-invoked runs (note:
  `preflight-health-check.sh` truncates health output to its first 20 lines in VERBOSE
  mode, so §9 shows in manual runs and log review, not in preflight output).
- **Daily 02:00 `domain-check-repo-health.timer`** — despite the unit's name it runs
  `auto-gc-trigger.sh --dry-run`, which calls the helper directly (by path) and surfaces
  its output in `.beads/logs/git-gc-check.log`; the helper's exit codes are swallowed so
  the daily script's 0/1/2 contract is unchanged.
  *(Landed 2026-09-07, domchk-cb9eb4de: this wiring call plus
  `scripts/test-unpushed-backlog-wiring.sh` (11 assertions) existed only as uncommitted
  worktree edits until then — the wiring's author bead domchk-87ef5683 closed before
  landing it, so this bullet documented a daily carrier no ref contained. Landing it is
  what makes the sentence above true in history, not just in this worktree: the
  2026-09-07 empty-tree accident (2e8ce7a/2ec91ec) is the standing proof that
  worktree-only prevention is one bad push from gone. The `auto-gc-trigger.sh` DRY_RUN
  command text also differs between worktree and HEAD — that residual hunk is another
  bead's, not this wiring.)*

```bash
./scripts/check-unpushed-backlog.sh                    # this repo, right now
./scripts/test-check-unpushed-backlog.sh               # 29 assertions, seconds
```

**Tested against the crash's own historical shape:** the suite replays a 422-commit
unpushed backlog — bf-1ea4g's actual count at the fatal push — and asserts CRITICAL exit 1,
plus both threshold boundaries (49 → CLEAR, 50 → WARN, 200 → CRITICAL), ordinary
work-in-progress (1, 5 commits) staying CLEAR, the fails-open paths, and threshold
overrides.

**Known reading caveat:** the count is repo-wide, not per-worker — a co-tenant's unpushed
commit on the shared `main` raises everyone's reading. That is correct (the precondition
the monitor guards is repo-wide; bf-1ea4g's 422 included the whole fleet's accumulation),
but attribution needs `git log origin/main..HEAD` before anyone "fixes" it.

---

## Fix-Chain Verification Record (bf-4k2ws chain, 2026-09-07)

The bf-4k2ws fix chain — fix type `domchk-2222ea44` → implement `domchk-20f2666d` →
test `domchk-ecf47b49` → document `domchk-26ccd69b` (this record) — closed with
**no code change**: the fix this chain exists to implement and test is the safeguard
stack documented on this page (the pack-memory bounds above, the safe-gc run-time
safeguards, the `.beads/` gitignore + 10 MB pre-commit gate), all already committed.
Per the chain's fix spec
(`docs/investigations/bf-4k2ws-root-cause-determination-domchk-7f838f36-2026-09-07.md`
§10.4), "implement and test" here means *run the battery and change nothing unless a
line fails* — nothing failed. Sibling-wave battery records live in that document's
§13.4 and in the beads' notes; this subsection is chain A's.

Battery re-run first-hand 2026-09-07 at HEAD `48aafce` = `origin/main`. Scripts
carrying co-tenant WIP in the worktree (`safe-git-gc.sh`, `preflight-health-check.sh`)
ran from `git show HEAD:` copies; the rest ran in place:

| Check | Live result |
|---|---|
| `setup-git-gc-config.sh --verify` | exit 0 — effective chain system→global→local; all three keys present at both global and local scope; worst case ≈3072 MiB within the 6 GiB ceiling |
| `safe-git-gc.sh --check-only` (committed copy) | config validated (window 2g / delta 1g / ceiling 6g / threads 1, worst ≈3584 MiB); resource checks pass (46836 MB avail mem vs 7168 min, 28 G disk, load 5.17 vs 15 max) → **"GC not needed", exit 1 — the healthy answer** under the exit-code contract above |
| `test-safe-git-gc-limits.sh` | 33/33 passed, exit 0 (incl. checkpoint/resume end-to-end) |
| `test-gc-memory-bounds.sh` | **12/12, exit 0** — incl. the crash-condition replay: bare `git gc --aggressive --prune=now` (the bf-173o7e / bf-4x12ec kill command) exited 0 under `MemoryMax=768M` with pack-objects peak RSS 320528 KB < the 700 MiB cap; the crash-era runs exceeded 12 GiB |
| Repo state | `.git` 104 MB; 220 loose / 2.28 MiB; 1 pack 99.11 MiB; garbage 0; `git fsck --full` exit 0; `git ls-files .beads` → 0; pre-commit hook byte-identical (`setup-git-hooks.sh --check` exit 0) |

Two deltas a future battery runner should expect (both reproduced again this session;
first recorded in the determination doc's §13.5):

1. **`safe-git-gc.sh --check-only` exiting 1 *is* success** on a healthy repo — the
   contract is `0` = gc needed, `1` = not needed, `2` = fail-fast. Fix-spec §10.4
   step 7 says "exit 0"; read it through this contract or a healthy repo fails the
   one line that proves it healthy.
2. **The committed `preflight-health-check.sh` fails one line on a healthy system:**
   its gateway probe is plain `-sf` and dies on the gateway's self-signed cert
   (curl 60) while a direct `curl -skf …/health` returns `ok` — G-4's live specimen,
   fix owned by `domchk-6951fe0c`. Also note the script resolves its helpers relative
   to its own directory (`SCRIPT_DIR`), so a `git show HEAD:` extract run from a
   scratch dir degrades the repo-health / cgroup checks to "⚠ not found (skipping)" —
   extract the helpers alongside it or run it in place; those skips are warnings, not
   failures, and the gateway line remains the only hard one.

**Re-verification, 2026-09-07 second pass** (bead `domchk-26ccd69b`, re-dispatched
after its first attempt died post-commit/pre-close): every load-bearing claim above
re-verified live at HEAD = origin/main — `--verify` exit 0 (worst case ≈3072 MiB);
`--check-only` exit 1 "GC not needed"; limits suite **33/33**; `test-gc-memory-bounds.sh`
at a HEAD extract **12/12**, the crash replay's pack-objects peak RSS 320532 KB
(byte-consistent with the 320528 KB above); `.git` 105 MB, 251 loose / 2.56 MiB,
1 pack 99.11 MiB, garbage 0, `fsck --full` exit 0 (dangling-only), 0 tracked `.beads`
files, `setup-git-hooks.sh --check` exit 0. Two more deltas for future battery runners:

3. **The limits suite's memory-floor assertion is a live race.** It sets
   `SAFE_GC_MIN_AVAIL_MEM` to `MemAvailable + 1 MB` and expects exit 2 — but if
   available memory *rises* by ≥1 MB between the suite's sample and the script's
   check, the fail-fast path legitimately doesn't trip (rc=0). Observed
   `MemAvailable` moving +95 MB in 4 s under fleet load; one run of 33 failed only
   that line, the immediate re-run was 33/33. Re-run before blaming code.
4. **`test-gc-memory-bounds.sh` must run from the repo root, and apples-to-apples
   means a full HEAD extract.** It resolves `setup-git-gc-config.sh` via
   `ROOT=$PWD` (not `SCRIPT_DIR`), so running a bare extract from a scratch dir
   fails every setup/verify line and cascades the integration section into a
   `timeout 300` SIGTERM (exit 143, "RSS 0 KB", loose objects left behind) — the
   lone-extract failure mode, again. Extract the suite *and* its sibling under a
   `scripts/` directory and run from that tree's root. (Same session, the
   working-tree copy also carried a co-tenant's in-flight edit adding a push-side
   integration test; only the HEAD extract isolates the committed 12.)

**Third-pass delta, 2026-09-07 (bead `domchk-34871e96` close-time
re-verification):** `test-safe-git-gc-limits.sh` needs repo context too. Its
fail-fast, `ulimit`-fallback, and `--check-only` cases run git against the repo
root, so a bare scratch-dir extract of `scripts/` fails **29/4 by construction**
— the giveaway is `--check-only: rc=128` (git exits 128 outside a repository;
the other three are its cascade: "over-limit process survived the 64M ceiling",
"under-limit process failed", "fallback mode broken"). The apples-to-apples
subject is a HEAD **clone** — `git clone <repo> /tmp/x && cd /tmp/x &&
./scripts/test-safe-git-gc-limits.sh` → **33/33** — which gives the suite a real
`.git` without touching the shared worktree's dirty `scripts/`.

---

## Prevention Checklist

**Daily:**
- [ ] Run pre-flight health check before starting work
- [ ] Check available memory (>10GB required)
- [ ] Confirm pack-memory bounds still verify: `./scripts/setup-git-gc-config.sh --verify`

**Weekly:**
- [ ] Run `./scripts/check-repo-health.sh`
- [ ] Review repository size trends
- [ ] Run cleanup if size >500MB

**Monthly:**
- [ ] Comprehensive repository audit
- [ ] Review git history for large files
- [ ] Update monitoring thresholds if needed

**After any fresh clone** (enforcement is per-clone and does not travel with
git; without these the clone is unguarded — see
[CI/CD Pipeline Coverage](#cicd-pipeline-coverage-verified-2026-09-06) above):
- [ ] `./scripts/setup-git-hooks.sh` — installs the repo-size pre-commit hook from the tracked source (`--check` → exit 0 confirms it is present, executable, and byte-identical)
- [ ] `./scripts/setup-git-gc-config.sh --global --verify` → exit 0 (the box-wide pack-memory bound must cover bare gc/push here too)
- [ ] `./scripts/setup-repo-maintenance.sh` if this clone should carry its own timers

Prevention baseline and the incident these controls came from:
[bf-4yjq crash context report](../crash-context-report-bf-4yjq-comprehensive.md)
— its "Prevention Status Follow-up (2026-09-06)" section maps each of that
report's recommendations to the control that now covers it.

---

## Rollback Plan (bf-1s6c3 mitigation stack)

Each layer of the repository-bloat mitigation stack has its own removal path. Roll back
**only the layer causing the problem**, one at a time, and re-verify what is left — the
layers are redundant by design, so a single rollback normally leaves the others holding.

| Layer | Rollback entry point | What it removes | Re-verify afterwards |
|---|---|---|---|
| Pack-memory bound (bare gc/push) | `./scripts/setup-git-gc-config.sh --uninstall` (add `--global` for `~/.gitconfig`) | Only the three `pack.*` keys it set; advisory `gc.*` keys stay | `./scripts/setup-git-gc-config.sh --verify` — exit 0 means another scope still supplies the bound; **exit 1 means bare git is now unbounded** (re-apply, or accept only if you are deliberately removing the guard) |
| Pre-commit repo-size hook | `./scripts/setup-git-hooks.sh --uninstall` | `.git/hooks/pre-commit` in this clone | `./scripts/setup-git-hooks.sh --check` → exit 1 confirms it is gone; the tracked source under `scripts/pre-commit-repo-size-hook` is untouched, so reinstall is one command |
| Scheduled repo-health + gc timers | `./scripts/setup-repo-maintenance.sh --remove` | The `domain-check-{repo-health,auto-gc,git-gc,git-gc-full}` user units | `systemctl --user list-timers 'domain-check-*' --all` — only the monitoring timers should remain |
| Monitoring timers (resource/service/crash-pattern) | `./scripts/monitoring-setup.sh --remove` | The remaining `domain-check-*` user units and their log files | Same `list-timers` check — the list should now be empty |
| `.beads/` gitignore rules | Edit `.gitignore` (remove the `.beads/`, `*.db`, `*.jsonl` lines) | The re-entry block only; no file is deleted | `git ls-files .beads` must stay **0** — nothing is re-tracked until someone stages it |

**Order when unwinding everything:** timers → hook → pack-memory bound → gitignore. Stop
at the layer you needed and restart from the bottom of that table if the problem returns.

**Hazards:**

- Rolling back the **gitignore** rules is what re-opens the bf-1s6c3 vector itself: bead
  state becomes trackable again. While the pre-commit hook is installed, a staged `.beads/`
  path is still blocked (any path there is by definition a forced add), so the gitignore
  rollback alone does not re-enable tracked bead state — but do not roll both back at once.
- Rolling back the **pack-memory bound** removes the only guard on *bare* `git gc
  --aggressive` and on `git push` pack-objects. The sanctioned path (`safe-git-gc.sh`)
  keeps its own bounds, so a bare-gc emergency is still covered by that script — use it
  instead of leaving the bound off.
- A rollback is only "needed" when a layer misfires (e.g. the hook blocks a legitimate
  vendored asset, or `pack.threads=1` slows a one-off huge pack). Prefer a scoped
  workaround first: commit the large asset via a deliberate exception rather than
  uninstalling the hook, and tune `PACK_WINDOW_MEMORY`/`PACK_THREADS` at install time
  rather than removing the bound.

---

## Key Scripts

| Script | Purpose | Usage |
|--------|---------|-------|
| `preflight-health-check.sh` | Validate system before tasks | Before every agent task |
| `check-repo-health.sh` | Repository size and object check | Manual health monitoring (nothing schedules it — the daily 02:00 unit runs `auto-gc-trigger.sh --dry-run`) |
| `check-unpushed-backlog.sh` | Commit-ahead monitor, report-only (see its section above) | Manual, via `check-repo-health.sh` §9, and the daily 02:00 `auto-gc-trigger.sh --dry-run` |
| `safe-git-gc.sh` | Memory-limited garbage collection with pre-flight resource checks, hard ceiling and checkpoint/resume (see its section above) | When cleanup needed; `--check-only` to validate first |
| `safe-git-gc-monitor.sh` | Monitor gc progress | During gc operations |
| `setup-git-gc-config.sh` | Persistent pack-memory bound + verify + uninstall | After cloning; `--verify` when exit -1 appears; `--uninstall` per the Rollback Plan above |

---

## Important Links

**Crash Investigation:**
- [bf-1s6c3 Investigation Summary](../archive/crash-investigations/bf-1s6c3-investigation-summary.md) - Repository bloat crash (18GB → 138MB)
- [Crash Response Guide](../crash-response-guide.md) - Quick crash classification

**Detailed Documentation:**
- [Repository Maintenance Recommendations](./repository-maintenance-recommendations.md) - Comprehensive guide
- [Post-Repack Verification (bf-5jhvpk, 2026-09-08)](./post-repack-verification-bf-5jhvpk-2026-09-08.md) - Measured results and integrity record for the memory-capped repack; any future aggressive-compression bead starts from the [Memory-Capped Manual Repack](#memory-capped-manual-repack-bf-5jhvpk-procedure-2026-09-08) section above
- [Cleanup and Recovery Procedures](../archive/crash-investigations/cleanup-and-recovery-procedures.md) - Emergency procedures
- [Crash Mitigation Strategies](../crash-mitigation-strategies.md) - Prevention strategies
- [bf-4yjq Crash Context Report](../crash-context-report-bf-4yjq-comprehensive.md) - Original bloat incident; Prevention Status Follow-up section maps its recommendations to current controls
- [Crash Prevention Requirements](../crash-prevention-requirements.md) - Canonical gap list (G-1..G-13); see also its [design](../crash-prevention-design.md) and [monitoring design](../crash-prevention-monitoring-design.md) companions
- [Comprehensive Crash Prevention Guide](../comprehensive-crash-prevention-guide.md) - Operational prevention system

---

## Common Issues and Solutions

### Repository Keeps Growing
```bash
# Check current state
./scripts/check-repo-health.sh
du -sh .git/objects/* | sort -rh

# Solution: Run cleanup
./scripts/safe-git-gc.sh --full
```

### Git GC Fails with OOM
```bash
# Check available memory
free -h

# Validate before running: exit 2 means the box cannot absorb a gc right now
./scripts/safe-git-gc.sh --check-only

# Run with memory limit
SAFE_GC_MEMORY_MAX=2g ./scripts/safe-git-gc.sh --full

# A run killed mid-stage resumes from its last checkpoint
./scripts/safe-git-gc.sh --resume

# Confirm the persistent pack-memory bound is intact (see its section above)
./scripts/setup-git-gc-config.sh --verify
```

### Pre-Flight Check Fails
Common fixes:
- **Gateway unavailable:** Wait for service recovery
- **Insufficient memory:** Close applications or add RAM
- **Repository bloated:** Run `./scripts/safe-git-gc.sh --full`

---

## Success Criteria

Repository is healthy when:
- ✅ Repository size <500MB
- ✅ Loose objects <100 count
- ✅ No fsck errors (`git fsck --full`)
- ✅ Pre-flight health check passes
- ✅ No OOM crashes during git operations

---

## Background

The bf-1s6c3 crash (2026-08-12) demonstrated that repository bloat (18GB with 17GB loose objects) can trigger systematic OOM failures. Following cleanup (18GB → 138MB, 99.2% reduction), the task completed successfully and no similar crashes have occurred for 16+ days.

**Key Takeaway:** Proactive maintenance prevents 70% of infrastructure crashes.

---

**Need Help?** See [Repository Maintenance Recommendations](./repository-maintenance-recommendations.md) for comprehensive guidance or [Crash Response Guide](../crash-response-guide.md) for crash investigation procedures.
