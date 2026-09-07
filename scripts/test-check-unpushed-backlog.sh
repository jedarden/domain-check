#!/usr/bin/env bash
# Self-test for scripts/check-unpushed-backlog.sh (gap analysis M-1,
# docs/crash-prevention-gaps-bf-1ea4g.md).
#
# Builds throwaway git repos (never touches this repo's index) and asserts the
# rule against the crash's own historical shape:
#   - 0 commits ahead after a push            -> CLEAR, exit 0
#   - 1 and 5 commits ahead (normal WIP)      -> CLEAR, no page  (false-positive validation)
#   - 49 commits ahead                        -> CLEAR (one below the warn threshold)
#   - 50 and 60 commits ahead                 -> WARN, exit 0 (spec threshold >= 50, inclusive)
#   - 200 commits ahead                       -> CRITICAL, exit 1 (spec threshold >= 200, inclusive)
#   - 422 commits ahead (bf-1ea4g's actual
#     backlog at the fatal push, 2026-08-13)  -> CRITICAL, exit 1
#   - BACKLOG_CRITICAL_THRESHOLD override     -> respected
#   - no upstream and no origin/main          -> not measurable, exit 0 (fails open)
#   - origin/main fallback (no branch
#     upstream configured)                    -> used and reported
#   - usage errors (missing dir / non-git)    -> exit 2
#
# Usage: scripts/test-check-unpushed-backlog.sh   (no arguments)

set -uo pipefail

REPO_ROOT=$(git rev-parse --show-toplevel)
DETECTOR="$REPO_ROOT/scripts/check-unpushed-backlog.sh"

PASS=0
FAIL=0
WORK=$(mktemp -d)
trap 'rm -rf "$WORK"' EXIT

pass() { PASS=$((PASS + 1)); echo "  ✅ $1"; }
fail() { FAIL=$((FAIL + 1)); echo "  ❌ $1"; }

