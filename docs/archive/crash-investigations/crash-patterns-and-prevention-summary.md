# Crash Patterns and Prevention Summary

**Created:** 2026-09-01  
**Purpose:** Consolidated crash patterns reference with prevention measures  
**Related:** `docs/crash-response-guide.md`, `docs/crash-mitigation-strategies.md`, `docs/comprehensive-crash-investigation-report-2026-09-01.md`

---

## Quick Reference: Crash Patterns by Exit Code

| Pattern | Exit Code | Frequency | Classification | Primary Cause | Resolution |
|---------|-----------|-----------|----------------|----------------|------------|
| **Post-Completion False Positive** | -1 | ~40% | Infrastructure | SIGKILL/SIGHUP during cleanup | ✅ False positive - work completed |
| **Git GC Operations** | 137 or -1 | ~15% | Infrastructure | OOM during gc | ✅ Use safe-git-gc scripts |
| **Repository Bloat** | -1 | ~15% (of infra crashes) | Infrastructure | OOM from large repo | ✅ Prevent + gc + monitoring |
| **Service Availability Failure** | 1 | ~8% | External Service | Inference gateway 503/502 | ⚠️ Retry with backoff |
| **Max Turns Exhaustion** | 1 | ~20% | Workflow | Turn limit during bead closing | ✅ False positive - task succeeded |
| **SIGHUP Cascade** | -1 | System-wide event | Infrastructure | Memory pressure (94.71%) | ✅ Infrastructure event |
| **CPU Saturation** | -1 | System-wide event | Infrastructure | 4.46x load (31.21 on 7 cores) | ✅ Infrastructure event |

**Note:** 70% of crashes are infrastructure events, 20% are workflow issues, 8% are service failures, and only 2% are actual code defects.

---

## Pattern 1: Post-Completion False Positives (~40% of crashes)

### Characteristics

- **Exit Code:** -1 (SIGKILL/SIGHUP)
- **Timing:** Crash occurs AFTER successful task completion
- **Evidence:** Work committed, documented, and preserved
- **Root Cause:** Process termination during cleanup/idle time, not task failure

### Detection

```bash
# Check if work completed before crash
git log --since="<crash_timestamp-30min>" --until="<crash_timestamp+30min>" --oneline

# If commit exists < 30 seconds before crash → FALSE POSITIVE
commit_time=$(git log -1 --format=%ct <commit_hash>)
crash_time=$(date -d "<crash_timestamp>" +%s)
gap=$((crash_time - commit_time))

if [ $gap -lt 30 ]; then
  echo "FALSE POSITIVE: Work completed $gap seconds before crash"
fi
```

### Example Timeline (bf-5tgsk)

```
16:35:54 UTC - Investigation completed, commit 549aa42 created
16:36:24 UTC - Agent terminated (SIGKILL, exit code -1)
16:36:51 UTC - Bead closed successfully
```

**Time Gap:** 30 seconds between completion and termination

### Prevention Measures

**Already In Place:**
- ✅ None - this is an infrastructure signal pattern, not a preventable code issue

**Recommended:**
- ⚠️ NEEDLE system fix: Implement work completion detection before generating crash alerts
- ⚠️ Check bead status (in_progress → closed transition) before alerting

### Resolution

- **Classification:** FALSE POSITIVE
- **Action Required:** NONE
- **Documentation:** Update bead notes with "false positive - infrastructure event during post-processing"
- **Bead Status:** Close with clear notes

---

## Pattern 2: Git GC Operations (~15% of crashes)

### Characteristics

- **Exit Code:** 137 (SIGKILL from OOM) or -1 (SIGKILL/SIGHUP)
- **Context:** `git gc --aggressive` or `git gc` in progress
- **Memory:** High memory usage during git operation (1-8GB range)
- **Root Cause:** Bare `git gc --aggressive` without memory limits

### Detection

```bash
# Check if git gc completed successfully
git count-objects -vH
git fsck --full

# Verify repository integrity
du -sh .git
```

### Evidence from bf-173o7e

