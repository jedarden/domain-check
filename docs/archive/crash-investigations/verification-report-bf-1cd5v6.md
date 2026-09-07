# Verification Report: Crash Alert bf-1cd5v6 — Duplicate False Positive

**Alert bead:** bf-1cd5v6 — "ALERT: Agent crash on bead bf-173o7e"
**Target bead:** bf-173o7e — "Execute git gc --aggressive with pruning"
**Alert created:** 2026-08-14T21:04:15.514673885Z
**Verification date:** 2026-09-02 (dispatch domchk-bfee6600)
**Classification:** ✅ FALSE POSITIVE — duplicate alert targeting an already-resolved incident
**Alert status:** OPEN — closure is owned by sibling bead domchk-f78f9e84 (see "Dispatch structure")

> **Revision note (2026-09-02).** This report supersedes the 2026-08-26 revision
> (commits `91a7719` and `3a0a4c4`) on root-cause attribution only. The
> false-positive verdict is unchanged and is restated with stronger evidence.
> The Aug-26 revision claimed the "actual" crash was exit code 1
> (`error_max_turns`) and a workflow issue. That attribution was **superseded by
> the 2026-09-02 final determination** on bf-173o7e (domchk-b7d85b1c, commit
> `5d501a8`; storm counts refined by `07ab240` and `6b4aa4c`): the Aug-14 agent
> deaths really were `exit -1` kernel memcg OOM SIGKILLs. The max_turns exit-1
> event was a **separate, later, post-completion** bead-close failure. The two
> events were conflated in most pre-Sept-02 reports — including the prior
> revision of this file. Sections below document both events distinctly, with
> timestamps.

## Executive Summary

bf-1cd5v6 is one of **129** auto-generated alert beads, all titled "ALERT:
Agent crash on bead bf-173o7e", all created on 2026-08-14 during the
crash-storm window (12:59–23:24 UTC). The bead it alerts on — bf-173o7e — was
**CLOSED on 2026-08-17** with a successful-outcome close reason, has since been
re-verified resolved twice (2026-09-02, domchk-b7d85b1c and domchk-673b47e3),
and the repository it references is healthy (fsck clean, single 90.18 MiB
pack). No action is required; the alert is a stale duplicate re-flagging a
resolved incident.

## Alert Identity (verified first-hand, 2026-09-02)

| Field | Value | Source |
|---|---|---|
| ID / title | bf-1cd5v6 / "ALERT: Agent crash on bead bf-173o7e" | `bead show bf-1cd5v6` |
| Status | Open (revision 16) | `bead show bf-1cd5v6` |
| Created | 2026-08-14T21:04:15.514673885Z | `bead show bf-1cd5v6` |
| Labels | `alert`, `crash`, `failure-count:1`, `signal--1`, `split-child`, `umbrella`, `verification-failed` | bead store |
| Alert body claim | "Exit code: -1 (signal -1)" at 2026-08-14T21:04:15.508Z | alert description |
| Duplicate family | 129 beads with the identical title, all created 2026-08-14 (12:xx–23:xx UTC); as of 2026-09-02: 75 closed, 34 in_progress, 18 open, 2 deferred | bead store query |

## Evidence Compiled from the Split-Child Beads

This dispatch is child 3 of an auto-split of bf-1cd5v6 created
2026-08-26T23:03Z. Children 1 and 2 are closed and their findings are folded
in here.

### Child 1 — domchk-e2a694b6 "Verify original crash bf-173o7e resolution status" (CLOSED)

- bf-173o7e is **CLOSED**: final closure **2026-08-17T17:12:09Z**, close
  reason: *"Git gc completed successfully - 17.20GB loose objects packed into
  444MB pack file, repository valid"*. (Re-verified first-hand on 2026-09-02
  from `.beads/checkpoint/forensic.jsonl`; an earlier same-day closure at
  16:21:24Z recorded the 745.67 → 444.23 MiB pack transition, 30 → 0 loose.)
