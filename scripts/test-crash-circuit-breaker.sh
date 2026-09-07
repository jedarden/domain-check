#!/usr/bin/env bash
# Test Crash Circuit Breaker
# Verifies the per-bead consecutive-crash breaker that stops crash-storm
# retry loops (bf-65lsdu: 127 identical doomed dispatches in 2.5 hours).
# Created: 2026-09-02

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BREAKER="$SCRIPT_DIR/crash-circuit-breaker.sh"

echo "=========================================="
echo "Testing Crash Circuit Breaker"
echo "=========================================="
echo ""

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

test_count=0
pass_count=0
fail_count=0

# Isolated state for every test
TEST_TMP="$(mktemp -d)"
STATE_FILE="$TEST_TMP/breaker-state.json"

# Clean up the sandbox unless asked to keep it for debugging
cleanup() {
    if [[ "${BREAKER_TEST_KEEP:-0}" == "1" ]]; then
        echo "sandbox kept: $TEST_TMP"
    else
        rm -rf "$TEST_TMP"
    fi
}
trap cleanup EXIT

pass() {
    test_count=$((test_count + 1))
    pass_count=$((pass_count + 1))
    echo -e "${GREEN}✓ PASS${NC} - $1"
    echo ""
}

fail() {
    test_count=$((test_count + 1))
    fail_count=$((fail_count + 1))
    echo -e "${RED}✗ FAIL${NC} - $1"
    echo ""
}

# Run a breaker command against the isolated state; capture output and rc
brk() {
    BREAKER_STATE_FILE="$STATE_FILE" "$BREAKER" "$@" 2>&1
}

fresh_state() {
    rm -f "$STATE_FILE"
}

# Test 1: breaker script exists and is executable
if [[ -x "$BREAKER" ]]; then
    pass "crash-circuit-breaker.sh exists and is executable"
else
    fail "crash-circuit-breaker.sh not found or not executable"
fi

# Test 2: fresh bead is allowed
fresh_state
OUT=$(brk check bf-nocrash)
RC=$?
if [[ $RC -eq 0 ]] && [[ "$OUT" == *"ALLOWED"* ]]; then
    pass "fresh bead dispatch allowed (exit 0)"
else
    fail "fresh bead should be allowed (exit 0), got rc=$RC: $OUT"
fi

# Test 3: crashes below threshold stay allowed
fresh_state
brk record bf-thresh -1 >/dev/null
brk record bf-thresh -1 >/dev/null
OUT=$(brk check bf-thresh)
RC=$?
if [[ $RC -eq 0 ]] && [[ "$OUT" == *"ALLOWED"* ]]; then
    pass "2 consecutive crashes below threshold (3) still allowed"
else
    fail "below-threshold crashes should stay allowed, got rc=$RC: $OUT"
fi

# Test 4: threshold consecutive crashes trip the breaker and block dispatch
fresh_state
for i in 1 2 3; do brk record bf-trip -1 >/dev/null; done
OUT=$(brk check bf-trip)
RC=$?
if [[ $RC -eq 4 ]] && [[ "$OUT" == *"BLOCKED"* ]]; then
    pass "3 consecutive crashes trip breaker; check blocks with exit 4"
else
    fail "tripped breaker should block with exit 4, got rc=$RC: $OUT"
fi

# Test 5: a success resets the breaker
brk record bf-trip 0 >/dev/null
OUT=$(brk check bf-trip)
RC=$?
if [[ $RC -eq 0 ]] && [[ "$OUT" == *"ALLOWED"* ]]; then
    pass "success (exit 0) closes a tripped breaker"
else
    fail "success should close the breaker, got rc=$RC: $OUT"
fi

# Test 6: non-infrastructure exit codes do not advance the counter
fresh_state
brk record bf-workflow 124 >/dev/null   # timeout (RCA §3: separate code)
brk record bf-workflow 1 >/dev/null     # workflow failure
brk record bf-workflow 124 >/dev/null
OUT=$(brk check bf-workflow)
RC=$?
STATE_CHECK=$(jq -r '.beads["bf-workflow"] // "absent"' "$STATE_FILE")
if [[ $RC -eq 0 ]] && [[ "$STATE_CHECK" == "absent" ]]; then
    pass "timeout (124) and workflow (1) exits do not count as breaker crashes"
else
    fail "non-crash codes should be ignored, got rc=$RC state=$STATE_CHECK"
fi

