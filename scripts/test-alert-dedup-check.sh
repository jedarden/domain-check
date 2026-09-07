#!/usr/bin/env bash
# Test Suite for alert-deduplication.sh `check` + live resolution tracking
#
# Covers the known scenarios from the alert deduplication gap analysis
# (docs/alert-deduplication-gap-analysis-2026-09-07.md) for the beads
# domchk-b3de301f contract:
#
#   D-1  resolution check evaluates LIVE bead state, not a 0-record ledger
#   D-2  dedup keyed on the crash TARGET, not the alert-bead instance
#   D-4  target extraction matches the real "ALERT: Agent crash on bead X"
#        title and accepts domchk-* alert beads (no ^bf- gate)
#   D-6  alert-deduplication.sh check <bead-id>: exit-code contract, paths
#        derived from BASH_SOURCE (caller CWD irrelevant), report lines that
#        never say "duplicate" when there is no duplicate
#   D-10 closure-based resolution never expires; cached closure does not
#        suppress a REOPENED target
#   G-9  VERIFIED work-completion marker counts as resolution
#
# Hermetic: copies the scripts under test into a temp sandbox, puts a fake
# `bead` CLI on PATH, and derives nothing from the real bead store. The real
# store is exercised by the live scenarios at the bottom only when
# DEDUP_TEST_LIVE=1 (default off, so `go test`-style default runs stay fast
# and offline).
#
# Exit codes: 0 all passed, 1 at least one failure.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
DEDUP_SCRIPT="$SCRIPT_DIR/alert-deduplication.sh"
RESOLUTION_TRACKER="$SCRIPT_DIR/crash-resolution-tracker.sh"

SBX="$(mktemp -d /tmp/alert-dedup-test.XXXXXX)"
trap 'rm -rf "$SBX"' EXIT

mkdir -p "$SBX/bin" "$SBX/scripts" "$SBX/.beads/logs" "$SBX/.beads/state/work-completion" "$SBX/.beads/traces"
cp "$DEDUP_SCRIPT" "$RESOLUTION_TRACKER" "$SBX/scripts/"
SBX_DEDUP="$SBX/scripts/alert-deduplication.sh"
SBX_TRACKER="$SBX/scripts/crash-resolution-tracker.sh"
FAKE_STORE="$SBX/beads.jsonl"
: > "$FAKE_STORE"

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

assert_not_contains() { # assert_not_contains <desc> <haystack> <needle>
    if grep -q "$3" <<<"$2"; then fail "$1" "output unexpectedly contains: $3"; else pass "$1"; fi
}

# Fake bead CLI. `bead list --json` emits the store as JSONL; `bead show <id>`
# prints aStatus line the way the real CLI does. FAKE_BEAD_FAIL_SHOW=1 makes
# `show` fail (store unreadable) without affecting `list`.
cat > "$SBX/bin/bead" <<'SHIM'
#!/usr/bin/env bash
STORE="${FAKE_STORE:?FAKE_STORE not set}"
cmd="${1:-}"; shift || true
case "$cmd" in
    list)
        [[ "${FAKE_BEAD_FAIL_LIST:-0}" == "1" ]] && exit 1
        cat "$STORE" 2>/dev/null
        ;;
    show)
        [[ "${FAKE_BEAD_FAIL_SHOW:-0}" == "1" ]] && exit 1
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

add_bead() { # add_bead <id> <status> <title> [labels-comma-separated]
    local id="$1" status="$2" title="$3" labels="${4:-}"
    jq -c -n --arg id "$id" --arg status "$status" --arg title "$title" \
        --argjson labels "[\"${labels//,/\",\"}\"]" \
        '{id: $id, status: $status, title: $title, labels: $labels}' >> "$FAKE_STORE"
}

set_status() { # set_status <id> <new-status>  (simulates close / reopen)
    local tmp="$FAKE_STORE.tmp"
    jq -c --arg id "$1" --arg status "$2" 'if .id == $id then .status = $status else . end' \
        "$FAKE_STORE" > "$tmp" && mv "$tmp" "$FAKE_STORE"
}

