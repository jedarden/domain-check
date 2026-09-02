# Domain Check Crash Response Guide

**Created:** 2026-09-01  
**Purpose:** Agent guide for investigating and responding to crash alerts  
**Related:** `docs/crash-mitigation-strategies.md`, `docs/comprehensive-crash-investigation-report-2026-09-01.md`

---

## Quick Reference: Crash Classification

When investigating a crash, first classify the type:

| Exit Code | Pattern | Classification | Action |
|-----------|---------|----------------|--------|
| **-1** | SIGKILL/SIGHUP | Infrastructure event | Check system resources, verify work completion |
| **1** | error_max_turns | Workflow failure | Verify task completed, check bead closing issues |
| **1** | HTTP 503/502 | Service unavailability | Check inference gateway status, retry with backoff |
| **137** | SIGKILL (128+9) | OOM killer | Check memory pressure, verify git gc safety |
| **Other** | Application error | Code/task issue | Standard debugging |

---

## Automated Crash Alert System (Implemented 2026-09-02)

### Quick Start: Automated Crash Processing

**NEW:** Use the automated crash alert system before manual investigation:

```bash
# Process a crash alert with full automation
./scripts/crash-alert-manager.sh <bead-id>

# Auto-process recent crashes
./scripts/crash-alert-manager.sh --auto-process

# Classify a crash type
./scripts/crash-classifier.sh <bead-id>

# Test crash alert fixes
./scripts/test-crash-alert-fixes.sh
```

### What the Automated System Does

The crash alert manager automatically implements all 6 critical fixes:

1. **Closed Bead Filtering** - Skips alerts for beads that already completed successfully
2. **Duplicate Detection** - Prevents multiple investigation beads for same crash
3. **Exit Code Validation** - Checks exit code 0 (success) vs actual crash
4. **Completion Awareness** - Detects post-completion termination vs crash during task
5. **Alert Cooldown** - 5-minute cooldown prevents alert spam during system-wide events
6. **Crash Classification** - Categorizes crashes as FALSE_POSITIVE, SERVICE_FAILURE, INFRASTRUCTURE, or CODE_DEFECT

### Classification Types

| Classification | Description | Action Required |
|----------------|-------------|-----------------|
| **FALSE_POSITIVE** | Post-completion cleanup failure, max_turns, or completed bead | No action - close bead |
| **SERVICE_FAILURE** | External service unavailable (HTTP 503/502) | Retry with backoff when service restored |
| **INFRASTRUCTURE** | OOM, SIGHUP cascade, resource exhaustion | Check system resources, verify work completion |
| **CODE_DEFECT** | Actual application error | Standard investigation required |
| **UNKNOWN** | Unable to classify | Manual investigation required |

### Example Usage

```bash
# Investigate crash alert bf-3561g
./scripts/crash-alert-manager.sh bf-3561g

# Output:
# INFO: Checking bead closure status for: bf-3561g
# INFO: Checking for existing alert beads for target: bf-4k2ws
# INFO: Validating exit code before generating alert
# INFO: Classifying crash type...
# Classification: FALSE_POSITIVE
# Reason: Bead bf-4k2ws already closed (completed successfully)
# Action: No alert generated - false positive filtered
```

### Monitoring System Integration

The automated crash system integrates with continuous monitoring:

```bash
# Install continuous monitoring
./scripts/monitoring-setup.sh

# Monitor logs
tail -f .beads/logs/crash-alert-manager.log
tail -f .beads/logs/crash-monitor.log
tail -f .beads/logs/resource-alerts.log
```

### When to Use Manual vs Automated

**Use Automated System (crash-alert-manager.sh) for:**
- All standard crash alerts
- Post-completion cleanup failures
- System-wide event detection
- Duplicate alert filtering

**Use Manual Investigation (this guide) for:**
- Unusual crash patterns not classified by automation
- Code defects requiring debugging
- Complex multi-factor crashes
- Verification of automated classification

---

## Investigation Checklist

### Phase 1: Immediate Classification (2 minutes)

