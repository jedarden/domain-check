#!/usr/bin/env bash
# Test Alert Cooldown (scripts/alert-cooldown.sh)
#
# Functional test of the global crash-alert cooldown: runs the module AS A
# SUBPROCESS against a sandboxed state file, simulating the rapid crash
# sequence that a SIGHUP cascade / infrastructure event produces — one alert
# opening a 5-minute window, further crashes arriving inside it (logged, not
# alerted), and the first crash after expiry carrying the suppressed-crash
# summary. Time travel is done by backdating last_alert_epoch in the sandbox
# state (the same technique as the bf-6d3d6 cascade replay of domchk-81938e89,
# which proved the manager's per-classification cooldown is a window, not a
# wall).
#
# The live state file .beads/logs/crash-alert-metadata.json is NEVER touched:
# every test runs with ALERT_COOLDOWN_METADATA_FILE inside a mktemp sandbox.
#
# Usage: ./scripts/test-alert-cooldown.sh   (exit 0 = all pass, 1 = failure)
# Created: 2026-09-07 (domchk-7b404946)

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MODULE="$SCRIPT_DIR/alert-cooldown.sh"

GREEN='\033[0;32m'; RED='\033[0;31m'; NC='\033[0m'
pass_count=0
fail_count=0
test_count=0

SANDBOX="$(mktemp -d)"
STATE_FILE="$SANDBOX/crash-alert-metadata.json"
export ALERT_COOLDOWN_METADATA_FILE="$STATE_FILE"

cleanup() { rm -rf "$SANDBOX"; }
trap cleanup EXIT

# --- helpers -----------------------------------------------------------------

pass() { echo -e "  ${GREEN}✓ PASS${NC} - $1"; pass_count=$((pass_count + 1)); }
fail() { echo -e "  ${RED}✗ FAIL${NC} - $1"; fail_count=$((fail_count + 1)); }

check() { # $1 = description, $2 = command result (0 = true)
    test_count=$((test_count + 1))
    if [[ "$2" -eq 0 ]]; then pass "$1"; else fail "$1"; fi
}

# Run a module command; capture rc + stdout in CHECK_RC / CHECK_OUT.
run_mod() {
    CHECK_OUT=$("$MODULE" "$@" 2>/dev/null)
    CHECK_RC=$?
}

# Backdate the sandbox window's start by $1 seconds (time travel).
age_window() {
    local secs="$1"
    local tmp="$STATE_FILE.age.$$"
    jq --argjson s "$secs" '.last_alert_epoch = (.last_alert_epoch - $s)' \
        "$STATE_FILE" > "$tmp" && mv "$tmp" "$STATE_FILE"
}

fresh_state() { rm -f "$STATE_FILE"; }

assert_eq() { # $1 = desc, $2 = actual, $3 = expected
    test_count=$((test_count + 1))
    if [[ "$2" == "$3" ]]; then pass "$1"; else fail "$1 (expected '$3', got '$2')"; fi
}

assert_contains() { # $1 = desc, $2 = haystack, $3 = needle
    test_count=$((test_count + 1))
    if grep -q "$3" <<<"$2"; then pass "$1"; else fail "$1 (missing '$3')"; fi
}

assert_not_contains() { # $1 = desc, $2 = haystack, $3 = needle
    test_count=$((test_count + 1))
    if grep -q "$3" <<<"$2"; then fail "$1 (unexpected '$3')"; else pass "$1"; fi
}

state_val() { jq -r "$1" "$STATE_FILE" 2>/dev/null; }

echo "=========================================="
echo "Testing Alert Cooldown (scripts/alert-cooldown.sh)"
echo "Sandbox: $SANDBOX"
echo "=========================================="
echo ""

# --- Test 1: module present, executable, syntactically valid -----------------
echo "Test 1: module exists, is executable, and parses"
test_count=$((test_count + 1))
if [[ -x "$MODULE" ]] && bash -n "$MODULE" 2>/dev/null; then
    pass "alert-cooldown.sh executable with valid syntax"
