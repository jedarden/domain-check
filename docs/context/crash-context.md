# Crash Context — Gathered 2026-09-02

**Bead:** domchk-24032f23 ("Gather and review crash context")
**Method:** primary sources only — `.beads/events.jsonl` (NEEDLE fleet event log, live),
`journalctl` (system + user units), `.beads/logs/*` (monitoring), `bead show`, `git log`.
All timestamps UTC.

## Scope note

The task did not name a specific crash. The **most recent crash event on record** is
therefore the primary subject, with the surrounding historical storm summarized for
context. The fleet event log (`.beads/events.jsonl`) covers **2026-08-16T04:21 → present**
and is the authoritative crash record: 247 crash events total, all `exit_code: -1`.

## Findings (summary)

1. **Most recent crash:** bead `bf-12gb0r`, worker `lab-drawrace` (explore strand),
   killed at **2026-08-26T22:54:48Z** after 164.6 s, `exit_code -1`. Isolated — the only
   crash that day, no cascade, worker kept completing other beads.
2. **Almost certainly a post-completion kill, not a work-losing crash.** The agent
   **closed the bead at 22:54:40.73 — 8.06 s before the kill**. The bead is `Closed`
   with its investigation notes intact. Per `docs/crash-response-guide.md`, work
   committed < 30 s before a kill is the FALSE_POSITIVE (post-completion cleanup)
   pattern; this one is 8 s.
3. **No kernel OOM involved.** `journalctl -k` for 2026-08-26 contains **zero**
   oom/out-of-memory lines. Exit −1 here is a signal kill from the process-management
   layer, not memory exhaustion.
4. **The host was CPU-saturated at kill time.** needle repeatedly refused agent
   launches: `CPU load saturated: 7.47-7.51 (1-min avg) / 7 cores = 1.07 > threshold
   0.80` at 22:53:32, 22:54:05, 22:54:32, 22:55:00. Workers were restart-looping
   (`lab-drawrace` counter 279→280 in this window; lifetime counters elsewhere:
   roam-2 at 2745, bead-forge at 4233). Load remains saturated as of this review
   (7.84 / 9.06 / 8.65 at 2026-09-02T14:52Z).
5. **Two monitoring defects are polluting crash signal** (both reproduced from source,
   see §Monitoring defects). Until fixed, the per-10-minute "247 crashes in last
   24hours" alert lines are **all-time counts, not 24 h counts**, and the 03:15Z
   "191% memory pressure CRITICAL" alert was a unit-conversion artifact (real PSI
   ≈ 1.91%).
6. **The fleet has been crash-free for 6d16h** as of this review (last crash
   2026-08-26T22:54:48Z; 5,450 subsequent fleet events, zero crashes).

---

## Primary crash event record

Raw record from `.beads/events.jsonl` (last of 247):

```json
{"bead":"bf-12gb0r","duration_ms":164614,"event":"crash","exit_code":-1,
 "outcome":"crash","strand":"explore","ts":"2026-08-26T22:54:48.794711874+00:00",
 "worker":"lab-drawrace"}
```

Decoded:

| Field | Value | Notes |
|---|---|---|
| Bead | `bf-12gb0r` | *"ALERT: Agent crash on bead bf-173o7e"* — itself a crash-alert bead |
| Worker / strand | `lab-drawrace` / `explore` | explore strand picks up work in other workspaces |
| Dispatched | 2026-08-26T22:52:04Z | adapter `claude-code-glm-4.7`, model `glm-4.7` |
| Killed | 2026-08-26T22:54:48Z | `exit_code: -1`, duration 164.6 s |
| Bead fate | **Closed** at 22:54:40.73 | notes: "Investigation complete: Original crash report was incorrect. Bead bf-173o7e succeeded (exit code 1, max turns exceeded)… Repository is healthy." |

Prior history of the same bead: claimed and **completed successfully** on
2026-08-17T18:21:17Z (exit 0, 289.6 s, `lab-domain-check`, auto strand). The 08-26 run
was a later re-claim that finished its work and was killed during teardown.

### Full bead detail at review time (`bead show bf-12gb0r`)

- Status: **Closed**, Revision 7, updated 2026-08-26T22:54:40.73Z (the close itself)
- Assignee at close: `claude-code-glm-4.7-lab-drawrace`

---

## Timeline — pre-crash / crash / post-crash