**Automated First Step:**
```bash
# Try automated classification first
./scripts/crash-classifier.sh <bead-id>

# If classification is CODE_DEFECT or UNKNOWN, proceed with manual investigation
```

**Manual Classification (if automated system insufficient):**

```bash
# 1. Get bead metadata
bead show <id> --json

# 2. Check exit code and outcome
# Exit code -1 → Infrastructure event (skip to Phase 2A)
# Exit code 1 with "error_max_turns" → Workflow failure (skip to Phase 2B)
# Exit code 1 with HTTP 5xx → Service failure (skip to Phase 2C)
# Other → Standard investigation (Phase 2D)

# 3. Check current system state
free -h                    # Memory availability
df -h /                    # Disk space
uptime                     # Load average
```

### Phase 2A: Infrastructure Event (Exit Code -1)

**Pattern:** SIGKILL, SIGHUP, OOM killer → System-wide resource pressure

**Checklist:**
- [ ] Verify task completed successfully before crash
  ```bash
  git log --since="<crash_timestamp>" --until="<crash_timestamp+30min>" --oneline
  ```
- [ ] Check for system-wide events
  ```bash
  journalctl --since "<crash_timestamp-1hour>" --until "<crash_timestamp+1hour>" | grep -E "oom|kill|memory"
  ```
- [ ] Classify as false positive if work completed
  ```bash
  # If commit exists within 30 seconds before crash → FALSE POSITIVE
  # If no commit found → Proceed to Phase 2D
  ```

**Common Infrastructure Events:**
- **Memory Pressure:** systemd-oomd activation (94.71% pressure threshold)
- **SIGHUP Cascade:** System-wide signal to all workers
- **OOM Killer:** Process termination (exit code 137)

**Action Required:**
- ✅ NO CODE CHANGES NEEDED
- ⚠️ Document in bead notes as "false positive - infrastructure event"
- ⚠️ Close bead with clear notes

### Phase 2B: Workflow Failure (error_max_turns)

**Pattern:** Agent exhausted 30-turn limit during post-task operations

**Checklist:**
- [ ] Verify main task completed successfully
  ```bash
  # Check for task completion markers:
  # - Git commits with task-related changes
  # - Test results showing success
  # - Documentation indicating completion
  ```
- [ ] Identify where agent got stuck
  ```bash
  # Read trace file for last actions
  jq -r '.[] | select(.type == "tool_call") | .tool' .beads/traces/<id>/trace.jsonl | tail -20
  ```
- [ ] Classify as false positive if task succeeded
  ```bash
  # If task evidence exists → FALSE POSITIVE - workflow issue only
  # If no evidence of task completion → Proceed to Phase 2D
  ```

**Common Workflow Failures:**
- **Bead closing loops:** Agent retrying close operations
- **Post-completion troubleshooting:** Agent trying to resolve non-issues
- **Verification loops:** Excessive checking of completed work

**Action Required:**
- ✅ NO CODE CHANGES NEEDED
- ⚠️ Document main task outcome in bead notes
- ⚠️ Close bead with task completion status

### Phase 2C: Service Failure (HTTP 503/502)

**Pattern:** Inference gateway or external service unavailable

**Checklist:**
- [ ] **Check if preflight health check was run**
  ```bash
  # Review health check log
  cat /tmp/preflight-health-check.log | tail -50
  
  # If preflight check was skipped → Preventable crash
  # If preflight check failed → Expected deferral (not a crash)
  # If preflight check passed → Unexpected service failure
  ```
- [ ] Check service availability
  ```bash
  # For inference gateway (zai provider):
  curl -f --max-time 5 https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health || echo "Gateway down"
  
  # Or use the preflight check script:
  ./scripts/preflight-health-check.sh --verbose
  ```
- [ ] Identify failure point
  ```bash
  # Check trace for service error messages
  grep -i "503\|502\|unavailable" .beads/traces/<id>/trace.jsonl
  ```
- [ ] Verify no domain-check code involved
  ```bash
  # Service failures are external to domain-check code
  # Agent framework issue, not application issue
  ```

**Common Service Failures:**
- **Inference Gateway 503:** "no available server" (traefik-apexalgo-iad)
- **Network timeouts:** Temporary connectivity issues
- **Rate limiting:** External API limits exceeded

