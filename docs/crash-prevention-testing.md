# Crash Prevention Testing — Consolidated Results

**Parent bead:** domchk-b1626933 ("Test and document crash prevention solution", umbrella)
**Consolidator:** domchk-a61cc009 (child 5 of 5)
**Consolidated:** 2026-09-08 (spot-checks re-run at consolidation time, noted inline)
**Scope limit:** no child ran `DOMCHECK_RUN_LONG_TESTS=1`; long-running tests (10-minute memory
growth, sustained-load benchmarks) are out of scope for this package by the parent bead's rule.

Children 1–4 each ran their checks **first-hand on this box** and committed an evidence summary at a
tracked path (the `.beads/state/crash-prevention-testing/` tree where raw logs were tasked is
gitignored by the repo's bloat-prevention layer — `.gitignore:66`, whole `.beads/`, 0 tracked files —
so raw logs live there untracked and the identical summaries are committed under `docs/verification/`).
This file consolidates their results; it does not re-derive them.

## Verdict summary

| # | Criterion (parent bead) | Child bead | Commit(s) | Result |
|---|---|---|---|---|
| 1 | Monitoring system tested and verified | domchk-e0e1120e | `dfda388` | ✅ 4 monitors exit 0; all 6 required timers future-triggered |
| 2 | Safe-git-gc memory limits confirmed | domchk-6a227734 | `520c40e`, `5d0c83f` | ✅ 17/17 bounded tests; peaks 227 MiB / 313 MiB under a 768M cgroup |
| 3 | Alert thresholds validated | domchk-189d2f80 | `017452a` | ✅ suite 13/13 (×2), thresholds cross exactly, cooldown/dedup 16/16 |
| 4 | All safeguards tested and working | domchk-ea18c7a8 | `0d1d8e1` | ✅ with attribution: pure HEAD build fails on a missing *untracked* co-tenant file; with that file present, build rc 0 and internal/server 151/151 |
| 5 | Complete documentation package | domchk-a61cc009 | this commit | ✅ this file + bf-4yjq report §15 + operations runbook |

---

## Child 1 — Monitoring stack (domchk-e0e1120e)

**Run window:** 2026-09-08T01:21–01:23Z · **Committed summary:**
[`docs/verification/domchk-e0e1120e-monitoring-stack-verification-2026-09-08.md`](verification/domchk-e0e1120e-monitoring-stack-verification-2026-09-08.md)
· **Raw logs:** `.beads/state/crash-prevention-testing/*.log` (untracked by design)

| Command | Date | Observed result |
|---|---|---|
| `./scripts/resource-monitor.sh --once` | 2026-09-08 01:21Z | exit 0 — MEMORY 53GB OK · DISK 79GB OK · CPU 2.27 OK · PRESSURE 0% OK · UNSAFE_GC none OK |
| `./scripts/service-monitor.sh --once` | 2026-09-08 01:21Z | exit 0 — inference gateway HEALTHY (HTTP 200), "PRE-FLIGHT CHECK PASSED" |
| `./scripts/crash-pattern-detection.sh` | 2026-09-08 01:22Z | exit 0 — "No crashes detected in the last 24hours", System Status: STABLE |
| `./scripts/check-repo-health.sh` | 2026-09-08 01:21Z | exit 0 — `.git` 104 MB, 150 loose / 12,174 in-pack / 100.25 MiB single pack / 0 garbage, unpushed backlog 1 (< 50) |
| `systemctl --user list-timers 'domain-check-*' --all` | 2026-09-08 01:22Z | 8 timers, all future-triggered; the six required cadences match spec exactly (2 min / 5 min / 10 min monitors; daily 02:00 repo-health; daily 03:00 gc; Sun 04:00 full gc) |

Disclosed in the child's record: the health check's single ⚠️ line (five 14.28 MB GoReleaser
`dist/` blobs in history) is pre-existing inert weight, blocked going forward by the 10 MB
pre-commit hook; removal would need a history rewrite, which the standing rules prohibit.

## Child 2 — GC memory bounds (domchk-6a227734)

