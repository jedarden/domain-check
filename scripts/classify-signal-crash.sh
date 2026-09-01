#!/bin/bash
# Signal -1 Crash Classification Script
# Distinguishes OOM SIGKILL events from SIGHUP cascade events
# This is Layer 0 of the crash remediation strategy

set -euo pipefail

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
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

log_section() {
    echo -e "${BLUE}=== $1 ===${NC}"
}

# Check if we're in a git repository
if ! git rev-parse --git-dir > /dev/null 2>&1; then
    log_error "Not in a git repository"
    exit 1
fi

# Main classification logic
main() {
    log_section "Signal -1 Crash Classification"
    echo "Timestamp: $(date -Iseconds)"
    echo "Workspace: $(pwd)"
    echo ""

    # Check 1: Repository health
    log_section "Repository Health Check"
    REPO_SIZE_KB=$(du -sk .git 2>/dev/null | awk '{print $1}')
    REPO_SIZE_MB=$((REPO_SIZE_KB / 1024))

    GIT_STATS=$(git count-objects -vH 2>/dev/null || true)
    LOOSE_OBJECTS=$(echo "$GIT_STATS" | grep '^count:' | awk '{print $2}' || echo 0)
    PACK_COUNT=$(echo "$GIT_STATS" | grep '^packs:' | awk '{print $2}' || echo 0)
    IN_PACK=$(echo "$GIT_STATS" | grep '^in-pack:' | awk '{print $2}' || echo 0)

    echo "Repository Size: ${REPO_SIZE_MB}MB"
    echo "Loose Objects: $LOOSE_OBJECTS"
    echo "Pack Files: $PACK_COUNT"
    echo ""

    # Check 2: System memory
    log_section "System Memory Check"
    MEM_AVAIL_MB=$(free -m | awk '/^Mem:/ {print $7}')
    MEM_TOTAL_MB=$(free -m | awk '/^Mem:/ {print $2}')
    MEM_PERCENT=$(awk "BEGIN {printf \"%.1f\", ($MEM_AVAIL_MB/$MEM_TOTAL_MB)*100}")

    echo "Available Memory: ${MEM_AVAIL_MB}MB (${MEM_PERCENT}%)"
    echo "Total Memory: ${MEM_TOTAL_MB}MB"
    echo ""

    # Classification logic
    log_section "Diagnostic Assessment"

    BLOAT_DETECTED=0
    OOM_LIKELIHOOD="LOW"
    CLASSIFICATION=""

    # Check for repository bloat
    if [ "$REPO_SIZE_MB" -gt 500 ]; then
        log_warn "Repository bloat detected (${REPO_SIZE_MB}MB > 500MB threshold)"
        BLOAT_DETECTED=1
        OOM_LIKELIHOOD="HIGH"
    fi

    if [ "$LOOSE_OBJECTS" -gt 1000 ]; then
        log_warn "Excessive loose objects (${LOOSE_OBJECTS} > 1000 threshold)"
        BLOAT_DETECTED=1
        OOM_LIKELIHOOD="HIGH"
    fi

    # Check for memory pressure (using awk for floating-point comparison)
    MEM_LOW=$(awk "BEGIN {print ($MEM_PERCENT < 10) ? \"1\" : \"0\"}")
    if [ "$MEM_LOW" -eq 1 ]; then
        log_warn "Low memory available (${MEM_PERCENT}% < 10%)"
        OOM_LIKELIHOOD="HIGH"
    fi

    echo ""
    log_section "Classification Result"

    if [ "$BLOAT_DETECTED" -eq 1 ] || [ "$OOM_LIKELIHOOD" = "HIGH" ]; then
        CLASSIFICATION="OOM_SIGKILL"
        log_error "CLASSIFICATION: LIKELY OOM SIGKILL (Signal 9)"
        echo ""
        echo "Root Cause: Repository bloat or memory exhaustion → OOM killer intervention"
        echo ""
        echo "Recommended Actions:"
        echo "  1. Run repository recovery:"
        echo "     ./scripts/recover-repo-bloat.sh"
        echo ""
        echo "  2. If memory is low, close other processes or increase system memory"
        echo ""
        echo "  3. Check for large files in git history:"
        echo "     git rev-list --objects --all | git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)' | awk '/^blob/ {print substr(\$0,6)}' | sort -nk2 | tail -10"
        echo ""
        exit 1
    else
        CLASSIFICATION="SIGHUP_CASCADE"
        log_info "Repository is healthy (${REPO_SIZE_MB}MB, ${LOOSE_OBJECTS} loose objects)"
        log_info "CLASSIFICATION: LIKELY SIGHUP CASCADE (Signal 1)"
        echo ""
        echo "Root Cause: External system process (systemd/fleet manager) termination"
        echo ""
        echo "Recommended Actions:"
        echo "  1. Document this as a fleet-wide event (no repository action needed)"
        echo "  2. Verify temporal clustering (check for other crashes in same time window)"
        echo "  3. Check other workers for simultaneous crashes"
        echo "  4. Add to crash documentation as external event pattern"
        echo ""
        echo "Additional Verification Steps:"
        echo "  - Review recent system logs: journalctl -xe"
        echo "  - Check for systemd service restarts: systemctl status"
        echo "  - Document in bead notes as fleet-wide SIGHUP event"
        echo ""
        exit 0
    fi
}

# Run main function
main "$@"
