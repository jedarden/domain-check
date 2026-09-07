# bf-4yjq — Comprehensive Crash Report

**Dispatch bead:** domchk-7c9a4311 — "Document findings and create mitigation report" (terminal writeup)
**Subject bead:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo mirror has diverged/gone stale" (P2, closed 2026-08-17)
**Crash date:** 2026-08-12, 17:54:00 → 20:30:43 UTC
**Report date:** 2026-09-06
**Classification:** INFRASTRUCTURE — resource exhaustion (cgroup memcg-OOM SIGKILL during git operations). Not a code defect.
**Status:** ✅ REPORT COMPLETE — investigation chain closed; subject task verified complete; crash class extinct

> **Corrections-list compliance.** This report restates no entry on the corpus's canonical
> superseded-claims list (§7 of
> [`docs/investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md`](../investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md)):
> `exit -1` here means **needle's sentinel for a signal death with no recorded code**, the
> kill mechanism is **cgroup-scoped kernel memcg-OOM SIGKILL inside the 12 GiB dispatch
> scope** (not SIGHUP, not host-wide OOM), and the event size is **50 crashes at
> ~3.1-minute intervals** (not 9 at ~17 minutes).

> **Document map — which bf-4yjq report to trust.** Two earlier documents carry
> near-identical titles and are **superseded on crash count and mechanism**:
> [`docs/reports/bf-4yjq-comprehensive-crash-report.md`](../reports/bf-4yjq-comprehensive-crash-report.md)
> (9-crash record) and
> [`docs/crashes/bf-4yjq-crash-report.md`](../crashes/bf-4yjq-crash-report.md) (9-crash
> record, intermediate 1.7 GB post-cleanup figure). Both already banner-link the canonical.
> Reading order for the full record: **this report** (chain-terminal integration) →
> [canonical crash investigation](bf-4yjq-crash-investigation.md) (domchk-4eab7c59 — 50-crash
> verification, storm table, task outcome) →
> [root cause determination](bf-4yjq-root-cause-determination-domchk-54bc57df-2026-09-06.md)
> (domchk-54bc57df — contributing factors, evidence chain E1–E10, SIGHUP falsification) →
> [consolidated findings](../crashes/bf-4yjq-consolidated-findings-domchk-4ed0544b-2026-09-06.md)
> (domchk-4ed0544b, the 2026-08-26 chain's terminal — fix verification, nine lessons).

**Position in chain:** domchk-b6a900e4 (artifacts preserved) → domchk-b9513e0b (classification,
commit `8884670`) → domchk-54bc57df (root cause, commit `7028918`) → **this record (crash
report)** → unblocks alert umbrella [bf-2weev] together with domchk-dcc7762d (fix-verification
bead, closed 2026-09-02 as resolved-duplicate: target already complete).

---

## 1. Executive summary

bf-4yjq was an ordinary **git remote reconciliation task** (repoint `origin` at Forgejo,
reconcile the Forgejo/GitHub divergence with a merge commit — no force-push — and configure
the Forgejo→GitHub server-side push mirror). On 2026-08-12 the retry loop dispatched agents
against it into a lethal environment and lost **50 consecutive agents in 2 h 37 m**, every one
with exit code −1, one every ~3.1 minutes.

The environment: the repository's object store had been bloated to **~18 GB** (17.2 GiB loose
across ~4,594 objects vs 9.6 MiB packed — a ≈1,800:1 inverted loose:packed ratio) by earlier
commits of ~237 MB `.beads/` JSONL snapshots. Every substantive git operation on that store
pulled a working set larger than the **12 GiB needle dispatch scope** could hold, and the
kernel's memory-cgroup OOM killer ended the git process with an uncatchable SIGKILL. Nothing
any retry did could shrink the trigger — the kills left the loose set untouched — so each
re-dispatch hit the same wall.

The bead sat in the **middle shift of a workspace-wide storm: 455 exit-−1 events across 6
beads, 05:36–23:57 UTC that day**. Crashes stopped when the repository was packed down on
Aug 13–14 and never returned; the task itself then completed normally (closed 2026-08-17) and
is verified correct on the live repo. **Zero domain-check code defects were found** — the
crash exposure was a function of when the bead was scheduled relative to the bloat window,
not of what it was doing.

## 2. Timeline

