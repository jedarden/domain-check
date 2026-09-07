#!/usr/bin/env bash
# Crash-safe repository bloat cleanup.
#
# Replaces the old "bare git gc --aggressive --prune=now" one-liner that caused
# the bf-1s6c3 crash (18GB repo / 17GB loose objects -> OOM kill mid-gc).
#
# Safety features:
#   - Pre-flight checks: free disk, free memory, load average, concurrent-gc
#     lock staleness, repository integrity (fsck, itself memory-bounded)
#   - Layered memory bounds on every git invocation: a hard systemd MemoryMax
#     scope when the user manager is reachable, plus an RSS-sampling guard
#     that kills the whole process group at the cap before the kernel
#     OOM killer can
#   - Staged execution with checkpoint/resume after every stage
#   - Progress heartbeat and verbose logging to $GIT_DIR/cleanup-bloat.log
#
# Usage: scripts/cleanup-bloat.sh [options]
#
# Options:
#   --check-only        Report whether cleanup is needed, change nothing.
#                       Exit 0 = cleanup needed, exit 1 = not needed — the
#                       exit code reflects repository state ONLY. Resource
#                       state is logged exactly as it would gate a real run,
#                       but a resource shortfall never becomes exit 1, which
#                       monitoring would misread as "no cleanup needed".
#                       Takes no lock: safe to run while a cleanup or a git
#                       gc is in progress, and it never clears stale locks.
#   --dry-run           Run pre-flight checks and print the staged plan
#                       without touching the repository.
#   --resume            Resume after the last checkpointed stage.
#   --conservative      Keep reflogs and only prune objects older than
#                       2 weeks (less space reclaimed, nothing unrecoverable).
#   --force             Run even if the bloat heuristics say it is not needed,
#                       and clear a stale concurrent-gc lock.
#   --memory-max MB     git pack memory limit AND basis for the process-tree
#                       RSS kill cap (default: 2048).
#   --verbose, -v       Per-sample monitor output (a heartbeat is logged every
#                       10th sample regardless).
#   --selftest-monitor  Verify the memory guard kills an over-cap process.
#                       No git operations are performed.
#   --help, -h          This message.
#
# Environment variables (flags take precedence where both exist):
#   CLEANUP_BLOAT_MEMORY_MAX_MB     default 2048
#   CLEANUP_BLOAT_RSS_HEADROOM_MB   kill cap = memory-max + this (default 512)
#   CLEANUP_BLOAT_MIN_AVAIL_MB      min available memory to start (default 10240)
#   CLEANUP_BLOAT_MIN_DISK_MB       min free disk to start (default 20480)
#   CLEANUP_BLOAT_DISK_HEADROOM     free disk must also exceed repo size x this
#                                   (default 2)
#   CLEANUP_BLOAT_MAX_LOAD          max 1-minute load average (default 15)
#   CLEANUP_BLOAT_CGROUP            hard memory ceiling via a transient systemd
#                                   user scope: auto (default — use when the
#                                   user manager answers) | always (abort if
#                                   unavailable) | never
#   CLEANUP_BLOAT_DELTA_CACHE_MB    git pack.deltaCacheSize; 0 derives it from
#                                   memory-max (default 0)
#   CLEANUP_BLOAT_FSCK_MAX_REPO_MB  skip the fsck gate above this repo size in
#                                   MB; 0 never skips (default 0)
#   CLEANUP_BLOAT_FSCK_TIMEOUT      seconds before fsck is skipped as slow
#                                   (default 300)
#   CLEANUP_BLOAT_MONITOR_INTERVAL  seconds between RSS samples (default 2)
#   CLEANUP_BLOAT_THREADS           git pack.threads, 1 = lowest memory
#                                   (default 1)
#
# For routine maintenance (small loose-object counts) prefer safe-git-gc.sh;
# this script is for reclaiming genuine bloat: large loose-object pools,
# unreachable objects, and over-fragmented packs.

set -euo pipefail

# ---------------------------------------------------------------------------
# Configuration
# ---------------------------------------------------------------------------

MEMORY_MAX_MB="${CLEANUP_BLOAT_MEMORY_MAX_MB:-2048}"
RSS_HEADROOM_MB="${CLEANUP_BLOAT_RSS_HEADROOM_MB:-512}"
MIN_AVAIL_MB="${CLEANUP_BLOAT_MIN_AVAIL_MB:-10240}"
MIN_DISK_MB="${CLEANUP_BLOAT_MIN_DISK_MB:-20480}"
DISK_HEADROOM="${CLEANUP_BLOAT_DISK_HEADROOM:-2}"
MAX_LOAD="${CLEANUP_BLOAT_MAX_LOAD:-15}"
FSCK_TIMEOUT="${CLEANUP_BLOAT_FSCK_TIMEOUT:-300}"
FSCK_MAX_REPO_MB="${CLEANUP_BLOAT_FSCK_MAX_REPO_MB:-0}"
MONITOR_INTERVAL="${CLEANUP_BLOAT_MONITOR_INTERVAL:-2}"
PACK_THREADS="${CLEANUP_BLOAT_THREADS:-1}"
CGROUP_MODE="${CLEANUP_BLOAT_CGROUP:-auto}"
DELTA_CACHE_MB="${CLEANUP_BLOAT_DELTA_CACHE_MB:-0}"   # 0 = derive from memory-max

MODE="aggressive"     # or "conservative"
RESUME=false
CHECK_ONLY=false
DRY_RUN=false
FORCE=false
VERBOSE=false
SELFTEST=false

