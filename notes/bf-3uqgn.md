# Metrics Recording Verification (bf-3uqgn)

## Task
Record bulk check and served metrics from API handlers.

## Verification
All acceptance criteria are already implemented in `internal/server/handlers_api.go`:

### CheckHandler (lines 139-142)
```go
if h.metrics != nil {
    h.metrics.RecordBulkCheck(1)
    h.metrics.AddChecksServed(1)
}
```

### MultiTLDHandler (lines 265-268)
```go
if h.metrics != nil {
    h.metrics.RecordBulkCheck(len(tlds))
    h.metrics.AddChecksServed(response.Succeeded)
}
```

### BulkHandler (lines 458-460, 532-534)
```go
// Record bulk check size metric
if h.metrics != nil {
    h.metrics.RecordBulkCheck(len(req.Domains))
}

// ... later in the function ...

// Record checks served metric
if h.metrics != nil {
    h.metrics.AddChecksServed(response.Succeeded)
}
```

## Acceptance Criteria
- ✅ BulkHandler calls metrics.RecordBulkCheck with domain count
- ✅ CheckHandler calls metrics.RecordBulkCheck(1) for single checks
- ✅ Both handlers call metrics.AddChecksServed(n) with number of domains checked
- ✅ go build ./... passes (verified)

## Implementation Notes
- Metrics are only recorded when `h.metrics != nil` (metrics are optional)
- ChecksServed counts successful checks (response.Succeeded), not total attempted
- All bulk check variants (single, multi-TLD, bulk) properly record metrics
