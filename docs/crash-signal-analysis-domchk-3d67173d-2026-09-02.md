# Crash Signal Analysis: exit code -1 (domchk-3d67173d)

**Date:** 2026-09-02
**Bead:** domchk-3d67173d ("Analyze crash signal and system state")
**Method:** Primary evidence from kernel journal, systemd unit logs, and live cgroup
inspection — not prior crash docs (several of which are corrected below).

## Verdict

**System-related (infrastructure), not domain-check code.** `git gc --aggressive`
on the 17GB+ repository exceeded the **12 GiB memory cap of the agent session's
own systemd cgroup** and was killed by the kernel's memcg OOM killer. The agent
session then ended without a collected exit code, which the fleet records as
exit code -1. Host memory (62G) was never exhausted.

## 1. What "exit code -1" actually is

-1 is **not a signal**. It is the "no exit code was collected" sentinel: the
session process either died to an unobserved signal or its parent (the NEEDLE
worker) died first and the result was never read.

Prior repo docs state "-1 = SIGHUP cascade." That is **unsupported**. A SIGHUP
death would surface in systemd as `code=killed, signal=HUP` — and on 2026-08-26
systemd recorded **zero** signal-killed agent scopes:

```
journalctl --since 2026-08-26 --until 2026-08-27 | grep -cE "run-p.*scope.*code=killed"
→ 0
```

**Caveat:** the original crash context ("git gc --aggressive on a 17GB+ repo")
matches the bf-1s6c3 incident of 2026-08-12. The current kernel journal's first
entry is 2026-08-15 19:26 EDT, so kernel evidence for that day is unrecoverable.
Everything below is reconstructed from the surviving Aug 16 evidence of the same
operation on the same box.

## 2. OOM killer activity in system logs

All OOM kills in the current journal fall on **one day — Aug 16** — and all are
cgroup-constrained, at the **per-agent-session scope**:

```
journalctl -k | grep "invoked oom-killer" | awk '{print $1, $2}' | sort | uniq -c
→ 414 Aug 16        (828 lines incl. the paired "Killed process" lines; 0 on Aug 26)

Aug 16 09:01:52 kernel: oom-kill:constraint=CONSTRAINT_MEMCG,
  oom_memcg=/user.slice/user-1001.slice/user@1001.service/app.slice/run-p771362-i210459717.scope,
  task=git,pid=802361,uid=1001
Aug 16 09:01:52 kernel: memory: usage 12582912kB, limit 12582912kB, failcnt 54220
Aug 16 09:01:52 kernel: Memory cgroup out of memory: Killed process 802361 (git)
  total-vm:13726856kB, anon-rss:12301064kB, ... oom_score_adj:200
```

Key readings:

- **Each dispatched session runs in its own `run-p*.scope` with MemoryMax = 12 GiB**
  (12582912 kB — usage met limit exactly at kill time).
- `failcnt 54220` — the cgroup had hit the ceiling tens of thousands of times
  before the kill. This was sustained pressure, not a one-off spike.
- Victims are the heavy subprocesses agents launch: `git` (anon-rss ~12.3 GB —
  consistent with `git gc --aggressive` on a multi-GB repo) and `node`
  (vitest workers, up to 20 GB total-vm). **`claude` itself was never
  OOM-killed** — no `claude` victim appears anywhere in the journal.
- The 414 kills cluster at hours 08:00–13:00 EDT — exactly the window prior
  docs describe as the "Aug 16 SIGHUP cascade." The kernel shows what actually
  killed things then: **mass memcg OOM kills of git/vitest inside session
  scopes**, not an external hangup signal.

## 3. Memory available at crash time

| When | Host free | Binding constraint | Events |
|---|---|---|---|
| Now (2026-09-02) | 51 GiB avail of 62G | — | healthy, load 7.9 |
| Aug 26 (bead date) | no pressure logged | per-session 12 GiB cgroup | **zero OOM** |
| Aug 16 (OOM day) | host never exhausted | per-session 12 GiB cgroup | 414 kills |

Hierarchy on this box: host 62G → `user-1001.slice` MemoryHigh 32G / MemoryMax
48G → per-session scope **12 GiB**. The host and slice never bound; the
per-session cap did.

## 4. Did git gc --aggressive exceed available memory?

