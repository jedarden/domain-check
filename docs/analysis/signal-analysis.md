# Signal Analysis: exit code −1 and the bf-4x12ec crash-vs-resolution timing

**Investigator bead:** domchk-2539cf8c
**Subject bead:** bf-4x12ec ("Execute aggressive git garbage collection to eliminate OOM risk", Closed 2026-08-17)
**Analysis date:** 2026-09-02
**Method:** primary sources only — needle worker event log for 2026-08-14, needle 0.6.0 source
(`/home/coding/scratch/needle-src-ro-20260902`), per-attempt Claude Code transcripts, current-boot
kernel journal, live cgroup inspection, live bead records. Every number below was re-derived from
these sources for this analysis; prior addenda were used as cross-checks, not as inputs.

---

## 1. Executive summary

1. **"Signal −1" is not a signal.** `exit_code: -1` in the needle event stream is the harness's
   sentinel for *"the agent process terminated without an exit status"* — i.e. it was killed by a
   signal whose number the harness does not record. It is produced by
   `status.code().unwrap_or(-1)` in the agent-exit path (`src/dispatch/mod.rs:991,996`): Rust's
   `ExitStatus::code()` returns `None` exactly when the process died by signal.
2. **The signal was SIGKILL (9), sent by the kernel's memcg OOM killer.** The agent ran inside a
   transient `run-p*.scope` capped at `MemoryMax=12 GiB` (verified live on this box today; test
   scopes get 6 GiB) and is marked `oom_score_adj=200`, a preferred OOM victim. Needle's own
   timeout path would have produced `124`, not `−1` — and phase 2 of the same bead shows that path
   working, 8 times.
3. **The crash is not a post-completion event.** All 44 signal deaths ended at 11:27:26Z; the work
   completed at 12:58:45Z — 91 minutes later. The kills happened mid-task, each attempt dying while
   `git gc --aggressive --prune=now` was running (transcripts end with the gc tool call and no
   result).
4. **No crash-vs-completion race exists — but three genuine synchronization defects do:**
   a zero-backoff release→re-claim retry loop that re-injected the identical hazardous workload 44
   times (173 times counting the child bead); alert regeneration from historical logs without a
   bead-state check (the reason this very bead was dispatched 9.3 days after the subject bead
   closed); and single-task OOM victim selection (`memory.oom.group=0`) that makes *which* process
   dies nondeterministic while the *cause* stays constant.

---

## 2. What `exit_code: -1` is, at the source level

Needle 0.6.0's agent execution waits on the child and maps its exit status:

```rust
// src/dispatch/mod.rs (needle 0.6.0)
let exit_code = if timeout_dur.is_zero() {
    let status = child.wait().await ...;
    status.code().unwrap_or(-1)            // ← −1 is born here
} else {
    match tokio::time::timeout(timeout_dur, child.wait()).await {
        Ok(Ok(status))  => { kill_guard.disarm(); status.code().unwrap_or(-1) }
        Err(_)          => { libc::killpg(pid, libc::SIGKILL); ...; 124 }   // harness timeout
    }
};
```

Three facts follow directly from this code:

| Observed code | Meaning | Evidence path |
|---|---|---|
| `0` | normal exit, success | `Outcome::Success` |
| `1`, `2–128` | normal exit with failure code | `Outcome::Failure` |
| `124` | **needle's own timeout**: process group SIGKILLed by the harness at the configured cap | distinct code path, hardcoded `124` |
| `−1` | `ExitStatus::code() == None` → **terminated by a signal not initiated by needle's timeout path**; the signal number is discarded | `unwrap_or(-1)` |

The outcome classifier confirms −1 is deliberately "abnormal termination":

```rust
// src/types/mod.rs:252
i32::MIN..=-1 => Outcome::Crash(exit_code),   // negative → "crash"
124 => Outcome::Timeout,                      // GNU-timeout convention
```

So the log line `{"exit_code":-1,"outcome":"crash"}` decodes as: *the agent process was killed by a
signal that needle did not send as part of a timeout, and needle does not know which one.* Note the
transform pipeline *does* preserve the signal number in its error string
(`"terminated by signal {sig} (not initiated by needle)"`, `dispatch/mod.rs:1120-1127`) — the agent
path simply never records it. Any claim that "signal −1" identifies a specific signal is a
misreading of the sentinel.

**Which signal, then?** SIGKILL is the only candidate consistent with the evidence:

