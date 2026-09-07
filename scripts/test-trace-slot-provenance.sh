#!/usr/bin/env bash
# Tests for trace-slot provenance gating in crash-classifier.sh
#
# Covers docs/crash-root-cause-bf-3561g.md §6 recommendation 7: a
# .beads/traces/<id>/ slot holds only the MOST RECENT run of a bead id, so it
# must not be quoted as crash evidence unless its metadata.json captured_at
# falls inside the incident window. The regression this guards against is the
# bf-3561g trap: the slot held a 2026-08-17 SUCCESS run (exit_code 0) while
# the crash was 2026-08-16T17:21Z, and the stale transcript's strings drove a
# FALSE_POSITIVE with a fabricated reason.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLASSIFIER="$SCRIPT_DIR/crash-classifier.sh"

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

passed=0
failed=0

pass() { echo -e "  ${GREEN}✓ PASS${NC} - $1"; passed=$((passed + 1)); }
fail() { echo -e "  ${RED}✗ FAIL${NC} - $1"; failed=$((failed + 1)); }

# Build a scratch workspace with a bead trace slot + event stream.
#   make_fixture <dir> <bead-id> <captured_at|none> <trace_exit_code> <events_exit_code|none>
make_fixture() {
    local dir="$1" bead="$2" captured="$3" trace_exit="$4" events_exit="$5"
    mkdir -p "$dir/.beads/traces/$bead"
    if [ "$captured" != "none" ]; then
        cat > "$dir/.beads/traces/$bead/metadata.json" <<EOF
{
  "bead_id": "$bead",
  "agent": "test-agent",
  "exit_code": $trace_exit,
  "outcome": "success",
  "duration_ms": 59043,
  "captured_at": "$captured",
  "trace_format": "claude_json"
}
EOF
        # A success-run transcript whose strings mimic the bf-3561g trap: it
        # mentions completed work, so unprovenanced greps would call it a
        # post-completion crash.
        printf '%s\n' '{"role":"assistant","text":"git commit done — work complete, task done"}' \
            > "$dir/.beads/traces/$bead/trace.jsonl"
    fi
    if [ "$events_exit" != "none" ]; then
        cat > "$dir/.beads/events.jsonl" <<EOF
{"bead":"$bead","duration_ms":305382,"event":"crash","exit_code":$events_exit,"outcome":"crash","strand":"auto","ts":"2026-08-16T17:21:28.132817919+00:00","worker":"lab-test"}
{"bead":"other-bead","duration_ms":1000,"event":"crash","exit_code":-1,"outcome":"crash","strand":"auto","ts":"2026-08-16T18:00:00+00:00","worker":"lab-test"}
EOF
    fi
}

run_classifier() {
    local dir="$1" bead="$2"
    (cd "$dir" && "$CLASSIFIER" "$bead" 2>&1)
}

echo "=============================================="
echo "Trace-slot provenance tests (crash-classifier)"
echo "=============================================="

# --- Test 1: the bf-3561g trap — success run in the slot, crash in events ----
echo ""
echo "Test 1: trace captured AFTER the incident window is rejected as evidence"
T1="$(mktemp -d)"
make_fixture "$T1" "bf-trap" "2026-08-17T11:06:29.750351214Z" 0 -1
OUT1="$(run_classifier "$T1" "bf-trap")"
echo "$OUT1" | grep -q "Trace provenance: mismatch" \
    && pass "mismatch detected (captured_at 2026-08-17 outside 2026-08-16 window)" \
    || fail "expected 'Trace provenance: mismatch', got: $(echo "$OUT1" | grep -i provenance)"
echo "$OUT1" | grep -q "Incident window: 1 crash record(s)" \
    && pass "incident window derived from events.jsonl" \
    || fail "incident window not derived: $(echo "$OUT1" | grep -i 'Incident window')"
echo "$OUT1" | grep -q "NOT crash evidence" \
    && pass "mismatch warning says the slot is not crash evidence" \
    || fail "missing mismatch warning"
echo "$OUT1" | grep -q "trace-derived pattern checks skipped" \
    && pass "stale transcript strings excluded from classification" \
    || fail "trace greps still ran on a mismatched slot"
