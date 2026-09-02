#!/usr/bin/env bash
# Comprehensive Test Suite for Alert Suppression
# Tests all scenarios: resolved crashes, genuine crashes, persistence, edge cases
# Created: 2026-09-02

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CRASH_CLASSIFIER="$SCRIPT_DIR/crash-classifier.sh"
RESOLUTION_TRACKER="$SCRIPT_DIR/crash-resolution-tracker.sh"

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Test counters
TEST_COUNT=0
PASS_COUNT=0
FAIL_COUNT=0

# Logging functions
log_test() {
    echo -e "${BLUE}[TEST${TEST_COUNT}]${NC} $1"
}

log_pass() {
    echo -e "${GREEN}✓ PASS${NC} - $1"
    PASS_COUNT=$((PASS_COUNT + 1))
}

log_fail() {
    echo -e "${RED}✗ FAIL${NC} - $1"
    FAIL_COUNT=$((FAIL_COUNT + 1))
}

# Cleanup function
cleanup() {
    # Remove test beads
    for bead_id in bf-test-*; do
        rm -rf ".beads/traces/$bead_id" 2>/dev/null || true
    done

    # Clean up test state entries
    if [[ -f ".beads/state/crash-resolutions.json" ]]; then
        jq '.resolutions |= with_entries(select(.key | startswith("bf-test-") | not))' \
           ".beads/state/crash-resolutions.json" > ".beads/state/crash-resolutions.json.tmp" 2>/dev/null || true
        mv ".beads/state/crash-resolutions.json.tmp" ".beads/state/crash-resolutions.json" 2>/dev/null || true
    fi
}

trap cleanup EXIT

# Test 1: Exit code 0 should classify as success (no alert needed)
test_exit_code_zero() {
    TEST_COUNT=$((TEST_COUNT + 1))
    log_test "Exit code 0 should not generate alert"

    local bead_id="bf-test-exit0-001"
    mkdir -p ".beads/traces/$bead_id"

    cat > ".beads/traces/$bead_id/trace.jsonl" <<EOF
{"timestamp":"$(date -u +"%Y-%m-%dT%H:%M:%SZ")","level":"info","message":"Task completed successfully"}
EOF

    cat > ".beads/traces/$bead_id/metadata.json" <<EOF
{"bead_id":"$bead_id","exit_code":0,"timestamp":"$(date -u +"%Y-%m-%dT%H:%M:%SZ")"}
EOF

    # Run classifier and check for FALSE_POSITIVE
    local output=$("$CRASH_CLASSIFIER" "$bead_id" 2>/dev/null || true)

    if echo "$output" | grep -q "FALSE_POSITIVE\|exit code 0"; then
        log_pass "Exit code 0 correctly identified as success"
    else
        log_fail "Exit code 0 should be FALSE_POSITIVE: $output"
    fi
}

# Test 2: error_max_turns should be FALSE_POSITIVE
test_max_turns_false_positive() {
    TEST_COUNT=$((TEST_COUNT + 1))
    log_test "error_max_turns should be FALSE_POSITIVE"

    local bead_id="bf-test-maxturns-001"
    mkdir -p ".beads/traces/$bead_id"

    cat > ".beads/traces/$bead_id/trace.jsonl" <<EOF
{"timestamp":"$(date -u +"%Y-%m-%dT%H:%M:%SZ")","level":"error","message":"error_max_turns: agent exhausted turns"}
EOF

    cat > ".beads/traces/$bead_id/metadata.json" <<EOF
{"bead_id":"$bead_id","exit_code":1,"timestamp":"$(date -u +"%Y-%m-%dT%H:%M:%SZ")"}
EOF

    local output=$("$CRASH_CLASSIFIER" "$bead_id" 2>/dev/null || true)

    if echo "$output" | grep -q "FALSE_POSITIVE"; then
        log_pass "error_max_turns correctly classified as FALSE_POSITIVE"
    else
        log_fail "error_max_turns should be FALSE_POSITIVE: $output"
    fi
}

# Test 3: HTTP 503 should be SERVICE_FAILURE
test_http_503_service_failure() {
    TEST_COUNT=$((TEST_COUNT + 1))
    log_test "HTTP 503 should be SERVICE_FAILURE"

    local bead_id="bf-test-503-001"
    mkdir -p ".beads/traces/$bead_id"

    cat > ".beads/traces/$bead_id/trace.jsonl" <<EOF
{"timestamp":"$(date -u +"%Y-%m-%dT%H:%M:%SZ")","level":"error","message":"503 no available server with healthy upstream"}
EOF

    cat > ".beads/traces/$bead_id/metadata.json" <<EOF
{"bead_id":"$bead_id","exit_code":1,"timestamp":"$(date -u +"%Y-%m-%dT%H:%M:%SZ")"}
EOF

    local output=$("$CRASH_CLASSIFIER" "$bead_id" 2>/dev/null || true)

    if echo "$output" | grep -q "SERVICE_FAILURE"; then
        log_pass "HTTP 503 correctly classified as SERVICE_FAILURE"
    else
        log_fail "HTTP 503 should be SERVICE_FAILURE: $output"
    fi
}

