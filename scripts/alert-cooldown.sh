#!/usr/bin/env bash
# Alert Cooldown — global crash-alert cooldown with suppressed-crash accounting.
#
# Purpose
#   SIGHUP cascades and infrastructure events kill many workers within seconds,
#   and every kill arrives as its own crash trace. Without a cooldown the alert
#   layer fans out one ALERT bead per kill — exactly what turned the
#   2026-08-16 cascade into 177 crashes across 59 beads. This module holds a
#   single global cooldown window: the first crash alert opens it, every crash
#   that arrives while it is open is LOGGED but not alerted, and the first
#   crash after it closes carries a summary of everything that was suppressed.
#
# State
#   .beads/logs/crash-alert-metadata.json  (gitignored under .beads/)
#
#   {
#     "version": 1,
#     "cooldown_seconds": 300,
#     "last_alert_timestamp": "2026-09-07T19:00:00Z",   <- canonical, ISO-8601 UTC
#     "last_alert_epoch": 1778238000,                   <- arithmetic copy of the same instant
#     "last_alert_bead": "domchk-xxxx",
#     "last_alert_classification": "INFRASTRUCTURE",
#     "cooldown_active": true,
#     "suppressed_count": 2,                            <- crashes suppressed in THIS window
#     "suppressed_recent": [ {"bead_id": "...", "classification": "...",
#                             "timestamp": "...", "reason": "..."} ],
#     "summary_pending": true,                          <- window ended with suppressions not yet summarized
#     "total_alerts": 7,                                <- lifetime
#     "suppressed_total": 19,                           <- lifetime (never reset)
#     "updated_at": "..."
#   }
#
# Usage (CLI — each invocation is one serialized read-modify-write)
#   alert-cooldown.sh check
#       Exit 0 = alert may fire. Exit 3 = cooldown ACTIVE: log the crash, do
#       not alert (call record-suppressed). Exit 2 = usage/tooling error.
#       Fail-open: a missing or corrupt state file is treated as exit 0 —
#       alerting never depends on this gate being runnable.
#       On the first exit 0 after a window that suppressed crashes, stdout
#       carries the end-of-cooldown summary (COOLDOWN_EXPIRED block) for the
#       caller to fold into its alert; the summary is emitted exactly once.
#   alert-cooldown.sh record-alert <bead-id> [classification]
#       Open a new cooldown window now (the alert that just fired).
#   alert-cooldown.sh record-suppressed <bead-id> [classification] [reason]
#       Account one crash that arrived during the window and was not alerted.
#   alert-cooldown.sh status | reset
#       Print state JSON / clear state.
#
# Exit codes: 0 allowed/success · 3 cooldown active (suppress) · 2 usage error
#
# Scope: GLOBAL, not per-classification — the cooldown's job is to stop
# system-wide event fan-out, and a cascade does not wait for one
# classification to finish before the next starts. The per-classification
# window in crash-alert-manager.sh (alert-state.json) continues to exist;
# this file is the coarser outer gate and its state of record for
# "when did the last alert of ANY kind fire".
#
# Concurrency: every command takes an exclusive flock on
# <metadata>.lock (10 s wait, then proceeds best-effort) and writes via
# tmp+mv, so parallel worker invocations cannot interleave a read-modify-write.
#
# Environment
#   ALERT_COOLDOWN_METADATA_FILE  state path override (tests; sandboxing)
#   ALERT_COOLDOWN_SECONDS        window length, default 300 (5 minutes);
#                                 the value captured in the state file at
#                                 record-alert time governs that window, so a
#                                 mid-window config change cannot shorten it
#
# Library use: sourcing this file defines its cooldown_* functions and
# performs no I/O; the CLI runs only when executed directly.
#
# Created: 2026-09-07 (domchk-7b404946)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

ALERT_COOLDOWN_METADATA_FILE="${ALERT_COOLDOWN_METADATA_FILE:-$PROJECT_ROOT/.beads/logs/crash-alert-metadata.json}"
ALERT_COOLDOWN_SECONDS="${ALERT_COOLDOWN_SECONDS:-300}"
SUPPRESSED_RECENT_CAP=100   # entries kept in suppressed_recent
SUPPRESSED_SUMMARY_CAP=20   # detail lines printed in one summary

