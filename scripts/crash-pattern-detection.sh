#!/bin/bash
# Crash Pattern Detection Script
# Purpose: Detect systematic crash patterns and infrastructure events
# Usage: ./scripts/crash-pattern-detection.sh [--alert] [--since <timeframe>]

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ALERT_LOG="$PROJECT_ROOT/.beads/logs/crash-pattern-alerts.log"
EVENTS_FILE="$PROJECT_ROOT/.beads/events.jsonl"

# Thresholds
CRASH_SURGE_THRESHOLD=10       # crashes in 10 minutes = infrastructure event
HIGH_CRASH_RATE_THRESHOLD=5    # crashes in 1 hour = elevated
SYSTEM_EVENT_WINDOW="10minutes" # time window for surge detection

# Defaults
ALERT_MODE=false
SINCE_TIME="24hours"
VERBOSE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --alert)
      ALERT_MODE=true
      shift
      ;;
    --since)
      SINCE_TIME="$2"
      shift 2
      ;;
    --verbose)
      VERBOSE=true
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [--alert] [--since <timeframe>] [--verbose]"
      echo "  --alert    Generate alert if patterns detected"
      echo "  --since    Time window to analyze (default: 24hours)"
      echo "  --verbose  Show detailed analysis"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

# Logging function
log_alert() {
  local message="$1"
  local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  echo "[$timestamp] $message" | tee -a "$ALERT_LOG"
}

# Verbose output
verbose() {
  if [[ "$VERBOSE" == true ]]; then
    echo "$@"
  fi
}

# Check if events file exists
if [[ ! -f "$EVENTS_FILE" ]]; then
  echo "ERROR: Events file not found: $EVENTS_FILE"
  exit 1
fi

echo "=== Crash Pattern Detection ==="
echo "Time Window: $SINCE_TIME"
echo "Analysis Date: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
echo

# Get recent crash events
verbose "Extracting crash events from $EVENTS_FILE..."
RECENT_CRASHES=$(grep -i "\"event\":\"crash\"" "$EVENTS_FILE" 2>/dev/null || echo "")
CRASH_COUNT=$(echo "$RECENT_CRASHES" | wc -l)

if [[ $CRASH_COUNT -eq 0 ]]; then
  echo "✅ No crashes detected in the last $SINCE_TIME"
  echo "System Status: STABLE"
  exit 0
fi

echo "Total Crashes (last $SINCE_TIME): $CRASH_COUNT"
echo

# Analyze crashes by exit code
echo "### Crash Classification by Exit Code"
echo "$RECENT_CRASHES" | jq -r '.exit_code' | sort | uniq -c | sort -rn | while read count exit_code; do
  classification=""
  case $exit_code in
    -1)
      classification="Infrastructure (SIGKILL/SIGHUP)"
      ;;
    1)
      classification="Application Error"
      ;;
    137)
      classification="OOM Killer (128+9)"
      ;;
    *)
      classification="Unknown"
      ;;
  esac
  printf "  Exit Code %3s: %3d crashes - %s\n" "$exit_code" "$count" "$classification"
done
echo

# Analyze by worker
echo "### Crash Distribution by Worker"
echo "$RECENT_CRASHES" | jq -r '.worker' | sort | uniq -c | sort -rn | while read count worker; do
  percentage=$((count * 100 / CRASH_COUNT))
  printf "  %20s: %3d crashes (%2d%%)\n" "$worker" "$count" "$percentage"
done
echo

# Detect crash surge (infrastructure event)
verbose "Checking for crash surge pattern..."
SURGE_CRASHES=$(echo "$RECENT_CRASHES" | jq -r 'select(.ts >= "'"$(date -d "$SINCE_TIME ago" -u +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -d "now - $SINCE_TIME" -u +"%Y-%m-%dT%H:%M:%SZ")"'")' | wc -l)

if [[ $SURGE_CRASHES -ge $CRASH_SURGE_THRESHOLD ]]; then
  echo "⚠️  INFRASTRUCTURE EVENT DETECTED"
  echo "   $SURGE_CRASHES crashes in last $SINCE_TIME"
  echo "   This indicates a system-wide event (OOM, SIGHUP cascade, etc.)"

  if [[ "$ALERT_MODE" == true ]]; then
    log_alert "INFRASTRUCTURE EVENT: $SURGE_CRASHES crashes in $SINCE_TIME (threshold: $CRASH_SURGE_THRESHOLD)"
  fi
fi

# Detect high crash rate
verbose "Checking for elevated crash rate..."
if [[ $CRASH_COUNT -ge $HIGH_CRASH_RATE_THRESHOLD ]]; then
  echo "⚠️  ELEVATED CRASH RATE"
  echo "   $CRASH_COUNT crashes in last $SINCE_TIME"
  echo "   Monitoring recommended"

  if [[ "$ALERT_MODE" == true ]]; then
    log_alert "ELEVATED CRASH RATE: $CRASH_COUNT crashes in $SINCE_TIME"
  fi
fi

# Analyze temporal patterns (detect simultaneous crashes)
verbose "Analyzing temporal patterns..."
echo "### Temporal Clustering"
echo "$RECENT_CRASHES" | jq -r '.ts' | cut -d'T' -f2 | cut -d':' -f1 | sort | uniq -c | sort -rn | head -5 | while read count hour; do
  if [[ $count -gt 3 ]]; then
    echo "  Hour $hour: $count crashes (clustered pattern)"
  fi
done

# Check for duplicate alerts (same crash investigated multiple times)
verbose "Checking for duplicate investigation patterns..."
DUPLICATE_THRESHOLD=3
echo "$RECENT_CRASHES" | jq -r '.bead' | sort | uniq -c | sort -rn | while read count bead_id; do
  if [[ $count -ge $DUPLICATE_THRESHOLD ]]; then
    echo "⚠️  DUPLICATE ALERT PATTERN: bead $bead_id crashed $count times"
    echo "   This may indicate retry loops or lack of deduplication"
  fi
done

echo
echo "=== Analysis Complete ==="

# Exit codes
if [[ $SURGE_CRASHES -ge $CRASH_SURGE_THRESHOLD ]]; then
  exit 2  # Infrastructure event
elif [[ $CRASH_COUNT -ge $HIGH_CRASH_RATE_THRESHOLD ]]; then
  exit 1  # Elevated rate
else
  exit 0  # Normal
fi