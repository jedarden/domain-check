#!/usr/bin/env bash
# Install automated git GC scheduling for repository bloat prevention
# Prevents OOM crashes by running periodic garbage collection
#
# Usage: ./scripts/install-git-gc-cron.sh [options]
#   --dry-run        Show what would be installed without installing
#   --uninstall      Remove existing git GC cron jobs
#   --verify         Verify installation status
#
# Exit codes:
#   0 - Success
#   1 - Installation failed
#   2 - Invalid arguments

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
SAFE_GC="$SCRIPT_DIR/safe-git-gc.sh"
AUTO_GC="$SCRIPT_DIR/auto-gc-trigger.sh"
CRON_MARKER="# === Domain Check Git GC Scheduling"

# Parse arguments
DRY_RUN=false
UNINSTALL=false
VERIFY=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    --uninstall)
      UNINSTALL=true
      shift
      ;;
    --verify)
      VERIFY=true
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [options]"
      echo "Options:"
      echo "  --dry-run        Show what would be installed without installing"
      echo "  --uninstall      Remove existing git GC cron jobs"
      echo "  --verify         Verify installation status"
      echo "  -h, --help       Show this help message"
      exit 0
      ;;
    *)
      echo "Error: Unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

# Verify installation status
if [[ "$VERIFY" == true ]]; then
  echo "=== Git GC Scheduling Verification ==="
  echo ""

  if crontab -l > /dev/null 2>&1; then
    if crontab -l 2>/dev/null | grep -q "$CRON_MARKER"; then
      echo "✅ Git GC cron jobs are installed"
      echo ""
      echo "=== Scheduled Jobs ==="
      crontab -l 2>/dev/null | grep -A 10 "$CRON_MARKER"
      exit 0
    else
      echo "❌ Git GC cron jobs are NOT installed"
      echo ""
      echo "Install with: $0"
      exit 1
    fi
  else
    echo "❌ No crontab exists (git GC jobs not installed)"
    exit 1
  fi
fi

# Uninstall existing jobs
if [[ "$UNINSTALL" == true ]]; then
  echo "=== Removing Git GC Scheduling ==="

  if crontab -l > /dev/null 2>&1; then
    if crontab -l 2>/dev/null | grep -q "$CRON_MARKER"; then
      TEMP_CRON=$(mktemp)
      crontab -l > "$TEMP_CRON"
      grep -v "$CRON_MARKER" "$TEMP_CRON" > "$TEMP_CRON.new" || true
      crontab "$TEMP_CRON.new"
      rm "$TEMP_CRON" "$TEMP_CRON.new"

      echo "✅ Git GC cron jobs removed"
      echo ""
      echo "Remaining jobs:"
      crontab -l 2>/dev/null | grep -v "^#" | grep -v "^$" || echo "  (none)"
      exit 0
    else
      echo "⚠️  No git GC cron jobs found (nothing to remove)"
      exit 0
    fi
  else
    echo "⚠️  No crontab exists (nothing to remove)"
    exit 0
  fi
fi

# Install cron jobs
echo "=== Installing Git GC Scheduling ==="
echo "Repository: $REPO_ROOT"
echo ""

# Check if scripts exist
if [[ ! -x "$SAFE_GC" ]]; then
  echo "❌ Safe GC script not found or not executable: $SAFE_GC"
  exit 1
fi

if [[ ! -x "$AUTO_GC" ]]; then
  echo "❌ Auto GC trigger script not found or not executable: $AUTO_GC"
  exit 1
fi

# Create temporary crontab file
TEMP_CRON=$(mktemp)
if crontab -l > /dev/null 2>&1; then
  crontab -l > "$TEMP_CRON"
fi

# Check if jobs already exist
if grep -q "$CRON_MARKER" "$TEMP_CRON" 2>/dev/null; then
  echo "⚠️  Git GC cron jobs already exist"
  echo "Run: $0 --uninstall to remove existing jobs"
  echo "Run: $0 --verify to check current installation"
  rm "$TEMP_CRON"
  exit 1
fi

# Add cron jobs
cat >> "$TEMP_CRON" << EOF

$CRON_MARKER (installed $(date))
# Repository health check daily at 2 AM
0 2 * * * cd $REPO_ROOT && $AUTO_GC --dry-run >> $REPO_ROOT/.beads/logs/git-gc-check.log 2>&1

# Standard git gc daily at 3 AM (stages 1-2, ~10-30 minutes)
0 3 * * * cd $REPO_ROOT && $SAFE_GC >> $REPO_ROOT/.beads/logs/git-gc.log 2>&1

# Full git gc weekly on Sunday at 4 AM (all stages, ~1-2 hours)
0 4 * * 0 cd $REPO_ROOT && $SAFE_GC --full >> $REPO_ROOT/.beads/logs/git-gc-full.log 2>&1
EOF

if [[ "$DRY_RUN" == true ]]; then
  echo "🔍 DRY RUN: Would install the following cron jobs:"
  echo ""
  grep -A 10 "$CRON_MARKER" "$TEMP_CRON"
  rm "$TEMP_CRON"
  echo ""
  echo "To actually install, run: $0"
  exit 0
fi

# Install crontab
crontab "$TEMP_CRON"
rm "$TEMP_CRON"

echo "✅ Git GC cron jobs installed"
echo ""
echo "=== Installed Jobs ==="
crontab -l 2>/dev/null | grep -A 10 "$CRON_MARKER"
echo ""
echo "=== Schedule ==="
echo "1. Repository health check: Daily at 2 AM (dry-run check)"
echo "2. Standard git gc: Daily at 3 AM (~10-30 minutes)"
echo "3. Full git gc: Weekly on Sunday at 4 AM (~1-2 hours)"
echo ""
echo "=== Log Files ==="
echo "Health check: $REPO_ROOT/.beads/logs/git-gc-check.log"
echo "Standard GC: $REPO_ROOT/.beads/logs/git-gc.log"
echo "Full GC: $REPO_ROOT/.beads/logs/git-gc-full.log"
echo ""
echo "=== Monitor GC Progress ==="
echo "Watch: $SCRIPT_DIR/safe-git-gc-monitor.sh --watch"
echo ""
echo "=== Verify Installation ==="
echo "Run: $0 --verify"
echo ""
echo "=== Uninstall ==="
echo "Run: $0 --uninstall"
