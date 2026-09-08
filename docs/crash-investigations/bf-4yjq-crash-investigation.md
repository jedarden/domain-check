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
found a domain-check code defect. The formal classification record — the
[crash response guide](../crash-response-guide.md) framework mapping, the indicator
inventory split available/missing, and the confidence basis — is **§11** (added 2026-09-08,
domchk-8055c332).

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

> **Dated correction (2026-09-07, domchk-7a34eb37):** [bf-4yjq-evidence-reconciliation-domchk-7a34eb37-2026-09-07.md](bf-4yjq-evidence-reconciliation-domchk-7a34eb37-2026-09-07.md)
> re-verified this section against all raw-log sources. Count (50) and window confirmed from
> three independent sources; the mean interval is **191.9 s (~3.2 min)**, not 188 s (that
> figure divided the span by 50 events rather than 49 gaps); and the "no raw session
> evidence survives" statement below is **superseded for dispatch transcripts** — 56 per-run
> session transcripts survive in `docs/crash/bf-4yjq/raw-logs/` (recovered 2026-09-06,
> domchk-495041ac). The statement remains true for core dumps, kernel logs, and trace
> capture. The transcripts show all 50 crash runs' last tool call is `git push` — the storm
> was uniformly push-side (see the reconciliation doc §3). The same corrections apply to the
> §5 indicator table ("~188 s" → 191.9 s; "raw telemetry: none survives" → superseded for
> session transcripts), and to §6 step 2's "contemporaneous telemetry": the load-15–17 /
> disk-84 % readings trace to `docs/crash-analysis/bf-4yjq-system-state-snapshot-2026-09-01.txt`,
> a **Sep-1 capture** (byte-identical to the Sep-1 `.beads/crash-bf-4yjq-summary.txt`), not an
> Aug-12 instrument reading — no monitor log under `.beads/logs/` reaches back to Aug-12.

> **Dated addendum (2026-09-08, domchk-ce45c03b) — crash-time task state, workspace
> then-vs-now, and an independent completion re-check.** All three re-derived first-hand this
> pass: attempts 01 and 51 extracted from the raw-logs tarball and read directly, all 56
> transcripts scanned for remote/tip state, live commands run 2026-09-08 01:18–01:27 UTC.
>
> **What the agent was doing at the first and final crash.** Both endpoint runs — attempt 1
> (dispatched 17:50:23.059Z, last transcript event 17:53:38.489Z, classified crash −1 at
> 17:53:53.875Z, alert bead `bf-276uk`) and attempt 51 (dispatched 20:26:09.897Z, classified
> −1 at 20:30:38.310Z, alert bead `bf-2n3ve`) — end in the identical literal command
> **`git push origin main`** (read from the transcripts, not inferred from the 50/50 census).
> Attempt 1's transcript captures the crash-time remote state: **`origin` already pointed at
> `git.ardenone.com/jedarden/domain-check.git`**, alongside a second remote named `github` on
> github.com — so the task's step 3 (repoint `origin` at Forgejo) was already in place inside
> the window, and the leg in flight was step 2's reconciliation push: local `main` sat at
> `199b70c`, `[origin/main: ahead 305]` against Forgejo's `63ba024`. Across the whole window
> Forgejo's tip never moved — every transcript that captured `Forgejo origin/main:` shows
> `63ba024` — so **50 pushes were attempted and 0 delivered**. Local `main` did advance
> (12 distinct tips, ahead-count 305 → 324 across the transcripts; authorship per commit is
> not attributable from transcripts on a shared worktree). Each push therefore fed a
> multi-hundred-commit pack over the 17 GiB loose-object store inside the 12 GiB dispatch
> scope and was killed — the push-side mechanism the reconciliation doc established, now
> confirmed at both endpoints with the remote state it acted on.
>
> **Workspace state, crash time vs now.** Crash time: `.git` ≈18 GB, ≈17.2 GiB loose across
> ~4,594 objects vs 9.6 MiB packed (§6). Re-verified live 2026-09-08: `.git` **105 MB**;
> **150 loose objects / 1,012 KiB**; **1 pack / 100.25 MiB**; `check-repo-health.sh` exit 0
> with the effective pack-memory bound (≈3,072 MiB worst case) verified inside the 12 GiB
> dispatch scope. Loose count sits in the ordinary daily-churn band (364 objects / 2.44 MiB
> at the 00:28Z repack verification the same morning) — four orders of magnitude below the
> storm-era figure.
>
> **Task completion, re-verified independently.** `git remote -v`: `origin` →
> `https://git.ardenone.com/jedarden/domain-check.git` ✅. `git ls-remote` of **both**
> remotes: Forgejo `main` tip = GitHub `main` tip = `77057c3` ✅. The Forgejo server-side
> push mirror is present and live (`remote_mirror_3KJHNKYU5Mw`, 8 h interval, last update
> 2026-09-08T01:18:20Z) ✅. Local `main` was ahead of `origin/main` by 1 commit at check
> time — a co-tenant's bf-4x12ec docs commit (`11bd848`, domchk-d7241598), not this task's
> work, left unpushed per the never-push-a-sibling rule. The §8 residual stands unchanged: a
> client-side remote (named `github` at crash time, `github-mirror` today) still points at
> github.com where the convention wants server-side mirroring only.

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
| `docs/crash-investigations/bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md` | **Consistent** — the guide's four-way taxonomy verdict (INFRASTRUCTURE) with false-positive rules applied; verdict identical to §11 |
| `docs/crash-investigations/bf-4yjq-crash-pattern-analysis-domchk-b9513e0b-2026-09-06.md` | **Consistent** — exit-code −1 semantics, workspace-wide pattern analysis from raw worker logs, and the last-`exit_code=-1`-anywhere date (2026-08-17); adds the cross-date incident-family comparison |
| `docs/crash-investigations/bf-4yjq-root-cause-determination-domchk-54bc57df-2026-09-06.md` | **Consistent** — renders §6 as a standalone causal chain with per-step confidence, contributing factors, and the evidence chain (also resolves the SIGHUP question against the archived framing) |
| `docs/crash-investigations/bf-4yjq-evidence-reconciliation-domchk-7a34eb37-2026-09-07.md` | **Consistent** — raw-log re-verification of §3/§5/§6 (count, window, 191.9 s interval, recovered transcripts); see its banner in §3 |

