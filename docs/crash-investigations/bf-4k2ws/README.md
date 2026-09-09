# bf-4k2ws — Crash Investigation and Resolution: Final Report

**Report date:** 2026-09-09
**Author bead:** `domchk-aea1baca` (final report / knowledge capture)
**Target bead:** `bf-4k2ws` — "Analyze divergent Forgejo and GitHub branch states"
**Classification:** **INFRASTRUCTURE** — repository-bloat-era kill regime, with a
separate (fixed) false-positive defect in the alert layer
**Status:** ✅ RESOLVED — target completed and closed 2026-08-16; kill mechanism
remediated and re-verified live 2026-09-09

> **Read this first — corpus router.** This directory holds twelve 2026-08/09-era
> investigation files that all **predate the 2026-09-07 reclassification** and carry
> dated SUPERSEDED banners (§8 below maps them). The canonical, current record for
> this incident is:
>
> **`docs/investigations/bf-4k2ws-root-cause-determination-domchk-7f838f36-2026-09-07.md`**
> (16 append-only sections, §§1–17; every figure re-derived from the primary log).
>
> The older corpus taught a superseded premise — "FALSE POSITIVE, no crash occurred,
> SIGHUP cascade." The crashes were real. This README states the current
> determination and routes to it; it re-derives no figure that the determination
> already derived, and its provenance is itemized in §9.

---

## 1. Executive summary

On 2026-08-13, bead `bf-4k2ws` — a read-only git branch-divergence analysis — was
claimed at 02:01:29.710Z and then ran a **62-attempt loop on one worker session**
(`8446529e`, agent `claude-code-glm-4.7`) from 02:03:33Z to 07:17:41Z:
**55 attempts killed mid-run (exit −1), 5 hit the 600 s dispatch cap (exit 124),
and 2 completed successfully (exit 0)**. The task itself **completed**: both
successes (04:48:09.546Z, 07:17:41.039Z) passed verification, and the bead closed
2026-08-16T15:35:42Z with all 8 acceptance criteria met. No work was lost and no
domain-check code defect exists.

