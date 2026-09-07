#!/usr/bin/env bash
# Alert Deduplication for Domain Check Crashes
# Purpose: decide whether a crash alert is a duplicate of an already-resolved
#          or already-covered crash, so resolved crashes stop generating new
#          investigation beads.
# Created: 2026-09-02; contract rewritten 2026-09-07 (domchk-b3de301f) per
#          docs/alert-deduplication-gap-analysis-2026-09-07.md (D-2/D-4/D-6).
#
# Modes:
#   check <bead-id>   Per-alert gate. The mode callers should use.
#   report            Fleet-wide report over .beads/events.jsonl (the default
#                     when no arguments are given; retained for
#                     crash-alert-manager.sh's legacy no-argument wiring).
#
# Exit contract for `check`:
#   0  DUPLICATE   suppress — the crash target is already resolved, or an open
#                  alert already covers it
#   1  UNIQUE      legitimate alert — proceed
#   2  USAGE       bad arguments
#   3  INDETERMINATE  cannot determine (bead store unreadable, bead unknown) —
#                  fail OPEN: callers proceed, crash alerting never depends on
#                  this gate being runnable
#
# Design notes (gap analysis §3):
#   * Every verdict is keyed on the CRASH TARGET bead, never on the alert-bead
#     instance (D-2) — duplicates arrive as fresh alert beads for the same
#     target, so an instance-keyed ledger is structurally blind to them.
#   * The target comes from the title, with no ^bf- gate on the alert bead's
#     own ID (D-4): needle's title is "ALERT: Agent crash on bead bf-XXXX" and
#     September-era alert bead IDs are domchk-*. Dependency edges are NOT used
#     as a target source — in this workspace they are prerequisite "blocker"
#     edges ({"blocker": ..., "kind": "blocks"}), not target pointers, and
#     alert beads carry none.
#   * All paths derive from BASH_SOURCE, not the caller's CWD (D-6).
#   * Resolution is decided by crash-resolution-tracker.sh (live bead closure,
#     verified work-completion marker, ledger cache) so there is one authority
#     for "is this crash resolved".

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"
BEAD_DIR="$PROJECT_ROOT/.beads"
LOG_DIR="$BEAD_DIR/logs"
ALERT_LOG="$LOG_DIR/alert-deduplication.log"
EVENTS_FILE="$BEAD_DIR/events.jsonl"
WORK_COMPLETION_DIR="$BEAD_DIR/state/work-completion"
RESOLUTION_TRACKER="$SCRIPT_DIR/crash-resolution-tracker.sh"

EXIT_DUPLICATE=0    # suppress
EXIT_UNIQUE=1       # proceed
EXIT_USAGE=2        # bad arguments
EXIT_UNKNOWN=3      # cannot determine — fail open

BEAD_SCAN_LIMIT="${BEAD_SCAN_LIMIT:-999999}"
REPORT_WINDOW_HOURS="${REPORT_WINDOW_HOURS:-24}"

mkdir -p "$LOG_DIR"

# Verdict carrier: functions always return 0 and record the outcome here, so
# `set -e` never fights the exit contract.
VERDICT=""

log_dedup() {
    echo "[$(date -u +"%Y-%m-%dT%H:%M:%SZ")] $*" >>"$ALERT_LOG"
}