- Final determination note on bf-173o7e (domchk-b7d85b1c, 2026-09-02):
  Aug-14 exit −1 storm was **kernel memcg OOM inside the 12 GiB dispatch
  scope**; the gc actually completed on the sole exit-0 run at 23:25:35Z Aug-14;
  INFRASTRUCTURE, work complete, **no retry needed**.
- Fresh health at child-1 verification: `git fsck --full` exit 0, single pack
  90.18 MiB, .git 93–94 MB, loose objects ≈1.1 MiB (normal doc-commit churn),
  gc memory bounds verified via `scripts/setup-git-gc-config.sh --verify`
  (worst-case pack ≈3072 MiB, within the 12 GiB scope ceiling).
- Child-1 verdict: stale auto-split child hitting an already-resolved incident;
  verification-only, no file changes warranted.

### Child 2 — domchk-be0f34f7 "Investigate crash alert bf-1cd5v6 validity and classification" (CLOSED)

- bf-1cd5v6 is one of the 129 duplicate alerts for bf-173o7e, created
  2026-08-14T21:04:15Z — 19 days before the child-2 investigation, with no new
  crash since.
- **Exit-code discrepancy resolved:** the alert's "signal −1" claim **matches**
  the real crash — 129 × exit −1 within the 132-dispatch Aug-14 storm (kernel
  memcg OOM, HIGH confidence, commit `07ab240`). The task premise "actual exit
  code 1 (max_turns)" traces to earlier reports (`f57c556`, `b54b0a9`,
  `df98bdf`, and the 2026-08-26 revision of this very report) that **conflated
  two distinct events** (see the table below). Commit `9992c8e`
  (`docs/research/git-gc-oom-crash-analysis.md`) explicitly disambiguates them.
- The alert timestamp is a `HANDLING_RELEASE_DONE` release heartbeat seconds
  after the real `agent.completed` kill — consistent with the whole storm; it
  is not an independent death time.
- Verdict: FALSE POSITIVE (duplicate of resolved crash).

## Key "Discrepancies" — Corrected Reading, With Timestamps

The dispatch task statement asked this report to document three discrepancies.
Each is resolved below against the final determination; the first two turn out
to be documentation artifacts rather than genuine misclassifications.

| # | Task premise | What the evidence actually shows | Consequence |
|---|---|---|---|
| 1 | Alert claims signal −1 vs "actual" exit code 1 (max_turns) | **Both are real, and they are different events.** Event A — 2026-08-14, 12:59–23:25Z: 132 dispatches / 131 completions; 129 × exit −1 (kernel memcg OOM SIGKILL), 1 × exit 124 (600 s cap), 1 × exit 0 at **23:25:35Z** which completed the gc. Event B — **2026-08-17T17:06Z**: a single post-completion `error_max_turns` bead-close failure (exit 1), administrative only, after the work was done. | The alert's signal −1 claim is **accurate** for Event A. The exit-1 claim belongs to Event B, two days later. Pre-Sept-02 reports (including the prior revision of this file) merged the two; commits `07ab240`, `9992c8e`, `5d501a8` supersede that reading. |
| 2 | Alert implies gc failure vs "actual" workflow issue | The gc **did not fail** in either event: it completed successfully (Aug-14 23:25:35Z exit-0 run; final close reason 2026-08-17T17:12:09Z: *"Git gc completed successfully - 17.20GB loose objects packed into 444MB pack file, repository valid"*). Event B was an administrative close-loop failure, not a task failure. | Correct conclusion, corrected mechanism: the incident was an **infrastructure** event (memcg OOM), with an administrative workflow hiccup afterwards — not a workflow-caused task failure. |
| 3 | Original bead CLOSED successfully | Confirmed first-hand: bf-173o7e `base_status: closed`, `closed_at: 2026-08-17T17:12:09.406429872Z`, revision 17, resolution notes updated 2026-09-02. | This is the genuine false-positive ground: **the alert targets a resolved bead**. |

