# bf-4x12ec crash investigation — consolidated record

Reconstructed from primary sources 2026-09-07. Child 1 of the split of
umbrella `domchk-c99cdf80` (bead `domchk-c1c0afd8`); later sections of this
file are appended by the sibling children of that split.

## Original bead context

### Where the record was recovered from

Three independent sources, all read 2026-09-07. They agree on every field they
share; each is noted per fact below.

| Source | What it supplied |
|---|---|
| **Live bead store** — `.beads/beads.db` via `bead show bf-4x12ec` | Authoritative current state: title, type, priority, status, revision, and the completion notes carrying the final cleanup metrics. |
| **Durable checkpoint** — `.beads/checkpoint/forensic.jsonl` (gitignored; read-only grep, never written) | Issue record at line 981 (`created_at`, labels, dependencies, `close_reason`) plus 195 event records mentioning the bead — including seq 2194 `assignment_cleared` (line 5635, carrying `prior_assignee`) and seq 3407 `closed` (line 6848, carrying the close reason) — and the 44 auto-minted `ALERT: Agent crash on bead bf-4x12ec` issue records. |
| **Prior docs** — `docs/crash-investigations/bf-4x12ec-crash-investigation.md`, `docs/crash-investigations/evidence/bf-4x12ec/crash-logs/` (README, `alert-beads-raw.jsonl`, `exit-code-timeline.txt`, the four verbatim attempt transcripts), `docs/signal-analysis-exit-code-negative-one.md` | The needle worker-log event stream (1,146 bf-4x12ec lines from `claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl`), the per-attempt transcripts, verbatim alert-bead records, and the exit-code semantics. |

Two provenance caveats:

- `bead show` displays **no assignee and no labels/dependencies** — the
  assignee is attested by the checkpoint's `assignment_cleared` event and by
  the needle log's `worker_id`, not by any live field.
- The evidence bundle under `docs/crash-investigations/evidence/bf-4x12ec/`
  was committed in 9b32085 (force-added past the repo-wide `*.jsonl` ignore).
  As of this dispatch a co-tenant has staged **uncommitted** `git rm --cached`
  deletions of that bundle in this shared worktree — cite the bundle by commit
  (9b32085), not by working-tree state.

### The record

