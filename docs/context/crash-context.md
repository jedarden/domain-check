# Crash Context — Gathered 2026-09-02

**Bead:** domchk-15516836 ("Gather and review crash context")
**Collected:** 2026-09-02T14:39–14:55 UTC (10:39–10:55 EDT)
**Method:** Read-only collection from `.beads/events.jsonl`, `.beads/logs/*`, kernel and
user systemd journals, live `/proc` state, and monitor scripts. Nothing was mutated.
**Revised (domchk-24032f23, 2026-09-02T15:05Z):** added §4.2.1 — kill-moment anatomy of
the final crash (`bf-12gb0r`) from the journal and bead store. All other content is
unchanged from the domchk-15516836 collection.
**Re-verified (domchk-24032f23 retry, 2026-09-02T15:13Z):** snapshot below re-checked
against live state and holds. Still **247 crash events, none after 2026-08-26T22:54:48Z**
— zero new signal crashes. Sep 2 counters at 15:13Z: 731 completes, 213 exit-1 fails,
1 timeout → 29.1% fail rate (same elevated band as the 28.1% captured at 14:39Z).
System: 50 Gi mem available, load 8.11/7.93/7.88, disk 94 G free, `.git` 92 MB with
**5 loose objects** (down from 35 at collection — daily gc ran clean). No findings in
§6 (monitor false positives) required revision.

All timestamps are **UTC** unless suffixed EDT (local = EDT, UTC−4).

---

## 1. Headline findings

1. **No signal crashes (exit −1) have occurred in the last 7 days.** The most recent
   crash event in `.beads/events.jsonl` is `bf-12gb0r` at `2026-08-26T22:54:48Z`.
   The box is currently clean of agent signal-crashes.
2. **The "ELEVATED CRASH RATE: 247 crashes in 1hour" alert fired today
   (2026-09-02T02:11:23Z) is a false positive.** `scripts/crash-pattern-detection.sh`
   counts *all* crash events ever recorded (247, spanning Aug 16–26) and labels the
   total with the `--since` window. There is no time filter on the count (§6.1).
3. **The live issue is exit-1 task-failure rate, not signal crashes:** 184 exit-1
   failures on Sep 1 (44.2% of completions) and 204 through 14:39Z on Sep 2 (28.1%),
   spread evenly across all 12 workers and 142 distinct beads — fleet-wide, not a
   retry storm on a few beads (§4.3).
4. **Two monitoring bugs generate false infrastructure alarms:** memory-pressure
   CRITICALs from double-scaled PSI readings (§6.2) and inference-gateway
   "UNHEALTHY" from a self-signed certificate the monitor's curl rejects while the
   gateway actually serves HTTP 200 in 60 ms (§6.3).
5. **All OOM kills observed (Aug 16 storm and today) were memcg-constrained
   (`CONSTRAINT_MEMCG`) — contained kills inside cgroups, never system-wide.** On
   Aug 16 the kernel killed 257 `git` processes (largest at 12.3 GiB anon RSS) inside
   run-scoped cgroups; today's 6 kills are 63 MB `bash` processes inside
   `safe-git-gc-*.scope` units created by `test-safe-git-gc-limits.sh`
   (`SAFE_GC_MEMORY_MAX=64M`) — the intended kill-path test, not pressure (§5.2).

---

## 2. System state at collection time (2026-09-02T14:39Z)

| Metric | Value | Assessment |
|---|---|---|
| Memory | 62 Gi total, 12 Gi used, **49 Gi available**, 0 swap used | Healthy |
| Load avg (1/5/15 min) | 7.53 / 7.51 / 7.61 | Elevated (12-core box), below warning threshold 10 |
| Disk `/` | 94 G available | Healthy (warning threshold 30 G) |
| Uptime | 18 days (no reboot since ~Aug 15) | Stable |
| Repo size | `.git` = 92 MB; 1 pack 90.18 MiB; **35 loose objects** (300 KiB); 0 garbage | Healthy — all thresholds green (>1 GB / >500 MB loose is critical) |
| Gateway (inference) | HTTP 200 in 60 ms with `-k`; **self-signed cert** presented | Up, but see §6.3 |

