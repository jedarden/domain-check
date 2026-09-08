#!/usr/bin/env bash
# Functional test for the dispatch-layer circuit-breaker gate in
# scripts/needle-with-limiter.sh.
#
# The breaker's own suite (test-crash-circuit-breaker.sh) and the storm replay
# (test-crash-storm-regression.sh) exercise the breaker's STATE MACHINE — leg 2
# of the storm replay reimplements the documented check/defer contract rather
# than invoking this wrapper, and the breaker suite never sources the wrapper.
# Neither can see a WIRING bug. This suite does: it runs the real
# needle-with-limiter.sh against the real crash-circuit-breaker.sh with only
# `bead`, `needle` and the concurrency limiter stubbed, and asserts on the
# store mutations and dispatches that actually happen.
#
# Regressions pinned here:
#   - an OPEN breaker (cooldown not elapsed) must DEFER the bead out of the
#     ready frontier and still dispatch (the wrapper gates the sweep, not the
#     requested run)  — this is the case that stayed green under the
#     `if ! cmd; then rc=$?` idiom that read the negated pipeline status and
#     folded BLOCKED (4) into the allow path, making enforcement dead code
#   - a breaker-deferred bead (marker in Notes) is re-opened ONCE at half-open
#   - a HUMAN-deferred bead (no marker) is never touched
#   - a missing breaker script fails OPEN; advisory mode mutates nothing
#   - timeouts (124) and workflow exits (1) never trip the breaker
#   - needle still receives its arguments verbatim
#
# bash-only, mktemp sandboxes, no live bead store, standalone from
# `go test ./...`, ~1s runtime.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
WRAPPER="$SCRIPT_DIR/needle-with-limiter.sh"
BREAKER="$SCRIPT_DIR/crash-circuit-breaker.sh"

PASS=0
FAIL=0

say()  { printf '%s\n' "$*"; }
ok()   { PASS=$((PASS + 1)); printf '  \033[0;32mPASS\033[0m: %s\n' "$1"; }
bad()  { FAIL=$((FAIL + 1)); printf '  \033[0;31mFAIL\033[0m: %s\n' "$1"; }

assert_eq() { # $1 desc, $2 actual, $3 expected
    if [[ "$2" == "$3" ]]; then ok "$1"; else
        bad "$1 (expected [$3], got [$2])"
    fi
}
assert_contains() { # $1 desc, $2 haystack, $3 needle
    if [[ "$2" == *"$3"* ]]; then ok "$1"; else
        bad "$1 (missing [$3])"
    fi
}
assert_not_contains() { # $1 desc, $2 haystack, $3 needle
    if [[ "$2" != *"$3"* ]]; then ok "$1"; else
        bad "$1 (unexpectedly contains [$3])"
    fi
}

# ── Sandbox ───────────────────────────────────────────────────────────────────
# $SB/scripts/  real wrapper + real breaker + stub limiter
# $SB/bin/      fake `bead` (canned show output, logs every call) + fake needle
# $SB/fake/     per-test canned state for the fake bead

SB="$(mktemp -d "${TMPDIR:-/tmp}/limiter-gate-test.XXXXXX")"
trap 'rm -rf "$SB"' EXIT
mkdir -p "$SB/scripts" "$SB/bin" "$SB/fake" "$SB/beads/logs"

cp "$WRAPPER" "$SB/scripts/"
cp "$BREAKER" "$SB/scripts/"

# Stub limiter: the gate under test runs BEFORE the limiter; grant a slot
# immediately so the tests exercise only the breaker gate.
cat > "$SB/scripts/agent-concurrency-limiter.sh" <<'EOF'
agent_limit_start() { return 0; }
agent_limit_cleanup() { return 0; }
EOF

cat > "$SB/bin/needle" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "needle $*" >> "$NEEDLE_CALLS"
exit 0
EOF
chmod +x "$SB/bin/needle"