check() { # check <description> <expected-exit> <command...>
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

expect_marker() { # expect_marker <output> <pattern> <description>
  if grep -q "$2" <<<"$1"; then
    pass "$3"
  else
    fail "$3 — output lacked /$2/"
    echo "----- output -----"
    echo "$1"
    echo "------------------"
  fi
}

absent_marker() { # absent_marker <output> <pattern> <description>
  if grep -q "$2" <<<"$1"; then
    fail "$3 — output unexpectedly contained /$2/"
    grep "$2" <<<"$1" | sed 's/^/      /'
  else
    pass "$3"
  fi
}

# make_repo <dir> — bare origin + work clone on main with upstream configured,
# one commit pushed (so upstream exists and is reachable).
make_repo() {
  local dir=$1
  git init -q --bare "$dir/origin.git"
  git init -q "$dir/work"
  git -C "$dir/work" config user.email test@example.com
  git -C "$dir/work" config user.name backlog-test
  git -C "$dir/work" config commit.gpgsign false
  : >"$dir/work/file.txt"
  git -C "$dir/work" add file.txt
  git -C "$dir/work" commit -q -m initial
  git -C "$dir/work" branch -m main
  git -C "$dir/work" remote add origin "$dir/origin.git"
  git -C "$dir/work" push -q -u origin main
}

# add_commits <dir> <n> — accumulate n local commits WITHOUT pushing (the S3
# self-amplifying-retry shape: each attempt commits, none pushes).
add_commits() {
  local dir=$1 n=$2 i
  for ((i = 1; i <= n; i++)); do
    git -C "$dir" commit -q --allow-empty -m "unpushed-attempt-snapshot-$i"
  done
}

echo "check-unpushed-backlog.sh self-test"
echo

echo "— script hygiene"
check "detector passes bash -n" 0 bash -n "$DETECTOR"

echo
echo "— baseline: pushed repo is CLEAR (and not measurable cases)"
R="$WORK/base"; make_repo "$R"
check "0 commits ahead -> CLEAR, exit 0" 0 "$DETECTOR" "$R/work"
expect_marker "$LAST_OUT" "✅ CLEAR" "  clear level reported"
expect_marker "$LAST_OUT" "ahead of upstream: 0 commit" "  count line reports 0"

R="$WORK/noupstream"
git init -q "$R"
git -C "$R" config user.email test@example.com
git -C "$R" config user.name backlog-test
git -C "$R" commit -q --allow-empty -m initial
check "no upstream, no origin/main -> fails open, exit 0" 0 "$DETECTOR" "$R"
expect_marker "$LAST_OUT" "not measurable" "  skip reason is reported"

R="$WORK/fallback"; make_repo "$R"
git -C "$R/work" branch --unset-upstream
check "origin/main fallback -> exit 0 at 0 ahead" 0 "$DETECTOR" "$R/work"
expect_marker "$LAST_OUT" "fallback" "  fallback source is named"
expect_marker "$LAST_OUT" "origin/main" "  fallback ref is origin/main"

echo
echo "— false-positive validation: ordinary work-in-progress must not page"
R="$WORK/wip1"; make_repo "$R"; add_commits "$R/work" 1
check "1 unpushed commit -> CLEAR, exit 0" 0 "$DETECTOR" "$R/work"
expect_marker "$LAST_OUT" "✅ CLEAR" "  1 commit stays clear"
absent_marker "$LAST_OUT" "WARN|CRITICAL" "  1 commit raises no level"
R="$WORK/wip5"; make_repo "$R"; add_commits "$R/work" 5
check "5 unpushed commits -> CLEAR, exit 0" 0 "$DETECTOR" "$R/work"
absent_marker "$LAST_OUT" "WARN|CRITICAL" "  5 commits raise no level"

echo
echo "— spec thresholds (warn >= 50, critical >= 200), boundaries inclusive"
R="$WORK/below"; make_repo "$R"; add_commits "$R/work" 49
check "49 commits ahead (warn - 1) -> CLEAR, exit 0" 0 "$DETECTOR" "$R/work"
absent_marker "$LAST_OUT" "WARN|CRITICAL" "  49 commits raise no level"

R="$WORK/warn"; make_repo "$R"; add_commits "$R/work" 50
check "50 commits ahead (exactly warn) -> WARN, exit 0" 0 "$DETECTOR" "$R/work"
expect_marker "$LAST_OUT" "⚠️  WARN: 50 unpushed commits" "  warn boundary is inclusive (>= 50)"

R="$WORK/warn60"; make_repo "$R"; add_commits "$R/work" 60
check "60 commits ahead -> WARN, exit 0" 0 "$DETECTOR" "$R/work"
absent_marker "$LAST_OUT" "CRITICAL" "  60 commits is not critical"

R="$WORK/crit"; make_repo "$R"; add_commits "$R/work" 200
check "200 commits ahead (exactly critical) -> CRITICAL, exit 1" 1 "$DETECTOR" "$R/work"
expect_marker "$LAST_OUT" "🚨 CRITICAL: 200 unpushed commits" "  critical boundary is inclusive (>= 200)"

R="$WORK/historical"; make_repo "$R"; add_commits "$R/work" 422   # bf-1ea4g's actual figure
check "422 commits ahead (bf-1ea4g backlog) -> CRITICAL, exit 1" 1 "$DETECTOR" "$R/work"
expect_marker "$LAST_OUT" "🚨 CRITICAL: 422 unpushed commits" "  critical level names the historical count"
expect_marker "$LAST_OUT" "bf-1ea4g" "  critical output cites the crash shape"

echo
echo "— threshold overrides"
R="$WORK/override"; make_repo "$R"; add_commits "$R/work" 60
check "override BACKLOG_CRITICAL_THRESHOLD=10 makes 60 critical" 1 \
  env BACKLOG_CRITICAL_THRESHOLD=10 "$DETECTOR" "$R/work"
R="$WORK/override2"; make_repo "$R"; add_commits "$R/work" 60
check "override BACKLOG_WARN_THRESHOLD=100 demotes 60 to clear" 0 \
  env BACKLOG_WARN_THRESHOLD=100 "$DETECTOR" "$R/work"

echo
echo "— usage errors"
check "missing directory -> exit 2" 2 "$DETECTOR" "$WORK/does-not-exist"
check "non-git directory -> exit 2" 2 "$DETECTOR" "$WORK"

echo
echo "— live repo (informational only — co-tenant commits make this nondeterministic)"
"$DETECTOR" "$REPO_ROOT" | sed 's/^/  | /'

echo
echo "Result: $PASS passed, $FAIL failed"
if [ "$FAIL" -ne 0 ]; then
  exit 1
fi
echo "✅ all assertions passed"
