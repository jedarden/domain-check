# Crash Prevention Requirements

**Date:** 2026-09-06
**Bead:** domchk-d7c086d6
**Depends on:** domchk-ff1b585c (root cause analysis) — closed
**Sources:** ~460 crash investigation documents under `docs/`, the safeguard scripts under `scripts/`, and live verification of git config, systemd timers, hooks, and thresholds performed 2026-09-06.

---

## Purpose

This is the single requirements list for crash prevention in this workspace. It
consolidates what every prior investigation concluded, inventories which
safeguards actually exist today (verified, not just documented), and names what
is still missing. Where the source documents disagree, the disagreement is
stated rather than averaged away — several of the discrepancies below are
themselves action items.

**Standing conclusion, unchanged by this review:** domain-check code has zero
known defects. Every crash investigated to date traces to infrastructure,
agent-workflow, or external-service causes. All requirements below are about
the environment the code runs in, not the code.

---

## 1. Summary of crash types

| Type | Signature | Canonical example | Cause chain |
|------|-----------|-------------------|-------------|
| **Repository bloat → OOM** | `exit -1` during a git operation | bf-1s6c3, bf-4yjq | `.beads/` JSONL committed repeatedly → repo grew to 18 GB (17.16 GB loose, ratio 1,832:1) → git operations load loose objects → memory exhaustion → kill. Cleaned 2026-08-16: 18 GB → ~138 MB. |
| **memcg OOM of a git process inside the dispatch scope** | `exit -1`, kernel `CONSTRAINT_MEMCG`, `usage == limit == 12582912kB` | bf-4x12ec, bf-198ne | Unbounded `pack-objects` inside the 12 GiB needle dispatch cgroup — during `git gc --aggressive` (bf-4x12ec, Aug-14) and during `git push` of a 720-commit backlog carrying 5.6 GB of retired bead-forge state (bf-198ne, Aug-16). 414 memcg kills fleet-wide that day. |
| **SIGHUP / signal cascade (fleet-wide)** | mass `exit -1` across workers in minutes | 2026-08-16 cascade, bf-3561g (9 crashes in 16 min) | Fleet management restart cascade. Beads complete successfully on retry; alerts are largely false positives. |
| **Post-completion agent death** | `exit -1` or `exit 1` seconds after work is committed | bf-173o7e (Aug-17), bf-198ne | Work is done and pushed; the agent dies during bead-close/cleanup. ~40–75% of alerts are in this class per later analyses. Self-heals on retry. |
| **`error_max_turns` workflow failure** | `exit 1`, terminal reason `error_max_turns` | bf-173o7e Aug-17 event (30 turns, `num_turns: 31`) | Retry loop on a failing `bead close` exhausts the turn budget. Task work itself succeeded. |
| **Inference gateway unavailable** | `exit 1`, HTTP 503/502 | domchk-c9641ac5 | Gateway `traefik-apexalgo-iad:8443` returns 503 "no available server". No retry/backoff in the agent framework. |
| **CPU saturation mass-crash** | `exit -1` under load 17–37 | bf-xumcu (2026-08-16: 826 crashes/day, peak load 37.42 = 5.35× cores) | Sustained 2.46×+ normalized load; repo healthy, 41 GB free — explicitly not OOM. Warnings exist, throttling does not. |
| **Duplicate / false-positive alerts** | Not crashes at all | bf-173o7e (129 duplicate alerts), bf-1ea4g (18+), bf-4k2ws | Alert system regenerates investigations for already-resolved events. Consumes agent-days; obscures real signal. |

### The exit-code semantics problem

Four incompatible readings of `exit -1` appear across the corpus, and this is
the largest single source of misclassification:

1. **SIGKILL (9) from host OOM** — early bf-1s6c3 / bf-4yjq docs.
2. **SIGHUP (1)** — bf-3hivb, `signal-minus1-root-cause-analysis-verified-2026-09-02.md`.
3. **Ambiguous, one of the above** — `root-cause-analysis-exit-code-negative-one.md`.
4. **A needle sentinel for *any* signal death** (`code().unwrap_or(-1)`), **not
   a signal number at all** — `docs/crashes/bf-198ne-crash-report.md`, which is
   the only reading backed by kernel records and which explicitly supersedes
   (1) and (2).

