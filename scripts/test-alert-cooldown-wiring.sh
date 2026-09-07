#!/usr/bin/env bash
# Functional test: the global alert cooldown actually gates the alert layer
# (domchk-7b404946)
#
# Companion to test-alert-cooldown.sh, which exercises alert-cooldown.sh as a
# subprocess in isolation. This file runs the REAL crash-alert-manager against
# a simulated rapid crash sequence — the wiring is the part the isolated suite
# cannot see, and wiring is exactly how the classification defect (domchk-
# f6fff20f) slipped past the grep-marker tests: both sides carried the right
# strings and were still disconnected. The same lesson, one gate over: assert
# on behaviour (exit codes, whether an alert fired, what the state holds), not
# on source markers.
#
# Cascade shape asserted here (the 2026-08-16 fan-out that reached 177
# crashes across 59 beads): one crash alerts and opens the 5-minute global
# window, five further crashes arrive inside it and are logged but NOT
# alerted, and the first crash after the window closes alerts carrying their
# summary. All of it inside a mktemp sandbox — the live
# .beads/logs/crash-alert-metadata.json is never read or written.
#
# Usage: ./scripts/test-alert-cooldown-wiring.sh   (exit 0 = all pass)
# Created: 2026-09-07 (domchk-7b404946)

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
MANAGER="$SCRIPT_DIR/crash-alert-manager.sh"

PASS=0
FAIL=0
SB=""

# Fixed crash instant so trace-slot provenance sees the capture inside the
# incident window (the classifier's evidence gate).
WS_ISO="2026-08-16T17:20:00Z"
WE_ISO="2026-08-16T17:21:00Z"

# Hermetic classifier overrides: box health must not decide test outcomes.
HERMETIC_MEM=67108864   # 64 GiB MemAvailable — memory signal suppressed
HERMETIC_REPO=1048576   # 1 MiB .git          — bloat signal suppressed

# exit -1 on an open bead with a neutral trace classifies INFRASTRUCTURE;
# the gateway trace classifies SERVICE_FAILURE — a mixed-classification
# cascade, which is exactly what the per-classification window cannot
# coalesce and this global one must.
INFRA_TRACE='{"tool":"Bash","input":{"command":"ls -la"}}'
SVC_TRACE='{"error":"HTTP 503 no available server"}'

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
    SB="$(mktemp -d /tmp/cooldown-wiring.XXXXXX)"
    # The manager sources its siblings by SCRIPT_DIR, so the sandbox needs the
    # whole scripts directory (a lone extracted copy fails by construction).
    cp -r "$SCRIPT_DIR" "$SB/scripts"
    chmod -R u+w "$SB/scripts"
    mkdir -p "$SB/bin" "$SB/.beads/logs"
    printf '#!/usr/bin/env bash\necho "Status: %s"\n' "${1:-open}" > "$SB/bin/bead"
    chmod +x "$SB/bin/bead"
}

# make_crash <id> <trace> — an exit -1 crash record + its trace slot.
make_crash() {
    local id="$1" trace="$2"
    mkdir -p "$SB/.beads/traces/$id"
    printf '{"exit_code": -1, "captured_at": "%s"}\n' "$WE_ISO" \
        > "$SB/.beads/traces/$id/metadata.json"
    printf '%s\n' "$trace" > "$SB/.beads/traces/$id/trace.jsonl"
}

# run_manager <id> [extra args...] — the real manager in the sandbox.
run_manager() {
    local id="$1"; shift
    (cd "$SB" && env \
        PATH="$SB/bin:$PATH" \
        ALERT_COOLDOWN_METADATA_FILE="$SB/.beads/logs/crash-alert-metadata.json" \
        CRASH_WINDOW_START="$WS_ISO" \
        CRASH_WINDOW_END="$WE_ISO" \
        MEM_AVAILABLE_KB="$HERMETIC_MEM" \
        REPO_BYTES="$HERMETIC_REPO" \
        bash "$SB/scripts/$(basename "$MANAGER")" "$@" "$id" 2>/dev/null)
}

