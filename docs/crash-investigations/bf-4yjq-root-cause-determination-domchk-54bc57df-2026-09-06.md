# bf-4yjq — Root Cause Determination and Contributing Factors

**Dispatch bead:** domchk-54bc57df — "Investigate root cause and contributing factors"
**Subject bead:** bf-4yjq (closed 2026-08-17) — the 2026-08-12 crash storm window
**Analysis date:** 2026-09-06
**Position in chain:** domchk-b6a900e4 (artifacts) → domchk-b9513e0b (classification, commit
`8884670`) → **this record (root cause)** → domchk-7c9a4311 (crash report)

**Scope of this record.** The causal chain is established by the canonical
[`bf-4yjq-crash-investigation.md`](bf-4yjq-crash-investigation.md) §6 and is not re-derived
here. What this record renders is the dispatch's three deliverables as a standalone document:

1. **Root cause analysis** — the corrected mechanism, with confidence levels per step
2. **Contributing factors list** — primary vs secondary, each with mitigation status
3. **Evidence chain supporting the conclusion** — every step sourced, with the live
   re-verification performed by this dispatch marked **[LIVE 2026-09-06]**

It also discharges the dispatch's four acceptance criteria directly (§3), including the
SIGHUP question, which no committed bf-4yjq document had answered with primary evidence
before (§3.2) — and which resolves **against** the archived SIGHUP-cascade framing.

An earlier document,
[`docs/crash-root-cause-bf-4yjq.md`](../crash-root-cause-bf-4yjq.md) (2026-08-17), carries
this same deliverable's title but predates the corrected mechanism; it is now banner-linked
here and its superseded claims are enumerated in §6.

---

## 1. Root cause

**Statement:** bf-4yjq's 50 deaths were the resource-exhaustion death of the era: the
repository's object store was bloated to ~18 GB (17.2 GiB loose vs 9.6 MiB packed, ≈1,800:1
inverted), every substantive git operation on that store pulled a working set larger than the
dispatch scope could hold, and the kernel's memory-cgroup OOM killer ended the process. The
bead's retry loop then re-dispatched into the same condition ~every 3 minutes for 2.6 hours.
The task content (git remote reconciliation) was irrelevant; crash exposure was a function of
scheduling inside the bloat window.

**Mechanism (corrected framing).** The constraint was the **dispatch scope's cgroup limit,
not host memory**. The kernel call trace recovered for later members of the same death class
runs `mem_cgroup_out_of_memory → try_charge_memcg → do_anonymous_page`: git faulted in a page,
the cgroup charge failed at the scope's limit, and the OOM killer fired *within the cgroup* —
an uncatchable SIGKILL, hence no core dump and needle's sentinel `exit_code=-1`. A live
dispatch scope today holds `MemoryMax=12884901888` = **exactly 12 GiB** **[LIVE 2026-09-06]**.

**Confidence** (inherited from canonical §6, restated so this document is self-contained):

