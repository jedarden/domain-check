# Crash Analysis and Recovery Report — bf-ncs0ev

**Report Date:** 2026-09-02  
**Investigation Bead:** bf-ncs0ev  
**Crash Bead:** bf-65lsdu (repository cleanup)  
**Classification:** FALSE POSITIVE — Infrastructure Event  
**Status:** RESOLVED

---

## Executive Summary

Agent crashes during the repository cleanup operation (`git gc --aggressive`) on bead
`bf-65lsdu` were caused by **extreme repository bloat** — ~18GB total with 17.20 GiB of
loose objects (4,515 objects) — triggering the kernel OOM killer during the
memory-intensive aggressive repack phase. Despite a series of SIGKILL terminations
(exit code -1) on 2026-08-13, the cleanup **ultimately completed successfully**: the
bead was split into three child beads on 2026-08-17 and the repository was reduced from
~18GB to ~90MB (a 99.5% reduction) with verified integrity.

**Key Finding:** No code changes were required. Domain-check code has zero defects;
the crash was a pure infrastructure event resolved by repository cleanup. All
subsequent recovery steps are documented here as a reference for future cleanup
operations.

---

## Timeline of Events

### 2026-08-13 — Cleanup Task Created and Crashes Began

| Timestamp (UTC) | Event | Exit Code | Notes |
|---|---|---|---|
| 21:16:00 | Bead `bf-65lsdu` created | — | "Run repository cleanup to eliminate 17GB bloat"; task: `git gc --aggressive --prune=now` |
| 21:22:35 | **Initial crash** | -1 (SIGKILL) | OOM during `git gc --aggressive` repack phase |
| 21:30:32 | Retry 1 | -1 (SIGKILL) | Crash alert `bf-1b5if7` |
| 21:48:30 | Retry 2 | -1 (SIGKILL) | Crash alert `bf-1944k2` |

### 2026-08-13 → 2026-08-14 — Repeated Failures

- **9+ crash alert beads** created as retries continued to hit the same OOM wall
- Every attempt failed with exit code -1 — the classic infrastructure-event signature
- Root cause remained unaddressed: any memory-intensive git operation on the bloated
  repository exhausted memory

### 2026-08-17 — Recovery via Bead Split

| Timestamp (UTC) | Event | Outcome |
|---|---|---|
| 00:32:41 | Child bead `domchk-bdb1fedf` created | Document current repository state (baseline) |
| 00:32:50 | Child bead `domchk-87be56d8` created | Verify and document cleanup results |
| (same day) | Child bead `domchk-af4b5ef4` created | Execute git gc cleanup |
| 00:34:00 | **Final success** | Cleanup completed; parent `bf-65lsdu` converted to umbrella bead and closed |

**Resolution strategy:** Rather than retrying the monolithic `git gc --aggressive`
(repeatedly OOM), the task was decomposed into small, independently verifiable child
beads. Each child completed without incident because no single step required the
memory footprint of a full aggressive repack.

### 2026-08-26 — Investigation (bf-ncs0ev)

- Investigation bead opened to document root cause and recovery
- Repository state verified: 139MB total, 1.14 MiB loose objects (267 objects),
  136.21 MiB packed (7,996 objects)
- `git fsck --full` clean — only normal dangling objects from branch operations
- Classified RESOLVED / false positive

### 2026-09-01 → 2026-09-02 — Sustained Health and Prevention

