#!/usr/bin/env bash
# Self-test for scripts/setup-git-gc-config.sh — the persistent pack-memory
# bound (bare-gc/push rollback of the bf-173o7e / bf-4x12ec memcg-OOM class)
# and its --uninstall rollback mode.
#
# Runs entirely inside a throwaway git repo with GIT_CONFIG_GLOBAL and
# GIT_CONFIG_SYSTEM pointed at scratch files, so the box-wide global bound in
# the real ~/.gitconfig is never read, clobbered, or removed. Assertions:
#   - fresh sandbox verifies UNSAFE (exit 1) at both scopes
#   - install applies the three pack.* keys and --verify passes
#   - repo-local install forces gc.auto=0 even over a hand-tuned value (GAP-2
#     of the bf-65lsdu proposal: the unserialized background auto-gc path must
#     stay closed); --global only fills gc.auto when absent, and repo-local 0
#     wins over the global fill
#   - GC_AUTO is the deliberate re-enable escape hatch
#   - --uninstall removes the pack.* keys plus repo-local gc.auto (advisory
#     keys stay)
#   - --uninstall (local) exits 0 while the global scope still supplies the
#     effective bound, and exits 1 when it removed the LAST bound
#   - --verify cannot be combined with --uninstall (exit 2)
#
# Usage: scripts/test-setup-git-gc-config.sh   (no arguments)

set -uo pipefail

REPO_ROOT=$(git rev-parse --show-toplevel)
SCRIPT="$REPO_ROOT/scripts/setup-git-gc-config.sh"

PASS=0
FAIL=0
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

pass() { PASS=$((PASS + 1)); echo "  ✅ $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  ❌ $1"; }

check() { # check <description> <expected: 0|1|2> <command...>
  local desc=$1 want=$2
  shift 2
  local out rc
  out=$("$@" 2>&1)
  rc=$?
  if [ "$rc" -eq "$want" ]; then
    pass "$desc (exit $rc)"
  else
    fail "$desc — wanted exit $want, got $rc"
    echo "----- output -----"
    echo "$out"
    echo "------------------"
  fi
  LAST_OUT=$out
}

expect_output() { # expect_output <pattern> <description>
  if printf '%s' "$LAST_OUT" | grep -q "$1"; then
    pass "$2"
  else
    fail "$2 — pattern not in output: $1"
    printf '%s\n' "$LAST_OUT" | sed 's/^/      /'
  fi
}

cfg_local() { git config --local --get "$1" 2>/dev/null || true; }
cfg_global() { git config --global --get "$1" 2>/dev/null || true; }

section() { echo ""; echo "$1"; }

# ---------------------------------------------------------------- sandbox
section "Sandboxing git config (real ~/.gitconfig untouched)"
export GIT_CONFIG_GLOBAL="$WORK/global-gitconfig"
export GIT_CONFIG_SYSTEM="$WORK/system-gitconfig"
: >"$GIT_CONFIG_GLOBAL"
: >"$GIT_CONFIG_SYSTEM"
git init -q -b main "$WORK/repo" 2>/dev/null || git init -q "$WORK/repo"
cd "$WORK/repo" || exit 1
git config --local user.email github@jedarden.com
git config --local user.name jedarden

# ---------------------------------------------------------------- baseline
section "Fresh sandbox has no bound anywhere"
check "--verify (local) is UNSAFE" 1 "$SCRIPT" --verify
check "--verify --global is UNSAFE" 1 "$SCRIPT" --verify --global
check "--verify rejects --uninstall combination" 2 "$SCRIPT" --verify --uninstall

# ---------------------------------------------------------------- install
section "Install (repo-local)"
git config --local gc.auto 100   # stale hand-tuned value the safety core must override (GAP-2)
check "install applies the local bound" 0 "$SCRIPT"
[ "$(cfg_local pack.windowMemory)" = "2g" ] &&
  pass "pack.windowMemory = 2g" || fail "pack.windowMemory = '$(cfg_local pack.windowMemory)'"
[ "$(cfg_local pack.deltaCacheSize)" = "1g" ] &&
  pass "pack.deltaCacheSize = 1g" || fail "pack.deltaCacheSize = '$(cfg_local pack.deltaCacheSize)'"
