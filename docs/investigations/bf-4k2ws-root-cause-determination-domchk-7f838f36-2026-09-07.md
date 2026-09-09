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
| `docs/archive/crash-investigations/crash-documentation-domchk-e3e443bf-2026-09-02.md` (documentation bead `domchk-e3e443bf`'s own 2026-09-02 deliverable, commit `66592dc`; archived by `a883044`) | **Superseded** on premise — "FALSE POSITIVE - No actual crash occurred", SIGHUP (exit −1 misread as signal 1), "Root Cause: NEEDLE crash detection deficiencies"; its findings-and-resolution summary is restated under the current determination in §14; its task-completion finding (closed 2026-08-16, deliverables intact) stands. The same superseded story is also in that bead's Notes (corrected by dated note append 2026-09-07) |

## 8. Incident chronology summary (bead `domchk-4311aaa8`, 2026-09-07)

Dispatched to "write a summary of what happened" for `bf-4k2ws` (chronology,
alert generation, FALSE-POSITIVE clarification, timestamp confusion). Appended
here per §6 instead of adding yet another separate `bf-4k2ws` document. Every
figure below was re-derived this session from the primary log, the live bead
store, and `origin/main`; all of it reproduces §1–§2 byte-exact, with two small
deltas flagged.

### 8.1 Chronology (all times UTC, 2026-08-13 unless dated)

| Instant | Event |
|---|---|
| 01:57:53.592Z | `bf-4k2ws` created ("Analyze divergent Forgejo and GitHub branch states"). **Bead-existence instant, not a crash instant** — see 8.4 |
| 02:01:22.561Z | predecessor `bf-1s6c3` completes exit 0 on the same worker/session `8446529e` — and is itself orphaned 5.2 s later (02:01:27.732Z), the same verify-then-close debt that later amplifies this storm |
| 02:01:29.710Z | `bf-4k2ws` claimed — **7.15 s** after the predecessor's completion |
| 02:03:33.620Z | attempt 1 killed (exit −1, 123.6 s in). Storm begins |
| 02:03:40.611Z | first `HANDLING_RELEASE_DONE` heartbeat; first alert bead created 6 ms later (02:03:40.617Z) — one alert per kill starts here |
| 02:03 → 04:48 | attempts 1–32: **30 mid-task kills** + the night's first 2 dispatch-cap timeouts (attempts 16, 17, 600.0 s each) |
| 04:48:09.546Z | attempt 33 — **first genuine success** (exit 0, 379.0 s); `verification.passed` 17 ms later |
| 04:48:15.306Z | `bead.orphaned` — the verified success is discarded back onto the queue; the loop does not end |
| 04:48 → 07:14 | attempts 34–61: **25 post-completion kills** (attempts 34–57, 60) re-doing already-satisfied work, plus 3 more dispatch-cap timeouts (attempts 58, 59, 61) |
| 07:03:53.920Z | last kill (attempt 60, 528.9 s — the longest); its alert fires 07:04:03.300Z |
| 07:14:06.478Z | attempt 61 times out (fourth-to-last 600 s cap hit) |
| 07:17:41.039Z | attempt 62 — **second genuine success** (exit 0, 193.4 s); `verification.passed`; orphaned again 6.4 s later (07:17:47.390Z) |
| 2026-08-16 15:35:42Z | `bf-4k2ws` **closed**, all 8 acceptance criteria met; deliverables in the 08-16 squash `c27899f` |

Attempt census (§2, reproduced): **62 attempts = 55 kills (exit −1) + 5
timeouts (exit 124, all 600.02 s ± 20 ms) + 2 successes (exit 0)**. Kills ran
123.6–528.9 s, median 252.9 s, none cap-adjacent. Zero `max_turns` mentions in
the whole day's 395 completions.

### 8.2 Crash alert generation

The pre-0.4.2 needle `handle_crash` path ran, per kill: `outcome.classified`
(crash) → `bead.released` → **`outcome.handled {"action": "alerted"}`** → one
new alert bead. Verified on both sides of the ledger:

- **In the worker log:** exactly **55** `action=alerted` events for
  `bf-4k2ws`, first 02:03:43.020Z, last 07:04:03.300Z — one per kill, no
  fingerprint, no dedup, no cooldown.
- **In the bead store:** exactly **55** beads titled `ALERT: Agent crash on
  bead bf-4k2ws`, all created inside the storm window (02:03:40.617Z →
  07:04:00.881Z). Status today: **17 closed, 36 open, 2 in_progress** —
  the determination doc's §3.6 "~37 still open" is exactly 36 open + 2
  in_progress (first delta). The wider pool of beads *naming* `bf-4k2ws`
  (alert + downstream investigation/verification beads) is 183:
  118 closed / 59 open / 6 in_progress.

Each alert bead's creation timestamp is the `HANDLING_RELEASE_DONE` heartbeat
of its kill — within **6 ms** of it (bead 1: 02:03:40.617Z vs heartbeat
02:03:40.611Z; bead 2: 02:09:27.299Z vs 02:09:27.293Z; bead 3: 02:13:47.524Z
vs 02:13:47.517Z), and the heartbeats themselves land **5.1–9.8 s after** the
real kill across all 60 completions. The alert that seeded this investigation
chain, `bf-15k67` (instant 02:33:47.409682217Z), is attempt 8: kill at
02:33:41.384776124Z (173.9 s run) → heartbeat 02:33:47.409670765Z (**6.025 s
after death**) → bead row written **11.5 µs** after the heartbeat.

### 8.3 FALSE POSITIVE — what that determination does and does not mean

Three layers, which the superseded 2026-09-02 corpus flattened into one:

1. **The kills were real.** 55 genuine mid-run deaths, recounted from the
   primary log (§2). "FALSE POSITIVE" never meant "no crash occurred" — that
   premise was superseded by the reclassification (`ef39024`, 2026-09-07).
2. **The false positives were in the alert layer, and they were of two
   kinds.** (a) *Multiplication*: 55 alert beads for one root cause, because
   `handle_crash` alerted per kill with no fingerprint/dedup/cooldown — the
   defect the `scripts/crash-alert-manager.sh` fixes exist for. (b) *Stale
   target*: the loop's two verified successes were orphaned instead of
   closing, so 25 of the 55 kills (and their alerts) fired for work that was
   already done; and every alert after 2026-08-16 15:35:42Z — the bulk of the
   ~180-bead downstream pool — fired against an already-closed bead.
3. **The current classification** is **INFRASTRUCTURE (repository-bloat-era
   kill regime) with a separate, already-fixed alert-layer defect** (§3.6).
   `bf-4k2ws` itself is a *victim* bead, not a false alarm: the task
   completed 8/8, its four deliverable docs are on `origin/main`
   (`docs/divergence-analysis-bf-4k2ws-2026-08-13-pre-merge.md`,
   `docs/branch-divergence-bf-4k2ws-2026-08-13.md`,
   `docs/branch-divergence-analysis-bf-4k2ws-current.md`,
   `docs/branch-divergence-analysis.md`), the storm window holds **zero**
   commits on `main` or `origin/main` (re-verified live today), and branch
   divergence is **0/0** at today's fetch.

### 8.4 Timestamp confusion, disentangled

Every wrong figure in the superseded corpus traces to one of five distinct
timestamp layers:

| Layer | What it stamps | bf-4k2ws values |
|---|---|---|
| Bead creation | when the bead row was written | 01:57:53.592Z — **3.6 min before the first claim and 5.7 min before the first kill**; quoting it as "crash time" is the oldest corpus error |
| Death (`agent.completed`) | the actual kill | 55 instants, 02:03:33.620Z → 07:03:53.920Z; **the only correct "crash timestamps"** |
| Alert/heartbeat | `HANDLING_RELEASE_DONE`, then the bead write | death + 5.1–9.8 s; an alert instant **never names a distinct crash** — read back to the preceding `agent.completed` instead (e.g. 02:33:47.409Z = attempt 8, not a 56th event) |
| Timezone | needle/JSONL logs are **UTC (`Z`)**; journald stamps **local EDT (−4 h)** | the storm ran 21:57 EDT Aug-12 → 03:17 EDT Aug-13 local; and no journald record of it can exist — the single boot begins 2026-08-15 19:56:33 EDT (re-verified live today) |
| Closure | `bead close` | 2026-08-16 15:35:42Z — three days *after* the storm; anything timestamped later describes a closed bead |

Exit-code layer, same confusion family: `−1` is the unrecorded-signal sentinel
(kernel kill), `124` is the 600 s dispatch cap (attempts 16, 17, 58, 59, 61 —
**not** max-turns; zero max-turns evidence exists anywhere in the day's log),
`0` is success. "62 attempts" and "55 crashes" are both correct counts of
different things (second delta, purely presentational: the 5 timeouts sit at
attempts 16, 17, 58, 59, 61 in the recovered sequence, so kill #N ≠ attempt #N
anywhere after attempt 15).

---

## 9. NEEDLE system deficiencies — consolidated RCA (bead `domchk-a7bc56b5`, 2026-09-07)

Dispatched to "document the investigation findings for the NEEDLE system
issue": crash detection deficiencies, completion detection failure, timestamp
confusion mechanism, lack of deduplication. Appended here per §6. No
contradiction with §1–§8: this session re-derived §2's census byte-exact from
the untouched Aug-13 log (62 completions = 55 × −1 + 5 × 124 + 2 × 0; kills
123.6–528.9 s, median 252.9 s; zero `max_turns` mentions in the day's 395
completions) and §8.2's alert ledger in the live bead store (exactly 55
ALERT-titled beads: 17 closed / 36 open / 2 in_progress). The delta is the
system-level statement of the four deficiencies, each tied to its live
re-verification.

### 9.1 Crash detection is post-hoc only, and records too little to name its cause

- **No in-run signal exists.** The sole crash indicator in the Aug-13 log is
  `agent.completed {exit_code: −1}` — detection fires only at process exit.
  The dispatch scope's 12 GiB ceiling was crossed with no warning event; the
  window's only resource telemetry (`fleet.cpu_saturated`, 59 samples) measures
  the host axis, not the per-dispatch scope budget. Detection could therefore
  neither anticipate a kill nor stop the retry layer from re-dispatching the
  identical task into identical conditions 55 times.
- **What detection records cannot name its cause.** Single-slot traces retain
  nothing older than 2026-08-16 and journald's single boot begins
  2026-08-15 19:56:33 EDT, so all 55 kills carry only the unrecorded-signal
  sentinel −1 with no kernel line — the reason this bead's mechanism stays
  chain-inferred (§5) rather than kernel-proven.
- **Fix-state (verified live):** the standing mitigations are repo-side, not
  needle-side — the bloat repair itself (`.git` 103 MB vs ≈18 GB, `.beads/`
  fully gitignored) removes the allocation pressure that exhausted the scope,
  and `pack.windowMemory=2g` / `pack.threads=1` (worst case ≈3 GiB,
  `./scripts/setup-git-gc-config.sh --verify`) bound the gc/push pack paths;
  `check-repo-health.sh` plus the daily timers detect size regressions before
  they become kills. Needle itself still has no in-run scope-pressure alarm.

### 9.2 Completion detection failure: a verified success got `action: none`

The full `outcome.handled` census for the bead (re-derived this session):
**crash → alerted ×55, timeout → deferred ×5, success → none ×2.** The two
successes record, in sequence:

```text
04:48:09.546Z  agent.completed exit 0 (378.98 s)
+17 ms         verification.passed {gates_run: 1}
+5.74 s        bead.orphaned  ‖  outcome.handled {"action": "none", "outcome": "success"}
07:17:41.039Z  agent.completed exit 0 (193.38 s)
+19 ms         verification.passed {gates_run: 1}
+6.33 s        bead.orphaned  ‖  outcome.handled {"action": "none", "outcome": "success"}
```

`outcome: "success"` was **known and recorded**, yet the handler's chosen
action was **`none`** — no terminal transition, the bead re-queued, and the
retry layer re-dispatched. The failure path took a stronger action than the
success path (`alerted` per kill vs `none` per verified success); only the
timeout path (`deferred`) behaved sanely, and none of the three could end a
loop whose work was already done. Consequence, quantified in §3.4: 25 of the
55 kills (45 %) landed after the first verified success, re-doing satisfied
work, and every later alert fired for a target whose task was already done.
This is a needle workflow defect — the bead's own task completed 8/8 both
times.

### 9.3 Timestamp confusion is structural: the pipeline never stamps the death

§8.4 tabulates the five confusion layers; the mechanism generating them is
that the crash pipeline's recorded instant is not the death. Per kill the
recorded sequence is `agent.completed` (the only true crash instant) →
`HANDLING_RELEASE_DONE` heartbeat **+5.10–9.80 s** (median 6.10 s — re-derived
this session across all 60 crash completions by sequence-ordered pairing) →
alert-bead row **≤6 ms** after the heartbeat. Any consumer quoting the alert
bead's `created_at` as "crash time" is wrong by construction, by +5.1–9.8 s
at minimum — and by a whole event class when a heartbeat is read as a
distinct crash (§8.4). The UTC-vs-EDT offset and the Aug-15 journald boundary
compound this mechanically, not accidentally.

### 9.4 Lack of deduplication, and its current countermeasures

Pre-0.4.2 `handle_crash` alerted per kill with no fingerprint, no cooldown,
no target-state check — re-verified on both ledger sides this session (55
`action=alerted` events 02:03:43.020Z → 07:04:03.300Z in the log; exactly 55
`ALERT: Agent crash on bead bf-4k2ws` beads in the store, all created inside
the storm window). **Countermeasures, with an effectiveness caveat:** the
repo-side pieces exist as code — `scripts/crash-alert-manager.sh` carries a
300 s cooldown (`ALERT_COOLDOWN_SECONDS`), processed-alert tracking
(`PROCESSED_ALERTS_FILE`), dedup via `scripts/alert-deduplication.sh`,
closed-target filtering, and exit-code validation separating the −1 sentinel
from the 124 cap class — but presence is not effectiveness: the same-day gap
analysis ([`../alert-deduplication-gap-analysis-2026-09-07.md`](../alert-deduplication-gap-analysis-2026-09-07.md),
bead `domchk-b5448b6a`, commit `7d34f8c`) finds that pipeline has **never
once fired in production** — no production caller invokes
`crash-alert-manager.sh` (the only timer in this stack runs the report-only
`crash-pattern-detection.sh`), every dedup ledger is empty while the alert
pool holds 1,714 beads (~177 open against closed targets), and ten structural
gaps (D-1–D-10) are documented there with reproductions. This subsection's
"verified live" therefore means the knobs are present; the §8.2
alert-multiplication mechanism is current behavior, not history. The 36
still-open historical alert beads remain for their owners — outside this
bead's scope.

### 9.5 Domain-check code exoneration (re-verified)

§3.5's result stands: read-only git/remote analysis, no Go code path in the
storm, all four deliverable docs present on `origin/main` (re-verified via
`git cat-file -e origin/main:<path>` this session), **zero** commits in the
storm window on `main`/`origin/main`, and the corpus-wide result of zero
domain-check defects across 157+ investigations. The NEEDLE deficiencies
above are infrastructure and workflow defects of the agent platform, not of
the checked application.

---

## 10. Fix specification — implementation requirements for the fix chain (bead `domchk-9bd1f524`, 2026-09-07)

Dispatched to "document specific fix requirements based on root cause
analysis": review the crash docs, name the exact change needed, list affected
files/components, and hand the next bead an implementation checklist. Appended
here per §6 rather than as a new document. Chain position: fix-type
determination `domchk-2222ea44` (closed) → implementation bead
`domchk-20f2666d` (closed **no-commit** — the fix type is already implemented)
→ **this requirements document** → `domchk-aa986d4b` "Implement and locally
test the crash fix" (open; its description names "bead from step 1
(requirements document)" as its blocker — that is this section) →
`domchk-ecf47b49` (test) → `domchk-26ccd69b` / `domchk-05d44870`
(document/commit/push) → `domchk-34871e96` (verify effectiveness).

The fix type per `domchk-2222ea44`'s determination, re-verified this session:
**PRIMARY — resource limit** (bound the memory a dispatch can materialize; the
5 × exit 124 are the 600 s dispatch cap — attempts 16, 17, 58, 59, 61 at
600.02 s ± 20 ms — a work-time bound, not a repo defect); **SECONDARY — safer
git patterns**; signal handling ruled out (G-6); NEEDLE-infrastructure
reporting already discharged as G-9..G-13. **Every component of that fix
already exists, committed on `origin/main`.** This spec therefore binds each
requirement to the exact committed artifact that satisfies it, marks the short
list of items still open *with their owners*, and gives the implementing bead
a pass/fail battery — so it verifies rather than rebuilds, and does not
manufacture a code change. Every "verified live" figure below is this bead's
own run at HEAD `7587471` = `origin/main`, 2026-09-07.

### 10.1 Requirement → committed artifact

| # | Requirement (from the fix type) | Committed artifact (affected files/components) | Live verification, this bead |
|---|---|---|---|
| R1 | Pack memory bounded inside the 12 GiB dispatch scope — for **both** `gc` and push-side `pack-objects` | git config `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` at **global and local** scope; installer/verifier `scripts/setup-git-gc-config.sh` | `--verify` exit 0: effective chain resolves system→global→local, worst case ≈3072 MiB, "within the 6442450944 ceiling for a 12GiB dispatch scope" |
| R2 | Object store must not re-bloat | `.gitignore` lines 66–70 (`.beads/`, `*.db`, `*.db.backup.*`, `*.jsonl`); `scripts/setup-git-hooks.sh` → 10 MB `.git/hooks/pre-commit` gate (G-1, closed `dfa60a9`) | `git ls-files .beads` → empty; hook check exit 0 "byte-identical to tracked source"; `.git` 103 MB, 157 loose objects / 1.69 MiB, one 99.11 MiB pack, garbage 0 |
| R3 | Remediation stays bounded and **unconditional** | `scripts/safe-git-gc.sh` (staged, checkpoint/resume, `--check-only`); daily 03:00 `domain-check-git-gc.service` + weekly Sun 04:00 `--full`, both `MemoryMax=4G` | `--check-only` exit 0, resource checks pass ("GC not needed" is the healthy line, not a failure); all 7 `domain-check-*` user timers present with future NEXT |
| R4 | Heavy work gated on environment | `scripts/preflight-health-check.sh`; `scripts/resource-monitor.sh` (`PRESSURE_WARNING=70` / `PRESSURE_CRITICAL=80`); `scripts/system-event-mode.sh` surge gate (G-3 implementation, `e0fab45`) | preflight exit 0, 4/4 checks; resource/service/monitoring timers fired minutes before this run; `system-event-mode.sh check` exit 0 "clear" |
| R5 | **Not** the fix: signal handling (G-6 — hardens `internal/server/server.go`, not the dying needle workers); domain-check code (§9.5 exoneration); the alert-dedup layer (§9.4's caveat: knobs present, pipeline never fired — D-1..D-10) | — | — |

The storm's two kill legs are covered by different halves of this table: the
55 mid-run kills by R1+R2 (bound the allocation, keep the object store small),
the 5 × exit 124 by nothing in this repo — the cap is a NEEDLE work-time knob
(G-11/G-12), and the correct repo-side response is the R4 gate, not a longer
cap.

### 10.2 Genuinely open items — with owners; not this chain's work

| Item | Where | Owner / vehicle |
|---|---|---|
| G-3 adoption half — actually *call* the `system-event-mode.sh` gate from `crash-alert-manager.sh` and `preflight-health-check.sh` | those two scripts | **OPEN bead `domchk-6951fe0c`** — do not duplicate |
| Alert-layer D-1..D-10 (dedup pipeline never once fired in production) | `docs/alert-deduplication-gap-analysis-2026-09-07.md` | `domchk-b5448b6a`'s prioritized fix list; note its repo-boundary caveat (needle owns bead creation) |
| Work-completion detection at the alert source (G-9), dispatch-scope sizing (G-10), retry/backoff (G-11), turn budgets (G-12), CPU throttling (G-13) | NEEDLE repo | external canon — Phase 3 of `docs/crash-prevention-requirements.md` |
| G-4 / G-5 / G-7 / G-8 hygiene (gateway failover, prevention feedback loop, `monitoring-setup.sh` retirement, evidence retention) | `docs/crash-prevention-requirements.md` §5 | register items, separate scope |

**Worktree hazard for any implementation bead:** `scripts/` currently carries
~48 files of co-tenant uncommitted edits — including `safe-git-gc.sh` and
`preflight-health-check.sh`, two of R3/R4's artifacts. Build on the committed
versions (`git ls-tree origin/main -- scripts/<file>`), never on the dirty
copies, and do not sweep them into a commit.

### 10.3 Anti-requirements (do NOT do these)

1. No bare `git gc --aggressive` — the bf-4x12ec mechanism; the guard is
   persistent git config (R1), not convention.
2. No gc-threshold re-tuning and no conditional `--auto-when-needed` gating of
   the nightly gc — withdrawn at G-2 with the bf-198ne rationale (conditional
   gates are blind to bloat *inside* a pack).
3. No Go changes: the crash population never touched the application (§9.5);
   a "fix" commit under `internal/` would be manufacture.
4. No new signal-handling work (G-6).
5. No new `bf-4k2ws`-scope document — append a dated § here per §6.

### 10.4 Implementation checklist for `domchk-aa986d4b` (implement + local test)

Because R1–R4 are already implemented, "implement and locally test" for this
chain means **run the battery, record each result on the bead, and change
nothing unless a line FAILS** — a failure is the work item; a full pass is the
correct terminal state, and the bead closes without a code commit (mirroring
`domchk-20f2666d`).

| # | Command | Pass criterion |
|---|---|---|
| 1 | `git fetch origin; git rev-parse HEAD origin/main; git merge-base --is-ancestor HEAD origin/main` | equal SHAs, exit 0 — zero unpushed |
| 2 | `./scripts/setup-git-gc-config.sh --verify` | exit 0; worst case ≈3072 MiB within the 6 GiB ceiling |
| 3 | `git config --show-scope --get-all pack.windowMemory` and likewise `pack.threads`, `pack.deltaCacheSize` | both a `global` and a `local` row for each key |
| 4 | `du -sh .git; git count-objects -vH` | `.git` ≈103 MB; loose ≲2 MiB; pack ≈99 MiB; garbage 0 |
| 5 | `grep -n -e beads -e '\.db' -e jsonl .gitignore` then `git ls-files .beads` | the four ignore rules; second command prints nothing |
| 6 | `./scripts/setup-git-hooks.sh --check` | exit 0, "byte-identical to tracked source" |
| 7 | `./scripts/safe-git-gc.sh --check-only` | exit 0; resource checks pass |
| 8 | `./scripts/preflight-health-check.sh` | exit 0, 4/4 checks |
| 9 | `systemctl --user list-timers 'domain-check-*' --all` | 7 timers, every NEXT in the future |
| 10 | `./scripts/system-event-mode.sh check` | exit 0 "clear" (75 = defer is the correct *active-event* answer, not a failure) |
| 11 | `./scripts/test-crash-alert-fixes.sh` | exit 0, 12/12 (knobs present; pipeline effectiveness is §9.4/D-1..D-10, out of scope) |
| 12 | `./scripts/test-safe-git-gc-limits.sh` (33 assertions, seconds), optionally `./scripts/test-gc-memory-bounds.sh` (768 MiB cgroup replay of the crash command) | exit 0 |

Optional negative proofs are the `DOMCHECK_RUN_LONG_TESTS=1` variants per
`scripts/README.md` — not required for closure.

*§10 appended by `domchk-9bd1f524`, 2026-09-07. Docs reviewed: this document
§1–§9, `docs/crashes/bf-4k2ws-crash-report.md` including its Classification
Correction, `docs/crash-prevention-requirements.md` (G-1..G-13 with the
09-06/09-07 closures and the G-2 withdrawal), and
`docs/alert-deduplication-gap-analysis-2026-09-07.md` (D-1..D-10). Every R1–R4
verification line in §10.1 and steps 1–11 of §10.4 were executed live this
session at HEAD `7587471`; step 12 cites the committed self-tests unexecuted.*

## 11. RCA verification — bead `domchk-e02032f2`, 2026-09-07

Dispatched to "verify root cause analysis and findings" against the
`docs/crash-investigations/bf-4k2ws/root-cause-analysis-*.md` pair, with
acceptance criteria covering root cause + evidence, the code-vs-infrastructure
/ transient-vs-systemic / crash-vs-false-positive distinctions, repository
health, system resource state at crash time, and crash-response-guide
cross-references. Appended here per §6. **Verdict: the determination holds —
every load-bearing figure re-derived byte-exact this session; one actionable
delta (11.3).**

### 11.1 §2 census reproduced byte-exact from the primary log

Same log (3,111,314 bytes, mtime 2026-08-13 19:59 local — untouched). All
figures re-derived independently with a fresh `json.loads` pass:

- 62 `bf-4k2ws` completions; exit histogram **{−1: 55, 124: 5, 0: 2}**;
  `outcome.classified` **55 crash / 5 timeout / 2 success**.
- Kills **123,571–528,854 ms** (123.6–528.9 s), median **252,874 ms**
  (252.9 s), bucket histogram **[0, 15, 22, 15, 2, 1, 0]**, **0** kills within
  70 s of the 600 s cap.
- Timeouts **600,018 / 600,020 / 600,020 / 600,029 / 600,041 ms** — exact.
- Successes **04:48:09.546532879Z** (378,983 ms) and **07:17:41.039398822Z**
  (193,380 ms); `verification.passed` **+17 ms / +19 ms**; `bead.orphaned`
  **04:48:15.306452783Z / 07:17:47.390472518Z** (+5.74 s / +6.33 s) — exact.
- **0** `max_turns`/`max-turns` mentions in the whole day's **395**
  `agent.completed` events.
- **59** `fleet.cpu_saturated` samples in 02:01–07:18Z, load **7.63–18.51**
  on **9** cores, first 02:03:45Z, last 07:17:49Z — exact.
- **55** `outcome.handled {action: alerted}` events, first **02:03:43.020Z**,
  last **07:04:03.300Z**; **60** `heartbeat.emitted` HANDLING_RELEASE_DONE
  rows, death deltas **5.10–9.80 s median 6.10 s**; the seed-alert chain
  reproduces (kill 02:33:41.384776124Z → heartbeat 02:33:47.409670765Z,
  **6.025 s** after death) — exact.

### 11.2 Repo, bead store, remote — live re-verified

- `.git` **103 MB**; loose **171 objects / 1.81 MiB**; one pack **99.11 MiB**
  (11,360 in-pack); `git fsck --full` exit 0 (dangling-only); `.beads/` **0**
  tracked files; `check-repo-health.sh` and `setup-git-gc-config.sh --verify`
  both exit 0. Loose count/size is the third same-day snapshot in the creep
  series §2 already flags (88/102 → 110/1.18 MiB → 171/1.81 MiB) — the
  binding order of magnitude (~1–2 MiB vs ≈17 GB) is unaffected.
- Bead store: **exactly 55** ALERT-titled beads, **17 closed / 36 open /
  2 in_progress** — §8.2's census unchanged. The wider pool naming `bf-4k2ws`
  is now **189 (124c / 58o / 7ip)** vs §8.2's 183 (118c / 59o / 6ip) — pure
  same-day growth from ongoing dispatches, no bearing on the determination.
- All four deliverable docs present on `origin/main`; storm window
  01:00–09:00Z holds **0** commits on `main` and `origin/main`; divergence
  **0/0**; squash `c27899f` present. `scripts/crash-classifier.sh bf-4k2ws`
  exits **2** (no trace survives; oldest `.beads/traces/` dirs are 2026-08-16,
  re-checked by mtime).
- Acceptance criteria: root cause + evidence (§1–§2 ✓), code-vs-infra and
  transient-vs-systemic (§3.5–§3.6 ✓), crash-vs-false-positive three-layer
  split (§8.3 ✓), repo health (§2 table + above ✓), resource state at crash
  time (§2 CPU + §3.3's verified-unrecoverable memory/disk — single journald
  boot confirmed 2026-08-15 19:56:33 EDT ✓), crash-response-guide
  cross-reference (`docs/crash-response-guide.md` present, exit −1 patterns
  §"Exit Code -1" ✓; mitigation scripts `crash-alert-manager.sh`,
  `alert-deduplication.sh`, `safe-git-gc.sh` all present ✓).

### 11.3 Delta: `root-cause-analysis-signal-minus1.md` still carries no superseded banner

§7 supersedes it in the map, but the file itself redirects nobody — unlike
its sibling `root-cause-analysis-final-bf-4k2ws.md`, which carries the
2026-09-07 SUPERSEDED header. A reader landing on it cold still gets the
retired SIGHUP-cascade premise stated as current, plus three stale figures
the rest of the corpus has corrected: exit −1 equated with "SIGHUP/SIGKILL"
(−1 is the unrecorded-signal sentinel, §8.4), the invented
40/30/15/15 % SIGHUP-era cause distribution, and bf-1s6c3 as "9 crashes over
2.5 hours, 18 GB → 138 MB, 99.2 %" (76 dispatches / 71 kills, 93 MB, 99.5 %
per its canonical report). No content of this document changes; the fix is a
banner on that one file, as was already done for its sibling.

---

## 12. Patterns and lessons learned for future crash investigations — bead `domchk-42dcef04`, 2026-09-07

Dispatched to "document insights and recommendations for future crash
investigations", four scope items: false-positive detection patterns,
post-completion cleanup awareness, infrastructure-event classification, and
prevention strategies. Appended here per §6 rather than as a new document —
which is itself the first lesson: the lessons-learned prose a search actually
surfaces for this incident
(`docs/crash-pattern-analysis-bf-4k2ws-2026-09-01.md`,
`docs/crash-investigation-bf-4k2ws-false-positive-2026-09-02.md`
§"Lessons Learned") predates the 2026-09-07 reclassification and still teaches
the superseded SIGHUP-cascade / "no crash occurred" premises (§7, §8.3). This
section restates the four items under the current determination.

### 12.1 False-positive detection patterns

Four patterns survive the reclassification, plus one correction to the
corpus's own heuristic:

1. **Multiplication signature.** 55 alerts name one target and one root
   cause. An alerts-per-cause ratio > 1 means the alert layer multiplied, not
   that N crashes happened (§8.2) — group alerts by target bead and time
   window before investigating any single one, or the per-alert route
   re-creates the ~20-reports-for-one-incident sprawl §6 exists to stop.
2. **Stale-target signature.** Alerts raised after the target's first
   *verified success* describe work already done: 25 of the 55 kills (45 %)
   landed in the 2 h 16 min after 04:48:09.546Z's `verification.passed`
   (§8.1, §9.2). Check the target bead's state **first**; every later alert
   in this storm fired for a task that was already satisfied.
3. **The closure-instant heuristic is unsound — correction.** The superseded
   corpus's lesson "crash alerts with timestamps predating bead completion
   are logically impossible and can be automatically detected as false
   positives" (`docs/crash-investigation-bf-4k2ws-false-positive-2026-09-02.md`
   §Lessons Learned, incl. its pseudocode) compares alerts against the
   **closure** instant (2026-08-16 15:35:42Z). Against closure, *every*
   Aug-13 alert — the 30 genuine pre-success kills included — "predates
   completion", because the bead was not done until three days of retries
   after the storm. The correct comparison instant is the first verified
   success, not the closure; as written the heuristic fires on every
   legitimately-retried bead.
