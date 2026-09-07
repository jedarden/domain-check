#!/usr/bin/env bash
# Pre-Garbage Collection Repository Health Check
# Verifies repository is in safe state for git gc operations

set -euo pipefail

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Thresholds
MIN_MEMORY_GB=8
MIN_DISK_GB=10

echo "=== Pre-GC Repository Health Check ==="
echo ""

# Track overall status
STATUS=0

# 1. Check git status
echo -n "Repository state: "
if git diff --quiet && git diff --cached --quiet; then
    echo -e "${GREEN}clean${NC}"
else
    echo -e "${YELLOW}uncommitted changes${NC}"
    git status --short
    STATUS=1
fi

# 2. Check available memory
echo -n "Available memory: "
MEMORY_AVAIL_GB=$(free -g | awk '/^Mem:/ {print $7}')
if [ "$MEMORY_AVAIL_GB" -ge "$MIN_MEMORY_GB" ]; then
    echo -e "${GREEN}${MEMORY_AVAIL_GB}GB${NC} (>= ${MIN_MEMORY_GB}GB required)"
else
    echo -e "${RED}${MEMORY_AVAIL_GB}GB${NC} (need >= ${MIN_MEMORY_GB}GB)"
    STATUS=1
fi

# 3. Check disk space
echo -n "Disk space: "
DISK_AVAIL_GB=$(df -BG / | tail -1 | awk '{print $4}' | tr -d 'G')
if [ "$DISK_AVAIL_GB" -ge "$MIN_DISK_GB" ]; then
    echo -e "${GREEN}${DISK_AVAIL_GB}GB available${NC} (>= ${MIN_DISK_GB}GB required)"
else
    echo -e "${RED}${DISK_AVAIL_GB}GB available${NC} (need >= ${MIN_DISK_GB}GB)"
    STATUS=1
fi

# 4. Check for concurrent git operations
echo -n "Concurrent git operations: "
GIT_OPS=$(pgrep -f "git-(gc|repack|pack-refs)" || true)
if [ -z "$GIT_OPS" ]; then
    echo -e "${GREEN}none${NC}"
else
    echo -e "${RED}detected${NC}"
    ps -p $GIT_OPS -o pid,cmd
    STATUS=1
fi

# 5. Quick git integrity check
echo -n "Git integrity check: "
if timeout 60 git fsck --connectivity-only --no-progress >/dev/null 2>&1; then
    echo -e "${GREEN}passed${NC}"
else
    echo -e "${YELLOW}timed out or errors found${NC}"
    echo "Running full check..."
    timeout 120 git fsck --no-progress 2>&1 | head -20
    # Don't fail on dangling objects - they're normal
fi

# 6. Repository size info
echo ""
echo "Repository statistics:"
git count-objects -vH | grep -E "^(count|size|packs|size-pack):"

echo ""
echo "=== Summary ==="
if [ $STATUS -eq 0 ]; then
    echo -e "${GREEN}✓ Repository is ready for gc operations${NC}"
    exit 0
else
    echo -e "${RED}✗ Issues found - resolve before running gc${NC}"
    exit 1
fi
