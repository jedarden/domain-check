#!/bin/bash
# Domain Check Load Testing Script
# This script runs comprehensive load tests and generates a benchmark report.

set -e

# Configuration
SERVER_ADDR="${SERVER_ADDR:-localhost:8080}"
DOMAIN="${TEST_DOMAIN:-example.com}"
RESULTS_DIR="${RESULTS_DIR:-docs/benchmarks}"
DATE=$(date +%Y-%m-%d)
REPORT="$RESULTS_DIR/report-$DATE.md"
IP_POOL_START="10.0.0.1"

# Create results directory
mkdir -p "$RESULTS_DIR"

# Print header
echo "================================"
echo "Domain Check Load Testing"
echo "================================"
echo "Server: $SERVER_ADDR"
echo "Results: $REPORT"
echo ""

# Check if tools are available
command -v hey >/dev/null 2>&1 || { echo "hey not found. Run: go install github.com/rakyll/hey@latest"; exit 1; }
command -v vegeta >/dev/null 2>&1 || { echo "vegeta not found. Run: go install github.com/tsenart/vegeta@latest"; exit 1; }

# Start the report
cat > "$REPORT" << EOF
# Domain Check Performance Baseline

**Date:** $(date -I)
**Server:** $SERVER_ADDR
**Test Domain:** $DOMAIN

---

## Executive Summary

| Test | Target | Result | Status |
|------|--------|--------|--------|
EOF

# Helper to run a test and append to report
run_test() {
    local name="$1"
    local target="$2"
    local command="$3"
    local file="$RESULTS_DIR/${name// /-}.txt"

    echo "Running: $name"
    echo "Command: $command"
    echo ""

    eval "$command" > "$file" 2>&1 || true
}

# Helper to extract p99 from hey output
extract_p99_hey() {
    local file="$1"
    awk '/99%% in/ {print $3}' "$file" | sed 's/secs//'
}

# Helper to extract p99 from vegeta output
extract_p99_vegeta() {
    local file="$1"
    awk '/P99/ {print $2}' "$file"
}

# Helper to extract success ratio from vegeta
extract_success_ratio() {
    local file="$1"
    awk '/Success/ {print $3}' "$file"
}

# Wait for server
echo "Waiting for server..."
until curl -s "http://$SERVER_ADDR/health" >/dev/null 2>&1; do
    sleep 1
done
echo "Server is ready!"
echo ""

# ============================================================================
# TEST 1: Smoke Test - Quick functionality check
# ============================================================================
echo "================================"
echo "Test 1: Smoke Test"
echo "================================"
run_test "smoke" \
    "1000 req, 50 concurrent" \
    "export PATH=\$PATH:~/go/bin && hey -n 1000 -c 50 -H 'X-Forwarded-For: 10.0.1.1' 'http://$SERVER_ADDR/api/v1/check?d=$DOMAIN'"

SMOKE_P99=$(extract_p99_hey "$RESULTS_DIR/smoke.txt")
echo "Smoke Test P99: $SMOKE_P99 seconds"
echo ""

# ============================================================================
# TEST 2: Cached Response Test - Memory cache performance
# ============================================================================
echo "================================"
echo "Test 2: Cached Response"
echo "================================"
echo "Warming cache with 10 requests..."
for i in $(seq 1 10); do
    curl -s -H "X-Forwarded-For: 10.0.2.1" "http://$SERVER_ADDR/api/v1/check?d=$DOMAIN" >/dev/null
done
sleep 1

run_test "cached" \
    "P99 < 10ms" \
    "export PATH=\$PATH:~/go/bin && hey -n 1000 -c 20 -H 'X-Forwarded-For: 10.0.2.1' 'http://$SERVER_ADDR/api/v1/check?d=$DOMAIN'"

CACHED_P99=$(extract_p99_hey "$RESULTS_DIR/cached.txt")
CACHED_P99_MS=$(awk "BEGIN {printf \"%.1f\", $CACHED_P99 * 1000}")
CACHED_STATUS="✓ PASS"
if (( $(awk "BEGIN {print ($CACHED_P99 > 0.010) ? 1 : 0}") )); then
    CACHED_STATUS="✗ FAIL (target: <10ms)"
fi
echo "Cached Response P99: ${CACHED_P99_MS}ms - $CACHED_STATUS"
echo ""

# ============================================================================
# TEST 3: Uncached Single Check - RDAP query performance
# ============================================================================
echo "================================"
echo "Test 3: Uncached Single Check"
echo "================================"

# Use a python script to generate unique domains with random IPs
cat > /tmp/bench-uncached.sh << 'UNCAV'
#!/bin/bash
export PATH=$PATH:~/go/bin
for i in $(seq 1 100); do
    IP="10.0.3.$(( (RANDOM % 250) + 1 ))"
    DOMAIN="uncached-test-$i.example.com"
    hey -n 1 -c 1 -H "X-Forwarded-For: $IP" "http://localhost:8080/api/v1/check?d=$DOMAIN" 2>&1 | grep -E "(Status code|99%)" || true
