# Git GC Agent Crash — Incident Report: bf-4x12ec

> **Scope of this document.** Child 1 of 5 in the split of parent bead
> `domchk-f6757c18` (umbrella: *Document crash findings and
> create verification report*). This child contributes the **Summary block** and
> the **incident timeline**, consolidated from investigation work that already
> exists in this repo. Section ownership in this split: **Root Cause** +
> **Impact** — child 2 (`domchk-08bdde8d`); **Repository State** — child 3
> (`domchk-0936d2db`); **Resolution** + **Lessons Learned** — child 4
> (`domchk-1ef6b252`); CLAUDE.md procedures + finalization — child 5
> (`domchk-6f771e64`).

## Summary

- **Date**: 2026-08-14
- **Bead**: `bf-4x12ec` — "Execute aggressive git garbage collection to eliminate OOM risk" (task / P2)
- **Exit code**: `-1` (×44) — a **harness sentinel**, not a POSIX exit value and
  not a signal number: needle's classification for a child agent that terminated
  without a wait status (signal death). The 8 later attempts exited `124`
  (the 600 s agent timeout) and the 53rd exited `0`.
- **Resolution**: ✅ **COMPLETED** — the objective (pack the 17.20 GiB and
  eliminate the OOM hazard) was achieved and is holding, but via decomposition
  and the child beads, not via the 44 crashed attempts: the 53rd attempt split
  the work, and the gc itself completed under child `bf-173o7e`, commit-recorded
  (`91e7d05`) and post-verified (`0a61037`). Verdict + narrative:
  [Resolution](#resolution).
- **Workspace / worker / session**: `/home/coding/domain-check` /
  `claude-code-glm-4.7-lab-domain-check` / session `a6dbb1fc` (agent
  `claude-code-glm-4.7`, model `glm-4.7`)
- **Shape of the incident**: not one crash but a **retry storm over 53
  attempts** — a **64-minute crash loop** of 44 × exit `-1` (38.9–115.8 s
  each), then 8 × exit `124` (timeout loop, exactly 600.0 s each), then 1 ×
  exit `0` (success, 491.8 s). The 64 minutes is the crash loop alone (10:23 –
  11:27); the full 53-attempt span is ~2.6 h (10:21 – 12:58 UTC)
- **Classification**: INFRASTRUCTURE — no domain-check code defect
- **Source of record**: the primary needle event log
  `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl`
  (10,138 lines; 53 claims / 53 dispatched / 53 completed), independently
  re-derived three times — most recently the 2026-09-08 child-5 final pass
  against the live log

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
| 10:23:16 → 11:27:26 | **43 more claim→crash cycles** — 44 kills total, 38.9–115.8 s each; the worker alerts, releases and re-claims every time | needle log |
| 10:43:35.281 | `worker.handling.timeout` — `bf sync --flush-only failed` mid-storm: the bead-store write path was also struggling (attempt #15 in flight) | needle log |
| 10:43:58.590 | Mid-storm capture (transcript `9539f3b2`): disk 85% used / 67G free; **mem 45Gi available, swap 0B** | transcript |
| 10:44:07 | That attempt launches gc → killed at 10:44:53 (~8 s later, exit -1) | transcript + needle log |
| 11:28:07 | First of 3 `pluck` attempts producing **zero assistant output** → exit 124 at exactly 600 s (exits 11:38:07, 11:48:28, 11:58:51) | needle log |
| 11:59:06 | Needle switches to the **`split` template** ("Auto-Split: Decompose This Bead", prompt_len 3896) | needle log |
| 11:59:06 → 12:50:14 | 5 more auto-split attempts → all exit 124 at 600 s, no tool calls (last exit verified 12:50:14.282Z; 12:50:33 is the *next* attempt's `bead.claim.succeeded`) | needle log |
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
| **Repository state** | **Healthy — never corrupted.** The kills left the object store byte-identical (4,649 loose objects / 17.20 GiB before and after every attempt; no half-written pack). After the eventual cleanup: 753 MB → 92 MB (2026-09-02) → **106 MB** (fresh snapshot 2026-09-08), 0 garbage objects. Fresh `count-objects`/`fsck` snapshot: [Repository State](#repository-state) below. |
| **Git operations** | **Working.** Broken only inside the failing dispatch scopes during the storm — every phase-1 attempt died before writing a pack, and ordinary git use on this workspace was never broken. Post-cleanup, all operations pass: `./scripts/check-repo-health.sh` green (re-run 2026-09-08, exit 0), scheduled bounded gc and pushes running daily. |
| **Data loss** | **None.** No commits were lost — the repo sat at its 2026-08-09 baseline (`00117cb`) for the whole incident and no commit exists inside the crash window (git-history table above). No working-tree or object-store loss: every kill preceded any pruning, so the 17.20 GiB of loose objects was intact after each death, and the later size reduction was a verified consolidation into a single pack, not deletion. |

The casualty was the *agent's time*, not the repository: 44 crashes + 8
timeouts bought zero packing progress, and the completed bead was released
orphaned instead of closed (`bead.orphaned`, 12:58:55Z), which kept alerts
regenerating until the manual close on 2026-08-17.

## Repository State

> Contributed by child 3 of the split (`domchk-0936d2db`). The parent template's
> Impact-block questions are answered at summary level in
> [Impact](#impact) — repository state, whether git operations were broken, and
> data loss. This section is the metrics record: before/after object-store
> figures, a fresh post-gc snapshot taken for this child, and the fsck
> caveat future verifications need.

**Verdict: HEALTHY — the repository was never corrupted.** Every phase-1 kill
preceded any pack write, so the object store stayed byte-identical across all
44 attempts; the later size reduction was a verified consolidation into a pack,
not deletion of reachable data.

### Before / after

| Milestone | `.git` size | Loose objects | Pack | Source |
|---|---|---|---|---|
| **At crash time** — 2026-08-14 10:21Z (before) | **18G** | **4,649 / 17.20 GiB** | 9.60 MiB | crash-window transcript (`git count-objects -vH`, `du -sh .git`; timeline above) |
| First cleanup complete — closed 2026-08-17, measured 2026-08-26 | 753 MB | 141 | 10,265 objects / 750.67 MiB | [`bf-4x12ec-verification-report.md`](bf-4x12ec-verification-report.md) |
| Post-gc verification — 2026-09-02 (commit `0a61037`) | 92M | 54 | 1 pack / 10,478 objects / 90.18 MiB | [`repo-health-verification-post-gc-2026-09-02.md`](../archive/crash-investigations/repo-health-verification-post-gc-2026-09-02.md) |
| **Fresh snapshot — 2026-09-08T03:28Z** (this child) | **106M** | 289 / 1.95 MiB | 1 pack / 12,174 objects / 100.25 MiB | commands below |

The 18G → 753 MB → ~100M trajectory *is* the incident's cleanup. The drift
since 2026-09-02 (92M → 106M; 10,478 → 12,174 in-pack) is normal churn from
concurrent agent commits plus the 2026-09-08 `depth=250/window=250` repack that
consolidated 2 packs into the current single 100.25 MiB pack — not renewed
bloat. Loose objects sit at 289 / 1.95 MiB (daily-churn range; 0
prune-packable, **0 garbage**), and every health threshold passes with an order
of magnitude to spare.

### Fresh snapshot (taken for this child, 2026-09-08T03:28Z)

```console
$ git count-objects -vH
count: 289
size: 1.95 MiB
in-pack: 12174
packs: 1
size-pack: 100.25 MiB
prune-packable: 0
garbage: 0
size-garbage: 0 bytes

$ du -sh .git
106M	.git
```

- **`git fsck --full`** — 2026-09-08 fresh run: **exit 0 in 2.7 s, zero
  errors**; dangling-object notices only, which are benign (unreachable-recent,
  not damage). Latest prior verified result: `--full` exit 0 with **zero
  findings** at all (2026-09-02, `0a61037`). See the caveat below before
  reaching for `--no-full`.
- **`./scripts/check-repo-health.sh`** — 2026-09-08 fresh run: **exit 0, all
  criteria passing** — size healthy (105 MB as the script reports it), single
  pack, effective pack-memory bound verified (worst case ≈3072 MiB, within the
  6 GiB ceiling for a 12 GiB dispatch scope), no unmanaged aggressive gc
  running, unpushed backlog clear. Its one ⚠️ lists five 14.3 MB
  `dist/domain-check_darwin_amd64_v1/` release binaries in history —
  informational inventory, unrelated to the gc incident.
- **Git operations** — working. `git fsck --full` completes in ~3 s where the
  pre-cleanup repo timed out; the 2026-09-02 criteria table measured
  `git status` at 0.009 s and `git log --oneline -5` at 0.003 s. Scheduled
  bounded gc and ordinary pushes run daily without incident.
- **Data loss** — none. See [Impact](#impact): the repo sat at its 2026-08-09
  baseline for the whole incident and no commit exists inside the crash window.

### `git fsck --no-full` "invalid reflog entry" output is NOT corruption

`git fsck --no-full` in this repo exits 2 with ~1,008
`error: … invalid reflog entry <OID>` lines. That output is **git 2.50.1
`--no-full` noise on a packed repository, not corruption** (bead
`domchk-b037ca90`): with `--no-full`, fsck never opens packfiles, so reflog
entries whose targets live in the pack are misreported as invalid. Every
flagged OID was individually verified to exist (`git cat-file -t` → `commit`)
and to be reachable. The control experiment on an unrelated healthy repo on
this box (`~/SIGIL`) fails identically under `--no-full` (5,238 of the same
error) and is clean under `--full` — the noise is systemic to the flag on
packed repos, not damage in this one. **Use `git fsck --full` as the integrity
gate on this box** (dangling-object warnings under `--full` are benign), and do
not "repair" the reflog (`reflog expire` / `reflog delete`) — the entries are
valid and expiring them would destroy real history.

**Sources for this section:** commit **`0a61037`** +
[`docs/archive/crash-investigations/repo-health-verification-post-gc-2026-09-02.md`](../archive/crash-investigations/repo-health-verification-post-gc-2026-09-02.md)
(bead `domchk-b037ca90` — post-gc acceptance-criteria table, SIGIL control) ·
[`bf-4x12ec-verification-report.md`](bf-4x12ec-verification-report.md)
(first-cleanup figures only; that document's *framing* is superseded — see the
caution under [Sources](#sources-read-not-re-derived)) · fresh
`git count-objects -vH` / `du -sh .git` / `git fsck --full` /
`./scripts/check-repo-health.sh` run for this child at 2026-09-08T03:28Z.

## Resolution

> Contributed by child 4 of the split (`domchk-1ef6b252`). Assembled from outcome
> evidence already in the repo — the 2026-08-26 verification report, the
> commit record, and the consolidated report — not re-derived.

**Resolution: ✅ COMPLETED.** The task's objective — pack the 17.20 GiB /
~4,650 loose objects and eliminate the OOM hazard during git operations — was
fully achieved and still holds: 106 MB `.git`, 289 loose objects / 1.95 MiB,
0 garbage, `git fsck --full` clean (fresh snapshot 2026-09-08,
[Repository State](#repository-state) above). The verdict is COMPLETED rather
than PARTIAL because every acceptance criterion in the bead body is met on the
commit-recorded record (`0a61037`, 2026-09-02) and on today's snapshot. What
keeps it from being a clean first-try completion is the *path*: 52 of 53
attempts (44 kills + 8 timeouts) bought zero packing progress, and the work
finished only after needle's auto-split decomposed the monolith.

### Did the task actually succeed despite the crash?

**Yes — by decomposition, not by survival.** Three outcomes must not be
conflated:

1. **The 44 phase-1 attempts: failed completely.** Each was memcg-OOM killed
   inside its dispatch scope before writing any pack bytes — the object store
   was byte-identical before and after every death. Nothing was accomplished
   by any of them, and nothing of any attempt survived its kill.
2. **The 53rd attempt: succeeded** — exit 0 at 12:58:45Z after 491.8 s, with a
   passing verification gate. Its work product was *organizational*: children
   `bf-173o7e` (gc) / `bf-5jhvpk` (repack) / `bf-im2sl1` (verify), chained
   with dependencies under the umbrella label. "gc" / "repack" / "verify" each
   fit an individual scope budget where the monolith did not.
3. **The bead's actual objective: completed under child `bf-173o7e`** — Closed
   2026-08-17T17:12:09Z, reason *"Git gc completed successfully — 17.20GB
   loose objects packed into 444MB pack file, repository valid"* (that close
   reason's figures are its own day-of measurement; the 2026-08-26 live
   measurement is 753 MB `.git` / 750.67 MiB pack,
   [Repository State](#repository-state)). The parent `bf-4x12ec` itself was
   closed manually 2026-08-17T14:50:41Z, after being released orphaned
   (`bead.orphaned`, 2026-08-14T12:58:55Z) instead of closed at success.

This corrects the 2026-08-26
[`bf-4x12ec-verification-report.md`](bf-4x12ec-verification-report.md) in one
specific way while keeping its verdict: that report attributed the completion
to background survival — "the actual git operation was already in progress via
the git subprocess" and finished after the agent died. That mechanism is **not
supported**: every phase-1 kill left the repo byte-identical, so no gc
survived a kill, and no gc ran to completion on Aug-14 at all. What that
report got right, and what stands, is the **outcome** — work completed
successfully, repository healthy, no remediation needed. (Its single-crash
framing and "signal -1" reading are superseded — see the caution under
[Sources](#sources-read-not-re-derived).)

### Did the work product survive the crash?

**Yes — as repository state, not as in-window commits.** The storm left no
commit: the repo sat at its 2026-08-09 baseline `00117cb` for the whole
incident, and `git rev-list --count 00117cb..8373e5d` = 1 (the next commit is
the day-after bead migration; re-verified live 2026-09-08). A `git gc` changes
the object store, not the tree, so the surviving evidence of completion is
**commit-recorded, not commit-carried**:

| Commit | Date (UTC) | What it evidences |
|---|---|---|
| `8373e5d` | 2026-08-15 13:56 | first commit after the storm — repo and git operations alive the next morning |
| `91e7d05` | 2026-08-17 00:43 | message records the completed final gc pass: *"Before: 527M .git, 163 loose objects (3 pack files) → After: 752M .git, 0 loose objects (1 optimized pack file)"*. It touches only `.needle-predispatch-sha` (dispatch bookkeeping) — a gc leaves no tree diff to commit |
| `0a61037` | 2026-09-02 | post-gc repo-health verification — **all criteria pass**: 92M `.git`, 54 loose objects, 1 pack / 10,478 objects / 90.18 MiB, `git fsck --full` exit 0 with zero findings, `git status` 0.009 s |
| `89c66af` | 2026-09-02 | the corrected root-cause record the outcome rests on (victim selection; 12 GiB `MemoryMax` directly verified) |

Read `91e7d05`'s "before" figure carefully: **527M is the state after the
earlier reduction passes, not the 18G crash-time figure** — that message
records the final pass of a multi-pass cleanup. The 18G → 753 MB first
reduction leg is not commit-recorded at all (no commit exists inside the
window); its figures come from the live 2026-08-26 measurement in
[`bf-4x12ec-verification-report.md`](bf-4x12ec-verification-report.md).

### Acceptance criteria — met, and when

| Bead criterion | Target | Met | Evidence |
|---|---|---|---|
| `git gc --aggressive --prune=now` completes without OOM/timeout | no kill | 2026-08-16/17 — **not** on 08-14 | `bf-173o7e` close reason; `91e7d05` message |
| `git repack -a -d --depth=250 --window=250` | completes | 2026-09-08 | 2 packs → 1 / 100.25 MiB ([Repository State](#repository-state)) |
| Repository size | < 500 MB | by 2026-09-02 | `0a61037` — 92M (the first post-cleanup snapshot, 2026-08-26, was 753 MB — above target) |
| Loose objects | < 100 | by 2026-09-02 | `0a61037` — 54 (first snapshot was 141) |
| `git fsck` completes without timeout | no timeout | 2026-08-26 | verification report; `--full` is the integrity gate ([fsck caveat](#git-fsck---no-full-invalid-reflog-entry-output-is-not-corruption)) |
| Git operations without OOM | clone/fetch/checkout OK | 2026-08-26, holding | verification report; daily bounded gc and pushes since |

The two criteria that missed their number on the first post-cleanup snapshot
(753 MB vs <500 MB; 141 vs <100) were met by the scheduled maintenance
trajectory — bounded gc, not more aggressive gc.

### Data loss

**No.** No commit was lost — no commit exists inside the crash window and the
repo sat at its `00117cb` baseline for the whole incident (git-history table
above). No working-tree or object-store loss — every kill preceded any
pruning, so all 17.20 GiB of loose objects were intact after each of the 44
deaths. The later size reduction was a verified consolidation into a pack, not
deletion: `git fsck --full` exit 0 with zero findings (`0a61037`) and **0
garbage objects** in every snapshot since. What the incident cost was time, not
data — ~2.6 hours of dispatch churn across 53 attempts, plus the orphaned
release that kept false-positive alerts regenerating until the manual close
(2026-08-17) and again in the 2026-08-26 wave.

## Lessons Learned

> Contributed by child 4 of the split (`domchk-1ef6b252`). The seven-lesson source
> of record is
> [`docs/crash-investigations/bf-4x12ec-final-crash-report.md`](../crash-investigations/bf-4x12ec-final-crash-report.md)
> ("Lessons Learned"); this section carries the four **prevention**
> recommendations that came out of this incident, with their live status, and
> compresses the rest.

**The one-sentence lesson:** the hazard was in the task text, the binding
constraint was the dispatch scope's memory ceiling, and 44 retries of a
deterministic kill bought exactly what the first one bought — nothing. Each
prevention measure below attacks one of those three.

### Prevention recommendations (status verified live 2026-09-08)

| # | Hazard from this incident | Recommendation | Status (2026-09-08) |
|---|---|---|---|
| 1 | The bead's own body prescribed bare `git gc --aggressive --prune=now`, which builds delta chains across the *entire* 17.20 GiB object set **in memory before writing a pack byte** — that is why 44 kills left the repo byte-identical | **Run gc only through `scripts/safe-git-gc.sh`** — soft `SAFE_GC_MEMORY_MAX` (drives `pack.windowMemory`, default 2g) under a hard `SAFE_GC_CGROUP_MAX` ceiling (default 6g), `ulimit -v` fallback, fail-fast preflight (exit 2 before any git work), per-stage checkpoint/resume | Live. The same bounds are also persisted as plain git config (`pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` — the window limit is per-thread) repo-locally **and** globally via `scripts/setup-git-gc-config.sh`; `--verify` reports worst case ≈3,072 MiB, inside the 6 GiB ceiling for a 12 GiB dispatch scope |
| 2 | Every attempt ran in a needle transient scope with `MemoryMax=12 GiB` (agent scopes; 6 GiB test-runner scopes), `oom_score_adj=200`, `memory.oom.group=0` — a workload whose peak exceeds the scope *must* die, and with `oom.group=0` *which* task dies is nondeterministic (agent task on Aug-14, `git` on Aug-16) | **Size the work to the scope, not the scope to the work.** Before dispatching a heavy operation, bound its peak (delta window/depth, `pack.windowMemory`, chunking/decomposition) so it fits the dispatch scope; decompose rather than raise limits; never read "host had N GB free" as headroom for a memcg-scoped task | Caps directly verified in the live scopes ([Root Cause](#root-cause)); the durable fix here was bounding the command (row 1), not raising the scope — the host had 45 Gi free all storm and attempts died anyway |
| 3 | The one successful attempt released the bead **orphaned** instead of closing it (`bead.orphaned`, 12:58:55Z) — false-positive alerts regenerated until the manual close on 2026-08-17 and again in the 2026-08-26 wave | **Gate every close on work verification:** `./scripts/verify-work-completion.sh <bead-id> --summary "..."` before `bead close` — fails on unpushed commits or missing expected artifacts, and writes `.beads/state/work-completion/<bead-id>.json` so crash triage can tell post-completion deaths from mid-task ones | Live (`scripts/README.md`); the alert side is also fixed — closed-bead filtering + duplicate detection in `scripts/crash-alert-manager.sh` (CLAUDE.md "Crash Alert System") |
| 4 | Nothing watched the repository between incidents — the bloat that armed this trap was invisible until git operations started dying | **Keep the repo-health automation installed:** systemd user timers (repo health + auto-gc check daily, incremental gc daily 03:00, full gc weekly Sun 04:00 at `MemoryMax=4G`), the 10 MB pre-commit size gate, and CLAUDE.md's bloat thresholds | Live: all 8 `domain-check-*` timers present with future trigger times (`systemctl --user list-timers 'domain-check-*'`); `check-repo-health.sh` exit 0; pre-commit hook installed and current (`./scripts/setup-git-hooks.sh --check`) |

### The rest of the lesson set (compressed — full text in the final report)

1. **Stop retrying deterministic failures — decompose.** Same cause + same
   command + kill within ~2 min should trip the circuit breaker
   (`scripts/crash-circuit-breaker.sh`, now present) or the split path *early*;
   needle's auto-split only engaged ~96 minutes in, and the split is what
   worked.
2. **Never rule out OOM from host memory alone.** Check the process's cgroup
   budget — this box memcg-kills git while tens of GB of host RAM are free.
3. **Do not let task text prescribe memory-hazardous commands.** This bead's
   body did, authored as a mitigation *for* bloat; the safe script now exists
   and is codified in CLAUDE.md.
4. **Close beads at success** — never release them orphaned (row 3 above).
5. **Pre-flight config hygiene.** Two attempts died to a stale
   `gc.aggressivewindow='1.hour'` before the crash loop even started; validate
   git config before large operations.
6. **Capture kernel evidence immediately — it does not survive.** Journal
   rotation erased every Aug-14 kernel line, which is why this report's root
   cause is inference-plus-corroboration and says so.
7. **Write crash reports from primary event logs, not earlier summaries.** The
   "57-minute gc" and single-crash narratives propagated across documents
   until re-derived from the JSONL event stream.

## CLAUDE.md Procedure Updates (child 5 of 5)

> Contributed by child 5 of the split (`domchk-6f771e64`). The task's bar was
> **iff**: update the repo `CLAUDE.md` / `scripts/README.md` only if this
> incident exposed a genuinely new procedure, and record the conclusion either
> way. This section is that record.

**Conclusion: one new procedure — everything else already codified.** Each
prevention recommendation above was checked against the repo `CLAUDE.md` and
`scripts/README.md` as they stood at finalization (2026-09-08):

| Prevention row | Already codified? | Where |
|---|---|---|
| 1 — gc only via `safe-git-gc.sh` + persisted `pack.windowMemory` bounds | Yes — no update | CLAUDE.md "Git Operations Safety" and the "Mechanical guard for the bare-gc path" note (`scripts/setup-git-gc-config.sh --verify`) |
| 2 — size the work to the dispatch scope; host free RAM is not headroom | Rationale, not a separate procedure | It is the *reason* for row 1's documented bounds; the analytical lesson stays in [Root Cause](#root-cause) here rather than becoming a second procedure |
| 3 — gate closes on `verify-work-completion.sh`; closed-bead alert filtering | Yes — no update | CLAUDE.md "Pre-Close Work Verification" and "Crash Alert System" |
| 4 — repo-health timers, pre-commit size gate, bloat thresholds | Yes — no update | CLAUDE.md "Scheduled Maintenance" and "Repository Bloat Prevention and Detection" |

**The one addition (made):** the [`git fsck` caveat](#git-fsck---no-full-invalid-reflog-entry-output-is-not-corruption)
from this chain's *verification* leg (commit `0a61037`, bead `domchk-b037ca90`)
was not in either file, and acting on it wrongly is destructive: `git fsck
--no-full` on this packed repo exits 2 with ~1,008 false `invalid reflog
entry` errors, and "repairing" the reflog in response would destroy real
history. One rule added to the CLAUDE.md "Current Repository Health"
integrity bullet: **`--full` is the integrity gate on this box; never
"repair" the reflog over `--no-full` noise.** `scripts/README.md` needed no
change — its checks already specify `git fsck --full`.

**Considered and rejected:** `scripts/crash-circuit-breaker.sh` (lesson 1's
"stop retrying deterministic failures") is real and live, but it is the
*bf-65lsdu* chain's deliverable (commit `ad73b42`, bead `domchk-0c916ec7`),
not a bf-4x12ec finding — bf-4x12ec's docs cite it, and its CLAUDE.md home
belongs to that chain, not to this report's mandate.

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
**Report date:** 2026-09-08 · Split of `domchk-f6757c18` — summary + timeline `domchk-779d1180` (child 1) · root cause + impact `domchk-08bdde8d` (child 2) · repository state `domchk-0936d2db` (child 3) · resolution + lessons learned `domchk-1ef6b252` (child 4) · **CLAUDE.md procedures + finalization `domchk-6f771e64` (child 5)**
**Sections complete:** Summary, Incident timeline, Root Cause, Impact, Repository State, Resolution, Lessons Learned, [CLAUDE.md Procedure Updates](#claudemd-procedure-updates-child-5-of-5)
**Final consistency pass (child 5, 2026-09-08):** every event-log figure re-verified first-hand against the primary needle log — 53 `bead.claim.succeeded` / 53 `agent.dispatched` / 53 `agent.completed`, exits 44 × `-1` / 8 × `124` / 1 × `0`, first kill 10:23:02.958Z, last kill 11:27:26.173Z, first alert 10:23:14.244Z, last alert 11:28:04.917Z, `verification.passed` 12:58:45.126Z (`gates_run: 1`), `bead.orphaned` 12:58:55.502Z. Two slips corrected (43 post-first-kill cycles, not 42; last exit-124 12:50:14Z, not 12:50:33); the re-derivation count harmonized at three (the final pass is the third); git-history table dates re-confirmed in UTC (`git rev-list --count 00117cb..8373e5d` = 1; `bf-4x12ec` manual-close stamp 2026-08-17T14:50:41Z live). All cited documents and all cited commits (`89c66af`, `fc96211`, `0a61037`, `91e7d05`) re-checked to exist. No placeholders remain.
