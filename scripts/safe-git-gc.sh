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
#   SAFE_GC_MEMORY_MAX     Soft pack memory: drives pack.windowMemory (default: 2g)
#   SAFE_GC_CGROUP_MAX     Hard cgroup ceiling applied to every git invocation
#                          (default: 6g). Must cover the soft sum — see the
#                          assertion in validate_config.
#   SAFE_GC_DELTA_CACHE    pack.deltaCacheSize used by stage 1 (default: 1g)
#   SAFE_GC_CHECKPOINT     Checkpoint file path (default: .git/safe-gc-checkpoint.json)
#   SAFE_GC_LOCK_FILE      Box-wide gc lock (default: /tmp/domain-check-safe-git-gc.lock)
#   SAFE_GC_LOCK_WAIT      Seconds to wait for the gc lock before skipping (default: 1800)
#   SAFE_GC_NO_CGROUP      Set to 1 to disable the cgroup ceiling (a ulimit -v
#                          fallback is applied instead when one is available)
#   SAFE_GC_MIN_DISK_GB    Minimum free disk in GB (default: 5, and >= 1.5x repo size)
#   SAFE_GC_MIN_AVAIL_MEM  Minimum available memory (default: cgroup ceiling + 1g)
#   SAFE_GC_MAX_LOAD       Maximum 1-minute load average (default: 15)
#
# Exit codes:
#   0  success (--check-only: gc IS needed)
#   1  failure (--check-only: gc not needed)
#   2  fail-fast: invalid configuration or insufficient resources
#
# The memory ceiling and box-wide lock are the implemented mitigations for the
# bf-65lsdu OOM crash (see docs/fix-proposal-bf-65lsdu-oom-git-gc-2026-09-02.md);
# scripts/test-safe-git-gc-limits.sh exercises both. When run from a systemd
# user service, the unit must set PATH (NixOS user-manager PATH lacks bash).

set -euo pipefail

# Configuration
# Soft limits (what git is *told* it may use) are kept separate from the hard
# ceiling (what the kernel is *told* to kill at). They were a single value
# before, which meant a run that legitimately filled its pack.windowMemory plus
# its delta cache was killed by its own protective cgroup — the failure the
# ceiling exists to prevent, relocated (docs/maintenance/stepwise-git-gc-strategy.md §7.1).
MEMORY_MAX="${SAFE_GC_MEMORY_MAX:-2g}"
CGROUP_MAX="${SAFE_GC_CGROUP_MAX:-6g}"
DELTA_CACHE="${SAFE_GC_DELTA_CACHE:-1g}"
CHECKPOINT_FILE="${SAFE_GC_CHECKPOINT:-.git/safe-gc-checkpoint.json}"
LOG_FILE=".git/safe-gc.log"
REPO_ROOT="$(git rev-parse --show-toplevel)"

# Box-wide serialization: the bf-65lsdu OOM (2026-08-13) was amplified by
# multiple git gc operations running concurrently, each consuming >4GB. One
# shared lockfile caps the whole box at a single gc run at a time.
LOCK_FILE="${SAFE_GC_LOCK_FILE:-/tmp/domain-check-safe-git-gc.lock}"
LOCK_WAIT_SECS="${SAFE_GC_LOCK_WAIT:-1800}"
NO_CGROUP="${SAFE_GC_NO_CGROUP:-0}"
NO_ULIMIT="${SAFE_GC_NO_ULIMIT:-0}"
CGROUP_CAP="unknown"
ULIMIT_CAP="off"
CHECKPOINT_STATUS="unknown"

# Unique transient unit name per call. A name must never be reused, for two
# independent reasons: $$ is recycled across processes on a long-uptime box
# (an OOM-killed scope lingers in failed state until reset), and every capped
# git invocation runs inside a pipeline (`run_memory_capped git … | tee`), so
# the function body executes in a subshell where a per-process counter resets
# to the same value each call. Reusing a still-loaded name makes systemd-run
# fail client-side with "already loaded or has a fragment file" and the
# command never runs. Nanosecond resolution inside the call keeps names fresh.
scope_unit_name() {  # scope_unit_name <role>
  echo "safe-git-gc-$1-$$-$(date +%s%N 2>/dev/null || echo "$RANDOM$RANDOM")"
}
GC_NEEDED=false
GC_REASON=""

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

