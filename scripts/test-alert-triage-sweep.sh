#!/usr/bin/env bash
# Test Suite for alert-triage-sweep.sh — the fleet-wide, report-only triage
# pass (gap analysis §5 P1(a)/P4, docs/alert-deduplication-gap-analysis-
# 2026-09-07.md).
#
# Covers:
#   verdicts       RESOLVED_TARGET (closure / VERIFIED marker), ORPHANED_TARGET,
#                  FANOUT_KEEPER + FANOUT_DUPLICATE (oldest alert is the keeper),
#                  NEEDS_REVIEW (unresolved sole target, FAILED marker)
#   D-10 flow      a cached closure must not suppress a REOPENED target
#   report-only    the sweep runs `bead list`/`bead show` and NOTHING else —
#                  no close/update/create/reopen/release/claim call at all
#   fail-open      unreadable store → exit 3, no queue written, no verdicts
#   D-6 lesson     paths derive from BASH_SOURCE — caller CWD is irrelevant
#   scope          in_progress alerts are swept; non-alert beads are not
#   delta          a second sweep reports 0 new close candidates
#
# Hermetic, like test-alert-dedup-check.sh: copies the scripts under test into
# a temp sandbox, puts a fake `bead` CLI on PATH, and derives nothing from the
# real bead store.
#
# Exit codes: 0 all passed, 1 at least one failure.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SWEEP_SCRIPT="$SCRIPT_DIR/alert-triage-sweep.sh"
TRACKER_SCRIPT="$SCRIPT_DIR/crash-resolution-tracker.sh"

SBX="$(mktemp -d /tmp/alert-triage-test.XXXXXX)"
trap 'rm -rf "$SBX"' EXIT

mkdir -p "$SBX/bin" "$SBX/scripts" "$SBX/.beads/logs" "$SBX/.beads/state/work-completion"
cp "$SWEEP_SCRIPT" "$TRACKER_SCRIPT" "$SBX/scripts/"
SBX_SWEEP="$SBX/scripts/alert-triage-sweep.sh"
SBX_TRACKER="$SBX/scripts/crash-resolution-tracker.sh"
FAKE_STORE="$SBX/beads.jsonl"
BEAD_CALLS="$SBX/bead-calls.log"
: > "$FAKE_STORE"
: > "$BEAD_CALLS"

TESTS_RUN=0
TESTS_PASSED=0

pass() { TESTS_RUN=$((TESTS_RUN + 1)); TESTS_PASSED=$((TESTS_PASSED + 1)); echo "  PASS - $1"; }
fail() { TESTS_RUN=$((TESTS_RUN + 1)); echo "  FAIL - $1"; echo "         $2"; }

assert_exit() { # assert_exit <desc> <expected> <actual>
    if [[ "$2" == "$3" ]]; then pass "$1 (exit $3)"; else fail "$1" "expected exit $2, got $3"; fi
}

assert_contains() { # assert_contains <desc> <haystack> <needle>
    if grep -q "$3" <<<"$2"; then pass "$1"; else fail "$1" "output missing: $3"; fi
}

# Assert a queue verdict. assert_verdict <desc> <alert-id> <expected-verdict>
assert_verdict() {
    local desc="$1" alert="$2" want="$3" got
    got=$(jq -r --arg a "$alert" 'select(.alert == $a) | .verdict' "$SBX/.beads/state/alert-triage/queue.jsonl" 2>/dev/null)
    if [[ "$got" == "$want" ]]; then pass "$desc ($got)"; else fail "$desc" "expected verdict $want for $alert, got '${got:-<absent>}'"; fi
}

# Fake bead CLI. Every invocation is logged so the report-only contract is
# testable: the sweep must never run a mutating verb.
cat > "$SBX/bin/bead" <<'SHIM'
#!/usr/bin/env bash
STORE="${FAKE_STORE:?FAKE_STORE not set}"
CALLS="${BEAD_CALLS:?BEAD_CALLS not set}"
cmd="${1:-}"; shift || true
echo "$cmd" >> "$CALLS"
case "$cmd" in
    list)
        [[ "${FAKE_BEAD_FAIL_LIST:-0}" == "1" ]] && exit 1
        cat "$STORE" 2>/dev/null
        ;;
    show)
        id="${1:-}"
        rec=$(jq -c --arg id "$id" 'select(.id == $id)' "$STORE" 2>/dev/null | head -1)
        if [[ -z "$rec" ]]; then echo "Error: bead $id not found" >&2; exit 1; fi
        echo "ID: $id"
        echo "Status: $(jq -r '.status' <<<"$rec")"
        echo "Title: $(jq -r '.title' <<<"$rec")"
        ;;
    *)
        echo "fake bead: unsupported command: $cmd" >&2
        exit 64
        ;;