**Related investigations:** bf-1s6c3 (repository bloat, `docs/crashes/repository-bloat-crash-bf-1s6c3-2026-08-12.md`),
bf-173o7e (gc execution, `docs/crash-investigations/bf-173o7e-crash-investigation.md`),
bf-2ildm (bloat source, `docs/crash-context-bf-2ildm-complete.md`).

---

## 11. Formal Classification Record (2026-09-08, domchk-8055c332)

Formal application of the [crash response guide](../crash-response-guide.md) classification
framework to bf-4yjq's exit signals and error indicators. §1 states the verdict; this section
records how it maps onto the guide, what evidence supports it, and where the evidence stops.
Figures marked **[re-derived 09-08]** were recomputed first-hand this pass from the raw sources
(`.beads/checkpoint/forensic.jsonl`, the recovered session transcripts, live host/journal
state); carried figures keep their §3/§6 citations. This section does not overturn any earlier
verdict — it consolidates them and links the classification chain (§10).

### Verdict

| Field | Value |
|-------|-------|
| **Classification** | **INFRASTRUCTURE — repository bloat** (guide Quick Reference row 2: exit −1 + fixed-cadence re-dispatch deaths + `.git` > 5 GB; guide note 1 makes bloat a distinct infrastructure *sub-type*, not generic memory pressure) |
| **Not CODE_DEFECT** | No domain-check code was executing or implicated in any of the 50 deaths — every fatal command is a git plumbing operation over the bloated object store. Consistent with every other investigation of this workspace (zero code defects across 157+) |
| **Not FALSE_POSITIVE** | The deaths are mid-task, not post-completion (see Phase 2A below) |
| **Confidence — bloat correlation** | **HIGH** |
| **Confidence — kill mechanism** | **MEDIUM-HIGH** (kernel logs for Aug-12 are not retained — see missing evidence) |

### Phase 1 — automated classification unavailable by construction

`./scripts/crash-classifier.sh bf-4yjq` exits with `ERROR: Bead trace not found` **[re-derived
09-08]**. Both of its inputs post-date the storm:

- `.beads/traces/bf-4yjq/` does not exist — the trace directory is a single slot and holds
  nothing for the subject bead (trace capture did not exist on Aug-12).
- `.beads/events.jsonl` starts 2026-08-16T04:21 — four days *after* the last kill.

The classifier's silence is an input-coverage gap, **not** evidence that no crash occurred:
a trace-less `outcome: crash` in this era classifies from the corpus, not from the tool. The
same gap is recorded in the classification chain (domchk-b9513e0b, 2026-09-06). This is a
manual Phase 2A classification throughout.

### Phase 2A checklist, applied

| Guide checklist item | Result |
|----------------------|--------|
| Exit code / outcome | `Exit code: -1 (signal -1)` on **50/50** distinct alert beads, window 2026-08-12 17:54:00.249 → 20:30:43.716 UTC (alert-bead `created_at`, the canonical window) **[re-derived 09-08]** |
| Work completed before crash? | **No.** Every death is mid-task — a reconciliation `git push` in flight that never delivered (Forgejo `origin/main` at `63ba024` across the whole window; 50 pushes attempted, 0 delivered, §3 addendum). The task completed 2026-08-17 only after the storm ended → **INFRASTRUCTURE, not FALSE_POSITIVE** |
| System-wide event check | **Unrecoverable.** The journal holds a single boot from 2026-08-15 19:56:33 EDT (the Aug-14 reboot); `journalctl -k` for the crash window returns "No entries" **[re-derived 09-08]** |
| Cgroup boundary check | Binding limit is the dispatch scope's `MemoryMax` = 12 GiB, not the host — the guide names bf-4yjq as the type case where the kill lands inside the scope on a healthy host. Not directly re-verifiable for Aug-12 (no kernel records) |
| Pattern 1 — post-completion FP (~40%) | Excluded: work was not complete during the window |
| Pattern 2 — git gc (~15%) | Excluded: the last fatal command is `git push`, not gc (the gc-side sibling is bf-4x12ec, 2026-08-14) |
| Pattern 3 — repository bloat | **Every symptom matches**: zero exit-code variation, fixed cadence, `.git` ≈18 GB, 17.2 GiB loose vs 9.6 MiB packed, routine git operations killing agents, multiple deaths in a short period |

