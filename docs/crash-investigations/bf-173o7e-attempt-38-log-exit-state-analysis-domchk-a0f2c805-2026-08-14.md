# Crash Log & Exit State Analysis — 2026-08-14T14:02:25Z

**Investigation bead:** `domchk-a0f2c805` · **Analysis date:** 2026-09-02
**Crash identified as:** attempt **#38 of the bf-173o7e Aug-14 dispatch storm** (worker `claude-code-glm-4.7-lab-domain-check`, session `a6dbb1fc`)
**Root cause:** kernel **memcg OOM SIGKILL** inside the 12 GiB dispatch scope — identical mechanism to the rest of the storm (see [`bf-173o7e-aug14-storm-root-cause-2026-09-02.md`](bf-173o7e-aug14-storm-root-cause-2026-09-02.md), [`../analysis/signal-analysis.md`](../analysis/signal-analysis.md))
**Confidence:** HIGH — event-log signature re-derived directly from the primary needle log

---

## 1. The assigned timestamp is not the kill time

The task's timestamp `2026-08-14T14:02:25.576775668+00:00` matches — to within
13 µs — the `heartbeat.emitted {state: HANDLING_RELEASE_DONE}` event at
`14:02:25.576762535Z` (sequence 4502) in the primary needle event log. The
agent itself died **24.4 s earlier**:

| Event | Timestamp (UTC) |
|---|---|
| `agent.dispatched` (attempt #38, prompt `sha256:c995c129…`, 70 842 B, pluck template) | 2026-08-14T14:01:18.413934592Z |
| **`agent.completed` — the actual kill** (exit −1, duration 42 231 ms) | **2026-08-14T14:02:01.202143405Z** |
| `HANDLING_FLUSH_DONE` / `HANDLING_RELEASE` | 2026-08-14T14:02:17.644287200Z |
| `HANDLING_RELEASE_DONE` — **the alert timestamp** (13 µs offset) | 2026-08-14T14:02:25.576762535Z |
| next `agent.dispatched` (attempt #39) | 2026-08-14T14:02:36.697443241Z |

This is the **third confirmed instance** of the alert-timestamp pattern in this
storm (after `domchk-17ca8b7d`: heartbeat 34.3 s after kill; `domchk-0eea1a4b`:
7.7 s). Release-bookkeeping heartbeats, not the kill, generate alert-family
timestamps — the observed gap range is now **7.7–120 s** after `agent.completed`.

## 2. What exit −1 / "signal −1" means

Not a signal number — no Unix signal is −1 (signals are 1…64; shell convention
maps a signal death to `128 + N`). Needle records
`ExitStatus::code().unwrap_or(-1)` (`src/dispatch/mod.rs:991,996`): Rust's
`ExitStatus::code()` returns `None` **exactly when the process died on a
signal**, so `−1` is the harness sentinel for *"the agent died on a signal
needle did not send"*. Any claim that "signal −1" names a specific signal is a
misreading of the sentinel.

The signal behind this storm's `−1`s is **SIGKILL (9)** from the kernel's
memory-cgroup OOM killer: the agent ran in a transient `run-p*.scope` with
`MemoryMax=12 GiB` (re-verified live on this box: agent scopes 12 884 901 888
bytes, test scopes 6 GiB) and `oom_score_adj=200`. Needle's timeout path
produces `124`, not `−1` — and `124` appears once in this very storm (607.6 s),
proving the paths are distinguishable. Ruled out for this kill: SIGHUP cascade
(other workers completed cleanly in the same window, §4), timeout governor,
host-wide OOM, domain-check code defect.

## 3. Placement within the storm

From `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl`
(session `a6dbb1fc`, booted 05:18:53Z):

- **132** `agent.dispatched` events for `bf-173o7e` (first 12:58:58.215Z, last 23:24:54.871Z)
- **131** `agent.completed`: **129 × exit −1**, **1 × exit 124** (timeout), **1 × exit 0** (orphaned-on-success)
- The analyzed crash is the **38th dispatch** of that sequence (counting basis:
  dispatch order in this log; the `#41` cited for the 14:06:08 kill in
  `domchk-0eea1a4b` differs by 2 due to counting basis, not substance)
- Every attempt re-injected the **identical prompt** (same `prompt_hash`), the
  signature of needle's zero-backoff release→re-claim loop
- 42.2 s attempt duration sits in the storm's typical 21–217 s kill band —
  the agent was SIGKILLed mid-`git gc --aggressive` against a repo then carrying
  ~17 GiB of loose objects (workspace HEAD at crash time: `00117cb`, verified
  an ancestor of current HEAD — no work was lost)

## 4. Correlated events at crash time (14:01–14:03Z, all workers)

| Worker | Bead | Completion | Exit | Duration | Reading |
|---|---|---|---|---|---|
| lab-drawrace | bf-12lgi | 14:01:14.377Z | **−1** | 43.2 s | same signature: short-lived scope SIGKILL, 47 s before ours |
| **lab-domain-check** | **bf-173o7e** | **14:02:01.202Z** | **−1** | **42.2 s** | **this crash** |
| lab-roam-2 | bf-3iaht | 14:02:00.288Z | 124 | 600.0 s | clean governor timeout, same second as our kill |
| lab-roam-1 | bf-3iaht | 14:02:17.187Z → 14:02:34.092Z | 1 → 0 | — | retry succeeded 33 s later |
| lab-test-fix | bf-317iuc | 14:03:33.006Z | 124 | 600.0 s | clean governor timeout |

Two `−1` kills 47 s apart on different workers, while neighbours timed out or
succeeded cleanly in the same seconds: **not a fleet-wide SIGHUP/systemic
event**. Each worker's agent runs in its own 12 GiB scope, so the kills are
scope-local memory exhaustion; the near-simultaneity reflects two workers
running heavy git work concurrently on one box (~45 GiB host RAM was free
mid-storm per bf-4x12ec §3), not a shared kill.

## 5. Available log evidence — and what is gone

Available:

- **Primary:** `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl`
  (2.6 MB; 4 975 events in session `a6dbb1fc`) — all events cited above
- Sibling worker logs `…-2026-08-14.jsonl` for lab-drawrace, lab-roam-1/2,
  lab-s1, lab-test-fix (correlation table, §4)
- No crash dumps of any kind from Aug 14 (see limits below) — the event log is
  the sole primary witness

Evidence limits (confirmed live 2026-09-02):

- **No kernel logs survive:** current boot began 2026-08-15 19:26 EDT;
  `journalctl --since "2026-08-14 …"` returns nothing. The memcg-OOM mechanism
  therefore rests on the event-log signature plus kernel proof of the identical
  mechanism in the same period (257 `task=git` `CONSTRAINT_MEMCG` kills on
  Aug-16, anon-rss hugging the 12 GiB cap — bf-4x12ec §2)
- **No coredumps before Aug 25**; needle traces are single-slot (last dispatch
  only); `.beads/logs/` starts Sep 1 — nothing covers Aug 14

## 6. Known failure mode — yes, and already resolved

This crash is a fully characterized, previously documented failure mode:
memcg OOM during `git gc --aggressive` in the 12 GiB dispatch scope, amplified
by the zero-backoff retry loop into a 131-attempt storm. Not a code defect —
domain-check code is cleared in every investigation of this family. Current
repo state confirms the remediation held: 93 loose objects / 752 KiB, single
90.18 MiB pack, zero crashes since Aug 26.
