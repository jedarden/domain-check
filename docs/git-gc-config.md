# Git Automatic GC Configuration

This repository's automatic garbage collection settings, and the pack-memory bounds that
prevent the Aug-14 OOM crashes.

> **Corrected 2026-09-05 (domchk-adb15fe5).** An earlier version of this page documented
> `gc.aggressivewindow=1` (never actually set — and not an integer git accepts), a
> `gc.autoPackLimit` of 10 (the local value is 50), and recommended bare `git gc
> --aggressive` for manual gc — the command that caused the 129-kill memcg-OOM storm of
> 2026-08-14 (bf-173o7e). Values below are read from the live config, not from intent. For
> the gc strategy itself see
> [docs/maintenance/stepwise-git-gc-strategy.md](maintenance/stepwise-git-gc-strategy.md).

## Configuration

Effective values (`git config --show-origin --get-regexp '^(gc|pack|repack)'`):

| Key | Value | Scope |
|---|---|---|
| `gc.auto` | 100 | local |
| `gc.autoPackLimit` | 50 | local (box-wide global is 10) |
| `gc.aggressiveDepth` | 50 | local (same as git's default — a no-op) |
| `gc.aggressiveWindow` | *unset* | — |
| `gc.pruneExpire` | 2.weeks.ago | local + global |
| `gc.packRefs` | true | local |
| `gc.reflogExpire` | 90 days | local |
| `gc.reflogExpireUnreachable` | 30 days | local |
| `pack.window` | 5 | local (git default 10) |
| `pack.depth` | 20 | local (git default 50) |
| `pack.windowMemory` | 2g | local + global |
| `pack.deltaCacheSize` | 1g | local + global |
| `pack.threads` | 1 | local + global |
| `repack.writeBitmaps` | true | local |

## Rationale

- **`pack.windowMemory=2g` + `pack.deltaCacheSize=1g` + `pack.threads=1`** — the OOM fix.
  `pack.threads` defaults to the CPU count (12 here) and git multiplies the window memory
  by the thread count, so an unthreaded bound is no bound at all. Together these cap a pack
  run at ≈3 GiB; measured peak RSS dropped from >12 GiB to ≈313 MiB. See
  `scripts/setup-git-gc-config.sh` (apply/verify) and
  `scripts/test-gc-memory-bounds.sh` (proof).
- **`gc.auto=100`** — packs loose objects early instead of letting them pile up toward the
  17 GiB that made Aug-14 unrecoverable. Note this starts git's *own* background auto-gc,
  which does not pass through `safe-git-gc.sh`'s box-wide lock; it is bounded by the pack
  config above.
- **`pack.window 5` / `pack.depth 20`** — conservative delta search for routine gc. These
  protect plain `git gc` only: `git gc --aggressive` passes `--window=250 --depth=50` on the
  command line, which overrides config. That is a reason not to invoke `--aggressive`, not a
  protection against it.

## Verification

```bash
git config --show-origin --get-regexp '^(gc|pack|repack)'   # effective values + scope
./scripts/setup-git-gc-config.sh --verify                   # exit 1 = OOM bound missing
```

## Manual gc

Use the safe path, not `git gc --aggressive`:

```bash
./scripts/safe-git-gc.sh              # stages 1-2, cgroup-capped, checkpointed
./scripts/safe-git-gc.sh --full       # adds the deep stage
```

Design and rationale for the phased strategy:
[docs/maintenance/stepwise-git-gc-strategy.md](maintenance/stepwise-git-gc-strategy.md).

## Related

- [git-gc(1)](https://git-scm.com/docs/git-gc), [git-repack(1)](https://git-scm.com/docs/git-repack)
- [Repository Maintenance Guide](maintenance/repository-maintenance-guide.md)
