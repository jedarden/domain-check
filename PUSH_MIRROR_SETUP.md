# Forgejo to GitHub Push Mirror Setup

## Status: Blocked - Requires Manual API Token Generation

## Current State

### ✅ Completed
1. GitHub repository exists: `jedarden/domain-check` (public)
2. Git remotes configured:
   - `origin` → https://git.ardenone.com/jedarden/domain-check.git (Forgejo)
   - `github-mirror` → https://github.com/jedarden/domain-check.git (GitHub)

### ❌ Blocked
Forgejo API authentication failing with git credential token:
- **Root Cause:** Git credential token lacks API scope (`write:user`)
- Token retrieved from git credentials: `772b78d9d2...`
- API returns "Only signed in user is allowed to call APIs"
- Tested with: `Authorization: token`, `Authorization: Bearer`, Basic auth
- API version check also requires authentication
- **Note from CLAUDE.md:** "The stored token lacks the `write:user` scope and the call fails"

## Required Actions

### 1. Generate Forgejo API Token (Manual)
**Root Cause:** Git credential token lacks `write:user` API scope (per CLAUDE.md: "The stored token lacks the `write:user` scope and the call fails")

**Steps:**
1. Access https://git.ardenone.com
2. Navigate to: Settings → Applications → Generate New Token
3. Create token with scopes: `repo` (read/write) + `write:user` (if available)
4. Save token securely (do not commit to repo)
5. **Alternative:** Check if there's an existing admin API token in sealed secrets or external secrets

### 2. Configure Push Mirror (Automated once token is available)
```bash
FORGEJO_TOKEN="<api-token-from-step-1>"
GH_TOKEN="<github-personal-access-token>"

curl -s -X POST "https://git.ardenone.com/api/v1/repos/jedarden/domain-check/push_mirrors" \
  -H "Authorization: token $FORGEJO_TOKEN" \
  -H "Content-Type: application/json" \
  -d "{
    \"remote_name\": \"github-mirror\",
    \"remote_address\": \"https://jedarden:${GH_TOKEN}@github.com/jedarden/domain-check.git\",
    \"sync_on_commit\": true,
    \"interval\": \"8h\"
  }"
```

### 3. Verify Mirror Configuration
```bash
curl -s "https://git.ardenone.com/api/v1/repos/jedarden/domain-check/push_mirrors" \
  -H "Authorization: token $FORGEJO_TOKEN" | jq '.'
```

### 4. Test Sync
After configuration, any push to Forgejo will automatically sync to GitHub within 8 hours (or immediately with `sync_on_commit: true`).

## Technical Notes

- **Forgejo Version:** 15.0.7~gitea-1.22.0
- **API Endpoint:** https://git.ardenone.com/api/v1/
- **Authentication Required:** All API operations require authentication
- **Push Mirror Sync:** Server-side, automatic on commit

## Next Steps

Once the Forgejo API token is generated, complete steps 2-4 above to enable the push mirror.

## References

- Standard pattern from CLAUDE.md:
  1. Get FORGEJO_TOKEN from git credential fill
  2. Ensure GitHub repo exists (gh repo create)
  3. POST to /api/v1/repos/jedarden/domain-check/push_mirrors
  4. Include sync_on_commit=true and interval=8h
