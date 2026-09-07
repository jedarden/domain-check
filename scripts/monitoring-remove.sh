#!/usr/bin/env bash
# monitoring-remove.sh — retired alias for `monitoring-setup.sh --remove` (gap G-7).
#
# This used to edit the crontab directly and then point the operator back at
# the cron-based installer for reinstallation. On this NixOS box there is no
# cron daemon, so both directions were silent no-ops: nothing was removed and
# nothing could be reinstalled. Removal now goes through the same systemd-only
# path as installation — see scripts/monitoring-setup.sh for the exit-code
# contract (0 ok · 1 usage · 2 verification failed · 3 cannot verify).

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "monitoring-remove.sh is retired as a cron editor (gap G-7);"
echo "delegating to monitoring-setup.sh --remove (systemd user timers)."
echo ""

exec "$SCRIPT_DIR/monitoring-setup.sh" --remove "$@"
