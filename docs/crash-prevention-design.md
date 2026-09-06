# Crash Prevention — System Architecture and Design

**Date:** 2026-09-06
**Bead:** domchk-21fb2ceb
**Depends on:** domchk-d7c086d6 (`crash-prevention-requirements.md`, 1b21053 — closed), domchk-ff1b585c (root cause analysis — closed)
**Companion:** `docs/crash-prevention-monitoring-design.md` (domchk-b1068c3a) — the detailed detection/threshold specification this architecture contains
**Status:** Design submitted for review (§12)

---

## 0. Document authority map

This workspace has ~460 crash documents, several of which contradict each
other. To avoid adding a fourth competing "master plan", this section fixes
which document is authoritative for what. Everything here was verified live on
2026-09-06; where a claim could not be verified it is marked as an assumption.

| Topic | Authoritative document | Everything else |
|-------|------------------------|-----------------|
| What crashes happened, and why | `docs/investigations/root-cause-determination-domchk-6281555d-2026-09-06.md` | older RCAs are reconstructions, some later inverted |
| Which safeguards exist vs. which are missing | `docs/crash-prevention-requirements.md` (G-1…G-13) | `crash-prevention-status-2026-09-01.md` (its "timers broken" finding is stale — fixed 09-02) |
| Detection: metrics, thresholds, instruments | `docs/crash-prevention-monitoring-design.md` | CLAUDE.md's limit table (kept aligned, less detailed) |
| **Response: gates, retry, error handling, notification** | **this document** | `docs/crash-response-guide.md` (triage procedure, still current) |
| Git gc procedure | `docs/maintenance/repository-maintenance-guide.md` | — |

**What `exit -1` means (R-DOC-1, binding):** "the process died by signal;
mechanism unknown until you read kernel/journald records." It is a needle
sentinel, not a signal number. A correct SIGKILL death encodes as 137. Any new
document asserting a specific signal for `exit -1` without kernel evidence is
wrong by construction.

---

## 1. Purpose and scope

This document is the top-level architecture for crash prevention in this
workspace. The requirements doc names *what* is missing (G-1…G-13); the
monitoring design specifies *how detection works* (metrics, thresholds,
instruments). Neither specifies **what the system does when it fires** — the
gates, the retry/error-handling contracts, the notification routing, and the
ordering of implementation. That is the content of this document.

**Standing constraint:** domain-check application code has zero known defects
across 157+ investigations. Everything here is about the environment the code
runs in. No design element below adds an application code path.

### 1.1 Design rules inherited from the prior analyses

These are constraints, not preferences. Each was learned from an investigation
that first reached the wrong conclusion.

- **R-A — Operate at the altitude of the binding constraint.** The only memory
  limit that has ever killed anything here is the per-dispatch memcg scope
  (`MemoryMax=12 GiB`), not the host's 62 GB. Responses key off per-scope
  readings; host-wide readings are advisory context only.
- **R-B — Fix the measurement before trusting the threshold.** Two live
  instrument defects (a ×100 PSI mis-scale, a syntax-broken surge detector)
  were found while specifying thresholds; both are fixed as of 2026-09-06, and
  Phase 0 keeps the guard (`bash -n` self-tests) that makes that class of
  defect unshippable.
- **R-C — Deferred is not crashed.** The dominant historical cost is false
  positives (60–75% of alerts in the audited window). A deliberate,
  resumable stop must never be counted as a crash attempt — this is the
  exit-75 contract in §6.
- **R-D — Convention is the weakest tier.** A rule that exists only in prose
  *will* be violated under pressure. Every safeguard that matters gets a
  mechanically-enforced backstop (§5.1). Examples of convention-only failures
  already in the record: `setup-git-hooks.sh` cited by three docs including
  CLAUDE.md but never existing; `monitoring-setup.sh` offering a cron install
  path on a NixOS box with no crontab.
- **R-E — Evidence is perishable; design for the next investigation.** Kills
  are uncatchable SIGKILL, so recoverability must be written to disk *before*
  the kill, and retention must outlive the investigation that needs it.

---

## 2. System architecture

### 2.1 Layered model

