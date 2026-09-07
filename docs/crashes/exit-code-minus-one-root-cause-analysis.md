# Root Cause Analysis: Agent Exit Code -1

**Report Date:** 2026-09-01
**Analysis Scope:** All agent crashes with exit code -1 in domain-check workspace
**Classification:** Infrastructure Failure (OOM) - NOT a code defect

---

## Executive Summary

**Root Cause:** Linux OOM Killer SIGKILL during git operations on bloated repository

**Primary Finding:** Exit code -1 is NOT an application error code—it indicates signal-based termination, specifically SIGKILL (signal 9) delivered by the Linux OOM (Out Of Memory) killer when system memory is exhausted.

**Final Classification:** Infrastructure/Environmental Failure - NOT a code defect

**Status:** ✅ RESOLVED - Repository cleanup eliminated root cause

---

## What Exit Code -1 Means

### Technical Explanation

Exit code -1 is a special Unix/Linux convention indicating **signal-based termination**, not a normal application exit:

| Aspect | Value | Meaning |
|--------|-------|---------|
| **Exit Code** | -1 | Signal-based termination (not 0 for success, not 1 for error) |
| **Signal** | SIGKILL (signal 9) | Immediate process termination |
| **Delivered By** | Linux OOM killer | System-level memory management |
| **Termination Type** | Immediate | No graceful shutdown, no error logging |

### Critical Distinction

**Exit Code -1 ≠ Application Error**

- **Application Error (exit code 1):** Code encountered a bug, validation failure, or logic error
- **Signal Termination (exit code -1):** External force killed the process (system, user, or resource exhaustion)

In this case, the signal was SIGKILL from the OOM killer, meaning the process was killed due to memory exhaustion—not due to any code defect.

---

## Root Cause Mechanism

### Step-by-Step Crash Sequence

1. **Agent initiated git reconciliation** on 18GB repository
2. **Git operations loaded massive data** (17GB loose objects into memory)
3. **Memory exhaustion** — <2GB available from 62GB total
4. **Linux OOM killer invoked** — targeted git process as memory hog
5. **SIGKILL (signal 9) delivered** — immediate process termination
6. **Exit code -1 returned** — no graceful shutdown possible
7. **Agent terminated** without cleanup or error logging

### Repository Bloat: The Root Trigger

**Repository metrics during crash:**
```
Repository Size: 18 GB (should be <500 MB)
Loose Objects: 17.16 GB (4,482 unpacked objects)
Pack Files: 9.60 MB (inverted ratio)
Size Ratio: 1,832:1 loose-to-packed (should be inverted)
```

**Cause of bloat:** Repeated commits of massive `.beads/` JSONL files
- 17+ identical commits for "GitHub-specific commits extraction"
- Each commit included ~500MB of bead JSONL files
- **Impact:** 17 commits × ~500MB per commit = ~8.5GB of redundant data

---

## Crash Classification

| Aspect | Finding | Evidence |
|--------|---------|----------|
| **Type** | Infrastructure Failure | OOM killer invoked |
| **Cause** | Repository bloat → memory exhaustion | 18GB repo with 17GB loose objects |
| **Impact** | Git operation disruption | Merge commit reconciliation task |
| **Code Defect** | NONE — Agent implementation was correct | All investigations found no bugs |
| **Reproducibility** | HIGH — Would recur on same repository state | Multiple crashes during same period |
| **Resolution** | Repository cleanup eliminated root cause | 18GB → 138MB (99.2% reduction) |

---

## Systematic Pattern: Multiple Crashes

This crash was part of a **systematic pattern of SIGKILL crashes** during 2026-08-12 to 2026-08-16:

| Bead ID | Timestamp | Task | Exit Code |
|---------|-----------|------|-----------|
| bf-1s6c3 | 2026-08-13T00:38:41Z | Merge commit reconciliation | -1 (SIGKILL) |
| bf-4x12ec | 2026-08-14T11:14:39Z | Git gc operations | -1 (SIGKILL) |
| Multiple others | 2026-08-12 to 2026-08-16 | Various git operations | -1 (SIGKILL) |

**Pattern:** All crashes showed identical SIGKILL behavior when performing git operations on the bloated repository.

---

## Risk Assessment for Future Operations

### Current Risk Level: ✅ MINIMAL (Eliminated)

