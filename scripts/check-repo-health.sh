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
LARGE_BINARIES=$(find . -type f -size +10M -not -path "./.git/*" -not -path "./.beads/*" -not -path "./node_modules/*" -not -path "./target/*" 2>/dev/null || true)

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

# 9. Unpushed-commit backlog (gap analysis M-1, docs/crash-prevention-gaps-bf-1ea4g.md):
# bf-1ea4g died 56 times in `git push` against a 422-commit unpushed backlog that
# accumulated silently across ~30 killed attempts — nothing measured commit-ahead
# between close-time gates. Report-only per the G-2 correction: remediation stays
# with the unconditional bounded nightly gc; this check only names the precondition.
if BACKLOG_OUT="$(bash "$SCRIPT_DIR/check-unpushed-backlog.sh" "$REPO_ROOT" 2>&1)"; then
    BACKLOG_RC=0
else
    BACKLOG_RC=$?
fi
sed 's/^/  /' <<<"$BACKLOG_OUT"
mkdir -p "$REPO_ROOT/.beads/logs"
if [ "$BACKLOG_RC" -eq 1 ]; then
    echo "$(date -u +"%Y-%m-%dT%H:%M:%SZ") CRITICAL repo-health: unpushed backlog at or above 200 commits — a push will materialize the whole series in one pack-objects run (bf-1ea4g shape)" >> "$REPO_ROOT/.beads/logs/repo-health.log"
elif [ "$BACKLOG_RC" -eq 0 ] && grep -q "⚠️  WARN" <<<"$BACKLOG_OUT"; then
    echo "$(date -u +"%Y-%m-%dT%H:%M:%SZ") WARN repo-health: unpushed backlog at or above 50 commits" >> "$REPO_ROOT/.beads/logs/repo-health.log"
elif [ "$BACKLOG_RC" -eq 2 ]; then
    echo "⚠️  backlog check skipped (usage error)"
fi
echo ""

echo "✅ Comprehensive health check complete!"
