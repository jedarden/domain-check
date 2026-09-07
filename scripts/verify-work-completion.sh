#!/usr/bin/env bash
# Work Completion Verification
# Run as the LAST step before `bead close` to verify the work actually landed.
#
# Background: multiple agents have crashed (exit code -1) AFTER finishing their
# task, leaving no durable record that the work was complete. This script closes
# that gap in both directions:
#   1. Before close: verifies commits are pushed, expected artifacts exist, the
#      bead carries a completion note, and the box is healthy enough to finish.
#   2. After a crash: writes a durable marker under .beads/state/work-completion/
#      so triage can distinguish "crashed after verified completion" (usually a
#      false positive) from "crashed mid-task" (needs investigation).
#
# Exit codes:
#   0  work verified — safe to close the bead
#   1  verification failed — do NOT close the bead
#   2  usage error
#
# Usage: scripts/verify-work-completion.sh <bead-id> [options]
#
# Options:
#   --require-path PATH        PATH must exist (repeatable)
#   --require-grep FILE:PAT    PAT must match in FILE (repeatable; split on
#                              first colon, PAT may itself contain colons)
#   --require-command CMD      CMD must exit 0 (repeatable)
#   --summary TEXT             free-text completion summary recorded on the
#                              marker and in the log
#   --strict-clean             treat uncommitted changes as a failure instead
#                              of a warning (default: warn — this workspace is
#                              shared and other agents' dirty files are normal)
#   --allow-unpushed           downgrade unpushed commits to a warning
#   --fail-on-empty-notes      empty bead notes are a failure (default: warn)
#   --skip-bead                skip bead-store lookups (tests, non-bead repos);
#                              <bead-id> is still required — it names the marker
#   --skip-health              skip memory/disk/load checks
#   --fetch-timeout SECONDS    timeout for `git fetch` (default: 15)
#   --json                     machine-readable JSON on stdout
#   -h, --help                 show this help
#
# Thresholds (env-overridable, defaults match CLAUDE.md resource limits):
#   VWC_MIN_AVAIL_MEM_GB   minimum available memory in GB   (default 10)
#   VWC_MIN_DISK_FREE_GB   minimum free disk space in GB    (default 20)
#   VWC_MAX_CPU_LOAD       maximum 1-minute load average    (default 10)

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# Markers and logs attach to the repo being verified (the caller's work tree),
# falling back to the repo that ships this script.
WORKTREE="$(git -C "$PWD" rev-parse --show-toplevel 2>/dev/null || true)"
if [[ -z "$WORKTREE" ]]; then
    WORKTREE="$REPO_ROOT"
fi

BEAD_DIR="$WORKTREE/.beads"
STATE_DIR="$BEAD_DIR/state/work-completion"
LOG_DIR="$BEAD_DIR/logs"
LOG_FILE="$LOG_DIR/work-completion.log"

VWC_MIN_AVAIL_MEM_GB="${VWC_MIN_AVAIL_MEM_GB:-10}"
VWC_MIN_DISK_FREE_GB="${VWC_MIN_DISK_FREE_GB:-20}"
VWC_MAX_CPU_LOAD="${VWC_MAX_CPU_LOAD:-10}"

log() {
    local timestamp
    timestamp=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    echo "[$timestamp] $*" | tee -a "$LOG_FILE" >&2
}

mkdir -p "$STATE_DIR" "$LOG_DIR"

# ---------------------------------------------------------------- arguments

BEAD_ID=""
SUMMARY=""
STRICT_CLEAN=false
ALLOW_UNPUSHED=false
FAIL_ON_EMPTY_NOTES=false
SKIP_BEAD=false
SKIP_HEALTH=false
FETCH_TIMEOUT=15
JSON_OUT=false
REQUIRE_PATHS=()
REQUIRE_GREPS=()
REQUIRE_COMMANDS=()

show_usage() {
    echo "Usage: $0 <bead-id> [options]"
    echo ""
    grep '^#' "$0" | sed -n '6,60p' | sed 's/^# \{0,1\}//'
}

