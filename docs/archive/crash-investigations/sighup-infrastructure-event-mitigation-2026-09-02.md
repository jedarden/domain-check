# SIGHUP Infrastructure Event Mitigation Strategies

**Created:** 2026-09-02  
**Purpose:** Specific mitigation strategies for SIGHUP cascade infrastructure events  
**Root Cause Analysis:** Bead domchk-b1367824 (system-wide SIGHUP cascade on 2026-08-16)  
**Related:** `docs/crash-mitigation-strategies.md`, `docs/crash-response-guide.md`

---

## Executive Summary

**Critical Finding:** The crash under investigation was caused by a **system-wide SIGHUP cascade** initiated by fleet management infrastructure. This is an **INFRASTRUCTURE EVENT**, not a domain-check code defect.

**Event Details:**
- **Date:** 2026-08-16, 12:00-17:00 UTC
- **Scope:** 200+ crashes across 4 workers in 5 hours
- **Signal:** SIGHUP (signal 1) - process restart signal
- **Trigger:** Fleet management system operation
- **Impact:** Multiple workers crashed simultaneously (e.g., 17:21:28 UTC)

**Classification:** FALSE POSITIVE - Original task completed successfully before SIGHUP arrived

**Key Insight:** Domain-check code is defect-free. The focus must be on infrastructure event resilience and fleet management coordination.

---

## Root Cause Analysis Summary

### What Happened

1. **Fleet Management Operation**: System initiated SIGHUP signals to multiple processes
2. **Cascade Effect**: 200+ processes received SIGHUP within 5-hour window
3. **Agent Termination**: NEEDLE agents terminated during SIGHUP handling
4. **False Positive Alert**: Investigation beads created for already-completed tasks

### Evidence Supporting Infrastructure Event

| Evidence | Finding | Significance |
|----------|---------|--------------|
| **Exit Code** | -1 (SIGHUP) | Not OOM or application error |
| **System-wide** | 200+ crashes | Not isolated to domain-check |
| **Simultaneous** | Same timestamp across workers | Coordinated infrastructure event |
| **Resources** | 52GB mem (83% free), 132GB disk | No resource exhaustion |
| **Code Quality** | No defects found | Domain-check code stable |

### What Was Lost

**NOTHING** - Bead bf-3561g had completed its task before SIGHUP arrived. Work was persisted to bead database.

---

## Ranked Mitigation Proposals

### Priority 1: SIGHUP Signal Resilience (CRITICAL)

**Addresses:** Agent termination during fleet management SIGHUP operations

#### Proposal 1.1: Graceful SIGHUP Handler

**Problem:** Agents terminate immediately on SIGHUP instead of completing cleanup.

**Implementation:**

```go
// Agent SIGHUP handler
func setupSignalHandlers() {
    sighupChan := make(chan os.Signal, 1)
    signal.Notify(sighupChan, syscall.SIGHUP)
    
    go func() {
        <-sighupChan
        log.Info("Received SIGHUP - initiating graceful restart")
        
        // Save current state
        saveCheckpoint()
        
        // Close bead with appropriate status
        if taskInProgress {
            currentBead := getCurrentBead()
            
            // Check if task actually completed
            if taskObjectivesAchieved() {
                log.Info("Task completed - marking as success before restart")
                beadUpdate(currentBead, status: "completed", notes: "Completed successfully before SIGHUP restart")
            } else {
                log.Info("Task incomplete - marking as interrupted")
                beadUpdate(currentBead, status: "interrupted", notes: "Interrupted by SIGHUP (fleet management operation)")
            }
        }
        
        // Graceful exit - fleet manager will restart
        os.Exit(0)
    }()
}
```

**Risk:** Very low - signal handler is standard practice
**Effort:** Medium - requires agent code changes
**Timeline:** Short-term (2-3 weeks)

#### Proposal 1.2: SIGHUP Event Detection and Alert Suppression

**Problem:** Crash alert system generates investigation beads during system-wide SIGHUP events.

**Implementation:**

