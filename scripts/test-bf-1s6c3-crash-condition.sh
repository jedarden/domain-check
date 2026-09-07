#!/usr/bin/env bash
# Re-creates the bf-1s6c3 crash conditions at reduced scale, safely, and
# asserts that each deployed prevention layer stops one link of the chain.
#
# The original (2026-08-12, 76 dispatches / 71 memcg-OOM kills over ~4.5 h;
# canonical record: docs/crash-analysis-bf-1s6c3-2026-09-06.md):
#   1. An automated bead-state writer committed 17+ near-identical ~237 MB
#      `.beads/*.jsonl` snapshots, growing the repo to ~18 GB (~17 GB loose).
#   2. Significant git operations then ran git-pack-objects with an unbounded
#      working set inside the dispatch scope's MemoryMax=12GiB.
#   3. 71 of the 76 attempts died at `git push`'s pack-objects; the Aug-14
#      variant died at bare `git gc --aggressive --prune=now` (bf-173o7e /
#      bf-4x12ec). Kernel memcg OOM SIGKILL -> needle-visible `exit -1`.
#
# The full 17 GB condition must NOT be re-created on this box (it is what the
# repo-health guardrails exist to prevent). Like
# scripts/test-bf-4yjq-crash-condition.sh — which reproduces the kill
# mechanism with incompressible random blobs — this harness preserves the
# scaling relation that makes it deterministic (pack-objects' peak RSS scales
# with the loose-object set) and tests at ~1/17th scale, with two
# bf-1s6c3-specific differences:
#   * the bloat is built the way bf-1s6c3's actually was: ONE snapshot path
#     re-committed with a near-identical JSONL payload each round, so the
#     loose store is exactly the delta-search fodder the aggressive gc and
#     the push packer died on;
#   * the first asserted death step is `git push` — where 71/76 of the
#     original deaths happened — before the aggressive-gc variant.
#
# The snapshot payload embeds a fixed random field per record (same bytes in
# every round) and moves only a round counter, so each blob is ~99% identical
# to its predecessor yet barely compressible — real bead JSONL carries ids,
# timestamps and hashes that keep zlib from collapsing it, and this payload
# has the same property by construction. Sizes are quoted in RAW snapshot
# bytes; zlib puts the loose objects at roughly a third of that (measured
# 0.34), so the defaults land ~1 GiB loose.
#
# Assertions:
#   A1 (bloat forms)          16 commits of one ~192 MiB snapshot -> ~1 GiB loose, 0 packs
#   A2 (crash re-created)     bare `git push`       @ MemoryMax=512M -> SIGKILL, OOM attributed, store still loose
#   A3 (crash re-created)     `git gc --aggressive --prune=now` @ 512M -> SIGKILL, OOM attributed, store still loose
#   B1 (bounds mitigate)      same gc + deployed pack.* bounds   -> exit 0 inside the same 512M
#   B2 (bounds mitigate)      same push + deployed pack.* bounds -> exit 0 inside the same 512M
#   C  (gitignore prevents)   live .gitignore rules ignore .beads/** and *.jsonl
#   D  (hook prevents)        installed pre-commit hook blocks an 11 MB file and anything under .beads/
#
# Safety — nothing here can damage the host or the live repo:
#   * every git/gc/push execution runs inside a `systemd-run --user --scope`
#     hard-capped at MemoryMax=512M (MemorySwapMax=0) with a wall-clock
#     timeout; the kill assertions are themselves the proof that the cap
#     contains the blast radius;
#   * all scratch state lives in one mktemp dir under /tmp, removed on exit
#     (keep it with DOMCHECK_KEEP_BF1S6C3=1);
#   * the live repository is only read — its .gitignore and the hook
#     installer are copied/applied to the scratch repo, never written;
#   * preflight refuses to start (exit 2, before any git work) when free
#     disk or available memory is short, when the requested scale exceeds a
#     4 GiB hard cap, or when systemd-run / python3 are unavailable.
#
# Usage:
#   ./scripts/test-bf-1s6c3-crash-condition.sh
#   DOMCHECK_KEEP_BF1S6C3=1 ./scripts/test-bf-1s6c3-crash-condition.sh
#   BF1S6C3_SNAPSHOT_MB=192 BF1S6C3_SNAPSHOTS=16 BF1S6C3_MEMORY_MAX=512M \
#     ./scripts/test-bf-1s6c3-crash-condition.sh        # scale overrides
#
# Exit codes: 0 = all assertions passed; 1 = one or more failed;
#             2 = preflight/config refusal (nothing was run)

