#!/usr/bin/env bash
# Alert Triage Sweep — fleet-wide, report-only triage of open crash alerts.
#
# Purpose (gap analysis §5 P1(a)/P4,
# docs/alert-deduplication-gap-analysis-2026-09-07.md): `alert-deduplication.sh
# check <bead>` answers one alert at a time, and nothing in this repo ran any
# of it automatically until 2026-09-07 — the gate existed only as a tool a
# worker had to know about. This sweep is the automated pass: it walks every
# OPEN alert bead in the store, resolves each crash TARGET through
# crash-resolution-tracker.sh (the single authority for "is this crash
# resolved"), and writes a triage queue that closure-bead blockers, agents,
# and humans drain.
#
#   IT NEVER CLOSES, UPDATES, OR CREATES A BEAD. Closure stays with each
#   alert's closure-bead blocker (standing convention — the alert-layer
#   workers who own an alert's end state); this script only produces the
#   verdicts they act on. Every `bead` invocation it makes is read-only
#   (`list`, `show` via the tracker).
#
# Verdicts (one per open alert bead, written to the queue):
#   RESOLVED_TARGET    the crash target is resolved (closure / VERIFIED marker /
#                      ledger) — close candidate for its closure-bead blocker
#   ORPHANED_TARGET    the crash target no longer exists in the store
#   FANOUT_KEEPER      target still unresolved, and this is the alert that
#                      should carry the investigation (oldest of the open
#                      alerts aimed at that target)
#   FANOUT_DUPLICATE   target still unresolved, but an older open alert already
#                      carries it — duplicate of the keeper
#   NEEDS_REVIEW       target still unresolved and this is the only open alert
#                      for it — the only verdict that deserves fresh
#                      investigation effort
#
# Relationship to the gate: the gate's verdict for a FANOUT_DUPLICATE is the
# same DUPLICATE its step 3 produces (an open alert covers the target), and a
# RESOLVED_TARGET is the gate's step-1/step-2 DUPLICATE — by construction,
# because both derive the target and the resolution the same way. The sweep
# adds what the gate cannot say per-alert: which sibling to keep.
#
# Exit codes:
#   0  sweep ran, queue written (findings are in the queue/log, not the code)
#   2  usage error
#   3  bead store unreadable — fail open, nothing written
#
# Usage:
#   alert-triage-sweep.sh [--json] [--help]
#
# Environment:
#   BEAD_SCAN_LIMIT   bead list --limit (default: 999999)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BEAD_DIR="$PROJECT_ROOT/.beads"
LOG_DIR="$BEAD_DIR/logs"
STATE_DIR="$BEAD_DIR/state/alert-triage"
QUEUE_FILE="$STATE_DIR/queue.jsonl"
SWEEP_LOG="$LOG_DIR/alert-triage.log"
RESOLUTION_TRACKER="$SCRIPT_DIR/crash-resolution-tracker.sh"

EXIT_OK=0
EXIT_USAGE=2
EXIT_STORE_UNREADABLE=3

BEAD_SCAN_LIMIT="${BEAD_SCAN_LIMIT:-999999}"

show_usage() {
    cat <<EOF
Usage: $0 [--json]

Walk every open/in-progress alert bead in the bead store, resolve each crash
target through crash-resolution-tracker.sh, and write the triage queue to
$QUEUE_FILE

Report-only: it never closes, updates, or creates a bead. Closure stays with
each alert's closure-bead blocker.

  --json   print the queue records (JSONL) to stdout instead of the summary
  -h, --help

Exit codes:
  0  sweep ran (findings live in the queue and the log)
  2  usage error
  3  bead store unreadable — fail open, nothing written

Environment:
  BEAD_SCAN_LIMIT   bead list --limit (default: $BEAD_SCAN_LIMIT)
EOF
}

log_sweep() {
    echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] $*" >>"$SWEEP_LOG"
}

