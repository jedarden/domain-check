# Root Cause Determination: bf-173o7e Aug-14 `exit −1` event

| Field | Value |
|---|---|
| **Crash bead** | `bf-173o7e` — "Execute git gc --aggressive with pruning" (task / P2, created 2026-08-14T12:57:54Z, closed 2026-08-17T17:15:23Z) |
| **Assigned event** | `exit_code = −1` at "2026-08-14T13:21:05.530129268+00:00" (investigation bead `domchk-17ca8b7d`) |
| **Actual kill** | `agent.completed` **2026-08-14T13:20:31.223873707Z**, exit −1, duration 119.602 s (dispatched 13:18:31.368Z), classified `crash` |
| **Determination date** | 2026-09-02 · bead `domchk-17ca8b7d` |
| **Root cause** | Kernel **memcg OOM kill** — identical mechanism to `bf-4x12ec` |
| **Confidence** | HIGH (event-log signature directly re-derived; identical mechanism kernel-verified same period; Aug-14 kernel logs rotated away — see Evidence Limits) |

---

## 1. The assigned timestamp is not the kill time

The task's crash timestamp matches — to nanosecond serialization — the
`heartbeat.emitted {state: HANDLING_RELEASE_DONE}` event at
`13:21:05.530122531Z` in the primary needle event log. The agent itself died
**34.3 s earlier**. Alert-family timestamps are release bookkeeping, not the
kill; second confirmed example from the same storm: the alert cited in
[`bf-173o7e-false-positive.md`](bf-173o7e-false-positive.md)
(`21:44:27.262339275Z`) trails its actual completion
(`21:42:29.847909541Z`, exit −1, 36.3 s) by ~118 s. Always re-derive the kill
time from `agent.completed` in
`~/.needle/logs/claude-code-glm-4.7-lab-<worker>-<date>.jsonl`.

Known assigned-timestamp → actual-kill mappings for this storm (re-derive any
new one the same way; do not treat the assigned value as the kill):

| Assigned timestamp | Event it serializes | Actual kill (`agent.completed`, exit −1) | Attempt |
|---|---|---|---|
| 13:21:05.530Z | `HANDLING_RELEASE_DONE` heartbeat | 13:20:31.223Z (119.6 s run) | 1st hour-13Z kill analysed by domchk-17ca8b7d |
| **14:06:16.551Z** | `HANDLING_RELEASE_DONE` heartbeat (seq 4562) | **14:06:08.828Z** (69.7 s run; dispatched 14:04:58.872Z, seq 4550) | **dispatch #41 of the storm** |
| 21:44:27.262Z | crash-alert creation | 21:42:29.847Z (36.3 s run) | hour-21Z attempt |
| 13:55:36.566Z | `HANDLING_RELEASE_DONE` heartbeat | 13:55:24.357Z (71.7 s run; claimed 13:54:12.622Z, dispatched 13:54:12.637Z) | cycle 4 — the crash subject of `domchk-760530a8` (re-derived under its sibling `domchk-31e43626`) |

The 14:06:08 kill is isolated: the only other fleet completion within
±30 s is `lab-s1` finishing `bf-3ev0q` at 14:06:01 with **exit 124** (the
600 s cap) — an unrelated timeout on a different worker, not a second crash.

The 13:55:24 kill is the storm's most direct **mid-gc proof**: its agent
transcript (`4a6daee4-aa55-4a1b-a276-71cd1ecc9cc0.jsonl` under
`~/.claude/projects/-home-coding-domain-check/`) spans exactly the run
(13:54:13.645Z → 13:55:13.665Z) and ends with the assistant text *"The git gc
process is running successfully. Let me wait a bit more"* followed by a
`sleep 10 && tail -50 /tmp/git-gc-domain-check.log` progress check — the agent
was SIGKILLed 10.7 s later while waiting on the gc it had launched inside the
same 12 GiB scope. This cycle's transcript is truncated flush-by-kill (the
others end at the gc call itself), which is why it shows the wait state
explicitly.

## 2. What `exit −1` means

Not a signal number. Needle records `ExitStatus::code().unwrap_or(-1)` — the
sentinel for **"the agent died on a signal needle did not send"**
(source-level decode: [`../analysis/signal-analysis.md`](../analysis/signal-analysis.md)).
The signal here was SIGKILL from the kernel's memory-cgroup OOM killer.

## 3. The assigned crash was one of 131 attempts — a storm never previously analyzed

Re-derived from `claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl` (all
times UTC):

| Hour | Attempts | Kill-duration range |
|---|---|---|
| 12Z (12:58:58 claim) | 1 | 50.2 s |
| 13Z | 35 | 40.4 – 214.3 s |
| 14Z | 17 | 37.5 – 123.9 s |
| *15Z–19Z* | *0* | *(storm paused)* |
| 20Z | 1 | 58.9 s |
| 21Z | 45 | 28.2 – 128.5 s |
| 22Z | 15 | 37.6 – **607.6 s** |
| 23Z | 17 | 38.7 – 198.9 s |

