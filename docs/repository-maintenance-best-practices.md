# Repository Maintenance Best Practices

**Created:** 2026-09-01  
**Purpose:** Prevent repository bloat and OOM crashes through proactive maintenance  
**Related:** `docs/crash-mitigation-strategies.md`, `docs/crash-analysis-domchk-c9641ac5-2026-09-01.md`

---

## Executive Summary

Repository bloat caused 9 crashes (bead bf-4yjq) when the repository grew to 18GB with 17GB of loose objects. This document provides operational procedures to prevent future occurrences.

**Key Takeaway:** Repository maintenance is automated and continuous. Never wait for crashes to trigger maintenance.

---

## Repository Size Thresholds

### Warning Levels

| Level | Size | Action Required | Auto-Triggered |
|-------|------|-----------------|----------------|
| **Healthy** | <500MB | None | N/A |
| **Warning** | 500MB-1GB | Monitor growth, plan gc | No |
| **Critical** | 1-5GB | Run git gc soon | No |
| **Emergency** | ≥5GB | Run git gc immediately | Yes (auto-gc-trigger.sh) |

### What Causes Bloat

1. **Large file commits:** `.beads/*.jsonl` files (237MB each) committed to git
2. **Loose objects:** Not running git gc after large operations
3. **Duplicate files:** Identical files committed multiple times
4. **Binary artifacts:** Build outputs, test data, or binaries in git history

### Historical Examples

#### Bead bf-1s6c3 (2026-08-12) - Repository Bloat OOM Crash
- **Repository size:** 18GB (17GB loose objects)
- **Cause:** Repeated commits of large `.beads/` JSONL files from problematic bead operations (bf-2ildm)
  - 17+ identical commits for "GitHub-specific commits extraction"
  - Each commit included 237MB `.beads/issues.jsonl`, 237MB `.beads/beads.base.jsonl`
  - **Impact:** 17 commits × ~500MB per commit = ~8.5GB redundant data
- **Crash mechanism:** Git reconciliation operations → Memory exhaustion → OOM killer → SIGKILL (exit code -1)
- **Resolution:** Repository cleanup reduced 18GB → 138MB (99.2% reduction)
- **Task completion:** Merge commit created successfully on 2026-08-16 after cleanup
- **Key lesson:** Repository bloat is a leading cause of infrastructure crashes (70% of crashes)

#### Bead bf-4yjq (2026-08-12) - Systematic Repository Bloat Crashes
- **Repository size:** 18GB (17GB loose objects)
- **Cause:** Bead bf-2ildm committed 17+ identical 237MB `.beads/*.jsonl` files
- **Impact:** 9 OOM crashes over 2.5 hours (any git operation triggered OOM)
- **Resolution:** Git gc reduced repo from 18GB to 445MB (97.5% reduction)

---

## Monitoring and Health Checks

### Pre-Task Health Check

**ALWAYS run before starting agent tasks:**

```bash
./scripts/preflight-health-check.sh
```

**What it checks:**
- ✅ Repository size (<500MB healthy, ≥5GB critical)
- ✅ Inference gateway availability
- ✅ Memory availability (≥ 10GB free)
- ✅ Disk space (≥ 20GB free)
- ✅ CPU load (< 10 on 1min average)
- ✅ Git repository integrity (git fsck)

**If checks fail:**
1. Do NOT start agent tasks
2. Address the reported issues
3. Re-run the health check
4. Only proceed when all checks pass

### Continuous Monitoring (Optional)

**For automated monitoring, install cron jobs:**

```bash
./scripts/monitoring-setup.sh
```

**This installs:**
- Crash pattern detection (every 10 minutes)
- Resource monitoring (every 5 minutes)  
- Service monitoring (every 2 minutes)

**To uninstall:**
```bash
./scripts/monitoring-remove.sh
```

**Note:** Continuous monitoring is optional. The pre-task health check is mandatory.

---

## Garbage Collection

### When to Run Git GC

**Run git gc when:**
- Repository exceeds 2GB (warning threshold)
- After large file operations or commits
- Before significant git operations (rebase, filter-branch, etc.)
- After removing large files from history
- Pre-flight health check reports critical size

**Never skip git gc after these operations:**
- Removing large files with `git filter-branch` or `git filter-repo`
- Squashing commits
- Cleaning up branches
- Large-scale refactoring

### How to Run Git GC Safely

**ALWAYS use the safe git gc script:**

