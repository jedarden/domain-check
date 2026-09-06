# Crash Classification: bf-1s6c3 (2026-08-12 → 08-13 storm)

**Classification bead:** domchk-56b5ba67
**Date classified:** 2026-09-06
**Method:** `docs/crash-response-guide.md` — Quick Reference exit-code table (rows 1–2), note 2
(`exit -1` sentinel), False Positive Detection heuristics (Rules 1–3), Common Crash Patterns
(Pattern 3)
**Evidence basis:** primary-source timeline from the predecessor investigation bead
**domchk-1fb4ad35** (closed; deliverable `docs/crashes/bf-1s6c3-crash-storm-timeline-2026-09-06.md`,
commit c562ca3), re-verified live for this classification: exit-code census recounted, attempt-4
death window re-read directly from
`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl` (3,589,712 bytes,
unchanged since Aug-12), merge commit `42a7b07` re-inspected with git, bead state and repo
health re-checked live 2026-09-06.

---

## Classification

### Crash type: `infrastructure` — repository-bloat sub-type (Pattern 3)

**Primary class: Infrastructure. Confidence: high (~95%).**
**Sub-type: repository bloat. Confidence: high (~90%), epistemic caveat below.**

**Alert disposition: no action** — the subject bead completed and closed (see False-Positive
Checks), the repository condition is repaired and holding, and the guard layers that would
prevent recurrence are in place. The crash type and the alert disposition answer different
questions: what killed the worker (infrastructure, repository bloat) versus what remediation
the alert warrants today (verification only — the work is already done).

## Verified crash facts

All figures below were re-derived for this classification, not copied.

| Fact | Value | Verification |
|---|---|---|
| Storm span | 2026-08-12T21:36:44Z → 2026-08-13T02:01:22Z (265 min) | recounted from worker log |
| Dispatches | 76 completions: **71 × exit −1**, 4 × exit 124 (600 s timeout), 1 × exit 0 | recounted; matches committed timeline (c562ca3) |
| Cadence | median inter-completion gap **177 s**, max 613 s | recounted |
| Kill density | **2.68 kills / 10 min** (16.1/hour) | recounted — relevant to FP Rule 3 below |
| Attempt-4 death | `agent.completed` **2026-08-12T21:48:06.650Z**, exit −1, duration 285,151 ms | read from live log |
| Attempt-4 deliverable | merge `42a7b07` "Merge reconciliation: Forgejo and GitHub remote histories", **21:47:07Z** | `git log -1 42a7b07` live; survives only on branch `pre-squash-history-20260816` |
| Commit → death gap | **59.6 s** (commit 21:47:07Z → kill 21:48:06.650Z) | both events verified first-hand |
| Re-dispatch loop | `bead.released` 21:48:16.519Z → `bead.claim.succeeded` 21:48:18.804Z → `agent.dispatched` 21:48:18.815Z (~10 s cycle) | read from live log |
| Subject bead state | **closed** (close event 2026-08-16T14:00:13Z, actor `system`; reason cites merge as `7dd79eb` — a dead pre-squash SHA of `42a7b07`) | `bead list` live + predecessor's forensic read |
| Deliverable on main | `42a7b07` is **not** an ancestor of `main`; main's reconciliation is `46293c5` (2026-08-17) | `git merge-base --is-ancestor` live |
| Repo at crash time | ≈18 GB `.git`, ≈17 GB loose objects (17+ identical ~237 MB `.beads/*.jsonl` snapshots committed) | canon-sourced — **not re-measurable**; objects no longer present |
| Repo today | `.git` 98 MB · 154 loose objects / 4.38 MiB · pack 90.93 MiB · 0 garbage · `fsck` clean · `.beads/` gitignored, 0 tracked files | measured live 2026-09-06 |

## Guide criteria applied

### 1. Exit-code mapping → infrastructure