# Test 7: cooldown elapsed -> half-open probe is allowed
fresh_state
for i in 1 2 3; do brk record bf-half -1 >/dev/null; done
PAST=$(date -u -d '1 hour ago' +"%Y-%m-%dT%H:%M:%SZ")
jq --arg ts "$PAST" '.beads["bf-half"].retry_after = $ts' "$STATE_FILE" > "$STATE_FILE.tmp" \
    && mv "$STATE_FILE.tmp" "$STATE_FILE"
OUT=$(brk check bf-half)
RC=$?
if [[ $RC -eq 0 ]] && [[ "$OUT" == *"half-open"* ]]; then
    pass "elapsed cooldown allows a half-open probe"
else
    fail "elapsed cooldown should allow probe (exit 0), got rc=$RC: $OUT"
fi

# Test 8: a crashing probe re-trips with doubled backoff
brk record bf-half -1 >/dev/null
RETRY=$(jq -r '.beads["bf-half"].retry_after' "$STATE_FILE")
OPEN_COUNT=$(jq -r '.beads["bf-half"].open_count' "$STATE_FILE")
STATE_OUT=$(brk check bf-half >/dev/null 2>&1; echo $?)
if [[ "$OPEN_COUNT" == "2" ]] && [[ $STATE_OUT -eq 4 ]]; then
    RETRY_EPOCH=$(date -d "$RETRY" +%s)
    NOW_EPOCH=$(date +%s)
    DIFF=$((RETRY_EPOCH - NOW_EPOCH))
    if [[ $DIFF -gt 1800 && $DIFF -le 3600 ]]; then
        pass "crashing probe re-trips breaker with doubled backoff (~3600s, trip #2)"
    else
        fail "re-trip backoff should be ~3600s, got ${DIFF}s"
    fi
else
    fail "crashing probe should re-trip (open_count=2, check=4), got open_count=$OPEN_COUNT rc=$STATE_OUT"
fi

# Test 9: backoff is capped at BREAKER_MAX_COOLDOWN
fresh_state
for trip in 1 2 3 4 5; do
    for i in 1 2 3; do brk record bf-cap -1 >/dev/null 2>&1; done
    PAST=$(date -u -d '1 hour ago' +"%Y-%m-%dT%H:%M:%SZ")
    jq --arg ts "$PAST" '.beads["bf-cap"].retry_after = $ts' "$STATE_FILE" > "$STATE_FILE.tmp" \
        && mv "$STATE_FILE.tmp" "$STATE_FILE"
    brk record bf-cap -1 >/dev/null 2>&1
done
RETRY=$(jq -r '.beads["bf-cap"].retry_after' "$STATE_FILE")
RETRY_EPOCH=$(date -d "$RETRY" +%s)
NOW_EPOCH=$(date +%s)
DIFF=$((RETRY_EPOCH - NOW_EPOCH))
if [[ $DIFF -le 14400 && $DIFF -gt 0 ]]; then
    pass "backoff capped at BREAKER_MAX_COOLDOWN (14400s), got ~${DIFF}s"
else
    fail "backoff should cap at 14400s, got ~${DIFF}s"
fi

# Test 10: defer on a non-open breaker is a no-op
fresh_state
OUT=$(brk defer bf-notopen)
RC=$?
if [[ $RC -eq 0 ]] && [[ "$OUT" == *"NOT OPEN"* ]]; then
    pass "defer on closed breaker is a no-op (exit 0)"
else
    fail "defer on closed breaker should no-op, got rc=$RC: $OUT"
fi

# Test 11: defer on an open bead calls 'bead update --status deferred'
fresh_state
MOCK_BIN="$TEST_TMP/mockbin"
mkdir -p "$MOCK_BIN"
cat > "$MOCK_BIN/bead" <<'MOCK'
#!/usr/bin/env bash
echo "MOCK bead $@" >> "$MOCK_CALLS"
exit 0
MOCK
chmod +x "$MOCK_BIN/bead"
export MOCK_CALLS="$TEST_TMP/mock-calls.log"
export MOCK_BIN_PATH="$MOCK_BIN"
for i in 1 2 3; do PATH="$MOCK_BIN:$PATH" brk record bf-defer -1 >/dev/null; done
OUT=$(PATH="$MOCK_BIN:$PATH" brk defer bf-defer)
RC=$?
if [[ $RC -eq 0 ]] && grep -q "update bf-defer --status deferred" "$MOCK_CALLS"; then
    pass "defer on open breaker invokes 'bead update --status deferred'"
