# Crash Investigation: bf-mje3pd

**Subject bead:** bf-mje3pd — "Implement fix and verify agent crash prevention" (P2,
Closed rev 2, 2026-08-17T00:15:35Z)
**Documentation bead:** domchk-354eecb0 (2026-09-07) — final consolidation leg of the
bf-mje3pd root-cause chain
**Crash date:** 2026-08-13, window 18:53:50Z → 21:18:36Z (2h24m46s, 14 dispatches)
**Task-named crash instant:** 2026-08-13T19:18:56+00:00, exit code −1
**Status:** ✅ INVESTIGATION COMPLETE — crashes genuine, work delivered in full, cause
identified, regime repaired and holding; remaining alert beads are stale, not false

**This is the canonical bf-mje3pd report.** It consolidates the nine prior legs of the
bf-mje3pd investigation chain (§8) into the structure the final documentation bead was
tasked with, and carries every load-bearing figure re-verified first-hand — most recently
by the chain's root-cause determination (domchk-7a3b46fa) and again live by this dispatch
(§9). It supersedes the attempt count and cadence figures in
[`docs/verification/bf-mje3pd-crash-analysis.md`](../verification/bf-mje3pd-crash-analysis.md)
(see §7) and is consistent with all nine sibling records.

---

## 1. Summary

- **Bead ID:** bf-mje3pd — implement repository-bloat and OOM-crash prevention for this
  workspace, following the bf-4yjq root-cause analysis
- **Crash window:** 2026-08-13 18:53:50Z (first claim) → 21:18:36Z (`bead.orphaned`)
- **Task-named crash instant:** 2026-08-13T19:18:56+00:00 — **proven a release heartbeat,
  not a kill**: it is alert bead bf-56kmlk's `Timestamp` field
  (`HANDLING_RELEASE_DONE` at 19:18:56.178350930Z), emitted **12.945 s after** the kill it
  names (attempt 4, `outcome.classified` exit −1 at 19:18:43.233798Z)
- **Exit Code:** −1 — needle's *death-by-signal sentinel*, not a signal number. The
  corpus reading "exit −1 = SIGKILL" is retired; SIGKILL via memcg-OOM is the era's
  kernel-proven mechanism (bf-198ne, bf-1ea4g) but is **not kernel-proven for this bead**
  (its kernel records were destroyed by the Aug-14/15 reboot)
- **Classification:** INFRASTRUCTURE — repository-bloat regime, workspace-scoped
- **Root Cause:** repeated death-by-signal terminations of the agent worker during
  git-heavy work *inside the ~18 GB bloated repository the bead was fixing*, within the
  12 GiB memcg dispatch scope. Mechanism **regime-matched, not kernel-proven**
- **Confidence:** HIGH on the category, MEDIUM on the mechanism, HIGH on not-a-code-defect
  and on the crashes being real
- **Outcome:** zero work lost — the bead delivered on dispatch 14 (21:18:23Z) and closed
  2026-08-17; the scripts it shipped are ancestors of the prevention stack that ended the
  regime (verified live 2026-09-07, §6)
- **Not a domain-check code defect.** No investigation in this workspace has ever found
  one; bf-mje3pd's deliverable is repo-maintenance scripts, and the deaths are signal
  deaths of the *agent worker*, not application exits

## 2. Timeline (UTC, 2026-08-13)

Verified 14-dispatch table, re-derived first-hand from the primary worker log
(`claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl`, 281 bf-mje3pd events):

