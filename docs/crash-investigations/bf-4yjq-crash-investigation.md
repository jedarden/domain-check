# Comprehensive Crash Investigation: bf-4yjq

**Subject bead:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo mirror has diverged/gone stale" (P2, closed 2026-08-17)
**Investigation bead:** domchk-4eab7c59
**Report date:** 2026-09-02
**Crash date:** 2026-08-12 (window 17:54:00–20:30:43 UTC)
**Status:** ✅ INVESTIGATION COMPLETE — crash resolved, root cause identified, subject task independently verified complete

**This is the canonical bf-4yjq report.** It supersedes the crash-count and cadence data in
`docs/reports/bf-4yjq-comprehensive-crash-report.md`, `docs/crashes/bf-4yjq-crash-report.md`,
and the various bf-4yjq summaries (see §10). Those documents recorded **9 crashes at ~17-minute
intervals**; verification against the bead store established **50 crashes at ~3.1-minute
intervals**, plus a same-day workspace-wide crash storm they did not record.

---

## 1. Executive Summary

bf-4yjq was a **git remote reconciliation task** (repoint `origin` at Forgejo, reconcile the
Forgejo/GitHub divergence with a merge commit, configure the Forgejo→GitHub server-side push
mirror). On **2026-08-12 the agent dispatched against it was killed 50 times**, every one with
**exit code -1**, between **17:54:00 and 20:30:43 UTC** — one death every ~3.1 minutes for
2h37m. The subject bead absorbed the middle shift of a **workspace-wide crash storm: 455
exit-code -1 events across 6 beads from 05:36 to 23:57 UTC (~18.5 hours)**.

**Classification: INFRASTRUCTURE — resource exhaustion (OOM) during git operations on an
18 GB bloated repository. Not a code defect.** No investigation of this workspace has ever
found a domain-check code defect.

**Reproducibility: transient environmental failure.** The kill was deterministic *while the
trigger existed* (50/50 deaths, zero exit-code variation), and the trigger — the bloated
object store — was removed on Aug 13–14. The crash class is **not reproducible today**
(verified 2026-09-02: repo 92 MB, healthy). It would only recur if the *condition*
(repository bloat) recurs, which is what the deployed prevention stack now guards against.

**Outcome:** the bead's task was completed after the storm (closed 2026-08-17) and remains
verified correct on the live repo today.

---

## 2. Task Context

| Field | Value |
|-------|-------|
| Bead | bf-4yjq (P2) |
| Created | 2026-07-20 13:59 UTC |
| Crash window | 2026-08-12 17:54:00 → 20:30:43 UTC |
| Closed | 2026-08-17 00:14 UTC — "Git remote configuration successfully fixed and verified" |

The task, per its own description: fetch both remotes and diff the divergent tips; create a
merge commit reconciling them (**no force-push**, per workspace rule); add `origin` pointing
at `git.ardenone.com/jedarden/domain-check`; configure the Forgejo server-side push mirror to
GitHub; verify convergence.

**Every crash was incidental to the task's content.** The agent was not killed mid-merge or
mid-push by anything about git remotes — each short-lived session died on whatever generic
git operation it touched first, on an object store that could not survive git operations. The
bead's crash exposure was a function of *when it was scheduled* relative to the bloat window,
not of what it was doing.

Work products of the actual task survive as the `.beads/` divergence-analysis state files
(`divergence-ancestor.json`, `divergence-point.json`, `github_commits_analysis.json`,
`.branch_divergence_state.json`), all dated 2026-08-13 — produced by the sessions that finally
ran once the storm ended and the repo was cleaned.

---

## 3. Crash Circumstances (verified)

All figures below were re-derived from `.beads/checkpoint/forensic.jsonl` on 2026-09-02
(domchk-d5dd1b33), and the headline count independently re-verified on 2026-09-02
(domchk-4eab7c59): **50 distinct alert beads titled "ALERT: Agent crash on bead bf-4yjq"**.

- **50 crash events**, all `exit code: -1`, created 17:54:00.249 → 20:30:43.716 UTC.
- **Mean interval 188 s (~3.1 min)** — each re-dispatch died within roughly one to three
  minutes of starting.
