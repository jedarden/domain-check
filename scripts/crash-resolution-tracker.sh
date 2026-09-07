#!/usr/bin/env bash
# Crash Resolution Tracker
# Tracks and manages crash resolution state to prevent false positive alerts
# Maintains persistent state across restarts to identify resolved crashes

set -euo pipefail

# Configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BEAD_DIR="$PROJECT_ROOT/.beads"
TRACE_DIR="$BEAD_DIR/traces"
LOG_DIR="$BEAD_DIR/logs"
STATE_DIR="$BEAD_DIR/state"

# Resolution state file
RESOLUTION_STATE_FILE="$STATE_DIR/crash-resolutions.json"
LOG_FILE="$LOG_DIR/crash-resolution-tracker.log"

# Ensure directories exist
mkdir -p "$LOG_DIR" "$STATE_DIR"

# Resolution criteria
TASK_COMPLETION_WINDOW_SECONDS=30  # Task completed within 30 seconds of crash
RESOLUTION_AGE_DAYS=30  # Keep resolution records for 30 days

# Logging
log_resolution() {
    local level="$1"
    shift
    local message="$*"
    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    echo "[$timestamp] [$level] $message" | tee -a "$LOG_FILE"
}

# Usage
show_usage() {
    cat <<EOF
Usage: $0 <bead-id> [action]

Actions:
  check              Check if crash is resolved (default)
  mark-resolved      Mark crash as resolved
  mark-unresolved    Mark crash as unresolved
  show-state        Show resolution state for a bead
  list-resolved     List all resolved crashes
  cleanup           Clean up old resolution records
  -h, --help        Show this help message

Resolution Criteria:
  - task_completion    VERIFIED work-completion marker (.beads/state/work-completion/<id>.json),
                       or exit code 0 in the bead's trace (mark-resolved only)
  - bead_closure      Bead status is CLOSED
  - repository_recovery Repository cleaned/healthy (for OOM crashes)
  - manual            Manually marked resolved

`check` evaluates LIVE state first — bead closure via the bead store, then the
work-completion marker — and only then consults
.beads/state/crash-resolutions.json, which is a cache of manual marks and
FALSE_POSITIVE auto-marks, not the source of truth. Closure-based resolutions
never expire; other cached records expire after $RESOLUTION_AGE_DAYS days.

Exit Codes:
  0  Crash is resolved (or action succeeded)
  1  Crash is not resolved
  2  Error in processing

EOF
}

# Initialize resolution state file
init_state_file() {
    if [[ ! -f "$RESOLUTION_STATE_FILE" ]]; then
        cat > "$RESOLUTION_STATE_FILE" <<EOF
{
  "resolutions": {},
  "metadata": {
    "version": "1.0",
    "created": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")",
    "last_updated": "$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
  }
}
EOF
        log_resolution "INFO" "Initialized resolution state file"
    fi
    return 0
}

# Check if task completed before crash
check_task_completion() {
    local bead_id="$1"
    local trace_file="$TRACE_DIR/$bead_id/trace.jsonl"

    if [[ ! -f "$trace_file" ]]; then
        echo "no_trace"
        return
    fi

    # Check for exit code 0 (success) — space-tolerant: 99.7% of trace
    # metadata is pretty-printed ("exit_code": 0), which the exact-match grep
    # silently missed (gap D-5)
    if grep -qE '"exit_code":\s*0' "$trace_file" 2>/dev/null; then
        echo "exit_code_0"
        return
    fi

    # Check for task completion indicators
    if grep -q "work.*complete\|task.*done\|all.*acceptance.*criteria.*met" "$trace_file" 2>/dev/null; then
        echo "completion_indicators"
        return
    fi

    # Check for git commit after work completion
    if grep -q "git commit\|committed.*changes" "$trace_file" 2>/dev/null; then
        # Extract timestamps to verify completion was before crash
        local last_commit=$(grep -o '"timestamp":"[^"]*"' "$trace_file" 2>/dev/null | grep commit | tail -1 || echo "")
        local last_event=$(grep -o '"timestamp":"[^"]*"' "$trace_file" 2>/dev/null | tail -1 || echo "")

        if [[ -n "$last_commit" ]] && [[ -n "$last_event" ]]; then
            # Parse timestamps and check if commit was within completion window
            local commit_time=$(echo "$last_commit" | cut -d'"' -f4)
            local event_time=$(echo "$last_event" | cut -d'"' -f4)

            if [[ -n "$commit_time" ]] && [[ -n "$event_time" ]]; then
                local commit_secs=$(date -d "$commit_time" +%s 2>/dev/null || echo "0")
                local event_secs=$(date -d "$event_time" +%s 2>/dev/null || echo "0")

                if [[ $commit_secs -gt 0 ]] && [[ $event_secs -gt 0 ]]; then
                    local diff=$((event_secs - commit_secs))
                    if [[ $diff -le $TASK_COMPLETION_WINDOW_SECONDS ]]; then
                        echo "committed_before_crash"
                        return
                    fi
                fi
            fi
        fi
    fi

    echo "no_completion"
}