# Fake `bead`: `show <id>` prints Status (+ optional breaker marker) from
# $SB/fake/<id>.status / .marker; every invocation is logged to $STORE_CALLS.
cat > "$SB/bin/bead" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "bead $*" >> "$STORE_CALLS"
if [[ "${1:-}" == "show" ]]; then
    id="$2"
    echo "ID: $id"
    echo "Status: $(cat "$SB_DIR/fake/$id.status" 2>/dev/null || echo Open)"
    if [[ -f "$SB_DIR/fake/$id.marker" ]]; then
        echo "Notes: circuit breaker: 3 consecutive infrastructure crashes"
    fi
    exit 0
fi
exit 0
EOF
chmod +x "$SB/bin/bead"

export SB_DIR="$SB"
export STORE_CALLS="$SB/store-calls.log"
export NEEDLE_CALLS="$SB/needle-calls.log"
: > "$STORE_CALLS"
: > "$NEEDLE_CALLS"
export BREAKER_SCRIPT="$SB/scripts/crash-circuit-breaker.sh"
export BREAKER_STATE_FILE="$SB/beads/logs/circuit-breaker-state.json"
export PATH="$SB/bin:$PATH"

# Trip a bead through the REAL breaker, exactly as the manager would.
trip_bead() { # $1 = bead id
    local id="$1" i
    for ((i = 0; i < 3; i++)); do
        "$BREAKER_SCRIPT" record "$id" -1 >/dev/null 2>&1
    done
}

