# Verification Report: bf-4iviwf alert resolution — bf-173o7e Aug-14 crash

| Field | Value |
|---|---|
| **Alert bead** | `bf-4iviwf` — "ALERT: Agent crash on bead bf-173o7e" (created 2026-08-14T13:40:59Z, still open at time of writing) |
| **Original bead** | `bf-173o7e` — "Execute git gc --aggressive with pruning" (closed 2026-08-17T17:15:23Z, work complete) |
| **Resolving bead** | `domchk-b7d85b1c` — "Document verification and close crash alert" (this dispatch) |
| **Disposition** | **RESOLVED — INFRASTRUCTURE** (memcg OOM); original work **complete**; **no retry needed** |
| **Root cause** | Kernel memory-cgroup OOM SIGKILL inside needle's 12 GiB dispatch scope → needle `exit −1` sentinel. Full mechanism: [`crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md`](crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md) |
| **Supersedes** | The exit-code claim in [`verification-report-bf-4iviwf-duplicate-alert-resolved-bf-173o7e.md`](verification-report-bf-4iviwf-duplicate-alert-resolved-bf-173o7e.md) (2026-08-26) — see §4 |

---

## 1. What this alert was

`bf-4iviwf` is one of the ~30 alert beads generated during **bf-173o7e's
Aug-14 dispatch storm**. Its body reports "exit code −1 (signal −1)" at
`2026-08-14T13:40:59.281Z`.

**Timestamp re-derivation (done for this report, per the standing rule that
alert timestamps must never be treated as kill times):** no event in any
Aug-14 needle log serializes `13:40:59.281116876` exactly, so this value is
the alert pipeline's own creation clock, not a copied heartbeat. The nearest
preceding `agent.completed` for bf-173o7e is **13:40:37.865Z, exit −1,
49.4 s run** — 21.4 s before the alert timestamp, with `heartbeat.emitted`
events firing 13:40:37–13:40:52. The alert therefore corresponds to one of
the **35 hour-13Z kills** of the storm, and the kill itself is real.

## 2. Crash summary (consolidated from the primary-source analyses)

- **Task:** `git gc --aggressive --prune=now` over **17.20 GiB of loose
  objects** (18 GB `.git`, 99 %+ loose vs 9.6 MiB packed; disk 94 % full;
  memory available < 2 GB during gc runs). Pure repository maintenance —
  zero code changes; no files attributable to the bead were modified.
- **Storm:** **131 attempts** between 12:58:58Z and 23:25:35Z —
  **129 × exit −1** (21.6–216.6 s each), 1 × exit 124 (607.6 s, the 600 s
  cap), **1 × exit 0** (23:25:35Z, 40.1 s) which completed the gc — then the
  bead was `bead.orphaned`-on-success (known worker-side defect) rather than
  closed. bf-173o7e was itself a duplicate of sibling gc bead bf-4x12ec,
  whose 53-attempt storm had ended 13 s earlier.
- **Mechanism:** `git gc --aggressive` builds delta chains across the whole
  object set in memory before writing pack bytes; inside needle's transient
  `run-*.scope` (`MemoryMax=12 GiB`, `oom_score_adj=200`) that exhausts the
  **scope's** memory budget in tens of seconds and the kernel SIGKILLs the
  highest-badness task — usually the agent itself, hence needle's
  `exit −1` sentinel (not a signal number). Corroborated by **257
  `task=git` `CONSTRAINT_MEMCG` kernel kills on Aug-16**, anon-rss median
  12,301,364 kB hugging the 12 GiB cap. Host RAM was ~45 GiB free mid-storm:
  a scope-budget kill, not host exhaustion.
- **Outcome:** the gc the bead asked for **succeeded** — 18 GB → 445 MB
  (97.5 % reduction), later consolidated to today's single 90.18 MiB pack.

## 3. Fixes implemented since (recurrence prevention)

| Fix | Artifact | Status |
|---|---|---|
| Bare `git gc --aggressive` banned; staged, memory-limited, checkpoint/resumable gc | `scripts/safe-git-gc.sh` (+ `--check-only`, `--resume`, monitor) | In use; daily incremental + weekly full gc timers |
| Dispatch-scope memcg headroom check before memory-heavy work | cgroup memory guard, commit `f0a7a81` (bead `domchk-c67caf80`, closed) | Merged |
| Same-cause kill-loop breaker | `scripts/crash-circuit-breaker.sh` | In place |
| Alert-pipeline fixes: closed-bead filtering, duplicate detection, post-completion awareness, 5-min cooldown, classification | `scripts/crash-alert-manager.sh` (+ `crash-classifier.sh`, `alert-deduplication.sh`; test suite 12/12) | Implemented 2026-09-02 |
| Continuous monitoring | 6 systemd user timers — monitoring 10 min, resource 5 min, service 2 min, repo-health daily, gc daily, full-gc weekly | **Verified firing 2026-09-02** (`systemctl --user list-timers 'domain-check-*'`) |
| Pre-close work verification (distinguishes post-completion crashes from mid-task ones) | `scripts/verify-work-completion.sh` | In use |