**Requirement R-DOC-1:** treat `exit -1` as *"the process died by signal;
mechanism unknown until you read kernel/journald records."* The correct Unix
encoding of a SIGKILL death is 137 (128+9). Any new document asserting a
specific signal for `exit -1` without kernel evidence is wrong by construction.

---

## 2. Root cause patterns identified

### P1 — Memory is bounded per-dispatch, not per-machine
The limiter that actually kills processes is the **12 GiB memcg dispatch
scope**, not the host's 62 GB. Early docs reasoning from "62 GB total /
<2 GB available" reached wrong conclusions; bf-198ne's kernel records
(`usage 12582912kB, limit 12582912kB`) settle it. Any safeguard that only
watches host-wide memory misses the binding constraint.

### P2 — Git operations are the memory bomb
Every hard kill in the corpus's top tier is a git process: `gc --aggressive`,
`pack-objects` during push, merge/reconciliation over a bloated object store.
This is why the mitigations that worked are git-specific (pack config bounds,
safe-gc scripts), not generic.

### P3 — Alert quality, not crash volume, is the dominant cost
Of 247 crashes in the 2026-09-02 24-hour window, 60–75% were classified false
positive. bf-173o7e alone generated 129 duplicate investigations. The fleet
spends more agent-time on phantom crashes than on real ones.

### P4 — Crashes cluster in time and are system-wide
73% of the 247-event window's crashes fell in hours 12–17 UTC; the Aug-16
wave hit 4 worker types within 18 minutes. Per-bead response is the wrong
granularity for a system-wide event.

### P5 — Repository state is the one cause this repo fully owns
Bloat was self-inflicted (`.beads/` committed by bf-2ildm) and self-healed
(pack config + safe gc + gitignore + hook). It has not recurred since
2026-08-16 — the longest-running successful prevention in this workspace.
Current state, verified 2026-09-06: `.git` 94 MB, 176 loose objects, 90.43 MiB
pack — all deep in the healthy band.

### P6 — Evidence is perishable
Single-slot trace retention, reflog truncation to Sep-1, no coredumps before
Aug-25, and UTC/EDT timestamp mixing degraded nearly every investigation.
Several "root causes" in older docs are reconstructions from insufficient
evidence and were later inverted (bf-4x12ec's cause was first reported as
"NOT OOM," then re-attributed to memcg OOM).

---

## 3. Existing safeguards inventory

Everything in this table was **verified live on 2026-09-06**, not copied from
the doc that claims it.

### 3.1 Git / repository safeguards

| Safeguard | Where | Verified state |
|-----------|-------|----------------|
| Pack memory bounds | `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` | ✅ Present in **both** local and global config (checked effective chain). Worst case ≈ 3 GiB per pack run vs the 12 GiB scope that killed bf-198ne. |
| Bound verification | `scripts/setup-git-gc-config.sh --verify` | ✅ Present; resolves system→global→local and fails when no effective bound exists. |
| Safe gc wrapper | `scripts/safe-git-gc.sh` (+ `--check-only`, `--full`, `--resume`) | ✅ Present, with `safe-git-gc-monitor.sh` and cgroup-bounded test in `scripts/test-gc-memory-bounds.sh` (pack-objects peak RSS ≈ 312 MiB under a 768 MiB limit). |
| Large-file pre-commit hook | `.git/hooks/pre-commit`, `MAX_SIZE_MB=10` | ✅ Installed and executable (since 2026-09-01). ⚠️ but see gap G-1. |
| `.gitignore` protection | `.beads/`, `*.db`, `*.db.backup.*`, `*.jsonl` | ✅ Present; `git ls-files .beads/` empty. |
| Repo health checks | `scripts/check-repo-health.sh`, `check-repo-size.sh`, `auto-gc-trigger.sh`, `repo-health-monitor.sh` | ✅ All present. |

### 3.2 Monitoring safeguards

