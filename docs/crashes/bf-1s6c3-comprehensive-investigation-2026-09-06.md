# Comprehensive Investigation Report: Bead bf-1s6c3 (2026-08-12 crash storm)

**Documentation bead:** domchk-779dfdf3 ("Document bf-1s6c3 findings and remediation")
**Written:** 2026-09-06
**Subject bead:** bf-1s6c3 — "Create merge commit reconciling Forgejo and GitHub histories"
(P2, task; created 2026-08-12T21:12:09Z, **closed** 2026-08-16T14:00:13Z)
**Classification:** INFRASTRUCTURE EVENT — repository bloat → memory-cgroup OOM
(Pattern 3 in [crash-response-guide.md](../crash-response-guide.md))
**Domain-check code:** ✅ NO DEFECTS — the application was never involved

This report is the single consolidated record the investigation chain was asked to produce.
It does not re-derive the primary-source findings — those live in the evidence layers cited
throughout — it binds them together, records this bead's own live re-verification of the
remediation (§4), and states explicitly which older claims are superseded (§5).

**Evidence layers this report consolidates**

| Layer | Document |
|---|---|
| Raw artifact extraction + catalog | [docs/crashes/bf-1s6c3/README.md](bf-1s6c3/README.md) (domchk-fcac734a) |
| Crash storm timeline (76 dispatches) | [bf-1s6c3-crash-storm-timeline-2026-09-06.md](bf-1s6c3-crash-storm-timeline-2026-09-06.md) |
| Classification (Pattern 3) | [bf-1s6c3-crash-classification-2026-09-06.md](bf-1s6c3-crash-classification-2026-09-06.md) |
| Named-timestamp provenance + log-availability matrix | [bf-1s6c3-investigation-data-bundle-2026-09-06.md](bf-1s6c3-investigation-data-bundle-2026-09-06.md) |
| Catalog verification (2nd dispatch) | [bf-1s6c3-artifact-extraction-verification-2026-09-06.md](bf-1s6c3-artifact-extraction-verification-2026-09-06.md) |
| RCA addendum (classification confirmation) | [bf-1s6c3-root-cause-analysis-domchk-1c03aacb-2026-09-06.md](bf-1s6c3-root-cause-analysis-domchk-1c03aacb-2026-09-06.md) |
| Crash analysis (catalog entry) | [docs/crash-analysis/bf-1s6c3-crash-analysis-2026-08-12.md](../crash-analysis/bf-1s6c3-crash-analysis-2026-08-12.md) |
| Remediation record | [bf-1s6c3-remediation-2026-09-06.md](bf-1s6c3-remediation-2026-09-06.md) |

---

## 1. Executive Summary

