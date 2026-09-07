#!/bin/bash
# Memory Pressure Monitoring Script
# Proactively alerts before OOM threshold is reached

set -euo pipefail

# Configuration
MEMORY_PRESSURE_WARNING=70    # Alert at 70% pressure
MEMORY_PRESSURE_CRITICAL=80   # OOM threshold
AVAILABLE_MEMORY_WARNING=10   # Alert at 10GB available
AVAILABLE_MEMORY_CRITICAL=5   # Critical at 5GB available
LOG_DIR="$HOME/.beads/logs"
MEMORY_LOG="$LOG_DIR/memory-pressure.log"
ALERT_COOLDOWN=300  # 5 minutes between alerts

# Ensure log directory exists
mkdir -p "$LOG_DIR"

# Get memory pressure percentage (Linux-specific)
get_memory_pressure() {
    # Try systemd-oomd's memory.pressure if available
    if [ -f "/proc/pressure/memory" ]; then
        # Extract the "some" pressure metric (10-second average)
        local pressure_line=$(grep "some" /proc/pressure/memory | cut -d' ' -f2)
        local pressure_value=$(echo "$pressure_line" | sed 's/%//')
        echo "$pressure_value"
        return 0
    fi

    # Fallback: estimate pressure from available memory
    local total_mem=$(free -g | awk '/^Mem:/{print $2}')
    local avail_mem=$(free -g | awk '/^Mem:/{print $7}')
    local used_mem=$((total_mem - avail_mem))
    local pressure=$((used_mem * 100 / total_mem))
    echo "$pressure"
    return 0
}

# Get available memory in GB
get_available_memory() {
    free -g | awk '/^Mem:/{print $7}'
}

# Check if we should alert (cooldown period)
should_alert() {
    local log_file="$1"

    if [ ! -f "$log_file" ]; then
        return 0  # No recent alerts
    fi

    local last_alert_time=$(tail -1 "$log_file" | cut -d'|' -f1)
    local current_time=$(date +%s)
    local time_since_alert=$((current_time - last_alert_time))

    if [ "$time_since_alert" -lt "$ALERT_COOLDOWN" ]; then
        return 1  # Still in cooldown
    fi

    return 0  # Ready to alert
}

# Log memory state
log_memory_state() {
    local level="$1"
    local message="$2"
    local timestamp=$(date +%s)
    local human_time=$(date -d "@$timestamp" '+%Y-%m-%d %H:%M:%S')

    echo "$timestamp|$level|$human_time|$message" >> "$MEMORY_LOG"

    # Also print to stdout with color if terminal
    case "$level" in
        CRITICAL)
            echo -e "\033[31m[$human_time] CRITICAL: $message\033[0m"
            ;;
        WARNING)
            echo -e "\033[33m[$human_time] WARNING: $message\033[0m"
            ;;
        INFO)
            echo -e "\033[36m[$human_time] INFO: $message\033[0m"
            ;;
    esac
}

# Main monitoring function
monitor_memory_pressure() {
    local pressure=$(get_memory_pressure)
    local available=$(get_available_memory)
    local timestamp=$(date '+%Y-%m-%d %H:%M:%S')

    echo "=== Memory Pressure Check: $timestamp ==="
    echo "Memory Pressure: ${pressure}%"
    echo "Available Memory: ${available}GB"

    # Check for critical conditions
    if [ "$pressure" -ge "$MEMORY_PRESSURE_CRITICAL" ] || [ "$available" -le "$AVAILABLE_MEMORY_CRITICAL" ]; then
        if should_alert "$MEMORY_LOG"; then
            local msg="CRITICAL: Memory pressure ${pressure}% >= ${MEMORY_PRESSURE_CRITICAL}% OR available ${available}GB <= ${AVAILABLE_MEMORY_CRITICAL}GB. OOM risk imminent!"
            log_memory_state "CRITICAL" "$msg"
            echo "🚨 $msg"
            return 2
        fi
    fi

    # Check for warning conditions
    if [ "$pressure" -ge "$MEMORY_PRESSURE_WARNING" ] || [ "$available" -le "$AVAILABLE_MEMORY_WARNING" ]; then
        if should_alert "$MEMORY_LOG"; then
            local msg="WARNING: Memory pressure ${pressure}% >= ${MEMORY_PRESSURE_WARNING}% OR available ${available}GB <= ${AVAILABLE_MEMORY_WARNING}GB. Approaching OOM threshold."
            log_memory_state "WARNING" "$msg"
            echo "⚠️  $msg"
            return 1
        fi
    fi

    # Normal state
    log_memory_state "INFO" "Memory pressure ${pressure}%, available ${available}GB - Normal"
    echo "✓ Memory state healthy"
    return 0
}

# Continuous monitoring mode
monitor_continuous() {
    local interval="${1:-60}"  # Default 60 seconds

    echo "Starting continuous memory monitoring (check every ${interval}s)"
    echo "Press Ctrl+C to stop"

    while true; do
        monitor_memory_pressure
        sleep "$interval"
    done
}

# One-shot check (default)
monitor_once() {
    monitor_memory_pressure
}

# Check if we should suppress new bead claims
should_throttle_bead_claims() {
    local pressure=$(get_memory_pressure)
    local available=$(get_available_memory)

    # Throttle if approaching critical thresholds
    if [ "$pressure" -ge 75 ] || [ "$available" -le 8 ]; then
        echo "THROTTLE"
        return 0
    fi

    echo "NORMAL"
    return 1
}

# Main entry point
case "${1:-once}" in
    continuous|monitor)
        monitor_continuous "${2:-60}"
        ;;
    once|check|"")
        monitor_once
        ;;
    throttle)
        should_throttle_bead_claims
        ;;
    *)
        echo "Usage: $0 [once|continuous|throttle] [interval_seconds]"
        echo "  once       - Single memory check (default)"
        echo "  continuous - Monitor continuously (default interval: 60s)"
        echo "  throttle   - Check if bead claims should be throttled"
        exit 1
        ;;
esac