require_int() {
  if ! [[ "$1" =~ ^[0-9]+$ ]]; then
    echo "ERROR: expected an integer, got '$1' ($2)" >&2
    exit 2
  fi
}
require_int "$MEMORY_MAX_MB"     "CLEANUP_BLOAT_MEMORY_MAX_MB"
require_int "$MIN_AVAIL_MB"      "CLEANUP_BLOAT_MIN_AVAIL_MB"
require_int "$MIN_DISK_MB"       "CLEANUP_BLOAT_MIN_DISK_MB"
require_int "$RSS_HEADROOM_MB"   "CLEANUP_BLOAT_RSS_HEADROOM_MB"
require_int "$FSCK_TIMEOUT"      "CLEANUP_BLOAT_FSCK_TIMEOUT"
require_int "$FSCK_MAX_REPO_MB"  "CLEANUP_BLOAT_FSCK_MAX_REPO_MB"
require_int "$MONITOR_INTERVAL"  "CLEANUP_BLOAT_MONITOR_INTERVAL"
require_int "$PACK_THREADS"      "CLEANUP_BLOAT_THREADS"
require_int "$DELTA_CACHE_MB"    "CLEANUP_BLOAT_DELTA_CACHE_MB"
require_int "$DISK_HEADROOM"     "CLEANUP_BLOAT_DISK_HEADROOM"
case "$CGROUP_MODE" in
  auto|always|never) ;;
  *) echo "ERROR: CLEANUP_BLOAT_CGROUP must be auto, always or never (got '$CGROUP_MODE')" >&2; exit 2 ;;
esac
if (( MONITOR_INTERVAL < 1 )); then
  echo "WARN: CLEANUP_BLOAT_MONITOR_INTERVAL < 1 would busy-loop; using 1s" >&2
  MONITOR_INTERVAL=1
fi

# ---------------------------------------------------------------------------
# Argument parsing
# ---------------------------------------------------------------------------

usage() {
  # print the leading comment block, minus the "# " prefix
  awk 'NR == 1 { next } /^#/ { sub(/^# ?/, ""); print; next } { exit }' "$0"
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check-only)       CHECK_ONLY=true; shift ;;
    --dry-run)          DRY_RUN=true; shift ;;
    --resume)           RESUME=true; shift ;;
    --conservative)     MODE="conservative"; shift ;;
    --force)            FORCE=true; shift ;;
    --verbose|-v)       VERBOSE=true; shift ;;
    --memory-max)       [[ $# -ge 2 ]] || { echo "--memory-max requires a value" >&2; exit 2; }
                        require_int "$2" "--memory-max"
                        MEMORY_MAX_MB="$2"; shift 2 ;;
    --selftest-monitor) SELFTEST=true; shift ;;
    --help|-h)          usage; exit 0 ;;
    *)                  echo "Unknown option: $1 (see --help)" >&2; exit 2 ;;
  esac
done

if (( MEMORY_MAX_MB < 64 )); then
  echo "ERROR: --memory-max must be at least 64MB (got ${MEMORY_MAX_MB}MB)" >&2
  exit 2
fi

if (( DELTA_CACHE_MB == 0 )); then
  # Derive the delta cache from the operator's memory budget instead of a
  # fixed 256MB (audit gap 10): with --memory-max 64 the fixed cache alone
  # promised git four times the memory the operator configured.
  DELTA_CACHE_MB=$(( MEMORY_MAX_MB / 4 ))
  (( DELTA_CACHE_MB < 32 ))  && DELTA_CACHE_MB=32
  (( DELTA_CACHE_MB > 256 )) && DELTA_CACHE_MB=256
fi

# ---------------------------------------------------------------------------
# Logging
# ---------------------------------------------------------------------------

RED='\033[0;31m'; GREEN='\033[0;32m'; YELLOW='\033[1;33m'; BLUE='\033[0;34m'; NC='\033[0m'

log()        { echo -e "${BLUE}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $*" | tee -a "$LOG_FILE"; }
log_ok()     { echo -e "${GREEN}[✓]${NC} $*" | tee -a "$LOG_FILE"; }
log_warn()   { echo -e "${YELLOW}[!]${NC} $*" | tee -a "$LOG_FILE"; }
log_error()  { echo -e "${RED}[✗]${NC} $*" | tee -a "$LOG_FILE"; }
log_verbose() {
  if [[ "$VERBOSE" == true ]]; then
    echo -e "  ${BLUE}[trace]${NC} $*" | tee -a "$LOG_FILE"
  else
    echo "  [trace] $*" >> "$LOG_FILE"
  fi
}

human_kb() {
  awk -v k="${1:-0}" 'BEGIN {
    if (k+0 >= 1048576)      printf "%.1fG", k/1048576;
    else if (k+0 >= 1024)    printf "%.1fM", k/1024;
    else                     printf "%dK", k;
  }'
}

# ---------------------------------------------------------------------------
# Locate repository (before LOG_FILE exists — plain errors, no logging yet)
# ---------------------------------------------------------------------------

if ! GIT_DIR="$(git rev-parse --absolute-git-dir 2>/dev/null)"; then
  echo "ERROR: not inside a git repository" >&2
  exit 1
fi
REPO_ROOT="$(git rev-parse --show-toplevel)"
cd "$REPO_ROOT"

LOG_FILE="$GIT_DIR/cleanup-bloat.log"
CKPT_FILE="$GIT_DIR/cleanup-bloat-checkpoint"
STATS_FILE="$GIT_DIR/cleanup-bloat-monitor.stats"
VIOLATION_FILE="$GIT_DIR/cleanup-bloat-monitor.violation"
LOCK_DIR="$GIT_DIR/cleanup-bloat.lock"
RSS_CAP_MB=$((MEMORY_MAX_MB + RSS_HEADROOM_MB))
PAGE_SIZE="$(getconf PAGESIZE 2>/dev/null || echo 4096)"

SELFTEST_TMP=""
if [[ "$SELFTEST" == true ]]; then
  # The self-test exercises the RSS guard, which writes $STATS_FILE /
  # $VIOLATION_FILE and appends to the log — keep all of that out of the real
  # repository's .git (audit gap 4): a self-test running next to a real
  # cleanup must not be able to delete the live stage's violation evidence or
  # corrupt its peak accounting.
  SELFTEST_TMP="$(mktemp -d "${TMPDIR:-/tmp}/cleanup-bloat-selftest.XXXXXX")"
  LOG_FILE="$SELFTEST_TMP/cleanup-bloat.log"
  STATS_FILE="$SELFTEST_TMP/cleanup-bloat-monitor.stats"
  VIOLATION_FILE="$SELFTEST_TMP/cleanup-bloat-monitor.violation"
