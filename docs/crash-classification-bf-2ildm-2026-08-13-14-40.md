# Crash Classification: bf-2ildm @ 2026-08-13T14:40:42.628685942+00:00

**Classification dispatch:** domchk-2c792cc9 (2026-09-07)
**Crash target:** bf-2ildm — "Extract GitHub-specific commits" (CLOSED 2026-08-16T22:44:38Z)
**Alert bead carrying this stamp:** bf-66sw7c (created 2026-08-13T14:40:42.642Z)
**Reported exit code:** −1
**Alert-level verdict:** FALSE_POSITIVE (resolved target; no work lost)
**Kill-level verdict:** INFRASTRUCTURE — repository-bloat-era kill regime
**Confidence:** HIGH (per-attempt event index; no surviving kernel record for Aug-13)
**Classifier result:** UNKNOWN → resolved by manual artifact analysis (see §1)

This is the per-instant classification record for the alert whose carried stamp is
`2026-08-13T14:40:42.628685942+00:00`, companion to
`docs/crash-classification-bf-2ildm-2026-08-13-15-01.md` (the 15:01:52 instant,
bf-z15pix) and
`docs/archive/crash-investigations/crash-summary-bf-2ildm-timestamp-2026-08-13-13-44.md`
(the 13:44:20 instant). Same bead, same storm, same verdict.

## Verdict

Two levels, and they answer different questions:

| Level | Classification | Meaning |
|-------|----------------|---------|
| **Kill** (what killed attempt 20) | **INFRASTRUCTURE** — repository-bloat sub-type | Real mid-task death, exit −1, in the Aug-13 bloat-era kill regime |
| **Alert** (bf-66sw7c's claim that work was lost) | **FALSE_POSITIVE** | Target bf-2ildm completed on 2026-08-16 and closed successfully; nothing needed re-doing |

The kill was real; the alarm was not. Both findings hold at once, and for bf-2ildm
they do — identical to the 15:01:52 companion classification and to bf-1ea4g's
two-level structure.

## 1. The classifier returned UNKNOWN, and why that is the expected result

`./scripts/crash-classifier.sh bf-2ildm` (run 2026-09-07) returns **UNKNOWN /
"Insufficient data to classify"**, with an explicit provenance warning:

> trace slot holds a run captured 2026-08-16T22:28:44.172164374Z, exit_code 0 …
> provenance cannot be established … Do not quote this trace as crash evidence.

That is not a classifier malfunction — it is the **single-slot retention blind
spot** this bead is the textbook case of. `.beads/traces/bf-2ildm/` holds only the
last attempt, which is the *successful* Aug-16 retry (exit 0, 85.3 s), so every
trace-derived pattern check is correctly skipped. The classifier's own "Next steps"
for UNKNOWN is "Manual investigation of crash artifacts," which is what the rest of
this record does, using the 2026-09-07 per-attempt event index.

## 2. The carried stamp is a handler heartbeat, not the death instant

Per the per-attempt index (`docs/crashes/bf-2ildm/attempt-index.tsv` row for
attempt 20, cross-checked against `crash-alert-ledger.tsv` and the alert bead's
own description):

- **Real death of the attempt this alert names: 2026-08-13T14:40:29.551762239Z**
  (attempt 20 — claimed 14:38:24.455Z, killed after 124,759 ms ≈ 2 m 05 s, mid-task).
- **Carried stamp 14:40:42.628Z is 13.1 s AFTER that kill**, and 14 ms before the
  alert bead's creation (14:40:42.642Z) — the crash handler's post-kill heartbeat.
- Uniform across the bead: all 38 alert stamps sit 7.7–29.4 s after their attempt's
  kill. No dispatch-named "crash instant" on bf-2ildm is a death time.

The 14:00Z hour alone holds **15 consecutive kills** (attempts 9–23, 14:04:19Z →
14:56:04Z), roughly one every 3–4 minutes — the storm's densest hour, and this
alert (attempt 20) sits in the middle of it.

## 3. Exit code −1 and the kill regime

Exit −1 is needle's sentinel for a signal death with no recorded exit code
(`code().unwrap_or(-1)`), not a signal number; the alert text's "signal -1" echoes
the sentinel. A genuine SIGKILL encodes 137. No kernel record can upgrade the
sentinel here: journald on this host has a single boot beginning 2026-08-15
19:56:33 EDT — **no Aug-13 kernel line survives for any bead**, so the memcg-OOM
mechanism (SIGKILL inside the 12 GiB per-dispatch scope) is assigned **by regime
match** from the same morning's proven sibling kills, not by a direct record.

The regime match is unambiguous. bf-2ildm's Aug-13 series: 43 attempts —
**38 exit −1 signal deaths** (13:35:34Z → 15:53:48Z, fixed ~2–5 min re-dispatch
cadence, durations 98–309 s, all mid-task), 4 exit-124 600 s caps (attempts 39–42),
1 exit-1 19 ms immediate failure (attempt 43, quarantined at 16:35:40Z,
`failure_count: 5`). Fixed-cadence exit −1 re-dispatch deaths are
`docs/crash-response-guide.md`'s **"Infrastructure: repository bloat"** sub-type
(Pattern 3): the trigger is the repository's own size — 18 GB of loose objects,
`.beads/` snapshots still tracked, a 422-commit unpushed backlog — so the kill
recurs on every dispatch until the repo is cleaned. The same-day census (commit
`d9c4622`, R5) attributes 38 exit-minus-one kills to bf-2ildm alongside
bf-65lsdu 127, bf-1ea4g 56, bf-4k2ws 55, and bf-1s6c3 22.

## 4. Why the alert is a false positive

- **Target state:** bf-2ildm is CLOSED (2026-08-16T22:44:38.873Z, by system, rev 7)
  with reason "Analysis complete: GitHub and Forgejo repos are fully synchronized at
  commit 9656fc4. Zero GitHub-specific commits found." The successful retry's trace
  metadata shows `exit_code: 0, outcome: success`, captured 2026-08-16T22:28:44Z.
- **Dedup gate:** `./scripts/alert-deduplication.sh check bf-66sw7c` →
  `DUPLICATE: crash target bf-2ildm is already resolved` (exit 0).
- **Nothing was lost:** 38 killed attempts cost ~2.3 hours of wall time, not work.
- **Prior verification:** `docs/verification/verification-report-bf-66sw7c-false-positive-alert-resolved-bf-2ildm-crash.md`
  (2026-08-26) already reached FALSE POSITIVE for this exact bead and recommended
  closing it — which did not happen; the alert bead stayed open until this chain.

## 5. Dated correction to the 2026-08-26 verification report's timing claim

That report's Root Cause §3 says the crash alert "was generated **after**
completion (2026-08-13, but bead closed 2026-08-16)." The word "after" is wrong in
the other direction from the 2026-09-02 corpus's "3+ days BEFORE completion" — both
anchor on the wrong instant. The precise ordering is:

1. Attempt 20 killed 2026-08-13T14:40:29.551Z (bead open, mid-task).
2. Handler heartbeat 14:40:42.628Z → alert bead created 14 ms later.
3. Target completed and closed 2026-08-16T22:44:38Z.

The alert was generated while the target was open and being killed; it became stale
when the target later succeeded. The alert-level FALSE_POSITIVE verdict, this
report's recommendation to close bf-66sw7c, and the systemic fixes (closed-bead
filtering, dedup, 7-day history window) all stand unchanged.

