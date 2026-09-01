#!/bin/bash
# Service Availability Monitoring Script
# Purpose: Monitor external service availability and detect outages
# Usage: ./scripts/service-monitor.sh [--continuous] [--interval <seconds>] [--services <list>]

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ALERT_LOG="$PROJECT_ROOT/.beads/logs/service-alerts.log"
METRICS_LOG="$PROJECT_ROOT/.beads/logs/service-metrics.log"

# Default services to monitor
DEFAULT_SERVICES=(
  "inference-gateway|https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health"
  "argo-workflows|https://argo-ci.ardenone.com"
  "argocd|https://argocd-ro-ardenone-manager-ts.ardenone.com:8444/api/v1/applications"
)

# Monitoring defaults
CONTINUOUS_MODE=false
INTERVAL_SECONDS=60          # 1 minute
TIMEOUT_SECONDS=5
VERBOSE=false
RUN_ONCE=false
CONSECUTIVE_FAILURES=0
MAX_CONSECUTIVE_FAILURES=3

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
    --timeout)
      TIMEOUT_SECONDS="$2"
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
      echo "  --interval <secs>  Check interval in seconds (default: 60)"
      echo "  --timeout <secs>   Request timeout in seconds (default: 5)"
      echo "  --once             Run single check then exit"
      echo "  --verbose          Show detailed check output"
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
  local service="$2"
  local message="$3"
  local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  echo "[$timestamp] [$level] [$service] $message" | tee -a "$ALERT_LOG"
}

log_metric() {
  local service_name="$1"
  local metric_value="$2"  # 0 = down, 1 = up
  local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  echo "$timestamp service_up{service=\"$service_name\"}=$metric_value" >> "$METRICS_LOG"
}

verbose() {
  if [[ "$VERBOSE" == true ]]; then
    echo "$@"
  fi
}

# Check single service
check_service() {
  local service_name="$1"
  local service_url="$2"

  verbose "Checking $service_name at $service_url..."

  local start_time=$(date +%s%N)
  local http_code=$(curl -s -o /dev/null -w "%{http_code}" --max-time "$TIMEOUT_SECONDS" "$service_url" 2>&1)
  local end_time=$(date +%s%N)
  local response_time=$(( (end_time - start_time) / 1000000000 ))  # Convert nanoseconds to seconds

  # Log metrics
  if [[ "$http_code" == "000" ]]; then
    # Connection failed
    log_metric "$service_name" 0
    verbose "  ❌ Connection failed (timeout or DNS error)"
    echo "DOWN: $service_name (connection failed)"
    return 1
  elif [[ "$http_code" == "200" ]] || [[ "$http_code" == "204" ]]; then
    # Service healthy
    log_metric "$service_name" 1
    verbose "  ✅ Healthy (HTTP $http_code, ${response_time}s)"
    echo "UP: $service_name (HTTP $http_code, ${response_time}s)"
    return 0
  else
    # Service returning error code
    log_metric "$service_name" 0
    verbose "  ⚠️  HTTP $http_code (service error)"
    echo "DOWN: $service_name (HTTP $http_code)"
    return 1
  fi
}

# Single monitoring cycle
monitor_cycle() {
  local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  local total_services=${#DEFAULT_SERVICES[@]}
  local services_up=0

  echo "=== Service Monitor: $timestamp ==="

  for service_entry in "${DEFAULT_SERVICES[@]}"; do
    IFS='|' read -r service_name service_url <<< "$service_entry"

    if check_service "$service_name" "$service_url"; then
      ((services_up++))
      CONSECUTIVE_FAILURES=0
    else
      ((CONSECUTIVE_FAILURES++))

      # Alert on consecutive failures
      if [[ $CONSECUTIVE_FAILURES -ge $MAX_CONSECUTIVE_FAILURES ]]; then
        log_alert "CRITICAL" "$service_name" "Service down for $CONSECUTIVE_FAILURES consecutive checks"
      fi
    fi
  done

  local availability=$((services_up * 100 / total_services))
  echo "Availability: $services_up/$total_services services (${availability}%)"
  echo

  # Alert if all services are down
  if [[ $services_up -eq 0 ]]; then
    log_alert "CRITICAL" "ALL_SERVICES" "All monitored services are down - possible infrastructure issue"
  fi
}

# Main function
main() {
  if [[ "$RUN_ONCE" == true ]] || [[ "$CONTINUOUS_MODE" == false ]]; then
    # Single check mode
    monitor_cycle

    # Exit code based on service availability
    if [[ $services_up -eq ${#DEFAULT_SERVICES[@]} ]]; then
      exit 0  # All services up
    elif [[ $services_up -gt 0 ]]; then
      exit 1  # Some services down
    else
      exit 2  # All services down
    fi
  fi

  # Continuous mode
  echo "=== Continuous Service Monitoring Started ==="
  echo "Interval: ${INTERVAL_SECONDS}s"
  echo "Services monitored: ${#DEFAULT_SERVICES[@]}"
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