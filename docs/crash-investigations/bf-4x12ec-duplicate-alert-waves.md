# bf-4x12ec — The Two-Wave Duplicate-Alert Pattern

**Bead:** domchk-1e38a7c2 · Child 3 of the `domchk-bcdd4b4f` split (*Check for duplicate crash
alerts on bf-4x12ec*)
**Written:** 2026-09-08
**Canonical analysis:** [bf-4x12ec crash investigation](bf-4x12ec-crash-investigation.md) —
Addendum 3 (independent verification + kernel evidence), Addendum 2 (primary-source retry-storm
table). That report owns the *crash*; this document records the *alert-layer* pattern around it
and deliberately does not restate it.

Companion documents in this split:

| Split child | Deliverable |
|---|---|
| Child 1 — `domchk-f05d6f91` | [Alert-bead inventory](bf-4x12ec-alert-inventory.md) — every bead touching bf-4x12ec, with statuses |
| Child 2 — `domchk-b2b77f12` | Per-bead false-positive verdict table (appended to the same inventory) |
| Child 3 — `domchk-1e38a7c2` (**this doc**) | The two-wave duplicate-alert pattern |
| Child 4 — `domchk-a3690174` | Mitigations vs. regeneration |
| Child 5 — `domchk-e9ed52aa` | Bulk-close residual wave-1 alert beads |

## The pattern in one table

| | **Wave 1 — retry storm** | **Wave 2 — regeneration** |
|---|---|---|
| Date | 2026-08-14, 10:23:11Z → 11:28:02Z (64.9 min) | 2026-08-26, 20:17:41Z → 21:13:57Z (56 min) |
| Beads created | 44 `bf-*` ALERT beads (`bf-fmg2cw` → `bf-5x69lm`) | 16 `domchk-*` investigation beads (+ 3 body-only the same evening, + 1 pre-wave on 08-17) |
| Generator | the live kill loop — one alert bead per exit -1 | re-derivation of the historical Aug-14 alerts into new work |
| Target state at creation | **open** — every alert was real | **closed** since 2026-08-17T14:50:41Z — every bead spurious |
| Delay from reference point | — | 9.2 days after closure; 12.4 days after the storm |
| Cadence | mean 90.5 s (min 60, max 155) | mean 225 s |
| What ended it | resource easing (crashes → timeouts → success), not an alert fix | no regeneration since; mitigations landed 2026-09-02 |
| Verdict | real at creation, FALSE POSITIVE retrospectively | FALSE POSITIVE at creation |

## Wave 1 — retry storm (2026-08-14 10:23–11:28Z)

Each needle attempt ran `git gc --aggressive --prune=now` against a repo carrying ~17.2 GB of
loose objects (≈18 GB `.git`). Every dispatch ran inside a transient `run-p<id>-i<id>.scope` with
`MemoryMax=12GiB`; the pack could not fit under that cap, the memcg hit it, and the kernel OOM
killer SIGKILLed the highest-badness task in the scope (`CONSTRAINT_MEMCG`,
`oom_score_adj=200`). The worker recorded `exit_code: -1`, classified `crash`, minted one
`ALERT: Agent crash on bead bf-4x12ec` bead (needle pre-0.4.2 mints one bead **per kill**),
released bf-4x12ec, and immediately re-claimed it — Addendum 3 resolves attempt 26's handling
window: death 11:01:31.9 → release 11:01:43.9 → re-claim 11:01:46. A fresh agent then started the
same gc and died the same way. 44 times in 65 minutes.

**The ~90 s cadence is the attempt lifetime plus handling — it is not a configured rate.**
Decomposing all 44 alerts and 43 inter-alert gaps from the storm evidence
(`evidence/bf-4x12ec/crash-logs/exit-code-timeline.txt` and `alert-beads-exit-timestamps.txt`):

| component | mean (range) |
|---|---|
| attempt lifetime, dispatch → SIGKILL (44 attempts) | 64.6 s (38.9–115.8) |
| kill → alert bead created (the `HANDLING_RELEASE_DONE` heartbeat) | 15.7 s (7.0–36.0) |
| release → re-claim → next dispatch (derived by subtraction) | ~11 s |
| **sum** | **≈ 91 s** |
| **observed mean inter-alert gap** | **90.5 s (60–155)** |

The identity closes to within ~1 s. Two consequences worth keeping:

- The storm's bead-production rate was set by **how fast the box could kill an agent**, not by any
  alert throttle. Nothing in the alert layer could have slowed wave 1; only dedup or aggregation
  (one bead per *incident*) could have shrank it.
- The gap's 60–155 s spread tracks the per-attempt lifetime spread — an early-killed attempt
  produces the next alert sooner.

**Three phases, one alert asymmetry.** The retry loop ran 53 attempts in three distinct regimes:

| Phase | Attempts | Window (completion stamps) | Outcome | Alert beads |
|---|---|---|---|---|
| 1 — memcg-OOM crash | 1–44 | 10:23:02 → 11:27:26 (each 38.9–115.8 s) | exit -1 | **44 — one each** |
| 2 — harness timeout | 45–52 | 11:38:07 → 12:50:14 (each exactly 600.0 s) | exit 124 | **0** |
| 3 — success | 53 | 12:58:45 (491.8 s) | exit 0 | 0 — bead closed instead |

Only `outcome=crash` mints an alert bead. The 8 timeouts and the success minted nothing, so the
alert count froze at 44 at 11:28:02Z even though the retry loop kept working until 12:58:45Z —
wave 1's *bead* footprint ended ~90 minutes before its *work* footprint did. This asymmetry is why
child 2's per-bead table has 44 entries and not 53.

**How wave 1 actually ended.** Not by any alert-layer change: the box's pressure eased. Addendum 3
shows `fleet.cpu_saturated` firing on nearly every storm dispatch (load 10.4–30.9 on 9 reported
cores), falling to 7.84 by 12:50 and 9.86 at the successful attempt — memory pressure eased with
it, which moved the failure mode from "killed in ~65 s" to "killed at the 600 s cap" to "completes
in 492 s". The sweep of all six Aug-14 worker logs confirms the storm never left this bead's retry
cycle: 45 signal-deaths in the 10:20–13:00 window, 44 × bf-4x12ec and 1 × bf-173o7e (the
separately-documented neighbouring crash), zero on any other worker.

## Wave 2 — regeneration (2026-08-26)

The retry storm's alerts were real; the regeneration wave was not. On 2026-08-26, 20:17:41 →
21:13:57Z, 16 new `domchk-*` investigation beads were minted against bf-4x12ec — **9.2 days after
it closed** (2026-08-17T14:50:41Z) and 12.4 days after the storm itself. No new evidence existed;
the beads re-derived the historical Aug-14 alerts as if they were fresh.

**Dating correction (carried forward from child 1).** The split description's "2026-08-25/26" is
not borne out by creation timestamps: **all 16 wave-2 beads fall on 2026-08-26; none on 08-25.**
The only Aug-25 activity in the corpus is *handling* of a wave-1 alert (bf-4833lh's investigation,
afdbc2d). One earlier bead sits between the waves — `domchk-c95117c0` (2026-08-17T15:59:21Z, 69
minutes after the target closed) — and three more genuine investigations the same evening name
bf-4x12ec only in their bodies (`domchk-862d95d1`, `domchk-30d451d3`, `domchk-0bda808c`). Wave 2
is therefore 16 title-matching, 19 same-evening.

**Cadence.** Mean gap 225 s over the 56-minute span — ~2.5× wave 1's. Unlike wave 1 there is no
kill loop to pace it; the interval is set by whatever re-derived the beads, which is itself
evidence the two waves have different generators.

