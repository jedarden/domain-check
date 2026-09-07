# Crash Investigation Report: bf-6d3d6 — Identify Common Ancestor Commit

**Crash date:** 2026-08-13 (report written 2026-09-07)
**Target bead:** bf-6d3d6 "Identify common ancestor commit" — created 2026-08-13T11:12:50.305Z, closed 2026-08-13T13:12:31.199Z, reason `Completed`
**Report bead:** domchk-0dd6ea6e (claude-code-glm-5.3-flash-lab-roam-11)
**Scope:** stage-4 documentation deliverable of the bf-6d3d6 crash-alert pipeline — synthesizes stages 1–3 and renders the preventive recommendations
**Canonical record:** [docs/crash-investigations/crash-investigation-bf-6d3d6.md](../crash-investigations/crash-investigation-bf-6d3d6.md) (body 2026-08-16 + §2 re-verification domchk-9caed39b + §3 systemic analysis domchk-c382e224)

> **Position in the corpus.** This is the tracked deliverable of the pipeline
> crash-artifacts (domchk-2330bec7) → classification (domchk-f5eee65e) → root
> cause (domchk-ee248adc) → **this report** (domchk-0dd6ea6e), the closure chain
> gating alert bead **bf-14ydo**. Stages 1–3 wrote gitignored JSON artifacts
> under `.beads/logs/` (`.gitignore:66 .beads/`), so this file is the only
> git-tracked member of the chain. It adds no new analysis to the canonical
> report; it consolidates the pipeline's findings, re-verifies every
> live-state claim first-hand at write time (§7), and points at the canonical
> doc for depth. Do not fork the analysis into further new files — append to
> the canonical doc per the corpus dedup-append convention.
>
> **Naming note:** the dispatch's acceptance-criteria line asked for
> `investigation-bf-6d3d6-YYYY-MM-DD.md` while its Output section named
> `bf-6d3d6-crash-investigation-2026-08-13.md`; the explicit Output path is
> used here, dated by the crash (2026-08-13) rather than the write date
> (2026-09-07) — both dates are stated above so the corpus stays unambiguous.

---

## §1 Executive summary

- **bf-6d3d6 was not a code failure and lost no work.** It was a one-command
  read-only task (`git merge-base` for a Forgejo/GitHub divergence analysis)
  whose deliverable — `docs/.branch-divergence-temp.json` — was committed in
  attempt 1 as `e09cf84` at 11:16:27Z, **29.5 s before the first of six
  process kills**. Attempt 8 then exited 0, passed all validation gates, and
  the bead closed `Completed` 2 h after creation.
- **Classification: INFRASTRUCTURE — repository-bloat kill regime
  (2026-08-12/13 era).** Six genuine signal deaths (needle sentinel
  `exit -1`, not a signal number) at 11:16:56Z–13:10:10Z, in the era when
  this repo's `.git` held ~18 GB (≈17 GB loose objects from 17+ identical
  237 MB `.beads/*.jsonl` snapshots) and a 422-commit unpushed backlog
  amplified every pack-objects run. Per-instant kill mechanism is attributed
  by era evidence only — no Aug-13 kernel record survives.
- **The alerts were false positives as actionable work** (the crashes were
  real; the work was not). Pre-0.4.2 needle minted one alert bead per kill:
  `bf-1qht8`, `bf-1936h`, `bf-w4fwe`, `bf-2r30u`, `bf-14ydo`, `bf-5npjj`.
  Four closed by 2026-08-26; **`bf-14ydo` and `bf-5npjj` are still Open on
  2026-09-07** — by design, each gated by its own closure-bead blockers,
  with this pipeline being bf-14ydo's leg.
- **The crash mechanism is repaired and verified holding.** Repo: `.git`
  106 MB, 372 loose objects / 3.50 MiB, one 99.11 MiB pack, local-vs-origin
  divergence **0/0** (re-verified 2026-09-07). Recurrence risk for this
  mechanism: none in the current repo state.
- **Residual risk is process, not infrastructure:** retry without a
  stop-condition (PF-2/H-1) and dispatch bookkeeping committed to shared
  main (PF-3) are still live; alert-premise validation (PF-4) is partial.
  This very pipeline is an instance of the residue — it exists because six
  stale alerts kept regenerating investigation work 25 days after the
  target closed.