fi

# Repository stats -----------------------------------------------------------

repo_size_kb()   { du -sk "$GIT_DIR" 2>/dev/null | awk '{print $1}'; }
git_field()      { git count-objects -v 2>/dev/null | awk -F': ' -v k="$1" '$1 == k {print $2; exit}'; }
loose_count()    { git_field "count"; }
loose_size_kb()  { git_field "size"; }
pack_count()     { git_field "packs"; }
garbage_kb()     { git_field "size-garbage"; }

# RSS of a process plus all of its descendants, in KB.
# Walks /proc directly so the full git tree (repack -> pack-objects -> ...)
# is covered regardless of depth.
tree_rss_kb() {
  local root="$1"
  cat /proc/[0-9]*/stat 2>/dev/null | awk -v root="$root" -v page="$PAGE_SIZE" '
    {
      pid = $1
      # comm (field 2, inside parens) may itself contain ")" — find the LAST
      # paren within the first 64 chars (comm is capped at 15 chars).
      close_paren = 0
      limit = length($0); if (limit > 64) limit = 64
      for (i = 1; i <= limit; i++)
        if (substr($0, i, 1) == ")") close_paren = i
      rest = substr($0, close_paren + 2)
      split(rest, f, " ")
      ppid = f[2]        # f[1] is state
      rss_pages = f[22]  # overall field 24 (rss) minus the 2 fields we dropped
      parent[pid] = ppid
      size[pid] = rss_pages
    }
    END {
      total = 0
      for (p in size) {
        in_tree = (p + 0 == root + 0)
        q = p; hops = 0
        while (!in_tree && q != "" && q + 0 != 0 && hops < 4096) {
          q = parent[q]
          if (q + 0 == root + 0) in_tree = 1
          hops++
        }
        if (in_tree) total += size[p]
      }
      printf "%d\n", total * page / 1024
    }'
}

# ---------------------------------------------------------------------------
# Launch bounds
# ---------------------------------------------------------------------------
#
# Every bounded git invocation gets two layers on top of git's own
# pack.windowMemory / deltaCacheSize limits:
#
#   1. A hard ceiling (audit gap 6): when the systemd user manager is
#      reachable, the command runs inside a transient scope with MemoryMax set
#      to the same cap the RSS guard uses, so the kernel enforces the bound
#      continuously and an intra-interval RSS spike cannot overshoot between
#      samples. Without it the sampler is best-effort polling.
#   2. A killable unit (audit gap 5): the command runs in its own session /
#      process group (setsid), so the guard and the interrupt handler signal
#      the whole tree — repack's pack-objects child included — instead of
#      killing the root pid and orphaning the rest.
#
# The RSS sampler stays in the loop under both: it provides peak-RSS
# accounting and aborts early, before the kernel has to.

SETSID_OK=0
if command -v setsid >/dev/null 2>&1; then SETSID_OK=1; fi

CGROUP_STATE=""   # "" unresolved | yes | no

resolve_cgroup() {
  if [[ -n "$CGROUP_STATE" ]]; then return 0; fi
  if [[ "$CGROUP_MODE" == "never" ]]; then CGROUP_STATE=no; return 0; fi
  if ! command -v systemd-run >/dev/null 2>&1 || [[ -z "${XDG_RUNTIME_DIR:-}" ]]; then
    CGROUP_STATE=no
  elif ! systemd-run --user --quiet --scope -p MemoryMax=32M true 2>/dev/null; then
    CGROUP_STATE=no
  else
    CGROUP_STATE=yes
  fi
  if [[ "$CGROUP_STATE" == "yes" ]]; then
    log "memory bound: hard systemd user scope (MemoryMax=${RSS_CAP_MB}M) + RSS guard at the same cap"
  else
    log "memory bound: RSS guard only (systemd user scope unavailable)"
    if [[ "$CGROUP_MODE" == "always" ]]; then
      log_error "CLEANUP_BLOAT_CGROUP=always requires a reachable systemd user manager"
      exit 2
    fi
  fi
}

launch_bg() {
  # Launch $* as a bounded background child; its pid lands in $LAUNCHED_PID.
  LAUNCHED_PID=""
  local -a wrap=()
  if [[ "$CGROUP_STATE" == "yes" ]]; then
    wrap=(systemd-run --user --quiet --scope -p "MemoryMax=${RSS_CAP_MB}M")
  fi
  if (( SETSID_OK == 1 )); then
    setsid "${wrap[@]}" "$@" &
  else
    "${wrap[@]}" "$@" &
  fi
  LAUNCHED_PID=$!
}

kill_tree() {
  # $1 = root pid, $2 = signal name. Signal the whole process group when the
  # child was launched into its own group; fall back to the bare pid when it
  # was not (setsid unavailable), so the call never becomes a no-op.
  local pid="$1" sig="$2"
  if (( SETSID_OK == 1 )) && kill "-$sig" -- "-$pid" 2>/dev/null; then
    return 0
  fi
  kill "-$sig" "$pid" 2>/dev/null || true
}

# ---------------------------------------------------------------------------
# Memory monitor
# ---------------------------------------------------------------------------

MONITOR_PID=""
GIT_PID=""
CURRENT_STAGE=""

start_monitor() {
  # $1 = pid to watch, $2 = cap KB, $3 = interval seconds
  rm -f "$VIOLATION_FILE"
  (
    target="$1"
    cap_kb="$2"
    interval="$3"
    peak=0
    n=0
    start="$(date +%s)"
    while kill -0 "$target" 2>/dev/null; do
      rss="$(tree_rss_kb "$target")"
      if (( rss > peak )); then peak="$rss"; fi
      n=$((n + 1))
      elapsed=$(( $(date +%s) - start ))
      printf 'stage=%s elapsed=%ss rss_kb=%s peak_kb=%s samples=%s\n' \
        "$CURRENT_STAGE" "$elapsed" "$rss" "$peak" "$n" > "$STATS_FILE"
      if [[ "$VERBOSE" == true ]] || (( n % 10 == 0 )); then
        log "  [monitor] stage=${CURRENT_STAGE:-?} elapsed=${elapsed}s rss=$(human_kb "$rss") peak=$(human_kb "$peak")"
      fi
      if (( cap_kb > 0 && rss > cap_kb )); then
        log_error "MEMORY CAP EXCEEDED during '${CURRENT_STAGE:-?}': rss $(human_kb "$rss") > cap $(human_kb "$cap_kb")"
        log_error "Terminating the git process group (root pid $target) before the kernel OOM killer can"
        printf 'cap_kb=%s peak_kb=%s rss_kb=%s stage=%s\n' \
          "$cap_kb" "$peak" "$rss" "$CURRENT_STAGE" > "$VIOLATION_FILE"
        kill_tree "$target" TERM
        sleep 2
        kill_tree "$target" KILL
        break
      fi
      sleep "$interval"
    done
  ) &
  MONITOR_PID=$!
}

