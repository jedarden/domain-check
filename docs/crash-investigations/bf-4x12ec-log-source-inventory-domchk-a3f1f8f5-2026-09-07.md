# bf-4x12ec — Surviving Log-Source Inventory (2026-09-07)

Investigator bead: **domchk-a3f1f8f5** (child 1 of 4 of the `domchk-2ff261ce`
split) · Crash bead: **bf-4x12ec** · Worker `claude-code-glm-4.7-lab-domain-check`
· Needle session `a6dbb1fc` · Named crash instant
2026-08-14T10:23:11.219513632Z (the crash-#1 `HANDLING_RELEASE_DONE` heartbeat;
the kill itself is 10:23:02.958717335Z — see the bundles' timestamp
reconciliation tables).

This is the log-source inventory the bead was tasked to produce. It does not
re-derive the crash narrative — that is the canonical report's job — it answers
three questions for every source: where is it, what window does it cover, and
is it still recoverable. Every existence/coverage claim below was re-verified
live on this box on **2026-09-07**.

## 1. Findings new to the corpus

1. **All 53 attempt transcripts survive — not just the 5 previously captured.**
   The two existing bundles (domchk-48f3e34d, domchk-4bad8e94) committed four
   and one session transcripts respectively. A first-line-tag census of
   `~/.claude/projects/-home-coding-domain-check/*.jsonl` dated 2026-08-14
   finds **53 `bf-4x12ec`-tagged transcripts** (8,236,994 bytes total;
   17,357–197,250 bytes each), one per attempt — none had been committed
   outside those five. They are now captured verbatim in
   [`docs/crashes/bf-4x12ec/transcripts/`](../crashes/bf-4x12ec/transcripts/census.tsv)
   (53 files, `cp -p`, all `cmp`-verified byte-identical to their sources, with
   `census.tsv` + `MANIFEST.sha256`). Until 2026-09-07 a `~/.claude` retention
   event would have silently destroyed 48 of the 53 primary transcripts with
   no record that they had existed.
2. **The mid-tool-call death shape is now proven for every crash attempt, not
   just the sampled ones.** Per-attempt last-conversation-record analysis of
   the 53 transcripts: **all 44 exit −1 attempts end in an assistant `tool_use`
   with no matching `tool_result`** — 43 end at
   `Bash: git gc --aggressive --prune=now`, and attempt 8 ends at
   `Bash: echo "Starting aggressive git garbage collection at $(date)" && git gc …`
   (the same operation behind an echo header). The 8 exit-124 attempts (45–52)
   contain **only the dispatch text** (1 conversation record, role `user` —
   zero assistant output in 600 s), consistent with the timeout phase; attempt
   53 ends with assistant `text` at 12:58:44.943Z, 0.17 s before its
   `exit_code=0`. Previously this shape was documented only for attempts 1, 2,
   15, 49 and 53.

## 2. Sources found (all re-verified live 2026-09-07)

