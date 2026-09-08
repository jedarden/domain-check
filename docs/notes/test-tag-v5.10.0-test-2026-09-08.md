# Test Tag `v5.10.0-test` — Created and Pushed to Forgejo

**Bead:** domchk-5d2b7bed
**Date:** 2026-09-08
**Scope:** Create a test tag on this repository following versioning conventions and push it to Forgejo origin (`git.ardenone.com`), then verify it server-side.

## The Tag

| Field | Value |
|-------|-------|
| Tag name | `v5.10.0-test` |
| Target commit (full SHA) | `91ca7ffbe0d09ac2057a130ab771e21c4743bba6` |
| Target commit (short) | `91ca7ff` — "chore: update VERSION to 5.10.0-test" |
| Tag object SHA | `d117067e2fdc0b9c2679fb27a35fb36f58ba2a1a` |
| Tag type | Annotated |
| Tag message | `Test tag for bead domchk-5d2b7bed` |
| Tagger | `jedarden <github@jedarden.com>` |
| Created (tag timestamp) | `2026-09-08T08:19:47-04:00` |

The tag was created on `main` HEAD **at the moment of creation** (`91ca7ff`, which was
`origin/main` after the VERSION bump push — `52bf5ab..91ca7ff main -> main`). The tag is a
normal push (`* [new tag]`); no force-update of any existing ref was involved.

## Versioning Convention Followed

The repo's test-tag pattern (in evidence since `v0.0.1-test`, 448 local tags):

1. Bump the tracked `VERSION` file to the next `X.Y.0-test` value and commit it —
   `chore: update VERSION to <X>-test` (identical shape to `v5.8.0-test`'s commit
   `826114f` and `v5.7.0-test`'s `5456338`).
2. Place the test tag on that commit.

`VERSION` went `5.8.0-test` → `5.10.0-test`: the previous tag `v5.9.0-test` (below) was
created by an earlier attempt of this same bead **without** a VERSION bump, so the file
name leaps over `5.9` to stay in lockstep with the tag sequence. Tag types vary
historically (lightweight through `v5.8.0-test`, annotated for `v5.9.0-test`); this tag
is annotated so it carries the owning bead's ID. Nothing in the codebase reads `VERSION`
(no references in `*.go`/`*.sh`/`*.yml`) — it is convention-tracking only.

## Why a New Tag Instead of Re-Using `v5.9.0-test`

A prior attempt of this bead (2026-08-25) created and pushed `v5.9.0-test`, but it is
**unusable for "current main HEAD"** and unmovable:

- It points at `ec4ad86`, 13 days stale relative to this task's HEAD.
- The remote's `v5.9.0-test` further diverges from the local ref: Forgejo pins tag object
  `7a389c8` → commit `2939d39`, which lives in the **orphaned pre-rewrite history** (986
  commits, unrelated root) left behind by main's history rewrite. Moving it would require
  a force-update, which the org-wide no-force-push rule forbids.
- Resolution per [remote-divergence-analysis-2026-09-02.md](../investigations/remote-divergence-analysis-2026-09-02.md):
  leave the orphaned tag alone; create a fresh tag from current HEAD instead.

## Server-Side Verification (both criteria run live)

```
$ git ls-remote origin refs/tags/v5.10.0-test 'refs/tags/v5.10.0-test^{}'
d117067e2fdc0b9c2679fb27a35fb36f58ba2a1a	refs/tags/v5.10.0-test
91ca7ffbe0d09ac2057a130ab771e21c4743bba6	refs/tags/v5.10.0-test^{}

$ curl -s -H "Authorization: token …" \
    https://git.ardenone.com/api/v1/repos/jedarden/domain-check/tags/v5.10.0-test
{
    "name": "v5.10.0-test",
    "message": "Test tag for bead domchk-5d2b7bed",
    "id": "d117067e2fdc0b9c2679fb27a35fb36f58ba2a1a",
    "commit": { "sha": "91ca7ffbe0d09ac2057a130ab771e21c4743bba6",
                "created": "2026-09-08T08:19:47-04:00" },
    "zipball_url": ".../archive/v5.10.0-test.zip",
    "tarball_url": ".../archive/v5.10.0-test.tar.gz"
}
```

Local tag object, server-side ref, and the Forgejo API all agree on
`d117067e` → `91ca7ff`. The tag is browsable in the Forgejo web UI at
`https://git.ardenone.com/jedarden/domain-check/src/tag/v5.10.0-test`.

## Post-Tag Commit Note

This documentation file was committed **after** the tag was pushed, so `main` has moved
past `91ca7ff`. That does not affect the tag: a tag pins a commit immutably, and
"created from current main HEAD" was satisfied at creation time — the same way `v5.8.0-test`
pins `826114f` while later commits advanced main.
