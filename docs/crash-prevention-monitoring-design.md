# Crash Prevention Monitoring System — Design

**Date:** 2026-09-06
**Bead:** domchk-b1068c3a
**Depends on:** domchk-ff1b585c (root cause analysis, closed), domchk-d7c086d6 (`crash-prevention-requirements.md`, committed 1b21053)
**Status:** Design submitted for review — approval outstanding (see §11)

---

## 1. Purpose

This document specifies the monitoring and alerting system for the three primary
crash categories identified in the root cause analysis, and integrates it with
the safeguards that already exist.

The three categories, from the canonical RCA
(`docs/investigations/root-cause-determination-domchk-6281555d-2026-09-06.md`)
and the requirements inventory:

| # | Category | Historical share of hard kills | Binding constraint |
|---|----------|-------------------------------|--------------------|
| C1 | **Memory exhaustion** — memcg OOM inside a bounded cgroup | 100% of surviving kernel kill records (`CONSTRAINT_MEMCG`) | the per-dispatch scope's `MemoryMax=12 GiB`, **not** host RAM |
| C2 | **Resource saturation** — CPU load / disk pressure | bf-xumcu: sustained 2.46×+ normalized load, 826 crashes/day | host-wide |
| C3 | **Service unavailability** — inference gateway 503/502 | ~8–12% of crashes; the dominant post-Aug-26 failure class (`exit_code=1` waves) | external dependency |

Everything here is about the **environment**, not the code: 157+
investigations have found zero domain-check defects, and this design adds no
application code paths.

### 1.1 Two design rules inherited from the RCA

These are constraints on the design, not preferences. Both come from
investigations that first reached the wrong conclusion.

