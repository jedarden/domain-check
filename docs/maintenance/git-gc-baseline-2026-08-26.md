# Git GC Baseline Metrics

**Date:** 2026-08-26  
**Repository:** domain-check  
**Purpose:** Capture repository state before running git garbage collection

## Repository Overview

- **Total .git Directory Size:** 92M
- **Total Commits:** 2,160
- **Total Reachable Objects:** 9,381

## Object Storage Analysis

### Current Object Distribution
```
Loose objects:      221 objects (1.49 MiB)
Packed objects:     9,164 objects (88.70 MiB)
Total packs:        1 pack
Garbage objects:    0 (0 bytes)
```

### Object Statistics
- **Loose Object Count:** 221
- **Loose Object Size:** 1.49 MiB
- **Packed Object Count:** 9,164
- **Pack File Size:** 88.70 MiB
- **Prune-packable Objects:** 0
- **Garbage:** 0 objects (0 bytes)

## Repository Health

### Git FSCK Status
- **Dangling Objects:** None detected
- **Corrupted Objects:** None detected

### Reflog Status
- **Expirable Reflog Entries:** None (dry-run showed no output)

## Pre-GC Observations

### Storage Efficiency
- **Pack Efficiency:** 98.3% of objects are packed (9,164 / 9,381)
- **Loose Object Overhead:** Minimal (221 loose objects, 1.49 MiB)
- **Storage Ratio:** 88.70 MiB pack size for 9,381 total objects

### Cleanup Potential
- **Immediate Cleanup Targets:** Minimal
  - No dangling objects to remove
  - No expirable reflog entries
  - No garbage objects detected

### Post-GC Expectations
Based on current metrics:
- **Space Savings:** Minimal expected (repository is already well-packed)
- **Primary Benefit:** Object consolidation and pack optimization
- **Risk:** Low (no complex cleanup operations needed)

## Next Steps

1. Run `git gc --prune=now` to consolidate objects
2. Compare post-GC metrics against this baseline
3. Document actual space savings achieved

## Notes

- Repository appears well-maintained with good object packing
- No significant cleanup opportunities identified
- Safe to proceed with standard git gc operation