## §2 Crash timeline and artifacts

### §2.1 Attempt timeline (2026-08-13, all UTC)

Eight claims across two worker sessions, six kills, one success — compiled by
stage 1 (`.beads/logs/bf-6d3d6-crash-artifacts.json`) and re-verified against
`~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2` at write
time (32 mentions of bf-6d3d6: exactly **6 × `exit_code=-1 outcome=Crash(-1)`**
and **1 × `exit_code=0 outcome=Success`**, plus six `crash alert bead created`
lines with ids in kill order).

| # | Event | Time (UTC) | Alert minted |
|---|---|---|---|
| — | bead created | 11:12:50.305 | — |
| 1 | claim #1 (session 8446529e) | 11:14:14.892 | — |
| 1 | **deliverable committed `e09cf84`** (`docs/.branch-divergence-temp.json`) | **11:16:27** | — |
| 1 | Crash(−1) #1 | 11:16:56.548 | `bf-1qht8` (11:17:06Z) |
| 2 | claim #2 → Crash(−1) #2 | 11:17:09.353 → 11:20:54.151 | `bf-1936h` (11:21:03Z) |
| 3 | claim #3 (11:21:03.406) | | |
| 4 | claim #4 (session e29942f7) 12:57:54.223 → Crash(−1) #3 13:02:11.273 | | `bf-w4fwe` |
| 5 | claim #5 13:02:23.676 → Crash(−1) #4 13:06:16.782 | | `bf-2r30u` |
| 6 | claim #6 13:06:29.626 → Crash(−1) #5 13:08:17.766 | | `bf-14ydo` (13:08:28Z) |
| 7 | claim #7 13:08:30.867 → Crash(−1) #6 13:10:10.261 | | `bf-5npjj` |
| 8 | claim #8 13:10:24.500 → **exit 0, `outcome=Success`**, gates passed | 13:12:47.884 | — |
| — | bead closed `Completed` | 13:12:31.199 | — |

Every kill instant **post-dates the committed deliverable** — attempts 2–8
re-ran a bead whose work was already on disk. Kill gaps (4.0 / 101.3 / 4.1 /
2.0 / 1.9 min) are fixed-cadence re-dispatch, not escalating workload.

**Timestamp hygiene (from stage 1):** the alert body timestamp carried by
bf-14ydo's dispatch (13:08:26.256Z) postdates the worker's Crash handling line
(13:08:17.767Z) by ~8.5 s — the standard post-kill heartbeat pattern. Alert
timestamps are handling-release instants, never death instants; the true death
is ≤ the worker log line.

### §2.2 Artifacts

| Artifact | Location | Tracked? |
|---|---|---|
| Stage 1 crash-artifacts JSON (domchk-2330bec7) | `.beads/logs/bf-6d3d6-crash-artifacts.json` | No (`.beads/` gitignored) |
| Stage 2 classification JSON (domchk-f5eee65e) | `.beads/logs/bf-6d3d6-classification.json` | No |
| Stage 3 root-cause JSON (domchk-ee248adc) | `.beads/logs/bf-6d3d6-root-cause.json` | No |
| **This report (domchk-0dd6ea6e)** | `docs/investigations/bf-6d3d6-crash-investigation-2026-08-13.md` | **Yes** |
| Target bead's own deliverable | `docs/.branch-divergence-temp.json` via commit `e09cf84`; working-tree copy deleted 2026-09-01 (`079905a`); recoverable as `git show c27899f:docs/.branch-divergence-temp.json` | Content survives in squash `c27899f` |
| Canonical narrative report | [docs/crash-investigations/crash-investigation-bf-6d3d6.md](../crash-investigations/crash-investigation-bf-6d3d6.md) | Yes |
| Needle worker log (primary crash evidence) | `~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2` | No (outside repo) |