show_usage() {
    cat <<EOF
Usage: $0 check <bead-id>
       $0 report [--window-hours N]

check <bead-id>
    Decide whether the alert/crash bead <bead-id> is a duplicate. The crash
    target is taken from the bead title (e.g. "ALERT: Agent crash on bead
    bf-XXXX"); the verdict is keyed on that target, not on this bead.

    Exit codes:
      0  DUPLICATE     suppress (target resolved, or an open alert covers it)
      1  UNIQUE        legitimate alert — proceed
      2  USAGE         bad arguments
      3  INDETERMINATE cannot determine — fail open (proceed)

report
    Fleet-wide report of repeat crash activity from .beads/events.jsonl (the
    append-only record) within --window-hours N (default: $REPORT_WINDOW_HOURS).
    Lines that begin with "DUPLICATE" mark per-target repeats; fleet-wide
    observations never use that word, so a consumer grepping for "duplicate"
    only ever sees genuine per-target repeats.

Environment:
    BEAD_SCAN_LIMIT      bead list --limit (default: $BEAD_SCAN_LIMIT)
    REPORT_WINDOW_HOURS  report window when --window-hours is not given
EOF
}

# Is there a VERIFIED work-completion marker for this bead? (G-9)
# verify-work-completion.sh writes result: VERIFIED | FAILED — only VERIFIED
# counts as resolution; a FAILED marker means the work did NOT hold up.
marker_verified() {
    local bead_id="$1"
    local marker="$WORK_COMPLETION_DIR/$bead_id.json"
    [[ -f "$marker" ]] || return 1
    local result
    result=$(jq -r '.result // "VERIFIED"' "$marker" 2>/dev/null || echo "VERIFIED")
    [[ "$result" == "VERIFIED" ]]
}

# Scan analysis. stdin: `bead list --json` JSONL; argv[1]: the bead to judge.
# stdout: FOUND / TARGET / TARGET_STATUS / OPEN_ALERTS lines.
SCAN_PY=$(cat <<'PY'
import json, re, sys

bead_id = sys.argv[1]
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

me = rows.get(bead_id)
if me is None:
    print("FOUND no")
    sys.exit(0)
print("FOUND yes")

# "ALERT: Agent crash on bead bf-XXXX" / "Investigate agent crash on bead X" /
# "Identify root cause of bf-YYYY agent crash"
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


target = target_of(me)
print("TARGET " + target)
tgt = rows.get(target)
print("TARGET_STATUS " + (tgt.get("status", "absent") if tgt else "absent"))

# Live investigations (open / in_progress alert beads) already covering this
# same crash target — the duplicate shape the fleet actually produces.
open_alerts = []
for rec in rows.values():
    rid = rec.get("id")
    if not rid or rid == bead_id:
        continue
    if (rec.get("status") or "").lower() not in ("open", "in_progress"):
        continue
    if not is_alert(rec):
        continue
    if target_of(rec) == target:
        open_alerts.append(rid)

print("OPEN_ALERTS " + (",".join(sorted(open_alerts)) if open_alerts else "-"))
PY
)

dedup_check() {
    local bead_id="$1"
    VERDICT=$EXIT_UNKNOWN

    local scan analysis
    scan=$(bead list --json --limit "$BEAD_SCAN_LIMIT" 2>/dev/null || true)
    if [[ -z "$scan" ]]; then
        echo "INDETERMINATE: bead store unreadable - failing open (proceed)"
        log_dedup "WARN" "check $bead_id: bead scan unavailable - fail open"
        return 0
    fi

    if ! analysis=$(printf '%s\n' "$scan" | python3 -c "$SCAN_PY" "$bead_id" 2>&1); then
        echo "INDETERMINATE: scan analysis failed - failing open (proceed)"
        log_dedup "WARN" "check $bead_id: analysis error: $analysis"
        return 0
    fi

    local found target target_status open_alerts
    found=$(awk '$1 == "FOUND" {print $2}' <<<"$analysis")
    if [[ "$found" != "yes" ]]; then
        echo "INDETERMINATE: bead $bead_id not found in store - failing open (proceed)"
        log_dedup "WARN" "check $bead_id: bead not found - fail open"
        return 0
    fi
    target=$(awk '$1 == "TARGET" {print $2}' <<<"$analysis")
    target_status=$(awk '$1 == "TARGET_STATUS" {print $2}' <<<"$analysis")
    open_alerts=$(awk '$1 == "OPEN_ALERTS" {print $2}' <<<"$analysis")

    # 1. Belt-and-braces, first because it is local and cheap: a VERIFIED
    #    work-completion marker for the target resolves the crash even when
    #    the resolution tracker itself is missing or not executable (G-9).
    #    Checking it before the tracker also lets the verdict cite its
    #    evidence instead of the tracker's generic "already resolved".
    if marker_verified "$target"; then
        echo "DUPLICATE: crash target $target has a VERIFIED work-completion marker"
        log_dedup "INFO" "check $bead_id: SUPPRESS (work-completion marker for $target)"
        VERDICT=$EXIT_DUPLICATE
        return 0
    fi

    # 2. Is the crash already resolved? One authority: the resolution tracker's
    #    live evaluation (bead closure, verified work-completion marker, ledger
    #    cache). Never expired for closure-based resolutions (D-10).
    if [[ -x "$RESOLUTION_TRACKER" ]]; then
        if "$RESOLUTION_TRACKER" "$target" check >/dev/null 2>&1; then
            echo "DUPLICATE: crash target $target is already resolved"
            log_dedup "INFO" "check $bead_id: SUPPRESS (resolved target $target)"
            VERDICT=$EXIT_DUPLICATE
            return 0
        fi
    fi

    # 3. An open alert already covers this same crash — the fan-out shape that
    #    produced 176 open alerts against closed targets (gap analysis §1).
    if [[ -n "$open_alerts" && "$open_alerts" != "-" ]]; then
        echo "DUPLICATE: open alert(s) already cover target $target: ${open_alerts//,/ }"
        log_dedup "INFO" "check $bead_id: SUPPRESS (open alerts cover $target: $open_alerts)"
        VERDICT=$EXIT_DUPLICATE
        return 0
    fi

    echo "PROCEED: no resolution and no open alert for target $target (status: $target_status)"
    log_dedup "INFO" "check $bead_id: PROCEED (target $target status $target_status, no open alerts)"
    VERDICT=$EXIT_UNIQUE
    return 0
}

# Fleet-wide report over the append-only crash record. Single-slot traces are
# the wrong source for crash history (one file per bead dir, overwritten every
# dispatch); events.jsonl keeps every crash/fail/timeout event.
REPORT_PY=$(cat <<'PY'
import collections
import datetime
import json
import os
import sys

path, window_hours = sys.argv[1], float(sys.argv[2])
cutoff = datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(hours=window_hours)


def parse_ts(value):
    if not value:
        return None
    try:
        return datetime.datetime.fromisoformat(value.replace("Z", "+00:00"))
    except ValueError:
        return None


per_bead = collections.Counter()
per_exit = collections.defaultdict(set)
total = 0

try:
    with open(path, encoding="utf-8", errors="replace") as handle:
        for line in handle:
            line = line.strip()
            if not line:
                continue
            try:
                rec = json.loads(line)
            except ValueError:
                continue
            if rec.get("event") not in ("crash", "fail", "timeout"):
                continue
            ts = parse_ts(rec.get("ts"))
            if ts is None or ts < cutoff:
                continue
            bead = rec.get("bead")
            if not bead:
                continue
            total += 1
            per_bead[bead] += 1
            per_exit[str(rec.get("exit_code"))].add(bead)
except FileNotFoundError:
    print("No crash event record found - no crashes to analyze")
    sys.exit(0)

window = f"last {window_hours:g}h"
if not per_bead:
    print(f"No crash-class events recorded in the {window}")
    sys.exit(0)

print(f"Crash-class events in the {window}: {total} across {len(per_bead)} beads")
for bead, count in per_bead.most_common():
    if count >= 3:
        print(f"DUPLICATE ALERT PATTERN: bead {bead} had {count} crash-class events in the {window}")
for code, beads in sorted(per_exit.items()):
    if len(beads) >= 5:
        print(f"FLEET-WIDE CRASH OBSERVATION: {len(beads)} distinct beads exited {code} "
              f"in the {window} - infrastructure event, not an alert-level repeat")
PY
)

dedup_report() {
    VERDICT=0
    local window_hours="${REPORT_WINDOW_HOURS:-24}"

    if [[ ! -f "$EVENTS_FILE" ]]; then
        echo "No crash event record found ($EVENTS_FILE) - no crashes to analyze"
        log_dedup "INFO" "report: no events file"
        return 0
    fi

    local report
    if ! report=$(python3 -c "$REPORT_PY" "$EVENTS_FILE" "$window_hours" 2>&1); then
        echo "Report analysis failed: $report"
        log_dedup "WARN" "report: analysis error: $report"
        return 0
    fi

    echo "=== Alert Deduplication Report (events.jsonl, last ${window_hours}h) ==="
    echo "$report"
    echo "=== End Report ==="
    log_dedup "INFO" "report: $(printf '%s\n' "$report" | grep -c 'DUPLICATE ALERT PATTERN' || true) repeat pattern(s) in window"
    return 0
}

main() {
    case "${1:-}" in
        check)
            [[ $# -ge 2 ]] || { echo "ERROR: check requires a bead ID"; show_usage; exit $EXIT_USAGE; }
            [[ "$2" =~ ^(bf|domchk)-[a-z0-9]+$ ]] || { echo "ERROR: not a bead ID: $2"; exit $EXIT_USAGE; }
            dedup_check "$2"
            exit "$VERDICT"
            ;;
        report)
            shift
            while [[ $# -gt 0 ]]; do
                case "$1" in
                    --window-hours)
                        [[ $# -ge 2 ]] || { echo "ERROR: --window-hours needs a value"; exit $EXIT_USAGE; }
                        REPORT_WINDOW_HOURS="$2"
                        shift 2
                        ;;
                    -h|--help) show_usage; exit 0 ;;
                    *) echo "ERROR: unknown option: $1"; show_usage; exit $EXIT_USAGE ;;
                esac
            done
            dedup_report
            exit "$VERDICT"
            ;;
        "")
            # No arguments = report: crash-alert-manager.sh's legacy wiring
            # invokes this script bare (`$DEDUPE_SCRIPT 2>&1`) and greps the
            # output, so report must stay the no-argument default.
            dedup_report
            exit "$VERDICT"
            ;;
        -h|--help)
            show_usage
            exit 0
            ;;
        *)
            echo "ERROR: unknown mode: $1"
            show_usage
            exit $EXIT_USAGE
            ;;
    esac
}

main "$@"