Repo state artifact `.git-repository-state.txt` (from domchk-66ae89db) records an
older HEAD `0bb8a5a`; current HEAD is `db3f1f2` — the file is stale, not evidence of
divergence.

---

## 3. Crash event inventory (`.beads/events.jsonl`)

File: 13,645 events, 2.19 MB, entries begin 2026-08-16T04:21Z (no rotation files
present; earlier storms — the Aug 12–14 / 455-event wave investigated in bf-4yjq and
bf-4x12ec — predate this file and live only in committed docs).

| Event | Exit code | Count | Meaning |
|---|---|---|---|
| `claim` | — | 4,839 | worker claimed bead |
| `dispatch` | — | 4,432 | adapter dispatched agent |
| `complete` | 0 | 3,227 | task succeeded |
| `fail` | 1 | 678 | task failed (application/workflow error) |
| `fail` | 0 | 207 | task ended failure-outcome with exit 0 |
| `crash` | **−1** | **247** | agent killed by signal (SIGKILL/SIGHUP class) |
| `timeout` | 124 | 17 | task exceeded time limit |

Crash events span **2026-08-16T04:27:36Z → 2026-08-26T22:54:48Z**, then stop.
Events carry `bead`, `duration_ms`, `exit_code`, `outcome`, `ts`, `worker`, `strand` —
no stack traces are recorded in this file.

---

## 4. Timeline

### 4.1 Pre-crash / storm day — 2026-08-16 (245 of the 247 crashes)

Kernel journal (retention reaches Aug 15) shows the infrastructure context:

- **414 memcg OOM kills**, all `CONSTRAINT_MEMCG`, hourly from 00h through 13h,
  peaking at **78 kills in hour 12**. Victim processes: **257 × `git`** — the largest
  killed with `anon-rss: 12301364kB` (~12.3 GiB) inside scope
  `run-p3295453-i208789…`; `node (vitest)` also invoked the oom-killer at 00:36.
- These are runaway git operations (gc/repack class) being killed *inside their own
  scoped cgroups* — the exact mechanism `safe-git-gc.sh`'s `run_memory_capped` was
  later built to contain (its comment cites the bf-65lsdu 2026-08-13 crash).
- Dispatch pressure: **457 dispatches between 12:00–17:59Z** (peaking 122 in hour 16).

Agent crash distribution that day (by UTC hour): 04h 11, 05h 3, 06h 17, 07h 2,
10h 11, **12h 29, 13h 49, 14h 34, 15h 22, 16h 43, 17h 24** — crashes cluster in the
same afternoon window as peak dispatch concurrency and follow the morning's git-OOM
churn.

Retry-loop signature (same bead crashing repeatedly): bf-44x3a ×18, bf-1vuk2 ×18,
bf-9b8oe ×14, bf-3riuu ×14, bf-uoyie ×11, plus 18 more beads with ≥3 crashes.

Workers hit: lab-domain-check 154, lab-drawrace 41, lab-test-fix 32, lab-roam-1 20.
Despite the storm, 283 tasks still completed that day.

### 4.2 Decay and tail — Aug 17 → Aug 26

- 2026-08-17: 173 exit-1 failures (32.0% fail rate), 1 signal crash (bf-4833lh,
  686 s, lab-domain-check, 16:00Z).
- 2026-08-25/26: activity resumes (314/896 completes) at low fail rates (2.9% / 7.0%).
- **Last signal crash on record: `bf-12gb0r`, 2026-08-26T22:54:48Z**, duration 165 s,
  worker lab-drawrace, strand explore, exit −1.
- 2026-08-27/28: near-zero activity (52 / 2 completes), zero failures.

### 4.2.1 Kill-moment anatomy of the final crash (`bf-12gb0r`) — added domchk-24032f23

Journal (`journalctl` system + user manager `systemd[2877630]`) and bead-store state
around 2026-08-26T22:54Z:

| Time | Event |
|---|---|
| 22:50:54 | `needle-worker@lab-drawrace` instance (PID 3231977) exit-fails: launch deferred 4× (125 s), "CPU load saturated: 6.98 / 7 cores = 1.00 > threshold 0.80" |
| 22:51:24 | systemd restart, **counter at 279**; unit `Starting needle worker lab-drawrace...` |
| 22:52:03 / :04 | (events.jsonl) `lab-drawrace` claims `bf-12gb0r` (explore) and dispatches `claude-code-glm-4.7` |
| 22:52:54 | systemd logs `Started needle worker lab-drawrace` |
| 22:53:32 / 22:54:05 / 22:54:32 | launch deferrals across `lab-domain-check`, `lab-test-fix`, `lab-s1` — CPU 7.48–7.51 (1.07/core) — each service exit-fails |
| 22:54:39–:40 | SSH session 3687 (from Tailscale `100.72.170.64`) opens and is disconnected by user; `session-3687.scope` deactivates cleanly |
| **22:54:40.73** | **Agent closes bead `bf-12gb0r`** (bead-rs revision 7: "Investigation complete: Original crash report was incorrect. Bead bf-173o7e succeeded (exit code 1, max turns exceeded)… Repository is healthy.") |
| 22:54:43 | `cgov` collector errors (`Failed to load cursors: JSON error… line 4812`) — recurring noise |
| **22:54:48.79** | **Agent killed — recorded `crash`, `exit_code: −1`** (events.jsonl) |
| 22:55:00 | `needle-worker@lab-drawrace` (PID 3232778) exit-fails after 125 s of launch deferral (CPU 7.47) — supervisor churn, not agent-related |
| 22:56:36 → 23:14 | Same worker claims/completes other beads normally (`bf-3d9bqk` 22:59:21, `bf-1tqhm8` 23:02:27, `bf-5nfu3z` 23:06:51); no retry of
`bf-12gb0r` was needed — it stayed Closed |

Assessment:

- **Zero kernel OOM on 2026-08-26** (`journalctl -k` for the day has no oom lines) —
  the exit −1 here is a process-management signal kill, not memory exhaustion.
