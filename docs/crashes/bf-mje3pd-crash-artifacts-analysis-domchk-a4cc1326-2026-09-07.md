# bf-mje3pd — Crash Artifact Analysis

**Investigation bead:** domchk-a4cc1326 (2026-09-07)
**Target bead:** bf-mje3pd — "Implement fix and verify agent crash prevention"
**Method:** first-hand re-extraction from the raw fleet logs; prior reports checked
against that extraction, not cited on faith.

## Target bead state (checked before analysis)

bf-mje3pd is **Closed** (rev 2, updated 2026-08-17T00:15:35Z, assignee
`claude-code-glm-4.7-lab-domain-check`). Created 2026-08-13T18:25:38Z. The alert
premise ("exit code -1") is **real**: the crash loop happened on 2026-08-13, and
the bead's work completed the same evening. Any later alert pointing at this bead
is stale, not fabricated.

## Artifact inventory

**Available (used):**

| Artifact | Content |
|---|---|
| `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` | Primary record — 281 bf-mje3pd events (claims, dispatches, completions, outcome classifications, releases) |
| `~/.needle/logs/claude-code-glm-4.7-lab-drawrace-2026-08-13.jsonl` | `peer.crashed` observing the lab-domain-check worker at 20:36:55Z + two `mend.zero_activity_log_cleaned` records |
| `~/.needle/logs/claude-code-glm-4.7-lab-roam-1-2026-08-14.jsonl` | One further zero-activity log-clean record (2026-08-14T04:38:26Z) |
| Bead store | bf-mje3pd (Closed rev 2), alert bead bf-1y1d0g ("ALERT: Agent crash on bead bf-mje3pd", created 2026-08-13T19:03:21Z, rev 21) |
| Git objects | `ea23bd1`, `164b62d` — in-loop fix commits (see §Work survival) |

**Missing / destroyed:**

- **Agent transcript** `claude-code-glm-4.7-lab-domain-check-bf-mje3pd.agent.jsonl`
  — deleted by the `mend` zero-activity cleaner (deletions recorded at
  2026-08-13T20:36:55Z, 21:06:58Z, and 2026-08-14T04:38:26Z).
- **Kernel / journald records for the crash window.** The current boot's journal
  starts 2026-08-15 19:56:33 EDT — the Aug-14/15 reboot discarded everything
  earlier (same loss already documented for the Aug-12 storms). Nothing in the
  live journal can confirm or deny a memcg-OOM kill for this bead.
- **Per-bead trace**: `.beads/traces/` has 1,970 dispatch dirs at this
  re-verification (the count drifts upward as co-tenant workers dispatch), none
  for bf-mje3pd (single-slot retention predating the Sep-1 `.beads/logs/` era,
  which starts five days too late for this crash).

## Census (first-hand, from the primary log)

First claim 2026-08-13T18:53:50Z → terminal `bead.orphaned` 21:18:36Z:
**2h24m46s** end to end.

- **14 dispatches**, **13 recorded completions**, 13 claims, 9 releases,
  2 `worker.handling.timeout` ("timeout after 50s"), 1 `bead.mitosis.evaluated`,
  1 `verification.passed`, 1 `bead.orphaned`.
- Outcomes: **7 × exit -1 (crash)**, 3 × exit 1 (failure), 1 × exit 124
  (timeout, 600 s wall), 2 × exit 0, plus the 1 dispatch with no completion
  record (attempt 12 below). Handling: **6 × `outcome.handled action=alerted`**,
  2 × `worker.handling.timeout` ("timeout after 50s"), plus released / deferred /
  none for the failure / timeout / success outcomes.
- Attempts 1–12 dispatched under worker session `e29942f7`; 13–14 under
  `3bcc4996` (worker restarted in between). Twelve of the first 13 dispatches
  ran the `pluck-default` template with the same 70,921-byte prompt
  (`sha256:26e51b96…`); the exception is attempt 10, needle's own
  `mitosis` split evaluation (2,005-byte prompt, `sha256:f6f13187…`, 11 s,
  exit 0, `splittable: false`); the final attempt ran a `split` template with a
  3,119-byte prompt (`sha256:eb99be37…`).

## Timeline (UTC, 2026-08-13)