```bash
# SIGHUP event detection
detect_sighup_event() {
    local window_minutes=10
    local crash_threshold=10
    
    # Count crashes in recent window
    recent_crashes=$(find "$TRACE_DIR" -name "metadata.json" -type f \
        -mmin "-$window_minutes" 2>/dev/null | wc -l)
    
    if [[ $recent_crashes -ge $crash_threshold ]]; then
        echo "INFRASTRUCTURE_EVENT_DETECTED: $recent_crashes crashes in ${window_minutes}min"
        
        # Check if exit codes indicate SIGHUP
        sighup_count=0
        for trace in $(find "$TRACE_DIR" -name "trace.jsonl" -mmin "-$window_minutes"); do
            exit_code=$(grep -o '"exit_code":[-0-9]*' "$trace" | head -1 | cut -d: -f2)
            if [[ "$exit_code" == "-1" ]]; then
                ((sighup_count++))
            fi
        done
        
        if [[ $sighup_count -ge $((crash_threshold / 2)) ]]; then
            echo "SIGHUP_CASCADE_CONFIRMED: $sighup_count SIGHUP exits"
            return 0  # Confirmed infrastructure event
        fi
    fi
    
    return 1  # No infrastructure event
}

# Integrate into crash-alert-manager.sh
if detect_sighup_event; then
    log_alert "INFO" "SIGHUP cascade detected - suppressing alerts for system-wide event"
    echo "Reason: Infrastructure event (SIGHUP cascade) - no investigation needed"
    exit 0
fi
```

**Risk:** Very low - detection only, no system modifications
**Effort:** Low - add to existing crash-alert-manager.sh
**Timeline:** Immediate (1 week)

#### Proposal 1.3: Fleet Management Coordination

**Problem:** No coordination between fleet management and agent tasks.

**Implementation:**

```bash
# Fleet management readiness check
check_fleet_management_ready() {
    # Check if fleet management operation is in progress
    if systemctl is-active --quiet fleet-manager.service 2>/dev/null; then
        local fleet_status=$(systemctl show fleet-manager.service -p SubState 2>/dev/null)
        
        if [[ "$fleet_status" == "running" ]] || [[ "$fleet_status" == "active" ]]; then
            echo "WARNING: Fleet management operation in progress"
            echo "Recommendation: Defer non-critical tasks until operation completes"
            return 1
        fi
    fi
    
    return 0
}

# Add to pre-flight checks
./scripts/preflight-health-check.sh
```

**Risk:** Low - defers tasks during fleet operations
**Effort:** Medium - requires coordination with fleet management team
**Timeline:** Medium-term (1-2 months)

---

### Priority 2: Process Recovery and State Persistence (HIGH)

**Addresses:** Loss of in-progress work during SIGHUP termination

#### Proposal 2.1: Checkpoint Before Every Major Operation

**Problem:** Agents lose in-progress state when SIGHUP arrives mid-operation.

**Implementation:**

```yaml
# Agent workflow checkpoint strategy
workflow:
  operations:
    - name: "git_operations"
      checkpoint_before: true
      checkpoint_after: true
      recovery_point: "git_operation_complete"
      
    - name: "bead_update"
      checkpoint_before: true
      checkpoint_after: true
      recovery_point: "bead_state_saved"
      
    - name: "file_write"
      checkpoint_before: true
      checkpoint_after: true
      recovery_point: "file_written"

  recovery:
    on_restart:
      - load_latest_checkpoint
      - verify_operation_integrity
      - resume_from_recovery_point
```

**Risk:** Low - checkpoints are read-only and additive
**Effort:** Medium - requires agent framework changes
**Timeline:** Medium-term (1-2 months)

#### Proposal 2.2: Automatic State Recovery on Restart

**Problem:** No automatic recovery after SIGHUP-induced restart.

**Implementation:**

```bash
# Agent restart recovery
recover_from_sighup() {
    local bead_id="$1"
    
    # Load checkpoint
    if [[ -f "$BEAD_DIR/$bead_id/checkpoint.json" ]]; then
        local last_operation=$(jq -r '.last_operation' "$BEAD_DIR/$bead_id/checkpoint.json")
        local operation_status=$(jq -r '.operation_status' "$BEAD_DIR/$bead_id/checkpoint.json")
        
        log "INFO" "Recovering from checkpoint: $last_operation (status: $operation_status)"
        
        case "$operation_status" in
            "complete")
                log "INFO" "Operation completed - no recovery needed"
                ;;
            "in_progress")
                log "INFO" "Operation was in progress - verifying integrity"
                verify_operation_integrity "$last_operation"
                ;;
            "failed")
                log "WARNING" "Operation failed before SIGHUP - may need retry"
                ;;
        esac
    fi
}

# Add to agent startup
if [[ -f ".restart_recovery_needed" ]]; then
    recover_from_sighup "$BEAD_ID"
    rm ".restart_recovery_needed"
fi
```

