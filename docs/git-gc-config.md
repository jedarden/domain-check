# Git Automatic GC Configuration

This repository has automatic garbage collection (GC) configured to prevent repository bloat.

## Configuration

The following git GC settings are configured in `.git/config`:

```bash
gc.auto=100                    # Run auto GC when ≥100 loose objects exist
gc.autoPackLimit=10           # Run auto GC when ≥10 pack files exist
gc.aggressivedepth=50          # Aggressive repack depth
gc.aggressivewindow=1          # Aggressive repack window
gc.pruneexpire=2.weeks.ago     # Prune loose objects older than 2 weeks
gc.packrefs=true              # Pack refs during GC
gc.reflogexpire=90 days       # Reflog expiration
gc.reflogexpireunreachable=30 days  # Unreachable reflog expiration
```

## Rationale

- **`gc.auto=100`**: More aggressive than default (256). Triggers automatic GC when 100+ loose objects accumulate, preventing repository bloat before manual intervention is needed.
- **`gc.autoPackLimit=10`**: Triggers automatic GC when 10+ pack files exist, consolidating packs for better performance.

## Verification

To verify current settings:

```bash
git config --get gc.auto
git config --get gc.autoPackLimit
```

To trigger a manual GC:

```bash
git gc --aggressive
```

## Related

See [git-gc(1)](https://git-scm.com/docs/git-gc) for full documentation on git garbage collection settings.
