# Crash Analysis: Bead bf-1s6c3

**Analysis Date:** 2026-09-06
**Crash Date:** 2026-08-12 (storm span 2026-08-12T21:36:44Z → 2026-08-13T02:01:22Z)
**Bead ID:** bf-1s6c3 — "Create merge commit reconciling Forgejo and GitHub histories"
**Bead Created:** 2026-08-12T21:12:09Z · **Bead Closed:** 2026-08-16T14:00:13Z
**Agent:** claude-code-glm-4.7-lab-domain-check (worker) · session `8446529e`
**Model:** glm-4.7 (zai provider) · template `pluck-default`
**Classification:** INFRASTRUCTURE EVENT — repository bloat → memory-cgroup OOM (NOT a code defect)
**Confidence Level:** HIGH on crash type (~95%); HIGH on repository-bloat sub-type (~90%, epistemic caveat below)

---

## Executive Summary

**CRITICAL FINDING:** This was an **INFRASTRUCTURE CRASH** caused by repository bloat — an
≈18 GB `.git` holding ≈17 GB of loose objects — which made every significant git operation
exceed the dispatch scope's memory budget until the kernel's memory-cgroup OOM killer
terminated the worker. It was **NOT** a domain-check code defect, a workflow failure, or a
service outage.

The single "crash at 2026-08-12T22:24:04" framing in the dispatch is a **misreading of one
event inside a 4.5-hour retry storm**: bf-1s6c3 was dispatched **76 times**, producing
**71 × exit −1 (signal death), 4 × exit 124 (600 s timeout), and 1 × exit 0 (success)**.
The 22:24:04 timestamp is a `HANDLING_RELEASE_DONE` heartbeat recorded **7.2 seconds after**
the actual exit −1 kill, not the kill itself.