Five layers plus a cause-side plane. L0–L2 and their thresholds are specified
in the companion monitoring design; they appear here as boxes to show how
response attaches to them.

```
        CAUSE-SIDE PLANE (stop the repository from becoming the bomb)
   ┌────────────────────────────────────────────────────────────────┐
   │ write-time:  .gitignore (.beads/, *.db, *.jsonl) · pre-commit   │
   │              10 MB gate (G-1: needs a reproducible installer)   │
   │ op-time:     pack.windowMemory=2g · deltaCacheSize=1g ·         │
   │              threads=1  → worst case ≈3 GiB per pack run        │
   │ remediate:   auto-gc-trigger → safe-git-gc (G-2: wire + auto)   │
   └────────────────────────────────────────────────────────────────┘
                                  │ repo health metrics
   ┌──────────────────────────────▼─────────────────────────────────┐
   │ L0 MEASURE   cgroup-memory-guard (per-scope) · PSI · loadavg ·  │
   │              df · gateway probe (-skf) · .beads/events.jsonl    │
   ├────────────────────────────────────────────────────────────────┤
   │ L2 DETECT    thresholds from monitoring design §5:              │
   │              memcg 70/80/85 · disk 30/20 GB · load 10/15 ·      │
   │              surge 3-in-5m early / 10-in-10m declared ·         │
   │              gateway 2-probe confirm                            │
   ├────────────────────────────────────────────────────────────────┤
   │ L3 CLASSIFY  crash-classifier (4 classes) · dedup · 5-min       │
   │  & SUPPRESS   cooldown · closed-bead filter · verify-work-      │
   │               completion · (new) exit-75 = deferred             │
   ├────────────────────────────────────────────────────────────────┤
   │ L4 RESPOND   surge gate (G-3) · preflight-before-dispatch (G-4) │
   │  & GATE       · checkpoint/resume (§7) · circuit breaker (§6.4) │
   │               · concurrency limiter (in flight, §9.3)           │
   ├────────────────────────────────────────────────────────────────┤
   │ L5 NOTIFY    routing table (§8): journal · alert logs ·         │
   │              gate files · operator hook (fail-open)             │
   └────────────────────────────────────────────────────────────────┘
```

### 2.2 The two structural decisions

**D1 — Detection and response are separated by a file contract, not a call.**
L2/L3 never *push* an action; they write state (`.beads/state/*.json` gate
files, `*-alerts.log` lines) and L4 consumers *pull* it. Consequences:

- a broken detector degrades to "no gate active", never to a wrong action;
- a responder can be added, tested, or removed without touching the detector;
- the gate file is itself evidence — an investigation can read *what the
  system knew* at the moment it acted.

**D2 — Every layer fails open except the two that guard data.** Detection
gates (memory, surge, gateway) fail open: an unreadable reading yields
"unknown", and only `--strict` promotes unknown to refuse. The write-time data
guards fail closed: a pre-commit hook that cannot run must block, not pass,
because the one fully-owned crash cause (repo bloat, P5) entered exactly
through a commit that should have been blocked. This asymmetry is deliberate —
availability of the fleet is less critical than integrity of the repository.

### 2.3 Scheduling substrate (verified live 2026-09-06)

All on-box scheduling is **systemd user timers**. Six exist and all had future
triggers when checked; `cgroup-memory-guard.sh` — the C1 authority — is on
none of them (gap, §10 Phase 1). Operating rule: after editing any unit file,
`systemctl --user daemon-reload`, or the timer silently never fires (this bit
the weekly full-gc on 2026-09-02).

| Timer | Cadence | ExecStart |
|-------|---------|-----------|
| `domain-check-service-monitor` | 2 min | `service-monitor.sh --once` |
| `domain-check-resource-monitor` | 5 min | `resource-monitor.sh --once` |
| `domain-check-monitoring` | 10 min | `crash-pattern-detection.sh` |
| `domain-check-repo-health` | daily 02:00 | `auto-gc-trigger.sh --dry-run` |
| `domain-check-git-gc` | daily 03:00 | `safe-git-gc.sh` (`MemoryMax=4G`) |
| `domain-check-git-gc-full` | Sun 04:00 | `safe-git-gc.sh --full` (`MemoryMax=4G`) |

