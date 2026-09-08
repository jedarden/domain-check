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
# All phase sandboxes live under one root so a single trap cleans them all.
SB_ROOT="$(mktemp -d "${TMPDIR:-/tmp}/closed-bead-filter-XXXXXX")"
trap 'rm -rf "$SB_ROOT"' EXIT
SANDBOX="$SB_ROOT/phase1"
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

# ---------------------------------------------------------------------------
# Phase 2: the ALERT-bead shape (domchk-cd8ec29e).
#
# For an ALERT bead the bead being processed ($BEAD_ID) is the alert bead
# itself — Open, because it is the thing being investigated — and the real
# crash target is a *different* bead named in the title. Phase 1's shape
# (crashed bead == closed target) cannot cover that: fabricating it against
# the live store would mean mutating real beads, and which alert beads happen
# to be open on a given day is not something a test may depend on. These
# scenarios therefore run the manager against a stub `bead` on PATH that
# answers `show` for the fabricated ids and logs-and-refuses every other
# subcommand; the refusal log staying empty is itself an assertion that the
# suppression path attempts no store mutation.
#
# alert-deduplication.sh is deliberately absent except in the last scenario.
# The manager fails open around a missing dedup gate, so in that
# configuration suppression has to come from the closed-target gate itself —
# that is the gap this phase pins (before domchk-cd8ec29e an Open ALERT bead
# against a Closed target generated an alert, exit 1, in exactly this
# configuration, and was caught only by dedup when that gate was present).
# ---------------------------------------------------------------------------

new_alert_shape_sandbox() { # $1 = sandbox dir; dedup gate NOT copied
    local sb="$1"
    mkdir -p "$sb/scripts" "$sb/bin"
    for script in crash-alert-manager.sh crash-classifier.sh; do
        cp "$SCRIPT_DIR/$script" "$sb/scripts/$script"
        chmod +x "$sb/scripts/$script"
    done
}

write_stub_bead() { # $1=sandbox $2=alert id $3=alert title $4=target id $5=target status
    local sb="$1" alert="$2" title="$3" target="$4" tstatus="$5"
    cat > "$sb/bin/bead" <<STUB
#!/usr/bin/env bash
# Stub bead for the ALERT-shape scenarios: answers \`bead show\` for the ids
# the scenario fabricates; logs and refuses every other subcommand so the
# test can assert no store mutation was attempted.
if [[ "\$1" == "show" ]]; then
    case "\$2" in
        "$alert")
            printf 'ID: %s\nTitle: %s\nStatus: Open\nPriority: P2\n' "\$2" "$title"
            ;;
        "$target")
            printf 'ID: %s\nTitle: crash subject\nStatus: %s\nPriority: P2\n' "\$2" "$tstatus"
            ;;
        *)
            printf 'stub: no such bead: %s\n' "\$2" >&2
            exit 3
            ;;
    esac
    exit 0
fi
printf '%s refused: %s\n' "\$(date -u +%FT%TZ)" "\$*" >> "$sb/refused-bead-calls.log"
exit 4
STUB
    chmod +x "$sb/bin/bead"
}

run_alert_shape() { # $1=sandbox $2=alert bead id; sets ALERT_SHAPE_OUT / ALERT_SHAPE_RC
    local sb="$1" bead_id="$2"
    mkdir -p "$sb/.beads/traces/$bead_id"
    printf '{"bead_id":"%s","exit_code":-1,"captured_at":"2026-09-08T00:00:00Z"}\n' "$bead_id" \
        > "$sb/.beads/traces/$bead_id/metadata.json"
    printf 'synthetic trace for ALERT-shape scenario\n' > "$sb/.beads/traces/$bead_id/trace.jsonl"
    # The manager resolves its own dirs from its own path (absolute), but the
    # classifier it shells out to resolves .beads/traces relative to the CWD,
    # so the manager must run FROM the sandbox root - in a subshell, so this
    # suite's own cwd (which phase 1's live `bead show` premise depends on)
    # is untouched.
    set +e
    ALERT_SHAPE_OUT="$(cd "$sb" && PATH="$sb/bin:$PATH" bash "$sb/scripts/crash-alert-manager.sh" "$bead_id" 2>&1)"
    ALERT_SHAPE_RC=$?
    set -e
}

# Scenario A — the gap itself: Open ALERT bead whose title names a Closed
# target, no dedup gate. Must suppress (exit 0) from the closed-target gate.
ALERT_BEAD_A="domchk-alrtshapea"
TARGET_A="bf-shapea"
SANDBOX_A="$SB_ROOT/shape-a"
new_alert_shape_sandbox "$SANDBOX_A"
write_stub_bead "$SANDBOX_A" "$ALERT_BEAD_A" "ALERT: Agent crash on bead $TARGET_A" "$TARGET_A" "Closed"
run_alert_shape "$SANDBOX_A" "$ALERT_BEAD_A"