| # | Dispatch | End | Exit | Outcome |
|---|---|---|---|---|
| 1 | 18:53:50 | 19:03:11 | −1 | crash |
| 2 | 19:03:26 | 19:10:10 | 1 | failure |
| 3 | 19:10:30 | 19:15:17 | −1 | crash |
| 4 | 19:15:36 | **19:18:43** | **−1** | crash — **the kill behind this task's input timestamp** |
| 5 | 19:19:04 | 19:21:55 | −1 | crash |
| 6 | 19:22:08 | 19:27:13 | 1 | failure (+ handling timeout 19:28:48) |
| 7 | 19:28:51 | 19:32:37 | −1 | crash |
| 8 | 19:33:08 | 19:36:39 | −1 | crash (+ handling timeout 19:37:29) |
| 9 | 19:37:41 | 19:42:59 | 1 | failure |
| 10 | 19:43:42 | 19:43:53 | — | mitosis eval; completed with **no** classified outcome |
| 11 | 19:43:56 | 19:46:33 | −1 | crash → `outcome.handled action=alerted` 19:47:00 |
| 12 | 19:47:03 | *no record* | — | death bracketed: `peer.crashed` for this worker at 20:36:55Z, 49m52s in — the worker died with it |
| 13 | 21:00:14 | 21:10:14 | 124 | timeout (600 s) → deferred |
| 14 | 21:10:33 | **21:18:23** | **0** | **success** → `verification.passed` → `bead.orphaned` |

Census: 14 dispatched / 13 completed / 12 classified = **7 × (exit −1, crash)**,
3 × (1, failure), 1 × (124, timeout), 1 × (0, success).

Anchor points around the table:

- **18:25:38Z** — bead created
- **19:02:32Z** — `ea23bd1` "implement OOM crash prevention and repository health
  monitoring" (39 s before attempt 1's kill)
- **19:18:43.227Z → 19:18:43.233Z → 19:18:56.178Z** — attempt 4's full bracket:
  `agent.completed` exit −1 → `outcome.classified` crash → `HANDLING_RELEASE_DONE`
  heartbeat. The task's named instant matches the heartbeat to ~0.2 ms. Under pre-0.4.2
  needle one alert bead is minted per kill, so several alert stamps name this bead
- **19:21:41Z** — `164b62d` "add preventive scripts for repository bloat and OOM crash
  prevention" (14 s before attempt 5's kill)
- **21:18:23.229Z → 21:18:23.232Z → 21:18:23.247Z** — success tail: `agent.completed`
  exit 0 → `success` → `verification.passed` (gates_run 1); **21:18:36.324Z**
  `bead.orphaned`
- **2026-08-17T00:15:35Z** — bead Closed (rev 2)

Alert beads minted during the loop: bf-1y1d0g (19:03:21), bf-1pidqn (19:15:29),
bf-56kmlk (19:18:56), bf-1cezsk (19:32:58), bf-x88dnf (19:37:19), bf-3dxljn (19:46:57),
bf-3za7vh — seven in total, each named to this one bead.

## 3. Evidence

**Available and used:**

| Artifact | Content |
|---|---|
| `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` | Primary record — 281 bf-mje3pd events (claims, dispatches, completions, outcome classifications, releases) |
| All six Aug-13 worker logs | Workspace-scoping census (§4) |
| Bead store | bf-mje3pd Closed rev 2; seven alert beads naming it; neighbours bf-29h1yy / bf-2o7nlw / bf-65lsdu all Closed |
| Git objects | `ea23bd1`, `164b62d` — the bead's in-loop deliverable commits, reachable only from `pre-squash-history-20260816` (not `main`); content classes live at HEAD |
| Era kernel records | bf-198ne (`CONSTRAINT=MEMCG`, usage == limit == 12 GiB) and bf-1ea4g — SIGKILL memcg-OOM of pack-objects, the regime's kernel-proven analogues |

**Missing / destroyed** (limits what can be claimed):

- **Agent transcript** — deleted by the `mend` zero-activity cleaner (recorded deletions
  2026-08-13T20:36:55Z, 21:06:58Z, 2026-08-14T04:38:26Z)
- **Kernel / journald records for the window** — the single surviving boot starts
  2026-08-15 19:56:33 EDT; the Aug-14/15 reboot discarded everything earlier. No record
  can confirm or deny a memcg-OOM kill *for this bead*
- **Per-bead trace** — single-slot retention predates the Sep-1 `.beads/logs/` era;
  `crash-classifier.sh bf-mje3pd` → exit 2 "Bead trace not found"

