# bf-mje3pd — Root Cause Analysis and Classification

**Investigation bead:** domchk-1b407bff (2026-09-07)
**Target bead:** bf-mje3pd — "Implement fix and verify agent crash prevention"
**Crash date:** 2026-08-13 (18:53:50Z first claim → 21:18:36Z `bead.orphaned`, 2h24m46s)
**Depends on:** domchk-a4cc1326 (crash artifact analysis, `9b83c81`) — its extraction was
re-derived independently by this bead; every load-bearing figure below was verified
first-hand from the raw artifacts during this dispatch, not carried over.
**Sibling records folded in:** domchk-bf8c4fd3 (classification, `63ca904`),
domchk-916a1e66 (evidence compilation, `bfef974` — untracked when this RCA was
drafted, committed while it was in flight), domchk-a6059a67 (bf-4yjq chain report,
`f5e6377` — landed the same hour, same classification).

## Answer

**INFRASTRUCTURE — repository-bloat regime sub-type.** Exit −1 death-by-signal kills of
the agent, 7 of them in a 43m22s burst, on a bead doing git-heavy work in this
workspace during the documented Aug-13 bloat era (~18 GB `.git` / ~17 GB loose
objects). The mechanism is **regime-matched, not kernel-proven**: no kernel or journald
record survives for the window. Not a false positive at the time — the work was 4
attempts into a 14-attempt loop at the named instant — and **stale, not actionable,
today**: the bead reached `outcome: success` with `verification.passed` the same
evening and closed 2026-08-17, so all five still-open alert beads pointing at it
warrant no action.

## 1. Classifier run (acceptance criterion 1)

```
$ ./scripts/crash-classifier.sh bf-mje3pd
ERROR: Bead trace not found: .beads/traces/bf-mje3pd/trace.jsonl   (exit 2)
```

The automated classifier is **unavailable** for this crash, on three independent
retention floors, all verified live:

- `.beads/traces/` is single-slot — 1,970 dispatch dirs at this re-check, none for
  bf-mje3pd (Aug-13 predates the Sep-1 `.beads/logs/` era).
- `.beads/logs/` earliest entries are 2026-09-01 — five days too late.
- `journalctl --list-boots` shows a **single boot** whose first entry is
  2026-08-15 19:56:33 EDT — the Aug-14/15 reboot discarded every kernel record for
  the crash window.

Classification therefore comes from the raw worker log
(`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl`, JSON-parsed
by `event_type` — grep overcounts, since each crash surfaces on both
`outcome.classified` and `outcome.handled`), per the documented classifier-window
limits. Had a trace survived, the guide's decision tree would land in the same place:
exit −1 → **Infrastructure event** (`docs/crash-response-guide.md` line 15), refined
to **Infrastructure: Repository bloat** by the fixed-cadence re-dispatch shape
(line 16, Pattern 3), with line 28's standing note that exit −1 is a sentinel for
death by signal, not a signal number.

## 2. Specific failure mode

**Death by unrecorded signal (needle `exit_code: -1`) during git-heavy agent work, in
a fixed-cadence auto-retry loop, inside the memcg-constrained dispatch scope of a
repository in the bloat regime.** The loop, first-hand:

| # | Dispatch | Completed | Dur | Exit | Outcome |
|---|---|---|---|---|---|
| 1 | 18:53:50 | 19:03:11 | 560 s | −1 | crash |
| 2 | 19:03:26 | 19:10:10 | 403 s | 1 | failure |
| 3 | 19:10:30 | 19:15:17 | 287 s | −1 | crash |
| 4 | 19:15:36 | **19:18:43** | **186 s** | **−1** | crash — *the kill behind the task's input timestamp* |
| 5 | 19:19:04 | 19:21:55 | 171 s | −1 | crash |
| 6 | 19:22:08 | 19:27:13 | 305 s | 1 | failure (+handling timeout 19:28:48) |
| 7 | 19:28:51 | 19:32:37 | 226 s | −1 | crash |
| 8 | 19:33:08 | 19:36:39 | 211 s | −1 | crash (+handling timeout 19:37:29) |
| 9 | 19:37:41 | 19:42:59 | 318 s | 1 | failure |
| 10 | 19:43:42 | 19:43:53 | 11 s | — | mitosis eval, completed with no classified outcome |
| 11 | 19:43:56 | 19:46:33 | 156 s | −1 | crash → `outcome.handled action=alerted` 19:47:00 |
| 12 | 19:47:03 | *no record* | ≥49m52s | — | death bracketed: `peer.crashed` for this worker at 20:36:55Z; worker died with it |
| 13 | 21:00:14 | 21:10:14 | 600 s | 124 | timeout → deferred |
| 14 | 21:10:33 | 21:18:23 | 470 s | 0 | **success** → `verification.passed` → `bead.orphaned` 21:18:36 |

