#!/usr/bin/env bash
# Service Availability Monitor
# Checks critical external services and reports availability
# Used as pre-flight check before starting agent tasks

set -euo pipefail

# Configuration
INFERENCE_GATEWAY="https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health"
TIMEOUT=5
RETRIES=3
RETRY_DELAY=2

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Logging
log_info() {
    echo -e "${GREEN}[INFO]${NC} $1"
}

log_warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Check inference gateway with retry logic
check_inference_gateway() {
    local attempt=1
    local status_code="000"
    local response_body=""

    while [ $attempt -le $RETRIES ]; do
        log_info "Checking inference gateway (attempt $attempt/$RETRIES)..."

        # Capture HTTP status code and body
        response=$(curl -s -w "\n%{http_code}" -m "$TIMEOUT" "$INFERENCE_GATEWAY" 2>&1 || echo "000")
        status_code=$(echo "$response" | tail -1)
        response_body=$(echo "$response" | head -n -1)

        if [ "$status_code" = "200" ]; then
            log_info "✓ Inference gateway: HEALTHY (HTTP 200)"
            return 0
        elif [ "$status_code" = "503" ]; then
            log_warn "✗ Inference gateway: UNAVAILABLE (HTTP 503 - no available server)"
            if [ $attempt -lt $RETRIES ]; then
                log_info "Retrying in ${RETRY_DELAY}s..."
                sleep $RETRY_DELAY
            fi
        elif [ "$status_code" = "000" ]; then
            log_error "✗ Inference gateway: CONNECTION FAILED (timeout or network error)"
            if [ $attempt -lt $RETRIES ]; then
                log_info "Retrying in ${RETRY_DELAY}s..."
                sleep $RETRY_DELAY
            fi
        else
            log_warn "✗ Inference gateway: UNEXPECTED STATUS (HTTP $status_code)"
            if [ $attempt -lt $RETRIES ]; then
                log_info "Retrying in ${RETRY_DELAY}s..."
                sleep $RETRY_DELAY
            fi
        fi

        attempt=$((attempt + 1))
    done

    log_error "Inference gateway: UNHEALTHY after $RETRIES attempts"
    return 1
}

# Check system resources
check_system_resources() {
    log_info "Checking system resources..."

    # Memory check
    local avail_mem_gb=$(free -g | awk '/^Mem:/{print $7}')
    if [ "$avail_mem_gb" -lt 10 ]; then
        log_warn "⚠ Low memory: ${avail_mem_gb}GB available (<10GB threshold)"
    else
        log_info "✓ Memory: ${avail_mem_gb}GB available"
    fi

    # Disk check
    local avail_disk_gb=$(df -BG / | tail -1 | awk '{print $4}' | sed 's/G//')
    if [ "$avail_disk_gb" -lt 20 ]; then
        log_warn "⚠ Low disk space: ${avail_disk_gb}GB free (<20GB threshold)"
    else
        log_info "✓ Disk: ${avail_disk_gb}GB free"
    fi

    # Load average
    local load_1min=$(uptime | awk -F'load average:' '{print $2}' | awk '{print $1}' | sed 's/,//')
    local load_int=$(echo "$load_1min" | cut -d. -f1)
    if [ "$load_int" -gt 10 ]; then
        log_warn "⚠ High load: ${load_1min} (1-minute average)"
    else
        log_info "✓ Load: ${load_1min} (1-minute average)"
    fi
}

# Main function
main() {
    echo "=================================="
    echo "Service Availability Monitor"
    echo "=================================="
    echo ""

    local gateway_healthy=true

    # Check inference gateway
    if ! check_inference_gateway; then
        gateway_healthy=false
    fi

    echo ""

    # Check system resources
    check_system_resources

    echo ""
    echo "=================================="

    if [ "$gateway_healthy" = false ]; then
        log_error "PRE-FLIGHT CHECK FAILED: Inference gateway is unavailable"
        log_error "Recommendation: Wait for service recovery or investigate gateway status"
        echo ""
        echo "Manual gateway check:"
        echo "  curl -sf --max-time 5 $INFERENCE_GATEWAY || echo 'Gateway down'"
        exit 1
    else
        log_info "PRE-FLIGHT CHECK PASSED: All services healthy"
        exit 0
    fi
}

# Run if executed directly
if [ "${BASH_SOURCE[0]}" = "${0}" ]; then
    main "$@"
fi