| Question | Answer |
|---|---|
| Did a crash occur? | **Yes** — 55 genuine mid-run kills. The 2026-09-02 corpus's "no crash occurred" premise is superseded |
| Root cause | Dispatches running git-remote-heavy work against the then-**≈18 GB** object store inside needle's **12 GiB dispatch scope** were killed when the scope's memory budget was exhausted; the retry layer re-queued the identical task 55 times with nothing bounding the loop |
| Mechanism proof | **Chain-inferred** for this bead (no Aug-13 kernel record survives the Aug-14 reboot), via the kernel-proven gc/push siblings `bf-4x12ec` / `bf-198ne` — confidence MEDIUM-HIGH |
| Amplifier | **Verify-then-close debt:** both verified successes were orphaned (`bead.orphaned`) instead of closing, so 25 of the 55 kills were post-completion re-work |
| Separate defect | Pre-dedup needle (< 0.4.2) filed **one alert bead per kill** — 55 ALERT beads, no fingerprint/dedup/cooldown. The false-positive element is confined to this alert layer |
| Co-factor | Continuous host CPU saturation: 59 `fleet.cpu_saturated` samples in-window, load 7.63–18.51 on 9 cores (threshold > 7.2) |
| Code defect | **None** — domain-check code had no role (re-verified §3.5 of the determination; zero `max_turns` mentions in the day's 395 completions) |
| Resolution | Repository repaired (~18 GB → 106 MB as of this writing), pack-memory bounds deployed repo-wide and globally, safe-gc stack + timers live, alert-layer fixes committed with test suites — battery re-run green 2026-09-09 (§5.4) |

**Why this incident matters beyond one bead:** the ratio between *one* root cause
and *55 alert beads* (plus a downstream pool of ~189 beads naming `bf-4k2ws`,
§17.3 of the determination) is the cost of per-alert investigation without
dedup — and of investigating from derived reports instead of the primary log.

---

## 2. Timeline of events

### 2.1 Storm night — 2026-08-13 (all times UTC; from the primary worker log)

| Instant | Event |
|---|---|
| 01:57:53.592Z | `bf-4k2ws` created. **Bead-existence instant, not a crash instant** — the oldest timestamp error in the superseded corpus |
| 02:01:22.561Z | Predecessor `bf-1s6c3` completes exit 0 on the same worker/session — itself orphaned 5.2 s later (the same verify-then-close debt); its own 71-kill storm (Aug-12 21:31Z → Aug-13 02:01Z) is the same kill regime |
| 02:01:29.710Z | `bf-4k2ws` claimed — **7.15 s** after the predecessor's completion |
| 02:03:33.620Z | Attempt 1 killed (exit −1, 123.6 s in). Storm begins |
| 02:03:40.611Z | First `HANDLING_RELEASE_DONE` heartbeat; first alert bead written **6 ms** later — one-alert-per-kill starts here |
| 02:03 → 04:48 | Attempts 1–32: **30 mid-task kills** + the night's first 2 dispatch-cap timeouts (attempts 16, 17 — 600.0 s each) |
| 04:48:09.546Z | Attempt 33 — **first genuine success** (exit 0, 379.0 s); `verification.passed` 17 ms later |
| 04:48:15.306Z | `bead.orphaned` — the verified success is discarded back onto the queue; the loop does not end |
| 04:48 → 07:14 | Attempts 34–61: **25 post-completion kills** re-doing satisfied work + 3 more cap timeouts (attempts 58, 59, 61) |
| 07:03:53.920Z | Last kill (attempt 60, 528.9 s — the longest); its alert fires 07:04:03.300Z |
| 07:17:41.039Z | Attempt 62 — **second genuine success** (exit 0, 193.4 s); verified, then orphaned again 6.4 s later |
| — | **Attempt census: 62 = 55 × exit −1 + 5 × 124 + 2 × 0.** Kills ran 123.6–528.9 s, median 252.9 s, zero cap-adjacent; `outcome.handled`: crash→alerted ×55, timeout→deferred ×5, success→none ×2 |

The alert instant that seeded this investigation chain, `bf-15k67`
(`2026-08-13T02:33:47.409682217Z`), is **attempt 8**: kill at 02:33:41.384776124Z
(173.9 s run) → heartbeat 6.025 s after death → bead row 11.5 µs after the
heartbeat. Every "crash timestamp" in a dispatch of this era is such a heartbeat,
not the death.

### 2.2 Task completion and investigation era

| Date | Event |
|---|---|
| 2026-08-16 15:35:42Z | `bf-4k2ws` **closed**, 8/8 acceptance criteria met; deliverables entered git in the same-day squash `c27899f` (the storm window itself holds **zero** commits on `main`/`origin/main` — re-verified repeatedly, most recently 2026-09-09) |
| 2026-09-02 | First comprehensive report (`docs/crashes/bf-4k2ws-crash-report.md`, investigation `domchk-dd05bc9c`): classified FALSE_POSITIVE / "no crash occurred" / SIGHUP cascade — premise later superseded, alert-fix recommendations stood |
| 2026-09-07 | **Reclassification** (commit `ef39024`, bead `domchk-3fca6de4`): 62-attempt census recounted from the untouched primary log → INFRASTRUCTURE, alert-layer FP only |
| 2026-09-07 | Canonical determination authored (`domchk-7f838f36`, §§1–7) and extended append-only by the split chain: chronology §8 (`domchk-4311aaa8`), NEEDLE deficiencies §9 (`domchk-a7bc56b5`), fix spec §10 (`domchk-9bd1f524`), verification §11 (`domchk-e02032f2`), lessons §12 (`domchk-42dcef04`), mitigation §13 (`domchk-8a20810b`), summary + alert closure §14–15 (`domchk-b3966b66`, closed parent alert `bf-5wxej`) |
| 2026-09-07 | Evidence bundle collected (`domchk-8e0fc94d`): [`docs/crash-artifacts-bf-4k2ws/`](../../crash-artifacts-bf-4k2ws/README.md) — primary-log extract, alert↔death mapping, unrecoverable-era inventory |
| 2026-09-08 | Erratum landed (commit `da72cef`, `domchk-474e649d`): bf-1s6c3's mechanism wording corrected to chain-inferred (kernel records for Aug-12/13 do not survive) |
| 2026-09-09 | §17 append (`domchk-29f7f613`): full §2 census re-derived **byte-exact, zero deltas**; dispatch-scope cap read live from a running scope (`memory.max = 12884901888` = 12 GiB); this README written (`domchk-aea1baca`) |

---

## 3. Root cause analysis

### 3.1 Primary cause — the repository-bloat-era kill regime

**INFRASTRUCTURE.** The crash-night repository was the bf-1s6c3/bf-4yjq bloat-era
store (~18 GB, ~17 GB loose objects — 17+ identical 237 MB `.beads/*.jsonl`
snapshots committed). `bf-4k2ws`'s task is git-remote-heavy (fetch / ls-remote /
rev-list against Forgejo and GitHub) — exactly the operation class that era turned
into deterministic kills. Each dispatch ran inside a needle systemd scope capped at
**12 GiB** (`memory.max = 12884901888`, re-read live from a running scope on
2026-09-09); when an attempt's working set exhausted that budget, the kernel's
memory-cgroup OOM killer SIGKILLed it — uncatchable, recorded by needle only as
the sentinel `exit_code = −1`.

