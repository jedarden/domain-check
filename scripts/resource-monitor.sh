#!/bin/bash
# Resource Monitoring and Alerting Script
# Purpose: Monitor system resources and generate alerts before crashes occur
# Usage: ./scripts/resource-monitor.sh [--continuous] [--interval <seconds>] [--alert-on <threshold>]

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ALERT_LOG="$PROJECT_ROOT/.beads/logs/resource-alerts.log"
METRICS_LOG="$PROJECT_ROOT/.beads/logs/resource-metrics.log"

# Resource thresholds (from crash analysis)
MEMORY_WARNING_GB=10          # Warning at 10GB available
MEMORY_CRITICAL_GB=5          # Critical at 5GB available
DISK_WARNING_GB=30           # Warning at 30GB free
DISK_CRITICAL_GB=20          # Critical at 20GB free
CPU_WARNING=10               # Warning at load 10
CPU_CRITICAL=15              # Critical at load 15
PRESSURE_WARNING=70          # Warning at 70% memory pressure
PRESSURE_CRITICAL=80         # Critical at 80% memory pressure (OOM threshold)

# Monitoring defaults
CONTINUOUS_MODE=false
INTERVAL_SECONDS=300         # 5 minutes
ALERT_ON_WARNING=true
ALERT_ON_CRITICAL=true
VERBOSE=false
RUN_ONCE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --continuous)
      CONTINUOUS_MODE=true
      shift
      ;;
    --interval)
      INTERVAL_SECONDS="$2"
      shift 2
      ;;
    --alert-on)
      ALERT_ON_WARNING=false
      ALERT_ON_CRITICAL=false
      case "$2" in
        warning) ALERT_ON_WARNING=true ;;
        critical) ALERT_ON_CRITICAL=true ;;
        all)
          ALERT_ON_WARNING=true
          ALERT_ON_CRITICAL=true
          ;;
      esac
      shift 2
      ;;
    --once)
      RUN_ONCE=true
      shift
      ;;
    --verbose)
      VERBOSE=true
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [options]"
      echo "Options:"
      echo "  --continuous       Run continuously (default: single check)"
      echo "  --interval <secs>  Check interval in seconds (default: 300)"
      echo "  --once             Run single check then exit"
      echo "  --alert-on <level> Alert threshold: warning, critical, or all (default: all)"
      echo "  --verbose          Show detailed metrics"
      echo "  -h, --help         Show this help message"
      exit 0
      ;;
    *)
      echo "Error: Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

# Ensure log directory exists
mkdir -p "$(dirname "$ALERT_LOG")"
mkdir -p "$(dirname "$METRICS_LOG")"

# Logging functions
log_alert() {
  local level="$1"
  local message="$2"
  local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  echo "[$timestamp] [$level] $message" | tee -a "$ALERT_LOG"
}

log_metric() {
  local metric_name="$1"
  local metric_value="$2"
  local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  echo "$timestamp $metric_name=$metric_value" >> "$METRICS_LOG"
}

verbose() {
  if [[ "$VERBOSE" == true ]]; then
    echo "$@"
  fi
}