stop_monitor() {
  if [[ -n "$MONITOR_PID" ]]; then
    kill "$MONITOR_PID" 2>/dev/null || true
    wait "$MONITOR_PID" 2>/dev/null || true
    MONITOR_PID=""
  fi
}

monitor_peak_kb() {
  if [[ -f "$STATS_FILE" ]]; then
    awk -F'peak_kb=' '/peak_kb=/{print $2}' "$STATS_FILE" | tail -1 | awk '{print $1 + 0}'
  else
    echo 0
  fi
}

# ---------------------------------------------------------------------------
# Checkpoint (plain KEY=VALUE; parsed with grep/cut — never sourced)
# ---------------------------------------------------------------------------

# Records that the integrity gate was skipped, for the next checkpoint write:
# a skipped corruption gate must be visible in the post-mortem artifact, not
# only in the run log (audit gap 3).
GATE_NOTE=""
note_gate_skip() {
  GATE_NOTE="${GATE_NOTE:+$GATE_NOTE; }integrity gate skipped: $1"
}

ckpt_write() {
  # $1 = last_completed_stage, $2 = status, $3 = message
  local stage="$1" status="$2" msg="${3:-}"
  cat > "$CKPT_FILE" <<EOF
version=1
updated=$(date -Iseconds)
mode=$MODE
status=$status
last_completed_stage=$stage
message=$msg${GATE_NOTE:+; $GATE_NOTE}
repo_size_kb=$(repo_size_kb)
peak_rss_kb=$(monitor_peak_kb)
EOF
  log_verbose "checkpoint: last_completed=$stage status=$status"
}

ckpt_field() {
  local key="$1" def="${2:-}"
  if [[ -f "$CKPT_FILE" ]]; then
    grep -m1 "^${key}=" "$CKPT_FILE" 2>/dev/null | cut -d= -f2- || true
  else
    echo "$def"
  fi
}

# ---------------------------------------------------------------------------
# Locking (prevents concurrent cleanup / concurrent git gc)
# ---------------------------------------------------------------------------

acquire_lock() {
  if [[ -d "$LOCK_DIR" ]]; then
    local other
    other="$(cat "$LOCK_DIR/pid" 2>/dev/null || echo '?')"
    if [[ -n "$other" && "$other" != "?" ]] && kill -0 "$other" 2>/dev/null; then
      log_error "Another cleanup-bloat run is active (pid $other). Aborting."
      exit 1
    fi
    log_warn "Removing stale cleanup lock (pid ${other:-?} is gone)"
    rm -rf "$LOCK_DIR"
  fi
  if [[ -f "$GIT_DIR/gc.pid" ]]; then
    local gcpid
    gcpid="$(cut -d' ' -f1 "$GIT_DIR/gc.pid" 2>/dev/null || echo '?')"
    if [[ -n "$gcpid" && "$gcpid" != "?" ]] && kill -0 "$gcpid" 2>/dev/null; then
      log_error "git gc is already running (pid $gcpid, $GIT_DIR/gc.pid). Aborting."
      exit 1
    fi
    if [[ "$FORCE" == true ]]; then
      log_warn "Removing stale git gc lock (pid ${gcpid:-?} is gone)"
      rm -f "$GIT_DIR/gc.pid"
    else
      log_error "Stale git gc lock present ($GIT_DIR/gc.pid, pid ${gcpid:-?} is gone)."
      log_error "Re-run with --force to clear it."
      exit 1
    fi
  fi
  mkdir -p "$LOCK_DIR"
  echo "$$" > "$LOCK_DIR/pid"
}

release_lock() {
  if [[ -d "$LOCK_DIR" && "$(cat "$LOCK_DIR/pid" 2>/dev/null)" == "$$" ]]; then
    rm -rf "$LOCK_DIR"
  fi
}

# ---------------------------------------------------------------------------
# Pre-flight checks
# ---------------------------------------------------------------------------

# git fsck under the same layered bounds as the stages (audit gap 3): the
# hard MemoryMax scope when available, a process group the guard can kill as
# a unit, the RSS sampler, and the time bound. Echoes fsck's rc; a guard kill
# is visible afterwards as a non-empty $VIOLATION_FILE, the peak as
# $FSCK_PEAK_KB.
FSCK_PEAK_KB=0
monitored_fsck() {
  local rc=0 pid
  FSCK_PEAK_KB=0
  rm -f "$STATS_FILE" "$VIOLATION_FILE"
  CURRENT_STAGE="fsck"
  set +e
  launch_bg timeout "$FSCK_TIMEOUT" git fsck --connectivity-only --no-progress >>"$LOG_FILE" 2>&1
  pid="$LAUNCHED_PID"
  GIT_PID="$pid"
  start_monitor "$pid" "$((RSS_CAP_MB * 1024))" "$MONITOR_INTERVAL"
  wait "$pid"
  rc=$?
  set -e
  stop_monitor
  GIT_PID=""
  FSCK_PEAK_KB="$(monitor_peak_kb)"
  CURRENT_STAGE=""
  return "$rc"
}

