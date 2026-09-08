# bf-4x12ec — System Resource State Around the Crash Window

Investigation bead: **domchk-40c9c99a** · Compiled: **2026-09-07**

Companion to `crash-logs/README.md` (bead domchk-48f3e34d), which established the
crash timestamp used here.

## Crash timestamp and window

| Field | Value | Source |
|-------|-------|--------|
| Recorded crash timestamp | **2026-08-14T10:23:11.219513632Z** | `crash-logs/README.md` — crash #1's `HANDLING_RELEASE_DONE` heartbeat, needle seq 1750; carried by alert bead `bf-fmg2cw` |
| Actual kill instant, crash #1 | 2026-08-14T10:23:02.958717335Z | `crash-logs/README.md` — `agent.completed` seq 1741 |
| Storm window (44 × exit −1) | 2026-08-14T10:21:06Z → 11:27:26Z | `crash-logs/exit-code-timeline.txt` |
| **Analysis window used here** | **09:53:11Z → 10:53:11Z** (±30 min around the recorded timestamp) | this doc |

The ±30 min window captures **21 of the 44 kills**. Storm-wide figures are given
alongside where they differ materially.

## Primary finding: the requested monitoring logs do not cover the crash

The three log files named in the task all exist today, and **all three begin
18+ days after the crash**. They contain zero 2026-08 records in any timestamp
format (`grep -c "2026-08"` = 0 in each; the only month prefix present anywhere
in `resource-metrics.log` is `2026-09`).

| Log | Earliest entry | Aug-14 coverage |
|-----|----------------|-----------------|
| `.beads/logs/resource-metrics.log` | `2026-09-01T22:49:42Z memory_available_gb=48` | **none** |
| `.beads/logs/resource-monitor.log` | `=== Resource Monitor: 2026-09-02T01:50:47Z ===` (line 2; line 1 is an `Unknown argument: --quiet` argv error from a first manual run) | **none** |
| `.beads/logs/repo-health.log` | `[2026-09-01T10:43:30-04:00] Repository Health Check` (36 lines total; setup-day manual runs only — this log is dormant, the 02:00 timer writes to `git-gc-check.log`) | **none** |

**Why:** the repo-health / resource monitoring layer was installed **2026-09-01**
— eighteen days *after* this crash. There was no `resource-metrics.log` to read
on 2026-08-14, so no timestamped memory/load/disk series from these files can
exist for the window. This matches the negative-findings table already recorded
in `crash-logs/README.md` for `crash-monitor.log`.

`resource-metrics.log`'s field set shows what a reading would have looked like
(`memory_available_gb`, `memory_used_percent`, `memory_pressure_percent`,
`disk_free_gb`, `disk_used_percent`) — none of it was being emitted on Aug 14.

## What contemporaneous telemetry does survive

Three independent sources carry real system-state readings from inside the
window. All are verbatim copies already in `crash-logs/`, re-read for this
analysis; the needle-log series was re-extracted from the source log
(`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl`,
mtime still 2026-08-14 19:50 — unchanged since extraction).

### 1. Memory and disk — the only in-window reading (attempt 15, session 9539f3b2)

`crash-logs/transcript-midstorm-9539f3b2.jsonl` captures an agent measuring the
box 8 s before it issued the fatal command, and 54 s before its own kill:

| Reading | Value |
|---------|-------|
| Timestamp | **2026-08-14T10:43:59.438Z** (`df -h / && free -h` tool_result) |
| Disk `/` | 444G total · 355G used · **67G free** · 85% used |
| Memory | 62Gi total · 16Gi used · 24Gi free · 22Gi buff/cache · **45Gi available** |
| Swap | 24Gi total · **0B used** · 24Gi free |

Bracketing, from the same transcript plus `crash-logs/exit-code-timeline.txt`
(attempt 15):

| Time (UTC) | Event |
|------------|-------|
| 10:43:38.687Z | attempt 15 dispatched |
| 10:43:59.438Z | **df/free captured — host memory-healthy, 67G disk free** |
| 10:44:07.643Z | `git gc --aggressive --prune=now` issued (`timeout: 600000`) |
| 10:44:53.202Z | **killed — exit −1, duration 74 285 ms** — 46 s into the gc |