**Why not Prometheus for the box itself:** the fleet crashes on the lab box,
and a monitor that must reach a cluster over the network adds a dependency
that fails when the network is the problem (C3's own failure mode). On-box
timers with append-only logs keep detection inside the failure domain, where
it keeps working. Prometheus *is* the right instrument for the containerized
domain-check application in `apexalgo-iad` — a `PrometheusRule` mirroring the
same 0.70/0.85 ratios against the container's own limit, deployed through the
declarative-config GitOps path, is already specified (companion §9). Same
rule, two altitudes; neither substitutes for the other.

---

## 3. Crash classes and their designed responses

The requirements doc's taxonomy, extended with the response each class gets in
this architecture. This table is the behavioural core of the design.

| # | Class | Signature | Designed response | Explicitly *not* |
|---|-------|-----------|-------------------|------------------|
| K1 | memcg OOM in dispatch scope | `exit -1`, kernel `CONSTRAINT_MEMCG`, `usage==limit` | pre-empt: checkpoint + exit 75 at the 85% refuse line (§7.3); bound the operation (pack config); isolate with `run-isolated.sh` when available | alert as a code defect; re-run unbounded |
| K2 | repo bloat | `.git` > 1 GB, loose:packed inverted | auto-remediate: auto-gc-trigger → safe-gc (G-2); write-time guards stop recurrence | manual bare `git gc --aggressive` |
| K3 | SIGHUP / restart cascade | mass signal deaths, minutes, many workers | declare event (10-in-10); defer dispatch; one event record; beads self-heal on retry | per-bead investigation beads |
| K4 | post-completion death | work committed + `verify-work-completion` marker exists | classify FALSE_POSITIVE, suppress, close | re-investigate; re-dispatch the finished bead |
| K5 | `error_max_turns` | `exit 1`, terminal reason `error_max_turns` | verify the deliverable first (most are complete); split only if genuinely unfinished | blind retry at the same turn budget |
| K6 | gateway 503/502 | `exit 1` + HTTP 5xx | 2-probe confirmation → retry with backoff (§6.2); defer beads at 3 consecutive | immediate investigation bead |
| K7 | CPU saturation | load > 2× normalized sustained | warn at 10, gate new dispatch at 15; queue via concurrency limiter | rely on warnings alone (bf-xumcu: 826 crashes/day did exactly that) |
| K8 | duplicate/false alert | alert for a closed/resolved bead | dedup + closed-bead filter + cooldown (exists, 12/12 tests) | treat alert volume as crash volume |

---

## 4. Operational safeguards (guard rails)

### 4.1 Three enforcement tiers

Safeguards are classified by how hard they are to bypass. The design goal is
that every safeguard that protects data sits in Tier H, and every Tier P rule
that matters gets a Tier H or E backstop.

| Tier | Meaning | Current members | Bypass cost |
|------|---------|-----------------|-------------|
| **H — hard** | kernel or tool enforces; forgetting is impossible | memcg `MemoryMax` on scopes; `pack.windowMemory=2g` / `deltaCacheSize=1g` / `threads=1` (local *and* global, verified); `.gitignore` (`/`, `*.db`, `*.jsonl`); pre-commit 10 MB hook | must be deliberately removed |
| **E — enforced gate** | a script refuses; bypassing is a visible decision | preflight-health-check; `cgroup-memory-guard.sh` refuse at 85%; (new) surge gate; (new) preflight-before-dispatch; (new) concurrency limiter | `--force` / explicit skip flag |
| **P — procedural** | prose and convention | "run preflight before dispatch"; "flush beads before commit"; "never bare gc" | nothing — this is the gap tier |

**Rule (R-D applied):** converting P→E or P→H is the highest-leverage change
available. G-1, G-3, G-4, and G-7 are all exactly this conversion.

### 4.2 The four enforcement points

Where a guard rail can act, in the order a unit of work passes through them:

1. **Write-time** (before a bad object enters history): gitignore patterns +
   pre-commit size gate. Strongest and cheapest; this is what ended bf-4yjq.
   *Currently the only P-tier member of this set is the hook's installability —
   G-1 fixes it by restoring `scripts/setup-git-hooks.sh` (installer +
   `bash -n` self-test), or better by moving the hook to a tracked
   `core.hooksPath` directory so a fresh clone inherits it.*
2. **Operation-time** (while a memory-heavy command runs): pack memory bounds
   (H), `cgroup-memory-guard.sh --check` before each stage (E), graceful
   abort before the kernel OOM decision (`memory-watch.sh`, in flight), and
   sub-scope isolation (`run-isolated.sh`, in flight) so a runaway can only
   kill itself, not the dispatch scope's agent.
3. **Dispatch-time** (before a worker starts): preflight gate, extended by
   G-3 (surge gate — "is a system event active? → DEFER") and G-4 (gateway
   gate — "is the dependency up? → DEFER"). The convention "check preflight
   first" becomes an enforced gate; a failed preflight defers the bead
   instead of starting a worker that will die.
4. **Remediation-time** (after damage, before it compounds): auto-gc-trigger
   wired into the daily 02:00 timer with `safe-git-gc.sh --auto-when-needed`
   implemented (G-2), turning the existing detection into a closing action.
   Remediation is always the *bounded* safe-gc path — never bare gc.

### 4.3 Guard-rail contracts

Each gate is a script with the same shape, so consumers need one integration
pattern:

- **Input:** the resource it guards; no arguments required for the common case.
- **Output:** human-readable reason on stdout/stderr, and a single-line JSON
  verdict appended to its state file (`.beads/state/gates/<gate>.json`) with
  `{gate, verdict, reason, observed, threshold, at}` — this is what makes the
  gate auditable after the fact.
- **Exit codes:** the §6.1 contract (`0` pass / `2` refuse / `3` unknown).
- **Fail-open default, `--strict` for refuse-on-unknown** (D2).

New gates to build (all E-tier, all currently missing — verified 2026-09-06):
`system-event-mode.sh` (G-3), `gateway-failover.sh` (G-4),
`setup-git-hooks.sh` (G-1), and the `--auto-when-needed` mode of safe-gc
(G-2). `scripts/monitoring-setup.sh` (G-7) is retired to a shim that prints
the systemd-timer instructions, because a live install path that silently
no-ops is worse than none.

---

## 5. Resource monitoring — summary by reference

The thresholds are specified, justified, and where possible live-verified in
the companion monitoring design (§3 per signal, §5 consolidated). This
architecture adopts them unchanged; the one-line summary:

| Signal | Altitude | Warn | Critical | Gate/refuse |
|--------|----------|------|----------|-------------|
| memcg used / `MemoryMax` | **per-scope** | 70% | 80% (advisory) | **85%** |
| host PSI memory | host | 70% | 80% | — (advisory only, R-A) |
| disk free on `/` | host | < 30 GB | < 20 GB | pre-build gate at 20 GB |
| CPU load (1 min) | host | > 10 | > 15 | gate new dispatch > 15 |
| crash surge | workspace | 3 in 5 min | 10 in 10 min | defer while declared |
| gateway health | external | 1 failed probe (`-skf`) | 2 consecutive | defer dispatch at 3 |

Two additions this architecture requires of the measurement layer, both from
the RCA's handoff: **observe pack-objects peak RSS** during gc (the bound is
enforced but unmeasured), and **detect staleness of the event source** — a
monitor that cannot distinguish "no crashes" from "not looking" reports health
through an outage.

