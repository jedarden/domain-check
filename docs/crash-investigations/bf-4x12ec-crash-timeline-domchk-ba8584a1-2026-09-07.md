# bf-4x12ec — Crash Timeline and Bead Context (2026-08-14)

Investigator bead: **domchk-ba8584a1** (child 2 of 4 of the `domchk-2ff261ce`
split) · Crash bead: **bf-4x12ec** ("Execute aggressive git garbage collection
to eliminate OOM risk", task, P2) · Worker
`claude-code-glm-4.7-lab-domain-check` · Needle session `a6dbb1fc` · All
timestamps UTC (box local = UTC−4).

Companion to the surviving log-source inventory
([`bf-4x12ec-log-source-inventory-domchk-a3f1f8f5-2026-09-07.md`](bf-4x12ec-log-source-inventory-domchk-a3f1f8f5-2026-09-07.md),
child 1 of this split), which established *which* sources survive and captured
all 53 attempt transcripts. This document is the timeline itself. Every count,
timestamp, and quoted task line in §1–§5 was re-read from a primary source
during this attempt (2026-09-08 00:2xZ); the few figures carried over from
prior documents are marked as citations. It does not re-open the mechanism
determination — that is the canonical report's
([`bf-4x12ec-crash-investigation.md`](bf-4x12ec-crash-investigation.md),
v1.9 + Addenda 2–7) — and it adds nothing to that report's body, whose
harmonization is owned by `domchk-8c78ae8b`.

## 1. Headline: one bead, three phases, 53 attempts, 2 h 41 m

Counts re-derived live this attempt from the primary worker log
(`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl`, 2,598,910
bytes / 10,138 lines, mtime still 2026-08-14 19:50 local): **1,146**
`bf-4x12ec` records — **53 × `bead.claim.succeeded`**, 53 × `agent.dispatched`,
53 × `agent.completed` (**44 × `exit_code −1`** classified `crash`, **8 ×
`exit_code 124`** classified `timeout`, **1 × `exit_code 0`** classified
`success`), 53 × `build.heartbeat` / `transform.started` / `transform.completed`
/ `outcome.classified` / `outcome.handled`, 52 × `bead.released`, plus
handling heartbeats (561 total: 301 `HANDLING`, 53 `HANDLING_POST_HANDLER`,
52 `HANDLING_FLUSH`/`HANDLING_RELEASE`/`HANDLING_RELEASE_DONE`,
51 `HANDLING_FLUSH_DONE`), 106 `agent.routing_decision`, 1
`worker.handling.timeout`, 1 `verification.passed`, 1 `bead.orphaned`. This
matches the committed bundle extract (`docs/crashes/bf-4x12ec/needle-events-2026-08-14-bf-4x12ec.jsonl.gz`,
1,146 lines) and `attempt-index.tsv` exactly.

| Phase | Attempts | Template / prompt | Window (UTC) | Exit | Classification | Duration each |
|---|---|---|---|---|---|---|
| 1. Crash loop | 1–44 (44) | `pluck/pluck-default`, 71,698 B | kills 10:23:02.958 → 11:27:26.174 (64 m 23 s) | −1 | `crash` | 38.9 – 115.8 s |
| 2. Timeout loop | 45–52 (8) | 45–47 `pluck` 71,698 B; 48–52 `split/split-default` 3,896 B | completions 11:38:07.867 → 12:50:14.283 (72 m 07 s) | 124 | `timeout` | exactly 600.0 s |
| 3. Success | 53 (1) | `split/split-default`, 3,896 B | 12:50:33.203 → 12:58:45.114 | 0 | `success` | 491.8 s |

- Bead created → first claim: **220.6 s** (3 m 41 s) of queue time.
- Bead created → final success: **9,678.7 s = 2 h 41 m 18.7 s**.
- `exit_code −1` is needle's writer-side sentinel for a child that died with no
  wait status — **not** a POSIX signal number, and the alert bodies' "(signal
  −1)" is template arithmetic on that sentinel. Semantics:
  [`bf-4x12ec-evidence-signal-semantics-domchk-15854355-2026-09-07.md`](bf-4x12ec-evidence-signal-semantics-domchk-15854355-2026-09-07.md);
  canonical: Addendum 2 "Signal −1, precisely".