| Field | Value | Source |
|---|---|---|
| ID | `bf-4x12ec` | live store + checkpoint:981 |
| Title | Execute aggressive git garbage collection to eliminate OOM risk | live store + checkpoint:981 |
| Type | `task` | live store + checkpoint:981 |
| Priority | P2 (numeric 2) | live store + checkpoint:981 |
| Status | Closed, revision 4 | live store; `closed` event seq 3407 |
| Created | 2026-08-14T10:17:26.387856508Z | live store + checkpoint:981 (the investigation doc's "created at 10:17:26Z" matches) |
| Closed | 2026-08-17T14:50:41.544361971Z | live store + checkpoint:981. The work itself completed 2026-08-14T12:58:45Z (`verification.passed`; `bead.orphaned` 12:58:55Z) — the Aug-17 timestamp is closure/notes time, not completion time |
| Assigned agent at crash time | worker **`claude-code-glm-4.7-lab-domain-check`** — adapter `claude-code-glm-4.7` (model glm-4.7), needle session `a6dbb1fc` | needle log `worker_id` on all 1,146 bf-4x12ec event lines; checkpoint event seq 2194 `assignment_cleared` → `prior_assignee: claude-code-glm-4.7-lab-domain-check` (2026-08-17T04:43:12Z); the auto-minted crash alerts describe the agent as `claude-code-glm-4.7`; attempt-1 transcript first-line tag `[needle:claude-code-glm-4.7-lab-domain-check:bf-4x12ec:auto]` |
| Assignee today | empty — cleared at release/close. The store holds no assignment event earlier than 2026-08-17 for this bead (the Aug-14 claims predate the bead-forge → bead-rs migration), so the crash-time assignment rests on the worker log + `prior_assignee`, not on a live assignee field | checkpoint event seq 2194 |
| Labels (today) | `deferred`, `split-child`, `umbrella` — the umbrella/split-child pair is residue of the Aug-14 auto-split and later release cycles (the same zero-children-label shape documented for bf-3dxljn) | live store + checkpoint:981 |
| Dependencies | `bf-im2sl1` —blocks→ `bf-4x12ec` (the verify child). Live 2026-09-07: `bf-173o7e` (gc child) **Closed**; `bf-5jhvpk` (repack) and `bf-im2sl1` (verify) still **Open** — so this Closed bead still carries an open blocker | `bead list --json` (`bead show` hides deps) |
| Close reason | "Git cleanup completed successfully. Repository size reduced from ~18GB to 753MB, loose objects from 4,627 to 141. All git operations working normally without OOM risk. Agent crash was recoverable - cleanup process completed despite process termination." | checkpoint:981 `close_reason` — close reasons appear only in the checkpoint, not in `bead show` |

Description (core verbatim; full text in the live record and checkpoint:981):

> ## Task: Execute Phase 1 emergency stabilization
> Execute aggressive git garbage collection to pack 17.20GB of loose objects
> into compressed pack files, eliminating the OOM risk during git operations.
>
> ### Root Cause
> - Repository contains 17.20GB of loose objects (4,627 loose objects)
> - Git operations on bloated repository trigger OOM killer (SIGKILL)
> - This is Phase 1.2 from the root cause analysis (CRITICAL - NOT YET EXECUTED)

Acceptance criteria (verbatim): `git gc --aggressive --prune=now`;
`git repack -a -d --depth=250 --window=250`; size <500MB (from 18GB); loose
objects <100 (from 4,627); `git fsck --no-full` without timeout;
clone/fetch/checkout without OOM. Outcome per the bead's own completion notes:
gc + repack completed; **753MB / 141 loose objects / 10,265 pack objects in a
750.67 MiB pack** — the size and loose-count targets were recorded PARTIAL and
accepted (both were driven below target by later maintenance).

Figure drift worth knowing: the description's **4,627** is the creation-time
loose count; the crash-day attempts measured **4,649** live
(`git count-objects -vH` → `count: 4649, size: 17.20 GiB`, attempt
transcripts, Addendum 4 §1). Same repository minutes apart — both are correct
for their moment, and the corpus carries both.

### Crash timestamp — confirmed, with the precision the stated date needs

The stated **2026-08-14T10:25:30Z is a real recorded event, but it is not a
kill instant** — it is the `HANDLING_RELEASE_DONE` release heartbeat of the
**second** kill of the storm:

| Fact | Value | Source |
|---|---|---|
| Stated timestamp resolved | 2026-08-14T10:25:30Z = crash #2's release heartbeat | needle event seq 1783 `heartbeat.emitted / HANDLING_RELEASE_DONE` at `10:25:30.457670958Z` (evidence `needle-worker-log-crash-window-full.jsonl` line 55); alert bead `bf-3m9m1v` description "Timestamp: 2026-08-14T10:25:30.457683731+00:00", `created_at 10:25:30.464340624Z` (checkpoint; verbatim copy in evidence `alert-beads-raw.jsonl`). Three nanosecond values in one second = three capture points (heartbeat event, alert body, alert creation), not a discrepancy |
| Crash #2's actual death | 10:25:01.512001992Z (`agent.completed`, exit −1, 104,481 ms) | evidence `exit-code-timeline.txt`, attempt 2 |
| Kill #1 — first death of the storm | **2026-08-14T10:23:02.958717335Z**, `agent.completed` seq 1741; attempt #1 dispatched 10:21:06.979830682Z, duration 115,797 ms (the longest phase-1 run) | needle events file + `exit-code-timeline.txt` attempt 1; evidence README "Timestamp reconciliation" |
| First alert heartbeat | 10:23:11.219513632Z (seq 1750) — carried by alert bead `bf-fmg2cw`, the first of 44; the last was `bf-5x69lm` at 11:28:02.194277764Z | evidence README + `alert-beads-exit-timestamps.txt` |
| Storm extent | 44 × exit −1 across 10:23:02–11:27:26Z, each surviving 38.9–115.8 s; then 8 × exit 124 (exactly 600.0 s) 11:38:07–12:50:14Z; then 1 × exit 0 at 12:58:45Z | evidence `exit-code-timeline.txt` (all 53 attempts); investigation doc Addendum 2 phase table |
| Window-start variance | the evidence README opens the window at 10:21:06Z (attempt #1's *dispatch*); the investigation doc's 10:23:02Z is attempt #1's *death*. Definitional, not contradictory | README vs investigation doc line 12 |

With 44 kills there is no single "the crash timestamp". Every figure cited
anywhere for this bead — 10:23:11, 10:25:30, 10:29:46, 10:39:42, 10:41:13,
10:52:14, 11:01:40, 11:14:39, 11:18:53 — is one of the 44 auto-minted alert
beads' release heartbeats, ~8–120 s after its own kill (needle pre-0.4.2
minted one alert bead per kill). Cite an instant only together with its
definition.

### Exit signal — confirmed

| Fact | Value | Source |
|---|---|---|
| Recorded exit | **`exit_code = −1` on all 44 kills** | needle `agent.completed` events (evidence events file); `exit-code-timeline.txt` |
| What −1 means | needle's **sentinel** for a worker process that died by signal — no wait status was reaped (`ExitStatus::code()` is `None`, normalized to −1). **Not a literal signal number**; the classifier maps negative → `crash` | `docs/signal-analysis-exit-code-negative-one.md`; investigation doc Addendum 2 "Signal −1, precisely" |
| Delivery | SIGKILL — the canonical inference from instant death, zero application error logs, and no core dumps. The −1 record alone does not identify which signal | Addendum 2 "Signal −1, precisely"; `transcript-attempt1-crash-8b2a5b0d.jsonl` ends at the `git gc --aggressive --prune=now` tool_use with no tool_result — killed mid-gc |
| Mechanism | **memcg-OOM SIGKILL of the gc inside the 12 GiB dispatch scope** (`MemoryMax=12GiB` transient `run-p*.scope`) against 17.20 GiB / 4,649 loose objects (18G `.git`) | Addendum 3 (live scope inspection); Addendum 6 (257 `CONSTRAINT_MEMCG` git kills on 2026-08-16 at 11.7–12.6 GB anon-rss — the same cleanup window, same mechanism on full display); the mechanical-guard record in the repo CLAUDE.md |
| Confidence | **Regime-matched, not kernel-proven for Aug-14.** The kernel journal for the crash window is unrecoverable (single surviving boot starts 2026-08-15 19:26 EDT). Host memory was NOT exhausted — 45Gi available captured mid-storm at 10:43:59Z (`transcript-midstorm-9539f3b2`); the limit hit was the scope's cgroup cap, and `oom_score_adj=200` marks these agent children preferred OOM victims | Addendum 2 "Evidence-window limitation"; Addendum 3; evidence README mechanism row |

### Cross-check against docs/crash-investigations/bf-4x12ec-crash-investigation.md

The doc's §Crashed Bead Details (lines 6–19) matches every recoverable record
field — id, title, task/P2, purpose, created 10:17:26Z — and its §Original
Work Context quotes the description accurately. Flagged items:

1. **Line 31, "The crash occurred at 10:25:30 UTC" (v1.0/v1.1 body)** —
   retained as historical; the doc's own Addendum 2 table (line 219) resolves
   it as crash #2's alert heartbeat, after attempt #2 died at 10:25:01Z. The
   stated timestamp for this child task inherits exactly that reading. Not a
   live error, but a reader of the body alone will misread it as the kill
   instant.
2. **Line 4 (Summary), "the `git gc --aggressive --prune=now` operation
   completed on the 53rd attempt at 12:58:45Z"** — **contradicted by the
   doc's own Addendum 4 §3 (lines 425–436), which the Summary was never
   updated for**: the 53rd attempt executed needle's auto-split template
   (created bf-173o7e / bf-5jhvpk / bf-im2sl1, chained them, labelled the
   umbrella, `SPLIT_COMPLETE`); the gc itself completed under child
   **bf-173o7e** (now Closed). This is the one substantive internal
   inconsistency this cross-check surfaces.
3. **Line 46, "Signal -1 = SIGKILL (Signal 9) in Linux", and line 99, "OOM
   killer invoked SIGKILL (signal 9)"** — imprecise as written: −1 is needle's
   sentinel, not a POSIX signal number. The doc's own Addendum 2 (lines
   257–265) states this correctly; SIGKILL remains the inferred delivery.
