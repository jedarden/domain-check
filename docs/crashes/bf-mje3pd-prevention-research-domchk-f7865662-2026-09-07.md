# bf-mje3pd — Preventive-Measure Research Brief

**Research bead:** domchk-f7865662 (2026-09-07)
**Depends on:** domchk-1b407bff (root-cause classification, `7672f9e`) — classification
**INFRASTRUCTURE / repository-bloat regime**, regime-matched not kernel-proven.
**Purpose:** child 3 of the bf-mje3pd chain — inventory what prevention already exists for
this crash type, what its gaps are, and what the next bead should do. Every status claim
below was verified live on 2026-09-07 against HEAD `60f8a87` and the running system; the
worktree was 110 files dirty at dispatch time, so all file/line citations were checked with
`git show HEAD:<path>`, not the working tree. Head advanced to `e7f5b97` before commit; see
§7 for the re-verification.

---

## 1. The crash type being prevented

Four links, from the dependency RCA:

1. **Regime (necessary condition):** the workspace `.git` in the bloat state (~18 GB,
   ~17.16 GB loose on 2026-08-13; origin bf-2ildm committing 17+ identical 237 MB
   `.beads/*.jsonl` snapshots).
2. **Mechanism (regime-matched):** git-heavy agent work inside the 12 GiB memcg dispatch
   scope dies by signal (needle `exit -1` sentinel); no kernel record survives for this
   bead (single journald boot starts 2026-08-15).
3. **Amplifier:** needle re-claims the bead 1.9–17.6 s after every release — 14 dispatches,
   7 kills in 43m22s for this one bead.
4. **Alert-layer residue:** pre-0.4.2 needle emits one alert bead per kill with no
   lifecycle to retire it — five alert beads naming bf-mje3pd are still Open today.

Layers 1–2 are *prevented* (repo repaired 2026-09-01, held since). Layer 3 is
*unaddressed and out of this repo*. Layer 4 is *detected but not closed* — the newest and
most actionable gap.

## 2. Sources reviewed (acceptance-criteria map)

| Criterion | Source | Note |
|---|---|---|
| Reviewed `docs/crash-mitigation-strategies.md` | ✅ v2.3 at HEAD | Priority 3 (repo bloat) is this crash type; its "Implementation Status — bf-1s6c3 Crash Type" section live-verifies the stack, re-verified 2026-09-07 (domchk-4e8821ca). Carries superseded figures in its older body — see §5 note. |
| Checked monitoring/maintenance scripts | ✅ `scripts/` inventoried against HEAD + 8 installed timers inspected (§3) | |
| Preventive measures from crash docs | ✅ `docs/crash-prevention-requirements.md` (G-1..G-13 canon), `docs/crash-investigations/bf-4yjq-crash-chain-resolution-domchk-a6059a67-2026-09-07.md` §6 (stack table), `docs/crash-prevention-gaps-bf-1ea4g.md` (H/M/L items), `docs/maintenance/repository-maintenance-guide.md` | |
| Related crash patterns + resolutions | ✅ §4 below | |
| Recommended actions | ✅ §6 below | |

## 3. Applicable existing preventive measures — live-verified 2026-09-07

All commands below were run by this dispatch. Repo state at verification: `.git` **106 MB**,
313 loose objects / 2.15 MiB, 2 packs / 99.78 MiB, 0 garbage, `check-repo-health.sh` exit 0,
0 unpushed commits.