| Step | Claim | Confidence |
|------|-------|------------|
| R1 | Bloat-source commits (`bf-2ildm`-era `.beads/` JSONL) inflated the repo | **HIGH** — contemporaneous `git count-objects` figures recorded on the era's own summary |
| R2 | Those figures (4,594 objects / 17.20 GiB loose / 9.60 MiB packed) are accurate for the window | **HIGH** — recorded contemporaneously, consistent across independent documents |
| R3 | Substantive git operations on that store OOM-killed agents | **MEDIUM-HIGH** — inferred; Aug-12 kernel logs do not survive (§5) |
| R4 | The kill was memcg-constrained (scope limit), not host OOM | **MEDIUM-HIGH for Aug-12 itself** (by family inference); **PROVEN by kernel record** for the same death class on Aug-14/16 (bf-4x12ec, bf-198ne) and re-confirmed live in this dispatch (§3.1) |
| R5 | Crashes stopped when the trigger was removed (repo packed Aug-13/14) and never returned | **HIGH** — zero `exit_code=-1` records anywhere after 2026-08-17 across continuous log-slot coverage **[LIVE 2026-09-06, per predecessor's recount; unchanged here]** |

The gap between "HIGH on correlation" and "MEDIUM-HIGH on the OOM step" is entirely an
evidence-retention artifact (§5), not a competing hypothesis: no alternative mechanism has
survived any test applied to it, and §3.2 closes the last one that had been published.

## 2. Contributing factors

### Primary — causal; each is a condition without which the deaths do not occur as they did

| # | Factor | Role | Status 2026-09-06 |
|---|--------|------|--------------------|
| P1 | **Bloated object store** — `bf-2ildm`-era commits of ~237 MB `.beads/` JSONL → 18 GB repo, 17.2 GiB loose / 9.6 MiB packed | Supplied the oversized working set. The trigger. | **Mitigated.** Repo packed Aug-13/14; `.beads/` gitignored; holding at ~97 MB **[LIVE 2026-09-06]** |
| P2 | **Unbounded git operations in the dispatch scope** — any substantive operation loaded the loose store | Converted the bloated store into a multi-GB working set | **Partially mitigated.** `pack.windowMemory=2g` / `deltaCacheSize=1g` / `threads=1` bound the bare-gc and push paths (verified ≈312 MiB peak on the crash command); arbitrary ad-hoc git invocations remain unbounded by config |
| P3 | **Dispatch scope `MemoryMax` = 12 GiB** — the binding constraint | Turned "slow" into "killed". Live scope verified at exactly 12 GiB **[LIVE 2026-09-06]** | **Standing.** This is infrastructure, not a defect; it is what makes the P1×P2 combination lethal rather than merely slow |
| P4 | **Retry loop without environment re-check** — re-dispatch ~every 3 min into an unchanged condition | Turned one lethal condition into 50 deaths over 2.6 h. Median 156 s between deaths **[LIVE re-derivation]** | **Open.** Nothing gates a re-dispatch on the condition that killed the last one |

### Secondary — aggravating, detectability, and evidence factors

| # | Factor | Role | Status 2026-09-06 |
|---|--------|------|--------------------|
| S1 | **No storm-level detection** — per-bead crash thresholds; bf-4yjq peaked at **5 deaths in any 10-minute window** **[LIVE re-derivation]**, under the guide's ≥10/10-min surge line for its entire 2h37m | The regime was invisible per-bead while it consumed the workspace (455 deaths across 6 beads that day) | **Open.** `scripts/crash-pattern-detection.sh` still keys per-bead and currently reports **DEGRADED** (its event source `.beads/events.jsonl` newest record 2026-08-26 — predecessor's finding, not re-owned here) |
| S2 | **No `.gitignore` exclusion of `.beads/` at the time** | Enabled the bloat source (P1) | **Mitigated.** Whole directory ignored; 0 tracked files |
| S3 | **No pre-commit large-file gate at the time** | Nothing blocked a 237 MB commit | **Mitigated.** 10 MB hook installed in this clone (per-clone; no installer) |
| S4 | **Evidence-retention gap** — no kernel logs before Aug-15 19:46 EDT, no coredumps before Aug-25, single-slot traces, no surviving Aug-12 heartbeats | Held R3/R4 at MEDIUM-HIGH instead of proven | **Partially mitigated.** Resource-monitor + service timers now run; kernel OOM events recoverable via `journalctl -k` going forward. No archive-with-timestamps policy yet (canonical §9 rec 3) |
| S5 | **Alert-hygiene debt** — 50 bf-4yjq alert beads, hundreds workspace-wide, still open | Distorts counting; generates duplicate investigations | **Open.** Canonical §9 rec 2 (bulk-close) not yet executed |
| S6 | **Alert-sampled histories undercount** — era docs recorded 9 crashes vs the verified 50 | Made the event look like scattered flakiness rather than a deterministic loop | **Documented.** Forensic checkpoint established as source of truth (canonical §3) |

## 3. Acceptance criteria — discharged