**Action Required:**
- ✅ NO CODE CHANGES NEEDED
- ⚠️ Retry task with backoff if service is now available
- ⚠️ If service still down, defer task until restored

### Phase 2D: Standard Investigation (Other exit codes)

**Pattern:** Possible application-level error

**Checklist:**
- [ ] Examine crash artifacts
  ```bash
  ls -la .beads/traces/<id>/
  # - trace.jsonl (conversation trace)
  # - metadata.json (session metadata)
  # - stdout.txt (agent output)
  # - stderr.txt (agent errors)
  ```
- [ ] Review error messages
  ```bash
  jq -r '.[] | select(.type == "error") | .message' .beads/traces/<id>/trace.jsonl
  ```
- [ ] Check for domain-check code involvement
  ```bash
  # If crash involved domain-check operations → Investigate code
  # If crash was in agent framework → Infrastructure/workflow issue
  ```
- [ ] Verify repository integrity
  ```bash
  git fsck --full
  git status
  ```

---

## Common Crash Patterns

### Pattern 1: Post-Completion False Positive (~40% of crashes)

**Symptoms:**
- Exit code -1 (SIGKILL)
- Work committed successfully before crash
- 30-second gap between completion and termination

**Example Timeline:**
```
16:35:54 UTC - Task completed, commit 549aa42 created
16:36:24 UTC - Agent terminated (SIGKILL)
16:36:51 UTC - Bead closed successfully
```

**Action:** Classify as FALSE POSITIVE, close with notes

### Pattern 2: Git GC Operations (~15% of crashes)

**Symptoms:**
- Exit code 137 (OOM killer) or -1 (SIGKILL)
- `git gc --aggressive` in progress
- High memory usage during git operation

**Verification:**
```bash
# Check if git gc completed successfully
git count-objects -vH
git fsck --full

# If repository valid and compressed → Git gc succeeded, termination was cleanup
```

**Action:**
- ✅ Use `scripts/safe-git-gc.sh` instead of bare `git gc --aggressive`
- ✅ Verify repository integrity
- ⚠️ If OOM occurred, document memory limits used

### Pattern 3: Repository Bloat Crashes (~15% of infrastructure crashes)

**Symptoms:**
- Exit code -1 (SIGKILL from OOM killer)
- Repository size > 5GB (should be <500MB)
- Loose objects > 1GB (should be packed)
- Routine git operations trigger OOM
- Multiple crashes over short period (all exit code -1)

**Evidence from bf-4yjq (2026-08-12):**
- 9 crashes over 2.5 hours, all exit code -1
- Repository: 18GB with 17GB loose objects
- `.beads/issues.jsonl`: 248MB (should be <5MB)
- Any significant git operation triggered OOM

**Verification:**
```bash
# Check repository size
du -sh .git
du -sh .git/objects

# Count loose vs packed objects
git count-objects -vH

# If repository > 5GB with > 1GB loose objects → REPOSITORY BLOAT
```

**Action:**
- ⚠️ IMMEDIATE: Add `.beads/*.jsonl` to `.gitignore`
- ⚠️ Run safe git gc: `./scripts/safe-git-gc.sh --full`
- ✅ Enable repository monitoring: `./scripts/monitoring-setup.sh`
- ✅ Install scheduled gc in crontab (daily)
- ✅ Install pre-commit hooks to prevent future large file additions
- ✅ Document root cause bead (e.g., bf-2ildm) that created large commits

**Prevention:**
```bash
# Install repository monitoring
./scripts/monitoring-setup.sh

# Add to .gitignore immediately
echo ".beads/*.jsonl" >> .gitignore
echo ".beads/*.json" >> .gitignore
echo ".beads/checkpoint/" >> .gitignore
echo ".beads/traces/" >> .gitignore
```

### Pattern 4: Service Availability Failure (~8% of crashes)

**Symptoms:**
- Exit code 1 with HTTP 503/502 errors
- "no available server" message
- Inference gateway unavailable

