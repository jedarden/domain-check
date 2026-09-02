#!/usr/bin/env bash
# Tests for scripts/setup-git-gc-config.sh — the persistent pack-memory bound
# that defends the bare `git gc --aggressive` path against memcg OOM
# (root cause of bf-173o7e / bf-4x12ec exit -1 storms; see
# docs/maintenance/repository-maintenance-guide.md).
#
# Unit tests check the config lands and verifies. The integration test
# reproduces the crash COMMAND at reduced scale (8x64MiB incompressible blobs
# instead of 17GiB of loose objects) and asserts the bare aggressive gc the
# agent ran 129 times now completes inside a 768MiB cgroup — 1/16th of the
# 12GiB needle dispatch scope.
#
# Usage: ./test-gc-memory-bounds.sh            # all tests
#        ./test-gc-memory-bounds.sh --unit     # skip the slow integration test
#        DOMCHECK_KEEP_GCMB=1 ...              # keep temp repos for inspection

set -uo pipefail
cd "$(dirname "$0")/.." || exit 1
ROOT=$PWD
SETUP="$ROOT/scripts/setup-git-gc-config.sh"

PASS=0
FAIL=0
WORKROOT=""

fail() { echo "❌ FAIL: $*"; FAIL=$((FAIL + 1)); }
ok()   { echo "✅ PASS: $*"; PASS=$((PASS + 1)); }

cleanup() {
  if [[ -n "$WORKROOT" && -d "$WORKROOT" && "${DOMCHECK_KEEP_GCMB:-0}" != "1" && "$FAIL" -eq 0 ]]; then
    rm -rf "$WORKROOT"
  elif [[ -d "$WORKROOT" ]]; then
    echo "   (artifacts kept: $WORKROOT)"
  fi
}
trap cleanup EXIT

# GNU time is not at /usr/bin/time on NixOS; resolve it once, empty means absent
TIME_BIN=""
for c in /usr/bin/time /bin/time /run/current-system/sw/bin/time; do
  [[ -x "$c" ]] && { TIME_BIN=$c; break; }
done

# k/m/g or bare -> bytes
to_bytes() {
  local v="${1,,}" n="${v:0:-1}" unit="${v: -1}"
  case "$unit" in
    k) echo $((n * 1024)) ;;
    m) echo $((n * 1024 * 1024)) ;;
    g) echo $((n * 1024 * 1024 * 1024)) ;;
    *) echo "$v" ;;
  esac
}

# setup_in <repo> [args...] — the setup script binds to its CWD, so run it there
setup_in() {
  local repo=$1; shift
  ( cd "$repo" && "$SETUP" "$@" )
}

verify_in() {  # verify_in <repo> -> 0 iff --verify passes there
  local repo=$1
  ( cd "$repo" && "$SETUP" --verify ) >/dev/null 2>&1
}

# Same, but hide this box's global/system gitconfig. The negative tests below
# assert rejection of MISSING bounds; without isolation the box-wide global
# bound (setup-git-gc-config.sh --global) would satisfy the effective-bound
# lookup and mask the absence.
verify_isolated_in() {
  local repo=$1
  ( cd "$repo" && GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 "$SETUP" --verify ) >/dev/null 2>&1
}

get_rss_kb() {  # "$1" = /usr/bin/time -v stderr log
  grep -oE 'Maximum resident set size \(kbytes\): [0-9]+' "$1" | grep -oE '[0-9]+$'
}

newrepo() {
  local d
  d=$(mktemp -d "$WORKROOT/repo.XXXXXX") || return 1
  git -C "$d" init -q
  git -C "$d" config user.email test@local
  git -C "$d" config user.name test
  echo "$d"
}

mkdir -p "${TMPDIR:-/home/coding/scratch}" 2>/dev/null
WORKROOT=$(mktemp -d "${TMPDIR:-/home/coding/scratch}/gcmb.XXXXXX") || exit 1