**Mechanism confidence is chain-inferred for this bead, and the record says so.**
The Aug-14 16:39 reboot destroyed the kernel records for Aug-12/13, so no
`CONSTRAINT_MEMCG` line exists for `bf-4k2ws` itself. The same mechanism is
**kernel-proven** for the later gc/push siblings `bf-4x12ec` and `bf-198ne`, on
the same repository and scope — hence MEDIUM-HIGH confidence, not certainty.

**The kill-duration distribution is the scope-budget signature.** Kills at
123.6–528.9 s (median 252.9 s), spread across buckets [0, 15, 22, 15, 2, 1, 0]
with **zero** within 70 s of the 600 s cap — inconsistent with any timeout or
fixed boundary, exactly what an allocation failure at an unpredictable point in a
memory-hungry operation produces. The 5 × exit 124 attempts, by contrast, sit at
600,018–600,041 ms: the dispatch cap, a separate (workflow-class) event.

### 3.2 Why 55 kills — the two amplifiers

1. **Unbounded retry.** Each kill released the bead back to the ready frontier and
   the dispatch layer re-claimed it — 55 times, ~4.5 min median cycle, with no
   storm breaker or backoff in that era. Nothing bounded the loop.
2. **Verify-then-close debt.** The task *succeeded* at 04:48 and again at 07:17 —
   each success `verification.passed`, each then `bead.orphaned` ~6 s later
   instead of closing. The terminal condition was unreachable from the success
   side, so **25 of the 55 kills (45 %) were post-completion**: killed re-doing
   work that was already done and verified.

### 3.3 The separate alert-layer defect (where the "false positive" lives)

Pre-0.4.2 needle's `handle_crash` path ran, per kill: `outcome.classified` (crash)
→ `bead.released` → `outcome.handled {"action":"alerted"}` → **one new alert
bead**. Verified on both sides of the ledger: 55 `action=alerted` events in the
worker log, and exactly **55** beads titled `ALERT: Agent crash on bead bf-4k2ws`
in the store (created 02:03:40.617Z → 07:04:00.881Z; 21 closed / 32 open /
2 in_progress at the 2026-09-09 recount). No fingerprint, no dedup, no cooldown —
one root cause, 55 beads. Every alert *after* the 2026-08-16 closure additionally
fired against an already-closed bead.

So: **the kills were real; the false positives were in the alert layer** — of two
kinds: multiplication (55 alerts for one cause) and stale targets (post-success
and post-closure alerts describing work already finished).

### 3.4 Co-factor — host CPU saturation

59 `fleet.cpu_saturated` samples inside the loop window: load 7.63–18.51 on 9
cores against a saturation threshold of load > 7.2. The box was saturated
throughout, including across the mapped kill. Present, but not independently
sufficient: the era's kill mechanism is cgroup-scoped, and no host-OOM kill exists
in the surviving kernel record. Per-kill causal attribution between the two is
impossible without the lost kernel records — the determination does not claim it.

### 3.5 Ruled out

