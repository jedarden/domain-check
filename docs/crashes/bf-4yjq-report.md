# bf-4yjq — Crash Report

**Report leg:** domchk-b6e16f96 ("Document investigation findings in crash report"), executed
2026-09-09 at HEAD `e4c30d0`. Consolidates the chain's two evidence legs — **gather**
(domchk-b06e87d4, `docs/crashes/bf-4yjq-evidence.md` §1–§9) and **classify** (domchk-7345947c,
same file §10) — into the chain's tasked full report.
**Subject bead:** `bf-4yjq` — *"Git origin remote points to GitHub directly; Forgejo mirror has
diverged/gone stale"* (P2, type `task`, **Closed** rev 2, 2026-08-17T00:14:14Z).
**Workspace:** `/home/coding/domain-check`, worker `claude-code-glm-4.7-lab-domain-check`,
needle session `8446529e`, agent `claude-code-glm-4.7`.

Every load-bearing figure in this report was **re-derived first-hand at this dispatch** from the
primary sources listed in §5 — not copied from the prior legs. Where this dispatch's numbers
match theirs, that agreement is itself the verification; where live state has drifted, the
drift is recorded as such.

---

## 1. Executive Summary

On 2026-08-12, needle dispatched bead `bf-4yjq` (a Forgejo-primary git remote reconciliation
task) **56 times in 3 h 24 m**. **50 of the 56 runs were killed mid-task** — every one recorded
as `exit_code=-1` (`Crash(-1)`, the worker's no-wait-status sentinel) — in a fixed-cadence
storm: kill→kill gap median **155.5 s**, over a **9,404 s** window (17:53:53.875Z →
20:30:38.310Z). All 50 crashed runs died at the same recorded step: **`git push origin main`**
(full 50/50 transcript census, re-run this dispatch). The remaining six runs were the storm's
tail: one `exit 1` workflow failure, four `exit 124` dispatch-cap timeouts, and one `exit 0`
success that survived by *abandoning* the death operation (it issued zero git commands) — not
because the environment had improved.

**Classification: INFRASTRUCTURE — repository-bloat sub-type** (the `docs/crash-response-guide.md`
framework, applied formally by the classify leg and re-checked here). At crash time the
repository was **18 GB** — 4,594 loose objects / 17.20 GiB against 9.60 MiB packed — the
bf-2ildm-era bloat of 17+ identical 237 MB `.beads/*.jsonl` snapshots. With the store in that
state, `git push`'s pack-objects is the memory-hungry process, and the kills are the push-side
memcg-OOM regime later **kernel-proven** at bf-198ne (2026-08-16) and, for the gc-side variant,
at bf-4x12ec. For bf-4yjq itself the mechanism is **regime-matched, not kernel-proven**: the
Aug-12 kernel records are unrecoverable (the system journal's single boot starts
2026-08-15 20:01:33 EDT — re-verified live this dispatch).

The needle layer amplified one undrainable repository condition into a storm: **one alert bead
was minted per kill (50/50, paired 7–13 s after each death; first `bf-276uk`, last `bf-2n3ve`)**,
each re-dispatch landing a median 155.5 s after the last kill. No work was lost — the task was
completed and the bead closed five days later (2026-08-17), after a same-evening split child
(`bf-2xygo`) finished the remote work. The repository was repaired 2026-09-01 (18 GB → 92 MB,
re-verified repeatedly since; **106 MB / 95 loose objects / 0 garbage live this dispatch**) and
the crash surge detector reports **STABLE, 0 crashes/24 h**. The 50 alert beads are stale by
closure and are being dispositioned by sibling workers (live census this dispatch: **36 closed /
12 open / 2 in progress**).

---

## 2. Crash Metadata

| Field | Value | Source (all re-derived 2026-09-09) |
|---|---|---|
| Subject bead | `bf-4yjq`, P2, `task`, labels `deferred`, `umbrella`; blocks `bf-1h6rk` | checkpoint issue snapshot + live store |
| Task | Reconcile git remotes to Forgejo-primary: merge diverged histories (no force-push), repoint `origin`, set up server-side push mirror, verify convergence | bead description |
| Needle session / agent | `8446529e` / `claude-code-glm-4.7` on worker `claude-code-glm-4.7-lab-domain-check` | every log record's span fields |
| Dispatch attempts | **56** `claim_auto` events, 17:50:23.048Z → 21:11:40.837Z | worker-log slot `.log.2`, live re-count |
| Deaths | **50 × `exit_code=-1` `outcome=Crash(-1)`** — zero exit-code variation | live re-count: 50/50 |
| Storm window | **2026-08-12T17:53:53.875Z → 20:30:38.310Z = 9,404 s** (2 h 36 m 44 s) | computed from the 50 kill instants |
| Kill cadence | gap min **75.6 s** / **median 155.5 s** / mean **191.9 s** / max **576.8 s** (n=49) | computed this dispatch from the same instants |
| Surge signature | **3 kills in one 300 s window (from 18:18:13Z); 5 in 600 s** — meets `CRASH_SURGE_THRESHOLD=3` | recomputed this dispatch |
| Other outcomes | 1 × `exit 1` `Failure` (18:00:17Z); 4 × `exit 124` `Timeout` (20:40:47 / 20:51:01 / 21:01:14 / 21:11:27Z); 1 × `exit 0` `Success` (21:14:56.748Z) | live re-count |
| Alert beads minted | **50**, one per death, created **7.0–12.6 s** after each kill; first `bf-276uk`, last `bf-2n3ve` | pairing re-derived this dispatch, 50/50 order-monotonic |
| Death point | **`git push origin main`** — final recorded Bash call in **50/50** crash transcripts | full census re-run this dispatch on the extracted bundle |
| Repo at crash time | **18 GB** `.git`; 4,594 loose objects / **17.20 GiB** vs 9.60 MiB packed; load 15–17 | contemporaneous snapshot (repo figures; its crash count is superseded — §8) |
| Repo now | **106 MB**; 95 loose / 640 KiB; 1 pack 100.70 MiB; 0 garbage; `check-repo-health.sh` exit 0 | live this dispatch |
| Exit-code semantics | `-1` is needle's **no-wait-status sentinel**, not a signal number | guide note 2, which uses bf-4yjq as its named example |

---

## 3. Timeline and Events

### 3.1 The storm (2026-08-12, all times UTC)

| Time | Event |
|---|---|
| 2026-07-20T13:59:43Z | Bead `bf-4yjq` created (deferred until the August queue reaches it) |
| 17:50:23.048Z | First `claim_auto` of the evening (attempt 1 of 56) |
| **17:53:53.875Z** | **Kill #1** — `exit -1`, mid-push. Alert bead `bf-276uk` minted ~seconds later |
| 17:53:53Z → 20:30:38Z | 50 kills at fixed cadence (median 155.5 s). Each retry re-runs the task from the top and dies again at `git push origin main`. **One alert bead per kill**, minted 7.0–12.6 s after each death |
| 18:00:17Z | Attempt 2 ends `exit 1` (`Failure`) at `git add -A && git commit …` — workflow-class noise inside the storm, not a separate cause |
| 18:18:13Z | Densest surge point: **3 kills inside 300 s** (detector threshold met); **5 inside 600 s** |
| ~20:30–21:11Z | Tail: four `exit 124` dispatch-cap timeouts (attempts 52–55; two show tool activity, two show none) |
| 21:12:00Z | Split child **`bf-2xygo`** created by the split-shaped attempt 56 |
| 21:14:56.748Z | Attempt 56 ends **`exit 0`** — it issued **zero git operations** (its 14 substantive commands are all `bf`-CLI split bookkeeping) and survived by abandoning the death operation |
| 21:30:57Z | `bf-2xygo` Closed — the remote reconciliation *work itself* completes the same evening |
| 2026-08-17T00:14:14Z | **`bf-4yjq` Closed, rev 2.** Verbatim close reason (re-read live from the checkpoint): "Git remote configuration successfully fixed and verified. Origin now points to Forgejo (git.ardenone.com), GitHub mirror is working via server-side push mirror, both repositories are in sync (a245b38), and push mirror last synced successfully at 2026-08-17T00:11:34Z with no errors." |
| 2026-09-01 | Repository repaired: 18 GB → 92 MB (verified; 106 MB this dispatch) |

### 3.2 What each retry was doing

Every retry re-ran the whole Forgejo reconciliation task and died at the same terminal step.
Per-run git activity before death: 7–32 git commands (mean 15.6); 18 of the 50 crashed runs
show a `git commit` earlier in the session, 32 pushed with no prior commit call. The bead's own
deliverable (the reconciliation) landed only after the *task shape changed* — the split child —
which is why the storm read is "mid-task kills, zero work lost," not "post-completion deaths."

---

## 4. Classification Analysis

**Primary classification: INFRASTRUCTURE — repository-bloat sub-type.** Derived via the
`docs/crash-response-guide.md` framework (classify leg domchk-7345947c, re-checked this
dispatch):

**Exit-code census (n=56 claims, re-derived live):**

| Outcome | Count | Guide row | Disposition |
|---|---|---|---|
| `exit -1` `Crash(-1)` | **50** | −1 → Infrastructure; fixed-cadence + `.git` > 5 GB → **Repository bloat** | the storm |
| `exit 124` `Timeout` | 4 | Workflow: dispatch-cap timeout | storm tail; 2 with tool activity, 2 without |
| `exit 1` `Failure` | 1 | not `error_max_turns`, not HTTP 5xx | workflow-class noise (died at a commit, not a push) |
| `exit 0` `Success` | 1 | — | terminal; survived by task-shape change — see Rule 2 below |

**Rationale:**

1. **Row match.** 50/56 claims died `exit -1` with zero variation; the guide's sentinel note
   names bf-4yjq as its worked example.
2. **Sub-type.** Fixed cadence (median 155.5 s) + 18 GB `.git` satisfies the repository-bloat
   row verbatim — the sub-type that recurs on every re-dispatch until the repo is cleaned.
3. **Not SERVICE_FAILURE.** No HTTP 503/502 signature anywhere in the surviving telemetry; the
   guide's exit-1 service row never occurs.
4. **Not CODE_DEFECT.** No stack trace, no core dump, no application error output anywhere in
   the chain (none exist for SIGKILL-class deaths; `coredumpctl`'s earliest entry is
   2026-08-17, unrelated). The killed work is a git operation, not domain-check code —
   consistent with the corpus-wide zero-defect finding.
5. **Pattern 3 signature: 6/6** — zero exit-code variation ✅; fixed cadence ✅; repo > 5 GB
   (18 GB, 36× the threshold) ✅; loose objects > 1 GB (4,594 / 17.20 GiB, inverted ratio) ✅;
   routine git operation triggers the deaths (50/50 at push) ✅; multiple crashes in a short
   period (50 in 2 h 37 m) ✅.
6. **Rule 3 (system-wide event): positive.** The surge math meets the committed detector's
   threshold (3/300 s, 5/600 s). One environmental regime, shared with bf-1s6c3's same-evening
   storm — not 50 independent task failures. Runbook F triage (stop load, fix the environment)
   is what eventually ended it.

**False-positive checks:**

- **Rule 1 (post-completion death): negative — genuinely mid-task.** The last substantive
  command in every crash transcript is the task's own push step; the remote reconciliation was
  not satisfied anywhere in the window, so no retry ran against already-completed work.
- **Rule 2 (crash → retry → success = self-healed): surface match only, and the caveat bites.**
  The exit-0 attempt issued **zero git operations** — it survived by abandoning the death
  operation (task-shape change), while the store was still 18 GB. Per the decision tree's
  exit-0-after-storm branch, the storm is **not** recorded as self-healed.
- **Alert-layer false positives: yes — the 50 alerts target a bead that Closed 2026-08-17.**
  They are stale by closure; that is a statement about the alerts, not the crashes. The kills
  were real.

**Work completion:** no marker exists (`.beads/state/work-completion/` postdates the storm);
the checklist applies — target bead **Closed rev 2** with a verbatim close reason recording
completed verification, five days *after* the storm, via split child `bf-2xygo` (present in the
live store, re-read this dispatch). Mid-task kills + eventual completion = genuine
infrastructure crash, **zero work lost**.

**Confidence: HIGH on the classification; MODERATE-HIGH on the mechanism.** The mechanism is
regime-matched, not kernel-proven for this bead: the Aug-12 kernel records are unrecoverable
(journal floor re-verified live this dispatch — single boot `52309698`, first entry
2026-08-15 20:01:33 EDT), and no signal is asserted from the `-1` sentinel. The regime match is
strong: uniform death point 50/50, the 18 GB precondition measured contemporaneously, and
sibling storms of the same era and repo kernel-proven (bf-198ne push-side, bf-4x12ec gc-side).
Residual caveat: a transcript's last recorded tool call is not proof of the killed process (the
agent is not sampled between calls) — but 50/50 uniformity on the identical command makes an
alternative death point implausible.