# Check if bead is closed. Three states, because the ledger path below needs to
# tell "the store answered and it is not closed" (reopened) apart from "the
# store could not be read" (transient failure — trust the cache instead).
check_bead_closure() {
    local bead_id="$1"

    local bead_status=$(bead show "$bead_id" 2>/dev/null | grep -i "^Status" || echo "")

    if [[ "$bead_status" =~ [Cc]losed ]]; then
        echo "closed"
    elif [[ -n "$bead_status" ]]; then
        echo "open"
    else
        echo "unknown"
    fi
}

# Check for a VERIFIED work-completion marker (gap G-9): verify-work-completion.sh
# writes .beads/state/work-completion/<id>.json with result VERIFIED | FAILED.
# Only VERIFIED counts as resolution — a FAILED marker means the work did not
# hold up under re-verification.
check_work_completion_marker() {
    local bead_id="$1"
    local marker="$STATE_DIR/work-completion/$bead_id.json"

    [[ -f "$marker" ]] || { echo "no_marker"; return; }

    local result=$(jq -r '.result // "VERIFIED"' "$marker" 2>/dev/null || echo "VERIFIED")
    if [[ "$result" == "VERIFIED" ]]; then
        echo "verified_marker"
    else
        echo "marker_$result"
    fi
}

# Check repository health (for OOM crashes)
check_repository_health() {
    local bead_id="$1"

    # git count-objects -v (NOT -vH): with -vH the awk field was the unit token
    # ("MiB"), so the loose-object test compared unit strings and was inert
    # (gap D-8). Thresholds are CLAUDE.md's repository size limits.
    local repo_size_kb=$(du -sk "$PROJECT_ROOT/.git" 2>/dev/null | awk '{print $1}' || echo "0")
    local loose_bytes=$(git -C "$PROJECT_ROOT" count-objects -v 2>/dev/null | sed -n 's/^size: //p' | tr -d '[:space:]')
    local loose_count=$(git -C "$PROJECT_ROOT" count-objects -v 2>/dev/null | sed -n 's/^count: //p' | tr -d '[:space:]')

    repo_size_kb=${repo_size_kb:-0}
    loose_bytes=${loose_bytes:-0}
    loose_count=${loose_count:-0}

    # Healthy: repo < 1GB, loose objects < 100MB and fewer than 1000 of them
    if [[ "$repo_size_kb" -lt 1048576 ]] \
       && [[ "$loose_bytes" -lt 104857600 ]] \
       && [[ "$loose_count" -lt 1000 ]]; then
        echo "healthy"
        return
    fi

    echo "unhealthy"
}

# Determine resolution type for a crash
determine_resolution_type() {
    local bead_id="$1"

    # Check task completion first
    local task_status=$(check_task_completion "$bead_id")
    case "$task_status" in
        exit_code_0|completion_indicators|committed_before_crash)
            echo "task_completion"
            return
            ;;
    esac

    # Check bead closure
    local closure_status=$(check_bead_closure "$bead_id")
    if [[ "$closure_status" == "closed" ]]; then
        echo "bead_closure"
        return
    fi

    # Check repository health (for OOM crashes)
    local trace_file="$TRACE_DIR/$bead_id/trace.jsonl"
    if [[ -f "$trace_file" ]] && grep -qi "oom\|memory" "$trace_file" 2>/dev/null; then
        local repo_health=$(check_repository_health "$bead_id")
        if [[ "$repo_health" == "healthy" ]]; then
            echo "repository_recovery"
            return
        fi
    fi

    echo "unresolved"
}

# Get resolution state for a bead
get_resolution() {
    local bead_id="$1"

    if [[ ! -f "$RESOLUTION_STATE_FILE" ]]; then
        echo ""
        return
    fi

    jq -r --arg id "$bead_id" '.resolutions[$id] // empty' "$RESOLUTION_STATE_FILE" 2>/dev/null || echo ""
}

