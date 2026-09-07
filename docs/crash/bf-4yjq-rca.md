# Root Cause Analysis — bf-4yjq agent crash

**Subject bead:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo mirror has
diverged/gone stale" (P2; the *task* closed 2026-08-17, verified complete — this RCA covers
the **agent crashes** that occurred while it was being worked)
**RCA dispatch:** domchk-b1177594 · **Written:** 2026-09-06 (lab, EDT)
**Input:** [`bf-4yjq-crash-context.md`](bf-4yjq-crash-context.md) (domchk-577a6273) and the
four gather layers under [`bf-4yjq/`](bf-4yjq/)
**Crash date:** 2026-08-12 — 50 deaths, 17:53:53.875Z → 20:30:38.310Z UTC

---

## 1. Confirmed root cause

> **Repository bloat met a finite memory boundary.** The workspace's git store had grown to
> ~18 GB — 17.2 GiB of loose objects, 17+ identical ~237 MB `.beads/*.jsonl` snapshots that
> a pre-gitignore workflow kept committing. Every agent dispatch ran inside a
> `systemd-run --user --scope` unit with `MemoryMax=12GiB`. Each attempt committed one more
> snapshot and then died at `git push origin main`: `git pack-objects`, walking the 17.2 GiB
> loose set, exceeded the 12 GiB cgroup limit and the kernel's memcg OOM killer killed it.
> The needle-visible result was `exit_code=-1`. The host was never out of memory — the
> *dispatch scope* was.

Two co-requisite defects, neither sufficient alone:

1. **Unbounded accumulation** — `.beads/` was tracked and nothing gated what entered history.
2. **Unbounded operation** — no memory ceiling on git's packing path, inside a finite scope.

Both are now fixed (§7); fixing either alone would have left the crash reachable.

**Category: INFRASTRUCTURE** — a kernel-enforced cgroup memory boundary, not a domain-check
defect, not a service failure, not a workflow failure. The contribution of *config* is real
but secondary: the missing `pack.windowMemory` bound and the missing `.beads/` gitignore were
workspace-configuration gaps that set up the infrastructure failure; the killing mechanism
itself was the memcg OOM. See §6 for the full category argument.

**Confidence:** trigger = **HIGH**; the kill being a memcg OOM = **MEDIUM-HIGH** (the Aug-12
kernel record is unrecoverable — system journald begins 2026-08-15; the mechanism is instead
**live-reproduced at reduced scale, with kernel OOM attribution, on 2026-09-06** — §5.3).

---

## 2. Failure mode

| Property | Value |
|---|---|
| Observable failure | needle `exit_code=-1`, `outcome=Crash(-1)`, `signal_code=-1` — **a sentinel for "killed, signal not recorded"**, never a signal number and never SIGHUP (that would surface as 129) |
| Failure mode | **Resource exhaustion: cgroup memcg OOM** during a git packing operation |
| Death point | Immediately after the attempt's `git push origin main` tool call — 49 of 50 crash transcripts truncate there; no crash attempt ever received a git result back |
| Death count | **50 consecutive** agent dispatches, one every mean 192 s / median 156 s (min 75 s, max 577 s) across a 156.8-minute window *(re-computed first-hand from `sessions-index.tsv`, §4)* |
| Kill-to-classification latency | median 13.7 s, max 23.3 s, min 0.19 s (n=50) — abrupt-truncation shape, no error text |
| What the agent saw | Nothing. No panic, no stack trace, no OOM message. The transcript truncation point *is* the death record |
| Domain-check involvement | **None** — no domain-check binary was built or executed during the storm; the dying process was `git` |

Adjacent outcomes in the same window (not part of the failure mode): 1 exit-1 failure
(ended at `git add -A && git commit`), 4 command timeouts (exit 124), 1 exit-0 run that
still failed to close the bead (`bead.orphaned`).

---

## 3. Mechanism — chain of events

1. **Accumulation (pre-Aug-12).** The bead-forge-era workflow repeatedly committed ~237 MB
   `.beads/*.jsonl` snapshots: `.beads/` not gitignored, no pre-commit size gate →
   ~18 GB `.git`, 17.2 GiB loose vs 9.6 MiB packed (inverted ≈1,800:1).
2. **Bound (constant).** Every dispatch runs under `needle.slice` with `MemoryMax=12GiB`
   (parent slice 24 GiB high / 32 GiB max; host 62 GiB).
