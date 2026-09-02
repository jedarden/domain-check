#!/usr/bin/env bash
# Pre-flight Health Check
# Run before starting agent tasks to ensure service availability
# Returns exit code 0 if healthy, 1 if unhealthy

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SERVICE_MONITOR="$SCRIPT_DIR/service-monitor.sh"

# Run the service monitor in check-only mode
bash "$SERVICE_MONITOR" --once

exit_code=$?

if [ $exit_code -eq 0 ]; then
    echo "✓ Pre-flight check passed"
    exit 0
else
    echo "✗ Pre-flight check failed"
    echo ""
    echo "Recommended actions:"
    echo "  1. Check inference gateway status"
    echo "  2. Wait for service recovery"
    echo "  3. Investigate gateway logs if service remains down"
    exit 1
fi