# Convert a systemd-style size (bare bytes, K, M, G) to bytes
mem_to_bytes() {
  local value="${1^^}"
  local number
  number="$(tr -dc '0-9' <<< "$value")"
  if [[ -z "$number" ]]; then
    echo 0
    return
  fi
  case "$value" in
    *K) echo $((number * 1024)) ;;
    *M) echo $((number * 1024 * 1024)) ;;
    *G) echo $((number * 1024 * 1024 * 1024)) ;;
    *)  echo "$number" ;;
  esac
}

# Effective pack.threads from the chain a real gc actually sees
# (system -> global -> local). Unset or 0 means git auto-sizes to every core
# and multiplies pack.windowMemory by that count, so it is also the number the
# ceiling assertion has to assume.
effective_pack_threads() {
  local t
  t="$(git config --get pack.threads 2>/dev/null || echo "")"
  if [[ ! "$t" =~ ^[0-9]+$ ]] || [[ "$t" -eq 0 ]]; then
    nproc 2>/dev/null || echo 1
    return
  fi
  echo "$t"
}

# Fail fast on a configuration that cannot bound the run. Every size variable
# must parse, and the hard ceiling must cover the soft worst case
# (pack.windowMemory x pack.threads + pack.deltaCacheSize + slack). A ceiling
# below that sum turns the protection into a self-inflicted OOM kill — the
# exact failure the ceiling exists to prevent, relocated (see
# docs/maintenance/stepwise-git-gc-strategy.md §7.1 item 1).
validate_config() {
  local rc=0
  local window_bytes delta_bytes ceiling_bytes threads worst_bytes
  window_bytes="$(mem_to_bytes "$MEMORY_MAX")"
  delta_bytes="$(mem_to_bytes "$DELTA_CACHE")"
  ceiling_bytes="$(mem_to_bytes "$CGROUP_MAX")"

  if [[ $window_bytes -le 0 ]]; then
    log_error "SAFE_GC_MEMORY_MAX='$MEMORY_MAX' is not a valid size (use e.g. 2g, 512M, 1024k, or bare bytes)"
    rc=2
  fi
  if [[ $ceiling_bytes -le 0 ]]; then
    log_error "SAFE_GC_CGROUP_MAX='$CGROUP_MAX' is not a valid size (use e.g. 6g, 4G)"
    rc=2
  fi
  if [[ $delta_bytes -le 0 ]]; then
    log_error "SAFE_GC_DELTA_CACHE='$DELTA_CACHE' is not a valid size (use e.g. 1g)"
    rc=2
  fi
  if [[ $rc -ne 0 ]]; then
    return "$rc"
  fi

  threads="$(effective_pack_threads)"
  worst_bytes=$((window_bytes * threads + delta_bytes + 512 * 1024 * 1024))
  if [[ $ceiling_bytes -lt $worst_bytes ]]; then
    log_error "SAFE_GC_CGROUP_MAX ($CGROUP_MAX) does not cover the soft worst case: window $MEMORY_MAX x ${threads} threads + delta cache $DELTA_CACHE + 512m slack"
    if [[ "$threads" -gt 1 ]]; then
      log_error "pack.threads is not pinned — run ./scripts/setup-git-gc-config.sh to set pack.threads=1 and shrink the worst case"
    fi
    log_error "Raise SAFE_GC_CGROUP_MAX or lower the soft limits; refusing to start a run its own ceiling would kill"
    return 2
  fi

  log "Config validated: window=$MEMORY_MAX delta=$DELTA_CACHE ceiling=$CGROUP_MAX threads=$threads (worst case ≈ $((worst_bytes / 1024 / 1024))MiB)"
  return 0
}