3. **Trigger (each attempt).** Attempt commits another snapshot, then runs
   `git push origin main` → `git send-pack` → **`git pack-objects`**, which must walk the
   whole loose set to build the pack → working set exceeds 12 GiB → **memcg OOM SIGKILL**.
   The agent never sees an error; needle records the sentinel −1 and releases the bead.
4. **Amplification (the loop).** Zero-backoff re-dispatch re-enters step 3 ~3 minutes later.
   The kill leaves the loose set untouched and each attempt's snapshot commit grows it —
   **the storm grew the exact store its deaths were caused by.** Nothing opposed it. This is
   why the loop is 50 deep rather than 1: no retry could shrink the trigger, only grow it.
5. **Terminus.** The Aug-13/14 cleanup removed the bloat (18 GB → ~91–94 MB); the crash
   class **stopped instantly** and has not returned. The task then completed and the bead
   closed 2026-08-17.

The same mechanism was later kernel-proven twice on this host: bf-4x12ec (2026-08-14, gc-side
`git gc --aggressive`) and bf-198ne (2026-08-16, push-side — `git` at anon-RSS ≈ 11.7 GiB,
`oom-kill:constraint=CONSTRAINT_MEMCG … Killed process 3322486 (git)`). bf-198ne is the same
kill later recorded, not a "later variant": the Aug-12 storm was itself uniformly push-side.

---

## 4. Evidence

### 4.1 Primary sources (all committed)

| Artifact | What it establishes |
|---|---|
| [`bf-4yjq/raw-logs/`](bf-4yjq/raw-logs/README.md) — 56 session transcripts (14.76 MB), needle event log (1,071 records), worker-log extract (225 lines), `sessions-index.tsv`, `MANIFEST.sha256` | Per-attempt outcomes, death points, byte-pinned raw telemetry |
| [`bf-4yjq/error-analysis.md`](bf-4yjq/error-analysis.md) (305 L) | Complete error inventory; **verified absence** of stack traces, core dumps, stderr, kernel records |
| [`bf-4yjq/version-info.md`](bf-4yjq/version-info.md) (188 L) | HEAD `199b70c` (305 ahead), origin `63ba024`, Claude Code 2.1.227 / glm-4.7, needle 0.3.1-era |
| [`bf-4yjq/infrastructure-status.md`](bf-4yjq/infrastructure-status.md) (292 L) | 12 GiB scope bound; no host telemetry existed for Aug-12; CPU saturated all day (storm in the calmest stretch); disk/gateway/SIGHUP/reboot each excluded |
| [`bf-4yjq-cleanup-verification.md`](../crashes/bf-4yjq-cleanup-verification.md) | The trigger was removed and stayed removed |

### 4.2 Re-verified live for this RCA (2026-09-06, working repo)

Executed by this dispatch — not quoted from an earlier record:

| Check | Result |
|---|---|
| `sessions-index.tsv` outcome tally | **50 crash / 4 timeout / 1 success / 1 failure** (56 rows) — byte-exact with the context doc |
| Death window | first `2026-08-12T17:53:53.875682Z`, last `2026-08-12T20:30:38.310348Z` (worker-log `exit_code=-1` lines) |
| Interval statistics (computed from the index) | mean **192 s**, median **156 s**, min 75 s, max 577 s over 49 intervals — reconciles the context doc's "~3.1 min" (mean) with the classification record's "median 156 s" |
| Kill gap (transcript-end → needle classification) | median 13.7 s, max 23.3 s, min 0.19 s (n=50) |
| Worker-log extract | 50 ERROR lines / 50 `exit_code=-1` / 225 total |
| Current repo health | `.git` **101 M**, 3 packs totalling **99.13 MiB**, **0 garbage**, count 21 loose (136 KiB); **0 tracked `.beads/` files** |
| `setup-git-gc-config.sh --verify` | exit 0 — effective `pack.windowMemory`/`deltaCacheSize`/`threads=1` bounds present |
| `crash-pattern-detection.sh --quiet` | exit 0 |

*Expected drift, not an error:* the context doc's `[LIVE]` snapshot read 103 M / a single
90.93 MiB pack; today's 101 M across 3 packs reflects doc commits landing after that snapshot
and before the next incremental gc. Both are healthy (limits: <500 MB total, <100 loose MB).

### 4.3 The negative evidence is load-bearing

- **No stack trace / panic / OOM message exists anywhere** — positive search of all
  14.76 MB of transcripts, the structured event log, and the worker log; zero of 1,071
  structured records carries an `error` field. An application-level crash leaves text; an
  abrupt cgroup kill does not.
