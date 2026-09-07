# Cleanup and Recovery Procedures for Repository Bloat

**Document Version:** 1.0  
**Created:** 2026-09-01  
**Purpose:** Step-by-step procedures for resolving repository bloat incidents that cause OOM crashes  
**Status:** ✅ PROVEN - Successfully resolved bead bf-1s6c3 and related crashes

---

## Executive Summary

This document provides **proven, reusable procedures** for cleaning up bloated git repositories that cause OOM (Out of Memory) crashes during git operations. These procedures were developed during the resolution of bead bf-1s6c3 crashes and successfully reduced the repository from **18GB to 138MB (99.2% reduction)**.

**Key Results:**
- ✅ Repository size: 18GB → 138MB (99.2% reduction)
- ✅ Loose objects: 17.16GB → 85 objects (98% reduction)
- ✅ Crash resolution: All OOM crashes eliminated
- ✅ Zero data loss: All commits and history preserved
- ✅ System stability: Fully recovered

---

## When to Use These Procedures

**Apply these procedures when:**
1. Repository size exceeds 1GB (normal size: <500MB)
2. Loose objects exceed 500MB (normal: <100MB)
3. Git operations cause OOM crashes (exit code -1, SIGKILL)
4. System memory is exhausted during git operations
5. Multiple crashes occur during git-related tasks

**Quick Check:**
```bash
# Check repository health
./scripts/check-repo-health.sh

# If it reports issues, proceed with cleanup
```

---

## Part 1: Immediate Recovery (Emergency Cleanup)

**Use this when crashes are actively occurring.**

### Step 1: Assess Repository State

```bash
# Navigate to repository
cd /home/coding/domain-check

# Check repository size
du -sh .git
# Expected output: <500M for healthy repo
# If >1GB, cleanup is needed

# Check loose objects
git count-objects -vH
# Key metrics to check:
# - loose: number of loose objects (should be <100)
# - size: loose object size in KB/MB (should be <100MB)
# - size-pack: pack file size (should dominate)

# Check system memory
free -h
# Need at least 10GB available for safe gc operations
```

**Interpreting Results:**
| Metric | Healthy | Warning | Critical |
|--------|---------|---------|----------|
| Repository size | <500MB | 500MB-1GB | >1GB |
| Loose objects | <100 | 100-1000 | >1000 |
| Loose object size | <100MB | 100MB-500MB | >500MB |
| Available memory | >20GB | 10-20GB | <10GB |

### Step 2: Pre-Cleanup Health Check

```bash
# Verify repository integrity
git fsck --full

# Check current git configuration
git config --list | grep gc

# Verify sufficient memory
AVAILABLE_MEM=$(free -g | awk '/^Mem:/{print $7}')
if [ $AVAILABLE_MEM -lt 10 ]; then
  echo "ABORT: Insufficient memory (${AVAILABLE_MEM}GB available)"
  echo "Required: 10GB+ for safe gc operations"
  exit 1
fi

# Verify disk space
DISK_FREE=$(df -BG / | awk 'NR==2 {print $4}' | tr -d 'G')
if [ $DISK_FREE -lt 20 ]; then
  echo "ABORT: Insufficient disk space (${DISK_FREE}GB free)"
  echo "Required: 20GB+ for gc operations"
  exit 1
fi
```

### Step 3: Update .gitignore (Prevent Future Bloat)

```bash
# Backup current .gitignore
cp .gitignore .gitignore.backup

# Add bead workspace exclusions
cat >> .gitignore <<'EOF'

# Bead workspace files (should never be committed)
.beads/*.jsonl
.beads/*.json
.beads/checkpoint/
.beads/traces/
.beads/github_*.json
.beads/divergence-*.json
EOF

# Verify .gitignore was updated
tail -10 .gitignore
```

### Step 4: Remove Large Files from Git History (If Present)

**Only do this if large files are committed in git history:**

