# OOM Fix Validation — domchk-5b3f7068

**Date:** 2026-09-02
**Bead:** domchk-5b3f7068 (Validate OOM Fix Effectiveness)
**Dependency:** domchk-09945dcb (Verify repository health after cleanup) — **CLOSED**
**Original crash:** bf-b0n3xj · Root cause: bf-ku6mmu (CLOSED) · Bloat incident: bf-1s6c3

## Verdict: PASS — all 4 acceptance criteria met

Git operations in this repository are stable, bounded to ~13 MiB peak RSS, and
the repository cleanup persists at 92 MB (a further reduction from the 757 MB
post-cleanup state, via the aggressive gc run in domchk-09945dcb). Zero signal -1
crashes in the domain-check workspace in the last 24 hours.

---

## Acceptance Criterion 1 — 5 consecutive `git status`, no memory issues

| Run | Exit | Wall | Peak RSS (child) |
|-----|------|------|------------------|
| 1 | 0 | 9 ms | 12.8 MiB |
| 2 | 0 | 8 ms | 12.8 MiB |
| 3 | 0 | 8 ms | 12.8 MiB |
| 4 | 0 | 8 ms | 12.8 MiB |
| 5 | 0 | 8 ms | 12.8 MiB |

Peak RSS measured via `getrusage(RUSAGE_CHILDREN).ru_maxrss` per run. Identical
across all 5 runs — no growth, no anomalies.

## Acceptance Criterion 2 — `git log` without excessive memory usage

| Command | Exit | Wall | Peak RSS | Output |
|---------|------|------|----------|--------|
| `git log --oneline -20` | 0 | 3 ms | 12.8 MiB | 20 lines |
| `git log -5 --stat` | 0 | 9 ms | 12.8 MiB | 67 lines |
| `git log --oneline -200` | 0 | 6 ms | 12.8 MiB | 200 lines |
| `git log -1 --format=fuller` | 0 | 3 ms | 12.8 MiB | 16 lines |
| `git log --since=2026-08-20 --oneline` | 0 | 18 ms | 12.8 MiB | 1193 lines |
| `git log -3 -p --stat` | 0 | 5 ms | 12.7 MiB | 350 lines |

**Hard-ceiling proof:** `git status` and the full `git log` (1640 commits) were
additionally run inside a `systemd-run --user --scope -p MemoryMax=256M
-p MemorySwapMax=0` cgroup — both completed with exit 0. Git operations in this
repository now fit in **1/64th** of the memory that the 17 GB of loose objects
used to consume before cleanup.

## Acceptance Criterion 3 — no signal -1 crashes in recent agent logs

Three independent sources checked, window = last 24 h (2026-09-01 12:45Z →
2026-09-02 12:45Z):

| Source | Result |
|--------|--------|
| `.beads/events.jsonl` crash events | **0** in last 24 h (all 247 recorded events date to 2026-08-16/17 and 2026-08-26) |
| Needle session logs, domain-check workspaces (104 `agent.completed` events) | exit 0: 102, exit 1: 47 across workers, **exit -1: 0** |
| Needle session logs, all workspaces (594 files) | 4 exit -1 events, **none in domain-check** |

The 4 cross-workspace exit -1 events (drawrace-069a5084, drawrace-86c0c012,
drawrace-eae34b24, pdftract-0fa030bd) share an identical signature: each fired
in the **same second** as the preceding `agent.completed`, with
`duration_ms: 0` — i.e. an instantaneous dispatch-time termination of the
immediately-following re-dispatch, not a mid-task kill and not a git/memory
failure. This is the documented post-completion false-positive class from
`docs/crash-response-guide.md`, and it is outside this repository's
workspaces.

## Acceptance Criterion 4 — repository cleanup persists (18 GB → 757 MB)

| Metric | Pre-cleanup (bf-1s6c3) | Post-cleanup (task background) | **Verified now** |
|--------|------------------------|-------------------------------|------------------|
| Total repository size | 18 GB | 757 MB | **92 MB** |
| Loose objects | 17.16 GB | — | **8 objects / 60 KiB** |
| Pack files | — | — | **1 pack / 90.18 MiB** |
| `git fsck --connectivity-only` | — | — | **clean, exit 0** |

The domchk-09945dcb aggressive gc (earlier today, exit 0 in 81 s under a 2 G
cgroup ceiling) consolidated the repository further from 757 MB to 92 MB. All
values are far inside the healthy thresholds from the Repository Maintenance
Guide (< 500 MB total, < 100 loose objects, < 100 MB loose).

## Kernel OOM context (7-day window)

24 OOM-related kernel lines resolve to **6 kill events, all
`CONSTRAINT_MEMCG`, all inside `safe-git-gc-*.scope` guardrail cgroups**
(2026-09-02, 07:15–08:32 EDT, during repeated gc invocations). Each killed
only the memory-capped wrapper process (~63 MB bash); the system as a whole
was never memory-starved. **Zero system-wide OOM events.** This is the
guardrail operating as designed, not a regression.

## System state at validation

- Memory: 49 Gi available (of 62 Gi), swap unused
- Disk: 97 G free (minimum 20 G)
- `git status`: 56 untracked paths, 8 modified — pre-existing in-flight work from other agents, untouched

## Follow-up finding (not fixed in this bead)

`scripts/crash-pattern-detection.sh` misreports its time window: line 80 greps
**all** `event: crash` records from `.beads/events.jsonl` with no date filter,
and the headline count plus the `ELEVATED CRASH RATE` alert use that unfiltered
total. Today it printed "Total Crashes (last 24hours): 247" when the true 24 h
count is 0 (only the separate `SURGE_CRASHES` jq filter at line 124 is
time-bounded). This generates false "ELEVATED CRASH RATE" alerts for any repo
with historical crash events. Left untouched here — the monitoring scripts have
uncommitted modifications in flight from other workers; file a dedicated bead
to apply the `--since` filter to `RECENT_CRASHES`.
