# Crash Investigation: bf-4yjq — classification record

**Investigation Date:** 2026-09-06
**Bead ID:** bf-4yjq (subject) — dispatch domchk-48e02d6f (this record)
**Agent:** claude-code-glm-4.7
**Exit Code:** -1
**Classification:** INFRASTRUCTURE

**Scope of this record.** bf-4yjq already has a committed canonical investigation
([`bf-4yjq-crash-investigation.md`](bf-4yjq-crash-investigation.md), 2026-09-02) and a
same-day log-level extraction
([`bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md`](bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md)).
No new evidence was gathered here and no finding below is new except where marked. What this
page adds is the step the dispatch asked for: an explicit verdict in
`docs/crash-response-guide.md`'s four-way taxonomy, the guide's false-positive rules applied
against the verified crash corpus, and a fresh (2026-09-06) work-completion and health check.

---

## Executive Summary

bf-4yjq — a git-remote reconciliation task, not a crash task — lost 50 consecutive agents to
exit code -1 between 2026-08-12T17:53:53Z and 20:30:38Z (median 156 s between deaths; each run
did 65–375 s of real work before dying). This was an **INFRASTRUCTURE event**: resource
exhaustion during git operations on the era's ~18 GB bloated repository (17.2 GiB loose objects).
It is not a code defect, not a service failure, and not a false positive.

## Classification

| Category | Verdict | Basis |
|----------|---------|-------|
| **INFRASTRUCTURE** | ✅ **This crash** | Exit code -1 on 50/50 events → guide's Quick Reference maps -1 to Infrastructure. Contemporaneous telemetry: repo 18 GB / 17.16 GiB loose, load 15–17 on 12 cores, memory effectively exhausted during git ops, disk 84%. Deaths stopped when the repo was cleaned (Aug 13–14) and never returned. |
| CODE_DEFECT | ✗ Excluded | The killed process was the dispatch agent, not the application; no domain-check error text exists anywhere in the corpus (extraction §3). Consistent with the standing repo-wide finding of zero domain-check code defects. |
| SERVICE_FAILURE | ✗ Excluded | No HTTP 503/502, no gateway-unavailable signature. The only non-`-1` outcomes in the window were 1 exit-1 agent failure, 4 dispatch timeouts (600.1 s), and 1 orphaned exit-0 — none of them gateway-shaped. |
| FALSE_POSITIVE | ✗ Excluded | See assessment below — all three of the guide's false-positive rules come out negative. |

### Reading the exit code

`-1` is **needle's sentinel for a process death whose signal was not recorded** — not a signal
number, and not proof of SIGKILL by itself (extraction §1). The older
`docs/crash-artifacts-bf-4yjq.md` states "Signal -1 = SIGKILL (Signal 9), delivered by the OOM
killer" as fact; that equivalence is an inference. The kernel OOM step is **unverifiable for
this event** (system journal starts 2026-08-15, earliest coredump 2026-08-17) and is held at
MEDIUM-HIGH confidence by the canonical report (§6) — fleet-wide, this death class is the
memcg OOM kill inside the 12 GiB dispatch scope, verified for other events. Downstream readers
should not restate OOM/SIGKILL as directly proven here.

## System State at Crash Time

From contemporaneous telemetry recorded in the canonical report (§6) and the superseded-record
catalog, for the 2026-08-12 window:

| Metric | Value at crash time | Healthy threshold |
|--------|--------------------|-------------------|
| Repository size | ~18 GB | < 500 MB |
| Loose objects | 17.16 GiB / ~4,594 objects | < 100 MB |
| Packed | 9.6 MiB (ratio ≈ 1,800:1, inverted) | — |
| `.beads/issues.jsonl` | 248 MB (committed to git) | not tracked |
| Load average | 15–17 on 12 cores | < 5 |
| Memory | effectively exhausted during git operations | ≥ 20 GB available |
| Disk | 84% full | > 50 GB free |

Current state (live check, 2026-09-06): repo 93 MB, 93 loose objects (748 KiB) + one 90.43 MiB
pack, 44 GB memory available, load 0.46, disk 80 G free — all green. The trigger condition no
longer exists.

## Work Completion Verification

**At crash time: not complete — the crashes were mid-task, not post-completion.**
No commits exist in the storm window (crash-era commits are absent from the DAG); bf-4yjq's
work products (`.beads/divergence-*.json`, `.beads/github_commits_analysis.json`) are dated
2026-08-13, after the repo was cleaned. Per-run survival of 65–375 s against a 600 s dispatch
timeout confirms each agent was killed mid-task, not at startup and not at timeout.

**Today: complete and verified (re-run live, 2026-09-06).**

| Check | Result |
|-------|--------|
| `git remote -v` | `origin` → `https://git.ardenone.com/jedarden/domain-check.git` ✅ Forgejo-primary |
| Branch sync | local `main` `61ab725` == `origin/main` ✅ |
| `bead show bf-4yjq` | **Closed**, updated 2026-08-17T00:14:14Z ✅ |

(The local `github-mirror` remote noted in the canonical report §8 remains as an
out-of-scope observation; mirroring is meant to be server-side on Forgejo.)

## False Positive Assessment

Against the three rules in `docs/crash-response-guide.md`:

1. **Work committed < 30 s before crash → FALSE_POSITIVE: does not apply.** Zero commits exist
   in the live DAG during the storm window — but crash-era commits **do** survive on the
   `pre-squash-history-20260816` backup ref (17 of them, all bead-state chores), so this rule
   must be tested against **all refs**, not just `main`. Re-checked 2026-09-06
   (domchk-0af2dd94): **2 of the 50 deaths have a commit 25–26 s before it**, inside this
   rule's 30-second gate. The verdict is unchanged — those commits are mid-task
   `chore: update bead tracking state before git reconciliation` bookkeeping (the agent
   committed bead state, then died in the reconciliation git operation that followed), not the
   task's deliverable, which landed only after the repo was cleaned five days later with the
   bead open throughout. The other 48 deaths have no commit within the gate at all.
2. **Crash → retry → success → SELF-HEALED TRANSIENT: does not apply.** Retry did not
   succeed within the storm — 50 consecutive deaths. Work completed only after the
   environmental condition (repo bloat) was removed, 5 days later. The loop was stopped by an
   environment change, not by retry luck; that is a condition-terminated failure, not a
   self-healed one.
3. **≥ 10 crashes in 10 min → INFRASTRUCTURE EVENT: not triggered — and that is the finding.**
   Computed over the 50 verified death timestamps, the **peak is 5 deaths in any 10-minute
   window** (18:18:13Z onward; mean gap 192 s, median 156 s), and no other bead was crashing
   during 17:54–20:30Z. So the guide's own surge detector would **not** have fired for this
   window despite a 50-kill, 2.6-hour storm — it keys on an aggregate rate this regime sat
   under. This substantiates the canonical report's recommendation #1 (per-hour aggregate
   detection, e.g. ≥ 10/hour — this window averaged ~19/hour) and explains why the alert
   system escalated via per-bead `failure-count` labels instead.

**Verdict: genuine INFRASTRUCTURE event; not a false positive.**

## Action Required

- ✅ **NO CODE CHANGES NEEDED.** Subject bead closed and its task independently verified.
- Prevention stack (repo-health thresholds, `.gitignore` exclusion of `.beads/`, pre-commit
  large-file hook, safe-git-gc timers) stays enforced — it is why this class has not recurred.
- Alert-hygiene debt (hundreds of Aug-12 alert beads still open) is tracked in the canonical
  report §9; this dispatch does not own it.

## Classification

**INFRASTRUCTURE ISSUE** — resource exhaustion (OOM-class) during git operations on an
18 GB bloated repository, 2026-08-12; resolved by repository cleanup, not reproducible today.

## Sources

| Document | Role |
|----------|------|
| [`bf-4yjq-crash-investigation.md`](bf-4yjq-crash-investigation.md) | Canonical analysis — root cause, storm table, reproducibility |
| [`bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md`](bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md) | Per-run deaths/alerts/survival, exit-code semantics, absence evidence |
| [`bf-4yjq-artifact-catalog-2026-09-06.md`](bf-4yjq-artifact-catalog-2026-09-06.md) | Live-verified artifact inventory |
| `docs/crash-response-guide.md` | Taxonomy, false-positive rules, Phase 2A checklist |
| `docs/crash-artifacts-bf-4yjq.md` | Contemporaneous system-state telemetry (counts superseded; mechanism statement corrected above) |

---

## Addendum: independent re-verification (2026-09-06, domchk-0af2dd94)

A later dispatch on the same alert pool re-derived this record's load-bearing figures rather
than re-stating them. Every number held:

| Claim in this record | Re-derivation | Result |
|----------------------|---------------|--------|
| 50 deaths, 17:53:53.875682Z → 20:30:38.310348Z | recount of `docs/crash-analysis/bf-4yjq-needle-worker-log-extract.log` | ✅ 50/50 `Crash(-1)`, endpoints byte-identical |
| mean gap 192 s / median 156 s | same recount | ✅ 191.9 s / 156 s |
| Rule 3: peak 5 deaths in any 10-min window | sliding-window scan over the 50 timestamps | ✅ 5 (18:18:13Z → 18:26:56Z); ~19.1/hour aggregate |
| Rule 3: "no other bead was crashing during 17:54–20:30Z" | all `exit_code=-1` records in the domain-check worker log slots, date-constrained | ✅ window contains only bf-4yjq. (bf-mje3pd ×7, bf-29h1yy ×2, bf-2o7nlw ×1 died **Aug-13**, the decay phase — an unconstrained query wrongly sweeps them in) |
| Rule 1: no commits in the storm window | `git log` on all refs, date-constrained | ⚠️ **Corrected above** — true of the live DAG; 17 crash-era commits survive on `pre-squash-history-20260816`, 2 within 30 s of a death |
| Current state green | live `du -sh .git`, `git count-objects -vH`, `git fsck --full` | ✅ `.git` 97 MB, 52 loose (3.78 MiB) + 2 packs (90.93 MiB), 0 garbage, fsck clean; 47 G memory available, load 2.46 |

Two transferable notes for the next reader:

- **Time-of-day traps both directions.** An unconstrained `git log --since/--until` query
  sweeps in other days' commits/deaths sharing the window's clock range (here it manufactured
  10 extra "storm commits" out of Aug-13's decay-phase deaths); a naive-datetime parse of UTC
  log stamps on this EDT box shifts displayed windows by +4 h. Constrain the date and parse
  with an explicit timezone, or both bugs fire at once.
- **The surge-rule gap is confirmed from the workspace side, not just this bead's.** Rule 3's
  aggregate counter would not have fired even counting every bead in the workspace, because
  the concurrent-die-off that would have tripped it came a day later. Only an hourly
  per-bead/per-alert view (canonical report §9 rec. 1) catches a single-bead 2.6-hour
  deterministic retry-kill loop.
