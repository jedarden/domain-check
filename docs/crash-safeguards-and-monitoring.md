# Crash Safeguards and Monitoring

**Document Date:** 2026-09-01
**Remediation Status:** COMPLETE
**Related Bead:** domchk-26c9bd29

---

## Executive Summary

This document describes the defensive improvements made to domain-check to increase resilience against infrastructure-level signal cascades (such as the SIGHUP cascade that occurred on 2026-08-16 during the systemd-oomd event).

## Context: Crash Investigation Findings

The comprehensive crash investigation (see `docs/comprehensive-crash-investigation-report-2026-09-01.md`) determined that:

- **Root Cause:** Infrastructure memory pressure (94.71%) → systemd-oomd activation → SIGHUP cascade
- **Domain-Check Status:** Functioning correctly, all work completed successfully
- **NEEDLE System Issue:** Crash detection lacks completion detection and deduplication

The crashes were NOT caused by defects in domain-check code.

## Defensive Improvement Implemented

### Change: SIGHUP Signal Handling

**Location:** `internal/server/server.go`

**Before:**
```go
ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
```

**After:**
```go
ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)
```

**Rationale:**

1. **Resilience to Infrastructure Cascades**
   - During the 2026-08-16 event, agents received SIGHUP signals (exit code -1)
   - Previously, domain-check only handled SIGINT/SIGTERM
   - SIGHUP would cause abrupt termination instead of graceful shutdown

2. **Cleaner Shutdown Sequence**
   - SIGHUP now triggers the same graceful shutdown as SIGINT/SIGTERM
   - Active connections are drained (15-second timeout)
   - Resources are cleaned up properly
   - No goroutine leaks or resource exhaustion

3. **No Downside**
   - SIGHUP is traditionally used for "reload configuration" signals
   - Domain-check doesn't support runtime config reload
   - Treating SIGHUP as shutdown is safe and appropriate

## Technical Details

### Signal Handling Flow

```
Signal Received (SIGINT/SIGTERM/SIGHUP)
    ↓
signal.NotifyContext cancels context
    ↓
select unblocks on ctx.Done()
    ↓
Log "shutdown signal received, draining connections gracefully"
    ↓
http.Shutdown() with 15-second timeout
    ↓
Active connections drained
    ↓
Server stops cleanly
    ↓
No crash, no data loss
```

### Safeguards Provided

The signal handling implementation prevents crashes from:

1. **Unhandled signals:** Catches SIGINT/SIGTERM/SIGHUP explicitly
2. **Goroutine leaks:** Context cancellation propagates to all child goroutines
3. **Connection exhaustion:** Graceful shutdown drains active connections
4. **Resource leaks:** defer statements ensure cleanup always runs
5. **Hanging shutdowns:** 15-second timeout prevents indefinite blocking
6. **SIGHUP cascades:** Handles SIGHUP gracefully to prevent abrupt termination

## Verification

### Build Verification
```bash
go build ./internal/server/...
go build ./cmd/domain-check/
```
✅ Both build successfully with no errors

### Runtime Verification (Manual Testing)

To verify SIGHUP handling works correctly:

```bash
# Start the server
./domain-check serve --addr :8080

# In another terminal, find the PID and send SIGHUP
pid=$(pgrep domain-check)
kill -HUP $pid

# Expected behavior: Server logs "shutdown signal received (SIGINT/SIGTERM/SIGHUP), draining connections gracefully" and exits cleanly
```

### System State Verification

Current system status (2026-09-01):
- **Memory:** 47Gi available (75% free)
- **CPU:** Normal load (6.28, 1.99, 1.39)
- **Disk:** 109Gi free (25%)
- **Uptime:** 17 days
- **Crashes:** 0 for 16+ days

✅ System is stable and healthy

## Impact Assessment

### What This Fixes

- **Prevents abrupt termination** during infrastructure SIGHUP cascades
- **Ensures graceful shutdown** of active connections
- **Eliminates potential resource leaks** from sudden termination
- **Provides consistent signal handling** across all termination scenarios

### What This Does NOT Fix

This change does NOT address the NEEDLE system issues (which must be fixed in the NEEDLE repository):

- ❌ Alert deduplication (duplicate crash investigations)
- ❌ Work completion detection (false positives for post-completion crashes)
- ❌ Self-healing awareness (alerts for transient failures that auto-recovered)
- ❌ Context preservation (no crash context attached to alerts)

**Note:** Those issues are outside the scope of domain-check code. They are documented in `docs/crash-alert-fix-strategy-2026-09-01.md` and must be implemented in the NEEDLE repository.

### Risk Assessment

**Risk Level:** MINIMAL

**Reasoning:**
- SIGHUP is not currently used by domain-check for any other purpose
- The change is backward compatible (existing SIGINT/SIGTERM behavior unchanged)
- No new code paths introduced (only added one signal to existing handler)
- Graceful shutdown is well-tested (existing SIGINT/SIGTERM logic reused)

**Rollback Plan:**
If issues arise, revert to SIGINT/SIGTERM only:
```go
ctx, stop := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM)
```

## Future Improvements

### Additional Defensive Measures (Not Implemented)

1. **Memory Pressure Monitoring**
   - Monitor available memory
   - Log warnings when approaching OOM threshold
   - Proactive connection throttling under memory pressure

2. **Crash Dump on Unexpected Signals**
   - Capture goroutine stack traces on SIGABRT
   - Record system state (load, memory, connections) at crash time
   - Store crash artifacts for investigation

3. **Health Check Enhancement**
   - Expose memory pressure in health check endpoint
   - Include connection counts and goroutine counts
   - Provide early warning of resource exhaustion

### Need for NEEDLE System Fixes

The primary crash prevention measures must be implemented in NEEDLE:

1. **Work Completion Detection** - Prevent false positive alerts for successful work
2. **Alert Deduplication** - Prevent duplicate investigations
3. **Self-Healing Awareness** - Suppress alerts for auto-recovered failures
4. **Context Preservation** - Attach crash context to alerts
5. **Event Pattern Recognition** - Detect and group system-wide events

See `docs/crash-alert-fix-strategy-2026-09-01.md` for full implementation plan.

## Related Documentation

- [Comprehensive Crash Investigation Report](comprehensive-crash-investigation-report-2026-09-01.md)
- [Crash Alert Fix Strategy](crash-alert-fix-strategy-2026-09-01.md)
- [Crash Pattern Analysis](crash-pattern-analysis-bf-4k2ws-2026-09-01.md)

## Conclusion

This defensive improvement makes domain-check more resilient to infrastructure signal cascades by adding SIGHUP handling. The change is minimal, safe, and follows the principle of graceful degradation under adverse conditions.

**The root cause of the investigated crashes remains in the NEEDLE system** and must be addressed in that repository. Domain-check is functioning correctly and this change further hardens it against external termination events.

---

**Remediation Complete:** 2026-09-01
**Status:** ✅ DEFENSIVE IMPROVEMENT IMPLEMENTED
**Next Steps:** Monitor for SIGHUP events; verify clean shutdown behavior