**The sibling measurement.** Umbrella `domchk-30b53d74` (parent bead `bf-25uq3d` — wave-1 alert
#43) diagnosed the regeneration as a systemic detection flaw and measured a **93.75% false-positive
rate (15/16)**, with a 6.25% signal-to-noise ratio. Its denominator is what it could see: 1 true
crash + 15 duplicates. Against primary sources that is an **undercount** — the true denominator is
44 wave-1 alerts + 16 wave-2 beads, of which exactly one item (the original crash) was ever
actionable. Its four systemic causes, in its own severity order:

1. **No resolution tracking** (critical) — nothing recorded that bf-4x12ec's crash was resolved;
   each alert was treated as an independent event.
2. **No deduplication** (high) — no fingerprint by (exit code, task, timestamp), no cooldown, no
   cross-reference against resolved crashes.
3. **Exit-code misclassification** (medium) — administrative failures mapped onto signal exits.
4. **No status correlation** (medium) — bead status never checked before alerting.

Report: `docs/archive/crash-investigations/verification-report-domchk-30b53d74-complete.md`
(relocated under `docs/archive/` by a883044).

## The unifying pattern

Both waves are the same missing feature — **incident identity** — seen at two time scales:

- **Within an incident (wave 1):** one kill = one alert bead. 44 kills of one retry cycle = 44
  beads. No fingerprint, no aggregation, no cooldown; the alert layer was a passive per-event
  recorder.
- **After resolution (wave 2):** a closed target = an unknown. Nothing correlated the alert against
  bead status, so a resolved crash regenerated as fresh investigation work 9 days later.

The waves also feed each other. Wave 1 minted 44 alert beads and closed 23 of them; the other 21
sat open/in_progress/deferred in the store as standing, unresolved history — exactly the material a
regeneration pass sweeps up. Child 5 (`domchk-e9ed52aa`) owns closing that residue, which removes
the standing invitation.

Mitigations (closed-bead filtering, duplicate detection, completion awareness, 5-minute cooldown,
crash classification) landed 2026-09-02 in `scripts/crash-alert-manager.sh` and are **child 4's
scope** (`domchk-a3690174`) — not re-verified here.

## Repository state — the cleanup held

The storm's precondition was the bloat; the bloat has not returned.

| Reading | `.git` | Loose objects | Packed | Garbage |
|---|---|---|---|---|
| 2026-08-12 (the bloat that caused the storm) | ~18 GB | ~17.2 GB | — | — |
| First cleanup, closed 2026-08-17, measured 2026-08-26 (incident report) | 753 MB | 141 | 750.67 MiB | — |
| Same cleanup as summarized by the `domchk-30b53d74` analysis | 139 MB | — | — | — |
| 2026-09-02 (per this split's own verification; the incident report's committed record reads 54) | **92 MB** | **53** (10,408 in-pack) | 1 pack, 90.18 MiB | 0 |
| 2026-09-08 03:29Z (this document, live) | 106 MB | 289 (1.95 MiB) | 1 pack, 100.25 MiB | 0 |

(The two 08-26 readings of the same cleanup differ by source — 753 MB in the incident report's
table, 139 MB in the umbrella analysis; both records agree the 18 GB state was gone. This document
takes no side: the relevant fact for the alert pattern is only that the bloat precondition was
removed and stayed removed.)

18 GB → 92 MB, holding at 106 MB — the size is ~165× below the bloat state and far under the 500 MB
warning threshold. The loose-object *count* drifts between the scheduled gc runs (20/35/43 through
2026-09-02, 289 at this reading, 1.95 MiB of churn accumulated since the 2026-09-08 depth-250
repack with the nightly 03:00 local gc still ahead of it); the *size* metric is the bloat signal,
and it is nowhere near trouble. `git count-objects -vH` at this reading: `count: 289`,
`size: 1.95 MiB`, `in-pack: 12174`, `packs: 1`, `size-pack: 100.25 MiB`, `garbage: 0`.

## Sources

- Canonical: `docs/crash-investigations/bf-4x12ec-crash-investigation.md` — Addendum 2
  (53-attempt table), Addendum 3 (kernel evidence, `MemoryMax=12GiB`, load telemetry, isolation
  sweep), Addendum 4 (session transcripts: every phase-1 death ends mid-`git gc`).
- Inventory + per-bead FP verdicts: `docs/crash-investigations/bf-4x12ec-alert-inventory.md`
  (children 1 and 2 of this split).
- Storm evidence (this repo, untracked-under-`.gitignore` JSONL except the two `.txt` extracts):
  `docs/crash-investigations/evidence/bf-4x12ec/crash-logs/` — `exit-code-timeline.txt`,
  `alert-beads-exit-timestamps.txt`, and the raw worker-log JSONL they derive from.
- Sibling umbrella report: `docs/archive/crash-investigations/verification-report-domchk-30b53d74-complete.md`.
- Incident report (a different split on the same target; source of the 2026-08-17 mid-cleanup
  reading): `docs/crash-reports/bf-4x12ec-git-gc-crash.md` — Repository State section.
- Bead store: `bf-4x12ec` closed rev 4, 2026-08-17T14:50:41Z (re-confirmed live by child 2).

---
**Author:** bead domchk-1e38a7c2 (child 3 of the domchk-bcdd4b4f split), 2026-09-08