- Git gc completed successfully in 6 minutes
- Repository optimized: ~18GB → 445MB (97.5% reduction)
- Peak memory usage: 1.1GB (well within 49GB available)
- No OOM events occurred
- Repository integrity verified (fsck passed)

### Prevention Measures

**Already In Place:**
- ✅ `scripts/safe-git-gc.sh` - Memory-limited git gc with monitoring
- ✅ `scripts/safe-git-gc-monitor.sh` - Progress tracking
- ✅ Three-stage gc strategy (standard → incremental → deep compression)
- ✅ Checkpoint/resume capability after each stage

**Recommended:**
- ⚠️ **ALWAYS use** `./scripts/safe-git-gc.sh --full` instead of bare `git gc --aggressive`
- ⚠️ Add pre-flight repository health check before gc
- ⚠️ Monitor gc memory usage with `scripts/safe-git-gc-monitor.sh --watch`

**When NOT to use --aggressive:**
- On systems with < 8GB RAM
- On repos > 5GB without memory limits
- Without monitoring/resumability
- On shared systems where OOM affects others

**When --aggressive is Acceptable:**
- On dedicated systems with > 16GB free RAM
- With cgroup memory limits in place
- For one-time optimization of large repos
- When safe-git-gc scripts are unavailable

### Resolution

- **Classification:** GIT GC OPERATION (not a crash - gc succeeded)
- **Action Required:** Use safe-git-gc scripts for future gc operations
- **Verification:** Check repository integrity with `git fsck --full`
- **Documentation:** Note gc completion in bead notes

---

## Pattern 3: Repository Bloat Crashes (~15% of infrastructure crashes)

### Characteristics

- **Exit Code:** -1 (SIGKILL from OOM killer)
- **Repository Size:** > 5GB (should be <500MB)
- **Loose Objects:** > 1GB (should be packed)
- **Pattern:** Multiple crashes over short period (all exit code -1)
- **Root Cause:** `.beads/*.jsonl` files committed to git history, causing massive repository growth

### Evidence from bf-4yjq (2026-08-12)

- **9 crashes over 2.5 hours**, all exit code -1 (SIGKILL from OOM)
- Repository: 18GB with 17GB loose objects (should be <500MB)
- `.beads/issues.jsonl`: 248MB (should be <5MB)
- Root cause: Bead bf-2ildm committed 17+ identical 237MB JSONL files
- Any significant git operation triggered OOM due to repository bloat
- Cleanup result: 18GB → 138MB (99.2% reduction)
- Task completed successfully after cleanup

### Detection

```bash
# Check repository health
du -sh .git
git count-objects -vH

# Automated health check
./scripts/check-repo-health.sh

# Monitor continuously
./scripts/repo-health-monitor.sh
```

**Repository Size Limits:**

| Metric | Healthy | Warning | Critical | Action Required |
|--------|---------|---------|----------|-----------------|
| **Total Repository Size** | <500MB | 500MB-1GB | >1GB | Immediate cleanup |
| **Loose Objects** | <100MB | 100MB-500MB | >500MB | Run git gc |
| **Loose Object Count** | <100 | 100-1000 | >1000 | Pack objects |
| **Size Ratio (Loose:Packed)** | <1:10 | 1:10 to 1:2 | >1:2 | Inverted - critical |

### Prevention Measures

**Already In Place:**
- ✅ `scripts/check-repo-health.sh` - Repository health monitoring
- ✅ `scripts/safe-git-gc.sh` - Memory-limited git gc operations
- ✅ `scripts/safe-git-gc-monitor.sh` - GC progress monitoring
- ✅ `scripts/repo-health-monitor.sh` - Continuous monitoring
- ✅ `scripts/monitoring-setup.sh` - Automated monitoring installation

**Recommended:**
- ⚠️ **IMMEDIATE:** Add `.beads/*.jsonl` to `.gitignore`
- ⚠️ **IMMEDIATE:** Install pre-commit hooks to block large files (>10MB)
- ⚠️ **IMMEDIATE:** Run `./scripts/safe-git-gc.sh --full` to clean bloated repository
- ⚠️ Enable automated monitoring: `./scripts/monitoring-setup.sh`
- ⚠️ Schedule weekly gc in crontab

