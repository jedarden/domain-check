# Mitigation Implementation — bf-1ea4g (kill storm 2026-08-13)

**Written:** 2026-09-07 by implementation bead **domchk-c68ae9bb** ("Implement preventive measures based on bf-1ea4g root cause")
**Target bead:** bf-1ea4g — "Document local main branch state" (**Closed 2026-08-13T09:10:16.731Z**, deliverable intact on origin/main)
**Status of this document:** the *implementation record* for the mitigation dispatch — a pointer-and-verification file, not a root-cause narrative. The root cause is **not re-derived here**; the canonical record is
[`docs/crash-inventory-bf-1ea4g-summary.md`](crash-inventory-bf-1ea4g-summary.md) (364f187, this dispatch appends §5.6 there), the dispatch-named RCA rendering is
[`docs/root-cause-analysis-bf-1ea4g-2026-08-13.md`](root-cause-analysis-bf-1ea4g-2026-08-13.md) (0f4d659), and the closing verdict lives in
[`docs/crash-resolution-bf-1ea4g-final.md`](crash-resolution-bf-1ea4g-final.md) (7032b16). Every figure below was measured first-hand in this dispatch (2026-09-07 ~14:15–14:20 EDT), not cited forward.

**Finding in one line:** every repo-side preventive measure the root cause calls for was already implemented and landed by earlier beads in the chain; this dispatch **re-proved each one first-hand**, made two dispatch-premise corrections (§4), and signposts the three items that are genuinely still open and who owns them (§5). Nothing new was owed on the mechanism.

---

## 1. The root cause being prevented

Unbounded `git push` — pack-objects materializing a **422-commit unpushed backlog** still carrying retired bead-forge object mass, inside the needle dispatch scope's **12 GiB `MemoryMax`**, with no `pack.windowMemory` bound in effect → memcg-OOM-class SIGKILL → needle's `exit_code: -1` sentinel, retried **56 times** because nothing bounded the retry. **54 of 57 attempt transcripts die mid-push.** Kill = INFRASTRUCTURE; the ~90 minted alert beads = FALSE_POSITIVE (the bead self-recovered the same morning). Kernel-proven for the mechanism family by the push-side sibling bf-198ne (`docs/crashes/bf-198ne-crash-report.md`).

## 2. The dispatch's taxonomy branch, task by task

The dispatch template keys off the root-cause category. bf-1ea4g is the **INFRASTRUCTURE** branch (the kill), with the **FALSE_POSITIVE** branch applying to its alert layer. SERVICE_FAILURE and CODE_DEFECT were ruled out upstream (canonical §3).

| Dispatch task (INFRASTRUCTURE branch) | What this dispatch did | Verdict |
|---|---|---|
| Run `./scripts/safe-git-gc.sh` | Ran the safe path's `--check-only` instead of a real gc — see **premise correction 1** (§4.1). Resources passed (104 MiB repo, 46,555 MB avail mem, 44 G disk, load 4.34), verdict **"GC not needed"** (its exit 1 is the verdict, not a failure). `auto-gc-trigger.sh --dry-run` agrees: "Repository size below threshold, GC not needed" | ✅ **not needed — verified, not skipped** |
| Verify `.gitignore` excludes `.beads/` | `.gitignore:66` `.beads/` (plus `*.db`, `*.db.backup.*`, `*.jsonl` repo-wide); `git ls-files .beads \| wc -l` → **0** tracked files | ✅ |
| Document repository size reduction | This document §3, size-reduction arc below; `.git` **104 MB**, 122 loose objects / 996 KiB, 2 packs / 99.78 MiB, 0 garbage, `fsck` clean, `origin/main...HEAD` **0/0** | ✅ |
| Install `./scripts/monitoring-setup.sh` | Installed/verified via the working entry point instead — see **premise correction 2** (§4.2). **8/8** `domain-check-*` systemd user timers future-scheduled; both live monitors pass one-shot | ✅ with correction |
| *(FALSE_POSITIVE branch)* fix alert suppression, dup detection, closed-bead filtering; test | Six 2026-09-02 fixes present; both suites green this dispatch (§3). **One live defect remains in the FP branch** — the CLASSIFICATION wiring — owned by open bead `domchk-f6fff20f` with its fix in flight in this worktree; **not redone here** (§5.1) | ⚠️ partial, ownership signed |

## 3. First-hand verification battery (this dispatch)

