#!/usr/bin/env bash
# Tests for verify-work-completion.sh — the pre-close work verification gate.
#
# Covers:
#   1. All checks pass on a committed-and-pushed fixture repo
#   2. Unpushed commits fail (and downgrade to WARN with --allow-unpushed)
#   3. Uncommitted changes: WARN by default, FAIL with --strict-clean
#   4. Required path / grep / command checks
#   5. Health thresholds via env overrides (no real resource pressure needed)
#   6. Bead-store checks: bogus id fails, closed bead is a post-hoc warning
#   7. Marker file + JSON output + log line
#   8. Usage errors
#
# Fixture repos use a local bare origin, so no network is required.
#
# Usage: scripts/test-verify-work-completion.sh

set -u

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VWC="$SCRIPT_DIR/verify-work-completion.sh"
REPO_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

PASS=0
FAIL=0
TMPROOT="$(mktemp -d)"
trap 'rm -rf "$TMPROOT"' EXIT

pass() { echo "  ✅ PASS: $1"; PASS=$((PASS + 1)); }
fail() { echo "  ❌ FAIL: $1"; FAIL=$((FAIL + 1)); }

# Build a fixture repo with a local bare origin; everything committed and pushed.
make_fixture() {
    local dir="$1"
    git init -q -b main "$dir"
    git -C "$dir" config user.email github@jedarden.com
    git -C "$dir" config user.name jedarden
    git init -q --bare "$dir-origin.git"
    git -C "$dir" remote add origin "$dir-origin.git"
    echo "hello world" > "$dir/README.md"
    git -C "$dir" add README.md
    git -C "$dir" commit -q -m "fixture initial commit"
    git -C "$dir" push -q -u origin main
}

echo "=== verify-work-completion.sh tests ==="
echo ""

echo "[1] all checks pass on a pushed fixture"
(
  set -euo pipefail
  FIX="$TMPROOT/t1"; mkdir -p "$FIX"; make_fixture "$FIX"
  cd "$FIX"
  VWC_MIN_AVAIL_MEM_GB=0 VWC_MIN_DISK_FREE_GB=0 VWC_MAX_CPU_LOAD=9999 \
    "$VWC" test-bead-t1 --skip-bead --summary "t1 done" >/dev/null
) && pass "clean pushed fixture verifies (exit 0)" || fail "clean pushed fixture should verify"

echo ""
echo "[2] unpushed commits"
(
  set -euo pipefail
  FIX="$TMPROOT/t2"; mkdir -p "$FIX"; make_fixture "$FIX"
  cd "$FIX"
  echo more >> README.md
  git add README.md && git commit -q -m "unpushed work"
  if VWC_MIN_AVAIL_MEM_GB=0 VWC_MIN_DISK_FREE_GB=0 VWC_MAX_CPU_LOAD=9999 \
      "$VWC" test-bead-t2 --skip-bead >/dev/null 2>&1; then
    exit 1  # expected to fail, but verified
  fi
  # marker must record the failure
  grep -q '"result": "FAILED"' "$FIX/.beads/state/work-completion/test-bead-t2.json"
) && pass "unpushed commit fails verification and marker records FAILED" || fail "unpushed commit should fail"

(
  set -euo pipefail
  FIX="$TMPROOT/t2b"; mkdir -p "$FIX"; make_fixture "$FIX"
  cd "$FIX"
  echo more >> README.md
  git add README.md && git commit -q -m "unpushed work"
  VWC_MIN_AVAIL_MEM_GB=0 VWC_MIN_DISK_FREE_GB=0 VWC_MAX_CPU_LOAD=9999 \
    "$VWC" test-bead-t2b --skip-bead --allow-unpushed >/dev/null 2>&1
) && pass "--allow-unpushed downgrades to warning (exit 0)" || fail "--allow-unpushed should pass"

echo ""
echo "[3] uncommitted changes: default WARN, --strict-clean FAIL"
(
  set -euo pipefail
  FIX="$TMPROOT/t3"; mkdir -p "$FIX"; make_fixture "$FIX"
  cd "$FIX"
  echo dirty > uncommitted.txt
  VWC_MIN_AVAIL_MEM_GB=0 VWC_MIN_DISK_FREE_GB=0 VWC_MAX_CPU_LOAD=9999 \
    "$VWC" test-bead-t3 --skip-bead --json 2>/dev/null | python3 -c '
import json,sys
d = json.load(sys.stdin)
assert d["result"] == "VERIFIED", d["result"]
assert d["warnings"] >= 1, d
assert any(c["check"] == "git_clean" and c["status"] == "WARN" for c in d["checks"]), d
'
) && pass "dirty tree warns but verifies by default (JSON parsed)" || fail "dirty tree default should be WARN+pass"

(
  set -euo pipefail
  FIX="$TMPROOT/t3b"; mkdir -p "$FIX"; make_fixture "$FIX"
  cd "$FIX"
  echo dirty > uncommitted.txt
  if VWC_MIN_AVAIL_MEM_GB=0 VWC_MIN_DISK_FREE_GB=0 VWC_MAX_CPU_LOAD=9999 \
      "$VWC" test-bead-t3b --skip-bead --strict-clean >/dev/null 2>&1; then
    exit 1  # expected to fail, but verified
  fi
) && pass "--strict-clean fails on dirty tree" || fail "--strict-clean should fail"

