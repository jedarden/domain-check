#!/usr/bin/env bash
# Crash Pattern Detection - Identify systematic crash patterns in NEEDLE system
#
# Purpose: Detect systematic crash patterns that indicate infrastructure events
# rather than individual task failures. Helps distinguish between:
# - Infrastructure events (system-wide crashes)
# - Service failures (specific error patterns)
# - Isolated task failures (genuine issues)
#
# Usage: ./scripts/crash-pattern-detection.sh [options]
#   --hours=N       Look back N hours (default: 24)
#   --crash-rate=N  Alert if crash rate exceeds N per hour (default: 5)
#   --output=FILE   Write detailed report to FILE
#   --quiet         Suppress informational output
#   --verbose       Show detailed analysis
#
# Exit codes:
#   0 - No concerning patterns detected
#   1 - Systematic crash pattern detected
#   2 - Error in execution

set -euo pipefail

# Configuration
HOURS="${HOURS:-24}"
CRASH_RATE_THRESHOLD="${CRASH_RATE_THRESHOLD:-5}"
OUTPUT_FILE=""
QUIET=false
VERBOSE=false

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --hours=*)
      HOURS="${1#*=}"
      shift
      ;;
    --crash-rate=*)
      CRASH_RATE_THRESHOLD="${1#*=}"
      shift
      ;;
    --output=*)
      OUTPUT_FILE="${1#*=}"
      shift
      ;;
    --quiet)
      QUIET=true
      shift
      ;;
    --verbose)
      VERBOSE=true
      shift
      ;;
    -h|--help)
      cat <<EOF
Usage: $0 [options]

Detect systematic crash patterns in NEEDLE beads.

Options:
  --hours=N         Look back N hours (default: 24)
  --crash-rate=N    Alert if crashes/hour exceeds N (default: 5)
  --output=FILE     Write detailed report to FILE
  --quiet           Suppress informational output
  --verbose         Show detailed analysis
  -h, --help        Show this help message

Exit codes:
  0 - No concerning patterns detected
  1 - Systematic crash pattern detected
  2 - Error in execution
EOF
      exit 0
      ;;
    *)
      echo "Error: Unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

# Logging functions
log_info() {
  if [[ "$QUIET" != true ]]; then
    echo -e "${BLUE}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $*"
  fi
}

log_warn() {
  echo -e "${YELLOW}[WARNING]${NC} $*"
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $*" >&2
}

log_success() {
  echo -e "${GREEN}[OK]${NC} $*"
}

log_verbose() {
  if [[ "$VERBOSE" == true ]]; then
    echo -e "[VERBOSE] $*"
  fi
}

# Check if bead command is available
check_bead_available() {
  if ! command -v bead &> /dev/null; then
    log_error "bead command not found. This script requires bead-rs CLI."
    exit 2
  fi
}

# Get recent crashed beads
get_crashed_beads() {
  local since="${1}"

  log_verbose "Getting crashed beads from last $since..."

  # Try to get crashed beads using bead list
  # Note: bead list may not have time filtering, so we get all and filter
  if bead list --status "in_progress" --json 2>/dev/null | jq -r '.[] | select(.effective_status == "in_progress") | .id' 2>/dev/null; then
    # If that worked, try alternative approach
    bead list --json 2>/dev/null | jq -r '.[] | select(.status == "in_progress") | .id' 2>/dev/null || echo ""
  else
    # Fallback: check git log for crash-related commits
    git log --since="$since" --grep="crash" --oneline | wc -l
  fi
}

# Analyze crash patterns
analyze_crash_patterns() {
  local crash_count=$1
  local time_period_hours=$2

  log_info "=== Crash Pattern Analysis ==="
  log_info "Time period: Last $time_period_hours hours"
  log_info "Crash threshold: $CRASH_RATE_THRESHOLD crashes/hour"
  log_info "Crashes detected: $crash_count"

  # Calculate crash rate per hour
  if [[ "$crash_count" -gt 0 ]]; then
    crash_rate=$(awk "BEGIN {printf \"%.2f\", $crash_count / $time_period_hours}")
    log_info "Crash rate: $crash_rate crashes/hour"

    # Check if crash rate exceeds threshold (using awk for floating point comparison)
    if awk "BEGIN {exit !($crash_rate > $CRASH_RATE_THRESHOLD)}"; then
      log_warn "⚠️  HIGH CRASH RATE DETECTED"
      log_warn "   Current rate: $crash_rate crashes/hour"
      log_warn "   Threshold: $CRASH_RATE_THRESHOLD crashes/hour"
      log_warn ""
      log_warn "   This indicates a SYSTEMATIC ISSUE, not isolated failures:"
      log_warn "   - Infrastructure event (memory pressure, OOM)"
      log_warn "   - Service availability failure (inference gateway)"
      log_warn "   - Agent workflow issue (max turns, bead closing)"
      return 1
    else
      log_success "Crash rate within acceptable limits"
      return 0
    fi
  else
    log_success "No crashes detected in time period"
    return 0
  fi
}

# Detect crash clustering
detect_crash_clustering() {
  log_verbose "Checking for crash clustering..."

  # Look for git commits with crash-related messages in recent history
  local since="${1} hours ago"

  if git log --since="$since" --grep="crash\|OOM\|killed\|SIGTERM\|SIGKILL" --oneline 2>/dev/null | grep -q .; then
    local clustered_count=$(git log --since="$since" --grep="crash\|OOM\|killed\|SIGTERM\|SIGKILL" --oneline 2>/dev/null | wc -l)

    if [[ "$clustered_count" -gt 3 ]]; then
      log_warn "⚠️  CRASH CLUSTERING DETECTED"
      log_warn "   $clustered_count crash-related commits in last $HOURS hours"
      log_warn "   Pattern suggests infrastructure event or service failure"
      return 1
    fi
  fi

  return 0
}

