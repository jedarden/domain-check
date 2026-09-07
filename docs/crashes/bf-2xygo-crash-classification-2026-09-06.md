# Crash Classification: bf-2xygo (2026-08-12)

**Classification bead:** domchk-fe1e4a60
**Date classified:** 2026-09-06
**Method:** `docs/crash-response-guide.md` — Quick Reference exit-code table, False Positive
Detection Heuristics (Rules 1–3), Common Crash Patterns (Patterns 1 & 3)
**Evidence basis:** committed raw-event extract `docs/crash/bf-2xygo/raw-logs/` (91 events,
commit fb7bedd) re-checked against the surviving source log
`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl` (3,589,712 bytes,
unchanged since Aug-12), plus live recounts across all six Aug-12 agent logs

---

## Classification

### Crash type: `infrastructure-event` — repository-bloat sub-type

Per the guide's Quick Reference, exit −1 maps to **Infrastructure event**, and the
"fixed-cadence re-dispatch deaths + `.git` > 5GB" row refines it to **Infrastructure:
Repository bloat (Pattern 3)**. bf-2xygo satisfies that signature on every criterion that is
still checkable. **Alert disposition: false-positive / no action** — the subject bead
completed and closed the same evening (see False-Positive Checks below). The two answers are
different questions: what killed the worker (infrastructure) versus what the alert warrants
today (nothing — the retry already healed it).

This converges with the bead-notes classification of domchk-d24a604d (2026-09-02, never
committed): "FALSE POSITIVE — Infrastructure Event (OOM from repository bloat)".

## Verified crash facts

| Attempt | agent.completed (UTC) | Exit | Duration | Basis |
|---|---|---|---|---|
| 1 | 21:18:21.891 | −1 | 196,226 ms | raw event, re-verified from extract |
| 2 | 21:21:25.960 | −1 | 174,755 ms | raw event |
| 3 | 21:24:44.890 | −1 | 188,795 ms | raw event |
| 4 | 21:28:24.006 | −1 | 209,674 ms | raw event |
| 5 | 21:31:21.362 | **0** | 167,541 ms | raw event; `verification.passed` → `bead.completed` |

- All five attempts: identical dispatch (`prompt_len=70650`, template `pluck-default`)
- `exit_code=-1` is needle's `wait()` sentinel for **died by signal** — not a signal number
  (repo canon: `docs/crashes/signal-minus1-root-cause-analysis-verified-2026-09-02.md`)
- Bead closed 21:30:57.461Z, 24 s **before** attempt 5's process exit — the worker closed the
  bead itself, then shut down cleanly (no release-conflict death; exit was 0, not −1)
- Task: fetch + compare Forgejo/GitHub remotes. Deliverable in main as
  `docs/notes/forgejo-github-sync-analysis-2026-08-26.md` (3838a30, 2026-08-26); attempt 5
  itself committed nothing (zero main-branch commits 2026-08-09 → 2026-08-12 22:00Z,
  re-verified live via `git log --until`)

## Guide criteria applied

### 1. Exit-code mapping → infrastructure

Four signal deaths, zero exit-code variation, no `error_max_turns`, no HTTP 5xx. The mapped
class is infrastructure; the alternates are excluded below.

### 2. Pattern 3 (repository bloat) signature

| Pattern 3 criterion | bf-2xygo | Status |
|---|---|---|
| Fixed-cadence re-dispatch deaths, minutes apart, for hours | 4 deaths at ~3.2-min cadence (196s/175s/189s/210s runs), needle re-claimed within ~10 s each time | ✅ verified from raw events |
| Repository > 5GB | `.git` ≈ 18 GB with ≈ 17 GB loose objects at crash time | ✅ canon-sourced (below) |
| Routine git operations trigger the kill | Task's own operation class was `git fetch` of both remotes on the bloated repo | ✅ by task definition |
| Zero exit-code variation | All four deaths exit −1 | ✅ verified |
| Sits inside a same-mechanism storm | Between bf-4yjq's 50 kills (17:54–20:30) and bf-1s6c3's 49 kills (from 21:36), same evening | ✅ verified from day log |

**Epistemic status of the repo-size criterion:** the 18 GB figure is contemporaneous
(bf-2igib close reason, 2026-08-16; `docs/crashes/repository-bloat-crash-bf-1s6c3-2026-08-12.md`)
but is **not re-measurable** — the repo was repaired to 94 MB and re-verified 2026-09-06. The
signal-level kill mechanism (memcg `CONSTRAINT_MEMCG` SIGKILL, as proven for the later
bf-198ne) is **unverifiable for Aug-12 specifically**: journald on this box begins
2026-08-15 19:46 EDT, so no kernel records exist for that day. The classification rests on the
Pattern-3 signature, not on a kernel record — which is exactly why the guide provides the
signature as a detection heuristic.

