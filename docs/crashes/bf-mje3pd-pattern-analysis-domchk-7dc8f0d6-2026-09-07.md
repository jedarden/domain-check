# bf-mje3pd — Crash Pattern Analysis and False-Positive Detection

**Analysis bead:** domchk-7dc8f0d6 (2026-09-07)
**Target bead:** bf-mje3pd — "Implement fix and verify agent crash prevention"
**Input instant:** 2026-08-13T19:18:56.178560656Z, exit code −1
**Collection dependency:** domchk-f0513fd1 (Closed rev 4) supplied the artifact inventory.
**Sibling legs folded in:** domchk-a4cc1326 (artifact analysis, `9b83c81`),
domchk-bf8c4fd3 (classification, `63ca904`), domchk-916a1e66 (evidence compilation,
`bfef974`), domchk-a6059a67 (bf-4yjq chain report, `f5e6377`),
domchk-1b407bff (RCA, `7672f9e`). Every number below was re-derived first-hand from
the raw artifacts in this dispatch; agreement and disagreements are noted where relevant.

## Answer

**Part of a larger infrastructure event — not a false positive, not an isolated crash.**
The bead's own outcome was a same-evening recovery on retry (crash → retry → success,
zero work lost), but the guide's Rule 2 caveat applies: the *cause* outlasted the retry
loop by 19 days, so the storm is **Infrastructure** (workspace-scoped repository-bloat
regime), not a self-healed transient. Mapping to the task's three-way taxonomy:
TRANSIENT describes only the bead's completion; the pattern classification is
INFRASTRUCTURE EVENT (workspace-scoped), with every open alert on this target now stale.

This leg adds four things the sibling records do not contain: a ±1 h frequency census
with the surge-detector thresholds actually evaluated (§3), the day-level surge peak
localized to 22:17Z (§4), the demonstration that the guide's 30-second false-positive
heuristic **does** fire naively on one of this bead's commits and why that verdict is
wrong (§1), and the post-success kill continuation that settles the Rule 2 caveat
first-hand (§2).

## 0. Provenance of the input instant (carried forward, re-verified)

`19:18:56.178560656Z` is **not a kill instant**. The raw log has attempt 4's
`HANDLING_RELEASE_DONE` heartbeat at `19:18:56.178350930Z` — the input matches it to
~0.2 ms — while attempt 4's actual death is `agent.completed exit_code: -1` at
`19:18:43.227Z` / `outcome.classified crash` at `19:18:43.233798Z`: the input instant
is **12.945 s after** the kill it names. It is the `Timestamp:` field of alert bead
bf-56kmlk (one alert per kill under pre-0.4.2 needle). Frequency analysis anchored on
this instant is therefore anchored on a release heartbeat, which is what the ±1 h
window in §3 uses.

## 1. Time between crash and last work commit

Two of the loop's killed attempts committed mid-task, seconds before their kills
(both reachable only from `pre-squash-history-20260816`; content live at HEAD):

| Commit | Committed (UTC) | Content | Nearest kill | Gap |
|---|---|---|---|---|
| `ea23bd1` | 19:02:32 | `.gitignore` + `check-repo-health.sh` (84 lines) — *bloat detection* | attempt 1, 19:03:11.253Z | **39.3 s before** |
| `164b62d` | 19:21:41 | `check-repo-health.sh` rewrite, `cleanup-repo-bloat.sh`, `pre-commit-repo-size-hook`, `.gitignore` — *bloat prevention* | attempt 5, 19:21:55.686Z | **14.7 s before** |

Against the task's input instant itself (19:18:56.179Z), the last commit *before* it is
`ea23bd1` — **16.4 minutes** earlier; `164b62d` lands 2.75 minutes *after* it.

**The 30-second heuristic fires naively and is wrong here.** Guide Rule 1
(`docs/crash-response-guide.md` HEAD lines 249, 589–595) reads "commit within 30 s
before crash → FALSE POSITIVE", and `164b62d` → attempt 5 at 14.7 s satisfies it
literally. Rule 1's premise is *post-completion cleanup* — the commit was the final
deliverable and the crash was cleanup noise. Four first-hand facts break that premise:

1. **The bead was not done.** At the input instant it was 4 attempts into a
   14-attempt loop with 9 dispatches still ahead of it; the terminal success came at
   21:18:23Z, and the bead stayed Open until 2026-08-17 (closed rev 2).
