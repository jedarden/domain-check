# Git GC Baseline Metrics

**As of:** 2026-09-06 12:42 EDT (16:42 UTC)
**Repository:** domain-check (`/home/coding/domain-check`, branch `main` at `e0dfa61`)
**Bead:** domchk-825c89bc (final git gc results + maintenance records)
**Status:** Post-gc baseline. This is the rolling baseline the maintenance
runbook refers to when it says "compare against baseline"; dated snapshots it
supersedes are listed under [History](#history).

> This file is the *current* baseline. Re-measure and update it after every
> explicit gc run or scheduled full gc; leave the dated snapshots alone so the
> trend stays auditable.

## Verified Run This Baseline Reflects

`./scripts/safe-git-gc.sh` — mode `standard`, 2026-09-06 11:29:38–11:29:44 EDT
(~6 s wall, peak RSS 165.7 MB against a 2g ceiling), checkpoint `complete`.
Verified by [git-gc-results-2026-09-06.md](git-gc-results-2026-09-06.md); the
figures below were re-measured ~73 minutes later to confirm the state is
holding under concurrent fleet load.

## Repository Overview

| Metric | Value |
|--------|-------|
| Commits (reachable from all refs) | 2,448 |
| Commits (reachable from `HEAD`) | 1,726 |
| Refs | 455 (branches + remotes + 400+ `v*-test` tags) |
| Total objects | 10,955 |
| Tracked files (working tree) | 2,639 |
| Working-tree checkout size | 2.82 MiB |

## .git Directory Size

- **Total `.git` size:** 93M (integrity-checked, healthy band)

## Object Storage Details

### Loose Objects
- **Count:** 49 objects
- **Size:** 324 KiB
- All reachable; re-accumulated from concurrent fleet commits in the window
  after the 11:29 gc. Normal churn, not a failed cleanup.

### Pack Files
- **Number of packs:** 1 (fully consolidated)
- **Packed objects:** 10,906
- **Pack efficiency:** 99.6% of all objects are packed
- **Pack payload:** 94,856,353 bytes = 90.46 MiB
- **`size-pack` (payload + `.idx`/`.rev`):** 90.75 MiB

### Garbage and Prunable Objects
- **Garbage objects:** 0
- **Prune-packable objects:** 0
- **Dangling (`git fsck --full`):** 2 trees
- **Unreachable (`git fsck --unreachable`):** 5 trees
- `git fsck --full` **exit 0** — no corruption, no missing objects

### Loose:packed Ratio
- **≈1:287** (324 KiB loose vs 90.75 MiB packed) — healthy band is <1:10

## Compression Ratio

Measured as total uncompressed object content vs the compressed pack that
holds it — the ratio `git gc` is actually responsible for:

```
$ git cat-file --batch-all-objects --batch-check='%(objectsize)'
  → 786,059,102 bytes total

$ ls -l .git/objects/pack/*.pack
  →      94,856,353 bytes
```

| Measure | Uncompressed | Packed | Ratio | Space saved |
|---------|--------------|--------|-------|-------------|
| All objects (blob+tree+commit+tag) | 749.64 MiB | 90.46 MiB | **8.29:1** | 87.9% |
| Same, vs `size-pack` (incl. `.idx`) | 749.64 MiB | 90.75 MiB | 8.26:1 | 87.9% |

Uncompressed content by object type:

| Type | Count | Uncompressed | Share of content |
|------|-------|--------------|------------------|
| blob | 4,444 | 729.45 MiB | 97.3% |
| tree | 3,857 | 19.05 MiB | 2.5% |
| commit | 2,455 | 1.10 MiB | 0.15% |
| tag | 199 | 40 KiB | <0.01% |

Two readings worth keeping straight:

- **8.29:1** is delta+zlib compression of object *content* — the interesting
  number for a history repo whose 729 MiB of unique blob content collapses to
  a 90 MiB pack.
- The `du -sh .git` figure (93M) is *storage*, not compression; it also
  carries ~2 MiB of non-object overhead. Do not divide one by the other and
  call it a compression ratio.

## Healthy-Band Check (CLAUDE.md size limits)

| Metric | Healthy | Warning | This repo | Verdict |
|--------|---------|---------|-----------|---------|
| Total repository size | <500MB | 500MB–1GB | **93M** | ✅ 19% of healthy ceiling |
| Loose objects size | <100MB | 100–500MB | **324 KiB** | ✅ |
| Loose object count | <100 | 100–1000 | **49** | ✅ |
| Size ratio (loose:packed) | <1:10 | 1:10–1:2 | **≈1:287** | ✅ |
| Garbage | 0 | — | **0** | ✅ |

## Analysis

The repository is in excellent condition and is *holding* that condition under
daily fleet load:

- Single consolidated pack, 99.6% of objects packed, 0 garbage.
- 8.29:1 content compression — the pack is doing real work, not just
  accumulating loose objects.
- Loose objects re-accumulate at roughly **30–50 objects / 1.5 MiB per hour**
  while agents are committing (189 at 10:40 → 217 at 11:29 → 0 at gc → 49 at
  12:42). The daily 03:00 incremental gc is comfortably sufficient for that
  rate.
- Growth is bounded: ~1,000 commits over 11 days (Aug 26 → Sep 6) cost
  +2.05 MiB of pack. At that slope the 500 MB healthy ceiling is years away,
  and no bloat signature exists anywhere in the trend.

## Expected GC Impact (next run)

Given this baseline, the next gc should:
1. Pack the ~50–200 loose objects accumulated since — sub-second, <2 MiB moved.
2. Produce **no meaningful size reduction** — there is nothing left to
   compress; 0 prune-packable and 0 garbage.
3. Prune the handful of dangling/unreachable trees accumulated since.

Deep or aggressive compression is explicitly **not** warranted: the weekly
full gc already deep-repacked to 92M on 2026-09-06 04:00, within 1M of this
state, and aggressive passes are this workspace's historically dangerous
operation (bf-4x12ec, bf-198ne memcg OOMs).

## History

| Date | Document | .git | Loose | Pack |
|------|----------|------|-------|------|
| 2026-08-26 | [git-gc-baseline-2026-08-26.md](git-gc-baseline-2026-08-26.md) | 92M | 235 / 1.57 MiB | 88.70 MiB ×1 |
| 2026-08-26 | [git-gc-results-2026-08-26.md](git-gc-results-2026-08-26.md) | — | — | — |
| 2026-09-01 | [pre-gc-backup-metrics-2026-09-01.md](pre-gc-backup-metrics-2026-09-01.md) | — | — | — |
| 2026-09-01 | [repository-cleanup-2026-09-01.md](repository-cleanup-2026-09-01.md) | 91M | 16 / 116 KiB | 89.04 MiB ×2 |
| 2026-09-02 | [pre-repack-verification-2026-09-02.md](pre-repack-verification-2026-09-02.md) | — | — | — |
| 2026-09-06 | [git-gc-pre-flight-2026-09-06.md](git-gc-pre-flight-2026-09-06.md) | 94M | 189 / 1.43 MiB | 90.43 MiB ×1 |
| **2026-09-06** | **this file** | **93M** | **49 / 324 KiB** | **90.75 MiB ×1** |

The 18 GB / 17 GB-loose state that caused bf-1s6c3 and bf-4yjq (2026-08-12)
predates every row above and was repaired 2026-09-01; see
[repository-maintenance-guide.md](repository-maintenance-guide.md).
