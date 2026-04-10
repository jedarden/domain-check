# Domain Check — Vegeta Performance Test Results

## Test Summary

Date: 2026-04-10  
Target: localhost:8080

### Results

| Scenario | p99 Latency | Target | Status |
|----------|-------------|--------|--------|
| **Cached responses** | 4.1ms | <10ms | ✅ PASS |
| **Uncached single check** | 100.1ms | <2s | ✅ PASS |
| **Bulk (50 domains)** | 7.38s | <5s | ❌ FAIL (47% over target) |
| **Sustained 1 req/s** | 4.6ms | <50ms | ✅ PASS |

### Detailed Results

#### Test 1: Cached Responses
- **Requests:** 60 total @ 1/s
- **Success Rate:** 100%
- **Latencies:** min=354µs, mean=555µs, p50=464µs, p90=569µs, p95=630µs, p99=4.1ms
- **Status:** ✅ PASS (p99 4.1ms < 10ms target)

#### Test 2: Uncached Single Check
- **Requests:** 20 total @ 1/s (unique domains)
- **Success Rate:** 100%
- **Latencies:** min=74ms, mean=84ms, p50=84ms, p90=92ms, p95=96ms, p99=100ms
- **Status:** ✅ PASS (p99 100ms < 2s target)

#### Test 3: Bulk Check (50 domains per request)
- **Requests:** 10 total @ 1/s
- **Success Rate:** 100%
- **Latencies:** min=996µs, mean=3.14s, p50=2.04s, p90=7.34s, p95=7.38s, p99=7.38s
- **Status:** ❌ FAIL (p99 7.38s > 5s target)
- **Note:** Bulk requests take longer due to per-registry rate limiting and sequential processing

#### Test 4: Sustained Load (1 req/s)
- **Requests:** 60 total @ 1/s for 60 seconds
- **Success Rate:** 100%
- **Latencies:** min=417µs, mean=692µs, p50=535µs, p90=646µs, p95=1.06ms, p99=4.6ms
- **Status:** ✅ PASS (p99 4.6ms < 50ms target)

### Notes

1. **Rate limiting:** Tests were run at 1 req/s to stay within the default 60 req/min per-IP rate limit
2. **Bulk performance:** The bulk endpoint is slower than target due to:
   - Per-registry rate limiting (Verisign: 10/s, others: 1-2/s)
   - Sequential processing of domains with same registry
   - RDAP query latency (100-200ms per registry)
3. **Cached performance:** Excellent - sub-millisecond to low-millisecond latencies
4. **Uncached performance:** Good - single digit digit latencies well under 2s target

### Recommendations

1. **Bulk optimization:** Consider increasing per-registry rate limits or implementing better parallelization
2. **Sustained load:** Current implementation handles sustained load well within rate limits
3. **Cache effectiveness:** Cache provides excellent performance improvement (4ms vs 100ms)

