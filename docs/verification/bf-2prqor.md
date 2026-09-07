# Verification Report: bf-2prqor — Retrospective Crash Alert for Resolved bf-65lsdu

**Date:** 2026-09-02
**Verification Bead:** domchk-e590f9e0 (this report)
**Alert Bead:** bf-2prqor — "ALERT: Agent crash on bead bf-65lsdu"
**Original Crashed Bead:** bf-65lsdu — "Run repository cleanup to eliminate 17GB bloat"
**Alert Created:** 2026-08-13T21:49:51Z | **Alert Closed:** 2026-09-02T10:44:59Z
**Classification:** FALSE POSITIVE — alert fired on a task that ultimately succeeded
**Status:** ✅ VERIFIED RESOLVED — no further action required

---

## 1. Crash Summary

**What:** The agent working bead `bf-65lsdu` was killed by the kernel OOM killer while
running `git gc --aggressive --prune=now` against a repository that had bloated to
~18GB, of which **17.20 GiB (4,515 objects) was loose, unpacked objects**. The
aggressive repack's delta computation required an estimated 10–20GB of working memory
on top of git's 2–4GB base footprint — more than the shared box could supply. The
SIGKILL surfaced to the workflow as **exit code -1 (signal -1)**.

**When:** 2026-08-13, during the 21:22–21:50Z window of a crash storm that ran
2026-08-13T21:22Z → 2026-08-14T00:20Z. Alert bead `bf-2prqor` was raised at
**2026-08-13T21:49:51Z** (its body timestamp matches its creation time); the
bf-1mcxco investigation pinned one crash event in this window at 21:27:56Z. The
differing timestamps across documents are distinct events of the same retry storm,
not contradictions (see the synthesis report, §4.3).

**Which agent:** `claude-code-glm-4.7` (same agent signature across all 13 alert
beads raised by this storm).

**The task being performed:** Pack the 17GB of loose objects that were causing OOM
crashes during all git operations across the workspace — i.e., the cleanup task for
the bloat crisis was itself the single most memory-intensive operation ever attempted
on that bloat, run without a memory cap.

---

## 2. Timeline of Events

| When (UTC) | Event |
|---|---|
| 2026-08-13 21:16:00 | Bead `bf-65lsdu` created: run `git gc --aggressive --prune=now` on the 17GB-loose-object repository |
| 2026-08-13 21:22 → 21:50 | Crash storm begins — repeated OOM-killer SIGKILLs during the aggressive repack; alert beads raised for each retry failure |
| 2026-08-13 21:49:51 | **Alert bead `bf-2prqor` created** for the crash on `bf-65lsdu` (exit code -1) |
| 2026-08-13 21:50 → 2026-08-14 00:20 | Storm continues — 11 distinct failures total, all exit -1, all identical signature; every retry re-ran the same deterministic memory exhaustion |
| 2026-08-16 | A successful `git gc --aggressive` run recorded a single optimized pack (750.53 MiB), documented in `docs/notes/repository-cleanup-2026-08-13.md` |
| 2026-08-17 00:32–00:34 | **Durable resolution:** `bf-65lsdu` split into three child beads — domchk-bdb1fedf (baseline), domchk-af4b5ef4 (execute), domchk-87be56d8 (verify). Final run exit 0 in 90.3s |
| 2026-08-17 00:45:33 | **Bead `bf-65lsdu` closed** — task complete |
| 2026-08-17 00:12 (commit 993d8cd) | Cleanup results recorded: 17GB → 753MB, 9,525 objects in a single 750.53 MiB pack |
| 2026-08-26 | Retrospective verification batch: false-positive determination written for `bf-2prqor` (`docs/verification-report-bf-2prqor-false-positive-retrospective-crash-alert-resolved-bf-65lsdu.md`) alongside sibling alert beads; repository measured 139MB / 273 loose |
| 2026-09-02 07:22 (commit cc194ec) | Final investigation synthesis (domchk-944eb8bb) compiles the whole chain and reconciles documentation discrepancies |
| 2026-09-02 10:44:59 | **Alert bead `bf-2prqor` closed** |
| 2026-09-02 (this report) | Final verification artifact written to `docs/verification/bf-2prqor.md` |

---

## 3. Root Cause Analysis Findings

### 3.1 Causal chain