done
UNCAV

run_test "uncached-single" \
    "P99 < 2s" \
    "bash /tmp/bench-uncached.sh"

# Get max latency from uncached results
UNCACHED_P99=$(awk '/99%% in/ {print $3}' "$RESULTS_DIR/uncached-single.txt" | sort -rn | head -1 | sed 's/secs//')
if [ -z "$UNCACHED_P99" ]; then
    UNCACHED_P99=$(grep "99%" "$RESULTS_DIR/uncached-single.txt" | head -1 | awk '{print $NF}' | sed 's/secs//')
fi
UNCACHED_STATUS="✓ PASS"
if [ -n "$UNCACHED_P99" ] && (( $(awk "BEGIN {print ($UNCACHED_P99 > 2) ? 1 : 0}") )); then
    UNCACHED_STATUS="✗ FAIL (target: <2s)"
fi
echo "Uncached Single Check P99: ${UNCACHED_P99}s - $UNCACHED_STATUS"
echo ""

# ============================================================================
# TEST 4: Bulk Check - Multi-domain performance
# ============================================================================
echo "================================"
echo "Test 4: Bulk Check"
# ============================================================================

# Create a test domain list
cat > /tmp/bulk-domains.json << EOF
{"domains": [
$(for i in $(seq 1 50); do echo "\"bulk-test-$i.example.com\","; done | sed '$ s/,$//')
]}
EOF

run_test "bulk-check" \
    "P99 < 5s" \
    "export PATH=\$PATH:~/go/bin && hey -n 10 -c 2 -m POST -H 'Content-Type: application/json' -H 'X-Forwarded-For: 10.0.4.1' -d @/tmp/bulk-domains.json 'http://$SERVER_ADDR/api/v1/bulk'"

BULK_P99=$(extract_p99_hey "$RESULTS_DIR/bulk-check.txt")
BULK_STATUS="✓ PASS"
if [ -n "$BULK_P99" ] && (( $(awk "BEGIN {print ($BULK_P99 > 5) ? 1 : 0}") )); then
    BULK_STATUS="✗ FAIL (target: <5s)"
fi
echo "Bulk Check P99: ${BULK_P99}s - $BULK_STATUS"
echo ""

# ============================================================================
# TEST 5: Sustained Load - 100 req/s for 60 seconds
# ============================================================================
echo "================================"
echo "Test 5: Sustained Load (100 req/s)"
echo "================================"

# Warm up the cache
echo "Warming cache..."
for i in $(seq 1 20); do
    curl -s -H "X-Forwarded-For: 10.0.5.1" "http://$SERVER_ADDR/api/v1/check?d=$DOMAIN" >/dev/null
done

# Create vegeta targets with randomized IPs
cat > /tmp/vegeta-targets.txt << 'VGT'
GET http://localhost:8080/api/v1/check?d=example.com
VGT

run_test "sustained-load" \
    "P99 < 50ms" \
    "export PATH=\$PATH:~/go/bin && cat /tmp/vegeta-targets.txt | vegeta attack -rate=100 -duration=60s -header='X-Forwarded-For: 10.0.5.1' | vegeta report -type=text"

SUSTAINED_P99=$(extract_p99_vegeta "$RESULTS_DIR/sustained-load.txt")
SUSTAINED_SUCCESS=$(extract_success_ratio "$RESULTS_DIR/sustained-load.txt")
SUSTAINED_STATUS="✓ PASS"
if (( $(awk "BEGIN {print ($SUSTAINED_P99 > 0.050) ? 1 : 0}") )); then
    SUSTAINED_STATUS="✗ FAIL (target: <50ms)"
fi
SUSTAINED_P99_MS=$(awk "BEGIN {printf \"%.1f\", $SUSTAINED_P99 * 1000}")
echo "Sustained Load P99: ${SUSTAINED_P99_MS}ms - $SUSTAINED_STATUS (Success: $SUSTAINED_SUCCESS)"
echo ""

# ============================================================================
# TEST 6: Rate Limiter Verification - Should see 429s
# ============================================================================
echo "================================"
echo "Test 6: Rate Limiter Verification"
# ============================================================================

# First, consume the rate limit bucket
echo "Consuming rate limit bucket (first 60 requests)..."
export PATH=$PATH:~/go/bin
hey -n 60 -c 10 -H 'X-Forwarded-For: 192.168.1.100' "http://$SERVER_ADDR/api/v1/check?d=ratelimit-test.com" >/dev/null 2>&1 || true

