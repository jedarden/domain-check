#!/usr/bin/env bash
# RETIRED — do not use.
#
# This script used to run `git filter-repo --force` (or BFG / filter-branch)
# followed by a bare `git gc --aggressive --prune=now`. Both halves are
# implicated in this workspace's crash history:
#
#   - The bare aggressive gc is the exact pattern that OOM-killed the box in
#     bf-1s6c3 and bf-65lsdu (17GB+ of loose objects, exit code -1 mid-gc).
#     Cleanup now goes through scripts/cleanup-bloat.sh, which pre-flights
#     resources, monitors the git process tree's RSS, and checkpoints stages.
#   - The history-rewrite half stripped .beads/ paths from history and ended
#     by inviting a force-push to "verify history". Force pushes are
#     prohibited here, and rewriting history is a deliberate one-off
#     operation that must never sit behind an unattended "cleanup" name.
#
# Nothing calls this script. It is kept as a pointer so stale notes, aliases
# or cron entries fail loudly instead of silently rewriting history.
#
# If you genuinely need objects removed from history, do it by hand, in a
# throwaway clone, with an explicit plan — not via this file.

set -euo pipefail

REPO_ROOT="$(git rev-parse --show-toplevel 2>/dev/null || pwd)"

cat >&2 <<EOF
ERROR: scripts/cleanup-repo-bloat.sh is retired.

It ran an unbounded 'git gc --aggressive --prune=now' (the pattern that
OOM-killed this box in bf-1s6c3 and bf-65lsdu) plus a forced history rewrite.
Neither is safe to run unattended.

Replacements, in order of preference:
  ./scripts/cleanup-bloat.sh --check-only   # is cleanup needed? (no changes)
  ./scripts/cleanup-bloat.sh --dry-run      # pre-flight checks + staged plan
  ./scripts/cleanup-bloat.sh                # monitored, staged bloat cleanup
  ./scripts/cleanup-bloat.sh --resume       # continue an interrupted run
  ./scripts/safe-git-gc.sh                  # routine maintenance gc

History rewriting (removing .beads/ or other paths from past commits) is NOT
automated: do it manually in a throwaway clone.

Nothing was modified. Refusing to continue (repo: $REPO_ROOT).
EOF

exit 1