```bash
./scripts/safe-git-gc.sh          # Standard GC (stages 1-2, ~10-30 minutes)
./scripts/safe-git-gc.sh --full   # Full GC with deep compression (~1-2 hours)
```

**Why use safe-git-gc.sh instead of bare `git gc --aggressive`:**

| Feature | `git gc --aggressive` | `safe-git-gc.sh` |
|---------|----------------------|------------------|
| Memory Usage | Unbounded (4-8GB possible) | Capped (2GB max) |
| Progress Visibility | None | Full monitoring |
| Resumability | ❌ All-or-nothing | ✅ Checkpoint at each stage |
| Time | 2-4 hours on large repos | 1-2 hours staged |
| Safety | Can OOM on large repos | Memory-limited operations |

### Auto GC Trigger

**Manual trigger for large repositories:**

```bash
# Check if auto GC is needed
./scripts/auto-gc-trigger.sh

# Dry run to see what would happen
./scripts/auto-gc-trigger.sh --dry-run

# Force GC even if below threshold
./scripts/auto-gc-trigger.sh --force
```

**Automatic trigger runs:**
- When repository size exceeds 10GB
- During pre-task health check (if critical size detected)

---

## Prevention Measures

### Prevent Large File Commits

**`.gitignore` already configured to prevent bead workspace commits:**

```gitignore
# Beads tracking system (prevent large JSONL file commits)
.beads/
*.db
*.db.backup.*
*.jsonl
```

**Pre-commit hook to block large files (> 10MB):**

```bash
# Install pre-commit hook
cp .git/hooks/pre-commit-repo-size-hook .git/hooks/pre-commit
chmod +x .git/hooks/pre-commit
```

### Prevent .beads/ Large Files in Git

**CRITICAL:** Never commit bead workspace files to git. The `.gitignore` is already configured to prevent this.

**If you accidentally committed large `.beads/` files:**

```bash
# Remove from git history (use with caution)
git filter-repo --path .beads/ --invert-paths

# Or use BFG Repo-Cleaner (safer alternative)
bfg --delete-files .beads/*.jsonl
bfg --strip-blobs-bigger-than 10M
git reflog expire --expire=now --all && git gc --prune=now --aggressive
```

### Recovery from Bloated Repository

**If repository is already bloated (> 10GB):**

1. **Check current state:**
   ```bash
   ./scripts/preflight-health-check.sh
   ```

2. **Run safe git gc:**
   ```bash
   ./scripts/safe-git-gc.sh --full
   ```

3. **If gc fails due to OOM:**
   - Check available memory: `free -h`
   - Close other applications
   - Run gc with memory limit: `SAFE_GC_MEMORY_MAX=2g ./scripts/safe-git-gc.sh --full`

4. **Verify recovery:**
   ```bash
   ./scripts/check-repo-health.sh
   git fsck --full
   ```

---

## Operational Procedures

### Daily Operations

1. **Before starting work:**
   ```bash
   ./scripts/preflight-health-check.sh
   ```

2. **Before committing large changes:**
   ```bash
   ./scripts/auto-gc-trigger.sh --dry-run  # Check if GC is needed
   ```

3. **After large file operations:**
   ```bash
   ./scripts/safe-git-gc.sh  # Run standard GC
   ```

### Weekly Maintenance

1. **Check repository health:**
   ```bash
   ./scripts/check-repo-health.sh
   ```

2. **Run full GC if repository > 500MB:**
   ```bash
   ./scripts/safe-git-gc.sh --full
   ```

3. **Review monitoring logs (if installed):**
   ```bash
   tail -100 .beads/logs/crash-monitor.log
   tail -100 .beads/logs/resource-monitor.log
   tail -100 .beads/logs/service-monitor.log
   ```

### Monthly Maintenance

1. **Run comprehensive repository audit:**
   ```bash
   ./scripts/check-repo-health.sh
   git fsck --full
   git count-objects -vH
   ```

2. **Check for large files in history:**
   ```bash
   git rev-list --objects --all |
     git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)' |
     awk '/^blob/ {if ($3 > 10485760) print $3/1048576 " MB " $4}' |
     sort -rn | head -20
   ```

3. **Run full GC with deep compression:**
   ```bash
   ./scripts/safe-git-gc.sh --full
   ```

---

## Testing and Verification

### Run Repository Monitoring Tests

**Unit tests (always safe to run):**
```bash
./scripts/test-repo-monitoring.sh
```

