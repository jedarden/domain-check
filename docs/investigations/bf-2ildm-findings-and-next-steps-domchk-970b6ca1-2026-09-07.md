# bf-2ildm investigation — consolidated findings and next steps

**Author bead:** domchk-970b6ca1 — "Document investigation findings and determine
next steps" (Child Bead 4 of the 2026-09-02 investigation split)
**Date:** 2026-09-07
**Crash target:** bf-2ildm — "Extract GitHub-specific commits" (Closed 2026-08-16T22:44:38Z)
**Scope:** synthesis and determination only. This bead does not implement fixes;
the one remediation action it determines is needed is handed to a new bead
(domchk-4a05973f, §7).

This document compiles the findings of every prior bead in the chain, states the
crash classification with its evidence, lists the alert-system defects the chain
observed, and determines what (if anything) remains to be done.

## 1. Chain inventory — what each bead found and where it is recorded

| Bead | Role | Status | Deliverable |
|---|---|---|---|
| domchk-ea755548 | Child 1 — retrieve crash logs and metadata | Closed | Evidence bundle `docs/crashes/bf-2ildm/` (`attempt-index.tsv` 43 attempts, `crash-alert-ledger.tsv` 38 alerts, `README.md`, trace-slot current state, attempt-1 transcript) |
| domchk-0d6cd2b9 | Child 2 — verify target bead closure | Closed | bf-2ildm closed 2026-08-16T22:44:38.873946777Z, rev 7, zero reopen events |
| domchk-58866353 | Chain parent — investigate crash circumstances | Open (notes updated by this bead) | 2026-09-02-era finding "stale FALSE_POSITIVE alert"; three of its supporting claims are dated-corrected in §5 below |
| domchk-41508c5c | Sibling — analyze root cause (2026-08-26 template) | Closed | Systemic pattern analysis: INFRASTRUCTURE primary, alert-system bugs secondary, repo bloat tertiary |
| domchk-48231aec | Sibling — implement remediation / close alert | Closed | The six alert-system fixes of 2026-09-02 (closed-bead filter, dedup, exit-code validation, completion awareness, cooldown, classification) |
| domchk-1b80364e | Child 3 — classify crash and analyze alert validity | Closed | Two-level verdict: kill INFRASTRUCTURE (repository-bloat regime), alert FALSE_POSITIVE/stale; automated tools return UNKNOWN/DUPLICATE for reasons documented there |
| domchk-e05fa5a8 | Companion — per-instant classification (15:01:52Z stamp) | Closed | `docs/crash-classification-bf-2ildm-2026-08-13-15-01.md` (e3a8820) |
| domchk-a863a1f9 | Companion — root cause determination | Closed | `docs/investigations/bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md` (ef960d2); supersedes the 2026-09-02 RCA |
| **domchk-970b6ca1** | **Child 4 — this document** | this bead | This synthesis; parent-bead note update; remediation bead domchk-4a05973f |

## 2. Consolidated findings

**The target's work was never lost.** bf-2ildm was created 2026-08-13T11:12:57Z,
suffered 43 dispatch attempts on 2026-08-13, then completed successfully on a
2026-08-16T22:28:44Z retry (exit 0; duration 85,542 ms per `.beads/events.jsonl`,
85,327 ms per trace metadata — a 215 ms instrument difference between the two
sources) and closed at 2026-08-16T22:44:38.873946777Z (rev 7). It has never been
reopened. Re-verified live 2026-09-07 (`bead show bf-2ildm` → Closed, rev 7).

**The crashes were real.** 2026-08-13, 43 attempts in the window
13:35:34Z → 16:35:40Z:

| Exit | Attempts | Meaning | Alert? |
|---|---|---|---|
| −1 | **38** (attempts 1–38, 13:37:24Z–15:53:28Z) | needle's died-without-exit-code sentinel — signal death | 1 alert bead each, 1:1 with kills |
| 124 | 4 (attempts 39–42, each exactly 600,000 ms) | timeout cap | no |
| 1 | 1 (attempt 43, 19 ms) | died before work started | no — bead quarantined 16:35:40Z (failure_count 5/5) |

