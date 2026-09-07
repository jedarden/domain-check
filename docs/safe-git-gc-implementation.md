# Safe Git GC Implementation Guide

**Created:** 2026-09-01  
**Scripts:** `scripts/safe-git-gc.sh`, `scripts/safe-git-gc-monitor.sh`

## Overview

This implementation provides a memory-efficient, resumable git garbage collection system that prevents OOM issues through staged operations with progress tracking.

## Components

### 1. Main GC Script (`scripts/safe-git-gc.sh`)

**Features:**
- ✅ Three-stage gc strategy (standard → incremental → deep compression)
- ✅ Memory-limited operations (configurable via `SAFE_GC_MEMORY_MAX`)
- ✅ Checkpoint/resume capability after each stage
- ✅ Pre-flight integrity checks
- ✅ Progress logging to `.git/safe-gc.log`
- ✅ Checkpoint state in `.git/safe-gc-checkpoint.json`

**Usage:**
```bash
# Standard gc (stages 1-2, ~10-30 minutes)
./scripts/safe-git-gc.sh

# Full gc with deep compression (all stages, ~1-2 hours)
./scripts/safe-git-gc.sh --full

# Resume from last checkpoint
./scripts/safe-git-gc.sh --resume

# Check if gc is needed (exit code 0 = needed, 1 = not needed)
./scripts/safe-git-gc.sh --check-only
```

**Environment Variables:**
```bash
# Maximum memory for git operations (default: 2g)
export SAFE_GC_MEMORY_MAX=2g

# Custom checkpoint file location
export SAFE_GC_CHECKPOINT=.git/safe-gc-checkpoint.json
```

### 2. Monitor Script (`scripts/safe-git-gc-monitor.sh`)

**Features:**
- ✅ Real-time status display
- ✅ Checkpoint state inspection
- ✅ Running process detection
- ✅ Repository statistics
- ✅ Recent log entries

**Usage:**
```bash
# One-time status check
./scripts/safe-git-gc-monitor.sh

# Watch mode (auto-refresh every 2 seconds)
./scripts/safe-git-gc-monitor.sh --watch
```

## Stage Details

### Stage 1: Standard GC
- **Command:** `git gc --prune=now`
- **Time:** Seconds to minutes
- **Memory:** ~100-500MB
- **Purpose:** Remove loose objects, create basic pack
- **When:** Weekly/daily automated

### Stage 2: Incremental Repack
- **Commands:** 
  - `git repack -q -d --max-pack-size=500m`
  - `git repack -q -d -f --depth=50 --window=50`
- **Time:** 5-15 minutes
- **Memory:** ~500MB-1GB
- **Purpose:** Better compression without aggressive delta search
- **When:** Monthly or when repo feels sluggish

### Stage 3: Deep Compression (Optional)
- **Command:** `git repack -q -d -f --depth=10 --window=10 --window-memory=1g`
- **Time:** 30-60 minutes
- **Memory:** Capped at configured limit (default: 1GB)
- **Purpose:** Maximum compression for archive repos
- **When:** Quarterly or after large imports

## Checkpoint Format

The checkpoint file (`.git/safe-gc-checkpoint.json`) contains:
```json
{
  "timestamp": "2026-09-01T12:00:00+00:00",
  "stage": "stage2",
  "status": "complete",
  "message": "Completed in 245s",
  "repo_size": "445M",
  "loose_objects": 3,
  "pack_count": 1,
  "mode": "standard"
}
```

## Resume Workflow

If gc is interrupted (SIGKILL, OOM, system crash):

```bash
# Check status
./scripts/safe-git-gc-monitor.sh

# Resume from last completed stage
./scripts/safe-git-gc.sh --resume
```

The script will:
1. Read checkpoint file
2. Determine last completed stage
3. Resume from next stage
4. Skip completed work

## Safety Features

### Pre-flight Checks
- ✅ Verify adequate disk space (>5GB)
- ✅ Run `git fsck` to ensure repository integrity
- ✅ Record baseline statistics

### Memory Limits
- ✅ Configure `pack.windowMemory` to cap memory
- ✅ Set `pack.deltaCacheSize` to limit cache
- ✅ Use `--window-memory` flag in repack operations

### Progress Tracking
- ✅ Checkpoint after each stage
- ✅ Timestamps and duration tracking
- ✅ Detailed logging to `.git/safe-gc.log`

### Integrity Verification
- ✅ `git fsck` after each stage
- ✅ Final verification before completion
- ✅ Stats comparison (before/after)

## Monitoring Integration

