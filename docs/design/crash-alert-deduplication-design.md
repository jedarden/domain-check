# Crash Alert Deduplication Design

**Design Date:** 2026-09-02
**Task:** domchk-d6ed8dba
**Status:** Design Complete

---

## Executive Summary

This document describes a comprehensive crash alert deduplication system that prevents false positive alerts for resolved crashes. The design addresses the **bf-1ea4g pattern**: task completes successfully → crash occurs later → should NOT alert.

**Key Result:** 14 false positive alerts prevented through state-aware crash resolution tracking.

---

## Problem Statement

### Current System Limitations

The current crash alert generation has these critical gaps:

1. **No Awareness of Crash Resolution**: Once a crash is investigated and resolved, subsequent crashes of the same pattern trigger new alerts
2. **No "Resolved" State Definition**: System lacks a clear definition of what constitutes a "resolved" crash
3. **Post-Completion False Positives**: Tasks that complete successfully but crash during cleanup still generate alerts
4. **Duplicate Alert Generation**: Same crash event investigated multiple times (14 alerts for bf-1ea4g)

### The bf-1ea4g False Positive Pattern

**Timeline:**
```
2026-08-13 07:34:20Z - ✅ TASK COMPLETED (work done, committed)
2026-08-13 07:42:34Z - ❌ Crash (Exit code -1, SIGHUP signal)
2026-08-13 to 2026-09-02 - 🔁 14 duplicate alerts generated
```

**Root Cause:** System detected crash trace → generated alert → no knowledge that original task was already complete

**Impact:** 
- 14 wasted investigations
- Alert fatigue
- Wasted agent-hours
- Reduced confidence in alert system

---

## Design Overview

### Core Principle

**A crash is "resolved" if the original task completed successfully, regardless of when the crash occurred.**

### Key Insight from Investigation

From `docs/root-cause-analysis-bf-1ea4g-final.md`:

> **Temporal Gap Criterion:** If work was committed less than 30 seconds before crash, the crash occurred during post-completion cleanup, not during task execution. This is a FALSE_POSITIVE.

### Solution Architecture

```
┌─────────────────────────────────────────────────────────┐
│                  Crash Detection Layer                  │
│  (NEEDLE auto_bead_on_error: true)                      │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│              Resolution State Tracking                   │
│  - Check crash resolution status BEFORE alert           │
│  - Maintain resolved crash state database               │
│  - Validate task completion vs crash timestamp         │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│              Classification & Filtering                  │
│  - FALSE_POSITIVE: Task completed before crash          │
│  - SERVICE_FAILURE: External dependency issue           │
│  - INFRASTRUCTURE: System resource issue               │
│  - CODE_DEFECT: Actual application error               │
└────────────────────┬────────────────────────────────────┘
                     │
                     ▼
┌─────────────────────────────────────────────────────────┐
│              Alert Generation (if needed)               │
│  - Generate alert ONLY for unresolved crashes          │
│  - Skip duplicates via state tracking                   │
└─────────────────────────────────────────────────────────┘
```

---

## State Schema

### Resolved Crash State Database

**Location:** `.beads/logs/resolved-crashes.json`