- `exit_code 124` is the harness's 600 s dispatch cap; all eight phase-2
  durations are 600,018–600,024 ms.

## 2. Master timeline (2026-08-14, UTC)

| Time | Event | Source |
|---|---|---|
| 10:17:26.387856508 | **Bead bf-4x12ec created** (task, P2) — body prescribes the gc (§5) | live bead store, this attempt |
| 10:17:26 → 10:21:06 | Queue wait, 3 m 41 s | derived |
| 10:21:06.969922824 | `bead.claim.succeeded` — attempt 1, seq 1729 | worker log |
| 10:21:07.995 | Prompt (71,698 B) enqueued → Claude session `8b2a5b0d` | transcript (census.tsv) |
| 10:21:23.336 | `git count-objects -vH` → **4,649 loose objects / 17.20 GiB**, in-pack 4,081, 1 pack 9.60 MiB, 0 garbage | attempt-1 transcript |
| 10:21:41.782 | gc attempt 1 → **exit 128**: `gc.aggressivewindow = 1.hour` is not numeric | attempt-1 transcript (cited: artifacts doc §5) |
| 10:22:02.982 | Retry with `1h` → **exit 128** again | attempt-1 transcript (cited) |
| 10:22:18.314 | `git config --unset gc.aggressivewindow` → OK; the key is still unset today | attempt-1 transcript (cited: Addendum 4 §2) |
| 10:22:36.260 | **`git gc --aggressive --prune=now` launched** — no tool result ever returned | attempt-1 transcript |
| 10:23:02.958717335 | **Kill #1** — `agent.completed` `exit_code −1`, 115.8 s, seq 1741 | worker log |
| 10:23:02.959 | `outcome.classified` → `crash` | worker log |
| 10:23:11.225983666 | Alert bead **bf-fmg2cw** minted — the first of **44** (needle pre-0.4.2 mints one per kill; the 8 timeouts minted none) | forensic.jsonl via sibling bundle |
| 10:23:16 → 11:27 | Attempts 2–44: identical claim → measure → gc → killed cycle, 58–145 s claim-to-claim | worker log (§3) |
| 10:43:35.281763879 | Only handling anomaly: `worker.handling.timeout` (seq 2157, "bf sync --flush-only failed", flush op) during attempt 14's release — release then completed | worker log |
| 10:43:58–59 | Mid-storm capture (transcript `9539f3b2`): host **45 Gi available / 0 B swap used**, disk 85 % — the kill was not host-wide OOM; repo still **4,649 loose / 17.20 GiB**, i.e. zero packing progress in 22 min | attempt-15 transcript (cited: artifacts doc §4) |
| 11:27:26.173929030 | **Kill #44** — last `exit −1` (attempt 44, 86.4 s) | worker log |
| 11:28:02.199938557 | Alert bead **bf-5x69lm** minted — the last of the 44 | forensic.jsonl via sibling bundle |
| 11:28:07.790 | Attempt 45 claimed — first timeout attempt, still on `pluck` | worker log |
| 11:38:07.867 → 11:58:51.854 | Attempts 45–47: 600 s each, **zero assistant records** — the 71,698-byte prompt produced no output at all | worker log + transcripts census |
| 11:59:06.309774818 | **Template switch**: attempt 48 dispatched on `split/split-default`, prompt 71,698 → 3,896 B ("Auto-Split: Decompose This Bead") | worker log (dispatch events, this attempt) |
| 11:59:06 → 12:50:14 | Attempts 48–52: five more 600 s timeouts, still zero assistant records | worker log |
| 12:50:33.202654756 | Attempt 53 claimed (`split`, seq 3323) — last claim of the day | worker log, this attempt |
| 12:57:53.851 / 12:57:55.238 / 12:57:55.868 | Child beads created: **bf-173o7e** (gc), **bf-5jhvpk** (repack), **bf-im2sl1** (verify) | attempt-53 transcript, this attempt |
| 12:58:12.681–.710 | Dep chain `bf-173o7e → bf-5jhvpk → bf-im2sl1 → bf-4x12ec`; parent labelled `umbrella` (transcript records the pre-migration `bf` CLI) | attempt-53 transcript, this attempt |
| 12:58:44.943 | Final assistant text: `SPLIT_COMPLETE` | attempt-53 transcript |
| 12:58:45.113834930 | **`agent.completed` `exit_code 0`** (491.8 s) — the bead's "final success" | worker log |
| 12:58:45.126649351 | `verification.passed` (`gates_run: 1`, seq 3340) | worker log |
| 12:58:55.502272008 | `bead.orphaned` (seq 3343); `outcome.handled` `action: none` (seq 3344) — no alert on success | worker log |
| *Epilogue* | Parent **closed 2026-08-17T14:50:41.544Z** (rev 4, final metrics 753 MB / 141 loose); the gc itself completed under child **bf-173o7e** (closed 2026-08-17T17:12:09Z) | live bead stores (cited: Addendum 4 §3, Addendum 7) |

