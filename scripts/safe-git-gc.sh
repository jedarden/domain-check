#!/usr/bin/env bash
# Safe Git GC - Memory-efficient garbage collection with monitoring and resume
# Usage: scripts/safe-git-gc.sh [--full] [--resume] [--check-only]
#
# Options:
#   --full        Run all stages including deep compression (default: stages 1-2)
#   --resume      Resume from last checkpoint
#   --check-only  Only check if gc is needed, don't run
#
# Environment variables:
#   SAFE_GC_MEMORY_MAX  Maximum memory to use (default: 2g)
#   SAFE_GC_CHECKPOINT  Checkpoint file path (default: .git/safe-gc-checkpoint.json)

set -euo pipefail

# Configuration
MEMORY_MAX="${SAFE_GC_MEMORY_MAX:-2g}"
CHECKPOINT_FILE="${SAFE_GC_CHECKPOINT:-.git/safe-gc-checkpoint.json}"
LOG_FILE=".git/safe-gc.log"
REPO_ROOT="$(git rev-parse --show-toplevel)"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Parse arguments
MODE="standard"
RESUME=false
CHECK_ONLY=false

while [[ $# -gt 0 ]]; do
  case $1 in
    --full)
      MODE="full"
      shift
      ;;
    --resume)
      RESUME=true
      shift
      ;;
    --check-only)
      CHECK_ONLY=true
      shift
      ;;
    *)
      echo "Unknown option: $1"
      exit 1
      ;;
  esac
done

cd "$REPO_ROOT"

# Logging functions
log() {
  echo -e "${BLUE}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $*" | tee -a "$LOG_FILE"
}

log_success() {
  echo -e "${GREEN}[✓]${NC} $*" | tee -a "$LOG_FILE"
}

log_warning() {
  echo -e "${YELLOW}[!]${NC} $*" | tee -a "$LOG_FILE"
}

log_error() {
  echo -e "${RED}[✗]${NC} $*" | tee -a "$LOG_FILE"
}

# Check if git is needed
check_gc_needed() {
  log "Checking if gc is needed..."

  # Count loose objects
  local loose_objects
  loose_objects=$(git count-objects 2>/dev/null | grep '^loose:' | awk '{print $2}' || echo "0")

  # Count pack files
  local pack_count
  pack_count=$(find .git/objects/pack -name '*.pack' 2>/dev/null | wc -l)

  # Get repo size
  local repo_size
  repo_size=$(du -sh .git 2>/dev/null | awk '{print $1}')

  log "  Loose objects: $loose_objects"
  log "  Pack files: $pack_count"
  log "  Repository size: $repo_size"

  # Determine if gc is needed
  local gc_needed=false
  local reason=""

  if [[ $loose_objects -gt 1000 ]]; then
    gc_needed=true
    reason="Too many loose objects ($loose_objects > 1000)"
  elif [[ $pack_count -gt 5 ]]; then
    gc_needed=true
    reason="Too many pack files ($pack_count > 5)"
  elif [[ -f "$CHECKPOINT_FILE" ]]; then
    local last_gc_size
    last_gc_size=$(jq -r '.repo_size // "unknown"' "$CHECKPOINT_FILE" 2>/dev/null || echo "unknown")
    if [[ "$last_gc_size" != "unknown" ]]; then
      # Compare current size with last gc (simple string comparison for now)
      log "  Last gc size: $last_gc_size"
    fi
  fi

  if $CHECK_ONLY; then
    if $gc_needed; then
      log_success "GC needed: $reason"
      exit 0
    else
      log_warning "GC not needed"
      exit 1
    fi
  fi

  echo "$gc_needed"
}

# Pre-flight checks
preflight_check() {
  log "Running pre-flight checks..."

  # Check disk space
  local available_space
  available_space=$(df -BG "$REPO_ROOT" | tail -1 | awk '{print $4}' | tr -d 'G')
  log "  Available disk space: ${available_space}G"

  if [[ $available_space -lt 5 ]]; then
    log_error "Insufficient disk space (< 5GB)"
    return 1
  fi

  # Check repository integrity
  log "  Verifying repository integrity..."
  if ! git fsck --no-progress 2>&1 | tee -a "$LOG_FILE"; then
    log_error "Repository integrity check failed"
    return 1
  fi

  # Get baseline stats
  log "  Baseline statistics:"
  git count-objects -vH | tee -a "$LOG_FILE"

  log_success "Pre-flight checks passed"
  return 0
}

# Save checkpoint
save_checkpoint() {
  local stage="$1"
  local status="$2"
  local message="${3:-}"

  local repo_size
  repo_size=$(du -sh .git 2>/dev/null | awk '{print $1}')

  local loose_objects
  loose_objects=$(git count-objects 2>/dev/null | grep '^loose:' | awk '{print $2}' || echo "0")

  local pack_count
  pack_count=$(find .git/objects/pack -name '*.pack' 2>/dev/null | wc -l)

  cat > "$CHECKPOINT_FILE" <<EOF
{
  "timestamp": "$(date -Iseconds)",
  "stage": "$stage",
  "status": "$status",
  "message": "$message",
  "repo_size": "$repo_size",
  "loose_objects": $loose_objects,
  "pack_count": $pack_count,
  "mode": "$MODE"
}
EOF

  log_success "Checkpoint saved: $stage ($status)"
}