| # | Layer | Mechanism | Live verification this dispatch |
|---|---|---|---|
| 1 | Source elimination | whole `.beads/` + `*.db` + `*.jsonl` gitignored (`.gitignore:66-70` at HEAD) | `git ls-files .beads` → 0 tracked files |
| 2 | Commit gate | pre-commit repo-size hook (10 MB/file, 50 MB/commit, hard block on staged `.beads/` paths) | `scripts/setup-git-hooks.sh --check` → exit 0, "byte-identical to tracked source" |
| 3 | Pack-memory bounds | `pack.windowMemory=2g`, `deltaCacheSize=1g`, `threads=1` — bounds bare `git gc` **and** `git push` pack-objects | `setup-git-gc-config.sh --verify` → exit 0, worst case ≈3072 MiB per pack run |
| 4 | Bounded maintenance | `safe-git-gc.sh` (staged, checkpoint/resumable, preflight), `cleanup-repo-bloat.sh`, `check-repo-health.sh` | all tracked at HEAD; health check exit 0; "No unmanaged aggressive git gc/repack running" |
| 5 | Scheduled enforcement | systemd **user** timers (NixOS — no crontab) | **8 timers, all future-triggered**: service 2 min, resource 5 min, crash-pattern 10 min, alert-triage hourly, repo-health daily 02:00, auto-gc daily 02:30, gc daily 03:00, full gc Sun 04:00 |
| 6 | Alert hygiene | closed-bead filter, dedup, 5-min cooldown, classification (`crash-alert-manager.sh` + `crash-classifier.sh`) | both suites re-run this dispatch from the repo cwd: `test-crash-alert-fixes.sh` exit 0, `test-closed-bead-filter.sh` exit 0 |
| 7 | **Threshold-triggered remediation (new since the 2026-09-06 stack table)** | daily 02:30 `domain-check-auto-gc.timer` → `safe-git-gc.sh --auto-when-needed` under `MemoryMax=4G` | ran 2026-09-07 02:30:09 — lock acquired, thresholds checked (loose ≤1000, packs ≤5, loose ≤500 MiB, repo ≤1024 MiB), verdict "gc not needed". Correctly supersedes the G-2 note in `docs/crash-prevention-requirements.md`, which recorded `--auto-when-needed` as *deliberately not implemented* on 2026-09-07 — it has since landed and is scheduled. The unconditional 03:00 gc remains as the backstop; the conditional 02:30 run adds nothing harmful (it only skips work the thresholds say is unneeded) |
| 8 | **Alert triage sweep (new, hourly)** | `domain-check-alert-triage.timer` → `scripts/alert-triage-sweep.sh`, report-only, queue at `.beads/state/alert-triage/queue.jsonl` | latest sweep 2026-09-07 20:01:06Z: alerts=240, targets=29, **resolved=222**, keeper=2, fanout_dup=14, needs_review=2. **All five bf-mje3pd alert beads are in the queue as `RESOLVED_TARGET`** (evidence: `resolution_type: bead_closure, target_status: closed`) — detection has caught every one of this chain's stale alerts |

Layers 1–6 are the stack `docs/crash-mitigation-strategies.md` §"Implementation Status" and
`bf-4yjq-crash-chain-resolution` §6 document; layers 7–8 landed after those documents were
written and are the reason this brief exists rather than re-affirming their "one open gap"
finding.

## 4. Related crash incidents and their resolutions

Same regime (bloat → git-operation memory death in the dispatch scope), all resolved:

| Bead | Date | Shape | Resolution |
|---|---|---|---|
| **bf-2ildm** | 2026-08-12 | *Origin* — committed 17+ identical 237 MB `.beads/*.jsonl` snapshots | State snapshots retired; `.beads/` gitignored so the vector cannot re-open (layer 1) |
| **bf-1s6c3** | 2026-08-12/13 | 76 dispatches / 71 memcg-OOM kills in the 18 GB store | Store packed; prevention stack built on top; canonical record `docs/crash-analysis-bf-1s6c3-2026-09-06.md` |
| **bf-4yjq** | 2026-08-12 | 50 exit -1 kills, ~3.1-min cadence | Chain closed end-to-end; `docs/crash-investigations/bf-4yjq-crash-chain-resolution-domchk-a6059a67-2026-09-07.md` |
| **bf-mje3pd** | 2026-08-13 | 14 dispatches / 7 kills while shipping the prevention scripts into the still-bloated repo | Succeeded on dispatch 14, closed 2026-08-17; scripts it shipped are ancestors of layers 1–4 |
| **bf-1ea4g** | 2026-08-13 | Push-side variant: unbounded `git push` pack-objects over a 422-commit backlog; 54/57 transcripts die in push | Root-caused (`docs/crash-inventory-bf-1ea4g-summary.md`); layer 3 covers push as well as gc; gap analysis `docs/crash-prevention-gaps-bf-1ea4g.md` |
| **bf-198ne** | 2026-08-16 | Push variant with surviving kernel proof (`CONSTRAINT=MEMCG`, `usage == limit == 12 GiB`) | `docs/crashes/bf-198ne-crash-report.md`; re-verified resolved 2026-09-06 |
| **bf-4x12ec** | 2026-08-14 | Bare `git gc --aggressive` memcg-OOM, 129 attempts | Now reproducible-safe under bounds: bare gc peak RSS ≈313 MiB in the 768 MiB cgroup test (`scripts/test-gc-memory-bounds.sh`) — the *config* makes the death command survivable, which is why the remaining problem is only that HEAD still *recommends* it (§5 gap 1) |
| **bf-4k2ws** | 2026-08-13 | 55 real kills same era; alert storm on top | Classification `ef39024`; false positives confined to the alert layer |

