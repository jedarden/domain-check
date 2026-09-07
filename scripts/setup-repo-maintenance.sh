#!/usr/bin/env bash
# Repository Maintenance Setup Script
# Implements automated git gc scheduling and repository health monitoring
#
# Based on crash mitigation strategies (Priority 3 - CRITICAL):
# - Prevents repository bloat that caused bf-4yjq incident (9 OOM crashes from 18GB repo)
# - Automated git gc: daily standard + weekly full
# - Repository health monitoring: daily size checks with alerts
#
# Usage: scripts/setup-repo-maintenance.sh [--remove]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git rev-parse --show-toplevel)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Parse arguments
REMOVE=false
if [[ "${1:-}" == "--remove" ]]; then
  REMOVE=true
fi

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Repository Maintenance Setup${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

if [[ "$REMOVE" == true ]]; then
  echo -e "${YELLOW}Removing repository maintenance automation...${NC}"
else
  echo -e "${GREEN}Installing repository maintenance automation...${NC}"
fi
echo ""

# Check if running as root or with sudo
if [[ $EUID -eq 0 ]]; then
  echo -e "${RED}✗ This script should NOT be run as root${NC}"
  echo "  Systemd user timers should be installed as the regular user"
  exit 1
fi

# Create log directories
mkdir -p "$REPO_ROOT/.beads/logs"

if [[ "$REMOVE" == true ]]; then
  # Stop and disable timers
  echo -e "${YELLOW}Stopping and disabling systemd timers...${NC}"

  timers=(
    "domain-check-repo-health.timer"
    "domain-check-auto-gc.timer"
    "domain-check-git-gc.timer"
    "domain-check-git-gc-full.timer"
    "domain-check-alert-triage.timer"
  )

  for timer in "${timers[@]}"; do
    if systemctl --user is-active --quiet "$timer" 2>/dev/null; then
      systemctl --user stop "$timer"
      echo -e "${GREEN}✓ Stopped $timer${NC}"
    fi

    if systemctl --user is-enabled --quiet "$timer" 2>/dev/null; then
      systemctl --user disable "$timer"
      echo -e "${GREEN}✓ Disabled $timer${NC}"
    fi
  done

  # Remove service and timer files
  echo -e "${YELLOW}Removing systemd service and timer files...${NC}"

  files=(
    "domain-check-repo-health.service"
    "domain-check-repo-health.timer"
    "domain-check-auto-gc.service"
    "domain-check-auto-gc.timer"
    "domain-check-git-gc.service"
    "domain-check-git-gc.timer"
    "domain-check-git-gc-full.service"
    "domain-check-git-gc-full.timer"
    "domain-check-alert-triage.service"
    "domain-check-alert-triage.timer"
  )

  for file in "${files[@]}"; do
    target="$HOME/.config/systemd/user/$file"
    if [[ -f "$target" ]]; then
      rm -f "$target"
      echo -e "${GREEN}✓ Removed $file${NC}"
    fi
  done

  # Reload systemd
  systemctl --user daemon-reload 2>/dev/null || true

  echo -e "${GREEN}✓ Repository maintenance automation removed${NC}"

else
  # Note when timers already exist. This is a REFRESH, not a skip: re-copying
  # the unit files and daemon-reloading is how an installed-but-stale unit
  # ever gets updated (the stale-unit trap documented in CLAUDE.md — an edited
  # unit file does nothing until the manager reloads), and it is how a newly
  # added unit (e.g. domain-check-auto-gc) reaches a box that installed the
  # earlier three already. enable/start below are idempotent, so refreshing
  # over live timers is safe.
  if systemctl --user is-enabled --quiet domain-check-repo-health.timer 2>/dev/null; then
    echo -e "${YELLOW}⚠ Repository maintenance systemd timers already exist — refreshing unit files${NC}"
    echo ""
    systemctl --user list-timers 'domain-check-*' --all --no-pager || true
    echo ""
  fi

  echo -e "${GREEN}Installing systemd service and timer files...${NC}"
  echo ""

  # Create systemd user directory if it doesn't exist
  mkdir -p "$HOME/.config/systemd/user"

  # Copy service and timer files
  service_files=(
    "domain-check-repo-health.service:domain-check-repo-health.service"
    "domain-check-repo-health.timer:domain-check-repo-health.timer"
    "domain-check-auto-gc.service:domain-check-auto-gc.service"
    "domain-check-auto-gc.timer:domain-check-auto-gc.timer"
    "domain-check-git-gc.service:domain-check-git-gc.service"
    "domain-check-git-gc.timer:domain-check-git-gc.timer"
    "domain-check-git-gc-full.service:domain-check-git-gc-full.service"
    "domain-check-git-gc-full.timer:domain-check-git-gc-full.timer"
  )

  for source_target in "${service_files[@]}"; do
    source="$SCRIPT_DIR/${source_target%%:*}"
    target="${source_target##*:}"
    target_path="$HOME/.config/systemd/user/$target"

    if [[ ! -f "$source" ]]; then
      echo -e "${YELLOW}⚠ Source file not found: $source${NC}"
      continue
    fi
    # Some installed units are hard links to the tracked source (identical
    # inode), so a plain `cp` aborts with "same file" — and under `set -e`
    # that killed the whole install before daemon-reload/enable. Content
    # decides: identical → nothing to do; different → replace the destination
    # outright (--remove-destination handles the hard-linked case), leaving a
    # plain file that future refreshes compare the same way.
    if [[ -e "$target_path" ]] && cmp -s "$source" "$target_path"; then
      echo -e "${GREEN}✓ $target already up to date${NC}"
    else
      cp --remove-destination "$source" "$target_path"
      echo -e "${GREEN}✓ Installed $target${NC}"
    fi
  done

  echo ""
  echo -e "${GREEN}Reloading systemd daemon...${NC}"
  systemctl --user daemon-reload

  echo ""
  echo -e "${GREEN}Enabling and starting timers...${NC}"

  # Enable and start timers
  timers=(
    "domain-check-repo-health.timer"
    "domain-check-auto-gc.timer"
    "domain-check-git-gc.timer"
    "domain-check-git-gc-full.timer"
  )

  for timer in "${timers[@]}"; do
    systemctl --user enable "$timer"
    systemctl --user start "$timer"
    echo -e "${GREEN}✓ Started $timer${NC}"
  done

  echo ""
  echo -e "${GREEN}✓ Repository maintenance automation installed${NC}"
  echo ""
  echo -e "${YELLOW}Active timers:${NC}"
  systemctl --user list-timers | grep domain-check || echo "  (No timers found)"
fi

echo ""
echo -e "${BLUE}========================================${NC}"

if [[ "$REMOVE" == false ]]; then
  echo -e "${GREEN}✓ Repository maintenance automation installed${NC}"
  echo ""
  echo -e "${YELLOW}What happens next:${NC}"
  echo "  • Daily repository health check at 2 AM (detection)"
  echo "  • Daily threshold-triggered auto gc at 2:30 AM (remediates only when the repo crossed a bloat threshold)"
  echo "  • Daily standard git gc at 3 AM"
  echo "  • Weekly full git gc (Sunday 4 AM)"
  echo ""
  echo -e "${YELLOW}Logs:${NC}"
  echo "  • $REPO_ROOT/.beads/logs/repo-health.log"
  echo "  • $REPO_ROOT/.beads/logs/auto-gc.log"
  echo "  • $REPO_ROOT/.beads/logs/git-gc.log"
  echo ""
  echo -e "${YELLOW}To check timer status:${NC}"
  echo "  systemctl --user list-timers | grep domain-check"
  echo ""
  echo -e "${YELLOW}To view logs:${NC}"
  echo "  journalctl --user -u domain-check-repo-health.service"
  echo "  journalctl --user -u domain-check-auto-gc.service"
  echo "  journalctl --user -u domain-check-git-gc.service"
  echo ""
  echo -e "${YELLOW}To remove: $0 --remove${NC}"
fi

echo -e "${BLUE}========================================${NC}"
