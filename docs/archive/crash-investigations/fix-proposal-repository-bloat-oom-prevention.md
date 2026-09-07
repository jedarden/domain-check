# Fix Proposal: Repository Bloat OOM Prevention

**Created:** 2026-09-01  
**Target:** Exit Code -1 (SIGKILL from OOM killer)  
**Root Cause:** Repository bloat (18GB with 17GB loose objects) causing memory exhaustion during git operations  
**Classification:** Infrastructure Failure - NOT a code defect  

---

## Executive Summary

**Crash Cause:** Linux OOM killer delivered SIGKILL (exit code -1) during git operations on a bloated 18GB repository containing 17GB of loose objects. The bloat was caused by 17+ identical commits of ~500MB `.beads/` JSONL files.

**Fix Strategy:** Implement comprehensive repository health monitoring, automated maintenance, and preventive controls to eliminate repository bloat before it causes OOM crashes.

**Impact:** Prevents the most common crash type (70% of crashes are infrastructure-related, with repository bloat being a major contributor).

---

## Root Cause Analysis

### What Happened

| Metric | During Crash | After Cleanup | Reduction |
|--------|--------------|---------------|-----------|
| **Repository Size** | 18GB | 138MB | 99.2% |
| **Loose Objects** | 17.16GB (4,482 objects) | 85 objects | 98% |
| **Pack Files** | 9.60MB | Properly packed | Inverted ratio corrected |
| **Available Memory** | <2GB (CRITICAL) | 51GB | System recovered |

### Crash Timeline

1. **Agent initiated git reconciliation** on 18GB repository
2. **Git operations loaded massive data** (17GB loose objects into memory)
3. **Memory exhaustion** — <2GB available from 62GB total
4. **Linux OOM killer invoked** — targeted git process as memory hog
5. **SIGKILL (signal 9) delivered** — immediate process termination
6. **Exit code -1 returned** — no graceful shutdown possible

### Why This Occurred

**Repository Bloat Trigger:** Bead bf-2ildm committed 17+ identical 237MB `.beads/*.jsonl` files to git history. Each commit included ~500MB of bead workspace files that should never have been committed.

**Result:** Any significant git operation (merge, gc, pull, push) triggered OOM killer intervention because git had to load 17GB of loose objects into memory.

---

## Fix Proposal

### Primary Fix: Repository Bloat Prevention (CRITICAL - IMMEDIATE)

#### Fix 1.1: Prevent .beads/ Files from Being Committed

**Problem:** Bead workspace files (`.beads/*.jsonl`, `.beads/checkpoint/`, `.beads/traces/`) are being committed to git, causing repository bloat.

**Implementation:**

```bash
# Add to .gitignore immediately
cat >> .gitignore <<EOF
# Bead workspace files (should never be committed)
.beads/*.jsonl
.beads/*.json
.beads/checkpoint/
.beads/traces/
.beads/github_*.json
.beads/divergence-*.json
EOF
```

**Pre-commit Hook to Block Large Files:**

```bash
# Install .git/hooks/pre-commit
cat > .git/hooks/pre-commit <<'EOF'
#!/bin/bash
# Block commits with files > 10MB
MAX_SIZE_MB=10

large_files=$(git diff --cached --name-only --diff-filter=ACMR | while read file; do
  if [ -f "$file" ]; then
    size=$(stat -f%z "$file" 2>/dev/null || stat -c%s "$file" 2>/dev/null)
    size_mb=$(echo "$size / 1048576" | bc)
    if [[ $(echo "$size_mb > $MAX_SIZE_MB" | bc -l) -eq 1 ]]; then
      echo "$file (${size_mb}MB)"
    fi
  fi
done)

if [ -n "$large_files" ]; then
  echo "ERROR: Commit blocked - large files detected:"
  echo "$large_files"
  echo "Maximum file size: ${MAX_SIZE_MB}MB"
  echo "Add large files to .gitignore or use git-lfs"
  exit 1
fi
EOF

chmod +x .git/hooks/pre-commit
```

**Risk:** Very Low - Prevents accidental large file commits  
**Timeline:** Immediate  
**Status:** ✅ Ready to implement

---

