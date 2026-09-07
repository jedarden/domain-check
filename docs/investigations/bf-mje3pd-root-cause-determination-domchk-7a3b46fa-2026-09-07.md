# bf-mje3pd — Root Cause Determination

**Determination bead:** domchk-7a3b46fa (2026-09-07) — synthesis leg of the
bf-mje3pd root-cause chain
**Target bead:** bf-mje3pd — "Implement fix and verify agent crash prevention"
(Closed rev 2, 2026-08-17T00:15:35Z, deliverable landed 2026-08-13T21:18:23Z)
**Crash window:** 2026-08-13, 18:53:50Z first claim → 21:18:36Z `bead.orphaned`
(2h24m46s, 14 dispatches)
**Input instant given to this chain:** 2026-08-13T19:18:56.178560656Z, exit −1 —
proven a release heartbeat, not a kill (§2)

**Inputs (all three child legs closed, deliverables committed):**

| Leg | Bead | Deliverable | Commit |
|---|---|---|---|
| Classification | domchk-bf8c4fd3 | `docs/crashes/bf-mje3pd-crash-classification-domchk-bf8c4fd3-2026-09-07.md` | 63ca904 |
| Artifacts | domchk-f0513fd1 | closed verification-only; names domchk-a4cc1326's analysis as the artifact record | (9b83c81) |
| Artifact analysis | domchk-a4cc1326 | `docs/crashes/bf-mje3pd-crash-artifacts-analysis-domchk-a4cc1326-2026-09-07.md` | 9b83c81 |
| Pattern / FP analysis | domchk-7dc8f0d6 | `docs/crashes/bf-mje3pd-pattern-analysis-domchk-7dc8f0d6-2026-09-07.md` | 395cf89 |

Every load-bearing figure below was re-verified first-hand by this dispatch from the
raw artifacts (§8) — nothing is carried on faith, including from the child legs.

---

## 1. Root cause statement

**PRIMARY CAUSE: INFRASTRUCTURE — the workspace's repository-bloat regime.**

**Mechanism:** repeated death-by-signal terminations of the agent worker
(needle `exit_code: −1`, needle's sentinel for death by signal — not a signal
number) during git-heavy work on a bead implementing bloat prevention *inside the
bloated repository itself* — the documented 2026-08-13 state of ~18 GB `.git` /
~17 GB loose objects. The kill cadence (7 kills in 43m22s at 3–6 min intervals,
instant 1.9–17.6 s re-claims) inside the memcg-constrained dispatch scope, the
workspace-scoped blast pattern, and the day's kill table topped by the 17 GB-cleanup
bead together match the repository-bloat memcg-OOM regime that the same era's
kernel-proven events (bf-198ne, bf-1ea4g — SIGKILL, memcg) establish by direct
record. **For this bead the mechanism is regime-matched, not kernel-proven:** the
Aug-13 kernel/journald records were destroyed by the Aug-14/15 reboot (single
surviving boot starts 2026-08-15 19:56:33 EDT — re-verified live this dispatch), so
no record survives to name the signal for *this* bead.

**Confidence:**

| Claim | Confidence | Basis |
|---|---|---|
| Category = INFRASTRUCTURE | **HIGH** | Five independent first-hand lines converge (§3) with zero contradicting evidence; five independent dispatches reached the same verdict (§7) |
| Mechanism = memcg-OOM-class kills in the bloat regime | **MEDIUM** | Regime-matched, not kernel-proven: kernel records for this bead are destroyed; the match rests on cadence, scoping, task type, and kernel-proven same-era analogues |
| Not a code defect | **HIGH** | App code not in the death path (§5); corpus-wide zero-defect record |
| Not a false positive | **HIGH** (that the crashes were real) | Mid-task at the input instant; heartbeat provenance (§2) |

**Disposition of the alerts (separate question from the cause):** the crashes were
genuine and mid-task, so the alerts were **correct when raised**; bf-mje3pd closed
2026-08-17 with its deliverable landed and verified the same evening as the loop, so
the remaining alert beads are **stale, not false** — they warrant no investigation.
Re-read live this dispatch: **bf-56kmlk, bf-1pidqn, bf-3dxljn, bf-x88dnf Open**
(four — bf-1cezsk has retired via its own domchk-40cd5fde leg since 395cf89 read
five); bf-1y1d0g, bf-3za7vh, bf-1cezsk Closed.

