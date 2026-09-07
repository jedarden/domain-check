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
#   record <bead-id> [--target ID] [--classification TYPE] [--crash-ts ISO]
#                     Append one crash-history entry (.beads/logs/
#                     crash-history.jsonl) for a generated alert: bead_id,
#                     crash_bead_id, crash_timestamp, classification,
#                     recorded_at. `check` leg 4 reads it back.
#   report            Fleet-wide report over .beads/events.jsonl (the default
#                     when no arguments are given; retained for
#                     crash-alert-manager.sh's legacy no-argument wiring).
#
# Exit contract for `check`:
#   0  DUPLICATE   suppress — the crash target is already resolved, an open
#                  alert already covers it, or an alert for the same crash
#                  target was recorded within the 7-day history window
#   1  UNIQUE      legitimate alert — proceed
#   2  USAGE       bad arguments
#   3  INDETERMINATE  cannot determine (bead store unreadable, bead unknown) —
#                  fail OPEN: callers proceed, crash alerting never depends on
#                  this gate being runnable
#
# Exit contract for `record`:
#   0  entry written (or this alert bead was already recorded — idempotent)
#   2  USAGE       bad arguments
#   3  entry could not be written
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
HISTORY_FILE="$LOG_DIR/crash-history.jsonl"
WORK_COMPLETION_DIR="$BEAD_DIR/state/work-completion"
RESOLUTION_TRACKER="$SCRIPT_DIR/crash-resolution-tracker.sh"

EXIT_DUPLICATE=0    # suppress
EXIT_UNIQUE=1       # proceed
EXIT_USAGE=2        # bad arguments
EXIT_UNKNOWN=3      # cannot determine — fail open

BEAD_SCAN_LIMIT="${BEAD_SCAN_LIMIT:-999999}"
REPORT_WINDOW_HOURS="${REPORT_WINDOW_HOURS:-24}"
# How long a recorded alert keeps suppressing fresh alerts for the same crash
# target. 7 days per the crash-response guide's deduplication window.
DEDUP_WINDOW_DAYS="${DEDUP_WINDOW_DAYS:-7}"

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
       $0 record <bead-id> [--target ID] [--classification TYPE] [--crash-ts ISO]
       $0 report [--window-hours N]