# age_window <secs> — time-travel the sandbox window's start backwards.
age_window() {
    local secs="$1"
    local state="$SB/.beads/logs/crash-alert-metadata.json"
    local tmp="$state.age.$$"
    jq --argjson s "$secs" '.last_alert_epoch = (.last_alert_epoch - $s)' \
        "$state" > "$tmp" && mv "$tmp" "$state"
    # The manager's per-classification window (alert-state.json .recent[],
    # keyed by classification) is written by the SAME fired alert that opens
    # the global window, so in production the two states are always the same
    # age. Travel both: leaving the manager's own state behind manufactures a
    # skew no real cascade can reach, and its gate then exits 0 on the
    # post-window crash before the global gate's summary logic ever runs.
    local mgr_state="$SB/.beads/logs/alert-state.json"
    if [[ -f "$mgr_state" ]]; then
        local tmp2="$mgr_state.age.$$"
        jq --argjson s "$secs" \
            '.recent |= (if . then map(.timestamp = ((.timestamp | fromdateiso8601) - $s | todateiso8601)) else . end)' \
            "$mgr_state" > "$tmp2" && mv "$tmp2" "$mgr_state"
    fi
}

cooldown_state() { # $1 = jq path — value from the sandbox cooldown state
    local state="$SB/.beads/logs/crash-alert-metadata.json"
    [ -f "$state" ] || { echo "<no state>"; return 1; }
    # NOT `$1 // "<unset>"`: jq's alternative operator treats FALSE the same
    # as null, so the value this assertion most needs to read —
    # summary_pending: false after the summary is consumed — came back as
    # "<unset>" and failed its own pass condition. Distinguish a genuinely
    # missing key by its TYPE instead (a missing path types as "null").
    if [ "$(jq -r "$1 | type" "$state" 2>/dev/null)" = "null" ]; then
        echo "<unset>"
    else
        jq -r "$1" "$state" 2>/dev/null
    fi
}

alert_count() { # alerts recorded in the manager's own alert-state.json
    local state="$SB/.beads/logs/alert-state.json"
    [ -f "$state" ] || { echo 0; return 0; }
    jq -r '.recent | length' "$state" 2>/dev/null || echo 0
}

echo "=========================================="
echo "Testing cooldown -> alert-manager wiring"
echo "Sandbox: $SB"
echo "=========================================="
echo ""

# --- 1. First crash alerts and opens the global window ----------------------
echo "Case 1: the first crash alerts and opens the cooldown window"
new_sandbox
make_crash bf-cw1 "$INFRA_TRACE"
OUT=$(run_manager bf-cw1); RC=$?
[ "$RC" -eq 1 ] && pass "manager exit 1 = alert generated" || fail "manager exit 1 = alert generated" "rc=$RC out=$OUT"
[ "$(cooldown_state .last_alert_bead)" = "bf-cw1" ] \
    && pass "cooldown state names the alerting bead" \
    || fail "cooldown state names the alerting bead" "$(cooldown_state .last_alert_bead)"
[ "$(cooldown_state .cooldown_active)" = "true" ] \
    && pass "window is active after the alert" \
    || fail "window is active after the alert" "$(cooldown_state .cooldown_active)"
[ "$(alert_count)" = "1" ] && pass "one alert in alert-state.json" \
    || fail "one alert in alert-state.json" "$(alert_count)"
echo ""

# --- 2. Rapid crash sequence: five in-window crashes logged, NOT alerted ----
echo "Case 2: rapid crash sequence — 5 in-window crashes are logged, not alerted"
for i in 2 3 4 5 6; do
    make_crash "bf-cw$i" "$SVC_TRACE"
    OUT=$(run_manager "bf-cw$i"); RC=$?
    if [ "$RC" -eq 0 ]; then
        pass "crash bf-cw$i suppressed (exit 0, no ALERT bead)"
    else
        fail "crash bf-cw$i suppressed (exit 0, no ALERT bead)" "rc=$RC out=$OUT"
    fi
    if grep -q "global alert cooldown active" <<<"$OUT"; then
        pass "crash bf-cw$i output names the cooldown as the reason"
    else
        fail "crash bf-cw$i output names the cooldown as the reason" "$OUT"
    fi
done
[ "$(alert_count)" = "1" ] \
    && pass "still exactly ONE alert after the 5-crash surge" \
    || fail "still exactly ONE alert after the 5-crash surge" "$(alert_count)"
[ "$(cooldown_state .suppressed_count)" = "5" ] \
    && pass "all 5 suppressed crashes accounted in the cooldown state" \
    || fail "all 5 suppressed crashes accounted" "$(cooldown_state .suppressed_count)"
[ "$(cooldown_state .summary_pending)" = "true" ] \
    && pass "summary is pending while the window stays open" \
    || fail "summary is pending while the window stays open" "$(cooldown_state .summary_pending)"