if [[ $ALERT_SHAPE_RC -eq 0 ]] \
    && grep -q "Reason: Target bead $TARGET_A is already closed" <<< "$ALERT_SHAPE_OUT"; then
    pass "Closed target suppresses the alert (exit 0) with no dedup gate present"
else
    fail "Open ALERT bead against Closed target not suppressed (rc=$ALERT_SHAPE_RC, expected 0)"
    echo "$ALERT_SHAPE_OUT" | tail -5
fi

if grep -q "is already CLOSED - no alert needed" <<< "$ALERT_SHAPE_OUT" \
    && ! grep -q "Genuine crash detected" <<< "$ALERT_SHAPE_OUT"; then
    pass "suppression came from the closed-target gate, not the alert path"
else
    fail "closed-target gate message absent (or the alert path ran)"
fi

if [[ -f "$SANDBOX_A/refused-bead-calls.log" ]]; then
    fail "stub refused calls on the closed-target path (store mutation attempted):"
    cat "$SANDBOX_A/refused-bead-calls.log"
else
    pass "no mutating bead call attempted on the closed-target path"
fi

if [[ ! -f "$SANDBOX_A/.beads/logs/alert-state.json" ]]; then
    pass "no alert state written for the closed-target suppression"
else
    fail "alert-state.json written despite closed-target suppression"
fi

# Scenario B — control: same shape but the target is still Open. The alert
# must still fire (exit 1); the new gate must not over-suppress.
ALERT_BEAD_B="domchk-alrtshapeb"
TARGET_B="bf-shapeb"
SANDBOX_B="$SB_ROOT/shape-b"
new_alert_shape_sandbox "$SANDBOX_B"
write_stub_bead "$SANDBOX_B" "$ALERT_BEAD_B" "ALERT: Agent crash on bead $TARGET_B" "$TARGET_B" "Open"
run_alert_shape "$SANDBOX_B" "$ALERT_BEAD_B"

if [[ $ALERT_SHAPE_RC -eq 1 ]] && grep -q "Genuine crash detected" <<< "$ALERT_SHAPE_OUT"; then
    pass "Open target still alerts (exit 1) - gate does not over-suppress"
else
    fail "Open-target control did not generate an alert (rc=$ALERT_SHAPE_RC, expected 1)"
    echo "$ALERT_SHAPE_OUT" | tail -5
fi

if [[ -f "$SANDBOX_B/refused-bead-calls.log" ]]; then
    fail "stub refused calls on the alert path (store mutation attempted):"
    cat "$SANDBOX_B/refused-bead-calls.log"
else
    pass "alert path made no mutating bead call"
fi

# Scenario C — control: a work bead with no crash target in the title. The
# closed-target gate must be inert (no target, no suppression), alert fires.
ALERT_BEAD_C="bf-shapeworkc"
SANDBOX_C="$SB_ROOT/shape-c"
new_alert_shape_sandbox "$SANDBOX_C"
write_stub_bead "$SANDBOX_C" "$ALERT_BEAD_C" "Fix flaky retry backoff in the bulk path" "none" "Open"
run_alert_shape "$SANDBOX_C" "$ALERT_BEAD_C"

if [[ $ALERT_SHAPE_RC -eq 1 ]] && grep -q "Genuine crash detected" <<< "$ALERT_SHAPE_OUT"; then
    pass "work bead with no title target still alerts (gate inert)"
else
    fail "no-target control did not generate an alert (rc=$ALERT_SHAPE_RC, expected 1)"
    echo "$ALERT_SHAPE_OUT" | tail -5
fi

if grep -q "already CLOSED" <<< "$ALERT_SHAPE_OUT"; then
    fail "closed-target gate fired without a target in the title"
else
    pass "closed-target gate silent when the title names no target"
fi

# Scenario D — precedence: dedup gate present AND target Closed. The manager
# fails open around a missing gate but must not need this one: the
# closed-target gate fires first, so the reason is its, not dedup's.
ALERT_BEAD_D="domchk-alrtshaped"
TARGET_D="bf-shaped"
SANDBOX_D="$SB_ROOT/shape-d"
new_alert_shape_sandbox "$SANDBOX_D"
cp "$SCRIPT_DIR/alert-deduplication.sh" "$SANDBOX_D/scripts/alert-deduplication.sh"
chmod +x "$SANDBOX_D/scripts/alert-deduplication.sh"
write_stub_bead "$SANDBOX_D" "$ALERT_BEAD_D" "ALERT: Agent crash on bead $TARGET_D" "$TARGET_D" "Closed"
run_alert_shape "$SANDBOX_D" "$ALERT_BEAD_D"

if [[ $ALERT_SHAPE_RC -eq 0 ]] \
    && grep -q "Reason: Target bead $TARGET_D is already closed" <<< "$ALERT_SHAPE_OUT"; then
    pass "closed-target gate fires ahead of the dedup gate (reason is FIX 1's, suppression no longer depends on dedup)"
else
    fail "with dedup present the reason was not the closed-target gate's (rc=$ALERT_SHAPE_RC)"
    echo "$ALERT_SHAPE_OUT" | tail -5
fi

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
