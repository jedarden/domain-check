# Fix Verification: bf-173o7e — implement-fix child `domchk-5355a695`

| Field | Value |
|---|---|
| **Bead** | `domchk-5355a695` — "Implement fix for agent crash issue" (fix child of the bf-173o7e umbrella, created 2026-08-17) |
| **Task** | Implement a fix for the root cause identified for the bf-173o7e crash |
| **Verification date** | 2026-09-08 |
| **HEAD at verification** | `3a6f3d1` |
| **Conclusion** | **The fix already exists — implemented and committed by earlier beads; re-verified live first-hand this date. No new functional code was needed.** This bead's deliverable is this dated re-verification record plus its bead note. |
| **Precedent** | Same disposition as sibling fix-children `domchk-03295497` ("fixes already implemented and operational … no code changes needed") and `domchk-0597954c` |

---

## 1. What the fix had to address (RCA recap)

bf-173o7e ("Execute git gc --aggressive with pruning") died **129 times by exit −1**
across 131 completed attempts (12:59:48Z – 23:24:21Z on 2026-08-14, run durations
28.2–216.6 s). Root cause (canonical:
[`bf-173o7e-aug14-storm-root-cause-2026-09-02.md`](crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md)):
each attempt ran a **bare `git gc --aggressive --prune=now`** inside the dispatch
scope's **12 GiB memcg** (`run-p*.scope`, `memory.max=12884901888`), the
unbounded `git pack-objects` RSS exceeded the cgroup, and the kernel's
memory-cgroup OOM killer SIGKILLed the agent (recorded as the `exit −1`
sentinel). Needle re-dispatched the same gc task, producing a 131-attempt storm.

So the fix has to close three things: the bloat that made the repo
gc-worthy, the unbounded memory of the gc operation itself, and the
retry loop that turned one kill into 131 attempts.

## 2. The fix, layer by layer — all pre-existing and committed

Re-verified live 2026-09-08 in this workspace (every check below was run
first-hand for this record, not quoted from an older report):

| # | Causal link | Control in force | Landed in | Live evidence, 2026-09-08 |
|---|---|---|---|---|
| 1 | Repo bloat source (bead snapshots committed to git) | `.beads/` + `*.db` + `*.jsonl` gitignored | `4e169ee` (2026-08-17) | `git ls-files .beads` → **0 tracked files**; rules present at `.gitignore:66,68,70` |
| 2 | Bloat source (large files) | 10 MB pre-commit size gate | hook source `2ec91ec`; installer shipped 2026-09-06 | `.git/hooks/pre-commit` installed and **byte-identical** to tracked source (`setup-git-hooks.sh --check` rc 0) |
| 3 | **The kill locus itself** — unbounded pack memory vs the 12 GiB scope | Persistent git config `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` (repo-local **and** global) | `2ec91ec` | `setup-git-gc-config.sh --verify` → rc 0, effective worst case **≈3072 MiB** (windowMemory local, threads local, deltaCache local) vs this dispatch scope's cgroupfs `memory.max=12884901888` |
| 4 | **Death-op replay** — the exact command that killed 129 attempts | same command under a 768 MiB cgroup | `scripts/test-gc-memory-bounds.sh` | **17/17 passed, rc 0** — bare `git gc --aggressive --prune=now` **exited 0 under `MemoryMax=768M`**, pack-objects **peak RSS 320,476 KB < 700 MiB cap** (the crash run exceeded 12 GiB); bf-1ea4g push leg also passes (peak 232,500 KB) |
| 5 | Bounded maintenance path so no agent needs bare gc | `safe-git-gc.sh` (preflight, staged stages, checkpoint/resume) | `2ec91ec` | `--check-only`: resource checks pass ("GC not needed" = verdict, not failure); `test-safe-git-gc-limits.sh` **33/33 passed** |
| 6 | Continuous enforcement without manual runs | 8 systemd **user** timers (repo-health 02:00, auto-gc 02:30, incremental gc 03:00, weekly full gc `MemoryMax=4G`, plus monitoring timers) | `setup-repo-maintenance.sh` | `systemctl --user list-timers 'domain-check-*'` → **8/8 present, all future-triggered** |
| 7 | The retry storm itself (1 kill → 131 attempts) | per-bead crash circuit breaker + `needle-with-limiter.sh` pre-dispatch gate | `ad73b42` / `7f8af4d` (2026-09-07) | both scripts present at HEAD; dispatches route through the breaker/concurrency limiter so a bead in storm backoff is deferred, not re-fired |

