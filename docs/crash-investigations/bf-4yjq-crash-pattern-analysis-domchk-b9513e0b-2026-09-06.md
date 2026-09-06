# bf-4yjq crash — pattern analysis and similar-incident comparison

**Dispatch bead:** domchk-b9513e0b — "Analyze crash pattern and classify failure type"
**Subject bead:** bf-4yjq (closed 2026-08-17; the crash record attached to it is the
2026-08-12 storm window)
**Analysis date:** 2026-09-06

**Scope of this record.** The classification verdict for this crash is already rendered —
[`bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md`](bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md)
(four-way taxonomy verdict) and the canonical
[`bf-4yjq-crash-investigation.md`](bf-4yjq-crash-investigation.md) (mechanism, storm table,
reproducibility). This record does not re-derive those. What it adds is the remainder of the
dispatch's deliverables: the **exit-code -1 semantics review**, a **workspace-wide
pattern analysis re-derived from raw worker logs**, and a **comparison with similar
incidents** across the family this crash belongs to — the "isolated incident or pattern"
question, which no committed document answers across dates.

## Verdict

| Question | Answer |
|----------|--------|
| Classification | **INFRASTRUCTURE** — resource exhaustion (memcg-OOM class) during git operations on the era's ~18 GB bloated repository. Not a workflow failure, not a service failure, not a code defect. |
| Isolated incident or pattern? | **Not isolated — three distinct patterns stack.** (1) It is the middle shift of a same-day 455-kill workspace storm across 6 beads; (2) it belongs to a cross-date incident family (Aug-11 → Aug-17) that also claimed the gc-side and push-side variants; (3) within its own window it is a deterministic retry-kill loop, not a one-off. |
| Still occurring? | **No.** The last `exit_code=-1` record anywhere in the surviving log slots is **2026-08-17T16:00:26Z**; zero in the 20 days since. |

## Exit code -1 — what it does and does not mean

`-1` is **needle's sentinel for a process death whose signal was not recorded**. It is not a
signal number, and by itself it does not prove SIGKILL. For this event the kernel OOM step is
unverifiable in raw logs (system journal starts 2026-08-15 19:46 EDT; no coredumps before
2026-08-25) and is held at MEDIUM-HIGH confidence by the canonical report §6 on the strength of
later, better-instrumented reproductions of the same death class (kernel `CONSTRAINT_MEMCG`
kills inside the `MemoryMax=12 GiB` dispatch scope — bf-4x12ec, bf-198ne). Downstream readers
should restate it as "signal death, OOM-class by family inference", not "proven SIGKILL from
OOM".

Tooling note: `scripts/crash-classifier.sh bf-4yjq` cannot classify this event — it exits with
`ERROR: Bead trace not found: .beads/traces/bf-4yjq/trace.jsonl`. Trace capture did not exist on
Aug-12, so the classification rests on the documented evidence corpus, not on the automated
classifier.

## Pattern analysis (re-derived from raw worker logs, 2026-09-06)

**Method.** All four surviving needle log slots for this workspace form one continuous chain —
`.log.2` 2026-08-11T14:12 → 08-15T20:35, `.log.1` 08-15T20:35 → 08-16T03:09, active `.log`
08-16T03:09 → 09-05, plus the glm-5.3-flash slot 09-05 → now — so "no record exists" below is a
statement over continuous coverage, not a rotation gap. Counted: lines containing
`exit_code=-1` (worker deaths).

**Methodological trap:** do not extend this grep to `exit_code=1`. On 2026-08-15 alone that
pattern matches **3,546 lines, all `ERROR needle::bead_store` records**, not worker completions —
a naive count would manufacture a phantom crash wave. Only the `-1` count behaves as a
completion-class statistic (the Aug-12 figure below matches the canonical report's
deduplicated-by-alert-bead total exactly).

### exit_code=-1 by day (this workspace, all slots)

