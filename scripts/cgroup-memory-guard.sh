#!/usr/bin/env bash
# Cgroup memory guard - check the *dispatch scope's own* memcg headroom before
# memory-heavy work, and optionally gate a command behind that check.
#
# Why this exists: the signal -1 crash root cause (bf-4x12ec, bf-173o7e — see
# docs/research/root-cause-analysis-signal-minus-one-crashes.md) is memcg OOM
# against a cgroup limit, not system-wide memory exhaustion. A NEEDLE worker
# runs in `run-*.scope` with MemoryMax=12G under `needle.slice` (32G), and the
# kernel kills when the *tightest bounded cgroup in the ancestry* runs out of
# headroom. `free -g` and /proc/pressure/memory are system-wide and can look
# healthy while the scope or the slice is about to be OOM-killed. This script
# reads the caller's own cgroup tree and evaluates every bounded level.
#
# Usage:
#   scripts/cgroup-memory-guard.sh --check                 # report + exit code
#   scripts/cgroup-memory-guard.sh --json                  # machine-readable
#   scripts/cgroup-memory-guard.sh <command> [args...]     # gate, then exec
#
# Exit codes:
#   0  pass                 - headroom OK at every bounded level
#   1  warn                 - degraded; proceed with caution
#   2  refuse               - insufficient headroom; command NOT run
#   3  unknown              - cgroup state unreadable; fail-open with warning
#                             (use --strict to turn unknown into refuse)
#
# Environment overrides (all optional):
#   MEMGUARD_MIN_HEADROOM   bytes of headroom required at every bounded level
#                           (default 2g; accepts K/M/G suffixes)
#   MEMGUARD_WARN_PCT       leaf usage %% that triggers warn (default 70)
#   MEMGUARD_MAX_USED_PCT   leaf usage %% that triggers refuse (default 85)
#   MEMGUARD_CGROUP_ROOT    cgroupfs mount (default /sys/fs/cgroup; tests point
#                           this at a fixture tree)
#   MEMGUARD_PROC_CGROUP    map file (default /proc/self/cgroup)
#   MEMGUARD_SYS_AVAIL_MIN  system-wide MemAvailable floor for the unbounded
#                           fallback, in bytes (default 10g, matching the
#                           pre-task resource check in CLAUDE.md)
#
# scripts/test-cgroup-memory-guard.sh exercises the decision matrix via
# fixture trees, plus a live check and a live low-MemoryMax scope.

set -uo pipefail

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

CGROUP_ROOT="${MEMGUARD_CGROUP_ROOT:-/sys/fs/cgroup}"
PROC_CGROUP="${MEMGUARD_PROC_CGROUP:-/proc/self/cgroup}"
MIN_HEADROOM="${MEMGUARD_MIN_HEADROOM:-2g}"
WARN_PCT="${MEMGUARD_WARN_PCT:-70}"
MAX_USED_PCT="${MEMGUARD_MAX_USED_PCT:-85}"
SYS_AVAIL_MIN="${MEMGUARD_SYS_AVAIL_MIN:-10g}"

MODE="check"      # check | json | exec | report-and-exec
STRICT=false
CMD=()

usage() {
  sed -n '2,40p' "$0" | sed 's/^# \{0,1\}//'
  exit 0
}

while [[ $# -gt 0 ]]; do
  case "$1" in
    --check)    MODE="check"; shift ;;
    --json)     MODE="json"; shift ;;
    --strict)   STRICT=true; shift ;;
    --quiet)    exec 1>/dev/null; shift ;;
    --min-headroom) MIN_HEADROOM="${2:?--min-headroom needs a value}"; shift 2 ;;
    --warn-pct)     WARN_PCT="${2:?--warn-pct needs a value}"; shift 2 ;;
    --max-used-pct) MAX_USED_PCT="${2:?--max-used-pct needs a value}"; shift 2 ;;
    --help|-h)  usage ;;
    --)         shift; CMD=("$@"); break ;;
    -*)
      echo "Unknown option: $1" >&2
      exit 3
      ;;
    *)
      CMD=("$@")
      break
      ;;
  esac
done
[[ ${#CMD[@]} -gt 0 ]] && MODE="exec"

log()  { [[ "$MODE" == "json" ]] && return 0; echo -e "${BLUE}[memguard]${NC} $*"; }
warn() { [[ "$MODE" == "json" ]] && return 0; echo -e "${YELLOW}[memguard]${NC} $*"; }
die()  { [[ "$MODE" == "json" ]] && return 0; echo -e "${RED}[memguard]${NC} $*"; }

# to_bytes 2g -> 2147483648
to_bytes() {
  local v="$1"
  case "$v" in
    *[Kk]) echo $(( ${v%[Kk]} * 1024 )) ;;
    *[Mm]) echo $(( ${v%[Mm]} * 1048576 )) ;;
    *[Gg]) echo $(( ${v%[Gg]} * 1073741824 )) ;;
    *)     echo "$v" ;;
  esac
}