The deliverable — merge commit `42a7b07` — **landed mid-storm** at 2026-08-12T21:47:07Z
(attempt #4, itself killed 59.6 s later); the remaining 72 dispatches ran against an
already-satisfied task. The storm ended when attempt #76 completed successfully at
02:01:22Z.

### Key Facts

| Fact | Value |
|------|-------|
| Exit Code | −1 (needle's `wait()` sentinel for "died by signal, code unrecorded" — **not** a signal number) |
| Dispatches | 76 (71 crash, 4 timeout, 1 success) |
| Storm span | 265 minutes (2026-08-12T21:36:44Z → 2026-08-13T02:01:22Z) |
| Median inter-kill gap | 177 s (kill density 2.68/10 min) |
| Repository at crash time | ≈18 GB `.git`, ≈17 GB loose objects (should be <500 MB) |
| Root cause | 17+ identical ~237 MB `.beads/*.jsonl` bead-state snapshots committed to git |
| Domain-check code | ✅ No defect — the task never touched application code |

---

## What Bead bf-1s6c3 Was Trying to Accomplish

### Task Description
- **Title:** Create merge commit reconciling Forgejo and GitHub histories
- **Objective:** Using the divergence analysis from bead bf-2xygo (created 2026-08-12T21:12:00Z,
  9 seconds before this bead), create a merge commit reconciling the divergent Forgejo and
  GitHub branches — reconcile with a merge commit, never force-push
- **Priority:** P2 · **Type:** task
- **Network operations:** none (local git only)

### Task Outcome: ✅ COMPLETED (after a 4.5-hour kill storm)

| Metric | Value | Status |
|--------|-------|--------|
| Deliverable | merge `42a7b07` (parents `47e7758` + `00117cb`), 2026-08-12T21:47:07Z | ✅ landed mid-storm |
| Final attempt | exit 0 at 2026-08-13T02:01:22.561Z (384,204 ms) | ✅ success |
| Bead closure | 2026-08-16T14:00:13Z, actor `system` | ✅ closed |
| Worker deaths along the way | 71 signal deaths + 4 timeouts | ❌ infrastructure |

> **Dispatch-premise correction.** The dispatch context states the work "was completed by
> another agent after crash." Primary sources do not support a second agent: the deliverable
> was committed by **attempt #4 of this same bead/worker/session**, and the bead's own
> **attempt #76** later exited 0. No other worker or bead contributed the merge.

---

## Crash Details

### The cited timestamp, reconciled

The dispatch cites **2026-08-12T22:24:04, exit code −1**. Both halves are real but neither is
"the" crash:

```json
{"timestamp":"2026-08-12T22:23:56.990750799Z","event_type":"agent.completed", "...":"exit_code": -1, "duration_ms": 225105}
{"timestamp":"2026-08-12T22:23:56.999212098Z","event_type":"outcome.classified","data":{"exit_code": -1, "outcome": "crash"}}
{"timestamp":"2026-08-12T22:24:04.189335040Z","event_type":"heartbeat.emitted","data":{"state":"HANDLING_RELEASE_DONE"}}
```

The 22:24:04.189Z event is the crash handler's `HANDLING_RELEASE_DONE` **heartbeat**, 7.2 s
after the real kill (`agent.completed`, exit −1, at 22:23:56.990Z) — the same heartbeat-trails-
kill offset that mis-dated other alerts in this workspace. That attempt had run 225,105 ms
(~3.8 min). It is one of the 71 deaths, and crash-alert records for this event carry the
heartbeat timestamp (`...22:24:04.189392531+00:00`) as their "Original Crash Date."

### Full storm census (worker log, primary source)

Source: `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl` and
`-2026-08-13.jsonl`. Single worker, single session `8446529e`. Every one of the 76 attempts
produced an `outcome.classified` event — there are no silent-death gaps.

| Window (UTC) | Claims | exit −1 crash | exit 124 timeout | exit 0 success |
|---|---|---|---|---|
| Aug-12 21:31 → 23:57 | 50 | 49 | 0 | 0 |
| Aug-13 00:00 → 02:01 | 26 | 22 | 4 | 1 |
| **Total** | **76** | **71** | **4** | **1** |

- **First claim:** 2026-08-12T21:31:27.663Z (19 min after bead creation)
- **First kill:** 2026-08-12T21:36:44.519Z — exit −1, after 316,572 ms
- **Attempt durations:** ~2–8 min while dying fast; exactly 600 s (`exit 124`) from 01:11Z;
  final attempt succeeded in 384 s
- **Re-dispatch cadence:** `bead.released` → `bead.claim.succeeded` → `agent.dispatched` in
  ~10 s, so the bead was effectively continuously in-flight for 4.5 hours

### What Killed the Process

**Answer:** the kernel's memory-cgroup OOM killer, terminating the worker mid-attempt during
git work on the bloated repository. Needle records such deaths as `exit_code = -1` — a
`wait()` sentinel meaning "died by signal, exit code unrecorded," not a signal number. The
correct Unix encodings (137 = SIGKILL, 129 = SIGHUP) appear nowhere in the log, exactly as
the sentinel predicts.

**Mechanism:**
1. Agent dispatched to build a two-parent history merge on the 18 GB repository
2. Git work pulled the oversized loose-object store through memory
3. The dispatch scope's memory cgroup limit was exceeded
4. Kernel OOM killer delivered uncatchable SIGKILL to the worker
5. Needle recorded exit −1, classified `crash`, released the bead (~10 s later it was re-claimed)
6. Repeat — with no stop-condition for "deliverable present, bead still open"

### The deliverable landed mid-storm

| Event | Timestamp (UTC) | Note |
|-------|-----------------|------|
| Attempt #4 claimed | 2026-08-12T21:43:20Z | |
| **Merge `42a7b07` committed** | **2026-08-12T21:47:07Z** | the task's deliverable |
| Attempt #4 killed | 2026-08-12T21:48:06.650Z | exit −1, **59.6 s after the commit** |
| …72 further dispatches | 21:48Z → 02:01Z | against already-satisfied work |
| Attempt #76 succeeded | 2026-08-13T02:01:22.561Z | exit 0; storm ends |
| Bead closed | 2026-08-16T14:00:13Z | close reason cites the merge as `7dd79eb` |

So the event is neither a pure "crash during task" nor a post-completion false positive: the
work product survived on disk 16 minutes into a storm whose remaining volume was pure
re-dispatch against finished work.

### Post-crash history caveat

- `42a7b07` is a real two-parent merge but is **not an ancestor of `main`**. The 2026-08-16
  history squash moved it to branch **`pre-squash-history-20260816`**, its only containing ref.
- `main` carries the **later** reconciliation: `46293c5` "Merge Forgejo and GitHub histories"
  (2026-08-17), resolving the bf-4k2ws divergence.
- The bead's close reason cites `7dd79eb` — a **dead pre-squash name** of `42a7b07` (identical
  subject and timestamp). Earlier catalog docs cite `2832106`, which **does not exist**.
  Any acceptance re-verification must use `46293c5` for "main contains the reconciled
  history," and `42a7b07` for this bead's own deliverable.

---

## Evidence and Investigation Findings

### Primary sources

| Source | What it establishes |
|--------|---------------------|
| `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl` (3,589,712 bytes, unchanged since Aug-12) | the 49 Aug-12 kills, attempt durations, heartbeat trail |
| `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` | the 22 kills, 4 timeouts, and the exit-0 success |
| `docs/crash-analysis/bf-1s6c3-agent-logs-preserved.md` | preserved extract of the Aug-12/13 events |
| `docs/crashes/bf-1s6c3/needle-events-2026-08-12-bf-1s6c3.jsonl` | raw events incl. the 22:24:04 heartbeat |
| `.beads/checkpoint/forensic.jsonl` | the bead's `closed` event and close reason |
| live `git` inspection (2026-09-06) | `42a7b07` identity, ancestry, dead-SHA confirmation |

### Repository state at crash time

| Metric | Value | Healthy threshold |
|--------|-------|-------------------|
| Repository size | ≈18 GB | <500 MB |
| Loose objects | ≈17 GB (4,482 objects) | <100 MB |
| Pack files | 9.60 MB | pack-dominant |
| Loose:packed ratio | 1,832:1 (inverted) | <1:10 |

**Cause of the bloat:** 17+ identical ~237 MB `.beads/*.jsonl` bead-state snapshots
(`issues.jsonl`, `beads.base.jsonl`, `.bf_history/issues-*.jsonl`) had been committed during
earlier automated divergence-analysis runs. `.beads/` was not gitignored and no pre-commit
size gate existed, so the repository silently grew 36× until routine git operations became
memory-fatal.

### Epistemic status of the sub-type (why ~90%, not 100%)

Two legs rest on contemporaneous documentation rather than re-measurable state:

1. **The 18 GB figure is canon-sourced, not re-measurable.** The offending blobs were packed
   away on 2026-09-01; the largest blob surviving today is 14,970,288 bytes (a Mach-O
   executable, 6 copies). No 237 MB `.beads/*.jsonl` snapshot blob remains inspectable.
2. **No kernel record exists for Aug-12.** journald on this box begins
   2026-08-15T19:46:33 EDT, so the "<2 GB available / >50 GB git spike" figures in earlier
   docs are reconstruction, not kernel-proven. The memcg `CONSTRAINT_MEMCG` mechanism is
   kernel-proven only for the better-instrumented later siblings (bf-4x12ec, bf-198ne).

The classification therefore rests on the **Pattern-3 signature** — fixed-cadence re-dispatch
deaths for hours on a >5 GB repository where the task itself is a git operation, with zero
exit-code variation — which is fully present here, not on a kernel record that cannot exist.

---

## Classification

### Infrastructure event — repository bloat sub-type (Pattern 3)

| Aspect | Finding |
|--------|---------|
| **Crash type** | Infrastructure — memcg OOM SIGKILL during git operations |
| **Exit code** | −1 (signal-death sentinel), zero variation across all 71 deaths |
| **Code defect** | ✅ NONE — the task was pure git history reconciliation; no domain-check code was involved |
| **Reproducibility** | Was high (every git attempt died); now eliminated — repository repaired |
| **Alert disposition** | No action — subject bead closed, work represented on `main`, repo repaired |

### Excluded alternates

- **Workflow failure** — requires exit 1 + `error_max_turns`. None: every death was a signal
  death, and `transform.completed` succeeded on each attempt.
- **Service failure** — requires HTTP 503/502 from the inference gateway. No 5xx in any of
  the 76 attempts.
- **Code defect** — no application error surfaced in any attempt (see above); consistent with
  the standing finding that no domain-check code defect has ever been confirmed in this
  workspace's crash record.
- **FALSE_POSITIVE (post-completion cleanup death)** — the guide's rule triggers on a kill
  <30 s after the final commit. Here the deaths were **mid-attempt** (median run 177 s);
  attempt #4's commit-to-death gap was 59.6 s, above the threshold, and 71 further kills
  followed. The alert *sense* does contain a false-positive component — 72 of 76 dispatches
  ran against satisfied work — but that is the re-dispatch amplifier, not the kill mechanism.

### Root cause statement

- **Immediate cause:** each dispatch executed significant git work (two-parent merge across
  diverged histories) against an ≈18 GB repository whose object store exceeded the dispatch
  scope's memory budget; the kernel killed the worker mid-attempt.
- **Amplifying cause (why 4.5 hours):** needle's crash handler released and re-claimed the
  bead on a ~10 s cycle, and no stop-condition existed for "deliverable present, bead still
  open" — so 72 dispatches redid expensive git work against satisfied work until one attempt
  happened to survive.
- **Underlying cause:** 17+ identical ~237 MB `.beads/*.jsonl` snapshots committed to git —
  the same underlying cause as bf-4yjq and bf-2xygo earlier the same evening.

---

## Cross-References — the 2026-08-12 OOM Period

bf-1s6c3 was **not an isolated event**. It sits in the middle of a same-day workspace storm:
**455 exit −1 events across 6 beads over ~18.5 h**, concentrated rather than synchronized —
one bead's retry loop would exhaust and the next would take over (a rolling handoff, not a
system-wide simultaneous event).

