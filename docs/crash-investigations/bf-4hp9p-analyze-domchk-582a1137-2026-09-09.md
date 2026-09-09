# Analyze: crash root cause for bead bf-4hp9p (domchk-582a1137)

**Leg:** domchk-582a1137 — analyze child of alert bead domchk-6ce337d0 (auto-split 2026-09-01T16:25Z)
**Subject:** the five 2026-08-16 kills of the workers *processing* alert bead **bf-4hp9p** (kill #1 at
14:31:52Z is the instant this leg's template carries)
**Scope:** analysis only — no fix, no new prevention work. Root-cause mechanism is subordinate to the
canon RCA (`docs/investigations/root-cause-determination-domchk-6281555d-2026-09-06.md`, §storm table
line 103 records this very storm). Inputs: the gather leg's fact-finding summary
(`docs/crash-investigations/bf-4hp9p-gather-domchk-fb213816-2026-09-09.md`, commit 04a21bc) plus the
primary sources re-read first-hand this dispatch.
**Date:** 2026-09-09. Every figure below was re-verified live this dispatch.

## 1. Verdict (one paragraph)

The signal −1 exits were **kernel memory-cgroup OOM kills, not a bug and not a service failure**. At
each of the five instants the kernel recorded `oom-kill:constraint=CONSTRAINT_MEMCG … task=git`
against a `git` process holding 7.4–12.3 GiB anon-rss inside a 12 GiB (`12,582,912 kB`) `systemd-run`
dispatch scope; the uncatchable SIGKILL took down the whole agent process tree, and needle recorded
`exit_code=-1 outcome=Crash(-1)` and minted a fresh alert bead for each death. The resource that ran
out was never the host's memory — it was the **cgroup's** — and the working set that blew through it
was git operating on this repository during the bloat era (the ~17 GB of loose `.beads/*.jsonl`
objects committed 2026-08-12, not repaired until 2026-09-01). The Aug-25 investigation doc
(79d3649) attributed the death to "extreme CPU saturation (4.46× load)"; that is **superseded** — the
kernel names memory as the kill constraint on every line, and CPU load never SIGKILLs anything.

## 2. Immediate cause of the signal −1 exit

`exit code -1` is needle's sentinel for "the process died without reporting an exit status" — the
signature of SIGKILL, which cannot be caught or reported by the dying process. The kill records:

| # | Outcome (UTC, needle log) | Kernel OOM (EDT = UTC−4) | Victim | anon-rss | Scope (12 GiB bound) |
|---|---|---|---|---|---|
| 1 | 14:31:52.844 → alert domchk-6ce337d0 | 10:31:52 | git pid 1477169 | **12,287,564 kB ≈ 11.72 GiB** | run-p1444226-i211132581.scope |
| 2 | 14:35:30.927 → alert domchk-e4b9af74 | 10:35:30 | git ×2 (1500032 / 1499296) | 12,241,908 / 12,312,048 kB | run-p1468410 / run-p1477493 |
| 3 | 14:37:35.979 → alert domchk-4817f168 | 10:37:35 | git pid 1524158 | 12,333,836 kB | run-p1505045 |
| 4 | 14:40:33.334 → alert domchk-4440149b | 10:40:33 | git pid 1549720 | 7,420,992 kB | run-p1525600 |
| 5 | 14:48:23.023 → alert domchk-ccd3421d | 10:48:22–23 | git ×2 (1618140 / 1619298) | 7,641,532 / 4,691,164 kB | run-p1604120 |

Every line reads `constraint=CONSTRAINT_MEMCG` — the kernel's own statement that the memory cgroup
(the dispatch scope), not the host, hit its limit (`try_charge_memcg` failed at usage ≈ the
12,582,912 kB bound). Kills #2 and #5 each show a *second* git pid dying the same second: the wave
killed git processes in neighboring scopes too (10:30:54 run-p1446290, 10:33:42 needle.slice
run-p1476877, 10:41:12, 10:43:38, 10:44:36, 10:46:20 …), so the bf-4hp9p five are five members of a
sustained local storm, not five isolated faults.

**Attribution confidence.** Kernel OOM lines record no cwd and no bead, so the per-second match
between kernel stamps and needle's `handling agent outcome … outcome=Crash(-1)` lines (0.9 s delta on
kill #1: kernel 10:31:52 → outcome 14:31:52.844Z) is the attribution — high confidence, identical
signature to the neighboring kills, but not absolute per-pid proof. This is the same epistemic status
the canon assigns the whole era, and the journald boot (begins 2026-08-15) covers all five instants,
so unlike the Aug-13/14 storms these five are **kernel-proven**, not mechanism-inferred.

## 3. Classification: resource issue — OOM, and specifically *cgroup* OOM

Against the three candidate classes:

- **Resource issue — YES.** Memory-cgroup exhaustion, per the kernel lines above. Not "CPU
  saturation": the load-1min figures of 17–23 that 79d3649 leaned on are real (needle rate-limit
  warnings in the same window) but co-symptomatic — fleet-wide concurrent git thrash over bloated
  stores produced both the load *and* the memory deaths. The kernel OOM killer selects victims on
  memory pressure alone (`badness()` scoring); no CPU condition SIGKILLs a process, so "4.46× load"
  cannot be the kill mechanism whatever it correlates with.
