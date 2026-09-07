#!/usr/bin/env bash
# Repository Health Monitoring Script
# Automated monitoring of repository health metrics to prevent bloat-induced crashes
#
# Usage: ./scripts/repo-health-monitor.sh [options]
#   --warn-only     Exit 0 regardless of findings (for monitoring)
#   --verbose       Show detailed metrics
#   --cron          Cron-friendly output (minimal)
#
# Exit codes:
#   0 - All checks passed
#   1 - Health issues detected (unless --warn-only)
#   2 - Error in execution

set -euo pipefail

# Configuration
WARN_ONLY=false
VERBOSE=false
CRON_MODE=false

# Alert thresholds (from crash pattern analysis)
# Overridable via environment so tests can exercise the warn paths without
# touching real repository state.
REPO_SIZE_WARN_GB="${REPO_SIZE_WARN_GB:-1}"        # Warn if repo > 1GB
LOOSE_OBJECTS_WARN="${LOOSE_OBJECTS_WARN:-1000}"   # Warn if loose objects > 1000
LOOSE_SIZE_WARN_MB="${LOOSE_SIZE_WARN_MB:-100}"    # Warn if loose MASS > 100MB (count misses a few huge objects)
PACK_FILES_WARN="${PACK_FILES_WARN:-2}"            # Pack count > this is fragmentation (not a signal on its own)
PACK_SIZE_WARN_MB="${PACK_SIZE_WARN_MB:-500}"      # ...fragmentation warns only when packed mass also exceeds this
DISK_FREE_WARN_GB="${DISK_FREE_WARN_GB:-30}"       # Warn if disk free < 30GB

# Colors (disable in cron mode)
if [[ -t 1 ]] && [[ "$CRON_MODE" != true ]]; then
  RED='\033[0;31m'
  GREEN='\033[0;32m'
  YELLOW='\033[1;33m'
  BLUE='\033[0;34m'
  NC='\033[0m'
else
  RED=''
  GREEN=''
  YELLOW=''
  BLUE=''
  NC=''
fi

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --warn-only)
      WARN_ONLY=true
      shift
      ;;
    --verbose)
      VERBOSE=true
      shift
      ;;
    --cron)
      CRON_MODE=true
      shift
      ;;
    -h|--help)
      cat <<EOF
Usage: $0 [options]

Monitor repository health metrics to prevent bloat-induced crashes.

Options:
  --warn-only     Exit 0 regardless of findings (for cron/monitoring)
  --verbose       Show detailed metrics
  --cron          Cron-friendly minimal output
  -h, --help      Show this help message

Environment (all thresholds overridable, defaults shown):
  REPO_SIZE_WARN_GB=1       Warn if .git exceeds this size
  LOOSE_OBJECTS_WARN=1000   Warn if loose object COUNT exceeds this
  LOOSE_SIZE_WARN_MB=100    Warn if loose object MASS exceeds this
  PACK_FILES_WARN=2         Pack count above this is fragmentation (informational on its own)
  PACK_SIZE_WARN_MB=500     ...and only warns when packed mass also exceeds this
  DISK_FREE_WARN_GB=30      Warn if free disk space drops below this

Exit codes:
  0 - All checks passed
  1 - Health issues detected (unless --warn-only)
  2 - Error in execution

Calibration note (2026-09-07): pack COUNT alone is a non-signal — a healthy
repo holds several small packs as normal bounded-gc churn — so the
fragmentation warning keys on packed MASS (size-pack) as well. Loose-object
MASS (size) is checked alongside the loose-object count, because the historical
bloat (bf-1s6c3/bf-4yjq) was a few hundred huge objects that a count threshold
never sees. See docs/crash-mitigation-strategies.md.
EOF
      exit 0
      ;;
    *)
      echo "Error: Unknown argument: $1" >&2
      exit 2
      ;;
  esac
done

# Logging functions
log_info() {
  [[ "$CRON_MODE" != true ]] && echo -e "${BLUE}[INFO]${NC} $*" || true
}

