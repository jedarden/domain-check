# bf-mje3pd — Evidence Compilation

**Investigation bead:** domchk-916a1e66 (2026-09-07)
**Alert bead being split:** bf-3dxljn — "ALERT: Agent crash on bead bf-mje3pd"
**Scope:** inventory of what already exists. No new analysis, no new log
extraction. Every claim below was re-checked live during this dispatch; where
an earlier doc's number has since been corrected, the correction is noted
rather than repeated.

**HEAD moved during compilation.** This note was drafted against `9b83c81`;
before it was committed two sibling beads landed `f21e381` (global crash-alert
cooldown, domchk-7b404946) and `63ca904` (a first-hand **classification of
bf-mje3pd itself**, domchk-bf8c4fd3). The classification doc is folded in
below and is now the authoritative word on this crash — several figures in
older docs, including ones this note initially repeated, are corrected by it.

---

## 1. Target bead status (checked live via `bead show bf-mje3pd`)

| Field | Value |
|---|---|
| ID | bf-mje3pd |
| Title | Implement fix and verify agent crash prevention |
| **Status** | **Closed** (rev 2) |
| Updated | **2026-08-17T00:15:35Z** — matches the expected "Closed 2026-08-17" |
| Created | 2026-08-13T18:25:38Z |
| Assignee | `claude-code-glm-4.7-lab-domain-check` |
| Type / Priority | task / P2 |

The alert premise is **real, not fabricated**: bf-mje3pd did run a genuine
exit -1 crash loop on 2026-08-13. But the target bead closed the same evening
(rev 2, 2026-08-17), so **any alert pointing at it after that date is stale**
— this is the classic closed-target shape that `scripts/crash-alert-manager.sh`'s
FIX 1/5 closed-bead filter exists for.

### The alert-bead family for this one target

**Seven** alert beads all name bf-mje3pd. Their statuses were read live
2026-09-07 (the two marked † were surfaced by the sibling classification doc,
not by this bead's own search):

| Alert bead | Created | Status | Notes |
|---|---|---|---|
| bf-1y1d0g | 2026-08-13T19:03:21Z | **Closed** (2026-09-02) | resolution report committed d77d9ff |
| bf-1pidqn † | 2026-08-13T19:15:29Z | **Open** | no verification report found |
| bf-56kmlk † | 2026-08-13T19:18:56Z | **Open** | the alert whose timestamp the classification doc traces to attempt 4's kill |
| bf-3za7vh | 2026-08-26 | **Closed** (2026-08-26) | the report this task cites by path |
| bf-x88dnf | 2026-08-26 | **Open** | verification report exists (c0ee8f3) — concluded false positive |
| bf-1cezsk | 2026-08-26 | **Open** | verification report exists (c9ac788) — concluded false positive |
| **bf-3dxljn** | 2026-08-13T19:46:57Z | **Open** (rev 16) | **this bead**, exit code -1 |

Five Open, two Closed. The classification doc (`63ca904`) already names five
of these — including bf-3dxljn — as stale-and-warranting-no-action. The two
bf-x88dnf / bf-1cezsk verification reports additionally called *those* alerts
false positives. Nothing in this family needs new investigation.

### This bead's own timestamp is a heartbeat, not a kill

bf-3dxljn's report reads `Timestamp: 2026-08-13T19:46:57.229442127Z`. The
classification doc establishes that these alert timestamps are release
heartbeats stamped seconds *after* the real kill, and maps the family onto
the attempt table. Mapping this bead the same way: attempt 11 was killed at
**19:46:33Z**, and `outcome.handled action=alerted` fired at **19:47:00Z** —
bf-3dxljn's 19:46:57 sits 24 s after the kill and 3 s before the alerted
record. That is attempt 11's heartbeat, not a 14th distinct crash. (The same
shape explains why bf-3dxljn and bf-56kmlk exist at all: pre-0.4.2 needle
emitted one ALERT bead per kill, so a 7-kill loop minted ~7 alert beads.)

---

## 2. Existing docs covering this crash

### Path correction (important)

The task names
`docs/verification-report-bf-3za7vh-crash-analysis-bf-mje3pd-2026-08-26.md`.
**That path does not exist.** The file was moved into the crash-doc archive
freeze (387 docs under `docs/archive/crash-investigations/`, commit `a883044`)
and now lives at:

```
docs/archive/crash-investigations/verification-report-bf-3za7vh-crash-analysis-bf-mje3pd-2026-08-26.md
```

Content is intact; only the path moved. Cite the archived path.

### The docs

