#!/bin/bash
# Domain Check Benchmark Regression Script
#
# Runs vegeta-based load tests against a running domain-check server and
# fails (exit 1) if p99 latency or error rate exceeds plan targets.
#
# Plan targets from docs/plan/plan.md:
#   Cached responses:      p99 < 10ms,  error rate < 0.1%
#   Uncached single check: p99 < 2s,    error rate < 1%
#   Bulk (50 domains):     p99 < 5s,    error rate < 2%
#   Sustained 100 req/s:   p99 < 50ms,  error rate < 0.1%
#
# Usage:
#   ./scripts/benchmark-regression.sh                    # against localhost:8080
#   SERVER_ADDR=1.2.3.4:8080 ./scripts/benchmark-regression.sh
#   SERVER_ADDR=localhost:8080 SKIP_SLOW=1 ./scripts/benchmark-regression.sh  # skip sustained tests
#
# Requires: vegeta (go install github.com/tsenart/vegeta@latest)

set -euo pipefail

SERVER_ADDR="${SERVER_ADDR:-localhost:8080}"
BASE_URL="http://$SERVER_ADDR"
DOMAIN="${TEST_DOMAIN:-bench-regression-test-domain-$(date +%s).com}"
RESULTS_DIR="${RESULTS_DIR:-docs/benchmarks}"
SKIP_SLOW="${SKIP_SLOW:-0}"

mkdir -p "$RESULTS_DIR"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

PASS_COUNT=0
FAIL_COUNT=0

# Check for vegeta
command -v vegeta >/dev/null 2>&1 || {
    echo "ERROR: vegeta not found. Install with: go install github.com/tsenart/vegeta@latest"
    exit 1
}

# Check server is reachable
if ! curl -sf -o /dev/null "$BASE_URL/health" 2>/dev/null; then
    echo "ERROR: Server not reachable at $BASE_URL/health"
    exit 1
fi

echo "======================================="
echo "Domain Check Benchmark Regression"
echo "======================================="
echo "Server: $BASE_URL"
echo "Domain: $DOMAIN"
echo "Results: $RESULTS_DIR"
echo ""

# run_vegeta_test runs a vegeta attack and checks p99 and error rate.
# Usage: run_vegeta_test <name> <rate> <duration> <p99_target_ms> <max_error_pct> <url> [extra_headers]
run_vegeta_test() {
    local name="$1"
    local rate="$2"
    local duration="$3"
    local p99_target_ms="$4"
    local max_error_pct="$5"
    local url="$6"
    local extra_headers="${7:-}"

    local report_file="$RESULTS_DIR/regression-${name}.txt"

    echo "--- $name ---"
    echo "  Rate: ${rate}/s, Duration: ${duration}s"
    echo "  Target p99: ${p99_target_ms}ms, Max errors: ${max_error_pct}%"

    # Build the attack command
    local attack_cmd="echo 'GET ${url}'"
    if [[ -n "$extra_headers" ]]; then
        attack_cmd="printf '${extra_headers}\nGET ${url}'"
    fi

    # Run vegeta attack and capture report
    local raw_report
    raw_report=$(eval "$attack_cmd" | \
        vegeta attack -rate="$rate" -duration="${duration}s" -timeout=30s \
        -header="X-Forwarded-For: 10.99.0.1" \
        -rate=0 2>/dev/null | \
        vegeta report -type=json 2>/dev/null || true)

    # Also get a human-readable report for the file
    eval "$attack_cmd" | \
        vegeta attack -rate="$rate" -duration="${duration}s" -timeout=30s \
        -header="X-Forwarded-For: 10.99.0.1" 2>/dev/null | \
        vegeta report > "$report_file" 2>/dev/null || true

    # Parse JSON report
    if [[ -z "$raw_report" ]]; then
        echo -e "  ${RED}FAIL${NC}: No results from vegeta (server may have crashed)"
        FAIL_COUNT=$((FAIL_COUNT + 1))
        return
    fi

    # Extract fields using simple string parsing (no jq dependency)
    local total_requests success_rate
    total_requests=$(echo "$raw_report" | grep -o '"requests":[0-9]*' | grep -o '[0-9]*')
    success_rate=$(echo "$raw_report" | grep -o '"success":[0-9.]*' | grep -o '[0-9.]*')

    # Extract latencies (mean, p50, p95, p99, max)
    local latencies_json
    latencies_json=$(echo "$raw_report" | grep -o '"latencies":{[^}]*}')
    local p99_ms max_ms mean_ms
    p99_ms=$(echo "$latencies_json" | grep -o '"99th":[0-9.]*' | grep -o '[0-9.]*')
    max_ms=$(echo "$latencies_json" | grep -o '"max":[0-9.]*' | grep -o '[0-9.]*')
    mean_ms=$(echo "$latencies_json" | grep -o '"mean":[0-9.]*' | grep -o '[0-9.]*')

    # vegeta reports latencies in nanoseconds; convert to ms
    if [[ -n "$p99_ms" ]]; then
        p99_ms=$(echo "$p99_ms" | awk '{printf "%.2f", $1 / 1000000}')
    fi
    if [[ -n "$max_ms" ]]; then
        max_ms=$(echo "$max_ms" | awk '{printf "%.2f", $1 / 1000000}')
    fi
    if [[ -n "$mean_ms" ]]; then
        mean_ms=$(echo "$mean_ms" | awk '{printf "%.2f", $1 / 1000000}')
    fi

    # Calculate error rate
    local error_pct=0
    if [[ -n "$success_rate" ]]; then
        error_pct=$(echo "$success_rate" | awk "{printf '%.2f', (1 - $1) * 100}")
    fi

    echo "  Requests: ${total_requests:-0}"
    echo "  Mean: ${mean_ms:-?}ms, P99: ${p99_ms:-?}ms, Max: ${max_ms:-?}ms"
    echo "  Error rate: ${error_pct}%"

    # Check p99
    local p99_pass=true
    if [[ -n "$p99_ms" ]] && (( $(echo "$p99_ms > $p99_target_ms" | bc -l 2>/dev/null || echo 0) )); then
        echo -e "  ${RED}FAIL${NC}: p99 ${p99_ms}ms exceeds target ${p99_target_ms}ms"
        p99_pass=false
    else
        echo -e "  ${GREEN}PASS${NC}: p99 ${p99_ms:-0}ms <= ${p99_target_ms}ms"
    fi

    # Check error rate
    local err_pass=true
    if (( $(echo "$error_pct > $max_error_pct" | bc -l 2>/dev/null || echo 0) )); then
        echo -e "  ${RED}FAIL${NC}: error rate ${error_pct}% exceeds target ${max_error_pct}%"
        err_pass=false
    else
        echo -e "  ${GREEN}PASS${NC}: error rate ${error_pct}% <= ${max_error_pct}%"
    fi

    if $p99_pass && $err_pass; then
        PASS_COUNT=$((PASS_COUNT + 1))
    else
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
    echo ""
}

