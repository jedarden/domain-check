#!/usr/bin/env bash
# Test crash-classifier.sh classification signals (domchk-701bcfa5)
#
# Covers the evidence checks the crash-response-guide decision tree adds on
# top of the raw exit code, for exit -1 crashes on still-open beads:
#   1. deliverable commit within COMMIT_WINDOW_SEC (30s)  -> FALSE_POSITIVE
#   2. fleet-wide clustering (>= CLUSTER_MIN_BEADS other
#      beads within CLUSTER_WINDOW_SEC)                   -> INFRASTRUCTURE
#   3. memory exhausted (MemAvailable < MEM_FLOOR_KB)     -> INFRASTRUCTURE
#   4. repository bloat (.git > REPO_BLOAT_LIMIT_BYTES)   -> INFRASTRUCTURE
# plus precedence between them and the pre-signal verdicts (closed bead,
# max_turns, 503, provenance mismatch) staying intact.
#
# Every case runs the real classifier in a throwaway sandbox (mock `bead`
# binary, fabricated traces/events, its own git history). Live measurements
# are overridden (MEM_AVAILABLE_KB / REPO_BYTES) so the suite is hermetic and
# does not depend on the health of the box it runs on.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CLASSIFIER="$SCRIPT_DIR/crash-classifier.sh"

PASS=0
FAIL=0
SANDBOX=""
OUT=""

# Fixed crash instant: 2026-08-16T17:21:00Z (the bf-3561g-class era).
T="$(date -u -d "2026-08-16T17:21:00Z" +%s)"
WS_ISO="$(date -u -d "@$((T - 60))" +%Y-%m-%dT%H:%M:%SZ)"
WE_ISO="$(date -u -d "@${T}" +%Y-%m-%dT%H:%M:%SZ)"

# Hermetic measurement overrides: box health must not decide test outcomes.
HERMETIC_MEM=67108864   # 64 GiB MemAvailable — memory signal suppressed
HERMETIC_REPO=1048576   # 1 MiB .git          — bloat signal suppressed

# Per-case extra env, consumed by run_classify and reset afterwards.
EXTRA_ENV=()

pass() { PASS=$((PASS + 1)); echo "  ✓ PASS: $1"; }
fail() {
    FAIL=$((FAIL + 1))
    echo "  ✗ FAIL: $1"
    if [ -n "${2:-}" ]; then
        echo "    got: ${2}" | head -8
    fi
}

new_sandbox() {
    [ -n "$SANDBOX" ] && rm -rf "$SANDBOX"
    SANDBOX="$(mktemp -d /tmp/classifier-signals.XXXXXX)"
    EXTRA_ENV=()
    mkdir -p "$SANDBOX/bin" "$SANDBOX/.beads/traces"

    cat > "$SANDBOX/bin/bead" <<'EOF'
#!/usr/bin/env bash
echo "Status: $(cat "$MOCK_STATUS_FILE" 2>/dev/null || echo open)"
EOF
    chmod +x "$SANDBOX/bin/bead"

    git init -q "$SANDBOX"
    git -C "$SANDBOX" config user.email github@jedarden.com
    git -C "$SANDBOX" config user.name jedarden
}

# make_commit <bead-id> <epoch> — a deliverable commit mentioning the bead.
# Both dates pinned: the timing signal compares the COMMITTER date (%ct).
make_commit() {
    GIT_AUTHOR_DATE="$2 +0000" GIT_COMMITTER_DATE="$2 +0000" \
        git -C "$SANDBOX" commit --allow-empty -q \
        -m "feat: deliverable for $1 (verified work completion)"
}

# build_fixture <id> <exit_code> <status> <trace-text> [captured_at_iso]
build_fixture() {
    local id="$1" ec="$2" status="$3" trace="$4"
    local captured="${5:-$WE_ISO}"
    mkdir -p "$SANDBOX/.beads/traces/$id"
    printf '{"exit_code": %s, "captured_at": "%s"}\n' "$ec" "$captured" \
        > "$SANDBOX/.beads/traces/$id/metadata.json"
    printf '%s\n' "$trace" > "$SANDBOX/.beads/traces/$id/trace.jsonl"
    echo "$status" > "$SANDBOX/.beads/$id.status"
}

# add_events <id> <exit_code> <ts-iso> — one crash record.
add_events() {
    printf '{"bead":"%s","event":"crash","exit_code":%s,"ts":"%s"}\n' \
        "$1" "$2" "$3" >> "$SANDBOX/.beads/events.jsonl"
}

# add_cluster_events <n> — n distinct OTHER beads crashing T-300s.
add_cluster_events() {
    local n="$1" i ts
    ts="$(date -u -d "@$((T - 300))" +%Y-%m-%dT%H:%M:%SZ)"
    for ((i = 1; i <= n; i++)); do
        add_events "bf-cluster-$i" -1 "$ts"
    done
}