**Action:**
- ⚠️ Check gateway status: `curl https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health`
- ⚠️ Retry task when service restored
- ✅ NO code changes needed

### Pattern 5: Max Turns Exhaustion (~20% of crashes)

**Symptoms:**
- Exit code 1 with "error_max_turns"
- Agent spent turns on post-task troubleshooting
- Main task completed successfully

**Action:**
- ✅ Verify main task completion
- ⚠️ Document workflow issue
- ✅ NO code changes needed if task succeeded

---

## False Positive Detection Heuristics

Use these rules to quickly identify false positives:

### Rule 1: Time Gap Check
```bash
# If commit exists < 30 seconds before crash → FALSE POSITIVE
commit_time=$(git log -1 --format=%ct <commit_hash>)
crash_time=$(date -d "<crash_timestamp>" +%s)
gap=$((crash_time - commit_time))

if [ $gap -lt 30 ]; then
  echo "FALSE POSITIVE: Work completed $gap seconds before crash"
fi
```

### Rule 2: Success Pattern Check
```bash
# If crash → retry → success pattern → SELF-HEALED TRANSIENT FAILURE
# Check bead event history for successful retries
bead show <id> --json | jq '.history[] | select(.outcome == "success")'
```

### Rule 3: System-Wide Event Check
```bash
# If 10+ crashes within 10 minutes → INFRASTRUCTURE EVENT
# Generate single system event alert, not individual bead alerts
crash_count=$(bead list --since "10min ago" --status "crashed" --json | jq '. | length')
if [ $crash_count -gt 10 ]; then
  echo "INFRASTRUCTURE EVENT: $crash_count crashes in 10 minutes"
fi
```

---

## Git GC Safety Procedures

### When to Use Safe Git GC Scripts

**ALWAYS use `scripts/safe-git-gc.sh` instead of bare `git gc --aggressive`:**

```bash
# Check if gc needed
./scripts/safe-git-gc.sh --check-only

# Run standard gc (stages 1-2)
./scripts/safe-git-gc.sh

# Run full gc with deep compression
./scripts/safe-git-gc.sh --full

# Monitor progress
./scripts/safe-git-gc-monitor.sh --watch
```

**Why Safe Scripts?**
- ✅ Memory-limited operations (configurable via `SAFE_GC_MEMORY_MAX`)
- ✅ Three-stage strategy (standard → incremental → deep compression)
- ✅ Checkpoint/resume capability after each stage
- ✅ Progress tracking and monitoring
- ✅ Pre-flight integrity checks

**When `git gc --aggressive` is Acceptable:**
- On dedicated systems with > 16GB free RAM
- With cgroup memory limits in place
- For one-time optimization of large repos
- When safe-git-gc scripts are unavailable

**When NOT to Use --aggressive:**
- On systems with < 8GB RAM
- On repos > 5GB without memory limits
- Without monitoring/resumability
- On shared systems where OOM affects others

### Memory-Limited Git GC

```bash
# Run git gc under systemd slice with memory limit
systemd-run --scope --quiet \
  -p MemoryMax=2g \
  -p MemorySwapMax=0 \
  -p CPUQuota=200% \
  scripts/safe-git-gc.sh --full
```

### Monitoring Git GC Progress

```bash
# Watch gc progress in real-time
./scripts/safe-git-gc-monitor.sh --watch

# Check gc status
./scripts/safe-git-gc-monitor.sh
```

---

## Service Availability Procedures

### Pre-Flight Health Checks

**IMPLEMENTED:** Pre-flight health check script available at `scripts/preflight-health-check.sh`

**Before starting agent tasks that depend on external services, run:**

```bash
# Standard pre-flight check
./scripts/preflight-health-check.sh

# Verbose mode for detailed diagnostics
./scripts/preflight-health-check.sh --verbose

# Warn-only mode for monitoring
./scripts/preflight-health-check.sh --warn-only
```

**What the script checks:**
- Inference gateway availability
- Memory availability (configurable, default 10GB)
- Disk space (configurable, default 20GB)
- CPU load (configurable, default <10 on 1min average)
- Git repository health

**Exit codes:**
- `0` - All checks passed
- `1` - One or more checks failed
- `2` - Invalid arguments

