#!/usr/bin/env bash
# Test: preflight-health-check.sh Check 4 (crash-storm circuit breaker)
#
# Runs the REAL preflight script in a sandbox git repo with stubbed sibling
# checks (service / repo-health / cgroup / system-event) and the REAL
# scripts/crash-circuit-breaker.sh, then asserts the breaker check's contract:
#   - an OPEN breaker is surfaced by bead id at preflight time (GAP-4: nothing
#     in the live path looked at breaker state before this check)
#   - it WARNS but never fails the preflight (the remedy is per-bead deferral,
#     not a box-wide stop; load-shedding is check 0's job)
#   - missing breaker script and unparseable state both fail open
#   - a tracked-but-untripped bead is reported as tracked, not open
#
# Created: 2026-09-08 by domchk-e1400e03 (bf-4x12ec fix-implementation child).
# Stub mechanics follow scripts/test-closed-bead-filter.sh; isolated state per
# test follows scripts/test-crash-circuit-breaker.sh.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PREFLIGHT="$SCRIPT_DIR/preflight-health-check.sh"
REAL_BREAKER="$SCRIPT_DIR/crash-circuit-breaker.sh"

echo "=========================================="
echo "Testing preflight circuit-breaker check"
echo "=========================================="
echo ""

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

test_count=0
pass_count=0
fail_count=0

TEST_TMP="$(mktemp -d)"
cleanup() {
    if [[ "${PREFLIGHT_TEST_KEEP:-0}" == "1" ]]; then
        echo "sandbox kept: $TEST_TMP"
    else
        rm -rf "$TEST_TMP"
    fi
}
trap cleanup EXIT

pass() {
    pass_count=$((pass_count + 1))
    echo -e "  ${GREEN}✓${NC} $1"
}
fail() {
    fail_count=$((fail_count + 1))
    echo -e "  ${RED}✗${NC} $1"
}

# assert_contains <label> <needle> <haystack-file>
assert_contains() {
    local label="$1" needle="$2" file="$3"
    if grep -qF -- "$needle" "$file"; then
        pass "$label"
    else
        fail "$label (missing: $needle)"
        echo "    --- actual output ---"
        sed 's/^/    /' "$file"
    fi
}

# make_sandbox <name> — git repo with stubbed siblings + the real breaker
make_sandbox() {
    local sbx="$TEST_TMP/$1"
    mkdir -p "$sbx/scripts" "$sbx/.beads/logs"
    git init -q "$sbx"
    # Stub every expensive/networked sibling check so the preflight's other
    # legs pass deterministically and only the breaker check varies.
    printf '#!/usr/bin/env bash\nexit 0\n' > "$sbx/scripts/service-monitor.sh"
    printf '#!/usr/bin/env bash\nexit 0\n' > "$sbx/scripts/check-repo-health.sh"
    printf '#!/usr/bin/env bash\nexit 0\n' > "$sbx/scripts/cgroup-memory-guard.sh"
    printf '#!/usr/bin/env bash\nexit 0\n' > "$sbx/scripts/system-event-mode.sh"
    # The preflight resolves its siblings from its own directory, so the copy
    # under test must sit beside the same stubs/breaker it will invoke.
    cp "$PREFLIGHT" "$sbx/scripts/preflight-health-check.sh"
    cp "$REAL_BREAKER" "$sbx/scripts/crash-circuit-breaker.sh"
    SANDBOX="$sbx"
}

# run_preflight — runs the real script inside the sandbox; echoes its exit code
run_preflight() {
    local rc=0
    (cd "$SANDBOX" && bash scripts/preflight-health-check.sh) > "$SANDBOX.out" 2>&1 || rc=$?
    return "$rc"
}

# write_state <json> — fabricate breaker state the way needle-with-limiter.sh
# and crash-circuit-breaker.sh record leave it
write_state() {
    printf '%s' "$1" > "$SANDBOX/.beads/logs/circuit-breaker-state.json"
}

open_bead_json() {
    # $1 = bead id, $2 = seconds from now for retry_after
    local id="$1" in_secs="$2" retry
    retry=$(date -u -d "@$(( $(date +%s) + in_secs ))" +"%Y-%m-%dT%H:%M:%SZ")
    printf '{"beads":{"%s":{"state":"open","consecutive_crashes":3,"open_count":1,"opened_at":"2026-09-08T00:00:00Z","retry_after":"%s","last_crash_ts":"2026-09-08T00:00:00Z","last_exit_code":-1}},"updated_at":"2026-09-08T00:00:00Z"}' "$id" "$retry"
}

# --- Test 1: no breaker state at all -> pass, preflight healthy -------------
echo "Test 1: no breaker state — check passes"
test_count=$((test_count + 1))
make_sandbox t1
if run_preflight; then
    pass "preflight exits 0 with no breaker state"
else
    fail "preflight exited nonzero with no breaker state"
fi
assert_contains "check reports no open breakers" "No open breakers" "$SANDBOX.out"
echo ""

# --- Test 2: OPEN breaker -> surfaced by id, preflight still exits 0 --------
echo "Test 2: open breaker is surfaced but does not fail the preflight"
test_count=$((test_count + 1))
make_sandbox t2
write_state "$(open_bead_json domchk-stormtest 1800)"
run_preflight
RC2=$?
if [[ $RC2 -eq 0 ]]; then
    pass "preflight exits 0 despite an OPEN breaker (warn, not fail)"
else
    fail "preflight exited $RC2 on an OPEN breaker — must warn, never fail"
