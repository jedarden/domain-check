# Git GC Baseline Metrics

**Date:** 2026-08-26  
**Repository:** domain-check  
**Purpose:** Baseline measurements before running git gc operations

## Repository Overview

- **Total commits:** 1,440
- **Total objects:** 9,396

## .git Directory Size

- **Total .git size:** 92M

## Object Storage Details

### Loose Objects
- **Count:** 235 objects
- **Size:** 1.57 MiB

### Pack Files
- **Number of packs:** 1
- **Packed objects:** 9,164
- **Pack size:** 88.70 MiB

### Garbage and Prunable Objects
- **Garbage objects:** 0
- **Prune-packable objects:** 0

## Analysis

The repository is in good condition:
- **Pack efficiency:** 97.5% of objects are already packed (9,164 / 9,399)
- **Loose objects:** Minimal - only 235 loose objects (2.5% of total)
- **No garbage:** Repository has no orphaned or corrupted objects
- **Single pack:** Well-optimized with a single pack file

## Expected GC Impact

Given the current state, running `git gc` will likely:
1. **Pack the 235 loose objects** into the existing pack file
2. **Minimal size reduction** - loose objects are only 1.57 MiB
3. **Potential minor compression improvements** from repacking

The repository appears to have been recently maintained or gc'd already, as evidenced by:
- High pack ratio (97.5%)
- Very few loose objects
- No garbage accumulation

## Notes

- Baseline captured before any git gc operations
- Use this as reference point for post-gc comparison
- Consider running `git gc --aggressive` only if significant size reduction is needed
