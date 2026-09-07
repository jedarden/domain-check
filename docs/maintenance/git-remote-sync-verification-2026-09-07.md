# Git Remote & Sync Verification — 2026-09-07

**Verified:** 2026-09-07 16:13 EDT (20:13 UTC)
**Repository:** `/home/coding/domain-check` (branch `main`, `4c15988` = `origin/main` = GitHub `main`)
**Bead:** domchk-e5239547
**Context:** bf-4yjq's git remote configuration; the bf-mje3pd crash occurred while fixing it
**Method:** `git remote -v` + `git fetch` + `rev-list --left-right --count` + `git ls-remote` on both remotes + Forgejo `/push_mirrors` API + `./scripts/check-repo-health.sh`
**Baselines:** [repository-health-check-2026-09-07.md](repository-health-check-2026-09-07.md), [pre-repack-verification-2026-09-02.md](pre-repack-verification-2026-09-02.md)

## Decision: ✅ VERIFIED — remote configuration correct and stable

All five acceptance criteria pass. Every path that carries a commit — local
`main`, Forgejo `origin/main`, and the GitHub mirror's `refs/heads/main` —
resolves to the identical commit `4c1598879cb7129f667cd60f00983007bfd5c11c`,
with zero divergence in either direction and a healthy object store.

## Acceptance Criteria

| # | Criterion | Result | Evidence |
|---|-----------|--------|----------|
| 1 | `origin` points to Forgejo | ✅ | `origin` → `https://git.ardenone.com/jedarden/domain-check.git` (fetch + push) |
| 2 | GitHub mirror exists as `github-mirror` | ✅ | Local remote `github-mirror` → `https://github.com/jedarden/domain-check.git` **and** the Forgejo **server-side** push mirror that actually performs the sync (see below) |
| 3 | Both remotes show same HEAD | ✅ | `git ls-remote`: Forgejo `4c159887` == GitHub `4c159887` == local `4c159887` |
| 4 | No unpushed / unpulled commits | ✅ | After `git fetch origin`: `git rev-list --left-right --count HEAD...origin/main` → `0  0` |
| 5 | Repository in clean state | ✅ with attribution | No merge/rebase/cherry-pick in progress; health check exit 0; working tree carries 96 co-tenant entries — see caveat |

## Sync state

```
$ git rev-list --left-right --count HEAD...origin/main   # after git fetch
0	0

$ git ls-remote origin refs/heads/main
4c1598879cb7129f667cd60f00983007bfd5c11c	refs/heads/main

$ git ls-remote https://github.com/jedarden/domain-check.git refs/heads/main
4c1598879cb7129f667cd60f00983007bfd5c11c	refs/heads/main
```

`git push` targets `origin` (`branch.main.remote = origin`); the local
`github-mirror` remote is inert unless explicitly named, so there is no
client-side dual-push path — GitHub is fed by Forgejo's server-side mirror, as
required.

## Forgejo server-side push mirror (the mechanism that does the sync)

`GET /api/v1/repos/jedarden/domain-check/push_mirrors` (2026-09-07 20:12 UTC):

| Field | Value |
|-------|-------|
| remote_name | `remote_mirror_3KJHNKYU5Mw` |
| remote_address | `https://github.com/jedarden/domain-check.git` |
| created | 2026-09-02T22:06:57Z |
| sync_on_commit | `true` (plus 8h fallback interval) |
| last_update | **2026-09-07T20:10:25Z** |
| last_error | *(empty)* |

The mirror is live and keeping up: HEAD commit `4c15988` landed at
20:09:57Z and the mirror had already pushed it to GitHub by 20:10:25Z — **28
seconds** of end-to-end latency, `sync_on_commit` working as configured. The
`ls-remote` match above is the ground truth; the repo-detail API's
`push_mirrors: null` is a known lie — use the `/push_mirrors` endpoint.

## Repository health (the bloat regime that caused the original issue)

`./scripts/check-repo-health.sh` → **exit 0** (2026-09-07 20:12 UTC):

| Metric | Value | Threshold | Verdict |
|--------|-------|-----------|---------|
| Repository size | 104 MB | < 500 MB | ✅ |
| Loose objects | 251 / 1.79 MiB | < 100 MB | ✅ (normal inter-gc churn) |
| Packed objects | 11,700 in 2 packs, 99.78 MiB | consolidated | ✅ |
| Garbage | 0 bytes | 0 | ✅ |
| Effective pack-memory bound | verified, worst case ≈3072 MiB | within 6 GiB ceiling | ✅ |
| Unpushed backlog | 0 commits | < 50 warn | ✅ |

The script's "large files in history" note (five 14.28 MB entries) is the
historical `dist/` goreleaser output already inside the pack — informational,
not a size regression, and not something a rewrite of pushed history should
ever be attempted to remove.

## Caveats (recorded, not failures)

1. **Working tree is not literally clean — co-tenant work, not sync drift.**
   This is a shared worktree; `git status --porcelain` shows 96 entries (52
   untracked, 22 modified, 12 staged+modified, 10 staged-deletes). None
   reference this bead, none are mine, and none affect what any remote holds:
   HEAD is exactly `origin/main` exactly GitHub `main`. Reverting or stashing
   sibling work to make the tree clean would destroy live work, so "clean
   state" is satisfied in the sense this criterion exists for — no mid-operation
   git state, no divergence, no unpushed/unpulled commits, healthy object store.
   (Same condition as the 92-entry tree recorded at the prior dispatch.)
2. **`pre-squash-history-20260816` is a deliberate local-only branch** — no
   upstream, divergent from `main` by design (the preserved pre-squash history
   archive). It is not unpushed backlog; see the phantom-divergence writeup in
   [../../branch-divergence-analysis.md](../../branch-divergence-analysis.md).
3. **`.git-repository-state.txt`** at the repo root is an untracked leftover of
   the 2026-09-02 verification (bead domchk-66ae89db), left for the record.

## Verdict

The remote configuration that bf-4yjq exposed is correct and stable: Forgejo
`origin` is the single push target, the GitHub mirror exists at both layers
(local remote + Forgejo server-side push mirror), and all three refs agree on
`4c15988` with zero divergence and zero errors from the mirror. No
configuration change was needed or made — this bead's deliverable is this
verification record.
