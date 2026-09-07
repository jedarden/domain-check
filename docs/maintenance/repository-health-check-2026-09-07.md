# Repository Health Check — 2026-09-07

**Verified:** 2026-09-07 01:24 EDT (05:24 UTC)
**Repository:** `/home/coding/domain-check` (branch `main`, `99c4fef` = `origin/main`, 0/0 divergence)
**Bead:** domchk-b344425d
**Method:** `./scripts/check-repo-health.sh` (exit 0) + `git count-objects -vH` + `git fsck --full` + `./scripts/setup-git-gc-config.sh --verify`
**Baselines:** [git-gc-results-2026-09-06.md](git-gc-results-2026-09-06.md), [repository-cleanup-2026-09-01.md](repository-cleanup-2026-09-01.md)

## Decision: ✅ HEALTHY — no cleanup required

Every size threshold from the CLAUDE.md bloat table passes. `safe-git-gc.sh` was
**not** run: the task's gc trigger (>1 GB repository or >500 MB loose objects)
is nowhere near firing, and the daily 03:00 incremental gc timer owns the loose
churn. Spending an operation with the memcg-OOM history this repo's gc has for
no measurable gain would be the wrong trade.

## Key Metrics (2026-09-07 01:24 EDT)

| Metric | Value | Threshold | Verdict |
|--------|-------|-----------|---------|
| **Total repository size** (`.git`) | **103 MB** | < 500 MB healthy | ✅ Healthy |
| **Loose objects size** | **1.20 MiB** | < 100 MB healthy | ✅ Healthy |
| **Loose object count** | 157 | < 100 healthy, 100–1000 pack | ⚠️ Warning band by count — normal churn, daily 03:00 gc packs it |
| **Packed objects** | 11,182 in 3 packs, 99.13 MiB | consolidated | ✅ Healthy |
| **Garbage** | 0 bytes | 0 | ✅ Healthy |
| **Size ratio (loose : packed)** | **1 : 83** | < 1 : 10 healthy | ✅ Healthy |
| `git fsck --full` | exit 0 (dangling trees only, see below) | clean | ✅ Clean |
| `check-repo-health.sh` | exit 0 | passes | ✅ Pass |
| Effective pack-memory bound | ✅ verified, worst case ≈3 GiB | within 12 GiB dispatch scope | ✅ Verified |

Loose count 157 / 1.20 MiB is ordinary inter-gc churn, not bloat drift: it was
136 / 1.04 MiB at the 2026-09-06 check and the nightly timer packs it back to 0.
Size — the metric that actually caused the August memcg-OOMs — is 1.2 MiB against
a 100 MB healthy ceiling.

## Integrity

```
$ git fsck --full        # 01:24 EDT, exit 0
dangling tree 7245ce454db3849c26d990368c1d07778b009dc7
dangling tree 1bd75c267ed7092c6e661f914be9d34aca067b7a
dangling tree 61ea467f72eb3657b395d54c12e4bc32bad7e378
```

Exit 0, no corruption, no missing objects. The three dangling trees are
unreferenced working-tree snapshots from recent commits (this is a
multi-worker checkout with frequent commits); they are pruned by the next
`--prune`-carrying gc pass. Same class of finding the 2026-09-06 pre-flight
recorded, and that run's gc pruned them without incident.

## Cleanup Performed

**None.** No gc, no repack, no prune — nothing was triggered and nothing was
needed. Effective git config bounds were re-verified rather than run:
`setup-git-gc-config.sh --verify` → ✅ windowMemory=2g (local), deltaCache=1g
(local), threads=1 (local) → worst-case pack memory ≈3072 MiB, inside the
6442450944-byte ceiling for a 12 GiB dispatch scope. Both bare `git gc` and
`git push`'s pack-objects stay bounded.

## Advisories (no action taken, recorded for the next check)

1. **Historical `dist/` binaries in the pack** — `check-repo-health.sh` flags
   five ~14.28 MB copies of `dist/domain-check_darwin_amd64_v1/domain-check`
   (a goreleaser artifact committed once). **0 tracked at HEAD today**;
   the copies live only in history and are already delta-compressed inside the
   99.13 MiB pack. They are part of why the pack is ~99 MiB rather than smaller,
   but they cannot grow and rewriting pushed history to remove them is
   forbidden here. Leave alone; do not re-run goreleaser into `dist/` and commit.
2. **`.beads/` is 4.3 GB on disk** (4.0 GB `traces/`, 306 MB `state/`) — this is
   the gitignored bead workspace, **not** repository bloat (0 tracked files,
   `.gitignore` covers the whole directory). It cannot re-create the 2026-08-12
   failure mode, but it is the largest consumer of the shared 444 G disk
   (52 G free at check time). Trace retention is a workspace-level concern,
   out of scope for a repo-health bead — flagged here so someone with
   ownership of `.beads/traces/` can decide.

## Comparison to Baselines

| Date | .git | Loose objects | Source |
|------|------|---------------|--------|
| 2026-08-12 (crisis) | ~18 GB | 17.16 GB loose, 17+ × 237 MB `.beads/*.jsonl` commits | bf-1s6c3 / bf-4yjq |
| 2026-09-01 (post-cleanup) | 92 MB | packed, 1 pack | repository-cleanup-2026-09-01.md |
| 2026-09-06 | 93–94 MB | 136 / 1.04 MiB | git-gc-results-2026-09-06.md |
| **2026-09-07 (this check)** | **103 MB** | **157 / 1.20 MiB** | this document |

Ten MB over yesterday's figure is loose-churn accumulation plus a day of
commits, not a trend — an order of magnitude under the 500 MB healthy ceiling.
Next scheduled touchpoints: daily repo-health check 02:00, incremental gc 03:00,
weekly full gc Sun 04:00 (`MemoryMax=4G`).
