# Crash artifact bundle — bf-1ea4g attempt 30 (2026-08-13)

Investigation directory for the crash named as **bead bf-1ea4g, exit code −1,
timestamp 2026-08-13T08:23:51.806516385+00:00** — the timestamp embedded in
alert bead **bf-1nb5u** ("ALERT: Agent crash on bead bf-1ea4g", created
2026-08-13T08:23:51.812870634Z). Assembled 2026-09-07 by **domchk-93ac565b**
(collection child of bf-1nb5u's 2026-09-02 split; two earlier dispatches of the
collection bead died before delivering, this one completes it).

## Timestamp resolution — the named instant is not the kill

| Event | Timestamp (UTC) | Source |
|---|---|---|
| Attempt-30 dispatch | 08:22:12.663463008Z | worker log line 3320 |
| Kill: `agent.completed` `exit_code: -1` (duration 92,079 ms) | **08:23:44.918359819Z** | worker log line 3323 |
| `outcome.classified` → crash | 08:23:44.919209399Z | line 3326 |
| `HANDLING_RELEASE_DONE` heartbeat — worker log copy | 08:23:51.806509194Z | line 3332 |
| **`HANDLING_RELEASE_DONE` heartbeat — alert payload copy = the named instant** | **08:23:51.806516385+00:00** | bf-1nb5u description (worker-log copy differs by 7 µs; the alert-path intermediary record for Aug-13 is unrecoverable — journald absent) |
| `bead.released` (`release_success`) | 08:23:54.063149663Z | line 3333 |
| `outcome.handled` `action: alerted` → bf-1nb5u created | 08:23:54.063156382Z / 08:23:51.812870634Z | line 3334 / bead store |

This is the standard alert-stamp pattern in this fleet: the bead-carried
"crash timestamp" is the post-kill handling heartbeat, ~6.9 s after the actual
kill. The kill is **attempt 30 of 57** dispatches of bf-1ea4g on Aug-13
(56 × exit −1 from 07:17:49Z to 09:08:30Z, then 1 × exit 0 at 09:08:39Z →
bead closed 09:10:16Z). The attempt numbering in `attempt-index.tsv` was
derived independently by this bundle and by sibling domchk-e2065699's trace
reconstruction; attempt instants 10, 30 and 56 agree byte-exact.

## Artifact inventory

| File | What it is | Provenance |
|---|---|---|
| `attempt-index.tsv` | All 57 attempts: dispatch/completion timestamps, exit codes, durations, source line numbers | Extracted from `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` (3,111,314 bytes, mtime 2026-08-13 19:59 local) |
| `needle-events-2026-08-13-bf-1ea4g-attempt30.jsonl` | Raw event bracket for the target crash: attempt-29 claim (line 3283, 08:20:37.420Z) → attempt-31 release (line 3361, 08:25:10.765Z), verbatim lines | Same source, lines 3283–3361 |
| `needle-events-2026-08-13-bf-1ea4g.jsonl.gz` | Every bf-1ea4g record in the Aug-13 worker log (1,093 records: 465 heartbeats, 114 routing decisions, 57× claim/dispatch/transform/completed/classified/handled, 56 released, 1 verification.passed, 1 bead.completed) | Same source, filtered |
| `session-transcript-attempt30-449405f5.jsonl` | The attempt-30 agent session transcript, verbatim (187,006 bytes, 40 records) | `~/.claude/projects/-home-coding-domain-check/449405f5-1fc7-4766-82fe-20dc0fd400b2.jsonl` (mtime 2026-08-13T08:23:44.770582Z) |
| `system-state-2026-08-13T082344Z.md` | CPU/load evidence + documented absences (memory, disk, journald, coredump) | Derived from the same worker log; absence claims cross-checked live 2026-09-07 |
| `MANIFEST.sha256` | Content hashes of every file above | — |

The durable originals remain in place at the paths listed above; the bundle
copies exist so analysis never depends on `~/.needle` or `~/.claude` retention.

## Evidence checklist

- [x] **Crash logs identified and copied** — worker-event bracket + full-bead
      extract + agent session transcript (see inventory). Source file, record
      line numbers, and original mtimes are recorded here and in
      `system-state-2026-08-13T082344Z.md`.
- [x] **System state at crash time documented** — load average recorded
      directly (10.80 at dispatch, 9.86 twelve seconds after the kill; box
      saturated for the entire 07:00–10:00Z window, storm peak 19.87/9 cores).
      Memory and disk were **not recorded** anywhere on this box for Aug-13 —
      the gap itself is documented with the collector start dates that prove
      it.
- [x] **Crash dump location confirmed (absence noted)** — no coredump
      infrastructure before Aug-25; `exit_code −1` is needle's
      died-without-exit-code sentinel (not a signal number); no panic/stack
      trace in any record. The two "panic" strings in the transcript are fuzz
      test comments embedded in the dispatch prompt, not crash output.
- [x] **All artifacts preserved with timestamps** — every extract is verbatim
      (original in-band timestamps), source line numbers recorded in
      `attempt-index.tsv`, original file mtimes recorded here and in the
      system-state doc, content pinned by `MANIFEST.sha256`. Collection ran
      2026-09-07 — 25 days after the crash; all sources are append-only logs,
      so the delay does not affect fidelity.
