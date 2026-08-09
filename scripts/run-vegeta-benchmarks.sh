#!/bin/bash
# Domain Check Performance Tests with Vegeta
# Tests all key performance scenarios with precise measurements

set -e

export PATH=$PATH:~/.local/bin

# Configuration
SERVER_ADDR="${SERVER_ADDR:-localhost:8080}"
DOMAIN="${TEST_DOMAIN:-example.com}"
RESULTS_DIR="${RESULTS_DIR:-docs/benchmarks}"
DATE=$(date +%Y-%m-%d)
REPORT="$RESULTS_DIR/vegeta-report-$DATE.md"

# Create results directory
mkdir -p "$RESULTS_DIR"

# Print header
echo "================================"
echo "Domain Check Performance Tests"
echo "================================"
echo "Server: $SERVER_ADDR"
echo "Results: $REPORT"
echo ""

# Ensure server is ready
echo "Checking server..."
curl -s "http://$SERVER_ADDR/health" | jq -e '.status == "ok"' >/dev/null
echo "Server is ready!"
echo ""

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

pass_fail() {
    local actual=$1
    local target=$2
    local unit=$3
    local name=$4

    # Convert to numbers for comparison (handle ms vs s)
    if [ "$unit" = "ms" ]; then
        actual_ms=$(echo "$actual" | awk '{printf "%.0f", $1 * 1000}')
        target_ms=$(echo "$target" | awk '{printf "%.0f", $1 * 1000}')
        if [ "$actual_ms" -lt "$target_ms" ]; then
            echo -e "${GREEN}✓ PASS${NC} - ${actual_ms}ms < ${target_ms}ms"
            return 0
        else
            echo -e "${RED}✗ FAIL${NC} - ${actual_ms}ms >= ${target_ms}ms"
            return 1
        fi
    else
        if (( $(echo "$actual < $target" | bc -l) )); then
            echo -e "${GREEN}✓ PASS${NC} - ${actual}${unit} < ${target}${unit}"
            return 0
        else
            echo -e "${RED}✗ FAIL${NC} - ${actual}${unit} >= ${target}${unit}"
            return 1
        fi
    fi
}

# ============================================================================
# TEST 1: Cached Response Test - Memory cache performance
# Target: P99 < 10ms
# ============================================================================
echo "================================"
echo "Test 1: Cached Response (target: P99 < 10ms)"
echo "================================"

echo "Warming cache with 20 requests..."
for i in $(seq 1 20); do
    curl -s -H "X-Forwarded-For: 10.1.0.1" "http://$SERVER_ADDR/api/v1/check?d=$DOMAIN" >/dev/null
done
sleep 1

echo "Running cached test (1000 requests @ 20 req/s)..."
echo "GET http://$SERVER_ADDR/api/v1/check?d=$DOMAIN" | \
    ~/.local/bin/vegeta attack -rate=20 -duration=60s \
    -header="X-Forwarded-For: 10.1.0.1" \
    -targets=/dev/stdin 2>/dev/null | \
    ~/.local/bin/vegeta report -type=text > "$RESULTS_DIR/cached.txt"

CACHED_P50=$(awk '/P50/ {print $2}' "$RESULTS_DIR/cached.txt")
CACHED_P90=$(awk '/P90/ {print $2}' "$RESULTS_DIR/cached.txt")
CACHED_P99=$(awk '/P99/ {print $2}' "$RESULTS_DIR/cached.txt")
CACHED_P99_MS=$(awk "BEGIN {printf \"%.1f\", $CACHED_P99 * 1000}")

echo ""
echo "Results:"
echo "  P50: ${CACHED_P50}s"
echo "  P90: ${CACHED_P90}s"
echo "  P99: ${CACHED_P99}s (${CACHED_P99_MS}ms)"
echo -n "  Status: "
pass_fail "$CACHED_P99" "0.010" "s" "Cached"
CACHED_STATUS=$?
echo ""

# ============================================================================
# TEST 2: Uncached Single Check - RDAP query performance
# Target: P99 < 2s
# ============================================================================
echo "================================"
echo "Test 2: Uncached Single Check (target: P99 < 2s)"
echo "================================"

