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

1. **Work committed < 30 s before crash → FALSE_POSITIVE: does not apply.** No commits were
   made at all during the storm window; the subject task's commits came later, after the repo
   was cleaned.
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