**What the 12:58:45 "final success" actually was:** not the gc. Attempt 53
executed needle's auto-split — it created three children, chained them, labelled
the parent an umbrella, and exited 0. The `git gc --aggressive --prune=now` the
bead was created to run completed later, under child bf-173o7e, on 2026-08-17
(Addendum 4 §3; the Summary of the canonical report's body predates that
correction). The success tail's `bead.orphaned` event is the
orphaned-after-success pattern that later re-triggered alert regeneration
against this closed bead — the reason the corpus holds ~46 investigation beads
for this one crash chain (alert inventory:
[`bf-4x12ec-alert-inventory.md`](bf-4x12ec-alert-inventory.md)).

## 3. Per-attempt timeline — all 53 attempts

Derived from `docs/crashes/bf-4x12ec/attempt-index.tsv` (which
domchk-4bad8e94 verified line-by-line against the worker log) and re-derived
independently this attempt; "tail" is the last conversation record in each
attempt's committed transcript (`docs/crashes/bf-4x12ec/transcripts/`,
re-censused this attempt — see §4). Gap = seconds since the previous claim.

| # | Claim | Ended | Dur s | Exit | Class | Template | Gap | Last transcript record |
|---|---|---|---|---|---|---|---|---|
| 1 | 10:21:06.969 | 10:23:02.958 | 115.8 | -1 | crash | pluck | — | gc tool_use, unanswered |
| 2 | 10:23:16.800 | 10:25:01.512 | 104.5 | -1 | crash | pluck | 130 | gc tool_use, unanswered |
| 3 | 10:25:41.457 | 10:26:47.738 | 66.1 | -1 | crash | pluck | 145 | gc tool_use, unanswered |
| 4 | 10:27:21.400 | 10:28:26.319 | 64.8 | -1 | crash | pluck | 100 | gc tool_use, unanswered |
| 5 | 10:28:48.372 | 10:29:33.156 | 44.6 | -1 | crash | pluck | 87 | gc tool_use, unanswered |
| 6 | 10:29:56.540 | 10:31:05.871 | 69.2 | -1 | crash | pluck | 68 | gc tool_use, unanswered |
| 7 | 10:31:29.792 | 10:32:08.823 | 38.9 | -1 | crash | pluck | 93 | gc tool_use, unanswered |
| 8 | 10:32:27.426 | 10:33:11.442 | 43.8 | -1 | crash | pluck | 58 | echo+gc tool_use, unanswered |
| 9 | 10:33:26.224 | 10:34:40.898 | 74.5 | -1 | crash | pluck | 59 | gc tool_use, unanswered |
| 10 | 10:35:07.928 | 10:36:05.465 | 57.3 | -1 | crash | pluck | 102 | gc tool_use, unanswered |
| 11 | 10:36:47.416 | 10:37:54.324 | 66.6 | -1 | crash | pluck | 99 | gc tool_use, unanswered |
| 12 | 10:38:28.566 | 10:39:27.018 | 58.0 | -1 | crash | pluck | 101 | gc tool_use, unanswered |
| 13 | 10:39:47.943 | 10:40:53.801 | 65.7 | -1 | crash | pluck | 79 | gc tool_use, unanswered |
| 14 | 10:41:39.757 | 10:42:58.570 | 78.6 | -1 | crash | pluck | 112 | gc tool_use, unanswered |
| 15 | 10:43:38.668 | 10:44:53.202 | 74.3 | -1 | crash | pluck | 119 | gc tool_use, unanswered |
| 16 | 10:45:07.362 | 10:46:16.579 | 69.0 | -1 | crash | pluck | 89 | gc tool_use, unanswered |
| 17 | 10:46:49.463 | 10:48:14.021 | 84.3 | -1 | crash | pluck | 102 | gc tool_use, unanswered |
| 18 | 10:48:46.061 | 10:49:35.988 | 49.7 | -1 | crash | pluck | 117 | gc tool_use, unanswered |
| 19 | 10:49:52.128 | 10:50:43.362 | 51.1 | -1 | crash | pluck | 66 | gc tool_use, unanswered |
| 20 | 10:51:03.223 | 10:51:49.607 | 46.2 | -1 | crash | pluck | 71 | gc tool_use, unanswered |
| 21 | 10:52:20.180 | 10:53:06.749 | 46.4 | -1 | crash | pluck | 77 | gc tool_use, unanswered |
| 22 | 10:53:44.899 | 10:55:21.576 | 96.4 | -1 | crash | pluck | 85 | gc tool_use, unanswered |
| 23 | 10:55:55.879 | 10:57:03.172 | 67.1 | -1 | crash | pluck | 131 | gc tool_use, unanswered |
| 24 | 10:57:16.435 | 10:58:12.166 | 55.6 | -1 | crash | pluck | 81 | gc tool_use, unanswered |
| 25 | 10:58:44.678 | 10:59:48.535 | 63.7 | -1 | crash | pluck | 88 | gc tool_use, unanswered |
| 26 | 11:00:15.587 | 11:01:31.948 | 76.1 | -1 | crash | pluck | 91 | gc tool_use, unanswered |
| 27 | 11:01:46.427 | 11:02:59.658 | 73.0 | -1 | crash | pluck | 91 | gc tool_use, unanswered |
| 28 | 11:03:35.468 | 11:04:55.025 | 79.4 | -1 | crash | pluck | 109 | gc tool_use, unanswered |
| 29 | 11:05:12.936 | 11:05:58.444 | 45.3 | -1 | crash | pluck | 97 | gc tool_use, unanswered |
| 30 | 11:06:17.434 | 11:07:07.394 | 49.8 | -1 | crash | pluck | 64 | gc tool_use, unanswered |
| 31 | 11:07:27.559 | 11:08:26.236 | 58.5 | -1 | crash | pluck | 70 | gc tool_use, unanswered |
| 32 | 11:08:49.807 | 11:09:44.290 | 54.3 | -1 | crash | pluck | 82 | gc tool_use, unanswered |
| 33 | 11:10:09.876 | 11:11:03.458 | 53.4 | -1 | crash | pluck | 80 | gc tool_use, unanswered |
| 34 | 11:11:16.130 | 11:12:12.344 | 56.1 | -1 | crash | pluck | 66 | gc tool_use, unanswered |
| 35 | 11:12:25.091 | 11:13:05.688 | 40.4 | -1 | crash | pluck | 69 | gc tool_use, unanswered |
| 36 | 11:13:38.100 | 11:14:21.326 | 42.9 | -1 | crash | pluck | 73 | gc tool_use, unanswered |
| 37 | 11:14:50.962 | 11:15:34.379 | 43.2 | -1 | crash | pluck | 73 | gc tool_use, unanswered |
| 38 | 11:15:52.844 | 11:16:58.674 | 65.5 | -1 | crash | pluck | 62 | gc tool_use, unanswered |
| 39 | 11:17:31.685 | 11:18:40.715 | 68.7 | -1 | crash | pluck | 99 | gc tool_use, unanswered |
| 40 | 11:19:12.458 | 11:20:56.353 | 103.7 | -1 | crash | pluck | 101 | gc tool_use, unanswered |
| 41 | 11:21:25.653 | 11:22:28.529 | 62.6 | -1 | crash | pluck | 133 | gc tool_use, unanswered |
| 42 | 11:22:55.832 | 11:24:19.158 | 83.0 | -1 | crash | pluck | 90 | gc tool_use, unanswered |
| 43 | 11:24:47.738 | 11:25:36.847 | 48.9 | -1 | crash | pluck | 112 | gc tool_use, unanswered |
| 44 | 11:25:58.620 | 11:27:26.173 | 87.3 | -1 | crash | pluck | 71 | gc tool_use, unanswered |
| 45 | 11:28:07.790 | 11:38:07.866 | 600.0 | 124 | timeout | pluck | 129 | dispatch text only — no assistant record |
| 46 | 11:38:28.014 | 11:48:28.078 | 600.0 | 124 | timeout | pluck | 620 | dispatch text only — no assistant record |
| 47 | 11:48:51.791 | 11:58:51.854 | 600.0 | 124 | timeout | pluck | 624 | dispatch text only — no assistant record |
| 48 | 11:59:06.300 | 12:09:06.366 | 600.0 | 124 | timeout | split | 615 | dispatch text only — no assistant record |
| 49 | 12:09:21.021 | 12:19:21.091 | 600.0 | 124 | timeout | split | 615 | dispatch text only — no assistant record |
| 50 | 12:19:38.093 | 12:29:38.161 | 600.0 | 124 | timeout | split | 617 | dispatch text only — no assistant record |
| 51 | 12:29:57.153 | 12:39:57.224 | 600.0 | 124 | timeout | split | 619 | dispatch text only — no assistant record |
| 52 | 12:40:14.213 | 12:50:14.282 | 600.0 | 124 | timeout | split | 617 | dispatch text only — no assistant record |
| 53 | 12:50:33.202 | 12:58:45.113 | 491.8 | 0 | success | split | 619 | assistant text — SPLIT_COMPLETE |

