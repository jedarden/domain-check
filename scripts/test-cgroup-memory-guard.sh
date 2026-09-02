#!/usr/bin/env bash
# Tests for scripts/cgroup-memory-guard.sh
#
# The decision matrix is exercised against fixture cgroup trees (no root
# needed: MEMGUARD_CGROUP_ROOT / MEMGUARD_PROC_CGROUP point the guard at
# crafted files), then two live checks run against the real cgroup:
#   - the caller's actual dispatch scope must be detected with its real 12G cap
#   - a systemd-run scope with a tiny MemoryMax must be REFUSED by the guard,
#     which is the exact memcg-OOM mechanism from the signal -1 RCA
#     (bf-4x12ec / bf-173o7e)
#
# Usage: scripts/test-cgroup-memory-guard.sh [--skip-live]

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GUARD="$SCRIPT_DIR/cgroup-memory-guard.sh"
SKIP_LIVE=false
[[ "${1:-}" == "--skip-live" ]] && SKIP_LIVE=true

PASS=0
FAIL=0
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

ok()   { echo "  PASS: $1"; PASS=$((PASS + 1)); }
bad()  { echo "  FAIL: $1"; FAIL=$((FAIL + 1)); }

# fixture_tree <leaf_max> <leaf_cur> <slice_max> <slice_cur> [oom_kills]
# Builds a two-level tree and points the guard at it via globals.
FIXTURE=""
fixture_tree() {
  local leaf_max="$1" leaf_cur="$2" slice_max="$3" slice_cur="$4" oom="${5:-0}"
  FIXTURE="$TMP/fixture-$RANDOM"
  mkdir -p "$FIXTURE/root/test.slice/run-x.scope"
  echo "$leaf_max"  > "$FIXTURE/root/test.slice/run-x.scope/memory.max"
  echo "$leaf_cur"  > "$FIXTURE/root/test.slice/run-x.scope/memory.current"
  printf 'oom_kill %s\n' "$oom" > "$FIXTURE/root/test.slice/run-x.scope/memory.events"
  echo "$slice_max" > "$FIXTURE/root/test.slice/memory.max"
  echo "$slice_cur" > "$FIXTURE/root/test.slice/memory.current"
  printf 'oom_kill 0\n' > "$FIXTURE/root/test.slice/memory.events"
  export MEMGUARD_CGROUP_ROOT="$FIXTURE/root"
  export MEMGUARD_PROC_CGROUP="$FIXTURE/map"
  echo "0::/test.slice/run-x.scope" > "$FIXTURE/map"
}

G=1073741824  # 1G

run_check() { bash "$GUARD" --check; }

echo "== Decision matrix (fixture trees) =========="

# 1. Healthy: 8G slice headroom, scope barely used.
fixture_tree $((12*G)) $((1*G)) $((32*G)) $((24*G))
if run_check >/dev/null 2>&1; then ok "healthy tree -> pass (exit 0)"; else bad "healthy tree -> pass (exit 0)"; fi

# 2. THE KEY CASE: slice nearly full (1.5G headroom) while our scope is nearly
#    empty. System-wide free memory would look fine here; the guard must refuse.
fixture_tree $((12*G)) $((1*G)) $((32*G)) $((32*G - 1536*1048576))
run_check >/dev/null 2>&1; rc=$?
if [[ $rc -eq 2 ]]; then ok "slice at 1.5G headroom -> refuse (exit 2)"; else bad "slice at 1.5G headroom -> refuse (got exit $rc)"; fi

# 3. Own scope over the refuse percentage (12G/11G = 92%).
fixture_tree $((12*G)) $((11*G)) $((64*G)) $((12*G))
run_check >/dev/null 2>&1; rc=$?
if [[ $rc -eq 2 ]]; then ok "scope at 92% of own limit -> refuse (exit 2)"; else bad "scope at 92% of own limit -> refuse (got exit $rc)"; fi

# 4. Warn band: scope at 75% of own limit.
fixture_tree $((12*G)) $((9*G)) $((64*G)) $((12*G))
run_check >/dev/null 2>&1; rc=$?
if [[ $rc -eq 1 ]]; then ok "scope at 75% of own limit -> warn (exit 1)"; else bad "scope at 75% of own limit -> warn (got exit $rc)"; fi