**R-A — Alert at the altitude of the binding constraint.**
The only memory limit that has ever killed anything here is the *dispatch
scope's* memcg (`oom_memcg=…/run-p*.scope`, verified live: `MemoryMax=12 GiB`,
`MemoryHigh=infinity`). Host-wide memory alerting is **structurally blind** to
that kill and is carried only as advisory context (RCA §3.5, anti-recommendation
#1). Every memory *alert* in this design is computed from a cgroup reading;
host-wide readings are telemetry only.

**R-B — Fix the measurement before trusting the threshold.**
A threshold applied to a mis-scaled metric is worse than no metric: it either
fires constantly (training operators to ignore it) or never. Two live defects
of exactly this shape were found while writing this design (§7) and are
scheduled ahead of any new threshold work.

---

## 2. Architecture

### 2.1 Layered model

```
┌──────────────────────────────────────────────────────────────────────┐
│ L4 RESPONSE  · gate new work while an event is active (G-3)          │
│              · defer bead dispatch on failed preflight (G-4)         │
│              · checkpoint + resume long-running operations (§6)      │
└──────────▲───────────────────────────────────────────────────────────┘
           │ gate file + exit codes
┌──────────┴──────────────────────────────────────────────────────────┐
│ L3 CLASSIFY & SUPPRESS · crash-classifier.sh (4 classes)             │
│   (exists)                · alert-deduplication.sh + 5-min cooldown  │
│                          · closed-bead / post-completion filtering   │
│                          · verify-work-completion.sh pre-close gate  │
└──────────▲───────────────────────────────────────────────────────────┘
           │ alert records (JSONL, .beads/logs/*-alerts.log)
┌──────────┴──────────────────────────────────────────────────────────┐
│ L2 DETECT (thresholding)                                             │
│   · per-scope memcg headroom      → 70% warn / 85% refuse (C1)      │
│   · host PSI memory pressure      → advisory only          (C1)      │
│   · disk free                     → 30 GB warn / 20 GB crit (C2)    │
│   · CPU load (normalized)         → 10 warn / 15 crit      (C2)     │
│   · crash surge                   → 3-in-5m early / 10-in-10m conf  │
│                                     (C1+C2 composite)                │
│   · gateway health                → 2 consecutive failures (C3)     │
└──────────▲───────────────────────────────────────────────────────────┘
           │ normalized metric lines
┌──────────┴──────────────────────────────────────────────────────────┐
│ L0 MEASURE (no thresholds — one value per line, one rotation policy)│
│   · cgroup-memory-guard.sh  (per-scope, the C1 authority)           │
│   · /proc/pressure/memory, /proc/loadavg, df  (host context)        │
│   · .beads/events.jsonl crash records (surge input)                 │
│   · gateway HTTP probe (-skf, self-signed cert)                     │
└──────────────────────────────────────────────────────────────────────┘
        scheduled by 6 systemd user timers (verified live §4.7)
```

Layer separation is the load-bearing decision. L0 emits numbers; L2 owns every
threshold; L3 owns suppression; L4 owns action. A threshold may never appear in
a measurement script and an action may never appear in a detection script —
that is what allows a threshold to be re-tuned from data (G-5) without
re-auditing the collection path, and what keeps a broken probe from silently
disabling a response.

### 2.2 Data flow

1. **Timers** invoke the L2 scripts (`--once`) on their existing cadences.
2. Each L2 script reads L0 sources, applies thresholds, appends a
   **normalized metric line** to `.beads/logs/resource-metrics.log` and any
   breach to the matching `*-alerts.log`.
3. `crash-pattern-detection.sh` reads `.beads/events.jsonl` (the same source
   the needle-side crash records land in) for surge detection.
4. Breaches that represent crashes flow through L3 (classify → dedup →
   cooldown) before any investigation bead is created.
5. L4 gates are **pull-based**: any dispatcher, preflight, or long-running
   operation consults the gate files before starting work. Nothing in L2/L3
   pushes work anywhere.

### 2.3 Metric line format (new convention)

Today `resource-metrics.log` carries lines like
`2026-09-06T15:15:47Z memory_pressure_percent=0` — one metric per line, no
units, no source, no rotation. The design keeps the shape (grep-able, append-
only) and adds the three fields that investigations kept needing after the
fact:

```
<ISO-8601 UTC> <metric>=<value> <unit> <scope> <source>
2026-09-06T15:15:47Z memcg_used_percent=42 pct run-p1003531-i227467261.scope cgroup-memory-guard
2026-09-06T15:15:47Z psi_mem_some_avg60=0.0 pct host /proc/pressure/memory
2026-09-06T15:15:47Z disk_free_gb=41 gb host df -/
2026-09-06T15:15:47Z cpu_load_1min=0.66 ratio host /proc/loadavg
2026-09-06T15:15:47Z gateway_health=200 code external traefik-apexalgo-iad:8444
```

Rules: **UTC only** (journald/EDT timestamp mixing corrupted several past
investigations — G-8); value and unit never merged; every host-wide metric
carries `host` in the scope column so a cgroup-altitude query can exclude them
by construction.

---

## 3. Monitoring points

Each point states: what is measured, the threshold, the rationale, and the
**verified live 2026-09-06** implementation status.

### 3.1 C1 — Memory pressure

**Primary metric: per-scope memcg headroom** (`scripts/cgroup-memory-guard.sh`,
tracked, verified present).

Reads the caller's own cgroup ancestry and evaluates every bounded level
(`run-*.scope` at 12 GiB → `needle.slice` at 32 GiB), which is the only
altitude that has ever produced a kill here (R-A).

| Level | Threshold | Rationale |
|-------|-----------|-----------|
| warn | **70%** of the scope's `MemoryMax` | Matches the task-specified 70% early-warning level. Above ~70%, `pack.windowMemory=2g` worst case (≈3 GiB) plus a running agent can no longer be assumed to fit under 12 GiB; this is the point where starting a *second* memory-heavy operation in the same scope becomes the risky act. |
| refuse | **85%** of `MemoryMax` | The 80% figure in the task spec is retained as the *alert* point but the enforcement point is 85%: `git gc` at the 2026-08-16 storm peaked at 12,555,188 kB ≈ 98% of the bound, and a kill at 80% would land inside the window where a bounded operation is still legitimately finishing. 85% leaves ~1.8 GiB of margin — enough for a bounded pack run to complete, not enough for an unbounded one to hide in. |

Exit contract already implemented and kept: `0` pass, `1` warn, `2` refuse,
`3` unknown (fail-open; `--strict` promotes to refuse). Long-running operations
consume this via the L4 gate (§6.3).

**Secondary metric: host PSI memory pressure** — `some avg60` from
`/proc/pressure/memory`, thresholds 70% warn / 80% critical **retained but
demoted to advisory**. Rationale: PSI stalls indicate host-wide reclaim
pressure, which did not cause any of the surviving kills and cannot
distinguish them (R-A). It is kept because it *is* the right signal for the
host-level OOM class and costs nothing to collect — but no response action may
key off it alone.

**Gap:** `cgroup-memory-guard.sh` is not on any timer (verified: no
`domain-check-*` unit references it). It runs only when a preflight or
`run-isolated.sh` invokes it. A monitoring point that only fires when something
else remembers to call it is not monitoring. §8.1 schedules it.

### 3.2 C2 — Disk space

Measured: `df` on `/` (single 444G root disk shared by all repos).

| Level | Threshold | Rationale |
|-------|-----------|-----------|
| warn | **< 30 GB free** | Verified live in `resource-monitor.sh` (`DISK_WARNING_GB=30`) and matching CLAUDE.md's operating limit. Rust `target/` dirs grow to 80–100 GB each; 30 GB is the point where one mid-size build plus one concurrent gc still fits, and it precedes the ~20 GB pressure threshold at which CLAUDE.md mandates clearing an idle `target/`. |
| critical | **< 20 GB free** | Matches CLAUDE.md's pre-build gate. Below this, agc + a Rust rebuild contend for the same space; git gc needs transient headroom roughly equal to the pack it is writing. |

**Status: working** — no change required. These thresholds are already correct
and already alerting.

**Note:** the repository-size guards (`check-repo-health.sh`,
`auto-gc-trigger.sh`, 10 MB pre-commit hook) are the *cause-side* controls for
disk exhaustion via repo bloat and are out of scope here; this point covers the
host filesystem only.

### 3.3 C2 — CPU load

Measured: 1-minute load average from `/proc/loadavg`, **normalized by core
count** (12 cores on this box).

| Level | Threshold | Rationale |
|-------|-----------|-----------|
| warn | **> 10** raw load (~0.83× normalized) | Task-specified. Verified live in `resource-monitor.sh` (`CPU_WARNING=10`). This box has real headroom below 12, so 10 raw = "approaching saturation", and the 2026-09-02 alerts at 10.74/10.96/14.08 confirm it fires in the right band. |
| critical | **> 15** raw (~1.25× normalized) | Task-specified, verified live (`CPU_CRITICAL=15`). bf-xumcu's crash regime began at 2.46× normalized; 1.25× is the early band before that, chosen to warn while throttling is still possible rather than after saturation has already killed workers. |

**Gap (G-13, outside this repo):** warnings are informational; nothing
throttles or queues. The design's contribution is the normalized metric line
(§2.3) plus the L4 gate, which lets a dispatcher *decline* new work at critical
load. Actual load-based scheduling belongs to NEEDLE.

### 3.4 Crash surge detection

Measured: crash events in `.beads/events.jsonl`, windowed.

Two levels, because the task's 10-in-10-minutes and the implemented
3-in-5-minutes answer different questions:

| Level | Threshold | Meaning | Action |
|-------|-----------|---------|--------|
| early warning | **3 crashes in 5 min** (implemented, tightened from 10-in-10) | "something system-wide may be starting" — per P4, crashes cluster and are system-wide | write the surge gate file (§8.3); advisories only |
| confirmed event | **10 crashes in 10 min** (task-specified) | "this is an infrastructure event" — matches CLAUDE.md's alert table and the historical Aug-16 signature | gate active: defer non-essential dispatches; per-bead investigation suppressed in favour of one event record |

Rationale for keeping both: 3-in-5 is the *early* signal and is deliberately
sensitive (a false early warning costs one advisory line); 10-in-10 is the
*declared event* and is deliberately specific, because declaring an event
changes behaviour fleet-wide. Collapsing them into one threshold is what made
the original 10-in-10 rule late — it could not fire until the event was already
17 minutes old in the bf-3561g timeline.

**Status: detection logic exists and is more sensitive than the task
minimum — but two defects currently stand between it and a working signal**
(§7.1 syntax error, §7.3 no consumer). Fixing those is phase 0 of the
implementation plan (§8).

### 3.5 C3 — Service availability (inference gateway)

Measured: HTTPS GET `https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health`.

| Parameter | Value | Rationale |
|-----------|-------|-----------|
| probe | `curl -skf`, timeout 5 s, 3 retries, 2 s delay | **`-sk` is mandatory** — the gateway serves a self-signed cert, so plain `-sf` fails with curl exit 60 while the gateway answers `200 ok`. This is a documented false-alarm path that has manufactured "gateway down" conclusions; the RCA lists it as a doc-level bug by construction (§3.6). Verified live in `service-monitor.sh:46`. |
| degradation | **2 consecutive failed probes** on the 2-minute timer | A single failed probe on an interval this short is indistinguishable from a network blip; the post-Aug-26 service waves were sustained, so requiring a second confirmation costs at most 2 minutes of detection latency and removes the majority of single-probe noise. |
| critical | gateway down ≥ 6 minutes (3 consecutive) | Matches the observed service-wave duration class; at this point preflight failure is expected to *defer beads* (G-4) rather than let workers start and die. |

**Status: monitoring works** (2-minute timer verified firing).
**Gap (G-4):** no failover target and no enforcement point — `gateway-failover.sh`
does not exist, and nothing *requires* a preflight pass before dispatch. The
design adds the enforcement contract (§8.4); choosing a failover gateway is an
operator decision recorded as an open item (§11).

---

## 4. Existing infrastructure this design builds on (verified live 2026-09-06)

### 4.1 Scripts (tracked)

| Script | Role in this design |
|--------|---------------------|
| `scripts/resource-monitor.sh` | L2 for disk / CPU / host PSI. Thresholds verified: 30/20 GB, 10/15 load, 70/80% PSI. **Contains the PSI ×100 defect (§7.2).** |
| `scripts/cgroup-memory-guard.sh` | L0/L2 authority for per-scope memory (C1). Correct altitude, correct exit contract. **Not scheduled (§3.1 gap).** |
| `scripts/crash-pattern-detection.sh` | L2 surge detection. **Working tree copy is syntactically broken (§7.1); HEAD is clean.** |
| `scripts/service-monitor.sh` | L2 gateway health. Working. |
| `scripts/preflight-health-check.sh` | L4 pre-dispatch gate — already calls `cgroup-memory-guard.sh`. |
| `scripts/crash-alert-manager.sh` + `crash-classifier.sh` + `alert-deduplication.sh` | L3. 12/12 tests passing (re-verified 2026-09-06). |
| `scripts/verify-work-completion.sh` | L3 completion evidence for post-completion-death classification. |
| `scripts/safe-git-gc.sh` | The checkpoint/resume reference implementation (§6). |
| `scripts/auto-gc-trigger.sh`, `check-repo-health.sh` | Cause-side disk/repo controls, already on the 02:00 timer. |

Present but **untracked** (another worker's in-flight changes — this design
records them, does not assume them): `memory-watch.sh`, `git-gc-monitor.sh`,
`crash-circuit-breaker.sh`, `scripts/test-*.sh` companions. None is scheduled.
If they land, `memory-watch.sh` and `git-gc-monitor.sh` slot into L2 and the
circuit breaker into L3 without structural change.

### 4.2 Scheduling — 6 systemd user timers, all firing

| Timer | Cadence | Unit's ExecStart |
|-------|---------|------------------|
| `domain-check-monitoring` | 10 min | `crash-pattern-detection.sh` |
| `domain-check-resource-monitor` | 5 min | `resource-monitor.sh --once` |
| `domain-check-service-monitor` | 2 min | `service-monitor.sh --once` |
| `domain-check-repo-health` | daily 02:00 | `auto-gc-trigger.sh --dry-run` |
| `domain-check-git-gc` | daily 03:00 | `safe-git-gc.sh` (`MemoryMax=4G`) |
| `domain-check-git-gc-full` | Sun 04:00 | `safe-git-gc.sh --full` (`MemoryMax=4G`) |

All six had future trigger times when checked. Operating rule carried from
CLAUDE.md: after editing any unit file, `systemctl --user daemon-reload`, or
the timer silently never fires (this bit the weekly full-gc on 2026-09-02).

### 4.3 Alert and metric logs (`.beads/logs/`, gitignored)

`crash-monitor.log`, `resource-alerts.log`, `resource-metrics.log`,
`service-monitor.log`, `repo-health.log`, plus L3's
`crash-pattern-alerts.log` / `alert-deduplication.log` /
`crash-alert-manager.log` and `work-completion.log`. Actively written (latest
entries current at time of writing).

**Gap (G-8):** no rotation. `resource-metrics.log` at 449 KB and
`crash-monitor.log` at 2.4 MB grow without bound; the evidence-retention
requirement is ≥30 days for kernel/journald records, so the design specifies
retention explicitly rather than leaving it to logrotate's absence (§8.6).

---

## 5. Alert threshold summary

Single table, for review. "Altitude" is which constraint the threshold binds
(R-A); a host-altitude alert may never trigger a response action.

| # | Signal | Altitude | Warn | Critical | Refuse/gate | Rationale (abbrev.) |
|---|--------|----------|------|----------|-------------|---------------------|
| 1 | memcg used / `MemoryMax` | **per-scope** | 70% | 80% (advisory) | 85% | 70% = second heavy op becomes unsafe; 85% = bounded pack still completes, unbounded cannot hide |
| 2 | host PSI mem `some avg60` | host | 70% | 80% | — | advisory only; blind to the kill mechanism |
| 3 | disk free on `/` | host | < 30 GB | < 20 GB | pre-build gate at 20 GB | CLAUDE.md operating limits; target/-rebuild headroom |
| 4 | CPU load 1 min | host | > 10 | > 15 | gate at > 15 | task spec; 1.25× normalized is early band before the 2.46× crash regime |
| 5 | crash surge | workspace | 3 in 5 min | 10 in 10 min | gate while active | early = sensitive; declared = specific |
| 6 | gateway health | external | 1 failed probe | 2 consecutive | defer dispatch at 3 | `-skf` mandatory; 2-probe confirmation ≈2 min latency |
| 7 | gc pack memory (observe only) | per-scope | — | > 2 GiB RSS | — | RCA handoff: the bound is enforced but unobserved |
| 8 | crash event staleness | workspace | source older than 2× timer interval | — | — | detection is blind if the event source stops recording |

Rows 7 and 8 are new observability the RCA asked for explicitly
(handoff item 2) and that the current scripts lack; row 8 exists because a
monitoring system that cannot tell "no crashes" from "not looking" will report
health through an outage.

**Suppression rules (L3, already implemented — restated so the table is
complete):** closed-bead filtering, duplicate detection, 5-minute cooldown,
post-completion grace, exit-code validation. These carry P3: with 60–75% of
historical alerts false positives, suppression is not an optimization, it is
the difference between signal and noise.

---

## 6. Checkpoint / resume strategy for long-running operations

The operations that die here are long and memory-heavy (`git gc`, large
pushes, sustained test runs). A kill is uncatchable SIGKILL, so **nothing can
be recovered at death time** — resume must be designed in before the kill, as
state written to disk at stage boundaries.

### 6.1 The reference pattern (already proven)

`scripts/safe-git-gc.sh` is the model; it has survived production use:

- work decomposed into **ordered, individually-bounded stages**;
- a **JSON checkpoint** (`.git/safe-gc-checkpoint.json`: `stage`, `status`,
  `duration`, `repo_size`) written after every stage, on success *and* failure;
- `--resume` reads the checkpoint and continues from the last `complete` stage;
- each stage is **idempotent** — re-running a completed stage is safe, so an
  ambiguous checkpoint resolves by re-execution rather than guesswork.

### 6.2 Generalized protocol (new long-running operations follow this)

1. **Stage table defined up front** — ordered steps, each with its own memory
   bound and a idempotency statement. If a step cannot be made idempotent, it
   is wrapped so its *effect* is (e.g. write-to-temp + rename).
2. **Checkpoint file at a fixed, declared path** —
   `.beads/state/checkpoints/<operation>.json`, containing:
   `{operation, stage, status, started_at, updated_at, attempt, command,
   inputs_fingerprint, host, scope}`.
   `inputs_fingerprint` (hash of the inputs the stage consumed) is what makes
   resume safe: a checkpoint whose fingerprint no longer matches is discarded
   rather than resumed.
3. **Write checkpoint after every stage boundary, both outcomes.** A crash
   mid-stage resumes into that stage from its start; idempotency makes that
   correct.
4. **`--resume` flag on every long-running script**, with the same semantics as
   safe-gc: read → validate fingerprint → continue at first non-`complete`
   stage. No checkpoint ⇒ run from the beginning (never fail closed on a
   missing file).
5. **Checkpoint on a resource breach, not just on completion.** When the L2
   layer reports refuse (§5 row 1) or the L4 gate opens mid-run, the operation
   checkpoints and exits non-zero *before* the kernel makes the decision for
   it. This converts a future memcg kill into a resumable stop — the single
   highest-value change in this section, because it is the only one that acts
   *before* SIGKILL.
6. **Attempt counter in the checkpoint.** If the same stage fails N times
   (N=3) with flat duration — the "repetition without progress" signature from
   the RCA — the operation records `degenerate: true` and stops instead of
   letting an external retry loop re-enter a deterministic failure. This is
   the in-process counterpart to NEEDLE's auto-split.

### 6.3 Gate integration

Before each stage, a long-running operation calls:

```bash
scripts/cgroup-memory-guard.sh --check || {
  save_checkpoint "$OP" "$STAGE" "deferred: $(cat /tmp/memguard.reason)"
  exit 75   # EX_TEMPFAIL — resumable, distinguishable from failure
}
```

Exit 75 is the contract for "stopped by a gate, safe to resume" — L3 and any
retry logic treat it as *deferred*, not *crashed*, which prevents exactly the
false-positive class that dominates alert volume (P3).

**Explicit non-goals**, per repo rules: no checkpointing of Rust `target/`
(fully regenerable; backing it up doubles disk use), and no bead-checkpoint
flush inside a long-running git operation (flush belongs before commit, and
never before `git pull`).

---

## 7. Live defects found while writing this design

Found by running the existing monitoring during design verification on
2026-09-06. Recorded here because a monitoring design that silently inherits
them would be specifying thresholds on top of broken instruments. These are
phase 0 of the implementation plan.

### 7.1 `crash-pattern-detection.sh` — surge detector currently aborts

The **working-tree** copy (uncommitted, 151 insertions vs HEAD) calls
`warn("…")` with Python-style parentheses at lines 158–161:

```
./scripts/crash-pattern-detection.sh: line 158: syntax error near unexpected
token `"⚠️  DEGRADED: crash event source is stale"'
```

`warn()` is defined at line 85 as a normal shell function, so the call must be
`warn "…"` without parens. The script dies at line 158 — **before** surge
detection at lines 210–245 — so the 10-minute monitoring timer has been running
a script that produces nothing past the header. `bash -n` catches this
instantly; HEAD's copy passes it, so the defect arrived with uncommitted work.

**Required:** fix the call sites; add `bash -n` to every monitoring script's
self-test path so a syntax-broken detector cannot ship again. (Per the
shared-worktree rule this design does not commit a fix to another worker's
in-flight file — it is listed for the implementation bead to resolve.)

### 7.2 `resource-monitor.sh` — PSI memory pressure reported 100× too high

`check_memory_pressure()` (line 254) reads PSI `some avg60` — **already a
percentage, 0–100** — then multiplies by 100:

```bash
local pressure_percent=$(awk -v p="$avg60" 'BEGIN { printf "%d", p * 100 }')
```

Live consequence in `.beads/logs/resource-alerts.log`:

```
[2026-09-06T07:55:08Z] [CRITICAL] Memory pressure critical: 299% (>= 80% OOM threshold)
[2026-09-06T08:10:00Z] [CRITICAL] Memory pressure critical: 120% (>= 80% OOM threshold)
```

A pressure value above 100% is impossible; the real readings were 2.99% and
1.20%. With the threshold at 80, *any* host memory stall above 0.8% avg60
declares a critical OOM-level alert. The correct line is
`printf '%d' "$p"` (or, better, compare `avg60` directly against a
fractional threshold).

**Required:** drop the ×100; backfill a correction note in
`resource-alerts.log`; treat every PSI-percentage figure recorded before
2026-09-06 as 100×-inflated when reading history. This defect is in a
**tracked, unmodified** file, so it is safe to fix directly.

**Fixed in this bead (2026-09-06):** `resource-monitor.sh:254` now compares
against the true PSI percentage (`printf '%d', p`, no rescale), with a comment
at the site explaining the prior 100× inflation. Post-fix run reports
`PRESSURE: 0% [OK]` where the same conditions previously produced a spurious
CRITICAL. Remaining follow-up (implementation bead, not this one): backfill
the correction note into `resource-alerts.log` so historical 100×-inflated
entries are marked as such.

### 7.3 Surge signal has no consumer (requirement G-3)

Detection exists; the recommended response — defer new tasks while an event is
active — does not. `scripts/system-event-mode.sh` does not exist, and no
preflight consults any surge state. Addressed in §8.3.

### 7.4 Per-scope memory is checked but never scheduled

`cgroup-memory-guard.sh` (the C1 authority, §3.1) is invoked only from
preflight and `run-isolated.sh`. Between dispatches, nobody is watching the
12 GiB scopes — the exact blind spot the RCA's handoff item 2 describes
("enforced but unobserved"). Addressed in §8.1.

---

## 8. Implementation plan

Ordered so that instruments are fixed before thresholds are added, and
detection is consumed before detection is refined.

### Phase 0 — fix the instruments (blocks everything else)

| # | Item | Effort |
|---|------|--------|
| 0.1 | ~~Fix PSI ×100 in `resource-monitor.sh` (§7.2)~~ — **done in this bead**; remaining: correction note in the alert log | ✅ / 15 min |
| 0.2 | Resolve the `crash-pattern-detection.sh` syntax error (§7.1) with whoever owns the in-flight edit; add `bash -n` self-tests for all monitoring scripts | 30 min |
| 0.3 | Add a `--self-test` mode to each L2 script: verify its own data sources are readable and non-stale (row 8, §5) | 1 h |

### Phase 1 — schedule and consume what exists

| # | Item | Effort |
|---|------|--------|
| 1.1 | New timer `domain-check-memguard.timer` (every 5 min) running `cgroup-memory-guard.sh --check` in **report-only** mode first, emitting §2.3 metric lines; promote to enforcement after 1 week of baseline data | 1 h |
| 1.2 | Emit `gc pack-objects` peak-RSS observation (§5 row 7) from `safe-git-gc.sh`'s existing monitor into the metrics log | 2 h |
| 1.3 | **G-3:** `scripts/system-event-mode.sh` — writes/clears `.beads/state/system-event-mode.json` from the surge signal; `preflight-health-check.sh` consults it and reports `DEFER` while active | 3 h |
| 1.4 | **G-4:** `scripts/gateway-failover.sh` + preflight enforcement: a failed preflight defers the bead rather than starting a worker that will die | 2 h |
| 1.5 | **G-8:** log rotation for `.beads/logs/*.log` — 30-day retention for alert/evidence logs, 7-day for raw metrics, UTC-only timestamps | 2 h |

### Phase 2 — checkpoint/resume rollout (§6)

| # | Item | Effort |
|---|------|--------|
| 2.1 | Extract a shared `scripts/lib/checkpoint.sh` from `safe-git-gc.sh`; refactor safe-gc onto it (behaviour-preserving, verified by its existing tests) | 3 h |
| 2.2 | Apply the protocol to the other long-running paths: `auto-gc-trigger.sh` (auto-remediation, also closes G-2) and any bulk operation | 3 h |
| 2.3 | Exit-75 deferred-not-crashed contract wired into L3 classification | 1 h |

### Phase 3 — feedback loop

| # | Item | Effort |
|---|------|--------|
| 3.1 | **G-5:** `scripts/crash-prevention-feedback.sh` — weekly pass over the metrics/alert logs proposing threshold changes with data; output is a *recommendation*, never an auto-tune | 3 h |
| 3.2 | Reconcile the five competing crash distributions into one counted classification (requirements §5 item 8) so the feedback loop optimizes the right objective | separate bead |

### 8.1 Deferred to NEEDLE (outside this repo, restated for completeness)

G-9 completion detection at the alert source · G-10 dispatch-scope sizing ·
G-11 retry with backoff · G-12 complexity-aware turn budgets · G-13 load-based
throttling. This design's L4 gates are the repo-side half of G-13's gap; the
scheduling half is NEEDLE's.

---

## 9. Integration with declarative-config (k8s side)

### 9.1 Scope boundary — stated plainly, because it is easy to conflate

The fleet crashes this design monitors happen on the **lab box**, in systemd
user scopes. The manifests in `jedarden/declarative-config`
(`k8s/apexalgo-iad/domain-check/`) run the **domain-check application** in
kubernetes, which is a different failure domain: the application has zero known
defects and its container has a 128 Mi memory limit in a namespace with a
working kube-prometheus-stack. **The lab-box monitoring is not moved into k8s**;
systemd user timers on the box remain the scheduler for C1–C3 (they already
work, and containerizing them would put the monitor inside the failure domain
it watches).

What declarative-config *does* get is the k8s-side counterpart of the same
design rules — per-scope (i.e. per-container) memory alerting against the
binding limit, not host-wide:

### 9.2 Planned addition: `k8s/apexalgo-iad/domain-check/prometheusrule.yaml`

A `PrometheusRule` alongside the existing `ServiceMonitor` (which already
scrapes `/metrics` at 30 s; `release: prometheus` label), following the
established precedent `k8s/rs-manager/ai-code-battle/acb-metrics-monitoring.yml`:

```yaml
apiVersion: monitoring.coreos.com/v1
kind: PrometheusRule
metadata:
  name: domain-check-resource-alerts
  namespace: domain-check
  labels:
    release: prometheus
spec:
  groups:
    - name: domain-check.resources
      rules:
        # container-scoped memory against ITS OWN limit (R-A applied to k8s)
        - alert: DomainCheckContainerMemoryHigh
          expr: |
            container_memory_working_set_bytes{pod=~"domain-check-.*",container!="POD"}
              / on(pod) kube_pod_container_resource_limits{resource="memory"} > 0.70
          for: 10m
          labels: {severity: warning}
          annotations:
            summary: "domain-check container >70% of its memory limit"
        - alert: DomainCheckContainerMemoryCritical
          expr: |
            container_memory_working_set_bytes{pod=~"domain-check-.*",container!="POD"}
              / on(pod) kube_pod_container_resource_limits{resource="memory"} > 0.85
          for: 5m
          labels: {severity: critical}
        # the app's own saturating-resource signals
        - alert: DomainCheckDown
          expr: up{job=~".*domain-check.*"} == 0
          for: 5m
          labels: {severity: critical}
```

Thresholds 0.70/0.85 mirror §5 row 1 deliberately: same rule, same rationale,
different altitude (container limit instead of dispatch scope). The
`for:` durations follow the same "two confirmations" logic as §3.5.

Deployment is via the normal GitOps path — manifest in
`declarative-config` → commit → push → ArgoCD sync. **No `kubectl apply`**;
the repo's hard prohibitions apply to this file like any other.

### 9.3 Deliberately excluded

- Alertmanager routing/receiver configuration — operator-owned (notification
  targets are a personal-infrastructure decision, not a design one).
- Host-lab metrics into Prometheus (no node-exporter path from the lab box to
  this cluster is established; inventing one would add a network dependency to
  a monitor that must work when the network is the problem).

---

## 10. Acceptance criteria mapping

| Criterion (from the bead) | Where satisfied |
|---------------------------|-----------------|
| Architecture document written to `docs/crash-prevention-monitoring-design.md` | this document |
| Alert thresholds documented with justification | §3 (per point), §5 (single table with rationale) |
| Integration plan with existing declarative-config | §9 |
| Review and approval of design approach | §11 — **requested, not yet granted** |

---

## 11. Review and approval

This is a design deliverable; the approval criterion cannot be satisfied by
the author. The following is the review ask.

**Reviewer:** repository owner (`jedarden`), or a delegated NEEDLE-side
maintainer for §8.1's items.

**What to review, in priority order:**

1. §3.1 / §5 row 1 — is 85% the right refuse point for per-scope memory, with
   70/80 retained as warn/advisory? (This is the one place the design
   deliberately *adjusts* a task-specified number, and the reasoning should be
   checked against operational experience.)
2. §6.2 step 5 — checkpoint-and-exit-`EX_TEMPFAIL` on a resource breach means
   long-running operations sometimes stop voluntarily before the kernel would
   stop them. Acceptable trade of completion latency for kill-avoidance?
3. §9.1 — agreement that lab-box fleet monitoring stays on systemd user timers
   and is *not* moved into the cluster.
4. §9.3 — whether Alertmanager routing should be in scope.
5. §8 — phase ordering and effort estimates.

**Decision recorded here once made:**

> **Status: PENDING REVIEW** — design complete and submitted 2026-09-06
> (domchk-b1068c3a). Phase 0 (§7 defect fixes) is safe to start on approval of
> this document's findings alone, since both defects are independently
> verifiable with the commands given.

## 12. Related documentation

- `docs/crash-prevention-requirements.md` — the audited inventory and gap list (G-1…G-13) this design implements against
- `docs/investigations/root-cause-determination-domchk-6281555d-2026-09-06.md` — the canonical RCA; source of design rules R-A/R-B and the C1 mechanism
- `docs/crash-response-guide.md` — triage procedure consuming L3 output
- `docs/maintenance/repository-maintenance-guide.md` — gc procedure and repo-size thresholds
- `docs/crashes/bf-198ne-crash-report.md` — the `exit -1` sentinel finding
- `scripts/README.md` — script usage