# Scan analysis. stdin: `bead list --json` JSONL. stdout: one JSON object per
# open alert bead — id, target, target_status, created_at, keeper, siblings.
# Target derivation and the alert-shape predicate are the gate's own
# (alert-deduplication.sh SCAN_PY), so the two tools cannot disagree about
# what an alert is or which crash it points at.
SCAN_PY=$(cat <<'PY'
import json
import re
import sys

rows = {}
for line in sys.stdin:
    line = line.strip()
    if not line:
        continue
    try:
        rec = json.loads(line)
    except ValueError:
        continue
    if rec.get("id"):
        rows[rec["id"]] = rec

CRASH_REF = re.compile(r"(?:on|of)\s+(?:bead\s+)?((?:bf|domchk)-[a-z0-9]+)", re.I)
ANY_REF = re.compile(r"(?:bf|domchk)-[a-z0-9]+", re.I)


def is_alert(rec):
    title = rec.get("title") or ""
    labels = rec.get("labels") or []
    return title.upper().startswith("ALERT:") or ("alert" in labels)


def target_of(rec):
    """The crash a bead is about; falls back to the bead itself."""
    self_id = (rec.get("id") or "").lower()
    title = rec.get("title") or ""
    m = CRASH_REF.search(title)
    if m:
        return m.group(1)
    refs = [t for t in ANY_REF.findall(title) if t.lower() != self_id]
    if refs:
        return refs[-1]
    return rec.get("id")


alerts = [rec for rec in rows.values() if is_alert(rec)
          and (rec.get("status") or "").lower() in ("open", "in_progress")]

# Group open alerts by target, then pick the keeper of each group: the oldest
# created_at, ID order breaking ties (the original alert, deterministically).
groups = {}
for rec in alerts:
    groups.setdefault(target_of(rec), []).append(rec)

for target, members in groups.items():
    members.sort(key=lambda r: ((r.get("created_at") or ""), r.get("id") or ""))
    keeper = members[0][ "id" ]
    tgt = rows.get(target)
    target_status = tgt.get("status", "absent") if tgt else "absent"
    for rec in members:
        print(json.dumps({
            "alert": rec.get("id"),
            "target": target,
            "target_status": target_status,
            "created_at": rec.get("created_at") or "",
            "keeper": keeper,
            "siblings": len(members),
        }))
PY
)

# Resolve one target through the single authority. stdout: "<resolved|not> <type>"
# — never let a tracker failure read as "resolved" (fail-open discipline: a
# triage tool must not manufacture close candidates out of its own breakage).
resolve_target() {
    local target="$1" out type
    if [[ ! -x "$RESOLUTION_TRACKER" ]]; then
        echo "not tracker_missing"
        return 0
    fi
    out=$("$RESOLUTION_TRACKER" "$target" check 2>/dev/null || true)
    if grep -q '^RESOLVED' <<<"$out"; then
        type=$(grep -o '"resolution_type": *"[^"]*"' <<<"$out" | head -1 | sed 's/.*: *"\([^"]*\)"/\1/')
        echo "resolved ${type:-unknown}"
    elif grep -q '^NOT_RESOLVED' <<<"$out"; then
        echo "not unresolved"
    else
        echo "not tracker_error"
    fi
}

