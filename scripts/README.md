# Repository Health Scripts

This directory contains scripts for monitoring and maintaining repository health to prevent signal -1 crashes caused by repository bloat.

## Background

These scripts were created in response to a comprehensive root cause analysis (bead bf-ku6mmu) of agent signal -1 crashes. The crashes were caused by repository bloat (18GB with 17GB of loose git objects) triggering the Linux OOM killer during git operations.

### Root Cause Summary

- **Signal -1** = **SIGKILL (signal 9)** from Linux **OOM killer**
- **Repository State**: 18GB total (should be <500MB) with 17.16GB loose objects
- **Contributing Pattern**: 17+ identical commits containing 237MB `.beads/` JSONL files
- **Crash Count**: 9 systematic crashes over 2.5 hours (100% SIGKILL pattern)

## Scripts

### repo-health-check.sh

Monitors repository health and alerts when thresholds are exceeded.

**Usage:**
```bash
./scripts/repo-health-check.sh
```

**Checks performed:**
- Repository size (warn: 500MB, critical: 1GB)
- Loose objects count (warn: 10k, critical: 50k)
- Pack file ratio (should be majority packed)
- Large files in working directory (>10MB)
- .gitignore protections for .beads/ and *.jsonl

**Exit codes:**
- `0` - Repository is healthy
- `1` - Repository is degraded (warning)
- `2` - Repository is critical (risk of signal -1 crashes)

### setup-git-gc-config.sh

Configures git garbage collection settings to prevent repository bloat.

**Usage:**
```bash
./scripts/setup-git-gc-config.sh
```

**Settings configured:**
- `gc.auto = 256` - More frequent GC (default: 6700)
- `gc.autoPackLimit = 10` - More aggressive consolidation (default: 50)
- `gc.aggressiveWindow = 1.hour` - Aggressive compression window
- `gc.pruneExpire = 2.weeks.ago` - Prune old objects
- `repack.writeBitmaps = true` - Better clone performance

## Integration with CI/CD

These scripts can be integrated into CI/CD pipelines:

```bash
# Run health check before git operations
./scripts/repo-health-check.sh || exit 1

# Configure git settings in CI environment
./scripts/setup-git-gc-config.sh
```

## Manual Remediation

If repository health check reports critical status, execute:

```bash
# Aggressive garbage collection (may take hours on large repos)
git gc --aggressive --prune=now

# Verify cleanup
./scripts/repo-health-check.sh
```

## Pre-commit Protection

The `.git/hooks/pre-commit` script automatically prevents large file commits:

- **Max file size**: 10MB
- **Max total commit**: 50MB
- **Blocks .beads/** files that should be .gitignored

## Monitoring Recommendations

Run repository health checks periodically:

```bash
# Daily cron job
0 0 * * * /path/to/domain-check/scripts/repo-health-check.sh

# Before major git operations
./scripts/repo-health-check.sh && git push
```

## Related Documentation

- Root Cause Analysis: `/docs/analysis/agent-signal-minus1-root-cause-analysis.md`
- Executive Summary: `/signal-1-crash-executive-summary.md`
- Investigation Report: `/signal-1-crash-investigation-beads-root-cause-analysis.md`
