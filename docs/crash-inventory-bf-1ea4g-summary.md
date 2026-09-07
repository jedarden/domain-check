# Crash Inventory & Consolidated Summary — bead bf-1ea4g

**Compiled:** 2026-09-07 by investigation bead **domchk-6a5f4207**
**Target bead:** bf-1ea4g — "Document local main branch state" (created 2026-08-13T07:14:47Z, **Closed 2026-08-13T09:10:16Z** — re-verified live this session via `bead show`)
**Purpose:** One reference document that states the current, evidence-grounded determination for bf-1ea4g, inventories every investigation document that bears on it, and records which claims in the older corpus are superseded — so subsequent investigations cite the right record instead of re-deriving (or re-importing) a wrong one.

**How to read this document:** everything in §1–§5 is the *current* determination. §6–§7 inventory the corpus and its contradictions. The corpus spans 2026-08-16 → 2026-09-07 and disagrees with itself extensively (§7); where a claim is wrong, the superseding evidence and its source are named. Nothing here rewrites another document — per the archive freeze policy, superseded claims are corrected *by pointing at them*, not by editing them.

---

## 1. Determination at a glance

| Field | Current determination | Confidence / basis |
|---|---|---|
| **Crash(s)** | **Not one crash — 56 kills + 1 success.** 57 dispatch attempts 07:17:49Z → 09:08:30Z on 2026-08-13: **56 × `exit_code: -1`**, then 1 × exit 0 (attempt 57) | HIGH — raw worker log, `docs/crashes/bf-1ea4g/attempt-index.tsv` (commit 2ce9cd9) |
| **Crash instant (the one an alert names)** | Alert bf-1nb5u's `2026-08-13T08:23:51.806Z` is the post-kill `HANDLING_RELEASE_DONE` heartbeat. The actual kill is `agent.completed exit_code: -1` at **08:23:44.918Z** (duration 92,079 ms) — **attempt 30 of 57** | HIGH — resolved byte-exact from the worker log, bundle README |
| **Exit code** | **−1 = needle's died-without-exit-code sentinel.** It identifies **no signal number** (not SIGHUP, not SIGKILL-9) | HIGH — 2026-09-07 fleet-wide reclassification |
| **What the agent was doing when killed** | **Mid-task, mid-`git push`** — 13.8 s inside the push, with no result recorded. **Not** post-completion cleanup: the bead was still open, 47 min before it closed | HIGH — attempt-30 session transcript ends on an unanswered `tool_use` at 08:23:31.123Z |
| **Classification — the kill** | **INFRASTRUCTURE** — the repository-bloat-era kill regime: memcg-OOM-class SIGKILL of unbounded `git push` inside the 12 GiB per-dispatch `MemoryMax` | HIGH on the operation + mechanism class; **MEDIUM** that memcg OOM was this exact instant's killer (§4) |
| **Classification — the alert(s)** | **FALSE_POSITIVE** — the bead self-recovered and closed successfully the same morning; zero data loss; domain-check code uninvolved | HIGH — re-verified 2026-09-07 (domchk-596f8499) |
| **Root cause** | Unbounded `git push` (pack-objects) materializing a **422-commit unpushed backlog** still carrying retired bead-forge object mass, on a repo still tracking `.beads/` state, inside the dispatch scope's 12 GiB `MemoryMax` | HIGH (§4) |
| **Resolution** | Bead closed 2026-08-13 by attempt 57 (which skipped the push); repo de-bloated and bounded afterwards; every protective layer now in place and **live-verified 2026-09-07** (§5) | LIVE-verified this session |
| **Code defect** | **NONE** — the killed process was `git`; consistent with every investigation in this corpus (zero domain-check defects) | HIGH |

The two classification lines are not a contradiction: *the kill was real and infrastructure-caused*; *the alert was false-positive-shaped* because the work did not need rescuing. Those are two different layers, and conflating them is the main source of confusion in the older corpus.

---

## 2. Timeline (2026-08-13, all UTC)

