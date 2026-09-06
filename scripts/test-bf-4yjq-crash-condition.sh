#!/usr/bin/env bash
# Re-creates the bf-4yjq crash condition at reduced scale and asserts that the
# deployed mitigations neutralize it. Specification:
# docs/crash-investigations/bf-4yjq-crash-workload-test-spec-domchk-b90505ad-2026-09-06.md
#
# The crash (2026-08-12, 50 exit-code -1 deaths in 2h37m): substantive git
# operations against a repository holding 17.2 GiB of loose objects inside a
# memory-bounded agent scope pulled a working set larger than the bound, so the
# kernel memcg OOM killer SIGKILLed the process — the needle-visible "exit -1".
#
# The full 17.2 GiB condition must NOT be re-created on the live repo (it is
# what the repo-health guardrails exist to prevent). This harness preserves the
# scaling relation that makes the mechanism deterministic — pack-objects' peak
# RSS scales with the loose-object set (measured >12 GiB RSS on 17.2 GiB loose
# in the bf-173o7e storm) — and tests at 1/17th scale: 16 x 64 MiB of
# incompressible blobs (1 GiB loose) inside a 512 MiB cgroup.
#
# Assertions:
#   A (crash re-created)      bare `git gc`, 1 GiB loose, MemoryMax=512M -> SIGKILL
#   B (pack bounds mitigate)  same repo + deployed pack.* bounds         -> exit 0
#   C (packed store mitigates) repo packed, ordinary agent ops @ 512M    -> exit 0
#
# Usage: ./test-bf-4yjq-crash-condition.sh
#        DOMCHECK_KEEP_BF4YJQ=1 ...   # keep the scratch repos for inspection
#        BF4YJQ_LOOSE_MB=1024 BF4YJQ_MEMORY_MAX=512M ...  # override scaling

set -uo pipefail
cd "$(dirname "$0")/.." || exit 1

PASS=0
FAIL=0
WORKROOT=""

LOOSE_MB="${BF4YJQ_LOOSE_MB:-1024}"        # total loose-object bytes to build
BLOB_COUNT="${BF4YJQ_BLOBS:-16}"           # blobs -> (LOOSE_MB / BLOB_COUNT) each
MEMORY_MAX="${BF4YJQ_MEMORY_MAX:-512M}"    # the stand-in agent-scope bound
TIMEOUT_SECS="${BF4YJQ_TIMEOUT:-600}"

# Deployed bare-gc bounds, as set by scripts/setup-git-gc-config.sh
PACK_WINDOW_MEMORY="${BF4YJQ_WINDOW_MEMORY:-128m}"
PACK_DELTA_CACHE_SIZE="${BF4YJQ_DELTA_CACHE:-64m}"
PACK_THREADS="${BF4YJQ_THREADS:-1}"

ok()   { echo "✅ PASS: $*"; PASS=$((PASS + 1)); }
fail() { echo "❌ FAIL: $*"; FAIL=$((FAIL + 1)); }

cleanup() {
  if [[ -n "$WORKROOT" && -d "$WORKROOT" && "${DOMCHECK_KEEP_BF4YJQ:-0}" != "1" ]]; then
    rm -rf "$WORKROOT"
  elif [[ -d "$WORKROOT" ]]; then
    echo "   (artifacts kept: $WORKROOT)"
  fi
}
trap cleanup EXIT

# systemd-run --user is the bound of record; without it the kill assertion
# cannot run and the harness must say so rather than pass vacuously
have_cgroup=false
if systemd-run --user --quiet --scope --unit="bf4yjq-probe-$$" -p MemoryMax=64M true 2>/dev/null; then
  have_cgroup=true
fi

bounded() {  # bounded <unit-name> <workdir> <command...>  -> runs under MemoryMax
  local unit=$1 dir=$2
  shift 2
  ( cd "$dir" && timeout "$TIMEOUT_SECS" systemd-run --user --quiet --scope \
      --unit="$unit-$$" -p MemoryMax="$MEMORY_MAX" -p MemorySwapMax=0 \
      "$@" ) 2>&1
}

exit_sig() {  # decode systemd-run scope exit status to "0" or "SIG<NAME/n>"
  local rc=$1 n
  if (( rc >= 128 )); then
    n=$((rc - 128))
    case "$n" in
      9) echo SIGKILL ;;
      15) echo SIGTERM ;;
      *) echo "SIG$n" ;;
    esac
  else
    echo "$rc"
  fi
}

WORKROOT=$(mktemp -d /tmp/bf4yjq-spec.XXXXXX) || exit 1
REPO="$WORKROOT/bloated"
git init -q "$REPO" || exit 1
git -C "$REPO" config user.email github@jedarden.com
git -C "$REPO" config user.name jedarden

