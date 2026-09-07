#!/bin/bash
# Repository health check script
# Monitors repository size and detects potential issues

set -e

# Color codes for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Thresholds
MAX_REPO_SIZE_MB=500
WARN_REPO_SIZE_MB=250
MAX_LOOSE_OBJECTS=1000
WARN_LOOSE_OBJECTS=500

echo "🏥 Repository Health Check"
echo "========================="
echo ""

# Check repository size
echo "📊 Repository size analysis:"
git count-objects -vH | while read -r line; do
    echo "   $line"
done
echo ""

# Get current repository stats
size_bytes=$(git count-objects -v | grep "size" | head -1 | cut -d: -f2 | tr -d ' ')
size_mb=$((size_bytes / 1024 / 1024))
loose_objects=$(git count-objects -v | grep "count" | head -1 | cut -d: -f2 | tr -d ' ')

echo "📈 Health Assessment:"
echo "===================="

# Check repository size
if [ "$size_mb" -gt "$MAX_REPO_SIZE_MB" ]; then
    echo -e "${RED}❌ CRITICAL: Repository size is ${size_mb}MB (exceeds ${MAX_REPO_SIZE_MB}MB)${NC}"
    echo -e "${RED}   Action required: Run 'git gc --aggressive --prune=now'${NC}"
elif [ "$size_mb" -gt "$WARN_REPO_SIZE_MB" ]; then
    echo -e "${YELLOW}⚠️  WARNING: Repository size is ${size_mb}MB (exceeds ${WARN_REPO_SIZE_MB}MB)${NC}"
    echo -e "${YELLOW}   Recommendation: Consider running 'git gc'${NC}"
else
    echo -e "${GREEN}✅ Repository size is healthy (${size_mb}MB)${NC}"
fi

# Check loose objects
if [ "$loose_objects" -gt "$MAX_LOOSE_OBJECTS" ]; then
    echo -e "${RED}❌ CRITICAL: $loose_objects loose objects (exceeds ${MAX_LOOSE_OBJECTS})${NC}"
    echo -e "${RED}   Action required: Run 'git gc' to pack objects${NC}"
elif [ "$loose_objects" -gt "$WARN_LOOSE_OBJECTS" ]; then
    echo -e "${YELLOW}⚠️  WARNING: $loose_objects loose objects (exceeds ${WARN_LOOSE_OBJECTS})${NC}"
    echo -e "${YELLOW}   Recommendation: Git will auto-GC soon${NC}"
else
    echo -e "${GREEN}✅ Loose objects count is healthy ($loose_objects)${NC}"
fi

echo ""
echo "🔍 Large files in repository history:"
echo "======================================"

# Find large files in current revision
git rev-list --objects --all |
    git cat-file --batch-check --batch-all-objects |
    awk -v size=$((10 * 1024 * 1024)) '$3 > size {print $1 $3 $4}' |
    sort -k3 -n |
    head -n 10 |
    while read -r line; do
        if [ -n "$line" ]; then
            size=$(echo "$line" | awk '{print $2/1024/1024 " MB"}')
            hash=$(echo "$line" | awk '{print $1}')
            echo "   $size - $(git rev-list --objects --all | grep "$hash" | head -1 | awk '{print $2}')"
        fi
    done || echo "   No large files found in current revision"

echo ""
echo "💡 Recommendations:"
echo "==================="
echo "1. Run './scripts/setup-git-gc-config.sh' to configure automatic GC"
echo "2. Ensure .gitignore includes .beads/ and *.jsonl"
echo "3. Review pre-commit hooks to prevent large file commits"
echo "4. For critical issues, consider repository history rewrite (caution!)"

echo ""
echo "📋 Git Configuration Check:"
echo "============================"

if git config gc.auto > /dev/null 2>&1; then
    echo "✅ Git GC is configured (gc.auto = $(git config gc.auto))"
else
    echo "⚠️  Git GC not configured - run setup script"
fi

echo ""
echo "✨ Health check complete!"