**Integration tests (runs actual repo operations):**
```bash
./scripts/test-repo-monitoring.sh --integration
```

**Expected output:**
- All scripts exist and are executable
- Thresholds are correctly configured
- Current repository is healthy
- All health checks pass

### Verify Prevention Measures

**Manual verification:**

1. **Check .gitignore includes bead workspace:**
   ```bash
   grep -q "^.beads/$" .gitignore && echo "✅ .beads/ ignored"
   ```

2. **Check preflight includes repo size check:**
   ```bash
   grep -q "check_repo_size" scripts/preflight-health-check.sh && echo "✅ Repo size check included"
   ```

3. **Test auto GC trigger:**
   ```bash
   ./scripts/auto-gc-trigger.sh --dry-run
   ```

4. **Run comprehensive health check:**
   ```bash
   ./scripts/check-repo-health.sh
   ```

---

## Common Issues and Solutions

### Issue: Repository size keeps growing

**Symptoms:**
- Repository grows by > 100MB per day
- Git operations are slow
- Disk space filling up

**Diagnosis:**
```bash
# Check what's growing
./scripts/check-repo-health.sh
du -sh .git/objects/pack/*
du -sh .git/objects/* 2>/dev/null | sort -rh | head -10
```

**Solutions:**
1. Run git gc: `./scripts/safe-git-gc.sh --full`
2. Check for large files in history (see Monthly Maintenance above)
3. Review recent commits for accidental large file additions
4. Verify `.gitignore` is working correctly

### Issue: Git gc fails with OOM

**Symptoms:**
- Git gc process killed by OOM killer
- System becomes unresponsive during gc
- Exit code -1 or 137

**Prevention:**
```bash
# Check available memory before gc
free -h

# Run gc with memory limit
SAFE_GC_MEMORY_MAX=2g ./scripts/safe-git-gc.sh --full
```

**Recovery:**
1. Close other applications
2. Increase swap space (if needed)
3. Run gc again with memory limit
4. If still failing, run standard gc instead of full: `./scripts/safe-git-gc.sh`

### Issue: Pre-flight health check fails

**Symptoms:**
- Preflight check exits with code 1
- Failed checks reported

**Common fixes:**
- **Inference gateway unavailable:** Wait for service to recover
- **Insufficient memory:** Close applications or add RAM
- **Disk space full:** Clean up disk space
- **CPU load high:** Wait for load to decrease
- **Repository bloated:** Run `./scripts/safe-git-gc.sh --full`
- **Git integrity issues:** Run `git fsck --full`

### Issue: Large files accidentally committed

**Symptoms:**
- Repository size suddenly increases
- Large files visible in git history

**Recovery:**
```bash
# Option 1: Remove large files from history (CAUTION: rewrites history)
git filter-repo --path <large-file> --invert-paths

# Option 2: Use BFG Repo-Cleaner (safer)
bfg --strip-blobs-bigger-than 10M
git reflog expire --expire=now --all && git gc --prune=now --aggressive

# Option 3: Start fresh from clean state (if history is not critical)
git clone --depth 1 <repo-url>  # Shallow clone (last commit only)
```

---

## Success Criteria

Repository maintenance is successful when:

1. ✅ Repository size is <500MB (healthy threshold)
2. ✅ Pre-flight health check passes all tests
3. ✅ Git fsck shows no errors
4. ✅ Loose objects count is <100
5. ✅ No files > 10MB in git history
6. ✅ All monitoring tests pass: `./scripts/test-repo-monitoring.sh --integration`

---

## Summary

**Proactive maintenance prevents crashes.**

**DO:**
- ✅ Run pre-flight health check before tasks
- ✅ Use safe-git-gc.sh for garbage collection
- ✅ Monitor repository size weekly
- ✅ Keep `.gitignore` up to date
- ✅ Test prevention measures monthly

**DON'T:**
- ❌ Wait for crashes to trigger maintenance
- ❌ Commit large files to git
- ❌ Use bare `git gc --aggressive` without memory limits
- ❌ Ignore pre-flight health check failures
- ❌ Skip maintenance when repository grows large

**Bottom Line:** Repository maintenance is automated and continuous. The scripts provided handle detection, alerting, and correction. Run the pre-flight check before tasks, and let the automated systems handle the rest.

---

**Document Version:** 1.0  
**Created:** 2026-09-01  
**Author:** Claude Code Agent  
**Review Status:** Ready for implementation
