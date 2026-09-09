# Fix Recommendations — bf-48wvu / bf-3hivb crash (domchk-c2227cb9, 2026-09-09)

**Deliverable for:** `domchk-c2227cb9` — "Recommend fixes or workarounds based on root
cause" (split-child of ALERT bead `bf-48wvu`, crash on `bf-3hivb`)
**Subordinate to:** [bf-48wvu-crash-investigation.md](bf-48wvu-crash-investigation.md) —
this doc adds no new cause claim; it restates the parent's root cause at current
precision, corrects one superseded recommendation, and maps every recommendation in the
parent report (dated 2026-08-16) to its implementation status as of 2026-09-09, verified
live on this box by the closing attempt.

---

## 1. Root cause (acceptance criterion 1)

**Root cause: repository bloat → memory-cgroup OOM during git operations.** Not a
timeout, not a caught signal, not a domain-check code defect.

- **The regime:** On 2026-08-12/13 this repository carried an **~18 GB `.git`** with
  **~17 GB of loose objects** — 17+ identical 237 MB `.beads/*.jsonl` snapshots committed
  before `.beads/` was gitignored. In that state every non-trivial git operation
  (`git log` over the bloated history, gc, push pack-objects) drove RSS past the
  **12 GiB `MemoryMax` of needle's per-dispatch systemd scope** and the kernel's memcg
  OOM killer delivered an uncatchable **SIGKILL** (`CONSTRAINT_MEMCG`).
- **Precision on the signal token:** `exit code -1` in these alerts is **needle's
  abnormal-child-death sentinel, not a signal number** (there is no signal −1). The
  kernel mechanism is **kernel-proven for the era-class** — 540 `CONSTRAINT_MEMCG`
  records survive from the 2026-08-16 journal — and **chain-inferred (MEDIUM-HIGH) for
  bf-3hivb specifically**, because the journal's first readable entry is
  2026-08-15 19:56:33 EDT and no kernel record survives for this particular kill. This
  matches how the canon treats the sibling beads of the same era (bf-4k2ws, bf-1ea4g).
- **Timeline (verified at HEAD):** `bf-3hivb` was created 2026-08-13T11:12:53Z. The
  crashed attempt's commit is stamped 13:15:55Z (per the parent report; that SHA sits in
  orphaned pre-squash history and is not reachable at HEAD). The alert's
  `Timestamp: 2026-08-13T13:16:42.828Z` is the **alert-creation heartbeat, not the kill
  instant** — so the kill falls in the ≤47 s window after that commit, i.e. during
  wrap-up/push, not mid-analysis. Retries landed 13:19:08Z (`ca99ee6`), 13:22:13Z
  (`7772b37`), 13:28:50Z (`67cdefb` — all reachable at HEAD, all first-hand verified),
  and **`bf-3hivb` closed successfully at 2026-08-13T13:34:57Z with the deliverable
  committed (0 Forgejo-specific commits; branches identical). Work was never lost.**
- **Classification:** INFRASTRUCTURE (repo-bloat era). The crash target was already
  resolved when the investigating chain was dispatched, so the remaining value of this
  chain is the prevention record — which is what this doc is.

## 2. Erratum to the parent report's Recommendations section

The parent report (§ Recommendations → Immediate Actions, 2026-08-16) opens with:

> **Run aggressive git garbage collection:** `git gc --aggressive --prune=now`

**Superseded — do not follow that instruction.** The bare form of exactly that command
was the death operation of the 2026-08-14 crash storm (bf-4x12ec: memcg-OOM SIGKILL,
129 killed attempts) and of bf-173o7e. The bounded replacement has existed since
2026-09-02 and is `./scripts/safe-git-gc.sh` (staged, memory-capped, checkpoint/resume,
`--check-only` preflight). Moreover the recommendation is **moot**: the bloat was packed
down on 2026-09-01 (18 GB → 93 MB) and is verified holding today (§4), so no cleanup
action of any kind is currently owed. (The parent report's other signal token,
"signal -1 (SIGKILL)", is likewise corrected by §1 above.)

## 3. Disposition of the parent report's recommendations

Every recommendation in the parent report, and where it stands today (each line
verified live 2026-09-09 by the closing attempt — see §6):

### Immediate Actions (Critical)