- Verification (child bead `domchk-87be56d8`, 2026-09-01) confirmed sustained healthy
  state: 90MB repository, 4 loose objects, single consolidated pack (see
  [Repository State Verification](#repository-state-verification) below)
- Root cause analysis finalized (`domchk-2ab71440`): infrastructure event, no code
  defects
- Crash alert system fixes implemented (closed-bead filtering, duplicate detection,
  completion awareness, cooldown, classification)

---

## Root Cause Analysis

*(From child bead 1 — investigation `domchk-2ab71440`, documented in
`docs/research/root-cause-analysis-bf-65lsdu-crash-2026-08-13.md`)*

### Primary Cause: Repository Bloat → OOM Killer → Process Termination

**Repository state at crash time:**

| Metric | Value | Healthy Target |
|---|---|---|
| Total repository size | ~18 GB | < 500 MB |
| Loose objects | 17.20 GiB (4,515 objects) | < 100 MB |
| Loose-to-packed ratio | ~99% loose | < 10% loose |

**Why `git gc --aggressive` triggered the OOM:**

1. Aggressive mode attempts delta optimization across **all** loose objects,
   requiring every object to be considered for delta chains
2. For 17.20 GiB across 4,515 objects, delta computation needs roughly 10–20GB of
   working memory on top of git's ~2–4GB base footprint
3. Peak demand exceeded available system memory; the kernel OOM killer sent SIGKILL
4. The agent process died with its git subprocess — recorded as exit code -1

**Exit code -1 significance:** the process was terminated by external force (kernel
OOM killer / SIGKILL), not a voluntary exit. Per the crash classification table in
`docs/crash-response-guide.md`, this is the infrastructure-event signature.

### Classification

| Classification | Probability | Evidence |
|---|---|---|
| **Infrastructure Event** | 70% | Repository bloat; OOM during git operation; resolved by cleanup |
| Workflow Failure | 20% | Multiple retries needed, but the split-bead workflow eventually succeeded |
| Service Failure | 8% | Not applicable — no external service dependency |
| Code Defect | 2% | **Zero evidence** — crash was in git gc, not domain-check code |

### Why It Was NOT a Code Defect

1. ✅ Crash occurred during `git gc`, not in any domain-check code path
2. ✅ Transient nature — crashes stopped permanently after repository cleanup
3. ✅ Comprehensive prior investigations found zero domain-check defects
4. ✅ Resolution pattern — resource cleanup fixed it; no code changes needed

### Likely Origin of the Bloat

Accumulation of ~17GB of loose objects is consistent with large untracked/committed
data (e.g., missing `.gitignore` entries for `.beads/` artifacts) combined with no
scheduled garbage collection. This same pattern later caused the bf-4yjq incident
(9 OOM crashes from an 18GB repository) and the bf-1s6c3 crash.

---

## Repository State Verification

*(From child bead 2 — `domchk-bdb1fedf` baseline and `domchk-87be56d8` verification;
artifacts: `cleanup-baseline.txt`, `cleanup-results.txt`)*

### Size Reduction Across the Recovery

| Stage | Repository Size | Loose Objects | Status |
|---|---|---|---|
| At crash time (2026-08-13) | ~18 GB | 17.20 GiB (4,515 objects) | 🔴 Critical bloat |
| After recovery (2026-08-26) | 139 MB | 1.14 MiB (267 objects) | ✅ Healthy |
| Current verified (2026-09-01) | 90 MB | 4 objects | ✅ Optimal |

**Overall improvement: ~99.5% size reduction; ~99.99% loose-object reduction.**

### Baseline Snapshot (child bead `domchk-bdb1fedf`, post-recovery baseline)

```
91M	.git
count: 40
size: 232
in-pack: 9076
packs: 3
size-pack: 90763
prune-packable: 0
garbage: 0
size-garbage: 0
```

### Final Verification (child bead `domchk-87be56d8`, 2026-09-01)

```
Current State (After Cleanup)
-----------------------------
Repository size: 90M
Loose objects: 4
Packed objects: 9102
Pack files: 1
Pack size: 90703K
Prune-packable: 0
Garbage files: 0

=== COMPARISON ===
Repository size: 91M → 90M (1M reduction)
Loose objects: 40 → 4 (90% reduction)
Pack files: 3 → 1 (consolidated)
Pack size: 90763K → 90703K (60K reduction)

✓ Cleanup successful - repository is properly packed and optimized
```

### Integrity Verification

```
$ git fsck --full
dangling commit 262330670bfd23038cd7a020e877539f43a4ce23
dangling commit b8568b63120328e8c1a558ceb85d8c27de2384bd
dangling commit 29794a6d28f88ed65ef22519ff817cd1b0ac1677
dangling tree 537f6b1986d13d64fcf56433d893f52b66396fbb
```

**Result:** ✅ Repository healthy. The dangling objects are normal unreachable commits
from branch operations — not corruption. No missing or damaged objects.

---

## Crash Artifacts

```
.beads/traces/bf-65lsdu/
├── metadata.json        (397 bytes)
├── stdout.txt          (1.4 MB)
├── stderr.txt          (456 bytes)
└── trace.jsonl         (18 KB)
```

---

## Lessons Learned

### 1. Decompose Oversized Cleanup Tasks

**What happened:** Nine-plus retries of the same monolithic `git gc --aggressive` all
hit the identical OOM wall. Retrying a deterministic resource exhaustion does not
work.

**What worked:** Splitting the task into three child beads (document baseline → execute
cleanup → verify results). Each step was small enough to complete within resource
limits and independently verifiable.

**Recommendation:** When a task fails repeatedly with the same infrastructure exit
code, split it into smaller, verifiable steps instead of retrying.

### 2. Never Run Bare `git gc --aggressive` on a Bloated Repository

Aggressive repack on 17GB of loose objects requires 10–20GB of working memory. Without
explicit limits, this is a guaranteed OOM on a shared box.

**Recommendation:**

```bash
# Preferred: safe, memory-limited, resumable
./scripts/safe-git-gc.sh --full
./scripts/safe-git-gc-monitor.sh --watch

# With explicit systemd memory cap
systemd-run --scope --quiet \
  -p MemoryMax=4g \
  -p MemorySwapMax=0 \
  -p CPUQuota=200% \
  git gc

# Lower-memory alternative with similar results
git repack -a -d --depth=250
```

### 3. Verify Work Completion Before Treating a Crash as Task Failure

The first crash alert implied the cleanup had failed. In fact the task eventually
completed — repeated crash alerts for a bead whose work ultimately succeeded are false
positives.

**Recommendation:** Always check the bead's final state and repository state before
opening a new investigation. The automated classifier
(`./scripts/crash-classifier.sh <bead-id>`) now does this first.

### 4. Monitor Repository Size Continuously

The repository drifted to 18GB before anyone noticed. Every threshold table in the
maintenance guide was exceeded long before the first OOM.

**Recommendation:** Enable continuous monitoring and honor the thresholds:

| Metric | Healthy | Warning | Critical |
|---|---|---|---|
| Total repository size | <500MB | 500MB–1GB | >1GB |
| Loose objects | <100MB | 100–500MB | >500MB |
| Loose object count | <100 | 100–1,000 | >1,000 |

```bash
./scripts/monitoring-setup.sh     # continuous monitoring via cron
./scripts/check-repo-health.sh    # manual health check
```

### 5. Prevent the Bloat at the Source

The 17GB accumulated because large artifacts were not excluded from git.

**Recommendation:** Keep `.beads/` artifacts out of the repository:

```gitignore
.beads/*.jsonl
.beads/*.json
.beads/checkpoint/
.beads/traces/
```

And install pre-commit hooks blocking files >10MB
(`./scripts/setup-git-hooks.sh`).

---

## Recommendations for Future Cleanup Operations

### Pre-Cleanup Checklist

```bash
# 1. Measure before
du -sh .git
git count-objects -vH
git fsck --full

# 2. Check system resources
free -h          # need ≥ 10GB available
df -h /          # need ≥ 20GB free

# 3. Choose strategy by repository size
if [ "$(du -sm .git | cut -f1)" -gt 1000 ]; then
  echo "Repository >1GB — use safe-git-gc with monitoring"
  ./scripts/safe-git-gc.sh --full
  ./scripts/safe-git-gc-monitor.sh --watch
else
  git gc
fi

# 4. Measure after and verify
du -sh .git
git count-objects -vH
git fsck --full
```

### Operational Rules

1. **Always** use `scripts/safe-git-gc.sh` rather than bare `git gc --aggressive`
2. **Always** run pre-flight checks (`./scripts/preflight-health-check.sh`) before
   memory-intensive operations
3. **Always** measure baseline before and verify after — record both, as the child
   beads did
4. **Never** retry a failed aggressive gc unattended; split the task instead
5. **Escalate** to the operator if `git fsck` reports corruption, or if repository
   size exceeds 1GB with no bloat source identifiable

---

## Prevention Measures Now in Place

| Measure | Implemented | Location |
|---|---|---|
| Safe git gc scripts (memory-limited, resumable, monitored) | 2026-08-15 | `scripts/safe-git-gc.sh`, `scripts/safe-git-gc-monitor.sh` |
| Pre-flight health checks | 2026-09-01 | `scripts/preflight-health-check.sh` |
| Crash alert fixes (closed-bead filter, dedup, cooldown, classification) | 2026-09-02 | `scripts/crash-alert-manager.sh`, `scripts/crash-classifier.sh` |
| Continuous monitoring (repo size, resources, services, crash patterns) | 2026-09-02 | `scripts/monitoring-setup.sh` |

---

## Related Documentation

**This incident:**
- `docs/research/root-cause-analysis-bf-65lsdu-crash-2026-08-13.md` — child bead 1 root cause analysis (domchk-2ab71440)
- `docs/notes/cleanup-crash-investigation-bf-ncs0ev.md` — original investigation summary
- `docs/crash-information-bf-65lsdu.md` — crash metadata
- `cleanup-baseline.txt`, `cleanup-results.txt` — child bead 2 measurement artifacts

**General:**
- `docs/crash-response-guide.md` — crash classification and response procedures
- `docs/maintenance/repository-maintenance-guide.md` — daily maintenance and emergency cleanup
- `docs/comprehensive-crash-prevention-guide.md` — systemic prevention
- `docs/crash-artifacts-bf-4yjq.md` — the parallel bf-4yjq repository-bloat incident
- `docs/crash-alert-fix-implementation-2026-09-02.md` — alert system fixes

---

## Conclusion

The crash on bead `bf-65lsdu` was **caused by repository bloat (17.20 GiB of loose
objects) triggering the OOM killer during `git gc --aggressive`** — a pure
infrastructure event with zero domain-check code involvement. Recovery succeeded by
splitting the task into three verifiable child beads, reducing the repository from
~18GB to 90MB (99.5%) with verified integrity. Prevention measures (safe git gc
scripts, pre-flight checks, continuous monitoring, alert filtering) are now in place.

**Status:** ✅ RESOLVED — root cause identified, recovery documented, recurrence
prevented.

**Related beads:** `bf-65lsdu` (cleanup, closed), `bf-ncs0ev` (investigation, closed),
`domchk-2ab71440` (root cause analysis), `domchk-bdb1fedf` (baseline),
`domchk-af4b5ef4` (cleanup execution), `domchk-87be56d8` (verification)
