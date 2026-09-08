#!/usr/bin/env bash
# Crash Alert Manager
# Purpose: Classify crashes, deduplicate alerts, and prevent alert fatigue
# Integrates crash classification with alert deduplication to filter false positives
# Created: 2026-09-02

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BEAD_DIR="$PROJECT_ROOT/.beads"
TRACE_DIR="$BEAD_DIR/traces"
LOG_DIR="$BEAD_DIR/logs"
ALERT_LOG="$LOG_DIR/crash-alert-manager.log"
CLASSIFIER_SCRIPT="$SCRIPT_DIR/crash-classifier.sh"
DEDUPE_SCRIPT="$SCRIPT_DIR/alert-deduplication.sh"

# Ensure log directory exists
mkdir -p "$LOG_DIR"

# Alert state tracking
ALERT_STATE_FILE="$LOG_DIR/alert-state.json"
ALERT_COOLDOWN_SECONDS=300  # 5 minutes cooldown for same alert type
PROCESSED_ALERTS_FILE="$LOG_DIR/processed-alerts.txt"  # Track processed alert beads

# Resolution tracking integration
RESOLUTION_TRACKER="$SCRIPT_DIR/crash-resolution-tracker.sh"

# Global alert-cooldown integration (domchk-7b404946): the outermost
# suppression gate — ONE 5-minute window for crash alerts of ANY
# classification, held in .beads/logs/crash-alert-metadata.json. The
# per-classification window below cannot see a cascade that arrives as mixed
# classifications and dedup keys on the crash target, so a fleet-wide event
# still fans out one ALERT bead per kill (the 2026-08-16 cascade reached 177
# crashes across 59 beads). This gate opens the window on the first alert,
# logs (without alerting) every crash that lands inside it, and attaches the
# suppressed-crash summary to the first alert after it closes.
COOLDOWN_SCRIPT="${COOLDOWN_SCRIPT:-$SCRIPT_DIR/alert-cooldown.sh}"

# System-event gate integration (bf-3561g RCA §6 recommendation 5, gap G-3):
# a system-wide crash/memory surge is exactly when alert fan-out does the most
# damage — during the 2026-08-16 cascade 57 of 59 window-crashing beads were
# ALERT beads. Before an alert is generated, the gate decides whether this
# event window may carry one more; exit 4 = the event already has an alert, so
# coalesce instead of fanning out into a new ALERT bead. (`check`, exit 75, is
# preflight-health-check.sh's leg of the same gate.)
SYSTEM_EVENT_GATE="${SYSTEM_EVENT_GATE:-$SCRIPT_DIR/system-event-mode.sh}"
SUPPRESSION_LOG="$LOG_DIR/system-event-suppressions.jsonl"

# Crash-storm circuit breaker integration (bf-65lsdu RCA §7):
# trips after N consecutive infrastructure crashes on the same bead, so the
# release-and-retry loop defers instead of re-dispatching a doomed task.
CIRCUIT_BREAKER="$SCRIPT_DIR/crash-circuit-breaker.sh"

# Initialize processed alerts file
if [[ ! -f "$PROCESSED_ALERTS_FILE" ]]; then
    touch "$PROCESSED_ALERTS_FILE"
fi

# Logging functions
log_alert() {
    local level="$1"
    shift
    local message="$*"
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    echo "[$timestamp] [$level] $message" | tee -a "$ALERT_LOG"
}

# Record a suppressed (coalesced) alert, so surge-time fan-out is visible in
# one place: how many alerts each event window absorbed, and for which beads.
record_system_event_suppression() { # $1 = alert-gate exit code
    local gate_rc="$1"
    mkdir -p "$LOG_DIR"
    # jq for JSONL; fall back to plain text if jq is unavailable or the gate
    # output contains something the JSON round-trip chokes on.
    if ! jq -n --arg ts "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
               --arg bead "$BEAD_ID" \
               --argjson gate_rc "$gate_rc" \
               --arg detail "$(printf '%s' "${SYSTEM_EVENT_OUTPUT:-}" | head -1)" \
               '{timestamp: $ts, bead_id: $bead, gate_exit: $gate_rc, detail: $detail}' \
            >>"$SUPPRESSION_LOG" 2>/dev/null; then
        printf '%s bead=%s gate_exit=%s %s\n' "$(date -u +"%Y-%m-%dT%H:%M:%SZ")" \
            "$BEAD_ID" "$gate_rc" "$(printf '%s' "${SYSTEM_EVENT_OUTPUT:-}" | head -1)" \
            >>"$SUPPRESSION_LOG"
    fi
}