| Time | Event |
|---|---|
| 07:14:47 | Bead bf-1ea4g created ("Document local main branch state" — a read-only snapshot task) |
| 07:17:49 | First dispatch; the kill storm begins. It runs 1 h 50 min |
| 07:04Z | — (context: bf-4k2ws's 55-kill storm ends 13 min before this bead starts — the documented rolling-handoff pattern) |
| 07:00–10:00 | Box CPU-saturated continuously: 71/71 `fleet.cpu_saturated` samples above threshold on 9 recorded cores, window peak **19.87** @ 08:37:03Z |
| 08:22:12 | Attempt 30 dispatched; 1-min load **10.80** |
| 08:22:32 | Attempt 30 reads main's tip: `33356c14…` — *the prior attempt's own commit, 37 s old*. Main had advanced one snapshot commit per killed retry (self-amplifying backlog) |
| 08:23:01–13 | Attempt 30 writes + verifies its actual deliverable (trivial) |
| 08:23:18 | Attempt 30 commits: `6da701b` — the `-am` sweep deletes a **tracked** `.beads/.bf_history/issues-*.jsonl` file, proving `.beads/` state was still in git that morning |
| 08:23:31.123 | Attempt 30 issues `git push` — **no result follows** |
| **08:23:44.918** | **Kill recorded: `agent.completed exit_code: -1`** (attempt 30 of 57) |
| 08:23:51.806 | Post-kill heartbeat — the instant alert beads carry; bf-1nb5u created 08:23:51.812Z |
| 09:08:30 | 56th and last `exit_code: -1` |
| 09:08:39 | Attempt 57 succeeds — by *skipping the push*: it wrote its snapshot to `/tmp` and closed the bead. Main was still ~500 commits ahead at 09:28:57Z (sibling commit `1ec2354`), so the loop ended by behavior change, not remediation |
| 09:10:16 | **Bead bf-1ea4g CLOSED** |

Attempt distribution of last recorded tool call (all 57 surviving transcripts, first-hand 2026-09-07): **54 died inside `git push`**, 2 inside `git add && git commit` (08:18:12Z, 08:55:16Z), 1 = `bf close` (success).

**Era context:** Aug-12 measured 18 GB `.git` / 17.16 GB loose objects (bf-1s6c3 / bf-4yjq); Aug-13 still had `.beads/` tracked and a 422-commit unpushed backlog (660 on Aug-12 → 0 only after later plain pushes); Aug-16 measured the same mechanism kernel-proven at 720 commits / 5.6 GB (bf-198ne). Repaired 2026-09-01, re-verified 2026-09-06 and today.

---

## 3. Classification (dispatch taxonomy: INFRASTRUCTURE / WORKFLOW / SERVICE / CODE_DEFECT)

| Class | Verdict | Why |
|---|---|---|
| **INFRASTRUCTURE** | ✅ **This is it** (the kill) | Operation-correlated deaths: 54/56 land on one heavy git operation scattered over 1 h 50 min — not time-correlated, not fleet-event-correlated. Mechanism kernel-proven for the era and for sibling instants (bf-198ne: `task=git`, `CONSTRAINT_MEMCG`, at 100 % of the 12 GiB `MemoryMax`) |
| **WORKFLOW** (max-turns etc.) | ❌ Ruled out | No `max_turns` evidence anywhere in the attempt record; deaths are mid-operation, not turn-limits |
| **SERVICE** (gateway 5xx) | ❌ Ruled out | No 5xx class; the operation died locally 13.8 s in, with no result rather than an error result |
| **CODE_DEFECT** | ❌ Ruled out | The killed process was `git`; the workload a docs snapshot. No application error in any of the 57 transcripts (the two "panic" strings are fuzz-test comments inside the dispatch prompt) |
| **Alert layer** | **FALSE_POSITIVE** | The bead self-recovered: attempt 57 closed it 88 min after the first kill, deliverable intact, zero data loss. 88 beads carry bf-1ea4g in their title — 56 of them ALERT-shaped, i.e. one per kill — which is the alert-generation defect, not a defect in the work |

**SIGHUP:** retired. The 2026-09-02 corpus read `exit -1` as SIGHUP for the FALSE_POSITIVE docs and SIGKILL-9 for the OOM docs; both are wrong for the same reason — the sentinel carries no signal number, and a SIGHUP cascade would strike irrespective of the running operation.

---

## 4. Root cause

**Primary:** unbounded `git push` — pack-objects materializing every object the remote lacks over a multi-hundred-commit unpushed backlog still carrying retired bead-forge mass, inside the needle dispatch scope's 12 GiB `MemoryMax` → memcg-OOM-class SIGKILL → needle's `exit_code: -1` sentinel.

- **Backlog:** 660 commits ahead on Aug-12 → **422 on Aug-13** → 0 only after later plain pushes (`docs/branch-divergence-analysis.md`).
- **What it carried:** retired bead-forge state. Kernel-proven three days later with the size attached — bf-198ne (Aug-16): 720-commit backlog whose tree carried **5.6 GB**, killed at the `MemoryMax` ceiling.
- **The bound that turned "big" into "dead":** no `pack.windowMemory` / `deltaCacheSize` / `threads` config existed until 2026-09-02. The window limit is per-thread, so `pack.threads=1` matters.

**Amplifiers (why 56 times, and why 88 alert beads):**
1. **Self-amplifying retry loop** — every killed attempt had already committed another snapshot onto main, so each retry's push was bigger than the last. Nothing bounded the retry.
2. **No memory bound on git transport** (landed 2026-09-02, covering gc *and* push).
3. **Pre-dedup alerting** — one alert bead per kill: 88 bf-1ea4g-titled beads against a target that closed the same morning.
4. **Fleet CPU saturation** all morning (peak 19.87 on 9 recorded cores) — amplifies per-operation memory pressure and slows pushes; context, not the mechanism.

**Resources at the named instant:** load 10.80 at dispatch / 9.86 twelve seconds post-kill (neighbors 9.84 / 9.89); **no memory or disk record exists for Aug-13** (first sampler starts 2026-08-15 23:53 EDT; needle emitted no memory/disk event type that day). Host RAM was not the kill boundary — the charged memcg's 12 GiB was.

**Confidence tiers:** HIGH on the operation (54/57 transcripts, first-hand) and on the mechanism class (kernel-proven combination on adjacent days); **MEDIUM that memcg OOM was this specific instant's killer** — no Aug-13 kernel record can exist (journald's first entry is 2026-08-15 19:56:33 EDT, single boot; earliest surviving OOM line is Aug-16 00:27:35 EDT). That unknowability is a coverage gap, not evidence for any alternative mechanism.