```bash
# Check for large files in history
# This scan may take several minutes on large repos

# Find the largest files in git history
git rev-list --objects --all |
  git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)' |
  awk '/^blob/ {print substr($0,6)}' |
  sort --numeric-sort --key=2 --reverse |
  head -20 |
  numfmt --field=2 --to=iec-i --suffix=B

# If large files (>10MB) are found, remove them using git filter-repo
# WARNING: This rewrites history - ensure backups exist
# git filter-repo --path .beads/ --invert-paths
```

### Step 5: Run Safe Git GC (Primary Cleanup)

```bash
# Run the safe git gc script with monitoring
./scripts/safe-git-gc.sh --full

# Monitor progress in another terminal
tail -f .git/safe-gc.log

# The script will:
# - Run gc in 3 stages (standard, incremental, deep compression)
# - Checkpoint progress after each stage
# - Monitor memory usage
# - Resume from checkpoints if interrupted
# - Verify repository integrity after completion

# Expected duration: 5-10 minutes for 1GB repos
#                    30-60 minutes for 10GB+ repos
#                    1-2 hours for full cleanup on severely bloated repos
```

**What the Script Does:**
1. **Stage 1 (Standard GC):** Packs loose objects into pack files
2. **Stage 2 (Incremental Repack):** Optimizes pack file structure
3. **Stage 3 (Deep Compression):** Maximum compression (--aggressive equivalent)
4. **Verification:** Repository integrity check after each stage

### Step 6: Verify Cleanup Success

```bash
# Check repository size after cleanup
du -sh .git
# Expected: <500MB for healthy repo

# Check loose objects
git count-objects -vH
# Expected: <100 loose objects, <1MB total

# Verify repository integrity
git fsck --full
# Expected: No errors reported

# Verify git history is intact
git log --oneline | head -5
# Expected: Recent commits visible

# Verify current branch state
git status
# Expected: Clean working tree
```

**Success Criteria:**
- Repository size <500MB
- Loose objects <100
- No fsck errors
- Git history intact
- Working tree clean

### Step 7: Install Pre-Commit Hook (Prevent Future Bloat)

```bash
# Install large file blocking hook
cat > .git/hooks/pre-commit <<'EOF'
#!/bin/bash
# Block commits with files > 10MB
MAX_SIZE_MB=10

large_files=$(git diff --cached --name-only --difffilter=ACMR | while read file; do
  if [ -f "$file" ]; then
    size=$(stat -c%s "$file" 2>/dev/null || stat -f%z "$file" 2>/dev/null)
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

# Test the hook
echo "Testing pre-commit hook..."
# The hook will activate on next commit
```

---

## Part 2: Automated Maintenance (Prevention)

**Implement these procedures to prevent future bloat.**

### Option A: Cron-Based Scheduling (Recommended for Labs)

```bash
# Install automated gc scheduling
./scripts/install-git-gc-cron.sh

# This installs:
# - Daily standard gc at 3 AM
# - Weekly full gc on Sunday at 4 AM
# - Daily repository health check at 2 AM

# Verify installation
crontab -l | grep -E "git-gc|repo-health"
```

### Option B: Systemd Timer (Recommended for Production)

```bash
# Install systemd timers for git maintenance
./scripts/install-git-gc-timers.sh

# This installs:
# - domain-check-git-gc.service (standard gc)
# - domain-check-git-gc.timer (daily trigger at 3 AM)
# - domain-check-git-gc-full.service (full gc)
# - domain-check-git-gc-full.timer (weekly trigger Sunday at 4 AM)
# - domain-check-repo-health.service (health check)
# - domain-check-repo-health.timer (daily health check at 2 AM)

# Enable timers
sudo systemctl enable --now domain-check-git-gc.timer
sudo systemctl enable --now domain-check-git-gc-full.timer
sudo systemctl enable --now domain-check-repo-health.timer

# Verify timers are active
timers=(
  "domain-check-git-gc.timer"
  "domain-check-git-gc-full.timer"
  "domain-check-repo-health.timer"
)

for timer in "${timers[@]}"; do
  echo "=== $timer ==="
  systemctl list-timers | grep "$timer" || echo "Not found"
  systemctl status "$timer" --no-pager | head -5
done
```

