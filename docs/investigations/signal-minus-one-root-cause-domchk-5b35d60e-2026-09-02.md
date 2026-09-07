# Root cause: "signal −1" agent crashes despite successful gc completion

**Investigator bead:** domchk-5b35d60e
**Subject:** the exit-code −1 agent crash family around the bf-4x12ec aggressive git gc
(alert bead bf-4833lh: "Agent crash on bead bf-4x12ec … Exit code: -1 … The git garbage
collection task from bf-4x12ec has been successfully completed despite the original agent
crash")
**Analysis date:** 2026-09-02
**Method:** primary sources — `.beads/events.jsonl`, kernel journal (`journalctl -k`, single
boot covering 2026-08-15 → present), needle 0.6.0 source (`dispatch/mod.rs`), live cgroup
inspection of the investigator's own dispatch scope, live bead records.
**Cross-references (deep dives, not re-derived here):** domchk-2539cf8c (signal semantics +
crash/completion timing), domchk-9e2aa740 (bf-4x12ec root cause), domchk-24032f23
(bf-12gb0r kill-moment anatomy), domchk-8c3fceeb (53-attempt storm report).

## Answers to the four acceptance criteria

### 1. What "signal −1" is

**It is not a signal number.** In needle 0.6.0 the agent exit path records
`status.code().unwrap_or(-1)` (`src/dispatch/mod.rs:991,996`): Rust's `ExitStatus::code()`
returns `None` exactly when the process died by a signal, so `−1` means *the agent was killed
by a signal whose number the harness does not record*. Needle's own timeout path returns `124`,
not `−1`. Re-verified today against the installed needle 0.6.0 source and the live journal:

- **Aug 14–16 storm (bf-4x12ec / bf-173o7e):** the signal was **SIGKILL(9) from the kernel
  memcg OOM killer** — 414 `Memory cgroup out of memory` kills on Aug 16 alone, all
  `constraint=CONSTRAINT_MEMCG` inside `run-*.scope` / git scopes. Host memory was never
  exhausted; the 12 GiB per-dispatch ceiling was.
- **The two post-storm crashes (bf-4833lh, Aug 17 16:00Z; bf-12gb0r, Aug 26 22:54Z):**
  **zero kernel OOM lines exist for those days.** These were process-management kills
  (worker restart churn under CPU saturation for Aug 26; Aug 17 journal window is quiet) —
  signal sent, number unrecorded, hence `−1`.

### 2. During or after gc?

Both, in distinct phases:

| Crash batch | Relative to gc | Work lost? |
|---|---|---|
| Aug 14–16 storm (245 crash events) | **During** — each attempt died while `git gc --aggressive --prune=now` ran (transcripts end at the gc call with no result); all analyzed deaths precede the eventual completion by 91 min | Yes, per attempt — until the retry finally completed |
| bf-4833lh (Aug 17) | **After** — it is the alert bead for the storm, and its own notes record gc completion (17.20 GB → 747 M, 4 627 → 27 loose objects) | No |
| bf-12gb0r (Aug 26) | **After** — bead closed **8.06 s before** the kill; post-completion cleanup termination, classified FALSE_POSITIVE by the <30 s rule | No |

The premise "crashed despite successful gc completion" is therefore confirmed: the gc
ultimately succeeded (repo verified healthy again today — 93 M `.git`, 90.18 MiB pack,
63 loose objects), and the two post-gc crashes destroyed no work.

### 3. Resource exhaustion and limits reached

- **Per-dispatch ceiling: 12 GiB, verified live on the investigator's own scope**
  (`systemctl --user show run-p3196072-… -p MemoryMax` → `12884901888`; `MemoryHigh=infinity`;
  `/proc/self/oom_score_adj` → `200`, a preferred OOM victim).
  **Pitfall:** `systemctl show` *without* `--user` reports `MemoryMax=infinity` because these
  scopes live in the systemd **user** manager — querying the system manager silently returns
  the wrong answer.
- **Fleet ceiling:** parent `needle.slice` has `MemoryHigh=24 GiB`, `MemoryMax=32 GiB` — all
  concurrent dispatch scopes share it.
- **Historical exhaustion was memcg-scoped only** (`oom-kill:constraint=CONSTRAINT_MEMCG`);
  no host-level OOM in the entire boot.
- **safe-git-gc self-containment:** `SAFE_GC_MEMORY_MAX` defaults to `2g`
  (`scripts/safe-git-gc.sh:25`); today's 6 memcg kills are the script's own ceiling firing
  inside `safe-git-gc-*.scope` (victim: the script's bash at ~63 MB RSS while git children
  held the budget) — containment working as designed, not a crash.
- **Current headroom is ample:** 51.5 GiB MemAvailable of 62.5 GiB, swap untouched,
  `ulimit` generous (524 288 fds, no memory/cpu caps).

### 4. Patterns in previous agent crashes

247 signal crashes in the bead store, all between Aug 14 and Aug 26, none since:

| Day | Signal crashes | Notes |
|---|---|---|
| 2026-08-16 | 245 | Retry storm — zero-backoff release→re-claim loop re-injecting the same hazardous gc workload; workers hit: lab-domain-check 154, lab-drawrace 41, lab-test-fix 32, lab-roam-1 20 |
| 2026-08-17 | 1 | bf-4833lh (post-gc alert bead) |
| 2026-08-26 | 1 | bf-12gb0r (post-completion false positive) |
| Aug 27 → Sep 02 | 0 | 7 days clean |

Kernel-side, only two days in the whole boot have memcg OOM kills: Aug 16 (414, the storm)
and Sep 02 (6, all safe-git-gc self-containment). Exit-code-1 failures are a **separate
class** (workflow/max-turns, not crashes) and remain elevated (44.2% Sep 01, 28.1% Sep 02) —
including this bead's own first dispatch, which failed exit 1 at 27.7 min on 2026-09-02
17:42Z and was released for retry; it was a failure, not a signal crash.

## Contributing causes (all previously identified, re-confirmed here)

1. **Containment, not host pressure:** agents are oom_score_adj=200 preferred victims inside
   12 GiB scopes, so a memory-hungry child (`git gc --aggressive` over 17 GB of loose
   objects) doomed the agent rather than the box.
2. **Zero-backoff retry loop** amplified one hazardous workload into 245 agent deaths.
3. **Alert regeneration from historical logs without bead-state checks** created
   investigation beads (including this one) days after the subject closed, and left
   bf-4833lh Open despite its resolution notes. The 2026-09-02 crash-alert fixes
   (closed-bead filtering, dedup, cooldown) target exactly this.

## Bottom line

`exit_code: −1` = "killed by an unrecorded signal." For the storm it was the memcg OOM killer
SIGKILLing agents mid-gc under the 12 GiB dispatch scope; for the two tail events it was
unrecorded process kills *after* work was already complete. The gc those crashes interrupted
succeeded, the repository is healthy, and no signal crash has occurred since Aug 26.
