# Crash Classification: bf-mje3pd (2026-08-13)

**Classification bead:** domchk-bf8c4fd3
**Date classified:** 2026-09-07
**Method:** `docs/crash-response-guide.md` exit-code decision tree (line 1026: exit −1 →
Infrastructure Event), first-hand extraction from the raw fleet log
`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` (281 bf-mje3pd
events, JSON-parsed), the bead store, and live journal/git checks.
**Automated classification unavailable:** `crash-classifier.sh bf-mje3pd` →
`ERROR: Bead trace not found` — per-bead traces are single-slot and this one is gone
(no `.beads/traces/bf-mje3pd/`; the Sep-1 `.beads/logs/` era starts far too late for an
Aug-13 crash). Classified from the raw worker log instead, per the documented
classifier-window limits.

---

## Classification

### Crash type: `INFRASTRUCTURE` — repository-bloat regime sub-type

Per the guide, exit −1 maps to **Infrastructure event**. The regime refinement is
**repository bloat**: this workspace was in the documented 18 GB `.git` / ~17 GB
loose-objects state on 2026-08-13, and bf-mje3pd's task was itself the bloat-prevention
fix ("Implement fix and verify agent crash prevention").

**Alert disposition: the crashes were real and mid-task — not false positives at the
time.** At the named instant the bead was 4 attempts into a 14-attempt loop, so the
guide's "work completed within 30 s → FALSE POSITIVE" branch does **not** apply.
The disposition today is **stale, not false**: bf-mje3pd completed and closed, so the
five still-open alert beads pointing at it (`bf-56kmlk`, `bf-1cezsk`, `bf-1pidqn`,
`bf-3dxljn`, `bf-x88dnf`) warrant no action. Two different questions, both answered:
what killed the worker (infrastructure) vs what the alert warrants now (nothing).

### Provenance of the task's input timestamp — it is a heartbeat, not a kill

The task's named instant `2026-08-13T19:18:56.178560656+00:00` is **not a crash
instant**. It is the `Timestamp:` field of alert bead **`bf-56kmlk`** (created
19:18:56.190600211Z, label `failure-count:5`), and the instant itself is a
`heartbeat.emitted` / `HANDLING_RELEASE_DONE` record (19:18:56.178350930) emitted
**12.9 s after** attempt 4's actual kill (`outcome.classified exit=-1` at
19:18:43.233). The alert generator read the clock microseconds after the heartbeat
fired. Six other bf-mje3pd alert beads carry the same shape, each stamped at its own
release heartbeat (bf-1y1d0g 19:03:21, bf-1pidqn 19:15:29, bf-1cezsk 19:32:58) —
one alert bead per kill under pre-0.4.2 needle, none of them a distinct crash.

## Verified crash facts (first-hand from the raw log)

14 dispatches, 13 completions, 12 classified outcomes; 18:53:50Z first claim →
21:18:36Z `bead.orphaned` (2h24m46s).

| # | Dispatch | Kill / completion | Exit | Outcome |
|---|---|---|---|---|
| 1 | 18:53:50 | 19:03:11 | −1 | crash |
| 2 | 19:03:26 | 19:10:10 | 1 | failure |
| 3 | 19:10:30 | 19:15:17 | −1 | crash |
| 4 | 19:15:36 | **19:18:43** | **−1** | crash — *the kill behind this task's input timestamp* |
| 5 | 19:19:04 | 19:21:55 | −1 | crash |
| 6 | 19:22:08 | 19:27:13 | 1 | failure (+handling timeout 19:28:48) |
| 7 | 19:28:51 | 19:32:37 | −1 | crash |
| 8 | 19:33:08 | 19:36:39 | −1 | crash (+handling timeout 19:37:29) |
| 9 | 19:37:41 | 19:42:59 | 1 | failure |
| 10 | 19:43:42 | 19:43:53 | — | mitosis eval, completed with **no** classified outcome |
| 11 | 19:43:56 | 19:46:33 | −1 | crash → `outcome.handled action=alerted` 19:47:00 |
| 12 | 19:47:03 | *no record* | — | death bracketed: `peer.crashed` for this worker at 20:36:55Z (49m52s in); worker died with it |
| 13 | 21:00:14 | 21:10:14 | 124 | timeout (600 s) → deferred |
| 14 | 21:10:33 | 21:18:23 | 0 | success → `verification.passed` → `bead.orphaned` |

