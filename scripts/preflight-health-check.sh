#!/usr/bin/env bash
# Pre-flight health check for agent tasks
# Prevents crashes from external service failures and resource exhaustion
#
# Usage: ./scripts/preflight-health-check.sh [options]
#   --warn-only      Exit 0 even if checks fail (for monitoring)
#   --quiet          Suppress informational messages
#   --verbose        Show detailed check output
#
# Exit codes:
#   0 - All checks passed
#   1 - One or more checks failed
#   2 - Invalid arguments

# Don't exit on errors - we want to run all checks
# Only exit on explicit errors or pipe failures
set +e

# Configuration
CHECK_LOG="/tmp/preflight-health-check.log"
INFERERENCE_GATEWAY_URL="${INFERENCE_GATEWAY_URL:-https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health}"
MIN_AVAILABLE_MEM_GB="${MIN_AVAILABLE_MEM_GB:-10}"
MIN_DISK_FREE_GB="${MIN_DISK_FREE_GB:-20}"
MAX_CPU_LOAD="${MAX_CPU_LOAD:-10}"
CURL_TIMEOUT="${CURL_TIMEOUT:-5}"

# Parse arguments
WARN_ONLY=false
QUIET=false
VERBOSE=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --warn-only)
      WARN_ONLY=true
      shift
      ;;
    --quiet)
      QUIET=true
      shift
      ;;
    --verbose)
      VERBOSE=true
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [options]"
      echo "Options:"
      echo "  --warn-only      Exit 0 even if checks fail (for monitoring)"
      echo "  --quiet          Suppress informational messages"
      echo "  --verbose        Show detailed check output"
      echo "  -h, --help       Show this help message"
      exit 0
      ;;
    *)
      echo "Error: Unknown argument: $1" >&2
      echo "Run '$0 --help' for usage" >&2
      exit 2
      ;;
  esac
done

# Logging functions
log_info() {
  if [[ "$QUIET" != true ]]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] INFO: $*" | tee -a "$CHECK_LOG"
  fi
}

log_warn() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] WARN: $*" | tee -a "$CHECK_LOG"
}

log_error() {
  echo "[$(date '+%Y-%m-%d %H:%M:%S')] ERROR: $*" | tee -a "$CHECK_LOG" >&2
}

log_verbose() {
  if [[ "$VERBOSE" == true ]]; then
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] VERBOSE: $*" | tee -a "$CHECK_LOG"
  fi
}

# Check result tracking
CHECKS_PASSED=0
CHECKS_FAILED=0
FAILED_CHECKS=()

# Check: Inference gateway availability
check_inference_gateway() {
  log_info "Checking inference gateway availability..."
  log_verbose "Gateway URL: $INFERERENCE_GATEWAY_URL"
  log_verbose "Timeout: ${CURL_TIMEOUT}s"

  if curl -sf --max-time "$CURL_TIMEOUT" "$INFERERENCE_GATEWAY_URL" > /dev/null 2>&1; then
    log_info "✓ Inference gateway is healthy"
    ((CHECKS_PASSED++))
    return 0
  else
    log_error "✗ Inference gateway unavailable"
    log_error "  URL: $INFERERENCE_GATEWAY_URL"
    log_error "  Task should be deferred until service is restored"
    FAILED_CHECKS+=("inference_gateway")
    ((CHECKS_FAILED++))
    return 1
  fi
}

# Check: Memory availability
check_memory() {
  log_info "Checking memory availability..."
  log_verbose "Minimum required: ${MIN_AVAILABLE_MEM_GB}GB"

  # Get available memory in GB (using /proc/meminfo on Linux)
  if [[ -f /proc/meminfo ]]; then
    available_mem_kb=$(grep '^MemAvailable:' /proc/meminfo | awk '{print $2}')
    if [[ -n "$available_mem_kb" ]]; then
      available_mem_gb=$((available_mem_kb / 1024 / 1024))
    else
      # Fallback to MemFree + Buffers + Cached
      mem_free=$(grep '^MemFree:' /proc/meminfo | awk '{print $2}')
      buffers=$(grep '^Buffers:' /proc/meminfo | awk '{print $2}')
      cached=$(grep '^Cached:' /proc/meminfo | awk '{print $2}' | head -1)
      available_mem_kb=$((mem_free + buffers + cached))
      available_mem_gb=$((available_mem_kb / 1024 / 1024))
    fi
  else
    # Fallback to free command
    available_mem_gb=$(free -g | awk '/^Mem:/{print $7}')
  fi

  log_verbose "Available memory: ${available_mem_gb}GB"

  if [[ "$available_mem_gb" -ge "$MIN_AVAILABLE_MEM_GB" ]]; then
    log_info "✓ Sufficient memory available (${available_mem_gb}GB)"
    ((CHECKS_PASSED++))
    return 0
  else
    log_error "✗ Insufficient memory (${available_mem_gb}GB available, ${MIN_AVAILABLE_MEM_GB}GB required)"
    FAILED_CHECKS+=("memory")
    ((CHECKS_FAILED++))
    return 1
  fi
}

