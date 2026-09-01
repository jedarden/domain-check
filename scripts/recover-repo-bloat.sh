#!/bin/bash
# Automated repository recovery script
# Diagnoses and repairs repository bloat issues

set -euo pipefail

REPO_SIZE_THRESHOLD=$((500 * 1024 * 1024))  # 500MB in KB (du reports in KB)

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${GREEN}✅ $1${NC}"
}

log_warn() {
    echo -e "${YELLOW}⚠️  $1${NC}"
}

log_error() {
    echo -e "${RED}❌ $1${NC}"
}

# Get current repository size
get_repo_size() {
    du -sk .git 2>/dev/null | awk '{print $1}'
}

# Check repository health
check_repo_health() {
    local repo_size_kb=$1

    echo ""
    echo "=== Repository Health Check ==="

    if [ "$repo_size_kb" -lt "$REPO_SIZE_THRESHOLD" ]; then
        local repo_size_mb=$((repo_size_kb / 1024))
        log_info "Repository size is healthy (${repo_size_mb}MB < 500MB threshold)"
        return 0
    else
        local repo_size_mb=$((repo_size_kb / 1024))
        log_warn "Repository bloat detected (${repo_size_mb}MB exceeds 500MB threshold)"
        return 1
    fi
}

# Run repository integrity check
check_repo_integrity() {
    echo ""
    echo "Step 1: Checking repository integrity..."
    if git fsck --full 2>&1 | grep -q "dangling"; then
        log_warn "Dangling objects found (will be cleaned up by gc)"
    else
        log_info "Repository integrity check passed"
    fi
}

# Perform conservative cleanup
run_conservative_gc() {
    echo ""
    echo "Step 2: Running conservative git gc..."
    git gc
    log_info "Conservative cleanup complete"
}

# Perform aggressive cleanup if still needed
run_aggressive_gc() {
    echo ""
    echo "Step 3: Running aggressive git gc..."
    log_warn "This may take several minutes..."
    git gc --aggressive --prune=now
    log_info "Aggressive cleanup complete"
}

# Show largest files in git history
show_largest_files() {
    echo ""
    echo "=== Largest Files in Git History ==="
    echo "Investigate these files for potential removal or .gitignore addition:"
    echo ""

    git rev-list --objects --all 2>/dev/null | \
    git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)' 2>/dev/null | \
    awk '/^blob/ {print substr($0,6)}' | \
    sort -nk2 | \
    tail -10 | \
    while read -r size rest; do
        local size_mb=$((size / 1024 / 1024))
        printf "  %6dMB - %s\n" "$size_mb" "$rest"
    done
}

# Main execution
main() {
    local initial_size=$(get_repo_size)

    echo "=== Repository Recovery Script ==="
    echo "Started at: $(date)"
    echo "Initial repository size: $((initial_size / 1024))MB"

    # Check if recovery is needed
    if ! check_repo_health "$initial_size"; then
        # Recovery is needed
        echo ""
        check_repo_integrity
        run_conservative_gc

        local after_conservative=$(get_repo_size)

        if [ "$after_conservative" -gt "$REPO_SIZE_THRESHOLD" ]; then
            run_aggressive_gc
        fi

        local final_size=$(get_repo_size)
        local initial_mb=$((initial_size / 1024))
        local final_mb=$((final_size / 1024))
        local reduction=$((initial_size - final_size))
        local reduction_mb=$((reduction / 1024))

        echo ""
        log_info "Recovery complete: ${initial_mb}MB → ${final_mb}MB (${reduction_mb}MB reduction)"

        # Final health check
        if check_repo_health "$final_size"; then
            echo ""
            log_info "Repository is now healthy"
            exit 0
        else
            echo ""
            log_error "Repository still large after cleanup"
            show_largest_files
            echo ""
            log_error "Manual investigation required. Consider:"
            echo "  1. Removing large files from git history (git filter-repo)"
            echo "  2. Adding large file patterns to .gitignore"
            echo "  3. Using Git LFS for binary assets"
            exit 1
        fi
    else
        log_info "No recovery needed - repository is healthy"
        exit 0
    fi
}

main
