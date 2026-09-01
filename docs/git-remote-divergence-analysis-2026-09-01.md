# Git Remote Divergence Analysis

**Date:** 2026-09-01  
**Repository:** domain-check  
**Task:** Investigate current Git remote state and divergence between GitHub and Forgejo

## Remote Configuration

```
github  https://github.com/jedarden/domain-check.git (fetch/push)
origin  https://git.ardenone.com/jedarden/domain-check.git (fetch/push)
```

- **`origin`**: Forgejo (git.ardenone.com) - Source of truth
- **`github`**: GitHub (github.com) - Read-only mirror

## Current State

### Remote Branches

| Remote | Branch | Commit | Message |
|--------|--------|--------|---------|
| `origin` (Forgejo) | main | `591bb1e` | docs: document bf-3561g scope and original task |
| `github` (GitHub) | main | `591bb1e` | docs: document bf-3561g scope and original task |
| Local | main | `591bb1e` | docs: document bf-3561g scope and original task |
| Local | github-main | `8373e5d` | migrate: rehydrate the bead workspace from bead-forge to bead-rs |

### Key Finding: **Remotes are IN SYNC**

Both `origin/main` (Forgejo) and `github/main` (GitHub) point to the exact same commit (`591bb1e`). There is NO divergence between the two remotes.

## Local Branch Status

The local `github-main` branch is **stale/outdated**:

- Located at commit `8373e5d` (older than current main)
- **1427 commits behind** current main branch
- Last common ancestor with current main: `8373e5d966109b3cea4fac90cb12d029b2031492`

This local branch appears to be an old reference that should be deleted or updated.

## Divergence Analysis

### Commits on Forgejo but not GitHub
**Count:** 0

### Commits on GitHub but not Forgejo
**Count:** 0

### Last Common Ancestor
**Commit:** `591bb1e` (current HEAD)

Both remotes are identical at commit `591bb1e966109b3cea4fac90cb12d029b2031492`.

## Acceptance Criteria Status

- ✅ All remote URLs documented
- ✅ Both GitHub and Forgejo refs fetched locally
- ✅ List of divergent commits on each side (0 on both sides)
- ✅ Last common ancestor commit identified (`591bb1e`)

## Recommendations

1. **Delete stale local branch:** The `github-main` local branch is 1427 commits behind and serves no purpose. It should be deleted:
   ```bash
   git branch -D github-main
   ```

2. **No action needed on remotes:** The Forgejo-to-GitHub mirror is working correctly. Both remotes are in sync.

3. **Continue normal workflow:** Push to `origin` (Forgejo) as the source of truth. GitHub will receive the mirror automatically via the server-side push mirror configured in Forgejo.

## Mirror Configuration

The Forgejo server has a push mirror configured to automatically sync commits to GitHub:

```bash
curl -X POST "https://git.ardenone.com/api/v1/repos/jedarden/domain-check/push_mirrors" \
  -H "Authorization: token $FORGEJO_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{\"remote_name\": \"github-mirror\", \
       \"remote_address\": \"https://jedarden:${GH_TOKEN}@github.com/jedarden/domain-check.git\", \
       \"sync_on_commit\": true, \
       \"interval\": \"8h\"}"
```

This ensures GitHub stays in sync without requiring client-side dual-push.

## Conclusion

**Status:** ✅ HEALTHY

The Git remote configuration is correct and both remotes are fully synchronized. There is no divergence between Forgejo (source of truth) and GitHub (read-only mirror). The only discrepancy is the stale local `github-main` branch, which can be safely deleted.