```
Large artifacts never excluded from git (.beads/ era data) + no scheduled gc
        │
        ▼
Repository bloat: ~18GB total, 17.20 GiB loose across 4,515 objects (~99% loose)
        │
        ▼
git gc --aggressive --prune=now  (delta optimization across ALL loose objects)
        │
        ▼
Working memory demand ≈ 10–20GB on top of git's 2–4GB base, uncapped
        │
        ▼
Peak demand exceeds what the shared 62GB box could supply
        │
        ▼
Kernel OOM killer → SIGKILL → agent recorded as exit code -1   ← ×11 retries
```

### 3.2 Classification

**Infrastructure Event** — per the weighted taxonomy in `docs/crash-response-guide.md`:

| Classification | Weight | Fit here |
|---|---|---|
| Infrastructure Event | 70% | ✅ **This case.** Repository bloat → OOM during git operation |
| Workflow Failure | 20% | Contributing: 11 identical retries before decomposition worked |
| Service Failure | 8% | N/A — no external service dependency |
| Code Defect | 2% | **Zero evidence** — crash was in a git subprocess, not domain-check code |

### 3.3 Why retrying could never work

The memory requirement of an aggressive repack is a deterministic function of the
repository contents. Nothing between retries changed those contents, so all 11
retries failed identically. The fix that worked changed the **approach**, not the
luck: decomposing the task into baseline → execute → verify child beads meant no
single step needed the full memory footprint of an aggressive repack.

### 3.4 Why it was not a domain-check code defect

1. The crash occurred inside `git gc`, not in any domain-check code path.
2. Crashes stopped permanently once the repository was cleaned — a transient,
   environment-shaped signature, not a code bug.
3. Every comprehensive investigation in this workspace has found domain-check
   defect-free; this chain is no exception.
4. Resolution required resource cleanup and task decomposition, not a code change.

---

## 4. False Positive Evidence — the Task Succeeded

The alert's premise — that the crash represented an unrecovered task failure
requiring investigation and remediation — was **false**:

1. **Original bead closed as complete.** `bf-65lsdu` is `Closed` (closed
   2026-08-17T00:45:33Z) after its acceptance criteria were demonstrably met.
2. **Acceptance criteria all satisfied:**
   - [x] Repository size before cleanup documented (~18GB, 17.20 GiB loose / 4,515 objects)
   - [x] Aggressive gc executed successfully (durable run 2026-08-17T00:34:00Z, exit 0)
   - [x] Repository size after cleanup far below the <500MB target
   - [x] Loose objects packed and verified via `git count-objects`
3. **Recovery is independently documented.** `docs/notes/repository-cleanup-2026-08-13.md`
   (committed as 993d8cd, 2026-08-17) records the before/after numbers; the
   verification child bead domchk-87be56d8 recorded the final verified state.
4. **The root cause itself was eliminated, not just the symptom.** The repository
   trended monotonically healthier after the incident: ~18GB → 752MB (08-16) →
   139MB (08-26) → 90–92MB (09-01 onward).

### 4.1 Current repository state (verified 2026-09-02, this report)

```
$ du -sh .git                    → 92M
$ git count-objects -vH
  count: 9              (loose, 80.00 KiB)
  in-pack: 10,349
  packs: 1
  size-pack: 90.14 MiB
  prune-packable: 0
  garbage: 0
$ git fsck --connectivity-only   → clean (exit 0)
$ ./scripts/safe-git-gc.sh --check-only → "GC not needed"
$ grep .beads/ .gitignore        → line 66: .beads/  (excluded)
```

| Metric | Crash time (2026-08-13) | Now (2026-09-02) | Status |
|---|---|---|---|
| Total repository size | ~18GB | 92MB | ✅ healthy (<500MB target) |
| Loose objects | 17.20 GiB / 4,515 | 80 KiB / 9 | ✅ healthy (<100MB / <100 target) |
| Packs | ~0 effective | 1 (90.14 MiB, 10,349 objects) | ✅ consolidated |
| Garbage | — | 0 | ✅ |
| Integrity | — | fsck clean | ✅ |

### 4.2 Documentation discrepancy noted (resolved)