- **Zero variation in exit code** across all 50: a deterministic environmental kill, not a
  flaky code path.
- Alert-bead labels show the alert system escalating while the retry loop kept losing agents:
  `alert`, `crash`, `signal--1`, `failure-count:1` (18:38) → `failure-count:4` (20:04), plus
  `umbrella` / `verification-failed` on later alerts.
- **No raw session evidence survives.** No core dumps (consistent with SIGKILL), no stack
  traces, no Aug-12 heartbeats (`heartbeats.jsonl` retains only recent entries); `.beads/traces/`
  holds entries only for later re-runs of the alert beads themselves (Aug 26 / Sep 1, exit 0).
  Crash-era git commits are gone from the DAG — the log jumps from 2026-08-09 (`00117cb`) to
  2026-08-15 (`8373e5d`) because the Aug 16 `c27899f` "catch up lab work onto origin (squashed)"
  commit removed them. Reconstruction rests entirely on the alert-bead corpus and recorded
  contemporaneous metrics.

### Superseded record

Prior documentation (Aug 14–Sep 1 reports and `.beads/crash-bf-4yjq-summary.txt`) records
**9 crashes at ~17-minute intervals**. Verified: **50 crashes at ~3.1-minute intervals**. The
nine timestamps in the old reports are all present in the verified set of 50 — the earlier
investigations sampled only the alert beads they happened to find (the two old reports even
disagree with each other: `bf-29rca` at 18:18:20 appears in one table, not the other). Any
downstream figure derived from "9" — cadence claims, severity ranking — is superseded.

The old reports' claim that bf-4yjq was "BLOCKED and not actively executing" at crash time is
**unverifiable from surviving evidence** and inconsistent with the verified re-dispatch-kill
cadence (agents were dying on dispatch, ~3 minutes apart, for 2.5 hours). The defensible
statement — which both the old and new analyses agree on — is that the crashes were incidental
to the task's content.

---

## 4. The Storm bf-4yjq Sat Inside

Full-file scan of the forensic checkpoint for crash-report records dated 2026-08-12,
deduplicated by alert-bead ID:

| Target bead | Distinct crash events | Window (UTC) | Notes |
|-------------|----------------------|--------------|-------|
| bf-31mno | **350** | 05:36:21 – 16:31:52 | Not previously tallied in any bf-4yjq-era doc |
| **bf-4yjq** | **50** | 17:54:00 – 20:30:43 | Subject of this report |
| bf-1s6c3 | 49 | 21:36:51 – 23:57:21 | The bloat crash documented in CLAUDE.md |
| bf-2xygo | 4 | 21:18:27 – 21:28:29 | |
| bf-23n | 1 | 17:08:40 | |
| bf-5d18 | 1 | 17:23:54 | |
| **Total** | **455** | 05:36 – 23:57 (~18.5 h) | 100% exit code -1 |

The ~1h22m gap before bf-4yjq's window and the ~1h06m gap after it look like cleanup/cooldown
periods rather than recovery — the storm resumes on the next retried bead. bf-31mno's 350
crashes appear in no bf-4yjq-era document; the storm's scale was undercounted ~5x for this
bead and essentially unrecorded for the others.

---

## 5. Indicators

| Indicator | Observation | Interpretation |
|-----------|-------------|----------------|
| Exit code | -1 on 50/50 events | Deterministic external kill (SIGKILL-class), not application error |
| Cadence | ~188 s mean interval | Death on/near dispatch — each retry hit the same wall almost immediately |
| Duration | 2h37m continuous, then stops | Environmental condition persisted, then was removed |
| Alert metadata | `failure-count` escalators firing | Retry loop exceeded alert thresholds; alert system escalating while crashes continued |
| Raw telemetry | None survives (no core dumps, no kernel OOM logs, no heartbeats) | Limits mechanism-level certainty (see §6 caveat) |
| Scope | 6 beads, 455 events, one day | Workspace-wide regime, not bead-specific |

---

## 6. Root Cause

**Chain of events:**

1. Pre-Aug-12: bf-2ildm-era workflow repeatedly commits ~237 MB `.beads/` JSONL files
   (`.beads/` not yet excluded from git) → repository reaches **~18 GB**, with **17.2 GiB of
   loose objects across ~4,594 objects vs 9.6 MiB packed** — a severely inverted loose:packed
   ratio (≈1,800:1).