# Pre-flight resource validation: refuse to start a gc the box cannot absorb.
# Defaults follow the documented operating limits (CLAUDE.md "Resource Limits").
# Used both by the real-run preflight (fail fast) and by --check-only (report,
# exit 2). Thresholds are configurable:
#   SAFE_GC_MIN_DISK_GB     free-disk floor, default 5G — raised to 1.5x the
#                           repository size, since a repack transiently needs
#                           roughly the repo size again for the new pack
#   SAFE_GC_MIN_AVAIL_MEM   available-memory floor, default cgroup ceiling + 1g
#   SAFE_GC_MAX_LOAD        1-minute load ceiling, default 15
check_resources() {
  local rc=0

  # Memory. The ceiling only protects the box if the memory is actually there;
  # below this floor the run is doomed before it starts.
  local avail_mem_kb=0 avail_mem_mb min_avail_bytes min_avail_mb
  avail_mem_kb="$(awk '/^MemAvailable:/{print $2}' /proc/meminfo 2>/dev/null || echo 0)"
  avail_mem_mb=$((avail_mem_kb / 1024))
  if [[ -n "${SAFE_GC_MIN_AVAIL_MEM:-}" ]]; then
    min_avail_bytes="$(mem_to_bytes "$SAFE_GC_MIN_AVAIL_MEM")"
    if [[ $min_avail_bytes -le 0 ]]; then
      log_error "SAFE_GC_MIN_AVAIL_MEM='$SAFE_GC_MIN_AVAIL_MEM' is not a valid size"
      return 2
    fi
  else
    min_avail_bytes=$(( $(mem_to_bytes "$CGROUP_MAX") + 1024 * 1024 * 1024 ))
  fi
  min_avail_mb=$((min_avail_bytes / 1024 / 1024))
  log "  Available memory: ${avail_mem_mb}M (minimum ${min_avail_mb}M)"
  if [[ $avail_mem_mb -lt $min_avail_mb ]]; then
    log_error "Insufficient memory: ${avail_mem_mb}M available < ${min_avail_mb}M required"
    rc=2
  fi

  # Disk. bf-4yjq ran an 18G repository — this floor is what refuses such a
  # repo before a repack doubles its footprint.
  local disk_gb=0 repo_mb=0 repo_gb=0 min_disk_gb
  disk_gb="$(df -BG --output=avail "$REPO_ROOT" 2>/dev/null | tail -1 | tr -d ' G')"
  disk_gb="${disk_gb:-0}"
  repo_mb="$(du -sm .git 2>/dev/null | awk '{print $1}')"
  repo_mb="${repo_mb:-0}"
  repo_gb=$(( (repo_mb + 1023) / 1024 ))
  min_disk_gb="${SAFE_GC_MIN_DISK_GB:-5}"
  if (( repo_gb * 3 / 2 > min_disk_gb )); then
    min_disk_gb=$(( repo_gb * 3 / 2 ))
  fi
  log "  Free disk: ${disk_gb}G (minimum ${min_disk_gb}G for a ${repo_gb}G repository)"
  if (( disk_gb < min_disk_gb )); then
    log_error "Insufficient disk space: ${disk_gb}G free < ${min_disk_gb}G required"
    rc=2
  fi

  # Load. Above the critical threshold the box is already thrashing; the
  # nightly timer will simply retry later.
  local load1 max_load
  load1="$(cut -d' ' -f1 /proc/loadavg 2>/dev/null || echo 0)"
  load1="${load1:-0}"
  max_load="${SAFE_GC_MAX_LOAD:-15}"
  log "  Load average (1m): $load1 (maximum $max_load)"
  if ! awk -v l="$load1" -v m="$max_load" 'BEGIN { exit !(l <= m) }'; then
    log_error "System load too high: 1m load $load1 > $max_load — deferring gc"
    rc=2
  fi

  if [[ $rc -eq 0 ]]; then
    log_success "Resource checks passed"
  fi
  return "$rc"
}

# Resolve which memory-enforcement mechanism applies. Preference order:
#   1. cgroup MemoryMax via the systemd user manager (cumulative, authoritative)
#   2. ulimit -v per-process address-space cap (no systemd / explicit opt-out)
#   3. nothing (only if both are disabled) — soft git limits alone
resolve_memory_enforcement() {
  CGROUP_CAP="off"
  if [[ "$NO_CGROUP" != "1" ]] && command -v systemd-run >/dev/null 2>&1; then
    if systemd-run --user --quiet --scope --unit="$(scope_unit_name probe)" \
        -p MemoryMin=1M -- true >/dev/null 2>&1; then
      CGROUP_CAP="on"
    fi
  fi

  ULIMIT_CAP="off"
  if [[ "$CGROUP_CAP" == "off" && "$NO_ULIMIT" != "1" ]]; then
    ULIMIT_CAP="on"
  fi
}

