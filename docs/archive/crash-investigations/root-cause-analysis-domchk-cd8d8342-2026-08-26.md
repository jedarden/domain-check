# Root Cause Analysis: domchk-cd8d8342 — agent death on bead bf-12gb0r (2026-08-26T22:54:48Z)

**Investigation bead:** domchk-cd8d8342
**Crashed dispatch:** session `a2774f2b`, worker `claude-code-glm-4.7-lab-drawrace`, bead `bf-12gb0r`
**Classification:** FALSE_POSITIVE (post-completion termination) + one verified needle defect
**Confidence:** HIGH — full dispatch log, agent transcript, and system journal all recovered

## Summary

The agent dispatched to `bf-12gb0r` completed 100% of its task, **closed the bead at
22:54:40.730Z**, emitted its final completion message at 22:54:48.291Z, and was then
killed by an unrecorded signal during teardown (~200 ms later). Needle recorded
`exit_code: -1` (the `code().unwrap_or(-1)` sentinel for a signal death, per the
bf-4x12ec analysis), classified the outcome as a crash, and raised this alert.
No work was lost; no retry was needed.

## Timeline (UTC, 2026-08-26)

| Time | Event |
|------|-------|
| 22:52:03.9 | drawrace claims `bf-12gb0r` (strand `explore`, P2) |
| 22:52:04.1 | agent dispatched — `--max-turns 30`, pluck template, 58,399-byte prompt |
| 22:52:16–22:53:20 | agent verifies ADR-001 Domain Watch: `go build ./...` ✓, `go test ./...` ✓ (watch pkg: 5 impl + 2 test files, all PASS) |
| 22:54:07.8 | `git add .needle-predispatch-sha && git commit` → "nothing to commit, working tree clean"; `git push` → "Everything up-to-date" |
| 22:54:40.690 | `bead close bf-12gb0r --reason "ADR-001 Domain Watch feature implementation complete…"` → success |
| 22:54:48.291 | final assistant message emitted — **complete, not truncated**; session-end bookkeeping (`last-prompt`, `atis-latch`) written |
| 22:54:48.343 | transform.completed (94 events, 164.1 s) |
| 22:54:48.546 | **agent.completed `exit_code: -1`** → classified crash → alert raised (this bead, domchk-cd8d8342) |
| 22:54:53 | needle crash handler runs `bead release bf-12gb0r` → **bead exit 4: "Cannot release issue in closed status"** → worker exits status=1 (restart counter 217) |

## Root cause of the exit −1

The claude process died by a signal needle could not record, **after** its final
message completed. Every standard cause is excluded by artifacts:

| Hypothesis | Evidence | Verdict |
|---|---|---|
| memcg OOM (the bf-4x12ec / bf-173o7e cause) | dispatch scope `run-p2866338` peaked at **366.5M** against the 12 GiB scope limit | excluded |
| kernel OOM kill | `journalctl -k` 18:00–19:30 local: zero oom/kill records (crash is inside the current boot, so records would survive) | excluded |
| systemd-oomd | oomd is enabled+active but logged **no kill** in the window (oomd logs every kill) | excluded |
| SIGSEGV / trap | no kernel trap records for any process in the window | excluded |
| max-turns exhaustion | 72 assistant messages ended in a **natural, complete** summary; `error_max_turns` exits 1, not a signal death | excluded |
| git gc / repo bloat | no gc running; `.git` = 93 MiB (healthy; bloat threshold is >1 GB) | excluded |
| fleet killer scripts (`memory-watch.sh`, `crash-circuit-breaker.sh`, `agent-concurrency-limiter.sh`) | all created 2026-09-02 — did not exist on Aug 26 | excluded |

The precise signaling agent is not recoverable from surviving artifacts (traces are
single-slot and long overwritten). What is proven: the death happened **after**
successful completion, in teardown — the exact "post-completion cleanup" signature
the crash-response guide classifies as FALSE_POSITIVE.

## Verified needle defect: crash handler kills the worker on closed beads

Five seconds after classifying the crash, the worker ran
`bead release bf-12gb0r` — but the agent had already closed the bead, so bead-rs
rejected it (exit 4). That error is **unhandled**: the worker main returned
`Error: backend 'bead-rs' operation 'release' exited with code 4`, systemd marked
`needle-worker@lab-drawrace` FAILED, and the unit auto-restarted (counter 217).

Scope of the defect on Aug 26 alone: **18 worker deaths** from
`Cannot release issue in closed status` plus 1 from
`Cannot release assigned open issue - use 'update --clear-assignee' instead`.
Each converts one agent crash into a worker crash + restart. The two release
variants share a shape: needle's outcome handler does not tolerate bead CLI
rejection of a release it requested speculatively.

Recommended fix (needle side): treat `bead release` exit 4 (conflict — wrong
status/assignment) as a benign no-op in the crash handler — the bead is already in
its terminal state; log and continue. Do **not** let the worker process exit on it.

## Fleet context (why Aug 26 looked like an infrastructure storm)

- **1,108** `worker stopped unexpectedly` events on Aug 26; restart counters in the
  thousands. ~22 of those had an immediate `Error:` line (the release conflicts
  above); the rest were external kills with no worker-side error — consistent with
  the day-wide churn described in the Aug-26/Sep-2 synthesis docs.
- `cgov` (claude governor, `~/.local/bin/cgov _token-collector`, up since Aug 16)
  failed its **collection pass every 2 minutes all day**: `Failed to load cursors:
  JSON error: key must be a string at line 4812 column 2`. Fleet-level claude
  process governance was blind during the churn window. (Not proven to be a killer
  here — but a degraded governor in the loop during a 1,108-restart day is worth
  fixing on its own; the cursor file at "line 4812" has a formatting defect.)

## Finding: task/bead mismatch in the crashed dispatch

`bf-12gb0r` is an **alert bead** ("ALERT: Agent crash on bead bf-173o7e", created
2026-08-14), yet the pluck prompt handed the agent the **ADR-001 Domain Watch
implementation task** (its first action, 12 s in, was reading
`docs/adr/001-domain-watch-webhook-notifications.md`). The close reason it wrote is
therefore mislabeled for this bead. Outcome was still acceptable — earlier attempts
had already resolved the underlying alert (bead notes: "Original crash report was
incorrect. Bead bf-173o7e succeeded (exit code 1, max turns exceeded)…"), and the
ADR-001 work itself was real (committed later that evening in `a0f3cda`, 01:03:14Z).
But dispatching a feature-implementation prompt against a crash-alert bead is a
prompt-source bug worth a separate needle-side look: strand `explore` claimed an
alert bead and received an unrelated task body.

## Disposition

- `bf-12gb0r`: correctly closed; the failed `bead release` meant no retry fired, so
  the crash produced **no duplicate work**.
- This bead (domchk-cd8d8342): crash investigated; classification FALSE_POSITIVE
  (post-completion) with one actionable needle defect (release-conflict worker
  death) and one degraded dependency (cgov cursor corruption) documented above.
- No domain-check code defect: the agent's session touched no source files
  (tree was clean; its commit attempt was a no-op).
