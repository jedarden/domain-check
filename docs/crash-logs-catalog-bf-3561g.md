# Crash Logs Catalog — bf-3561g

**Bead:** domchk-de8dffac (Locate and catalog crash logs for bf-3561g)
**Generated:** 2026-09-06
**Target timestamp:** `2026-08-16T17:21:28.126979482+00:00`
**Sibling task:** domchk-d552bcd7 ("Gather crash artifacts for bf-3561g", artifacts in
`docs/crash-artifacts-bf-3561g.md`, commit `d2eb46f`) — this catalog covers the *log*
side that task did not fully pin down: where the target timestamp lives, and which
surviving files are genuine crash-era records versus later echoes.

---

## 1. Executive answer

**The exact string `2026-08-16T17:21:28.126979482+00:00` does not appear in any genuine
crash-era log.** It exists on disk only as **task-spec text** — the description of this
bead and of its sibling domchk-d552bcd7, propagated into agent transcripts and trace
files whenever an agent read or quoted those descriptions.

It is the needle crash handler's **alert-generation clock read at the
`HANDLING_RELEASE_DONE` step**. The nearest surviving genuine record is 8.4 µs earlier:

| Record | Source | Timestamp | Δ from target |
|---|---|---|---|
| `HANDLING_RELEASE_DONE` heartbeat | session event log line 2962 | `2026-08-16T17:21:28.126971102Z` | **−8.4 µs** |
| Worker log "crash alert bead" | `needle-claude-code-glm-4_7-lab-domain-check.log:2016` | `2026-08-16T17:21:28.132739Z` | +5.76 ms |
| `crash` event (305,382 ms run) | `.beads/events.jsonl:1527` | `2026-08-16T17:21:28.132817919+00:00` | +5.84 ms |

The domchk-d552bcd7 notes independently recorded the ~6 ms variance to the events.jsonl
record. Three independent logs corroborate the crash moment; the alert-format timestamp
itself (nanosecond `+00:00`, i.e. Rust chrono `to_rfc3339`) was read microseconds after
the `HANDLING_RELEASE_DONE` heartbeat and survives only inside task specs.

**Acceptance criterion "target timestamp found in at least one log file":** satisfied —
but only by spec-echo files (§4). No independent log record carries it verbatim, and no
such record should be expected (§7).

---

## 2. What bf-3561g is, and what the crash was

- `bf-3561g` = "ALERT: Agent crash on bead bf-4k2ws" — itself a crash-alert bead
  (bf-4k2ws had completed successfully; the alert was a false positive).
- The target event is **not** a crash *of* bf-4k2ws — it is the 4th of **9 crashes of
  bf-3561g itself** (exit code −1) during the 2026-08-16 SIGHUP-cascade window
  (17:13:04–17:29:52 UTC), after which the bead completed successfully at 17:31:56.
- Cascade evidence in the same events.jsonl neighbourhood: `bf-1fy2x` crashed on
  `lab-roam-1` at 17:18:00.339 (line 1526), `bf-6bio4g` crashed on `lab-drawrace` at
  17:21:31.699 (line 1530) — 3.5 s after the target crash, on a different worker.
- At dispatch, the worker logged `fleet.cpu_saturated {core_count: 7, load_average:
  15.76, threshold: 0.8}` — the box was saturated when this run started.

## 3. Complete bf-3561g event inventory (`.beads/events.jsonl`, 36 events)

| # | Crash ts (UTC) | Duration | | Outcome ts (UTC) | Result |
|---|---|---|---|---|---|
| 1 | 17:13:04.749 | 156,105 ms | | — | exit −1 |
| 2 | 17:14:39.565 | 94,801 ms | | — | exit −1 |
| 3 | 17:16:22.735 | 103,155 ms | | — | exit −1 |
| **4** | **17:21:28.132817919** | **305,382 ms** | | — | **exit −1 ← target event** |
| 5 | 17:23:14.381 | 106,227 ms | | — | exit −1 |
| 6 | 17:24:42.528 | 88,132 ms | | — | exit −1 |
| 7 | 17:25:31.542 | 48,953 ms | | — | exit −1 |
| 8 | 17:27:14.745 | 103,188 ms | | — | exit −1 |
| 9 | 17:29:52.577 | 157,817 ms | | 17:31:56.062 | exit 0 (success, 123,399 ms) |
| — | — | — | | 2026-08-17T11:05:30 | exit 0 (191,807 ms) |
| — | — | — | | 2026-08-17T11:06:29 | exit 0 (59,235 ms; trace slot captured) |