---

## 5. Investigation Methods

Primary sources, with this dispatch's re-derivation commands:

| Source | Path | Used for |
|---|---|---|
| Needle plaintext worker log | `~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2` (225 `bf-4yjq` records, coverage 2026-08-11 → 08-15; still one rotation from erasure) | kill ledger, cadence, surge windows, alert-bead pairing (`grep 'bf-4yjq' … \| grep 'outcome=Crash(-1)'`) |
| Needle structured events | committed copy `docs/crash/bf-4yjq/raw-logs/needle-events-2026-08-12-bf-4yjq.jsonl` (1,071 records: 56 claims, 56 outcomes, 454 heartbeats) | claim/outcome census |
| **All 56 per-run session transcripts** | `docs/crash/bf-4yjq/raw-logs/bf-4yjq-crash-sessions-2026-08-12.tar.gz` (+ `sessions-index.tsv`, `MANIFEST.sha256`) | death-point census — extracted and re-run 50/56 this dispatch. This corrects pre-2026-09-06 reports claiming no per-run evidence survives |
| Contemporaneous system snapshot | `docs/crash-analysis/bf-4yjq-system-state-snapshot-2026-09-01.txt` | repo 18 GB / loose 17.20 GiB / load / disk (repo figures only — its crash count is superseded) |
| Bead state + close reason | `.beads/checkpoint/forensic.jsonl` + live store | timeline, verbatim close reason, alert-pool census (50/50 IDs re-paired and status-counted live) |
| Kernel record | `journalctl --list-boots` | Aug-12 absence: single boot, floor 2026-08-15 20:01:33 EDT |
| Current health | `git count-objects -vH`, `du -sh .git`, `./scripts/check-repo-health.sh`, `./scripts/crash-pattern-detection.sh` | "now" rows: 106 MB / 95 loose / 0 garbage / exit 0 / STABLE |

