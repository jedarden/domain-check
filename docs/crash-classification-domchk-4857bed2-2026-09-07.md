# Crash Classification — Corpus-Wide Verdicts (domchk-4857bed2, 2026-09-07)

**Bead:** domchk-4857bed2 ("Classify crash type and analyze failure mode")
**Chain:** consumes [`docs/crash-data-gathering-domchk-990ef135-2026-09-07.md`](crash-data-gathering-domchk-990ef135-2026-09-07.md)
(data gathering); feeds domchk-4f0b8b43 ("Document root cause with supporting evidence")
**Provenance:** drafted 2026-09-07T18:00–18:06Z (dispatch 2, `lab-roam-6`, repo at
`0f4d659` == `origin/main` at that time); **independently re-verified and corrected**
2026-09-07T18:11–18:30Z (dispatch 3, `lab-roam-11`, HEAD `d82b6a2`→`1f56d9c` as co-tenant
commits landed, `origin/main` equal to HEAD, 0 unpushed). Every load-bearing claim in
§3–§6 was re-derived from primary artifacts in the second pass; §4 records the pass-2
result per row and §9 lists what the corrections are. Where the two passes disagree,
**the pass-2 figures are the ones in the tables below** — they are the ones with the
method stated tightly enough to re-run.
**Method:** classification is evidence-first, with `scripts/crash-classifier.sh` run as a
corroboration (§5), not as the authority.

---

## 1. Verdict summary

