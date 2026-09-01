# Crash Prevention and Mitigation Strategies

**Created:** 2026-09-01  
**Purpose:** Concrete solutions to prevent agent crashes based on root cause analysis  
**Related Analysis:** `docs/crash-analysis-domchk-c9641ac5-2026-09-01.md`, `docs/investigation-summary-bf-173o7e-2026-09-01.md`

---

## Executive Summary

**Critical Finding:** The analyzed crashes were NOT caused by git gc or domain-check code defects. Root causes were:
1. **Service Availability Failure** (HTTP 503 from inference gateway)
2. **Administrative Workflow Failure** (max turns limit during bead closing)

**Recommendation:** Focus on agent system infrastructure and workflow improvements, NOT domain-check code changes.

---

## Root Cause Analysis Summary

### Crash 1: Bead domchk-c9641ac5 (Service Availability)

| Attribute | Value |
|-----------|-------|
| **Exit Code** | 1 (application error) |
| **Root Cause** | HTTP 503 "no available server" from inference gateway |
| **Error Source** | traefik-apexalgo-iad.tail1b1987.ts.net:8444 |
| **Classification** | External Service Dependency Failure |
| **Domain-Check Code** | ✅ Not involved |

### Crash 2: Bead bf-173o7e (False Positive)

| Attribute | Value |
|-----------|-------|
| **Exit Code** | 1 (error_max_turns) |
| **Root Cause** | Agent turn limit exhaustion during bead closing |
| **Task Outcome** | ✅ Git gc completed successfully (97.5% size reduction) |
| **Classification** | FALSE POSITIVE - Post-completion workflow failure |
| **Domain-Check Code** | ✅ No defects |

---

## Ranked Mitigation Proposals

### Priority 1: Service Availability Resilience (CRITICAL)

**Addresses:** Crash 1 (HTTP 503 from inference gateway)

#### Proposal 1.1: Exponential Backoff Retry for Transient Failures

**Problem:** Agent terminated immediately on HTTP 503 instead of retrying.

**Implementation:**
```bash
# Agent-level retry wrapper (pseudocode)
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

**Risk:** Low - standard pattern for external API calls
**Effort:** Medium - requires agent framework modification
**Timeline:** Short-term (1-2 weeks)

#### Proposal 1.2: Multiple Inference Gateway Failover

**Problem:** Single point of failure on traefik-apexalgo-iad gateway.

**Implementation:**
1. Configure secondary inference gateway endpoint
2. Agent attempts primary gateway first
3. On 503/502 errors, failover to secondary gateway
4. Circuit breaker pattern after N consecutive failures

**Risk:** Low - failover is transparent to agent
**Effort:** High - requires infrastructure setup
**Timeline:** Long-term (1-2 months)

#### Proposal 1.3: Pre-Flight Service Health Checks

**Problem:** Agent started task without verifying inference gateway availability.

**Implementation:**
```bash
#!/bin/bash
# Pre-flight health check
HEALTH_URL="https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health"

if ! curl -sf --max-time 5 "$HEALTH_URL" > /dev/null; then
  echo "ERROR: Inference gateway unavailable"
  echo "Deferring task until service is healthy"
  exit 1  # Task will be retried later
fi

echo "Gateway healthy - proceeding with task"
```

**Risk:** Very low - read-only check
**Effort:** Low - simple HTTP check
**Timeline:** Short-term (1 week)

---

### Priority 2: Agent Workflow Improvements (HIGH)

**Addresses:** Crash 2 (max turns limit during bead closing)

#### Proposal 2.1: Increase Max Turns Limit for Administrative Tasks

**Problem:** Agent exhausted 30-turn limit during bead closing troubleshooting.

**Implementation:**
```yaml
# Agent configuration
task_types:
  administrative:
    max_turns: 50  # Increased from 30
    description: "Tasks involving bead management, cleanup, or workflow operations"
  
  standard:
    max_turns: 30
    description: "Regular development tasks"
```

**Risk:** Low - only affects administrative tasks
**Effort:** Low - configuration change
**Timeline:** Immediate (can be deployed now)

#### Proposal 2.2: Non-Interactive Bead Closing Mode

**Problem:** Agent got stuck in troubleshooting loop during bead closing.

**Implementation:**
```bash
# Add --force-bypass flag to bead CLI
bead close <id> --reason "..." --force-bypass-workflow

