# Root Cause Determination: bf-4k2ws crash — investigation bead `domchk-7f838f36`

| Field | Value |
|---|---|
| **Subject bead** | `bf-4k2ws` — "Analyze divergent Forgejo and GitHub branch states" (created 2026-08-13T01:57:53Z, closed 2026-08-16T15:35:42Z, 8/8 acceptance criteria met — re-verified by `domchk-59478499`, commit `0aded2e`) |
| **Crash event** | 62-attempt loop, 2026-08-13 02:01:29Z → 07:17:41Z, single worker session `8446529e`: **55 × exit −1 + 5 × exit 124 + 2 × exit 0** |
| **Root cause** | **INFRASTRUCTURE — repository-bloat-era kill regime.** Each dispatch ran git-remote-heavy work (fetch / ls-remote / rev-list against Forgejo + GitHub) against the then-≈18 GB object store inside the 12 GiB dispatch scope and was killed mid-run; the mechanism is the kernel memcg-OOM SIGKILL proven for the same repository's gc- and push-side siblings (`bf-4x12ec`, `bf-198ne`). For this bead the mechanism is **chain-inferred, not kernel-proven** — no Aug-13 kernel record can exist (see §5) |
| **Failure mode** | Infrastructure event. Amplified by a workflow debt (verify-then-close: two verified successes were orphaned, so 25 of the 55 kills landed after the task was already done). The alert layer's one-alert-per-kill behaviour is separate and already corrected |
| **Confidence** | **Classification: HIGH. Mechanism (memcg OOM): MEDIUM-HIGH** — chain-inferred; direct proof is impossible for Aug-13 (§5). Every excluded category is a HIGH-confidence exclusion (§4) |
| **Determination by** | `domchk-7f838f36`, 2026-09-07 — re-verified every figure below first-hand from the primary source log; supersedes the 2026-09-02 corpus's SIGHUP / "did not crash" RCAs (§7) |

---

## 1. Causal chain

1. `bf-4k2ws` was claimed at **02:01:29.710Z — 7.15 s after** `bf-1s6c3`'s final
   `agent.completed` (02:01:22.561Z, exit 0) on the **same worker and session**
   (`8446529e`). It inherited both the worker and the night's kill regime: the
   predecessor had just finished its own 71-kill storm against the same
   then-≈18 GB repository (kernel-record era details:
   [`../crash-analysis-bf-1s6c3-2026-09-06.md`](../crash-analysis-bf-1s6c3-2026-09-06.md)).
2. The task is **git-remote-heavy** — fetch / ls-remote / rev-list against
   Forgejo and GitHub, on a repository holding ≈18 GB of objects (≈17 GB
   loose; canon-sourced from the cleanup record, not re-measurable — §3.2).
   Every significant git operation materializes or scans object data, so each
   dispatch carried a large transient allocation inside the dispatch scope.
3. Each dispatch ran inside needle's transient scope capped at **12 GiB**
   (`MemoryMax`). 55 of 62 attempts died **mid-run** — 123.6 s to 528.9 s in,
   median 252.9 s, **zero** cap-adjacent (§2) — the growth-curve signature of a
   scope budget exhausted while work was in progress, not of any timeout or
   fixed-duration boundary. Needle recorded the unrecorded-signal sentinel
   **exit −1**; the kernel records that would name the signal were lost to the
   2026-08-14 16:39 reboot (§5).
4. The retry layer re-dispatched the identical task for 5 h 16 m. **Nothing
   bounded the loop**, and the loop's two genuine successes were not allowed
   to end it: each exit-0 run passed its verification gate and was then
   **orphaned** back onto the queue ~6 s later instead of being closed.
