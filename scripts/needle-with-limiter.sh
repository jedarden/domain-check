#!/usr/bin/env bash
# Needle wrapper with concurrency limiting AND the crash-storm circuit breaker
# gate — the repo-side dispatch path.
#
# Two commands add agent dispatch load to this box: `needle run` and
# `needle supervise`. Both are gated here, in this order, before any worker
# launches:
#
#   1. crash-storm circuit breaker gate   (this file, dispatch_breaker_gate)
#   2. agent concurrency limiter          (agent-concurrency-limiter.sh, unchanged)
#
# ── Why the breaker is wired HERE ────────────────────────────────────────────
#
# NEEDLE's internal release-and-retry loop is what turned bf-65lsdu into 127
# identical doomed dispatches in 2.5 h
# (docs/research/root-cause-analysis-bf-65lsdu-signal-minus-one-2026-09-02.md
# §7: "the dispatcher's release-and-retry loop has no crash-storm breaker").
# That loop lives in the NEEDLE repo (design doc §9.2 Phase 4, G-13 — an
# external ask); it cannot be patched from here. But its re-dispatch fuel is
# the shared bead store's ready frontier, and a bead that is `deferred` in the
# store cannot be claimed by ANY worker through ANY path. So the gate here
# enforces the breaker's decision at the one place it binds:
#
#   breaker OPEN, cooldown not elapsed -> the bead is DEFERRED out of the
#                                         ready frontier: not re-dispatchable
#                                         until retry_after, whatever loop
#                                         asks for it
#   breaker OPEN, cooldown elapsed     -> ONE probe dispatch (half-open): the
#                                         bead is re-opened so the frontier
#                                         can serve it once; the atomic claim
#                                         means at most one worker holds it,
#                                         and its outcome either closes the
#                                         breaker (exit 0) or re-trips it with
#                                         doubled backoff (exit -1/137)
#   breaker closed / counting          -> dispatch proceeds untouched
#
# Semantics come from scripts/crash-circuit-breaker.sh and are not re-implemented
# here: trip at BREAKER_THRESHOLD (3) consecutive infrastructure crashes,
# doubling cooldown capped at BREAKER_MAX_COOLDOWN (4 h), 24 h counter decay,
# and — per RCA §3 — only exit -1 (NEEDLE "died without exiting") and 137
# (memcg SIGKILL) count as breaker crashes. Timeouts (124) and workflow
# failures (exit 1) are NOT breaker crashes; they stay owned by
# retry-with-backoff.sh and crash-classifier.sh.
#
# ── Division of labor (do not blur) ──────────────────────────────────────────
#
#   THIS WRAPPER   reads breaker state and enforces it on the store. It never
#                  records outcomes: a dispatch is `needle run` forking workers
#                  and returning, so per-bead exit codes are not visible here.
#   crash-alert-manager.sh
#                  writes breaker state (`record`) when it processes a crash,
#                  and defers on trip. Both layers resolve the same
#                  scripts/crash-circuit-breaker.sh, hence the same state file
#                  (.beads/logs/circuit-breaker-state.json) — that shared file
#                  is the whole integration.
#   crash-circuit-breaker.sh --rebuild
#                  bootstraps state from .beads/events.jsonl after a storm or
#                  on a fresh clone.
#
# ── Reconciliation (both directions, so the gate cannot strand work) ─────────
#
# A deferral is only held while the breaker says so. The breaker's own `defer`
# marks the bead's Notes with "circuit breaker:" — that marker is the only
# proof a deferral was ours, so:
#
#   - a bead a HUMAN deferred carries no marker and is never touched here;
#   - if breaker state is cleared (reset / reset-all / 24 h decay / lost state
#     file) while a bead is still deferred, the next dispatch sweep re-opens
#     it — a tripped bead cannot be stranded out of the frontier by a lost
#     state file.
#
# ── Safety: fail-open by construction ────────────────────────────────────────
#
# The gate may never take the fleet down with it. If the breaker script is
# missing or not executable, or `check` exits with anything other than its
# documented 0 (allow) / 4 (blocked), the sweep logs a warning and dispatch
# proceeds. If the `bead` CLI is unavailable, store mutations are skipped and
# the sweep degrades to log-only. Set BREAKER_DISPATCH_ENFORCE=0 for an
# advisory run: BLOCKED/PROBE are logged, nothing is deferred or re-opened.
#
# ── Usage ─────────────────────────────────────────────────────────────────────
#
#   ./needle-with-limiter.sh run [needle run args...]
#       Gated dispatch (breaker sweep, then concurrency limiter, then needle).
#   ./needle-with-limiter.sh supervise [args...]
#       Same gates for the fleet supervisor.
#   ./needle-with-limiter.sh breaker-status [bead-id]
#   ./needle-with-limiter.sh breaker-rebuild
#   ./needle-with-limiter.sh breaker-reset <bead-id>
#       Operator passthroughs to the breaker; the reset is the "root cause
#       fixed, release this bead" lever. Anything else is passed to needle.
#
#   Environment: BREAKER_SCRIPT (default: sibling crash-circuit-breaker.sh),
#   BREAKER_STATE_FILE (default: <repo>/.beads/logs/circuit-breaker-state.json),
#   BREAKER_DISPATCH_ENFORCE (default: 1), plus every AGENT_* limiter variable
#   documented in agent-concurrency-limiter.sh.
#
# Example:
#   ./needle-with-limiter.sh run --workspace /home/coding/domain-check \
#       --agent claude-code-glm-4.7 --count 1

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIMITER_SCRIPT="$SCRIPT_DIR/agent-concurrency-limiter.sh"
BREAKER_SCRIPT="${BREAKER_SCRIPT:-$SCRIPT_DIR/crash-circuit-breaker.sh}"
BREAKER_DISPATCH_ENFORCE="${BREAKER_DISPATCH_ENFORCE:-1}"
# Same default the breaker itself resolves, so both layers always agree on
# which state file is authoritative.
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BREAKER_STATE_FILE="${BREAKER_STATE_FILE:-$PROJECT_ROOT/.beads/logs/circuit-breaker-state.json}"
# Marker crash-circuit-breaker.sh writes into a bead's Notes when IT defers it.
BREAKER_NOTE_MARKER="${BREAKER_NOTE_MARKER:-circuit breaker:}"