| Document | Commit / date | What it establishes |
|---|---|---|
| `docs/crashes/bf-mje3pd-crash-classification-domchk-bf8c4fd3-2026-09-07.md` | **63ca904**, 2026-09-07 (domchk-bf8c4fd3) — **landed during this compilation; the authoritative classification** | **INFRASTRUCTURE**, repository-bloat regime sub-type, via the crash-response-guide exit-code decision tree. Full 14-attempt table from the raw worker log (281 events, JSON-parsed). Alert disposition stated precisely: the crashes were **real and mid-task — not false positives at the time**; the disposition *today* is **stale, not false**, because the bead closed. Names the five still-open alert beads as warranting no action. Three indirect lines pin the regime (workspace-locality, the 344-kill Aug-13 storm across 13 beads in this one worker's log, and the 3–6 min kill cadence); the mechanism is **regime-matched, not kernel-proven**. |
| `docs/crashes/bf-mje3pd-crash-artifacts-analysis-domchk-a4cc1326-2026-09-07.md` | 9b83c81, 2026-09-07 (sibling bead domchk-a4cc1326) | First-hand re-extraction from `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl`: 281 events, 14 dispatches / 13 completions across 18:53:50Z–21:18:36Z (2h24m46s). Corrects the older docs: the kill burst was **43m22s, not 53 min**; attempt 10 ran a 2005-byte mitosis template, not the shared pluck prompt; release-to-claim gaps 2–18s. |
| `docs/verification/bf-mje3pd-crash-analysis.md` | **81614ac**, 2026-09-02 (domchk-9bc6579f) — the commit the task names | The main verification report. RESULT: persistent infrastructure crash with **eventual success** — 11+ crash attempts over ~2h15m (19:03–21:18 UTC, 2026-08-13). Root cause: repository bloat (18GB `.git`, 17GB loose objects) triggering the OOM killer during git operations. Adds 3 remediation recommendations beyond the preventive measures already in place. *Superseded on counts by `63ca904` (see corrections below).* |
| `docs/archive/crash-investigations/verification-report-bf-3za7vh-crash-analysis-bf-mje3pd-2026-08-26.md` | 624b2d2, 2026-08-26 (alert bead bf-3za7vh) — the doc the task names by its former path | Earliest full analysis. Same conclusion (eventual success after 11+ attempts) and adds that the bead was marked "orphaned" by the system despite the successful outcome. **Superseded on detail by `63ca904`.** |
| `docs/notes/incident-resolution-bf-1y1d0g-bf-mje3pd-crash-2026-09-02.md` | d77d9ff, 2026-09-02 | Resolution record for sibling alert bf-1y1d0g. Status RESOLVED. **Corrected by `63ca904`:** says 9× exit -1 (first-hand 7), success on the 13th attempt (first-hand 14th dispatch), "2× exit 1" (first-hand 3), and its "17 GB objects loaded into memory → OOM → SIGKILL" is regime inference with no surviving kernel record. |
| `docs/archive/crash-investigations/crash-context-bf-mje3pd-summary.md` | 604c22e, 2026-08-26 | Crash context summary. |
| `docs/bead-verification/bf-x88dnf-verification-2026-08-26.md`, `docs/bead-verification/bf-1cezsk-verification-2026-08-26.md`, `docs/archive/bead-verification-reports/BEAD_BF-1Y1D0G_VERIFICATION_REPORT.md` | c0ee8f3 / c9ac788 / 0af6a99, 2026-08-26 | Verification reports for three of the other alert beads. The bf-x88dnf and bf-1cezsk ones conclude **false positive** — direct precedent for closing bf-3dxljn on evidence rather than re-investigating. |

### Verified crash census (from `63ca904`, the figure to cite going forward)

14 dispatches, 13 completions, 12 classified outcomes, 18:53:50Z first claim →
21:18:36Z `bead.orphaned` (2h24m46s): **7 × exit -1 (crash), 3 × exit 1,
1 × 124 timeout, 1 × 0 success**, one unclassified mitosis-eval completion,
and one dispatch whose death is bracketed only by a `peer.crashed` record at
20:36:55Z with no completion record at all. The deliverable landed on the 14th
dispatch at 21:18:23Z with `verification.passed`.

**Correction worth flagging:** the domchk-a4cc1326 doc reports the same census
as "2× exit 0" — it counts the unclassified mitosis-eval attempt as a zero.
The first-hand count is one classified exit 0 plus one attempt with no
classified outcome.

Supporting context, not bf-mje3pd-specific: the repository-bloat regime that
caused these kills is documented and repaired — `docs/crash-analysis-bf-1s6c3-2026-09-06.md`
(18GB → 93MB, re-verified 2026-09-06) and the repo-level CLAUDE.md health
section.

### Related fix commits (the bead's own deliverable survived)

`ea23bd1` ("fix: implement OOM crash prevention and repository health
monitoring") and `164b62d` ("fix: add preventive scripts for repository bloat
and OOM crash prevention") are bf-mje3pd's actual work, committed in-loop
during the crash window — `ea23bd1` at 19:02:32Z, 39 s before attempt 1's
kill, and `164b62d` at 19:21:41Z, 14 s before attempt 5's kill. **Reachability
nuance from `63ca904`:** both commits are reachable only from the local-only
`pre-squash-history-20260816` branch; their *content* is live at HEAD via
later landings. Cite "content live at HEAD", not the SHAs as ancestors.
The target's task completed; only the alert layer left debris.

---

## 3. Alert tooling available under `scripts/`

The three scripts the task names, all present and executable:

| Script | Size at HEAD | Purpose (from its own header) |
|---|---|---|
| `scripts/crash-classifier.sh` | 25,399 B | Analyzes crash artifacts and classifies crash type to prevent false positives; distinguishes technical crashes from administrative workflow failures. Usage: `crash-classifier.sh <bead-id>`. |
| `scripts/alert-deduplication.sh` | 23,461 B | Decides whether a crash alert duplicates an already-resolved or already-covered crash, so resolved crashes stop generating new investigation beads. Created 2026-09-02; **contract rewritten 2026-09-07** (domchk-b3de301f) per `docs/alert-deduplication-gap-analysis-2026-09-07.md` (D-2/D-4/D-6). |
| `scripts/crash-alert-manager.sh` | 20,193 B | Integrates classification with deduplication to filter false positives; carries the 6 critical fixes (closed-bead filtering, duplicate detection, processed-alert tracking, completion awareness, exit-code validation, 5-min cooldown). Created 2026-09-02. |

Sizes are the HEAD blob for each script. `crash-alert-manager.sh` also carries
a co-tenant's **+137-line uncommitted working-tree extension** (27,507 B on
disk) as of this compilation — that delta is not HEAD and is not described
here.

Wider family present in the same directory, relevant if this split grows
legs: `alert-cooldown.sh` (14,940 B — **new global cooldown with
suppressed-crash accounting, landed as `f21e381` during this compilation**;
its header explains that without a cooldown one ALERT bead per kill is what
turned the 2026-08-16 cascade into 177 crashes across 59 beads),
`alert-triage-sweep.sh` (+ `setup-alert-triage-timer.sh`,
`domain-check-alert-triage.{service,timer}`), `classify-signal-crash.sh`,
`crash-circuit-breaker.sh`, `crash-pattern-detection.sh`,
`crash-resolution-tracker.sh`, and their test suites
(`test-crash-alert-fixes.sh` 13/13, `test-closed-bead-filter.sh` 7/7 as
re-verified 2026-09-07). `alert-cooldown.sh` is the direct mechanical answer
to why this alert family has seven members: it exists to stop exactly that
fan-out.

### Classifier caveat for this specific bead

Reproduced first-hand during this compilation:

```
$ ./scripts/crash-classifier.sh bf-mje3pd
ERROR: Bead trace not found: .beads/traces/bf-mje3pd/trace.jsonl
```

Per-bead traces are single-slot and this one is gone; the `.beads/logs/` era
starts 2026-09-02 and the journal floor is 2026-08-15, so **no automated
classifier can see a 2026-08-13 crash**. A failed classifier run is therefore
not evidence that no crash occurred — classification for this bead comes from
the raw fleet worker log plus regime match, which `63ca904` already did.

---

## 4. Conclusion for the split

Everything needed to resolve bf-3dxljn already exists:

1. **Target bead closed** 2026-08-17, rev 2 — verified live.
2. **The crash is real and was mid-task** — 7 × exit -1 across 2h24m on
   2026-08-13 — so it was **not a false positive at the time**. It is
   **INFRASTRUCTURE** (repository-bloat regime, mechanism regime-matched; no
   surviving kernel record). **No work was lost**: the deliverable landed on
   the 14th dispatch with `verification.passed`.
3. **The alert is stale, not false.** `63ca904` states this disposition
   explicitly and already names bf-3dxljn among the open alerts warranting no
   action.
4. **Six sibling alert beads** cover the same target — two Closed with
   committed resolution records, two Open with committed false-positive
   verdicts, two more Open and unexamined (bf-1pidqn, bf-56kmlk).
5. **The alert tooling** that should have retired these alerts automatically
   is present and tested, and `f21e381` just added the global cooldown that
   prevents the one-bead-per-kill fan-out that minted this family.

**bf-3dxljn's own timestamp is a release heartbeat 24 s after attempt 11's
kill, not a distinct crash** — so this bead documents a kill already covered
by the census, not an additional event.

The remaining work on bf-3dxljn is the close decision, not investigation.
Prior art for the close reason: the bf-6d3d6 chain (closed end-to-end
2026-09-07) used *"crashes genuine / work never lost / alert stale"* — which
`63ca904` confirms verbatim for this bead.

---

*Compiled 2026-09-07 by domchk-916a1e66. Bead statuses read live from the
bead-rs store; doc paths verified against HEAD (`9b83c81` at draft, `63ca904`
at commit); `crash-classifier.sh bf-mje3pd` re-run first-hand; script sizes
are HEAD blob sizes from the same hour.*