preflight() {
  # --check-only reports resource state but must never fail on it: its exit
  # code means "cleanup needed / not needed", and a monitoring wrapper seeing
  # exit 1 would wrongly conclude the repository is healthy.
  local enforce=true
  if [[ "$CHECK_ONLY" == true ]]; then enforce=false; fi

  log "Running pre-flight checks..."

  # 1. Disk: absolute floor AND headroom over the current repo size
  #    (a repack temporarily needs roughly the repo size again)
  local avail_disk repo_kb need_disk
  avail_disk="$(df -BM --output=avail "$REPO_ROOT" 2>/dev/null | tail -1 | tr -dc '0-9')"
  repo_kb="$(repo_size_kb)"
  repo_kb="${repo_kb:-0}"
  if ! [[ "$avail_disk" =~ ^[0-9]+$ ]]; then
    log_error "Pre-flight failed: could not determine free disk space on $REPO_ROOT"
    return 1
  fi
  need_disk=$(( repo_kb * DISK_HEADROOM / 1024 ))
  if (( need_disk < MIN_DISK_MB )); then need_disk="$MIN_DISK_MB"; fi
  log "  disk: ${avail_disk}MB available, ${need_disk}MB required (repo $(human_kb "$repo_kb"))"
  if (( avail_disk < need_disk )); then
    if [[ "$enforce" == true ]]; then
      log_error "Pre-flight failed: insufficient disk (${avail_disk}MB available < ${need_disk}MB required)"
      return 1
    fi
    log_warn "insufficient disk (${avail_disk}MB < ${need_disk}MB) — a real run would abort here"
  fi

  # 2. Available memory
  local avail_mem
  avail_mem="$(free -m | awk '/^Mem:/{print $7}')"
  avail_mem="${avail_mem:-0}"
  log "  memory: ${avail_mem}MB available, ${MIN_AVAIL_MB}MB required"
  if (( avail_mem < MIN_AVAIL_MB )); then
    if [[ "$enforce" == true ]]; then
      log_error "Pre-flight failed: insufficient memory (${avail_mem}MB available < ${MIN_AVAIL_MB}MB required)"
      log_error "Free memory before retrying — running gc under memory pressure is how bf-1s6c3 crashed."
      return 1
    fi
    log_warn "insufficient memory (${avail_mem}MB < ${MIN_AVAIL_MB}MB) — a real run would abort here"
  fi

  # 3. Load average (1-minute)
  local load1
  load1="$(cut -d' ' -f1 /proc/loadavg)"
  log "  load: ${load1} (max ${MAX_LOAD})"
  if awk -v a="$load1" -v b="$MAX_LOAD" 'BEGIN { exit !(a + 0 > b + 0) }'; then
    if [[ "$enforce" == true ]]; then
      log_error "Pre-flight failed: load average ${load1} exceeds ${MAX_LOAD}"
      return 1
    fi
    log_warn "load average ${load1} > ${MAX_LOAD} — a real run would abort here"
  fi

  if [[ "$enforce" != true ]]; then
    log_ok "Resource state recorded (check-only — gates not enforced)"
    return 0
  fi

  # 4. Repository integrity — bounded in memory as well as time (audit gap 3):
  #    fsck ignores pack.windowMemory and this script's target scenario — the
  #    bloated repository — is exactly where its RSS peaks, so the gate must
  #    not itself be the run's first memcg-OOM victim. A skip is a warning
  #    (recorded in the checkpoint via GATE_NOTE), real corruption is fatal:
  #    never prune a corrupt repository.
  if (( FSCK_MAX_REPO_MB > 0 )); then
    local repo_mb=$(( repo_kb / 1024 ))
    if (( repo_mb > FSCK_MAX_REPO_MB )); then
      log_warn "fsck skipped: repo ${repo_mb}MB > CLEANUP_BLOAT_FSCK_MAX_REPO_MB=${FSCK_MAX_REPO_MB}MB — continuing without the integrity gate"
      note_gate_skip "repo ${repo_mb}MB exceeds fsck ceiling ${FSCK_MAX_REPO_MB}MB"
      log_ok "Pre-flight checks passed (integrity gate skipped)"
      return 0
    fi
  fi
  log "  integrity: git fsck --connectivity-only (timeout ${FSCK_TIMEOUT}s, rss cap ${RSS_CAP_MB}MB)"
  local rc=0
  rc="$(monitored_fsck)" || rc=$?
  if [[ -s "$VIOLATION_FILE" ]]; then
    log_warn "fsck exceeded the ${RSS_CAP_MB}MB RSS cap (peak $(human_kb "$FSCK_PEAK_KB")) — the guard killed it; continuing without the integrity gate"
    note_gate_skip "fsck killed at the ${RSS_CAP_MB}MB rss cap"
  elif (( rc == 124 )); then
    log_warn "fsck exceeded ${FSCK_TIMEOUT}s — continuing without the integrity gate (very large repo)"
    note_gate_skip "fsck timeout after ${FSCK_TIMEOUT}s"
  elif (( rc == 137 )); then
    # SIGKILL from outside the guard: the MemoryMax scope or the kernel's OOM
    # killer got there first. That is a resource event, not corruption.
    log_warn "fsck was SIGKILLed (MemoryMax ceiling or kernel OOM killer) — continuing without the integrity gate"
    note_gate_skip "fsck SIGKILLed (resource ceiling)"
  elif (( rc != 0 )); then
    log_error "Pre-flight failed: repository corruption detected (git fsck rc=$rc)"
    log_error "See $LOG_FILE — do NOT prune a corrupt repository."
    return 1
  else
    log_ok "Repository integrity verified"
  fi

  log_ok "Pre-flight checks passed"
}

# ---------------------------------------------------------------------------
# Is cleanup needed?
# ---------------------------------------------------------------------------

bloat_stats_line() {
  echo "loose=$(loose_count) objects / $(human_kb "$(loose_size_kb)"), packs=$(pack_count), garbage=$(human_kb "$(garbage_kb)"), repo=$(human_kb "$(repo_size_kb)")"
}