[ "$(cfg_local pack.threads)" = "1" ] &&
  pass "pack.threads = 1" || fail "pack.threads = '$(cfg_local pack.threads)'"
[ "$(cfg_local gc.auto)" = "0" ] &&
  pass "gc.auto forced to 0 — background auto-gc path closed" || fail "gc.auto = '$(cfg_local gc.auto)', want 0"
check "--verify passes after install" 0 "$SCRIPT" --verify
check "reinstall is idempotent" 0 "$SCRIPT"

section "GC_AUTO override (deliberate re-enable)"
check "GC_AUTO=100 install exits 0" 0 env GC_AUTO=100 "$SCRIPT"
[ "$(cfg_local gc.auto)" = "100" ] &&
  pass "GC_AUTO=100 honored over the default 0" || fail "GC_AUTO ignored: $(cfg_local gc.auto)"
check "reinstall restores gc.auto=0" 0 "$SCRIPT"
[ "$(cfg_local gc.auto)" = "0" ] &&
  pass "gc.auto back to 0 after plain reinstall" || fail "gc.auto = '$(cfg_local gc.auto)', want 0"

section "Install --global (sandboxed ~/.gitconfig)"
check "global install applies" 0 "$SCRIPT" --global
[ "$(cfg_global pack.windowMemory)" = "2g" ] &&
  pass "global pack.windowMemory = 2g" || fail "global pack.windowMemory = '$(cfg_global pack.windowMemory)'"
[ "$(cfg_global gc.auto)" = "256" ] &&
  pass "global gc.auto filled advisory 256 when absent" || fail "global gc.auto = '$(cfg_global gc.auto)', want 256"
[ "$(cfg_local gc.auto)" = "0" ] &&
  pass "repo-local gc.auto=0 still wins over the global fill" || fail "local gc.auto = '$(cfg_local gc.auto)', want 0"
check "--verify --global passes" 0 "$SCRIPT" --verify --global
check "local --verify still passes" 0 "$SCRIPT" --verify

# ---------------------------------------------------------------- uninstall local
section "--uninstall (repo-local) while global still holds"
check "uninstall exits 0 — effective bound survives" 0 "$SCRIPT" --uninstall
expect_output "still holds" "uninstall reported the global scope still supplies the bound"
[ -z "$(cfg_local pack.windowMemory)" ] &&
  pass "local pack.windowMemory removed" || fail "local pack.windowMemory survived: $(cfg_local pack.windowMemory)"
[ -z "$(cfg_local pack.threads)" ] &&
  pass "local pack.threads removed" || fail "local pack.threads survived"
[ "$(cfg_global pack.windowMemory)" = "2g" ] &&
  pass "global bound untouched by local uninstall" || fail "global bound lost"
[ -z "$(cfg_local gc.auto)" ] &&
  pass "repo-local gc.auto removed by uninstall" || fail "repo-local gc.auto survived: $(cfg_local gc.auto)"
[ "$(cfg_global gc.auto)" = "256" ] &&
  pass "global advisory gc.auto untouched by local uninstall" || fail "global advisory gc.auto lost"
check "--verify still passes (global supplies the chain)" 0 "$SCRIPT" --verify
check "second local uninstall is a clean no-op" 0 "$SCRIPT" --uninstall
expect_output "nothing to remove" "no-op uninstall reported nothing to remove"

# ---------------------------------------------------------------- uninstall global
section "--uninstall --global removes the LAST bound"
check "uninstall --global exits 1 — protection gone" 1 "$SCRIPT" --uninstall --global
expect_output "LAST effective pack-memory bound" "uninstall warned it removed the last bound"
[ -z "$(cfg_global pack.windowMemory)" ] &&
  pass "global pack.windowMemory removed" || fail "global pack.windowMemory survived"
check "--verify is UNSAFE again" 1 "$SCRIPT" --verify

# ---------------------------------------------------------------- summary
section "Summary"
echo "  passed: $PASS"
echo "  failed: $FAIL"
if [ "$FAIL" -eq 0 ]; then
  echo "✅ ALL TESTS PASSED"
  exit 0
fi
echo "❌ FAILURES PRESENT"
exit 1