set -uo pipefail
cd "$(dirname "$0")/.." || exit 1
ROOT=$PWD

PASS=0
FAIL=0
WORKROOT=""

SNAPSHOT_MB="${BF1S6C3_SNAPSHOT_MB:-64}"   # per-commit payload size (RAW MiB; lands ≈0.5x as loose objects)
SNAPSHOTS="${BF1S6C3_SNAPSHOTS:-32}"       # commits of the one snapshot path
MEMORY_MAX="${BF1S6C3_MEMORY_MAX:-512M}"   # stand-in for the 12GiB dispatch scope
TIMEOUT_SECS="${BF1S6C3_TIMEOUT:-1800}"
MIN_AVAIL_GB="${BF1S6C3_MIN_AVAIL_GB:-2}"
MAX_TOTAL_MB=4096                          # hard ceiling on the built bloat

# Pack bounds scaled to the 512M test scope (deployed values are 2g / 1g / 1 —
# see scripts/setup-git-gc-config.sh; the mechanism under test, window+cache
# caps with threads pinned, is identical).
PACK_WINDOW_MEMORY="${BF1S6C3_WINDOW_MEMORY:-128m}"
PACK_DELTA_CACHE_SIZE="${BF1S6C3_DELTA_CACHE:-64m}"
PACK_THREADS="${BF1S6C3_THREADS:-1}"

ok()   { echo "✅ PASS: $*"; PASS=$((PASS + 1)); }
fail() { echo "❌ FAIL: $*"; FAIL=$((FAIL + 1)); }
refuse() { echo "🛑 REFUSED: $*" >&2; exit 2; }

cleanup() {
  if [[ -n "$WORKROOT" && -d "$WORKROOT" && "${DOMCHECK_KEEP_BF1S6C3:-0}" != "1" ]]; then
    rm -rf "$WORKROOT"
  elif [[ -d "$WORKROOT" ]]; then
    echo "   (artifacts kept: $WORKROOT)"
  fi
}
trap cleanup EXIT

# ---------- preflight (fail fast, before any git work) ----------

TOTAL_MB=$(( SNAPSHOT_MB * SNAPSHOTS ))
NEED_MB=$(( TOTAL_MB * 3 + 1024 ))   # loose + killed-push partial pack + packed result + margin

command -v git         >/dev/null || refuse "git not found"
command -v systemd-run >/dev/null || refuse "systemd-run not found — the MemoryMax bound is the safety envelope"
command -v python3     >/dev/null || refuse "python3 not found — used to generate the JSONL snapshots"
command -v journalctl  >/dev/null || refuse "journalctl not found — needed to attribute the OOM kill"

(( TOTAL_MB > 0 && TOTAL_MB <= MAX_TOTAL_MB )) \
  || refuse "scale ${TOTAL_MB}MiB (SNAPSHOT_MB=$SNAPSHOT_MB x SNAPSHOTS=$SNAPSHOTS) exceeds the ${MAX_TOTAL_MB}MiB hard cap"

free_mb=$(df -BM --output=avail /tmp | tail -1 | tr -dc '0-9')
free_mb=${free_mb:-0}
(( free_mb >= NEED_MB )) \
  || refuse "only ${free_mb}MiB free on /tmp, need >= ${NEED_MB}MiB for a ${TOTAL_MB}MiB bloat run"

avail_gb=$(free -g | awk '/^Mem:/{print $7}')
avail_gb=${avail_gb:-0}
(( avail_gb >= MIN_AVAIL_GB )) \
  || refuse "only ${avail_gb}GiB MemAvailable, need >= ${MIN_AVAIL_GB}GiB"