# Backfill the ledger cache when live evaluation found a resolution the cache
# did not know about. Idempotent; the cache stays a cache — the bead store is
# the source of truth, so a failed write never changes the verdict.
record_live_resolution() {
    local bead_id="$1"
    local resolution_type="$2"

    local existing=$(get_resolution "$bead_id")
    if [[ -n "$existing" ]] && [[ "$existing" != "null" ]]; then
        return 0  # already cached
    fi

    mark_resolved "$bead_id" "$resolution_type" \
        "Live evaluation at check time (auto-backfilled cache)" \
        || log_resolution "WARN" "Could not backfill resolution cache for $bead_id"
}

# Check if crash is resolved.
#
# LIVE EVALUATION FIRST (gap D-1): `check` used to read only
# .beads/state/crash-resolutions.json — a ledger whose only writer is the
# (previously unreachable) FALSE_POSITIVE auto-mark, so it held 0 records and
# suppressed nothing, reporting NOT_RESOLVED for beads whose Status is Closed.
# The live store is now consulted directly:
#   1. bead closure           (bead show — closure never stales out, D-10)
#   2. VERIFIED work-completion marker (verify-work-completion.sh, gap G-9)
#   3. the ledger, as a cache of manual marks and FALSE_POSITIVE auto-marks —
#      the only records that expire (D-10: expiry is meaningful for
#      environment-level resolutions, not for closure)
is_resolved() {
    local bead_id="$1"

    local closure_status=$(check_bead_closure "$bead_id")
    if [[ "$closure_status" == "closed" ]]; then
        record_live_resolution "$bead_id" "bead_closure"
        return 0
    fi

    local marker_status=$(check_work_completion_marker "$bead_id")
    if [[ "$marker_status" == "verified_marker" ]]; then
        record_live_resolution "$bead_id" "task_completion"
        return 0
    fi

    local resolution=$(get_resolution "$bead_id")

    if [[ -z "$resolution" ]] || [[ "$resolution" == "null" ]]; then
        return 1  # Not resolved
    fi

    local resolution_type=$(echo "$resolution" | jq -r '.resolution_type // "manual"' 2>/dev/null || echo "manual")

    # A cached bead_closure record is never the deciding factor while the live
    # store answers: closure is re-evaluated above on every check (D-10 — it
    # cannot stale out), so reaching the ledger for a closure means the bead is
    # no longer closed. If the store says so outright (reopened for another
    # pass — a shape this fleet produces), a past closure must not suppress the
    # live crash. Only an unreadable store defers to the last observed closure.
    if [[ "$resolution_type" == "bead_closure" ]]; then
        if [[ "$closure_status" == "open" ]]; then
            return 1
        fi
        return 0
    fi

    # A cached task_completion record must not outlive the live marker it was
    # derived from. record_live_resolution backfills one at check time from the
    # then-VERIFIED marker, so a marker later re-verified to FAILED would
    # otherwise keep the crash "resolved" forever off that stale record — a
    # live negative signal has to veto it. No marker at all leaves the record
    # standing: mark-resolved's exit-code-0 path (check_task_completion) never
    # creates one, and there is nothing live to contradict it with.
    if [[ "$resolution_type" == "task_completion" && "$marker_status" != "no_marker" ]]; then
        log_resolution "INFO" "Work-completion marker for $bead_id is '$marker_status' - cached task_completion resolution withdrawn"
        return 1
    fi

    # Check if resolution record has expired
    local resolved_at=$(echo "$resolution" | jq -r '.resolved_at // empty' 2>/dev/null)
    if [[ -n "$resolved_at" ]]; then
        local resolved_secs=$(date -d "$resolved_at" +%s 2>/dev/null || echo "0")
        local current_secs=$(date +%s)
        local age_days=$(( (current_secs - resolved_secs) / 86400 ))

        if [[ $age_days -gt $RESOLUTION_AGE_DAYS ]]; then
            log_resolution "INFO" "Resolution record expired for $bead_id (age: ${age_days} days)"
            return 1  # Expired = not resolved
        fi
    fi

    return 0  # Resolved
}

