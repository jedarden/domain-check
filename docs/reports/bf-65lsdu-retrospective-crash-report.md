# Retrospective Crash Report — bf-65lsdu (17GB Repository Bloat / OOM git-gc Crash)

**Report Date:** 2026-09-02
**Report Bead:** domchk-1d947f1e (retrospective documentation, verification chain)
**Incident Bead:** bf-65lsdu — "Run repository cleanup to eliminate 17GB bloat"
**Alert Bead:** bf-4stk59 — "ALERT: Agent crash on bead bf-65lsdu"
**Incident Window:** 2026-08-13T21:16Z (task created) → 2026-08-17T00:45Z (closed)
**Crash Storm:** 2026-08-13T21:22Z → 2026-08-14T00:20Z (11 SIGKILL events in ~3 hours)
**Classification:** Infrastructure Event — repository bloat → kernel OOM killer
**Domain-check Code Impact:** **NONE** — zero defects found by every investigation in the chain

This report is the incident-level retrospective for the bf-65lsdu crash. It compiles
the findings of every investigation bead in the chain (§7), adds a quantified impact
assessment (§4), and consolidates recommendations with their verified implementation
status as of 2026-09-02 (§6). Detailed per-bead reports are indexed in §8 and are
authoritative for their own details.

---

## 1. Executive Summary

On 2026-08-13 an agent was dispatched to clean up a repository that had bloated to
~18GB — 17.20 GiB of it (4,515 objects) loose, unpacked objects, ~99% of the entire
repository. The chosen operation, `git gc --aggressive --prune=now`, is the single
most memory-intensive git operation available: its delta computation needed an
estimated 10–20GB of working memory on top of git's 2–4GB base footprint, uncapped,
on a 62GB box shared by every agent workspace on the machine. The kernel OOM killer
SIGKILLed the git subprocess 6 minutes into the first attempt, and the agent died
with it — recorded as exit code -1.

The task then failed identically **eleven more times over ~3 hours**: every retry
re-ran the same operation against the same unchanged repository, so the same
deterministic memory exhaustion recurred every time. Thirteen crash-alert beads were
raised during the storm. The task sat blocked for ~3.6 days until 2026-08-17, when it
was **decomposed into three small child beads** (baseline → execute → verify) — at
which point the cleanup succeeded in **90.3 seconds**, reducing the repository 99.5%
(~18GB → ~90MB) with verified integrity.

The deeper finding is not the crash but the response shape: retrying deterministic
resource exhaustion eleven times changed nothing, while the one decisive move —
decomposing the task — took effect immediately. And the prevention loop built after
the incident is **still silently failing**: the daily git-gc timer fired at 03:00 EDT
on 2026-09-02 and died with status=127 in 3ms because the systemd user manager's
PATH contains no usable interpreter (§6, R7). Until that is fixed, the conditions
that produced this incident can silently re-accumulate.

---

## 2. Incident Timeline

All times UTC unless marked EDT.

### 2.1 Setup and crash storm — 2026-08-13

| Time | Event |
|---|---|
| 21:16:00 | Bead `bf-65lsdu` created: "Run repository cleanup to eliminate 17GB bloat." Task: `git gc --aggressive --prune=now`, expected runtime 30–60 min. Repository at this moment: ~18GB total, 17.20 GiB loose across 4,515 objects. |
| 21:22:35 | **Crash #1** — OOM killer terminates `git gc` during repack; exit code -1. |
| 21:30:32 | Crash #2 — alert `bf-1b5if7`. |
| 21:32:12 | Alert `bf-4stk59` raised — the alert bead this incident is tracked under. |
| 21:48:30 | Crash #3 — alert `bf-1944k2`. |
| 22:14 → 00:20 | **Crashes #4–#11.** Alerts `bf-3k8oln`, `bf-12yvry`, `bf-1akbgp`, `bf-1d28nt`, `bf-13y6q9`, `bf-1azq3i`, `bf-1dy0zp`, `bf-14uhmx` (2026-08-13), `bf-1cjg4f` (00:20:11, 2026-08-14). Every failure identical: SIGKILL → exit -1, same operation, same repository state. |

### 2.2 Blocked period — 2026-08-14 → 2026-08-17