Census (first-hand, JSON-parsed on `event_type`): 281 events, **14 `agent.dispatched`**,
13 `agent.completed`, **12 `outcome.classified` = 7 × exit −1 (crash), 3 × exit 1
(failure), 1 × 124 (timeout), 1 × 0 (success)**. Nine `bead.released` →
`bead.claim.succeeded` pairs measure **1.9–17.6 s** re-claim gaps — needle retried each
kill almost instantly. Attempts 1–12 ran under worker session `e29942f7`; 13–14 under
`3bcc4996` (the worker itself died with attempt 12 and was restarted). The specific
signal is **not recoverable** (journal floor above); it is not SIGHUP by evidence —
see §7.

## 3. Root cause chain

1. **Regime (necessary condition):** this workspace's `.git` was in the ~18 GB /
   ~17 GB loose-objects bloat state on 2026-08-13 — the era documented for bf-1s6c3,
   bf-4yjq, bf-1ea4g and bf-4k2ws. Git operations in that state drive large
   pack-objects working sets.
2. **Mechanism (regime-matched, not kernel-proven):** the agent's dispatch scope has a
   12 GiB memory bound; git-heavy attempts inside it were killed by signal —
   by elimination of the alternatives below, the memcg-OOM class is the only
   mechanism consistent with all surviving evidence. No kernel record survives to
   upgrade this from regime match to proof.
3. **Loop amplifier:** needle's auto-retry re-claimed the bead 1.9–17.6 s after every
   release, re-entering the same resource regime 14 times; two attempts died with the
   worker itself (attempt 12; `peer.crashed` 20:36:55Z) and one timed out at 600 s.
