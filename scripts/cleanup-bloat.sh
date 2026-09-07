#!/bin/bash
# Repository cleanup script
# Runs git gc with aggressive cleanup to reduce repository size

set -euo pipefail

echo "Starting repository cleanup..."
echo "Current repository size: $(du -sh .git | cut -f1)"

echo "Running git gc --aggressive --prune=now..."
echo "This may take several minutes for large repositories..."

# Run aggressive garbage collection
git gc --aggressive --prune=now

echo "Cleanup complete!"
echo "New repository size: $(du -sh .git | cut -f1)"

# Show space saved
BEFORE_SIZE=$(du -s .git | awk '{print $1}')
echo "Repository object count: $(git count-objects -vH | grep -E 'count|size-pack' | head -2)"
