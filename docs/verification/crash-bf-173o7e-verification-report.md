# Crash Verification Report — Repository Health After bf-173o7e Recovery

**Verification Date:** 2026-09-02 (~11:35 EDT)
**Bead:** domchk-cf01acff (verification task) — original crash bead: bf-173o7e
**Crash Events on Record:** OOM SIGKILL (exit -1) 2026-08-14T13:10:47Z; post-completion max_turns (exit 1) 2026-08-17T17:06Z
**Verdict:** ✅ **REPOSITORY HEALTHY** — all checks pass; state is within the "Healthy" band of every repository-size limit.

## Executive Summary

The `git gc --aggressive` operation that crashed (bf-173o7e) was successfully retried and
completed on 2026-08-17. This report re-verifies repository health as of 2026-09-02, more
than two weeks of subsequent commits and scheduled maintenance later.

**All acceptance criteria confirmed:**

| Check | Result |
|-------|--------|
| `git fsck --full` | ✅ Exit 0, zero output — no corruption, no dangling objects |
| Total repository size | ✅ 92 MB (`du -sh .git`) — well under the 500 MB healthy limit |
| Loose objects | ✅ 43 objects / 372 KiB — trivial; normal post-commit residue, not bloat |
| Pack integrity | ✅ Single pack, 90.18 MiB, 10,478 objects, complete `.idx`/`.bitmap`/`.rev` |
| Git operations | ✅ Normal — log/status/rev-list all complete in milliseconds |
| Disk space | ✅ 94 GB free on `/` |

## Verified Repository State (2026-09-02)

### `git count-objects -vH`

```
count: 43
size: 372 KiB
in-pack: 10478
packs: 1
size-pack: 90.18 MiB
prune-packable: 0
garbage: 0
size-garbage: 0 bytes
```

> **Snapshot note:** the loose-object count was observed at 25 objects / 224 KiB roughly
> ten minutes earlier in the same session. The drift is commits by concurrently running
> agents landing after the last pack rewrite — routine activity, not bloat. For the same
> reason all figures in this report are a point-in-time snapshot.

### Disk usage

```
$ du -sh .git
92M	.git

$ df -BG --output=avail / | tail -1
94G
```

### Pack files

```
.git/objects/pack/
  pack-98054595755f56a27981ce0fa7ff3860f4b5ae3e.pack    90M
  pack-98054595755f56a27981ce0fa7ff3860f4b5ae3e.idx    288K
  pack-98054595755f56a27981ce0fa7ff3860f4b5ae3e.bitmap  61K
  pack-98054595755f56a27981ce0fa7ff3860f4b5ae3e.rev     41K
```

Exactly one pack, with a complete index, reachability bitmap, and reverse index — the
expected shape after a successful aggressive gc plus scheduled maintenance. Pack mtime is
**2026-09-02 11:09**, i.e. the pack has been rewritten by routine scheduled gc since the
recovery; it is not the original 2026-08-17 file.

### Integrity

```
$ git fsck --full ; echo "fsck exit: $?"
fsck exit: 0
```

Zero errors, zero warnings, zero dangling objects reported.

### Git operations behave normally

| Operation | Time |
|-----------|------|
| `git log --oneline -1000` | 0.021s |
| `git status --porcelain` | 0.034s |
| `git rev-list --objects --all` (10,493 entries) | 0.112s |

Object reads (`git cat-file -t` / `-p` on HEAD) succeed. HEAD at verification time:
`9992c8e` (2026-09-02T11:27:08-04:00). The repository is fully operational — nothing in
the pre-crash bloat profile (OOM during routine git operations, exit -1) remains.

## Comparison Against the Crash Context

| Metric | Pre-crash (2026-08-14) | Post-recovery (2026-08-17) | This verification (2026-09-02) |
|--------|------------------------|----------------------------|-------------------------------|
| Loose objects | 17.20 GB | ~0 | 43 objects / 372 KiB |
| Pack | — | 444.24 MiB, 7,753 objects | 90.18 MiB, 10,478 objects |
| Total `.git` | ~18 GB | ~445 MB | 92 MB |
| Git operations | OOM/SIGKILL risk | Normal | Normal (ms-scale) |

Two observations on the trend since recovery, both benign:

1. **Pack shrank 444.24 MiB → 90.18 MiB** while the object count *grew* 7,753 → 10,478
   (subsequent documentation commits). Subsequent aggressive gc runs re-packed reachable
   objects and pruned the unreachable mass that inflated the original recovery pack.
2. **Loose objects are not zero.** The recovery run reached ~0; today's 43 objects are
   residue from commits made after the most recent pack rewrite. This is the normal
   steady state of an actively-committed repository and is 4–5 orders of magnitude below
   the 17.20 GB that triggered the OOM.

## Discrepancies With the Task's Quoted Figures

The dispatch context quoted "445M total, 0 loose objects, pack created 2026-08-17, 53GB
free disk." Those were accurate on 2026-08-17 but are stale today. The measured 2026-09-02
values — 92 MB total, 43 loose objects, pack rewritten 2026-09-02 11:09, 94 GB free —
differ, and every difference is in the healthy direction. No criterion is failed by the
difference; this report documents the measured values.

## Health Assessment vs. Repository Size Limits

| Limit (from CLAUDE.md) | Healthy | Warning | Critical | Measured | Band |
|------------------------|---------|---------|----------|----------|------|
| Total repository size | <500MB | 500MB–1GB | >1GB | 92 MB | ✅ Healthy |
| Loose object size | <100MB | 100–500MB | >500MB | 372 KiB | ✅ Healthy |
| Loose object count | <100 | 100–1000 | >1000 | 43 | ✅ Healthy |
| Loose:packed ratio | <1:10 | 1:10–1:2 | >1:2 | ~1:247 | ✅ Healthy |
| Disk free | >50GB | 30–50GB | <20GB | 94 GB | ✅ Healthy |

## Conclusion

The bf-173o7e recovery is holding. Two weeks after the successful gc retry, the repository
is one-fifth the size it was at recovery, integrity-verified end to end, and fully
operational at millisecond operation latency. The crash is closed; the risk it represented
(17 GB of loose objects driving OOM during routine git operations) has not regressed.

No further action required. Scheduled maintenance (daily incremental gc, weekly full gc —
see the `domain-check-git-gc*` systemd timers) continues to keep the repository in this
state.

## References

- `docs/research/git-gc-oom-crash-analysis.md` — OOM SIGKILL analysis (2026-08-14 event)
- `docs/crash-investigation-bf-173o7e-definitive-2026-08-25.md` — max_turns disambiguation (2026-08-17 event)
- `docs/verification/verification-report-bf-173o7e-2026-08-26.md` — post-recovery verification (2026-08-26)
- `docs/maintenance/repository-maintenance-guide.md` — size limits and maintenance procedure
