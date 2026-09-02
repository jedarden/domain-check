# Investigation Report: bf-65lsdu Crash Context

**Investigation bead:** domchk-7e6c4b21
**Investigation date:** 2026-09-02
**Subject crash:** exit code -1 (signal -1) at **2026-08-13T22:14:36Z**, workspace `.`, agent `claude-code-glm-4.7`
**Verdict:** Infrastructure event — OOM termination during `git gc --aggressive` on a severely bloated repository. Not a domain-check code defect.

---

## 1. Original bead: purpose and description

| Field | Value |
|-------|-------|
| **ID** | bf-65lsdu |
| **Title** | Run repository cleanup to eliminate 17GB bloat |
| **Type / Priority** | task / P2 |
| **Created** | 2026-08-13T21:16:00.660527074Z |
| **Final status** | Closed (2026-08-17T00:45:33Z) |
| **Workspace** | `.` (this repo, `/home/coding/domain-check`) |

**Description (verbatim from the bead store):**

> ## Task
> Execute git gc --aggressive to pack the 17GB of loose objects that are causing OOM crashes.
>
> ## Context
> Repository currently has 17.20 GiB of loose objects (4,515 objects). This is what causes the
> OOM killer during git operations. The scripts/cleanup-bloat.sh script is already available.
>
> ## Acceptance Criteria
> - [ ] Repository size before cleanup documented (should be ~18GB)
> - [ ] git gc --aggressive --prune=now executed successfully
> - [ ] Repository size after cleanup documented (should be <500MB)
> - [ ] Loose objects packed (verify with git count-objects)
>
> ## Notes
> This may take 30-60 minutes to run. Monitor the process. If it fails or times out, may need
> to use git repack -a -d --depth=250 instead.

The bead was itself a remediation task: the 17.20 GiB of loose git objects (4,515 objects,
~18GB total repository) were the *cause* of OOM crashes elsewhere, and cleaning them up was
the fix.

## 2. The specific crash event (2026-08-13T22:14:36)

The crash is recorded in machine-generated crash alert bead **bf-3k8oln**
("ALERT: Agent crash on bead bf-65lsdu"), whose description is the crash report emitted by
the crash-detection hook:

```
## Agent Crash Report

- **Bead ID**: bf-65lsdu
- **Agent**: claude-code-glm-4.7
- **Exit code**: -1 (signal -1)
- **Workspace**: .
- **Timestamp**: 2026-08-13T22:14:36.443825925+00:00

The agent process was killed. This bead has been released for retry.
```

Details:

- **Crash timestamp:** 2026-08-13T22:14:36.443825925+00:00
- **Alert bead created:** 2026-08-13T22:14:36.453032707Z — 9 ms after the recorded crash, i.e.
  the alert was generated automatically at detection time, not retrospectively
- **Exit code / signal:** -1 / signal -1 → the agent process was killed by an external signal
  (SIGKILL-class), the OOM-killer signature. Not a graceful exit, not a code panic
- **Labels:** `alert`, `crash`, `signal--1`, `failure-count:4`, `umbrella`, `split-child`, `verification-failed`
- **Retry disposition:** the bead was released for retry immediately (per the crash report text)

**Position in the retry sequence:** the `failure-count:4` label makes this the **4th recorded
failure** of bf-65lsdu — between bf-1944k2 (21:48:30) and bf-12yvry (22:20:09) on the same
night. bf-65lsdu ultimately accumulated `failure-count:6` before succeeding. Roughly 10
crash-alert beads were raised against bf-65lsdu on 2026-08-13/14; all share the identical
exit -1 / signal -1 signature.

**Gap found and fixed:** this 22:14:36 alert is **absent from the crash timeline table** in
`docs/crash-information-bf-65lsdu.md` (which jumped from 21:48:30 to 22:20:09). The table has
been amended to include it.

## 3. What the agent was executing at crash time

The agent dispatched to bf-65lsdu was executing the repository cleanup itself: the memory-
intensive pack operation on the bloated repository —

- **Command shape:** `git gc --aggressive --prune=now` (with `scripts/cleanup-bloat.sh` as the
  prepared wrapper, and `git repack -a -d --depth=250` as the documented fallback)
- **Target state:** 17.20 GiB / 4,515 loose objects to be packed
- **Failure mode:** memory demand of aggressive delta compression over ~17 GB of loose objects
  exceeded available system memory → kernel OOM killer SIGKILLed the process tree → exit -1

Supporting evidence:

1. Every crash against this bead in the window has the same kill signature, and the task's
   own description names OOM-during-git-operations as the ambient failure mode.
2. The retrospective notes on bf-3k8oln attribute the crash to "memory pressure during git
   cleanup operation … exit code -1 indicates process termination, likely by the OOM killer."
3. The crashes stopped permanently once the repository was deflated — a clean
   cause-and-effect match with repository bloat, and no domain-check code was involved in
   the crashing process.
4. **No surviving trace of the crashed run exists.** `.beads/traces/bf-65lsdu/` holds the
   *successful* 2026-08-17 run (exit 0, 90.3 s), and `.beads/traces/bf-3k8oln/` holds the
   alert's *retrospective investigation* run (exit 0, 83.5 s, captured 2026-08-26). The
   2026-08-13 crash runs were never captured; the determination above rests on the crash
   report, the task definition, and the pattern across the sibling alerts.

### Data-quality caveat on the alert's notes

The notes later added to bf-3k8oln (during its 2026-08-26 retrospective closure) state a
timeline of "dispatched 23:59:02 … crashed 23:59:58 after ~55 seconds." That does not match
the alert's own machine-generated crash timestamp (22:14:36.443) and instead aligns with the
separate 23:56:16 alert (bf-1dy0zp). The bead `created_at`/crash-report timestamps are
authoritative; the note timeline appears to conflate sibling alerts and should not be relied
on for this specific event.

## 4. Workspace and agent context

| Aspect | Value |
|--------|-------|
| Workspace | `.` → `/home/coding/domain-check` (the domain-check repo) |
| Agent | `claude-code-glm-4.7` (provider `zai`, model `glm-4.7`) |
| Repository state at crash | ~18 GB total; 17.20 GiB loose objects across 4,515 objects; 99% loose / 1% packed |
| Bead age at crash | ~58 minutes (created 21:16 UTC, crashed 22:14 UTC) |

The crash had nothing to do with domain-check application code — the crashing workload was
git repository maintenance, and all subsequent domain-check investigations have found zero
code defects.

## 5. Resolution and aftermath

- **Retries:** after each kill, the crash hook released bf-65lsdu for retry; retries kept
  hitting the same OOM wall while the bloat persisted.
- **Success:** the cleanup ultimately completed — bf-65lsdu was closed 2026-08-17T00:45:33Z
  with reason *"Repository cleanup successfully completed. Reduced from ~18GB to 1.3GB, loose
  objects packed from 4,515 to 3. All acceptance criteria met - see commit `5bf23b7`."*
- **Alert closure:** bf-3k8oln itself was closed 2026-08-26 as a *false-positive retrospective
  alert* — the underlying task had long been completed and verified
  (`docs/verification-report-bf-3k8oln-false-positive-retrospective-crash-alert-resolved-bf-65lsdu.md`).
- **Current repository health (2026-09-02):** ~97 MB total, single-digit MiB of loose objects —
  well inside the healthy thresholds in `docs/maintenance/repository-maintenance-guide.md`.

## 6. Summary of findings

1. **bf-65lsdu** was the P2 remediation task "Run repository cleanup to eliminate 17GB bloat":
   pack 17.20 GiB / 4,515 loose objects via `git gc --aggressive --prune=now`.
2. At crash time (2026-08-13T22:14:36Z) the agent (`claude-code-glm-4.7`, workspace `.`) was
   executing that git gc/repack cleanup; the process was killed by the OOM killer
   (exit -1, signal -1) under the memory pressure of compressing ~17 GB of loose objects.
3. This was the 4th of six recorded failures of the bead, all with the identical kill
   signature; the alert (bf-3k8oln) was auto-created 9 ms after detection and the bead was
   released for retry.
4. Root cause: infrastructure (repository bloat → OOM). Domain-check code was not involved
   and remains defect-free.

## 7. Sources

- Bead store: `bead show bf-65lsdu`, `bead show bf-3k8oln`, `bead show domchk-1d947f1e`
- Crash report text: `.beads/checkpoint/forensic.jsonl` (issue `bf-3k8oln`)
- Trace captures: `.beads/traces/bf-65lsdu/metadata.json`, `.beads/traces/bf-3k8oln/metadata.json`
- `docs/crash-information-bf-65lsdu.md` (timeline — amended by this investigation)
- `docs/verification-report-bf-3k8oln-false-positive-retrospective-crash-alert-resolved-bf-65lsdu.md`
- `docs/investigation-bf-65lsdu-agent-context-2026-09-02.md`