4. **Line 38, "OOM killer active, <2GB available during git operations"
   (v1.0, carried over from parallel investigations)** — contradicted by
   direct evidence: 45Gi available captured mid-storm. The binding limit was
   the dispatch scope's cgroup cap, not host memory. Addendum 2 already
   corrects the opposite hearsay error ("OOM impossible, 51GB was available");
   both framings fail the same way — they read system RAM where the limit was
   cgroup-local.
5. **Line 41, "9 systematic crashes in 2.5 hours on bf-4yjq alone"** —
   superseded corpus figure (corrected counts: 76 dispatches / 71 kills for
   bf-1s6c3 and ~50 for bf-4yjq the same evening;
   `docs/crash-analysis-bf-1s6c3-2026-09-06.md`, repo CLAUDE.md).
6. Cosmetic: the body's "17.16GB loose" (line 26) vs the bead description's
   17.20GB and the attempts' 18G `.git` / 4,649 objects — different measures
   of the same bloat at slightly different moments, the same effect as the
   4,627/4,649 pair above.

No contradiction touches the record identity (title / type / priority /
assignee), the crash-window facts, or the exit-signal semantics. Where the
doc's body and its addenda disagree, the addenda carry the primary-source
version.

## Exit code and signal analysis (child 3 of the split, `domchk-0e707410`)

Written 2026-09-07 by the exit/signal child of the `domchk-c99cdf80` split.
Source material is the retrieved bundle `docs/crashes/bf-4x12ec/`
(`domchk-4bad8e94`) and its sibling evidence bundle (9b32085); every figure
below was re-extracted first-hand from those files for this section rather
than carried over from the addenda.

### What the retrieved logs record

Exit-code census over the 53 `agent.completed` events in
`needle-events-2026-08-14-bf-4x12ec.jsonl.gz` (re-tallied from the file, and
cross-checked against `attempt-index.tsv` — the two agree):

| exit_code | n | `outcome.classified` | attempts | completion window (UTC) | durations |
|---|---|---|---|---|---|
| −1 | 44 | `crash` | 1–44 | 10:23:02.958Z → 11:27:26.174Z | 38.9 – 115.8 s |
| 124 | 8 | `timeout` | 45–52 | 11:38:07.867Z → 12:50:14.283Z | 600,018 – 600,024 ms (the 600 s cap) |
| 0 | 1 | `success` | 53 | 12:58:45.114Z | 491,784 ms |

The stream records **no other exit codes** — in particular no 137 (a shell's
own 128+9 job report can appear only inside captured stderr text, never in
the `exit_code` field) and no ≥129 signal-shaped code. Attempt 2 — the death
the tasked timestamp names — completed 10:25:01.512001992Z, exit −1, duration
104,481 ms.

### What "exit code −1" and "signal −1" denote

- **−1 cannot be a real exit status** (POSIX statuses are 0–255). It is
  needle's writer-side sentinel: `status.code().unwrap_or(-1)` —
  `ExitStatus::code()` is `None` for a signal death, so *any* signal flattens
  to −1, and −1 is also written literally when no wait status was ever
  reaped (`docs/signal-analysis-exit-code-negative-one.md` §2, source-pinned
  and re-verified 2026-09-07 by `domchk-15999f2c`).
- **The one kill needle does not record as −1 is its own deadline kill**, which
  becomes 124 (GNU timeout convention). That is why the 8 × 124 rows separate
  cleanly from the 44 × −1 — and it rules out needle's own timeout machinery
  as the source of the −1s: those attempts died at 38.9–115.8 s, nowhere near
  the 600 s ceiling the agent itself passed to the gc (`timeout: 600000` on
  the final `tool_use`).
- **Classification path:** `Outcome::classify` maps negative → `Crash(code)`;
  the crash handler then renders the alert body
  `- **Exit code**: {code} (signal {signal_num})` with
  `signal_num = code ≤ 128 ? code : code−128`, and labels the alert
  `signal-{signal_num}`. On the sentinel this produces, verbatim,
  `- **Exit code**: -1 (signal -1)` and the label `signal--1` — confirmed
  live in the sibling bundle's `alert-beads-raw.jsonl`: **44/44** alert beads
  carry both the body line and the label.
- **"(signal −1)" is template arithmetic on a sentinel, not a signal number.**
  No negative signal exists (`signal(7)` numbers 1–64); the `signal--1` label
  is cosmetically broken and semantically empty. The records therefore carry
  **no signal identity** — SIGKILL is inferred from context, not recorded
  (see the correlation table below for that inference).
- The corpus's older "−1 ⇒ SIGHUP" reading is the Python-subprocess
  convention; needle is Rust and flattens every signal to −1, and this log
  contains zero sighup/hangup evidence (signal-analysis doc §5.2).

### Crash classification (per `docs/crash-response-guide.md`)

**INFRASTRUCTURE — memcg-OOM SIGKILL inside the 12 GiB dispatch scope.**

- **Entry rule:** guide Phase 1 reads "Exit code -1 → Infrastructure event
  (Phase 2A)". All 44 deaths match it.
- **Class definition match:** the guide's INFRASTRUCTURE class is
  "memcg-OOM inside the dispatch scope, resource exhaustion, repository
  bloat → Check the cgroup boundary and repo size, verify work completion" —
  this crash matches every clause.
- **Not the other classes:** not SERVICE_FAILURE (no 5xx, no external service
  in the path); not FALSE_POSITIVE-by-completion *at the death instants*
  (each kill landed mid-task inside the gc — the work completed only later,
  12:58:45Z, under child bf-173o7e); not CODE_DEFECT (no application error in
  any of the 53 attempts, and the identical binary/template/prompt completed
  at attempt 53 and again under the child).
- **Phase 2A checklist disposition:**

  | Checklist item | Result |
  |---|---|
  | Check the **cgroup boundary**, not just the host | ✅ host had 50 Gi available 65 s before the kill; the binding limit is the scope's `MemoryMax=12GiB`, identified from live scopes (canonical report Addendum 3) |
  | Kernel `oom-kill`/`CONSTRAINT_MEMCG` lines in the window | ⚠️ **unrecoverable for Aug-14** — the surviving boot begins 2026-08-15 19:26 EDT. The guide's own caveat (a notice with no kernel line may be a replayed counter) applies, so the classification rests on corroboration, stated as such below |
  | Verify task completion before classifying | ✅ completed by retry — `verification.passed` 12:58:45.126649351Z; bead closed 2026-08-17 rev 4 |
  | 30-second-rule FALSE_POSITIVE check | ✅ not applicable — per-attempt committing was not the pattern here; every kill landed inside the gc itself |