while [[ $# -gt 0 ]]; do
    case "$1" in
        --require-path)    REQUIRE_PATHS+=("$2"); shift 2 ;;
        --require-grep)    REQUIRE_GREPS+=("$2"); shift 2 ;;
        --require-command) REQUIRE_COMMANDS+=("$2"); shift 2 ;;
        --summary)         SUMMARY="$2"; shift 2 ;;
        --strict-clean)    STRICT_CLEAN=true; shift ;;
        --allow-unpushed)  ALLOW_UNPUSHED=true; shift ;;
        --fail-on-empty-notes) FAIL_ON_EMPTY_NOTES=true; shift ;;
        --skip-bead)       SKIP_BEAD=true; shift ;;
        --skip-health)     SKIP_HEALTH=true; shift ;;
        --fetch-timeout)   FETCH_TIMEOUT="$2"; shift 2 ;;
        --json)            JSON_OUT=true; shift ;;
        --help|-h)         show_usage; exit 0 ;;
        --*)               echo "Unknown option: $1" >&2; exit 2 ;;
        *)                 BEAD_ID="$1"; shift ;;
    esac
done

if [[ -z "$BEAD_ID" ]]; then
    echo "Error: <bead-id> is required" >&2
    show_usage >&2
    exit 2
fi

# ------------------------------------------------------------ check helpers

CHECKS=()        # "name|status|detail"  status: PASS|WARN|FAIL
FAILURES=0
WARNINGS=0

record() {
    local status="$1" name="$2" detail="$3"
    CHECKS+=("$name|$status|$detail")
    case "$status" in
        FAIL) FAILURES=$((FAILURES + 1)) ;;
        WARN) WARNINGS=$((WARNINGS + 1)) ;;
    esac
}

# ------------------------------------------------------------------- checks

# 1. Git: unpushed commits
if git -C "$PWD" rev-parse --is-inside-work-tree >/dev/null 2>&1; then
    GITDIR="$PWD"
    UPSTREAM=""
    UPSTREAM_KIND=""
    if UPSTREAM="$(git -C "$GITDIR" rev-parse --abbrev-ref --symbolic-full-name '@{u}' 2>/dev/null)"; then
        UPSTREAM_KIND="upstream"
    elif git -C "$GITDIR" remote get-url origin >/dev/null 2>&1; then
        # No upstream configured but an origin exists — track its same-named branch
        local_branch="$(git -C "$GITDIR" rev-parse --abbrev-ref HEAD 2>/dev/null || echo '')"
        if [[ -n "$local_branch" ]] && git -C "$GITDIR" rev-parse --verify -q "origin/$local_branch" >/dev/null; then
            UPSTREAM="origin/$local_branch"
            UPSTREAM_KIND="origin-fallback"
        fi
    fi

    if [[ -z "$UPSTREAM" ]]; then
        record "WARN" "git_pushed" "no upstream/origin ref to compare against — push state unknown"
    else
        FETCHED="no"
        if command -v timeout >/dev/null 2>&1; then
            if timeout "$FETCH_TIMEOUT" git -C "$GITDIR" fetch origin --quiet >/dev/null 2>&1; then
                FETCHED="yes"
            fi
        elif git -C "$GITDIR" fetch origin --quiet >/dev/null 2>&1; then
            FETCHED="yes"
        fi
        if [[ "$FETCHED" != "yes" ]]; then
            record "WARN" "git_fetch" "could not fetch origin (offline?) — comparing against last known $UPSTREAM"
        fi

        AHEAD="$(git -C "$GITDIR" rev-list --count "$UPSTREAM..HEAD" 2>/dev/null || echo unknown)"
        BEHIND="$(git -C "$GITDIR" rev-list --count "HEAD..$UPSTREAM" 2>/dev/null || echo unknown)"

        if [[ "$AHEAD" == "unknown" ]]; then
            record "FAIL" "git_pushed" "cannot count commits ahead of $UPSTREAM"
        elif [[ "$AHEAD" -gt 0 ]]; then
            if [[ "$ALLOW_UNPUSHED" == "true" ]]; then
                record "WARN" "git_pushed" "$AHEAD unpushed commit(s) (allowed by --allow-unpushed)"
            else
                record "FAIL" "git_pushed" "$AHEAD unpushed commit(s) not on $UPSTREAM — push before closing"
            fi
        else
            record "PASS" "git_pushed" "HEAD is on $UPSTREAM"
        fi

        if [[ "$BEHIND" != "unknown" && "$BEHIND" -gt 0 ]]; then
            record "WARN" "git_diverged" "HEAD is $BEHIND commit(s) behind $UPSTREAM — reconcile before pushing more work"
        fi
    fi

    # 2. Git: uncommitted changes (warn by default — shared workspace)
    DIRTY_COUNT="$(git -C "$GITDIR" status --porcelain 2>/dev/null | wc -l | tr -d ' ')"
    if [[ "$DIRTY_COUNT" -gt 0 ]]; then
        SAMPLES="$(git -C "$GITDIR" status --porcelain 2>/dev/null | head -5 | tr '\n' ' ')"
        if [[ "$STRICT_CLEAN" == "true" ]]; then
            record "FAIL" "git_clean" "$DIRTY_COUNT uncommitted change(s): $SAMPLES"
        else
            record "WARN" "git_clean" "$DIRTY_COUNT uncommitted change(s) (shared workspace; --strict-clean to fail): $SAMPLES"
        fi
    else
        record "PASS" "git_clean" "working tree clean"
    fi