### 3. Excluded alternates

- **workflow-failure** — requires exit 1 + `error_max_turns`. All four deaths were signal
  deaths; `transform.completed` succeeded on every attempt (29/29/28/35 events).
- **service-failure** — requires HTTP 503/502 to the inference gateway. No 5xx recorded in any
  attempt; no gateway-failure signature in the day log.
- **code-defect** — no application error, panic, or domain-check code in the failure path (the
  task never executed application code). Consistent with 157+ investigations of this workspace
  finding zero domain-check defects.

## False-positive checks (guide heuristics)

- **Rule 1 (commit < 30 s before crash): does not fire.** Last commit before the window is
  00117cb at 2026-08-09T17:00:56Z — four days earlier. The deaths were **mid-task**, not
  post-completion cleanup. Pattern 1 (post-completion, ~40% of crashes) does not match.
- **Rule 2 (crash → retry → success): fires.** Attempt 5 exited 0 with `verification.passed`
  and the bead closed the same minute-window. Per the guide this is a **self-healed transient
  failure**: the fleet's own retry resolved it; re-dispatch was never needed.
- **Closed-bead filter (alert fixes 1 & 5): fires today.** bf-2xygo is CLOSED with its
  deliverable on main. An alert re-fired now is a duplicate of a resolved crash — it has been
  independently investigated at least 8 times across 6 beads (bf-36tp5, bf-2igib, bf-2lxwt,
  domchk-71e698fe, domchk-3042abf1, domchk-d24a604d, domchk-cd364e0a, this bead).
- **Rule 3 (surge: 10+ crashes in 10 minutes): does not fire for this window.** Live recount
  across all six Aug-12 agent logs (`domain-check`, `drawrace`, `roam-1`, `roam-2`, `s1`,
  `test-fix`): 21:15–21:32 UTC contains **exactly 4** exit-−1 deaths — all bf-2xygo's own. The
  day's 455 kills were concentrated, not synchronized: bf-31mno 350, bf-4yjq 50, bf-1s6c3 49,
  bf-2xygo 4, two singles. The surge threshold was crossed **once** that day — 11 kills in a
  trailing 10-minute window ending 10:49 UTC (bf-31mno morning storm) — nowhere near this
  window. bf-2xygo's crashes were bead-local re-dispatch deaths inside a repo-bloat storm, not
  a fleet-wide wave, so no separate system-event alert is warranted.

## Corrections to prior records (found during this classification)

1. **`docs/crash/bf-2xygo/raw-logs/README.md` is wrong that "the cited primary log contains no
   load or CPU metrics of any kind."** The Aug-12 log contains 545 `fleet.cpu_saturated` events
   carrying `{load_average, core_count, threshold}` and 82 `worker.launch.deferred` events
   whose reasons quote measured 1-minute load (e.g. "10.79 / 9 cores = 1.20 > threshold 0.80").
   The Aug-25 doc's four load values (9.11 / 9.4 / 8.47 / 8.21) are therefore **real recorded
   launch-gate samples**, not fabricated numbers — each was written within ~6 ms of one of
   bf-2xygo's own dispatch events. The README's circularity point survives: the samples were
   taken at dispatch-evaluation moments, so the Aug-25 doc's "correlation between load and
   crashes" is partly mechanical, and saturation was chronic all day (events in every hour
   05–23), so it cannot by itself discriminate bf-2xygo's four deaths from the other 451.
2. **The Aug-25 doc's CPU-saturation mechanism claim remains superseded** (per the plan's RCA
   and the corrected canon): needle's gate throttling evidences CPU pressure, but the
   established kill mechanism for git operations on the bloated repo is memcg OOM at the
   dispatch-scope bound. For Aug-12 the mechanism is unverifiable (no kernel records — see
   above); the Pattern-3 signature is the available and sufficient basis.

## Action required

- **No code changes.** No new mitigation needed: the cause (repository bloat) is repaired and
  held at 94 MB (re-verified 2026-09-06), `.beads/` is structurally closed (0 tracked files),
  pack-operations are memory-bounded repo-locally and globally
  (`./scripts/setup-git-gc-config.sh --verify` → ✅), and the six systemd monitoring timers are
  firing.
- The narrative crash-analysis write-up remains owned by the open sibling bead
  **domchk-49962f7e**; this record supplies its classification layer so that bead can cite
  rather than re-derive.
- Close the alert: subject bead completed and closed 2026-08-12; crash type recorded here.