**GitIgnore Configuration:**
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

**Pre-commit Hook:**
```bash
# Install hook to block large files
./scripts/setup-git-hooks.sh
```

**Automated Monitoring:**
```bash
# Install continuous monitoring
./scripts/monitoring-setup.sh

# This installs cron jobs for:
# - Crash pattern detection: every 10 minutes
# - Resource monitoring: every 5 minutes
# - Service monitoring: every 2 minutes
# - Repository health monitoring: every hour
```

### Resolution

- **Classification:** REPOSITORY BLOAT (infrastructure issue, not code defect)
- **Immediate Action:** 
  1. Add `.beads/` to `.gitignore`
  2. Run `./scripts/safe-git-gc.sh --full`
  3. Enable monitoring: `./scripts/monitoring-setup.sh`
- **Verification:** Check repository size after cleanup (should be <500MB)
- **Documentation:** Document root cause bead (e.g., bf-2ildm) that created large commits

---

## Pattern 4: Service Availability Failure (~8% of crashes)

### Characteristics

- **Exit Code:** 1 (application error)
- **Error:** HTTP 503 "no available server" or HTTP 502
- **Service:** Inference gateway (traefik-apexalgo-iad.tail1b1987.ts.net:8444)
- **Root Cause:** External service dependency unavailable
- **Domain-Check Code:** ✅ Not involved

### Evidence from domchk-c9641ac5

- Exit Code: 1 (application error)
- Root Cause: HTTP 503 "no available server" from inference gateway
- Error Source: traefik-apexalgo-iad.tail1b1987.ts.net:8444
- Classification: External Service Dependency Failure
- Domain-Check Code: ✅ Not involved

### Detection

```bash
# Check service availability
curl -sf --max-time 5 https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health

# Or use preflight check script
./scripts/preflight-health-check.sh --verbose
```

### Prevention Measures

**Already In Place:**
- ✅ `scripts/preflight-health-check.sh` - Pre-flight service availability check
- ✅ Exponential backoff retry pattern documented

**Recommended:**
- ⚠️ **IMMEDIATE:** Run pre-flight health check before starting agent tasks
- ⚠️ Implement exponential backoff retry for transient failures (3+ attempts)
- ⚠️ Configure secondary inference gateway for failover

**Pre-Flight Health Check:**
```bash
#!/bin/bash
# Before starting agent task
if ! ./scripts/preflight-health-check.sh; then
  echo "ERROR: System health check failed"
  echo "Task deferred until system is healthy"
  exit 1
fi

# Task proceeds knowing resources are sufficient
./agent-task.sh
```

**Exponential Backoff Retry:**
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
    exit 1  # Non-transient error
  fi
done
```

### Resolution

- **Classification:** SERVICE FAILURE (external dependency, not code defect)
- **Immediate Action:** Check gateway status, retry when service restored
- **Verification:** Service health check passes
- **Documentation:** Note service availability issue in bead notes
- **NO code changes required**

---

## Pattern 5: Max Turns Exhaustion (~20% of crashes)

### Characteristics

- **Exit Code:** 1 with "error_max_turns"
- **Context:** Agent exhausted 30-turn limit during post-task operations
- **Task Outcome:** ✅ Main task completed successfully
- **Root Cause:** Agent spent turns on post-task troubleshooting (bead closing loops)
- **Domain-Check Code:** ✅ No defects

### Detection

```bash
# Read trace file for last actions
jq -r '.[] | select(.type == "tool_call") | .tool' .beads/traces/<id>/trace.jsonl | tail -20

