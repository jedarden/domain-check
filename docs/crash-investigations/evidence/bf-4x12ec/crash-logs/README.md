# bf-4x12ec — Raw Crash Logs

Raw evidence bundle for the bf-4x12ec crash ("Execute aggressive git garbage
collection to eliminate OOM risk", 2026-08-14). Extracted from the surviving
needle run artifacts and local logs; **no original was moved, modified, or
deleted** — every file here is a verbatim copy or a clearly-labelled derived
index built from a verbatim copy.

Investigation bead: **domchk-48f3e34d** · Extracted: **2026-09-07**

## Crash timestamp and exit signal

| Field | Value |
|-------|-------|
| Bead | `bf-4x12ec` (P2, task) |
| Worker | `claude-code-glm-4.7-lab-domain-check` |
| Needle session | `a6dbb1fc` · adapter `claude-code-glm-4.7` (model glm-4.7) |
| Crash window | 2026-08-14 **10:21:06Z → 11:27:26Z** — **44 attempts killed**, each surviving 39–116 s |
| **Recorded crash timestamp** | **2026-08-14T10:23:11.219513632Z** — the first kill's `HANDLING_RELEASE_DONE` heartbeat (needle seq 1750); this is the timestamp carried by alert bead `bf-fmg2cw`, the first of 44 auto-minted alert beads |
| **Actual kill instant (crash #1)** | **2026-08-14T10:23:02.958717335Z** — `agent.completed` event, seq 1741, duration 115 797 ms |
| **Exit status** | **exit_code = −1** on all 44 kills (needle's sentinel for a killed worker process — delivered as SIGKILL; **not** a literal signal number) |
| Outcome classification | `outcome.classified → "crash"` ×44, → `"timeout"` ×8 (exit 124 at the 600 s agent cap), → `"success"` ×1 (12:58:45Z, attempt 53) |
| Mechanism | memcg-OOM SIGKILL of `git gc --aggressive --prune=now` inside the 12 GiB dispatch scope, against a repo holding 17.20 GiB / 4 649 loose objects (18G `.git`). Host memory was NOT exhausted (45Gi available captured mid-storm, 10:43:59Z). Kernel journal for the window is unrecoverable (single surviving boot starts 2026-08-15 19:26 EDT), so the mechanism is regime-matched, not kernel-proven, for this bead. |
| Trigger (verbatim, attempt 1 transcript, line 48 of 50) | `git gc --aggressive --prune=now` — tool_use issued, **no tool_result ever returned**; the agent died inside the operation. The same command opens `transcript-attempt1-crash-8b2a5b0d.jsonl`'s tail. |

Attempt census (full per-attempt table: `exit-code-timeline.txt`):
53 claims → **44 × exit −1** (10:23:02–11:27:26Z) → **8 × exit 124** (600 s
timeouts, 11:28–12:50Z) → **1 × exit 0** (12:58:45Z — the auto-split template
decomposed the bead into children bf-173o7e / bf-5jhvpk / bf-im2sl1 and closed
the parent as an umbrella).

## Files in this directory

| File | What it is | Source path | Captured |
|------|------------|-------------|----------|
| `needle-worker-log-bf4x12ec-events.jsonl` | **All 1 146 raw needle event lines** mentioning bf-4x12ec: claims, dispatches, `agent.completed` with exit codes + durations, `outcome.classified`, `outcome.handled`, release heartbeats, `verification.passed`, `bead.orphaned` (seq 1729–3345) | `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl` (10 138 lines) via `grep "bf-4x12ec"` | 2026-09-07 |
| `needle-worker-log-crash-window-full.jsonl` | **The complete raw worker-log segment** covering the crash window — source file lines 4411–6027 — including the 471 interleaved lines without the bead id (`worker.state_transition`, `rate_limit.allowed`, `fleet.cpu_saturated`, …) for full loop context | same source, `sed -n '4411,6027p'` | 2026-09-07 |
| `exit-code-timeline.txt` | *Derived* (not raw): per-attempt table pairing claim → dispatch → `agent.completed` (exit code, duration) → classification → handled action, for all 53 attempts, with `line` references into the events file above | built from `needle-worker-log-bf4x12ec-events.jsonl` | 2026-09-07 |
| `transcript-attempt1-crash-8b2a5b0d.jsonl` | Verbatim transcript of **crash #1** (session `8b2a5b0d-0aae-4226-8ff8-e9d263e84045`, 50 lines). First-line tag `[needle:claude-code-glm-4.7-lab-domain-check:bf-4x12ec:auto]`. Contains the failing `gc.aggressivewindow` config errors (exit 128 ×2), the `git config --unset` fix, and ends at the fatal `git gc --aggressive --prune=now` tool_use with no result | `~/.claude/projects/-home-coding-domain-check/8b2a5b0d-0aae-4226-8ff8-e9d263e84045.jsonl` | 2026-09-07 |
| `transcript-midstorm-9539f3b2.jsonl` | Verbatim transcript of a mid-storm attempt (session `9539f3b2-…`, 23 lines) that captured `df -h` / `free -h` **8 s before its own kill**: disk 85% used / 67G free, **45Gi memory available, 0B swap** — the host was not OOM; the kill was scope-limited | `~/.claude/projects/-home-coding-domain-check/9539f3b2-eabe-432b-8d9f-7b5b0abc931d.jsonl` | 2026-09-07 |
| `transcript-autosplit-timeout-c48ec3f3.jsonl` | Verbatim transcript of a `split`-template attempt (session `c48ec3f3-…`, 7 lines; contains the "Auto-Split: Decompose This Bead" prompt) producing **zero assistant events** — exit 124 at exactly 600 s. One of the 8 timeouts | `~/.claude/projects/-home-coding-domain-check/c48ec3f3-0420-455f-9e48-7c728cc914fe.jsonl` | 2026-09-07 |
| `transcript-split-success-31800ee3.jsonl` | Verbatim transcript of the **successful final attempt** (session `31800ee3-…`, 62 lines, exit 0): creates the three child beads, chains them, labels the umbrella, closes the parent | `~/.claude/projects/-home-coding-domain-check/31800ee3-de7c-4619-abe8-07468fb7de32.jsonl` | 2026-09-07 |
| `alert-beads-raw.jsonl` | Verbatim forensic-checkpoint records of the **44 auto-minted "ALERT: Agent crash on bead bf-4x12ec" beads** (needle pre-0.4.2 minted one alert bead per kill) | `/home/coding/domain-check/.beads/checkpoint/forensic.jsonl` (read-only grep) | 2026-09-07 |
| `alert-beads-exit-timestamps.txt` | *Derived* index of those 44 alert beads: id → recorded heartbeat timestamp → exit code → created_at, showing first `bf-fmg2cw` @ 10:23:11.219513632Z, last `bf-5x69lm` @ 11:28:02.194277764Z | built from `alert-beads-raw.jsonl` | 2026-09-07 |

`~` above = `/home/coding`.

## Integrity — source vs copy (sha256, 2026-09-07)

Every verbatim copy hash-matches its source at capture time; the sources were
re-hashed **after** extraction and are unchanged.

```
61ee5956…de695d  8b2a5b0d…jsonl  (source) == transcript-attempt1-crash-8b2a5b0d.jsonl
d844a5d7…842b9b  9539f3b2…jsonl  (source) == transcript-midstorm-9539f3b2.jsonl
f753c28c…70f3b40  c48ec3f3…jsonl  (source) == transcript-autosplit-timeout-c48ec3f3.jsonl
55859d43…2d9271  31800ee3…jsonl  (source) == transcript-split-success-31800ee3.jsonl
needle log source: 61efe590…1b1c6d (24 lines matching bf-4x12ec … 1 146 lines total, unchanged 2026-08-14 19:50 mtime)
```

The needle-log source file's mtime is still 2026-08-14 19:50 — untouched.
Transcript source mtimes remain 2026-08-14 (06:23, 06:44, 08:09, 08:58).

## Extraction method

```bash
# needle event stream (raw, verbatim)
grep "bf-4x12ec" ~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl \
  > needle-worker-log-bf4x12ec-events.jsonl          # 1146 lines
sed -n '4411,6027p' ~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl \
  > needle-worker-log-crash-window-full.jsonl        # 1617 lines, seq 1729-3345
# transcripts (raw, verbatim, cp -p)
cp -p ~/.claude/projects/-home-coding-domain-check/<session>.jsonl transcript-*.jsonl
# alert beads (read-only grep of the bead-rs durable checkpoint; .beads/ never written)
grep bf-4x12ec .beads/checkpoint/forensic.jsonl → 44 'ALERT' issue records → alert-beads-raw.jsonl
```

Independent cross-check: a prior extraction of the same needle log
(`/tmp/bf4x12ec-events.jsonl`, from the 2026-09-02 investigation domchk-2ff261ce)
is **byte-identical** to `needle-worker-log-bf4x12ec-events.jsonl` — the source
log has not changed since 2026-09-02.

## Sources searched and negative findings

| Location | Result |
|----------|--------|
| `~/.needle/logs/*.jsonl` | Only `claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl` mentions bf-4x12ec (1 146 lines). No other needle log hits. |
| `.beads/logs/crash-monitor.log` | **0 hits** — the crash monitors did not exist on Aug 14 (earliest entries 2026-09-01). |
| `.beads/logs/crash-pattern-alerts.log` | Single line, 2026-09-02 (unrelated later storm). No Aug-14 record. |
| `.beads/logs/` other | `crash-resolution-tracker.log` 1 line (2026-09-07: bf-4x12ec marked resolved, bead_closure) — disposition, not crash-era evidence. |
| `/tmp` | `/tmp/bf4x12ec-events.jsonl` (prior extraction, byte-identical to ours) and `/tmp/bf4x12ec_forensic_matches.jsonl` (370 broader forensic records incl. non-issue types; the 44 alert-bead issue records here are the filtered subset). Not treated as sources — both re-derived from primaries above. |
| Kernel journal | **Unrecoverable.** Single surviving boot starts 2026-08-15 19:26 EDT — after the crash. No kernel OOM lines for Aug 14 survive. |
| `.git/gc.log`, reflog, Aug-14 pack | Absent / expired / rewritten by later maintenance (per the 2026-09-02 artifacts survey). |
| Needle log itself | Contains no "oom"/memory events — only exit codes and heartbeats. |

## Timestamp reconciliation (why two "crash timestamps" exist)

- **10:23:11.219513632Z** — the dispatch's recorded crash timestamp = the
  `HANDLING_RELEASE_DONE` heartbeat of crash #1 (seq 1750, 8.3 s *after* the
  kill). Alert bead `bf-fmg2cw` carries it. Alert-bead timestamps are release
  heartbeats, never kill instants.
- **10:23:02.958717335Z** — the actual kill instant (`agent.completed`, seq 1741).
- **10:25:30.457683731Z** — the heartbeat of **crash #2** (alert bead
  `bf-3m9m1v`); cited as "the" crash time by
  `docs/crash-investigations/bf-4x12ec-crash-investigation.md`. With 44 kills,
  no single instant is "the" crash — see `exit-code-timeline.txt` for all 44.

## Related records

- `docs/crash-investigations/bf-4x12ec-crash-artifacts-2026-09-02.md` — the
  artifacts survey this bundle extracts (domchk-2ff261ce)
- `docs/crash-investigations/bf-4x12ec-root-cause.md`,
  `bf-4x12ec-final-crash-report.md`, `bf-4x12ec-crash-investigation.md`,
  `bf-4x12ec-log-review-2026-09-02.md`
- Root-cause family: memcg-OOM in the 12 GiB dispatch scope — same mechanism as
  bf-173o7e (the gc child bead's own later storm) and bf-198ne (the
  `git push` variant). Mitigation: `pack.windowMemory=2g` / `deltaCacheSize=1g`
  / `threads=1` (repo + global) + `safe-git-gc.sh` — see
  `docs/maintenance/repository-maintenance-guide.md`.