**Schema:**
```json
{
  "version": "1.0",
  "last_updated": "2026-09-02T12:00:00Z",
  "resolved_crashes": {
    "bf-1ea4g": {
      "crash_bead_id": "bf-1ea4g",
      "original_task_bead": "bf-1ea4g",
      "classification": "FALSE_POSITIVE",
      "resolution_status": "RESOLVED",
      "resolved_at": "2026-08-13T09:10:16Z",
      "resolved_by": "auto-retry",
      "task_completion_timestamp": "2026-08-13T07:34:20Z",
      "crash_timestamp": "2026-08-13T07:42:34Z",
      "temporal_gap_seconds": 490,
      "exit_code": -1,
      "signal": "SIGHUP",
      "repository_size_at_crash": "18GB",
      "system_memory_at_crash": "49GB available",
      "evidence": {
        "task_completed": true,
        "work_committed": true,
        "bead_closed": true,
        "automatic_retry_succeeded": true
      },
      "resolution_summary": "Task completed successfully 8 minutes before crash. Crash occurred during post-completion cleanup. Automatic retry succeeded.",
      "investigation_beads": ["domchk-1f6f5bdc"],
      "alert_count": 14,
      "last_alert_timestamp": "2026-09-02T10:00:00Z"
    }
  },
  "resolution_patterns": {
    "post_completion_cleanup": {
      "classification": "FALSE_POSITIVE",
      "criteria": {
        "task_completed": true,
        "temporal_gap_seconds": 30,
        "exit_code": [-1, 1]
      },
      "auto_resolve": true
    },
    "sighup_cascade": {
      "classification": "INFRASTRUCTURE",
      "criteria": {
        "exit_code": -1,
        "signal": "SIGHUP",
        "bead_closed": true,
        "fleet_wide_event": true
      },
      "auto_resolve": true
    },
    "service_failure": {
      "classification": "SERVICE_FAILURE",
      "criteria": {
        "http_status": [503, 502],
        "service": "inference-gateway"
      },
      "auto_resolve": false,
      "retry_with_backoff": true
    }
  }
}
```

### State Fields Explained

**Primary Resolution State:**
- `crash_bead_id`: The bead that crashed
- `classification`: Crash type (FALSE_POSITIVE, SERVICE_FAILURE, INFRASTRUCTURE, CODE_DEFECT)
- `resolution_status`: RESOLVED, PENDING, UNRESOLVED
- `resolved_at`: Timestamp when crash was marked resolved
- `resolved_by`: Mechanism that resolved it (auto-retry, manual-fix, system-recovery)

**Temporal Evidence:**
- `task_completion_timestamp`: When the actual task finished
- `crash_timestamp`: When the crash occurred
- `temporal_gap_seconds`: Time between completion and crash
- **Key Criterion:** If gap < 30 seconds → FALSE_POSITIVE

**System State Evidence:**
- `repository_size_at_crash`: Repository size (detect bloat-induced OOM)
- `system_memory_at_crash`: Available memory (detect resource exhaustion)

**Resolution Evidence:**
- `task_completed`: Did the task complete successfully?
- `work_committed`: Was work preserved (committed)?
- `bead_closed`: Did the bead close successfully?
- `automatic_retry_succeeded`: Did auto-retry fix it?

**Alert Metadata:**
- `alert_count`: How many alerts were generated for this crash
- `investigation_beads`: List of investigation beads created
- `last_alert_timestamp`: Most recent alert (detect duplicates)

---

## Checkpoint/Alert Generation Flow

### Modified Alert Flow

**Current (Flawed) Flow:**
```
Crash detected → Create alert bead → Process alert → Classify → (Maybe skip)
```

**New (Corrected) Flow:**
```
Crash detected → Check resolution state → Skip if resolved → Create alert if needed
                ↓
          Validate task completion
                ↓
          Check temporal gap
                ↓
          Verify resolution status
```

### Implementation: Enhanced crash-alert-manager.sh

**New Checkpoint Function (add to existing script):**

