#!/bin/bash
# Setup script to configure git repository health settings
# This script configures automatic git GC and installs pre-commit hooks

set -e

echo "🔧 Configuring git repository health settings..."

# Configure git automatic GC thresholds
# These settings prevent repository bloat by running GC automatically
git config gc.auto 256          # Run GC when there are >256 loose objects
git config gc.autoPackLimit 2000  # Pack when >2000 loose objects
git config gc.packRefs true    # Pack refs during GC
git config gc.reflogExpire "90 days"  # Clean up reflog after 90 days
git config gc.reflogExpireUnreachable "30 days"  # Clean unreachable reflog after 30 days

echo "✅ Git GC configuration applied"
echo "   - gc.auto = 256 (run GC when >256 loose objects)"
echo "   - gc.autoPackLimit = 2000 (pack when >2000 loose objects)"
echo "   - gc.reflogExpire = 90 days"

# Install pre-commit hooks
if [ -d ".githooks" ]; then
    echo ""
    echo "🔧 Installing pre-commit hooks..."

    # Copy hooks from .githooks to .git/hooks
    for hook in .githooks/*; do
        if [ -f "$hook" ]; then
            hook_name=$(basename "$hook")
            echo "   Installing $hook_name..."
            cp "$hook" ".git/hooks/$hook_name"
            chmod +x ".git/hooks/$hook_name"
        fi
    done

    # Configure git to use local hooks
    git config core.hooksPath .githooks

    echo "✅ Pre-commit hooks installed"
    echo "   - Large file blocker (>10MB)"
    echo "   - Repository size monitoring"
else
    echo "⚠️  No .githooks directory found"
fi

echo ""
echo "✨ Git repository health setup complete!"
echo ""
echo "💡 Tips to maintain repository health:"
echo "   1. Run 'git status' to check for large files before committing"
echo "   2. The pre-commit hook will automatically block large files"
echo "   3. Git will automatically run GC when needed"
echo "   4. Run './scripts/repo-health-check.sh' to check repository health"