| When (UTC unless noted) | Event |
|---|---|
| 2026-07-20 13:59 | bf-4yjq created — git remote reconciliation task |
| pre-Aug-12 | bf-2ildm-era workflow commits ~237 MB `.beads/` JSONL snapshots; repo reaches ~18 GB (17.2 GiB loose / 9.6 MiB packed) |
| 2026-08-12 05:36 | Workspace storm opens — first of **455 exit-−1 events across 6 beads** (bf-31mno 350, bf-4yjq 50, bf-1s6c3 49, bf-2xygo 4, bf-23n 1, bf-5d18 1) |
| 2026-08-12 17:54:00 → 20:30:43 | **bf-4yjq loses 50 consecutive agents**, 100% exit −1, mean inter-death gap 188–192 s (median 156 s; per-run survival median 149 s), peak 5 deaths/10 min — below every per-bead surge threshold for the entire window |
| 2026-08-12 (window) | Contemporaneous telemetry: load 15–17 on 12 cores, memory exhausted *during git operations*, disk 84% full, `git fsck` timing out; `failure-count` alert escalators fire 1→4 while deaths continue |
| 2026-08-12 21:36 → 23:57 | Storm's last shift absorbed by bf-1s6c3 (49 deaths) — the bloat crash documented in CLAUDE.md |
| 2026-08-13 → 08-14 | Repository packed down (18 GB → ~91–138 MB); bf-4yjq's divergence-analysis artifacts dated Aug-13 show work proceeding normally |
| 2026-08-14 → 08-16 | Same death class recurs on better-instrumented days: bf-4x12ec (gc-side) and bf-198ne (push-side, kernel-proven `CONSTRAINT_MEMCG`) |
| 2026-08-17 00:14 | bf-4yjq closed — remote reconciliation completed and verified (re-verified live 2026-09-02, canonical §8) |
| 2026-08-26 / 09-01 | Investigation waves producing the 9-crash record and first remediation strategy |
| 2026-09-01/02 | Bloat-prevention system landed (commit `753ea04`); crash-alert fixes (2026-09-02); **canonical crash investigation** corrects the record to 50 crashes (domchk-4eab7c59); this dispatch's split chain created |
| 2026-09-06 | Full verification day: cleanup re-verified holding, mechanism re-created at 1/17th scale (6/6 assertions × 3 runs), fix re-verified live, corrections list canonized, root cause determined (domchk-54bc57df), **this report** closes the chain |

## 3. Evidence — what survives and what cannot

The mechanism for Aug-12 itself rests on **contemporaneous telemetry plus a directly observed
reproduction** — the raw Aug-12 kernel record cannot exist (system journal begins 2026-08-15
19:46 EDT) and no core dumps, heartbeats, session traces, or crash-era commits survive. The
numbered evidence chain (E1–E10) with per-step confidence lives in the
[root cause determination](bf-4yjq-root-cause-determination-domchk-54bc57df-2026-09-06.md) §4;
key steps:

- **E1** Era `git count-objects`: 4,594 objects / 17.20 GiB loose / 9.60 MiB packed (contemporaneous).
- **E2/E3** 50/50 deaths exit −1 with zero variation; cadence ~3.1 min for 2.61 h, stopping exactly when the repo was packed ([preserved worker-log extract](../crash-analysis/bf-4yjq-needle-worker-log-extract.log), 225 records, re-scanned live 2026-09-06).
- **E5** Kernel journal starts 2026-08-15 — the Aug-12 window has no kernel record (confirmed absence).
- **E6/E8** Every kernel-recorded kill in the recoverable window is `CONSTRAINT_MEMCG` (447/447; 269 victims `task=git`), trace `mem_cgroup_out_of_memory → try_charge_memcg`; the gc-side (bf-4x12ec) and push-side (bf-198ne) variants are kernel-proven members of the same death class.
- **E7** Live dispatch scope `MemoryMax=12884901888` = exactly 12 GiB — the binding constraint exists today as it did then.
- **E9** Zero `exit_code=-1` anywhere after 2026-08-17 across continuous log-slot coverage — trigger removed, class extinct.
- **E10** SIGHUP ruled out: 0 signal attributions in the primary telemetry, `-1` is not a signal number, and 213 kernel memcg git-kills fall inside the very window an archived doc had labeled a "SIGHUP cascade".
- **Re-creation** `scripts/test-bf-4yjq-crash-condition.sh` reproduces the memcg-OOM kill at 1/17th scale and shows both mitigations neutralizing it (6/6 assertions × 3 runs).