# 5. Unbounded tree (all "max") falls back to system-wide MemAvailable.
FIXTURE="$TMP/fixture-unbounded"
mkdir -p "$FIXTURE/root/test.scope"
echo "max" > "$FIXTURE/root/test.scope/memory.max"
echo $((2*G)) > "$FIXTURE/root/test.scope/memory.current"
printf 'oom_kill 0\n' > "$FIXTURE/root/test.scope/memory.events"
export MEMGUARD_CGROUP_ROOT="$FIXTURE/root" MEMGUARD_PROC_CGROUP="$FIXTURE/map"
echo "0::/test.scope" > "$FIXTURE/map"
out="$(bash "$GUARD" --json 2>/dev/null)"; rc=$?
if [[ $rc -eq 0 && "$out" == *'"decision":"pass"'* && "$out" == *MemAvailable* ]]; then
  ok "unbounded tree -> MemAvailable fallback, pass"
else
  bad "unbounded tree -> MemAvailable fallback (rc=$rc out=$out)"
fi

# 6. Unknown state (v1-style map, no 0:: line): fail-open exit 3, --strict refuses.
FIXTURE="$TMP/fixture-unknown"
mkdir -p "$FIXTURE/root"
export MEMGUARD_CGROUP_ROOT="$FIXTURE/root" MEMGUARD_PROC_CGROUP="$FIXTURE/map"
printf '12:cpu:/x\n' > "$FIXTURE/map"
run_check >/dev/null 2>&1; rc=$?
strict_rc=$(bash "$GUARD" --check --strict >/dev/null 2>&1; echo $?)
if [[ $rc -eq 3 && "$strict_rc" -eq 2 ]]; then
  ok "unreadable cgroup state -> unknown exit 3, --strict -> refuse"
else
  bad "unreadable cgroup state (got exit $rc, strict $strict_rc)"
fi

# 7. oom_kill telemetry surfaced in JSON.
fixture_tree $((12*G)) $((1*G)) $((32*G)) $((24*G)) 3
out="$(bash "$GUARD" --json 2>/dev/null)"
if [[ "$out" == *'"oom_kills":3'* ]]; then ok "lifetime oom_kill count reported in JSON"; else bad "oom_kill telemetry missing ($out)"; fi

echo "== Wrapper gating =========="

# 8. Refuse blocks the command.
fixture_tree $((12*G)) $((1*G)) $((32*G)) $((32*G - 1536*1048576))
rm -f "$FIXTURE/marker"
bash "$GUARD" -- touch "$FIXTURE/marker" >/dev/null 2>&1; rc=$?
if [[ $rc -eq 2 && ! -e "$FIXTURE/marker" ]]; then
  ok "wrapper: command NOT run when refused (exit 2, no marker)"
else
  bad "wrapper: refuse gating broken (rc=$rc marker=$([[ -e $FIXTURE/marker ]] && echo present))"
fi

# 9. Pass executes the command and propagates its exit code.
fixture_tree $((12*G)) $((1*G)) $((32*G)) $((24*G))
out="$(bash "$GUARD" -- sh -c 'echo guard-ran-through' 2>/dev/null)"; rc=$?
if [[ "$out" == *"guard-ran-through"* && $rc -eq 0 ]]; then
  ok "wrapper: command runs when headroom is healthy"
else
  bad "wrapper: command not run on pass (rc=$rc out=$out)"
fi

if [[ "$SKIP_LIVE" == true ]]; then
  echo "== Live checks SKIPPED (--skip-live) =========="
else
  echo "== Live checks (real cgroup) =========="

  # 10. The guard must see this dispatch scope's real 12G cap.
  unset MEMGUARD_CGROUP_ROOT MEMGUARD_PROC_CGROUP
  out="$(bash "$GUARD" --json 2>/dev/null)"
  if [[ "$out" == *'"max":12884901888'* || "$out" == *'"max":34359738368'* ]]; then
    ok "live: detected real scope/slice caps (12G scope / 32G needle.slice)"
  else
    bad "live: real caps not detected ($out)"
  fi
  rc=$(bash "$GUARD" --check >/dev/null 2>&1; echo $?)
  if [[ "$rc" =~ ^[0-3]$ ]]; then ok "live: --check exits cleanly (exit $rc)"; else bad "live: --check exit $rc"; fi

  # 11. The exact crash mechanism: run the guard inside a scope with a tiny
  #     MemoryMax and confirm it refuses instead of letting the job plow ahead.
  if command -v systemd-run >/dev/null 2>&1; then
    out="$(systemd-run --user --scope -q -p MemoryMax=64M bash "$GUARD" --check 2>&1)"; rc=$?
    if [[ $rc -eq 2 ]]; then
      ok "live: guard inside MemoryMax=64M scope REFUSES (exit 2) — the crash mechanism is caught"
    else
      bad "live: guard inside MemoryMax=64M scope (rc=$rc): $(echo "$out" | tail -2 | tr '\n' ' ')"
    fi
  else
    echo "  SKIP: systemd-run not available"
  fi
fi

echo ""
echo "Results: $PASS passed, $FAIL failed"
[[ $FAIL -eq 0 ]]
