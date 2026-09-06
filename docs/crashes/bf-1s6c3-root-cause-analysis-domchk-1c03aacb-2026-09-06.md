# Root Cause Analysis — bf-1s6c3 (addendum: precedent contrast + threshold review)

**Analysis bead:** domchk-1c03aacb
**Parent alert:** bf-5cd2d (`ALERT: Agent crash on bead bf-1s6c3`, 2026-08-12T21:52:00Z)
**Prereq (data gathering):** domchk-224ee354 — closed; verified 76 dispatches / 71 × exit −1
**Base classification:** domchk-56b5ba67 — closed; deliverable
[`docs/crashes/bf-1s6c3-crash-classification-2026-09-06.md`](bf-1s6c3-crash-classification-2026-09-06.md)
(commit `9b92cd9`)
**Date:** 2026-09-06

## Relationship to the existing classification

This dispatch's classification, false-positive, and frequency analysis steps were already
rendered by the sibling bead domchk-56b5ba67. That work is **confirmed, not redone**: this
bead re-verified its live claims first-hand (below) and then supplies the two analysis steps
its document does not cover — the named-precedent contrast required by step 4 of this
dispatch, and an explicit review against the workspace resource-limit tables (step 3).

**Confirmed classification (unchanged):** `infrastructure` — repository-bloat sub-type
(crash-response-guide Pattern 3). Confidence high (~95% class / ~90% sub-type, the sub-type
carrying the documented epistemic caveat that no Aug-12 kernel record can exist). Excluded
alternates: workflow failure (no `error_max_turns`, all deaths are signal deaths), service
failure (no HTTP 5xx in any of the 76 attempts), code defect (task never touched
domain-check code).

## Live re-verification (2026-09-06, this bead's own measurements)

| Claim from the base classification | Re-verified value | Result |
|---|---|---|
| `.git` ≈ 98 MB, repaired and holding | `du -sh .git` → **98M** | ✅ |
| Loose objects normal churn, 0 garbage | 171 objects / 4.49 MiB; pack 90.93 MiB; **garbage 0** | ✅ |
| Repository integrity clean | `git fsck --full` → exit 0 (dangling trees only — normal churn) | ✅ |
| `42a7b07` is not an ancestor of `main` | `git merge-base --is-ancestor` → **NOT ancestor** | ✅ |
| `main`'s reconciliation is `46293c5` | `46293c5` "Merge Forgejo and GitHub histories", 2026-08-17 | ✅ |
| `.beads/` cannot re-bloat the repo | `git ls-files .beads` → **0**; `.gitignore:66` `.beads/` | ✅ |
| Divergence resolved (alert notes claim "664 ahead") | `git rev-list --left-right --count origin/main...main` → **0 / 0** | ✅ |
| Host healthy today | 45 G mem available, 58 G disk free, load 5.36/5.40/4.51 | ✅ |

No figure in the base classification failed re-verification.

## Step 4 — known-pattern contrast: bf-1s6c3 vs. `domchk-c9641ac5`

The dispatch names `domchk-c9641ac5` as the known-pattern precedent to check against. That
event is the workspace's canonical **service-failure** case
([`docs/crash-analysis-domchk-c9641ac5-2026-09-01.md`](../crash-analysis-domchk-c9641ac5-2026-09-01.md)),
so the comparison is the cleanest available contrast between the two guide classes that are
most often confused in this corpus. **Verdict: bf-1s6c3 does not match that pattern — it is
the other class.**

| Dimension | bf-1s6c3 (this event) | domchk-c9641ac5 (named precedent) |
|---|---|---|
| Guide class | **Infrastructure** — Pattern 3 repository bloat | **Service failure** — inference gateway |
| Exit code | **−1** (signal death, code unrecorded) × 71 | **1** (application-level error), single event |
| Mechanism | Kernel memcg kill of git work on an ≈18 GB object store | HTTP 503 "no available server" from the gateway |
| Duration signature | 71 deaths of ~2–8 min each over 265 min | One 490,905 ms (~8.2 min) session |
| Persistence | **Persistent** — recurs on every dispatch until the repo is cleaned | **Transient** — gateway recovered; no repo condition involved |
| Detectable pre-crash | Yes (repo size table) — Pattern 3's defining advantage | No — external service state |
| FP Rule 1 (commit < 30 s before death) | Not triggered — 59.6 s, and deaths were mid-attempt | N/A (no deliverable commit in flight) |
| FP Rule 2 (crash → retry → success) | **Surface match only** — attempt 76 exited 0, but the cause was persistent, not healed | **Genuine match** — self-healed transient, the rule's intended case |
| FP Rule 3 (10+ crashes / 10 min) | Not triggered (2.68 / 10 min) — see the base doc's Rule-3 nuance | Not applicable (single event) |
| Correct remediation | Pack the object store; keep bead state out of git; bound pack-objects | Retry with backoff; monitor the gateway health endpoint |
| Recurrence risk after remediation | Eliminated in-repo (98 MB, `.beads/` gitignored, bounded gc) | Unchanged — lives in the fleet, outside this repo |