# Address-space cap (KiB) for the ulimit fallback: everything a legitimate git
# process can map — the delta window, the delta cache, both the old and the new
# pack during a repack — plus slack. Below this a runaway allocation dies
# instead of growing into the box.
ulimit_cap_kb() {
  local packs_bytes=0
  packs_bytes="$(du -sb .git/objects/pack 2>/dev/null | awk '{print $1}')"
  packs_bytes="${packs_bytes:-0}"
  echo $(( ($(mem_to_bytes "$MEMORY_MAX") + $(mem_to_bytes "$DELTA_CACHE") + 2 * packs_bytes + 512 * 1024 * 1024) / 1024 ))
}

# Run a command inside a hard memory bound. With the cgroup ceiling a runaway
# gc is OOM-killed inside its own cgroup instead of exhausting the box and
# letting the global OOM killer shoot an unrelated agent process — the
# mechanism behind the bf-65lsdu signal -1 crash (2026-08-13).
# Deliberately no MemoryHigh below the ceiling: a high<max throttle band turns
# an over-limit repack into an allocation-stall crawl rather than a fast kill.
run_memory_capped() {
  if [[ "$CGROUP_CAP" == "on" ]]; then
    local max_bytes
    max_bytes="$(mem_to_bytes "$CGROUP_MAX")"
    systemd-run --user --quiet --scope \
      --unit="$(scope_unit_name run)" \
      -p "MemoryMax=${max_bytes}" \
      -- "$@"
  elif [[ "$ULIMIT_CAP" == "on" ]]; then
    # Weaker than the cgroup: ulimit -v is per-process, not cumulative across
    # the child processes git spawns — but it still stops a single runaway.
    local cap_kb
    cap_kb="$(ulimit_cap_kb)"
    (
      if ! ulimit -v "$cap_kb" 2>/dev/null; then
        echo "safe-git-gc: could not apply ulimit -v ${cap_kb}KiB; running without the soft bound" >&2
      fi
      exec "$@"
    )
  else
    "$@"
  fi
}

# Serialize gc runs box-wide so total gc memory is bounded to one ceiling.
# Skips cleanly (exit 0) when another gc holds the lock — the nightly timer
# will simply run again, whereas stacking two gc runs is what OOM'd the box.
acquire_gc_lock() {
  if ! command -v flock >/dev/null 2>&1; then
    log_warning "flock not available; running without gc serialization"
    return 0
  fi
  if ! exec 9>"$LOCK_FILE"; then
    log_warning "Cannot open lock file $LOCK_FILE; running without gc serialization"
    return 0
  fi
  if ! flock -w "$LOCK_WAIT_SECS" 9; then
    log_warning "Another git gc holds $LOCK_FILE (waited ${LOCK_WAIT_SECS}s); skipping this run"
    exit 0
  fi
  log "Acquired gc lock: $LOCK_FILE"
}

# Check if gc is needed. Sets GC_NEEDED / GC_REASON rather than exiting, so
# --check-only can report the resource verdict in the same run and still exit
# with the established contract (0 = needed, 1 = not needed, 2 = resources).
check_gc_needed() {
  log "Checking if gc is needed..."
  GC_NEEDED=false
  GC_REASON=""

  # Count loose objects (`count:` is the loose-object count in verbose output;
  # there is no `loose:` key, so grep for that instead)
  local loose_objects
  loose_objects=$(git count-objects -v 2>/dev/null | grep '^count:' | awk '{print $2}' || echo "0")

  # Count pack files
  local pack_count
  pack_count=$(find .git/objects/pack -name '*.pack' 2>/dev/null | wc -l)

  # Get repo size
  local repo_size
  repo_size=$(du -sh .git 2>/dev/null | awk '{print $1}')

  log "  Loose objects: $loose_objects"
  log "  Pack files: $pack_count"
  log "  Repository size: $repo_size"

  if [[ $loose_objects -gt 1000 ]]; then
    GC_NEEDED=true
    GC_REASON="Too many loose objects ($loose_objects > 1000)"
  elif [[ $pack_count -gt 5 ]]; then
    GC_NEEDED=true
    GC_REASON="Too many pack files ($pack_count > 5)"
  elif [[ -f "$CHECKPOINT_FILE" ]]; then
    local last_gc_size
    last_gc_size=$(jq -r '.repo_size // "unknown"' "$CHECKPOINT_FILE" 2>/dev/null || echo "unknown")
    if [[ "$last_gc_size" != "unknown" ]]; then
      # Compare current size with last gc (simple string comparison for now)
      log "  Last gc size: $last_gc_size"
    fi
  fi
}