## 6. Sources

- `docs/crashes/bf-2ildm/attempt-index.tsv` (attempt 20 row), `crash-alert-ledger.tsv`
  (bf-66sw7c row), `trace-archive-current-state.json` — extracted 2026-09-07
  (untracked sibling bundle, owning chain domchk-ea755548)
- `./scripts/crash-classifier.sh bf-2ildm` — live run 2026-09-07 (UNKNOWN + provenance warning)
- `./scripts/alert-deduplication.sh check bf-66sw7c` — live run 2026-09-07 (DUPLICATE, exit 0)
- Live store: `bead show bf-2ildm` (closed, rev 7), `bead show bf-66sw7c` (open, rev 27),
  `.beads/checkpoint/forensic.jsonl` (bf-2ildm close event 2026-08-16T22:44:38.873Z)
- `docs/crash-response-guide.md` — classification table, exit −1 sentinel note (2),
  repository-bloat sub-type
- `docs/verification/verification-report-bf-66sw7c-false-positive-alert-resolved-bf-2ildm-crash.md`
  — prior 2026-08-26 FALSE POSITIVE verification for this bead
- Companion instant records: `docs/crash-classification-bf-2ildm-2026-08-13-15-01.md`
  (bf-z15pix, attempt 24), `docs/archive/crash-investigations/crash-summary-bf-2ildm-timestamp-2026-08-13-13-44.md`
  (bf-1wkda, attempt 3)
- Commit `d9c4622` (domchk-508e54c0) — Aug-13 per-bead kill census

## 7. Disposition

- **bf-66sw7c:** classify FALSE_POSITIVE at the alert level, close with the
  same reason convention as sibling alert bf-26r8bi (attempt 21, closed
  2026-08-16T23:11:35Z) — target resolved, no work lost. No implementation work.
- **No investigation needed** (FALSE_POSITIVE branch): the kill-level regime is
  already fully documented in the maintenance guide; the repo-side cause is
  repaired and holding (94 MB, re-verified 2026-09-06).
- Sibling open alert beads from this storm belong to their own verify-and-close
  chains, not this one.