---

## 6. Retry logic and error handling

### 6.1 The exit-code contract

One contract, every gate and long-running script. The distinguishing idea is
R-C: a *deliberate* stop is not a failure.

| Exit | Meaning | Retried? | Alerted? | Resumed? |
|------|---------|----------|----------|----------|
| `0` | passed | — | no | — |
| `1` | genuine failure | per class (§3) | yes, after classify | no (restart op) |
| `2` | refuse (gate) | no | metric line only | when gate clears |
| `3` | unknown (fail-open) | treated as pass | no | — |
| `75` | **EX_TEMPFAIL — deferred, checkpoint written** | no | no | **yes, from checkpoint** |

Exit 75 exists because the fleet's costliest error is *classification*, not
crashes: without it, a gate-induced stop is indistinguishable from a kill and
re-enters the false-positive machinery (P3). Any retry counter that sees 75
must ignore it.

### 6.2 Retry policy by error class

Retry is not a property of an operation; it is a property of the *error*. The
classes and their policies:

| Error | Policy | Backoff | Cap |
|-------|--------|---------|-----|
| HTTP 502/503 (gateway) | retry, then defer bead | 1→2→4→8→16 s, ±20% jitter | 5 attempts |
| network timeout / DNS | retry | same schedule | 3 attempts |
| gate refuse (exit 2/75) | **never retried** — resume when gate clears | n/a | n/a |
| max-turns | verify deliverable, then split; never blind-retry | n/a | 1 verification pass |
| same stage failed 3× with flat duration | circuit-break (§6.4) — stop and record `degenerate: true` | n/a | hard stop |
| surge active | defer everything non-essential until gate clears | n/a | gate lifetime |

