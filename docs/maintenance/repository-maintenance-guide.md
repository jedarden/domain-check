# Repository Maintenance Guide

**Purpose:** Prevent repository bloat and maintain system stability  
**Last Updated:** 2026-09-01  
**Status:** ✅ Active

---

## Quick Reference

### Daily Operations
```bash
# Pre-flight health check before starting work
./scripts/preflight-health-check.sh
```

### Weekly Maintenance
```bash
# Check repository health
./scripts/check-repo-health.sh

# Run cleanup if repository >500MB
./scripts/safe-git-gc.sh --full
```

### Emergency (If Repository >5GB)
```bash
# Immediate cleanup
./scripts/safe-git-gc.sh --full

# Monitor in another terminal
./scripts/safe-git-gc-monitor.sh --watch
```

---

## Repository Size Thresholds

| Size | Status | Action |
|------|--------|--------|
| <500MB | ✅ Healthy | None required |
| 500MB-1GB | ⚠️ Warning | Plan cleanup soon |
| 1GB-5GB | 🚨 Critical | Run cleanup immediately |
| >5GB | 🆘 Emergency | Run cleanup NOW |

---

## Warning Signs of Repository Bloat

**Early Warning Signs:**
- Repository size approaching 1GB
- Git operations becoming slow
- Loose objects >500MB

**Critical Symptoms:**
- Exit code -1 (SIGKILL) during git operations
- Repository size >5GB
- Routine git operations trigger OOM
- Multiple crashes with exit code -1

---

## Prevention Checklist

**Daily:**
- [ ] Run pre-flight health check before starting work
- [ ] Check available memory (>10GB required)

**Weekly:**
- [ ] Run `./scripts/check-repo-health.sh`
- [ ] Review repository size trends
- [ ] Run cleanup if size >500MB

**Monthly:**
- [ ] Comprehensive repository audit
- [ ] Review git history for large files
- [ ] Update monitoring thresholds if needed

---

## Key Scripts

| Script | Purpose | Usage |
|--------|---------|-------|
| `preflight-health-check.sh` | Validate system before tasks | Before every agent task |
| `check-repo-health.sh` | Repository size and object check | Weekly health monitoring |
| `safe-git-gc.sh` | Memory-limited garbage collection | When cleanup needed |
| `safe-git-gc-monitor.sh` | Monitor gc progress | During gc operations |

---

## Important Links

**Crash Investigation:**
- [bf-1s6c3 Investigation Summary](../bf-1s6c3-investigation-summary.md) - Repository bloat crash (18GB → 138MB)
- [Crash Response Guide](../crash-response-guide.md) - Quick crash classification

**Detailed Documentation:**
- [Repository Maintenance Recommendations](./repository-maintenance-recommendations.md) - Comprehensive guide
- [Cleanup and Recovery Procedures](../cleanup-and-recovery-procedures.md) - Emergency procedures
- [Crash Mitigation Strategies](../crash-mitigation-strategies.md) - Prevention strategies

---

## Common Issues and Solutions

### Repository Keeps Growing
```bash
# Check current state
./scripts/check-repo-health.sh
du -sh .git/objects/* | sort -rh

# Solution: Run cleanup
./scripts/safe-git-gc.sh --full
```

### Git GC Fails with OOM
```bash
# Check available memory
free -h

# Run with memory limit
SAFE_GC_MEMORY_MAX=2g ./scripts/safe-git-gc.sh --full
```

### Pre-Flight Check Fails
Common fixes:
- **Gateway unavailable:** Wait for service recovery
- **Insufficient memory:** Close applications or add RAM
- **Repository bloated:** Run `./scripts/safe-git-gc.sh --full`

---

## Success Criteria

Repository is healthy when:
- ✅ Repository size <500MB
- ✅ Loose objects <100 count
- ✅ No fsck errors (`git fsck --full`)
- ✅ Pre-flight health check passes
- ✅ No OOM crashes during git operations

---

## Background

The bf-1s6c3 crash (2026-08-12) demonstrated that repository bloat (18GB with 17GB loose objects) can trigger systematic OOM failures. Following cleanup (18GB → 138MB, 99.2% reduction), the task completed successfully and no similar crashes have occurred for 16+ days.

**Key Takeaway:** Proactive maintenance prevents 70% of infrastructure crashes.

---

**Need Help?** See [Repository Maintenance Recommendations](./repository-maintenance-recommendations.md) for comprehensive guidance or [Crash Response Guide](../crash-response-guide.md) for crash investigation procedures.