2. **The commits were checkpoints, not completion.** Both are mid-loop increments of
   the same deliverable — `164b62d` *extends* `ea23bd1`'s `check-repo-health.sh`
   rather than finishing it.
3. **The same bead kept working after both kills** — needle re-claimed in 1.9–17.6 s
   and the loop ran on; a post-completion FP has no such tail.
4. **The guide's own storm caveat (bf-1s6c3, HEAD lines 590–596)** already says to
   ask where the *deliverable* landed rather than to trust the last-commit/last-crash
   pairing. Here the deliverable is present at HEAD (the three scripts exist and are
   the maintained versions), so the correct reading is "work landed mid-storm, loop
   kept spending attempts", not false positive.

A mechanical note on *why* the heuristic fires at all: with 7 kills at a 3–12 min
cadence against a bead committing per-attempt, some commit/kill pairs land within
seconds by construction. Proximity alone is not evidence of completion — bead state
at the instant is, and it says mid-task.

## 2. Retry pattern (did this bead succeed on retry?)

**Yes — verified from the raw log, not from the bead record.** 14 `agent.dispatched`
/ 13 `agent.completed` / 12 `outcome.classified` (7 × exit −1, 3 × exit 1, 1 × 124,
1 × 0), with the tail first-hand:

```
21:10:14.126  outcome.classified  exit 124  timeout        (attempt 13)
21:10:33.445  agent.dispatched                            (attempt 14)
21:18:23.229  agent.completed    exit 0    (470 s run)
21:18:23.232  outcome.classified exit 0    success
21:18:23.247  verification.passed  gates_run: 1
21:18:36.324  bead.orphaned  /  outcome.handled action=none
```