- **No kernel record for Aug-12 exists** (journald starts 2026-08-15 19:46 EDT; core dumps
  2026-08-17) — so the killing signal is never *named* for this date. Hence MEDIUM-HIGH, not
  proven, for the OOM attribution on the original event — and hence the live reproduction in
  §5.3.
- **No host-level OOM anywhere** — all 458 journalled OOM records on this host are
  `CONSTRAINT_MEMCG`. The 62 GiB host was never the exhausted resource.

---

## 5. Reproducible or intermittent?

**Three-part answer — deterministic within the condition, not re-runnable as an event,
re-creatable as a condition:**

### 5.1 Within the bloated window: deterministic, not intermittent

50 of 50 crash attempts died at the same step on the same command, at a regular cadence
(mean 192 s set by task duration + re-dispatch), across sessions of varying length and
content. Nothing about the failure was probabilistic — it fired on every attempt that
reached the push. The one attempt that failed differently (exit 1 at `git add`) never
reached the trigger.

### 5.2 The event itself: not re-runnable

The crash-time tree was rewritten by the Aug-16 squash (c27899f) that purged the 237 MB
blobs; every crash-era SHA is absent from today's object store, and the Aug-12 kernel/journal
records died with the Aug-14 16:39 reboot. This crash cannot be replayed from its artifacts.

### 5.3 The condition: re-created live at 1/17th scale, with kernel OOM attribution — 2026-09-06

`scripts/test-bf-4yjq-crash-condition.sh` (spec:
`docs/crash-investigations/bf-4yjq-crash-workload-test-spec-domchk-b90505ad-2026-09-06.md`)
rebuilds the scaling relation — pack-objects' peak RSS scales with the loose set — at
1/17th scale (16 × 64 MiB incompressible blobs = 1 GiB loose) inside a 512 MiB scope. Run
for this RCA, **6/6 assertions passed**:

- **A — the crash re-created:** bare `git gc` over 1 GiB loose inside `MemoryMax=512M`
  died, loose set intact (48 objects), with OOM attribution in both journals. The kernel
  line, captured live:

  ```
  Sep 06 20:34:28 lab kernel: oom-kill:constraint=CONSTRAINT_MEMCG,…,
  oom_memcg=…/app.slice/bf4yjq-crash-a-3822933.scope,task=git,pid=3824966,uid=1001
  ```

  `task=git` — the killed process is git, killed by its memcg, exactly the inferred Aug-12
  mechanism.
- **B — the deployed bound mitigates:** the *same repository that just died* completed
  `git gc` (exit 0) once `pack.windowMemory`/`deltaCacheSize`/`threads=1` were set.
- **C — the packed-store state is safe:** with the store packed and bounds unset, ordinary
  agent operations (`status`, `log`, `fsck --full`) all completed inside 512 M.

One presentational nuance the reproduction adds: the scope surfaced **exit 143 (SIGTERM)**
as systemd failed the unit with `result 'oom-kill'`, while the kernel record shows the OOM
kill landed on `task=git`. The same event class can therefore present different observable
exit codes depending on which process absorbs the signal and how systemd tears the scope
down — which is precisely why needle's sentinel is an unrecorded-signal `-1` rather than a
signal number, and why OOM attribution must come from the journal, never from the exit code.

### 5.4 On today's repo: not reproducible — and guarded

The trigger is gone (101 MB store, 0 tracked `.beads/` files) and the operation is bounded
(`--verify` exit 0, worst case ≈3 GiB per pack run inside the 12 GiB scope). Reproduction
**B** demonstrates the bound neutralizing the exact condition that killed **A**; the repo's
own history since Aug-14 — zero exit-−1 events in live agent work — demonstrates it holding.

---

## 6. Category: code vs config vs infrastructure

