# Crash log findings summary — bf-2ildm (trace analysis, relayed from domchk-a5a51981)

Assembled 2026-09-07 by **domchk-167289ae**, the summary bead for
**domchk-a5a51981** ("Extract and analyze crash logs for bf-2ildm", Closed
rev 4, findings dated 2026-09-02; itself a child of alert bead **bf-o6vbwl**).
Like its sibling context relay
[`docs/crash-context-bf-2ildm-2026-08-13.md`](crash-context-bf-2ildm-2026-08-13.md)
(domchk-fe9f7c5b, relaying domchk-b049ea9e), this is **not a new
investigation**: it extracts what domchk-a5a51981 found about the crash logs
and the trace, re-verifies every load-bearing figure first-hand against the
artifacts still on disk, and marks the conclusions that records newer than
2026-09-02 have superseded.

Companion records, all newer than the findings summarized here:

- Current root-cause record:
  [`docs/investigations/bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md`](investigations/bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md)
- Per-attempt evidence bundle: `docs/crashes/bf-2ildm/` (domchk-ea755548)
- Classification docs: `docs/crash-classification-bf-2ildm-2026-08-13-14-40.md`,
  `docs/crash-classification-bf-2ildm-2026-08-13-15-01.md`

**Bottom line — read the FALSE POSITIVE label at the level where it is true.**
Every *observation* domchk-a5a51981 recorded about the surviving trace
reproduces exactly (exit 0, outcome `success`, 85,327 ms, no crash artifacts in
the bundle, the four-child split all visible in `trace.jsonl`). Two of its
*inferences* do not survive: "no crash occurred" and "the reported exit −1 was
placeholder data." The −1s were 38 real kills on 2026-08-13, and the trace slot
simply holds a later attempt — the single-slot retention blind spot documented
in the superseding RCA. The alert-level FALSE_POSITIVE verdict stands: each
alert fired at a real kill but implied lost work, and no work was lost.

## 1. Exit code analysis — reported −1 vs actual 0

**As recorded by domchk-a5a51981 (2026-09-02):**

| Field | Crash alert says | Trace metadata says | Child bead's verdict |
|---|---|---|---|
| Exit code | −1 | **0** | reported FALSE, trace TRUE |
| Outcome | crash (signal −1) | **`success`** | reported FALSE, trace TRUE |
| Duration | — | **85,327 ms (85.3 s)** — reasonable for the task | — |

**Re-verified live 2026-09-07** — `.beads/traces/bf-2ildm/metadata.json` reads
exactly as summarized: `exit_code: 0`, `outcome: "success"`,
`duration_ms: 85327`, `captured_at: 2026-08-16T22:28:44.172164374Z`,
`agent: claude-code-glm-4.7` (provider `zai`, model `glm-4.7`),
`trace_format: claude_json`, `pruned: false`, `timeout_reason: null`. The
retrieval bundle's copy (`docs/crashes/bf-2ildm/trace-archive-current-state.json`)
is byte-identical to the live slot (`cmp` clean), as is its stderr copy.

**What the contrast actually means (superseded framing).** The child bead read
"alert −1 vs trace 0" as fabrication next to ground truth. Both numbers are
real records of *different attempts*: the trace archive is **single-slot** —
the last attempt overwrites all prior ones — so it holds only the successful
2026-08-16 retry, while the −1s are the sentinel values of **38 real kills**
during the 2026-08-13 storm (13:37:24Z → 15:53:28Z,
`docs/crashes/bf-2ildm/attempt-index.tsv`: 43 attempts = 38 × exit −1,
4 × exit 124 at the 600 s cap, 1 × exit 1 at 19 ms quarantined at
`failure_count: 5`). Reading the surviving record as the record *of the
reported crash* was the 09-02 corpus's central evidence-selection error
(superseding RCA §4).

## 2. Crash log search results — NONE found, and what that proves

**As recorded:** no core dumps, no crash log files, no stack traces, no
exception information — concluded "because no crash occurred."