## 4. What the bead was doing when it was killed

Re-censused this attempt from the 53 committed transcripts (not inherited from
prior docs):

- **All 44 phase-1 attempts end in an assistant `tool_use` with no matching
  `tool_result`** — 43 of them exactly `Bash: git gc --aggressive
  --prune=now` (tool timeout 600000 ms), and attempt 8 at the same operation
  behind an `echo "Starting aggressive git garbage collection at $(date)" &&`
  header. The agents were killed **mid-tool-call**, running the command the
  bead's own body prescribed.
- Every phase-1 transcript that got that far recorded the same measurements
  first: `git count-objects -vH` → 4,649 loose / 17.20 GiB, `du -sh .git` →
  18G, `free -h` → tens of GB available — then launched the gc and died.
  Because aggressive gc builds delta chains in memory before writing any pack,
  **no attempt made any packing progress**: the repo read identically at
  10:21:23 and 10:43:49.
- **All 8 phase-2 attempts (45–52) contain only the dispatch text** — one
  conversation record, role `user`, zero assistant records in 600 s. Phase 2
  is not "the gc running slowly"; it is the agent producing no output at all
  while the box was still loaded (Addendum 3: `fleet.cpu_saturated` on
  essentially every dispatch, load 10.4–30.92, mean ~13.8, peak 30.92 at
  11:21:25; by the successful attempt it was 9.86).
