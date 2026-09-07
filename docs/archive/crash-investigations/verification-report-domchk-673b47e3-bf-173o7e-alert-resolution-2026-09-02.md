# Verification Report: domchk-673b47e3 — bf-173o7e crash documentation alert resolved

| Field | Value |
|---|---|
| **Dispatch bead** | `domchk-673b47e3` — "Document crash findings and resolution" (created 2026-08-26T22:41:40Z; this dispatch) |
| **Subject bead** | `bf-173o7e` — "Execute git gc --aggressive with pruning" (closed 2026-08-17T17:15:23Z, work complete) |
| **Disposition** | **RESOLVED — INFRASTRUCTURE** (kernel memcg OOM); original work **complete**; **no retry needed** |
| **Root cause** | Kernel memory-cgroup OOM SIGKILL of `git gc --aggressive --prune=now` inside needle's 12 GiB dispatch scope → needle `exit −1` sentinel. Full mechanism: [`crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md`](crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md) |
| **Consolidates** | This bead's task was already discharged by the committed chain: `domchk-b7d85b1c` → `docs/verification-report-domchk-b7d85b1c-bf-4iviwf-alert-resolution-2026-09-02.md` (5d501a8), `domchk-a0f2c805` (6b4aa4c), `domchk-536862b8` (d283576), `domchk-858fecb1` (db1acb3), `domchk-2e371a2c` (07ab240), `domchk-4c4a6163` (b1ae579) |

## 1. Why this bead existed

`domchk-673b47e3` was created 2026-08-26, twelve days after the crash, asking
to "document the complete investigation findings and resolution for the agent
crash on bead bf-173o7e." That documentation was produced across the
2026-09-02 investigation cycle and committed (see **Consolidates** above); the
dispatch is a stale re-fire of resolved work, handled here by verifying the
resolution still holds and recording this bead's own verification report.

## 2. Crash summary (acceptance criterion 1)

- **Agent:** claude-code-glm-4.7 (lab, domain-check worker)
- **Exit code:** −1 — the needle sentinel `code().unwrap_or(-1)` for a dispatch
  whose process died without recording an exit code, **not a signal number**.
  The underlying death was SIGKILL from the kernel's memory cgroup controller.
- **Timestamp in the alert (`2026-08-14T14:06:16`):** the
  `HANDLING_RELEASE_DONE` release heartbeat, ~7.7 s after the real
  `agent.completed` kill at **14:06:08.828Z — 69.7 s into storm dispatch #40**
  (the "#41" label in older notes is an off-by-one; corrected in 6b4aa4c).
  Standing rule reaffirmed: alert timestamps are never kill times.
- **Task at death:** `git gc --aggressive --prune=now` over **17.20 GiB of
  loose objects** (18 GB `.git`, 99 %+ loose) inside needle's transient
  `run-*.scope` with `MemoryMax=12 GiB`. Delta-chain building across the whole
  object set exhausted the **scope's** budget in tens of seconds; the kernel
  killed the highest-badness task. Host RAM had ~45 GiB free — a scope-budget
  kill, not host exhaustion.
- **Scale:** bf-173o7e ran its own **132-dispatch storm** (12:58:58–23:25:35Z
  Aug-14): **129 × exit −1** (21.6–216.6 s each), 1 × exit 124 (607.6 s, the
  600 s cap), **1 × exit 0**. The bead was itself a duplicate of sibling gc
  bead bf-4x12ec, whose 53-attempt storm ended 13 s earlier.

## 3. GC operation status (acceptance criterion 2)

**Did not complete during the killed attempts; completed later — YES,
ultimately successful.**

- **At this alert's kill (14:06Z):** mid-gc, in the pre-pack-write phase. No
  gc had completed before or within that attempt; the 129 flat kill durations
  over 10.5 h prove the object set never shrank between attempts (d283576).
- **Completion:** the storm's final **exit-0 attempt at 23:25:35Z Aug-14**
  (40.1 s) is the first successful in-attempt run; the fully packed state was
  consolidated by **Aug-17** (0 loose / 7,765 packed / 445 MB `.git`, per
  bf-173o7e's own closure note), and to today's single pack on Sep-2.
- **Verified fresh 2026-09-02 for this report:** `git count-objects -vH` →
  148 loose / 1.16 MiB, 1 pack / **90.18 MiB**, `.git` **94 MB**; `git fsck`
  clean (per db1acb3). No bloat regression.

## 4. False-positive assessment (acceptance criterion 3)

**The kill was real; the alert was a false alarm only at the alerting level.**

- 129 SIGKILLs genuinely happened — this is a true **INFRASTRUCTURE** event,
  not a phantom crash and not a domain-check code defect.
- But the alert's implied conclusion ("task failed / needs retry") was wrong:
  the work the bead asked for **did complete**, the bead was **correctly
  closed 2026-08-17**, and no retry was ever needed. The gc did **not**
  "succeed despite this crash" — it succeeded ~9.3 h *after* this dispatch's
  kill, via the storm's final exit-0 attempt and subsequent consolidation.
- The alert itself fired 12 days post-crash against a closed bead — the
  stale-alert generation problem since fixed by closed-bead filtering,
  duplicate detection, and cooldown in `scripts/crash-alert-manager.sh`.

## 5. Subject bead notes (acceptance criterion 4)

`bf-173o7e` notes already carried the **FINAL DETERMINATION** (added by
`domchk-b7d85b1c`, 2026-09-02): memcg OOM, 129× exit −1 + 1× 124 + 1× 0,
INFRASTRUCTURE, work complete, no retry, bead correctly closed. This dispatch
appended one line recording that duplicate alert bead `domchk-673b47e3` was
likewise verified and resolved, with a pointer to this report.

## 6. Resolution (acceptance criterion 5)

`domchk-673b47e3` **closed as resolved** with the conclusion above. No split
emitted (the auto-split premise does not apply — the underlying work and its
documentation both exist); no new investigation opened.

## 7. Lessons learned (recurrence prevention, already in force)

| Lesson | Artifact |
|---|---|
| Never run bare `git gc --aggressive`; use staged, memory-limited, resumable gc | `scripts/safe-git-gc.sh` + daily/weekly systemd timers |
| Bound pack memory at the config layer (`pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` → ≈3 GiB worst case), verified exit-0 today | `scripts/setup-git-gc-config.sh --verify`, `scripts/test-gc-memory-bounds.sh` |
| Treat alert timestamps as heartbeats; re-derive kills from `agent.completed` in the primary needle log | Standing rule, [`agent-crash-bf-173o7e-aug14-storm-root-cause`](crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md) |
| Verify target-bead state before honoring re-dispatched/split alerts on closed beads | `scripts/crash-alert-manager.sh` fixes; `docs/crash-reports/bf-173o7e.md` |
