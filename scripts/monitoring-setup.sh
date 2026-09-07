#!/usr/bin/env bash
# Domain-check monitoring setup — systemd-only shim (closes gap G-7).
#
# HISTORY / WHY THIS IS A SHIM:
#   Through 2026-09-06 this script installed monitoring by writing crontab
#   entries. This box is NixOS and has no cron daemon, so every "successful"
#   run was a silent no-op: the script exited 0, nothing was scheduled, and
#   monitoring never actually came back after a removal. That failure mode is
#   gap G-7 in docs/crash-prevention-requirements.md — the bf-1s6c3 class of
#   crash (repository bloat) is exactly what uninstalled monitoring fails to
#   catch. The cron path is retired. This script now delegates to the tracked
#   systemd user-timer installers and VERIFIES the result before reporting
#   success, so it cannot no-op silently again.
#
# WHAT IT DELEGATES TO (the tracked, supported install path):
#   monitoring timers   -> scripts/install-monitoring.sh
#                          (crash-pattern, resource, service, repo-health)
#   maintenance timers  -> scripts/setup-repo-maintenance.sh
#                          (repo-health, daily gc, weekly full gc)
#   removal             -> scripts/remove-monitoring.sh
#                       + scripts/setup-repo-maintenance.sh --remove
#
# Usage:
#   monitoring-setup.sh              # ensure installed, then verify (no churn if in force)
#   monitoring-setup.sh --status     # read-only verification (alias: --list)
#   monitoring-setup.sh --remove     # delegate to the systemd removers
#   monitoring-setup.sh --dry-run    # print what would run; change nothing
#   monitoring-setup.sh --help
#
# Exit codes:
#   0  monitoring in force (or --dry-run / --help completed)
#   1  usage error
#   2  verification failed — at least one expected timer is not enabled+active
#   3  cannot verify — no systemd user session answered
#
# Test seams (used only by scripts/test-monitoring-setup-shim.sh; unset in
# real use): DOMCHECK_SYSTEMCTL substitutes the systemctl binary, and
# DOMCHECK_INSTALL_MONITORING / DOMCHECK_INSTALL_MAINTENANCE /
# DOMCHECK_REMOVE_MONITORING substitute the three delegation targets.

set -uo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CTL="${DOMCHECK_SYSTEMCTL:-systemctl}"

# The six timers CLAUDE.md documents as the installed monitoring set.
MONITORING_TIMERS=(
  domain-check-monitoring.timer        # crash-pattern detection, every 10 min
  domain-check-resource-monitor.timer  # resource thresholds, every 5 min
  domain-check-service-monitor.timer   # gateway/service checks, every 2 min
)
MAINTENANCE_TIMERS=(
  domain-check-repo-health.timer       # daily 02:00 repo health + auto-gc check
  domain-check-git-gc.timer            # daily 03:00 incremental gc
  domain-check-git-gc-full.timer       # weekly Sun 04:00 full gc (MemoryMax=4G)
)
ALL_TIMERS=("${MONITORING_TIMERS[@]}" "${MAINTENANCE_TIMERS[@]}")

INSTALL_MONITORING="${DOMCHECK_INSTALL_MONITORING:-$SCRIPT_DIR/install-monitoring.sh}"
INSTALL_MAINTENANCE="${DOMCHECK_INSTALL_MAINTENANCE:-$SCRIPT_DIR/setup-repo-maintenance.sh}"
REMOVE_MONITORING="${DOMCHECK_REMOVE_MONITORING:-$SCRIPT_DIR/remove-monitoring.sh}"

usage() {
  # Print the leading comment block (everything after the shebang).
  awk 'NR==1 {next} /^#/ {sub(/^# ?/, ""); print; next} {exit}' "${BASH_SOURCE[0]}"
}

# systemd_available: 0 = the user manager answered, 1 = it did not
systemd_available() {
  "$CTL" --user list-timers --no-legend >/dev/null 2>&1
}

# timer_ok <timer>: 0 = enabled AND active (i.e. installed and firing)
timer_ok() {
  local t="$1"
  "$CTL" --user is-enabled "$t" >/dev/null 2>&1 || return 1
  "$CTL" --user is-active "$t" >/dev/null 2>&1 || return 1
  return 0
}

# not_ok_in: echoes each timer argument that is not enabled+active
not_ok_in() {
  local t
  for t in "$@"; do
    timer_ok "$t" || echo "$t"
  done
}

