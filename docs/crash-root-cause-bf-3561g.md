# Root Cause Analysis — bf-3561g crash (exit −1)

**Document:** Analysis-phase root-cause report for the bf-3561g crash chain
**Analysis bead:** domchk-3c95693a (downstream of the phase-2 checksummed extracts, per
[`docs/crash-context-bf-3561g/MANIFEST.md`](crash-context-bf-3561g/MANIFEST.md))
**Date:** 2026-09-06
**Verdict:** kernel memcg OOM (SIGKILL of `git gc`) inside the 12 GiB per-dispatch cgroup —
**NOT** SIGHUP, **NOT** a domain-check code defect
**Confidence: HIGH** — every figure in this document was re-derived from primary sources by
this analysis pass (§5), independent of the phase beads' own derivations.

**Companion documents:**
[`docs/crash-artifacts-bf-3561g.md`](crash-artifacts-bf-3561g.md) (consolidated artifacts
report — full detail, corrections log) ·
[`docs/crash-logs-catalog-bf-3561g.md`](crash-logs-catalog-bf-3561g.md) (phase 1) ·
[`docs/crash-context-bf-3561g/MANIFEST.md`](crash-context-bf-3561g/MANIFEST.md) (phase 2) ·
[`docs/cascade-timeline-bf-3561g-2026-08-16.md`](cascade-timeline-bf-3561g-2026-08-16.md) (phase 3)

---

## 1. Root cause verdict

### 1.1 The target event (crash #4, 2026-08-16T17:21:28Z)

The crashed run's own agent invoked `git gc --aggressive --prune=now` at 17:19:23Z
(re-verified: it is the **last `tool_use` in that run's transcript**, timeout 300000 ms).
Against the then-bloated repository (~18 GB `.git`, ~17 GB loose objects) the gc's
pack-objects grew to ~11.7 GiB anonymous RSS inside the dispatch's 12 GiB cgroup
`MemoryMax`. The kernel killed it:

```
oom-kill:constraint=CONSTRAINT_MEMCG ... task=git,pid=2718298,uid=1001
Memory cgroup out of memory: Killed process 2718298 (git)
  total-vm:13161252kB, anon-rss:12301708kB (= 11.73 GiB) ... oom_score_adj:200
scope: run-p2695224-i212383579.scope — usage 12,582,912 kB = limit 12,582,912 kB (failcnt 8948)
```

(Committed extract: [`docs/crash-context-bf-3561g/kernel-oom-kill-2026-08-16T172127Z.txt`](crash-context-bf-3561g/kernel-oom-kill-2026-08-16T172127Z.txt);
journald stamps LOCAL, = UTC−4, so `13:21:27` local = `17:21:27` UTC — the crash second.)

The needle dispatch wrapper saw its child vanish to a kernel action it could not observe,
and recorded the abnormal death as the **`exit −1` sentinel**. The agent process itself was
healthy and waiting on the gc; it had *finished its investigation work* 99 ms earlier
(`transform.completed` at 17:21:27.878Z, then `agent.completed exit_code −1` at .977Z).

**Crash #4 was self-inflicted, not a passive cascade victim.** An oomd kill of a
*different* dispatch scope 5 s earlier (`run-p2713992-i212402347.scope`, 9.7 G, at
user.slice pressure 94.29 %) is the system-pressure backdrop, but the fatal blow was the
run's own unbounded gc in a bounded cgroup.

### 1.2 The class mechanism (all 9 bf-3561g crashes + the day's cascade)

> **12 GiB per-dispatch `MemoryMax`** + **repository bloat** (18 GB / 17 GB loose
> objects) + **system-wide memory pressure** ⇒ every git-heavy dispatch could pin its
> scope and get memcg-OOM-killed; needle then auto-retried **and** created a new
> `ALERT:` bead, which was itself dispatched onto the same repo — the alert system was
> the cascade's amplifier (§4).

### 1.3 Dispatch-premise corrections

The dispatch text for this analysis carried two premises the primary sources overturn:

| Dispatch premise | Correct finding |
|---|---|
| "signal −1 suggests SIGHUP" (task 4) | −1 is needle's outcome-classifier sentinel for an unclassifiable abnormal child death — **not a signal number**, and no hangup is involved. The kernel record names the mechanism: memcg OOM (§1.1). This matches the bf-4x12ec / bf-198ne lesson |
| "cascade crash pattern … 40+ crashes 12:00–17:00" (task 7) | **177** crashes across **59 distinct beads** in that window (re-derived, §5.2); the "40+" figure undercounts by ~4× |

So the answers to the dispatch's framing questions: task 4 — there was no SIGHUP to
handle; task 8 — the crash was **not** part of a system-wide SIGHUP event, it was part of
a system-wide **memcg-OOM** event.

---

## 2. Chronological crash sequence (reconstructed)

Three independent logs (session event stream, worker tracing log, `.beads/events.jsonl`)
agree to the millisecond. Condensed; full ladders in the companion report §3.

```
17:13:04.749Z  crash #1 (156 s)  ┐
17:14:39.565Z  crash #2 (95 s)   │ nine auto-retry crashes on ONE worker
17:16:22.735Z  crash #3 (103 s)  │ (claude-code-glm-4.7-lab-domain-check) —
17:16:22.746Z  │ dispatch #4: rate_limit.allowed fleet.cpu_saturated load 15.76/7 cores
17:19:15.682Z  │ agent: git config core.hooksPath → .githooks (routine pre-flight)
17:19:23.191Z  │ agent: LAST tool_use — git gc --aggressive --prune=now (timeout 300 s)
               │ … 124 s of gc runtime, no further transcript activity …
17:21:22.###Z  │ systemd-oomd kills a DIFFERENT scope (9.7 G) — user.slice pressure 94.29 %
17:21:27.878Z  │ transform.completed 305,110 ms — the run's work was already done
17:21:27.977Z  │ agent.completed exit_code −1  ← kernel memcg OOM killed git pid 2718298
17:21:27.978Z  │ outcome.classified −1 → "crash"; worker log ERROR "agent crashed"
17:21:28.126Z  │ heartbeat HANDLING_RELEASE_DONE  ← alert timestamp = this clock read +8.4 µs
17:21:28.132Z  │ bead.released; events.jsonl:1527 crash record (305,382 ms)
17:21:28.144Z  │ claim + dispatch — auto-retry, crash #5 begins
17:23:14 → 17:29:52  crashes #5–#9 (106/88/49/103/158 s)
17:29:52.577Z  crash #9 — the LAST crash of 2026-08-16 anywhere on the box
17:31:56.062Z  dispatch #10 — exit 0, SUCCESS (123 s). Pressure had eased; done.
```

Two independent clocks record the same run's duration differently (305,050 ms session log
vs 305,382 ms events.jsonl) — same run, two timestamp sources, not a discrepancy in events.

