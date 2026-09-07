#!/usr/bin/env bash
# Crash Circuit Breaker
# Purpose: Stop crash-storm retry loops by tripping a per-bead breaker after
#          N consecutive crashes, then backing off / deferring instead of
#          letting the release-and-retry loop re-dispatch a doomed task.
# Created: 2026-09-02
# Committed: 2026-09-07 by domchk-0c916ec7 — authored in-worktree 2026-09-02
# for the bf-65lsdu chain and cited as a live mitigation by committed docs
# (docs/crash-prevention-design.md, docs/crash-investigations/bf-173o7e-*),
# but never landed by an owning bead. Landed alongside its storm regression
# test scripts/test-crash-storm-regression.sh, which pins the RCA §7 contract
# this file implements (trip at 3 consecutive -1/137 crashes, alerts bounded,
# bead deferred instead of re-dispatched).
#
# Implements the residual gap identified in
# docs/research/root-cause-analysis-bf-65lsdu-signal-minus-one-2026-09-02.md §7:
#   "the dispatcher's release-and-retry loop has no crash-storm breaker — it
#    re-dispatched an identical doomed task 127 times in 2.5 hours ... A
#    circuit breaker (N consecutive crashes on the same bead -> back off /
#    defer) would convert a 127-alert storm into one alert and one deferral."
#
# State machine (per bead):
#   closed    -> crashes recorded; trips at BREAKER_THRESHOLD consecutive
#   open      -> dispatch blocked; retry_after = now + cooldown, doubling per re-trip
#   half-open -> after retry_after elapses, ONE probe dispatch is allowed
#   closed    <- probe (or any attempt) exits 0; open again on crash
#
# Only infrastructure-style deaths count as breaker crashes: exit code -1
# (NEEDLE "died without exiting", see RCA §3) and 137 (SIGKILL / memcg OOM).
# Timeouts (124) and workflow failures (exit 1) are handled elsewhere
# (retry-with-backoff.sh, crash-classifier.sh) and do not trip the breaker.

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
LOG_DIR="$PROJECT_ROOT/.beads/logs"
BREAKER_LOG="$LOG_DIR/circuit-breaker.log"

BREAKER_STATE_FILE="${BREAKER_STATE_FILE:-$LOG_DIR/circuit-breaker-state.json}"
BREAKER_THRESHOLD="${BREAKER_THRESHOLD:-3}"              # consecutive crashes to trip
BREAKER_BASE_COOLDOWN="${BREAKER_BASE_COOLDOWN:-1800}"   # 30 min first backoff
BREAKER_MAX_COOLDOWN="${BREAKER_MAX_COOLDOWN:-14400}"    # 4 h backoff ceiling
BREAKER_DECAY_SECONDS="${BREAKER_DECAY_SECONDS:-86400}"  # counter resets if last crash older than 24h
# Exit codes treated as infrastructure crashes (RCA §3: -1 = died without exiting; 137 = OOM SIGKILL)
BREAKER_CRASH_CODES="${BREAKER_CRASH_CODES:--1,137}"
EVENTS_FILE="${BEAD_EVENTS_FILE:-$PROJECT_ROOT/.beads/events.jsonl}"

mkdir -p "$LOG_DIR"

log_breaker() {
    local level="$1"
    shift
    local message="$*"
    local timestamp
    timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    echo "[$timestamp] [$level] $message" | tee -a "$BREAKER_LOG"
}