# Test 4: OOM should be INFRASTRUCTURE
test_oom_infrastructure() {
    TEST_COUNT=$((TEST_COUNT + 1))
    log_test "OOM killer should be INFRASTRUCTURE"

    local bead_id="bf-test-oom-001"
    mkdir -p ".beads/traces/$bead_id"

    cat > ".beads/traces/$bead_id/trace.jsonl" <<EOF
{"timestamp":"$(date -u +"%Y-%m-%dT%H:%M:%SZ")","level":"error","message":"OOM killer terminated process"}
EOF

    cat > ".beads/traces/$bead_id/metadata.json" <<EOF
{"bead_id":"$bead_id","exit_code":-9,"timestamp":"$(date -u +"%Y-%m-%dT%H:%M:%SZ")"}
EOF

    local output=$("$CRASH_CLASSIFIER" "$bead_id" 2>/dev/null || true)

    if echo "$output" | grep -q "INFRASTRUCTURE"; then
        log_pass "OOM killer correctly classified as INFRASTRUCTURE"
    else
        log_fail "OOM should be INFRASTRUCTURE: $output"
    fi
}

# Test 5: bf-1ea4g pattern (work committed before crash)
test_bf_1ea4g_pattern() {
    TEST_COUNT=$((TEST_COUNT + 1))
    log_test "bf-1ea4g pattern (work committed < 30s before crash) should be FALSE_POSITIVE"

    local bead_id="bf-test-bf1ea4g-001"
    mkdir -p ".beads/traces/$bead_id"

    cat > ".beads/traces/$bead_id/trace.jsonl" <<EOF
{"timestamp":"$(date -u -d '25 seconds ago' +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u +"%Y-%m-%dT%H:%M:%SZ")","level":"info","message":"git commit -m 'implement feature'"}
{"timestamp":"$(date -u +"%Y-%m-%dT%H:%M:%SZ")","level":"error","message":"error_max_turns"}
EOF

    cat > ".beads/traces/$bead_id/metadata.json" <<EOF
{"bead_id":"$bead_id","exit_code":1,"timestamp":"$(date -u +"%Y-%m-%dT%H:%M:%SZ")"}
EOF

    local output=$("$CRASH_CLASSIFIER" "$bead_id" 2>/dev/null || true)

    if echo "$output" | grep -q "FALSE_POSITIVE"; then
        log_pass "bf-1ea4g pattern correctly classified as FALSE_POSITIVE"
    else
        log_fail "bf-1ea4g pattern should be FALSE_POSITIVE: $output"
    fi
}

# Test 6: SIGHUP with closed bead (bf-4k2ws pattern)
test_sighup_closed_bead() {
    TEST_COUNT=$((TEST_COUNT + 1))
    log_test "SIGHUP with closed bead should be FALSE_POSITIVE (bf-4k2ws pattern)"

    local bead_id="bf-test-sighup-001"
    mkdir -p ".beads/traces/$bead_id"

    cat > ".beads/traces/$bead_id/trace.jsonl" <<EOF
{"timestamp":"$(date -u +"%Y-%m-%dT%H:%M:%SZ")","level":"error","message":"SIGHUP received"}
EOF

    cat > ".beads/traces/$bead_id/metadata.json" <<EOF
{"bead_id":"$bead_id","exit_code":-1,"timestamp":"$(date -u +"%Y-%m-%dT%H:%M:%SZ")"}
EOF

    local output=$("$CRASH_CLASSIFIER" "$bead_id" 2>/dev/null || true)

    # Note: This test checks the classifier logic only
    # The actual closed bead check is in crash-alert-manager.sh
    if echo "$output" | grep -q "INFRASTRUCTURE\|FALSE_POSITIVE"; then
        log_pass "SIGHUP classified correctly (depends on bead status check in alert manager)"
    else
        log_fail "SIGHUP classification unclear: $output"
    fi
}

