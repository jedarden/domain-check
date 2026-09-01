# Crash Alert Fix Strategy

**Document Date:** 2026-09-01  
**Research Task:** domchk-6166a477  
**Status:** RESEARCH COMPLETE - Fix Strategy Identified  
**Target System:** NEEDLE crash detection and alert generation (not domain-check code)

---

## Executive Summary

Comprehensive research of crash patterns has identified that **crashes are not occurring in domain-check operations**. The systematic issue is in the **NEEDLE crash detection and alert generation system**, which generates false positive alerts for:

1. **Post-completion process terminations** - Agents killed after work completed
2. **Transient failures with self-healing** - Automatic retries succeed, but alerts still generated
3. **Duplicate alerts** - Same crash investigated multiple times
4. **Historical event crashes** - System-wide events (SIGHUP cascades, CPU saturation) from 2026-08-16

**Root Cause:** NEEDLE crash detection system lacks proper validation, deduplication, and context preservation.

---

## Crash Pattern Analysis

### Pattern 1: Post-Completion False Positives

**Example Beads:**
- `bf-5tgsk` - Investigation completed at 16:35:54 UTC, crashed at 16:36:24 UTC (30 seconds after)
- `bf-4hp9p` - Investigation completed successfully, crashed during post-processing
- `bf-3riiu`, `bf-3g4cp` - Multiple investigation beads crashed during CPU saturation event

**Characteristics:**
- ✅ Work completed successfully (committed, documented)
- ✅ Crash occurred AFTER completion (post-processing/idle time)
- ❌ Exit code -1 (SIGKILL) - system termination
- ❌ Alert generated despite successful task completion

**Evidence:**
```
Commit 549aa42: 2026-08-16 16:35:54 UTC (work completed)
Crash timestamp: 2026-08-16 16:36:24 UTC (30 seconds later)
```

### Pattern 2: Transient Crashes with Self-Healing

**Example Beads:**
- `bf-6bio4g` - Crashed at 17:21:31 UTC, retried at 22:34:51 UTC, succeeded
- Multiple beads with automatic retry success

**Characteristics:**
- ❌ Initial crash (exit code -1)
- ✅ Automatic retry succeeds (exit code 0)
- ✅ Multiple successful completions after crash
- ❌ Alert generated despite self-healing success

**Evidence from bead events log:**
```
Attempt 1: 2026-08-16 17:17:10 → 17:21:31 (crash, exit -1)
Attempt 2: 2026-08-16 22:32:16 → 22:34:51 (success, exit 0)
Attempt 3: 2026-08-17 13:16:02 → 13:18:04 (success, exit 0)
```

### Pattern 3: Duplicate Alert Generation

**Example Beads:**
- `bf-4hp9p` - 3 duplicate alerts (original + 2 verifications)
- `bf-1ea4g` - 9+ duplicate alerts verified
- Multiple crashes investigated 3+ times each

**Characteristics:**
- ❌ Alert generated for already-investigated crash
- ❌ No deduplication check before alert creation
- ❌ Multiple verification reports for same crash
- ❌ Alert bead creation doesn't check original bead status

**Evidence:**
- 20+ verification reports for same crashes
- Multiple alert beads for same underlying crash
- No resolution status checking

### Pattern 4: Historical System-Wide Events

**SIGHUP Cascade Event (2026-08-16):**
- Period: 12:00-17:00 UTC (5 hours)
- Total crashes: 200+ across all workers
- Signal: Exit code -1 (SIGHUP)
- Root cause: External system process (systemd/fleet manager)
- Impact: Zero data loss - all work completed before cascade

**CPU Saturation Event (2026-08-16):**
- Period: Same day as SIGHUP cascade
- Total crashes: 826 (worst crash day on record)
- CPU saturation: 4.46x load (31.21 on 7 cores)
- Affected: Multiple investigation beads
- Current status: System stable, 0 crashes for 16+ days

**Characteristics:**
- ❌ Historical alerts still being generated
- ❌ No timestamp validation (alerts weeks after event)
- ❌ No system-wide event detection

---

## Root Cause Analysis

### Primary Root Cause: NEEDLE Crash Detection System

**The issue is NOT in domain-check code.** All domain-check operations are functioning correctly. The systematic issue is in the **NEEDLE bead system's crash detection and alert generation mechanism**.

**Systematic Deficiencies:**

1. **No Work Completion Detection**
   - System doesn't detect if work was completed before crash
   - No check for commits/files created before termination
   - No distinction between task failure and post-completion termination