The backoff schedule is deliberately modest: the documented CLAUDE.md pattern
(5 attempts, base 1 s, doubling) is retained so prose and implementation
agree. Jitter is added because the Aug-16 cascade showed synchronized retries
re-creating the load that caused the event.

### 6.3 Checkpoint / resume protocol

SIGKILL is uncatchable, so recovery is designed *before* the kill. The proven
reference is `safe-git-gc.sh` (staged, idempotent, JSON checkpoint after every
stage on both outcomes, `--resume`). Generalized for new long-running
operations (companion §6.2, adopted):

- checkpoint at a fixed path `.beads/state/checkpoints/<operation>.json`,
  carrying an **inputs fingerprint** — a mismatched fingerprint discards the
  checkpoint instead of resuming into changed state;
- checkpoint on gate breach and exit 75 **before** the kernel decides (§7.3) —
  the single highest-value change in this section;
- attempt counter with flat-duration detection feeds the circuit breaker;
- explicit non-goals: never checkpoint Rust `target/` (regenerable; backing it
  up doubles disk), never flush beads before `git pull`.

### 6.4 Circuit breaker

Retry loops are themselves a crash cause: bf-173o7e's 129 duplicate
investigations and its 30-turn close loop are both "repetition without
progress". The breaker (per bead *and* per operation):

- trip after **3 consecutive failures without progress** (no checkpoint
  advance, no output growth, flat duration);
- while tripped: refuse re-entry, record `degenerate: true` with evidence, and
  defer to a human or a different approach;
- reset only on demonstrated progress, never on a timer.

`scripts/crash-circuit-breaker.sh` (in flight, untracked) implements the
per-bead form; the operation-level form comes from the checkpoint protocol's
attempt counter. Same rule, two scopes.

---

## 7. Response design: what happens when a threshold fires

Tracing three representative fires end-to-end, since this is the part the
companion doc deliberately leaves to this document.

### 7.1 Surge goes from early-warning to declared

1. `crash-pattern-detection.sh` (10-min timer) counts 3 events in 5 min →
   early warning: metric line + advisory alert log entry.
2. It writes `.beads/state/system-event-mode.json` **{active: true,
   declared_at, window_count}** (G-3 — the missing consumer).
3. Every subsequent preflight reads the gate file and returns **DEFER**;
   dispatchers and agents stop adding load. Per-bead crash alerting is
   suspended for the duration — one event record stands in for N investigations
   (K3/P4).
4. The gate clears when the window count decays below the declared threshold
   for two consecutive readings (hysteresis, so a flickering surge does not
   flap the fleet).
5. Post-event, a single summary record is written naming the window, the
   count, and what was deferred — the artifact a later investigation actually
   needs.

### 7.2 Gateway goes down

1. `service-monitor.sh` (2-min timer) probe fails once → warn, metric line.
2. Second consecutive failure → critical alert; `gateway-failover.sh`
   (G-4) writes `.beads/state/gates/gateway.json` {verdict: refuse}.
3. Preflight consults it → beads **defer**, not dispatch-and-die. In-flight
   work finishes; nothing new starts.
