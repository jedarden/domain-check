#!/usr/bin/env bash
# Tests for the resource safeguards in safe-git-gc.sh
#
# Covers the bf-65lsdu / domchk-1b9940a3 crash mechanism (OOM during git gc,
# amplified by concurrent gc runs):
#   1. mem_to_bytes size parsing
#   2. cgroup ceiling: over-limit process is killed, under-limit passes
#   3. fallback mode (SAFE_GC_NO_CGROUP=1) runs without the ceiling
#   4. box-wide lock: contended run skips cleanly, free lock is acquired
#   5. --check-only still reports an unhealthy-free repo without taking the lock
#   6. systemd unit files validate (no fatal errors)
#   7. configuration validation (invalid sizes, ceiling below the soft sum)
#   8. resource validation (memory / disk / load thresholds fail fast)
#   9. memory-enforcement resolution (cgroup -> ulimit -> none)
#  10. ulimit fallback bounds an over-limit process
#  11. progress checkpoints: running / interrupted / resume-at-stage
#  12. checkpoint/resume end-to-end in a scratch repository
#
# Tests that run a real git gc on THIS repository (the slow, disruptive ones)
# are gated behind DOMCHECK_RUN_LONG_TESTS=1, matching the convention used by
# the Go memory-growth tests. Everything else runs in seconds.
#
# Usage: scripts/test-safe-git-gc-limits.sh
#        DOMCHECK_RUN_LONG_TESTS=1 scripts/test-safe-git-gc-limits.sh

set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
LIB="$SCRIPT_DIR/safe-git-gc.sh"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

PASS=0
FAIL=0

pass() { echo "  ✅ PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  ❌ FAIL: $1"; FAIL=$((FAIL + 1)); }

command -v systemd-run >/dev/null 2>&1 && CGROUP_AVAILABLE=true || CGROUP_AVAILABLE=false

echo "=== safe-git-gc memory ceiling & serialization tests ==="
echo ""

echo "[1] mem_to_bytes size parsing"
(
  set -euo pipefail
  # shellcheck source=/dev/null
  source "$LIB"
  [[ "$(mem_to_bytes 2g)" == "$((2 * 1024 * 1024 * 1024))" ]] || exit 1
  [[ "$(mem_to_bytes 512M)" == "$((512 * 1024 * 1024))" ]] || exit 1
  [[ "$(mem_to_bytes 1024k)" == "$((1024 * 1024))" ]] || exit 1
  [[ "$(mem_to_bytes 4096)" == "4096" ]] || exit 1
) && pass "parses g/M/k and bare byte sizes" || fail "size parsing"

echo ""
echo "[2] cgroup memory ceiling"
if [[ "$CGROUP_AVAILABLE" == true ]]; then
  (
    set -euo pipefail
    # The hard ceiling is SAFE_GC_CGROUP_MAX; SAFE_GC_MEMORY_MAX only sets the
    # soft pack.windowMemory. SAFE_GC_DELTA_CACHE is lowered so validate_config
    # does not reject the tiny ceiling used here.
    SAFE_GC_NO_CGROUP=0 SAFE_GC_CGROUP_MAX=64M SAFE_GC_MEMORY_MAX=8M SAFE_GC_DELTA_CACHE=1M \
      SAFE_GC_CHECKPOINT=/tmp/domchk-test-checkpoint.json \
      bash -c '
        source "'"$LIB"'"
        resolve_memory_enforcement
        [[ "$CGROUP_CAP" == "on" ]] || { echo "cap unavailable" >&2; exit 99; }
        run_memory_capped bash -c "d=\$(head -c 300M /dev/zero | tr \"\\0\" x)" >/dev/null 2>&1
      '
    rc=$?
    # 137 = SIGKILL from the cgroup OOM; anything else means the ceiling failed
    [[ $rc -eq 137 ]] || exit 1
  ) && pass "over-limit process killed by cgroup (SIGKILL)" \
     || fail "over-limit process survived the 64M ceiling"

  (
    set -euo pipefail
    SAFE_GC_NO_CGROUP=0 SAFE_GC_CGROUP_MAX=64M SAFE_GC_MEMORY_MAX=8M SAFE_GC_DELTA_CACHE=1M \
      SAFE_GC_CHECKPOINT=/tmp/domchk-test-checkpoint.json \
      bash -c '
        source "'"$LIB"'"
        resolve_memory_enforcement
        run_memory_capped true
      '
  ) && pass "under-limit process completes normally" \
     || fail "under-limit process failed"