**Total: 131 attempts — 129 × exit −1 (21.6–216.6 s), 1 × exit 124 (607.6 s,
the 600 s cap, at 22:19:04Z), 1 × exit 0 (40.1 s, 23:25:35Z).**
Dispatch/completion reconciliation (verified 2026-09-02): the log holds **132
`agent.dispatched` vs 131 `agent.completed` events for `bf-173o7e`** — one
dispatch never recorded a completion; the exit breakdown otherwise matches
exactly. The bead was
then `bead.orphaned` at 23:25:49 — released unassigned on success, the same
worker-side defect documented for bf-4x12ec. The storm opened **13 s after**
sibling bead `bf-4x12ec`'s 53rd attempt exited 0 (12:58:45Z): bf-173o7e was a
duplicate of the same gc task, and its body prescribed the same lethal
command — `git gc --aggressive --prune=now` over 17.20 GiB of loose objects.

## 4. Root cause

Same as [bf-4x12ec](bf-4x12ec-root-cause.md), which carries the full
mechanism analysis and is not repeated here: `git gc --aggressive` computes
delta chains across the whole object set in memory before writing pack
bytes; inside needle's transient `run-*.scope` (`MemoryMax=12 GiB`,
`oom_score_adj=200`, `memory.oom.group=0`) the scope budget is exhausted in
tens of seconds and the kernel SIGKILLs the highest-badness task — on
Aug-14 typically the agent process itself, which is why needle recorded
`exit −1` rather than a git exit code. Kernel proof of the identical
mechanism in the same period: **257 `task=git` `CONSTRAINT_MEMCG` kills
(Aug-16), anon-rss median 12,301,364 kB hugging the 12 GiB cap**
(bf-4x12ec-root-cause.md §2).

## 5. Alternatives ruled out (this bead's own data)

| Candidate | Verdict | Basis |
|---|---|---|
| SIGHUP cascade / fleet-wide event | ❌ | At 13:20:31 exactly one agent died; the nearest other completion is `roam-1` finishing `bf-59csb` with **exit 0** 0.4 s earlier |
| Timeout governor | ❌ | 129 kills at 21–217 s sit far below the 600 s cap that one surviving run visibly reached (exit 124 at 607.6 s) — supersedes the "timeout monitor" guess in [`bf-173o7e-false-positive.md`](bf-173o7e-false-positive.md) |
| Host-wide OOM | ❌ | Non sequitur for memcg kills (scope budget only; ~45 Gi host RAM free mid-storm, per bf-4x12ec §3) |
| domain-check code defect | ❌ | The crashing process was the agent scope running a git command embedded in the bead text |
| The Aug-17 `exit 1` / `max_turns` event | different event | Post-completion bead-close exhaustion, already analyzed by [`bf-173o7e-root-cause.md`](bf-173o7e-root-cause.md); its "exit code was 1, NOT −1" statements apply only to Aug-17 |

## 6. Evidence limits

- **No kernel logs survive for Aug-14** — current boot began 2026-08-15
  19:26 EDT; `journalctl --since "2026-08-14 12:00" --until "... 14:00"`
  returns no entries (re-confirmed 2026-09-02). Snapshot `journalctl -k`
  first in every future investigation.
- **No traces for killed attempts** — `.beads/traces/bf-173o7e/` holds only
  the Aug-17 run; a SIGKILLed dispatch writes no trace files.
- The Aug-16 kernel kills corroborate the mechanism but **predate that
  day's first agent completions (04:23Z)**; they belong to cleanup-window gc
  runs in dispatch scopes, not agent dispatches.

## 7. Disposition

- **Work: complete.** The repo was fully packed by the storm's end and is
  healthy today (58 loose objects / 488 KiB, 10,478 in-pack, 90.18 MiB pack,
  `.git` 93 MB — verified live 2026-09-02).
- **Task bead `domchk-17ca8b7d` was itself an artifact of the stale-alert
  regeneration defect** — created 2026-08-26, twelve days after the crash,
  against an already-closed bead (see bf-4x12ec-root-cause.md §"Why It Took
  53 Attempts", item 4; fixes: closed-bead filtering, dedup, cooldown in
  `scripts/crash-alert-manager.sh`).
- **Mitigations holding:** bare `git gc --aggressive` banned in favour of
  `scripts/safe-git-gc.sh`; `scripts/crash-circuit-breaker.sh` exists for
  same-cause kill loops.

---

**Determination:** memcg OOM → SIGKILL inside the dispatch scope → needle
`exit −1` sentinel. INFRASTRUCTURE. Determined 2026-09-02 · bead
`domchk-17ca8b7d`.
