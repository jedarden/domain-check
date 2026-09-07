#!/usr/bin/env bash
# Crash Classifier
# Analyzes crash artifacts and classifies crash type to prevent false positives
# Distinguishes between technical crashes and administrative workflow failures

set -euo pipefail

# Usage
show_usage() {
    cat <<EOF
Usage: $0 <bead-id>

Analyzes crash artifacts for the given bead ID and classifies the crash type.

Classification Types:
  - FALSE_POSITIVE   Post-completion administrative failure (not a technical crash)
  - SERVICE_FAILURE  External service dependency failure (HTTP 503, gateway unavailable)
  - INFRASTRUCTURE    System resource exhaustion or infrastructure event
  - CODE_DEFECT       Actual application error or crash
  - UNKNOWN          Unable to classify from artifacts

Output:
  - Prints the bare classification token as the FIRST line of stdout
    (machine contract: crash-alert-manager.sh reads the classification out
    of this output, so nothing may precede the token). Human context —
    provenance, next steps — follows.
  - Exit code 0: Successfully classified
  - Exit code 1: Classification failed
  - Exit code 2: Missing artifacts

Trace-slot provenance:
  .beads/traces/<id>/ holds only the most recent run of a bead id, so it is
  quoted as crash evidence only when metadata.json captured_at falls inside
  the incident window (derived from the bead's crash records in
  .beads/events.jsonl). Override the window with:
    CRASH_WINDOW_START=<iso8601> CRASH_WINDOW_END=<iso8601>
  Alternative event stream: BEADS_EVENTS=<path>

Classification signals (docs/crash-response-guide.md "Quick Decision Tree"):
  For an exit -1 crash on a still-open bead, evidence is gathered in order —
  the first signal that asserts decides the verdict:
    1. A deliverable commit for the bead landed within COMMIT_WINDOW_SEC
       (default 30) before the crash instant      -> FALSE_POSITIVE
    2. CLUSTER_MIN_BEADS (default 10) OTHER beads crashed within
       CLUSTER_WINDOW_SEC (default 600)           -> INFRASTRUCTURE (fleet wave)
    3. MemAvailable under MEM_FLOOR_KB (default 5 GiB) -> INFRASTRUCTURE (memory)
    4. .git over REPO_BLOAT_LIMIT_BYTES (default 500 MB) -> INFRASTRUCTURE (bloat)
  MEM_AVAILABLE_KB / REPO_BYTES override the live measurements (for tests).
  Every signal fails open: unavailable evidence leaves the generic
  INFRASTRUCTURE verdict unchanged. Signals 1-2 need the crash instant, from
  CRASH_WINDOW_END, .beads/events.jsonl, or a provenance-ok trace.
EOF
}

# Arguments
BEAD_ID="${1:-}"

if [ -z "$BEAD_ID" ]; then
    show_usage
    exit 1
fi

# Artifact paths
TRACE_DIR=".beads/traces/${BEAD_ID}/trace.jsonl"
METADATA_DIR=".beads/traces/${BEAD_ID}/metadata.json"

if [ ! -f "$TRACE_DIR" ]; then
    echo "ERROR: Bead trace not found: $TRACE_DIR"
    exit 2
fi

# Extract bead data from trace file
extract_bead_data() {
    local bead_id="$1"
    local trace_file=".beads/traces/${bead_id}/trace.jsonl"
    if [ -f "$trace_file" ]; then
        cat "$trace_file"
    fi
}

# Extract exit code from metadata.json
extract_exit_code() {
    local bead_id="$1"
    local metadata_file=".beads/traces/${bead_id}/metadata.json"
    if [ -f "$metadata_file" ]; then
        grep -oP '"exit_code":\s*-?\d+' "$metadata_file" 2>/dev/null | sed 's/"exit_code"://' | tr -d ' ' | head -1
    fi
}