| Safeguard | Where | Verified state |
|-----------|-------|----------------|
| Memory-pressure early warning | `scripts/resource-monitor.sh` — `PRESSURE_WARNING=70`, `PRESSURE_CRITICAL=80` | ✅ The 70% early-warning threshold recommended on 2026-09-02 **is** applied. |
| Crash-surge early detection | `scripts/crash-pattern-detection.sh` — `CRASH_SURGE_THRESHOLD=3` in `SYSTEM_EVENT_WINDOW=5minutes` | ✅ Tightened from 10-in-10 as recommended. |
| Service monitor | `scripts/service-monitor.sh` | ✅ Present (gateway health; note it must use `curl -skf` — the self-signed cert makes plain `-sf` a false alarm). |
| Preflight health check | `scripts/preflight-health-check.sh` (memory / disk / load / gateway / repo health) | ✅ Present. |
| Scheduling | 6 systemd **user** timers: monitoring (10 min), resource (5 min), service (2 min), repo-health (daily 02:00), git-gc (daily 03:00), git-gc-full (Sun 04:00, MemoryMax=4G) | ✅ All 6 enabled with future trigger times — the "bad-setting" failures recorded on 2026-09-01 were fixed on 2026-09-02. |

### 3.3 Alert / response safeguards

| Safeguard | Where | Verified state |
|-----------|-------|----------------|
| Crash classification | `scripts/crash-classifier.sh` — FALSE_POSITIVE / SERVICE_FAILURE / INFRASTRUCTURE / CODE_DEFECT | ✅ Present. |
| Duplicate detection | `scripts/alert-deduplication.sh` | ✅ Present. |
| Alert pipeline | `scripts/crash-alert-manager.sh` — closed-bead filtering, dedup, 5-min cooldown (`ALERT_COOLDOWN_SECONDS=300`), `--classify-only`, `--force-alert` | ✅ Present. 12/12 tests in `test-crash-alert-fixes.sh`. |
| Work-completion verification | `scripts/verify-work-completion.sh` → writes `.beads/state/work-completion/<bead>.json` | ✅ Present; lets triage distinguish post-completion deaths from mid-task ones. |
| Task-split guidance | `scripts/bead-split-recommender.sh` | ✅ Present. |
| Application signal handling | `internal/server/server.go:88` — `SIGINT, SIGTERM, SIGHUP` → graceful drain | ✅ Present in source. ⚠️ but see gap G-6. |

### 3.4 What this inventory does *not* cover

NEEDLE-side safeguards (dispatch-scope limits, work-completion detection,
alert dedup at source, retry logic) live in the NEEDLE repository and are out
of scope here. Where they are the right fix, they appear in §4 as external
requirements rather than being silently dropped.

---

## 4. Missing safeguards requirements

### Gaps in this repo (implementable here)

**G-1 — Hook installation is not reproducible.**
`scripts/setup-git-hooks.sh` is cited by CLAUDE.md, `crash-prevention-status-2026-09-01.md`,
and `systemic-crash-prevention-recommendations-2026-09-02.md` — **the file does
not exist**. The hook works today only because it was installed by hand on
2026-09-01; a fresh clone has no bloat protection at all.
*Requirement:* restore an installer that writes `.git/hooks/pre-commit` (or
move the check to a tracked `core.hooksPath` directory), with a self-test.

**G-2 — No automatic remediation of repo bloat.**
`auto-gc-trigger.sh` exists and thresholds are defined (10 GB trigger), but
`safe-git-gc.sh --auto-when-needed` (recommendation #1.3 of the 2026-09-02
systemic doc) was never implemented, and nothing schedules the trigger.
Detection exists; the closing action is manual.
*Requirement:* wire `auto-gc-trigger.sh` into the daily repo-health timer and
implement the `--auto-when-needed` flag (or drop the flag from the docs).