show_usage() {
    cat <<EOF
Usage: $0 <command> [args]

Per-bead circuit breaker that stops crash-storm retry loops.

Commands:
  check <bead-id>         Is dispatch allowed? Exit 0 = allow (closed or
                          half-open probe), exit 4 = BLOCKED (breaker open).
  record <bead-id> <code> Record an attempt outcome. Nonzero infrastructure
                          crash codes (-1, 137) advance the consecutive-crash
                          counter; 0 resets it. Trips the breaker at threshold.
  status [bead-id]        Print breaker state (one bead or all).
  defer <bead-id>         Defer an open breaker's bead via 'bead update
                          --status deferred' instead of release-and-retry.
  reset <bead-id>         Manually reset one bead's breaker (closed, counter 0).
  reset-all               Reset the entire breaker state.
  --rebuild               Rebuild state from .beads/events.jsonl crash events.

Options (environment):
  BREAKER_THRESHOLD       consecutive crashes to trip (default: 3)
  BREAKER_BASE_COOLDOWN   first backoff seconds (default: 1800)
  BREAKER_MAX_COOLDOWN    backoff ceiling seconds (default: 14400)
  BREAKER_DECAY_SECONDS   counter decay age (default: 86400)
  BREAKER_CRASH_CODES     exit codes counted as crashes (default: "-1,137")
  BREAKER_STATE_FILE      state file path (default: .beads/logs/circuit-breaker-state.json)

Exit Codes (check):
  0  Dispatch allowed
  4  Dispatch blocked - breaker is OPEN
  3  Usage / processing error
EOF
}

is_crash_code() {
    local exit_code="$1"
    local code
    IFS=',' read -ra codes <<< "$BREAKER_CRASH_CODES"
    for code in "${codes[@]}"; do
        if [[ "$exit_code" == "$code" ]]; then
            return 0
        fi
    done
    return 1
}

now_epoch() {
    date +%s
}

now_iso() {
    date -u +"%Y-%m-%dT%H:%M:%SZ"
}

# Initialize state file if missing
init_state() {
    if [[ ! -f "$BREAKER_STATE_FILE" ]]; then
        echo '{"beads":{}}' > "$BREAKER_STATE_FILE"
    fi
}

# Read a field of a bead's state; empty string if bead absent
bead_field() {
    local bead_id="$1"
    local field="$2"
    jq -r --arg id "$bead_id" --arg f "$field" '.beads[$id][$f] // empty' "$BREAKER_STATE_FILE" 2>/dev/null || echo ""
}

# Write/replace a bead's state object
set_bead_state() {
    local bead_id="$1"
    local state_json="$2"
    local tmp="$BREAKER_STATE_FILE.tmp"
    jq --arg id "$bead_id" --argjson s "$state_json" --arg ts "$(now_iso)" \
        '.beads[$id] = $s | .updated_at = $ts' "$BREAKER_STATE_FILE" > "$tmp"
    mv "$tmp" "$BREAKER_STATE_FILE"
}

remove_bead_state() {
    local bead_id="$1"
    local tmp="$BREAKER_STATE_FILE.tmp"
    jq --arg id "$bead_id" --arg ts "$(now_iso)" \
        '.beads |= del(.[ $id ]) | .updated_at = $ts' "$BREAKER_STATE_FILE" > "$tmp"
    mv "$tmp" "$BREAKER_STATE_FILE"
}

# Backoff for the Nth trip of this bead (exponential, capped)
cooldown_for_trip() {
    local open_count="$1"
    local cooldown=$BREAKER_BASE_COOLDOWN
    local i
    for ((i = 1; i < open_count; i++)); do
        cooldown=$((cooldown * 2))
        if [[ $cooldown -ge $BREAKER_MAX_COOLDOWN ]]; then
            cooldown=$BREAKER_MAX_COOLDOWN
            break
        fi
    done
    echo "$cooldown"
}

# Apply counter decay: a crash long after the last one starts a fresh count
apply_decay_if_stale() {
    local bead_id="$1"
    local last_ts
    last_ts=$(bead_field "$bead_id" "last_crash_ts")
    [[ -z "$last_ts" ]] && return 0

    local last_epoch
    last_epoch=$(date -d "$last_ts" +%s 2>/dev/null || echo "0")
    local now
    now=$(now_epoch)
    if [[ $last_epoch -gt 0 ]] && [[ $((now - last_epoch)) -ge $BREAKER_DECAY_SECONDS ]]; then
        log_breaker "INFO" "Counter for $bead_id decayed (last crash $last_ts older than ${BREAKER_DECAY_SECONDS}s)"
        remove_bead_state "$bead_id"
    fi
}

