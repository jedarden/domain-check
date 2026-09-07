#!/usr/bin/env bash
# Configure git so that even a bare `git gc --aggressive` cannot exceed the
# memory ceiling of a needle dispatch scope (run-p*.scope, MemoryMax=12GiB in
# the systemd user manager).
#
# Root cause this addresses (bf-173o7e, bf-4x12ec): `git gc --aggressive
# --prune=now` over a bloated repo pushed git-pack-objects RSS past the 12GiB
# memcg cap and the kernel SIGKILLed the agent (exit code -1, 129 attempts).
# safe-git-gc.sh bounds only its own sanctioned path; the bare invocation is
# defended solely by pack.* config. These settings therefore must be
# persistent and verifiable, not hand-applied per repo.
#
# Memory math (git 2.50.1 docs): pack.windowMemory caps the delta search
# window PER THREAD and pack.threads MULTIPLIES it, so the hard ceiling is
# roughly  windowMemory * threads + deltaCacheSize.  threads=1 is what makes
# windowMemory a whole-process bound; leaving threads unset lets git use all
# cores and scales the window back up, which is the failure mode we are
# closing. With the defaults below the ceiling is 2g*1 + 1g = 3GiB — a
# quarter of the dispatch scope, with the object roster on top.
#
# Auto-gc concurrency bound (GAP-2 of the bf-65lsdu mitigation proposal,
# docs/fix-proposal-bf-65lsdu-oom-git-gc-2026-09-02.md): bf-65lsdu was not one
# runaway gc but SEVERAL concurrent ones — memory-bounded individually, yet
# unserialized, which is what amplified a single OOM into 7 crashes. A nonzero
# gc.auto re-opens that path: every routine git command spawns a background
# auto-gc that the box-wide safe-git-gc lock never sees. Repo-local mode
# therefore sets gc.auto=0 in the safety core; packing is the nightly
# safe-git-gc timer's job (24h loose-object latency, far under the 500MB
# warning threshold). Global mode deliberately leaves gc.auto advisory
# (fill-when-absent below): the other repos on this box have no maintenance
# timer that would compensate.
#
# Usage:
#   ./setup-git-gc-config.sh              # apply bounds to this repo (local)
#   ./setup-git-gc-config.sh --global     # apply bounds to ~/.gitconfig (all repos for this user)
#   ./setup-git-gc-config.sh --verify     # check the effective bound (system -> global -> local); exit 1 if unsafe
#   ./setup-git-gc-config.sh --verify --global
#   ./setup-git-gc-config.sh --uninstall          # remove the bounds from this repo
#   ./setup-git-gc-config.sh --uninstall --global # remove the bounds from ~/.gitconfig
#
# --uninstall is the rollback step for this layer of the bf-1s6c3 mitigation
# stack (rollback plan for the whole stack:
# docs/maintenance/repository-maintenance-guide.md). It removes only the three
# pack.* keys this script owns and then re-runs --verify, exiting 1 when the
# rollback removed the LAST effective bound — bare gc/push are unbounded in
# that state, which is the exit-code -1 mechanism this script exists to close.
#
# Environment overrides:
#   PACK_WINDOW_MEMORY     (default 2g)
#   PACK_DELTA_CACHE_SIZE  (default 1g)
#   PACK_THREADS           (default 1)
#   GC_AUTO                (default 0; repo-local mode only — the deliberate
#                          re-enable escape hatch for the auto-gc policy)

set -euo pipefail

WINDOW_MEMORY="${PACK_WINDOW_MEMORY:-2g}"
DELTA_CACHE="${PACK_DELTA_CACHE_SIZE:-1g}"
THREADS="${PACK_THREADS:-1}"
GC_AUTO="${GC_AUTO:-0}"

# Total anonymous memory a pack run may reach; must stay well under the
# 12GiB dispatch scope. 6GiB leaves headroom for the object roster and git
# baseline. Used by --verify.
MAX_TOTAL_BYTES=$((6 * 1024 * 1024 * 1024))

MODE=local
VERIFY=0
UNINSTALL=0
for arg in "$@"; do
  case "$arg" in
    --global) MODE=global ;;
    --verify) VERIFY=1 ;;
    --uninstall) UNINSTALL=1 ;;
    --help|-h)
      awk 'NR>1 { if ($0 !~ /^#/) exit; sub(/^# ?/, ""); print }' "$0"
      exit 0
      ;;
    *)
      echo "Unknown option: $arg (supported: --global, --verify, --uninstall)" >&2
      exit 2
      ;;
  esac
done

if (( VERIFY && UNINSTALL )); then
  echo "--verify is check-only and cannot be combined with --uninstall" >&2
  exit 2
