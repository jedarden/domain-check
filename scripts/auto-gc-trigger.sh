#!/usr/bin/env bash
# Automatic garbage collection trigger for repository bloat prevention
# Runs safe git GC when repository exceeds thresholds
#
# Usage: ./scripts/auto-gc-trigger.sh [options]
#   --dry-run        Show what would be done without executing
#   --force          Force GC even if below threshold
#   --aggressive     Use aggressive GC mode (slower but better compression)
#
# Exit codes:
#   0 - GC completed successfully or not needed
#   1 - GC failed
#   2 - Invalid arguments

set -euo pipefail

# Configuration
AUTO_GC_THRESHOLD_MB=10240  # 10 GB - trigger automatic GC
WARN_THRESHOLD_MB=2048      # 2 GB - warn but don't GC
DRY_RUN=false
FORCE=false
AGGRESSIVE=false

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    --force)
      FORCE=true
      shift
      ;;
    --aggressive)
      AGGRESSIVE=true
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [options]"
      echo "Options:"
      echo "  --dry-run        Show what would be done without executing"
      echo "  --force          Force GC even if below threshold"
      echo "  --aggressive     Use aggressive GC mode (slower but better compression)"
      echo "  -h, --help       Show this help message"
      echo ""
      echo "Thresholds:"
      echo "  Auto GC trigger: $(($AUTO_GC_THRESHOLD_MB/1024)) GB"
      echo "  Warning threshold: $(($WARN_THRESHOLD_MB/1024)) GB"
      exit 0
      ;;
    *)
      echo "Error: Unknown argument: $1" >&2
      echo "Run '$0 --help' for usage" >&2
      exit 2
      ;;
  esac
done

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
  echo "Error: Not in a git repository" >&2
  exit 1
fi

# Get repository size
REPO_SIZE_MB=$(du -s .git | awk '{print int($1/1024)}')
REPO_SIZE_GB=$(echo "scale=1; $REPO_SIZE_MB/1024" | awk '{printf "%.1f", $1}')

echo "=== Repository GC Trigger Check ==="
echo "Repository size: ${REPO_SIZE_GB} GB (${REPO_SIZE_MB} MB)"
echo "Auto GC threshold: $(($AUTO_GC_THRESHOLD_MB/1024)) GB"
echo ""

# Get detailed stats
git_stats=$(git count-objects -vH 2>/dev/null || echo "")
loose_objects=$(echo "$git_stats" | grep "^count:" | awk '{print $2}')
pack_size_mb=$(echo "$git_stats" | grep "^size-pack:" | awk '{print $2}' | sed 's/MiB//')

echo "Statistics:"
echo "  Loose objects: $loose_objects"
echo "  Pack size: ${pack_size_mb}MiB"
echo ""

# Check if GC is needed
if [ "$FORCE" = true ]; then
  echo "🔧 FORCE MODE: Running GC regardless of size"
  NEEDS_GC=true
elif [ "$REPO_SIZE_MB" -ge "$AUTO_GC_THRESHOLD_MB" ]; then
  echo "⚠️  Repository size exceeds auto GC threshold"
  NEEDS_GC=true
else
  echo "✅ Repository size below threshold, GC not needed"
  NEEDS_GC=false
fi

if [ "$NEEDS_GC" = false ]; then
  if [ "$REPO_SIZE_MB" -ge "$WARN_THRESHOLD_MB" ]; then
    echo "⚠️  WARNING: Repository is large but below auto GC threshold"
    echo "   Consider running: ./scripts/safe-git-gc.sh"
  fi
  exit 0
fi

# Run GC
if [ "$DRY_RUN" = true ]; then
  echo "🔍 DRY RUN: Would run git GC"
  if [ "$AGGRESSIVE" = true ]; then
    echo "   Command: git gc --aggressive --prune=now"
  else
    echo "   Command: git gc"
  fi
  exit 0
fi

# Check if safe-git-gc.sh exists
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SAFE_GC="$SCRIPT_DIR/safe-git-gc.sh"

if [ -x "$SAFE_GC" ]; then
  echo "🧹 Running safe git GC..."

  if [ "$AGGRESSIVE" = true ]; then
    "$SAFE_GC" --full
  else
    "$SAFE_GC"
  fi

  GC_EXIT=$?

  if [ $GC_EXIT -eq 0 ]; then
    echo "✅ Safe GC completed successfully"

    # Show new size
    NEW_SIZE_MB=$(du -s .git | awk '{print int($1/1024)}')
    NEW_SIZE_GB=$(echo "scale=1; $NEW_SIZE_MB/1024" | awk '{printf "%.1f", $1}')
    SAVED_MB=$((REPO_SIZE_MB - NEW_SIZE_MB))
    SAVED_GB=$(echo "scale=1; $SAVED_MB/1024" | awk '{printf "%.1f", $1}')

    echo ""
    echo "Before: ${REPO_SIZE_GB} GB"
    echo "After:  ${NEW_SIZE_GB} GB"
    echo "Saved:  ${SAVED_GB} GB (${SAVED_MB} MB)"
    exit 0
  else
    echo "❌ Safe GC failed with exit code $GC_EXIT" >&2
    exit 1
  fi
else
  echo "⚠️  Safe GC script not found, falling back to standard git gc"
  echo "   Consider using: $SAFE_GC"
  echo ""

  # Standard git gc
  if [ "$AGGRESSIVE" = true ]; then
    echo "🧹 Running aggressive git gc..."
    git gc --aggressive --prune=now
  else
    echo "🧹 Running standard git gc..."
    git gc
  fi

  if [ $? -eq 0 ]; then
    echo "✅ Git GC completed successfully"
    exit 0
  else
    echo "❌ Git GC failed" >&2
    exit 1
  fi
fi