MIN_HEADROOM_B="$(to_bytes "$MIN_HEADROOM")"
WARN_HEADROOM_B=$(( MIN_HEADROOM_B * 2 ))
SYS_AVAIL_MIN_B="$(to_bytes "$SYS_AVAIL_MIN")"

human() {
  local b="$1"
  if (( b >= 1073741824 )); then
    awk -v b="$b" 'BEGIN{printf "%.1fG", b/1073741824}'
  elif (( b >= 1048576 )); then
    awk -v b="$b" 'BEGIN{printf "%.0fM", b/1048576}'
  else
    echo "${b}B"
  fi
}

# Returns "0:<current>" if the file holds a numeric byte count, "1:max" style
# marker for unlimited, "2:" if unreadable.
read_bytes_file() {
  local f="$1" v
  [[ -r "$f" ]] || { echo "2:"; return; }
  v="$(head -c 64 "$f" | tr -d '[:space:]')"
  case "$v" in
    max) echo "1:" ;;
    ''|*[!0-9]*) echo "2:" ;;
    *) echo "0:$v" ;;
  esac
}

# Emit one level's numbers on stdout as a pipe-delimited record:
# depth|path|max_or_-1|current|used_pct_or_-1|headroom_or_-1|oom_kills
scan_level() {
  local path="$1" depth="$2" dir max_r cur_r ev
  dir="${CGROUP_ROOT}${path}"
  max_r="$(read_bytes_file "$dir/memory.max")"
  cur_r="$(read_bytes_file "$dir/memory.current")"
  [[ "${max_r%%:*}" == "0" && "${cur_r%%:*}" == "0" ]] || return 1

  local max="${max_r#*:}" cur="${cur_r#*:}"
  local used_pct=-1 headroom=-1
  headroom=$(( max - cur ))
  (( max > 0 )) && used_pct=$(( cur * 100 / max ))

  local oom_kills=0
  if [[ -r "$dir/memory.events" ]]; then
    ev="$(grep -E '^oom_kill [0-9]+$' "$dir/memory.events" | awk '{print $2}')"
    [[ -n "$ev" ]] && oom_kills="$ev"
  fi

  echo "$depth|$path|$max|$cur|$used_pct|$headroom|$oom_kills"
  return 0
}

# Collect the caller's ancestry, leaf first. Fills the LEVELS array.
declare -a LEVELS=()
collect_levels() {
  local line path
  line="$(grep '^0::' "$PROC_CGROUP" 2>/dev/null | head -1)" || true
  if [[ -z "$line" ]]; then
    # cgroup v1 or unreadable map file -> unknown state
    return 1
  fi
  path="${line#0::}"
  local depth=0
  while true; do
    local rec
    rec="$(scan_level "$path" "$depth")" || break   # stop at unbounded/unreadable levels
    LEVELS+=("$rec")
    [[ "$path" == "/" || -z "${path%/}" ]] && break
    path="$(dirname "$path")"
    [[ "$path" == "." ]] && path="/"
    depth=$((depth + 1))
  done
  return 0
}