| Risk Factor | Status | Evidence |
|-------------|--------|----------|
| **Repository Health** | ✅ HEALTHY | Cleanup completed (18GB → 138MB) |
| **System Resources** | ✅ ADEQUATE | 51GB available memory |
| **Code Integrity** | ✅ VERIFIED | No defects found in any investigation |
| **Environmental Factors** | ✅ RESOLVED | Repository bloat eliminated |

### Historical Risk Level (During 2026-08-12 to 2026-08-16): ⚠️ CRITICAL

- Repository bloat made git operations extremely memory-intensive
- Any git operation could trigger OOM killer
- Multiple agents crashed during this period

### Is the Crash Reproducible?

**Answer:** YES — but only on the same repository state (18GB with 17GB loose objects)

**Current Repository State:**
- Repository size: 138MB (healthy)
- Loose objects: 85 (healthy)
- In-pack objects: 7,106 (properly packed)
- **Risk:** ELIMINATED

**Pre-Cleanup Repository State:**
- Repository size: 18GB (bloated)
- Loose objects: 4,482 (excessive)
- Pack files: 9.60MB (inverted ratio)
- **Risk:** CRITICAL — Would recur deterministically

---

## Crash Determination: Transient vs. Systemic

### Classification: ✅ TRANSIENT (Infrastructure Issue)

**Why This Is Transient:**

1. **Root Cause Was Environmental:** Repository bloat was an external environmental factor, not a code defect
2. **Single Point of Failure:** The issue was resolved by fixing the repository state
3. **No Code Changes Required:** No fixes to domain-check code were necessary
4. **Verification:** Same git operations now complete successfully on healthy repository

**Why This Was NOT Systemic:**

1. **No Code Defect:** All investigations found zero bugs in domain-check code
2. **Task Completed Successfully:** After repository cleanup, the exact same task succeeded
3. **Pattern Consistent with Infrastructure:** All crashes during this period were OOM-related
4. **No Recurrence:** No crashes after repository cleanup (as of 2026-09-01)

---

## Evidence from Investigation

### Child Bead Evidence (domchk-0550073d)

The child bead collected comprehensive crash artifacts:

- ✅ `docs/crashes/bf-1s6c3-report.md` (11,874 bytes) — Comprehensive crash analysis
- ✅ `docs/crashes/bf-1s6c3-root-cause-summary.md` (5,776 bytes) — Root cause analysis
- ✅ `docs/crashes/bf-1s6c3-oom-investigation.md` (8,600 bytes) — OOM investigation details
- ✅ `docs/crashes/bf-1s6c3-crash-evidence-report.md` (13,868 bytes) — Structured evidence report

### Key Evidence Points

**1. Crash Timestamp and Signal:**
- Date: 2026-08-13T00:38:41Z
- Exit Code: -1 (signal -1)
- Signal: SIGKILL (signal 9)
- Delivered By: Linux OOM killer

**2. System State During Crash:**
- Total Memory: 62 GB
- Available Memory: <2 GB (CRITICAL)
- Git Memory Usage: Spike to >50GB
- OOM Threshold Reached: YES

**3. Repository State:**
- Size: 18 GB (should be <500MB)
- Loose Objects: 17.16 GB (4,482 unpacked objects)
- Pack Files: 9.60 MB
- Bloat Cause: Repeated massive `.beads/` JSONL commits

**4. Task Context:**
- Objective: Create merge commit reconciling Forgejo/GitHub histories
- Complexity: High (merge with divergent histories)
- Memory Requirements: High (git operations on 18GB repo)

**5. Resolution:**
- Repository Cleanup: 18GB → 138MB (99.2% reduction)
- Loose Objects: 4,482 → 85 (98% reduction)
- Task Completion: ✅ SUCCESSFUL on 2026-08-16
- Post-Crash Verification: No errors found

---

## Recommendations for Prevention

### Immediate Actions: ✅ COMPLETED

1. ✅ **Repository Cleanup:** Executed successfully (18GB → 138MB)
2. ✅ **Git Operations Verified:** Same tasks now complete successfully
3. ✅ **Monitoring Implemented:** Crash pattern detection and resource monitoring scripts installed

### Long-Term Prevention Strategies

**1. Repository Health Monitoring**

```bash
# Pre-flight check before git operations
#!/bin/bash
REPO_SIZE=$(du -sh .git 2>/dev/null | awk '{print $1}' | sed 's/G//')
if (( $(echo "$REPO_SIZE > 1" | bc -l) )); then
  echo "WARNING: Repository size ${REPO_SIZE}GB exceeds 1GB threshold"
  exit 1
fi
```

