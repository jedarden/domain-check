# Git Remote State - Pre-Migration Baseline

**Date:** 2026-09-01  
**Purpose:** Document current git remote configuration before GitHub → Forgejo migration

## Current Remotes

```
$ git remote -v
github	https://github.com/jedarden/domain-check.git (fetch)
github	https://github.com/jedarden/domain-check.git (push)
origin	https://git.ardenone.com/jedarden/domain-check.git (fetch)
origin	https://git.ardenone.com/jedarden/domain-check.git (push)
```

## Remote Details

### origin (Forgejo - Primary)
- **URL:** `https://git.ardenone.com/jedarden/domain-check.git`
- **Purpose:** Primary remote - Forgejo instance
- **Status:** Active, current primary

### github (GitHub - Mirror)
- **URL:** `https://github.com/jedarden/domain-check.git`
- **Purpose:** Mirror - GitHub mirror
- **Status:** Active, secondary

## Migration Goal

Convert from GitHub as primary to Forgejo as primary, with GitHub as read-only mirror.

## Next Steps

1. Verify Forgejo remote has all necessary branches
2. Confirm Forgejo is source of truth
3. Update documentation to reflect Forgejo-primary workflow
4. Consider renaming remotes for clarity (optional)

---
*This file captures the pre-migration state for verification and rollback purposes.*