**The alert storm maps 1:1 onto the kills.** All 38 `ALERT: Agent crash on bead
bf-2ildm` beads correspond to exactly one kill each (nearest-after match,
7.7–29.4 s later). No duplicates, no orphans inside the storm. This is the
known pre-0.4.2 one-alert-per-kill generation behaviour (cf. bf-4k2ws), not a
dedup failure.

**The carried stamps are not death times.** Every alert bead's crash stamp is
the crash handler's post-kill heartbeat 7.7–29.4 s after the real
`agent.completed` kill. Example (the per-instant record's subject): bf-z15pix's
stamp 15:01:52.450Z = attempt 24's real kill 15:01:35.775Z + 16.7 s. Cite kills
from `attempt-index.tsv`, never from an alert bead's stamp.

**The kill regime is historical and repaired.** The 38 kills belong to the
repository-bloat era (~18 GB repo, ~17 GB loose objects) — the same regime as
the same-day census (bf-65lsdu 127, bf-1ea4g 56, bf-4k2ws 55, bf-1s6c3 22
kills). The memcg-OOM mechanism (SIGKILL inside the 12 GiB per-dispatch scope,
git pack-objects the largest victim) is assigned **by regime match only**: no
Aug-13 kernel record survives (single journald boot begins 2026-08-15 19:56:33
EDT; the box rebooted twice on Aug-14). Live re-verification by this bead,
2026-09-07 ~19:2xZ: `.git` 105 MB, 167 loose objects / 1.27 MiB, 11,700
in-pack across 2 packs (99.78 MiB), 0 garbage, 0 unpushed commits — the regime's
preconditions are gone and the mitigation layer (gitignore → 10 MB pre-commit
gate → persistent pack-memory bounds → bounded safe-gc → daily/weekly timers)
holds.

## 3. Crash classification

The dispatch's four categories (FALSE_POSITIVE, SERVICE_FAILURE,
INFRASTRUCTURE, CODE_DEFECT) answer different questions, and bf-2ildm needs two
of them at two levels:

| Level | Question | Classification | Confidence |
|---|---|---|---|
| **Kill** — what happened to the 38 attempts on 2026-08-13 | real mid-task deaths? | **INFRASTRUCTURE** — repository-bloat regime; memcg-OOM mechanism by regime match | HIGH (per-attempt index; documented absence of kernel records) |
| **Alert** — should these alert beads have been raised / do they still demand action? | work lost? | **FALSE_POSITIVE** — stale. Target completed 2026-08-16 and closed successfully; nothing needed re-doing | HIGH (bead state + events.jsonl + dedup check) |

Explicitly **not** the other two: **SERVICE_FAILURE** — no 5xx/503 evidence
anywhere in the attempt series; the deaths are signal deaths in a git-heavy
dispatch scope, not gateway errors. **CODE_DEFECT** — the target completed
successfully on an unchanged codebase three days later, and no investigation in
this chain (or the corpus) has found a domain-check code defect; the failure was
in the workspace the agent ran in, not the code it ran.