# Usage
show_usage() {
    cat <<EOF
Usage: $0 <bead-id> [--classify-only] [--force-alert]

Processes crash alerts with classification and deduplication.

Arguments:
  bead-id          Bead ID to analyze

Options:
  --classify-only  Classify crash without generating alert
  --force-alert    Bypass deduplication and force alert generation
  -h, --help       Show this help message

Exit Codes:
  0  No alert needed (false positive, duplicate, cooldown, or suppressed
     by the system-event gate because the event window already has an alert)
  1  Alert generated (new genuine crash)
  2  Classification failed
  3  Error processing

System-Event Gate (scripts/system-event-mode.sh alert-gate):
  Consulted immediately before an alert is generated. While a system event
  is active, the FIRST alert of that event window is allowed and recorded in
  the gate's ledger; every further alert for the same event id is suppressed
  here (exit 0, recorded in $SUPPRESSION_LOG) instead of fanning out into a
  new ALERT bead. A missing or failing gate fails open, so crash alerting
  never depends on the gate being runnable.

Global Alert Cooldown (scripts/alert-cooldown.sh):
  The outermost suppression gate. The first alert of ANY classification opens
  a 5-minute global window (state: .beads/logs/crash-alert-metadata.json);
  crashes arriving while it is open are logged but not alerted (exit 0, no
  ALERT bead), and the first crash after it closes carries a summary of the
  suppressed crashes in its alert body. -f/--force bypasses the gate and
  still opens a new window.

Classification Types (from crash-classifier.sh):
  - FALSE_POSITIVE   Post-completion administrative failure
  - SERVICE_FAILURE  External service dependency failure
  - INFRASTRUCTURE   System resource exhaustion or infrastructure event
  - CODE_DEFECT      Actual application error or crash
  - UNKNOWN          Unable to classify

Crash-Storm Circuit Breaker:
  Every processed crash is recorded in scripts/crash-circuit-breaker.sh.
  After N consecutive infrastructure crashes (-1/137) on the same bead the
  breaker trips: repeat alerts are suppressed and the bead is deferred
  instead of being release-and-retried.

EOF
}

# Parse arguments
BEAD_ID=""
CLASSIFY_ONLY=false
FORCE_ALERT=false
AUTO_PROCESS=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --classify-only)
            CLASSIFY_ONLY=true
            shift
            ;;
        --force-alert)
            FORCE_ALERT=true
            shift
            ;;
        --auto-process)
            AUTO_PROCESS=true
            shift
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        -*)
            echo "ERROR: Unknown option: $1"
            show_usage
            exit 3
            ;;
        *)
            if [[ -z "$BEAD_ID" ]]; then
                BEAD_ID="$1"
            else
                echo "ERROR: Multiple bead IDs specified"
                show_usage
                exit 3
            fi
            shift
            ;;
    esac
done

# Auto-process mode: find and process all recent unprocessed crashes
if [[ "$AUTO_PROCESS" == true ]]; then
    log_alert "INFO" "Auto-processing recent crashes..."

    # Find crashes in the last hour
    RECENT_CRASHES=$(find "$TRACE_DIR" -name "metadata.json" -type f -mmin -60 2>/dev/null || true)

    if [[ -z "$RECENT_CRASHES" ]]; then
        log_alert "INFO" "No recent crashes found in last hour"
        exit 0
    fi

    PROCESSED_COUNT=0
    ALERTS_GENERATED=0
    FALSE_POSITIVES=0

    while IFS= read -r metadata_file; do
        bead_dir=$(dirname "$metadata_file")
        bead_id=$(basename "$bead_dir")

        # Skip if already processed (check for processed marker)
        if [[ -f "$bead_dir/.alert-processed" ]]; then
            continue
        fi

        # CRITICAL FIX 5: Check if bead is already CLOSED before processing
        BEAD_STATUS=$(bead show "$bead_id" 2>/dev/null | grep -i "status" | head -1 || echo "unknown")
        if [[ "$BEAD_STATUS" =~ [Cc]losed ]]; then
            log_alert "INFO" "Skipping crash: $bead_id (already closed)"
            touch "$bead_dir/.alert-processed"
            continue
        fi

        # CRITICAL FIX 6: Check if this was a successful completion (exit code 0)
        METADATA_FILE="$bead_dir/metadata.json"
        if [[ -f "$METADATA_FILE" ]]; then
            EXIT_CODE=$(grep -o '"exit_code":[0-9-]*' "$METADATA_FILE" 2>/dev/null | cut -d: -f2 | head -1)
            if [[ "$EXIT_CODE" == "0" ]]; then
                log_alert "INFO" "Skipping crash: $bead_id (exit code 0 - successful completion)"
                touch "$bead_dir/.alert-processed"
                continue
            fi
        fi

        log_alert "INFO" "Processing crash: $bead_id"

        # Process this bead
        if ALERT_OUTPUT=$("$0" "$bead_id" --classify-only 2>&1); then
            PROCESSED=$((PROCESSED + 1))

            # Check if this was a false positive
            if echo "$ALERT_OUTPUT" | grep -q "FALSE_POSITIVE"; then
                FALSE_POSITIVES=$((FALSE_POSITIVES + 1))
            fi

            # Mark as processed
            touch "$bead_dir/.alert-processed"
        fi
    done <<< "$RECENT_CRASHES"

    log_alert "INFO" "Auto-process complete: processed=$PROCESSED, false_positives=$FALSE_POSITIVES, alerts=$ALERTS_GENERATED"
    exit 0
