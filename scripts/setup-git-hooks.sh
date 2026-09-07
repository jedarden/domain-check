#!/usr/bin/env bash
# Install the repository-bloat pre-commit hook from its tracked source.
#
# Closes gap G-1 of docs/crash-prevention-requirements.md: the hook previously
# existed only as a hand-installed, per-clone copy, so a fresh clone had no
# bloat protection at all. This installer makes that one command reproducible.
#
# Usage:
#   scripts/setup-git-hooks.sh              # install (idempotent)
#   scripts/setup-git-hooks.sh --check      # verify only; exit 1 when missing,
#                                           # not executable, or drifted
#   scripts/setup-git-hooks.sh --uninstall  # remove the installed hook
#
# The hook blocks staged files >10MB, commits whose total staged payload
# exceeds 50MB, and anything staged under .beads/. Rationale is documented in
# the hook source (scripts/pre-commit-repo-size-hook).
#
# Alternative for clones that want the hook without running the installer:
#   git config --local core.hooksPath scripts/git-hooks   # then commit the
#   hook under that path. This repo standardizes on the installer instead, so
#   there is exactly one canonical copy of the hook.

set -euo pipefail

usage() {
  sed -n '2,22p' "$0" | sed 's/^# \{0,1\}//'
  exit "${1:-0}"
}

case "${1:-}" in
  "") MODE="install" ;;
  --check) MODE="check" ;;
  --uninstall) MODE="uninstall" ;;
  -h | --help) usage 0 ;;
  *) echo "Unknown option: $1" >&2; usage 1 ;;
esac

# Source lives next to this script (the checkout that tracks it); the target
# is the repo the invoking directory belongs to. They are normally the same
# checkout, but the split lets a clone install its hook from a pristine
# source tree — which is exactly what the self-test exercises.
SCRIPT_DIR=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd -P)
SOURCE="$SCRIPT_DIR/pre-commit-repo-size-hook"

if ! HOOKS_DIR=$(git rev-parse --path-format=absolute --git-path hooks 2>/dev/null); then
  echo "❌ Not inside a git repository — run this from the clone to protect." >&2
  exit 1
fi
TARGET="$HOOKS_DIR/pre-commit"

[ -f "$SOURCE" ] || {
  echo "❌ Hook source not found: $SOURCE" >&2
  echo "   scripts/pre-commit-repo-size-hook is missing from this checkout." >&2
  exit 1
}

installed_is_current() {
  [ -f "$TARGET" ] || return 1
  cmp -s "$SOURCE" "$TARGET" || return 1
  [ -x "$TARGET" ] || return 1
}

case "$MODE" in
  install)
    if installed_is_current; then
      echo "✅ Pre-commit hook already installed and current: $TARGET"
      exit 0
    fi
    mkdir -p "$HOOKS_DIR"
    install -m 0755 "$SOURCE" "$TARGET"
    echo "✅ Installed pre-commit hook: $TARGET"
    echo "   Guards: files >10MB, commits >50MB total, anything staged under .beads/."
    echo "   Verify anytime with: scripts/setup-git-hooks.sh --check"
    ;;
  check)
    if installed_is_current; then
      echo "✅ Pre-commit hook installed and byte-identical to tracked source"
      exit 0
    fi
    if [ ! -f "$TARGET" ]; then
      echo "❌ No pre-commit hook installed at $TARGET" >&2
      echo "   A fresh clone has no bloat protection until you run:" >&2
      echo "     scripts/setup-git-hooks.sh" >&2
    elif [ ! -x "$TARGET" ]; then
      echo "❌ Pre-commit hook at $TARGET is not executable" >&2
    else
      echo "❌ Installed pre-commit hook has drifted from scripts/pre-commit-repo-size-hook" >&2
      echo "   Re-run scripts/setup-git-hooks.sh to overwrite it with the tracked source." >&2
    fi
    exit 1
    ;;
  uninstall)
    if [ -f "$TARGET" ]; then
      rm "$TARGET"
      echo "✅ Removed pre-commit hook: $TARGET"
      echo "   ⚠️  This clone now has no bloat protection (see bead bf-4yjq)."
    else
      echo "No pre-commit hook installed at $TARGET"
    fi
    ;;
esac