All entries from `journalctl` (system + user manager `systemd[2877630]`) and
`.beads/events.jsonl` unless noted.

### Pre-crash (22:49–22:54)

| Time | Event |
|---|---|
| 22:50:54 | `needle-worker@lab-drawrace` instance (PID 3231977) exit-fails: launch deferred 4× (125 s total wait), "CPU load saturated: 6.98 / 7 cores = 1.00 > threshold 0.80" |
| 22:51:24 | systemd schedules restart, **counter at 279**; unit `Starting needle worker lab-drawrace...` |
| 22:52:03 | (events.jsonl) `lab-drawrace` claims `bf-12gb0r` (explore) |
| 22:52:04 | (events.jsonl) dispatch `claude-code-glm-4.7` agent |
| 22:52:54 | systemd logs `Started needle worker lab-drawrace` |
| 22:53:32 | `lab-domain-check` launch deferred — CPU 7.48 (1.07/core) — service exit-fails |
| 22:53:57 | `lab-bead-forge` worker boot fails: bead-forge-source workspace has no bead backend binding (separate chronic failure) |
| 22:54:05 | `lab-test-fix` launch deferred — CPU 7.50 (1.07/core) |
| 22:54:27 | `lab-bead-forge` scheduled restart, **counter at 4233** |
| 22:54:32 | `lab-s1` launch deferred — CPU 7.51 (1.07/core) |

### Crash moment (22:54:39–22:54:48)

| Time | Event |
|---|---|
| 22:54:39 | SSH session 3687 opened for `coding` from `100.72.170.64` (Tailscale) — short-lived connect/close cycle |
| 22:54:40 | Session 3687 disconnected by user; `session-3687.scope` deactivated successfully; logind removes session |
| 22:54:40.73 | **Agent closes bead `bf-12gb0r`** (bead-rs update timestamp — work complete) |
| 22:54:43 | `cgov` (claude_governor) collector errors: `Failed to load cursors: JSON error: key must be a string at line 4812 column 2` (recurring; also at 22:52:43) |
| 22:54:48.79 | **Agent process killed — recorded as `crash`, `exit_code: -1`** (events.jsonl) |

### Post-crash (22:55:00 onward)

| Time | Event |
|---|---|
| 22:55:00 | `needle-worker@lab-drawrace` (PID 3232778) exit-fails after 125 s of launch deferral (CPU 7.47). This is the supervisor's own deferral loop, 12 s after the kill — the worker service was not running an agent at that point |
| 22:55:30 | systemd schedules restart, counter at 280; `lab-drawrace` keeps operating |
| 22:56:36→23:14 | Same worker claims/completes other beads normally (`bf-3d9bqk` complete 22:59:21, `bf-1tqhm8` 23:02:27, `bf-5nfu3z` 23:06:51) |
| — | **No retry of `bf-12gb0r` was needed or attempted**: it stayed Closed; no further events for that bead exist |
| — | Kernel log for 2026-08-26: **0** OOM events (`journalctl -k` grep count) |

### Correlated-but-unconfirmed

- SSH session 3687 opening/closing in the same second as the bead close — correlated
  timing only; the scope deactivated cleanly and no kill is attributable to it.
- `cgov` cursor JSON errors — recurring collector noise, no kill path identified.

### Mechanism assessment (hypothesis, not proven)

Exit −1 = harness-recorded signal kill (see `docs/signal-analysis-exit-code-negative-one.md`).
With kernel OOM excluded, the credible sources here are the supervisor/lifecycle layer:
teardown SIGHUP/SIGKILL during the worker's own churn under saturation, or session-scope
signal propagation. What is **proven** is the classification that matters operationally:
the work was finished and committed to the store 8 s before the kill, so this is a
**post-completion termination (false-positive crash)**, matching the
`docs/crash-response-guide.md` "<30 s before crash" rule.

---

## Error context

- **Exit code −1** (247/247 crash events in the log): infrastructure-level signal kill;
  **not** a domain-check code defect and **not** OOM in this instance (no kernel OOM).
- **Durability:** bead close (revision 7) survived the kill — the store commit landed
  first; zero work lost.
- **Supervisor-side errors at the same instant** (not the kill, but the ambient
  condition): repeated `Error: worker launch deferred 4 times (125s total wait),
  system still saturated` across `lab-domain-check`, `lab-test-fix`, `lab-s1`,
  `lab-drawrace`.