71 of 76 completions are `exit -1` with zero exit-code variation among the deaths. Per the
guide's note 2, `-1` is needle's `wait()` sentinel for **died by signal, code unrecorded** —
not a signal number; the correct Unix encodings (137 SIGKILL / 129 SIGHUP) are absent from the
log for exactly the reason the note gives. The mapped class is **Infrastructure event**
(Quick Reference row 1).

### 2. Pattern 3 (repository bloat) signature refines the sub-type

| Pattern 3 criterion | bf-1s6c3 | Status |
|---|---|---|
| Fixed-cadence re-dispatch deaths, minutes apart, for hours | 71 deaths at median 177 s over 265 min; needle re-claimed within ~10 s of each death (`bead.released` → `bead.claim.succeeded` → `agent.dispatched`) | ✅ verified from live log |
| Repository > 5 GB | ≈18 GB `.git` with ≈17 GB loose objects at crash time | ✅ canon-sourced (not re-measurable) |
| Routine git operations trigger the kill | Task was itself a git operation (two-parent history merge) on the bloated repo; each attempt redid expensive git work | ✅ by task definition + log |
| Zero exit-code variation across deaths | All 71 deaths exit −1 | ✅ recounted |
| Sits inside a same-mechanism same-evening storm | Between bf-4yjq's 50 kills (17:54–20:30Z) and the bf-1s6c3 storm (from 21:36Z), same evening, same repo condition | ✅ per committed timeline + guide |

**Epistemic status (why 90%, not 100%, on the sub-type):** two legs rest on contemporaneous
documentation rather than re-measurable state. (a) The 18 GB repo size survives only in
cleanup-era docs — the offending blobs were packed away 2026-09-01 and the repo now measures
98 MB. (b) The specific kill mechanism (memcg `CONSTRAINT_MEMCG` SIGKILL) has no Aug-12 kernel
record — journald on this box begins 2026-08-15 19:46 EDT. The mechanism is kernel-proven only
for the better-instrumented later siblings (bf-4x12ec, bf-198ne). This is precisely why the
guide supplies the Pattern-3 signature as a detection heuristic: the classification rests on
the signature, which is fully present, not on a kernel record that cannot exist.

### 3. Excluded alternates

- **Workflow failure** — requires exit 1 + `error_max_turns`. None: every death is a signal
  death, and `transform.completed` succeeded on each attempt (template rendering was never the
  failure point).
- **Service failure** — requires HTTP 503/502 to the inference gateway. No 5xx recorded in any
  of the 76 attempts; no gateway-failure signature in the day log.
- **Code defect** — no application error surfaced in any attempt; the task never touched
  domain-check code (it was a git history reconciliation). Consistent with the standing
  finding that no domain-check code defect has ever been confirmed in this workspace's crash
  record.

## False-Positive Detection rules (all three applied)

| Rule | Threshold | bf-1s6c3 | Verdict |
|---|---|---|---|
| 1. Work committed < 30 s before crash → FALSE_POSITIVE | < 30 s | Attempt 4 committed `42a7b07` at 21:47:07Z and was killed 21:48:06.650Z — **59.6 s**, and the deaths were mid-attempt (median run 177 s), not post-completion cleanup | **Not triggered** |
| 2. Crash → retry → success → SELF-HEALED TRANSIENT | final retry exits 0 | The 76th attempt did exit 0 at 02:01:22Z — the surface pattern matches, but the cause was **persistent** (18 GB repo), not transient; nothing was healed, the retry loop outlasted the kills | **Surface match only — does not downgrade the classification** |
| 3. 10+ crashes in 10 min → INFRASTRUCTURE EVENT (system-wide) | ≥ 10 / 10 min | **2.68 kills / 10 min** | **Not triggered** |

