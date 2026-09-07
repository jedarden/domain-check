# Git Remote Divergence Analysis — Forgejo vs GitHub

**Bead:** domchk-e4bf05f7
**Date:** 2026-09-02
**Scope:** Read-only fetch + comparison of `origin` (Forgejo, `git.ardenone.com` — source of truth) and `github-mirror` (GitHub — read-only mirror). No reconciliation performed.

## TL;DR

| Surface | State |
|---------|-------|
| `main` branch | **Fully synchronized.** Local = Forgejo = GitHub = `5eb6fa1`. Zero divergence. |
| Tag `v5.9.0-test` | **Diverged.** Local points into current (rewritten) history; both remotes point into an orphaned pre-rewrite history. |
| Local-only tags | 447 tags exist locally on neither remote (448 local vs 1 remote). |
| Working tree | 45 uncommitted paths exist on neither remote (8 modified, 37 untracked). |

## Method

```bash
git fetch origin --prune        # Forgejo — already up to date locally
git fetch github-mirror --prune # GitHub — advanced main debd24f..5eb6fa1
git ls-remote <remote>          # authoritative server-side ref set for both
```

Server-side ref sets were compared via `ls-remote` (not just local remote-tracking refs) so tags and branches not auto-followed by fetch are included.

## main branch: no divergence

```
local main           5eb6fa1a9281103c866674e16ac7d45aec7fbe75
origin/main          5eb6fa1a9281103c866674e16ac7d45aec7fbe75
github-mirror/main   5eb6fa1a9281103c866674e16ac7d45aec7fbe75
merge-base           5eb6fa1a9281103c866674e16ac7d45aec7fbe75  (= the tip itself)
```

- Commits on GitHub not on Forgejo: **0** (`git rev-list --count origin/main..github-mirror/main`)
- Commits on Forgejo not on GitHub: **0** (`git rev-list --count github-mirror/main..origin/main`)
- Total commits on main: 1637. Tip: `5eb6fa1` — "docs: verify bf-284lqt closure reason satisfies false-positive acceptance criteria (domchk-b77fb9c9)", 2026-09-02 08:22 EDT.
- Both remotes advertise only `refs/heads/main` — no other branches exist on either server.

### The GitHub "behind" was local staleness, not divergence

Before this fetch, our remote-tracking ref `github-mirror/main` pointed at `debd24f`
("docs: add comprehensive root cause analysis for agent crash bead domchk-ac6f3e1f",
2026-09-02 04:10 EDT) — 45 commits behind the tip. `debd24f` is a strict ancestor of
`5eb6fa1`, so the fetch was a **fast-forward**: GitHub had received the Forgejo push
mirror, our clone simply hadn't fetched. Nothing to reconcile.

## Tag `v5.9.0-test`: the actual divergence

Three different values for the same ref name:

| Location | Tag object | → Commit | Reachable from main? |
|----------|-----------|----------|----------------------|
| Local | `25c0952` | `ec4ad86` | **Yes** |
| Forgejo | `7a389c8` | `2939d39` | **No** |
| GitHub | `7a389c8` | `2939d39` | **No** |

Evidence:

- Both tagged commits have the **identical tree** `409eeedaceb07de8b5c405dca2793a28dcde8b80`,
  identical message ("feat: add domain name suggestion mode with suggest=true parameter")
  and identical author date (2026-08-25 13:23:47 -0400) — `ec4ad86` is the rewrite-era
  duplicate of `2939d39`.
- The two histories are **completely unrelated**: `git merge-base ec4ad86 2939d39`
  returns nothing. Current main is rooted at `00117cb` (2026-08-09, "fix: remove unused
  time import and update bootstrap test initialization"); the orphaned line is rooted at
  `9c20897` (2026-03-22, "Initial commit: research and architecture plan") and carries
  986 commits.
- So main's history was rewritten (re-rooted) after 2026-08-25 and pushed to both
  remotes, but the tag on the remotes was never moved — it still pins the **old
  pre-rewrite history**, 986 commits that survive on both servers only because the tag
  references them. Locally the tag was re-created against the new history.
- Why the divergence persists silently: `git fetch` **does not update an existing local
  tag** by default (tag auto-follow only fills in missing names), so the mismatch never
  surfaces as a fetch conflict. The Forgejo→GitHub push mirror, by contrast, synced the
  tag correctly — the two remotes agree with each other; the local ref is the outlier.

### Related orphaned history

Local branch `pre-squash-history-20260816` (tip `7e4edf6`, 2026-08-16, 722 commits) is a
**second**, separately-unrelated history line — `git merge-base 2939d39
pre-squash-history-20260816` is also empty. The repo has therefore had at least two
independent history-rewrite events, each leaving orphaned lines behind.

## Tag inventory asymmetry

| Location | Tag count |
|----------|-----------|
| Local | 448 |
| Forgejo | 1 (`v5.9.0-test`) |
| GitHub | 1 (`v5.9.0-test`) |

447 local tags (almost all `v*-test` / goreleaser-e2e pipeline-verification tags) exist on
neither remote. Local-only tag bloat — a cleanup candidate, but not remote divergence.

## Content on neither remote

45 uncommitted working-tree paths (8 modified tracked files — mostly `scripts/*.sh` and
systemd units — plus 37 untracked docs/scripts from recent crash investigations) exist on
neither server. This is local-only work, listed here for completeness; it is not part of
remote-vs-remote divergence.

## Recommended follow-ups (NOT executed — this bead was analysis-only)

1. **Tag `v5.9.0-test`:** decide which target is canonical. Since the remotes' tag pins
   986 otherwise-dead commits, the cheapest reconciliation is to force-update the tag on
   Forgejo (and let the mirror propagate) to the local value `25c0952` → `ec4ad86`, then
   `git fetch --force --tags` locally. Note the org-wide **no-force-push rule** — a tag
   force-update needs explicit operator sign-off or the orphaned tag should simply be
   deleted on both remotes.
2. **Tag bloat:** prune the 447 local-only `v*-test` tags (`git tag -l | grep -Ff <list>
   | xargs git tag -d` after review) — they add clone weight and noise.
3. **Working tree:** commit or discard the 45 uncommitted paths so clones and mirrors
   carry the same crash-investigation docs the workspace references.
4. If history rewrites are expected to continue, consider documenting the canonical
   root and retiring stale preservation branches (`pre-squash-history-20260816`) once
   their content is confirmed recoverable elsewhere.
