# Crash Artifacts Report — Bead bf-3561g (consolidated)

**Document:** Consolidated crash artifacts report for `bf-3561g`
**Consolidation bead:** domchk-ec8f9b06 (final compilation of the bf-3561g chain)
**Generated:** 2026-09-06 (supersedes the 2026-08-25 version of this file)
**Classification:** FALSE POSITIVE crash alert, itself crashed 9× in the 2026-08-16 memory-pressure cascade
**Confidence:** HIGH — every figure below re-derived from primary sources by the phase beads (§13)

This document consolidates the three-phase bf-3561g investigation chain:

| Phase | Bead | Deliverable | This doc |
|---|---|---|---|
| 1 — log catalog | domchk-de8dffac | [`docs/crash-logs-catalog-bf-3561g.md`](crash-logs-catalog-bf-3561g.md) | §7 |
| 2 — crash context | domchk-a39caced | [`docs/crash-context-bf-3561g/MANIFEST.md`](crash-context-bf-3561g/MANIFEST.md) | §3.2, §5, §6, §8 |
| 3 — scope + cascade | domchk-786e838f | [`docs/bead-bf-3561g-scope-and-original-task.md`](bead-bf-3561g-scope-and-original-task.md), [`docs/cascade-timeline-bf-3561g-2026-08-16.md`](cascade-timeline-bf-3561g-2026-08-16.md) | §2, §4 |
| 4 — compilation | domchk-ec8f9b06 | this file | — |

---

## 0. Corrections to the previous version of this document

The 2026-08-25 edition of this file (commits `512d55f`/`c627a18`/`d2eb46f`) predates
the phase investigations and carries claims the primary sources have since overturned.
They are corrected in place below; listed here so the diff is legible:

| Previous claim | Status | Correct reading |
|---|---|---|
| "Signal Pattern: Exit code −1 (**SIGHUP**)" — cascade killed by hangup signal | **wrong** | −1 is needle's outcome-classifier sentinel for an abnormal child death, **not a signal number**. The kernel record names the mechanism: memcg OOM (`CONSTRAINT_MEMCG`) (§6) |
| "Total Crashes: 201+ across all beads… 12:00–17:00 UTC (5 hours)" | **mislabelled** | **177** crashes in 12:00–17:00; **201** is 12:00:00 → 17:29:52 (the window extended to the end of bf-3561g's own chain); 245 for all of 2026-08-16 (§4.1) |
| "Creation Date: 2026-08-16" for bf-3561g | **wrong** | created **2026-08-13**T03:58:25Z — the same second as the (never-real) bf-4k2ws crash it alerts on; its crashes are 08-16 (§2) |
| `.beads/traces/bf-3561g/` metadata quoted as crash evidence (`duration_ms: 305382, exit_code: −1, crashed: true`) | **wrong** | the trace slot is **single-slot per bead** and holds the **2026-08-17T11:06:29Z success run** (`exit_code: 0, outcome: "success", duration_ms: 59043`) — the 11th dispatch, useless for the crash. Re-verified live 2026-09-06 (§7.1) |
| "Commit 549aa42 — bf-4k2ws work completion" | **wrong bead** | 549aa42 is *"chore: finalize needle predispatch SHA after crash recovery for **bf-5tgsk**"* (2026-08-16 16:35:54 UTC). bf-4k2ws's completion is recorded on the bead, exit 0 at 15:35:42Z (§2) |
| "System State at Crash Time: Memory 52 GB available (83% free)… Repository: 90 MB" | **not crash-time** | those are later box/repo readings. At the crash: user.slice memory pressure **94.29 %**, a *different* dispatch scope oomd-killed 5 s earlier, this scope at 100 % of its 12 GiB limit, load 15.76 on 7 cores, repository still in the **~18 GB / 17 GB-loose-objects** bloated condition (§5) |
| "killed by the SIGHUP cascade" (crash #4 framing) | **incomplete** | crash #4 was this run's **own** `git gc --aggressive --prune=now` growing to ~11.7 GiB anon inside the 12 GiB dispatch scope — kernel memcg OOM, not a passive signal victim (§8) |

Unaffected by all of the above and unchanged: the original **false-positive finding**
(bf-4k2ws never crashed; no work was lost anywhere in the chain).

---

## 1. Executive summary

`bf-3561g` — **"ALERT: Agent crash on bead bf-4k2ws"** — was a crash-alert bead
created 2026-08-13T03:58:25Z to investigate a purported exit −1 crash on `bf-4k2ws`
("Analyze divergent Forgejo and GitHub branch states"). The target bead had in fact
**completed successfully** (exit 0, 2026-08-16T15:35:42Z) and never crashed: the alert
was a false positive from the moment of creation.

Dispatched into the 2026-08-16 memory-pressure cascade, bf-3561g crashed **9 times**
(exit −1) between 17:13:04Z and 17:29:52Z, then **succeeded on its 10th dispatch** at
17:31:56Z. Its investigation work — splitting into 3 child investigation beads and
converting itself to an umbrella — was persisted to the bead database **before** the
crashes; nothing was lost.

Crash #4 (17:21:28Z, the target event of this artifact chain) is fully pinned: that
run's own agent invoked `git gc --aggressive --prune=now`, the gc reached ~11.7 GiB
anon RSS inside the 12 GiB per-dispatch cgroup `MemoryMax`, and the kernel memcg OOM
kill ended it. The needle classifier recorded the abnormal death as the `−1` sentinel
and generated the next-generation alert beads — the same alert-amplification loop that
produced 177 crashes across 4 workers in the 12:00–17:00 window.

**Key findings**

- ✅ Target bead bf-4k2ws completed successfully — no crash ever occurred
- ✅ bf-3561g's own work (bead split → umbrella) persisted before the crashes
- ❌ Infrastructure event: sustained system memory pressure → kernel memcg OOM kills
- ❌ False-positive alert chain: alerts about alerts, generated for a non-existent crash
- ✅ Mechanism since mitigated: bare-gc memory bounded by persistent git config (2026-09-02), repository de-bloated and verified (2026-09-01/06)

---

## 2. The original task: what bf-3561g was investigating (phase 3)

### 2.1 bf-3561g's mission

| Field | Value |
|---|---|
| Bead | bf-3561g — "ALERT: Agent crash on bead bf-4k2ws" |
| Created | **2026-08-13T03:58:25Z** |
| Type | crash-alert bead (not a work bead) |
| Target | bf-4k2ws — "Analyze divergent Forgejo and GitHub branch states" |
| Reported crash | exit −1, reported 2026-08-13 |
| Worker | claude-code-glm-4.7-lab-domain-check |
| Final state | **CLOSED** — succeeded 2026-08-16T17:31:56Z after 9 crashes |

Mission objectives: investigate the reported bf-4k2ws crash, analyze artifacts and
logs, determine root cause, document findings. bf-3561g executed this via a **bead
split**: it created 3 child investigation beads, converted itself to an umbrella, and
persisted the split to the database before the cascade reached it:

1. **domchk-ee8f5300** — crash investigation for bf-4k2ws
2. **domchk-e8c835b8** — crash investigation for bf-4k2ws
3. **domchk-ab71919d** — crash investigation for bf-4k2ws

Final output: *"SPLIT_COMPLETE: Created 3 children, parent converted to umbrella"* —
all 3 children block bf-3561g.

### 2.2 The target: bf-4k2ws never crashed

| Field | Value |
|---|---|
| Bead | bf-4k2ws — "Analyze divergent Forgejo and GitHub branch states" |
| Outcome | ✅ **completed successfully, exit 0, 2026-08-16T15:35:42Z** |
| Nature | read-only analysis task |

What bf-4k2ws actually did: analyzed Forgejo↔GitHub branch divergence, found both
remotes synchronized, documented local main as 418 commits ahead of both, verified the
safety of pushing, and wrote its deliverables:

1. `docs/divergence-analysis-bf-4k2ws-2026-08-13-pre-merge.md`
2. `docs/branch-divergence-bf-4k2ws-2026-08-13.md`
3. `docs/branch-divergence-analysis-bf-4k2ws-current.md`

(all three present in the repo, verified 2026-09-06). The reported "crash" did not
happen; the alert's 2026-08-13 timestamp even **predates** the target's 2026-08-16
successful completion — a timestamp inversion that alone marks the alert as false.

### 2.3 Why bf-3561g itself then crashed

bf-3561g sat open for three days and was dispatched into the 2026-08-16 cascade. Its
9 crashes are the cascade's tail (§4.5), and its alert children are nodes of the same
amplification loop (§4.4). It is simultaneously a **false-positive alert** and a
**crasher** — the defining shape of this event.

---

## 3. Crash timeline (with timestamps)

### 3.1 bf-3561g's full chain (`.beads/events.jsonl`, 36 events)

| # | Crash ts (UTC) | Duration | Outcome |
|---|---|---|---|
| 1 | 17:13:04.749 | 156,105 ms | exit −1 |
| 2 | 17:14:39.565 | 94,801 ms | exit −1 |
| 3 | 17:16:22.735 | 103,155 ms | exit −1 |
| **4** | **17:21:28.132817919** | **305,382 ms** | **exit −1 ← target event** |
| 5 | 17:21:28.144 → 17:23:14.381 | 106,227 ms | exit −1 |
| 6 | 17:23:14.389 → 17:24:42.528 | 88,132 ms | exit −1 |
| 7 | 17:24:42.565 → 17:25:31.542 | 48,953 ms | exit −1 |
| 8 | 17:25:31.550 → 17:27:14.745 | 103,188 ms | exit −1 |
| 9 | 17:27:14.753 → 17:29:52.577 | 157,817 ms | exit −1 (last crash of 2026-08-16) |
| 10 | — | 123,399 ms | **exit 0 — success** @ **17:31:56.062Z** |
| 11–12 | — | 191,807 / 59,235 ms | exit 0 @ 2026-08-17T11:05:30 / 11:06:29Z |

Each crash is followed within ~15 ms by `claim` + `dispatch` (auto-retry): **one
continuous retry chain on one worker** (`claude-code-glm-4.7-lab-domain-check`), not
nine independent failures. 17:29:52Z is also the **last crash of the entire day**
(§4.1 window note).

### 3.2 Kill-boundary reconstruction of crash #4

Three independent logs pin the target moment
`2026-08-16T17:21:28.126979482+00:00`. That string is **not a log record key** — it is
the needle alert clock read taken at the `HANDLING_RELEASE_DONE` step, +8.4 µs after
heartbeat `17:21:28.126971102Z`, and it survives only inside task specs (alert bead
domchk-90eb78b3). The genuine records around it (session event log lines 2946–2982,
`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-b7afe97d-2026-08-16.jsonl`):

```
17:16:22.746262671Z  rate_limit.allowed / fleet.cpu_saturated (load 15.76 / 7 cores)
17:16:22.746342133Z  worker.state_transition DISPATCHING → EXECUTING
17:16:22.749843447Z  agent.dispatched  prompt sha256:610bb529fca9…
17:16:22.759229642Z  transform.started
17:21:27.878875853Z  transform.completed   305,110 ms, 45 events written
17:21:27.977054297Z  agent.completed       exit_code −1          ← the kill
17:21:27.977986691Z  state EXECUTING → HANDLING
17:21:27.978025804Z  heartbeat HANDLING
17:21:27.978604550Z  outcome.classified    exit −1 → "crash"
17:21:27.979149806Z  heartbeat HANDLING
17:21:28.121995283Z  heartbeat HANDLING_FLUSH_DONE
17:21:28.122005309Z  heartbeat HANDLING_RELEASE
17:21:28.126971102Z  heartbeat HANDLING_RELEASE_DONE        ← target ts = +8.4 µs
17:21:28.132739Z     worker log: "crash alert bead"
17:21:28.132778276Z  bead.released  reason="release_success"
17:21:28.132817919Z  .beads/events.jsonl:1527  crash record (305,382 ms)
17:21:28.144255889Z  claim (auto-retry) → 17:21:28.148552975Z dispatch
17:21:28.167930643Z  transform.started — retry run begins
```

Exit context, all three sources agreeing:

| UTC | Source | Record |
|---|---|---|
| 17:21:27.977054 | session line 2953 | `agent.completed` `exit_code −1`, `duration_ms 305050` |
| 17:21:27.978613 | worker log line 2015 | `ERROR needle::outcome: agent crashed — releasing bead and creating alert bead_id=bf-3561g signal_code=-1` |
| 17:21:27.978604 | session line 2955 | `outcome.classified` `exit_code −1` → `crash` |
| 17:21:28.132739 | worker log line 2016 | `bead.outcome` INFO |
| 17:21:28.132817 | `.beads/events.jsonl` line 1527 | `event=crash` `exit_code −1` `duration_ms 305382` |
| 17:21:28.144219 | worker log line 2017 | auto-retry setup |
| 17:21:28.148855 | worker log line 2018 | `agent.dispatch` — next attempt begins |

(Duration differs by clock: 305,050 ms on the session log vs 305,382 ms on
events.jsonl — same run, two timestamps.)

The agent had **finished its work** — `transform.completed` fired 99 ms before the
kill — consistent with the chain-wide finding that no work was lost. The alert clock
read lands ~0.33 s after the agent was already dead.

---

## 4. Cascade patterns (phase 3)

### 4.1 Window numbers, 2026-08-16 12:00–17:00 UTC

Measured from `.beads/events.jsonl` (UTC) and system journald:

| Metric | Value |
|---|---|
| bead `crash` events in window | **177** |
| distinct beads crashing | **59** |
| distinct workers | **4** — lab-domain-check 98, lab-drawrace 34, lab-test-fix 30, lab-roam-1 15 |
| median run duration | 120.9 s |
| kernel memcg OOM kills, 12:00–17:05 UTC | **295** across **283 distinct dispatch scopes** |
| kernel OOM victims | 193 `git` (1.20 / **11.73** / 11.97 GiB anon; 123 at ≥11 GiB) + 101 `node (vitest 8)` (2.01 / 4.69 / 8.21 GiB — another workspace's JS tests, collateral) |
| crashes with a kernel OOM in the **same second** | 70 / 177 (40 %) |
| crashes with a kernel OOM **within ±2 s** | **176 / 177 (99 %)** |

> **Window-boundary note.** The frequently-quoted "**201 crashes**" is
> 12:00:00 → **17:29:52** — the window extended to the end of bf-3561g's own chain.
> 12:00–17:00 is **177**. All of 2026-08-16 totals **245**: 44 before 12:00, **zero
> after 17:29:52** — the end of bf-3561g's chain is the day's last crash.

### 4.2 Mechanism: kernel memcg OOM, not SIGHUP

`exit_code −1` / `signal_code=-1` is needle's sentinel for an abnormal child death.
The journald record for the same second as bead `bf-9b8oe`'s 12:25:24.622Z crash:

```
oom-kill:constraint=CONSTRAINT_MEMCG ... task=git,pid=485129,uid=1001
Memory cgroup out of memory: Killed process 485129 (git)
  total-vm:10496992kB, anon-rss:9161772kB ... oom_score_adj:200
```

Every pre-2026-09-06 "SIGHUP cascade" attribution for this window was wrong on
mechanism (§0). Victim memory profiles confirm the **12 GiB per-dispatch `MemoryMax`**
being hit: `git` victims pin the limit (median 11.73 GiB); `node` victims are other
repos on the same box losing workers to the same system-wide pressure.

### 4.3 Shape: sustained, not spiky

This is a **sustained low-level cascade across five hours**, not a single spike. Peak
minute is 3 crashes (13:53, 14:28); 32 different minutes have ≥2 distinct beads
crashing simultaneously; only six quiet gaps ≥5 min, longest 36 min (14:57 → 15:33) —
a lull, not an end. Full per-minute chart:
[`docs/cascade-timeline-bf-3561g-2026-08-16.md`](cascade-timeline-bf-3561g-2026-08-16.md) §3.

### 4.4 Dependency map: an alert-amplification loop

The decisive structural fact: **57 of the 59 distinct crashing beads were themselves
`ALERT:` beads**; only two were real work beads. Each crash makes needle auto-retry
*and* create a new crash-alert bead; under memory pressure those alert beads are
dispatched too, run git operations against the same bloated repository, and OOM as
well — creating more alert beads. **The alert system was the cascade's amplifier, not
just its reporter.**

```
                    bloated repo (18 GB, 17 GB loose objects)
                                   │
        ┌──────────────────────────┴───────────────────────────┐
        │  7 divergence/merge subject beads (bf-1s6c3, bf-4yjq,│
        │  bf-4k2ws, bf-1ea4g, bf-2xygo, bf-ncxbt, bf-574w1)   │
        └──────────────────────────┬───────────────────────────┘
               git op → ~11.7 GiB anon → memcg OOM (12 GiB MemoryMax)
                                   │
                        agent exit −1  (sentinel)
                                   │
                ┌──────────────────┴───────────────────┐
                │  needle: auto-retry  +  new ALERT bead │
                └──────────────────┬───────────────────┘
                                   │  57 ALERT beads dispatched
                                   ▼
                    they too run git on the same repo → OOM
                                   │
                                   └──►  more ALERT beads  (loop)
```

Root subjects and their alert fan-out (57 ALERT beads from 7 subjects):

| Root subject | Title | ALERT beads that crashed |
|---|---|---|
| **bf-1s6c3** | Create merge commit reconciling Forgejo and GitHub histories | **25** |
| **bf-4yjq** | Git origin remote points to GitHub directly; mirror diverged/stale | **12** |
| **bf-4k2ws** | Analyze divergent Forgejo and GitHub branch states | **7** (incl. bf-3561g) |
| **bf-1ea4g** | Document local main branch state | **6** |
| **bf-2xygo** | Fetch and analyze divergence between Forgejo and GitHub remotes | **3** |
| **bf-ncxbt** | Document remote GitHub mirror state | **3** |
| **bf-574w1** | Identify divergence and write analysis document | **1** |
| **total** | 7 subjects | **57** |

Work beads that crashed directly: `bf-1vuk2` (17 crashes — the single most-crashing
bead in the window) and `bf-31p3g`. All seven subjects plus both work beads are the
**same piece of work** — the Forgejo↔GitHub divergence analysis and merge
reconciliation — run against the bloated repository. One concentrated job, one
pathological repo, one memory ceiling: every dispatch OOMed regardless of which bead
ID carried it.

**There is no causal dependency between crashes through the bead graph.** The
"cascade" is **shared-infrastructure coupling**: every concurrent dispatch drew from
the same host memory pool against the same bloated repo, so each kill raised pressure
on the survivors — which is why the signal appears simultaneously on four unrelated
workers, and a bead's crash probability depended only on what *it* was doing.

### 4.5 bf-3561g inside the cascade tail (17:00–17:32 UTC)

bf-3561g is **absent from the 12:00–17:00 set** — its 9 crashes begin 17:13:04Z, at
the cascade's tail, interleaved with other workers still crashing:

| UTC | Bead | Worker | Duration | Note |
|---|---|---|---|---|
| 17:12:03.539 | bf-saupc | lab-roam-1 | 58 s | cascade still active |
| **17:13:04.749** | **bf-3561g** | lab-domain-check | 156 s | crash #1 |
| **17:14:39.565** | **bf-3561g** | lab-domain-check | 95 s | crash #2 |
| 17:14:58.062 | bf-w4fwe | lab-drawrace | 209 s | |
| 17:15:24.772 | bf-1fy2x | lab-roam-1 | 73 s | |
| **17:16:22.735** | **bf-3561g** | lab-domain-check | 103 s | crash #3 |
| 17:17:09.583 | bf-w4fwe | lab-drawrace | 130 s | |
| 17:18:00.339 | bf-1fy2x | lab-roam-1 | 154 s | |
| **17:21:28.132** | **bf-3561g** | lab-domain-check | **305 s** | **crash #4 — target event** |
| 17:21:31.699 | bf-6bio4g | lab-drawrace | 261 s | **3.5 s after #4**, different worker |
| **17:23:14.381** | **bf-3561g** | lab-domain-check | 106 s | crash #5 |
| **17:24:42.528** | **bf-3561g** | lab-domain-check | 88 s | crash #6 |
| **17:25:31.542** | **bf-3561g** | lab-domain-check | 49 s | crash #7 |
| **17:27:14.745** | **bf-3561g** | lab-domain-check | 103 s | crash #8 |
| **17:29:52.577** | **bf-3561g** | lab-domain-check | 158 s | crash #9 (last of the day) |
| **17:31:56.062** | **bf-3561g** | lab-domain-check | 123 s | **exit 0 — success** |

### 4.6 Condensed causal sequence

1. **Precondition** — repository bloated to 18 GB with 17 GB loose objects.
2. **Trigger** — seven related divergence/merge beads dispatched; every git op on that repo balloons toward ~11.7 GiB anon.
3. **Kill** — kernel memcg OOM inside the dispatch's 12 GiB `MemoryMax` (`oom_score_adj:200`, `CONSTRAINT_MEMCG`); 295 such kills in the window.
4. **Classification** — needle maps the abnormal death to the `−1` sentinel, `outcome: crash`.
5. **Amplification** — needle auto-retries **and** creates a new `ALERT:` bead.
6. **Re-dispatch** — alert beads run the same git work on the same repo and OOM too; 57 crash, fanned out from 7 subjects.
7. **Tail** — pressure eases after ~17:30; bf-3561g's 10th dispatch succeeds at 17:31:56Z.

---

## 5. System state at crash time

For crash #4 (17:21:28Z), from the phase-2 extracts (journald + transcript + worker log):

| Aspect | Value |
|---|---|
| user.slice memory pressure | **94.29 % > 80 % for >20 s**, with reclaim activity, `Pgscan: 1953981` |
| systemd-oomd | killed a **different** dispatch scope (`run-p2713992-i212402347.scope`, 9.7 G current usage) at **17:21:22Z — 5 s before** crash #4 |
| This run's scope | `run-p2695224-i212383579.scope` at **100 % of its 12 GiB limit**; kernel memcg OOM fired there |
| memcg counters | usage 12,582,912 kB = limit 12,582,912 kB (`failcnt 8948` charge failures before the kill) |
| Victim process | `git` pid 2718298, **anon-rss ≈ 11.7 GiB** — one process ≈ 98 % of the scope |
| Process tree | `bash(2695224) → claude(2695227, ~256 MiB) → bash(2718051) → git(2718283) → git(2718297) → git(2718298)` |
| CPU | `fleet.cpu_saturated {core_count: 7, load_average: 15.76}` at this run's dispatch (17:16:22Z); `rate_limit` WARN `load_1min=55.38 normalized=7.91` just after the crash |
| Repository | still in the **~18 GB / 17 GB-loose-objects** bloated condition that made every git operation balloon |
| Concurrency | **two dispatch scopes resident** — one reaped by oomd, one by the kernel, ~5 s apart |

Agent state inside the crashed run (transcript, session `d7cd18df`, 113 records;
needle dispatch 17:16:22.746Z → transcript first record 17:16:23.214Z, +0.47 s):

- `17:19:15.682Z` — `git config core.hooksPath` → `.githooks` (routine pre-flight); thinking: *"Good, now the git hooks path is configured. Let me verify it's set correctly."*
- `17:19:23.191Z` — **final action, tool_use Bash: `git gc --aggressive --prune=now`** (timeout 300000); last thinking block: *"Perfect! Now the git hooks are configured. Next, I need to run git gc to pack the loose objects. Given that the previous crash happened during git operations on this bloated repository, I should run git gc carefully. Let me run git gc with a moderate timeout first to see if it completes successfully."*
- No further transcript activity — the gc ran 124 s before the OOM kill; the agent died inside the tool call and never saw an error. Note the final reasoning: the agent was *aware* of both the repository bloat and the earlier git-operation crashes, yet still picked the unbounded `--aggressive --prune=now` variant — a 5-minute tool timeout bounds duration, not memory, so it changed nothing about the OOM.

For contrast, the figures in the previous edition of this document ("52 GB available,
83 % free; 90 MB repo") are **later** readings of the box and the repaired repository —
they are not crash-time state.

---

## 6. Signal −1 context (what "exit code −1" actually is)

- **It is a sentinel, not a signal number.** `exit_code: −1` / `signal_code: -1` in
  needle records is the outcome classifier's value for *an abnormal child death it
  cannot otherwise classify*. It does **not** mean SIGHUP (signal 1), and no hangup is
  involved anywhere in this event.
- **The real mechanism is in the kernel record.** For crash #4, the journald kernel
  extract (`docs/crash-context-bf-3561g/kernel-oom-kill-2026-08-16T172127Z.txt`, same
  second) shows `oom-kill:constraint=CONSTRAINT_MEMCG`, victim `git` at ~11.7 GiB anon
  against the 12 GiB cgroup limit, `oom_score_adj:200`. The only stack in scope is the
  kernel call trace (`dump_header → oom_kill_process → mem_cgroup_out_of_memory →
  try_charge_memcg → do_anonymous_page → exc_page_fault`) — there is **no userspace
  stack**: the agent process was not crashing, it was waiting on a child the kernel
  SIGKILLed, with no cleanup opportunity.
- **Why the sentinel exists:** the dispatch wrapper sees the child vanish to a fatal
  kernel action and records −1; it cannot see *why* the kernel acted. Matching crash
  investigations to kernel records (shifted −4 h for journald's local stamping) is the
  required step — this is the bf-4x12ec/bf-198ne lesson, and the same mechanism as the
  2026-08-14 gc storms.
- **Downstream effect of the sentinel:** needle classified the death as `crash`,
  released the bead, and created the next-generation alert bead — feeding the
  amplification loop (§4.4) — even though the agent had already finished its work
  99 ms earlier (§3.2).

---

## 7. Artifact and log catalog (phase 1 + phase 2 extracts)

### 7.1 Genuine crash-era artifacts

| File | Size | Notes |
|---|---|---|
| `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-b7afe97d-2026-08-16.jsonl` | 810,583 B (3,145 records) | **Primary.** Session event stream, 13:06:22Z → 17:33:41Z. Kill boundary at lines 2952–2962 (§3.2); includes `agent.completed {exit_code: −1}` and the full heartbeat ladder |
| `~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log` | 90,732,929 B | Worker tracing log (`Z`-format). Crash window at **lines 2013–2018**: `bead.outcome outcome="crash"` → alert creation → auto-retry, plus the `rate_limit` WARN (`load_1min=55.38`). Still the live rotation slot — those lines migrate to `.log.1` on the next 128 MiB rotation (recoverable, not lost) |
| `.beads/events.jsonl` | 2,325,437 B | Append-only fleet event log (never rotated; canonical crash inventory). bf-3561g crash at **line 1527**, claim/dispatch 1528–1529, cascade neighbours 1526/1530 |
| `.beads/checkpoint/forensic.jsonl` | — | 166 records referencing bf-3561g: state history, close reasons, prior investigation notes |
| journald (system) | — | Holds the 2026-08-16 kernel memcg-OOM and oomd records; extracted byte-faithfully into the two committed `.txt` files below |

**⚠️ Not the crash:** `.beads/traces/bf-3561g/` (metadata.json 396 B · stderr.txt 457 B ·
stdout.txt 763,196 B · trace.jsonl 10,534 B). Trace slots are **single-slot per bead
id**, overwritten every dispatch. This one was captured **2026-08-17T11:06:29Z** with
`exit_code: 0, outcome: "success", duration_ms: 59043` — the last of the chain's
dispatches (final row of §3.1). Re-verified live 2026-09-06. Useful only as a "what this bead's runs looked like"
reference; any earlier document quoting it as crash evidence is wrong (§0).
`stderr.txt` shows only a non-fatal SessionEnd-hook warning.

### 7.2 Phase-2 crash-context extracts (`docs/crash-context-bf-3561g/`)

Byte-faithful copies of the named source ranges, checksummed for the analysis phase
(downstream: domchk-3c95693a). Per repo policy (born of the bf-4yjq 17 GB-`.jsonl`
incident) the four log-shaped extracts are **deliberately not committed** — they live
in the worktree, byte-exact and re-derivable via Source+Range; the two journald
extracts **are** committed (`.txt` is not ignored; journald is the source most likely
to purge).

| File | Source | Range | Center |
|---|---|---|---|
| `session-event-log-lines-2903-3003.jsonl` *(untracked)* | session file above | lines 2903–3003 (25.7 KB) | line 2953 `agent.completed` @ 17:21:27.977Z |
| `needle-worker-log-lines-1966-2066.log` *(untracked)* | worker `.log` above | lines 1966–2066 (42 KB) | lines 2013–2018 crash window |
| `beads-events-lines-1477-1577.jsonl` *(untracked)* | `.beads/events.jsonl` | lines 1477–1577 (16 KB) | line 1527 crash record |
| `kernel-oom-kill-2026-08-16T172127Z.txt` ✅ committed | `journalctl -k` | 13:21:26–13:21:29 local | the memcg-OOM dump for the kill in the crash second |
| `systemd-oomd-2026-08-16T172122Z.txt` ✅ committed | `journalctl -t systemd-oomd` | 13:21:20–13:21:25 local | the oomd kill 5 s before — system-pressure context |
| `agent-transcript-d7cd18df-head-tail.jsonl` *(untracked)* | `~/.claude/projects/-home-coding-domain-check/d7cd18df-1799-4bce-b915-420e2e2dae73.jsonl` | first 2 + last 9 of 113 records | the crashed run's own transcript |

MD5 (2026-09-06, each verified identical to a fresh `sed -n '<range>p'` of its source):

```
261ac83437283bd91d42c3f07451dc98  agent-transcript-d7cd18df-head-tail.jsonl
8048e84327f4d7bc6ae9cbd7b1116adf  beads-events-lines-1477-1577.jsonl
49ae1f995b6f09f6d9788a8831b5637d  kernel-oom-kill-2026-08-16T172127Z.txt
6f1085f982f5d57e505d3eebfc06d9c4  needle-worker-log-lines-1966-2066.log
758f09b7a61d513630de336c34df1954  session-event-log-lines-2903-3003.jsonl
16476efd35f628ffc3284542800c3910  systemd-oomd-2026-08-16T172122Z.txt
```

### 7.3 The target timestamp and echo artifacts

The target timestamp `2026-08-16T17:21:28.126979482+00:00` appears in **no genuine
crash-era log** — it is the handler clock read at `HANDLING_RELEASE_DONE` (+8.4 µs
after the heartbeat, §3.2) and survives on disk **only as task-spec text** echoed into
agent transcripts:

| Echo location | Count | Nature |
|---|---|---|
| `.beads/traces/*/` | 72 files / 61 dispatch dirs | tool results in other agents' transcripts quoting this bead's/sibling's description |
| `~/.claude/projects/-home-coding-domain-check/**` | dozens | same echo mechanism from prior dispatches |
| the phase-1 dispatch's own agent log | 1 | carries the string because its task spec embeds it |

A home-wide grep for the nanosecond suffix `126979482` (excluding `target/`,
`node_modules`, `.git`) surfaces nothing outside these echo classes.

### 7.4 Negative results (checked and absent)

| Source | Result |
|---|---|
| journald coverage of Aug 16 | **was absent at phase-1 time** ("first entry 2026-08-17 15:33:14 EDT"); since recovered — system journald does hold Aug-16 kernel records (they back §5/§6). Caveat: system journald begins 2026-08-15 19:46 EDT, so Aug-16 kills are recoverable but nothing earlier is |
| `~/.needle/logs/archive/` | 2 files, none from 2026-08-16 |
| worker log slots `.log.2` / `-2.log` | 0 hits for the crash moment |
| `/var/log` | no syslog/kern/messages (NixOS, journald only) |
| sibling agent logs (`5af59baa` Aug-17, roam-1) | mention bf-3561g, not the 17:21:28 moment |
| `docs/` | nothing carried the exact target string before the phase-1 catalog |

### 7.5 Retention and naming semantics (for future investigations)

- `.beads/traces/<id>/` — **single slot per id**, overwritten each dispatch; a surviving trace proves only *its own* run (bf-3561g is the canonical trap, §7.1)
- `~/.needle/logs/needle-<agent>_<worker>.log[.N]` — worker tracing log, size-rotated; historical lines migrate to `.log.1`, `.log.2`, …
- `~/.needle/logs/<agent>-<worker>-<session8>-<YYYY-MM-DD>.jsonl` — per-session event stream; **the kill boundary lives here**; alert timestamps derive from the `HANDLING_RELEASE_DONE` heartbeat (µs-level, so expect sub-millisecond — not exact — agreement)
- Alert-format timestamps are handler clock reads, **not log keys** — match the surrounding event ladder (±50 ms), never the literal string

---

## 8. Root cause

### Crash #4 specifically (the target event)

This run's own agent invoked `git gc --aggressive --prune=now` at 17:19:23Z. The gc
grew to ~11.7 GiB anon inside the 12 GiB dispatch scope, and the kernel memcg OOM
killed it (§3.2, §5, §6). The run was **not a passive cascade victim** — the oomd kill
5 s earlier, of a *different* scope, is the cascade's system-pressure backdrop, but the
fatal blow was self-inflicted by an unbounded gc in a bounded cgroup.

### The class mechanism (all 9 crashes + the 177-crash window)

Per-dispatch cgroup memory limit (12 GiB `MemoryMax`) + repository bloat (18 GB,
17 GB loose objects) + system-wide pressure ⇒ every git-heavy dispatch could pin its
scope and get memcg-OOM-killed, and the alert system then amplified the kills by
dispatching more git-heavy alert beads against the same repo (§4.4).

### Mitigation status (so this cannot recur as-is)

- **2026-09-01** — repository de-bloated: ~18 GB → ~92 MB; `.beads/` wholly gitignored; re-verified 2026-09-06 (`docs/crashes/bf-4yjq-cleanup-verification.md`)
- **2026-09-02** — the bare-gc path bounded by persistent git config: `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` → worst case ≈3 GiB per pack run, covering **both** `git gc` and `git push` pack-objects (the bf-198ne push-side variant); verify with `./scripts/setup-git-gc-config.sh --verify`
- **Crash-alert system (2026-09-02)** — closed-bead filtering, duplicate detection, completion awareness, exit-code validation, cooldown, classification — implemented in response to exactly this false-positive/alert-loop family (`docs/crash-alert-fix-implementation-2026-09-02.md`)
- None of these bounds existed on 2026-08-16; the same mechanism today is caught by config, the 10 MB pre-commit gate, and the daily 02:00 repo-health timer.

---

## 9. Nested crash-alert pattern

```
Layer 1: bf-4k2ws (original task: branch divergence analysis)
   ↓ COMPLETED SUCCESSFULLY — exit 0, 2026-08-16T15:35:42Z — never crashed
   ↓ deliverables: 3 analysis documents (present in repo)

Layer 2: bf-3561g ("ALERT: Agent crash on bead bf-4k2ws")
   ↓ investigating a crash that never happened (false positive from creation)
   ↓ itself crashed 9× in the 2026-08-16 cascade (17:13:04–17:29:52Z)
   ↓ work (split into 3 children → umbrella) persisted before the crashes
   ↓ succeeded on 10th dispatch 17:31:56Z — CLOSED

Layer 3: domchk-* beads ("ALERT: Agent crash on bead bf-3561g")
   ↓ alert beads about the alert bead — the amplification loop's next generation
   ↓ every investigation reaches the same conclusion: bf-4k2ws never crashed;
   ↓   bf-3561g lost no work; the crashes were infrastructure (memcg OOM)
   ↓ near-duplicates continue to cycle; each re-derives the same finding
```

Pattern defects it exposes: alerts generated for beads that completed; alerts about
alerts (no deduplication at generation time); per-bead alerts during a system-wide
event (no surge suppression); timestamps that predate the supposed crash.

---

## 10. Impact assessment

| Item | Status | Impact |
|---|---|---|
| bf-4k2ws original work | ✅ complete (exit 0) | none — bead succeeded |
| bf-3561g investigation work (split → umbrella) | ✅ persisted pre-crash | none |
| Child beads created | ✅ 3 created | none |
| Documentation | ✅ preserved | none |
| Git history / bead database | ✅ intact / consistent | none |

**No work was lost anywhere in the chain.** The crashes destroyed running *processes*,
not work products: the only casualty of crash #4 beyond the process itself was the
unbounded `git gc --aggressive --prune=now` it was running — which, given the repo's
bloated state, would itself have been the next hazard had it succeeded (§8).

---

## 11. Acceptance criteria

### 11.1 This compilation bead (domchk-ec8f9b06)

- [x] **All previous phase findings integrated** — phase 1 (§7), phase 2 (§3.2, §5, §6, §7.2), phase 3 (§2, §4), plus §0 corrections
- [x] **Report written to `docs/crash-artifacts-bf-3561g.md`** — this file
- [x] **Crash timeline included with timestamps** — §3 (chain table + kill-boundary ladder), §4.5 (tail interleaving)
- [x] **Cascade patterns documented** — §4 (window numbers, mechanism, shape, amplification loop, condensed sequence)
- [x] **System state at crash time captured** — §5
- [x] **Signal −1 context explained** — §6
- [x] **Original bead task (bf-4k2ws investigation) summarized** — §2
- [x] **All acceptance criteria from parent bead addressed** — §11.2

### 11.2 Parent bead domchk-786e838f ("Investigate original bead bf-3561g task and cascade patterns")

| Parent AC | Where addressed |
|---|---|
| bf-3561g original task documented (investigating bf-4k2ws) | §2 |
| Cascade crash patterns identified | §4.2, §4.4, §4.6 |
| Crash timeline for 2026-08-16 12:00–17:00 created | §4.1, §4.3, §4.5 (full chart: cascade-timeline doc §3) |
| Related crashes cataloged with timestamps | §4.5 tail table; §3.1 chain table |
| Dependency relationships between crashes mapped | §4.4 fan-out table + loop diagram + no-causal-edge finding |

---

## 12. Conclusions

- **bf-3561g** was a false-positive crash alert about a bead that completed
  successfully; it nonetheless crashed 9 times as a cascade participant, lost no work,
  and succeeded on its 10th dispatch.
- **Crash #4** — the target event of this chain — is fully explained: the run's own
  unbounded `git gc --aggressive --prune=now` pinned the 12 GiB dispatch scope and was
  memcg-OOM-killed by the kernel; `exit −1` is needle's sentinel for that abnormal
  death, not a signal.
- **The cascade** was sustained system memory pressure (295 kernel memcg kills / 283
  scopes in 5 hours) amplified by an alert system that dispatched more git-heavy beads
  onto the same bloated repo — 57 of 59 crashing beads were ALERT beads.
- **Classification: infrastructure event, not a domain-check code defect.** Consistent
  with every investigation in this workspace: zero code defects found.
- **Remediation landed since** (repo de-bloat, gc/push memory bounds, crash-alert
  fixes) closes each leg of the loop this event ran through.

---

## 13. Sources and references

**Chain deliverables (integrated here):**
[`docs/crash-logs-catalog-bf-3561g.md`](crash-logs-catalog-bf-3561g.md) (phase 1) ·
[`docs/crash-context-bf-3561g/MANIFEST.md`](crash-context-bf-3561g/MANIFEST.md) (phase 2) ·
[`docs/bead-bf-3561g-scope-and-original-task.md`](bead-bf-3561g-scope-and-original-task.md) +
[`docs/cascade-timeline-bf-3561g-2026-08-16.md`](cascade-timeline-bf-3561g-2026-08-16.md) (phase 3)

**Underlying false-positive chain:**
`docs/crash-investigation-bf-4k2ws.md` ·
`docs/crash-investigation-bf-4k2ws-2026-09-01.md` ·
`docs/root-cause-analysis-bf-4k2ws-2026-09-02.md` ·
`docs/verification-report-bf-5l84o-duplicate-alert-resolved-crash-bf-4k2ws.md`

**Corpus-wide context:**
`docs/crash-logs-catalog.md` (247 crash events 2026-08-16 → 26) ·
`docs/comprehensive-crash-investigation-report-2026-09-01.md` ·
`docs/crash-response-guide.md` (exit −1 → infrastructure event) ·
`docs/crash-alert-fix-implementation-2026-09-02.md` ·
`docs/crashes/bf-198ne-crash-report.md` (push-side memcg variant) ·
`docs/maintenance/repository-maintenance-guide.md`

**Primary data (this box):** `.beads/events.jsonl` (line 1527) ·
`.beads/checkpoint/forensic.jsonl` ·
`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-b7afe97d-2026-08-16.jsonl` (lines 2952–2962) ·
`~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log` (lines 2013–2018) ·
system journald 2026-08-16 (kernel + oomd; local-stamped, UTC−4)

**Committed extracts:** `docs/crash-context-bf-3561g/kernel-oom-kill-2026-08-16T172127Z.txt` ·
`docs/crash-context-bf-3561g/systemd-oomd-2026-08-16T172122Z.txt`
