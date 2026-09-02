#!/usr/bin/env bash
# Alert Deduplication Script for Domain Check Crashes
# Purpose: Identify and report duplicate crash patterns to prevent alert fatigue
# Created: 2026-09-02

set -euo pipefail

BEAD_DIR=".beads"
LOG_DIR="$BEAD_DIR/logs"
ALERT_LOG="$LOG_DIR/alert-deduplication.log"
mkdir -p "$LOG_DIR"

log_alert() {
    echo "[$(date -Iseconds)] $*" | tee -a "$ALERT_LOG"
}

# Analyze crash patterns for duplicate alerts
analyze_duplicate_alerts() {
    echo "=== Alert Deduplication Analysis ===" | tee -a "$ALERT_LOG"

    # Check for recent crash traces
    if [[ ! -d "$BEAD_DIR/traces" ]]; then
        log_alert "No traces directory found - no crashes to analyze"
        return 0
    fi

    # Find all trace files
    local trace_files=$(find "$BEAD_DIR/traces" -name "trace.jsonl" -type f 2>/dev/null || true)

    if [[ -z "$trace_files" ]]; then
        log_alert "No trace files found - no crashes to analyze"
        return 0
    fi

    # Extract crash patterns and count duplicates
    declare -A crash_signatures
    declare -A bead_crash_counts

    while IFS= read -r trace_file; do
        local bead_id=$(basename "$(dirname "$trace_file")")

        # Count crashes per bead
        bead_crash_counts[$bead_id]=$((${bead_crash_counts[$bead_id]:-0} + 1))

        # Extract exit codes and create signature
        local exit_codes=$(grep -o '"exit_code":[0-9-]*' "$trace_file" 2>/dev/null | cut -d: -f2 | sort -u | tr '\n' ',' | sed 's/,$//')
        local timestamp=$(head -1 "$trace_file" | grep -o '"timestamp":"[^"]*"' | cut -d'"' -f4 | cut -dT -f1 | head -1)

        if [[ -n "$exit_codes" ]]; then
            local signature="${timestamp}:${exit_codes}"
            crash_signatures[$signature]=$((${crash_signatures[$signature]:-0} + 1))
        fi
    done <<< "$trace_files"

    # Report beads with multiple crashes (potential retry loops)
    local duplicate_found=0
    for bead_id in "${!bead_crash_counts[@]}"; do
        local count=${bead_crash_counts[$bead_id]}
        if [[ $count -ge 3 ]]; then
            log_alert "⚠️  DUPLICATE ALERT PATTERN: bead $bead_id crashed $count times"
            log_alert "   This may indicate retry loops or lack of deduplication"
            duplicate_found=1
        fi
    done

    # Report crash signature patterns
    for signature in "${!crash_signatures[@]}"; do
        local count=${crash_signatures[$signature]}
        if [[ $count -ge 5 ]]; then
            log_alert "⚠️  REPEATING CRASH SIGNATURE: $signature ($count occurrences)"
            duplicate_found=1
        fi
    done

    if [[ $duplicate_found -eq 0 ]]; then
        log_alert "✅ No duplicate alert patterns detected"
    fi

    echo "=== Analysis Complete ===" | tee -a "$ALERT_LOG"
}

# Generate deduplication recommendations
generate_recommendations() {
    echo "=== Deduplication Recommendations ===" | tee -a "$ALERT_LOG"

    local recommendations=(
        "1. Implement exponential backoff retry for transient failures (exit code 1 with HTTP 503/502)"
        "2. Add task completion detection to prevent post-completion retry loops"
        "3. Increase max turns limit for administrative tasks (bead closing, cleanup)"
        "4. Use non-interactive bead closing mode for agent operations"
        "5. Add pre-flight service health checks before starting tasks"
        "6. Monitor repository health and run git gc before tasks if repo > 1GB"
        "7. Implement alert aggregation to group identical alerts within time windows"
    )

    for rec in "${recommendations[@]}"; do
        echo "$rec" | tee -a "$ALERT_LOG"
    done

    echo "=== End Recommendations ===" | tee -a "$ALERT_LOG"
}

# Main execution
main() {
    log_alert "Starting alert deduplication analysis..."

    analyze_duplicate_alerts
    generate_recommendations

    log_alert "Alert deduplication analysis complete"
    echo ""
    echo "✅ Alert deduplication analysis logged to: $ALERT_LOG"
}

# Run main function
main "$@"
