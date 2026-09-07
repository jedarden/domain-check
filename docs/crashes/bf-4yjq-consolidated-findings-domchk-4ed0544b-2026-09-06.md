# bf-4yjq Crash Investigation — Consolidated Findings Report

**Date:** 2026-09-06
**Dispatch bead:** domchk-4ed0544b — final link of the 2026-08-26 chain
[root cause analysis](../../crash-root-cause-bf-4yjq.md) (domchk-04e36955, closed) →
[fix proposal + verification](bf-4yjq-fix-proposal-verification-2026-09-06.md) (domchk-0c601026, closed) →
**this consolidated report**
**Subject bead:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo mirror has diverged/gone stale" (P2, closed 2026-08-17)
**Canonical crash record:** [`docs/crash-investigations/bf-4yjq-crash-investigation.md`](../crash-investigations/bf-4yjq-crash-investigation.md) (domchk-4eab7c59, 2026-09-02)
**Status:** ✅ INVESTIGATION CLOSED — root cause identified, fix deployed and verified, task independently verified complete

> **Superseded-claims compliance.** This report restates no entry on the corpus's
> canonical corrections list (§7 of
> [`docs/investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md`](../investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md)).
> In particular: `exit -1` here is **needle's sentinel for a signal death with no
> recorded code**, not a signal number; the kill mechanism is **cgroup-scoped kernel
> memcg OOM SIGKILL**, not SIGHUP and not host-wide exhaustion; and the event size is
> **50 crashes at ~3.1-minute intervals**, not the 9-at-17-minutes figure still
> repeated in several older documents. Where an older document disagrees with this
> report, this report defers to the sources in §8.

This document is the single entry point the chain was asked to produce: a summary for
quick reference (§1), pointers into the evidence for deep dives (§3), the root cause
with its confidence basis (§4), the deployed fix with its verification steps (§5), and
the lessons future investigations should carry (§6). It adds no new analysis of the
crash itself — its new content is the consolidation and the live re-verification of
2026-09-06 recorded in §5.2.

---

## 1. Quick reference

| | |
|---|---|
| **What** | Every agent dispatched against bf-4yjq was killed on arrival by the kernel's memory-cgroup OOM killer while running git operations in this workspace |
| **When** | 2026-08-12, 17:54:00 → 20:30:43 UTC — 2 h 37 m (alert-bead creation times; the deaths themselves land ~6 s earlier) |
| **Scale** | **50 consecutive deaths, 100 % exit code −1, zero variation** — mean interval 188 s; each run did 65–375 s of real work before dying (median survival 149 s) |
| **Context** | The middle shift of a same-day workspace storm: **455 exit −1 events across 6 beads, 05:36–23:57 UTC** (bf-31mno 350, bf-4yjq 50, bf-1s6c3 49, bf-2xygo 4, bf-23n 1, bf-5d18 1) |
| **Root cause** | Unbounded git pack operations over **17.2 GiB of loose objects** (≈1,800:1 loose:packed) inside the **12 GiB needle dispatch scope** → cgroup-scoped memcg OOM SIGKILL |
| **Classification** | **INFRASTRUCTURE — resource exhaustion.** Not a code defect (zero domain-check defects in any investigation of this workspace), not a service failure, not a false positive |
| **Fix** | Two layers, both deployed: (A) stop the accumulation, (B) bound the operation — see §5 |
| **Verification** | Crash command re-run under 1/16th of the dispatch scope on 2026-09-06: exit 0 at ~313 MiB peak RSS; mechanism independently re-created and mitigated at 1/17th scale (`scripts/test-bf-4yjq-crash-condition.sh`, 6/6 assertions × 3 runs) |
| **Task outcome** | The bead's actual work (Forgejo-primary remotes + server-side push mirror) completed after the storm, closed 2026-08-17, re-verified live 2026-09-02 |
| **Current state** | Repo 94 MB, all safeguards green — verified live 2026-09-06, §5.2 |

---

## 2. Crash summary