- [x] **Agent session transcript near crash time documented** — attempt 30's
      transcript survives and brackets the kill: first record 08:22:13.644Z
      (needle dispatch tag `[needle:claude-code-glm-4.7-lab-domain-check:bf-1ea4g:auto]`),
      last assistant record 08:23:31.123Z — a `tool_use` with **no** matching
      result, i.e. death mid-tool-call ≥13.8 s before the recorded kill
      instant. **The kill is mid-task**, not post-completion cleanup. (The
      bead itself completed under attempt 57 at 09:10:16Z; deliverable
      `main_branch_state_bf-1ea4g.json` / `.beads/local-main-state-bf-1ea4g.json`.)

## Corpus discrepancies this bundle resolves for this instant

Recorded so downstream analysis does not inherit them (this bundle takes no
side in mechanism classification):

1. The Sep-2 corpus (`docs/investigations/bf-1ea4g-root-cause-determination-2026-09-02.md`,
   `docs/archive/crash-investigations/final-resolution-bf-1ea4g-2026-09-02.md`)
   classified bf-1ea4g crashes FALSE POSITIVE partly on "repository healthy →
   not OOM from repository bloat". That premise is anachronistic for Aug-13:
   the documented bloat era (bf-1s6c3 / bf-4yjq, ~18 GB `.git`, 17 GB loose
   objects) spans Aug-12 → mid-August and was only repaired 2026-09-01.
2. Verification-report titles for this alert ("OOM after task completion,
   repo cleaned") are contradicted for this specific instant: the attempt-30
   transcript ends mid-tool-call, 13.8 s before the kill and 47 min before the
   bead closed.
3. The alert-stamp timestamp (08:23:51.8065…) is repeatedly quoted as the
   crash time; the kill is 08:23:44.918359819Z (table above).

---

## Artifact fidelity re-verification — 2026-09-07 (domchk-ea755548)

Collection child of the same 2026-09-02 split, cut from alert bead
**bf-otbk6** (created 2026-08-13T08:45:05.914Z — the 08:45:05 instant, the
other unresolved kill stamp in this family). Task scope is retrieve and
organize crash data only; classification is owned by domchk-6b123791.

**Retrieval result: the bundle above already satisfies every acceptance
criterion this bead carries.** Rather than re-collect it, this dispatch
re-derived the data from the primary source and verified the bundle against
it. Nothing was re-collected and no file below was altered except this README
(and its hash, re-pinned in `MANIFEST.sha256`).

### Crash metadata (re-extracted first-hand 2026-09-07)

| Field | Value |
|---|---|
| Target bead | bf-1ea4g — "Document local main branch state" (closed 2026-08-13T09:10:16Z) |
| Alert beads | 56 in the store, one per kill (pre-0.4.2 needle, one-alert-per-kill). This chain's: **bf-otbk6** (08:45:05Z); the bundle's original target bf-1nb5u (08:23:51Z) is resolved by the table above |
| Needle worker | `claude-code-glm-4.7-lab-domain-check` |
| Session ID | `8446529e` |
| Primary crash log | `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` — 3,111,314 bytes, 12,131 lines, **1,093 bf-1ea4g records** |
| Attempts | 57 — first claim 07:17:49.928Z, last record 09:10:43.080Z |
| Exit codes | **56 × `-1`, 1 × `0`** (attempt 57: dispatch 09:08:39.095Z, completed 09:10:39.562Z, 120,347 ms) |
| `outcome.classified` | 56 × `crash`, 1 × `success` |
| `outcome.handled` | 56 × `alerted`, 1 × `none` (the successful attempt) |
| `bead.released` | 56 × `release_success`; attempt 57 has no release record — the bead closed |
| `.beads/traces/` | **No bf-1ea4g slot survives.** Traces are single-slot per dispatch and the Aug-13 slots were reclaimed; the worker log above is the only surviving crash log for this bead. 525 `bf-*` slot dirs remain, none for bf-1ea4g |

### Verification performed against the committed bundle

- `MANIFEST.sha256`: **6/6 files OK** (`sha256sum -c`, re-run 2026-09-07).
- `attempt-index.tsv` re-derived independently from the primary worker log and
  compared field-by-field: **57/57 attempts, 0 mismatches** across
  `dispatch_ts`, `completed_ts`, `exit`, `duration_ms`, `classified_ts`,
  `released_ts`, `release_reason`.
- bf-otbk6's named instant `2026-08-13T08:45:05.908593271+00:00` (alert
  payload copy) matches attempt 42's `HANDLING_RELEASE_DONE` heartbeat
  `08:45:05.908552183Z` at worker-log **L3673**, 6.10 s after attempt 42's kill
  (`agent.completed` `exit_code: -1`, 76,159 ms, 08:44:58.908Z). Resolved
  independently here and in
  `docs/crash-context-bf-1ea4g-2026-08-13.md` (domchk-cf6855ad); both agree,
  and the ~7 µs worker-log-vs-alert-payload skew matches the bf-1nb5u pattern
  documented in the table above.

### Corpus discrepancy observed in the live store

Alert bead **bf-otbk6** (still Open, rev 24) carries a Notes claim the
retrieved record contradicts: *"Task bf-1ea4g was completed successfully 8
minutes BEFORE crash occurred."* The record shows the opposite — this kill is
attempt 42 of a loop that ran 25 more minutes before attempt 57 closed the
bead at 09:10:16Z, and attempt 30's transcript ends mid-tool-call 13.8 s
before its kill (table above). Left uncorrected in the store: notes correction
is outside this collection bead's scope and `bead update --notes` replaces
wholesale. Flagged here so the classifier (domchk-6b123791) does not inherit
the premise.