### 3.1 System resources: memory pressure and OOM events **[LIVE 2026-09-06]**

Current host — nothing for a resource-exhaustion mechanism to act on:

| Check | Result |
|-------|--------|
| `free -h` | 47 Gi available of 62 Gi (no pressure) |
| `uptime` | load 2.21 / 1.35 / 0.87 on 12 cores |
| `df -BG /` | 61 GB free |
| `git count-objects -vH` | 52 loose / 3.78 MiB; 10,980 in-pack / 90.93 MiB |
| `du -sh .git` | 97 MB (healthy limit 500 MB) |
| `git fsck --connectivity-only` | clean (exit 0; two dangling trees, normal churn) |

Historical OOM events, from the kernel journal — which **begins 2026-08-15 19:51:30 EDT**
(re-verified live; the Aug-12 window has no kernel record, see §5):

- **894 `oom-kill` lines** in the recoverable window; **447 constraint records, 100%
  `CONSTRAINT_MEMCG`** — zero system-wide (`CONSTRAINT_NONE`) kills. Host-level OOM does not
  occur on this host; every recorded kill is a cgroup-limit kill.
- Victim breakdown: **`task=git` 269**, `node (vitest)` 157, `bash` 20, `python3` 1 — git is
  the dominant OOM victim on this box, inside per-dispatch `run-p*.scope` cgroups.
- Earliest record (2026-08-16 00:27:35 EDT) traced through
  `mem_cgroup_out_of_memory → try_charge_memcg → do_anonymous_page` with `Comm: git` — the
  exact mechanism R4 asserts, kernel-stated.
- Era contemporaneous telemetry (canonical §6; `.beads/crash-bf-4yjq-summary.txt`, cited for
  object-store metrics only): load 15–17 on 12 cores, memory effectively exhausted *during git
  operations*, disk 84% full, `git fsck --no-full` timing out — i.e. pressure that appeared
  **when a git operation ran**, not a host that was idle-starved.

### 3.2 SIGHUP and other signals — ruled out, with the last published alternative hypothesis falsified

Three independent lines, two of them primary evidence:

1. **Primary telemetry contains no SIGHUP.** The preserved worker-log extract for the window
   (225 records) was re-scanned in this dispatch: **50 death records, zero mentions of
   SIGHUP or any signal attribution**. Every record is `outcome=Crash(-1)` with the signal
   unrecorded **[LIVE re-scan]**.
2. **Exit-code semantics.** `-1` is needle's sentinel for *a process death whose signal was
   not recorded* — it is not a signal number. In shell convention a SIGHUP death surfaces as
   exit **129** (128+1), never -1. The archived framing "exit code -1 (SIGHUP)" was a
   misreading of the sentinel.
3. **The SIGHUP-cascade hypothesis fails where it is testable.** The archived Aug-26 framing
   (`docs/archive/crash-investigations/research/crash-incident-summary-domain-check-2026-08-26.md`)
   attributed a "system-wide SIGHUP cascade" to **2026-08-16 12:00–17:00 UTC, 200+ crashes,
   'Not Related To: Git gc, resource exhaustion'**. The kernel journal covers that window and
   says otherwise: **213 kernel-recorded `CONSTRAINT_MEMCG` kills of `task=git` fall inside
   08:00–13:00 EDT (= 12:00–17:00 UTC) on Aug-16 alone** (hourly: 30, 51, 35, 24, 48, 25)
   **[LIVE 2026-09-06]**. The very window the SIGHUP theory was built on is the push-side
   memcg-OOM variant (bf-198ne, kernel-verified). The SIGHUP attribution was constructed
   before kernel logs were known to be recoverable; where both speak, the kernel wins.

**Conclusion: no SIGHUP involvement.** The signal class of this crash is the uncatchable
**SIGKILL** delivered by the memory-cgroup OOM killer — consistent with the absence of core
dumps, stack traces, and graceful shutdown across all 50 events.

### 3.3 Workflow execution logs **[LIVE re-derivation]**

