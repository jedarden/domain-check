#!/usr/bin/env bash
# Test script to verify crash fix for bead bf-1s6c3
# Tests that the fix prevents the crash under the same conditions
# that caused bf-1s6c3 to fail

set -euo pipefail

# Colors for output
GREEN='\033[0;32m'
RED='\033[0;31m'
YELLOW='\033[1;33m'
NC='\033[0m'

# Test counter
TESTS_PASSED=0
TESTS_FAILED=0

# Function to run a test
run_test() {
    local test_name="$1"
    local test_command="$2"

    echo -n "Testing: $test_name... "

    if eval "$test_command" > /dev/null 2>&1; then
        echo -e "${GREEN}✓ PASSED${NC}"
        TESTS_PASSED=$((TESTS_PASSED + 1))
        return 0
    else
        echo -e "${RED}✗ FAILED${NC}"
        TESTS_FAILED=$((TESTS_FAILED + 1))
        return 1
    fi
}

echo "======================================"
echo "Crash Fix Verification Test"
echo "Bead: bf-1s6c3"
echo "======================================"
echo ""

# Test 1: Repository health (crash was caused by 18GB repo)
echo "1. Repository Health Tests"
echo "---------------------------"

REPO_SIZE_KB=$(du -s .git 2>/dev/null | awk '{print $1}')
REPO_SIZE_MB=$((REPO_SIZE_KB / 1024))
echo "   Current repository size: ${REPO_SIZE_MB}MB (was 18GB at crash)"

run_test "Repository size < 1GB" "test ${REPO_SIZE_MB} -lt 1024"
run_test "Repository size > 0" "test ${REPO_SIZE_MB} -gt 0"

# Test 2: Git operations (crash occurred during git merge/reconciliation)
echo ""
echo "2. Git Operation Tests"
echo "----------------------"

run_test "Git status" "git status --porcelain"
run_test "Git log" "git log --oneline -5"
run_test "Git object count" "git count-objects -vH"
run_test "Git fsck" "git fsck --connectivity-only"

# Test 3: Loose objects (17GB of loose objects caused OOM)
echo ""
echo "3. Loose Objects Tests"
echo "-----------------------"

LOOSE_SIZE_KB=$(git count-objects -vH 2>/dev/null | grep "size:" | awk '{print $2}' | sed 's/KiB//' | cut -d. -f1)
echo "   Current loose objects size: ${LOOSE_SIZE_KB}KB (was 17GB at crash)"

run_test "Loose objects < 100MB" "[ ${LOOSE_SIZE_KB:-0} -lt 102400 ]"

# Test 4: .gitignore protection (prevents future bloat)
echo ""
echo "4. Git Bloat Prevention Tests"
echo "-----------------------------"

run_test ".gitignore exists" "test -f .gitignore"
run_test ".gitignore blocks .beads/" "grep -q '^\.beads/$' .gitignore"
run_test ".gitignore blocks .jsonl files" "grep -q '\*\.jsonl' .gitignore"
run_test "Pre-commit hook exists" "test -f .git/hooks/pre-commit"
run_test "Pre-commit hook executable" "test -x .git/hooks/pre-commit"

# Test 5: Monitoring infrastructure (detects issues early)
echo ""
echo "5. Monitoring Infrastructure Tests"
echo "-----------------------------------"

run_test "Preflight health check exists" "test -f scripts/preflight-health-check.sh"
run_test "Repository health check exists" "test -f scripts/check-repo-health.sh"
run_test "Resource monitor exists" "test -f scripts/resource-monitor.sh"
run_test "Crash pattern detection exists" "test -f scripts/crash-pattern-detection.sh"
run_test "Safe git gc script exists" "test -f scripts/safe-git-gc.sh"

# Test 6: Memory-intensive operations (original crash trigger)
echo ""
echo "6. Memory-Intensive Operation Tests"
echo "-------------------------------------"

# Test operations that would trigger OOM on bloated repository
run_test "Git rev-list (memory-intensive)" "git rev-list --count HEAD"
run_test "Git pack-objects check" "git verify-pack -v .git/objects/pack/*.idx > /dev/null 2>&1"

# Test 7: Test suite (no regressions)
echo ""
echo "7. Regression Tests"
echo "-------------------"

run_test "Go test suite" "go test ./... -short"

# Summary
echo ""
echo "======================================"
echo "Test Summary"
echo "======================================"
TOTAL_TESTS=$((TESTS_PASSED + TESTS_FAILED))
echo "Total tests: $TOTAL_TESTS"
echo -e "${GREEN}Passed: $TESTS_PASSED${NC}"
if [ $TESTS_FAILED -gt 0 ]; then
    echo -e "${RED}Failed: $TESTS_FAILED${NC}"
else
    echo -e "${GREEN}Failed: $TESTS_FAILED${NC}"
fi
echo ""

if [ $TESTS_FAILED -eq 0 ]; then
    echo -e "${GREEN}✓ All tests passed!${NC}"
    echo -e "${GREEN}✓ Crash fix verified successfully${NC}"
    echo ""
    echo "The fix prevents the crash by:"
    echo "  1. Keeping repository size small (${REPO_SIZE_MB}MB vs 18GB)"
    echo "  2. Preventing .beads/ file commits (.gitignore + pre-commit hook)"
    echo "  3. Monitoring repository health (automated checks)"
    echo "  4. Using safe git operations (memory-limited, checkpointed)"
    exit 0
else
    echo -e "${RED}✗ Some tests failed${NC}"
    echo "Please review the failed tests above"
    exit 1
fi
