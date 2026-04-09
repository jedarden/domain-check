# Domain Check — Benchmarks

Comprehensive performance testing and benchmarking results for Domain Check, an authoritative domain availability checker powered by RDAP.

## Overview

This directory contains performance baselines, load test results, and Go microbenchmark data for the Domain Check service. All tests are designed to verify that the system meets its performance targets:

| Scenario | Target P99 | Target Error Rate |
|----------|-----------|-------------------|
| Cached responses | < 10ms | < 0.1% |
| Uncached single check | < 2s | < 1% |
| Bulk (50 domains, mixed TLDs) | < 5s | < 2% |
| Sustained 100 req/s (cached) | < 50ms | < 0.1% |

## Quick Reference

- **[Latest Report](./report-2026-04-09.md)** — Most recent comprehensive test results
- **[Baseline Summary](./baseline-summary.md)** — Performance baseline established on 2026-04-06
- **[Go Benchmarks](./go-benchmarks.md)** — Microbenchmark results for core operations
- **[Memory Testing](./memory-testing.md)** — Memory leak detection and stability tests
- **[Load Testing Scripts](../../scripts/)** — Benchmark automation scripts

## Test Categories

### 1. HTTP Load Testing (Integration)

External tools (`hey`, `vegeta`) test the full HTTP stack:

- **Smoke Test** — Quick functionality verification (1000 req @ 50 concurrent)
- **Cached Response** — In-memory cache performance (P99 target: < 10ms)
- **Uncached Single Check** — Real RDAP query performance (P99 target: < 2s)
- **Bulk Check** — Multi-domain endpoint with 50 domains (P99 target: < 5s)
- **Sustained Load** — 100 req/s for 60 seconds (P99 target: < 50ms)
- **Rate Limiter** — Verifies per-IP rate limiting enforcement
- **Memory Growth** — 10 minutes @ 50 req/s to detect leaks (target: < 100MB growth)

### 2. Go Microbenchmarks

Go `testing.B` benchmarks for individual components:

- `BenchmarkCheckBulk_10Domains` — Bulk check performance for 10 domains
- `BenchmarkCheckBulk_50Domains` — Bulk check performance for 50 domains
- `BenchmarkDNSPreFilter_Check` — DNS pre-filter optimization

### 3. Memory and Leak Testing

Long-running tests to verify stability:

- **Memory Growth Test** — Monitors heap allocation over 10 minutes of sustained load
- **IP Limiter Cleanup** — Verifies stale IP limiter entries are evicted
- **Goroutine Leak Detection** — Ensures goroutines don't accumulate

## Running Benchmarks

### Prerequisites

```bash
# Install load testing tools
go install github.com/rakyll/hey@latest
go install github.com/tsenart/vegeta@latest

# Build the server
go build -o domain-check ./cmd/domain-check/
```

### Quick Test

```bash
# Start server in background
./domain-check serve --addr localhost:8080 --trust-proxy &
SERVER_PID=$!

# Run smoke test
hey -n 1000 -c 50 "http://localhost:8080/api/v1/check?d=example.com"

# Cleanup
kill $SERVER_PID
```

### Full Suite

```bash
# Start server
./domain-check serve --addr localhost:8080 --trust-proxy &
SERVER_PID=$!

# Run comprehensive benchmarks
./scripts/run-benchmarks.sh

# Or use vegeta for precise measurements
./scripts/run-vegeta-benchmarks.sh

# Cleanup
kill $SERVER_PID
```

### Go Benchmarks Only

```bash
# Run all Go benchmarks
go test -bench=. -benchmem ./internal/checker/

# Run specific benchmark
go test -bench=BenchmarkCheckBulk_10Domains -benchmem ./internal/checker/

# Run memory growth test (2 minutes)
go test -v -run TestMemoryGrowthUnderLoad -timeout 5m ./internal/server/

# Run full 10-minute memory test
go test -v -run TestMemoryGrowthUnderLoadFull -timeout 15m ./internal/server/
```

## Performance Targets Status

| Target | Status | Result |
|--------|--------|--------|
| Cached P99 < 10ms | ✓ PASS | 5.4ms |
| Uncached P99 < 2s | ✓ PASS | ~200ms (varies by registry) |
| Bulk 50 domains P99 < 5s | ✓ PASS | ~3.5s |
| Sustained 100 req/s P99 < 50ms | ✓ PASS | 1.69ms |
| Error rate < 0.1% | ✓ PASS | ~0% |
| Memory growth < 100MB | ✓ PASS | -68KB (decreased) |

**All targets met as of 2026-04-09.**

## Historical Results

| Date | Commit | Notes |
|------|--------|-------|
| 2026-04-09 | Current | Comprehensive benchmark suite execution |
| 2026-04-06 | 20f84a2 | Baseline established, tooling verified |
| 2026-04-05 | Initial | Load testing scripts created |

## Test Environment

**Hardware:** Hetzner EX44 (dedicated server)
- **CPU:** Intel Core i5-13500 (20 cores)
- **Memory:** 62 GB RAM
- **OS:** Linux 6.12.63+deb13-amd64
- **Go:** 1.26.1 linux/amd64

**Software:**
- **hey:** github.com/rakyll/hey (HTTP load generator)
- **vegeta:** github.com/tsenart/vegeta (HTTP load testing toolkit)
- **domain-check:** Built from `cmd/domain-check/main.go`

## Contributing

When adding new features or optimizations:

1. Run the full benchmark suite before and after changes
2. Update this directory with new results
3. Document any regressions or improvements
4. Include benchmark results in pull requests

## License

MIT License — See [../../LICENSE](../../LICENSE) for details.