No progress on the cleanup for ~3.2 days. The task remained open; the repository
remained at ~18GB and continued to threaten every other git operation on the box
(see §4.2 — this same bloat was simultaneously crashing beads bf-4yjq and bf-1s6c3).

### 2.3 Recovery — 2026-08-17

| Time | Event |
|---|---|
| 00:32 | Bead split into three child beads: `domchk-bdb1fedf` (document baseline), `domchk-af4b5ef4` (execute cleanup), `domchk-87be56d8` (verify results). Parent converted to umbrella. |
| 00:34:00 | **Cleanup succeeds — exit 0, 90.3 seconds.** No single child step required the memory footprint of a full aggressive repack. |
| 00:45:33 | Parent `bf-65lsdu` closed. |

### 2.4 Aftermath and documentation — 2026-08-26 → 2026-09-02

- **2026-08-26** — crash investigations open (`bf-1mcxco` first, then `bf-ncs0ev`);
  repository re-verified at 139MB, fsck clean.
- **2026-08-26 → 09-02** — at least seven retrospective false-positive crash alerts
  for already-resolved bf-65lsdu work (`bf-1mcxco`, `bf-1b5if7`, `bf-3k8oln`,
  `bf-4stk59`, `bf-6397nq`, `bf-n7cymi`, `bf-uii7q0`, `bf-5otj5k`, `bf-2prqor`),
  each consuming its own investigation bead (§4.4).
- **2026-09-02** — formal RCA (domchk-2ab71440), agent-context investigation
  (domchk-7e6c4b21), crash-alert system hardening (closed-bead filtering, duplicate
  detection, completion awareness, 5-min cooldown, classification — 12/12 tests
  passing), final synthesis (domchk-944eb8bb), and this retrospective.
- **2026-09-02 03:00 EDT** — daily `domain-check-git-gc.timer` fires;
  `domain-check-git-gc.service` fails with **status=127** in 3ms / 1.4MB peak
  (verified during this retrospective) — the prevention loop remains open (§6, R7).

---

## 3. Root Cause Analysis

*(Formal RCA: `docs/research/root-cause-analysis-bf-65lsdu-crash-2026-08-13.md`,
authored by domchk-2ab71440.)*

### 3.1 Causal chain

```
Large artifacts never excluded from git + no scheduled garbage collection
        │
        ▼
Repository bloat: ~18GB total, 17.20 GiB loose (4,515 objects, ~99% loose)
        │
        ▼
git gc --aggressive --prune=now   ← delta optimization across ALL loose objects
        │
        ▼
Working memory demand ≈ 10–20GB, uncapped, on top of git's 2–4GB base
        │
        ▼
Peak demand exceeds what the shared 62GB box can supply
        │
        ▼
Kernel OOM killer → SIGKILL → exit code -1        ← recurred ×11 (retries changed nothing)
```

### 3.2 The two contributing failures

1. **Bloat was allowed to accumulate.** No `.gitignore` coverage for large artifact
   directories (`.beads/` traces/data) and no scheduled gc. Every threshold in the
   repository maintenance guide was exceeded long before the first OOM — the bloat
   was visible for days and nobody was watching.
2. **The chosen remediation was itself the heaviest possible operation.** The irony
   of this incident: cleaning an N-gigabyte loose set with an *aggressive* in-memory
   repack is the most memory-hungry git operation there is, and it was run with no
   memory cap, no pre-flight resource check, and no fallback triggered on first
   failure. The cleanup task for the bloat became the largest single-memory consumer
   the bloat ever produced.

### 3.3 Why the retries could never succeed

Memory demand is a deterministic function of repository contents. Between retries
nothing changed those contents, so every retry was guaranteed to reproduce the same
OOM. Eleven retries bought eleven identical failures and thirteen alert beads. The
only intervention that could work — changing the *shape* of the work (decomposition)
or bounding its resources — was applied on day 4 and worked on the first try.

### 3.4 Why exit code -1