Attempt 1 (crash #1) supplies the matching repository-size reading 100 s before
the first kill (`crash-logs/transcript-attempt1-crash-8b2a5b0d.jsonl`,
`git count-objects -vH` at **10:21:23.336Z**): `count: 4649 · size: 17.20 GiB ·
in-pack: 4081 · packs: 1 · size-pack: 9.60 MiB · garbage: 0`. Attempt 15 re-ran
the same command at 10:43:49.108Z and got a byte-identical answer — **no gc ever
completed**, so the repo sat at 17.20 GiB for the whole storm.

### 2. Load average — a real series through the window (needle `fleet.cpu_saturated`)

The Aug-14 worker log's only resource-telemetry event type is
`fleet.cpu_saturated`, emitting `{"core_count": 9, "load_average": <1m load>,
"threshold": 0.8}`. 383 events that day; 44 fall inside the storm window.

| Segment | n | min | max | mean | median |
|---------|---|-----|-----|------|--------|
| Pre-storm 09:53:11–10:21:06 | 5 | 8.77 | 22.51 | 15.18 | 13.59 |
| **Storm 10:21:06–11:27:26** | **44** | **10.37** | **30.92** | **14.47** | **13.54** |
| Post-storm 11:27:26–11:38:00 | 1 | 15.79 | 15.79 | 15.79 | 15.79 |
| ±30 min window (full) | 26 | 8.77 | 22.72 | 14.67 | 14.00 |

Merged timeline, crash #1 through attempt 15 (`L` = load sample, `K` =
bf-4x12ec kill):

```
10:21:06.973  L 13.13     10:33:26.227  L 12.22
10:23:02.958  K exit=-1 dur=115797ms   <<<< CRASH #1 KILL
10:23:16.805  L 12.30     10:34:40.898  K exit=-1 dur=74530ms
                          10:35:07.935  L 12.25
10:25:01.512  K exit=-1 dur=104481ms
10:25:41.461  L 15.84     10:36:05.465  K exit=-1 dur=57343ms
                          10:36:47.420  L 14.42
10:26:47.738  K exit=-1 dur=66085ms
10:27:21.404  L 11.73     10:37:54.324  K exit=-1 dur=66631ms
                          10:38:28.575  L 22.72
10:28:26.319  K exit=-1 dur=64755ms
10:28:48.375  L 10.49     10:39:27.018  K exit=-1 dur=57975ms
                          10:39:47.947  L 14.55
10:29:33.156  K exit=-1 dur=44613ms
10:29:56.544  L 10.56     10:40:53.801  K exit=-1 dur=65693ms
                          10:41:39.760  L 15.30
10:31:05.871  K exit=-1 dur=69152ms
10:31:29.797  L 15.22     10:42:58.570  K exit=-1 dur=78582ms
                          10:43:38.675  L 19.66
10:32:08.823  K exit=-1 dur=38882ms
10:32:27.431  L 13.26     10:44:53.202  K exit=-1 dur=74285ms  (attempt 15)
                          10:45:07.366  L 17.31
10:33:11.442  K exit=-1 dur=43834ms
                          10:46:16.579  K exit=-1 dur=69027ms
                          10:46:49.473  L 17.52
```

Inside the ±30 min window: 21 kills, all exit −1, durations 38 882–115 797 ms
(mean 65 208 ms).

### 3. Negative results from the surviving telemetry

- **No memory telemetry exists at all** for Aug 14. The worker log's full
  `event_type` inventory contains no memory/RSS/cgroup gauge; `fleet.cpu_saturated`
  is the only resource signal. `grep -ci` for `oom`, `memory`, and `sigkill`
  over the whole 10 138-line log returns **0, 0, 0** — needle recorded exit
  codes and heartbeats, never a memory reading.
- **Kernel journal unrecoverable** (as `crash-logs/README.md` established): the
  single surviving boot starts 2026-08-15 19:26 EDT, after the crash, so no
  `oom-kill` lines exist for the window.

## Trend summary

**Neither memory pressure nor disk pressure preceded the kill — at host level,
every reading argues against host exhaustion.**

- **Memory:** flat-to-healthy, not deteriorating. The only in-window gauge
  (10:43:59Z, mid-storm, 46 s before a kill) shows **45 Gi of 62 Gi available
  and 0 B of swap used** — the opposite of the profile a host approaches an OOM
  with. There is no series, but the single point sits mid-storm at attempt 15 of
  44, i.e. after 14 prior kills had already failed to move the host toward
  exhaustion.