5. The task **did complete** — twice inside the loop (04:48:09.546Z and
   07:17:41.039Z, both `verification.passed`) — and the bead closed cleanly on
   2026-08-16 with all 8 criteria met. The deliverables entered git in the
   2026-08-16 squash `c27899f`; **no commit exists in the storm window on
   `main` or `origin/main`** (`git log --since 2026-08-13T01:00Z --until
   09:00Z` is empty on both). The only in-window commits anywhere (`--all`)
   sit on the local-only, unpushed `pre-squash-history-20260816` ref and are
   sibling bead `bf-1ea4g`'s main-branch-state snapshots — none touch this
   bead's deliverable files.

## 2. Supporting evidence (primary sources; re-verified live 2026-09-07 by this bead)

All counts below were re-derived this session directly from
`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl`
(3.1 MB, mtime 2026-08-13 19:59 local — untouched since the crash night), not
copied from prior reports. They match the committed classification correction
(`docs/crashes/bf-4k2ws-crash-report.md`, 2026-09-07 section) and the artifact
bundle (`docs/crash-artifacts-bf-4k2ws/README.md`) exactly.

- **Attempt census (62 completions):** exit histogram **{−1: 55, 0: 2, 124: 5}**;
  first completion 02:03:33.620Z, last 07:17:41.039Z; `outcome.classified`
  labels: 55 `crash` / 5 `timeout` / 2 `success`. The two successes sit at
  04:48:09.546Z (378.98 s) and 07:17:41.039Z (193.38 s), each followed by
  `verification.passed` and then `bead.orphaned` at 04:48:15.306Z /
  07:17:47.390Z. Split by the first success: **30 kills mid-task** (02:03–04:48)
  and **25 kills post-completion** (04:48–07:17).
- **Kill-duration signature (new this session):** the 55 kills ran
  **123.6–528.9 s**, median **252.9 s**; histogram by 100 s bucket
  `[0, 15, 22, 15, 2, 1, 0]`; **zero kills within 70 s of the 600 s cap**.
  Deaths are spread across mid-run, consistent with resource exhaustion
  growing during work — and inconsistent with a timeout, a dispatch cap, or a
  fixed-lifetime failure mode.
- **The 5 × exit 124 are dispatch-cap timeouts, not max-turns:** durations
  **600,018 / 600,020 / 600,020 / 600,029 / 600,041 ms** — the 600 s dispatch
  cap to within tens of milliseconds.
- **Zero max-turns evidence anywhere:** the full day's log (all 395
  `agent.completed` events on this worker, every bead) contains **zero**
  `max_turns` / `max-turns` mentions, and no `error_max_turns` outcome exists.
- **Host CPU saturation co-factor:** **59 `fleet.cpu_saturated` samples** fall
  inside the 02:01–07:18Z window, load **7.63–18.51** on 9 cores (threshold
  ratio 0.8 ⇒ load > 7.2) — the box was saturated continuously, from the first
  sample at 02:03:45Z (load 14.43) to the last at 07:17:49Z (12.84).
- **Repo state, crash night vs now:**

  | Metric | 2026-08-13 (crash night) | 2026-09-07 (re-verified live) |
  |---|---|---|
  | `.git` size | ≈18 GB (canon-sourced) | **103 MB** |
  | Loose objects | ≈17 GB | 110 objects / **1.18 MiB** |
  | Packed | — | one pack, **99.11 MiB** |
  | `git fsck --full` | — | clean (exit 0) |
  | `check-repo-health.sh` | — | **passes**, effective gc bounds ≈3 GiB worst case |

  Loose-object count/size creep by normal churn between same-day snapshots
  (the committed crash report's correction section recorded 88 / 102 objects
  earlier on 09-07); the binding facts are the order of magnitude — ~1 MiB
  loose vs ≈17 GB on the crash night — and that `.beads/` stays fully
  gitignored with 0 tracked files, so the bloat path cannot recur.

  The crash-night figure cannot be re-measured — it survives only in the
  cleanup-era record
  ([`../crashes/bf-4yjq-cleanup-verification.md`](../crashes/bf-4yjq-cleanup-verification.md):
  "93 MB (was ~18GB)"). What *is* live-verified is that the bloat preconditions
  are gone and cannot recur through `.beads/` (fully gitignored, 0 tracked
  files).