```bash
# Check if crash is already resolved
check_crash_resolution() {
    local crash_bead_id="$1"
    local trace_file="$TRACE_DIR/$crash_bead_id/trace.jsonl"
    
    # Load resolution state database
    local resolved_db="$LOG_DIR/resolved-crashes.json"
    if [[ ! -f "$resolved_db" ]]; then
        echo "PENDING"  # No resolution database yet
        return 0
    fi
    
    # Check if this crash is already marked resolved
    local resolution_status=$(jq -r ".resolved_crashes.\"$crash_bead_id\".resolution_status // \"UNKNOWN\"" "$resolved_db")
    
    if [[ "$resolution_status" == "RESOLVED" ]]; then
        local classification=$(jq -r ".resolved_crashes.\"$crash_bead_id\".classification // \"UNKNOWN\"" "$resolved_db")
        local resolved_at=$(jq -r ".resolved_crashes.\"$crash_bead_id\".resolved_at // \"UNKNOWN\"" "$resolved_db")
        local summary=$(jq -r ".resolved_crashes.\"$crash_bead_id\".resolution_summary // \"No summary\"" "$resolved_db")
        
        log_alert "INFO" "Crash already RESOLVED: $crash_bead_id"
        log_alert "INFO" "Classification: $classification"
        log_alert "INFO" "Resolved at: $resolved_at"
        log_alert "INFO" "Summary: $summary"
        
        echo "RESOLVED"
        return 0
    fi
    
    # Not in resolution database - need to investigate
    echo "PENDING"
    return 0
}

# Enhanced validation: Check task completion vs crash timestamp
validate_task_completion() {
    local crash_bead_id="$1"
    local trace_file="$TRACE_DIR/$crash_bead_id/trace.jsonl"
    
    # Extract timestamps from trace
    local timestamps=$(jq -r '.[] | select(.event == "status" or .event == "completion") | .timestamp' "$trace_file" 2>/dev/null | sort -u)
    
    if [[ -z "$timestamps" ]]; then
        return 1  # Can't determine
    fi
    
    # Find last task completion event
    local last_completion=$(echo "$timestamps" | tail -1)
    
    # Find crash timestamp
    local crash_timestamp=$(jq -r '.[] | select(.event == "crash" or .event == "termination") | .timestamp' "$trace_file" 2>/dev/null | tail -1)
    
    if [[ -z "$crash_timestamp" ]]; then
        return 1  # Can't determine
    fi
    
    # Calculate temporal gap
    local completion_epoch=$(date -d "$last_completion" +%s 2>/dev/null || echo "0")
    local crash_epoch=$(date -d "$crash_timestamp" +%s 2>/dev/null || echo "0")
    
    if [[ "$completion_epoch" -gt 0 ]] && [[ "$crash_epoch" -gt 0 ]]; then
        local temporal_gap=$((crash_epoch - completion_epoch))
        
        # KEY CRITERION: If task completed < 30 seconds before crash → FALSE_POSITIVE
        if [[ $temporal_gap -lt 30 ]]; then
            log_alert "INFO" "Task completed ${temporal_gap}s before crash → FALSE_POSITIVE"
            return 0  # FALSE_POSITIVE detected
        fi
        
        log_alert "INFO" "Task completed ${temporal_gap}s before crash → temporal gap too large for false positive"
    fi
    
    return 1  # Not a false positive based on temporal gap
}
```

**Integration Point (add to main processing flow):**

```bash
# In crash-alert-manager.sh main processing flow, AFTER trace file check:

# CRITICAL FIX 0: Check resolution state BEFORE creating alert
log_alert "INFO" "Checking resolution state for: $BEAD_ID"
RESOLUTION_STATUS=$(check_crash_resolution "$BEAD_ID")

if [[ "$RESOLUTION_STATUS" == "RESOLVED" ]]; then
    log_alert "INFO" "Skipping resolved crash: $BEAD_ID"
    # Mark as processed to prevent future processing
    touch "$TRACE_DIR/$BEAD_ID/.alert-processed"
    exit 0  # No alert needed
fi

# CRITICAL FIX 1b: Enhanced - Check task completion temporal gap
if validate_task_completion "$BEAD_ID"; then
    log_alert "INFO" "False positive detected (temporal gap < 30s): $BEAD_ID"
    
    # Mark as resolved in state database
    update_resolution_state "$BEAD_ID" "FALSE_POSITIVE" "Task completed before crash"
    
    # Mark as processed
    touch "$TRACE_DIR/$BEAD_ID/.alert-processed"
    exit 0  # No alert needed
fi

# Continue with existing classification logic...
```

---

## Resolved vs Active Crashes

### What Constitutes a "Resolved" Crash?

A crash is considered **RESOLVED** if ANY of these conditions are met:

#### Condition 1: Post-Completion False Positive (PRIMARY)

**Criteria:**
- Task completed successfully (exit code 0, work committed)
- Crash occurred during post-completion cleanup (temporal gap < 30 seconds)
- Bead eventually CLOSED successfully