# Check memory availability
check_memory() {
  verbose "Checking memory availability..."

  # Get memory info from /proc/meminfo
  if [[ -f /proc/meminfo ]]; then
    # Try MemAvailable first (kernel 4.4+)
    available_mem_kb=$(grep '^MemAvailable:' /proc/meminfo | awk '{print $2}')
    if [[ -z "$available_mem_kb" ]]; then
      # Fallback: MemFree + Buffers + Cached
      mem_free=$(grep '^MemFree:' /proc/meminfo | awk '{print $2}')
      buffers=$(grep '^Buffers:' /proc/meminfo | awk '{print $2}')
      cached=$(grep '^Cached:' /proc/meminfo | awk '{print $2}' | head -1)
      available_mem_kb=$((mem_free + buffers + cached))
    fi

    total_mem_kb=$(grep '^MemTotal:' /proc/meminfo | awk '{print $2}')
    available_mem_gb=$((available_mem_kb / 1024 / 1024))
    total_mem_gb=$((total_mem_kb / 1024 / 1024))
    used_percent=$((100 - (available_mem_kb * 100 / total_mem_kb)))

    verbose "  Available: ${available_mem_gb}GB / ${total_mem_gb}GB (${used_percent}% used)"
  else
    # Fallback to free command
    available_mem_gb=$(free -g | awk '/^Mem:/{print $7}')
    total_mem_gb=$(free -g | awk '/^Mem:/{print $2}')
    used_percent=$(free | awk '/^Mem:/{printf "%.0f", $3/$2*100}')
    verbose "  Available: ${available_mem_gb}GB / ${total_mem_gb}GB (${used_percent}% used)"
  fi

  # Log metrics
  log_metric "memory_available_gb" "$available_mem_gb"
  log_metric "memory_used_percent" "$used_percent"

  # Check thresholds
  local status="OK"
  local alert_level=""

  if [[ "$available_mem_gb" -le "$MEMORY_CRITICAL_GB" ]]; then
    status="CRITICAL"
    alert_level="CRITICAL"
    if [[ "$ALERT_ON_CRITICAL" == true ]]; then
      log_alert "CRITICAL" "Memory critically low: ${available_mem_gb}GB available (< ${MEMORY_CRITICAL_GB}GB threshold)"
    fi
  elif [[ "$available_mem_gb" -le "$MEMORY_WARNING_GB" ]]; then
    status="WARNING"
    alert_level="WARNING"
    if [[ "$ALERT_ON_WARNING" == true ]]; then
      log_alert "WARNING" "Memory low: ${available_mem_gb}GB available (< ${MEMORY_WARNING_GB}GB threshold)"
    fi
  fi

  echo "MEMORY: ${available_mem_gb}GB available [${status}]"
  return 0
}

# Check disk space
check_disk() {
  verbose "Checking disk space..."

  local disk_free_gb=$(df -BG / | awk 'NR==2 {print $4}' | tr -d 'G')
  local disk_used_percent=$(df / | awk 'NR==2 {print $5}' | tr -d '%')
  local disk_total_gb=$(df -BG / | awk 'NR==2 {print $2}' | tr -d 'G')

  verbose "  Free: ${disk_free_gb}GB / ${disk_total_gb}GB (${disk_used_percent}% used)"

  # Log metrics
  log_metric "disk_free_gb" "$disk_free_gb"
  log_metric "disk_used_percent" "$disk_used_percent"

  # Check thresholds
  local status="OK"

  if [[ "$disk_free_gb" -le "$DISK_CRITICAL_GB" ]]; then
    status="CRITICAL"
    if [[ "$ALERT_ON_CRITICAL" == true ]]; then
      log_alert "CRITICAL" "Disk critically low: ${disk_free_gb}GB free (< ${DISK_CRITICAL_GB}GB threshold)"
    fi
  elif [[ "$disk_free_gb" -le "$DISK_WARNING_GB" ]]; then
    status="WARNING"
    if [[ "$ALERT_ON_WARNING" == true ]]; then
      log_alert "WARNING" "Disk low: ${disk_free_gb}GB free (< ${DISK_WARNING_GB}GB threshold)"
    fi
  fi

  echo "DISK: ${disk_free_gb}GB free [${status}]"
  return 0
}

