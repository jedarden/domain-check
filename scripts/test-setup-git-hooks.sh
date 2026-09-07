#!/usr/bin/env bash
# Self-test for scripts/setup-git-hooks.sh and the hook it installs.
#
# Exercises the installer and the installed hook end-to-end inside a throwaway
# git repo (never touches this repo's index), asserting that the bf-4yjq bloat
# classes are actually blocked:
#   - staged file >10MB                       -> commit blocked
#   - total staged payload >50MB              -> commit blocked
#   - forced-added .beads/ state              -> commit blocked
#   - normal / deletion-only / rename commits -> allowed
#   - --check detects a missing, drifted, and non-executable hook
#   - --uninstall removes the hook
#
# Usage: scripts/test-setup-git-hooks.sh   (no arguments)

set -uo pipefail

REPO_ROOT=$(git rev-parse --show-toplevel)
INSTALLER="$REPO_ROOT/scripts/setup-git-hooks.sh"
SOURCE="$REPO_ROOT/scripts/pre-commit-repo-size-hook"

PASS=0
FAIL=0
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

pass() { PASS=$((PASS + 1)); echo "  ✅ $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  ❌ $1"; }

check() { # check <description> <expected: 0|1> <command...>
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

expect_block() { # expect_block <description> <output-of-failed-commit>
  if grep -q "COMMIT BLOCKED" "$1"; then
    pass "$2"
  else
    fail "$2 — commit failed but without a COMMIT BLOCKED report"
    sed 's/^/      /' "$1"
  fi
}

commit_and_capture() { # commit_and_capture <message> -> writes $WORK/out.txt
  local msg=$1
  if git commit -q -m "$msg" >"$WORK/out.txt" 2>&1; then
    return 0
  fi
  return 1
}

section() { echo ""; echo "$1"; }

# ---------------------------------------------------------------- setup
section "Setting up throwaway repo at $WORK"
git init -q -b main "$WORK/clone" 2>/dev/null || git init -q "$WORK/clone"
cd "$WORK/clone" || exit 1
git config --local user.email github@jedarden.com
git config --local user.name jedarden
git config --local commit.gpgsign false

# ---------------------------------------------------------------- installer
section "Installer"
check "installer runs in a fresh clone" 0 "$INSTALLER"
[ -x .git/hooks/pre-commit ] && pass "installed hook is executable" ||
  fail "installed hook is not executable"
cmp -s "$SOURCE" .git/hooks/pre-commit &&
  pass "installed hook is byte-identical to tracked source" ||
  fail "installed hook differs from tracked source"
check "installer is idempotent" 0 "$INSTALLER"
check "--check passes on a fresh install" 0 "$INSTALLER" --check

# ---------------------------------------------------------------- allowed commits
section "Commits that must pass"
echo "hello" > small.txt
git add small.txt
check "baseline commit with a small file" 0 commit_and_capture "baseline"
grep -q "Pre-commit check passed" "$WORK/out.txt" &&
  pass "hook reported success on baseline commit" ||
  fail "hook did not report success on baseline commit"

git mv small.txt small-renamed.txt
check "rename-only commit" 0 commit_and_capture "rename small file"
[ -f small-renamed.txt ] && [ ! -f small.txt ] &&
  pass "rename applied cleanly (hook scanned destination path without error)" ||
  fail "rename did not apply cleanly"

echo "gone" > doomed.txt
git add doomed.txt
git commit -q -m "add doomed"
git rm -q doomed.txt
check "deletion-only commit" 0 commit_and_capture "remove doomed"

# ---------------------------------------------------------------- blocked commits
section "Commits that must be blocked"
truncate -s 11M big.bin
git add big.bin
if commit_and_capture "one 11MB file"; then
  fail "11MB file commit was allowed"
else
  expect_block "$WORK/out.txt" "11MB file blocked"
fi
git restore --staged big.bin

for i in 1 2 3 4 5 6; do truncate -s 9M "chunk$i.bin"; git add "chunk$i.bin"; done
if commit_and_capture "six 9MB files (54MB total)"; then
  fail "54MB total commit was allowed"
else
  expect_block "$WORK/out.txt" "54MB total (no single file over 10MB) blocked"
fi
git restore --staged chunk*.bin

mkdir -p .beads
echo '{"snapshot": true}' > .beads/state.jsonl
git add -f .beads/state.jsonl
if commit_and_capture "forced-added bead state"; then
  fail ".beads/ forced-add commit was allowed"
else
  expect_block "$WORK/out.txt" ".beads/ forced-add blocked"
fi
git restore --staged .beads/state.jsonl

# The guard covers every shape under .beads/, not just the .jsonl snapshots
# bf-4yjq leaked — beads.db, logs, state/ are all gitignored force-adds too.
mkdir -p .beads/logs
echo 'x' > .beads/beads.db
echo 'log line' > .beads/logs/crash-monitor.log
git add -f .beads/beads.db .beads/logs/crash-monitor.log
if commit_and_capture "forced-added non-jsonl bead state"; then
  fail ".beads/ non-jsonl forced-add commit was allowed"
else
  expect_block "$WORK/out.txt" ".beads/ non-jsonl forced-add (beads.db, logs/) blocked"
fi
git restore --staged .beads/beads.db .beads/logs/crash-monitor.log

# ---------------------------------------------------------------- drift handling
section "--check drift detection and repair"
echo "# drift" >>.git/hooks/pre-commit
check "--check fails on drifted hook" 1 "$INSTALLER" --check
check "installer overwrites drifted hook" 0 "$INSTALLER"
check "--check passes after repair" 0 "$INSTALLER" --check
chmod -x .git/hooks/pre-commit
check "--check fails on non-executable hook" 1 "$INSTALLER" --check
check "installer restores executable hook" 0 "$INSTALLER"

section "--uninstall"
check "uninstall removes the hook" 0 "$INSTALLER" --uninstall
[ -f .git/hooks/pre-commit ] && fail "hook still present after uninstall" ||
  pass "hook gone after uninstall"
check "--check fails once hook is gone" 1 "$INSTALLER" --check

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