# check <bead-id> — exit 0 allow / 4 blocked
cmd_check() {
    local bead_id="$1"
    init_state

    local state
    state=$(bead_field "$bead_id" "state")

    if [[ -z "$state" || "$state" == "closed" ]]; then
        echo "ALLOWED: $bead_id (breaker closed)"
        exit 0
    fi

    local retry_after
    retry_after=$(bead_field "$bead_id" "retry_after")
    local now
    now=$(now_epoch)
    local retry_epoch
    retry_epoch=$(date -d "$retry_after" +%s 2>/dev/null || echo "0")

    if [[ $now -ge $retry_epoch ]]; then
        # Cooldown elapsed — allow ONE half-open probe
        echo "ALLOWED: $bead_id (half-open probe, breaker was open since $(bead_field "$bead_id" "opened_at"))"
        exit 0
    fi

    echo "BLOCKED: $bead_id (breaker OPEN)"
    echo "  consecutive_crashes: $(bead_field "$bead_id" "consecutive_crashes")"
    echo "  opened_at: $(bead_field "$bead_id" "opened_at")"
    echo "  retry_after: $retry_after ($((retry_epoch - now))s remaining)"
    echo "  Action: bead is deferred instead of release-and-retried"
    exit 4
}

# record <bead-id> <exit-code>
cmd_record() {
    local bead_id="$1"
    local exit_code="$2"
    init_state
    apply_decay_if_stale "$bead_id"

    if [[ "$exit_code" == "0" ]]; then
        # Success — close the breaker
        if [[ -n "$(bead_field "$bead_id" "state")" ]] && [[ "$(bead_field "$bead_id" "state")" != "closed" ]]; then
            log_breaker "INFO" "SUCCESS closes breaker for $bead_id (was $(bead_field "$bead_id" "state"))"
        fi
        remove_bead_state "$bead_id"
        echo "OK: $bead_id breaker closed (success recorded)"
        exit 0
    fi

    if ! is_crash_code "$exit_code"; then
        # Workflow failure / timeout — not an infrastructure crash, not counted
        echo "OK: exit code $exit_code is not a breaker crash code ($BREAKER_CRASH_CODES) — counter unchanged"
        exit 0
    fi

    local consecutive
    consecutive=$(bead_field "$bead_id" "consecutive_crashes")
    consecutive=$(( ${consecutive:-0} + 1 ))

    local ts
    ts=$(now_iso)

    if [[ $consecutive -ge $BREAKER_THRESHOLD ]]; then
        local open_count
        open_count=$(bead_field "$bead_id" "open_count")
        open_count=$(( ${open_count:-0} + 1 ))
        local cooldown
        cooldown=$(cooldown_for_trip "$open_count")
        local retry_after
        retry_after=$(date -u -d "@$(( $(now_epoch) + cooldown ))" +"%Y-%m-%dT%H:%M:%SZ")

        local state_json
        state_json=$(jq -n \
            --argjson crashes "$consecutive" \
            --argjson open_count "$open_count" \
            --arg state "open" \
            --arg opened_at "$ts" \
            --arg retry_after "$retry_after" \
            --arg last_crash_ts "$ts" \
            --argjson last_exit_code "$exit_code" \
            '{state: $state, consecutive_crashes: $crashes, open_count: $open_count,
              opened_at: $opened_at, retry_after: $retry_after,
              last_crash_ts: $last_crash_ts, last_exit_code: $last_exit_code}')
        set_bead_state "$bead_id" "$state_json"

        log_breaker "TRIPPED" "$bead_id OPEN after $consecutive consecutive crashes (exit $exit_code); back off ${cooldown}s (trip #$open_count), defer instead of re-dispatch"
        echo "TRIPPED: $bead_id breaker OPEN"
        echo "  consecutive_crashes: $consecutive"
        echo "  backoff_seconds: $cooldown"
        echo "  retry_after: $retry_after"
        exit 1
    fi

    # Below threshold — keep counting, stay closed
    local state_json
    state_json=$(jq -n \
        --argjson crashes "$consecutive" \
        --arg state "closed" \
        --arg last_crash_ts "$ts" \
        --argjson last_exit_code "$exit_code" \
        '{state: $state, consecutive_crashes: $crashes,
          last_crash_ts: $last_crash_ts, last_exit_code: $last_exit_code}')
    set_bead_state "$bead_id" "$state_json"

    log_breaker "INFO" "$bead_id crash recorded (exit $exit_code): $consecutive/$BREAKER_THRESHOLD consecutive"
    echo "OK: $bead_id crash recorded ($consecutive/$BREAKER_THRESHOLD consecutive)"
    exit 0
}

