# Investigation Report: Crash bf-2ildm

**Report bead:** domchk-87b7b0e8 (2026-09-07) — synthesis of the investigation
chain under alert bead **bf-z15pix**
**Crash target:** bf-2ildm — "Extract GitHub-specific commits" (**Closed**
2026-08-16T22:44:38.873946777Z, rev 7, zero reopen events)
**Supersedes:** the 2026-09-02 report
[`bf-2ildm-final-investigation-report-2026-09-02.md`](bf-2ildm-final-investigation-report-2026-09-02.md)
and [`root-cause-determination-bf-2ildm-2026-09-02.md`](root-cause-determination-bf-2ildm-2026-09-02.md)
on the kill question (see §5). Consolidates: domchk-0022f45f (classification,
`5d9459e`), domchk-e715aed3 (crash-time resources, `825710e`), domchk-579d25b0
(crash artifacts/pattern, `f6ee892`), domchk-970b6ca1 (findings and next steps,
`07eb8cb`), and domchk-a863a1f9 (root-cause determination, `ef960d2`, amended
`a78f118`).

## Executive Summary

- **Root Cause:** **INFRASTRUCTURE** — the workspace's repository-bloat regime.
  On 2026-08-13 this workspace still carried ~18 GB of `.git` (~17 GB of loose
  objects from 17+ identical 237 MB `.beads/*.jsonl` snapshots) and a
  422-commit unpushed backlog; git/bead-state work inside needle's
  12 GiB `MemoryMax` per-dispatch scope exceeded the cgroup limit and the
  kernel's memory-cgroup OOM killer SIGKILLed the worker. The precise memcg
  mechanism is assigned **by regime match** (MODERATE) — no Aug-13 kernel
  record survives — but the regime itself is proven (HIGH) and was repaired
  2026-09-06, verified holding live in §6.
- **Classification:** **two-level, and both levels hold.** At the **alert**
  level: **FALSE_POSITIVE** — all 38 alert beads fired at real kills, but each
  implied lost work, and no work was lost; the alerts aged into staleness
  because the alert layer had no lifecycle (no closed-bead filter, dedup gate,
  cooldown, completion awareness, or working classification path) to retire
  them when the target resolved. At the **kill** level: **INFRASTRUCTURE**
  (above). **CODE_DEFECT ruled out** (HIGH) — zero panics or stack traces in
  any of the 43 attempts; the identical task succeeded on 2026-08-16 in 85 s.
- **Confidence:** **HIGH** on the two load-bearing verdicts (work not lost;
  kills real, infrastructure-class, code excluded). The one attenuated figure
  is the precise kill mechanism (memcg-OOM by regime match, MODERATE), an
  evidence-retention limit, not a competing hypothesis.
- **Work lost: none.** bf-2ildm completed on a 2026-08-16T22:28:44Z retry
  (exit 0, 85,327 ms) and closed 15 m 54 s later with every acceptance
  criterion met. The storm's cost was ~2.3 h of wall clock across 38 killed
  attempts — not work.

## Crash Characteristics

| Field | Value | Note |
|---|---|---|
| Alert bead | bf-z15pix — "ALERT: Agent crash on bead bf-2ildm" | one of 38, all for this target |
| Carried stamp | **2026-08-13T15:01:52.450373520+00:00** | **not a death time** — it is the crash handler's post-kill `HANDLING_RELEASE_DONE` heartbeat, **+16.7 s** after the real kill |
| Real kill | **2026-08-13T15:01:35.775155479Z** — attempt **24** of 43, exit −1, 309,461 ms, mid-task | `agent.completed` in the worker log; `outcome.classified` → `crash` 2.2 ms later |
| Exit code | **−1 ("signal −1")** | needle's died-without-exit-code sentinel (`code().unwrap_or(-1)`). **Not** a signal number (a delivered SIGKILL encodes 137) and **not** placeholder data — all 38 alert exit codes match the crash-era worker log exactly |
| Agent | `claude-code-glm-4.7` (provider `zai`, model `glm-4.7`), worker `claude-code-glm-4.7-lab-domain-check` | single worker, domain-check workspace |
| Target state at fire time | Open, mid-task | the alert was **correct when it fired**; it became a false positive retroactively, when the target succeeded on 2026-08-16 |

**Full attempt census (2026-08-13, 13:35:34Z → 16:35:40Z)** — re-counted from
`docs/crashes/bf-2ildm/attempt-index.tsv` (43 rows + header):