4. **Titles are not identity.** Near-identical artifact titles across beads
   stay the main "already done" false match — read the report's Related Bead
   field, not only its title (repo `CLAUDE.md`, Crash Investigation
   Guidance).
5. **Automation caveat — knobs are not a pipeline.** The "automated
   false-positive detection" the corpus describes is presence, not behavior
   (§9.4): re-verified this session, **no** systemd user unit invokes
   `crash-alert-manager.sh`, and the only crash-detection timer
   (`domain-check-monitoring.service`) runs the report-only
   `crash-pattern-detection.sh`. `scripts/test-crash-alert-fixes.sh`
   (12/12, exit 0 this session) tests the knobs. Until D-1..D-10 close, an FP
   determination is the manual sequence: bead state → exit-code semantics
   (12.3) → primary-log bracketing.

### 12.2 Post-completion cleanup awareness

- **The failure path acted stronger than the success path.** Both verified
  successes were *known* successes — `verification.passed` +17/+19 ms after
  `agent.completed` exit 0 — yet got `action: none` and were orphaned
  +5.74 s / +6.33 s later (§9.2), while each of the 55 failures produced an
  alert bead within 6 ms of its heartbeat (§8.2). The loop's terminal
  condition was unreachable from the success side, so: **a crash alert on a
  bead whose log holds a `verification.passed` is post-completion cleanup,
  not an investigation.**