run_classify() {
    local id="$1"
    OUT="$(cd "$SANDBOX" && env \
        PATH="$SANDBOX/bin:$PATH" \
        MOCK_STATUS_FILE="$SANDBOX/.beads/$id.status" \
        BEADS_EVENTS="$SANDBOX/.beads/events.jsonl" \
        CRASH_WINDOW_START="$WS_ISO" \
        CRASH_WINDOW_END="$WE_ISO" \
        MEM_AVAILABLE_KB="$HERMETIC_MEM" \
        REPO_BYTES="$HERMETIC_REPO" \
        ${EXTRA_ENV[@]+"${EXTRA_ENV[@]}"} \
        "$CLASSIFIER" "$id" 2>&1)"
    EXTRA_ENV=()
}

expect_class() {
    if echo "$OUT" | grep -qE "^(FALSE_POSITIVE|SERVICE_FAILURE|INFRASTRUCTURE|CODE_DEFECT|UNKNOWN)$"; then
        if echo "$OUT" | grep -qE "^${1}\$"; then
            pass "verdict $1"
        else
            fail "verdict $1" "$(echo "$OUT" | grep -E '^(FALSE_POSITIVE|SERVICE_FAILURE|INFRASTRUCTURE|CODE_DEFECT|UNKNOWN)' | head -1)"
        fi
    else
        fail "verdict $1 (classifier produced no verdict line)" "$OUT"
    fi
}

expect_reason() {
    if echo "$OUT" | grep -qF "$1"; then
        pass "reason contains '$1'"
    else
        fail "reason contains '$1'" "$OUT"
    fi
}

absent_reason() {
    if echo "$OUT" | grep -qF "$1"; then
        fail "reason must NOT contain '$1'" "$OUT"
    else
        pass "reason does not contain '$1'"
    fi
}

NEUTRAL_TRACE='{"tool":"Bash","input":{"command":"ls -la"}}'

echo "=========================================="
echo "Testing crash-classifier.sh signals"
echo "=========================================="
echo ""

# --- 1. Generic verdict preserved: no signal data -> INFRASTRUCTURE --------
echo "Case 1: exit -1, open bead, no signal evidence -> generic INFRASTRUCTURE"
new_sandbox
build_fixture bf-sig-1 -1 open "$NEUTRAL_TRACE"
add_events bf-sig-1 -1 "$WE_ISO"
run_classify bf-sig-1
expect_class INFRASTRUCTURE
absent_reason "Fleet-wide crash clustering"
absent_reason "Memory exhausted"
absent_reason "Repository bloat"
echo ""

# --- 2. Completion timing: deliverable committed 10s before the crash ------
echo "Case 2: deliverable commit 10s before crash -> FALSE_POSITIVE"
new_sandbox
build_fixture bf-sig-2 -1 open "$NEUTRAL_TRACE"
add_events bf-sig-2 -1 "$WE_ISO"
make_commit bf-sig-2 $((T - 10))
run_classify bf-sig-2
expect_class FALSE_POSITIVE
expect_reason "within 30s of the crash instant"
expect_reason "$(date -u -d "@$((T - 10))" +%Y-%m-%dT%H:%M:%SZ)"
echo ""

# --- 3. Commit outside the 30s window -> falls through to INFRASTRUCTURE ---
echo "Case 3: commit 60s before crash (outside window) -> INFRASTRUCTURE"
new_sandbox
build_fixture bf-sig-3 -1 open "$NEUTRAL_TRACE"
add_events bf-sig-3 -1 "$WE_ISO"
make_commit bf-sig-3 $((T - 60))
run_classify bf-sig-3
expect_class INFRASTRUCTURE
absent_reason "within 30s of the crash instant"
echo ""

# --- 4. Closed bead wins even when a fleet cluster is present --------------
echo "Case 4: CLOSED bead beats the fleet-cluster signal -> FALSE_POSITIVE"
new_sandbox
build_fixture bf-sig-4 -1 closed "$NEUTRAL_TRACE"
add_events bf-sig-4 -1 "$WE_ISO"
add_cluster_events 12
run_classify bf-sig-4
expect_class FALSE_POSITIVE
absent_reason "Fleet-wide crash clustering"
echo ""

# --- 5. Fleet-wide clustering -> INFRASTRUCTURE with the count -------------
echo "Case 5: 12 other beads crashing within the window -> INFRASTRUCTURE (fleet wave)"
new_sandbox
build_fixture bf-sig-5 -1 open "$NEUTRAL_TRACE"
add_events bf-sig-5 -1 "$WE_ISO"
add_cluster_events 12
run_classify bf-sig-5
expect_class INFRASTRUCTURE
expect_reason "Fleet-wide crash clustering"
expect_reason "12 other beads"
echo ""

# --- 6. Cluster below threshold -> generic verdict, not a fleet wave -------
echo "Case 6: only 3 other beads -> below CLUSTER_MIN_BEADS, generic INFRASTRUCTURE"
new_sandbox
build_fixture bf-sig-6 -1 open "$NEUTRAL_TRACE"
add_events bf-sig-6 -1 "$WE_ISO"
add_cluster_events 3
run_classify bf-sig-6
expect_class INFRASTRUCTURE
absent_reason "Fleet-wide crash clustering"
echo ""