have_cgroup=false
if systemd-run --user --quiet --scope --unit="bf1s6c3-probe-$$" -p MemoryMax=64M true 2>/dev/null; then
  have_cgroup=true
else
  refuse "systemd-run --user scope test failed — cannot bound the crash operations"
fi

# ---------- helpers ----------

# bounded <unit-name> <workdir> <command...> -> runs under MemoryMax, prints output
# Callers must pass a unit name unique per call: systemd rejects a transient
# scope whose same-named predecessor is still tearing down ("was already
# loaded or has a fragment file") with a client-side exit 1 before the command
# runs, so uniqueness lives at the call site (this function runs inside $( )
# and cannot hand a generated name back).
bounded() {
  local unit=$1 dir=$2
  shift 2
  ( cd "$dir" && timeout "$TIMEOUT_SECS" systemd-run --user --quiet --scope \
      --unit="$unit" -p MemoryMax="$MEMORY_MAX" -p MemorySwapMax=0 \
      "$@" ) 2>&1
}

exit_sig() {  # decode systemd-run scope exit status to "0" or "SIG<NAME/n>"
  local rc=$1 n
  if (( rc >= 128 )); then
    n=$((rc - 128))
    case "$n" in
      9) echo SIGKILL ;;
      15) echo SIGTERM ;;
      24) echo "timeout(SIGTERM)" ;;
      *) echo "SIG$n" ;;
    esac
  else
    echo "$rc"
  fi
}

loose_n()   { git -C "$1" count-objects -v | awk '/^count:/{print $2}'; }
loose_kib() { git -C "$1" count-objects -v | awk '/^size:/{print $2}'; }
packs_n()   { find "$1/.git/objects/pack" -name '*.pack' 2>/dev/null | wc -l; }

# oom_records <unit> -> "<user-journal hits> <kernel hits>"
# Kernel CONSTRAINT_MEMCG lines carry oom_memcg=<...unit>.scope; the user
# journal carries systemd's own "killed by the OOM killer" notice. A notice
# with no kernel line can also be a NixOS switch-to-configuration replay of
# stale counters, so both are reported and either corroborates the kill.
oom_records() {
  local u=$1
  local oom_user oom_kern
  oom_user=$(journalctl --user --no-pager -u "$u" 2>/dev/null | grep -cE "killed by the OOM killer|oom-kill")
  oom_kern=$(journalctl -k --no-pager --since "-30 min" 2>/dev/null | grep -c "oom_memcg=.*$u")
  echo "$oom_user $oom_kern"
}

new_bare_remote() {  # fresh remote per push so a killed push's partial pack never leaks into the next assertion
  rm -rf "$WORKROOT/remote.git"
  git init -q --bare "$WORKROOT/remote.git"
}

# write_snapshots <outdir> <rounds> <records> — all rounds in ONE python
# process, so the per-record random field is generated once and reused: every
# round's file is byte-identical except the round counter, i.e. maximally
# delta-able, while the random field keeps zlib from collapsing the blob.
# One file per round (snapshot-<n>.jsonl); the caller commits them in
# sequence, because the whole point is N commits of one evolving path.
write_snapshots() {
  python3 - "$1" "$2" "$3" <<'PY'
import base64, os, sys
outdir, rounds, records = sys.argv[1], int(sys.argv[2]), int(sys.argv[3])
filler = ("database holding bead state: ids, titles, statuses, priorities, "
          "labels, dependency edges and external references ")
# one fixed random field per record — identical in every round
payloads = [base64.b64encode(os.urandom(84)).decode() for _ in range(records)]
# pre-render everything except the round counter (last field on each line)
prefixes = []
for i, payload in enumerate(payloads):
    prefixes.append(
        f'{{"rec":{i},"id":"bf-1s6c3-{i}","issue_type":"task","status":"open",'
        f'"priority":2,"title":"snapshot record {i}","updated":"2026-08-12T21:36:{(i % 60):02d}.{i:09d}Z",'
        f'"body":"{filler}{payload}","seq":'
    )
prefixes = [p.encode() for p in prefixes]
for rnd in range(1, rounds + 1):
    with open(os.path.join(outdir, f"snapshot-{rnd}.jsonl"), "wb", buffering=1024 * 1024) as out:
        tail = f"{rnd}}}\n".encode()
        for p in prefixes:
            out.write(p + tail)
PY
}