else
    record "WARN" "git_repo" "not inside a git work tree — git checks skipped"
fi

# 3. Bead state
if [[ "$SKIP_BEAD" == "true" ]]; then
    record "WARN" "bead_state" "skipped (--skip-bead)"
else
    BEAD_JSON="$(bead show "$BEAD_ID" --json 2>/dev/null || true)"
    if [[ -z "$BEAD_JSON" ]] || ! printf '%s' "$BEAD_JSON" | grep -qF "\"id\":\"$BEAD_ID\""; then
        record "FAIL" "bead_state" "bead $BEAD_ID not found in store"
    else
        if printf '%s' "$BEAD_JSON" | grep -qF '"notes":""'; then
            if [[ "$FAIL_ON_EMPTY_NOTES" == "true" ]]; then
                record "FAIL" "bead_notes" "bead has no completion notes — record findings with 'bead update' before closing"
            else
                record "WARN" "bead_notes" "bead has no completion notes (use --fail-on-empty-notes to fail)"
            fi
        else
            record "PASS" "bead_notes" "bead carries completion notes"
        fi
        BEAD_STATUS="$(printf '%s' "$BEAD_JSON" | grep -oE '"status":"[a-z_]+"' | head -1 | cut -d'"' -f4 || true)"
        if [[ "$BEAD_STATUS" == "closed" ]]; then
            record "WARN" "bead_status" "bead is already closed — verification is post-hoc"
        else
            record "PASS" "bead_status" "bead status: ${BEAD_STATUS:-unknown}"
        fi
    fi
fi

# 4. Required paths
for path in "${REQUIRE_PATHS[@]:-}"; do
    [[ -z "$path" ]] && continue
    if [[ -e "$path" ]]; then
        record "PASS" "require_path" "$path exists"
    else
        record "FAIL" "require_path" "$path does not exist"
    fi
done

# 5. Required greps (FILE:PATTERN — split on first colon)
for spec in "${REQUIRE_GREPS[@]:-}"; do
    [[ -z "$spec" ]] && continue
    FILE="${spec%%:*}"
    PATTERN="${spec#*:}"
    if [[ ! -e "$FILE" ]]; then
        record "FAIL" "require_grep" "$FILE does not exist (pattern: $PATTERN)"
    elif grep -qE -- "$PATTERN" "$FILE" 2>/dev/null; then
        record "PASS" "require_grep" "pattern matched in $FILE"
    else
        record "FAIL" "require_grep" "pattern not found in $FILE: $PATTERN"
    fi
done

# 6. Required commands
for cmd in "${REQUIRE_COMMANDS[@]:-}"; do
    [[ -z "$cmd" ]] && continue
    if bash -c "$cmd" >/dev/null 2>&1; then
        record "PASS" "require_command" "$cmd"
    else
        record "FAIL" "require_command" "command failed: $cmd"
    fi
done

# 7. Agent/box health (post-task mirror of preflight-health-check.sh)
if [[ "$SKIP_HEALTH" == "true" ]]; then
    record "WARN" "health" "skipped (--skip-health)"