#### Fix 1.2: Automated Git GC Scheduling

**Problem:** Manual git gc only happens after crashes; preventive gc is needed to pack loose objects before they accumulate.

**Implementation:**

```bash
# Install automated gc scheduling (cron-based)
cat > /tmp/install-git-gc-cron.sh <<'SCRIPT'
#!/bin/bash
# Schedule daily git gc during low-activity hours

# Run standard gc daily at 3 AM
(crontab -l 2>/dev/null; echo "0 3 * * * cd /home/coding/domain-check && ./scripts/safe-git-gc.sh >> /var/log/git-gc.log 2>&1") | crontab -

# Run full gc weekly on Sunday at 4 AM
(crontab -l 2>/dev/null; echo "0 4 * * 0 cd /home/coding/domain-check && ./scripts/safe-git-gc.sh --full >> /var/log/git-gc.log 2>&1") | crontab -

# Repository health check daily at 2 AM
(crontab -l 2>/dev/null; echo "0 2 * * * cd /home/coding/domain-check && ./scripts/repository-health-check.sh >> /var/log/repo-health.log 2>&1") | crontab -

echo "Git gc scheduling installed:"
crontab -l | grep -E "git-gc|repo-health"
SCRIPT

chmod +x /tmp/install-git-gc-cron.sh
/tmp/install-git-gc-cron.sh
```

**Repository Health Check Script:**

```bash
# Create scripts/repository-health-check.sh
cat > scripts/repository-health-check.sh <<'EOF'
#!/bin/bash
# Repository health monitoring script
REPO_MAX_SIZE_GB=1  # Alert if repo exceeds 1GB
LOOSE_OBJECTS_MAX_GB=0.5  # Alert if loose objects exceed 500MB

check_repository_health() {
  local repo_size=$(du -s .git 2>/dev/null | awk '{print $1/1048576}')  # Convert KB to GB
  local loose_objects=$(git count-objects -v 2>/dev/null | grep "size: " | awk '{print $2/1048576}')
  local pack_files=$(git count-objects -v 2>/dev/null | grep "size-pack: " | awk '{print $2/1048576}')

  echo "[$(date)] Repository Size: ${repo_size}GB"
  echo "[$(date)] Loose Objects: ${loose_objects}GB"
  echo "[$(date)] Pack Files: ${pack_files}GB"

  if [[ $(echo "$repo_size > $REPO_MAX_SIZE_GB" | bc -l) -eq 1 ]]; then
    echo "[$(date)] WARNING: Repository size (${repo_size}GB) exceeds threshold (${REPO_MAX_SIZE_GB}GB)"
    echo "[$(date)] Action required: Run git gc or investigate large files"
    return 1
  fi

  if [[ $(echo "$loose_objects > $LOOSE_OBJECTS_MAX_GB" | bc -l) -eq 1 ]]; then
    echo "[$(date)] WARNING: Loose objects (${loose_objects}GB) exceed threshold (${LOOSE_OBJECTS_MAX_GB}GB)"
    echo "[$(date)] Action required: Run 'git gc' to pack loose objects"
    return 1
  fi

  echo "[$(date)] Repository health: OK"
  return 0
}

check_repository_health
exit $?
EOF

chmod +x scripts/repository-health-check.sh
```

**Risk:** Low - gc operations are safe with memory limits via safe-git-gc.sh  
**Timeline:** Immediate  
**Status:** ✅ Ready to implement

---

#### Fix 1.3: Pre-Task Repository Health Check

**Problem:** Agents start git operations without checking repository health, leading to OOM crashes on bloated repositories.

**Implementation:**

```bash
# Create scripts/pre-task-check.sh
cat > scripts/pre-task-check.sh <<'EOF'
#!/bin/bash
# Pre-task repository health check (run before git operations)

check_repository_health_before_task() {
  local repo_size=$(du -s .git 2>/dev/null | awk '{print $1/1048576}')
  local max_size_gb=2
  
  if [[ $(echo "$repo_size > $max_size_gb" | bc -l) -eq 1 ]]; then
    echo "ERROR: Repository bloated (${repo_size}GB) - exceeds ${max_size_gb}GB threshold"
    echo "Run: ./scripts/safe-git-gc.sh"
    return 1
  fi
  
  echo "Repository health OK (${repo_size}GB)"
  return 0
}

check_repository_health_before_task
exit $?
EOF

chmod +x scripts/pre-task-check.sh
```