| # | Dispatch | Completed | Dur | Exit | Outcome |
|---|---|---|---|---|---|
| 1 | 18:53:50 | 19:03:11 | 560 s | -1 | crash |
| 2 | 19:03:26 | 19:10:10 | 403 s | 1 | failure |
| 3 | 19:10:30 | 19:15:17 | 287 s | -1 | crash |
| 4 | 19:15:36 | 19:18:43 | 186 s | -1 | crash |
| 5 | 19:19:04 | 19:21:55 | 171 s | -1 | crash |
| 6 | 19:22:08 | 19:27:13 | 305 s | 1 | failure (+handling timeout 19:28:48) |
| 7 | 19:28:51 | 19:32:37 | 226 s | -1 | crash |
| 8 | 19:33:08 | 19:36:39 | 211 s | -1 | crash (+handling timeout 19:37:29) |
| 9 | 19:37:41 | 19:42:59 | 318 s | 1 | failure |
| 10 | 19:43:42 | 19:43:53 | 11 s | 0 | mitosis eval (no classification) |
| 11 | 19:43:56 | 19:46:33 | 156 s | -1 | crash → `outcome.handled action=alerted` 19:47:00 |
| 12 | 19:47:03 | *no record* | ≥49m52s | — | death bracketed, see below |
| 13 | 21:00:14 | 21:10:14 | 600 s | 124 | timeout → deferred |
| 14 | 21:10:33 | 21:18:23 | 470 s | 0 | success (split) → `verification.passed` → `bead.orphaned` 21:18:36 |

**Attempt 12's missing completion** is bracketed, not imagined: the dispatch is
logged at 19:47:03Z, the lab-domain-check worker then goes silent, the
lab-drawrace worker records `peer.crashed` for it at **20:36:55Z** (49m52s into
the attempt), and the next claim in the primary log comes from a new session
(`3bcc4996`) at 21:00:14Z. The attempt died with its worker; needle never wrote
a completion or outcome record for it — the same capture-race shape noted in
`docs/crash-root-cause-domchk-4f0b8b43-2026-09-07.md` §4-C, so "no crash record"
here would not have meant "no crash".

## Exit code and signal

- Needle recorded `exit_code: -1` on 7 completions, classifying each as
  `outcome: crash`. Exit -1 is needle's sentinel for infrastructure-level
  process termination (death by signal, not an application exit) —
  `docs/signal-analysis-exit-code-negative-one.md`. It is not a signal number;
  the specific signal is **not recoverable** because the kernel/journald records
  for the window were lost to the Aug-14/15 reboot.
- **Mechanism is regime-matched, not kernel-proven.** The crash window sits in
  the repository-bloat memcg-OOM era (the 18 GB / 17 GB-loose-objects state
  documented for bf-1s6c3, bf-4yjq, bf-1ea4g, and bf-4k2ws on Aug-12/13), and
  the exit -1 bursts at 3–6 min intervals match that regime. Earlier reports
  state "OOM killer SIGKILL (signal 9)" and "pack-objects consumed 3–6 GB" as
  fact; for this bead those figures are inference from the regime — no kernel
  record survives to confirm them.

## Work survival — no work was lost

Two of the seven killed attempts committed their work seconds before the kill
landed (the per-attempt-commit localization pattern):