| Bead | Relationship to bf-1s6c3 | Record |
|------|--------------------------|--------|
| **bf-31mno** | Largest storm of the day — 350 kills; no dedicated RCA | `docs/crash-summary-bf-4yjq-2026-09-06.md` (context table) |
| **bf-4yjq** | Immediately preceding shift of the same storm — 50 kills, 17:54–20:30Z, same evening, same repo condition | `docs/crash-summary-bf-4yjq-2026-09-06.md`, `docs/crash-analysis/bf-4yjq-resolution-record-2026-09-01.md` |
| **bf-2xygo** | Direct predecessor — the divergence analysis this bead's merge was to act on; created 9 s before bf-1s6c3; hit the same bloat | `docs/crashes/bf-2xygo-crash-classification-2026-09-06.md` |
| **bf-4k2ws** | Successor divergence the *later* reconciliation (`46293c5`) resolved | `docs/crash-investigation-bf-4k2ws-2026-09-01.md` |
| **bf-4x12ec** (2026-08-14) | Same mechanism, gc variant — first kernel-proven memcg kill; 129-attempt storm | `docs/crash-investigation-bf-4x12ec.md`, `docs/crashes/bf-198ne-crash-report.md` (mechanism) |
| **bf-173o7e** (2026-08-14) | Same mechanism, duplicate-gc-bead variant — 131-attempt storm | `docs/root-cause-analysis-bf-173o7e-2026-09-01.md` |
| **bf-198ne** (2026-08-16) | Same mechanism, `git push` pack-objects variant | `docs/crashes/bf-198ne-crash-report.md` |

