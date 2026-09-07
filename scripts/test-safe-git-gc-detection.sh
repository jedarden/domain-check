#!/usr/bin/env bash
# Regression test for safe-git-gc.sh bloat detection (domchk-ca7d6d12).
#
# Verifies that `safe-git-gc.sh --check-only` detects a loose-object count
# above the 1000 threshold — the condition that caused the bf-65lsdu OOM
# crashes (4,515 loose objects / 17GB).
#
# Regression being guarded: the detector grepped for '^loose:' in
# `git count-objects` output, a key git never emits (the loose count is
# `count:` in `git count-objects -v`), so it always read 0 and never
# triggered, reporting "GC not needed" for a badly bloated repository.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SAFE_GC="$SCRIPT_DIR/safe-git-gc.sh"

THRESHOLD=1000
NUM_OBJECTS=1100 # above threshold, still fast to create

WORKDIR="$(mktemp -d)"
trap 'rm -rf "$WORKDIR"' EXIT

git init -q -b main "$WORKDIR/repo"
cd "$WORKDIR/repo"
git config user.email github@jedarden.com
git config user.name jedarden

mkdir data
python3 - "$NUM_OBJECTS" <<'PY'
import random, sys
for i in range(int(sys.argv[1])):
    with open(f'data/file{i:05d}.txt', 'w') as f:
        f.write('x' * random.randint(64, 512) + str(random.random()))
PY

git add data
git commit -q -m "bloat detection test data" -- data

loose=$(git count-objects -v | grep '^count:' | awk '{print $2}')
if [[ $loose -le $THRESHOLD ]]; then
  echo "FAIL: expected > $THRESHOLD loose objects in test repo, got $loose" >&2
  exit 1
fi

if "$SAFE_GC" --check-only > "$WORKDIR/out.log" 2>&1; then
  if grep -q "Too many loose objects ($loose > $THRESHOLD)" "$WORKDIR/out.log"; then
    echo "PASS: bloat detected ($loose loose objects > $THRESHOLD)"
  else
    echo "FAIL: check-only exited 0 but not for the loose-object reason:" >&2
    cat "$WORKDIR/out.log" >&2
    exit 1
  fi
else
  echo "FAIL: check-only reported GC not needed with $loose loose objects:" >&2
  cat "$WORKDIR/out.log" >&2
  exit 1
fi
