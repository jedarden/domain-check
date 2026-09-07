# Root Cause Analysis: bf-65lsdu "signal -1" Crashes (evidence-based)

**Analysis Date:** 2026-09-02
**Investigation Bead:** domchk-f853408b
**Crash Bead:** bf-65lsdu ("Run repository cleanup to eliminate 17GB bloat")
**Crash Window:** 2026-08-13T21:22:35Z – 2026-08-13T23:59:58Z
**Supersedes parts of:** `docs/research/root-cause-analysis-bf-65lsdu-crash-2026-08-13.md`
(authored by domchk-2ab71440 — right conclusion on repo bloat, wrong/unsupported
crash mechanics; corrected below)

---

## Executive Summary

bf-65lsdu was **not** an isolated crash. It was the last of six beads that were
crash-stormed across the whole of 2026-08-13 in the domain-check workspace:
**344 of 395 agent runs (87%) on the `lab-domain-check` worker died with
`exit_code: -1` that day**, including **127 consecutive attempts on bf-65lsdu
between 21:22 and 23:59** (one every ~60–95 s, mean lifetime 52 s).

The root cause chain:

1. **Ultimate cause:** `.beads/` JSONL files (~237 MB each) were repeatedly
   committed to git, bloating the repository to ~18 GB with 17.20 GiB of loose
   objects (identified in bf-1ziy13; recorded in bf-5j6k2i's description).
2. **Proximate cause:** agent dispatch scopes were killed by out-of-memory
   management — systemd-oomd (scope kill at >80 % memory pressure for >20 s)
   and kernel memcg OOM (git killed at ~12.3 GB anon RSS inside a needle
   dispatch scope, directly observed in the journal on 2026-08-16, the same
   standing condition two days later).
3. **Why bf-65lsdu specifically:** its task — `git gc --aggressive` over
   17.2 GiB of loose objects — is the single most memory-hungry operation
   possible on that repository. Every dispatch spiked memory into a box that
   was already saturated (worker-measured load average 12.7–39.1 against a
   9-core budget), hit the pressure/OOM threshold within ~30–60 s, and took the
   whole agent scope down.

**"signal -1" is not a signal.** It is how the NEEDLE harness reports an agent
process that died without exiting (killed by an unhandled/unidentified signal).
Timeout kills are separately coded `124`. See §3.

**Classification: Infrastructure event (memory exhaustion), workspace-local
trigger, box-wide amplifier. NOT a domain-check code defect.**

---

## 1. What the logs actually contain

### 1.1 Per-worker crash census, 2026-08-13 (`~/.needle/logs/claude-code-glm-4.7-lab-*-2026-08-13.jsonl`)

| Worker | Repo health | Runs | exit −1 | exit 0 | exit 124 (timeout) | exit 1 |
|---|---|---|---|---|---|---|
| **lab-domain-check** | **~18 GB, 17.2 GiB loose** | 395 | **344 (87 %)** | 18 | 22 | 11 |
| lab-drawrace | 28 MB | 155 | 2 | 63 | 80 | 10 |
| lab-test-fix | small | 185 | 2 | 93 | 61 | 29 |
| lab-s1 | small | 169 | 1 | 73 | 76 | 19 |
| lab-roam-1 | varies | 232 | 2 | 155 | 51 | 24 |
| lab-roam-2 | varies | 210 | 2 | 124 | 66 | 18 |

**Control group:** on the same box, the same day, under the same CPU
saturation (`fleet.cpu_saturated` fired 427–766 times per worker; load 12.7 at
minimum), every other worker lost ≤2 of ~200 runs to exit −1 (<1.5 %). Only
the worker operating the bloated repository lost the vast majority of its runs.
The crash trigger was **workspace-local** (repo bloat × memory-heavy git
operations), amplified by box-wide load — not a SIGHUP cascade, not a
system-wide event, and not a fleet-wide fault.

The day's −1 crashes on this worker were concentrated on the six
cleanup/investigation beads: bf-1s6c3 (22), bf-4k2ws (55), bf-1ea4g (56),
bf-2vtzg (9), bf-ncxbt (11), bf-2ildm (38), bf-mje3pd (7), bf-65lsdu (127)…

### 1.2 A complete crashed attempt (first bf-65lsdu attempt)

From `claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl`:

```
21:19:40  agent.completed  bf-5j6k2i  exit=0  (control success — ".beads/ gitignore" fix)
21:19:53  bead.claim.succeeded  bf-65lsdu
21:19:53  fleet.cpu_saturated   load_average=15.92  core_count=9
21:19:53  agent.dispatched      bf-65lsdu
21:22:23  agent.completed       bf-65lsdu  duration_ms=149195  exit_code=-1
21:22:23  outcome.classified    {"exit_code": -1, "outcome": "crash"}   (13 stream events only)
21:22:35  ALERT bead bf-ncs0ev created
21:22:39  bead.released  {"bead_id": "bf-65lsdu", "reason": "release_success"}
21:22:42  bead.claim.succeeded  bf-65lsdu            ← immediate re-dispatch
21:22:42  fleet.cpu_saturated   load_average=39.06   ← box now 4.3× oversubscribed
21:22:43  agent.dispatched      bf-65lsdu            ← loop repeats
```

bf-65lsdu attempt lifetimes: **min 30 s / mean 52 s / max 370 s**. A run that
lives 52 s never gets deep into `git gc --aggressive` packing — it dies during
agent startup/preflight/early git operations as memory pressure crosses the
oomd kill threshold (>80 % for >20 s). The first attempt lasted 149 s; later
attempts died faster as cumulative load climbed (load 39.06 by 21:22:42).

### 1.3 The alert-bead flood explained

The ~123 ALERT beads for bf-65lsdu (bf-ncs0ev, bf-2i5toy, … bf-2hgp86) are not
123 independent incidents. Each retry cycle created one alert bead
(`outcome.handled {"action": "alerted"}`) before releasing the bead, and the
dispatcher re-claimed bf-65lsdu immediately. The same loop produced the day's
other alert clusters (343 crash alerts across 13+ beads on 2026-08-13).

---

## 2. Physical evidence of the kill mechanism

The kernel journal does not reach back to 2026-08-13 (the box **rebooted
2026-08-15 19:26 EDT** — itself consistent with the era's instability; the
repository was not cleaned until 2026-08-17). The retained journal catches the
same standing condition two days later, with the exact kill signatures:

```
Aug 16 00:27:35 kernel: Memory cgroup out of memory: Killed process 3322486 (git)
  total-vm:13847248kB, anon-rss:12301364kB
  oom_memcg=.../app.slice/run-p3295453-i208789885.scope  ← a needle agent dispatch scope
Aug 16 00:28:37 kernel: Memory cgroup out of memory: Killed process 3331821 (git)
  anon-rss:12329416kB  (again, a run-p*.scope)
Aug 15 23:25:00 systemd-oomd[569]: Killed /user.slice/user-1001.slice/session-161.scope
  due to memory pressure for /user.slice being 90.60% > 80.00% for > 20s
Aug 15 23:43:26 systemd-oomd[569]: Killed .../agentscribe.service
  ... memory pressure ... 93.82% > 80.00%
```

`git` reaching ~12.3 GB anon RSS inside an agent's `run-p*.scope` — on a repo
holding 17.2 GiB of loose objects — is the kill mechanism in one line. When
git is memcg-OOM-killed or oomd deletes the scope, the agent process dies to a
signal it cannot handle; the harness records `exit_code: -1`.

---

## 3. What "signal -1" means (harness semantics, corrected)

- `-1` is **not a Unix signal number**. NEEDLE reports `exit_code: -1` when the
  agent process terminated **without an exit code** — i.e. killed by a signal
  (typically SIGKILL, which is uncatchable and leaves no in-process trace).
  Primary-source confirmation from the worker telemetry
  (`needle-claude-code-glm-4.7-lab-domain-check.stderr.log.pre-crash-2GB.bak`):
  `needle.agent.exit_code=-1`, and the supervisor wrapper prints:
  *"NEEDLE worker stopped unexpectedly … This indicates the worker was killed
  by an external process (e.g., SIGKILL, OOM, capacity governor)"*.
- Timeouts are a **different code**: `exit_code: 124` (22 occurrences on this
  worker that day). Exit −1 ≠ max-turns/timeout.
- The task description's parenthetical "(likely SIGABRT or similar)" is
  incorrect: SIGABRT would surface as a Go panic/abort with a stack trace in
  stderr. The preserved agent stderr for this era is empty of any such trace
  (see `.beads/traces/bf-65lsdu/stderr.txt` — only a hook warning). A silent,
  traceless death is the SIGKILL/OOM signature.

---

## 4. Why the prior RCA needed correction

`root-cause-analysis-bf-65lsdu-crash-2026-08-13.md` concluded "OOM during
`git gc --aggressive`" and treated the event as a bf-65lsdu-specific incident.
Evidence-based corrections:

| Prior claim | Corrected finding |
|---|---|
| Crash = OOM while executing `git gc --aggressive` | Attempts died at **mean 52 s**, too early for gc packing to be the direct killer; deaths occur when box memory pressure crosses the oomd threshold during startup/early git operations of a task whose *purpose* was a memory-heavy gc. Trigger and victim were the same bloat, but the kill is pressure-based scope termination, not gc runtime exhaustion per se. |
| Isolated incident on one bead | Tail of a **344-crash day** on this worker; 127 consecutive crashes on bf-65lsdu alone; 343 alert beads across 13+ beads that day. |
| "signal -1 = OOM killer termination" (implied SIGKILL equivalence) | Corrected semantics: −1 is the harness's "died without exiting" code; kill attribution rests on the kernel/oomd evidence in §2. |
| No control group | Same-day control: 5 other workers on healthy repos lost <1.5 % of runs; this worker lost 87 %. Workspace-local trigger confirmed. |
| "11 crash alert beads during retries" | Actually **~123** alert beads for bf-65lsdu; the count in earlier docs was under-enumerated. |

What the prior RCA got right, and stands: repo bloat as root cause,
infrastructure classification, no domain-check code defect, resolution by
staged cleanup.

---

## 5. Root cause chain (final)

```
.beads/ JSONL commits (~237 MB each, repeated)          ← ultimate cause
        │
        ▼
Repository bloat: ~18 GB total, 17.20 GiB loose objects (4,515)
        │
        ├──► every git operation in the workspace memory-hungry
        │
        ▼
bf-65lsdu task: git gc --aggressive over 17.2 GiB       ← maximal memory demand
        │
        ▼
Box already saturated (load 12.7–39.1 vs 9-core budget; 6 workers)
        │
        ▼
systemd-oomd kills agent run-p*.scope at >80% pressure >20s
kernel memcg-OOM kills git at ~12.3 GB anon RSS in dispatch scope
        │
        ▼
agent dies to uncatchable signal → needle exit_code −1 → "signal −1"
        │
        ▼
ALERT bead created + bead released → immediate re-dispatch → identical death
        │
        ▼
127 crashes on bf-65lsdu, 21:22–23:59 on 2026-08-13     ← the observed storm
```

---

## 6. Resolution and verification

| Step | Evidence |
|---|---|
| `.gitignore` now excludes `.beads/` (line 66) | Prevention bead bf-5j6k2i completed 2026-08-13T21:19:29 (exit 0), minutes before the storm began |
| Box rebooted | journal first entry 2026-08-15 19:26 EDT |
| Staged cleanup via 3 child beads | domchk-bdb1fedf / domchk-af4b5ef4 / domchk-87be56d8, completed 2026-08-16/17 |
| Repo health restored | 18 GB → 97 MB (99.5 %); loose 17.20 GiB → 5.23 MiB; verified again 2026-09-02: `count: 2` loose (20 KiB), `size-pack: 90.17 MiB` |
| Same task then succeeds | bf-65lsdu final run exit 0, 90 s (trace metadata `2026-08-17T00:34:00Z`) |

The transient-failure signature is complete: same task, same repo — dead 127×
while bloated, clean 90 s run once cleaned. No code change was ever needed.

---

## 7. Prevention already in place (verified 2026-09-02)

- `.beads/` in `.gitignore` — the accumulation vector is closed.
- `scripts/safe-git-gc.sh` (+ `--check-only`, `--resume`, memory caps) for any
  future deep-gc need; never bare `git gc --aggressive`.
- Repo-health thresholds and monitoring scripts (see `CLAUDE.md` §Repository
  Bloat Prevention).
- Crash-alert manager dedup/cooldown (2026-09-02) now collapses this
  retry-storm pattern into fewer alerts.

Residual gap worth noting: the dispatcher's release-and-retry loop has no
crash-storm breaker — it re-dispatched an identical doomed task 127 times in
2.5 hours, each iteration creating an alert bead and load. A circuit breaker
(N consecutive crashes on the same bead → back off / defer) would convert a
127-alert storm into one alert and one deferral.

---

## Sources

- `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` (12,131 telemetry events)
- `~/.needle/logs/claude-code-glm-4.7-lab-{drawrace,test-fix,s1,roam-1,roam-2}-2026-08-13.jsonl` (control group)
- `~/.needle/logs/needle-claude-code-glm-4.7-lab-domain-check.stderr.log.pre-crash-2GB.bak` (exit_code −1 semantics)
- `journalctl` (retained window: 2026-08-15 onward) — kernel memcg OOM + systemd-oomd kill lines
- `.beads/traces/bf-65lsdu/{metadata.json,stderr.txt}` — final successful run
- Bead store: bf-65lsdu, bf-5j6k2i, bf-1ziy13, domchk-b2003d19, alert beads bf-ncs0ev…bf-2hgp86