- `ea23bd1` (19:02:32Z, 39 s before attempt 1's kill): `.gitignore` +3,
  `scripts/check-repo-health.sh` +84.
- `164b62d` (19:21:41Z, 14 s before attempt 5's kill): `.gitignore` +9,
  `scripts/check-repo-health.sh` rewrite, `scripts/cleanup-repo-bloat.sh` +73,
  `scripts/pre-commit-repo-size-hook` +75, plus the `.needle-predispatch-sha`
  dispatch marker (+2/−1).

Both commits are reachable **only from `pre-squash-history-20260816`**, not from
`main` (the Aug-16 history squash dropped them from the main line). Their
content is present at HEAD via later landings — `scripts/check-repo-health.sh`,
`scripts/cleanup-repo-bloat.sh`, and `scripts/pre-commit-repo-size-hook` all
exist and are the maintained versions in use today. The "catch-22" framing in
the prior report (fixing bloat crashed on the bloat) is directionally right but
overstated: the crashed attempts were shipping the prevention scripts and being
killed on post-commit work, and the scripts survived.

## Checked against prior reports

- `docs/verification/bf-mje3pd-crash-analysis.md` (2ec91ec, domchk-9bc6579f):
  its 13-row attempt table matches this extraction line for line on all 13
  recorded completions. Corrections: (a) it omits the 14th dispatch (19:47:03Z,
  no completion record) — the true census is 14 dispatches / 13 completions, not
  "11+ attempts"; (b) its "2 hours 15 minutes" measures first *crash* (19:03:11)
  → final success (21:18:23); first *dispatch* → final success is 2h24m46s;
  (c) its OOM signal/pack-objects figures are inference (see above).
- `docs/archive/crash-investigations/crash-context-bf-mje3pd-summary.md`
  (a883044; the doc lives in the archive now — path corrected here 2026-09-07):
  consistent; superseded in detail by the table above.
- Aug-26 alert verifications (`bf-1y1d0g`, `bf-3za7vh`, `bf-1cezsk`,
  `bf-x88dnf` — see `docs/bead-verification/`, `docs/archive/crash-investigations/`):
  all concluded stale alerts against an already-closed bead. Consistent with this
  extraction: the crashes were real, the work was finished the same evening, so
  any later alert is stale rather than false about the crash.
- `docs/notes/incident-resolution-bf-1y1d0g-bf-mje3pd-crash-2026-09-02.md`:
  consistent.

## Symptoms summary

An `exit -1` crash loop — 7 kills in 43m22s (19:03:11–19:46:33Z) at 3–6 min
intervals, one 50-minute silent death with the worker itself (attempt 12), one
600 s timeout — on a bead implementing repository-bloat prevention during the
Aug-13 memcg-OOM regime, with needle auto-retrying each kill within seconds
(release → re-claim gaps of 2–18 s, median 3 s), a first alert bead (bf-1y1d0g)
generated 10 s after the first crash classification, and the bead reaching
`outcome: success` only after the 14th dispatch. Zero accepted-code loss: both
in-loop commits survived, and their content is live at HEAD.

## Verification record (domchk-a4cc1326, this attempt)

The report above was written by an earlier attempt of this bead and left
uncommitted; this attempt re-derived every load-bearing figure first-hand from
the raw logs before committing it. Re-executed 2026-09-07 ~20:2x–20:4xZ against
the same artifacts:

- **Re-extracted from `claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl`:**
  281 bf-mje3pd events; 14 dispatches / 13 completions / 13 claims / 9 releases;
  exit distribution 7 × -1, 3 × 1, 1 × 124, 2 × 0; every dispatch and completion
  timestamp in the table above matched to the second; sessions and prompt
  hashes/lengths matched. 2h24m46s first-claim → orphaned confirmed
  (18:53:50.48Z → 21:18:36.32Z).
- **Corrections applied by this attempt:** attempt 10's prompt was a 2,005-byte
  `mitosis` template (`f6f13187`), not the shared pluck prompt — the earlier
  text claimed all of attempts 1–13 ran pluck; the kill burst is 43m22s, not
  "53 minutes"; release → re-claim gaps measure 2–18 s (measured per pair), not
  "3–15 s"; traces-dir count refreshed to 1,970; the
  `crash-context-bf-mje3pd-summary.md` citation updated to its archived path.
- **Confirmed unchanged:** `peer.crashed` for bf-mje3pd at 20:36:55.03Z and the
  three `mend.zero_activity_log_cleaned` deletions of
  `…-bf-mje3pd.agent.jsonl` (20:36:55Z, 21:06:58Z, 2026-08-14T04:38:26Z); both
  handling timeouts ("timeout after 50s", 19:28:48 / 19:37:29); 6 ×
  `action=alerted` with the first at 19:03:24Z; alert bead bf-1y1d0g created
  19:03:21.49Z (10.5 s after the first crash classification); `verification.passed`
  (gates_run 1) 21:18:23Z; `bead.orphaned` 21:18:36Z; journal floor
  2026-08-15 19:56:33 EDT (single boot, first entry = socat startup); commits
  `ea23bd1` / `164b62d` with the stated diffstats and timestamps, reachable only
  from `pre-squash-history-20260816`; no bf-mje3pd directory under
  `.beads/traces/`.
- **Cross-checked:** the 13 completion rows of
  `docs/verification/bf-mje3pd-crash-analysis.md` (added 81614ac,
  domchk-9bc6579f; restored at 2ec91ec) match this extraction line for line.
