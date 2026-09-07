#!/usr/bin/env bash
# Tests for the unpushed-backlog monitor's DAILY WIRING — the call site
# scripts/auto-gc-trigger.sh gained for the bf-1ea4g prevention gap (M-1,
# docs/crash-prevention-gaps-bf-1ea4g.md §4).
#
# Scope boundary: this file tests the WIRING ONLY — that the daily 02:00 repo
# health report (the timer executes `auto-gc-trigger.sh --dry-run`, not
# check-repo-health.sh, which nothing schedules) runs the backlog check, shows
# its output, keeps the daily script's 0/1/2 exit contract even when the check
# reports CRITICAL, and degrades to a visible skip line when the helper is
# absent. The check's own thresholds, upstream resolution and exit codes are
# the carrier bead's responsibility (domchk-f239e178,
# scripts/check-unpushed-backlog.sh) and are asserted here only through the
# contract the wiring depends on, via stubs. A live integration case runs
# against the real helper when it exists and reports SKIPPED when it does not,
# so this suite stays green in a fresh clone before the carrier bead commits.
#
# Usage: ./test-unpushed-backlog-wiring.sh

set -uo pipefail
cd "$(dirname "$0")/.." || exit 1
ROOT=$PWD

PASS=0
FAIL=0
WORKROOT=""

fail() { echo "❌ FAIL: $*"; FAIL=$((FAIL + 1)); }
ok()   { echo "✅ PASS: $*"; PASS=$((PASS + 1)); }
skip() { echo "⏭️  SKIP: $*"; }

cleanup() {
  if [[ -n "$WORKROOT" && -d "$WORKROOT" ]]; then
    rm -rf "$WORKROOT"
  fi
}
trap cleanup EXIT

WORKROOT="$(mktemp -d /tmp/backlog-wiring.XXXXXX)"

# A git repo with a configured upstream and N commits ahead of it — the shape
# the monitor watches. Isolated from the real repo: push to a local bare
# remote, never to origin.
make_repo() { # $1 = path, $2 = commits ahead
  local path="$1" n="$2"
  git init -q --bare "$path/remote.git"
  git init -q -b main "$path/work" 2>/dev/null || { git init -q "$path/work" && git -C "$path/work" symbolic-ref HEAD refs/heads/main; }
  git -C "$path/work" config user.email test@local
  git -C "$path/work" config user.name wiring-test
  git -C "$path/work" commit -q --allow-empty -m seed
  git -C "$path/work" remote add origin "$path/remote.git"
  git -C "$path/work" push -q -u origin main
  local i
  for i in $(seq 1 "$n"); do
    git -C "$path/work" commit -q --allow-empty -m "backlog-$i"
  done
}

# A scripts/ dir containing ONLY the wiring (helper absent) or wiring + stub.
make_scripts_dir() { # $1 = path, $2 = stub-exit-code ("" = no helper)
  local sdir="$1" stub_rc="$2"
  mkdir -p "$sdir"
  cp "$ROOT/scripts/auto-gc-trigger.sh" "$sdir/"
  if [[ -n "$stub_rc" ]]; then
    cat > "$sdir/check-unpushed-backlog.sh" <<STUB
#!/usr/bin/env bash
# Stand-in for the carrier's helper, honouring only the contract the wiring
# relies on: prints identifiable lines, exits with the given code.
echo "STUB backlog line (rc=$stub_rc)"
exit $stub_rc
STUB
    chmod +x "$sdir/check-unpushed-backlog.sh"
  fi
}

run_daily() { # $1 = scripts dir, $2 = repo path; echoes output, sets RC
  RC=0
  OUT="$(cd "$2" && bash "$1/auto-gc-trigger.sh" --dry-run 2>&1)" || RC=$?
}

