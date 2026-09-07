# bf-1s6c3 Crash Storm — Primary-Source Timeline and Corrections

**Investigation date:** 2026-09-06
**Bead:** bf-1s6c3 ("Create merge commit reconciling Forgejo and GitHub histories") — CLOSED 2026-08-16T14:00:13Z
**Dispatch bead:** domchk-1fb4ad35
**Primary sources:** `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl`, `-2026-08-13.jsonl` (worker log, UTC), `.beads/checkpoint/forensic.jsonl` (close event), live `git` inspection

This note re-derives the bf-1s6c3 crash record from the worker logs rather than from
the earlier investigation docs. Several widely-cited figures for this bead are wrong;
the corrections are below. All timestamps are UTC (needle logs) unless marked EDT.

## Corrections vs. the existing doc corpus

| Claim in existing docs | Primary-source finding |
|---|---|
| Single crash at `2026-08-12T21:36:51Z` (also cited as `2026-08-13T00:38:41Z` in another doc) | Those are the **first** and one **mid-storm** attempt of a continuous retry storm. Neither is "the" crash. |
| "9+ crashes over 2.5 hours" | **76 dispatches in 4h30m: 71 × exit −1 (crash), 4 × exit 124 (600 s timeout), 1 × exit 0 (success).** Understates by ~8×. |
| Merge commit `2832106`, remotes synchronized at `61d27ac` | Neither object exists in the repository. The real merge is **`42a7b07`** ("Merge reconciliation: Forgejo and GitHub remote histories", 2026-08-12T21:47:07Z). |
| Classification "FALSE POSITIVE — post-completion infrastructure event" | Each attempt died **mid-attempt** (exit −1 ~2–8 min in). The deliverable landed early and survived; the bead itself stayed open 4 more hours. See "What actually happened". |
| Close reason cites merge `7dd79eb` | Dead SHA — pre-squash name of `42a7b07` (identical subject and 21:47:07Z timestamp). Survives as `42a7b07` on branch `pre-squash-history-20260816`. |

## Storm census (worker log, bead bf-1s6c3 only)

Single worker (`claude-code-glm-4.7-lab-domain-check`), single session `8446529e`.

| Window (UTC) | Claims | exit −1 crash | exit 124 timeout | exit 0 success |
|---|---|---|---|---|
| Aug-12 21:31 → 23:57 | 50 | 49 | 0 | 0 |
| Aug-13 00:00 → 02:01 | 26 | 22 | 4 | 1 |
| **Total** | **76** | **71** | **4** | **1** |

- First recorded claim: `2026-08-12T21:31:27.663Z` (19 min after bead creation 21:12:09Z)
- First crash outcome: `2026-08-12T21:36:44.520Z` — `{"exit_code": -1, "outcome": "crash"}`
- Every one of the 76 attempts produced an `outcome.classified` event; there are no
  silent-death gaps — the classifier recorded each kill explicitly.
- Attempt durations: ~2–8 min while dying fast (memcg kill), switching to exactly
  600 s (`exit 124`) from 01:11Z, then the final attempt succeeded in 384 s.
- Storm end: `2026-08-13T02:01:22.561Z` — `{"exit_code": 0, "outcome": "success"}`

## What actually happened

1. **21:47:07Z — the deliverable landed mid-storm.** Merge commit `42a7b07`
   (parents `47e7758` + `00117cb`) was created by attempt #4 (claimed 21:43:20Z).
   That same attempt exited −1 at 21:48:06Z — 60 s after committing. The work
   product survived on disk; the agent did not.
2. **21:48Z → 02:01Z — 72 more dispatches** against an already-satisfied task.
   Each attempt redid expensive git work on the bloated 18 GB repo and was killed;
   none could close the bead because the kill landed before `bead close`.
3. **02:01:22Z — the storm's last attempt succeeded** and the loop stopped.
4. **2026-08-16T14:00:13Z — bead closed** (actor `system`), reason citing the merge
   by its then-current SHA `7dd79eb`.

So the event is neither a pure "crash during task" nor the docs' "post-completion
false positive": the deliverable landed 16 minutes into a 4.5-hour kill storm whose
remaining volume was pure re-dispatch against finished work.

## Post-crash history caveat — the merge is no longer in main

- `42a7b07` is a real 2-parent merge but is **not an ancestor of `main`**. The
  2026-08-16 history squash moved it onto branch **`pre-squash-history-20260816`**,
  which is its only containing ref.
- `main` does contain the **later** reconciliation: `46293c5` "Merge Forgejo and
  GitHub histories" (2026-08-17 06:21:43 EDT), reconciling the bf-4k2ws divergence
  (identical-content `.needle-predispatch-sha` commits `cf82b54` / `c87849c`).
- Any verification of bf-1s6c3's acceptance criterion ("local main contains the
  reconciled history") must therefore cite `46293c5`, not the bead's own close
  reason, and not the catalog docs' `2832106`.

## System state

**At crash time: unrecoverable.** journald on this box starts
`2026-08-15T19:46:33 EDT`; there are no kernel OOM records for Aug-12. The
"<2 GB available / >50 GB git spike" figures in earlier docs are reconstruction,
not kernel-proven. The memcg-OOM mechanism itself is established for the better-
instrumented later siblings (bf-4x12ec, bf-198ne — kernel `oom-kill` records
recovered; see `docs/crashes/bf-198ne-crash-report.md`).

**Bloat evidence no longer in the object store.** The largest surviving blob is
14,970,288 bytes (a Mach-O executable, 6 identical copies); no 237 MB `.beads/*.jsonl`
snapshot blobs survive. The "17+ identical 237 MB snapshots" claim rests on the
committed cleanup-verification docs, not on inspectable objects.

**Live verification, 2026-09-06 (repo repaired and holding):**
- `.git` 97 MB; 130 loose objects / 4.22 MiB; pack 90.93 MiB; 0 garbage; `fsck` clean
- `.beads/` gitignored (`.gitignore:66`), 0 tracked files
- Local HEAD == `origin/main` == `dec31058`
- Host: 62 GB RAM / 44 GB available, 58 GB disk free, load ~2.0

## Artifacts actually examined

`.beads/crashes/` — the location named in the dispatch — **does not exist**. The
real artifact locations are:

- `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-1{2,3}.jsonl` — worker logs (source of the census above)
- `.beads/checkpoint/forensic.jsonl` — the bead's `closed` event and close reason
- `.beads/crash-reports/` — empty
- `docs/crash-analysis/bf-1s6c3-agent-logs-preserved.md` — preserved agent logs
- Doc corpus: `docs/crashes/bf-1s6c3-*`, `docs/crash-investigation-bf-1s6c3-2026-09-01.md`,
  `docs/crash-investigation/crash-artifacts-bf-1s6c3.md` (the catalog whose Git-evidence
  section this note corrects)

## Classification

- **Crash type:** infrastructure — memcg OOM SIGKILL (exit −1), git operations on
  the bloated repository. Not a code defect; no domain-check code was implicated.
- **Severity anomaly worth keeping:** the retry loop multiplied one infrastructure
  event into 76 dispatches because the kill consistently landed between the
  deliverable's commit and the bead's close. The durable fixes for this shape are
  the `pack.windowMemory` bounds (gc *and* push) plus the closed-bead/duplicate
  filtering in the crash alert system — both in place.
- **Status:** resolved. Bead closed 2026-08-16; repository repaired (18 GB → 97 MB)
  and verified repeatedly since, most recently 2026-09-06.
