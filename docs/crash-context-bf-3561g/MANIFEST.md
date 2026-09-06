# Raw crash-context extracts — bf-3561g, crash #4

**Bead:** domchk-a39caced (phase 2 of the bf-3561g chain)
**Extracted:** 2026-09-06 · **Re-verified and completed:** 2026-09-06 (retry attempt)
**Upstream catalog:** [`docs/crash-logs-catalog-bf-3561g.md`](../crash-logs-catalog-bf-3561g.md)
**Downstream analysis:** domchk-3c95693a ("Analyze root cause of bf-3561g crash") — this
directory is its input; no analysis prose is filed here.

Every file below is a **byte-faithful copy** of the named source range (no edits, no
line-number prefixes) so the analysis phase can re-parse it. Provenance lives here
instead of in the files, which would break JSONL parsing.

Target timestamp `2026-08-16T17:21:28.126979482+00:00` is the needle alert clock read
carried on alert bead domchk-90eb78b3 ("ALERT: Agent crash on bead bf-3561g"), read at
`HANDLING_RELEASE_DONE` (+8.4 µs after heartbeat `17:21:28.126971102Z`). It is **not a
log record key** — no log line carries it — so the windows below are centered on the
genuine crash-era records around it.

| File | Source | Range | Center |
|---|---|---|---|
| `session-event-log-lines-2903-3003.jsonl` | `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-b7afe97d-2026-08-16.jsonl` (3,145 records) | lines 2903–3003 (101 lines, 25.7 KB) | line 2953 `agent.completed exit_code −1` @ `17:21:27.977054297Z` |
| `needle-worker-log-lines-1966-2066.log` | `~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log` (201,380 lines) | lines 1966–2066 (101 lines, 42 KB) | lines 2013–2018, the crash window (outcome → alert → auto-retry) |
| `beads-events-lines-1477-1577.jsonl` | `.beads/events.jsonl` (14,516 records at extraction; appends only — range unaffected) | lines 1477–1577 (101 lines, 16 KB) | line 1527 crash record @ `17:21:28.132817919+00:00` (305,382 ms, exit −1) |
| `kernel-oom-kill-2026-08-16T172127Z.txt` | system journald (`journalctl -k`, local = UTC−4) | 13:21:26–13:21:29 local | the kernel memcg-OOM dump for the kill in the crash second (6.9 KB) |
| `systemd-oomd-2026-08-16T172122Z.txt` | system journald, `-t systemd-oomd` | 13:21:20–13:21:25 local | the oomd kill 5 s before the crash — system-pressure context |
| `agent-transcript-d7cd18df-head-tail.jsonl` | `~/.claude/projects/-home-coding-domain-check/d7cd18df-1799-4bce-b915-420e2e2dae73.jsonl` | first 2 + last 9 of 113 records | the crashed run's own transcript (dispatch prompt head, final tool-call tail) |

Not copied whole: the transcript (302 KB, 113 records) and the parent logs (810 KB,
90.7 MB, 2.3 MB). Ranges above are everything the crash second needs; the parents remain
on disk under the retention noted in `docs/crash-logs-catalog-bf-3561g.md` §4 — the
worker `.log` is still the live rotation slot, so **lines 2013–2018 will migrate to
`.log.1` on the next 128 MiB rotation** (recoverable there, not lost).

**What is committed vs. on-disk.** Repo policy (`AGENTS.md`/`CLAUDE.md`, born of the
bf-4yjq 17 GB-`.jsonl` OOM incident) blanket-ignores `*.log` and `*.jsonl`, so the four
log-shaped extracts above are **deliberately not committed** — they live at this path in
the worktree, byte-exact and checksummed below, for the analysis phase to parse. The two
journald extracts **are** committed (`.txt` is not ignored, and journald is the source
most likely to actually purge). Every file's bytes remain re-derivable on this box via
the Source+Range column; `.beads/events.jsonl` is append-only and never rotated
(catalog §8), and the dated session file and transcript are no longer being written.

## What the extracts show (crash #4 of bf-3561g)

### Signal −1 is a sentinel; the real kill is a memcg OOM

`exit_code −1` / `signal_code=-1` in the needle records is the outcome classifier's
sentinel for an abnormal child death, **not a signal number**. The kernel record names
the actual mechanism:

```
oom-kill:constraint=CONSTRAINT_MEMCG ... task=git,pid=2718298,uid=1001
Memory cgroup out of memory: Killed process 2718298 (git) total-vm:13161252kB,
  anon-rss:12301708kB ... oom_score_adj:200
```

- memcg `usage 12582912kB, limit 12582912kB` — the scope hit its **12 GiB `MemoryMax`
  exactly** (`failcnt 8948` charge failures before the kill).
- The victim is `git` (pid 2718298) at **anon-rss ≈ 11.7 GiB** — one process consuming
  ~98% of the scope. The dump's process tree is the dispatch scope's own tree:
  `bash(2695224) → claude(2695227, ~256 MiB) → bash(2718051) → git(2718283) →
  git(2718297) → git(2718298)`. The allocation that triggered the OOM came from a
  different `git` (pid 2718317), which does not appear in the final task dump.
- The kernel call trace (`dump_header → oom_kill_process → mem_cgroup_out_of_memory →
  try_charge_memcg → do_anonymous_page → exc_page_fault`) is the only stack trace in
  scope — there is no userspace stack: the agent was not crashing, it was waiting on a
  child that the kernel SIGKILLed.

### Agent state at crash time (transcript, session d7cd18df)