Class-level status: **no bloat-era crash has recurred since 2026-08-17**, and repo state has
been re-measured healthy on 09-01, 09-06, 09-07 (three independent verifications).

## 5. Gaps in prevention coverage for this crash type

Ordered by relevance to this crash type, each with this dispatch's evidence.

**Gap 1 — HEAD's emergency-cleanup path still ships and still documents the death command.**
At HEAD, `scripts/cleanup-bloat.sh` is a 21-line script whose body is
`git gc --aggressive --prune=now` — the exact operation that memcg-OOM'd bf-1s6c3's era and
bf-4x12ec's 129 attempts — and `docs/repository-health.md` still instructs running it
(`git show HEAD:docs/repository-health.md` lines 80, 113: "Run aggressive GC if needed",
"Runs `git gc --aggressive --prune=now`"). Layer 3's pack-memory config makes this
survivable today (that is what `test-gc-memory-bounds.sh` proves), so it is no longer an
OOM hazard on this repo — but the repo's own documented emergency procedure bypasses every
bounded path (no preflight, no checkpoint, no fsck, no lock) and is the one instruction a
panicking agent is most likely to follow. A safe 809-line rewrite exists **uncommitted** in
the worktree and was audited by domchk-7de40513 (commit `60f8a87`: memory monitoring
partial, fsck gap, single-pid kill orphans pack-objects, 10 ordered gaps). Nothing at HEAD
prevents the next fresh clone from shipping the bare script.