# ---------------------------------------------------------------------------
# Trace-slot provenance (docs/crash-root-cause-bf-3561g.md §6 recommendation 7)
#
# `.beads/traces/<id>/` is a SINGLE SLOT: it holds only the most recent run of
# a bead id. After any later run — an auto-retry success, a manual re-run — it
# no longer describes the crash. For bf-3561g the slot holds the 2026-08-17
# SUCCESS run (exit_code 0, captured 2026-08-17T11:06Z) while the crash was
# 2026-08-16T17:21Z, so reading it as crash evidence inverted the
# classification. No trace may be quoted as crash evidence until its
# metadata.json `captured_at` is checked against the incident window.
# ---------------------------------------------------------------------------

# ISO8601 (Z or ±HH:MM offset, optional fractional seconds) -> epoch seconds.
iso_to_epoch() {
    local iso="$1"
    date -d "$iso" +%s 2>/dev/null || echo ""
}

# Incident window for a bead id, derived from the fleet event stream
# (.beads/events.jsonl crash records — the authoritative crash record; trace
# metadata is not). Needle emits the failed dispatch as "event":"fail" with an
# "exit_code"; "event":"crash" is accepted too for callers that pre-record one.
# Prints "<start_epoch> <end_epoch> <count>", nothing if no records. Override
# with env CRASH_WINDOW_START / CRASH_WINDOW_END (ISO8601).
crash_window_from_events() {
    local bead_id="$1"
    local events="${BEADS_EVENTS:-.beads/events.jsonl}"
    [ -f "$events" ] || return 0
    python3 - "$events" "$bead_id" <<'PY' 2>/dev/null || true
import calendar, json, re, sys

path, bead = sys.argv[1], sys.argv[2]
pat = re.compile(r"^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(Z|[+-]\d{2}:?\d{2})$")

def epoch(ts):
    m = pat.match(ts or "")
    if not m:
        return None
    y, mo, d, h, mi, s, _frac, off = m.groups()
    base = calendar.timegm((int(y), int(mo), int(d), int(h), int(mi), int(s), 0, 0))
    if off and off != "Z":
        sign = 1 if off[0] == "+" else -1
        off = off[1:].replace(":", "")
        base -= sign * (int(off[:2]) * 3600 + int(off[2:]) * 60)
    return base

times = []
with open(path, encoding="utf-8", errors="replace") as fh:
    for line in fh:
        line = line.strip()
        if '"crash"' not in line and '"fail"' not in line:
            continue
        try:
            rec = json.loads(line)
        except ValueError:
            continue
        if rec.get("event") in ("crash", "fail") and rec.get("bead") == bead:
            e = epoch(rec.get("ts"))
            if e is not None:
                times.append(e)
if times:
    print(min(times), max(times), len(times))
PY
}

# Exit code of the bead's crash record(s) from the event stream — the value
# that describes the CRASH, independent of whatever run the trace slot holds.
events_exit_code() {
    local bead_id="$1"
    local events="${BEADS_EVENTS:-.beads/events.jsonl}"
    [ -f "$events" ] || return 0
    python3 - "$events" "$bead_id" <<'PY' 2>/dev/null || true
import json, sys

path, bead = sys.argv[1], sys.argv[2]
codes = []
with open(path, encoding="utf-8", errors="replace") as fh:
    for line in fh:
        line = line.strip()
        if '"crash"' not in line and '"fail"' not in line:
            continue
        try:
            rec = json.loads(line)
        except ValueError:
            continue
        if rec.get("event") in ("crash", "fail") and rec.get("bead") == bead:
            if rec.get("exit_code") is not None:
                codes.append(rec.get("exit_code"))
if codes:
    print(codes[-1])
PY
}

