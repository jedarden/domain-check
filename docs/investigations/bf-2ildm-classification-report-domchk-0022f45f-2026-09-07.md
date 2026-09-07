# Classification report: bf-2ildm (2026-08-13 alert storm)

**Classification dispatch:** domchk-0022f45f (2026-09-07)
**Crash target:** bf-2ildm — "Extract GitHub-specific commits" (**Closed**, rev 7;
successful retry 2026-08-16T22:28:44Z, exit 0, 85,327 ms)
**Alert beads classified:** 38, created 2026-08-13T13:37Z → 15:53Z; the chain's
parent is **bf-o6vbwl** (ledger row 34/38), the worked example is **bf-66sw7c**
(created 14:40:42.642Z)
**Inputs:** domchk-**fe9f7c5b** ("Extract and summarize crash context findings",
Closed rev 4 → [`docs/crash-context-bf-2ildm-2026-08-13.md`](../crash-context-bf-2ildm-2026-08-13.md))
and domchk-**167289ae** ("Extract and summarize crash log findings", Closed rev 4 →
[`docs/crash-log-findings-bf-2ildm-2026-09-07.md`](../crash-log-findings-bf-2ildm-2026-09-07.md))
— see §6 for the ID discrepancy in this dispatch's own task text.
**Position in the chain:** context summary (fe9f7c5b) → crash-log summary
(167289ae) → **this classification** → report-to-parent (domchk-d932c30c, open).

## Verdict

**Crash type: FALSE_POSITIVE** — at the alert level, which is the level this
classification is asked about. The kill level is separately **INFRASTRUCTURE**.
The two answer different questions and both hold; neither cancels the other.