log_warn() {
  echo -e "${YELLOW}[WARN]${NC} $*"
}

log_error() {
  echo -e "${RED}[ERROR]${NC} $*" >&2
}

log_success() {
  echo -e "${GREEN}[OK]${NC} $*"
}

# Get repository metrics
get_repo_metrics() {
  local repo_size_kb=$(du -sk .git 2>/dev/null | awk '{print $1}')
  local repo_size_mb=$((repo_size_kb / 1024))
  local repo_size_gb=$(awk "BEGIN {printf \"%.2f\", $repo_size_mb / 1024}")

  local git_stats=$(git count-objects -vH 2>/dev/null || true)
  local loose_objects=$(echo "$git_stats" | grep '^count:' | awk '{print $2}' || echo 0)
  local pack_count=$(echo "$git_stats" | grep '^packs:' | awk '{print $2}' || echo 0)
  local in_pack=$(echo "$git_stats" | grep '^in-pack:' | awk '{print $2}' || echo 0)

  # Raw sizes (KiB) from the non-humanized output: loose mass and packed mass
  # are the operative bloat metrics — object COUNT is a non-signal.
  local git_sizes=$(git count-objects -v 2>/dev/null || true)
  local loose_size_mb=$(echo "$git_sizes" | grep '^size:' | awk '{print int($2/1024)}' || echo 0)
  local pack_size_mb=$(echo "$git_sizes" | grep '^size-pack:' | awk '{print int($2/1024)}' || echo 0)
  loose_size_mb=${loose_size_mb:-0}
  pack_size_mb=${pack_size_mb:-0}

  local disk_free=$(df -BG / 2>/dev/null | awk 'NR==2 {print $4}' | tr -d 'G' || echo 0)

  echo "$repo_size_gb|$loose_objects|$pack_count|$in_pack|$disk_free|$loose_size_mb|$pack_size_mb"
}