export PATH="$SBX/bin:$PATH"
export FAKE_STORE

# ----------------------------------------------------------------------------
echo "=== Scenario: target resolution keyed on the crash target (D-1/D-2) ==="

# bf-tgtclosed is CLOSED; bf-alertcl is a fresh open alert bead about it.
add_bead "bf-tgtclosed" "closed" "Fix the bloated gc path"
add_bead "bf-alertcl" "open" "ALERT: Agent crash on bead bf-tgtclosed" "alert"

out=$(cd / && "$SBX_DEDUP" check bf-alertcl 2>&1); rc=$?
assert_exit "closed target suppresses the fresh alert (D-1)" 0 "$rc"
assert_contains "verdict names the resolved target" "$out" "DUPLICATE: crash target bf-tgtclosed is already resolved"

# The same verdict from the tracker, the exact call shape
# crash-alert-manager.sh:263 uses before generating an alert.
out=$("$SBX_TRACKER" bf-tgtclosed check 2>&1); rc=$?
assert_exit "tracker check: closed bead is RESOLVED" 0 "$rc"
assert_contains "tracker output carries the RESOLVED marker the manager greps" "$out" "RESOLVED"

# ----------------------------------------------------------------------------
echo "=== Scenario: legitimate alert still fires (AC: legit alerts fire) ==="

# bf-tgtopen: open target, sole alert, no marker, no ledger record.
add_bead "bf-tgtopen" "open" "Investigate the flaky gateway"
add_bead "bf-alertun" "open" "ALERT: Agent crash on bead bf-tgtopen" "alert"

out=$(cd /tmp && "$SBX_DEDUP" check bf-alertun 2>&1); rc=$?
assert_exit "open unresolved target: alert proceeds (exit 1 UNIQUE)" 1 "$rc"
assert_contains "proceed verdict names the target" "$out" "PROCEED: no resolution and no open alert for target bf-tgtopen"

out=$("$SBX_TRACKER" bf-tgtopen check >/dev/null 2>&1); rc=$?
assert_exit "tracker check: open bead is NOT_RESOLVED (exit 1)" 1 "$rc"

# A NON-alert investigation bead with no crash reference resolves to itself
# and must not be suppressed (fail-open for unstructured titles).
out=$("$SBX_DEDUP" check bf-plain01 2>&1); rc=$?  # bf-plain01 not in store
assert_exit "unknown bead fails open (exit 3 INDETERMINATE)" 3 "$rc"
assert_contains "fail-open verdict says proceed" "$out" "failing open"

# ----------------------------------------------------------------------------
echo "=== Scenario: VERIFIED work-completion marker resolves (G-9) ==="

add_bead "bf-tgtmark" "open" "Retry the push that oomed"
add_bead "bf-alertmk" "open" "ALERT: Agent crash on bead bf-tgtmark" "alert"

out=$("$SBX_DEDUP" check bf-alertmk 2>&1); rc=$?
assert_exit "no marker yet: alert proceeds" 1 "$rc"

echo '{"bead_id":"bf-tgtmark","result":"VERIFIED","timestamp":"2026-09-07T00:00:00Z"}' \
    > "$SBX/.beads/state/work-completion/bf-tgtmark.json"
out=$("$SBX_DEDUP" check bf-alertmk 2>&1); rc=$?
assert_exit "VERIFIED marker suppresses (exit 0)" 0 "$rc"
assert_contains "verdict cites the work-completion marker" "$out" "VERIFIED work-completion marker"

echo '{"bead_id":"bf-tgtmark","result":"FAILED","timestamp":"2026-09-07T00:00:00Z"}' \
    > "$SBX/.beads/state/work-completion/bf-tgtmark.json"
out=$("$SBX_DEDUP" check bf-alertmk 2>&1); rc=$?
assert_exit "FAILED marker does not suppress: alert proceeds" 1 "$rc"