| Exit | Attempts | Meaning | Alerted? |
|---|---|---|---|
| **−1** | **38** (attempts 1–38; kills 13:37:24.838573229Z → 15:53:28.914636262Z; durations 98–309 s, all mid-task) | signal death — the crash population | 1 alert bead each |
| 124 | 4 (attempts 39–42, each exactly ≈600,000 ms) | the 10-minute timeout cap — a different, non-crash class | no |
| 1 | 1 (attempt 43, 19 ms) | died before work started; bead quarantined 16:35:40.675Z at `failure_count` 5/5 | no |

All 38 alert beads map **1:1** onto the 38 kills (nearest-after match,
**7.7–29.4 s** stamp gap; 0 duplicates, 0 orphans). Worked example, attempt 19
(the chain's citation instant): claim 14:38:24.455725579Z → kill
**14:40:29.551762239Z** (−1, 124,759 ms) → heartbeat stamp 14:40:42.628685942Z
(**+13.1 s**) → alert bead bf-66sw7c created 14:40:42.642439194Z (+14 ms after
the stamp).

## Investigation Findings

### 1. Crash classification (exit code −1, signal −1)

From domchk-0022f45f (`5d9459e`), consolidating the per-instant records
(`d82b6a2`, `e3a8820`):

| Level | Question | Verdict | Confidence |
|---|---|---|---|
| Alert | Did the alerts imply something true? | **FALSE_POSITIVE** — fired at real kills, implied lost work; nothing was lost | HIGH |
| Kill | What killed the 38 attempts? | **INFRASTRUCTURE** — repository-bloat regime | regime HIGH; mechanism MODERATE (regime match) |
| Code | Domain-check defect? | **CODE_DEFECT ruled out** | HIGH |

Live checks re-run for this report: `./scripts/alert-deduplication.sh check
bf-66sw7c` and `… bf-z15pix` → **"DUPLICATE: crash target bf-2ildm is already
resolved"** (the alert layer itself now agrees both are stale);
`./scripts/crash-classifier.sh bf-2ildm` → **UNKNOWN** with the provenance
warning "trace slot does not describe the incident run". Automated UNKNOWN is
**not** evidence of no crash — the classifier's machine-readable sources
(`.beads/events.jsonl`, floor 2026-08-16) postdate the storm, and the single-slot
trace holds only the retry. The sole surviving crash-era witness is needle's
worker log (919 bf-2ildm records on 2026-08-13).

The sentinel semantics resolve the 2026-09-02 "placeholder exit code" premise:
−1 is what needle records when a process dies without an exit code; the
trace's exit 0 is the **Aug-16 retry** that later overwrote the slot, not a
contradiction of the alert.

### 2. Work completion status — FALSE_POSITIVE vs. real crash

From domchk-970b6ca1 (`07eb8cb`):

- **The crashes were real** (38 mid-task signal deaths, verified against the
  worker log 38/38), **and the work was never lost.** Both are true; the
  either/or framing in the original task is the trap this section closes.
- bf-2ildm was created 2026-08-13T11:12:57Z, was killed through 38 attempts on
  the 13th, then succeeded on a **2026-08-16T22:28:44Z** retry — exit 0,
  85,327 ms per `.beads/traces/bf-2ildm/metadata.json` (85,542 ms per
  `.beads/events.jsonl`; a 215 ms instrument difference between sources) — and
  closed at **22:44:38.873946777Z** (rev 7). Never reopened. Last kill →
  closure: **3 d 6 h 51 m**; every alert predates completion by 3 d 6 h 35 m
  to 3 d 8 h 51 m, i.e. every alert was live and valid **at generation**.
- Why the target succeeded where its 38 predecessors died: the retry ran
  **after** the repository repair (2026-09-06 record documents the regime
  ending; the Aug-14 reboots and cleanup ended it), not because anything about
  the task changed. The task itself is crash-irrelevant — `git log
  <ancestor>..<github-branch>` extraction, third step of a branch-divergence
  chain — and its 2026-08-16 success on the same code is half of the
  CODE_DEFECT exclusion.
- Alert-bead statuses live as of 2026-09-07 (recounted this dispatch):
  **28 closed / 7 open / 3 in_progress** (the in_progress three are stale
  assignments last touched 2026-09-02, not active work). The 10 unsettled
  alerts are the residue of the no-lifecycle defect; remediation bead
  **domchk-4a05973f** carries their per-bead closure.

### 3. System resource state at crash time

From domchk-e715aed3 (`825710e`), first-hand from the raw Aug-13 fleet logs:

- **What survives:** almost nothing, and one survivor.
  `/var/log/journal` is persistent but holds a **single boot beginning
  2026-08-15 19:56:33 EDT** — 2.4 days after the crash — so no kernel OOM line
  exists for any Aug-13 kill (epoch-bounded query returned empty). No
  sar/atop/sysstat. `.beads/logs/` starts 2026-09-02. The trace slot was
  overwritten. The **only** crash-time resource telemetry is needle's
  `fleet.cpu_saturated` events.
- **Timezone trap, documented for the next investigator:** this box runs
  EDT (UTC−4) and `journalctl --since "2026-08-13T14:50:00"` parses naive
  timestamps as **local** time — the naive form would have queried
  18:50–19:10Z and reported "no OOM events" from an empty window. Query kill
  windows by `@epoch` (`date -d "2026-08-13T15:01:52Z" +%s` → 1786633312).
- **Load at the instant** (saturated-side samples only, so these are lower
  bounds; threshold is 7.2 = 0.8 × 9 cores): **10.22** at attempt 24's
  dispatch (14:56:25Z) → **15.40** at 15:01:21Z, **14 s before the kill** →
  **16.97** at the next dispatch (15:01:59Z) → **24.07–24.63** peak at
  15:07:08–14Z. Load rose *through* the kill. CPU saturation alone kills
  nothing, and **no memory or disk telemetry of any kind survives** — the
  box-memory-pressure and disk-exhaustion hypotheses are therefore untestable
  directly.
- **Neighbor survival census (new affirmative evidence for
  workspace-locality):** in 14:55–15:10Z, five *other* workers on four *other*
  repos completed **7 runs with exit 0** in the same 15 minutes — including
  two of 533 s and 595 s — around attempt 24's death. A box-wide memory event
  does not spare those. The kill was confined to the domain-check dispatch
  scope. Honest limit: neighbors ran other repos, so this rules out box-wide
  exhaustion, not the scope-local mechanism.
- **Era repository state vs. measured now:**

| Metric | 2026-08-13 (era, documented) | 2026-09-07 (measured live) |
|---|---|---|
| `.git` size | ~18 GB | **105 MB** |
| Loose objects | ~17 GB (17+ identical 237 MB `.beads/*.jsonl` snapshots) | 236 objects / 1.71 MiB |
| Packed | fragmented | 11,700 in-pack, 2 packs, 99.78 MiB, 0 garbage |
| Unpushed backlog | 422 commits | **0** |

### 4. Crash artifact analysis

From domchk-579d25b0 (`f6ee892`) and the retrieval bundle (domchk-ea755548):

- **Evidence bundle** `docs/crashes/bf-2ildm/` — 8 files, `sha256sum -c
  MANIFEST.sha256` **8/8 OK**; `attempt-index.tsv` (43 attempts, the
  authoritative ordinals), `crash-alert-ledger.tsv` (38 alerts + header), README
  with the documented absences, `trace-archive-current-state.json`, attempt 1's
  transcript.
- **Surge-threshold verdicts, computed from the raw fleet log.** The three
  circulating thresholds disagree at per-bead scope, and the discrepancy is the
  finding: bf-2ildm's ~2–5 min re-dispatch cadence (38 kills over 2 h 16 m)
  **never stacks 10 kills into 10 minutes** (max **4**), so the CLAUDE.md
  "10+ crashes in 10 minutes → infrastructure event" line, applied to one
  bead, is a **false negative** for this storm. The readings that fire: the
  storm rate (**15 kills in the 14:00Z hour; 16.8/h sustained** vs the 10/h
  threshold) and every threshold at fleet scope (7 in 5 min; 12 in 10 min;
  56 in the 23:00Z hour). Also structural: `crash-pattern-detection.sh` reads
  only `events.jsonl` (floor 2026-08-16), so no automated detector could have
  seen this storm at all — the fleet logs are the sole witness and sit outside
  every automated detector's input.
- **Cascade structure.** 2026-08-13 saw **353 exit −1 kills across 20 beads**,
  all on the single domain-check worker, 00:00:15Z → 23:59:58Z — the bloat
  regime active essentially all day. The storms are **sequential, not
  simultaneous**: bf-2ildm is storm **16 of 20** (13:37 → 15:53), and it is the
  only major storm with **zero concurrent bead deaths** — its 38 kills *are*
  the fleet's exit −1 activity in that window (bf-3hivb's storm ended 474 s
  before its first kill; the next bead's kill came 9,639 s after its last).
  The day's surge peak (12 kills/10 min, 22:13–22:23Z) belongs to bf-65lsdu's
  127-kill storm, five hours later — citing "the Aug-13 surge peak" for
  bf-2ildm would be wrong by five hours. The sequence is not contagion: each
  retry loop re-enters the same bloated workspace, and a storm's end heals
  nothing.

## Root Cause

**Definitive determination, stated per level** (confidence in parentheses):

1. **Kill level — INFRASTRUCTURE (regime HIGH).** The 38 attempts died in the
   repository-bloat regime: ~18 GB of `.git` with ~17 GB of loose objects and
   a 422-commit unpushed backlog made git/bead-state operations exceed the
   12 GiB `MemoryMax` of needle's per-dispatch systemd scope, and the kernel's
   memory-cgroup OOM killer SIGKILLed the worker (exit −1 is the
   died-without-exit-code sentinel observed from outside the killed process).
   Every dispatch re-entered the same workspace, which is why deaths recurred
   at a fixed ~2–5 min cadence for 2 h 16 m and why the day produced 353 kills
   across 20 beads.
   **Mechanism MODERATE** — assigned by regime match, because the single-boot
   journal begins 2.4 days after the storm and no `CONSTRAINT_MEMCG` line
   survives for this bead. The mechanism is the era's proven one, recovered
   with kernel records for bf-4x12ec, bf-198ne and bf-1s6c3; this is an
   evidence-retention limit, not an open question about the fix.
2. **Alert level — FALSE_POSITIVE / stale (HIGH).** The crash-alert layer had
   **no lifecycle**: alerts were created at kills and never re-validated
   against the target afterwards — no closed-bead filter, no completion
   awareness, no dedup gate, no cooldown, and (once classification existed) a
   wiring bug that made the FALSE_POSITIVE branch dead code. Any one of those
   would have retired the alerts on 2026-08-16; with none, 38 alerts for one
   resolved target survived to be processed as if the target were still
   crashing. **The kill alerts were true; the crash *case* they imply was
   false.**
3. **Not the cause (all ruled out):**

| Alternate | Why ruled out |
|---|---|
| **CODE_DEFECT** | zero panics/stack traces in any of the 43 attempts; the same task on the same code succeeded in 85 s on 2026-08-16; consistent with 157+ prior investigations of this workspace finding no domain-check defect |
| **SERVICE_FAILURE** | deaths are mid-task signal losses at variable durations (98–309 s), not HTTP 5xx/timeout exits; no gateway involvement in the attempt profile |
| **WORKFLOW_FAILURE (max turns)** | that shape exits 1 `error_max_turns`, not −1; the four exit-124 attempts are the timeout class, distinct from the kills |
| **bead-store corruption / wrong tool** | no schema errors in any record; every death precedes any store mutation |

4. **How the first (2026-09-02) investigation got it wrong** — recorded so the
   error is not repeated: it was told "exit −1," found the single-slot trace's
   Aug-16 exit 0, and concluded the −1s were fabricated and the alert
   impossible. Two evidence-selection errors: reading the *retained* trace
   (the retry's) as the record *of the reported crash*, and calling
   alert-before-completion impossible when that is the expected order for a
   real crash whose retry later succeeds. Its task-completion finding and its
   list of alert-layer defects survive; its "did NOT crash" and
   "placeholder exit code" premises are superseded.

## Recommendations

### In force and verified holding (no new work opened)

The layered defense that ends this regime, re-verified live for this report:

1. **`.beads/` fully gitignored** (plus `*.db`, `*.jsonl` repo-wide) — bead
   state cannot re-bloat the repository; 0 tracked `.beads/` files.
2. **10 MB pre-commit gate** — installed and current (`setup-git-hooks.sh
   --check` exit 0); the backstop that would have blocked the 237 MB snapshot
   commits.
3. **Persistent pack-memory bounds** (`pack.windowMemory=2g`,
   `pack.deltaCacheSize=1g`, `pack.threads=1`), repo-local **and** global —
   bare `git gc` and `git push` are bounded too; effective worst case
   ≈3 GiB per pack run within the dispatch scope's ceiling.
4. **`safe-git-gc.sh`** bounded path with preflight, checkpoint/resume and
   monitoring — never bare `git gc --aggressive`.
5. **systemd user timers** — daily repo health + incremental gc, weekly full
   gc; **8/8 timers** with future trigger times.

### Gaps worth closing (each with its owner/candidate status)

| # | Gap | Why it mattered here | Status |
|---|---|---|---|
| 1 | **Single-slot trace retention** — the last attempt overwrites all prior ones | manufactured the wrong 2026-09-02 RCA; starves `crash-classifier.sh` into UNKNOWN | Compensating control in place: retrieval bundles with per-attempt indexes (domchk-ea755548). Candidate improvement, **no owner**: multi-slot retention keyed by (bead, attempt) |
| 2 | **Alert lifecycle** — nothing retires an alert when its target resolves | 10 of 38 alerts still open 25 days after closure | **Fixed**: closed-bead filter + dedup gate + cooldown (suites `test-crash-alert-fixes.sh` 13/13, `test-closed-bead-filter.sh` 7/7, re-run from repo cwd for this report). Storm residue assigned to domchk-4a05973f; a generic target-resolved sweep remains a candidate |
| 3 | **Classification wiring** — the manager grepped the classifier's banner as `CLASSIFICATION`, so the FALSE_POSITIVE branch never fired | dead FP branch + constant-string cooldown + garbage history field | **Fixed** at `8cc1172` (anchored-token grep with `head -1` fallback, `scripts/crash-alert-manager.sh:401-402`), verified pushed; tracking bead **domchk-f6fff20f** still open with its owner. Lesson: grep-marker suite tests cannot see wiring bugs — a functional replay can |
| 4 | **Alert-stamp provenance** — alert beads carry the handler heartbeat (+7.7–29.4 s), never the kill instant | the "crash instant" in this alert's own task text is a heartbeat 16.7 s after the death | Documented control: cite kills from `attempt-index.tsv`, never the bead stamp. Candidate: stamp the kill instant on the alert bead |
| 5 | **Per-bead surge thresholds** — "10 crashes in 10 min" misses a single bead being killed continuously for hours (max here: 4 in 10 min) | bf-2ildm's storm is invisible to that line | Use the hourly/storm rate (16.8/h vs the 10/h threshold) and fleet-scope counts; `crash-pattern-detection.sh` already keys detectors system-wide. Numbers recorded here for the next per-bead investigator |
| 6 | **Signal retention floor** — `.beads/events.jsonl` begins 2026-08-16, so any older storm classifies UNKNOWN by machine | UNKNOWN ≠ no crash | Documented limitation; manual pass over the worker log is the method. Same root cause as gap 1 |
| 7 | **Per-clone hook protection** — the 10 MB gate is per-clone | a fresh clone is unprotected | Documented in CLAUDE.md; run `./scripts/setup-git-hooks.sh install` on new clones |

## First-hand verification battery (this report, 2026-09-07)

| Check | Command | Result |
|---|---|---|
| Target state | `bead show bf-2ildm` | Closed, rev 7 (updated 2026-09-07T17:47:17Z) |
| Retry trace | read `.beads/traces/bf-2ildm/metadata.json` | exit 0, `success`, 85,327 ms, captured 2026-08-16T22:28:44.172Z |
| Attempt census | `awk` over `attempt-index.tsv` col 6 | 43 = **38 × −1, 4 × 124, 1 × 1** |
| Alert mapping | distinct/empty `alert_bead` among −1 rows | **38 distinct, 0 empty**; gaps **7.7 s / 29.4 s** |
| Row 19 / row 24 | bundle rows | 14:40:29.551Z kill → bf-66sw7c +13.1 s; 15:01:35.775Z kill → bf-z15pix +16.7 s |
| Alert ledger | `wc -l crash-alert-ledger.tsv` | 39 = 38 alerts + header |
| Alert-bead statuses | full-store recount | **28 closed / 7 open / 3 in_progress** |
| Dedup gate | `alert-deduplication.sh check bf-66sw7c` / `bf-z15pix` | **DUPLICATE (target resolved)**, both |
| Classifier | `crash-classifier.sh bf-2ildm` | **UNKNOWN** + trace-provenance warning (expected, §Findings 1) |
| Alert fixes present | `grep -n "CRITICAL FIX" scripts/crash-alert-manager.sh` | FIX 1–6 at lines 210, 218, 294, 317, 354, 602 |
| Classification wiring | `sed -n '401,402p'` + `git merge-base --is-ancestor 8cc1172 origin/main` | anchored-token grep + `head -1` fallback; **pushed** |
| Alert-fix suite | `./scripts/test-crash-alert-fixes.sh` (repo cwd) | pass, exit 0 (13 assertions) |
| Closed-bead filter suite | `./scripts/test-closed-bead-filter.sh` (repo cwd) | pass, exit 0 (7 assertions) |
| Repo health | `./scripts/check-repo-health.sh` | **exit 0** |
| Repository | `du -sh .git`; `git count-objects -vH` | **105 MB**; 236 loose / 1.71 MiB; 11,700 in-pack, 2 packs, 99.78 MiB, 0 garbage |
| Backlog | `git rev-list --count origin/main..HEAD` | **0** |
| System memory | `free -g` | 45 G available |
| Timers | `systemctl --user list-timers 'domain-check-*'` | 8/8 present |

Measured values drift slightly between same-day dispatches (loose objects
129 → 167 → 236 across the day; load and disk move with co-tenant activity) —
ordinary churn on a shared box, not contradictions. System load at write time
(1-min ≈ 13) and disk free (~7 G transiently, ~70 G after an idle `target/`
was cleared per the disk policy) are co-tenant conditions unrelated to this
crash and are noted only for snapshot honesty.

## What this report closes and does not

**Closes:** the synthesis obligation for the bf-2ildm chain — one report
carrying the classification, the work-completion answer, the crash-time
resource picture, the artifact/threshold analysis, the definitive two-level
root cause with confidence, and the mitigation posture with each gap's status.

**Still open, owned elsewhere:** the 10 unsettled storm alerts →
**domchk-4a05973f**; the CLASSIFICATION-wiring tracking bead **domchk-f6fff20f**
(mechanism fixed and pushed); the wider pool of auto-split investigation beads
mentioning bf-2ildm — stale by the same proof, but per corpus convention each
is verified and closed by its own chain, not bulk-closed from here; the parent
alert **bf-z15pix** receives this report's summary as its notes (done by this
bead).

## Sources

- Child reports (all 2026-09-07, this chain): domchk-0022f45f →
  [`bf-2ildm-classification-report-domchk-0022f45f-2026-09-07.md`](bf-2ildm-classification-report-domchk-0022f45f-2026-09-07.md)
  (`5d9459e`); domchk-e715aed3 →
  [`bf-2ildm-crash-time-resources-domchk-e715aed3-2026-09-07.md`](bf-2ildm-crash-time-resources-domchk-e715aed3-2026-09-07.md)
  (`825710e`); domchk-579d25b0 →
  [`bf-2ildm-crash-pattern-analysis-domchk-579d25b0-2026-09-07.md`](bf-2ildm-crash-pattern-analysis-domchk-579d25b0-2026-09-07.md)
  (`f6ee892`); domchk-970b6ca1 →
  [`bf-2ildm-findings-and-next-steps-domchk-970b6ca1-2026-09-07.md`](bf-2ildm-findings-and-next-steps-domchk-970b6ca1-2026-09-07.md)
  (`07eb8cb`); domchk-a863a1f9 →
  [`bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md`](bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md)
  (`ef960d2`, amended `a78f118`)
- Evidence bundle: `docs/crashes/bf-2ildm/` — `attempt-index.tsv`,
  `crash-alert-ledger.tsv`, `README.md`, `trace-archive-current-state.json`
  (domchk-ea755548)
- Per-instant classifications: `docs/crash-classification-bf-2ildm-2026-08-13-14-40.md`
  (`d82b6a2`), `docs/crash-classification-bf-2ildm-2026-08-13-15-01.md` (`e3a8820`)
- Raw fleet log: `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl`
  (session `e29942f7`) and the five sibling Aug-13 worker logs
- Era evidence: commit `d9c4622` (Aug-13 per-bead kill census);
  `docs/crash-analysis-bf-1s6c3-2026-09-06.md` (bloat mechanism);
  `docs/maintenance/repository-maintenance-guide.md` (mitigation layer)
- Superseded (context only):
  [`root-cause-determination-bf-2ildm-2026-09-02.md`](root-cause-determination-bf-2ildm-2026-09-02.md),
  [`bf-2ildm-final-investigation-report-2026-09-02.md`](bf-2ildm-final-investigation-report-2026-09-02.md)
- Live (this dispatch): `bead show` on bf-2ildm / bf-z15pix; the two alert
  suites; the dedup and classifier scripts; `check-repo-health.sh`; git
  object counts; `free -g`; systemd timer list

---
*Authored by domchk-87b7b0e8, 2026-09-07. Synthesis only — no code, scripts, or
alert-system behaviour was changed by this bead.*