- **Action per the guide: "NO CODE CHANGES NEEDED."** The mitigations are
  configuration, not code: `pack.windowMemory=2g` / `pack.deltaCacheSize=1g` /
  `pack.threads=1` (repo + global) and `scripts/safe-git-gc.sh` — all landed
  (repo CLAUDE.md, "Mechanical guard for the bare-gc path").

### Resource-state correlation (the supporting evidence)

Attempt-2 session transcript (`session-transcript-attempt2-971486ad.jsonl`,
28 records, read first-hand), aligned against the worker-log kill record:

| Time (UTC) | Record | Reading |
|---|---|---|
| 10:23:26.221Z | `git count-objects -vH` | **4,649 loose objects, 17.20 GiB** (in-pack 4,081 in 1 pack, 9.60 MiB; 0 garbage) |
| 10:23:43.210Z | `du -sh .git/` | **18G** |
| 10:23:56.490Z | `free -h` | 62 Gi total, 11 Gi used, **50 Gi available**; swap 24 Gi, 0 B used; up 2 days |
| 10:24:08.660Z | final transcript record | `Bash` `tool_use` — **`git gc --aggressive --prune=now`**, timeout 600000, **with no matching `tool_result`** |
| 10:25:01.512Z | `agent.completed` (worker log) | **exit −1**, 104,481 ms |

1. **The kill lands inside the gc.** 52.9 s after the last transcript record:
   the agent died running the exact operation it was tasked to run, with the
   tool call still outstanding.
2. **The binding resource is not host RAM.** 50 Gi available at the last
   reading, 65 s before the kill. The ceiling hit is the dispatch scope's
   cgroup cap (12 GiB): 17.20 GiB of loose objects cannot be packed under it
   with `--aggressive`'s large pack windows. This is exactly the situation
   the guide's Phase 2A warns about — "the kill can land while the host has
   memory to spare".