Method notes: alert beads were paired to kills by the committed 7.0–12.6 s offset rule and the
50 pairs proved order-monotonic with no interleaving; the death-point claim was re-verified as
a **full census** of the extracted transcripts (parse each session's last `Bash` tool_use
block), not a spot-check; every "now" value was read live this dispatch.

**Confirmed absences (bounded the investigation):** no stack traces/core dumps (none exist for
SIGKILL-class deaths); no Aug-12 kernel records (journal floor); no separate stderr capture for
the era (the transcripts' truncation point *is* the death record — transcript-last-event sits
0–23 s before `outcome.classified`); no `.beads/traces/bf-4yjq/` (predates trace capture); no
`.beads/logs/*` monitoring files (monitoring layer starts 2026-09-01).

---

## 6. Findings

- **F1 — The storm was real, mid-task, and single-caused.** 50 genuine infrastructure deaths of
  one bead in 2 h 37 m, zero exit-code variation, one shared environmental precondition (the
  18 GB store). Not service-class, not code-defect, not post-completion.
- **F2 — The death operation was `git push`, uniformly.** 50/50 crash transcripts end at
  `git push origin main`. The Aug-12 storm was push-side — the same mechanism kernel-proven at
  bf-198ne — not a gc-side event.
- **F3 — Needle's re-dispatch loop was the amplifier (the guide's H-1 residual).** A median
  155.5 s re-claim after each kill, with one alert bead minted per kill, converted one
  undrainable repository condition into 56 dispatches and 50 alert beads. Alerts scaled with
  kills, not with the single cause.