bf-4yjq was a **git remote reconciliation task**, not a crash task: repoint `origin` at
Forgejo, reconcile the Forgejo/GitHub divergence with a merge commit (no force-push),
configure the Forgejo→GitHub server-side push mirror. Between 17:54 and 20:31 UTC on
2026-08-12, the retry loop lost **50 consecutive agents** — one every ~3.1 minutes for
2 h 37 m — while the workspace sat on a ~18 GB repository whose object store was
**17.2 GiB loose across ~4,594 objects vs 9.6 MiB packed**.

Each dispatch died the same way: the agent started, spent 65–375 seconds on ordinary
work, touched its first store-walking git operation (status/fetch on a cold store,
packing, fsck), and the operation's working set exceeded the memory bound of the needle
dispatch scope it ran in. The kernel's **memcg OOM killer SIGKILLed the git process**,
the scope died, and needle recorded the sentinel `exit code: -1`. The kill was
deterministic — 50/50 with zero exit-code variation — and it left the loose set
untouched, so nothing any retry did could shrink the trigger: every re-dispatch hit the
same wall. `failure-count` alert escalators (1→4) fired while the crashes continued.

**Impact.** The bead's own work was stalled for the window and progressed only after the
repository was packed down on Aug 13–14 (18 GB → ~91–138 MB); it then completed normally
and closed 2026-08-17, and remains verified correct on the live repo (canonical §8).
The wider cost was the storm it sat inside: 455 kills in one day across 6 beads, plus
the duplicate-alert wake that followed — bf-4yjq alone attracted dozens of alert beads,
several of which generated further investigation dispatches for an already-resolved
event.

**What the crash was not.** Not a domain-check code defect (none has ever been found).
Not caused by anything in the remote-reconciliation work — the crash exposure was a
function of *when the bead was scheduled* relative to the bloat window, not of what it
was doing. The older reports' claim that the bead was "BLOCKED and not actively
executing" at crash time is unverifiable from surviving evidence and inconsistent with
the verified ~3-minute re-dispatch-kill cadence; the defensible statement, which all
investigation waves agree on, is that the crashes were **incidental to the task's
content**.

---

## 3. Evidence gathered — and what did not survive

Every path below was re-checked live on 2026-09-06 by the
[artifact catalog](../crash-investigations/bf-4yjq-artifact-catalog-2026-09-06.md)
(domchk-e92faa40), which supersedes the earlier `crash-artifacts-bf-4yjq*.md`
inventories.

| Evidence | Where | Status |
|---|---|---|
| **Worker fleet log slot `.log.2`** — the only surviving Aug-12 primary source; 225 bf-4yjq records (17:50:23Z → 21:14:59Z), re-counted live | `~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2` (134 MB, 2026-08-11 → 08-15) | ⚠️ Single rotation slot — will be overwritten as the log rotates; it is the last primary record of this event |
| **50 alert beads** in the bead store — the 50-crash count was independently re-derived from `.beads/checkpoint/forensic.jsonl` twice (domchk-d5dd1b33, domchk-4eab7c59) | `.beads/checkpoint/forensic.jsonl` | ✅ Durable, gitignored |
| Per-run extraction: all 50 crashes with survival times and per-kill detail; median corrected 155 → 149 s | [`bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md`](../crash-investigations/bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md) | ✅ Committed |
| Contemporaneous system metrics (18 GB repo, load 15–17, disk 84 %) | `.beads/crash-bf-4yjq-summary.txt` | ✅ Retained; superseded on crash count only |
| **Mechanism proof for the crash class** — kernel `CONSTRAINT_MEMCG` records with `usage == limit == 12582912kB` | [`docs/crashes/bf-198ne-crash-report.md`](bf-198ne-crash-report.md) (Aug-16 push-side variant) | ✅ Committed — Aug-12's own kernel logs are gone |
| **Experimental re-creation of the mechanism** — memcg OOM kill reproduced at 1/17th scale, both mitigations verified | [`bf-4yjq-crash-workload-test-spec-domchk-b90505ad-2026-09-06.md`](../crash-investigations/bf-4yjq-crash-workload-test-spec-domchk-b90505ad-2026-09-06.md) + `scripts/test-bf-4yjq-crash-condition.sh` | ✅ Committed; 6/6 assertions × 3 runs |
| Core dumps, kernel OOM logs, Aug-12 heartbeats, session traces, crash-era git commits | — | ❌ **None survive.** No journald retention before 2026-08-15 19:46 EDT; traces are single-slot; the Aug-16 `c27899f` squashed the crash-era commits out of the DAG |

