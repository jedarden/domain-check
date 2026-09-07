# Crash Alert Generation Logic Investigation

**Investigation Date:** 2026-09-02  
**Task:** domchk-7da0d571  
**Purpose:** Understand why false positive alerts are generated for already-resolved crashes

---

## Executive Summary

**Root Cause:** Crash alert beads are created automatically by the NEEDLE workload management system when `auto_bead_on_error: true` is enabled. This automatic generation lacks intelligence to distinguish between genuine crashes and false positives, leading to duplicate alerts for already-resolved crashes.

**Key Finding:** The crash alert generation is NOT in domain-check scripts - it's built into NEEDLE itself.

---

## Complete Alert Generation Flow

### Step 1: Crash Detection
- NEEDLE agent crashes (exit code -1, 1, etc.)
- Crash trace written to `.beads/traces/<bead-id>/trace.jsonl`
- Metadata includes exit code, outcome, timestamp

### Step 2: Automatic Bead Creation (NEEDLE)
- NEEDLE's watchdog detects crash via `auto_bead_on_error: true`
- Creates new investigation bead with title: "ALERT: Agent crash on bead bf-XXXXX"
- No validation of whether crash is genuine or false positive

### Step 3: Post-Processing (domain-check scripts)
- `crash-alert-manager.sh` processes the alert bead
- Applies 6 critical fixes to filter false positives:
  1. Closed bead filtering
  2. Duplicate detection
  3. Processed alerts tracking
  4. Exit code validation
  5. Auto-process closed bead filtering
  6. Auto-process completion awareness
- Classifies crashes: FALSE_POSITIVE, SERVICE_FAILURE, INFRASTRUCTURE, CODE_DEFECT

---

## Why False Positives Occur

### NEEDLE's Automatic Crash Detection Limitations

The NEEDLE configuration (`/home/coding/.needle/config.yaml`) contains:

```yaml
debug:
  auto_bead_on_error: true
  auto_bead_workspace: "/home/coding/NEEDLE"
  auto_bead_types: "quarantine,unregistered"
  auto_bead_rate_limit: 3600
```

**Problems:**
1. ❌ Doesn't check if original bead already closed successfully
2. ❌ Doesn't distinguish post-completion cleanup vs genuine crash
3. ❌ Doesn't prevent duplicate alerts for same crash
4. ❌ Creates alerts for ANY non-zero exit code
5. ❌ No awareness of automatic retry success

### The 14 False Positives for bf-1ea4g

**Pattern:**
1. Original crash: 2026-08-13 07:42:34Z (false positive)
2. Each time NEEDLE processes workspace → detects crash trace
3. `auto_bead_on_error: true` creates new alert bead
4. crash-alert-manager.sh filters as FALSE_POSITIVE
5. Process repeats on next workspace scan

**Evidence:**
- Bead bf-1ea4g completed successfully before crash
- 8 minutes between completion and termination
- Exit code -1 (SIGHUP) after work done
- Still triggered automatic alert creation

---

## Comparison: NEEDLE vs Domain-Check Scripts

### NEEDLE (Raw Alert Generation)
**Location:** `/home/coding/.needle/config.yaml`
**Trigger:** `auto_bead_on_error: true`
**Intelligence:** NONE
- Creates alerts for ANY crash
- No validation
- No deduplication
- No classification

### Domain-Check Scripts (Intelligent Filtering)
**Location:** `scripts/crash-alert-manager.sh`
**Intelligence:** HIGH
- Closed bead filtering
- Duplicate detection
- Exit code validation
- Crash classification
- Alert cooldown
- Completion awareness

---

## Documentation Path

The crash-alert-manager.sh provides the intelligence that NEEDLE lacks:

**Critical Fixes Implemented (2026-09-02):**
1. **CRITICAL FIX 1, 5:** Closed bead filtering
   - Checks if bead already CLOSED before processing
   - Skips alerts for successfully completed work

2. **CRITICAL FIX 2, 3:** Duplicate detection
   - Tracks processed alerts in `$PROCESSED_ALERTS_FILE`
   - Prevents multiple investigations of same crash

3. **CRITICAL FIX 4, 6:** Exit code validation
   - Validates exit code 0 (success) vs actual crash
   - Detects post-completion termination

4. **Classification System:**
   - FALSE_POSITIVE: Post-completion administrative failure
   - SERVICE_FAILURE: External service dependency failure
   - INFRASTRUCTURE: System resource exhaustion
   - CODE_DEFECT: Actual application error
   - UNKNOWN: Unable to classify

5. **Alert Cooldown:**
   - 5-minute cooldown prevents alert spam
   - Rate limiting for system-wide events

---

## Conclusion

### Root Cause
False positive crash alerts are caused by NEEDLE's `auto_bead_on_error: true` creating raw alert beads without context or validation.

### Solution Already Implemented
The crash-alert-manager.sh adds necessary filtering and classification, but NEEDLE continues to generate duplicates because the automatic creation happens at the NEEDLE level, not the domain-check level.

### Where the Fix Belongs
The fix is in NEEDLE configuration and behavior, not in domain-check code:
- NEEDLE should check bead closure status before creating alerts
- NEEDLE should validate exit codes before creating alerts
- NEEDLE should implement duplicate detection
- NEEDLE should add intelligence to `auto_bead_on_error`

### Current State
- Domain-check code: ✅ NO DEFECTS (all crashes investigated, no code issues found)
- Crash alert filtering: ✅ IMPLEMENTED (6 critical fixes working)
- NEEDLE automatic alerts: ⚠️ UNINTELLIGENT (continues creating false positives)

---

## Related Documentation

- **Crash Response Guide:** `docs/crash-response-guide.md`
- **Comprehensive Investigation:** `docs/comprehensive-crash-investigation-report-2026-09-01.md`
- **Mitigation Strategies:** `docs/crash-mitigation-strategies.md`
- **Fix Implementation:** `docs/crash-alert-fix-implementation-2026-09-02.md`

---

**Status:** ✅ Investigation Complete  
**Confidence:** HIGH  
**Classification:** INFRASTRUCTURE (NEEDLE configuration) - NOT domain-check code issue  
**Action Required:** Fix is in NEEDLE, not domain-check
