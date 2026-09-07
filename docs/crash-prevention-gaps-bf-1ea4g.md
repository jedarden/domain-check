# Crash Prevention & Detection Gap Analysis — bf-1ea4g

**Date:** 2026-09-07 · **Bead:** domchk-29af544b · **Target bead:** bf-1ea4g ("Document local main branch state", closed 2026-08-13T09:10:16Z — re-verified this session)
**Question:** for bf-1ea4g's 56-kill sequence, what did prevention and detection miss, what already covers it today, and what is still owed?
**Method:** the crash sequence was decomposed into seven causal stages (§2); each stage was mapped against the safeguards that existed on 2026-08-13, the safeguards that exist today (every "today" claim live-verified this session, §8), and the two canonical gap registers this workspace already keeps. No determination here is re-derived: the crash's mechanism is cited to the canonical record, and the alert-layer findings are cited to the dedup analysis.

**Dispatch premise corrected:** the task names `docs/crash-prevention-guide.md` as a review input. **That file does not exist.** The live document of that name is [`docs/comprehensive-crash-prevention-guide.md`](comprehensive-crash-prevention-guide.md); it was reviewed, along with `docs/crash-mitigation-strategies.md` (v2.3) as named. Stale dispatch paths are a known corpus failure mode (bf-4k2ws §14.6) — recorded here so the template can be fixed.

**Relationship to the two existing gap registers** — this document does not duplicate either:
- [`docs/crash-prevention-requirements.md`](crash-prevention-requirements.md) (**G-1..G-13**) — the workspace-wide requirements list, with the safeguard inventory live-verified 2026-09-06. This doc maps *one crash* onto it and adds what the crash uniquely exposes.
- [`docs/alert-deduplication-gap-analysis-2026-09-07.md`](alert-deduplication-gap-analysis-2026-09-07.md) (**D-1..D-10**) — the alert layer, verified to script-line granularity the same day. Its repair plan is not restated here; §6 evaluates it *as it bears on bf-1ea4g*.

---

## 1. What the crash was (cite, don't re-derive)

Canonical record: [`docs/crash-inventory-bf-1ea4g-summary.md`](crash-inventory-bf-1ea4g-summary.md) (§1–§5 are the current determination; its §6–§7 inventory the 138-document corpus and its contradictions).

Compressed: **57 dispatch attempts in 1 h 50 min on 2026-08-13 — 56 × `exit_code: -1`, then 1 success.** 54 of the 57 surviving transcripts end mid-`git push`. The mechanism: unbounded `git push` pack-objects materializing a **422-commit unpushed backlog** still carrying retired bead-forge object mass, on a repo that still **tracked `.beads/` state**, inside the needle dispatch scope's **12 GiB `MemoryMax`**. The named alert instant (08:23:51.806Z) is the post-kill heartbeat; the kill is `agent.completed exit -1` at **08:23:44.918Z**, 13.8 s into attempt 30's push. The loop ended when attempt 57 **skipped the push** (snapshot to `/tmp`) and closed the bead — recovery by behavior change, not remediation. The kill class is INFRASTRUCTURE; the alerts are FALSE_POSITIVE (the bead self-recovered); domain-check code is uninvolved.

Two figures make this crash distinct from its siblings and are the reason a separate gap analysis is warranted:
- **bf-1s6c3 / bf-4yjq (Aug-12)** died in *gc and general git operations* against an 18 GB loose-object store. Every layer that class covers is built, verified, and replay-tested.
- **bf-1ea4g (Aug-13)** died in *push* against an **unpushed-commit backlog** — a precondition dimension that **no safeguard in this repo measures even today** (§4, M-1). It is also the clearest recorded instance of the **self-amplifying retry loop** (§2, S3), which remains the largest unowned residual.

---

## 2. The sequence, decomposed into seven stages