## 4. Correction to the 2026-08-26 verification report for this alert

The earlier report
([`verification-report-bf-4iviwf-duplicate-alert-resolved-bf-173o7e.md`](verification-report-bf-4iviwf-duplicate-alert-resolved-bf-173o7e.md))
concluded FALSE POSITIVE largely because "the actual exit code was 1
(error_max_turns), not −1". That claim is **superseded**: it conflated the
**Aug-17** post-completion bead-close exhaustion (exit 1, `error_max_turns` —
the only surviving trace) with the **Aug-14** storm kill that generated this
alert. The Aug-14 event was a **real** memcg-OOM kill, correctly reported as
exit −1 by the alert. What does survive from the Aug-26 report is its
practical conclusion — the original work was complete and no retry was
needed — which this report confirms with two more days of evidence.

## 5. Recurrence evidence (live-verified 2026-09-02)

**Repository health — the actual fix target:**

```
count: 93 (752 KiB loose)   in-pack: 10,478   packs: 1   size-pack: 90.18 MiB
.git 93M   ·   93G disk free   ·   50Gi RAM available
```

vs 17.20 GiB loose / 18 GB `.git` / 29 GB disk free at crash time.

**Kernel:** since Aug-26 there are **8 memcg OOM kills total, zero of them
`git`** (the recent ones are tiny `bash` test-harness processes, anon-rss
18–63 MB, killed inside tight scopes). For comparison, Aug-16 alone saw 257
git memcg kills. The crash mechanism has not recurred.

**Fleet:** since Aug-26, worker logs record **6,691 exit-0 completions vs 7
exit −1**. None involves a domain-check bead after Aug-26T22:54 (itself the
handling of alert bead bf-57nao4); the Sep-1/2 exit −1s are drawrace and
pdftract beads on other workspaces.

**Monitoring path proven live:** the monitors fired on real pressure today
(CPU load 20.24 and memory-pressure spikes logged CRITICAL in
`.beads/logs/resource-monitor.log` on 2026-09-02) — i.e. the detection that
was silent during the August storms now works, and no agent memcg kill
resulted from those spikes.

**Honest caveats:** (1) the bead store still carries ~276 open ALERT beads
from the Jul–Aug backlog — closing them is incremental (this task closes one);
(2) the box still sees transient pressure spikes, which the monitors now
surface; (3) several mitigation scripts carry uncommitted 2026-09-02 edits in
the working tree (out of scope here).

## 6. Lessons learned

1. **Alert timestamps are bookkeeping, not kill times.** They can be release
   heartbeats or the alert pipeline's own clock (both seen in this storm).
   Always re-derive the kill from `agent.completed` in the primary needle log.
2. **`exit −1` is a sentinel**, not a signal number — decode it as "died on a
   signal needle did not send" before reasoning about signals.
3. **Scope budget ≠ host memory.** memcg OOM kills happen while tens of GB of
   host RAM are free. Check the dispatch scope's own headroom (hence the
   cgroup memory guard) before memory-heavy work.
4. **Never prescribe bare `git gc --aggressive` inside a bounded dispatch
   scope** on a bloated object store — it is intrinsically scope-fatal. Use
   `scripts/safe-git-gc.sh`, and keep loose-object bloat from forming at all
   (`.beads/` is gitignored here; repo-health monitor watches the ratio).
5. **SIGKILLed dispatches write no traces** — the event log is the primary
   source for storm reconstruction; absence of a trace is not absence of work.
6. **Duplicate/stale alerts amplify one real event into dozens of beads**
   (30+ for this storm, some dispatched 12 days later). Closed-bead
   filtering + dedup + cooldown now prevent generation; the backlog still has
   to be worked down bead by bead.
7. **Orphaned-on-success** (success releases the bead instead of closing it)
   turns completed work into retry storms — verify work completion
   (`scripts/verify-work-completion.sh`) before closing, and treat
   "crashed" beads with committed work as post-completion failures first.
8. **Monitoring that only runs when someone remembers is not monitoring** —
   the systemd-timer setup (and the `daemon-reload` gotcha after editing
   units) is what turned silent OOM loops into same-minute CRITICAL logs.

## 7. Acceptance criteria mapping

| Criterion | Result |
|---|---|
| Verification report summarizing crash, root cause, fix | This document |
| bf-173o7e updated / marked appropriately | Closed 2026-08-17 (work complete); notes updated 2026-09-02 with the final memcg-OOM determination — no retry |
| Fix prevents recurrence (testing/monitoring) | §5 — zero git memcg kills since Aug-26, repo at 90 MiB single pack, 6 timers firing, monitors proven live |
| Alert bead bf-4iviwf closed with resolution summary | Closed 2026-09-02 citing this report (INFRASTRUCTURE / work-complete / no-retry) |
| Lessons learned documented | §6 |

---

*Determined 2026-09-02 · bead `domchk-b7d85b1c` · primary sources: storm RCA,
original-bead context review (`domchk-8304c1c0`), live kernel/needle-log and
systemd verification on this date.*