# --- 1. Helper absent: visible skip line, daily contract intact -------------
make_scripts_dir "$WORKROOT/sdir-nohelper" ""
make_repo "$WORKROOT/repo-a" 0
run_daily "$WORKROOT/sdir-nohelper" "$WORKROOT/repo-a/work"
grep -q "Unpushed Commit Backlog: skipped" <<<"$OUT" \
  && ok "helper absent -> visible skip line" \
  || fail "helper absent -> expected skip line, got: $(grep -A1 'Backlog' <<<"$OUT" | head -3)"
grep -q "GC not needed" <<<"$OUT" \
  && ok "helper absent -> daily report still completes" \
  || fail "helper absent -> daily report aborted early"
[[ "$RC" -eq 0 ]] && ok "helper absent -> exit 0" || fail "helper absent -> exit $RC, want 0"

# --- 2. Helper present, CLEAR: output shown, exit 0 --------------------------
make_scripts_dir "$WORKROOT/sdir-clear" 0
run_daily "$WORKROOT/sdir-clear" "$WORKROOT/repo-a/work"
grep -q "STUB backlog line (rc=0)" <<<"$OUT" \
  && ok "helper output surfaced in daily report" \
  || fail "helper output missing from daily report"
[[ "$RC" -eq 0 ]] && ok "helper CLEAR -> daily exit 0" || fail "helper CLEAR -> daily exit $RC"

# --- 3. Helper CRITICAL (exit 1): output shown AND daily exit stays 0 --------
# This is the load-bearing wiring contract: the backlog verdict is information
# for the log, not a gc failure — the 0/1/2 exit contract of the daily script
# must not change (02:00 timer, git-gc-check.log consumers).
make_scripts_dir "$WORKROOT/sdir-crit" 1
run_daily "$WORKROOT/sdir-crit" "$WORKROOT/repo-a/work"
grep -q "STUB backlog line (rc=1)" <<<"$OUT" \
  && ok "CRITICAL helper output still surfaced" \
  || fail "CRITICAL helper output missing"
[[ "$RC" -eq 0 ]] && ok "helper CRITICAL -> daily exit stays 0 (verdict swallowed)" \
  || fail "helper CRITICAL -> daily exit $RC, want 0 (wiring must not turn a report into a failure)"

# --- 4. Live integration against the real helper (skipped if not committed) --
REAL_HELPER="$ROOT/scripts/check-unpushed-backlog.sh"
if [[ -x "$REAL_HELPER" ]]; then
  make_scripts_dir "$WORKROOT/sdir-live" ""
  cp "$REAL_HELPER" "$WORKROOT/sdir-live/"
  # 200 commits ahead == the CRITICAL threshold the gap analysis specifies
  make_repo "$WORKROOT/repo-b" 200
  run_daily "$WORKROOT/sdir-live" "$WORKROOT/repo-b/work"
  grep -q "CRITICAL" <<<"$OUT" \
    && ok "live helper: 200-commit backlog reported CRITICAL" \
    || fail "live helper: expected CRITICAL at 200 commits, got: $(grep -i 'ahead of upstream' <<<"$OUT")"
  grep -q "ahead of upstream: 200 commit" <<<"$OUT" \
    && ok "live helper: count line present in daily report" \
    || fail "live helper: count line missing"
  [[ "$RC" -eq 0 ]] && ok "live helper: CRITICAL verdict leaves daily exit 0" \
    || fail "live helper: daily exit $RC at CRITICAL"
  # and a clean repo reads CLEAR through the same wiring
  run_daily "$WORKROOT/sdir-live" "$WORKROOT/repo-a/work"
  grep -q "✅ CLEAR" <<<"$OUT" \
    && ok "live helper: clean repo reads CLEAR in daily report" \
    || fail "live helper: expected CLEAR on clean repo, got: $(grep -i 'clear' <<<"$OUT" | head -2)"
else
  skip "live integration: scripts/check-unpushed-backlog.sh not committed yet (carrier bead domchk-f239e178)"
fi

echo ""
echo "Results: $PASS passed, $FAIL failed"
[[ "$FAIL" -eq 0 ]] || exit 1
exit 0