**Gap 2 — Alert lifecycle: detection landed, closure did not.** Five alert beads naming
bf-mje3pd (bf-1cezsk, bf-56kmlk, bf-1pidqn, bf-3dxljn, bf-x88dnf) are **Open** — re-read
live via `bead show` this dispatch. Their target closed 2026-08-17; every prior
verification converged on the same disposition (*crashes genuine, work complete, alerts
stale — no remediation required*). The hourly sweep now classifies all five
`RESOLVED_TARGET` in its queue, but is **report-only by design** ("it never closes, updates,
or creates a bead") and delegates closure to a closure bead that does not exist. Fleet-wide
the sweep holds 222 resolved-target alerts in the same state. Until a closure leg runs,
each stale alert remains re-dispatchable bait — the exact mechanism that produced this
research bead and its siblings.

**Gap 3 — The two newest layers are not reproducible from git.** Neither
`scripts/alert-triage-sweep.sh`, nor its installer `scripts/setup-alert-triage-timer.sh`,
nor its test `scripts/test-alert-triage-sweep.sh`, nor the four
`domain-check-{alert-triage,auto-gc}.{timer,service}` unit files, nor
`safe-git-gc.sh`'s `--auto-when-needed` flag are tracked at HEAD — all exist only on disk
in this worktree. The installed 02:30 unit invokes a flag HEAD's `safe-git-gc.sh` does not
have, so a fresh clone plus the tracked installer cannot reproduce the running state, and
the running state has no source of truth. (`setup-repo-maintenance.sh` in the worktree
references the auto-gc units but not the alert-triage ones; `bead-split-recommender.sh`,
listed as present by the requirements doc, is also disk-only.) This is the same shape G-1
was in before `setup-git-hooks.sh` landed: working here, invisible to the next clone.

**Gap 4 — The amplifier is NEEDLE-side and unowned.** The 1.9–17.6 s re-claim loop turned
7 kills into 14 dispatches for bf-mje3pd and 71 kills for bf-1s6c3. Nothing in this repo
can change it. Recorded as H-1 (`crash-prevention-gaps-bf-1ea4g.md`) and G-9/G-10
(`crash-prevention-requirements.md`): work-completion detection at the alert source, a
post-completion grace period, and dispatch-scope sizing for memory-heavy work. Correct
disposition is an external ask, not local work.

**Gap 5 — Classifier capture-race shape still yields bare `UNKNOWN`.**
`docs/crash-fix-strategy-domchk-3b605127-2026-09-07.md` §3.2 (at HEAD) proposes emitting
`INFRASTRUCTURE` + mechanism instead of bare `UNKNOWN` when a trace says `outcome: crash`
against a silent `events.jsonl` — the bf-57nao4 shape, where the kill wave takes the worker
before it writes the crash record. Verified at HEAD: `classify_crash` still exits 2
`UNKNOWN` when the trace slot is not provenance-ok, and the fall-through default is still
`UNKNOWN`. The classifier *has* since gained trace-provenance gating and repo-bloat
corroboration (both relevant to this crash type — a provenance-ok trace over a bloated
`.git` now corroborates the bloat-OOM mechanism), but the specific capture-race fix is not
landed. Impact: the *next* bloat-era kill is likelier to be under-classified than
mis-routed; classification remains possible from the worker log per the documented window
limits.

**Gap 6 — Bloat-detection false signal in `repo-health-monitor.sh`** (L-2 of the bf-1ea4g
gap analysis, recorded 2026-09-07, implementation bead owed): `PACK_FILES_WARN=2` flags
normal bounded churn (3 packs / 99 MiB on a 106 MB repo) regardless of pack size, and the
script is wired to no timer, so it can only mislead a manual run. Pack **count** is a
non-signal; `size-pack` and loose volume are the operative metrics.

**Out-of-scope-but-recorded** (other crash classes, unchanged from the requirements doc):
G-4 gateway failover/retry enforcement, G-5 prevention feedback loop, G-7 the cron-based
`monitoring-setup.sh` trap (still tracked at HEAD), G-8/M-3 evidence retention (worker
transcripts ≥30 days ranked highest-value).

**Stale-figure note for readers of `docs/crash-mitigation-strategies.md`:** its 2026-09-01
body predates the corrections the later records carry — bf-4yjq's "9 crashes over 2.5
hours" (correct: 50 kills for bf-4yjq; 7 for bf-mje3pd across 14 dispatches), "SIGKILL from
OOM" as observed fact (correct: sentinel; mechanism regime-matched for this bead,
kernel-proven only for bf-198ne), and its crontab scheduling proposals (this box is NixOS —
systemd user timers are the mechanism, and are installed). Its Priority-3 *conclusions* and
its live-verified stack section remain correct.

## 6. Recommended actions for the next bead

Ordered; items 1–3 are small, local, and directly close this crash type's residual exposure.

