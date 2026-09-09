# bf-4k2ws — Investigation Summary and Verification Report

**Dispatched leg:** document / synthesis + verification (`domchk-e396d000`, template-chain "document findings" step)
**Target bead:** `bf-4k2ws` — "Analyze divergent Forgejo and GitHub branch states" —
**Closed** (rev 2, 2026-08-16T15:35:42Z, 8/8 acceptance criteria met)
**Source alert bead:** `bf-4xlwo` (Open, rev 20, re-read live this session) ↔
**attempt 57 of 62**, death 2026-08-13T06:34:29.204954348Z
**Classification:** **INFRASTRUCTURE** — repository-bloat-era kill regime
(memcg-OOM SIGKILL inside the 12 GiB dispatch scope), with a separate — and
separately fixed — false-positive defect in the alert layer
**Verdict:** ✅ **RESOLVED** — target work complete and shipped; kill mechanism
remediated and re-verified live this session (§5.2); **no retry is needed, and a
retry would be actively wrong** (§5.1)
**Determination date:** 2026-09-09

> **What this document is.** The document-leg deliverable, filed at the
> dispatch-named path. It synthesizes the three prior template-chain legs —
> `domchk-6ba3b2af` (crash-context gather), `domchk-541f1089` (root-cause
> analyze), `domchk-5a0a456a` (severity determination) — plus the canon final
> report `domchk-aea1baca` (`docs/crash-investigations/bf-4k2ws/README.md`),
> and adds the one thing no prior leg carried as a standalone section: the
> **verification report** (retry safety, resolution status, prevention
> inventory) re-run live at write time (§5).
>
> **Subordination.** This document is **subordinate to the canonical record**:
> `docs/investigations/bf-4k2ws-root-cause-determination-domchk-7f838f36-2026-09-07.md`
> (§§1–17) and `docs/crash-investigations/bf-4k2ws/README.md`. It re-derives no
> figure those records already derived; census numbers are quoted with
> provenance, and the live checks in §5 were executed by this bead at write
> time. It is a leg-level synthesis, **not** a competing final report — the
> canon README is the final report.

---

## 0. Template-premise corrections (read first)

The dispatch template that commissioned this leg carried four stale or
under-determined premises — the pattern the canon's lesson 10 names ("a
dispatch template is a stale-premise vector"). All four are corrected before
use; the first is the substance of this leg's verification report.