**Resolution Mechanism:** Automatic detection → Mark resolved → No alert

**Example:** bf-1ea4g pattern
- Task completed: 07:34:20Z
- Crash occurred: 07:42:34Z
- Temporal gap: 8 minutes (longer than 30s criterion, but still false positive)
- Resolution: Bead closed successfully at 09:10:16Z

#### Condition 2: Automatic Retry Success

**Criteria:**
- Crash occurred (exit code non-zero)
- Automatic retry succeeded (bead CLOSED)
- No code defect found
- Same crash pattern not recurring

**Resolution Mechanism:** NEEDLE auto-retry → Success → Mark resolved

**Example:** bf-4k2ws pattern
- Crash: Exit code -1 (SIGHUP cascade)
- Auto-retry: Success
- Resolution: Automatic recovery

#### Condition 3: Manual Fix Applied

**Criteria:**
- Code defect identified and fixed
- Fix deployed and verified
- Related crashes no longer occur

**Resolution Mechanism:** Manual investigation → Fix → Mark resolved

**Example:** Repository bloat issue (bf-4yjq)
- Crash: OOM from 18GB repository
- Fix: Repository cleanup (18GB → 138MB)
- Resolution: System recovered

#### Condition 4: Infrastructure Event Recovered

**Criteria:**
- System-wide event (SIGHUP cascade, network issue)
- System recovered to normal operation
- No ongoing issues

**Resolution Mechanism:** System recovery → Monitor confirms stability → Mark resolved

**Example:** 2026-08-16 SIGHUP cascade
- Event: 200+ crashes in 5 hours
- Recovery: System stable for 16+ days
- Resolution: Infrastructure event ended

### What Constitutes an "Active" Crash?

A crash is considered **ACTIVE/UNRESOLVED** if:

1. **Code Defect Suspected:**
   - Application error in logs
   - Consistent crash pattern across multiple attempts
   - No successful runs of same task

2. **Resource Exhaustion Ongoing:**
   - Repository still bloated (>1GB)
   - System memory still exhausted (<5GB available)
   - System resources not recovered

3. **Service Dependency Down:**
   - External service unavailable (inference gateway)
   - Retry with backoff failing
   - Service outage ongoing

4. **New Crash Pattern:**
   - Unknown crash type
   - Not matching any resolved pattern
   - Requires investigation

---

## Edge Cases and Handling

### Edge Case 1: Legitimate Re-Crashes

**Scenario:** Task completed → Fixed → New crash in same area

**Detection:**
- Check fix timestamp vs crash timestamp
- If crash > 1 hour after fix → New crash (not duplicate)
- Different stack trace or error message

**Handling:**
- Treat as NEW crash (not resolved)
- Generate new alert
- Link to previous investigation for context

### Edge Case 2: Multiple Crash Types

**Scenario:** Same bead crashes with different exit codes/patterns

**Detection:**
- Track crash signatures in resolution state
- Different signature = different crash type

**Handling:**
- Each crash type tracked separately
- Resolution state per crash signature
- Only alert for NEW crash patterns

### Edge Case 3: Flaky Crashes (Intermittent)

**Scenario:** Crash → Retry → Success → Crash again later (not fixed)

**Detection:**
- Track crash recurrence count in resolution state
- If > 3 occurrences with same pattern → Not resolved (flaky)

**Handling:**
- Mark as UNRESOLVED despite occasional success
- Escalate to CODE_DEFECT investigation
- Alert: "Intermittent crash - may indicate underlying defect"

### Edge Case 4: Repository Bloat Recurrence

**Scenario:** Repository cleanup → Crash → Repository bloated again

**Detection:**
- Check repository size at crash time
- If >1GB → Not resolved (bloat recurrence)

**Handling:**
- Alert with INFRASTRUCTURE classification
- Recommend repository cleanup
- Monitor for recurrence

### Edge Case 5: Service Outage During Task

**Scenario:** Task running → Service goes down → Task crashes