- **F4 — No work was lost.** The task completed five days later (bead Closed rev 2, verbatim
  reconciliation-verified close reason; split child `bf-2xygo` closed the same evening). The
  crash exposure was scheduling inside the bloat window, not task content.
- **F5 — The exit-0 attempt is not a self-heal.** It survived by abandoning the death operation
  (zero git commands — all split bookkeeping) while the store was still 18 GB. Recorded as
  task-shape change, per the guide's exit-0-after-storm rule.
- **F6 — The alert layer was stale by closure and is being drained.** All 50 alert beads target
  a bead Closed 2026-08-17. Live census this dispatch: **36 closed / 12 open / 2 in progress**
  (first `bf-276uk`, last `bf-2n3ve`). Per the response guide's Runbook A these retire by
  target resolution, not investigation; the open ones belong to their own disposition
  dispatches, not this report leg.
- **F7 — The regime is gone and is defended in depth.** Repo 106 MB (was 18 GB); `.beads/`
  fully gitignored (0 tracked files); 10 MB pre-commit gate; effective pack-memory bound
  ≈3 GiB worst case covering bare `git gc` *and* `git push` (`setup-git-gc-config.sh --verify`);
  daily/weekly bounded-gc timers; surge detector **STABLE, 0 crashes/24 h** live this dispatch
  (its file notes the 50 bf-4yjq crashes as older-than-horizon history). The bf-4yjq
  precondition is structurally ruled out.