**Re-verified live 2026-09-07:** true, and still true today — the trace bundle
holds four files (`metadata.json` 396 B, `stderr.txt` 457 B, `stdout.txt`
885 KB, `trace.jsonl` 15 KB) and none contains a kill signature.

**Why it does not mean no crash occurred.** The search domain could not have
held the Aug-13 evidence for three documented retention reasons:

1. The single-slot trace archive was overwritten by the successful Aug-16 run
   — the very file the child bead examined.
2. journald on this host has a **single boot beginning 2026-08-15 19:56:33
   EDT**, so no Aug-13 kernel OOM line survives for this bead (or any sibling).
3. `.beads/logs/` telemetry begins 2026-09-02; nothing existed on Aug-13 to
   collect crash events.

"NONE found" is an evidence-retention limit, not a negative result. The crash
record that does exist for the killed attempts is the per-attempt index
assembled 2026-09-07 (`docs/crashes/bf-2ildm/attempt-index.tsv` — 43 rows, all
fields from needle's own event log), not the trace archive.

## 3. Trace file analysis

### metadata.json — confirms successful completion (verified)

Quoted in full in §1. Definitive *for the final attempt*: exit 0, `success`,
85,327 ms, captured 2026-08-16T22:28:44Z. Together with the target's close
record (2026-08-16T22:44:38.873Z, rev 7 — checkpoint-verified) it proves the
load-bearing point on which the child bead was right: **the work was not
lost**.

### stderr.txt — only minor warnings, no errors (verified)

Three lines, none crash-shaped:

1. `Running as unit: run-p3830620-i213518973.scope; invocation ID: e44889af…`
   — normal systemd invocation banner.
2. `⚠ claude.ai connectors are disabled because ANTHROPIC_API_KEY or another
   auth source is set and takes precedence over your claude.ai login` —
   expected environment warning.
3. `SessionEnd hook [/home/coding/.ccdash/hooks/session-end.sh] failed: …
   cannot execute: required file not found` — harmless missing hook.

No OOM, no SIGKILL, no stack trace, no exception text.

### trace.jsonl — full execution trace of the successful split (verified)

- **47 events**: 11 `agent_message`, 18 `tool_call`, 18 `tool_result` — every
  call has a result; exactly **one** result is a failure, and it is a benign
  usage error (`bead dep show` → "unrecognized subcommand 'show'", exit 2) not
  a crash.
- **Span**: first ts 1786919247.884 → last 1786919324.109 = 76.2 s of recorded
  activity against metadata's 85,327 ms end-to-end duration.
- **What the run did** — this is the run that performed the split the child
  bead described:
  - opened `bead show bf-2ildm` (target InProgress, rev 2);
  - created 4 children: `domchk-127bb100` (find common ancestor),
    `domchk-38e09d92` (extract GitHub-specific commit list),
    `domchk-0671a466` (parse and capture commit details),
    `domchk-cabae852` (save commit data to state file);
  - wired the chain `domchk-127bb100 → domchk-38e09d92 → domchk-0671a466 →
    domchk-cabae852` via `bead dep add … --kind blocks`, plus
    `bf-2ildm` blocked-by `domchk-cabae852`;
  - `bead label add bf-2ildm --label umbrella`;
  - verified with `bead show` / `bead dep add` / `bead list` calls, and closed
    with an agent message: *"SPLIT_COMPLETE: Created 4 children, parent
    converted to umbrella."*

**Wording precision:** the child bead's "parent converted to umbrella *type*"
was a **label** addition (`bead label add`), not an issue-type conversion; the
child bead's description of the trace contents is otherwise accurate as
recorded. (The same `bead dep show` usage error appears in this summary's own
session log — it is not a valid bead-rs subcommand, which is why the split's
verification used `bead dep add` idempotently instead.)

## 4. The "signal −1" explanation

**As recorded:** "Signal −1 was placeholder data in the crash alert generation
system, never validated against actual trace metadata."

**Superseded.** Exit −1 is needle's sentinel for a worker that died without a
recorded exit code — `code().unwrap_or(-1)` — not a signal number; a true
SIGKILL encodes 137. The alert's "(signal −1)" echoes that sentinel. The 38
Aug-13 alerts each carried the sentinel of a real kill, and each alert stamp
sits 7.7–29.4 s after its kill (`attempt-index.tsv`; re-derived live: 38
exit-−1 rows, 38 distinct alert beads, zero duplicates). The kill mechanism —
memcg-OOM inside needle's 12 GiB per-dispatch scope under the repository-bloat
regime — is assigned **by regime match**, because no Aug-13 kernel record
survives (§2); it is the same regime as same-day siblings (Aug-13 census:
bf-65lsdu 127, bf-1ea4g 56, bf-4k2ws 55, bf-2ildm 38, bf-1s6c3 22).

## 5. Timestamp analysis — the "alert 3+ days BEFORE completion (impossible)" argument

**As recorded:** the child bead listed timestamp analysis as HIGH-quality
evidence — "alert generated 3+ days BEFORE completion (impossible)."

**Superseded, with the ordinals corrected.** The numbers it compared were real
but misordered:

| Event | Instant (UTC) |
|---|---|
| Attempt 19 claim | 2026-08-13T14:38:24.455Z |
| Attempt 19 **kill** | 2026-08-13T14:40:29.551Z (exit −1, 124,759 ms, mid-task) |
| Crash-handler heartbeat (= the alert's carried stamp) | 2026-08-13T14:40:42.628Z (**+13.1 s**) |
| Alert bead bf-66sw7c created | 2026-08-13T14:40:42.642Z (+14 ms after the stamp) |
| Target **closes** successfully | 2026-08-16T22:44:38.873Z |

An alert generated at a kill, while the bead is open and mid-task, is exactly
what the alert system exists to produce. Nothing was generated "3+ days before
completion"; the alert was generated 3 days **before the completion that later
made it stale**. The same reversal corrects the 2026-08-26 verification
report's opposite claim of "generated after completion" (both directions
corrected in the superseding RCA §3–4 and the classification doc §5). Alert
stamps are handler heartbeats, never the kill instant — cite kills from the
per-attempt index.

## 6. Evidence classification and conclusion

**The child bead's evidence classes, re-graded as of 2026-09-07:**

| domchk-a5a51981 evidence item | Its grade then | Stands? | Why |
|---|---|---|---|
| Trace metadata: exit 0, success | "Definitive" | **Yes, at its true scope** — definitive *for the final attempt*, proving work not lost | `metadata.json` reproduces exactly; single-slot caveat documented |
| Bead status CLOSED | "Definitive" | **Yes** | close 2026-08-16T22:44:38.873Z checkpoint-verified; rev 7 |
| Timestamp analysis ("impossible") | "Definitive" | **No — inverted** | §5; alert-while-open is correct behavior |
| Crash logs NONE ⇒ no crash | "Definitive" | **No** | §2; retention gap, and the attempt index records 38 kills |
| Trace shows the 4-child split | — | **Yes** | §3; re-derived from `trace.jsonl` tool calls |

**Two-level verdict (matching the classification chain and the store's
2026-09-07 appendix on bf-2ildm itself):**

| Level | Verdict | Confidence |
|---|---|---|
| Kill level — did bf-2ildm crash? | **INFRASTRUCTURE** — 38 real exit-−1 kills, repository-bloat memcg-OOM regime (mechanism MODERATE: assigned by regime match, no surviving Aug-13 kernel record) | regime HIGH |
| Alert level — was the alert's implication right? | **FALSE_POSITIVE** — fired at real kills, but implied lost work; none was lost. Dedup gate agrees: `alert-deduplication.sh check bf-66sw7c` → DUPLICATE, target resolved | HIGH |

**What survives from domchk-a5a51981:** the task-completion finding (target
closed, all acceptance criteria met, work intact), the accurate trace-file
descriptions (§3), and its pointer to the alert-layer defects — the five it
listed (premature alerting, unvalidated exit code, no closed-bead filtering, no
timestamp validation, no duplicate prevention) were real alert-layer defects
regardless of the kill question, and the 2026-09-02 fixes stand
(`test-crash-alert-fixes.sh` 13/13, `test-closed-bead-filter.sh` 7/7,
re-verified 2026-09-07).

**What is superseded:** "NO CRASH OCCURRED," the placeholder-exit-code
explanation, the physical-impossibility timing argument, and the inference
that an empty crash-log search is exculpatory. The corrected record lives on
bf-2ildm's own notes (rev 7, dated classification appendix by domchk-e05fa5a8),
which states the correction directly: *"the section above's 'No actual crash
occurred' is corrected: the kills were real."*

**Chain state at writing:** parent alert bead **bf-o6vbwl** (rev 28) is still
**Open** — this summary closes the extraction child's obligation, not the
parent's.

## 7. First-hand verification battery (run 2026-09-07 for this summary)

| Check | Command | Result |
|---|---|---|
| Source findings | `bead show domchk-a5a51981` | Closed rev 4; findings as summarized above |
| Target state | `bead show bf-2ildm` + checkpoint `forensic.jsonl` | Closed rev 7; close 2026-08-16T22:44:38.873Z; 2026-09-07 correction appendix present |
| Trace survives | `ls .beads/traces/bf-2ildm/` | 4 files, mtime Aug 16 18:28 (local) |
| Exit code / outcome / duration | read `metadata.json` | 0 / `success` / 85,327 ms, captured 2026-08-16T22:28:44Z |
| stderr content | read `stderr.txt` | 3 benign warnings, no kill signature |
| Trace event census | parse `trace.jsonl` | 47 events (11/18/18), 1 benign failed result, 76.2 s span |
| Split narrative | extract tool calls | 4 children created + chained + umbrella label + SPLIT_COMPLETE |
| Bundle ↔ live trace | `cmp` bundle copies vs live slot | byte-identical (metadata + stderr) |
| Attempt census | `awk` over `attempt-index.tsv` | 43 rows: 38 × −1, 4 × 124, 1 × 1; 38 distinct alert beads, stamp gaps 7.7–29.4 s |
| Alert-ledger rows | `wc -l crash-alert-ledger.tsv` | 38 alerts, 1:1 with kills |
| Alert ordinals | rows 19 / 20 / 24 / 43 | match superseding RCA §3 (attempt 19 = bf-66sw7c, +13.1 s) |
| Cited docs exist | path checks | `docs/investigations/bf-2ildm-final-investigation-report-2026-09-02.md` ✅; `docs/investigations/root-cause-determination-bf-2ildm-2026-09-02.md` ✅ — the child bead's citation `docs/root-cause-determination-bf-2ildm-2026-09-02.md` (no `investigations/`) is a stale path |

## 8. Sources

- domchk-a5a51981 notes (`bead show`, rev 4, dated 2026-09-02) — the findings
  extracted here
- `.beads/traces/bf-2ildm/` — `metadata.json`, `stderr.txt`, `trace.jsonl`,
  `stdout.txt` (read live 2026-09-07; also mirrored in
  `docs/crashes/bf-2ildm/trace-archive-current-*`)
- `docs/crashes/bf-2ildm/` retrieval bundle (domchk-ea755548):
  `attempt-index.tsv`, `crash-alert-ledger.tsv`, `README.md`
- `docs/investigations/bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md`
  (superseding RCA — §2 kill mechanism, §3 alert-level root cause and ordinal
  correction, §4 supersession of the 09-02 determination)
- `docs/crash-context-bf-2ildm-2026-08-13.md` (sibling context relay,
  domchk-fe9f7c5b)
- `docs/crash-classification-bf-2ildm-2026-08-13-14-40.md` (d82b6a2),
  `docs/crash-classification-bf-2ildm-2026-08-13-15-01.md` (e3a8820)
- Live store: `bead show bf-2ildm` (rev 7, correction appendix),
  `bead show bf-o6vbwl` (Open, rev 28), checkpoint `forensic.jsonl`