# This flag:
# - Skips verification steps
# - Suppresses interactive prompts
# - Forces close even if sub-steps fail
# - Used only by agents, not humans
```

**Risk:** Medium - could close beads with incomplete work
**Mitigation:** Require explicit --agent flag to enable
**Effort:** Medium - CLI enhancement
**Timeline:** Short-term (2-3 weeks)

#### Proposal 2.3: Task Completion Detection

**Problem:** Agent continued attempting bead closing after task was complete.

**Implementation:**
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

**Risk:** Very low - task already succeeded
**Effort:** Low - agent workflow logic
**Timeline:** Short-term (1-2 weeks)

---

### Priority 3: Git GC Operation Safety (MEDIUM)

**Addresses:** General concern about git gc resource usage

#### Proposal 3.1: Use Safe Git GC Scripts (ALREADY IMPLEMENTED)

**Status:** ✅ **Complete** - Scripts already exist at `scripts/safe-git-gc.sh`

**Features:**
- ✅ Memory-limited operations (configurable via `SAFE_GC_MEMORY_MAX`)
- ✅ Three-stage gc strategy (standard → incremental → deep compression)
- ✅ Checkpoint/resume capability
- ✅ Progress tracking and monitoring
- ✅ Pre-flight integrity checks

**Evidence from crash analysis:**
- Git gc completed successfully in 6 minutes
- Repository optimized from ~18GB to 445MB (97.5% reduction)
- Peak memory usage: 1.1GB (well within limits)
- No OOM events occurred
- Repository integrity verified

**Recommendation:** Use existing safe-git-gc scripts instead of bare `git gc --aggressive`.

#### Proposal 3.2: Git GC Resource Monitoring

**Problem:** No visibility into git gc memory usage during execution.

**Implementation:**
```bash
# Monitor git gc process
while kill -0 $GIT_GC_PID 2>/dev/null; do
  memory=$(ps -o rss= -p $GIT_GC_PID | awk '{print $1/1024 " MB"}')
  echo "[$(date)] Git GC memory: $memory"
  
  if [[ $(ps -o rss= -p $GIT_GC_PID) -gt $MAX_MEMORY_KB ]]; then
    echo "WARNING: Memory limit exceeded, terminating git gc"
    kill -TERM $GIT_GC_PID
    exit 1
  fi
  
  sleep 5
done
```

**Risk:** Very low - monitoring only
**Effort:** Low - add to safe-git-gc.sh
**Timeline:** Short-term (1 week)

#### Proposal 3.3: Git GC Under Cgroup Limits

**Problem:** No hard limit on git gc memory usage.

**Implementation:**
```bash
# Run git gc under systemd slice with memory limit
systemd-run --scope --quiet \
  -p MemoryMax=2g \
  -p MemorySwapMax=0 \
  -p CPUQuota=200% \
  scripts/safe-git-gc.sh --full
```

**Risk:** Low - OOM will terminate gc, but repository remains intact
**Effort:** Low - wrapper script
**Timeline:** Short-term (1 week)

---

### Priority 4: Monitoring and Alerting (MEDIUM)

**Addresses:** Lack of visibility into agent health and service availability

#### Proposal 4.1: Inference Gateway Health Monitoring

**Problem:** No monitoring of inference gateway availability.

**Implementation:**
```yaml
# Prometheus monitoring
monitoring:
  endpoints:
    - name: inference_gateway_health
      url: https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health
      interval: 30s
      timeout: 5s
      
  alerts:
    - name: InferenceGatewayDown
      expr: up{job="inference_gateway"} == 0
      for: 1m
      annotations:
        summary: "Inference gateway is down"
        description: "Agents will fail until gateway is restored"
```

**Risk:** Very low - read-only monitoring
**Effort:** Medium - requires Prometheus setup
**Timeline:** Long-term (1-2 months)

#### Proposal 4.2: Agent Task Duration Monitoring

**Problem:** No alerting on abnormally long-running tasks.

**Implementation:**
```yaml
monitoring:
  metrics:
    - name: needle_agent_task_duration_seconds
      type: histogram
      buckets: [60, 300, 600, 1800, 3600]
      
  alerts:
    - name: NeedleAgentTaskStuck
      expr: needle_agent_task_duration_seconds{outcome="running"} > 7200
      for: 10m
      annotations:
        summary: "Agent task running > 2 hours"