4. **Per-attempt commit localization** (the strongest direct evidence tying deaths to
   the regime): two of the killed attempts committed seconds before their kills —
   `ea23bd1` 19:02:32Z (39 s before attempt 1's kill) and `164b62d` 19:21:41Z (14 s
   before attempt 5's kill). Both are reachable **only from
   `pre-squash-history-20260816`** (dropped from main by the Aug-16 squash); their
   content is live at HEAD — `scripts/check-repo-health.sh`,
   `scripts/cleanup-repo-bloat.sh`, `scripts/pre-commit-repo-size-hook` all exist and
   are the maintained versions in use today. **Zero accepted-code loss.**
5. **Self-referential resolution:** the bead's task *was* the bloat prevention. The
   kills stopped when the regime did — the Sep-1 cleanup packed 18 GB → ~92 MB and the
   five-layer prevention (gitignore, 10 MB pre-commit gate, pack-memory bounds,
   safe-gc, timers) is the standing defense documented in repo CLAUDE.md. The scripts
   the killed attempts shipped are that defense's ancestors.
6. **Alert-layer root cause (why this bead exists at all):** pre-0.4.2 needle emitted
   one alert bead per kill with no lifecycle to retire it. Seven alert beads name
   bf-mje3pd (bf-1y1d0g, bf-1pidqn, bf-56kmlk, bf-3dxljn, bf-3za7vh, bf-x88dnf,
   bf-1cezsk — inventory in the sibling compilation), each stamped at its own release
   heartbeat; the target succeeded at 21:18:23Z the same evening and nothing retired
   the five that remain **Open today** (bf-56kmlk, bf-1cezsk, bf-1pidqn, bf-3dxljn,
   bf-x88dnf — statuses re-read live). The Sep-7 cooldown work (`f21e381`) addresses
   the rate; the closed-bead filter (FIX 1/5) addresses exactly this residue.

## 4. Evidence correlation (why infrastructure, and why this regime)

Three independent lines, each re-verified first-hand by this dispatch:

1. **Workspace-local, not box-wide.** Across all six Aug-13 worker logs, the five peer
   workers (drawrace, roam-1, roam-2, s1, test-fix) logged **zero** `exit_code: -1`
   lines in the 19:00–19:35Z crash window (2–4 across their whole days), while
   lab-domain-check logged 10 raw −1 lines there and **344 classified exit −1 crashes
   across 13 distinct beads** for the day (00:00:15Z → 23:59:58Z). Whatever was
   killing this worker was scoped to this workspace's resource footprint — the
   signature of repo-local bloat, not a host-wide OOM or network event.
2. **Cadence match.** Crash completions every 3–12 min across a 43m22s burst with
   1.9–17.6 s re-claims is the guide's Pattern-3 fixed-cadence re-dispatch shape
   (line 16), the same signature kernel-confirmed for the same era's bf-198ne and
   bf-1ea4g.
3. **Exit-code mix rules out the alternatives.** 3 × exit 1 (application-level
   failure, not a service 503/502 signature), 1 × 124 (wall-clock timeout), 7 × −1
   (signal death). Nothing here matches SERVICE_FAILURE (the gateway's 503/502
   signature is absent from the log's completions) or CODE_DEFECT (domain-check code
   is not in the failure path of any attempt; no attempt failed on an application
   error signature). The 2 × `worker.handling.timeout` ("timeout after 50s") are
   alert-handling stalls, not crashes.

**Provenance caveat kept visible:** "memcg-OOM" for *this* bead is inference from the
regime + signature, because the kernel records are gone. Statements like "pack-objects
consumed N GB" are not made here because nothing survives to support a number.

## 5. False positive assessment (acceptance criterion 5)

**Not a false positive. The alerts were real when raised; they are stale now.**

- **The task's named instant is a heartbeat, not a kill.** The input timestamp
  `2026-08-13T19:18:56.178560656+00:00` is alert bead bf-56kmlk's `Timestamp:` field,
  and the instant is a `HANDLING_RELEASE_DONE` heartbeat logged at
  19:18:56.178350930Z — matching to the microsecond, **12.9 s after** attempt 4's
  actual kill (`agent.completed` exit −1 19:18:43.227Z, `outcome.classified crash`
  19:18:43.2338Z). The alert generator read the release heartbeat's clock, not a
  crash instant.
- **The 30-second rule does not fire.** At the named instant the bead was 4 attempts
  into a 14-attempt loop; nothing had completed. The two pre-kill commits (39 s and
  14 s before their kills) were mid-loop work, not task completion — the guide's
  "work committed < 30 s before crash → FALSE_POSITIVE" branch targets
  post-completion cleanup, which is not this shape.
- **The work completed — 2h later, same evening.** `outcome: success` +
  `verification.passed` at 21:18:23Z on attempt 14; `bead.orphaned` 21:18:36Z.
- **Target bead is Closed.** bf-mje3pd, rev 2, `closed_at` 2026-08-17T00:15:35Z.
  Every alert pointing at it after that date is stale.
- **Close-reason data-quality note (new, this bead).** The recorded close reason
  verifies **bf-4yjq's git-remote configuration** ("origin points to Forgejo … both at
  commit a245b38 … push mirror operational … bf-4yjq closed as complete") and says
  "no additional preventive measures needed" — it never mentions the prevention
  scripts this bead's own task called for. The closer worked three days after the
  loop, after the Aug-16 squash had dropped `ea23bd1`/`164b62d` from main, so the
  mid-loop deliverable was invisible to them on the main line. The close is
  substantively right — the preventive scripts exist and are maintained at HEAD — but
  its text addresses a sibling bead's criteria. Triage reading this close reason
  should not conclude the bead shipped nothing; the deliverable is in git, not in the
  close reason.
- **Disposition:** five Open alert beads (bf-56kmlk, bf-1cezsk, bf-1pidqn, bf-3dxljn,
  bf-x88dnf) can be retired on this evidence — crashes genuine, work complete, target
  closed. bf-1y1d0g (Closed 2026-09-02) and bf-3za7vh / bf-x88dnf / bf-1cezsk's
  verification reports already reach the same conclusion. One nuance: bf-1cezsk has an
  in-flight final-verification child (domchk-40cd5fde, noted by `f5e6377`), so that one
  retires through its own leg rather than this report.

## 6. Checked against prior records

- **domchk-a4cc1326 (dependency, `9b83c81`):** its census matches this bead's
  independent parse on every classified outcome. Its "2 × exit 0" counted the
  unclassified mitosis-eval attempt as a zero — the first-hand count is **one
  classified exit 0 plus one attempt with no classified outcome** (12 classified, not
  13).
- **domchk-bf8c4fd3 (classification, `63ca904`):** classification agrees
  (INFRASTRUCTURE / repository-bloat regime); this RCA re-derives its evidence and
  extends it with the close-reason finding in §5.
- **domchk-a6059a67 (bf-4yjq chain report, `f5e6377`):** same classification, same
  14-dispatch / 7-kill census, same sentinel-not-SIGHUP and regime-matched-not-proven
  corrections; adds the chain view (bf-2o7nlw → bf-1ziy13 → bf-mje3pd) and the
  756 MB → 92 MB figure correction. No disagreement found.
- **`docs/notes/incident-resolution-bf-1y1d0g-bf-mje3pd-crash-2026-09-02.md`:**
  conclusion right, figures superseded — 7 exit −1 (not 9), success on the **14th**
  dispatch (not 13th attempt), 3 × exit 1 (not 2), and its "17 GB loaded into memory →
  SIGKILL" is regime inference, not a surviving kernel record.
- **`docs/archive/crash-investigations/` signal analysis:** the corpus's claim that
  exit −1 *is* SIGHUP (`signal-minus1-root-cause-analysis-verified-2026-09-02.md`) is
  superseded — verified kernel records for same-era exits −1 (bf-198ne, bf-1ea4g) are
  SIGKILL, and for this bead the signal is unrecoverable rather than known.
- **`docs/verification/bf-mje3pd-crash-analysis.md`:** its 13 completion rows match
  this extraction line for line; it omits the 14th dispatch (no completion record).

## 7. Verification record (domchk-1b407bff, this attempt)

Executed 2026-09-07 ~20:3x–21:0xZ, first-hand unless attributed:

- `crash-classifier.sh bf-mje3pd` → exit 2, "Bead trace not found" (§1).
- JSON parse of `claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` by
  `event_type`: 281 events; 14 dispatched / 13 completed / 12 classified; exits
  7 × −1, 3 × 1, 1 × 124, 1 × 0; success 21:18:23.232Z, `verification.passed`
  21:18:23.247Z, `bead.orphaned` 21:18:36.324Z; 9 release→claim gaps 1.9–17.6 s.
- Attempt 4 bracket: `agent.completed` −1 19:18:43.227Z → `outcome.classified` crash
  19:18:43.2338Z → `HANDLING_RELEASE_DONE` heartbeat 19:18:56.178350930Z == the task's
  named input instant to the microsecond (12.9 s gap).
- Peer-log sweep: all five other Aug-13 worker logs — zero `exit_code: -1` lines in
  19:00–19:35Z, 2–4 across their days; lab-domain-check 10 in-window raw lines, 344
  classified exit −1 across 13 beads for the day.
- `journalctl --list-boots`: single boot, first entry 2026-08-15 19:56:33 EDT;
  `.beads/logs/` floor 2026-09-01; no `.beads/traces/bf-mje3pd/`.
- `ea23bd1` (19:02:32Z) / `164b62d` (19:21:41Z): exist, contained only by
  `pre-squash-history-20260816`; the three scripts they introduced present at HEAD.
- Bead store: bf-mje3pd Closed rev 2, `closed_at` 2026-08-17T00:15:35.48082325Z,
  close reason read verbatim from `.beads/checkpoint/forensic.jsonl`; five alert beads
  re-read **Open** via `bead show`, bf-1y1d0g Closed.
- Guide citations re-checked at HEAD: `docs/crash-response-guide.md` lines 15, 16, 28,
  1026.
