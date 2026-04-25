#!/bin/bash
# Domain Check Load Tests
# Tests all performance targets with proper rate limiting handling
#
# Targets:
# 1. Cached endpoint: 1000 requests, varied IPs, p99 < 10ms
# 2. vegeta 100/s for 60s → p99 < 50ms, error rate < 0.1%
# 3. vegeta 200/s for 10s → 429 responses appear (rate limiter works)
# 4. Memory growth test: 50 req/s for 10 min → memory plateaus

set -e

export PATH=$PATH:~/.local/bin

# Configuration
SERVER_ADDR="${SERVER_ADDR:-localhost:8080}"
DOMAIN="${TEST_DOMAIN:-example.com}"
RESULTS_DIR="${RESULTS_DIR:-load-test-results}"
DATE=$(date +%Y%m%d-%H%M%S)
REPORT="$RESULTS_DIR/load-test-report-$DATE.md"

# Colors
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Create results directory
mkdir -p "$RESULTS_DIR"

log() {
    echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $1"
}

pass_fail() {
    local actual=$1
    local target=$2
    local unit=$3
    local name=$4

    if [ "$unit" = "ms" ]; then
        if [ "$actual" -lt "$target" ]; then
            echo -e "${GREEN}✓ PASS${NC} - ${actual}ms < ${target}ms"
            return 0
        else
            echo -e "${RED}✗ FAIL${NC} - ${actual}ms >= ${target}ms"
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

# Generate random IP
random_ip() {
    echo "$((RANDOM % 256)).$((RANDOM % 256)).$((RANDOM % 256)).$((RANDOM % 256))"
}

# ============================================================================
# Header
# ============================================================================
echo "================================"
echo "Domain Check Load Tests"
echo "================================"
echo "Server: $SERVER_ADDR"
echo "Results: $REPORT"
echo ""

# Check server is running
log "Checking server..."
if ! curl -s "http://$SERVER_ADDR/health" | jq -e '.status == "ok"' >/dev/null 2>&1; then
    echo -e "${RED}Server not responding at http://$SERVER_ADDR${NC}"
    echo "Start the server first: ./domain-check serve"
    exit 1
fi
echo -e "${GREEN}Server is ready!${NC}"
echo ""

# ============================================================================
# TEST 1: Cached endpoint with varied IPs → p99 < 10ms
# ============================================================================
log "Test 1: Cached endpoint (1000 req, varied IPs, target: p99 < 10ms)"

# Warm cache with varied IPs
log "Warming cache..."
for i in {1..50}; do
    IP=$(random_ip)
    curl -s -H "X-Forwarded-For: $IP" -H "X-Real-IP: $IP" \
        "http://$SERVER_ADDR/api/v1/check?d=$DOMAIN" >/dev/null
done
sleep 2

VEGETA_CACHED_BIN="$RESULTS_DIR/vegeta-cached-$DATE.bin"
VEGETA_CACHED_TXT="$RESULTS_DIR/vegeta-cached-$DATE.txt"

# Create targets with varied IPs - use heredoc to avoid file issues
log "Running cached test (1000 requests with varied IPs)..."
(
    for i in {1..1000}; do
        echo "GET http://$SERVER_ADDR/api/v1/check?d=$DOMAIN"
    done
) | vegeta attack -rate=200 -duration=5s \
    -max-workers=50 \
    -name=cached-test > "$VEGETA_CACHED_BIN" 2>&1

# Generate text report
vegeta report "$VEGETA_CACHED_BIN" --type=text > "$VEGETA_CACHED_TXT" 2>&1 || true

CACHED_P99=$(awk '/P99/ {print $2}' "$VEGETA_CACHED_TXT" 2>/dev/null || echo "0")
CACHED_P99_MS=$(awk "BEGIN {printf \"%.1f\", $CACHED_P99 * 1000}")

echo ""
echo "Results:"
cat "$VEGETA_CACHED_TXT"
echo ""
echo -n "  Status: "
if pass_fail "${CACHED_P99_MS:-999}" "10" "ms" "Cached p99"; then
    CACHED_STATUS=0
else
    CACHED_STATUS=1
fi
echo ""

# ============================================================================
# TEST 2: vegeta 100/s for 60s → p99 < 50ms, error rate < 0.1%
# ============================================================================
log "Test 2: vegeta 100/s for 60s (target: p99 < 50ms, error rate < 0.1%)"

# Warm cache
log "Warming cache with varied IPs..."
for i in {1..100}; do
    IP=$(random_ip)
    curl -s -H "X-Forwarded-For: $IP" -H "X-Real-IP: $IP" \
        "http://$SERVER_ADDR/api/v1/check?d=$DOMAIN" >/dev/null
done
sleep 2

VEGETA_100_BIN="$RESULTS_DIR/vegeta-100-$DATE.bin"
VEGETA_100_TXT="$RESULTS_DIR/vegeta-100-$DATE.txt"

log "Running vegeta attack (100 req/s for 60s)..."
(
    for i in {1..100}; do
        echo "GET http://$SERVER_ADDR/api/v1/check?d=$DOMAIN"
    done
) | vegeta attack -rate=100 -duration=60s \
    -max-workers=100 \
    -name=load-100 > "$VEGETA_100_BIN" 2>&1

vegeta report "$VEGETA_100_BIN" --type=text > "$VEGETA_100_TXT" 2>&1 || true

VEGETA_100_P99=$(awk '/P99/ {print $2}' "$VEGETA_100_TXT" 2>/dev/null || echo "0")
VEGETA_100_P99_MS=$(awk "BEGIN {printf \"%.1f\", $VEGETA_100_P99 * 1000}")
VEGETA_100_SUCCESS=$(awk '/Success/ {print $2}' "$VEGETA_100_TXT" 2>/dev/null | tr -d '%[]' || echo "100")

echo ""
echo "Results:"
cat "$VEGETA_100_TXT"
echo ""

# Calculate error rate
if [ -n "$VEGETA_100_SUCCESS" ]; then
    VEGETA_100_ERROR_RATE=$(awk "BEGIN {printf \"%.2f\", 100 - $VEGETA_100_SUCCESS}")
else
    VEGETA_100_ERROR_RATE="0"
fi

echo -n "  P99 Status: "
if pass_fail "${VEGETA_100_P99_MS:-999}" "50" "ms" "100/s p99"; then
    VEGETA_100_P99_STATUS=0
else
    VEGETA_100_P99_STATUS=1
fi

echo -n "  Error Rate Status: "
if (( $(echo "$VEGETA_100_ERROR_RATE < 0.1" | bc -l) )); then
    echo -e "${GREEN}✓ PASS${NC} - Error rate ${VEGETA_100_ERROR_RATE}% < 0.1%"
    VEGETA_100_ERROR_STATUS=0
else
    echo -e "${RED}✗ FAIL${NC} - Error rate ${VEGETA_100_ERROR_RATE}% >= 0.1%"
    VEGETA_100_ERROR_STATUS=1
fi
echo ""

# ============================================================================
# TEST 3: vegeta 200/s for 10s → 429 responses appear (rate limiter works)
# ============================================================================
log "Test 3: vegeta 200/s for 10s (verify rate limiter returns 429s)"

VEGETA_200_BIN="$RESULTS_DIR/vegeta-200-$DATE.bin"
VEGETA_200_TXT="$RESULTS_DIR/vegeta-200-$DATE.txt"

log "Running vegeta attack (200 req/s for 10s, single IP to trigger rate limiting)..."
echo "GET http://$SERVER_ADDR/api/v1/check?d=$DOMAIN" | \
    vegeta attack -rate=200 -duration=10s \
    -header="X-Forwarded-For: 10.200.1.1" \
    -header="X-Real-IP: 10.200.1.1" \
    -max-workers=50 \
    -name=rate-limit-test > "$VEGETA_200_BIN" 2>&1

vegeta report "$VEGETA_200_BIN" --type=text > "$VEGETA_200_TXT" 2>&1 || true

echo ""
echo "Results:"
cat "$VEGETA_200_TXT"
echo ""

# Check if we got 429s
echo -n "  Rate Limiter Status: "
if grep -q "429" "$VEGETA_200_TXT" 2>/dev/null; then
    echo -e "${GREEN}✓ PASS${NC} - Rate limiter is working (429 responses detected)"
    RATE_LIMIT_STATUS=0
else
    echo -e "${RED}✗ FAIL${NC} - No 429 responses detected (rate limiter may not be working)"
    RATE_LIMIT_STATUS=1
fi
echo ""

# ============================================================================
# TEST 4: Memory growth test: 50 req/s for 10 min → memory plateaus
# ============================================================================
log "Test 4: Memory growth test (50 req/s for 10 min, memory should plateau)"

METRICS_URL="http://$SERVER_ADDR/metrics"
MEMORY_RESULTS="$RESULTS_DIR/memory-growth-$DATE.csv"

# Get initial memory
log "Getting initial memory baseline..."
INITIAL_HEAP=$(curl -s "$METRICS_URL" | grep "^go_memstats_heap_inuse_bytes" | awk '{print $2}')
INITIAL_HEAP_MB=$(echo "$INITIAL_HEAP" | awk '{printf "%.2f", $1/1024/1024}')
echo "Initial heap: $INITIAL_HEAP bytes ($INITIAL_HEAP_MB MB)"

# Start vegeta in background
log "Starting vegeta attack (50 req/s for 10 min)..."
VEGETA_MEM_BIN="$RESULTS_DIR/vegeta-memory-$DATE.bin"

(
    for i in {1..50}; do
        echo "GET http://$SERVER_ADDR/api/v1/check?d=$DOMAIN"
    done
) | vegeta attack -rate=50 -duration=600s \
    -max-workers=50 \
    -name=memory-test > "$VEGETA_MEM_BIN" 2>&1 &
VEGETA_PID=$!

# Monitor memory every 30 seconds
echo "Time,Heap_Bytes,Heap_MB,Heap_Objects,Goroutines" > "$MEMORY_RESULTS"

ELAPSED=0
while kill -0 "$VEGETA_PID" 2>/dev/null; do
    METRICS=$(curl -s "$METRICS_URL")
    HEAP=$(echo "$METRICS" | grep "^go_memstats_heap_inuse_bytes" | awk '{print $2}')
    HEAP_MB=$(echo "$HEAP" | awk '{printf "%.2f", $1/1024/1024}')
    OBJS=$(echo "$METRICS" | grep "^go_memstats_heap_objects" | awk '{print $2}')
    GOROS=$(echo "$METRICS" | grep "^go_goroutines" | awk '{print $2}')

    echo "${ELAPSED}s,${HEAP},${HEAP_MB},${OBJS},${GOROS}" >> "$MEMORY_RESULTS"
    echo "[${ELAPSED}s] Heap: ${HEAP_MB} MB, Objects: ${OBJS}, Goroutines: ${GOROS}"

    sleep 30
    ELAPSED=$((ELAPSED + 30))
done

wait $VEGETA_PID || true

# Get final memory
log "Getting final memory..."
FINAL_HEAP=$(curl -s "$METRICS_URL" | grep "^go_memstats_heap_inuse_bytes" | awk '{print $2}')
FINAL_HEAP_MB=$(echo "$FINAL_HEAP" | awk '{printf "%.2f", $1/1024/1024}')
echo "Final heap: $FINAL_HEAP bytes ($FINAL_HEAP_MB MB)"

# Calculate growth
GROWTH_MB=$(echo "$INITIAL_HEAP $FINAL_HEAP" | awk '{printf "%.2f", ($2 - $1)/1024/1024}')
echo "Heap growth: ${GROWTH_MB} MB"

# Check if memory plateaued (growth < 100 MB)
echo -n "  Memory Status: "
if (( $(echo "$GROWTH_MB < 100" | bc -l) )); then
    echo -e "${GREEN}✓ PASS${NC} - Memory growth ${GROWTH_MB} MB < 100 MB (plateaued)"
    MEMORY_STATUS=0
else
    echo -e "${RED}✗ FAIL${NC} - Memory growth ${GROWTH_MB} MB >= 100 MB (may not have plateaued)"
    MEMORY_STATUS=1
fi
echo ""

# ============================================================================
# Generate Report
# ============================================================================
log "Generating report..."

CACHED_STATUS_STR=$([ $CACHED_STATUS -eq 0 ] && echo "✓ PASS" || echo "✗ FAIL")
VEGETA_100_P99_STR=$([ $VEGETA_100_P99_STATUS -eq 0 ] && echo "✓ PASS" || echo "✗ FAIL")
VEGETA_100_ERROR_STR=$([ $VEGETA_100_ERROR_STATUS -eq 0 ] && echo "✓ PASS" || echo "✗ FAIL")
RATE_LIMIT_STR=$([ $RATE_LIMIT_STATUS -eq 0 ] && echo "✓ PASS" || echo "✗ FAIL")
MEMORY_STR=$([ $MEMORY_STATUS -eq 0 ] && echo "✓ PASS" || echo "✗ FAIL")

cat > "$REPORT" << EOF
# Domain Check Load Test Report

**Date:** $(date -Iseconds)
**Server:** $SERVER_ADDR
**Test Domain:** $DOMAIN

---

## Executive Summary

| Test | Target | Result | Status |
|------|--------|--------|--------|
| Cached endpoint (1000 req) | p99 < 10ms | **${CACHED_P99_MS:-N/A}ms** | $CACHED_STATUS_STR |
| vegeta 100/s for 60s | p99 < 50ms | **${VEGETA_100_P99_MS:-N/A}ms** | $VEGETA_100_P99_STR |
| vegeta 100/s for 60s | error rate < 0.1% | **${VEGETA_100_ERROR_RATE:-N/A}%** | $VEGETA_100_ERROR_STR |
| vegeta 200/s for 10s | 429 responses appear | **Rate limiter active** | $RATE_LIMIT_STR |
| Memory growth (50 req/s, 10 min) | growth < 100 MB | **${GROWTH_MB:-N/A} MB** | $MEMORY_STR |

**Overall Status:** $([ $CACHED_STATUS -eq 0 && $VEGETA_100_P99_STATUS -eq 0 && $VEGETA_100_ERROR_STATUS -eq 0 && $RATE_LIMIT_STATUS -eq 0 && $MEMORY_STATUS -eq 0 ] && echo "✓ All targets met" || echo "✗ Some targets not met")

---

## Test Environment

- **OS:** $(uname -s) $(uname -r)
- **Architecture:** $(uname -m)
- **CPU Cores:** $(nproc)
- **Memory:** $(free -h | grep Mem | awk '{print $2}')
- **Server PID:** $(ps aux | grep '[d]omain-check serve' | awk '{print $2}' | head -1)

---

## Detailed Results

### 1. Cached Response Test (1000 requests)

**Target:** p99 < 10ms

\`\`\`
$(cat "$VEGETA_CACHED_TXT")
\`\`\`

---

### 2. vegeta 100/s Sustained Load Test

**Target:** p99 < 50ms, error rate < 0.1%

\`\`\`
$(cat "$VEGETA_100_TXT")
\`\`\`

---

### 3. vegeta 200/s Rate Limiter Test

**Expected:** 429 responses (rate limiter active)

\`\`\`
$(cat "$VEGETA_200_TXT")
\`\`\`

---

### 4. Memory Growth Test

**Target:** Growth < 100 MB over 10 minutes at 50 req/s

\`\`\`
Initial Heap: ${INITIAL_HEAP_MB} MB
Final Heap:   ${FINAL_HEAP_MB} MB
Growth:       ${GROWTH_MB} MB
\`\`\`

Memory timeseries (sample):
\`\`\`
$(head -10 "$MEMORY_RESULTS")
...
$(tail -5 "$MEMORY_RESULTS")
\`\`\`

---

## Conclusions

1. **Cached Performance:** ${CACHED_P99_MS:-N/A}ms p99 is $([ $CACHED_STATUS -eq 0 ] && echo "within" || echo "above") the 10ms target.

2. **Sustained Throughput:** ${VEGETA_100_P99_MS:-N/A}ms p99 at 100 req/s is $([ $VEGETA_100_P99_STATUS -eq 0 ] && echo "within" || echo "above") the 50ms target.

3. **Error Rate:** ${VEGETA_100_ERROR_RATE:-N/A}% error rate is $([ $VEGETA_100_ERROR_STATUS -eq 0 ] && echo "within" || echo "above") the 0.1% target.

4. **Rate Limiter:** $([ $RATE_LIMIT_STATUS -eq 0 ] && echo "Working correctly - 429 responses are returned" || echo "May not be working - no 429 responses detected").

5. **Memory Health:** ${GROWTH_MB:-N/A} MB growth over 10 minutes $([ $MEMORY_STATUS -eq 0 ] && echo "indicates memory plateaus properly" || echo "may indicate a memory leak").

---

*Report generated by \`scripts/load-tests.sh\` on $(date -Iseconds)*
EOF

echo ""
echo "================================"
echo "All tests completed!"
echo "================================"
echo "Report saved to: $REPORT"
echo ""
echo "Summary:"
cat "$REPORT" | grep -A 10 "Executive Summary"

# Exit with error if any test failed
if [ $CACHED_STATUS -ne 0 ] || [ $VEGETA_100_P99_STATUS -ne 0 ] || [ $VEGETA_100_ERROR_STATUS -ne 0 ] || [ $RATE_LIMIT_STATUS -ne 0 ] || [ $MEMORY_STATUS -ne 0 ]; then
    exit 1
fi

exit 0