The one property the two events share is the thing that generated this alert family rather
than the crash itself: in both, the *work* outlived the *worker*, so an alert fired on an
already-satisfied bead. Their dispositions still differ — `domchk-c9641ac5` needed nothing
because the failure was transient and self-healed; bf-1s6c3 needs nothing because the
condition was repaired and is verified holding.

## Step 3 — resource limits and thresholds review

Measured against the standing tables in `CLAUDE.md`. At crash time (2026-08-12) the only
**measurable** violated dimension is the repository-size one — host memory, disk, and load
for that evening have no surviving record (system journald on this box begins
2026-08-15 19:46 EDT), so they are recorded as unknowable rather than assumed healthy.

| Threshold | Value | Crash time (2026-08-12) | Today (2026-09-06) |
|---|---|---|---|
| Total repository size | < 500 MB healthy / > 1 GB critical | **≈18 GB — critical, ~18× the critical line** | 98 MB ✅ |
| Loose objects | < 100 MB healthy / > 500 MB critical | **≈17 GB — critical** | 4.49 MiB ✅ |
| `.git/objects` (Pattern 3 high-risk line) | > 10 GB → preemptive cleanup | **Exceeded** | 4.49 MiB ✅ |
| Dispatch scope memory | MemoryMax 12 GiB | **Exceeded by pack-objects** → memcg SIGKILL | n/a — no gc/push storm running |
| Available memory | ≥ 20 GB healthy | Unknowable (no record) | 45 GB ✅ |
| Disk free | ≥ 50 GB healthy | Unknowable (no record) | 58 GB ✅ |
| Load (1 min) | < 5 conservative / < 10 warning | Unknowable (no record) | 5.36 — inside the warning band on a 12-core box (~45% util.), consistent with normal fleet activity; not a crash factor |

Reading: the violated thresholds were all on the **repository-size** axis, none on the host
axis — which is exactly why Pattern 3 is carved out as its own infrastructure sub-type rather
than generic memory pressure, and why the fix landed in the repository (gitignore + bounded
gc + 10 MB pre-commit gate) rather than in host capacity. The pre-task resource check that
`CLAUDE.md` prescribes (`free -g` → abort under 10 G available) would **not** have caught
this event, because host memory was not the binding constraint — the cgroup against an 18 GB
object store was. That asymmetry is the reason the repo-size table, not the memory table, is
the operative pre-flight for this crash type.

## Alert disposition

**No further analysis or remediation action is required for this event.** Concretely:

- Subject bead bf-1s6c3: **closed** 2026-08-16T14:00:13Z. Its deliverable is represented on
  `main` by `46293c5` — any acceptance re-verification must use that SHA, not the bead's
  cited `7dd79eb` (dead pre-squash SHA) or `2832106` (never existed).
- Parent alert bf-5cd2d: remains **Open**, and this bead does not close it. Per workspace
  convention the alert's closure belongs to its dedicated closure bead, not to an analysis
  child. Its own notes carry the stale claims corrected in
  [`docs/crashes/bf-1s6c3-crash-storm-timeline-2026-09-06.md`](bf-1s6c3-crash-storm-timeline-2026-09-06.md)
  (`7dd79eb`, "664 commits ahead", "task completed successfully") — the divergence half of
  that is separately written up in
  [`docs/branch-divergence-analysis.md`](../branch-divergence-analysis.md) (0/0 today).
- Open remediation items are unchanged from the base classification's table: item 3
  (pre-commit hook installer / drift) is the one in-repo gap worth a bead; item 6
  (re-dispatch stop-condition for satisfied work — the amplifier that turned one kill into
  71) is NEEDLE-side and outside this repository.