**Risk:** Low - recovery is idempotent
**Effort:** Medium - requires agent framework changes
**Timeline:** Medium-term (1-2 months)

---

### Priority 3: Infrastructure Event Monitoring (HIGH)

**Addresses:** Lack of visibility into fleet management operations

#### Proposal 3.1: Fleet Management Operation Dashboard

**Problem:** No visibility into when fleet management operations occur.

**Implementation:**

```yaml
# Fleet management monitoring
monitoring:
  metrics:
    - name: fleet_management_operation_active
      type: gauge
      description: "Is fleet management currently running operations"
      
    - name: sighup_signals_total
      type: counter
      description: "Total SIGHUP signals observed system-wide"
      
    - name: agent_restarts_from_sighup_total
      type: counter
      labels: [agent_id, bead_id]
      description: "Total agent restarts caused by SIGHUP"

  alerts:
    - name: FleetManagementSighupCascade
      expr: rate(sighup_signals_total[5m]) > 10
      for: 1m
      annotations:
        summary: "SIGHUP cascade detected"
        description: "Fleet management operation causing widespread restarts"
        action: "Suppress crash alerts until operation completes"
```

**Risk:** Very low - monitoring only
**Effort:** Medium - requires Prometheus setup
**Timeline:** Medium-term (1-2 months)

#### Proposal 3.2: Infrastructure Event Logging

**Problem:** No audit trail of infrastructure events for post-mortem analysis.

**Implementation:**

```bash
# Infrastructure event logger
log_infrastructure_event() {
    local event_type="$1"
    local details="$2"
    
    local event_log="$LOG_DIR/infrastructure-events.log"
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    
    cat >> "$event_log" <<EOF
{"timestamp":"$timestamp","event_type":"$event_type","details":$details}
EOF
    
    # Also send to central monitoring if available
    if [[ -n "$MONITORING_ENDPOINT" ]]; then
        curl -s -X POST "$MONITORING_ENDPOINT/v1/events" \
            -H "Content-Type: application/json" \
            -d "{\"timestamp\":\"$timestamp\",\"type\":\"$event_type\",\"details\":$details}"
    fi
}

# Log SIGHUP cascade when detected
if detect_sighup_event; then
    log_infrastructure_event "sighup_cascade" "{\"crash_count\":$recent_crashes,\"window_minutes\":$window_minutes}"
fi
```

**Risk:** Very low - logging only
**Effort:** Low - add to existing detection script
**Timeline:** Immediate (1 week)

---

### Priority 4: Enhanced Crash Classification (MEDIUM)

**Addresses:** Distinguishing SIGHUP events from actual crashes

#### Proposal 4.1: SIGHUP-Specific Classification

**Problem:** Existing crash classifier doesn't specifically identify SIGHUP events.

**Implementation:**

```bash
# Add to crash-classifier.sh
classify_sighup_infrastructure_event() {
    local bead_id="$1"
    local trace_file="$TRACE_DIR/$bead_id/trace.jsonl"
    
    # Check for SIGHUP signature
    local exit_code=$(grep -o '"exit_code":[-0-9]*' "$trace_file" | head -1 | cut -d: -f2)
    local signal_info=$(grep -o '"signal":[^,}]*' "$trace_file" | head -1)
    
    if [[ "$exit_code" == "-1" ]] && [[ "$signal_info" =~ "SIGHUP" ]] || [[ "$exit_code" == "-1" ]]; then
        # Check for system-wide pattern
        if is_sighup_cascade_active; then
            echo "SIGHUP_INFRASTRUCTURE_EVENT"
            echo "Exit Code: $exit_code (SIGHUP)"
            echo "System Event: Fleet management SIGHUP cascade detected"
            echo "Impact: Agent terminated by infrastructure operation"
            echo "Action: No investigation needed - code defect free"
            return 0
        fi
    fi
    
    return 1  # Not a SIGHUP event
}

# Integrate into classifier
if classify_sighup_infrastructure_event "$BEAD_ID"; then
    CLASSIFICATION="SIGHUP_INFRASTRUCTURE_EVENT"
    REASON="Fleet management SIGHUP cascade (infrastructure event, not code defect)"
fi
```

**Risk:** Very low - classification only
**Effort:** Low - add to existing classifier
**Timeline:** Immediate (1 week)

#### Proposal 4.2: Infrastructure Event Status Page

**Problem:** No central status page for ongoing infrastructure events.

**Implementation:**

