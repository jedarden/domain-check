#!/usr/bin/env bash
# Pre-flight Health Check
# Run before starting agent tasks to ensure service availability and repository health
# Returns exit code 0 if healthy, 1 if unhealthy, or 75 when deferred because
# a system event is active (scripts/system-event-mode.sh). That deferral is a
# load-shedding directive, not a failed check, so it is not relaxable by
# --warn-only.
#
# Implements Proposal 3.4 from crash mitigation strategies:
# - Pre-task repository health check to prevent OOM crashes from repository bloat
# - Checks service availability (inference gateway)
# - Checks system resources (memory, disk, CPU)
# - Checks repository health (size, loose objects)
#
# Usage: scripts/preflight-health-check.sh [--verbose] [--warn-only]

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(git rev-parse --show-toplevel)"
SERVICE_MONITOR="$SCRIPT_DIR/service-monitor.sh"
REPO_HEALTH_CHECK="$SCRIPT_DIR/check-repo-health.sh"
SYSTEM_EVENT_GATE="$SCRIPT_DIR/system-event-mode.sh"

# Parse arguments
VERBOSE=false
WARN_ONLY=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --verbose|-v)
      VERBOSE=true
      shift
      ;;
    --warn-only)
      WARN_ONLY=true
      shift
      ;;
    --help|-h)
      echo "Usage: $0 [--verbose] [--warn-only]"
      echo ""
      echo "Options:"
      echo "  --verbose, -v    Show detailed check output"
      echo "  --warn-only      Warn on failures but don't exit with error"
      echo "  --help, -h       Show this help message"
      exit 0
      ;;
    *)
      echo "Unknown option: $1"
      exit 2
      ;;
  esac
done

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

CHECKS_PASSED=0
CHECKS_FAILED=0

echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Pre-flight Health Check${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""

# Function to run a check
run_check() {
  local check_name="$1"
  local check_command="$2"

  echo -e "${BLUE}[$((CHECKS_PASSED + CHECKS_FAILED + 1))]${NC} $check_name"

  if [[ "$VERBOSE" == true ]]; then
    echo -e "   Running: $check_command"
  fi

  if eval "$check_command" > /dev/null 2>&1; then
    echo -e "   ${GREEN}✓${NC} $check_name passed"
    ((CHECKS_PASSED+=1))
    return 0
  else
    echo -e "   ${RED}✗${NC} $check_name failed"
    ((CHECKS_FAILED+=1))
    return 1
  fi
}

# Check 0: System event gate — DEFER new work while a crash/memory surge is
# active (bf-3561g RCA §6 recommendation 5 / gap G-3). Runs first and exits
# immediately on a latched event, so neither the rest of the preflight nor the
# task it gates adds load to a box that is already shedding it. 75 = EX_TEMPFAIL.
echo -e "${BLUE}System Event${NC}"
if [[ -f "$SYSTEM_EVENT_GATE" ]]; then
  SE_RC=0
  SE_OUT=$(bash "$SYSTEM_EVENT_GATE" check 2>&1) || SE_RC=$?
  if [[ $SE_RC -eq 75 ]]; then
    echo -e "   ${YELLOW}⏸${NC} deferred: system event active — do not start new work"
    if [[ -n "$SE_OUT" ]]; then
      echo "$SE_OUT" | sed 's/^/   /'
    fi
    echo ""
    echo "Deferred: system event active. Retry when the event clears (scripts/system-event-mode.sh status)."
    exit 75
  elif [[ $SE_RC -eq 0 ]]; then
    echo -e "   ${GREEN}✓${NC} No system event active"
    ((CHECKS_PASSED+=1))
  else
    # Fail open: an unreadable gate must not block the preflight itself.
    echo -e "   ${YELLOW}⚠${NC} System event gate unreadable (exit $SE_RC) — skipping check"
    echo "$SE_OUT" | sed 's/^/   /'
    ((CHECKS_PASSED+=1))
  fi
else
  echo -e "${YELLOW}⚠${NC} system-event-mode.sh not found (skipping)"
fi
echo ""

# Check 1: Service availability (inference gateway)
echo -e "${BLUE}Service Availability${NC}"
if bash "$SERVICE_MONITOR" --once > /tmp/service-monitor.$$ 2>&1; then
  echo -e "   ${GREEN}✓${NC} Inference gateway available"
  ((CHECKS_PASSED+=1))
else
  echo -e "   ${RED}✗${NC} Inference gateway unavailable"
  if [[ "$VERBOSE" == true ]]; then
    cat /tmp/service-monitor.$$
  fi
  ((CHECKS_FAILED+=1))
fi
rm -f /tmp/service-monitor.$$
echo ""

# Check 2: Repository health
echo -e "${BLUE}Repository Health${NC}"
if [[ -f "$REPO_HEALTH_CHECK" ]]; then
  # Check if repository size is acceptable.
  # Compare in integer KB (1GB = 1048576KB): bash has no floating point and
  # `bc` is not installed on this box, so the previous `bc -l` comparison
  # always failed open (127 -> "0") and passed repos of any size — a 1.2GB
  # synthetic repo reported "passed" before this was fixed 2026-09-06.
  REPO_SIZE_KB=$(du -s .git 2>/dev/null | awk '{print $1}')
  REPO_SIZE=$(awk -v kb="${REPO_SIZE_KB:-0}" 'BEGIN{printf "%.3f", kb/1048576}')

  if [[ "${REPO_SIZE_KB:-0}" -gt 1048576 ]]; then
    echo -e "   ${RED}✗${NC} Repository size (${REPO_SIZE}GB) exceeds threshold (1GB)"
    echo -e "   ${YELLOW}Recommended actions:${NC}"
    echo "     1. Run repository cleanup: ./scripts/safe-git-gc.sh --full"
    echo "     2. Investigate large files: git ls-files | xargs du -h | sort -rh | head -20"
    ((CHECKS_FAILED+=1))
  else
    echo -e "   ${GREEN}✓${NC} Repository health check passed (${REPO_SIZE}GB)"
    ((CHECKS_PASSED+=1))

    if [[ "$VERBOSE" == true ]]; then
      # `|| true`: head -20 closes the pipe once it has its lines, and under
      # `set -o pipefail` the SIGPIPE'd (141) pipeline would abort this script
      # mid-run whenever the health output exceeds 20 lines — it always did.
      bash "$REPO_HEALTH_CHECK" 2>&1 | head -20 || true
    fi
  fi
else
  echo -e "${YELLOW}⚠${NC} Repository health check script not found (skipping)"
fi
echo ""

# Check 3: Dispatch scope cgroup memory headroom
# System-wide free memory is not the kill domain — the kernel OOM-kills the
# tightest bounded cgroup (this scope's 12G MemoryMax, or the needle.slice
# ancestor it shares with every other worker). See
# docs/research/root-cause-analysis-signal-minus-one-crashes.md.
echo -e "${BLUE}Dispatch Scope Memory Headroom${NC}"
MEMORY_GUARD="$SCRIPT_DIR/cgroup-memory-guard.sh"
if [[ -f "$MEMORY_GUARD" ]]; then
  GUARD_RC=0
  GUARD_OUT=$(bash "$MEMORY_GUARD" --check 2>&1) || GUARD_RC=$?
  if [[ $GUARD_RC -eq 0 ]]; then
    echo -e "   ${GREEN}✓${NC} Cgroup headroom healthy"
    ((CHECKS_PASSED+=1))
    if [[ "$VERBOSE" == true ]]; then
      echo "$GUARD_OUT" | sed 's/^/   /'
    fi
  elif [[ $GUARD_RC -eq 1 ]]; then
    echo -e "   ${YELLOW}⚠${NC} Cgroup headroom degraded — proceed with caution, avoid memory-heavy work"
    echo "$GUARD_OUT" | sed 's/^/   /'
    ((CHECKS_PASSED+=1))
  elif [[ $GUARD_RC -eq 2 ]]; then
    echo -e "   ${RED}✗${NC} Insufficient cgroup headroom — memory-heavy work would risk a memcg OOM kill"
    echo "$GUARD_OUT" | sed 's/^/   /'
    echo -e "   ${YELLOW}Recommended actions:${NC}"
    echo "     1. Wait for concurrent workers to drain (needle.slice shared headroom)"
    echo "     2. Defer git gc / bulk operations: ./scripts/safe-git-gc.sh --check-only"
    ((CHECKS_FAILED+=1))
  else
    # Unknown cgroup state — fail-open so the pre-flight check itself never
    # blocks a task on an unreadable cgroupfs.
    echo -e "   ${YELLOW}⚠${NC} Cgroup state unreadable (skipping scope headroom check)"
    ((CHECKS_PASSED+=1))
  fi
else
  echo -e "${YELLOW}⚠${NC} cgroup-memory-guard.sh not found (skipping)"
fi
echo ""

# Summary
echo -e "${BLUE}========================================${NC}"
echo -e "${BLUE}Summary${NC}"
echo -e "${BLUE}========================================${NC}"
echo ""
echo -e "Checks passed: ${GREEN}$CHECKS_PASSED${NC}"
echo -e "Checks failed: ${RED}$CHECKS_FAILED${NC}"
echo ""

if [[ $CHECKS_FAILED -gt 0 ]]; then
  if [[ "$WARN_ONLY" == true ]]; then
    echo -e "${YELLOW}⚠ Pre-flight check completed with warnings (warn-only mode)${NC}"
    exit 0
  else
    echo -e "${RED}✗ Pre-flight check failed${NC}"
    echo ""
    echo "System is not healthy. Task deferred until issues are resolved."
    exit 1
  fi
else
  echo -e "${GREEN}✓ All pre-flight checks passed${NC}"
  echo ""
  echo "System is healthy. Task can proceed."
  exit 0
fi