**Catalog-wide context:** `docs/crash-analysis/repository-bloat-root-cause-analysis-2026-08-12.md`
(the period-level RCA for this directory) and
`docs/crashes/bf-1s6c3-crash-storm-timeline-2026-09-06.md` (the primary-source timeline whose
figures this document adopts).

---

## Resolution and Prevention

### Resolution

- **Bead:** closed 2026-08-16T14:00:13Z; deliverable committed 2026-08-12T21:47:07Z.
- **Repository:** the bloat was packed down on 2026-09-01 — **18 GB → ~94 MB** — and
  re-verified repeatedly since, most recently 2026-09-06 (`.git` 94–98 MB, 130–154 loose
  objects / ~4 MiB, one 90.93 MiB pack, 0 garbage, `git fsck --full` clean).

### Prevention now in force

| Layer | Status |
|-------|--------|
| `.beads/` gitignored (`.gitignore:66`), 0 tracked files, plus repo-wide `*.jsonl` rule | ✅ in place |
| Pre-commit hook blocking staged files >10 MB | ⚠️ installed at `.git/hooks/pre-commit` but per-clone and drifted from `scripts/pre-commit-repo-size-hook`; no installer committed — the one open gap |
| `pack.windowMemory=2g` / `pack.deltaCacheSize=1g` / `pack.threads=1` bounding bare `git gc` **and** `git push` | ✅ applied repo-local + global; `scripts/setup-git-gc-config.sh --verify` passes |
| `scripts/safe-git-gc.sh` for all maintenance (never bare `git gc --aggressive`) | ✅ in place |
| Six systemd user timers: repo-health daily, incremental gc daily, full gc weekly | ✅ installed and firing (re-verified 2026-09-06) |
| Crash-alert closed-bead/duplicate filtering (stops re-investigating satisfied work) | ✅ in place |
| Re-dispatch stop-condition for satisfied work (the amplifier) | ❌ NEEDLE-side, outside this repo — the only lever that would have bounded this storm; recorded as the systemic finding |

