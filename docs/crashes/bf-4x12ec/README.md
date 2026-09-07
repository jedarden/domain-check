# Crash artifact bundle — bf-4x12ec attempt 2 (2026-08-14)

Investigation directory for the crash named as **bead bf-4x12ec, agent
claude-code-glm-4.7, exit code −1 (signal −1), timestamp
2026-08-14T10:25:30.457683731+00:00** — the `Timestamp:` field embedded in
alert bead **bf-3m9m1v** ("ALERT: Agent crash on bead bf-4x12ec", created
2026-08-14T10:25:30.464340624Z). Assembled 2026-09-07 by **domchk-4bad8e94**:
the extracting attempt (`claude-code-glm-5.3-flash-lab-roam-2`, claim window
22:11:13–22:33:28Z) produced the data files below and died before committing;
this attempt re-verified every file against its primary source and added this
README and `MANIFEST.sha256`.

## Relationship to the evidence bundle at `docs/crash-investigations/evidence/bf-4x12ec/`

Sibling bead **domchk-48f3e34d** independently extracted the same worker-log
stream the same evening and landed it as commit `9b32085`
(`docs/crash-investigations/evidence/bf-4x12ec/crash-logs/`, extracted
2026-09-07). The two bundles are complementary, not duplicates; this one adds:

- the tasked location (`docs/crashes/bf-4x12ec/`, the house per-bead bundle
  convention used by `bf-1ea4g/`, `bf-1s6c3/`, `bf-2ildm/`);
- `attempt-index.tsv` — all 53 attempts keyed to **absolute line numbers in
  the original worker log** (the sibling's `exit-code-timeline.txt` indexes
  into its own extracted events file, and its crash-window copy preserves
  positions only implicitly);
- the **attempt-2 bracket** (raw contiguous worker-log lines 4411–4473) and
  the **attempt-2 session transcript** (`971486ad…`, not among the sibling's
  four transcripts);
- the full 1,146-event extract in gz form.

Both extractions agree: each independently pulled exactly 1,146 records from
the same source log, and the per-attempt outcome counts match (44 × exit −1,
8 × exit 124, 1 × exit 0).

## Timestamp resolution — the named instant is not the kill

| Event | Timestamp (UTC) | Source (worker log line) |
|---|---|---|
| Attempt-2 claim | 10:23:16.800695191Z | 4440 |
| Attempt-2 dispatch | 10:23:16.810887451Z | 4449 |
| Kill: `agent.completed` `exit_code: −1` (duration 104,481 ms) | **10:25:01.512001992Z** | 4452 |
| `outcome.classified` → crash | 10:25:01.515989118Z | 4455 |
| `HANDLING_RELEASE_DONE` heartbeat — worker-log copy | 10:25:30.457670958Z | 4465 |
| **`Timestamp:` in alert bead bf-3m9m1v = the named instant** | **10:25:30.457683731+00:00** | bead store (payload differs from the worker-log copy by ~12.8 µs) |
| bf-3m9m1v created | 10:25:30.464340624Z | bead store |
| `bead.released` (`release_success`) | 10:25:35.068944246Z | 4467 |
| `outcome.handled` `action: alerted` | 10:25:35.068951279Z | 4468 |

This is the standard alert-stamp pattern in this fleet: the bead-carried
"crash timestamp" is the post-kill handling heartbeat — here **28.95 s after
the actual kill**, because this kill's handling phase held a 23 s release
window (HANDLING heartbeats at 5 s intervals, 10:25:06–10:25:31Z) between
flush completion (10:25:07.328Z) and release (10:25:30.457Z). The kill is
**attempt 2 of 53** dispatches of bf-4x12ec on Aug-14. Needle pre-0.4.2
minted one alert bead per kill — 44 in total, first `bf-fmg2cw`
(10:23:11.219513632Z), last `bf-5x69lm` (11:28:02.194277764Z); the full
per-kill table is in the sibling bundle's
`alert-beads-exit-timestamps.txt`. Attempt 2's alert bead is bf-3m9m1v.

## What attempt 2 was doing when it was killed

From `session-transcript-attempt2-971486ad.jsonl` (28 records, session
`971486ad-8408-46ca-bbcc-11d4069ca04e`, first record 10:23:17.896Z):