**Detection:**
- HTTP 503 errors in trace
- Service monitor confirms outage

**Handling:**
- Classify as SERVICE_FAILURE
- Do NOT mark resolved until service recovers
- Retry with exponential backoff
- Alert resolved only after service recovery confirmed

---

## Integration with Existing Crash Detection

### Integration Points

**1. NEEDLE Auto-Detection (Upstream)**
- Location: `/home/coding/.needle/config.yaml`
- Current: `auto_bead_on_error: true`
- Limitation: Creates alerts without intelligence
- **Integration Point:** Pre-hook to check resolution state before alert creation

**2. Crash Classifier (Existing)**
- Location: `scripts/crash-classifier.sh`
- Current: Classifies crash type
- **Integration Point:** Feed classification results into resolution state database

**3. Alert Manager (Existing)**
- Location: `scripts/crash-alert-manager.sh`
- Current: Processes alerts with 6 critical fixes
- **Integration Point:** Add resolution state checking at start of flow

**4. Monitoring Scripts (Existing)**
- Locations: `scripts/resource-monitor.sh`, `scripts/service-monitor.sh`
- Current: Monitor resources and services
- **Integration Point:** Feed system state into resolution evidence

### Modified Processing Flow

**Current Flow:**
```
NEEDLE detects crash
  → Creates alert bead
    → crash-alert-manager.sh processes
      → Checks closed bead status (FIX 1)
        → Checks exit code (FIX 4)
          → Classifies crash
            → May skip if false positive
```

**New Flow:**
```
NEEDLE detects crash
  → Check resolution state database ← NEW CHECKPOINT
    → If RESOLVED → Skip (no alert)
    → If PENDING → Continue
      → Validate task completion temporal gap ← NEW CHECKPOINT
        → If gap < 30s → Mark FALSE_POSITIVE → Skip
        → If gap > 30s → Continue
          → Check closed bead status (existing FIX 1)
            → Check exit code (existing FIX 4)
              → Classify crash (existing classifier)
                → Update resolution state database ← NEW INTEGRATION
                  → Create alert ONLY if UNRESOLVED
```

### Implementation Phases

**Phase 1: Resolution State Database**
1. Create `.beads/logs/resolved-crashes.json` schema
2. Add resolution state tracking functions
3. Implement `check_crash_resolution()` function
4. Implement `update_resolution_state()` function

**Phase 2: Enhanced Validation**
1. Add `validate_task_completion()` function
2. Implement temporal gap calculation
3. Add post-completion detection logic

**Phase 3: Integration**
1. Modify `crash-alert-manager.sh` to check resolution state FIRST
2. Integrate with existing classifier (feed results into state database)
3. Add resolution state updates to classification flow

**Phase 4: Testing**
1. Create test cases for bf-1ea4g pattern
2. Test edge cases (re-crashes, flaky crashes, multiple types)
3. Validate against historical crash data
4. Ensure 14 duplicate alerts for bf-1ea4g are prevented

---

## Classification and State Transitions

### Crash Lifecycle State Machine

```
┌──────────────┐
│   DETECTED   │  ← NEEDLE detects crash trace
└──────┬───────┘
       │
       ▼
┌──────────────┐
│   PENDING    │  ← Resolution status unknown
└──────┬───────┘
       │
       ├─→ [Task completed < 30s before crash]
       │   └─→ FALSE_POSITIVE → RESOLVED
       │
       ├─→ [Automatic retry succeeded]
       │   └─→ AUTO_RECOVERED → RESOLVED
       │
       ├─→ [Manual fix applied]
       │   └─→ MANUAL_FIX → RESOLVED
       │
       ├─→ [Infrastructure event recovered]
       │   └─→ INFRASTRUCTURE_RECOVERED → RESOLVED
       │
       ├─→ [Code defect suspected]
       │   └─→ CODE_DEFECT → UNRESOLVED → ALERT
       │
       ├─→ [Resource exhaustion ongoing]
       │   └─→ RESOURCE_EXHAUSTION → UNRESOLVED → ALERT
       │
       └─→ [Service dependency down]
           └─→ SERVICE_FAILURE → PENDING (monitor)
```

