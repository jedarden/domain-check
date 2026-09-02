#!/usr/bin/env bash
# Install comprehensive crash prevention monitoring system
# Uses systemd timers instead of cron for better reliability

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=== Installing Domain Check Monitoring System ==="
echo "Repository: $REPO_ROOT"
echo ""

# Check if running as root (required for systemd user units)
if [[ $EUID -eq 0 ]]; then
    echo "❌ Do not run as root - this script installs user-level timers"
    exit 1
fi

# Create logs directory
mkdir -p "$REPO_ROOT/.beads/logs"

# Create systemd user directory if it doesn't exist
SYSTEMD_USER_DIR="$HOME/.config/systemd/user"
mkdir -p "$SYSTEMD_USER_DIR"

echo "Installing systemd service and timer files..."

# Copy service files
cp "$REPO_ROOT/scripts/domain-check-monitoring.service" "$SYSTEMD_USER_DIR/"
cp "$REPO_ROOT/scripts/domain-check-resource-monitor.service" "$SYSTEMD_USER_DIR/"
cp "$REPO_ROOT/scripts/domain-check-service-monitor.service" "$SYSTEMD_USER_DIR/"
cp "$REPO_ROOT/scripts/domain-check-repo-health.service" "$SYSTEMD_USER_DIR/"

# Copy timer files
cp "$REPO_ROOT/scripts/domain-check-monitoring.timer" "$SYSTEMD_USER_DIR/"
cp "$REPO_ROOT/scripts/domain-check-resource-monitor.timer" "$SYSTEMD_USER_DIR/"
cp "$REPO_ROOT/scripts/domain-check-service-monitor.timer" "$SYSTEMD_USER_DIR/"
cp "$REPO_ROOT/scripts/domain-check-repo-health.timer" "$SYSTEMD_USER_DIR/"

# Reload systemd daemon
systemctl --user daemon-reload

echo "✅ Systemd files installed"
echo ""

# Enable and start timers
echo "Enabling and starting timers..."

systemctl --user enable domain-check-monitoring.timer
systemctl --user start domain-check-monitoring.timer

systemctl --user enable domain-check-resource-monitor.timer
systemctl --user start domain-check-resource-monitor.timer

systemctl --user enable domain-check-service-monitor.timer
systemctl --user start domain-check-service-monitor.timer

systemctl --user enable domain-check-repo-health.timer
systemctl --user start domain-check-repo-health.timer

echo "✅ All timers enabled and started"
echo ""

echo "=== Monitoring System Active ==="
echo ""
echo "Active timers:"
systemctl --user list-timers | grep domain-check || echo "No timers listed yet (starting soon)"
echo ""
echo "=== Log Files ==="
echo "Crash monitoring: $REPO_ROOT/.beads/logs/crash-monitor.log"
echo "Resource monitoring: $REPO_ROOT/.beads/logs/resource-monitor.log"
echo "Service monitoring: $REPO_ROOT/.beads/logs/service-monitor.log"
echo "Repository health: $REPO_ROOT/.beads/logs/repo-health.log"
echo ""
echo "=== View Logs ==="
echo "Crash patterns: tail -f $REPO_ROOT/.beads/logs/crash-monitor.log"
echo "Resources: tail -f $REPO_ROOT/.beads/logs/resource-monitor.log"
echo "Services: tail -f $REPO_ROOT/.beads/logs/service-monitor.log"
echo ""
echo "=== Monitor Timer Status ==="
echo "Run: systemctl --user list-timers"
echo "Run: systemctl --user status domain-check-monitoring.timer"
echo ""
echo "=== Stop Monitoring ==="
echo "Run: ./scripts/remove-monitoring.sh"