**No work was lost.** The split into three child investigation beads (domchk-ee8f5300,
domchk-e8c835b8, domchk-ab71919d; parent converted to umbrella) was persisted to the bead
store *before* the crash window. The target of bf-3561g's alert, bf-4k2ws, had itself
**completed successfully** (exit 0, 15:35:42Z the same day) and never crashed — the alert
was false from creation, its 2026-08-13 timestamp even predating the success it reported on.

---

## 3. Contributing factors

| # | Factor | Role |
|---|---|---|
| 1 | Repository bloat — 18 GB `.git`, 17 GB loose objects | Precondition. Made every significant git op balloon toward the scope limit |
| 2 | 12 GiB per-dispatch cgroup `MemoryMax` | The ceiling that turned "slow gc" into "SIGKILLed dispatch" |
| 3 | Unbounded `git gc --aggressive --prune=now` run bare, no `pack.windowMemory` | Trigger for crash #4 specifically. This exact path was unbounded on 2026-08-16 (bounded 2026-09-02, §6) |
| 4 | Sustained system memory pressure (user.slice 94.29 %; two dispatch scopes resident 5 s apart; load 15.76 on 7 cores) | Amplifier — kills anywhere raised pressure on every survivor |
| 5 | Needle auto-retry + per-crash ALERT-bead creation | Converted each kill into another git-heavy dispatch on the same repo (57 of 59 window-crashing beads were ALERT beads) |
| 6 | Single-slot trace per bead id | Did not cause the crash but corrupted its *evidence* — `.beads/traces/bf-3561g/` holds the 2026-08-17 success run (`exit_code: 0, duration_ms: 59043`), which earlier docs misquoted as crash evidence |

Not contributing: domain-check code (none of it was executing — the process died inside a
git subprocess), bead-store corruption (explicitly checked, §5.4), SIGHUP anywhere.

---

## 4. Cascade analysis (dispatch task 7)

2026-08-16, 12:00–17:00 UTC — re-derived from `.beads/events.jsonl` and system journald
(§5.2, §5.3):

- **177 crashes / 59 distinct beads / 4 workers** (lab-domain-check 98, lab-drawrace 34,
  lab-test-fix 30, lab-roam-1 15); median run 120.9 s; sustained, not spiky (peak minute 3
  crashes; the day totals 245 = 44 pre-12:00 + 177 window + 24 tail, **zero after
  17:29:52Z** — bf-3561g's own chain is the day's last crash).