### Grounds for the false-positive classification (restated)

1. **Duplicate:** 1 of 129 identical alerts for the same target bead, all
   generated inside the 2026-08-14 storm window.
2. **Target resolved:** bf-173o7e CLOSED 2026-08-17T17:12:09Z with a
   successful-outcome close reason; re-verified resolved 2026-09-02
   (domchk-b7d85b1c, domchk-673b47e3).
3. **Work complete:** the gc objective was achieved (23:25:35Z Aug-14 exit-0
   run); final determination says explicitly **no retry needed**.
4. **Timestamp artifact:** the alert's timestamp is a release heartbeat
   seconds after the real kill, so it cannot mark a new, separate crash.
5. **No new crash:** no crash of bf-173o7e has occurred since 2026-08-14; the
   alert has sat open for 19 days while the underlying incident was closed and
   twice re-verified.

## Repository Health Evidence (live, 2026-09-02)

Measured directly by this dispatch, not quoted from earlier reports:

| Check | Result |
|---|---|
| `git fsck --full` | **exit 0** (one dangling tree `b5ace847` — benign, normal churn) |
| `git count-objects -vH` | 1 pack, **90.18 MiB**, 10,478 in-pack objects; 161 loose objects, **1.26 MiB** total; 0 garbage |
| `.git` size | **94 MB** (vs the 18 GB bloat state that caused the original bf-1s6c3 incident class) |
| Disk free on `/` | **92 GB** |
| Memory available | **48 GB** |
| gc memory bounds | `scripts/setup-git-gc-config.sh --verify` exit 0 (per domchk-673b47e3): effective `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1`; worst-case pack run ≈3 GiB, inside the 12 GiB dispatch scope |

## Existing Investigation Reports Referenced

All paths verified present on 2026-09-02. The chain reads oldest → newest; the
2026-09-02 entries carry the final, corrected attribution.

| Document | Role | Status |
|---|---|---|
| `docs/crash-investigation-bf-173o7e-definitive-2026-08-25.md` | First full investigation | **Superseded** on attribution (max_turns reading) |
| `docs/research/git-gc-oom-crash-analysis.md` (commit `9992c8e`) | Explicit disambiguation of the exit −1 OOM deaths vs the exit-1 max_turns close failure | Current |
| `docs/crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md` | Root-cause determination for the full Aug-14 storm (memcg OOM) | **Final** |
| `docs/investigations/bf-173o7e-original-bead-context-domchk-8304c1c0-2026-09-02.md` | Original bead context reconstruction | Current |
| `docs/verification-report-domchk-b7d85b1c-bf-4iviwf-alert-resolution-2026-09-02.md` (commit `5d501a8`) | Final determination + first duplicate-alert resolution | **Final** |
| `docs/verification-report-domchk-673b47e3-bf-173o7e-alert-resolution-2026-09-02.md` (commit `6210dbf`) | Second duplicate-alert resolution; fresh repo re-verification; gc memory bounds | **Final** |
| `BEAD_BF-1CD5V6_VERIFICATION_REPORT.md` (commit `91a7719`) and prior revision of this file (commit `3a0a4c4`), both 2026-08-26 | Earlier bf-1cd5v6 reports | **Superseded by this revision** on attribution; false-positive verdict unchanged |

Referenced commits verified in history: `9992c8e`, `07ab240`, `5d501a8`,
`6b4aa4c`, `6210dbf`, `91a7719`, `3a0a4c4`.

## Why the Alert System Fired Anyway (pattern, not mystery)

