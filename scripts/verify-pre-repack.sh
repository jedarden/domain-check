#!/usr/bin/env bash
# Pre-Repack Repository Verification
# Verifies repository is in a consistent state and resources are sufficient
# before running git repack. Complements pre-gc-health-check.sh (which gates gc).
#
# Checks (fail => exit 1):
#   1. Parent git gc operation is complete (no gc.pid, no gc/repack processes)
#   2. Repository passes git fsck --full validation
#   3. At least 2GB free disk space available
#   4. No git processes holding locks (index.lock, gc.pid, ref/pack locks)
#
# Usage:
#   ./scripts/verify-pre-repack.sh              # verify and report
#   ./scripts/verify-pre-repack.sh --quiet      # suppress informational stats
#
# Exit codes:
#   0 - repository is verified ready for repack
#   1 - one or more checks failed
#   2 - invalid arguments

set -euo pipefail

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Thresholds (overridable via environment)
MIN_DISK_GB="${MIN_DISK_GB:-2}"
FSCK_TIMEOUT="${FSCK_TIMEOUT:-300}"

QUIET=0
for arg in "$@"; do
    case "$arg" in
        --quiet) QUIET=1 ;;
        -h|--help)
            sed -n '2,17p' "$0" | sed 's/^# \{0,1\}//'
            exit 0
            ;;
        *)
            echo "Unknown argument: $arg" >&2
            echo "Usage: $0 [--quiet]" >&2
            exit 2
            ;;
    esac
done

echo "=== Pre-Repack Repository Verification ==="
echo ""

STATUS=0

# 1. Parent git gc operation must be complete
echo -n "Parent git gc complete: "
GC_PID_FILE=".git/gc.pid"
if [ -f "$GC_PID_FILE" ]; then
    echo -e "${RED}in progress (gc.pid present: $(cat "$GC_PID_FILE" | tr '\n' ' '))${NC}"
    STATUS=1
elif pgrep -f "git(-| )gc|git(-| )multi-pack-index --write" >/dev/null 2>&1; then
    echo -e "${RED}in progress (gc/multi-pack-index process running)${NC}"
    pgrep -af "git(-| )gc|git(-| )multi-pack-index --write" || true
    STATUS=1
else
    echo -e "${GREEN}yes${NC}"
fi

# 2. Repository consistency: full fsck validation
echo -n "git fsck --full: "
FSCK_OUTPUT=""
if ! FSCK_OUTPUT=$(timeout "$FSCK_TIMEOUT" git fsck --full --no-progress 2>&1); then
    RC=$?
    if [ "$RC" -eq 124 ]; then
        echo -e "${RED}timed out after ${FSCK_TIMEOUT}s${NC}"
    else
        echo -e "${RED}failed (exit $RC)${NC}"
    fi
    echo "$FSCK_OUTPUT" | head -20
    STATUS=1
else
    # fsck exit 0; dangling objects are informational, corruption is not
    if [ -n "$FSCK_OUTPUT" ]; then
        if echo "$FSCK_OUTPUT" | grep -q "^dangling"; then
            echo -e "${GREEN}passed${NC} ($(echo "$FSCK_OUTPUT" | wc -l | tr -d ' ') dangling object(s) — informational)"
            [ "$QUIET" -eq 0 ] && echo "$FSCK_OUTPUT" | head -5 | sed 's/^/    /'
        else
            echo -e "${YELLOW}passed with output${NC}"
            echo "$FSCK_OUTPUT" | head -5 | sed 's/^/    /'
        fi
    else
        echo -e "${GREEN}passed (no issues)${NC}"
    fi
fi

# 3. Sufficient disk space for the repack operation
echo -n "Disk space: "
DISK_AVAIL_GB=$(df -BG / | tail -1 | awk '{print $4}' | tr -d 'G')
if [ "$DISK_AVAIL_GB" -ge "$MIN_DISK_GB" ]; then
    echo -e "${GREEN}${DISK_AVAIL_GB}GB available${NC} (>= ${MIN_DISK_GB}GB required)"
else
    echo -e "${RED}${DISK_AVAIL_GB}GB available${NC} (need >= ${MIN_DISK_GB}GB)"
    STATUS=1
fi

# 4. No git processes holding locks
echo -n "Git processes: "
GIT_OPS=$(pgrep -f "git(-| )(repack|index-pack|pack-objects|fsck|prune|commit|merge|rebase|annotate)" || true)
if [ -z "$GIT_OPS" ]; then
    echo -e "${GREEN}none running${NC}"
else
    echo -e "${RED}detected${NC}"
    ps -p $GIT_OPS -o pid,cmd 2>/dev/null || true
    STATUS=1
fi

echo -n "Git lock files: "
LOCKS=""
for lock in .git/index.lock .git/gc.pid .git/config.lock .git/shallow.lock \
            .git/packed-refs.lock .git/packed-refs.new; do
    [ -f "$lock" ] && LOCKS+="$lock"$'\n'
done
# Ref/log/pack update locks are transient git-internal state
while IFS= read -r lock; do
    [ -n "$lock" ] && LOCKS+="$lock"$'\n'
done < <(find .git/refs .git/logs .git/objects/pack -name "*.lock" 2>/dev/null || true)

if [ -z "$LOCKS" ]; then
    echo -e "${GREEN}none held${NC}"
else
    echo -e "${RED}held${NC}"
    echo "$LOCKS" | sed '/^$/d' | sed 's/^/    /'
    STATUS=1
fi

# Informational: non-git custom lock files under .git (do not block repack,
# but flag them so a stale one is noticed)
if [ "$QUIET" -eq 0 ]; then
    CUSTOM_LOCKS=$(find .git -maxdepth 1 -name "*.lock" ! -name "index.lock" ! -name "config.lock" ! -name "shallow.lock" ! -name "packed-refs.lock" 2>/dev/null || true)
    if [ -n "$CUSTOM_LOCKS" ]; then
        echo -n "Non-git custom locks: "
        echo -e "${YELLOW}present (informational, not git-internal)${NC}"
        echo "$CUSTOM_LOCKS" | sed 's/^/    /'
    fi
fi

# Repository statistics (informational)
if [ "$QUIET" -eq 0 ]; then
    echo ""
    echo "Repository statistics:"
    git count-objects -vH | grep -E "^(count|size|packs|size-pack):" | sed 's/^/    /'
fi

echo ""
echo "=== Summary ==="
if [ $STATUS -eq 0 ]; then
    echo -e "${GREEN}✓ Repository verified ready for repack${NC}"
    exit 0
else
    echo -e "${RED}✗ Repository NOT ready for repack — resolve issues above and re-run${NC}"
    exit 1
fi
