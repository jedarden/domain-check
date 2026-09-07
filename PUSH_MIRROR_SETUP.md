# Forgejo to GitHub Push Mirror Setup

## Status: OPERATIONAL — verified 2026-09-06

The mirror described as "Blocked" below was configured on **2026-09-02T22:06:57Z**
and is confirmed working. Do not re-create it; verify with the commands in
"Verification" before assuming anything about its state.

## Current State (verified 2026-09-06)

1. GitHub repository exists: `jedarden/domain-check` (public)
2. Git remotes configured:
   - `origin` → https://git.ardenone.com/jedarden/domain-check.git (Forgejo — push here)
   - `github-mirror` → https://github.com/jedarden/domain-check.git (GitHub — read-only alias)
3. Server-side push mirror on Forgejo:
   - `remote_name`: `remote_mirror_3KJHNKYU5Mw` (server-assigned; the local remote
     name `github-mirror` is only a client-side fetch alias and is unrelated)
   - `remote_address`: https://github.com/jedarden/domain-check.git
   - `sync_on_commit`: true, `interval`: 8h
   - `last_update`: 2026-09-06T14:31:44Z, `last_error`: (empty)
4. Refs in sync at verification time: `main` byte-identical on Forgejo, GitHub, and local.
   Live end-to-end check: locally created commit `26472b5` reached Forgejo and then
   GitHub within minutes — mirror `last_update` advanced 14:31:44Z → 14:51:25Z,
   `last_error` empty, so `sync_on_commit` fired on the push.
5. Push path verified: `git push origin main` → exit 0. Don't expect a pinned SHA in
   this file to still match — `main` moves constantly here; run the ls-remote commands
   above for current state.

## Verification

```bash
# 1. Local remotes
git remote -v

# 2. Mirror exists and is healthy (needs a Forgejo API token; the git-credential
#    token is enough for GET here — see history below)
FORGEJO_TOKEN="$(git credential fill <<< 'protocol=https
host=git.ardenone.com
' | grep password | cut -d= -f2)"
curl -s "https://git.ardenone.com/api/v1/repos/jedarden/domain-check/push_mirrors" \
  -H "Authorization: token $FORGEJO_TOKEN"

# 3. Refs agree across Forgejo and GitHub
git ls-remote origin refs/heads/main
git ls-remote github-mirror refs/heads/main
```

Note: the repo-detail endpoint (`GET /repos/jedarden/domain-check`) reports
`push_mirrors: null` even when a mirror exists — use the dedicated
`/push_mirrors` endpoint above, not the repo-detail field.

## Known Deviation From Workspace Convention

The Forgejo repo is **private** while its GitHub mirror is **public**. The
documented convention is that public portfolio repos are public on both and
deliberately-private repos skip the mirror entirely. Domain-check sits between
the two; content is effectively public via GitHub. Flipping Forgejo visibility
is an outward-facing change and needs an explicit operator decision.

## History

- **2026-09-02T22:06:57Z** — push mirror created on Forgejo (server-side).
- **Earlier** — this file previously recorded the setup as *Blocked*: the
  stored git-credential token lacks the `write:user` API scope, so the
  `POST /push_mirrors` call failed with "Only signed in user is allowed to call
  APIs". Read-only and repo-scoped `GET` calls work with the same token, which
  is why the verification curl above needs no new token. The mirror was
  ultimately created by another path; the token limitation still holds for
  *writing* mirror config.

## Technical Notes

- **Forgejo Version:** 15.0.7~gitea-1.22.0
- **API Endpoint:** https://git.ardenone.com/api/v1/
- **Push Mirror Sync:** Server-side, automatic on commit (`sync_on_commit: true`),
  with an 8h reconciliation interval
- **Never set up client-side dual-push** — push to `origin` only; GitHub is fed
  by Forgejo. The `github-mirror` remote exists for fetch/ls-remote comparison.
