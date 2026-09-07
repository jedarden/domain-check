#!/usr/bin/env bash
# Install / remove / inspect the hourly alert-triage systemd user timer.
#
# Standalone on purpose: scripts/setup-repo-maintenance.sh installs the
# repo-health and git-gc timers, but it has carried another bead's uncommitted
# edits since before 2026-09-07, so wiring the triage timer through it would
# have meant committing someone else's in-flight work. Fold it in there once
# that working tree is clean.
#
# Usage:
#   setup-alert-triage-timer.sh            install or refresh + enable --now
#   setup-alert-triage-timer.sh --status   show the timer and its next trigger
#   setup-alert-triage-timer.sh --remove   stop, disable, and uninstall
#
# Exit codes: 0 installed/ok, 1 something did not verify, 2 usage.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
UNIT_SRC_SERVICE="$SCRIPT_DIR/domain-check-alert-triage.service"
UNIT_SRC_TIMER="$SCRIPT_DIR/domain-check-alert-triage.timer"
UNIT_DIR="${XDG_CONFIG_HOME:-$HOME/.config}/systemd/user"
SERVICE_NAME="domain-check-alert-triage.service"
TIMER_NAME="domain-check-alert-triage.timer"

usage() { sed -n '2,14p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; }

install_timer() {
    [[ -f "$UNIT_SRC_SERVICE" && -f "$UNIT_SRC_TIMER" ]] || {
        echo "ERROR: unit files not found next to this script ($UNIT_SRC_SERVICE, $UNIT_SRC_TIMER)" >&2
        return 1
    }

    mkdir -p "$UNIT_DIR"
    cp "$UNIT_SRC_SERVICE" "$UNIT_SRC_TIMER" "$UNIT_DIR/"

    # CLAUDE.md gotcha (2026-09-02): a stale unit state after editing a unit
    # file makes the timer silently never fire — daemon-reload is not optional.
    systemctl --user daemon-reload
    systemctl --user enable --now "$TIMER_NAME"

    verify_timer
}

verify_timer() {
    local ok=0
    systemctl --user is-active --quiet "$TIMER_NAME" || { echo "FAIL: $TIMER_NAME not active"; ok=1; }
    systemctl --user is-enabled --quiet "$TIMER_NAME" || { echo "FAIL: $TIMER_NAME not enabled"; ok=1; }

    # CLAUDE.md convention: a timer is only real if list-timers shows it.
    # A missing entry usually means the unit state is stale (daemon-reload
    # skipped) or the unit file failed to parse.
    if ! systemctl --user list-timers "$TIMER_NAME" --no-pager 2>/dev/null | grep -q "$TIMER_NAME"; then
        echo "FAIL: $TIMER_NAME has no scheduled trigger (daemon-reload skipped?)"
        ok=1
    fi

    if [[ "$ok" -eq 0 ]]; then
        echo "OK: $TIMER_NAME installed, enabled, and scheduled"
        echo "    unit files:  $UNIT_DIR/$SERVICE_NAME, $UNIT_DIR/$TIMER_NAME"
        echo "    sweep log:   /home/coding/domain-check/.beads/logs/alert-triage.log"
        echo "    queue:       /home/coding/domain-check/.beads/state/alert-triage/queue.jsonl"
    fi
    return "$ok"
}

show_status() {
    systemctl --user status "$TIMER_NAME" --no-pager -l || true
    echo
    systemctl --user list-timers "$TIMER_NAME" --all --no-pager || true
}

remove_timer() {
    systemctl --user disable --now "$TIMER_NAME" 2>/dev/null || true
    rm -f "$UNIT_DIR/$SERVICE_NAME" "$UNIT_DIR/$TIMER_NAME"
    systemctl --user daemon-reload
    systemctl --user reset-failed "$TIMER_NAME" 2>/dev/null || true
    echo "Removed $TIMER_NAME (queue and log under .beads/ left in place)"
}

case "${1:-}" in
    "") install_timer ;;
    --status) show_status ;;
    --remove) remove_timer ;;
    -h|--help) usage ;;
    *) echo "ERROR: unknown option: $1" >&2; usage; exit 2 ;;
esac