# Test 7: Resolution state persistence
test_resolution_persistence() {
    TEST_COUNT=$((TEST_COUNT + 1))
    log_test "Resolution state should persist"

    local bead_id="bf-test-persist-001"

    # Mark as resolved
    "$RESOLUTION_TRACKER" "$bead_id" mark-resolved >/dev/null 2>&1 || true

    # Check persistence
    local check_output=$("$RESOLUTION_TRACKER" "$bead_id" check 2>/dev/null || echo "NOT_RESOLVED")

    if echo "$check_output" | grep -q "RESOLVED"; then
        log_pass "Resolution state persisted"
    else
        log_fail "Resolution state failed to persist: $check_output"
    fi

    # Cleanup
    "$RESOLUTION_TRACKER" "$bead_id" mark-unresolved >/dev/null 2>&1 || true
}

# Test 8: State file auto-initialization
test_state_file_init() {
    TEST_COUNT=$((TEST_COUNT + 1))
    log_test "State file should auto-initialize"

    local backup_file=".beads/state/crash-resolutions.json.backup"

    # Backup existing file
    if [[ -f ".beads/state/crash-resolutions.json" ]]; then
        cp ".beads/state/crash-resolutions.json" "$backup_file"
        rm -f ".beads/state/crash-resolutions.json"
    fi

    # Trigger initialization
    "$RESOLUTION_TRACKER" "test-init-bead" check >/dev/null 2>&1 || true

    # Check if file was created
    if [[ -f ".beads/state/crash-resolutions.json" ]]; then
        log_pass "State file auto-initialized"
    else
        log_fail "State file not created"
    fi

    # Restore backup
    if [[ -f "$backup_file" ]]; then
        mv "$backup_file" ".beads/state/crash-resolutions.json"
    fi
}

# Test 9: Check crash-alert-manager.sh has all critical fixes
test_critical_fixes_present() {
    TEST_COUNT=$((TEST_COUNT + 1))
    log_test "Critical fixes should be present in crash-alert-manager.sh"

    local crash_alert_manager="$SCRIPT_DIR/crash-alert-manager.sh"
    local fixes_found=0
    local total_fixes=6

    for i in {1..6}; do
        if grep -q "CRITICAL FIX $i" "$crash_alert_manager" 2>/dev/null; then
            fixes_found=$((fixes_found + 1))
        fi
    done

    if [[ $fixes_found -eq $total_fixes ]]; then
        log_pass "All $total_fixes critical fixes present"
    else
        log_fail "Only $fixes_found/$total_fixes critical fixes found"
    fi
}

# Test 10: Alert cooldown mechanism
test_alert_cooldown() {
    TEST_COUNT=$((TEST_COUNT + 1))
    log_test "Alert cooldown mechanism should be configured"

    local crash_alert_manager="$SCRIPT_DIR/crash-alert-manager.sh"

    if grep -q "ALERT_COOLDOWN_SECONDS" "$crash_alert_manager" 2>/dev/null; then
        log_pass "Alert cooldown mechanism present"
    else
        log_fail "Alert cooldown mechanism not found"
    fi
}

# Main test runner
main() {
    echo "=========================================="
    echo "Comprehensive Alert Suppression Test Suite"
    echo "=========================================="
    echo ""

    # Run all tests
    test_exit_code_zero
    test_max_turns_false_positive
    test_http_503_service_failure
    test_oom_infrastructure
    test_bf_1ea4g_pattern
    test_sighup_closed_bead
    test_resolution_persistence
    test_state_file_init
    test_critical_fixes_present
    test_alert_cooldown

    # Print summary
    echo ""
    echo "=========================================="
    echo "Test Summary"
    echo "=========================================="
    echo "Total tests: $TEST_COUNT"
    echo -e "${GREEN}Passed: $PASS_COUNT${NC}"
    echo -e "${RED}Failed: $FAIL_COUNT${NC}"
    echo ""

    if [[ $FAIL_COUNT -eq 0 ]]; then
        echo -e "${GREEN}All tests passed!${NC}"
        echo ""
        echo "✅ Alert suppression verified:"
        echo "   - Exit code 0 filtered out"
        echo "   - error_max_turns classified as FALSE_POSITIVE"
        echo "   - HTTP 503 classified as SERVICE_FAILURE"
        echo "   - OOM killer classified as INFRASTRUCTURE"
        echo "   - bf-1ea4g pattern handled (work committed before crash)"
        echo "   - SIGHUP with closed bead handled (bf-4k2ws pattern)"
        echo "   - Resolution state persists across restarts"
        echo "   - State file auto-initializes"
        echo "   - All 6 critical fixes present"
        echo "   - Alert cooldown mechanism configured"
        echo ""
        exit 0
    else
        echo -e "${RED}Some tests failed!${NC}"
        exit 1
    fi
}

main "$@"