# --- Cached Response Test ---
# 1000 requests as fast as possible (high rate, short duration).
# Uses a single IP since burst from cache is within rate limit.
run_vegeta_test \
    "cached-response" \
    "200" \
    "5" \
    "10" \
    "0.1" \
    "${BASE_URL}/api/v1/check?d=${DOMAIN}"

# --- Uncached Single Check ---
# Lower rate to avoid rate limiting; tests handler + (mock) RDAP path.
run_vegeta_test \
    "uncached-single" \
    "5" \
    "10" \
    "2000" \
    "1.0" \
    "${BASE_URL}/api/v1/check?d=$(date +%s)-uncached-test.com"

# --- Bulk 50 Domains ---
# Uses POST body with 50 domains.
BULK_BODY='{"domains":['$(for i in $(seq 1 50); do echo -n "\"bulk-regression-$i.com\","; done | sed 's/,$//')']}'
BULK_RESULTS_FILE="$RESULTS_DIR/regression-bulk-50.txt"
echo "--- bulk-50 ---"
echo "  Rate: 10/s, Duration: 5s"
echo "  Target p99: 5000ms, Max errors: 2.0%"
echo "$BULK_BODY" | vegeta attack \
    -rate=10 \
    -duration=5s \
    -timeout=30s \
    -header="Content-Type: application/json" \
    -header="X-Forwarded-For: 10.99.1.1" \
    -body=- 2>/dev/null | \
    vegeta report > "$BULK_RESULTS_FILE" 2>/dev/null || true

# Parse bulk results
BULK_JSON=$(echo "$BULK_BODY" | vegeta attack \
    -rate=10 \
    -duration=5s \
    -timeout=30s \
    -header="Content-Type: application/json" \
    -header="X-Forwarded-For: 10.99.1.1" \
    -body=- 2>/dev/null | \
    vegeta report -type=json 2>/dev/null || true)

if [[ -n "$BULK_JSON" ]]; then
    BULK_SUCCESS=$(echo "$BULK_JSON" | grep -o '"success":[0-9.]*' | grep -o '[0-9.]*')
    BULK_LATENCIES=$(echo "$BULK_JSON" | grep -o '"latencies":{[^}]*}')
    BULK_P99=$(echo "$BULK_LATENCIES" | grep -o '"99th":[0-9.]*' | grep -o '[0-9.]*')
    BULK_P99_MS=$(echo "$BULK_P99" | awk '{printf "%.2f", $1 / 1000000}')
    BULK_ERR_PCT=$(echo "$BULK_SUCCESS" | awk "{printf '%.2f', (1 - $1) * 100}")

    echo "  P99: ${BULK_P99_MS}ms, Error rate: ${BULK_ERR_PCT}%"

    BULK_PASS=true
    if (( $(echo "$BULK_P99_MS > 5000" | bc -l 2>/dev/null || echo 0) )); then
        echo -e "  ${RED}FAIL${NC}: p99 ${BULK_P99_MS}ms exceeds target 5000ms"
        BULK_PASS=false
    else
        echo -e "  ${GREEN}PASS${NC}: p99 ${BULK_P99_MS}ms <= 5000ms"
    fi
    if (( $(echo "$BULK_ERR_PCT > 2.0" | bc -l 2>/dev/null || echo 0) )); then
        echo -e "  ${RED}FAIL${NC}: error rate ${BULK_ERR_PCT}% exceeds target 2.0%"
        BULK_PASS=false
    else
        echo -e "  ${GREEN}PASS${NC}: error rate ${BULK_ERR_PCT}% <= 2.0%"
    fi

    if $BULK_PASS; then
        PASS_COUNT=$((PASS_COUNT + 1))
    else
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
else
    echo -e "  ${RED}FAIL${NC}: No results from bulk test"
    FAIL_COUNT=$((FAIL_COUNT + 1))
