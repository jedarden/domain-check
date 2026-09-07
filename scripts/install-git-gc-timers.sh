#!/usr/bin/env bash
# Install systemd timers for automated git GC and repository health monitoring
# Alternative to cron when crontab is not available
#
# Usage: ./scripts/install-git-gc-timers.sh [options]
#   --dry-run        Show what would be installed without installing
#   --uninstall      Remove existing systemd timers
#   --verify         Verify installation status
#   --user           Install for current user (default: system-wide)
#
# Exit codes:
#   0 - Success
#   1 - Installation failed
#   2 - Invalid arguments

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
USER_MODE="${USER_MODE:-false}"

# Parse arguments
DRY_RUN=false
UNINSTALL=false
VERIFY=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    --uninstall)
      UNINSTALL=true
      shift
      ;;
    --verify)
      VERIFY=true
      shift
      ;;
    --user)
      USER_MODE=true
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [options]"
      echo "Options:"
      echo "  --dry-run        Show what would be installed without installing"
      echo "  --uninstall      Remove existing systemd timers"
      echo "  --verify         Verify installation status"
      echo "  --user           Install for current user (default: system)"
      echo "  -h, --help       Show this help message"
      exit 0
      ;;
    *)
      echo "Error: Unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

# Systemctl command
if [[ "$USER_MODE" == true ]]; then
  SYSTEMCTL="systemctl --user"
else
  SYSTEMCTL="systemctl"
fi

# Verify installation status
if [[ "$VERIFY" == true ]]; then
  echo "=== Git GC Systemd Timer Verification ==="
  echo ""

  timers=(
    "domain-check-git-gc.timer"
    "domain-check-git-gc-full.timer"
    "domain-check-repo-health.timer"
  )

  for timer in "${timers[@]}"; do
    if $SYSTEMCTL is-active "$timer" &>/dev/null; then
      echo "✅ $timer is active"
      $SYSTEMCTL list-timers "$timer" --no-pager 2>/dev/null | grep -A2 "$timer" || true
    else
      echo "❌ $timer is not active"
    fi
  done
  echo ""
  echo "=== Next Trigger Times ==="
  $SYSTEMCTL list-timers --all --no-pager | grep -E "domain-check|NEXT" || echo "No timers found"
  exit 0
fi

# Uninstall existing timers
if [[ "$UNINSTALL" == true ]]; then
  echo "=== Removing Git GC Systemd Timers ==="

  timers=(
    "domain-check-git-gc.timer"
    "domain-check-git-gc.service"
    "domain-check-git-gc-full.timer"
    "domain-check-git-gc-full.service"
    "domain-check-repo-health.timer"
    "domain-check-repo-health.service"
  )

  for unit in "${timers[@]}"; do
    if $SYSTEMCTL is-active "$unit" &>/dev/null; then
      echo "Stopping $unit..."
      $SYSTEMCTL stop "$unit" 2>/dev/null || true
    fi
    if $SYSTEMCTL is-enabled "$unit" &>/dev/null; then
      echo "Disabling $unit..."
      $SYSTEMCTL disable "$unit" 2>/dev/null || true
    fi
  done

  echo "✅ Git GC timers removed"
  exit 0
fi

# Install timers
echo "=== Installing Git GC Systemd Timers ==="
echo "Repository: $REPO_ROOT"
echo "Mode: $([[ "$USER_MODE" == true ]] && echo "User" || echo "System")"
echo ""

# Check if systemd is available
if ! command -v systemctl &>/dev/null; then
  echo "❌ systemd not found (systemctl command not available)"
  exit 1
fi

# Copy unit files
UNIT_DIR="$REPO_ROOT/scripts"

if [[ "$USER_MODE" == true ]]; then
  TARGET_DIR="$HOME/.config/systemd/user"
  mkdir -p "$TARGET_DIR"
else
  TARGET_DIR="/etc/systemd/system"
  echo "⚠️  System-wide installation requires root privileges"
  echo "Consider using --user mode instead"
  exit 1
fi

# Copy files
if [[ "$DRY_RUN" == true ]]; then
  echo "🔍 DRY RUN: Would install the following timers:"
  echo ""
  echo "  • domain-check-git-gc.timer (daily at 3 AM)"
  echo "  • domain-check-git-gc-full.timer (weekly on Sunday at 4 AM)"
  echo "  • domain-check-repo-health.timer (daily at 2 AM)"
  echo ""
  echo "Target directory: $TARGET_DIR"
  echo ""
  echo "To actually install, run: $0"
  exit 0
fi

echo "Installing unit files to: $TARGET_DIR"
cp "$UNIT_DIR/domain-check-git-gc.service" "$TARGET_DIR/"
cp "$UNIT_DIR/domain-check-git-gc.timer" "$TARGET_DIR/"
cp "$UNIT_DIR/domain-check-git-gc-full.service" "$TARGET_DIR/"
cp "$UNIT_DIR/domain-check-git-gc-full.timer" "$TARGET_DIR/"
cp "$UNIT_DIR/domain-check-repo-health.service" "$TARGET_DIR/"
cp "$UNIT_DIR/domain-check-repo-health.timer" "$TARGET_DIR/"

# Reload systemd
echo "Reloading systemd daemon..."
$SYSTEMCTL daemon-reload

# Enable timers
echo "Enabling timers..."
$SYSTEMCTL enable domain-check-repo-health.timer
$SYSTEMCTL enable domain-check-git-gc.timer
$SYSTEMCTL enable domain-check-git-gc-full.timer

# Start timers
echo "Starting timers..."
$SYSTEMCTL start domain-check-repo-health.timer
$SYSTEMCTL start domain-check-git-gc.timer
$SYSTEMCTL start domain-check-git-gc-full.timer

echo ""
echo "✅ Git GC timers installed and started"
echo ""
echo "=== Installed Timers ==="
echo "1. Repository health check: Daily at 2 AM"
echo "2. Standard git gc: Daily at 3 AM"
echo "3. Full git gc: Weekly on Sunday at 4 AM"
echo ""
echo "=== Verify Installation ==="
echo "Run: $0 --verify"
echo ""
echo "=== View Logs ==="
echo "Health check: tail -f $REPO_ROOT/.beads/logs/git-gc-check.log"
echo "Standard GC: tail -f $REPO_ROOT/.beads/logs/git-gc.log"
echo "Full GC: tail -f $REPO_ROOT/.beads/logs/git-gc-full.log"
echo ""
echo "=== Uninstall ==="
echo "Run: $0 --uninstall"
