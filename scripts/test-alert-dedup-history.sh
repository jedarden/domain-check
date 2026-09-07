#!/usr/bin/env bash
# Functional test for the 7-day crash-history deduplication window
# (alert-deduplication.sh `record` + `check` leg 4, and its wiring in
# crash-alert-manager.sh).
#
# Covers the dispatch acceptance criteria this suite exists for: an alert
# recorded for a crash target suppresses a fresh alert for the SAME target
# for DEDUP_WINDOW_DAYS days (default 7) and names that alert to reference,
# and a generated alert is recorded so the window has something to match.
#
# Fully hermetic: a fake `bead` on PATH answers `list --json` and `show`
# from fixtures, so no live bead store, timer, or network is touched and the
# verdicts cannot drift as the real store changes. The stub classifier makes
# the manager e2e cases deterministic (classification itself is tested by
# test-crash-alert-fixes.sh).
#
# Usage: scripts/test-alert-dedup-history.sh
# Exit codes: 0 all assertions pass, 1 at least one failed.

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEDUP_SCRIPT="$SCRIPT_DIR/alert-deduplication.sh"
CRASH_ALERT_MANAGER="$SCRIPT_DIR/crash-alert-manager.sh"

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

pass_count=0
fail_count=0

pass() {
    echo -e "${GREEN}✓ PASS${NC} - $1"
    pass_count=$((pass_count + 1))
}

fail() {
    echo -e "${RED}✗ FAIL${NC} - $1"
    fail_count=$((fail_count + 1))
}

now_iso() { date -u +"%Y-%m-%dT%H:%M:%SZ"; }
days_ago_iso() { date -u -d "$1 days ago" +"%Y-%m-%dT%H:%M:%SZ"; }

# History entry factory: history_entry <bead_id> <crash_bead_id> <recorded_at>
history_entry() {
    jq -c -n --arg bead "$1" --arg target "$2" --arg when "$3" \
        '{bead_id: $bead, crash_bead_id: (if $target == "" then null else $target end),
          crash_timestamp: $when, classification: "INFRASTRUCTURE", recorded_at: $when}'
}

SANDBOX="$(mktemp -d "${TMPDIR:-/tmp}/alert-dedup-history-XXXXXX")"
trap 'rm -rf "$SANDBOX"' EXIT
mkdir -p "$SANDBOX/scripts" "$SANDBOX/bin" "$SANDBOX/.beads/logs" "$SANDBOX/.beads/traces"

cp "$DEDUP_SCRIPT" "$SANDBOX/scripts/alert-deduplication.sh"
cp "$CRASH_ALERT_MANAGER" "$SANDBOX/scripts/crash-alert-manager.sh"

# Stub classifier: classification is out of scope here; the manager e2e cases
# only need the manager to reach the dedup gate and the alert-generation path.
printf '#!/usr/bin/env bash\necho "INFRASTRUCTURE"\n' > "$SANDBOX/scripts/crash-classifier.sh"
chmod +x "$SANDBOX/scripts/crash-classifier.sh"

# Fake bead CLI. `list --json` emits $SANDBOX/fixture-beads.jsonl; `show` emits
# a show-format block for the id in $2 from the same fixture. Everything else
# answers empty so an unexpected call fails visibly, not silently.
cat > "$SANDBOX/bin/bead" <<'SHIM'
#!/usr/bin/env bash
FIXTURE="$(dirname "$(dirname "$(command -v bead)")")/fixture-beads.jsonl"
case "${1:-}" in
    list)
        [[ -f "$FIXTURE" ]] && cat "$FIXTURE"
        exit 0
        ;;
    show)
        target_id="${2:-}"
        if [[ -f "$FIXTURE" ]]; then
            while IFS= read -r line; do
                [[ -z "$line" ]] && continue
                id=$(printf '%s' "$line" | jq -r '.id // empty')
                [[ "$id" == "$target_id" ]] || continue
                printf 'ID: %s\n' "$id"
                printf 'Title: %s\n' "$(printf '%s' "$line" | jq -r '.title // ""')"
                printf 'Status: %s\n' "$(printf '%s' "$line" | jq -r '.status // "open"')"
                exit 0
            done < "$FIXTURE"
        fi
        echo "ID: $target_id"
        echo "Status: unknown"
        exit 0
        ;;
    *)
        exit 0
        ;;
esac
SHIM
chmod +x "$SANDBOX/bin/bead"

HISTORY_FILE="$SANDBOX/.beads/logs/crash-history.jsonl"

# with_fixture <beads-jsonl-content> <command...>: run a copied script with the
# fake bead CLI and the given bead-store fixture.
with_fixture() {
    local beads="$1"
    shift
    printf '%s\n' "$beads" > "$SANDBOX/fixture-beads.jsonl"
    PATH="$SANDBOX/bin:$PATH" "$@"
}

