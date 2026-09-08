# bf-4yjq Fix Verification — domchk-7734c7dc (implement-fix leg)

**Date:** 2026-09-08
**Bead:** domchk-7734c7dc — implement-fix child of the bf-4yjq auto-split chain
`domchk-d21e4ad0` (crash-context, Closed rev 4) → `domchk-9f807d6f` (root-cause
analysis, Closed) → **this bead** → `domchk-c91d604f` (verify-fix, open)
**Verdict:** ✅ **FIX ALREADY SHIPPED AND EFFECTIVE — no new code required; verified live on this date.**

This is the same disposition its sibling `domchk-5355a695` (bf-173o7e
implement-fix leg, commit 89b9179) reached: the RCA's prevention stack is
committed, so the implement-fix child's obligation is to re-verify it
first-hand against the specific root cause of *its* chain — not to re-implement
it. Two earlier fix records for this same target already exist and are cited
rather than restated: [fix proposal/verification, domchk-0c601026 chain](crashes/bf-4yjq-fix-proposal-verification-2026-09-06.md)
and [final test report, domchk-3e443d56 chain](crashes/bf-4yjq-fix-test-report-domchk-3e443d56-2026-09-06.md).

## 1. The root cause this fix must address

From the closed RCA (`domchk-9f807d6f` notes, re-read this dispatch): **memcg-OOM
SIGKILL of the agent's `git push origin main`** — pack-objects pulling a working
set larger than the 12 GiB `MemoryMax` needle dispatch scope, over a store
ballooned to ~18 GB (17.2 GiB loose objects from bf-2ildm-era 237 MB
`.beads/*.jsonl` commits, made before `.beads/` was gitignored). 50 deaths
2026-08-12 17:53:53Z→20:30:38Z, amplified into a retry storm by the crash
handler releasing the bead back to the ready frontier (55 `bead.released`
events, ~3-minute re-fire). `exit -1` is needle's unrecorded-signal sentinel,
not a signal number; the kill is cgroup-scoped (host was never out of memory).

Every element a fix must neutralize: **(a)** the bloat source, **(b)** the
unbounded pack operation (push-side *and* gc-side), **(c)** the retry-storm
amplifier, **(d)** detection if it ever starts again.

## 2. Fix layers verified live this date (2026-09-08)

| # | Layer (causal link) | Evidence | Result |
|---|---------------------|----------|--------|
| 1 | Bloat source removed | `.beads/` gitignored (`.gitignore:66`, plus `*.db`/`*.jsonl` rules); `git ls-files .beads` → **0 tracked files**; gitignore fix commit `4e169ee` (SHA verified) | ✅ |
| 2 | Oversize-commit gate | `./scripts/setup-git-hooks.sh --check` → rc 0, hook "installed and byte-identical to tracked source" (`dfa60a9`, closes G-1) | ✅ |
| 3 | Pack operation bounded — **the death-op fix** | `./scripts/setup-git-gc-config.sh --verify` → rc 0: effective (system→global→local) `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1`, worst case ≈**3072 MiB** inside the 12 GiB scope ceiling (`2ec91ec`). Read directly from config: **local AND global both carry 2g/1g/1**, so a bare `git push` from any working state is bounded | ✅ |
| 4 | Repo holding at healthy size | `./scripts/check-repo-health.sh` → rc 0; **0 unpushed commits**; no unmanaged aggressive gc running | ✅ |
| 5 | Retry-storm amplifier closed | `./scripts/crash-circuit-breaker.sh status` → no open breakers; dispatch gate `scripts/needle-with-limiter.sh` present (`ad73b42` breaker + `7f8af4d` gate wiring) | ✅ |
| 6 | Detection/monitoring in force | 8 `domain-check-*` systemd user timers present (monitoring, resource, service, repo-health, auto-gc, git-gc, git-gc-full, alert-triage) | ✅ |

## 3. Live battery — the crash condition re-created, then neutralized

Ran this date, all three suites exit 0:

**`scripts/test-bf-4yjq-crash-condition.sh` — 6/6, rc 0** (the target harness,
built to the bf-4yjq workload spec; 1 GiB incompressible loose objects in a
512 MiB stand-in scope, 1/17th scale so the 17.2 GiB condition is never
re-created on the live repo):

- **A — crash re-created:** bare `git gc` over the loose set died by signal
  (scope exit 143) with the kill attributed to the kernel memcg OOM killer
  (`CONSTRAINT_MEMCG`, `bf4yjq-crash-a-*` scope) — **the bf-4yjq kill
  signature, reproduced**.
- **B — the deployed bounds mitigate:** the same repository, same bound, with
  the shipped `pack.windowMemory`/`deltaCacheSize`/`threads=1` → **exit 0**.
- **C — the packed store holds:** ordinary agent ops (`status --porcelain`,
  `log --oneline -5`, `fsck --full`) inside 512 MiB → all exit 0, store fully
  packed (1 pack, 0 loose) — the state the Aug-13 cleanup produced.

**`scripts/test-gc-memory-bounds.sh` — 17/17, rc 0** (death-op replay). The
push leg is the direct replay of this chain's death operation class: a
**192 MiB unpushed backlog held loose** (the exact pre-push state bf-4yjq/bf-1ea4g
died from) pushed under `MemoryMax=768M` → **exit 0, backlog delivered to the
bare remote, push peak RSS 232,464KB** — versus the >12 GiB the unbounded push
consumed when it was killed. The gc leg replays the verbatim bf-173o7e crash
command `git gc --aggressive --prune=now` under 768 MiB → exit 0, pack-objects
peak 320,520KB.

**`scripts/test-safe-git-gc-limits.sh` — 33/33, rc 0** (bounded-gc layer guard:
bounds land, stale values overridden, `--verify` rejects unbounded and
thread-multiplied states, checkpoint/resume semantics).

## 4. No-regressions statement and attribution

- This bead ships **zero functional code** — one documentation file. The fix
  under verification is config + scripts already at HEAD; the suites above are
  the regression gate for those layers and all pass.
- **Pre-existing, out of scope (not introduced by this change):** the worktree
  `go build ./...` fails in `internal/watch/manager.go:491` (`domain.Parse
  undefined`) — that file carries a co-tenant's **uncommitted** in-flight edit
  (`git status`: `M internal/watch/{manager,store,webhook}.go`). Separately,
  HEAD does not track `internal/server/resource_monitor{,_test}.go` (present
  only as untracked worktree files; e4fcbec committed the call site without
  them). Both conditions were documented as pre-existing by the two prior
  verification passes on this box (89b9179 / 9770036) and are unaffected by a
  docs-only commit.
- Journal hygiene note for whoever audits this date: any
  `safe-git-gc-run-*` / `gcmb-*` / `bf4yjq-*` scope OOM kills in the journal
  are the test suites' own deliberate negative tests (the 64 MiB-ceiling case
  and assertion A), not live incidents.

## 5. Close argument

The RCA closed with an explicit handoff: *"Child bead domchk-7734c7dc
('Implement fix') is already satisfied by this shipped prevention stack — its
owner can verify-then-close against canon without new code."* This document is
that verification, performed first-hand on 2026-09-08: every causal link of the
bf-4yjq root cause is covered by a committed, live-verified layer, and the
crash condition itself was re-created in a scratch scope and neutralized by
the shipped bounds. The remaining chain child (`domchk-c91d604f`, verify-fix)
can cite §3 directly: its "test reproduces the old crash condition / test
passes with the fix applied" criteria are satisfied by
`scripts/test-bf-4yjq-crash-condition.sh` assertions A→B and the push leg of
`scripts/test-gc-memory-bounds.sh`.
