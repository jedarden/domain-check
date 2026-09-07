# Crash artifact bundle — bf-2ildm (2026-08-13 alert storm)

Retrieval bundle for the crash event **bead bf-2ildm** ("Extract GitHub-specific
commits", created 2026-08-13T11:12:57.942289666Z, now **Closed**). Assembled
2026-09-07 by **domchk-ea755548** ("Retrieve crash logs and metadata", Child
Bead 1 of the 2026-09-02 investigation split).

**Scope boundary:** this bundle retrieves and organizes crash data only —
timestamps, exit codes, session identities, log contents. Classification and
root-cause analysis belong to the sibling beads (domchk-e05fa5a8,
domchk-a863a1f9, domchk-87b7b0e8, …) and are deliberately not attempted here.

## Crash identification (acceptance criteria 1–3)

| Field | Value | Source |
|---|---|---|
| Crashed bead ID | **bf-2ildm** | worker log, every record |
| Needle worker | `claude-code-glm-4.7-lab-domain-check` | worker log `worker_id` |
| Needle worker session | `e29942f7` | worker log `session_id` |
| Agent / model | `claude-code-glm-4.7` (provider `zai`, model `glm-4.7`) | `.beads/traces/bf-2ildm/metadata.json`, routing events |
| Dispatch template | `pluck` / `pluck-default`, prompt_len 70,745 | `agent.dispatched` data |
| Crash window | **2026-08-13T13:37:24Z → 15:53:28Z** (38 kills in 2h16m) | worker log `agent.completed` |
| Exit code (crash era) | **−1** on all 38 kills — needle's died-without-exit-code sentinel, not a signal number | worker log + alert bead descriptions |

### Attempt summary — 43 attempts, 2026-08-13

| Exit code | Outcome | Attempts | Handling |
|---|---|---|---|
| −1 | crash | **38** (attempts 1–38, 13:37:24Z–15:53:28Z) | each → `bead.released` + `outcome.handled: alerted` → one alert bead each |
| 124 | timeout | 4 (attempts 39–42, 16:03:48Z–16:35:06Z, each exactly 600,000 ms) | `handled: deferred`, **no alert** |
| 1 | failure | 1 (attempt 43, 16:35:26Z, **19 ms** — died before work started) | **bead quarantined** 16:35:40.675Z (`failure_count: 5`, `threshold: 5`) |

Full per-attempt detail (claim/dispatch/kill/classified/released/handled
timestamps + source line numbers + alert mapping): `attempt-index.tsv`.

### Alert-storm mapping (acceptance criterion 3, continued)

Every one of the 38 crash attempts maps **1:1 and unambiguously** to one of the
38 `ALERT: Agent crash on bead bf-2ildm` beads, matched nearest-after the kill
(7.7 s min / 29.4 s max): `crash-alert-ledger.tsv` carries each alert bead's
ID, creation timestamp, bead-carried crash stamp, bead-carried exit code (−1),
and current status. No crash attempt lacks an alert and no alert bead lacks a
kill — there are no duplicates and no orphan alerts in this storm.

Metadata observation (retrieval-level, not analysis): the bead-carried crash
stamp is a post-kill handling heartbeat 7.7–29.4 s after the actual
`agent.completed` kill — the same alert-stamp pattern documented for
[bf-1ea4g](../bf-1ea4g/README.md). Cite the kill instants in
`attempt-index.tsv`, not the alert-bead stamps, when ordering events.

## Crash logs located (acceptance criterion 1)

| Location | What it held for bf-2ildm | Retention state |
|---|---|---|
| `.beads/traces/bf-2ildm/` (single-slot trace archive) | `metadata.json`: `exit_code: 0`, `outcome: "success"`, `captured_at: 2026-08-16T22:28:44Z`, duration 85,327 ms | **No crash-era trace survives** — the slot was overwritten by a later (successful) run. Current state copied verbatim to `trace-archive-current-state.json` + `trace-archive-current-stderr.txt` |
| `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` (3,111,314 bytes, mtime 2026-08-13 19:59 local) | **919 bf-2ildm records** — the primary crash-era source: 443 heartbeats, 86 routing decisions, 44 dispatches, 43 claims/completions/classifications/releases/handlings, 1 quarantine | Durable original still in place; full extract bundled as `needle-events-2026-08-13-bf-2ildm.jsonl.gz` |
| 38 alert bead records (`bf-3b3b8` … `bf-2v8x98`) | Embedded crash report per bead: bead ID, agent, exit code −1, timestamp | Live in the bead store; ledger extracted to `crash-alert-ledger.tsv` |
| `~/.claude/projects/-home-coding-domain-check/` agent transcripts | Crash-era attempt transcripts; **24 of 38 kill instants match a transcript mtime to the second** | Attempt 1's transcript bundled verbatim (below) |

## Crash log contents saved (acceptance criterion 4)

| File | Contents | Provenance |
|---|---|---|
| `needle-events-2026-08-13-bf-2ildm.jsonl.gz` | All 919 bf-2ildm worker-log records, verbatim, gzipped | Lines filtered from the 2026-08-13 worker log |
| `needle-events-2026-08-13-bf-2ildm-attempt01.jsonl` | First-crash event bracket, verbatim: attempt-1 claim (log line 5766, 13:35:34.306Z) → `outcome.handled` (line 5790, 13:37:37.298Z) — 19 records spanning dispatch, kill (exit −1, 110,309 ms), classification, release, alert | Same source, lines 5766–5790 |
| `session-transcript-attempt01-81bc97cc.jsonl` | Attempt-1 agent session transcript, verbatim (63 records, 199,117 bytes) | `~/.claude/projects/-home-coding-domain-check/81bc97cc-8c03-40f2-b941-b775e6cd25e0.jsonl`; identity confirmed by first-line tag `[needle:claude-code-glm-4.7-lab-domain-check:bf-2ildm:auto]`, first record 13:35:36.185Z, mtime = kill instant 13:37:24Z, ends abruptly (no completion record) |
| `attempt-index.tsv` | All 43 attempts with exit codes, durations, timestamps, log line numbers, alert-bead mapping | Derived from the worker log + bead store |
| `crash-alert-ledger.tsv` | The 38 alert beads with created timestamps, bead-carried crash stamps, exit codes, current status | `bead list` 2026-09-07 |
| `trace-archive-current-state.json` / `trace-archive-current-stderr.txt` | The single-slot trace's current (Aug-16 success) metadata and stderr, verbatim | Copied from `.beads/traces/bf-2ildm/` 2026-09-07 |

The durable originals remain at the paths listed above; the bundle copies exist
so analysis never depends on `~/.needle` / `~/.claude` retention, and so the
`.beads/`-gitignored trace state is represented in git.

## Documented absences

- **No kernel/OOM or coredump record** accompanies any of the 38 kills in any
  recovered source; `exit_code −1` is the sentinel value in every crash record.
- **No `.beads/logs/` telemetry exists for Aug-13** — the monitoring collectors
  that write there start 2026-09-02 (first `resource-monitor.log` entry). The
  worker log above is the only process-level witness.
- **Stderr for the archived Aug-16 run** (`trace-archive-current-stderr.txt`)
  records only the systemd scope invocation and a SessionEnd-hook failure; it
  is retained for provenance of the trace state, not as crash evidence.

## Target bead status (for sibling Child Bead 2)

bf-2ildm is **Closed** in the bead store (created 2026-08-13T11:12:57Z;
verified Closed live 2026-09-07). The record was appended again on 2026-09-07
(rev 7) with sibling bead domchk-e05fa5a8's crash classification — committed
as [crash-classification-bf-2ildm-2026-08-13-15-01.md](../crash-classification-bf-2ildm-2026-08-13-15-01.md)
@ e3a8820 — which cites this bundle's figures. The work the bead shipped
("Extract GitHub-specific commits") was completed by a later successful
attempt — consistent with the Aug-16 success capture in the trace archive.
Verification of the work product itself is domchk-0d6cd2b9's scope.

## Verification (re-checked against live sources 2026-09-07)

The assembling attempt (this bead's attempt 1) died before committing; attempt
3 re-verified every claim against the live sources and committed the bundle:

- `MANIFEST.sha256`: all 8 files hash-OK, none modified since assembly.
- `trace-archive-current-state.json` / `trace-archive-current-stderr.txt`:
  byte-identical (diff) to the live `.beads/traces/bf-2ildm/` files.
- Worker log `claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` re-counted:
  **919** bf-2ildm records; `agent.completed` exit codes **−1×38, 124×4, 1×1**
  (43 attempts); kill window **13:37:24.838573229Z → 15:53:28.914636262Z**;
  single session `e29942f7`, single worker
  `claude-code-glm-4.7-lab-domain-check`. All match the README table above.
- `attempt-index.tsv`: 43 data rows; attempt 1's kill instant
  (13:37:24.838573229Z) and duration (110,309 ms) match the log record
  exactly; attempt 43 (exit 1, 19 ms, quarantined) likewise.
- `crash-alert-ledger.tsv`: 38 rows; all 38 `status_now` values re-matched
  against the live bead store on 2026-09-07 ~14:00Z — **zero drift**.
- Attempt-1 transcript: first-line tag
  `[needle:claude-code-glm-4.7-lab-domain-check:bf-2ildm:auto]`, sessionId
  `81bc97cc-8c03-40f2-b941-b775e6cd25e0`, enqueue ts 13:35:36.185Z.

## Manifest

Content hashes of every file in this bundle: `MANIFEST.sha256`.