The evidentiary shape matters as much as the contents: the mechanism for Aug-12 itself
rests on **contemporaneous telemetry plus a directly observed reproduction**, not on
raw Aug-12 kernel logs, which cannot exist.

---

## 4. Root cause analysis

**Mechanism (confidence HIGH on the bloat correlation; MEDIUM-HIGH on the Aug-12
mechanism from telemetry, with the mechanism class directly observed in reproduction):**

1. Pre-Aug-12: the bf-2ildm-era workflow repeatedly committed ~237 MB `.beads/` JSONL
   files (`.beads/` was not yet gitignored) → repository reached **~18 GB**, with
   **17.2 GiB loose vs 9.6 MiB packed** — a severely inverted loose:packed ratio.
2. 2026-08-12 from 05:36 UTC: substantive git operations over that object store pulled
   multi-GB working sets and began dying; contemporaneous telemetry records load 15–17
   on 12 cores and disk 84 % full.
3. 17:54–20:31 UTC: bf-4yjq's retry loop lost 50 consecutive agents at ~3.1-minute
   intervals. Each kill was a **cgroup-scoped memcg OOM SIGKILL inside the 12 GiB
   dispatch scope** — the host was not out of memory, which is exactly why host-wide
   alerting could never have caught it.
4. Aug 13–14: the repository was packed down (18 GB → ~91–138 MB). bf-4yjq's work then
   proceeded normally; the bead closed 2026-08-17. Crashes of this class never returned.

**Two independent defects had to co-exist**, which is the analysis the fix must honor:

1. **Unbounded accumulation** — the repo held ~18 GB it should never have held
   (`.beads/` tracked; no size gate on commits).
2. **Unbounded git operation** — bare `git gc` (and the packing side of `git push`) had
   no memory ceiling, so repository size mapped one-to-one onto agent death.

Fixing either one alone leaves the crash reachable: bounds without accumulation
prevention would still let the repo grow until some non-packing operation choked, and
accumulation prevention without bounds would leave every already-bloated repo lethal.

**Why the mechanism is credited despite lost logs:** (a) the exit signature is uniform
(50/50 `-1`) at a re-dispatch cadence that only an environmental kill explains; (b) the
crashes stopped exactly when the trigger was removed and never returned; (c) the same
kill class is proven by kernel records for bf-198ne (the push-side variant, Aug-16) and
bf-4x12ec (Aug-14); and (d) the harness reproduces it end-to-end at 1/17th scale —
`CONSTRAINT_MEMCG`, anon-rss ≈ the bound, loose set untouched — and shows both
mitigations neutralizing it (§3, last row).

---

## 5. Proposed fix and verification

Deliverable of chain link 2
([`bf-4yjq-fix-proposal-verification-2026-09-06.md`](bf-4yjq-fix-proposal-verification-2026-09-06.md),
domchk-0c601026). **No new code was required — the fix was already deployed; the chain's
job was to verify it live.** Summarized here because a consolidated report must carry
the fix, with full trade-offs in the source document.

### 5.1 The two layers

**Layer A — stop the accumulation (kills the precondition):** `.beads/` gitignored
(`.gitignore:66`, plus `*.db` / `*.jsonl` repo-wide, 0 tracked `.beads/` files); 10 MB
pre-commit large-file gate (`.git/hooks/pre-commit`, `MAX_SIZE_MB=10`); daily
repo-health check with >500 MB alert thresholds (`scripts/check-repo-health.sh`,
`domain-check-repo-health.timer` 02:00); and the standing bloat itself repaired
(18 GB → 94 MB, [cleanup verification](bf-4yjq-cleanup-verification.md)).