**Alternatives ruled out:** network/remote error (would return exit 1 + an error result; these pushes return *no result*); needle timeout (recorded as exit 124 — 5 same-day bf-4k2ws specimens, none here); disk exhaustion (no error signature in any attempt); SIGHUP (sentinel has no signal number; deaths are operation-correlated); code defect (uninvolved).

---

## 5. Resolution status — live-verified 2026-09-07 (this session, unless noted)

| Item | Status | Evidence |
|---|---|---|
| Target bead bf-1ea4g | ✅ **Closed** 2026-08-13T09:10:16Z | `bead show` this session |
| Deliverable | ✅ On origin/main — `main_branch_state_bf-1ea4g.json`, blob `e77648c7` | `git ls-tree origin/main` this session |
| Repository | ✅ Healthy: `.git` 105 MB, 251 loose objects, 1 pack 99.11 MiB, 0 garbage | `du` + `git count-objects -vH` this session |
| Git-transport memory bound (the direct fix for this mechanism) | ✅ Effective bound present, gc *and* push, ≈3 GiB worst case | `./scripts/setup-git-gc-config.sh --verify` exit 0 this session |
| `.beads/` re-bloat path | ✅ Closed — whole directory gitignored, 0 tracked files | repo `.gitignore`; re-verified in canonical §R3 |
| Alert-system fixes (6 critical: closed-bead filter, duplicate detection, processed-alert tracking, completion awareness, exit-code validation, cooldown) | ✅ Implemented 2026-09-02; suite 12/12 re-verified 2026-09-06 | `docs/crash-alert-fix-implementation-2026-09-02.md`; CLAUDE.md |
| Residual alert pool | 🟡 Draining: **88** beads title-mention bf-1ea4g; **56 ALERT-shaped** (one per kill); **12 open/in_progress** | `bead list --json` census this session |
| Repo de-bloat holding | ✅ (105 MB today; was 18 GB era) | verified 2026-09-06 + this session |

**Nothing is owed on the kill mechanism:** the fix layer (repo de-bloat, `.beads/` gitignore, pack bounds, backlog-draining pushes) had already landed when the 2026-09-07 re-determination was written, and it proposed no new fix.

### 5.1 Fix implementation record (Child 3, domchk-9d840579, 2026-09-07)

The fix needed no new mechanism — the bound above was already effective, re-verified live this session (`./scripts/setup-git-gc-config.sh --verify` exit 0, worst case ≈3072MiB, supplied repo-locally and globally). What was missing was a recurrence test for *this* death operation: `scripts/test-gc-memory-bounds.sh` replayed only the gc-side crash command (bf-173o7e/bf-4x12ec), and the push-side pair in `test-bf-1s6c3-crash-condition.sh` brackets the extremes (A2: no bounds → SIGKILL at `MemoryMax=512M`; B2: bounds over a *packed* store → exit 0) without the middle case — bounds over the **unpacked** backlog, the exact state bf-1ea4g pushed from.

