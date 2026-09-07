#!/usr/bin/env bash
# System Event Mode — the crash-surge gate (defer new load while an event is active)
#
# Purpose: answer ONE question cheaply, for any caller that is about to add load
# to this box (a dispatch, a preflight, a bulk job, a new investigation/alert
# bead): "is a system-wide crash/memory event active right now?"
#
# Why: detection existed (crash-pattern-detection.sh, resource-monitor.sh) but
# nothing consumed it — during the 2026-08-16 cascade (177 crashes / 59 beads /
# 5 h) the correct action was to STOP ADDING LOAD, and nothing did that. Alert
# beads fanned out onto the same bloated repo under the same 12 GiB ceiling and
# each kill raised pressure on every survivor (57 of 59 window-crashing beads
# were ALERT beads). See docs/crash-root-cause-bf-3561g.md §6 recommendation 5
# ("alert amplification is structural, not fixed") and
# docs/crash-prevention-requirements.md gap G-3. This script is the missing
# response leg: a queryable gate plus a deferral convention.
#
# Usage:
#   scripts/system-event-mode.sh check [--quiet]   # exit 0 clear, 75 defer
#   scripts/system-event-mode.sh status            # human-readable state
#   scripts/system-event-mode.sh json              # machine-readable state
#   scripts/system-event-mode.sh on --reason "..." [--ttl SECONDS]
#   scripts/system-event-mode.sh off               # clear manual latch
#   scripts/system-event-mode.sh alert-gate <bead-id>
#                                                  # exit 0 = may create alert
#                                                  # exit 4 = suppress (coalesced)
#
# Exit codes:
#   0  clear — no system event (alert-gate: alert may proceed)
#   4  alert-gate only — this event window already recorded an alert; suppress
#   75 EX_TEMPFAIL — event active: defer new work (alert-gate never returns 75)
#   2  usage error / missing argument
#
# Trigger conditions (any one latches an event; all thresholds were measured
# against 3 days of steady-state data in this workspace, not guessed):
#   crash_burst      >= CRASH_BURST_THRESHOLD exit -1 "crash" events within
#                    SURGE_WINDOW seconds. Kernel deaths are categorically
#                    system-relevant, so no per-bead spread is required. The
#                    bf-3561g class.
#   fail_wave        >= FAIL_WAVE_THRESHOLD abnormal terminations ("fail"
#                    exit_code!=0, "timeout" 124, "crash" -1) within
#                    SURGE_WINDOW seconds across >= MIN_WAVE_BEADS distinct
#                    beads — the modern synchronized-exit wave signature.
#   sustained_storm  >= STORM_HOURLY_THRESHOLD abnormal terminations in the
#                    last hour across >= MIN_WAVE_BEADS distinct beads.
#   memory_pressure  /proc/pressure/memory "some avg60" >= PRESSURE_THRESHOLD
#                    percent — the backdrop of every memcg-OOM cascade
#                    (94.29% on 2026-08-16).
#   manual           an operator/agent latched it via "on" (TTL-bounded).
#
# Division of labor (do not blur these):
#   crash-circuit-breaker.sh  per-BEAD retry storms (one bead dying N times)
#   crash-pattern-detection.sh  scheduled detection + alert log (10-min timer)
#   resource-monitor.sh       scheduled pressure readings (5-min timer)
#   memory-watch.sh           aborts ONE command before the kernel kills its scope
#   THIS SCRIPT               system-wide gate: defer new load, coalesce alerts
#
# Staleness: if the event source's newest record is older than
# SOURCE_MAX_AGE_SECONDS the gate says so (DEGRADED) rather than silently
# answering "clear" while blind. Default warns only; set STRICT_SOURCE=1 to
# make a blind source defer (exit 75).
#
# Environment overrides (for testing and fleet-wide tuning):
#   EVENTS_FILE, PSI_FILE, STATE_DIR, EVENT_LOG, and every threshold below.
#
# Created: 2026-09-06 (bead domchk-d06cb3e6, remediation for bf-3561g)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# --- Configuration (env-overridable) -----------------------------------------
EVENTS_FILE="${EVENTS_FILE:-$PROJECT_ROOT/.beads/events.jsonl}"
PSI_FILE="${PSI_FILE:-/proc/pressure/memory}"
STATE_DIR="${STATE_DIR:-$PROJECT_ROOT/.beads/state/system-event}"
EVENT_LOG="${EVENT_LOG:-$PROJECT_ROOT/.beads/logs/system-event.log}"

