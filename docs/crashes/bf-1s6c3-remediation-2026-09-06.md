# Crash Remediation Record: bf-1s6c3 (infrastructure — repository bloat)

**Remediation bead:** domchk-9822e378 ("Execute remediation based on classification")
**Date executed:** 2026-09-06
**Classification source:** `docs/crashes/bf-1s6c3-crash-classification-2026-09-06.md`
(domchk-56b5ba67, commit 9b92cd9) — **Infrastructure / repository-bloat sub-type
(Pattern 3), confidence ~95%**
**Path applied:** the guide's Infrastructure Event remediation steps, instantiated by
the classification's "Recommended remediation path" table (its §Recommended remediation
path is addressed to this bead). Every status in that table was **re-verified live on
2026-09-06 by this bead**, not copied.

---

## Disposition

**Action taken: none further / no retry.** The event's remediation was already applied by
work between 2026-09-01 and 2026-09-06; this bead re-verified each layer live and found
them all in force. Subject bead **bf-1s6c3 is closed** (2026-08-16) and must not be
retried — the task it described is satisfied on `main`, and the remotes have converged.

## 1. Work-loss verification (infrastructure path, step 1)

bf-1s6c3's task was "create a merge commit reconciling the Forgejo and GitHub histories."

| Check | Result (live 2026-09-06) |
|---|---|
| Storm-era deliverable `42a7b07` (attempt 4, 21:47:07Z) | exists, killed 59.6 s after commit, **never pushed**; survives only on branch `pre-squash-history-20260816` (`git branch --contains 42a7b07`) |
| On-main reconciliation `46293c5` "Merge Forgejo and GitHub histories" (2026-08-17) | **is an ancestor of `main`** (`git merge-base --is-ancestor 46293c5 main` → 0) |
| Forgejo `origin/main` vs local `main` | identical: `e299c48` (`git fetch` + `rev-parse`, no local-only commits) |
| GitHub mirror `refs/heads/main` | identical: `e299c48` (`git ls-remote`) |

**No work was lost.** The deliverable the bead existed to produce is present in current
history via `46293c5`'s lineage, and both remotes carry it. The storm's 72 redundant
attempts produced nothing that needs recovering; their cost was compute and alerts, not
work.

## 2. Remediation layers — classification table re-executed live

| # | Layer | Live verification (2026-09-06, this bead) | Status |
|---|---|---|---|
| 1 | Bloated object store packed down | `.git` 98 MB · 171 loose objects / 4.49 MiB · pack 90.93 MiB · **0 garbage** · `git fsck --full` exit 0 · `check-repo-health.sh` exit 0 | ✅ in force |
| 2 | Bead state cannot re-enter git | `.gitignore:66` `.beads/`, `:68` `*.db`, `:70` `*.jsonl`; `git ls-files .beads` → **0** tracked files | ✅ in force |
| 3 | Pre-commit >10 MB backstop | `.git/hooks/pre-commit` present (3446 B, installed 2026-09-01) but **drifted** from tracked `scripts/pre-commit-repo-size-hook` (both 111 lines, contents differ); no installer committed | ⚠️ gap — already tracked, see §5 |
| 4 | Bare-gc/push pack-objects memory bound | `setup-git-gc-config.sh --verify` exit 0 — effective bound resolves local: `windowMemory=2g`, `deltaCacheSize=1g`, `threads=1`, worst case ≈3072 MiB inside the 6 GiB ceiling for the 12 GiB dispatch scope | ✅ in force |
| 5 | Scheduled health + bounded gc | all six `domain-check-*` systemd user timers present with future Trigger times (service-monitor 2 min, resource-monitor 5 min, crash-pattern 10 min, repo-health daily 02:00, incremental gc daily 03:00, full gc Sun 04:00) | ✅ in force |
| 6 | Re-dispatch stop-condition for satisfied work | NEEDLE-fleet-side; outside this repository's control. Recorded as the systemic finding: it is what converted one kill into 71 | ❌ out of scope (documented) |