# ---------- build the crash precondition (A1) ----------

WORKROOT=$(mktemp -d /tmp/bf1s6c3-repro.XXXXXX) || exit 1
REPO="$WORKROOT/bloated"
POLICY="$WORKROOT/policy"
REMOTE="$WORKROOT/remote.git"

git init -q "$REPO" || exit 1
git -C "$REPO" config user.email github@jedarden.com
git -C "$REPO" config user.name jedarden
new_bare_remote

BRANCH=$(git -C "$REPO" symbolic-ref --short HEAD)
git -C "$REPO" remote add origin "$REMOTE"                        # so bare `git push`
git -C "$REPO" config "branch.$BRANCH.remote" origin              # (the original
git -C "$REPO" config "branch.$BRANCH.merge" "refs/heads/$BRANCH" # command shape) works

echo "=== A1: building the bloat the way bf-1s6c3 formed it — ${SNAPSHOTS} commits of one near-identical ${SNAPSHOT_MB}MiB .beads JSONL snapshot ==="
echo "   repo: $REPO   bound: MemoryMax=$MEMORY_MAX"

# ~250-byte records -> records-per-snapshot from the target payload size
RECORDS=$(( SNAPSHOT_MB * 1024 * 1024 / 250 ))
SNAP="$REPO/.beads/issues.jsonl"
mkdir -p "$REPO/.beads" "$WORKROOT/snaps"
build_start=$SECONDS
write_snapshots "$WORKROOT/snaps" "$SNAPSHOTS" "$RECORDS"
for i in $(seq 1 "$SNAPSHOTS"); do
  cp "$WORKROOT/snaps/snapshot-$i.jsonl" "$SNAP"
  rm -f "$WORKROOT/snaps/snapshot-$i.jsonl"   # keep only one raw copy on disk; the loose object holds the rest
  git -C "$REPO" add ".beads/issues.jsonl"
  git -C "$REPO" commit -qm "bead-state snapshot $i"
done
loose_mib=$(( $(loose_kib "$REPO") / 1024 ))
echo "   built in $(( SECONDS - build_start ))s: $(loose_n "$REPO") loose objects, ${loose_mib}MiB loose (${TOTAL_MB}MiB raw), $(packs_n "$REPO") packs"

min_loose_mib=$(( TOTAL_MB * 30 / 100 ))   # measured payload deflate ratio ≈ 0.34 — floor at 0.30
if [[ "$loose_mib" -ge "$min_loose_mib" && "$(packs_n "$REPO")" -eq 0 ]]; then
  ok "A1: bloat formed — ${loose_mib}MiB across $(loose_n "$REPO") loose objects, 0 packs (>= ${min_loose_mib}MiB expected; the ~1/17th-scale stand-in for 17GB loose)"
else
  fail "A1: expected >= ${min_loose_mib}MiB loose and 0 packs, got ${loose_mib}MiB / $(packs_n "$REPO") packs"
fi

if [[ "$have_cgroup" != true ]]; then
  fail "systemd-run --user unavailable — cannot assert the memcg OOM kills (A2, A3) or the bounded mitigations (B1, B2)"
  echo; echo "=== $PASS passed, $FAIL failed ==="
  exit 1
fi

# ---------- A2: the actual death step — push ----------

echo
echo "=== A2: the crash re-created — bare 'git push' (71/76 original deaths) over ${loose_mib}MiB loose inside ${MEMORY_MAX} ==="
new_bare_remote
push_start=$SECONDS
OUT=$(bounded "bf1s6c3-push-a2-$$" "$REPO" git push)
rc=$?
sig=$(exit_sig "$rc")
push_secs=$(( SECONDS - push_start ))
read -r oom_user oom_kern <<< "$(oom_records "bf1s6c3-push-a2-$$.scope")"
loose_a2=$(( $(loose_kib "$REPO") / 1024 ))
echo "   push ran ${push_secs}s, scope exit $rc ($sig), OOM records: user=$oom_user kernel=$oom_kern, loose after: ${loose_a2}MiB"
if [[ "$rc" -eq 124 ]]; then
  fail "A2: push hit the ${TIMEOUT_SECS}s timeout instead of dying to memcg OOM — raise BF1S6C3_TIMEOUT or increase scale"
