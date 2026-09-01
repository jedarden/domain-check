#!/usr/bin/env bash
# Safe Git GC Monitor - Track progress and resource usage
# Usage: scripts/safe-git-gc-monitor.sh [--watch]

set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel)"
CHECKPOINT_FILE="${REPO_ROOT}/.git/safe-gc-checkpoint.json"
LOG_FILE="${REPO_ROOT}/.git/safe-gc.log"

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Print status
print_status() {
  echo "=== Safe Git GC Status ==="
  echo ""

  # Checkpoint info
  if [[ -f "$CHECKPOINT_FILE" ]]; then
    local timestamp
    timestamp=$(jq -r '.timestamp // "unknown"' "$CHECKPOINT_FILE" 2>/dev/null || echo "unknown")
    local stage
    stage=$(jq -r '.stage // "unknown"' "$CHECKPOINT_FILE" 2>/dev/null || echo "unknown")
    local status
    status=$(jq -r '.status // "unknown"' "$CHECKPOINT_FILE" 2>/dev/null || echo "unknown")
    local repo_size
    repo_size=$(jq -r '.repo_size // "unknown"' "$CHECKPOINT_FILE" 2>/dev/null || echo "unknown")
    local loose_objects
    loose_objects=$(jq -r '.loose_objects // 0' "$CHECKPOINT_FILE" 2>/dev/null || echo "0")
    local pack_count
    pack_count=$(jq -r '.pack_count // 0' "$CHECKPOINT_FILE" 2>/dev/null || echo "0")
    local message
    message=$(jq -r '.message // ""' "$CHECKPOINT_FILE" 2>/dev/null || echo "")

    echo -e "${GREEN}Last GC:${NC}"
    echo "  Timestamp: $timestamp"
    echo "  Stage: $stage"
    echo "  Status: $status"
    echo "  Message: $message"
    echo "  Repository size: $repo_size"
    echo "  Loose objects: $loose_objects"
    echo "  Pack files: $pack_count"
  else
    echo "No checkpoint file found (no gc has been run)"
  fi

  echo ""

  # Current repository stats
  echo -e "${BLUE}Current Repository:${NC}"
  cd "$REPO_ROOT"
  git count-objects -vH | sed 's/^/  /'

  echo ""

  # Git processes
  local git_procs
  git_procs=$(pgrep -f "git (gc|repack)" | wc -l)
  if [[ $git_procs -gt 0 ]]; then
    echo -e "${YELLOW}Git GC processes running: $git_procs${NC}"
    ps aux | grep -E "git (gc|repack)" | grep -v grep | sed 's/^/  /'
  else
    echo "No git gc processes running"
  fi

  echo ""

  # Recent log entries
  if [[ -f "$LOG_FILE" ]]; then
    echo -e "${BLUE}Recent log entries:${NC}"
    tail -10 "$LOG_FILE" | sed 's/^/  /'
  else
    echo "No log file found"
  fi
}

# Watch mode (update every 2 seconds)
watch_mode() {
  while true; do
    clear
    print_status
    echo ""
    echo "Press Ctrl+C to exit..."
    sleep 2
  done
}

# Parse arguments
WATCH=false

if [[ "${1:-}" == "--watch" ]]; then
  WATCH=true
fi

if $WATCH; then
  watch_mode
else
  print_status
fi