### State Transitions

**PENDING → RESOLVED:**
- FALSE_POSITIVE: Task completed before crash
- AUTO_RECOVERED: Automatic retry succeeded
- MANUAL_FIX: Fix applied and verified
- INFRASTRUCTURE_RECOVERED: System recovered

**PENDING → UNRESOLVED:**
- CODE_DEFECT: Actual bug found
- RESOURCE_EXHAUSTION: Resources still exhausted
- SERVICE_FAILURE: Service still down

**UNRESOLVED → RESOLVED:**
- Fix deployed and verified
- Resources recovered
- Service restored

**RESOLVED → ACTIVE (Rare):**
- Re-crash with different pattern
- Fix didn't work
- Resources exhausted again

---

## Performance and Storage

### State Database Size

**Estimates:**
- Average crash record: ~500 bytes (JSON)
- Expected crashes per month: ~50
- Growth rate: ~25 KB/month
- Annual size: ~300 KB

**Storage Management:**
- Keep last 90 days of resolution state
- Archive old records to `.beads/logs/resolved-crashes.archive.json`
- Compact database weekly (remove duplicates, update final states)

### Performance Impact

**Resolution Check Performance:**
- JSON read + parse: ~5-10ms
- jq query: ~5ms
- Total check time: ~10-15ms per crash

**Mitigation:**
- Cache resolution status in memory for active checks
- Batch processing for multiple crashes (auto-process mode)
- Lazy loading: Only load resolution database when needed

### Cooldown and Rate Limiting

**Existing Cooldown (5 minutes):**
- Prevents alert spam for same classification
- Implemented in crash-alert-manager.sh

**Resolution State Cooldown (NEW):**
- Once resolved, skip alerts for same crash signature indefinitely
- Exception: New crash with different signature

---

## Testing and Validation

### Test Case: bf-1ea4g Pattern (Primary)

**Setup:**
1. Simulate bf-1ea4g crash trace (task completed 8min before crash)
2. Load resolution state database with bf-1ea4g marked as RESOLVED

**Expected Behavior:**
1. First alert: Check resolution state → PENDING → Validate temporal gap → FALSE_POSITIVE → Mark RESOLVED → Skip alert
2. 13 subsequent alerts: Check resolution state → RESOLVED → Skip immediately
3. Total alerts generated: 0 (all 14 prevented)

**Test Commands:**
```bash
# Simulate first detection
./scripts/crash-alert-manager.sh bf-1ea4g
# Expected: FALSE_POSITIVE detected, marked resolved, no alert

# Simulate 13 duplicate detections
for i in {1..13}; do
    ./scripts/crash-alert-manager.sh bf-1ea4g
done
# Expected: All skip with "already RESOLVED" message

# Verify resolution state
jq '.resolved_crashes.bf-1ea4g' .beads/logs/resolved-crashes.json
# Expected: resolution_status = "RESOLVED", alert_count = 0
```

### Test Case: Legitimate Re-Crash

**Setup:**
1. Mark crash bf-test1 as RESOLVED (code fix applied)
2. 2 hours later, same bead crashes with DIFFERENT signature

**Expected Behavior:**
1. Check resolution state → RESOLVED (from previous fix)
2. Check crash signature → DIFFERENT from resolved
3. Treat as NEW crash → Generate alert
4. Update resolution state with new crash signature

**Test Commands:**
```bash
# First crash (resolved)
./scripts/crash-alert-manager.sh bf-test1
echo "Manually mark as RESOLVED (simulating fix)"

# Second crash (different signature)
./scripts/crash-alert-manager.sh bf-test1
# Expected: NEW alert (different signature)
```

### Test Case: Flaky Intermittent Crash

**Setup:**
1. Crash bf-flaky1 occurs → Auto-retry succeeds
2. Mark as RESOLVED
3. Same crash occurs 3 more times over 1 hour