- Attempt 53 (62 conversation records — the largest transcript) is the split
  session described in §2.
- Attempt 1 additionally burned 36.5 s of its 115.8 s budget on the two
  `gc.aggressivewindow` exit-128 failures (10:21:41.782 → 10:22:18.314) before
  its fatal gc launch at 10:22:36.260 (§2).

Mechanism (canonical, Addenda 3 and 6 — cited, not re-derived here): memcg OOM
inside the agent's transient `run-p*.scope` (`MemoryMax=12GiB`) over the
17–18 GiB loose-object repository; kernel-proven same-window by the 257 Aug-16
git `CONSTRAINT_MEMCG` kills at the 11–12 GB RSS ceiling. Aug-14 kernel logs
are unrecoverable (single surviving boot begins 2026-08-15 19:56:33 EDT —
value re-verified live 2026-09-07 by child 1), so for this bead the mechanism
stays regime-matched rather than kernel-proven.

## 5. The task body itself prescribed the crash trigger

Read live from the bead store this attempt (`bead show bf-4x12ec`, Closed rev
4). The bead was **created to run the exact operation that killed it**, in
unbounded form, against the bloated repo:

> **Title:** Execute aggressive git garbage collection to eliminate OOM risk
>
> **Task:** "Execute Phase 1 emergency stabilization — Execute aggressive git
> garbage collection to pack 17.20GB of loose objects into compressed pack
> files, eliminating the OOM risk during git operations."
>
> **Acceptance criterion 1:** "- [ ] Execute `git gc --aggressive --prune=now`
> successfully"
>
> **Commands to Execute:**
> ```bash
> # Step 1: Aggressive garbage collection (WARNING: may take 2-6 hours)
> git gc --aggressive --prune=now
> # Step 2: Additional repack optimization
> git repack -a -d --depth=250 --window=250
> ```
> *(excerpt — the body's Step 3 verifies cleanup with `git count-objects -vH`
> and `du -sh .git/`, and a Monitoring section prescribes a `watch` loop on
> `free -h`/`uptime`.)*