fi

if [[ -z "$BEAD_ID" ]]; then
    echo "ERROR: Bead ID required"
    show_usage
    exit 3
fi

# Check if trace exists
if [[ ! -f "$TRACE_DIR/$BEAD_ID/trace.jsonl" ]]; then
    log_alert "ERROR" "Trace file not found for bead: $BEAD_ID"
    exit 3
fi

# CRASH-STORM CIRCUIT BREAKER: if the breaker is OPEN for this bead, the storm
# is already handled — suppress repeat alerts (converts a 127-alert storm into
# one alert and one deferral). Exit 4 from the breaker means blocked.
if [[ -x "$CIRCUIT_BREAKER" ]]; then
    set +e
    BREAKER_CHECK_OUTPUT=$("$CIRCUIT_BREAKER" check "$BEAD_ID" 2>&1)
    BREAKER_CHECK_RC=$?
    set -e
    if [[ $BREAKER_CHECK_RC -eq 4 ]]; then
        log_alert "WARN" "Circuit breaker OPEN for $BEAD_ID - suppressing repeat alert (crash storm in progress)"
        echo "Reason: Circuit breaker OPEN for $BEAD_ID (consecutive crash storm - bead deferred, not re-dispatched)"
        echo "$BREAKER_CHECK_OUTPUT"
        exit 0
    fi
else
    log_alert "WARN" "Circuit breaker not found or not executable - crash-storm protection inactive"
fi

# RESOLUTION TRACKING: Check if crash is already resolved
log_alert "INFO" "Checking resolution status for bead: $BEAD_ID"
if [[ -x "$RESOLUTION_TRACKER" ]]; then
    if RESOLUTION_CHECK=$("$RESOLUTION_TRACKER" "$BEAD_ID" check 2>&1); then
        if [[ "$RESOLUTION_CHECK" =~ RESOLVED ]]; then
            log_alert "INFO" "Crash $BEAD_ID is already resolved - no alert generated"
            echo "Reason: Crash already resolved"
            echo "$RESOLUTION_CHECK"
            exit 0
        fi
    fi
else
    log_alert "WARN" "Resolution tracker not found or not executable - skipping resolution check"
fi

# CRITICAL FIX 1: Check bead closure status BEFORE generating alert
log_alert "INFO" "Checking bead closure status for: $BEAD_ID"
BEAD_STATUS=$(bead show "$BEAD_ID" 2>/dev/null | grep -i "status" | head -1 || echo "unknown")

if [[ "$BEAD_STATUS" =~ [Cc]losed ]]; then
    log_alert "INFO" "Bead $BEAD_ID is already CLOSED - no alert needed"

    # Verify exit code from metadata
    EXIT_CODE=$(grep -o '"exit_code":[0-9-]*' "$TRACE_DIR/$BEAD_ID/metadata.json" 2>/dev/null | head -1 | cut -d: -f2)

    if [[ "$EXIT_CODE" == "0" ]]; then
        log_alert "INFO" "Bead completed successfully (exit code 0) - this is a false positive crash"
        echo "Reason: Bead already closed with exit code 0 (success)"
        exit 0
    else
        log_alert "INFO" "Bead closed with exit code $EXIT_CODE - may have completed work before crash"
        echo "Reason: Bead already closed (work may have completed before crash)"
        exit 0
    fi
fi