- **Bug — NO.** No Go panic, no stack trace, no application error in any record of this chain; the
  killed process is `git`, not any domain-check binary. The canon's 157+ investigations of this
  workspace have found zero domain-check code defects, and nothing in this chain disturbs that.
- **External service failure — NO.** No HTTP 503/502, no gateway outage, no inference-layer
  signature anywhere in the window; the deaths are local kernel enforcement.

The refinement worth stating: this is OOM **under a scope cap**, not host OOM. The host had ample
RAM; the 12 GiB `systemd-run` scope around each dispatch was the wall. That is why the same operation
class is benign today — the repo is repaired (105 MB) *and* every git path is memory-bounded, so the
working set no longer approaches the cap (see §7).

## 4. Root cause hypothesis and supporting evidence

**Hypothesis (HIGH confidence on the mechanism class; kernel-proven for these five kills):**

> The agent workers processing alert bead bf-4hp9p were killed because the git operations their
> dispatch scopes ran against this repository's bloat-era object store (~17 GB loose objects,
> Aug-12 → Sep-01) exceeded the scopes' 12 GiB memory bound, and the kernel's memory-cgroup OOM
> killer SIGKILLed them. Because each death minted a new alert bead and each retry re-entered the
> same bloated repository, the crash *reproduced its own investigation queue* — five processor kills
> in twenty minutes, each one an investigator killed by the very crash class it was dispatched to
> investigate.

Supporting evidence, each item re-verified this dispatch:

1. **Kernel constraint lines** at all five seconds: `CONSTRAINT_MEMCG`, `task=git`, anon-rss
   4.7–12.3 GiB against a 12,582,912 kB scope bound (§2 table).
2. **Per-second outcome correlation**: five `exit_code=-1 outcome=Crash(-1)` lines in
   `~/.needle/logs/needle-claude-code-glm-4_7-lab-drawrace.log` (session 70478ac5, cross-workspace
   pluck of bf-4hp9p from `/home/coding/domain-check`), each followed ~0.1 s later by
   `crash alert bead created bead_id=bf-4hp9p alert_id=…` naming the exact five alert beads.
3. **The repo was in its bloat era on 2026-08-16** — the ~18 GB loose-object state created by the
   2026-08-12 `.beads/*.jsonl` commits stood until the 2026-09-01 packing; bf-1s6c3, the crash
   bf-4hp9p was alerting about, is itself a canon member of that same class (76 dispatches / 71 kills
   on Aug-12, `docs/crash-analysis-bf-1s6c3-2026-09-06.md`).
4. **Kill #1's attempt committed only dispatch bookkeeping** — b74a5ac, `.needle-predispatch-sha`
   (1 insertion, 1 deletion), 14:31:44Z, **8 s before the kill** — i.e. death mid-attempt, during
   needle's own git reconciliation of this repo, not during any long analysis.