| Time (UTC) | Transcript event |
|---|---|
| 10:23:26.221Z | `git count-objects -vH` → **4649 loose objects, 17.20 GiB**, in-pack 4081, 1 pack, size-pack 9.60 MiB, 0 garbage |
| 10:23:43.210Z | `du -sh .git/` → **18G** |
| 10:23:56.490Z | `free -h` → 62Gi total, **50Gi available**, swap 24Gi / 0B used |
| 10:24:08.660Z | **Final record: a `Bash` `tool_use` — `git gc --aggressive --prune=now` (timeout 600000) — with no matching `tool_result`** |

The transcript therefore ends **mid-tool-call, ≥52.9 s before the recorded
kill instant** (10:25:01.512Z): the agent died inside the gc it had been
tasked to run. Host memory was plentiful at the last reading (50Gi available),
so the kill was not host-wide OOM — it is the dispatch-scope ceiling (the
canonical RCA at `docs/crash-investigations/bf-4x12ec-crash-investigation.md`
Addendum 3 verifies the 12 GiB `MemoryMax` scope directly; `exit_code −1` is
needle's died-without-exit-code sentinel, not a signal number). The same
fatal-command shape is documented for attempt 1 in the sibling bundle
(`transcript-attempt1-crash-8b2a5b0d.jsonl`).

Corpus note: alert bead bf-3m9m1v's close reason (2026-08-17) frames the
crash as OOM "during bead workspace migration" — the Aug-14-era reading. The
per-attempt transcripts show the killed operation was the bead's own task
command, the bare gc, inside the dispatch scope. Both agree the target work
completed later by retry; see the canonical report's Addenda 2–4 for the full
53-attempt reconciliation.

## Storm context (all from the same worker log)

- 53 attempts: **44 × exit −1** (attempts 1–44, completions
  10:23:02.958Z–11:27:26.174Z) → **8 × exit 124** (attempts 45–52, completions
  11:38:07.867Z–12:50:14.283Z, 600 s dispatch-cap durations) → **1 × exit 0** (attempt 53,
  completed 12:58:45.113834930Z — the auto-split template, which created
  children bf-173o7e / bf-5jhvpk / bf-im2sl1 and closed the parent).
- Success tail: `verification.passed` 12:58:45.126649351Z (`gates_run: 1`,
  seq 3340) → `bead.orphaned` 12:58:55.502272008Z (seq 3343) — the
  orphaned-after-success pattern that re-triggered alert regeneration.
- One `worker.handling.timeout` (seq 2157, 10:43:35.281763879Z, attempt 14's
  handling window): `bf sync --flush-only failed`, operation flush — then the
  release completed at 10:43:35.281775796Z (line 4840).
- Worker `claude-code-glm-4.7-lab-domain-check`, needle session `a6dbb1fc`,
  template `pluck/pluck-default`, prompt 71,698 bytes (constant across
  attempts).

## Artifact inventory

| File | What it is | Provenance |
|---|---|---|
| `attempt-index.tsv` | All 53 attempts: claim/dispatch/completion timestamps, exit codes, durations, classification, release reason/action, and the **source line number** of each event in the worker log | Extracted from `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl` (2,598,910 bytes, 10,138 lines, mtime 2026-08-14 19:50 local) |
| `bracket-source-lines.tsv` | Human-readable event table for the attempt-2 bracket window (event type, sequence, heartbeat state, exit code, outcome per line) | Same source, lines 4411–4473 |
| `needle-events-2026-08-14-bf-4x12ec-attempt2-bracket.jsonl` | Raw event bracket around the target crash: attempt-1 claim (line 4411, 10:21:06.969Z) → attempt-3 claim (line 4473, 10:25:41.457Z), verbatim contiguous lines | Same source, `sed -n '4411,4473p'` |
| `needle-events-2026-08-14-bf-4x12ec.jsonl.gz` | Every bf-4x12ec record in the Aug-14 worker log — 1,146 records (census below) | Same source, `grep bf-4x12ec`, gzipped |
| `session-transcript-attempt2-971486ad.jsonl` | The attempt-2 agent session transcript, verbatim (177,245 bytes, 28 records) | `~/.claude/projects/-home-coding-domain-check/971486ad-8408-46ca-bbcc-11d4069ca04e.jsonl` (mtime 2026-08-14 06:25 local) |
| `MANIFEST.sha256` | Content hashes of every file above | — |

`~` above = `/home/coding`. Event census of the 1,146 records: 301
`heartbeat.emitted:HANDLING`; 106 `agent.routing_decision`; 53 ×
claim/build.heartbeat/dispatch/transform.started/transform.completed/
agent.completed/outcome.classified/outcome.handled/HANDLING_POST_HANDLER;
52 × HANDLING_FLUSH/HANDLING_RELEASE/HANDLING_RELEASE_DONE/bead.released (the
53rd attempt closed instead of releasing); 51 HANDLING_FLUSH_DONE; 1
`worker.handling.timeout`; 1 `verification.passed`; 1 `bead.orphaned`.

The durable originals remain in place at the paths listed above; the bundle
copies exist so analysis never depends on `~/.needle` or `~/.claude`
retention. The `.jsonl` copies are committed with `git add -f` — the
repo-wide `*.jsonl` ignore rule (.gitignore:70) targets scratch artifacts,
not evidence bundles (same convention as the bf-1ea4g / bf-2ildm bundles).

## Sources searched and negative findings (2026-09-07)

| Location | Result |
|---|---|
| `~/.needle/logs/*.jsonl` (Aug-14 set: domain-check, drawrace, roam-1, roam-2, s1, test-fix) | Only `claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl` carries bf-4x12ec events (1,146). drawrace / roam-2 / s1 / test-fix: 0. roam-1: 1 unrelated housekeeping record (`mend.zero_activity_log_cleaned`, 20:40:00.646Z) naming the since-deleted per-bead agent log path `claude-code-glm-4.7-lab-domain-check-bf-4x12ec.agent.jsonl` |
| `.beads/logs/` | **No Aug-14 coverage.** Earliest entries across every log are 2026-09-01/2026-09-02 — the monitors postdate the crash by 18 days. This answers the task's `.beads/logs/` step by absence; the Aug-14 crash log lives only in `~/.needle/logs` (plus the session transcript in `~/.claude/projects`) |
| `.beads/traces/` | No `bf-4x12ec` slot survives (531 `bf-*` slots, none for this bead). Traces are single-slot per dispatch and the Aug-14 slots were reclaimed; the worker log is the only surviving crash log for this bead |
| Kernel journal | Unrecoverable — the single surviving boot starts 2026-08-15 19:56:33 EDT, after the crash. The memcg mechanism stays regime-matched, not kernel-proven, for this bead |

## Verification performed against the sources (2026-09-07, this attempt)

- `needle-events-2026-08-14-bf-4x12ec.jsonl.gz` decompresses to **exactly**
  the 1,146 lines `grep bf-4x12ec` returns on the primary worker log (diff
  clean, run 2026-09-07).
- `needle-events-2026-08-14-bf-4x12ec-attempt2-bracket.jsonl` is
  byte-identical to `sed -n '4411,4473p'` of the primary worker log.
- `session-transcript-attempt2-971486ad.jsonl` is byte-identical (`cmp`) to
  `~/.claude/projects/-home-coding-domain-check/971486ad-8408-46ca-bbcc-11d4069ca04e.jsonl`.
- `attempt-index.tsv` = 54 lines (header + 53 attempts); exit distribution
  44 × −1 / 8 × 124 / 1 × 0; line-number spot checks (4411 claim, 4452
  completed, 4465 heartbeat) byte-match the source log.
- Alert payload re-read live from the bead store 2026-09-07: bf-3m9m1v
  (Closed, rev 3) carries `Timestamp: 2026-08-14T10:25:30.457683731+00:00`,
  `Exit code: -1 (signal -1)`, `Agent: claude-code-glm-4.7` — the instant
  this bundle resolves.

## Related records

- `docs/crash-investigations/evidence/bf-4x12ec/crash-logs/` — sibling bundle
  (domchk-48f3e34d, commit `9b32085`): full crash-window worker-log segment,
  attempt-1 / mid-storm / autosplit / split-success transcripts, all 44
  alert-bead records and their timestamp table.
- `docs/crash-investigations/bf-4x12ec-crash-investigation.md` — canonical
  report (v1.6): 53-attempt storm, mechanism, addenda.
- Root-cause family: memcg-OOM SIGKILL inside the 12 GiB dispatch scope —
  same mechanism as bf-173o7e (the gc child's own later storm) and bf-198ne
  (the `git push` variant). Mitigation: `pack.windowMemory=2g` /
  `deltaCacheSize=1g` / `threads=1` (repo + global) + `safe-git-gc.sh` — see
  `docs/maintenance/repository-maintenance-guide.md`.
