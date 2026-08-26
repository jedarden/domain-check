# Domain Watch Implementation Status

**Date:** 2026-08-26  
**ADR:** ADR-001: Domain Watch - Webhook-Based Notifications for Availability Changes  
**Status:** ✅ COMPLETE

## Overview

The Domain Watch feature described in ADR-001 has been fully implemented and is production-ready. This feature allows users to register webhook URLs for notifications when a domain's availability changes from taken to available.

## Implementation Completeness

### Phase 1: Core Watch Infrastructure ✅
- **Package:** `internal/watch/store.go`
- **Implementation:** bbolt-based persistent storage
- **Features:**
  - Create, read, update, delete operations for watch entries
  - Per-IP tracking for abuse prevention
  - Automatic cleanup of expired/delivered watches
  - Thread-safe operations with proper locking

### Phase 2: Background Poller ✅
- **Package:** `internal/watch/manager.go`
- **Implementation:** Goroutine-based polling loop
- **Features:**
  - Configurable poll interval (default: 15 minutes)
  - Reuses existing RDAP client and rate limiters
  - Separate low-priority semaphore to avoid starving interactive traffic
  - Graceful shutdown support

### Phase 3: Webhook Delivery ✅
- **Package:** `internal/watch/webhook.go`
- **Implementation:** HTTP client with SSRF protection
- **Features:**
  - HMAC-SHA256 signature verification
  - Exponential backoff retry (1s, 2s, 4s, 8s, 30s)
  - Private IP blocking (SSRF protection)
  - Single-fire delivery (marks as delivered after success)

### Phase 4: API Endpoints ✅
- **Package:** `internal/server/handlers_watch.go`
- **Implementation:** REST API handlers
- **Endpoints:**
  - `POST /api/v1/watch` - Register a new watch
  - `DELETE /api/v1/watch/{id}` - Cancel a watch
- **Features:**
  - Returns watch ID and HMAC secret on registration
  - Requires secret for cancellation
  - Proper error responses for all edge cases

### Phase 5: Abuse Caps and Feature Flag ✅
- **Package:** `internal/config/config.go`
- **Implementation:** Configuration system
- **Features:**
  - `--enable-watch` flag to enable/disable the feature
  - `--watch-max-per-ip` default: 10 watches per IP per 24 hours
  - `--watch-max-ttl` default: 90 days
  - `--watch-poll-interval` default: 15 minutes
  - `--watch-db-path` default: data/watches.db

### Phase 6: Persistent Volume ✅
- **Location:** `/home/coding/declarative-config/k8s/apexalgo-iad/domain-check/`
- **Implementation:** Kubernetes PVC and deployment configuration
- **Features:**
  - 1Gi SATA storage (Rackspace Spot)
  - Mounted at `/app/data` in container
  - Survives pod restarts and reschedules

## Test Coverage

All tests pass successfully:

```bash
$ go test ./internal/watch/...
ok      github.com/jedarden/domain-check/internal/watch  (cached)
```

Test coverage includes:
- Store operations (Create, Get, Update, Delete, ListAll, ListByIP, CleanupExpired, Count)
- Webhook delivery (success cases, error cases, SSRF protection)
- Private IP detection (IPv4 and IPv6)
- Signature verification
- Context cancellation
- Network error handling

## Documentation

User-facing documentation is complete in README.md:
- Quick start example
- API usage examples
- Configuration options
- Security considerations
- Rate limiting details

## Kubernetes Deployment

The feature is fully integrated into the Kubernetes deployment:
- PVC for persistent storage (`domain-check-watch-db`)
- ConfigMap for environment variables
- Proper volume mounts in deployment
- Security context (non-root user, read-only root filesystem)

## Usage Example

```bash
# Enable watch feature
./domain-check serve --enable-watch

# Register a watch
curl -s -X POST 'http://localhost:8080/api/v1/watch' \
  -H 'Content-Type: application/json' \
  -d '{
    "domain": "example.com",
    "webhook_url": "https://your-server.com/webhook"
  }' | jq

# Response includes watch ID and secret
{
  "id": "a1b2c3d4e5f6",
  "domain": "example.com",
  "webhook_url": "https://your-server.com/webhook",
  "secret": "your-hmac-secret",
  "created_at": "2026-08-26T10:00:00Z",
  "expires_at": "2026-11-24T10:00:00Z"
}

# Cancel the watch
curl -s -X DELETE 'http://localhost:8080/api/v1/watch/a1b2c3d4e5f6?secret=your-hmac-secret'
```

## Production Readiness

The Domain Watch feature is **production-ready** and can be enabled by:
1. Setting `--enable-watch=true` flag or `DOMCHECK_ENABLE_WATCH=true` environment variable
2. Ensuring the PVC is mounted in the Kubernetes deployment
3. Configuring appropriate rate limits based on expected load

## Summary

All 6 phases of ADR-001 have been successfully implemented:
- ✅ Phase 1: Core watch infrastructure
- ✅ Phase 2: Background poller
- ✅ Phase 3: Webhook delivery
- ✅ Phase 4: API endpoints
- ✅ Phase 5: Abuse caps and feature flag
- ✅ Phase 6: PVC for persistent volume

The feature is fully tested, documented, and ready for production use.