5. **Fleet-scale corroboration**: 414 `CONSTRAINT_MEMCG` kernel kills on 2026-08-16
   (00:27:35–13:40:32 EDT, matching the canon's storm window 04:27:35Z–17:40:32Z end to end), 257 of
   them `git` victims — and the same hour histogram peaks the canon's two-wave reading describes.
6. **The class disappeared with the cause**: post-repair (Sep-01) the live fleet signature shows
   exit −1 near-zero (confined to synthetic test/gc scopes), and this repo's health re-verified clean
   this dispatch (§7). A mechanism that vanishes when its resource cause is removed is the strongest
   available confirmation of the cause.

**What the killed agent was working on (two layers, both verified):** the worker was processing
ALERT bead bf-4hp9p — i.e. investigating the bf-1s6c3 kill of 2026-08-12T23:31:51Z, which is the
subject named inside bf-4hp9p's alert text. The template's 14:31:52 timestamp belongs to the
*processor* kill generation, not to that subject; three timestamps must not be conflated
(gather leg §2 has the full reconciliation table).

## 5. Similar crash patterns in the workspace

Same mechanism class (`CONSTRAINT_MEMCG`, git, bounded scope, bloat-era store), all canon members:

| Family | Date | Shape | Relation to this chain |
|---|---|---|---|
| **bf-1s6c3 / bf-4yjq** | 2026-08-12 | 18 GB repo; 76 dispatches / 71 kills + 50 kills | **The subject of the alert bf-4hp9p** — this chain is generation 2 of that crash |
| bf-65lsdu | 2026-08-13/14 | 163 exit −1 kills / 163 alert beads over the 17 GB cleanup work | Same alert-multiplication shape, larger |
| bf-1ea4g | 2026-08-13 | Unbounded `git push` pack-objects over commit backlog | Same bound-exceeded-by-git shape, push side |
| bf-4x12ec | 2026-08-14 | Bare `git gc --aggressive --prune=now`, 44 identical kills | Same class, maintenance-command variant |
| bf-173o7e | 2026-08-14 | 132 dispatches / 129 memcg-OOM kills | Same class, own storm |
| bf-198ne | 2026-08-16 | Push-side memcg OOM, 720-commit backlog | **Same day as this chain's kills** |
| bf-3riuu | 2026-08-16 10:30:54 | git 6.99 GB killed one minute before kill #1 | Same wave, neighboring bead |

**The Aug-16 storm in fine structure** (fresh journalctl census this dispatch): 414
`CONSTRAINT_MEMCG` kills spread 00:27:35–13:40:32 EDT, in two main masses (00–03 EDT ≈ 65 kills,
08–13 EDT ≈ 332) with a minor 06 EDT tail (17). Hour peaks: 12:00 EDT 79, 09:00 71, 10:00 61 —
bf-4hp9p's five kills sit inside the 10:00 hour. Victim breakdown: **257 git, 156 node (vitest
workers, 4.5–8.6 GiB anon-rss), 1 other** — the wave saturated many scopes fleet-wide, and git
dominated the victim list because bloat-era stores gave git the largest working sets.

**Census floor:** 409 `outcome=Crash(-1)` events across **102 distinct beads** on 2026-08-16 (fresh
grep of retained `~/.needle/logs`). This is a floor — retention gaps — and it supersedes 79d3649's
"826 crashes" (unverified method); the canon's 461 outcome figure uses its own counting window. Use
the floor, labelled as such.

**The pattern-level finding for the record:** this chain is the alert-loop shape —
crash → alert bead → investigator dispatched into the same bloated repo → same kill → alert bead →
… — which is why a single Aug-12 crash produced a gen-1 alert (bf-4hp9p), five gen-2 alerts
(domchk-6ce337d0 + four siblings, all now Closed), and a gen-3 six-leg split (2026-09-01). The
mechanism was ended by removing the resource cause (repo repair) and by the alert-layer gates
(dedup, cooldown, surge detection, target-resolution check), not by fixing anything in the
investigation path itself.

## 6. Refinements this leg adds to the gather record

The gather leg conservatively recorded two claim ends as "unresolved" because no `Crash(-1)` line
followed them. The retained log does resolve both, and the resolution matters for the work-loss
assessment:

| Claim start | Outcome (verified this dispatch) | Interpretation |
|---|---|---|
| #5 — 14:40:33.864Z | `exit_code=1 outcome=Failure` at 14:46:02.638Z | A non-crash failure (workflow-class, e.g. max-turns), so **no alert was minted** — correct behavior: alerts fire only on `Crash(-1)` |
| #7 — 14:48:25.256Z | `exit_code=0 outcome=Success` at **14:52:48.878Z** | **The terminal success** — 4 m 26 s after the last kill |