SURGE_WINDOW="${SURGE_WINDOW:-300}"            # seconds (the canon 5-minute surge window)
CRASH_BURST_THRESHOLD="${CRASH_BURST_THRESHOLD:-3}"
FAIL_WAVE_THRESHOLD="${FAIL_WAVE_THRESHOLD:-5}"
MIN_WAVE_BEADS="${MIN_WAVE_BEADS:-3}"
STORM_HOURLY_THRESHOLD="${STORM_HOURLY_THRESHOLD:-25}"
PRESSURE_THRESHOLD="${PRESSURE_THRESHOLD:-70}" # PSI "some avg60" percent
HOLD_SECONDS="${HOLD_SECONDS:-900}"            # stay active this long after the last trigger
MANUAL_TTL="${MANUAL_TTL:-1800}"               # default lifetime of a manual latch
SOURCE_MAX_AGE_SECONDS="${SOURCE_MAX_AGE_SECONDS:-86400}"
STRICT_SOURCE="${STRICT_SOURCE:-0}"            # 1 = a blind source defers (exit 75)

AUTO_STATE="$STATE_DIR/state.json"
MANUAL_STATE="$STATE_DIR/manual.json"
ALERT_LEDGER="$STATE_DIR/alert-ledger.jsonl"

EV_TMP="$(mktemp "${TMPDIR:-/tmp}/system-event-XXXXXX")"
trap 'rm -f "$EV_TMP"' EXIT

# --- Helpers ------------------------------------------------------------------
usage() {
  sed -n '3,30p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'
}

log_event() { # $1 = message
  mkdir -p "$(dirname "$EVENT_LOG")"
  printf '%s %s\n' "$(date -u +%Y-%m-%dT%H:%M:%SZ)" "$1" >>"$EVENT_LOG"
}

state_field() { # $1 = file, $2 = key -> value (empty when absent)
  [[ -f "$1" ]] || return 0
  jq -r --arg k "$2" 'if (has($k) and .[$k] != null) then (.[$k] | tostring) else "" end' "$1" 2>/dev/null || true
}

psi_some_avg60() { # -> float percent, empty when unreadable
  [[ -r "$PSI_FILE" ]] || return 0
  local raw
  raw=$(awk '/^some / {for (i=1; i<=NF; i++) if ($i ~ /^avg60=/) {sub(/^avg60=/, "", $i); print $i; exit}}' "$PSI_FILE" 2>/dev/null)
  printf '%s' "$raw"
}

