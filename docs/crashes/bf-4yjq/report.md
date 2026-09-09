# bf-4yjq — Investigation Report

**Report dispatch:** domchk-52b232c5 ("Document investigation findings and mitigation plan"),
executed 2026-09-09 at HEAD `1ee3a23`. This file is that task's named deliverable at its named
path, alongside the sibling `analysis.md` (analyze leg domchk-ec2cbd03, commit `1ee3a23`).

**Relationship to canon:** the bf-4yjq chain's canonical synthesis is
[`docs/crashes/bf-4yjq-report.md`](../bf-4yjq-report.md) (report leg domchk-b6e16f96, commit
`ad0018c`), built on the gather leg (domchk-b06e87d4, commit `2b2456b`,
[`docs/crashes/bf-4yjq-evidence.md`](../bf-4yjq-evidence.md) §1–§9) and the classify leg
(domchk-7345947c, same file §10). This dispatch is a re-fire of the report leg against the
same task template; it derives **no new cause claim**. Its work is to restate the canon at
the task-named path in the task's required shape, and to **re-verify every load-bearing
figure first-hand** (Appendix A) rather than copy them.

**Subject bead:** `bf-4yjq` — *"Git origin remote points to GitHub directly; Forgejo mirror
has diverged/gone stale"* (P2, type `task`, **Closed** rev 2, 2026-08-17T00:14:14Z — re-read
live this dispatch).
**Workspace:** `/home/coding/domain-check`, worker `claude-code-glm-4.7-lab-domain-check`,
needle session `8446529e`, agent `claude-code-glm-4.7`.

---

## 1. Executive Summary

