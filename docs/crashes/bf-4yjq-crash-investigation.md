# Consolidated Crash Investigation: bf-4yjq

**Subject bead:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo mirror has diverged/gone stale" (P2, closed 2026-08-17)
**Consolidation bead:** domchk-8b3c1c69 (2026-09-08)
**Crash date:** 2026-08-12 — window 17:54:00.249 → 20:30:43.716 UTC
**Classification:** **INFRASTRUCTURE — repository bloat** (signal deaths during git operations on an ~18 GB repository). Not a code defect — no investigation of this workspace has ever found a domain-check code defect.
**Status:** ✅ RESOLVED — subject task completed 2026-08-17; crash class not reproducible today (re-verified first-hand 2026-09-08)

**This is the canonical consolidated crash investigation report for bf-4yjq** and the entry
point for the record. It consolidates the findings of every split-child investigation into
the parent deliverable set. The **detailed investigation record** — the full 11-section
treatment with the complete evidence chain, the formal §11 classification record, and the
per-correction banners — is
[`docs/crash-investigations/bf-4yjq-crash-investigation.md`](../crash-investigations/bf-4yjq-crash-investigation.md)
(domchk-4eab7c59, 2026-09-02). Where this report and the detailed record differ at
figure level, the detailed record's dated corrections govern; they are folded in below.

## Parent deliverables

| Deliverable | Satisfied by |
|-------------|--------------|
| Crash summary (task being performed) | §1 |
| Available logs / error indicators | §2 |
| Reproducibility assessment | §3 |

## Split-child findings consolidated

| Child bead | Finding | Record |
|-----------|---------|--------|
| domchk-d5dd1b33 (2026-09-02) | Crash circumstances: re-derived the crash from the forensic checkpoint — 50 alert beads, not 9; placed bf-4yjq inside the 455-event storm table | [`docs/archive/crash-investigations/crash-circumstances-bf-4yjq-domchk-d5dd1b33-2026-09-02.md`](../archive/crash-investigations/crash-circumstances-bf-4yjq-domchk-d5dd1b33-2026-09-02.md) |
| domchk-4eab7c59 (2026-09-02) | Original consolidated record (commit `db3f1f2`): 50 verified crashes, storm context, root cause, reproducibility assessment; superseded the 9-crash figures and banner-linked the two older competing comprehensive reports | [detailed record](../crash-investigations/bf-4yjq-crash-investigation.md) |
| domchk-dd389d34 (2026-09-06) | Agent-log extraction from storage | [`bf-4yjq-log-extraction-domchk-dd389d34-2026-09-06.md`](../crash-investigations/bf-4yjq-log-extraction-domchk-dd389d34-2026-09-06.md) |
| domchk-495041ac (2026-09-06) | Recovered the raw evidence: 56 per-run session transcripts, worker log, needle events, with `sessions-index.tsv` and `MANIFEST.sha256` | [`docs/crash/bf-4yjq/raw-logs/`](../crash/bf-4yjq/raw-logs/README.md) |
| domchk-48e02d6f (2026-09-06) | Crash classification per the response-guide taxonomy: INFRASTRUCTURE | [`bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md`](../crash-investigations/bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md) |
| domchk-54bc57df (2026-09-06) | Root-cause determination rendered as a standalone causal chain with per-step confidence | [`bf-4yjq-root-cause-determination-domchk-54bc57df-2026-09-06.md`](../crash-investigations/bf-4yjq-root-cause-determination-domchk-54bc57df-2026-09-06.md) |
| domchk-b9513e0b (2026-09-06) | Pattern analysis: exit-code −1 semantics, workspace-wide pattern from raw worker logs, cross-date incident-family comparison | [`bf-4yjq-crash-pattern-analysis-domchk-b9513e0b-2026-09-06.md`](../crash-investigations/bf-4yjq-crash-pattern-analysis-domchk-b9513e0b-2026-09-06.md) |
| domchk-ea5c6a63 / domchk-4ed0544b (2026-09-06) | Artifact consolidation and findings write-up (two-layer fix, lessons learned, source index) | [`bf-4yjq-consolidated-summary-domchk-ea5c6a63-2026-09-06.md`](../crash-investigations/bf-4yjq-consolidated-summary-domchk-ea5c6a63-2026-09-06.md), [`bf-4yjq-consolidated-findings-domchk-4ed0544b-2026-09-06.md`](bf-4yjq-consolidated-findings-domchk-4ed0544b-2026-09-06.md) |
| domchk-7a34eb37 (2026-09-07) | Evidence reconciliation across all raw-log sources: mean interval **191.9 s** (not 188 s); **all 50 crash runs' last tool call is `git push`**; "no raw session evidence survives" superseded for dispatch transcripts | [`bf-4yjq-evidence-reconciliation-domchk-7a34eb37-2026-09-07.md`](../crash-investigations/bf-4yjq-evidence-reconciliation-domchk-7a34eb37-2026-09-07.md) |
| domchk-c73c65bb (2026-09-08) | Reproducibility verdict re-tested first-hand five days on — stands unchanged; all three crash-response-guide false-positive tests applied | §7 re-verification of the detailed record (commit `9a28b1d`) |
| domchk-8055c332 (2026-09-08) | Formal classification record: guide-mapping, indicator inventory split available/missing, automated-classifier coverage gap, confidence basis | §11 of the detailed record |