log_alert "INFO" "Processing crash alert for bead: $BEAD_ID (status: $BEAD_STATUS)"

# CRITICAL FIX 2: Check for existing alert beads for the same target bead
log_alert "INFO" "Checking for existing alert beads for target: $BEAD_ID"

# Extract target bead ID from current alert bead (if this is an alert bead)
TARGET_BEAD_ID=""
if [[ "$BEAD_ID" =~ ^bf-[a-z0-9]+$ ]]; then
    # This might be an alert bead, check if it references another bead
    BEAD_TITLE=$(bead show "$BEAD_ID" 2>/dev/null | grep -i "title" || echo "")

    if [[ "$BEAD_TITLE" =~ [Aa]gent[[:space:]-][Cc]rash[[:space:]-][Oo]n[[:space:]-]bf-[a-z0-9]+ ]]; then
        # Extract the original bead ID from the title
        TARGET_BEAD_ID=$(echo "$BEAD_TITLE" | grep -oP 'bf-[a-z0-9]+' | tail -1)
        log_alert "INFO" "Alert bead references target bead: $TARGET_BEAD_ID"
    fi
fi

# Check if we've already processed alerts for this target bead
if [[ -n "$TARGET_BEAD_ID" ]] && grep -q "$TARGET_BEAD_ID" "$PROCESSED_ALERTS_FILE" 2>/dev/null; then
    log_alert "INFO" "Alert already processed for target bead $TARGET_BEAD_ID - no alert generated"
    echo "Reason: Duplicate alert for already-processed crash"
    exit 0
fi

# CRITICAL FIX 1 (target leg): for an ALERT bead, $BEAD_ID is the alert bead
# itself — Open, because it is the thing being investigated — and the actual
# crash target is a different bead named in its title. The self-status gate
# above consults $BEAD_ID only, so every ALERT bead passed FIX 1 and an alert
# against an already-closed target travelled on toward the alert path;
# suppression for that shape came only from alert-deduplication.sh's
# target-resolution leg — a later gate, after classification and the breaker
# record, and one this manager deliberately fails open around when the script
# is missing (the bf-29rca shape: an ALERT bead for closed bf-4yjq, suppressed
# by dedup when present, generated outright when not). Consult the target's
# status here instead, ahead of classification and the breaker record, so the
# chain's premise — suppress alerts whose target bead is Closed — holds for
# the ALERT-bead shape (the dominant bead class in the trace store) without
# depending on any downstream gate being runnable.
#
# The title is parsed here rather than consumed from the extraction above:
# that block's historical `on<sep>bf-` pattern matched neither of needle's
# real alert-title shapes ("ALERT: Agent crash on bead bf-…", "Investigate
# agent crash on bead …" — the word "bead" sits between) and its ^bf- gate
# excluded domchk-* alert beads, so there TARGET_BEAD_ID came back empty for
# every real ALERT bead and a gate keyed on it would have been dead code.
# Parsing locally keeps this gate correct whatever the extraction above
# evolves into; when the extraction does supply a target, it wins.
if [[ -z "$TARGET_BEAD_ID" ]]; then
    ALERT_TITLE=$(bead show "$BEAD_ID" 2>/dev/null | grep -i "^title" || echo "")
    if [[ "$ALERT_TITLE" =~ [Cc]rash[[:space:]]+on[[:space:]]+(bead[[:space:]]+)?((bf|domchk)-[a-z0-9]+) ]]; then
        # BASH_REMATCH[2] is the bead id ([1] the optional "bead " prefix, [3]
        # the bf|domchk alternation). A bead is never its own crash target.
        if [[ "${BASH_REMATCH[2]}" != "$BEAD_ID" ]]; then
            TARGET_BEAD_ID="${BASH_REMATCH[2]}"
            log_alert "INFO" "Target-closure gate resolved crash target: $TARGET_BEAD_ID"
        fi
    fi
fi

if [[ -n "$TARGET_BEAD_ID" ]] && [[ "$TARGET_BEAD_ID" != "$BEAD_ID" ]]; then
    TARGET_STATUS=$(bead show "$TARGET_BEAD_ID" 2>/dev/null | grep -i "^status" | head -1 || echo "unknown")
    if [[ "$TARGET_STATUS" =~ [Cc]losed ]]; then
        log_alert "INFO" "Crash target $TARGET_BEAD_ID is already CLOSED - no alert needed"
        echo "Reason: Target bead $TARGET_BEAD_ID is already closed (nothing left to investigate)"
        exit 0
    fi
    if [[ "$TARGET_STATUS" == "unknown" ]]; then
        # Fail open: an unreadable target status must not stop crash alerting,
        # the same contract the resolution tracker and dedup gates follow.
        log_alert "WARN" "Could not read status of target bead $TARGET_BEAD_ID - target-closure gate fails open"
    fi
