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
  0  No alert needed (false positive, duplicate, or cooldown)
  1  Alert generated (new genuine crash)
  2  Classification failed
  3  Error processing

Classification Types (from crash-classifier.sh):
  - FALSE_POSITIVE   Post-completion administrative failure
  - SERVICE_FAILURE  External service dependency failure
  - INFRASTRUCTURE   System resource exhaustion or infrastructure event
  - CODE_DEFECT      Actual application error or crash
  - UNKNOWN          Unable to classify

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
        TRACE_FILE="$bead_dir/trace.jsonl"
        if [[ -f "$TRACE_FILE" ]]; then
            EXIT_CODES=$(grep -o '"exit_code":[0-9-]*' "$TRACE_FILE" 2>/dev/null | cut -d: -f2 | sort -u | tr '\n' ' ')
            if [[ "$EXIT_CODES" =~ "0" ]] && [[ ! "$EXIT_CODES" =~ "-" ]]; then
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

# CRITICAL FIX 1: Check bead closure status BEFORE generating alert
log_alert "INFO" "Checking bead closure status for: $BEAD_ID"
BEAD_STATUS=$(bead show "$BEAD_ID" 2>/dev/null | grep -i "status" | head -1 || echo "unknown")

if [[ "$BEAD_STATUS" =~ [Cc]losed ]]; then
    log_alert "INFO" "Bead $BEAD_ID is already CLOSED - no alert needed"

    # Verify exit code from trace
    EXIT_CODE=$(grep -o '"exit_code":[0-9-]*' "$TRACE_DIR/$BEAD_ID/trace.jsonl" 2>/dev/null | head -1 | cut -d: -f2)

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

    if [[ "$BEAD_TITLE" =~ [Aa]gent[ -][Cc]rash[ -]on[ -]bf-[a-z0-9]+ ]]; then
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
EXIT_CODES=$(grep -o '"exit_code":[0-9-]*' "$TRACE_DIR/$BEAD_ID/trace.jsonl" 2>/dev/null | cut -d: -f2 | sort -u | tr '\n' ' ')

if [[ "$EXIT_CODES" =~ "0" ]] && [[ ! "$EXIT_CODES" =~ "-" ]]; then
    log_alert "INFO" "Bead completed successfully (exit code 0) - no alert generated"
    echo "Reason: Exit code 0 indicates successful completion, not a crash"
    exit 0
fi

# Extract classification type
CLASSIFICATION=$(echo "$CLASSIFICATION_OUTPUT" | head -1)
log_alert "INFO" "Classification: $CLASSIFICATION"

# Handle false positives - no alert needed
if [[ "$CLASSIFICATION" == "FALSE_POSITIVE" ]]; then
    log_alert "INFO" "False positive detected - no alert generated"
    echo "Reason: $(echo "$CLASSIFICATION_OUTPUT" | grep "Reason:" | cut -d: -f2-)"
    exit 0
fi

# Handle service failures - check if deduplicated
if [[ "$CLASSIFICATION" == "SERVICE_FAILURE" ]]; then
    log_alert "INFO" "Service failure detected - checking for duplicates..."

    # Run deduplication check
    DEDUPE_OUTPUT=$($DEDUPE_SCRIPT 2>&1)
    DEDUPE_REASON=$(echo "$DEDUPE_OUTPUT" | grep -i "duplicate\|same.*pattern" || true)

    if [[ -n "$DEDUPE_REASON" ]] && [[ "$FORCE_ALERT" != true ]]; then
        log_alert "INFO" "Duplicate service failure detected - no alert generated"
        echo "Reason: $DEDUPE_REASON"
        exit 0
    fi
fi

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

# Generate alert
log_alert "ALERT" "Genuine crash detected: $BEAD_ID"
echo "Classification: $CLASSIFICATION"
echo "Timestamp: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
echo ""

# Show detailed classification output
echo "=== Classification Details ==="
echo "$CLASSIFICATION_OUTPUT"
echo ""

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
jq --argjson new "$ALERT_ENTRY" '.recent |= (. + [$new] | tail(50))' "$ALERT_STATE_FILE" > "$ALERT_STATE_FILE.tmp"
mv "$ALERT_STATE_FILE.tmp" "$ALERT_STATE_FILE"

# CRITICAL FIX 3: Mark this alert as processed to prevent future duplicates
echo "$(date -Iseconds) - $BEAD_ID${TARGET_BEAD_ID:+ (target: $TARGET_BEAD_ID)}" >> "$PROCESSED_ALERTS_FILE"
log_alert "INFO" "Alert bead $BEAD_ID marked as processed"

# Exit with alert code
log_alert "ALERT" "Alert generated for bead $BEAD_ID (classification: $CLASSIFICATION)"
exit 1