**Rule 2 nuance (documented, not smoothed over).** The storm's last attempt succeeding is what
ended it, and the deliverable had in fact landed 4 hours earlier — so in the *alert* sense
there is a false-positive component: 72 of 76 dispatches ran against an already-satisfied task.
That is why earlier docs labeled this event "FALSE POSITIVE". The guide's FALSE_POSITIVE
classification carries the action "no action — close bead", and its premise (post-completion
cleanup death) does not hold here: workers died mid-task for the entire storm. The correct
two-layer reading, consistent with the bf-2xygo classification precedent (domchk-fe1e4a60):

- **What killed the workers:** Infrastructure — repository bloat (Pattern 3). Not a workflow
  artifact, not a service outage, not a code defect.
- **What the alert warrants:** nothing further. Subject bead closed 2026-08-16; the deliverable
  it described is represented on `main` by the later reconciliation `46293c5` (the bead's own
  close reason cites a dead SHA — any acceptance re-verification must use `46293c5`, not
  `7dd79eb` or `2832106`); the repo is repaired and re-verified (98 MB, fsck clean, 2026-09-06).

**Rule 3 nuance.** The 10-crashes/10-min heuristic detects *system-wide* events. The bloat
pattern is *per-repository and persistent* — lower instantaneous density (2.68/10 min, bounded
by each attempt's 2–8 min runtime) but deterministic and self-sustaining for 4.5 hours. The
infrastructure classification here rests on the exit-code mapping plus the Pattern-3 signature,
not on Rule 3; the rule's non-triggering is recorded because its absence is part of the
signature distinction, not a counterargument.

## Root cause statement

**Immediate cause:** every dispatch executed significant git work (merge construction across
diverged histories) against an ≈18 GB repository whose object store exceeded the dispatch
scope's memory budget; the kernel killed the worker mid-attempt each time (needle records the
unrecorded-code deaths as `exit -1`).

**Amplifying cause (why 4.5 hours):** needle's crash handler released and immediately
re-claimed the bead on a ~10 s cycle, and the task's deliverable had already landed on disk at
21:47:07Z (attempt 4, itself killed 60 s later) — no stop-condition existed for
"deliverable present, bead still open", so 72 further dispatches redid expensive git work
against satisfied work until one attempt happened to survive to completion (02:01:22Z).

**Underlying cause:** 17+ identical ~237 MB `.beads/*.jsonl` bead-state snapshots had been
committed to git, bloating the object store — the same underlying cause as bf-4yjq and
bf-2xygo earlier the same evening.

## Recommended remediation path

For the downstream child (domchk-9822e378 "Execute remediation based on classification").
Ordered by the classification, with live status 2026-09-06:

| # | Remediation | Status |
|---|---|---|
| 1 | Pack down the bloated object store (`safe-git-gc.sh`, never bare `git gc --aggressive`) | ✅ Done 2026-09-01; re-verified live today: 98 MB, fsck clean, 0 garbage |
| 2 | Prevent bead-state re-entry: `.beads/` gitignored, 0 tracked files; repo-wide `*.jsonl` rule | ✅ In place (`.gitignore:66`) |
| 3 | Pre-commit backstop blocking > 10 MB staged files | ⚠️ Installed at `.git/hooks/pre-commit` but **per-clone and drifted** from tracked `scripts/pre-commit-repo-size-hook`; no installer committed — the one open gap in this layer |
| 4 | Bound the bare-gc/push pack-objects path (`pack.windowMemory=2g`, `deltaCacheSize=1g`, `threads=1`) | ✅ Applied repo-local + global; `setup-git-gc-config.sh --verify` passes |
| 5 | Scheduled repo-health + bounded gc | ✅ Six systemd user timers installed and firing (re-verified 2026-09-06) |
| 6 | Re-dispatch stop-condition for satisfied work (the amplifier) | ❌ NEEDLE-side; outside this repo's control. Record it as the systemic finding — it is what converted one kill into 71 |

Net: **no further remediation action is required for this event in this repository.** Item 3 is
a hardening gap worth a bead; item 6 is the only lever that would have bounded the storm's
duration, and it lives in the NEEDLE fleet, not here.
