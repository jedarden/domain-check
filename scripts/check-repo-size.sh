#!/bin/bash
# Repository health monitoring script
# Checks repository size and alerts if it exceeds thresholds

set -euo pipefail

# Thresholds (in MB for easier integer comparison)
WARN_THRESHOLD_MB=2048    # 2 GB
CRITICAL_THRESHOLD_MB=5120  # 5 GB

# Get repository size in MB
REPO_SIZE_MB=$(du -s .git | awk '{print int($1/1024)}')
REPO_SIZE_GB=$(echo "scale=1; $REPO_SIZE_MB/1024" | awk '{printf "%.1f", $1}')

echo "Repository size: ${REPO_SIZE_GB} GB (${REPO_SIZE_MB} MB)"

if [ "$REPO_SIZE_MB" -ge "$CRITICAL_THRESHOLD_MB" ]; then
    echo "❌ CRITICAL: Repository size exceeds $(($CRITICAL_THRESHOLD_MB/1024)) GB!"
    echo "Repository health is critical. Immediate cleanup required."
    echo "Consider running: git gc --aggressive --prune=now"
    exit 2
elif [ "$REPO_SIZE_MB" -ge "$WARN_THRESHOLD_MB" ]; then
    echo "⚠️  WARNING: Repository size exceeds $(($WARN_THRESHOLD_MB/1024)) GB"
    echo "Consider running: git gc --aggressive"
    exit 1
else
    echo "✅ Repository size is healthy"
    exit 0
fi
