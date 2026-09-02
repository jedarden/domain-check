# Verification Report: Agent Crash Alert bf-28su5u

**Date:** 2026-08-26
**Alert Bead ID:** bf-28su5u
**Target Bead:** bf-173o7e
**Alert:** Agent crash on bead bf-173o7e
**Status:** ✅ RESOLVED - Duplicate False Positive

> **⚠️ 2026-09-02 addendum appended below.** The mechanism attribution in this
> report ("turn limit exhaustion", "NOT an OOM kill") was based on the wrong
> dispatch's trace and is superseded by the definitive 2026-09-02 root-cause
> determination: **INFRASTRUCTURE — kernel memcg OOM SIGKILL**. The disposition
> is unchanged: the kill was real, the work completed, and no retry was needed.

## Summary

The crash alert bf-28su5u was generated to investigate an agent crash on bead bf-173o7e. **This alert is a duplicate false positive.** The target crash has been thoroughly investigated and resolved. The reported "crash" was actually a turn limit exhaustion during administrative operations, not a technical failure.

## Investigation Findings

### Alert Details
- **Alert Bead:** bf-28su5u (created 2026-08-14T14:02:25.583997541Z)
- **Target Bead:** bf-173o7e (Execute git gc --aggressive with pruning)
- **Reported Exit Code:** -1 (signal -1)
- **Reported Cause:** Agent process killed

### Original Crash Investigation (bf-173o7e)

The crash on bead bf-173o7e has been thoroughly investigated across multiple reports:

| Report | Date | Finding |
|--------|------|---------|
| `docs/verification-report-bf-26sup4-crash-alert-resolved-bf-173o7e.md` | 2026-08-26 | False positive - turn limit exhaustion |
| `docs/crash-investigation-bf-173o7e-definitive-2026-08-25.md` | 2026-08-25 | Definitive investigation - administrative failure |
| `docs/crash-evidence-bf-173o7e-complete-summary.md` | 2026-08-25 | Complete evidence summary |
| `docs/system-state-investigation-bf-173o7e-2026-08-14.md` | 2026-08-14 | System state analysis |

### Actual Events (from established evidence)

**Real Exit Code:** 1 (NOT -1 as reported)
**Real Error:** `error_max_turns` (turn limit exhaustion)
**Real Timestamp:** 2026-08-17T17:06:59.953876423Z

### Task Execution Status

The git gc task on bead bf-173o7e **completed successfully**:

| Aspect | Status | Evidence |
|--------|--------|----------|
| Git GC Operation | ✅ Success | Repository reduced from ~18GB to 445MB (97.5% reduction) |
| Repository Integrity | ✅ Valid | 8,384 objects packed successfully, git status confirmed |
| Resource Usage | ✅ Normal | Peak memory 1.1GB, duration ~7 minutes |
| Acceptance Criteria | ✅ All Met | All three criteria satisfied |

### Duplicate Alert Pattern

This crash alert (bf-28su5u) is one of many duplicate alerts generated for the same resolved crash:

- bf-26sup4 - resolved as false positive
- bf-2e7xrf - duplicate alert referencing resolved bf-173o7e
- bf-4byenr - false positive alert resolved
- bf-2s53ez - duplicate false positive referencing resolved bf-173o7e
- bf-4cxa1d - duplicate false positive referencing resolved bf-173o7e
- bf-4iviwf - duplicate alert referencing resolved bf-173o7e
- bf-ac23zs - crash alert referencing bf-173o7e
- **bf-28su5u** - this alert

## Classification

**DUPLICATE FALSE POSITIVE** - Administrative process failure, already resolved

### What This Was NOT
- ❌ A signal-based crash (exit code was 1, not -1)
- ❌ An OOM kill during task execution (peak memory was only 1.1GB)
- ❌ A code defect or agent malfunction
- ❌ Repository corruption or data loss
- ❌ Task failure (all objectives achieved)
- ❌ A new crash event

### What This WAS
- ✅ A duplicate alert referencing an already-investigated crash
- ✅ Turn limit exhaustion during administrative operations (original event)
- ✅ Successful git gc operation (97.5% size reduction)
- ✅ Repository optimization completed (original task)
- ✅ Expected behavior for long-running administrative tasks
- ✅ Already resolved and documented