**Layer B — bound the operation (kills the mechanism):** `pack.windowMemory=2g`,
`pack.deltaCacheSize=1g`, `pack.threads=1` applied **repo-local and box-global**
(`scripts/setup-git-gc-config.sh` — `threads=1` is mandatory because the window limit
is per-thread), worst case ≈3 GiB per pack run vs the 12 GiB scope; effective-bound
verification (`--verify`, exit 1 unless the *effective* system→global→local chain
carries a safe bound); the sanctioned gc path (`scripts/safe-git-gc.sh`, memory-limited
with checkpoint/resume); and the weekly full-gc timer self-capped at `MemoryMax=4G`.

Layer B is fleet-wide by construction (it lives in global git config) and covers both
observed kill paths — bare `git gc` (bf-4yjq, bf-173o7e, bf-4x12ec) and `git push`
(bf-198ne).

### 5.2 Verification — re-executed live 2026-09-06 for this report

| Check | Command | Result |
|---|---|---|
| Effective pack bound | `./scripts/setup-git-gc-config.sh --verify` | **exit 0** — windowMemory=2g / deltaCache=1g / threads=1, worst case ≈3072 MiB within the 6 GiB ceiling for a 12 GiB scope |
| Crash command under bound | `./scripts/test-gc-memory-bounds.sh` | **12 passed, 0 failed** — bare `git gc --aggressive --prune=now` exits 0 under `MemoryMax=768M` (1/16th of the dispatch scope), pack-objects peak RSS **320,388 KB (~313 MiB)** vs >12 GiB unbounded; repo fully packed |
| Negative controls | same suite | `--verify` correctly rejects an unbounded repo and an unthreaded one, preserves unrelated `gc.*` tuning, and passes via the global bound alone |
| Accumulation gates | `grep`/`git ls-files`, `./scripts/check-repo-health.sh` | `.beads/` ignored (line 66), 0 tracked `.beads/` files, health check **exit 0** |
| Repository state | `git count-objects -vH`, `du -sh .git` | 205 loose objects (1.54 MiB), 10,712 in-pack, **1 pack (90.43 MiB)**, 0 garbage, `.git` **94 MB** |
| Monitoring alive | `systemctl --user list-timers 'domain-check-*'` | all **6 timers** armed with future trigger times |
| Mechanism re-creation | `scripts/test-bf-4yjq-crash-condition.sh` (per the test spec) | 6/6 assertions × 3 runs: memcg OOM kill reproduced at 1/17th scale, then neutralized by both mitigations |

**Regression safety:** the bounds cap memory only — the suite demonstrates a fully
packed, fsck-clean repo, and its negative controls prove `--verify` fails closed on
unsafe configs. The costs are performance-only (`threads=1` single-threaded packing; a
smaller delta window can lengthen searches) and are accepted in the fix-proposal
document's trade-off table, which also records the residual risks: non-packing git
reads stay unbounded (gated in practice by Layer A), the pre-commit hook is bypassable
with `--no-verify` (the daily health timer still catches the result within 24 h), and
needle's zero-backoff re-claim loop — which amplified one deterministic kill into a
50-death storm — is needle-side and out of this repo's reach.

---

## 6. Lessons learned

Patterns and process improvements this investigation contributed to the workspace's
standing guidance. The first four are also root-cause patterns P1–P4 of
[`docs/crash-prevention-requirements.md`](../crash-prevention-requirements.md), which is
the live-verified inventory this report does not duplicate.