# Mark a crash as resolved
mark_resolved() {
    local bead_id="$1"
    local resolution_type="${2:-auto}"
    local reason="${3:-}"

    init_state_file

    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    # Update state file using jq with individual arguments
    jq --arg id "$bead_id" \
       --arg timestamp "$timestamp" \
       --arg resolution_type "$resolution_type" \
       --arg reason "$reason" \
       '.resolutions[$id] = {
           "bead_id": $id,
           "resolved_at": $timestamp,
           "resolution_type": $resolution_type,
           "reason": $reason,
           "verified": true
         } | .metadata.last_updated = $timestamp' \
       "$RESOLUTION_STATE_FILE" > "$RESOLUTION_STATE_FILE.tmp"

    mv "$RESOLUTION_STATE_FILE.tmp" "$RESOLUTION_STATE_FILE"

    log_resolution "INFO" "Marked bead $bead_id as resolved (type: $resolution_type)"
}

# Mark a crash as unresolved
mark_unresolved() {
    local bead_id="$1"
    local reason="${2:-}"

    init_state_file

    local timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    # Remove resolution record
    jq --arg id "$bead_id" \
       '.resolutions |= del(.[$id]) | .metadata.last_updated = "'"$timestamp"'"' \
       "$RESOLUTION_STATE_FILE" > "$RESOLUTION_STATE_FILE.tmp"

    mv "$RESOLUTION_STATE_FILE.tmp" "$RESOLUTION_STATE_FILE"

    log_resolution "INFO" "Marked bead $bead_id as unresolved (reason: $reason)"
}

# Show resolution state for a bead
show_state() {
    local bead_id="$1"

    local resolution=$(get_resolution "$bead_id")

    if [[ -z "$resolution" ]] || [[ "$resolution" == "null" ]]; then
        echo "No resolution state found for bead: $bead_id"
        return 0
    fi

    echo "Resolution state for bead: $bead_id"
    echo "$resolution" | jq -r '.'
}

# List all resolved crashes
list_resolved() {
    if [[ ! -f "$RESOLUTION_STATE_FILE" ]]; then
        echo "No resolution state file found"
        return 0
    fi

    echo "Resolved crashes:"
    echo ""

    jq -r '.resolutions | to_entries[] |
           "- \(.key): \(.value.resolution_type) at \(.value.resolved_at)"' \
       "$RESOLUTION_STATE_FILE" 2>/dev/null || echo "No resolved crashes found"
}

# Clean up old resolution records
cleanup() {
    if [[ ! -f "$RESOLUTION_STATE_FILE" ]]; then
        echo "No resolution state file found"
        return 0
    fi

    local current_secs=$(date +%s)
    local cutoff_secs=$((current_secs - (RESOLUTION_AGE_DAYS * 86400)))

    # Filter out expired records
    jq --argjson cutoff $cutoff_secs \
       '.resolutions |= with_entries(select(
           (.value.resolved_at | fromdateiso8601) >= $cutoff
       ))' \
       "$RESOLUTION_STATE_FILE" > "$RESOLUTION_STATE_FILE.tmp"

    mv "$RESOLUTION_STATE_FILE.tmp" "$RESOLUTION_STATE_FILE"

    log_resolution "INFO" "Cleaned up expired resolution records"
}

# Main action
main() {
    # Initialize state file for all operations
    init_state_file

    # Handle help flag first (before parsing bead_id)
    if [[ "${1:-}" == "-h" ]] || [[ "${1:-}" == "--help" ]]; then
        show_usage
        exit 0
    fi

    local bead_id="${1:-}"
    local action="${2:-check}"

    # Validate bead_id for most actions
    if [[ "$action" != "list-resolved" ]] && [[ "$action" != "cleanup" ]] && [[ -z "$bead_id" ]]; then
        echo "ERROR: Bead ID required for action: $action"
        show_usage
        exit 2
    fi

    case "$action" in
        check)
            if is_resolved "$bead_id"; then
                echo "RESOLVED"
                get_resolution "$bead_id" | jq -r '.'
                exit 0
            else
                echo "NOT_RESOLVED"
                exit 1
            fi
            ;;

        mark-resolved)
            # Auto-determine resolution type
            log_resolution "INFO" "Determining resolution type for bead: $bead_id"
            local resolution_type=$(determine_resolution_type "$bead_id")
            local reason="Auto-detected resolution: $resolution_type"

            mark_resolved "$bead_id" "$resolution_type" "$reason"
            exit 0
            ;;

        mark-unresolved)
            mark_unresolved "$bead_id" "Manually marked unresolved"
            exit 0
            ;;

        show-state)
            show_state "$bead_id"
            exit 0
            ;;

        list-resolved)
            list_resolved
            exit 0
            ;;

        cleanup)
            cleanup
            exit 0
            ;;

        *)
            echo "ERROR: Unknown action: $action"
            show_usage
            exit 2
            ;;
    esac
}

main "$@"