| Date | Deaths | Phase |
|------|-------:|-------|
| 2026-08-11 | 97 | storm building (bf-31mno 84, bf-5vp 7, bf-2yw76 6) |
| **2026-08-12** | **455** | **peak — bf-31mno 350, bf-4yjq 50, bf-1s6c3 49, bf-2xygo 4, bf-23n 1, bf-5d18 1** |
| 2026-08-13 | 344 | decay begins (repo being packed; bf-65lsdu 127, bf-1ea4g 56, bf-4k2ws 55, bf-2ildm 38 — the bloat-source bead — …) |
| 2026-08-14 | 274 | gc-side variant (bf-173o7e 129, bf-4x12ec 44, …) |
| 2026-08-16 | 157 | push-side variant (bf-198ne et al., broad spread) |
| 2026-08-17 | 1 | bf-4833lh — the last ever |
| 2026-08-18 → 2026-09-06 | **0** | extinct for 20 days |

Three phases, one curve: **bloom** (Aug-11/12, bloat-era object store lethal to any substantive
git operation), **decay** (Aug-13–16 — the repo was packed on Aug-13/14, removing the original
trigger, but pack-objects on the retired bead-forge-era history still produced oversized working
sets in the gc and push paths until those were separately resolved), **extinction** (Aug-17
onward).

### The current regime is a different failure class

With `-1` extinct, the live signal in the same logs is `exit_code=1` waves — Sep-1: 27, Sep-2:
52, Sep-6: 23 completion-class records, consistent with the fleet-wide synchronized exit-1
regime (service-class, 13–15 workers/minute) documented in the September census. That is a
**workflow/service** class of failure, not a recurrence of this infrastructure family.

Also recorded: `scripts/crash-pattern-detection.sh` currently reports **DEGRADED** — its event
source `.beads/events.jsonl` holds 247 crash events with newest 2026-08-26, so the detector is
blind to new events until that source records again (tracked under bead domchk-0c601026; not
owned by this dispatch). Recent-history analysis therefore must use the raw log slots, as above.

### Within-window pattern (bf-4yjq itself)

Re-derived from the preserved extract
([`bf-4yjq-needle-worker-log-extract.log`](../crash-analysis/bf-4yjq-needle-worker-log-extract.log)):
**50 deaths, all `outcome=Crash(-1)`, first 2026-08-12T17:53:53.875682Z, last
20:30:38.310348Z** — matching the preservation pass and the classification record exactly, an
independent third count. Per-run survival 65–375 s (median ~149 s) against a 600 s dispatch
timeout: every agent was killed **mid-task**. This is a deterministic retry-kill loop (zero
exit-code variation across 50 attempts), not scattered flakiness — and at a peak of **5 deaths
in any 10-minute window** it sits *under* the guide's "≥10 crashes in 10 minutes" surge
threshold for its entire 2h37m; only an aggregate per-hour view (~19/hour) would have caught it.
That detection gap is canonical report recommendation #1 and is not re-litigated here.

## Comparison with similar incidents

| Incident | Date | Deaths (log recount) | Git operation | Mechanism | Investigated? |
|----------|------|--------------------:|---------------|-----------|---------------|
| bf-31mno | Aug-11/12 | **434** (84+350) | general, on bloated repo | same family — largest storm of the era | **No dedicated RCA exists** — largest event, least documented |
| **bf-4yjq** (this) | Aug-12 | **50** | general, on bloated repo | same family — middle shift of the peak day | Canonical + classification record |
| bf-1s6c3 | Aug-12 | 49 | general, on bloated repo | same family — the CLAUDE.md-documented bloat crash | `docs/crashes/repository-bloat-crash-bf-1s6c3-2026-08-12.md` |
| bf-65lsdu / bf-1ea4g / bf-4k2ws / bf-2ildm | Aug-13 | 127 / 56 / 55 / 38 | general, during packing | same family, decay phase | counts only here; not independently verified per bead |
| bf-173o7e | Aug-14 | 129 | `git gc --aggressive` | **gc-side variant** — pack-objects at the 12 GiB scope bound | `bf-173o7e-crash-investigation.md` |
| bf-4x12ec | Aug-14 | 44 | `git gc --aggressive` | gc-side variant, kernel `CONSTRAINT_MEMCG` proven | `docs/crash-investigation-bf-4x12ec.md` |
| bf-198ne | Aug-16 | (2 agents in its dispatch series) | `git push` | **push-side variant** — pack-objects at the scope bound, kernel-proven | `docs/crashes/bf-198ne-crash-report.md` (verified 2026-09-06) |
| bf-4833lh | Aug-17 | 1 | — | last `-1` event on record | not investigated here |

