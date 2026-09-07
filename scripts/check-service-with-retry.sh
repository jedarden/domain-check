#!/usr/bin/env bash
# Service health check with exponential backoff retry
# Purpose: Verify inference gateway availability before starting agent tasks
# Created: 2026-09-02
# Task: domchk-a05288ab

set -euo pipefail

# Configuration
GATEWAY_URL="${GATEWAY_URL:-https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health}"
TIMEOUT="${TIMEOUT:-5}"
MAX_RETRIES=${MAX_RETRIES:-3}
BASE_DELAY=${BASE_DELAY:-2}

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_status() {
    local level="$1"
    shift
    local message="$*"
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    case "$level" in
        INFO)
            echo -e "${GREEN}[${timestamp}] [INFO]${NC} $message"
            ;;
        WARN)
            echo -e "${YELLOW}[${timestamp}] [WARN]${NC} $message"
            ;;
        ERROR)
            echo -e "${RED}[${timestamp}] [ERROR]${NC} $message"
            ;;
        *)
            echo "[${timestamp}] [$level] $message"
            ;;
    esac
}

show_usage() {
    cat <<EOF
Usage: $0 [--silent] [--gateway-url URL]

Checks inference gateway health with exponential backoff retry.

Options:
  --silent          Suppress output, exit code only
  --gateway-url URL Custom gateway URL (default: \$GATEWAY_URL or traefik-apexalgo-iad)
  -h, --help        Show this help message

Environment Variables:
  GATEWAY_URL    Gateway health endpoint (default: traefik-apexalgo-iad health)
  TIMEOUT        HTTP timeout in seconds (default: 5)
  MAX_RETRIES    Maximum retry attempts (default: 3)
  BASE_DELAY     Initial delay in seconds (default: 2)

Exit Codes:
  0  Service is healthy
  1  All retries exhausted
  2  Invalid usage

Examples:
  # Check before starting agent task
  $0 && echo "Gateway healthy, starting task..."

  # Custom gateway with more retries
  GATEWAY_URL=https://custom-gateway/health MAX_RETRIES=5 $0

  # Silent check for scripts
  $0 --silent || exit 1

EOF
}

SILENT=false

while [[ $# -gt 0 ]]; do
    case $1 in
        --silent)
            SILENT=true
            shift
            ;;
        --gateway-url)
            GATEWAY_URL="$2"
            shift 2
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        *)
            echo "ERROR: Unknown option: $1"
            show_usage
            exit 2
            ;;
    esac
done

# Function to check health endpoint
check_health() {
    local attempt="$1"

    if [[ $SILENT == false ]]; then
        log_status INFO "Attempt $attempt: Checking gateway health: $GATEWAY_URL"
    fi

    # Use curl with timeout
    if curl -sf --max-time "$TIMEOUT" "$GATEWAY_URL" >/dev/null 2>&1; then
        if [[ $SILENT == false ]]; then
            log_status INFO "✓ Gateway is healthy"
        fi
        return 0
    else
        local exit_code=$?
        if [[ $SILENT == false ]]; then
            log_status WARN "✗ Gateway unavailable (curl exit code: $exit_code)"
        fi
        return 1
    fi
}

# Retry with exponential backoff
for attempt in $(seq 1 $MAX_RETRIES); do
    if check_health "$attempt"; then
        exit 0
    fi

    if [[ $attempt -lt $MAX_RETRIES ]]; then
        local delay=$((BASE_DELAY * (2 ** (attempt - 1))))
        if [[ $SILENT == false ]]; then
            log_status INFO "Retrying in ${delay}s..."
        fi
        sleep "$delay"
    fi
done

# All retries exhausted
if [[ $SILENT == false ]]; then
    log_status ERROR "All $MAX_RETRIES retries exhausted - Gateway unavailable"
    log_status ERROR "This may cause agent tasks to fail with HTTP 503 errors"
    log_status ERROR "Recommended actions:"
    log_status ERROR "  1. Check if traefik-apexalgo-iad cluster is healthy"
    log_status ERROR "  2. Verify Tailscale connectivity: kubectl --server=http://traefik-apexalgo-iad:8001 get pods"
    log_status ERROR "  3. Check inference gateway pod status"
fi

exit 1
