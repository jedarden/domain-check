# Root cause analysis report — bf-2ildm, delivered to alert parent bf-o6vbwl (2026-08-13 alert storm)

**Reporting dispatch:** domchk-**d932c30c** ("Report findings to parent bead
bf-o6vbwl") — the final, synthesis step of the chain; blocks the original
2026-09-02 investigation bead **domchk-ca1f6d53** and is blocked by the
classification child domchk-0022f45f.
**Crash target:** bf-2ildm — "Extract GitHub-specific commits" (**Closed**,
rev 7; successful retry 2026-08-16T22:28:44Z, exit 0, 85,327 ms; closed
22:44:38.873Z)
**Alert parent:** bf-o6vbwl — Open, rev 28; ledger **row 34 of 38**, carried
stamp 2026-08-13T15:36:14.415407055Z (a post-kill handler heartbeat, not a
death instant)
**Inputs (the three children this report compiles):**

| Child | Role | Deliverable |
|---|---|---|
| domchk-**fe9f7c5b** (Closed rev 4) | context summary | [`docs/crash-context-bf-2ildm-2026-08-13.md`](../crash-context-bf-2ildm-2026-08-13.md) (e9ab3d4) |
| domchk-**167289ae** (Closed rev 4) | crash-log / trace summary | [`docs/crash-log-findings-bf-2ildm-2026-09-07.md`](../crash-log-findings-bf-2ildm-2026-09-07.md) (af44fc6) |
| domchk-**0022f45f** (Closed rev 4) | classification | [`docs/investigations/bf-2ildm-classification-report-domchk-0022f45f-2026-09-07.md`](bf-2ildm-classification-report-domchk-0022f45f-2026-09-07.md) (5d9459e) |

Standing records this report builds on and does not re-derive:
[`bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md`](bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md)
(superseding RCA), evidence bundle `docs/crashes/bf-2ildm/` (domchk-ea755548).

## Verdict

**Crash type: FALSE_POSITIVE — at the alert level, which is the level this
chain's classification was asked about. The kill level is separately
INFRASTRUCTURE.** Both hold; neither cancels the other, and neither may be
read as "no crash happened."

| Level | Question | Classification | Confidence |
|---|---|---|---|
| **Alert** | Did the 38 alerts imply something true? | **FALSE_POSITIVE** — each fired at a real kill but implied lost work; **no work was lost**; the dedup gate itself now says DUPLICATE | **HIGH** |
| **Kill** | What killed the 38 attempts? | **INFRASTRUCTURE** — the repository-bloat regime (memcg-OOM inside needle's 12 GiB per-dispatch scope) | regime **HIGH**; mechanism **MODERATE** (assigned by regime match — no Aug-13 kernel record survives) |
| Code | Domain-check defect? | **CODE_DEFECT ruled out** | **HIGH** — zero panics/stack traces across all 43 attempts; the identical task succeeded in 85 s on 2026-08-16 |

**Root cause of the false positive, one line:** the crash-alert layer had **no
lifecycle** — alerts were created at kills and never re-validated against the
target's state afterwards, so when bf-2ildm resolved three days later the 38
alerts kept presenting a resolved crash as a live one.

## 1. Executive summary

1. **The work survived.** bf-2ildm's task (GitHub-side `git log` extraction,
   step 3 of a branch-divergence chain) completed successfully on
   2026-08-16T22:28:44Z (exit 0, 85,327 ms) and the bead closed at
   22:44:38.873Z. Nothing needed re-doing; the storm cost ~2.3 h of wall
   clock across 38 killed attempts, **not work**.
2. **The kills were real.** 43 attempts are indexed in
   `docs/crashes/bf-2ildm/attempt-index.tsv` = **38 × exit −1** (2026-08-13,
   13:37:24Z → 15:53:28Z, all mid-task) + 4 × exit-124 (600 s caps, no
   alerts) + 1 × exit-1 at 19 ms (quarantined at `failure_count: 5`). Exit −1
   is needle's died-without-exit-code sentinel (`code().unwrap_or(-1)`); a
   true SIGKILL would encode 137. It is **not placeholder data**.
3. **The alerts were individually correct at fire time and false in
   aggregate afterwards.** The 38 alert beads map **1:1 to the 38 kills**
   (zero duplicates, zero orphans *inside* the storm); each stamp lands
   **7.7–29.4 s** after its kill — a post-kill handler heartbeat, never the
   death instant. What was missing was any mechanism to retire an alert when
   its premise expired.
4. **The 2026-09-02 "FINAL DETERMINATION" still recorded in bf-o6vbwl's notes
   is superseded.** It concluded "bf-2ildm did NOT crash," that exit −1 was
   placeholder data, and that an alert 3+ days before completion was
   "physically impossible." Its alert-level verdict stands; its kill-level
   reasoning was an evidence-selection error driven by a single-slot trace
   that holds only the successful Aug-16 retry. §5 restates each of its
   claims with current standing.
5. **The alert-layer defects it did name are real and fixed**, plus a sixth
   the 2026-09-02 corpus could not see (§6). Verified live at HEAD f6ee892
   today: FIX 1–6 present in `scripts/crash-alert-manager.sh`, classification
   wiring fixed and pushed (8cc1172), `test-crash-alert-fixes.sh` **13/13**,
   `test-closed-bead-filter.sh` **7/7**.

## 2. Why "FALSE_POSITIVE" — and in what sense

Read the worked example (attempt 19 → alert bf-66sw7c, the chain's own
ancestor), all UTC, re-read first-hand from the bundle for this report:

| Instant | Event |
|---|---|
| 2026-08-13T14:38:24.455Z | attempt 19 claimed |
| 14:40:29.551762Z | **attempt 19 killed** (exit −1, 124,759 ms, mid-task) |
| 14:40:42.628685Z | post-kill handler heartbeat = the alert's carried stamp (**+13.1 s**) |
| 14:40:42.642439Z | alert bead **bf-66sw7c** created (+14 ms after the stamp) |
| 2026-08-16T22:28:44Z | **successful retry** (exit 0, 85,327 ms) |
| 2026-08-16T22:44:38Z | bf-2ildm **closed** |

At 14:40:42 on Aug-13 the alert was *doing its job*: bead open, mid-task,
worker just killed. **The alert became a false positive retroactively** — on
2026-08-16, when the target succeeded and closed and the "work was lost,
investigate" premise expired with nothing to retire it. That is a defect of
alert *lifecycle*, not of alert *firing*: an alerting layer silent through 38
real kills would be broken in the other direction.

Corroborated live at the alert layer today: `./scripts/alert-deduplication.sh
check bf-66sw7c` → **"DUPLICATE: crash target bf-2ildm is already resolved"**;
`./scripts/crash-classifier.sh bf-2ildm` → **UNKNOWN** with the provenance
warning "trace slot does not describe the incident run." The dedup gate is
the alert layer itself now agreeing the alert is stale; the classifier's
UNKNOWN is a *retention* artifact, not evidence of no crash (§6, gap 2).

## 3. Root cause determination

### 3.1 Alert level (the classification this report delivers)

The crash-alert generation system had **no lifecycle**. Concretely, at storm
time there was: no closed-bead filter (FIX 1/5), no completion awareness
(FIX 4/6), no dedup gate (FIX 2/3), no cooldown, and — once classification
was later added — a wiring bug that kept the FALSE_POSITIVE branch dead code
(fixed at 8cc1172). Any one of these would have retired bf-66sw7c on first
processing; with none of them, 38 alerts for one target's storm survived to
be processed as if the target were still crashing. **The kill alerts were
true; the crash *case* they imply was false.**

### 3.2 Kill level (context — owned by the superseding RCA)

The 38 attempts were killed by the **repository-bloat regime**: ~18 GB of
loose objects from 17+ identical 237 MB `.beads/*.jsonl` snapshots still
tracked, plus a 422-commit unpushed backlog, driving git/bead-state
operations past the 12 GiB `MemoryMax` of needle's per-dispatch scope
(memcg-OOM SIGKILL). Derivation, ruled-out alternates, and the live
re-verification that the regime is repaired and holding are in
[`bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md`](bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md)
§2 and §5; the mitigation battery is re-proven first-hand in
[`docs/mitigation-implementation-bf-2ildm-2026-08-13.md`](../mitigation-implementation-bf-2ildm-2026-08-13.md).
**Nothing new is owed on the mechanism.** Same-day census for scale: bf-65lsdu
127 kills, bf-1ea4g 56, bf-4k2ws 55, **bf-2ildm 38**, bf-1s6c3 22.

**Ruled out for this bead:** CODE_DEFECT (no panics in 43 attempts; identical
task succeeded on retry), SERVICE_FAILURE (mid-task signal losses, not 5xx /
timeout exits), WORKFLOW_FAILURE (exit would be 1 `error_max_turns`, not −1),
bead-store corruption (no schema errors; deaths precede any store mutation).

## 4. Evidence chain from the three child beads

**domchk-fe9f7c5b (context summary of domchk-b049ea9e) → supports the
alert-level FALSE_POSITIVE and the work-not-lost premise.** Establishes the
task context (GitHub `git log` extraction, crash-irrelevant), the target's
state (Closed rev 7, verified live), and the claim-by-claim adjudication of
the 2026-09-02 context findings — two of its five evidence items stand, two
are superseded, one (the "physically impossible" timestamp argument) is
reversed outright.

**domchk-167289ae (crash-log summary of domchk-a5a51981) → supports the kill
level and the supersession of the 09-02 reasoning.** Reproduces the surviving
trace byte-for-byte (`metadata.json` exit 0 / `success` / 85,327 ms /
2026-08-16T22:28:44Z; `stderr.txt` three benign warnings; `trace.jsonl` 47
events, exactly one failed result and it is a benign `bead dep show` usage
error) and names why that clean trace misleads: **it is the retry's trace** —
single-slot retention overwrote the crash era. Also re-derives the attempt
census (43 = 38 + 4 + 1), the 1:1 alert mapping, the 7.7–29.4 s stamp gaps,
and the sentinel-not-signal correction.

**domchk-0022f45f (classification) → the two-level verdict and the per-bug
standing table.** Classifies alert FALSE_POSITIVE (HIGH) / kill
INFRASTRUCTURE (regime HIGH, mechanism MODERATE) / CODE_DEFECT ruled out
(HIGH), restates the six alert-layer defects each with fix status, and names
the two evidence-path defects that misdirected the 09-02 investigation.

**Both summary children agree on the load-bearing point from different
directions** — bead state says the target closed; the trace archive says the
surviving run was a clean success. Together: false positive, located in alert
lifecycle, not in fabricated crash data.

### 4.1 Dispatch-premise correction — the phantom child-bead ID

This dispatch's task text (and domchk-0022f45f's, which inherited it) names
child bead **`domchk-fe9f7f7b`**. **No such bead exists** — re-verified live
for this report against the full store (3,441 beads): 0 occurrences as an
issue ID, 0 as a dependency edge, `bead show` → "Issue not found". The
intended bead is **`domchk-fe9f7c5b`** — one transposition away (`…f7f7b` vs
`…f7c5b`), it is the bead that actually blocks domchk-167289ae, and its
title/task is exactly the role the task text assigns. This report compiles
from it. No store mutation is made: the phantom ID lives only in prose, and
the dependency graph is already correct. The 2026-09-02 dispatch that created
this chain introduced it; nothing in the alert system depends on it.