1. **`exit -1` is a sentinel, not a signal number.** Needle writes `-1` for any signal
   death with no recorded code (`code().unwrap_or(-1)`); the correct Unix encoding of a
   SIGKILL death is 137. Four incompatible readings coexisted across the corpus and it
   was the largest single source of misclassification. **Rule (R-DOC-1):** treat
   `exit -1` as "died by signal; mechanism unknown until you read kernel/journald
   records," and never assert a specific signal without kernel evidence.
2. **The binding memory constraint is the dispatch scope, not the host.** The kill
   decision happened at the 12 GiB cgroup boundary while the 62 GB host had memory to
   spare. Early docs reasoning from host totals ("62 GB total / <2 GB available") reached
   wrong conclusions, and any safeguard that watches only host-wide memory misses the
   constraint that actually kills.
3. **Alert sampling cannot measure scale — the forensic checkpoint is the source of
   truth.** Alert-bead sampling recorded **9** crashes for this bead; the full
   checkpoint scan showed **50**, plus a 350-kill storm (bf-31mno) recorded nowhere.
   Any per-bead history built from the alerts an investigator happened to find
   undercounts by whatever the search missed.
4. **Crashes cluster; per-bead response is the wrong granularity.** 455 kills across 6
   beads in one day, and 73 % of a later 247-event window fell in five hours. A spike of
   sub-3-minute exit −1 re-dispatch deaths across multiple beads is an *environmental
   regime* — triage repo size / memory / load at the workspace level before any
   per-bead debugging. Surge detection now keys on 3-in-5-minutes for exactly this
   reason.
5. **A crash like this is a condition you re-create, not an event you re-run.** The kill
   was deterministic while the trigger existed (50/50) and unobtainable once it was
   removed. The productive reproduction is the *condition* at scaled size, which is what
   the harness does — rebuilding 17 GiB of loose objects on the live repo would itself
   be the hazard the guardrails exist to prevent.
6. **Fix both co-requisite defects or the crash stays reachable.** Accumulation and
   unbounded operation each independently suffice to kill; the durable fix needed the
   gitignore/hook layer *and* the pack-bounds layer, and the verification had to prove
   each layer's negative case, not just the happy path.
7. **Evidence is perishable, and its decay shaped every conclusion.** Single-slot trace
   retention, no journald before Aug-15, UTC/EDT timestamp mixing, and the squashed
   crash-era commits meant the mechanism could only be established by inference plus
   later-variant kernel records. Retain kernel OOM + journald ≥30 days, rotate rather
   than overwrite traces, stamp everything UTC (gap G-8).
8. **The retry loop amplifies one deterministic kill into a storm.** Fifty deaths here
   are one environmental condition meeting a zero-backoff re-dispatch loop; treating
   them as fifty failures produces fifty phantom investigations. The amplification is
   needle-side (external requirement G-11); `crash-alert-manager.sh` contains only the
   alert-side of it.
9. **Near-identical artifact titles are a dedup hazard in both directions.** This
   workspace's crash corpus grew to ~460 documents with contradictory claims, several
   generated by re-dispatched investigations matching a title rather than a bead. Every
   new document must cite or extend the corrections list rather than restate its
   entries, and every dispatch should be checked against the bead's own deliverable and
   `git log --grep <bead-id>` before new work starts — this report exists because that
   check showed the chain's final link was genuinely missing, not because the
   investigation lacked documentation.

---

## 7. Open items (not this chain's scope)

- **Prevention gaps** are catalogued, prioritized, and live-verified in
  [`docs/crash-prevention-requirements.md`](../crash-prevention-requirements.md) —
  including **G-1** (hook installation not reproducible on a fresh clone), **G-2**
  (detection without automatic remediation), **G-3** (no system-event response gate),
  and **G-8** (evidence retention); plus NEEDLE-side asks **G-9–G-13** (work-completion
  detection at the alert source, dispatch-scope sizing, retry with backoff,
  complexity-aware turn budgets, CPU saturation controls). Implementation belongs to
  those items, not here.
- **Aug-12 alert-bead backlog:** hundreds of storm alert beads, including most of
  bf-4yjq's 50, remain open — the known alert-hygiene debt (canonical §9, recommendation
  2). A one-time bulk-close of alerts whose targets are already resolved is warranted.
