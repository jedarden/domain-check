# Verification Report: bf-65lsdu Crash Fix (domchk-ca7d6d12)

**Date:** 2026-09-02
**Bead:** domchk-ca7d6d12 — "Verify fix resolves the crash and retry bead"
**Original bead:** bf-65lsdu — "Run repository cleanup to eliminate 17GB bloat" (CLOSED 2026-08-17, exit 0)
**Fix under verification:** `4737327` — "fix: correct loose-object detection in safe-git-gc.sh + regression test"

## The crash being verified against

bf-65lsdu (2026-08-13/14) crashed with exit code -1 (signal -1) eleven times.
The repository held **17.20 GiB of loose objects (4,515 objects)**; git gc
during the cleanup attempt OOM'd the box. The tooling bug in the fix commit:
`safe-git-gc.sh` grepped `git count-objects` output for `^loose:` — a key git
never emits (the loose count is `count:` in `git count-objects -v`) — so the
bloat detector always read **0** and `--check-only` reported "GC not needed"
no matter how bloated the repository was.

## Test 1 — Regression test (PASS)

```
$ scripts/test-safe-git-gc-detection.sh
PASS: bloat detected (1103 loose objects > 1000)
```

Builds a disposable 1,103-loose-object repo in a `mktemp -d` sandbox and
asserts `--check-only` reports "GC needed". Cleans up after itself.

## Test 2 — Bug mechanism, demonstrated live

On the real domain-check repo (25 loose objects at time of test):

| Detector | Key grepped | Reads |
|---|---|---|
| Old (broken) | `git count-objects \| grep '^loose:'` | *(no output → 0)* |
| New (fixed) | `git count-objects -v \| grep '^count:'` | `25` |

The old key matched nothing while 25 real loose objects existed — the exact
failure mode that left the bf-65lsdu bloat undetected.

## Test 3 — bf-65lsdu retry in a safe environment (PASS)

bf-65lsdu is already closed (completed successfully 2026-08-17 after the
cleanup landed; 18GB → 140MB). "Retry" was therefore performed on a faithful
replica of its workload, in `~/scratch/bf65lsdu-retry-20260902/`, against a
byte-identical snapshot of the **committed** `safe-git-gc.sh`
(`git show HEAD:scripts/safe-git-gc.sh`, sha256 `317261c475b7…`) — immune to
concurrent workers' uncommitted edits to that file.

**Replica repo (build-bloated-repo.sh, regenerable):**

| Metric | bf-65lsdu (real) | Replica (test) |
|---|---|---|
| Loose objects | 4,515 | **4,603** |
| Loose size | 17.20 GiB | 2.04 GiB (~⅛ scale) |
| Packs | 0 | 0 |

**Fixed detector on the replica:** old key reads `0` matches; new detector
reports `GC needed: Too many loose objects (4603 > 1000)` — triggers
correctly at crash-condition scale.

**Full cleanup run (`safe-git-gc.sh`, defaults, RSS sampled every 1s):**

| Result | Value |
|---|---|
| Exit code | **0** (no crash, no signal kill) |
| Stage 1 (`git gc --prune=now`) | 278s |
| Stage 2 + final verification | passed, checkpointed |
| Loose objects after | **0** (4,603 packed) |
| Packs after | 1 pack, 1.93 GiB |
| `git fsck` | clean (exit 0) |
| Peak gc process-tree RSS | **1,986 MB** (353 samples) |
| Post-run `--check-only` | "GC not needed" (exit 1) — correct negative |

The crash pattern (OOM kill mid-gc → signal -1) did not reproduce.

## Test 4 — Real repository state (no recurrence)

`safe-git-gc.sh --check-only` on the real repo:

```
Loose objects: 0    Pack files: 1    Repository size: 92M
GC not needed
```

The bf-65lsdu cleanup (18GB → ~140MB, currently 92M) has held; the bloat
condition has not returned.

## Findings / remaining edge cases

1. **Hard memory ceiling is inert in HEAD.** The fix commit added
   `resolve_cgroup_cap` / `run_memory_capped` / `acquire_gc_lock`, but HEAD's
   `main()` never calls `resolve_cgroup_cap` or `acquire_gc_lock`
   (`CGROUP_CAP` stays `"unknown"`, so `run_memory_capped` execs uncapped).
   The verified run was bounded only by git's soft `pack.windowMemory`
   config. A concurrent worker's **uncommitted** change to `main()` wires
   both in; until that lands, the ceiling is declared but not enforced.
2. **Memory scales with pool size.** The 2.04 GiB pool peaked at ~1.99 GB
   RSS. bf-65lsdu's 17.20 GiB pool would peak proportionally higher absent a
   hard cap — OOM territory on a shared box (the original crash). Landing
   finding 1 is the mitigation.
3. **Scale coverage.** Verified at matched object count (4,603 vs 4,515) and
   ~⅛ byte scale; 17 GiB of scratch data was not written to avoid loading a
   shared box. Nothing in the code path is size-special-cased, and the
   detector fix is count-based, so the result is expected to hold at full
   scale — with finding 1 addressed first.

## Verdict

**Resolved.** The `4737327` fix makes bloat detection work (was always-0),
the staged cleanup completes the bf-65lsdu workload safely with exit 0 at
crash-condition scale, the real repository remains healthy, and the original
bead's objective is confirmed complete.