# Use unique domains to avoid cache
echo "Running uncached test (50 unique domains)..."
TARGETS=$(mktemp)
for i in $(seq 1 50); do
    echo "GET http://$SERVER_ADDR/api/v1/check?d=uncached-test-$i-$RANDOM.com" >> "$TARGETS"
done

~/.local/bin/vegeta attack -rate=5 -duration=20s \
    -header="X-Forwarded-For: 10.2.$((RANDOM % 256)).$((RANDOM % 256))" \
    -targets="$TARGETS" 2>/dev/null | \
    ~/.local/bin/vegeta report -type=text > "$RESULTS_DIR/uncached.txt"

rm -f "$TARGETS"

UNCACHED_P50=$(awk '/P50/ {print $2}' "$RESULTS_DIR/uncached.txt")
UNCACHED_P90=$(awk '/P90/ {print $2}' "$RESULTS_DIR/uncached.txt")
UNCACHED_P99=$(awk '/P99/ {print $2}' "$RESULTS_DIR/uncached.txt")

echo ""
echo "Results:"
echo "  P50: ${UNCACHED_P50}s"
echo "  P90: ${UNCACHED_P90}s"
echo "  P99: ${UNCACHED_P99}s"
echo -n "  Status: "
pass_fail "$UNCACHED_P99" "2.0" "s" "Uncached"
UNCACHED_STATUS=$?
echo ""

# ============================================================================
# TEST 3: Bulk Check - Multi-domain performance
# Target: P99 < 5s
# ============================================================================
echo "================================"
echo "Test 3: Bulk Check (50 domains, target: P99 < 5s)"
echo "================================"

# Create bulk request
BULK_FILE=$(mktemp)
cat > "$BULK_FILE" << 'BULK'
POST http://localhost:8080/api/v1/bulk
@/tmp/bulk-payload.json
BULK

BULK_PAYLOAD=$(mktemp)
cat > "$BULK_PAYLOAD" << 'EOF'
{"domains": [
"bulk-test-1.com", "bulk-test-2.com", "bulk-test-3.com", "bulk-test-4.com", "bulk-test-5.com",
"bulk-test-6.com", "bulk-test-7.com", "bulk-test-8.com", "bulk-test-9.com", "bulk-test-10.com",
"bulk-test-11.com", "bulk-test-12.com", "bulk-test-13.com", "bulk-test-14.com", "bulk-test-15.com",
"bulk-test-16.com", "bulk-test-17.com", "bulk-test-18.com", "bulk-test-19.com", "bulk-test-20.com",
"bulk-test-21.com", "bulk-test-22.com", "bulk-test-23.com", "bulk-test-24.com", "bulk-test-25.com",
"bulk-test-26.com", "bulk-test-27.com", "bulk-test-28.com", "bulk-test-29.com", "bulk-test-30.com",
"bulk-test-31.com", "bulk-test-32.com", "bulk-test-33.com", "bulk-test-34.com", "bulk-test-35.com",
"bulk-test-36.com", "bulk-test-37.com", "bulk-test-38.com", "bulk-test-39.com", "bulk-test-40.com",
"bulk-test-41.com", "bulk-test-42.com", "bulk-test-43.com", "bulk-test-44.com", "bulk-test-45.com",
"bulk-test-46.com", "bulk-test-47.com", "bulk-test-48.com", "bulk-test-49.com", "bulk-test-50.com"
]}
EOF

echo "Running bulk test (10 requests of 50 domains each)..."
~/.local/bin/vegeta attack -rate=1 -duration=20s \
    -header="X-Forwarded-For: 10.3.0.1" \
    -targets="$BULK_FILE" \
    -body="$BULK_PAYLOAD" 2>/dev/null | \
    ~/.local/bin/vegeta report -type=text > "$RESULTS_DIR/bulk.txt"

rm -f "$BULK_FILE" "$BULK_PAYLOAD"

BULK_P50=$(awk '/P50/ {print $2}' "$RESULTS_DIR/bulk.txt")
BULK_P90=$(awk '/P90/ {print $2}' "$RESULTS_DIR/bulk.txt")
BULK_P99=$(awk '/P99/ {print $2}' "$RESULTS_DIR/bulk.txt")