Layer 4 is the decisive one: the variable that killed bf-173o7e was
pack-objects RSS against the scope bound. The bound now fits the operation in
**1/16th** of the scope (320 MiB peak under a 768 MiB cgroup), so the 12 GiB
dispatch scope has ~38× headroom, and the repository that made aggressive gc
attractive (17+ GB of loose bead snapshots) can no longer be created through
`.beads/`.

## 3. Scope of this bead's changes

- **No functional code changes** — none were needed; every layer above predates
  this dispatch. Re-implementing any of it would duplicate closed work (the
  same conclusion siblings `domchk-03295497` and `domchk-0597954c` reached for
  their crash families).
- This bead ships: this verification record, a live re-run of the layer
  battery, and the bead note recording the disposition.
- Go build/test gate: **N/A to this change** — it touches no Go code. For the
  record, both current build failures in this workspace are pre-existing and
  neither involves this bead (see §4).

## 4. Build-state flag (pre-existing; out of scope for this bead, surfaced for its owner)

- **HEAD `3a6f3d1` does not compile.** A `git archive HEAD` extract fails:
  `internal/server/server.go:117:13: undefined: NewResourceMonitor` — `e4fcbec`
  (2026-09-07 18:48 EDT) committed the call site but never
  `internal/server/resource_monitor{,_test}.go`, which sit **untracked** in the
  worktree. This blocks the `domain-check-build` Docker image from building on
  main; it is a service-deployment defect owned elsewhere, unrelated to this
  git-layer crash fix.
- The **worktree** build additionally fails today in `internal/watch`
  (`manager.go:491: domain.Parse undefined`) — a co-tenant's in-flight
  uncommitted edit. A green worktree `go build` earlier in the week rode on the
  untracked resource_monitor files; neither observation is evidence about this
  bead's change, which adds one `docs/*.md` file only.

## 5. How to re-verify

```bash
./scripts/check-repo-health.sh                     # bloat + bound + backlog
./scripts/setup-git-gc-config.sh --verify          # effective pack-memory bound
./scripts/safe-git-gc.sh --check-only              # preflight (exit 1 = "GC not needed" verdict)
./scripts/test-gc-memory-bounds.sh                 # death-op replay, 17 assertions
./scripts/test-safe-git-gc-limits.sh               # safe-gc suite, 33 assertions
systemctl --user list-timers 'domain-check-*' --all
git ls-files .beads | wc -l                        # must stay 0
./scripts/setup-git-hooks.sh --check               # hook installed and current
```

## Related

- RCA: [`docs/crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md`](crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md)
- Crash-context pass (same date): [`docs/crash-investigations/bf-173o7e-crash-context-domchk-114d6172-2026-09-08.md`](crash-investigations/bf-173o7e-crash-context-domchk-114d6172-2026-09-08.md)
- Repo cleanup record: [`docs/crashes/bf-173o7e-cleanup-verification.md`](crashes/bf-173o7e-cleanup-verification.md)
- Prevention inventory + gaps: [`docs/crash-prevention-requirements.md`](crash-prevention-requirements.md) (G-1..G-13)
- Per-layer validation procedures: [`docs/crash-prevention-validation.md`](crash-prevention-validation.md)
- Maintenance procedures: [`docs/maintenance/repository-maintenance-guide.md`](maintenance/repository-maintenance-guide.md)