# Decision: the binding constraint is the bounded level with the least
# headroom — an ancestor's memory.current includes every sibling scope, so a
# nearly-full slice OOM-kills us even when our own scope is nearly empty.
evaluate() {
  local min_headroom="" binding="" leaf_used=-1 total_oom=0 lvl
  for lvl in "${LEVELS[@]}"; do
    IFS='|' read -r depth path max cur used_pct headroom oom_kills <<< "$lvl"
    [[ "$depth" == "0" ]] && leaf_used="$used_pct"
    total_oom=$(( total_oom + oom_kills ))
    if [[ -z "$min_headroom" || "$headroom" -lt "$min_headroom" ]]; then
      min_headroom="$headroom"
      binding="$path"
    fi
  done

  DECISION="pass"
  REASON="all bounded levels have headroom >= $(human "$MIN_HEADROOM_B")"

  if [[ -n "$min_headroom" ]]; then
    if (( min_headroom < MIN_HEADROOM_B )); then
      DECISION="refuse"
      REASON="only $(human "$min_headroom") headroom at $binding (need $(human "$MIN_HEADROOM_B"))"
    elif (( leaf_used >= MAX_USED_PCT )); then
      DECISION="refuse"
      REASON="this scope is ${leaf_used}% of its own limit (>= ${MAX_USED_PCT}%)"
    elif (( min_headroom < WARN_HEADROOM_B )); then
      DECISION="warn"
      REASON="only $(human "$min_headroom") headroom at $binding (warn below $(human "$WARN_HEADROOM_B"))"
    elif (( leaf_used >= WARN_PCT )); then
      DECISION="warn"
      REASON="this scope is ${leaf_used}% of its own limit (>= ${WARN_PCT}%)"
    fi
  else
    # No bounded level found (all "max") -> fall back to system-wide memory,
    # which is the only meaningful signal in an unbounded tree.
    local avail
    if [[ -r /proc/meminfo ]] && avail="$(grep -E '^MemAvailable:' /proc/meminfo | awk '{print $2}')"; then
      avail=$(( avail * 1024 ))
      min_headroom="$avail"
      binding="system MemAvailable (no bounded cgroup found)"
      if (( avail < SYS_AVAIL_MIN_B )); then
        DECISION="refuse"
        REASON="unbounded cgroup tree and MemAvailable $(human "$avail") < $(human "$SYS_AVAIL_MIN_B")"
      elif (( avail < SYS_AVAIL_MIN_B * 2 )); then
        DECISION="warn"
        REASON="unbounded cgroup tree; MemAvailable $(human "$avail") below 2x floor"
      else
        REASON="unbounded cgroup tree; MemAvailable $(human "$avail") OK"
      fi
    fi
  fi

  MIN_HEADROOM_FOUND="${min_headroom:--1}"
  BINDING_LEVEL="$binding"
  LEAF_USED_PCT="$leaf_used"
  TOTAL_OOM_KILLS="$total_oom"
}

report() {
  log "cgroup memory headroom (leaf -> root):"
  local lvl
  for lvl in "${LEVELS[@]}"; do
    IFS='|' read -r depth path max cur used_pct headroom oom_kills <<< "$lvl"
    log "  $(printf '%-70s' "$path") max=$(human "$max") used=$(human "$cur") (${used_pct}%) headroom=$(human "$headroom") oom_kills=$oom_kills"
  done
  [[ ${#LEVELS[@]} -eq 0 ]] && log "  (no bounded levels found)"

  case "$DECISION" in
    pass)   log "$(echo -e "${GREEN}PASS${NC}") — $REASON" ;;
    warn)   warn "$(echo -e "${YELLOW}WARN${NC}") — $REASON" ;;
    refuse) die  "$(echo -e "${RED}REFUSE${NC}") — $REASON" ;;
  esac
  (( TOTAL_OOM_KILLS > 0 )) && log "note: $TOTAL_OOM_KILLS lifetime memcg oom_kill(s) recorded in this tree"
}

json_report() {
  local out lvl levels=""
  for lvl in "${LEVELS[@]}"; do
    IFS='|' read -r depth path max cur used_pct headroom oom_kills <<< "$lvl"
    levels+="${levels:+,}{\"path\":\"$path\",\"max\":$max,\"current\":$cur,\"used_pct\":$used_pct,\"headroom\":$headroom,\"oom_kills\":$oom_kills}"
  done
  printf '{"decision":"%s","reason":"%s","min_headroom":%s,"binding_level":"%s","leaf_used_pct":%s,"oom_kills":%s,"levels":[%s]}\n' \
    "$DECISION" "$REASON" "$MIN_HEADROOM_FOUND" "$BINDING_LEVEL" "$LEAF_USED_PCT" "$TOTAL_OOM_KILLS" "$levels"
}

DECISION="unknown"
REASON="cgroup state unreadable (no v2 unified map line at $PROC_CGROUP)"
MIN_HEADROOM_FOUND=-1
BINDING_LEVEL="unknown"
LEAF_USED_PCT=-1
TOTAL_OOM_KILLS=0

if collect_levels; then
  evaluate
else
  if [[ "$STRICT" == true ]]; then
    DECISION="refuse"
    REASON="$REASON (--strict)"
  fi
fi

case "$MODE" in
  json)
    json_report
    ;;
  check)
    report
    [[ "$DECISION" == "refuse" ]] && exit 2
    [[ "$DECISION" == "warn" ]] && exit 1
    [[ "$DECISION" == "unknown" ]] && { warn "proceeding despite unknown state (fail-open; use --strict to refuse)"; exit 3; }
    exit 0
    ;;
  exec)
    if [[ "$DECISION" == "refuse" ]]; then
      report
      die "refusing to run: ${CMD[*]}"
      exit 2
    fi
    [[ "$MODE" != "json" ]] && report
    exec "${CMD[@]}"
    ;;
esac
