# Domain Check Scripts

This directory contains utility scripts for the domain-check project.

## Pre-flight Health Check (`preflight-health-check.sh`)

**Purpose:** Prevent agent crashes from external service failures and resource exhaustion by verifying system health before starting tasks.

**Quick Start:**
```bash
# Standard health check (fails if any check fails)
./scripts/preflight-health-check.sh

# Verbose mode with detailed output
./scripts/preflight-health-check.sh --verbose

# Monitoring mode (exit 0 even if checks fail)
./scripts/preflight-health-check.sh --warn-only
```

**What It Checks:**
1. **Inference Gateway Availability** - Verifies external AI service is reachable
2. **Memory Availability** - Ensures minimum RAM available (default: 10GB)
3. **Disk Space** - Checks sufficient free space (default: 20GB)
4. **CPU Load** - Verifies system load is acceptable (default: <10 on 1min average)
5. **Git Repository Health** - Validates repository integrity

**Exit Codes:**
- `0` - All checks passed (or `--warn-only` mode)
- `1` - One or more checks failed
- `2` - Invalid arguments

**Environment Variables:**
```bash
# Inference gateway health endpoint
export INFERENCE_GATEWAY_URL="https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health"

# Minimum available memory in GB
export MIN_AVAILABLE_MEM_GB=10

# Minimum disk space in GB
export MIN_DISK_FREE_GB=20

# Maximum CPU load (1-minute average)
export MAX_CPU_LOAD=10

# HTTP timeout for gateway check
export CURL_TIMEOUT=5
```

**Usage Pattern:**
```bash
# Before starting any agent task
if ! ./scripts/preflight-health-check.sh; then
  echo "System not healthy - deferring task"
  exit 1
fi

# Task proceeds knowing resources are sufficient
./run-agent-task.sh
```

**Documentation:** See `docs/crash-mitigation-strategies.md` (Proposal 1.3)

## Git Maintenance

### Safe Git GC (`safe-git-gc.sh`)

A memory-efficient, resumable git garbage collection system that prevents OOM issues through staged operations.

**Quick Start:**
```bash
# Check if gc is needed
./scripts/safe-git-gc.sh --check-only

# Run standard gc (stages 1-2, ~10-30 minutes)
./scripts/safe-git-gc.sh

# Run full gc with deep compression (all stages, ~1-2 hours)
./scripts/safe-git-gc.sh --full

# Resume from last checkpoint if interrupted
./scripts/safe-git-gc.sh --resume
```

**Features:**
- ✅ Three-stage gc strategy (standard → incremental → deep compression)
- ✅ Memory-limited operations (configurable via `SAFE_GC_MEMORY_MAX`)
- ✅ Checkpoint/resume capability after each stage
- ✅ Pre-flight integrity checks
- ✅ Progress logging to `.git/safe-gc.log`

**Environment Variables:**
```bash
# Maximum memory for git operations (default: 2g)
export SAFE_GC_MEMORY_MAX=2g

# Custom checkpoint file location
export SAFE_GC_CHECKPOINT=.git/safe-gc-checkpoint.json
```

**Documentation:**
- Strategy overview: `docs/safer-git-gc-strategy.md`
- Implementation guide: `docs/safe-git-gc-implementation.md`

### Safe Git GC Monitor (`safe-git-gc-monitor.sh`)

Monitor progress and resource usage of git gc operations.

**Usage:**
```bash
# One-time status check
./scripts/safe-git-gc-monitor.sh

# Watch mode (auto-refresh every 2 seconds)
./scripts/safe-git-gc-monitor.sh --watch
```

**Features:**
- ✅ Real-time status display
- ✅ Checkpoint state inspection
- ✅ Running process detection
- ✅ Repository statistics
- ✅ Recent log entries

## Usage Examples

### Daily Maintenance
```bash
# Check if gc needed, run if so
./scripts/safe-git-gc.sh --check-only || ./scripts/safe-git-gc.sh

# Monitor progress
./scripts/safe-git-gc-monitor.sh --watch
```

### After Large Changes
```bash
# Run full gc with deep compression
./scripts/safe-git-gc.sh --full
```

### If Interrupted
```bash
# Resume from last checkpoint
./scripts/safe-git-gc.sh --resume
```

## Why Not `git gc --aggressive`?

The standard `git gc --aggressive` command:
- ❌ Can consume gigabytes of RAM (unbounded)
- ❌ Runs for hours on large repositories
- ❌ Cannot be resumed if interrupted
- ❌ No progress visibility

Our staged approach:
- ✅ Memory-capped operations (~1-2GB max)
- ✅ Faster execution (10-120 minutes depending on mode)
- ✅ Checkpoint after each stage (resumable)
- ✅ Full progress monitoring and logging

See `docs/safer-git-gc-strategy.md` for detailed comparison.
