# Bead bf-4dcin: Record RDAP Request Metrics

## Task Verification

The requested metrics recording functionality is **already fully implemented** in the codebase.

## Implementation Location

### `internal/checker/rdap.go` (lines 105-135)

The `Check` method in `RDAPClient` already includes comprehensive metrics recording:

```go
// Record RDAP request metrics if metrics is available
if c.metrics != nil {
    duration := time.Since(start).Seconds()
    status := "success"
    if rdapErr != nil {
        switch {
        case errors.Is(rdapErr, context.DeadlineExceeded):
            status = "timeout"
        case errors.Is(rdapErr, context.Canceled):
            status = "canceled"
        case errors.Is(rdapErr, ratelimit.ErrServiceBusy) || strings.Contains(rdapErr.Error(), "429"):
            status = "rate_limited"
        default:
            status = "error"
        }
    } else if resp != nil {
        switch resp.StatusCode {
        case http.StatusOK:
            status = "success"
        case http.StatusNotFound:
            status = "not_found"
        case http.StatusTooManyRequests:
            status = "rate_limited"
        case http.StatusBadRequest:
            status = "bad_request"
        default:
            status = fmt.Sprintf("http_%d", resp.StatusCode)
        }
    }
    c.metrics.RecordRDAPRequest(registry, status, duration)
}
```

### Acceptance Criteria Status

✅ **All criteria met:**

1. ✅ `metrics.RecordRDAPRequest` called with registry ID, HTTP status, and duration for every RDAP request
2. ✅ Covers all required cases:
   - Success: HTTP 200
   - Error: Connection failures, registry errors
   - Timeout: Context deadline exceeded
   - Additional: Canceled, rate_limited, not_found, bad_request, and other HTTP codes
3. ✅ `go build ./...` passes

### Implementation History

This functionality was added in commit `3b1815e` (bead bf-3ma3) as part of wiring ServiceMonitor to API handlers.

### Metrics Interface

The `RDAPMetrics` interface is defined in `internal/checker/rdap.go`:

```go
type RDAPMetrics interface {
    RecordRDAPRequest(registry, status string, durationSeconds float64)
}
```

And implemented in `internal/server/metrics.go`:

```go
func (m *Metrics) RecordRDAPRequest(registry, status string, durationSeconds float64) {
    m.rdapRequests.WithLabelValues(registry, status).Inc()
    m.rdapDuration.WithLabelValues(registry).Observe(durationSeconds)
}
```

## Conclusion

No implementation work was required. The RDAP request metrics recording is fully functional and already integrated into the RDAP client.