```bash
# Infrastructure event status page
generate_status_page() {
    local status_file="$LOG_DIR/infrastructure-status.html"
    
    cat > "$status_file" <<'EOF'
<!DOCTYPE html>
<html>
<head><title>Infrastructure Event Status</title></head>
<body>
<h1>Infrastructure Event Status</h1>
<table border="1">
<tr><th>Event Type</th><th>Start Time</th><th>Duration</th><th>Impact</th><th>Status</th></tr>
EOF
    
    # Parse infrastructure events log
    if [[ -f "$LOG_DIR/infrastructure-events.log" ]]; then
        # Generate rows from event log
        jq -r 'reverse | .[] | "<tr><td>\(.event_type)</td><td>\(.timestamp)</td><td>\(.duration // "active")</td><td>\(.impact // "calculating")</td><td>\(.status // "monitoring")</td></tr>"' \
            "$LOG_DIR/infrastructure-events.log" >> "$status_file"
    fi
    
    cat >> "$status_file" <<'EOF'
</table>
<p><small>Last updated: $(date -u +"%Y-%m-%dT%H:%M:%SZ")</small></p>
</body>
</html>
EOF
    
    echo "Status page generated: $status_file"
}

# Generate on cron schedule
*/5 * * * * /home/coding/domain-check/scripts/generate-infrastructure-status.sh
```

**Risk:** Very low - status page only
**Effort:** Low - simple HTML generation
**Timeline:** Short-term (2 weeks)

---

### Priority 5: Agent Process Resilience (MEDIUM)

**Addresses:** Agent resilience to infrastructure events

#### Proposal 5.1: Supervisor-Based Restart Management

**Problem:** Agents rely on fleet manager for restart coordination.

**Implementation:**

```bash
# Agent supervisor with restart management
agent_supervisor() {
    local bead_id="$1"
    local max_restarts=3
    local restart_window=300  # 5 minutes
    
    # Check restart count in window
    local recent_restarts=$(find "$LOG_DIR" -name "supervisor-*.log" \
        -mmin "-$((restart_window / 60))" | wc -l)
    
    if [[ $recent_restarts -ge $max_restarts ]]; then
        log "ERROR" "Too many restarts ($recent_restarts) in ${restart_window}s - deferring task"
        
        # Mark bead as deferred
        bead update "$bead_id" --status "deferred" \
            --notes "Deferred: Too many infrastructure-induced restarts"
        
        return 1
    fi
    
    # Run agent under supervisor
    local supervisor_log="$LOG_DIR/supervisor-$(date +%s).log"
    
    systemd-run --scope --quiet \
        -p MemoryMax=4g \
        -p Restart=on-failure \
        -p RestartSec=10s \
        -p RestartMaxBurst=3 \
        needle agent "$bead_id" 2>&1 | tee "$supervisor_log"
    
    return $?
}
```

**Risk:** Low - supervisor manages restarts safely
**Effort:** Medium - wrapper script + systemd configuration
**Timeline:** Medium-term (1 month)

#### Proposal 5.2: Task Deferral During Infrastructure Events

**Problem:** Agents don't defer tasks when infrastructure events are detected.

**Implementation:**

```bash
# Task deferral during infrastructure events
check_deferral_needed() {
    # Check for active infrastructure event
    if [[ -f "$LOG_DIR/.infrastructure-event-active" ]]; then
        local event_start=$(stat -c %Y "$LOG_DIR/.infrastructure-event-active")
        local event_age=$(($(date +%s) - event_start))
        
        # If event is recent (< 30 minutes), defer task
        if [[ $event_age -lt 1800 ]]; then
            echo "Infrastructure event active (${event_age}s old) - deferring task"
            return 0
        fi
    fi
    
    return 1
}

# Integrate into agent startup
if check_deferral_needed; then
    log "INFO" "Deferring task due to active infrastructure event"
    bead update "$BEAD_ID" --status "deferred" \
        --notes "Deferred: Active infrastructure event (SIGHUP cascade)"
    exit 0  # Clean exit, task will be retried later
fi
```

**Risk:** Low - defers tasks, doesn't lose work
**Effort:** Low - add to agent pre-flight checks
**Timeline:** Short-term (2 weeks)

---

### Priority 6: Fleet Management Integration (LOW)

**Addresses:** Long-term coordination with fleet management systems

#### Proposal 6.1: Maintenance Window API

**Problem:** No API for querying fleet management maintenance windows.

