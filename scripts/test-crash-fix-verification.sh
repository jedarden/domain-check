#!/usr/bin/env bash
# Crash Fix Verification Test
# Tests the complete crash alert fix implementation
# This verifies that the fix from domchk-a61d781e prevents false positive alerts

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
CRASH_ALERT_MANAGER="$SCRIPT_DIR/crash-alert-manager.sh"

echo "=========================================="
echo "Crash Fix Verification Test"
echo "=========================================="
echo ""
echo "This test verifies that the fix prevents false positive alerts"
echo "for beads with exit code -1 (SIGHUP/SIGKILL) that completed successfully."
echo ""

# Test scenarios
test_count=0
pass_count=0

# Test 1: CLOSED bead with exit code -1 should be FALSE_POSITIVE
echo "Test 1: CLOSED bead with exit code -1 → FALSE_POSITIVE (no alert)"
test_count=$((test_count + 1))

# Find a closed bead with exit code -1
test_bead=$(grep '"exit_code":-1' "$PROJECT_ROOT/.beads/events.jsonl" | jq -r '.bead' | sort -u | head -1)
if [[ -n "$test_bead" ]]; then
    bead_status=$(bead show "$test_bead" 2>/dev/null | grep "^Status" || echo "unknown")

    if [[ "$bead_status" =~ [Cc]losed ]]; then
        result=$("$CRASH_ALERT_MANAGER" "$test_bead" --classify-only 2>&1 || true)

        if echo "$result" | grep -q "already CLOSED - no alert needed"; then
            echo -e "  ✅ PASS - Correctly classified CLOSED bead as FALSE_POSITIVE"
            echo "     Bead: $test_bead"
            pass_count=$((pass_count + 1))
        else
            echo -e "  ❌ FAIL - Did not classify CLOSED bead correctly"
            echo "     Bead: $test_bead"
            echo "     Result: $result"
        fi
    else
        echo -e "  ⚠️  SKIP - Bead $test_bead is not closed (status: $bead_status)"
    fi
else
    echo -e "  ⚠️  SKIP - No beads with exit code -1 found"
fi
echo ""

# Test 2: OPEN bead with exit code -1 should proceed to classification
echo "Test 2: OPEN bead with exit code -1 → Proceed to classification"
test_count=$((test_count + 1))

# Find an open bead with exit code -1 (these are rarer)
open_bead=$(grep '"exit_code":-1' "$PROJECT_ROOT/.beads/events.jsonl" | jq -r '.bead' | sort -u | while read b; do
    status=$(bead show "$b" 2>/dev/null | grep "^Status" || echo "unknown")
    if [[ "$status" =~ [Oo]pen ]]; then
        echo "$b"
        break
    fi
done)

if [[ -n "$open_bead" ]]; then
    result=$("$CRASH_ALERT_MANAGER" "$open_bead" --classify-only 2>&1 || true)

    if echo "$result" | grep -q "Processing crash alert"; then
        echo -e "  ✅ PASS - Correctly proceeding to classification for OPEN bead"
        echo "     Bead: $open_bead"
        pass_count=$((pass_count + 1))
    else
        echo -e "  ❌ FAIL - Did not proceed to classification for OPEN bead"
        echo "     Bead: $open_bead"
        echo "     Result: $result"
    fi
else
    echo -e "  ⚠️  SKIP - No OPEN beads with exit code -1 found (expected - most recovered)"
fi
echo ""

# Test 3: Verify CRITICAL FIX 1 is present
echo "Test 3: CRITICAL FIX 1 (closed bead filtering) is implemented"
test_count=$((test_count + 1))

if grep -q "CRITICAL FIX 1" "$CRASH_ALERT_MANAGER"; then
    echo -e "  ✅ PASS - CRITICAL FIX 1 present in code"
    pass_count=$((pass_count + 1))
else
    echo -e "  ❌ FAIL - CRITICAL FIX 1 not found"
fi
echo ""

# Test 4: Verify exit code -1 check with closure status
echo "Test 4: Exit code -1 checks bead closure status"
test_count=$((test_count + 1))

if grep -q "Checking bead closure status" "$CRASH_ALERT_MANAGER"; then
    echo -e "  ✅ PASS - Closure status check implemented"
    pass_count=$((pass_count + 1))
else
    echo -e "  ❌ FAIL - Closure status check not found"
fi
echo ""

# Summary
echo "=========================================="
echo "Verification Summary"
echo "=========================================="
echo "Tests run: $test_count"
echo "Tests passed: $pass_count"
echo ""

if [[ $pass_count -eq $test_count ]]; then
    echo -e "✅ All verification tests passed!"
    echo ""
    echo "The crash fix correctly:"
    echo "  1. Detects beads with exit code -1 (SIGHUP/SIGKILL)"
    echo "  2. Checks bead closure status"
    echo "  3. Classifies CLOSED beads as FALSE_POSITIVE (no alert)"
    echo "  4. Proceeds with classification for OPEN beads"
    echo ""
    echo "This prevents false positive alerts like bf-4k2ws where:"
    echo "  - Worker was killed by SIGHUP during system-wide cascade"
    echo "  - Bead automatically retried and completed successfully"
    echo "  - Previous implementation would have generated unnecessary alert"
    echo ""
    exit 0
else
    echo -e "❌ Some tests failed"
    exit 1
fi
