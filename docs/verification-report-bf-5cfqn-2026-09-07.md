# Verification Report: Crash Alert bf-5cfqn

**Date:** 2026-09-07
**Alert Bead:** bf-5cfqn — "ALERT: Agent crash on bead bf-1s6c3" (Open, P2)
**Original Crash Bead:** bf-1s6c3 — "Create merge commit reconciling Forgejo and GitHub histories" (**Closed**)
**Determination:** **DUPLICATE** — one alert bead among the 2026-08-12→13 bf-1s6c3 memcg-OOM storm; the crash was already investigated and resolved
**Original Alert (first of the storm):** bf-oplew, created 2026-08-12T21:36:51.250595813Z
**This report's bead:** domchk-06c4e978 (documentation blocker — the last open blocker gating bf-5cfqn's closure)
**Resolution authority:** [docs/crash-analysis-bf-1s6c3-2026-09-06.md](crash-analysis-bf-1s6c3-2026-09-06.md) (canonical; §12 holds every chain subsection's first-hand verification)

## Executive Summary

bf-5cfqn is a **duplicate alert**. It is one of the alert beads needle emitted for the
bf-1s6c3 crash storm of 2026-08-12→13, in which an ~18 GB repository (~17 GB of loose
objects from committed `.beads/*.jsonl` snapshots) drove repeated kernel memcg-OOM
SIGKILLs of git operations inside 12 GiB dispatch scopes. The subject task (bf-1s6c3)
was completed and the bead closed; the canonical analysis is
`docs/crash-analysis-bf-1s6c3-2026-09-06.md`.

bf-5cfqn's named crash instant (`2026-08-13T00:28:36.425389752+00:00`) is **not a
distinct crash** — it is attempt 58's kill plus a 6.229 s release heartbeat, re-proven
first-hand below from the committed raw extract. Root cause is infrastructure
(repository-bloat-era memcg OOM); **zero domain-check code defects**, consistent with
every investigation in this workspace's record.

This report is the deliverable of **domchk-06c4e978**, the one open dependency blocking
bf-5cfqn. bf-5cfqn has already been closed five times on this same duplicate
determination (2026-08-16, 2026-08-17, 3× on 2026-08-26) and each close auto-reopened
while its closure blockers were open — with this bead closed, the next close can stick
(the pattern already demonstrated on sibling alert bf-kk87a, commit 8d44ac8).

## 1. Crash Details (from the alert record)

| Field | Value |
|---|---|
| Alert bead | bf-5cfqn (labels: `alert`, `crash`, `signal--1`, `umbrella`) |
| Created | 2026-08-13T00:28:36.431534024Z |
| Alerted bead | bf-1s6c3 (merge commit reconciling Forgejo/GitHub histories) |
| Agent | claude-code-glm-4.7 |
| Exit code | −1 (signal −1 — sentinel for a kernel SIGKILL, not a signal number) |
| Named instant | 2026-08-13T00:28:36.425389752+00:00 |
| Workspace | `.` (this repo) |

## 2. Investigation Findings

### 2.1 The named instant is not a distinct crash — re-verified first-hand 2026-09-07

Re-read this session, byte-identical to canonical §12, from the committed extract
[docs/crashes/bf-1s6c3/needle-events-2026-08-13-bf-1s6c3.jsonl](crashes/bf-1s6c3/needle-events-2026-08-13-bf-1s6c3.jsonl)
(attempt 58's window is L151 → L168):

| Event | Timestamp (UTC) | Record |
|---|---|---|
| Attempt 58 claims bf-1s6c3 | 00:23:44.047163899 | `bead.claim.succeeded`, seq 6249 (L151) |
| **The real kill** | **00:28:30.196295497** | `agent.completed`, `exit_code=-1`, `duration_ms=285898`, seq 6261 (L158) |
| Release heartbeat | 00:28:36.425380287 | `heartbeat.emitted`, `HANDLING_RELEASE_DONE`, seq 6270 (L166) |
| **bf-5cfqn created** | **00:28:36.431534024** | = named instant +5.3 µs clock-provenance drift; heartbeat **+6.15 ms** |

The kill→heartbeat gap is 6.229 s, so the alert's timestamp is the
`HANDLING_RELEASE_DONE` heartbeat (kill + 6.229094255 s), not a crash instant. Attempt
58 is the 58th of 76 `agent.completed` records (census: 71 × exit −1, 4 × exit 124,
1 × exit 0) — one of the 71 identical memcg-OOM kills, killed while running
`git push origin main` (canonical §12, immediate-cause subsection: kill = push + 14.701 s).

**Correction this finding carries:** the original root-cause section of
[docs/bead-verification/bf-5cfqn.md](bead-verification/bf-5cfqn.md) claims "agent
timeout (600s) … no OOM condition, pure timeout issue." That is **false and
superseded** (dated correction appended 2026-09-07 by domchk-9c040403, commit 805e796):
it contradicts the alert's own `exit_code=-1` (timeouts are the exit-124 class —
4 of 76 attempts), and it came from the superseded 2026-09-01 corpus. Any report
still citing the timeout attribution is stale.

### 2.2 Duplicate determination

- **Duplicate.** Original alert bead: **bf-oplew** (created 2026-08-12T21:36:51.250595813Z,
  alerting the storm's first kill at 21:36:51.240046999Z — verified live this session).
- Storm size: 76 dispatches of bf-1s6c3 (71 × exit −1 kills, 4 × exit 124 timeouts,
  1 × exit 0 success). Pre-0.4.2 needle emitted **one ALERT bead per kill**, and a
  first-hand count of the current bead store finds **71 distinct ALERT-titled beads**
  for bf-1s6c3 — matching the 71 kills 1:1 (the closed sibling determination
  domchk-9c040403 cites "76 alert beads", which is the full dispatch count).
- **Subject bead bf-1s6c3: Closed.** Its notes carry the 2026-09-07 correction from
  umbrella domchk-b79733ba (closed 2026-09-07): repository bloat → OOM during git
  reconciliation, task completed after the 18 GB → ~100 MB cleanup, no code defect.

**Alert lifecycle — five closes, five auto-reopens** (from
`.beads/checkpoint/forensic.jsonl`, read this session):

| # | Closed (UTC) | Reopened (UTC) | Close reason (abbreviated) |
|---|---|---|---|
| 1 | 2026-08-16T14:52:48Z | 2026-08-16T14:53:09Z | "Crash investigation complete… bf-1s6c3 completed before the crash" |
| 2 | 2026-08-17T10:15:03Z | 2026-08-17T10:15:11Z | "OOM killer (SIGKILL) due to repository bloat… bf-1s6c3 completed" |
| 3 | 2026-08-26T12:27:33Z | 2026-08-26T12:28:15Z | "Duplicate alert for resolved crash bf-1s6c3" (→ `failure-count:1`) |
| 4 | 2026-08-26T12:32:27Z | 2026-08-26T12:32:45Z | "Duplicate alert… completed successfully after timeout crash" (→ `failure-count:2`) |
| 5 | 2026-08-26T12:34:01Z | 2026-08-26T12:34:16Z | "Duplicate alert… completed successfully" (→ `failure-count:3`) |

Close #4's reason repeats the superseded timeout claim — the corrections in §2.1 are
what the final close reason should carry instead. After close #5 the system attached
the split chain: `dependency_added` making **domchk-06c4e978 (this report's bead) the
blocker of bf-5cfqn** (2026-08-26T12:34:38Z) alongside determination sibling
domchk-9c040403 (closed 2026-09-07, commit 805e796). The closes bounced because the
blockers were open — not because the determination was wrong; every one of the five
determinations was already correct.

### 2.3 Root cause analysis

- **Root cause:** kernel memcg-OOM SIGKILL (`CONSTRAINT_MEMCG`) of `git push`'s
  pack-objects on the then-~18 GB repository, inside the 12 GiB per-dispatch systemd
  scope. The host was never out of memory — the constraint was the cgroup; the kill is
  uncatchable and recorded as `exit_code=-1`.
- **Amplifier:** needle's ~10 s no-backoff re-dispatch loop turned one recurring OOM
  into 76 dispatches / 71 kills across ~4.5 h, and one alert bead per kill turned each
  kill into its own investigation — the duplicate-alert pool this bead belongs to.
- **Contributing condition (since repaired):** 17+ identical ~237 MB `.beads/*.jsonl`
  snapshots committed to git → ~17 GB of loose objects. `.beads/` is now gitignored
  (0 tracked files), and the persistent `pack.windowMemory=2g` / `pack.threads=1`
  config bounds pack-objects under both `gc` and `push`.
- **Classification:** INFRASTRUCTURE (repository-bloat era). **No domain-check code
  defect** — consistent with 157+ investigations in this workspace, none of which
  found an application defect.

## 3. Current State (live-verified 2026-09-07, this session)

| Check | Result | Healthy threshold |
|---|---|---|
| `.git` size | **103 MB** | < 500 MB |
| Loose objects | 144 / 1.60 MiB | < 100 MB |
| Packed objects | 11,360 in 1 pack, 99.11 MiB | consolidated |
| Garbage | 0 bytes | 0 |
| `git fsck --full` | exit 0 (dangling-only) | clean |
| `git ls-files .beads \| wc -l` | **0** | 0 |
| `HEAD...origin/main` | **0 / 0** | 0 / 0 |
| bf-1s6c3 | Closed | closed |
| bf-5cfqn | Open (`failure-count:4`, `verification-failed`) — blocked only by this report's bead | — |

## 4. Document Lineage (which bf-5cfqn doc to trust)

| Document | Status |
|---|---|
| [docs/crash-analysis-bf-1s6c3-2026-09-06.md](crash-analysis-bf-1s6c3-2026-09-06.md) | **Canonical** — §12 verifies every chain subsection |
| [docs/bead-verification/bf-5cfqn.md](bead-verification/bf-5cfqn.md) | 2026-08-26 verification report; its root-cause section is **superseded** by the 2026-09-07 correction appended in-place (commit 805e796) |
| [docs/crash-investigations/bf-5cfqn-duplicate-alert-resolution.md](crash-investigations/bf-5cfqn-duplicate-alert-resolution.md) | 2026-08-26 companion; already carried the correct OOM root cause, but cites a superseded primary investigation |
| **This file** | Named deliverable of the documentation blocker (pattern `docs/verification-report-<alert>-YYYY-MM-DD.md`); where older docs conflict, the canonical report and §2.1 above win |

## 5. Recommendations

### Preventing the crash class (repository-bloat memcg-OOM)

All four layers below are already in force — keep them, do not run bare git memory
operations outside them:

1. **Gitignore bead state** — `.beads/`, `*.db`, `*.jsonl` repo-wide; verify with
   `git ls-files .beads | wc -l` → 0. This is the fix that ended the bloat era.
2. **10 MB pre-commit gate** — per-clone; a fresh clone is unprotected until
   `./scripts/setup-git-hooks.sh install` runs.
3. **Persistent pack-memory bounds** — `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`,
   `pack.threads=1`, repo-local **and** global, so bare `git gc` and `git push` are
   bounded too; verify with `./scripts/setup-git-gc-config.sh --verify`.
4. **Scheduled bounded maintenance** — systemd user timers (daily health check,
   daily incremental gc, weekly bounded full gc); use `./scripts/safe-git-gc.sh`
   for any manual gc, never bare `git gc --aggressive`.

### Preventing the alert-layer churn (duplicate alerts and bounced closes)

1. **Keep the closed-bead filter and duplicate detection** (crash-alert-manager.sh,
   2026-09-02 fixes) so a resolved crash does not mint 71 more alert beads.
2. **Order closures correctly:** an alert's close bounces while its closure-bead
   blockers are open. Close the documentation/determination blockers first, then the
   alert — then remove the stale `verification-failed` label (demonstrated on bf-3pee6,
   the first final close of a 5×-reopened alert, and on sibling alert bf-kk87a,
   commit 8d44ac8).
3. **Cite the canonical report; append dated corrections in place** rather than
   minting new documents that re-assert a superseded root cause — the timeout/OOM
   contradiction in §2.1 propagated through three documents before being corrected.
4. **Verify the target bead's actual state before investigating** — every one of the
   five prior closes was correct and would have stuck had the closure ordering been
   right; the re-investigations were the waste, not the determinations.

## 6. Resolution

- **Status: DUPLICATE — no action required on the subject.** bf-1s6c3 is closed and
  verified; the repository is healthy; the crash class is prevented by the four layers
  above.
- **Chain:** close domchk-06c4e978 (this report's bead), then close bf-5cfqn — with its
  only blocker closed the close should finally stick — and remove the stale
  `verification-failed` label.
- **Recommended close reason for bf-5cfqn:** "Duplicate alert for resolved crash
  bf-1s6c3 — memcg-OOM repository-bloat era; named instant 00:28:36.425389752Z is
  attempt 58's kill (00:28:30.196295497Z, exit −1, 285,898 ms) + 6.229 s
  HANDLING_RELEASE_DONE heartbeat, not a distinct crash; original alert bf-oplew;
  canonical analysis docs/crash-analysis-bf-1s6c3-2026-09-06.md; deliverable
  docs/verification-report-bf-5cfqn-2026-09-07.md (domchk-06c4e978)."