log_wrap() {
    echo "[$(date -Iseconds)] [needle-wrapper] $*" >&2
}

# ── Store helpers ─────────────────────────────────────────────────────────────
# Every store read is a `bead` CLI call; every helper fails soft (empty output,
# rc 0) so a store hiccup degrades to "skip this bead", never to a broken
# dispatch.

store_status() { # $1 = bead-id -> Open|InProgress|Deferred|Closed|"" (unknown)
    local out
    if ! out=$(bead show "$1" 2>/dev/null); then
        echo ""
        return 0
    fi
    awk '/^Status:/ {print $2; exit}' <<< "$out"
}

store_deferred_by_breaker() { # $1 = bead-id -> 0 only if OUR marker is present
    local out
    if ! out=$(bead show "$1" 2>/dev/null); then
        return 1
    fi
    # Substring test, not `grep -q`: grep's early exit can SIGPIPE an upstream
    # stage under pipefail and flip a true result into a false one (the same
    # hazard verify-changes.sh documents).
    [[ "$out" == *"$BREAKER_NOTE_MARKER"* ]]
}

# ── Breaker gate ──────────────────────────────────────────────────────────────

gate_one_bead() { # $1 = bead-id, $2 = breaker state, $3 = consecutive crashes
    local bead_id="$1" breaker_state="$2" consecutive="$3"
    local check_out check_rc status line

    if [[ "$breaker_state" != "open" ]]; then
        # Counting toward the threshold (or a residual closed entry) — no gate,
        # but say so: 1-2 consecutive kills on one bead is the early part of
        # the storm shape.
        if [[ "$consecutive" != "0" ]]; then
            log_wrap "Breaker counting for $bead_id: $consecutive consecutive infrastructure crash(es), dispatch proceeds"
        fi
        return 0
    fi

    # Capture the breaker's exit code directly: `if ! cmd; then rc=$?` reads the
    # NEGATED pipeline status (always 0 in the then-branch), which would fold
    # BLOCKED (4) into the allow path and make the enforcement case below dead
    # code. `|| rc=$?` preserves the breaker's own verdict.
    check_rc=0
    check_out=$("$BREAKER_SCRIPT" check "$bead_id" 2>&1) || check_rc=$?

    case "$check_rc" in
        0)
            if [[ "$check_out" != *"half-open"* ]]; then
                return 0 # closed entry the sweep has nothing to do for
            fi
            log_wrap "Breaker half-open for $bead_id: cooldown elapsed, ONE probe dispatch allowed"
            [[ "$BREAKER_DISPATCH_ENFORCE" == "1" ]] || {
                log_wrap "  advisory mode (BREAKER_DISPATCH_ENFORCE=0): no store mutation"
                return 0
            }
            status=$(store_status "$bead_id")
            if [[ "$status" != "Deferred" ]]; then
                log_wrap "  $bead_id is ${status:-unreadable} - probe dispatch can proceed without a store change"
                return 0
            fi
            if ! store_deferred_by_breaker "$bead_id"; then
                log_wrap "  $bead_id is Deferred without a breaker note - left alone (not ours to release)"
                return 0
            fi
            if bead update "$bead_id" --status open >/dev/null 2>&1; then
                log_wrap "  re-opened $bead_id for its probe dispatch (was deferred by the breaker)"
            else
                log_wrap "  WARN: could not re-open $bead_id ('bead update' failed) - no probe dispatch until it is re-opened"
            fi
            ;;
        4)
            log_wrap "Breaker OPEN for $bead_id: consecutive infrastructure crashes - re-dispatch blocked"
            while IFS= read -r line; do log_wrap "  $line"; done <<< "$check_out"
            [[ "$BREAKER_DISPATCH_ENFORCE" == "1" ]] || {
                log_wrap "  advisory mode (BREAKER_DISPATCH_ENFORCE=0): no store mutation"
                return 0
            }
            status=$(store_status "$bead_id")
            case "$status" in
                Open)
                    # The enforcement step: a deferred bead is invisible to the
                    # ready frontier, so no worker can claim it until the
                    # cooldown elapses and this sweep re-opens it.
                    if "$BREAKER_SCRIPT" defer "$bead_id" >/dev/null 2>&1; then
                        log_wrap "  deferred $bead_id out of the ready frontier until retry_after"
                    else
                        log_wrap "  WARN: could not defer $bead_id (breaker defer failed) - it stays claimable, gate UNENFORCED"
                    fi
                    ;;
                Deferred)
                    log_wrap "  $bead_id already deferred - holding"
                    ;;
                InProgress)
                    log_wrap "  $bead_id is InProgress (dispatch already in flight) - the breaker gates its next release"
                    ;;
                Closed)
                    log_wrap "  $bead_id is Closed - nothing to gate; stale breaker entry, consider 'breaker-reset $bead_id'"
                    ;;
                *)
                    log_wrap "  WARN: cannot read $bead_id in the store - enforcement skipped for this bead"
                    ;;
            esac
            ;;
        *)
            # Anything that is not the breaker's documented 0/4 is a broken
            # helper, not a gate verdict: fail OPEN.
            log_wrap "WARN: breaker check for $bead_id exited $check_rc (expected 0 or 4) - failing OPEN, dispatch proceeds"
            ;;
    esac
    return 0
}

