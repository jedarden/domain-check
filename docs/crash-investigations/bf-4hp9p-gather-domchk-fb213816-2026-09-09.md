# Gather: crash logs and context for bead bf-4hp9p (domchk-fb213816)

**Leg:** domchk-fb213816 — gather child of alert bead domchk-6ce337d0 (auto-split 2026-09-01T16:25:36Z)
**Subject:** the crash reported against alert bead **bf-4hp9p**
**Scope:** fact-finding only — no fix, no new cause claim. Root-cause mechanism is subordinate to the
canon RCA (`docs/investigations/root-cause-determination-domchk-6281555d-2026-09-06.md`): memcg-OOM
SIGKILL of git operations inside 12 GiB dispatch scopes during the bloat era.
**Date:** 2026-09-09. Every figure below was re-verified first-hand this dispatch.

## 1. What bf-4hp9p is

`bf-4hp9p` is an **ALERT bead**, not a work bead (rev 21, **Closed** 2026-08-25T14:29:25Z):

> ALERT: Agent crash on bead **bf-1s6c3** — Agent claude-code-glm-4.7, Exit code -1 (signal -1),
> Timestamp 2026-08-12T23:31:51.020140865+00:00

So bf-4hp9p's *report content* describes the **bf-1s6c3** kill (bloat-era git-reconciliation crash;
canon: 76 dispatches / 71 kills, `docs/crash-analysis-bf-1s6c3-2026-09-06.md`). bf-1s6c3 is
**Closed rev 5** (2026-09-07) and its task ("Create merge commit reconciling Forgejo and GitHub
histories", the ~685-commit divergence) completed successfully on retry.

The chain then went one generation deeper: **the agent processing alert bf-4hp9p was itself killed**,
five times, on 2026-08-16 — each kill minting its own second-generation alert bead.

## 2. The three timestamps in play (template premise corrected)

This gather leg's template carries "`-1`, `2026-08-16T14:31:52`". Three distinct instants exist and
must not be conflated:

| Instant (UTC) | What it actually is | Primary evidence |
|---|---|---|
| **2026-08-12T23:31:51.020140865Z** | The bf-1s6c3 kill — the crash bf-4hp9p was ALERTING about. Heartbeat recorded in bf-4hp9p's alert report. | bf-4hp9p description; forensic.jsonl event 873 |
| **2026-08-16T14:31:52.942290611Z** | The kill of the worker **processing** bf-4hp9p. Recorded in the crash report of alert bead **domchk-6ce337d0** (created 14:31:52.943895854Z, 1.6 ms after the record) — the umbrella this leg split from. This is the timestamp the task template carries. | domchk-6ce337d0 description; drawrace worker log line 8166 |
| **2026-08-16T14:35:31.031211882Z** | A *different* sibling kill — the record inside alert bead **domchk-e4b9af74**. The 2026-08-25 investigation doc (79d3649) used this one as "the" crash timestamp. | domchk-e4b9af74 description |

## 3. Kill ledger — bf-4hp9p's processors on 2026-08-16

From `~/.needle/logs/needle-claude-code-glm-4_7-lab-drawrace.log` (worker
`claude-code-glm-4.7-lab-drawrace`, session `70478ac5`, model `glm-4.7`), which plucked bf-4hp9p
cross-workspace (`bead is from remote workspace, switching store … remote_workspace=/home/coding/domain-check`):

**7 claims, 5 `outcome=Crash(-1)` deaths**, each death → one sibling alert bead:

| # | Claim (UTC) | Killed (UTC) | Attempt span | Alert bead minted |
|---|---|---|---|---|
| 1 | 14:28:15.157 | **14:31:52.844** | 3 m 37 s | domchk-6ce337d0 (14:31:52.943) — umbrella, still Open |
| 2 | 14:31:53.255 | 14:35:30.927 | 3 m 37 s | domchk-e4b9af74 (14:35:31.031) — Closed rev 10 |
| 3 | 14:35:31.346 | 14:37:35.979 | 2 m 04 s | domchk-4817f168 (14:37:36.081) — Closed rev 8 |
| 4 | 14:37:36.394 | 14:40:33.334 | 2 m 57 s | domchk-4440149b (14:40:33.442) — Closed rev 12 |
| 5 | 14:46:04.237 | 14:48:23.023 | 2 m 19 s | domchk-ccd3421d (14:48:24.146) — Closed rev 8 |

Two claim ends (14:40:33.864 → re-claim 14:46:04; 14:48:25.256 → bead later Closed rev 21) produced
no `Crash(-1)` outcome line in the retained log — recorded here as unresolved claim ends, not
inferred as successes.

Each needle death line reads: `handling agent outcome bead_id=bf-4hp9p exit_code=-1 outcome=Crash(-1)`
→ `agent crashed — releasing bead and creating alert … signal_code=-1 agent=claude-code-glm-4.7`
→ `crash alert bead created bead_id=bf-4hp9p alert_id=<id>`.

## 4. Kernel evidence — the deaths are memcg OOM, same second

`journalctl` (LOCAL = EDT = UTC−4) records `CONSTRAINT_MEMCG` kills at exactly the five seconds
above (kernel 10:31:52 EDT = 14:31:52 UTC), every victim `git`:

| Kernel (EDT) | Victim | pid | anon-rss | Scope (12 GiB bound) |
|---|---|---|---|---|
| 10:30:54 | git | 1466814 | 6,997,508 kB | run-p1446290 (the bf-3riuu death, same wave) |
| **10:31:52** | **git** | **1477169** | **12,287,564 kB ≈ 11.72 GiB** | **run-p1444226-i211132581.scope** ← kill #1 |
| 10:35:30 | git ×2 | 1500032 / 1499296 | 12,241,908 / 12,312,048 kB | run-p1468410 / run-p1477493 ← kill #2 |
| 10:37:35 | git | 1524158 | 12,333,836 kB | run-p1505045 ← kill #3 |
| 10:40:33 | git | 1549720 | 7,420,992 kB | run-p1525600 ← kill #4 |
| 10:48:22–23 | git ×2 | 1618140 / 1619298 | 7,641,532 / 4,691,164 kB | run-p1604120 ← kill #5 |

`oom-kill:constraint=CONSTRAINT_MEMCG … task=git` — the kernel names the constraint; this is the
canon mechanism (bloat-era store, git inside a 12 GiB systemd-run scope), kernel-proven for these
five kills because the journald boot covers 2026-08-16. The kernel line does not record a cwd, so
the per-second match with needle's `bead.outcome` lines (0.9 s after the kernel stamp, e.g. kill #1:
kernel 10:31:52 → outcome 14:31:52.844Z) is the attribution — high confidence, same signature as the
neighboring wave kills, not absolute proof for any single pid.