- **Kernel side:** 295 memcg kills in 12:00–17:05 UTC across 283 dispatch scopes —
  193 `git` victims (median anon 11.73 GiB, i.e. pinned at the 12 GiB limit) + 101
  `node (vitest 8)` victims from *another workspace's* test suite: same pressure, collateral.
- **Coupling is shared infrastructure, not the bead graph:** there is no causal edge
  between the crashing beads. 57 of the 59 were `ALERT:` beads fanned out from just 7
  root subjects — all one piece of work (the Forgejo↔GitHub divergence analysis) run
  against the same bloated repo under the same ceiling. Each kill raised pressure on the
  survivors, which is why four unrelated workers show synchronized exits.
- **bf-3561g sits in the cascade's tail** (17:13:04–17:29:52Z): simultaneously a
  false-positive alert and a 9× crasher, succeeding on dispatch #10 once pressure eased —
  the defining shape of the event.

---

## 5. Independent verification performed by this analysis (2026-09-06)

What distinguishes this document from the phase deliverables: every load-bearing claim
below was **re-derived from primary sources in this pass**, not carried forward.

### 5.1 The crash moment (byte-exact)

| Claim | Re-derived from | Result |
|---|---|---|
| crash record: exit −1, 305,382 ms, 17:21:28.132817919Z | `.beads/events.jsonl` line 1527 | ✅ exact, incl. worker + neighbours 1526/1528/1529 |
| 12-row chain table (9 crashes, durations, success @ 17:31:56.062Z / 123,399 ms, 2 Aug-17 successes) | events.jsonl grep | ✅ all 12 rows match |
| kill-boundary ladder: transform.completed → agent.completed → outcome.classified → HANDLING_RELEASE_DONE → bead.released | session log `…b7afe97d-2026-08-16.jsonl` lines 2952–2962 | ✅ all timestamps to the ns; alert ts = heartbeat + **8.38 µs** |
| worker-log crash window ("agent crashed … signal_code=-1", release, auto-retry, next dispatch) | worker `.log` lines 2013–2018 | ✅ |
| last tool_use = `git gc --aggressive --prune=now` @ 17:19:23.191Z, 113 records | transcript `d7cd18df-….jsonl` | ✅ |
| trace-slot trap: metadata = success run (exit 0, 59,043 ms, 2026-08-17T11:06:29.750Z) | `.beads/traces/bf-3561g/metadata.json` | ✅ |

### 5.2 Cascade numbers (recomputed, not copied)

| Metric | Recomputed | Report §4.1 | Match |
|---|---|---|---|
| crashes 12:00–17:00 UTC | 177 | 177 | ✅ |
| distinct beads | 59 | 59 | ✅ |
| per-worker split | 98/34/30/15 | 98/34/30/15 | ✅ |
| median duration | 120.9 s | 120.9 s | ✅ |
| day partition | 44 + 177 + 24 = 245 | same | ✅ |
| 57/59 ALERT-titled; non-ALERT = bf-1vuk2 + bf-31p3g | via `bead list --json` title join | same | ✅ |
| crashes with kernel OOM within ±2 s | **176/177** | 176/177 | ✅ |

### 5.3 Kernel records (re-pulled from journald)

- 301 memcg kills in 12:00–17:10 UTC (196 `git` + 105 `node (vitest 8)`) — consistent with
  the report's 295 for its tighter 12:00–17:05 window (193 + 101); the git-victim count and
  the ±2 s correlation agree exactly.
- Crash #4's own kill: victim `git` pid 2718298, anon-rss 12,301,708 kB = **11.73 GiB**;
  process tree `bash → claude → bash → git → git → git` — byte-identical to the committed
  extract; scope at usage = limit, `failcnt 8948`.

### 5.4 Bead store integrity (dispatch task 6 — no prior phase had checked this)

`bead doctor` (read-only, 2026-09-06T16:08Z): **database integrity ✅, schema ✅ (3,420
issues), dependency graph ✅ (1,771 deps, no cycles), checkpoint valid ✅, no orphaned
temp files.** Zero corruption indicators attributable to the crash chain. The only
warnings are the fleet's known steady-state ones (118 stale non-leased in-progress beads;
advisory-only secret scan; 17 manually-blocked issues).

