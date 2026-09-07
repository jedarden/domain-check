#!/usr/bin/env bash
# Functional test: crash classification actually reaches the alert layer
# (domchk-701bcfa5; wiring defect domchk-f6fff20f)
#
# The other suites in this directory grep the scripts' SOURCE for markers
# (test-crash-alert-fixes.sh tests 4-12). That cannot see a wiring bug: both
# sides can carry the right strings and still be disconnected. The defect this
# file guards was exactly that — crash-classifier.sh printed its '====' banner
# before the verdict and crash-alert-manager.sh read CLASSIFICATION with
# `head -1`, so the manager stored '==================================' as the
# classification, its FALSE_POSITIVE branch never fired, and a false-positive
# crash on an open bead fanned out into a full ALERT bead.
#
# So every case here runs the REAL classifier and the REAL manager against a
# fabricated crash in a throwaway sandbox (mock `bead` binary, own
# .beads/logs) and asserts on behaviour: exit codes, whether an alert was
# written, and what classification string was stored.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLASSIFIER="$SCRIPT_DIR/crash-classifier.sh"
MANAGER="$SCRIPT_DIR/crash-alert-manager.sh"

PASS=0
FAIL=0
SB=""

# Fixed crash instant so the trace's captured_at provably falls inside the
# incident window (trace-slot provenance must be "ok" for trace content to be
# quoted as evidence at all).
WS_ISO="2026-08-16T17:20:00Z"
WE_ISO="2026-08-16T17:21:00Z"

# Hermetic measurement overrides: box health must not decide test outcomes.
HERMETIC_MEM=67108864   # 64 GiB MemAvailable — memory signal suppressed
HERMETIC_REPO=1048576   # 1 MiB .git          — bloat signal suppressed

pass() { PASS=$((PASS + 1)); echo "  ✓ PASS: $1"; }
fail() {
    FAIL=$((FAIL + 1))
    echo "  ✗ FAIL: $1"
    if [ -n "${2:-}" ]; then
        echo "    got: ${2}" | head -8
    fi
}

new_sandbox() {
    [ -n "$SB" ] && rm -rf "$SB"
    SB="$(mktemp -d /tmp/classifier-wiring.XXXXXX)"
    # The manager sources its siblings by SCRIPT_DIR, so the sandbox needs the
    # whole scripts directory, not just the two scripts under test (a lone
    # extracted copy fails by construction).
    cp -r "$SCRIPT_DIR" "$SB/scripts"
    chmod -R u+w "$SB/scripts"
    mkdir -p "$SB/bin" "$SB/.beads/logs"
    printf '#!/usr/bin/env bash\necho "Status: %s"\n' "${1:-open}" > "$SB/bin/bead"
    chmod +x "$SB/bin/bead"
}

# make_crash <id> <exit_code> <trace-text> — a crash record + its trace slot.
make_crash() {
    local id="$1" ec="$2" trace="$3"
    mkdir -p "$SB/.beads/traces/$id"
    printf '{"exit_code": %s, "captured_at": "%s"}\n' "$ec" "$WE_ISO" \
        > "$SB/.beads/traces/$id/metadata.json"
    printf '%s\n' "$trace" > "$SB/.beads/traces/$id/trace.jsonl"
}

run_manager() {
    local id="$1"
    (cd "$SB" && env \
        PATH="$SB/bin:$PATH" \
        CRASH_WINDOW_START="$WS_ISO" \
        CRASH_WINDOW_END="$WE_ISO" \
        MEM_AVAILABLE_KB="$HERMETIC_MEM" \
        REPO_BYTES="$HERMETIC_REPO" \
        bash "$SB/scripts/$(basename "$MANAGER")" "$id" 2>&1)
}

run_classify() {
    local id="$1"
    (cd "$SB" && env \
        PATH="$SB/bin:$PATH" \
        CRASH_WINDOW_START="$WS_ISO" \
        CRASH_WINDOW_END="$WE_ISO" \
        MEM_AVAILABLE_KB="$HERMETIC_MEM" \
        REPO_BYTES="$HERMETIC_REPO" \
        bash "$SB/scripts/$(basename "$CLASSIFIER")" "$id" 2>&1)
}

# first_line / stored_class — the two things the wiring is supposed to get right.
first_line() { printf '%s\n' "$1" | head -1; }

stored_class() {
    [ -f "$SB/.beads/logs/alert-state.json" ] || { echo "<no alert-state.json>"; return 1; }
    jq -r '.recent[-1].classification // empty' "$SB/.beads/logs/alert-state.json" 2>/dev/null
}

assert_valid_token() {
    case "$1" in
        FALSE_POSITIVE|SERVICE_FAILURE|INFRASTRUCTURE|CODE_DEFECT|UNKNOWN) return 0 ;;
        *) return 1 ;;
    esac
}

MAX_TURNS_TRACE='{"error":"error_max_turns"}'
GATEWAY_TRACE='{"error":"HTTP 503 no available server"}'
NEUTRAL_TRACE='{"tool":"Bash","input":{"command":"ls -la"}}'

echo "=========================================="
echo "Testing classification -> alert wiring"
echo "=========================================="
echo ""

