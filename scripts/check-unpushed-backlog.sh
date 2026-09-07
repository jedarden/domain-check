#!/usr/bin/env bash
# Unpushed Commit Backlog Monitor — gap analysis M-1
# (docs/crash-prevention-gaps-bf-1ea4g.md §4 M-1, bead domchk-f239e178)
#
# Why this exists: bf-1ea4g (2026-08-13) died 56 times in `git push` because a
# 422-commit unpushed backlog — still carrying retired bead-forge object mass —
# had accumulated silently across ~30 killed attempts, and no check in this
# workspace measured commit-ahead. The only ahead/behind count that existed was
# verify-work-completion.sh, evaluated once per bead at close time. If such a
# backlog ever regrows (a dead worker's unpushed series, a self-amplifying
# retry loop), no daily check, no preflight, and no monitor would mention it.
# This is that check.
#
# REPORT-ONLY, by design: per the G-2 correction (docs/crash-prevention-requirements.md)
# remediation is owned by the unconditional bounded nightly gc (03:00 timer);
# attaching remediation here would re-invent conditional gating that is blind
# to bloat accumulating inside a pack. Detection without an actor is still the
# right instrument for a *precondition* — it turns "slow pushes and close-gate
# failures" into a named, dated log line before the push that dies.
#
# Usage:
#   scripts/check-unpushed-backlog.sh [repo-path]     # default: current dir
#
# Environment:
#   BACKLOG_WARN_THRESHOLD     default 50   (spec: warn at >= 50)
#   BACKLOG_CRITICAL_THRESHOLD default 200  (spec: CRITICAL at >= 200)
#
# Exit codes:
#   0  measured and below the warn threshold, warn only, OR upstream not
#      measurable (fresh clone, no upstream, no origin/main — fails open)
#   1  CRITICAL: backlog at or above BACKLOG_CRITICAL_THRESHOLD
#   2  usage error (path missing or not a git repository)
#
# Output is parseable: the count line is always
#     "ahead of upstream: <N> commit(s)"
# and the level line starts with CLEAR / WARN / CRITICAL.

set -uo pipefail

REPO="${1:-$PWD}"

if [ ! -d "$REPO" ]; then
    echo "USAGE ERROR: not a directory: $REPO" >&2
    exit 2
fi

if ! git -C "$REPO" rev-parse --git-dir >/dev/null 2>&1; then
    echo "USAGE ERROR: not a git repository: $REPO" >&2
    exit 2
fi

WARN_THRESHOLD="${BACKLOG_WARN_THRESHOLD:-50}"
CRITICAL_THRESHOLD="${BACKLOG_CRITICAL_THRESHOLD:-200}"

for v in "$WARN_THRESHOLD" "$CRITICAL_THRESHOLD"; do
    if ! [ "$v" -eq "$v" ] 2>/dev/null; then
        echo "USAGE ERROR: thresholds must be integers (got '$v')" >&2
        exit 2
    fi
done

echo "📜 Unpushed Commit Backlog:"

# Resolve the comparison point: the branch's configured upstream first, then a
# bare origin/main fallback (the pair this repo actually uses).
UPSTREAM="$(git -C "$REPO" rev-parse --abbrev-ref --symbolic-full-name '@{upstream}' 2>/dev/null || true)"
UPSTREAM_SOURCE="branch upstream"
if [ -z "$UPSTREAM" ]; then
    if git -C "$REPO" show-ref --verify --quiet refs/remotes/origin/main; then
        UPSTREAM="origin/main"
        UPSTREAM_SOURCE="fallback (no branch upstream configured)"
    fi
fi

if [ -z "$UPSTREAM" ]; then
    echo "  upstream: none measurable (no branch upstream, no origin/main)"
    echo "  ✅ CLEAR: backlog not measurable here — nothing to compare HEAD against (fails open)"
    exit 0
fi

AHEAD="$(git -C "$REPO" rev-list --count "$UPSTREAM..HEAD" 2>/dev/null || echo none)"
if [ "$AHEAD" = "none" ]; then
    echo "  upstream: $UPSTREAM ($UPSTREAM_SOURCE)"
    echo "  ✅ CLEAR: upstream ref unreadable — backlog not measurable (fails open)"
    exit 0
fi

echo "  upstream: $UPSTREAM ($UPSTREAM_SOURCE)"
echo "  ahead of upstream: $AHEAD commit(s)"

if [ "$AHEAD" -ge "$CRITICAL_THRESHOLD" ]; then
    echo "  🚨 CRITICAL: $AHEAD unpushed commits (>= $CRITICAL_THRESHOLD) — a push materializes the"
    echo "     whole backlog in one pack-objects run (the bf-1ea4g shape); investigate what is"
    echo "     accumulating unpushed before the next push or close gate"
    exit 1
elif [ "$AHEAD" -ge "$WARN_THRESHOLD" ]; then
    echo "  ⚠️  WARN: $AHEAD unpushed commits (>= $WARN_THRESHOLD) — backlog grows silently across"
    echo "     retries and dead workers; push soon and check for an abandoned series"
    exit 0
else
    echo "  ✅ CLEAR: no unpushed backlog ($AHEAD < $WARN_THRESHOLD warn threshold)"
    exit 0
fi
