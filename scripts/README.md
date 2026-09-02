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

## Work Completion Verification (`verify-work-completion.sh`)

**Purpose:** The last step before `bead close` — verifies the work actually landed
and records a durable marker so crash triage can tell "crashed after verified
completion" apart from "crashed mid-task". This closes both failure directions of
the exit-code -1 incidents: agents closing beads whose work never got pushed, and
agents crashing after successful work with no record that it was done.

**Quick Start:**
```bash
# After committing and pushing, before closing the bead
./scripts/verify-work-completion.sh <bead-id> \
  --require-path docs/plan/plan.md \
  --summary "implemented X, committed in <sha>"

# On success (exit 0):
bead close <bead-id> --reason "work verified via verify-work-completion.sh"
```

**What It Checks:**
1. **Pushed commits** — HEAD must be on its upstream/origin ref (FAIL if ahead; `--allow-unpushed` downgrades to WARN). Falls back to the last known origin ref with a warning when offline.
2. **Uncommitted changes** — WARN by default (this workspace is shared; other agents' dirty files are normal), FAIL with `--strict-clean`
3. **Bead state** — bead exists in the store; completion notes present (WARN if empty, `--fail-on-empty-notes` to fail); already-closed beads verify as post-hoc WARN
4. **Expected artifacts** — `--require-path`, `--require-grep FILE:PATTERN`, `--require-command` (all repeatable)
5. **Box health** — memory/disk/1-min load, same limits as the pre-flight check, env-overridable (`VWC_MIN_AVAIL_MEM_GB`, `VWC_MIN_DISK_FREE_GB`, `VWC_MAX_CPU_LOAD`); `--skip-health` to omit

**Result Recording:** Every run writes `.beads/state/work-completion/<bead-id>.json`
(VERIFIED/FAILED, timestamp, per-check detail) and appends to
`.beads/logs/work-completion.log`. The marker is what crash triage should consult
when a worker died with exit code -1: a VERIFIED marker means the task itself was
complete and the crash is post-completion.

**Exit Codes:**
- `0` — work verified, safe to close
- `1` — verification failed, do NOT close the bead
- `2` — usage error

**Tests:** `./scripts/test-verify-work-completion.sh` (self-contained; fixture repos with local bare origins, no network or live bead mutation needed)

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

### Pre-Repack Verification (`verify-pre-repack.sh`)

Gate script to run immediately before any `git repack`. Verifies the
repository is in a consistent post-gc state and that resources are
sufficient.

**Usage:**
```bash
./scripts/verify-pre-repack.sh            # full output
./scripts/verify-pre-repack.sh --quiet    # suppress statistics
```

**Checks (any failure exits 1):**
- Parent git gc operation is complete (no `gc.pid`, no gc process)
- `git fsck --full` validation passes (dangling objects are informational)
- At least 2GB free disk space (`MIN_DISK_GB`, default 2)
- No git processes running and no git-internal lock files held

Exit codes: `0` ready for repack, `1` checks failed, `2` invalid arguments.

**Documentation:**
- Verified baseline: `docs/maintenance/pre-repack-verification-2026-09-02.md`

## Crash Prevention

### Crash Pattern Detection (`crash-pattern-detection.sh`)

Detects systematic crash patterns that indicate infrastructure events rather than isolated task failures.

**Quick Start:**
```bash
# Check last 24 hours for crash patterns
./scripts/crash-pattern-detection.sh

# Analyze different time periods
./scripts/crash-pattern-detection.sh --hours=48

# Generate detailed report
./scripts/crash-pattern-detection.sh --verbose --output=crash-report.txt
```

**What It Detects:**
1. **High Crash Rate** - Alerts if crashes/hour exceeds threshold (default: 5/hour)
2. **Crash Clustering** - Identifies concentrated crash events indicating infrastructure issues
3. **System Health** - Checks current memory, CPU, and disk health

**Exit Codes:**
- `0` - No concerning patterns detected
- `1` - Systematic crash pattern detected (take action)
- `2` - Error in execution

**Environment Variables:**
```bash
# Analysis time window in hours
export HOURS=24

# Crash rate threshold (crashes per hour)
export CRASH_RATE_THRESHOLD=5
```

**Integration with Monitoring:**
```bash
# Scheduled via systemd user timer (every 10 min) — not cron; this box has no crontab
systemctl --user list-timers domain-check-monitoring.timer

# Alert if pattern detected
if ! ./scripts/crash-pattern-detection.sh --quiet; then
  echo "WARNING: Systematic crash pattern detected" | mail -s "Alert" admin@example.com
fi
```

**Documentation:** See `docs/crash-mitigation-strategies.md` (Proposal 4.3)

### Resource Monitor (`resource-monitor.sh`)

Monitors system resources and generates alerts before crashes occur.

**Quick Start:**
```bash
# Single resource check
./scripts/resource-monitor.sh

# Continuous monitoring (5-minute intervals)
./scripts/resource-monitor.sh --continuous

# Custom interval (1 minute)
./scripts/resource-monitor.sh --continuous --interval 60
```

**What It Monitors:**
1. **Memory Availability** - Alert at 10GB warning, 5GB critical
2. **Disk Space** - Alert at 30GB warning, 20GB critical
3. **CPU Load** - Alert at 10 warning, 15 critical
4. **Memory Pressure** - Alert at 70% warning, 80% critical (OOM threshold)

**Alert Logs:**
- Resource alerts: `.beads/logs/resource-alerts.log`
- Metrics data: `.beads/logs/resource-metrics.log`

**Usage Pattern:**
```bash
# Scheduled via systemd user timer (every 5 min): domain-check-resource-monitor.timer
systemctl --user list-timers domain-check-resource-monitor.timer
```

**Documentation:** See `docs/crash-monitoring-implementation.md` (Proposal 4.2)

### Service Monitor (`service-monitor.sh`)

Monitors external service availability and detects outages.

**Quick Start:**
```bash
# Single service check
./scripts/service-monitor.sh

# Continuous monitoring (1-minute intervals)
./scripts/service-monitor.sh --continuous

# Verbose mode with detailed output
./scripts/service-monitor.sh --verbose
```

**What It Monitors:**
1. **Inference Gateway** - https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health
2. **Argo Workflows** - https://argo-ci.ardenone.com
3. **ArgoCD API** - https://argocd-ro-ardenone-manager-ts.ardenone.com:8444

**Features:**
- HTTP status code checking with configurable timeout
- Response time tracking
- Consecutive failure detection (3 failures = alert)
- Alert logging to `.beads/logs/service-alerts.log`

**Usage Pattern:**
```bash
# Scheduled via systemd user timer (every 2 min): domain-check-service-monitor.timer
systemctl --user list-timers domain-check-service-monitor.timer
```

**Documentation:** See `docs/crash-monitoring-implementation.md` (Proposal 4.1)

## Usage Examples

### Daily Maintenance
```bash
# Check if gc needed, run if so
./scripts/safe-git-gc.sh --check-only || ./scripts/safe-git-gc.sh

# Monitor progress
./scripts/safe-git-gc-monitor.sh --watch

# Check for crash patterns
./scripts/crash-pattern-detection.sh
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

### Health Checks Before Tasks
```bash
# Verify system health before starting agent work
./scripts/preflight-health-check.sh && ./scripts/crash-pattern-detection.sh

# Check resource status
./scripts/resource-monitor.sh

# Verify service availability
./scripts/service-monitor.sh
```

### Continuous Monitoring Setup
```bash
# Resource monitoring (every 5 minutes)
*/5 * * * * /home/coding/domain-check/scripts/resource-monitor.sh --continuous --interval 300

# Service monitoring (every 1 minute)
* * * * * /home/coding/domain-check/scripts/service-monitor.sh --continuous --interval 60

# Crash pattern detection (hourly)
0 * * * * /home/coding/domain-check/scripts/crash-pattern-detection.sh --alert --since "1hour"
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