fi

# Also check if this exact alert bead has been processed
if grep -q "$BEAD_ID" "$PROCESSED_ALERTS_FILE" 2>/dev/null; then
    log_alert "INFO" "Alert bead $BEAD_ID already processed - no alert generated"
    echo "Reason: Already processed this alert bead"
    exit 0
fi

# Run crash classifier
log_alert "INFO" "Running crash classification..."
if ! CLASSIFICATION_OUTPUT=$($CLASSIFIER_SCRIPT "$BEAD_ID" 2>&1); then
    log_alert "ERROR" "Classification failed for bead $BEAD_ID"
    exit 2
fi

# CRITICAL FIX 4: Validate exit code before generating alert
# Exit code 0 means success, not a crash
EXIT_CODE=$(grep -o '"exit_code":[0-9-]*' "$TRACE_DIR/$BEAD_ID/metadata.json" 2>/dev/null | head -1 | cut -d: -f2)

if [[ "$EXIT_CODE" == "0" ]]; then
    log_alert "INFO" "Bead completed successfully (exit code 0) - no alert generated"
    echo "Reason: Exit code 0 indicates successful completion, not a crash"
    exit 0
fi

# CRASH-STORM CIRCUIT BREAKER: record the outcome. A success resets the
# consecutive-crash counter; an infrastructure crash (-1/137) advances it and
# trips the breaker at the threshold, converting the release-and-retry storm
# (bf-65lsdu: 127 identical doomed dispatches) into one deferral.
if [[ -x "$CIRCUIT_BREAKER" ]]; then
    BREAKER_EXIT_CODE="$EXIT_CODE"
    if [[ -z "$BREAKER_EXIT_CODE" ]]; then
        # metadata may be pretty-printed ("exit_code": -1) - extract space-tolerantly
        BREAKER_EXIT_CODE=$(grep -o '"exit_code":[ ]*[0-9-]*' "$TRACE_DIR/$BEAD_ID/metadata.json" 2>/dev/null | head -1 | grep -o '[0-9-]*$' || echo "")
    fi
    if [[ -n "$BREAKER_EXIT_CODE" ]]; then
        set +e
        BREAKER_RECORD_OUTPUT=$("$CIRCUIT_BREAKER" record "$BEAD_ID" "$BREAKER_EXIT_CODE" 2>&1)
        BREAKER_RECORD_RC=$?
        set -e
        if [[ $BREAKER_RECORD_RC -eq 1 ]]; then
            log_alert "WARN" "CIRCUIT BREAKER TRIPPED for $BEAD_ID after repeated consecutive crashes"
            log_alert "INFO" "$BREAKER_RECORD_OUTPUT"
            # Back off / defer instead of letting the retry loop re-dispatch
            if DEFER_OUTPUT=$("$CIRCUIT_BREAKER" defer "$BEAD_ID" 2>&1); then
                log_alert "ACTION" "Bead $BEAD_ID deferred by circuit breaker (backoff instead of re-dispatch)"
            else
                log_alert "WARN" "Deferral of $BEAD_ID failed (bead CLI) - breaker stays open, dispatch still blocked by breaker check"
            fi
        fi
    else
        log_alert "WARN" "Could not determine exit code for $BEAD_ID - circuit breaker not updated"
    fi
fi

# Extract classification type. Anchored token match, NOT head -1: the
# classifier prints the bare token as line 1, but taking the first line blind
# is how the '====' banner became CLASSIFICATION and left the FALSE_POSITIVE
# branch below dead (domchk-f6fff20f). The grep keeps this correct even if the
# classifier's preamble changes again. Falls back to head -1 only when no
# token is present at all, so an unparseable output still surfaces verbatim
# rather than silently becoming empty.
CLASSIFICATION=$(printf '%s\n' "$CLASSIFICATION_OUTPUT" | grep -m1 -E '^(FALSE_POSITIVE|SERVICE_FAILURE|INFRASTRUCTURE|CODE_DEFECT|UNKNOWN)[[:space:]]*$' || true)
CLASSIFICATION="${CLASSIFICATION:-$(printf '%s\n' "$CLASSIFICATION_OUTPUT" | head -1)}"
log_alert "INFO" "Classification: $CLASSIFICATION"