| Alternate | Why excluded |
|---|---|
| SIGHUP cascade (the 2026-09-02 claim) | **Zero exit-129s anywhere in the loop**; −1 is needle's unrecorded-signal sentinel, never a signal number; the claimed Aug-16 window was also the wrong day |
| Domain-check code defect | No code defect found in any investigation of this workspace; the task was read-only git analysis; zero `max_turn` mentions in the day's 395 completions |
| Max-turns / workflow exhaustion | Same zero-`max_turns` evidence; the 5 × 124 are the dispatch cap, not turn budgets |
| Service failure (gateway 503/502) | No service-class signature in the loop; the night's failure mode is uniform exit −1 |
| Host memory/disk exhaustion | Host was never out of memory — the constraint was the cgroup; the "52 GB free / clean repo" readings in the 2026-09-02 report were captured 2026-09-02, three weeks post-repair, describing the wrong night |
| Kernel-unrecorded ≠ kernel-absent caution | Correctly handled the other way: the *absence* of Aug-13 kernel lines is explained by the reboot (single boot begins 2026-08-15 19:56 EDT), not by absence of kills |

### 3.6 Reading the record — exit codes and timestamps

Exit codes are **classes, not signals**:

| Exit | Count here | Meaning |
|---|---|---|
| −1 | 55 | Needle's **unrecorded-signal sentinel** (signal death, code unrecorded). Never name a signal from it |
| 124 | 5 | The **600 s dispatch cap** (attempts 16, 17, 58, 59, 61) — *not* max-turns |
| 0 | 2 | Success (both `verification.passed`, both then orphaned) |

Five distinct timestamp layers, of which only one names a death:

| Layer | Stamps | bf-4k2ws values |
|---|---|---|
| Bead creation | when the bead row was written | 01:57:53.592Z — quoted as "crash time" is the corpus's oldest error |
| Death (`agent.completed`) | the actual kill | 55 instants, 02:03:33.620Z → 07:03:53.920Z — **the only correct "crash timestamps"** |
| Alert/heartbeat | `HANDLING_RELEASE_DONE`, then the bead write | death + 5.1–9.8 s (bead write ≤ 6 ms after heartbeat); an alert instant never names a distinct crash |
| Timezone | needle/JSONL logs are UTC (`Z`); journald is local EDT (−4 h) | storm = 21:57 EDT Aug-12 → 03:17 EDT Aug-13; no journald record can exist (boot starts Aug-15) |
| Closure | `bead close` | 2026-08-16 15:35:42Z — three days *after* the storm |

---

## 4. Impact

**Work: none lost.** All 8 acceptance criteria met; four deliverable docs on
`origin/main` (`docs/divergence-analysis-bf-4k2ws-2026-08-13-pre-merge.md`,
`docs/branch-divergence-bf-4k2ws-2026-08-13.md`,
`docs/branch-divergence-analysis-bf-4k2ws-current.md`,
`docs/branch-divergence-analysis.md` — presence re-verified via `git cat-file -e`
on 2026-09-09). The analysis's substantive finding — branch divergence **0/0**
— was re-verified through this chain and still holds.

**Cost of the response, not the crash:** 55 alert beads; a downstream pool of
~189 beads naming `bf-4k2ws` (§17.3; 124 carry it in the title — this session's
title-only recount), of which 48 remained open at the last §17.3 re-count; ~20
superseded-era reports written before the primary log was read. This is the
incident's real lesson in economics: the crash cost one night of lost cycles; the
uncoordinated response cost weeks.

**Repository:** the ≈18 GB bloat was the whole era's damage, not this bead's. It
is repaired and holding — 106 MB `.git`, 181 loose objects, one 100.49 MiB pack,
garbage 0, `git fsck --full` clean, 0 tracked `.beads/` files (re-measured
2026-09-09 for this report).

---

## 5. Fixes applied

### 5.1 Kill-mechanism layers (requirement → artifact, determination §10.1)