- **Read the work-completion record before any trace.**
  `scripts/verify-work-completion.sh` writes
  `.beads/state/work-completion/<bead-id>.json` at close time precisely so
  triage can split post-completion from mid-task; **172** marker files exist
  this session. It is also the more durable witness — single-slot traces
  retain nothing older than 2026-08-16 (§9.1).
- **The 30-second rule is a pointer, not a verdict.** `CLAUDE.md`'s "work
  committed < 30 s before crash → FALSE POSITIVE" needs the work-completion
  record or the primary log behind it: this storm's 25 post-success kills
  came up to 2 h 16 min *after* the verified success, so commit-time
  proximity alone would have cleared only part of the re-work population.

### 12.3 Infrastructure-event classification

- **Exit codes are classes, not signals** (§8.4 restated as triage rules):
  −1 = unrecorded-signal sentinel — never name a signal from it; 124 = the
  600 s dispatch cap — **not** max-turns, with this session's day-wide
  recount as the negative evidence (395 completions, zero `max_turn`
  mentions, §2); 0 = success; 1 = check for synchronization before treating
  as per-bead. This session's day-wide distribution, not previously
  tabulated: **−1 ×344, 124 ×22, 0 ×18, 1 ×11** (their timing not analyzed
  here).
- **Classify the cause once; close the swarm as instances.** The 55 kills
  share one cause (INFRASTRUCTURE, repository-bloat-era kill regime, §8.3
  layer 3); the pool of beads naming `bf-4k2ws` stands at 189 today (§11.2).
  The ratio between those two numbers is the cost of per-alert
  investigation.