echo "=== unit: bounds land in a fresh repo ==="
r=$(newrepo) || exit 1
setup_in "$r" >/dev/null 2>&1 || fail "setup exited nonzero in fresh repo"
for kv in "pack.windowMemory 2g" "pack.deltaCacheSize 1g" "pack.threads 1"; do
  k=${kv% *}; want=${kv#* }
  got=$(git -C "$r" config "$k")
  [[ "$got" == "$want" ]] && ok "$k = $got in fresh repo" || fail "$k = '$got', want '$want'"
done
verify_in "$r" && ok "--verify passes in a bounded repo" || fail "--verify rejects a bounded repo"

echo "=== unit: safety core overrides, gc policy does not clobber ==="
r2=$(newrepo) || exit 1
git -C "$r2" config gc.auto 7
git -C "$r2" config pack.windowMemory 128m
setup_in "$r2" >/dev/null 2>&1 || fail "setup exited nonzero in pre-tuned repo"
[[ "$(git -C "$r2" config gc.auto)" == "7" ]] \
  && ok "existing gc.auto=7 preserved" || fail "gc.auto clobbered: $(git -C "$r2" config gc.auto)"
[[ "$(git -C "$r2" config pack.windowMemory)" == "2g" ]] \
  && ok "safety core overrode stale pack.windowMemory=128m" || fail "safety core did not override"

echo "=== unit: verify rejects unbounded and thread-multiplied states ==="
r3=$(newrepo) || exit 1   # no setup run at all
verify_isolated_in "$r3" && fail "--verify accepted a repo with no bounds" || ok "--verify rejects an unbounded repo"
r4=$(newrepo) || exit 1
git -C "$r4" config pack.windowMemory 4g
git -C "$r4" config pack.deltaCacheSize 1g
# pack.threads deliberately unset: git auto-sizes to all cores, multiplying the window
verify_isolated_in "$r4" && fail "--verify accepted unset pack.threads" || ok "--verify rejects unset pack.threads (per-thread multiplication)"

echo "=== unit: verify sees the box-wide global bound when the repo has none ==="
r6=$(newrepo) || exit 1
gtmp="$WORKROOT/global-gitconfig"
git config --file "$gtmp" pack.windowMemory 2g
git config --file "$gtmp" pack.deltaCacheSize 1g
git config --file "$gtmp" pack.threads 1
( cd "$r6" && GIT_CONFIG_GLOBAL="$gtmp" GIT_CONFIG_NOSYSTEM=1 "$SETUP" --verify ) >/dev/null 2>&1 \
  && ok "--verify passes via the global bound alone (effective chain)" \
  || fail "--verify rejects a repo protected only by the global bound"

[[ "${1:-}" == "--unit" ]] && { echo; echo "=== unit only: $PASS passed, $FAIL failed ==="; exit $(( FAIL > 0 )); }

echo "=== integration: the bf-173o7e crash command under a 768MiB cgroup ==="
r5=$(newrepo) || exit 1
PACK_WINDOW_MEMORY=128m PACK_DELTA_CACHE_SIZE=64m PACK_THREADS=1 setup_in "$r5" >/dev/null 2>&1 \
  || fail "setup failed in integration repo"
verify_in "$r5" || fail "integration repo failed verify"
for i in 1 2 3 4 5 6 7 8; do
  dd if=/dev/urandom of="$r5/blob-$i.bin" bs=1M count=64 status=none || fail "dd blob $i"
  git -C "$r5" add "blob-$i.bin"
  git -C "$r5" commit -qm "blob $i"
done
loose=$(git -C "$r5" count-objects -v | awk '/^count:/{print $2}')
echo "   prepared $loose loose objects of incompressible data (8 x 64MiB)"
echo "   running the exact crash command: git gc --aggressive --prune=now"

TIME_WRAP=()
[[ -n "$TIME_BIN" ]] && TIME_WRAP=("$TIME_BIN" -v)

if systemd-run --user --quiet --scope --unit="gcmb-probe-$$" -p MemoryMax=768M true 2>/dev/null; then
  ( cd "$r5" && timeout 300 systemd-run --user --quiet --scope \
      --unit="gcmb-bare-aggressive-$$" -p MemoryMax=768M \
      "${TIME_WRAP[@]}" git gc --aggressive --prune=now ) 2> "$WORKROOT/gc-stderr.log"
  rc=$?
  [[ $rc -eq 0 ]] && ok "bare 'git gc --aggressive --prune=now' exited 0 under MemoryMax=768M" \
                  || fail "crash command exited $rc under MemoryMax=768M (see $WORKROOT/gc-stderr.log)"
else
  echo "   systemd-run --user unavailable; asserting the config bound via RSS only"
  ( cd "$r5" && timeout 300 "${TIME_WRAP[@]}" git gc --aggressive --prune=now ) 2> "$WORKROOT/gc-stderr.log"
  rc=$?
  [[ $rc -eq 0 ]] && ok "bare aggressive gc exited 0 (no cgroup available)" || fail "gc exited $rc"
fi
if [[ -n "$TIME_BIN" ]]; then
  rss=$(get_rss_kb "$WORKROOT/gc-stderr.log" || echo 0)
  if (( rss > 0 && rss < 700 * 1024 )); then
    ok "pack-objects peak RSS ${rss}KB < 700MiB cap (the crash run exceeded 12GiB)"
  else
    fail "peak RSS ${rss}KB outside expected range"
  fi
else
  echo "   (GNU time not found; RSS assertion skipped, cgroup/exit assertions still ran)"
fi
loose_after=$(git -C "$r5" count-objects -v | awk '/^count:/{print $2}')
[[ "$loose_after" == "0" ]] && ok "repo fully packed after gc" || fail "loose objects remain: $loose_after"

echo
echo "=== $PASS passed, $FAIL failed ==="
exit $(( FAIL > 0 ))
