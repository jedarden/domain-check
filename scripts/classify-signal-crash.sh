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

    # Check 3: CPU load (NEW - for CPU saturation detection)
    log_section "CPU Load Check"
    CPU_CORES=$(nproc)
    LOAD_1MIN=$(awk '{print $1}' /proc/loadavg)
    LOAD_5MIN=$(awk '{print $2}' /proc/loadavg)
    LOAD_15MIN=$(awk '{print $3}' /proc/loadavg)

    LOAD_PERCENT_1=$(awk "BEGIN {printf \"%.1f\", ($LOAD_1MIN/$CPU_CORES)*100}")
    LOAD_PERCENT_5=$(awk "BEGIN {printf \"%.1f\", ($LOAD_5MIN/$CPU_CORES)*100}")
    LOAD_PERCENT_15=$(awk "BEGIN {printf \"%.1f\", ($LOAD_15MIN/$CPU_CORES)*100}")

    echo "CPU Cores: $CPU_CORES"
    echo "Load Average (1min): $LOAD_1MIN (${LOAD_PERCENT_1}%)"
    echo "Load Average (5min): $LOAD_5MIN (${LOAD_PERCENT_5}%)"
    echo "Load Average (15min): $LOAD_15MIN (${LOAD_PERCENT_15}%)"
    echo ""

    # Classification logic
    log_section "Diagnostic Assessment"

    BLOAT_DETECTED=0
    CPU_SATURATED=0
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

    # Check for CPU saturation (load average > 80% of cores)
    LOAD_HIGH=$(awk "BEGIN {print ($LOAD_1MIN > ($CPU_CORES * 0.8)) ? \"1\" : \"0\"}")
    if [ "$LOAD_HIGH" -eq 1 ]; then
        log_warn "High CPU load detected (${LOAD_PERCENT_1}% utilization > 80% threshold)"
        CPU_SATURATED=1
    fi

    echo ""
    log_section "Classification Result"

    # Priority 1: Check for repository bloat/OOM (highest priority)
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

    # Priority 2: Check for CPU saturation (transient resource issue)
    elif [ "$CPU_SATURATED" -eq 1 ]; then
        CLASSIFICATION="CPU_SATURATION"
        log_warn "CLASSIFICATION: LIKELY CPU SATURATION CRASH (SIGKILL/SIGTERM due to resource pressure)"
        echo ""
        echo "Root Cause: High CPU load (>${LOAD_PERCENT_1}%) causing system resource management intervention"
        echo ""
        echo "Characteristics:"
        echo "  - Repository is healthy (${REPO_SIZE_MB}MB, ${LOOSE_OBJECTS} loose objects)"
        echo "  - Memory is available (${MEM_PERCENT}%)"
        echo "  - CPU is saturated (load average ${LOAD_1MIN} on ${CPU_CORES} cores)"
        echo ""
        echo "Recommended Actions:"
        echo "  1. NO CODE REMEDIATION NEEDED - This is a transient resource event"
        echo "  2. Document as CPU saturation crash (similar to SIGHUP cascade)"
        echo "  3. Verify automatic retry will succeed when CPU pressure decreases"
        echo "  4. Check for fleet-wide crashes in same time window (system-wide load event)"
        echo ""
        echo "Additional Verification Steps:"
        echo "  - Review system load history: uptime"
        echo "  - Check for concurrent heavy processes: top -b -n 1 | head -20"
        echo "  - Document in bead notes as transient CPU saturation event"
        echo ""
        exit 0  # Exit 0 because no remediation needed (similar to SIGHUP)

    # Priority 3: Default to SIGHUP cascade (external event)
    else
        CLASSIFICATION="SIGHUP_CASCADE"
        log_info "Repository is healthy (${REPO_SIZE_MB}MB, ${LOOSE_OBJECTS} loose objects)"
        log_info "Memory is available (${MEM_PERCENT}%)"
        log_info "CPU load is normal (${LOAD_PERCENT_1}%)"
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
