#!/bin/bash
# Repository Health Monitoring Script
# Checks for repository bloat, large objects, and other health issues

set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "=== Repository Health Check ==="
echo "Repository: $REPO_ROOT"
echo "Date: $(date)"
echo

# Color codes for output
RED='\033[0;31m'
YELLOW='\033[1;33m'
GREEN='\033[0;32m'
NC='\033[0m' # No Color

# Warning thresholds
MAX_REPO_SIZE_GB=1
MAX_LOOSE_OBJECTS=1000
MAX_PACK_FILE_COUNT=10

health_score=100
issues=()

# 1. Check repository size
echo "1. Repository Size"
repo_size_gb=$(du -s .git | awk '{printf "%.1f", $1/1024/1024}')
echo "   Current: ${repo_size_gb}GB"
if (( $(echo "$repo_size_gb > $MAX_REPO_SIZE_GB" | bc -l) )); then
    echo -e "   ${RED}❌ FAIL: Repository size exceeds ${MAX_REPO_SIZE_GB}GB${NC}"
    issues+=("Repository size: ${repo_size_gb}GB (limit: ${MAX_REPO_SIZE_GB}GB)")
    health_score=$((health_score - 30))
else
    echo -e "   ${GREEN}✅ OK${NC}"
fi
echo

# 2. Check loose objects
echo "2. Loose Objects"
loose_count=$(git count-objects -v 2>/dev/null | grep "^count:" | awk '{print $2}' || echo "0")
echo "   Current: $loose_count loose objects"
if [ "$loose_count" -gt "$MAX_LOOSE_OBJECTS" ]; then
    echo -e "   ${YELLOW}⚠️  WARNING: High number of loose objects${NC}"
    echo "   Consider running: git gc --aggressive"
    issues+=("High loose object count: $loose_count")
    health_score=$((health_score - 10))
else
    echo -e "   ${GREEN}✅ OK${NC}"
fi
echo

# 3. Check pack files
echo "3. Pack Files"
pack_count=$(ls .git/objects/pack/*.pack 2>/dev/null | wc -l)
echo "   Current: $pack_count pack files"
if [ "$pack_count" -gt "$MAX_PACK_FILE_COUNT" ]; then
    echo -e "   ${YELLOW}⚠️  WARNING: High number of pack files${NC}"
    echo "   Consider running: git gc --aggressive"
    issues+=("High pack file count: $pack_count")
    health_score=$((health_score - 10))
else
    echo -e "   ${GREEN}✅ OK${NC}"
fi
echo

# 4. Check for large files in current tree
echo "4. Large Files in Working Tree"
large_files=$(find . -type f -size +10M -not -path "./.git/*" -not -path "./node_modules/*" -not -path "./dist/*" 2>/dev/null)
if [ -n "$large_files" ]; then
    echo -e "   ${YELLOW}⚠️  WARNING: Found files larger than 10MB:${NC}"
    echo "$large_files" | head -10 | sed 's/^/   - /'
    issues+=("Large files found in working tree")
    health_score=$((health_score - 5))
else
    echo -e "   ${GREEN}✅ OK${NC}"
fi
echo

# 5. Check for common problematic patterns
echo "5. Problematic File Patterns"
problematic_files=$(find . -name "*.jsonl" -o -name "*.db" -o -name "*.db.backup*" 2>/dev/null | grep -v ".git/" | head -5)
if [ -n "$problematic_files" ]; then
    echo -e "   ${YELLOW}⚠️  WARNING: Found problematic file patterns:${NC}"
    echo "$problematic_files" | sed 's/^/   - /'
    echo "   Ensure these are in .gitignore"
    issues+=("Problematic file patterns found")
    health_score=$((health_score - 5))
else
    echo -e "   ${GREEN}✅ OK${NC}"
fi
echo

# 6. Check git config
echo "6. Git Configuration"
if git config gc.auto > /dev/null 2>&1; then
    echo "   ✅ Auto GC configured: $(git config gc.auto)"
else
    echo -e "   ${YELLOW}⚠️  Auto GC not configured${NC}"
    health_score=$((health_score - 5))
fi
echo

# 7. Check branch status
echo "7. Branch Status"
branch_status=$(git status --short | wc -l)
echo "   Uncommitted changes: $branch_status files"
if [ "$branch_status" -gt 50 ]; then
    echo -e "   ${YELLOW}⚠️  WARNING: High number of uncommitted changes${NC}"
    issues+=("High uncommitted change count: $branch_status")
    health_score=$((health_score - 5))
else
    echo -e "   ${GREEN}✅ OK${NC}"
fi
echo

# 8. Summary
echo "=== Health Summary ==="
echo -n "Overall Health Score: "
if [ $health_score -ge 80 ]; then
    echo -e "${GREEN}$health_score/100${NC}"
    echo "Status: HEALTHY"
elif [ $health_score -ge 50 ]; then
    echo -e "${YELLOW}$health_score/100${NC}"
    echo "Status: DEGRADED"
else
    echo -e "${RED}$health_score/100${NC}"
    echo "Status: UNHEALTHY"
fi
echo

if [ ${#issues[@]} -gt 0 ]; then
    echo "Issues Found:"
    printf '   - %s\n' "${issues[@]}"
    echo
    echo "Recommendations:"
    echo "1. Run repository cleanup if size is too large"
    echo "2. Check .gitignore includes problematic file patterns"
    echo "3. Run git gc --aggressive if loose object count is high"
    echo "4. Review and commit or clean up uncommitted changes"
fi

if [ $health_score -ge 50 ]; then
    exit 0
else
    exit 1
fi