| # | Stage | What happened (bf-1ea4g figures) | Prevention on Aug-13 | Prevention today (live-verified 2026-09-07) | Residual |
|---|---|---|---|---|---|
| S1 | **Repo precondition** | `.beads/` still tracked (attempt 30's `git commit -am` swept a tracked `.beads/.bf_history` file); backlog 660 → 422 commits of retired bead-forge mass | none | ✅ closed: `.beads/` gitignored repo-wide, `git ls-files .beads` → 0; 10 MB + 50 MB pre-commit gate (installer `--check` exit 0); repo 105 MB / 267 loose / 2.71 MiB, pack 99.11 MiB, 0 garbage; daily 02:00 health + bounded 03:00 gc | none in this repo |
| S2 | **The trigger operation** | unbounded `git push` → pack-objects → memcg-OOM-class SIGKILL inside the 12 GiB scope | none — no `pack.*` config existed | ✅ closed: `pack.windowMemory=2g` / `deltaCacheSize=1g` / `threads=1`, repo **and** global (`setup-git-gc-config.sh --verify` exit 0 today; `check-repo-health.sh` re-runs it daily and logs CRITICAL if the effective bound disappears); replay test asserts **this exact death operation** — a 192 MiB unpacked backlog pushed under `MemoryMax=768M` — passes (`test-gc-memory-bounds.sh` 12/12, integration case "the bf-1ea4g death operation") | none in this repo |
| S3 | **Self-amplifying retry loop** | needle re-claimed ~every 2 min for 110 min; each attempt *committed another snapshot before pushing*, so every retry's push was bigger than the last (attempt 30 read main's tip as the prior attempt's commit, 37 s old) | none | ❌ **unchanged** — nothing bounds the loop or checks "is the work already satisfied / has this bead died N times with one signature" before re-claiming | **H-1 — the largest residual, and it is not in this repo** (§4) |
| S4 | **In-storm detection** | 110 min, 56 kills, box CPU-saturated all morning (71/71 samples, peak 19.87/9 cores) — zero repo-owned detection existed (every monitor in the workspace landed 2026-09-01/02; `.beads/logs/` starts Sep-1) | none | 🟡 would now fire but not act end-to-end: `crash-pattern-detection.sh` (10-min timer) flags per-bead ≥3 crashes/24 h — print-only, no consumer (D-9); `system-event-mode.sh` surge gate is live and **wired into `preflight-health-check.sh`** (exit 75 = defer, fails open) — but deferring *new* work does not touch the retry loop already spinning on S3; host-wide resource monitoring does not see the 12 GiB memcg doing the killing (P1) | M-2, M-3, L-1 (§4) |
| S5 | **Alert generation** | one alert bead per kill (pre-dedup needle): 58 ALERT-shaped beads against a target that closed the same morning | none | 🟡 design landed 2026-09-02 (six fixes, suite 12/12) but **not in the alert-creation path** and individually inert where it matters (D-3 banner-as-classification, D-4 title regex never matches `ALERT: Agent crash on bead bf-…`, D-5 exit-code parse misses 99.7 % of traces) — §6 | H-2 (§4) |
| S6 | **Loop termination** | attempt 57 succeeded by *skipping the push* — behavior change, not remediation; backlog still ~500 commits at 09:28:57Z and drained only later | none | 🟡 the close-time gate now exists (`verify-work-completion.sh` counts ahead/behind and fails the close on an unpushed deliverable) but nothing records that a bead's *green outcome came from abandoning the standard path* — inventory learning #6 | folded into H-1 (the same loop-blindness) |
| S7 | **Investigation latency** | mechanism undetermined for **25 days** (Aug-13 → the 2026-09-07 transcript re-derivation); the interim corpus produced 4 circulating crash instants and 5 competing root causes (SIGHUP, host-RAM OOM, Aug-16 systemd-oomd, healthy-repo, bloat) | n/a | 🟡 improved but not closed: kernel + journald retention now understood as the missing record (no Aug-13 kernel record *can* exist — single boot from Aug-15 19:56:33 EDT), but single-slot traces remain and **worker transcripts** — the evidence that actually settled this crash — have no retention requirement at all | M-4 (§4) |

**Reading the table:** S1 and S2 — the stages this repo owns — are fully closed and *proven* by a replay test of the exact death operation. Everything still open is in S3–S7: the loop, the alert path, and the evidence layer.

---

## 3. What detection missed — then, and would it miss it again?

**On 2026-08-13, detection missed everything, by absence.** No repo-owned monitor existed. The only signals that fired that morning were needle's own `fleet.cpu_saturated` samples (71/71 saturated 07:00–10:00Z, peak 19.87) — recorded and acted on by nothing — and the kills themselves. The first repo-side artifact was the alert beads, one per kill, which *was* the incident's chief cost amplifier.

**A bf-1ea4g repeat today would be detected, partly deflected, and still not stopped:**

| Signal | Would fire today? | Then what? |
|---|---|---|
| Per-bead repeat crashes (`crash-pattern-detection.sh`, ≥3/24 h) | ✅ within ~10–15 min of storm start | a log line (D-9: no consumer) |
| Synchronized exit-wave / crash-burst (`system-event-mode.sh`) | ✅ | preflight returns **75** for anyone who runs it → new dispatches defer; nothing tells the running loop to stop |
| Pack-bound regression (`check-repo-health.sh` daily → CRITICAL to `repo-health.log`) | ✅ if the bound were removed | a log line; the bound's absence would not itself be *caused* by the storm |
| Unpushed backlog growth | ❌ **no such check exists anywhere** (§4 M-1) | — |
| Dispatch-scope memory pressure (the actual kill boundary) | ❌ host-wide monitors only (requirements P1) | — |
| Retry-loop stop-condition | ❌ NEEDLE-side, unowned | — |

So the honest answer to "what did detection miss" has two halves: *everything, then* — since repaired to the point where the storm would page within minutes — and *the two things that would still decide the outcome*, scope-level memory telemetry (M-2) and the loop's stop-condition (H-1), neither of which is a missing detection *rule* in this repo.

---

## 4. Gaps that remain open, prioritized

Priority = (cost of the gap recurring) × (is anyone already carrying it) ÷ (ownership reach). Items already carried by another bead or analysis are marked so they are not double-implemented.

### H-1 — The retry loop that multiplies one kill into 56 (NEEDLE-side; unowned)
Nothing checks, before re-claiming a bead, whether the previous attempts died identically on the same operation. bf-1ea4g's loop re-claimed ~every 2 min for 110 min, and because each attempt committed before pushing, **the loop grew its own kill condition** — the reason 1 kill became 56. Identical shape on record: bf-1s6c3 (71 kills / 265 min, named "the re-dispatch stop-condition" residual in [`docs/crash-mitigation-strategies.md`](crash-mitigation-strategies.md)) and the auto-split-on-resolved family (requirements G-9/G-12 are the adjacent registered asks; this is their loop-side twin).
*Requirement (external, NEEDLE):* a per-bead consecutive-failure stop-condition — after N deaths with the same exit sentinel *and* the same last-tool-call operation, release the bead and raise one (1) system event instead of re-claiming; plus the "did the work already get satisfied" check before claim.
*Why H:* it is the difference between one kill and an incident. Every other layer is already built.

### H-2 — Put false-positive prevention in the path that creates alerts (carried — do not re-implement)
The six 2026-09-02 fixes exist and their suite passes, but the manager is invoked by nothing in production and its four load-bearing paths are broken at the parse level (D-1..D-10). For bf-1ea4g this is not hypothetical: **58 ALERT-shaped beads against a target closed the same morning; 12 still unresolved today (46 closed / 3 in_progress / 9 open — census in §6)**. The sibling analysis's §5 P1/P2 items are the implementation spec; the untracked `scripts/alert-triage-sweep.sh` + `domain-check-alert-triage.timer` (hourly, firing as of today) are a sibling's in-flight implementation of its P1(a)/P4 — treat those as the carrier, don't build a second one. What this doc adds: **the backlog of already-created duplicate alerts is being drained manually, one closure-bead at a time, while the gate that would have prevented them stays out of the path.** The repair should be judged by that ledger flipping to non-empty, not by test suites.

### M-1 — Monitor unpushed-backlog growth (this repo; the one new rule this crash motivates)
**Nothing in the workspace measures commit-ahead.** The only ahead/behind count is `verify-work-completion.sh:168` — evaluated once, at close time, per bead. The 422-commit condition that turned bf-1ea4g's push lethal accumulated silently across ~30 killed attempts, and if it ever regrows (a dead worker's unpushed series, a loop like S3), no daily check, no preflight, and no monitor would mention it.
*Requirement:* add to `scripts/check-repo-health.sh` (already daily-02:00-scheduled, already runs the pack-bound verify and self-appends CRITICAL lines to `.beads/logs/repo-health.log`):
`git rev-list --count "@{upstream}..HEAD"` (fallback `origin/main..HEAD`), warn at ≥50, CRITICAL at ≥200, reported alongside the existing size/object table. Milliseconds to run, zero remediation attached — this stays a report, per the G-2 correction (the 03:00 timer owns remediation; conditional gating must not be re-invented here).
*Why M and not H:* with the pack bound in place a regrown backlog degrades (slow pushes, close-gate failures, remote-side growth) rather than kills — this is a regression-guard on the *precondition*, cheap insurance against the one crash dimension with zero telemetry. It is also the only recommendation in this document that is new: neither the requirements doc's G-1..G-13 nor the dedup analysis' D-1..D-10 registers it.

