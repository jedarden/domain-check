# Monitoring Stack Verification — domchk-e0e1120e (2026-09-08)

Child 1 of 5 of the `domchk-b1626933` split (parent criterion: "Monitoring system
tested and verified"). Every monitoring component was run live on this box and the
timers checked in the same session. All four monitors exited 0 and all six required
systemd timers are present with future next-trigger times.

- **Run window:** 2026-09-08T01:21–01:23Z (box local time 2026-09-07 21:21–21:23 EDT)
- **HEAD at time of run:** `77057c3` (one co-tenant commit `11bd848` also unpushed — not part of this work)
- **Raw outputs:** `.beads/state/crash-prevention-testing/*.log` — gitignored by the
  repo-wide `.beads/` rule, deliberately **not** committed

## 1. Preflight

| Check | Threshold | Observed | Verdict |
|---|---|---|---|
| Available memory | ≥ 10 GB | 52 GB | pass |
| Disk free on `/` | ≥ 20 GB | 79 GB | pass |

## 2. Monitors run once

| Script | Exit | Result |
|---|---|---|
| `./scripts/resource-monitor.sh --once` | 0 | MEMORY 53GB OK · DISK 79GB OK · CPU 2.27 OK · PRESSURE 0% OK · UNSAFE_GC none OK |
| `./scripts/service-monitor.sh --once` | 0 | Inference gateway HEALTHY (HTTP 200) · memory/disk/load OK · "PRE-FLIGHT CHECK PASSED: All services healthy" |
| `./scripts/crash-pattern-detection.sh` | 0 | "No crashes detected in the last 24hours" · System Status: STABLE |
| `./scripts/check-repo-health.sh` | 0 | `.git` 104 MB · 150 loose objects / 12174 in-pack / 100.25 MiB single pack / 0 garbage · fragmentation 1 pack · effective pack-memory bound ≈3072 MiB (within the 6 GiB ceiling for a 12 GiB dispatch scope) · no unmanaged aggressive gc running · unpushed backlog 1 (< 50 threshold) |

No unexpected errors in any of the four. Neither threshold monitor emitted a warning.

### The one ⚠️ line, explained

`check-repo-health.sh` prints:

> ⚠️ Found large files in history (>10MB): `dist/domain-check_darwin_amd64_v1/domain-check` × 5, 14.28 MB each

This is **expected and pre-existing**, not a regression:

- The five blobs are the *same* GoReleaser build artifact (`dist/…darwin_amd64_v1/domain-check`)
  from past release runs — already in git history, not in the working tree. The same check
  reports "✅ No large files found in working directory".
- The repo total is 104 MB, far under the 500 MB healthy threshold, so the historical weight
  costs nothing operationally.
- New occurrences are blocked going forward by the 10 MB pre-commit repo-size hook
  (`scripts/pre-commit-repo-size-hook`, installed and self-tested).
- Removing them would require a history rewrite (`git filter-repo` + force-push), which the
  standing rules prohibit. Accepted as inert historical weight; no action owed.

## 3. Timers

`systemctl --user list-timers 'domain-check-*' --all` reports **8 timers, all with future
next-trigger times** — the six required by this bead plus two this box additionally runs
(alert-triage, auto-gc). Schedules read from the unit files themselves, not inferred.

| Timer | Unit spec | Required cadence | Next trigger (observed) | Last fired |
|---|---|---|---|---|
| `domain-check-service-monitor.timer` | `OnCalendar=*:00/2` | every 2 min | 21:24:00 EDT (28 s ahead) | 21:22:07 EDT |
| `domain-check-resource-monitor.timer` | `OnCalendar=*:00/5` | every 5 min | 21:25:00 EDT | 21:20:06 EDT |
| `domain-check-monitoring.timer` | `OnCalendar=*:00/10` | every 10 min | 21:30:00 EDT | 21:20:06 EDT |
| `domain-check-repo-health.timer` | `OnCalendar=*-*-* 02:00:00` | daily 02:00 | Tue 2026-09-08 02:00 EDT | 2026-09-07 02:00:13 EDT |
| `domain-check-git-gc.timer` | `OnCalendar=*-*-* 03:00:00` | daily 03:00 | Tue 2026-09-08 03:00 EDT | 2026-09-07 03:00:21 EDT |
| `domain-check-git-gc-full.timer` | `OnCalendar=Sun *-*-* 04:00:00` | weekly Sun 04:00 | Sun 2026-09-13 04:00 EDT | Sun 2026-09-06 04:00:31 EDT |
| `domain-check-auto-gc.timer` *(extra)* | `OnCalendar=*-*-* 02:30:00` | — | Tue 2026-09-08 02:30 EDT | 2026-09-07 02:30:09 EDT |
| `domain-check-alert-triage.timer` *(extra)* | `OnUnitActiveSec=1h` | — | 22:02:43 EDT | 21:02:43 EDT |

All six required cadences match the specification exactly. The weekly full-gc timer's last
fire (Sun 2026-09-06) and next (Sun 2026-09-13) confirm the weekly cadence is actually
advancing, not stalled.

## 4. Conclusion

The monitoring stack is operational on this box as of 2026-09-08: all four monitors run
clean with no unexpected errors, the repo-health check's single warning is a documented
historical artifact, and all timers are installed, loaded, and scheduled with future
trigger times. This record supplies the "Monitoring system tested and verified" criterion
for parent bead `domchk-b1626933`; `docs/crash-prevention-testing.md` (child
`domchk-a61cc009`) is the consolidating deliverable and should cite this file rather than
re-run the checks.
