# bf-4x12ec Crash Log Review — 2026-09-02 (bead domchk-30d451d3)

Independent re-verification of the primary-source needle event log for the
bf-4x12ec crash, plus new primary-source findings that extend Addendum 2 of
[`bf-4x12ec-crash-investigation.md`](bf-4x12ec-crash-investigation.md).

**Sources reviewed:**
- `/home/coding/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl` (2.6 MB, primary)
- The other five same-day needle logs (roam-1, roam-2, drawrace, s1, test-fix) — isolation cross-check
- Live bead record `bf-4x12ec`
- [`docs/crash-context-bf-4x12ec-summary.md`](../crash-context-bf-4x12ec-summary.md) (2026-08-26 summary — see corrections note)

## Workspace and agent version

| Field | Value (from every one of the 53 events) |
|---|---|
| Workspace | `/home/coding/domain-check` |
| Worker | `claude-code-glm-4.7-lab-domain-check` (no `-2` worker involved on Aug 14) |
| Session | `a6dbb1fc` |
| Agent | `claude-code-glm-4.7` |
| Model | `glm-4.7` |
| Transform binary | `needle-transform-claude` |

## What bf-4x12ec was trying to do

Phase 1.2 emergency repository stabilization: run `git gc --aggressive
--prune=now` plus `git repack -a -d --depth=250 --window=250` to pack 17.20 GB
of loose objects (4,627 objects, ~18 GB repo) and eliminate the OOM risk
during git operations. The bead was created 2026-08-14T10:17:26Z; the first
agent claimed it at 10:21:06.969Z and died 116 s later.

## Verified event-log facts (53 attempts, all 2026-08-14)

| Phase | Window (UTC) | Attempts | Exit code | Classified / handled | Duration |
|---|---|---|---|---|---|
| 1. Crash loop | 10:23:02 – 11:27:26 | 44 | `-1` | `crash` / `alerted` | 38.9 – 115.8 s |
| 2. Timeout loop | 11:38:07 – 12:50:14 | 8 | `124` | `timeout` / `deferred` | exactly 600.0 s |
| 3. Success | 12:58:45 | 1 | `0` | `success` / `none` | 491.8 s |

- **First death:** 2026-08-14T10:23:02.958Z (`exit_code=-1`, 115.8 s into attempt #1)
- **First alert:** 2026-08-14T10:23:14.244801795Z — 11.3 s after the first
  death. **This is the earliest alert for this bead and supersedes all four
  timestamps in the "conflicting crash timestamps" table in Addendum 2**
  (10:25:30, 10:39:42, 10:41:13, 11:14:39 — each is just one of the 44 alerts
  inside the storm; last alert 11:28:04.917Z).
- **Work completed:** 2026-08-14T12:58:45Z (`exit_code=0`); the bead's Aug-17
  14:50:41Z "Updated" timestamp is the manual closure time, not completion.
- Attempts 2–44 died the same way; no attempt in phase 1 survived longer than
  115.8 s. The bead was re-claimed and re-dispatched 53 times in total.

### New primary-source findings (not in Addendum 2)

1. **`worker.handling.timeout` at 10:43:35.281763879Z** — `operation: flush`,
   `error: "bf sync --flush-only failed"`, `outcome: release`. A bead
   checkpoint flush failed mid-storm while attempt #15 was in flight (attempt
   #15 died at 10:44:53Z). This is
   direct evidence that the bead-store write path was also struggling during
   the memory-pressure window, and a concrete instance of the
   "flush before pull" hazard documented in CLAUDE.md.
2. **`verification.passed` at 12:58:45.126Z** (`gates_run: 1`) — 11 ms after
   the successful attempt, a verification gate ran and passed. The success was
   gated, not merely exit-0.
3. **`bead.orphaned` at 12:58:55.502Z** — 10 s after the success the bead was
   orphaned (released unassigned) rather than closed. This explains why the
   bead stayed open until someone closed it manually on Aug 17 with the
   completion notes, and is the likely reason the alert system kept
   regenerating alerts for it (the false-positive pattern in
   `docs/crash-context-bf-4x12ec-summary.md`).
4. **Dispatch metadata:** 47 of 53 dispatches used template
   `pluck/pluck-default` with an identical 71,698-byte prompt
   (`sha256:b975715d…`) — the same task text re-sent every retry. The 6
   phase-2 dispatches used `split/split-default` (3,896 bytes, 6 distinct
   hashes — the deferred re-dispatch context differed slightly each time).
5. **Isolation holds fleet-wide, not just within the worker log.** Addendum 2
   checked only the domain-check log (0/44 within ±3 s). Scanning all six
   same-day worker logs (980 other-bead completions) finds exactly one
   alignment: `bf-rgk1v` on the unrelated `roam-1` worker exited 0 after a
   15.0 s run, 2.77 s before bf-4x12ec attempt #11's death. Expected chance
   alignments at this density are ~3, so this is noise; the storm remained
   specific to bf-4x12ec's retry cycle.

## Exit code −1, precisely

`exit_code=-1` is the needle worker's classification for a child agent that
terminated without a wait status (killed by a signal), not a POSIX value. The
worker classified each death `outcome=crash`, alerted, released the bead and
immediately re-claimed it — 44 times in 64 minutes. SIGKILL from the kernel
OOM killer under a cgroup memory limit remains the most consistent
explanation (see Addendum 2 for the CONSTRAINT_MEMCG corroboration and the
kernel-log evidence-window limitation; Aug-14 journals do not survive).

## Outcome

The work bf-4x12ec existed to do **succeeded at 12:58:45Z on the 53rd
attempt**: repo ~18 GB → 753 MB, loose objects 4,627 → 141, git operations
restored. Since then the scheduled safe-git-gc maintenance has taken it
further (92 MB / 20 loose objects as of 2026-09-02). No code defect; no
recovery action needed; bead Closed.

## ⚠️ Uncommitted working-tree contradiction (noted 2026-09-02)

The working tree of `docs/crash-investigations/bf-4x12ec-crash-investigation.md`
carries an **uncommitted** edit to its Summary/Crash-Window lines that
reintroduces the "gc SIGKILLed at 11:14:39Z after ~57 minutes" narrative — a
claim Addendum 2 (committed in `d0c7497`) explicitly refuted: no phase-1
attempt survived longer than 115.8 s and the 57-minute figure matches nothing
in the event log. This document is written from the primary source; **the
committed Addendum 2 + this review are authoritative** — do not cite the
uncommitted v1.3-in-progress summary lines. The edit was left untouched (not
this task's change to make).

## Corrections to `docs/crash-context-bf-4x12ec-summary.md` (2026-08-26)

That summary predates Addendum 2 and asserts "Crash Time:
2026-08-14T11:14:39Z", root cause "timeout/capacity governance … NOT OOM", and
"the original crash occurred during a long-running git garbage collection
operation". All three are contradicted by the event log (see Addendum 2 §
Corrections, items 1–4). Read it for its false-positive-alert inventory, not
for its crash mechanism.

---
**Review Date:** 2026-09-02 (bead domchk-30d451d3)
**Method:** independent re-parse of the primary JSONL event log; every figure
in the phase table re-derived from `agent.completed` events, not copied from
prior reports
**Confidence:** HIGH — primary source, single worker log, complete event stream