# Handle false positives - no alert needed
if [[ "$CLASSIFICATION" == "FALSE_POSITIVE" ]]; then
    log_alert "INFO" "False positive detected - marking as resolved"

    # Auto-mark as resolved since false positives are resolved crashes
    if [[ -x "$RESOLUTION_TRACKER" ]]; then
        "$RESOLUTION_TRACKER" "$BEAD_ID" mark-resolved >/dev/null 2>&1 || true
        log_alert "INFO" "Marked bead $BEAD_ID as resolved (false positive)"
    fi

    echo "Reason: $(echo "$CLASSIFICATION_OUTPUT" | grep "Reason:" | cut -d: -f2-)"
    exit 0
fi

# (Duplicate detection moved below, after the classify-only early-return: it
# is now the alert-deduplication.sh `check` gate for EVERY classification,
# instead of a SERVICE_FAILURE-only grep of a fleet-wide report's prose — the
# string-matching inversion recorded as gap D-6 in
# docs/alert-deduplication-gap-analysis-2026-09-07.md.)

# Check alert cooldown for same classification
if [[ "$FORCE_ALERT" != true ]] && [[ -f "$ALERT_STATE_FILE" ]]; then
    LAST_ALERT=$(jq -r --arg type "$CLASSIFICATION" '.recent[] | select(.classification == $type) | .timestamp' "$ALERT_STATE_FILE" 2>/dev/null | tail -1 || echo "")

    if [[ -n "$LAST_ALERT" ]]; then
        LAST_ALERT_SECONDS=$(date -d "$LAST_ALERT" +%s 2>/dev/null || echo "0")
        CURRENT_SECONDS=$(date +%s)
        ELAPSED=$((CURRENT_SECONDS - LAST_ALERT_SECONDS))

        if [[ $ELAPSED -lt $ALERT_COOLDOWN_SECONDS ]]; then
            log_alert "INFO" "Alert cooldown active for $CLASSIFICATION (${ELAPSED}s elapsed, ${ALERT_COOLDOWN_SECONDS}s required)"
            exit 0
        fi
    fi
fi

# Generate alert if we reach here
if [[ "$CLASSIFY_ONLY" == true ]]; then
    log_alert "INFO" "Classification complete (classify-only mode, no alert generated)"
    exit 0
fi

# DUPLICATE DETECTION — the per-alert dedup gate, for every classification.
# Keyed on the CRASH TARGET (from the alert title), not on this bead. Exit 0 =
# duplicate: the target is already resolved, an open alert already covers it,
# or an alert for the same crash target is in the 7-day crash-history window
# (.beads/logs/crash-history.jsonl) — the output names that alert so the
# investigation can reference it instead of fanning out. Exit 1 = unique.
# 2/3 (usage / indeterminate) fail open: crash alerting never depends on this
# gate being runnable.
if [[ "$FORCE_ALERT" != true ]]; then
    log_alert "INFO" "Running duplicate detection for bead: $BEAD_ID"
    set +e
    DEDUPE_OUTPUT=$("$DEDUPE_SCRIPT" check "$BEAD_ID" 2>&1)
    DEDUPE_RC=$?
    set -e
    if [[ $DEDUPE_RC -eq 0 ]]; then
        log_alert "INFO" "Duplicate detected for $BEAD_ID - no alert generated"
        echo "Reason: $DEDUPE_OUTPUT"
        exit 0
    elif [[ $DEDUPE_RC -ne 1 ]]; then
        log_alert "WARN" "Duplicate gate returned $DEDUPE_RC for $BEAD_ID - failing open"
        log_alert "WARN" "$DEDUPE_OUTPUT"
    fi
fi