## 4. Analysis

### 4.1 Why INFRASTRUCTURE — five converging lines

1. **Exit signature.** 7 of 12 classified outcomes are exit −1 — death by signal,
   infrastructure-class by the guide's decision tree and by construction. The three
   exit-1 failures are secondary churn inside the same loop, not its defining mode.
2. **Mid-task at the instant.** At the heartbeat the bead was 4 attempts into a
   14-attempt loop with 9 dispatches ahead; terminal success came 2h later and the bead
   stayed Open until Aug-17.
3. **Workspace-scoped, not host-wide.** ±1 h window (18:18:56–20:18:56Z) census across
   all six Aug-13 worker logs: **lab-domain-check 10 × exit −1; all five peers
   (drawrace, roam-1, roam-2, s1, test-fix) zero** in-window and ≤2 on their whole day.
   Day-level: 344 kills / 13 beads, all on one worker. A host-wide OOM or service outage
   does not localize to one workspace's resource footprint; repo-local bloat does.
4. **Regime match with kernel-proven analogues.** The workspace was in its documented
   ~18 GB / ~17 GB-loose state; the same era's kernel-proven kills are memcg-OOM SIGKILL
   of pack-objects during git operations, and bf-mje3pd was doing exactly that class of
   work at exactly that cadence.
5. **The regime's own tell.** The day's top victim is bf-65lsdu — "Run repository cleanup
   to eliminate 17GB bloat" — with **127 kills**. Cleanup beads topping the kill table is
   what a bloat regime predicts and nothing else does.

Surge thresholds were evaluated rather than assumed: window maximum 2-in-5min and
3-in-10min — neither the `CRASH_SURGE_THRESHOLD=3` nor the task framework's 10-in-10 rule
fires. What fits is the guide's low-and-slow environmental-regime corollary: the window is
~2% of a day-scale event (344 kills) it precedes by two hours.

### 4.2 Not a false positive