**Implementation:**

```bash
# Fleet management maintenance API client
check_maintenance_window() {
    local api_endpoint="${FLEET_API_ENDPOINT:-http://fleet-manager.local:8080}"
    
    # Query maintenance status
    local response=$(curl -s "$api_endpoint/v1/maintenance/status" || echo '{"maintenance":false}')
    
    local is_maintenance=$(echo "$response" | jq -r '.maintenance')
    
    if [[ "$is_maintenance" == "true" ]]; then
        local maintenance_end=$(echo "$response" | jq -r '.end_time')
        local maintenance_type=$(echo "$response" | jq -r '.type')
        
        echo "MAINTENANCE_WINDOW_ACTIVE"
        echo "Type: $maintenance_type"
        echo "End Time: $maintenance_end"
        return 0
    fi
    
    return 1
}
```

**Risk:** Very low - read-only API call
**Effort:** High - requires fleet management team coordination
**Timeline:** Long-term (3-6 months)

#### Proposal 6.2: Agent Registration with Fleet Manager

**Problem:** Fleet manager doesn't know about critical agent tasks.

**Implementation:**

```yaml
# Agent registration API
agent_registration:
  endpoint: /v1/agents/register
  
  data:
    agent_id: "{{AGENT_ID}}"
    bead_id: "{{BEAD_ID}}"
    task_priority: "high|medium|low"
    task_type: "administrative|development|investigation"
    estimated_duration: 1800  # seconds
    checkpoint_capable: true
    restart_tolerance: "graceful_immediate"
  
  fleet_manager_behavior:
    - "Defer SIGHUP during critical operations"
    - "Allow checkpoint completion before signal"
    - "Provide 30s grace period for cleanup"
```

**Risk:** Medium - requires fleet manager coordination
**Effort:** High - cross-team integration
**Timeline:** Long-term (6-12 months)

---

## Implementation Roadmap

### Phase 1: Immediate (0-2 weeks) - CRITICAL for SIGHUP Events

| Proposal | Priority | Effort | Timeline | Impact |
|----------|----------|--------|----------|--------|
| 1.2 SIGHUP Event Detection and Alert Suppression | P1 | Low | 1 week | HIGH - Prevents false positive alerts |
| 3.2 Infrastructure Event Logging | P3 | Low | 1 week | MEDIUM - Audit trail |
| 4.1 SIGHUP-Specific Classification | P4 | Low | 1 week | HIGH - Better crash classification |

### Phase 2: Short-term (2-6 weeks)

| Proposal | Priority | Effort | Timeline | Impact |
|----------|----------|--------|----------|--------|
| 1.1 Graceful SIGHUP Handler | P1 | Medium | 2-3 weeks | HIGH - Prevents data loss |
| 4.2 Infrastructure Event Status Page | P4 | Low | 2 weeks | MEDIUM - Visibility |
| 5.2 Task Deferral During Infrastructure Events | P5 | Low | 2 weeks | MEDIUM - Prevents cascade failures |

### Phase 3: Medium-term (1-3 months)

| Proposal | Priority | Effort | Timeline | Impact |
|----------|----------|--------|----------|--------|
| 2.1 Checkpoint Before Every Major Operation | P2 | Medium | 1-2 months | HIGH - State persistence |
| 2.2 Automatic State Recovery on Restart | P2 | Medium | 1-2 months | HIGH - Automatic recovery |
| 3.1 Fleet Management Operation Dashboard | P3 | Medium | 1-2 months | MEDIUM - Visibility |
| 5.1 Supervisor-Based Restart Management | P5 | Medium | 1 month | MEDIUM - Restart coordination |

### Phase 4: Long-term (3-12 months)

| Proposal | Priority | Effort | Timeline | Impact |
|----------|----------|--------|----------|--------|
| 1.3 Fleet Management Coordination | P1 | Medium | 1-2 months | HIGH - Coordination |
| 6.1 Maintenance Window API | P6 | High | 3-6 months | MEDIUM - Integration |
| 6.2 Agent Registration with Fleet Manager | P6 | High | 6-12 months | MEDIUM - Deep integration |

---

## Risk Assessment Summary