LOCK_FILE="${ALERT_COOLDOWN_METADATA_FILE}.lock"
LOG_DIR="$(dirname "$ALERT_COOLDOWN_METADATA_FILE")"
SUPPRESSED_LOG="$LOG_DIR/alert-cooldown.log"

log_line() { # $1=level, rest=message — append to the audit log, echo to stderr
    local level="$1"; shift
    local msg="$*"
    local ts
    ts=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    echo "[$ts] [$level] $msg" >&2
    echo "[$ts] [$level] $msg" >> "$SUPPRESSED_LOG" 2>/dev/null || true
}

# --- state helpers -----------------------------------------------------------

state_valid() { # $1 = raw JSON — 0 if parseable
    jq -e . >/dev/null 2>&1 <<<"$1"
}

load_state() { # echoes raw state JSON; empty output = absent/corrupt
    [[ -f "$ALERT_COOLDOWN_METADATA_FILE" ]] || return 0
    local raw
    raw=$(cat "$ALERT_COOLDOWN_METADATA_FILE" 2>/dev/null) || return 0
    state_valid "$raw" || { raw=""; printf '%s' ""; return 0; }
    printf '%s' "$raw"
}

write_state() { # $1 = raw JSON — atomic replace
    mkdir -p "$LOG_DIR"
    local tmp="$ALERT_COOLDOWN_METADATA_FILE.tmp.$$"
    printf '%s\n' "$1" > "$tmp"
    mv "$tmp" "$ALERT_COOLDOWN_METADATA_FILE"
}

state_get() { # $1=raw JSON, $2=jq path expr, $3=default — echoes value
    local val=""
    val=$(jq -r "$2" <<<"$1" 2>/dev/null) || val=""
    [[ -n "$val" && "$val" != "null" ]] || val="$3"
    printf '%s' "$val"
}

iso_now() { date -u +"%Y-%m-%dT%H:%M:%SZ"; }
epoch_now() { date +%s; }

# --- commands ----------------------------------------------------------------

cmd_check() {
    local state
    state=$(load_state)
    # Fail-open: no state, or unreadable/corrupt state, never blocks an alert.
    if [[ -z "$state" ]]; then
        echo "COOLDOWN_INACTIVE"
        return 0
    fi

    local last_epoch cooldown suppressed summary_pending
    last_epoch=$(state_get "$state" '.last_alert_epoch // 0' "0")
    cooldown=$(state_get "$state" '.cooldown_seconds // 0' "0")
    suppressed=$(state_get "$state" '.suppressed_count // 0' "0")
    summary_pending=$(state_get "$state" '.summary_pending // false' "false")
    [[ "$cooldown" -gt 0 ]] || cooldown="$ALERT_COOLDOWN_SECONDS"

    # No window has ever been opened (or the epoch copy was lost): inert.
    [[ "$last_epoch" -gt 0 ]] || { echo "COOLDOWN_INACTIVE"; return 0; }

    local now elapsed remaining
    now=$(epoch_now)
    elapsed=$((now - last_epoch))
    [[ $elapsed -ge 0 ]] || elapsed=0   # clock stepped back: treat as just-opened
    remaining=$((cooldown - elapsed))
    [[ $remaining -gt 0 ]] || remaining=0

    if [[ $remaining -gt 0 ]]; then
        echo "COOLDOWN_ACTIVE elapsed=$elapsed remaining=$remaining suppressed=$suppressed"
        return 3
    fi

    # Window closed. If it suppressed crashes nobody has summarized yet, the
    # first crash through the closed gate carries their summary — exactly once.
    if [[ "$summary_pending" == "true" && "$suppressed" -gt 0 ]]; then
        local bead cls closed_at count total
        bead=$(state_get "$state" '.last_alert_bead // "unknown"' "unknown")
        cls=$(state_get "$state" '.last_alert_classification // "UNKNOWN"' "UNKNOWN")
        closed_at=$(iso_now)
        count="$suppressed"
        total=$(state_get "$state" '.suppressed_total // 0' "0")

        echo "COOLDOWN_EXPIRED suppressed=$count last_alert_bead=$bead classification=$cls expired_at=$closed_at"
        local detail
        detail=$(jq -r --argjson cap "$SUPPRESSED_SUMMARY_CAP" \
            '.suppressed_recent[:$cap][] | "SUPPRESSED \(.timestamp) \(.bead_id) \(.classification // "-") \(.reason // "-")"' \
            <<<"$state" 2>/dev/null || true)
        if [[ -n "$detail" ]]; then
            printf '%s\n' "$detail"
            local listed overflow
            listed=$(printf '%s\n' "$detail" | wc -l)
            overflow=$((count - listed))
            if [[ $overflow -gt 0 ]]; then
                echo "SUPPRESSED ... and $overflow more (full list: $SUPPRESSED_LOG)"
            fi
        fi
        echo "SUMMARY: $count crash(es) suppressed during the ${cooldown}s cooldown that followed alert $bead ($cls); cooldown expired at $closed_at (lifetime suppressed: $total)."

        # Consume: clear the window accounting, keep lifetime counters.
        jq '.summary_pending = false | .suppressed_count = 0 | .suppressed_recent = [] | .updated_at = $ts' \
            --arg ts "$(iso_now)" <<<"$state" > "$ALERT_COOLDOWN_METADATA_FILE.tmp.$$" 2>/dev/null \
            && mv "$ALERT_COOLDOWN_METADATA_FILE.tmp.$$" "$ALERT_COOLDOWN_METADATA_FILE" \
            || log_line "WARN" "could not persist summary consumption for window of $bead"
        return 0
    fi

    echo "COOLDOWN_INACTIVE"
    return 0
}

