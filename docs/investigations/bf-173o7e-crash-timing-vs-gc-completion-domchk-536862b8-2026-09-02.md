# Crash Timing vs GC Completion: bf-173o7e, the 14:06:16Z assigned kill

| Field | Value |
|---|---|
| **Investigating bead** | `domchk-536862b8` — "Analyze crash timing vs gc completion" |
| **Assigned crash** | `2026-08-14T14:06:16.551Z` — `heartbeat.emitted {HANDLING_RELEASE_DONE}` seq 4562 (release bookkeeping, not the kill) |
| **Actual kill** | `agent.completed` **14:06:08.828Z**, exit −1, **69.7 s** run (dispatched 14:04:58.872Z, seq 4550), dispatch **#41 of 131** |
| **Verdict** | **DURING** — the agent was killed mid-`git gc --aggressive --prune=now`, ~69.7 s in, in the pre-pack-write phase; **no gc had completed before the kill, and this attempt's gc never completed** |
| **Confidence** | HIGH for the verdict; MEDIUM-HIGH for the split correction (event-log metadata, no transcript survives) |

---

## 1. Verdict and micro-timeline (primary log, re-derived 2026-09-02)

Source: `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl`
(all times UTC).

| Time | Event | Meaning |
|---|---|---|
| 12:57:54–12:58:02 | bf-173o7e / bf-5jhvpk / bf-im2sl1 created | the gc work is **carved out as new beads** (see §4) |
| 12:58:45.113 | bf-4x12ec attempt #53 exits 0 (491.8 s) | **bead-split run, not a gc** (§4) |
| 12:59:48.629 | bf-173o7e attempt #1 killed, exit −1, 50.2 s | storm opens 63 s after the split; repo still bloated |
| … | 129 × exit −1 (21.6–216.6 s), 1 × exit 124 | identical kill signature for 10.5 h — no attempt changed the object set |
| 14:04:58.872 | dispatch #41 (`pluck`, 70,842-byte prompt) | attempt under analysis here |
| **14:06:08.828** | **`agent.completed` exit −1, 69.7 s** | **the actual kill — mid-gc** |
| 14:06:16.551 | `HANDLING_RELEASE_DONE` heartbeat seq 4562 | the **assigned** 14:06:16 timestamp (8.7 s after death) |
| 22:19:04.950 | attempt #62 exit 124 (607.6 s) | the only run to reach the 600 s cap — still mid-gc |
| 23:24:21.728 | attempt #130 killed, exit −1, 71.1 s | last kill |
| 23:25:35.038 | attempt #131 exits 0 (**40.1 s**), `verification.passed` 47 ms later | storm ends; bead orphaned 23:25:49 |

**Why the kill was mid-gc, not before or after:**

1. **The kill mechanism requires a live gc.** Exit −1 is needle's sentinel for
   "died on a signal needle did not send"; the signal was the kernel memcg-OOM
   SIGKILL inside the 12 GiB `run-*.scope` (`oom_score_adj=200`). The scope
   only reaches its ceiling if the running gc is consuming it — a 69.7 s-old
   gc over ~17.20 GiB of loose objects is exactly in its count/delta-chain
   phase, before any pack bytes are written.
2. **Nothing completed in this dispatch.** The worker's success gate
   (`verification.passed`) fired only 15 times all day, every one alongside an
   exit-0 run — nearest are 12:58:45.126 (the split run) and 23:25:35.085.
   There is no completion event of any kind between dispatch and kill.
3. **No earlier attempt had completed the gc either.** Git gc's cost scales
   with the object set. Had any of the 40 prior attempts (or any orphaned git
   child) finished a repack, the next attempt would have found a small object
   set and completed in ~40 s — exactly what the final attempt did at
   23:25:35. Instead the kill durations stayed flat (21.6–216.6 s) for 10.5 h.
   The unchanged signature *is* the evidence that the object set never shrank
   during the storm.
4. **"Was gc still running at the kill instant?" — yes**, with one nuance:
   `memory.oom.group=0` in the dispatch scope means the kernel kills the single
   highest-badness task, typically the agent process itself (hence needle
   records −1 rather than a git exit code). The git child was killed by a
   separate OOM pass or orphaned; an orphan that completed a repack would have
   flattened the later kill durations, which it did not.

## 2. The task's git-side forensics no longer exist

Every check the task prescribed returns post-crash state, not Aug-14 state:

| Check | Result today (2026-09-02) |
|---|---|
| `git reflog show --all \| grep gc` | **Reflog reaches back only to 2026-09-01 21:42** — zero Aug-14 entries. Expiry config is the 90-day default, so the Aug-14 entries were not aged out naturally; they were expunged or the reflog files recreated during the Sep-1 repo-cleanup work |
| `ls .git/objects/pack/*.pack` | **One pack, written today 11:09** (90.18 MiB + bitmap/idx/rev). No Aug-14 pack survives any later repack |
| Repo state | 93 loose objects / 752 KiB, 10,478 in-pack, `.git` 93 MB — the healthy end state, 19 days after the crash |
| `find .git -name "*gc*lock*" -o "*.lock"` | only `.git/needle-trailer.lock` (needle bookkeeping); no `gc.pid`, no `gc.log` |
| `git log -1` | HEAD `9d7e810` (Sep-2 docs). Crash-time HEAD `00117cb` (2026-08-09) verified still reachable |

**The reflog evidence the task asked for existed once and was deliberately
removed:** bf-173o7e's own Aug-17 note records *"Cleaned up invalid reflog
entries from interrupted gc"* — direct contemporary evidence that the
interrupted gc left reflog residue, expunged during the Aug-17 repair. That
note is the closest surviving thing to a git-side timestamp of the
interruption.

## 3. When the repo was *actually* packed (the completion anchor)

Not by any storm attempt. The 129 mid-gc kills prove the object set was intact
through 23:24:21Z, and the final attempt's 40.1 s success cannot itself have
packed 17.20 GiB. bf-4833lh (the storm's alert bead) states it plainly: the
operation *"was subsequently completed successfully by another process"*. The
first clean measurements all appear **Aug-17**: bf-4x12ec's closure note
(14:50:41Z) records 753 MB `.git` / 141 loose / 10,265 in-pack; bf-173o7e's
note records 445 MB / 0 loose / 7,765 in-pack; the Aug-17 16:08Z attempt found
444.24 MiB / 9 loose. So the crash's gc work was completed by a non-attempt
process some time between Aug-14 23:25 and Aug-17 — every kill in between,
including dispatch #41, interrupted the gc without losing ground.

## 4. Correction: the 12:58:45Z "completion" was the bead split, not a gc

Two committed documents state the gc completed on bf-4x12ec's 53rd attempt at
12:58:45Z ("18 GB → 753 MB"). The event log contradicts the interpretation:

- Attempt #53 was dispatched **12:50:33.212Z with `template=split`,
  prompt_len 3,896** — needle's mitosis re-dispatch, not the 70,842-byte
  `pluck` gc prompt used by every crashing attempt.
- The three child beads were created **inside its run window**
  (12:57:54–12:58:02Z); its transcript ends printing `SPLIT_COMPLETE`
  (Addendum 2, §mitosis); `verification.passed` (`gates_run: 1`) 11 ms after
  exit gated the **split**, not a repo measurement.
- The 753 MB / 141-loose figures come from bf-4x12ec's **Aug-17** closure
  notes — measured 2.5 days later, not at 12:58:45.

**Scope of the correction:** the event facts (exit 0, 491.8 s, gate passed,
orphaned 10 s later) stand; only the attribution changes. Supersedes:
`bf-4x12ec-log-review-2026-09-02.md` §Outcome ("The work … succeeded at
12:58:45Z … repo ~18 GB → 753 MB") and the same claim in
`bf-4x12ec-crash-investigation.md` §Status/Addendum 2 headline. Consequence:
the storm's opening 63 s after that exit-0 was not paradoxical — the repo was
never packed in it.

## 5. Acceptance criteria mapping

| Criterion | Result |
|---|---|
| Reflog for gc timestamps | §2 — Aug-14 entries gone (truncated to Sep-1); the interrupted gc's reflog residue was cleaned up Aug-17 per bf-173o7e's note |
| Crash time vs gc completion evidence | §1/§3 — kill 14:06:08.828Z (assigned 14:06:16.551Z is a heartbeat); completion Aug-14 23:25 → Aug-17, by a non-attempt process |
| gc objects created before/after crash | Neither — the crashed attempt wrote no pack (pre-pack-write phase); no Aug-14 pack survives |
| Pack file timestamps | All destroyed by later repacks; sole pack dates 2026-09-02 11:09 |
| Was gc still running at the kill | Yes — mid-gc, 69.7 s in; the gc's memory use is what triggered the kill |

## 6. Evidence limits

- Killed attempts write no traces and the event log carries no tool calls —
  the mid-gc phase is inferred from durations + kill mechanism, not observed.
- No kernel logs for Aug-14 (boot began Aug-15 19:26 EDT); the memcg mechanism
  is corroborated by the Aug-16 kernel record (257 `task=git`
  `CONSTRAINT_MEMCG` kills, anon-rss ~12.3 GB).
- Repo state at 14:06 is inferred (17.20 GiB loose per the bead body +
  unchanged kill signature), not measured — the Aug-25 "9.60 MB pack at
  crash" figure is a reconstruction, not a contemporaneous measurement.
- No transcript of attempt #53 survives; the split attribution rests on
  template/prompt metadata, in-window child creation, and SPLIT_COMPLETE.

---
*Determined 2026-09-02 · bead `domchk-536862b8` · primary source re-derived,
not copied.*