**Exit-code semantics (guide note 2).** `-1` is needle's sentinel for a signal death whose
code was not recorded (`code().unwrap_or(-1)`), **not** a signal number — and the
`signal -1` wording in the alert descriptions is that sentinel echoed back, not a kernel
signal reading. The correct SIGKILL encoding (137) never appears anywhere in the record, so
no specific signal may be asserted from `-1` alone; the OOM-class attribution rests on the
family inference below, and the defensible restatement is "signal death, OOM-class by family
inference".

### Indicator inventory

**Available:**

| # | Indicator | Value | Source |
|---|-----------|-------|--------|
| 1 | Exit code | `-1` on 50/50 crash alert beads, zero variation | forensic.jsonl **[re-derived 09-08]** |
| 2 | Alert labels | `signal--1` on all 50; `failure-count` escalators 1→12; `umbrella`/`verification-failed` on 33 | forensic.jsonl **[re-derived 09-08]** |
| 3 | Cadence | 191.9 s mean interval; 2h37m continuous, then stops | §3 (2026-09-07 correction; the sibling record's median 156 s is consistent) |
| 4 | Last fatal command | `git push origin main` in **50/50** crash-run transcripts (56 survive; the other 6 are non-crash sessions) | raw-logs tarball **[re-derived 09-08]** |
| 5 | Repo state at crash | `.git` ≈18 GB; 17.2 GiB loose across ~4,594 objects vs 9.6 MiB packed (≈1,800:1) | §6, corroborated by alert-bead notes written 2026-08-16 ("18GB .git with 17.2GB loose objects") |
| 6 | Temporal boundary | Kills stop exactly when the repo is packed (Aug 13); zero recurrences in 3+ weeks | §6/§7 — live check today: `.git` 105 MB, 178 loose / 1.16 MiB, 1 pack, health clean **[re-derived 09-08]** |
| 7 | Storm context | 455 exit −1 events across 6 beads the same day (100% of the day's kills) | §4 |

**Missing (named):**

| # | Missing evidence | Why it matters / why it is gone |
|---|------------------|--------------------------------|
| 1 | Kernel memcg OOM records for Aug-12 | The only *direct* proof of the mechanism (`oom-kill`, `CONSTRAINT_MEMCG`, scope/task/rss lines). Lost to the Aug-14 reboot — the journal's first entry is 2026-08-15 |
| 2 | Exit code 137 records | The direct OOM encoding; never captured for any of the 50 |
| 3 | Core dumps | None — consistent with SIGKILL, but not probative of it |
| 4 | Stack traces | None exist and none can exist: a SIGKILL leaves none. Their absence is therefore not evidence of an application fault |
| 5 | Monitor logs reaching Aug-12 | `.beads/logs/` resource-monitor floor is 2026-09-02; the monitor timers were installed long after the storm |
| 6 | Needle event records for Aug-12 | `.beads/events.jsonl` and the forensic event log both start 2026-08-16 |
| 7 | Automated classifier input | Trace slot empty (Phase 1 above) |
| 8 | Crash-era commits | Removed from the DAG by the Aug-16 squash commit |

### Confidence basis

- **Bloat correlation — HIGH.** Independent of any kernel telemetry: the contemporaneous repo
  size is recorded (item 5), the crash window ends exactly at the cleanup (item 6), the crash
  never returned once the repo stayed small, and the same regime killed 455 attempts across 6
  beads in one day (item 7).
- **Kill mechanism (memcg-OOM SIGKILL inside the 12 GiB dispatch scope during `git push`) —
  MEDIUM-HIGH.** For Aug-12 itself the mechanism is inferred, not observed: sentinel −1 plus a
  uniform push-side death plus an inverted loose:packed ratio plus a multi-hundred-commit pack
  per attempt. It is *directly kernel-verified* only for the later members of the same death
  class — bf-4x12ec (gc-side, 2026-08-14) and bf-198ne (push-side, 2026-08-16), whose
  `CONSTRAINT_MEMCG` records inside the same `MemoryMax=12 GiB` scopes established the
  mechanism (bf-1ea4g belongs to the same push-side class by its RCA). That is why this is not
  HIGH: the Aug-12 kills are attributed by class membership, not by their own kernel records.

**Nothing above changes §1/§6. What it adds is the guide-mapping, the automated tooling's
coverage boundary, an explicit statement of which evidence is present and which is missing,
and the reason each confidence level is what it is.**