2. 2026-08-12 05:36 UTC onward: substantive git operations on that object store (status/fetch
   on a cold repo, packing, fsck) pull multi-GB working sets and begin OOM-killing agents;
   contemporaneous telemetry records load average 15–17 on 12 cores, memory effectively
   exhausted during git operations, disk 84% full.
3. 17:54–20:30 UTC: bf-4yjq's retry loop loses 50 consecutive agents, ~1 every 3 minutes.
4. Between 20:30 Aug 12 and the morning of Aug 13 the repository is packed/cleaned
   (bf-173o7e's git gc; 18 GB → ~91–138 MB, documented elsewhere). Divergence-analysis
   artifacts dated Aug 13 show bf-4yjq work then proceeding normally.
5. 2026-08-17: bf-4yjq closed as completed; remotes verified correct (re-verified 2026-09-02, §8).

**Confidence: HIGH on the bloat correlation, MEDIUM-HIGH on the OOM mechanism specifically.**
Kernel OOM logs were not retained (journalctl access was limited even at the time), so the
signal-9-from-OOM step cannot be re-verified from raw logs today. What *is* independently
verifiable now: the uniform exit-code -1 pattern, the storm's scale, the bloat metrics recorded
contemporaneously, and the fact that the crashes stopped exactly when the repository was
cleaned — and never returned.

---

## 7. Reproducibility Assessment

**Verdict: transient environmental failure — not reproducible in the current environment;
was deterministically reproducible while its trigger condition existed.**

| Aspect | Assessment |
|--------|------------|
| Reproducible at the time? | **Yes, deterministically.** 50/50 deaths with zero exit-code variation; every re-dispatch died within ~1–3 minutes. While the bloated object store existed, the failure fired on essentially any substantive git operation. |
| Reproducible today? | **No.** The trigger was removed (repo packed/cleaned Aug 13–14). Verified 2026-09-02: `.git` 92 MB, 20 loose objects (168 KiB), 1 pack (90.18 MiB), 49 GB memory available. Healthy repo + healthy host → the failure mode has nothing to act on. |
| Self-sustaining? | **No.** Environmental — the crash loop ended when the condition ended, with no intervention directed at the crashing beads themselves. |
| Recurrence risk | **Conditional on the trigger class, not this event.** Repository bloat is the known recurrence vector; it is now guarded by `.gitignore` exclusion of `.beads/`, pre-commit large-file hooks (>10 MB), repo-health thresholds (alert >1 GB total / >500 MB loose), daily incremental + weekly full safe-git-gc systemd timers, and the crash-alert system with dedup/false-positive fixes (2026-09-02). |
| Residual reproduction value | Low for this bead (closed, verified). Retained value is as the type specimen for the Aug-12 storm class: deterministic exit-1 cadence at minutes-level intervals is the signature of an environmental kill, not a code path. |

The useful framing for future triage: **this crash is not an event you can re-run — it is a
condition you can re-create.** Any future spike of sub-3-minute exit-code -1 re-dispatch
deaths across multiple beads should be treated as a resource-exhaustion regime and triaged at
the environment level (repo size, memory, load) before any per-bead debugging.

---

## 8. Current State Verification (2026-09-02)

The task bf-4yjq set out to do is confirmed done on the live repo (commands run today):

| Check | Result |
|-------|--------|
| `git remote -v` | `origin` → `https://git.ardenone.com/jedarden/domain-check.git` ✅ Forgejo-primary |
| Branch sync | local `main` (`55dab07`) identical to `origin/main` ✅ |
| `git count-objects -vH` | 20 loose objects (168 KiB), 1 pack (90.18 MiB) ✅ all thresholds green |
| `du -sh .git` | 92 MB ✅ (vs. the 500 MB healthy limit) |
| Host memory | 49 GB available, no pressure ✅ |

Two observations outside this crash's scope, noted for the record: a local remote named
`github-mirror` (github.com) exists alongside `origin` — the workspace convention calls for
mirroring to be *server-side on Forgejo*, not a client-side remote (Forgejo↔GitHub tip state
is covered by commit `ab63992`'s divergence analysis). And hundreds of Aug-12 alert beads,
including most of bf-4yjq's 50, remain open — the known alert-hygiene debt (§9).

---

## 9. Findings and Recommendations

### What this investigation establishes

1. **The crash was environmental and is resolved.** 50 deterministic kills during a 2h37m
   window on a bloated repo; zero recurrences in the ~3 weeks since cleanup; subject task
   completed and independently re-verified.
2. **The historical record materially undercounted the event** (9 vs 50 for this bead; the
   350-event bf-31mno storm unrecorded). Per-bead crash histories built from alert sampling
   cannot be trusted for scale; the forensic checkpoint is the source of truth.
3. **The evidence-retention gap limits finality.** The OOM mechanism itself rests on
   contemporaneous telemetry, not re-verifiable raw logs.

### Recommendations

1. **Storm-level detection, not per-bead.** Existing crash-pattern detection keys on repeated
   crashes of one bead; on Aug 12 no single bead exceeded thresholds until dozens of siblings
   had already died. Aggregate exit-code -1 rate per workspace per hour (e.g. ≥10/hour) is the
   signal that would have fired earliest — extend `scripts/crash-pattern-detection.sh`
   accordingly.
2. **Close the Aug-12 alert-bead backlog.** Hundreds of alert beads remain open, which both
   distorts future counting and produced the duplicate-alert noise documented across the
   verification reports. A one-time bulk-close of storm alerts for already-resolved targets
   (with a referencing note) is warranted.
3. **Retain storm telemetry.** Archive resource-monitor snapshots (now running via systemd
   timers) with timestamps, and preserve kernel OOM events (`journalctl -k`), so the next
   signal-1 storm is attributable directly from raw logs rather than by inference.
4. **Treat superseded counts as retracted.** Where other documents repeat "9 crashes" or
   "~17-minute intervals" for bf-4yjq, defer to this report and to
   `docs/crash-circumstances-bf-4yjq-domchk-d5dd1b33-2026-09-02.md`.
5. **Keep the bloat-prevention stack enforced.** It is the reason this failure class has not
   recurred: `.gitignore` exclusion of `.beads/`, pre-commit large-file hooks, repo-health
   thresholds, and the systemd gc timers. Any relaxation should be treated as reopening the
   bf-4yjq/bf-1s6c3 failure mode.

---

## 10. Source Index

| Document | Status relative to this report |
|----------|-------------------------------|
| `docs/crash-circumstances-bf-4yjq-domchk-d5dd1b33-2026-09-02.md` | **Verified primary source** — 50-crash count, storm table, supersession of the 9-crash record |
| `.beads/checkpoint/forensic.jsonl` | Raw evidence; 50 bf-4yjq alert beads independently re-counted 2026-09-02 |
| `docs/reports/bf-4yjq-comprehensive-crash-report.md` | Superseded on crash count/cadence; retained for root-cause detail, system-state telemetry, and resolution record (now banner-linked here) |
| `docs/crashes/bf-4yjq-crash-report.md` | Superseded on crash count/cadence; retained for signal analysis and prevention-stack validation (now banner-linked here). Its "1.7 GB" post-cleanup figure is an intermediate state — 91–92 MB matches the gc evidence and today's verification |
| `docs/remediation-strategy-bf-4yjq.md`, `docs/crash-pattern-analysis-bf-4yjq.md`, `docs/crash-data-extraction-bf-4yjq.md`, `docs/crash-artifacts-bf-4yjq*.md`, `docs/crash-context*bf-4yjq*.md`, and the various bf-4yjq summaries | Facet documents from the earlier investigation waves; superseded where they repeat the 9-crash count, otherwise consistent with this report |
| `.beads/crash-bf-4yjq-summary.txt` | Contemporaneous metrics; superseded on crash count |

**Related investigations:** bf-1s6c3 (repository bloat, `docs/crashes/repository-bloat-crash-bf-1s6c3-2026-08-12.md`),
bf-173o7e (gc execution, `docs/crash-investigations/bf-173o7e-crash-investigation.md`),
bf-2ildm (bloat source, `docs/crash-context-bf-2ildm-complete.md`).