cmd_record_alert() { # $1 = bead that just alerted, $2 = classification (optional)
    local bead="${1:-}"
    local classification="${2:-UNKNOWN}"
    if [[ -z "$bead" ]]; then
        echo "usage: $0 record-alert <bead-id> [classification]" >&2
        return 2
    fi

    local state
    state=$(load_state)

    # A window replaced before its suppressed crashes were summarized loses
    # that summary — say so in the audit log rather than dropping it silently.
    if [[ -n "$state" ]]; then
        local pending pcount
        pending=$(state_get "$state" '.summary_pending // false' "false")
        pcount=$(state_get "$state" '.suppressed_count // 0' "0")
        if [[ "$pending" == "true" && "$pcount" -gt 0 ]]; then
            log_line "WARN" "window of $(state_get "$state" '.last_alert_bead // "unknown"' "unknown") replaced by alert $bead with $pcount unsummarized suppressed crash(es) (lifetime counters retain them)"
        fi
    fi

    local now_iso now_epoch cooldown total_alerts new_state
    now_iso=$(iso_now)
    now_epoch=$(epoch_now)
    cooldown="$ALERT_COOLDOWN_SECONDS"
    total_alerts=$(( $(state_get "$state" '.total_alerts // 0' "0") + 1 ))

    new_state=$(jq -n \
        --arg ts "$now_iso" --argjson epoch "$now_epoch" \
        --arg bead "$bead" --arg cls "$classification" \
        --argjson cd "$cooldown" --argjson alerts "$total_alerts" \
        --argjson suppressed_total "$(state_get "$state" '.suppressed_total // 0' "0")" \
        '{version: 1,
          cooldown_seconds: $cd,
          last_alert_timestamp: $ts,
          last_alert_epoch: $epoch,
          last_alert_bead: $bead,
          last_alert_classification: $cls,
          cooldown_active: true,
          suppressed_count: 0,
          suppressed_recent: [],
          summary_pending: false,
          total_alerts: $alerts,
          suppressed_total: $suppressed_total,
          updated_at: $ts}')
    write_state "$new_state"
    log_line "INFO" "alert $bead ($classification) opened a ${cooldown}s cooldown window"
    return 0
}

