# Crash pattern analysis: bf-2ildm — surge indicators and cascade structure

**Investigation dispatch:** domchk-579d25b0 (2026-09-07, "review crash artifacts and
diagnostic data")
**Crash target:** bf-2ildm — "Extract GitHub-specific commits" (Closed 2026-08-16,
work intact)
**Relation to the chain:** corroborates
[`bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md`](bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md)
(§2/§3) and the retrieval bundle (`docs/crashes/bf-2ildm/`). Adds two dimensions the
chain does not quantify: (a) the surge-threshold verdicts computed first-hand from the
raw Aug-13 fleet logs, and (b) the day's cascade structure — where bf-2ildm's storm
sits among the day's other storms. Supersedes nothing.

## 1. Acceptance-criteria checks, re-executed first-hand 2026-09-07

| Check | Result |
|---|---|
| Bundle integrity (`sha256sum -c MANIFEST.sha256` in `docs/crashes/bf-2ildm/`) | **8/8 files OK**, none modified since assembly |
| `.beads/logs/` for bf-2ildm | **No Aug-13 telemetry exists** — the collectors start 2026-09-02. Four logs mention the bead and all are later tooling: `crash-alert-manager.log` (2026-09-02, "already CLOSED - no alert needed"), `crash-resolution-tracker.log` (2026-09-07, resolved), `alert-deduplication.log` (2026-09-07, 48× `SUPPRESS (resolved target bf-2ildm)` across repeated checks of the 38 alert beads), `work-completion.log` (2026-09-07, sibling verifications) |
| `.beads/events.jsonl` | **3 records, all the Aug-16 success** (claim 22:27:18Z → complete 22:28:44Z, `exit_code: 0`). No Aug-13 witness — the store floor postdates the storm, which is why `crash-classifier.sh` returns UNKNOWN and the pattern script sees nothing |
| Trace slot (`.beads/traces/bf-2ildm/`) | Still the Aug-16 success only (`metadata.json`: exit 0, 85,327 ms, captured 2026-08-16T22:28:44Z) — the single-slot overwrite that produced the wrong 2026-09-02 RCA |
| `crash-pattern-detection.sh` (run live) | `✅ No crashes detected in the last 24hours / System Status: STABLE` — correct for today, and structurally unable to see 2026-08-13: it reads `events.jsonl` (floor 2026-08-16), not the fleet logs |
| `.beads/` SIGKILL grep | No crash-era witness; hits are the bf-4yjq resolution notes and sibling state JSONL |

Every load-bearing crash-era figure below is re-derived from the primary source,
`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` plus the other
five Aug-13 worker logs, not cited from the bundle.

## 2. Surge-threshold verdicts (computed from the raw fleet log)

Three thresholds are in circulation and they disagree at per-bead scope:

| Threshold | Source | bf-2ildm alone | Fleet (6 workers, 20 beads) |
|---|---|---|---|
| 3 crashes in 5 min (surge) | `crash-pattern-detection.sh:46` | **3** — reaches it (best span 13:51:50Z→13:55:53Z, 243 s, holds 3) | **7** — fires |
| 10 crashes in 10 min | CLAUDE.md "Recommended Alerts" | **4 max** — does **not** fire | **12** (22:13:16Z→22:23:13Z) — fires |
| 10 crashes/hour sustained (storm) | `crash-pattern-detection.sh:49` | **15 in the 14:00Z hour; 16.8/h sustained over the storm** — fires | **56 in the 23:00Z hour; 14.7/h across the day** — fires |

**The attribution trap:** the CLAUDE.md "10+ crashes in 10 minutes" line, applied to a
single bead's kills, is a false negative for this storm — bf-2ildm's ~2–5 min
re-dispatch cadence (38 kills over 2 h 16 m) never stacks 10 kills into 10 minutes even
though the bead was being killed continuously. A bead-scoped investigator applying the
documented line would conclude "not an infrastructure event." The correct readings are
the hourly/storm rate (fires at 15–17/h vs the 10/h threshold) and the fleet-wide count
(fires on every threshold). `crash-pattern-detection.sh`'s own header already states
this design choice — per-bead thresholds "fire far too late in a multi-bead storm" —
and keys its detectors system-wide; but no doc previously recorded the actual numbers
for this bead.

Also structural: even a firing threshold could not have caught this storm through the
automated path, because the script's only source (`events.jsonl`) begins three days
*after* it. The fleet logs are the sole witness, and they are outside every automated
detector's input.

## 3. Cascade structure — where bf-2ildm sits in the day

2026-08-13 saw **353 exit −1 kills across 20 beads, all on the single
`claude-code-glm-4.7-lab-domain-check` worker**, from 00:00:15.943Z to 23:59:58.067Z —
the repository-bloat regime active essentially the entire day. The storms are
**sequential, not simultaneous**: each bead's retry loop absorbs the regime for minutes
to hours, then the next storm starts seconds-to-minutes later (inter-storm gaps
27 s – 2.7 h):

| # | Bead | Kills | Window (UTC) | # | Bead | Kills | Window (UTC) |
|---|---|---|---|---|---|---|---|
| 1 | bf-1s6c3 | 22 | 00:00→01:24 | 11 | bf-2vtzg | 9 | 09:13→09:40 |
| 2 | bf-1e99m | 2 | 00:02→00:05 | 12 | bf-ncxbt | 11 | 09:46→10:22 |
| 3 | bf-4s21u | 2 | 00:05→00:11 | 13 | bf-574w1 | 5 | 10:41→11:03 |
| 4 | bf-4k2ws | 55 | 02:03→07:03 | 14 | bf-6d3d6 | 6 | 11:16→13:10 |
| 5–8 | (1-kill beads ×4) | 4 | 02:54→03:01 | 15 | bf-3hivb | 5 | 13:16→13:29 |
| 9 | bf-67t2w | 1 | 05:16 | 16 | **bf-2ildm** | **38** | **13:37→15:53** |
| 10 | bf-1ea4g | 56 | 07:20→09:08 | 17 | bf-2o7nlw | 1 | 18:34 |
| — | — | — | — | 18–20 | bf-29h1yy / bf-mje3pd / bf-65lsdu | 2/7/127 | 18:48→23:59 |

bf-2ildm is the 16th of 20 sequential bead-storms. Three properties distinguish it:

1. **It is the only major storm with zero concurrent bead deaths.** Every
   `agent.completed` with exit −1 anywhere in the fleet during 13:35–16:40Z belongs to
   bf-2ildm — its 38 kills *are* the fleet's activity in that window. bf-3hivb's storm
   ended 474 s before bf-2ildm's first kill; the next bead's kill (bf-2o7nlw) came
   9,639 s after its last. Contrast the 02:54–03:01 cluster, where four single-kill
   beads died *inside* bf-4k2ws's 5 h storm — multi-bead concurrency existed on the
   day, just not during bf-2ildm's window.
2. **The fleet-wide surge peak is not bf-2ildm's.** The 12-kills/10-min fleet maximum
   (22:13–22:23Z) and both 50+ kill hours (22:00, 23:00) belong to bf-65lsdu's 127-kill
   storm, five hours after bf-2ildm went quiet. Citing "the Aug-13 surge peak" for
   bf-2ildm would be wrong by five hours.
3. **Same regime, so the sequence is not evidence of contagion.** The storms chain
   because each retry loop re-enters the same ~18 GB loose-object workspace; a storm
   ending does not heal anything, it just frees the worker for the next claim. The
   determination's attribution (workspace-local repo bloat, neighbors on other repos
   completing normally) is what explains the sequencing — this section adds the
   *temporal* shape to that attribution, not a new cause.