# Check: Disk space
check_disk_space() {
  log_info "Checking disk space..."
  log_verbose "Minimum required: ${MIN_DISK_FREE_GB}GB"

  # Get available disk space in GB (root filesystem)
  disk_free_gb=$(df -BG / | awk 'NR==2 {print $4}' | tr -d 'G')
  disk_used_percent=$(df / | awk 'NR==2 {print $5}' | tr -d '%')

  log_verbose "Available disk space: ${disk_free_gb}GB"
  log_verbose "Disk usage: ${disk_used_percent}%"

  if [[ "$disk_free_gb" -ge "$MIN_DISK_FREE_GB" ]]; then
    log_info "✓ Sufficient disk space (${disk_free_gb}GB free, ${disk_used_percent}% used)"
    ((CHECKS_PASSED++))
    return 0
  else
    log_error "✗ Insufficient disk space (${disk_free_gb}GB free, ${MIN_DISK_FREE_GB}GB required)"
    FAILED_CHECKS+=("disk_space")
    ((CHECKS_FAILED++))
    return 1
  fi
}

# Check: CPU load
check_cpu_load() {
  log_info "Checking CPU load..."
  log_verbose "Maximum allowed 1min load average: ${MAX_CPU_LOAD}"

  # Get 1-minute load average
  load_1min=$(uptime | awk -F'load average:' '{print $2}' | awk '{print $1}' | tr -d ',')

  log_verbose "1-minute load average: ${load_1min}"

  # Use awk for floating point comparison
  if awk -v cpu_load="$load_1min" -v max_load="$MAX_CPU_LOAD" 'BEGIN { exit !(cpu_load < max_load) }'; then
    log_info "✓ CPU load acceptable (${load_1min} on 1min average)"
    ((CHECKS_PASSED++))
    return 0
  else
    log_error "✗ CPU load too high (${load_1min} on 1min average, max ${MAX_CPU_LOAD})"
    FAILED_CHECKS+=("cpu_load")
    ((CHECKS_FAILED++))
    return 1
  fi
}

# Check: Git repository health
check_git_health() {
  log_info "Checking git repository health..."

  # Only run if we're in a git repository
  if ! git rev-parse --git-dir > /dev/null 2>&1; then
    log_verbose "Not in a git repository - skipping git health check"
    return 0
  fi

  # Run git fsck to check repository integrity
  # Use --connectivity-only for a fast check, --no-progress to suppress progress
  git_fsck_output=$(git fsck --connectivity-only --no-progress 2>&1)
  echo "$git_fsck_output" | tee -a "$CHECK_LOG" >/dev/null

  # Check for actual problems: missing, corrupt, bad objects
  # "dangling" objects are normal and not an error
  if echo "$git_fsck_output" | grep -qE "(missing|corrupt|bad object|error|fatal)" && ! echo "$git_fsck_output" | grep -q "^dangling"; then
    log_error "✗ Git repository has issues (run 'git fsck --full' for details)"
    FAILED_CHECKS+=("git_health")
    ((CHECKS_FAILED++))
    return 1
  else
    log_info "✓ Git repository is healthy"
    ((CHECKS_PASSED++))
    return 0
  fi
}