Automated-tool results, for the record (Child 3, re-confirmed by the
classification docs): `crash-classifier.sh` returns **UNKNOWN** on every path —
`events.jsonl` begins 2026-08-16 so no Aug-13 machine-readable witness survives,
and the classifier's signals evaluate live state while the regime is historical.
Automated UNKNOWN is **not** evidence of no crash. `alert-deduplication.sh
check` returns **DUPLICATE** 38/38 ("crash target bf-2ildm is already
resolved") — the check leg now suppresses correctly.

## 4. Evidence for the classification

**Timestamps** (all UTC, from `docs/crashes/bf-2ildm/attempt-index.tsv` — 43
rows + header, re-counted 2026-09-07):

- First kill 2026-08-13T13:37:24Z; last kill 15:53:28Z; 38 kills over 2 h 16 m
  at a fixed ~2–5 min re-dispatch cadence.
- Successful retry 2026-08-16T22:28:44Z (exit 0); closure 2026-08-16T22:44:38Z.
- Last kill → closure: **3 d 6 h 51 m**. Every alert predates completion by
  3 d 6 h 35 m to 3 d 8 h 51 m — so the alerts were live and valid *at
  generation* and became stale only when the target closed.

**Exit codes:**

- Alert beads carry exit −1 — and that is **correct**, 38/38 against the
  crash-era worker log (`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl`,
  919 bf-2ildm records). The alerts were not fabricated.
- −1 is needle's sentinel (`code().unwrap_or(-1)`), not a signal number; a true
  SIGKILL encodes 137. No kernel record survives to upgrade it for this bead.
- The exit-0 "actual trace" in `.beads/traces/bf-2ildm/` belongs to the
  2026-08-16 success: the single-slot archive was overwritten by the later run.
  **The 2026-09-02 investigation read that surviving trace as "did NOT crash" —
  the retention blind spot, superseded by ef960d2.**

**Bead status:**

- Target bf-2ildm: Closed, rev 7, zero reopen events (domchk-0d6cd2b9;
  re-verified live 2026-09-07).
- Alert beads: **28 of 38 closed, 10 not** — live recount 2026-09-07 ~19:1xZ,
  matching Child 3's count exactly. Remaining: open bf-30imq, bf-1wkda,
  bf-2uo5sa, bf-z15pix, bf-4fvi9h, bf-o6vbwl, bf-3homxa; in_progress bf-4q1bda,
  bf-435w94, bf-35ajx2 (last touched 2026-09-02 — stale assignments, not active
  work). Ledger drift since Child 1's ~14:00Z snapshot: bf-66sw7c was closed
  2026-09-07T18:14:08Z by the RCA leg.
- `alert-deduplication.sh check` → DUPLICATE 38/38.

## 5. Corrections this synthesis carries forward

The chain's own 2026-09-02-era records contain three claims the 2026-09-07
evidence corrects. They are dated-corrected here (and in the parent bead's
notes) so no later reader inherits them:

1. **"Alert generated 3+ days BEFORE bead completion (impossible)"** — not
   impossible. The alerts fired seconds after the real 2026-08-13 kills; the
   target then completed on 2026-08-16 retry. Alert-before-completion is the
   expected order for a genuine crash whose retry later succeeds.
2. **"Use of placeholder data (exit code −1) instead of actual trace data"** —
   the 38 alert exit codes match the crash-era worker log 38/38. No placeholder
   existed. The misleading artifact was the *retained* trace (the Aug-16
   success), i.e. single-slot overwrite, not placeholder data.
3. **"21+ duplicate alerts … no duplicate prevention"** — the storm is
   one-alert-per-kill (38 kills → 38 alerts, no intra-storm duplicates). The
   duplicate-*looking* pile is many kills, not a dedup failure; and post-fix,
   the dedup check leg returns DUPLICATE 38/38 correctly.

Also superseded: `docs/investigations/root-cause-determination-bf-2ildm-2026-09-02.md`
("bf-2ildm did NOT crash") — true at the alert level only, false at the kill
level; dated supersession banner already prepended by ef960d2.

The alert-level verdict and its six fixes are **unchanged** by all of the above:
the target resolved, no work was lost, and the fixes (closed-bead filter,
dedup, exit-code validation, completion awareness, cooldown, classification)
address real defects regardless of which artifacts misled the 09-02 read.

## 6. Alert-system bugs identified (listing only; ownership noted)

1. **Classification wiring** — `crash-alert-manager.sh` reads the classifier's
   first stdout line as `CLASSIFICATION`, but `crash-classifier.sh main()`
   prints its `====` banner first, so the FALSE_POSITIVE branch is dead code and
   the cooldown keys on a constant string. Owner: open bead **domchk-f6fff20f**
   (fix in flight in the co-tenant worktree; scripts untouched by this chain per
   the shared-worktree hazard). Cascade prevention is unaffected (replay-verified
   2026-09-07, 25eda96).
2. **Single-slot trace retention** — the last attempt overwrites all prior
   ones; it manufactured the false "did NOT crash" conclusion and starves the
   classifier. Compensating control: retrieval bundles with per-attempt indexes
   (domchk-ea755548). Candidate improvement, no owner: multi-slot retention
   keyed by (bead, attempt).
3. **Alert-stamp provenance** — alert beads carry the handler heartbeat, never
   the kill instant; every dispatch-named "crash instant" in this family is a
   heartbeat. Documented control: cite kills from the per-attempt index.
   Candidate: stamp the kill instant on the alert bead.
4. **Signal retention floor for classification** — `.beads/events.jsonl`
   begins 2026-08-16, so any storm older than that classifies UNKNOWN by
   machine and needs a manual pass over the worker log. Documented limitation,
   no owner.
5. **No stale-alert sweep** — nothing closes an alert when its target resolves;
   10 of these 38 are still open 25 days after the target closed (§4). This is
   the gap the remediation bead below fills for this storm; a generic
   target-resolved sweep would prevent the next accumulation.
6. **Per-clone hook protection** — the 10 MB pre-commit gate is per-clone; a
   fresh clone is unprotected until `setup-git-hooks.sh install`. Documented in
   CLAUDE.md (repo-side, adjacent to this regime).

## 7. Next steps — additional beads needed

**Needed and created by this bead: domchk-4a05973f** — "Verify and close the 10
remaining stale bf-2ildm storm alerts". It carries the bead list, the evidence
citations, the per-bead method (matching the bf-66sw7c / bf-26r8bi disposition:
crashes genuine / work never lost / alert stale), and the cautions
(`update --notes` REPLACES; `--if-revision`; skip any bead that has gained
genuine new work). No other remediation of this storm is owed.

**Needed but already owned elsewhere (no new bead):**
- domchk-f6fff20f — the CLASSIFICATION wiring fix (§6.1).
- The kill regime's repo-side mitigation — closed; verified holding live
  (§2). Nothing new owed (ef960d2 §6 reached the same conclusion).

**Optional, not created (candidate, would need an owner to be worth a bead):**
multi-slot trace retention and kill-instant stamping (§6.2, §6.3). Both are
quality improvements with compensating controls in place; a bead with no owner
in this pool is noise, so this report records them as candidates only.

**Out of scope, signposted:** the wider open pool of beads mentioning bf-2ildm
(23 open/in_progress besides this chain and the 10 storm alerts — mostly
auto-split copies of this same investigation template, plus a few unrelated
beads that mention bf-2ildm in their body). They are stale by the same proof as
the storm alerts, but per corpus convention each is verified and closed by its
own chain (verify the bead's deliverable, then refuse-and-close) rather than by
a bulk close from this one. The two documents in §1 (classification e3a8820,
RCA ef960d2) are the reusable evidence any of those chains needs.

## 8. Sources

- Evidence bundle: `docs/crashes/bf-2ildm/` — `attempt-index.tsv` (43 rows),
  `crash-alert-ledger.tsv` (38 rows), `README.md`,
  `trace-archive-current-state.json` (domchk-ea755548)
- Classification: `docs/crash-classification-bf-2ildm-2026-08-13-15-01.md`
  (e3a8820); companion `docs/crash-classification-bf-2ildm-2026-08-13-14-40.md`
  (d82b6a2)
- Root cause: `docs/investigations/bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md`
  (ef960d2), including its §5 live-verification table and §6 residual gaps
- Superseded: `docs/investigations/root-cause-determination-bf-2ildm-2026-09-02.md`
  (banner prepended by ef960d2); the parent bead's 2026-09-02 notes (§5 above)
- Live store, re-run by this bead 2026-09-07: `bead show bf-2ildm` (Closed,
  rev 7); alert-bead status recount 28 closed / 7 open / 3 in_progress;
  `wc -l` on both bundle TSVs; `du -sh .git` (105 MB); `git count-objects -vH`
  (167 loose / 1.27 MiB, 11,700 in-pack, 2 packs, 99.78 MiB, 0 garbage);
  `git rev-list --count origin/main..HEAD` (0)
- Chain notes: `bead show` on domchk-ea755548, domchk-0d6cd2b9,
  domchk-1b80364e, domchk-58866353, domchk-41508c5c, domchk-48231aec,
  domchk-a863a1f9 (all as of 2026-09-07)

---
*Authored by domchk-970b6ca1 (Child Bead 4), 2026-09-07. Synthesis only — no
code, scripts, or alert-system behaviour was changed by this bead.*
