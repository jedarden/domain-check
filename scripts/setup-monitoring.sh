#!/usr/bin/env bash
# setup-monitoring.sh — retired alias for monitoring-setup.sh (gap G-7).
#
# This used to be a second, independent cron-based installer. This box is
# NixOS and has no cron daemon, so its installs were silent no-ops. It is
# kept as an alias — docs/crashes/bf-4yjq-crash-report.md still documents
# `setup-monitoring.sh --list` — so old references land on the safe path
# instead of a dead one.
#
# All flags pass through to scripts/monitoring-setup.sh, including --list
# (mapped there to --status). See that script for the exit-code contract:
#   0 in force · 1 usage · 2 verification failed · 3 cannot verify

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "setup-monitoring.sh is retired as a cron installer (gap G-7);"
echo "delegating to monitoring-setup.sh (systemd user timers)."
echo ""

exec "$SCRIPT_DIR/monitoring-setup.sh" "$@"