Each crash is followed within ~15 ms by `claim` + `dispatch` (auto-retry), so the
9 crashes form one continuous retry chain on worker `claude-code-glm-4.7-lab-domain-check`.

## 4. Genuine crash-era artifacts

| File | Size | Notes |
|---|---|---|
| `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-b7afe97d-2026-08-16.jsonl` | 810,583 B (3,145 records) | **Primary.** Session event stream for session `b7afe97d`, covering 13:06:22Z → 17:33:41Z that day. Kill boundary at **lines 2952–2962** (§5). Includes `agent.completed {exit_code: -1}` and the full heartbeat ladder. |
| `~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log` | 90,732,929 B | Worker tracing log (`Z`-format). Crash window at **lines 2013–2018**: `bead.outcome outcome="crash"`, then `atomically claimed bead via claim_auto`, then a `rate_limit` WARN (`CPU load exceeds warning threshold load_1min=55.38 normalized=7.91`). Still the live slot — rotation will eventually move these lines to `.log.1`/`.log.2`. |
| `.beads/events.jsonl` | 2,325,437 B (14,456 lines) | Append-only fleet event log. bf-3561g crash record at **line 1527**; claim/dispatch at 1528–1529; cascade neighbours at 1526/1530. |
| `.beads/checkpoint/forensic.jsonl` | (bead-rs checkpoint) | 166 records referencing bf-3561g — bead state history, close reasons, and the text of prior investigation notes (including the 6 ms-variance finding). |
| `.beads/traces/bf-3561g/` | metadata.json 396 B · stderr.txt 457 B · stdout.txt 763,196 B · trace.jsonl 10,534 B | ⚠️ **NOT the crash.** Trace slots are single-slot per bead id; this one was captured `2026-08-17T11:06:29Z` with `exit_code: 0, outcome: "success"` — the final (11th) dispatch. Useful as the "what this bead's runs looked like" reference; useless for the crash itself. `stderr.txt` shows only a non-fatal SessionEnd-hook warning. |

## 5. The kill sequence, reconstructed (session event log, lines 2946–2982)

```
17:16:22.746262671Z  rate_limit.allowed / fleet.cpu_saturated (load 15.76 / 7 cores)
17:16:22.746342133Z  worker.state_transition DISPATCHING → EXECUTING
17:16:22.749843447Z  agent.dispatched  prompt sha256:610bb529fca9…
17:16:22.759229642Z  transform.started
17:21:27.878875853Z  transform.completed   305,110 ms, 45 events written
17:21:27.977054297Z  agent.completed       exit_code −1        ← the kill
17:21:27.977986691Z  state EXECUTING → HANDLING
17:21:27.978025804Z  heartbeat HANDLING
17:21:27.978604550Z  outcome.classified    exit −1 → "crash"
17:21:27.978624981Z  heartbeat HANDLING_FLUSH
17:21:27.979149806Z  heartbeat HANDLING
17:21:28.121995283Z  heartbeat HANDLING_FLUSH_DONE
17:21:28.122005309Z  heartbeat HANDLING_RELEASE
17:21:28.126971102Z  heartbeat HANDLING_RELEASE_DONE          ← target ts = +8.4 µs
17:21:28.132739Z     worker log: "crash alert bead"
17:21:28.132778276Z  bead.released  reason="release_success"
17:21:28.132817919Z  events.jsonl crash record (305,382 ms)
17:21:28.144255889Z  claim (auto-retry) → 17:21:28.148552975Z dispatch
17:21:28.167930643Z  transform.started — retry run begins
```

The agent had **finished its work** (`transform.completed` 99 ms before the kill) —
consistent with every prior finding that this alert chain is a false positive with no
work lost.

