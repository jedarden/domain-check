#!/usr/bin/env bash
# Comprehensive repository health check
# Runs all repository health diagnostics

set -e

echo "🏥 Running comprehensive repository health check..."
echo ""

# Script directory
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

# 1. Check repository size
echo "📊 Repository Size Check:"
if [ -f "$SCRIPT_DIR/check-repo-size.sh" ]; then
    bash "$SCRIPT_DIR/check-repo-size.sh"
else
    echo "⚠️  check-repo-size.sh not found"
fi
echo ""

# 2. Check for large files in history
echo "🔍 Large Files in Git History:"
LARGE_FILES=$(git rev-list --objects --all |
    git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)' |
    awk '/^blob/ {if ($3 > 10485760) print $3/1048576 " MB " $4}' |
    sort -rn | head -5)

if [ -n "$LARGE_FILES" ]; then
    echo "⚠️  Found large files in history (>10MB):"
    echo "$LARGE_FILES"
else
    echo "✅ No large files (>10MB) found in history"
fi
echo ""

# 3. Check git object count
echo "📦 Git Object Count:"
git count-objects -vH | grep -E "count|size-pack|size-garbage|in-pack" | head -10
echo ""

# 4. Check for large binary files in working directory
echo "💎 Large Binary Files in Working Directory:"
LARGE_BINARIES=$(find . -type f -size +10M -not -path "./.git/*" -not -path "./node_modules/*" -not -path "./target/*" 2>/dev/null || true)

if [ -n "$LARGE_BINARIES" ]; then
    echo "⚠️  Found large files in working directory (>10MB):"
    echo "$LARGE_BINARIES"
    echo "   Consider adding these to .gitignore or using Git LFS"
else
    echo "✅ No large files found in working directory"
fi
echo ""

# 5. Check repository fragmentation
echo "🧩 Repository Fragmentation:"
PACK_FILES=$(find .git/objects/pack -name "*.pack" 2>/dev/null | wc -l)
echo "Pack files: $PACK_FILES"

if [ "$PACK_FILES" -gt 20 ]; then
    echo "⚠️  High fragmentation (>$PACK_FILES pack files)"
    echo "   Consider running: git gc --aggressive"
else
    echo "✅ Acceptable fragmentation level"
fi
echo ""

# 6. Check git configuration
echo "⚙️  Git GC Configuration:"
git config --local --get-regexp "^gc\." | sed 's/^/  /' || echo "  No local GC configuration found"
echo ""

echo "✅ Comprehensive health check complete!"
