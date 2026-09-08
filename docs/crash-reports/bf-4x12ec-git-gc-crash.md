# Git GC Agent Crash — Incident Report: bf-4x12ec

> **Scope of this document.** Child 1 of 5 in the split of parent bead
> `domchk-f6757c18` (umbrella: *Document crash findings and
> create verification report*). This child contributes the **Summary block** and
> the **incident timeline**, consolidated from investigation work that already
> exists in this repo. The four placeholder sections at the end — **Root Cause**,
> **Repository State**, **Resolution**, **Lessons Learned** — are owned by later
> children in this split and are deliberately left empty here.

## Summary

- **Date**: 2026-08-14
- **Bead**: `bf-4x12ec` — "Execute aggressive git garbage collection to eliminate OOM risk" (task / P2)
- **Exit code**: `-1` (×44) — a **harness sentinel**, not a POSIX exit value and
  not a signal number: needle's classification for a child agent that terminated
  without a wait status (signal death). The 8 later attempts exited `124`
  (the 600 s agent timeout) and the 53rd exited `0`.
- **Resolution**: ⏳ **PENDING** — placeholder; a later child in this split
  writes the verdict and the resolution narrative. (The outcome *events* are in
  the timeline below.)
- **Workspace / worker / session**: `/home/coding/domain-check` /
  `claude-code-glm-4.7-lab-domain-check` / session `a6dbb1fc` (agent
  `claude-code-glm-4.7`, model `glm-4.7`)
- **Shape of the incident**: not one crash but a **64-minute retry storm over 53
  attempts** — 44 × exit `-1` (crash loop, 38.9–115.8 s each), then 8 × exit
  `124` (timeout loop, exactly 600.0 s each), then 1 × exit `0` (success,
  491.8 s)
- **Classification**: INFRASTRUCTURE — no domain-check code defect
- **Source of record**: the primary needle event log
  `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl`
  (10,138 lines; 53 claims / 53 dispatched / 53 completed), independently
  re-derived twice

All timestamps below are **UTC** unless marked EDT (local = UTC−4).

## Incident timeline (2026-08-14, UTC)