Per the storm analysis (`07ab240` and the consolidated reports above): each
dispatch that died by memcg OOM released its bead; the release path emitted an
`HANDLING_RELEASE_DONE` heartbeat a few seconds after the real
`agent.completed` kill, and the alert generator created one alert bead per
release — 129 of them for bf-173o7e. The generator did not check whether the
target was already resolved, and re-opening cycles (bf-1cd5v6 itself has been
closed and re-opened repeatedly since 2026-08-26) keep these stale alerts
surfacing. The 2026-09-02 alert-system fixes (closed-bead filtering, duplicate
detection, alert cooldown — see `docs/crash-alert-fix-implementation-2026-09-02.md`
and CLAUDE.md "Crash Alert System") target exactly this loop.

## Impact Assessment

- **Business/system impact: none.** No data loss, no corruption, no service
  disruption; the original task succeeded and its bead is closed.
- **Cost of the alert:** investigation churn only — bf-173o7e has now been
  re-investigated 20+ times by duplicate-alert dispatches, each reaching the
  same conclusion. This dispatch exists to make the bf-1cd5v6 record accurate
  so the loop can be closed.

## Verification Methodology

1. ✅ `bead show bf-1cd5v6` and `bead show bf-173o7e` — status, timestamps,
   resolution notes (bf-173o7e: closed, revision 17, notes updated 2026-09-02).
2. ✅ Closure event read **first-hand** from `.beads/checkpoint/forensic.jsonl`
   (close reasons are invisible to `bead show`): `closed_at
   2026-08-17T17:12:09.406429872Z` with the successful-gc reason.
3. ✅ Duplicate-family census from the bead store: 129 same-title alerts,
   creation-hour histogram across the 2026-08-14 storm window.
4. ✅ Child-bead evidence compiled from closed children domchk-e2a694b6 and
   domchk-be0f34f7 (both 2026-09-02).
5. ✅ Live repository health: `git fsck --full` (exit 0), `git count-objects
   -vH`, `du -sh .git`, disk and memory checks — all green.
6. ✅ All referenced reports and commits verified to exist (table above).
7. ✅ Prior revisions of this report identified as superseded on attribution;
   this file rewritten rather than duplicated, so the canonical
   `docs/verification-report-bf-1cd5v6.md` path carries the corrected record.

## Confidence

**HIGH (false-positive classification).** Every ground rests on first-hand
2026-09-02 evidence: bead status read live, closure event read from the
forensic journal, duplicate census taken live, fsck and repo metrics run
during this dispatch, and the root-cause determination committed at
`07ab240`/`5d501a8`.

**HIGH (corrected attribution).** The memcg-OOM determination re-verified the
primary needle log live (129 × exit −1, kernel `CONSTRAINT_MEMCG` kills) and
was refined by `6b4aa4c`; the exit-1 max_turns event is independently
documented at 2026-08-17T17:06Z and is visibly a different, later event.

## Action

- **No remediation required** — target incident resolved, repository healthy.
- **This dispatch:** documentation-only; this report is the only file change.
- **Alert closure:** deliberately **not** performed here — the auto-split's
  dedicated closure child **domchk-f78f9e84** ("Close false positive alert
  bf-1cd5v6 with resolution evidence", open) owns closing bf-1cd5v6 and should
  reference this report when it does.

## Final Status

| Item | State |
|---|---|
| Alert bf-1cd5v6 | ✅ FALSE POSITIVE — duplicate of resolved crash |
| Original crash bf-173o7e | ✅ RESOLVED — closed 2026-08-17T17:12:09Z, re-verified 2026-09-02 |
| Aug-14 root cause | ✅ Kernel memcg OOM SIGKILL (INFRASTRUCTURE), corrected from the earlier max_turns reading |
| gc task outcome | ✅ Completed (23:25:35Z Aug-14 exit-0 run; 17.20 GB → 444 MB pack) |
| Repository health | ✅ HEALTHY — fsck clean, 90.18 MiB pack, 94 MB `.git`, 92 GB disk, 48 GB RAM free |
| Action required | ❌ NONE (closure delegated to domchk-f78f9e84) |