cleanup_needed() {
  local loose lsize packs garbage
  loose="$(loose_count)";   loose="${loose:-0}"
  lsize="$(loose_size_kb)"; lsize="${lsize:-0}"
  packs="$(pack_count)";    packs="${packs:-0}"
  garbage="$(garbage_kb)";  garbage="${garbage:-0}"

  if (( loose > 100 )); then    echo "loose object count $loose > 100";                   return 0; fi
  if (( lsize > 102400 )); then echo "loose objects total $(human_kb "$lsize") > 100M";   return 0; fi
  if (( packs > 10 )); then     echo "pack fragmentation ($packs packs > 10)";            return 0; fi
  if (( garbage > 0 )); then    echo "garbage files present ($(human_kb "$garbage"))";    return 0; fi
  return 1
}

# ---------------------------------------------------------------------------
# Staged execution
# ---------------------------------------------------------------------------

GIT_MEM_ARGS=(git)

git_mem_args() {
  GIT_MEM_ARGS=(git
    -c "pack.windowMemory=${MEMORY_MAX_MB}m"
    -c "pack.deltaCacheSize=${DELTA_CACHE_MB}m"
    -c "pack.threads=${PACK_THREADS}")
}

# Run a command under the memory monitor.
# Returns 0 on success, 125 if the memory guard killed it, else the command's rc.
run_monitored() {
  local stage="$1"; shift
  CURRENT_STAGE="$stage"

  local size_before t0 rc
  size_before="$(repo_size_kb)"
  t0="$(date +%s)"
  log "── stage: $stage ──"
  log "  started $(date '+%H:%M:%S'), repo size $(human_kb "$size_before")"
  log_verbose "exec: $*"

  ckpt_write "$LAST_COMPLETED" "running" "stage $stage in progress"

  rm -f "$STATS_FILE" "$VIOLATION_FILE"
  set +e
  launch_bg "$@" >>"$LOG_FILE" 2>&1
  GIT_PID="$LAUNCHED_PID"
  start_monitor "$GIT_PID" "$((RSS_CAP_MB * 1024))" "$MONITOR_INTERVAL"
  wait "$GIT_PID"
  rc=$?
  set -e
  stop_monitor
  GIT_PID=""

  local elapsed size_after peak
  elapsed=$(( $(date +%s) - t0 ))
  size_after="$(repo_size_kb)"
  peak="$(monitor_peak_kb)"

  if [[ -s "$VIOLATION_FILE" ]]; then
    ckpt_write "$LAST_COMPLETED" "failed" "memory cap exceeded during $stage"
    log_error "Stage '$stage' aborted by the memory guard (peak $(human_kb "$peak"))."
    log_error "The repository is intact — the git process group was terminated cleanly."
    log_error "Options: raise the limit (--memory-max / CLEANUP_BLOAT_MEMORY_MAX_MB)"
    log_error "or free system memory, then re-run with --resume to retry this stage."
    CURRENT_STAGE=""
    return 125
  fi

  if (( rc != 0 )); then
    ckpt_write "$LAST_COMPLETED" "failed" "stage $stage exited rc=$rc"
    log_error "Stage '$stage' failed (rc=$rc) after ${elapsed}s. See $LOG_FILE."
    if (( rc == 137 )); then
      log_error "rc=137 is SIGKILL — with the systemd scope active this was the MemoryMax ceiling."
      log_error "Raise --memory-max / CLEANUP_BLOAT_MEMORY_MAX_MB and re-run with --resume."
    else
      log_error "Fix the cause and re-run with --resume."
    fi
    CURRENT_STAGE=""
    return "$rc"
  fi

  ckpt_write "$stage" "stage_complete" "stage $stage ok in ${elapsed}s"
  LAST_COMPLETED="$stage"
  log_ok "stage '$stage' completed in ${elapsed}s (size $(human_kb "$size_before") -> $(human_kb "$size_after"), peak rss $(human_kb "$peak"))"
  CURRENT_STAGE=""
  return 0
}

stage_pack_refs() {
  run_monitored "pack-refs" "${GIT_MEM_ARGS[@]}" pack-refs --all --prune
}

stage_reflog() {
  log_warn "Expiring all reflogs — unreferenced commits become unreachable and will be pruned."
  log_warn "(Use --conservative to keep reflogs and reclaim less space.)"
  run_monitored "reflog-expire" git reflog expire --expire=now --all
}

stage_repack() {
  local -a cmd=("${GIT_MEM_ARGS[@]}" repack -a -d "--window-memory=${MEMORY_MAX_MB}m")
  if [[ "$MODE" == "aggressive" ]]; then
    # depth 50 / window 250 mirrors what --aggressive does internally
    cmd+=(-f --depth=50 --window=250)
  else
    cmd+=(-f --depth=50 --window=100)
  fi
  run_monitored "repack" "${cmd[@]}"
}

stage_prune() {
  local expire="now"
  if [[ "$MODE" == "conservative" ]]; then
    expire="2.weeks.ago"
  fi
  run_monitored "prune" git prune "--expire=$expire" || return $?
  run_monitored "prune-packed" git prune-packed
}

stage_verify() {
  log "Verifying repository integrity after cleanup..."
  local rc=0
  rc="$(monitored_fsck)" || rc=$?
  if [[ -s "$VIOLATION_FILE" ]]; then
    log_warn "post-cleanup fsck exceeded the ${RSS_CAP_MB}MB RSS cap (peak $(human_kb "$FSCK_PEAK_KB")) — skipped"
    ckpt_write "verify" "stage_complete" "post-cleanup fsck skipped (memory guard)"
    LAST_COMPLETED="verify"
    return 0
  elif (( rc == 124 )); then
    log_warn "post-cleanup fsck exceeded ${FSCK_TIMEOUT}s — skipped"
    ckpt_write "verify" "stage_complete" "post-cleanup fsck skipped (timeout)"
    LAST_COMPLETED="verify"
    return 0
  elif (( rc == 137 )); then
    log_warn "post-cleanup fsck was SIGKILLed (MemoryMax ceiling or kernel OOM killer) — skipped"
    ckpt_write "verify" "stage_complete" "post-cleanup fsck skipped (SIGKILL at resource ceiling)"
    LAST_COMPLETED="verify"
    return 0
  elif (( rc != 0 )); then
    log_error "Repository integrity check FAILED after cleanup (rc=$rc). See $LOG_FILE."
    ckpt_write "$LAST_COMPLETED" "failed" "post-cleanup fsck failed rc=$rc"
    return 1
  fi
  ckpt_write "verify" "stage_complete" "post-cleanup fsck ok"
  LAST_COMPLETED="verify"
  log_ok "Post-cleanup integrity verified"
  log "after: $(bloat_stats_line)"
}

