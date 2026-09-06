#!/usr/bin/env bash
# Test System Event Mode
# Fixture-driven tests for scripts/system-event-mode.sh — the crash-surge gate
# (remediation for bf-3561g, bead domchk-d06cb3e6).
#
# Every test runs the gate against a synthetic EVENTS_FILE / PSI_FILE and an
# isolated STATE_DIR, so nothing here touches the live .beads/ store.
# Windows are generated relative to "now", matching how the gate measures.
#
# Usage: ./scripts/test-system-event-mode.sh

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GATE="$SCRIPT_DIR/system-event-mode.sh"

TMP="$(mktemp -d /tmp/system-event-test-XXXXXX)"
trap 'rm -rf "$TMP"' EXIT

GREEN='\033[0;32m'; RED='\033[0;31m'; NC='\033[0m'
pass_count=0
fail_count=0

NOW=$(date +%s)

# --- helpers -------------------------------------------------------------------
ok() { pass_count=$((pass_count + 1)); echo -e "  ${GREEN}✓ PASS${NC} $1"; }
bad() { fail_count=$((fail_count + 1)); echo -e "  ${RED}✗ FAIL${NC} $1"; }

check() { # check <desc> <expected_rc> <actual_rc>
  if [[ "$2" == "$3" ]]; then ok "$1 (exit $3)"; else bad "$1 — expected exit $2, got $3"; fi
}

mk_event() { # mk_event <outdir> <kind> <exit_code> <bead> <offset_seconds>
  local ts
  ts=$(date -u -d "@$((NOW + $5))" '+%Y-%m-%dT%H:%M:%S+00:00')
  printf '{"event":"%s","exit_code":%s,"bead":"%s","ts":"%s","worker":"test"}\n' \
    "$2" "$3" "$4" "$ts" >>"$1/events.jsonl"
}

fresh_source() { # a source is alive when ANY event is recent — simulate normal traffic
  mk_event "$1" complete 0 "ok-bead-a" -30
  mk_event "$1" complete 0 "ok-bead-b" -60
  mk_event "$1" dispatch null "ok-bead-c" -15
}

new_case() { # new_case <name> -> sets FIXTURES and SD for the test
  FIXTURES="$TMP/$1/events"; SD="$TMP/$1/state"
  mkdir -p "$FIXTURES" "$SD"
  : >"$FIXTURES/events.jsonl"
}

gate() { # gate <args...>  (uses FIXTURES / SD)
  EVENTS_FILE="$FIXTURES/events.jsonl" \
  PSI_FILE="${PSI_FIX:-$TMP/psi-normal}" \
  STATE_DIR="$SD" EVENT_LOG="$SD/gate.log" \
    "$GATE" "$@" 2>"$SD/stderr.txt"
}

write_psi() { # write_psi <avg60>
  { printf 'some avg10=0.00 avg60=%s avg300=0.00 total=100\n' "$1"
    printf 'full avg10=0.00 avg60=0.00 avg300=0.00 total=50\n'
  } >"$TMP/psi-hot"
}

kind_of() { gate json 2>/dev/null | jq -r '.kind'; }

# --- tests ----------------------------------------------------------------------

echo "=========================================="
echo "Testing System Event Mode"
echo "=========================================="

echo "Test 1: script syntax (bash -n)"
if bash -n "$GATE"; then ok "gate parses"; else bad "gate has a syntax error"; fi

echo "Test 2: healthy workspace -> clear (exit 0)"
new_case clear; fresh_source "$FIXTURES"
gate check; check "quiet workspace is clear" 0 $?
grep -q "DEGRADED" "$SD/stderr.txt" && bad "healthy source wrongly flagged stale" || ok "no false staleness warning"

echo "Test 3: 3+ kernel deaths latch a crash_burst (the bf-3561g class)"
new_case burst
fresh_source "$FIXTURES"
mk_event "$FIXTURES" crash -1 "bf-3561g" -60
mk_event "$FIXTURES" crash -1 "bf-173o7e" -90
mk_event "$FIXTURES" crash -1 "bf-4x12ec" -120
gate check; check "crash burst defers" 75 $?
[[ "$(kind_of)" == "crash_burst" ]] && ok "kind=crash_burst" || bad "expected crash_burst, got $(kind_of)"
grep -q "EVENT LATCHED" "$SD/gate.log" && ok "latch logged" || bad "no EVENT LATCHED in log"
gate alert-gate domchk-first; check "first alert of the event allowed" 0 $?
gate alert-gate domchk-second; check "second alert coalesced/suppressed" 4 $?

echo "Test 4: synchronized fail wave across 3+ beads latches fail_wave"
new_case wave
fresh_source "$FIXTURES"
for i in 1 2 3 4 5; do mk_event "$FIXTURES" fail 1 "bead-$((i % 3))" -$((i * 20)); done
gate check; check "multi-bead fail wave defers" 75 $?
[[ "$(kind_of)" == "fail_wave" ]] && ok "kind=fail_wave" || bad "expected fail_wave, got $(kind_of)"

echo "Test 5: a single bead's retry storm does NOT trip the system gate"
new_case onebead
fresh_source "$FIXTURES"
for i in 1 2 3 4 5 6; do mk_event "$FIXTURES" fail 1 "same-bead" -$((i * 30)); done
gate check; check "per-bead storm stays clear (circuit breaker's job)" 0 $?

