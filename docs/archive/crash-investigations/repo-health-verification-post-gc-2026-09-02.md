# Repository Health Verification — Post-GC (bf-4x12ec follow-up)

**Bead:** domchk-b037ca90 · **Date:** 2026-09-02 · **Verdict: HEALTHY — all criteria pass**

Verification that the repository is healthy and stable after the git gc task
(bf-4x12ec) reported completion.

## Acceptance criteria results

| # | Criterion | Threshold | Measured | Result |
|---|-----------|-----------|----------|--------|
| 1 | Loose objects (`git count-objects -vH`) | < 200 | **54** (25 at check start; grew with concurrent agent commits — normal churn) | ✅ |
| 2 | `git fsck` — no corruption | dangling OK | **`--full`: exit 0, zero findings** (see note below) | ✅ |
| 3 | `.git` size (`du -sh .git`) | < 1GB | **92M** | ✅ |
| 4 | `git status` duration | < 5s | **0.009s** | ✅ |
| 5 | `git log --oneline -5` | no hang | **0.003s**, works | ✅ |
| 6 | No stuck/hung git processes | none | **None running, none hung** | ✅ |

Supporting detail: single pack, 10,478 in-pack objects, 90.18 MiB size-pack,
0 garbage bytes. `main` == `origin/main` (0 ahead / 0 behind after fetch).
Both stash entries (`stash@{0}`, `stash@{1}`) resolve and their commit objects
exist. HEAD/branch history: 1,657 commits, intact and fully reachable.

## Important finding: `git fsck --no-full` "invalid reflog entry" noise is NOT corruption

`git fsck --no-full` in this repo exits 2 with 1,008 `error: … invalid reflog
entry <OID>` lines. This looked like corruption but is a **git 2.50.1
`--no-full` artifact on packed repositories**: with `--no-full`, fsck does not
open packfiles, so reflog entries whose targets live in the pack are misreported
as invalid. Every flagged OID was individually verified to exist
(`git cat-file -t` → `commit`) and to be reachable
(`git merge-base --is-ancestor` → yes).

Control experiment on an unrelated healthy repo on this box (`~/SIGIL`):

| Repo | `fsck --no-full` | `fsck --full` |
|------|------------------|---------------|
| domain-check | exit 2, 1,008 invalid-reflog-entry errors | **exit 0, zero output** |
| SIGIL (control) | exit 2, 5,238 invalid-reflog-entry errors | exit 0, 185 dangling-object warnings only |

Both repos fail identically under `--no-full` and both are clean under
`--full`. Conclusion: the `--no-full` reflog errors are systemic noise, not
damage. No reflog repair (`reflog expire`, `reflog delete`) is needed or
warranted — attempting it would destroy valid history entries.

**Guidance for future verification beads:** use `git fsck --full` (not
`--no-full`) as the integrity gate on this box, or expect the `--no-full`
invalid-reflog-entry noise on any packed repo. Dangling-object warnings under
`--full` are benign.

## Process check

During verification, one `git fsck --full` from a concurrent needle session was
observed running at ~87% CPU — actively running (state `R`), not stuck; it
finished on its own. Remaining `git`-matching processes were needle dispatch
wrapper shells, not git operations. At verification close: zero git processes
running.

## Conclusion

The gc (bf-4x12ec) left the repository in a healthy, stable state: object
database fully intact, history complete and in sync with Forgejo, repository
well under all size thresholds (92M vs 1GB limit), all everyday operations
effectively instant, no leaked processes. This matches the repo's documented
pattern that domain-check infrastructure issues resolve to gc/config artifacts,
not object-store damage.