---

## 7. Conclusion

`bf-4yjq`'s 50 kills were one infrastructure event: a repository-bloat regime (18 GB store,
17.20 GiB loose) in which every re-dispatch of the Forgejo-reconciliation task died at the same
`git push origin main` step, at fixed cadence, for 2 h 37 m — while the alert layer minted one
stale alert per kill. The classification is INFRASTRUCTURE / repository-bloat with high
confidence; the specific mechanism (push-side memcg-OOM inside the dispatch scope) is
regime-matched rather than kernel-proven for this bead only because the Aug-12 kernel records
no longer exist — sibling storms of the same era and repo are kernel-proven at the same
death point. Nothing about domain-check code was implicated (no stack trace, no application
error, anywhere in the chain). The task's work survived intact; the repository was repaired
and verified; the regime cannot recur through `.beads/`; and the monitoring layer that would
have caught the surge in progress is now installed and reporting STABLE.

**Disposition for this leg:** documentation complete. The 12 still-open alert beads are owned
by their own disposition dispatches (Runbook A target-resolution) and are explicitly *not*
actioned here.

## 8. Related Crashes

| Bead | What happened | Relation to bf-4yjq | Canonical record |
|---|---|---|---|
| **bf-173o7e** | 2026-08-14 storm: **132 dispatches / 129 × `exit −1`** over ~10.5 h | Same era's *gc-side* storm — same repo-bloat regime, later kernel-proven (memcg OOM of bounded-unaware git operations in the dispatch scope); the template chain this report belongs to parallels its chain | `docs/crash-investigations/bf-173o7e-crash-context-domchk-114d6172-2026-09-08.md`; chain closed 2026-09-09 |
| **bf-1s6c3** | 2026-08-12, same evening: **76 dispatches / 71 kills** | Same 18 GB store, same evening, same regime — Rule 3's "one environmental regime, two beads' worth of kills" | `docs/crash-analysis-bf-1s6c3-2026-09-06.md` (§12 is the dedup-append target) |
| **bf-198ne** | 2026-08-16 push-side memcg OOM, **kernel-proven** | The same push-side mechanism as bf-4yjq's storm, with surviving kernel records | `docs/crashes/bf-198ne-crash-report.md` |
| **bf-4x12ec** | memcg OOM of bare `git gc --aggressive` in the 12 GiB dispatch scope, kernel-proven | The gc-side kernel proof for the regime bf-4yjq ran inside; source of the `--full` fsck caveat | `docs/crash-reports/bf-4x12ec-git-gc-crash.md` |
| **bf-2ildm** | Committed the 17+ identical 237 MB `.beads/*.jsonl` snapshots | The *cause* of the bloat bf-4yjq's pushes choked on | `docs/investigations/bf-2ildm-crash-pattern-analysis-domchk-579d25b0-2026-09-07.md` |

**Supersessions to observe when quoting older bf-4yjq docs** (full table in the evidence file
§8): "9 crashes / 17-minute cadence" → **50 / median 155.5 s**; "-1 = SIGKILL stated as fact" →
**sentinel + regime-matched mechanism**; "no session evidence survives" → **all 56 transcripts
survive** (`docs/crash/bf-4yjq/raw-logs/`); "gc-side OOM" → **uniformly push-side**.

**Chain pointers:** evidence + classification live in `docs/crashes/bf-4yjq-evidence.md`
(§1–§9 gather, §10 classify); repair verification in
`docs/crashes/bf-4yjq-cleanup-verification.md`; framework in
`docs/crash-response-guide.md`; prevention-validation record in
`docs/crash-prevention-validation.md`.