**Integration with Preflight Health Check:**

```bash
# Add repository check to existing preflight-health-check.sh
cat >> scripts/preflight-health-check.sh <<'EOF'

# Repository health check
if ! ./scripts/pre-task-check.sh; then
  echo "ERROR: Repository health check failed"
  exit 1
fi
EOF
```

**Risk:** Very Low - Read-only check before task start  
**Timeline:** Short-term (1 week)  
**Status:** ✅ Ready to implement

---

### Secondary Fix: Enhanced Monitoring

#### Fix 2.1: Crash Pattern Detection

**Problem:** No automated detection of systematic crash patterns (e.g., 10+ crashes in 10 minutes indicating infrastructure event).

**Implementation:**

```bash
# Create scripts/crash-pattern-detection.sh
cat > scripts/crash-pattern-detection.sh <<'EOF'
#!/bin/bash
# Analyze crash patterns to detect infrastructure events

detect_crash_patterns() {
  local crash_count=$(bead list --status "crashed" --since "10min" --json 2>/dev/null | jq '. | length')
  
  if [[ $crash_count -gt 10 ]]; then
    echo "[$(date)] INFRASTRUCTURE EVENT DETECTED: $crash_count crashes in 10 minutes"
    echo "[$(date)] Classifying as system-wide event, not individual bead failures"
    
    # Classify crashes by exit code
    bead list --status "crashed" --since "10min" --json 2>/dev/null | \
      jq -r '[group_by(.exit_code) | .[] | {exit_code: .[0].exit_code, count: length}]'
    
    return 1
  fi
  
  echo "[$(date)] Crash pattern normal ($crash_count crashes in 10 minutes)"
  return 0
}

detect_crash_patterns
exit $?
EOF

chmod +x scripts/crash-pattern-detection.sh
```

**Risk:** Very Low - Analysis only  
**Timeline:** Short-term (1 week)  
**Status:** ✅ Ready to implement

---

#### Fix 2.2: Resource Monitoring

**Problem:** No continuous monitoring of system resources to predict OOM conditions.

**Implementation:**

```bash
# Create scripts/resource-monitor.sh
cat > scripts/resource-monitor.sh <<'EOF'
#!/bin/bash
# Monitor system resources for OOM risk

MEMORY_WARNING_GB=10
MEMORY_CRITICAL_GB=5
DISK_WARNING_GB=30
DISK_CRITICAL_GB=20
CPU_WARNING=10

check_resources() {
  local available_mem=$(free -g | awk '/^Mem:/{print $7}')
  local disk_free=$(df -BG / | awk 'NR==2 {print $4}' | tr -d 'G')
  local cpu_load=$(uptime | awk '{print $10}' | cut -d, -f1)
  
  echo "[$(date)] Available Memory: ${available_mem}GB"
  echo "[$(date)] Disk Free: ${disk_free}GB"
  echo "[$(date)] CPU Load (1min): $cpu_load"
  
  local status=0
  
  if [ $available_mem -lt $MEMORY_CRITICAL_GB ]; then
    echo "[$(date)] CRITICAL: Memory critically low (${available_mem}GB < ${MEMORY_CRITICAL_GB}GB)"
    echo "[$(date)] OOM risk imminent"
    status=1
  elif [ $available_mem -lt $MEMORY_WARNING_GB ]; then
    echo "[$(date)] WARNING: Memory low (${available_mem}GB < ${MEMORY_WARNING_GB}GB)"
    status=1
  fi
  
  if [ $disk_free -lt $DISK_CRITICAL_GB ]; then
    echo "[$(date)] CRITICAL: Disk space critically low (${disk_free}GB < ${DISK_CRITICAL_GB}GB)"
    status=1
  elif [ $disk_free -lt $DISK_WARNING_GB ]; then
    echo "[$(date)] WARNING: Disk space low (${disk_free}GB < ${DISK_WARNING_GB}GB)"
    status=1
  fi
  
  if [[ $(echo "$cpu_load > $CPU_WARNING" | bc -l) -eq 1 ]]; then
    echo "[$(date)] WARNING: High CPU load ($cpu_load > $CPU_WARNING)"
    status=1
  fi
  
  return $status
}

check_resources
exit $?
EOF

chmod +x scripts/resource-monitor.sh
```