else
    fail "defer should call bead update --status deferred, got rc=$RC: $OUT"
fi

# Test 12: defer survives a bead CLI failure and keeps the breaker open
fresh_state
cat > "$MOCK_BIN/bead" <<'MOCK'
#!/usr/bin/env bash
exit 1
MOCK
chmod +x "$MOCK_BIN/bead"
for i in 1 2 3; do PATH="$MOCK_BIN:$PATH" brk record bf-deferfail -1 >/dev/null 2>&1; done
OUT=$(PATH="$MOCK_BIN:$PATH" brk defer bf-deferfail)
RC=$?
STILL_OPEN=$(PATH="$MOCK_BIN:$PATH" brk check bf-deferfail >/dev/null 2>&1; echo $?)
if [[ $RC -eq 2 ]] && [[ $STILL_OPEN -eq 4 ]]; then
    pass "failed deferral keeps breaker OPEN (check still blocks, exit 2 from defer)"
else
    fail "failed deferral should exit 2 and stay open, got rc=$RC check=$STILL_OPEN"
fi

# Test 13: reset clears one bead's state
OUT=$(brk reset bf-deferfail)
STILL_OPEN=$(brk check bf-deferfail >/dev/null 2>&1; echo $?)
if [[ $STILL_OPEN -eq 0 ]]; then
    pass "reset reopens dispatch for a tripped bead"
else
    fail "reset should clear the breaker, check rc=$STILL_OPEN"
fi

# Test 14: --rebuild trips breakers from crash history
fresh_state
EVENTS="$TEST_TMP/events.jsonl"
cat > "$EVENTS" <<'EOF'
{"bead":"bf-storm","event":"claim","ts":"2026-08-13T21:19:53Z","worker":"lab-domain-check"}
{"bead":"bf-storm","event":"crash","exit_code":-1,"ts":"2026-08-13T21:22:23Z","worker":"lab-domain-check"}
{"bead":"bf-storm","event":"crash","exit_code":-1,"ts":"2026-08-13T21:24:00Z","worker":"lab-domain-check"}
{"bead":"bf-other","event":"crash","exit_code":-1,"ts":"2026-08-13T21:25:00Z","worker":"lab-domain-check"}
{"bead":"bf-storm","event":"crash","exit_code":-1,"ts":"2026-08-13T21:26:00Z","worker":"lab-domain-check"}
{"bead":"bf-storm","event":"complete","exit_code":0,"ts":"2026-08-13T21:30:00Z","worker":"lab-domain-check"}
{"bead":"bf-storm","event":"crash","exit_code":-1,"ts":"2026-08-13T21:35:00Z","worker":"lab-domain-check"}
{"bead":"bf-timeout","event":"crash","exit_code":124,"ts":"2026-08-13T21:36:00Z","worker":"lab-domain-check"}
EOF
OUT=$(BEAD_EVENTS_FILE="$EVENTS" brk --rebuild)
RC=$?
STORM_BLOCKED=$(BREAKER_STATE_FILE="$STATE_FILE" "$BREAKER" check bf-storm >/dev/null 2>&1; echo $?)
STORM_COUNT=$(jq -r '.beads["bf-storm"].consecutive_crashes // "absent"' "$STATE_FILE")
# bf-storm had 3 consecutive, then success reset, then 1 more -> should NOT be tripped
# (rebuild resets counters on success). bf-other has 1. Neither reaches threshold.
if [[ "$STORM_COUNT" == "1" ]] && [[ $STORM_BLOCKED -eq 0 ]]; then
    pass "--rebuild counts consecutive crashes and respects success resets"
else
    fail "--rebuild state wrong: storm count=$STORM_COUNT check rc=$STORM_BLOCKED"
fi