else
    fail "alert-cooldown.sh missing, non-executable, or syntax error"
fi
echo ""

# --- Test 2: no state file = allowed (fail-open on absence) ------------------
echo "Test 2: check with no state file allows the alert"
fresh_state
run_mod check
check "exit 0 with no state file (fail-open)" "$CHECK_RC"
assert_contains "stdout is COOLDOWN_INACTIVE" "$CHECK_OUT" "COOLDOWN_INACTIVE"
echo ""

# --- Test 3: record-alert creates the metadata file --------------------------
echo "Test 3: record-alert writes last_alert_timestamp tracking"
fresh_state
run_mod record-alert domchk-cool0001 INFRASTRUCTURE
check "record-alert exit 0" "$CHECK_RC"
check "state file created" "$([[ -f "$STATE_FILE" ]] && echo 0 || echo 1)"
assert_contains "last_alert_timestamp recorded (ISO-8601 UTC)" \
    "$(state_val .last_alert_timestamp)" "T..:..:..Z"
check "last_alert_epoch recorded and positive" \
    "$([[ "$(state_val .last_alert_epoch)" -gt 0 ]] && echo 0 || echo 1)"
assert_eq "last_alert_bead recorded" "$(state_val .last_alert_bead)" "domchk-cool0001"
assert_eq "last_alert_classification recorded" "$(state_val .last_alert_classification)" "INFRASTRUCTURE"
assert_eq "cooldown_active is true" "$(state_val .cooldown_active)" "true"
assert_eq "cooldown_seconds captured at 300" "$(state_val .cooldown_seconds)" "300"
echo ""

# --- Test 4: inside the window the alert is suppressed -----------------------
echo "Test 4: check inside the window suppresses (exit 3)"
run_mod check
check "exit 3 while window is open" "$([[ "$CHECK_RC" -eq 3 ]] && echo 0 || echo 1)"
assert_contains "stdout is COOLDOWN_ACTIVE" "$CHECK_OUT" "COOLDOWN_ACTIVE"
assert_contains "remaining seconds reported" "$CHECK_OUT" "remaining="
echo ""

# --- Test 5: simulated rapid crash sequence (5 crashes in-window) ------------
echo "Test 5: rapid crash sequence — 5 crashes during the cooldown are logged, not alerted"
for i in 2 3 4 5 6; do
    bead="domchk-cool000$i"
    run_mod record-suppressed "$bead" INFRASTRUCTURE "kill during cascade"
    if [[ "$CHECK_RC" -ne 0 ]]; then fail "record-suppressed $bead exit 0"; fi
done
pass "5 suppressed crashes recorded (beads domchk-cool0002..0006)"
test_count=$((test_count + 1))
assert_eq "suppressed_count is 5" "$(state_val .suppressed_count)" "5"
assert_eq "summary_pending is true" "$(state_val .summary_pending)" "true"
assert_eq "lifetime suppressed_total is 5" "$(state_val .suppressed_total)" "5"
SUPPRESSED_LOG_LINES=$(grep -c "suppressed by active cooldown" "$SANDBOX/alert-cooldown.log" 2>/dev/null || echo 0)
assert_eq "each suppressed crash logged to the audit log" "$SUPPRESSED_LOG_LINES" "5"
RECENT_LEN=$(jq '.suppressed_recent | length' "$STATE_FILE" 2>/dev/null || echo 0)
assert_eq "suppressed_recent ledger holds all 5" "$RECENT_LEN" "5"
echo ""

# --- Test 6: the cooldown is a window, not a wall ----------------------------
echo "Test 6: cooldown expires after the window (299s elapsed still holds)"
age_window 299
run_mod check
check "exit 3 at 299s elapsed (1s left)" "$([[ "$CHECK_RC" -eq 3 ]] && echo 0 || echo 1)"
echo ""