# ----------------------------------------------------------------------------
echo "=== Scenario: an open alert already covering the target is a duplicate (D-2) ==="

add_bead "bf-tgtcov" "open" "The bead everyone keeps alerting about"
add_bead "bf-alertc1" "open" "ALERT: Agent crash on bead bf-tgtcov" "alert"
add_bead "bf-alertc2" "open" "ALERT: Agent crash on bead bf-tgtcov" "alert"
add_bead "bf-alertc3" "closed" "ALERT: Agent crash on bead bf-tgtcov" "alert"

out=$("$SBX_DEDUP" check bf-alertc2 2>&1); rc=$?
assert_exit "second open alert on the same target is a duplicate (exit 0)" 0 "$rc"
assert_contains "verdict lists the covering alert, not this bead" "$out" "bf-alertc1"

# Closed siblings do not count as coverage.
add_bead "bf-tgtcl2" "open" "Target whose only other alert closed"
add_bead "bf-alertx1" "closed" "ALERT: Agent crash on bead bf-tgtcl2" "alert"
add_bead "bf-alertx2" "open" "ALERT: Agent crash on bead bf-tgtcl2" "alert"
out=$("$SBX_DEDUP" check bf-alertx2 2>&1); rc=$?
assert_exit "closed sibling alerts do not cover: alert proceeds" 1 "$rc"

# ----------------------------------------------------------------------------
echo "=== Scenario: target extraction from real needle titles (D-4) ==="

# domchk-* alert id (no ^bf- gate) + the exact real title shape, and the
# "Investigate ... crash on bead X" shape.
add_bead "bf-tgtttl" "closed" "Crashed bead"
add_bead "domchk-ttl01" "open" "ALERT: Agent crash on bead bf-tgtttl" "alert"
out=$("$SBX_DEDUP" check domchk-ttl01 2>&1); rc=$?
assert_exit "domchk-* alert id with real needle title extracts the target (D-4)" 0 "$rc"
assert_contains "target taken from the title, not the bead id" "$out" "crash target bf-tgtttl is already resolved"

add_bead "domchk-ttl02" "open" "Investigate agent crash on bead bf-tgtttl" "alert"
out=$("$SBX_DEDUP" check domchk-ttl02 2>&1); rc=$?
assert_exit "'Investigate ... crash on bead X' title extracts the target too" 0 "$rc"

# ----------------------------------------------------------------------------
echo "=== Scenario: closure-based resolution vs expiry and reopen (D-10) ==="

add_bead "bf-tgtexp" "closed" "Bead that was closed long ago"
out=$("$SBX_TRACKER" bf-tgtexp check >/dev/null 2>&1); rc=$?
assert_exit "closure resolves regardless of record age (live-first)" 0 "$rc"

# After closure is cached, REOPEN the target: the cached bead_closure record
# must not keep suppressing a live, reopened crash.
set_status "bf-tgtexp" "open"
out=$("$SBX_TRACKER" bf-tgtexp check >/dev/null 2>&1); rc=$?
assert_exit "reopened target: cached closure no longer suppresses (exit 1)" 1 "$rc"

# Store unreadable + a cached closure record: defer to the last observed
# closure rather than re-alerting on a transient outage.
set_status "bf-tgtexp" "closed"
"$SBX_TRACKER" bf-tgtexp check >/dev/null 2>&1   # re-cache the closure
FAKE_BEAD_FAIL_SHOW=1 "$SBX_TRACKER" bf-tgtexp check >/dev/null 2>&1; rc=$?
assert_exit "store unreadable + cached closure: defer to cache (exit 0)" 0 "$rc"

# A stale MANUAL cache record, by contrast, expires (30-day window): the
# ledger is a cache of manual marks, not a permanent resolution.
add_bead "bf-tgtman" "open" "Manually marked resolved ages ago"
mkdir -p "$SBX/.beads/state"
jq -n --arg ts "$(date -u -d '40 days ago' +%Y-%m-%dT%H:%M:%SZ)" \
    '{resolutions: {"bf-tgtman": {bead_id: "bf-tgtman", resolved_at: $ts,
     resolution_type: "manual", reason: "test", verified: true}},
     metadata: {version: "1.0"}}' > "$SBX/.beads/state/crash-resolutions.json"
