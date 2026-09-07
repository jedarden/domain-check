# Git GC OOM Crash Analysis — Bead bf-173o7e

**Document Date:** 2026-09-02
**Bead ID:** bf-173o7e
**Agent:** claude-code-glm-4.7 (provider: zai)
**Crash Timestamp:** 2026-08-14T13:10:47Z
**Exit Code:** -1 (death by signal — SIGKILL, likely OOM killer)
**Classification:** Infrastructure event (memory pressure), not a code defect
**Task Outcome:** ✅ Operation successfully completed on retry

## Executive Summary

An agent executing the task bead bf-173o7e — `git gc --aggressive --prune=now` against a
repository carrying **17.20 GB of loose objects** — was terminated with exit code -1
(SIGKILL). The kill signature, the memory profile of aggressive repacking at that object
volume, and the absence of a core dump together point to the kernel OOM killer as the
terminating agent. No domain-check code was involved or at fault.

The operation was **successfully retried and completed** on 2026-08-17: aggressive gc
finished in ~6 minutes at a peak of 864 MB–1.3 GB RSS, producing a healthy 444.24 MiB
pack, with repository integrity verified by `git fsck` and zero data loss. The bead was
closed as completed at 2026-08-17 17:15.

> **Disambiguation — two terminations are on record for this bead.** The **OOM SIGKILL
> (exit -1)** analyzed here occurred on **2026-08-14**, during the gc itself, while the
> repository still held ~17 GB of loose objects. A *separate*, later recording
> (2026-08-17T17:06Z, exit code 1, `error_max_turns`) captured a post-completion workflow
> failure during bead closing — **not** the OOM event. That second event is analyzed in
> [docs/crash-investigation-bf-173o7e-definitive-2026-08-25.md](../crash-investigation-bf-173o7e-definitive-2026-08-25.md).
> Conflating the two misattributes the root cause; both must be read together.

## Crash Event

### Task Being Executed

```bash
git gc --aggressive --prune=now
```

| Parameter | Value |
|-----------|-------|
| Command | `git gc --aggressive --prune=now` |
| Exit code | -1 (terminated by signal; SIGKILL leaves no core dump) |
| Expected duration | 2–6 hours at 17 GB loose object scale |
| Actual duration before kill | < 2.5 hours (crash at 2026-08-14T13:10:47Z) |
| Host | Lab box — Dell OptiPlex 3000 Micro, 12 cores / 62 GB RAM / 444 GB disk, **no swap** |

### Exit Code -1 Interpretation

On Unix, exit code -1 indicates the process died by signal rather than exiting normally:

- **SIGKILL (9)** — leaves no core dump, no in-process trace. **Most consistent with the observed evidence.** The usual sender at this memory scale is the kernel OOM killer.
- SIGSEGV (11) — would normally leave a core dump; none was found.
- SIGHUP (1) — no hangup source applies to this context.

System logs from the crash window were lost to log rotation, so the OOM verdict rests on
the signature match rather than a direct `oom-kill` log line. It is recorded as
"likely OOM killer" with that caveat.

## The 17.20 GB Loose Object Context

The repository state at crash time was the direct precondition for the kill:

```
Loose objects:        17.20 GB across 4,515 objects
Loose:packed ratio:   ~95.7% of total repository size was loose
```

- Source: `docs/cleanup-resolution-2026-08-17.md`; also captured in
  `docs/crash-artifacts-bf-4yjq-raw.md` (`size: 17.20 GiB (loose objects)`).
- The accumulation fed back on itself: the memory-intensive gc attempts that crashed were
  themselves large git operations, and interrupted/retried repacks added further loose
  objects before cleanup.

### Why This Volume Kills Aggressive GC

`git gc --aggressive` (via `git repack -a -d -f --depth=250 --window=250`) forces full
re-delta-computation across **every** object in the repository:

- `--window=250` compares each object against up to 250 candidates; the delta search
  working set scales with total object count and average object size.
- With 17.20 GB of object content in the delta window's reach, peak RSS was estimated at
  **10–20 GB**, versus ~100–500 MB for a standard `git gc` on the same repo.
- **The box has no swap.** With no swap, memory pressure converts directly into OOM
  kills rather than thrashing — there is no soft landing.
- Aggressive mode additionally disables the incremental-repack fast paths that would
  otherwise let a previously packed repository repack only new objects.

The 62 GB of RAM was sufficient in isolation, but the operation ran unbounded alongside
the rest of the box's workload (agent fleet, other repos' builds) with no cgroup memory
cap — exactly the condition under which the OOM killer selects the largest RSS process,
which during a repack is `git pack-objects`.

## Root Cause

**Root cause:** OOM killer termination (SIGKILL → exit code -1) of a memory-unbounded
`git gc --aggressive --prune=now` operating over 17.20 GB of loose objects on a no-swap
host.

**Contributing factors:**

1. **Unbounded memory** — no `memory.limit_in_bytes`/cgroup cap on the gc process.
2. **No swap** — OOM instead of degradation.
3. **Wrong tool tier for the state** — `--aggressive` invoked directly on a maximally
   bloated repository, rather than a staged standard-gc-first approach.