echo ""
echo "Results:"
echo "  P50: ${BULK_P50}s"
echo "  P90: ${BULK_P90}s"
echo "  P99: ${BULK_P99}s"
echo -n "  Status: "
pass_fail "$BULK_P99" "5.0" "s" "Bulk"
BULK_STATUS=$?
echo ""

# ============================================================================
# TEST 4: Sustained Load - 100 req/s for 60 seconds
# Target: P99 < 50ms
# ============================================================================
echo "================================"
echo "Test 4: Sustained Load (100 req/s, 60s, target: P99 < 50ms)"
echo "================================"

# First warm the cache
echo "Warming cache..."
for i in $(seq 1 20); do
    curl -s "http://$SERVER_ADDR/api/v1/check?d=$DOMAIN" >/dev/null
done
sleep 1

echo "Running sustained load test..."
echo "GET http://$SERVER_ADDR/api/v1/check?d=$DOMAIN" | \
    ~/.local/bin/vegeta attack -rate=100 -duration=60s \
    -header="X-Forwarded-For: 10.4.0.1" \
    -targets=/dev/stdin 2>/dev/null | \
    ~/.local/bin/vegeta report -type=text > "$RESULTS_DIR/sustained.txt"

SUSTAINED_P50=$(awk '/P50/ {print $2}' "$RESULTS_DIR/sustained.txt")
SUSTAINED_P90=$(awk '/P90/ {print $2}' "$RESULTS_DIR/sustained.txt")
SUSTAINED_P99=$(awk '/P99/ {print $2}' "$RESULTS_DIR/sustained.txt")
SUSTAINED_P99_MS=$(awk "BEGIN {printf \"%.1f\", $SUSTAINED_P99 * 1000}")

echo ""
echo "Results:"
echo "  P50: ${SUSTAINED_P50}s"
echo "  P90: ${SUSTAINED_P90}s"
echo "  P99: ${SUSTAINED_P99}s (${SUSTAINED_P99_MS}ms)"
echo -n "  Status: "
pass_fail "$SUSTAINED_P99" "0.050" "s" "Sustained"
SUSTAINED_STATUS=$?
echo ""

# ============================================================================
# TEST 5: Error Rate Analysis
# ============================================================================
echo "================================"
echo "Test 5: Error Rate Analysis"
echo "================================"

# Get error counts from sustained test
SUCCESS=$(awk '/Success/ {print $2}' "$RESULTS_DIR/sustained.txt")
FAILURE=$(awk '/Failure/ {print $2}' "$RESULTS_DIR/sustained.txt")

echo "Sustained load error breakdown:"
echo "  Success: $SUCCESS"
echo "  Failure: $FAILURE"

# Calculate error rate
SUCCESS_COUNT=$(echo "$SUCCESS" | tr -d '%[' | sed 's/\..*//')
if [ -n "$SUCCESS_COUNT" ] && [ "$SUCCESS_COUNT" -gt 0 ]; then
    ERROR_RATE=$(echo "scale=2; 100 - $SUCCESS_COUNT" | bc)
    echo "  Error rate: ${ERROR_RATE}%"

    if (( $(echo "$ERROR_RATE < 0.1" | bc -l) )); then
        echo -e "  Status: ${GREEN}✓ PASS${NC} - Error rate < 0.1%"
        ERROR_STATUS=0
    else
        echo -e "  Status: ${RED}✗ FAIL${NC} - Error rate >= 0.1%"
        ERROR_STATUS=1
    fi
else
    echo "  Status: Could not parse success rate"
    ERROR_STATUS=1
fi
echo ""

# ============================================================================
# Generate Report
# ============================================================================
echo "================================"
echo "Generating Report"
echo "================================"