echo "Test 6: sustained storm (>=25/h across 3+ beads, none in the 5-minute window)"
new_case storm
fresh_source "$FIXTURES"
for i in $(seq 1 30); do
  mk_event "$FIXTURES" fail 1 "bead-$((i % 4))" -$((320 + i * 100))
done
gate check; check "sustained storm defers" 75 $?
[[ "$(kind_of)" == "sustained_storm" ]] && ok "kind=sustained_storm" || bad "expected sustained_storm, got $(kind_of)"

echo "Test 7: memory pressure latches without any crash events"
new_case pressure
fresh_source "$FIXTURES"
write_psi 85.00
PSI_FIX="$TMP/psi-hot" gate check; check "high PSI defers" 75 $?
if PSI_FIX="$TMP/psi-hot" gate json 2>/dev/null | jq -e '.kind == "memory_pressure"' >/dev/null; then
  ok "kind=memory_pressure"
else
  bad "expected memory_pressure, got $(PSI_FIX="$TMP/psi-hot" kind_of)"
fi

echo "Test 8: manual latch on/off"
new_case manual
fresh_source "$FIXTURES"
gate on --reason "operator: pausing fleet during kernel upgrade" >/dev/null; check "manual on accepted" 0 $?
gate check; check "manual latch defers" 75 $?
[[ "$(kind_of)" == "manual" ]] && ok "kind=manual" || bad "expected manual, got $(kind_of)"
gate off >/dev/null
gate check; check "manual off clears" 0 $?

echo "Test 9: manual latch TTL expiry"
new_case ttl
fresh_source "$FIXTURES"
gate on --reason "brief pause" --ttl 1 >/dev/null
gate check; check "fresh TTL latch defers" 75 $?
sleep 1.3
gate check; check "expired TTL latch clears" 0 $?

echo "Test 10: hold period keeps the gate active after the burst ends"
new_case hold
fresh_source "$FIXTURES"
for i in 1 2 3; do mk_event "$FIXTURES" crash -1 "bf-hold-$i" -$((i * 10)); done
gate check >/dev/null; check "burst latches" 75 $?
: >"$FIXTURES/events.jsonl"; fresh_source "$FIXTURES"   # burst stops
gate check; check "still active during hold" 75 $?
# fast variant with HOLD_SECONDS=1 to prove expiry actually clears
rm -rf "$TMP/hold-fast"; mkdir -p "$TMP/hold-fast/events" "$TMP/hold-fast/state"
FIXTURES="$TMP/hold-fast/events"; SD="$TMP/hold-fast/state"; fresh_source "$FIXTURES"
for i in 1 2 3; do mk_event "$FIXTURES" crash -1 "bf-hold-fast-$i" -$((i * 10)); done
EVENTS_FILE="$FIXTURES/events.jsonl" PSI_FILE="$TMP/psi-normal" STATE_DIR="$SD" \
  EVENT_LOG="$SD/gate.log" HOLD_SECONDS=1 "$GATE" check 2>/dev/null
: >"$FIXTURES/events.jsonl"; fresh_source "$FIXTURES"
sleep 1.3
EVENTS_FILE="$FIXTURES/events.jsonl" PSI_FILE="$TMP/psi-normal" STATE_DIR="$SD" \
  EVENT_LOG="$SD/gate.log" HOLD_SECONDS=1 "$GATE" check 2>/dev/null
check "hold expires and clears" 0 $?
grep -q "EVENT CLEARED" "$SD/gate.log" && ok "clear logged with duration" || bad "no EVENT CLEARED in log"

echo "Test 11: blind source warns DEGRADED; STRICT_SOURCE defers"
new_case stale
mk_event "$FIXTURES" crash -1 "bf-ancient" -172800
mk_event "$FIXTURES" fail 1 "bf-ancient-2" -172900
gate check; check "stale source warns but does not defer by default" 0 $?
grep -q "DEGRADED" "$SD/stderr.txt" && ok "DEGRADED warning present" || bad "expected DEGRADED warning"
EVENTS_FILE="$FIXTURES/events.jsonl" PSI_FILE="$TMP/psi-normal" STATE_DIR="$SD" \
  EVENT_LOG="$SD/gate.log" STRICT_SOURCE=1 "$GATE" check 2>/dev/null
check "STRICT_SOURCE=1 defers on a blind source" 75 $?

echo "Test 12: json output is valid and machine-readable"
new_case json
fresh_source "$FIXTURES"
if gate json | jq -e '.active == 0 and .kind == "" and .readings.abnormal_5m == 0' >/dev/null 2>&1; then
  ok "json shape and clear state"
else
  bad "json output unexpected"
fi

echo "Test 13: usage errors"
new_case usage
"$GATE" nonsense-command >/dev/null 2>&1; check "unknown command" 2 $?
"$GATE" alert-gate >/dev/null 2>&1; check "alert-gate without bead id" 2 $?
"$GATE" on >/dev/null 2>&1; check "on without --reason" 2 $?

echo ""
echo "=========================================="
echo "Total tests: $((pass_count + fail_count))"
echo -e "${GREEN}Passed: $pass_count${NC}"
echo -e "${RED}Failed: $fail_count${NC}"
echo ""

if [[ $fail_count -eq 0 ]]; then
    echo -e "${GREEN}All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}Some tests failed!${NC}"
    exit 1
fi