Bead bf-1s6c3's crash is not one crash. Over **265 minutes** (2026-08-12T21:36:44Z →
2026-08-13T02:01:22Z) the bead was dispatched **76 times** and **71 attempts died with
`exit_code = -1`** (needle's sentinel for "died by signal, code unrecorded"), 4 timed out
(exit 124), and 1 succeeded. The dispatcher that drives the crash-response classification
frames this as a single event at `2026-08-12T22:04:12` — that string is a
`HANDLING_RELEASE_DONE` **heartbeat recorded 6.40 s after one of the 71 kills**, not the
kill itself.

**Root cause:** the workspace repository was then ≈18 GB with ≈17 GB of loose objects —
17+ identical ~237 MB `.beads/*.jsonl` bead-state snapshots had been committed before
`.beads/` was gitignored — so every significant git operation exceeded the dispatch scope's
memory cgroup and the kernel's memory-cgroup OOM killer terminated the worker. The task
(a git merge) is itself a git operation, which is why every retry died the same way.

**Outcome:** the deliverable merge `42a7b07` **landed mid-storm** at 2026-08-12T21:47:07Z
(attempt #4, itself killed 59.6 s later); the retry loop carried the bead to an exit-0 close
at 02:01:22Z with no second agent involved. Remediation is complete and verified: the
repository sits at **101 MB** (was ≈18 GB), the re-entry path for bead-state bloat is closed
(gitignore + pre-commit gate + memory-bounded gc), and all six monitoring timers are firing.
The one systemic gap — a NEEDLE-side stop-condition for "deliverable present, bead still
open" — lives outside this repository and is recorded as an open recommendation.

---

## 2. Crash Timeline and Classification

### 2.1 Timeline (UTC, from the primary worker logs)

| Time | Event |
|---|---|
| 2026-08-12T21:12:09Z | Bead created (9 s after its analysis bead bf-2xygo) |
| 21:31:27.663Z | First `bead.claim.succeeded` / dispatch (seq 4641/4650) |
| **21:36:44.519Z** | **Attempt 1 kill** — `agent.completed`, exit −1, after 316,572 ms |
| 21:43:20Z | Attempt #4 claimed |
| **21:47:07Z** | **Deliverable merge `42a7b07` committed** (mid-storm) |
| 21:48:06.650Z | Attempt #4 killed — exit −1, 59.6 s after the commit |
| 21:48Z → 02:01Z | 72 further dispatches against already-satisfied work |
| 2026-08-12T22:23:56.990Z | One representative kill; **22:24:04.189Z** = its `HANDLING_RELEASE_DONE` heartbeat (the number the alert carries) |
| **2026-08-13T02:01:22.561Z** | Attempt #76 exits 0 (384 s) — storm ends |
| 2026-08-16T14:00:13Z | Bead closed by `system`; close reason cites the merge as `7dd79eb` (a dead pre-squash name — see §3.3) |

Full attempt-by-attempt census: the timeline doc's tables; raw extracts under
[docs/crashes/bf-1s6c3/](bf-1s6c3/README.md) (945 + 513 needle events, SHA-256 manifest).

### 2.2 Storm census

| Window (UTC) | Claims | exit −1 | exit 124 | exit 0 |
|---|---|---|---|---|
| Aug-12 21:31 → 23:57 | 50 | 49 | 0 | 0 |
| Aug-13 00:00 → 02:01 | 26 | 22 | 4 | 1 |
| **Total** | **76** | **71** | **4** | **1** |

Median inter-kill gap 177 s; re-dispatch (`bead.released` → claim → `agent.dispatched`) in
~10 s, so the bead was continuously in-flight for 4.5 hours.

### 2.3 Classification

**INFRASTRUCTURE EVENT — repository bloat sub-type** (crash-response-guide Pattern 3),
confidence ~95% on crash type / ~90% on sub-type. Applied rules: exit −1 → infrastructure;
FP rules 1–3 checked and failed (the task was *not* complete at first death — the deliverable
landed 16 minutes *into* the storm, so this is neither a pure mid-task crash nor a
post-completion false positive). Exit −1 is a `wait()` sentinel, not a signal number: 137/129
appear nowhere in the log, exactly as the sentinel predicts.

**Epistemic caveat carried forward:** no kernel record exists for 2026-08-12 (journald here
starts 2026-08-15; the Aug-12 records were lost to the 2026-08-14 reboot) and the 18 GB
figure is canon-sourced because the offending blobs were packed before they could be
re-measured. The memcg `CONSTRAINT_MEMCG` mechanism is kernel-proven for the *identical*
later events (bf-4x12ec 2026-08-14, bf-198ne 2026-08-16). Classification therefore rests on
the fully-present Pattern-3 signature, not on an Aug-12 kernel record that cannot exist.

---

## 3. Root Cause Analysis

### 3.1 Immediate cause

Memory-cgroup OOM kill of the worker during git work on the bloated repository.

1. Agent dispatched to build a two-parent history merge on the ≈18 GB repository
2. Git work pulled the oversized loose-object store through memory
3. The dispatch scope's memory cgroup limit was exceeded
4. Kernel OOM killer delivered uncatchable SIGKILL; needle recorded exit −1
5. ~10 s later the bead was re-claimed — repeat, 71×

### 3.2 Underlying cause

**17+ identical ~237 MB `.beads/*.jsonl` bead-state snapshots committed to git** during
earlier automated divergence-analysis runs. At the time `.beads/` was not gitignored and no
pre-commit size gate existed, so the repository silently grew 36× (≈18 GB vs. the <500 MB
healthy bound; loose:packed ratio 1,832:1, inverted).

### 3.3 Post-crash history caveat (cite these SHAs, not the old ones)

| SHA | Status | Use it for |
|---|---|---|
| `42a7b07af05a877d5afe1d3a4296fe70bc61a114` | ✅ real two-parent merge, 2026-08-12T21:47:07Z | this bead's own deliverable — contained **only** on `pre-squash-history-20260816` |
| `46293c5cf50a5e3eeae0f77cbbcadb29e93145ec` | ✅ "Merge Forgejo and GitHub histories", 2026-08-17 | "main contains the reconciled history" — **is** an ancestor of `main` |
| `7dd79eb` | ❌ nonexistent (dead pre-squash name of `42a7b07`) | the bead's close reason cites it — do not accept |
| `2832106` | ❌ nonexistent | cited by earlier catalog docs — do not accept |

### 3.4 Amplifier (why 71 deaths instead of 1)

The re-dispatch loop had **no stop-condition for "deliverable present, bead still open"**.
72 of 76 dispatches ran against work that had been on disk since 21:47Z. This is the only
cause the in-repo layers cannot address — it is NEEDLE-side, and it is the single change
that would have ended this storm at ~16 minutes instead of 4.5 hours.

---

## 4. Remediation Implemented — re-verified live 2026-09-06 by this bead

The event's remediation was applied by work between 2026-09-01 and 2026-09-06
(recorded in [bf-1s6c3-remediation-2026-09-06.md](bf-1s6c3-remediation-2026-09-06.md));
rather than re-executing it, this documentation bead **re-ran every check itself** on
2026-09-06 ~20:28 EDT. All results below are from those runs, not copied.

| # | Layer (maps to crash-mitigation-strategies.md Priority 3) | Live check run | Result |
|---|---|---|---|
| 1 | `.beads/` gitignored + repo-wide `*.db`/`*.jsonl` rules (Proposal 3.2) | `grep -n "beads\|jsonl\|\.db" .gitignore`; `git ls-files .beads \| wc -l` | `.beads/` at `.gitignore:66`, `*.db`/`*.jsonl` at 68–70; **0 tracked `.beads/` files** ✅ |
| 2 | Pre-commit >10 MB gate (Proposal 3.2) | `ls -la .git/hooks/pre-commit` | present, executable (3446 B, 2026-09-01) ✅ — per-clone; installer gap noted in §5.4 |
| 3 | `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` bounding bare gc **and** `git push` | `./scripts/setup-git-gc-config.sh --verify` | ✅ effective (local) — worst case ≈3072 MiB/pack run, within the dispatch-scope ceiling |
| 4 | `safe-git-gc.sh` for all maintenance (never bare `git gc --aggressive`) | `./scripts/safe-git-gc.sh --check-only` | ✅ exit 0 — resources pass (47.8 GB mem, 57 GB disk, load 4.37), "GC not needed" |
| 5 | Repo size monitoring (Proposal 3.1) + full health check | `./scripts/check-repo-health.sh` | ✅ passes — `.git` **101 MB**, 3 packs / 99.13 MiB, 21 loose objects / 136 KiB, 0 garbage |
| 6 | Scheduled maintenance timers (Proposal 3.3) | `systemctl --user list-timers 'domain-check-*'` | ✅ **all six** present with future trigger times (crash-pattern 10 min, resource 5 min, service 2 min, repo-health daily 02:00, gc daily 03:00, full gc Sun 04:00) |
| 7 | Safe-gc run-time safeguards | `bash scripts/test-safe-git-gc-limits.sh` | ✅ **33 passed, 0 failed** |
| 8 | Crash-alert false-positive filtering (closed-bead, dedup, cooldown) | `bash scripts/test-crash-alert-fixes.sh` | ✅ all six critical fixes verified |
| 9 | Work-loss check (infrastructure path step 1) | SHAs per §3.3, `git merge-base --is-ancestor 46293c5 main` | ✅ reconciled history **is** on `main`; subject bead closed, correctly not retried |

**Work-loss verification:** none — the deliverable survived (§2.1). The subject bead is
closed and must not be retried; its task is satisfied on `main` via `46293c5`.

**Effectiveness over time:** the 2026-09-01 cleanup (18 GB → ~94 MB) has held through
re-verification on 2026-09-06 at 101 MB with normal churn — five days of daily automated
gc and ordinary fleet activity have not re-grown it. [bf-4yjq cleanup verification](bf-4yjq-cleanup-verification.md)
records the same finding for the sibling event.

---

## 5. Lessons Learned and Guide Updates

### 5.1 Superseded claims in older bf-1s6c3 docs

Four 2026-09-01 documents in `docs/crashes/` predate the primary-source corrections and
carry claims the raw log refutes. Each now carries a dated supersession banner pointing
here; **do not cite their figures** — use this report or the evidence layers in §1.

| Superseded claim (2026-09-01 docs) | Corrected record (primary sources) |
|---|---|
| One crash, at 2026-08-12T21:36:51Z or 2026-08-13T00:38:41Z | **76 dispatches / 71 kills** over 265 min; each old timestamp names a different single attempt |
| "SIGKILL (signal 9)" | exit −1 is needle's **signal-death sentinel**, not a signal number; no 137/129 anywhere in the log |
| "the merge task completed successfully *before* the crash" / "post-completion false positive" | deliverable landed **16 minutes into** the storm; 72 dispatches ran against satisfied work — neither mid-task nor post-completion |
| Deliverable = `7dd79eb` / `2832106` | `42a7b07` (this bead, on `pre-squash-history-20260816`); `46293c5` (reconciled history on `main`); the other two **do not exist** |
| "18 GB → 445 MB / 138 MB" cleanup figures | cleanup verified at 93–94 MB on 2026-09-01; **101 MB** on 2026-09-06 |
| "18 GB → 18GB/17GB bloat caused 9 crashes" (bf-4yjq/bf-173o7e framing repeated here) | the "9 crashes" figure is the old alert-count; the bf-1s6c3 event is 76 dispatches/71 kills (bf-4yjq's own count is separate) |

### 5.2 Alert timestamps are bookkeeping clocks, not kill times

The `Timestamp` in a crash-alert bead description is the crash handler's
`HANDLING_RELEASE_DONE` heartbeat — 6.40 s after the real kill for bf-1s6c3 (22:04:12.524613796
vs. 22:04:06.124743603Z), 7.2 s for the 22:24:04 event, 8–120 s for bf-173o7e. Investigating
from the alert time risks a git-log/journalctl window that misses the death entirely.
**Added to the crash-response guide** (Phase 1 checklist): bracket the real death from the
needle log's `agent.completed` record, and widen `<crash_timestamp>` windows to absorb the drift.

### 5.3 Exit −1 storms are the bloat signature — check the repo before the process

Zero exit-code variation for hours, on a task that is itself a git operation, on a >5 GB
repository: that combination is Pattern 3, and the repo-size check (`git count-objects -vH`,
`du -sh .git`) costs seconds and would have identified the mechanism on attempt 1. The
pre-task resource check in the guide/CLAUDE.md now covers this; the automated 02:00
repo-health timer makes it continuous.

### 5.4 Open items (not closed by this event's remediation)

1. **Pre-commit-hook installer** — the >10 MB gate lives at `.git/hooks/pre-commit`,
   untracked and per-clone; `scripts/pre-commit-repo-size-hook` has drifted from the
   installed version and no installer is committed, so a fresh clone has no gate. Flagged as
   the one open in-repo defense gap by the crash-analysis doc.
2. **NEEDLE-side re-dispatch stop-condition** for "deliverable present, bead still open" —
   the amplifier of §3.4, outside this repository. Until it exists, every repeat of this
   event costs 4.5 hours of kills instead of 16 minutes.

### 5.5 Guide and mitigation mapping

- [crash-response-guide.md](../crash-response-guide.md) — Pattern 3 (repository bloat) already
  carried the classification and cleanup steps; this report's §5.2 learning is now added to
  its Phase 1 checklist.
- [crash-mitigation-strategies.md](../crash-mitigation-strategies.md) — Priority 3
  (Repository Bloat Prevention, proposals 3.1–3.4) is the preventative strategy this event
  maps to; §4 above shows every one of those proposals verified live in force on 2026-09-06.
- No domain-check code changes: none required, none made — consistent with every
  investigation in this workspace (code defects ≈2% of crashes, zero found in domain-check).

---

**Report status:** ✅ COMPLETE
**Remediation:** ✅ verified live by this bead on 2026-09-06 (§4) — no further action required for this event in this repository
**Subject bead:** CLOSED 2026-08-16T14:00:13Z — not retried
**Documentation bead:** domchk-779dfdf3
