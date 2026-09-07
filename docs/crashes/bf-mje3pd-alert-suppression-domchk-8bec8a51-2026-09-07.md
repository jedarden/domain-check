# bf-mje3pd alert-suppression verification (domchk-8bec8a51, 2026-09-07)

Split step 3 of ALERT bead **bf-3dxljn** ("ALERT: Agent crash on bead
bf-mje3pd"). The target bead bf-mje3pd is Closed, so the 2026-09-02 alert
fixes should have suppressed this alert. This note answers that question
first-hand, verified against a `git archive HEAD` extract of `79e45de`
(HEAD advanced twice more mid-dispatch — `0c0c174`, `395cf89`, both
docs-only; `git diff --stat 79e45de..395cf89 -- scripts/` is empty, so
every result here carries over verbatim).

**Verdict: suppression is confirmed at every layer that would see a fresh
bf-mje3pd alert.** The alert bead itself is STALE, not false — the crashes
were genuine and mid-task, the work delivered on attempt 14, and the bead
closed 2026-08-17. bf-3dxljn stays Open only because the 2026-09-02 stack
suppresses *future alert creation* and nothing retires an *already-created*
alert bead — the closure gap already recorded in
`docs/crashes/bf-mje3pd-prevention-research-domchk-f7865662-2026-09-07.md`
("detection landed, closure has not") and
`docs/crash-investigations/bf-4yjq-crash-chain-resolution-domchk-a6059a67-2026-09-07.md`
("no alert lifecycle retires alerts when the target resolves").

## 1. Live bead state at verification time (2026-09-07 ~21:00Z)

| Bead | Title | Status |
|------|-------|--------|
| bf-mje3pd | Implement fix and verify agent crash prevention | **Closed** (rev 2, updated 2026-08-17T00:15:35Z) |
| bf-3dxljn | ALERT: Agent crash on bead bf-mje3pd | Open (rev 16) — the stale alert under split |
| bf-1pidqn | ALERT: Agent crash on bead bf-mje3pd | Open |
| bf-56kmlk | ALERT: Agent crash on bead bf-mje3pd | Open |
| bf-x88dnf | ALERT: Agent crash on bead bf-mje3pd | Open |

Enumerated with the same target-extraction logic `alert-deduplication.sh`
uses (title regex `(?:on|of)\s+(?:bead\s+)?((?:bf|domchk)-[a-z0-9]+)`): **4
open alert beads cover target bf-mje3pd** as of 2026-09-07 ~21:20Z. The
fifth from domchk-f7865662's and domchk-7dc8f0d6's same-day counts
(bf-1cezsk) re-reads **Closed** in the live store now — it closed after
395cf89's "five re-read Open" was written — so the fleet is converging on
its own.

## 2. Criterion 1 — `scripts/test-crash-alert-fixes.sh` pass count

| Tree | Result | Exit |
|------|--------|------|
| **HEAD 79e45de** (`git archive HEAD` extract) | **12/12 passed, 0 failed** | 0 |
| Shared worktree | 13/13 passed, 0 failed | 0 |

The task's "expected 12/12" matches **HEAD exactly**. The worktree's 13th
test ("Test 13: Verifying closed bead bf-2vtzg does not trigger a new
alert…", the functional closed-bead check) is a **co-tenant's uncommitted
addition** — `scripts/test-crash-alert-fixes.sh` is ` M` in `git status`,
and `git diff HEAD` shows the whole delta is that one appended test. Count
drift = suite growth, not staleness; both trees pass everything they
contain. Independently corroborated mid-dispatch by domchk-c1b09ba2
(`0c0c174`), which records the same pair — 12/12 at a HEAD extract,
13/13 in the worktree — from a different dispatch.

## 3. Criterion 2 — `scripts/alert-deduplication.sh` checks

### 3.1 The live check on the actual stale alert

```
$ bash scripts/alert-deduplication.sh check bf-3dxljn
DUPLICATE: crash target bf-mje3pd is already resolved
(exit 0)
```

Identical verdict from the HEAD-extract copy (cwd = repo so `bead list`
resolves the real workspace; the copy's log write lands in the extract,
not `.beads/logs/`). This is the same gate the manager invokes before
generating any alert: a fresh alert for bf-mje3pd would be suppressed as a
duplicate of an already-resolved crash.

### 3.2 Which suppression leg fires (leg-by-leg, per the script's order)

| Leg | Input checked | Result for bf-mje3pd |
|-----|---------------|----------------------|
| 1. VERIFIED work-completion marker | `.beads/state/work-completion/bf-mje3pd.json` | **Absent** — this Aug-13 bead predates `verify-work-completion.sh`; leg not applicable |
| 2. Resolution tracker (bead closure) | `crash-resolution-tracker.sh bf-mje3pd check` | **FIRES** — `RESOLVED`, exit 0, `"resolution_type": "bead_closure"`, `"verified": true`, "Live evaluation at check time (auto-backfilled cache)" |
| 3. Open alerts covering target | 4 open alert beads title-referencing bf-mje3pd | Available (§1) but **unreached** — leg 2 returns first |
| 4. 7-day crash-history window | `grep bf-mje3pd .beads/logs/crash-history.jsonl` | **No entries** — nothing was ever `record`ed for this target; the ledger only fills when a generated alert passes the manager's record leg, so this alert predates the ledger. Suppression never depended on it here |

So the "closed bead" half of criterion 2 is carried by leg 2 (live bead
closure, the tracker being the single resolution authority), and the
"duplicate detection" half is demonstrated by leg 3's input: any *further*
alert for bf-mje3pd would additionally be suppressed as covered by the
four open alerts.

## 4. End-to-end manager suppression, bf-mje3pd itself

`crash-alert-manager.sh bf-mje3pd` was run against a sandbox copy
(PROJECT_ROOT derives from the copy's own path, so every write lands in
the sandbox; `bead show` still resolves the real workspace because the run
uses cwd = repo) with a fabricated `.beads/traces/bf-mje3pd/` trace of
`{"exit_code":-1}` — the bead's real crash signature, so FIX 4 cannot
pre-empt the closure gates. Both runs exit 0 and generate nothing:

| Scenario | Gate that fired | Output |
|----------|-----------------|--------|
| A — tracker deliberately absent (isolates CRITICAL FIX 1, as `test-closed-bead-filter.sh` does for bf-2vtzg) | FIX 1 closed-bead gate | `Bead bf-mje3pd is already CLOSED - no alert needed` → `Reason: Bead already closed (work may have completed before crash)` — the exit-code −1 branch, correct for a bead that completed work on a later attempt |
| B — tracker present (the manager's real gate order: resolution check at `crash-alert-manager.sh:281` precedes FIX 1 at :294) | Resolution tracker | `Crash bf-mje3pd is already resolved - no alert generated` → `Reason: Crash already resolved` + the RESOLVED JSON of §3.2 |

Zero alert-generation entries in either sandbox log; no
`processed-alerts.txt` alert record; no alert-state file.

Supporting suite: `scripts/test-closed-bead-filter.sh` (functional FIX 1
suite on closed bead bf-2vtzg) — **7/7 passed, exit 0**, byte-identical at
HEAD and in the worktree. Run from the repo cwd per the known
cwd-sensitivity of its `bead show` premise.

## 5. Conclusion for the split

A fresh alert targeting bf-mje3pd would be stopped four times over, in
order: manager resolution gate → FIX 1 closed-bead gate → FIX 2
existing-alert check → dedup leg 2 (closure), with leg 3 (four covering
alerts) as backstop. **The 2026-09-02 fixes work as designed; this alert
was never the kind they suppress** — bf-3dxljn was created 2026-08-13,
weeks before the fixes, and no component of the stack retires an
already-created alert bead once its target closes. Disposition matches the
chain's settled reasoning (domchk-bf8c4fd3 / domchk-a47705c9 /
domchk-1b407bff / domchk-7dc8f0d6): crashes genuine, mid-task at every
named instant, work
delivered on attempt 14 with `verification.passed`, nothing lost — the
alert is **stale, not false**, and the correct action is to close it as
stale with no investigation owed. The remaining four open duplicates are
the same shape and need no investigation either; they wait on the alert
lifecycle-closure layer that domchk-f7865662 lists as an open gap.

## 6. Environment caveats (shared worktree, co-tenant in-flight work)

All conclusions above were re-verified from a `git archive HEAD` extract
(`/tmp`), because the shared worktree carried substantial co-tenant work
that is **not** this dispatch's and was left untouched:

- Staged (`git rm --cached`) deletions of the five sibling bf-mje3pd/bf-2ildm
  notes and 5 alert test scripts (`alert-cooldown.sh`,
  `test-alert-cooldown*.sh`, `test-crash-alert-classification-wiring.sh`,
  `test-crash-classifier-signals.sh`, `test-trace-slot-provenance.sh`) —
  files still present on disk, untracked; plus staged modifications to
  `CLAUDE.md`, `docs/alerting-system-guide.md`, `docs/crash-response-guide.md`,
  `scripts/README.md`.
- `scripts/crash-alert-manager.sh` carries a co-tenant's **+137 uncommitted
  lines** vs HEAD; `scripts/test-crash-alert-fixes.sh` the uncommitted 13th
  test. `scripts/alert-deduplication.sh` and `scripts/crash-classifier.sh`
  are byte-identical to HEAD in the worktree (their `MM` status is staged
  churn the worktree has already reverted).
- Suppression therefore does **not** depend on the uncommitted +137 lines:
  every result in §2–§4 was reproduced with HEAD's scripts only.
- HEAD advanced `e7f5b97 → 79e45de → 0c0c174 → 395cf89` mid-dispatch
  (prevention-research, test-coverage-verification, and pattern-analysis
  notes — all docs-only; `git diff --stat 79e45de..395cf89 -- scripts/`
  is empty). No claim here is affected; the verification runs in §2–§4
  used the `79e45de` extract, whose alert scripts are byte-identical to
  those at `395cf89`, and citations are to paths verified present at HEAD,
  not to the dirty worktree.

## 7. Closing-attempt re-verification (2026-09-07 ~21:35Z, HEAD `ab9042f`)

Re-executed first-hand by the bead's closing attempt, against a
`git archive HEAD` extract of `ab9042f` (the note above + two docs-only
commits later than the `79e45de` extract; `git diff --stat
79e45de..ab9042f -- scripts/` is empty, so the same scripts ran), from
the repo cwd so `bead show` resolves the live workspace. Extract copies
only — `PROJECT_ROOT` derives from `SCRIPT_DIR/..`, so nothing was
written to the live `.beads/logs/`:

- `test-crash-alert-fixes.sh` → **12/12 passed, 0 failed, exit 0** — the
  task's expected count, matched at HEAD (`13/13` remains the shared
  worktree's count, the co-tenant's uncommitted 13th test).
- `test-closed-bead-filter.sh` → **7/7 passed, exit 0** — its premise
  re-resolved bf-2vtzg as CLOSED in the live store. This is an
  end-to-end `crash-alert-manager.sh` run for a closed bead, so it
  re-proves §4 scenario A live; §4 scenario B's gate is re-proven by the
  dedup leg below.
- `alert-deduplication.sh check bf-3dxljn` → `DUPLICATE: crash target
  bf-mje3pd is already resolved`, exit 0, via leg 2:
  `crash-resolution-tracker.sh bf-mje3pd check` → `RESOLVED`,
  `"resolution_type": "bead_closure"`, `"verified": true`.
- Live store re-read matches §1 exactly: bf-mje3pd **Closed**;
  bf-3dxljn / bf-1pidqn / bf-56kmlk / bf-x88dnf **Open** (4 open alerts
  covering the target); bf-1cezsk **Closed**.

Every result matches §§1–4; nothing in this note needed correction.