# Test 15: --rebuild trips a bead that truly stormed
fresh_state
RECENT_1=$(date -u -d '3 minutes ago' +"%Y-%m-%dT%H:%M:%SZ")
RECENT_2=$(date -u -d '2 minutes ago' +"%Y-%m-%dT%H:%M:%SZ")
RECENT_3=$(date -u -d '1 minute ago' +"%Y-%m-%dT%H:%M:%SZ")
cat > "$EVENTS" <<EOF
{"bead":"bf-doomed","event":"crash","exit_code":-1,"ts":"$RECENT_1"}
{"bead":"bf-doomed","event":"crash","exit_code":-1,"ts":"$RECENT_2"}
{"bead":"bf-doomed","event":"crash","exit_code":-1,"ts":"$RECENT_3"}
EOF
OUT=$(BEAD_EVENTS_FILE="$EVENTS" brk --rebuild)
DOOMED_BLOCKED=$(BREAKER_STATE_FILE="$STATE_FILE" "$BREAKER" check bf-doomed >/dev/null 2>&1; echo $?)
DOOMED_STATE=$(jq -r '.beads["bf-doomed"].state // "absent"' "$STATE_FILE")
if [[ $DOOMED_BLOCKED -eq 4 ]] && [[ "$DOOMED_STATE" == "open" ]] && [[ "$OUT" == *"TRIPPED (rebuild): bf-doomed"* ]]; then
    pass "--rebuild trips breakers for genuinely stormed beads (recent storm stays blocked)"
else
    fail "--rebuild should trip bf-doomed (check rc=$DOOMED_BLOCKED state=$DOOMED_STATE): $OUT"
fi

# Test 15b: a historical storm rebuilds as open but probeable (backoff expired)
fresh_state
cat > "$EVENTS" <<'EOF'
{"bead":"bf-ancient","event":"crash","exit_code":-1,"ts":"2026-08-13T21:22:23Z"}
{"bead":"bf-ancient","event":"crash","exit_code":-1,"ts":"2026-08-13T21:24:00Z"}
{"bead":"bf-ancient","event":"crash","exit_code":-1,"ts":"2026-08-13T21:26:00Z"}
EOF
OUT=$(BEAD_EVENTS_FILE="$EVENTS" brk --rebuild)
ANCIENT_STATE=$(jq -r '.beads["bf-ancient"].state // "absent"' "$STATE_FILE")
ANCIENT_CHECK=$(BREAKER_STATE_FILE="$STATE_FILE" "$BREAKER" check bf-ancient 2>&1; echo "RC=$?")
if [[ "$ANCIENT_STATE" == "open" ]] && [[ "$ANCIENT_CHECK" == *"half-open"*"RC=0"* ]]; then
    pass "historical storm rebuilds open but allows half-open probe (backoff expired)"
else
    fail "historical storm should be open+probeable, got state=$ANCIENT_STATE check=$ANCIENT_CHECK"
fi

# Test 16: stale crash counters decay instead of tripping unrelated new work
fresh_state
OLD=$(date -u -d '3 days ago' +"%Y-%m-%dT%H:%M:%SZ")
for i in 1 2; do brk record bf-stale -1 >/dev/null; done
jq --arg ts "$OLD" '.beads["bf-stale"].last_crash_ts = $ts' "$STATE_FILE" > "$STATE_FILE.tmp" \
    && mv "$STATE_FILE.tmp" "$STATE_FILE"
brk record bf-stale -1 >/dev/null   # crash 3 days later -> fresh count of 1
COUNT=$(jq -r '.beads["bf-stale"].consecutive_crashes' "$STATE_FILE")
OUT=$(brk check bf-stale >/dev/null 2>&1; echo $?)
if [[ "$COUNT" == "1" ]] && [[ $OUT -eq 0 ]]; then
    pass "counter decays after 24h: old crashes do not trip new work"
else
    fail "decay should reset counter to 1, got count=$COUNT check rc=$OUT"
fi

# Test 17: usage/help exits cleanly
brk --help >/dev/null
RC=$?
if [[ $RC -eq 0 ]]; then
    pass "--help exits 0"
else
    fail "--help should exit 0, got $RC"
fi

# Cleanup
rm -rf "$TEST_TMP"

echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo "Total tests: $test_count"
echo -e "Passed: ${GREEN}$pass_count${NC}"
echo -e "Failed: ${RED}$fail_count${NC}"
echo ""

if [[ $fail_count -eq 0 ]]; then
    echo -e "${GREEN}All tests passed!${NC}"
    echo ""
    echo "✅ Circuit breaker is properly implemented:"
    echo "   - Per-bead consecutive-crash counting (-1/137 only)"
    echo "   - Trip at threshold, dispatch blocked (exit 4)"
    echo "   - Success closes; half-open probe after backoff"
    echo "   - Exponential backoff with max cap"
    echo "   - Deferral via 'bead update --status deferred'"
    echo "   - Rebuild from .beads/events.jsonl"
    echo "   - 24h counter decay"
    exit 0
else
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
fi