```

**Risk:** Very low - monitoring only
**Effort:** Low - add metrics to agent
**Timeline:** Short-term (2-3 weeks)

#### Proposal 4.3: Crash Pattern Detection

**Problem:** No automated detection of systematic crash patterns.

**Implementation:**
```bash
# Analyze crash patterns
crash_pattern_detection() {
  recent_crashes=$(bead list --status "crashed" --since "24h" --json | jq '. | length')
  
  if [[ $recent_crashes -gt 5 ]]; then
    echo "WARNING: High crash rate detected (past 24h)"
    
    # Classify crashes by exit code
    bead list --status "crashed" --since "24h" --json | \
      jq -r 'group_by(.exit_code) | map({exit_code: .[0].exit_code, count: length})'
    
    # Alert if systematic pattern found
    if [[ systematic_pattern_detected ]]; then
      send_alert "Systematic crash pattern detected - investigate immediately"
    fi
  fi
}
```

**Risk:** Very low - analysis only
**Effort:** Low - cron job + script
**Timeline:** Short-term (1-2 weeks)

---

### Priority 5: Process Isolation and Fault Tolerance (LOW)

**Addresses:** System stability and crash containment

#### Proposal 5.1: Agent Cgroup Resource Limits

**Problem:** No resource limits on agent processes.

**Implementation:**
```bash
# Run agent under systemd slice
systemd-run --scope --quiet \
  -p MemoryMax=4g \
  -p MemorySwapMax=1g \
  -p CPUQuota=300% \
  -p TasksMax=100 \
  needle agent <task_id>
```

**Risk:** Low - prevents runaway memory/CPU
**Effort:** Low - wrapper in needle agent launcher
**Timeline:** Medium-term (1 month)

#### Proposal 5.2: Agent Graceful Shutdown on SIGTERM

**Problem:** Agent may not handle SIGTERM gracefully.

**Implementation:**
```go
// Agent signal handler
func setupSignalHandlers() {
  sigTermChan := make(chan os.Signal, 1)
  signal.Notify(sigTermChan, syscall.SIGTERM, syscall.SIGINT)
  
  go func() {
    <-sigTermChan
    log.Info("Received termination signal - initiating graceful shutdown")
    
    // Save current state
    saveCheckpoint()
    
    // Close bead with appropriate status
    if taskInProgress {
       beadUpdate(status: "interrupted", notes: "Terminated by signal")
    }
    
    os.Exit(0)  # Clean exit, not crash
  }()
}
```

**Risk:** Very low - signal handler is standard practice
**Effort:** Medium - requires agent code changes
**Timeline:** Medium-term (1-2 months)

#### Proposal 5.3: Crash Recovery Workflow

**Problem:** No automated recovery from transient failures.

**Implementation:**
```yaml
# Needle automatic retry policy
retry_policy:
  transient_errors:
    - error_max_turns
    - exit_code_1_with_503_response
    
  max_retries: 3
  backoff: exponential
  base_delay: 60s
  
  on_final_failure:
    status: "failed_transient"
    notes: "Failed after N retries - requires manual intervention"
