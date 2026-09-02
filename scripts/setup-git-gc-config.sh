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
# Usage:
#   ./setup-git-gc-config.sh              # apply bounds to this repo (local)
#   ./setup-git-gc-config.sh --global     # apply bounds to ~/.gitconfig (all repos for this user)
#   ./setup-git-gc-config.sh --verify     # check the effective bound (system -> global -> local); exit 1 if unsafe
#   ./setup-git-gc-config.sh --verify --global
#
# Environment overrides:
#   PACK_WINDOW_MEMORY     (default 2g)
#   PACK_DELTA_CACHE_SIZE  (default 1g)
#   PACK_THREADS           (default 1)

set -euo pipefail

WINDOW_MEMORY="${PACK_WINDOW_MEMORY:-2g}"
DELTA_CACHE="${PACK_DELTA_CACHE_SIZE:-1g}"
THREADS="${PACK_THREADS:-1}"

# Total anonymous memory a pack run may reach; must stay well under the
# 12GiB dispatch scope. 6GiB leaves headroom for the object roster and git
# baseline. Used by --verify.
MAX_TOTAL_BYTES=$((6 * 1024 * 1024 * 1024))

MODE=local
VERIFY=0
for arg in "$@"; do
  case "$arg" in
    --global) MODE=global ;;
    --verify) VERIFY=1 ;;
    --help|-h)
      awk 'NR>1 { if ($0 !~ /^#/) exit; sub(/^# ?/, ""); print }' "$0"
      exit 0
      ;;
    *)
      echo "Unknown option: $arg (supported: --global, --verify)" >&2
      exit 2
      ;;
  esac
done

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

# --- Auto-gc policy: advisory, only filled in when absent so rerunning here
# does not clobber this repo's hand-tuned values (gc.auto=100).
git config "${scope_flags[@]}" gc.auto >/dev/null 2>&1 || \
  { git config "${scope_flags[@]}" gc.auto 256; echo "✅ gc.auto = 256 (auto GC when >256 loose objects)"; }
git config "${scope_flags[@]}" gc.autoPackLimit >/dev/null 2>&1 || \
  { git config "${scope_flags[@]}" gc.autoPackLimit 10; echo "✅ gc.autoPackLimit = 10"; }
git config "${scope_flags[@]}" gc.pruneExpire >/dev/null 2>&1 || \
  { git config "${scope_flags[@]}" gc.pruneExpire "2.weeks.ago"; echo "✅ gc.pruneExpire = 2.weeks.ago"; }

echo ""
echo "✅ Pack-memory bounds configured (${label}). Worst case ≈ $(( ($(to_bytes "$WINDOW_MEMORY") * THREADS + $(to_bytes "$DELTA_CACHE")) / 1024 / 1024 ))MiB per pack run."
echo "   Verify anytime:  ./setup-git-gc-config.sh --verify${MODE:+ --global}"