**Yes — but the "available memory" it exceeded is the 12 GiB session cgroup
allocation, not the host.** Kernel record: usage 12 GiB = limit 12 GiB, git
killed at 12.3 GB anon-rss.

Two follow-on constraints this exposes:

1. The sanctioned mitigation (`domain-check-git-gc.service` /
   `domain-check-git-gc-full.service`) runs with **MemoryMax=4 GiB** — *tighter*
   than the session scope. `--aggressive` on a repo with 17GB of loose objects
   provably needs >12 GiB, so it would OOM under the "safe" wrapper too. The
   staged/incremental path is fine for normal gc; `--full` on a bloated repo is
   not safe at 4 GiB.
2. A 12 GiB cap is a hard ceiling for *any* single in-session operation. Repo
   bloat prevention (the existing `.gitignore` / pre-commit hooks / weekly
   health checks) remains the only real protection, because once a repo is
   17GB-loose, no in-session repair path has enough memory.

## 5. Additional systemic findings (new)

These are not part of the original crash chain but were found while checking
system state, and they manufacture *more* -1 exits:

- **NEEDLE workers crash-loop chronically.** On Aug 26 alone:
  1015 exits with `status=72/OSFILE` (EX_OSFILE — a required file could not be
  opened), 585 with status 1, 73 with 101 (Rust panic); restart counter reached
  323+ per unit. Deaths were spread evenly across all 24 hours (36–68/hour).
- **NEEDLE's own death message is wrong.** For these deaths it prints "killed by
  an external process (e.g., SIGKILL, OOM, capacity governor)" while systemd
  simultaneously records `Main process exited, code=exited, status=72/OSFILE` —
  a normal exit with an error code, not a kill. This message has sent several
  prior investigations (including the SIGHUP-cascade theory) down the wrong path.
- **The token collector has been blind for weeks.** `claude-token-collector.service`
  (cgov) fails its collection pass every 2 minutes on a corrupted
  `~/.needle/state/collector-cursors.json` — 8,639 failures in August, 943 in
  the first two days of September, **still failing as of 2026-09-02 07:07 EDT**.
  The file has a missing opening quote at line 4812 and a mangled
  `/ho/home/...` path at line 4814 — consistent with unsynchronized concurrent
  writes. The governor cannot observe token usage until this file is removed
  (it should rebuild it) or repaired.
- No ENOSPC / disk-full events anywhere in the journal; disk is at 77%
  (101G free). Repository is currently healthy: `.git` = 92M, 25 loose objects,
  single 90 MiB pack.

## 6. Recommendations

1. **Bound `git gc` size, not just its memory.** Raise `MemoryMax` on
   `domain-check-git-gc-full.service` above the 12 GiB demonstrated requirement
   *only if* run out-of-session deliberately; otherwise treat any repo >~1GB
   loose objects as "no in-session gc" and require operator cleanup, per the
   existing repo-health thresholds.
2. **Fix the EX_OSFILE crash-loop** (needle-side). 1,700 worker deaths/day
   wakes restart loops that orphan in-flight sessions — a large standing source
   of -1 exits that has nothing to do with the tasks being run.
3. **Correct the misleading worker death message** so future investigations
   start from systemd's actual exit status.
4. **Clear the corrupt `collector-cursors.json`** (fleet-level; coordinate with
   the operator — it is shared state outside this repo) and add atomic
   write/rename to the collector to prevent recurrence.
5. **Preserve kernel evidence:** journald is volatile across boots here. Enable
   persistent journal storage (or ship OOM events to a log sink) so pre-crash
   days like Aug 12 are investigable.

## Evidence commands

```bash
journalctl -k | grep -c "invoked oom-killer"                          # 414, all Aug 16
journalctl -k | grep -m1 -A2 "oom-kill:constraint"                    # memcg + scope path
journalctl -k --since "2026-08-16 09:01:50" | grep "usage 12582912"   # 12GiB limit proof
journalctl --since 2026-08-26 | grep -cE "run-p.*scope.*code=killed"  # 0
journalctl --since 2026-08-26 | grep -c "status=72/OSFILE"            # 1015
systemctl show user-1001.slice -p MemoryMax,MemoryHigh                # 48G / 32G
systemctl --user show domain-check-git-gc-full.service -p MemoryMax   # 4G
systemctl status 2877640                                              # cgov = claude-token-collector
```