dispatch_breaker_gate() {
    if [[ ! -x "$BREAKER_SCRIPT" ]]; then
        log_wrap "Breaker gate: $BREAKER_SCRIPT missing or not executable - no crash-storm gating on this dispatch (fail-open)"
        return 0
    fi

    local entries
    # Read-only enumeration; every decision and every mutation still goes
    # through the breaker's own check/defer, so the state machine stays in one
    # place.
    if ! entries=$(jq -r '.beads // {} | to_entries[]
            | [.key, (.value.state // "closed"), (.value.consecutive_crashes // 0)] | @tsv' \
            "$BREAKER_STATE_FILE" 2>/dev/null); then
        entries=""
    fi

    if [[ -z "$entries" ]]; then
        log_wrap "Breaker gate: no beads held down - dispatch proceeds"
        return 0
    fi

    local bead_id breaker_state consecutive
    while IFS=$'\t' read -r bead_id breaker_state consecutive; do
        [[ -n "$bead_id" ]] || continue
        gate_one_bead "$bead_id" "$breaker_state" "$consecutive"
    done <<< "$entries"
    return 0
}

# ── Operator passthroughs (no dispatch, no limiter slot) ─────────────────────

case "${1:-}" in
    breaker-status)
        shift
        exec "$BREAKER_SCRIPT" status "$@"
        ;;
    breaker-rebuild)
        exec "$BREAKER_SCRIPT" --rebuild
        ;;
    breaker-reset)
        [[ $# -eq 2 ]] || { log_wrap "usage: $0 breaker-reset <bead-id>" >&2; exit 2; }
        exec "$BREAKER_SCRIPT" reset "$2"
        ;;
esac

# ── Gated dispatch ────────────────────────────────────────────────────────────

case "${1:-}" in
    run|supervise)
        dispatch_breaker_gate
        ;;
esac

# Source the limiter to get its functions
# shellcheck source=agent-concurrency-limiter.sh
source "$LIMITER_SCRIPT"

# Check if we need to wait for a slot
if ! agent_limit_start; then
    echo "[$(date -Iseconds)] [needle-wrapper] Agent could not obtain execution slot" >&2
    echo "[$(date -Iseconds)] [needle-wrapper] This usually means the system is overloaded and queue timeout expired" >&2
    echo "[$(date -Iseconds)] [needle-wrapper] Consider increasing AGENT_QUEUE_TIMEOUT or reducing system load" >&2
    exit 1
fi

# Set cleanup trap
trap agent_limit_cleanup EXIT

# Execute needle with all arguments
echo "[$(date -Iseconds)] [needle-wrapper] Executing: needle $*" >&2
exec needle "$@"