# Check system health indicators
check_system_health() {
  log_verbose "Checking current system health..."

  local warnings=0

  # Check memory pressure
  if [[ -f /proc/meminfo ]]; then
    local mem_available_kb=$(grep '^MemAvailable:' /proc/meminfo | awk '{print $2}')
    local mem_total_kb=$(grep '^MemTotal:' /proc/meminfo | awk '{print $2}')

    if [[ -n "$mem_available_kb" && -n "$mem_total_kb" ]]; then
      local mem_percent=$(awk "BEGIN {printf \"%.1f\", (1 - $mem_available_kb / $mem_total_kb) * 100}")

      log_verbose "Memory usage: ${mem_percent}%"

      # Use awk for floating point comparison instead of bc
      if awk "BEGIN {exit !($mem_percent > 80)}"; then
        log_warn "⚠️  High memory usage: ${mem_percent}%"
        warnings=$((warnings + 1))
      fi
    fi
  fi

  # Check load average
  local load_1min=$(uptime | awk -F'load average:' '{print $2}' | awk '{print $1}' | tr -d ',')
  local load_5min=$(uptime | awk -F'load average:' '{print $2}' | awk '{print $2}' | tr -d ',')

  log_verbose "Load average: ${load_1min} (1min), ${load_5min} (5min)"

  # Check disk space
  local disk_free=$(df -BG / 2>/dev/null | awk 'NR==2 {print $4}' | tr -d 'G')
  log_verbose "Disk space free: ${disk_free}GB"

  if [[ "$disk_free" -lt 10 ]]; then
    log_warn "⚠️  Low disk space: ${disk_free}GB free"
    warnings=$((warnings + 1))
  fi

  return $warnings
}

# Generate detailed report
generate_report() {
  local crash_count=$1
  local pattern_detected=$2

  local report="Crash Pattern Detection Report
Generated: $(date)
Analysis Period: Last $HOURS hours

SUMMARY
-------
Crashes detected: $crash_count
Crash rate threshold: $CRASH_RATE_THRESHOLD crashes/hour
Pattern detected: $pattern_detected

"

  if [[ "$crash_count" -gt 0 ]]; then
    crash_rate=$(awk "BEGIN {printf \"%.2f\", $crash_count / $HOURS}")
    report+="Crash rate: $crash_rate crashes/hour
"
  fi

  report+="
SYSTEM HEALTH
------------
"
  # Get memory info
  if [[ -f /proc/meminfo ]]; then
    local mem_available=$(grep '^MemAvailable:' /proc/meminfo | awk '{print $2}')
    local mem_total=$(grep '^MemTotal:' /proc/meminfo | awk '{print $2}')
    if [[ -n "$mem_available" && -n "$mem_total" ]]; then
      local mem_used=$(awk "BEGIN {printf \"%.1f\", (($mem_total - $mem_available) / $mem_total) * 100}")
      report+="Memory usage: ${mem_used}%\n"
    fi
  fi
  report+="Load average: $(uptime | awk -F'load average:' '{print $2}')\n"
  report+="Disk space: $(df -h / | awk 'NR==2 {print $4}') available\n"

  report+="
RECOMMENDATIONS
--------------
"

  if [[ "$pattern_detected" == "true" ]]; then
    report+="Systematic crash pattern detected. Immediate actions:
1. Check infrastructure: journalctl --since '$HOURS hours ago' | grep -E 'oom|kill|memory'
2. Check service health: ./scripts/preflight-health-check.sh --verbose
3. Review crash classification guide: docs/crash-response-guide.md
4. Consider deferring new tasks until system stabilizes
"
  else
    report+="No systematic patterns detected. System operating normally.
Continue standard operations and monitoring.
"
  fi

  if [[ -n "$OUTPUT_FILE" ]]; then
    echo "$report" > "$OUTPUT_FILE"
    log_info "Report written to: $OUTPUT_FILE"
  fi

  if [[ "$VERBOSE" == true ]]; then
    echo ""
    echo "$report"
  fi
}

# Main execution
main() {
  log_info "=== Crash Pattern Detection Started ==="

  # Check dependencies
  check_bead_available

  # Get crash count
  local crash_count=0

  # Try to detect crashes from git history
  local crash_commits=$(git log --since="$HOURS hours ago" --grep="crash\|OOM\|killed" --oneline 2>/dev/null | wc -l)
  crash_count=$crash_commits

  log_verbose "Found $crash_count crash-related commits in git history"

  # Analyze patterns
  local pattern_detected=false

  if ! analyze_crash_patterns "$crash_count" "$HOURS"; then
    pattern_detected=true
  fi

  # Check for clustering
  if detect_crash_clustering "$HOURS"; then
    pattern_detected=true
  fi

  # Check system health
  check_system_health || true

  # Generate report
  generate_report "$crash_count" "$pattern_detected"

  # Final assessment
  log_info "=== Analysis Complete ==="

  if [[ "$pattern_detected" == "true" ]]; then
    log_warn "⚠️  SYSTEMATIC CRASH PATTERN DETECTED"
    log_warn "   Review recommendations above and consider deferring tasks"
    exit 1
  else
    log_success "No concerning crash patterns detected"
    exit 0
  fi
}

# Run main
main "$@"