## 2. The input instant is a heartbeat 12.9 s after the kill it names

Re-verified first-hand from the raw log this dispatch: attempt 4's bracket is
`agent.completed exit_code −1` at **19:18:43.227305346Z** → `outcome.classified
crash` at **19:18:43.233797898Z** → `HANDLING_RELEASE_DONE` heartbeat at
**19:18:56.178350930Z**. The task's named instant `19:18:56.178560656` matches that
heartbeat to ~0.2 ms and sits **12.945 s after** the kill — it is the `Timestamp:`
field of alert bead **bf-56kmlk** (one alert bead per kill under pre-0.4.2 needle).
Consequence: any analysis anchored on the named instant is anchored on a release
heartbeat, and the guide's "completed <30 s before the timestamp → FALSE_POSITIVE"
branch reads a clock that never was the death instant.

## 3. Why the category is INFRASTRUCTURE — five converging lines

1. **Exit signature.** 12 classified outcomes: **7 × (−1, crash)**, 3 × (1,
   failure), 1 × (124, timeout), 1 × (0, success) — re-derived first-hand this
   dispatch. Exit −1 is infrastructure-class by the guide's decision tree and by
   construction (death by signal, not an application exit). The three exit-1
   failures are secondary churn inside the same loop, not the loop's defining mode.
2. **Mid-task at the instant.** At the heartbeat the bead was 4 attempts into a
   14-attempt loop with 9 dispatches ahead of it; terminal success came at
   21:18:23Z and the bead stayed Open until 2026-08-17.
3. **Workspace-scoped, not host-wide.** ±1 h window (18:18:56–20:18:56Z) census
   across all six Aug-13 worker logs, re-computed first-hand: **lab-domain-check
   10 × exit −1, all five peers (drawrace, roam-1, roam-2, s1, test-fix) zero**
   in-window and ≤2 on their whole day. Day-level: 344 kills / 13 beads, all on one
   worker. A host-wide OOM or a service outage does not localize to one workspace's
   resource footprint; repo-local bloat does.
4. **Regime match with kernel-proven analogues.** This workspace was in the
   documented ~18 GB / ~17 GB-loose bloat state (bf-1s6c3, bf-4yjq, verified
   cleanup 18 GB → 92 MB on 2026-09-01). The same era's kernel-proven kills
   (bf-198ne, bf-1ea4g) are memcg-OOM **SIGKILL** of pack-objects during git
   operations. bf-mje3pd was doing exactly that class of work, dying at exactly
   that cadence.
5. **The regime's own tell.** The day's top victim is bf-65lsdu — "Run repository
   cleanup to eliminate 17GB bloat", **127 kills**. Cleanup beads topping the kill
   table is what a bloat regime predicts and nothing else does.

## 4. What the three child legs each establish

- **Classification (domchk-bf8c4fd3, 63ca904):** INFRASTRUCTURE, repository-bloat
  sub-type; automated classifier unavailable (`crash-classifier.sh bf-mje3pd` →
  exit 2 "Bead trace not found" — single-slot traces plus the Sep-1 `.beads/logs/`
  era floor), so classification comes from the raw worker log; full 14-row attempt
  table; heartbeat provenance of the input instant; bead-state check (Closed rev 2).
- **Artifacts (domchk-f0513fd1 → domchk-a4cc1326, 9b83c81):** the artifact
  inventory and what is *missing* — agent transcript deleted by the `mend`
  zero-activity cleaner (three recorded deletions), kernel/journald records lost to
  the Aug-14/15 reboot, per-bead trace gone; the work-survival record (`ea23bd1`,
  `164b62d`); correction of the prior report's "9 × exit −1 / 13th attempt" figures.
