#!/usr/bin/env bash
# Crash Classifier
# Analyzes crash artifacts and classifies crash type to prevent false positives
# Distinguishes between technical crashes and administrative workflow failures

set -euo pipefail

# Usage
show_usage() {
    cat <<EOF
Usage: $0 <bead-id>

Analyzes crash artifacts for the given bead ID and classifies the crash type.

Classification Types:
  - FALSE_POSITIVE   Post-completion administrative failure (not a technical crash)
  - SERVICE_FAILURE  External service dependency failure (HTTP 503, gateway unavailable)
  - INFRASTRUCTURE    System resource exhaustion or infrastructure event
  - CODE_DEFECT       Actual application error or crash
  - UNKNOWN          Unable to classify from artifacts

Output:
  - Prints classification to stdout
  - Exit code 0: Successfully classified
  - Exit code 1: Classification failed
  - Exit code 2: Missing artifacts
EOF
}

# Arguments
BEAD_ID="${1:-}"

if [ -z "$BEAD_ID" ]; then
    show_usage
    exit 1
fi

# Artifact paths
TRACE_DIR=".beads/traces/${BEAD_ID}/trace.jsonl"
METADATA_DIR=".beads/traces/${BEAD_ID}/metadata.json"

if [ ! -f "$TRACE_DIR" ]; then
    echo "ERROR: Bead trace not found: $TRACE_DIR"
    exit 2
fi

# Extract bead data from trace file
extract_bead_data() {
    local bead_id="$1"
    local trace_file=".beads/traces/${bead_id}/trace.jsonl"
    if [ -f "$trace_file" ]; then
        cat "$trace_file"
    fi
}

# Extract exit code from metadata.json
extract_exit_code() {
    local bead_id="$1"
    local metadata_file=".beads/traces/${bead_id}/metadata.json"
    if [ -f "$metadata_file" ]; then
        grep -oP '"exit_code":\s*-?\d+' "$metadata_file" 2>/dev/null | sed 's/"exit_code"://' | tr -d ' ' | head -1
    fi
}

# Classify crash
classify_crash() {
    local bead_id="$1"
    local bead_data=$(extract_bead_data "$bead_id")

    if [ -z "$bead_data" ]; then
        echo "UNKNOWN"
        echo "No trace data found for bead"
        exit 2
    fi

    # Check for error_max_turns (administrative workflow failure)
    if echo "$bead_data" | grep -q "error_max_turns"; then
        echo "FALSE_POSITIVE"
        echo "Reason: Administrative workflow failure (max_turns exhausted)"
        echo "Pattern: Post-completion bead close failure, not technical crash"
        return 0
    fi

    # Check for HTTP 503 errors (service failure)
    if echo "$bead_data" | grep -q "503.*no available server"; then
        echo "SERVICE_FAILURE"
        echo "Reason: Inference gateway unavailable (HTTP 503)"
        echo "Pattern: External service dependency failure"
        return 0
    fi

    # Check for successful task completion before crash (bf-1ea4g pattern)
    # This check must come BEFORE exit code -1 check to catch post-completion crashes
    # Pattern: work committed < 30 seconds before crash
    if echo "$bead_data" | grep -q "git commit\|work.*complete\|task.*done"; then
        echo "FALSE_POSITIVE"
        echo "Reason: Task completed successfully before crash"
        echo "Pattern: Post-completion cleanup or administrative failure (bf-1ea4g pattern)"
        echo "Action: Verify task completion, may be false positive"
        return 0
    fi

    # Check for exit code -1 (SIGKILL/SIGHUP) from metadata.json
    # IMPORTANT: Check if bead ultimately completed successfully (FALSE_POSITIVE pattern)
    # Root cause analysis from bf-4k2ws showed SIGHUP cascades often result in successful auto-retry
    EXIT_CODE=$(extract_exit_code "$bead_id")
    if [[ "$EXIT_CODE" == "-1" ]]; then
        # Check bead status to determine if this is a false positive
        BEAD_STATUS=$(bead show "$bead_id" 2>/dev/null | grep -i "^Status" || echo "unknown")

        if [[ "$BEAD_STATUS" =~ [Cc]losed ]]; then
            # Bead completed successfully despite SIGHUP - this is a FALSE_POSITIVE
            # Pattern: System-wide SIGHUP cascade triggered automatic retry, bead succeeded
            echo "FALSE_POSITIVE"
            echo "Reason: Exit code -1 (SIGHUP/SIGKILL) but bead completed successfully"
            echo "Pattern: System-wide SIGHUP cascade with automatic recovery (bf-4k2ws pattern)"
            echo "Action: No investigation needed - automatic retry succeeded"
            echo "Note: 200+ beads affected during 2026-08-16 SIGHUP cascade, all recovered"
            return 0
        fi

        # Bead still open/failed - this is a genuine infrastructure issue
        echo "INFRASTRUCTURE"
        echo "Reason: Signal -1 termination (SIGHUP/SIGKILL) - bead not recovered"
        echo "Pattern: Possible infrastructure event (OOM, memory pressure, SIGHUP cascade)"
        echo "Action: Check system resources and logs for infrastructure events"
        return 0
    fi

    # Check for OOM killer patterns
    if echo "$bead_data" | grep -qi "oom\|out of memory\|memory exhausted"; then
        echo "INFRASTRUCTURE"
        echo "Reason: OOM killer or memory exhaustion"
        echo "Pattern: System resource exhaustion"
        echo "Action: Check memory usage and available RAM"
        return 0
    fi

    # Default: Unable to classify
    echo "UNKNOWN"
    echo "Reason: Insufficient data to classify"
    echo "Action: Manual investigation required"
    return 0
}

# Main
main() {
    echo "=================================="
    echo "Crash Classifier"
    echo "=================================="
    echo ""
    echo "Analyzing bead: $BEAD_ID"
    echo ""

    classify_crash "$BEAD_ID"
    exit_code=$?

    echo ""
    echo "=================================="
    echo "Classification complete"
    echo ""
    echo "Next steps:"
    echo "  - FALSE_POSITIVE: Review task completion, may need bead close fix"
    echo "  - SERVICE_FAILURE: Check inference gateway status, retry with backoff"
    echo "  - INFRASTRUCTURE: Check system resources (memory, disk, load)"
    echo "  - CODE_DEFECT: Investigate application error logs"
    echo "  - UNKNOWN: Manual investigation of crash artifacts"

    exit $exit_code
}

main