| # | Requirement | Committed artifact | Verification |
|---|---|---|---|
| R1 | Bound pack memory inside the 12 GiB scope — for gc **and** push-side `pack-objects` | git config `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` at **global and local** scope; installer/verifier `scripts/setup-git-gc-config.sh` | `--verify` exit 0: worst case ≈3072 MiB within the scope ceiling (window is per-thread, hence `threads=1`) |
| R2 | Object store must not re-bloat | `.gitignore` covers `.beads/`, `*.db`, `*.jsonl` repo-wide; `scripts/setup-git-hooks.sh` → 10 MB pre-commit gate | `git ls-files .beads` → 0; hook byte-identical to tracked source; pack consolidated, garbage 0 |
| R3 | Remediation bounded and unconditional | `scripts/safe-git-gc.sh` (staged, checkpoint/resume, `--check-only`); daily 03:00 incremental + weekly Sun 04:00 full gc timers, both `MemoryMax=4G` | `--check-only` passes preflight ("GC not needed" is the healthy verdict); all `domain-check-*` timers future-scheduled |
| R4 | Heavy work gated on environment | `scripts/preflight-health-check.sh`; `scripts/resource-monitor.sh` (pressure warn 70 / critical 80); `scripts/system-event-mode.sh` surge gate | preflight 4/4 exit 0; surge gate "clear" |
| R5 | **Not** the fix | Signal handling in `internal/server` (hardens the wrong process — the dying workers are needle's); longer dispatch cap (a NEEDLE knob, G-11/G-12); alert-dedup as *deployed* prevention (see §5.3 caveat) | — |

The two kill legs map to different halves of this table: the 55 mid-run kills are
answered by R1 + R2 (bound the allocation, keep the store small); the 5 × 124 cap
class is answered repo-side by the R4 gates, not by a longer cap.

### 5.2 Alert-layer fixes (the multiplication defect)

`scripts/crash-alert-manager.sh` + `scripts/crash-classifier.sh` +
`scripts/alert-deduplication.sh`: closed-bead (and target-closure) filtering,
duplicate detection + processed-alerts ledger, completion awareness, exit-code
validation, crash classification, 300 s cooldown. Suites:
`scripts/test-crash-alert-fixes.sh` (13/13 at last re-run), `test-closed-bead-filter.sh`
(7/7), plus a 10-assertion cascade replay proving the cooldown is a window, not a
wall. `bf-4k2ws` remains the motivating case for all of them.

### 5.3 Effectiveness caveat (do not over-claim)

The alert-manager fixes are **knobs, not yet a pipeline**: no systemd user unit
invokes `crash-alert-manager.sh`; the only crash-detection timer runs the
report-only `crash-pattern-detection.sh`. The dedup layer has never once fired in
production. The open gap list D-1..D-10 is owned by bead `domchk-b5448b6a` (with
the repo-boundary caveat that needle owns bead creation); the G-3 *adoption* half
— actually calling the surge gate from the manager/preflight — is owned by
`domchk-6951fe0c`. Audit prevention by **invocation**, not existence:
`grep -l <script> ~/.config/systemd/user/*.service`.

### 5.4 Resolution status — re-verified live 2026-09-09 for this report

All five read-only checks green at write time: `check-repo-health.sh` exit 0
(106 MB, 0 unpushed), `setup-git-gc-config.sh --verify` exit 0 (≈3072 MiB worst
case within the 12 GiB ceiling), `preflight-health-check.sh` exit 0 (4/4),
`system-event-mode.sh check` → clear, `crash-circuit-breaker.sh status` → no open
entries. The kill class has not recurred in live work (determination §16.3: the
window's kernel memcg kills are all synthetic test/gc scopes, zero from live
dispatch scopes).

---

## 6. Lessons learned

Consolidated from determination §12 (lessons leg) and §14.6; each was re-derived
from this incident's record, not imported.

**Investigation patterns**

1. **Multiplication signature.** 55 alerts naming one target and one root cause
   means the alert layer multiplied — group alerts by target bead and time window
   *before* investigating any single one. Per-alert investigation is what produced
   ~20 reports for this one incident.
2. **Stale-target signature.** Alerts raised after the target's first *verified
   success* describe work already done: 45 % of this storm's kills were
   post-completion. Read the bead's state and work-completion record before any
   trace.
3. **The closure-instant heuristic is unsound — corrected.** "Alert predates bead
   completion ⇒ false positive" compares against the *closure* instant; a
   legitimately retried bead closes days later, so every genuine alert in this
   storm "predates completion" by that measure. The correct comparison instant is
   the **first verified success**.
4. **Titles are not identity.** Near-identical artifact titles across beads are
   the main "already done" false match — check the report's Related Bead field.
5. **Exit codes are classes, not signals.** −1 = unrecorded-signal sentinel;
   124 = dispatch cap (not max-turns); 0 = success; 1 = check for synchronization
   before treating as per-bead. A single zero-`max_turns` sweep across the day's
   log killed an entire hypothesis family here.
6. **A classification is only as good as its derivation.** The superseded corpus
   classified this incident from derived reports; the current classification
   exists only because successive beads re-derived the census from the untouched
   primary log. Rule: **any figure you did not re-derive is a hypothesis**, and a
   classification built on hypotheses inherits their premise errors.
7. **Classify the cause once; close the swarm as instances.** One INFRASTRUCTURE
   determination governs 55 kills and their ~189-bead pool.

**Prevention patterns**

8. **Audit prevention by invocation, not existence.** A countermeasure no timer,
   hook, or caller invokes is documentation, not prevention (§5.3).
9. **Documentation hygiene is prevention.** Twelve superseded files in this very
   directory mis-taught the next investigator until each carried a dated banner
   naming its own stale premises. Banners + an append-to-canonical protocol are
   part of the prevention stack — and the cheapest layer to skip.
10. **A dispatch template is a stale-premise vector.** This incident's own
    dispatch template carried three superseded premises (exit −1 "(SIGKILL)", a
    heartbeat as "crash timestamp", the superseded directory as the findings
    location). Verify template fields against the current determination before
    rendering a summary from them.
11. **Parallel appends to one canonical document collide silently.** One append in
    this chain was lost to a same-file collision; re-read the file immediately
    before appending and reconcile section numbers after.
12. **The failure path must not act stronger than the success path.** Here, every
    failure produced an alert within milliseconds while both verified successes
    got `action: none` and were orphaned. When the success terminal is weaker
    than the failure terminal, retries manufacture work and alerts both.

---

## 7. Recommendations for future crash investigations

1. **Start at the dedup/target-resolution gate, not the investigation.** Read the
   target bead's state, its deliverable, and `git log --all --grep <bead-id>`
   first — most alerts point at work another worker already finished
   (`docs/crash-response-guide.md` → "Operational Runbooks by Alert Type").
2. **Go to the primary log before any derived report.** Derived reports here were
   wrong at premise level for five weeks. Worker logs live in
   `~/.needle/logs/`; bracket deaths between `agent.completed` records and JSON-
   parse (`outcome.classified` / `outcome.handled` live on separate events —
   grep misses them).
3. **Name the timestamp layer for every figure you quote.** If a number traces to
   bead creation, a heartbeat, a closure, or the wrong timezone, it is not a
   crash instant (§3.6 table).
4. **Respect what the record cannot say.** No Aug-13 kernel record survives;
   the mechanism is chain-inferred via kernel-proven siblings. Say so — an
   honest MEDIUM-HIGH beats a confident wrong certainty, and the superseded
   corpus's certainty is what took five weeks to unwind.
5. **Preserve evidence you will need.** Traces are single-slot; health collection
   starts 2026-08-15; kernel records die with reboots. The surviving census was
   possible only because the worker log survived. `scripts/verify-work-completion.sh`
   markers (`.beads/state/work-completion/`) are the durable post-completion
   witness — run it before every close.
6. **Append to the canonical document; banner the stale ones.** Do not open a new
   report per question (that is how this directory reached twelve files). New
   findings append a section to
   `docs/investigations/bf-4k2ws-root-cause-determination-domchk-7f838f36-2026-09-07.md`;
   anything contradicted gets a dated SUPERSEDED banner naming what it claimed.
7. **Do not re-open owned gaps.** G-3 adoption = `domchk-6951fe0c`; alert-layer
   D-1..D-10 = `domchk-b5448b6a`; G-9..G-13 are NEEDLE-external
   (`docs/crash-prevention-requirements.md`). Re-opening them from a
   lessons-learned pass is the duplicate-generation pattern this incident already
   paid for.
8. **Before validating any prevention claim, re-run its layer** —
   `docs/crash-prevention-validation.md` per-layer procedures, or the five-check
   battery in §5.4. A claim is only as current as its last validation.

---

## 8. Corpus map and supersession record

### 8.1 This directory (all pre-reclassification files carry dated banners)

| File | Era claim | Status |
|---|---|---|
| `comprehensive-crash-report-bf-4k2ws.md` (1016 lines) | most comprehensive 2026-08/09 corpus file | SUPERSEDED premise; bannered |
| `final-investigation-report-2026-09-02.md` | "final" under the no-crash premise | SUPERSEDED premise; bannered |
| `root-cause-analysis-final-bf-4k2ws.md` | SIGHUP-cascade root cause | SUPERSEDED; bannered |
| `root-cause-analysis-signal-minus1.md` | "signal −1" as a signal | SUPERSEDED; bannered (§11.3 delta closed) |
| `fixes.md` | six alert fixes under the old premise | Fixes **stand**; causal premise superseded; bannered |
| `crash-evidence-summary-2026-09-02.md`, `crash-evidence-summary-bf-4k2ws.md`, `crash-investigation-report.md`, `crash-summary-2026-08-26.md`, `crash-diagnostics-summary-domchk-af961320.md`, `investigation-summary-domchk-9377ad1d.md`, `needle-workspace-log-analysis-bf-4k2ws.md` | era evidence/diagnostic summaries | SUPERSEDED premise; bannered |

### 8.2 Related documents outside this directory

- **Canonical determination:** `docs/investigations/bf-4k2ws-root-cause-determination-domchk-7f838f36-2026-09-07.md` (§§1–17)
- **Crash report + correction:** `docs/crashes/bf-4k2ws-crash-report.md` (2026-09-02 body; 2026-09-07 correction section appended; 2026-09-08 erratum `da72cef`)
- **Evidence bundle:** `docs/crash-artifacts-bf-4k2ws/README.md` (primary-log extract, alert↔death mapping)
- **The era's mechanism, kernel-proven:** `docs/crash-analysis-bf-1s6c3-2026-09-06.md` (predecessor storm), `docs/crash-reports/bf-4x12ec-git-gc-crash.md` and `docs/crashes/bf-198ne-crash-report.md` (gc/push siblings)
- **Operating canon:** `docs/crash-response-guide.md`, `docs/crash-prevention-requirements.md`, `docs/crash-prevention-validation.md`, `docs/maintenance/repository-maintenance-guide.md`

### 8.3 What the correction changed (supersession table)

| Superseded claim | Current determination |
|---|---|
| "FALSE POSITIVE — no crash occurred; Total Crash Events: 0" | 55 real kills; FP confined to the alert layer |
| "exit −1 was SIGHUP (signal 1)" | Unrecorded-signal sentinel; zero exit-129s |
| SIGHUP window "2026-08-16 12:00–17:00" | Wrong day — the loop ran 2026-08-13 02:01–07:17Z |
| "NOT Repository Bloat — clean repo (<500 MB), 52 GB free" | Readings captured three weeks post-repair; crash night was the ≈18 GB bloat-era repo |
| "crash alert predates completion ⇒ auto-FP" | Compare against first verified success, not closure |
| Task completed 8/8, closed 2026-08-16; alert-system fixes needed | **Stands** |

---

## 9. Provenance for this README

Written 2026-09-09 by `domchk-aea1baca` at HEAD `6f28468` = `origin/main`, zero
unpushed. **Re-verified first-hand this session:** the 55 ALERT-bead ledger and
its 21/32/2 status split (live `bead list --json` parse); presence of the four
deliverable docs, the determination, the crash report, and the bundle README on
`origin/main` (`git cat-file -e`); HEAD↔origin parity; the five-check prevention
battery (§5.4, all exit 0); repo metrics in §4 (`du`, `count-objects -vH`,
`fsck --full`, `ls-files .beads`). **Quoted with provenance, not re-derived:**
every census figure, chronology instant, and confidence level from the canonical
determination (§§1–17, most recently re-derived byte-exact by its §17 on
2026-09-09) and the evidence-bundle findings.