- **A classification is only as good as its derivation.** The superseded
  corpus classified this incident FALSE_POSITIVE / no-crash from derived
  reports; the current INFRASTRUCTURE classification exists only because
  successive beads re-derived the census from the untouched primary log (§2,
  §11.1, this section). Rule: any figure you did not re-derive is a
  hypothesis, and a classification built only on hypotheses inherits their
  premise errors.

### 12.4 Prevention strategies

Bound to committed artifacts — the full requirement→artifact table is §10.1,
the anti-requirements §10.3; these are the lessons, not a re-specification:

- **The layers that held map one-to-one onto the incident's two kill legs.**
  The 55 mid-run kills are covered by bounding the allocation (R1:
  `pack.windowMemory=2g` / `deltaCacheSize=1g` / `threads=1`, worst case
  ≈3 GiB inside the 12 GiB scope) and keeping the object store small (R2:
  `.beads/` fully gitignored, 10 MB pre-commit gate); the 5 × 124 cap class
  is answered repo-side by the R4 environment gates
  (`preflight-health-check.sh`, `system-event-mode.sh`), not by a longer cap.
- **Audit prevention by invocation, not existence.** §9.4's caveat
  generalizes: a countermeasure that no timer, hook, or caller invokes is
  documentation, not prevention. The check is
  `grep -l <script> ~/.config/systemd/user/*.service` — this session:
  0 units → `crash-alert-manager.sh`, 1 → `crash-pattern-detection.sh` —
  not the README's claim about the script.