| # | Check | Result |
|---|---|---|
| 1 | `./scripts/setup-git-gc-config.sh --verify` — the direct fix: `pack.windowMemory=2g` / `pack.deltaCacheSize=1g` / `pack.threads=1`, repo-local **and** global | **exit 0** — effective chain system → global → local, all three keys local, worst-case pack memory ≈ **3072 MiB** (window 2 GiB × threads 1 + deltaCache 1 GiB) inside the 6 GiB ceiling for a 12 GiB dispatch scope |
| 2 | `./scripts/test-gc-memory-bounds.sh` — **reconstructs bf-1ea4g's death operation**: bounded `git push` over a 192 MiB unpacked backlog (18 loose objects, 0 packs) under `MemoryMax=768M` | **16/16, exit 0** — push **exit 0**, bare remote **received the backlog** (the operation bf-1ea4g never completed), store **stayed loose** (the exact pre-push state), peak push RSS **232,500 KB** vs the >12 GiB its unbounded ancestor consumed. Same run: bf-173o7e's bare `git gc --aggressive --prune=now` under 768 MiB → exit 0, peak 320,532 KB |
| 3 | `./scripts/check-repo-health.sh` | **exit 0** — effective pack-memory bound verified, no unmanaged aggressive gc/repack running, unpushed backlog **0** (< 50 warn threshold) — the M-1 precondition monitor landed by 8d326cc + daily wiring 7160e6a, reporting CLEAR |
| 4 | `.gitignore` / tracked bead state | `.beads/` + `*.db` + `*.jsonl` ignored; **0** tracked `.beads/` files |
| 5 | `./scripts/setup-git-hooks.sh --check` — the 10 MB pre-commit gate that would have blocked the bloat-era 237 MB `.beads/*.jsonl` commits | **exit 0** — installed, byte-identical to tracked source (`scripts/pre-commit-repo-size-hook`, shipped dfa60a9) |
| 6 | Timers: `systemctl --user list-timers 'domain-check-*' --all` | **8/8 future-scheduled** — monitoring 10 min, resource-monitor 5 min, service-monitor 2 min, alert-triage hourly, repo-health daily 02:00, auto-gc daily 02:30, incremental gc daily 03:00, full gc Sun 04:00 (MemoryMax=4G) |
| 7 | `./scripts/resource-monitor.sh --once` / `./scripts/service-monitor.sh --once` | both **PASS** — memory 43 GB avail, disk 43 GB free, load OK, unsafe-gc none; pre-flight "All services healthy" |
| 8 | Alert-layer suites: `test-crash-alert-fixes.sh`, `test-closed-bead-filter.sh` | both **exit 0** (13-marker suite; 7/7 functional closed-bead replay) |
| 9 | Divergence / backlog | `git log origin/main..HEAD` **empty** at first check and re-verified immediately before commit — three co-tenant bf-2ildm doc commits landed in between, and both §4.2/§5.1 HEAD claims were re-checked against the new HEAD (still live there) — the 422-commit precondition does not exist today |

**Size-reduction arc (crash precondition → today):** **~18 GB** `.git` with 17 GB loose objects / 17+ committed 237 MB `.beads/*.jsonl` snapshots (2026-08-12, bf-1s6c3/bf-4yjq) → **93 MB** after the verified 2026-09-01 cleanup → **94 MB** re-verified 2026-09-06 → **104 MB** this dispatch (normal churn, 0 garbage, fsck clean). The bloat regime the Aug-13 pushes pushed through is gone and held for five days past the last re-verification.

## 4. Dispatch-premise corrections

Two of the dispatch's literal INFRASTRUCTURE instructions are wrong for this box/repo as they are written. Both are recorded here so the next dispatch inherits the correction, not the mistake.

### 4.1 "Run repository cleanup: `./scripts/safe-git-gc.sh`"