| Parent recommendation | Status | Where it lives now |
|---|---|---|
| Run aggressive git gc | **SUPERSEDED** | `scripts/safe-git-gc.sh` (bounded, staged, resumable); unnecessary today — repo is 105 MB |
| Remove large loose objects | **DONE** | 14 loose objects / 92 KiB; daily auto-gc check 02:30 + incremental gc 03:00 timers |
| Repository health monitoring | **DONE** | `scripts/check-repo-health.sh` exit 0 today; daily 02:00 `domain-check-repo-health.timer`; `scripts/preflight-health-check.sh` |
| Reduce concurrent git operations | **DONE** | `scripts/needle-with-limiter.sh` gates every dispatch through the crash-storm circuit breaker + concurrency limiter; breaker state clean (no open records) |

### Operational Changes

| Parent recommendation | Status | Where it lives now |
|---|---|---|
| Break complex git ops into smaller steps | **DONE** | `safe-git-gc.sh` staged design with per-stage checkpoints and `--resume` |
| Add progress logging | **DONE** | `.git/safe-gc.log` + `scripts/safe-git-gc-monitor.sh --watch` |
| Memory monitoring for git operations | **DONE** | Persistent git config — `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` (repo-local **and** global, so bare `git gc`/`git push` are bounded too). Verified effective bound today: worst case ≈3072 MiB per pack run, inside the 12 GiB dispatch scope |
| Retry logic with exponential backoff | **DONE** | Application-level policy implemented in code (`docs/notes/service-availability-retry-strategy.md`); ops-level storm backoff in the circuit breaker defers re-dispatch instead of re-entering the same crash |

### Long-term Solutions

| Parent recommendation | Status | Where it lives now |
|---|---|---|
| Repository migration / fresh clone | **NOT NEEDED — rejected** | The pack-down achieved the same end with zero history risk (and pushed history must never be rewritten). Verified holding 2026-09-06 → today |
| Document safe git operation patterns | **DONE** | `docs/maintenance/repository-maintenance-guide.md`; CLAUDE.md "Repository Bloat Prevention" + "Git Operations Safety" |
| Automated CI/CD repo-size checks | **DONE (different layer, by design)** | GitHub Actions is banned org-wide and CI is Docker-build-only (Argo Workflows, `iad-ci`); the size gates live in the daily systemd timers + the pre-commit hook instead |
| Pre-commit hooks blocking large files | **DONE — with one standing requirement** | 10 MB stage blocker installed and current (verified today). It is **per-clone**: any fresh clone must run `./scripts/setup-git-hooks.sh install`. Canonical tracked source: `scripts/pre-commit-repo-size-hook.sh` |

**The root-cause-class fix itself** is `.gitignore` line 66 (`.beads/`, plus repo-wide
`*.db`, `*.db.backup.*`, `*.jsonl`): bead state can no longer be committed, so the
17-snapshot failure mode cannot recur through it. `git ls-files .beads` → **0 tracked
files**, verified today.

## 4. Remaining recommendations, prioritized (acceptance criteria 2–4)

Nothing in the P1 tier is *new work* — the highest-impact items are standing behaviors
to preserve. Prioritized by impact × feasibility:

| # | Pri | Recommendation | Impact | Feasibility |
|---|-----|----------------|--------|-------------|
| R1 | **P1** | **Keep `.beads/` untracked.** Re-verify `git ls-files .beads \| wc -l` → 0 before any `.gitignore` change; never force-add bead state (scratch `.jsonl` needs explicit `git add -f` review) | Highest — this is the root-cause class | Trivial (already in force) |
| R2 | **P1** | **Never run bare `git gc --aggressive` / ad-hoc `git repack`.** Use `safe-git-gc.sh`; confirm bounds any time with `./scripts/setup-git-gc-config.sh --verify` (exit 0 = effective bound + pinned threads). This also closes the erratum in §2 | Highest — removes the one command that killed 129 attempts | Trivial (scripts exist; verify command is one line) |
| R3 | **P2** | **Drain the stale alert chain** — the crash layer is resolved; the alert layer is what still generates noise. Close this umbrella `bf-48wvu` (target `bf-3hivb` resolved 2026-08-13), duplicate sibling alert `bf-1p5h9`, and final leg `domchk-084b4d3e` once it cites this doc. Consistent with the fleet-wide finding that RESOLVED_TARGET alerts dominate the queue | Medium — removes false-positive investigation load | Easy (verify-then-close, no investigation owed) |
| R4 | **P2** | **Per-clone hook coverage:** run `./scripts/setup-git-hooks.sh install` on any fresh clone (idempotent), since the 10 MB gate is per-clone by design | Medium — the backstop that would have blocked the 237 MB commits | Easy |
| R5 | **P3** | **Disk headroom:** 21 GB free today on the single 444 GB root disk vs the 20 GB verify-gate floor and the 50 GB "healthy" threshold — the tightest current resource. Watch via the 5-min resource-monitor timer; clear idle `target/` dirs only under the documented pressure rule | Medium-term | Easy |
| R6 | **P3** | **Precision discipline in crash records:** cite `exit -1` as the abnormal-child-death sentinel and keep kernel-proven (journal era ≥ 2026-08-16) separate from chain-inferred (2026-08-12/13 kills) so future legs don't re-litigate the signal question | Low — documentation hygiene | Easy |