# Pre-flight checks: resources first (fail fast before any git work), then
# repository integrity.
preflight_check() {
  log "Running pre-flight checks..."

  if ! check_resources; then
    log_error "Resource checks failed — refusing to start (tune with SAFE_GC_MIN_DISK_GB / SAFE_GC_MIN_AVAIL_MEM / SAFE_GC_MAX_LOAD)"
    return 1
  fi

  # Check repository integrity
  log "  Verifying repository integrity..."
  if ! run_memory_capped git fsck --no-progress 2>&1 | tee -a "$LOG_FILE"; then
    log_error "Repository integrity check failed"
    return 1
  fi

  # Get baseline stats
  log "  Baseline statistics:"
  git count-objects -vH | tee -a "$LOG_FILE"

  log_success "Pre-flight checks passed"
  return 0
}

# Progress tracking. The checkpoint file doubles as the live progress record:
# a stage marks itself "running" (with its pid) when it starts, so
# safe-git-gc-monitor.sh and --check-only can tell a run in flight from the
# last completed one instead of only reporting stale post-run data.
save_progress() {
  local stage="$1"
  cat > "$CHECKPOINT_FILE" <<EOF
{
  "timestamp": "$(date -Iseconds)",
  "stage": "$stage",
  "status": "running",
  "message": "stage in progress",
  "pid": $$,
  "mode": "$MODE"
}
EOF
}

mark_interrupted() {
  local stage="$1"
  cat > "$CHECKPOINT_FILE" <<EOF
{
  "timestamp": "$(date -Iseconds)",
  "stage": "$stage",
  "status": "interrupted",
  "message": "run died mid-stage; resume with --resume",
  "mode": "$MODE"
}
EOF
  log_warning "Marked stage $stage interrupted (resume with --resume restarts at that stage)"
}

# A "running" entry left by a dead run is rewritten as "interrupted" so the
# monitor stops showing a phantom in-flight gc. A live pid means the lock
# failed somewhere — reported, not touched.
reap_stale_progress() {
  [[ -f "$CHECKPOINT_FILE" ]] || return 0
  local status
  status="$(jq -r '.status // "unknown"' "$CHECKPOINT_FILE" 2>/dev/null || echo "unknown")"
  [[ "$status" == "running" ]] || return 0

  local pid stage
  pid="$(jq -r '.pid // 0' "$CHECKPOINT_FILE" 2>/dev/null || echo 0)"
  stage="$(jq -r '.stage // "unknown"' "$CHECKPOINT_FILE" 2>/dev/null || echo "unknown")"
  if [[ "$pid" =~ ^[0-9]+$ ]] && [[ "$pid" -gt 0 ]] && kill -0 "$pid" 2>/dev/null; then
    log_warning "Checkpoint says a gc is running (pid $pid, stage $stage) but the lock was free — pid may be stale or recycled"
    return 0
  fi
  log_warning "Previous run died during stage $stage (pid $pid no longer exists)"
  mark_interrupted "$stage"
}