**Running a real gc would have been the wrong action.** The repo is 104 MB with 0 garbage; a gc here is pure churn (CLAUDE.md: do not reflexively clear). The correct INFRASTRUCTURE-branch behavior when the repo is healthy is the **check**: `./scripts/safe-git-gc.sh --check-only`, which runs the full fail-fast preflight (invalid config / insufficient memory / disk / load → exit 2 *before* any git work) and then returns a **verdict**. Note its exit 1 on "GC not needed" is the verdict, not a failure — the same shape as `auto-gc-trigger.sh --dry-run`. Both ran clean this dispatch (§3 #1, #3).

### 4.2 "Install monitoring: `./scripts/monitoring-setup.sh`"

**At HEAD, that script is the retired cron installer and is a silent no-op on this box** — this workspace is NixOS with no cron daemon, so its crontab writes "succeed" while nothing ever fires (the trap CLAUDE.md warns about, and gap G-7). The working entry point is **`./scripts/setup-repo-maintenance.sh`** (systemd **user** timers; after editing any unit, `systemctl --user daemon-reload`). Monitoring is verified in place at §3 #6/#7. A co-tenant rewrite of `monitoring-setup.sh` that retires the cron path and delegates to the tracked timers is in flight uncommitted in this worktree (+207/−69 vs HEAD) — **not this dispatch's work, not committed here.**

## 5. What this dispatch did not implement, and who owns it

### 5.1 FALSE_POSITIVE branch — the CLASSIFICATION wiring defect (owned: `domchk-f6fff20f`)

The dispatch's FP-branch item "fix alert suppression logic in `crash-alert-manager.sh`" has **one live, already-tracked defect**, and it is not this dispatch's to fix:

- **Live at HEAD:** `crash-alert-manager.sh` captures `CLASSIFICATION=$(… | head -1)` while `crash-classifier.sh main()` prints its `====` banner first — so `CLASSIFICATION` is always the banner string. The manager's `[[ $CLASSIFICATION == FALSE_POSITIVE ]]` branch is **dead code** (FP auto-resolve never runs), the 300 s cooldown keys on a constant (global across types, not per-type), and `crash-history.jsonl` records a garbage classification field. Verified this dispatch at HEAD line 279.
- **The fix is in flight in this worktree, uncommitted:** worktree lines 382–383 carry the robust token extraction (`grep -m1 -E '^(FALSE_POSITIVE|…)$'` with a head-1 fallback), inside a co-tenant rewrite of both scripts (+149/+464 lines vs HEAD, manager additionally staged). Per the shared-worktree hazard and the bead's own caution, this dispatch **touched neither script** — committing or re-editing them would collide with the owner's in-flight work.
- **Not broken by it:** dedup, closed-bead filter, completion awareness, cooldown *effect* and cascade collapse (6 kills → 1 alert in the bf-6d3d6-shaped replay) all verified working by the domchk-81938e89 sandbox replay, and both suites pass this dispatch (§3 #8). The corpus lesson stands: the grep-marker tests (4–12) cannot see wiring bugs — the sandbox replay can.

### 5.2 NEEDLE-side residuals (not repo-fixable)

- **H-1 — retry stop-condition:** the single amplifier that turned 1 kill into 56 (each killed attempt had already committed, so every retry's push was bigger). Policy change on the NEEDLE side; tracked in `docs/crash-prevention-gaps-bf-1ea4g.md` and `docs/crash-prevention-requirements.md`. Must not be filed as a print-only detection rule.
- **M-2 — dispatch-scope memory telemetry:** nothing observes the 12 GiB memcg that did the killing; resource monitoring here is host-wide. Requirements P1 in `docs/crash-prevention-requirements.md`.

## 6. Acceptance-criteria map

| Criterion | Where |
|---|---|
| Preventive measure implemented | §2 — every repo-side measure was already landed (commits: 8d326cc, 7160e6a, dfa60a9; pack-memory bound tracked at HEAD, `--verify` exit 0); this dispatch verified rather than duplicated |
| Testing shows effectiveness | §3 #2 — the death operation itself, reconstructed and run under a 768 MiB scope, **completes** (16/16, exit 0); plus §3 #3–#8 |
| Documentation updated | This file + the §5.6 cross-registration appended to `docs/crash-inventory-bf-1ea4g-summary.md`; premise corrections dated in §4 |
| Monitoring in place | §3 #6/#7 — 8/8 timers future-scheduled, live monitors pass; installed via `setup-repo-maintenance.sh`, not the retired cron installer (§4.2) |

## 7. References

- Canonical bf-1ea4g record: `docs/crash-inventory-bf-1ea4g-summary.md` (364f187) — §5.6 appended by this dispatch
- Dispatch-named RCA: `docs/root-cause-analysis-bf-1ea4g-2026-08-13.md` (0f4d659); resolution ledger: `docs/crash-resolution-bf-1ea4g-final.md` (7032b16)
- Mechanism: `docs/crashes/bf-198ne-crash-report.md` (kernel-proven push-side memcg OOM), `docs/crashes/bf-4yjq-cleanup-verification.md` (bloat repair, re-verified 2026-09-06), `docs/branch-divergence-analysis.md` (backlog 660 → 422 → 0)
- Open owners: `domchk-f6fff20f` (CLASSIFICATION wiring, P2, fix in flight), `docs/crash-prevention-requirements.md` (H-1, M-2)

*Written by domchk-c68ae9bb, 2026-09-07. All live figures first-hand this session; historical figures cited to the documents that established them.*