# Decide what the trace slot may be used for. Sets globals:
#   PROVENANCE   ok | mismatch | unverified
#   WINDOW_INFO  human-readable incident-window summary ("" if none)
#   TRACE_INFO   human-readable captured_at/exit_code summary ("" if none)
check_trace_provenance() {
    local bead_id="$1"
    local metadata_file=".beads/traces/${bead_id}/metadata.json"

    PROVENANCE="unverified"
    WINDOW_INFO=""
    TRACE_INFO=""

    # Incident window: explicit env override, else derived from events.jsonl
    local w_start="" w_end="" w_count=""
    if [ -n "${CRASH_WINDOW_START:-}" ] && [ -n "${CRASH_WINDOW_END:-}" ]; then
        w_start="$(iso_to_epoch "$CRASH_WINDOW_START")"
        w_end="$(iso_to_epoch "$CRASH_WINDOW_END")"
        w_count="caller-supplied"
    else
        read -r w_start w_end w_count < <(crash_window_from_events "$bead_id") || true
    fi
    if [ -n "$w_start" ] && [ -n "$w_end" ]; then
        WINDOW_INFO="${w_count} crash record(s) in .beads/events.jsonl ($(date -u -d "@${w_start}" +%Y-%m-%dT%H:%M:%SZ) .. $(date -u -d "@${w_end}" +%Y-%m-%dT%H:%M:%SZ))"
    fi

    if [ ! -f "$metadata_file" ]; then
        TRACE_INFO="no metadata.json — trace has no provenance"
        return 0
    fi

    local captured_at exit_code
    captured_at="$(grep -oP '"captured_at":\s*"[^"]*"' "$metadata_file" 2>/dev/null | sed 's/"captured_at":\s*//; s/"//g' | head -1)"
    exit_code="$(extract_exit_code "$bead_id")"
    TRACE_INFO="trace slot holds a run captured ${captured_at:-<unknown>}, exit_code ${exit_code:-<unknown>}"

    if [ -z "$captured_at" ]; then
        return 0   # unverified — no provenance field
    fi
    if [ -z "$w_start" ] || [ -z "$w_end" ]; then
        return 0   # unverified — no incident window to check against
    fi

    local captured_epoch
    captured_epoch="$(iso_to_epoch "$captured_at")"
    if [ -n "$captured_epoch" ] && [ "$captured_epoch" -ge "$w_start" ] && [ "$captured_epoch" -le "$w_end" ]; then
        PROVENANCE="ok"
    else
        PROVENANCE="mismatch"
    fi
}

# ---------------------------------------------------------------------------
# Classification signals (domchk-701bcfa5)
#
# The decision tree in docs/crash-response-guide.md ("Quick Decision Tree")
# refines a raw exit -1 with evidence before it becomes INFRASTRUCTURE:
#
#   Exit -1 -> work completed within 30s of the crash?  -> FALSE_POSITIVE
#           -> fleet-wide clustering / memory exhausted / repo bloat
#              -> INFRASTRUCTURE, with the specific mechanism named
#
# Every signal is evidence-gathering that can fail. Unavailable data, a
# missing tool, or a slow command must leave the classification exactly what
# it would have been without the signal, so each helper below returns nonzero
# for "signal not asserted" and never aborts the run.
# ---------------------------------------------------------------------------

# Crash instant (epoch seconds), or nothing when none is trustworthy. Order:
# caller-supplied window (CRASH_WINDOW_END), the incident window derived from
# .beads/events.jsonl, then the trace's captured_at — the last only when
# provenance is ok (a single-slot overwrite makes an unverified captured_at
# meaningless as crash evidence).
resolve_crash_epoch() {
    local bead_id="$1"
    local t="" w_start="" w_end="" w_count="" captured=""
    if [ -n "${CRASH_WINDOW_START:-}" ] && [ -n "${CRASH_WINDOW_END:-}" ]; then
        t="$(iso_to_epoch "$CRASH_WINDOW_END")" || t=""
        [ -n "$t" ] && { echo "$t"; return 0; }
    fi
    read -r w_start w_end w_count < <(crash_window_from_events "$bead_id") || true
    if [ -n "${w_end:-}" ]; then
        echo "$w_end"
        return 0
    fi
    if [ "$PROVENANCE" = "ok" ] && [ -f "$METADATA_DIR" ]; then
        captured="$(grep -oP '"captured_at":\s*"[^"]*"' "$METADATA_DIR" 2>/dev/null | sed 's/"captured_at":\s*//; s/"//g' | head -1)"
        if [ -n "$captured" ]; then
            t="$(iso_to_epoch "$captured")" || t=""
            [ -n "$t" ] && { echo "$t"; return 0; }
        fi
    fi
    return 0
}

# FALSE_POSITIVE signal: a deliverable commit for this bead landed within
# COMMIT_WINDOW_SEC (default 30) before the crash instant — post-completion
# termination, the guide tree's "Work completed within 30s?" branch. Commits
# are located by bead id in the message across ALL refs, so unpushed
# prior-attempt branches count. Prints the commit epoch when asserted.
signal_commit_within_window() {
    local bead_id="$1" crash_epoch="$2"
    local window_sec="${COMMIT_WINDOW_SEC:-30}"
    local last_commit_epoch=""
    [ -n "$crash_epoch" ] || return 1
    command -v git >/dev/null 2>&1 || return 1
    last_commit_epoch="$(timeout 15 git log --all --grep="$bead_id" --format=%ct 2>/dev/null | head -1)" || return 1
    [ -n "$last_commit_epoch" ] || return 1
    if [ "$last_commit_epoch" -ge "$((crash_epoch - window_sec))" ] && \
       [ "$last_commit_epoch" -le "$crash_epoch" ]; then
        echo "$last_commit_epoch"
        return 0
    fi
    return 1
}

# INFRASTRUCTURE signal: fleet-wide clustering — at least CLUSTER_MIN_BEADS
# (default 10) OTHER beads recorded a crash within CLUSTER_WINDOW_SEC
# (default 600) of this crash instant, in .beads/events.jsonl. A synchronized
# wave is service-class (fleet crash signature, September 2026 census), not a
# per-bead defect. Prints the distinct-bead count when asserted.
signal_fleet_cluster() {
    local bead_id="$1" crash_epoch="$2"
    local window_sec="${CLUSTER_WINDOW_SEC:-600}"
    local min_beads="${CLUSTER_MIN_BEADS:-10}"
    local events="${BEADS_EVENTS:-.beads/events.jsonl}"
    [ -n "$crash_epoch" ] || return 1
    [ -f "$events" ] || return 1
    local n=""
    n="$(python3 - "$events" "$bead_id" "$crash_epoch" "$window_sec" <<'PY' 2>/dev/null || true
import calendar, json, re, sys

path, bead, crash_s, window_s = sys.argv[1], sys.argv[2], int(sys.argv[3]), int(sys.argv[4])
pat = re.compile(r"^(\d{4})-(\d{2})-(\d{2})T(\d{2}):(\d{2}):(\d{2})(?:\.(\d+))?(Z|[+-]\d{2}:?\d{2})$")

def epoch(ts):
    m = pat.match(ts or "")
    if not m:
        return None
    y, mo, d, h, mi, s, _frac, off = m.groups()
    base = calendar.timegm((int(y), int(mo), int(d), int(h), int(mi), int(s), 0, 0))
    if off and off != "Z":
        sign = 1 if off[0] == "+" else -1
        off = off[1:].replace(":", "")
        base -= sign * (int(off[:2]) * 3600 + int(off[2:]) * 60)
    return base

beads = set()
with open(path, encoding="utf-8", errors="replace") as fh:
    for line in fh:
        if '"crash"' not in line and '"fail"' not in line:
            continue
        try:
            rec = json.loads(line)
        except ValueError:
            continue
        if rec.get("event") not in ("crash", "fail") or rec.get("bead") == bead:
            continue
        e = epoch(rec.get("ts"))
        if e is not None and abs(e - crash_s) <= window_s:
            beads.add(rec.get("bead"))
print(len(beads))
PY
)"
    [ -n "$n" ] || return 1
    if [ "$n" -ge "$min_beads" ]; then
        echo "$n"
        return 0
    fi
    return 1
}

# INFRASTRUCTURE signal: memory exhaustion — MemAvailable under MEM_FLOOR_KB
# (default 5 GiB, the "Critical" row of the resource-limits table). Live
# memory describes the box NOW, so it is strongest for recent crashes; it
# never overrides the completion-timing check, which runs first. Prints the
# evidence when asserted.
signal_memory_exhausted() {
    local floor_kb="${MEM_FLOOR_KB:-5242880}"
    local avail="${MEM_AVAILABLE_KB:-}"
    if [ -z "$avail" ] && [ -r /proc/meminfo ]; then
        avail="$(awk '/^MemAvailable:/{print $2}' /proc/meminfo 2>/dev/null)"
        avail="${avail:-}"
    fi
    if [ -n "$avail" ] && [ "$avail" -lt "$floor_kb" ]; then
        echo "MemAvailable ${avail}kB below ${floor_kb}kB floor"
        return 0
    fi
    return 1
}

# INFRASTRUCTURE signal: repository bloat — .git over REPO_BLOAT_LIMIT_BYTES
# (default 500 MB, the repo-health warning boundary; the bf-1s6c3/bf-4yjq
# 18 GB sat ~36x over it), where significant git operations memcg-OOM. Prints
# the measured size when asserted.
signal_repo_bloated() {
    local limit_bytes="${REPO_BLOAT_LIMIT_BYTES:-524288000}"
    local measured="${REPO_BYTES:-}"
    if [ -z "$measured" ]; then
        [ -d ".git" ] || return 1
        measured="$(timeout 30 du -sb .git 2>/dev/null | awk '{print $1}')" || measured=""
        measured="${measured:-}"
    fi
    if [ -n "$measured" ] && [ "$measured" -gt "$limit_bytes" ]; then
        echo ".git measures ${measured} bytes (limit ${limit_bytes})"
        return 0
    fi
    return 1
}

# Classify crash
classify_crash() {
    local bead_id="$1"
    # Trace content is crash evidence ONLY when the trace slot provably holds
    # the incident run. On mismatch/unverified its strings belong to some other
    # run — for bf-3561g the success-run transcript matched the "task done"
    # grep below and produced a FALSE_POSITIVE with a fabricated reason.
    local bead_data=""
    if [ "$PROVENANCE" = "ok" ]; then
        bead_data=$(extract_bead_data "$bead_id")
        if [ -z "$bead_data" ]; then
            echo "UNKNOWN"
            echo "No trace data found for bead"
            exit 2
        fi
    fi
    # Note the trace was NOT quoted. Printed here — after the verdict, which
    # must stay line 1 of stdout for crash-alert-manager.sh — not before it.

    if [ -n "$bead_data" ]; then
    # Check for error_max_turns (administrative workflow failure)
    if echo "$bead_data" | grep -q "error_max_turns"; then
        echo "FALSE_POSITIVE"
        echo "Reason: Administrative workflow failure (max_turns exhausted)"
        echo "Pattern: Post-completion bead close failure, not technical crash"
        return 0
    fi

    # Check for HTTP 503 errors (service failure)
    if echo "$bead_data" | grep -q "503.*no available server"; then
        echo "SERVICE_FAILURE"
        echo "Reason: Inference gateway unavailable (HTTP 503)"
        echo "Pattern: External service dependency failure"
        return 0
    fi

    # Check for successful task completion before crash (bf-1ea4g pattern)
    # This check must come BEFORE exit code -1 check to catch post-completion crashes
    # Pattern: work committed < 30 seconds before crash
    if echo "$bead_data" | grep -q "git commit\|work.*complete\|task.*done"; then
        echo "FALSE_POSITIVE"
        echo "Reason: Task completed successfully before crash"
        echo "Pattern: Post-completion cleanup or administrative failure (bf-1ea4g pattern)"
        echo "Action: Verify task completion, may be false positive"
        return 0
    fi
    fi

    # Check for exit code -1. Source depends on trace provenance:
    #   provenance ok        -> the trace slot describes the incident run; use it
    #   mismatch/unverified  -> the slot may hold a DIFFERENT run (single-slot
    #                           overwrite), so take the crash's exit code from the
    #                           event stream instead. Falling back to the trace's
    #                           exit_code here is how bf-3561g's success run
    #                           (exit 0) got quoted as crash evidence.
    # IMPORTANT: Check if bead ultimately completed successfully (FALSE_POSITIVE pattern)
    # The 2026-08-16 cascade showed exit -1 crashes often recover via auto-retry.
    EXIT_CODE=""
    if [ "$PROVENANCE" = "ok" ]; then
        EXIT_CODE=$(extract_exit_code "$bead_id")
    else
        EXIT_CODE=$(events_exit_code "$bead_id")
    fi
    if [[ "$EXIT_CODE" == "-1" ]]; then
        # Check bead status to determine if this is a false positive
        BEAD_STATUS=$(bead show "$bead_id" 2>/dev/null | grep -i "^Status" || echo "unknown")

        if [[ "$BEAD_STATUS" =~ [Cc]losed ]]; then
            # Bead recovered after the crash - this is a FALSE_POSITIVE
            # Pattern: crash triggered automatic retry, a later attempt succeeded
            echo "FALSE_POSITIVE"
            echo "Reason: Exit code -1 (abnormal child death) but bead recovered via auto-retry"
            echo "Pattern: Crash + automatic recovery (bf-4k2ws pattern)"
            echo "Action: No investigation needed - automatic retry succeeded"
            echo "Note: exit -1 is needle's sentinel for abnormal child death, not a signal number; the 2026-08-16 cascade (177 crashes / 59 beads, 12:00-17:00 UTC) was kernel memcg OOM, and all its beads recovered"
            return 0
        fi

        # Bead still open/failed. Work the evidence before the generic
        # verdict — completion timing first (the guide tree's "Work completed
        # within 30s?" branch): a deliverable that landed just before the kill
        # means post-completion termination, whatever the mechanism.
        local crash_epoch="" commit_epoch="" cluster_beads="" mem_evidence="" repo_evidence=""
        crash_epoch="$(resolve_crash_epoch "$bead_id")" || crash_epoch=""

        if commit_epoch="$(signal_commit_within_window "$bead_id" "$crash_epoch")"; then
            echo "FALSE_POSITIVE"
            echo "Reason: Deliverable commit for $bead_id landed at $(date -u -d "@${commit_epoch}" +%Y-%m-%dT%H:%M:%SZ 2>/dev/null || echo "<unknown>") — within ${COMMIT_WINDOW_SEC:-30}s of the crash instant"
            echo "Pattern: Post-completion termination (work committed <30s before crash; bf-2vtzg pattern)"
            echo "Action: No investigation needed — verify the deliverable and close"
            return 0
        fi

        if cluster_beads="$(signal_fleet_cluster "$bead_id" "$crash_epoch")"; then
            echo "INFRASTRUCTURE"
            echo "Reason: Fleet-wide crash clustering — ${cluster_beads} other beads crashed within ${CLUSTER_WINDOW_SEC:-600}s of this crash instant"
            echo "Pattern: Synchronized service-class wave, not a per-bead defect"
            echo "Action: Check system-wide events for the crash window; do not investigate this bead in isolation"
            return 0
        fi

        if mem_evidence="$(signal_memory_exhausted)"; then
            echo "INFRASTRUCTURE"
            echo "Reason: Memory exhausted — $mem_evidence"
            echo "Pattern: System resource exhaustion (kernel may have OOM-killed a child)"
            echo "Action: Check memory usage and journalctl oom-kill records for the crash second"
            return 0
        fi

        if repo_evidence="$(signal_repo_bloated)"; then
            echo "INFRASTRUCTURE"
            echo "Reason: Repository bloat — $repo_evidence"
            echo "Pattern: Git operations over a bloated repository memcg-OOM (bf-1s6c3/bf-4yjq pattern)"
            echo "Action: Run ./scripts/check-repo-health.sh and route cleanup through ./scripts/safe-git-gc.sh"
            return 0
        fi

        # Bead still open/failed - this is a genuine infrastructure issue
        echo "INFRASTRUCTURE"
        echo "Reason: Abnormal child death (exit -1 sentinel) - bead not recovered"
        echo "Pattern: Common mechanisms: kernel memcg OOM kill of a child process (grep journalctl for oom-kill), system memory pressure, service-class failure wave. NOT SIGHUP (docs/crash-root-cause-bf-3561g.md §1.3)"
        echo "Action: Check system resources and journalctl oom-kill records for the crash second"
        return 0
    fi

    # Check for OOM killer patterns
    if echo "$bead_data" | grep -qi "oom\|out of memory\|memory exhausted"; then
        echo "INFRASTRUCTURE"
        echo "Reason: OOM killer or memory exhaustion"
        echo "Pattern: System resource exhaustion"
        # Corroborate with repo health where the trace already says OOM: a
        # bloated .git is the established bloat-OOM mechanism.
        if repo_ev="$(signal_repo_bloated)"; then
            echo "Evidence: $repo_ev — bloat-OOM mechanism (bf-1s6c3/bf-4yjq pattern)"
        fi
        echo "Action: Check memory usage and available RAM"
        return 0
    fi

    # Default: Unable to classify
    echo "UNKNOWN"
    echo "Reason: Insufficient data to classify"
    echo "Action: Manual investigation required"
    return 0
}

# Main
#
# Output contract (domchk-701bcfa5, wiring defect domchk-f6fff20f): the FIRST
# line of stdout is the bare classification token. crash-alert-manager.sh
# reads the classification out of this output, so whatever comes first IS the
# classification — the old banner-first layout made the manager record
# CLASSIFICATION='==================================' and turned its
# FALSE_POSITIVE branch into dead code. All human context follows the token.
main() {
    # Trace-slot provenance must be established BEFORE the trace is quoted as
    # crash evidence (the single-slot overwrite misdirected the early bf-3561g
    # documents — docs/crash-root-cause-bf-3561g.md §3 factor 6, §6 rec 7).
    # It only sets globals; nothing is printed until after the verdict.
    check_trace_provenance "$BEAD_ID"

    classify_crash "$BEAD_ID"
    exit_code=$?

    if [ "$PROVENANCE" != "ok" ]; then
        echo "Note: trace-derived pattern checks skipped — trace slot does not describe the incident run"
    fi

    echo ""
    echo "=================================="
    echo "Crash Classifier"
    echo "=================================="
    echo ""
    echo "Analyzing bead: $BEAD_ID"
    echo "Trace provenance: $PROVENANCE"
    [ -n "$TRACE_INFO" ] && echo "  $TRACE_INFO"
    [ -n "$WINDOW_INFO" ] && echo "  Incident window: $WINDOW_INFO"
    if [ "$PROVENANCE" = "mismatch" ]; then
        echo "  WARNING: trace slot was captured OUTSIDE the incident window — it"
        echo "  describes a different run (single-slot overwrite). Its contents are"
        echo "  NOT crash evidence; classification uses .beads/events.jsonl instead."
    elif [ "$PROVENANCE" = "unverified" ]; then
        echo "  WARNING: provenance cannot be established (no metadata or no incident"
        echo "  window). Do not quote this trace as crash evidence; the classifier"
        echo "  prefers .beads/events.jsonl where available."
    fi
    echo ""
    echo "Next steps:"
    echo "  - FALSE_POSITIVE: Review task completion, may need bead close fix"
    echo "  - SERVICE_FAILURE: Check inference gateway status, retry with backoff"
    echo "  - INFRASTRUCTURE: Check system resources (memory, disk, load)"
    echo "  - CODE_DEFECT: Investigate application error logs"
    echo "  - UNKNOWN: Manual investigation of crash artifacts"

    exit $exit_code
}

main