1. **Commit the safe `cleanup-bloat.sh` rewrite (or minimally neutralize HEAD's copy) and
   correct `docs/repository-health.md`.** Smallest safe change at HEAD: make
   `cleanup-bloat.sh` refuse and delegate — exit non-zero with a pointer to
   `safe-git-gc.sh` — rather than shipping the bare death command; then land the audited
   rewrite when its 10 audit gaps are addressed. Removes the repo's last instruction path
   that reproduces the Aug-14/16 kills on any clone.
2. **Land the alert-triage layer in git** (sweep, installer, test, the four unit files) and
   the `--auto-when-needed` flag in `safe-git-gc.sh`, so the running prevention state has a
   tracked source. Then create the **closure bead** the sweep's queue is waiting for:
   retire the 222 `RESOLVED_TARGET` alerts, starting with this chain's five — their
   disposition text is already converged (*crashes genuine, target closed 2026-08-17, work
   complete and pushed, alerts stale*).
3. **Fix `repo-health-monitor.sh`'s pack-count warning** (L-2): warn on `size-pack`/loose
   volume, not pack count.
4. **File the NEEDLE-side asks** (do not implement locally): retry stop-condition /
   work-completion detection at the alert source (G-9), dispatch-scope sizing for
   memory-heavy work (G-10), per the bf-1ea4g H-1 framing.
5. **Land the classifier capture-race fix** (domchk-3b605127 §3.2): trace-says-crash +
   silent `events.jsonl` → `INFRASTRUCTURE` + mechanism, never bare `UNKNOWN`.
6. **Keep the standing process rules** that make layers 6–8 effective: `bead show` the
   target before investigating any alert; `verify-work-completion.sh` before close; run
   `test-closed-bead-filter.sh` from the repo cwd only (cwd-sensitive premise).

## 7. Verification record (domchk-f7865662, this dispatch)

2026-09-07, worktree 110 files dirty (all citations above re-checked via
`git show HEAD:<path>`), first against HEAD `60f8a87`. Before commit, HEAD advanced
`60f8a87` → `e7f5b97` (sibling domchk-a47705c9's bf-mje3pd classification doc for alert
bf-3dxljn — docs-only, 2 files, no script or prevention-layer change); every load-bearing
citation was re-verified at `e7f5b97` before this commit: `cleanup-bloat.sh` still carries
the bare `git gc --aggressive --prune=now` (2 occurrences), `safe-git-gc.sh` still has no
`--auto-when-needed`, still zero alert-triage/auto-gc files tracked, the classifier's
`UNKNOWN` paths are unchanged, and `.gitignore` / `crash-prevention-requirements.md` /
`crash-mitigation-strategies.md` / `repository-health.md` are untouched by the advance.

- `du -sh .git` → 106M; `git count-objects -vH` → 313 loose / 2.15 MiB, 2 packs /
  99.78 MiB, 0 garbage.
- `scripts/setup-git-gc-config.sh --verify` → exit 0 (worst case ≈3072 MiB);
  `scripts/setup-git-hooks.sh --check` → exit 0 (byte-identical);
  `scripts/check-repo-health.sh` → exit 0, 0 unpushed, no unmanaged aggressive gc running.
- `systemctl --user list-timers 'domain-check-*'` → 8 timers, all future-triggered;
  `systemctl --user cat` on the alert-triage (hourly, report-only, `MemoryMax=512M`) and
  auto-gc (daily 02:30, `safe-git-gc.sh --auto-when-needed`, `MemoryMax=4G`) units.
- `.beads/logs/auto-gc.log` → 2026-09-07 02:30 run, "Auto mode: gc not needed";
  `.beads/logs/alert-triage.log` → 2026-09-07 20:01:06Z sweep, alerts=240 / resolved=222.
- `.beads/state/alert-triage/queue.jsonl` → all five bf-mje3pd alerts present with verdict
  `RESOLVED_TARGET`; `bead show` → all five Open.
- `git show HEAD:scripts/cleanup-bloat.sh` → 21-line bare `git gc --aggressive --prune=now`;
  `git show HEAD:docs/repository-health.md` → lines 80/113 instruct it;
  `git show HEAD:scripts/safe-git-gc.sh` → no `--auto-when-needed`;
  `git cat-file -e` → no alert-triage/auto-gc scripts or units tracked at HEAD.
- `scripts/test-crash-alert-fixes.sh` exit 0; `scripts/test-closed-bead-filter.sh` exit 0
  (both run from the repo cwd).
- `git show HEAD:scripts/crash-classifier.sh` → `UNKNOWN` at lines 20/403/532/584; exit-2
  path when trace not provenance-ok; repo-bloat corroboration present.