```

**Risk:** Low - only retries transient failures
**Effort:** Medium - needle configuration
**Timeline:** Medium-term (1 month)

---

## Recommendation on Git GC --aggressive

### Is `git gc --aggressive` Safe to Use?

**Answer:** ⚠️ **Use with caution - prefer safe-git-gc scripts instead**

#### Evidence from Crash Analysis

The git gc --aggressive operation in bead bf-173o7e:
- ✅ **Completed successfully** in ~6 minutes
- ✅ **No OOM occurred** (1.1GB peak usage vs 49GB available)
- ✅ **Repository integrity verified** (fsck passed)
- ✅ **97.5% size reduction** achieved (~18GB → 445MB)
- ✅ **No code defects** found in domain-check

#### Why Prefer Safe Scripts Over --aggressive

| Aspect | `git gc --aggressive` | `safe-git-gc.sh` |
|--------|----------------------|------------------|
| **Memory Usage** | Unbounded (4-8GB possible) | Capped (2GB max) |
| **Progress Visibility** | None | Full monitoring |
| **Resumability** | ❌ All-or-nothing | ✅ Checkpoint at each stage |
| **Time** | 2-4 hours on large repos | 1-2 hours staged |
| **Safety** | Can OOM on large repos | Memory-limited operations |
| **Monitoring** | No metrics | Real-time status |

#### Recommendations

**For Active Development Repos:**
```bash
# Use safe-git-gc scripts
./scripts/safe-git-gc.sh --full  # Stages 1-3 with monitoring
```

**For Archive/Large Repos:**
```bash
# Stage 3 with memory limit
SAFE_GC_MEMORY_MAX=2g ./scripts/safe-git-gc.sh --full
```

**When NOT to Use --aggressive:**
- On systems with < 8GB RAM
- On repos > 5GB without memory limits
- Without monitoring/resumability
- On shared systems where OOM affects others

**When --aggressive is Acceptable:**
- On dedicated systems with plenty of RAM (> 16GB free)
- With cgroup memory limits
- For one-time optimization of large repos
- When safe-git-gc scripts are unavailable

---

## Implementation Roadmap

### Phase 1: Immediate (0-2 weeks)

| Proposal | Priority | Effort | Timeline |
|----------|----------|--------|----------|
| 1.3 Pre-Flight Service Health Checks | P1 | Low | 1 week |
| 2.1 Increase Max Turns for Admin Tasks | P2 | Low | Immediate |
| 2.3 Task Completion Detection | P2 | Low | 1 week |
| 3.2 Git GC Resource Monitoring | P3 | Low | 1 week |
| 4.2 Agent Task Duration Monitoring | P4 | Low | 2 weeks |
| 4.3 Crash Pattern Detection | P4 | Low | 1 week |

### Phase 2: Short-term (2-6 weeks)

| Proposal | Priority | Effort | Timeline |
|----------|----------|--------|----------|
| 1.1 Exponential Backoff Retry | P1 | Medium | 2-3 weeks |
| 2.2 Non-Interactive Bead Closing | P2 | Medium | 2-3 weeks |
| 3.3 Git GC Cgroup Limits | P3 | Low | 1 week |
| 5.1 Agent Cgroup Resource Limits | P5 | Low | 2 weeks |

### Phase 3: Long-term (1-3 months)

| Proposal | Priority | Effort | Timeline |
|----------|----------|--------|----------|
| 1.2 Multiple Inference Gateway Failover | P1 | High | 1-2 months |
| 4.1 Inference Gateway Health Monitoring | P4 | Medium | 1-2 months |
| 5.2 Agent Graceful Shutdown on SIGTERM | P5 | Medium | 1-2 months |
| 5.3 Crash Recovery Workflow | P5 | Medium | 1 month |

---

## Risk Assessment Summary

| Proposal | Risk Level | Risk Mitigation |
|----------|------------|-----------------|
| 1.1 Exponential Backoff Retry | Low | Standard pattern, tested extensively |
| 1.2 Gateway Failover | Low | Failover is transparent |
| 1.3 Pre-Flight Health Checks | Very Low | Read-only check |
| 2.1 Increase Max Turns Limit | Low | Only affects admin tasks |
| 2.2 Non-Interactive Bead Closing | Medium | Requires --agent flag, audit trail |
| 2.3 Task Completion Detection | Very Low | Task already succeeded |
| 3.2 Git GC Monitoring | Very Low | Monitoring only |
| 3.3 Git GC Cgroup Limits | Low | OOM terminates gc, repo remains intact |
| 4.1 Gateway Health Monitoring | Very Low | Read-only monitoring |
| 4.2 Task Duration Monitoring | Very Low | Monitoring only |
| 4.3 Crash Pattern Detection | Very Low | Analysis only |
| 5.1 Agent Cgroup Limits | Low | Prevents runaway, standard practice |
| 5.2 Agent Graceful Shutdown | Very Low | Signal handler is standard |
| 5.3 Crash Recovery Workflow | Low | Only retries transient failures |

---

## Success Metrics

### Service Availability Resilience
- ✅ Agent retries on HTTP 503 (3+ attempts with backoff)
- ✅ Pre-flight health checks prevent doomed tasks
- ✅ Gateway failover reduces single-point-of-failure risk

### Agent Workflow Improvements
- ✅ Administrative tasks complete within turn limits
- ✅ Bead closing succeeds without troubleshooting loops
- ✅ Task completion is detected and logged correctly

### Git GC Safety
- ✅ All git gc operations use safe-git-gc scripts
- ✅ Memory usage monitored and capped
- ✅ Resumability prevents lost progress

### Monitoring and Alerting
- ✅ Gateway health is visible in dashboards
- ✅ Long-running tasks trigger alerts
- ✅ Crash patterns are detected automatically

### Process Isolation
- ✅ Agents run under resource limits
- ✅ Graceful shutdown on SIGTERM
- ✅ Transient failures are automatically retried

---

## Conclusion

The crash analysis revealed that domain-check code is **NOT defective**. The crashes were caused by:
1. External service dependency failures (inference gateway)
2. Agent workflow limitations (max turns, bead closing)

**Recommendation:** Implement agent system improvements (Priority 1-2) and use existing safe-git-gc scripts (Priority 3). No changes to domain-check code are required.

**Immediate Actions:**
1. Enable pre-flight health checks (Proposal 1.3)
2. Increase max turns for administrative tasks (Proposal 2.1)
3. Use `scripts/safe-git-gc.sh --full` instead of bare `git gc --aggressive`

---

**Status:** ✅ Mitigation strategies defined  
**Next Steps:** Implement Phase 1 proposals (immediate timeline)  
**Tracking:** Bead domchk-61505afc

---

**Document Version:** 1.0  
**Created:** 2026-09-01  
**Author:** Claude Code Agent  
**Review Status:** Ready for implementation