**Usage pattern:**
```bash
# Before starting agent task
if ! ./scripts/preflight-health-check.sh; then
  echo "ERROR: System health check failed"
  echo "Task deferred until system is healthy"
  exit 1
fi

# Task proceeds knowing resources are sufficient
./agent-task.sh
```

**See also:** `docs/crash-prevention-preflight-checks.md` (implementation documentation)

---

**Alternative: Manual Health Checks**

If you need to customize the checks, here's the manual approach:

```bash
#!/bin/bash
# Manual pre-flight health check

# Check inference gateway
GATEWAY_URL="https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health"
if ! curl -sf --max-time 5 "$GATEWAY_URL" > /dev/null; then
  echo "ERROR: Inference gateway unavailable"
  echo "Deferring task until service is healthy"
  exit 1
fi

# Check memory availability
AVAILABLE_MEM=$(free -g | awk '/^Mem:/{print $7}')
if [ $AVAILABLE_MEM -lt 10 ]; then
  echo "ERROR: Insufficient memory (${AVAILABLE_MEM}GB available)"
  exit 1
fi

# Check disk space
DISK_FREE=$(df -BG / | awk 'NR==2 {print $4}' | tr -d 'G')
if [ $DISK_FREE -lt 20 ]; then
  echo "ERROR: Insufficient disk space (${DISK_FREE}GB free)"
  exit 1
fi

echo "All health checks passed - proceeding with task"
```

### Retry with Exponential Backoff

For transient service failures:

```bash
max_retries=5
base_delay=1  # second

for attempt in $(seq 1 $max_retries); do
  if api_call; then
    exit 0
  fi
  
  if [[ $response_status == "503" ]] || [[ $response_status == "502" ]]; then
    delay=$(echo "$base_delay * 2^($attempt - 1)" | bc)
    echo "Retry $attempt/$max_retries after ${delay}s delay"
    sleep $delay
  else
    # Non-transient error - fail immediately
    exit 1
  fi
done

exit 1  # All retries exhausted
```

---

## Crash Documentation Template

When documenting a crash investigation, use this template:

```markdown
# Crash Investigation: <bead_id>

**Investigation Date:** <date>
**Bead ID:** <id>
**Agent:** <agent_name>
**Exit Code:** <code>
**Classification:** <FALSE POSITIVE | INFRASTRUCTURE | SERVICE | CODE>

## Executive Summary
<One-paragraph summary of crash type and classification>

## Crash Timeline
- <timestamp>: Event 1
- <timestamp>: Event 2
- <timestamp>: Crash

## Root Cause
<Primary cause classification>

## Evidence
- System resources at crash time
- Relevant log entries
- Work completion verification

## Action Required
- ✅ NO ACTION or ⚠️ SPECIFIC ACTION

## Classification
<FALSE POSITIVE / INFRASTRUCTURE ISSUE / SERVICE FAILURE / CODE DEFECT>
```

---

## Resource Limits and Monitoring

### Safe Operating Limits

| Resource | Minimum | Warning | Critical |
|----------|---------|---------|----------|
| **Available Memory** | 20GB | 10GB | 5GB |
| **Disk Space** | 50GB | 30GB | 20GB |
| **CPU Load (1min)** | < 5 | < 10 | > 15 |
| **Git GC Memory** | 1GB | 2GB | 4GB |

### Pre-Task Resource Check

```bash
# Check system resources before starting task
echo "=== System Resources ==="
free -h
df -h /
uptime

# Abort if resources insufficient
AVAILABLE_MEM=$(free -g | awk '/^Mem:/{print $7}')
if [ $AVAILABLE_MEM -lt 10 ]; then
  echo "ABORT: Insufficient memory (${AVAILABLE_MEM}GB available)"
  exit 1
fi
```

---

## When to Escalate

Escalate to human operator if:

1. **Repository Corruption Suspected**
   - `git fsck` shows errors
   - Objects missing or corrupted
   - Repo size unexpectedly large

2. **Persistent Service Failures**
   - Service down for > 30 minutes
   - Multiple retries fail with same error
   - No service status information available