- Death was immediate, with zero stderr and no transcript error (44/44 attempts; SIGTERM/SIGHUP
  would produce a graceful Claude Code shutdown message or at least partial output).
- No core dumps anywhere in the window (SIGKILL cannot be caught or dump).
- Needle's `kill_guard`/worker shutdown is excluded: the worker kept running and re-claimed the bead
  13.8 s after every death — it was not the killer.
- The kernel's memcg OOM killer kills with SIGKILL and is *known active on this box in exactly this
  configuration* (§3).

---

## 3. Why the SIGKILL was sent: cgroup memory exhaustion, not host OOM

**Live cgroup inspection (2026-09-02):** every transient `run-p*.scope` on this box carries a hard
memory cap. Agent-dispatch scopes (those running `claude --print` under `bash -c`) get
`MemoryMax=12884901888` (12 GiB); test-runner scopes get `6442450944` (6 GiB). Both also show
`MemoryHigh=infinity`. `needle-worker@.service` itself is unlimited — the cap that bites is the
per-dispatch scope.

**The kernel's behavior in these scopes is observable and was fatal 413 times on Aug-16 alone:**

- `journalctl -k` for 2026-08-16: **413 `Killed process` events** — 257 `git` (anon-rss at kill
  1.2–12.0 GB, the majority hugging an ~11–12 GB ceiling), 156 `node`/vitest — all
  `constraint=CONSTRAINT_MEMCG`, `oom_score_adj=200`, inside `run-p*.scope` memcgs. No git OOM kill
  occurs on any other day of the current boot.
- These Aug-16 kills are the *same cleanup effort two days later* with kernel logging available:
  identical command, identical repository, identical scope cap.

**Direct evidence from inside the Aug-14 window** (attempt transcript `9539f3b2…`, tool output
captured 10:43:59Z — mid-storm):

```
Filesystem  444G  355G  67G  85% /
Mem:        62Gi  16Gi  24Gi  (buff/cache 22Gi)  available 45Gi
Swap:       24Gi  0B    24Gi
```

Twenty minutes into the storm the host had **45 GiB available and zero swap pressure — and the next
gc attempt was still killed ~55 s after launch.** That single observation eliminates host-wide
memory exhaustion and pins the kill to the scope-local cap: `git gc --aggressive` delta-solving an
18 GB loose-object repository cannot fit under 12 GiB, so the kernel killed a task inside the
scope while the host had plenty to spare.

**Agent transcript shape (44/44 attempts):** `git count-objects -vH` → 4,649 loose / 17.20 GiB;
`du -sh .git` → 18G; then `git gc --aggressive --prune=now` (tool timeout 600000) — **transcript
ends, no tool result**. Example verified here: `8b2a5b0d…` (attempt 1) and `9539f3b2…` (mid-storm).

**Why the agent was the victim, not `git`:** with `memory.oom.group=0` (verified on these scopes)
the kernel kills the single highest-badness *task* in the hitting memcg. On Aug-16, gc processes
lived long enough to outgrow the agent and were usually the victim — the agent survived, recorded a
tool failure, and could retry. On Aug-14, deaths came 39–116 s into each attempt, before `git`
exceeded the node-based agent's RSS; the kernel selected the agent (or its `bash -c` parent) 44
times out of 44. Same cause, different victim — which is why the needle record shows the *agent*
dying with `−1` instead of a git exit code.

**Evidence-window caveat (unchanged from prior addenda, re-verified):** the current boot began
2026-08-15 19:26 EDT and archived journals start 19:31 — no kernel log covers Aug-14. The
determination rests on (a) the source-level meaning of `−1`, (b) 44/44 mid-gc transcript endings,
(c) the 45-GiB-host-yet-killed observation, and (d) 413 same-mechanism kernel kills two days later.
This is corroborated inference, not direct Aug-14 kernel telemetry.

---

## 4. Timeline: crash vs bf-4x12ec resolution (2026-08-14, UTC)

Re-derived from `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl`
(53 `agent.completed` events for bf-4x12ec: 44 × −1, 8 × 124, 1 × 0):