# status [bead-id]
cmd_status() {
    init_state
    if [[ -n "${1:-}" ]]; then
        local state
        state=$(bead_field "$1" "state")
        if [[ -z "$state" ]]; then
            echo "breaker: $1 CLOSED (no recorded crashes)"
            exit 0
        fi
        jq --arg id "$1" '.beads[$id]' "$BREAKER_STATE_FILE"
        exit 0
    fi
    jq . "$BREAKER_STATE_FILE"
}

# defer <bead-id> — back off / defer instead of release-and-retry
cmd_defer() {
    local bead_id="$1"
    local state
    state=$(bead_field "$bead_id" "state")
    if [[ "$state" != "open" ]]; then
        echo "NOT OPEN: $bead_id breaker is ${state:-closed} — nothing to defer"
        exit 0
    fi

    log_breaker "ACTION" "Deferring $bead_id (breaker open, retry_after $(bead_field "$bead_id" "retry_after"))"
    if bead update "$bead_id" --status deferred \
        --notes "circuit breaker: $(bead_field "$bead_id" "consecutive_crashes") consecutive infrastructure crashes (last exit $(bead_field "$bead_id" "last_exit_code")); retry after $(bead_field "$bead_id" "retry_after")" >> "$BREAKER_LOG" 2>&1; then
        log_breaker "ACTION" "$bead_id deferred successfully"
        echo "DEFERRED: $bead_id (was open)"
        exit 0
    else
        log_breaker "WARN" "Failed to defer $bead_id (bead CLI error) — breaker remains open, dispatch still blocked by 'check'"
        echo "DEFER FAILED: $bead_id (bead CLI error) — breaker remains open"
        exit 2
    fi
}

# reset <bead-id>
cmd_reset() {
    init_state
    remove_bead_state "$1"
    log_breaker "INFO" "$1 breaker manually reset"
    echo "RESET: $1"
}

# reset-all
cmd_reset_all() {
    echo '{"beads":{}}' > "$BREAKER_STATE_FILE"
    log_breaker "INFO" "All breaker state reset"
    echo "RESET ALL"
}

