# Crash Investigation Synthesis — bf-65lsdu (Final Report)

**Report Date:** 2026-09-02
**Synthesis Bead:** domchk-944eb8bb (this report)
**Alert Bead:** bf-4stk59 ("ALERT: Agent crash on bead bf-65lsdu")
**Crash Bead:** bf-65lsdu ("Run repository cleanup to eliminate 17GB bloat")
**Agent:** claude-code-glm-4.7
**Exit Code:** -1 (signal -1 / SIGKILL)
**Classification:** Infrastructure Event — repository bloat → OOM killer
**Domain-check Code Defects:** **NONE**
**Status:** ✅ RESOLVED — crash resolved 2026-08-17; this report closes the synthesis

---

## Executive Summary

On 2026-08-13 the agent assigned to bead `bf-65lsdu` was killed by the kernel OOM
killer while running `git gc --aggressive --prune=now` against a repository that had
bloated to ~18GB, of which **17.20 GiB (4,515 objects) was loose, unpacked objects**.
The aggressive repack's delta computation needed 10–20GB of working memory on top of
git's 2–4GB base footprint, exceeding what the box could supply; the SIGKILL surfaced
to the workflow as exit code -1. The crash repeated **11 times over ~3 hours** because
every retry re-ran the same deterministic memory-exhaustion failure.

The task ultimately succeeded on **2026-08-17T00:34:00Z** (exit 0) after the bead was
split into three small, independently verifiable child beads. The repository went from
~18GB to ~90–92MB — a **99.5% reduction** — with verified integrity. No domain-check
code was involved or changed; every prior investigation in this chain found the
application defect-free.

This report compiles the findings of all beads in the investigation chain, reconciles
discrepancies between the earlier write-ups, records the verified current repository
and system state, and identifies one **new open gap**: the systemd user schedulers
that were built to prevent recurrence are currently **failing to run** (PATH and unit
definition problems), so the bloat-prevention loop is not actually closed until they
are fixed.

---

## 1. What Happened

### 1.1 Setup

- **Bead `bf-65lsdu` created 2026-08-13T21:16:00Z** — "Run repository cleanup to
  eliminate 17GB bloat"; task: `git gc --aggressive --prune=now`; expected runtime
  30–60 minutes; fallback documented (`git repack -a -d --depth=250`).
- Repository state at that moment: ~18GB total, 17.20 GiB loose across 4,515 objects
  (~99% loose / ~1% packed — an inverted healthy ratio), the accumulated result of
  large artifacts (`.beads/` traces/data era) never being ignored or packed.

### 1.2 Crash storm — 2026-08-13 21:22 → 2026-08-14 00:20 (UTC)

Eleven recorded failures, all exit code -1 (SIGKILL), all during the same git
operation:

| # | Alert bead | Timestamp (UTC) | Disposition |
|---|------------|-----------------|-------------|
| 1 | (initial) | 2026-08-13T21:22:35 | initial crash during repack |
| 2 | bf-1b5if7 | 2026-08-13T21:30:32 | open when investigated |
| 3 | bf-1944k2 | 2026-08-13T21:48:30 | closed, false positive |
| 4 | **bf-4stk59** | **2026-08-13T21:32:12** | **the ALERT bead this report closes** |
| 5 | bf-3k8oln | 2026-08-13T22:14:36 | closed, false positive |
| 6 | bf-12yvry | 2026-08-13T22:20:09 | closed, duplicate |
| 7 | bf-1akbgp | 2026-08-13T22:39:42 | closed, duplicate |
| 8 | bf-1d28nt | 2026-08-13T22:57:42 | closed, duplicate |
| 9 | bf-13y6q9 | 2026-08-13T23:10:13 | closed, duplicate |
| 10 | bf-1azq3i | 2026-08-13T23:45:15 | closed, duplicate |
| 11 | bf-1dy0zp | 2026-08-13T23:56:16 | closed, duplicate |
| 12 | bf-1cjg4f | 2026-08-14T00:20:11 | closed, duplicate |
| 13 | bf-14uhmx | 2026-08-14T00:14:42 | closed, duplicate |