## 4. Answers to the investigation questions

- **Other exit −1 crashes around the same time?** Adjacent in time (bf-3hivb ended
  474 s before; 353 fleet kills the same day), none concurrent (§3.1). Within
  bf-2ildm's own 43 attempts: 4 exit-124 timeouts (exactly 600,0XX ms, attempts 39–42,
  not alerted) and 1 exit-1 at 19 ms (attempt 43, quarantined) — a different,
  non-crash class per the determination.
- **Cascade or isolated?** Cascade — one bead's storm inside a day-long,
  20-bead, 353-kill infrastructure event; INFRASTRUCTURE (repository-bloat regime) at
  the kill level, FALSE_POSITIVE/stale at the alert level (per the determination's
  two-level verdict, unchanged here).
- **What diagnostic data exists?** The retrieval bundle (8 files, hash-verified), the
  six Aug-13 fleet logs (919 bf-2ildm records in the domain-check log), 38 alert-bead
  records, and attempt 1's session transcript. Gone: kernel/OOM journal (single boot
  begins 2026-08-15), `.beads/logs/` telemetry (starts 2026-09-02), the crash-era trace
  slot (overwritten by the Aug-16 success), sar/atop. See the bundle README's
  "Documented absences" for the authoritative list.

## Sources

- Raw: `~/.needle/logs/claude-code-glm-4.7-lab-{domain-check,drawrace,roam-1,roam-2,s1,test-fix}-2026-08-13.jsonl`
  (`agent.completed`, `exit_code: -1`) — all figures in §2/§3 recomputed 2026-09-07
- Bundle: `docs/crashes/bf-2ildm/` (manifest-verified this dispatch)
- Chain: the determination (domchk-a863a1f9), the crash-time resource analysis
  (domchk-e715aed3), the findings synthesis (domchk-970b6ca1)
- Thresholds: `scripts/crash-pattern-detection.sh:46-49`; CLAUDE.md "Recommended Alerts"
- Live: `.beads/logs/*`, `.beads/events.jsonl`, `.beads/traces/bf-2ildm/metadata.json`,
  `crash-pattern-detection.sh` run