# Check CPU load
check_cpu() {
  verbose "Checking CPU load..."

  local load_1min=$(uptime | awk -F'load average:' '{print $2}' | awk '{print $1}' | tr -d ',')
  local load_5min=$(uptime | awk -F'load average:' '{print $2}' | awk '{print $2}' | tr -d ',')
  local load_15min=$(uptime | awk -F'load average:' '{print $2}' | awk '{print $3}')

  verbose "  Load averages: ${load_1min}, ${load_5min}, ${load_15min}"

  # Log metrics
  log_metric "cpu_load_1min" "$load_1min"
  log_metric "cpu_load_5min" "$load_5min"
  log_metric "cpu_load_15min" "$load_15min"

  # Check thresholds (using awk for floating point comparison)
  local status="OK"

  if awk -v cpu_load="$load_1min" -v critical="$CPU_CRITICAL" 'BEGIN { exit !(cpu_load >= critical) }'; then
    status="CRITICAL"
    if [[ "$ALERT_ON_CRITICAL" == true ]]; then
      log_alert "CRITICAL" "CPU load critical: ${load_1min} (>= ${CPU_CRITICAL} threshold)"
    fi
  elif awk -v cpu_load="$load_1min" -v warning="$CPU_WARNING" 'BEGIN { exit !(cpu_load >= warning) }'; then
    status="WARNING"
    if [[ "$ALERT_ON_WARNING" == true ]]; then
      log_alert "WARNING" "CPU load high: ${load_1min} (>= ${CPU_WARNING} threshold)"
    fi
  fi

  echo "CPU: ${load_1min} load [${status}]"
  return 0
}

# Check memory pressure (for systems with PSI)
check_memory_pressure() {
  verbose "Checking memory pressure..."

  local pressure_file="/proc/pressure/memory"

  if [[ ! -f "$pressure_file" ]]; then
    verbose "  Memory pressure interface not available (kernel too old or PSI not enabled)"
    echo "PRESSURE: N/A [PSI not available]"
    return 0
  fi

  # Get memory pressure metrics
  # Format: some avg10=0.00 avg60=0.00 avg300=0.00 total=0
  local pressure_line=$(grep '^some' "$pressure_file")
  local avg60=$(echo "$pressure_line" | awk -F'avg60=' '{print $2}' | awk '{print $1}' | tr -d ' ')
  local avg10=$(echo "$pressure_line" | awk -F'avg10=' '{print $2}' | awk '{print $1}' | tr -d ' ')

  if [[ -z "$avg60" ]]; then
    verbose "  Could not read memory pressure metrics"
    return 0
  fi

  # Convert to percentage (using awk instead of bc)
  local pressure_percent=$(awk -v p="$avg60" 'BEGIN { printf "%d", p * 100 }')
  pressure_percent=${pressure_percent:-0}

  verbose "  Memory pressure (60s avg): ${pressure_percent}%"

  # Log metrics
  log_metric "memory_pressure_percent" "$pressure_percent"

  # Check thresholds
  local status="OK"

  if [[ "$pressure_percent" -ge "$PRESSURE_CRITICAL" ]]; then
    status="CRITICAL"
    if [[ "$ALERT_ON_CRITICAL" == true ]]; then
      log_alert "CRITICAL" "Memory pressure critical: ${pressure_percent}% (>= ${PRESSURE_CRITICAL}% OOM threshold)"
    fi
  elif [[ "$pressure_percent" -ge "$PRESSURE_WARNING" ]]; then
    status="WARNING"
    if [[ "$ALERT_ON_WARNING" == true ]]; then
      log_alert "WARNING" "Memory pressure elevated: ${pressure_percent}% (>= ${PRESSURE_WARNING}% threshold)"
    fi
  fi

  echo "PRESSURE: ${pressure_percent}% [${status}]"
  return 0
}

# Single monitoring cycle
monitor_cycle() {
  local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

  echo "=== Resource Monitor: $timestamp ==="

  check_memory
  check_disk
  check_cpu
  check_memory_pressure

  echo
}

# Main function
main() {
  if [[ "$RUN_ONCE" == true ]] || [[ "$CONTINUOUS_MODE" == false ]]; then
    # Single check mode
    monitor_cycle
    exit 0
  fi

  # Continuous mode
  echo "=== Continuous Resource Monitoring Started ==="
  echo "Interval: ${INTERVAL_SECONDS}s"
  echo "Alert on: warning=$ALERT_ON_WARNING, critical=$ALERT_ON_CRITICAL"
  echo "Press Ctrl+C to stop"
  echo

  while true; do
    monitor_cycle

    verbose "Sleeping for ${INTERVAL_SECONDS}s..."
    sleep "$INTERVAL_SECONDS"
  done
}

# Run main
main