4. Retry-with-backoff applies to the *calls*, the gate to the *dispatches*;
   at 3 consecutive failures (≈6 min) the gate is treated as an event for
   notification purposes (§8).
5. Choice of a failover gateway is an operator decision (open item §12); until
   one exists the gate still functions — it just has no second target.

### 7.3 Memory approaches the refuse line mid-operation

1. `cgroup-memory-guard.sh --check` before each stage (and on the new 5-min
   timer) reads the caller's own cgroup ancestry — the only altitude that has
   ever killed here (R-A).
2. At 85% of the scope's `MemoryMax`: the long-running operation writes its
   checkpoint, emits **exit 75**, and stops. No kill occurs; nothing is
   alerted; resumption is a `--resume` away.
3. This converts the K1 class — the only mechanism with surviving kernel kill
   records — from "uncatchable SIGKILL + investigation" into "resumable stop +
   log line". It is the one intervention that acts *before* the kernel.

---

## 8. Alert notification design

### 8.1 Destinations on this box

There is no pager and no on-call rotation; the honest design names the
destinations that actually exist and defines one clean boundary for adding an
outbound channel later.

| Destination | What reaches it | Latency | Persistence |
|-------------|-----------------|---------|-------------|
| systemd journal | every timer run's stdout/stderr | immediate | journald retention |
| `.beads/logs/*-alerts.log` | WARN and above | immediate | **30-day retention (G-8), rotated** |
| `.beads/state/gates/*.json` | any verdict that changes behaviour | immediate | until cleared (the *action* channel) |
| `.beads/logs/resource-metrics.log` | every L0 reading | immediate | 7-day retention (raw, high-volume) |
| operator notify hook | CRITICAL and EVENT | immediate | whatever the endpoint provides |

**The notify hook contract:** a single script boundary
`scripts/notify.sh <severity> <title> <body>` that L3/L5 call and *only* it
configures endpoints (ntfy/e-mail/anything). It must be **fail-open** — a dead
notifier logs and returns 0; a monitoring system that crashes because its
notifier is down has converted its own failure into a blind spot. The endpoint
choice is operator configuration, recorded as an open item in §12, exactly as
the companion doc deferred Alertmanager routing.

### 8.2 Routing table

| Severity | Fires when | Destinations | Required human action |
|----------|-----------|--------------|----------------------|
| INFO | any L0 reading | metrics log | none |
| WARN | first threshold crossing, single-probe failure, early-warning surge | metrics + alert log | glance at next session |
| CRITICAL | confirmed breach (disk < 20 GB, load > 15, gateway ×2, memcg ≥ 85% observed) | alert log + gate file + notify hook | same day; the gate has already acted |
| EVENT | surge declared (10-in-10), or a K1-class kill with kernel evidence | alert log + event record + notify hook | review the *event* record, not per-bead alerts |

**"To whom"** resolves on this box to: the operator (via the hook, for
CRITICAL/EVENT) and any agent running preflight (via gate files, for
everything that changes behaviour). There is intentionally no email-to-bead
path — alert-to-investigation goes through L3 classification, never directly.

### 8.3 Anti-noise rules

These are the design's answer to P3 (60–75% false positives is the dominant
cost). They are constraints on L3/L5, not optimizations:

1. **Classify before creating.** No investigation bead exists until L3 has
   run: closed-bead filter → dedup → cooldown → work-completion check.
2. **One event, one record.** During a declared surge, per-bead alerts are
   suppressed into a single event record with the window's evidence.
3. **Cooldown:** 5 minutes per (class, subject) — implemented, 12/12 tests.
4. **Storm suppression:** if > 20 alerts of one class arrive in 5 minutes, L5
   emits one storm summary and drops the rest to the metrics log. (The Aug-16
   wave produced hundreds of lines nobody read.)
5. **Every alert carries its evidence pointer:** the metric line, the gate
   file, or the kernel record that justifies it. An alert without a pointer is
   not emit-able — this is the doc-corpus lesson (four incompatible readings
   of `exit -1`) applied at emit time.

### 8.4 Evidence retention (G-8, adopted as a hard requirement)