esac
SHIM
chmod +x "$SBX/bin/bead"

add_bead() { # add_bead <id> <status> <title> <created_at> [labels-comma-separated]
    local id="$1" status="$2" title="$3" created="$4" labels="${5:-}"
    jq -c -n --arg id "$id" --arg status "$status" --arg title "$title" \
        --arg created "$created" --argjson labels "[\"${labels//,/\",\"}\"]" \
        '{id: $id, status: $status, title: $title, created_at: $created, labels: $labels}' >> "$FAKE_STORE"
}

set_status() { # set_status <id> <new-status>  (simulates close / reopen)
    local tmp="$FAKE_STORE.tmp"
    jq -c --arg id "$1" --arg status "$2" 'if .id == $id then .status = $status else . end' \
        "$FAKE_STORE" > "$tmp" && mv "$tmp" "$FAKE_STORE"
}

export PATH="$SBX/bin:$PATH"
export FAKE_STORE BEAD_CALLS

# ----------------------------------------------------------------------------
echo "=== Scenario: verdicts across the target-resolution matrix ==="

# Closed target -> RESOLVED_TARGET citing bead_closure.
add_bead "bf-tgtclosed" "closed" "Fix the bloated gc path" "2026-08-01T00:00:00Z"
add_bead "bf-al-close" "open" "ALERT: Agent crash on bead bf-tgtclosed" "2026-08-02T00:00:00Z" "alert"

# Open target, sole alert, nothing resolved -> NEEDS_REVIEW.
add_bead "bf-tgtopen" "open" "Retry the push that oomed" "2026-08-01T00:00:00Z"
add_bead "bf-al-review" "open" "ALERT: Agent crash on bead bf-tgtopen" "2026-08-02T00:00:00Z" "alert"

# Open target, two open alerts -> oldest is the keeper, the other a duplicate.
add_bead "bf-tgtfan" "open" "Investigate the flaky gateway" "2026-08-01T00:00:00Z"
add_bead "bf-al-old" "open" "ALERT: Agent crash on bead bf-tgtfan" "2026-08-02T00:00:00Z" "alert"
add_bead "bf-al-new" "open" "ALERT: Agent crash on bead bf-tgtfan" "2026-08-05T00:00:00Z" "alert"

# Open target with a VERIFIED work-completion marker -> RESOLVED_TARGET.
add_bead "bf-tgtmark" "open" "Re-run the gc under bounds" "2026-08-01T00:00:00Z"
add_bead "bf-al-mark" "open" "ALERT: Agent crash on bead bf-tgtmark" "2026-08-02T00:00:00Z" "alert"
echo '{"bead":"bf-tgtmark","result":"VERIFIED"}' > "$SBX/.beads/state/work-completion/bf-tgtmark.json"

# Open target with a FAILED work-completion marker -> NOT resolution.
add_bead "bf-tgtfail" "open" "Sweep the loose objects" "2026-08-01T00:00:00Z"
add_bead "bf-al-fail" "open" "ALERT: Agent crash on bead bf-tgtfail" "2026-08-02T00:00:00Z" "alert"
echo '{"bead":"bf-tgtfail","result":"FAILED"}' > "$SBX/.beads/state/work-completion/bf-tgtfail.json"

# Target absent from the store entirely -> ORPHANED_TARGET.
add_bead "bf-al-orph" "open" "ALERT: Agent crash on bead bf-goneout" "2026-08-02T00:00:00Z" "alert"

# in_progress alerts are swept too.
add_bead "bf-al-wip" "in_progress" "ALERT: Agent crash on bead bf-tgtclosed" "2026-08-03T00:00:00Z" "alert"

# A plain open investigation bead is NOT an alert and must not be swept.
add_bead "bf-notalert" "open" "Update alerting and monitoring to prevent duplicate alerts" ""