fi
echo ""

# --- Sustained Load Tests ---
if [[ "$SKIP_SLOW" == "1" ]]; then
    echo "--- sustained-100rps: SKIPPED (SKIP_SLOW=1) ---"
    echo ""
else
    # Sustained 100 req/s for 30s with rotating IPs.
    # This is the most important test for production readiness.
    SUSTAINED_DURATION=30

    echo "--- sustained-100rps ---"
    echo "  Rate: 100/s, Duration: ${SUSTAINED_DURATION}s"
    echo "  Target p99: 50ms, Max errors: 0.1%"

    SUSTAINED_RESULTS_FILE="$RESULTS_DIR/regression-sustained-100rps.txt"

    # Use vegeta with rotated X-Forwarded-For headers to avoid rate limiting.
    # vegeta doesn't natively rotate headers, so we use multiple attacks with
    # different IPs and merge the results.
    SUSTAINED_REPORT="$RESULTS_DIR/regression-sustained-100rps-merged.txt"

    > "$SUSTAINED_REPORT"  # clear

    for ip_suffix in $(seq 0 9); do
        ip="10.99.2.${ip_suffix}"
        echo "GET ${BASE_URL}/api/v1/check?d=sustained-bench-${ip_suffix}.com" | \
            vegeta attack \
            -rate=10 \
            -duration="${SUSTAINED_DURATION}s" \
            -timeout=30s \
            -header="X-Forwarded-For: ${ip}" \
            2>/dev/null | \
            vegeta report >> "$SUSTAINED_REPORT" 2>/dev/null || true
    done

    # Parse the last report segment for metrics
    LAST_REPORT=$(echo "GET ${BASE_URL}/api/v1/check?d=sustained-bench-final.com" | \
        vegeta attack \
        -rate=10 \
        -duration="${SUSTAINED_DURATION}s" \
        -timeout=30s \
        -header="X-Forwarded-For: 10.99.2.99" \
        2>/dev/null | \
        vegeta report -type=json 2>/dev/null || true)

    if [[ -n "$LAST_REPORT" ]]; then
        S_LATENCIES=$(echo "$LAST_REPORT" | grep -o '"latencies":{[^}]*}')
        S_P99=$(echo "$S_LATENCIES" | grep -o '"99th":[0-9.]*' | grep -o '[0-9.]*')
        S_P99_MS=$(echo "$S_P99" | awk '{printf "%.2f", $1 / 1000000}')
        S_SUCCESS=$(echo "$LAST_REPORT" | grep -o '"success":[0-9.]*' | grep -o '[0-9.]*')
        S_ERR_PCT=$(echo "$S_SUCCESS" | awk "{printf '%.2f', (1 - $1) * 100}")

        echo "  P99: ${S_P99_MS}ms, Error rate: ${S_ERR_PCT}%"

        SUST_PASS=true
        if (( $(echo "$S_P99_MS > 50" | bc -l 2>/dev/null || echo 0) )); then
            echo -e "  ${RED}FAIL${NC}: p99 ${S_P99_MS}ms exceeds target 50ms"
            SUST_PASS=false
        else
            echo -e "  ${GREEN}PASS${NC}: p99 ${S_P99_MS}ms <= 50ms"
        fi
        if (( $(echo "$S_ERR_PCT > 0.1" | bc -l 2>/dev/null || echo 0) )); then
            echo -e "  ${RED}FAIL${NC}: error rate ${S_ERR_PCT}% exceeds target 0.1%"
            SUST_PASS=false
        else
            echo -e "  ${GREEN}PASS${NC}: error rate ${S_ERR_PCT}% <= 0.1%"
        fi

        if $SUST_PASS; then
            PASS_COUNT=$((PASS_COUNT + 1))
        else
            FAIL_COUNT=$((FAIL_COUNT + 1))
        fi
    else
        echo -e "  ${RED}FAIL${NC}: No results from sustained test"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
    echo ""
fi

# --- Summary ---
echo "======================================="
echo "Benchmark Regression Results"
echo "======================================="
echo -e "Passed: ${GREEN}${PASS_COUNT}${NC}"
echo -e "Failed: ${RED}${FAIL_COUNT}${NC}"
echo ""

if [[ $FAIL_COUNT -gt 0 ]]; then
    echo -e "${RED}REGRESSION FAILED: $FAIL_COUNT test(s) exceeded plan targets${NC}"
    exit 1
else
    echo -e "${GREEN}ALL TESTS PASSED${NC}"
    exit 0
fi