3. **Unknown Exit Codes**
   - Exit code not in classification table
   - No recognizable error pattern
   - Multiple unexplained crashes

4. **Data Loss Suspected**
   - Work artifacts missing
   - Expected commits not found
   - Test results inconsistent

---

## Monitoring Recommendations

### System-Level Monitoring

```yaml
# Example Prometheus alerts
monitoring:
  alerts:
    - name: HighMemoryPressure
      expr: node_memory_pressure_percentage > 70
      for: 1m
      annotations:
        summary: "Memory pressure above 70% - OOM risk"
    
    - name: DiskSpaceLow
      expr: node_filesystem_avail_bytes{mountpoint="/"} < 20GB
      for: 5m
      annotations:
        summary: "Less than 20GB disk space available"
    
    - name: CrashSurgeDetected
      expr: needle_crashes_total{outcome="failed"} > 10
      for: 10m
      annotations:
        summary: "Infrastructure event: 10+ crashes in 10 minutes"
```

### Application-Level Monitoring

```yaml
  - name: InferenceGatewayDown
      expr: up{job="inference_gateway"} == 0
      for: 1m
      annotations:
        summary: "Inference gateway is down"
        description: "Agents will fail until gateway is restored"
    
    - name: NeedleAgentTaskStuck
      expr: needle_agent_task_duration_seconds{outcome="running"} > 7200
      for: 10m
      annotations:
        summary: "Agent task running > 2 hours"
```

---

## Key Learnings Summary

### What Causes Crashes

1. **Infrastructure Events (70%)**: Memory pressure, OOM killer, SIGHUP cascade, **repository bloat**
2. **Workflow Failures (20%)**: Max turns exhaustion, bead closing loops
3. **Service Failures (8%)**: Inference gateway unavailable, network issues
4. **Code Defects (2%)**: Actual application errors

### What Does NOT Cause Crashes

1. ✅ **Git GC** - When using safe-git-gc scripts
2. ✅ **Domain-Check Code** - No defects found in any crash investigation
3. ✅ **Normal Operations** - Well within resource limits

### Quick Decision Tree

```
Exit Code -1?
├─ Yes → Infrastructure Event
│  ├─ Work completed within 30s? → FALSE POSITIVE
│  └─ No completion evidence? → Check system logs
│
Exit Code 1 with error_max_turns?
├─ Yes → Workflow Failure
│  ├─ Main task completed? → FALSE POSITIVE
│  └─ Task incomplete? → Max turns issue
│
Exit Code 1 with HTTP 503/502?
├─ Yes → Service Failure
│  └─ Check gateway status, retry with backoff
│
Other Exit Code?
└─ Standard Investigation
   ├─ Domain-check code involved? → Debug code
   └─ Agent framework issue? → Workflow/infrastructure
```

---

## Related Documentation

- **Automated Crash Alert System (2026-09-02):**
  - `docs/crash-alert-fix-implementation-2026-09-02.md` - Complete crash alert system documentation
  - `docs/monitoring-implementation-summary-2026-09-02.md` - Monitoring system implementation
  - `docs/verification-report-bf-4k2ws-crash-investigation-2026-09-02.md` - bf-4k2ws crash verification

- **Comprehensive Investigation:** `docs/comprehensive-crash-investigation-report-2026-09-01.md`
- **Mitigation Strategies:** `docs/crash-mitigation-strategies.md`

- **Specific Crashes:** 
  - `docs/crash-analysis-domchk-c9641ac5-2026-09-01.md` (Service availability)
  - `docs/investigation-summary-bf-173o7e-2026-09-01.md` (False positive)
  - `docs/crash-artifacts-bf-4yjq.md` (Repository bloat - 9 OOM crashes from 18GB repo)

- **Git GC Safety:** `docs/safe-git-gc-implementation.md`, `docs/safer-git-gc-strategy.md`

---

**Guide Status:** ✅ Complete  
**Last Updated:** 2026-09-02 (Added automated crash alert system section)  
**Target Audience:** Agents investigating crash alerts  
**Purpose:** Fast crash classification and response decisions