---

## 1. Crash summary — the task being performed

bf-4yjq was a **git remote reconciliation task**: fetch both remotes and diff the divergent
tips, create a merge commit reconciling Forgejo/GitHub divergence (**no force-push**, per
workspace rule), repoint `origin` at `git.ardenone.com/jedarden/domain-check`, configure the
Forgejo server-side push mirror to GitHub, verify convergence.

**What happened:** on 2026-08-12 the agent dispatched against it was killed **50 times**,
every one with **exit code −1**, between **17:54:00.249 and 20:30:43.716 UTC** — one death
every **191.9 s** on average (~3.2 min) for 2h37m, then the deaths stopped. Each retry
dispatch died on a substantive git operation against a repository that could not survive one:
`.git` ≈ **18 GB**, with **17.2 GiB of loose objects across ~4,594 objects vs 9.6 MiB
packed** (an ≈1,800:1 inverted ratio), built up when the bf-2ildm-era workflow repeatedly
committed ~237 MB `.beads/` JSONL snapshots before `.beads/` was gitignored.

**Every crash was incidental to the task's content** — the agent was not killed by anything
about git remotes; its crash exposure was a function of *when the bead was scheduled*
relative to the bloat window. The crashes were **mid-task, not post-completion**: all 50
deaths fall in a window ending ≈4 days *before* the bead closed (2026-08-17 00:14 UTC, "Git
remote configuration successfully fixed and verified"), and the session transcripts show
50 `git push` attempts with **0 delivered** — Forgejo `origin/main` stood at `63ba024`
through the whole window. The task then completed normally once the trigger was removed.

**bf-4yjq was the middle shift of a workspace-wide crash storm** (full-file scan of
`.beads/checkpoint/forensic.jsonl`, deduplicated by alert-bead ID):

| Target bead | Distinct crash events | Window (UTC, 2026-08-12) |
|-------------|----------------------|--------------------------|
| bf-31mno | 350 | 05:36:21 – 16:31:52 |
| **bf-4yjq** | **50** | **17:54:00 – 20:30:43** |
| bf-1s6c3 | 49 | 21:36:51 – 23:57:21 |
| bf-2xygo | 4 | 21:18:27 – 21:28:29 |
| bf-23n / bf-5d18 | 1 + 1 | 17:08 / 17:23 |
| **Total** | **455** | 05:36 – 23:57 (~18.5 h), 100% exit −1 |

**Root cause** (see §4): resource exhaustion — memcg-OOM-class signal deaths inside the
per-dispatch systemd scope (`MemoryMax` = 12 GiB) during git operations on the bloated
object store. **Confidence: HIGH on the bloat correlation, MEDIUM-HIGH on the OOM mechanism
itself** — kernel logs for Aug-12 were lost to the Aug-14 reboot, so the mechanism is
attributed by class membership (directly kernel-verified for the later members of the same
death class: bf-4x12ec, gc-side, 2026-08-14; bf-198ne, push-side, 2026-08-16).

**Resolution:** the repository was packed/cleaned Aug 13–14 (18 GB → ~91–138 MB); the
deaths stopped at exactly that boundary and never returned. bf-4yjq's own work then
completed and was closed 2026-08-17. Live today: `origin` → Forgejo ✅.

## 2. Available logs and error indicators

**Surviving evidence (all re-verified 2026-09-07/09-08 by the reconciliation and
classification children):**

| Indicator | Value | Source |
|-----------|-------|--------|
| Exit code | `−1` on **50/50** crash alert beads, zero variation (needle's sentinel for an unrecorded signal death — *not* a signal number; SIGKILL's 137 never appears, so no specific signal may be asserted) | `.beads/checkpoint/forensic.jsonl` — 50-count re-derived first-hand 2026-09-08 (domchk-8b3c1c69) |
| Alert labels | `signal--1` on all 50; `failure-count` escalators 1→12; `umbrella`/`verification-failed` on 33 | forensic.jsonl |
| Crash window / cadence | 17:54:00.249 → 20:30:43.716 UTC; **191.9 s** mean interval (2026-09-07 correction — the earlier 188 s divided the span by 50 events rather than 49 gaps) | forensic.jsonl + reconciliation doc |
| Last fatal command | **`git push origin main` in 50/50 crash-run transcripts** | recovered session transcripts |
| Session transcripts | **56 of the run's sessions survive** (50 crash runs + 6 non-crash), each with dispatch/kill timestamps, durations, and per-line SHA-256s in `sessions-index.tsv` + `MANIFEST.sha256` | [`docs/crash/bf-4yjq/raw-logs/`](../crash/bf-4yjq/raw-logs/README.md) (recovered 2026-09-06, domchk-495041ac) |
| Worker log / needle events | `needle-worker-log-bf-4yjq-slot2.log`, `needle-events-2026-08-12-bf-4yjq.jsonl` | same bundle |
| Repo state at crash | ~18 GB `.git`, 17.2 GiB loose / 9.6 MiB packed | contemporaneous alert-bead notes (2026-08-16) and the bf-2ildm-era measurements |
| Temporal boundary | Deaths stop exactly at the Aug-13 cleanup; zero recurrences since | forensic.jsonl + live repo state |
| Storm context | 455 exit −1 events across 6 beads the same day | forensic.jsonl |

**Missing evidence (named, with why):** kernel memcg-OOM records for Aug-12 (the only
*direct* mechanism proof — lost to the Aug-14 reboot; the journal's first entry is
2026-08-15 19:56:33 EDT), exit-code-137 records (never captured), core dumps and stack
traces (none; the latter cannot exist for a SIGKILL, so their absence is not evidence of an
application fault), monitor logs reaching Aug-12 (`.beads/logs/` floor is 2026-09-02),
needle event records (`.beads/events.jsonl` starts 2026-08-16), automated classifier input
(`crash-classifier.sh` exits "Bead trace not found" — the trace slot is single and empty for
this era; its silence is an input-coverage gap, **not** evidence that no crash occurred),
and the crash-era commits themselves (removed from the DAG by the Aug-16 squash commit).
The 09-01-era "load 15–17 / disk 84%" telemetry traces to a **Sep-1** system-state snapshot,
not an Aug-12 instrument reading.

**Classification:** INFRASTRUCTURE — repository bloat; not CODE_DEFECT (every fatal command
is git plumbing over the bloated store, zero code implicated) and not FALSE_POSITIVE (all
deaths mid-task, ≈4 days before completion). Full guide-mapping in §11 of the detailed
record.

## 3. Reproducibility assessment

**Verdict: transient environmental failure — deterministic while the trigger condition
existed, not reproducible in the current environment.**

| Aspect | Assessment |
|--------|------------|
| Reproducible at the time? | **Yes, deterministically.** 50/50 deaths, zero exit-code variation, every re-dispatch dead within ~1–3 minutes. While the bloated object store existed, the failure fired on essentially any substantive git operation — the observed trigger was `git push`, the gc-side sibling (bf-4x12ec) died the same way on `git gc --aggressive`. |
| Reproducible today? | **No.** The trigger was removed (repo packed Aug 13–14). Re-verified first-hand 2026-09-08 by domchk-c73c65bb (§7 re-verification of the detailed record) and again live for this consolidation: `.git` 106 MB, 310 loose objects (2.10 MiB), 1 pack (100.25 MiB), 0 garbage; `check-repo-health.sh` exit 0; effective pack-memory bound ≈3072 MiB worst case, inside the ceiling. Healthy repo + bounded git config + healthy host → the failure mode has nothing to act on. |
| Self-sustaining? | **No.** Environmental — the loop ended when the condition ended, with no intervention directed at the crashing beads themselves. |
| Not a false positive? | **Confirmed.** All three crash-response-guide tests applied 2026-09-08: completion timing (deaths ≈4 days *before* close — not FP); retry-then-success (no successes inside the window — doesn't apply); storm threshold (crossed workspace-wide, 11 deaths per rolling 10 min across targets on 455 alerts — INFRASTRUCTURE EVENT). |
| Recurrence risk | **Conditional on the trigger class, not this event.** Repository bloat is the known recurrence vector, now guarded: `.beads/` fully gitignored, 10 MB pre-commit hook, repo-health thresholds (alert >1 GB total / >500 MB loose), `pack.windowMemory`/`pack.threads` bounds on bare gc *and* push, daily incremental + weekly full safe-git-gc systemd timers. Any relaxation reopens the bf-4yjq/bf-1s6c3 failure mode. |
| Residual reproduction value | Low for this bead (closed, verified complete). Retained value is as the **type specimen for the Aug-12 storm class**: a fixed-cadence run of exit-−1 re-dispatch deaths across multiple beads is the signature of an environmental kill regime — triage the environment (repo size, memory, load) before any per-bead debugging. |

The useful framing: **this crash is not an event you can re-run — it is a condition you can
re-create.** It would recur only if the condition (repository bloat) recurs, which is what
the deployed prevention stack now guards against.

## 4. Root cause (condensed)

1. Pre-Aug-12: bf-2ildm-era workflow repeatedly commits ~237 MB `.beads/` JSONL files →
   `.git` reaches ~18 GB (17.2 GiB loose, ≈1,800:1 loose:packed).
2. 2026-08-12 05:36 UTC onward: substantive git operations on that store pull multi-GB
   working sets inside the 12 GiB dispatch scopes and begin killing agents.
3. 17:54–20:30 UTC: bf-4yjq's retry loop loses 50 consecutive agents, ~1 per 3.2 minutes —
   every death on `git push`, nothing delivered.
4. Aug 13–14: the repository is packed (18 GB → ~91–138 MB). The deaths stop at exactly
   that boundary; divergence-analysis artifacts dated Aug 13 show bf-4yjq work proceeding
   normally.
5. 2026-08-17: bf-4yjq closed as completed; remotes verified correct (and still correct
   today — `origin` → Forgejo, verified live 2026-09-08).

Full causal chain with per-step confidence: §6 of the detailed record and
[`bf-4yjq-root-cause-determination-domchk-54bc57df-2026-09-06.md`](../crash-investigations/bf-4yjq-root-cause-determination-domchk-54bc57df-2026-09-06.md).

## 5. Superseded reports and source index

| Document | Status relative to this report |
|----------|-------------------------------|
| [`docs/crashes/bf-4yjq-crash-report.md`](bf-4yjq-crash-report.md) (2026-09-01) | **Superseded — banner-linked here.** Recorded 9 crashes at ~17-minute intervals and a "1.7 GB" post-cleanup figure; its signal analysis and prevention-stack validation remain valid |
| [`docs/reports/bf-4yjq-comprehensive-crash-report.md`](../reports/bf-4yjq-comprehensive-crash-report.md) (2026-08-14) | **Superseded — banner-linked here.** Same 9-crash record; its root-cause detail, system-state telemetry, and resolution record remain valid. The two older comprehensive reports also disagreed with each other (e.g. `bf-29rca` at 18:18:20 appears in only one table) |
| `docs/archive/crash-investigations/crash-investigation-report-bf-4yjq-comprehensive.md` (2026-08-26) | Same 9-crash-era record; now in the frozen archive (`docs/archive/`, per its README) — superseded in place, not edited |
| `.beads/crash-bf-4yjq-summary.txt` | Contemporaneous metrics; superseded on crash count |
| The various bf-4yjq facet documents (`docs/crash-artifacts-bf-4yjq*.md`, `docs/crash-context*bf-4yjq*.md`, `docs/crash-data-extraction-bf-4yjq.md`, `docs/remediation-strategy-bf-4yjq.md`, the Aug/Sep summaries) | Facet documents from the earlier investigation waves; superseded where they repeat the 9-crash count, otherwise consistent |
| [Detailed investigation record](../crash-investigations/bf-4yjq-crash-investigation.md) | **Companion deep record** — 11 sections; governs at figure level via its dated corrections |

**The count correction, once and for all:** the 9-crash / ~17-minute record sampled only the
alert beads the earlier investigations happened to find. Verified against the forensic
checkpoint: **50 crashes at ~3.2-minute intervals** (191.9 s mean), inside a 455-event
workspace-wide storm. Any downstream figure derived from "9" — cadence, severity ranking —
is retracted. The old reports' "BLOCKED and not actively executing" claim is unverifiable
from surviving evidence and inconsistent with the verified re-dispatch-kill cadence.

**Related investigations:** bf-1s6c3 (repository bloat, same storm),
bf-173o7e (gc execution), bf-2ildm (bloat source), bf-4x12ec / bf-198ne / bf-1ea4g (the
kernel-verified later members of the same death class).