| Time | Event |
|---|---|
| 10:17:26.387 | Bead created (P2) |
| 10:21:06.969 | First claim (`prompt_len` 71,698); dispatched 0.04 s later |
| 10:21:41 / 10:22:09 | Tool-level failures inside attempt 1: `gc.aggressivewindow=1.hour` rejected, exit 128, twice |
| 10:22:18 | Agent unsets the bad config; 10:22:36 launches the gc |
| **10:23:02.958** | **Attempt 1 dies — exit −1 after 115.8 s, mid-gc.** Classified `crash` +1 ms; released +11.3 s; re-claimed +13.8 s; re-dispatched with the identical prompt |
| 10:23:02 → 11:27:26 | **Phase 1: 44 signal deaths in 64.4 min.** Durations 38.9–115.8 s (mean 64.6); death→death 53.3–135.6 s (mean 89.8) |
| 10:43:59 | Host snapshot mid-storm: 45 GiB available (§3) — attempts keep dying |
| 11:38:07 → 12:50:14 | **Phase 2: 8 attempts, each exactly 600.0 s → exit 124** (needle's timeout path, working as coded). Gap after phase 1: 10.7 min |
| **12:58:45.113** | **Phase 3: 53rd attempt exits 0 after 491.8 s.** It did not run the gc — it executed needle's auto-split template: created children bf-173o7e (gc) / bf-5jhvpk (repack) / bf-im2sl1 (verify), chained them, `SPLIT_COMPLETE`. `verification.passed` fires the same second; `bead.orphaned` +10.4 s |
| 12:59:48 | Child bf-173o7e's first attempt dies (exit −1) — the same storm continues on the child: 129 × −1 on Aug-14 alone (12:59:48→23:25:35) |
| later | The actual `git gc` completes under bf-173o7e: 18 GB → 753 MB, 4,649 → 141 loose objects |
| 2026-08-17 14:50:41 | bf-4x12ec formally closed (73.9 h after completion) |
| 2026-08-26 21:28:22 | **This analysis' alert bead (domchk-2539cf8c) created** — 12.4 days after completion, 9.3 days after closure |

**Headline latencies:**

- first crash → work complete: **2.60 h**
- last signal death → work complete: **91 min** (the decisive interval for §5.1)
- work complete → formal closure: 73.9 h (notes-writing, not work)
- closure → this alert: 9.3 days (§5.2)

Context: the box was simultaneously CPU-saturated — 45 `fleet.cpu_saturated` events inside the
phase-1 window, load 10.37–30.92 (mean 14.5 on a 12-core box) — easing to ~8–10 by the successful
attempt, the same "easing pressure" progression the three phases trace.

---

## 5. Race-condition assessment

The task asks whether the crash is timing-related. Five distinct timing questions were tested
against the event log; verdicts:

### 5.1 Crash-vs-completion race — **NOT PRESENT**

All 44 deaths precede completion by ≥91 minutes. The kill always arrived *during* the hazardous
work, never after it. This distinguishes bf-4x12ec from the confirmed post-completion class (e.g.
bf-12gb0r, killed 8.06 s after its bead closed): here there is no "work finished, kill arrived late"
moment to explain. The kills were caused by the work, not mistimed relative to it.

### 5.2 Alert-vs-bead-state race — **PRESENT** (and it is why this bead exists)

The subject bead has been Closed since 2026-08-17T14:50:41Z, yet an alert generated
domchk-2539cf8c on 2026-08-26 — a monitor regenerating an "agent crashed" alert from a historical
needle-log record without consulting current bead state. At alert time the crash was 12.4 days old,
fully investigated (six prior addenda), and its work complete. This is a genuine
synchronization defect between the crash-monitor scan and the bead store, documented and fixed in
the 2026-09-02 alert system rework (closed-bead filtering in `scripts/crash-alert-manager.sh`).
Classification of this dispatch: **FALSE POSITIVE duplicate alert** — the underlying crash analysis
is real and correct; the alert was stale.

### 5.3 Retry-loop synchronization — **PRESENT** (the significant engineering finding)

The death→re-claim sequence is fully deterministic and has no backoff:

```
agent.completed(exit −1)          T
outcome.classified("crash")       T + 0.001 s
bead.released                     T + 11.3 s
outcome.handled(action=alerted)   T + 11.3 s
bead.claim.succeeded (same worker) T + 13.8 s
agent.dispatched (same prompt, 71,698 chars) T + 13.8 s
```

Mean cycle: one death every 89.8 s for 64 minutes — the same worker, the same prompt, the same
hazardous command, 44 times, with no failure-count escalation in the retry path. Escalation
eventually came from *outside* the retry mechanism (the prompt-template switch to auto-split, and
only after 8 zero-output timeouts). The loop also amplified the load problem it was drowning in:
45 `cpu_saturated` events during the window mean each re-dispatch landed on an already-saturated
box, and the storm then propagated to the child bead (173 same-command signal deaths on Aug-14
across parent and child). A per-bead consecutive-crash breaker (2–3 failures → defer/auto-split,
plus dispatch backoff) would have cut the storm from 173 attempts to a handful.

### 5.4 Cross-bead / cross-worker kill correlation — **NOT PRESENT**

Whole-day histogram of this worker's 274 signal deaths shows deaths in many hours (other beads:
bf-65lsdu ×36, bf-59bwz ×27, …), but the 10:20–13:00 window contains exactly **45 deaths: 44 ×
bf-4x12ec + 1 × bf-173o7e (12:59:48, the child created by attempt 53)**. No other bead's
completion falls within ±3 s of any of the 44 (prior addenda's 0/44 check). The kills are
per-scope memcg events; there is no evidence of interference from other workers' processes, and
the storm never left bf-4x12ec's own retry cycle.

### 5.5 Victim-selection nondeterminism — **PRESENT, benign but misleading**

`memory.oom.group=0` makes the kernel pick one task per OOM event. On Aug-14 the victim was the
agent process (agent records `exit −1`); on Aug-16 it was usually `git` (agent survives, retries).
Investigators reading only the needle record see "agent crashed"; reading only the Aug-16 kernel
log would conclude "git crashes". Both are the same memcg exhaustion. Any monitoring that keys on
*which* process died will miscount this kill class; key on the scope hitting its cap instead.

---

## 6. Conclusions

- `exit_code −1` on bf-4x12ec = the needle harness recording a **signal death it did not cause**;
  the signal was **SIGKILL from the kernel memcg OOM killer** as `git gc --aggressive` blew through
  the 12 GiB `MemoryMax` of the agent's dispatch scope, 44 times in 64 minutes.
- The crash is **not a race**: every kill was mid-work, 91+ minutes before completion, uncorrelated
  with any other bead.
- The timing problems are elsewhere and are all **fixable mechanism, not mystery**: (1) a
  zero-backoff same-worker re-claim loop that amplified one environmental hazard into 173 attempts
  and propagated it to a child bead; (2) alert regeneration from historical logs without a bead-state
  check, which produced this duplicate-alert dispatch 9.3 days after closure; (3) single-victim OOM
  semantics that make the recorded victim (agent vs git) unrepresentative of the cause.
- The work itself (18 GB → 753 MB repo, OOM risk eliminated) completed under child bf-173o7e; the
  repository is 92 MB / ~20–43 loose objects as of 2026-09-02. No domain-check code was involved at
  any point.

## 7. Acceptance criteria

- [x] Signal −1 root cause identified — §2 (sentinel semantics, source-level) and §3 (SIGKILL via
      memcg OOM under `MemoryMax=12 GiB`, with the host-had-45-GiB proof)
- [x] Timeline of crash vs bf-4x12ec resolution — §4 (all 53 attempts, three phases, resolution via
      auto-split child, closure and alert latencies)
- [x] Race condition assessment — §5 (five timing hypotheses tested; 2 confirmed defects, 3 cleared)
- [x] Signal analysis document — this file

## 8. Cross-references

Consolidates and cross-checks, from primary sources: the bf-4x12ec investigation report Addenda 2–6
(`docs/crash-investigations/bf-4x12ec-crash-investigation.md`), the artifacts summary
(`docs/crash-investigations/bf-4x12ec-crash-artifacts-2026-09-02.md`), the generic signal explainer
`docs/signal-analysis-exit-code-negative-one.md` (domchk-e4a11c19), and
`docs/research/root-cause-analysis-signal-minus-one-crashes.md`. The retry-storm table in Addendum 2
and the memcg mechanism in Addendum 3 reproduced exactly under independent re-derivation here; the
live scope inspection this analysis performed confirms Addendum 3's `MemoryMax=12 GiB` finding
(12884901888 bytes observed on agent scopes, 6 GiB on test scopes).

**Sources:** needle worker event log 2026-08-14 (all six workers scanned for §5.4);
needle 0.6.0 source (`src/dispatch/mod.rs`, `src/types/mod.rs`); Claude Code session transcripts
`8b2a5b0d…`, `9539f3b2…`; `journalctl -k` (current boot); live `systemctl --user` scope properties;
live bead records bf-4x12ec, bf-173o7e.