else
  echo "  ⚠️  SKIP: systemd-run not available"
fi

echo ""
echo "[3] fallback mode without cgroup ceiling"
(
  set -euo pipefail
  SAFE_GC_NO_CGROUP=1 SAFE_GC_MEMORY_MAX=64M \
    SAFE_GC_CHECKPOINT=/tmp/domchk-test-checkpoint.json \
    bash -c '
      source "'"$LIB"'"
      resolve_memory_enforcement
      [[ "$CGROUP_CAP" == "off" ]] || exit 1
      run_memory_capped true
    '
) && pass "SAFE_GC_NO_CGROUP=1 disables the ceiling and still runs" \
   || fail "fallback mode broken"

echo ""
echo "[4] box-wide gc lock"
if [[ "${DOMCHECK_RUN_LONG_TESTS:-0}" != "1" ]]; then
  echo "  ⚠️  SKIP: set DOMCHECK_RUN_LONG_TESTS=1 (these run a real git gc on this repository)"
else
TEST_LOCK="/tmp/domchk-test-gc-lock-$$.lock"
cleanup_holder() { kill "$HOLDER_PID" 2>/dev/null || true; wait "$HOLDER_PID" 2>/dev/null || true; rm -f "$TEST_LOCK"; }
HOLDER_PID=""
trap cleanup_holder EXIT

(
  set -euo pipefail
  exec 9>"$TEST_LOCK"
  flock 9
  sleep 6
) &
HOLDER_PID=$!
sleep 0.3

OUT=$(SAFE_GC_LOCK_FILE="$TEST_LOCK" SAFE_GC_LOCK_WAIT=2 \
  bash "$LIB" 2>&1)
RC=$?
if [[ $RC -eq 0 ]] && grep -q "Another git gc holds" <<<"$OUT"; then
  pass "contended run skips cleanly with exit 0"
else
  fail "contended run: rc=$RC, expected clean skip message"
fi

cleanup_holder
OUT=$(SAFE_GC_LOCK_FILE="$TEST_LOCK" SAFE_GC_LOCK_WAIT=5 \
  bash "$LIB" 2>&1)
RC=$?
if [[ $RC -eq 0 ]] && grep -q "Acquired gc lock" <<<"$OUT"; then
  pass "free lock is acquired and run proceeds"
else
  fail "uncontended run: rc=$RC, expected lock acquisition"
fi
trap - EXIT
fi

echo ""
echo "[5] --check-only unchanged"
OUT=$(bash "$LIB" --check-only 2>&1)
RC=$?
if [[ $RC -eq 1 ]] && grep -q "GC not needed" <<<"$OUT"; then
  pass "--check-only reports 'GC not needed' (exit 1) on a healthy repo"
else
  fail "--check-only: rc=$RC, expected exit 1 with healthy repo"
fi

echo ""
echo "[6] systemd unit validation"
if command -v systemd-analyze >/dev/null 2>&1; then
  UNIT_ERR=$(systemd-analyze --user verify \
    "$SCRIPT_DIR/domain-check-git-gc.service" \
    "$SCRIPT_DIR/domain-check-git-gc-full.service" 2>&1)
  RC=$?
  if [[ $RC -eq 0 && ! ("$UNIT_ERR" == *fatal*) ]]; then
    pass "gc and gc-full unit files load without fatal errors"
  else
    fail "unit validation: rc=$RC: $UNIT_ERR"
  fi
else
  echo "  ⚠️  SKIP: systemd-analyze not available"
fi

echo ""
echo "[7] configuration validation"

# Everything from here on runs inside throwaway repositories: logs,
# checkpoints and git operations stay out of this repo entirely.
# pack.threads=1 mirrors what setup-git-gc-config.sh installs on this box,
# which makes validate_config's worst-case arithmetic hermetic (an unpinned
# threads count would multiply the window by nproc and reject the default
# ceiling on a 12-core host).
WORKROOT="$(mktemp -d)"
cleanup_workroot() {
  if [[ "${DOMCHECK_KEEP_SGGL:-0}" != "1" && "$FAIL" -eq 0 ]]; then
    rm -rf "$WORKROOT"
  else
    echo "   (artifacts kept: $WORKROOT)"
  fi
}
trap cleanup_workroot EXIT