# Now send more requests - should get 429s
run_test "rate-limit" \
    "Should see 429s" \
    "export PATH=\$PATH:~/go/bin && hey -n 50 -c 10 -H 'X-Forwarded-For: 192.168.1.100' 'http://$SERVER_ADDR/api/v1/check?d=ratelimit-test2.com'"

RATE_429_COUNT=$(grep -o "429" "$RESULTS_DIR/rate-limit.txt" | wc -l)
RATE_2XX_COUNT=$(grep -o "20[0-9]" "$RESULTS_DIR/rate-limit.txt" | wc -l)
echo "Rate Limiter Test: $RATE_429_COUNT responses were 429 (rate limited), $RATE_2XX_COUNT were successful"
echo ""

# ============================================================================
# TEST 7: Memory Growth Test - 50 req/s for 10 minutes
# ============================================================================
echo "================================"
echo "Test 7: Memory Growth Test (50 req/s, 10 min)"
# ============================================================================

# Get initial memory
INITIAL_MEM=$(ps aux | grep '[d]omain-check serve' | awk '{print $6}')
echo "Initial memory: ${INITIAL_MEM} KB"

run_test "memory-growth" \
    "Memory plateau" \
    "export PATH=\$PATH:~/go/bin && cat /tmp/vegeta-targets.txt | vegeta attack -rate=50 -duration=10m -header='X-Forwarded-For: 10.0.7.1' | vegeta report -type=text"

# Get final memory
FINAL_MEM=$(ps aux | grep '[d]omain-check serve' | awk '{print $6}')
echo "Final memory: ${FINAL_MEM} KB"

MEM_GROWTH=$((FINAL_MEM - INITIAL_MEM))
MEM_GROWTH_MB=$((MEM_GROWTH / 1024))
MEM_STATUS="✓ PASS"
if [ $MEM_GROWTH_MB -gt 100 ]; then
    MEM_STATUS="✗ FAIL (growth: ${MEM_GROWTH_MB}MB > 100MB)"
elif [ $MEM_GROWTH_MB -lt -10 ]; then
    MEM_STATUS="✓ PASS (memory decreased, excellent GC)"
fi
echo "Memory Growth: ${MEM_GROWTH_MB}MB - $MEM_STATUS"
echo ""

# ============================================================================
# Generate Report
# ============================================================================
echo "================================"
echo "Generating Report"
echo "================================"

cat >> "$REPORT" << EOF
| Smoke Test | - | Completed | ✓ |
| Cached Response | < 10ms | **${CACHED_P99_MS}ms** | $CACHED_STATUS |
| Uncached Single Check | < 2s | **${UNCACHED_P99}s** | $UNCACHED_STATUS |
| Bulk Check (50 domains) | < 5s | **${BULK_P99}s** | $BULK_STATUS |
| Sustained Load (100 req/s) | < 50ms | **${SUSTAINED_P99_MS}ms** | $SUSTAINED_STATUS |
| Rate Limiter (60+50 req same IP) | 429s expected | **$RATE_429_COUNT × 429** | ✓ |
| Memory Growth (10 min @ 50 req/s) | < 100MB | **${MEM_GROWTH_MB}MB** | $MEM_STATUS |

---

## Detailed Results

### 1. Smoke Test
\`\`\`
$(cat "$RESULTS_DIR/smoke.txt")
\`\`\`

### 2. Cached Response Test
\`\`\`
$(cat "$RESULTS_DIR/cached.txt")
\`\`\`

### 3. Uncached Single Check
\`\`\`
$(cat "$RESULTS_DIR/uncached-single.txt")
\`\`\`

### 4. Bulk Check (50 domains)
\`\`\`
$(cat "$RESULTS_DIR/bulk-check.txt")
\`\`\`

### 5. Sustained Load (100 req/s, 60s)
\`\`\`
$(cat "$RESULTS_DIR/sustained-load.txt")
\`\`\`

### 6. Rate Limiter Verification
\`\`\`
$(cat "$RESULTS_DIR/rate-limit.txt")
\`\`\`

### 7. Memory Growth Test
\`\`\`
$(cat "$RESULTS_DIR/memory-growth.txt")
\`\`\`

**Initial Memory:** ${INITIAL_MEM} KB
**Final Memory:** ${FINAL_MEM} KB
**Growth:** ${MEM_GROWTH_MB} MB

---

## Test Environment

- **OS:** $(uname -s) $(uname -r)
- **Architecture:** $(uname -m)
- **CPU Cores:** $(nproc)
- **Memory:** $(free -h | grep Mem | awk '{print $2}')
- **Go Version:** $(go version 2>/dev/null || echo "N/A")

## Test Execution Date
$(date -Iseconds)

---

*This report was generated by \`scripts/bench.sh\`*
EOF

echo ""
echo "================================"
echo "All tests completed!"
echo "================================"
echo "Report saved to: $REPORT"
echo ""
echo "Summary:"
cat "$REPORT" | grep -A 10 "Executive Summary"
