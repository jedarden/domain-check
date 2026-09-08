# Root Cause Analysis — claude-code-glm-4.7 agent killed with exit −1

**Bead:** domchk-e8c835b8 ("Identify root cause of agent failure") — split-child 2 of 3
created by the bf-3561g run's auto-split (siblings: domchk-ee8f5300 investigate-logs,
domchk-ab71919d implement-fixes)
**Date:** 2026-09-08
**Verdict:** kernel memcg-OOM SIGKILL of the run's own `git gc --aggressive --prune=now`
inside the 12 GiB per-dispatch cgroup — **not** SIGHUP, **not** a timeout, **not** a
domain-check code defect. Confidence HIGH.
**Canonical RCA:** [`docs/crash-root-cause-bf-3561g.md`](crash-root-cause-bf-3561g.md)
(domchk-3c95693a, commit e0dfa61; live re-verification addendum §8 by domchk-1534f0bc,
commit f5f3f1e). This bead is that analysis' sibling — this document independently
re-verifies the load-bearing evidence and answers this bead's dispatch questions; it does
not supersede the canonical report.

## The crash

The claude-code-glm-4.7 worker (`claude-code-glm-4.7-lab-domain-check`) running the
bf-3561g alert investigation died at **2026-08-16T17:21:27.977Z** with
`agent.completed exit_code −1` — crash #4 of nine auto-retry crashes on that bead
(17:13:04Z → 17:29:52Z). The `−1` is needle's outcome-classifier **sentinel for an
unclassifiable abnormal child death, not a signal number**; the kernel record names the
actual mechanism.

## Acceptance criteria

**1. Failure pattern in the logs → OOM (not timeout, panic, or app error).** No stack
trace, no application error, no handler timeout. The transcript shows routine work — the
run's investigation was already finished (`transform.completed` 305,110 ms at
17:21:27.878Z, 99 ms before the death record), and the last tool_use (17:19:23.191Z,
timeout 300 s) was `git gc --aggressive --prune=now`, with 124 s of gc runtime and no
further transcript activity before the kill.

**2. Resource-related → YES: memory, and specifically the cgroup ceiling, not host RAM.**
The host was never out of memory — the constraint was the dispatch scope:

```
Aug 16 13:21:27 lab kernel: oom-kill:constraint=CONSTRAINT_MEMCG,
  oom_memcg=…/app.slice/run-p2695224-i212383579.scope, task=git, pid=2718298, uid=1001
Aug 16 13:21:27 lab kernel: Memory cgroup out of memory: Killed process 2718298 (git)
  total-vm:13161252kB, anon-rss:12301708kB (= 11.73 GiB) … oom_score_adj:200
Aug 16 13:21:27 lab kernel: memory: usage 12582912kB, limit 12582912kB, failcnt 8948
```

Both records re-read live from journald on 2026-09-08 for this bead (journald stamps
LOCAL = UTC−4, so `13:21:27` = `17:21:27Z`). Against the then-bloated repository
(~18 GB `.git`, ~17 GB loose objects) the gc's pack-objects grew to 11.73 GiB anonymous
RSS and pinned its 12 GiB `MemoryMax` scope (usage = limit, failcnt 8948).

**3. Signal termination → YES: uncatchable SIGKILL from the kernel OOM killer.** Killed
by `mem_cgroup_out_of_memory` inside `try_charge_memcg` on an anonymous-page fault —
SIGKILL cannot be handled, so the dispatch wrapper saw the child vanish and recorded the
`exit −1` sentinel. The dispatch premise "signal −1 suggests SIGHUP" is wrong; no hangup
is involved anywhere in the record.

**4. Correlation with known issues → YES: this is the established repo-bloat memcg-OOM
mechanism.** Same path as bf-4x12ec (bare `git gc --aggressive` in the 12 GiB dispatch
scope), bf-198ne (the git-push variant) and bf-173o7e. Backdrop, not cause: systemd-oomd
killed a *different* scope 5 s earlier (17:21:22Z, `run-p2713992-i212402347.scope`,
9.7 G, user.slice pressure 94.29 % — re-verified live in journald). Contributing
amplifier: needle auto-retry created a fresh ALERT bead per kill, re-dispatching
git-heavy work onto the same bloated repo (crashes #5–#9, 106/88/49/103/158 s).
Post-crash mitigations all in force since: `pack.windowMemory=2g`/`pack.threads=1` git
config (2026-09-02), `safe-git-gc.sh` bounds, repo repaired to ~94–107 MB
(re-verified 2026-09-06/08).

**5. Hypothesis (confirmed) → the crash was self-inflicted:** the run's own bare,
memory-unbounded `git gc --aggressive --prune=now` over an 18 GB repository exceeded the
12 GiB per-dispatch memcg limit and was SIGKILLed by the kernel; needle recorded the
uncatchable death as `exit −1` and auto-retried into the same conditions eight more
times. The final attempt (dispatch #10, 17:31:56Z) succeeded with exit 0 once memory
pressure eased. Not contributing: domain-check code (the process died inside a git
subprocess), bead-store corruption, SIGHUP, and — notably — the alert's own target:
bf-4k2ws had completed successfully (exit 0, 15:35:42Z that day) before this alert
chain even began, making the original alert false from creation.

## Disposition

This bead's scope is fully covered by the canonical RCA (domchk-3c95693a, Closed), whose
sibling-investigation leg (domchk-ee8f5300, Closed) produced the checksummed artifacts and
log catalog. Nothing unique remained to investigate; the implement-fixes sibling
(domchk-ab71919d) and the umbrella (domchk-d06cb3e6) are separate beads not closed here.
Evidence extracts live at [`docs/crash-context-bf-3561g/`](crash-context-bf-3561g/MANIFEST.md).