echo "$OUT1" | grep -q "Task completed successfully before crash" \
    && fail "the bf-1ea4g branch fired on a DIFFERENT run's transcript (the exact bug)" \
    || pass "bf-1ea4g branch did NOT fire on the foreign transcript"
rm -rf "$T1"

# --- Test 2: trace inside the window is still used ---------------------------
echo ""
echo "Test 2: trace captured INSIDE the incident window remains usable"
T2="$(mktemp -d)"
# Trace captured 2026-08-16T17:21:28.5Z sits inside a window derived from an
# events record stamped the same second (17:21:28Z) — the same run.
make_fixture "$T2" "bf-inside" "2026-08-16T17:21:28.500000000Z" 1 1
printf '%s\n' '{"role":"assistant","text":"error_max_turns exhausted"}' > "$T2/.beads/traces/bf-inside/trace.jsonl"
OUT2="$(run_classifier "$T2" "bf-inside")"
echo "$OUT2" | grep -q "Trace provenance: ok" \
    && pass "in-window trace accepted (provenance ok)" \
    || fail "expected provenance ok, got: $(echo "$OUT2" | grep -i provenance)"
echo "$OUT2" | grep -q "max_turns exhausted" \
    && pass "trace-derived max_turns classification used when provenance is ok" \
    || fail "in-window trace greps did not run"
rm -rf "$T2"

# --- Test 3: no metadata at all -> unverified, not silently trusted ----------
echo ""
echo "Test 3: trace slot without metadata.json cannot be quoted as evidence"
T3="$(mktemp -d)"
mkdir -p "$T3/.beads/traces/bf-nometa"
printf '%s\n' '{"role":"assistant","text":"task done"}' > "$T3/.beads/traces/bf-nometa/trace.jsonl"
make_fixture "$T3" "bf-nometa" none 0 none
OUT3="$(run_classifier "$T3" "bf-nometa")"
echo "$OUT3" | grep -q "Trace provenance: unverified" \
    && pass "missing metadata -> unverified" \
    || fail "expected unverified, got: $(echo "$OUT3" | grep -i provenance)"
echo "$OUT3" | grep -q "Task completed successfully before crash" \
    && fail "unprovenanced transcript still drove classification" \
    || pass "unprovenanced transcript excluded"
rm -rf "$T3"

# --- Test 4: caller-supplied window override ---------------------------------
echo ""
echo "Test 4: CRASH_WINDOW_START/END override the events-derived window"
T4="$(mktemp -d)"
make_fixture "$T4" "bf-ovr" "2026-08-20T00:00:00Z" 0 -1
OUT4="$(cd "$T4" && CRASH_WINDOW_START=2026-08-19T23:00:00Z CRASH_WINDOW_END=2026-08-20T01:00:00Z "$CLASSIFIER" "bf-ovr" 2>&1)"
echo "$OUT4" | grep -q "Trace provenance: ok" \
    && pass "caller-supplied window honored (trace now inside it)" \
    || fail "window override not honored: $(echo "$OUT4" | grep -i provenance)"
rm -rf "$T4"

# --- Test 5: exit code sourced from events, not from the foreign trace -------
echo ""
echo "Test 5: on mismatch the -1 branch reads the crash record's exit code"
T5="$(mktemp -d)"
make_fixture "$T5" "bf-src" "2026-08-17T11:06:29Z" 0 -1
OUT5="$(run_classifier "$T5" "bf-src")"
if echo "$OUT5" | grep -qE "^INFRASTRUCTURE$"; then
    pass "crash record exit_code -1 reached the -1 branch (bead not found -> not closed)"
elif echo "$OUT5" | grep -q "recovered via auto-retry"; then
    pass "crash record exit_code -1 reached the -1 branch (recovered path)"
else
    fail "events-sourced exit_code did not drive classification: $(echo "$OUT5" | grep -E '^(FALSE_POSITIVE|INFRASTRUCTURE|UNKNOWN|SERVICE_FAILURE)$')"
fi
rm -rf "$T5"

echo ""
echo "=============================================="
echo "Results: $passed passed, $failed failed"
echo "=============================================="
[ "$failed" -eq 0 ]