| # | Source | Path | Coverage (UTC) | Size / extent | Notes |
|---|--------|------|----------------|---------------|-------|
| 1 | **Needle worker log, Aug-14 (primary)** | `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl` | 2026-08-14 00:00:03.070 → 23:46:16.665 (full day) | 2,598,910 B / 10,138 lines | mtime still 2026-08-14 19:50 EDT — untouched since the crash. Carries all 1,146 bf-4x12ec records (seq 1729–3345): 53 × claim/dispatch/transform/`agent.completed`/`outcome.classified`/`outcome.handled`, exit −1 ×44, 124 ×8, 0 ×1, plus `verification.passed`, `bead.orphaned`, 1 `worker.handling.timeout` |
| 2 | Five same-day sibling worker logs (isolation cross-check) | `~/.needle/logs/claude-code-glm-4.7-lab-{drawrace,roam-1,roam-2,s1,test-fix}-2026-08-14.jsonl` | same day | 0.89–1.58 MB each | 0 bf-4x12ec events in four of them; roam-1 has 1 unrelated housekeeping record (per the crashes-bundle README) |
| 3 | **Agent session transcripts, 53 of 53 attempts** | `~/.claude/projects/-home-coding-domain-check/<uuid>.jsonl` | enqueue 10:21:07.995 → 12:50:34.279; completions 10:23:02.958 → 12:58:44 | 8.24 MB total | Now also captured: [`docs/crashes/bf-4x12ec/transcripts/`](../crashes/bf-4x12ec/census.tsv) — `transcript-attemptNN-<session8>.jsonl` + `census.tsv` (per-attempt session uuid, bytes, sha256, enqueue/completed, exit, classification, conversation-record count, last record) + `MANIFEST.sha256` |
| 4 | Alert-bead records (44) | `.beads/checkpoint/forensic.jsonl` (read-only grep) | created 10:23:14.244 → 11:28:04.917 (first `bf-fmg2cw`, last `bf-5x69lm`) | 44 issue records | Captured verbatim as the evidence bundle's `alert-beads-raw.jsonl` + derived `alert-beads-exit-timestamps.txt` |
| 5 | Live bead record | `bead show bf-4x12ec` | closed 2026-08-17 14:50:41Z | — | Carries the final metrics in its notes (`.git` 753 MB / 141 loose) |
| 6 | Committed bundle A | `docs/crash-investigations/evidence/bf-4x12ec/crash-logs/` (domchk-48f3e34d) | extracted 2026-09-07 | 10 files | Worker-log event extract + crash-window segment, 4 transcripts, 44 alert-bead records, derived exit-code timeline |
| 7 | Committed bundle B | `docs/crashes/bf-4x12ec/` (domchk-4bad8e94) | extracted/re-verified 2026-09-07 | 7 files | `attempt-index.tsv` (all 53 attempts → source line numbers), attempt-2 bracket (raw lines 4411–4473) + transcript, full 1,146-record extract (gz) |

Sources 1–4 are primaries; 5–7 are derived/committed records. The two bundles
and this bead's `transcripts/` copy are mutually consistent: both extractions
pulled exactly 1,146 worker-log records, and this census's exit distribution
(44 × −1, 8 × 124, 1 × 0) matches `attempt-index.tsv` independently.

## 3. Sources checked and NOT recoverable (re-verified 2026-09-07)

| Location | Result | Why |
|---|---|---|
| Kernel / systemd journal for 2026-08-14 | **Gone** | `journalctl --list-boots` shows a single boot: **first entry 2026-08-15 19:56:33 EDT** (last entry 2026-09-07 19:49 EDT — no reboot since). Nothing from Aug 14 survives, so the memcg mechanism stays regime-matched, not kernel-proven, for this bead. **Figure note:** older docs (the 2026-09-02 artifacts survey, the final report, the evidence-bundle README) state the boot began "19:26 EDT"; today's `--list-boots` query returns 19:56:33 EDT — the value the crashes-bundle README already records. Cite 19:56:33; treat 19:26 as a stale earlier reading |
| `.beads/logs/` monitor logs | **No Aug-14 coverage** | Earliest entries anywhere: `repo-health.log` 2026-09-01 10:43 EDT, `resource-metrics.log` 2026-09-01 22:49Z, `crash-pattern-alerts.log` 2026-09-02 02:11Z; `crash-monitor.log` / `resource-monitor.log` / `service-monitor.log` begin with an argv error line, no timestamped content. The monitors postdate the crash by 18 days — this answers the task's `.beads/logs/` step by absence |
| `.beads/traces/` | **No bf-4x12ec slot** | Traces are single-slot per dispatch and the Aug-14 slots were reclaimed (531 `bf-*` slots, none this bead) |
| `.git/gc.log`, git reflog, the Aug-14 pack | Absent / expired / rewritten | No failed-gc residue; reflog has nothing from Aug 14–15; the current pack postdates the cleanup (scheduled maintenance) |
| Needle log "oom"/memory events | **None exist** | The worker log records exit codes and heartbeats only — the kernel-level kill left no needle-side trace |
| Core dumps / stack traces | None | SIGKILL-style cgroup kills leave neither |

## 4. Representative excerpts (timestamps UTC, from the live sources)

Per-attempt tails are captured exhaustively in
[`transcripts/census.tsv`](../crashes/bf-4x12ec/transcripts/census.tsv); the
rows below are the shape of each phase. "Tail" = the last conversation record
in the transcript.

