#!/usr/bin/env bash
# Pre-dispatch CPU load check
# Use this script before dispatching heavy operations to ensure system has capacity
# Exit code 0 = OK to dispatch, Exit code 1 = CPU too saturated, defer dispatch

set -euo pipefail

# Configuration
CPU_WARNING_THRESHOLD=80   # Warning at 80% CPU utilization
CPU_CRITICAL_THRESHOLD=90  # Critical at 90% CPU utilization
MAX_REQUEUE_DELAY=300       # Maximum requeue delay (5 minutes)

# Colors for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

echo_check() {
    local status=$1
    local message=$2
    case $status in
        OK)
            echo -e "${GREEN}✓ OK${NC}: $message"
            ;;
        WARN)
            echo -e "${YELLOW}⚠ WARNING${NC}: $message"
            ;;
        CRITICAL)
            echo -e "${RED}✗ CRITICAL${NC}: $message"
            ;;
    esac
}

# Get system metrics
CPU_CORES=$(nproc)
LOAD_1MIN=$(awk '{print $1}' /proc/loadavg)
LOAD_5MIN=$(awk '{print $2}' /proc/loadavg)
LOAD_15MIN=$(awk '{print $3}' /proc/loadavg)

# Calculate CPU utilization percentages
LOAD_PERCENT_1=$(awk "BEGIN {printf \"%.1f\", ($LOAD_1MIN/$CPU_CORES)*100}")
LOAD_PERCENT_5=$(awk "BEGIN {printf \"%.1f\", ($LOAD_5MIN/$CPU_CORES)*100}")
LOAD_PERCENT_15=$(awk "BEGIN {printf \"%.1f\", ($LOAD_15MIN/$CPU_CORES)*100}")

# Get memory info
MEM_TOTAL=$(grep MemTotal /proc/meminfo | awk '{print $2}')
MEM_AVAILABLE=$(grep MemAvailable /proc/meminfo | awk '{print $2}')
MEM_PERCENT_AVAIL=$(awk "BEGIN {printf \"%.1f\", ($MEM_AVAILABLE/$MEM_TOTAL)*100}")

# Get swap usage
SWAP_TOTAL=$(grep SwapTotal /proc/meminfo | awk '{print $2}')
SWAP_FREE=$(grep SwapFree /proc/meminfo | awk '{print $2}')
if [ "$SWAP_TOTAL" -gt 0 ]; then
    SWAP_PERCENT_USED=$(awk "BEGIN {printf \"%.1f\", 100 - (($SWAP_FREE/$SWAP_TOTAL)*100)}")
else
    SWAP_PERCENT_USED=0
fi

# Output header
echo "=== CPU Load Check ==="
echo "Timestamp: $(date -Iseconds)"
echo ""

# Display CPU metrics
echo "CPU Metrics:"
echo "  Cores: $CPU_CORES"
echo "  Load Averages: ${LOAD_1MIN} (1m), ${LOAD_5MIN} (5m), ${LOAD_15MIN} (15m)"
echo "  Utilization: ${LOAD_PERCENT_1}% (1m), ${LOAD_PERCENT_5}% (5m), ${LOAD_PERCENT_15}% (15m)"
echo ""

# Display memory metrics
echo "Memory Metrics:"
echo "  Total: $((MEM_TOTAL / 1024 / 1024)) GB"
echo "  Available: $((MEM_AVAILABLE / 1024 / 1024)) GB (${MEM_PERCENT_AVAIL}%)"
echo "  Swap Usage: ${SWAP_PERCENT_USED}%"
echo ""

# Determine status
if [ "$(awk "BEGIN {print ($LOAD_1MIN/$CPU_CORES) >= ($CPU_CRITICAL_THRESHOLD/100)}")" = "1" ]; then
    STATUS="CRITICAL"
    echo_check CRITICAL "CPU utilization at ${LOAD_PERCENT_1}% (above ${CPU_CRITICAL_THRESHOLD}% threshold)"
    echo ""
    echo "Recommendation: DEFER dispatch - CPU is critically saturated"
    echo "System cannot handle additional load without risking process termination"
    echo ""
    echo "Suggested actions:"
    echo "  1. Wait for load to decrease (check again in 30-60 seconds)"
    echo "  2. Identify and defer non-critical background work"
    echo "  3. Coordinate with fleet to distribute load across workers"
    echo ""
    echo "Recommended requeue delay: $((RANDOM % MAX_REQUEUE_DELAY + 30)) seconds"
    exit 1
elif [ "$(awk "BEGIN {print ($LOAD_1MIN/$CPU_CORES) >= ($CPU_WARNING_THRESHOLD/100)}")" = "1" ]; then
    STATUS="WARN"
    echo_check WARN "CPU utilization at ${LOAD_PERCENT_1}% (above ${CPU_WARNING_THRESHOLD}% threshold)"
    echo ""
    echo "Recommendation: CAUTION - CPU is under elevated load"
    echo "Heavy operations may experience delays or resource contention"
    echo ""
    echo "Suggested actions:"
    echo "  1. Consider deferring non-urgent work"
    echo "  2. Monitor load during operation"
    echo "  3. Prepare for potential retry if saturation increases"
    echo ""
    # Still allow dispatch but with warning
    exit 0
else
    STATUS="OK"
    echo_check OK "CPU utilization at ${LOAD_PERCENT_1}% (healthy)"
    echo ""
    echo "Recommendation: OK to dispatch"
    echo "System has capacity for additional work"
    exit 0
fi