# Check for task completion markers
# - Git commits with task-related changes
# - Test results showing success
# - Documentation indicating completion
```

### Evidence from bf-173o7e

- Exit Code: 1 (error_max_turns)
- Root Cause: Agent turn limit exhaustion during bead closing
- Task Outcome: ✅ Git gc completed successfully (97.5% size reduction)
- Classification: FALSE POSITIVE - Post-completion workflow failure
- Domain-Check Code: ✅ No defects

### Prevention Measures

**Already In Place:**
- ✅ None - this is a NEEDLE agent workflow limitation

**Recommended:**
- ⚠️ **IMMEDIATE:** Increase max turns limit for administrative tasks (30 → 50)
- ⚠️ Implement task completion detection to stop post-completion troubleshooting
- ⚠️ Add non-interactive bead closing mode (--force-bypass flag)

**Task Completion Detection:**
```yaml
# Agent workflow logic
task_complete: false
max_post_completion_turns: 5

if task_objectives_achieved:
  task_complete = true
  post_completion_turn_count = 0

if task_complete:
  post_completion_turn_count += 1
  
  if post_completion_turn_count > max_post_completion_turns:
    log_warning("Task complete but closing failed - marking as success anyway")
    bead_update(status: "completed", notes: "Task succeeded, closing workflow failed")
    exit 0  # Success, not failure
```

### Resolution

- **Classification:** FALSE POSITIVE - WORKFLOW ISSUE (not code defect)
- **Verification:** Main task completed successfully (check commits, tests, docs)
- **Action Required:** Document task completion status in bead notes
- **NO code changes required**

---

## Pattern 6: SIGHUP Cascade (System-wide infrastructure event)

### Characteristics

- **Exit Code:** -1 (SIGHUP signal)
- **Scale:** 10+ crashes within 10 minutes across all workers
- **Context:** Memory pressure event (94.71%) → systemd-oomd activation
- **Root Cause:** Infrastructure event, not application-specific defect
- **Timeline:** 2026-08-16 12:00-17:00 UTC (5 hours)

### Evidence from Comprehensive Investigation

- **Memory Pressure:** 94.71% (exceeds 80% threshold)
- **Duration:** >20 seconds above threshold
- **Total Crashes:** 201+ across all beads in 5-hour window
- **Affected Workers:** lab-domain-check, lab-drawrace, lab-test-fix, lab-roam-1
- **OOM Kill:** Git process (PID 1933332) with 12GB RSS

**System Logs:**
```
Aug 16 12:00:59 systemd-oomd: Considered 19 cgroups for killing
Aug 16 12:00:59 systemd-oomd: Killed /user.slice/user-1001.slice/...
Aug 16 12:00:59 systemd-oomd: Memory Pressure: 94.71% > 80.00% for > 20s
Aug 16 12:01:15 kernel: Out of memory: Killed process 1933332 (git)
```

### Detection

```bash
# Check for system-wide events
journalctl --since "<crash_timestamp-1hour>" --until "<crash_timestamp+1hour>" | grep -E "oom|kill|memory"

# Check crash surge pattern
crash_count=$(bead list --since "10min ago" --status "crashed" --json | jq '. | length')
if [ $crash_count -gt 10 ]; then
  echo "INFRASTRUCTURE EVENT: $crash_count crashes in 10 minutes"
fi
```

### Prevention Measures

**Already In Place:**
- ✅ System monitoring via scripts/monitoring-setup.sh
- ✅ Resource monitoring every 5 minutes
- ✅ Crash pattern detection every 10 minutes

**Recommended:**
- ⚠️ Implement memory pressure alerting (70% threshold before 80% OOM)
- ⚠️ Implement crash surge detection (10+ crashes in 10 minutes → single infrastructure event alert)
- ⚠️ OOM event tracking and dashboard integration

**Monitoring Alerts:**
```yaml
monitoring:
  alerts:
    - name: HighMemoryPressure
      expr: node_memory_pressure_percentage > 70
      for: 1m
      annotations:
        summary: "Memory pressure above 70% - OOM risk"
    
    - name: CrashSurgeDetected
      expr: needle_crashes_total{outcome="failed"} > 10
      for: 10m
      annotations:
        summary: "Infrastructure event: 10+ crashes in 10 minutes"