# --- Test 7: expiry emits the suppressed-crash summary once ------------------
echo "Test 7: first crash after expiry carries the summary of suppressed crashes"
age_window 2   # now 301s elapsed
run_mod check
check "exit 0 once the window has expired" "$CHECK_RC"
assert_contains "summary header present" "$CHECK_OUT" "COOLDOWN_EXPIRED"
assert_contains "suppressed count in header" "$CHECK_OUT" "suppressed=5"
assert_contains "SUMMARY line present" "$CHECK_OUT" "SUMMARY: 5 crash(es) suppressed"
assert_contains "suppressed bead 0002 named" "$CHECK_OUT" "domchk-cool0002"
assert_contains "suppressed bead 0006 named" "$CHECK_OUT" "domchk-cool0006"
assert_contains "originating alert named" "$CHECK_OUT" "domchk-cool0001"
check "state accounting cleared after consumption" \
    "$([[ "$(state_val .summary_pending)" == "false" && "$(state_val .suppressed_count)" == "0" ]] && echo 0 || echo 1)"
assert_eq "lifetime suppressed_total survives consumption" "$(state_val .suppressed_total)" "5"

run_mod check
check "second check after expiry: exit 0" "$CHECK_RC"
assert_not_contains "summary emitted exactly once" "$CHECK_OUT" "COOLDOWN_EXPIRED"
echo ""

# --- Test 8: global window — a different classification cannot escape --------
echo "Test 8: cooldown is global, not per-classification"
fresh_state
"$MODULE" record-alert domchk-cool1000 INFRASTRUCTURE >/dev/null 2>&1
run_mod record-suppressed domchk-cool1001 SERVICE_FAILURE "different class, same cascade"
check "record-suppressed exit 0" "$CHECK_RC"
run_mod check
test_count=$((test_count + 1))
if [[ "$CHECK_RC" -eq 3 ]]; then
    pass "SERVICE_FAILURE crash still suppressed by an INFRASTRUCTURE alert's window (exit 3)"
else
    fail "SERVICE_FAILURE crash not suppressed (expected exit 3, got $CHECK_RC)"
fi
echo ""

# --- Test 9: expiry with NO suppressed crashes emits no summary --------------
echo "Test 9: cooldown expiring with no crashes is silent"
fresh_state
"$MODULE" record-alert domchk-cool2000 CODE_DEFECT >/dev/null 2>&1
age_window 400
run_mod check
check "exit 0 after quiet expiry" "$CHECK_RC"
assert_not_contains "no summary for a quiet window" "$CHECK_OUT" "COOLDOWN_EXPIRED"
assert_not_contains "no SUMMARY line for a quiet window" "$CHECK_OUT" "SUMMARY:"
echo ""

# --- Test 10: corrupt state fails open ---------------------------------------
echo "Test 10: corrupt state file fails open (alerting never depends on this gate)"
echo 'NOT JSON {{{' > "$STATE_FILE"
run_mod check
check "exit 0 on corrupt state" "$CHECK_RC"
assert_contains "treated as inactive" "$CHECK_OUT" "COOLDOWN_INACTIVE"
echo ""

# --- Test 11: inert state (no epoch) is inactive ------------------------------
echo "Test 11: state without a window epoch is inert"
fresh_state
jq -n '{version: 1, cooldown_seconds: 300, last_alert_epoch: 0, suppressed_count: 0, summary_pending: false}' > "$STATE_FILE"
run_mod check
check "exit 0 with last_alert_epoch=0" "$CHECK_RC"
echo ""

# --- Test 12: replacing an unsummarized window warns in the audit log --------
echo "Test 12: a window replaced before its summary is issued is flagged, not dropped"
fresh_state
"$MODULE" record-alert domchk-cool3000 INFRASTRUCTURE >/dev/null 2>&1
"$MODULE" record-suppressed domchk-cool3001 INFRASTRUCTURE "in-window kill" >/dev/null 2>&1
"$MODULE" record-alert domchk-cool3002 INFRASTRUCTURE >/dev/null 2>&1
WARN_LINES=$(grep -c "unsummarized suppressed crash" "$SANDBOX/alert-cooldown.log" 2>/dev/null || echo 0)
check "audit log carries the unsummarized-window warning" "$([[ "$WARN_LINES" -ge 1 ]] && echo 0 || echo 1)"
run_mod check
check "new window still suppresses" "$([[ "$CHECK_RC" -eq 3 ]] && echo 0 || echo 1)"
echo ""