- **The work was finished before the kill**: bead close landed **8.06 s** before the
  kill timestamp and persisted (bead is Closed with full notes). Per
  `docs/crash-response-guide.md` ("< 30 seconds before crash → FALSE POSITIVE,
  post-completion cleanup"), this final crash is a **post-completion termination,
  not a work-losing crash**.
- Mechanism (hypothesis, unproven): teardown-time SIGHUP/SIGKILL amid the worker
  restart churn under CPU saturation; the SSH session churn in the same second is
  correlated but not attributable (the scope deactivated cleanly). The bead store
  commit winning the race is the operationally decisive fact either way.

### 4.3 Current condition — Sep 1 → Sep 2 (no signal crashes; exit-1 rate elevated)

Daily fail rate (exit-1 fails vs completes):

| Day | Completes | Exit-1 fails | Fail rate | Signal crashes |
|---|---|---|---|---|
| 2026-08-16 | 283 | 46 | 16.3% | 245 |
| 2026-08-17 | 541 | 173 | 32.0% | 1 |
| 2026-08-25 | 314 | 9 | 2.9% | 0 |
| 2026-08-26 | 896 | 63 | 7.0% | 1 |
| 2026-09-01 | 416 | 184 | **44.2%** | 0 |
| 2026-09-02 (→14:39Z) | 725 | 204 | **28.1%** | 0 |

Character of today's 204 exit-1 failures:

- Spread over **142 distinct beads** — 99 failed once, 27 twice, 13 three times, only
  3–4 beads four times (domchk-d574822c, domchk-be3cf290, domchk-a8b0e719).
  Unlike Aug 16, there is **no concentrated retry-loop signature**.
- Spread evenly across **all 12 workers** (lab-roam-1…11, lab-domain-check: 10–23
  each) — worker-local causes excluded; the condition is fleet-wide.
- Durations: median 653 s, p90 1,822 s, max 3,582 s; 8 events in the 1,800–2,100 s
  band (max-turns/time-limit shaped), 28 under 300 s (quick fails).
- Per the crash-response-guide classification, exit 1 = workflow/application failure
  (commonly `error_max_turns`), i.e. NEEDLE task-level failures — **not** the
  infrastructure/signal class.

Same-window system events (see §5–§6): CPU load warnings 10.3–12.1 at 10:15–11:20Z;
bogus memory-pressure CRITICALs (§6.2); 6 contained test-scope OOM kills 07:15–08:32Z
(§5.2); gateway false-UNHEALTHY reports (§6.3).

### 4.4 Post-crash state / alert-system activity

- `crash-alert-manager.log` (through 12:18Z): reprocessed the Aug 26 `bf-12gb0r`
  crash event, tripped its circuit breaker (3 consecutive, 1,800 s backoff), deferred
  instead of re-dispatching, then suppressed the repeat alert
  ("crash storm in progress") — the dedup/suppression path working as designed.
- `circuit-breaker.log` and `crash-resolution-tracker.log` entries from ~08:00–12:30Z
  are **test beads only** (bf-cap, bf-defer, bf-deferfail, bf-doomed, bf-ancient,
  bf-stale, bf-test-resolved-001, bf-test-persist-001) — the alert-infrastructure
  test suite was exercised this morning; not real incidents.
- `processed-alerts.txt` is 0 bytes (no alerts in the processed queue).

---

## 5. Error context: signals and OOM

### 5.1 Exit codes observed

| Code | Signal/class | Count | Window | Classification |
|---|---|---|---|---|
| −1 | SIGKILL/SIGHUP class | 247 | Aug 16–26 only | Infrastructure event |
| 1 | application/workflow error | 678 | ongoing; elevated Sep 1–2 | Workflow failure |
| 124 | timeout | 17 | 14 on Aug 16, 1 each Aug 17 and Sep 2 | Time limit |
| 137 | OOM killer (128+9) | **0** | — | none recorded |

No exit-137 events exist: even during the Aug 16 storm, agents were never killed by
the *global* OOM killer — the git processes that died were killed by their own
memcg limits (below), and agents died by signal (exit −1) instead.

### 5.2 OOM kills, both windows, are memcg-contained

- **Aug 16:** 414 kills, 100% `CONSTRAINT_MEMCG`; 257 victims named `git`, largest
  ~12.3 GiB anon RSS (total-vm 13.8 GiB) in `run-p3295453-i208789…`. Kill hourly
  histogram: 00h 12, 01h 11, 02h 39, 03h 3, 06h 17, 08h 44, 09h 71, 10h 61, 11h 34,
  12h 78, 13h 43 — none after 13h.
- **Sep 2:** 6 kills, 07:15:14 → 08:32:59Z, all `CONSTRAINT_MEMCG` inside
  `safe-git-gc-<pid>-1.scope`; every victim is `bash` at ~63 MB anon RSS. This
  matches `scripts/test-safe-git-gc-limits.sh`, which runs the gc under
  `SAFE_GC_MEMORY_MAX=64M`; `run_memory_capped` wraps pipeline commands (bash
  subshells) in a `systemd-run --scope -p MemoryMax=64M`, so a 63 MB subshell trips
  the ceiling — the intended, contained kill-path exercise. **Zero system-wide
  (global-constraint) OOM kills in either window.**

---

## 6. Instrumentation defects discovered (affect crash-context interpretation)

### 6.1 `scripts/crash-pattern-detection.sh` — no time filter on crash count

Line ~80: `RECENT_CRASHES=$(grep -i "\"event\":\"crash\"" "$EVENTS_FILE")` counts all
crash events in the file's history; only the separate `SURGE_CRASHES` check applies a
window. Consequence: the 02:11:23Z alert "ELEVATED CRASH RATE: 247 crashes in 1hour"
(and a verbose run reporting the same 247 as "last 24hours") reports Aug 16–26
history as if it were current. The temporal-clustering section ("Hour 13: 49
crashes") is likewise all-Aug-16 data. **The box has had 0 crashes in the last 7
days.**

### 6.2 `scripts/resource-monitor.sh` — PSI double-scaling → false CRITICAL

`check_memory_pressure` reads PSI `some avg60` — already a percentage (0–100) — and
multiplies it by 100 again (`awk -v p="$avg60" 'BEGIN { printf "%d", p * 100 }'`),
then compares against 70/80 thresholds. Result: today's CRITICAL alerts of
**191% (03:15Z), 1184% (11:10Z), 233% (11:15Z)** correspond to actual PSI avg60
readings of 1.91%, 11.84%, 2.33% — i.e. mild, transient memory stall, while
`memory_used_percent` in the same metric lines read 18–27% and 45–51 GB was free.
Every "Memory pressure critical" alert in `resource-alerts.log` is scaled ~100×.

### 6.3 `scripts/service-monitor.sh` — gateway declared UNHEALTHY over TLS, not health

The monitor checks `https://traefik-apexalgo-iad…:8444/health` with bare `curl -sf`;
the endpoint presents a **self-signed certificate**, so curl exits 60 (SSL cert
problem) and the monitor logs "UNEXPECTED STATUS (HTTP 000000)" → "UNHEALTHY after 3
attempts" → "PRE-FLIGHT CHECK FAILED." Verified live: the same URL with `-k` returns
**HTTP 200 in 60 ms**. The service is up; the check cannot authenticate it. Any
task gated on this pre-flight check is being blocked by a certificate-verification
failure, not an outage.

---

## 7. Log inventory (collected sources)

| File | Size | Last write | Content |
|---|---|---|---|
| `.beads/events.jsonl` | 219 KB | 14:53Z | 13,645 claim/dispatch/complete/fail/crash/timeout events since Aug 16 |
| `.beads/logs/crash-monitor.log` | 293 KB | 10:30 EDT | pattern-analysis output (unfiltered totals, §6.1) |
| `.beads/logs/service-monitor.log` | 375 KB | 10:36 EDT | gateway/resource pre-flight results (false UNHEALTHY, §6.3) |
| `.beads/logs/resource-metrics.log` | 62 KB | 10:35 EDT | 5-min metrics: mem 45–51 GB avail all day; disk 94–109 GB |
| `.beads/logs/resource-monitor.log` | 22 KB | 10:35 EDT | threshold checks |
| `.beads/logs/resource-alerts.log` | 1 KB | 07:20 EDT | 11 alerts — all PSI-bug CRITICALs or load warnings |
| `.beads/logs/circuit-breaker.log` | 19 KB | 08:22 EDT | breaker events — test beads + real bf-12gb0r |
| `.beads/logs/crash-alert-manager.log` | 24 KB | 08:18 EDT | alert processing/dedup for bf-12gb0r |
| `.beads/logs/alert-deduplication.log` | 3 KB | 05:34 EDT | "No duplicate alert patterns detected" |
| `.beads/logs/crash-pattern-alerts.log` | 65 B | Sep 1 22:11 EDT | the single false "247 crashes in 1hour" alert |
| `.beads/logs/crash-resolution-tracker.log` | 3 KB | 04:43 EDT | test-bead resolution tracking only |
| `.beads/logs/git-gc.log` | 2 KB | 07:55 EDT | today's standard gc on this repo: 2 g cap, clean stages |
| kernel journal (`journalctl -k`) | — | since Aug 15 | OOM events cited in §5.2 |
| user journal (`journalctl --user`) | — | — | scope-level OOM unit messages |

---

## 8. Assessment and pointers

- **Signal-crash posture: clean.** Zero exit −1 for 7 days; repo compact (92 MB);
  memory/disk healthy; cgroup-capped gc in place — the Aug 16 trigger class (runaway
  git in scoped cgroups) is contained by current tooling.
- **The final crash (`bf-12gb0r`) is a post-completion false positive, not a
  work-losing crash** — the agent closed its bead 8.06 s before the kill, and no
  kernel OOM exists that day (§4.2.1).
- **Open: exit-1 failure rate.** 44.2% (Sep 1) and 28.1% (Sep 2 so far) versus 2.9–7%
  on Aug 25–26, fleet-wide across workers and beads, mixed short and max-turn-shaped
  durations. The step change begins Sep 1; no signal crashes, OOM pressure, or
  repo-size growth accompanies it. Candidate context to check next: inference
  gateway TLS change (§6.3) and any NEEDLE dispatch-config change effective Sep 1.
- **Fix the three monitor defects in §6** before trusting any of their alerts;
  today's entire alert volume from crash-pattern, resource, and service monitors is
  false-positive output.
- Related committed investigations: bf-4yjq (50 verified crashes, Aug-12 storm),
  bf-4x12ec (retry storm addendum), bf-65lsdu (signal −1 / gc cgroup mechanism),
  `docs/crash-response-guide.md` (classification used above).