```

### Resolution

- **Classification:** INFRASTRUCTURE EVENT (system-wide, not code defect)
- **Verification:** Multiple workers affected simultaneously (not isolated to one task)
- **Action Required:** Document infrastructure event, monitor system resources
- **NO code changes required**

---

## Pattern 7: CPU Saturation (System-wide infrastructure event)

### Characteristics

- **Exit Code:** -1 (process termination due to system unresponsiveness)
- **Scale:** 826 crashes in single day (worst crash day on record)
- **Context:** 4.46x CPU load saturation (31.21 on 7 cores)
- **Root Cause:** System became unresponsive, processes terminated abnormally
- **Timeline:** 2026-08-16 (same day as SIGHUP cascade)

### Evidence

- Peak Load: 31.21 on 7 cores (4.46x saturation)
- Worst Crash Day: 826 crashes on 2026-08-16
- System State: Unresponsive, processes terminated abnormally
- All Workers Affected: Not selective to specific tasks

### Detection

```bash
# Check system load
uptime
# Output: load average: 2.89, 3.34, 3.10 (1min, 5min, 15min)

# Calculate CPU saturation
cpu_count=$(nproc)
load_1min=$(uptime | awk -F'load average:' '{print $2}' | awk '{print $1}' | tr -d ',')
saturation=$(echo "$load_1min / $cpu_count" | bc -l)

echo "CPU Saturation: ${saturation}x (load: ${load_1min}, cores: ${cpu_count})"
```

### Prevention Measures

**Already In Place:**
- ✅ CPU load monitoring via scripts/resource-monitor.sh
- ✅ Automated monitoring every 5 minutes

**Recommended:**
- ⚠️ Implement CPU saturation alerting (> 3.0x load)
- ⚠️ Agent cgroup resource limits to prevent runaway processes
- ⚠️ Graceful shutdown on SIGTERM for clean process termination

**Monitoring Alerts:**
```yaml
monitoring:
  alerts:
    - name: HighCPUSaturation
      expr: (node_load1 / node_cpu_count) > 3.0
      for: 5m
      annotations:
        summary: "CPU saturation above 3.0x - system unresponsive"
```

### Resolution

- **Classification:** INFRASTRUCTURE EVENT (system-wide, not code defect)
- **Verification:** All workers affected equally (not task-specific)
- **Action Required:** Monitor system resources, implement CPU saturation alerts
- **NO code changes required**

---

## Retry Behavior and Manual Intervention

### Automatic Retry Behavior

**NEEDLE System Automatic Retry:**
- Transient failures are automatically retried
- Crash → retry → success pattern is common
- Example (bf-6bio4g):
  ```
  Attempt 1: 2026-08-16 17:17:10 → 17:21:31 (crash, exit -1)
  Attempt 2: 2026-08-16 22:32:16 → 22:34:51 (success, exit 0)
  Attempt 3: 2026-08-17 13:16:02 → 13:18:04 (success, exit 0)
  ```

**Success Rate:** ~90% of transient failures recover via automatic retry

### When Manual Intervention is Required

**Escalation Triggers:**

1. **Repository Corruption Suspected**
   - `git fsck` shows errors
   - Objects missing or corrupted
   - Repo size unexpectedly large (>1GB)
   - Action: Run `./scripts/safe-git-gc.sh --full`

2. **Persistent Service Failures**
   - Service down for > 30 minutes
   - Multiple retries fail with same error
   - No service status information available
   - Action: Defer task until service restored

3. **Unknown Exit Codes**
   - Exit code not in classification table
   - No recognizable error pattern
   - Multiple unexplained crashes
   - Action: Investigate crash artifacts, check system logs

4. **Data Loss Suspected**
   - Work artifacts missing
   - Expected commits not found
   - Test results inconsistent
   - Action: Verify repository integrity, check git history

### Manual Intervention Steps

**Step 1: Classify the Crash**
```bash
# Get bead metadata
bead show <id> --json

# Check exit code and pattern
# Exit code -1 → Infrastructure event
# Exit code 1 with error_max_turns → Workflow failure
# Exit code 1 with HTTP 503/502 → Service failure
# Other → Standard investigation
```

**Step 2: Check System Resources**
```bash
# Resource health check
free -h                    # Memory: Need 10GB+ available
df -h /                    # Disk: Need 20GB+ free
uptime                     # Load: Should be < 10 on 1min average

