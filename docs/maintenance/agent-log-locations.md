# Agent Log Locations — domain-check workspace

Where agent/fleet logs live on this box, their naming conventions, and their
retention. Every figure below was measured live on **2026-09-06**; counts and
sizes drift, the paths and naming rules do not.

## Summary

| What | Where | Naming | Retention |
|------|-------|--------|-----------|
| Needle agent dispatch logs (primary) | `~/.needle/logs/` | `<adapter>-<scope>-<dispatch-id>[-<date>][.agent].jsonl` | Dated/legacy families: **none** (pruner broken since 2026-08-17); `.agent.jsonl`: rolling mend prune |
| Repo-local monitoring logs | `<repo>/.beads/logs/` | `<monitor-name>.log` | Append-only, never rotated (6.7 MB since Sep 1) |
| Claude Code session transcripts | `~/.claude/projects/-home-coding-domain-check/` | `<session-uuid>.jsonl` | ~30-day rolling (Claude Code default) |
| System journal (kernel kills, unit logs) | `/var/log/journal/` | binary `.journal` segments | Persistent, unbounded (4 GB; starts 2026-08-15) |

## 1. `~/.needle/logs/` — primary agent logs (7.0 GB, ~39,900 files)

Set by `telemetry.log_dir` in `~/.needle/config.yaml`. One JSONL file per agent
dispatch, each a stream of `agent_message` / tool events with epoch-UTC `ts`
fields. Timestamps here are **UTC** — system journald stamps the same events in
**local EDT** (−4h); do not mix them on one timeline without normalizing.

Naming (current → legacy):

- `claude-code-<model>-<scope>-<tag>.agent.jsonl` — current naming; `<tag>` is
  a bead ID or short workspace tag (`domchk-…`, `sigil`, `pdftract`), e.g.
  `claude-code-glm-5.3-flash-lab-roam-6-domchk-ad053346.agent.jsonl`.
  Short-lived: mend prunes this family on a rolling basis (~162 files survive,
  all within ~2 days of now — `mend.max_log_files: 100` targets these).
- `claude-code-<model>-<scope>-<8hex>-YYYY-MM-DD.jsonl` — older naming; 8-hex
  dispatch ID + UTC date, e.g. `claude-code-glm-4.7-lab-roam-6-0c12f60b-2026-09-03.jsonl`
  (the dominant family: ~38,700 files, ~6.97 GB, ~6,200 files/day recently)
- Legacy/no-date one-offs (~1,450 files, 66 MB): `claude-code-glm-4.7-roam1-<8hex>.jsonl`,
  `claude-test-worker-<8hex>.jsonl`, `strand-runner-<8hex>.jsonl`,
  `needle-<worker>.stderr.log`
- `background-update-<uuid>-YYYY-MM-DD.jsonl` — needle self-update records
  (44 files, 207 B each)

Subdirectories:

- `archive/` — per-day tarballs (`YYYY-MM-DD.tar.gz`), produced by the
  (now broken) pruner; only 2026-07-30 and 2026-07-31 exist (1.6 MB)
- `samples/` — one-off stderr head/tail captures (`<scope>-stderr-{head,tail}-2MB.log`)

## 2. `<repo>/.beads/logs/` — repo-local monitoring logs (append-only)

Written by the `domain-check-*` systemd **user** units and repo scripts, all
append (`>>`), **no rotation anywhere**. `.beads/` is gitignored — these are
local-only and start 2026-09-01.

| File | Writer |
|------|--------|
| `crash-monitor.log` | `domain-check-monitoring.service` (every 10 min) |
| `resource-monitor.log`, `resource-alerts.log`, `resource-metrics.log` | `domain-check-resource-monitor.service` (every 5 min) |
| `service-monitor.log`, `service-metrics.log` | `domain-check-service-monitor.service` (every 2 min) |
| `repo-health.log`, `git-gc-check.log` | `domain-check-repo-health.service` (daily 02:00) |
| `git-gc.log` / `git-gc-full.log` | `domain-check-git-gc.service` (daily 03:00) / `domain-check-git-gc-full.timer` (Sun 04:00) |
| `crash-alert-manager.log`, `alert-deduplication.log`, `circuit-breaker.log`, `crash-resolution-tracker.log`, `processed-alerts.txt` | `scripts/crash-alert-manager.sh` and siblings |
| `work-completion.log` | `scripts/verify-work-completion.sh` |

## 3. `~/.claude/projects/-home-coding-domain-check/` — session transcripts

Raw Claude Code session transcripts, one `<session-uuid>.jsonl` per session;
the directory name is the cwd with `/` → `-` (22 such dirs, one per workspace).
~6,940 files / 1.7 GB; oldest mtime 2026-08-09 → rolling ~30-day cleanup
(Claude Code default; no `cleanupPeriodDays` override in `~/.claude/settings.json`).
Anything older than ~30 days is gone for good — copy it out if an investigation
needs it.

## 4. Adjacent stores (crash triage)

- `~/.needle/state/predispatch/` — one tiny JSON per dispatch attempt
  (`<worker-id>-<bead_id>.json`, ~2,800 files spanning 2026-08-12 → now)
- `~/.needle/state/heartbeats/` — per-scope heartbeat + `-circuit-breaker.txt`
- `~/.needle/state/pulse/` — per-worker JSON
- `~/.needle/state/token-history.{db,jsonl}` — token/billing accounting (~150 MB)
- `~/.needle/snapshots/` — heap snapshots (manual capture, 172 MB)
- `~/.needle/fabric.db` — pruner's ingest store (75 MB, mtime 2026-08-06)
- System journald — `Storage=persistent`, 4 GB, **no** `MaxRetentionSec` limit.
  Earliest segment 2026-08-15 ~19:51 EDT; nothing before that is recoverable.
  This is where memcg-OOM SIGKILLs of dispatch scopes are recorded.

## Retention — and the broken pruner

Designed policy: `fabric-prune.timer` runs the FABRIC Log Pruner nightly at
23:00 EDT (`node dist/cli.js prune --source ~/.needle/logs`), ingesting logs
into `~/.needle/fabric.db` and archiving/removing what it processed.

Actual state: **broken since 2026-08-17.** The service hardcodes
`WorkingDirectory=/home/coding/FABRIC`, which no longer exists, so every run
dies at `status=200/CHDIR` (first failure 2026-08-17 23:00; last effective run
~2026-08-06, per `fabric.db` mtime). Consequences:

- The **dated and legacy families** (the ~40k files — one per dispatch, the
  main crash-forensics source) have had **zero deletion since Aug 6**: 7.0 GB
  and growing at ~6,200 files/day. The pre-commit/10MB hook and repo-health
  checks do not cover this directory (it is outside any repo).
- The **current `.agent.jsonl` family is the exception** — `mend`
  (`mend.max_log_files: 100` in `~/.needle/config.yaml`) prunes it on a
  rolling basis, so per-attempt files under that naming disappear within days;
  capture them early if an investigation needs one.

Fixing the pruner is an operator decision (the unit lives outside this repo and
points at a deleted checkout); until then, expect the directory to keep growing
and do not treat "logs from last week are gone" as true — everything since
Aug 11 is still on disk.