fi

scope_flags=(--local)
label="repo-local"
if [[ "$MODE" == "global" ]]; then
  scope_flags=(--global)
  label="global (~/.gitconfig)"
fi

# "2g" / "512m" / "1024k" / bare bytes -> bytes
to_bytes() {
  local v="${1,,}"
  local n="${v:0:-1}" unit="${v: -1}"
  case "$unit" in
    k) echo $((n * 1024)) ;;
    m) echo $((n * 1024 * 1024)) ;;
    g) echo $((n * 1024 * 1024 * 1024)) ;;
    *) echo "$v" ;;
  esac
}

if [[ "$VERIFY" == "1" ]]; then
  # A bare gc sees the EFFECTIVE config (system -> global -> local). Repo-local
  # mode must resolve that whole chain, not just .git/config, or a repo
  # protected only by the box-wide global bound reports UNSAFE — a false alarm
  # in exactly the repos the global setup exists to protect. --verify --global
  # still checks ~/.gitconfig itself.
  if [[ "$MODE" == "global" ]]; then
    lookup() { git config --global --get "$1" 2>/dev/null || true; }
    origin_of() { echo global; }
    chain="global (~/.gitconfig)"
  else
    lookup() { git config --get "$1" 2>/dev/null || true; }
    origin_of() {  # which scope supplies this key: local | global | system
      if [[ -n "$(git config --local --get "$1" 2>/dev/null || true)" ]]; then
        echo local
      elif [[ -n "$(git config --global --get "$1" 2>/dev/null || true)" ]]; then
        echo global
      else
        echo system
      fi
    }
    chain="effective (system -> global -> local)"
  fi
  missing=()
  origins=""
  for key in pack.windowMemory pack.deltaCacheSize pack.threads; do
    if [[ -z "$(lookup "$key")" ]]; then
      missing+=("$key")
    else
      origins+=" ${key#pack.}=$(origin_of "$key")"
    fi
  done
  if (( ${#missing[@]} )); then
    echo "❌ UNSAFE: no effective bound for ${missing[*]} (${chain})."
    echo "   A bare 'git gc --aggressive' in this state is unbounded and can exceed"
    echo "   the 12GiB needle dispatch scope (memcg OOM SIGKILL, exit code -1)."
    if [[ "$MODE" == "global" ]]; then
      echo "   Fix: ./setup-git-gc-config.sh --global"
    else
      echo "   Fix: ./setup-git-gc-config.sh           (this repo)"
      echo "        ./setup-git-gc-config.sh --global  (every repo for this user)"
    fi
    exit 1
  fi
  wm=$(to_bytes "$(lookup pack.windowMemory)")
  dc=$(to_bytes "$(lookup pack.deltaCacheSize)")
  th=$(lookup pack.threads)
  if ! [[ "$th" =~ ^[0-9]+$ ]] || (( th < 1 )); then
    echo "❌ UNSAFE: pack.threads='$th' — unset or 0 lets git auto-size threads and multiply the window." >&2
    exit 1
  fi
  total=$((wm * th + dc))
  if (( total > MAX_TOTAL_BYTES )); then
    printf '❌ UNSAFE: worst-case pack memory %d bytes (windowMemory %d x threads %s + deltaCache %d) exceeds %d-byte ceiling.\n' \
      "$total" "$wm" "$th" "$dc" "$MAX_TOTAL_BYTES" >&2
    exit 1
  fi
  echo "✅ Verified — ${chain}; scope:${origins}; worst-case pack memory ≈ $((total / 1024 / 1024))MiB (windowMemory=$wm, threads=$th, deltaCache=$dc) — within the ${MAX_TOTAL_BYTES} ceiling for a 12GiB dispatch scope."
  exit 0
fi

if [[ "$UNINSTALL" == "1" ]]; then
  echo "Removing pack-memory bounds (${label})..."
  for key in pack.windowMemory pack.deltaCacheSize pack.threads; do
    if [[ -n "$(git config "${scope_flags[@]}" --get "$key" 2>/dev/null || true)" ]]; then
      # --unset-all, not --unset: a multi-valued key would make --unset error
      # out and leave the bound half-removed.
      git config "${scope_flags[@]}" --unset-all "$key"
      echo "🗑  removed ${key} (${label})"
    else
      echo "·  ${key} not set in ${label} (nothing to remove)"
    fi
  done
  if [[ "$MODE" == "local" ]]; then
    # gc.auto is safety-core in repo-local mode, so the rollback removes it
    # too; the effective policy falls back to global/default (auto-gc on).
    if [[ -n "$(git config --local --get gc.auto 2>/dev/null || true)" ]]; then
      git config --local --unset-all gc.auto
      echo "🗑  removed gc.auto (repo-local) — background auto-gc falls back to the global/default policy"
    else
      echo "·  gc.auto not set in repo-local (nothing to remove)"
    fi
    advisory="gc.autoPackLimit, gc.pruneExpire"
  else
    advisory="gc.auto, gc.autoPackLimit, gc.pruneExpire"
  fi
  echo ""
  echo "Advisory keys (${advisory}) are left in place —"
  echo "this script only fills them when absent, so they may hold hand-tuned values."
  echo ""
  verify_args=(--verify)
  if [[ "$MODE" == "global" ]]; then
    verify_args+=(--global)
  fi
  if "$0" "${verify_args[@]}"; then
    echo ""
    echo "✅ Rollback complete (${label}); the effective pack-memory bound still holds — bare git stays bounded."
  else
    echo ""
    echo "❌ Rollback removed the LAST effective pack-memory bound. Bare 'git gc --aggressive' and" >&2
    echo "   'git push' are unbounded in this state and can exceed the 12GiB dispatch scope" >&2
    echo "   (memcg OOM SIGKILL, exit code -1 — the bf-173o7e / bf-4x12ec mechanism)." >&2
    echo "   Re-apply with:" >&2
    if [[ "$MODE" == "global" ]]; then
      echo "     ./setup-git-gc-config.sh --global" >&2
    else
      echo "     ./setup-git-gc-config.sh            (this repo)" >&2
      echo "     ./setup-git-gc-config.sh --global   (every repo for this user)" >&2
    fi
    exit 1
  fi
  exit 0
fi

echo "Configuring git pack-memory bounds (${label})..."

# --- Safety core: enforced unconditionally. These are the fix, so they win
# over any earlier value; operators who tuned them consciously can rerun with
# the environment overrides above.
wm_old=$(git config "${scope_flags[@]}" --get pack.windowMemory || true)
git config "${scope_flags[@]}" pack.windowMemory "$WINDOW_MEMORY"
echo "✅ pack.windowMemory = $WINDOW_MEMORY (was: ${wm_old:-unset}) caps the delta search window"

dc_old=$(git config "${scope_flags[@]}" --get pack.deltaCacheSize || true)
git config "${scope_flags[@]}" pack.deltaCacheSize "$DELTA_CACHE"
echo "✅ pack.deltaCacheSize = $DELTA_CACHE (was: ${dc_old:-unset}) caps the delta write-out cache"

th_old=$(git config "${scope_flags[@]}" --get pack.threads || true)
git config "${scope_flags[@]}" pack.threads "$THREADS"
echo "✅ pack.threads = $THREADS (was: ${th_old:-unset}) stops the per-thread window multiplication"

# gc.auto is safety-core in repo-local mode only (GAP-2): a nonzero value
# re-opens the unserialized background auto-gc path, so here it wins over any
# earlier value. Global mode skips this — see the advisory block below.
if [[ "$MODE" == "local" ]]; then
  ga_old=$(git config --local --get gc.auto || true)
  git config --local gc.auto "$GC_AUTO"
  echo "✅ gc.auto = $GC_AUTO (was: ${ga_old:-unset}) closes the unserialized background auto-gc path; packing is the nightly safe-git-gc timer's job"
fi

# --- Auto-gc policy: advisory, only filled in when absent so rerunning does
# not clobber hand-tuned values. Repo-local gc.auto never reaches this block
# (the safety core owns it above); global mode still fills it because the
# other repos on this box have no maintenance timer to compensate.
git config "${scope_flags[@]}" gc.auto >/dev/null 2>&1 || \
  { git config "${scope_flags[@]}" gc.auto 256; echo "✅ gc.auto = 256 (auto GC when >256 loose objects)"; }
git config "${scope_flags[@]}" gc.autoPackLimit >/dev/null 2>&1 || \
  { git config "${scope_flags[@]}" gc.autoPackLimit 10; echo "✅ gc.autoPackLimit = 10"; }
git config "${scope_flags[@]}" gc.pruneExpire >/dev/null 2>&1 || \
  { git config "${scope_flags[@]}" gc.pruneExpire "2.weeks.ago"; echo "✅ gc.pruneExpire = 2.weeks.ago"; }

echo ""
echo "✅ Pack-memory bounds configured (${label}). Worst case ≈ $(( ($(to_bytes "$WINDOW_MEMORY") * THREADS + $(to_bytes "$DELTA_CACHE")) / 1024 / 1024 ))MiB per pack run."
echo "   Verify anytime:  ./setup-git-gc-config.sh --verify${MODE:+ --global}"