- **Disk:** not a factor. **67 G free (85% used)** on a 444 G root — nowhere near
  full. The 17.20 GiB of loose objects was a *repository bloat* problem (disk
  consumption, and the thing the task was trying to fix), not disk exhaustion.
- **Load:** flat across the entire storm. Pre-storm mean 15.18 vs storm mean
  14.47 — **no ramp into the kills, and no correlation between kill instants and
  load spikes.** The day's highest 1-minute load, 55.98 at 08:23:02Z, occurred
  ~2 h *before* the storm and produced no such event. The storm's own max
  (30.92 at 11:21:25Z) came 18 min before the last kill and is unremarkable —
  a busy 9-core box, not a runaway. The kills are evenly spaced ~90–100 s apart
  with 39–116 s lifetimes, which is the signature of a **deterministic,
  per-attempt, scope-local kill**, not a system-wide resource collapse.

## Interpretation and limits

This evidence is **consistent with — and required by — the established
mechanism** (memcg-OOM SIGKILL of `git gc --aggressive --prune=now` inside the
12 GiB dispatch scope, per `crash-logs/README.md`): a scope-limited memory kill
is exactly what you expect when the *host* holds 45 Gi free, swap untouched, and
load steady, while each attempt's gc drives its *own cgroup* over its own limit.
Host-level health and a cgroup-local OOM are not in tension; the latter is only
invisible to host-level gauges.

Two limits should be stated plainly:

1. **The host-level readings cannot directly observe the dispatch scope's memory
   cgroup.** No per-cgroup telemetry existed on Aug 14 (no memory event type in
   the needle log; no cgroup sampler). So this analysis *corroroborates* the
   memcg-scope mechanism by excluding host exhaustion — it does not, and cannot,
   *prove* the cgroup watermark from measurements. `crash-logs/README.md` already
   records that the mechanism is regime-matched, not kernel-proven, for this bead.
2. **The load series is threshold-gated, not periodic.** `fleet.cpu_saturated`
  emits only when 1-minute load exceeds 0.8 × 9 cores = 7.2, so silence means
  "below 7.2", not "no activity". All 26 in-window samples are well above that
  floor, which is what makes the *flatness* conclusion sound: load was
  persistently ≥ ~8.8 for the whole window and did not trend. Load average also
  measures CPU run queue, not memory — its value here is that a host entering
  swap-death or allocator thrash would show escalating load, and this one does not.

## Source log paths

| Source | Path | Used for |
|--------|------|----------|
| Named in task — **no Aug-14 coverage** | `/home/coding/domain-check/.beads/logs/resource-monitor.log` | absence finding (earliest 2026-09-02T01:50:47Z) |
| Named in task — **no Aug-14 coverage** | `/home/coding/domain-check/.beads/logs/resource-metrics.log` | absence finding (earliest 2026-09-01T22:49:42Z) |
| Named in task — **no Aug-14 coverage** | `/home/coding/domain-check/.beads/logs/repo-health.log` | absence finding (earliest 2026-09-01T10:43:30-04:00) |
| Load-average series | `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl` → `fleet.cpu_saturated` (383 events); verbatim copy at `crash-logs/needle-worker-log-crash-window-full.jsonl` (lines 4411–6027 of source) | load trend |
| df / free / count-objects readings | `crash-logs/transcript-midstorm-9539f3b2.jsonl` (verbatim copy of `~/.claude/projects/-home-coding-domain-check/9539f3b2-eabe-432b-8d9f-7b5b0abc931d.jsonl`) | memory + disk point readings |
| Repo state at crash #1 | `crash-logs/transcript-attempt1-crash-8b2a5b0d.jsonl` (verbatim copy of `~/.claude/projects/-home-coding-domain-check/8b2a5b0d-0aae-4226-8ff8-e9d263e84045.jsonl`) | 17.20 GiB / 4 649 loose objects |
| Kill instants and durations | `crash-logs/exit-code-timeline.txt` (derived from `crash-logs/needle-worker-log-bf4x12ec-events.jsonl`) | kill/load correlation |

`~` = `/home/coding`. All times UTC.