Linkage: needle dispatch `agent.dispatch` @ `17:16:22.746Z` → transcript first record
`17:16:23.214Z` (+0.47 s); needle duration 305,050 ms spans exactly that window. The
session is the Aug-16 auto-dispatch of the bf-3561g alert task itself ("ALERT: Agent
crash on bead bf-4k2ws" — the umbrella's original subject).

- `17:19:15.682Z` — `git config core.hooksPath` → `.githooks` (routine pre-flight)
- `17:19:23.191Z` — **final action, tool_use Bash: `git gc --aggressive --prune=now`**
  (timeout 300000). Last thinking block: "Good, now the git hooks path is configured."
- No further transcript activity. The gc ran 124 s before the OOM kill; the agent never
  saw an error message — it died inside the tool call.

### Exit context (needle records, all three sources agree)

| UTC | Source | Record |
|---|---|---|
| 17:21:27.977054 | session line 2953 | `agent.completed` `exit_code −1`, `duration_ms 305050` |
| 17:21:27.978613 | worker log line 2015 | `ERROR needle::outcome: agent crashed — releasing bead and creating alert bead_id=bf-3561g signal_code=-1` |
| 17:21:27.978604 | session line 2955 | `outcome.classified` `exit_code −1` → `crash` |
| 17:21:28.132739 | worker log line 2016 | `bead.outcome` INFO |
| 17:21:28.132817 | `.beads/events.jsonl` line 1527 | `event=crash` `exit_code −1` `duration_ms 305382` |
| 17:21:28.144219 | worker log line 2017 | `needle::worker: auto…` (auto-retry setup) |
| 17:21:28.148855 | worker log line 2018 | `agent.dispatch` WARN — next attempt begins |

The alert-bead clock read (target timestamp) lands inside that flush, +8.4 µs after the
`17:21:28.126971102Z` heartbeat — i.e. **~0.33 s after the agent was already dead**.

### System state near crash time

- **5 s before** (`17:21:22Z`), systemd-oomd killed a **different** dispatch scope
  (`run-p2713992-i212402347.scope`, 9.7 G current usage) because user.slice memory
  pressure was **94.29% > 80% for >20 s** with reclaim activity; `Pgscan: 1953981`.
- **At the crash second** the surviving scope `run-p2695224-i212383579.scope` (the one
  holding this agent) was itself at 100% of its 12 GiB limit, and the kernel's memcg OOM
  fired there instead.
- Read together: the box was under system-wide memory pressure with **two concurrent
  dispatch scopes resident**; one was reaped by oomd, the other by the kernel.

### Relation to the catalog's cascade framing

The catalog (§2) places crash #4 inside the 2026-08-16 **SIGHUP-cascade window**
(17:13:04–17:29:52 UTC, 9 crashes of bf-3561g, 3.5 s before the bf-6bio4g crash) and
records `fleet.cpu_saturated {core_count: 7, load_average: 15.76}` at this run's
dispatch. The extracts here sharpen that class-level attribution into a concrete
mechanism for crash #4 specifically: this run was not a passive SIGHUP victim — its own
agent invoked `git gc --aggressive --prune=now`, that gc grew to ~11.7 GiB anon inside
the 12 GiB dispatch scope, and the kernel memcg OOM killed it (the oomd kill 5 s
earlier, of a *different* scope, is the cascade's system-pressure backdrop). The same
mechanism as the Aug-14 bf-173o7e/bf-4x12ec storms; since 2026-09-02 the bare-gc path is
bounded by persistent `pack.windowMemory`/`pack.threads` git config, which did not exist
on Aug 16. The `rate_limit` WARN the catalog cites after line 2016
(`load_1min=55.38`) is inside this extract's window (lines 1966–2066).

## Verification (2026-09-06, retry attempt)

Each row re-derived from the live source on this box; `diff -q` of
`sed -n '<range>p' <source>` against the extracted file:

| File | Result |
|---|---|
| `session-event-log-lines-2903-3003.jsonl` | IDENTICAL |
| `needle-worker-log-lines-1966-2066.log` | IDENTICAL |
| `beads-events-lines-1477-1577.jsonl` | IDENTICAL |
| `kernel-oom-kill-2026-08-16T172127Z.txt` | IDENTICAL vs `journalctl -k --since 13:21:26 --until 13:21:29` (journald still holds it) |
| `systemd-oomd-2026-08-16T172122Z.txt` | IDENTICAL vs `journalctl -t systemd-oomd --since 13:21:20 --until 13:21:26` |
| `agent-transcript-d7cd18df-head-tail.jsonl` | regenerated this attempt as first 2 + last 9 (the first attempt had dropped transcript line 2, an empty `queue-operation: dequeue` record); IDENTICAL vs `{ head -2; tail -9; }` |

Corrections to the first attempt's manifest: worker-log parent count is 201,380 lines
(not 201,381); transcript extract is 11 lines. Center-record claims (session line 2953,
worker log 2013–2018, events line 1527) were re-read and hold exactly as stated.

MD5 (post-correction):

```
261ac83437283bd91d42c3f07451dc98  agent-transcript-d7cd18df-head-tail.jsonl
8048e84327f4d7bc6ae9cbd7b1116adf  beads-events-lines-1477-1577.jsonl
49ae1f995b6f09f6d9788a8831b5637d  kernel-oom-kill-2026-08-16T172127Z.txt
6f1085f982f5d57e505d3eebfc06d9c4  needle-worker-log-lines-1966-2066.log
758f09b7a61d513630de336c34df1954  session-event-log-lines-2903-3003.jsonl
16476efd35f628ffc3284542800c3910  systemd-oomd-2026-08-16T172122Z.txt
```