elif [[ "$sig" == SIG* && "$loose_a2" -gt 0 && ( "$oom_user" -gt 0 || "$oom_kern" -gt 0 ) ]]; then
  ok "A2: push died by signal ($sig) with the loose set intact and the kill attributed to the memcg OOM killer — the bf-1s6c3 death step"
else
  fail "A2: expected a memcg-OOM signal kill at push: exit $rc ($sig), loose=${loose_a2}MiB, oom records user=$oom_user kernel=$oom_kern"
fi

# ---------- A3: the Aug-14 variant — bare aggressive gc ----------

echo
echo "=== A3: the crash re-created — bare 'git gc --aggressive --prune=now' inside ${MEMORY_MAX} ==="
gc_start=$SECONDS
OUT=$(bounded "bf1s6c3-gc-a3-$$" "$REPO" git gc --aggressive --prune=now)
rc=$?
sig=$(exit_sig "$rc")
gc_secs=$(( SECONDS - gc_start ))
read -r oom_user oom_kern <<< "$(oom_records "bf1s6c3-gc-a3-$$.scope")"
loose_a3=$(loose_n "$REPO")
echo "   gc ran ${gc_secs}s, scope exit $rc ($sig), OOM records: user=$oom_user kernel=$oom_kern, loose after: $loose_a3 objects / $(packs_n "$REPO") packs"
if [[ "$rc" -eq 124 ]]; then
  fail "A3: gc hit the ${TIMEOUT_SECS}s timeout instead of dying to memcg OOM — raise BF1S6C3_TIMEOUT or increase scale"
elif [[ "$sig" == SIG* && "$loose_a3" -gt 0 && ( "$oom_user" -gt 0 || "$oom_kern" -gt 0 ) ]]; then
  ok "A3: aggressive gc died by signal ($sig) with the loose set intact and the kill attributed to memcg OOM — the bf-173o7e/bf-4x12ec variant of the same mechanism"
else
  fail "A3: expected a memcg-OOM signal kill at gc: exit $rc ($sig), loose=$loose_a3 objects, oom records user=$oom_user kernel=$oom_kern"
fi

# ---------- B1/B2: the deployed pack bounds let the same operations finish ----------

echo
echo "=== B1: deployed mitigation — the same aggressive gc with pack.windowMemory/deltaCacheSize/threads bounds ==="
git -C "$REPO" config pack.windowMemory "$PACK_WINDOW_MEMORY"
git -C "$REPO" config pack.deltaCacheSize "$PACK_DELTA_CACHE_SIZE"
git -C "$REPO" config pack.threads "$PACK_THREADS"
gc_start=$SECONDS
OUT=$(bounded "bf1s6c3-gc-b1-$$" "$REPO" git gc --aggressive --prune=now)
rc=$?
gc_secs=$(( SECONDS - gc_start ))
echo "   gc ran ${gc_secs}s, scope exit $rc ($(exit_sig "$rc"))"
if [[ "$rc" -eq 0 && "$(packs_n "$REPO")" -ge 1 ]]; then
  ok "B1: bounded aggressive gc completed (exit 0, $(packs_n "$REPO") pack) on the same repo that just died — worst case ≈ ${PACK_WINDOW_MEMORY} x ${PACK_THREADS} + ${PACK_DELTA_CACHE_SIZE}, inside ${MEMORY_MAX}"
else
  fail "B1: bounded aggressive gc exited $rc ($(exit_sig "$rc")) with $(packs_n "$REPO") packs — expected the deployed bounds to let it finish"
fi