# Main monitoring logic
main() {
  local exit_code=0

  if [[ "$CRON_MODE" != true ]]; then
    log_info "=== Repository Health Monitor ==="
    log_info "Timestamp: $(date -Iseconds)"
    log_info "Workspace: $(pwd)"
    echo ""
  fi

  # Get metrics
  local metrics=$(get_repo_metrics)
  local repo_size_gb=$(echo "$metrics" | cut -d'|' -f1)
  local loose_objects=$(echo "$metrics" | cut -d'|' -f2)
  local pack_count=$(echo "$metrics" | cut -d'|' -f3)
  local in_pack=$(echo "$metrics" | cut -d'|' -f4)
  local disk_free=$(echo "$metrics" | cut -d'|' -f5)
  local loose_size_mb=$(echo "$metrics" | cut -d'|' -f6)
  local pack_size_mb=$(echo "$metrics" | cut -d'|' -f7)

  if [[ "$VERBOSE" == true ]] || [[ "$CRON_MODE" == true ]]; then
    echo "REPO_SIZE_GB=$repo_size_gb"
    echo "LOOSE_OBJECTS=$loose_objects"
    echo "LOOSE_SIZE_MB=$loose_size_mb"
    echo "PACK_COUNT=$pack_count"
    echo "IN_PACK=$in_pack"
    echo "PACK_SIZE_MB=$pack_size_mb"
    echo "DISK_FREE_GB=$disk_free"
  fi

  if [[ "$CRON_MODE" != true ]]; then
    echo "Repository Size: ${repo_size_gb}GB"
    echo "Loose Objects: $loose_objects (${loose_size_mb}MB)"
    echo "Pack Files: $pack_count (${pack_size_mb}MB packed)"
    echo "Packed Objects: $in_pack"
    echo "Disk Space Free: ${disk_free}GB"
    echo ""
  fi

  # Check repository size
  repo_size_warn=$(awk "BEGIN {print ($repo_size_gb > $REPO_SIZE_WARN_GB) ? \"1\" : \"0\"}")
  if [[ "$repo_size_warn" -eq 1 ]]; then
    log_warn "Repository size exceeds threshold: ${repo_size_gb}GB > ${REPO_SIZE_WARN_GB}GB"
    log_warn "Action needed: Run ./scripts/safe-git-gc.sh --full"
    exit_code=1
  fi

  # Check loose objects
  loose_warn=$(awk "BEGIN {print ($loose_objects > $LOOSE_OBJECTS_WARN) ? \"1\" : \"0\"}")
  if [[ "$loose_warn" -eq 1 ]]; then
    log_warn "Excessive loose objects: $loose_objects > $LOOSE_OBJECTS_WARN"
    log_warn "Action needed: Run git gc or ./scripts/safe-git-gc.sh"
    exit_code=1
  fi

  # Check loose object MASS — the count band misses the historical bloat shape
  # (bf-1s6c3/bf-4yjq: 17GB loose in a few hundred huge snapshot blobs)
  loose_mass_warn=$(awk "BEGIN {print ($loose_size_mb > $LOOSE_SIZE_WARN_MB) ? \"1\" : \"0\"}")
  if [[ "$loose_mass_warn" -eq 1 ]]; then
    log_warn "Excessive loose object mass: ${loose_size_mb}MB > ${LOOSE_SIZE_WARN_MB}MB ($loose_objects objects)"
    log_warn "Action needed: Run ./scripts/safe-git-gc.sh"
    exit_code=1
  fi

  # Check pack store: fragmentation is only a signal at scale. A healthy repo
  # holds several small packs as normal bounded-gc churn (2026-09-07: 3 packs /
  # 99 MiB on a 103 MB repo — flagged here as a false alarm before calibration),
  # so warn only when the packed MASS is also up, and report the rest
  # informationally.
  pack_warn=$(awk "BEGIN {print ($pack_count > $PACK_FILES_WARN && $pack_size_mb > $PACK_SIZE_WARN_MB) ? \"1\" : \"0\"}")
  if [[ "$pack_warn" -eq 1 ]]; then
    log_warn "Fragmented and heavy pack store: $pack_count packs totalling ${pack_size_mb}MB (packed mass > ${PACK_SIZE_WARN_MB}MB)"
    log_warn "Action needed: consolidate with ./scripts/safe-git-gc.sh --full"
    exit_code=1
  elif [[ "$pack_count" -gt "$PACK_FILES_WARN" ]]; then
    log_info "Pack store: $pack_count packs / ${pack_size_mb}MB — fragmented but small (informational, no action)"
  fi

  # Check disk space
  disk_warn=$(awk "BEGIN {print ($disk_free < $DISK_FREE_WARN_GB) ? \"1\" : \"0\"}")
  if [[ "$disk_warn" -eq 1 ]]; then
    log_warn "Low disk space: ${disk_free}GB free < ${DISK_FREE_WARN_GB}GB"
    log_warn "Action needed: Free up disk space or clean git objects"
    exit_code=1
  fi

  # Final summary
  if [[ "$exit_code" -eq 0 ]]; then
    log_success "All repository health checks passed"
  else
    log_warn "Repository health issues detected"
    if [[ "$CRON_MODE" != true ]]; then
      echo ""
      echo "Recommended actions:"
      [[ "$repo_size_warn" -eq 1 ]] && echo "  - Run: ./scripts/safe-git-gc.sh --full"
      [[ "$loose_warn" -eq 1 ]] && echo "  - Run: ./scripts/safe-git-gc.sh"
      [[ "$loose_mass_warn" -eq 1 ]] && echo "  - Run: ./scripts/safe-git-gc.sh (loose mass: ${loose_size_mb}MB)"
      [[ "$pack_warn" -eq 1 ]] && echo "  - Run: ./scripts/safe-git-gc.sh --full"
      [[ "$disk_warn" -eq 1 ]] && echo "  - Free disk space: df -h /"
    fi
  fi

  # If warn-only mode, exit 0 regardless
  [[ "$WARN_ONLY" == true ]] && exit_code=0

  exit $exit_code
}

main "$@"