## 3. Acceptance-criteria walk-through

### 3.1 Crash artifacts and classification from previous steps

Reviewed: the corrected classification
([`../crashes/bf-4k2ws-crash-report.md`](../crashes/bf-4k2ws-crash-report.md)
— **INFRASTRUCTURE**, repo-bloat era, alert layer false-positive; supersedes
the 2026-09-02 "FALSE POSITIVE — no crash occurred" report on its premise),
the artifact bundle
([`../crash-artifacts-bf-4k2ws/README.md`](../crash-artifacts-bf-4k2ws/README.md)
— alert↔death mapping: alert `bf-15k67`'s 02:33:47.409Z instant is a
`HANDLING_RELEASE_DONE` heartbeat 6.025 s **after** the real kill at
02:33:41.384Z), and the alert-generation correction (`9ae17f2` — real crash
alert = kill 7 of the 55; target legitimately still open at generation time).
Both prior steps' figures reproduced byte-exact this session (§2). The
automated classifier is an honest empty result — no trace survives (single-slot
traces; oldest surviving `.beads/traces/` dir is 2026-08-16), so
`scripts/crash-classifier.sh bf-4k2ws` exits 2.

### 3.2 Did repository bloat contribute?

**Yes — it is the primary axis, era-contextually.** The crash-night repository
was the bf-1s6c3/bf-4yjq bloat-era repository: ≈18 GB `.git`, ≈17 GB loose
objects, ~36× the healthy ceiling. That is canon-sourced, not re-measurable
(§2 table). Three independent lines tie the bloat to *these* kills:

1. **Temporal:** the loop sits inside the bloat era (2026-08-12 → 09-01) and
   starts 7.15 s after the predecessor's 71-kill bloat-era storm ended, on the
   same worker.
2. **Operational:** the task's own work (fetch / ls-remote / rev-list, plus
   each dispatch's transform/context build) is exactly the object-store-touching
   operation class the bloat era turned into kills.
3. **Mechanistic:** kills land mid-run at varying depths (§2 signature) — the
   pattern of a scope budget consumed as work proceeds, which an ≈18 GB object
   store forces on every significant git operation.

The 2026-09-02 report's "NOT Repository Bloat — clean repository state
(<500MB)" reading was captured 2026-09-02, three weeks post-repair, and
describes the wrong night (superseded in the crash report's correction
section; restated here because it is the single most misleading line in the
old corpus).

### 3.3 Memory pressure events around the crash timestamp

**Direct records cannot exist; here is exactly why, verified live:**

- `journalctl --list-boots` → **one boot, first entry 2026-08-15 19:56:33
  EDT**. The 2026-08-14 16:39 reboot discarded everything earlier; no memcg
  `oom-kill` line, no `memory.events`, no oomd record for Aug-13 survives.
- Health metric collection (memory/disk samples) begins 2026-08-15 — nothing
  exists for Aug-13.
- `.beads/traces/` retains nothing older than 2026-08-16.

What **is** recoverable for the window is host-axis CPU (§2: 59 saturated
samples, load 7.63–18.51 on 9 cores) — a confirmed stressor, but per-kill
causal attribution from load alone is impossible. Memory-pressure attribution
therefore rests on the chain: the mechanism is kernel-proven for the **same
repository** one day later (`bf-4x12ec`, gc-side, 2026-08-14) and five days
later (`bf-198ne`, push-side, 2026-08-16), both `CONSTRAINT_MEMCG` kills
inside dispatch scopes, with zero host-OOM kills in the era's kernel record.
This bead's kills carry the same exit sentinel, the same cadence, the same
repo, and the same mid-run signature.

### 3.4 Were agent workflow limitations reached (max turns)?