- **Stale documents still repeating the 9-crash figure** with no supersession banner —
  e.g. `docs/crash-investigation-bf-4yjq-summary-2026-08-26.md` — belong to the
  documentation-refresh bead domchk-7625a5cc (flagged by the
  [consolidated summary](../crash-investigations/bf-4yjq-consolidated-summary-domchk-ea5c6a63-2026-09-06.md)).
- **Scaled harness chain:** the spec's successors (domchk-30e8aab9 baseline run,
  domchk-10404857 regression suite, domchk-3e443d56 test report, domchk-fbb7bbbd verify
  fix) own the recurring execution of `scripts/test-bf-4yjq-crash-condition.sh`.

---

## 8. Source index

| Document | Role |
|---|---|
| [`docs/crash-investigations/bf-4yjq-crash-investigation.md`](../crash-investigations/bf-4yjq-crash-investigation.md) | **Canonical crash record** — 50-crash verification, storm table, task outcome, recommendations |
| [`docs/crash-circumstances-bf-4yjq-domchk-d5dd1b33-2026-09-02.md`](../crash-circumstances-bf-4yjq-domchk-d5dd1b33-2026-09-02.md) | Verified primary analysis; superseded the 9-crash record |
| [`docs/crash-investigations/bf-4yjq-artifact-catalog-2026-09-06.md`](../crash-investigations/bf-4yjq-artifact-catalog-2026-09-06.md) | Live-verified evidence inventory (§3 of this report defers to it) |
| [`docs/crash-investigations/bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md`](../crash-investigations/bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md) | Per-run extraction, all 50 crashes; timing figures |
| [`docs/crash-investigations/bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md`](../crash-investigations/bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md) | INFRASTRUCTURE verdict under the crash-response guide's four-way taxonomy |
| [`docs/crashes/bf-4yjq-fix-proposal-verification-2026-09-06.md`](bf-4yjq-fix-proposal-verification-2026-09-06.md) | Chain link 2 — fix description, trade-offs, residual risks |
| [`docs/crashes/bf-4yjq-cleanup-verification.md`](bf-4yjq-cleanup-verification.md) | Repository repair record (18 GB → 94 MB, holding) |
| [`docs/crash-investigations/bf-4yjq-crash-workload-test-spec-domchk-b90505ad-2026-09-06.md`](../crash-investigations/bf-4yjq-crash-workload-test-spec-domchk-b90505ad-2026-09-06.md) | Mechanism reproduction at 1/17th scale + harness |
| [`docs/crashes/bf-198ne-crash-report.md`](bf-198ne-crash-report.md) | Kernel-record proof of the mechanism class (push-side variant) |
| [`docs/crash-prevention-requirements.md`](../crash-prevention-requirements.md) | Live-verified safeguard inventory and the gap list (§7 defers to it) |
| [`docs/investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md`](../investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md) §7 | Canonical superseded-claims list this report complies with |
| `docs/crashes/bf-4yjq-crash-report.md`, `docs/reports/bf-4yjq-comprehensive-crash-report.md`, `docs/crash-root-cause-bf-4yjq.md`, `docs/remediation-strategy-bf-4yjq.md`, `docs/crash-pattern-analysis-bf-4yjq.md`, and the remaining bf-4yjq summaries | Earlier waves — **superseded on crash count/cadence and (pre-Sep-06) on mechanism**; retained for telemetry and prevention-stack history. The 2026-08-26/09-01 summaries and `docs/crash-investigation-bf-4yjq-summary-2026-08-26.md` still carry the 9-crash figure (refresh owned by domchk-7625a5cc) |

---

*Consolidated for dispatch domchk-4ed0544b, 2026-09-06. Live verification commands
recorded in §5.2 were executed for this report on the working repo (`.git` 94 MB,
90.43 MiB single pack, 0 garbage) — not quoted from an earlier record.*