| Time | Event | Source |
|---|---|---|
| 10:17:26.387 | Bead `bf-4x12ec` created (P2, auto strand) | bead record |
| 10:21:06.969 | First claim — session `a6dbb1fc`, template `pluck`, 71,698-byte prompt | needle log seq 1729 |
| 10:21:07.995 | Prompt enqueued → Claude session `8b2a5b0d` | transcript |
| 10:21:23.336 | `git count-objects -vH` → **4,649 loose objects / 17.20 GiB**, 9.60 MiB pack | transcript |
| 10:21:32.406 | `du -sh .git/` → **18G** | transcript |
| 10:21:41.782 | gc attempt 1 → **exit 128**: `bad numeric config value '1.hour' for 'gc.aggressivewindow'` | transcript |
| 10:22:02.982 | Retry `git config gc.aggressivewindow "1h"` → still **exit 128** at 10:22:09 (git expects an integer number of *days*) | transcript |
| 10:22:18.314 | `git config --unset gc.aggressivewindow` → OK (the key is still unset today) | transcript |
| 10:22:36.260 | **gc attempt 2 launched** (`git gc --aggressive --prune=now`) — no result ever returned | transcript |
| **10:23:02.958** | **Attempt #1 dies — `exit_code=-1`**, 115.8 s in (needle seq 1741) | needle log |
| 10:23:02.959 | `outcome.classified` — exit -1 → outcome `crash` (seq 1744) | needle log |
| 10:23:11.219 | `HANDLING_RELEASE_DONE` heartbeat (seq 1750) — **the dispatch's recorded "crash timestamp"** | needle log |
| **10:23:14.244** | **First alert** (`bead.released` + `outcome.handled` action=`alerted`, seq 1752–1753) — 11.3 s after the first death; earliest alert for this bead | needle log |
| 10:23:16 → 11:27:26 | **42 more claim→crash cycles** — 44 kills total, 38.9–115.8 s each; the worker alerts, releases and re-claims every time | needle log |
| 10:43:35.281 | `worker.handling.timeout` — `bf sync --flush-only failed` mid-storm: the bead-store write path was also struggling (attempt #15 in flight) | needle log |
| 10:43:58.590 | Mid-storm capture (transcript `9539f3b2`): disk 85% used / 67G free; **mem 45Gi available, swap 0B** | transcript |
| 10:44:07 | That attempt launches gc → killed at 10:44:53 (~8 s later, exit -1) | transcript + needle log |
| 11:28:07 | First of 3 `pluck` attempts producing **zero assistant output** → exit 124 at exactly 600 s (exits 11:38:07, 11:48:28, 11:58:51) | needle log |
| 11:59:06 | Needle switches to the **`split` template** ("Auto-Split: Decompose This Bead", prompt_len 3896) | needle log |
| 11:59:06 → 12:50:33 | 5 more auto-split attempts → all exit 124 at 600 s, no tool calls | needle log |
| 12:57:53 – 12:58:39 | Final attempt (transcript `31800ee3`): creates children `bf-173o7e` (gc) / `bf-5jhvpk` (repack) / `bf-im2sl1` (verify), chains them with dependencies, sets the umbrella label | transcript |
| **12:58:45.113** | **Exit 0** after 491.8 s; `verification.passed` 11 ms later (12:58:45.126, `gates_run: 1`) — gated success, not merely exit-0 | needle log |
| 12:58:55.502 | `bead.orphaned` — released unassigned instead of closed. This is why alerts kept regenerating until the manual close on Aug 17 | needle log |

### Phase summary

| Phase | Window (UTC) | Attempts | Exit | Classified / handled | Duration each |
|---|---|---|---|---|---|
| 1. Crash loop | 10:23:02 – 11:27:26 | 44 | `-1` | `crash` / `alerted` | 38.9 – 115.8 s |
| 2. Timeout loop | 11:38:07 – 12:50:14 | 8 | `124` | `timeout` / `deferred` | exactly 600.0 s |
| 3. Success | 12:58:45 | 1 | `0` | `success` / `none` | 491.8 s |

Every phase-1 attempt made **zero** packing progress: the loose-object count and
byte figures are identical at 10:21:23 and at 10:43:49, because aggressive gc
builds delta chains in memory before writing any pack.

### Git history across the window (from `git log`)

The storm left **no commits**. The repo sat at a 2026-08-09 baseline for the
entire incident — `git rev-list --count 00117cb..8373e5d` is `1`, i.e. no commit
exists inside the crash window; `git log --since=2026-08-13 --until=2026-08-16`
returns only post-cleanup catch-up commits.

| Commit time (UTC) | Commit | Note |
|---|---|---|
| 2026-08-09T17:00:56Z | `00117cb` "fix: remove unused time import and update bootstrap test initialization" | **HEAD at crash time** — last commit before the incident |
| 2026-08-15T13:56:53Z | `8373e5d` "migrate: rehydrate the bead workspace from bead-forge to bead-rs" | first commit after the storm, the next day |
| 2026-08-16T22:20:34Z | `c27899f` "chore: catch up lab work onto origin (squashed)" | post-cleanup catch-up begins |
| 2026-08-17T00:43:19Z | `91e7d05` "chore: complete repository cleanup to eliminate git bloat" | cleanup recorded |
| 2026-08-17T01:18:54Z | `bb7455f` "docs: consolidate crash investigation documentation into organized directory structure" | investigation docs reorganized |

This is consistent with what the bead was doing: `git gc` changes the object
store, not the tree, and the killed attempts changed nothing at all. The
incident's trail lives in the needle event log and the bead records, not in
`git log`.

### Timestamp reconciliation

Four other "crash timestamps" circulate in older reports for this bead —
10:25:30, 10:39:42.223, 10:41:13 and 11:14:39. All are single events *inside*
the phase-1 storm: each is one attempt's `HANDLING_RELEASE_DONE` heartbeat or
its alert, not a separate crash. **There were 44.**

- **First death: 2026-08-14T10:23:02.958Z** — the agent process actually died here.
- **First alert: 2026-08-14T10:23:14.244Z**; **last alert: 11:28:04.917Z**.
- The dispatch-recorded **10:23:11.219Z** is crash #1's heartbeat, 8.3 s after
  the death. **Alert timestamps are release heartbeats, not kill times.**
- The bead's 2026-08-17T14:50:41Z "Updated" stamp is the **manual closure**
  time, not completion — work finished 2026-08-14T12:58:45Z.

## Root Cause

> **memcg OOM inside the agent dispatch scope's 12 GiB `MemoryMax`.**
> `git gc --aggressive --prune=now`, run against a repository holding 4,649
> loose objects totalling 17.20 GiB, drove the anonymous memory of needle's
> transient `run-p*.scope` past `MemoryMax=12884901888` (12 GiB), and the
> kernel's cgroup OOM killer (`CONSTRAINT_MEMCG`) SIGKILLed the
> highest-badness task in the hitting memcg. **On Aug-14 that victim was the
> agent dispatch task itself** (or its `bash -c` parent), at 39–116 s into
> each attempt — which is why needle recorded `exit_code = -1` for the agent
> rather than a `git` exit code. Classification: **INFRASTRUCTURE** — no
> panic, no timeout governor, no domain-check code defect.

**Corrected finding.** The initial determination (commit `7db6a25`) named the
`git` process as the killed task. Commit **`89c66af`** (bead
`domchk-9e2aa740`, cross-ref `domchk-2539cf8c`) corrected the victim
selection: these scopes set `memory.oom.group=0`, so the kernel picks a
victim per task by OOM badness — *which* task dies is nondeterministic while
the *cause* stays constant. On Aug-14 the agent dispatch task died first; on
Aug-16, when `git` processes lived long enough to outgrow the agent's RSS,
`git` was the usual victim (257 kernel `task=git` memcg kills in that
window). Same mechanism, different recorded victim. (The opening statement
block of [`docs/crash-investigations/bf-4x12ec-root-cause.md`](../crash-investigations/bf-4x12ec-root-cause.md)
still reads "the `git` process"; the corrected victim selection is that
document's §4 and `89c66af`.)

### Why the kills were deterministic — 44 × in 64 minutes

| Step | Mechanism |
|---|---|
| Lethal command | The bead body itself prescribed `git gc --aggressive --prune=now` (plus `repack -a -d --depth=250 --window=250`) — the hazard was in the task text, authored as a mitigation *for* repository bloat |
| Zero progress | `--aggressive` builds delta chains across the *entire* object set (window/depth 250) **in memory before writing any pack bytes**, so every kill left the repo byte-identical — 4,649 loose objects / 17.20 GiB before and after, 44 times |
| Scope budget | Each attempt ran in a fresh needle transient `run-p*.scope`, cap **directly verified** at `MemoryMax=12884901888` (12 GiB) for agent dispatches (6 GiB for test-runner scopes), with `oom_score_adj=200` and `memory.oom.group=0` |
| Kill | Scope anonymous memory reached the cap → kernel memcg OOM killer (`CONSTRAINT_MEMCG`) → SIGKILL of the highest-badness task — on Aug-14, the agent dispatch task itself |
| Retry | Needle re-claimed and re-ran the same command in a fresh scope against the same repository state — identical death, 38.9–115.8 s each, until auto-split decomposed the workload at 11:59:06Z and the 53rd attempt exited 0 |

### `exit -1` is a harness sentinel, not a signal number

needle (Rust) reads the child's wait status with
`status.code().unwrap_or(-1)` (`src/dispatch/mod.rs:991,996`). Rust's
`ExitStatus::code()` returns `None` **exactly when the process died by
signal**, and the `unwrap_or(-1)` flattens every signal — SIGKILL, SIGHUP,
SIGTERM alike — to the single recorded value `−1`. A needle `−1` therefore
means *"the agent terminated without an exit status"* and identifies **no**
signal number; here the signal source was the kernel OOM killer's SIGKILL.
The `−1 → SIGHUP` mapping that appears in some 2026-09-02-era documents is
the Python-subprocess writer convention, not needle's, and is superseded by
the dated correction atop
[`docs/research/root-cause-analysis-signal-minus-one-crashes.md`](../research/root-cause-analysis-signal-minus-one-crashes.md).

### Why "the host had 45 Gi free" does not contradict this

A memcg kill needs only the **scope** budget exceeded, not host exhaustion.
The mid-storm capture (10:43:59 UTC, 8 s before one attempt's kill) shows
45 Gi available and 0 B swap used — and the next attempt died anyway. All
257 same-period `git` kills in the journal are `CONSTRAINT_MEMCG` with host
RAM to spare.

### Evidence limits

No kernel logs survive for 2026-08-14 (the current boot began 2026-08-15).
The Aug-14 verdict rests on the retry-storm signature from the primary
needle event log (independently re-derived three times, all concordant),
direct kernel evidence of the identical mechanism 257 times in the Aug-16
cleanup window, and host-memory figures that exclude host-wide OOM. High
confidence — but not an Aug-14 `dmesg` line, and this report does not claim
one.

**Sources for this section:** [`docs/crash-investigations/bf-4x12ec-root-cause.md`](../crash-investigations/bf-4x12ec-root-cause.md)
(formal root-cause statement; §4 victim selection) ·
[`docs/research/root-cause-analysis-signal-minus-one-crashes.md`](../research/root-cause-analysis-signal-minus-one-crashes.md)
+ its dated-correction note ·
[`docs/analysis/signal-analysis.md`](../analysis/signal-analysis.md) (bead
`domchk-2539cf8c` — source-level sentinel decode, live scope verification) ·
commit **`89c66af`** (root-cause correction) · commit **`fc96211`** (signal
analysis for exit −1) · [`docs/crash-investigations/bf-4x12ec-final-crash-report.md`](../crash-investigations/bf-4x12ec-final-crash-report.md)
(canonical consolidated companion).

## Impact

| Question | Answer |
|---|---|
| **Repository state** | **Healthy — never corrupted.** The kills left the object store byte-identical (4,649 loose objects / 17.20 GiB before and after every attempt; no half-written pack). After the eventual cleanup: 753 MB → 92 MB (2026-09-02) → **105 MB today**, 0 garbage objects. Fresh `count-objects`/`fsck` snapshot: [Repository State](#repository-state) below. |
| **Git operations** | **Working.** Broken only inside the failing dispatch scopes during the storm — every phase-1 attempt died before writing a pack, and ordinary git use on this workspace was never broken. Post-cleanup, all operations pass: `./scripts/check-repo-health.sh` green (re-run 2026-09-08, exit 0), scheduled bounded gc and pushes running daily. |
| **Data loss** | **None.** No commits were lost — the repo sat at its 2026-08-09 baseline (`00117cb`) for the whole incident and no commit exists inside the crash window (git-history table above). No working-tree or object-store loss: every kill preceded any pruning, so the 17.20 GiB of loose objects was intact after each death, and the later size reduction was a verified consolidation into a single pack, not deletion. |

The casualty was the *agent's time*, not the repository: 44 crashes + 8
timeouts bought zero packing progress, and the completed bead was released
orphaned instead of closed (`bead.orphaned`, 12:58:55Z), which kept alerts
regenerating until the manual close on 2026-08-17.

## Repository State

> ⏳ **PLACEHOLDER — not filled in by this child.** Covers the parent template's
> Impact block: repository state (healthy / corrupted), whether git operations
> were working or broken, and whether any data was lost. Source of record:
> `docs/crash-investigations/bf-4x12ec-final-crash-report.md` ("System State at
> Crash" and "Work Completion Status").

## Resolution

> ⏳ **PLACEHOLDER — PENDING. Not filled in by this child.** A later child in
> this split records the verdict (COMPLETED / PARTIAL / FAILED) and what
> actually happened, including which bead performed the eventual cleanup.

## Lessons Learned

> ⏳ **PLACEHOLDER — not filled in by this child.** Recommendations to prevent
> similar crashes. Source of record:
> `docs/crash-investigations/bf-4x12ec-final-crash-report.md` ("Lessons Learned").

## Sources (read, not re-derived)

| Document | Contribution to this report |
|---|---|
| [`docs/crash-investigations/bf-4x12ec-final-crash-report.md`](../crash-investigations/bf-4x12ec-final-crash-report.md) | canonical consolidated report (bead `domchk-8c3fceeb`, 2026-09-02) — summary, phase table, reconciliation |
| [`docs/crash-investigations/bf-4x12ec-crash-artifacts-2026-09-02.md`](../crash-investigations/bf-4x12ec-crash-artifacts-2026-09-02.md) | transcript-level timeline rows, artifact inventory, heartbeat reconciliation (bead `domchk-2ff261ce`) |
| [`docs/crash-investigations/bf-4x12ec-log-review-2026-09-02.md`](../crash-investigations/bf-4x12ec-log-review-2026-09-02.md) | independent event-log re-derivation; flush failure, verification gate, `bead.orphaned` findings (bead `domchk-30d451d3`) |
| `git log --since=2026-08-13 --until=2026-08-16` | the git-history table above (verified live against HEAD at write time) |

**Caution when citing this incident:** `docs/crash-reports/bf-4x12ec-verification-report.md`
(a 2026-08-26-era summary) states "Exit Code: -1 (signal -1)" and a
single-crash framing — both superseded. `-1` is a harness sentinel, and the
incident was 44 crashes. Cite the consolidated report.

---
**Report date:** 2026-09-08 · Split of `domchk-f6757c18` — summary + timeline `domchk-779d1180` (child 1) · **root cause + impact `domchk-08bdde8d` (child 2)** · repository state `domchk-0936d2db` (child 3) · resolution + lessons learned `domchk-1ef6b252` (child 4) · CLAUDE.md procedures + finalization `domchk-6f771e64` (child 5)
**Sections completed:** Summary, Incident timeline, Root Cause, Impact · **Pending from later children:** Repository State, Resolution, Lessons Learned
