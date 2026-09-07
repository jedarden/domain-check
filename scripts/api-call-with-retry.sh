#!/usr/bin/env bash
# API call wrapper with exponential backoff retry
# Purpose: Execute curl/HTTP calls with automatic retry on transient failures
# Created: 2026-09-02
# Task: domchk-a05288ab

set -euo pipefail

# Import retry logic
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
RETRY_SCRIPT="$SCRIPT_DIR/retry-with-backoff.sh"

# Configuration
TIMEOUT="${TIMEOUT:-30}"
MAX_RETRIES="${MAX_RETRIES:-5}"
BASE_DELAY="${BASE_DELAY:-1}"

show_usage() {
    cat <<EOF
Usage: $0 <curl-options-and-args>

Wrapper for curl/HTTP calls with automatic retry on transient failures.

All arguments are passed directly to curl, with automatic retry on:
  - HTTP 503 (Service Unavailable)
  - HTTP 502 (Bad Gateway)
  - HTTP 504 (Gateway Timeout)
  - Connection timeouts
  - Connection refused errors

Environment Variables:
  TIMEOUT        HTTP timeout in seconds (default: 30)
  MAX_RETRIES    Maximum retry attempts (default: 5)
  BASE_DELAY     Initial delay in seconds (default: 1)

Exit Codes:
  0  Request succeeded
  1  All retries exhausted
  2  Invalid usage

Examples:
  # GET request with automatic retry
  $0 -sf https://api.example.com/data

  # POST request with JSON payload
  $0 -X POST -H "Content-Type: application/json" \\
     -d '{"key": "value"}' \\
     https://api.example.com/endpoint

  # Custom timeout and retries
  TIMEOUT=10 MAX_RETRIES=3 $0 https://api.example.com/health

Notes:
  - Use -s flag for silent mode (no progress meter)
  - Use -f flag to fail on HTTP errors (4xx, 5xx)
  - All curl options are supported (see man curl)

EOF
}

if [[ $# -eq 0 ]] || [[ "$1" == "-h" ]] || [[ "$1" == "--help" ]]; then
    show_usage
    exit 0
fi

# Check if retry script exists
if [[ ! -x "$RETRY_SCRIPT" ]]; then
    echo "ERROR: Retry script not found or not executable: $RETRY_SCRIPT"
    exit 2
fi

# Build curl command with timeout
CURL_CMD="curl --max-time $TIMEOUT"

# Execute with retry wrapper
MAX_RETRIES=$MAX_RETRIES BASE_DELAY=$BASE_DELAY "$RETRY_SCRIPT" $CURL_CMD "$@"