## 4. Root cause (statement; derivation not re-derived here)

**The 50 deaths were the resource-exhaustion death of the era: an ~18 GB bloated object store
(accumulation) × unbounded git operations (mechanism) inside the 12 GiB dispatch scope
(constraint), amplified into a storm by a retry loop with no environment re-check.** Two
independent defects had to co-exist — bounds without accumulation prevention still lets a repo
grow until some non-packing operation chokes; accumulation prevention without bounds leaves
every already-bloated repo lethal. Task content was irrelevant. Full derivation, confidence
table, and contributing factors: canonical §6 and
[root cause determination](bf-4yjq-root-cause-determination-domchk-54bc57df-2026-09-06.md) §§1–2.

## 5. Mitigation status and recommendations

### 5.1 Deployed mitigations — re-verified live for this report, 2026-09-06

| Layer | Safeguard | Live check this dispatch | Result |
|---|---|---|---|
| A — stop accumulation | `.beads/` gitignored (`.gitignore:66`) + repo-wide `*.db` / `*.jsonl` | `grep` + `git ls-files .beads` | ✅ 0 tracked `.beads/` files |
| A | 10 MB pre-commit large-file gate | `.git/hooks/pre-commit` present, executable | ✅ installed (this clone only) |
| A | Daily repo-health + auto-gc, weekly full gc | `systemctl --user list-timers 'domain-check-*'` | ✅ all 6 timers armed with future triggers |
| A | Standing bloat repaired | `du -sh .git` / `git count-objects -vH` | ✅ 97 MB; 85 loose / 3.98 MiB; 10,980 in-pack / 90.93 MiB; 0 garbage |
| B — bound the operation | `pack.windowMemory=2g` / `deltaCacheSize=1g` / `threads=1`, repo-local + box-global | `./scripts/setup-git-gc-config.sh --verify` | ✅ exit 0 — effective bound ≈3072 MiB worst case, within ceiling for the 12 GiB scope; covers bare `git gc` **and** `git push` |
| B | Crash command under bound | `scripts/test-gc-memory-bounds.sh` (2026-09-06, consolidated §5.2) | ✅ bare `gc --aggressive --prune=now` exits 0 at ~313 MiB peak under a 768 MiB cgroup |

Figures above are this dispatch's own runs and supersede earlier snapshots (94 MB / 1 pack at
11:11 today, 92 MB on 2026-09-02) — churn between runs is normal; re-verify rather than cite.

### 5.2 Open gaps — recommended next actions, prioritized

| # | Gap | Evidence | Recommendation |
|---|-----|----------|----------------|
| 1 | **Storm detection is blind right now** — per-bead keying missed this event (peak 5/10 min < 10/10 min threshold) and the detector's only event source is stale | `./scripts/crash-pattern-detection.sh` → **DEGRADED**: `.beads/events.jsonl` newest record 2026-08-26, none within 24 h (re-run live this dispatch) | Add an aggregate workspace-level exit-−1 rate signal (canonical §9 rec 1) **and** repoint/refresh the event source — a detector that is green-on-stale is worse than no detector |
| 2 | **Retry loop without environment re-check** turned one lethal condition into 50 deaths | 50/50 deaths at ~3-min cadence; median survival 149 s | Needle-side: back off re-dispatch of a bead whose last death was a signal kill until the environment is re-checked (gap G-11); alert-side suppression already exists in `crash-alert-manager.sh` |
| 3 | **Pre-commit gate is per-clone and bypassable** | No installer; source copy drifted; `--no-verify` bypasses | Ship an installer + wire the check into the daily health timer (gap G-1); CI clone stays unguarded by design (fresh clone each build) |
| 4 | **Evidence retention** still limits finality for any future event of this class | Aug-12 has no kernel record; traces single-slot | Retain kernel OOM + journald ≥30 days, rotate rather than overwrite traces, stamp UTC (gap G-8) |
| 5 | **Alert-hygiene debt** — hundreds of storm alert beads open, distorting counts and spawning duplicate investigations | Most of bf-4yjq's 50 alert beads open (canonical §8) | One-time bulk-close of alerts whose targets are resolved, with a referencing note (canonical §9 rec 2) |