# --rebuild — bootstrap state from .beads/events.jsonl
cmd_rebuild() {
    if [[ ! -f "$EVENTS_FILE" ]]; then
        echo "ERROR: events file not found: $EVENTS_FILE" >&2
        exit 3
    fi
    init_state

    local crash_lines
    crash_lines=$(jq -c 'select(.event == "crash" or .event == "complete") |
        select(.exit_code != null) |
        {bead: .bead, exit_code: .exit_code, ts: .ts}' "$EVENTS_FILE" 2>/dev/null)

    if [[ -z "$crash_lines" ]]; then
        echo "No crash/complete events found — state unchanged"
        exit 0
    fi

    local tripped=0
    local last_bead=""
    local consecutive=0
    local last_crash_ts=""

    while IFS= read -r line; do
        local bead exit_code ts
        bead=$(jq -r '.bead' <<< "$line")
        exit_code=$(jq -r '.exit_code' <<< "$line")
        ts=$(jq -r '.ts' <<< "$line")

        if [[ "$bead" != "$last_bead" ]]; then
            consecutive=0
            last_bead="$bead"
        fi

        if [[ "$exit_code" == "0" ]]; then
            consecutive=0
            continue
        fi
        is_crash_code "$exit_code" || continue
        consecutive=$((consecutive + 1))
        last_crash_ts="$ts"

        if [[ $consecutive -ge $BREAKER_THRESHOLD ]] && [[ -z "$(bead_field "$bead" "state")" || "$(bead_field "$bead" "state")" == "closed" ]]; then
            local open_count
            open_count=$(bead_field "$bead" "open_count")
            open_count=$(( ${open_count:-0} + 1 ))
            local cooldown
            cooldown=$(cooldown_for_trip "$open_count")
            local retry_after
            retry_after=$(date -u -d "@$(( $(date -d "$ts" +%s) + cooldown ))" +"%Y-%m-%dT%H:%M:%SZ" 2>/dev/null || date -u -d "@$(( $(now_epoch) + cooldown ))" +"%Y-%m-%dT%H:%M:%SZ")
            local state_json
            state_json=$(jq -n \
                --argjson crashes "$consecutive" \
                --argjson open_count "$open_count" \
                --arg opened_at "$ts" \
                --arg retry_after "$retry_after" \
                --arg last_crash_ts "$ts" \
                --argjson last_exit_code "$exit_code" \
                '{state: "open", consecutive_crashes: $crashes, open_count: $open_count,
                  opened_at: $opened_at, retry_after: $retry_after,
                  last_crash_ts: $last_crash_ts, last_exit_code: $last_exit_code}')
            set_bead_state "$bead" "$state_json"
            log_breaker "TRIPPED" "$bead OPEN on rebuild ($consecutive consecutive crashes ending $ts)"
            echo "TRIPPED (rebuild): $bead — $consecutive consecutive crashes ending $ts"
            tripped=$((tripped + 1))
        elif [[ -z "$(bead_field "$bead" "state")" ]]; then
            local state_json
            state_json=$(jq -n \
                --argjson crashes "$consecutive" \
                --arg last_crash_ts "$ts" \
                --argjson last_exit_code "$exit_code" \
                '{state: "closed", consecutive_crashes: $crashes,
                  last_crash_ts: $last_crash_ts, last_exit_code: $last_exit_code}')
            set_bead_state "$bead" "$state_json"
        fi
    done <<< "$crash_lines"

    log_breaker "INFO" "Rebuild from events.jsonl complete: $tripped breaker(s) open"
    echo "Rebuild complete: $tripped breaker(s) open, state at $BREAKER_STATE_FILE"
}

# Main
COMMAND="${1:-}"
shift || true

case "$COMMAND" in
    check)
        [[ $# -eq 1 ]] || { echo "ERROR: check requires exactly one bead-id" >&2; show_usage; exit 3; }
        cmd_check "$1"
        ;;
    record)
        [[ $# -eq 2 ]] || { echo "ERROR: record requires <bead-id> <exit-code>" >&2; show_usage; exit 3; }
        cmd_record "$1" "$2"
        ;;
    status)
        cmd_status "${1:-}"
        ;;
    defer)
        [[ $# -eq 1 ]] || { echo "ERROR: defer requires exactly one bead-id" >&2; show_usage; exit 3; }
        cmd_defer "$1"
        ;;
    reset)
        [[ $# -eq 1 ]] || { echo "ERROR: reset requires exactly one bead-id" >&2; show_usage; exit 3; }
        cmd_reset "$1"
        ;;
    reset-all)
        cmd_reset_all
        ;;
    --rebuild)
        cmd_rebuild
        ;;
    -h|--help|"")
        show_usage
        [[ -z "$COMMAND" ]] && exit 3
        exit 0
        ;;
    *)
        echo "ERROR: Unknown command: $COMMAND" >&2
        show_usage
        exit 3
        ;;
esac