echo
echo "=== B2: deployed mitigation — the same push over the packed store, fresh remote ==="
new_bare_remote
push_start=$SECONDS
OUT=$(bounded "bf1s6c3-push-b2-$$" "$REPO" git push)
rc=$?
push_secs=$(( SECONDS - push_start ))
remote_head=no
git -C "$REMOTE" rev-parse --verify -q HEAD >/dev/null 2>&1 && remote_head=yes
echo "   push ran ${push_secs}s, scope exit $rc ($(exit_sig "$rc")), remote has HEAD: $remote_head"
if [[ "$rc" -eq 0 && "$remote_head" == yes ]]; then
  ok "B2: bounded push completed (exit 0) and the remote received the branch — the step that killed 71/76 original dispatches now finishes inside ${MEMORY_MAX}"
else
  fail "B2: bounded push exited $rc ($(exit_sig "$rc")), remote HEAD present: $remote_head"
fi
git -C "$REPO" config --unset pack.windowMemory
git -C "$REPO" config --unset pack.deltaCacheSize

# ---------- C: the gitignore layer — the snapshot path can no longer enter history ----------

echo
echo "=== C: prevention layer — the live repo's .gitignore rules refuse the bf-1s6c3 payload shapes ==="
git init -q "$POLICY"
cp "$ROOT/.gitignore" "$POLICY/.gitignore"
all_ignored=1
for p in ".beads/issues.jsonl" ".beads/beads.db" ".beads/checkpoint/current.json" "stray-snapshot.jsonl"; do
  if git -C "$POLICY" check-ignore -q "$p"; then
    echo "   ignored: $p ($(git -C "$POLICY" check-ignore -v "$p" | cut -f1))"
  else
    echo "   NOT ignored: $p"
    all_ignored=0
  fi
done
if [[ "$all_ignored" -eq 1 ]]; then
  ok "C: every bf-1s6c3 payload shape (.beads/**, *.jsonl, *.db) is refused by the deployed gitignore before a commit can form"
else
  fail "C: at least one payload shape is no longer gitignored — the bloat-forming path is open again"
fi

# ---------- D: the hook layer — the backstop that would have blocked the 237MB commits ----------

echo
echo "=== D: prevention layer — installed pre-commit hook blocks an oversized file and anything under .beads/ ==="
( cd "$POLICY" && "$ROOT/scripts/setup-git-hooks.sh" ) >/dev/null || fail "D: hook installer failed in the scratch repo"

hook_blocks=1

head -c $(( 11 * 1024 * 1024 )) /dev/zero > "$POLICY/big-blob.bin"
git -C "$POLICY" add "big-blob.bin"
if OUT=$( cd "$POLICY" && git commit -qm "oversized file" 2>&1 ); then
  echo "   11 MB file: commit was ALLOWED (hook did not fire)"
  hook_blocks=0
else
  echo "   11 MB file: commit blocked ($(echo "$OUT" | grep -m1 'COMMIT BLOCKED' | sed 's/\x1b\[[0-9;]*m//g' | cut -c1-90))"
fi

git -C "$POLICY" reset -q
mkdir -p "$POLICY/.beads"
echo '{"id":"bf-1s6c3"}' > "$POLICY/.beads/issues.jsonl"
git -C "$POLICY" add -f ".beads/issues.jsonl"
if OUT=$( cd "$POLICY" && git commit -qm "bead state" 2>&1 ); then
  echo "   .beads/issues.jsonl: commit was ALLOWED (hook did not fire)"
  hook_blocks=0
else
  echo "   .beads/issues.jsonl: commit blocked ($(echo "$OUT" | grep -m1 'COMMIT BLOCKED' | sed 's/\x1b\[[0-9;]*m//g' | cut -c1-90))"
fi

if [[ "$hook_blocks" -eq 1 ]]; then
  ok "D: the installed hook blocks both the 11 MB file and the force-added .beads/ snapshot — the commits that bloated this repo are refused at the door"
else
  fail "D: the hook let a bf-1s6c3-shaped payload through"
fi

echo
echo "=== $PASS passed, $FAIL failed ==="
exit $(( FAIL > 0 ))