| Population | n | Classification | Failure mode (one line) |
|---|---|---|---|
| bf-57nao4, exit −1, 2026-08-26T22:54:16Z | 1 | **FALSE_POSITIVE** (post-completion) | External SIGKILL-class kill 7 s after the bead was closed and 50.6 s into a third re-dispatch of a bead already completed exit=0 twice; scope memory peak 231 M |
| bf-12gb0r, exit −1, 2026-08-26T22:54:48Z | 1 | **FALSE_POSITIVE** (post-completion) | Same wave, 32 s later; deliverable independently verified in tree (`0d64373`); scope peak 366.5 M |
| domchk-e761cbd5, exit −1, 2026-09-02T10:04Z | 1 | **INFRASTRUCTURE** (external kill, early death) | SIGKILL at 45 s uptime, 0 turns — no work existed to lose |
| exit = 124 hard timeouts | 7 | **WORKFLOW_FAILURE** (time-cap, not a crash) | 5× 3600 s same-day cluster Sep 6 + 1× 3600 s Sep 2 + 1× 600 s Aug 17; workload-bound runs reaching the dispatch cap |
| exit = 1, terminal service-class API error | 129 | **SERVICE_FAILURE** | terminal census: `API Error: 503 no available server` ×105, `Connection refused` ×12, can't reach API server ×7, 429 rate-limit ×5 (§3.5; subtotal 129 is method-robust, buckets are not) |
| exit = 1, terminal turn-cap | 43 | **WORKFLOW_FAILURE** (`error_max_turns`, all at num_turns 31) | no terminal API error and **no detectable service-failure signature** in the body (§3.6 — the draft's cascade claim is retracted) |
| exit = 1, no marker string | 1 | undetermined from string markers | domchk-6129d0c7 — ends in a normal-looking `result` event, exit 1 anyway (§3.6) |
| CODE_DEFECT | **0** | — | zero panics / zero real goroutine dumps / zero application errors in any captured crash record |

The headline: **no event in this corpus is a domain-check code defect, and no captured
kill is a memory-exhaustion kill.** Real mechanisms are (a) external SIGKILL-class kills of
already-completed workers (bookkeeping-only loss), (b) one early-death external kill
(zero loss), (c) service-class API unavailability driving 74.6 % of the exit=1 failures
directly. The draft's fourth mechanism — turn-cap exits *caused* by service degradation —
was an artifact of a substring-matching method and is withdrawn (§3.6, §7).

---

## 2. What was classified

Scope is the workspace trace corpus (`.beads/traces/`, 1925 captures Aug 16 → Sep 7 as
cataloged by the upstream data-gathering bead; 1945 dirs live at pass-2 time — 20 new
captures landed after the catalog snapshot, none in the classified populations), plus the
stray top-level crash record. Per-event-class rationale follows; the evidence matrix with
verification status is §4.

### 3.1 bf-57nao4 — FALSE_POSITIVE

Rationale, each element re-verified live in both passes:

- **Work was complete before the kill.** The trace's final `agent_message` reads "✅ **Bead
  bf-57nao4 completed and closed** … This crash alert was a **duplicate false positive**"
  (25 events, tail re-read 2026-09-07). The event layer goes further than the upstream doc
  recorded: bf-57nao4 **completed exit=0 twice** (22:50:00 Z, 79.5 s; 22:53:01 Z, 108.2 s),
  was re-claimed 0.25 s after the second completion (22:53:02 Z), and the kill landed 50.6 s
  into that *third* re-dispatch — i.e. the destroyed attempt was re-verifying an
  already-closed bead. The bead store confirms the close directly: `bead show bf-57nao4` →
  **Status Closed, Updated 2026-08-26T22:54:09.7Z — closed 7 s before the kill**
  (22:54:16.03Z). (Pass-2 additions; see §7.1.)
- **No resource mechanism.** Scope teardown (`run-p2868370-i233525862.scope`, journalctl
  --user): "Consumed 2.995s CPU time, **231M memory peak**" at 18:54:15 EDT — re-verified
  byte-exact. The scope↔trace linkage is now proven from a second primary artifact: the
  trace's own `stderr.txt` first line names
  `Running as unit: run-p2868370-i233525862.scope`. No kernel oom-kill line and no
  systemd-oomd entry in the window.
- **Kill actor is external.** The needle worker's own message names SIGKILL / OOM /
  capacity governor; the same minutes show heavy teardown activity (2 `needle-worker@`
  teardowns + 14 total scope teardowns 18:53:40–18:55:30 EDT; 14 worker stderr logs carry
  the external-kill text).

### 3.2 bf-12gb0r — FALSE_POSITIVE

- **Deliverable verified present in the tree**, which is stronger than "the agent said so":
  `docs/adr/001-domain-watch-webhook-notifications.md` (added in commit `0d64373`, Aug 17),
  `internal/watch/`, and `internal/server/handlers_watch.go` all exist on disk today
  (checked 2026-09-07). The trace's closing summary ("ADR-001 Domain Watch feature has been
  fully implemented and verified", 93 events) matches the deliverable.
- **Same wave, same non-mechanism.** Scope `run-p2866338-i233523830.scope`: "Consumed
  10.638s CPU time, **366.5M memory peak**" at 18:54:48 EDT, matching the crash capture to
  the second. Crash event recorded in `.beads/events.jsonl` at 22:54:48 Z (dur 164.6 s).
  231–366 M peaks are 3–4 orders of magnitude below the repo-bloat-era GB-scale memcg kills
  and ~35× below the 12 GiB dispatch ceiling — memory exhaustion is excluded as the mechanism.

### 3.3 domchk-e761cbd5 — INFRASTRUCTURE (external kill, early death)

The one record that names its signal explicitly: `{"exit_code": -1, "signal": "SIGKILL",
"uptime_seconds": 45, "turns_completed": 0}` (re-read 2026-09-07). With zero turns
completed, nothing was lost and no application code executed to fail — the only honest
classification is the infrastructure/external-kill class. It is *not* a FALSE_POSITIVE:
no completion preceded the kill.

### 3.4 exit = 124 (7 events) — WORKFLOW_FAILURE (time cap)

All seven carry `timeout_reason: hard` and sit at exactly their configured limits
(6× 3600 s, 1× 600 s) — pinned-to-the-cap runs, not kills, not errors. The Sep-6 same-day
5× 3600 s cluster is workload-shaped (long verification runs under the glm-5.3 era),
which is a scheduling/scoping concern, not a crash and not a defect.

### 3.5 exit = 1 terminal service-class errors (129 of 173) — SERVICE_FAILURE

Terminal-cause census over all 173 exit=1 trace bodies (`stdout.txt`), classifying each
record by its **last** `API Error …` line; every one of the 173 records' stderr was also
checked — **none** carries an OOM or signal marker (catalog `stderr_oom`/`stderr_signal`
columns empty for all 173):

| Terminal error | n | share of 173 |
|---|---|---|
| `API Error: 503 no available server` | 105 | 60.7% |
| `Connection refused` | 12 | 6.9% |
| `Can't reach the API server` | 7 | 4.0% |
| `API Error: Request rejected (429) · Rate limit exceeded` | 5 | 2.9% |
| `Connection lost mid-response` (as terminal error) | 0 | — |
| **subtotal, service-terminal** | **129** | **74.6%** |

Method note (pass 2): the **subtotal is robust — 129 under every classification rule
tried** (first error, last error, presence). The *buckets* are not: classifying by *first*
API Error gives 503×104 / refused×12 / reach×7 / 429×5 / conn-lost×1, and counting
exact-string *presence* anywhere gives 105 / 14 / 7 / 5 / 2 with overlap. The draft's
103/12/7/5/2 split was one draw from that method spread; cite the subtotal, not the
buckets. Two records (`domchk-0f5ec6d6`, `domchk-b319800b`) contain
`Connection lost mid-response` mid-trace but terminate on other errors.

Transcript shape corroborates the mechanism: **all 129** service-terminal records end on
`is_error: true` results with the assistant message on `model: "<synthetic>"` — the
harness's own record of an API-level failure, at num_turns 1–42. This is the
service-class synchronized-wave signature (§6.4) reproduced from primary artifacts rather
than from monitor logs.

### 3.6 exit = 1 terminal turn-cap (43 of 173) — WORKFLOW_FAILURE; the cascade claim is RETRACTED

43 records end with no terminal API error and terminate on `error_max_turns` —
**every one at exactly `num_turns: 31`** (`stop_reason: tool_use`), the agent's turn budget
expiring mid-work. One further record (`domchk-6129d0c7`) has neither marker and ends in a
normal-looking `result` event at num_turns 20 while still exiting 1 — genuinely
undetermined by string markers; 43 + 1 + 129 = 173.

**Correction to the draft:** it claimed "43 of the 48 records with no `API Error` string
(90%) still contain a service-failure signature (503/429/connection) somewhere in the
trace body" and "97% of all exit=1 traces (168/173) carry a service signature somewhere".
Both figures do not survive a controlled re-run. Bare-string matching for `503`/`429` does
hit 44/44 of these records — but the hits are **substring matches inside UUIDs and
token-count fields** (e.g. `…8503-4af0…` inside `uuid` values), not API errors. With the
*specific* service-error strings used in §3.5 plus the broader real-world failure markers
(`Service Unavailable`, `overloaded`, `rate limit` as an error, `Connection error`,
`ECONNRESET`/`ECONNREFUSED`/`ETIMEDOUT`, `fetch failed`, `socket hang up`), the counts are:
**0 of 44** records carry a genuine service-failure signature. The honest census is:

| exit=1 population | n | share |
|---|---|---|
| terminal service-class error (§3.5) | 129 | 74.6% |
| terminal turn-cap, no service signature | 43 | 24.9% |
| undetermined | 1 | 0.6% |

So the turn-cap bucket stands classified **WORKFLOW_FAILURE on its own evidence**, and the
draft's "secondary to service degradation" cascade is **withdrawn as unproven** — it
remains a plausible hypothesis (a slow gateway could inflate turns without ever emitting a
terminal error string), but nothing in these 43 trace bodies demonstrates it. Lesson
recorded in §7: string-marker censuses over claude-code transcripts must use the full
error-string form; bare numbers fabricate findings out of UUID text.

### 3.7 Kernel memcg kills — synthetic harness only; zero live dispatches

The upstream §4.4 claim ("exclusively `safe-git-gc-run-*`") is **substantively right but
incomplete** — refined by reading `task_memcg` in the oom-kill records (Sep 6 → present,
pass-2 census, 96 kill lines):

| task_memcg scope family | mentions |
|---|---|
| `test-gc-memory-bounds` replay family (`bf1s6c3-push-*`, `bf1s6c3-gc-*`, `bf4yjq-crash-*`, `gcmb-bare-aggressive`) | 50 |
| `safe-git-gc-run-*` / `safe-git-gc-*` | 43 |
| `mw-oom-*` / `mw-diag-*` (memory-watch harness) | 3 |
| **needle dispatch scopes (`run-p*`)** | **0** |

(96 = 50 + 43 + 3, matching the killed-process census: `git` ×50 and `bash` ×46 — the
draft's table summed to 82 against its own 96 process count; the pass-2 table is
internally consistent.) Killed processes at tiny RSS (17–63 MB anon) — deliberate
bound-verification and death-command replays hitting small cgroup ceilings, matching the
documented test harnesses (`scripts/test-gc-memory-bounds.sh`, `scripts/memory-watch.sh`).
**No kernel kill in the journal targets a live agent dispatch.** (Method note so the next
reader doesn't repeat the detour: `Killed process` lines carry the *process* name, not the
scope; scope attribution lives in `task_memcg`, and the value must be split on `,` before
basename-matching, or comma-suffixed entries mis-bin.)

---

## 4. Evidence matrix

Every row re-verified in pass 2 (2026-09-07T18:11–18:30Z) unless noted; "pass-2 result"
differs from the draft where the draft was wrong.

| # | Claim | Evidence | Pass-2 verification |
|---|---|---|---|
| E1 | bf-57nao4 closed + summarized before kill | trace tail, final `agent_message`; bead store | ✅ re-read; **Closed, Updated 22:54:09.7Z — 7 s before the kill** |
| E2 | bf-57nao4 completed exit=0 twice, then re-claimed sub-second later, third attempt killed | `.beads/events.jsonl` 22:48–22:55 Z sequence | ✅ exact: complete 22:50:00.7 (79.5 s) / 22:53:01.9 (108.2 s), claim 22:53:02.2 (+0.25 s), kill 22:54:16 (50.6 s) |
| E3 | bf-12gb0r deliverable in tree | `internal/watch/`, `handlers_watch.go`, ADR-001 @ `0d64373` | ✅ filesystem + `git show 0d64373` |
| E4 | Scope peaks 231 M / 366.5 M, CPU 2.995 s / 10.638 s | journalctl --user teardown lines 18:54:15 / 18:54:48 EDT | ✅ byte-exact; **+ scope↔trace link proven** via trace `stderr.txt` naming `run-p2868370-i233525862.scope` |
| E5 | No kernel OOM, no systemd-oomd, Aug 26 window | journalctl -k / -u systemd-oomd | ✅ (0 oom-kill lines; oomd prints only "-- No entries --") |
| E6 | domchk-e761cbd5 SIGKILL, 45 s, 0 turns | stray `.beads/traces/domchk-e761cbd5.jsonl` | ✅ re-read, byte-exact |
| E7 | 7× exit=124 at hard caps | catalog `timeout_reason` column + per-trace metadata | ✅ 6× 3600 s + 1× 600 s, all `hard` |
| E8 | 129/173 terminal service errors | terminal-error census over all 173 trace bodies (§3.5) | ✅ subtotal exact; **buckets corrected** (105/12/7/5/0 terminal, not 103/12/7/5/2) |
| E9 | No OOM/signal marker in any exit=1 stderr | catalog stderr marker columns all empty | ✅ all 173 empty |
| E10 | Turn-cap exits | error_max_turns census (§3.6) | ✅ **43, not 44**, all num_turns 31; **cascade signature 0/44, not 43/48** — draft claim retracted |
| E11 | Zero panics / code defects | `panic:` in 0/173; `goroutine` census | ✅ panic 0; `goroutine` string in 12 records but **0 real goroutine dumps** (`goroutine <num>` pattern: 0) — source-text echoes, draft's "8" undercounted, conclusion stands |
| E12 | Kernel kills = synthetic harness scopes only, 0 dispatch scopes | `task_memcg` distribution, Sep 6 → present (§3.7) | ✅ refined: 96 lines = replay 50 + safe-gc 43 + mw 3 + dispatch 0 (draft table summed 82 vs its own 96) |
| E13 | Six-kill needle wave 22:52–22:57 Z Aug 26 | upstream §3.3 citation | ⚠️ NOT independently reproduced — needle stderr kill lines carry **no per-line timestamp** (14 worker stderr logs carry the external-kill text, none stamped); pass 2 adds: 2 `needle-worker@` + 14 total scope teardowns 18:53:40–18:55:30 EDT corroborate heavy external kill activity in the window |
| E14 | Classifier verdict FALSE_POSITIVE for both Aug-26 kills | `scripts/crash-classifier.sh` runs (§5) | ✅ FALSE_POSITIVE for both — **but bf-12gb0r needs no supplied window** (draft's §5 wrong, see below) |

---

## 5. Automated classifier outcome — corroboration, and where it actually breaks

`scripts/crash-classifier.sh` was run against both Aug-26 kills in pass 2:

- **bf-12gb0r: `FALSE_POSITIVE` with no caller-supplied window.** The classifier derives
  the incident window from the bead's **crash record in `.beads/events.jsonl`** — and
  bf-12gb0r has one (`"event":"crash","exit_code":-1"` at 22:54:48.794 Z, dur 164.6 s),
  so the automated path reaches the correct verdict unaided.
- **bf-57nao4: `UNKNOWN` — "Insufficient data to classify" / "trace slot does not describe
  the incident run" — with no window, and `FALSE_POSITIVE` with
  `CRASH_WINDOW_START/END` supplied.** The root cause is narrow and now identified:
  **needle recorded bf-57nao4's kill in the trace metadata (`outcome: crash`,
  `exit_code: -1`, captured 22:54:16.033 Z) but never wrote a crash record to
  `.beads/events.jsonl`** — the events layer for that bead holds claim/dispatch/complete
  only. The classifier's window derivation cannot see a crash that exists only in trace
  metadata.
- Compounding known defect `domchk-f6fff20f` (open): `crash-alert-manager.sh` reads the
  classifier's first stdout line as `CLASSIFICATION`, and that line is the `====` banner —
  so even the verdicts that do exist never reach the alert layer.

Implication for downstream consumers, corrected from the draft: the automated path is
**not** uniformly non-functional — it classifies correctly wherever the events layer
captured the crash. It fails for exactly the gap bf-57nao4 exposes (crash in trace
metadata, absent from events.jsonl), and then the banner-swallowing defect blocks
delivery of the verdicts that do exist. Evidence-first manual classification remains
necessary for the events-layer blind spot, and the events-layer capture inconsistency
itself is a new needle-side finding (§7.2).

---

## 6. Pattern comparison with similar crashes

1. **Post-completion external kill — the `bf-1ea4g` shape.** The classifier itself names
   the pattern. bf-1ea4g (57 attempts, closed 2026-08-13) established that exit −1 is a
   SIGKILL-class sentinel and that its alert stream was dominated by beads another worker
   had already finished. bf-57nao4/bf-12gb0r are the same shape with sharper evidence:
   the kill lands *after* close + summary, so the loss is bookkeeping only. Consistent
   with the standing FP-detection guidance ("verify the target bead's actual state first").
2. **What it is *not*: the repo-bloat memcg-OOM shape (bf-1s6c3 / bf-4yjq, 2026-08-12).**
   Those kills were GB-scale RSS under `CONSTRAINT_MEMCG` against an 18 GB repository.
   Aug-26 peaks are 231 M / 366.5 M and the repo is ~104 M with the pack-memory bound
   verified. Nothing in this corpus is a recurrence; today's kernel-side memcg kills are
   the *harness's own* bound checks (§3.7).
3. **What it is *not*: the push-side variant (bf-198ne).** Same exclusion — that was
   pack-objects RSS over an unpushed backlog; this workspace shows 0 unpushed backlog and
   no `git push` in any crash record.
4. **Service-class synchronized failure waves (Sep 1–2, Sep 7).** The fleet-level finding
   — "the dominant live signal is synchronized exit=1 waves, service-class" — is
   independently reproduced here from primary artifacts: 117 of the 173 exit=1 failures
   land on Sep 1–2, 76 of them inside the Sep 1 19:00 Z–Sep 2 10:59 Z band, with terminal
   strings naming the mechanism (`503 no available server`, 429 rate-limit). Per-bead
   classification of these is the wrong unit; classify the wave.
5. **systemd-oomd attribution — corrected by absence, and the citation fixed.** The
   draft cited `docs/crash-artifacts-bf-3561g.md` as "attributing the Aug-26 era to
   systemd-oomd at 94.71% pressure" — that document actually records **94.29%** pressure
   and describes **Aug-16** crashes (oomd killing a *different* dispatch scope at
   17:21:22 Z, 5 s before its crash #4), not the Aug-26 era. What stands unchanged is the
   substantive point: the journal has **no oomd entries for Aug 26** (E5), so no oomd
   mechanism is confirmed for the Aug-26 kills; the reproducible record there is external
   SIGKILL-class kills at sub-GB scope peaks.
6. **Alert-layer noise ratio.** 50,488 crash-monitor lines (`.beads/logs/crash-monitor.log`,
   re-counted) against 3 real crash records in the corpus — alert volume is not
   proportional to real kills, which is why every alert needs the bead-state check before
   investigation (the FALSE_POSITIVE filter's reason to exist).

---

## 7. New findings this classification adds

1. **bf-57nao4's kill hit a third re-dispatch of a twice-completed bead — and the third
   attempt closed the bead 7 s before dying** (E1/E2): complete 22:50:00 → complete
   22:53:01 → re-claim 22:53:02 (+0.25 s) → close 22:54:09.7 (bead store) → killed
   22:54:16. The immediate re-claim of a bead whose run had just exited 0 is a
   queue/re-dispatch-loop observation the upstream doc did not surface; handed to
   domchk-4f0b8b43 as a root-cause lead (needle re-dispatch behavior, not domain-check
   code — the close had already succeeded in-run before the kill).
2. **Needle's crash capture is inconsistent across the two layers**: bf-12gb0r's kill
   appears both in trace metadata and as a crash record in `.beads/events.jsonl`;
   bf-57nao4's kill exists **only** in trace metadata. That single missing event record is
   what defeats the classifier's window derivation (§5) — an actionable needle-side bug,
   also handed to domchk-4f0b8b43.
3. **Kernel-kill attribution refined and made self-consistent**: synthetic harness scope
   families are three (`safe-git-gc-*`, the `test-gc-memory-bounds` replay family, `mw-*`),
   totalling all 96 Sep-6→present kill lines; dispatch scopes: zero (§3.7).
4. **The needle external-kill lines carry no per-line timestamps** — any future citation
   of "N kills in window W" from those logs needs a different timestamp source (events
   layer or journal). E13 records the one upstream claim this pass could not reproduce.
5. **Terminal-error census of the exit=1 population** (E8/E9/E10): the corpus-level
   failure-mode split (129 service-terminal / 43 turn-cap / 1 undetermined) and the
   empty-stderr negative are new quantitative facts, with the grep method documented for
   re-runs — including its failure mode.
6. **Method hazard, recorded because it produced a real error here**: bare-number
   greps (`503`, `429`) over claude-code transcript bodies match UUIDs and token-count
   fields and will fabricate a "service signature" in essentially every record. The
   draft's 90%/97% cascade figures came from exactly that. Any future census in this
   workspace must match the full error-string form and spot-check hit contexts.
7. **Classifier gap narrowed** (§5): the automated path works where the events layer
   holds the crash record (bf-12gb0r); it fails only at the trace-metadata-only gap
   (bf-57nao4), and the `domchk-f6fff20f` banner defect separately blocks delivery of
   existing verdicts.

---

## 8. Handoff to domchk-4f0b8b43 (root cause)

- Synthesis should take the three-mechanism headline from §1 and attach two needle-side
  leads to the Aug-26 wave's *cause* question (why did the queue re-claim a closed bead
  within 0.25 s, what external actor killed workers in that window, and why did the events
  layer miss bf-57nao4's crash record while capturing bf-12gb0r's).
- The FALSE_POSITIVE verdicts for bf-57nao4/bf-12gb0r are final at this layer: both beads'
  work is delivered and closed; no further investigation bead should be spawned for them.
- Do **not** carry forward the draft's "turn-cap secondary to service degradation"
  cascade — retracted at §3.6. If synthesis wants to test it, the test is: correlate the
  43 turn-cap runs' per-turn latency/TTFT series against the 503-wave windows, not string
  greps.
- Reuse, don't re-derive: the censuses in §3.5–§3.7 are reproducible from the committed
  catalog `docs/crash-data-traces-catalog-2026-09-07.csv` plus the live `.beads/traces/`
  bodies, with methods stated inline.

---

## 9. Pass-2 correction record

For auditability — what the second pass changed relative to the 18:06 Z draft:

| Draft claim | Pass-2 finding | Where |
|---|---|---|
| exit=1 split 129 service / 44 turn-cap / "0–5" undetermined; "48 no-API-error" | 129 / 43 / 1 (=173); the draft's own 129+44 vs "48" was inconsistent | §1, §3.6 |
| "43/48 carry service signature mid-trace (90%)"; "97% (168/173) overall" | retracted — bare `503`/`429` matches are UUID/token-field substrings; genuine signatures: 0/44 | §3.6, §7.6 |
| Terminal buckets 503×103, conn-lost×2 | terminal(last-error) 503×105, refused×12, reach×7, 429×5, conn-lost 0; subtotal 129 robust, buckets method-sensitive | §3.5 |
| Automated classifier UNKNOWN for both beads without a window | bf-12gb0r classifies FALSE_POSITIVE unaided; only bf-57nao4 needs a window — events-layer crash-record gap identified as the cause | §5, §7.2 |
| Kernel table 43/36/3/0 (=82) | 43/50/3/0 (=96, matches the 50 git + 46 bash process census) | §3.7 |
| bf-3561g doc "attributes the Aug-26 era to oomd at 94.71%" | that doc says 94.29% and is about Aug-16, not Aug-26; the no-oomd-for-Aug-26 conclusion stands on E5 | §6.5 |
| trace event counts 26 / 94 | 25 / 93 | §3.1, §3.2 |
| "num_turns from 1 to 31" (service-terminal) | 1 to 42 | §3.5 |
| re-claim "1 s after" completion | +0.25 s | §3.1 |
| `goroutine` in 8 records | in 12, real dumps 0 | E11 |
| Classification window "18:00–19:30Z", repo at `0f4d659` | draft authored 18:00–18:06Z (window end was never reached — dispatch 2 was released at 18:08 Z); pass 2 18:11–18:30Z at HEAD `d82b6a2`/`1f56d9c` | header |

Everything else in the draft was reproduced exactly and stands: the two FALSE_POSITIVE
verdicts, the INFRASTRUCTURE early-death verdict, the WORKFLOW_FAILURE time-cap verdict,
the service-wave dates (117 Sep 1–2 / 76 in band), the empty-stderr negative, the
scope peaks (byte-exact), the deliverable-in-tree checks, the 50,488-line monitor volume,
and the catalog population (1925 rows: 1726 success / 173 failure / 7 timeout / 2 crash).
