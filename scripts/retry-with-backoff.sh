#!/usr/bin/env bash
# Retry wrapper with exponential backoff for transient failures
# Purpose: Prevent crash on temporary HTTP 503/502 errors from inference gateway
# Created: 2026-09-02
# Task: domchk-a05288ab

set -euo pipefail

# Configuration
MAX_RETRIES=${MAX_RETRIES:-5}
BASE_DELAY=${BASE_DELAY:-1}  # seconds
MAX_DELAY=${MAX_DELAY:-60}   # seconds
TRANSIENT_ERRORS="503|502|504|timeout|connection refused|temporarily unavailable"

# Usage
show_usage() {
    cat <<EOF
Usage: $0 <command> [args...]

Retries a command with exponential backoff on transient failures.

Environment Variables:
  MAX_RETRIES    Maximum retry attempts (default: 5)
  BASE_DELAY     Initial delay in seconds (default: 1)
  MAX_DELAY      Maximum delay between retries (default: 60)

Transient Errors (trigger retry):
  - HTTP status codes: 503, 502, 504
  - Error messages: timeout, connection refused, temporarily unavailable

Exit Codes:
  0  Command succeeded
  1  All retries exhausted
  2  Invalid usage
  3  Non-transient error (no retry)

Examples:
  # Retry inference API call
  $0 curl -sf https://inference-gateway/v1/models

  # Retry with custom settings
  MAX_RETRIES=3 BASE_DELAY=2 $0 ./script.sh

  # Use in pipelines
  $0 critical-operation.sh || handle_failure.sh

EOF
}

if [[ $# -eq 0 ]] || [[ "$1" == "-h" ]] || [[ "$1" == "--help" ]]; then
    show_usage
    exit 0
fi

# Function to check if error is transient
is_transient_error() {
    local output="$1"
    local exit_code="$2"

    # Check exit code for curl-like behavior
    if [[ $exit_code -eq 7 ]] || [[ $exit_code -eq 28 ]] || [[ $exit_code -eq 52 ]]; then
        return 0  # curl: (7) couldn't connect, (28) operation timeout, (52) empty reply
    fi

    # Check output for transient error patterns
    if echo "$output" | grep -qiE "$TRANSIENT_ERRORS"; then
        return 0
    fi

    return 1
}

# Function to calculate exponential backoff delay
calculate_delay() {
    local attempt="$1"
    local delay=$((BASE_DELAY * (2 ** (attempt - 1))))

    # Cap at MAX_DELAY
    if [[ $delay -gt $MAX_DELAY ]]; then
        echo $MAX_DELAY
    else
        echo $delay
    fi
}

# Main retry loop
LAST_OUTPUT=""
LAST_EXIT_CODE=0

for attempt in $(seq 1 $MAX_RETRIES); do
    # Run command
    if [[ $attempt -eq 1 ]]; then
        echo "[retry-with-backoff] Attempt $attempt/$MAX_RETRIES: $*"
    else
        echo "[retry-with-backoff] Retry $attempt/$MAX_RETRIES after ${delay}s delay: $*"
    fi

    # Capture output and exit code
    if LAST_OUTPUT=$("$@" 2>&1); then
        LAST_EXIT_CODE=0
        echo "[retry-with-backoff] ✓ Command succeeded on attempt $attempt/$MAX_RETRIES"
        echo "$LAST_OUTPUT"
        exit 0
    else
        LAST_EXIT_CODE=$?
        LAST_OUTPUT="$LAST_OUTPUT (exit code: $LAST_EXIT_CODE)"
    fi

    # Check if error is transient
    if is_transient_error "$LAST_OUTPUT" "$LAST_EXIT_CODE"; then
        if [[ $attempt -lt $MAX_RETRIES ]]; then
            delay=$(calculate_delay $attempt)
            echo "[retry-with-backoff] ℹ Transient error detected, waiting ${delay}s before retry..."
            echo "[retry-with-backoff] Error: $LAST_OUTPUT"

            sleep $delay
        else
            echo "[retry-with-backoff] ✗ All $MAX_RETRIES retries exhausted for transient failure"
            echo "[retry-with-backoff] Last error: $LAST_OUTPUT"
            exit 1
        fi
    else
        # Non-transient error - fail immediately
        echo "[retry-with-backoff] ✗ Non-transient error (no retry)"
        echo "[retry-with-backoff] Error: $LAST_OUTPUT"
        exit 3
    fi
done

# Should not reach here
echo "[retry-with-backoff] ✗ Unexpected error"
exit 1