Kernel OOM records and journald entries: ≥ 30 days. Alert logs: 30 days,
rotated not truncated. Raw metrics: 7 days. Traces: rotate, never
single-slot-overwrite. `gc.log`/`gc.pid` preserved after runs. **All
timestamps UTC** — the EDT/UTC mixing corrupted several past investigations.
Implementation is a small rotation stanza on the daily 02:00 repo-health
timer, not a new daemon.

---

## 9. Implementation phases and priorities

Ordering logic: **fix instruments → consume what exists → add resilience →
add feedback → ask outside.** Each phase is independently shippable and leaves
the system strictly better.

### Phase 0 — fix the instruments (blocks all threshold work)

| Item | Ref | Effort |
|------|-----|--------|
| ~~Resolve the `crash-pattern-detection.sh` syntax error~~ — **resolved 2026-09-06**: the working-tree copy now passes `bash -n` (verified). Remaining: add `bash -n` to every monitoring script's self-test so a syntax-broken detector cannot ship again | companion §7.1 | 30 min |
| Backfill the PSI ×100 correction note into `resource-alerts.log`; historical PSI figures are 100×-inflated (the ×100 rescale itself is fixed — verified absent from `resource-monitor.sh`) | companion §7.2 | 15 min |
| `--self-test` mode per L2 script: sources readable, non-stale | §5 row 8 | 1 h |

### Phase 1 — gates and consumers (all repo-side, closes G-1..G-4, G-7)

| Item | Ref | Effort |
|------|-----|--------|
| `system-event-mode.sh` surge gate + preflight DEFER (§7.1) | G-3 | 3 h |
| `gateway-failover.sh` + preflight enforcement (§7.2) | G-4 | 2 h |
| `setup-git-hooks.sh` installer + self-test, or tracked `core.hooksPath` | G-1 | 1 h |
| `--auto-when-needed` in safe-gc + wire auto-gc-trigger into the 02:00 timer | G-2 | 2 h |
| Retire `monitoring-setup.sh` to a systemd-only shim | G-7 | 30 min |
| Timer for `cgroup-memory-guard.sh --check` (report-only → enforce after 1 week of baseline) | companion §3.1 | 1 h |
| Log rotation + UTC-only (§8.4) | G-8 | 2 h |

### Phase 2 — resilience (checkpoint/resume, §6.3–6.4)

| Item | Ref | Effort |
|------|-----|--------|
| Extract `scripts/lib/checkpoint.sh` from safe-gc; refactor safe-gc onto it | §6.3 | 3 h |
| Apply the protocol to auto-gc and bulk paths; exit-75 wired into L3 | §6.1 | 3 h |
| Circuit breaker at operation level; integrate the in-flight per-bead breaker | §6.4 | 2 h |

### Phase 3 — feedback loop

| Item | Ref | Effort |
|------|-----|--------|
| `crash-prevention-feedback.sh`: weekly pass over metrics/alert logs proposing threshold changes with data (recommendation only, never auto-tune) | G-5 | 3 h |
| Reconcile the five competing crash distributions into one counted classification | req §5.8 | separate bead |

### Phase 4 — NEEDLE / infrastructure (external asks, restated from requirements §4)

G-9 completion-detection at the alert source · G-10 dispatch-scope sizing ·
G-11 retry with backoff in the agent framework · G-12 complexity-aware turn
budgets · G-13 load-based scheduling. This design's gates are the repo-side
half of G-13; the scheduling half is NEEDLE's. These are asks, not commitments
— they are recorded so their absence is visible rather than silent.

### 9.3 In-flight work this design does not assume

The working tree holds another worker's uncommitted scripts. Verified present
2026-09-06, untracked, unscheduled: `memory-watch.sh` (graceful abort before
kernel OOM — slots into §4.2 point 2), `run-isolated.sh` (sub-scope isolation,
§4.2 point 2), `git-gc-monitor.sh` (pack RSS observation, §5), `crash-circuit-breaker.sh`
(§6.4), `agent-concurrency-limiter.sh` / `needle-with-limiter.sh` (queueing at
capacity — the repo-side half of G-13). The architecture has a slot for each;
none is counted as existing until it is tracked, scheduled, and tested. Their
tests (`test-*.sh`) should be wired into the Phase 0 self-test pass when they
land.