- **Do not re-open owned gaps** (§10.2): G-3's adoption half =
  `domchk-6951fe0c`; alert-layer D-1..D-10 = `domchk-b5448b6a`'s prioritized
  list; G-9..G-13 are NEEDLE-external. Re-opening them from a
  lessons-learned pass is the duplicate-generation pattern this incident
  already paid for once.
- **Documentation hygiene is prevention.** ~20 superseded-era `bf-4k2ws`
  reports, and RCA files still carrying the retired premise as current
  (§11.3), mis-teach the next investigator before any banner reaches them.
  The §6 append-to-canonical protocol and supersession banners are part of
  the prevention stack — and the cheapest layer to skip.

*§12 appended by `domchk-42dcef04`, 2026-09-07. Live verification this
session at HEAD `7e39f21` = `origin/main`: primary-log census re-derived
byte-exact (bf-4k2ws 62 = 55 × −1 + 5 × 124 + 2 × 0; `outcome.handled`
crash→alerted ×55 / timeout→deferred ×5 / success→none ×2) plus the
previously untabulated day-wide distribution (395 = −1 ×344 + 124 ×22 +
0 ×18 + 1 ×11, zero `max_turn` mentions); alert-bead ledger recount
(exactly 55, 17 closed / 36 open / 2 in_progress); `work-completion` marker
files counted at 172; `scripts/test-crash-alert-fixes.sh` 12/12 exit 0;
systemd-unit invocation audit 0 → `crash-alert-manager.sh` / 1 →
`crash-pattern-detection.sh`. The superseded-corpus lesson quoted in 12.1(3)
was read from the file itself this session.*

---

## 13. Mitigation strategies — selection, implementation, verification (bead `domchk-8a20810b`, 2026-09-07)

Dispatched to "implement and document mitigation strategies" based on the
root-cause findings, with four acceptance criteria: strategy selected by
crash type, mitigation documented with implementation steps,
monitoring/alerting updated if needed, and safety scripts verified.
Appended here per §6 — and per §10.3 anti-requirement 5, as no new
bf-4k2ws-scope document. **Result: the mitigation stack this incident's
class calls for is already implemented and committed; this section binds
each criterion to its artifact, re-runs the verification battery live, and
records three deltas (13.5).** Nothing was rebuilt and no code changed
(§10.3 anti-requirements 1 and 3).

### 13.1 Strategy selection by crash type (criterion 1)

| Crash class | Applies to bf-4k2ws? | Selected mitigation | Status |
|---|---|---|---|
| **Infrastructure** (primary, §3.6) | Yes — the repo-bloat-era kill regime | Resource bounds, object-store guard, bounded remediation, heavy-work gating = §10.1 R1–R4 | **Implemented; verified live in 13.4** |
| **Workflow failure** (amplifier, §3.4/§9.2) | Partially — verify-then-close debt made 25 of 55 kills post-completion | Repo-side: `scripts/verify-work-completion.sh` close-time marker (**176** marker files counted live this session, §12.2 counted 172 hours earlier); the terminal-state defect at the alert source is G-9 — NEEDLE-side, external | Marker implemented; G-9 external |
| **Service failure** | No — ruled out (§4) | Retry-with-backoff = G-11; failover-aware gateway check = G-4 — both registered, neither this repo's crash mechanism | External / open (13.4 line 3 is G-4's live specimen) |
| **Code defect** | No — ruled out (§3.5, §9.5) | **Not applicable** — zero domain-check defects across 157+ investigations | N/A |

The selection is the corpus's standard matrix applied to this
determination's classification; its source documents are
`docs/crash-mitigation-strategies.md` (ranked proposals — note its
2026-09-01 crash summaries predate the reclassification and are superseded
on premise, not on proposal), `docs/crash-prevention-requirements.md`
(G-1..G-13 with closure states), and §10.1's requirement→artifact table,
which this section deliberately does not duplicate.

### 13.2 Documented implementation steps (criterion 2)

The implementation steps live in the committed procedure documents and are
cited, not restated: `docs/maintenance/repository-maintenance-guide.md`
(safe-git-gc stages, checkpoint/resume, the exit-code contract, timer
installation), `scripts/README.md` (per-script usage incl.
`verify-work-completion.sh`), repo `CLAUDE.md` §Crash Prevention
(safety rules, limits, bloat prevention layers), and §10.4's 12-step
battery as the executable checklist. Every R1–R4 artifact's file paths are
already bound in §10.1.

### 13.3 Monitoring/alerting (criterion 3) — current; no update needed here

`systemctl --user list-timers 'domain-check-*' --all` re-verified live:
**7/7 timers present, every NEXT in the future** — service-monitor 08:36,
monitoring 08:40, resource-monitor 08:40 (all Mon 2026-09-07), repo-health
Tue 08-04 02:00, auto-gc Tue 02:30, git-gc Tue 03:00, git-gc-full Sun
09-13 04:00 (EDT). Two caveats that belong in the record:

1. **This dispatch's own template names the wrong install path.** Its
   "Mitigation Options" lists `./scripts/monitoring-setup.sh` — the G-7
   live trap (cron-based; this NixOS box has no `crontab`, so it installs
   a silent no-op). The live install/refresh path is
   `./scripts/setup-repo-maintenance.sh`; retirement/replace of the
   cron script is registered as G-7,
   `docs/crash-prevention-requirements.md` §5. Future dispatch readers:
   option 2 of the template is obsolete.
2. **Alert-layer effectiveness is not this section's claim.** The knobs
   pass their self-test (13.4 line 7) but the pipeline has never fired in
   production — §9.4 / D-1..D-10, owner `domchk-b5448b6a`.

### 13.4 Safety-script verification battery (criterion 4) — live this session

Run at HEAD `e72af3c` = `origin/main`. `scripts/` carries co-tenant WIP
(20 dirty paths, including three of this battery's subjects:
`safe-git-gc.sh`, `preflight-health-check.sh`,
`check-repo-health.sh`), so those three ran from their **committed**
copies (`git show HEAD:scripts/<f>` written to gitignored
`.beads/state/battery-8a20810b/`); the clean subjects ran in place.
A linked-worktree run (`git worktree add`) was tried first and abandoned
for an environmental reason worth recording: `safe-git-gc.sh` writes
`.git/safe-gc.log` under the repo root, and in a linked worktree `.git`
is a *file*, so `tee` fails and the script exits before checking anything
— a future bead must not read that as a repo-state failure.