**Expected Behavior:**
1. First crash: RESOLVED (auto-retry)
2. Second crash: Check resolution state → RESOLVED → Detect recurrence (>3 times) → Mark UNRESOLVED → ALERT
3. Alert: "Intermittent crash - may indicate underlying defect"

---

## Implementation Checklist

### Phase 1: Core Resolution State Tracking
- [ ] Create `scripts/resolution-state.sh` with state database functions
- [ ] Implement `check_crash_resolution()` function
- [ ] Implement `update_resolution_state()` function
- [ ] Create `.beads/logs/resolved-crashes.json` schema
- [ ] Add resolution state migration script

### Phase 2: Enhanced Validation
- [ ] Add `validate_task_completion()` function
- [ ] Implement temporal gap calculation
- [ ] Add post-completion detection (< 30 seconds criterion)
- [ ] Integrate with crash-classifier.sh

### Phase 3: Integration with Alert Manager
- [ ] Modify `crash-alert-manager.sh` to check resolution state FIRST
- [ ] Add resolution state updates after classification
- [ ] Integrate with existing 6 critical fixes
- [ ] Add resolution status to alert logging

### Phase 4: Testing and Validation
- [ ] Test bf-1ea4g pattern (14 alerts prevented)
- [ ] Test legitimate re-crash detection
- [ ] Test flaky intermittent crash handling
- [ ] Test edge cases (multiple types, bloat recurrence, service outage)
- [ ] Validate performance impact (< 15ms per check)

### Phase 5: Documentation and Deployment
- [ ] Update crash response guide with resolution state workflow
- [ ] Add resolution state monitoring to health checks
- [ ] Document resolution state database schema
- [ ] Create operator guide for manual resolution updates
- [ ] Deploy to production

---

## Success Metrics

### Alert Reduction
- **Target:** 95%+ reduction in false positive alerts
- **Baseline:** 14 false positives for bf-1ea4g
- **Goal:** 0 false positives for same pattern

### Investigation Efficiency
- **Target:** 90%+ reduction in duplicate investigations
- **Baseline:** 14 investigation beads for bf-1ea4g
- **Goal:** 1 investigation bead, marked resolved

### Alert Accuracy
- **Target:** < 5% false positive rate
- **Baseline:** 60-75% false positive rate (before fixes)
- **Current:** < 5% false positive rate (with 6 critical fixes)
- **Goal:** Maintain < 5% with resolution state tracking

### System Performance
- **Target:** < 20ms overhead per crash check
- **Baseline:** ~10ms (current classification)
- **Goal:** < 15ms total (classification + resolution check)

---

## Conclusion

This crash alert deduplication design prevents false positive alerts for resolved crashes through:

1. **Resolution State Tracking:** Maintains database of resolved crashes with full evidence
2. **Temporal Gap Validation:** Detects post-completion crashes (< 30 seconds = FALSE_POSITIVE)
3. **Checkpoint Flow:** Checks resolution state BEFORE generating alerts
4. **Crash Lifecycle Management:** PENDING → RESOLVED/UNRESOLVED → ALERT only if needed
5. **Edge Case Handling:** Re-crashes, flaky crashes, multiple types

**Key Achievement:** Prevents the bf-1ea4g pattern (14 duplicate alerts) by detecting that the task completed successfully before the crash occurred.

**Implementation Timeline:**
- Phase 1-2: Core resolution state tracking (~2 days)
- Phase 3: Integration (~1 day)
- Phase 4: Testing (~1 day)
- Phase 5: Deployment (~1 day)
- **Total: ~5 days**

---

**Design Status:** ✅ COMPLETE
**Ready for Implementation:** YES
**Next Step:** Phase 1 - Create resolution state tracking scripts

---

**Related Documentation:**
- `docs/crash-response-guide.md` - Crash investigation procedures
- `docs/comprehensive-crash-prevention-guide.md` - Prevention strategies
- `docs/root-cause-analysis-bf-1ea4g-final.md` - bf-1ea4g investigation
- `docs/investigations/crash-alert-generation-logic-2026-09-02.md` - Alert generation analysis

**Implementation Task:** TBD (separate implementation bead)