Rule 1's "completed <30 s before the timestamp" heuristic fires naively on `164b62d`
(14.7 s before attempt 5's kill) and is wrong on four first-hand facts: the bead was 4
attempts in at the named instant with 9 dispatches ahead; both commits are mid-loop
checkpoints (`164b62d` extends `ea23bd1`'s `check-repo-health.sh`); the loop ran on after
both kills (re-claims 1.9–17.6 s); the deliverable is live at HEAD. Per-attempt committing
against a 3–12 min kill cadence lands some commit/kill pairs within seconds *by
construction* — proximity alone is not completion evidence; bead state at the instant is.

### 4.3 Ruled-out alternatives

| Alternative | Verdict | Deciding evidence |
|---|---|---|
| Workflow failure (max-turns, bead-closing) | Ruled out | 0 `max_turns` in the 281 events; the bead did close (rev 2, Aug-17) |
| Service failure (gateway 503/502) | Ruled out | **0 status-shaped** 503/502 records. A naive substring grep "finds" seven — all nanosecond-timestamp fragments (e.g. `.351085030Z`), the documented census trap |
| SIGHUP cascade | Ruled out | 0 `sighup` records here or in the whole Aug-13 worker log; the era's kernel-proven kills are SIGKILL |
| Code defect (domain-check) | Ruled out | Signal deaths of the agent worker; deliverable is repo-maintenance scripts; corpus-wide zero-defect record |
| Disk / CPU exhaustion | No support | No record constrains either; memory is the only resource with both a kernel-proven mechanism and a regime match |
| FALSE_POSITIVE | Ruled out | Input instant is a heartbeat 12.9 s *after* a real classified kill; work 4 attempts in |
| Self-healed transient (as *cause*) | Ruled out | Crash→retry→success is real (attempt 14, zero work lost), but kills continued after the success (hourly −1 histogram 19/52/56 across hours 21–23) and the regime survived until 2026-09-01 — transient describes only the bead's completion |

## 5. Root Cause

**PRIMARY CAUSE: INFRASTRUCTURE — the workspace's repository-bloat regime.**

Repeated death-by-signal terminations of the agent worker (needle `exit_code: −1`) during
git-heavy work on a bead implementing bloat prevention *inside the bloated repository
itself* — the documented 2026-08-13 state of ~18 GB `.git` / ~17 GB loose objects — with
the 1.9–17.6 s automatic re-claim turning 7 kills into 14 dispatches inside the
12 GiB memcg dispatch scope.

**Mechanism status: regime-matched, not kernel-proven.** The Aug-13 kernel/journald
records were destroyed by the Aug-14/15 reboot, so no record survives to name the signal
for *this* bead. The match rests on cadence, workspace scoping, task type, and the era's
kernel-proven analogues (bf-198ne, bf-1ea4g — SIGKILL, memcg, pack-objects).

| Claim | Confidence | Basis |
|---|---|---|
| Category = INFRASTRUCTURE | **HIGH** | Five independent first-hand lines, zero contradicting evidence; five independent dispatches reached the same verdict |
| Mechanism = memcg-OOM-class kill in the bloat regime | **MEDIUM** | Regime-matched; kernel records for this bead destroyed |
| Not a code defect | **HIGH** | App code not in the death path; corpus-wide zero-defect record |
| Crashes real / not a false positive | **HIGH** | Mid-task at the instant; heartbeat provenance proven |

**Disposition of the alerts (separate question from the cause):** the crashes were genuine
and mid-task, so the alerts were correct when raised; bf-mje3pd closed 2026-08-17 with its
deliverable landed and verified, so the surviving alert beads are **stale, not false** —
they warrant no investigation (§6.3).

## 6. Resolution

### 6.1 The bead's own deliverable

bf-mje3pd shipped its prevention scripts in-loop despite the kills: `ea23bd1` (health
monitoring) and `164b62d` (bloat-prevention scripts). Both are reachable only from
`pre-squash-history-20260816` — not `main` — but their content classes are all live in
HEAD's tree, re-verified this dispatch: `scripts/check-repo-health.sh` (3545 bytes),
`scripts/cleanup-repo-bloat.sh` (2159 bytes), `scripts/pre-commit-repo-size-hook`
(4187 bytes). Nothing was lost.

### 6.2 The regime is repaired and holding

The bloat state that killed this bead was packed down on 2026-09-01 (verified 18 GB →
92 MB, `docs/crashes/bf-4yjq-cleanup-verification.md`) and the recurrence paths are closed
by the deployed prevention stack. **Live re-verification by this dispatch, 2026-09-07:**

| Layer | Mechanism | This dispatch |
|---|---|---|
| Source elimination | `.beads/` + `*.db` + `*.jsonl` gitignored | `git ls-files .beads` → **0** tracked files |
| Commit gate | pre-commit repo-size hook (10 MB/file, hard block on staged `.beads/`) | `setup-git-hooks.sh --check` → exit 0, hook installed and byte-identical to tracked source |
| Pack-memory bounds | `pack.windowMemory=2g`, `deltaCacheSize=1g`, `threads=1` — bounds bare gc **and** `git push` | `setup-git-gc-config.sh --verify` → exit 0; worst case ≈3072 MiB per pack run, within the 6 GiB ceiling for the 12 GiB dispatch scope |
| Bounded maintenance | `safe-git-gc.sh` (staged, checkpoint/resumable, preflight), `cleanup-repo-bloat.sh`, `check-repo-health.sh` | `check-repo-health.sh` → **exit 0**; "No unmanaged aggressive git gc/repack running"; 0 unpushed commits |
| Scheduled enforcement | systemd **user** timers (NixOS — no crontab) | **8 timers, all future-triggered**: service 2 min, resource 5 min, crash-pattern 10 min, alert-triage hourly, repo-health daily 02:00, auto-gc 02:30, gc 03:00, full gc Sun 04:00 |
| Threshold-triggered remediation | daily 02:30 `safe-git-gc.sh --auto-when-needed` under `MemoryMax=4G` | timer present and future-triggered |
| Alert hygiene | closed-bead filter, dedup, 5-min cooldown, classification | suites green at HEAD per the 2026-09-07 re-verifications recorded in `CLAUDE.md` |
| Alert triage sweep | hourly, report-only | `bf-56kmlk` / `bf-1pidqn` / `bf-x88dnf` all queued **`RESOLVED_TARGET`** (evidence `bead_closure` / target closed), swept 2026-09-07T22:00:59Z |

Repo state at verification: `.git` **106 MB** — 364 loose objects / 2.44 MiB, 2 packs /
99.78 MiB, 0 garbage. No bloat-era crash has recurred since 2026-08-17; the repo has been
re-measured healthy on 09-01, 09-06, and 09-07.

### 6.3 Alert-layer resolution

Seven alert beads were minted against this one bead during the loop. Disposition —
converged independently by at least five dispatches and re-verified live by this one:
**stale, not false; close as stale; no investigation owed.** Current state
(re-read live 2026-09-07):

| Alert bead | State |
|---|---|
| bf-1y1d0g, bf-3za7vh, bf-1cezsk | Closed |
| bf-3dxljn | **Closed** (rev 20) — retired via its own four-child split, disposition `docs/crashes/bf-3dxljn-alert-disposition-domchk-9eddbfb0-2026-09-07.md` |
| **bf-56kmlk** | **Open** — the alert whose timestamp this task's chain was dispatched on. Both blockers now satisfied: domchk-a187559a (Closed rev 4) and **domchk-354eecb0, this bead**. Ready to close |
| bf-1pidqn, bf-x88dnf | Open — same shape, same disposition, waiting on the alert-lifecycle-closure layer |

Suppression is verified working: a *fresh* alert on this target would be stopped at every
layer (`alert-deduplication.sh check bf-3dxljn` → DUPLICATE exit 0, "crash target
bf-mje3pd is already resolved"; closed-bead filter; completion awareness), but beads
minted on 2026-08-13 predate those fixes and no stack component retires an already-created
alert bead — hence stale, not false. The hourly sweep classifies all three survivors
`RESOLVED_TARGET` but is report-only by design.

**bf-56kmlk is ready to close** with the converged reason: *crashes genuine and mid-task;
work delivered in full on dispatch 14 with `verification.passed`; target Closed
2026-08-17; classification INFRASTRUCTURE / repository-bloat regime, regime-matched not
kernel-proven; not a domain-check code defect; alert predates the 2026-09-02 suppression
fixes — stale, close without investigation.* Suggested close reason recorded on the bead
itself (see the dated note appended this dispatch).

### 6.4 Residual gaps (owned elsewhere, not by this report)

1. **`scripts/cleanup-bloat.sh` at HEAD still ships the death command**
   (`git gc --aggressive --prune=now`) and `docs/repository-health.md` still documents it.
   Survivable today only because the pack-memory config bounds it; the repo's documented
   emergency procedure bypasses every bounded path.
2. **Alert lifecycle: detection landed, closure did not.** The sweep holds 222
   `RESOLVED_TARGET` alerts fleet-wide (3 of them this bead's family) with no closure leg
   to retire them.
3. **The two newest layers are not reproducible from git** — the alert-triage sweep, its
   installer/test/unit files, and `safe-git-gc.sh --auto-when-needed` exist only on disk.
4. **The amplifier is NEEDLE-side and unowned** — the 1.9–17.6 s re-claim loop
   (recorded as H-1 and G-9/G-10; external ask, not local work).
5. **Classifier capture-race shape still yields bare `UNKNOWN`**
   (`docs/crash-fix-strategy-domchk-3b605127-2026-09-07.md` §3.2).

## 7. Corrections to earlier bf-mje3pd records

Annotate-don't-rewrite: earlier records are left as written; this table is the
correction of record.

| Earlier claim | Verified correction |
|---|---|
| `docs/verification/bf-mje3pd-crash-analysis.md` (2026-09-02): "11+ crash attempts", 13-row table, "attempt 10 = brief success at 19:43:53" | **14 dispatches / 7 classified kills.** Attempt 10 (19:43:42 → 19:43:53) is the mitosis eval — completed with *no* classified outcome, not a success. The old table also omits attempt 12, the dispatch whose worker itself died (bracketed by `peer.crashed` 20:36:55Z). Its exec-summary framing "eventual success after cleanup" remains correct |
| "Repository bloat (18GB…) triggered the OOM killer" | Right at regime level; for this bead the mechanism is **regime-matched, not kernel-proven** — the kernel records were destroyed. "OOM killer" as observed fact is kernel-proven only for bf-198ne / bf-1ea4g |
| `exit code -1` read as a signal | **Sentinel for death by signal, no signal number** — the corpus reading "−1 = SIGKILL" is retired |
| Crash instant 19:18:56+00:00 taken as a kill | **Release heartbeat 12.945 s after** attempt 4's kill — the `Timestamp` field of alert bead bf-56kmlk |
| `docs/crash-mitigation-strategies.md` crontab proposals | This box is NixOS — systemd **user** timers are the mechanism, and 8 are installed and firing |

## 8. Cross-referenced investigation docs

**This chain (domchk-354eecb0's siblings, all committed 2026-09-07):**

| Leg | Bead | Deliverable |
|---|---|---|
| Classification | domchk-bf8c4fd3 | `docs/crashes/bf-mje3pd-crash-classification-domchk-bf8c4fd3-2026-09-07.md` (63ca904) |
| Artifact analysis | domchk-f0513fd1 → domchk-a4cc1326 | `docs/crashes/bf-mje3pd-crash-artifacts-analysis-domchk-a4cc1326-2026-09-07.md` (9b83c81) |
| Pattern / FP analysis | domchk-7dc8f0d6 | `docs/crashes/bf-mje3pd-pattern-analysis-domchk-7dc8f0d6-2026-09-07.md` (395cf89) |
| Root cause determination | domchk-7a3b46fa | `docs/investigations/bf-mje3pd-root-cause-determination-domchk-7a3b46fa-2026-09-07.md` (e1e5bf0) — the designated root-cause source this report consolidates |
| Evidence compilation | domchk-916a1e66 | `docs/investigations/bf-mje3pd-evidence-compilation-domchk-916a1e66-2026-09-07.md` |
| RCA (parallel chain) | domchk-1b407bff | `docs/crashes/bf-mje3pd-root-cause-analysis-domchk-1b407bff-2026-09-07.md` (7672f9e) |
| Classification (bf-3dxljn split step 2) | domchk-a47705c9 | `docs/crashes/bf-mje3pd-crash-classification-domchk-a47705c9-2026-09-07.md` (e7f5b97) |
| Prevention research | domchk-f7865662 | `docs/crashes/bf-mje3pd-prevention-research-domchk-f7865662-2026-09-07.md` — source of §6.4's gap list |
| Alert-suppression verification | domchk-8bec8a51 | `docs/crashes/bf-mje3pd-alert-suppression-domchk-8bec8a51-2026-09-07.md` |
| Alert disposition (bf-3dxljn) | domchk-9eddbfb0 | `docs/crashes/bf-3dxljn-alert-disposition-domchk-9eddbfb0-2026-09-07.md` |

**Earlier records:** `docs/verification/bf-mje3pd-crash-analysis.md` (domchk-9bc6579f,
2026-09-02 — superseded in part, §7);
`docs/notes/incident-resolution-bf-1y1d0g-bf-mje3pd-crash-2026-09-02.md`.

**Regime canon:** `docs/crash-analysis-bf-1s6c3-2026-09-06.md`,
`docs/crash-investigations/bf-4yjq-crash-investigation.md`,
`docs/crash-inventory-bf-1ea4g-summary.md`, `docs/crashes/bf-198ne-crash-report.md`,
`docs/crash-response-guide.md`, `docs/maintenance/repository-maintenance-guide.md`.

## 9. Verification record (domchk-354eecb0, this dispatch, 2026-09-07)

Every figure in this report traces to a first-hand re-execution — this dispatch's own
where marked, else the cited sibling leg's:

- **Repo state live:** `du -sh .git` → 106M; `git count-objects -vH` → 364 loose /
  2.44 MiB, 2 packs / 99.78 MiB, 0 garbage; `check-repo-health.sh` exit 0;
  `setup-git-gc-config.sh --verify` exit 0 (worst case ≈3072 MiB); 0 unpushed commits;
  `git ls-files .beads` → 0; hook installed (`setup-git-hooks.sh --check` exit 0);
  8 `domain-check-*` timers all future-triggered
- **Deliverable content classes:** `git cat-file -e HEAD:scripts/{check-repo-health.sh,
  cleanup-repo-bloat.sh,pre-commit-repo-size-hook}` all present; sizes as in §6.1
- **Commit reachability:** `git branch -a --contains` → `ea23bd1` and `164b62d` reachable
  only from `pre-squash-history-20260816`
- **Bead store live re-read:** bf-mje3pd Closed rev 2; bf-3dxljn **now Closed** (rev 20 —
  retired via its own split since domchk-7a3b46fa read it Open); open alerts down to three
  — bf-56kmlk, bf-1pidqn, bf-x88dnf; bf-1cezsk / bf-1y1d0g / bf-3za7vh Closed
- **bf-56kmlk blocker graph** (`bead list --json`): blocked by domchk-a187559a (Closed
  rev 4) and domchk-354eecb0 (this bead) — nothing else
- **Triage queue** (`.beads/state/alert-triage/queue.jsonl`, swept 2026-09-07T22:00:59Z):
  all three open alerts `RESOLVED_TARGET`, evidence `bead_closure` / `target_status:
  closed`

## 10. Lessons Learned

1. **Alert timestamps are not kill instants.** This chain was dispatched on an instant
   that is a release heartbeat 12.945 s after the death it names. Grep the nanosecond
   stamp in the forensic log and read back to `outcome.classified` before anchoring any
   analysis on a `Timestamp:` field.
2. **`exit −1` is a sentinel, not a signal number.** Reading it as "SIGKILL" produces
   confident-sounding wrong mechanism claims; the kernel record is the only thing that
   pins a signal — and kernel records here die with the reboot. Preserve them (journald
   persistence is an open evidence-retention gap).
3. **Say "regime-matched" when it is regime-matched.** The honest split — HIGH on
   category, MEDIUM on mechanism — costs nothing and keeps the record falsifiable. Five
   dispatches agreeing on a category is convergence; none of them witnessed the signal.
4. **Repo-local scoping is the discriminator.** One worker at 10 kills in-window while
   five peers sit at zero rules out host-wide OOM and service outage in one move, and is
   exactly what repo-local bloat predicts.
5. **Cleanup beads topping the kill table is the regime's tell.** The day's top victim
   was the 17 GB-cleanup bead itself. When the beads trying to fix the repository are the
   ones dying, look at the repository.
6. **Proximity of a commit to a kill is not completion evidence.** With per-attempt
   committing against a 3–12 min kill cadence, sub-30-s commit→kill pairs arise by
   construction; bead state at the instant decides.
7. **Naive greps fabricate census figures.** Seven "503" substring hits in this bead's
   events are all nanosecond-timestamp fragments. Context-aware matching is the floor for
   any count that ends up in a report.
8. **One alert bead per kill pre-0.4.2 means stale alerts accumulate** — seven minted
   against one bead, three still open three weeks after their target closed. The
   lifecycle gap turns every stale alert into re-dispatchable bait: this bead is the
   tenth investigation spawned by one closed target. Verify the target's state before
   investigating any alert.
9. **The loop's deaths did not cost the work.** In-loop commits plus automatic re-claim
   carried the deliverable through seven kills; per-attempt committing is what made
   post-mortem localization possible at all.