run_dedup() {  # run_dedup <args...>  (env already set by caller)
    PATH="$SANDBOX/bin:$PATH" bash "$SANDBOX/scripts/alert-deduplication.sh" "$@"
}

ALERT_BEAD_FIXTURE='{"id":"bf-dupalert1","title":"ALERT: Agent crash on bead bf-windowtgt","status":"open"}
{"id":"bf-windowtgt","title":"Do the actual work","status":"open"}'

DERIVE_FIXTURE='{"id":"domchk-testalert2","title":"Investigate agent crash on bead bf-derivetgt","status":"open"}
{"id":"bf-derivetgt","title":"Do the actual work","status":"open"}'

echo "=========================================="
echo "Crash-History Dedup Window Test"
echo "=========================================="
echo ""

echo "--- record: writes the ledger the window reads ---"

# 1. record writes one entry carrying every field the acceptance criteria name.
rm -f "$HISTORY_FILE"
with_fixture "$ALERT_BEAD_FIXTURE" \
    bash "$SANDBOX/scripts/alert-deduplication.sh" \
    record bf-dupalert1 --target bf-windowtgt --classification INFRASTRUCTURE \
    --crash-ts "$(days_ago_iso 1)" >/dev/null 2>&1
RECORD_RC=$?
FIELDS_OK=$(jq -r 'select(.bead_id == "bf-dupalert1") |
    has("bead_id") and has("crash_bead_id") and has("crash_timestamp")
    and has("classification") and has("recorded_at")' "$HISTORY_FILE" 2>/dev/null | head -1)
if [[ $RECORD_RC -eq 0 && "$FIELDS_OK" == "true" && "$(wc -l < "$HISTORY_FILE")" -eq 1 ]]; then
    pass "record writes one JSONL entry with bead_id, crash_bead_id, crash_timestamp, classification, recorded_at"
else
    fail "record entry malformed or missing (rc=$RECORD_RC, fields=$FIELDS_OK)"
fi

# 2. record is idempotent per alert bead — the manager re-processes beads.
with_fixture "$ALERT_BEAD_FIXTURE" \
    bash "$SANDBOX/scripts/alert-deduplication.sh" \
    record bf-dupalert1 --target bf-windowtgt >/dev/null 2>&1
if [[ "$(wc -l < "$HISTORY_FILE")" -eq 1 ]]; then
    pass "re-recording the same alert bead does not stack a second entry"
else
    fail "record is not idempotent ($(wc -l < "$HISTORY_FILE") lines)"
fi

# 3. record derives the crash target from the title when --target is absent.
rm -f "$HISTORY_FILE"
with_fixture "$DERIVE_FIXTURE" \
    bash "$SANDBOX/scripts/alert-deduplication.sh" \
    record domchk-testalert2 --classification SERVICE_FAILURE >/dev/null 2>&1
DERIVED=$(jq -r 'select(.bead_id == "domchk-testalert2") | .crash_bead_id // "null"' "$HISTORY_FILE" 2>/dev/null)
if [[ "$DERIVED" == "bf-derivetgt" ]]; then
    pass "record derives crash_bead_id from the alert title (got $DERIVED)"
else
    fail "record did not derive the crash target (got $DERIVED)"
fi

# 4. an underivable target still records, with a null crash_bead_id.
rm -f "$HISTORY_FILE"
with_fixture "$DERIVE_FIXTURE" \
    bash "$SANDBOX/scripts/alert-deduplication.sh" \
    record domchk-testalert9 --classification UNKNOWN >/dev/null 2>&1
NULL_TARGET=$(jq -r 'select(.bead_id == "domchk-testalert9") | .crash_bead_id' "$HISTORY_FILE" 2>/dev/null)
if [[ "$NULL_TARGET" == "null" && -s "$HISTORY_FILE" ]]; then
    pass "underivable target records with null crash_bead_id (alert existence is never lost)"
else
    fail "underivable target was dropped or mis-recorded (got $NULL_TARGET)"
fi

echo ""
echo "--- check leg 4: the 7-day window ---"

# 5. A prior alert for the same crash target inside the window suppresses and
#    references that alert.
rm -f "$HISTORY_FILE"
history_entry bf-prior1 bf-windowtgt "$(days_ago_iso 2)" > "$HISTORY_FILE"
SUPPRESS_OUT=$(with_fixture "$ALERT_BEAD_FIXTURE" bash -c 'PATH="$1/bin:$PATH" bash "$1/scripts/alert-deduplication.sh" check bf-dupalert1 2>&1' _ "$SANDBOX")
SUPPRESS_RC=$?
if [[ $SUPPRESS_RC -eq 0 && "$SUPPRESS_OUT" == *"DUPLICATE"* && "$SUPPRESS_OUT" == *"bf-prior1"* && "$SUPPRESS_OUT" == *"7-day window"* ]]; then
    pass "fresh alert for a target alerted 2 days ago is suppressed, referencing bf-prior1"
else
    fail "in-window duplicate not suppressed (rc=$SUPPRESS_RC): $SUPPRESS_OUT"
fi

# 6. The same entry 8 days later no longer suppresses — the window expires.
rm -f "$HISTORY_FILE"
history_entry bf-prior1 bf-windowtgt "$(days_ago_iso 8)" > "$HISTORY_FILE"
EXPIRED_OUT=$(with_fixture "$ALERT_BEAD_FIXTURE" bash -c 'PATH="$1/bin:$PATH" bash "$1/scripts/alert-deduplication.sh" check bf-dupalert1 2>&1' _ "$SANDBOX")
EXPIRED_RC=$?
if [[ $EXPIRED_RC -eq 1 && "$EXPIRED_OUT" == *"PROCEED"* ]]; then
    pass "entry recorded 8 days ago is outside the window: check proceeds"
else
    fail "8-day-old entry still suppressed (rc=$EXPIRED_RC): $EXPIRED_OUT"
fi

# 7. DEDUP_WINDOW_DAYS is honored, so the default 7 is a policy, not a constant.
rm -f "$HISTORY_FILE"
history_entry bf-prior1 bf-windowtgt "$(days_ago_iso 2)" > "$HISTORY_FILE"
OVERRIDE_OUT=$(with_fixture "$ALERT_BEAD_FIXTURE" bash -c 'PATH="$1/bin:$PATH" DEDUP_WINDOW_DAYS=0 bash "$1/scripts/alert-deduplication.sh" check bf-dupalert1 2>&1' _ "$SANDBOX")
OVERRIDE_RC=$?
if [[ $OVERRIDE_RC -eq 1 && "$OVERRIDE_OUT" == *"PROCEED"* ]]; then
    pass "DEDUP_WINDOW_DAYS=0 override expires even a fresh entry"
else
    fail "DEDUP_WINDOW_DAYS override ignored (rc=$OVERRIDE_RC): $OVERRIDE_OUT"
fi

# 8. An alert is never a duplicate of itself (manager re-processing the same bead).
rm -f "$HISTORY_FILE"
history_entry bf-dupalert1 bf-windowtgt "$(now_iso)" > "$HISTORY_FILE"
SELF_OUT=$(with_fixture "$ALERT_BEAD_FIXTURE" bash -c 'PATH="$1/bin:$PATH" bash "$1/scripts/alert-deduplication.sh" check bf-dupalert1 2>&1' _ "$SANDBOX")
SELF_RC=$?
if [[ $SELF_RC -eq 1 && "$SELF_OUT" == *"PROCEED"* ]]; then
    pass "an alert does not suppress itself via its own history entry"
else
    fail "self-entry suppressed the alert (rc=$SELF_RC): $SELF_OUT"
fi

# 9. A corrupt history line fails open instead of crashing or suppressing.
rm -f "$HISTORY_FILE"
printf 'this is not json\n' > "$HISTORY_FILE"
CORRUPT_OUT=$(with_fixture "$ALERT_BEAD_FIXTURE" bash -c 'PATH="$1/bin:$PATH" bash "$1/scripts/alert-deduplication.sh" check bf-dupalert1 2>&1' _ "$SANDBOX")
CORRUPT_RC=$?
if [[ $CORRUPT_RC -eq 1 && "$CORRUPT_OUT" == *"PROCEED"* ]]; then
    pass "corrupt history line fails open (proceed, no crash)"
else
    fail "corrupt history line broke the scan (rc=$CORRUPT_RC): $CORRUPT_OUT"
fi

# 10. An entry with a null crash_bead_id never matches any target.
rm -f "$HISTORY_FILE"
history_entry bf-prior2 "" "$(now_iso)" > "$HISTORY_FILE"
NULL_OUT=$(with_fixture "$ALERT_BEAD_FIXTURE" bash -c 'PATH="$1/bin:$PATH" bash "$1/scripts/alert-deduplication.sh" check bf-dupalert1 2>&1' _ "$SANDBOX")
NULL_RC=$?
if [[ $NULL_RC -eq 1 && "$NULL_OUT" == *"PROCEED"* ]]; then
    pass "null crash_bead_id entry never suppresses a real target"
else
    fail "null-target entry over-suppressed (rc=$NULL_RC): $NULL_OUT"
fi

# 11. An entry for a DIFFERENT crash target does not suppress.
rm -f "$HISTORY_FILE"
history_entry bf-prior1 bf-othertgt "$(now_iso)" > "$HISTORY_FILE"
OTHER_OUT=$(with_fixture "$ALERT_BEAD_FIXTURE" bash -c 'PATH="$1/bin:$PATH" bash "$1/scripts/alert-deduplication.sh" check bf-dupalert1 2>&1' _ "$SANDBOX")
OTHER_RC=$?
if [[ $OTHER_RC -eq 1 && "$OTHER_OUT" == *"PROCEED"* ]]; then
    pass "entry for a different crash target does not suppress"
else
    fail "cross-target entry over-suppressed (rc=$OTHER_RC): $OTHER_OUT"
fi

echo ""
echo "--- manager wiring: gate before generation, record after ---"

# Shared manager fixture pieces: an open crash target and its trace slot.
setup_manager_case() {  # setup_manager_case <alert-bead-id>
    local alert_id="$1"
    rm -rf "$SANDBOX/.beads/traces/$alert_id"
    mkdir -p "$SANDBOX/.beads/traces/$alert_id"
    : > "$SANDBOX/.beads/traces/$alert_id/trace.jsonl"
    printf '{"bead_id":"%s","exit_code":-1,"captured_at":"2026-09-07T15:17:07.747106495Z"}\n' "$alert_id" \
        > "$SANDBOX/.beads/traces/$alert_id/metadata.json"
    rm -f "$SANDBOX/.beads/logs/alert-state.json"
}

run_manager() {  # run_manager <alert-bead-id>
    with_fixture "$ALERT_BEAD_FIXTURE" bash -c \
        'PATH="$1/bin:$PATH" bash "$1/scripts/crash-alert-manager.sh" "$2" 2>&1' _ "$SANDBOX" "$1"
}

# 12. Gate BEFORE generation: with the target already alerted on, the manager
#     skips creation and references the existing alert.
setup_manager_case bf-dupalert1
rm -f "$HISTORY_FILE"
history_entry bf-prior1 bf-windowtgt "$(days_ago_iso 1)" > "$HISTORY_FILE"
E2E_SUPPRESS_OUT=$(run_manager bf-dupalert1)
E2E_SUPPRESS_RC=$?
if [[ $E2E_SUPPRESS_RC -eq 0 \
      && "$E2E_SUPPRESS_OUT" == *"already covers crash target bf-windowtgt"* \
      && "$E2E_SUPPRESS_OUT" != *"Genuine crash detected"* ]] \
    && ! grep -q '"bead_id": "bf-dupalert1"' "$SANDBOX/.beads/logs/alert-state.json" 2>/dev/null
then
    pass "manager suppressed an in-window duplicate and referenced bf-prior1 (no alert created)"
else
    fail "manager generated or mishandled an in-window duplicate (rc=$E2E_SUPPRESS_RC)"
    echo "$E2E_SUPPRESS_OUT" | tail -5
fi

# 13. Record AFTER generation: with an empty ledger the alert fires and is then
#     recorded, so the NEXT alert for this target lands inside the window.
setup_manager_case bf-dupalert1
rm -f "$HISTORY_FILE"
E2E_UNIQUE_OUT=$(run_manager bf-dupalert1)
E2E_UNIQUE_RC=$?
RECORDED=$(jq -r 'select(.bead_id == "bf-dupalert1") |
    [.crash_bead_id, .classification, .crash_timestamp] | @tsv' "$HISTORY_FILE" 2>/dev/null)
if [[ $E2E_UNIQUE_RC -eq 1 && "$E2E_UNIQUE_OUT" == *"Genuine crash detected"* \
      && "$RECORDED" == "$(printf 'bf-windowtgt\tINFRASTRUCTURE\t2026-09-07T15:17:07Z')" ]]
then
    pass "generated alert was recorded with target, classification, and the trace's crash timestamp"
else
    fail "generated alert was not recorded correctly (rc=$E2E_UNIQUE_RC, recorded: ${RECORDED:-none})"
    echo "$E2E_UNIQUE_OUT" | tail -5
fi

# Summary
echo ""
echo "=========================================="
echo "Test Summary"
echo "=========================================="
if [[ $fail_count -eq 0 ]]; then
    echo -e "${GREEN}All $pass_count assertions passed.${NC}"
    echo ""
    echo "✅ Crash-history dedup window is properly implemented:"
    echo "   - record ledger (fields, idempotency, target derivation)"
    echo "   - 7-day window (suppress + reference / expiry / override)"
    echo "   - fail-open on corrupt or non-matching history"
    echo "   - manager wiring (check before generation, record after)"
    exit 0
else
    echo -e "${RED}$fail_count of $((pass_count + fail_count)) assertions failed.${NC}"
    exit 1
fi