**G-3 — No system-event response.**
Surge **detection** was tightened to 3-in-5-minutes, but the recommended
response — defer new tasks while an event is active (`system-event-mode.sh`,
recommendation #2.3) — does not exist. During the Aug-16 wave the correct
action was to stop adding load; nothing does that.
*Requirement:* a gate script the dispatcher (or an agent's preflight) can
query: "is a crash surge active right now?" plus a documented deferral
convention.

**G-4 — Gateway failover and the retry mandate are unimplemented.**
Recommendation #3 (retry with backoff + failover) is documented as code and as
a CLAUDE.md snippet, but `scripts/gateway-failover.sh` does not exist and
nothing enforces the preflight before dispatch. Service failures cost ~8–12%
of crashes and are fully addressable at the process level.
*Requirement:* at minimum, a failover-aware health check and a hard rule that
preflight failure defers the bead (the convention is documented; the
enforcement point is not).

**G-5 — No prevention feedback loop.**
Recommendation #5.5 (`crash-prevention-feedback.sh`: "3+ of the same class →
propose a threshold/policy change") does not exist. Prevention is currently
static; nothing revisits thresholds against new data.
*Requirement:* weekly analysis job over the monitoring logs that emits
threshold-change recommendations.

**G-6 — The SIGHUP safeguard protects the wrong process.**
`server.go` handles SIGHUP gracefully — but the crash population is **needle
agent workers**, not the domain-check server. This change hardens a binary
that is not the one dying, and the doc that announces it
(`crash-safeguards-and-monitoring.md`) acknowledges the real fix belongs in
NEEDLE. It is harmless and correct as far as it goes; it is not crash
prevention for the fleet.
*Requirement:* keep it, but reclassify it as service hygiene, and carry
"graceful shutdown / signal handling in the agent runner" as an explicit
NEEDLE requirement (it is already listed there as out-of-scope work).

**G-7 — `monitoring-setup.sh` is a live trap.**
It is cron-based (10+ `crontab` references) and this box is NixOS with no
crontab — CLAUDE.md documents that it does not work and that systemd timers
are the mechanism. The script is still present and still referenced by the
docs it contradicts.
*Requirement:* delete it or reduce it to a shim that calls
`scripts/setup-repo-maintenance.sh`, so no future agent installs a silent
no-op.

**G-8 — Evidence retention is inadequate for the next investigation.**
Single-slot traces (last dispatch only), reflog truncation, no coredumps
before Aug-25, and repeated UTC/EDT confusion. Six of the corrections in §1
exist only because someone later found better evidence.
*Requirement:* retain kernel OOM + journald records for ≥30 days, rotate
rather than overwrite `.beads/traces/`, record all timestamps in UTC, and keep
`gc.log`/`gc.pid` after gc runs.

### Gaps outside this repo (NEEDLE / infrastructure requirements)

**G-9 — Work-completion detection at the alert source.** A 30-second
post-completion grace period plus a check of "did the bead actually close?"
would have suppressed most of the 60–75% false-positive class before any
human or agent saw it. `verify-work-completion.sh` provides the marker; the
*producer* of alerts does not consult it.

**G-10 — Per-worker dispatch-scope sizing.** The 12 GiB memcg limit is
uniform. Git-heavy work needs either a larger scope or (better) the guarantee
that pack memory is pre-bounded — which this repo now provides, making the
12 GiB scope survivable. Any workflow that can exceed it without git (e.g.
large test runs) remains unprotected.

**G-11 — Agent-framework retry with backoff** for 502/503, and
**G-12 — complexity-aware turn budgets** (30 turns is not enough for a
task whose *close* step alone loops). Both are NEEDLE configuration.

**G-13 — CPU saturation controls.** Warnings at 2.0× normalized load are
informational only; no throttling, queuing, or per-worker isolation exists
(bf-xumcu: 826 crashes in one day).

---

## 5. Prioritized implementation list

Ordered by (share of historical crashes addressed) × (effort) × (ownership).
Phase 1 is entirely inside this repo.

### Phase 1 — this repo, small, do first

| # | Item | Addresses | Est. effort |
|---|------|-----------|-------------|
| 1 | **G-1** — restore `setup-git-hooks.sh` (or tracked `core.hooksPath`) + self-test | P5 — keeps the one fully-owned cause fixed | 1 h |
| 2 | **G-2** — implement `--auto-when-needed`; schedule `auto-gc-trigger.sh` in the daily timer | P2/P5 — bloat remediation closes the loop detection already has | 2 h |
| 3 | **G-7** — retire/replace cron-based `monitoring-setup.sh` | Removes a documented-but-broken install path | 0.5 h |
| 4 | **G-3** — `system-event-mode.sh` surge gate + documented deferral convention | P4 — system-wide events | 3 h |
| 5 | **G-5** — `crash-prevention-feedback.sh` weekly threshold review | Keeps prevention matched to data | 3 h |
| 6 | **G-4** — failover-aware gateway check + enforced preflight-before-dispatch | Service-failure class | 2 h |

### Phase 2 — evidence and correctness hygiene

| # | Item | Addresses |
|---|------|-----------|
| 7 | **G-8** — retention: 30-day kernel/journald, rotated traces, UTC-only timestamps, keep `gc.log` | P6 |
| 8 | Reconcile the five competing crash distributions (see §6) into one measured, dated classification and retire the rest | P3 |
| 9 | Sweep docs that assert a specific signal for `exit -1` and replace with the R-DOC-1 wording | P6 — stops misclassification at the source |
| 10 | Re-classify `server.go` SIGHUP work as service hygiene (G-6) so it stops being cited as fleet crash prevention | P3 |

### Phase 3 — NEEDLE / infrastructure (external asks)

| # | Item | Addresses |
|---|------|-----------|
| 11 | **G-9** — consult work-completion markers + 30 s grace before raising an alert | P3 — the dominant cost |
| 12 | **G-10** — dispatch-scope sizing policy for memory-heavy work | P1 |
| 13 | **G-11/G-12** — retry with backoff; complexity-aware turn budgets | Service + workflow classes |
| 14 | **G-13** — load-based throttling / adaptive queuing | CPU-saturation class |

---

## 6. Note on the 70/20/8/2 distribution

The task asked for the 70/20/8/2 pattern to be identified. It is documented —
it appears in CLAUDE.md, `docs/crash-documentation-index.md`,
`crash-root-cause-analysis-bf-1s6c3-final.md`, and
`comprehensive-crash-investigation-report-2026-09-01.md` as
**infrastructure 70% / workflow 20% / service 8% / code defects 2%** — but a
requirements doc that repeated it uncritically would mislead implementation
work. Three things must be recorded:

1. **No document derives the numbers from a counted population.** The only
   table that attaches counts (`crash-investigation-findings-summary-2026-09-02.md`)
   maps 70%→180, 20%→47, 8%→15, 2%→5 against 247 events — and then reports
   domain-check defects as **0% = 0** in the same table, contradicting its own
   2% row.
2. **At least five competing splits exist** for overlapping datasets, all
   dated 2026-09-01/02: 70/20/8/2; CPU 60/SIGHUP 35/OOM 5; infra 70/alert
   bugs 20/repo bloat 8; infra 85/workflow 10/service 5/code 0; and
   post-completion 40/SIGHUP 30/bloat-OOM 15/max-turns 10/service 3/code 2.
   The 20% bucket means *workflow failures* in one scheme and *alert-system
   bugs* in another.
3. **Every individual investigation found zero domain-check defects.** A
   distribution that allocates 2% to code defects is not supported by its own
   evidence base.

The 70/20/8/2 figures are best read as the *initial triage heuristic* from the
2026-08-13→09-01 investigation period, subsequently refined (not refuted) by
the 247-event analysis. **Action item #8 in §5 Phase 2** is to replace all five
with a single counted, dated classification — until then, prevention work
should be prioritized by *which causes have documented hard kills* (git
memory, repo bloat, dispatch-scope pressure), which is what §5 does, rather
than by any percentage.

---

## 7. Related documentation

- `docs/crash-response-guide.md` — per-crash triage procedure
- `docs/crash-documentation-index.md` — doc index (2026-09-01)
- `docs/systemic-crash-prevention-recommendations-2026-09-02.md` — the five-recommendation plan this doc audits
- `docs/crash-prevention-status-2026-09-01.md` — status snapshot (⚠️ its "timers broken" finding is stale; fixed 2026-09-02)
- `docs/maintenance/repository-maintenance-guide.md` — git gc procedure
- `docs/crashes/bf-198ne-crash-report.md` — the memcg/sentinel findings that correct earlier mechanism claims
