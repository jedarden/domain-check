# Git GC Pre-Flight Verification — 2026-09-06

Bead: domchk-4b8a3472
Repository: `/home/coding/domain-check` (branch `main` at `1b21053`)
Verified at: 2026-09-06 10:39–10:40 EDT (14:39–14:40 UTC)

## Result: ✓ All pre-flight checks pass — repository verified ready for git gc

Five acceptance criteria pass. No blockers; gc may proceed.

## Acceptance Criteria

| Criterion | Requirement | Status | Evidence |
|-----------|-------------|--------|----------|
| Available memory | ≥ 10 GB | ✅ PASS | **47 GiB** available (`free -h`: 15 Gi used, 21 Gi free, 26 Gi buff/cache) — 4.7× requirement |
| Free disk space | ≥ 30 GB | ✅ PASS | **63 GB** free on `/` (444G disk, 359G used, 86% used) — 2.1× requirement |
| Repository size documented | — | ✅ DONE | `.git` = **94 MB**; single pack 90.43 MiB, 1.43 MiB loose |
| `git fsck` integrity | clean | ✅ PASS | `git fsck --full` exit 0; only 2 dangling objects (informational, see below) |
| Loose objects counted | — | ✅ DONE | **189 loose objects / 1.43 MiB**, 0 prune-packable, 0 garbage |

## System Resources

```
$ free -h
               total        used        free      shared  buff/cache   available
Mem:            62Gi        15Gi        21Gi        17Mi        26Gi        47Gi
Swap:           24Gi          0B        24Gi

$ df -h /
Filesystem      Size  Used Avail Use% Mounted on
/dev/disk/...   444G  359G   63G  86% /

$ uptime
load average: 0.95, 1.04, 1.16   (up 22 days)
```

Both resources sit comfortably inside the "Safe Operating Limits" table from
`CLAUDE.md` (memory warning threshold is 10 GB; disk warning is 30 GB — both
are cleared by wide margins). Load is idle.

## Repository State

```
$ du -sh .git
94M     .git

$ git count-objects -vH
count: 189
size: 1.43 MiB
in-pack: 10712
packs: 1
size-pack: 90.43 MiB
prune-packable: 0
garbage: 0
size-garbage: 0 bytes
```

Every metric is inside the "Healthy" band of the repository size limits table:
total < 500 MB, loose < 100 MB, loose count < 100 MB/100 objects, and the
loose:packed ratio is ~1:63 (healthy band is < 1:10). No bloat signature —
compare the 18 GB / 17 GB-loose state that caused bf-1s6c3.

Context: the daily incremental gc ran 2026-09-06 03:00:01 EDT and the weekly
full gc ran 2026-09-06 04:00:31 EDT (`domain-check-git-gc.timer` /
`domain-check-git-gc-full.timer`), so this snapshot already reflects today's
scheduled maintenance. Next incremental: 2026-09-07 03:00 EDT.

## Repository Integrity

```
$ git fsck --full
dangling tree fe8f444d6361e77a2e702b3775a15879257d7ccc
dangling commit 5e3434c0cdadd8173a0628f8e1e85d63632f49d4
(exit 0)
```

Exit 0 — no corruption, no missing objects, no broken links. The 2 dangling
objects are unreachable leftovers of normal branch/reflog churn (a repo with
active agent commits grows these constantly); they are what gc is expected to
prune, not an integrity defect. A further `--unreachable` pass lists 23
unreachable entries, consistent with recent non-gc'd churn.

## Effective GC Memory Bounds (bonus check)

Because an unbounded pack-objects is the known OOM mechanism in this
workspace (bf-4x12ec / bf-198ne), the effective bound chain was verified with
the repo's own gate:

```
$ ./scripts/setup-git-gc-config.sh --verify
✅ Verified — effective (system -> global -> local); scope: windowMemory=local
   deltaCacheSize=local threads=local; worst-case pack memory ≈ 3072MiB
   (windowMemory=2147483648, threads=1, deltaCache=1073741824) — within the
   6442450944 ceiling for a 12GiB dispatch scope.
```

Both repo-local and box-global configs are in place; a bare `git gc` (and
`git push`'s pack-objects) are memory-bounded to ≈3 GiB worst case.

## Concurrency / Locks

- No `.git/gc.pid`; no `git gc/repack/fsck/prune/pack-objects/index-pack`
  processes running
- No git-internal lock files (`index.lock`, pack/ref locks all absent)
- One **non-git** custom lock present: `.git/needle-trailer.lock` (0 bytes,
  2026-08-09) — needle trailer hook serialization, not consulted by git,
  does not block gc (same informational finding as the 2026-09-02 pre-repack
  verification)

## Conclusion

Memory (47 GiB), disk (63 GB), integrity (fsck clean), and object state
(189 loose objects / 1.43 MiB, 0 garbage) all pass, with effective gc memory
bounds verified and no concurrent git activity. The repository is verified
ready for git gc.
