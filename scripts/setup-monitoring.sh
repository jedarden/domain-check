#!/usr/bin/env bash
# Setup Automated Monitoring for Crash Prevention
# Configures cron jobs for automated repository health monitoring
#
# Usage: ./scripts/setup-monitoring.sh [options]
#   --remove        Remove monitoring cron jobs
#   --dry-run       Show what would be done without making changes
#   --list          List current monitoring cron jobs

set -euo pipefail

DRY_RUN=false
REMOVE=false
LIST_ONLY=false

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --remove)
      REMOVE=true
      shift
      ;;
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    --list)
      LIST_ONLY=true
      shift
      ;;
    -h|--help)
      cat <<'EOF'
Usage: setup-monitoring.sh [options]

Setup automated monitoring cron jobs for crash prevention.

Options:
  --remove        Remove monitoring cron jobs
  --dry-run       Show what would be done without making changes
  --list          List current monitoring cron jobs
  -h, --help      Show this help message

Monitoring jobs:
  - Repository health check (daily at 2am)
  - Crash pattern detection (every 6 hours)
EOF
      exit 0
      ;;
    *)
      echo "Error: Unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git rev-parse --show-toplevel)"

# Cron job identifiers
CRON_ID_REPO_HEALTH="domain-check-repo-health"
CRON_ID_CRASH_PATTERN="domain-check-crash-pattern"

# Logging functions
log_info() {
  echo -e "${GREEN}[INFO]${NC} $*"
}

log_warn() {
  echo -e "${YELLOW}[WARN]${NC} $*"
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $*" >&2
}

# List current monitoring jobs
list_jobs() {
  echo "=== Current Monitoring Cron Jobs ==="
  echo ""

  local crontab=$(crontab -l 2>/dev/null || true)

  if echo "$crontab" | grep -q "$CRON_ID_REPO_HEALTH"; then
    echo "✅ Repository Health Monitoring:"
    echo "$crontab" | grep "$CRON_ID_REPO_HEALTH" | sed 's/^/    /'
    echo ""
  else
    echo "❌ Repository Health Monitoring: NOT CONFIGURED"
    echo ""
  fi

  if echo "$crontab" | grep -q "$CRON_ID_CRASH_PATTERN"; then
    echo "✅ Crash Pattern Detection:"
    echo "$crontab" | grep "$CRON_ID_CRASH_PATTERN" | sed 's/^/    /'
    echo ""
  else
    echo "❌ Crash Pattern Detection: NOT CONFIGURED"
    echo ""
  fi
}

# Remove monitoring jobs
remove_jobs() {
  local crontab=$(crontab -l 2>/dev/null || true)
  local modified=false

  # Filter out monitoring jobs
  local new_crontab=$(echo "$crontab" | grep -v "$CRON_ID_REPO_HEALTH" | grep -v "$CRON_ID_CRASH_PATTERN" || true)

  if [[ "$crontab" != "$new_crontab" ]]; then
    modified=true
    if [[ "$DRY_RUN" == true ]]; then
      log_warn "[DRY RUN] Would remove monitoring cron jobs"
      echo "Jobs to be removed:"
      echo "$crontab" | grep -E "$CRON_ID_REPO_HEALTH|$CRON_ID_CRASH_PATTERN" | sed 's/^/  /'
    else
      echo "$new_crontab" | crontab -
      log_info "Removed monitoring cron jobs"
    fi
  else
    log_info "No monitoring cron jobs found"
  fi

  [[ "$modified" == true ]] && return 0 || return 1
}

# Add monitoring jobs
add_jobs() {
  local crontab=$(crontab -l 2>/dev/null || true)
  local modified=false

  # Repository health check (daily at 2am)
  if ! echo "$crontab" | grep -q "$CRON_ID_REPO_HEALTH"; then
    local repo_health_job="0 2 * * * cd $REPO_ROOT && $SCRIPT_DIR/repo-health-monitor.sh --warn-only >> $REPO_ROOT/.beads/logs/repo-health.log 2>&1 # $CRON_ID_REPO_HEALTH"

    if [[ "$DRY_RUN" == true ]]; then
      log_warn "[DRY RUN] Would add repository health monitoring job:"
      echo "  $repo_health_job"
    else
      (echo "$crontab"; echo "$repo_health_job") | crontab -
      crontab="$repo_health_job
$crontab"
      log_info "Added repository health monitoring job (daily at 2am)"
      modified=true
    fi
  else
    log_info "Repository health monitoring already configured"
  fi

  # Crash pattern detection (every 6 hours)
  if ! echo "$crontab" | grep -q "$CRON_ID_CRASH_PATTERN"; then
    local crash_pattern_job="0 */6 * * * cd $REPO_ROOT && $SCRIPT_DIR/crash-pattern-detection.sh --hours=6 --quiet # $CRON_ID_CRASH_PATTERN"

    if [[ "$DRY_RUN" == true ]]; then
      log_warn "[DRY RUN] Would add crash pattern detection job:"
      echo "  $crash_pattern_job"
    else
      (echo "$crontab"; echo "$crash_pattern_job") | crontab -
      log_info "Added crash pattern detection job (every 6 hours)"
      modified=true
    fi
  else
    log_info "Crash pattern detection already configured"
  fi

  return 0
}

# Main execution
main() {
  if [[ "$LIST_ONLY" == true ]]; then
    list_jobs
    exit 0
  fi

  echo "=== Automated Monitoring Setup ==="
  echo "Repository: $REPO_ROOT"
  echo ""

  if [[ "$REMOVE" == true ]]; then
    echo "Action: Remove monitoring cron jobs"
    echo ""
    remove_jobs
  else
    echo "Action: Add monitoring cron jobs"
    echo ""
    add_jobs
    echo ""
    echo "Monitoring jobs configured:"
    list_jobs
    echo ""
    echo "Log files:"
    echo "  - Repository health: $REPO_ROOT/.beads/logs/repo-health.log"
    echo "  - Crash patterns: Check system logs for script output"
  fi
}

main "$@"