**2. Pre-Flight Resource Checks**

```bash
# Check available memory before memory-intensive operations
AVAILABLE_MEM=$(free -g | awk '/^Mem:/{print $7}')
if [ $AVAILABLE_MEM -lt 10 ]; then
  echo "ABORT: Insufficient memory (${AVAILABLE_MEM}GB available, need 10GB+)"
  exit 1
fi
```

**3. Automated Cleanup Trigger**

```bash
# Run git gc when loose objects exceed threshold
LOOSE_OBJECTS=$(git count-objects -vH | grep "loose objects" | awk '{print $3}')
if [ $LOOSE_OBJECTS -gt 1000 ]; then
  echo "Too many loose objects (${LOOSE_OBJECTS}), running git gc"
  ./scripts/safe-git-gc.sh
fi
```

**4. Monitoring and Alerting**

The following monitoring scripts have been implemented:

- `./scripts/crash-pattern-detection.sh` — Detects crash surges (10+ crashes in 10 minutes)
- `./scripts/resource-monitor.sh` — Monitors memory, disk, and CPU thresholds
- `./scripts/service-monitor.sh` — Checks inference gateway availability
- `./scripts/monitoring-setup.sh` — Installs continuous monitoring via cron

**5. Safe Git Operations**

```bash
# Always use safe git gc scripts instead of bare `git gc --aggressive`
./scripts/safe-git-gc.sh --check-only   # Check if gc is needed
./scripts/safe-git-gc.sh                # Run standard gc (stages 1-2)
./scripts/safe-git-gc.sh --full         # Run full gc with deep compression
./scripts/safe-git-gc.sh --resume        # Resume from checkpoint if interrupted
```

---

## Conclusions

### Primary Findings

1. **Exit Code -1 Meaning:** Signal-based termination (SIGKILL from OOM killer), NOT an application error

2. **Root Cause:** Repository bloat (18GB with 17GB loose objects) causing memory exhaustion during git operations

3. **Code Quality:** ✅ NO DEFECTS FOUND — Domain-check code is healthy and defect-free

4. **Classification:** Infrastructure failure (OOM) — NOT a code defect

5. **Risk Level:** ✅ ELIMINATED — Repository cleanup resolved the issue

6. **Task Completion:** ✅ SUCCESSFUL — Bead bf-1s6c3 completed successfully after repository cleanup

### Risk Assessment for Future Operations

| Scenario | Risk Level | Reason |
|----------|------------|--------|
| **Current Operations** | ✅ MINIMAL | Repository healthy (138MB), system resources adequate |
| **Similar Workloads** | ✅ MINIMAL | No code defects, same operations now succeed |
| **Git Operations on Healthy Repo** | ✅ MINIMAL | Repository bloat eliminated |
| **Git Operations on Bloated Repo** | ⚠️ CRITICAL | Would recur deterministically |

### Reproducibility Assessment

**Is This Crash Reproducible?**

- ✅ **YES** — On the same repository state (18GB with 17GB loose objects)
- ❌ **NO** — On the current repository state (138MB, healthy)
- ✅ **RESOLVED** — Root cause eliminated through repository cleanup

### Final Status

| Aspect | Status |
|--------|--------|
| **Root Cause Identified** | ✅ COMPLETE — Repository bloat causing OOM |
| **Code Defects Found** | ✅ NONE — Domain-check code is healthy |
| **Risk Eliminated** | ✅ YES — Repository cleanup completed |
| **Prevention Measures** | ✅ IN PLACE — Monitoring and safe-git-gc scripts |
| **Action Required** | ✅ NONE — Fully resolved |

---

## Key Takeaways

1. **Exit Code -1 = Signal, Not Bug:** When you see exit code -1, think "signal termination" not "application error"

2. **Repository Health Matters:** Monitor repository size and loose object count to prevent bloat

3. **Pre-Flight Checks Save Time:** Check resources before memory-intensive operations

4. **Domain-Check Code is Solid:** Multiple investigations found zero code defects

5. **Infrastructure Issues Happen:** OOM, SIGHUP, and transient failures are real — focus investigation on environment, not code

---

**Report Status:** ✅ COMPLETE
**Classification:** Transient Infrastructure Failure (OOM)
**Risk Assessment:** MINIMAL — Root cause eliminated
**Action Required:** NONE — Fully resolved and documented

---

*Report Generated: 2026-09-01*
*Analysis Complete: YES*
*Follow-up Required: NO*