# Surface a currently-running gc in --check-only output.
report_running_gc() {
  [[ -f "$CHECKPOINT_FILE" ]] || return 0
  local status
  status="$(jq -r '.status // ""' "$CHECKPOINT_FILE" 2>/dev/null || echo "")"
  [[ "$status" == "running" ]] || return 0

  local pid stage ts elapsed
  pid="$(jq -r '.pid // 0' "$CHECKPOINT_FILE" 2>/dev/null || echo 0)"
  stage="$(jq -r '.stage // "?"' "$CHECKPOINT_FILE" 2>/dev/null || echo "?")"
  ts="$(jq -r '.timestamp // ""' "$CHECKPOINT_FILE" 2>/dev/null || echo "")"
  elapsed="unknown"
  if [[ -n "$ts" ]]; then
    elapsed=$(( $(date +%s) - $(date -d "$ts" +%s 2>/dev/null || echo "$(date +%s)") ))s
  fi
  if [[ "$pid" =~ ^[0-9]+$ ]] && [[ "$pid" -gt 0 ]] && kill -0 "$pid" 2>/dev/null; then
    log "GC in progress: pid $pid, stage $stage, elapsed $elapsed"
  else
    log "Stale 'running' checkpoint (pid $pid gone) — no gc is actually in flight"
  fi
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
  loose_objects=$(git count-objects -v 2>/dev/null | grep '^count:' | awk '{print $2}' || echo "0")

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

# Load checkpoint. Sets LAST_STAGE and CHECKPOINT_STATUS rather than echoing:
# main needs BOTH values, and a function called in a command substitution
# cannot hand back an assignment — the subshell's copy of CHECKPOINT_STATUS
# was discarded on return, so --resume always saw "unknown" and the
# interrupted-stage restarts below were dead code.
#   LAST_STAGE         the recorded stage, "none" without a checkpoint
#   CHECKPOINT_STATUS  complete | interrupted | failed | unknown | none
#                      (repack and gc are safe to re-run — they write new
#                      packs before removing old ones)
load_checkpoint() {
  LAST_STAGE="none"
  CHECKPOINT_STATUS="none"
  if [[ ! -f "$CHECKPOINT_FILE" ]]; then
    return 0
  fi

  local stage
  stage=$(jq -r '.stage // "none"' "$CHECKPOINT_FILE" 2>/dev/null || echo "none")
  local status
  status=$(jq -r '.status // "unknown"' "$CHECKPOINT_FILE" 2>/dev/null || echo "unknown")

  LAST_STAGE="$stage"
  CHECKPOINT_STATUS="$status"
  log "Found checkpoint: stage=$stage, status=$status"
}

# Stage 1: Standard GC
stage1_standard_gc() {
  save_progress "stage1"
  log "Stage 1: Running standard gc..."

  local start_time
  start_time=$(date +%s)

  # Configure memory limits
  git config pack.windowMemory "$MEMORY_MAX"
  git config pack.deltaCacheSize "$DELTA_CACHE"

  # Run standard gc
  if run_memory_capped git gc --prune=now 2>&1 | tee -a "$LOG_FILE"; then
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
  save_progress "stage2"
  log "Stage 2: Running incremental repack..."

  local start_time
  start_time=$(date +%s)

  # This repo sets repack.writeBitmaps=true, and incremental repacks (repack
  # -d without -a) are incompatible with bitmap indexes: git aborts both of
  # the steps below with "Incremental repacks are incompatible with bitmap
  # indexes" unless --no-write-bitmap-index is passed. The 2026-09-02 07:55
  # run skipped both steps for exactly that reason while the surrounding
  # `|| true` reported the stage complete — so the flag is now explicit and
  # a repack failure fails the stage (resumable) instead of being swallowed.
  # Pack any remaining loose objects
  log "  Packing loose objects..."
  if ! run_memory_capped git repack -q -d --no-write-bitmap-index --max-pack-size=500m 2>&1 | tee -a "$LOG_FILE"; then
    save_checkpoint "stage2" "failed" "incremental repack (loose objects) failed"
    log_error "Stage 2 failed: incremental repack (loose objects)"
    return 1
  fi

  # Consolidate small packs
  log "  Consolidating packs..."
  if ! run_memory_capped git repack -q -d -f --no-write-bitmap-index --depth=50 --window=50 2>&1 | tee -a "$LOG_FILE"; then
    save_checkpoint "stage2" "failed" "incremental repack (pack consolidation) failed"
    log_error "Stage 2 failed: incremental repack (pack consolidation)"
    return 1
  fi

  local end_time
  end_time=$(date +%s)
  local duration=$((end_time - start_time))

  save_checkpoint "stage2" "complete" "Completed in ${duration}s"
  log_success "Stage 2 completed in ${duration}s"
  return 0
}

# Stage 3: Deep compression (optional)
stage3_deep_compression() {
  save_progress "stage3"
  log "Stage 3: Running deep compression..."

  local start_time
  start_time=$(date +%s)

  # Deep repack with memory limits (--no-write-bitmap-index: -d alone is an
  # incremental repack, which cannot run against this repo's bitmap index;
  # see stage 2 for the failure this previously hid)
  log "  Deep repacking with memory limit: $MEMORY_MAX"
  if ! run_memory_capped git repack -q -d -f --no-write-bitmap-index --depth=10 --window=10 --window-memory="$MEMORY_MAX" 2>&1 | tee -a "$LOG_FILE"; then
    save_checkpoint "stage3" "failed" "deep repack failed"
    log_error "Stage 3 failed: deep repack"
    return 1
  fi

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
  if ! run_memory_capped git fsck --no-progress 2>&1 | tee -a "$LOG_FILE"; then
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
  log "Memory: window=$MEMORY_MAX delta=$DELTA_CACHE ceiling=$CGROUP_MAX"
  log "Checkpoint file: $CHECKPOINT_FILE"

  # Refuse a configuration that cannot bound the run before doing anything.
  if ! validate_config; then
    log_error "Configuration validation failed"
    exit 2
  fi

  # --check-only never mutates anything and must work regardless of a
  # concurrently running gc, so it does not take the lock.
  if ! $CHECK_ONLY; then
    acquire_gc_lock
    resolve_memory_enforcement
    if [[ "$CGROUP_CAP" == "on" ]]; then
      log "Memory enforcement: cgroup MemoryMax=$CGROUP_MAX"
    elif [[ "$ULIMIT_CAP" == "on" ]]; then
      log "Memory enforcement: ulimit -v ($(ulimit_cap_kb)KiB address-space cap; no systemd user manager)"
    else
      log_warning "Memory enforcement: none — soft git limits only (SAFE_GC_NO_CGROUP=1 SAFE_GC_NO_ULIMIT=1)"
    fi
    reap_stale_progress
  fi

  # The thresholds inside check_gc_needed only drive --check-only reporting.
  # An explicit or scheduled run always executes the stages: an early exit here
  # would let small bloat accumulate between nightly runs, and the previous
  # `if ! check_gc_needed` guard was dead code — outside --check-only mode the
  # function always returns 0.
  check_gc_needed

  if $CHECK_ONLY; then
    # Resource validation mode: report the box's ability to absorb a gc and
    # the gc-needed verdict. Exit contract: 2 = resources insufficient,
    # 0 = gc needed, 1 = gc not needed.
    if ! check_resources; then
      log_error "Resource validation failed (exit 2)"
      exit 2
    fi
    report_running_gc
    if $GC_NEEDED; then
      log_success "GC needed: $GC_REASON"
      exit 0
    fi
    log_warning "GC not needed"
    exit 1
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
    load_checkpoint
    last_stage="$LAST_STAGE"

    case "$last_stage" in
      "none")
        log "No checkpoint found, starting from stage 1"
        start_stage="stage1"
        ;;
      "stage1")
        if [[ "$CHECKPOINT_STATUS" == "interrupted" ]]; then
          log "Stage 1 was interrupted — restarting it"
          start_stage="stage1"
        else
          log "Resuming from stage 2"
          start_stage="stage2"
        fi
        ;;
      "stage2")
        if [[ "$CHECKPOINT_STATUS" == "interrupted" ]]; then
          log "Stage 2 was interrupted — restarting it"
          start_stage="stage2"
        elif [[ "$MODE" == "full" ]]; then
          log "Resuming from stage 3"
          start_stage="stage3"
        else
          log_warning "Previous run completed stage 2, use --full for stage 3"
          exit 0
        fi
        ;;
      "stage3")
        if [[ "$CHECKPOINT_STATUS" == "interrupted" ]]; then
          log "Stage 3 was interrupted — restarting it"
          start_stage="stage3"
        else
          log_warning "All stages already completed"
          exit 0
        fi
        ;;
      "complete")
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

# Run main only when executed directly; sourcing this file (e.g. from
# scripts/test-safe-git-gc-limits.sh) exposes the helpers without side effects.
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
  main "$@"
fi