check <bead-id>
    Decide whether the alert/crash bead <bead-id> is a duplicate. The crash
    target is taken from the bead title (e.g. "ALERT: Agent crash on bead
    bf-XXXX"); the verdict is keyed on that target, not on this bead.

    Suppression reasons, in order: the target is resolved (live closure or a
    VERIFIED work-completion marker); an open alert already covers it; or an
    alert for the same crash target was recorded in the crash history
    ($HISTORY_FILE) within the last $DEDUP_WINDOW_DAYS days — in which case the
    earlier alert is named so callers can reference it.

    Exit codes:
      0  DUPLICATE     suppress (resolved / covered / recorded within window)
      1  UNIQUE        legitimate alert — proceed
      2  USAGE         bad arguments
      3  INDETERMINATE cannot determine — fail open (proceed)

record <bead-id>
    Append one crash-history entry for a GENERATED alert <bead-id>:
    bead_id, crash_bead_id, crash_timestamp, classification, recorded_at.
    --target supplies the crash bead when the caller already knows it;
    otherwise it is derived from the title. Re-recording the same alert bead
    is a no-op (idempotent). check leg 4 reads these entries back for the
    $DEDUP_WINDOW_DAYS-day duplicate window.

    Exit codes: 0 written (or already recorded), 2 usage, 3 could not write.

report
    Fleet-wide report of repeat crash activity from .beads/events.jsonl (the
    append-only record) within --window-hours N (default: $REPORT_WINDOW_HOURS).
    Lines that begin with "DUPLICATE" mark per-target repeats; fleet-wide
    observations never use that word, so a consumer grepping for "duplicate"
    only ever sees genuine per-target repeats.

Environment:
    BEAD_SCAN_LIMIT      bead list --limit (default: $BEAD_SCAN_LIMIT)
    REPORT_WINDOW_HOURS  report window when --window-hours is not given
    DEDUP_WINDOW_DAYS    check leg 4 window in days (default: $DEDUP_WINDOW_DAYS)
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

# Crash-history window scan (check leg 4). argv: history file, crash target,
# the alert bead being judged, window in days. A prior alert recorded for the
# SAME crash target inside the window suppresses this one — the shape behind
# "bf-2vtzg has 5 duplicate alerts": each retry re-alerted a crash another
# alert already covered, and no ledger remembered it.
HISTORY_PY=$(cat <<'PY'
import datetime
import json
import sys

path, target, self_id, window_days = sys.argv[1], sys.argv[2], sys.argv[3], float(sys.argv[4])
cutoff = datetime.datetime.now(datetime.timezone.utc) - datetime.timedelta(days=window_days)


def parse_ts(value):
    if not value:
        return None
    try:
        return datetime.datetime.fromisoformat(str(value).replace("Z", "+00:00"))
    except ValueError:
        return None


beads = []
first_when = ""
try:
    with open(path, encoding="utf-8", errors="replace") as handle:
        for line in handle:
            line = line.strip()
            if not line:
                continue
            try:
                rec = json.loads(line)
            except ValueError:
                continue  # a corrupt line never suppresses anything
            if rec.get("crash_bead_id") != target:
                continue
            bead = rec.get("bead_id")
            if not bead or bead == self_id:
                continue  # never treat an alert as a duplicate of itself
            ts = parse_ts(rec.get("recorded_at")) or parse_ts(rec.get("crash_timestamp"))
            if ts is None or ts < cutoff:
                continue
            when = rec.get("recorded_at") or rec.get("crash_timestamp") or "unknown"
            if bead not in beads:
                beads.append(bead)
            if not first_when:
                first_when = str(when)
except FileNotFoundError:
    sys.exit(0)  # no history yet — nothing to suppress on

if beads:
    print("HISTORY_MATCH " + ",".join(sorted(beads)))
    print("HISTORY_WHEN " + first_when)
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

    # 4. Crash-history window: an alert for this same crash target recorded
    #    within the last DEDUP_WINDOW_DAYS (default 7) means the crash was
    #    already alerted on — skip and reference that alert. Covers the
    #    investigated-and-closed alert shape leg 3 cannot see (the covering
    #    alert is no longer open, but the crash was still handled recently).
    #    An unreadable or corrupt history file never suppresses: fail open.
    if [[ -f "$HISTORY_FILE" ]]; then
        local hist hist_hits hist_when
        if hist=$(python3 -c "$HISTORY_PY" "$HISTORY_FILE" "$target" "$bead_id" "$DEDUP_WINDOW_DAYS" 2>&1); then
            hist_hits=$(awk '$1 == "HISTORY_MATCH" {print $2}' <<<"$hist")
            if [[ -n "$hist_hits" ]]; then
                hist_when=$(awk '$1 == "HISTORY_WHEN" {print $2}' <<<"$hist")
                echo "DUPLICATE: alert ${hist_hits//,/ } already covers crash target $target (recorded ${hist_when:-recently}, within the ${DEDUP_WINDOW_DAYS}-day window)"
                log_dedup "INFO" "check $bead_id: SUPPRESS (history: alert $hist_hits covers $target, within ${DEDUP_WINDOW_DAYS}d window)"
                VERDICT=$EXIT_DUPLICATE
                return 0
            fi
        else
            log_dedup "WARN" "check $bead_id: history scan failed - skipping window leg: $hist"
        fi
    fi

    echo "PROCEED: no resolution and no open alert for target $target (status: $target_status)"
    log_dedup "INFO" "check $bead_id: PROCEED (target $target status $target_status, no open alerts, no recent history)"
    VERDICT=$EXIT_UNIQUE
    return 0
}

# Append one crash-history entry for a GENERATED alert. Idempotent per alert
# bead: a re-run must not stack a second entry (the manager re-processes
# beads). VERDICT: 0 written/already present, 2 usage, 3 could not write.
record_history() {
    local bead_id="$1"
    shift
    VERDICT=$EXIT_UNKNOWN

    local target="" classification="UNKNOWN" crash_ts=""
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --target)         [[ $# -ge 2 ]] || { echo "ERROR: --target needs a value"; VERDICT=$EXIT_USAGE; return 0; }
                              target="$2"; shift 2 ;;
            --classification) [[ $# -ge 2 ]] || { echo "ERROR: --classification needs a value"; VERDICT=$EXIT_USAGE; return 0; }
                              classification="$2"; shift 2 ;;
            --crash-ts)       [[ $# -ge 2 ]] || { echo "ERROR: --crash-ts needs a value"; VERDICT=$EXIT_USAGE; return 0; }
                              crash_ts="$2"; shift 2 ;;
            *) echo "ERROR: unknown record option: $1"; VERDICT=$EXIT_USAGE; return 0 ;;
        esac
    done

    mkdir -p "$LOG_DIR"

    if [[ -f "$HISTORY_FILE" ]] && grep -q "\"bead_id\": *\"$bead_id\"" "$HISTORY_FILE" 2>/dev/null; then
        echo "ALREADY_RECORDED: $bead_id is already in $HISTORY_FILE"
        log_dedup "INFO" "record $bead_id: already recorded - no new entry"
        VERDICT=0
        return 0
    fi

    # The caller supplies the crash target when it knows it; otherwise derive
    # it from the title with the same extractor `check` uses (D-4). An
    # underivable target still records — with a null crash_bead_id — so the
    # alert's existence is never lost; it just cannot match a future check.
    if [[ -z "$target" ]]; then
        target=$(derive_target "$bead_id")
    fi

    local now entry
    now=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    if ! entry=$(jq -c -n \
            --arg bead "$bead_id" \
            --arg target "${target:-}" \
            --arg class "$classification" \
            --arg crash_ts "${crash_ts:-$now}" \
            --arg now "$now" \
            '{bead_id: $bead,
              crash_bead_id: (if $target == "" then null else $target end),
              crash_timestamp: $crash_ts,
              classification: $class,
              recorded_at: $now}'); then
        echo "ERROR: could not build the crash-history entry (jq unavailable?)"
        log_dedup "WARN" "record $bead_id: entry build failed - nothing written"
        return 0
    fi

    if ! printf '%s\n' "$entry" >>"$HISTORY_FILE"; then
        echo "ERROR: could not write $HISTORY_FILE"
        log_dedup "WARN" "record $bead_id: write failed"
        return 0
    fi

    echo "RECORDED: $bead_id -> $HISTORY_FILE (target: ${target:-unknown}, classification: $classification)"
    log_dedup "INFO" "record $bead_id: target ${target:-unknown} classification $classification"
    VERDICT=0
    return 0
}

# Crash target for record, from the same scan `check` uses. Empty on any
# failure — record still writes the entry, just without a matchable target.
derive_target() {
    local bead_id="$1" scan
    scan=$(bead list --json --limit "$BEAD_SCAN_LIMIT" 2>/dev/null || true)
    [[ -z "$scan" ]] && return 0
    printf '%s\n' "$scan" | python3 -c "$SCAN_PY" "$bead_id" 2>/dev/null \
        | awk '$1 == "TARGET" {print $2}'
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
        record)
            [[ $# -ge 2 ]] || { echo "ERROR: record requires a bead ID"; show_usage; exit $EXIT_USAGE; }
            [[ "$2" =~ ^(bf|domchk)-[a-z0-9]+$ ]] || { echo "ERROR: not a bead ID: $2"; exit $EXIT_USAGE; }
            # "${@:2}" is the bead id plus any --target/--classification/--crash-ts
            # options; record_history takes the bead id as its first argument. (A
            # `shift 2` here drops the bead id, so --target arrived as it and the
            # first value errored as an unknown option: every record exited 2 and
            # nothing ever reached the ledger — caught by
            # test-alert-dedup-history.sh cases 1-4/13, fixed 2026-09-07
            # domchk-fb636819.)
            record_history "${@:2}"
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