## 5. Standing of the 2026-09-02 "FINAL DETERMINATION" recorded in bf-o6vbwl's notes

The parent bead's notes (rev 28) still carry the 2026-09-02 determination
verbatim, above this report's appendix. Its alert-level verdict stands; five
of its factual claims do not. This table is the dated correction the parent's
notes now also carry.

| 2026-09-02 claim (still in the parent's notes) | Standing 2026-09-07 |
|---|---|
| "Bead bf-2ildm did NOT crash" | **Superseded** — 38 real exit −1 kills on 2026-08-13 (`attempt-index.tsv`, re-census for this report) |
| "Actual Exit Code: 0 (SUCCESS)" for the reported run | **Superseded provenance** — the trace's exit 0 (85,327 ms) is the 2026-08-16 **retry**; the single-slot trace overwrote the crash era, so no crash-era trace survives |
| "Alert generated 3+ days BEFORE completion (IMPOSSIBLE)" | **Reversed** — alerts fired 7.7–29.4 s after real kills, mid-task, bead open; they are 3 days before *the completion that later made them stale* |
| "Incorrect Placeholder Data: Exit code −1" | **Superseded** — needle's died-without-exit-code sentinel; 38 real kills produced it (a true SIGKILL encodes 137) |
| "21+ duplicate alerts for same event" | **Reframed** — 38 alert beads map 1:1 to 38 distinct kills, 0 orphans; the duplication was *re-processing of resolved targets*, not intra-storm duplication |
| "No crash artifacts exist" (logs, core dumps, stack traces) | **Non-probative** — journald's single boot begins 2026-08-15, `.beads/logs/` begins 2026-09-02; the absence is a retention gap, not exculpatory |
| "FALSE POSITIVE from systematic bugs in the crash alert generation system" | **Stands at the alert level** — with the mechanism corrected: no alert *lifecycle* (§3.1), not fabricated data |
| "12/12 tests passing" | **Grown, not stale** — live today: 13/13 (suite grew as checks were added) and the closed-bead functional suite 7/7 |

## 6. Recommended fix approach for the crash alert system

### 6.1 Fixed and verified (no further action)

| Fix | Where | Verified live 2026-09-07 |
|---|---|---|
| Closed-bead filter (FIX 1/5) | `scripts/crash-alert-manager.sh:294` | `test-closed-bead-filter.sh` **7/7** — fabricated closed-target trace produces no alert |
| Completion awareness + exit-code validation (FIX 4/6) | `:218`, `:317`, `:354` | `test-crash-alert-fixes.sh` **13/13** |
| Dedup + processed-alerts tracking (FIX 2/3) | `:317`, `:602` | same suite; dedup gate re-run → DUPLICATE |
| 300 s alert cooldown | manager | cascade replay 10/10 (2026-09-07, per CLAUDE.md record) |
| **CLASSIFICATION wiring** — the manager read the classifier's `====` banner as the verdict, so the FALSE_POSITIVE branch was dead code and the cooldown keyed a constant | `:401-402` — anchored classification-token grep with `head -1` fallback | fixed at **8cc1172**, **pushed to origin/main** (`merge-base --is-ancestor` ✅); tracking bead **domchk-f6fff20f Closed** (rev 4, 19:39:31Z, "the defect this bead tracks is FIXED and verified at HEAD") |

Fix markers present at HEAD f6ee892: `grep -n "CRITICAL FIX"
scripts/crash-alert-manager.sh` → FIX 1–6 at lines 294, 210, 317, 602, 354,
218.

### 6.2 Residual gaps (each owned or explicitly unowned)

1. **No stale-alert sweep** — the concrete gap this incident still exposes:
   **10 of the 38 storm alert beads remain unresolved today** (live recount
   from the store: 28 closed / 7 open / 3 in_progress — bf-o6vbwl itself is
   one of the open ones, 25 days after its target resolved). Owned:
   **domchk-4a05973f** ("Verify and close the 10 remaining stale bf-2ildm
   storm alerts", Open, P3). Recommended generalization: a periodic sweep that
   re-runs the dedup gate over open alert beads and auto-retires those whose
   target is resolved.
2. **Single-slot trace retention** — `.beads/traces/<bead>/` keeps only the
   last attempt, so the Aug-16 retry overwrote every crash-era trace. This
   one defect produced the classifier's UNKNOWN, the "exit 0 vs −1"
   fabrication reading, and the wrong 09-02 RCA. Compensating control in
   place: retrieval bundles with per-attempt indexes (domchk-ea755548).
   **Candidate improvement, no owner yet:** multi-slot retention keyed by
   (bead, attempt).
3. **Alert-stamp provenance** — alert beads carry the post-kill handler
   heartbeat (7.7–29.4 s after the kill), never the death instant.
   Documented control: cite kills from `attempt-index.tsv`, never the bead
   stamp. **Candidate:** stamp the kill instant on the alert bead at creation.
4. **Pre-Aug-16 automated classification returns UNKNOWN** — `events.jsonl`
   begins 2026-08-16 and no Aug-13 kernel record survives, so the classifier
   can never confirm that storm from primary records. A trace that says
   `outcome: crash` against a silent `events.jsonl` should classify
   **INFRASTRUCTURE** with a mechanism note rather than bare UNKNOWN
   (proposed fix: `docs/crash-fix-strategy-domchk-3b605127-2026-09-07.md`
   §3.2).
5. **Per-clone pre-commit hook** — the 10 MB repo-size gate is per-clone; a
   fresh clone is unprotected until `setup-git-hooks.sh install`. Documented
   in CLAUDE.md; no code change proposed.

### 6.3 Not owed

Nothing on the kill mechanism (mitigation battery verified holding: repo
~105 MB, 0 unpushed, 0 tracked `.beads/` files, pack-memory bound within
ceiling, health exit 0). Nothing on domain-check code — no defect has ever
been found in any investigation of this workspace.

## 7. What this report closes and does not close

**Closes:** this chain's reporting obligation. The synthesis the task asks
for is delivered here and appended to bf-o6vbwl's notes.

**Does not close:**
- **bf-o6vbwl itself** — left **Open** deliberately: closing the 10 stale
  storm alerts (including this one) is owned by **domchk-4a05973f**, so the
  parent's disposition is made in one place against one verified checklist.
  This report supplies the classification that closure will cite.
- **domchk-ca1f6d53** (the 2026-09-02 investigation bead this dispatch
  blocks) — receives this report as its input.
- Gaps 1–5 above, each with its owner stated in §6.2.

## 8. First-hand verification battery (run 2026-09-07, HEAD f6ee892, repo cwd)

| Check | Command | Result |
|---|---|---|
| Target state | `bead show bf-2ildm` | **Closed**, rev 7 (updated 2026-09-07T17:47:17Z) |
| Parent state | `bead show bf-o6vbwl` | **Open**, rev 28 |
| Phantom input | `bead show domchk-fe9f7f7b` | **"Issue not found"**; real first child domchk-fe9f7c5b resolves (§4.1) |
| Child states | `bead show` ×3 | fe9f7c5b / 167289ae / 0022f45f all **Closed rev 4** |
| Attempt census | `awk -F'\t'` over `attempt-index.tsv` col 6 | **43 = 38 × −1 + 4 × 124 + 1 × 1** |
| Alert mapping | distinct col 12 among −1 rows | **38 distinct beads, 0 empty** |
| Stamp gaps | col 13 min/max | **7.7 s / 29.4 s** |
| Worked example | row 19 | claim 14:38:24.455725579Z → kill 14:40:29.551762239Z, −1, 124,759 ms → bf-66sw7c, **13.1 s** |
| Alert ledger | `wc -l crash-alert-ledger.tsv`; row for bf-o6vbwl | **39 lines = 38 + header**; bf-o6vbwl row 34, stamp 15:36:14.415407055Z, status open |
| Successful retry | read `.beads/traces/bf-2ildm/metadata.json` | `exit_code 0`, `success`, 85,327 ms, captured 2026-08-16T22:28:44.172Z |
| Dedup gate | `./scripts/alert-deduplication.sh check bf-66sw7c` | **DUPLICATE: crash target bf-2ildm is already resolved** |
| Classifier | `./scripts/crash-classifier.sh bf-2ildm` | **UNKNOWN** + "trace slot does not describe the incident run" (expected; §2) |
| Alert-fix suite | `./scripts/test-crash-alert-fixes.sh` | **13/13 passed**, exit 0 |
| Closed-bead filter suite | `./scripts/test-closed-bead-filter.sh` (repo cwd — it is cwd-sensitive) | **7/7 passed**, exit 0 |
| Wiring fix pushed | `git merge-base --is-ancestor 8cc1172 origin/main` | **pushed**; anchored-token grep present at `scripts/crash-alert-manager.sh:401-402` |
| Tracking bead | `bead show domchk-f6fff20f` + checkpoint | **Closed** rev 4 (19:39:31Z) — newer than the classification report's "still open" |
| Live alert census | `bead list --json --limit 100000`, filtered by the 38 ledger IDs | **28 closed / 7 open / 3 in_progress** (matches domchk-970b6ca1) |
| Remediation owner | `bead show domchk-4a05973f` | **Open**, P3 — owns closing the 10 stale alerts |
| Working tree | `git log origin/main..HEAD --oneline \| wc -l` | **0 unpushed** |

## 9. Sources

- Child deliverables: [`docs/crash-context-bf-2ildm-2026-08-13.md`](../crash-context-bf-2ildm-2026-08-13.md);
  [`docs/crash-log-findings-bf-2ildm-2026-09-07.md`](../crash-log-findings-bf-2ildm-2026-09-07.md);
  [`docs/investigations/bf-2ildm-classification-report-domchk-0022f45f-2026-09-07.md`](bf-2ildm-classification-report-domchk-0022f45f-2026-09-07.md)
- Superseding RCA: [`docs/investigations/bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md`](bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md)
  (§2 kill mechanism, §3 alert-level root cause, §4 supersession, §5 live
  verification, §6 residual gaps)
- Evidence bundle: `docs/crashes/bf-2ildm/` — `attempt-index.tsv`,
  `crash-alert-ledger.tsv`, `trace-archive-current-state.json` (domchk-ea755548)
- Mitigation: [`docs/mitigation-implementation-bf-2ildm-2026-08-13.md`](../mitigation-implementation-bf-2ildm-2026-08-13.md);
  UNKNOWN-classification fix proposal:
  [`docs/crash-fix-strategy-domchk-3b605127-2026-09-07.md`](../crash-fix-strategy-domchk-3b605127-2026-09-07.md)
- Live (this dispatch, 2026-09-07): store (`bead show` / `bead list --json`),
  `.beads/traces/bf-2ildm/metadata.json`, both alert suites, dedup gate,
  classifier, `scripts/crash-alert-manager.sh` at HEAD f6ee892
- Superseded (context only): `docs/investigations/root-cause-determination-bf-2ildm-2026-09-02.md`
  (frozen copy in `docs/archive/crash-investigations/`),
  `docs/investigations/bf-2ildm-final-investigation-report-2026-09-02.md` —
  the source of the parent's 2026-09-02 notes and of this dispatch's task text