**Runs:** pass 1 2026-09-08 ~02:28–02:34Z, pass 2 (post shipped-work-gate bounce, cause = a
co-tenant's unpushed commit, since pushed) 2026-09-08 ~06:12–06:23Z · **Committed summary:**
[`docs/verification/domchk-6a227734-gc-bounds-verification-2026-09-08.md`](verification/domchk-6a227734-gc-bounds-verification-2026-09-08.md)
· **Raw logs:** `gc-memory-bounds-run{,-2nd-2026-09-08}.log`, `gc-bounds.md`

| Command | Date | Observed result |
|---|---|---|
| `./scripts/setup-git-gc-config.sh --verify` | both passes | exit 0 — effective bound (system → global → local): `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` → worst case ≈ 3072 MiB, within the 6 GiB ceiling for a 12 GiB dispatch scope |
| `./scripts/test-gc-memory-bounds.sh` | both passes | **17 passed, 0 failed, exit 0** — includes replays of both historical memcg-OOM death operations under `MemoryMax=768M`: bounded `git push` over a 192 MiB unpacked backlog → exit 0, peak RSS 232,468–232,476 KB ≈ 227 MiB; bare `git gc --aggressive --prune=now` over 8×64 MiB blobs → exit 0, peak RSS 320,472–320,524 KB ≈ 313 MiB |
| `journalctl --user` (test scopes) | both passes | whole-cgroup peaks 274.4–274.5M (push) and 595.7–596.8M (gc) — 2–5% of the 12 GiB dispatch scope, no OOM kill |
| `./scripts/safe-git-gc.sh --check-only` | both passes | exit 1 = the documented **"GC not needed" verdict**, not a failure; config validated and resource gates passed first (`.git/safe-gc.log`) |

Unit checks inside the same suite: bounds land in a fresh repo; stale `gc.auto` is overridden to 0
while advisory `gc.autoPackLimit` is preserved; `--verify` rejects an unbounded repo and an unset
`pack.threads`; `--verify` passes via the box-wide global bound alone.

**Consolidation-time spot-check (2026-09-08 06:4xZ):** `./scripts/setup-git-gc-config.sh --verify`
→ exit 0, same ≈3072 MiB figure; `./scripts/check-repo-health.sh` → exit 0, 0 unpushed.

## Child 3 — Alert thresholds, cooldown, dedup (domchk-189d2f80)

**Run window:** 2026-09-08 10:03:22–10:04:44Z (attempt 2; attempt 1's logs retained as
corroboration) · **Code state tested:** HEAD `efb1603` · **Committed summary:**
[`docs/verification/domchk-189d2f80-alert-thresholds-verification-2026-09-08.md`](verification/domchk-189d2f80-alert-thresholds-verification-2026-09-08.md)
· **Raw logs:** `*-189d2f80-r2.log` (+ `*-189d2f80-fresh.log` from attempt 1)

| Command / mechanism | Observed result |
|---|---|
| `./scripts/test-crash-alert-fixes.sh` | **13/13, exit 0**, twice — worktree copy and a fresh `git archive HEAD` extract run from the repo cwd. (The task's "12/12" was stale: test 13 was added at `e4fcbec` 2026-09-07.) |
| `./scripts/resource-monitor.sh` PSI crossings (sandboxed fixture) | Boundaries cross exactly: 70.00 → WARNING, 69.90 → OK; 80.00 → CRITICAL, 79.90 → WARNING (`printf "%d"` truncation then `-ge`). |
| MEM / DISK / CPU crossings (lowered-threshold sandbox copies) | MEMORY 54GB < 55 → CRITICAL; DISK 62GB < 80 → WARNING; CPU 8.55 ≥ 3 → WARNING — comparison operators behave as published. |
| live `./scripts/resource-monitor.sh --once`, unmodified | rc 0, all five checks OK. |
| cooldown/dedup driver (sandboxed, `bead` stubbed) | **16/16, exit 0**: genuine crash → alert rc 1 (INFRASTRUCTURE); second alert same classification suppressed by the **per-classification 300 s** cooldown (A2); independent **global** window still suppresses after backdating only that one (A3b); both backdated 400 s → refires (A3, window-not-wall); re-processing a processed bead suppressed (A4); `--force-alert` bypasses (A5); Closed-target alert suppressed (A6, both worktree and HEAD `efb1603`); 7-day target dedup suppresses fresh (A7) and expires at 8 days (A7b). |
| dedup gate with unreadable bead store | **fails open** — gate returns 3 → "failing open (proceed)"; ledger legs (A4/A7) and cooldowns still hold. |
| `./scripts/crash-classifier.sh` categories | FALSE_POSITIVE ×2 (max-turns trace; exit −1 with bead already Closed), SERVICE_FAILURE ×1, INFRASTRUCTURE ×3 (memory floor / repo-bloat / abnormal-exit fallback) all confirmed; benign real id exits 2 with "Bead trace not found". |

**Findings disclosed by the child (pre-existing, documented — not regressions):**

- **CODE_DEFECT is unreachable** in `crash-classifier.sh` (0 emitting branches): a panic-shaped
  trace classifies UNKNOWN. This is the same UNKNOWN gap CLAUDE.md records — automated UNKNOWN ≠
  no crash (a kill wave can take the worker before the crash record is written). Proposed fix:
  `docs/crash-fix-strategy-domchk-3b605127-2026-09-07.md` §3.2.
- Attempt 1's "closed-target gate absent at HEAD" contrast was **superseded**: its extract
  predated `efb1603`, which is exactly that fix; worktree and HEAD now agree.
- Worktree dirt disclosure: the scripts the criteria exercise (`test-crash-alert-fixes.sh`,
  `crash-classifier.sh`, `resource-monitor.sh`, `alert-deduplication.sh`) were identical to HEAD;
  co-tenant in-flight edits to other scripts were tested as-is and disclosed.

## Child 4 — Server safeguards on committed HEAD (domchk-ea18c7a8)

**Run window:** 2026-09-08 ~10:17–10:23Z · **Code state tested:** HEAD `017452a`, in a throwaway
`git archive HEAD` extract (not the shared worktree) · **Committed summary:**
[`docs/verification/domchk-ea18c7a8-safeguards-head-verification-2026-09-08.md`](verification/domchk-ea18c7a8-safeguards-head-verification-2026-09-08.md)
· **Raw logs:** `safeguards-*.log`

| Step | Observed result |
|---|---|
| `go build ./...` on pure HEAD | **rc 1** — `internal/server/server.go:117:13: undefined: NewResourceMonitor` |
| attribution (md5-attested A/B) | adding exactly the two co-tenant **untracked** files `resource_monitor.go` + `resource_monitor_test.go` to the extract flips the build to **rc 0** — the failure is uncommitted co-tenant work, not a defect in any committed byte |
| `go test ./internal/server/ -run 'TestSafeguard\|TestServer\|TestResourceMonitor' -count=1` | rc 0, **6/6 PASS** (0.205s) |
| `go test ./internal/server/ -count=1` (whole package) | rc 0, **151/151 PASS** (4.704s) |
| `DOMCHECK_RUN_LONG_TESTS` | unset — no long tests ran |

**Coverage nuance (disclosed by the child):** there is **no `TestSafeguard*` prefix** in the
codebase; the name pattern matches only 6 tests and under-covers the safeguards by 145. The
panic-recovery / timeout / signal-handling tests are named `TestRecover*`, `TestTimeout*`,
`TestSignalHandling`, `TestGracefulShutdown*`, etc. — cite the **whole-package 151/151 run** as
the safeguards result. The same caveat applies to any future run keyed on that pattern.

**Known open item (documented, pre-existing):** pure `main` has been unbuildable in isolation
since `e4fcbec` (2026-09-07) because `resource_monitor{,_test}.go` were never committed — the
documented F1 finding (commit `7879715`, CLAUDE.md "HEAD unbuildable" note). Green worktree runs
ride on the untracked files.

---

## Where the evidence lives

| Layer | Location | Tracked? |
|---|---|---|
| Committed per-child summaries | `docs/verification/domchk-{e0e1120e,6a227734,189d2f80,ea18c7a8}-*.md` | ✅ yes |
| Raw per-check logs | `.beads/state/crash-prevention-testing/*.log` | ❌ no — `.beads/` is wholly gitignored (`.gitignore:66`) by the bloat-prevention layer that ended bf-4yjq; never re-track it |
| Per-child commit summaries | child commits `dfda388`, `520c40e` (+`5d0c83f`), `017452a`, `0d1d8e1` | ✅ yes |

## Related documents

- Solution design: [`docs/crash-prevention-design.md`](crash-prevention-design.md) (@ `fb4d2f4`)
- Monitoring design: [`docs/crash-prevention-monitoring-design.md`](crash-prevention-monitoring-design.md) (@ `780115e`)
- Requirements inventory: [`docs/crash-prevention-requirements.md`](crash-prevention-requirements.md) (@ `1b21053`; amended since at HEAD)
- Consolidated bf-4yjq findings: [`docs/crashes/bf-4yjq-consolidated-findings-domchk-4ed0544b-2026-09-06.md`](crashes/bf-4yjq-consolidated-findings-domchk-4ed0544b-2026-09-06.md) (@ `778e2fd`)
- Operator runbook: [`docs/operations/crash-prevention-runbook.md`](operations/crash-prevention-runbook.md)
- bf-4yjq crash report §15 (this package in context): [`docs/crashes/bf-4yjq-crash-report.md`](crashes/bf-4yjq-crash-report.md)