## Evidence Sources

### Primary Evidence (from original investigation)
- `.beads/traces/bf-173o7e/metadata.json` - Exit code 1, error_max_turns
- `.beads/traces/bf-173o7e/trace.jsonl` - Full execution trace (21,570 lines)
- `.beads/traces/bf-173o7e/stdout.txt` - Agent output (1.5MB)

### Established Investigation Reports
- `docs/verification-report-bf-26sup4-crash-alert-resolved-bf-173o7e.md` - Comprehensive verification
- `docs/crash-evidence-bf-173o7e-complete-summary.md` - Complete evidence summary
- `docs/crash-investigation-bf-173o7e-definitive-2026-08-25.md` - Definitive investigation
- `docs/system-state-investigation-bf-173o7e-2026-08-14.md` - System state analysis

## Conclusions

### Alert Validity
**INVALID DUPLICATE** - The crash alert bf-28su5u is a duplicate of an already-resolved alert:
1. Original crash (bf-173o7e) was investigated and resolved
2. Multiple duplicate alerts have been generated for the same event
3. No new information or evidence is presented
4. The underlying issue (turn limit exhaustion) is understood and documented

### Task Success
The underlying git gc task on bead bf-173o7e **completed successfully**:
- ✅ Repository size reduced from ~18GB to 445MB (97.5% reduction)
- ✅ All 8,384 objects successfully packed
- ✅ No OOM or timeout issues during execution
- ✅ Repository integrity maintained and verified

### Alert Resolution
**RESOLVED** - No action required. This is a duplicate of an already-investigated and resolved crash.

## Recommendations

### For Crash Detection System
1. **Implement duplicate detection** - Cross-reference new alerts against existing crash beads before creating new ones
2. **Track crash resolution status** - Mark resolved crashes to prevent duplicate investigation
3. **Correlation by task** - Group alerts that reference the same original crash bead
4. **Alert consolidation** - Prevent multiple alerts for the same historical crash event

### For Future Alerts
1. **Verify existing reports** - Before creating new alerts, search for existing verification reports
2. **Check bead status** - Verify if the target bead has already been investigated
3. **Consolidate investigations** - If multiple alerts exist for the same crash, consolidate into one report

## Verification Status

✅ **Alert Resolved** - Duplicate false positive confirmed. No action required.

---

**Verification Performed By:** claude-code-glm-4.7-lab-domain-check-2
**Verification Date:** 2026-08-26
**Classification:** Duplicate False Positive - Already resolved crash
**Related Beads:** bf-173o7e (closed, successful), bf-28su5u (this duplicate alert)
**Related Alerts:** bf-26sup4, bf-2e7xrf, bf-4byenr, bf-2s53ez, bf-4cxa1d, bf-4iviwf, bf-ac23zs

---

## Addendum 2026-09-02 — Mechanism reclassified: INFRASTRUCTURE (kernel memcg OOM SIGKILL)

**Addendum by:** claude-code-glm-4.7-lab-roam-4 (verification dispatch `domchk-e1792d54`)
**Supersedes:** the "Actual Events", "What This Was NOT / WAS", and "Primary
Evidence" sections above — on the mechanism question only.
**Disposition unchanged:** the kill was real, the gc work completed, bead
bf-173o7e was correctly closed 2026-08-17, and **no retry was ever needed**.

### Why this report misattributed the mechanism

This report (2026-08-26) cited `.beads/traces/bf-173o7e/` (exit 1,
`error_max_turns`, 2026-08-17T17:06:59Z) as "the real event". Needle traces are
**single-slot — they hold only the most recent dispatch**. By Aug-26 the trace
belonged to the bead's *final* Aug-17 dispatch, not to the Aug-14 kill that
generated alert bf-28su5u (created 2026-08-14T14:02:25Z). The evidence cited
was real but belonged to a different event eleven days later.

### Corrected mechanism (definitive, HIGH confidence)

Per
[`docs/investigations/bf-173o7e-root-cause-determination-domchk-2e371a2c-2026-09-02.md`](investigations/bf-173o7e-root-cause-determination-domchk-2e371a2c-2026-09-02.md)
(commit `07ab240`), consolidating the 2026-09-02 investigation cycle:

- bf-28su5u's kill is one of the **129 × exit −1** events of bf-173o7e's
  **132-dispatch Aug-14 retry storm** (12:58:58Z → 23:25:35Z). Its alert
  timestamp 14:02:25Z is a `HANDLING_RELEASE_DONE` heartbeat trailing the real
  `agent.completed` kill by seconds-to-minutes — alert timestamps are never
  kill times.
- `exit −1` is needle's sentinel for a dispatch that died without recording an
  exit code; it is **not a signal number**. The underlying death was
  **SIGKILL from the kernel memory-cgroup controller**: the bead-prescribed
  bare `git gc --aggressive --prune=now` over 17.20 GiB of loose objects built
  delta chains in memory with no `pack.windowMemory` bound and exhausted the
  dispatch scope's `MemoryMax=12 GiB` (`oom_score_adj=200`) — pack-objects RSS
  hugging the 12 GiB cap in the kernel OOM records, all `CONSTRAINT_MEMCG`.
  Host RAM was healthy (~45 GiB free): a scope-budget kill, not host
  exhaustion.
- Retracted for this alert's event: "❌ An OOM kill during task execution
  (peak memory was only 1.1GB)" — the 1.1 GB / "~7 minutes" figures come from
  a *successful* gc run measured later on the already-packed repository, and
  cannot describe the killed Aug-14 attempts, which died mid-pack at scope
  budget. Likewise "Real exit code: 1" — the killed dispatches never recorded
  an exit code (sentinel −1); exit 1 / `error_max_turns` belongs to the
  separate Aug-17 final dispatch.

### Task completion (conclusion unchanged, now correctly explained)

The gc did **not** complete in the killed attempts — the 129 flat kill
durations of 21.6–216.6 s spread over 10.5 h prove the object set never shrank
between attempts. It completed via the storm's final **exit-0 attempt at
23:25:35Z Aug-14** (40.1 s), and was consolidated by the Aug-17 closure
("17.20GB loose objects packed into 444MB pack file, repository valid" —
re-verified 2026-09-02 from the bf-173o7e closed event in
`.beads/checkpoint/forensic.jsonl`).

**Repository health re-verified 2026-09-02 for this addendum:** 161 loose
objects / 1.26 MiB, 1 pack / 90.18 MiB, `.git` = 94 MB, `git fsck` clean
(dangling objects only). No bloat regression.

### Corrected classification

**INFRASTRUCTURE (kernel memcg OOM SIGKILL) — alert-level false positive only.**
The kill was real; "false positive" applies only to the alert's implied
conclusion that the task failed and needed retry. Sibling precedent:
[`docs/verification-report-domchk-673b47e3-bf-173o7e-alert-resolution-2026-09-02.md`](verification-report-domchk-673b47e3-bf-173o7e-alert-resolution-2026-09-02.md)
(commit `6210dbf`) resolves the equivalent stale alert `domchk-673b47e3` with
the same classification.

### Remediation now in force (post-dates the original event)

| Fix | Artifact |
|---|---|
| Bare `git gc` bounded at the config layer (`pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` → ≈3 GiB worst case; crash command re-run under a 768 MiB cgroup, peak pack-objects RSS ≈ 312 MiB, exit 0) | `scripts/setup-git-gc-config.sh --verify`, `scripts/test-gc-memory-bounds.sh` |
| Staged, memory-limited, resumable replacement for bare gc | `scripts/safe-git-gc.sh` + systemd timers |
| Closed-bead filtering, duplicate detection, cooldown — prevents stale alerts like this one from being re-fired against resolved beads | `scripts/crash-alert-manager.sh` |

### Note on the archived copy

`docs/archive/bead-verification-reports/BEAD_BF-28SU5U_VERIFICATION_REPORT.md`
(the root-level report from commits `7bdb217`/`8eb2af0`, later archived)
retains its original uncorrected text; this addendum supersedes it on
mechanism as well.

**Addendum verification date:** 2026-09-02
**Corrected classification:** INFRASTRUCTURE — memcg OOM SIGKILL; work complete; no retry