- **Pattern / FP (domchk-7dc8f0d6, 395cf89):** not a false positive (Rule 1's
  14.7 s commit→kill hit on `164b62d` is a mid-loop checkpoint — per-attempt
  commits against a 3–12 min kill cadence land some pairs within seconds by
  construction); retry succeeded on attempt 14 but Rule 2's caveat applies — kills
  continued after the success (hourly −1 histogram 19/52/56 across hours 21–23) and
  the regime survived 19 days, so **transient at the bead level, infrastructure at
  the cause level**; both surge thresholds evaluated rather than assumed (window max
  2-in-5min and 3-in-10min — neither fires; the guide's low-and-slow
  environmental-regime corollary is what fires).

## 5. Ruled-out alternatives

| Alternative | Verdict | Deciding evidence (first-hand) |
|---|---|---|
| **Workflow failure** (max-turns exhaustion, bead-closing issue) | Ruled out | 0 `max_turns` occurrences in the bead's 281 log events (re-checked this dispatch); the exit-1 failures are generic agent failures, not `error_max_turns`; the bead *did* close (rev 2, 2026-08-17) |
| **Service failure** (gateway 503/502) | Ruled out | **0 status-shaped** HTTP 503/502 records in the 281 events. A naive substring grep "finds" seven `503`s — all are nanosecond-timestamp fragments (e.g. `.351085030Z`, `.503284830Z`), the documented census trap; context-aware matching finds none |
| **SIGHUP cascade** | Ruled out | 0 `sighup` occurrences in the bead's events (and 0 in the whole Aug-13 worker log); exit −1 is a sentinel, and the era's kernel-proven kills are SIGKILL memcg-OOM, not SIGHUP — the corpus doc claiming exit −1 *is* SIGHUP is superseded |
| **Code defect** (domain-check) | Ruled out | The deaths are signal deaths of the *agent worker*, not application exits; bf-mje3pd's deliverable is repo-maintenance scripts, not domain-check code; no investigation in this workspace has ever found a domain-check code defect |
| **Disk/CPU resource exhaustion** | No supporting evidence | No record constrains disk or CPU as the killer; memory is the only resource with both a kernel-proven same-era mechanism and a regime match |
| **FALSE_POSITIVE** (crash not real) | Ruled out | Input instant is a heartbeat 12.9 s *after* a real classified kill; work was 4 attempts in; the 30 s heuristic's literal hit on `164b62d` is a mid-task checkpoint, not post-completion cleanup (Rule 1's premise broken on four first-hand facts) |
| **Self-healed transient / isolated event** | Ruled out as *cause* | Crash → retry → success is real (attempt 14, zero work lost), but kills continued after the success and the regime survived until 2026-09-01 — transient describes only the bead's completion |

## 6. Similar past crashes (the pattern this event belongs to)

Same regime, same workspace, same mechanism class:

- **bf-1s6c3 / bf-4yjq** (2026-08-12) — the regime's founding events: 17+
  identical 237 MB `.beads/*.jsonl` snapshots committed → ~18 GB repo → 76
  dispatches / 71 memcg-OOM kills (bf-1s6c3) + 50 (bf-4yjq) in one evening.
- **bf-4k2ws** (2026-08-13) — 55 kills the same day (kernel-era census, corrected
  9ae17f2); **bf-65lsdu** (2026-08-13) — 127 kills, the cleanup bead itself.
- **bf-1ea4g** (2026-08-13) — 56 kills; kernel-proven push-side variant
  (unbounded `git push` pack-objects over a 422-commit backlog).
- **bf-198ne** (2026-08-16) — kernel-proven memcg-OOM SIGKILL, the push-side
  variant; the record that pins SIGKILL (not SIGHUP) as this era's signal.
- **bf-173o7e / bf-31mno** (2026-08-14) — the same-day storm's largest loops
  (131 attempts; 434 kills, no RCA owed — regime-class, mechanism shared).