out=$(cd / && "$SBX_SWEEP" 2>&1); rc=$?
assert_exit "sweep succeeds from an unrelated CWD (D-6)" 0 "$rc"
assert_verdict "closed target -> close candidate" "bf-al-close" "RESOLVED_TARGET"
assert_verdict "in_progress alert swept like an open one" "bf-al-wip" "RESOLVED_TARGET"
assert_verdict "unresolved sole target -> needs review" "bf-al-review" "NEEDS_REVIEW"
assert_verdict "oldest fan-out alert is the keeper" "bf-al-old" "FANOUT_KEEPER"
assert_verdict "newer fan-out alert is a duplicate" "bf-al-new" "FANOUT_DUPLICATE"
assert_verdict "VERIFIED marker resolves an open target" "bf-al-mark" "RESOLVED_TARGET"
assert_verdict "FAILED marker does not resolve" "bf-al-fail" "NEEDS_REVIEW"
assert_verdict "missing target -> orphaned" "bf-al-orph" "ORPHANED_TARGET"

if jq -e --arg a "bf-notalert" 'select(.alert == $a)' "$SBX/.beads/state/alert-triage/queue.jsonl" >/dev/null; then
    fail "non-alert investigation bead is not swept" "bf-notalert appears in the queue"
else
    pass "non-alert investigation bead is not swept"
fi
# 8 alert beads are created above (close/review/old/new/mark/fail/orph/wip) and
# every one has a verdict assertion — the swept count is 8 across 7 targets.
# (The assertion previously expected 7: the in_progress and orphan alerts were
# added to the scenario without bumping it.)
assert_contains "summary names the swept alert count" "$out" "Open alerts: 8"
assert_contains "summary names the distinct target count" "$out" "across 6 targets"

# ----------------------------------------------------------------------------
echo "=== Scenario: a cached closure must not suppress a REOPENED target (D-10) ==="

add_bead "bf-tgtreopen" "open" "Close out the alert chain" "2026-08-01T00:00:00Z"
add_bead "bf-al-reopen" "open" "ALERT: Agent crash on bead bf-tgtreopen" "2026-08-02T00:00:00Z" "alert"
jq -n --arg id "bf-tgtreopen" --arg ts "2026-08-10T00:00:00Z" \
    '{resolutions: {($id): {bead_id: $id, resolved_at: $ts, resolution_type: "bead_closure", reason: "cached", verified: true}},
      metadata: {version: "1.0", created: $ts, last_updated: $ts}}' \
    > "$SBX/.beads/state/crash-resolutions.json"

out=$("$SBX_SWEEP" 2>&1)
assert_verdict "live open target outranks its cached closure" "bf-al-reopen" "NEEDS_REVIEW"

# And once the target actually closes, the same alert becomes a candidate.
set_status "bf-tgtreopen" "closed"
"$SBX_SWEEP" >/dev/null 2>&1
assert_verdict "closed target after reopen -> close candidate" "bf-al-reopen" "RESOLVED_TARGET"

# ----------------------------------------------------------------------------
echo "=== Scenario: report-only contract ==="

mutating=$(grep -vE '^(list|show)$' "$BEAD_CALLS" || true)
if [[ -z "$mutating" ]]; then
    pass "sweep only read the store (list/show), never mutated a bead"
else
    fail "sweep only read the store (list/show), never mutated a bead" "mutating calls observed: $mutating"
fi

# ----------------------------------------------------------------------------
echo "=== Scenario: delta against the previous sweep ==="

out=$("$SBX_SWEEP" 2>&1)
assert_contains "second sweep reports no new close candidates" "$out" "New close candidates since the previous sweep: 0"

# ----------------------------------------------------------------------------
echo "=== Scenario: unreadable store fails open and writes nothing ==="

rm -f "$SBX/.beads/state/alert-triage/queue.jsonl"
out=$(FAKE_BEAD_FAIL_LIST=1 "$SBX_SWEEP" 2>&1); rc=$?
assert_exit "unreadable store -> exit 3" 3 "$rc"
assert_contains "fail-open verdict says nothing was swept" "$out" "nothing swept, nothing written"
if [[ -f "$SBX/.beads/state/alert-triage/queue.jsonl" ]]; then
    fail "fail-open writes no queue" "queue file exists after a failed sweep"
else
    pass "fail-open writes no queue"
fi

# ----------------------------------------------------------------------------
echo "=== Scenario: usage ==="

out=$("$SBX_SWEEP" --bogus 2>&1); rc=$?
assert_exit "unknown option -> usage exit 2" 2 "$rc"

# ----------------------------------------------------------------------------

echo
echo "Passed: $TESTS_PASSED"
echo "Failed: $((TESTS_RUN - TESTS_PASSED))"
if [[ "$TESTS_PASSED" -eq "$TESTS_RUN" ]]; then
    echo "All tests passed!"
    exit 0
fi
echo "TESTS FAILED"
exit 1
