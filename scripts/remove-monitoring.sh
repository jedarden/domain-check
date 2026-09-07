#!/usr/bin/env bash
# Remove crash prevention monitoring system

set -e

echo "=== Removing Domain Check Monitoring System ==="
echo ""

SYSTEMD_USER_DIR="$HOME/.config/systemd/user"

# Stop and disable timers
echo "Stopping timers..."
systemctl --user stop domain-check-monitoring.timer 2>/dev/null || true
systemctl --user disable domain-check-monitoring.timer 2>/dev/null || true

systemctl --user stop domain-check-resource-monitor.timer 2>/dev/null || true
systemctl --user disable domain-check-resource-monitor.timer 2>/dev/null || true

systemctl --user stop domain-check-service-monitor.timer 2>/dev/null || true
systemctl --user disable domain-check-service-monitor.timer 2>/dev/null || true

systemctl --user stop domain-check-repo-health.timer 2>/dev/null || true
systemctl --user disable domain-check-repo-health.timer 2>/dev/null || true

echo "✅ Timers stopped and disabled"
echo ""

# Remove service and timer files
echo "Removing systemd files..."
rm -f "$SYSTEMD_USER_DIR/domain-check-monitoring.service"
rm -f "$SYSTEMD_USER_DIR/domain-check-resource-monitor.service"
rm -f "$SYSTEMD_USER_DIR/domain-check-service-monitor.service"
rm -f "$SYSTEMD_USER_DIR/domain-check-repo-health.service"

rm -f "$SYSTEMD_USER_DIR/domain-check-monitoring.timer"
rm -f "$SYSTEMD_USER_DIR/domain-check-resource-monitor.timer"
rm -f "$SYSTEMD_USER_DIR/domain-check-service-monitor.timer"
rm -f "$SYSTEMD_USER_DIR/domain-check-repo-health.timer"

# Reload systemd daemon
systemctl --user daemon-reload

echo "✅ Systemd files removed"
echo ""

echo "=== Monitoring System Removed ==="
echo ""
echo "Log files preserved in: .beads/logs/"