out=$("$SBX_TRACKER" bf-tgtman check >/dev/null 2>&1); rc=$?
assert_exit "stale manual cache record expires (exit 1)" 1 "$rc"

# A closure-type cache record never expires while the store is unreadable,
# even if ancient.
jq -n --arg ts "$(date -u -d '400 days ago' +%Y-%m-%dT%H:%M:%SZ)" \
    '{resolutions: {"bf-tgtman": {bead_id: "bf-tgtman", resolved_at: $ts,
     resolution_type: "bead_closure", reason: "test", verified: true}},
     metadata: {version: "1.0"}}' > "$SBX/.beads/state/crash-resolutions.json"
FAKE_BEAD_FAIL_SHOW=1 "$SBX_TRACKER" bf-tgtman check >/dev/null 2>&1; rc=$?
assert_exit "ancient cached closure still defers to cache when store unreadable" 0 "$rc"

# ----------------------------------------------------------------------------
echo "=== Scenario: usage contract (D-6) ==="

"$SBX_DEDUP" check >/dev/null 2>&1; rc=$?
assert_exit "check without bead id: usage (exit 2)" 2 "$rc"
"$SBX_DEDUP" check ../etc/passwd >/dev/null 2>&1; rc=$?
assert_exit "check with non-bead id: usage (exit 2)" 2 "$rc"
"$SBX_DEDUP" totally-bogus >/dev/null 2>&1; rc=$?
assert_exit "unknown mode: usage (exit 2)" 2 "$rc"
"$SBX_DEDUP" --help >/dev/null 2>&1; rc=$?
assert_exit "--help exits 0" 0 "$rc"

# Store unreadable -> fail open, never suppress on a broken gate.
FAKE_BEAD_FAIL_LIST=1 "$SBX_DEDUP" check bf-alertun >/dev/null 2>&1; rc=$?
assert_exit "unreadable store fails OPEN (exit 3, proceed)" 3 "$rc"

# ----------------------------------------------------------------------------
echo "=== Scenario: report mode semantics (D-6) ==="

# Three crash-class events on ONE bead inside the window -> duplicate pattern.
now_epoch=$(date +%s)
mk_ts() { date -u -d "@$((now_epoch - $1))" +%Y-%m-%dT%H:%M:%S+00:00; }
{
    for i in 0 1 2; do
        printf '{"event":"crash","bead":"bf-rep01","ts":"%s","exit_code":-1}\n' "$(mk_ts $((i * 3600)))"
    done
    # Six DIFFERENT beads, one crash each, same day, same exit code: the old
    # signature branch called this a "REPEATING CRASH SIGNATURE" (gap D-6).
    for b in 1 2 3 4 5 6; do
        printf '{"event":"fail","bead":"bf-wave0%s","ts":"%s","exit_code":1}\n' "$b" "$(mk_ts 60)"
    done
    # Old events outside the window are ignored.
    printf '{"event":"crash","bead":"bf-old01","ts":"%s","exit_code":-1}\n' \
        "$(date -u -d "@$((now_epoch - 86400 * 5))" +%Y-%m-%dT%H:%M:%S+00:00)"
} > "$SBX/.beads/events.jsonl"

out=$(cd /var/tmp && "$SBX_DEDUP" report 2>&1); rc=$?
assert_exit "report exits 0" 0 "$rc"
assert_contains "genuine per-bead repeat is flagged" "$out" "DUPLICATE ALERT PATTERN: bead bf-rep01 had 3 crash-class events"
assert_contains "fleet-wide same-day same-exit coincidence is infrastructure, not a duplicate" "$out" "FLEET-WIDE CRASH OBSERVATION"
assert_not_contains "old signature false positive is gone" "$out" "REPEATING CRASH SIGNATURE"
assert_not_contains "out-of-window event is ignored" "$out" "bf-old01"