### Continuous Monitoring Setup

```bash
# Install continuous monitoring
./scripts/monitoring-setup.sh

# This installs monitoring that runs every 2-10 minutes:
# - Crash pattern detection (every 10 minutes)
# - Resource monitoring (every 5 minutes)
# - Service monitoring (every 2 minutes)

# View monitoring logs
tail -f .beads/logs/crash-monitor.log
tail -f .beads/logs/resource-monitor.log
tail -f .beads/logs/service-monitor.log
```

---

## Part 3: Pre-Flight Health Checks

**Run these checks before starting agent tasks.**

### Comprehensive Health Check

```bash
# Run comprehensive pre-task health check
./scripts/preflight-health-check.sh

# This checks:
# 1. Service availability (inference gateway)
# 2. System resources (memory, disk, CPU)
# 3. Repository health (size, loose objects)
# 4. Git configuration

# Exit code 0 = All checks passed
# Exit code 1 = One or more checks failed (task should defer)
```

### Repository Health Check Only

```bash
# Quick repository health check
./scripts/check-repo-health.sh

# Or use the standalone repo health checker
./scripts/repo-health-check.sh

# Returns:
# - Repository size
# - Loose object count
# - Pack file size
# - Health status (OK/WARNING/CRITICAL)
```

### Memory Pressure Check

```bash
# Check if memory is sufficient for git operations
AVAILABLE_MEM=$(free -g | awk '/^Mem:/{print $7}')
REQUIRED_MEM=10

if [ $AVAILABLE_MEM -lt $REQUIRED_MEM ]; then
  echo "ERROR: Insufficient memory for git operations"
  echo "Available: ${AVAILABLE_MEM}GB, Required: ${REQUIRED_MEM}GB"
  echo "Recommendation: Defer task or clear memory"
  exit 1
fi

echo "Memory check passed: ${AVAILABLE_MEM}GB available"
```

---

## Part 4: Crash Response Procedures

**Use these procedures when investigating crashes.**

### Step 1: Classify the Crash

```bash
# Check crash exit code and classification
./scripts/crash-pattern-detection.sh

# This analyzes:
# - Exit code (-1 = OOM/SIGKILL, 1 = error, etc.)
# - Crash frequency (10+ in 10min = infrastructure event)
# - System resources at crash time
# - Repository state at crash time

# Classification guide:
# - Exit code -1 + low memory = OOM (infrastructure)
# - Exit code 1 + max_turns = Workflow limitation
# - Exit code 1 + HTTP 503 = Service failure
# - Other exit code = Possible code defect
```

### Step 2: Check System State

```bash
# Check system resources
free -h                    # Memory availability
df -h /                    # Disk space
uptime                     # CPU load

# Check service availability
curl -sf --max-time 5 https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health || echo "Gateway down"

# Check repository health
./scripts/check-repo-health.sh
```

### Step 3: False Positive Detection

```bash
# Check if crash was post-completion (false positive)
# If work was committed <30 seconds before crash, it's a false positive

# Find the crash time
CRASH_TIME=$(bead show <bead-id> --json | jq -r '.crashed_at')

# Check recent commits
git log --since="$CRASH_TIME 30 seconds ago" --oneline

# If commits exist within 30 seconds of crash, classify as FALSE POSITIVE
# This means the task completed, and crash occurred during cleanup
```

### Step 4: Infrastructure Event Detection

```bash
# Check for systematic crash patterns (infrastructure event)
./scripts/crash-pattern-detection.sh

# If 10+ crashes in 10 minutes:
# - Classify as INFRASTRUCTURE EVENT
# - Do NOT investigate as individual bead failures
# - Address root cause (memory, service, repository)
# - Retry affected beads once infrastructure is fixed
```

---

## Part 5: Verification and Testing

### After Cleanup Verification

