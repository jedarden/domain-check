#!/usr/bin/env bash
# Setup continuous monitoring for crash prevention
# This script configures cron jobs for automated monitoring

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

echo "=== Domain Check Monitoring Setup ==="
echo "Repository: $REPO_ROOT"
echo ""

# Check if crontab exists
if ! crontab -l > /dev/null 2>&1; then
    echo "No existing crontab found - creating new one"
fi

# Create temporary crontab file
TEMP_CRON=$(mktemp)
if crontab -l > /dev/null 2>&1; then
    crontab -l > "$TEMP_CRON"
fi

# Check if monitoring jobs already exist
if grep -q "domain-check.*monitoring" "$TEMP_CRON" 2>/dev/null; then
    echo "⚠️  Monitoring cron jobs already exist"
    echo "Run: crontab -l to view existing jobs"
    echo ""
    read -p "Remove existing monitoring jobs and reinstall? (y/N) " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        # Remove existing domain-check monitoring jobs
        grep -v "domain-check.*monitoring" "$TEMP_CRON" > "$TEMP_CRON.new" || true
        mv "$TEMP_CRON.new" "$TEMP_CRON"
        echo "Removed existing monitoring jobs"
    else
        rm "$TEMP_CRON"
        echo "Setup cancelled"
        exit 0
    fi
fi

# Add monitoring cron jobs
cat >> "$TEMP_CRON" << EOF

# === Domain Check Monitoring (installed $(date)) ===
# Crash alert manager with classification and deduplication (every 5 minutes)
*/5 * * * * cd $REPO_ROOT && $SCRIPT_DIR/crash-alert-manager.sh --auto-process >> $REPO_ROOT/.beads/logs/crash-alert-manager.log 2>&1

# Crash pattern detection (every 10 minutes)
*/10 * * * * cd $REPO_ROOT && $SCRIPT_DIR/crash-pattern-detection.sh --quiet >> $REPO_ROOT/.beads/logs/crash-monitor.log 2>&1

# Resource monitoring (every 5 minutes)
*/5 * * * * cd $REPO_ROOT && $SCRIPT_DIR/resource-monitor.sh --once --quiet >> $REPO_ROOT/.beads/logs/resource-monitor.log 2>&1

# Service monitoring (every 2 minutes)
*/2 * * * * cd $REPO_ROOT && $SCRIPT_DIR/service-monitor.sh --once --quiet >> $REPO_ROOT/.beads/logs/service-monitor.log 2>&1

# Repository health monitoring (every hour)
0 * * * * cd $REPO_ROOT && $SCRIPT_DIR/check-repo-health.sh --quiet >> $REPO_ROOT/.beads/logs/repo-health.log 2>&1
EOF

# Install crontab
crontab "$TEMP_CRON"
rm "$TEMP_CRON"

echo "✅ Monitoring cron jobs installed"
echo ""
echo "=== Installed Jobs ==="
echo "1. Crash pattern detection: every 10 minutes"
echo "2. Resource monitoring: every 5 minutes"
echo "3. Service monitoring: every 2 minutes"
echo ""
echo "=== Log Files ==="
echo "Crash monitoring: $REPO_ROOT/.beads/logs/crash-monitor.log"
echo "Resource monitoring: $REPO_ROOT/.beads/logs/resource-monitor.log"
echo "Service monitoring: $REPO_ROOT/.beads/logs/service-monitor.log"
echo ""
echo "=== View Jobs ==="
echo "Run: crontab -l"
echo ""
echo "=== View Logs ==="
echo "Crash patterns: tail -f $REPO_ROOT/.beads/logs/crash-monitor.log"
echo "Resources: tail -f $REPO_ROOT/.beads/logs/resource-monitor.log"
echo "Services: tail -f $REPO_ROOT/.beads/logs/service-monitor.log"
echo ""
echo "=== Remove Monitoring ==="
echo "Run: $SCRIPT_DIR/monitoring-remove.sh"