Gaps G-1..G-13 are catalogued with owners in
[`docs/crash-prevention-requirements.md`](../crash-prevention-requirements.md); NEEDLE-side
items (G-9..G-13, including the retry loop) are external asks, not repo work.

## 6. Lessons learned (summary)

Distilled from this chain; the full nine, with their rule formulations, are consolidated
findings §6 — cited, not restated. The first four are also root-cause patterns P1–P4 of the
prevention requirements doc.

1. **`exit -1` is a sentinel, not a signal** — assert no signal without kernel evidence (R-DOC-1). This single misreading drove more misclassification than any other factor.
2. **The binding constraint is the dispatch scope, not the host** — host-wide memory reasoning and host-wide alerting both miss the boundary that actually kills.
3. **Alert sampling cannot measure scale** — it recorded 9 crashes where the forensic checkpoint held 50, plus a 350-kill storm recorded nowhere. The checkpoint is the source of truth.
4. **Crashes cluster; per-bead response is the wrong granularity** — a spike of sub-3-minute exit-−1 re-dispatch deaths across beads is an environmental regime; triage repo/memory/load at the workspace level first.
5. **Fix both co-requisite defects or the crash stays reachable** — accumulation (Layer A) and unbounded operation (Layer B) each independently suffice to kill; the verification had to prove each layer's negative case.
6. **Evidence is perishable** — every limitation in this report traces to retention decay, not investigative effort.
7. **Near-identical titles are a dedup hazard in both directions** — this chain's own reports were re-dispatched against a title match; check the bead's deliverable and `git log --grep <bead-id>` before starting, and cite-or-extend the corrections list, never restate it.

## 7. Sources

| Document | Role |
|---|---|
| [`bf-4yjq-crash-investigation.md`](bf-4yjq-crash-investigation.md) | Canonical — 50-crash verification, storm table, task outcome, recommendations |
| [`bf-4yjq-root-cause-determination-domchk-54bc57df-2026-09-06.md`](bf-4yjq-root-cause-determination-domchk-54bc57df-2026-09-06.md) | Direct predecessor — contributing factors, evidence chain E1–E10, SIGHUP falsification |
| [`../crashes/bf-4yjq-consolidated-findings-domchk-4ed0544b-2026-09-06.md`](../crashes/bf-4yjq-consolidated-findings-domchk-4ed0544b-2026-09-06.md) | 2026-08-26 chain terminal — fix layers, live verification §5.2, nine lessons |
| [`../crashes/bf-4yjq-fix-proposal-verification-2026-09-06.md`](../crashes/bf-4yjq-fix-proposal-verification-2026-09-06.md) | Fix trade-offs and residual risks |
| [`../crashes/bf-4yjq-cleanup-verification.md`](../crashes/bf-4yjq-cleanup-verification.md) | Repository repair record (18 GB → 97 MB, holding) |
| [`bf-4yjq-crash-workload-test-spec-domchk-b90505ad-2026-09-06.md`](bf-4yjq-crash-workload-test-spec-domchk-b90505ad-2026-09-06.md) | Mechanism re-creation at 1/17th scale + harness |
| [`../crash-analysis/bf-4yjq-needle-worker-log-extract.log`](../crash-analysis/bf-4yjq-needle-worker-log-extract.log) | Primary crash-era telemetry (225 records) |
| [`../crash-prevention-requirements.md`](../crash-prevention-requirements.md) | Gap list G-1..G-13 (owners, priorities) |
| [`../investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md`](../investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md) §7 | Canonical superseded-claims list |
| `docs/crashes/bf-198ne-crash-report.md`, `docs/crash-investigation-bf-4x12ec.md` | Kernel-proven family members (push-side, gc-side) |
| Live system state (this dispatch, 2026-09-06): `git count-objects`, `du`, `systemctl --user list-timers`, `setup-git-gc-config.sh --verify`, `crash-pattern-detection.sh` | §5.1 verification column |

---

*Report for dispatch domchk-7c9a4311, 2026-09-06. Live verification commands recorded in §5.1
were executed on the working repo during this dispatch. This document adds no new analysis of
the crash — its content is the terminal integration: timeline, evidence summary, mitigation
status re-verified live, and the recommendations that remain open.*
