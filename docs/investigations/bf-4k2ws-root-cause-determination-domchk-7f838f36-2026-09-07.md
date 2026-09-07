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

---

*Determination by `domchk-7f838f36`, 2026-09-07; §8 appended by
`domchk-4311aaa8`, 2026-09-07; §9 appended by `domchk-a7bc56b5`, 2026-09-07.
Every count in §2 was re-derived this session from
`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` and the
live repository state; no figure is cited from prior reports without
independent reproduction. §8's counts were likewise re-derived from the same
primary log, the live bead store, and `origin/main`. §9's counts were
re-derived from the same primary log, the live bead store, `origin/main`, and
live `scripts/crash-alert-manager.sh`. §10's verification lines were executed
live by `domchk-9bd1f524` as itemized in its footer.*
