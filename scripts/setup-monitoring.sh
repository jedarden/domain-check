#!/usr/bin/env bash
# Setup Automated Monitoring for Crash Prevention
# Configures cron jobs for automated repository health monitoring
#
# Usage: ./scripts/setup-monitoring.sh [options]
#   --remove        Remove monitoring cron jobs
#   --dry-run       Show what would be done without making changes
#   --list          List current monitoring cron jobs

set -euo pipefail

DRY_RUN=false
REMOVE=false
LIST_ONLY=false

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# Parse arguments
while [[ $# -gt 0 ]]; do
  case $1 in
    --remove)
      REMOVE=true
      shift
      ;;
    --dry-run)
      DRY_RUN=true
      shift
      ;;
    --list)
      LIST_ONLY=true
      shift
      ;;
    -h|--help)
      cat <<EOF
Usage: $0 [options]

Setup automated monitoring cron jobs for crash prevention.

Options:
  --remove        Remove monitoring cron jobs
  --dry-run       Show what would be done without making changes
  --list          List current monitoring cron jobs
  -h, --help      Show this help message

Monitoring jobs:
  - Repository health check (daily at 2am)
  - Crash pattern detection (every 6 hours)