# Interrupted repacks can leave temporary pack files behind; safe to remove
# once we hold the lock and verified no other gc is running.
clean_stale_tmp() {
  local found
  found="$(find "$GIT_DIR/objects/pack" -maxdepth 1 -type f \( -name '*.tmp' -o -name 'tmp_pack_*' \) 2>/dev/null || true)"
  if [[ -n "$found" ]]; then
    log_warn "Removing stale temporary pack files from a previous interrupted run:"
    echo "$found" | sed 's/^/    /' | tee -a "$LOG_FILE"
    find "$GIT_DIR/objects/pack" -maxdepth 1 -type f \( -name '*.tmp' -o -name 'tmp_pack_*' \) -delete 2>/dev/null || true
  fi
}

# ---------------------------------------------------------------------------
# Monitor self-test (no git operations — proves the OOM guard actually kills)
# ---------------------------------------------------------------------------

selftest_monitor() {
  if ! command -v python3 >/dev/null 2>&1; then
    log_error "self-test requires python3"
    return 1
  fi
  CURRENT_STAGE="selftest"
  log "Self-test: spawning a 256MB RSS hog with a child process; expecting the guard to kill the group at a 96MB cap"

  rm -f "$STATS_FILE" "$VIOLATION_FILE"
  # Launched directly (never through the cgroup wrap): this path exists to
  # prove the sampler + guard. setsid gives the hog its own process group so
  # the guard's group kill is exercised against the child too (audit gap 5).
  local hog_cmd=(python3 -c 'import subprocess, time
child = subprocess.Popen(["sleep", "60"])
b = bytearray(256 * 1024 * 1024)
b[0] = 1
time.sleep(60)')
  if (( SETSID_OK == 1 )); then
    setsid "${hog_cmd[@]}" &
  else
    "${hog_cmd[@]}" &
  fi
  local hog=$!
  start_monitor "$hog" $((96 * 1024)) 1

  # Capture the hog's children while it is alive so their death can be
  # asserted after the kill.
  local child_pids="" c
  sleep 1
  child_pids="$(cat "/proc/$hog/task/$hog/children" 2>/dev/null || true)"

  local waited=0
  while (( waited < 30 )); do
    if [[ -s "$VIOLATION_FILE" ]]; then break; fi
    if ! kill -0 "$hog" 2>/dev/null; then break; fi
    sleep 1
    waited=$((waited + 1))
  done
  stop_monitor

  local peak
  peak="$(monitor_peak_kb)"
  if [[ -s "$VIOLATION_FILE" ]] && (( peak >= 96 * 1024 )); then
    if (( SETSID_OK == 1 )) && [[ -n "$child_pids" ]]; then
      sleep 1
      for c in $child_pids; do
        if kill -0 "$c" 2>/dev/null; then
          log_error "Self-test FAILED: hog child $c survived the kill (orphaned)"
          CURRENT_STAGE=""
          return 1
        fi
      done
      log_ok "hog children also terminated ($child_pids) — process-group kill confirmed"
    fi
    log_ok "Self-test passed: guard fired at peak $(human_kb "$peak") and terminated the process group"
    sed 's/^/    /' "$VIOLATION_FILE" | tee -a "$LOG_FILE"
    CURRENT_STAGE=""
    return 0
  fi
  log_error "Self-test FAILED: guard did not trip (peak $(human_kb "$peak"))"
  CURRENT_STAGE=""
  return 1
}

# ---------------------------------------------------------------------------
# Signals / exit
# ---------------------------------------------------------------------------

on_interrupt() {
  log_error "Interrupted during stage: ${CURRENT_STAGE:-none}"
  if [[ -n "$CURRENT_STAGE" ]]; then
    ckpt_write "$LAST_COMPLETED" "interrupted" "interrupted during $CURRENT_STAGE"
  fi
  if [[ -n "$GIT_PID" ]]; then
    # Signal the whole process group: killing only the root pid would orphan
    # repack's pack-objects child (audit gap 5).
    kill_tree "$GIT_PID" TERM
  fi
  stop_monitor
  exit 130
}

on_exit() {
  stop_monitor
  release_lock
  if [[ -n "$SELFTEST_TMP" ]]; then
    rm -rf "$SELFTEST_TMP"
  fi
}
trap on_interrupt INT TERM
trap on_exit EXIT

# ---------------------------------------------------------------------------
# Main
# ---------------------------------------------------------------------------

main() {
  if [[ "$SELFTEST" != true ]]; then
    # Rotate the previous run's log aside instead of truncating it — after a
    # crash, the failed run's log is the primary post-mortem artifact. The
    # self-test logs into its throwaway directory instead (audit gap 4).
    if [[ -s "$LOG_FILE" ]]; then
      mv -f "$LOG_FILE" "$LOG_FILE.1" 2>/dev/null || true
    fi
    : > "$LOG_FILE" 2>/dev/null || true
  fi

  if [[ "$SELFTEST" == true ]]; then
    selftest_monitor
    return
  fi

  # Decide the launch bounds once, before any git work (the hard MemoryMax
  # scope needs a reachable systemd user manager; --check-only never launches
  # anything, so it stays maximally passive and skips the probe).
  if [[ "$CHECK_ONLY" != true ]]; then
    resolve_cgroup
  fi

  log "=== Repository Bloat Cleanup Started ==="
  log "mode=$MODE memory_max=${MEMORY_MAX_MB}MB rss_kill_cap=${RSS_CAP_MB}MB threads=$PACK_THREADS monitor_interval=${MONITOR_INTERVAL}s"
  if [[ "$MODE" == "conservative" ]]; then
    log "policy: conservative — reflogs preserved, only objects unreachable for 2+ weeks are pruned"
  else
    log "policy: aggressive — reflogs expired, all unreachable objects pruned immediately"
  fi
  log "repo: $REPO_ROOT"
  local size_at_start
  size_at_start="$(repo_size_kb)"
  size_at_start="${size_at_start:-0}"
  log "before: $(bloat_stats_line)"

  # --check-only never mutates anything and must work regardless of a
  # concurrently running cleanup or gc (same rule as safe-git-gc.sh). Its
  # exit code has to reflect repository state only — a lock conflict must not
  # be mistaken for "cleanup not needed".
  if [[ "$CHECK_ONLY" != true ]]; then
    acquire_lock
  fi

  if ! preflight; then
    log_error "Aborted at pre-flight. Nothing was modified."
    exit 1
  fi

  # --check-only: report and exit (0 = needed, 1 = not needed)
  if [[ "$CHECK_ONLY" == true ]]; then
    local reason
    if reason="$(cleanup_needed)"; then
      log_ok "Cleanup needed: $reason"
      exit 0
    else
      log_ok "Cleanup not needed ($(bloat_stats_line))"
      exit 1
    fi
  fi

  local needed_reason
  if needed_reason="$(cleanup_needed)"; then
    log "Cleanup indicated: $needed_reason"
  else
    if [[ "$FORCE" == true ]]; then
      log_warn "Bloat heuristics report nothing to do — continuing (--force)"
    else
      log_ok "Cleanup not needed ($(bloat_stats_line)). Use --force to override."
      # Deliberately no checkpoint write: an existing checkpoint (e.g. from a
      # failed run awaiting --resume) is post-mortem evidence — don't clobber
      # it just because today's heuristics find nothing to do.
      exit 0
    fi
  fi

  if [[ "$DRY_RUN" == true ]]; then
    log "Dry run — the following stages WOULD execute:"
    log "  1. pack-refs     : git -c pack.windowMemory=... pack-refs --all --prune"
    if [[ "$MODE" == "aggressive" ]]; then
      log "  2. reflog-expire : git reflog expire --expire=now --all"
    fi
    log "  3. repack        : git -c pack.threads=$PACK_THREADS repack -a -d -f --window-memory=${MEMORY_MAX_MB}m"
    log "  4. prune         : git prune --expire=$( [[ "$MODE" == "aggressive" ]] && echo now || echo 2.weeks.ago ), then git prune-packed"
    log "  5. verify        : git fsck --connectivity-only + final stats"
    log "Memory guard: kill the git process group if RSS exceeds $(human_kb "$((RSS_CAP_MB * 1024))")."
    if [[ "$CGROUP_STATE" == "yes" ]]; then
      log "Hard ceiling: each stage runs in a transient systemd scope with MemoryMax=$(human_kb "$((RSS_CAP_MB * 1024))")."
    else
      log "Hard ceiling: none (systemd user scope unavailable) — the RSS guard is the only bound."
    fi
    log_ok "Dry run complete — repository untouched."
    exit 0
  fi

  # Resume handling ----------------------------------------------------------
  local -a STAGES=(pack-refs reflog repack prune verify)
  if [[ "$MODE" == "conservative" ]]; then
    STAGES=(pack-refs repack prune verify)
  fi

  LAST_COMPLETED="$(ckpt_field last_completed_stage "none")"
  local prev_status prev_mode start_idx=0
  prev_status="$(ckpt_field status "none")"
  prev_mode="$(ckpt_field mode "$MODE")"

  # A completed checkpoint never blocks a new run: the bloat heuristics above
  # are the single source of truth for whether work is needed. If they report
  # fresh bloat, a "complete" recorded weeks ago must not turn this run into
  # a silent exit-0 no-op.
  if [[ "$prev_status" == "complete" ]]; then
    log "Previous run completed ($(ckpt_field message '')) — heuristics report fresh bloat; starting a new run."
  fi
  if [[ "$prev_status" != "none" && "$prev_mode" != "$MODE" ]]; then
    log_warn "Previous run used mode=$prev_mode, this run uses mode=$MODE (stages are idempotent; reflog/prune aggressiveness follows the current mode)."
  fi
  if [[ "$prev_status" != "none" && "$prev_status" != "complete" ]]; then
    if [[ "$RESUME" == true ]]; then
      local i
      for i in "${!STAGES[@]}"; do
        if [[ "${STAGES[$i]}" == "$LAST_COMPLETED" ]]; then
          start_idx=$((i + 1))
          break
        fi
      done
      log "Resuming after stage '$LAST_COMPLETED' (starting at index $start_idx)"
    else
      log_warn "Found incomplete previous run (status=$prev_status, last completed: $LAST_COMPLETED)."
      log_warn "Stages are idempotent — re-running from the start. Use --resume to skip completed stages."
    fi
  fi

  clean_stale_tmp
  git_mem_args

  local idx stage_name rc=0
  for idx in "${!STAGES[@]}"; do
    if (( idx < start_idx )); then
      log "  skipping completed stage '${STAGES[$idx]}'"
      continue
    fi
    stage_name="${STAGES[$idx]}"
    rc=0
    case "$stage_name" in
      pack-refs) stage_pack_refs || rc=$? ;;
      reflog)    stage_reflog   || rc=$? ;;
      repack)    stage_repack   || rc=$? ;;
      prune)     stage_prune    || rc=$? ;;
      verify)    stage_verify   || rc=$? ;;
    esac
    if (( rc != 0 )); then
      log_error "Aborted at stage '$stage_name' (rc=$rc)."
      log_error "Progress is checkpointed in $CKPT_FILE — re-run with --resume once resolved."
      exit "$rc"
    fi
  done

  ckpt_write "all" "complete" "cleanup finished"
  log "after: $(bloat_stats_line)"

  local after saved
  after="$(repo_size_kb)"
  saved=$(( size_at_start - after ))
  if (( saved > 0 )); then
    log_ok "Reclaimed $(human_kb "$saved") (repo $(human_kb "$size_at_start") -> $(human_kb "$after"))"
  else
    log_ok "Repository size unchanged ($(human_kb "$after")); object store consolidated."
  fi
  log_ok "=== Repository Bloat Cleanup Completed ==="
}

main "$@"
