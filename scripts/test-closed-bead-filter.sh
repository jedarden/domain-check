#!/usr/bin/env bash
# Functional test for the closed-bead alert filter (crash-alert-manager.sh
# CRITICAL FIX 1 / FIX 5).
#
# scripts/test-crash-alert-fixes.sh checks that the FIX 1 / FIX 5 markers are
# present in the source; this script exercises the filter end-to-end: it runs
# the real manager against a fabricated trace for bf-2vtzg — the resolved
# crash whose duplicate-alert storm (bf-5o8ey, bf-xg2gg, bf-39xem, ...) the
# filter exists to stop — and asserts that no alert is generated.
#
# The sandbox isolates all writes: PROJECT_ROOT is derived from the copied
# script's own path, so the fabricated trace and the alert log/state files
# land under a temp directory, never in the live .beads/traces.
#
# crash-resolution-tracker.sh is deliberately NOT copied into the sandbox. In
# the live deployment that tracker answers first (bf-2vtzg resolves as
# "bead_closure"), so FIX 1 is the second gate; dropping the tracker is what
# lets this test prove the closed-bead filter itself fires.
#
# Usage: scripts/test-closed-bead-filter.sh
# Exit codes: 0 all assertions pass, 1 at least one failed.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CRASH_ALERT_MANAGER="$SCRIPT_DIR/crash-alert-manager.sh"

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

pass_count=0
fail_count=0
test_count=0

pass() {
    echo -e "${GREEN}✓ PASS${NC} - $1"
    pass_count=$((pass_count + 1))
    test_count=$((test_count + 1))
}

fail() {
    echo -e "${RED}✗ FAIL${NC} - $1"
    fail_count=$((fail_count + 1))
    test_count=$((test_count + 1))
}

# Target bead: the closed crash the filter is named for. If it is ever
# reopened or deleted the precondition below fails — that is a broken test
# premise, not a filter regression, and the message says so.
TARGET_BEAD="bf-2vtzg"

echo "=========================================="
echo "Closed Bead Filter Functional Test"
echo "=========================================="
echo ""

# Sandbox with the manager's expected layout: <root>/scripts/<script>,
# <root>/.beads/traces/<bead>/.
SANDBOX="$(mktemp -d "${TMPDIR:-/tmp}/closed-bead-filter-XXXXXX")"
trap 'rm -rf "$SANDBOX"' EXIT
mkdir -p "$SANDBOX/scripts" "$SANDBOX/.beads/traces/$TARGET_BEAD"

for script in crash-alert-manager.sh crash-classifier.sh alert-deduplication.sh; do
    cp "$SCRIPT_DIR/$script" "$SANDBOX/scripts/$script"
    chmod +x "$SANDBOX/scripts/$script"
done

printf 'synthetic trace for closed-bead filter test\n' \
    > "$SANDBOX/.beads/traces/$TARGET_BEAD/trace.jsonl"
# exit_code -1 matches bf-2vtzg's real crash signature; a 0 would trip
# FIX 4 (completion awareness) before the closure check could be exercised.
printf '{"exit_code":-1}\n' \
    > "$SANDBOX/.beads/traces/$TARGET_BEAD/metadata.json"

# Precondition: the target bead must actually be closed, and via the same
# `bead show` parsing the manager itself uses (FIX 1).
if ! command -v bead > /dev/null 2>&1; then
    fail "bead CLI not on PATH - cannot verify $TARGET_BEAD status"
    echo "Total tests: $test_count, Passed: $pass_count, Failed: $fail_count"
    exit 1
fi

BEAD_STATUS="$(bead show "$TARGET_BEAD" 2>/dev/null | grep -i "^Status:" | head -1 || echo "unknown")"
if [[ "$BEAD_STATUS" =~ [Cc]losed ]]; then
    pass "$TARGET_BEAD is CLOSED per live bead store (precondition)"
else
    fail "$TARGET_BEAD status is '$BEAD_STATUS', expected Closed - test premise broken (bead reopened or deleted?), not a filter result"
fi

if [[ -x "$CRASH_ALERT_MANAGER" ]]; then
    pass "crash-alert-manager.sh exists and is executable"
else
    fail "crash-alert-manager.sh not found or not executable"
fi

# Run the manager from the sandbox. Exit codes are documented in its usage:
# 0 = no alert needed, 1 = alert generated.
MANAGER_OUTPUT="$(bash "$SANDBOX/scripts/crash-alert-manager.sh" "$TARGET_BEAD" 2>&1)"
MANAGER_EXIT=$?
MANAGER_LOG="$SANDBOX/.beads/logs/crash-alert-manager.log"

if [[ "$MANAGER_EXIT" -eq 0 ]]; then
    pass "exit 0 - no alert needed (alert path would exit 1)"
else
    fail "exit $MANAGER_EXIT, expected 0 (1 = alert generated)"
    echo "$MANAGER_OUTPUT"
fi

if grep -q "is already CLOSED - no alert needed" <<< "$MANAGER_OUTPUT"; then
    pass "FIX 1 closed-bead gate fired (not some other skip path)"
else
    fail "closed-bead gate message absent from output"
    echo "$MANAGER_OUTPUT"
fi

if grep -q "Reason: Bead already closed" <<< "$MANAGER_OUTPUT"; then
    pass "reason reported: task already complete, alert skipped"
else
    fail "'Reason: Bead already closed' absent from output"
fi

# Negative assertions: nothing alert-shaped was produced in the sandbox.
if [[ -f "$MANAGER_LOG" ]] && grep -q "ALERT" "$MANAGER_LOG"; then
    fail "alert log contains ALERT-level entries"
    grep "ALERT" "$MANAGER_LOG"
else
    pass "no ALERT-level entry in sandbox alert log"
fi

PROCESSED_FILE="$SANDBOX/.beads/logs/processed-alerts.txt"
if [[ -f "$PROCESSED_FILE" ]] && grep -q "$TARGET_BEAD" "$PROCESSED_FILE"; then
    fail "sandbox processed-alerts.txt recorded an alert for $TARGET_BEAD"
else
    pass "no alert recorded in sandbox processed-alerts.txt"
fi

rm -rf "$SANDBOX"

echo ""
echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo "Total tests: $test_count"
echo -e "${GREEN}Passed: $pass_count${NC}"
echo -e "${RED}Failed: $fail_count${NC}"
echo ""

if [[ "$fail_count" -eq 0 ]]; then
    echo -e "${GREEN}All tests passed!${NC}"
    echo "Closed bead $TARGET_BEAD did not trigger a new alert."
    exit 0
fi
echo -e "${RED}Some tests failed!${NC}"
exit 1