CACHED_STATUS_STR=$([ $CACHED_STATUS -eq 0 ] && echo "✓ PASS" || echo "✗ FAIL")
UNCACHED_STATUS_STR=$([ $UNCACHED_STATUS -eq 0 ] && echo "✓ PASS" || echo "✗ FAIL")
BULK_STATUS_STR=$([ $BULK_STATUS -eq 0 ] && echo "✓ PASS" || echo "✗ FAIL")
SUSTAINED_STATUS_STR=$([ $SUSTAINED_STATUS -eq 0 ] && echo "✓ PASS" || echo "✗ FAIL")
ERROR_STATUS_STR=$([ $ERROR_STATUS -eq 0 ] && echo "✓ PASS" || echo "✗ FAIL")

cat > "$REPORT" << EOF
# Domain Check Performance Report - Vegeta

**Date:** $(date -Iseconds)
**Server:** $SERVER_ADDR
**Test Domain:** $DOMAIN

---

## Executive Summary

| Test | Target | Result | Status |
|------|--------|--------|--------|
| Cached Response | < 10ms | **${CACHED_P99_MS}ms** | $CACHED_STATUS_STR |
| Uncached Single Check | < 2s | **${UNCACHED_P99}s** | $UNCACHED_STATUS_STR |
| Bulk Check (50 domains) | < 5s | **${BULK_P99}s** | $BULK_STATUS_STR |
| Sustained Load (100 req/s) | < 50ms | **${SUSTAINED_P99_MS}ms** | $SUSTAINED_STATUS_STR |
| Error Rate | < 0.1% | **${ERROR_RATE}%** | $ERROR_STATUS_STR |

**Overall Status:** $([ $CACHED_STATUS -eq 0 && $UNCACHED_STATUS -eq 0 && $BULK_STATUS -eq 0 && $SUSTAINED_STATUS -eq 0 && $ERROR_STATUS -eq 0 ] && echo "✓ All targets met" || echo "✗ Some targets not met")

---

## Test Environment

- **OS:** $(uname -s) $(uname -r)
- **Architecture:** $(uname -m)
- **CPU Cores:** $(nproc)
- **Memory:** $(free -h | grep Mem | awk '{print $2}')
- **Server PID:** $(ps aux | grep '[d]omain-check serve' | awk '{print $2}' | head -1)

---

## Detailed Results

### 1. Cached Response Test

**Target:** P99 < 10ms

\`\`\`
$(cat "$RESULTS_DIR/cached.txt")
\`\`\`

---

### 2. Uncached Single Check Test

**Target:** P99 < 2s

\`\`\`
$(cat "$RESULTS_DIR/uncached.txt")
\`\`\`

---

### 3. Bulk Check Test (50 Domains)

**Target:** P99 < 5s

\`\`\`
$(cat "$RESULTS_DIR/bulk.txt")
\`\`\`

---

### 4. Sustained Load Test (100 req/s for 60s)

**Target:** P99 < 50ms

\`\`\`
$(cat "$RESULTS_DIR/sustained.txt")
\`\`\`

---

## Conclusions

1. **Cached Performance:** ${CACHED_P99_MS}ms P99 is $([ $CACHED_STATUS -eq 0 ] && echo "well within" || echo "above") the 10ms target.

2. **RDAP Query Performance:** ${UNCACHED_P99}s P99 for uncached queries is $([ $UNCACHED_STATUS -eq 0 ] && echo "within" || echo "above") the 2s target.

3. **Bulk Operations:** ${BULK_P99}s P99 for 50-domain bulk checks is $([ $BULK_STATUS -eq 0 ] && echo "within" || echo "above") the 5s target.

4. **Sustained Throughput:** ${SUSTAINED_P99_MS}ms P99 at 100 req/s is $([ $SUSTAINED_STATUS -eq 0 ] && echo "within" || echo "above") the 50ms target.

5. **Error Rate:** ${ERROR_RATE}% error rate is $([ $ERROR_STATUS -eq 0 ] && echo "within" || echo "above") the 0.1% target.

---

*Report generated by \`scripts/run-vegeta-benchmarks.sh\` on $(date -Iseconds)*
EOF

echo ""
echo "================================"
echo "All tests completed!"
echo "================================"
echo "Report saved to: $REPORT"
echo ""
echo "Summary:"
cat "$REPORT" | grep -A 10 "Executive Summary"