make_scratch_repo() {
  local dir="$1"
  git init -q -b main "$dir"
  git -C "$dir" config user.email github@jedarden.com
  git -C "$dir" config user.name jedarden
  git -C "$dir" config pack.threads 1
  echo x > "$dir/file.txt"
  git -C "$dir" add file.txt
  git -C "$dir" commit -qm "scratch"
}

seed_ckpt() {  # seed_ckpt <file> <stage> <status> <pid>
  cat > "$1" <<EOF
{"timestamp": "$(date -Iseconds)", "stage": "$2", "status": "$3", "message": "seeded by test", "pid": ${4:-0}, "mode": "standard"}
EOF
}

# Source the library inside the scratch repo and echo the snippet's exit code.
# Env assignments (arg 1) must precede the source: MEMORY_MAX, CGROUP_MAX and
# DELTA_CACHE are snapshotted from the environment when the library is sourced.
run_scratch() {  # run_scratch "<VAR=VAL ...>" "<snippet>"
  (
    cd "$WORKROOT/repo" || exit 97
    env $1 bash -c '
      source "'"$LIB"'"
      '"$2"'
    ' >/dev/null 2>&1
    echo "$?"
  )
}

# Same, but keeps the snippet's output so callers can assert on messages.
run_scratch_out() {  # run_scratch_out "<VAR=VAL ...>" "<snippet>" <outfile>
  (
    cd "$WORKROOT/repo" || exit 97
    env $1 bash -c '
      source "'"$LIB"'"
      '"$2"'
    ' >"$3" 2>&1
    echo "$?"
  )
}

make_scratch_repo "$WORKROOT/repo"

[[ "$(run_scratch "" "validate_config")" == "0" ]] \
  && pass "defaults validate (window 2g, delta 1g, ceiling 6g, threads pinned)" \
  || fail "default configuration rejected"

[[ "$(run_scratch "SAFE_GC_MEMORY_MAX=bogus" "validate_config")" == "2" ]] \
  && pass "invalid SAFE_GC_MEMORY_MAX refused (exit 2)" \
  || fail "invalid SAFE_GC_MEMORY_MAX not refused"

[[ "$(run_scratch "SAFE_GC_CGROUP_MAX=bogus" "validate_config")" == "2" ]] \
  && pass "invalid SAFE_GC_CGROUP_MAX refused (exit 2)" \
  || fail "invalid SAFE_GC_CGROUP_MAX not refused"

[[ "$(run_scratch "SAFE_GC_DELTA_CACHE=bogus" "validate_config")" == "2" ]] \
  && pass "invalid SAFE_GC_DELTA_CACHE refused (exit 2)" \
  || fail "invalid SAFE_GC_DELTA_CACHE not refused"

# A ceiling below the soft worst case is the self-inflicted OOM the ceiling
# exists to prevent (stepwise-git-gc-strategy.md §7.1 item 1) — refuse it.
[[ "$(run_scratch "SAFE_GC_CGROUP_MAX=1g" "validate_config")" == "2" ]] \
  && pass "ceiling below the soft worst case refused (2g window + 1g delta > 1g)" \
  || fail "undersized ceiling accepted"

[[ "$(run_scratch "SAFE_GC_CGROUP_MAX=8g" "validate_config")" == "0" ]] \
  && pass "ceiling covering the soft worst case accepted" \
  || fail "adequate ceiling rejected"

echo ""
echo "[8] resource validation fails fast"

OUT8="$WORKROOT/out8.txt"
[[ "$(run_scratch_out "" "check_resources" "$OUT8")" == "0" ]] \
  && pass "a healthy box passes resource checks" \
  || { fail "resource checks rejected a healthy box"; cat "$OUT8"; }

# Floors set just past what the box actually has: deterministic failures that
# prove the fail-fast path without depending on absolute thresholds.
RC=$(run_scratch_out \
  "SAFE_GC_MIN_AVAIL_MEM=$(( $(awk '/^MemAvailable:/{print $2}' /proc/meminfo) / 1024 + 1 ))M" \
  "check_resources" "$OUT8")
