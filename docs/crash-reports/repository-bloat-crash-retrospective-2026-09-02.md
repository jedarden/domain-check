# Retrospective: Repository Bloat Crash — Investigation and Resolution

**Date:** 2026-09-02
**Documentation bead:** domchk-8acb7fd3
**Primary crash event:** bf-1s6c3 (2026-08-12) with follow-on crash storm bf-65lsdu (2026-08-13/14)
**Classification:** Infrastructure failure — OOM SIGKILL during git operations
**Code defects found in domain-check:** None
**Status:** ✅ RESOLVED and VERIFIED (re-verified 2026-09-02, see [Verification](#7-verification-report-2026-09-02))

---

## 1. Executive Summary

Between 2026-08-12 and 2026-08-17 the domain-check workspace suffered a cluster of
`exit code -1` (SIGKILL) crashes. Root cause was **repository bloat**, not application
code: 17+ identical ~237MB `.beads/*.jsonl` files had been committed to git history,
growing `.git` to ~18GB with ~17.2 GiB of loose objects (4,500+ objects). Any git
operation on that repository — merge, status, gc — inflated memory until the kernel
OOM killer terminated the agent process.

Resolution took two passes:

1. **Emergency cleanup (2026-08-16/17):** the cleanup itself crashed repeatedly
   (bf-65lsdu crash storm, 11+ exit-code -1 alerts) before a memory-capped run
   completed in 90.3s and shrank the repo 18GB → 138MB (−99.2%).
2. **Hardening (2026-08-16 → 2026-09-02):** `.gitignore` exclusion of `.beads/`,
   pre-commit large-file hook, memory-capped/serialized `safe-git-gc.sh`, repository
   health checks and continuous monitoring, followed by a scheduled maintenance gc
   (2026-08-26) and a verification pass (2026-09-01/02).

As of 2026-09-02 the repository is healthy and verified: **92MB `.git`, 0 loose
objects, 1 pack (90.19 MiB, 10,383 objects), `git fsck --full` clean.**

---

## 2. Crash Summary

### 2.1 Initial event — bf-1s6c3 (2026-08-12)

| Field | Value |
|-------|-------|
| First crash | 2026-08-12 21:36:51 UTC, during git merge reconciliation |
| Exit code | −1 (terminated by external signal; SIGKILL from OOM killer) |
| Crash count | 9+ crashes in 2.5 hours, identical signature |
| Beads affected | bf-1s6c3, bf-4x12ec, bf-4yjq, bf-173o7e |
| Repo state at crash | ~18GB `.git`, 17.16 GiB loose objects, 4,482 loose object files |

### 2.2 Crash storm — bf-65lsdu (2026-08-13/14)

The remediation bead "Run repository cleanup to eliminate 17GB bloat" itself became
the second incident: agents dispatched to run the cleanup were killed mid-`git gc`
because gc on 17GB of loose objects is itself the most memory-hungry operation on the
box. 11+ exit-code −1 alerts were raised (bf-1944k2, bf-1b5if7, bf-3k8oln, bf-12yvry,
bf-1akbgp, bf-1d28nt, bf-13y6q9, bf-1azq3i, bf-1dy0zp, bf-1cjg4f, bf-14uhmx); several
were duplicates or post-completion false positives, which motivated the crash-alert
deduplication/false-positive work of 2026-09-02.

The successful run completed 2026-08-17 00:34:00 UTC, exit 0, duration 90.3s.

---

## 3. Root Cause Analysis

**Mechanism (confidence: HIGH — direct evidence chain from metrics to OOM):**

1. `.beads/*.jsonl` issue-export files (~237MB each, 17+ copies) were committed
   because `.gitignore` did not exclude `.beads/`.
2. Repeated rewrite/commit cycles left the objects loose (unpacked) — ~17.2 GiB
   across 4,500+ files.
3. Every git command touching the object database (status/merge/gc) had to map and
   delta-scan that set; memory demand exceeded the box limit and the kernel OOM
   killer SIGKILLed the agent → harness reports `exit code -1`.
4. Cleanup attempts ran `git gc --aggressive` **on the bloated repo without a memory
   ceiling**, so the fix reproduced the crash — the bf-65lsdu storm.

**Contributing factors:**

- No `.gitignore` entry for `.beads/` at the time.
- No pre-commit guard against large files.
- No repository-size monitoring or pre-flight check before git-heavy work.
- No memory cap or serialization on gc runs (multiple agents gc'ing concurrently
  multiplied peak memory).
- Crash alerts lacked deduplication and post-completion false-positive detection,
  so one real incident generated a queue of duplicate investigation beads.

**Ruled out:** domain-check application code. Every investigation in this series
(200+ crash alerts reviewed as of 2026-09-02) found zero code defects; the Go binary
and its operations were unaffected bystanders.

---

## 4. Investigation Steps

1. **Pattern detection** — clustered the exit-code −1 alerts by timestamp; all
   coincided with git operations in one workspace.
2. **Repository state capture** — `du -sh .git` (18GB), `git count-objects -vH`
   (17.16 GiB loose, 4,482 objects), establishing the before snapshot.
3. **Object provenance** — identified the loose objects as `.beads/*.jsonl` blobs,
   17+ near-identical 237MB files from repeated bead exports.
4. **OOM confirmation** — kernel logs showed SIGKILL on the agent processes during
   git activity; memory pressure on the box was the limiting factor.
5. **Classification** — infrastructure event, not code defect (per
   `docs/crash-response-guide.md` categories).
6. **Controlled remediation** — cleanup rerun with memory limits after the unbounded
   attempts crashed; verified with `git fsck --full` post-cleanup.
7. **Recurrence watch** — follow-up baselines on 2026-08-26 and 2026-09-01, plus the
   monitoring/health-check scripts; safe-git-gc loose-object detection bug found and
   fixed with a regression test (commit 4737327, 2026-09-02).

Full evidence per bead: `docs/crashes/repository-bloat-crash-bf-1s6c3-2026-08-12.md`,
`docs/crash-information-bf-65lsdu.md`, and the bf-65lsdu RCA series.

---

## 5. Resolution

- **Cleanup:** loose objects packed and pruned under a memory cap; `.git` reduced
  18GB → 138MB (−99.2%); `git fsck --full` passed; bf-1s6c3's original operation
  (git merge) re-tested successfully afterwards.
- **Source of bloat removed:** `.beads/` excluded via `.gitignore` (line 66).
- **Guardrails added:** pre-commit hook blocking files >10MB (based on this
  incident's analysis), plus `prepare-commit-msg` hook.
- **Safe gc tooling:** `scripts/safe-git-gc.sh` — cgroup memory ceiling
  (`SAFE_GC_MEMORY_MAX`, default 2g), box-wide lock serializing concurrent gc runs,
  staged execution with checkpoint/resume, pre-flight integrity checks, and
  `--check-only`/`--full`/`--resume`/monitor modes.
- **Monitoring:** `scripts/check-repo-health.sh`, `repo-health-monitor.sh`,
  `preflight-health-check.sh`, resource/service/crash-pattern monitors, and cron
  installation via `scripts/monitoring-setup.sh`.
- **Alert quality:** 2026-09-02 crash-alert manager adds closed-bead filtering,
  duplicate detection, completion awareness, cooldown, and classification.

---

## 6. Before/After Repository Metrics

Canonical metric is `.git` size plus object breakdown (`git count-objects -vH`).
Working-tree totals are not comparable across dates (untracked caches, `.beads/`
store, build output) and are noted only where recorded.

| Date | Event | `.git` size | Loose objects | Packs | In-pack objects | Pack size |
|------|-------|------------:|--------------:|------:|----------------:|----------:|
| 2026-08-12 | 🔴 Crash (bf-1s6c3) | ~18 GB | 17.16 GiB / 4,482 files | — | — | — |
| 2026-08-16 | Emergency cleanup | 138 MB | ~eliminated | — | — | — |
| 2026-08-26 | Pre-gc baseline | 137 MB | 15 / 60 KiB | 1 | 6,863 | 136.01 MiB |
| 2026-08-26 | After `git gc --aggressive --prune=now` | 137 MB | **0** | 1 | 6,878 | 136.01 MiB |
| 2026-09-01 | Re-baseline | 91 MB | 17 / 48 KiB | 2 | 9,404 | 89.04 MiB |
| **2026-09-02** | **Verified (this report)** | **92 MB** | **0** | **1** | **10,383** | **90.19 MiB** |

Current status vs. the health thresholds in `CLAUDE.md`: total size 92MB
(healthy, <500MB), loose objects 0 bytes (healthy, <100MB), loose count 0
(healthy, <100), pack fragmentation 1 pack (acceptable), 1,633 commits.

Note on the "~1.4G" figure in `docs/git-gc-results-2026-08-26.md`: that measurement
included the working directory, not just `.git`. Against `.git`-only metrics the
2026-08-12 → today delta is **~18GB → 92MB (−99.5%)**, and the repo has stayed under
140MB continuously since the 2026-08-16 cleanup.

---

## 7. Verification Report (2026-09-02)

Commands run and results during this retrospective:

| Check | Command | Result |
|-------|---------|--------|
| Object database integrity | `git fsck --full` | Exit 0, no errors |
| Size | `du -sh .git` | 92M |
| Object stats | `git count-objects -vH` | count 0, in-pack 10,383, size-pack 90.19 MiB, garbage 0 |
| Pack files | `ls .git/objects/pack/` | 1 pack, 90M (pack-abdff25…) |
| Bloat source excluded | `grep beads .gitignore` | `.beads/` present (line 66) |
| Pre-commit guard | `.git/hooks/pre-commit` | Installed, MAX_SIZE_MB=10 |
| GC configuration | `git config` query | gc.auto=100, aggressivedepth=50, autopacklimit=50 |
| Health check | `./scripts/check-repo-health.sh` | 0 loose objects, 1 pack, fragmentation acceptable; flags only untracked `.beads/` store files (correctly ignored by git) |

**Verdict:** RESOLVED and holding. No bloat recurrence in the 17 days since cleanup;
all preventive controls verified active on 2026-09-02.

---

## 8. Lessons Learned

1. **The fix can reproduce the crash.** `git gc` is the most memory-hungry git
   operation; running it unbounded on a bloated repo caused a second crash storm.
   Remediation of a memory incident must itself be memory-capped and serialized.
2. **Ignore generated stores at the repo root, aggressively.** `.beads/` (and any
   local database/export directory) must be in `.gitignore` before the first commit,
   not after the first incident.
3. **Small files compound.** One 237MB file committed 17 times cost 17GB. Per-file
   size limits at commit time (the 10MB pre-commit hook) catch this cheaply.
4. **Record the baseline before touching anything.** The before/after table above is
   only possible because `git count-objects -vH` snapshots were taken first; the
   2026-08-26 chain (baseline → gc → verify → document) is the template to reuse.
5. **Loose objects are the leading indicator.** Count and size of loose objects
   climbed days before total size became critical — monitoring watches that ratio,
   not just total size.
6. **Alert hygiene is part of incident response.** The unbounded duplicate and
   false-positive alerts during bf-65lsdu delayed real work; deduplication,
   cooldown, and closed-bead filtering now run before an investigation bead is
   created.
7. **Exit code −1 is a symptom, not a cause.** It means "killed by signal"; the
   investigation must find which signal and why (here: OOM SIGKILL) before
   classifying the event.

---

## 9. Preventive Measures (Standing Controls)

| Control | Where | Purpose |
|---------|-------|---------|
| `.beads/` in `.gitignore` | `.gitignore:66` | Keeps bead stores out of git history (root-cause removal) |
| 10MB pre-commit block | `.git/hooks/pre-commit` | Prevents large-file commits |
| Memory-capped, serialized gc | `scripts/safe-git-gc.sh` (`SAFE_GC_MEMORY_MAX`, default 2g; box-wide lock; staged with resume) | gc can no longer OOM the box |
| Repository health check | `scripts/check-repo-health.sh`, `repo-health-monitor.sh` | Size / loose-object / fragmentation thresholds |
| Pre-flight check | `scripts/preflight-health-check.sh` | Memory/disk/load gate before agent tasks |
| Continuous monitoring (cron) | `scripts/monitoring-setup.sh` → `.beads/logs/*.log` | Resource, service, crash-pattern, repo-health alerts |
| Crash-alert quality gate | `scripts/crash-alert-manager.sh`, `crash-classifier.sh`, `alert-deduplication.sh` | No duplicate/false-positive investigation beads |
| Maintenance schedule | weekly `safe-git-gc.sh --check-only` + `check-repo-health.sh` (see `docs/maintenance/repository-maintenance-guide.md`) | Routine hygiene |

---

## 10. Remaining Risks

- **Uncommitted working tree:** several untracked investigation docs and modified
  scripts are present at any given time; they do not affect `.git` size until
  committed, but should not be left to accumulate.
- **`.beads/` store growth on disk:** the bead SQLite store, traces, and checkpoint
  files are large (multi-MB) but correctly git-ignored; disk pressure from them is a
  general disk-space concern, not a git-bloat one.
- **Concurrent agent gc:** mitigated by the safe-git-gc box-wide lock; bare
  `git gc --aggressive` outside the script remains unsafe and should not be used
  (per `CLAUDE.md`).

---

## 11. References

- `docs/crashes/repository-bloat-crash-bf-1s6c3-2026-08-12.md` — original crash report
- `docs/crashes/root-cause-analysis-bf-1s6c3-repository-bloat-2026-08-12.md` — RCA detail
- `docs/crash-information-bf-65lsdu.md` — cleanup crash-storm summary
- `docs/fix-proposal-bf-65lsdu-oom-git-gc-2026-09-02.md` — gc OOM fix proposal
- `docs/git-gc-baseline.txt`, `docs/git-gc-results-2026-08-26.md` — maintenance gc baselines/results
- `docs/crashes/crash-documentation-index-2026-09-02.md` — index of all crash documentation
- `docs/crash-response-guide.md` — classification guide
- `docs/maintenance/repository-maintenance-guide.md` — ongoing procedures