BLOB_MB=$(( LOOSE_MB / BLOB_COUNT ))
echo "=== building the crash condition: ${LOOSE_MB}MiB of incompressible loose objects ==="
echo "   repo: $REPO   bound: MemoryMax=$MEMORY_MAX"
for i in $(seq 1 "$BLOB_COUNT"); do
  dd if=/dev/urandom of="$REPO/blob-$i.bin" bs=1M count="$BLOB_MB" status=none
  git -C "$REPO" add "blob-$i.bin"
  git -C "$REPO" commit -qm "incompressible blob $i"
done
loose_kib=$(git -C "$REPO" count-objects -v | awk '/^size:/{print $2}')
echo "   prepared: $(git -C "$REPO" count-objects -v | awk '/^count:/{print $2}') loose objects, $((loose_kib / 1024)) MiB"

if [[ "$have_cgroup" != true ]]; then
  fail "systemd-run --user unavailable — cannot assert the memcg OOM kill (A) or the bounded mitigations (B, C)"
  echo; echo "=== $PASS passed, $FAIL failed ==="
  exit 1
fi

echo
echo "=== A: the crash re-created — bare 'git gc' over $((LOOSE_MB / 1024))GiB loose inside ${MEMORY_MAX} ==="
OUT=$(bounded bf4yjq-crash "$REPO" git gc)
rc=$?
sig=$(exit_sig "$rc")
loose_after_a=$(git -C "$REPO" count-objects -v | awk '/^count:/{print $2}')
echo "$OUT" | tail -2 | sed 's/^/   /'
UNIT="bf4yjq-crash-$$.scope"
oom_user=$(journalctl --user --no-pager -u "$UNIT" 2>/dev/null | grep -cE "killed by the OOM killer|result 'oom-kill'")
oom_kern=$(journalctl -k --no-pager --since "-30 min" 2>/dev/null | grep -c "oom_memcg=.*$UNIT")
if [[ "$oom_user" -gt 0 || "$oom_kern" -gt 0 ]]; then
  echo "   OOM attribution: user journal records=$oom_user, kernel CONSTRAINT_MEMCG records=$oom_kern ($UNIT)"
fi
if [[ "$sig" == SIG* && "$loose_after_a" -gt 0 && ( "$oom_user" -gt 0 || "$oom_kern" -gt 0 ) ]]; then
  ok "git gc died by signal ($sig, scope exit $rc) with the loose set intact ($loose_after_a objects) and the kill is attributed to the memcg OOM killer — the bf-4yjq kill signature"
else
  fail "expected a memcg-OOM signal kill with loose objects intact: scope exit $rc ($sig), loose=$loose_after_a, oom records user=$oom_user kernel=$oom_kern"
fi

echo
echo "=== B: deployed mitigation — the same gc with pack.windowMemory/deltaCacheSize/threads=1 ==="
git -C "$REPO" config pack.windowMemory "$PACK_WINDOW_MEMORY"
git -C "$REPO" config pack.deltaCacheSize "$PACK_DELTA_CACHE_SIZE"
git -C "$REPO" config pack.threads "$PACK_THREADS"
OUT=$(bounded bf4yjq-bounded-gc "$REPO" git gc)
rc=$?
echo "$OUT" | tail -2 | sed 's/^/   /'
if [[ "$rc" -eq 0 ]]; then
  ok "bounded 'git gc' completed (exit 0) on the same repository that just died"
else
  fail "bounded 'git gc' exited $rc ($(exit_sig "$rc")) — expected the deployed bounds to let it complete"
fi

echo
echo "=== C: historical mitigation — packed store, ordinary agent ops inside ${MEMORY_MAX} ==="
git -C "$REPO" config --unset pack.windowMemory
git -C "$REPO" config --unset pack.deltaCacheSize
git -C "$REPO" config --unset pack.threads
allok=1
for cmd in "git status --porcelain" "git log --oneline -5" "git fsck --full"; do
  OUT=$(bounded bf4yjq-packed-op "$REPO" $cmd)
  rc=$?
  if [[ "$rc" -eq 0 ]]; then
    ok "'$cmd' on the packed repo completed inside ${MEMORY_MAX} (exit 0)"
  else
    fail "'$cmd' on the packed repo exited $rc ($(exit_sig "$rc"))"
    allok=0
  fi
done
packs=$(find "$REPO/.git/objects/pack" -name '*.pack' 2>/dev/null | wc -l)
loose_after_c=$(git -C "$REPO" count-objects -v | awk '/^count:/{print $2}')
[[ "$packs" -ge 1 && "$loose_after_c" -eq 0 ]] \
  && ok "store fully packed ($packs pack, 0 loose) — the state the Aug-13/14 cleanup produced" \
  || fail "expected a packed store, found $packs packs / $loose_after_c loose"

echo
echo "=== $PASS passed, $FAIL failed ==="
exit $(( FAIL > 0 ))