reset_fixtures() {
    : > "$STORE_CALLS"
    : > "$NEEDLE_CALLS"
    rm -f "$SB"/fake/*
    rm -f "$BREAKER_STATE_FILE"
}

run_wrapper() {
    "$SB/scripts/needle-with-limiter.sh" run --workspace /fake/ws --agent fake --count 1 2>&1
}

say "=== needle-with-limiter.sh dispatch-breaker gate ==="

# 1. No breaker state at all: sweep is a no-op, dispatch proceeds untouched.
reset_fixtures
out=$(run_wrapper)
assert_contains "no state -> sweep reports no beads held" "$out" "no beads held down - dispatch proceeds"
assert_contains "no state -> needle executed" "$(cat "$NEEDLE_CALLS")" "needle run --workspace /fake/ws --agent fake --count 1"
assert_eq "no state -> no store mutation" "$(wc -l < "$STORE_CALLS")" "0"

# 2. OPEN breaker, cooldown not elapsed, bead claimable -> DEFER it, then
#    dispatch anyway (the sweep gates the fleet, not the requested run).
reset_fixtures
trip_bead sb-trip
rc=0; out=$(run_wrapper) || rc=$?
assert_eq "wrapper exit 0 with an open breaker" "$rc" "0"
assert_contains "open breaker -> BLOCKED verdict logged" "$out" "Breaker OPEN for sb-trip"
assert_contains "open breaker -> bead deferred out of the frontier" "$out" "deferred sb-trip out of the ready frontier"
assert_contains "open breaker -> store saw the deferral" "$(cat "$STORE_CALLS")" "--status deferred"
assert_contains "open breaker -> deferral note carries the breaker marker" "$(cat "$STORE_CALLS")" "circuit breaker:"
assert_contains "open breaker -> dispatch still ran" "$(cat "$NEEDLE_CALLS")" "needle run --workspace"

# 3. Same breaker state, bead now Deferred BY THE BREAKER -> hold, no mutation.
reset_fixtures   # clears state file and call logs, not fixtures below
trip_bead sb-trip
echo -n "Deferred" > "$SB/fake/sb-trip.status"
touch "$SB/fake/sb-trip.marker"
out=$(run_wrapper)
assert_contains "breaker-deferred bead -> holding" "$out" "already deferred - holding"
assert_not_contains "breaker-deferred bead -> no re-defer" "$(cat "$STORE_CALLS")" "--status deferred"
assert_not_contains "breaker-deferred bead -> no probe re-open" "$(cat "$STORE_CALLS")" "--status open"

# 4. Half-open (cooldown elapsed): breaker-deferred bead is re-opened ONCE.
jq --arg id sb-trip --arg ts "$(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%SZ)" \
    '.beads[$id].retry_after = $ts' "$BREAKER_STATE_FILE" > "$BREAKER_STATE_FILE.tmp" \
    && mv "$BREAKER_STATE_FILE.tmp" "$BREAKER_STATE_FILE"
out=$(run_wrapper)
assert_contains "half-open -> probe allowed" "$out" "ONE probe dispatch allowed"
assert_contains "half-open -> bead re-opened for the probe" "$(cat "$STORE_CALLS")" "--status open"

# 5. Half-open, but the bead was deferred by a HUMAN (no marker) -> untouched.
reset_fixtures
trip_bead sb-human
echo -n "Deferred" > "$SB/fake/sb-human.status"   # no .marker file
jq --arg id sb-human --arg ts "$(date -u -d '1 hour ago' +%Y-%m-%dT%H:%M:%SZ)" \
    '.beads[$id].retry_after = $ts' "$BREAKER_STATE_FILE" > "$BREAKER_STATE_FILE.tmp" \
    && mv "$BREAKER_STATE_FILE.tmp" "$BREAKER_STATE_FILE"
out=$(run_wrapper)
assert_contains "human-deferred bead -> left alone" "$out" "Deferred without a breaker note - left alone"
assert_not_contains "human-deferred bead -> no store write" "$(cat "$STORE_CALLS")" "--status open"

# 6. Missing breaker script -> fail OPEN, dispatch proceeds.
reset_fixtures
out=$(BREAKER_SCRIPT="$SB/scripts/does-not-exist.sh" run_wrapper)
assert_contains "missing breaker -> fail-open warning" "$out" "missing or not executable"
assert_contains "missing breaker -> dispatch proceeded" "$(cat "$NEEDLE_CALLS")" "needle run --workspace"

# 7. Advisory mode (BREAKER_DISPATCH_ENFORCE=0): verdict logged, store untouched.
reset_fixtures
trip_bead sb-adv
echo -n "Open" > "$SB/fake/sb-adv.status"
out=$(BREAKER_DISPATCH_ENFORCE=0 run_wrapper)
assert_contains "advisory mode -> BLOCKED verdict still logged" "$out" "Breaker OPEN for sb-adv"
assert_contains "advisory mode -> no store mutation" "$out" "advisory mode (BREAKER_DISPATCH_ENFORCE=0): no store mutation"
assert_not_contains "advisory mode -> no deferral written" "$(cat "$STORE_CALLS")" "--status deferred"

# 8. Non-infrastructure exits never trip the breaker (124 timeout, exit 1).
reset_fixtures
for i in 1 2 3 4 5; do
    "$BREAKER_SCRIPT" record sb-timeout 124 >/dev/null 2>&1
    "$BREAKER_SCRIPT" record sb-workflow 1 >/dev/null 2>&1
done
"$BREAKER_SCRIPT" check sb-timeout >/dev/null 2>&1; timeout_rc=$?
"$BREAKER_SCRIPT" check sb-workflow >/dev/null 2>&1; workflow_rc=$?
assert_eq "5x timeout(124) -> breaker still closed (check rc 0)" "$timeout_rc" "0"
assert_eq "5x workflow(1) -> breaker still closed (check rc 0)" "$workflow_rc" "0"

# 9. Wrapper syntax and the breaker check contract.
bash -n "$WRAPPER" && ok "wrapper passes bash -n" || bad "wrapper fails bash -n"
reset_fixtures
trip_bead sb-rc
"$BREAKER_SCRIPT" check sb-rc >/dev/null 2>&1; blocked_rc=$?
assert_eq "tripped bead -> breaker check exits 4" "$blocked_rc" "4"

say ""
say "=========================================="
say "Total tests: $((PASS + FAIL))"
say "Passed: $PASS"
say "Failed: $FAIL"
say ""
if [[ $FAIL -eq 0 ]]; then
    echo -e "\033[0;32mAll tests passed!\033[0m"
    echo ""
    echo "✅ The dispatch path enforces the breaker's verdict:"
    echo "   - OPEN + cooldown pending -> bead DEFERRED out of the ready frontier"
    echo "   - half-open -> exactly one probe (breaker-deferred bead re-opened)"
    echo "   - human deferrals untouched; missing breaker fails open; advisory mutates nothing"
    echo "   - only exit -1/137 trip the breaker (124/1 stay with retry/classifier)"
    exit 0
else
    echo -e "\033[0;31mFAILURES present\033[0m"
    exit 1
fi