Crash → retry → success is Rule 2's surface shape. **The caveat decides it:**
(`docs/crash-response-guide.md` HEAD lines 612–617 — "an exit-0 terminal attempt
after a kill storm is only a *surface* match … confirm the environment actually
changed"). The environment had **not** changed:

- Kills on this worker **continued after** bf-mje3pd's success — the same day's
  hourly exit −1 histogram runs 19 (21:00 h) → 52 (22:00 h) → 56 (23:00 h). The
  regime was still killing while this bead exited 0.
- The bloat regime itself persisted until the verified cleanup packed ~18 GB → 92 MB
  on 2026-09-01 — 19 days *after* the loop's success
  (`docs/crashes/bf-4yjq-cleanup-verification.md`).
- What did change for this bead: attempt 14 ran under the restarted worker session
  (`3bcc4996`, after the worker died with attempt 12) and finished in 470 s — the
  remaining work fit inside the dispatch scope, it was not operating in a healed
  environment.

So: **transient at the bead level** (work recovered same evening, zero loss — both
pre-kill commits' content live at HEAD), **infrastructure at the cause level**. The
guide's caveat names this exact misread and bf-mje3pd is a clean instance of it.

## 3. Crash frequency in the ±1 h window (18:18:56 – 20:18:56Z, 2026-08-13)

Census of `outcome.classified exit_code: -1` across all six Aug-13 worker logs
(JSON-parsed on `event_type`; grep overcounts):

| Worker | −1 in window | −1 whole day | Distinct crash beads (day) |
|---|---|---|---|
| **lab-domain-check** | **10** | **344** | **13** |
| drawrace | 0 | 2 | 2 |
| roam-1 | 0 | 2 | 2 |
| roam-2 | 0 | 2 | 2 |
| s1 | 0 | 1 | 1 |
| test-fix | 0 | 0 | 0 |

- **100 % of the window's crash burden is one worker.** Five peers running the same
  box in the same window logged zero exit −1; the event is **workspace-scoped, not
  host-wide** — the signature of repo-local bloat rather than a system-wide OOM.
- **Per bead inside the window:** bf-mje3pd 7, bf-29h1yy 2 ("Assess bf-4yjq
  completion state…"), bf-2o7nlw 1 ("Investigate crash context and gather logs for
  signal −1") — i.e. the window's crashes are bf-mje3pd **and its own bloat-response
  chain neighbours**, all working the same problem in the same bloated repo.
- **Surge-detector thresholds, evaluated rather than assumed:**
  - Current committed detector (`scripts/crash-pattern-detection.sh`,
    `CRASH_SURGE_THRESHOLD=3` in 5 min): window maximum is **2** (18:48:41–18:53:39)
    → **does not fire** for this window.
  - Older 10-in-10-minutes rule (quoted in this task's framework): window maximum is
    **3** (19:15:17–19:21:55, attempt 3 → 5) → **does not fire** either.
- **What does fire is the guide's low-and-slow corollary** (HEAD, bf-4yjq
  corollaries): "a sustained low-and-slow cadence is still an environmental regime…
  fixed-cadence exit −1 re-dispatch deaths across multiple beads means triage repo
  size / memory / load at the workspace level before any per-bead debugging." That is
  precisely this window's shape: kills every 3–12 min across 43 m 22 s, 1.9–17.6 s
  re-claims, three beads, one workspace.

A naive pass over this task's framework ("10+ crashes in 10 min = system-wide event,
otherwise isolated") would read the non-firing threshold as *isolated event*. The
threshold is a floor for *surge*-class events, not a test for regime-class ones — the
guide carries an explicit second path for exactly this cadence, and §4 shows the same
day *did* cross the surge threshold two hours later.

## 4. Part of a larger infrastructure event? — Yes

Same worker, same day (all first-hand from the one log):

- **344 classified exit −1 kills across 13 distinct beads** (00:00:15Z → 23:59:58Z):
  bf-65lsdu 127, bf-1ea4g 56, bf-4k2ws 55, bf-2ildm 38, bf-1s6c3 22, bf-ncxbt 11,
  bf-2vtzg 9, **bf-mje3pd 7**, bf-6d3d6 6, bf-574w1 5, bf-3hivb 5, bf-29h1yy 2,
  bf-2o7nlw 1. bf-mje3pd's loop is **2 % of the day's kills** — a slice, not an event.
- **The day crossed both surge thresholds, elsewhere:** maximum **7 kills in 5 min**
  at 22:17:20–22:22:20Z (hour 22 alone holds 52) and 12 in 10 min at 22:13–22:23Z.
  2026-08-13 was a surge-class infrastructure event on this worker; bf-mje3pd's
  window sat in its long low-and-slow approach.
- **The day's largest victim is the regime's own tell:** bf-65lsdu — "Run repository
  cleanup to eliminate 17GB bloat", 127 kills — died most *because it ran the
  heaviest git work inside the bloated repo*. Same for bf-1s6c3 (22 kills on the 13th
  after its 71 on the 12th). Cleanup beads topping the kill table is what a
  repository-bloat regime predicts and nothing else does.
- **Regime confirmed by the record and by measurement:** ~18 GB `.git` / ~17 GB loose
  objects on 2026-08-13 (documented; cleanup-verified 18 GB → 92 MB on 2026-09-01),
  versus this dispatch's live measurement — `.git` **106 MB**, 313 loose objects /
  2.15 MiB, pack 99.78 MiB, 0 garbage, `check-repo-health.sh` ✅ (effective pack-memory
  bound ≈3072 MiB within the 6 GiB ceiling, 0 unpushed backlog). The pattern's
  enabling condition is gone.
- **Mechanism stays regime-matched, not kernel-proven** (carried from the sibling
  legs, unchanged): the single journald boot starts 2026-08-15, so no Aug-13 kernel
  record survives to name the signal.

**Framework checklist:**

| Framework item | Finding |
|---|---|
| Repeated crashes, same agent/session | Yes — 7 in this loop; 344 across 13 beads for the same worker-day |
| Repository bloat indicators | Historical: ~18 GB `.git` / ~17 GB loose (>1 GB / >500 MB thresholds, 36× normal). Current: 106 MB, healthy |
| Memory pressure (system-wide OOM) | Workspace-scoped only — five peer workers: zero −1 in-window, ≤2 on their days; no host-wide event |
| SIGHUP cascades | None — 0 `sighup` records in the worker log; exit −1 is needle's death-by-signal *sentinel*, and the same era's kernel-proven kills (bf-198ne, bf-1ea4g) are SIGKILL memcg-OOM, not SIGHUP |
| Current pattern state | `./scripts/crash-pattern-detection.sh` (this dispatch): "No crashes detected in the last 24 hours — System Status: STABLE" — the pattern is closed, not ongoing |

## 5. Classification against the task's taxonomy

| Candidate | Verdict | Deciding evidence |
|---|---|---|
| **FALSE_POSITIVE** | **No** | Input instant is a heartbeat 12.9 s *after* a real kill; work was 4 attempts in; bead Open until Aug-17; Rule 1's 14.7 s hit on `164b62d` is a mid-task checkpoint, not post-completion cleanup (§1). The alerts were real when raised — they are **stale** now (target Closed, work complete), which is a different disposition |
| **TRANSIENT** (self-healed) | **Surface only** | Crash → retry → success same evening is real (§2), but kills continued for hours after the success and the regime survived 19 days — Rule 2's caveat converts the label to Infrastructure. Applies *only* to the bead's completion: zero work lost, deliverable live at HEAD |
| **ISOLATED_CRASH** | **No** | 10 exit −1 in the ±1 h window (bf-mje3pd + both chain neighbours), 344/13 beads on the day, one shared signature, one shared cause |

**Final: INFRASTRUCTURE EVENT — workspace-scoped repository-bloat regime, bead
recovered on retry, cause remediated 2026-09-01, alerts stale.** The five still-Open
alert beads naming this target (bf-56kmlk, bf-1cezsk, bf-1pidqn, bf-3dxljn,
bf-x88dnf — re-read live this dispatch; bf-1y1d0g and bf-3za7vh Closed) warrant no
further investigation; bf-1cezsk retires through its own final-verification leg
(domchk-40cd5fde) per the chain report.

## 6. Verification record (domchk-7dc8f0d6, this dispatch)

Executed 2026-09-07 ~21:0xZ, first-hand unless attributed:

- Worker log `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl`
  JSON-parsed by `event_type` (12,131 events): 281 bf-mje3pd events; 14 dispatched /
  13 completed / 12 classified = 7 × −1, 3 × 1, 1 × 124, 1 × 0; attempt-4 bracket
  `agent.completed −1 19:18:43.227Z` → `outcome.classified 19:18:43.233798Z` →
  `HANDLING_RELEASE_DONE 19:18:56.178350930Z` (input matches to ~0.2 ms; gap 12.945 s);
  success tail `exit 0 21:18:23.229Z` → `success 21:18:23.232Z` →
  `verification.passed 21:18:23.247Z (gates_run 1)` → `bead.orphaned 21:18:36.324Z`.
- Window/surge census computed over all six `claude-code-*2026-08-13.jsonl` logs
  (§3 table, per-minute and sliding 5/10-min maxima; day-level 7-in-5-min peak at
  22:17:20Z, 344 kills / 13 beads, per-bead and hourly histograms).
- `git show -s ea23bd1 164b62d`: author dates 19:02:32Z / 19:21:41Z, both contained
  only by `pre-squash-history-20260816`; diffstats read for content (detection scripts
  vs prevention stack); deltas to kills computed against first-hand kill instants
  (39.3 s / 14.7 s; 16.4 min commit→input-instant).
- `./scripts/check-repo-health.sh` ✅; `du -sh .git` → 106M; `git count-objects -vH` →
  313 loose / 2.15 MiB, 99.78 MiB pack, 0 garbage; `./scripts/crash-pattern-detection.sh`
  → no crashes in 24 h, STABLE; `grep -ic sighup` on the Aug-13 log → 0.
- `bead show`: bf-mje3pd Closed rev 2; seven alert beads re-read live (5 Open / 2
  Closed); domchk-f0513fd1 Closed rev 4; bf-29h1yy / bf-2o7nlw / bf-65lsdu titles read
  for the window and storm attribution.
- Guide citations re-checked at **HEAD** (`docs/crash-response-guide.md` lines 249,
  589–596, 608–617, 618+), not the dirty worktree copy; bloat figures re-checked at
  HEAD (`CLAUDE.md`, `docs/crashes/bf-4yjq-cleanup-verification.md`).
- Agreement with siblings: census matches domchk-bf8c4fd3 / domchk-1b407bff on every
  classified outcome; classification identical (INFRASTRUCTURE, repository-bloat
  regime). New in this leg: §3's threshold evaluation, §4's day-level surge peak and
  cleanup-bead kill-table tell, §1's Rule-1 naive-firing analysis, §2's post-success
  kill continuation. No disagreement found.