(The alert beads slightly outnumber the distinct failures because several were raised
for the same retry — the alert system's duplicate detection postdates this incident.)

Every failure is the same event signature: OOM killer → SIGKILL → exit -1. Retrying
never had a chance of working: the memory requirement is a deterministic function of
the repository contents, and nothing between retries changed those contents.

### 1.3 Recovery — 2026-08-16 → 2026-08-17

- **2026-08-16** — a successful `git gc --aggressive` run recorded a single optimized
  pack (750.53 MiB), described in commit message form as "Before: 527M .git, 163 loose
  objects (3 pack files) / After: 752M .git, 0 loose objects (1 optimized pack file)".
  (See §4 on the provenance of that commit reference.)
- **2026-08-17T00:32–00:34Z** — the durable, verified resolution: bead `bf-65lsdu` was
  split into three child beads and the parent converted to an umbrella:
  1. `domchk-bdb1fedf` — document current repository state (baseline)
  2. `domchk-af4b5ef4` — execute git gc cleanup
  3. `domchk-87be56d8` — verify and document cleanup results
  Final run: 2026-08-17T00:34:00Z, exit 0, 90.3 seconds. Parent `bf-65lsdu` closed at
  00:45:33Z.
- Each child succeeded where the monolithic task had failed eleven times because **no
  single step required the memory footprint of a full aggressive repack**.

### 1.4 Aftermath investigations — 2026-08-26 → 2026-09-02

Sustained-health verification, root-cause formalization, crash-alert system fixes
(closed-bead filtering, duplicate detection, completion awareness, 5-minute cooldown,
classification), and context investigations. All are compiled in §3.

---

## 2. Root Cause Analysis

*(Formalized by child investigation `domchk-2ab71440` in
`docs/research/root-cause-analysis-bf-65lsdu-crash-2026-08-13.md`.)*

### 2.1 Causal chain

```
Large artifacts not excluded from git + no scheduled gc
        │
        ▼
Repository bloat: ~18GB total, 17.20 GiB loose objects (4,515 objects, ~99% loose)
        │
        ▼
git gc --aggressive --prune=now  (delta optimization across ALL loose objects)
        │
        ▼
Working memory demand ≈ 10–20GB on top of git's 2–4GB base
        │
        ▼
Peak demand exceeds available memory on the shared box
        │
        ▼
Kernel OOM killer → SIGKILL → agent recorded as exit code -1   ← ×11 retries
```

### 2.2 Why exit code -1

Exit code -1 is the infrastructure-event signature per `docs/crash-response-guide.md`:
the process did not exit voluntarily — it was terminated by external force (here, the
kernel OOM killer's SIGKILL), and the agent process died with its git subprocess.

### 2.3 Classification

| Classification | Probability | Evidence |
|---|---|---|
| **Infrastructure Event** | 70% | Repository bloat; OOM during git operation; permanently resolved by cleanup |
| Workflow Failure | 20% | 11 identical retries before the bead-split strategy worked |
| Service Failure | 8% | Not applicable — no external service dependency |
| Code Defect | 2% | **Zero evidence** — crash was in a git subprocess, not domain-check code |

### 2.4 Why it was not a code defect

1. The crash occurred inside `git gc`, not in any domain-check code path.
2. Transient nature — crashes stopped permanently once the repository was cleaned.
3. Every comprehensive investigation in this workspace has found zero domain-check
   defects; the codebase is stable.
4. The resolution required resource cleanup, not a single code change.

### 2.5 Origin of the bloat, and the recurrence family

The 17GB accumulated because large artifacts (consistent with un-ignored `.beads/`
data and traces) were committed while no garbage-collection schedule existed. The same
failure mode produced the wider bloat-crisis family:

| Bead | Task | Outcome |
|---|---|---|
| bf-1ea4g | documentation task | SIGKILL during git ops, recovered |
| bf-4yjq | git remote fix | 9 OOM crashes from an 18GB repo, recovered |
| bf-1s6c3 | git reconciliation | 18GB repo → OOM; cleanup 18GB → 138MB |
| **bf-65lsdu** | **repository cleanup itself** | **11 OOM crashes, recovered via bead split** |

The irony of bf-65lsdu is instructive: **the cleanup task for the bloat was itself the
heaviest memory operation ever attempted on the bloat**. Cleaning an N-gigabyte loose
set with an aggressive in-memory repack is the single most memory-intensive git
operation available, and it was being run without a memory cap.

---

## 3. Compiled Findings from the Investigation Chain

| Bead | Role | Key contribution | Document |
|---|---|---|---|
| **bf-4stk59** | ALERT for the crash | Recorded the 21:32:12Z crash; closed with investigation summary after resolution | (bead notes) |
| **bf-1mcxco** | First crash investigation | Established SIGKILL-during-gc hypothesis, recovery evidence, systemic bloat context | `docs/crash-investigations/bf-65lsdu-crash-investigation.md` |
| **bf-ncs0ev** | Investigation bead (2026-08-26) | Full timeline, verified post-recovery repo (139MB, fsck clean), lessons learned, operational rules | `docs/research/crash-analysis-bf-ncs0ev.md` |
| **domchk-2ab71440** | Root cause (child bead 1) | Formal RCA: bloat → OOM, classification, why-not-code-defect | `docs/research/root-cause-analysis-bf-65lsdu-crash-2026-08-13.md` |
| **domchk-bdb1fedf** | Baseline (child bead 2a) | Post-recovery baseline: 91M .git, 40 loose, 9,076 in-pack, 3 packs | `cleanup-baseline.txt` |
| **domchk-af4b5ef4** | Cleanup execution (child bead 2b) | Executed the successful cleanup | — |
| **domchk-87be56d8** | Verification (child bead 2c) | Final state 2026-09-01: 90M, 4 loose, 1 pack; integrity verified | `cleanup-results.txt` |
| **domchk-7e6c4b21** | Context investigation | Reconstructed what the agent was doing at crash time; added bf-3k8oln to the timeline | `docs/investigation-bf-65lsdu-agent-context-2026-09-02.md`, `docs/crash-information-bf-65lsdu.md` |
| **domchk-944eb8bb** | **This synthesis** | Compiles all of the above; verifies current state; identifies open config gaps | this file |

**Consolidated lessons (from across the chain):**

1. **Retrying deterministic resource exhaustion never works.** Eleven identical
   failures; the fix was changing the approach (decompose), not retrying.
2. **Decompose oversized operations.** The bead split (baseline → execute → verify)
   succeeded where the monolithic run could not.
3. **Never run bare `git gc --aggressive` on a bloated repository.** Use the
   memory-limited, resumable, monitored `scripts/safe-git-gc.sh`.
4. **Verify work completion before treating a crash as task failure.** Several alert
   beads for ultimately-successful work were false positives.
5. **Monitor repository size continuously.** Every threshold in the maintenance guide
   was exceeded long before the first OOM.
6. **Prevent bloat at the source.** `.gitignore` for `.beads/` artifacts and pre-commit
   size limits.

---

## 4. Documentation Discrepancies Reconciled

The chain's reports disagree on a few details; recorded here so future readers don't
re-litigate them:

1. **Repository size after cleanup varies across documents** — 752MB (2026-08-16),
   139MB (2026-08-26), 90–97MB (2026-09-01), 92MB (measured today, §5). All were
   accurate when written: the 752MB figure predates later pack consolidation and
   pruning (3 packs → 1 pack between 08-26 and 09-01 accounts for most of the
   difference). The trend is monotonic improvement.
2. **The "completion commit" `5bf23b7` (2026-08-16) is not reachable in current
   history.** It is cited by the bf-1mcxco investigation as the successful-cleanup
   commit, but `git log` cannot resolve it today — it was most likely on a branch or
   working state that was later dropped, or itself a victim of subsequent history
   cleanup. The **authoritative recovery record is the bead trail** (bf-65lsdu split +
   child beads + traces in `.beads/traces/bf-65lsdu/`), which is internally consistent.
3. **Crash timestamps differ across documents** (21:22:35 / 21:27:56 / 21:30:32 /
   21:32:12 on 2026-08-13). These are *different events of the same retry storm*, not
   contradictions — each alert bead carries its own timestamp; bf-4stk59's is 21:32:12.

---

## 5. Current Repository State (verified 2026-09-02, this synthesis)

```
$ du -sh .git                  → 92M
$ git count-objects -vH
  count: 25            (loose objects, 192 KiB)
  in-pack: 10324
  packs: 1
  size-pack: 90.11 MiB
  prune-packable: 0
  garbage: 0
$ git fsck --connectivity-only → clean (exit 0)
$ ./scripts/safe-git-gc.sh --check-only → "GC not needed"
$ grep .beads/ .gitignore      → line 66: .beads/   (excluded)
```

| Metric | Crash time (2026-08-13) | Now (2026-09-02) | Status |
|---|---|---|---|
| Total repository size | ~18GB | 92MB | ✅ healthy (<500MB target) |
| Loose objects | 17.20 GiB / 4,515 | 192 KiB / 25 | ✅ healthy (<100MB / <100 target) |
| Packs | ~0 effective | 1 (90.11 MiB, 10,324 objects) | ✅ consolidated |
| Garbage | — | 0 | ✅ |
| Integrity | — | fsck clean | ✅ |

**System state at verification:** 62Gi RAM total / 48Gi available; 97G disk free;
load average 22.6 (1-min) — elevated by co-tenant agent activity on this shared box,
which is itself a reason to keep heavy git operations capped and off-peak (§8).

---

## 6. System Constraints (for future reference)

**The box:** Dell OptiPlex 3000 Micro, 12 cores / 62G RAM / single 444G root disk
shared by **all** repos and agent workspaces on this machine. Every GB a git operation
consumes is taken from co-tenant agents; the global OOM killer picks whatever pushes
memory over the limit — in this incident it picked the git process, but it can pick an
unrelated agent instead (which is how exit -1 crashes spread across beads).

| Constraint | Value / rule |
|---|---|
| Memory available for heavy ops | ≥ 10GB free required before starting; abort below 10GB |
| Disk free | ≥ 20GB required; abort below |
| Load (1-min) | < 10 preferred; this box routinely runs higher with many agents |
| Bare `git gc --aggressive` | **Forbidden** on any repo >1GB; use `scripts/safe-git-gc.sh` |
| `scripts/safe-git-gc.sh` | Memory-capped (`SAFE_GC_MEMORY_MAX`, default 2g), checkpoint/resume, pre-flight integrity check, monitor via `scripts/safe-git-gc-monitor.sh --watch` |
| Repo size thresholds | <500MB healthy / 500MB–1GB warning / >1GB critical → immediate cleanup |
| Loose-object thresholds | <100MB / <100 objects healthy; >500MB or >1,000 critical |
| Retry policy | A second identical exit -1 failure means **stop and decompose the task** (bead split), not retry |
| `.beads/` | Excluded via `.gitignore`; never commit traces/data |
| Fork-heavy cleanup tasks | Split into baseline → execute → verify child beads |

---

## 7. Recommended Remediation Steps

**Completed (no further action):**

- [x] Repository cleaned: ~18GB → 92MB, single pack, fsck clean, integrity verified
- [x] Root cause identified and classified (infrastructure event, no code defect)
- [x] `.gitignore` excludes `.beads/` (line 66)
- [x] `scripts/safe-git-gc.sh` (memory-limited, resumable) + monitor script available
- [x] Pre-flight health check script (`scripts/preflight-health-check.sh`)
- [x] Crash alert system hardened (closed-bead filter, dedup, cooldown, classification)
- [x] All 13 alert beads from the storm resolved; parent bead closed 2026-08-17
- [x] Investigation chain documented end-to-end

**Outstanding (owner: operator / future bead):**

- [ ] **Fix the systemd user schedulers — see §8.** This is the only open item that
      materially affects recurrence risk.
- [ ] Install pre-commit hook blocking >10MB files (`scripts/setup-git-hooks.sh`) if
      not already active for this working copy.
- [ ] After the schedulers are fixed, verify one full cycle: repo-health run at 02:00
      and git-gc run at 03:00 succeeding in the journal.

---

## 8. System Configuration Changes Needed (new findings, 2026-09-02)

The prevention tooling exists but **the schedulers that run it are currently broken**.
Discovered during this synthesis:

### 8.1 systemd user units fail: PATH is unusable

The systemd **user manager's PATH is only the systemd store directory**:

```
$ systemctl --user show-environment | grep ^PATH
PATH=/nix/store/kiplbb6yv7rmjf21hf9ky01b9kmgmnqn-systemd-257.10/bin/
```

Scripts use `#!/usr/bin/env bash`, and `bash`, `git`, and `jq` all live under
`/run/current-system/sw/bin` — not on that PATH. Observed failures:

- `domain-check-git-gc.service` → **status=127** (command not found), daily at 03:00,
  dying in 3ms / 1.4MB — the interpreter was never found.
- `domain-check-monitoring.service` (crash-pattern detection, every 5 min) →
  **status=1**; the script runs fine interactively, so its failure is also
  environment-shaped.
- (`domain-check-service-monitor` and `domain-check-resource-monitor` fire on the same
  timers and should be re-verified after the fix.)

**Fix (either form, in each unit's `[Service]` section):**

```ini
Environment=PATH=/run/current-system/sw/bin:/home/coding/.local/bin:/home/coding/.nix-profile/bin
```

or bypass PATH lookup entirely:

```ini
ExecStart=/run/current-system/sw/bin/bash /home/coding/domain-check/scripts/safe-git-gc.sh
```

Then: `systemctl --user daemon-reload && systemctl --user start domain-check-git-gc.service`
and confirm success in `journalctl --user -u domain-check-git-gc.service`.

### 8.2 `domain-check-git-gc-full.service` has an invalid `User=` setting

`systemctl --user cat domain-check-git-gc-full.service` shows `User=%i` in a
`systemd --user` unit, where `User=` is invalid — systemd marks the unit
**bad-setting** and refuses to run it. (The sibling `git-gc.service` even carries a
comment saying exactly this.) **Fix: delete the `User=%i` line.** Note this unit file
exists both in `~/.config/systemd/user/` and in the repo at
`scripts/domain-check-git-gc-full.service` — fix the repo copy too, and keep the two
in sync.

### 8.3 Cron-based monitoring instructions are inoperative on this box

`crontab` does not exist on this NixOS machine, so the cron-based setup documented in
project docs (`./scripts/monitoring-setup.sh` writing a crontab) **cannot work as
written**. The correct mechanism here is systemd user timers — which already exist for
this repo:

| Timer | Schedule | Backing service | Status 2026-09-02 |
|---|---|---|---|
| `domain-check-service-monitor.timer` | every 1 min | service-monitor.service | firing |
| `domain-check-monitoring.timer` | every 5 min | monitoring.service | firing but **service fails** (§8.1) |
| `domain-check-resource-monitor.timer` | every 5 min | resource-monitor.service | firing |
| `domain-check-repo-health.timer` | daily 02:00 | repo-health.service | inactive/dead (awaiting next fire) |
| `domain-check-git-gc.timer` | daily 03:00 | git-gc.service | firing but **service fails 127** (§8.1) |

**Action:** after applying §8.1/§8.2, documentation that recommends cron-based
monitoring should be updated to point at these timers instead.

### 8.4 Why this matters for recurrence

Repository bloat was the root cause of this incident and of bf-4yjq and bf-1s6c3. The
defense against recurrence is the daily 03:00 gc and the 02:00 repo-health check.
Both have been silently failing — meaning the bloat-prevention loop is **open**. Until
§8.1/§8.2 are applied, the primary protection is the alert-threshold checks that only
run when a human or agent invokes them manually.

---

## 9. Verification Checklist (post-fix)

```bash
# Units load without bad-setting
systemctl --user list-units 'domain-check*' --no-pager

# git-gc runs to completion (Status "0", not 127)
systemctl --user start domain-check-git-gc.service
journalctl --user -u domain-check-git-gc.service -n 20
tail -5 .beads/logs/git-gc.log

# crash-pattern detection exits 0 under systemd
systemctl --user start domain-check-monitoring.service
journalctl --user -u domain-check-monitoring.service -n 10

# Repository stays healthy across a week of timer cycles
./scripts/check-repo-health.sh
```

---

## 10. Closure Note (added to bead bf-4stk59)

> Final synthesis (domchk-944eb8bb) compiled the full investigation chain for the
> bf-65lsdu crash. What happened: 11 SIGKILL (exit -1) events on 2026-08-13–14 during
> `git gc --aggressive` on an ~18GB repository with 17.20 GiB loose objects; kernel
> OOM killer terminated the git subprocess and the agent with it. Root cause:
> repository bloat + uncapped memory-intensive git operation — infrastructure event;
> zero domain-check code defects. Resolution: bead split into three verifiable child
> beads (domchk-bdb1fedf / domchk-af4b5ef4 / domchk-87be56d8); success 2026-08-17
> 00:34Z; repository ~18GB → 92MB (99.5%), fsck clean. Current state verified
> 2026-09-02: 92MB .git, 25 loose objects (192 KiB), 1 pack (90.11 MiB, 10,324
> objects), no garbage, gc not needed. Full report:
> docs/research/crash-investigation-synthesis-bf-65lsdu-2026-09-02.md. One open item
> for the operator: the systemd user schedulers (daily git-gc timer, crash-pattern
> monitoring) are currently failing — user-manager PATH lacks bash/git/jq, and
> domain-check-git-gc-full.service carries an invalid `User=%i` — so the automated
> bloat-prevention loop needs the PATH/Environment fix in §8 of the report.

---

## 11. Related Documentation

**This incident:**
- `docs/crash-information-bf-65lsdu.md` — crash metadata and full alert-bead table
- `docs/research/root-cause-analysis-bf-65lsdu-crash-2026-08-13.md` — formal RCA (domchk-2ab71440)
- `docs/research/crash-analysis-bf-ncs0ev.md` — recovery report (bf-ncs0ev)
- `docs/investigation-bf-65lsdu-agent-context-2026-09-02.md` — agent context (domchk-7e6c4b21)
- `docs/crash-investigations/bf-65lsdu-crash-investigation.md` — first investigation (bf-1mcxco)

**General / prevention:**
- `docs/crash-response-guide.md` — classification and response procedures
- `docs/maintenance/repository-maintenance-guide.md` — thresholds and daily maintenance
- `docs/comprehensive-crash-prevention-guide.md` — systemic prevention
- `docs/crash-alert-fix-implementation-2026-09-02.md` — alert system hardening
- `scripts/safe-git-gc.sh`, `scripts/check-repo-health.sh`, `scripts/preflight-health-check.sh`

---

## 12. Conclusion

The bf-65lsdu crash was a pure infrastructure event: an ~18GB repository with 17.20 GiB
of loose objects met an uncapped `git gc --aggressive`, and the kernel OOM killer
killed the git subprocess — eleven times, because retries changed nothing. The
resolution (task decomposition into baseline → execute → verify child beads) reduced
the repository 99.5% to 92MB with verified integrity, and domain-check code was never
implicated: it remains defect-free.

The remaining risk is not the crash itself but **silence in the prevention loop** — the
daily gc and monitoring timers built after this incident are currently failing on
environmental grounds (§8). Applying the PATH/Environment fix, removing the invalid
`User=` setting, and confirming one clean timer cycle closes the loop for good.

**Status:** ✅ SYNTHESIS COMPLETE — findings compiled, current state verified,
remediation defined, constraints documented, bf-4stk59 closure note delivered.
