# Safer Git GC Strategy

**Created:** 2026-09-01  
**Purpose:** Memory-efficient git garbage collection strategy that prevents OOM and provides resumability

## Problem Statement

`git gc --aggressive` is memory-intensive and can cause OOM issues on large repositories:
- **Memory usage:** Can consume gigabytes of RAM for delta compression
- **Time:** Runs for hours on large repos
- **Atomic:** Failure loses all progress
- **No monitoring:** No visibility into memory usage or progress

## Strategy: Incremental Multi-Stage Approach

Instead of monolithic `git gc --aggressive`, use a staged approach with memory limits and progress tracking.

### Stage 1: Standard GC (Fast, Low Memory)

```bash
git gc --prune=now
```

- **Time:** Seconds to minutes
- **Memory:** ~100-500MB
- **Purpose:** Remove loose objects, create basic pack
- **Safe:** Can be interrupted, minimal memory impact

### Stage 2: Incremental Repack (Progressive)

```bash
# Pack loose objects if any remain
git repack -q -d --max-pack-size=500m

# Consolidate small packs
git repack -q -d --unpack-unreachable=1.hour.ago
git repack -q -d -f --depth=50 --window=50
```

- **Time:** Minutes
- **Memory:** ~500MB-1GB
- **Purpose:** Create better compression without aggressive delta search
- **Safe:** Each step is independent

### Stage 3: Optional Deep Compression (With Limits)

```bash
# Only for repos needing aggressive optimization
git repack -q -d -f --depth=10 --window=10 --window-memory=1g
```

- **Time:** 30-60 minutes
- **Memory:** Capped at 1GB per `--window-memory`
- **Purpose:** Deep delta compression with memory guard
- **Safe:** Memory-limited, can be monitored

## Why This is Safer

| Aspect | `git gc --aggressive` | Staged Approach |
|--------|----------------------|-----------------|
| **Memory** | Unbounded (gigabytes) | Capped (~1GB max) |
| **Time** | Hours on large repos | Minutes to 1 hour |
| **Progress** | All-or-nothing | Each stage completes independently |
| **Interrupt** | Lose all progress | Resume from last completed stage |
| **Monitoring** | No visibility | Full progress tracking |
| **Recovery** | Must restart | Checkpoint at each stage |

## Memory Limits

### Git Configuration

```bash
# Limit memory used by pack operations
git config pack.windowMemory 1g
git config pack.depth 50
git config pack.deltaCacheSize 1g
```

### Cgroup Limits (Optional)

```bash
# Run under systemd slice with memory limit
systemd-run --scope --quiet -p MemoryMax=2g git gc
```

## Monitoring and Progress Tracking

### Metrics to Track

1. **Memory usage:** RSS of git process
2. **Disk usage:** Repository size before/after each stage
3. **Pack files:** Number and size of pack files
4. **Loose objects:** Count before/after each stage
5. **Time:** Duration of each stage

### Checkpoint System

After each stage:
1. Record completion timestamp
2. Save metrics to checkpoint file
3. Verify repository integrity
4. Commit checkpoint to git

### Resume Capability

If interrupted:
1. Read checkpoint file
2. Determine last completed stage
3. Resume from next stage
4. No duplicate work

## When to Use Each Strategy

### Standard GC (Stage 1 only)
- **Frequency:** Weekly/daily (automated)
- **When:** Routine maintenance
- **Memory:** Safe for any system
- **Time:** < 5 minutes

### Standard + Incremental (Stages 1-2)
- **Frequency:** Monthly
- **When:** Repository feels sluggish
- **Memory:** Requires ~1GB free
- **Time:** 10-30 minutes

### Full Deep Compression (Stages 1-3)
- **Frequency:** Quarterly or after large imports
- **When:** Archive repository, need maximum compression
- **Memory:** Requires ~2GB free
- **Time:** 1-2 hours

## Implementation

See `scripts/safe-git-gc.sh` for the complete implementation with:
- Progress monitoring
- Memory tracking
- Checkpoint/resume
- Automatic stage selection based on repo size

## Safety Features

1. **Pre-flight checks:** Verify disk space, memory, repository integrity
2. **Memory monitoring:** Track RSS, abort if exceeding limits
3. **Progress checkpoints:** Resume capability after interruption
4. **Integrity verification:** `git fsck` after each stage
5. **Rollback:** Keep old pack files until verification complete

## Comparison with Other Approaches

### git gc --aggressive
- ❌ Unbounded memory usage
- ❌ No progress visibility
- ❌ Cannot resume if interrupted
- ❌ Can OOM on large repositories

### git gc (standard)
- ✅ Fast and memory-efficient
- ⚠️ Limited compression
- ✅ Safe for frequent use

### Our Staged Approach
- ✅ Memory-capped operations
- ✅ Progress tracking and resumability
- ✅ Better compression than standard gc
- ✅ Safer than --aggressive
- ✅ Suitable for automation

## Recommendations

1. **Daily:** Run `git gc --prune=daily` (via cron)
2. **Weekly:** Run Stage 1 (standard gc)
3. **Monthly:** Run Stages 1-2 for repos with active development
4. **Quarterly:** Run Stages 1-3 for archive repos or after large imports
5. **Manual:** Use monitoring script to detect when gc is needed

## Monitoring Integration

The monitoring script tracks:
- Repository size trend
- Loose object count
- Pack file fragmentation
- Historical gc results

Alert when:
- Repository grows > 20% since last gc
- Loose objects > 1000
- Pack files > 5

## Success Criteria

A safe git gc strategy should:
- ✅ Never cause OOM
- ✅ Provide progress visibility
- ✅ Support resume after interruption
- ✅ Complete in reasonable time
- ✅ Achieve adequate compression
- ✅ Maintain repository integrity

Our staged approach satisfies all criteria.