**Risk:** Very Low - Monitoring only  
**Timeline:** Short-term (1 week)  
**Status:** ✅ Ready to implement

---

### Tertiary Fix: Safe Git Operations

#### Fix 3.1: Enforce Safe Git GC Usage

**Problem:** Bare `git gc --aggressive` commands can cause OOM on large repos without memory limits.

**Implementation:**

**Status:** ✅ **ALREADY IMPLEMENTED** - `scripts/safe-git-gc.sh` exists with:
- Memory-limited operations (configurable via `SAFE_GC_MEMORY_MAX`)
- Three-stage gc strategy (standard → incremental → deep compression)
- Checkpoint/resume capability
- Progress tracking and monitoring
- Pre-flight integrity checks

**Evidence from Crash Analysis:**
- Git gc completed successfully in 6 minutes
- Repository optimized from ~18GB to 445MB (97.5% reduction)
- Peak memory usage: 1.1GB (well within limits)
- No OOM events occurred
- Repository integrity verified

**Recommendation:** Use existing `scripts/safe-git-gc.sh --full` instead of bare `git gc --aggressive`.

**Risk:** Very Low - Proven safe implementation  
**Timeline:** Immediate - use existing scripts  
**Status:** ✅ Already implemented

---

## Implementation Steps

### Phase 1: Immediate (Today)

1. **Add .gitignore rules** (5 minutes)
   ```bash
   cat >> .gitignore <<EOF
   # Bead workspace files (should never be committed)
   .beads/*.jsonl
   .beads/*.json
   .beads/checkpoint/
   .beads/traces/
   .beads/github_*.json
   .beads/divergence-*.json
   EOF
   ```

2. **Install pre-commit hook** (5 minutes)
   ```bash
   # Copy pre-commit hook from Fix 1.1
   chmod +x .git/hooks/pre-commit
   ```

3. **Create repository health check script** (10 minutes)
   ```bash
   # Create scripts/repository-health-check.sh from Fix 1.2
   chmod +x scripts/repository-health-check.sh
   ```

4. **Install automated gc scheduling** (5 minutes)
   ```bash
   # Run install-git-gc-cron.sh from Fix 1.2
   ```

**Total Time:** ~25 minutes  
**Risk:** Very Low

### Phase 2: Short-term (This Week)

1. **Create pre-task repository health check** (15 minutes)
   ```bash
   # Create scripts/pre-task-check.sh from Fix 1.3
   chmod +x scripts/pre-task-check.sh
   ```

2. **Create crash pattern detection script** (15 minutes)
   ```bash
   # Create scripts/crash-pattern-detection.sh from Fix 2.1
   chmod +x scripts/crash-pattern-detection.sh
   ```

3. **Create resource monitoring script** (15 minutes)
   ```bash
   # Create scripts/resource-monitor.sh from Fix 2.2
   chmod +x scripts/resource-monitor.sh
   ```

4. **Install continuous monitoring** (10 minutes)
   ```bash
   # Create scripts/monitoring-setup.sh
   # This installs cron jobs for continuous monitoring
   ```

**Total Time:** ~55 minutes  
**Risk:** Very Low

### Phase 3: Integration (Next Week)

1. **Integrate repository health check into preflight check** (10 minutes)
   ```bash
   # Add repository check to scripts/preflight-health-check.sh
   ```

2. **Test all monitoring scripts** (30 minutes)
   ```bash
   # Run all scripts manually to verify they work
   ./scripts/repository-health-check.sh
   ./scripts/crash-pattern-detection.sh
   ./scripts/resource-monitor.sh
   ```

3. **Document in CLAUDE.md** (20 minutes)
   ```bash
   # Update CLAUDE.md with repository health procedures
   ```

**Total Time:** ~60 minutes  
**Risk:** Very Low

