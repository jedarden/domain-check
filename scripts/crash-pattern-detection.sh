#!/usr/bin/env bash
# Crash Pattern Detection Script
# Purpose: Detect systematic crash patterns and infrastructure events
# Usage: ./scripts/crash-pattern-detection.sh [--alert] [--since <timeframe>]
#
# Exit codes: 0 = normal (or degraded: stale data), 1 = elevated rate,
#             2 = infrastructure event (storm). The systemd unit treats exit 1
#             as success (SuccessExitStatus=1) — only a storm fails the unit.
#
# Fix history (bead domchk-0c601026, 2026-09-06):
#   - CRASH_COUNT counted every crash event in the file regardless of the
#     --since window, so after the source went stale (last event Aug 26) the
#     monitor logged "ELEVATED CRASH RATE: 247 crashes in 1hour" and exited 1
#     on every 10-minute run. Events are now filtered to the window first.
#   - The surge check said "N crashes in 5 minutes" but measured --since
#     (SYSTEM_EVENT_WINDOW was defined and never used). It now measures the
#     documented 5-minute window.
#   - Added the workspace-aggregate storm-rate signal recommended by
#     docs/crash-investigations/bf-4yjq-crash-investigation.md §9 (≥10
#     crashes/hour sustained) — per-bead thresholds fire far too late in a
#     multi-bead storm like 2026-08-12 (455 events across 6 beads).
#   - A source with no events inside the analysis window is now reported as
#     DEGRADED and exits 0: a monitor that cannot see must say so rather than
#     false-alarm on old data.
#   - EVENTS_FILE can be overridden via the environment for testing.
#
# Fix history (bead domchk-f49dfe47, 2026-09-06):
#   - DEGRADED keyed on crash events alone, which reads a healthy workspace as
#     blind: events.jsonl records claim/dispatch/complete/fail/timeout too,
#     and the live source has fresh records of those every minute. Zero recent
#     crash events on a live source means "no crashes" (STABLE); DEGRADED is
#     now reserved for a source with no events of ANY kind in the window.
#   - --quiet left the per-row exit-code and worker tables visible (raw printf
#     in the loops); they now go through say() so --quiet output carries only
#     alert-class lines.

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
ALERT_LOG="$PROJECT_ROOT/.beads/logs/crash-pattern-alerts.log"
EVENTS_FILE="${EVENTS_FILE:-$PROJECT_ROOT/.beads/events.jsonl}"

# Thresholds
CRASH_SURGE_THRESHOLD=3        # crashes in 5 minutes = infrastructure event (early detection)
HIGH_CRASH_RATE_THRESHOLD=5    # crashes in the analysis window = elevated
SYSTEM_EVENT_WINDOW="5minutes" # time window for surge detection (reduced from 10min)
STORM_RATE_PER_HOUR=10         # sustained workspace-wide crashes/hour = infrastructure event

# Defaults
ALERT_MODE=false
SINCE_TIME="24hours"
VERBOSE=false
QUIET=false

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --alert)
      ALERT_MODE=true
      shift
      ;;
    --since)
      SINCE_TIME="$2"
      shift 2
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
      echo "Usage: $0 [--alert] [--since <timeframe>] [--quiet] [--verbose]"
      echo "  --alert    Generate alert if patterns detected"
      echo "  --since    Time window to analyze (default: 24hours)"
      echo "  --quiet    Suppress informational output; keep alerts"
      echo "  --verbose  Show detailed analysis"
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      exit 1
      ;;
  esac
done

say() { [[ "$QUIET" == true ]] || echo "$@"; }

# Alert-class output: always printed, even under --quiet. A storm must reach
# the log whatever the invocation.
warn() { echo "$@"; }

# Logging function
log_alert() {
  local message="$1"
  local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
  echo "[$timestamp] $message" | tee -a "$ALERT_LOG"
}

# Verbose output
verbose() {
  if [[ "$VERBOSE" == true ]]; then
    echo "$@"
  fi
}

# Emit an ISO-8601 UTC cutoff ("YYYY-MM-DDTHH:MM:SSZ") for a GNU-date
# duration like "24hours", or an empty string if the duration cannot be parsed.
iso_cutoff() {
  date -u -d "$1 ago" +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || echo ""
}

# Seconds in a GNU-date duration, or empty on parse failure.
duration_seconds() {
  local now cutoff
  now=$(date -u +%s)
  cutoff=$(date -u -d "$1 ago" +%s 2>/dev/null || echo "")
  [[ -n "$cutoff" ]] || { echo ""; return; }
  echo $((now - cutoff))
}

