# Crash-time resource analysis: bf-2ildm @ 2026-08-13T15:01:52Z

**Investigation dispatch:** domchk-e715aed3 (2026-09-07)
**Crash target:** bf-2ildm — "Extract GitHub-specific commits" (CLOSED 2026-08-16, work intact)
**Investigated instant:** 2026-08-13T15:01:52Z
**Relation to the current determination:** corroborates
`bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md` (§2/§3/§5); adds what
its era column lacks — the resource state **at the crash instant**, not just today.
Supersedes nothing; that doc and the classification chain remain canonical.

## 1. The instant resolved, first-hand from the raw fleet log

The named instant is **not a death time**. Re-derived independently from
`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` (worker
`claude-code-glm-4.7-lab-domain-check`, session `e29942f7`), without using the retrieval
bundle, and then checked against it:

| Field | Value (raw log) | Bundle row (attempt 24) |
|---|---|---|
| claim / dispatch | 14:56:25.959852428Z / 14:56:25.971717149Z | same |
| **kill (`agent.completed` `exit_code: -1`)** | **15:01:35.775155479Z** (309,461 ms) | same |
| `outcome.classified` | 15:01:35.777897479Z (`exit_code: -1, outcome: "crash"`) | — |
| `HANDLING_RELEASE_DONE` heartbeat | **15:01:52.450364051Z** ← the dispatch's instant | **+16.7 s** after kill |
| `bead.released` + `outcome.handled action: "alerted"` | 15:01:55.708950269Z | same |
| next claim (`bead.claim.succeeded`, attempt 25) | 15:01:59.067065373Z | same |

Ordinals 19 (14:40:29.551Z → bf-66sw7c) and 24 (15:01:35.775Z → bf-z15pix) reproduce the
bundle's `attempt-index.tsv` exactly, so this dispatch's instant is attempt 24 of 43.
Full bf-2ildm census, re-counted from the same log: 38 × exit −1 (13:37:24.838Z →
15:53:28.914Z, durations 98–309 s), 4 × exit 124 (each exactly ≈600,0XX ms — the 10-min
cap, attempts 39–42), 1 × exit 1 at 19 ms (attempt 43). Sums to the bundle's 43.

## 2. What resource telemetry exists for 2026-08-13 — almost nothing, and one trap

Every potential source was checked; exactly one survives:

| Source | Status |
|---|---|
| Kernel journal (OOM/kill lines) | **Gone.** `/var/log/journal` is persistent, but `journalctl --list-boots` shows a single boot beginning **2026-08-15 19:56:33 EDT** — 2.4 days *after* the crash. Epoch-bounded query (`--since @1786632600 --until @1786633800`) → `-- No entries --`. |
| sar/atop/sysstat | Not installed, no `/var/log/sa*`, no `/var/log/atop`. |
| `.beads/logs/` collectors | Start 2026-09-02; nothing for Aug-13. |
| Needle single-slot trace archive | Overwritten by the successful Aug-16 retry (exit 0) — the blind spot that produced the wrong 09-02 RCA. |
| **`fleet.cpu_saturated` events** (needle fleet logs) | **Survive.** The only crash-time resource telemetry that exists. |

**Timezone trap (would have produced a false negative):** this box runs
America/New_York (EDT, UTC−4), and `journalctl --since "2026-08-13T14:50:00"` parses a
naive timestamp as **local** time. The dispatch's suggested command would therefore have
queried 18:50–19:10 UTC — 3 h 48 m to 4 h 08 m *after* the death — and reported "no OOM
events" from an empty (and, post-reboot, doubly empty) window. Query kill windows by
`@epoch` (`date -d "2026-08-13T15:01:52Z" +%s` → 1786633312), never by naive ISO.

## 3. Crash-time load (the surviving telemetry)

`fleet.cpu_saturated` fires per dispatch decision when 1-min load exceeds
`threshold 0.8 × core_count 9 = 7.2`. Samples exist **only on the saturated side**, so
every figure below is a lower bound. All from the 14:30–15:15Z window across all Aug-13
fleet logs:

| Time (Z) | Load (1-min) | Note |
|---|---|---|
| 14:52:34 | 12.91 | attempt 23 dispatch |
| 14:56:25 | 10.22 | **attempt 24 dispatch** (the killed attempt) |
| 15:01:21 | 15.40 | 14 s before attempt 24's kill |
| 15:01:59 | 16.97 | attempt 25 dispatch, 24 s after the kill |
| 15:07:08–15:07:14 | 24.07–24.63 | peak in the window; attempt 25 dies at 15:06:41 |

Load was **rising through the kill** (10.2 → 15.4 → 17.0 → 24.6) and the box was CPU-
saturated essentially all day (3,614 saturation events across workers; the day's worst
hours were 20:00/22:00/23:00 with 656/632/652 events each — not 15:00). CPU saturation
alone kills nothing, and no memory telemetry exists at all (the needle-side M-2 gap, seen
from the fleet-log side).

## 4. Neighbor survival census — the crash was workspace-local, not box-wide

All `agent.completed` events in 14:55–15:10Z across every Aug-13 fleet log:

| Time (Z) | Bead | Worker/repo | Exit | Duration |
|---|---|---|---|---|
| 14:59:01 | bf-5etzu | s1 | **0** | 533,580 ms |
| 15:04:39 | bf-6d2rk | drawrace | **0** | 417,990 ms |
| 15:04:50 | bf-4vx934 | test-fix | **0** | 372,805 ms |
| 15:04:50 | bf-6d2rk | roam-1 | **0** | 208,544 ms |
| 15:05:14 | bf-6d2rk | roam-1 | 1 | 34 ms |
| 15:06:56 | bf-6d2rk | roam-1 | **0** | 98,674 ms |
| 15:08:18 | bf-610wq | roam-2 | 124 | 600,032 ms |
| 15:08:57 | bf-5gfkt | s1 | **0** | 594,933 ms |
| 15:09:48 | bf-4ej9sq | test-fix | **0** | 297,103 ms |

Attempt 24's kill sits 22 s after a 533-second clean completion and 3 m before four
more. **Seven exit-0 completions by five other workers (four other repos) in the same
15 minutes**, including runs of 595 s and 534 s — a box-wide memory event would not
spare those. This is new affirmative evidence for the determination's attribution: the
kill was confined to the domain-check workspace's own 12 GiB dispatch scope (the
repository-bloat regime), while the box as a whole kept completing work. The honest
limit: neighbors ran other repos, so their survival says nothing about domain-check's
own repo state — it rules out box-wide exhaustion, not the scope-local mechanism.

## 5. Repository bloat — era vs. measured now

| Metric | 2026-08-13 (era, documented) | 2026-09-07 19:06 UTC (measured this dispatch) |
|---|---|---|
| `.git` size | ~18 GB | **105 MB** |
| Loose objects | ~17 GB (17+ identical 237 MB `.beads/*.jsonl` snapshots) | **167 objects / 1.27 MiB** |
| Packed | fragmented | 11,700 in-pack, 2 packs, 99.78 MiB, 0 garbage |
| Unpushed backlog | 422 commits | 0 (`origin/main..HEAD` empty) |

Era figures are the documented ones (bf-1s6c3/bf-4yjq record; no contemporaneous
measurement survives to re-derive them). The loose-object count differs from the
determination §5's 129/1.01 MiB taken hours earlier the same day — ordinary churn from
co-tenant activity between the two measurements, not a contradiction.

## 6. Correlation with exit −1

- exit −1 is needle's died-without-exit-code sentinel (`code().unwrap_or(-1)`), not a
  signal number; a delivered SIGKILL would encode 137 — consistent with a memcg-OOM
  SIGKILL observed from outside the killed process.
- The 38 kills are mid-task signal losses at variable durations (98–309 s), not fixed
  ceilings — no timeout shape (the timeout shape appears separately as the four exactly
  600,0XX ms exit-124 attempts at 16:03–16:35, a different, non-crash class, alerted
  never).
- Kernel proof for *this* bead is unrecoverable (§2), so the memcg mechanism stays
  assigned by regime match at MODERATE confidence, exactly as the determination holds;
  nothing found here weakens or strengthens that beyond its published level.
- The alert stamp carries the +16.7 s heartbeat offset (§1) — third independent
  confirmation (after bf-66sw7c +13.1 s and bf-173o7e's 8–120 s pattern) that alert-bead
  timestamps must never be cited as kill instants.

## 7. Acceptance criteria, answered

| Criterion | Finding |
|---|---|
| Memory / disk / load at crash time | Load 10.2–15.4 on 9 cores around the kill (saturated-side lower bounds, §3); **no memory or disk telemetry of any kind survives for that instant** — the disk-exhaustion and box-memory-pressure hypotheses are untestable directly, and box-wide memory pressure is affirmatively ruled out by §4 |
| OOM killer events near 15:01:52 | None recoverable — journal has no entries before 2026-08-15 19:56:33 EDT (§2); mechanism attribution is by regime match (MODERATE), unchanged |
| Repository bloat | Era ~18 GB documented; repaired and holding at 105 MB / 167 loose objects / 0 garbage (§5) |
| Correlate with exit −1 | Real kill 15:01:35.775Z (attempt 24 of 38), sentinel semantics, mid-task, variable-duration, workspace-local (§1, §6, §4) |

## Sources (all first-hand this dispatch unless noted)

- Raw fleet log: `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl`
  (session `e29942f7`, sequences 1016–1093) plus all `*2026-08-13*.jsonl` fleet logs for
  the cross-worker census and saturation profile
- Journal: `sudo journalctl --list-boots`; `sudo journalctl -k --since @1786632600 --until @1786633800`
- Live repo: `du -sh .git`, `git count-objects -vH`, `git log origin/main..HEAD`, `free -h`, `df -h /`, `uptime`
- Cross-checked (not re-derived): `docs/crashes/bf-2ildm/attempt-index.tsv` rows 19/23–25;
  `bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md` §2/§3/§5
- Era documentation: `docs/crash-analysis-bf-1s6c3-2026-09-06.md`