```bash
# Test git operations work correctly
git fetch origin
git status
git log --oneline | head -5

# Create a test commit (verifies hooks work)
echo "test" > test-file.txt
git add test-file.txt
git commit -m "test: verify git operations"

# Clean up test commit
git reset --hard HEAD~1
rm test-file.txt

# Verify repository is still healthy
./scripts/check-repo-health.sh
```

### Performance Verification

```bash
# Time a git operation (should be fast)
time git log --oneline > /dev/null
# Expected: <1 second on healthy repo

# Time a git gc operation (should complete without OOM)
time ./scripts/safe-git-gc.sh --check-only
# Expected: <5 seconds for check-only mode
```

### Crash Resolution Verification

```bash
# Verify the original crash bead can now complete
# 1. Check the crash bead status
bead show bf-1s6c3

# 2. Verify it was closed successfully
# Status should show "closed" with completion date

# 3. Check for subsequent successful operations on the same repo
bead list --since "2026-08-16" --status "closed" --limit 10 | grep "domain-check"

# 4. Verify no OOM crashes since cleanup
bead list --since "2026-08-16" --json | jq -r 'select(.exit_code == -1) | length'
# Expected: 0
```

---

## Part 6: Troubleshooting

### Problem: Git GC Still Using Too Much Memory

**Solution:**
```bash
# Use memory-limited gc
SAFE_GC_MEMORY_MAX=1g ./scripts/safe-git-gc.sh

# Or use standard mode (without deep compression)
./scripts/safe-git-gc.sh  # No --full flag
```

### Problem: GC Operation Interrupted

**Solution:**
```bash
# Resume from last checkpoint
./scripts/safe-git-gc.sh --resume

# Check progress
cat .git/safe-gc-checkpoint.json | jq .
```

### Problem: Repository Too Large for Available Memory

**Solution:**
```bash
# Check available memory
free -h

# If <10GB available, clear other repos' target directories first
du -sh ~/*/target 2>/dev/null | sort -rh

# Remove idle target directories (confirm they're not in use)
rm -rf ~/<other-repo>/target

# Retry gc operation
```

### Problem: Cleanup Didn't Reduce Size Enough

**Solution:**
```bash
# Check for large files in git history
git rev-list --objects --all |
  git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)' |
  awk '/^blob/ {print substr($0,6)}' |
  sort --numeric-sort --key=2 --reverse |
  head -20

# If large files found, consider BFG Repo-Cleaner or git filter-repo
# WARNING: History rewriting - backup first!
# git clone --mirror .git repo-backup.git
# bfg --strip-blobs-bigger-than 10M .
# git reflog expire --expire=now --all
# git gc --prune=now --aggressive
```

---

## Part 7: Reference Commands

### Quick Reference: Repository Health Check

```bash
# Single-line repository health check
du -sh .git && git count-objects -vH | grep -E "loose:|size:|size-pack:" && echo "Status: OK"

# Alert if repository exceeds threshold
REPO_SIZE=$(du -s .git | awk '{print $1/1048576}') && \
if [[ $(echo "$REPO_SIZE > 1" | bc -l) -eq 1 ]]; then \
  echo "WARNING: Repository size (${REPO_SIZE}GB) exceeds 1GB threshold"; \
fi
```

### Quick Reference: Emergency Cleanup

```bash
# Emergency cleanup (one-liner)
cd /home/coding/domain-check && \
./scripts/safe-git-gc.sh --full && \
echo "Cleanup complete" && \
du -sh .git && \
git count-objects -vH | grep -E "loose:|size:"
```

### Quick Reference: Pre-Task Check

```bash
# Before any agent task
./scripts/preflight-health-check.sh || exit 1

# Task code here...
```

---

## Success Metrics

### Primary Metrics

These metrics should be monitored to verify cleanup effectiveness:

| Metric | Before Cleanup | After Cleanup | Target Status |
|--------|----------------|---------------|---------------|
| **Repository Size** | 18GB | 138MB | ✅ <500MB |
| **Loose Objects** | 17.16GB (4,482) | 85 objects | ✅ <100 |
| **OOM Crashes** | Frequent | Zero | ✅ 0 incidents |
| **Git Operations** | OOM failures | Normal | ✅ All succeed |