Independently re-derived from
[`docs/crash-analysis/bf-4yjq-needle-worker-log-extract.log`](../crash-analysis/bf-4yjq-needle-worker-log-extract.log)
(225 records, the preserved primary telemetry):

| Measure | This dispatch | Canonical/predecessor | Agreement |
|---------|--------------|----------------------|-----------|
| Death records | 50 (of 225 lines; 175 non-death) | 50 | ✅ third independent count |
| Window | 17:53:53.875 → 20:30:38.310 UTC (9,404 s ≈ 2.61 h) | 17:54:00 → 20:30:43 | ✅ |
| Inter-death gap | mean 192 s, median 156 s, min 76 s, max 577 s | mean 188 s (~3.1 min); per-run survival median ~149 s | ✅ (gap vs survival measure the same regime) |
| Exit-code variation | 0 across 50/50 | 0 | ✅ deterministic loop |
| Peak deaths / 10 min | **5** | 5 | ✅ — below the ≥10 surge threshold all window |

Every agent died **mid-task** on re-dispatch into an unchanged environment: a deterministic
retry-kill loop, not scattered flakiness, and — per S1 — a storm no per-bead threshold would
have flagged while it ran.

## 4. Evidence chain (numbered; each step independently checkable)

| Step | Evidence | Type | Confidence |
|------|----------|------|------------|
| E1 | Era `git count-objects`: 4,594 objects / 17.20 GiB loose / 9.60 MiB packed (`.beads/crash-bf-4yjq-summary.txt`) | contemporaneous record | HIGH |
| E2 | 50/50 `exit_code=-1` in the preserved extract; zero signal attribution | primary telemetry (re-scanned live) | HIGH |
| E3 | Cadence: mean gap 192 s, 2.61 h continuous, then stops when repo was packed | primary telemetry (re-derived live) | HIGH |
| E4 | Storm context: 455 exit-1 events across 6 beads on Aug-12 (canonical §4, re-derived by predecessor from raw log slots) | raw logs | HIGH |
| E5 | Kernel journal starts 2026-08-15 19:51 EDT — Aug-12 window has no kernel record | confirmed absence (live) | HIGH (that the record is absent) |
| E6 | 447/447 kernel kills in the recoverable window are `CONSTRAINT_MEMCG`; 269 victims are `task=git`; trace `mem_cgroup_out_of_memory → try_charge_memcg → do_anonymous_page` | kernel record (live) | HIGH — proves the death class, Aug-15 onward |
| E7 | Live dispatch scope `MemoryMax=12884901888` = 12 GiB | live config | HIGH — the binding constraint exists today as it did in the era |
| E8 | bf-4x12ec (Aug-14, gc) and bf-198ne (Aug-16, push) kernel-proven memcg kills of git in this workspace | kernel-verified sibling reports | HIGH — family inference for Aug-12's mechanism |
| E9 | Zero `exit_code=-1` after 2026-08-17 across continuous slot coverage; repo ~97 MB and fsck-clean today | live re-verification | HIGH — trigger absent, class extinct |
| E10 | SIGHUP falsification: 213 memcg git-kills inside the window the archived doc called a "SIGHUP cascade" | kernel record (live) | HIGH — alternative hypothesis rejected |

**Bounding statement.** For Aug-12 itself, steps E1–E5 and E8–E9 are as far as evidence can
go; the raw kill record for that specific window does not exist and cannot come to exist
(§5). The conclusion does not rest on filling that gap — it rests on the trigger's removal
ending the deaths exactly (E9), and on every later, better-instrumented occurrence of the
same death class resolving to the same mechanism (E6, E8, E10).

## 5. Confirmed absences (what new evidence cannot exist)

Re-verified or inherited from the preservation pass
(`docs/crash-analysis/bf-4yjq-artifact-preservation-2026-09-06.md`):

- No kernel log for the Aug-12 window — system journal begins 2026-08-15 19:46–19:51 EDT
  **[re-verified live this dispatch]**
