#!/bin/bash
# Crash Alert System Improvements
# Prevents false positive alerts and duplicate investigations

set -euo pipefail

ALERT_LOG_DIR="$HOME/.beads/logs"
ALERT_LOG="$ALERT_LOG_DIR/crash-alerts.log"
RECENT_ALERTS_WINDOW="300"  # 5 minutes in seconds

# Ensure log directory exists
mkdir -p "$ALERT_LOG_DIR"

# Function to check if bead is already closed
check_bead_closed() {
    local bead_id="$1"

    if bead show "$bead_id" 2>/dev/null | grep -q "Status: Closed"; then
        echo "CLOSED"
        return 0
    fi

    echo "OPEN"
    return 1
}

# Function to check for duplicate alerts
check_duplicate_alert() {
    local bead_id="$1"
    local crash_timestamp="$2"

    if [ ! -f "$ALERT_LOG" ]; then
        touch "$ALERT_LOG"
        return 1  # No duplicates if log doesn't exist
    fi

    # Create alert key: bead_id + timestamp window (5 min buckets)
    local timestamp_bucket=$(($(date -d "$crash_timestamp" +%s) / RECENT_ALERTS_WINDOW))
    local alert_key="${bead_id}_${timestamp_bucket}"

    if grep -q "$alert_key" "$ALERT_LOG"; then
        echo "DUPLICATE"
        return 0
    fi

    # Log this alert
    echo "$alert_key" >> "$ALERT_LOG"

    # Clean old entries (older than 1 hour)
    local current_time=$(date +%s)
    local cutoff_time=$((current_time - 3600))

    tempfile=$(mktemp)
    while IFS= read -r line; do
        local line_time=$(echo "$line" | cut -d_ -f2)
        if [ "$line_time" -ge "$cutoff_time" ]; then
            echo "$line" >> "$tempfile"
        fi
    done < "$ALERT_LOG"
    mv "$tempfile" "$ALERT_LOG"

    echo "NEW"
    return 1
}

# Function to validate crash timestamp vs completion timestamp
validate_timestamp_consistency() {
    local bead_id="$1"
    local crash_timestamp="$2"

    # Get completion timestamp if bead is closed
    local completion_timestamp=$(bead show "$bead_id" 2>/dev/null | grep "Updated:" | cut -d: -f2- | xargs || echo "")

    if [ -z "$completion_timestamp" ]; then
        echo "UNKNOWN"
        return 2
    fi

    # Convert to epoch seconds for comparison
    local crash_epoch=$(date -d "$crash_timestamp" +%s 2>/dev/null || echo "0")
    local completion_epoch=$(date -d "$completion_timestamp" +%s 2>/dev/null || echo "0")

    if [ "$crash_epoch" -lt "$completion_epoch" ]; then
        echo "INCONSISTENT"
        return 0
    fi

    echo "CONSISTENT"
    return 1
}

# Function to detect system-wide crash patterns
detect_system_wide_event() {
    local crash_count="$1"
    local time_window_minutes="$2"

    if [ "$crash_count" -gt 10 ] && [ "$time_window_minutes" -lt 10 ]; then
        echo "SYSTEM_WIDE"
        return 0
    fi

    echo "ISOLATED"
    return 1
}

# Function to classify crash by exit code
classify_crash() {
    local exit_code="$1"

    case "$exit_code" in
        -1)
            echo "INFRASTRUCTURE"
            return 0
            ;;
        1)
            echo "APPLICATION"
            return 0
            ;;
        137)
            echo "OOM_KILLED"
            return 0
            ;;
        0)
            echo "SUCCESS"
            return 0
            ;;
        *)
            echo "UNKNOWN"
            return 1
            ;;
    esac
}

# Main validation function for crash alerts
validate_crash_alert() {
    local bead_id="$1"
    local crash_timestamp="$2"
    local exit_code="$3"

    echo "=== Validating crash alert for bead $bead_id ==="

    # Check 1: Is bead already closed?
    local bead_status=$(check_bead_closed "$bead_id")
    if [ "$bead_status" = "CLOSED" ]; then
        echo "❌ FALSE POSITIVE: Bead $bead_id is already CLOSED"
        echo "   Crash alerts should not be created for closed beads"
        return 1
    fi
    echo "✓ Bead status check: OPEN"

    # Check 2: Is this a duplicate alert?
    local duplicate_status=$(check_duplicate_alert "$bead_id" "$crash_timestamp")
    if [ "$duplicate_status" = "DUPLICATE" ]; then
        echo "❌ DUPLICATE ALERT: Crash alert for $bead_id already exists in recent window"
        echo "   Skipping duplicate investigation"
        return 2
    fi
    echo "✓ Duplicate check: NEW alert"

    # Check 3: Is crash timestamp consistent with completion?
    local timestamp_status=$(validate_timestamp_consistency "$bead_id" "$crash_timestamp")
    if [ "$timestamp_status" = "INCONSISTENT" ]; then
        echo "❌ FALSE POSITIVE: Crash timestamp before completion timestamp"
        echo "   Crash occurred before bead completion - impossible"
        return 3
    fi
    echo "✓ Timestamp consistency: CONSISTENT"

    # Check 4: Classify crash type
    local crash_class=$(classify_crash "$exit_code")
    echo "✓ Crash classification: $crash_class (exit code $exit_code)"

    # If exit code is 0, it's not a crash
    if [ "$crash_class" = "SUCCESS" ]; then
        echo "❌ FALSE POSITIVE: Exit code 0 indicates successful completion"
        return 4
    fi

    echo "✅ VALIDATION PASSED: Crash alert is legitimate"
    return 0
}

# Export functions for use by other scripts
export -f check_bead_closed
export -f check_duplicate_alert
export -f validate_timestamp_consistency
export -f detect_system_wide_event
export -f classify_crash
export -f validate_crash_alert

# If run directly, perform validation
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    if [ "$#" -lt 3 ]; then
        echo "Usage: $0 <bead_id> <crash_timestamp> <exit_code>"
        echo "Example: $0 bf-4k2ws '2026-08-16T17:21:28Z' -1"
        exit 1
    fi

    validate_crash_alert "$@"
fi