---

## Side Effects and Trade-offs

### Positive Side Effects

1. **Prevent All Repository Bloat Crashes:** OOM from git operations eliminated
2. **Earlier Detection:** Repository issues detected before crashes occur
3. **Automated Maintenance:** No manual intervention required for routine gc
4. **Better Visibility:** Continuous monitoring provides early warning

### Minimal Trade-offs

1. **Cron Job Overhead:** Daily gc runs consume minimal resources (<5 minutes/day)
2. **Pre-commit Hook Delay:** Adds ~1 second to commit time (acceptable trade-off for safety)
3. **Disk Space for Logs:** Monitoring logs consume minimal space (<1MB/day)

### Risk Assessment

| Fix | Risk Level | Mitigation |
|-----|------------|------------|
| 1.1 .gitignore Update | Very Low | Prevents accidental commits only |
| 1.2 Pre-commit Hook | Very Low | Can be bypassed with --no-verify if needed |
| 1.2 Automated GC | Low | Uses safe-git-gc.sh with memory limits |
| 1.3 Pre-task Check | Very Low | Read-only check before task start |
| 2.1 Crash Pattern Detection | Very Low | Analysis only, no changes |
| 2.2 Resource Monitoring | Very Low | Monitoring only |
| 3.1 Safe Git GC | Very Low | Already proven safe |

---

## Success Metrics

### Primary Metrics

- ✅ **Zero OOM crashes from repository bloat** (target: 0 incidents/month)
- ✅ **Repository size < 500MB** (current: 138MB ✓)
- ✅ **Loose objects < 100MB** (current: minimal ✓)

### Secondary Metrics

- ✅ **Pre-commit hook prevents large file commits** (target: 0 large files committed)
- ✅ **Automated gc runs daily** (target: 100% completion rate)
- ✅ **Monitoring detects issues early** (target: 24+ hours warning before OOM risk)

### Verification

```bash
# Verify repository health
./scripts/repository-health-check.sh
# Expected: "Repository health: OK"

# Verify gc scheduling
crontab -l | grep git-gc
# Expected: Daily and weekly gc entries

# Verify monitoring
ls -la .beads/logs/
# Expected: crash-monitor.log, resource-monitor.log
```

---

## Alternative Approaches Considered

### Alternative 1: Git LFS for Large Files

**Approach:** Use git-lfs to track large files instead of blocking them.

**Rejected Because:**
- Adds complexity (git-lfs installation, configuration, server)
- `.beads/` files should NEVER be committed (not even via LFS)
- Simpler to just prevent commits via .gitignore

### Alternative 2: Increase System Memory

**Approach:** Add more RAM to prevent OOM.

**Rejected Because:**
- Does not address root cause (repository bloat)
- Memory is finite; bloat will eventually exceed any limit
- Cost-ineffective vs. preventive maintenance

### Alternative 3: Manual Git GC Before Operations

**Approach:** Require manual gc before git operations.

**Rejected Because:**
- Relies on human operator remembering
- No early warning before repository becomes problematic
- Automating via cron is more reliable

---

## Conclusion

**Fix Strategy:** Implement comprehensive repository bloat prevention through .gitignore rules, pre-commit hooks, automated gc scheduling, and continuous monitoring.

**Impact:** Eliminates the most common crash cause (70% infrastructure events, with repository bloat being a major contributor).

**Timeline:** 
- Phase 1 (Immediate): 25 minutes - .gitignore, pre-commit hook, health check script, gc scheduling
- Phase 2 (This Week): 55 minutes - Monitoring scripts (crash pattern, resources)
- Phase 3 (Next Week): 60 minutes - Integration, testing, documentation

**Total Effort:** ~2.5 hours over 2 weeks  
**Risk:** Very Low  
**Impact:** High - Prevents all repository bloat OOM crashes

---

**Status:** ✅ Ready for implementation  
**Next Steps:** Execute Phase 1 immediate actions  
**Tracking:** Update bead domchk-714e9e63 with implementation status

---

**Document Version:** 1.0  
**Created:** 2026-09-01  
**Author:** Claude Code Agent  
**Review Status:** Ready for implementation
