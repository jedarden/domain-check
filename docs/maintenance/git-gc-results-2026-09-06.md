# Git GC Results — 2026-09-06

**Verified:** 2026-09-06 11:45–11:47 EDT (15:45–15:47 UTC)
**Repository:** `/home/coding/domain-check` (branch `main`)
**Bead:** domchk-45da5280 (post-gc verification)
**Chain:** pre-flight domchk-4b8a3472 ([git-gc-pre-flight-2026-09-06.md](git-gc-pre-flight-2026-09-06.md), 10:39 EDT) → standard gc run → this verification
**Baselines:** [git-gc-baseline.md](git-gc-baseline.md) (rolling, updated for this run),
[git-gc-baseline-2026-08-26.md](git-gc-baseline-2026-08-26.md),
[git-gc-results-2026-08-26.md](git-gc-results-2026-08-26.md)
**Follow-up:** re-verified 2026-09-06 12:42 EDT by domchk-825c89bc (fsck, state
holding, [compression ratio](#compression-ratio) added) — see
[Re-verification](#re-verification-1242-edt) below.

## Decision: ✅ DONE — repository healthy, no deep compression required

All five acceptance criteria pass. The repository sits deep inside the
"Healthy" band of the CLAUDE.md size-limits table with a single consolidated
pack and a clean fsck. Deep compression would buy nothing (the weekly full
gc's deep repack already ran today at 04:00 and landed at 92M) while spending
the operation class with the worst history in this workspace (bf-4x12ec /
bf-198ne memcg OOMs).

## The Verified Run

`./scripts/safe-git-gc.sh` — mode `standard`, started 11:29:38 EDT,
completed 11:29:44 EDT (~6 s wall), checkpoint state `complete`.

| Property | Value |
|----------|-------|
| Stage 1 | `git gc --prune=now` — 1 s |
| Stage 2 | `git repack -q -d --no-write-bitmap-index --max-pack-size=500m`, then `git repack -q -d -f --no-write-bitmap-index --depth=50 --window=50` — <1 s |
| Final verification | 3 s (fsck + count-objects) |
| Cgroup ceiling | `MemoryMax=2g` (systemd-run scope `safe-git-gc-2487371-1.scope`) |
| **Peak memory** | **165.7 M** (8% of ceiling), 2.475 s CPU — per `journalctl --user` |
| OOM events | none |
| Effective config bounds | ✅ `./scripts/setup-git-gc-config.sh --verify` re-checked post-run: windowMemory=2g, deltaCache=1g, threads=1 → ≈3 GiB worst case |

Peak memory matters here: bounded `git gc`/`repack` is the exact operation
that memcg-OOM'd agents in August when run unbounded. This run peaked at
~166 MB — roughly 12× under its own 2g ceiling.

## Integrity Verification

```
$ git fsck --full          # 11:45:03 EDT
(no output)
FSCK_EXIT=0
```

- **Exit 0, zero findings** — no corruption, no missing objects, no broken links
- **0 dangling** and **0 unreachable** (`git fsck --unreachable` → 0 entries)
- The 2 dangling objects recorded by the pre-flight (tree `fe8f444d…`,
  commit `5e3434c0…`) were pruned by this run's `--prune=now`

## Before/After — the Verified Run

| Metric | Pre (11:29:38) | Post (11:29:44) | Change |
|--------|----------------|-----------------|--------|
| **.git directory size** | 94M | 93M | −1M |
| **Loose objects** | 217 | **0** | −217 (−100%) |
| **Loose object size** | 1.64 MiB | 0 | −1.64 MiB |
| Packed objects | 10,712 | 10,906 | +194 |
| Pack files | 1 | 1 | — (stayed consolidated) |
| Pack size | 90.43 MiB | 90.75 MiB | +0.32 MiB |
| Prune-packable | 0 | 0 | — |
| Garbage | 0 | 0 | — |
| Dangling (fsck) | 2 | 0 | −2 |

(The pre-count had grown from the pre-flight's 189 loose objects at 10:40 to
217 at 11:29 — ~28 objects of normal fleet churn in 49 minutes on an
actively-worked repo.)

## Snapshot at Verification Time (11:46:56 EDT)

```
$ du -sh .git          → 93M
$ git count-objects -vH
count: 11
size: 76.00 KiB
in-pack: 10906
packs: 1
size-pack: 90.75 MiB
prune-packable: 0
garbage: 0
size-garbage: 0 bytes
```

- Total objects 10,917 (10,906 packed + 11 loose); total commits 2,442
- Pack efficiency **99.9%**; loose:packed ratio ≈ **1:992**
- The 11 loose objects reappeared in the 17 minutes after gc from concurrent
  fleet activity and are **all reachable** (`--unreachable` → 0) — normal
  churn, not a failed cleanup. Next scheduled incremental: 2026-09-07 03:00 EDT.

## Compression Ratio

*Added by the 12:42 EDT follow-up (domchk-825c89bc). Measured as total
uncompressed object content vs the compressed pack holding it — the ratio a
gc run is actually responsible for.*

```
$ git cat-file --batch-all-objects --batch-check='%(objectsize)'
  → 786,059,102 bytes total uncompressed content

$ ls -l .git/objects/pack/pack-b1b822e67a8930efb1664a55affa9682dd3090d6.pack
  →      94,856,353 bytes
```

| Measure | Uncompressed | Packed | Ratio | Space saved |
|---------|--------------|--------|-------|-------------|
| All objects (blob+tree+commit+tag) | 749.64 MiB | 90.46 MiB | **8.29:1** | 87.9% |
| Same, vs `size-pack` (incl. `.idx`) | 749.64 MiB | 90.75 MiB | 8.26:1 | 87.9% |

Uncompressed content by object type:

| Type | Count | Uncompressed | Share |
|------|-------|--------------|-------|
| blob | 4,444 | 729.45 MiB | 97.3% |
| tree | 3,857 | 19.05 MiB | 2.5% |
| commit | 2,455 | 1.10 MiB | 0.15% |
| tag | 199 | 40 KiB | <0.01% |

The 8.29:1 figure is the meaningful one: 729 MiB of unique blob content across
1,726 commits reachable from `HEAD` collapses into a 90 MiB pack. Note what it
is *not* — `du -sh .git` (93M) is storage including ~2 MiB of non-object
overhead, so dividing the checkout size by the `.git` size would not produce a
compression ratio.

## Re-verification (12:42 EDT)

domchk-825c89bc re-ran the acceptance checks ~73 minutes after the run to
confirm the post-gc state is holding under concurrent fleet load:

| Check | Result at 11:46 | Result at 12:42 |
|-------|-----------------|-----------------|
| `git fsck --full` | exit 0, 0 dangling | **exit 0**, 2 dangling trees |
| `--unreachable` | 0 entries | 5 trees |
| `.git` size | 93M | **93M** (unchanged) |
| Loose objects | 11 / 76 KiB | 49 / 324 KiB |
| Packed objects / packs | 10,906 / 1 | 10,906 / **1** |
| `size-pack` | 90.75 MiB | 90.75 MiB (unchanged) |
| Garbage | 0 | **0** |

Every figure holds or is ordinary churn. The 49 loose / 5 unreachable objects
accumulated from fleet commits in the interim, are all reachable-or-dangling
normal churn, and are exactly what the next scheduled gc consumes — the pack
itself did not grow at all. `./scripts/check-repo-health.sh` passes
(1 pack, "acceptable fragmentation", no large files in the working directory).
**The repository is verified healthy and ready for production use.**

## Healthy-Band Check (CLAUDE.md size limits)

| Metric | Healthy | Warning | This repo | Verdict |
|--------|---------|---------|-----------|---------|
| Total repository size | <500MB | 500MB–1GB | **93M** | ✅ 19% of healthy ceiling |
| Loose objects size | <100MB | 100–500MB | **76 KiB** | ✅ |
| Loose object count | <100 | 100–1000 | **11** | ✅ |
| Size ratio (loose:packed) | <1:10 | 1:10–1:2 | **≈1:992** | ✅ |
| Garbage | 0 | — | **0** | ✅ |

## Historical Comparison

| Date | .git | Loose objects | Pack size | Packs | Total objects |
|------|------|---------------|-----------|-------|---------------|
| 2026-08-26 (baseline) | 92M | 235 / 1.57 MiB | 88.70 MiB | 1 | 9,399 |
| 2026-09-01 (post-gc) | 91M | 16 / 116 KiB | 89.04 MiB | 2 | 9,420 |
| 2026-09-06 10:40 (pre-flight) | 94M | 189 / 1.43 MiB | 90.43 MiB | 1 | 10,901 |
| **2026-09-06 11:29 (post-gc)** | **93M** | **0** | **90.75 MiB** | **1** | **10,906** |
| 2026-09-06 11:46 (verified) | 93M | 11 / 76 KiB | 90.75 MiB | 1 | 10,917 |

Growth context: 1,440 commits (Aug 26) → 2,442 commits (Sep 6) — about 1,002
commits in 11 days cost only +2.05 MiB of pack. Steady, bounded growth; no
bloat signature anywhere in the trend.

## Run History Today (from `.git/safe-gc.log`)

| Time | Mode | Outcome |
|------|------|---------|
| 03:00:01 | standard (daily timer) | ✅ completed, 92M |
| 04:00:31 | full (weekly timer, deep repack) | ✅ completed, 92M |
| 09:04:01 / 10:28:42 / 11:10:58 / 11:28:24 | `--check-only` | "GC not needed" (217 loose < 1000 threshold, 1 pack ≤ 5) |
| **11:29:38** | **standard (explicit)** | ✅ **completed, 93M — run verified by this document** |

The four declines are expected behavior, not failures: per the comment at
`scripts/safe-git-gc.sh:418`, the thresholds inside `check_gc_needed` drive
`--check-only` reporting only — an explicit or scheduled run always executes
the stages.

## Why Not Deep Compression

1. Nothing left to compress — the repo is a single 90.75 MiB pack with 0
   loose, 0 garbage, 0 prune-packable objects.
2. The deep-repack equivalent already ran today: the weekly full gc
   (04:00:31) produced 92M, i.e. within 1M of the current state.
3. Aggressive/deep passes are the workspace's historically dangerous
   operation (bf-4x12ec, bf-198ne). Running one for ~zero upside contradicts
   the standing guidance in CLAUDE.md and
   [repository-maintenance-guide.md](repository-maintenance-guide.md).

## Recommendations

- **None required.** Next incremental gc 2026-09-07 03:00 EDT; next weekly
  full 2026-09-13 04:00 EDT. Re-run gc manually only if loose objects exceed
  several hundred between scheduled runs.
- **Stale-premise signpost for sibling beads:** the still-open beads premised
  on shrinking an 18 GB / 17 GB-loose repository (domchk-92ddb78e,
  domchk-f473834e, domchk-c0f3b98e — "Run aggressive git garbage collection")
  describe a state that no longer exists; this repo is 93M with 0 loose
  objects at gc completion and has been holding at ~93M since 2026-09-01.
  They should be verified against live state and closed by their owners, not
  executed.