# All-clear output must not contain the word "duplicate" — the manager's
# legacy wiring greps for it (the inversion that suppressed genuine alerts).
printf '{"event":"crash","bead":"bf-old02","ts":"%s","exit_code":-1}\n' \
    "$(date -u -d "@$((now_epoch - 86400 * 5))" +%Y-%m-%dT%H:%M:%S+00:00)" > "$SBX/.beads/events.jsonl"
out=$("$SBX_DEDUP" report 2>&1)
assert_contains "quiet window says so" "$out" "No crash-class events recorded"
assert_not_contains "all-clear output contains no 'duplicate' string (grep inversion fixed)" "$out" "duplicate"

# ----------------------------------------------------------------------------
echo "=== Scenario: paths derive from BASH_SOURCE, not the caller CWD (D-6) ==="

out=$(cd / && "$SBX_DEDUP" check bf-alertcl 2>&1); rc=$?
assert_exit "invoked from / still resolves the sandbox store" 0 "$rc"
out=$(cd /tmp && "$SBX_DEDUP" report >/dev/null 2>&1); rc=$?
assert_exit "report invoked from /tmp still finds the sandbox events.jsonl" 0 "$rc"

# ----------------------------------------------------------------------------
echo "=== Scenario: ledger stays a cache — live store is the authority (D-1) ==="

# mark-unresolved must actually flip a live-cached verdict once the store
# itself is the source of truth: a closed bead stays resolved while closed...
out=$("$SBX_TRACKER" bf-tgtclosed check >/dev/null 2>&1); rc=$?
assert_exit "still resolved while the bead is closed" 0 "$rc"
# ...and flipping the store to open flips the verdict even though the ledger
# holds a bead_closure record for it.
set_status "bf-tgtclosed" "open"
out=$("$SBX_TRACKER" bf-tgtclosed check >/dev/null 2>&1); rc=$?
assert_exit "store flipped to open: verdict follows the store, not the cache" 1 "$rc"
set_status "bf-tgtclosed" "closed"
out=$("$SBX_TRACKER" bf-tgtclosed check >/dev/null 2>&1); rc=$?
assert_exit "store flipped back to closed: resolved again" 0 "$rc"

# ----------------------------------------------------------------------------
echo "=== Live-store scenarios (DEDUP_TEST_LIVE=1) ==="
if [[ "${DEDUP_TEST_LIVE:-0}" == "1" ]]; then
    # The gap analysis's D-1 reproduction: bf-173o7e is Closed and used to
    # report NOT_RESOLVED because the ledger was empty.
    if bead show bf-173o7e 2>/dev/null | grep -qi '^Status: Closed'; then
        out=$("$SCRIPT_DIR/crash-resolution-tracker.sh" bf-173o7e check 2>&1); rc=$?
        assert_exit "LIVE bf-173o7e (Closed) is RESOLVED via live evaluation (D-1 repro inverted)" 0 "$rc"
    else
        echo "  SKIP - live bf-173o7e no longer Closed; D-1 repro needs a closed bead"
        TESTS_RUN=$((TESTS_RUN + 1))
    fi
else
    echo "  (skipped — set DEDUP_TEST_LIVE=1 to run against the real bead store)"
fi

# ----------------------------------------------------------------------------
echo
echo "=========================================="
echo "Test Summary"
echo "=========================================="
echo "Total tests: $TESTS_RUN"
echo "Passed: $TESTS_PASSED"
echo "Failed: $((TESTS_RUN - TESTS_PASSED))"

if [[ "$TESTS_RUN" -eq 0 ]]; then
    echo "ERROR: no tests ran"
    exit 1
fi
if [[ "$TESTS_PASSED" -eq "$TESTS_RUN" ]]; then
    echo "All tests passed!"
    exit 0
fi
exit 1