3. **The −1/124 boundary is itself evidence.** Attempts that hit the *memory*
   ceiling died at 38.9–115.8 s (−1); attempts that reached the *time*
   ceiling died at exactly 600.0 s (124, needle's own deadline kill); and the
   one clean exit (0, attempt 53) is the attempt that never ran the gc at all
   — it executed needle's auto-split template instead (Addendum 4 §3). The
   command, prompt (71,698 bytes) and template were identical throughout the
   44 + 8 — the variable that separates the two kill regimes is which ceiling
   each attempt reached first, memory or time; the eventual clean gc
   completion belongs to child bf-173o7e, after the retry chain had ended.
4. **No defect signature.** No application error output accompanies any of
   the 44 deaths, and there are no core dumps (Addendum 2).
5. **Same-period kernel corroboration.** The same cleanup effort, two days
   later with kernel logging available, produced **257 `CONSTRAINT_MEMCG`
   git kills on 2026-08-16** at 1.2–11.97 GB anon-rss (163 of 257 hugging the
   11–12 GB ceiling), all `oom_score_adj=200` inside transient
   `run-p*.scope` memcgs — the same scope, the same mechanism, kernel-visible
   (Addendum 3 §"NEW: 257 git OOM-kills on Aug-16"; Addendum 6). No git OOM
   kill occurs on any other day of the current boot.
6. **Why exit −1 rather than a 137 from inside git:** `memory.oom.group=0` on
   these scopes means the kernel kills a single task — the highest-badness
   one in the hitting memcg. On Aug-16 that was usually `git` itself; on
   Aug-14 it was the agent child (hence exit −1 on the worker record). Same
   cause, different victim (Addendum 3).
7. **Isolation.** 0/44 phase-1 deaths have any other bead completing within
   ±3 s (Addendum 2 §"Isolation: not a fleet-wide event") — a bead-local
   retry storm, not a system-wide event.

### Confidence

| Claim | Confidence | Basis |
|---|---|---|
| `exit_code −1` = died-without-exit-status sentinel | **Certain** | needle source, pinned and unit-tested; 44/44 records consistent |
| "(signal −1)" carries no signal identity | **Certain** | template arithmetic on the sentinel; no negative signal exists |
| Outcome class INFRASTRUCTURE (Phase 2A) | **High** | every Phase 1/2A rule and checklist item points the same way |
| Delivery = SIGKILL | **High (inferred)** | instant death, no error output, no core dump, 600 s cap never reached |
| Mechanism = memcg-OOM at the 12 GiB scope cap | **High (regime-matched, not kernel-proven for Aug-14)** | resource-state correlation above + 257 same-mechanism kills two days later; the one thing that would settle it outright — an Aug-14 kernel line — is unrecoverable (surviving boot starts 2026-08-15 19:26 EDT) |

## Workspace and system state at crash time (child 2 of the split, `domchk-d7241598`)

Written 2026-09-08 by the workspace/system-state child of the `domchk-c99cdf80` split. Scope:
the repository and host conditions inside which all 44 `exit −1` kills landed (10:23:02.958Z →
11:27:26.174Z — child 3 §"What the retrieved logs record"), each with its source. The
attempt-2-local readings are already in child 3's §"Resource-state correlation"; this section
carries the era-level telemetry, and it separates **contemporaneous measurement** from **later
reconstruction**, because the tasked figures mix the two and quoting them undifferentiated
reintroduces errors this doc's cross-check list already resolved.

### Repository state at crash time

| Metric | Value | Source |
|---|---|---|
| `.git` size | **18G** | **Read inside this crash's window** — attempt 2's own `du -sh .git/` at 10:23:43.210Z (`session-transcript-attempt2-971486ad.jsonl`, child 3 §"Resource-state correlation"). Era brackets agree: `docs/crash-analysis/bf-4yjq-system-state-snapshot-2026-09-01.txt` ("Total Repository Size: 18 GB"); `docs/archive/crash-investigations/system-state-investigation-bf-173o7e-2026-08-14.md` ("domain-check/.git: 18 GB total") |
| Loose objects | **17.20 GiB / 4,649 objects** | **Read inside this crash's window** — attempt 2's `git count-objects -vH` at 10:23:26.221Z (child 3 §"Resource-state correlation") |
| Loose objects, era readings | 17.16 GiB / 4,482 unpacked; 17.20 GB / 4,515 | `docs/crash-analysis/bf-4yjq-system-state-snapshot-2026-09-01.txt` (Aug-12 era); `docs/research/git-gc-oom-crash-analysis.md:66` region (cleanup-era count, credited there to `docs/cleanup-resolution-2026-08-17.md` and `docs/crash-artifacts-bf-4yjq-raw.md`) |
| Loose share of the repo | **~95.7%** | `docs/archive/crash-investigations/system-state-investigation-bf-173o7e-2026-08-14.md:52` ("17.16 GB (95.7% of repository)"); `docs/analysis/agent-signal-minus1-root-cause-analysis.md:61` and `docs/reports/bf-4yjq-comprehensive-crash-report.md:281` ("17.20 GB (95.7% of total repository size)"); `docs/research/git-gc-oom-crash-analysis.md:66` renders the same figure as a loose:packed ratio |
| Packed side | 9.60 MiB in 1 pack, 4,081 in-pack — loose:packed ≈ 1,800:1, inverted | bf-4yjq snapshot `git count-objects -v` transcription; the same inverted-ratio wording in `docs/archive/crash-investigations/system-state-investigation-bf-173o7e-2026-08-14.md` ("critically inverted ratio") |
| Committed bloat | ~237–248 MB `.beads/*.jsonl` × **17+ commits** | `docs/crash-investigation-bf-4yjq.md:143` ("17+ commits each carrying a ~237–248 MB `.beads/*.jsonl` snapshot from bead bf-2ildm's extraction"); `docs/crash-analysis/repository-bloat-root-cause-analysis-2026-08-12.md:27-31` (the three 237 MB file paths) |
| `git fsck` | timed out after 2 minutes | bf-4yjq snapshot ("`git fsck --no-full`: Times out after 2 minutes"); `docs/crash-investigation-bf-4yjq.md` §5 |

The loose-object counts differ across the bloat era (4,482 → 4,515 → 4,594 → 4,649) because the
bloat was still growing and because the interrupted/retried repacks added loose objects of their
own — `docs/research/git-gc-oom-crash-analysis.md` states that feedback loop explicitly ("the
memory-intensive gc attempts that crashed were themselves large git operations, and
interrupted/retried repacks added further loose objects before cleanup"). The **18G / 17.20 GiB /
4,649** triple is the one measured *inside* this crash's window; the rest are the era's brackets
and are the reason the bead description's "17.20GB" and the older body text's "17.16GB" are both
correct (cross-check item 6).

### Host state during the window

| Metric | Value | Source, and what kind of source it is |
|---|---|---|
| Load average | **15–17 on 12 cores** | `docs/crash-investigation-bf-4yjq.md:168` — bf-4yjq-era telemetry (Aug 12). Contemporaneous for that storm; **not re-measured in the bf-4x12ec window** |
| Disk | **84% full, ~71 GB free** (350 GB / 444 GB used) | `docs/crash-investigation-bf-4yjq.md:170`; `docs/crash-root-cause-bf-4yjq.md:109`; `docs/crash-data-extraction-bf-4yjq.md:148` — era-level, Aug 12 |
| Available memory during git ops | **"<2GB"** | `docs/crash-investigation-bf-1s6c3-2026-09-01.md:107,200,211`. **Later reconstruction, and contradicted for this crash** — caveat 1 below |
| OOM killer | **active**, delivering SIGKILL | `docs/crash-investigation-bf-1s6c3-2026-09-01.md:203` ("OOM Killer: Active - delivered SIGKILL events"). **Inference, not an Aug-14 observation** — caveat 2 below |
| Kernel records for the window | **none survive** | `journalctl` on the surviving boot starts 2026-08-15 19:56:33 EDT (`docs/crash-investigation-bf-4yjq.md` §2.3; the older "19:26" reading was corrected by `docs/crash-investigations/bf-4x12ec-log-source-inventory-domchk-a3f1f8f5-2026-09-07.md`) |
| Swap | 0 B used of 24 Gi | attempt 2's `free -h` at 10:23:56.490Z (child 3 §"Resource-state correlation") — read inside this window |

**Two corrections the tasked figures need before they can be quoted.** Both are already in this
doc's cross-check list; they are restated here because the acceptance criteria for this child
carry the uncorrected forms verbatim, and an undifferentiated table would undo that work.

1. **"<2GB available during git operations" is a reconstruction from the Aug-12/16 corpus, not a
   reading from this window — and the direct evidence for this window says the opposite.** Attempt
   2 read **50 Gi available** at 10:23:56.490Z, 65 s before the kill, and a mid-storm capture read
   **45 Gi** at 10:43:59Z (`transcript-midstorm-9539f3b2`). The host was never exhausted on
   Aug 14. What bound the gc was the dispatch scope's `MemoryMax=12GiB` cgroup cap: 17.20 GiB of
   loose objects cannot be packed under it with `--aggressive`'s large pack windows. The `<2GB>`
   figure describes a real mechanism (git loading the loose-object store) observed on *other*
   days of the bloat era; it does not describe the memory state at this crash. Cross-check item 4
   already resolved this; this section records the figure only so the tasked criterion is traceable
   to its source and to its correction.
2. **"OOM killer active" is an inference from the death signature, not an Aug-14 kernel
   observation.** The line that would prove it cannot exist — the journal for Aug 12–14 is gone.
   The same corpus that asserts it also overstates its own census ("9 systematic crashes",
   superseded to 76 dispatches / 71 kills for bf-1s6c3 and 50 for bf-4yjq — cross-check item 5).
   What carries the mechanism to the recorded confidence is later, kernel-visible corroboration:
   **257 `CONSTRAINT_MEMCG` git kills on 2026-08-16** at 1.2–11.97 GB anon-rss inside the same
   transient `run-p*.scope` memcgs (Addendum 3, Addendum 6) — the same cleanup effort, two days
   later, with logging available. Child 3's confidence table records this as "High
   (regime-matched, not kernel-proven for Aug-14)"; that is the correct strength, and this section
   does not upgrade it.

### Parallel investigations that establish these conditions

| Investigation | What it establishes for this section | Canonical record |
|---|---|---|
| **bf-4yjq** — Aug 12, 50 consecutive dispatch deaths, all exit −1, 17:53:53Z → 20:30:38Z | The era's system-state table (repo ~18 GB, loose 17.16–17.20 GiB, packed 9.60 MiB, load 15–17, disk 84% / ~71 GB free, `fsck` timeout), the inverted-ratio framing, and the log-source coverage table proving no kernel record exists for Aug 12 | `docs/crash-investigation-bf-4yjq.md` §2.3 and §5 |
| **bf-2ildm** | **The origin of the bloat.** Its GitHub-commits extraction wrote the ~237 MB `.beads/*.jsonl` files that were committed 17+ times between 2026-08-01 and 2026-08-12, growing the repo from ~500 MB to 18 GB. Its 2026-09-07 re-investigation independently states the kill mechanism (kernel memory-cgroup OOM after the 12 GiB `MemoryMax` per-dispatch scope was exceeded) and records that no memory or disk telemetry survives for its own window | `docs/crash-analysis/repository-bloat-root-cause-analysis-2026-08-12.md:20-31`; `docs/investigations/investigation-report-bf-2ildm-2026-09-07.md:22-23, 207-208` |
| **Aug-12 storm / bf-1s6c3** | The `<2GB available` / ">50GB git RSS" / "94.71% memory pressure" reconstruction and the "OOM Killer: Active" assertion — i.e. the source of the tasked memory figures, and the source of the superseded "9 crashes" census | `docs/crash-investigation-bf-1s6c3-2026-09-01.md:95-115, 195-215`; corrected by `docs/crash-analysis-bf-1s6c3-2026-09-06.md` and the repo CLAUDE.md |

These are one environmental regime, not three separate causes: **bf-2ildm created the bloat,
bf-4yjq and bf-1s6c3 died on it across Aug 12–13, and bf-4x12ec died on the same bloat on
Aug 14 while trying to remove it.** The task bf-4x12ec was dispatched to perform is what
terminated the era — and it is the task whose every attempt was killed.

### What the agent was attempting

**Phase 1.2 emergency repository stabilization.** The recovered bead description reads: *"Execute
aggressive git garbage collection to pack 17.20GB of loose objects into compressed pack files,
eliminating the OOM risk during git operations. This is Phase 1.2 from the root cause analysis
(CRITICAL - NOT YET EXECUTED)."* — quoted in `docs/crash-investigations/bf-4x12ec-crash-investigation.md:54`
(purpose stated at `:11`, restated at `:691-692`); `docs/crash-investigations/bf-4x12ec-final-crash-report.md:28`.

Every one of the 44 `exit −1` attempts was killed while running that same command. Attempt 2's
transcript ends at the `git gc --aggressive --prune=now` `tool_use` with no matching
`tool_result`; the kill landed 52.9 s after the last transcript record (child 3 §"Resource-state
correlation", item 1). The environment the task was meant to repair is therefore the same
environment that killed each attempt at it — the gc could not complete inside a 12 GiB scope
while 17.20 GiB of loose objects were what it had to read. The operation was eventually completed
under child **bf-173o7e**, after the retry chain had ended: the task succeeded, only the dispatch
attempts died (Addendum 4 §3; child 3 §"Resource-state correlation", item 3).
## Files and systems involved (`domchk-6df39087`)

Written 2026-09-08T01:50Z by the files-and-systems child of the `domchk-c99cdf80` split. Scope:
everything the bf-4x12ec task was aimed at, everything that killed it, and everything that now
carries its record — catalogued, not re-analyzed. The kill mechanism and its evidence chain are
child 3 §"Crash classification"; the repo/host conditions are child 2. Numbering note: this bead
and `domchk-0e707410` (§"Exit code and signal analysis") both carry "child 3" in their dispatch
descriptions — two sections, one number; the bead IDs are the stable identifiers.

Every path below was verified live at HEAD `b435372` (2026-09-08): existence and tracking with
`git ls-files --error-unmatch`, beads via the live store.

### The target of the original task — this repo's git object store

| Item | Detail |
|---|---|
| What bf-4x12ec was dispatched to repair | The `.git` object store of **this** repository: 18G, of which **17.20 GiB / 4,649 loose objects** — the attempt-2 readings taken inside the crash window (child 2 §Repository state). ~95.7% loose, loose:packed ≈ 1,800:1 |
| What grew it | ~237–248 MB `.beads/*.jsonl` snapshots committed 17+ times between 2026-08-01 and 2026-08-12 (bf-2ildm's GitHub-commits extraction). One environmental regime: bf-2ildm created the bloat, bf-4yjq and bf-1s6c3 died on it Aug 12–13, bf-4x12ec died on it Aug 14 **removing** it (child 2 §Parallel investigations) |
| The operation every attempt died in | Bare `git gc --aggressive --prune=now` — the bead's own acceptance criterion, alongside `git repack -a -d --depth=250 --window=250` and a `git fsck --no-full` that had been timing out at two minutes (§Original bead context; child 2 §Repository state) |
| Where the store stands now (measured live 2026-09-08T01:50Z) | `.git` 105M; 178 loose objects; 12,174 in-pack in a 100.25 MiB pack; `fsck` clean per the repo CLAUDE.md health record. `.beads/` is wholly gitignored (`.gitignore:66`) plus repo-wide `*.db` / `*.jsonl` (`.gitignore:68-70`), so the growth path that created the bloat cannot recur through bead state |

### The mitigation layer that exists because of this crash

This is the guide's "NO CODE CHANGES NEEDED" disposition (child 3) turned into tooling —
configuration and wrappers, not application changes. All paths tracked at HEAD `b435372`.

| Layer | Files |
|---|---|
| Bounded gc — replaces the bare one-liner that killed all 44 attempts | `scripts/safe-git-gc.sh` (memory-capped, staged, checkpoint/resume, preflight validation), `scripts/cleanup-bloat.sh` — whose header names its own provenance: *"Replaces the old 'bare git gc --aggressive --prune=now' one-liner that caused the bf-1s6c3 crash"* — plus `scripts/cleanup-repo-bloat.sh`, `scripts/recover-repo-bloat.sh`, `scripts/safe-git-gc-monitor.sh`, `scripts/git-gc-monitor.sh`, `scripts/detect-unsafe-gc.sh`, `scripts/pre-gc-health-check.sh`, `scripts/check-repo-size.sh` |
| The mechanical guard on the bare path | `scripts/setup-git-gc-config.sh` — persistent `pack.windowMemory=2g` / `pack.deltaCacheSize=1g` / `pack.threads=1`, applied repo-local **and** global, bounding bare `git gc` **and** `git push` pack-objects (the bf-198ne push-side variant of this same crash) |
| Repo-health detection | `scripts/check-repo-health.sh`, `scripts/repo-health-monitor.sh`, `scripts/repo-health-check.sh`, `scripts/monitor-repo-health.sh`, `scripts/auto-gc-trigger.sh`, `scripts/preflight-health-check.sh` |
| Scheduled maintenance | `scripts/setup-repo-maintenance.sh` plus the `domain-check-*-timer`/`-service` unit files in `scripts/` — systemd **user** timers (this box is NixOS; no crontab): crash-pattern 10 min, resource 5 min, service 2 min, repo-health + auto-gc check daily 02:00, incremental gc 03:00, full gc Sun 04:00 |
| Crash-alert pipeline — the layer that minted this crash's 44 alerts and now suppresses their duplicates | `scripts/crash-alert-manager.sh` (closed-bead filter, dedup + processed-alert tracking, completion awareness, exit-code validation, cooldown), `scripts/crash-classifier.sh`, `scripts/alert-deduplication.sh`, `scripts/alert-cooldown.sh`, `scripts/crash-pattern-detection.sh`, `scripts/alert-triage-sweep.sh`, `scripts/classify-signal-crash.sh`, `scripts/crash-circuit-breaker.sh`, `scripts/setup-alert-triage-timer.sh` |
| Work-completion and commit hygiene | `scripts/verify-work-completion.sh`, `scripts/pre-commit-repo-size-hook` (per-clone, installed by `scripts/setup-git-hooks.sh`) — the 10 MB staged-file gate that would have blocked the 237 MB `.beads/*.jsonl` commits that started the bloat |
| Test suites for the layer | `scripts/test-safe-git-gc-limits.sh`, `scripts/test-gc-memory-bounds.sh` (reruns both memcg-OOM death commands under a 768 MiB cgroup), `scripts/test-cleanup-bloat.sh`, `scripts/test-setup-git-hooks.sh`, `scripts/test-crash-alert-fixes.sh`, `scripts/test-closed-bead-filter.sh`, `scripts/test-repo-monitoring.sh`, and the remaining `scripts/test-*` companions |
| Run-time forensics the layer writes | `.git/safe-gc.log` and `.git/safe-gc-checkpoint.json` (what ran, peak RSS); `.beads/logs/git-gc-check.log`, `git-gc.log`, `git-gc-full.log`, `crash-monitor.log`, `resource-monitor.log`, `service-monitor.log` — all gitignored |

### The bead-forge → bead-rs migration (confounding system)

The store the crash-era record was born in and the store that holds it today are **different
systems**, and the seam falls directly across the crash.

| Fact | Value | Source |
|---|---|---|
| Store the bead was created and dispatched in | **bead-forge** — the `bf-` prefix; created 2026-08-14T10:17:26Z, all 44 kills the same day | §Original bead context |
| Store that holds it now | **bead-rs** — `.beads/config.json` + `.beads/beads.db` (SQLite). `bf`/`bead-forge` is retired on this box; running it against a bead-rs store is a documented corruption hazard, never a recovery tool | repo CLAUDE.md "Beads (bead-rs CLI)"; live workspace shape verified 2026-09-08 |
| Ordering | **The crash predates the migration.** Last crash-era event 2026-08-14T12:58:45Z (attempt 53); the rehydration commit `8373e5d` *"migrate: rehydrate the bead workspace from bead-forge to bead-rs"* is dated 2026-08-15 13:56:53Z (CLAUDE.md dates the migration program itself 2026-08-14). Under either date every bf-4x12ec death precedes the store that now carries the record | `git show -s 8373e5d`; repo CLAUDE.md |
| What the seam costs the record | The bead-rs checkpoint's **event** stream begins 2026-08-16T04:21:10Z — **zero Aug-14 events survive as events**. Everything crash-era (creation, the 44 alert beads, pre-migration assignment) survives only as **imported issue snapshots** in `forensic.jsonl`; the earliest bf-4x12ec-relevant *events* are post-migration (seq 2194 `assignment_cleared` 2026-08-17, seq 3407 `closed` 2026-08-17). This is the **cause** of §Original bead context's caveat that the crash-time assignee rests on the needle log's `worker_id` + `prior_assignee` rather than any live assignment event | checkpoint grep (this bead, 2026-09-08): earliest event `"time":"2026-08-16T04:21:10.940Z"` |
| Migration artifacts — all **outside** this repo | `~/bf-migration-backup/domain-check.*`: `beads.tgz`, `converted.jsonl`, `dest.jsonl`, `import.log` (1,571 issues inserted, 0 conflicted, 0 retained), plus the src/dest id, label and title maps | directory listing + import log, 2026-09-08 |
| Recovery path if the live store is lost, wrong-schema, or corrupt | `bead init`, then `bead sync import-only --input .beads/checkpoint/forensic.jsonl --restore-into-empty --actor <you>` — lossless only to the last explicit `bead sync flush-only`; **never** `--merge` (wipes external references, comments and structured data on updated issues); **never** any bf-shaped command | repo CLAUDE.md "Recovering a broken or fresh-clone workspace" |

### The systems in the kill path

| System | Role in this crash |
|---|---|
| NEEDLE dispatch | Owned every attempt: worker `claude-code-glm-4.7-lab-domain-check`, one dispatch per attempt inside a systemd transient scope, the per-attempt `agent.completed` records behind the 53-row census (child 3 §"What the retrieved logs record"), and — pre-0.4.2 — auto-minting **one alert bead per kill**: the 44 alerts, and downstream the Aug-26 duplicate/false-positive verification wave catalogued below |
| systemd cgroups | The binding limit: transient `run-p*.scope` memcg with `MemoryMax=12GiB`; `oom_score_adj=200` marking the agent children preferred victims; `memory.oom.group=0` meaning a single-task kill — which is why the same mechanism killed `git` itself on Aug-16 and the agent child on Aug-14 (child 3 §"Resource-state correlation", item 6) |
| Linux kernel memcg OOM killer | The killer: `CONSTRAINT_MEMCG` SIGKILL. Kernel-proven for the same cleanup effort on 2026-08-16 (257 git kills); regime-matched, not kernel-proven for Aug-14 itself — the journal for that window is unrecoverable (child 2 §Host state, caveat 2) |
| git (`pack-objects`) | The memory consumer: `--aggressive`'s pack windows reading 17.20 GiB of loose objects from inside a 12 GiB scope — arithmetically impossible to complete |
| **Not** involved | **domain-check application code** — zero defects across every investigation of this and every other crash in this workspace; no external service in the kill path (no 5xx, no timeout — so not SERVICE_FAILURE); the host's RAM was never exhausted (50 Gi available 65 s before the kill) |

### The files that carry the record

**Sibling docs already covering this crash** — the tasked three, one of which has moved:

| Tasked path | State at HEAD `b435372` |
|---|---|
| `docs/crash-investigation-bf-4x12ec.md` | Present — the early repo-root investigation |
| `docs/crash-summary-bf-4x12ec-comprehensive.md` | **Moved** — now `docs/archive/crash-investigations/crash-summary-bf-4x12ec-comprehensive.md`. The tasked root path no longer exists (same archive freeze that moved ~387 crash docs into `docs/archive/crash-investigations/`); cite the archive path |
| `docs/crash-investigations/bf-4x12ec-crash-investigation.md` | Present — the consolidated report with Addenda 1–7; this file's line-citation target |

**Verification reports referencing bf-4x12ec** — the tasked six, all tracked, all in
`docs/archive/crash-investigations/`, all dated 2026-08-26. They dispose of the *alert-layer*
aftermath (each re-verifies one of the 44 auto-minted alerts against the already-completed bead),
not new crashes:

| Alert bead | File | Verdict |
|---|---|---|
| bf-22h8jj | `verification-report-bf-22h8jj-false-positive-resolved-bf-4x12ec-crash.md` | FALSE POSITIVE resolved |
| bf-438934 | `verification-report-bf-438934-duplicate-alert-resolved-bf-4x12ec-crash.md` | DUPLICATE alert resolved |
| bf-1uh46l | `verification-report-bf-1uh46l-duplicate-alert-resolved-bf-4x12ec-crash.md` | DUPLICATE alert resolved |
| bf-22w69c | `verification-report-bf-22w69c-duplicate-alert-resolved-bf-4x12ec-crash.md` | DUPLICATE alert resolved |
| bf-qz9mov | `verification-report-bf-qz9mov-duplicate-alert-resolved-bf-4x12ec-crash.md` | DUPLICATE alert resolved |
| bf-whzeuf | `verification-report-bf-whzeuf-duplicate-alert-resolved-bf-4x12ec-crash.md` | DUPLICATE alert resolved |

Same-shape reports outside the tasked six, all tracked: `verification-report-bf-{2m532x,3cy3vk,44upi7,4h2mqq}-…`
in the same archive directory; `docs/crashes/bf-4nmj66-duplicate-alert-resolved-bf-4x12ec-crash.md`
and `docs/crashes/bf-5a3q4w-duplicate-alert-resolved-bf-4x12ec-crash.md`;
`docs/verification/bf-2u3dzu-crash-alert-bf-4x12ec.md` and `bf-5f9xqg-crash-alert-bf-4x12ec.md`;
`docs/crash-reports/bf-4x12ec-verification-report.md`.

**Evidence bundles** (tracked, force-added past the repo-wide `*.jsonl` ignore — the bundles the
earlier sections of this file quote from):

| Bundle | Contents |
|---|---|
| `docs/crashes/bf-4x12ec/` (retrieved by `domchk-4bad8e94`) | `needle-events-2026-08-14-bf-4x12ec.jsonl.gz` (the 53 `agent.completed` events), `needle-events-…-attempt2-bracket.jsonl`, `session-transcript-attempt2-971486ad.jsonl`, `attempt-index.tsv`, `bracket-source-lines.tsv`, `transcripts/`, `MANIFEST.sha256`, `README.md` |
| `docs/crash-investigations/evidence/bf-4x12ec/` (committed `9b32085`) | `crash-logs/` — `needle-worker-log-bf4x12ec-events.jsonl`, `needle-worker-log-crash-window-full.jsonl`, `alert-beads-raw.jsonl` (all 44 verbatim), `alert-beads-exit-timestamps.txt`, `exit-code-timeline.txt`, four verbatim attempt transcripts; plus `crash-logs/README.md`, `operation-summary.md`, `system-state.md` |

**Analysis trail in this repo** (tracked unless noted): `docs/crash-investigations/bf-4x12ec-*.md` —
`alert-inventory`, `crash-artifacts-2026-09-02`, `crash-timeline-domchk-ba8584a1-2026-09-07`,
`evidence-signal-semantics-domchk-15854355-2026-09-07`, `final-crash-report`,
`log-review-2026-09-02`, `log-source-inventory-domchk-a3f1f8f5-2026-09-07` (the doc that corrected
the surviving-boot first entry to 2026-08-15 19:56:33 EDT), `root-cause`,
`section-inventory-domchk-f6aba211-2026-09-07`; `docs/crash-investigations/bf-4x12ec-remediation-plan-domchk-0fcaef88-2026-09-08.md`
(untracked at this HEAD — a sibling's in-flight deliverable); `docs/signal-analysis-exit-code-negative-one.md`
(the −1-semantics source); and this file's own three earlier sections.

**Outside the repo:** `~/.needle/logs/needle-<worker>.log` (the fleet worker log the events files
were cut from); journald (the kernel memcg records — none survive for Aug-14); `.beads/beads.db`
and `.beads/checkpoint/forensic.jsonl` (gitignored — the authoritative store and its durable copy);
`~/bf-migration-backup/` (above); `.git/safe-gc.log` (post-repair gc forensics).

### Classification (per `docs/crash-response-guide.md`)

**INFRASTRUCTURE — memcg-OOM SIGKILL of the gc inside the 12 GiB dispatch scope.** The guide's
class table defines INFRASTRUCTURE as *"memcg-OOM inside the dispatch scope, resource exhaustion,
repository bloat"* — every clause matches, and its Phase 2A "Common Infrastructure Events" list
names bf-4x12ec explicitly. Explicitly not the others: not FALSE_POSITIVE *at the death instants*
(every kill landed mid-task inside the gc; completion came later, 12:58:45Z, under child
bf-173o7e), not SERVICE_FAILURE (no external service, no 5xx), and **not a domain-check
CODE_DEFECT** — zero application errors across all 53 attempts, with the identical
prompt/template completing at attempt 53 and again under the child. Disposition per the guide: no
code changes; the mitigations are the configuration layer catalogued above. The rule-by-rule
Phase-2A walkthrough and the confidence table are child 3 §"Crash classification" — not
duplicated here.