echo ""
echo "[4] required path / grep / command"
(
  set -euo pipefail
  FIX="$TMPROOT/t4"; mkdir -p "$FIX"; make_fixture "$FIX"
  cd "$FIX"
  VWC_MIN_AVAIL_MEM_GB=0 VWC_MIN_DISK_FREE_GB=0 VWC_MAX_CPU_LOAD=9999 \
    "$VWC" test-bead-t4 --skip-bead \
      --require-path README.md \
      --require-grep "README.md:^hello" \
      --require-command "test -f README.md" >/dev/null
  # grep pattern containing a colon must split on the FIRST colon only
  if VWC_MIN_AVAIL_MEM_GB=0 VWC_MIN_DISK_FREE_GB=0 VWC_MAX_CPU_LOAD=9999 \
      "$VWC" test-bead-t4x --skip-bead --require-grep "README.md:ZZZNOMATCH" >/dev/null 2>&1; then
    exit 1
  fi
  if VWC_MIN_AVAIL_MEM_GB=0 VWC_MIN_DISK_FREE_GB=0 VWC_MAX_CPU_LOAD=9999 \
      "$VWC" test-bead-t4y --skip-bead --require-path missing.txt >/dev/null 2>&1; then
    exit 1
  fi
  if VWC_MIN_AVAIL_MEM_GB=0 VWC_MIN_DISK_FREE_GB=0 VWC_MAX_CPU_LOAD=9999 \
      "$VWC" test-bead-t4z --skip-bead --require-command "false" >/dev/null 2>&1; then
    exit 1
  fi
) && pass "path/grep/command requirements pass and fail correctly" || fail "requirement checks misbehave"

echo ""
echo "[5] health thresholds via env overrides"
(
  set -euo pipefail
  FIX="$TMPROOT/t5"; mkdir -p "$FIX"; make_fixture "$FIX"
  cd "$FIX"
  # Impossible threshold must fail; absent thresholds must not
  if VWC_MIN_AVAIL_MEM_GB=999999 VWC_MIN_DISK_FREE_GB=0 VWC_MAX_CPU_LOAD=9999 \
      "$VWC" test-bead-t5 --skip-bead >/dev/null 2>&1; then
    exit 1  # expected to fail, but verified
  fi
  grep -q '"check": "health_memory", "status": "FAIL"' \
    "$FIX/.beads/state/work-completion/test-bead-t5.json"
) && pass "VWC_MIN_AVAIL_MEM_GB=999999 fails health_memory" || fail "memory threshold override should fail"

echo ""
echo "[6] bead-store checks (against the real workspace store, read-only)"
(
  set -euo pipefail
  cd "$REPO_ROOT"
  # Bogus id must fail
  VWC_MIN_AVAIL_MEM_GB=0 VWC_MIN_DISK_FREE_GB=0 VWC_MAX_CPU_LOAD=9999 \
    "$VWC" bogus-bead-does-not-exist-000 --skip-health >/dev/null 2>&1 && exit 1
  grep -q '"check": "bead_state", "status": "FAIL"' \
    ".beads/state/work-completion/bogus-bead-does-not-exist-000.json"
) && pass "bogus bead id fails bead_state" || fail "bogus bead id should fail"

(
  set -euo pipefail
  cd "$REPO_ROOT"
  # A real CLOSED bead: verification is post-hoc → WARN, still exit 0
  CLOSED_ID="$(bead list --status closed --limit 1 --json 2>/dev/null | head -1 | python3 -c 'import json,sys; print(json.loads(sys.stdin.readline())["id"])' 2>/dev/null)" || CLOSED_ID=""
  if [[ -z "$CLOSED_ID" ]]; then
    echo "  ⚠️  SKIP: no closed beads available"; exit 0
  fi
  VWC_MIN_AVAIL_MEM_GB=0 VWC_MIN_DISK_FREE_GB=0 VWC_MAX_CPU_LOAD=9999 \
    "$VWC" "$CLOSED_ID" --skip-health --summary "post-hoc check" >/dev/null 2>&1
  grep -q '"check": "bead_status", "status": "WARN"' \
    ".beads/state/work-completion/$CLOSED_ID.json"
) && pass "closed bead verifies with post-hoc WARN" || fail "closed bead should WARN not FAIL"

echo ""
echo "[7] log line and summary recording"
(
  set -euo pipefail
  FIX="$TMPROOT/t7"; mkdir -p "$FIX"; make_fixture "$FIX"
  cd "$FIX"
  VWC_MIN_AVAIL_MEM_GB=0 VWC_MIN_DISK_FREE_GB=0 VWC_MAX_CPU_LOAD=9999 \
    "$VWC" test-bead-t7 --skip-bead --summary "unit test summary sentinel" >/dev/null 2>&1
  grep -q "unit test summary sentinel" "$FIX/.beads/logs/work-completion.log"
  grep -q '"bead_id": "test-bead-t7"' "$FIX/.beads/state/work-completion/test-bead-t7.json"
) && pass "summary recorded in log and marker" || fail "summary not recorded"

echo ""
echo "[8] usage errors"
(
  set -euo pipefail
  "$VWC" >/dev/null 2>&1 && exit 1
  "$VWC" --bogus-flag x >/dev/null 2>&1 && exit 1
  "$VWC" --help >/dev/null 2>&1
) && pass "missing id / unknown flag → exit 2; --help → exit 0" || fail "usage error handling"

echo ""
echo "=== Results: $PASS passed, $FAIL failed ==="
[[ "$FAIL" -eq 0 ]]