**Deliverable content and its SHA caveat.** The state file recorded common
ancestor `63ba02474c9b6bc339388adb3a44542e10755a10` (jedarden, Sun Aug 9
13:00:56 2026 −0400, "fix: remove unused time import and update bootstrap test
initialization"). That SHA **does not resolve in this clone** — orphaned by the
2026-08-16 squash `c27899f` or mistranscribed. Commit `e09cf84`'s own message
names `00117cb879ecba7b1a819d80f1e4980ccb5d2881` for identical metadata, and
that object **exists and is an ancestor of main today** (verified
`merge-base --is-ancestor`). The identified change is pinned by its metadata
either way; the surviving name is `00117cb8…`. Downstream consumption of the
chain succeeded regardless (`510bf34` divergence statistics, consolidated in
[docs/branch-divergence-analysis.md](../branch-divergence-analysis.md)).

## §3 Findings from the pipeline stages (child beads 1–3)

### Stage 1 — domchk-2330bec7, crash artifacts

Established the evidentiary base: bead metadata at close (revision 1, assignee
`claude-code-glm-4.7-lab-domain-check`, close reason `Completed`, never
reopened); the full 18-event attempt timeline (§2.1); the dispatch-named crash
instant mapped to Crash(−1) #5 and identified as a heartbeat, not a death
instant; the alert-bead inventory (6 ids in kill order); the
self-healed-retry outcome. Key derived facts: **6 crash handlings, 8 claims,
1 successful completion; zero work lost.**

### Stage 2 — domchk-f5eee65e, classification

Applied `docs/crash-response-guide.md`'s classification matrix:

- **Matched (both `-1` rows):** "-1 | Signal death, code unrecorded (needle
  sentinel) → Infrastructure event" and "-1 | Fixed-cadence re-dispatch
  deaths, `.git` > 5GB → **Infrastructure: Repository bloat**". The era's
  ~18 GB `.git` sits far above that row's 5 GB trigger.
- **Excluded with evidence:** WORKFLOW_FAILURE (0 `max_turns` markers across
  all 32 log mentions), SERVICE_FAILURE (zero 503/502/gateway markers in both
  kill windows), SIGKILL-as-137 (note 2: `-1` is a sentinel, no signal number
  may be asserted from it), timeout-at-124 (kills were not at the dispatch
  cap), CODE_DEFECT (deliverable was a docs/state-file commit; no application
  error in any window).