- **7 × exit −1 (crash), 3 × exit 1, 1 × 124, 1 × 0**, one unclassified mitosis-eval
  completion, one dispatch with no completion record at all. Needle re-claimed within
  3–15 s after every kill.
- **exit −1 is needle's sentinel for death by signal, not a signal number.** The
  specific signal for these kills is **not recoverable**: the single boot in
  `journalctl --list-boots` starts 2026-08-15 19:56:33 EDT, so the Aug-13 kernel and
  journald records are gone (the same loss documented for the Aug-12 storms). The
  corpus's `docs/crashes/signal-minus1-root-cause-analysis-verified-2026-09-02.md`
  states exit −1 *is* SIGHUP; that universal claim is superseded — verified memcg-OOM
  kernel records exist for other same-era events (bf-198ne, bf-1ea4g), which are
  SIGKILL, not SIGHUP.

## Bead status check (acceptance criterion 2)

bf-mje3pd is **Closed** (rev 2, `closed_at` 2026-08-17T00:15:35Z) with reason:
*"Agent crash investigation and recovery complete. Analysis shows the crash during
bf-4yjq execution was a transient issue — the git remote configuration work was
successfully completed and verified."*

**No work was lost.** The deliverable landed on the 14th dispatch, and two of the
killed attempts had already committed seconds before their kills (both reachable only
from `pre-squash-history-20260816`; content live at HEAD via later landings):

- `ea23bd1` — 19:02:32Z, 39 s before attempt 1's kill
- `164b62d` — 19:21:41Z, 14 s before attempt 5's kill

## System state at crash time (acceptance criterion 3)

**No direct telemetry survives.** Verified absent: journal floor 2026-08-15 (above);
`.beads/logs/` earliest entries 2026-09-02. Three indirect lines pin the regime:

1. **Workspace-local, not box-wide.** Same window (19:00–19:35Z), all six Aug-13
   worker logs: `lab-domain-check` 7 dispatches → 5 × exit −1 + 2 × exit 1, while
   `drawrace`/`roam-1`/`roam-2`/`s1`/`test-fix` took 44 dispatches with **28 exit-0
   successes and zero exit −1**. Whatever was killing this worker was scoped to this
   workspace's resource footprint.
2. **Storm context.** The same worker log records **344 exit −1 kills across 13
   distinct beads** on 2026-08-13 alone — bf-mje3pd's 7 kills are a slice of the
   Aug-13 bloat-era storm already documented for bf-4k2ws, bf-1ea4g, bf-4yjq and
   bf-1s6c3.
3. **Cadence match.** 3–6 min kill intervals with instant re-claims, on a bead doing
   git work in the 18 GB repo, is the Pattern-3 repository-bloat signature. The
   mechanism is therefore **regime-matched, not kernel-proven**: any figure like
   "pack-objects consumed 3–6 GB" is inference.

## Checked against prior records

- **`bf-1y1d0g`'s 2026-09-02 resolution note** (the incident-resolution report):
  conclusions correct, three figures wrong. First-hand: **7** exit −1 (not 9),
  success on the **14th dispatch** (not the 13th attempt), and its "git operations
  loaded 17 GB objects into memory → OOM → SIGKILL" is regime inference with no
  surviving kernel record for this bead. Its "2 × exit 1" is also short — there are
  3.
- **`docs/crashes/bf-mje3pd-crash-artifacts-analysis-domchk-a4cc1326-2026-09-07.md`**
  (sibling bead domchk-a4cc1326, InProgress/untracked at classification time): the
  census agrees on every classified outcome; its "2 × exit 0" counts the unclassified
  mitosis-eval attempt as a zero — the first-hand count is one classified exit 0 plus
  one attempt with no classified outcome.

## Answer

**INFRASTRUCTURE** (repository-bloat regime, mechanism regime-matched — no surviving
kernel record). Crashes genuine, mid-task, not workflow/service/code-defect. The bead's
work completed the same evening and the bead is closed; every open alert pointing at
bf-mje3pd is stale and can be retired on this evidence.