- No coredumps before Aug-25 (SIGKILL prevents them; none retained)
- No Aug-12 heartbeats; `.beads/traces/` holds only later alert-bead re-runs
- Crash-era git commits excised from the DAG (Aug-16 squash commit)
- `scripts/crash-classifier.sh bf-4yjq` cannot classify this event — trace capture did not
  exist on Aug-12 (predecessor's live check, unchanged)

## 6. Superseded claims in the prior RCA

[`docs/crash-root-cause-bf-4yjq.md`](../crash-root-cause-bf-4yjq.md) (2026-08-17) is now
banner-linked here. Its superseded claims, for any future reader who reaches it directly:

| It says | Corrected |
|---------|-----------|
| "Signal -1 maps to SIGKILL", "SIGKILL definitively identified" | `-1` is needle's sentinel for an unrecorded signal; the SIGKILL attribution for Aug-12 is family-inferred (MEDIUM-HIGH), not definitive from raw logs |
| Linux OOM killer as a system-level event; "<2 GB available during git operations" | Constraint is the **12 GiB dispatch scope cgroup**, not host memory; 100% of kernel records in the recoverable window are `CONSTRAINT_MEMCG`, zero host-level kills |
| 9 crashes at ~17-minute intervals | **50 crashes at ~3.1-minute intervals** (canonical §3) |
| "BLOCKED at crash time, not actively executing" | Unverifiable and inconsistent with the ~3-min re-dispatch kill cadence; the defensible claim is *incidental to task content* |
| Repo cleaned to "753 MB"; bf-2ildm workflow issue and .gitignore "pending" | Verified cleanup is 91–94 MB; `.beads/` gitignored, 10 MB pre-commit hook installed, bounded gc + timers deployed — all landed |
| SIGHUP-era framing elsewhere in the Aug corpus | Ruled out with primary + kernel evidence (§3.2) |

## 7. Sources

| Document | Role |
|----------|------|
| [`bf-4yjq-crash-investigation.md`](bf-4yjq-crash-investigation.md) | Canonical — causal chain (§6), storm table, reproducibility |
| [`bf-4yjq-crash-pattern-analysis-domchk-b9513e0b-2026-09-06.md`](bf-4yjq-crash-pattern-analysis-domchk-b9513e0b-2026-09-06.md) | Predecessor handoff — classification, pattern, family table |
| [`bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md`](bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md) | Four-way taxonomy verdict |
| [`bf-4yjq-consolidated-summary-domchk-ea5c6a63-2026-09-06.md`](bf-4yjq-consolidated-summary-domchk-ea5c6a63-2026-09-06.md) | Reconciled figures |
| [`../crash-analysis/bf-4yjq-needle-worker-log-extract.log`](../crash-analysis/bf-4yjq-needle-worker-log-extract.log) | Primary crash-era telemetry (re-scanned this dispatch) |
| [`../crash-analysis/bf-4yjq-artifact-preservation-2026-09-06.md`](../crash-analysis/bf-4yjq-artifact-preservation-2026-09-06.md) | Provenance, integrity hashes, confirmed absences |
| `docs/investigations/root-cause-determination-domchk-6281555d-2026-09-06.md` | Canonical RCA for the 2026-09-01 corpus — 12 GiB scope-bound analysis this record's mechanism framing follows |
| `docs/crashes/bf-198ne-crash-report.md`, `docs/crash-investigation-bf-4x12ec.md` | Kernel-verified gc-side / push-side family members |
| `.beads/crash-bf-4yjq-summary.txt` | Era object-store metrics only (superseded on crash count) |
| Kernel journal (`journalctl -k`), live `systemd` scope properties, live repo/host state | Evidence steps E5, E6, E7, E9, E10 — all gathered 2026-09-06 |

---

**Status:** ✅ ROOT CAUSE DETERMINED — memcg-OOM death class on the bloated object store;
contributing factors enumerated with mitigation status; evidence chain complete with live
re-verification; SIGHUP alternative ruled out on primary and kernel evidence.
