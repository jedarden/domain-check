#!/bin/bash
# Repository Bloat Cleanup Script
# Removes large files from git history and runs aggressive garbage collection
# Usage: ./cleanup-repo-bloat.sh

set -e

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

echo "=== Repository Bloat Cleanup ==="
echo "Repository root: $REPO_ROOT"
echo

# Check current repository size
echo "Current repository size:"
du -sh .git
echo

# Run git fsck to check for corruption
echo "Running git fsck..."
timeout 30 git fsck --no-full || echo "fsck timed out or found issues (expected on large repos)"
echo

# Remove large files from git history
echo "Removing large files from git history..."

# Use git filter-repo or BFG if available, otherwise use git filter-branch
if command -v git-filter-repo &> /dev/null; then
    echo "Using git-filter-repo..."
    git filter-repo --invert-paths \
        --path .beads/beads.base.jsonl \
        --path .beads/beads.db \
        --path .beads/issues.jsonl \
        --path .beads/events.jsonl \
        --path-glob '*.db.backup.*' \
        --force
elif command -v bfg &> /dev/null; then
    echo "Using BFG Repo-Cleaner..."
    bfg --strip-blobs-bigger-than 10M
    git reflog expire --expire=now --all && git gc --aggressive --prune=now
else
    echo "Using git filter-branch (slower but built-in)..."
    git filter-branch --force --index-filter \
        'git rm --cached --ignore-unmatch .beads/beads.base.jsonl .beads/beads.db .beads/issues.jsonl .beads/events.jsonl' \
        --prune-empty --tag-name-filter cat -- --all
fi

echo

# Clean up references
echo "Cleaning up references..."
rm -rf .git/refs/original/
git reflog expire --expire=now --all
git gc --aggressive --prune=now
echo

# Set reasonable git gc thresholds to prevent recurrence
echo "Configuring git gc thresholds..."
git config gc.aggressiveDepth 50
git config gc.aggressiveWindow 250
git config gc.auto 256
git config gc.autoPackLimit 50
echo

# Show new repository size
echo "New repository size:"
du -sh .git
echo

echo "=== Cleanup Complete ==="
echo "Repository has been cleaned and optimized."
echo "Please verify that git history is intact before force-pushing."