| Level | Question | Classification | Confidence |
|---|---|---|---|
| **Alert** | Did the alerts imply something true? | **FALSE_POSITIVE** — 38 alerts fired at real kills, each implying lost work; **no work was lost** | **HIGH** |
| **Kill** | What killed the 38 attempts? | **INFRASTRUCTURE** — the repository-bloat regime (memcg-OOM inside needle's 12 GiB per-dispatch scope) | regime **HIGH**; mechanism **MODERATE** (assigned by regime match — no Aug-13 kernel record survives) |
| Code | Domain-check defect? | **CODE_DEFECT ruled out** | **HIGH** — zero panics/stack traces in any of the 43 attempts; the identical task succeeded in 85 s on 2026-08-16 |

This matches the standing records it builds on: the per-instant classification
docs (`docs/crash-classification-bf-2ildm-2026-08-13-14-40.md`,
`…-15-01.md`), the root-cause determination
(`bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md`), and the
correction appendix on bf-2ildm itself (rev 7). It **refines, and partly
reverses, this dispatch's own task text**, which was written 2026-09-02 — see
§4.

**Root cause of the false classification, one line:** the alert layer generated
38 individually-correct kill alerts and then had **no mechanism to retire them
when their premise expired**, so a resolved target kept presenting as a live
crash — and the two evidence-path defects below then steered the first
investigation (2026-09-02) to the wrong conclusion about the kills themselves.

## 1. Why the alert was a false positive — and in what sense

The honest statement is **not** "the alert fired when nothing was wrong." Read
the timeline (all UTC, from `docs/crashes/bf-2ildm/attempt-index.tsv`, re-read
by this dispatch):

| Instant | Event |
|---|---|
| 2026-08-13T14:38:24.455Z | attempt **19** claimed (the RCA's dated ordinal correction: 19, not 20 — every timestamp matches row 19 exactly) |
| 14:40:29.551762Z | **attempt 19 killed**, exit −1, 124,759 ms — mid-task |
| 14:40:42.628685Z | crash-handler post-kill heartbeat = the alert's carried stamp (**+13.1 s**) |
| 14:40:42.642439Z | alert bead **bf-66sw7c** created (+14 ms after the stamp) |
| 2026-08-16T22:28:44Z | **successful retry** — exit 0, 85,327 ms, the only run the single-slot trace retains |
| 2026-08-16T22:44:38Z | bf-2ildm **closed**, all acceptance criteria met |

At 14:40:42 on Aug-13 the alert was *doing its job*: the bead was open, mid-task,
and its worker had just been killed. **The alert became a false positive
retroactively** — three days later, when the target succeeded and closed, the
"work was lost, investigate" premise it carries expired and nothing retired it.
That is the precise defect, and it is a defect of *alert lifecycle*, not of
*alert firing*.

Corroborated at the alert layer today: `./scripts/alert-deduplication.sh check
bf-66sw7c` → **"DUPLICATE: crash target bf-2ildm is already resolved"**; and
`./scripts/crash-classifier.sh bf-2ildm` → **UNKNOWN** with the provenance
warning "trace slot does not describe the incident run … do not quote this
trace as crash evidence" (both re-run live 2026-09-07 for this report). The
dedup gate is the alert layer *itself* now agreeing the alert is stale.

Cost accounting: the storm cost ~2.3 h of wall clock across 38 killed attempts —
**not work**. Nothing needed re-doing; the 2026-08-16 retry reproduced the
deliverable.

## 2. The alert-generation bugs — each with its current standing

The 2026-09-02 investigation listed five "systematic bugs in the crash alert
generation system." That list survives only partly: two items were real
alert-layer defects, three were misreadings of this incident. Both real ones —
plus a sixth the 2026-09-02 corpus could not see — are fixed and verified.

| # | Bug as claimed (2026-09-02) | Standing for *this* incident | As an alert-layer defect | Fix status |
|---|---|---|---|---|
| 1 | **Premature alert generation** — "fired 3+ days BEFORE completion, physically impossible" | **Reversed.** Alerts fired 7.7–29.4 s after real kills, mid-task, bead open. Nothing was "before completion"; the alerts were 3 days before *the completion that later made them stale*. Alert-at-kill is correct behavior | — (not a defect; the premise was a timeline misread) | n/a |
| 2 | **Placeholder exit code −1** — "never validated against trace metadata" | **Superseded.** Exit −1 is needle's died-without-exit-code sentinel (`code().unwrap_or(-1)`; a true SIGKILL encodes 137), and 38 real kills produced it. The trace's exit 0 is the *Aug-16 retry*, not a contradiction | — | n/a |
| 3 | **No bead-status validation** (closed-bead filter) | — | **Real.** Alerts stayed actionable after the target resolved | **Fixed** (FIX 1/5). Live: `test-closed-bead-filter.sh` **7/7**, functional test fabricates a closed-target trace and asserts no alert |
| 4 | **No timestamp validation** | — | **Real** as a class: no validation that an alert's premise still holds, and no record that the carried stamp is a handler heartbeat (+7.7–29.4 s), never the death instant | **Fixed** for completion-awareness (FIX 4/6); stamp provenance remains a **documented residual gap** — cite kills from the per-attempt index, never the bead stamp |
| 5 | **No duplicate prevention** — "21+ duplicate alerts" | **Reframed.** The 38 alert beads map **1:1 to 38 distinct kills** — zero duplicates, zero orphans inside the storm. What felt like duplication was *re-processing* of resolved targets | **Real** as re-processing/dedup | **Fixed** (FIX 2/3 + 300 s cooldown). Live: `test-crash-alert-fixes.sh` **13/13**; cascade replay (six genuine kills in one minute → one alert) 10/10 |
| 6 | *(not visible to the 2026-09-02 corpus)* | — | **Real. `crash-alert-manager.sh` read the classifier's `====` banner as `CLASSIFICATION`**, so the FALSE_POSITIVE branch was dead code, the cooldown keyed on a constant string, and `crash-history.jsonl` recorded a garbage class | **Fixed** at HEAD (8cc1172, domchk-701bcfa5): the manager greps an anchored classification token with a `head -1` fallback (`scripts/crash-alert-manager.sh:401-402`); wiring verified pushed to `origin/main`. Tracking bead **domchk-f6fff20f still open** with its owner |

Fixes 1–6 are all present in the working tree at HEAD 7e34c2f (`grep -n
"CRITICAL FIX" scripts/crash-alert-manager.sh`), both suites pass from the repo cwd
(run live for this report), and the classification wiring at line 401 now
yields a real token — this report's §1 classifier run is the same code path.

**Adjacent evidence-path defects** — not alert-generation bugs, but the reason
the *investigation* went wrong before the alert layer could be judged fairly:

- **Single-slot trace retention.** `.beads/traces/<bead>/` keeps only the last
  attempt, so the successful Aug-16 retry overwrote every crash-era trace. This
  is what made exit −1 look fabricated next to exit 0, silenced the classifier
  into UNKNOWN, and produced the wrong 2026-09-02 RCA. Compensating control in
  place: retrieval bundles with per-attempt indexes (domchk-ea755548).
  Multi-slot retention keyed by (bead, attempt) is a candidate improvement, no
  owner yet.
- **journald/log retention gap.** This host's single boot begins 2026-08-15
  19:56:33 EDT and `.beads/logs/` begins 2026-09-02 — no Aug-13 kernel OOM line
  survives for *any* bead. "No crash logs found" is therefore an evidence-
  retention limit, **not exculpatory**; the kill record that does survive is
  needle's own worker log (43 attempts, bundled verbatim).

## 3. Root cause, stated per level

**Alert level (the classification this report delivers).** The crash-alert
generation system had **no lifecycle**: alerts were created at kills and never
re-validated against the target's state afterwards. Concretely — no
closed-bead check (FIX 1/5), no completion awareness (FIX 4/6), no dedup gate
(FIX 2/3), no cooldown (FIX 4), and, once classification was added, a wiring
bug that prevented the FALSE_POSITIVE branch from ever executing (FIX at
8cc1172). Any one of these would have retired bf-66sw7c on first processing;
with none of them, 38 alerts for one target's storm survived to be processed
as if the target were still crashing. **The kill alerts were true; the crash
*case* they imply was false** — that is the false positive.

**Kill level (context, not re-derived here).** The 38 attempts were killed by
the repository-bloat regime — ~18 GB of loose objects from 17+ identical
237 MB `.beads/*.jsonl` snapshots still tracked, plus a 422-commit unpushed
backlog, driving git/bead-state operations past the 12 GiB `MemoryMax` of
needle's per-dispatch scope (memcg-OOM SIGKILL). Full derivation, ruled-out
alternates, and the live re-verification that the regime is repaired and
holding (`.git` 104 MB, health exit 0, pack-memory bound ≈3072 MiB within the
ceiling, 0 unpushed, 0 tracked `.beads/` files, 8/8 timers) are in
[`bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md`](bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md)
§2 and §5. **Nothing new is owed on the mechanism**; the mitigation
implementation record (`docs/mitigation-implementation-bf-2ildm-2026-08-13.md`)
re-proves the battery first-hand.

**Ruled out for this bead:** CODE_DEFECT (§ Verdict), SERVICE_FAILURE (mid-task
signal losses, not HTTP 5xx/timeout exits), WORKFLOW_FAILURE (exit would be 1
`error_max_turns`, not −1), bead-store corruption (no schema errors; deaths
precede any store mutation).

## 4. Why the alert system *incorrectly flagged this as a crash* — the task's question, answered precisely

It didn't — not at fire time — and keeping that straight is the whole
classification:

1. **At fire time the flag was correct.** A real worker really died, mid-task,
   38 times. An alerting system that stayed silent through that would be
   broken in the other direction.
2. **What was incorrect was the alert's persistence.** Each alert asserted a
   live crash needing investigation. After 2026-08-16T22:44:38Z that assertion
   was false for all 38, and the layer had no closed-bead filter, dedup gate,
   cooldown, or working classification path to notice. The flag aged from true
   to false with nothing to retire it.
3. **What was also incorrect was the first investigation's reading of the
   flag.** Told "exit −1," the 2026-09-02 investigation went looking for
   corroboration, found the single-slot trace's Aug-16 exit 0, and concluded
   the −1s were fabricated placeholder data and the alert was impossible.
   Both steps of that reasoning were evidence-selection errors (§2), and they
   produced a confident wrong root cause — the failure mode this report exists
   to close out.
4. **Net:** FALSE_POSITIVE is the right *final* classification of the alert
   case — stale alert against a resolved target, zero lost work, dedup →
   DUPLICATE — provided it is never read as "no crash happened."

## 5. Evidence from the two child beads

**From domchk-fe9f7c5b (context summary of domchk-b049ea9e) — supports the
alert-level FALSE_POSITIVE and the work-not-lost premise:**

- Task context is crash-irrelevant: `git log <ancestor>..<github-branch>`
  extraction, third step of a branch-divergence chain; the same shape
  succeeded on 2026-08-16 in 85 s — so the task cannot be the kill's cause,
  which is what rules out CODE_DEFECT.
- bf-2ildm is **Closed** (verified by that bead live, and again by this one) —
  the durable core of the false-positive finding.
- Its five-item claim/standing table (§2 of the context doc) is the source of
  this report's §2: which of the 09-02 "bugs" stand as alert-layer defects and
  which were misreadings.
- Live re-verifications that this dispatch reproduced unchanged:
  `alert-deduplication.sh check bf-66sw7c` → DUPLICATE;
  `crash-classifier.sh bf-2ildm` → UNKNOWN with the provenance warning;
  `attempt-index.tsv` row 19 and the ledger rows match.

**From domchk-167289ae (crash-log summary of domchk-a5a51981) — supports the
kill level and the supersession of the 09-02 reasoning:**

- Trace analysis, reproduced byte-for-byte: `metadata.json` exit 0 / `success`
  / 85,327 ms / captured 2026-08-16T22:28:44Z; `stderr.txt` three benign
  warnings; `trace.jsonl` 47 events with exactly one failed result (a benign
  `bead dep show` usage error) — i.e. **the trace is clean because it is the
  retry's trace**, not because no kill happened.
- The single-slot retention blind spot, named as the mechanism that produced
  the wrong 09-02 conclusion — this report's §2 adjacent defect.
- Attempt census re-derived from the bundle: 43 attempts = 38 × exit −1 + 4 ×
  exit-124 (600 s caps, no alerts) + 1 × exit-1 at 19 ms (quarantined at
  `failure_count: 5`); 38 alert beads mapping 1:1, stamp gaps 7.7–29.4 s.
- The signal-−1 correction (sentinel, not signal; not placeholder) and the
  timestamp-ordering correction (kill → heartbeat +13.1 s → alert bead +14 ms
  → close 3 days later) — the two reversals this report's §1 and §2 rely on.
- Its two-level verdict table (kill INFRASTRUCTURE / alert FALSE_POSITIVE) is
  adopted here unchanged.

**Both children agree on the load-bearing point, from different directions:**
the work was completed and the target closed (fe9f7c5b, from bead state) and
the surviving trace is a clean successful run (167289ae, from the trace
archive). Together they establish the false positive *and* locate its actual
cause in alert lifecycle rather than in fabricated crash data.

## 6. Dispatch-premise correction: the phantom child-bead ID

This dispatch's task text (and domchk-d932c30c's) names a child bead
**`domchk-fe9f7f7b`**. **No such bead exists.** Verified 2026-09-07 against the
live store: 0 occurrences as an issue ID, 0 as a dependency edge, `bead show`
→ "Issue not found"; the only two occurrences anywhere in the workspace are
inside task-description *text*, both from the same 2026-09-02T09:28 split.

The intended bead is **`domchk-fe9f7c5b`** — one transposition away
(`…f7f7b` vs `…f7c5b`), it is the bead that actually blocks
`domchk-167289ae`, and its title/task ("Extract and summarize crash context
findings") is exactly the role the task text assigns. This report treats
domchk-fe9f7c5b as the first input. No store mutation is made: the phantom ID
lives only in prose, the dependency graph is already correct, and renaming
history is not this bead's to do.

This is a data-quality finding about the split that created this chain, not a
defect in the alert system — recorded here so the next link in the chain
(domchk-d932c30c, which cites the same phantom ID) does not re-derive it or
treat the missing bead as lost work.

## 7. First-hand verification battery (run 2026-09-07 for this report)

| Check | Command | Result |
|---|---|---|
| Target state | `bead show bf-2ildm` | **Closed**, rev 7 (updated 2026-09-07T17:47:17Z, classification append) |
| Successful retry | read `.beads/traces/bf-2ildm/metadata.json` | `exit_code 0`, `outcome success`, 85,327 ms, captured 2026-08-16T22:28:44.172Z |
| Dedup gate | `./scripts/alert-deduplication.sh check bf-66sw7c` | **DUPLICATE: crash target bf-2ildm is already resolved** |
| Classifier | `./scripts/crash-classifier.sh bf-2ildm` | **UNKNOWN** + "trace slot does not describe the incident run" provenance warning (expected; §2) |
| Attempt census | `awk` over `docs/crashes/bf-2ildm/attempt-index.tsv` (exit_code col) | **43 attempts = 38 × −1, 4 × 124, 1 × 1** |
| Alert mapping | distinct `alert_bead` among the −1 rows | **38 distinct beads, 0 empty** — 1:1 with the kills |
| Stamp gaps | `alert_stamp_s_after_kill` column, min/max | **7.7 s / 29.4 s** |
| Worked example | row 19 | claim 14:38:24.455725579Z → kill 14:40:29.551762239Z, −1, 124,759 ms → bf-66sw7c, **13.1 s** |
| Alert ledger | `wc -l crash-alert-ledger.tsv` | 39 lines = 38 alerts + header |
| Alert fixes present | `grep -n "CRITICAL FIX" scripts/crash-alert-manager.sh` | FIX 1–6 present (lines 210, 218, 294, 317, 354, 602) |
| Classification wiring | `sed -n '401,402p'` + `git merge-base --is-ancestor 8cc1172 origin/main` | anchored-token grep with `head -1` fallback; **8cc1172 is pushed** |
| Alert-fix suite | `./scripts/test-crash-alert-fixes.sh` (repo cwd) | **13/13 passed** |
| Closed-bead filter | `./scripts/test-closed-bead-filter.sh` (repo cwd — it is cwd-sensitive) | **7/7 passed**, no alert for closed bf-2vtzg |
| Phantom ID | `bead list --json` (full store) for `"id":"domchk-fe9f7f7b"` / `"blocker":"domchk-fe9f7f7b"`; `bead show` | **0 / 0 / Issue not found**; `domchk-fe9f7c5b` resolves (§6) |
| Chain state | `bead show` | parent **bf-o6vbwl Open** (rev 28); successor **domchk-d932c30c Open** (blocked by this bead) |

## 8. What this classification does and does not close

**Closed by this report:** the classification obligation of this chain. The
alert case for all 38 alert beads is FALSE_POSITIVE (resolved target, no lost
work, dedup-agreed); the kill case is INFRASTRUCTURE (repaired regime, verified
holding); the alert-layer defect list is stated with each item's fix status and
live verification.

**Still open, each owned elsewhere and not this bead's to act on:**

- **bf-o6vbwl** (parent alert, rev 28, Open) — the chain's reporting child
  **domchk-d932c30c** is the step that compiles and delivers this to the
  parent.
- **domchk-f6fff20f** — tracks closure/verification of the CLASSIFICATION
  wiring fix; the mechanism itself is fixed and pushed (8cc1172).
- **Single-slot trace retention** and **alert-stamp provenance** — residual
  gaps 1 and 3 of the RCA §6; compensating controls documented, no owner yet
  for multi-slot retention.

## 9. Sources

- Child inputs: domchk-fe9f7c5b →
  [`docs/crash-context-bf-2ildm-2026-08-13.md`](../crash-context-bf-2ildm-2026-08-13.md);
  domchk-167289ae →
  [`docs/crash-log-findings-bf-2ildm-2026-09-07.md`](../crash-log-findings-bf-2ildm-2026-09-07.md)
- Standing record:
  [`docs/investigations/bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md`](bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md)
  (§2 kill mechanism, §3 alert-level root cause and ordinal correction, §5 live
  infrastructure verification, §6 residual gaps)
- Per-instant classifications:
  [`docs/crash-classification-bf-2ildm-2026-08-13-14-40.md`](../crash-classification-bf-2ildm-2026-08-13-14-40.md),
  [`docs/crash-classification-bf-2ildm-2026-08-13-15-01.md`](../crash-classification-bf-2ildm-2026-08-13-15-01.md)
- Evidence bundle: `docs/crashes/bf-2ildm/` — `attempt-index.tsv`,
  `crash-alert-ledger.tsv`, `trace-archive-current-state.json` (domchk-ea755548)
- Live (this dispatch): `.beads/traces/bf-2ildm/{metadata.json,trace.jsonl}`;
  `bead show bf-2ildm` / `bf-o6vbwl` / `domchk-d932c30c`; full-store ID search
  for the phantom input; the two alert suites; the dedup and classifier scripts
- Superseded (context only): `docs/investigations/root-cause-determination-bf-2ildm-2026-09-02.md`,
  `docs/investigations/bf-2ildm-final-investigation-report-2026-09-02.md` — the
  source of this dispatch's 2026-09-02 task text, §4 above
- Mitigation status: `docs/mitigation-implementation-bf-2ildm-2026-08-13.md`;
  `CLAUDE.md` "Crash Alert System" (suite counts, replay verification, wiring-fix
  history)
