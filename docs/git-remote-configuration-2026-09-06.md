# Git Remote Configuration — Verified Live 2026-09-06

**Bead:** domchk-e16b61e7
**Scope:** Current remote setup only (read-only verification). This is the
baseline for any subsequent change; it records what exists, not a proposal.

## Remote URLs (`git remote -v`, captured 2026-09-06 21:45 UTC)

```
github-mirror	https://github.com/jedarden/domain-check.git (fetch)
github-mirror	https://github.com/jedarden/domain-check.git (push)
origin	https://git.ardenone.com/jedarden/domain-check.git (fetch)
origin	https://git.ardenone.com/jedarden/domain-check.git (push)
```

| Remote | Points to | Role | Fetch/push URL override |
|--------|-----------|------|-------------------------|
| `origin` | **Forgejo** — `git.ardenone.com/jedarden/domain-check.git` | Source of truth; all pushes go here | none (single URL used for both) |
| `github-mirror` | **GitHub** — `github.com/jedarden/domain-check.git` | Read-only mirror; populated by Forgejo's server-side push mirror | none (single URL used for both) |

## Additional Remotes

**None.** `git remote -v` lists exactly these two. There are no leftover
remotes, no per-remote `pushurl` overrides, and no `url.*.insteadOf` /
`pushInsteadOf` rewrites in either repo-local or global config — a push goes
to exactly the URL shown above.

## Branch Tracking

```
branch.main.remote = origin
branch.main.merge  = refs/heads/main      → main tracks origin/main
```

Local branches:

| Branch | Commit | Upstream | Note |
|--------|--------|----------|------|
| `main` | `e0fab45` | `origin/main` | current |
| `pre-squash-history-20260816` | `7e4edf6` | none | local-only safety pin from the 2026-08-16 history squash |

`branch.main.vscode-merge-base = origin/main` is also set — a VS Code Git
extension key, not a remote configuration concern.

## Sync State (verified live, not from cached remote-tracking refs)

`git ls-remote` against each remote, 2026-09-06 21:45 UTC:

| Ref | SHA |
|-----|-----|
| `origin` (Forgejo) `refs/heads/main` | `e0fab453966952808820b2907e942e20730ecc03` |
| `github-mirror` (GitHub) `refs/heads/main` | `e0fab453966952808820b2907e942e20730ecc03` |
| local `main` (HEAD) | `e0fab453966952808820b2907e942e20730ecc03` |

**Zero divergence.** Forgejo = GitHub = local HEAD.

## Server-Side Push Mirror (Forgejo → GitHub)

Queried `GET /api/v1/repos/jedarden/domain-check/push_mirrors` (the repo-detail
endpoint reports `push_mirrors: null` and cannot be used for this), 2026-09-06:

- **Configured and healthy**, `sync_on_commit: true`, interval `8h`
- `last_update: 2026-09-06T21:27:15Z` (~18 min before this check), `last_error: ''`

Consistent with the zero-divergence finding above: GitHub is kept current by
Forgejo on every push to `origin`, per the standing rule **never to set up
client-side dual-push**. The local `github-mirror` remote exists for read-only
divergence checks (`git ls-remote github-mirror`), not for pushing.

## Notes for Whatever Change Follows

1. **The remote configuration itself is already in the target shape** implied by
   `CLAUDE.md`: Forgejo `origin` as the single push target, GitHub reachable as
   a named mirror remote, server-side mirroring carrying the sync. No remote
   change is required for that policy to hold.
2. **Naming drift in older docs:** the GitHub remote was named plain `github`
   as late as `docs/git-remote-divergence-analysis-2026-09-01.md` and is
   `github-mirror` from 2026-09-02 onward (`docs/branch-divergence-analysis.md`).
   Any doc or command from before 2026-09-02 that says `git fetch github` /
   `git push github` refers to the remote now called `github-mirror`. Checked
   today: no script under `scripts/` references either the old or new name, so
   nothing executable is stale.
3. The stale `github-main` local branch recommended for deletion in the
   2026-09-01 analysis no longer exists.