---

## 10. Integration points

| Surface | Direction | Contract |
|---------|-----------|----------|
| systemd user timers | schedules L2 | `ExecStart` per §2.3; `daemon-reload` after any unit edit |
| `.beads/state/gates/*.json` | L2/L3 → L4 | the pull-based file contract (D1); single-line JSON verdicts |
| `.beads/state/work-completion/<bead>.json` | workers → L3 | pre-close verification marker; the K4 suppression input |
| `.beads/state/checkpoints/<op>.json` | ops → retry | fingerprinted resume state (§6.3) |
| `.beads/logs/*.log` | all layers → L5/evidence | append-only, UTC, rotated per §8.4 |
| `.beads/events.jsonl` | needle → L2 surge | the crash-event source (2.4 MB, actively written) |
| git config (local + global) | H-tier bounds | `setup-git-gc-config.sh --verify` resolves the effective chain; exit 1 = unprotected |
| preflight-health-check.sh | L4 entry point | already calls cgroup-memory-guard; gains surge + gateway gates in Phase 1 |
| declarative-config | k8s side only | `PrometheusRule` mirroring 0.70/0.85 against the container limit; GitOps path only, never `kubectl apply` |
| NEEDLE (external) | Phase 4 asks | G-9…G-13; carried as explicit requirements, not assumed |
| operator notify hook | L5 → human | `scripts/notify.sh`, fail-open, endpoint = operator config |

---

## 11. Acceptance criteria mapping

| Criterion (from the bead) | Where satisfied |
|---------------------------|-----------------|
| Complete architecture design for monitoring system | §2 (layers, decisions, scheduling), §5 (by reference to the companion's authoritative thresholds) |
| Defined thresholds for each resource type (memory, disk, CPU) | §5 table — memory 70/80/85% per-scope, disk 30/20 GB, CPU 10/15; rationale and live-verification status in companion §3/§5 |
| Design for operational safety guidelines integration | §4 (tiers H/E/P, four enforcement points, gate contracts, G-1…G-8 wiring) |
| Alert notification design (what to alert, when, to whom) | §8 (destinations, routing table, anti-noise rules, retention) |
| Implementation approach document | §9 (phased plan with effort, ordering logic, in-flight reconciliation) |
| Integration points with existing infrastructure | §10 |

---

## 12. Review ask and open items

Reviewer: repository owner, or a delegated NEEDLE-side maintainer for the
Phase 4 items.

1. **§6.1 exit 75 as the deferred-not-crashed contract** — this changes how
   retry counters and L3 treat gate stops. It is the design's central
   classification decision and should be checked against operational
   experience with false positives.
2. **§4.1 tier assignments** — is the P→E→H migration order right? G-1 and
   G-7 are pure conversions; G-2/G-3/G-4 add new E-tier gates.
3. **§8.1 notify hook** — confirm the endpoint should stay operator-chosen
   (open), rather than naming one now.
4. **§9.3** — confirm the in-flight scripts' owners intend them to land; if
   any is abandoned, its slot stays empty rather than being filled by a
   half-tracked file.
5. **§7.2** — whether a failover gateway target should be chosen before or
   after Phase 1 (the gate works either way).

> **Status: PENDING REVIEW** — design complete and submitted 2026-09-06
> (domchk-21fb2ceb). Phase 0 items are safe to start on approval of this
> document's findings alone; both instrument defects are independently
> verifiable with the commands in the companion §7.

---

## 13. Related documentation

- `docs/crash-prevention-requirements.md` — audited inventory and gap list (G-1…G-13)
- `docs/crash-prevention-monitoring-design.md` — companion detection/threshold spec (L0–L2 detail)
- `docs/investigations/root-cause-determination-domchk-6281555d-2026-09-06.md` — canonical RCA; source of R-A/R-B
- `docs/crash-response-guide.md` — per-crash triage procedure (consumes L3 output)
- `docs/maintenance/repository-maintenance-guide.md` — gc procedure and repo-size thresholds
- `docs/crashes/bf-198ne-crash-report.md` — the `exit -1` sentinel finding (R-DOC-1)
- `scripts/README.md` — script usage