for i in 2 4 6; do
    jq -e --arg b "bf-cw$i" '.suppressed_recent[] | select(.bead_id == $b)' \
        "$SB/.beads/logs/crash-alert-metadata.json" >/dev/null 2>&1 \
        && pass "suppressed crash bf-cw$i named in the state ledger" \
        || fail "suppressed crash bf-cw$i named in the state ledger"
done
echo ""

# --- 3. First crash after expiry alerts carrying the suppressed summary -----
echo "Case 3: the first crash after the window closes carries the summary"
age_window 360   # window fully expired
make_crash bf-cw7 "$INFRA_TRACE"
OUT=$(run_manager bf-cw7); RC=$?
[ "$RC" -eq 1 ] && pass "post-cooldown crash alerts (exit 1)" \
    || fail "post-cooldown crash alerts (exit 1)" "rc=$RC out=$OUT"
grep -q "Cooldown Summary" <<<"$OUT" \
    && pass "alert body carries the Cooldown Summary block" \
    || fail "alert body carries the Cooldown Summary block" "$OUT"
grep -q "suppressed=5" <<<"$OUT" \
    && pass "summary reports the suppressed count (suppressed=5)" \
    || fail "summary reports the suppressed count (suppressed=5)" "$OUT"
grep -q "bf-cw2" <<<"$OUT" && grep -q "bf-cw6" <<<"$OUT" \
    && pass "summary names the first and last suppressed crashes" \
    || fail "summary names the first and last suppressed crashes" "$OUT"
[ "$(cooldown_state .summary_pending)" = "false" ] \
    && pass "summary consumed from the state (pending=false)" \
    || fail "summary consumed from the state (pending=false)" "$(cooldown_state .summary_pending)"
[ "$(cooldown_state .suppressed_count)" = "0" ] \
    && pass "suppressed count reset after consumption" \
    || fail "suppressed count reset after consumption" "$(cooldown_state .suppressed_count)"
[ "$(alert_count)" = "2" ] \
    && pass "two alerts now in alert-state.json (the surge produced one)" \
    || fail "two alerts now in alert-state.json" "$(alert_count)"
echo ""

# --- 4. The summary is emitted exactly once ---------------------------------
echo "Case 4: the summary is not repeated by the next alert"
age_window 360   # expire bf-cw7's window (it suppressed nothing)
make_crash bf-cw8 "$INFRA_TRACE"
OUT=$(run_manager bf-cw8); RC=$?
[ "$RC" -eq 1 ] && pass "next post-window crash alerts (exit 1)" \
    || fail "next post-window crash alerts (exit 1)" "rc=$RC out=$OUT"
grep -q "Cooldown Summary" <<<"$OUT" \
    && fail "no Cooldown Summary when nothing was suppressed" "$OUT" \
    || pass "no Cooldown Summary when nothing was suppressed"
echo ""

# --- 5. --force-alert bypasses the gate but still opens a window ------------
echo "Case 5: --force-alert bypasses suppression and opens a fresh window"
make_crash bf-cw9 "$INFRA_TRACE"
OUT=$(run_manager bf-cw9 --force-alert); RC=$?
[ "$RC" -eq 1 ] && pass "forced alert generated despite an open window (exit 1)" \
    || fail "forced alert generated despite an open window (exit 1)" "rc=$RC out=$OUT"
[ "$(cooldown_state .last_alert_bead)" = "bf-cw9" ] \
    && pass "forced alert opened a fresh window" \
    || fail "forced alert opened a fresh window" "$(cooldown_state .last_alert_bead)"
echo ""

# --- 6. Missing cooldown module fails open ----------------------------------
echo "Case 6: a missing cooldown module never blocks alerting"
new_sandbox
rm -f "$SB/scripts/alert-cooldown.sh"
make_crash bf-cw10 "$INFRA_TRACE"
OUT=$(run_manager bf-cw10); RC=$?
[ "$RC" -eq 1 ] && pass "alert generated with the module absent (exit 1)" \
    || fail "alert generated with the module absent (exit 1)" "rc=$RC out=$OUT"
echo ""

# --- Summary ------------------------------------------------------------------
echo "=========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "=========================================="
[ -n "$SB" ] && rm -rf "$SB"

if [ "$FAIL" -gt 0 ]; then
    exit 1
fi
exit 0