**No.** Zero `max_turns` mentions in the entire day's worker log (§2). The
only workflow-class events in the loop are the **5 dispatch-cap timeouts**
(exit 124 at 600.02 s ± 20 ms). The genuine workflow finding is different and
smaller: **verify-then-close debt** — the loop's two exit-0 runs both passed
their verification gate but were orphaned instead of closed, so **25 of the 55
kills were spent re-doing already-satisfied work**. That is what kept the bead
queued until 08-16, not any per-run turn limit.

### 3.5 Did domain-check code have any role?

**No.** The task is read-only repository/remote analysis (fetch, ls-remote,
rev-list) writing three markdown docs
(`docs/divergence-analysis-bf-4k2ws-2026-08-13-pre-merge.md`,
`docs/branch-divergence-bf-4k2ws-2026-08-13.md`,
`docs/branch-divergence-analysis-bf-4k2ws-current.md` — all present on
origin/main). No Go code path executes, no commit exists in the storm window,
and the deliverables are docs-only. This matches the corpus-wide result:
157+ investigations of this workspace, **zero domain-check defects found**
(`docs/plan/plan.md`, Incident Response section).

### 3.6 Primary root cause

**INFRASTRUCTURE — the repository-bloat-era kill regime**: dispatches running
git-heavy work against the ≈18 GB object store inside a 12 GiB dispatch scope,
killed mid-run when the scope's budget was exhausted; the retry layer
re-queued the identical task 55 times. Supporting evidence: §1 (chain), §2
(census, signature, era size, CPU), §3.2–3.3 (bloat tie-in and the
kernel-proof chain via the siblings).

**Amplifier (secondary):** verify-then-close workflow debt — two verified
successes orphaned, converting a ~30-kill storm into a 55-kill one and keeping
the bead open for three more days.

**Alert layer (separate defect, already fixed):** pre-dedup needle
`handle_crash` generated **one alert bead per kill** (55 alert beads, ~37 still
open), with no fingerprint on any of them — the motivation for the
closed-bead/dedup/cooldown fixes in `scripts/crash-alert-manager.sh`.

**Co-factor:** continuous host CPU saturation (§2). Present throughout, but
not independently sufficient — the era's kill mechanism is cgroup-scoped, and
no host-OOM kill exists in the kernel record.

## 4. Ruled-out alternates

| Alternate | Verdict | Evidence |
|---|---|---|
| Max-turns exhaustion | **Ruled out** | 0 `max_turns` mentions in the whole day's log; no `error_max_turns` outcome; the 5 timeouts are 600 s cap hits, not turn limits |
| Service unavailability (503/502) | **Ruled out** | No service-failure classification anywhere in the loop; no 5xx-class event in the window; the loop's failures are uniform exit −1 |
| Code defect in domain-check | **Ruled out** | Read-only git task, no code executed, no commit in window; corpus-wide zero defects in 157+ investigations |
| SIGHUP cascade (2026-09-02 corpus claim) | **Ruled out** | **Zero exit-129s across all 62 attempts**; SIGHUP death would be 128+1; the mechanism was never kernel-confirmed and is retired corpus-wide |
| "No crash occurred" (2026-09-02 premise) | **Ruled out** | 55 real kills recounted from the primary log; the task's eventual success does not erase them |
| Host OOM (host out of memory) | **Ruled out** by proxy | Era kernel records (Aug-14→16) show 100 % `CONSTRAINT_MEMCG`, zero host-OOM kills; the constraint was the 12 GiB cgroup, not the host (46 GiB free today; host memory was never the binding constraint in this era) |
| Pure false positive (alert layer only) | **Partially true, not causal** | The alert layer *was* false-positive-prone (one bead per kill), but the kills themselves were real — the classification is INFRASTRUCTURE with an alert-layer defect, per the corrected report |

