#!/usr/bin/env bash
# Test repository monitoring and bloat prevention measures
#
# Usage: ./scripts/test-repo-monitoring.sh [options]
#   --verbose        Show detailed test output
#   --integration    Run integration tests (requires actual repo operations)
#   --unit           Run unit tests only (default)
#
# Exit codes:
#   0 - All tests passed
#   1 - One or more tests failed

# Don't exit on errors - we want to run all tests
set +e
set -o pipefail

# Configuration
VERBOSE=false
INTEGRATION=false
TESTS_PASSED=0
TESTS_FAILED=0
TESTS_SKIPPED=0

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --verbose)
      VERBOSE=true
      shift
      ;;
    --integration)
      INTEGRATION=true
      shift
      ;;
    -h|--help)
      echo "Usage: $0 [options]"
      echo "Options:"
      echo "  --verbose        Show detailed test output"
      echo "  --integration    Run integration tests (requires actual repo operations)"
      echo "  --unit           Run unit tests only (default)"
      echo "  -h, --help       Show this help message"
      exit 0
      ;;
    *)
      echo "Error: Unknown argument: $1" >&2
      exit 1
      ;;
  esac
done

# Logging functions
log_test() {
  echo -e "${BLUE}[TEST]${NC} $*"
}

log_pass() {
  echo -e "${GREEN}[PASS]${NC} $*"
  ((TESTS_PASSED++))
}

log_fail() {
  echo -e "${RED}[FAIL]${NC} $*"
  ((TESTS_FAILED++))
}

log_skip() {
  echo -e "${YELLOW}[SKIP]${NC} $*"
  ((TESTS_SKIPPED++))
}

log_verbose() {
  if [[ "$VERBOSE" == true ]]; then
    echo -e "  [VERBOSE] $*"
  fi
}

# Test helper functions
assert_equals() {
  local expected="$1"
  local actual="$2"
  local message="${3:-Values should be equal}"

  if [[ "$expected" == "$actual" ]]; then
    log_pass "$message"
    return 0
  else
    log_fail "$message (expected: $expected, actual: $actual)"
    return 1
  fi
}

assert_true() {
  local condition="$1"
  local message="${2:-Condition should be true}"

  if [[ "$condition" == "true" ]] || [[ "$condition" -eq 1 ]] || [[ "$condition" == "0" ]]; then
    log_pass "$message"
    return 0
  else
    log_fail "$message (condition: $condition)"
    return 1
  fi
}

assert_greater_than() {
  local threshold="$1"
  local value="$2"
  local message="${3:-Value should be greater than threshold}"

  if [[ "$value" -gt "$threshold" ]]; then
    log_pass "$message"
    return 0
  else
    log_fail "$message (value: $value, threshold: $threshold)"
    return 1
  fi
}

# Test: Repository size monitoring script exists and is executable
test_repo_size_monitoring_exists() {
  log_test "Repository size monitoring script exists and is executable"

  local script="./scripts/check-repo-size.sh"
  if [[ ! -f "$script" ]]; then
    log_fail "Script not found: $script"
    return 1
  fi

  if [[ ! -x "$script" ]]; then
    log_fail "Script not executable: $script"
    return 1
  fi

  log_pass "Repository size monitoring script exists and is executable"
  return 0
}

# Test: Preflight health check includes repository size checking
test_preflight_includes_repo_size() {
  log_test "Preflight health check includes repository size checking"

  local preflight_script="./scripts/preflight-health-check.sh"
  if [[ ! -f "$preflight_script" ]]; then
    log_fail "Preflight script not found: $preflight_script"
    return 1
  fi

  # Check if the script contains the check_repo_size function
  if grep -q "check_repo_size()" "$preflight_script" 2>/dev/null; then
    log_pass "Preflight script includes repository size checking"
    return 0
  else
    log_fail "Preflight script does not include repository size checking"
    return 1
  fi
}

# Test: Repository size thresholds are defined correctly
test_repo_size_thresholds() {
  log_test "Repository size thresholds are defined correctly"

  local preflight_script="./scripts/preflight-health-check.sh"

  # Extract thresholds from script
  local warn_threshold=$(grep "WARN_THRESHOLD_MB=" "$preflight_script" 2>/dev/null | head -1 | sed 's/.*WARN_THRESHOLD_MB=\([0-9]*\).*/\1/' || echo "")
  local critical_threshold=$(grep "CRITICAL_THRESHOLD_MB=" "$preflight_script" 2>/dev/null | head -1 | sed 's/.*CRITICAL_THRESHOLD_MB=\([0-9]*\).*/\1/' || echo "")
  local auto_gc_threshold=$(grep "AUTO_GC_THRESHOLD_MB=" "$preflight_script" 2>/dev/null | head -1 | sed 's/.*AUTO_GC_THRESHOLD_MB=\([0-9]*\).*/\1/' || echo "")

  if [[ -z "$warn_threshold" ]] || [[ -z "$critical_threshold" ]] || [[ -z "$auto_gc_threshold" ]]; then
    log_fail "Could not extract thresholds from preflight script"
    return 1
  fi

  # Check threshold hierarchy: warn < critical < auto_gc
  if [[ "$warn_threshold" -lt "$critical_threshold" ]] && [[ "$critical_threshold" -lt "$auto_gc_threshold" ]]; then
    log_pass "Repository size thresholds are correctly hierarchical (warn: ${warn_threshold}MB, critical: ${critical_threshold}MB, auto-gc: ${auto_gc_threshold}MB)"
    return 0
  else
    log_fail "Repository size thresholds are not hierarchical (warn: ${warn_threshold}MB, critical: ${critical_threshold}MB, auto-gc: ${auto_gc_threshold}MB)"
    return 1
  fi
}