### Example Status Output
```
=== Safe Git GC Status ===

✓ Last GC:
  Timestamp: 2026-09-01T12:34:56+00:00
  Stage: complete
  Status: complete
  Message: All stages finished, repo size: 445M
  Repository size: 445M
  Loose objects: 3
  Pack files: 1

Current Repository:
  count: 63
  size: 436.00 KiB
  in-pack: 9164
  packs: 1
  size-pack: 88.70 MiB
  prune-packable: 0
  garbage: 0
  size-garbage: 0 bytes

No git gc processes running

Recent log entries:
  [2026-09-01 12:34:56] === Safe Git GC Started ===
  [2026-09-01 12:34:56] Mode: standard
  [2026-09-01 12:34:56] Memory limit: 2g
  [2026-09-01 12:34:57] Checking if gc is needed...
  [2026-09-01 12:34:57]   Loose objects: 63
  [2026-09-01 12:34:57]   Pack files: 1
  ...
```

## Automation Examples

### Daily Check (Cron)
```cron
# Daily at 2am - check if gc needed
0 2 * * * /home/coding/domain-check/scripts/safe-git-gc.sh --check-only || /home/coding/domain-check/scripts/safe-git-gc.sh
```

### Weekly Standard GC
```cron
# Weekly on Sunday at 3am - standard gc
0 3 * * 0 /home/coding/domain-check/scripts/safe-git-gc.sh
```

### Monthly Full GC
```cron
# Monthly on 1st at 4am - full gc with deep compression
0 4 1 * * /home/coding/domain-check/scripts/safe-git-gc.sh --full
```

## Troubleshooting

### GC is not needed
```bash
# Force gc anyway
rm .git/safe-gc-checkpoint.json
./scripts/safe-git-gc.sh
```

### Resume from failed stage
```bash
# Check what failed
./scripts/safe-git-gc-monitor.sh

# Resume (will re-run failed stage)
./scripts/safe-git-gc.sh --resume
```

### Check logs
```bash
# View full log
cat .git/safe-gc.log

# View recent errors
grep ERROR .git/safe-gc.log

# View specific stage
grep "Stage 2" .git/safe-gc.log
```

### Manual intervention
```bash
# Stop running gc
pkill -f "git (gc|repack)"

# Clean up locks (if gc crashed)
rm -f .git/gc.log.lock

# Verify repository
git fsck --full
```

## Performance Comparison

| Operation | Time | Memory | Compression | Resume |
|-----------|------|--------|-------------|--------|
| `git gc --aggressive` | 2-4h | 4-8GB | Best | ❌ |
| `git gc` (standard) | 1-5m | 100-500MB | Good | ❌ |
| **Safe GC (standard)** | 10-30m | ~1GB | Good | ✅ |
| **Safe GC (full)** | 1-2h | ~2GB | Best | ✅ |

## Testing

### Test basic functionality
```bash
# Check if gc is needed
./scripts/safe-git-gc.sh --check-only && echo "GC needed" || echo "GC not needed"

# Run standard gc
./scripts/safe-git-gc.sh

# Monitor progress
./scripts/safe-git-gc-monitor.sh --watch
```

### Test resume capability
```bash
# Start gc, then interrupt
./scripts/safe-git-gc.sh &
PID=$!
sleep 10
kill $PID

# Resume
./scripts/safe-git-gc.sh --resume

# Verify completion
./scripts/safe-git-gc-monitor.sh
```

## Recommendations

### For Active Development Repos
- **Daily:** Check if gc needed (`--check-only`)
- **Weekly:** Standard gc (stages 1-2)
- **Monthly:** Full gc if repo > 1GB

### For Archive Repos
- **Monthly:** Standard gc
- **Quarterly:** Full gc with deep compression
- **After large imports:** Run full gc immediately

### For CI/CD Environments
- **Before merge tests:** Run `--check-only`
- **Nightly:** Standard gc on main repos
- **Weekly:** Full gc on large repos

## Success Metrics

A safe gc run should:
- ✅ Complete without OOM
- ✅ Show progress in monitor
- ✅ Create checkpoint file
- ✅ Pass final `git fsck`
- ✅ Reduce repo size or loose objects
- ✅ Complete in expected time frame

## Related Documentation

- Strategy overview: `docs/safer-git-gc-strategy.md`
- Implementation: This file
- Scripts: `scripts/safe-git-gc.sh`, `scripts/safe-git-gc-monitor.sh`

---

**Implementation Status:** ✅ Complete  
**Tested:** 2026-09-01  
**Ready for Production:** Yes