## Historical storm context (why the log shows 247)

Crash events by day in `.beads/events.jsonl`:

| Day | Crashes | Notes |
|---|---|---|
| 2026-08-16 | **245** | Sustained drip 04:27→17:29, 72 distinct beads, peak only 3/min — not a single spike. Split: lab-domain-check 153, lab-drawrace 40, lab-test-fix 32, lab-roam-1 20 |
| 2026-08-17 | 1 | `bf-4833lh`, `lab-domain-check`, 16:00:27Z, 686 s, exit −1 |
| 2026-08-26 | 1 | `bf-12gb0r` — the primary subject above |

All 247 are `exit_code: -1`. Existing investigations covering the storm era (do not
duplicate — reference): `docs/crash-investigations/bf-173o7e-*` (the alert family
`bf-12gb0r` belongs to), the bf-65lsdu signal-−1 storm RCA, the bf-4yjq canonical
investigation (Aug-12 storm, 50 verified crashes), and
`docs/signal-analysis-exit-code-negative-one.md`.

---

## Monitoring defects found during this review

Both verified against source on 2026-09-02. They directly degrade crash review and
should be fixed (separate beads recommended; **not** fixed in this pass — this task
is context-gathering only).

1. **`scripts/crash-pattern-detection.sh:80` — no time filtering.**
   `RECENT_CRASHES=$(grep -i "\"event\":\"crash\"" "$EVENTS_FILE")` reads the whole
   file, then the output is labeled `Total Crashes (last $SINCE_TIME)`. Result: every
   10-minute run on 2026-09-02 reports "247 crashes in last 24hours" and re-flags
   August beads ("bf-44x3a crashed 18 times") as fresh. Nothing has crashed in the
   last 24 h at all. The `--since` argument is parsed but never applied to the grep.

2. **`scripts/resource-monitor.sh:254` — PSI percent multiplied by 100.**
   Kernel PSI `avg60` from `/proc/pressure/memory` is already a percentage (0–100);
   `awk 'BEGIN { printf "%d", p * 100 }'` scales it again. The
   `[CRITICAL] Memory pressure critical: 191%` alert at 2026-09-02T03:15:03Z
   (`.beads/logs/resource-alerts.log`) corresponds to a real PSI avg60 of ≈1.91%.
   Current PSI: `some avg60=0.00` (verified). The companion `CPU load high: 11.93`
   warning is real, not affected.

## Current state at review time (2026-09-02 ~14:53 UTC)

| Signal | Value | Assessment |
|---|---|---|
| Crashes since 2026-08-26T22:54Z | 0 (5,450 fleet events since: 2,095 claims, 1,695 dispatches, 1,223 completes, 435 fails, 1 timeout) | 6d16h crash-free |
| Workflow failures (exit 0/1 `fail` events on 08-26) | 240 (177 exit 0, 63 exit 1 max-turns) | separate from crashes; includes retry-until-success loops (e.g. `bf-1cd5v6` failed 3× on one worker, succeeded elsewhere 6 min later) |
| Today's only anomaly | `domchk-038339b9` explore timeout, exit 124 at 11:25:42Z after 3603 s (60-min strand cap, `lab-roam-2`) | harness timeout, not a crash |
| Load average | 7.84 / 9.06 / 8.65 (14:52:59) | still above needle's 0.80/core launch threshold — the Aug-26 saturation condition is chronic |
| Memory | 13 Gi used, 49 Gi available of 62 Gi | healthy |
| Disk | 94 G free of 444 G (78%) | healthy |
| Repo | `.git` 92 MB, size-pack 90.18 MiB, 40 loose objects | healthy (post-bf-1s6c3 cleanup held) |
| Monitoring coverage | resource-metrics.log begins 2026-09-01T22:49Z | no per-minute metrics exist for the crash date; journal is the only system-state source for 08-26 |

## Recommended follow-ups (out of scope here)

1. Fix `scripts/crash-pattern-detection.sh` to actually apply the time window
   (filter crash events by `ts >= now - SINCE_TIME` before counting).
2. Fix `scripts/resource-monitor.sh` PSI unit handling (drop the ×100).
3. Consider recording the supervisor's deferral/saturation state in the crash event
   payload — it would make kill-source attribution mechanical instead of forensic.