if [[ "$RC" == "2" ]] && grep -q "Insufficient memory" "$OUT8"; then
  pass "memory floor above availability fails with exit 2"
else
  fail "memory threshold: rc=$RC, expected 2 + 'Insufficient memory'"
fi

RC=$(run_scratch_out \
  "SAFE_GC_MIN_DISK_GB=$(( $(df -BG --output=avail "$WORKROOT" | tail -1 | tr -dc '0-9') + 1 ))" \
  "check_resources" "$OUT8")
if [[ "$RC" == "2" ]] && grep -q "Insufficient disk" "$OUT8"; then
  pass "disk floor above free space fails with exit 2"
else
  fail "disk threshold: rc=$RC, expected 2 + 'Insufficient disk'"
fi

RC=$(run_scratch_out \
  "SAFE_GC_MAX_LOAD=$(awk '{printf "%.2f", $1 - 1}' /proc/loadavg)" \
  "check_resources" "$OUT8")
if [[ "$RC" == "2" ]] && grep -q "load too high" "$OUT8"; then
  pass "load ceiling below the current load fails with exit 2"
else
  fail "load threshold: rc=$RC, expected 2 + 'load too high'"
fi

RC=$(run_scratch_out "SAFE_GC_MIN_AVAIL_MEM=bogus" "check_resources" "$OUT8")
if [[ "$RC" == "2" ]] && grep -q "not a valid size" "$OUT8"; then
  pass "invalid SAFE_GC_MIN_AVAIL_MEM refused"
else
  fail "invalid SAFE_GC_MIN_AVAIL_MEM: rc=$RC, expected 2 + 'not a valid size'"
fi

echo ""
echo "[9] memory-enforcement resolution (cgroup -> ulimit -> none)"
if [[ "$CGROUP_AVAILABLE" != true ]]; then
  echo "  ⚠️  SKIP: systemd-run not available"