fi
assert_contains "open breaker named by bead id" "domchk-stormtest" "$SANDBOX.out"
assert_contains "open breaker flagged as OPEN" "breaker(s) OPEN" "$SANDBOX.out"
assert_contains "action line names the defer command" "crash-circuit-breaker.sh defer" "$SANDBOX.out"
echo ""

# --- Test 3: unreadable / wrong-schema state -> fail open -------------------
# Two distinct fail-open layers: corrupt JSON makes the breaker's own
# `status` exit nonzero ("unreadable"), while valid JSON with a wrong schema
# parses for the breaker but defeats the preflight's open-count query
# ("not parseable"). Both must skip, never fail the preflight.
echo "Test 3: corrupt breaker state fails open (unreadable layer)"
test_count=$((test_count + 1))
make_sandbox t3
write_state '{"beads": BROKEN'
if run_preflight; then
    pass "preflight exits 0 on corrupt breaker state"
else
    fail "preflight exited nonzero on corrupt breaker state"
fi
assert_contains "corrupt state reported as unreadable" "Breaker state unreadable" "$SANDBOX.out"

echo "Test 3b: wrong-schema state fails open (not-parseable layer)"
test_count=$((test_count + 1))
make_sandbox t3b
# A bead entry that is not a state object: the breaker's own `jq .` still
# prints it, but the open-count query errors indexing a string — the
# not-parseable layer's actual input. (.beads as a bare string would instead
# read as zero opens: jq's `?` suppresses the iteration error.)
write_state '{"beads": {"domchk-drifted": "not-a-state-object"}}'
if run_preflight; then
    pass "preflight exits 0 on wrong-schema breaker state"
else
    fail "preflight exited nonzero on wrong-schema breaker state"
fi
assert_contains "wrong-schema state reported as not parseable" "not parseable" "$SANDBOX.out"
echo ""

# --- Test 4: breaker script missing -> skipped ------------------------------
echo "Test 4: missing breaker script is skipped"
test_count=$((test_count + 1))
make_sandbox t4
rm "$SANDBOX/scripts/crash-circuit-breaker.sh"
if run_preflight; then
    pass "preflight exits 0 with the breaker script absent"
else
    fail "preflight exited nonzero with the breaker script absent"
fi
assert_contains "absent script reported as skipped" "crash-circuit-breaker.sh not found (skipping)" "$SANDBOX.out"
echo ""

# --- Test 5: tracked-but-untripped bead is not reported as open -------------
echo "Test 5: counting (below-threshold) bead is tracked, not open"
test_count=$((test_count + 1))
make_sandbox t5
write_state '{"beads":{"domchk-counting":{"state":"closed","consecutive_crashes":2,"last_crash_ts":"2026-09-08T00:00:00Z","last_exit_code":-1}},"updated_at":"2026-09-08T00:00:00Z"}'
if run_preflight; then
    pass "preflight exits 0 with a below-threshold bead"
else
    fail "preflight exited nonzero with a below-threshold bead"
fi
assert_contains "tracked count reported, none tripped" "1 bead(s) tracked, none tripped" "$SANDBOX.out"
if grep -qF "breaker(s) OPEN" "$SANDBOX.out"; then
    fail "below-threshold bead was misreported as OPEN"
else
    pass "below-threshold bead not misreported as OPEN"
fi
echo ""

# --- Test 6: multiple open breakers all surfaced ----------------------------
echo "Test 6: multiple open breakers are all surfaced with a count"
test_count=$((test_count + 1))
make_sandbox t6
# open_bead_json emits a whole state object for one bead; build the two-bead
# state by splicing the second bead into the first's object
FIRST="$(open_bead_json domchk-storm-a 1800)"
SECOND="$(open_bead_json domchk-storm-b 3600)"
write_state "$(printf '%s' "$FIRST" | jq --argjson b "$SECOND" '.beads["domchk-storm-b"] = $b.beads["domchk-storm-b"]')"
if run_preflight; then
    pass "preflight exits 0 with two OPEN breakers"
else
    fail "preflight exited nonzero with two OPEN breakers"
fi
assert_contains "count reflects both opens" "2 breaker(s) OPEN" "$SANDBOX.out"
assert_contains "first open bead named" "domchk-storm-a" "$SANDBOX.out"
assert_contains "second open bead named" "domchk-storm-b" "$SANDBOX.out"
echo ""

# --- Test 7: open entry missing optional fields renders the fallback --------
echo "Test 7: open entry missing retry_after renders the '?' fallback"
test_count=$((test_count + 1))
make_sandbox t7
# Partial state (e.g. written by an older breaker version) must not break the
# detail render: the fallback prints '?' and the preflight still exits 0.
write_state '{"beads":{"domchk-partial":{"state":"open","consecutive_crashes":3,"last_exit_code":-1}},"updated_at":"2026-09-08T00:00:00Z"}'
if run_preflight; then
    pass "preflight exits 0 with a partial open entry"
else
    fail "preflight exited nonzero with a partial open entry"
fi
assert_contains "partial entry still named by bead id" "domchk-partial" "$SANDBOX.out"
# The jq program's quotes around "?" are syntax, not output — the fallback
# renders a bare question mark after 'retry_after'.
assert_contains "missing retry_after renders fallback" 'retry_after ?' "$SANDBOX.out"
echo ""

# --- Summary ----------------------------------------------------------------
echo "=========================================="
echo "Tests passed: $pass_count / $((pass_count + fail_count))"
echo "=========================================="

if [[ $fail_count -eq 0 ]]; then
    echo -e "${GREEN}All preflight breaker-check tests passed${NC}"
    exit 0
else
    echo -e "${RED}FAILURES: $fail_count assertion(s) across $test_count test(s)${NC}"
    exit 1
fi