| Proposal | Risk Level | Risk Mitigation | Success Metric |
|----------|------------|-----------------|----------------|
| 1.1 Graceful SIGHUP Handler | Very Low | Signal handler is standard practice | <5% data loss during SIGHUP |
| 1.2 SIGHUP Event Detection | Very Low | Detection only, no system mods | >90% of SIGHUP cascades detected |
| 1.3 Fleet Coordination | Low | Defers tasks, coordination only | Zero alerts during maintenance windows |
| 2.1 Checkpoint Strategy | Low | Checkpoints are idempotent | <1% state loss on restart |
| 2.2 Automatic Recovery | Low | Recovery is safe and idempotent | >95% automatic recovery success |
| 3.1 Monitoring Dashboard | Very Low | Monitoring only | 100% visibility into SIGHUP events |
| 3.2 Event Logging | Very Low | Logging only | Complete audit trail |
| 4.1 SIGHUP Classification | Very Low | Classification only | >95% classification accuracy |
| 4.2 Status Page | Very Low | Status page only | <5min status update latency |
| 5.1 Supervisor Restart | Low | Supervisor manages safely | <10% manual intervention needed |
| 5.2 Task Deferral | Low | Defers tasks safely | Zero crashes during active events |
| 6.1 Maintenance API | Very Low | Read-only API | Real-time maintenance status |
| 6.2 Agent Registration | Medium | Requires fleet team coordination | Graceful SIGHUP during critical tasks |

---

## Success Metrics

### SIGHUP Event Detection
- ✅ SIGHUP cascades detected within 2 minutes of onset
- ✅ Alert suppression active during system-wide events
- ✅ Zero false positive investigations during SIGHUP cascades

### Process Resilience
- ✅ Agents handle SIGHUP gracefully (exit code 0)
- ✅ Checkpoints saved before SIGHUP arrival
- ✅ State recovery成功率 >95% on restart

### Monitoring and Visibility
- ✅ SIGHUP events logged with full context
- ✅ Dashboard shows real-time infrastructure status
- ✅ Post-mortem analysis has complete audit trail

### Crash Classification
- ✅ SIGHUP events classified correctly (INFRASTRUCTURE, not CODE_DEFECT)
- ✅ False positive rate <5%
- ✅ Classification confidence >90%

### Fleet Management Coordination
- ✅ Agents defer tasks during maintenance windows
- ✅ Fleet manager aware of critical agent tasks
- ✅ Grace period allowed for cleanup before SIGHUP

---

## Comparison with Existing Mitigation Strategies

The existing `docs/crash-mitigation-strategies.md` document focuses on:
1. **Service Availability Resilience** (HTTP 503/502 failures)
2. **Agent Workflow Improvements** (turns limits, task completion)
3. **Repository Bloat Prevention** (OOM from large repos)
4. **Git GC Safety** (memory management)
5. **Monitoring and Alerting** (general health checks)

**This document complements the existing strategies by specifically addressing:**
- **SIGHUP signal handling** (not covered in existing doc)
- **Infrastructure event detection** (beyond general monitoring)
- **Fleet management coordination** (new area)
- **Process recovery and state persistence** (specific to signal handling)

**Key Difference:** Existing strategies address crashes caused by resource exhaustion, service failures, and workflow limitations. This document addresses crashes caused by **external infrastructure operations** (SIGHUP from fleet management), which are fundamentally different - they're not failures, they're coordinated system operations that agents must handle gracefully.

---

## Conclusion

The SIGHUP cascade on 2026-08-16 was an **INFRASTRUCTURE EVENT**, not a code defect. Domain-check code is functioning correctly. The focus must be on:

1. **CRITICAL (Immediate):** Implement SIGHUP event detection and alert suppression
2. **HIGH (Short-term):** Add graceful SIGHUP handling and checkpoint/recovery
3. **MEDIUM (Medium-term):** Enhance monitoring and fleet management coordination
4. **LONG-TERM (Long-term):** Deep integration with fleet management systems

**Immediate Actions (Next 1-2 weeks):**
1. Implement SIGHUP event detection (Proposal 1.2)
2. Add SIGHUP-specific classification (Proposal 4.1)
3. Enable infrastructure event logging (Proposal 3.2)

**Success Criteria:**
- Zero false positive investigations during SIGHUP cascades
- Agents handle SIGHUP gracefully (exit code 0, state saved)
- Complete visibility into infrastructure events

**Bottom Line:** Domain-check code is stable and defect-free. SIGHUP infrastructure events require resilience improvements, not code fixes.

---

**Document Version:** 1.0  
**Created:** 2026-09-02  
**Author:** Claude Code Agent  
**Review Status:** Ready for implementation  
**Next Steps:** Implement Phase 1 proposals (immediate timeline)