| Candidate | Verdict |
|---|---|
| **Code defect** | **Excluded.** No domain-check binary was built or executed during the storm (last real code change `63ba024`, 3 days prior; the only unpushed Go deltas were `199b70c`+`70b8aab`, never run). The killed process was `git` plumbing under the dispatch scope. No investigation of this workspace has ever found a domain-check code defect. |
| **Service failure** | **Excluded.** All 56 attempts sustained multi-turn LLM work right up to death; zero message-shaped 5xx/gateway hits in 14.76 MB of transcripts or the full day's needle telemetry (naive `grep 503\|502` returns ~700 false positives — match message shapes); gateway `/health` → `ok`. |
| **Workflow failure** | **Excluded.** Each attempt was actively and correctly executing the bead's git task when killed — not stalled, not blocked (the older "BLOCKED" claim comes from the post-storm orphan check at 21:14:59Z, not crash time). The 3 auto-splits and 50 alert beads were downstream consequences. |
| **False positive** | **Excluded.** The bead was genuinely open and the work genuinely unfinished at every death. |
| **Config (contributing, remediated)** | The missing `.beads/` gitignore, the absent pre-commit size gate, and the absent `pack.windowMemory` bound were workspace-configuration gaps. They are *why* the condition existed, and both layers are now mechanically enforced — but they are predisposition, not the killing mechanism. |
| **Infrastructure** | ✅ **The category.** A kernel memcg OOM inside the 12 GiB dispatch scope, triggered by an 18 GB store. Under `docs/crash-response-guide.md`'s four-way taxonomy: `exit −1` + fixed-cadence re-dispatch deaths + `.git` ≫ 5 GB → INFRASTRUCTURE (repository bloat is an explicit member of that row). Independently recorded in [`bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md`](../crash-investigations/bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md). |

---

## 7. What keeps it fixed

| Layer | Control | Live status (2026-09-06) |
|---|---|---|
| A — stop accumulation | Whole-`.beads/` gitignore + repo-wide `*.db`/`*.jsonl` | ✅ 0 tracked `.beads/` files; store 101 M / 0 garbage |
| A | 10 MB pre-commit gate (`.git/hooks/pre-commit`, per-clone) | ✅ active in this clone |
| A | Daily repo-health timer (>500 MB loose alert) | ✅ among the six `domain-check-*` user timers |
| B — bound the operation | `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1`, repo-local + box-global — covers gc **and** the push pack path | ✅ `--verify` exit 0, worst case ≈3 GiB per pack run |
| B | `safe-git-gc.sh` as the sanctioned cleanup path | ✅ deployed since 2026-09-02 |
| Detection | Crash-pattern detector, surge threshold 3/5 min | ✅ exit 0 |
| Verification | `scripts/test-bf-4yjq-crash-condition.sh` | ✅ 6/6 re-run for this RCA |

Open items are owned elsewhere and are **not** re-implemented here (context doc §9.2):
uncommitted detector improvements needing a commit owner; the bf-4yjq alert backlog
(132 title-matching beads — bulk-close those whose targets are resolved); stale docs still
citing "9 crashes / ~17-minute intervals"; evidence-retention gaps G-8/G-10 (kernel OOM +
scope-level telemetry) and G-11 (retry backoff) from
[`crash-prevention-requirements.md`](../crash-prevention-requirements.md).

**Triage rule this RCA certifies:** a spike of sub-3-minute exit-−1 re-dispatch deaths is
the signature of an environmental kill, not a code path — triage at the environment level
(repo size, scope memory, load) before any per-bead debugging. This crash is not an event
you can re-run; it is a condition you can re-create, and the condition is what the guards
police.

---

## 8. Provenance — re-derive everything

```bash
cd /home/coding/domain-check

# outcome tally + death window (§4.2)
cut -f2 docs/crash/bf-4yjq/raw-logs/sessions-index.tsv | tail -n +2 | sort | uniq -c
grep -c "exit_code=-1" docs/crash/bf-4yjq/raw-logs/needle-worker-log-bf-4yjq-slot2.log
grep "exit_code=-1" docs/crash/bf-4yjq/raw-logs/needle-worker-log-bf-4yjq-slot2.log | head -1   # first death
grep "exit_code=-1" docs/crash/bf-4yjq/raw-logs/needle-worker-log-bf-4yjq-slot2.log | tail -1   # last death

# current repo health (§4.2)
du -sh .git && git count-objects -vH && git ls-files .beads | wc -l
./scripts/setup-git-gc-config.sh --verify
./scripts/crash-pattern-detection.sh --quiet

# live reproduction incl. kernel OOM attribution (§5.3)
./scripts/test-bf-4yjq-crash-condition.sh
journalctl -k --no-pager --since "-30 min" | grep "oom-kill:constraint=CONSTRAINT_MEMCG" | grep bf4yjq
```

*Root cause analysis for dispatch domchk-b1177594, 2026-09-06. Every figure marked
"re-verified" or "computed" in §4.2/§5.3 was executed by this dispatch on the working repo
and the reproduction harness; everything else is carried forward from
[`bf-4yjq-crash-context.md`](bf-4yjq-crash-context.md) and its four committed gather layers,
which this document does not duplicate.*