## 3. Retry decision (infrastructure path, step 3)

**bf-1s6c3 stays closed; no retry.**

- Bead state verified live: `Status: Closed` (close event 2026-08-16T14:00:13Z, actor
  `system`, per the classification bead's forensic read). Its close reason cites the merge
  as `7dd79eb` — a dead pre-squash SHA; **`46293c5` is the on-main representation** and any
  future acceptance re-verification must use that SHA.
- Every acceptance criterion the bead carried is satisfied by current history: merged
  history present, merge commit with explanatory message, both remote histories
  reconciled, zero divergence.
- A retry would manufacture a duplicate reconciliation against already-converged remotes —
  exactly the redundant-work shape that lengthened the original storm.

**Supersession note.** `docs/crash-fix-implementation-report-bf-1s6c3-2026-09-01.md`
(domchk-ee6d185d) reached a remediation conclusion for this crash on the **superseded**
mechanism ("SIGHUP cascade", "no OOM events found", "retry safe only when the inference
gateway is healthy"). The verified mechanism is memcg-OOM SIGKILL of `git push`'s
pack-objects inside the 12 GiB dispatch scope, per the classification and the raw-log
extraction. Its conditional-retry guidance is superseded by this record: there is nothing
to retry — the bead is closed and its work is on `main`.

## 4. Documentation updates (infrastructure path, step 4)

- This file — the remediation completion record for the chain.
- Crash documentation index (`docs/crashes/crash-documentation-index-2026-09-02.md`),
  bf-1s6c3 section: entry added below.
- Chain to date: `docs/crashes/bf-1s6c3/` raw-artifact bundle (collection bead
  domchk-fcac734a) → `docs/crashes/bf-1s6c3-crash-storm-timeline-2026-09-06.md`
  (domchk-1fb4ad35, c562ca3) → `docs/crashes/bf-1s6c3-crash-classification-2026-09-06.md`
  (domchk-56b5ba67, 9b92cd9) → **this remediation record (domchk-9822e378)**.
- No code changes: domain-check source is untouched by this bead, consistent with the
  standing finding that no crash in this workspace has ever been a domain-check defect.

## 5. Follow-up

1. **Pre-commit hook installer (layer 3 gap)** — deliberately **not** implemented here: it
   already has open tracking beads (`domchk-96810f05` install with executable permissions,
   `domchk-8a15e987` test, `domchk-f514cf2d` document; `bf-558qpy` and `domchk-08a56bdd`
   in progress) and in-flight uncommitted work (`scripts/setup-git-hooks.sh`,
   `scripts/test-setup-git-hooks.sh`, untracked 2026-09-06). Creating a duplicate bead
   would add exactly the duplicate-tracking noise this fleet's alert history warns about.
   When those land, a fresh-clone `setup-git-hooks.sh --check` run closes the layer.
2. **Split-chain workflow debt** — bf-1s6c3's 2026-08-13 split children are still live
   (`bf-31p3g` InProgress, `bf-7d8l5` and `bf-6b0fl` Open). Their objective is already
   satisfied by `46293c5` and the verified zero-divergence state, so they are
   close/release candidates for whoever holds them — not this bead's action.
3. **Monitoring** — nothing new required. The daily 02:00 repo-health timer already
   re-checks layers 1, 2 and 4; the classification's item 6 remains a NEEDLE-fleet
   finding with no repository-side lever.

## Acceptance-criteria mapping

| Criterion | Where satisfied |
|---|---|
| Appropriate remediation applied for classification | §2 — all in-repo layers verified in force; no further action warranted |
| Bead bf-1s6c3 status resolved | §3 — closed, no retry, with the dead-SHA caveat recorded |
| Remediation documented in crash tracking | §4 — this record + index entry |
| No unnecessary code changes | §4 — documentation-only commit |