Earlier reports in this chain cited a "completion commit `5bf23b7`" (2026-08-16).
That hash is **not reachable in current history** — most likely dropped in later
history reconciliation. Per synthesis §4.2, the authoritative recovery record is the
bead trail (`bf-65lsdu` split + child beads domchk-bdb1fedf / af4b5ef4 / 87be56d8 +
traces), which is internally consistent. This report therefore cites the verified
commit `993d8cd` (cleanup results documentation) and the bead record rather than the
unreachable hash.

---

## 5. Lessons Learned for Future Crash Alerts

1. **Check whether the target bead is already closed before raising or acting on a
   crash alert.** `bf-2prqor` fired on 2026-08-13 but was only processed
   retrospectively — by then `bf-65lsdu` had been closed for over two weeks. This
   exact gap is now fixed: the crash alert system (2026-09-02 hardening,
   `scripts/crash-alert-manager.sh`) filters alerts whose target bead is closed.
2. **Exit code -1 means "check the environment first," not "check the code."** Every
   exit -1 in this workspace's history has been an infrastructure event (OOM, memory
   pressure). Verification of work completion comes before investigation of code.
3. **A crash is not a task failure.** Verify whether the work completed before
   treating a crash alert as actionable. Several alert beads in this storm were
   raised for work that ultimately succeeded.
4. **Retrying deterministic resource exhaustion never works.** Eleven identical
   failures proved it. After a second identical exit -1, stop and decompose the task
   (baseline → execute → verify child beads) instead of retrying.
5. **Never run bare `git gc --aggressive` on a bloated repository.** Use the
   memory-limited, checkpoint/resumable `scripts/safe-git-gc.sh` with
   `scripts/safe-git-gc-monitor.sh --watch`.
6. **Duplicate alerts during a crash storm are noise without dedup.** The storm
   raised 13 alert beads for 11 distinct failures. The alert system now has
   duplicate detection and a 5-minute cooldown for system-wide events.
7. **Prevention beats investigation.** Continuous repository-size monitoring with
   the thresholds in `docs/maintenance/repository-maintenance-guide.md` would have
   flagged the bloat long before the first OOM. Note (per synthesis §8): the
   systemd user timers that automate this are currently failing on environment
   grounds (PATH missing bash/git/jq; invalid `User=%i` in
   `domain-check-git-gc-full.service`) — fixing those is the one remaining open
   item from the investigation chain, owned by the operator.

---

## 6. Recommendation

**Close alert `bf-2prqor` as resolved — false positive confirmed.**

The alert has already been closed (2026-09-02T10:44:59Z); this report is the final
documentation supporting that closure. No further action is required on this alert:

- The original crash is fully explained (infrastructure: bloat → OOM → SIGKILL).
- The original task (`bf-65lsdu`) completed successfully and is closed.
- The root cause is eliminated at the source; the repository is 92MB, fsck-clean,
  with gc not needed as of this verification.
- Zero domain-check code defects were involved.

**Residual action tracked elsewhere (not this alert):** fix the systemd user
schedulers so the automated bloat-prevention loop actually runs — see §8 of
`docs/research/crash-investigation-synthesis-bf-65lsdu-2026-09-02.md`.

---

## 7. Related Documentation

- `docs/research/crash-investigation-synthesis-bf-65lsdu-2026-09-02.md` — final synthesis (domchk-944eb8bb), authoritative
- `docs/research/root-cause-analysis-bf-65lsdu-crash-2026-08-13.md` — formal RCA (domchk-2ab71440)
- `docs/crash-investigations/bf-65lsdu-crash-investigation.md` — first investigation (bf-1mcxco)
- `docs/notes/repository-cleanup-2026-08-13.md` — cleanup results (commit 993d8cd)
- `docs/verification/bf-65lsdu-cleanup-verification.md` — cleanup success verification
- `docs/verification-report-bf-2prqor-false-positive-retrospective-crash-alert-resolved-bf-65lsdu.md` — initial retrospective write-up (2026-08-26)
- `docs/crash-response-guide.md` — crash classification and response
- `docs/maintenance/repository-maintenance-guide.md` — size thresholds and maintenance
- `docs/crash-alert-fix-implementation-2026-09-02.md` — alert system hardening

**Verification completed:** 2026-09-02, by domchk-e590f9e0
**Conclusion:** FALSE POSITIVE — original task succeeded; alert correctly closed as resolved