**"CPU saturation" is superseded.** The needle rate-limit warnings in the same window
(`CPU load exceeds warning threshold load_1min=17.01/22.66/23.15 normalized=2.43–3.31`) are real
but co-symptomatic: fleet-wide concurrent git thrash over bloat-era stores produced both the load
and the memory deaths. The kill mechanism on the kernel lines is memory-cgroup exhaustion, never CPU.

## 5. What the agent was working on when it crashed

Two layers, both verified:

1. **The killed worker (this leg's subject)** was processing ALERT bead bf-4hp9p — i.e. investigating
   the bf-1s6c3 crash — inside `/home/coding/domain-check`. Kill #1's attempt (3 m 37 s) committed
   only dispatch bookkeeping: b74a5ac, "chore: update needle predispatch sha after crash recovery
   (bf-4hp9p)", 14:31:44Z — **8 s before the kill** (mid-attempt, not post-completion). The `.needle-
   predispatch-sha` file is per-dispatch bookkeeping, never crash evidence.
2. **The subject of the alert it was investigating**: bf-1s6c3's task "Create merge commit reconciling
   Forgejo and GitHub histories" at 2026-08-12T23:31:51Z (bloat-era; ~685-commit divergence era).
   bf-1s6c3 closed rev 5 with its work complete; the "660/685 commits ahead" figure describes that
   resolved pre-squash state.

## 6. Work-loss assessment — none

- The bf-4hp9p investigation deliverable **landed during the same wave**: e210c84, 14:40:07Z —
  "investigation: complete crash investigation for bf-1s6c3" (119-line `bf-4hp9p-crash-`
  `investigation.md` at repo root; moved to `docs/crash-investigations/` by bb7455f the same day) —
  committed by kill #4's attempt **26 s before its own death**, i.e. that kill was post-completion.
- bf-4hp9p: Closed rev 21 (2026-08-25). bf-1s6c3: Closed rev 5 (2026-09-07).
- The two verification reports for the duplicate second-generation alerts: 32a4b7b (domchk-6ce337d0
  leg) and 713a6f6 (domchk-ccd3421d leg), both "duplicate alert … already investigated and resolved",
  now at `docs/archive/crash-investigations/`.

## 7. Corrections to `docs/crash-investigations/bf-4hp9p-crash-investigation-2026-08-16.md` (79d3649)

| Its claim | Correction (first-hand, this dispatch) |
|---|---|
| Root cause "extreme CPU saturation (4.46× load)" | Superseded — kernel lines at all five kill seconds read `CONSTRAINT_MEMCG`, task=git, ≈11.7 GiB anon-rss in 12 GiB scopes. Load 17–23 was co-symptom, not the kill. |
| Timestamp 14:35:31.031211882Z as "the" crash time | That is sibling alert domchk-e4b9af74's record (kill #2). The domchk-6ce337d0-leg kill is 14:31:52.942290611Z; the alert's own subject is 2026-08-12T23:31:51Z. |
| "1 of 826 crashes" on 2026-08-16 | Unverified method. Census this dispatch from retained `~/.needle/logs`: **409 `Crash(-1)` outcome events, 102 distinct beads** on 2026-08-16 — a floor (retention gaps), not a canon figure. |
| "crashed immediately after completing its investigation" | True only for kill #4 (doc landed 26 s before death). Kill #1 (this leg's subject) died mid-attempt at 3 m 37 s with only predispatch bookkeeping committed. |
| "OOM killer 70% / watchdog 20% / container 10%" | Resolved: kernel-proven memcg OOM (`try_charge_memcg`, usage 12582912 kB = limit). |
| Investigation doc at `docs/crash-investigations/bf-4hp9p-crash-investigation.md` | Correct post-bb7455f; originally committed at repo root by e210c84 (14:40:07Z). |

(Cosmetic: that doc also carries a stray CJK artifact at its "Task Completion Analysis" — evidence of
its generation era; content judged on the table above.)

## 8. Current family status (live `bead show`, 2026-09-09)

| Bead | Status | Note |
|---|---|---|
| bf-1s6c3 (original crash target) | **Closed** rev 5 | work complete; canon analysis 2026-09-06 |
| bf-4hp9p (first-generation alert) | **Closed** rev 21 | 2026-08-25 |
| domchk-e4b9af74 / 4817f168 / 4440149b / ccd3421d | **Closed** rev 10/8/12/8 | sibling second-generation alerts |
| **domchk-6ce337d0** | **Open** rev 16 | umbrella of the 2026-09-01 split (labels: alert, crash, signal--1, umbrella, split-child, verification-failed) — blocks the six split children below; **owned by the chain, not by this leg** |
| domchk-fb213816 (this leg) | InProgress | gather |
| domchk-582a1137 / efd2efda / 24615bf5 / f961d4c4 / fb86a21e | Open | sibling split legs: analyze / review / verify-fix / doc / verify-findings |

## 9. Sources (all read this dispatch)

- `bead show bf-4hp9p`, `domchk-6ce337d0`, `domchk-e4b9af74`, `domchk-4817f168`,
  `domchk-4440149b`, `domchk-ccd3421d`, `bf-1s6c3`, `domchk-fb213816`
- `.beads/checkpoint/forensic.jsonl` — 99 events mentioning bf-4hp9p; event 873 (bf-4hp9p creation
  2026-08-12T23:31:51.026848425Z); the domchk-6ce337d0 crash report line
- `~/.needle/logs/needle-claude-code-glm-4_7-lab-drawrace.log` lines 8164–8348 (claims + outcomes),
  `…lab-domain-check.log` lines 1633–1647 (same wave, bf-3riuu)
- `journalctl --since "2026-08-16 10:29" --until "2026-08-16 11:00"` — oom-kill / Killed-process lines
- `git show` b74a5ac, e210c84, 79d3649, 32a4b7b, 713a6f6, bb7455f; `git log --all --grep bf-4hp9p`
- Fleet census: `grep -h "outcome=Crash(-1)" ~/.needle/logs/*.log | grep -c ^2026-08-16` → 409;
  distinct `bead_id=` among those → 102

---
**Leg disposition:** docs-only gather summary (this file), subordinate to the canon RCA; summary also
recorded on the leg bead's own notes. The umbrella domchk-6ce337d0 and the five sibling legs are
separate beads — this leg neither closes nor re-investigates them.