else
  [[ "$(run_scratch "" 'resolve_memory_enforcement
[[ "$CGROUP_CAP" == "on" && "$ULIMIT_CAP" == "off" ]]')" == "0" ]] \
    && pass "default resolves to the cgroup tier" \
    || fail "cgroup tier not selected by default"

  [[ "$(run_scratch "SAFE_GC_NO_CGROUP=1" 'resolve_memory_enforcement
[[ "$CGROUP_CAP" == "off" && "$ULIMIT_CAP" == "on" ]]')" == "0" ]] \
    && pass "SAFE_GC_NO_CGROUP=1 falls back to the ulimit tier" \
    || fail "ulimit tier not selected when the cgroup is opted out"

  [[ "$(run_scratch "SAFE_GC_NO_CGROUP=1 SAFE_GC_NO_ULIMIT=1" 'resolve_memory_enforcement
[[ "$CGROUP_CAP" == "off" && "$ULIMIT_CAP" == "off" ]]')" == "0" ]] \
    && pass "both opt-outs leave soft git limits only" \
    || fail "opt-outs did not clear both tiers"

  [[ "$(run_scratch "SAFE_GC_NO_ULIMIT=1" 'resolve_memory_enforcement
[[ "$CGROUP_CAP" == "on" && "$ULIMIT_CAP" == "off" ]]')" == "0" ]] \
    && pass "SAFE_GC_NO_ULIMIT=1 alone keeps the cgroup tier" \
    || fail "SAFE_GC_NO_ULIMIT=1 disturbed the cgroup tier"
fi

echo ""
echo "[10] ulimit fallback bounds an over-limit process"
# 1M window + 1M delta + empty pack dir + 512m slack -> ~514M address-space
# cap. A 1G allocation must fail inside the capped process; because the cap
# makes malloc fail before memory is touched, this is safe to run ungated.
ULIMIT_ENV="SAFE_GC_NO_CGROUP=1 SAFE_GC_MEMORY_MAX=1M SAFE_GC_DELTA_CACHE=1M"
RC=$(run_scratch "$ULIMIT_ENV" 'resolve_memory_enforcement
[[ "$ULIMIT_CAP" == "on" ]] || exit 90
run_memory_capped bash -c "d=\$(head -c 1G /dev/zero | tr \"\\0\" x)"')
if [[ "$RC" != "0" ]]; then
  pass "over-limit allocation fails under the ulimit -v cap (rc=$RC)"
else
  fail "over-limit allocation survived the ulimit cap"
fi

RC=$(run_scratch "$ULIMIT_ENV" 'resolve_memory_enforcement
run_memory_capped true')
[[ "$RC" == "0" ]] \
  && pass "under-limit command completes under the ulimit tier" \
  || fail "under-limit command failed under the ulimit tier (rc=$RC)"

echo ""
echo "[11] progress checkpoints"
CKPT="$WORKROOT/progress.json"

(
  set -euo pipefail
  cd "$WORKROOT/repo"
  export SAFE_GC_CHECKPOINT="$CKPT"
  bash -c '
    source "'"$LIB"'"
    save_progress "stage2"
    [[ "$(jq -r ".stage" '"$CKPT"')" == "stage2" ]] || exit 1
    [[ "$(jq -r ".status" '"$CKPT"')" == "running" ]] || exit 1
    [[ "$(jq -r ".pid" '"$CKPT"')" == "$$" ]] || exit 1
  '
) && pass "save_progress records stage, pid and status=running" \
   || fail "save_progress checkpoint is wrong"

(
  set -euo pipefail
  cd "$WORKROOT/repo"
  export SAFE_GC_CHECKPOINT="$CKPT"
  bash -c '
    source "'"$LIB"'"
    mark_interrupted "stage2"
    [[ "$(jq -r ".status" '"$CKPT"')" == "interrupted" ]] || exit 1
  '
) && pass "mark_interrupted records status=interrupted" \
   || fail "mark_interrupted did not record the interrupted status"

# A "running" entry whose pid is gone is rewritten as "interrupted" so the
# monitor stops showing a phantom in-flight gc.
( sleep 0.2 ) & DEAD_PID=$!
wait "$DEAD_PID" 2>/dev/null || true
seed_ckpt "$CKPT" stage1 running "$DEAD_PID"
(
  set -euo pipefail
  cd "$WORKROOT/repo"
  export SAFE_GC_CHECKPOINT="$CKPT"
  bash -c '
    source "'"$LIB"'"
    reap_stale_progress
    [[ "$(jq -r ".status" '"$CKPT"')" == "interrupted" ]] || exit 1
  '
) && pass "reap_stale_progress rewrites a dead run's entry as interrupted" \
   || fail "stale running entry was not reaped"

# A live pid means a gc really is running: the entry is left untouched.
sleep 30 & LIVE_PID=$!
seed_ckpt "$CKPT" stage2 running "$LIVE_PID"
(
  set -euo pipefail
  cd "$WORKROOT/repo"
  export SAFE_GC_CHECKPOINT="$CKPT"
  bash -c '
    source "'"$LIB"'"
    reap_stale_progress
    [[ "$(jq -r ".status" '"$CKPT"')" == "running" ]] || exit 1
    report_running_gc
  ' > "$WORKROOT/out11.txt" 2>&1
) && { grep -q "GC in progress" "$WORKROOT/out11.txt" \
        && pass "a live pid survives reaping and is reported as in progress" \
        || fail "live run was not reported as in progress"; } \
   || fail "reap disturbed a live run's entry"
kill "$LIVE_PID" 2>/dev/null || true
wait "$LIVE_PID" 2>/dev/null || true

check_lc() {  # check_lc <seed-stage|none> <seed-status> <expected-stage> <expected-status>
  local seed_stage="$1" seed_status="$2" want_stage="$3" want_status="$4"
  if [[ "$seed_stage" == "none" ]]; then
    rm -f "$CKPT"
  else
    seed_ckpt "$CKPT" "$seed_stage" "$seed_status" 0
  fi
  (
    set -euo pipefail
    cd "$WORKROOT/repo"
    export SAFE_GC_CHECKPOINT="$CKPT"
    bash -c '
      source "'"$LIB"'"
      load_checkpoint
      [[ "$LAST_STAGE" == "'"$want_stage"'" && "$CHECKPOINT_STATUS" == "'"$want_status"'" ]] || exit 1
    '
  )
}

if check_lc none - none none \
  && check_lc stage1 complete stage1 complete \
  && check_lc stage2 interrupted stage2 interrupted \
  && check_lc stage3 failed stage3 failed; then
  pass "load_checkpoint reports stage + status across none/complete/interrupted/failed"
else
  fail "load_checkpoint stage/status reporting is wrong"
fi

echo ""
echo "[12] checkpoint/resume end-to-end in a scratch repository"

e2e_run() {  # e2e_run <repo> <ckpt> <lock> <args...>; output lands in e2e.out
  local repo="$1" ckpt="$2" lock="$3"
  shift 3
  (
    cd "$repo" || exit 97
    SAFE_GC_CHECKPOINT="$ckpt" SAFE_GC_LOCK_FILE="$lock" \
      bash "$LIB" "$@" >"$WORKROOT/e2e.out" 2>&1
    echo "$?"
  )
}

# (a) a clean run completes every stage and records a complete checkpoint
REPO_A="$WORKROOT/e2e-a"; make_scratch_repo "$REPO_A"
RC=$(e2e_run "$REPO_A" "$REPO_A/.git/ckpt.json" "$WORKROOT/e2e.lock")
if [[ "$RC" == "0" ]] \
  && grep -q "Completed Successfully" "$WORKROOT/e2e.out" \
  && [[ "$(jq -r ".status" "$REPO_A/.git/ckpt.json")" == "complete" ]]; then
  pass "clean run completes all stages and marks the checkpoint complete"
else
  fail "clean run: rc=$RC (out: $WORKROOT/e2e.out)"
fi

# (b) a completed stage is skipped on --resume
REPO_B="$WORKROOT/e2e-b"; make_scratch_repo "$REPO_B"
seed_ckpt "$REPO_B/.git/ckpt.json" stage1 complete 0
RC=$(e2e_run "$REPO_B" "$REPO_B/.git/ckpt.json" "$WORKROOT/e2e.lock" --resume)
if [[ "$RC" == "0" ]] && grep -q "Resuming from stage 2" "$WORKROOT/e2e.out"; then
  pass "--resume continues after a completed stage"
else
  fail "resume past complete stage: rc=$RC, expected 'Resuming from stage 2'"
fi

# (c) an interrupted stage restarts instead of being skipped
REPO_C="$WORKROOT/e2e-c"; make_scratch_repo "$REPO_C"
seed_ckpt "$REPO_C/.git/ckpt.json" stage2 interrupted 0
RC=$(e2e_run "$REPO_C" "$REPO_C/.git/ckpt.json" "$WORKROOT/e2e.lock" --resume)
if [[ "$RC" == "0" ]] && grep -q "Stage 2 was interrupted" "$WORKROOT/e2e.out"; then
  pass "--resume restarts an interrupted stage"
else
  fail "resume of interrupted stage: rc=$RC, expected a stage-2 restart"
fi

# (d) a fully completed run reports and exits cleanly
REPO_D="$WORKROOT/e2e-d"; make_scratch_repo "$REPO_D"
seed_ckpt "$REPO_D/.git/ckpt.json" complete complete 0
RC=$(e2e_run "$REPO_D" "$REPO_D/.git/ckpt.json" "$WORKROOT/e2e.lock" --resume)
if [[ "$RC" == "0" ]] && grep -q "All stages already completed" "$WORKROOT/e2e.out"; then
  pass "--resume on a completed run exits 0 without redoing work"
else
  fail "resume of completed run: rc=$RC, expected a clean no-op"
fi

# (e) a run that died mid-stage is detected and restarted from that stage
REPO_E="$WORKROOT/e2e-e"; make_scratch_repo "$REPO_E"
( sleep 0.2 ) & DEAD_PID=$!
wait "$DEAD_PID" 2>/dev/null || true
seed_ckpt "$REPO_E/.git/ckpt.json" stage1 running "$DEAD_PID"
RC=$(e2e_run "$REPO_E" "$REPO_E/.git/ckpt.json" "$WORKROOT/e2e.lock")
if [[ "$RC" == "0" ]] \
  && grep -q "Previous run died during stage stage1" "$WORKROOT/e2e.out"; then
  pass "a dead run's mid-stage entry is detected and the stage restarted"
else
  fail "dead-run recovery: rc=$RC, expected a mid-stage death notice"
fi

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
[[ $FAIL -eq 0 ]] && exit 0 || exit 1