# Check: Repository size and bloat prevention
check_repo_size() {
  log_info "Checking repository size..."

  # Only run if we're in a git repository
  if ! git rev-parse --git-dir > /dev/null 2>&1; then
    log_verbose "Not in a git repository - skipping repository size check"
    return 0
  fi

  # Thresholds (in MB)
  WARN_THRESHOLD_MB=2048    # 2 GB
  CRITICAL_THRESHOLD_MB=5120  # 5 GB
  AUTO_GC_THRESHOLD_MB=10240  # 10 GB - trigger automatic GC

  # Get repository size in MB
  repo_size_mb=$(du -s .git 2>/dev/null | awk '{print int($1/1024)}')
  repo_size_gb=$(echo "scale=1; $repo_size_mb/1024" | awk '{printf "%.1f", $1}')

  log_verbose "Repository .git size: ${repo_size_gb} GB (${repo_size_mb} MB)"

  # Get detailed git object statistics
  git_stats=$(git count-objects -vH 2>/dev/null || echo "")
  loose_objects=$(echo "$git_stats" | grep "^count:" | awk '{print $2}')
  loose_size_kb=$(echo "$git_stats" | grep "^size:" | awk '{print $2}')
  pack_size_mb=$(echo "$git_stats" | grep "^size-pack:" | awk '{print $2}' | sed 's/MiB//')

  log_verbose "Loose objects: $loose_objects"
  log_verbose "Loose size: $loose_size_kb"
  log_verbose "Pack size: ${pack_size_mb}MiB"

  # Check thresholds and take action
  if [ "$repo_size_mb" -ge "$AUTO_GC_THRESHOLD_MB" ]; then
    log_error "✗ Repository size critical: ${repo_size_gb} GB (threshold: $(($AUTO_GC_THRESHOLD_MB/1024)) GB)"
    log_error "  Automatic garbage collection required"
    log_error "  Loose objects: $loose_objects"
    log_error "  Pack size: ${pack_size_mb}MiB"
    log_error ""
    log_error "RECOMMENDED ACTION:"
    log_error "  Run: ./scripts/safe-git-gc.sh"
    log_error "  Or: git gc --aggressive --prune=now"

    # Check if safe-git-gc.sh exists and is executable
    script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
    if [ -x "$script_dir/safe-git-gc.sh" ]; then
      log_error ""
      log_error "Safe GC script available at: $script_dir/safe-git-gc.sh"
    fi

    FAILED_CHECKS+=("repo_size_critical")
    ((CHECKS_FAILED++))
    return 1

  elif [ "$repo_size_mb" -ge "$CRITICAL_THRESHOLD_MB" ]; then
    log_error "✗ Repository size exceeds critical threshold: ${repo_size_gb} GB (threshold: $(($CRITICAL_THRESHOLD_MB/1024)) GB)"
    log_error "  Garbage collection strongly recommended"
    log_error "  Loose objects: $loose_objects"
    log_error "  Pack size: ${pack_size_mb}MiB"

    FAILED_CHECKS+=("repo_size_critical")
    ((CHECKS_FAILED++))
    return 1

  elif [ "$repo_size_mb" -ge "$WARN_THRESHOLD_MB" ]; then
    log_warn "⚠ Repository size large: ${repo_size_gb} GB (warning threshold: $(($WARN_THRESHOLD_MB/1024)) GB)"
    log_warn "  Consider running: git gc"
    log_warn "  Loose objects: $loose_objects"

    # Warning doesn't fail the check, but does track it
    log_info "✓ Repository size acceptable but monitor growth"
    ((CHECKS_PASSED++))
    return 0

  else
    log_info "✓ Repository size healthy: ${repo_size_gb} GB"
    ((CHECKS_PASSED++))
    return 0
  fi
}

# Main health check execution
main() {
  log_info "=== Pre-flight Health Check Started ==="
  log_verbose "Configuration:"
  log_verbose "  Inference Gateway: $INFERERENCE_GATEWAY_URL"
  log_verbose "  Min Memory: ${MIN_AVAILABLE_MEM_GB}GB"
  log_verbose "  Min Disk: ${MIN_DISK_FREE_GB}GB"
  log_verbose "  Max CPU Load: $MAX_CPU_LOAD"

  # Run all checks
  check_inference_gateway
  check_memory
  check_disk_space
  check_cpu_load
  check_git_health
  check_repo_size

  # Summary
  total_checks=$((CHECKS_PASSED + CHECKS_FAILED))
  log_info "=== Health Check Summary ==="
  log_info "Total checks: $total_checks"
  log_info "Passed: $CHECKS_PASSED"
  log_info "Failed: $CHECKS_FAILED"

  if [[ $CHECKS_FAILED -gt 0 ]]; then
    log_error "Failed checks: ${FAILED_CHECKS[*]}"
    log_error ""
    log_error "RECOMMENDED ACTION:"
    log_error "  - Do not start agent tasks until all checks pass"
    log_error "  - Address the issues above before proceeding"
    log_error "  - Run '$0 --verbose' for detailed diagnostics"

    if [[ "$WARN_ONLY" == true ]]; then
      log_warn "WARN-ONLY MODE: Exiting with success despite failures"
      exit 0
    else
      exit 1
    fi
  else
    log_info "✓ All health checks passed - safe to proceed with tasks"
    exit 0
  fi
}

# Run main function
main
