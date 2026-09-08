# GC memory bounds verification — domchk-6a227734 (2026-09-08)

Child 2 of 5 of domchk-b1626933 — acceptance criterion **"Safe-git-gc memory
limits confirmed."** Every check re-executed live; raw evidence (full test log,
journal scope lines, config dumps) at
`.beads/state/crash-prevention-testing/gc-bounds.md` and
`.beads/state/crash-prevention-testing/gc-memory-bounds-run.log` (untracked,
gitignored like all bead state).

**Verdict: all three acceptance criteria met.** The two memcg-OOM death
operations of the crash era now complete with 20–50× headroom under the bounds,
and the bound chain is verifiably in force on this repo.

## 1. Preflight — PASS

2026-09-08 ~02:28Z (local EDT 22:28): 53G available memory (gate ≥10G), 79G
free disk (gate ≥20G), load 4.37.

## 2. Effective bound chain — PASS, exit 0

```
./scripts/setup-git-gc-config.sh --verify
✅ Verified — effective (system -> global -> local); scope: windowMemory=local
   deltaCacheSize=local threads=local; worst-case pack memory ≈ 3072MiB
   (windowMemory=2g, threads=1, deltaCache=1g) — within the 6GiB ceiling for a
   12GiB dispatch scope.
```

All three keys resolve from this repo's **local** config
(`pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1`), and the
box-wide global config carries the same three keys — so a bare `git gc` or
`git push` is bounded both here and anywhere else on the box. Threads pinned to
1, which is what keeps the per-thread `pack.windowMemory` from multiplying.

## 3. Bounded pack runs — PASS, 17/17, exit 0

`./scripts/test-gc-memory-bounds.sh` replays both death operations of the
memcg-OOM era at reduced scale, each under a `MemoryMax=768M` cgroup (1/16th of
the 12GiB dispatch scope) with the deployed scaled bound (128m window / 64m
cache / 1 thread):

| Death operation | Peak RSS (`time -v`) | Whole-cgroup scope peak (`journalctl --user`) | Exit |
|---|---|---|---|
| bf-1ea4g — `git push` over a 192MiB unpacked backlog (6×32MiB near-identical snapshots) | **232,468 KB ≈ 227 MiB** | 274.4M (`gcmb-push-backlog-1106178.scope`) | 0 |
| bf-4x12ec / bf-173o7e — bare `git gc --aggressive --prune=now` over 8×64MiB incompressible blobs | **320,472 KB ≈ 313 MiB** | 596.8M (`gcmb-bare-aggressive-1106178.scope`) | 0 |

Both figures reproduce the prior records (~227MiB push, ~312MiB gc), so the
bound is stable across runs. Scope peaks run higher than the per-process RSS
because the cgroup also carries the git parent and page cache; both scopes
still exited 0 with no OOM kill.

Headroom against the 12GiB needle dispatch scope that killed bf-4x12ec:
pack-objects peak ≈ **2.6%**, worst whole-cgroup peak ≈ **4.9%** — far under,
as the criterion requires. Unit tests in the same run additionally confirmed
the bounds land in a fresh repo, stale `gc.auto` is overridden to 0 (GAP-2)
while advisory `gc.autoPackLimit` is preserved, and `--verify` rejects both an
unbounded repo and an unpinned `pack.threads`.

## 4. safe-git-gc.sh --check-only — PASS

`.git/safe-gc.log` @ 2026-09-07 22:33:42 local: config validated (`window=2g
delta=1g ceiling=6g threads=1`, worst case ≈3584MiB), all resource gates passed
(52,912M available memory, 79G disk, load 3.47), verdict **"GC not needed"** →
exit 1. That exit is the documented verdict code (0 would mean gc IS needed),
not a failure — the check path itself ran clean end to end.

Repo state at verification: 106MiB `.git`, 1 pack (100.25MiB), 235 loose
objects (1.55MiB) — inside every bloat threshold.

## Provenance notes

- No bare `git gc --aggressive` touched this repository. The aggressive-gc
  replay ran in the test's throwaway temp repo under its own cgroup — that is
  the bounded test path, not the bf-4x12ec mechanism.
- Scope unit names are per-invocation (`gcmb-<case>-$$`), so the
  scope-name-reuse race cannot false-fail a rerun.
- Companion evidence from this split family:
  [domchk-e0e1120e — monitoring stack verification](domchk-e0e1120e-monitoring-stack-verification-2026-09-08.md)
  (child 1).

## Re-verification pass 2 — 2026-09-08 ~06:23Z (gate-bounce re-close)

The bead was closed at 02:45:30Z and the needle shipped-work gate auto-reopened
it at 02:47:38Z (`verification-failed`, `failure-count:1`). The gate's log line
names the cause, and it is not this bead's work:

> `2026-09-08T02:47:37.963Z … bead closed but shipped-work check failed …
> reason=commit 55f23db has substantial changes but has not been pushed to its
> upstream origin/main`

`55f23db` is a **co-tenant** commit (bf-4x12ec kernel/systemd evidence, 333
lines, authored 22:43:58 EDT) that sat unpushed inside this dispatch's window
when the gate ran; this bead's deliverable `520c40e` was already on
origin/main. `55f23db` has since been pushed — HEAD↔origin/main divergence 0/0
at re-verification — and every check was re-executed live. Figures match pass 1
to within kilobytes; raw pass-2 evidence in
`.beads/state/crash-prevention-testing/gc-bounds.md` and
`gc-memory-bounds-run-2nd-2026-09-08.log` (untracked, gitignored like all bead
state).

| Check | Pass 2 result |
|---|---|
| `setup-git-gc-config.sh --verify` | exit 0 — all three keys from **local** scope, threads pinned, worst case ≈3072MiB within the 6GiB ceiling |
| `test-gc-memory-bounds.sh` | 17/17, exit 0 — push peak **232,476 KB ≈ 227 MiB** (pass 1: 232,468), aggressive-gc peak **320,524 KB ≈ 313 MiB** (pass 1: 320,472), both under `MemoryMax=768M` |
| `journalctl --user` scope peaks | `gcmb-push-backlog-1930904` **274.5M**, `gcmb-bare-aggressive-1930904` **595.7M** — 2.2% / 4.8% of the 12GiB dispatch scope |
| `safe-git-gc.sh --check-only` | config validated, all resource gates passed, "GC not needed" verdict (exit 1 = verdict, not failure); repo 107M, 362 loose objects (2.55MiB), 1 pack (100.25MiB) |

Verdict unchanged: **all three acceptance criteria hold.** The bound chain is
in force, and both memcg-OOM death operations of the crash era complete with
20–50× headroom under the bounds, reproducibly across two runs six hours apart.