- Task-outcome dimension: **SELF-HEALED RETRY — no work lost**.
- An independent re-verification (roam-9, 2026-09-07 16:05Z) confirmed all six
  crash records to <1 ms, the eight byte-exact load readings
  (9.58/12.45/11.69/13.17/31.64/14.68/10.45/12.10 — the box was saturated, and
  one reading of 31.64 sits deep in the bloat era's load profile), and the
  invalid-object status of the dispatch SHA `b6d1439`.

### Stage 3 — domchk-ee248adc, root cause and false-positive status

- **Root cause (§4 below):** INFRASTRUCTURE, repository-bloat kill regime,
  high confidence on class, per-instant killer unknowable.
- **False-positive determination, stated precisely:** crash events are
  **GENUINE** (six real kill handlings, not fabricated, not a "no crash"
  case); the alerts were **false positives as actionable work** — no
  investigation action was ever warranted, because the deliverable predates
  the first kill and the task completed with zero work lost. This corrects
  the earlier close reasons that said "no crash occurred": the crashes were
  real; what was false was the premise that the target work needed rescue.
- **Corrections to prior records:** close reasons on bf-14ydo / bf-5npjj /
  bf-1936h cite commit `b6d1439`, which is **not a valid object** in this
  clone (`git cat-file -t` fails) — the real deliverable commit is `e09cf84`
  and the real investigation doc is the canonical report; the guide's
  "work committed < 30 s before crash → FALSE_POSITIVE" heuristic technically
  fires at 29.5 s but **understates** this case, where all six kills were
  post-deliverable, not just post-completion cleanup.
- **Alert-pool status at its write time:** four closed; bf-14ydo + bf-5npjj
  open by design with their closure-bead blockers named — which is what
  unblocked this stage-4 bead.

## §4 Classification and root cause

**Classification: INFRASTRUCTURE — repository bloat (2026-08-12/13 bloat-era
kill regime). Confidence: high on class; per-instant mechanism attributed by
era evidence only.**

Mechanism chain:

1. **Aug-12 bloat:** 17+ identical 237 MB `.beads/*.jsonl` snapshots committed
   to git → `.git` ~18 GB, ~17 GB of it loose objects (`.beads/` still
   tracked).
2. **Kill regime:** any significant git operation (gc, push, status pack work)
   could exceed the needle dispatch scope's 12 GiB `MemoryMax` → memcg-OOM
   SIGKILL → needle records `exit -1` (its sentinel for a signal death with
   unrecorded code — **not** a signal number).
3. **Alert cardinality = kill cardinality** (pre-0.4.2 needle): one alert bead
   per kill → six alerts for one bead.
4. **Retry with no stop-condition:** re-dispatch never asked whether the
   deliverable already existed, so six attempts paid full kill exposure to
   re-derive finished work.
5. **Self-heal:** attempt 8 completed cleanly (exit 0), gates passed, bead
   closed `Completed`.

Caveats that bound the claim:

- The **per-instant killer for these six kills is unknowable**: no
  `/var/log/messages` on NixOS, `dmesg` lost past the two Aug-14 reboots, and
  journald's single surviving boot starts 2026-08-15 19:56 EDT. Era
  attribution (INFRASTRUCTURE / repo-bloat) is high-confidence; memcg-OOM for
  any single Aug-13 kill is not kernel-provable.
- **Commit count amplified; object size killed.** The canonical report's
  original body §4 (2026-08-16) attributed the crash to "741 commits ahead of
  origin bloating history" — superseded by §2.5/§3.1 of the canonical doc:
  the unpushed backlog (422 on 08-13) raised the cost of the pack-objects
  kill site, but the ~17 GB of loose objects was the resource driver. The 741
  figure itself is an unrecomputable era measurement (its baseline `61d27ac`
  was destroyed by the same-day squash); the surviving pre-squash backup
  branch corroborates magnitude: 722 commits, 180 (≈25%) crash-labeled
  subjects.

## §5 Impact

- **Target work:** completed in attempt 1; **zero work lost** across six
  kills; bead closed `Completed` the same day.
- **Alert-layer cost (the real residue):** six investigation beads for one
  bead's six kills. Four closed by 2026-08-26. **`bf-14ydo` and `bf-5npjj`
  remain Open on 2026-09-07 — 25 days after the target closed** — each gated
  by its own closure-bead blockers. bf-14ydo has been closed and reopened
  four times historically; closing it from a triage pass bounces while its
  blockers are open, so closure belongs to those chains.
  **Dated update (2026-09-07, later the same day — bf-14ydo's own closing
  pass):** both of bf-14ydo's closure-bead blockers — `domchk-02bcb329` and
  this report's own bead `domchk-0dd6ea6e` — are Closed in the store
  (verified live immediately before closing), so the bead was closed from
  its own chain as this report prescribes, with the corrected reason:
  crashes genuine, zero work lost, alert a false positive only as
  actionable work. **`bf-5npjj` remains Open**, still gated
  (`domchk-81938e89` ← `domchk-6129d0c7`, both live at that check).
- **Waste continued past the crash layer's fix:** stages of this very chain
  were dispatched on stale premises — a fabricated SHA (`b6d1439`), a
  pre-consolidation path, and the restated "741 commits" figure — five weeks
  after the kill regime itself was repaired.
- **Era context:** bf-4k2ws absorbed 55 kills and bf-1ea4g 56 on the same
  day; bf-6d3d6's six are a small slice of one storm.

## §6 Preventive recommendations

Two layers: what already prevents a recurrence (verified live 2026-09-07),
and what still needs doing (the process residue that kept this pipeline
alive).

### §6.1 In place — why this cannot recur as a crash

| Layer | Mechanism | Verified |
|---|---|---|
| Resource driver | `.beads/` fully gitignored (repo-wide `*.jsonl`/`*.db` too), 0 tracked bead-state files; the 237 MB snapshot class can no longer be committed | `git ls-files .beads` → 0; repo `.git` 106 MB, 372 loose / 3.50 MiB, 0/0 divergence |
| Commit gate | 10 MB pre-commit hook (per-clone, `scripts/setup-git-hooks.sh`) — the backstop that would have blocked the Aug-12 snapshots | `./scripts/setup-git-hooks.sh --check` |
| Kill site | `pack.windowMemory=2g` + `pack.deltaCacheSize=1g` + `pack.threads=1`, repo-local and global — bounds pack-objects under gc **and** push | `./scripts/setup-git-gc-config.sh --verify` |
| Detection | Repo-health + auto-gc + full-gc systemd user timers; resource/service monitors | `systemctl --user list-timers 'domain-check-*'` |
| Alert cardinality | needle ≥0.4.2 dedup (ends one-alert-per-kill) + `scripts/crash-alert-manager.sh` six-fix stack (closed-bead filter, dedup + processed-alert tracking, completion awareness, 5-min cooldown, classification) | suite `test-crash-alert-fixes.sh` 12/12 |
| Work-completion check | `./scripts/verify-work-completion.sh <bead-id>` before close — fails on unpushed commits / missing artifacts, writes `.beads/state/work-completion/<id>.json` so triage can tell post-deliverable kills from mid-task ones (exactly the discriminator this crash needed) | script + state dir in use |
| **Backlog observability (M-1)** | **Implemented 2026-09-07 (`8d326cc`)**: `scripts/check-unpushed-backlog.sh`, wired into `check-repo-health.sh` §9 — WARN ≥50, CRITICAL ≥200 unpushed commits | read in `scripts/check-repo-health.sh:107-124` this session |

### §6.2 Still open — the process residue (as of 2026-09-07)

| ID | Gap | Status | Recommended action |
|---|---|---|---|
| **PF-1** | Alert cardinality equalled kill cardinality | ✅ Fixed at source and response layer; **residue: `bf-5npjj` still Open** — `bf-14ydo` closed 2026-09-07 via its own chain with a corrected reason (§5 dated update) | Close them **only via their own closure-bead chains** (this report closes one leg of bf-14ydo's). Never from a triage pass — their closes bounce while blockers are open, and the close reasons citing "no crash occurred" and `b6d1439` are wrong as stated (crashes genuine; alert stale). |
| **PF-2** | Retry with no deliverable/stop-condition — re-dispatch never checked whether the work already existed | 🔴 Open (H-1, [bf-1ea4g root-cause determination](bf-1ea4g-root-cause-determination-2026-09-02.md)) | Implement a retry-time deliverable check: before re-dispatching a released bead, test its acceptance-criteria artifacts on disk/in-history (here: "does the state file exist in a commit?") and short-circuit to close. `verify-work-completion.sh` covers the triage side only, not the retry decision. This single check would have prevented attempts 2–7 entirely — five of six kills and three of six alerts. |
| **PF-3** | Dispatch bookkeeping committed to shared main (`.needle-predispatch-sha` churn per dispatch) | 🔴 **Open and still live** — file tracked, no `.gitignore` rule (`git ls-files` hit verified this session) | Gitignore it or move it out of the repo, as was done for `.beads/`. It is the last un-gitignored member of the per-dispatch-state class this crash era minted. |
| **PF-4** | Alert/dispatch premises regenerated from templates, unvalidated against repo state (fabricated `b6d1439`; stale path; stale "741") | 🟡 Partial — dedup-append convention + this corpus's premise-correction sections are the working mitigation | Add mechanical premise validation at dispatch time: any cited SHA must `cat-file -t` clean, any cited path must exist at the current tip, any era figure must be recomputable — else drop it from the template. Cheapest of all the remaining fixes. |
| **PF-5** | No feedback from the loop to its operators | 🟡 Half closed — **M-1 implemented 2026-09-07**; **M-2 🔴 open** | M-2: expose dispatch-scope memory telemetry (the 12 GiB memcg that did the era's killing is still unobserved); host-wide monitoring cannot see a per-scope OOM. |

**Priority order:** PF-4 (cheap, would have prevented this pipeline's own
stale dispatches) → PF-2 (prevents the wasted kill exposure) → PF-3
(removes the remaining shared-main churn) → M-2 (closes the last
observability hole). None of these are needed to prevent the *crash* — §6.1
already does that — they prevent the *waste around* a crash of this shape.

**Operational rule this case adds to triage:** when a target bead is closed
and its deliverable predates the first kill, classify the alerts as stale
(false positive *as actionable work*) even when the crashes themselves are
genuine — and record both halves of that sentence. Half of it ("no crash
occurred") is what the earlier close reasons got wrong.

## §7 Method (reproducible, this report's own run 2026-09-07)

```
# live bead states (target, six alerts, pipeline stages, blockers)
bead show bf-6d3d6 bf-14ydo bf-5npjj bf-1qht8 bf-1936h bf-w4fwe bf-2r30u \
          domchk-2330bec7 domchk-f5eee65e domchk-ee248adc domchk-02bcb329

# primary crash evidence — counts, not greps of prose
grep -c 'bf-6d3d6' ~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2   # 32
grep 'bf-6d3d6' <log.2> | grep -o 'exit_code=[^,]*' | sort | uniq -c                  # 6× Crash(-1), 1× Success
grep 'bf-6d3d6' <log.2> | grep -io 'crash alert bead created[^,]*'                    # 6 alert ids in kill order

# deliverable survival + repo state
git show --no-patch --format='%H %aI %s' e09cf84
git merge-base --is-ancestor e09cf84 HEAD
git rev-list --count origin/main..HEAD; git rev-list --count HEAD..origin/main        # 0 / 0
git count-objects -vH; du -sh .git                                                    # 372 / 3.50 MiB / 106M
git show c27899f:docs/.branch-divergence-temp.json                                    # target deliverable

# process-residue status
git ls-files .needle-predispatch-sha; grep -c predispatch .gitignore                  # tracked / no rule
sed -n '107,124p' scripts/check-repo-health.sh                                        # M-1 wiring (8d326cc)
git show --no-patch --format='%h %cI %s' 8d326cc                                      # M-1 landed 16:09:11Z
```

Stage artifacts read directly: `.beads/logs/bf-6d3d6-crash-artifacts.json`,
`.beads/logs/bf-6d3d6-classification.json`,
`.beads/logs/bf-6d3d6-root-cause.json`.

## §8 Related investigations

| Document | Relationship |
|---|---|
| [crash-investigation-bf-6d3d6.md](../crash-investigations/crash-investigation-bf-6d3d6.md) | **Canonical record for this bead** — 2026-08-16 body, §2 re-verification (domchk-9caed39b), §3 systemic analysis (domchk-c382e224). Deep record; this file consolidates. |
| [bf-4k2ws-crash-investigation.md](../crash-investigations/bf-4k2ws-crash-investigation.md) | Same-day storm (55 kills, 2026-08-13); set the INFRASTRUCTURE / repo-bloat era classification this report applies; its §14.6 documents the stale-premise dispatch vector. |
| [bf-1s6c3 analysis (2026-09-06)](../crash-analysis-bf-1s6c3-2026-09-06.md) | Root of the bloat era: the 237 MB `.beads/*.jsonl` commits, 18 GB repo, and the fix that ended the mechanism. |
| [bf-1ea4g root-cause determination](bf-1ea4g-root-cause-determination-2026-09-02.md) | Push-side variant of the same kill regime; owns H-1 (PF-2), M-1 (now closed), M-2 (open). |
| [verification report bf-14ydo (archived)](../archive/crash-investigations/verification-report-bf-14ydo-false-positive-alert-resolved-bf-6d3d6.md) | The alert bead this pipeline gates; archived 2026-08-26 verification that first reported `b6d1439` as a fabricated SHA. |
| [docs/branch-divergence-analysis.md](../branch-divergence-analysis.md) | The analysis chain bf-6d3d6 was step 1 of — where its deliverable was consumed. |
| [docs/crash-response-guide.md](../crash-response-guide.md) | Classification matrix applied by stage 2 (rows at lines 14–17; sentinel note 2). |
| [docs/crash-prevention-requirements.md](../crash-prevention-requirements.md) | G/H/M gap register that PF-2/PF-5 statuses resolve against. |

---

**Report completed:** 2026-09-07
**Investigating bead:** domchk-0dd6ea6e (claude-code-glm-5.3-flash-lab-roam-11)
**Pipeline:** crash-artifacts domchk-2330bec7 → classification domchk-f5eee65e → root cause domchk-ee248adc → **this report**
