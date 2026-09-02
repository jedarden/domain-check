# Cgroup Memory Guard — closing the last unchecked kill domain

**Date:** 2026-09-02
**Task:** domchk-c67caf80 (implement preventive fix for signal -1 crashes)
**Builds on:** `docs/research/root-cause-analysis-signal-minus-one-crashes.md` (domchk-19a78c54), bf-4x12ec / bf-173o7e investigations

---

## The gap

Every memory safeguard that existed before this change measured the wrong
domain for the observed crash mechanism:

| Existing check | Domain measured | Blind to |
|---|---|---|
| `free -g`, `resource-monitor.sh`, `memory-pressure-monitor.sh` | system-wide RAM / PSI | a cgroup hitting **its own** limit while the box has gigabytes free |
| `safe-git-gc.sh` cgroup ceiling | caps the gc child it spawns | the **calling scope's** own pressure from any other workload |
| `preflight-health-check.sh` (before 2026-09-02) | system-wide resources | ditto |
| systemd unit `MemoryMax` | sets the limit | never *read back* before work starts |

The established root cause of the signal -1 crashes is a **memcg OOM kill**
against a cgroup limit: a worker runs in
`user@1001.service/needle.slice/run-*.scope` with `MemoryMax=12G`, and the
kernel kills when the tightest bounded cgroup in the ancestry can no longer
charge memory. `needle.slice` (32G) is shared by every concurrent worker, so
its `memory.current` includes all of them.

**Live evidence at implementation time (2026-09-02):**

```
run-p3226640-….scope   max=12G   current=0.18G  ( 1%)  headroom=11.8G
needle.slice           max=32G   current=24.7G  (74%)  headroom= 7.6G  ← binding level
user@1001.service      max=max   current=28.0G         oom_kill 6  (lifetime)
```

At that moment the box had tens of GB available system-wide and every existing
check would have passed — yet `needle.slice` was past its `memory.high` and
carrying the fleet's aggregate usage. That is precisely the state that
preceded the memcg kills in the Aug-14 storm. Only one thing was ever going to
terminate this worker early, and it wasn't system memory: it was the cgroup
tree, which nothing was reading.

## The fix

`scripts/cgroup-memory-guard.sh`:

1. Reads the caller's own ancestry from `/proc/self/cgroup` → cgroupfs
   (`memory.max` / `memory.current` / `memory.events` per level).
2. Evaluates **every bounded level**. The decision input is the *minimum
   headroom* across levels, because an ancestor's `current` includes sibling
   scopes — a nearly-full slice OOM-kills this scope even when the scope
   itself is empty. The report names the **binding level** so the operator can
   see whether the constraint is local (this task) or fleet-wide
   (too many concurrent workers).
3. Falls back to system-wide `MemAvailable` only when no bounded level exists
   (e.g. running outside any limited scope).
4. Surfaces lifetime `oom_kill` counts per level — nonzero counts are direct
   evidence the mechanism has already fired in that subtree.

Thresholds (env-overridable): refuse when any bounded level has < 2G headroom
or the leaf scope is ≥85% of its own limit; warn below 4G headroom or ≥70%
leaf usage. 2G matches `SAFE_GC_MEMORY_MAX` — the largest single allocation
this workspace knowingly makes.

### Modes

- `--check` — report + exit code (`0` pass / `1` warn / `2` refuse / `3` unknown)
- `--json` — machine-readable, includes the per-level array
- `<command>` — gate: refuse (exit 2, command **not run**) or `exec` the command
- `--strict` — unknown cgroup state refuses instead of failing open

### Integration

- **`preflight-health-check.sh` Check 3** runs the guard before every task; a
  refuse fails the pre-flight (with "wait for workers to drain" guidance), a
  warn passes with a caution note, unknown state fails open so the pre-flight
  itself never blocks work on an unreadable cgroupfs.
- Wrap any memory-heavy ad-hoc command in a dispatch scope:
  `./scripts/cgroup-memory-guard.sh -- <heavy command>`

## Verification

`scripts/test-cgroup-memory-guard.sh` — 12 checks, all passing:

- Fixture trees (no root needed; `MEMGUARD_CGROUP_ROOT` / `MEMGUARD_PROC_CGROUP`
  inject the tree): pass / warn / refuse bands, the slice-nearly-full-while-
  scope-empty case (the key one), unbounded fallback, unknown-state fail-open
  and `--strict`, `oom_kill` telemetry, wrapper gating (command blocked on
  refuse, runs and propagates exit on pass)
- Live: guard detects the real 12G scope + 32G `needle.slice` caps, and inside
  `systemd-run --user --scope -p MemoryMax=64M` the guard **refuses** — i.e.
  the exact mechanism that killed bf-4x12ec's scope now trips a gate instead
  of an unexplained SIGKILL.

Also fixed while integrating: `preflight-health-check.sh` died under
`set -e` at its first counter increment (`((X++))` on a zero counter returns
status 1), so it had never been able to emit more than one check — see
`git show HEAD:scripts/preflight-health-check.sh` for the pre-fix behavior.

## What this does NOT do

- It does not *raise* any limit (that is a NEEDLE/systemd-side change outside
  this repo) and does not cap the command it runs (use `safe-git-gc.sh` for
  gc, which already applies its own ceiling).
- It cannot prevent a kill that has already been decided by the kernel; it
  prevents *starting* work into a state where that kill becomes likely, and it
  converts the ones that still happen from "mysterious exit -1" into a
  pre-flight failure with the binding cgroup named.