**The pattern is closed, not ongoing** — re-verified live this dispatch:
`.git` = **106 MB** (345 loose objects / 2.34 MiB, 99.78 MiB pack, 0 garbage),
`check-repo-health.sh` exit 0, `crash-pattern-detection.sh` → "No crashes detected
in the last 24 hours — System Status: STABLE". Remediation that ended the regime:
`.beads/` fully gitignored (0 tracked files), 10 MB pre-commit gate,
`pack.windowMemory`/`deltaCacheSize`/`threads=1` bounds covering gc *and* push,
daily/weekly bounded-gc timers, verified 18 GB → 92 MB cleanup on 2026-09-01
(`docs/crashes/bf-4yjq-cleanup-verification.md`).

## 7. Agreement with sibling records

Five independent dispatches have now reached the same verdict on this bead; this
determination agrees with all of them and folds in the one leg the earliest RCA
could not (the pattern leg landed after it):

| Record | Verdict |
|---|---|
| domchk-bf8c4fd3 (63ca904) — this chain's classification leg | INFRASTRUCTURE, bloat regime |
| domchk-a4cc1326 (9b83c81) — artifacts | regime-matched, not kernel-proven |
| domchk-1b407bff (7672f9e) — parallel chain's RCA | INFRASTRUCTURE, bloat regime, regime-matched |
| domchk-a47705c9 (e7f5b97) — split step 2 of bf-3dxljn | INFRASTRUCTURE, regime-matched |
| domchk-7dc8f0d6 (395cf89) — this chain's pattern leg | INFRASTRUCTURE EVENT, workspace-scoped, stale-not-false |

What this determination adds beyond the siblings: the synthesis of all three of this
chain's legs in one record, the explicit confidence decomposition (HIGH on category,
MEDIUM on mechanism — the honest split the "regime-matched, not kernel-proven"
hedge implies), and the context-aware 503/502 check that closes the service-failure
branch with evidence rather than assumption.

## 8. Verification record (domchk-7a3b46fa, this dispatch, 2026-09-07 ~21:5xZ)

First-hand re-executions; agreement with the child legs found everywhere:

- **Raw log census** (`claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl`,
  JSON-parsed on `event_type`/`data.exit_code`): 281 bf-mje3pd events; 14
  dispatched / 13 completed / 12 classified = 7 × (−1, crash), 3 × (1, failure),
  1 × (124, timeout), 1 × (0, success). Attempt-4 bracket and success tail match
  §2 and the 21:18:23.229 → 21:18:23.232 → 21:18:23.247 (`verification.passed`,
  gates_run 1) → 21:18:36.324 (`bead.orphaned`) sequence.
- **Workspace scoping census** recomputed over all six Aug-13 worker logs:
  domain-check 10 × −1 in-window / 344 day; drawrace 0/2, roam-1 0/2, roam-2 0/2,
  s1 0/1, test-fix 0/0.
- **Marker greps** on the bead's 281 events: `sighup` 0, `max_turns` 0, status-
  shaped `503`/`502` 0 (naive substring hits = 7, all nanosecond-timestamp
  fragments — shown and dismissed in §5).
- **`journalctl --list-boots`**: single boot, first entry Sat 2026-08-15 19:56:33
  EDT — no Aug-13 kernel record survives.
- **Git archaeology**: `ea23bd1` (19:02:32Z, "implement OOM crash prevention and
  repository health monitoring") and `164b62d` (19:21:41Z, "add preventive scripts
  for repository bloat and OOM crash prevention") — **not** reachable from `main`,
  reachable only from `pre-squash-history-20260816`; their content classes
  (`scripts/check-repo-health.sh`, `scripts/cleanup-repo-bloat.sh`,
  `scripts/pre-commit-repo-size-hook`) all present in HEAD's tree.
- **Live bead states**: bf-mje3pd Closed rev 2; alerts Open = bf-56kmlk, bf-1pidqn,
  bf-3dxljn, bf-x88dnf; bf-1cezsk / bf-1y1d0g / bf-3za7vh Closed; neighbours
  bf-29h1yy, bf-2o7nlw, bf-65lsdu all Closed.
- **Current repo health**: `.git` 106 MB; `git count-objects -vH` 345 loose /
  2.34 MiB, 99.78 MiB pack, 0 garbage; `check-repo-health.sh` exit 0;
  `crash-pattern-detection.sh` → no crashes in 24 h, STABLE.