## 5. Workarounds (acceptance criterion 3)

The root cause is fixed and verified holding, so no workaround is currently *owed*.
These are the standing fallbacks should the condition ever re-appear (bloat detected, or
a git-operation kill wave):

- **W1 — suspected bloat:** `git count-objects -vH` + `du -sh .git` → if >500 MB, run
  `./scripts/safe-git-gc.sh --check-only`, then `./scripts/safe-git-gc.sh` (staged),
  monitoring with `./scripts/safe-git-gc-monitor.sh --watch`. Never the bare command.
- **W2 — crash storm in progress:** `./scripts/crash-circuit-breaker.sh status` (or
  preflight Check 4); all dispatches go through `./scripts/needle-with-limiter.sh` so a
  bead in storm backoff is deferred out of the ready frontier rather than re-crashed.
- **W3 — heavy git reads on a large history:** prefer targeted rev walks
  (`git log <a>..<b>` — exactly the shape bf-3hivb was running) over whole-history
  scans; the persistent pack-memory bounds cover the write side, and targeted walks
  keep the read side bounded too.

## 6. Verification record (all first-hand, 2026-09-09, this attempt)

| Check | Result |
|---|---|
| `du -sh .git` | **105M** (was ~18 GB) |
| `git count-objects -vH` | 14 loose / 92 KiB; 12,607 in-pack; **1 pack** 100.70 MiB; garbage 0 |
| `git ls-files .beads \| wc -l` | **0**; `.gitignore:66` `.beads/`, `:68-70` `*.db`/`*.db.backup.*`/`*.jsonl` |
| `./scripts/check-repo-health.sh` | **exit 0** — effective pack bound ≈3072 MiB worst case within ceiling; 0 unpushed backlog; no unmanaged aggressive gc running |
| `./scripts/setup-git-gc-config.sh --verify` | **exit 0** — windowMemory/deltaCacheSize/threads all pinned (local scope) |
| `./scripts/setup-git-hooks.sh --check` | **exit 0** — `.git/hooks/pre-commit` installed and byte-current |
| `systemctl --user list-timers 'domain-check-*'` | **8 timers, all with future trigger times** (service-monitor, monitoring, resource-monitor, alert-triage, repo-health, auto-gc, git-gc, git-gc-full) |
| `./scripts/crash-circuit-breaker.sh status` | no open breaker records |
| `scripts/needle-with-limiter.sh`, `scripts/preflight-health-check.sh` | present |
| Disk / memory | 21 GB free; 55 GB RAM available |
| Retry-commit SHAs (reachability) | `ca99ee6` 13:19:08Z, `7772b37` 13:22:13Z, `67cdefb` 13:28:50Z — all reachable at HEAD |
| `bead show bf-3hivb` | **Closed** 2026-08-13T13:34:57Z — target resolved, work never lost |

## 7. Scope

Docs-only. No application code, configuration, or cluster manifest was touched by this
bead — none was owed: every actionable recommendation from the parent report is either
implemented and verified (§3) or a standing behavior to preserve (§4). Build/test were
run in the shared worktree per the bead rules; note that `main`'s bare HEAD extract has
been independently unbuildable since 2026-09-07 (co-tenant `resource_monitor{,_test}.go`
exist only untracked in the worktree) — that is a pre-existing shared-worktree condition
documented elsewhere, not a result of this bead's docs-only change.