# Test: Auto GC trigger script exists and is executable
test_auto_gc_trigger_exists() {
  log_test "Auto GC trigger script exists and is executable"

  local script="./scripts/auto-gc-trigger.sh"
  if [[ ! -f "$script" ]]; then
    log_fail "Script not found: $script"
    return 1
  fi

  if [[ ! -x "$script" ]]; then
    log_fail "Script not executable: $script"
    return 1
  fi

  log_pass "Auto GC trigger script exists and is executable"
  return 0
}

# Test: Safe git GC script exists and is executable
test_safe_git_gc_exists() {
  log_test "Safe git GC script exists and is executable"

  local script="./scripts/safe-git-gc.sh"
  if [[ ! -f "$script" ]]; then
    log_fail "Script not found: $script"
    return 1
  fi

  if [[ ! -x "$script" ]]; then
    log_fail "Script not executable: $script"
    return 1
  fi

  log_pass "Safe git GC script exists and is executable"
  return 0
}

# Test: Repository health check script exists and is executable
test_repo_health_check_exists() {
  log_test "Repository health check script exists and is executable"

  local script="./scripts/check-repo-health.sh"
  if [[ ! -f "$script" ]]; then
    log_fail "Script not found: $script"
    return 1
  fi

  if [[ ! -x "$script" ]]; then
    log_fail "Script not executable: $script"
    return 1
  fi

  log_pass "Repository health check script exists and is executable"
  return 0
}

# Test: Monitoring setup script exists and is executable
test_monitoring_setup_exists() {
  log_test "Monitoring setup script exists and is executable"

  local script="./scripts/monitoring-setup.sh"
  if [[ ! -f "$script" ]]; then
    log_fail "Script not found: $script"
    return 1
  fi

  if [[ ! -x "$script" ]]; then
    log_fail "Script not executable: $script"
    return 1
  fi

  log_pass "Monitoring setup script exists and is executable"
  return 0
}