That case is now asserted in the suite (test adopted from a prior attempt's uncommitted work, then fixed and completed): bounded `git push` over a 192MiB unpacked 6×32MiB near-identical-snapshot backlog under `MemoryMax=768M` (1/16th of the 12GiB dispatch scope) — exit 0, the bare remote receives the backlog, the store stays loose (push alone did no gc), peak push RSS **232,504KB** vs the >12GiB the unbounded push consumed on 2026-08-13. Full suite **16/16** (gc replay unchanged: peak 320,532KB). Also fixed en route: the push test now runs with the suite's GNU `time -v` wrapper, which was defined after its first use.

### 5.2 Preventive-measures verification — the bf-393iv umbrella chain (domchk-e4b94513, 2026-09-07)

bf-393iv is the alert-layer twin of this record: the ALERT umbrella minted for bf-1ea4g's 08:58:35Z kill — one of the 88 bf-1ea4g-title-mentioning alert beads counted in §5 — carrying the same 2026-08-16-era "SIGHUP cascade / repo still 18GB" premises that §6.2 and §7 supersede. Its own two earlier verification reports (ef95fec, 2026-08-26; the meta-verification of domchk-690f7a43, 1748787, 2026-09-01) live frozen under `docs/archive/crash-investigations/`. Its chain: domchk-4e2f7b61 (remediation decision — FALSE_POSITIVE, no action required; closed 2026-09-02) and domchk-e4b94513 (verify + document, this subsection), the latter reached through domchk-89147775 (implement preventive measures per root cause domchk-6bdfe4dc = INFRASTRUCTURE, unbounded git-push pack-objects over the 422-commit backlog; closed 2026-09-07). The implement step found every measure in its dispatch's INFRASTRUCTURE branch already committed on origin/main and fixed the one live defect it met: `scripts/verify-work-completion.sh` died of instant SIGPIPE (exit 141, zero output) under `set -euo pipefail` whenever the shared worktree's dirty/staged count outgrew one early-exiting `head -5` read — fixed to `sed -n '1,5p'` in c3afc8a, folded into the restored tree as d6f8b53. A false-*blocking* failure of the verification tooling, not a new crash mechanism.

The terminal verify step re-ran the whole stack live 2026-09-07 and passed all of it:

| Measure (target: the unbounded-push-over-unpacked-backlog mechanism) | Result, 2026-09-07 |
|---|---|
| `./scripts/check-repo-health.sh` | exit 0 — 106 MB, 319 loose objects (3 MiB), 1 pack 99.11 MiB, 0 garbage, 11,360 in-pack; pack-memory-bound section ✅ |
| `./scripts/setup-git-gc-config.sh --verify` (the direct fix) | exit 0 — effective bound ≈3072MiB worst case, within the 6 GiB ceiling; covers gc *and* push |
| `./scripts/test-gc-memory-bounds.sh` (recurrence replay, §5.1) | **16/16** — bounded push over the 192MiB unpacked backlog under `MemoryMax=768M`, peak push RSS **232,488KB** this run (§5.1's run: 232,504KB — two independent passes) |
| `./scripts/safe-git-gc.sh --check-only` | resource checks pass; "GC not needed" (exit 1 is the documented not-needed outcome) |
| `./scripts/test-safe-git-gc-limits.sh` | **33/33** |
| `./scripts/test-verify-work-completion.sh` (the c3afc8a fix) | **11/11** |
| Pre-commit 10 MB gate + gitignore | hook byte-identical to tracked source (`setup-git-hooks.sh --check` exit 0); `.beads/`, `*.db`, `*.jsonl` ignored, **0** tracked `.beads/` files |
| Scheduled maintenance | **8/8** `domain-check-*` systemd user timers future-scheduled |
| Regression check | `go build ./...` + `go test ./...` exit 0 — worktree including a co-tenant's in-flight `internal/resilience` + resource-monitor changes compiles and passes |

Chain verdict: domchk-4e2f7b61 closed 2026-09-02 + this bead closing leaves bf-393iv with **no open blockers** — the umbrella is closable, and its stale `verification-failed` label postdates a verification that has now passed twice (ef95fec, this subsection). No new crash pattern and no new measure owed: the chain's only novel finding is the SIGPIPE gate defect above.

### 5.3 Documentation record — Child 4 of the 2026-09-02 split (domchk-3c8eaafc, 2026-09-07)

Child 4's dispatch asked for a comprehensive investigation report in `docs/crash-investigations/`, compiled from Children 1–3, covering crash details (bead ID, agent, exit code, timestamp), classification, root cause, fix implementation, and evidence + verification, committed to git. Per the corpus dedup-append convention **this record is that report** — no new bf-1ea4g doc was created. Where each criterion lives:

| Acceptance criterion | Where it is documented |
|---|---|
| Crash details — bead ID, agent, exit code, timestamp | §1 (determination at a glance) + §2 (timeline). Agent first-hand from the bundle's raw log: `worker_id: claude-code-glm-4.7-lab-domain-check`, first claim 07:17:49.928Z |
| Classification | §3 — kill = **INFRASTRUCTURE**, alert = **FALSE_POSITIVE** (child 1, domchk-596f8499) |
| Root cause | §4 and the re-determination's §8 (child 2, domchk-c2b8c832): unbounded `git push` pack-objects over the unpacked 422-commit backlog inside the 12 GiB dispatch scope |
| Fix implementation | §5.1 (child 3, domchk-9d840579) + the §5 table |
| Evidence and verification results | Evidence bundle `docs/crashes/bf-1ea4g/` (2ce9cd9); §5.2's live stack; the re-verification below |

First-hand re-verification, this session (2026-09-07):

- Target bead **closed** 2026-08-13T09:10:16.731Z (`bead show`); bundle `attempt-index.tsv` = 1 header + 57 rows.
- `./scripts/setup-git-gc-config.sh --verify` **exit 0** — effective bound ≈3072 MiB worst case (windowMemory 2g / deltaCache 1g / threads 1), covers gc *and* push.
- `./scripts/test-gc-memory-bounds.sh` **16/16, exit 0**, re-run in a throwaway clone at this tip (c0f2b70) — including the bf-1ea4g death-operation replay: bounded push over the 192 MiB unpacked backlog under `MemoryMax=768M`, peak push RSS **232,336KB** (third independent pass — §5.1: 232,504KB, §5.2: 232,488KB), and the gc-side replay's pack-objects at 320,476KB.
- Repository: `.git` 106 MB, 323 loose objects (3.14 MiB), 1 pack 99.11 MiB, 0 garbage, `origin/main...HEAD` 0/0 — all from direct `du`/`count-objects -vH`/`rev-list` this session; `check-repo-health.sh` exit 0 over the live worktree.
- Co-tenant disclosure: the worktree's `scripts/check-repo-health.sh` carries an **uncommitted** +97-line addition implementing the M-1 unpushed-commit-backlog monitor (warn ≥50) from the gap analysis. It is in-flight work by another worker — not attributed as landed here; HEAD's version does not have it.

One deliverable-shaped gap this dispatch did surface: the doc sitting at the dispatch-named path — `docs/crash-investigations/bf-1ea4g-crash-investigation.md` (2026-08-17) — is outside the §6 census's reviewed scopes and still argued the pre-re-determination record. It now carries a dated correction banner pointing here; see §6.4.

### 5.4 Resolution record + re-verification (domchk-e2c1e79e, 2026-09-07 ~16:45 UTC)

The "verify crash prevention improvements and document learnings" dispatch closed with a resolution ledger at [`docs/crash-resolution-bf-1ea4g-final.md`](crash-resolution-bf-1ea4g-final.md) — a pointer-and-verification record, cross-registered here rather than replacing this document. Its independent passes on the same day as §5.1–§5.3: `test-closed-bead-filter.sh` **7/7**; `test-gc-memory-bounds.sh` **16/16** (push replay peak RSS **232,504KB**, byte-identical to §5.1's run — deterministic workload, measured live from GNU `time -v`); `test-crash-alert-fixes.sh` **13/13** (worktree copy carrying a co-tenant's uncommitted +38-line test, disclosed); `check-repo-health.sh` exit 0 (106 MB, 372 loose / 3.50 MiB, 0 garbage); preflight 4/4; 8/8 timers; 0/0 divergence; **zero** alert beads created during the runs. Status deltas since this document was compiled: **M-1 has landed** (8d326cc, `check-repo-health.sh` now reports unpushed-backlog counts — §5.3's "uncommitted, not attributed" no longer holds), the bf-titled alert pool drained 12 → **10 unresolved** (7 open / 3 in_progress; 24 unresolved if the domchk-* title-mentions are included, the last open split child domchk-cb9eb4de in flight), and the ALERT-shaped count is **56** under this session's `^ALERT` shape rule.

---

## 6. Document inventory — what was reviewed

**Census method:** `grep -rl "bf-1ea4g" docs/ --include="*.md"` → **138 files** (69 in `docs/archive/`). Every bf-1ea4g mention in `docs/*.md` (15 files) and `docs/investigations/` (9 files) was reviewed individually, plus all six bf-1ea4g-named substantive docs in the archive and the evidence bundle. The remaining ~63 archive files are per-alert duplicate verifications (names like `verification-report-bf-3u5gj-duplicate-alert-resolved-bf-1ea4g-crash.md`) that each restate the same false-positive verdict for one alert bead; they are characterized as a class, with two examples read in full.

### 6.1 Canonical record + evidence bundle (cite these)

| Document | Status |
|---|---|
| `docs/investigations/bf-1ea4g-root-cause-determination-2026-09-02.md` (871 lines) | **THE canonical bf-1ea4g record.** Original 2026-09-02 section (domchk-c918d20b: FALSE_POSITIVE / SIGHUP / healthy repo) sits under a dated **SUPERSEDED-IN-PART banner**; two 2026-09-07 sections appended per the dedup-append convention: the **root-cause re-determination** (domchk-c2b8c832 — mechanism, §2–§8) and the **system resource analysis** (domchk-508e54c0 — R1–R7). Everything in §1–§5 above is this document's content |
| `docs/crashes/bf-1ea4g/` (commit 2ce9cd9, domchk-93ac565b) | **Evidence bundle.** README + `attempt-index.tsv` (all 57 attempts with source line numbers), verbatim attempt-30 event bracket (worker-log lines 3283–3361), the 1,093-record Aug-13 bf-1ea4g worker-log extract (gzipped), the attempt-30 session transcript copy, `system-state-2026-08-13T082344Z.md`, `MANIFEST.sha256`. Grounded in the raw log; rejects the "post-completion" framing; abstains on mechanism |

### 6.2 The 2026-09-02 same-day corpus (mutually contradictory; superseded in part by the banner + re-determination)

| Document | What it concluded | Status |
|---|---|---|
| `docs/crash-analysis-bf-1ea4g-signal-minus-one-2026-09-02.md` ("Analysis A") | FALSE_POSITIVE alert; root cause repository bloat 18 GB → OOM SIGKILL; task done 8 min pre-crash; cleanup "18 GB → 755 MB (2026-08-17)" | **Mechanism direction right, details wrong:** infers SIGKILL-9 from the sentinel; "post-completion" framing refuted for attempt 30; 07:42:34Z instant unverified; its own snapshot stamp (08:33:03Z) postdates its own crash time (07:42:34Z) |
| `docs/investigations/bf-1ea4g-crash-verification-2026-09-02.md` (domchk-862865a2) | Same as Analysis A (18 GB / 17.16 GB loose / 4,482 objects / 246 MB blobs; decision tree) | Same corrections; unique value = the repo-forensics breakdown and the bead-close timestamp |
| `docs/archive/crash-investigations/root-cause-analysis-bf-1ea4g-final.md` (domchk-1f6f5bdc) ("Analysis B") | FALSE_POSITIVE; **SIGHUP cascade**; "repository was healthy, NOT 18GB" | **Both premises superseded** by the banner: sentinel has no signal number; Aug-13 was 3 weeks into the bloat era (health was measured post-repair, anachronistically) |
| `docs/archive/crash-investigations/crash-analysis-signal-minus-one-bf-1ea4g-2026-09-02.md` (domchk-ac43ba28) | SIGHUP cascade driven by a **2026-08-16** systemd-oomd event (94.71 % pressure) | Wrong day entirely — links an Aug-13 kill to an Aug-16 window |
| `docs/archive/crash-investigations/investigation-completeness-report-bf-1ea4g-2026-09-02.md` (domchk-143387a1) | Declares the earlier bf-6903b investigation (bloat OOM) "INCORRECT", installs SIGHUP | The correction itself was corrected — the 2026-09-07 re-determination restores the bloat-era mechanism on transcript evidence |
| `docs/archive/crash-investigations/fix-proposal-bf-1ea4g-crash-pattern-2026-09-02.md` (domchk-a98e6091) | 8-step bloat mechanism (17+ identical 237 MB `.beads/` commits → 18 GB → OOM) + 9 implemented preventive measures | Mechanism family right (era-level), instant-level details superseded; its prevention layer is real and listed in §5 |
| `docs/archive/crash-investigations/final-resolution-bf-1ea4g-2026-09-02.md` (domchk-39fa19e0) | FALSE_POSITIVE / INFRASTRUCTURE; crash "09:00:18Z", 46 min after an "08:14:18Z" completion | Its instant and gap are unique to this doc and unverified; wrong for attempt 30 (which died mid-task at 08:23:44Z) |

All six archive docs were moved there 2026-09-06 (domchk-87a7bb2a; 387 files) and are **frozen as written** — none carries its own banner; the archive README states claims are "frozen as written and often wrong." `docs/archive/crash-investigations/local-main-state-bf-1ea4g.md` is the task's own deliverable-shaped snapshot, but its stated snapshot time (05:26:41Z, SHA `b2a71f7a`) **precedes the bead's creation** (07:14:47Z), so it cannot be this bead's output — one more instance of the corpus's snapshot-record confusion (§7).

### 6.3 Corpus-wide reports carrying bf-1ea4g rows (incidental mentions)

| Document | bf-1ea4g content |
|---|---|
| `docs/investigations/final-investigation-report-2026-09-01.md` (domchk-20dc36b4, commit 383241f) | **Origin of the 07:42:34Z instant and the "9+ duplicates" figure** every later doc repeats. Carries a 2026-09-06 Correction banner: its systemd-oomd → SIGHUP mechanism is superseded; conclusions (no code defects, alerting gaps) stand |
| `docs/investigations/findings-compilation-2026-09-05-domchk-65afcc88.md` | Grades Aug-12/13/14 events **[COMMIT]-attested only** — logs rotated, "treat the committed record as final." The 2026-09-07 transcript re-derivations supersede its "cannot be re-verified" for this bead specifically |
| `docs/investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md` | Carries the corpus §7 superseded-claims table retiring signal-number readings; grades its bf-1ea4g row MEDIUM (attested-only) — the 2026-09-07 first-hand sections in the canonical doc raise that |
| `docs/investigations/evidence-compilation-2026-09-01-crash-investigation.md` (domchk-59e1f1d5) | Provenance-tagged ([LIVE]/[COMMIT]/[REPORTED]/[GONE]) restatement of the same row |
| `docs/investigations/bf-4k2ws-root-cause-determination-domchk-7f838f36-2026-09-07.md` | Notes bf-1ea4g's snapshot commits survive only on the local-only `pre-squash-history-20260816` ref |
| `docs/investigations/root-cause-analysis-domchk-c62ce9ca-bf2vtzg-2026-09-02.md` | Uses bf-1ea4g as its Pattern-2 exemplar; names six duplicate-alert bead IDs (bf-3ulz5, bf-1nb5u, bf-1x9j5, bf-2rd24, bf-55j5g, bf-1ztab) |
| `docs/investigations/crash-alert-generation-logic-2026-09-02.md` (domchk-7da0d571) | Counts 14 bf-1ea4g false positives; documents NEEDLE's `auto_bead_on_error` gap that the alert-system fixes later patched |
| `docs/comprehensive-crash-investigation-report-2026-09-01.md` | bf-1ea4g is the worked example for its duplicate-alert pattern (9+ verifications) |
| `docs/crash-alert-fix-strategy-2026-09-01.md`, `docs/crash-pattern-analysis-bf-4k2ws-2026-09-01.md`, `docs/root-cause-hypotheses-ranked-2026-09-02.md` | Cite bf-1ea4g as the sole/leading evidence for NEEDLE deficiency #3 (no alert dedup) |
| `docs/crash-prevention-requirements.md` (domchk-d7c086d6) | Row: "Duplicate / false-positive alerts — bf-1ea4g (18+)" — the canonical live-verified gap list |
| `docs/alert-deduplication-gap-analysis-2026-09-07.md` (domchk-b5448b6a) | Live census: bf-1ea4g ranks 5th by alert concentration — 58 alert beads against a closed target |
| `docs/branch-divergence-analysis.md` | Backlog numbers (660 → 422 → 0) used by the re-determination; bf-1ea4g commit subjects in the diverged lineage |
| `docs/cascade-timeline-bf-3561g-2026-08-16.md`, `docs/crash-artifacts-bf-3561g.md` | 6 ALERT beads for bf-1ea4g crashed in the Aug-16 cascade; bf-3561g §4/§6 supplies the kernel-proven `MemoryMax`/`CONSTRAINT_MEMCG` mechanism |
| `docs/crash-investigation-bf-5tgsk-2026-08-16.md` | bf-5tgsk *was* an early bf-1ea4g alert investigation (itself crashed, exit −1) |
| `docs/crash-investigation-bf-4yjq-summary-2026-08-26.md`, `docs/crash-investigation-report-bf-4yjq-final.md`, `docs/remediation-strategy-bf-4yjq.md`, `docs/crash-context-report-bf-4yjq-comprehensive.md` | List bf-1ea4g among related signal-−1 beads (one gives "1 crash", another "2026-08-13 08:13" — see §7) |
| `docs/crash-documentation-index.md` | Indexed bf-1ea4g **nowhere** until this summary was added to its Key Individual Reports list |

### 6.4 The dispatch-named directory's own bf-1ea4g doc (registered 2026-09-07)

The §6 census reviewed `docs/*.md`, `docs/investigations/`, the archive and the evidence bundle — it did not sweep `docs/crash-investigations/`, which holds exactly one bf-1ea4g-named file: `docs/crash-investigations/bf-1ea4g-crash-investigation.md` (2026-08-17). Written before the bundle and the re-determination, it carries four superseded readings — single 07:42:34Z instant (§7 row 1), exit −1 = SIGKILL (row 3), era-level bloat OOM as the whole mechanism (row 5), and task-completed-before-crash (row 7) — alongside what stands: the agent attribution (`claude-code-glm-4.7-lab-domain-check`, re-confirmed from the raw log) and the era-level mechanism family. The file now carries a dated correction banner pointing at the canonical record; its body is untouched.

---

## 7. Claim-conflict matrix — what disagrees, and what won

| Claim | Values circulating in the corpus | Verified record |
|---|---|---|
| Crash instant | **07:42:34Z** (2026-09-01/02 corpus, originates in 383241f) · **08:13** (bf-4yjq docs) · **09:00:18Z** (final-resolution) · **08:23:51.806Z** (alert payload) | There were **56 kills**, 07:17:49Z → 09:08:30Z. The alert-named instant resolves to kill **08:23:44.918Z**; 08:23:51.806Z is its heartbeat. The three other figures are single-crash artifacts of docs that never read the raw log |
| Number of crashes | "1 crash" (remediation-strategy) vs "6 ALERT beads crashed" (cascade docs) vs single-crash framing throughout | **56 kills across 57 attempts** — the ALERT-shaped bead count (56) matches one-per-kill exactly |
| Exit −1 means | SIGHUP (Signal 1) · SIGKILL (Signal 9) · "either" | **Needle's died-without-exit-code sentinel — no signal number** (2026-09-07 fleet reclassification) |
| Classification | FALSE_POSITIVE · INFRASTRUCTURE | **Both, at different layers:** kill = INFRASTRUCTURE; alert = FALSE_POSITIVE (self-recovery) |
| Root cause | SIGHUP cascade on healthy repo · bloat OOM from host RAM · Aug-16 systemd-oomd · `.beads/`-snapshot bloat | **Unbounded `git push` pack-objects over the 422-commit backlog (retired bead-forge mass) inside the 12 GiB dispatch scope** — bloat-era OOM family, but scoped to the memcg, mid-task, mid-push |
| Repo health at crash time | "healthy, NOT 18GB" · "18 GB / 17.16 GB loose" | **No Aug-13 measurement survives** (squash rewrote the history). Bracketed: 18 GB Aug-12 → `.beads/` still tracked + 422-commit backlog Aug-13 → 5.6 GB mass Aug-16. "Healthy" was measured post-repair and is anachronistic |
| Work state when killed | "completed 8 min before crash (07:34:20Z)" · "46 min before (08:14:18Z)" | **Mid-task, mid-push** (attempt 30: deliverable written 08:23:13Z, push issued 08:23:31Z, killed 13.8 s in; bead still open 47 min more). The bead nonetheless self-recovered — which is why the alert-level verdict survives the premise's correction |
| Snapshot / commit SHAs | `b2a71f7a` · `017980ec` · `6f0c76fc` · `e19739af` | **Deliverable on origin/main: blob `e77648c7`** (live-verified). Attempt 30 committed `6da701b` documenting tip `33356c14`. The four circulating SHAs belong to different attempts/docs and are unresolvable in this clone (squash) |
| Output path | `/tmp/local-main-state-bf-1ea4g.json` · `.beads/local-main-state-bf-1ea4g.json` · repo-root file | Repo-root `main_branch_state_bf-1ea4g.json` is what shipped; attempt 57 wrote to `/tmp` and skipped the push — that is why it survived |
| Cleanup date | 2026-08-16 · 2026-08-17 · "18 GB → 755 MB" · "18 GB → 138 MB" | Squash 2026-08-16; de-bloat completed **2026-09-01**, re-verified 2026-09-06 and today (105 MB). The intermediate MB figures are point-in-time readings, not the endpoint |
| Duplicate-alert count | 9+ (2026-09-01) → 14 → 17 (2026-09-02) → 18+ (2026-09-06) → 58 alert-shaped (gap analysis census) → 88 title-mentions | All are point-in-time censuses of a growing pool (one alert per kill until dedup landed). Today: **88 title-mentions / 56 ALERT-shaped / 12 open** |
| Attempt count | "single crash" · "64 attempts" (child bead 1) | **57 attempts** (bundle `attempt-index.tsv`, independently corroborated by all 57 surviving transcripts) |

---

## 8. Key learnings

1. **`exit_code: -1` is a sentinel, not a signal.** Any doc that reads SIGHUP or SIGKILL-9 out of it is over-reading. The fleet reclassification retired both readings.
2. **Operation-correlated deaths are the mechanism discriminator.** 54/56 kills landing on one git operation, scattered across 1 h 50 min, rules out time-correlated (SIGHUP cascade, fleet event) and points at the operation's own resource profile. You only see this by reading *where in each attempt the kill landed* — last-tool-call transcript analysis.
3. **Alert timestamps are heartbeats, not kill instants.** Resolve any alert-carried instant through the worker log to the `agent.completed` record before building a timeline on it (here: 6.9 s later, and the corpus built three wrong timelines on unresolved stamps).
4. **A false-positive *alert* and a real *kill* coexist.** Classify the two layers separately: the kill was real, mid-task, infrastructure-caused; the alert was false-positive-shaped because the bead self-recovered. Corpus docs that argued "FP vs INFRASTRUCTURE" as if mutually exclusive were arguing past each other.
5. **A retry loop that commits before pushing is self-amplifying.** Each killed attempt grew the next attempt's push. Bound the retry (and the transport) or the loop feeds itself.
6. **Recovery by behavior change is not remediation.** Attempt 57 succeeded by skipping the push while the backlog was still ~500 commits; the condition drained only with later bounded pushes. A green outcome does not certify a fixed cause.
7. **Premises can be anachronistic.** "Repository healthy" was measured weeks after the crash era against a post-repair repo. Check which day a health claim was actually measured on.
8. **Provenance grading matters:** everything about Aug-12/13/14 was [COMMIT]-attested-only until the surviving transcripts were read first-hand in 2026-09-07 — attested-only is a reason to re-derive, not a reason to stop.
9. **Near-identical artifact titles across beads are the main false-positive source** in this corpus (multiple investigations of *this* crash matched titles word-for-word while belonging to other beads); check a report's Related Bead field before citing it.
10. **Corpus conventions that kept this sane:** the dedup-append convention (one canonical doc per bead; append dated sections, never rewrite), and the archive freeze policy (frozen docs get dated corrections elsewhere, never edits). The four circulating snapshot SHAs and three crash instants in §7 are what happens without them.

---

## 9. References

- Canonical record: `docs/investigations/bf-1ea4g-root-cause-determination-2026-09-02.md` (appends: domchk-c2b8c832 re-determination, commit 75ed2d8; domchk-508e54c0 resource analysis, commit 488823d)
- Evidence bundle: `docs/crashes/bf-1ea4g/` (commit 2ce9cd9) — README, attempt-index.tsv, attempt-30 bracket + transcript, full-bead log extract, system-state doc, sha256 manifest
- Same-day 2026-09-02 corpus: `docs/crash-analysis-bf-1ea4g-signal-minus-one-2026-09-02.md`, `docs/investigations/bf-1ea4g-crash-verification-2026-09-02.md`, and the six archived docs under `docs/archive/crash-investigations/` named in §6.2
- Corpus-wide: `docs/investigations/final-investigation-report-2026-09-01.md` (+ Correction banner), `docs/investigations/findings-compilation-2026-09-05-domchk-65afcc88.md`, `docs/investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md`, `docs/investigations/evidence-compilation-2026-09-01-crash-investigation.md`, `docs/investigations/bf-4k2ws-root-cause-determination-domchk-7f838f36-2026-09-07.md`
- Mechanism corroboration: `docs/crash-artifacts-bf-3561g.md` §4/§6 (kernel-proven `MemoryMax` kill path), `docs/crashes/bf-198ne-crash-report.md` (push-side variant, 5.6 GB), `docs/branch-divergence-analysis.md` (backlog 660 → 422 → 0)
- Alert layer: `docs/investigations/crash-alert-generation-logic-2026-09-02.md`, `docs/crash-alert-fix-implementation-2026-09-02.md`, `docs/alert-deduplication-gap-analysis-2026-09-07.md`, `docs/crash-prevention-requirements.md`
- Standing procedure: `CLAUDE.md` ("Crash Prevention and Investigation"), `docs/crash-response-guide.md`, `docs/crash-documentation-index.md`
- Archive policy: `docs/archive/crash-investigations/README.md` (frozen as written, often wrong)

---

*Compiled by domchk-6a5f4207, 2026-09-07. Every [LIVE] figure in §1–§5 was re-verified this session (`bead show bf-1ea4g`, `bead list --json` census, `git ls-tree origin/main`, `git count-objects -vH`, `du -sh .git`, `./scripts/setup-git-gc-config.sh --verify`); historical figures are cited to the documents and commits that established them rather than restated as first-hand.*