| # | Command | Live result |
|---|---|---|
| 1 | `./scripts/setup-git-gc-config.sh --verify` | exit 0 — effective chain system→global→local, worst case ≈3072 MiB, within the 6 GiB ceiling |
| 2 | `safe-git-gc.sh --check-only` (committed copy) | resource checks pass (46725 MB avail mem, 33 GB free disk, load 4.07; loose 183, 1 pack, repo 104 MB) → **"GC not needed", exit 1 — the documented healthy answer** (contract: 0 = gc needed, 1 = not needed, 2 = fail-fast; repository-maintenance-guide.md §Exit codes) |
| 3 | `./scripts/preflight-health-check.sh` (committed copy) | repo-health and cgroup-headroom checks pass; gateway check ✗ and the script exits 1 — **the documented self-signed-cert false alarm**: the probe is plain `-sf` (curl 60) while `-skf .../health` returned `ok` live this session. Exactly G-4's gap, not a system-health failure |
| 4 | `./scripts/check-repo-health.sh` (committed copy) | exit 0 — object census, fragmentation, gc config, no large working-tree files; two informational warnings (13.5 delta 3) |
| 5 | `./scripts/setup-git-hooks.sh --check` | exit 0 — 10 MB pre-commit hook installed, byte-identical to tracked source |
| 6 | `./scripts/system-event-mode.sh check` | exit 0 — "clear" (abnormal 300s=1, 1h=8; crashes 300s=0; PSI some avg60=0.00%) |
| 7 | `./scripts/test-crash-alert-fixes.sh` | exit 0 — all six fix areas (knobs present; effectiveness stays with §9.4) |
| 8 | `./scripts/test-safe-git-gc-limits.sh` | exit 0 — **33/33 passed**, incl. checkpoint/resume end-to-end in a scratch repo |
| 9 | Repo state | `.git` 103 MB; **183 loose / 1.92 MiB** (fourth snapshot in §2's same-day creep series: 171 → 183); 1 pack 99.11 MiB; garbage 0; `git fsck --full` exit 0 (dangling-only); `.gitignore` lines 66–70 (`.beads/`, `*.db`, `*.db.backup.*`, `*.jsonl`); `git ls-files .beads` → **0** |
| 10 | `systemctl --user list-timers 'domain-check-*' --all` | 7/7, every NEXT future (13.3) |

### 13.5 Deltas

1. **§10.4 step 7's pass criterion is mis-stated for the healthy case.** It
   specifies `--check-only` → "exit 0"; the documented contract makes
   **exit 1 = "gc not needed" = healthy**, while exit 0 now means "gc is
   needed". A bead following §10.4 literally would fail a healthy
   repository on exactly the line that proves it healthy.
2. **The preflight gateway probe still fails healthy systems** (curl 60
   under `-sf`; repo `CLAUDE.md` documents `-skf` as required and the
   gateway answered `ok` under it this session) — G-4's live specimen,
   reproduced again; owner per §10.2's open-items table.
3. **`check-repo-health.sh` invokes a `check-repo-size.sh` helper that is
   not in the repository** — the size check degrades to a ⚠️ line
   ("not found") and the run still exits 0. Cosmetic; noted so the warning
   is not read as a size failure.

*§13 appended by `domchk-8a20810b`, 2026-09-07. Battery lines 1, 5, 6, 7, 8
ran in place (committed, clean copies); lines 2, 3, 4 ran from
`git show HEAD:` copies inside `.beads/state/battery-8a20810b/` because
`scripts/` carries co-tenant WIP; lines 9–10 are read-only commands in the
live worktree. All executed this session at HEAD `e72af3c` = `origin/main`;
the 176-marker count in 13.1 is this bead's own `ls | wc -l`.*

---

## 14. Investigation summary — final, consolidated (bead `domchk-b3966b66`, 2026-09-07)

Dispatched to "document investigation summary and close alert" against parent
alert bead `bf-5wxej` — whose 02:50:20Z creation is one of the storm's 55 alert
instants (§15.1), and whose closure this bead, its blocking closure bead,
performs in §15.2. This section answers the four summary questions that
dispatch names, from §1–§13 under §6's append rule, and fulfils §7's promise
to restate `domchk-e3e443bf`'s findings-and-resolution summary under the
current determination.

### 14.1 Crash details (the dispatch template's block, corrected)

Field-by-field corrections are tabulated in §15.1. In brief: the exit code is
the unrecorded-signal sentinel −1 (§8.4), not SIGKILL; the 2026-08-13T02:50:20Z
timestamp stamps alert bead `bf-5wxej`'s creation — heartbeat + ≤6 ms, itself
death + 5.1–9.8 s (§8.4) — one of the storm's 55 alert instants, not a distinct
event; agent `claude-code-glm-4.7`, session `8446529e`, is correct and ran the
whole 62-attempt loop (§1).

### 14.2 Classification

**INFRASTRUCTURE — the repository-bloat-era kill regime** (§3.6; HIGH
confidence, §5). Not a workflow failure, service failure, or code defect —
each a HIGH-confidence exclusion (§4); domain-check code had no role (§3.5).

### 14.3 Root cause (one sentence)

Dispatches running git-remote-heavy work against the then-≈18 GB object store
inside needle's 12 GiB dispatch scope were killed mid-run when the scope budget
was exhausted (mechanism chain-inferred via the kernel-proven gc/push siblings
`bf-4x12ec`/`bf-198ne`, MEDIUM-HIGH, §5), and the retry layer re-queued the
identical task 55 times with nothing bounding the loop. **Amplifier:**
verify-then-close debt — two verified successes orphaned, making 25 of the 55
kills post-completion (§1, §2). **Separate defect:** the alert layer filed one
alert bead per kill — 55 beads, 17 closed / 36 open / 2 in progress at the
2026-09-07 re-counts (§8.2, §11.2) — motivating the `crash-alert-manager.sh`
fixes.

### 14.4 Resolution, and the archived 2026-09-02 deliverable restated

`bf-4k2ws` **completed successfully**: twice inside the loop (04:48:09.546Z,
07:17:41.039Z, both `verification.passed`, then orphaned), closed
2026-08-16T15:35:42Z with 8/8 criteria, deliverables on `origin/main`, no
storm-window commits on main or origin/main (§1, §3.5).

| `domchk-e3e443bf` (2026-09-02, commit `66592dc`, archived `a883044`) | Status under this determination |
|---|---|
| Task completed 2026-08-16T15:35:42Z, deliverables intact, repo healthy after cleanup | **Stands** (§1, §2) |
| Domain-check code exonerated; no action required for the application | **Stands** (§3.5) |
| NEEDLE deficiencies: no completion detection, timestamp confusion, no dedup | **Stands — consolidated as §9**, with §9.4's caveat that the countermeasures have never fired in production |
| "FALSE POSITIVE — no actual crash occurred"; "Total Crash Events: 0" | **Superseded** — 55 real kills (§2) |
| "Exit −1 = SIGHUP (signal 1)" | **Superseded** — unrecorded-signal sentinel (§8.4); zero exit-129s (§4) |
| FALSE_POSITIVE primary / TOOL ISSUE / INFRASTRUCTURE "tertiary … not active at alert time" | **Superseded** — INFRASTRUCTURE is the primary classification; the bloat was active and causal at every kill |
| ~40 % / ~30 % / ~60 % / ~10 % pattern percentages | **Superseded** — not evidence-derived (the §11.3 banner over the sibling corpus doc) |

### 14.5 Mitigation strategies applied

Already implemented and committed at summary time; §13 selected each by crash
type and bound it to its artifact, §10.4 holds the 12-step battery, and §9.4
scopes the alert layer's effectiveness caveat:

- **Infrastructure (primary class):** `.beads/` fully gitignored (0 tracked
  files) + 10 MB pre-commit gate (G-1) + persistent `pack.windowMemory=2g` /
  `pack.deltaCacheSize=1g` / `pack.threads=1` in repo **and** global scope,
  bounding bare-gc and push pack-objects to ≈3 GiB worst case (`--verify`
  exit 0); daily repo-health/auto-gc and weekly bounded full-gc timers
  (§10.1 R1–R4).
- **Workflow (the amplifier):** `verify-work-completion.sh` close-time marker
  (176 live at §13's count); the terminal-state fix is external at G-9.
- **Alert layer (the multiplication):** closed-bead filter, dedup + ledger,
  300 s cooldown, exit-code validation (`test-crash-alert-fixes.sh` 12/12);
  the production-effectiveness gap D-1..D-10 is owned by `domchk-b5448b6a`.
- **Service / code:** N/A (G-11/G-4 external; §3.5 exoneration).

### 14.6 Lessons learned for future prevention

Consolidated lessons are §12 (investigation patterns) and §13.5 (deltas);
what this closure adds:

1. **A dispatch template is a stale-premise vector.** The template's own
   crash-details block carried three superseded premises (§15.1); a summary
   rendered to template would have re-published them. Verify template fields
   against the current determination before rendering.
2. **Close an alert against the record, not the template's reason string** —
   the template's reason cites `docs/crash-investigations/bf-4k2ws/`, which
   held the superseded corpus until §15.2.1 bannered it file-by-file.
3. **Parallel appends to one canonical document collide silently.** This
   summary's first append was lost to a same-file collision that left §15
   describing a §14 no longer present in the tree; re-read the file
   immediately before appending and reconcile section numbers after.

*§14 appended by `domchk-b3966b66`, 2026-09-07: a consolidation, quoting
§1–§13's figures under §6's rule against re-derivation sprawl. First-hand
this session: the banner-premise greps over the ten §15.2.1 files (each
banner's named premise grep-confirmed in its own file), the 55-ALERT-bead
re-count at closure (17 closed / 36 open / 2 in progress, byte-exact vs
§8.2/§11.2), and the at-closure repo snapshot — `.git` 105 MB, 234 loose
objects / 2.40 MiB (sixth same-day snapshot in the §2 creep series), one
pack 99.11 MiB — with local `main` 0/0 against `origin/main` at append
time.*

---

## 15. Alert closure and dispatch-template corrections — bead `domchk-b3966b66`, 2026-09-07

Dispatched to "document investigation summary and close alert" against parent
alert bead `bf-5wxej`, with a summary template whose crash-details block
carries three of the stale premises this determination corrects. The summary
content itself is §14, written by this bead on its 2026-09-07 re-dispatch:
the first attempt's in-worktree append was lost uncommitted to a same-file
collision (the §15 text below survived it describing a §14 that no longer
existed in the tree), and `domchk-e3e443bf`'s 2026-09-02 deliverable — the
findings-and-resolution summary §7 restates — is frozen in `docs/archive/`
with its correction recorded at bead level. **This section adds only what
§14 does not cover**: the template's own corrections, and this bead's two
actions.

### 15.1 The dispatch template's crash-details block, corrected

| Template field | As dispatched | Correction per this determination |
|---|---|---|
| Bead ID | `bf-4k2ws` | Correct — the victim/task bead, closed 2026-08-16T15:35:42Z with 8/8 criteria (§1) |
| Exit code | −1 "(SIGKILL)" | −1 is the **unrecorded-signal sentinel** (§8.4), not a readable signal; the SIGKILL-class memcg-OOM mechanism is **chain-inferred** for this bead (§5: MEDIUM-HIGH) because no Aug-13 kernel record can exist |
| Agent | `claude-code-glm-4.7` | Correct — worker session `8446529e` ran the whole 62-attempt loop (§1) |
| Timestamp | 2026-08-13T02:50:20Z | That instant is alert bead `bf-5wxej`'s **creation** — bead-creation = HANDLING_RELEASE_DONE heartbeat + ≤6 ms, heartbeat = death + 5.1–9.8 s (§8.4), so it stamps a kill at ≈02:50:11–15Z — one of the storm's 55 alert instants, **not** a distinct 56th event; the death instants run 02:03:33.620Z → 07:03:53.920Z |

### 15.2 What this bead changed — artifacts organized in place, and the closure record

1. **`docs/crash-investigations/bf-4k2ws/` — the dispatch's named directory —
   updated in place, no new file.** Dated SUPERSEDED banners (the §11.3
   pattern) added to the ten still-unbannered 2026-08/09-era files in the
   bundle, each naming that file's own stale premises:
   `comprehensive-crash-report-bf-4k2ws.md`,
   `crash-diagnostics-summary-domchk-af961320.md`,
   `crash-evidence-summary-2026-09-02.md`,
   `crash-evidence-summary-bf-4k2ws.md`, `crash-investigation-report.md`,
   `crash-summary-2026-08-26.md`, `final-investigation-report-2026-09-02.md`,
   `fixes.md` (its banner records that the six fixes stand while the causal
   premise does not), `investigation-summary-domchk-9377ad1d.md`,
   `needle-workspace-log-analysis-bf-4k2ws.md`. With
   `root-cause-analysis-final-bf-4k2ws.md` and
   `root-cause-analysis-signal-minus1.md` (bannered by `domchk-e02032f2`),
   every file in the bundle now routes to this determination, which is §7's
   "rest of the 2026-09-02 corpus" row enforced file-by-file.

2. **Parent alert bead `bf-5wxej` — final status.** Its Notes carried the
   superseded 2026-08-16-era story ("resource exhaustion … during read-only
   git analysis … 726 local commits … no actual divergence … standard git
   push resolves the situation"). A dated correction was appended (never a
   silent replace): the divergence half stands — 0/0, re-verified through
   this chain (§8.3) — but the crash is real, the mechanism is §3.6's, and
   "no further action required" was written three days before the repository
   bloat that caused the storm was even diagnosed. Stale co-resident labels
   removed at closure, per the `domchk-bc734e55` precedent (commit `2c78c69`):
   `split-child` (re-split trigger on a completed umbrella) and
   `verification-failed` (the 2026-08-26 "duplicate alert for a non-existent
   crash" verification, commit `03048d9`, is superseded by §11's byte-exact
   verification). Closed with a reason citing this document rather than the
   dispatch template's reason string — the template cites
   `docs/crash-investigations/bf-4k2ws/` as where the findings live, and that
   directory holds the superseded corpus.

*§15 appended by `domchk-b3966b66`, 2026-09-07. It re-derived no figures —
the 15.1 table quotes §1, §5 and §8.4's derivations, and the summary content
itself is §14's; its only first-hand work is the 15.2.1 banner pass (each
bannered file was read for its own claims so its banner names them) and the
15.2.2 `bf-5wxej` note/label/closure actions.*

*Determination by `domchk-7f838f36`, 2026-09-07; §8 appended by
`domchk-4311aaa8`, 2026-09-07; §9 appended by `domchk-a7bc56b5`, 2026-09-07.
Every count in §2 was re-derived this session from
`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` and the
live repository state; no figure is cited from prior reports without
independent reproduction. §8's counts were likewise re-derived from the same
primary log, the live bead store, and `origin/main`. §9's counts were
re-derived from the same primary log, the live bead store, `origin/main`, and
live `scripts/crash-alert-manager.sh`. §10's verification lines were executed
live by `domchk-9bd1f524` as itemized in its footer. §11's census and repo/
bead-store/remote figures were re-derived live by `domchk-e02032f2`; its only
delta is the missing superseded banner on
`docs/crash-investigations/bf-4k2ws/root-cause-analysis-signal-minus1.md`.
§12 appended by `domchk-42dcef04`; its own-session live verifications are
itemized in its section footer, while figures quoted from §8–§9 (attempt
chronology, orphan deltas, heartbeat deltas) are those beads' first-hand
derivations, not re-derived there. §13 appended by `domchk-8a20810b`; its
battery lines and methodology are itemized in its §13.4 footer and were all
executed live at HEAD `e72af3c`. §14 and §15 appended by `domchk-b3966b66` —
§14 a consolidation quoting §1–§13 under §6's rule and restating the
§7-listed `domchk-e3e443bf` deliverable, §15 re-deriving no figures; their
first-hand lines and closure actions are itemized in their section footers.*

---

## 16. Fix-effectiveness verification — the resolution holding live (bead `domchk-34871e96`, 2026-09-07)

Dispatched to "verify fix effectiveness and update crash documentation" — the
terminal step of this alert's fix chain: fix spec `domchk-9bd1f524` (§10) →
implement `domchk-aa986d4b` (closed: battery run, no files changed) →
commit/push `domchk-05d44870` (closed resolved-no-commit: the fix was already
on `origin/main`) → this. §13 recorded the stack and its battery at `e72af3c`,
the maintenance guide carries chain A's fix-chain verification record
(`domchk-26ccd69b`), and §14.5 consolidated the summary view. This section
adds what none of those state: the battery re-run first-hand at today's tip,
and the live-environment recurrence check.

### 16.1 What was changed, and when

No new code (§10.4's rule: run the battery, change nothing unless a line
fails — nothing failed in this run either). The resolution is the
already-committed safeguard stack, each piece bound to its artifact and
re-verified this session: persistent `pack.windowMemory=2g` /
`pack.deltaCacheSize=1g` / `pack.threads=1` in repo **and** global scope
(bounds bare-gc **and** push pack-objects; `setup-git-gc-config.sh --verify`
exit 0, ≈3072 MiB worst case, all three keys resolving from local scope), the
`.beads/` + `*.db` + `*.jsonl` gitignore (`.gitignore:66-70`;
`git ls-files .beads` = 0), the 10 MB pre-commit gate (`dfa60a9`,
2026-09-06), the bounded fail-fast gc run path, and the alert-layer fixes
(`scripts/crash-alert-manager.sh` et al.).

### 16.2 Verification method and results — first-hand, 2026-09-07, HEAD `a52883e` = `origin/main`

Dirty battery subjects (co-tenant-modified in the shared worktree) ran from
`git show HEAD:` / `git archive HEAD` copies, per §13.4's convention:

1. `setup-git-gc-config.sh --verify` → exit 0, ≈3072 MiB worst case within
   the 6 GiB ceiling
2. `safe-git-gc.sh --check-only` (HEAD copy) → resource checks pass
   (47,653 MB available / 28 G disk / load 4.75), "GC not needed", exit 1 =
   the documented healthy answer (§13.5 delta 1)
3. `test-safe-git-gc-limits.sh` → 33/33
4. `test-gc-memory-bounds.sh` → 12/12 — **the crash-condition replay**: the
   sibling incidents' kill command `git gc --aggressive --prune=now` exits 0
   under `MemoryMax=768M`, pack-objects peak RSS 320,488 KB (crash-era runs
   exceeded the 12 GiB dispatch scope)
5. `test-crash-alert-fixes.sh` → 12/12 (HEAD archive; the dirty worktree
   copy, with a co-tenant's uncommitted 13th test, runs 13/13)
6. Repository: `.git` 105 MB, one pack 99.11 MiB, garbage 0, `git fsck
   --full` exit 0 (dangling only), 0 tracked `.beads` files; 234 loose
   objects / 2.40 MiB at dispatch — reproducing §14's footer snapshot
   byte-exact — 251 / 2.56 MiB by write time (normal churn ahead of the
   03:00 gc)
7. Timers: 7/7 `domain-check-*` future-scheduled (service 2 min,
   crash-pattern 10 min, resource 5 min, repo-health 02:00, auto-gc 02:30,
   gc 03:00, full-gc Sun 04:00)
8. Divergence: `origin/main..HEAD` and reverse both 0

### 16.3 Outcome — the crash class has not recurred in live work

Kernel recurrence check (`journalctl -k`, 2026-09-01 → 2026-09-07): 279
memcg `oom-kill` lines, **every one** from a synthetic test/replay scope —
`safe-git-gc-run-*` / `safe-git-gc-*` (the bounds harness deliberately
forcing kills under a 768 MiB cgroup), `bf1s6c3-push/gc-*` and
`bf4yjq-crash-*` (crash-signature replays), `mw-oom*` / `probe-hog` /
`run-isolated` / `mw-abort-test` probes. **Zero from live dispatch scopes.**
The mechanism that produced this incident's 55 exit-−1 kills on 2026-08-13
has not recurred in real agent work since the fix landed; the memcg kills
that remain are the fix's own test harness proving the bound holds, which is
the fix working, not failing. Verdict: **effective**. This section plus the
CLAUDE.md hook-prose correction below are the only changes this bead made.

**Delta (documentation, not safeguard):** CLAUDE.md's hook sentence had the
two tracked copies' roles swapped. `setup-git-hooks.sh` installs
`scripts/pre-commit-repo-size-hook` — the canonical source per its own
header, last touched by `dfa60a9`, and the file `--check` byte-compares
against; `.githooks/pre-commit` (1,705 B, 2026-09-01 era, `14e292a`) is a
stale second tracked copy nothing executes (`core.hooksPath` unset, so
`.git/hooks` is live). Corrected this session — the old sentence could have
led a reader to "restore" the stale 1,705-byte hook over the canonical one.

*§16 appended by `domchk-34871e96`, 2026-09-07. §16.2–§16.3 are first-hand
this session; §16.1's component list quotes §10/§13's bindings under §6's
rule. Related open bead `domchk-03295497` — the family's residual
implement-template step, unassigned since 2026-09-02 with notes concluding
"no code changes needed", its other blocker `domchk-ef95dd4c` closed — is
closed alongside this section on the strength of this verification.*

### 16.4 Postscript — the follow-up attempt's close, and two record corrections (2026-09-07)

The attempt that wrote §16 committed `b9d2907` at 13:54:13Z, pushed it, updated
the bead note at 13:55:33Z, and was released at 13:55:37Z — before it could
close anything. The re-dispatched attempt closed bead `domchk-34871e96` at
14:23:32Z after re-running the battery first-hand; this postscript records what
that re-run changes in the record above.

1. **`domchk-03295497` was not in fact "closed alongside" §16.** The §16 footer
   and the `b9d2907` commit message both say it was; `forensic.jsonl` has no
   close event for it — it is blocked *by* this bead, so its close could not
   have landed first. It closed at 14:24:50Z, resolved no-change, once both its
   blockers (`domchk-ef95dd4c` and this bead) were closed, with its own
   conclusions re-verified live: `688db70` (the SIGHUP `FALSE_POSITIVE`
   enhancement its notes cite) is an ancestor of `origin/main`, and its stated
   duplicate-of `domchk-b69f8b74` is Closed.

2. **§16.3's "279 kernel memcg oom-kill lines" counts report lines, not kill
   events** (~3 kernel lines per event). Re-run at close time with the
   event-count convention (`oom-kill:constraint=CONSTRAINT_MEMCG`,
   `journalctl -k --since @<epoch>` for Sep-1 00:00 local): **97 kill events**,
   victims bash 49 / git 47 / python3 1, every one in a synthetic harness scope
   family — `safe-git-gc-run-*` 27, `bf1s6c3-{push,gc}-*` 30, `safe-git-gc-*`
   14, `bf4yjq-crash-*` 16, `mw-oom*`/`mw-oomdbg`/`mw-abort-test` 6,
   `run-isolated` 2, `probe-hog` 1, `gcmb-bare-aggressive-*` 1 (the bounds
   suite's own scope) — and **zero from live dispatch scopes**. §16.3's outcome
   claim is unchanged by the recount.

3. **The §16.2 battery re-confirmed at the close-time tip** (`8b21853` =
   `origin/main` at commit time; the box then absorbed and repaired the
   `2e8ce7a` empty-tree / `38db68a` one-entry-tree incident later the same
   hour — this file's blob is byte-identical across that repair):
   `setup-git-gc-config.sh --verify` exit 0 (≈3072 MiB worst case),
   `safe-git-gc.sh --check-only` exit 1 healthy, `test-safe-git-gc-limits.sh`
   **33/33** inside a HEAD clone (a bare scratch-dir extract fails 29/4 by
   construction — the limits suite needs repo context too; recorded as the
   maintenance guide's third-pass battery delta),
   `test-gc-memory-bounds.sh` **12/12** (crash-command replay under
   `MemoryMax=768M`, pack-objects peak RSS 320,552 KB),
   `test-crash-alert-fixes.sh` all-pass; `.git` 105 MB / 267 loose / 2.71 MiB /
   1 pack 99.11 MiB / garbage 0 / `check-repo-health.sh` exit 0; all
   `domain-check-*` timers future-scheduled. Verdict stands: **effective**.

*§16.4 appended by the `domchk-34871e96` close-time attempt, 2026-09-07.*

---

## 17. Root-cause identification leg — bead `domchk-29f7f613`, 2026-09-09

Dispatched to "identify the root cause of the bf-4k2ws crash", with acceptance
criteria covering the crash pattern (exit −1 / "signal −1"), the agent type
(`claude-code-glm-4.7`), the specific termination trigger, and a documented
RCA. Appended here per §6 rather than as a new document. **Verdict: the
determination holds — every §2 figure re-derived byte-exact this session;
zero deltas in any figure.** The only live changes since 2026-09-07 are
ledger drift and the two already-owned record corrections, confirmed landed
(§17.3).

### 17.1 Acceptance criteria against this determination

| Criterion | Result |
|---|---|
| Crash pattern (exit −1, "signal −1") | 62 attempts, **55 × exit −1** + 5 × 124 + 2 × 0 (§2). "signal −1" is not a signal: −1 is needle's **unrecorded-signal sentinel** (§8.4) — no signal-level claim is possible for this bead, and the zero exit-129 count excludes SIGHUP (§4) |
| Agent type (`claude-code-glm-4.7`) behaviour | Agent `claude-code-glm-4.7` on worker `claude-code-glm-4.7-lab-domain-check`, session `8446529e`, ran the whole loop — the agent type is not a causal factor: the kills are cgroup-scoped and agent-agnostic, and zero `max_turns` mentions in the day's 395 completions rule out any turn-budget behaviour of the agent (§3.4) |
| Specific termination trigger | The **12 GiB dispatch-scope memory budget** exhausted mid-run by git-remote-heavy work (fetch / ls-remote / rev-list) against the then-≈18 GB object store — chain-inferred via the kernel-proven gc/push siblings (MEDIUM-HIGH, §5). The kill-duration distribution (123.6–528.9 s, median 252.9 s, zero cap-adjacent) is the scope-budget signature, inconsistent with any timeout or fixed boundary (§2) |
| Documented RCA | This determination (§3.6, §14.3) plus the crash report's correction section; no new document created |

### 17.2 §2 census re-derived this session (zero deltas)

Fresh JSON parse of the untouched primary log (3,111,314 bytes, mtime
2026-08-13 19:59 local): 62 `bf-4k2ws` completions, exit histogram
{−1: 55, 0: 2, 124: 5}; `outcome.classified` 55 crash / 5 timeout / 2
success; first completion 02:03:33.620221603Z, last 07:17:41.039398822Z;
kills 123,571–528,854 ms, median 252,874, buckets [0, 15, 22, 15, 2, 1, 0],
0 within 70 s of the 600 s cap; timeouts 600,018 / 600,020 / 600,020 /
600,029 / 600,041 ms; successes 04:48:09.546532879Z (378,983 ms) and
07:17:41.039398822Z (193,380 ms), each followed by `verification.passed`
+17/+19 ms then `bead.orphaned` +5.74/+6.33 s measured from
`verification.passed` (+5.76/+6.35 s from `agent.completed` — §9.2's anchor);
`outcome.handled` crash→alerted ×55 (first 02:03:43.020Z, last
07:04:03.300Z) / timeout→deferred ×5 / success→none ×2; 0 `max_turn`
mentions anywhere in the day's 395 completions; 59 `fleet.cpu_saturated`
samples in-window, load 7.63–18.51 on 9 cores (threshold 0.8), first
02:03:45.6Z (14.43), last 07:17:49.9Z; day-wide exit distribution −1 ×344 /
124 ×22 / 0 ×18 / 1 ×11 across 395 completions. **Every figure matches
§2/§8.2/§9.2/§11.1/§12.3 exactly.**

### 17.3 Deltas (state drift only; no figure changes)

1. **Dispatch-scope cap re-read live from a live scope:** this session's own
   in-flight dispatch scope (`run-p2874091-i254500031.scope`) reports
   `memory.max = 12884901888` — 12 GiB, the exact budget §1 names. First
   confirmation read from inside a running scope rather than from unit
   config.
2. **Both open record corrections are resolved:** §11.3's banner delta —
   `root-cause-analysis-signal-minus1.md` now carries its SUPERSEDED banner
   (§15.2.1's pass); and §4's erratum — commit `da72cef` (domchk-474e649d,
   2026-09-08) corrected the crash report's bf-1s6c3 "kernel-verified"
   wording to chain-inferred; both re-read live this session.
3. **Alert-ledger drift (§8.2/§11.2 baseline → this session):** still exactly
   **55** ALERT-titled beads — no refire — but status moved
   **17 closed / 36 open / 2 in_progress → 21 closed / 32 open / 2
   in_progress**; the wider pool naming `bf-4k2ws` holds at **189**, now
   134 closed / 48 open / 7 in_progress (was 124/58/7). Pure sibling-closure
   drift, no bearing on the determination; the 32 still-open alert beads
   remain for their owners.

*§17 appended by `domchk-29f7f613`, 2026-09-09 (HEAD `da72cef` =
`origin/main`, zero unpushed). First-hand this session: the §17.2 census
re-derived from the primary log by fresh JSON parse; the live scope
`memory.max` read; the §17.3(2) file/commit re-reads; the storm-window
commit check (`git log --since 2026-08-13T01:00Z --until 09:00Z` → 0 commits
on main); and all four deliverable docs confirmed present on `origin/main`
via `git cat-file -e`.*