| Attempt | Session | Enqueue | Completed (exit) | Tail |
|---|---|---|---|---|
| 1 | `8b2a5b0d` | 10:21:07.995 | 10:23:02.958 (−1, 115.8 s) | assistant `tool_use` `Bash: git gc --aggressive --prune=now` @ **10:22:36.260Z** — no result returned |
| 8 | `1bde2d04` | 10:32:28.712 | 10:33:11.442 (−1, 42.7 s) | assistant `tool_use` `Bash: echo "Starting aggressive git garbage collection at $(date)" && git gc …` @ 10:32:50.279Z |
| 15 | `9539f3b2` | 10:43:40.804 | 10:44:53.202 (−1, 72.4 s) | the mid-storm transcript: captured `df -h` (85 % used / 67 G free) and `free -h` (**45 Gi available, 0 B swap**) @ 10:43:58–59Z, then launched gc and was killed |
| 44 | `8dc25b31` | 11:25:59.764 | 11:27:26.173 (−1, 86.4 s) | assistant `tool_use` `git gc --aggressive --prune=now` @ **11:26:49.882Z** — the last crash of phase 1 |
| 45 | `eb56d2d7` | 11:28:08.818 | 11:38:07.866 (**124**, exactly 600 s) | user dispatch text only (`pluck` template) — zero assistant records |
| 49 | `c48ec3f3` | 12:09:21.949 | 12:19:21.091 (**124**, 600 s) | user dispatch text only (`split` template "Auto-Split: Decompose This Bead") |
| 53 | `31800ee3` | 12:50:34.279 | 12:58:45.113 (**0**, 491.8 s) | assistant `text` @ 12:58:44.943Z — the split into bf-173o7e / bf-5jhvpk / bf-im2sl1; `verification.passed` 11 ms later |

The needle-side counterpart of every row above is in
`docs/crashes/bf-4x12ec/attempt-index.tsv` (source-line-numbered) and the raw
event extracts in both bundles.

## 5. Verification performed (this bead, 2026-09-07)

- 53/53 captured transcripts `cmp`-identical to their `~/.claude` sources; per-file sha256 in `transcripts/MANIFEST.sha256` (largest file 197,250 B — far under the 10 MB pre-commit gate).
- `docs/crashes/bf-4x12ec/MANIFEST.sha256` passes (`sha256sum -c`, 6/6 OK).
- Census exit distribution (44/8/1) re-derived from the transcripts themselves matches `attempt-index.tsv`, which domchk-4bad8e94 had verified against the worker log line-by-line.
- Transcript↔attempt mapping derived by matching each transcript's first-line enqueue timestamp to `dispatch_ts` (enqueue follows dispatch by ~1 s, monotonic across all 53) — no attempt is unmatched or doubled.
- `journalctl --list-boots`, `.beads/logs/` earliest-entry scan, and the needle-log first/last event timestamps re-run on 2026-09-07 (§2–§3 figures are from those runs, not from older docs).

## 6. Scope of the split this bead belongs to

`domchk-2ff261ce` ("Gather crash artifacts for bead bf-4x12ec") was split into
four children; this bead is child 1 (log sources). For the record, the other
three children's scopes are already served by committed documents: the crash
timeline and bead context (child 2) by
[`bf-4x12ec-crash-artifacts-2026-09-02.md`](bf-4x12ec-crash-artifacts-2026-09-02.md)
§2/§6 and the canonical report's timeline; system-level evidence (child 3) by
the artifacts doc §4 and the final report's "System State at Crash"; the
artifacts-summary file (child 4) by the artifacts doc §3 plus the two
evidence bundles. This document adds the one thing none of them had: the
complete surviving-transcript census and its capture.

## Related records

- [`bf-4x12ec-final-crash-report.md`](bf-4x12ec-final-crash-report.md) — consolidated report (domchk-8c3fceeb)
- [`bf-4x12ec-crash-artifacts-2026-09-02.md`](bf-4x12ec-crash-artifacts-2026-09-02.md) — the umbrella's own artifact survey
- [`bf-4x12ec-log-review-2026-09-02.md`](bf-4x12ec-log-review-2026-09-02.md) — independent event-log re-derivation
- `docs/crashes/bf-4x12ec/README.md`, `docs/crash-investigations/evidence/bf-4x12ec/crash-logs/README.md` — the two extraction bundles

---
**Inventory date:** 2026-09-07 · Bead `domchk-a3f1f8f5` · Confidence HIGH —
every path listed was stat'ed/read live on this box on the date above; nothing
here is cited from an earlier document without re-verification.