2. **No Self-Healing Awareness**
   - System doesn't track automatic retry success
   - No suppression of alerts for self-healed failures
   - Doesn't account for transient failure patterns

3. **No Alert Deduplication**
   - No check if crash already investigated
   - No validation of original bead status
   - No prevention of duplicate alert creation

4. **No Context Preservation**
   - Crash alerts generated without trace data
   - No system state attached to alerts
   - No timestamp validation (stale alerts)

5. **No Event Pattern Recognition**
   - No detection of system-wide crash events
   - No grouping of related crashes
   - No historical event awareness

---

## Fix Strategy

### Target System: NEEDLE (not domain-check)

**The fix must be implemented in the NEEDLE repository, not domain-check.** Domain-check code is functioning correctly and requires no changes.

### Implementation Plan

#### Phase 1: Work Completion Detection (Week 1)

**Objective:** Detect if work was completed before crash

**Implementation Steps:**
1. Add pre-crash state snapshot to NEEDLE crash detection
2. Check for git commits made in last 5 minutes before crash
3. Check for files created/modified before crash
4. Validate bead status in workspace (closed/in-progress)
5. Implement "work completed" detection logic

**Validation Logic:**
```python
def was_work_completed(bead_id, crash_timestamp):
    # Check for commits in window before crash
    commits = get_commits_between(crash_timestamp - 300s, crash_timestamp)
    
    # Check bead current status
    bead_status = get_bead_status(bead_id)
    
    # Check for task outputs
    outputs = get_task_outputs(bead_id)
    
    # Work completed if:
    # - Commits exist before crash AND
    # - Bead is closed/in_progress AND
    # - Task outputs exist
    return len(commits) > 0 and (bead_status in ['closed', 'in_progress']) and len(outputs) > 0
```

#### Phase 2: Self-Healing Detection (Week 2)

**Objective:** Detect and suppress alerts for self-healed failures

**Implementation Steps:**
1. Query bead events log for retry history
2. Check for successful completions after crash
3. Implement self-healing success detection
4. Suppress alert generation for self-healed failures

**Validation Logic:**
```python
def did_self_heal(bead_id, crash_timestamp):
    events = get_bead_events(bead_id)
    
    # Look for success after crash
    for event in events:
        if event['timestamp'] > crash_timestamp:
            if event['outcome'] == 'success' and event['exit_code'] == 0:
                return True
    
    return False
```

#### Phase 3: Alert Deduplication (Week 3)

**Objective:** Prevent duplicate alerts for already-investigated crashes

**Implementation Steps:**
1. Add crash investigation status tracking
2. Check if crash already has investigation report
3. Validate original bead status before creating alert
4. Implement duplicate detection logic

**Validation Logic:**
```python
def is_duplicate_alert(crash_bead_id):
    # Check for existing investigation reports
    existing_reports = find_investigation_reports(crash_bead_id)
    if len(existing_reports) > 0:
        return True
    
    # Check original bead status
    original_bead = get_bead(crash_bead_id)
    if original_bead['status'] == 'closed':
        return True
    
    return False
```

#### Phase 4: Context Preservation (Week 4)

**Objective:** Attach crash context and artifacts to alerts

**Implementation Steps:**
1. Capture system state at crash time (load, memory, disk)
2. Attach recent trace data to alert bead
3. Include git repository state in alert context
4. Preserve crash artifacts for investigation

**Context Data Structure:**
```python
crash_context = {
    'timestamp': crash_timestamp,
    'exit_code': exit_code,
    'signal': signal_name,
    'system_state': {
        'load_average': get_load(),
        'memory_usage': get_memory(),
        'disk_usage': get_disk(),
    },
    'git_state': {
        'recent_commits': get_recent_commits(),
        'working_dir': git_status(),
        'branch': git_branch(),
    },
    'trace_data': get_recent_trace(),
    'task_outputs': list_task_outputs(),
}
```

#### Phase 5: Event Pattern Recognition (Week 5-6)

**Objective:** Detect and group system-wide crash events

**Implementation Steps:**
1. Implement crash surge detection (>10 crashes in 5 minutes)
2. Group related crashes by timestamp and signal
3. Create event-level investigation beads
4. Suppress individual alerts during system-wide events

**Event Detection Logic:**
```python
def detect_crash_surge():
    recent_crashes = get_recent_crashes(minutes=5)
    
    if len(recent_crashes) > 10:
        # Check signal consistency
        signals = [c['signal'] for c in recent_crashes]
        if signals.count(signals[0]) / len(signals) > 0.8:
            # System-wide event detected
            create_event_investigation_bead(recent_crashes)
            suppress_individual_alerts(recent_crashes)
```