# verify_timers: print a per-timer verdict table; exit 2 if any is not in force
verify_timers() {
  local bad=0 t state
  echo "Timer verification (systemd user session):"
  for t in "${ALL_TIMERS[@]}"; do
    if timer_ok "$t"; then
      state="enabled + active"
    elif "$CTL" --user is-enabled "$t" >/dev/null 2>&1 \
      || "$CTL" --user is-active "$t" >/dev/null 2>&1; then
      state="PARTIAL (enabled or active, not both)"
      bad=$((bad + 1))
    else
      state="NOT INSTALLED"
      bad=$((bad + 1))
    fi
    printf '  %-42s %s\n' "$t" "$state"
  done
  if [[ $bad -gt 0 ]]; then
    echo "❌ $bad of ${#ALL_TIMERS[@]} timers not in force"
    echo "   Install with: $0        (this shim) — or directly:"
    echo "     $INSTALL_MONITORING"
    echo "     $INSTALL_MAINTENANCE"
    return 2
  fi
  echo "✅ All ${#ALL_TIMERS[@]} timers in force"
  echo "   Next triggers:"
  "$CTL" --user list-timers 'domain-check-*' --all 2>/dev/null | sed 's/^/   /' || true
  return 0
}

MODE="ensure"
DRY_RUN=false

while [[ $# -gt 0 ]]; do
  case "$1" in
    --status|--list) MODE="status" ;;
    --remove)        MODE="remove" ;;
    --dry-run)       DRY_RUN=true ;;
    --help|-h)       usage; exit 0 ;;
    *) echo "monitoring-setup.sh: unknown option: $1" >&2; usage >&2; exit 1 ;;
  esac
  shift
done

echo "=== Domain Check Monitoring Setup (systemd-only shim) ==="
echo "Repository: $(cd "$SCRIPT_DIR/.." && pwd)"
echo "The cron-based installer this script used to be is retired (gap G-7):"
echo "this box has no cron daemon, so it now drives the systemd user timers."
echo ""

if [[ "$MODE" == "status" ]]; then
  if ! systemd_available; then
    echo "❌ Cannot verify: no systemd user session answered ('$CTL --user')."
    echo "   Check manually: systemctl --user list-timers 'domain-check-*' --all"
    exit 3
  fi
  verify_timers
  exit $?
fi

if [[ "$MODE" == "remove" ]]; then
  if [[ "$DRY_RUN" == true ]]; then
    echo "Dry run — would run:"
    echo "  $REMOVE_MONITORING"
    echo "  $INSTALL_MAINTENANCE --remove"
    exit 0
  fi
  rc=0
  echo "Removing monitoring timers via $REMOVE_MONITORING ..."
  "$REMOVE_MONITORING" || rc=1
  echo "Removing maintenance timers via $INSTALL_MAINTENANCE --remove ..."
  "$INSTALL_MAINTENANCE" --remove || rc=1
  if [[ $rc -eq 0 ]]; then
    echo ""
    echo "✅ Monitoring removal delegated to the systemd removers."
    echo "   To reinstall later, run: $0"
  else
    echo "❌ One or more removers failed (see above)." >&2
  fi
  exit $rc
fi

# MODE == ensure
if ! systemd_available; then
  echo "❌ Cannot verify or install: no systemd user session answered ('$CTL --user')."
  echo "   This is NOT a silent no-op — nothing was installed."
  echo "   Run the installers by hand once a user session is available:"
  echo "     $INSTALL_MONITORING"
  echo "     $INSTALL_MAINTENANCE"
  exit 3
fi

NEED_MONITORING=($(not_ok_in "${MONITORING_TIMERS[@]}"))
NEED_MAINTENANCE=($(not_ok_in "${MAINTENANCE_TIMERS[@]}"))

if [[ ${#NEED_MONITORING[@]} -eq 0 && ${#NEED_MAINTENANCE[@]} -eq 0 ]]; then
  echo "Monitoring is already in force — nothing to install."
  echo ""
  verify_timers
  exit $?
fi

if [[ "$DRY_RUN" == true ]]; then
  echo "Dry run — would run:"
  if [[ ${#NEED_MONITORING[@]} -gt 0 ]]; then
    echo "  $INSTALL_MONITORING        # for: ${NEED_MONITORING[*]}"
  fi
  if [[ ${#NEED_MAINTENANCE[@]} -gt 0 ]]; then
    echo "  $INSTALL_MAINTENANCE   # for: ${NEED_MAINTENANCE[*]}"
  fi
  echo "  …then re-verify all ${#ALL_TIMERS[@]} timers (exit 2 if any is still not in force)"
  exit 0
fi

rc=0
if [[ ${#NEED_MONITORING[@]} -gt 0 ]]; then
  echo "Installing monitoring timers via $INSTALL_MONITORING ..."
  echo "   missing: ${NEED_MONITORING[*]}"
  "$INSTALL_MONITORING" || rc=1
fi
if [[ ${#NEED_MAINTENANCE[@]} -gt 0 ]]; then
  echo "Installing maintenance timers via $INSTALL_MAINTENANCE ..."
  echo "   missing: ${NEED_MAINTENANCE[*]}"
  "$INSTALL_MAINTENANCE" || rc=1
fi

if [[ $rc -ne 0 ]]; then
  echo "❌ An installer exited non-zero (see above); verifying what actually landed." >&2
fi

echo ""
if ! systemd_available; then
  echo "❌ Cannot verify the result: no systemd user session answered." >&2
  echo "   Check manually: systemctl --user list-timers 'domain-check-*' --all" >&2
  exit 3
fi

verify_timers || rc=2
exit $rc