Three properties of that specification produced the storm:

1. **The prescribed command is the hazard.** An unbounded `git gc --aggressive`
   over a 17.20 GiB loose-object repository cannot fit inside the 12 GiB
   dispatch scope, so every attempt that reached it was killed by the same
   mechanism. No agent error was involved — each attempt did precisely what
   the bead asked and died identically.
2. **Retry could not change the outcome.** The task body was never revised
   between attempts, so all 44 phase-1 retries re-ran the identical command
   into the identical ceiling. The loop only broke when the *harness* changed
   (auto-split template at attempt 48), not when any agent did.
3. **The measured state was worse than the specification assumed.** The body
   cites **4,627** loose objects; the crash-time live reading the killed
   agents themselves took is **4,649** / 17.20 GiB. Both are genuine readings
   at different instants (canonical report's metric-provenance note names a
   third, 4,515, from `docs/cleanup-resolution-2026-08-17.md`); the 4,649
   figure is the one contemporaneous with the kills and is used in §2/§4.
   The size figure — 17.20 GiB loose in an 18 G `.git` — is consistent across
   all sources.

This is why the crash trigger is documented as the task's own prescription,
not agent behavior: the bead's deliverable was later completed only by
decomposing it (bf-173o7e ran the gc after the repo had been partially packed
by other cleanup work) and by the bounded replacements that exist today
(`scripts/safe-git-gc.sh`, `pack.windowMemory=2g` / `deltaCacheSize=1g` /
`pack.threads=1` — repo-local and global). Running the body's Step 1 verbatim
is now prohibited by workspace policy (CLAUDE.md, "Git Operations Safety"); a
bead created today with this body would be unsafe to dispatch as written.

