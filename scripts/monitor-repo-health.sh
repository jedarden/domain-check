#!/bin/bash
# Repository health monitoring script
# Tracks repository size and git object counts to detect bloat early

set -euo pipefail

# Configuration
REPO_SIZE_WARNING=$((500 * 1024 * 1024))  # 500MB in KB (du reports in KB)
LOOSE_OBJECTS_WARNING=1000
LOG_FILE="${LOG_FILE:-.beads/logs/repo-health.log}"

# Ensure log directory exists
mkdir -p "$(dirname "$LOG_FILE")"

# Get repository size (in KB)
REPO_SIZE_KB=$(du -sk .git 2>/dev/null | awk '{print $1}')
REPO_SIZE_MB=$((REPO_SIZE_KB / 1024))

# Get git object statistics
GIT_STATS=$(git count-objects -vH 2>/dev/null || true)
LOOSE_OBJECTS=$(echo "$GIT_STATS" | grep '^in-pack:' | awk '{print $2}' || echo 0)
PACK_COUNT=$(echo "$GIT_STATS" | grep '^packs:' | awk '{print $2}' || echo 0)
PRUNE_PACKABLE=$(echo "$GIT_STATS" | grep '^prune-packable:' | awk '{print $2}' || echo 0)
GARBAGE=$(echo "$GIT_STATS" | grep '^size-garbage:' | awk '{print $2}' || echo 0)

# Get timestamp
TIMESTAMP=$(date -Iseconds)

# Log the metrics
{
    echo "[$TIMESTAMP] Repository Health Check"
    echo "  .git size: ${REPO_SIZE_MB}MB (${REPO_SIZE_KB}KB)"
    echo "  Loose objects: $LOOSE_OBJECTS"
    echo "  Pack files: $PACK_COUNT"
    echo "  Prune-packable: $PRUNE_PACKABLE"
    echo "  Garbage: $GARBAGE"
} | tee -a "$LOG_FILE"

# Check thresholds and issue warnings
WARNING=0

if [ "$REPO_SIZE_KB" -gt "$REPO_SIZE_WARNING" ]; then
    echo "  ⚠️  WARNING: Repository size exceeds 500MB (${REPO_SIZE_MB}MB)"
    echo "  Consider running: git gc --aggressive"
    WARNING=1
fi

if [ "$LOOSE_OBJECTS" -gt "$LOOSE_OBJECTS_WARNING" ]; then
    echo "  ⚠️  WARNING: Loose objects count > $LOOSE_OBJECTS_WARNING (current: $LOOSE_OBJECTS)"
    echo "  Consider running: git gc"
    WARNING=1
fi

if [ "$GARBAGE" != "0" ] && [ "$GARBAGE" != "0 bytes" ]; then
    echo "  ⚠️  WARNING: Garbage objects detected: $GARBAGE"
    echo "  Consider running: git gc --prune=now"
    WARNING=1
fi

if [ "$WARNING" -eq 0 ]; then
    echo "  ✅ Repository health is good"
fi

exit 0