On 2026-08-12, needle dispatched bead `bf-4yjq` — a Forgejo-primary git remote reconciliation
task — **56 times in one evening**. **50 of the 56 runs were killed mid-task**, every one
recorded as `exit_code=-1` (`Crash(-1)`, the worker's no-wait-status sentinel), in a
fixed-cadence storm: kill→kill gap median **155.5 s** over a **9,404 s** window
(17:53:53.875Z → 20:30:38.310Z). All 50 crashed runs died at the same recorded step:
**`git push origin main`** — a full 50/50 transcript census, re-verified twice since the
gather leg and re-confirmed by this dispatch's census of the primary worker-log slot
(56 claims / 50 × `Crash(-1)` / 1 × `Failure(1)` / 4 × `Timeout(124)` / 1 × `Success(0)`).

**Classification: INFRASTRUCTURE — repository-bloat sub-type** (`docs/crash-response-guide.md`
framework; formally derived by the classify leg, re-checked here). At crash time the
repository was **18 GB** — 4,594 loose objects / 17.20 GiB against 9.60 MiB packed — the
bf-2ildm-era bloat of 17+ identical 237 MB `.beads/*.jsonl` snapshots. Over that store,
`git push`'s pack-objects is the memory-hungry process; the kills are the push-side memcg-OOM
regime later **kernel-proven** at bf-198ne (2026-08-16) and, for the gc-side variant, at
bf-4x12ec. For bf-4yjq itself the mechanism is **regime-matched, not kernel-proven**: the
Aug-12 kernel records are unrecoverable (the system journal's single boot `52309698` starts
2026-08-15 20:01:33 EDT — most recently re-verified live at the analyze leg).

Needle's per-kill alert minting amplified one undrainable repository condition into a
50-bead alert wave (one alert per kill, minted 7.0–12.6 s after each death; `bf-276uk` →
`bf-2n3ve`). **No work was lost**: the same-evening split child `bf-2xygo` completed the
remote work, and the target bead closed five days after the storm with a verification-bearing
close reason. The repository was repaired 2026-09-01 (18 GB → 92 MB) and holds at **106 MB**
live this dispatch with `check-repo-health.sh` exit 0 and the surge detector reporting **0
new crashes in the 24 h horizon** (the 50 on file are pre-horizon history). The alert-layer
false-positive surface (50 stale alerts against a Closed bead) is being drained by sibling
disposition dispatches: **36 closed / 12 open / 2 in progress** live this dispatch — zero
drift against the classify and analyze legs' counts.

## 2. Crash Metadata

| Field | Value | Source |
|---|---|---|
| Subject bead | `bf-4yjq`, P2, `task`, Closed rev 2 (2026-08-17T00:14:14Z) | live store, re-read this dispatch |
| Task | Reconcile git remotes to Forgejo-primary: merge diverged histories (no force-push), repoint `origin`, set up server-side push mirror, verify convergence | bead description |
| Session / agent | needle `8446529e` / `claude-code-glm-4.7` on `claude-code-glm-4.7-lab-domain-check` | worker-log span fields |
| Dispatch attempts | 56 `claim_auto` events, 17:50:23.048Z → 21:11:40.837Z | worker-log slot `.log.2`, re-counted live this dispatch |
| Deaths | **50 × `exit -1` `Crash(-1)`**, zero exit-code variation | re-counted live this dispatch (50/50) |
| Storm window | 2026-08-12T17:53:53.875Z → 20:30:38.310Z = **9,404 s** | canon; census-consistent |
| Kill cadence | gap min 75.6 s / **median 155.5 s** / mean 191.9 s / max 576.8 s (n=49) | canon (gather + classify + analyze legs agree) |
| Surge signature | 3 kills in one 300 s window (from 18:18:13Z); 5 in 600 s — meets `CRASH_SURGE_THRESHOLD=3` | canon §10.2 |
| Other outcomes | 1 × `Failure(1)` (18:00:17Z); 4 × `Timeout(124)` (20:40:47 → 21:11:27Z); 1 × `Success(0)` (21:14:56.748Z) | re-counted live this dispatch |
| Alert beads minted | 50, one per death, 7.0–12.6 s after each kill; first `bf-276uk`, last `bf-2n3ve` | canon paired ledger |
| Death point | `git push origin main` — final recorded Bash call in **50/50** crash transcripts | canon full transcript census |
| Repo at crash time | **18 GB** `.git`; 4,594 loose objects / 17.20 GiB vs 9.60 MiB packed; load 15–17 | contemporaneous snapshot |
| Repo this dispatch | **106 MB**; 118 loose / 784 KiB; 1 pack (12,607 objects / 100.70 MiB); 0 garbage | live: `git count-objects -vH`, `du -sh .git` |
| Exit-code semantics | `-1` is needle's **no-wait-status sentinel**, not a signal number | response-guide note, which uses bf-4yjq as its worked example |

## 3. Timeline of Events (2026-08-12, times UTC)

| Time | Event |
|---|---|
| 2026-07-20T13:59:43Z | Bead `bf-4yjq` created (queued for the August window) |
| 17:50:23.048Z | First `claim_auto` of the evening (attempt 1 of 56) |
| **17:53:53.875Z** | **Kill #1** — `exit -1`, mid-push; alert bead `bf-276uk` minted 8.8 s later |
| 17:53:53Z → 20:30:38Z | 50 kills at fixed cadence (median 155.5 s). Each retry re-runs the whole task and dies again at `git push origin main`; **one alert bead per kill** |
| 18:00:17Z | Attempt 2 ends `exit 1` (`Failure`) at `git add -A && git commit …` — workflow-class noise inside the storm, not a separate cause |
| 18:18:13Z | Densest surge: 3 kills in 300 s, 5 in 600 s (detector threshold met) |
| ~20:30–21:11Z | Tail: four `exit 124` dispatch-cap timeouts (attempts 52–55) |
| 21:12:00Z | Split child **`bf-2xygo`** created by the split-shaped attempt 56 |
| 21:14:56.748Z | Attempt 56 ends **`exit 0`** — it issued zero git operations (all split bookkeeping) and survived by *abandoning* the death operation |
| 21:30:57Z | `bf-2xygo` Closed — the remote reconciliation work completes the same evening |
| 2026-08-17T00:14:14Z | **`bf-4yjq` Closed, rev 2** — verbatim close reason records origin repointed to Forgejo, server-side push mirror working, remotes in sync at `a245b38`, mirror last synced 2026-08-17T00:11:34Z |
| 2026-09-01 | Repository repaired: 18 GB → 92 MB (verified; 106 MB this dispatch) |

## 4. Root-Cause Analysis

The full derivation is **[`analysis.md`](analysis.md)** in this directory (analyze leg
domchk-ec2cbd03) — the tasked root-cause document this report references. Its verdict,
re-checked by this dispatch rather than restated on faith:

**Mechanism chain:** repository bloat (bf-2ildm's 17+ identical 237 MB `.beads/*.jsonl`
snapshots → 18 GB store, inverted loose:packed ratio) → each re-dispatch re-ran the
remote-reconciliation task to its terminal `git push origin main` step, whose pack-objects is
the memory-hungry process over that store → the worker runs inside a systemd dispatch scope
capped at 12 GiB → memory-cgroup OOM kill (`CONSTRAINT_MEMCG`, uncatchable SIGKILL) → needle
records its no-wait-status sentinel `exit_code=-1`, releases the bead, mints an alert →
re-dispatch into the unchanged environment → repeat 50 times.

Provenance qualifiers, stated the way the canon states them:

- **`-1` is a sentinel, not a signal number.** No signal is asserted from it.
- **Regime-matched, not kernel-proven, for Aug-12 specifically.** The journal floor (single
  boot, first entry 2026-08-15 20:01:33 EDT) makes the Aug-12 kernel records unrecoverable;
  the mechanism is pinned by uniform per-run evidence (50/50 death command, fixed cadence,
  the contemporaneously measured 18 GB store) plus the kernel-proven siblings — bf-198ne
  (push-side, 2026-08-16) and bf-4x12ec (gc-side).
- **Amplifier, not cause:** needle's one-alert-per-kill loop converted a single undrainable
  repository condition into 50 alert beads. The alert layer is where this event's false
  positives live — the crashes themselves were real (§5).

This dispatch re-verified the mechanism's two cheapest load-bearing inputs live: the
exit-code census from the primary worker-log slot (56 / 50 / 1 / 4 / 1 — byte-identical to
canon) and the present-day absence of the precondition (`.git` 106 MB, 0 garbage, health
gate exit 0). It takes no position beyond the canon's.

## 5. Classification (crash-response-guide framework)

**Primary classification: INFRASTRUCTURE — repository-bloat sub-type.** Derived via
[`docs/crash-response-guide.md`](../../crash-response-guide.md): exit −1 → infrastructure;
fixed cadence + `.git` > 5 GB → repository-bloat row. Response-guide **Pattern 3** signature
checklist: **6/6** (zero exit-code variation; fixed cadence; repo 36× over the 5 GB
threshold; loose objects 17.20 GiB with inverted ratio; routine git operation is the death
point 50/50; 50 crashes in 2 h 37 m).

**False-positive checks** (guide heuristics, formally derived in the evidence file §10.3):

- **Rule 1 (post-completion death): negative — genuinely mid-task.** The last substantive
  command in all 50 crash transcripts is the task's own push step; no retry ran against
  already-completed work.
- **Rule 2 (crash → retry → success = self-healed): surface match only — the caveat bites.**
  The exit-0 attempt issued zero git operations and survived by task-shape change
  (abandoning the death operation) while the store was still 18 GB. The storm is **not**
  recorded as self-healed.
- **Rule 3 (system-wide event): positive.** One environmental regime shared with bf-1s6c3's
  same-evening storm; surge math meets the committed detector's threshold.

**Alert-layer false positives: yes.** All 50 alert beads target a bead Closed 2026-08-17 —
stale by closure. That is a statement about the alerts, not the crashes; per
[`docs/crash-response-guide.md`](../../crash-response-guide.md) Runbook A they retire by
target resolution + dedup gate, not by re-investigation.

## 6. Impact Assessment

| Dimension | Assessment |
|---|---|
| **Work lost** | **None.** The deliverable (remote reconciliation) landed via same-evening split child `bf-2xygo` (Closed 21:30:57Z); the target bead closed 2026-08-17 rev 2 with a verification-bearing close reason re-read live |
| **Data affected** | None corrupted. The deaths are SIGKILL-class kills of git processes; `git fsck --full` on the repaired repo is clean, and the pushed result (`a245b38`) verified in sync on both remotes at close time |
| **Compute cost** | 56 dispatch attempts (~3 h 20 m of dispatch churn) and 50 wasted worker runs for one completable task |
| **Alert-layer cost** | 50 alert beads minted, each nominally requiring disposition; 36 closed to date, **12 Open + 2 InProgress** (`bf-2j99a`, `bf-vcsxj`) live this dispatch — owned by their own dispatches, not actioned here |
| **Documentation cost** | The storm generated a large stale-doc surface: several 2026-09-01-era reports carry superseded figures ("9 crashes", SIGKILL-stated-as-fact, "no session evidence"). The supersession table lives in the evidence file §8; observe it when quoting older docs |
| **Code impact** | **Zero.** No domain-check code defect — no panic, stack trace, core dump, or application error anywhere in the chain, consistent with the corpus-wide zero-defect finding |

## 7. Investigation Methods and Sources

| Source | Path | Used for |
|---|---|---|
| Primary worker-log slot | `~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2` (134 MB, 225 `bf-4yjq` records — still present, one rotation from erasure) | **re-census this dispatch**: 56 claims / 50 × `Crash(-1)` / 1 × `Failure` / 4 × `Timeout` / 1 × `Success` |
| Full chain canon | `docs/crashes/bf-4yjq-evidence.md` (gather + classify), `docs/crashes/bf-4yjq-report.md` (report leg), `analysis.md` (analyze leg) | timeline, cadence, surge windows, death-point census, classification |
| Sibling analysis | `analysis.md` (domchk-ec2cbd03, HEAD `af94e85`) | root-cause derivation this report references |
| Repair record | `docs/crashes/bf-4yjq-cleanup-verification.md` | 18 GB → 92 MB repair and re-verification history |
| Live store | `bead show bf-4yjq`; `bead list --json` census | target Closed rev 2; alert pool 36/12/2 re-counted this dispatch |
| Live health gates | `git count-objects -vH`, `du -sh .git`, `check-repo-health.sh`, `crash-pattern-detection.sh` | "now" rows re-read this dispatch |

Method note: where this dispatch re-derived a figure (Appendix A), the agreement with the
prior legs *is* the verification; where it cites canon without re-deriving (cadence table,
surge windows, per-attempt transcript detail), the figure has already been re-derived
first-hand by at least two independent prior dispatches at different HEADs.

## 8. Findings

- **F1 — Real, mid-task, single-caused.** 50 genuine infrastructure deaths of one bead in
  2 h 37 m, zero exit-code variation, one shared environmental precondition (the 18 GB store).
- **F2 — The death operation was `git push`, uniformly** (50/50 transcripts). The Aug-12
  storm was push-side — the mechanism kernel-proven at bf-198ne — not gc-side.
- **F3 — Needle's re-dispatch loop was the amplifier** (response-guide H-1 residual): median
  155.5 s re-claim per kill, one alert bead per kill. Alerts scaled with kills, not with the
  single cause.
- **F4 — No work lost; no data affected.** Task completed via split child; bead closed with
  verification. Crash exposure was scheduling inside the bloat window, not task content.
- **F5 — The exit-0 attempt is not a self-heal** — it abandoned the death operation while the
  store was still 18 GB.
- **F6 — The alert layer was stale by closure and is draining** (36/12/2 live, zero drift
  across three same-day censuses).
- **F7 — The regime is gone and defended in depth** (§10): repo 106 MB, `.beads/` fully
  gitignored, pack-memory bounds in force, daily/weekly bounded-gc timers, surge detector
  reporting 0 new crashes in its 24 h horizon this dispatch.

## 9. Mitigation Recommendations

The chain's tasked mitigation record is
[`docs/crash-mitigation-strategies.md`](../../crash-mitigation-strategies.md) — **v2.4
(2026-09-09, recommend leg domchk-834ded2c, commit `af94e85`) already corrected its own
bf-4yjq figures to this canon** (50 kills / 56 dispatches, `-1` sentinel, regime-matched
mechanism), superseding the 2026-09-01-era "9 OOM crashes" count. This report endorses that
document and records the mapping from this crash to its priorities:

| bf-4yjq failure factor | Mitigation priority | Status |
|---|---|---|
| Unbounded store growth via committed bead state | Priority 3 — repository bloat prevention (gitignore + pre-commit size gate) | **Landed** — `.beads/` fully gitignored (0 tracked files); 10 MB pre-commit hook shipped as a tracked, per-clone-installable script (`dfa60a9`) |
| Memory-hungry pack-objects inside the 12 GiB dispatch scope | Priority 4 — Git GC operation safety (bounded git memory, covering the bare path too) | **Landed** — `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1`, applied repo-locally **and** globally so bare `git gc`/`git push` are bounded too; worst case ≈3 GiB |
| Kills invisible until the storm was over | Priority 5 — Monitoring and alerting | **Landed** — daily repo-health + bounded-gc systemd user timers; crash-pattern surge detector |
| Re-dispatch into the same crash (amplifier) | Storm gates — circuit breaker + concurrency limiter + surge gate at dispatch entry | **Landed** — `needle-with-limiter.sh` gate, `system-event-mode.sh` exit-75 defer, wired into preflight |
| 50 stale alerts against a Closed bead | Alert lifecycle — target-resolution + dedup gate before investigating | **In progress** — the alert layer's FIX 1/5 closed-bead gate and dedup window are in force; the 12 Open + 2 InProgress legacy beads belong to their own disposition dispatches (Runbook A), explicitly not actioned by this report |

**New recommendation from this dispatch: none.** Every mitigation this crash motivates is
already landed and validated (§10); introducing a new one here would duplicate the recommend
leg's v2.4 record.

## 10. Prevention Strategies (in force)

Per the response guide's prevention map, the layers that structurally rule out a bf-4yjq
recurrence — each claim only as current as its last validation
([`docs/crash-prevention-validation.md`](../../crash-prevention-validation.md); full battery
all-green 2026-09-08; this dispatch re-ran the two cheapest live gates, Appendix A):

1. **`.beads/` wholly gitignored + repo-wide `*.jsonl` rule** — the growth vector bf-2ildm
   used cannot re-enter history (0 tracked `.beads` files).
2. **10 MB pre-commit size gate** — the backstop that would have blocked the 237 MB
   snapshots; installed per clone via `setup-git-hooks.sh` (tracked canonical source).
3. **Persistent pack-memory bounds** — cover *both* death operations of this era: bare
   `git gc` and `git push` (`setup-git-gc-config.sh --verify` resolves the effective bound;
   ≈3 GiB worst case vs the 12 GiB scope).
4. **Daily repo-health + bounded-gc timers, weekly full gc** — bloat is caught at the 1 GB
   warning threshold, not at OOM.
5. **Crash surge detector + dispatch-entry storm gates** — a fixed-cadence wave now defers
   dispatches (exit 75) and opens the breaker instead of re-entering the crash 50 times.
6. **Alert-layer closed-bead + dedup gates** — a future kill wave against a bead that closes
   mid-storm no longer mints one investigation per kill.

## 11. Crash Classification Statistics

**Classification record:** INFRASTRUCTURE / repository-bloat sub-type — already carried by
the canon surfaces (evidence file §10 classification section; canon report §4; response-guide
Quick Reference row for repository bloat, which this storm exemplifies). No separate
aggregate statistics table exists in the response guide to update, and this dispatch records
**no classification change**.

**Not actioned here:** the crash-documentation index
(`docs/crashes/crash-documentation-index-2026-09-02.md`) does not yet list the 2026-09-09
chain artifacts (the canon report, `analysis.md`, or this file). That file is under active
concurrent edit by sibling workers (staged *and* unstaged changes in the shared worktree);
appending to it from this dispatch would interleave hunks with an in-flight sibling edit and
sweep their uncommitted work into this leg's commit. The gap is recorded; index maintenance
belongs to that file's own maintainers.

## 12. Conclusion

`bf-4yjq`'s 50 kills were one infrastructure event — a repository-bloat regime in which every
re-dispatch of the same git task died at the same `git push origin main` step for 2 h 37 m,
while the alert layer minted one stale alert per kill. The classification
(INFRASTRUCTURE / repository-bloat) is HIGH confidence; the push-side memcg-OOM mechanism is
regime-matched rather than kernel-proven for this bead only because the Aug-12 kernel records
no longer exist, with kernel-proven siblings at the same death point. Domain-check code was
not implicated anywhere in the chain. Work survived intact, the repository was repaired and
holds healthy, the regime cannot recur through `.beads/`, and the monitoring and storm-gate
layers that would have truncated this event are installed and green.

**Disposition for this dispatch:** documentation complete at the task-named path; no new
cause claim, no classification change, no new mitigation. The 12 Open + 2 InProgress alert
beads remain owned by their own disposition dispatches.

## 13. Related Crashes

| Bead | What happened | Relation to bf-4yjq | Canonical record |
|---|---|---|---|
| **bf-1s6c3** | 2026-08-12, same evening: 76 dispatches / 71 kills | Same 18 GB store, same regime — Rule 3's "one environmental regime, two beads' worth of kills" | `docs/crash-analysis-bf-1s6c3-2026-09-06.md` (§12 = dedup-append target) |
| **bf-173o7e** | 2026-08-14: 132 dispatches / 129 × `exit −1` over ~10.5 h | Same era's gc-side storm; the template chain this report belongs to parallels its (fully closed) chain | `docs/crash-investigations/bf-173o7e-crash-context-domchk-114d6172-2026-09-08.md` |
| **bf-198ne** | 2026-08-16 push-side memcg OOM, kernel-proven | The same push-side mechanism as bf-4yjq's storm, with surviving kernel records | `docs/crashes/bf-198ne-crash-report.md` |
| **bf-4x12ec** | memcg OOM of bare `git gc --aggressive` in the 12 GiB scope, kernel-proven | The gc-side kernel proof for the regime bf-4yjq ran inside | `docs/crash-reports/bf-4x12ec-git-gc-crash.md` |
| **bf-2ildm** | Committed the 17+ identical 237 MB `.beads/*.jsonl` snapshots | The *cause* of the bloat bf-4yjq's pushes choked on | `docs/investigations/bf-2ildm-crash-pattern-analysis-domchk-579d25b0-2026-09-07.md` |

---

## Appendix A — Figures re-verified live by this dispatch (2026-09-09, HEAD `1ee3a23`)

| Figure | Canon value | This dispatch | Agreement |
|---|---|---|---|
| Worker-log claims | 56 | 56 | ✅ |
| `Crash(-1)` deaths | 50 | 50 | ✅ |
| Other outcomes | 1 `Failure(1)` / 4 `Timeout(124)` / 1 `Success(0)` | 1 / 4 / 1 | ✅ |
| Target bead | Closed rev 2, 2026-08-17T00:14:14Z | same, re-read live | ✅ |
| Alert pool | 36 Closed / 12 Open / 2 InProgress | 36 / 12 / 2 (open: `bf-1dzwv`, `bf-1fvk2`, `bf-1jxy8`, `bf-22514`, `bf-2dj1g`, `bf-2ftau`, `bf-35bhc`, `bf-47ugw`, `bf-4tnae`, `bf-5egrf`, `bf-hw4i5`, `bf-mlv3u`; in progress: `bf-2j99a`, `bf-vcsxj`) | ✅ zero drift across three same-day censuses |
| `.git` size | 106 MB (at the analyze leg) | 106 MB | ✅ |
| Loose objects | 109 / 736 KiB (analyze leg) | 118 / 784 KiB | ✅ normal churn, healthy band |
| Pack | 1 pack, 12,607 objects | 1 pack, 12,607 objects / 100.70 MiB | ✅ |
| Garbage | 0 | 0 | ✅ |
| `check-repo-health.sh` | exit 0 | exit 0 | ✅ |
| Surge detector | STABLE, 0 crashes/24 h | 0 new in the 24 h horizon; 50 on file as pre-horizon history | ✅ |

Figures cited from canon without re-derivation here (cadence min/median/mean/max, surge
windows, alert-pairing offsets, per-attempt transcript detail, journal floor) were each
re-derived first-hand by at least two independent prior dispatches (gather `2b2456b`, classify
`e4c30d0`, report `ad0018c`, analyze `1ee3a23`) with identical results.

---

## Appendix B — Mitigation verification (implement-and-verify leg, 2026-09-09)

**Dispatch:** `domchk-1a78af71` ("Implement and verify crash mitigation" — this chain's
implement/verify leg), executed 2026-09-09 at HEAD `edee672`, load 3.7–5.1. Scope: the full
prevention battery of
[`docs/crash-prevention-validation.md`](../../crash-prevention-validation.md) — **13 live
checks + 19 tracked suites**, re-run first-hand, **all green, first pass** (dated row
appended to that doc's verification record). The §9 table's "Landed" statuses are re-verified
live below rather than cited; **zero functional changes** were made — nothing new to build,
per §9's own "new recommendation: none".

**§9's mitigation mapping, re-verified live this leg:**

| §9 priority | Live evidence (2026-09-09, this leg) |
|---|---|
| Repository-bloat prevention (gitignore + pre-commit gate) | `git ls-files .beads` → **0**; `.gitignore` still carries `.beads/`, `*.db`, `*.jsonl`; `setup-git-hooks.sh --check` rc 0 ("byte-identical to tracked source") |
| Bounded pack memory (covers the death operation) | `setup-git-gc-config.sh --verify` rc 0 — effective bound resolves system→global→local with **all three keys supplied repo-locally**: `windowMemory=2g`, `deltaCacheSize=1g`, `threads=1` → worst case **≈3072 MiB** against the 12 GiB scope below |
| Monitoring and alerting | **8/8** `domain-check-*` timers future-triggered; `service-monitor.sh --once` "PRE-FLIGHT CHECK PASSED"; `resource-monitor.sh --once` pressure 0% / UNSAFE_GC none; `crash-pattern-detection.sh` **STABLE — 0 new crashes in the 24 h horizon** (the 50 on file are pre-horizon history) |
| Storm gates (the re-dispatch amplifier) | `system-event-mode.sh check` rc 0 (`clear`, PSI 0.00%); `crash-circuit-breaker.sh status` → `{"beads": {}}`; `preflight-health-check.sh` **5/5**, rc 0 |
| Alert lifecycle (closed-bead + dedup gates) | `test-crash-alert-fixes.sh`, `test-closed-bead-filter.sh` (repo cwd), `test-alert-dedup-check.sh`, `test-alert-dedup-history.sh`, `test-alert-triage-sweep.sh` — all pass |

**Root-cause confirmation (the recurrence drill):**

- **Precondition absent.** `.git` **106 MB** vs the crash-time 18 GB (~170× smaller);
  `git count-objects -vH`: 127 loose / 840 KiB vs **1 pack** (12,607 objects / 100.70 MiB),
  0 garbage; `check-repo-health.sh` rc 0; `auto-gc-trigger.sh --dry-run` "GC not needed";
  unpushed backlog CLEAR (0 < 50).
- **Death operation now bounded.** `test-gc-memory-bounds.sh` **17/17** re-runs this crash's
  exact death operation (unbounded `git push` → pack-objects) inside a 768 MiB cgroup:
  push peak RSS **232,480 KB**, pack-objects peak **320,536 KB** — both under the 700 MiB
  assertion cap and ~40× under the dispatch scope, **re-read live this leg at
  `memory.max = 12,884,901,888` B (12 GiB)** from this dispatch's own cgroup.
  `test-safe-git-gc-limits.sh` **33/33**.
- **Amplifier gated.** The bf-4yjq shape — re-claim every ~155.5 s into an unchanged
  environment — now meets the surge detector's threshold and defers at dispatch entry
  (event-mode exit 75 / breaker exit 4) instead of re-entering the crash 50 times; live
  state clear with nothing latched.

**Criterion disposition — "close bead bf-4yjq as resolved: mitigated": already satisfied by
prior state.** `bf-4yjq` is **Closed rev 2** (2026-08-17T00:14:14Z; re-read live this leg via
`bead show`, close reason re-read from the checkpoint), with a verification-bearing close
reason, verbatim: *"Git remote configuration successfully fixed and verified. Origin now
points to Forgejo (git.ardenone.com), GitHub mirror is working via server-side push mirror,
both repositories are in sync (a245b38), and push mirror last synced successfully at
2026-08-17T00:11:34Z with no errors."* The bead is terminal-closed — `bead close` is not a
valid transition from `closed` — so this leg performs **no bead mutation** and re-opening a
resolved bead solely to re-close it would be churn. The "resolved" state this criterion asks
for is the one already recorded above; the mitigation evidence backing it is this appendix
plus the battery row in
[`docs/crash-prevention-validation.md`](../../crash-prevention-validation.md).

**Quality-gate attribution (docs-only leg):** worktree `go build ./...` / `go test ./...`
fail only inside a co-tenant's uncommitted `internal/watch/manager.go` edit (`domain.Parse`
undefined; its importers `internal/server` and `cmd/domain-check` fail on the same file) —
every other package `ok`. No Go file is touched by this leg.

**Box-state note (not prevention):** free disk read 19–20 GB at battery time (the [CRITICAL]
threshold); three stale regenerable `~/scratch/*-target` build dirs (≥20 h old, zero open
handles, no cargo/rustc running) were cleared per the standing procedure → 22 GB. The repo
and every prevention layer were unaffected.