**Family resemblance:** one mechanism, four triggers. Every member of this table is a signal
death of a dispatch agent whose git operation's working set exceeded the dispatch scope's
12 GiB `MemoryMax`. What changed over the six days is only *which* operation supplied the
working set: any operation on the 18 GB object store (Aug-11/12) → gc and push over the retired
bead-forge history (Aug-14/16). The family ended when both were removed — repo packed Aug-13/14,
gc bounded by `pack.windowMemory` — not when any bead was "fixed".

**What distinguishes bf-4yjq within the family:** nothing about its task content. Its crash
exposure was a function of scheduling — it drew the peak day's middle shift. The subject task
itself (git remote reconciliation) was completed and verified five days later, after the
environment healed.

## Live verification (this dispatch, 2026-09-06)

| Check | Result |
|-------|--------|
| `git count-objects -vH` / `du -sh .git` | 10 loose objects / 68 KiB, 1 pack 90.93 MiB, `.git` 93 MB — all green; trigger condition absent |
| Host memory | 46 GB available of 62 GB |
| Death count re-derivation | 50/50 `Crash(-1)`, window matches preservation pass and classification record |
| Same-day workspace total re-derivation | 455 on Aug-12, matching canonical §4 from an independent source (raw logs, not forensic.jsonl) |
| Post-Aug-17 recurrence | zero `exit_code=-1` records across continuous slot coverage through 2026-09-06 |

## Handoff to the next chain step

The successor dispatch (domchk-54bc57df, "Investigate root cause and contributing factors")
should not re-derive the causal chain — canonical report §6 already establishes it (bloat-source
commits bf-2ildm-era → 18 GB/17.2 GiB loose store → oversized git working sets → memcg kills in
the 12 GiB dispatch scope, confidence HIGH on correlation / MEDIUM-HIGH on the OOM step, with
the confirmed-absences list in the preservation pass bounding what new evidence can exist). The
contributing-factor list worth its attention: the 12 GiB scope bound, the absent storm-level
detection (§9 rec. 1), the evidence-retention gap, and the still-open Aug-12 alert-bead backlog
(§9 rec. 2).

## Sources

| Document | Role |
|----------|------|
| [`bf-4yjq-crash-investigation.md`](bf-4yjq-crash-investigation.md) | Canonical analysis — mechanism, storm table, reproducibility |
| [`bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md`](bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md) | Four-way taxonomy verdict, exit-code semantics, false-positive rules |
| [`../crash-analysis/bf-4yjq-needle-worker-log-extract.log`](../crash-analysis/bf-4yjq-needle-worker-log-extract.log) | Preserved primary crash-era telemetry (225 records) |
| [`bf-4yjq-artifact-preservation-2026-09-06.md`](../crash-analysis/bf-4yjq-artifact-preservation-2026-09-06.md) | Provenance, integrity hashes, confirmed absences |
| `docs/crash-investigation-bf-4x12ec.md`, `docs/crashes/bf-198ne-crash-report.md`, `bf-173o7e-crash-investigation.md` | Family members' own records (gc and push variants) |
| Raw needle log slots (`.log`, `.log.1`, `.log.2`) | By-day/by-bead recount, Aug-18+ zero-`-1` coverage — method above |