## 6. Verification performed this attempt

All commands run against primaries on this box, 2026-09-08 00:2x–00:5xZ:

- `grep -c bf-4x12ec` on the Aug-14 worker log → **1,146**; Python census by
  `event_type` → 53 claims / 53 dispatched / 53 completed / 44 × −1 / 8 × 124
  / 1 × 0; phase boundaries match §1 to the microsecond; heartbeat
  sub-reconciliation 301 + 53 + 52×3 + 51 = 561 (the committed README's "301"
  counts only the `HANDLING` state); sequence spot-checks read from the raw
  records: first claim seq 1729, kill #1 seq 1741, attempt-53 claim seq 3323,
  success seq 3335, `verification.passed` seq 3340, `bead.orphaned` seq 3343.
- `bead show bf-4x12ec` → Created `2026-08-14T10:17:26.387856508Z`, Closed
  rev 4, updated 2026-08-17T14:50:41.544361971Z; description and notes quoted
  in §5 and §2.
- `docs/crashes/bf-4x12ec/attempt-index.tsv` re-parsed → 54 lines
  (header + 53), template column per attempt: `pluck/pluck-default` 71,698 B
  for 1–47, `split/split-default` 3,896 B for 48–53; the switch is attempt
  48's dispatch at 11:59:06.309Z (confirmed against the raw
  `agent.dispatched` events' `prompt_len`).
- Transcript census over `docs/crashes/bf-4x12ec/transcripts/` (53 files) →
  44 × trailing unanswered tool_use (43 gc + 1 echo-gc), 8 × user-only,
  1 × assistant text; attempt 53's `bf create` ×3 / dep chain / umbrella
  label / `SPLIT_COMPLETE` timestamps read from the transcript itself.
- Alert bounds read from the sibling bundle's
  `alert-beads-exit-timestamps.txt` (44 rows, first `bf-fmg2cw`
  10:23:11.225983666Z, last `bf-5x69lm` 11:28:02.199938557Z, sourced from
  `.beads/checkpoint/forensic.jsonl`); timeouts minted no crash alerts.

## 7. Related records

| Record | What it holds |
|---|---|
| [`bf-4x12ec-log-source-inventory-domchk-a3f1f8f5-2026-09-07.md`](bf-4x12ec-log-source-inventory-domchk-a3f1f8f5-2026-09-07.md) | Child 1: which sources survive; the 53-transcript capture |
| [`bf-4x12ec-crash-investigation.md`](bf-4x12ec-crash-investigation.md) | Canonical report v1.9 + Addenda 2–7 (mechanism, corrections, consolidation) |
| [`bf-4x12ec-alert-inventory.md`](bf-4x12ec-alert-inventory.md) | The 44 storm alerts + 16 regeneration beads |
| [`bf-4x12ec-crash-artifacts-2026-09-02.md`](bf-4x12ec-crash-artifacts-2026-09-02.md) | The split parent's artifact survey (its §2 timeline is the short form of §2 here) |
| [`bf-4x12ec-evidence-signal-semantics-domchk-15854355-2026-09-07.md`](bf-4x12ec-evidence-signal-semantics-domchk-15854355-2026-09-07.md) | `exit_code −1` writer-side semantics |
| `docs/crashes/bf-4x12ec/` and `docs/crash-investigations/evidence/bf-4x12ec/crash-logs/` | The two extraction bundles (attempt index, raw events, transcripts, alert records) |

---
**Assembled:** 2026-09-08 00:5xZ (2026-09-07 20:5x EDT) · Bead
`domchk-ba8584a1` · Confidence HIGH — every count and timestamp re-derived
from the primary worker log, the live bead store, or the committed
byte-verified transcript bundle during this attempt.