# --- Test 13: end-to-end surge replay (the acceptance scenario) --------------
echo "Test 13: full surge replay — 6 crashes in one minute produce 1 alert + 5 suppressions, then 1 summary alert"
fresh_state
ALERTS=0; SUPPRESSED_N=0; SUMMARY_ALERTS=0
# Surge: six crash traces land within one minute (the 2026-08-16 cascade shape).
surge_crash() { # $1 = bead
    run_mod check
    case "$CHECK_RC" in
        0)
            if grep -q "COOLDOWN_EXPIRED" <<<"$CHECK_OUT"; then
                SUMMARY_ALERTS=$((SUMMARY_ALERTS + 1))
            fi
            ALERTS=$((ALERTS + 1))
            "$MODULE" record-alert "$1" INFRASTRUCTURE >/dev/null 2>&1
            ;;
        3)
            SUPPRESSED_N=$((SUPPRESSED_N + 1))
            "$MODULE" record-suppressed "$1" INFRASTRUCTURE "cascade kill" >/dev/null 2>&1
            ;;
        *) echo "  unexpected check rc=$CHECK_RC for $1" ;;
    esac
}
for i in 1 2 3 4 5 6; do surge_crash "domchk-surge000$i"; done
assert_eq "exactly ONE alert during the surge" "$ALERTS" "1"
assert_eq "five crashes suppressed during the surge" "$SUPPRESSED_N" "5"
# The cascade ends; the next crash arrives after the window has closed.
age_window 360
surge_crash "domchk-surge0007"
assert_eq "post-surge crash alerts" "$ALERTS" "2"
assert_eq "post-surge alert carries the suppressed summary" "$SUMMARY_ALERTS" "1"
echo ""

# --- Test 14: state stays valid JSON through the whole sequence --------------
echo "Test 14: state file integrity"
test_count=$((test_count + 1))
if jq -e . "$STATE_FILE" >/dev/null 2>&1; then
    pass "state file is valid JSON after the full sequence"
else
    fail "state file is not valid JSON after the full sequence"
fi
STATUS_OUT=$("$MODULE" status 2>/dev/null)
check "status command prints the state" "$([[ "$(jq -r .last_alert_bead <<<"$STATUS_OUT" 2>/dev/null)" != "null" ]] && [[ -n "$(jq -r .last_alert_bead <<<"$STATUS_OUT" 2>/dev/null)" ]] && echo 0 || echo 1)"
"$MODULE" reset >/dev/null 2>&1
check "reset clears state (next check allowed)" \
    "$([[ ! -f "$STATE_FILE" ]] && echo 0 || echo 1)"
run_mod check
check "check after reset: exit 0" "$CHECK_RC"
echo ""

# --- Test 15: usage errors ----------------------------------------------------
echo "Test 15: usage errors exit 2"
"$MODULE" >/dev/null 2>&1; check "no arguments" "$([[ $? -eq 2 ]] && echo 0 || echo 1)"
"$MODULE" bogus-cmd >/dev/null 2>&1; check "unknown command" "$([[ $? -eq 2 ]] && echo 0 || echo 1)"
"$MODULE" record-alert >/dev/null 2>&1; check "record-alert without a bead id" "$([[ $? -eq 2 ]] && echo 0 || echo 1)"
"$MODULE" record-suppressed >/dev/null 2>&1; check "record-suppressed without a bead id" "$([[ $? -eq 2 ]] && echo 0 || echo 1)"
echo ""

# --- Summary ------------------------------------------------------------------
echo "=========================================="
echo "Results: $pass_count passed, $fail_count failed, of $test_count checks"
echo "=========================================="

if [[ "$fail_count" -gt 0 ]]; then
    exit 1
fi
exit 0