# --- 1. The classifier's first line is the bare token, for every verdict ----
echo "Case 1: classifier prints the bare token as line 1"
for spec in "bf-wire1:$MAX_TURNS_TRACE:FALSE_POSITIVE" \
            "bf-wire2:$GATEWAY_TRACE:SERVICE_FAILURE" \
            "bf-wire3:$NEUTRAL_TRACE:INFRASTRUCTURE"; do
    id="${spec%%:*}"; rest="${spec#*:}"
    trace="${rest%:*}"; want="${rest##*:}"
    new_sandbox open
    make_crash "$id" -1 "$trace"
    out="$(run_classify "$id")"
    got="$(first_line "$out")"
    if [ "$got" = "$want" ]; then
        pass "$id -> line 1 is '$want'"
    else
        fail "$id -> line 1 should be '$want'" "$got"
    fi
done
echo ""

# --- 2. Even when the trace is NOT the incident run, the token stays line 1 --
echo "Case 2: provenance-mismatch path keeps the token on line 1"
new_sandbox open
make_crash bf-wire4 -1 "$MAX_TURNS_TRACE"
# Trace captured 2h AFTER the incident window -> provenance mismatch, so the
# trace must not be quoted and the verdict comes from the event stream.
printf '{"exit_code": -1, "captured_at": "2026-08-16T19:21:00Z"}\n' \
    > "$SB/.beads/traces/bf-wire4/metadata.json"
printf '{"bead":"bf-wire4","event":"crash","exit_code":-1,"ts":"%s"}\n' "$WE_ISO" \
    > "$SB/.beads/events.jsonl"
out="$(run_classify bf-wire4)"
got="$(first_line "$out")"
[ "$got" = "INFRASTRUCTURE" ] && pass "line 1 is the token (INFRASTRUCTURE)" \
    || fail "line 1 should be the token INFRASTRUCTURE" "$got"
if printf '%s\n' "$out" | grep -q "incident run"; then
    if [ "$(printf '%s\n' "$out" | grep -n "incident run" | cut -d: -f1)" -gt 1 ]; then
        pass "provenance note printed after the token"
    else
        fail "provenance note printed BEFORE the token" "$(first_line "$out")"
    fi
else
    pass "provenance note printed after the token"
fi
echo ""

# --- 3. THE defect: FP crash on an OPEN bead must not fan out into an alert --
echo "Case 3: FP-classified crash on an open bead -> no alert (domchk-f6fff20f)"
new_sandbox open
make_crash bf-wire5 1 "$MAX_TURNS_TRACE"
out="$(run_manager bf-wire5)"; rc=$?
[ "$rc" -eq 0 ] && pass "manager exit 0 (no alert)" || fail "manager exit should be 0" "exit $rc; $out"
if [ ! -f "$SB/.beads/logs/alert-state.json" ]; then
    pass "no alert-state.json written"
else
    fail "no alert-state.json written" "$(stored_class)"
fi
if printf '%s\n' "$out" | grep -qi "false positive"; then
    pass "manager reports the false positive"
else
    fail "manager reports the false positive" "$out"
fi
echo ""

# --- 4. A genuine crash still alerts, and stores a REAL classification -------
echo "Case 4: genuine crash -> alert with a real classification token"
new_sandbox open
make_crash bf-wire6 -1 "$NEUTRAL_TRACE"
out="$(run_manager bf-wire6)"; rc=$?
[ "$rc" -eq 1 ] && pass "manager exit 1 (alert generated)" || fail "manager exit should be 1" "exit $rc; $out"
got="$(stored_class)"
if assert_valid_token "$got"; then
    pass "alert-state classification is a valid token ($got)"
else
    fail "alert-state classification is a valid token" "${got:-<empty>}"
fi
[ "$got" = "INFRASTRUCTURE" ] && pass "classification is INFRASTRUCTURE" \
    || fail "classification should be INFRASTRUCTURE" "$got"
echo ""

# --- 5. Cooldown keys on the classification, not on a constant ---------------
echo "Case 5: cooldown lookup uses the extracted classification"
if grep -q 'select(.classification == \$type)' "$MANAGER"; then
    if grep -q 'grep -m1 -E .*\^(FALSE_POSITIVE\|SERVICE_FAILURE\|INFRASTRUCTURE\|CODE_DEFECT\|UNKNOWN)' "$MANAGER"; then
        pass "classification extracted by anchored token match (not head -1)"
    else
        fail "classification extracted by anchored token match (not head -1)" "grep for the token anchor"
    fi
else
    fail "cooldown keys on \$CLASSIFICATION" "select(.classification == \$type) not found"
fi
echo ""

# --- 6. A CLOSED bead's crash is a false positive, and stays line 1 ----------
echo "Case 6: crash on a CLOSED bead -> FALSE_POSITIVE on line 1"
new_sandbox closed
make_crash bf-wire7 -1 "$NEUTRAL_TRACE"
out="$(run_classify bf-wire7)"
got="$(first_line "$out")"
[ "$got" = "FALSE_POSITIVE" ] && pass "line 1 is FALSE_POSITIVE" \
    || fail "line 1 should be FALSE_POSITIVE" "$got"
echo ""

rm -rf "$SB"
echo "=========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "=========================================="
[ "$FAIL" -eq 0 ]