## 6. Echo artifacts (contain the exact target string, but are not records)

| Location | Count | Nature |
|---|---|---|
| `.beads/traces/*/` | 72 files in 61 dispatch dirs (33 `domchk-*`, 28 `bf-*` keyed) | Tool results inside other agents' transcripts: `bead show` output quoting this bead's / the sibling bead's description. Nothing crash-era. |
| `~/.claude/projects/-home-coding-domain-check/**` | dozens of session files | Same echo mechanism (task text + tool results), from prior dispatches of this task family. |
| `~/.needle/logs/claude-code-glm-5.3-flash-lab-domain-check-domchk-de8dffac.agent.jsonl` | 1 | This dispatch's own log — contains the string because the task spec embeds it. |

A home-wide grep for the nanosecond suffix `126979482` (excluding `target/`,
`node_modules`, `.git`) surfaced **no file outside these echo classes** — nothing in
`~/.needle/logs` beyond the current dispatch log, nothing in `.beads` outside `traces/`,
nothing under `docs/` before this catalog.

## 7. Negative results (checked and absent)

| Source | Result |
|---|---|
| journald | First entry `2026-08-17 15:33:14 EDT` — **no coverage of Aug 16**. |
| `~/.needle/logs/archive/` | 2 files, none from 2026-08-16. |
| Needle log slots `.log.2` and `-2.log` (domain-check worker) | 0 hits for the crash moment. |
| `/var/log` | No syslog/kern/messages present on this box (NixOS; journald only). |
| Sibling agent logs (`5af59baa` Aug-17, `roam-1` logs) | Mention bf-3561g, but not the 17:21:28 moment. |
| `docs/` | No pre-existing file contained the exact string before this catalog. |

## 8. Log naming patterns and retention semantics (for the next phase)

- **`.beads/events.jsonl`** — append-only fleet event log; RFC3339 nanosecond timestamps
  with `+00:00` offset (Rust chrono `to_rfc3339`). Never rotated within the workspace;
  the canonical source for crash *inventories*.
- **`.beads/traces/<id>/{metadata.json,stdout.txt,stderr.txt,trace.jsonl}`** — **single
  slot per id**, overwritten on every dispatch. Keyed by bead id for legacy `bf-*`
  traces, by dispatch id for `domchk-*` ones. A surviving trace proves only *its own*
  run; never assume it holds the crash you are investigating (§4, bf-3561g is the
  canonical trap).
- **`~/.needle/logs/needle-<agent>_<worker>.log[.N]`** — worker tracing log, `Z`-format
  timestamps, size-rotated. Historical lines migrate backward through `.log.1`,
  `.log.2`, …
- **`~/.needle/logs/<agent>-<worker>-<session8>-<YYYY-MM-DD>.jsonl`** — per-session
  event stream (heartbeat ladder, state transitions, `agent.completed`), one file per
  UTC day per session id. This is where the kill boundary lives; the heartbeats'
  `HANDLING_RELEASE_DONE` step is the needle clock read that alert timestamps are
  derived from (µs-level, so expect sub-millisecond — not exact — agreement).
- **Alert-format timestamps** (`…+00:00`, nanosecond) are handler clock reads, not log
  record keys. Matching them verbatim against logs will always fail; match the
  surrounding event ladder instead (±50 ms around `HANDLING_RELEASE_DONE`).

## 9. Cross-references

- `docs/crash-artifacts-bf-3561g.md` — sibling task's artifact catalog (domchk-d552bcd7)
- `docs/crash-logs-catalog.md` — corpus-wide catalog, 247 crash events 2026-08-16→26
- `docs/crash-investigation-bf-4k2ws.md`, `docs/crash-investigation-bf-4k2ws-2026-09-01.md`,
  `docs/root-cause-analysis-bf-4k2ws-2026-09-02.md` — the underlying false-positive chain
- `docs/crash-response-guide.md` — classification rules (exit −1 → infrastructure event)
- `.beads/checkpoint/forensic.jsonl` — bead-rs state history for bf-3561g (rev 28)