## Addendum 2026-09-06 22:05 UTC — tip advanced to `fb7bedd`, still zero divergence

Re-verified ~20 minutes after the check above (bead **domchk-2a4e7801**).
`origin/main` advanced one commit past the `e0fab45` recorded above:

| Ref (live `git ls-remote`, not cached refs) | SHA |
|-----|-----|
| `origin` (Forgejo) `refs/heads/main` | `fb7beddb74599da29173f0d41f76d03a87fa7a2c` |
| `github-mirror` (GitHub) `refs/heads/main` | `fb7beddb74599da29173f0d41f76d03a87fa7a2c` |
| local `main` (HEAD) | `fb7beddb74599da29173f0d41f76d03a87fa7a2c` |

**Zero divergence still holds** — Forgejo = GitHub = local HEAD, so the mirror
kept up with the new commit with no manual intervention.

Commit at the new tip:

| Field | Value |
|-------|-------|
| SHA | `fb7beddb74599da29173f0d41f76d03a87fa7a2c` |
| Subject | `docs: bf-2xygo raw needle logs — 91 crash events, claim verification + Aug-25 load-table correction (domchk-cd364e0a)` |
| Author | `jedarden <github@jedarden.com>` |
| Committer | `jedarden <github@jedarden.com>` |
| Date | `2026-09-06T17:55:09-04:00` (21:55:09 UTC) |
| Local state | up to date — `git fetch origin main` reported nothing new; HEAD already at `fb7bedd` |

Server-side push mirror re-checked via
`GET /api/v1/repos/jedarden/domain-check/push_mirrors`: `sync_on_commit: true`,
interval `8h`, `last_update: 2026-09-06T21:56:14Z` (65 s after the commit was
authored), `last_error: ''` — consistent with the mirror having already
delivered `fb7bedd` to GitHub.

Baseline figures in the body above remain correct for the `e0fab45` snapshot
they describe; only the tip SHA moved.

## Addendum 2026-09-06 22:14 UTC — GitHub side fetched and verified at `72f782f`, still zero divergence

GitHub-side counterpart of the addenda above (bead **domchk-0dcecbe7**).
`git fetch github-mirror main` succeeded and advanced the tracking ref
`fb7bedd..72f782f`, then both remotes were checked live with `git ls-remote`
(cached remote-tracking refs were not trusted):

| Ref (live `git ls-remote`) | SHA |
|-----|-----|
| `github-mirror` (GitHub) `refs/heads/main` | `72f782f32047497e8dc31e8cd469bdfe0ad4ff12` |
| `origin` (Forgejo) `refs/heads/main` | `72f782f32047497e8dc31e8cd469bdfe0ad4ff12` |
| local `main` (HEAD) | `72f782f32047497e8dc31e8cd469bdfe0ad4ff12` |

**Zero divergence still holds** — the push mirror delivered the 22:09 UTC
commit to GitHub, and GitHub = Forgejo = local HEAD.

Commit at the GitHub tip:

| Field | Value |
|-------|-------|
| SHA | `72f782f32047497e8dc31e8cd469bdfe0ad4ff12` |
| Subject | `docs: re-verify Forgejo origin/main — tip advanced to fb7bedd, zero divergence holds (domchk-2a4e7801)` |
| Author / Committer | `jedarden <github@jedarden.com>` |
| Date | `2026-09-06T18:09:58-04:00` (22:09:58 UTC, unix 1788732598) |
| Parent | `fb7beddb74599da29173f0d41f76d03a87fa7a2c` |

This snapshot is also recorded as structured state in
`docs/github-remote-state.json`, completing the `next_steps` list that
`docs/forgejo-remote-state.json` (2026-08-13) left open: "Document GitHub
mirror state" / "Compare Forgejo vs GitHub mirror states". The three-file
family (`forgejo-remote-state.json`, `local-main-state.json`,
`github-remote-state.json`) now covers all three refs at their respective
capture times; note the older two predate the 2026-08-16 history squash, so
their SHAs are not comparable to the current tip — only this GitHub snapshot
is current.