### M-2 — Dispatch-scope memory telemetry (infrastructure-side)
The boundary that killed every one of the 56 was the **12 GiB memcg**, and nothing then or now observes a dispatch scope's memory — resource monitoring is host-wide, and requirements P1 already states why that misses the binding constraint. A push sitting at 11 GiB for 13.8 s is invisible until it is a kill record.
*Requirement (external):* per-scope `memory.peak`/`memory.pressure` capture into the fleet log (the kernel already maintains it; only retention is missing), so "push X exceeded Y GiB" is a queryable event instead of an inference from a sentinel. Note: an untracked `scripts/memory-watch.sh` + test sits in the worktree (sibling work in flight, not yet committed or verified) — check its bead before starting this.

### M-3 — Evidence retention, with the bf-1ea4g-specific addition: transcripts (extends G-8)
The mechanism took 25 days to settle because the Aug-13 record is structurally absent: journald's first entry postdates the crash (single boot 2026-08-15 19:56:33 EDT), the health collector starts 2026-08-15 23:53 EDT, traces are single-slot. What *did* survive — needle's worker logs and the 57 session transcripts — settled it in one session, via last-tool-call analysis (inventory learning #2). G-8 already registers kernel/journald retention, trace rotation, UTC discipline, gc.log; **it does not name worker transcripts, which are the evidence class that actually closed this crash.**
*Requirement:* add transcript/worker-log retention (≥30 days) to G-8's list, and mark it the highest-value line of the four for crash attribution — kernel records prove *that* a memcg kill happened; transcripts prove *what the agent was doing*, which is what four wrong root causes in this corpus lacked.

### M-4 — Correct the prevention-status overclaims (documentation; cheap)
[`docs/comprehensive-crash-prevention-guide.md`](comprehensive-crash-prevention-guide.md) — the live file answering to the dispatch's "crash-prevention-guide.md" — states as ✅ ACHIEVED: *"false positive rate <5 % (95 %+ reduction)"*, *"false positive alerts REDUCED by 95%+"*, and *"0 crashes in 16+ days post-remediation"*. The first two are contradicted by the same-day dedup analysis (the in-repo pipeline has never fired in production; 176 open alert beads point at closed targets fleet-wide); the third was a 2026-09-02 snapshot, not a standing metric. A reader who trusts those lines would conclude §6's work is done — which is precisely how gaps survive in this corpus (four circulating crash instants for this one bead came from the same dynamic).
*Requirement:* dated correction note on those three metric lines pointing at the dedup analysis and requirements §6 — the live-doc analogue of the archive-freeze banner convention; the file is not archived, so an in-place dated note (not a rewrite) is the right instrument. Also fix the dispatch template's `crash-prevention-guide.md` path.

### L-1 — CPU-saturation response (external; amplifier, not mechanism)
71/71 saturated samples that morning, warnings only, no throttle — registered as G-13. bf-1ea4g's kills were operation-correlated, not load-correlated (54/56 land on one operation across 1 h 50 min), so saturation amplifies but does not explain this crash. Keep as the registered external ask; no new work here.

### L-2 — `repo-health-monitor.sh` pack-count misfire (recorded; implementation bead owed)
Warns/fails whenever a repo holds >2 pack files regardless of size — flagged 3 packs / 99 MiB on a healthy 103 MB repo (recorded 2026-09-07 by domchk-4e8821ca in the mitigation-strategies v2.3 addendum). Not wired to any timer, so it can only mislead a manual run. Carry as the already-recorded calibration note; nothing new to add.

---

## 5. Are new detection rules needed? (direct answer)

**One, yes:** unpushed-backlog growth (M-1). It is the only dimension of this crash that no existing rule, timer, or gate measures, and it is the crash's signature precondition.

**Two, no — and adding them would be worse than the gap:**
- *A rule for the kill operation itself* is not needed: the operation is memory-bounded (`pack.windowMemory` chain, verified exit 0 today) and the bound is **proven against this crash's exact replay** — `test-gc-memory-bounds.sh`'s integration case reconstructs a near-identical-shape unpacked backlog and asserts the push survives `MemoryMax=768M`. A detector watching for "large push in progress" would page on an operation that is now safe.
- *A second surge/repeat detector* is not needed: `crash-pattern-detection.sh` and `system-event-mode.sh` already detect both shapes a bf-1ea4g storm produces. What they lack is a consumer (D-9) and a loop-side actor — wiring, not rules.

**And one item that looks like a detection rule but is not:** the retry stop-condition (H-1) is a *policy change in the dispatcher* — detection without an actor is indistinguishable from no detection (D-9's own lesson). Filing it as a "new rule" would let it be "implemented" as another print-only log line.

---

## 6. False-positive prevention measures, evaluated for bf-1ea4g

**The two-layer lesson this bead teaches** (inventory §1, §8.4): the kill was real and infrastructure-caused; the alerts were false-positive-shaped because the bead self-recovered 88 min after the first kill, deliverable intact. Corpus documents that argued "FP vs INFRASTRUCTURE" as mutually exclusive were arguing past each other. Any FP-prevention measure must therefore decide *which layer it is suppressing* — suppressing the alert is right here; suppressing the kill would have been wrong.

**Measures evaluated (all six 2026-09-02 fixes + the two satellite tools):**

| Measure | Intended effect on a bf-1ea4g-shaped event | Verified reality (D-refs) | Verdict |
|---|---|---|---|
| Fix 1 — closed-bead filter | 56 of 58 alerts suppressed at creation (target closed 09:10:16Z same day) | keyed to the alert bead's own ID shape (`^bf-` gate) and a title regex that never matches `ALERT: Agent crash on bead bf-…` (D-4); exit-code parse empty on 99.7 % of traces (D-5) | design right, **never fires on the real shape** |
| Fix 2/3 — duplicate detection + processed-alerts ledger | 2nd..56th alerts for the same kill suppressed | instance-keyed, not crash-keyed (D-2); ledger 0 lines ever; no production caller | inert |
| Fix 4 — completion awareness | post-completion deaths not alerted | same parse bugs; the marker it should consult (`verify-work-completion.sh`) is written but never read by the alert path (D-1/G-9) | inert |
| Fix 5 — 5-min cooldown | 56 kills → a few coalesced alerts | keyed on classification, which is parsed as a banner line (D-3), and suppresses without recording (D-7) | would mis-fire if D-3 were fixed alone |
| Fix 6 — crash classification | INFRASTRUCTURE vs FALSE_POSITIVE decided once | `CLASSIFICATION` is literally `=====` (D-3); the manager's own log shows it | broken |
| `alert-deduplication.sh` | per-target duplicate verdict | fleet-wide prose report, grep-inverted (suppresses when *no* duplicate exists), CWD-dependent (D-6) | the weakest link |
| `crash-resolution-tracker.sh check` | "is this crash resolved?" gate | consults a 0-record ledger instead of the live store: returns NOT_RESOLVED for a bead whose `Status:` is `Closed` (D-1) | inverted in effect |
| NEW: `system-event-mode.sh` surge gate | defers new work during a storm | live, 32/32 tests, wired into preflight with the 75-deferral contract (verified this session) | working — but orthogonal to FP suppression (it gates *work*, not alerts) |
| NEW (in flight): `alert-triage-sweep.sh` hourly timer | drains the existing duplicate backlog | untracked sibling work implementing dedup-analysis P1(a)/P4; firing as of today | right instrument for the backlog half |

**Live census (this session; three prior censuses agree on the shape):** 88 beads title-mention bf-1ea4g · **58 ALERT-shaped** (one per kill until needle dedup landed) · **46 closed / 3 in_progress / 9 open = 12 unresolved, 25 days after the target closed**. The fleet-wide version of the same number: 176 open alert beads point at closed targets.

**Assessment:** the false-positive *design* is correct and was verified at the unit level (12/12), but for the event shape this bead actually produced it has suppressed nothing, ever. The gap is not a missing measure — it is (a) the parse-level defects D-3/D-4/D-5, (b) absence from the creation path, and (c) the source-side gap only NEEDLE can close (G-9). Until then, the load-bearing FP prevention in this workspace is the *human-process* one CLAUDE.md already prescribes: verify the target bead's actual state before investigating — which is exactly what drained 46 of these 58.

---

## 7. Bottom line

**Not owed on this crash:** everything in S1–S2. The repo-side prevention for bf-1ea4g's mechanism is the most complete in the corpus — re-entry block, commit gate, memory bound (repo + global), daily verification of that bound, and a replay test of the death operation itself. §4 M-1 is the single new thing this crash motivates, and it is a ~10-line report-only addition to an existing scheduled check.

**Owed, in order:** H-1 (NEEDLE retry stop-condition — the multiplier), H-2 (carried; judge by the ledger flipping, not tests), M-1 (this repo, cheap, unregistered until now), M-2/M-3 (telemetry + transcript retention — both cheap infrastructure asks), M-4 (doc corrections), L-1/L-2 (registered, no new work).

**And one meta-gap the corpus keeps re-deriving:** this dispatch's own named input did not exist, and the guide that does exist overstates the FP layer as ACHIEVED. Prevention-status documents in this workspace have repeatedly been *ahead* of the mechanisms they describe (M-4, requirements §6, bf-4k2ws §14.6). A gap analysis that trusted either would have reported this crash fully covered.

---

## 8. Live verifications performed this session (2026-09-07)

```bash
bead show bf-1ea4g                 # Status: Closed 2026-08-13T09:10:16Z
bead list --limit 20000 --json     # census: 88 title-mentions / 58 ALERT-shaped / 46 closed-3 ip-9 open
systemctl --user list-timers 'domain-check-*' --all   # 8 timers, all future-scheduled (incl. hourly alert-triage)
./scripts/setup-git-gc-config.sh --verify             # exit 0
./scripts/setup-git-hooks.sh --check                  # exit 0
git ls-files .beads | wc -l        # 0
du -sh .git; git count-objects -vH # 105 MB; 267 loose / 2.71 MiB; pack 99.11 MiB; 0 garbage
git rev-list --count origin/main..HEAD; git rev-list --count HEAD..origin/main  # 0 / 0
grep -n "rev-list\|ahead\|backlog" scripts/*.sh       # only verify-work-completion.sh counts ahead/behind (close-time)
grep -n "DUPLICATE_THRESHOLD" scripts/crash-pattern-detection.sh  # =3, print-only
sed -n 95,125p scripts/preflight-health-check.sh      # system-event gate wired, exit 75 deferral, fails open
```

Source-line claims about the alert manager/classifier (D-1..D-10) are cited to [`docs/alert-deduplication-gap-analysis-2026-09-07.md`](alert-deduplication-gap-analysis-2026-09-07.md) rather than re-verified line-by-line here; the census, the timers, the gates, and the repo state above are first-hand.

## 9. References

- Canonical crash record: [`docs/crash-inventory-bf-1ea4g-summary.md`](crash-inventory-bf-1ea4g-summary.md) → `docs/investigations/bf-1ea4g-root-cause-determination-2026-09-02.md`, evidence bundle `docs/crashes/bf-1ea4g/`
- Gap registers: [`docs/crash-prevention-requirements.md`](crash-prevention-requirements.md) (G-1..G-13), [`docs/alert-deduplication-gap-analysis-2026-09-07.md`](alert-deduplication-gap-analysis-2026-09-07.md) (D-1..D-10)
- Prevention docs reviewed: [`docs/comprehensive-crash-prevention-guide.md`](comprehensive-crash-prevention-guide.md), [`docs/crash-mitigation-strategies.md`](crash-mitigation-strategies.md) (v2.3)
- Replay test of the death operation: `scripts/test-gc-memory-bounds.sh` (integration case, lines 150–206)
- Standing procedure: `CLAUDE.md` ("Crash Prevention and Investigation"), `docs/crash-response-guide.md`