4. **Silent failure mode** — SIGKILL produces no core dump and (after log rotation) no
   kernel log, which is why this event was initially ambiguous.

**Ruled out:**

| Alternative | Verdict | Reason |
|-------------|---------|--------|
| Timeout kill | Ruled out | No timeout configured on bead execution |
| Manual operator SIGKILL | Ruled out | No concurrent operator activity logged |
| Disk exhaustion | Ruled out | 444 GB disk was not full |
| Repository corruption | Ruled out | `git fsck --full` clean after recovery; git repack is transactional |

## Successful Retry — Resolution

The bead system retried the task; the retry **succeeded** (evidence:
`docs/crash-investigation-bf-173o7e-definitive-2026-08-25.md`,
`docs/research/crash-pattern-analysis-bf-173o7e.md`):

### Timeline

| When (UTC) | Event |
|------------|-------|
| 2026-08-14 13:10 | **Crash** — SIGKILL during gc on 17.20 GB loose-object repo |
| 2026-08-17 12:38 | Retry gc completes — pack file created (~6 min runtime) |
| 2026-08-17 17:06 | Later recording captures `error_max_turns` during bead close (separate, post-completion event) |
| 2026-08-17 17:15 | Bead bf-173o7e closed as completed |

### Retry Results

```
Runtime:            ~6 minutes
Peak memory:        864 MB – 1.3 GB RSS (well within limits; ~1.1 GB peak)
CPU:                96–97% during repacking
Objects packed:     7,753
Pack file:          444.24 MiB, compressed
Repository (.git):  548 MB
Integrity:          git fsck clean — no corruption, no data loss
```

The retry succeeded at ~1/10th the estimated memory of the original attempt because the
repository had been partially cleaned between attempts (loose object mass reduced before
the successful repack — see `docs/cleanup-resolution-2026-08-17.md`). This is itself the
key operational lesson: **shrink first, then aggressive-repack.**

Verified again 2026-08-26: 7,765 packed objects in a single 445 MB pack, zero loose
objects, `git fsck --full` passed. Repository is healthy (91 MB `.git` with 9,404
objects as of the 2026-09-01 baseline in `docs/git-gc-baseline.txt`).

## Lessons Learned

1. **OOM is the silent kill.** Exit code -1 with no core dump and rotated-away kernel
   logs is the OOM signature on this box; treat that combination as OOM until proven
   otherwise.
2. **`git gc --aggressive` is not automation-safe at scale.** It is a workstation
   maintenance tool. In unbounded automation, memory scales with repository bloat — the
   exact condition that most needs the cleanup.
3. **Staged repacking beats one-shot aggressive gc:** standard `git gc` first, aggressive
   only afterwards on an already-packed repository.
4. **Git is transactional.** The SIGKILL left the repository fully consistent; retries
   are safe and were the correct recovery.
5. **Exit-code triage matters.** The same bead accrued both an OOM kill (exit -1) and an
   unrelated max_turns workflow failure (exit 1). Classification by exit code prevented
   misattributing either.

## Preventive Measures (Since Implemented)

The findings from this crash drove the repo's current maintenance posture:

- **`scripts/safe-git-gc.sh`** — staged gc (standard stages before aggressive), memory
  cap via `SAFE_GC_MEMORY_MAX`, checkpoint/resume, pre-flight integrity checks. Bare
  `git gc --aggressive` is banned in favor of this script (repo CLAUDE.md,
  "Git Operations Safety").
- **Resource pre-flight limits** — documented minimums (20 GB avail mem, 50 GB disk) and
  abort thresholds in the repo CLAUDE.md "Resource Limits" table.
- **Repository bloat monitoring** — `scripts/check-repo-health.sh`,
  `scripts/repo-health-monitor.sh`, and systemd user timers (daily repo-health 02:00,
  incremental gc 03:00, weekly full gc Sun 04:00 with `MemoryMax=4G`), with
  size/loose-object warning thresholds (bloat > 1 GB total or > 500 MB loose = act).
- **Bloat-prevention hygiene** — `.beads/` gitignored, pre-commit hook blocking files
  > 10 MB, after the bf-1s6c3 incident (18 GB repo → OOM) reproduced this same failure
  class.

## References

- `docs/crash-investigation-bf-173o7e-definitive-2026-08-25.md` — definitive trace-level
  investigation; retry success numbers and the max_turns disambiguation
- `docs/research/crash-pattern-analysis-bf-173o7e.md` — 2026-08-26 pattern analysis of
  this same event (exit -1, OOM verdict, recovery timeline)
- `docs/cleanup-resolution-2026-08-17.md` — 17.20 GB loose-object cleanup record
- `docs/investigation-summary-bf-173o7e-2026-09-01.md`,
  `docs/root-cause-analysis-bf-173o7e-2026-09-01.md` — later consolidation passes
- `docs/git-gc-mitigation-strategy.md`, `docs/safe-git-gc-implementation.md` —
  mitigation design
- `docs/git-gc-baseline.txt`, `docs/research/git-gc-results-2026-08-26.md` — repository
  health baselines after recovery