# Or use preflight check
./scripts/preflight-health-check.sh
```

**Step 3: Verify Work Completion**
```bash
# Check for commits around crash time
git log --since="<crash_timestamp-30min>" --until="<crash_timestamp+30min>" --oneline

# Check repository health
./scripts/check-repo-health.sh
```

**Step 4: Take Action**
```bash
# If repository bloated (>1GB):
./scripts/safe-git-gc.sh --full

# If service down:
# Check service status, defer task until restored

# If work completed:
# Document false positive, close bead with notes

# If actual code issue:
# Investigate crash artifacts, fix code, verify tests
```

**Step 5: Document and Close**
```bash
# Update bead with findings
bead update <id> --notes "Investigation findings and classification"

# Close bead
bead close <id> --reason "Crash investigated and resolved: <classification>"
```

---

## Key Learnings

### What Causes Crashes

1. **Infrastructure Events (70%)**: Memory pressure, OOM killer, SIGHUP cascade, **repository bloat (18GB → OOM)**
2. **Workflow Failures (20%)**: Max turns exhaustion, bead closing loops
3. **Service Failures (8%)**: Inference gateway unavailable, network issues
4. **Code Defects (2%)**: Actual application errors — **NONE found in domain-check**

### What Does NOT Cause Crashes

1. ✅ **Domain-check code** — No defects found in any crash investigation
2. ✅ **Git GC operations** — When using safe-git-gc scripts with memory limits
3. ✅ **Normal application operations** — Well within resource limits
4. ✅ **Repository maintenance** — With proper monitoring and pre-flight checks

### Critical Repository Bloat Finding

**The bf-1s6c3 crash (2026-08-12) was caused by:**
- Repository: 18GB with 17GB loose objects (should be <500MB)
- `.beads/issues.jsonl`: 248MB (should be <5MB)
- Root cause: Bead bf-2ildm committed 17+ identical 237MB JSONL files
- Result: 9 OOM crashes over 2.5 hours (all exit code -1)

**Resolution:**
- Repository cleanup: 18GB → 138MB (99.2% reduction)
- Task completed successfully after cleanup
- No code defects found — purely infrastructure issue

**Prevention:**
- Add `.beads/*.jsonl` to `.gitignore`
- Install pre-commit hooks to block large files
- Run `./scripts/check-repo-health.sh` weekly
- Enable automated monitoring: `./scripts/monitoring-setup.sh`

---

## Safe Operating Limits

| Resource | Minimum | Warning | Critical | Action Required |
|----------|---------|---------|----------|-----------------|
| **Available Memory** | 20GB | 10GB | 5GB | Abort task, investigate memory usage |
| **Disk Space** | 50GB | 30GB | 20GB | Abort task, clean disk space |
| **CPU Load (1min)** | < 5 | < 10 | > 15 | Defer task, investigate saturation |
| **Git GC Memory** | 1GB | 2GB | 4GB | Use safe-git-gc with memory limits |
| **Repository Size** | <500MB | 500MB-1GB | >1GB | Run git gc immediately |
| **Loose Objects** | <100MB | 100MB-500MB | >500MB | Run git gc to pack objects |

---

## Related Documentation

- **Crash Response Guide:** `docs/crash-response-guide.md` — Comprehensive investigation procedures
- **Mitigation Strategies:** `docs/crash-mitigation-strategies.md` — Detailed prevention proposals
- **Comprehensive Investigation:** `docs/comprehensive-crash-investigation-report-2026-09-01.md`
- **Specific Crashes:**
  - `docs/crash-analysis-domchk-c9641ac5-2026-09-01.md` (Service availability)
  - `docs/investigation-summary-bf-173o7e-2026-09-01.md` (Git GC false positive)
  - `docs/crash-artifacts-bf-4yjq.md` (Repository bloat - 9 OOM crashes from 18GB repo)

---

**Document Status:** ✅ Complete  
**Last Updated:** 2026-09-01  
**Target Audience:** Agents investigating crash patterns and implementing prevention  
**Purpose:** Consolidated crash patterns reference with prevention measures and resolution procedures