# GLOBAL ALERT COOLDOWN (domchk-7b404946): exit 3 = a crash alert fired less
# than 5 minutes ago — this crash is LOGGED (cooldown state + audit log) but
# NOT alerted. Exit 0 with a COOLDOWN_EXPIRED block = the previous window
# suppressed crashes nobody has summarized yet; this alert carries their
# summary (the gate emits it exactly once and consumes it from the state).
# Anything else fails open: crash alerting never depends on this gate.
COOLDOWN_SUMMARY=""
if [[ -x "$COOLDOWN_SCRIPT" ]]; then
    if [[ "$FORCE_ALERT" != true ]]; then
        set +e
        COOLDOWN_OUTPUT=$("$COOLDOWN_SCRIPT" check 2>/dev/null)
        COOLDOWN_RC=$?
        set -e
        if [[ $COOLDOWN_RC -eq 3 ]]; then
            log_alert "INFO" "Alert cooldown active for $BEAD_ID — crash logged, not alerted"
            if [[ -n "$COOLDOWN_OUTPUT" ]]; then
                log_alert "INFO" "$COOLDOWN_OUTPUT"
            fi
            if ! "$COOLDOWN_SCRIPT" record-suppressed "$BEAD_ID" "$CLASSIFICATION" \
                    "suppressed by the global cooldown window" >/dev/null 2>&1; then
                log_alert "WARN" "Could not record suppressed crash $BEAD_ID in cooldown state"
            fi
            echo "Reason: global alert cooldown active — crash logged, not alerted (state: $LOG_DIR/crash-alert-metadata.json)"
            if [[ -n "$COOLDOWN_OUTPUT" ]]; then
                echo "$COOLDOWN_OUTPUT"
            fi
            exit 0
        elif [[ $COOLDOWN_RC -eq 0 ]]; then
            if grep -q "^COOLDOWN_EXPIRED" <<<"$COOLDOWN_OUTPUT"; then
                COOLDOWN_SUMMARY="$COOLDOWN_OUTPUT"
                log_alert "INFO" "Cooldown expired with suppressed crashes — summary attached to this alert"
            fi
        else
            log_alert "WARN" "Alert cooldown gate returned $COOLDOWN_RC for $BEAD_ID — failing open"
        fi
    fi
else
    log_alert "WARN" "Alert cooldown module not found or not executable ($COOLDOWN_SCRIPT) — global cooldown inactive"
fi

# SYSTEM-EVENT GATE (bf-3561g RCA §6 recommendation 5 / gap G-3): the last
# check before a new ALERT is generated. Exit 0 = this is the event's first
# alert (the gate records it in its ledger) or no event is active. Exit 4 =
# this event window already has an alert: record the suppression and coalesce
# instead of fanning out — that fan-out is what turned the 2026-08-16 cascade
# into 177 crashes across 59 beads.
if [[ -x "$SYSTEM_EVENT_GATE" ]]; then
    set +e
    SYSTEM_EVENT_OUTPUT=$("$SYSTEM_EVENT_GATE" alert-gate "$BEAD_ID" 2>&1)
    GATE_RC=$?
    set -e
    if [[ $GATE_RC -eq 4 ]]; then
        log_alert "WARN" "System event gate SUPPRESS for $BEAD_ID — alert coalesced into the event's existing investigation"
        record_system_event_suppression "$GATE_RC"
        echo "Reason: deferred: system event active — alert suppressed (coalesced; recorded in $SUPPRESSION_LOG)"
        echo "$SYSTEM_EVENT_OUTPUT"
        exit 0
    elif [[ $GATE_RC -eq 75 ]]; then
        # alert-gate's contract is 0 or 4 only — 75 is `check`'s defer code.
        # Handle it defensively anyway: an active event is precisely when a
        # new alert bead must not be created.
        log_alert "WARN" "System event gate returned 75 for $BEAD_ID (contract: 0 or 4) — suppressing anyway"
        record_system_event_suppression "$GATE_RC"
        echo "Reason: deferred: system event active — alert suppressed (gate returned defer 75)"
        echo "$SYSTEM_EVENT_OUTPUT"
        exit 0
    elif [[ $GATE_RC -eq 0 ]]; then
        log_alert "INFO" "System event gate ALLOW for $BEAD_ID (first alert of this event, or no event active)"
    else
        # Fail open: a broken or unreadable gate must not stop crash alerting.
        log_alert "WARN" "System event gate failed for $BEAD_ID (exit $GATE_RC) — failing open"
        if [[ -n "$SYSTEM_EVENT_OUTPUT" ]]; then
            log_alert "WARN" "$SYSTEM_EVENT_OUTPUT"
        fi
    fi
else
    log_alert "WARN" "System event gate not found or not executable ($SYSTEM_EVENT_GATE) — event coalescing inactive"
fi

# Generate alert
log_alert "ALERT" "Genuine crash detected: $BEAD_ID"
echo "Classification: $CLASSIFICATION"
echo "Timestamp: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
echo ""

# Show detailed classification output
echo "=== Classification Details ==="
echo "$CLASSIFICATION_OUTPUT"
echo ""

