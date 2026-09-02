#!/bin/bash
# Remove continuous monitoring for crash prevention

set -e

echo "=== Removing Domain Check Monitoring ==="
echo ""

# Check if crontab exists
if ! crontab -l > /dev/null 2>&1; then
    echo "No crontab found - nothing to remove"
    exit 0
fi

# Create temporary crontab file
TEMP_CRON=$(mktemp)
crontab -l > "$TEMP_CRON"

# Check if monitoring jobs exist
if ! grep -q "domain-check.*monitoring" "$TEMP_CRON" 2>/dev/null; then
    echo "⚠️  No monitoring cron jobs found"
    rm "$TEMP_CRON"
    exit 0
fi

# Remove monitoring jobs
grep -v "domain-check.*monitoring" "$TEMP_CRON" > "$TEMP_CRON.new" || true
mv "$TEMP_CRON.new" "$TEMP_CRON"

# Install updated crontab
crontab "$TEMP_CRON"
rm "$TEMP_CRON"

echo "✅ Monitoring cron jobs removed"
echo ""
echo "=== Log Files Preserved ==="
echo "The following log files are preserved for review:"
echo "  - .beads/logs/crash-monitor.log"
echo "  - .beads/logs/resource-monitor.log"
echo "  - .beads/logs/service-monitor.log"
echo ""
echo "To reinstall monitoring, run: scripts/monitoring-setup.sh"