**Residual observation:** the three children bf-3561g created before crashing
(domchk-ee8f5300 / domchk-e8c835b8 / domchk-ab71919d) are still **Open** while their
parent is Closed. Their investigative purpose was fulfilled five times over by the phase
chain and this analysis; they are safe to release/close in a fleet-hygiene sweep. Not
acted on here — outside this dispatch's scope.

### 5.5 Mitigations live-verified today

| Mitigation | Check | Result |
|---|---|---|
| gc/push memory bounds (2026-09-02) | `./scripts/setup-git-gc-config.sh --verify` | ✅ effective windowMemory=2g, threads=1, deltaCache=1g → worst case ≈3 GiB per pack run |
| repository de-bloat (2026-09-01) | `du -sh .git`, `git count-objects -vH` | ✅ 93 MB `.git`, 45 loose objects / 296 KiB, pack 90.75 MiB, **0 garbage** |

---

## 6. Recommendations

**Landed (verified above) — the mechanism cannot recur as-is:**

1. Repository de-bloat + gitignore of `.beads/` (bf-4yjq fix) — holding at 93 MB.
2. Persistent git config bounding bare `git gc`/`git push` pack memory — covers the exact
   command that caused crash #4.
3. Crash-alert fixes (closed-bead filtering, dedup, completion awareness, cooldown, classification).
4. Layered detection: 10 MB pre-commit gate, daily 02:00 repo-health timer.

**Residual, in priority order:**

5. **Alert amplification is structural, not fixed.** The 2026-08-16 loop (kill → new ALERT
   bead → dispatch onto the same repo → kill) ran *before* the 2026-09-02 fixes; surge
   suppression at *generation* time (cap new alert beads per minute during system-wide
   events, or suspend alert-bead dispatch under memory pressure) remains the missing leg.
   The 2026-09-02 fixes filter duplicates downstream; they don't stop the fan-out itself.
6. **Ban bare `git gc --aggressive --prune=now` in agent task specs** — the config bounds
   it (≈3 GiB), but the 2026-08-16 run predates it and an agent invoking it today still
   burns the dispatch's entire memory budget for minutes. Prefer `./scripts/safe-git-gc.sh`.
7. **Trace-slot provenance:** any tooling or doc reading `.beads/traces/<id>/` as crash
   evidence must first check `metadata.json.captured_at` vs the incident window — the
   single-slot overwrite misdirected multiple early bf-3561g documents (§3, factor 6).
8. **Fleet hygiene sweep:** release/close the three stale Open children (§5.4); they block
   a Closed umbrella and re-trigger auto-split risk if they ever gain a `split-child` label.
9. **Timestamp hygiene:** alert-format timestamps are handler clock reads (here +8.4 µs
   after a heartbeat, landing 0.33 s after the agent was already dead) — never use them as
   log keys; match the surrounding event ladder (already documented in the companion
   report §7.5; restated here because it cost this chain a full phase to discover).

---

## 7. Sources

**Primary (re-read by this analysis):** `.beads/events.jsonl` (line 1527 ± 3) ·
`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-b7afe97d-2026-08-16.jsonl`
(lines 2952–2962) · `~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log`
(lines 2013–2018) · `~/.claude/projects/-home-coding-domain-check/d7cd18df-1799-4bce-b915-420e2e2dae73.jsonl`
(113 records) · `.beads/traces/bf-3561g/metadata.json` · system journald 2026-08-16
(kernel + systemd-oomd; LOCAL = UTC−4) · `bead doctor` · `bead list --json` ·
`./scripts/setup-git-gc-config.sh --verify` · `git count-objects -vH`

**Committed extracts:** [`docs/crash-context-bf-3561g/kernel-oom-kill-2026-08-16T172127Z.txt`](crash-context-bf-3561g/kernel-oom-kill-2026-08-16T172127Z.txt) ·
[`docs/crash-context-bf-3561g/systemd-oomd-2026-08-16T172122Z.txt`](crash-context-bf-3561g/systemd-oomd-2026-08-16T172122Z.txt)

**Chain:** [`docs/crash-artifacts-bf-3561g.md`](crash-artifacts-bf-3561g.md) (consolidated) ·
phase 1 [`docs/crash-logs-catalog-bf-3561g.md`](crash-logs-catalog-bf-3561g.md) ·
phase 2 [`docs/crash-context-bf-3561g/MANIFEST.md`](crash-context-bf-3561g/MANIFEST.md) ·
phase 3 [`docs/cascade-timeline-bf-3561g-2026-08-16.md`](cascade-timeline-bf-3561g-2026-08-16.md) ·
[`docs/bead-bf-3561g-scope-and-original-task.md`](bead-bf-3561g-scope-and-original-task.md)