---

## Conclusions

1. **Infrastructure, not code.** 71 signal deaths with zero exit-code variation, on a git
   task, inside a same-evening bloat storm — infrastructure event, repository-bloat sub-type.
2. **The cited timestamp is a heartbeat, not the kill.** 2026-08-12T22:24:04 trails the
   actual exit −1 at 22:23:56.990Z by 7.2 s.
3. **"The crash" is really 71.** Single-crash framings (21:36:51Z or 00:38:41Z) each name
   one attempt of a 76-dispatch storm.
4. **The deliverable outlived its worker.** Merge `42a7b07` landed 16 minutes into the storm;
   the retry loop, not a second agent, carried the bead to its exit-0 close.
5. **Cite living SHAs.** `7dd79eb` (close reason) and `2832106` (catalog docs) are dead or
   nonexistent; use `42a7b07` on `pre-squash-history-20260816` and `46293c5` on `main`.
6. **No domain-check code defect.** The application was never involved.

### Recommendations

1. **No further remediation for this event in this repository** — the object store is packed,
   the re-entry path is closed, and the alert layers are live.
2. **Commit a pre-commit-hook installer** so the >10 MB gate survives fresh clones (the one
   open gap in this repo's defense).
3. **Add a NEEDLE-side stop-condition** for "deliverable present, bead still open" — the only
   control that would have ended this storm at ~16 minutes instead of 4.5 hours.

---

**Analysis Status:** ✅ COMPLETE
**Evidence:** needle worker logs (primary), preserved extracts, bead checkpoint, live git inspection
**Classification:** INFRASTRUCTURE EVENT — repository bloat → memcg OOM
**Domain-Check Code:** ✅ NO DEFECTS
**Figures adopted from:** `docs/crashes/bf-1s6c3-crash-storm-timeline-2026-09-06.md` and
`docs/crashes/bf-1s6c3-crash-classification-2026-09-06.md` (primary-source corrections, which
supersede the single-crash and dead-SHA claims in earlier bf-1s6c3 docs)

---

**Analysis completed:** 2026-09-06
**Task:** Document bf-1s6c3 in the crash analysis catalog (bead domchk-671f228f)
**Subject bead:** bf-1s6c3 — CLOSED 2026-08-16T14:00:13Z
**Root cause:** repository bloat (≈18 GB / ≈17 GB loose objects) → memory-cgroup OOM SIGKILL during git reconciliation
**Exit code:** −1 (signal-death sentinel), 71 of 76 dispatches
**Signal source:** kernel memory-cgroup OOM killer (not a Unix signal delivered to the agent)