# Extract abnormal terminations as "epoch event exit_code bead" TSV.
# Abnormal = event "crash" (exit -1, the needle sentinel for a signal death),
# "fail" with a nonzero exit_code, or "timeout" (124).
load_events() {
  : >"$EV_TMP"
  [[ -f "$EVENTS_FILE" ]] || return 0
  jq -r '
    select(.ts != null)
    | select(.event == "crash" or .event == "fail" or .event == "timeout")
    | select((.exit_code // 0) != 0)
    | [(.ts | gsub("\\.[0-9]+"; "") | gsub("\\+00:00$"; "Z") | fromdateiso8601),
       .event, (.exit_code // -1), (.bead // "?")] | @tsv' \
    "$EVENTS_FILE" 2>/dev/null >"$EV_TMP" || true
}

# Source liveness is measured over ALL records (dispatch/claim/complete
# included), not just abnormal ones: a healthy, active workspace writes no
# abnormal events, and must not read as a blind source.
load_source_freshness() {
  NEWEST_EPOCH=0
  [[ -f "$EVENTS_FILE" ]] || return 0
  NEWEST_EPOCH=$(jq -r 'select(.ts != null) |
    (.ts | gsub("\\.[0-9]+"; "") | gsub("\\+00:00$"; "Z") | fromdateiso8601)' \
    "$EVENTS_FILE" 2>/dev/null | sort -n | tail -1)
  [[ -n "$NEWEST_EPOCH" ]] || NEWEST_EPOCH=0
}

# Summarise $EV_TMP into the four numbers the triggers need. NEWEST_EPOCH is
# NOT touched here: load_source_freshness owns it, measured over all records
# (a superset of these) — recomputing it from abnormal events only would read
# a healthy workspace as a blind source.
#   CRASH_5M ABNORMAL_5M BEADS_5M ABNORMAL_1H BEADS_1H
summarise() {
  local w5=$((NOW - SURGE_WINDOW)) w1=$((NOW - 3600))
  local row
  CRASH_5M=0; ABNORMAL_5M=0; BEADS_5M=0; ABNORMAL_1H=0; BEADS_1H=0
  local -A b5=() b1=()
  while IFS=$'\t' read -r ep ev code bead; do
    [[ -n "$ep" ]] || continue
    if ((ep >= w5)); then
      ((ABNORMAL_5M += 1))
      [[ "$ev" == "crash" ]] && ((CRASH_5M += 1))
      b5["$bead"]=1
    fi
    if ((ep >= w1)); then
      ((ABNORMAL_1H += 1))
      b1["$bead"]=1
    fi
  done <"$EV_TMP"
  BEADS_5M=${#b5[@]}
  BEADS_1H=${#b1[@]}
}

# Evaluate triggers -> TRIGGERS (newline list of "kind:detail") and KIND.
evaluate_triggers() {
  TRIGGERS=()
  KIND=""
  if ((CRASH_5M >= CRASH_BURST_THRESHOLD)); then
    KIND="crash_burst"
    TRIGGERS+=("crash_burst:${CRASH_5M} exit--1 crash events in ${SURGE_WINDOW}s")
  fi
  local psi
  psi=$(psi_some_avg60)
  PSI_READING="$psi"
  if [[ -n "$psi" ]]; then
    local hot
    hot=$(awk -v p="$psi" -v t="$PRESSURE_THRESHOLD" 'BEGIN {print (p + 0 >= t + 0) ? 1 : 0}')
    if ((hot)); then
      [[ -z "$KIND" ]] && KIND="memory_pressure"
      TRIGGERS+=("memory_pressure:PSI some avg60=${psi}% (threshold ${PRESSURE_THRESHOLD}%)")
    fi
  fi
  if ((ABNORMAL_5M >= FAIL_WAVE_THRESHOLD && BEADS_5M >= MIN_WAVE_BEADS)); then
    [[ -z "$KIND" ]] && KIND="fail_wave"
    TRIGGERS+=("fail_wave:${ABNORMAL_5M} abnormal terminations in ${SURGE_WINDOW}s across ${BEADS_5M} beads")
  fi
  if ((ABNORMAL_1H >= STORM_HOURLY_THRESHOLD && BEADS_1H >= MIN_WAVE_BEADS)); then
    [[ -z "$KIND" ]] && KIND="sustained_storm"
    TRIGGERS+=("sustained_storm:${ABNORMAL_1H} abnormal terminations in 1h across ${BEADS_1H} beads")
  fi
}

# True when the newest source record is older than the staleness bound.
source_is_stale() {
  ((NEWEST_EPOCH == 0 || NEWEST_EPOCH < NOW - SOURCE_MAX_AGE_SECONDS))
}

manual_active() {
  [[ -f "$MANUAL_STATE" ]] || return 1
  local until
  until=$(state_field "$MANUAL_STATE" "expires_at")
  [[ -n "$until" ]] || return 1
  ((NOW < until))
}

# An expired manual latch is removed rather than left on disk, so "status"
# does not report a latch that no longer defers anything.
clear_expired_manual() {
  [[ -f "$MANUAL_STATE" ]] || return 0
  local until id
  until=$(state_field "$MANUAL_STATE" "expires_at")
  [[ -n "$until" ]] || return 0
  if ((NOW >= until)); then
    id=$(state_field "$MANUAL_STATE" "event_id")
    log_event "MANUAL EXPIRED id=manual-${id} ttl_end=${until}"
    rm -f "$MANUAL_STATE"
  fi
}

# Write/refresh the auto-latched event. Keeps the ORIGINAL event_id while the
# event is ongoing so alert-gate coalescing spans the whole event, not each
# poll that re-triggers it.
latch_or_refresh() {
  mkdir -p "$STATE_DIR"
  local existing_id existing_latched existing_kind
  existing_id=$(state_field "$AUTO_STATE" "event_id")
  existing_latched=$(state_field "$AUTO_STATE" "latched_at")
  existing_kind=$(state_field "$AUTO_STATE" "kind")
  local held
  held=0
  if [[ -n "$existing_id" && -n "$existing_latched" ]]; then
    local prev_hold
    prev_hold=$(state_field "$AUTO_STATE" "hold_until")
    if [[ -n "$prev_hold" && $NOW -lt $prev_hold ]]; then
      held=1
    fi
  fi
  local id latched
  if ((held)); then
    id="$existing_id"; latched="$existing_latched"
    log_event "EVENT ONGOING id=${id} kind=${KIND} triggers=\"${TRIGGERS[*]}\""
  else
    id="$NOW"; latched="$NOW"
    log_event "EVENT LATCHED id=${id} kind=${KIND} triggers=\"${TRIGGERS[*]}\""
  fi
  local hold_until=$((NOW + HOLD_SECONDS))
  printf '{"event_id": %s, "kind": "%s", "latched_at": %s, "hold_until": %s, "updated_at": %s, "reasons": "%s", "crash_5m": %s, "abnormal_5m": %s, "beads_5m": %s, "abnormal_1h": %s}\n' \
    "$id" "$KIND" "$latched" "$hold_until" "$NOW" \
    "$(printf '%s; ' "${TRIGGERS[@]}" | sed 's/; $//; s/\\/\\\\/g; s/"/\\"/g')" \
    "$CRASH_5M" "$ABNORMAL_5M" "$BEADS_5M" "$ABNORMAL_1H" >"$AUTO_STATE"
  EVENT_ID="$id"
  LATCHED_AT="$latched"
  HOLD_UNTIL="$hold_until"
}

clear_expired_auto() {
  [[ -f "$AUTO_STATE" ]] || return 0
  local hold kind id latched
  hold=$(state_field "$AUTO_STATE" "hold_until")
  kind=$(state_field "$AUTO_STATE" "kind")
  id=$(state_field "$AUTO_STATE" "event_id")
  latched=$(state_field "$AUTO_STATE" "latched_at")
  # Same boundary as manual_active: the hold is live while NOW < hold_until,
  # so at exactly hold_until the event has served its full hold and clears.
  if [[ -n "$hold" && $NOW -ge $hold ]]; then
    local dur=0
    [[ -n "$latched" ]] && dur=$((NOW - latched))
    log_event "EVENT CLEARED id=${id} kind=${kind} duration_seconds=${dur}"
    rm -f "$AUTO_STATE"
  fi
}

# Resolve the current mode: sets ACTIVE (0/1), EVENT_ID, KIND, ACTIVE_REASON,
# LATCHED_AT, HOLD_UNTIL.
resolve() {
  # Initialise every evaluation global BEFORE any early return: the manual
  # and STRICT_SOURCE paths return without reading the event source, and an
  # unset NEWEST_EPOCH/counters would abort under set -u (exit 1, not 75).
  ACTIVE=0; EVENT_ID=""; KIND=""; ACTIVE_REASON=""; LATCHED_AT=""; HOLD_UNTIL=""
  NEWEST_EPOCH=0; CRASH_5M=0; ABNORMAL_5M=0; BEADS_5M=0; ABNORMAL_1H=0; BEADS_1H=0
  PSI_READING=""; TRIGGERS=()
  clear_expired_auto
  clear_expired_manual
  if manual_active; then
    ACTIVE=1
    KIND="manual"
    EVENT_ID="manual-$(state_field "$MANUAL_STATE" "event_id")"
    ACTIVE_REASON=$(state_field "$MANUAL_STATE" "reason")
    LATCHED_AT=$(state_field "$MANUAL_STATE" "latched_at")
    HOLD_UNTIL=$(state_field "$MANUAL_STATE" "expires_at")
    return 0
  fi
  load_events
  load_source_freshness
  summarise
  evaluate_triggers
  if source_is_stale && ((STRICT_SOURCE)); then
    ACTIVE=1
    KIND="source_stale"
    EVENT_ID="stale-$NOW"
    ACTIVE_REASON="crash event source blind: newest record ${NEWEST_EPOCH:-0} (STRICT_SOURCE=1 defers)"
    return 0
  fi
  if ((${#TRIGGERS[@]} > 0)); then
    latch_or_refresh
    ACTIVE=1
    ACTIVE_REASON="$(printf '%s; ' "${TRIGGERS[@]}" | sed 's/; $//')"
    return 0
  fi
  if [[ -f "$AUTO_STATE" ]]; then
    ACTIVE=1
    KIND=$(state_field "$AUTO_STATE" "kind")
    EVENT_ID=$(state_field "$AUTO_STATE" "event_id")
    LATCHED_AT=$(state_field "$AUTO_STATE" "latched_at")
    HOLD_UNTIL=$(state_field "$AUTO_STATE" "hold_until")
    ACTIVE_REASON="hold period after earlier trigger (kind=${KIND}, latched ${LATCHED_AT})"
    return 0
  fi
}

iso() { date -u -d "@$1" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "$1"; }

print_status_line() {
  if ((ACTIVE)); then
    printf 'system-event: ACTIVE kind=%s id=%s — %s\n' "$KIND" "${EVENT_ID:-?}" "$ACTIVE_REASON" >&2
    printf 'system-event: DEFER new heavy work (git gc, bulk jobs, new dispatches/alert beads) until clear.\n' >&2
  else
    printf 'system-event: clear (abnormal %ss=%d across %d beads, 1h=%d; crashes %ss=%d; PSI some avg60=%s%%)\n' \
      "$SURGE_WINDOW" "$ABNORMAL_5M" "$BEADS_5M" "$ABNORMAL_1H" "$SURGE_WINDOW" "$CRASH_5M" "${PSI_READING:-unavailable}" >&2
  fi
  if source_is_stale && [[ "$KIND" != "source_stale" ]]; then
    printf 'system-event: DEGRADED — %s has no record newer than %ss (newest %s); detection is blind\n' \
      "$EVENTS_FILE" "$SOURCE_MAX_AGE_SECONDS" "$([[ $NEWEST_EPOCH -gt 0 ]] && iso "$NEWEST_EPOCH" || echo 'never')" >&2
  fi
}

cmd_check() {
  resolve
  if ((QUIET == 0)); then
    print_status_line
  fi
  ((ACTIVE)) && return 75
  return 0
}

cmd_status() {
  resolve
  echo "=== System Event Mode — $(date -u +%Y-%m-%dT%H:%M:%SZ) ==="
  echo
  echo "Live readings"
  echo "  event source        ${EVENTS_FILE}"
  echo "  newest record (any) $([[ $NEWEST_EPOCH -gt 0 ]] && iso "$NEWEST_EPOCH" || echo 'none')"
  echo "  source state        $(source_is_stale && echo "DEGRADED (>${SOURCE_MAX_AGE_SECONDS}s old)" || echo 'fresh')"
  echo "  abnormal in ${SURGE_WINDOW}s    ${ABNORMAL_5M} across ${BEADS_5M} bead(s)"
  echo "  abnormal in 1h      ${ABNORMAL_1H} across ${BEADS_1H} bead(s)"
  echo "  exit -1 crashes in ${SURGE_WINDOW}s  ${CRASH_5M}"
  echo "  PSI memory some avg60  ${PSI_READING:-unavailable}%"
  echo
  echo "Mode"
  if ((ACTIVE)); then
    echo "  STATE               ACTIVE (${KIND})"
    echo "  event id            ${EVENT_ID}"
    echo "  reason              ${ACTIVE_REASON}"
    echo "  latched             $([[ -n "$LATCHED_AT" ]] && iso "$LATCHED_AT" || echo '-')"
    echo "  hold until          $([[ -n "$HOLD_UNTIL" ]] && iso "$HOLD_UNTIL" || echo '-')"
  else
    echo "  STATE               clear"
  fi
  if [[ -f "$MANUAL_STATE" ]]; then
    echo "  manual latch        reason=\"$(state_field "$MANUAL_STATE" "reason")\" expires=$(state_field "$MANUAL_STATE" "expires_at")"
  fi
  echo
  echo "Thresholds (tune via environment)"
  echo "  crash_burst         >= ${CRASH_BURST_THRESHOLD} exit--1 crashes in ${SURGE_WINDOW}s"
  echo "  fail_wave           >= ${FAIL_WAVE_THRESHOLD} abnormal in ${SURGE_WINDOW}s across >= ${MIN_WAVE_BEADS} beads"
  echo "  sustained_storm     >= ${STORM_HOURLY_THRESHOLD} abnormal in 1h across >= ${MIN_WAVE_BEADS} beads"
  echo "  memory_pressure     PSI some avg60 >= ${PRESSURE_THRESHOLD}%"
  echo "  hold                ${HOLD_SECONDS}s after last trigger"
  echo
  echo "Deferral convention: exit 0 = proceed; exit 75 = defer new heavy work."
  ((ACTIVE)) && return 75
  return 0
}

cmd_json() {
  resolve
  local stale
  stale=0; source_is_stale && stale=1
  jq -n \
    --arg now "$(date -u +%Y-%m-%dT%H:%M:%SZ)" \
    --argjson active "$ACTIVE" \
    --arg kind "$KIND" \
    --arg event_id "$EVENT_ID" \
    --arg reason "$ACTIVE_REASON" \
    --argjson latched_at "${LATCHED_AT:-0}" \
    --argjson hold_until "${HOLD_UNTIL:-0}" \
    --argjson abnormal_5m "$ABNORMAL_5M" \
    --argjson beads_5m "$BEADS_5M" \
    --argjson abnormal_1h "$ABNORMAL_1H" \
    --argjson crash_5m "$CRASH_5M" \
    --arg psi "${PSI_READING:-}" \
    --argjson newest_epoch "$NEWEST_EPOCH" \
    --argjson source_stale "$stale" \
    '{now: $now, active: $active, kind: $kind, event_id: $event_id, reason: $reason,
      latched_at: $latched_at, hold_until: $hold_until,
      readings: {abnormal_5m: $abnormal_5m, beads_5m: $beads_5m, abnormal_1h: $abnormal_1h,
                 crash_5m: $crash_5m, psi_some_avg60: $psi, newest_epoch: $newest_epoch,
                 source_stale: $source_stale}}'
}

cmd_on() {
  local reason="" ttl="$MANUAL_TTL"
  while (($#)); do
    case "$1" in
      --reason) reason="${2:-}"; shift 2 ;;
      --ttl) ttl="${2:?--ttl needs seconds}"; shift 2 ;;
      *) echo "on: unknown argument $1" >&2; return 2 ;;
    esac
  done
  [[ -n "$reason" ]] || { echo "on: --reason is required" >&2; return 2; }
  mkdir -p "$STATE_DIR"
  printf '{"event_id": %s, "reason": "%s", "latched_at": %s, "expires_at": %s}\n' \
    "$NOW" "$(printf '%s' "$reason" | sed 's/\\/\\\\/g; s/"/\\"/g')" "$NOW" "$((NOW + ttl))" >"$MANUAL_STATE"
  log_event "MANUAL ON id=$NOW ttl=${ttl}s reason=\"${reason}\""
  echo "system-event: manual event latched for ${ttl}s — new heavy work should defer (exit 75)."
  echo "system-event: clear it with: $0 off"
}

cmd_off() {
  if [[ -f "$MANUAL_STATE" ]]; then
    local id
    id=$(state_field "$MANUAL_STATE" "event_id")
    rm -f "$MANUAL_STATE"
    log_event "MANUAL OFF id=manual-${id}"
    echo "system-event: manual latch cleared."
  else
    echo "system-event: no manual latch present."
  fi
}

cmd_alert_gate() {
  local bead_id="${1:-}"
  [[ -n "$bead_id" ]] || { echo "alert-gate: bead-id required" >&2; return 2; }
  resolve
  if ((ACTIVE == 0)); then
    echo "ALLOW: no system event active — alert creation is not gated"
    return 0
  fi
  mkdir -p "$STATE_DIR"
  touch "$ALERT_LEDGER"
  if grep -q "\"event_id\": \"${EVENT_ID}\"" "$ALERT_LEDGER" 2>/dev/null; then
    echo "SUPPRESS: system event ${KIND} (${EVENT_ID}) already has a recorded alert — coalescing"
    echo "  active reason: ${ACTIVE_REASON}"
    echo "  record the outcome on the existing investigation instead of a new ALERT bead;"
    echo "  a new event id means a new event, and this event will re-open the gate at hold_until $( [[ -n "$HOLD_UNTIL" ]] && iso "$HOLD_UNTIL" || echo '-' )"
    return 4
  fi
  printf '{"event_id": "%s", "kind": "%s", "bead_id": "%s", "recorded_at": "%s"}\n' \
    "$EVENT_ID" "$KIND" "$bead_id" "$(date -u +%Y-%m-%dT%H:%M:%SZ)" >>"$ALERT_LEDGER"
  log_event "ALERT ALLOWED id=${EVENT_ID} kind=${KIND} bead=${bead_id} (first alert of this event)"
  echo "ALLOW: first alert of system event ${EVENT_ID} (${KIND}) — recorded in ${ALERT_LEDGER}"
  echo "  further alerts for THIS event id will be suppressed until the event clears"
  return 0
}

# --- Main ----------------------------------------------------------------------
NOW=$(date +%s)
QUIET=0
COMMAND="${1:-}"
shift || true
case "$COMMAND" in
  check)       while (($#)); do case "$1" in --quiet) QUIET=1; shift ;; *) echo "check: unknown argument $1" >&2; exit 2 ;; esac; done; cmd_check ;;
  status)      cmd_status "$@" ;;
  json)        cmd_json "$@" ;;
  on)          cmd_on "$@" ;;
  off)         cmd_off "$@" ;;
  alert-gate)  [[ -n "${1:-}" ]] || { echo "alert-gate: bead-id required" >&2; exit 2; }; cmd_alert_gate "$@" ;;
  help|--help|-h) usage ;;
  "")          usage; exit 2 ;;
  *)           echo "system-event-mode: unknown command '$COMMAND'" >&2; usage >&2; exit 2 ;;
esac