# jq filter: normalize a record's .ts to "YYYY-MM-DDTHH:MM:SSZ" so it can be
# compared lexicographically with the cutoff. events.jsonl stamps
# "2026-08-16T04:27:36.261347993+00:00" — the fractional seconds and the
# +00:00 offset both break a naive string compare against a Z-suffixed cutoff.
readonly TS_NORMALIZE='(.ts | sub("\\.[0-9]+"; "") | sub("\\+00:00$"; "Z"))'

# Check if events file exists
if [[ ! -f "$EVENTS_FILE" ]]; then
  echo "ERROR: Events file not found: $EVENTS_FILE"
  exit 1
fi

WINDOW_CUTOFF="$(iso_cutoff "$SINCE_TIME")"
if [[ -z "$WINDOW_CUTOFF" ]]; then
  echo "ERROR: cannot parse time window '$SINCE_TIME'"
  exit 1
fi

if [[ "$QUIET" != true ]]; then
  echo "=== Crash Pattern Detection ==="
  echo "Time Window: $SINCE_TIME (since $WINDOW_CUTOFF)"
  echo "Analysis Date: $(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  echo
fi

# All crash events ever recorded in the source (grep exits 1 on no match).
ALL_CRASHES=$(grep -i '"event":"crash"' "$EVENTS_FILE" 2>/dev/null || true)
TOTAL_EVENTS=$(printf '%s' "$ALL_CRASHES" | grep -c . || true)

# Crash events inside the analysis window.
WINDOW_CRASHES=""
if [[ $TOTAL_EVENTS -gt 0 ]]; then
  WINDOW_CRASHES=$(printf '%s\n' "$ALL_CRASHES" \
    | jq -rc --arg cutoff "$WINDOW_CUTOFF" "select(($TS_NORMALIZE) >= \$cutoff)" 2>/dev/null || true)
fi
CRASH_COUNT=$(printf '%s' "$WINDOW_CRASHES" | grep -c . || true)

# Source heartbeat: the newest event of ANY type in the file. events.jsonl
# records claim/dispatch/complete/fail/timeout alongside crash, so this — not
# the newest crash — distinguishes a live source from a dead one. Freshness
# computed from crash events alone reads a healthy workspace as blind.
readonly ANY_TS_NORMALIZE='(.ts // empty | sub("\\.[0-9]+"; "") | sub("\\+00:00$"; "Z"))'
NEWEST_ANY=$(jq -r "$ANY_TS_NORMALIZE" "$EVENTS_FILE" 2>/dev/null | sort | tail -1)

if [[ $CRASH_COUNT -eq 0 ]]; then
  if [[ -n "$NEWEST_ANY" && "$NEWEST_ANY" < "$WINDOW_CUTOFF" ]]; then
    NEWEST_CRASH="none on file"
    if [[ $TOTAL_EVENTS -gt 0 ]]; then
      NEWEST_CRASH=$(printf '%s\n' "$ALL_CRASHES" \
        | jq -r "$TS_NORMALIZE" 2>/dev/null | sort | tail -1)
    fi
    warn "⚠️  DEGRADED: crash event source is stale"
    warn "   $EVENTS_FILE: newest event of any kind $NEWEST_ANY — nothing at"
    warn "   all within the last $SINCE_TIME, so detection is blind"
    warn "   (newest crash event: $NEWEST_CRASH; see beads domchk-0c601026,"
    warn "   domchk-f49dfe47)."
    exit 0
  fi
  say "✅ No crashes detected in the last $SINCE_TIME"
  say "System Status: STABLE"
  exit 0
fi

say "Total Crashes (last $SINCE_TIME): $CRASH_COUNT"
say

# Analyze crashes by exit code
say "### Crash Classification by Exit Code"
printf '%s\n' "$WINDOW_CRASHES" | jq -r '.exit_code' | sort | uniq -c | sort -rn | while read -r count exit_code; do
  classification=""
  case $exit_code in
    -1)
      classification="Infrastructure (SIGKILL/SIGHUP)"
      ;;
    1)
      classification="Application Error"
      ;;
    137)
      classification="OOM Killer (128+9)"
      ;;
    *)
      classification="Unknown"
      ;;
  esac
  say "$(printf "  Exit Code %3s: %3d crashes - %s\n" "$exit_code" "$count" "$classification")"
done
say

# Analyze by worker
say "### Crash Distribution by Worker"
printf '%s\n' "$WINDOW_CRASHES" | jq -r '.worker' | sort | uniq -c | sort -rn | while read -r count worker; do
  percentage=$((count * 100 / CRASH_COUNT))
  say "$(printf "  %20s: %3d crashes (%2d%%)\n" "$worker" "$count" "$percentage")"