sweep() {
    mkdir -p "$STATE_DIR" "$LOG_DIR"

    local scan
    scan=$(bead list --json --limit "$BEAD_SCAN_LIMIT" 2>/dev/null || true)
    if [[ -z "$scan" ]]; then
        echo "INDETERMINATE: bead store unreadable - nothing swept, nothing written"
        log_sweep "WARN" "sweep: bead scan unavailable - fail open"
        return "$EXIT_STORE_UNREADABLE"
    fi

    local rows
    if ! rows=$(printf '%s\n' "$scan" | python3 -c "$SCAN_PY" 2>&1); then
        echo "INDETERMINATE: scan analysis failed - nothing swept, nothing written"
        log_sweep "WARN" "sweep: analysis error: $rows"
        return "$EXIT_STORE_UNREADABLE"
    fi
    local -a rows_arr=()
    mapfile -t rows_arr <<<"$rows"
    # An empty scan analysis emits a single empty line — that is zero alerts.
    [[ ${#rows_arr[@]} -eq 1 && -z "${rows_arr[0]}" ]] && rows_arr=()

    # One resolution verdict per DISTINCT target: a target with 55 open alerts
    # costs one tracker call, not 55.
    local -A target_verdict=()
    local target resolved type
    while IFS=$'\t' read -r target _; do
        [[ -n "$target" && -z "${target_verdict[$target]:-}" ]] || continue
        read -r resolved type <<<"$(resolve_target "$target")"
        target_verdict["$target"]="$resolved ${type:-unknown}"
    done < <(jq -r '[.target] | @tsv' <<<"$rows")

    # Delta against the previous sweep: how many alerts are NEW close
    # candidates — the number that should move over time if the gate upstream
    # is working, and that should reach zero once the backlog is drained.
    local -A previous=()
    if [[ -f "$QUEUE_FILE" ]]; then
        while IFS=$'\t' read -r alert verdict; do
            previous["$alert"]="$verdict"
        done < <(jq -r 'select(.verdict != null) | [.alert, .verdict] | @tsv' "$QUEUE_FILE" 2>/dev/null || true)
    fi

    local queue_tmp
    queue_tmp=$(mktemp "$STATE_DIR/queue.jsonl.tmp.XXXXXX")
    : >"$queue_tmp"

    local -A counts=( [RESOLVED_TARGET]=0 [ORPHANED_TARGET]=0 [FANOUT_KEEPER]=0 [FANOUT_DUPLICATE]=0 [NEEDS_REVIEW]=0 )
    local new_close_candidates=0
    local swept_at
    swept_at=$(date -u +"%Y-%m-%dT%H:%M:%SZ")

    local verdict evidence
    while IFS=$'\t' read -r alert target target_status created keeper siblings; do
        resolved="${target_verdict[$target]:-not tracker_error}"
        read -r resolved type <<<"$resolved"

        case "$target_status" in
            absent)
                verdict=ORPHANED_TARGET
                evidence='{"reason":"target_absent_from_store"}'
                ;;
            *)
                if [[ "$resolved" == "resolved" ]]; then
                    verdict=RESOLVED_TARGET
                    evidence="{\"resolution_type\":\"${type}\",\"target_status\":\"${target_status}\"}"
                elif [[ "$siblings" -gt 1 ]]; then
                    if [[ "$alert" == "$keeper" ]]; then
                        verdict=FANOUT_KEEPER
                        evidence="{\"sibling_alerts\":${siblings}}"
                    else
                        verdict=FANOUT_DUPLICATE
                        evidence="{\"keeper\":\"${keeper}\",\"sibling_alerts\":${siblings}}"
                    fi
                else
                    verdict=NEEDS_REVIEW
                    evidence="{\"target_status\":\"${target_status}\"}"
                fi
                ;;
        esac

        counts["$verdict"]=$(( ${counts["$verdict"]} + 1 ))
        if [[ "$verdict" == "RESOLVED_TARGET" && "${previous[$alert]:-}" != "RESOLVED_TARGET" ]]; then
            new_close_candidates=$((new_close_candidates + 1))
        fi

        printf '{"alert":"%s","target":"%s","verdict":"%s","evidence":%s,"created_at":"%s","siblings":%s,"swept_at":"%s"}\n' \
            "$alert" "$target" "$verdict" "$evidence" "$created" "$siblings" "$swept_at" >>"$queue_tmp"
    done < <(jq -r '[.alert, .target, .target_status, (.created_at // ""), .keeper, .siblings] | @tsv' <<<"$rows")

    # Atomic replace so a reader never sees a half-written queue.
    mv "$queue_tmp" "$QUEUE_FILE"

    if [[ "${JSON_OUT:-0}" == "1" ]]; then
        cat "$QUEUE_FILE"
    else
        echo "=== Alert Triage Sweep ($swept_at) ==="
        echo "Open alerts: ${#rows_arr[@]} across ${#target_verdict[@]} targets"
        echo "  RESOLVED_TARGET   ${counts[RESOLVED_TARGET]}  (close candidates for their closure-bead blockers)"
        echo "  ORPHANED_TARGET   ${counts[ORPHANED_TARGET]}"
        echo "  FANOUT_KEEPER     ${counts[FANOUT_KEEPER]}"
        echo "  FANOUT_DUPLICATE  ${counts[FANOUT_DUPLICATE]}"
        echo "  NEEDS_REVIEW      ${counts[NEEDS_REVIEW]}"
        echo "New close candidates since the previous sweep: $new_close_candidates"
        echo "Queue: $QUEUE_FILE"
    fi

    log_sweep "INFO" "sweep: alerts=${#rows_arr[@]} targets=${#target_verdict[@]} " \
        "resolved=${counts[RESOLVED_TARGET]} orphaned=${counts[ORPHANED_TARGET]} " \
        "keeper=${counts[FANOUT_KEEPER]} fanout_dup=${counts[FANOUT_DUPLICATE]} " \
        "needs_review=${counts[NEEDS_REVIEW]} new_close_candidates=$new_close_candidates"
    return "$EXIT_OK"
}

main() {
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --json) JSON_OUT=1; shift ;;
            -h|--help) show_usage; exit "$EXIT_OK" ;;
            *) echo "ERROR: unknown option: $1"; show_usage; exit "$EXIT_USAGE" ;;
        esac
    done
    sweep
}

main "$@"
