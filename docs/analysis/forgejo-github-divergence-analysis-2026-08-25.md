# Forgejo and GitHub Remote Divergence Analysis

**Analysis Date:** 2026-08-25  
**Repository:** domain-check  
**Remotes:**
- **Origin (Forgejo):** `https://git.ardenone.com/jedarden/domain-check.git` (source of truth)
- **GitHub Mirror:** `https://github.com/jedarden/domain-check.git` (read-only mirror)

## Current Status: ✅ SYNCHRONIZED

Both remotes are currently at the **exact same commit**:
```
Commit: 3f6e4f18c97e62100787e80f68fad35e27c078ac
Message: docs: add crash investigation for bead domchk-89c3d5c8 (bf-2igib crash)
```

## Commits Unique to Forgejo
**None** - All commits from Forgejo exist on GitHub

## Commits Unique to GitHub  
**None** - All commits from GitHub exist on Forgejo

## Most Recent Common Ancestor
```
Commit: 3f6e4f18c97e62100787e80f68fad35e27c078ac
```
(This is the current tip of both remotes)

## Analysis Method

1. Fetched both remotes:
   ```bash
   git fetch origin
   git fetch github-mirror
   ```

2. Compared remote refs:
   ```bash
   git rev-parse origin/main github-mirror/main
   ```

3. Result: Both refs resolve to identical SHA

## Historical Context

During the fetch operation, GitHub reported it was ahead by one commit:
```
From https://github.com/jedarden/domain-check
   0ce880b..3f6e4f1  main -> github-mirror/main
```

This indicated that at the moment of fetching, GitHub had commit `3f6e4f1` while Forgejo was still at `0ce880b`. However, subsequent fetches show both remotes are now synchronized at `3f6e4f1`.

## Conclusion

✅ **No divergence detected** - The server-side mirror between Forgejo and GitHub is functioning correctly. Both remotes contain identical commit history.

The Forgejo server-side push mirror is working as expected, keeping GitHub in sync with the source of truth.