else
    AVAIL_MEM="$(free -g 2>/dev/null | awk '/^Mem:/{print $7}')"
    if [[ -z "$AVAIL_MEM" ]]; then
        record "WARN" "health_memory" "could not read available memory"
    elif [[ "$AVAIL_MEM" -ge "$VWC_MIN_AVAIL_MEM_GB" ]]; then
        record "PASS" "health_memory" "${AVAIL_MEM}GB available (min ${VWC_MIN_AVAIL_MEM_GB}GB)"
    else
        record "FAIL" "health_memory" "only ${AVAIL_MEM}GB available (min ${VWC_MIN_AVAIL_MEM_GB}GB) — box under memory pressure, finish later or lower VWC_MIN_AVAIL_MEM_GB"
    fi

    DISK_FREE="$(df -BG --output=avail / 2>/dev/null | tail -1 | tr -dc '0-9')"
    if [[ -z "$DISK_FREE" ]]; then
        record "WARN" "health_disk" "could not read free disk space"
    elif [[ "$DISK_FREE" -ge "$VWC_MIN_DISK_FREE_GB" ]]; then
        record "PASS" "health_disk" "${DISK_FREE}GB free (min ${VWC_MIN_DISK_FREE_GB}GB)"
    else
        record "FAIL" "health_disk" "only ${DISK_FREE}GB free (min ${VWC_MIN_DISK_FREE_GB}GB) — disk pressure, cleanup before closing"
    fi

    LOAD="$(cut -d' ' -f1 /proc/loadavg 2>/dev/null || echo unknown)"
    if [[ "$LOAD" == "unknown" ]]; then
        record "WARN" "health_load" "could not read load average"
    elif awk -v l="$LOAD" -v m="$VWC_MAX_CPU_LOAD" 'BEGIN{exit !(l < m)}'; then
        record "PASS" "health_load" "1min load $LOAD (max $VWC_MAX_CPU_LOAD)"
    else
        record "FAIL" "health_load" "1min load $LOAD >= max $VWC_MAX_CPU_LOAD — wait for load to drop before closing"
    fi
fi

# ------------------------------------------------------------------- result

RESULT="VERIFIED"
RC=0
if [[ "$FAILURES" -gt 0 ]]; then
    RESULT="FAILED"
    RC=1
fi

sanitized_summary="${SUMMARY//\\/}"
sanitized_summary="${sanitized_summary//\"/\'}"
sanitized_summary="${sanitized_summary//$'\n'/ }"
sanitized_summary="${sanitized_summary//$'\r'/ }"

TIMESTAMP="$(date -u +"%Y-%m-%dT%H:%M:%SZ")"
MARKER="$STATE_DIR/$BEAD_ID.json"

CHECK_LINES=""
for entry in "${CHECKS[@]}"; do
    name="${entry%%|*}"
    rest="${entry#*|}"
    status="${rest%%|*}"
    detail="${rest#*|}"
    detail="${detail//\\/}"
    detail="${detail//\"/\'}"
    detail="${detail//$'\n'/ }"
    CHECK_LINES+="  {\"check\": \"$name\", \"status\": \"$status\", \"detail\": \"$detail\"},"$'\n'
done
CHECK_LINES="${CHECK_LINES%,*$'\n'}"

cat > "$MARKER" <<EOF
{
  "bead_id": "$BEAD_ID",
  "result": "$RESULT",
  "timestamp": "$TIMESTAMP",
  "failures": $FAILURES,
  "warnings": $WARNINGS,
  "summary": "$sanitized_summary",
  "checks": [
$CHECK_LINES
  ]
}
EOF

log "work-completion bead=$BEAD_ID result=$RESULT failures=$FAILURES warnings=$WARNINGS summary=${sanitized_summary:-none}"

if [[ "$JSON_OUT" == "true" ]]; then
    cat "$MARKER"
else
    echo ""
    echo "=== Work Completion Verification: $BEAD_ID ==="
    for entry in "${CHECKS[@]}"; do
        name="${entry%%|*}"
        rest="${entry#*|}"
        status="${rest%%|*}"
        detail="${rest#*|}"
        printf '  [%-4s] %-14s %s\n' "$status" "$name" "$detail"
    done
    echo ""
    echo "Result: $RESULT ($FAILURES failure(s), $WARNINGS warning(s))"
    echo "Marker: $MARKER"
    if [[ "$RESULT" == "VERIFIED" ]]; then
        echo "Safe to close: bead close $BEAD_ID --reason \"...\""
    else
        echo "Do NOT close — fix the failures above, then re-run this script."
    fi
fi
exit "$RC"