# Test: Best practices documentation exists
test_best_practices_doc_exists() {
  log_test "Best practices documentation exists"

  local doc="./docs/repository-maintenance-best-practices.md"
  if [[ ! -f "$doc" ]]; then
    log_fail "Documentation not found: $doc"
    return 1
  fi

  # Check that documentation contains key sections
  local required_sections=(
    "Repository Size Thresholds"
    "Monitoring and Health Checks"
    "Garbage Collection"
    "Best Practices"
  )

  local missing_sections=()
  for section in "${required_sections[@]}"; do
    if ! grep -q "$section" "$doc" 2>/dev/null; then
      missing_sections+=("$section")
    fi
  done

  if [[ ${#missing_sections[@]} -gt 0 ]]; then
    log_fail "Documentation missing required sections: ${missing_sections[*]}"
    return 1
  fi

  log_pass "Best practices documentation exists with all required sections"
  return 0
}

# Test: Repository size monitoring detects large repositories
test_repo_size_detection() {
  log_test "Repository size monitoring detects large repositories"

  if [[ "$INTEGRATION" != true ]]; then
    log_skip "Integration test (use --integration to run)"
    return 0
  fi

  # Get actual repository size
  local repo_size_mb=$(du -s .git 2>/dev/null | awk '{print int($1/1024)}')

  if [[ -z "$repo_size_mb" ]]; then
    log_fail "Could not determine repository size"
    return 1
  fi

  log_verbose "Current repository size: ${repo_size_mb} MB"

  # Check the size is within reasonable bounds
  # For a healthy Go repo, should be less than 10 GB (10240 MB)
  if [[ "$repo_size_mb" -lt 10240 ]]; then
    log_pass "Repository size is within reasonable bounds (${repo_size_mb} MB < 10240 MB)"
    return 0
  else
    log_fail "Repository size exceeds reasonable bounds (${repo_size_mb} MB >= 10240 MB)"
    return 1
  fi
}

# Test: Preflight health check runs successfully
test_preflight_runs_successfully() {
  log_test "Preflight health check runs successfully"

  if [[ "$INTEGRATION" != true ]]; then
    log_skip "Integration test (use --integration to run)"
    return 0
  fi

  if ./scripts/preflight-health-check.sh --warn-only > /tmp/preflight-test.log 2>&1; then
    log_pass "Preflight health check completed successfully"
    return 0
  else
    log_fail "Preflight health check failed (check /tmp/preflight-test.log for details)"
    return 1
  fi
}

# Test: Auto GC trigger dry-run works
test_auto_gc_trigger_dry_run() {
  log_test "Auto GC trigger dry-run works"

  if [[ "$INTEGRATION" != true ]]; then
    log_skip "Integration test (use --integration to run)"
    return 0
  fi

  if ./scripts/auto-gc-trigger.sh --dry-run > /tmp/auto-gc-test.log 2>&1; then
    log_pass "Auto GC trigger dry-run completed successfully"
    return 0
  else
    log_fail "Auto GC trigger dry-run failed (check /tmp/auto-gc-test.log for details)"
    return 1
  fi
}

# Test: Repository health check runs successfully
test_repo_health_check_runs() {
  log_test "Repository health check runs successfully"

  if [[ "$INTEGRATION" != true ]]; then
    log_skip "Integration test (use --integration to run)"
    return 0
  fi

  if ./scripts/check-repo-health.sh > /tmp/repo-health-test.log 2>&1; then
    log_pass "Repository health check completed successfully"
    return 0
  else
    log_fail "Repository health check failed (check /tmp/repo-health-test.log for details)"
    return 1
  fi
}

# Test: Check that current repository is healthy
test_current_repo_health() {
  log_test "Current repository is healthy"

  if [[ "$INTEGRATION" != true ]]; then
    log_skip "Integration test (use --integration to run)"
    return 0
  fi

  # Get git object statistics
  local git_stats=$(git count-objects -vH 2>/dev/null || echo "")
  local loose_objects=$(echo "$git_stats" | grep "^count:" | awk '{print $2}')
  local garbage=$(echo "$git_stats" | grep "^size-garbage:" | awk '{print $2}')

  log_verbose "Loose objects: $loose_objects"
  log_verbose "Garbage: $garbage"

  # For a healthy repo, loose objects should be reasonable (< 1000)
  # and garbage should be 0
  if [[ "$loose_objects" -lt 1000 ]] && [[ "$garbage" == "0" ]]; then
    log_pass "Repository is healthy (loose objects: $loose_objects, garbage: $garbage)"
    return 0
  else
    log_fail "Repository may need maintenance (loose objects: $loose_objects, garbage: $garbage)"
    return 1
  fi
}

# Test: Check git fsck passes
test_git_fsck() {
  log_test "Git fsck passes"

  if [[ "$INTEGRATION" != true ]]; then
    log_skip "Integration test (use --integration to run)"
    return 0
  fi

  if git fsck --connectivity-only --no-progress > /tmp/git-fsck-test.log 2>&1; then
    log_pass "Git fsck passed - repository integrity verified"
    return 0
  else
    log_fail "Git fsck failed (check /tmp/git-fsck-test.log for details)"
    return 1
  fi
}

# Main test runner
main() {
  echo "=== Repository Monitoring and Bloat Prevention Test Suite ==="
  echo ""

  if [[ "$INTEGRATION" == true ]]; then
    echo "Mode: Integration tests (will run actual repo operations)"
  else
    echo "Mode: Unit tests only (use --integration for full testing)"
  fi
  echo ""

  # Unit tests (always run)
  test_repo_size_monitoring_exists
  test_preflight_includes_repo_size
  test_repo_size_thresholds
  test_auto_gc_trigger_exists
  test_safe_git_gc_exists
  test_repo_health_check_exists
  test_monitoring_setup_exists
  test_best_practices_doc_exists

  # Integration tests (only if --integration flag is set)
  if [[ "$INTEGRATION" == true ]]; then
    echo ""
    echo "=== Running Integration Tests ==="
    echo ""

    test_repo_size_detection
    test_preflight_runs_successfully
    test_auto_gc_trigger_dry_run
    test_repo_health_check_runs
    test_current_repo_health
    test_git_fsck
  fi

  # Summary
  local total_tests=$((TESTS_PASSED + TESTS_FAILED + TESTS_SKIPPED))
  echo ""
  echo "=== Test Summary ==="
  echo "Total tests:  $total_tests"
  echo -e "${GREEN}Passed:${NC}       $TESTS_PASSED"
  echo -e "${RED}Failed:${NC}       $TESTS_FAILED"
  echo -e "${YELLOW}Skipped:${NC}      $TESTS_SKIPPED"

  if [[ $TESTS_FAILED -gt 0 ]]; then
    echo ""
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
  elif [[ $TESTS_PASSED -eq 0 ]] && [[ $TESTS_SKIPPED -gt 0 ]]; then
    echo ""
    echo -e "${YELLOW}⚠️  All tests were skipped (run with --integration for full testing)${NC}"
    exit 0
  else
    echo ""
    echo -e "${GREEN}✅ All tests passed${NC}"
    exit 0
  fi
}

# Run main
main