# End-of-cooldown summary (domchk-7b404946): the first alert after a window
# that suppressed crashes carries their summary — the gate emitted it exactly
# once and has already consumed it from the state.
if [[ -n "$COOLDOWN_SUMMARY" ]]; then
    echo "=== Cooldown Summary — crashes suppressed while the cooldown was active ==="
    echo "$COOLDOWN_SUMMARY"
    echo ""
fi

# Open the next cooldown window (domchk-7b404946): the alert that just fired
# starts a fresh 5-minute global cooldown during which further crashes are
# logged, not alerted. A failed record never fails the alert itself.
if [[ -x "$COOLDOWN_SCRIPT" ]]; then
    if ! "$COOLDOWN_SCRIPT" record-alert "$BEAD_ID" "$CLASSIFICATION" >/dev/null 2>&1; then
        log_alert "WARN" "Could not open the global cooldown window for $BEAD_ID"
    fi
fi

# Update alert state
ALERT_ENTRY=$(cat <<EOF
{
  "bead_id": "$BEAD_ID",
  "classification": "$CLASSIFICATION",
  "timestamp": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
  "trace_file": "$TRACE_DIR/$BEAD_ID/trace.jsonl"
}
EOF
)

# Initialize alert state file if needed
if [[ ! -f "$ALERT_STATE_FILE" ]]; then
    echo '{"recent":[]}' > "$ALERT_STATE_FILE"
fi

# Add alert entry and keep only last 50 alerts
# (.[-50:] and not tail(50): jq has no tail/1 builtin — the previous form was a
# compile error under `set -e`, so the state update killed the script and no
# alert ever reached its exit 1.)
jq --argjson new "$ALERT_ENTRY" '.recent |= (. + [$new] | .[-50:])' "$ALERT_STATE_FILE" > "$ALERT_STATE_FILE.tmp"
mv "$ALERT_STATE_FILE.tmp" "$ALERT_STATE_FILE"

# CRITICAL FIX 3: Mark this alert as processed to prevent future duplicates
echo "$(date -Iseconds) - $BEAD_ID${TARGET_BEAD_ID:+ (target: $TARGET_BEAD_ID)}" >> "$PROCESSED_ALERTS_FILE"
log_alert "INFO" "Alert bead $BEAD_ID marked as processed"

# CRASH HISTORY: append this generated alert to the 7-day dedup window ledger
# (.beads/logs/crash-history.jsonl). `alert-deduplication.sh check` leg 4 reads
# it back: the next alert bead for the same crash target is suppressed for
# DEDUP_WINDOW_DAYS days (default 7) and names this bead to reference instead
# of fanning out — the ledger leg that covers an alert investigated-and-closed
# inside the window, which the open-alert leg can no longer see. crash-ts is
# the trace's captured_at truncated to seconds (the closest record of the kill
# this repo owns; needle owns the kill-time event itself). A failed record
# never blocks the alert that already fired — the window is an optimization
# on top of the other three legs, not a dependency of them.
# Truncate captured_at to seconds. The closing quote must be stripped BEFORE
# the fraction rule: `\.[0-9]*Z$` anchors at end-of-string, so with the quote
# still present it never matched and the ledger stored full nanosecond
# timestamps (test-alert-dedup-history.sh case 13, fixed 2026-09-07
# domchk-fb636819).
CRASH_TS=$(grep -o '"captured_at": *"[^"]*"' "$TRACE_DIR/$BEAD_ID/metadata.json" 2>/dev/null | head -1 | sed 's/.*"captured_at": *"//; s/"$//; s/\.[0-9]*Z$/Z/' || true)
set +e
HISTORY_RECORD_OUTPUT=$("$DEDUPE_SCRIPT" record "$BEAD_ID" \
    --target "${TARGET_BEAD_ID:-}" \
    --classification "$CLASSIFICATION" \
    --crash-ts "${CRASH_TS:-$(date -u +"%Y-%m-%dT%H:%M:%SZ")}" 2>&1)
HISTORY_RECORD_RC=$?
set -e
if [[ $HISTORY_RECORD_RC -eq 0 ]]; then
    log_alert "INFO" "Crash history recorded: $HISTORY_RECORD_OUTPUT"
else
    log_alert "WARN" "Crash history record failed (exit $HISTORY_RECORD_RC) - the 7-day window will not cover this alert"
    log_alert "WARN" "$HISTORY_RECORD_OUTPUT"
fi

# Exit with alert code
log_alert "ALERT" "Alert generated for bead $BEAD_ID (classification: $CLASSIFICATION)"
exit 1