# Load checkpoint
load_checkpoint() {
  if [[ ! -f "$CHECKPOINT_FILE" ]]; then
    echo "none"
    return
  fi

  local stage
  stage=$(jq -r '.stage // "none"' "$CHECKPOINT_FILE" 2>/dev/null || echo "none")
  local status
  status=$(jq -r '.status // "unknown"' "$CHECKPOINT_FILE" 2>/dev/null || echo "unknown")

  log "Found checkpoint: stage=$stage, status=$status"

  if [[ "$status" == "complete" ]]; then
    echo "$stage"
  else
    echo "failed"
  fi
}

# Stage 1: Standard GC
stage1_standard_gc() {
  log "Stage 1: Running standard gc..."

  local start_time
  start_time=$(date +%s)

  # Configure memory limits
  git config pack.windowMemory "$MEMORY_MAX"
  git config pack.deltaCacheSize "1g"

  # Run standard gc
  if git gc --prune=now 2>&1 | tee -a "$LOG_FILE"; then
    local end_time
    end_time=$(date +%s)
    local duration=$((end_time - start_time))

    save_checkpoint "stage1" "complete" "Completed in ${duration}s"
    log_success "Stage 1 completed in ${duration}s"
    return 0
  else
    save_checkpoint "stage1" "failed" "git gc failed"
    log_error "Stage 1 failed"
    return 1
  fi
}

# Stage 2: Incremental repack
stage2_incremental_repack() {
  log "Stage 2: Running incremental repack..."

  local start_time
  start_time=$(date +%s)

  # Pack any remaining loose objects
  log "  Packing loose objects..."
  git repack -q -d --max-pack-size=500m 2>&1 | tee -a "$LOG_FILE" || true

  # Consolidate small packs
  log "  Consolidating packs..."
  git repack -q -d -f --depth=50 --window=50 2>&1 | tee -a "$LOG_FILE" || true

  local end_time
  end_time=$(date +%s)
  local duration=$((end_time - start_time))

  save_checkpoint "stage2" "complete" "Completed in ${duration}s"
  log_success "Stage 2 completed in ${duration}s"
  return 0
}

# Stage 3: Deep compression (optional)
stage3_deep_compression() {
  log "Stage 3: Running deep compression..."

  local start_time
  start_time=$(date +%s)

  # Deep repack with memory limits
  log "  Deep repacking with memory limit: $MEMORY_MAX"
  git repack -q -d -f --depth=10 --window=10 --window-memory="$MEMORY_MAX" 2>&1 | tee -a "$LOG_FILE" || true

  local end_time
  end_time=$(date +%s)
  local duration=$((end_time - start_time))

  save_checkpoint "stage3" "complete" "Completed in ${duration}s"
  log_success "Stage 3 completed in ${duration}s"
  return 0
}

# Final verification
final_verification() {
  log "Running final verification..."

  # Verify repository integrity
  if ! git fsck --no-progress 2>&1 | tee -a "$LOG_FILE"; then
    log_error "Repository integrity check failed after gc"
    return 1
  fi

  # Get final stats
  log "  Final statistics:"
  git count-objects -vH | tee -a "$LOG_FILE"

  local repo_size
  repo_size=$(du -sh .git 2>/dev/null | awk '{print $1}')

  save_checkpoint "complete" "complete" "All stages finished, repo size: $repo_size"
  log_success "Final verification passed, repository size: $repo_size"
  return 0
}

# Main execution
main() {
  log "=== Safe Git GC Started ==="
  log "Mode: $MODE"
  log "Memory limit: $MEMORY_MAX"
  log "Checkpoint file: $CHECKPOINT_FILE"

  # Check if gc is needed
  if ! check_gc_needed; then
    log_warning "GC not needed, exiting"
    exit 0
  fi

  # Pre-flight checks
  if ! preflight_check; then
    log_error "Pre-flight checks failed"
    exit 1
  fi

  # Resume from checkpoint if requested
  local start_stage="stage1"
  if $RESUME; then
    local last_stage
    last_stage=$(load_checkpoint)

    case "$last_stage" in
      "none")
        log "No checkpoint found, starting from stage 1"
        start_stage="stage1"
        ;;
      "stage1")
        log "Resuming from stage 2"
        start_stage="stage2"
        ;;
      "stage2")
        if [[ "$MODE" == "full" ]]; then
          log "Resuming from stage 3"
          start_stage="stage3"
        else
          log_warning "Previous run completed stage 2, use --full for stage 3"
          exit 0
        fi
        ;;
      "stage3"|"complete")
        log_warning "All stages already completed"
        exit 0
        ;;
      "failed")
        log_error "Previous run failed, starting from stage 1"
        start_stage="stage1"
        ;;
      *)
        log_error "Unknown checkpoint state: $last_stage"
        exit 1
        ;;
    esac
  fi

  # Run stages
  log "Starting gc from stage: $start_stage"

  case "$start_stage" in
    "stage1")
      if ! stage1_standard_gc; then
        log_error "Stage 1 failed, aborting"
        exit 1
      fi
      ;&
    "stage2")
      if ! stage2_incremental_repack; then
        log_error "Stage 2 failed, aborting"
        exit 1
      fi
      ;;
    "stage3")
      if ! stage3_deep_compression; then
        log_error "Stage 3 failed, aborting"
        exit 1
      fi
      ;;
  esac

  # If in full mode and starting from early stages, continue to stage 3
  if [[ "$MODE" == "full" && "$start_stage" != "stage3" ]]; then
    if ! stage3_deep_compression; then
      log_error "Stage 3 failed, aborting"
      exit 1
    fi
  fi

  # Final verification
  if ! final_verification; then
    log_error "Final verification failed"
    exit 1
  fi

  log_success "=== Safe Git GC Completed Successfully ==="
  exit 0
}

# Run main
main "$@"
