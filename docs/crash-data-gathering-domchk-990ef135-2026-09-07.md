# Crash Data Gathering — Traces Corpus + System State (domchk-990ef135, 2026-09-07)

**Bead:** domchk-990ef135 ("Gather crash artifacts and investigation data")
**Downstream consumer:** domchk-4857bed2 ("Classify crash type and analyze failure mode")
**Extraction performed:** 2026-09-07T16:20–16:45Z, live on `lab`, repo at `8d326cc` (== `origin/main`, 0 unpushed)
**Status:** data collection complete; classification intentionally NOT performed here (that is the next bead's scope)

This document is the structured data-collection record for the workspace's crash-artifact
corpus. It catalogs every artifact source, extracts crash timestamp / exit code / signal per
record, attaches system state at the crash instants where records survive, and compiles the
existing investigation canon with its live-verification status. Machine-readable companion:
[`docs/crash-data-traces-catalog-2026-09-07.csv`](crash-data-traces-catalog-2026-09-07.csv)
(one row per captured dispatch, 1925 rows).

---

## 1. Data sources catalog

| Source | Location | Records | Coverage | Retention / caveat |
|---|---|---|---|---|
| Per-dispatch traces | `.beads/traces/<bead-id>/` | 1925 dirs (1908 complete) | 2026-08-16 → 2026-09-07 (live) | **Single-slot per bead id** — a re-dispatch overwrites the prior capture, so repeat attempts are undercounted (e.g. bf-1ea4g's 57 attempts appear once) |
| Stray crash record | `.beads/traces/domchk-e761cbd5.jsonl` | 1 | 2026-09-02 | Top-level file, not a dir; bead-rs-style crash marker |
| Fleet worker logs | `~/.needle/logs/` | (fleet-wide) | Aug 11 → present | fabric-prune broken since Aug-17, so dated dispatch logs are never deleted; workspace-scoped counts live in the inventory docs below |
| Resource monitor metrics | `.beads/logs/resource-metrics.log` | 13,264 lines | Sep 1 → present, 5-min samples | cpu_load, memory_pressure, memory_available_gb, disk_free_gb, disk_used_percent |
| Resource alerts | `.beads/logs/resource-alerts.log` | 56 lines | Sep 1 → present | Threshold crossings only |
| Service / crash monitors | `.beads/logs/service-monitor.log` (91k), `.beads/logs/crash-monitor.log` (50k), `.beads/logs/crash-alert-manager.log` | — | Sep 1 → present | Alert-layer records, not raw crash evidence |
| Kernel OOM records | `journalctl -k` | 62,342 kernel lines | Aug 15 19:46 EDT → present | **Displays LOCAL time (EDT = UTC−4)**; pre-Aug-15 kernel records are lost to the Aug-14 16:39 reboot |
| systemd user scopes | `journalctl --user` | — | Aug → present | Per-scope `Consumed X CPU, Y memory peak` lines = the per-dispatch memory bound evidence |
| GC run forensics | `.git/safe-gc.log`, `.beads/logs/git-gc*.log`, `auto-gc.log` | — | Sep → present | What gc ran + outcome |
| Investigation corpus | `docs/` (86 crash-named top-level docs, `crashes/`, `crash-investigations/` ×104, `crash/`, `crash-analysis/`) | ~460 docs | Aug 12 → present | Many stale/contradictory claims — see §6 for the verified canon and `docs/crash-inventory-bf-1ea4g-summary.md` for the claim-conflict matrix |

Not parsed in this pass: the 1908 `stdout.txt` (1.5 MB each, full session transcripts) and
`trace.jsonl` bodies — only event counts and time spans were extracted. They remain in place
unmodified for per-incident drill-down; total corpus ≈ 2.7 GB under `.beads/` (gitignored).

Extraction method: walk each dir, parse `metadata.json` (bead_id, captured_at UTC, exit_code,
outcome, duration_ms, model, timeout_reason), count `trace.jsonl` events, probe `stderr.txt`
for the systemd scope line / OOM / signal strings. Extractor: `.beads/state/domchk-990ef135/extract_traces.py`
(untracked; `.beads/` is gitignored — regenerate from the method description here if lost).
Raw outputs: `traces-catalog.jsonl` + `summary-stats.json` in the same directory.

---

## 2. Corpus totals (1908 complete captures + 17 empty + 1 stray)

Exit code / outcome distribution:

| exit_code | outcome | count | share |
|---|---|---|---|
| 0 | success | 1726 | 90.5% |
| 1 | failure | 173 | 9.1% |
| 124 | timeout (hard limit) | 7 | 0.4% |
| −1 | crash (SIGKILL-class sentinel) | 2 | 0.1% |
| (none) | empty dir (in-flight at extraction) | 17 | — |

By date (all captures / exit=1 / exit=−1 / exit=124):

| Date (UTC) | captures | exit=1 | exit=−1 | exit=124 |
|---|---|---|---|---|
| 2026-08-16 | 2 | 0 | 0 | 0 |
| 2026-08-17 | 124 | 23 | 0 | 1 |
| 2026-08-25 | 72 | 0 | 0 | 0 |
| 2026-08-26 | 282 | 9 | **2** | 0 |
| 2026-08-27 | 32 | 0 | 0 | 0 |
| 2026-08-28 | 2 | 0 | 0 | 0 |
| 2026-09-01 | 390 | 48 | 0 | 0 |
| 2026-09-02 | 694 | 69 | 0 | 1 |
| 2026-09-05 | 2 | 0 | 0 | 0 |
| 2026-09-06 | 148 | 8 | 0 | **5** |
| 2026-09-07 | 160 | 16 | 0 | 0 |

Model eras: glm-4.7 = 1598 captures (through Sep 2), glm-5.3-flash = 310 (Sep 6 onward).

Failure-duration shape (exit=1 population): **no fast-fails** (0 of 173 under 60 s).
glm-4.7-era failures: n=149, median 500 s, with 17 landing in the 570–630 s band — the
600 s hard-timeout boundary of that era. glm-5.3-era failures: n=24, median 1891 s.
Interpretation for the classifier: the exit=1 population is agents failing mid-work after
minutes of execution, not instant crash-on-start — consistent with the service-class
synchronized-failure signature rather than with a code defect.

No bead id appears twice in the corpus (1925 unique ids / 1925 dirs) — see the single-slot caveat above.

---

## 3. Crash events (exit=−1) — full detail

### 3.1 bf-57nao4 — 2026-08-26T22:54:16Z

| Field | Value |
|---|---|
| exit_code / outcome | −1 / `crash` |
| Captured at | 2026-08-26T22:54:16.033Z (= 18:54:16 EDT) |
| Runtime before kill | 50.6 s (trace: 26 events) |
| Model | glm-4.7 |
| systemd scope | `run-p2868370-i233525862.scope` (invocation `beddd61c9ad04415a975d44ee05fb457`) |
| **Scope memory peak** | **231 M** (CPU 2.995 s) — `journalctl --user` teardown line at 18:54:15 EDT |
| Kernel OOM record | **none** in the 22:52–22:58Z window |
| Final trace actions | `bead close bf-57nao4 --reason "Duplicate false positive - original git gc work (bf-173o7e) completed successfully…"` → tool result success → closing summary message → killed |

### 3.2 bf-12gb0r — 2026-08-26T22:54:48Z

| Field | Value |
|---|---|
| exit_code / outcome | −1 / `crash` |
| Captured at | 2026-08-26T22:54:48.546Z (= 18:54:48 EDT) |
| Runtime before kill | 108.7 s (trace: 94 events) |
| Model | glm-4.7 |
| systemd scope | `run-p2866338-i233523830.scope` (invocation `cc7d08865c2d4fdfbaef16048bcd2c78`) |
| **Scope memory peak** | **366.5 M** (CPU 10.638 s) — teardown line at 18:54:48 EDT (matches captured_at to the second) |
| Kernel OOM record | **none** in the window |
| Final trace actions | `bead close bf-12gb0r --reason "ADR-001 Domain Watch feature implementation complete…"` → success → summary → killed |

### 3.3 Window context (22:52–22:58Z, Aug 26)

- Six needle log lines within the window: "This indicates the worker was killed by an
  external process (e.g., SIGKILL, OOM, **capacity governor**)" (22:52:37, 22:52:55,
  22:53:24, 22:53:48, 22:54:53, 22:57:53Z) — a synchronized multi-worker kill wave, not two isolates.
- `journalctl -u systemd-oomd`: **no entries** for Aug 26. (Note: the sibling catalog
  `docs/crash-artifacts-bf-3561g.md` attributes the Aug-26 era to systemd-oomd at 94.71%
  memory pressure; that mechanism is not reproducible from the journal for this window.
  What *is* reproducible: the needle external-kill lines and the 231 M / 366.5 M scope peaks —
  both orders of magnitude below any OOM boundary.)
- Post-kill cost check: bf-12gb0r's deliverable (ADR-001 Domain Watch) is present in the
  tree today (`internal/watch/`, `internal/server/handlers_watch.go`, `routes.go` watch
  routes, `docs/adr/001-domain-watch-webhook-notifications.md`) — the kill cost bookkeeping,
  not the feature work.

### 3.4 domchk-e761cbd5 — 2026-09-02T10:04:00Z (stray top-level record)

```json
{"timestamp": "2026-09-02T10:04:00Z", "exit_code": -1, "signal": "SIGKILL",
 "uptime_seconds": 45, "workspace": "/home/coding/domain-check", "turns_completed": 0}
```

Early-death kill: 45 s uptime, 0 turns completed, signal explicitly named SIGKILL. No trace
dir was captured for it (killed before the capture pipeline produced one).

### 3.5 Timeouts (exit=124, `timeout_reason.hard`)

| Bead | Captured (UTC) | Hard limit | Model |
|---|---|---|---|
| bf-2sddh9 | 2026-08-17T00:11:53Z | 600 s | glm-4.7 |
| domchk-038339b9 | 2026-09-02T11:25:40Z | 3600 s | glm-4.7 |
| domchk-a916732a | 2026-09-06T07:05:29Z | 3600 s | glm-5.3-flash |
| domchk-9d78187a | 2026-09-06T09:07:32Z | 3600 s | glm-5.3-flash |
| domchk-6b613372 | 2026-09-06T16:21:51Z | 3600 s | glm-5.3-flash |
| domchk-b9513e0b | 2026-09-06T19:07:26Z | 3600 s | glm-5.3-flash |
| domchk-db992d55 | 2026-09-06T21:14:14Z | 3600 s | glm-5.3-flash |

A same-day 5-event cluster on Sep 6 (glm-5.3 era), each pinned at exactly its 3600 s hard
limit — workload-bound runs reaching the cap, a distinct class from signal kills.

---

## 4. System state

### 4.1 At the Aug-26 crash instants (recovered)

- Worker scope memory peaks **231 M** and **366.5 M** (§3.1–3.2) — 4 orders of magnitude
  below the repo-bloat-era memcg-OOM peaks (GB-scale, `CONSTRAINT_MEMCG`, bf-1s6c3/bf-4yjq)
  and far below the 12 GiB dispatch `MemoryMax`.
- No kernel `oom-kill` lines, no systemd-oomd activity in the window → the kills came from
  an external actor (needle's own message names SIGKILL/OOM/**capacity governor**).
- Monitor coverage gap: `.beads/logs/` starts Sep 1, so no resource-monitor samples exist
  for Aug 26. journald is the only surviving state source for that date.

### 4.2 Live at extraction (2026-09-07T16:23Z)

| Metric | Value |
|---|---|
| Memory | 62 G total, **46 G available** (16 used, 26 buff/cache) |
| Load | 4.93 / 5.77 / 6.17 (1/5/15 min) on 12 cores |
| Disk `/` | 444 G, **48 G free** (89% used) |
| Repo health (`scripts/check-repo-health.sh`) | **exit 0** — effective pack-memory bound ≈3072 MiB (windowMemory=2g local, deltaCache=1g local, threads=1 local), 0 unpushed backlog |
| Repo size | 101 M `.git`, 1 pack (99.11 MiB), 0 garbage (03:00Z gc run today, completed successfully) |

### 4.3 Disk-pressure episode on extraction day (real, self-resolved)

`resource-alerts.log` + the monitor's own `resource-metrics.log` corroborate each other:
48 G free at 12:23Z → 22 GB at 14:00Z → **20 GB CRITICAL crossings 15:00–16:00Z**
(threshold 20 GB) → 48 G again by 16:25Z. ~28 GB was consumed and then released within ~4 h
(box-wide shared disk; consistent with a large regenerable build dir being created and
cleared). Recorded here because a classifier reading only the alert log, or only live `df`,
would see half the picture.

### 4.4 Kernel memcg kills — all attributed, none are live dispatches

Kernel `oom-kill:constraint=CONSTRAINT_MEMCG` records name **exclusively**
`safe-git-gc-run-*.scope` (35 distinct scopes, Sep 6–7), killing `git`/`bash` tasks at tiny
RSS (example: 63 MB anon) — these are the bounded-gc machinery's own bound-verification and
bounded wide-window runs hitting their designed ceilings. Earlier kernel-kill days:
Aug 16 (1656 OOM-family lines), Sep 2 (60), Sep 6 (132). This live-re-attributes the
fleet-crash-signature finding: **no kernel kill in the journal targets a live agent
dispatch scope.**

### 4.5 Alert-layer noise to discount

`.beads/logs/system-event.log` contains only a synthetic self-test event
(`bead=domchk-testbead`, crash_burst latch test, Sep 7 05:02Z). `crash-pattern-alerts.log`
holds one Sep-1 line. The alert layer's own volume (50k crash-monitor lines vs 3 real crash
records in the trace corpus) is itself a datum for the classifier: alert throughput is not
proportional to real kills.

---

## 5. Structured summary (classifier handoff)

Pre-classification reads, per event class, from the evidence above — each downstream claim
should cite the section here plus its own re-verification:

1. **bf-57nao4 (−1, Aug 26 22:54:16Z)** — post-completion kill: closed own bead as
   duplicate-false-positive, summary emitted, then killed. Memory peak 231 M. → matches the
   crash-response-guide's FALSE_POSITIVE (post-completion cleanup) shape, kill actor external.
2. **bf-12gb0r (−1, Aug 26 22:54:48Z)** — same shape, deliverable independently verified
   present in the tree (§3.3). → FALSE_POSITIVE-class loss profile; part of a synchronized
   6-kill window (INFRASTRUCTURE external actor), not an isolate.
3. **domchk-e761cbd5 (−1, Sep 2 10:04Z)** — early-death SIGKILL, 0 turns. → INFRASTRUCTURE
   (external kill), no work lost by definition.
4. **7× exit=124** — hard-timeout caps, workload-bound. → neither crash nor defect;
   a scheduling/scoping concern.
5. **173× exit=1** — mid-work failures, median ~8 min, no fast-fails, clustered Sep 1 (48) /
   Sep 2 (69) / Sep 7 (16) — consistent with the documented service-class synchronized-failure
   signature; classify per-wave, not per-bead.
6. **No captured event shows a domain-check application error** — zero panics, zero stack
   traces in the crash records; the 157+-investigation "no code defect" canon holds for this
   corpus.

---

## 6. Compiled existing investigation findings (verified canon)

All tracked on `origin/main` @ `8d326cc` (verified via `git ls-tree` from repo root — an
earlier empty probe was a wrong-cwd artifact, not a tracking gap):

| Doc | Role | Live-verified 2026-09-07 |
|---|---|---|
| `docs/crash-prevention-requirements.md` | Gap register G-1..G-13, entry point of the canon | n/a (register) |
| `docs/crash-prevention-design.md` | Response-half design | n/a |
| `docs/crash-prevention-monitoring-design.md` | Detection-half design | n/a |
| `docs/investigations/root-cause-determination-domchk-6281555d-2026-09-06.md` | Canonical RCA: memcg-OOM kill path, ruled-out alternates | mechanism confirmed absent from today's kills (§4.4) |
| `docs/crash-inventory-bf-1ea4g-summary.md` | Corpus inventory + claim-conflict matrix | cited, not re-derived |
| `docs/crash-response-guide.md` | Classification taxonomy + FP detection rules | taxonomy used in §5 |
| `docs/crash-analysis-bf-1s6c3-2026-09-06.md` | 76 dispatches / 71 kills repo-bloat record | superseded-era; repo now 101 M |
| `docs/crashes/bf-4yjq-cleanup-verification.md` | 18 GB → 94 MB repair record | health re-run: exit 0 |
| `docs/maintenance/repository-maintenance-guide.md` | Operating thresholds | bound verify: ✅ |
| `docs/branch-divergence-analysis.md` | Phantom-divergence corrections | health check: 0/0 divergence |

Compiled headline (from the canon, consistent with this extraction): **157+
investigations, zero domain-check code defects.** Real kill mechanisms on this box are
(a) memcg OOM of unbounded maintenance processes during the repo-bloat era — repaired,
repo 101 M, bound verified today; (b) service-class synchronized failures (exit=1 waves);
(c) post-completion external kills of workers that had already closed their bead.

Docs with claims that this extraction corrects or bounds:
- `docs/crash-artifacts-bf-3561g.md` — systemd-oomd attribution not reproducible from the
  Aug-26 journal (§3.3); treat as unconfirmed mechanism.
- Any doc reading `exit -1` as a literal signal number — in this corpus it is the needle
  sentinel for a SIGKILL-class external kill (§3.4 is the one record that names SIGKILL explicitly).

---

## 7. Gaps and caveats

1. **Single-slot traces** — per-bead overwrite; attempt-level counts (57× bf-1ea4g, 131×
   bf-173o7e…) exist only in the fleet-log-based inventory docs, not here.
2. **Monitor logs start Sep 1** — no resource samples for Aug events; journald (LOCAL-time
   stamps, EDT = UTC−4) is the only Aug state source.
3. **Kernel journal starts Aug 15 19:46 EDT** — earlier kernel kills lost to the Aug-14 reboot.
4. **17 empty dirs** — dispatches in flight at extraction time (incl. this bead's own);
   they will populate as those runs finish.
5. **`stdout.txt`/`trace.jsonl` bodies unparsed** — counts and spans only; full transcripts
   remain in place for drill-down.
6. **Corpus is workspace-scoped** — fleet-wide counts live in the committed inventory docs; do
   not sum the two.
7. **Snapshot in time** — extraction ran 2026-09-07T16:45Z; `.beads/traces/` is live and growing.