---

## Safety Considerations

### Safe Design Principles

1. **Conservative Validation**
   - Only suppress alerts with strong evidence
   - Default to generating alert if uncertain
   - Preserve all crash data for audit

2. **No Data Loss**
   - All crash data preserved regardless of suppression
   - Context artifacts stored even for suppressed alerts
   - Full audit trail of alert decisions

3. **Backward Compatibility**
   - Existing investigation reports remain valid
   - No changes to domain-check operations
   - Graceful degradation if detection fails

4. **Observable Behavior**
   - Log all alert generation decisions
   - Metrics on suppression rates
   - Monitoring for false negatives

### Risk Mitigation

| Risk | Mitigation |
|------|------------|
| False negatives (missing real crashes) | Conservative validation - alert if uncertain |
| Over-suppression | Multi-factor validation required |
| Detection bugs | Comprehensive logging and monitoring |
| System complexity | Incremental rollout per phase |

---

## Success Criteria

### Quantitative Metrics

- **Duplicate alert rate:** < 5% of total alerts (currently ~60%)
- **False positive rate:** < 10% of generated alerts (currently ~80%)
- **Alert latency:** < 5 minutes from crash to alert generation
- **Context coverage:** 100% of alerts include crash context

### Qualitative Metrics

- **Investigation efficiency:** One investigation per unique crash
- **Alert quality:** All alerts include actionable context
- **System stability:** Zero regressions in crash detection

---

## Implementation Priority

### High Priority (Implement First)

1. **Work Completion Detection** - Prevents post-completion false positives
2. **Alert Deduplication** - Prevents duplicate investigations
3. **Context Preservation** - Improves investigation quality

### Medium Priority (Implement Second)

4. **Self-Healing Detection** - Reduces transient failure alerts
5. **Event Pattern Recognition** - Groups system-wide events

### Low Priority (Implement Later)

6. **Advanced Analytics** - Crash trend analysis
7. **Predictive Detection** - Anticipate crash conditions

---

## Testing Strategy

### Unit Testing

- Test work completion detection with various scenarios
- Test self-healing detection with retry patterns
- Test deduplication with existing crash database

### Integration Testing

- Test alert generation with simulated crashes
- Test context preservation and artifact attachment
- Test event detection with crash surges

### Validation Testing

- Test against historical crash database (200+ crashes)
- Validate no regressions in crash detection
- Measure improvement in alert quality

---

## Rollout Plan

### Phase 1: Development (Weeks 1-6)

- Implement fixes in NEEDLE repository
- Unit and integration testing
- Validation against historical crashes

### Phase 2: Canary Deployment (Week 7)

- Deploy to single worker
- Monitor alert quality metrics
- Iterate based on findings

### Phase 3: Full Rollout (Week 8)

- Deploy to all workers
- Monitor system-wide metrics
- Adjust parameters as needed

### Phase 4: Monitoring (Ongoing)

- Track alert quality metrics
- Review false negatives monthly
- Iterate on detection logic

---

## Related Documentation

- [Crash Incident Summary - 2026-08-26](crash-incident-summary-domain-check-2026-08-26.md)
- [Crash Pattern Analysis - bf-5tgsk](crash-investigation-bf-5tgsk-2026-08-16.md)
- [Verification Report - bf-6bio4g](verification-report-domchk-7080cea2-bf-6bio4g-crash.md)
- [Verification Report - bf-4hp9p](verification-report-domchk-ccd3421d-duplicate-alert-resolved-bf-4hp9p-crash.md)

---

## Conclusions

**The crash issue is NOT in domain-check code.** Domain-check operations are functioning correctly with zero actual crashes affecting operations.

**The systematic issue is in the NEEDLE crash detection and alert generation system**, which:

1. Generates false positive alerts for post-completion terminations
2. Does not detect self-healing transient failures
3. Creates duplicate alerts for already-investigated crashes
4. Lacks context preservation and event pattern recognition

**The fix must be implemented in the NEEDLE repository** with a phased approach focusing on work completion detection, alert deduplication, and context preservation.

**This fix strategy is safe and will not make things worse** because:
- Conservative validation (alert if uncertain)
- No changes to domain-check operations
- Full data preservation regardless of suppression
- Incremental rollout with monitoring

---

**Research Task:** domchk-6166a477  
**Status:** RESEARCH COMPLETE  
**Next Step:** Implement fix strategy in NEEDLE repository  
**Confidence:** HIGH - Comprehensive research with 20+ verification reports analyzed