Full ledger: **7 claims → 5 Crash(−1) + 1 Failure(exit 1) + 1 Success(exit 0).** This also sharpens
79d3649's "crashed immediately after completing its investigation": the substantive deliverable
(e210c84, the 119-line bf-1s6c3 investigation doc, 14:40:07Z) landed during kill #4's attempt, 26 s
before that kill — and the *chain* closed out via the 7th attempt's success at 14:52:48Z (bf-4hp9p
itself reached Closed rev 21 on 2026-08-25). Work loss: none — the deliverable predates the deaths
that followed it, and every killed attempt's partial output was superseded, not orphaned.

## 7. Prevention state (existing layers; no new work proposed here)

Analysis-only leg — remediation is canon-owned (`docs/crash-prevention-requirements.md` G-1..G-13)
and already in force. Per the prevention-validation rule, the layers' claims are only as current as
their last validation (`docs/crash-prevention-validation.md`); the smallest live re-check was run
this dispatch:

- `./scripts/check-repo-health.sh` → **exit 0**; `.git` 105 MB, 61 loose objects, 1 pack
  (100.70 MiB), 0 garbage — the bloat-era precondition cannot recur through `.beads/` (gitignored,
  0 tracked files).
- The remaining layers (pack-memory bounds on bare gc/push, safe-git-gc.sh, 10 MB pre-commit gate,
  timers, storm breaker, dispatch limiter, alert dedup/cooldown/surge gates) are validated on the
  cadence recorded in `docs/crash-prevention-validation.md` — most recently all-green 2026-09-08
  (domchk-82c1ff9a), with dated re-runs by the bf-173o7e implement leg on 2026-09-09 (39da580).

Why the kills stopped, in one line each: the repo repair removed the giant working set; the
`pack.windowMemory=2g`/`threads=1` config bounds the git paths that remain; the alert gates stopped
one kill from minting an unbounded investigation queue.

## 8. Family status (live `bead show`, this dispatch 2026-09-09)

| Bead | Status | Note |
|---|---|---|
| bf-1s6c3 (crash the alert names) | **Closed** rev 5 | work complete; canon analysis 2026-09-06 |
| bf-4hp9p (gen-1 alert) | **Closed** rev 21 | 2026-08-25 |
| domchk-e4b9af74 / 4817f168 / 4440149b / ccd3421d | **Closed** rev 10 / 8 / 12 / 8 | gen-2 sibling alerts, one per processor kill |
| **domchk-6ce337d0** | **Open** rev 16 | gen-2 umbrella, owns the 2026-09-01 split — not this leg's bead |
| domchk-fb213816 | **Closed** rev 4 | gather leg (04a21bc) |
| **domchk-582a1137** | InProgress | **this analyze leg** |
| domchk-efd2efda / 24615bf5 / f961d4c4 / fb86a21e | Open | sibling legs: review / verify-fix / doc / verify-findings |

## 9. Sources (all read this dispatch)

- Gather leg deliverable: `docs/crash-investigations/bf-4hp9p-gather-domchk-fb213816-2026-09-09.md`
  (04a21bc) — premise corrections table and three-timestamp reconciliation
- `journalctl --since "2026-08-16 00:00" --until "2026-08-17 00:00"` — 414 `CONSTRAINT_MEMCG` lines;
  the five per-second kill rows; victim breakdown (257 git / 156 node / 1 other); hour histogram
- `~/.needle/logs/needle-claude-code-glm-4_7-lab-drawrace.log` lines 8164–8350 — all 7 claims and
  their outcomes, including the two the gather leg left unresolved (`Failure` exit 1 at
  14:46:02.638Z; `Success` exit 0 at 14:52:48.878Z)
- `bead show` bf-4hp9p, bf-1s6c3, domchk-6ce337d0, domchk-e4b9af74, domchk-4817f168,
  domchk-4440149b, domchk-ccd3421d, and the six split legs
- `git show` b74a5ac (bookkeeping-only, 14:31:44Z) and e210c84 (deliverable, 14:40:07Z)
- Canon RCA line 103 (Aug-16 storm row) — independently re-derived this dispatch and matching
- Fresh census: `grep -h "outcome=Crash(-1)" ~/.needle/logs/*.log | grep -c ^2026-08-16` → 409;
  distinct bead_ids → 102
- Live health: `./scripts/check-repo-health.sh` exit 0; `git count-objects -vH`; `du -sh .git`

---
**Leg disposition:** docs-only analysis (this file), subordinate to the canon RCA with no new cause
claim; summary also recorded on this leg bead's own notes. The umbrella domchk-6ce337d0 and the four
remaining sibling legs are separate beads — this leg neither closes nor re-investigates them.