done
say

# Infrastructure signals: either a tight burst (surge) or a sustained
# workspace-wide rate (storm). Both mean "triage the environment, not a bead".
IS_INFRASTRUCTURE=false

# Detect crash surge (infrastructure event)
verbose "Checking for crash surge pattern..."
SURGE_CUTOFF="$(iso_cutoff "$SYSTEM_EVENT_WINDOW")"
SURGE_CRASHES=$(printf '%s\n' "$WINDOW_CRASHES" \
  | jq -rc --arg cutoff "$SURGE_CUTOFF" "select(($TS_NORMALIZE) >= \$cutoff)" 2>/dev/null | grep -c . || true)

if [[ $SURGE_CRASHES -ge $CRASH_SURGE_THRESHOLD ]]; then
  IS_INFRASTRUCTURE=true
  warn "⚠️  INFRASTRUCTURE EVENT DETECTED"
  warn "   $SURGE_CRASHES crashes in the last $SYSTEM_EVENT_WINDOW"
  warn "   This indicates a system-wide event (OOM, SIGHUP cascade, etc.)"

  if [[ "$ALERT_MODE" == true ]]; then
    log_alert "INFRASTRUCTURE EVENT: $SURGE_CRASHES crashes in $SYSTEM_EVENT_WINDOW (threshold: $CRASH_SURGE_THRESHOLD)"
  fi
fi

# Detect sustained workspace-wide storm rate (bf-4yjq investigation §9 rec 1)
WINDOW_SECONDS="$(duration_seconds "$SINCE_TIME")"
WINDOW_HOURS=$(( WINDOW_SECONDS / 3600 ))
[[ $WINDOW_HOURS -ge 1 ]] || WINDOW_HOURS=1
STORM_THRESHOLD=$(( WINDOW_HOURS * STORM_RATE_PER_HOUR ))

if [[ $CRASH_COUNT -ge $STORM_THRESHOLD ]]; then
  IS_INFRASTRUCTURE=true
  warn "⚠️  WORKSPACE STORM DETECTED"
  warn "   $CRASH_COUNT crashes in the last $SINCE_TIME (≥ $STORM_RATE_PER_HOUR/hour sustained)"
  warn "   Multi-bead resource-exhaustion regime — triage repo size, memory and"
  warn "   load at the environment level before any per-bead debugging."

  if [[ "$ALERT_MODE" == true ]]; then
    log_alert "WORKSPACE STORM: $CRASH_COUNT crashes in $SINCE_TIME (threshold: $STORM_THRESHOLD)"
  fi
fi

# Detect high crash rate (informational — below storm level)
verbose "Checking for elevated crash rate..."
if [[ $CRASH_COUNT -ge $HIGH_CRASH_RATE_THRESHOLD ]]; then
  warn "⚠️  ELEVATED CRASH RATE"
  warn "   $CRASH_COUNT crashes in last $SINCE_TIME"
  warn "   Monitoring recommended"

  if [[ "$ALERT_MODE" == true ]] && [[ "$IS_INFRASTRUCTURE" == false ]]; then
    log_alert "ELEVATED CRASH RATE: $CRASH_COUNT crashes in $SINCE_TIME"
  fi
fi

# Analyze temporal patterns (detect simultaneous crashes)
verbose "Analyzing temporal patterns..."
say "### Temporal Clustering"
printf '%s\n' "$WINDOW_CRASHES" | jq -r '.ts' | cut -d'T' -f2 | cut -d':' -f1 | sort | uniq -c | sort -rn | head -5 | while read -r count hour; do
  if [[ $count -gt 3 ]]; then
    say "  Hour $hour: $count crashes (clustered pattern)"
  fi
done

# Check for duplicate alerts (same crash investigated multiple times)
verbose "Checking for duplicate investigation patterns..."
DUPLICATE_THRESHOLD=3
printf '%s\n' "$WINDOW_CRASHES" | jq -r '.bead' | sort | uniq -c | sort -rn | while read -r count bead_id; do
  if [[ $count -ge $DUPLICATE_THRESHOLD ]]; then
    say "⚠️  DUPLICATE ALERT PATTERN: bead $bead_id crashed $count times"
    say "   This may indicate retry loops or lack of deduplication"
  fi
done

say
say "=== Analysis Complete ==="

# Exit codes
if [[ "$IS_INFRASTRUCTURE" == true ]]; then
  exit 2  # Infrastructure event
elif [[ $CRASH_COUNT -ge $HIGH_CRASH_RATE_THRESHOLD ]]; then
  exit 1  # Elevated rate
else
  exit 0  # Normal
fi