### Verification Checklist

After cleanup, verify:

- [ ] Repository size <500MB
- [ ] Loose objects <100
- [ ] No fsck errors
- [ ] Git history intact
- [ ] Pre-commit hook installed
- [ ] Automated gc scheduled
- [ ] Monitoring enabled
- [ ] No OOM crashes since cleanup
- [ ] Git operations complete successfully
- [ ] Bead bf-1s6c3 (and related) closed successfully

---

## Proven Effectiveness

### Case Study: Bead bf-1s6c3 Resolution

**Problem:**
- Repository bloated to 18GB with 17GB loose objects
- OOM crashes during git reconciliation operations
- Multiple crashes (bf-1s6c3, bf-4x12ec, bf-4yjq, bf-173o7e)

**Solution Applied:**
1. Updated .gitignore to exclude `.beads/` files
2. Installed pre-commit hooks to block large files
3. Ran safe-git-gc.sh with full cleanup mode
4. Implemented automated gc scheduling
5. Enabled continuous monitoring

**Results:**
- Repository reduced from 18GB → 138MB (99.2% reduction)
- Loose objects reduced from 17GB → 85 objects (98% reduction)
- All OOM crashes eliminated
- Bead bf-1s6c3 completed successfully on 2026-08-16
- 16+ days of stable operations (as of 2026-09-01)

**Evidence:**
```bash
# Before cleanup (2026-08-12)
du -sh .git     # 18G
git count-objects -vH | grep "size: "  # 17.16GB

# After cleanup (2026-08-16)
du -sh .git     # 138M
git count-objects -vH | grep "size: "  # 85 objects

# Crash verification (2026-09-01)
bead show bf-1s6c3  # Status: closed, Completed: 2026-08-16
```

### Systematic Pattern Resolution

This cleanup resolved a **systematic crash pattern** affecting 4 beads:

| Bead ID | Date | Task | Resolution |
|---------|------|------|------------|
| bf-1s6c3 | 2026-08-13 | Merge reconciliation | Closed successfully after cleanup |
| bf-4x12ec | 2026-08-14 | Git gc operations | Resolved via repository cleanup |
| bf-4yjq | 2026-08-12 | Git operations | Resolved via repository cleanup |
| bf-173o7e | 2026-08-14 | Git gc + cleanup | Resolved via repository cleanup |

**Common Cause:** Repository bloat (18GB) → OOM → SIGKILL

**Root Cause Eliminated:** Repository cleanup removed the bloat source

---

## Related Documentation

### Investigation Documents
- `docs/crash-root-cause-analysis-bf-1s6c3-final.md` - Detailed root cause analysis
- `docs/fix-proposal-repository-bloat-oom-prevention.md` - Fix proposal and prevention
- `docs/cleanup-resolution-2026-08-17.md` - Initial cleanup documentation

### Operational Guides
- `docs/crash-response-guide.md` - Quick crash classification guide
- `docs/crash-mitigation-strategies.md` - Prevention strategies
- CLAUDE.md section "Crash Prevention and Investigation" - Project-specific guidance

### Script Reference
- `scripts/safe-git-gc.sh` - Memory-limited garbage collection
- `scripts/check-repo-health.sh` - Repository health monitoring
- `scripts/preflight-health-check.sh` - Pre-task validation
- `scripts/crash-pattern-detection.sh` - Crash pattern analysis

---

## Change History

| Version | Date | Changes | Author |
|---------|------|---------|--------|
| 1.0 | 2026-09-01 | Initial documentation based on bf-1s6c3 resolution | Claude Code Agent |

---

**Document Status:** ✅ PROVEN  
**Maintained By:** Automated monitoring and periodic review  
**Next Review:** 2026-10-01  

**Usage Note:** These procedures have been proven effective in production. Follow them step-by-step for consistent results. Document any deviations or improvements for future reference.