# --- 7. Memory exhaustion -> INFRASTRUCTURE with the evidence --------------
echo "Case 7: MemAvailable below floor -> INFRASTRUCTURE (memory)"
new_sandbox
build_fixture bf-sig-7 -1 open "$NEUTRAL_TRACE"
add_events bf-sig-7 -1 "$WE_ISO"
EXTRA_ENV=("MEM_AVAILABLE_KB=1048576")
run_classify bf-sig-7
expect_class INFRASTRUCTURE
expect_reason "Memory exhausted"
expect_reason "MemAvailable 1048576kB below"
echo ""

# --- 8. Repository bloat -> INFRASTRUCTURE with the size -------------------
echo "Case 8: .git over 500MB -> INFRASTRUCTURE (bloat)"
new_sandbox
build_fixture bf-sig-8 -1 open "$NEUTRAL_TRACE"
add_events bf-sig-8 -1 "$WE_ISO"
EXTRA_ENV=("REPO_BYTES=600000000")
run_classify bf-sig-8
expect_class INFRASTRUCTURE
expect_reason "Repository bloat"
expect_reason ".git measures 600000000 bytes"
echo ""

# --- 9. Precedence: completion timing beats the fleet cluster --------------
echo "Case 9: commit 10s before crash AND 12-bead cluster -> FALSE_POSITIVE"
new_sandbox
build_fixture bf-sig-9 -1 open "$NEUTRAL_TRACE"
add_events bf-sig-9 -1 "$WE_ISO"
add_cluster_events 12
make_commit bf-sig-9 $((T - 10))
run_classify bf-sig-9
expect_class FALSE_POSITIVE
expect_reason "within 30s of the crash instant"
absent_reason "Fleet-wide crash clustering"
echo ""

# --- 10-12. Legacy verdicts unchanged --------------------------------------
echo "Case 10: max_turns trace -> FALSE_POSITIVE (legacy)"
new_sandbox
build_fixture bf-sig-10 1 open '{"error":"error_max_turns exceeded"}'
add_events bf-sig-10 1 "$WE_ISO"
run_classify bf-sig-10
expect_class FALSE_POSITIVE
echo ""

echo "Case 11: HTTP 503 trace -> SERVICE_FAILURE (legacy)"
new_sandbox
build_fixture bf-sig-11 1 open '{"error":"503 no available server"}'
add_events bf-sig-11 1 "$WE_ISO"
run_classify bf-sig-11
expect_class SERVICE_FAILURE
echo ""

echo "Case 12: exit -1 with CLOSED bead -> FALSE_POSITIVE (legacy)"
new_sandbox
build_fixture bf-sig-12 -1 closed "$NEUTRAL_TRACE"
add_events bf-sig-12 -1 "$WE_ISO"
run_classify bf-sig-12
expect_class FALSE_POSITIVE
echo ""

# --- 13. Provenance mismatch intact ----------------------------------------
echo "Case 13: trace slot holds a SUCCESS run (mismatch) -> events drive INFRASTRUCTURE"
new_sandbox
build_fixture bf-sig-13 -1 open '{"result":"task done"}' "2026-08-17T11:06:00Z"
add_events bf-sig-13 -1 "$WE_ISO"
run_classify bf-sig-13
expect_class INFRASTRUCTURE
absent_reason "task completed successfully"
echo ""

# --- 14. Usage documents the signal knobs ----------------------------------
echo "Case 14: usage text documents the signal knobs"
OUT="$("$CLASSIFIER" 2>&1 || true)"
expect_reason "COMMIT_WINDOW_SEC"
expect_reason "CLUSTER_MIN_BEADS"
expect_reason "MEM_FLOOR_KB"
expect_reason "REPO_BLOAT_LIMIT_BYTES"
expect_reason "MEM_AVAILABLE_KB"
expect_reason "REPO_BYTES"
echo ""

# --- 15. OOM trace text corroborated by repo bloat -------------------------
echo "Case 15: OOM trace text + bloated repo -> INFRASTRUCTURE with bloat evidence"
new_sandbox
build_fixture bf-sig-15 137 open '{"error":"child killed: out of memory"}'
add_events bf-sig-15 137 "$WE_ISO"
EXTRA_ENV=("REPO_BYTES=600000000")
run_classify bf-sig-15
expect_class INFRASTRUCTURE
expect_reason "OOM killer or memory exhaustion"
expect_reason "bloat-OOM mechanism"
echo ""

[ -n "$SANDBOX" ] && rm -rf "$SANDBOX"

echo "=========================================="
echo "Results: $PASS passed, $FAIL failed"
echo "=========================================="

if [ "$FAIL" -eq 0 ]; then
    exit 0
fi
exit 1