cmd_record_suppressed() { # $1 = bead, $2 = classification (opt), $3 = reason (opt)
    local bead="${1:-}"
    local classification="${2:-UNKNOWN}"
    local reason="${3:-}"
    if [[ -z "$bead" ]]; then
        echo "usage: $0 record-suppressed <bead-id> [classification] [reason]" >&2
        return 2
    fi

    local state
    state=$(load_state)
    [[ -n "$state" ]] || state=$(jq -n '{version: 1, cooldown_seconds: 0, last_alert_timestamp: null, last_alert_epoch: 0, last_alert_bead: null, last_alert_classification: null, cooldown_active: false, suppressed_count: 0, suppressed_recent: [], summary_pending: false, total_alerts: 0, suppressed_total: 0, updated_at: null}')

    local now_iso now_epoch count total new_state
    now_iso=$(iso_now)
    now_epoch=$(epoch_now)
    count=$(( $(state_get "$state" '.suppressed_count // 0' "0") + 1 ))
    total=$(( $(state_get "$state" '.suppressed_total // 0' "0") + 1 ))

    # The crash that arrived during the window is LOGGED (audit log + JSON
    # ledger) even though it is not alerted — nothing is silently dropped.
    log_line "INFO" "crash $bead ($classification) suppressed by active cooldown (window suppressed total: $count)${reason:+ — $reason}"

    new_state=$(jq \
        --arg ts "$now_iso" --arg epoch "$now_epoch" \
        --arg bead "$bead" --arg cls "$classification" --arg reason "$reason" \
        --argjson count "$count" --argjson total "$total" \
        --argjson cap "$SUPPRESSED_RECENT_CAP" \
        '.cooldown_active = true
       | .suppressed_count = $count
       | .suppressed_total = $total
       | .summary_pending = true
       | .updated_at = $ts
       | .suppressed_recent = ((.suppressed_recent + [{bead_id: $bead, classification: $cls, timestamp: $ts, reason: (if $reason == "" then null else $reason end)}]) | .[-$cap:])' \
        <<<"$state") || { log_line "ERROR" "state update failed for suppressed crash $bead"; return 2; }
    write_state "$new_state"
    return 0
}

cmd_status() {
    local state
    state=$(load_state)
    if [[ -z "$state" ]]; then
        echo '{"state": "absent", "cooldown_active": false}'
        return 0
    fi
    jq . <<<"$state"
}

cmd_reset() {
    rm -f "$ALERT_COOLDOWN_METADATA_FILE" "$ALERT_COOLDOWN_METADATA_FILE.tmp.$$"
    log_line "INFO" "cooldown state cleared ($ALERT_COOLDOWN_METADATA_FILE)"
    return 0
}

usage() {
    cat <<EOF
Usage: $0 <command> [args]

Commands:
  check                                          0 = alert allowed, 3 = cooldown active, 2 = error
  record-alert <bead-id> [classification]        open a new cooldown window
  record-suppressed <bead-id> [cls] [reason]     log a crash suppressed by the window
  status                                         print state JSON
  reset                                          clear state

State: $ALERT_COOLDOWN_METADATA_FILE
Window: \$ALERT_COOLDOWN_SECONDS (default 300s, global — any alert type)
EOF
}

main() {
    local cmd="${1:-}"
    [[ -n "$cmd" ]] || { usage >&2; return 2; }
    shift

    # Serialize check/record across concurrent worker invocations. A busy lock
    # after 10s degrades to best-effort rather than failing an alert decision.
    if command -v flock >/dev/null 2>&1; then
        mkdir -p "$LOG_DIR" 2>/dev/null || true
        exec 9>>"$LOCK_FILE"
        flock -w 10 9 || log_line "WARN" "lock busy after 10s ($LOCK_FILE) — proceeding without serialization"
    fi

    case "$cmd" in
        check)             cmd_check ;;
        record-alert)      cmd_record_alert "$@" ;;
        record-suppressed) cmd_record_suppressed "$@" ;;
        status)            cmd_status ;;
        reset)             cmd_reset ;;
        help|--help|-h)    usage ;;
        *)                 usage >&2; return 2 ;;
    esac
}

# Sourceable: no side effects when sourced, CLI when executed.
if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
    exit $?
fi