**Erratum noted for the next pass on the crash report:** the 2026-09-07
correction section in `docs/crashes/bf-4k2ws-crash-report.md` calls
`bf-1s6c3`'s storm "**kernel-verified** memcg OOM". `bf-1s6c3`'s own canonical
report says the opposite about itself: "No kernel records exist for Aug-12"
and "kernel-proven only for the later siblings" (`bf-4x12ec`, `bf-198ne`). The
accurate statement — used throughout this document — is *chain-inferred for
both Aug-12/13 storms; kernel-proven for the gc/push siblings on the same
repo*. This document intentionally leaves that file untouched (one wording
fix, line ~387, remains open for a future pass); its authoring bead
(`domchk-3fca6de4`) has since closed, so no worker is actively editing it.

## 5. Confidence level

| Claim | Confidence | Basis |
|---|---|---|
| 55 real kills occurred, in this window, on this bead | **Certain** (primary-source recounted) | Exit histogram re-derived from the untouched Aug-13 worker log; matches two independent committed extractions |
| Classification: INFRASTRUCTURE | **HIGH** | Uniform exit −1 at fixed cadence for 5 h 16 m; no max-turns, no 5xx, no code path; era context; zero counter-evidence in any surviving record |
| Mechanism = memcg-OOM in the 12 GiB dispatch scope | **MEDIUM-HIGH** | Kernel-proven for the same repo's gc/push siblings; identical sentinel/cadence/signature; mid-run kill-duration distribution fits resource exhaustion. **Cannot be raised to certain for this bead:** the only records that could prove it were destroyed by the Aug-14 reboot — verified live (single journald boot begins 2026-08-15 19:56:33 EDT) |
| Exclusions (max-turns / service / code / SIGHUP / no-crash) | **HIGH** each | Independently verified per §4 |
| Verify-then-close debt as the loop's amplifier | **HIGH** | Two `verification.passed` + `bead.orphaned` pairs recounted; 25 post-completion kills follow mechanically from them |

What would change the mechanism verdict: recovery of an Aug-13 kernel record
(impossible — boot boundary verified) or a surviving dispatch-scope
`memory.peak` (none exists). Absent those, MEDIUM-HIGH is the ceiling this
evidence supports, and it is the same proof level every other Aug-12/13 storm
in this workspace carries.

## 6. Dedup target for parallel RCA beads

This alert pool still holds multiple open root-cause-scope beads
(`domchk-474e649d`, `domchk-29f7f613`, `domchk-24f329ec`, `domchk-541f1089`,
and verification bead `domchk-e02032f2`). **This document is the append target
for that scope.** If you hold one of those beads: re-run the §2 checks against
the primary log (they take minutes), append a dated subsection here with any
*delta* you find, and close — do not create a new RCA document. The corpus
already carries ~20 bf-4k2ws reports from the superseded SIGHUP/no-crash era;
that sprawl is what this section exists to stop.

## 7. Supersession map

| Document | Status |
|---|---|
| `docs/crashes/bf-4k2ws-crash-report.md` (2026-09-02 + 09-07 correction) | **Current** — its correction section stands; one wording erratum noted in §4 above |
| `docs/crash-artifacts-bf-4k2ws/` bundle (2026-09-07) | **Current** — primary-evidence extract; its "FALSE_POSITIVE" item 3 wording predates the 09-07 reclassification and is superseded by the crash report's correction |
| `docs/crash-investigations/bf-4k2ws/root-cause-analysis-final-bf-4k2ws.md` and the rest of the 2026-09-02 corpus (SIGHUP cascade / "did not crash") | **Superseded** on premise and mechanism; their task-completion finding (8/8 criteria, closed 08-16) stands |
| `docs/investigations/bf-4k2ws-crash-verification-2026-09-02.md` | Superseded by this document's §2/§3 (its figures were re-verified and extended) |

---

*Determination by `domchk-7f838f36`, 2026-09-07. Every count in §2 was
re-derived this session from
`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` and the
live repository state; no figure is cited from prior reports without
independent reproduction.*