| # | Template premise | As dispatched | Verified fact |
|---|---|---|---|
| 1 | "Whether bead bf-4k2ws **can be retried**" | Treats retry-ability as open | **The bead is Closed** (rev 2, 2026-08-16, 8/8 criteria, four deliverables on `origin/main` — all re-verified live this session). Retry is neither needed nor permitted: needle does not re-dispatch a closed bead, and re-opening a satisfied bead re-runs verified work — the exact post-completion re-work class that consumed 25 of the night's 55 kills (45 %) and inflated this incident into a ~189-bead pool. The answerable residue — *would* a retry of this task class be *safe* today, if it were ever genuinely needed — is **yes** (§5.1) |
| 2 | Source alert "Timestamp: 2026-08-13T06:34:35.805234687Z" | Quoted as a crash instant | **A heartbeat stamp, never a death** (carried from the gather leg, corrected before it reached this leg's template): it matches worker-log `heartbeat.emitted` seq 8376 (`HANDLING_RELEASE_DONE`, 06:34:35.805222217Z) to 12.47 µs; the kill is `agent.completed` at 06:34:29.204954348Z, **6.600 s earlier**. `bf-4xlwo` was written 5.9 ms after that heartbeat |
| 3 | "Exit code −1 (signal −1)" | Names a signal | **−1 is needle's unrecorded-signal sentinel — no such signal exists.** The mechanism of the signal death is memcg-OOM SIGKILL: chain-inferred for this bead (MEDIUM-HIGH; no Aug-13 kernel record survives the Aug-14 reboot), kernel-proven for the same-repo, same-scope siblings `bf-4x12ec` / `bf-198ne` |
| 4 | Deliverable path `docs/crash-investigation/` (singular) as the findings home | Implies the corpus home | The corpus home is `docs/crash-investigations/` (plural), which already holds the canon final report. This file is created at the named path anyway so the acceptance item resolves there — the same stance the three prior legs took; it is subordinate to the canon, not a second final report |

---

## 1. Executive summary

On 2026-08-13, `bf-4k2ws` — a **read-only** Forgejo-vs-GitHub branch-divergence
analysis — ran a **62-attempt dispatch loop on one worker session**
(`8446529e`, agent `claude-code-glm-4.7`, 02:01:29.710Z → 07:17:41Z):
**55 attempts killed mid-run (exit −1), 5 hit the 600 s dispatch cap (exit 124),
2 completed successfully (exit 0)**. The task itself **completed**: both
successes passed verification, and the bead closed 2026-08-16 with all 8
acceptance criteria met and four deliverable documents on `origin/main`.

| Question | Answer |
|---|---|
| Did a crash occur? | **Yes** — 55 genuine mid-run kills. The 2026-09-02 corpus's "no crash occurred / SIGHUP cascade" premise is superseded (reclassified 2026-09-07, commit `ef39024`) |
| Root cause | Dispatches running git-remote-heavy work against the then-**≈18 GB** object store inside needle's **12 GiB dispatch scope** (`memory.max = 12884901888`) were killed by the kernel's memory-cgroup OOM killer when an attempt's working set exhausted the scope budget; the retry layer then re-claimed the bead **55 times with nothing bounding the loop** |
| Mechanism proof | Chain-inferred for this bead (MEDIUM-HIGH) via kernel-proven siblings `bf-4x12ec` (bare `git gc --aggressive`) and `bf-198ne` (unbounded `git push` pack-objects); no Aug-13 kernel record survives the Aug-14 reboot |
| Why 55 kills | Two amplifiers: **unbounded retry** (no breaker/backoff in that era) and **verify-then-close debt** (both verified successes were `bead.orphaned` ~6 s after passing, so 25 of 55 kills were post-completion re-work) |
| Code defect | **None** — the task was read-only git analysis; no stderr/panic/stack exists; zero `max_turn` mentions in the day's 395 completions; the workspace's 157+ investigation record holds |
| Is the issue resolved? | **Yes for the kill mechanism** (repo repaired to 105 MB, pack memory bounded repo-wide and globally, safe-gc stack + timers live — battery re-run green by this bead, §5.2); **mitigated-with-open-items for the alert layer** (fixes committed with suites, but knobs not yet a wired pipeline — §5.3) |
| Can bf-4k2ws be retried? | **No — and none is owed.** Closed 8/8 with deliverables shipped and the divergence answer (0/0) still holding. Retrying would re-run verified work. A retry today *would* be mechanically safe if one were ever genuinely needed (§5.1) |
| Work lost | **Zero.** All 8 criteria met; four deliverable docs present on `origin/main` (`git cat-file -e` re-verified by this bead this session) |

**The incident's economic lesson** (canon §1, §4): the crash cost one night of
lost cycles; the uncoordinated response — 55 per-kill alert beads, a downstream
pool of ~189 beads naming `bf-4k2ws`, ~20 superseded-era reports written before
anyone read the primary log — cost weeks.

---

## 2. Investigation timeline

### 2.1 Incident and resolution (2026-08-13 → 2026-08-16; canon §2)

| Instant (UTC) | Event |
|---|---|
| 08-13 01:57:53.592Z | `bf-4k2ws` created — a bead-existence instant, **not** a crash instant (the superseded corpus's oldest timestamp error) |
| 08-13 02:01:29.710Z | First claim (7.15 s after predecessor `bf-1s6c3` completed exit 0 on the same session — itself orphaned) |
| 08-13 02:03:33.620Z | Attempt 1 killed — storm begins; first alert bead 6 ms after the first release heartbeat |
| 08-13 04:48:09.546Z | **First genuine success** (attempt 33, exit 0) — `verification.passed`, then `bead.orphaned` 5.8 s later; the loop continues |
| 08-13 06:34:29.204954348Z | **Attempt 57 killed** — the death this investigation chain's source alert (`bf-4xlwo`) represents; mid-distribution for the kill class (319.8 s vs median 252.9 s); its successor, attempt 58, hit the 600 s cap (exit 124, 600,020 ms) |
| 08-13 07:03:53.920Z | Last kill (attempt 60, 528.9 s — the longest) |
| 08-13 07:17:41.039Z | **Second genuine success** (attempt 62); verified, then orphaned again |
| 08-16 15:35:42Z | **`bf-4k2ws` closed** (rev 2, 8/8); deliverables entered git the same day in squash `c27899f` — the storm window itself holds zero commits on `main`/`origin/main` |

### 2.2 Investigation chronology

| Date | Event |
|---|---|
| 2026-09-02 | First comprehensive report (`docs/crashes/bf-4k2ws-crash-report.md`, `domchk-dd05bc9c`): classified FALSE_POSITIVE / "no crash occurred" / SIGHUP cascade — **premise later superseded**; its alert-fix recommendations stood |
| 2026-09-07 | **Reclassification** (`ef39024`, `domchk-3fca6de4`): 62-attempt census recounted from the untouched primary log → INFRASTRUCTURE, false-positive confined to the alert layer |
| 2026-09-07 | Canonical determination authored (`domchk-7f838f36`) and extended append-only by the split chain: chronology §8, NEEDLE deficiencies §9, fix spec §10, verification §11, lessons §12, mitigation §13, summary + parent-alert closure §14–15 |
| 2026-09-07 | Evidence bundle collected (`domchk-8e0fc94d`, `docs/crash-artifacts-bf-4k2ws/`): primary-log extract, alert↔death mapping, unrecoverable-era inventory |
| 2026-09-08 | Erratum `da72cef` (`domchk-474e649d`): predecessor `bf-1s6c3`'s mechanism wording corrected to chain-inferred |
| 2026-09-09 | §17 append (`domchk-29f7f613`): full §2 census re-derived **byte-exact, zero deltas**; 12 GiB dispatch cap read live from a running scope; canon final report written (`domchk-aea1baca`) |
| 2026-09-09 | Template-chain legs: gather `domchk-6ba3b2af` (→ `bf-4k2ws-crash-context.md`, commit `c794337`), analyze `domchk-541f1089` (→ `bf-4k2ws-root-cause.md`, commit `975cc31`), severity `domchk-5a0a456a` (→ `bf-4k2ws-severity.md`, commit `9677636`) |
| 2026-09-09 | **This leg** (`domchk-e396d000`): synthesis + verification report at the dispatch-named path |

---

## 3. Findings by phase

### 3.1 Phase 1 — crash-context gather (`domchk-6ba3b2af` → `bf-4k2ws-crash-context.md`)

- **Alert ↔ death mapping (this leg's new fact beyond the bundle):**
  `bf-4xlwo` ↔ **attempt 57**, the 57th claim of the loop. Full chronology
  re-derived first-hand from the untouched primary log
  (`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl`,
  3.1 MB, mtime 2026-08-13 19:59): claim 06:29:08.909Z → kill
  06:34:29.204954348Z (319,831 ms) → `outcome.classified` crash → four
  heartbeats → alert bead written 06:34:35.811Z (+5.9 ms) → release →
  `outcome.handled {"action":"alerted"}` → attempt 58 claimed 2.2 s later.
- **Timestamp premise corrected:** the template's "crash timestamp" is the
  heartbeat (death + 6.600 s), matching the log record to 12.47 µs.
- **Evidence inventory:** the primary worker log, its committed extract, the
  evidence bundle, and the alert-bead bodies survive; dispatch traces
  (single-slot), Aug-13 kernel/journald records (boot boundary — single boot
  begins 2026-08-15 19:56:33 EDT), coredumps (zero 2026-08-13 entries), and
  memory/disk samples (collection starts 2026-08-15) do not — recorded so
  nobody re-searches.

### 3.2 Phase 2 — root-cause analyze (`domchk-541f1089` → `bf-4k2ws-root-cause.md`)

- **Determination: INFRASTRUCTURE — resource limit.** Git-remote-heavy
  dispatches against the then-≈18 GB store inside the 12 GiB dispatch scope →
  memcg-OOM SIGKILL (uncatchable → sentinel −1 → `outcome.classified: crash`).
  Not an agent bug, not a domain-check code defect.
- **Exit codes are classes, not signals:** −1 × 55 (unrecorded-signal
  sentinel), 124 × 5 (the 600 s dispatch cap — attempts 16, 17, 58, 59, 61 —
  *not* max-turns), 0 × 2 (verified successes). SIGTERM ruled out (would have
  produced a recorded status), SIGHUP ruled out (zero exit-129s anywhere — the
  superseded corpus's claim).
- **Kill-duration distribution is the scope-budget signature:** 123.6–528.9 s,
  median 252.9 s, zero kills within 70 s of the cap — an allocation failure at
  an unpredictable point, matching no timeout.
- **Co-factor:** host CPU saturation — 59 `fleet.cpu_saturated` samples
  in-window (load 7.63–18.51 on 9 cores, threshold > 7.2), including one 6.5 ms
  before attempt 57's dispatch (load 11.65, seq 8361). Measurable at dispatch
  time, consumed by nothing in that era.
- **Amplifiers:** unbounded retry (55 re-claims, ~4.5 min median cycle) and
  verify-then-close debt (both successes orphaned → 45 % of kills were
  post-completion, including the one this alert represents).
- **Ruled out:** SIGHUP cascade, agent/code defect, max-turns exhaustion,
  service failure, host OOM/disk exhaustion (host never out of memory — the
  constraint was the cgroup).

### 3.3 Phase 3 — severity determination (`domchk-5a0a456a` → `bf-4k2ws-severity.md`)

- **Three layers, three labels** (canon §8.3): the kills were **real**
  (55 genuine deaths); the underlying cause was a **real infrastructure
  defect** — already remediated and verified holding; the **alert** is a
  **STALE FALSE POSITIVE** in both documented senses — multiplication (1 of 55
  per-kill beads, pre-dedup needle < 0.4.2) and stale target (attempt 57
  re-ran work that had already passed verification at 04:48:09.546Z; the
  target closed 8/8 three days later; zero work lost).
- **No new remediation owed.** R1–R4 landed pre-dispatch and re-verified green
  that session; genuinely open items have named owners (§5.3 here).
- **Four-step responder order for this alert shape:** verify the target's
  actual state before investigating → read the alert timestamp as a heartbeat →
  treat −1 as a class, not a signal → never re-fire a template chain at a
  closed target.

### 3.4 Phase 4 — canon final report (`domchk-aea1baca` → `docs/crash-investigations/bf-4k2ws/README.md`)

Consolidates everything above with the corpus router (twelve bannered
superseded files in that directory), the impact statement (no work lost; the
response cost more than the crash), the R1–R5 fix table with its "knobs, not
yet a pipeline" caveat, 12 lessons, and 8 recommendations for future
investigations — including recommendation 8, the one this leg implements:
**before validating any prevention claim, re-run its layer** (§5.2 here).

---

## 4. How the investigation was conducted (steps taken)

Method, so the report's confidence levels are auditable:

1. **Primary log before derived reports.** Every census figure traces to the
   untouched worker log, not to the ~20 derived reports written before it was
   read. The §17 append re-derived the full §2 census byte-exact (zero deltas)
   on 2026-09-09; the gather/analyze legs re-derived the attempt-57 window
   first-hand the same day.
2. **Live verification over citation.** Each leg re-read what it cited:
   `bead show` for target/alert status, alert bodies from the checkpoint
   snapshots, `journalctl --list-boots`, `coredumpctl list`, the dispatch-scope
   `memory.max` from an in-flight cgroup, and `git cat-file -e` for every
   deliverable claimed present.
3. **Classification from the record, not the trace.** Needle's automated
   classifier saw only `outcome.classified: crash`; the INFRASTRUCTURE
   determination comes from the worker-log census plus the kernel-proven
   siblings — mechanism confidence stated honestly as **MEDIUM-HIGH
   (chain-inferred)** for this bead because the Aug-13 kernel records cannot
   exist.
4. **Classify once, close the swarm as instances** (canon lesson 7). This
   chain's re-fired legs verify and disposition; none re-investigates or
   re-opens a determination.
5. **Prevention claims re-validated at cite time** (canon recommendation 8,
   `docs/crash-prevention-validation.md`). The §5.2 battery below was executed
   by this bead at write time, not inherited from a prior leg's record.

---

## 5. Verification report

Re-verified live by this bead (`domchk-e396d000`) on 2026-09-09 at HEAD
`9677636` = `origin/main`, **0 unpushed commits**, disk 31 GB free.

### 5.1 Can bead bf-4k2ws be retried? — **NO, and no retry is owed**

| Check | Result this session |
|---|---|
| Target status | `bead show bf-4k2ws` → **Closed**, rev 2, updated 2026-08-16T15:35:42.024Z ✓ |
| Acceptance criteria | 8/8 met (canon §4; the bead's own criteria list) ✓ |
| Deliverables shipped | All four present on `origin/main` via `git cat-file -e`: `docs/divergence-analysis-bf-4k2ws-2026-08-13-pre-merge.md`, `docs/branch-divergence-bf-4k2ws-2026-08-13.md`, `docs/branch-divergence-analysis-bf-4k2ws-current.md`, `docs/branch-divergence-analysis.md` ✓ |
| Substantive finding | Branch divergence **0/0**, re-verified through the chain and still holding (canon §4, §8.3) ✓ |

**Verdict: retry is unnecessary, and re-running the bead would be wrong.** The
work is complete, verified, and shipped; the "released for retry" line in the
`bf-4xlwo` body is the era's crash-handler boilerplate, three days stale.
Re-opening a satisfied bead is precisely the **post-completion re-work** class
that burned 25 of the storm's 55 kills and multiplied this incident into a
~189-bead pool — the loop's failure mode, not a remedy for it.

**The answerable residue — *would* a retry be *safe*, if one were ever
genuinely needed: yes.** The kill mechanism is remediated (§5.2), and even
under the unremediated regime the identical task + agent + session + template
succeeded twice on the crash night itself — a retry today has no remaining
failure mode attributable to that night.

**Adjacent disposition owed (not this bead's work):** source alert `bf-4xlwo`
remains **Open** (rev 20, re-read live) and unowned — stale by the standard
disposition (target closed, work shipped, canon §7.1). It owes a **disposition
close**, not an investigation and not a retry.

### 5.2 Is the crash issue resolved or mitigated? — **kill mechanism: RESOLVED; alert layer: mitigated with owned open items**

Battery re-run by this bead at write time (per `docs/crash-prevention-validation.md`):

| # | Check (layer) | Exit | Observed this session |
|---|---|---|---|
| 1 | `./scripts/check-repo-health.sh` (R2/R3) | 0 | `.git` **105 MB**; 212 loose objects; **1 pack**, 100.49 MiB; garbage **0**; no unmanaged aggressive gc; unpushed backlog **0** (informational large-binary listing from old `dist/` history — the check still passes) |
| 2 | `./scripts/setup-git-gc-config.sh --verify` (R1) | 0 | Effective bound system → global → local, worst case **≈3072 MiB** per pack run — within the verifier's 6,442,450,944-byte ceiling for the 12 GiB dispatch scope |
| 3 | `./scripts/preflight-health-check.sh` (R4) | 0 | **5/5** checks passed (system event, gateway, repo health 0.103 GB, cgroup headroom, breaker) — the suite has grown from the 4/4 recorded by the severity leg earlier the same day; count drift = suite growth, not regression |
| 4 | `./scripts/system-event-mode.sh check` (R4) | 0 | `clear` — abnormal 300s=0 / 1h=0, crashes 300s=0, PSI some avg60=0.67 % |
| 5 | `./scripts/crash-circuit-breaker.sh status` (R4) | 0 | No open breaker entries |

**Resolution logic.** The root cause was the ≈18 GB store; it is repaired
(105 MB, fully packed, `git fsck --full` clean per the health check, **0
tracked `.beads/` files**) and cannot re-bloat through `.beads/` (gitignored)
or oversized commits (10 MB pre-commit gate). Pack memory is bounded at
repo-local **and** global scope, so bare `git gc`/`git push` are bounded too —
the two operation classes that killed the era (gc-side `bf-4x12ec`, push-side
`bf-198ne`) and that this bead's regime fed on. The kill class has not recurred
in live work: the window's kernel memcg kills are all synthetic test/gc scopes,
zero from live dispatch scopes (determination §16.3).

**Honest caveat — do not over-claim** (canon §5.3): the **alert-layer** fixes
(closed-bead filtering, dedup + processed-alerts ledger, completion awareness,
exit-code validation, cooldown) are committed with suites green at recent
HEADs, but they are **knobs, not yet a wired pipeline** — no systemd unit
invokes `crash-alert-manager.sh`, and the dedup layer has never fired in
production. That gap is owned, not open-ended: §5.3.

### 5.3 Monitoring and prevention measures in place

**Kill-mechanism layers (R1–R4), all verified above:**

| Layer | Artifact |
|---|---|
| R1 — pack memory bounded (gc **and** push) | `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1`, repo-local **and** global; `scripts/setup-git-gc-config.sh` installs/verifies |
| R2 — store cannot re-bloat | `.gitignore` covers `.beads/`, `*.db`, `*.jsonl` repo-wide; `scripts/setup-git-hooks.sh` → 10 MB pre-commit gate (per-clone; `--check` to verify) |
| R3 — bounded, unconditional remediation | `scripts/safe-git-gc.sh` (staged, checkpoint/resume); systemd user timers — incremental gc daily 03:00, full gc Sun 04:00 (both `MemoryMax=4G`), repo health + auto-gc check daily 02:00 |
| R4 — heavy work gated on environment | `scripts/preflight-health-check.sh`, `scripts/resource-monitor.sh`, `scripts/system-event-mode.sh` surge gate, `scripts/crash-circuit-breaker.sh`; dispatch entry point `scripts/needle-with-limiter.sh` so a bead in storm backoff is deferred out of the ready frontier |
| Alerting/monitoring timers | Crash-pattern detection every 10 min; resource monitor every 5 min; service monitor every 2 min (plus the daily/weekly maintenance timers above) |
| Alert-layer fixes | `scripts/crash-alert-manager.sh` + `crash-classifier.sh` + `alert-deduplication.sh`; suites `test-crash-alert-fixes.sh` (13/13 at last re-run), `test-closed-bead-filter.sh` (7/7), 10-assertion cascade replay — with the §5.2 pipeline caveat |
| Validation protocol | `docs/crash-prevention-validation.md` — per-layer procedures and the rule that a prevention claim is only as current as its last validation |

**Open items — owned; not this chain's work** (canon §10.2; do not re-open):

| Item | Owner / vehicle |
|---|---|
| G-3 adoption half — actually call `system-event-mode.sh` from `crash-alert-manager.sh` / `preflight-health-check.sh` | bead `domchk-6951fe0c` |
| Alert-layer D-1..D-10 (dedup knobs exist; pipeline never fired in production) | bead `domchk-b5448b6a` (repo-boundary caveat: needle owns bead creation) |
| G-9 work-completion detection at the alert source, G-10 dispatch-scope sizing, G-11 retry/backoff, G-12 turn budgets, G-13 CPU throttling | NEEDLE repo — Phase 3 of `docs/crash-prevention-requirements.md` |

---

## 6. Conclusion and recommendations

**Conclusion.** The bf-4k2ws incident is closed in every sense that matters:
the 55 kills were real but are fully explained (INFRASTRUCTURE — memcg-OOM
SIGKILL of git-remote-heavy dispatches against the bloat-era ≈18 GB store
inside the 12 GiB dispatch scope, amplified by unbounded retry and
verify-then-close debt); the target bead's work completed and shipped with
nothing lost; the kill mechanism is remediated with all four layers verified
green by this bead at write time; the alert that seeded this chain is a stale
false positive confined to the (separately fixed, still-unwired) alert layer.
The investigation itself is **complete**: gather → analyze → severity →
canon final report → this synthesis, every figure either re-derived from the
primary log or quoted from the canon with provenance, and every prevention
claim re-validated live at cite time.

**Recommendations:**

1. **Do not retry `bf-4k2ws`.** It is Closed 8/8 with deliverables on
   `origin/main`; a retry re-runs verified work and manufactures exactly the
   duplicate-work pool this incident paid for. Treat any future dispatch
   targeting it as a stale-template re-fire and disposition, don't execute.
2. **Disposition the source alert, don't investigate it.** `bf-4xlwo` (and the
   other still-open ALERT beads against this target) owe the standard stale
   disposition close — target closed, work shipped (canon §7.1) — with no new
   investigation.
3. **Keep the responder order for this alert shape:** verify the target bead's
   actual state before investigating → read alert timestamps as heartbeats →
   treat exit −1 as a class, never a signal → never re-fire a template chain at
   a closed target (severity leg §3.3).
4. **Re-validate before citing.** Prevention statements in any future report
   should carry a same-session battery run (§5.2), per
   `docs/crash-prevention-validation.md`.
5. **Append, don't proliferate.** New findings about this incident append a
   section to the canonical determination; superseded documents get dated
   banners. This document exists only because its dispatch named a specific
   missing path — it is a synthesis of the canon, not a fourth report to route
   around.
6. **Leave the owned gaps to their owners** (§5.3) — G-3 adoption
   (`domchk-6951fe0c`), D-1..D-10 (`domchk-b5448b6a`), G-9..G-13 (NEEDLE
   upstream). Re-opening them from a lessons pass is the duplicate-generation
   pattern this incident already paid for.

---

## 7. Acceptance-criteria mapping

| Criterion (dispatch) | Answer | Where |
|---|---|---|
| Review findings from `domchk-6ba3b2af` (context) | Reviewed and carried — all three deliverables re-confirmed on `origin/main` this session; their figures quoted with provenance, not re-derived | §3.1, §2.2 |
| Review findings from `domchk-541f1089` (root cause) | Reviewed and carried — determination INFRASTRUCTURE resource limit; amplifiers, co-factor, ruled-out table | §3.2 |
| Review findings from `domchk-5a0a456a` (severity) | Reviewed and carried — REAL problem remediated-and-holding; alert = stale false positive; no new remediation owed | §3.3 |
| Comprehensive investigation summary | This document — executive summary, incident + investigation timelines, findings per phase, method | §§1–4 |
| Document all steps taken during the investigation | Leg-by-leg chronology plus the method's verification discipline | §2.2, §4 |
| Verification: can bf-4k2ws be retried? | **No — closed 8/8, deliverables shipped, retry would re-run verified work; a retry would nonetheless be mechanically safe today** | §5.1 |
| Verification: crash issue resolved or mitigated? | Kill mechanism **RESOLVED** (battery green this session); alert layer **mitigated** with owned open items and the knobs-not-pipeline caveat | §5.2 |
| Verification: monitoring/prevention measures | R1–R4 artifact table, timers, alert-layer suites, validation protocol, open items with owners | §5.3 |
| Save to `docs/crash-investigation/bf-4k2ws-investigation-report.md` | This file — created at the dispatch-named path, subordinate to the canon (§0, premise 4) | — |

---

## 8. Sources

**Re-verified first-hand by this bead, 2026-09-09 (write time):** `bead show
bf-4k2ws` (Closed rev 2); `bead show bf-4xlwo` (Open rev 20); `git cat-file -e
origin/main:<doc>` × 9 (four target deliverables, the determination, the canon
README, and the three prior legs — all present); HEAD `9677636` = `origin/main`,
0 unpushed; the five-check prevention battery of §5.2 (all exit 0); disk 31 GB
free.

**Quoted with provenance, not re-derived here:** every census figure,
chronology instant, and confidence level from the canonical determination
`docs/investigations/bf-4k2ws-root-cause-determination-domchk-7f838f36-2026-09-07.md`
(§§1–17; §17 re-derived the §2 census byte-exact on 2026-09-09) and the canon
final report `docs/crash-investigations/bf-4k2ws/README.md`; the attempt-57
window and alert↔death mapping from the gather leg
(`docs/crash-investigation/bf-4k2ws-crash-context.md`), which re-derived them
from the untouched primary log
`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` this
same day; kernel-proven sibling records
(`docs/crash-reports/bf-4x12ec-git-gc-crash.md`,
`docs/crashes/bf-198ne-crash-report.md`); evidence bundle
`docs/crash-artifacts-bf-4k2ws/README.md`.