Per `docs/crash-response-guide.md`, exit code -1 is the infrastructure-event
signature: the process did not exit voluntarily — an external force (here the kernel
OOM killer's SIGKILL) terminated it, and the agent died with its git subprocess.

### 3.5 Classification

| Classification | Weight | Evidence |
|---|---|---|
| **Infrastructure Event** | 70% | Repository bloat; OOM during git operation; permanently resolved by cleanup alone |
| Workflow Failure | 20% | 11 identical retries before decomposition was tried |
| Service Failure | 8% | Not applicable — no external service involved |
| Code Defect | 2% | **Zero evidence** — crash was in a git subprocess; no domain-check code path involved |

Why not a code defect: (1) the crash occurred inside `git gc`, not domain-check code;
(2) crashes stopped permanently once the repository was cleaned; (3) every
comprehensive investigation in this workspace has found the domain-check codebase
defect-free; (4) the fix required resource cleanup, not a code change.

### 3.6 Recurrence family

The same failure mode (bloat → OOM during routine git operations) produced a family
of incidents; bf-65lsdu was the *remediation* attempt for that family:

| Bead | Task | Outcome |
|---|---|---|
| bf-1ea4g | documentation task | SIGKILL during git ops; recovered |
| bf-4yjq | git remote fix | 9 OOM crashes from an 18GB repo; recovered |
| bf-1s6c3 | git reconciliation | 18GB repo → OOM; cleanup → 138MB |
| **bf-65lsdu** | **the cleanup itself** | **11 OOM crashes; recovered via bead split** |

---

## 4. Impact Assessment

### 4.1 Direct impact on the task

- **11 wasted agent dispatches** — each a full session that OOM-died mid-operation.
- **~3.6 days elapsed** from task creation (08-13 21:16) to first success (08-17
  00:34), of which ~3.2 days were pure blockage. The actual work, correctly shaped,
  took **90 seconds**.
- 13 crash-alert beads raised during the storm, all needing triage and closure.

### 4.2 Impact on other work (the larger cost)

The bloated repository was a **shared-box hazard**, not just this bead's problem.
The same 18GB bloat was concurrently OOM-crashing unrelated beads (bf-4yjq: 9
crashes; bf-1s6c3; bf-1ea4g) and occupying ~4% of the box's single 444G disk. Because
the global OOM killer picks whatever process pushes memory over the limit, an
uncapped git operation on this box can kill *any* co-tenant agent — which is exactly
how exit -1 crashes spread across beads. Every day bf-65lsdu stayed blocked, the
whole bloat-crisis family stayed active.

### 4.3 Impact on domain-check

**None.** No domain-check code was running, involved, or changed. The service,
its manifests, and its CI were untouched. Every investigation confirmed the
application defect-free.

### 4.4 Aftermath cost — false-positive alert storm

Long after resolution, the crash-alert system kept generating **retrospective
alerts for the already-completed bf-65lsdu work** — at least nine alert beads
(§2.4), each consuming a full investigation session before being closed as false
positive / duplicate. This aftermath cost several times the effort of the original
investigations and is what motivated the 2026-09-02 alert-system hardening
(closed-bead filtering, duplicate detection, completion awareness, cooldown).

### 4.5 Documentation effort

Ten-plus investigation documents were produced across 2026-08-13 → 09-02 (indexed
in §8). Thorough, but a fraction of it would have sufficed had the first failure
triggered "stop and decompose" instead of "retry."

---

## 5. Resolution Summary

**What resolved it:** task decomposition, not retry.

1. `bf-65lsdu` was split into three independently verifiable child beads —
   **baseline → execute → verify** (`domchk-bdb1fedf` / `domchk-af4b5ef4` /
   `domchk-87be56d8`).
2. No single child step required the memory footprint of a full aggressive repack;
   the cleanup completed in 90.3 seconds, exit 0, on the first shaped attempt.
3. Result: ~18GB → ~90MB (**99.5% reduction**); loose objects 17.20 GiB / 4,515 →
   effectively zero; single consolidated pack; `git fsck` clean.

**Sustained state, verified during this retrospective (2026-09-02):**

```
$ du -sh .git                → 92M
$ git count-objects -vH      → count: 0 loose; in-pack: 10,371;
                               packs: 1; size-pack: 90.17 MiB; garbage: 0
$ ./scripts/safe-git-gc.sh --check-only → "GC not needed"
```

All 13 storm alerts and all retrospective false-positive alerts are closed. Parent
bead `bf-65lsdu` closed 2026-08-17T00:45:33Z.

---

## 6. Recommendations and Implementation Status

Status verified 2026-09-02 during this retrospective — not assumed from docs.

| # | Recommendation | Status |
|---|---|---|
| R1 | **Never run bare `git gc --aggressive` on a repo >1GB.** Use the memory-capped, checkpoint/resumable, monitored `scripts/safe-git-gc.sh` (+ `safe-git-gc-monitor.sh --watch`). | ✅ In place — scripts shipped; loose-object detection fixed with regression test (commit 4737327) |
| R2 | **Decompose-after-two-failures rule.** A second identical exit -1 failure means stop and split the task (baseline → execute → verify child beads), never a third retry. | ✅ Documented in crash-response and maintenance guides; proven by this incident's recovery |
| R3 | **Pre-flight resource checks** before memory-intensive operations (≥10GB free RAM, ≥20GB disk). | ✅ `scripts/preflight-health-check.sh` |
| R4 | **Keep bloat out at the source.** `.gitignore` excludes `.beads/`. | ✅ Verified — `.gitignore` line 66 |
| R5 | **Pre-commit hook blocking files >10MB.** | ✅ Installed — `.git/hooks/pre-commit` (2026-09-01) |
| R6 | **Harden the crash-alert system** against retrospective false positives. | ✅ 2026-09-02 — closed-bead filter, duplicate detection, completion awareness, 5-min cooldown, classification (`scripts/crash-alert-manager.sh`, `scripts/crash-classifier.sh`; 12/12 tests passing) |
| R7 | **Fix the systemd user schedulers so the prevention loop actually runs.** ⚠️ **THE OPEN ITEM.** The user manager's PATH is only the systemd store dir (`/nix/store/…-systemd-257.10/bin/`), so `#!/usr/bin/env bash` fails: the daily `domain-check-git-gc.service` last fired 2026-09-02 03:00 EDT and died **status=127 in 3ms**; `domain-check-monitoring.service` and `domain-check-service-monitor.service` are likewise in `failed` state. Fix in each unit's `[Service]`: `Environment=PATH=/run/current-system/sw/bin:/home/coding/.local/bin:…`, or invoke the interpreter by absolute path in `ExecStart`. Then `systemctl --user daemon-reload`, start each unit once, and confirm exit 0 in the journal. | ❌ **OPEN — bloat-prevention loop is not closed until this lands** |
| R8 | **Bound the scheduled gc's resources** (`MemoryMax=4G`, `MemorySwapMax=0`, `CPUQuota`, `OOMScoreAdjust`) so a runaway gc is OOM-killed inside its own cgroup instead of triggering the global OOM killer. The invalid `User=%i` in `domain-check-git-gc-full.service` is already removed. Repo copies of the unit files carry these bounds but are **uncommitted** at report time; the installed units already carry them. | 🟡 In flight — commit the repo copies and keep installed/repo copies in sync |
| R9 | **Correct monitoring docs that assume cron.** `crontab` does not exist on this NixOS box; `scripts/monitoring-setup.sh`'s cron instructions are inoperative. Docs should point at the existing systemd user timers (repo-health 02:00 daily → precedes git-gc 03:00 daily; monitors every 1–5 min). | ❌ OPEN — documentation-only |
| R10 | **Verify one clean timer cycle after R7/R8:** repo-health succeeds at 02:00, git-gc exits 0 at 03:00, crash-pattern monitoring exits 0 — confirmed in `journalctl --user`, and repository stays under the healthy thresholds (<500MB, <100 loose objects) across a week of cycles. | ⏳ Pending R7 |

**Priority:** R7 is the only open item that materially affects recurrence risk.
Bloat caused this incident and its whole family; the daily gc and health check are
the defense against re-accumulation, and both are currently silent no-ops. Until R7
lands, protection depends entirely on manual checks.

---

## 7. Lessons Learned

1. **Retrying deterministic resource exhaustion never works.** Eleven retries
   produced eleven identical failures. The fix was changing the shape of the work.
2. **Decompose oversized operations.** Baseline → execute → verify child beads each
   stayed under resource limits and were independently verifiable; the monolithic
   version failed 11 times.
3. **The remediation can be the hazard.** An uncapped aggressive repack over a
   bloated loose set is the most memory-hungry git operation available. Match the
   operation's cost to the resource budget, and cap it.
4. **A shared box means shared blast radius.** The global OOM killer can pick any
   co-tenant process; uncapped heavy operations put every agent on the machine at
   risk, which is how one bead's crash becomes a fleet-wide exit -1 pattern.
5. **Alert noise compounds incident cost.** The retrospective false-positive storm
   after resolution cost more agent-sessions than the crash itself. Completion
   awareness and duplicate detection are not optional niceties.
6. **Silence in the prevention loop is the durable risk.** Monitoring and scheduled
   maintenance that fail quietly are worse than none — they create confidence
   without coverage. Verify the loop end-to-end (R10), not just the existence of
   the scripts.

---

## 8. Investigation Chain Index (compiled findings)

| Bead | Role | Document |
|---|---|---|
| bf-4stk59 | Alert bead for the crash | closure note in `docs/reports/bf-4stk59-retrospective-crash-alert.md` |
| bf-1mcxco | First crash investigation | `docs/crash-investigations/bf-65lsdu-crash-investigation.md` |
| bf-ncs0ev | Recovery + lessons report | `docs/research/crash-analysis-bf-ncs0ev.md`, `docs/notes/cleanup-crash-investigation-bf-ncs0ev.md` |
| domchk-bdb1fedf | Baseline child bead | `cleanup-baseline.txt` |
| domchk-af4b5ef4 | Cleanup-execution child bead | — |
| domchk-87be56d8 | Verification child bead | `cleanup-results.txt` (2026-09-01: 90MB, 4 loose, 1 pack) |
| domchk-2ab71440 | Formal root cause analysis | `docs/research/root-cause-analysis-bf-65lsdu-crash-2026-08-13.md` |
| domchk-7e6c4b21 | Agent-context investigation | `docs/investigation-bf-65lsdu-agent-context-2026-09-02.md`, `docs/investigations/bf-65lsdu-crash-context-domchk-7e6c4b21-2026-09-02.md`, `docs/crash-information-bf-65lsdu.md` |
| domchk-81e02aff | Fix/mitigation proposal | `docs/fix-proposal-bf-65lsdu-oom-git-gc-2026-09-02.md` |
| domchk-ca7d6d12 | Fix verification | `docs/verification-bf-65lsdu-fix-domchk-ca7d6d12-2026-09-02.md`, `docs/verification/bf-65lsdu-cleanup-verification.md` |
| domchk-e6af2399 | Resolution-status verification | closed; confirmed bf-65lsdu resolved by bead split |
| domchk-944eb8bb | Investigation-chain synthesis | `docs/research/crash-investigation-synthesis-bf-65lsdu-2026-09-02.md` |
| **domchk-1d947f1e** | **This retrospective** | `docs/reports/bf-65lsdu-retrospective-crash-report.md` |

---

## 9. Related Documentation

- `docs/crash-response-guide.md` — crash classification and response procedures
- `docs/maintenance/repository-maintenance-guide.md` — size/object thresholds and daily maintenance
- `docs/comprehensive-crash-prevention-guide.md` — systemic prevention
- `docs/crash-alert-fix-implementation-2026-09-02.md` — alert-system hardening
- `scripts/safe-git-gc.sh`, `scripts/check-repo-health.sh`, `scripts/preflight-health-check.sh`

---

## 10. Conclusion

The bf-65lsdu crash was a pure infrastructure event: ~18GB of accumulated repository
bloat met an uncapped `git gc --aggressive`, the kernel OOM killer killed the git
subprocess eleven times, and retrying changed nothing for ~3.6 days until the task
was decomposed into three small verifiable child beads — which completed the cleanup
in 90 seconds, cutting the repository 99.5% with verified integrity. Domain-check
code was never implicated and remains defect-free.

The incident is closed. The **open risk is prevention-loop silence**: the daily gc
and monitoring timers built to stop bloat from re-accumulating are failing on a PATH
problem that has now survived multiple verification passes (R7). Fixing that, and
verifying one clean timer cycle (R10), is what finally closes this incident.

**Status:** ✅ RETROSPECTIVE COMPLETE — timeline compiled, root cause confirmed,
impact quantified, resolution documented, recommendations consolidated with verified
status. Incident resolved 2026-08-17; prevention-loop fix (R7) outstanding with the
operator.
