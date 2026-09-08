# Crash Investigation: Agent Signal -1 on Bead bf-4x12ec

## Summary
Bead bf-4x12ec experienced an agent crash with exit code -1 (signal -1) on 2026-08-14, as part of a 64-minute retry storm: 44 attempts were SIGKILLed between 10:23:02Z and 11:27:26Z, each surviving only 39–116 seconds, before the bead was decomposed on the 53rd attempt at 12:58:45Z and the `git gc --aggressive --prune=now` completed under child bead bf-173o7e — attempt 53 executed needle's auto-split (`SPLIT_COMPLETE`), not the gc (Addendum 4 §3; corrected 2026-09-08, bead domchk-8c78ae8b, from "completed on the 53rd attempt"). Per established crash investigation protocol, this investigation determines the crash context, verifies work completion status, and documents findings.

## Crashed Bead Details
- **Bead ID:** bf-4x12ec
- **Title:** Execute aggressive git garbage collection to eliminate OOM risk
- **Type / Priority:** task / P2
- **Status:** Closed (work completed by retry agents)
- **Purpose:** Phase 1.2 emergency stabilization — pack 17.20GB of loose git objects into compressed pack files, eliminating OOM risk during git operations
- **Crash Window:** 2026-08-14 10:23:02–11:27:26Z — 44 attempts killed at 39–116 s each (primary event log; see Addendum 2); the bead had been created at 10:17:26Z
- **Signal:** -1 (environment-level process kill, SIGKILL; source most consistent with the OOM killer — see Signal Analysis and Addendum 2)

> Note: the bead record was not retrievable when this report was first written
> (the crash predates the bead-forge → bead-rs migration). It has since been
> recovered from the migrated workspace; the title and purpose above come from
> the live record and replace the earlier "Unknown / likely migration work"
> placeholders.

## Crash Context and Timeline

### Repository State at Crash Time
Based on analysis of the crash period (2026-08-12 to 2026-08-14):
- **Repository Size:** 18GB (severely bloated)
- **Loose Objects:** 17.16GB (95.7% of total size)
- **Root Cause:** Repository bloat from repeated large file commits
- **Git Operations:** Memory-intensive operations triggering OOM killer

### Crash Timing Analysis
The crash occurred at 10:25:30 UTC (6:25 AM EDT) during a period of intense git activity:
- **Git History:** Multiple commits around the crash time (09:08-09:25 EDT)
- **Operations:** Git garbage collection, cleanup, and predispatch updates
- **Migration Context:** During bead-forge to bead-rs workspace migration

### System State During Crash Period
From parallel crash investigations (bf-4yjq, bf-2ildm, etc.):
- **Memory Status:** OOM killer active, <2GB available during git operations
- **Load Average:** 15-17 (exceeding 12 CPU cores)
- **Disk Usage:** 84% full (350GB/444GB used)
- **Crash Pattern:** 9 systematic crashes in 2.5 hours on bf-4yjq alone

**Correction (2026-09-08, bead domchk-8c78ae8b):** the four figures above are
secondhand readings from parallel investigations, with no Aug-14 primary source
behind any of them. Addendum 2 §Evidence-window limitation records that *no*
kernel log or memory telemetry survives for 2026-08-14 (the current boot began
2026-08-15 19:26 EDT; `.beads/logs/resource-metrics.log` begins 2026-09-01), so
none of these contemporaneous numbers is verifiable. Addendum 3 §"the death
loop ran under heavy load saturation" supplies the sourced version of the load
figure from direct `fleet.cpu_saturated` telemetry: **load 10.4–30.92 on 9
reported cores, mean ~13.8**. The "<2GB available ⇒ OOM killer active" premise
is expressly corrected by Addendum 2 §Corrections item 4 — a **cgroup**
(`CONSTRAINT_MEMCG`) kill inside a capped scope does not require low *system*
free memory; ample free RAM and a memcg kill coexist. And the bf-4yjq line
cites exactly the count the corrected record supersedes:
`docs/crash-analysis-bf-1s6c3-2026-09-06.md` gives bf-4yjq **50 kills**
(17:54–20:30Z) on the evening of 2026-08-12, and CLAUDE.md's crash section
states the 2026-09-06 record "supersedes the 2026-09-01 corpus's '9 crashes in
2.5 hours' count." The bullets are retained as the v1.0-era historical record.

## Signal Analysis

**Signal -1 Identification** *(heading softened 2026-09-08, bead
domchk-8c78ae8b — was "Definitive Identification"; see the correction below)*:
- Signal -1 = **SIGKILL (Signal 9)** in Linux
- **Delivered by:** Linux OOM (Out Of Memory) killer
- **Process termination:** Immediate, no graceful shutdown
- **Core dump:** None generated (SIGKILL prevents core dumps)
- **Indication:** Memory exhaustion, not application error

**Correction (2026-09-08, bead domchk-8c78ae8b):** the claim carried by the
original "Definitive Identification" heading is stronger than the evidence.
`exit_code = -1` is needle's sentinel for a child agent that terminated
without a wait status — killed by a signal — and **is not itself a POSIX
signal** (Addendum 2 §"Signal -1, precisely"). SIGKILL(9)-via-OOM is the
*canonical inference* from instant death, zero application error logs and no
core dumps, not an identification; direct Aug-14 kernel logs do not survive
(Addendum 2 §Evidence-window limitation). The `## Crashed Bead Details`
Signal line already reads this way ("source most consistent with the OOM
killer — see Signal Analysis and Addendum 2"). The mechanism is corroborated,
not directly observed, by 257 same-window git memcg kills on Aug-16 (Addendum
6; refined by Addendum 3). The bullets are retained as the v1.0-era record.

## Original Work Context

The recovered bead record confirms bf-4x12ec's task was **repository cleanup**, not the workspace migration itself. Its description reads: *"Execute aggressive git garbage collection to pack 17.20GB of loose objects into compressed pack files, eliminating the OOM risk during git operations. This is Phase 1.2 from the root cause analysis (CRITICAL - NOT YET EXECUTED)."*

The bead workspace migration (bead-forge → bead-rs) was a separate effort, completed on 2026-08-15 (commit `61d27ac`). The initial draft of this report conflated the two because the bead record was not yet retrievable.

## Deliverable Verification

**Status: ✅ CLEANUP COMPLETED SUCCESSFULLY BY RETRY AGENTS**

### Resolution Steps (from the bead's recorded outcome)
1. **Removed the bloat source:** `.beads/checkpoint/` files excluded from git tracking via `.gitignore`
2. **Executed aggressive garbage collection:** `git gc --aggressive --prune=now` — the operation that killed the original agent run, completed under child bead **bf-173o7e** after the attempt-53 auto-split, not on any bf-4x12ec retry (no bf-4x12ec attempt ever completed it: 44 × exit -1, 8 × exit 124; Addendum 4 §3 — corrected 2026-09-08, bead domchk-8c78ae8b, from "completed on retry")
3. **Additional repack optimization:** `git repack -a -d --depth=250 --window=250`
4. **Verified integrity:** `git fsck --no-full` completes without timeout (dangling objects only)
5. **Verified git operations:** clone, fetch, and checkout all complete without OOM

### Final Verified Metrics (from bead bf-4x12ec's completion notes)
| Metric | Before | After | Target | Result |
|--------|--------|-------|--------|--------|
| Repository size (`.git`) | ~18GB (17.20GB loose) | **753MB** | <500MB | ⚠️ close |
| Loose objects | 4,627 | **141** | <100 | ⚠️ close |
| Pack objects | scattered loose | 10,265 in 750.67 MiB pack | — | ✅ |
| Git operations without OOM | failing | passing | required | ✅ |

The bead recorded both size targets as **PARTIAL** (753MB vs <500MB; 141 vs
<100) but accepted: the OOM risk was eliminated and all git operations returned
to normal. Loose objects were subsequently driven below 100 — see Addendum.

**Metric provenance (re-verified live 2026-09-07, bead domchk-791bfb2e):** the
"after" column above is confirmed verbatim against the live bead record —
bf-4x12ec FINAL METRICS read "`.git` size: 753MB (was ~18GB)", "Loose objects:
141 (was 4,627)", "Pack objects: 10,265 in 750.67 MiB pack". The "before"
loose count appears as three different numbers across primary sources, all
genuine and taken at different instants: **4,515** in the contemporaneous
`docs/cleanup-resolution-2026-08-17.md` (which also records a post-cleanup
118 loose / 9,525 packed in a 750.53 MB pack — a slightly later reading), and
**4,627** in bf-4x12ec's own description and completion notes. The table uses
4,627 because the bead's notes are the source it cites. A fourth figure,
**4,649**, is the crash-time live `git count-objects -vH` the killed agents
themselves ran (`count: 4649, size: 17.20 GiB, in-pack: 4081, packs: 1,
size-pack: 9.60 MiB` — surviving per-attempt transcripts,
`docs/crash-investigations/evidence/bf-4x12ec/crash-logs/`); see Addendum 4.
Crash-time state and post-cleanup verified state are kept separate throughout
(the table's Before/After columns; the 2026-08-17 body sections; the dated
re-verification snapshots in the addenda). Re-checked live 2026-09-07:
`.git` 107MB, 467 loose objects / 3.37 MiB, 11,700 in-pack, 2 packs,
size-pack 99.78 MiB, 0 garbage — normal churn since the 2026-09-02 snapshot
below, no bloat signature.

### Migration Context (separate, related effort)
- **Commit:** `61d27ac` (2026-08-15 09:56:53)
- **Action:** Complete bead workspace rehydration from bead-forge to bead-rs
- **Results:** 184 issues and 157 dependency edges recreated; workspace stable

### Success Evidence
1. ✅ **Repository size normalized:** 18GB → 753MB (≈96% reduction)
2. ✅ **Loose objects packed:** 4,627 → 141; system stable and optimized
3. ✅ **Migration completed:** Bead workspace fully transitioned to bead-rs
4. ✅ **Git operations stable:** Normal performance on all operations

## Root Cause Analysis

### Crash Mechanism
**Sequence of Events:**
1. Git operations on 17GB of loose objects loaded into memory
2. `git pack-objects` process consumed 3-6GB RAM per operation
3. Multiple concurrent git operations exhausted available memory
4. Linux OOM killer invoked SIGKILL (signal 9)
5. Process terminated immediately with exit code -1
6. Bead marked as crashed and released for retry

**Correction (2026-09-08, bead domchk-8c78ae8b):** steps 2 and 3 above are
superseded by Addendum 3's kernel evidence and are retained only as the
v1.0-era mechanism sketch. There is **no concurrency evidence** — each
phase-1 attempt ran exactly one `git gc --aggressive --prune=now`, and every
surviving transcript ends with that single tool call unanswered, killed
mid-gc (Addendum 4 §1; Addendum 3 §Root-cause determination). The exhaustion
was **cgroup** memory exhaustion, not system OOM: the dispatch scope runs
with `MemoryMax=12GiB`, which a `git gc --aggressive` over ~17 GB of loose
objects cannot fit under — git anon-rss at kill measured **1.2–11.97 GB, mean
10.14 GB**, with 163 of the 257 Aug-16 kernel-recorded git kills hugging the
11–12 GB ceiling (Addendum 3 §"257 git OOM-kills"; count corrected from
Addendum 2's "13 events" by Addendum 6). Step 4 names the right killer but
the constraint was the memcg cap, not free-system-memory exhaustion.

### Why the Crash Occurred
The crash occurred **not because of a bead implementation defect**, but because:
- Any significant git operation on the bloated repository triggered OOM
- The workspace had 17GB of loose git objects from previous problematic commits
- Memory-intensive git operations exceeded available memory
- The OOM killer terminated processes regardless of their specific task

## Crash Classification

**Type:** Infrastructure/Environmental Failure
**Cause:** Repository bloat triggering OOM killer
**Impact:** Workspace-wide git operation disruption
**Code Defect:** NONE - Bead implementation was correct
**Domain-Check Code Defect:** NONE - see the explicit finding below
**Reproducibility:** HIGH at the time (environmental trigger)
**Duration:** Part of systematic crash series during migration period

**No-Code-Defect Finding (made explicit 2026-09-07, bead domchk-e48b5e1b):**
no domain-check application code defect was found or implicated. The crash
happened in the agent dispatch infrastructure while the dispatched agent ran
`git gc --aggressive --prune=now`; no domain-check code was executing, and
domain-check appears in this crash only as the *name of the needle worker*
(`claude-code-glm-4.7-lab-domain-check`). The cause is environmental only —
memcg OOM inside the agent's transient `run-p*.scope` (`MemoryMax=12GiB`)
over a 17–18 GB loose-object repository — so **no code change to domain-check
is required, recommended, or implied by this investigation**, consistent with
workspace guidance (CLAUDE.md, "Crash Prevention and Investigation": domain-check
code has been thoroughly investigated and found to have NO defects).

## Current Status (August 17, 2026)

### Repository Health Status
✅ **HEALTHY** - All metrics normalized
- Repository size: 753MB (normal; down from ~18GB)
- Loose objects: 141 (down from 4,627; packed efficiently)
- Git operations: Stable and performant
- OOM risk: Eliminated at cleanup

### Migration Status
✅ **COMPLETE** - Bead workspace successfully migrated
- bead-forge to bead-rs transition completed
- All issues and dependencies preserved
- Workspace fully operational

### Crash Investigation Status
✅ **COMPLETE** - All acceptance criteria met:
- [x] Full crash context retrieved
- [x] Crash circumstances documented
- [x] Signal analysis completed
- [x] Root cause identified (environmental OOM)
- [x] Work completion verified (cleanup successful)

## Conclusion

No recovery action needed. Bead bf-4x12ec crashed due to environmental factors (repository bloat triggering OOM killer) during a period of systematic infrastructure issues. The crash was **incidental to the actual work being performed**—the aggressive git garbage collection that bf-4x12ec was created to run was successfully completed by retry agents, and the related bead workspace migration also completed on its own schedule (2026-08-15).

**The crash represents a workspace-wide infrastructure issue that has been fully resolved through repository cleanup and migration completion.**

**Attribution of the cause:** environmental only. No domain-check application code
defect was found or implicated (see the No-Code-Defect Finding under Crash
Classification) — nothing in this report calls for a fix to domain-check code;
the remediations it names are all infrastructure and process measures
(repository cleanup, `.gitignore` protection, monitoring timers,
`scripts/safe-git-gc.sh`, a bounded per-dispatch memory limit).

### Pattern Memory

This investigation follows the established protocol from needle crash analysis patterns: crash-alert beads verify (don't redo) work that retry agents have already completed. The signal -1 is consistently an environment-level kill from the OOM killer, not a code execution failure. Carried forward to every addendum below: no addendum changed this attribution — each strengthens the environmental determination (memcg OOM in the agent's dispatch scope) and none implicates domain-check application code, so no code fix is required anywhere in this record.

**Prevention Strategy:**
The implemented safeguards (repository cleanup, .gitignore protection, health monitoring) provide a robust defense against future repository bloat and OOM crashes.

## Addendum — Report Review (2026-09-02)

The repository has been re-verified at report-review time and the cleanup has
held and improved. Current state: **92MB** total `.git` size, **20 loose
objects** (10,408 in-pack), zero garbage — further reductions delivered by the
scheduled safe-git-gc maintenance timers. Repository metrics in the body of
this report are preserved as of their original 2026-08-17 investigation date.

### Corrections made in this revision
- **Repository size:** 757MB → **753MB**, matching the bead's recorded final
  metrics and the contemporaneous cleanup records
  (`docs/cleanup-resolution-2026-08-17.md`)
- **Loose objects:** added the missing count — **141** (down from 4,627),
  plus 10,265 pack objects in a 750.67 MiB pack
- **Resolution steps:** added as an explicit section; previously implicit and
  conflated with the separate migration commit `61d27ac`
- **Bead record:** title and purpose recovered from the live bead (was
  "Unknown" in v1.0) — confirming the task was repository cleanup, not the
  migration
- **Removed** the unverifiable "zero crashes since cleanup" claim

---

**Investigation Complete:** All work verified as completed with high-quality implementation.
**Confidence Level:** HIGH — Clear evidence from repository state and git history.
**System Status:** ✅ HEALTHY — All safeguards operational and effective.

**Investigation Date:** August 17, 2026
**Last Reviewed:** September 8, 2026 (Addendum 9, workload contribution + reproducibility assessment, bead domchk-78d89c6b, appended on top of origin/main `a00d02bb`; prior this date: body harmonized with the addenda — Summary/Resolution-step "53rd attempt" phrasing, RCA mechanism, Signal framing, System-State sources — per the section inventory, bead domchk-8c78ae8b; concurrent: Addendum 8, crash-window resource timeline + safe-operating-limits verdict, bead domchk-5f3ec6e1, appended at e0b70dab while these v1.10 edits were still uncommitted; prior: Addendum 7, parent acceptance-criteria mapping + live completion verification, bead domchk-fe10456e; no-code-defect finding made explicit for domain-check, bead domchk-e48b5e1b, 2026-09-07; metric provenance re-check, bead domchk-791bfb2e)
**Report Version:** 1.11 (Addenda 2–9 below; Addendum 4 re-verified by second dispatch; v1.7 corrects Addendum 4's attribution of the 753 MB final metrics from bf-173o7e to the parent bead bf-4x12ec; v1.8 adds the explicit "no domain-check code defect / environmental-only" finding under Crash Classification and in the Conclusion — a clarification, no prior claim was refuted; v1.9 appends Addendum 7, the consolidation record mapping parent domchk-46a00141's three acceptance criteria to their delivering beads/commits/artifacts and re-verifying bf-4x12ec's completion status live; v1.10 applies the section inventory's gap checklist (bead domchk-f6aba211) in place — dated Correction blocks under Summary, System State, Signal Analysis and Root Cause Analysis harmonizing the body's v1.0-era wording with the addenda's primary-source corrections, with no historical text removed; v1.11 appends Addendum 9, the workload-contribution and reproducibility assessment of split child 3 of umbrella domchk-4adc1a55 — deliverable command in progress at all 44 kills, workload contribution = necessary trigger via condition interaction, deterministic-then / non-reproducible-now, live-verified 2026-09-08)

## Addendum 2 — Primary-Source Retry-Storm Analysis (2026-09-02, bead domchk-661c2dc6)

A further investigation (alert bead `domchk-661c2dc6`, alert timestamp
`2026-08-14T10:39:42.223630206Z`) recovered the surviving needle event log for
the crash day and reconstructed the event from primary evidence:
`/home/coding/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl`
(worker `claude-code-glm-4.7-lab-domain-check`, session `a6dbb1fc`). This
addendum corrects several claims in the body of this report and in the
2026-08-25/26 summaries that primary evidence does not support.

### The real timeline: one retry storm, not one crash

bf-4x12ec has **53 recorded agent attempts, all on 2026-08-14** (zero events on
Aug 15/16). The day divides into three distinct phases:

| Phase | Window (UTC) | Attempts | Outcome | Duration each |
|-------|--------------|----------|---------|---------------|
| 1. Crash loop | 10:23:02 – 11:27:26 | 44 | `exit_code=-1`, classified `crash` | 38.9 – 115.8 s |
| 2. Timeout loop | 11:38:07 – 12:50:14 | 8 | `exit_code=124`, classified `timeout` | exactly 600.0 s |
| 3. Success | 12:58:45 | 1 | `exit_code=0`, classified `success` | 491.8 s |

The bead was formally closed 2026-08-17T14:50:41Z; per the event log the
**work actually completed 2026-08-14T12:58:45Z** (`outcome.classified:
success`, `outcome.handled: action=none`). The Aug-17 timestamp is the closure
and notes-writing time, not the completion time.

### Resolving the conflicting crash timestamps

Four different "crash timestamps" appear across the reports for this bead. All
four are events inside the phase-1 retry storm, not separate crashes:

| Timestamp | Where cited | What it actually is |
|-----------|-------------|---------------------|
| 10:25:30 | this report v1.0/v1.1 | Alert issued after attempt #2 died at 10:25:01 (`outcome.handled: alerted` at 10:25:35) |
| 10:39:42.223630206 | domchk-661c2dc6 alert | `HANDLING_RELEASE_DONE` heartbeat (10:39:42.223623Z) during teardown of attempt #12 |
| 10:41:13 | 2026-08-25 comprehensive summary | Alert record following attempt #13's death at 10:40:53 |
| 11:14:39 | signal-minus1 analysis | Alert following attempt #31's death at 11:14:21 |

The "crashed at 10:39:42" alert that triggered this investigation was
regenerated from this historical Aug-14 record — bf-4x12ec has been Closed
since Aug-17, so this is one more instance of the duplicate-alert pattern
documented in `docs/crash-context-bf-4x12ec-summary.md`.

### Corrections to prior root-cause narratives

1. **"The gc ran 57 minutes and was then SIGKILLed" — false.** No phase-1
   attempt survived longer than 115.8 s; the longest run of the day was the
   successful 491.8 s. The 57-minute figure matches nothing in the event log.
2. **"External timeout/capacity governor, NOT OOM" (2026-08-26 summary) —
   unsupported.** Phase-1 deaths at 39–116 s are far below the 600 s cap that
   phase-2 runs visibly survived to (exiting 124), so a harness timeout cannot
   explain phase 1. Conversely, a pure timeout cannot explain phase 3 — the
   same operation repeatedly hitting the cap, then completing in 491.8 s.
3. **The three-phase signature is the signature of easing memory pressure.**
   Attempts died fast while memory was tight; once pressure eased they survived
   (but ran long, hitting the cap); finally the operation fit under the cap.
   This is consistent with the OOM hypothesis and inconsistent with both
   single-cause alternatives above.
4. **"OOM impossible, 51GB was available" — non sequitur.** The system can have
   ample free RAM *and* still OOM-kill processes. Corroborating same-period
   evidence from the current boot's kernel ring buffer: on
   **2026-08-16 13:29:49 EDT** (inside the bf-4x12ec cleanup window, before the
   Aug-17 closure) the kernel killed `git` at **7.79 GB anon-rss** under
   `constraint=CONSTRAINT_MEMCG` — a **cgroup** memory limit in a transient
   `run-p*.scope`, not system exhaustion. All 13 `Killed process` events in the
   current boot (6 node/vitest, 6 bash, 1 git) are CONSTRAINT_MEMCG with
   `oom_score_adj=200`: this box's agent/child processes run inside
   memory-limited scopes and are marked preferred OOM victims. OOM under a
   cgroup limit remains the most consistent explanation for the 44 phase-1
   deaths; "free system memory" does not rule it out.

### Signal -1, precisely

`exit_code=-1` in the needle event stream is the worker's classification for a
child agent that terminated without a wait status — i.e. killed by a signal,
not a normal exit. It is not itself a POSIX signal. Given instant death, zero
application error logs, and no core dumps, SIGKILL (9) is the canonical cause,
delivered here most plausibly by the kernel OOM killer (above). The worker
classified each such death `outcome=crash`, alerted, released the bead, and
immediately re-claimed it — 44 times in 64 minutes.

### Isolation: not a fleet-wide event

Cross-referencing every one of the 44 phase-1 `agent.completed(exit=-1)`
events against all other beads in the same log: **no other bead completed
within ±3 s of any of them** (0/44). The storm was specific to bf-4x12ec's own
retry cycle. The nearby bf-173o7e crash at 12:59:48 is a separate bead, one
minute later, uncorrelated.

### Evidence-window limitation

Kernel logs for 2026-08-14 are unrecoverable: the current boot began
2026-08-15 19:26 EDT and the archived journals start 2026-08-15 19:31, so
`journalctl`/`dmesg` cannot cover the crash window. No historical memory
metrics exist either (`.beads/logs/resource-metrics.log` begins 2026-09-01).
The OOM determination above therefore rests on the retry-storm signature plus
same-period CONSTRAINT_MEMCG corroboration, not direct Aug-14 kernel logs —
this should be stated whenever this report is cited.

Minor correction: the crashing worker was `claude-code-glm-4.7-lab-domain-check`
(no `-2` suffix); the `-2` worker's log touches bf-4x12ec only on Aug 25 via a
commit-hook note.

Current system state (2026-09-02): 62Gi RAM / 50Gi available, swap 24Gi
unused, repository 92 MB with 20 loose objects — healthy, matching Addendum 1.

---
**Addendum 2 Investigation Date:** September 2, 2026
**Addendum 2 Sources:** needle event log 2026-08-14 (primary), live bead record, current-boot dmesg, journalctl coverage check
**Classification:** Technical Investigation - Infrastructure Failure

## Addendum 3 — Independent Verification & New Kernel Evidence (2026-09-02, bead domchk-0f9eb93a)

A further alert bead (`domchk-0f9eb93a`, crash timestamp cited as
`2026-08-14T11:01:40Z`) re-opened the same question. This addendum records an
independent re-verification of Addendum 2 against primary sources, plus
materially stronger kernel evidence that was missed the first time.

### The 11:01:40Z timestamp resolved

It is a fifth event inside the same phase-1 storm, not a separate crash. The
needle event log shows attempt **#26** died at `11:01:31.948Z`
(`agent.completed`, `exit_code: -1`, 76.1 s), the worker classified and
released the bead at `11:01:43.919` and re-claimed it at `11:01:46` — the
cited 11:01:40 falls inside that release/re-claim handling window.

### Verification results (all primary-source)

- **Retry storm reproduced exactly:** 53 attempts on Aug-14 — 44× `exit -1`
  (38.9–115.8 s) in 10:23–11:27, 8× `exit 124` (exactly 600.0 s) in
  11:38–12:50, 1× `exit 0` (491.8 s) at 12:58:45. Matches Addendum 2.
- **`journalctl -u needle` for the requested window returns "No entries".**
  The journal holds a single boot beginning 2026-08-15 19:26:03 EDT, so no
  kernel or system log covers Aug-14. Addendum 2's evidence-window limitation
  is confirmed.
- **Isolation extended to all workers:** a sweep of all six Aug-14 worker logs
  (`domain-check`, `drawrace`, `roam-1`, `roam-2`, `s1`, `test-fix`) finds 45
  signal-deaths in the 10:20–13:00 window: 44× bf-4x12ec plus 1× bf-173o7e
  (12:59:48, the separately-documented neighbouring crash). Zero deaths on any
  other worker — the storm never left bf-4x12ec's retry cycle.

### NEW: the death loop ran under heavy load saturation

`fleet.cpu_saturated` fired on essentially every dispatch of the death loop.
Readings inside 10:23–11:28: **load 10.4–30.92 on 9 reported cores, mean
~13.8** — peak 30.92 at 11:21:25. By 12:50:33 load was 7.84, and the
successful attempt at 12:58:45 ran at 9.86. The storm began as memory-limited
deaths while the box was also CPU-saturated, and both eased together into the
timeout loop and then success — the same "easing pressure" signature Addendum
2 describes, now with direct telemetry.

### NEW: 257 git OOM-kills on Aug-16 — the mechanism on full display

Addendum 2 cited a single kernel OOM event as corroboration. The current
boot's kernel journal actually contains **419 `Killed process` events: 413 on
Aug-16 (257 `git`, 156 `node (vitest)`), 6 `bash` on Sep-02**. All are
`CONSTRAINT_MEMCG` with `oom_score_adj=200` inside transient
`run-p*.scope` memcgs — correction: the "13 events" figure in Addendum 2 is
off by ~30×, and its cited single event (13:29:49 EDT, ~7.8 GB) corresponds to
the `13:29:51` git oom-kill (pid 2790353, anon-rss 7,788,052 KiB), one of 257,
not a lone occurrence.

The Aug-16 git kills are the same cleanup effort this bead was created for,
two days after the crash, with kernel logging available. Their shape:

- git anon-rss at kill: **1.2–11.97 GB, mean 10.14 GB**; 163 of 257 in the
  11–12 GB bucket — dying at a hard ceiling. (Refinement to Addendum 6's
  "11.7–12.6 GB" characterisation below: that range covers only the
  ceiling-hugging majority — 94 of the 257 kills fall below it, e.g. pid
  1947564 at 12:02:31 EDT, anon-rss 7,139,020 kB. Same killer, lower fill
  levels.)
- That ceiling is now identified from the live scopes: **agent dispatch scopes
  run with `MemoryMax=12GiB`** (12884901888 bytes); test scopes get 6 GiB and
  `CPUQuota=200%`. (The `needle-worker@*.service` units themselves are
  unlimited — Addendum 4 §5; the cap that kills the work bites inside the
  transient `run-p*.scope` each dispatch creates.) A `git gc --aggressive`
  over ~17 GB of loose objects cannot fit under a 12 GiB cap, so the kernel
  killed it — 257 times in one day — until the repo was finally repacked.
- `memory.oom.group=0` on these scopes, so the kernel kills a single task: the
  highest-badness one in the hitting memcg. On Aug-16 that was usually `git`
  itself; on Aug-14 it was the agent process (hence `exit -1` on the worker
  record). Same cause, different victim — whichever task held the most RSS
  when the scope hit its cap.

### Root-cause determination (unchanged conclusion, materially stronger evidence)

**Cgroup memory exhaustion, not system OOM, not timeout, not a code defect.**
`git gc --aggressive` on a 17–18 GB loose-object repository exceeds the 12 GiB
`MemoryMax` of the agent's transient scope; the kernel OOM killer SIGKILLs a
task inside that scope; the needle worker records `exit_code: -1`, classifies
`crash`, and re-claims the bead — 44 times in 64 minutes while the box was
also CPU-saturated. Phase-2's exactly-600 s deaths prove a harness timeout
existed and phase-1 died far earlier than it; phase-3's success at 12:58:45,
after load fell to ~8–10, closes the loop.

"Was git gc actually running at crash time?" is not directly observable from
the event log (no Aug-14 kernel logs; it stores only a `prompt_hash`), but it
is bounded: no attempt survived 115.8 s, so no long-running gc existed at
11:01:40 — the deaths were early-run, consistent with the agent launching the
gc and the scope hitting its cap within minutes, as the Aug-16 kills show it
doing repeatedly. Addendum 4's session transcripts have since supplied the
direct evidence this bound inferred: each phase-1 transcript ends with the
`git gc --aggressive --prune=now` tool call unanswered — killed mid-gc.

Repository state at review time (2026-09-02 ~10:53 EDT): `.git` 92 MB, 43
loose objects (360 KiB) + 10,408 packed in a single 90.18 MiB pack, zero
garbage — the cleanup held. (Loose-object counts drift between scheduled gc
runs; every reading this day, 20/35/43, is far below the 100 threshold.)

---
**Addendum 3 Investigation Date:** September 2, 2026
**Addendum 3 Sources:** needle event log 2026-08-14 (all six workers), current-boot kernel journal (`journalctl -k`), live cgroup inspection of needle/run-p scopes, `git count-objects`
**Classification:** Technical Investigation - Infrastructure Failure

## Addendum 4 — Session transcripts, gc config defect, and what the 53rd attempt actually did (2026-09-02, bead domchk-d986ce54)

Alert bead `domchk-d986ce54` (alert timestamp `2026-08-14T10:52:14.447218059Z`
= the `HANDLING_RELEASE_DONE` heartbeat during teardown of attempt #20 —
dispatched 10:51:03Z, exit -1 at 10:51:49Z after 46.2 s). Same duplicate-alert
class as Addenda 2–3. Net-new findings beyond them:

1. **Per-attempt session transcripts survive.** Each of the 53 attempts has a
   Claude Code transcript under `~/.claude/projects/-home-coding-domain-check/`,
   its mtime matching the corresponding `agent.completed` event to the second.
   Every phase-1 transcript shows the same shape — `git count-objects -vH` →
   `count: 4649, size: 17.20 GiB, size-pack: 9.60 MiB`; `du -sh .git` → `18G`;
   then `git gc --aggressive --prune=now` (tool timeout 600000) — and the
   transcript **ends with no tool result**. The agents were doing exactly what
   the bead asked and were killed mid-gc; the gc never completed within any
   phase-1 attempt. (Example: session `38af9cbf-addd-430f-8568-4837f7dcc0dd`,
   the attempt whose teardown produced this investigation's alert timestamp.)
2. **Attempt #1 also hit an invalid git config.** `.git/config` held
   `gc.aggressivewindow = 1.hour`; gc failed instantly with
   `fatal: bad numeric config value '1.hour' for 'gc.aggressivewindow' …
   invalid unit` (exit 128). The agent tried `1h` (also invalid), then unset
   the key at 10:22:18Z, so later attempts reached real gc execution.
   Follow-up commit `de7af48` (Aug 17) documented a numeric value, but its
   message ("value of 1 represents 1 hour") misreads the setting —
   `gc.aggressivewindow` is a delta-window object count, not a duration.
3. **The 53rd attempt did not run the gc — it executed needle's auto-split.**
   Its prompt opens "This bead has failed 8 times … Decompose This Bead", and
   its transcript (session `31800ee3-de7c-4619-abe8-07468fb7de32`) shows it
   creating three chained children (`bf create` → **bf-173o7e, bf-5jhvpk,
   bf-im2sl1**; `bf dep add` chaining them; umbrella label on bf-4x12ec), then
   printing `SPLIT_COMPLETE`. `verification.passed` fired at 12:58:45Z and
   `bead.orphaned` at 12:58:55Z. **The actual `git gc --aggressive
   --prune=now` ran under child bf-173o7e**, which closed 2026-08-17T17:12:09Z
   with reason "Git gc completed successfully - 17.20GB loose objects packed
   into 444MB pack file, repository valid" — the child's own recorded figure.
   The 753 MB / 141-loose final metrics were recorded on the **parent bead
   bf-4x12ec** (rev 4, updated 2026-08-17T14:50:41Z), i.e. *before* the child
   closed. (Attribution corrected 2026-09-07, bead domchk-791bfb2e, against
   the live bead records; an earlier revision of this addendum credited
   bf-173o7e with the 753 MB metrics, which its record does not contain.) The
   Summary's "gc completed on the 53rd attempt" is therefore corrected to:
   *the bead was decomposed on the 53rd attempt; the gc completed under
   bf-173o7e.*
4. **The storm continued on the child.** bf-173o7e recorded **131 completions
   on Aug 14 alone — 129 × exit -1 between 12:59:48Z and 23:25:35Z, plus one
   124 and one 0** — the same 39–116 s mid-gc kill pattern for another ~10.5
   hours. Addendum 2's "uncorrelated" note understates this: bf-173o7e was
   created by bf-4x12ec's own 53rd attempt, bringing Aug-14 totals to **173
   SIGKILLed attempts of the same gc command on the same repository** across
   parent and child.
5. **No per-worker memory containment exists.** Live inspection of
   `needle-worker@*.service` (2026-09-02): `MemoryMax=infinity`,
   `MemoryHigh=infinity`, `CPUQuotaPerSecUSec=infinity`. Combined with
   Addendum 2's CONSTRAINT_MEMCG findings for other scopes on this box, a
   bounded per-worker memory limit is the generic fix for this kill class;
   `scripts/safe-git-gc.sh` addresses it for gc specifically.
6. **Reproduction: deliberately not attempted.** The trigger state is gone
   (35 loose objects / 300 KiB today). Recreating an 18 GB loose-object repo
   to induce OOM on a shared box running 13+ needle workers risks killing
   unrelated agents' work, and with Aug-14 kernel logs unrecoverable (current
   boot began 2026-08-15 19:26 EDT) even a reproduced kill could not be
   validated against contemporaneous system records.
7. **Loose ends:** children bf-5jhvpk (repack) and bf-im2sl1 (verify) remain
   **Open** while the parent and bf-173o7e are Closed — housekeeping
   candidates for a later pass.

Classification for `domchk-d986ce54`: **FALSE POSITIVE** duplicate alert
(bf-4x12ec Closed since 2026-08-17), with the transcript-level evidence above
as this investigation's contribution to the record.

### Verification of this addendum (2026-09-02, second dispatch of domchk-d986ce54)

The first dispatch of this alert bead wrote the findings above but exited
before committing (two ~50-minute dispatches, both exit 1 — the
orphaned-after-success pattern this repo documents). This second dispatch
re-verified every claim above against primary sources before committing:

- **Attempt #20 / alert timestamp:** the needle event log shows dispatch at
  `10:51:03.234Z`, `agent.completed` `exit_code: -1` at `10:51:49.607Z`
  (46.4 s), and the `HANDLING_RELEASE_DONE` teardown heartbeat at
  `10:52:14.447Z` — exactly the alert timestamp that generated this
  investigation.
- **Transcript `38af9cbf`:** 22 lines; the agent measured 4,649 loose objects
  / 17.20 GiB / 18G `.git` and the transcript's final event is the
  `git gc --aggressive --prune=now` tool call with no tool result — killed
  mid-gc.
- **Transcript `31800ee3`:** opens with needle's auto-split prompt ("failed 8
  times in a row"), creates bf-173o7e / bf-5jhvpk / bf-im2sl1, ends
  `SPLIT_COMPLETE`.
- **bf-173o7e Aug-14 completions recomputed from the event log:** 131 =
  129× exit -1 + 1× 124 + 1× 0, as claimed. Children bf-5jhvpk (repack) and
  bf-im2sl1 (verify) confirmed still Open; bf-173o7e Closed.
- **`de7af48`** (2026-08-17): "fix: correct gc.aggressiveWindow to proper
  numeric format (1 hour)" — confirming the misreading described in point 2;
  `gc.aggressivewindow` is absent from `.git/config` today.
- **Refinement to point 5:** `needle-worker@.service` itself carries no
  Memory*/CPUQuota settings (defaults, i.e. unlimited), but the shared
  `needle.slice` is capped by drop-in at **`MemoryMax=32G` /
  `CPUQuota=700%`** — fleet-level containment does exist; what does not exist
  is per-worker or per-dispatch containment beyond the transient
  `run-p*.scope` `MemoryMax=12GiB` that killed the gc runs.
- **Current system state (2026-09-02 11:15 EDT):** 62G RAM / 51G available,
  load 7.9, 94G disk free, uptime 18 days (boot 2026-08-15 — confirming no
  Aug-14 kernel logs survive, per Addendum 2); repository 5 loose objects /
  44 KiB plus a single 90.18 MiB pack — healthy.

---
**Addendum 4 Investigation Date:** September 2, 2026
**Addendum 4 Sources:** per-attempt session transcripts (`~/.claude/projects/-home-coding-domain-check/`), needle event log 2026-08-14, live bead records (bf-173o7e, bf-5jhvpk, bf-im2sl1), systemd unit inspection, git history (`de7af48`, `8d7ce53`)
**Classification:** FALSE POSITIVE — duplicate alert; contributed transcript-level evidence and child-bead storm continuation analysis

## Addendum 5 — Re-verification and Summary Correction (2026-09-02, bead domchk-90640785)

Alert bead `domchk-90640785` (created 2026-08-26T21:13:53Z, dispatched
2026-09-02) tasked a fresh investigation of this crash. Findings:

- **bf-4x12ec is Closed** (2026-08-17T14:50:41Z); the work completed
  2026-08-14T12:58:45Z on the 53rd attempt *(phrasing corrected by Addendum 4
  §3: attempt 53 ran needle's auto-split, not the gc — the gc completed under
  child bf-173o7e; annotated 2026-09-08, bead domchk-8c78ae8b)*. The alert fired **nine days after
  both** — this is another instance of the duplicate-alert pattern documented
  in a dozen prior verification reports (bf-qz9mov, bf-1uh46l, bf-48vwac,
  bf-4h2mqq, bf-4xbt4g, bf-4oblul, bf-2m532x, bf-3cy3vk, bf-44upi7, bf-2u3dzu,
  bf-5f9xqg, domchk-661c2dc6). Classification: **FALSE POSITIVE**.
- **Primary event log independently re-verified**
  (`claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl`): 53 claims, 53
  dispatched, 53 completed — **44 × exit -1, 8 × exit 124, 1 × exit 0**,
  matching Addendum 2's three-phase table exactly.
- **Current repository health confirmed** (2026-09-02): 92MB `.git`, 35 loose
  objects, 10,408 in-pack, 0 garbage — the cleanup has held, with further
  reductions delivered by the scheduled safe-git-gc timers.

### Correction: reverted a reintroduced debunked claim

An uncommitted working-tree edit (2026-09-02 ~10:32 EDT) rewrote this report's
Summary to "first alert at 10:41:13Z" and "the `git gc` operation itself
SIGKILLed at 11:14:39Z after ~57 minutes" — both refuted by Addendum 2's
primary-source analysis (no attempt survived 115.8 s; 10:41:13 was attempt
#13's alert, not the first; 11:14:39 was attempt #31's). The 57-minute figure
is simply bead creation (10:17:26Z) to the attempt-#31 alert (11:14:39Z), not
a gc runtime. The Summary and Crash Window were restored to the
event-log-supported narrative; the body's v1.0/v1.1 sections are retained as
historical record, with Addendum 2's timestamp table resolving them.

---
**Addendum 5 Investigation Date:** September 2, 2026
**Addendum 5 Sources:** needle event log 2026-08-14 (primary), live bead record, live `git count-objects`
**Classification:** FALSE POSITIVE — duplicate alert on an already-resolved, closed bead

## Addendum 6 — OOM-Corroboration Count Correction (2026-09-02, bead domchk-4adc1a55)

A third independent re-verification (alert bead `domchk-4adc1a55`, created
2026-08-26T20:44:44Z, dispatched 2026-09-02) reproduced Addenda 2 and 3
exactly from the primary log: 53 `agent.completed` events for bf-4x12ec,
all on 2026-08-14 — **44 × exit -1 (38.9–115.8 s, 10:23:02–11:27:26Z), 8 ×
exit 124 (exactly 600.0 s, 11:38:07–12:50:14Z), 1 × exit 0 (491.8 s,
12:58:45Z)**; 0/44 correlated completions on any other bead within ±3 s;
`journalctl --list-boots` confirms the current boot began 2026-08-15
19:26:03 EDT and `.beads/logs/resource-metrics.log` begins
2026-09-01T22:49Z, so no direct Aug-14 memory telemetry exists.

### Correction: the memcg OOM record is 30× larger than Addendum 2 stated

Addendum 2 wrote: *"All 13 `Killed process` events in the current boot
(6 node/vitest, 6 bash, 1 git) are CONSTRAINT_MEMCG."* The actual count:

| comm | Killed-process events |
|------|----------------------|
| `git` | **257** |
| `node (vitest …)` | 156 |
| `bash` | 6 |
| **total** | **419** (420 `invoked oom-killer` reports, all `constraint=CONSTRAINT_MEMCG`) |

All 257 git kills fall on **2026-08-16, 00:27:35–13:29:51 EDT** — inside the
Aug-14→Aug-17 bloat-cleanup window (`docs/cleanup-resolution-2026-08-17.md`)
— at **11.7–12.6 GB anon-rss** (peak 12,555,188 kB), the memory profile of
pack-objects grinding an 18GB loose-object repository. **No git OOM kill
occurs on any other day of the current boot.** Kernel logs do not record
cwd, so individual kills cannot be attributed to bf-4x12ec's own gc
attempts, but the window, magnitude, and one-sided distribution make
bloat-era git operations the overwhelmingly likely driver. The cited
corroboration event is at 13:29:**51** EDT, not 13:29:49.

This correction **strengthens** the OOM determination: it is no longer one
same-period example but 257 same-window instances of the identical
mechanism (memcg limit, `oom_score_adj=200` preferred-victim marking) that
best explains the 44 phase-1 deaths.

Residual current-boot OOM activity outside Aug 16: only 6 `bash` kills on
2026-09-02 (07:15–08:32 EDT), each ~63 MB anon-rss — tiny victims inside
memory-limited transient `run-p*.scope`s, unrelated to repository bloat.
The repo-era risk is gone; the scope-limit pattern remains occasionally
active but harmless at current workload levels.

**Bottom line for dispatchers:** root cause confirmed as memcg OOM during
the bloat era, resolved by the Aug-17 cleanup; bf-4x12ec's workload (the
aggressive gc itself) was the crash trigger and is complete — no retry.

---
**Addendum 6 Investigation Date:** September 2, 2026
**Addendum 6 Sources:** needle event log 2026-08-14 (primary), `journalctl -k` current-boot full scan, `journalctl --list-boots`, live bead record, live `git count-objects`/`git fsck`
**Classification:** FALSE POSITIVE — duplicate alert; root cause memcg OOM (bloat era), resolved

## Addendum 7 — Consolidation: parent acceptance-criteria mapping and completion verification (2026-09-08, bead domchk-fe10456e)

This addendum is the final consolidation leg of the investigation chain run under parent
`domchk-46a00141` ("Investigate agent crash logs for bf-4x12ec"), whose three acceptance
criteria this record exists to satisfy:

1. Crash logs retrieved and saved to `docs/crashes/`
2. Summary of what bf-4x12ec was doing
3. Exit signal analysis documented

Nothing below re-opens the analysis. Every element was already delivered by committed,
pushed work from the chain's children; this section binds each criterion to its delivering
bead, commit, and on-disk artifact, and re-verifies the crashed bead's completion status
live. It is an index into the record above and into the sibling deliverables, not a new
determination.

### Criterion 1 — Crash logs retrieved and saved to `docs/crashes/` ✅

Delivered by the log-retrieval child **`domchk-4bad8e94`** (Closed): `docs/crashes/bf-4x12ec/`
— commits `c781138` (the bundle) and `8ae57ea` (restored the section-inventory document the
bundle commit had dropped). Verified on disk 2026-09-08: `README.md`, `MANIFEST.sha256`,
`attempt-index.tsv`, `bracket-source-lines.tsv`,
`needle-events-2026-08-14-bf-4x12ec.jsonl.gz` (the day's needle event stream for the bead),
`needle-events-…-attempt2-bracket.jsonl`, `session-transcript-attempt2-971486ad.jsonl`, and
`transcripts/` (55 files: all 53 per-attempt transcripts plus `census.tsv` and
`MANIFEST.sha256`).

Augmenting bundles delivered by the parallel `domchk-c99cdf80` split:

- `docs/crash-investigations/evidence/bf-4x12ec/crash-logs/` (`9b32085`, bead
  `domchk-48f3e34d`) — 1,146 raw needle event lines for the bead, the crash-window segment
  with interleaved worker-state events, four verbatim attempt transcripts (attempt-1 crash,
  mid-storm, auto-split timeout, split-success), the 44 auto-minted alert-bead records, and
  the per-attempt exit-code timeline (44 × −1 / 8 × 124 / 1 × 0).
- `docs/crash-investigations/evidence/bf-4x12ec/system-state.md` (`3b2bdf9`, bead
  `domchk-40c9c99a`) — monitoring logs absent on Aug-14; host metrics show no memory, disk,
  or load pressure at crash time.
- Log-source inventory
  `docs/crash-investigations/bf-4x12ec-log-source-inventory-domchk-a3f1f8f5-2026-09-07.md`
  (`e1d9477`, bead `domchk-a3f1f8f5`), and primary-source evidence + signal −1 semantics
  `docs/crash-investigations/bf-4x12ec-evidence-signal-semantics-domchk-15854355-2026-09-07.md`
  (`76ad268`, bead `domchk-15854355`).

### Criterion 2 — Summary of what bf-4x12ec was doing ✅

Delivered by the task-context child **`domchk-520b8682`** (Closed 2026-09-07 — cascade
close, "deliverable already rendered on origin/main"), rendered in
`docs/notes/bf-4x12ec-crash-investigation.md` § "Original bead context" (`a5c4c07`, bead
`domchk-c1c0afd8`) and in this document's **Crashed Bead Details** and **Original Work
Context** sections: bf-4x12ec was **Phase 1.2 emergency repository stabilization** —
"Execute aggressive git garbage collection to pack 17.20GB of loose objects into compressed
pack files, eliminating the OOM risk during git operations" — created
2026-08-14T10:17:26.387Z, first kill 10:23:02.958Z. The bead-workspace migration that v1.0
of this report conflated with it was a separate, completed effort (`61d27ac`, 2026-08-15).

### Criterion 3 — Exit signal analysis documented ✅

Delivered by this bead's direct dependency, the exit/signal child **`domchk-0e707410`**
(Closed, commit `d364ff5`): `docs/notes/bf-4x12ec-crash-investigation.md` § "Exit code and
signal analysis", grounded in the semantics record
`docs/signal-analysis-exit-code-negative-one.md` (present, 33.6 KB). Inside this document
the same analysis appears in the **Signal Analysis** section, Addendum 2's "Signal −1,
precisely", and Addendum 3's root-cause determination. The consolidated statement:

- `exit_code = −1` is needle's **writer-side sentinel** for a worker that died with no wait
  status (`status.code().unwrap_or(-1)`; any signal flattens to −1) — it is not a POSIX
  exit status, and the alert body's "(signal −1)" is the crash handler's template
  arithmetic on that sentinel, not a signal number.
- The 53-attempt census from the retrieved bundle: 44 × exit −1 (`crash`, 38.9–115.8 s),
  8 × exit 124 (`timeout`, exactly the 600 s cap), 1 × exit 0 (`success`, 491.8 s at
  12:58:45Z) — no other exit codes appear in the stream.
- Mechanism: **memcg OOM inside the agent's transient `run-p*.scope` (`MemoryMax=12GiB`)**
  over the 17–18 GB loose-object repository — kernel-proven same-window via the 257 Aug-16
  git `CONSTRAINT_MEMCG` kills at the 11–12 GB RSS ceiling (Addenda 3 and 6). Not system
  OOM, not a harness timeout, not a code defect.

### Completion status of bf-4x12ec (verified live 2026-09-08)

- **Closed**, rev 4, updated 2026-08-17T14:50:41Z; the work itself completed
  **2026-08-14T12:58:45Z** (attempt 53). Attempt 53 did not run the gc — it executed
  needle's auto-split (Addendum 4 §3), and the `git gc --aggressive --prune=now` completed
  under child **bf-173o7e** (Closed, rev 19).
- Auto-split family state at consolidation time: bf-173o7e (gc) **Closed**; **bf-5jhvpk
  (repack) Open** and actively in progress under bead `domchk-371e54d8` (the memory-capped
  `git repack -a -d --depth=250 --window=250` leg); **bf-im2sl1 (verify) Open**. The two
  targets the parent's completion notes marked PARTIAL (753 MB vs <500 MB; 141 vs <100
  loose) are exactly those two children's remit — open follow-on work, not unresolved
  defects in the gc.
- Live repository state at 2026-09-08T00:37Z (this bead's own reading): `.git` **104 MB**,
  **101 loose objects / 656 KiB**, 12,174 in-pack, 1 pack / 100.25 MiB, 0 garbage,
  `git fsck --no-full` exit 0 (dangling objects only). The bloat-era state is gone and has
  held for three weeks. The loose-object count sits at normal churn on the healthy/warning
  rounding boundary (101 vs the <100 line); total repository size — the metric that
  actually carried the OOM risk — is ~0.6% of the bloat-era 18 GB. No bloat signature.

**Close-time re-verification (2026-09-08, this bead's closing dispatch).** The
split-family bullet above is superseded in one respect: the repack leg is no
longer pending. `domchk-371e54d8` **Closed** (rev 4, 00:35:06Z) after executing
the bf-5jhvpk target repack at **2026-09-08T00:28:54Z** — `git repack -a -d
--depth=250 --window=250` at full depth/window (no fallback tier), exit 0 in 3 s,
scope peak **350.4 MB** against its 4 GB `MemoryMax` cap (no OOM in
journalctl/dmesg), **2 packs → 1** (single 100.25 MiB pack, old packs pruned),
`.git` 107 MB → 105 MB. "Actively in progress" was already stale when this
addendum was published (f78d150, 00:51Z — 22 minutes after that run finished);
the 1-pack / 100.25 MiB reading in the bullet below it *is* that run's output.
With the repack landed, the parent's first PARTIAL target (repository size
< 500 MB) is met at the repo level: **105 MB** at close time, ~0.6% of the
bloat-era 18 GB. Loose objects sat at **133 / 872 KiB** at close time (up from
101 at 00:37Z) — normal churn around the <100 line, swept by the daily 03:00 gc.
The target bead **bf-5jhvpk itself remains Open** (rev 18) with the execution
recorded on its agent bead, and **bf-im2sl1** (verify) remains Open — the same
open-target/closed-executor shape this family shows throughout. Nothing in the
AC mapping above changes: bf-4x12ec still Closed rev 4, the dependency child
`domchk-0e707410` still Closed (rev 4, verified live 2026-09-08), and all three
criteria still bind to the beads/commits/artifacts listed above.

### Residual work (owned elsewhere, not owed by this consolidation)

Body-level harmonization of this report — the section inventory's six contradictions and
thin spots (`cafd43e`, bead `domchk-f6aba211`: the "53rd attempt" phrasing in the Summary
and resolution step 2, the body RCA's superseded pack-figures, the Signal section's
"Definitive Identification" wording, the superseded 2026-09-01 corpus figures under System
State, and the unreconciled 4,627 vs 4,649 before-count) — is tasked to the
gap-finalization bead **`domchk-8c78ae8b`** (in progress at consolidation time). Addendum 4
§3 already corrects the 53rd-attempt narrative in place; this addendum deliberately leaves
the body text untouched to avoid colliding with that in-flight revision.

*(Completed 2026-09-08 by `domchk-8c78ae8b`, report v1.10: dated Correction blocks now
sit under the Summary, System State, Signal Analysis and Root Cause Analysis, harmonizing
each with the addenda; the 4,627 vs 4,649 reconciliation had already landed under the
metrics table via bead `domchk-791bfb2e`.)*

---
**Addendum 7 Investigation Date:** September 8, 2026
**Addendum 7 Sources:** live bead records (bf-4x12ec, bf-173o7e, bf-5jhvpk, bf-im2sl1, domchk-46a00141 and its four chain children), git history (`c781138`, `8ae57ea`, `9b32085`, `a5c4c07`, `d364ff5`, `e1d9477`, `76ad268`, `3b2bdf9`), on-disk artifact verification, live `git count-objects -vH` / `git fsck --no-full`
**Classification:** FALSE POSITIVE — duplicate alert on a closed bead; consolidation record, no new findings
## Addendum 8 — Crash-Window Resource Timeline and Safe-Operating-Limits Verdict (2026-09-08, bead domchk-5f3ec6e1)

Split child 2 of 4 of umbrella `domchk-4adc1a55`; scope: correlate resource
conditions with the crash window using the timestamps child 1 established
(domchk-15854355: 44 × exit −1, 2026-08-14 10:23:02Z → 11:27:26Z). Every figure
below was re-derived first-hand this dispatch from the cited primary sources;
where a figure also appears in `evidence/bf-4x12ec/system-state.md` (bead
domchk-40c9c99a) the independent derivation agrees. All times UTC.

### The monitoring layer named in the task did not exist yet

| Source named in the task | Earliest record | Aug-14 coverage |
|--------------------------|-----------------|-----------------|
| `.beads/logs/resource-monitor.log` | 2026-09-02T01:50:47Z (line 2; line 1 is a `--quiet` argv error from a first manual run) | none — `grep -c "2026-08"` = 0 |
| `.beads/logs/resource-metrics.log` | 2026-09-01T22:49:42Z | none — `grep -c "2026-08"` = 0 |
| journalctl (OOM killer / memcg / disk) | single boot, first entry **2026-08-15 19:56:33 EDT** | none — `journalctl --since @1786701191 --until @1786707480` (the whole 09:53:11Z–11:38:00Z window) returns **0 lines**; zero lines matching `Aug 14` exist anywhere in the journal |

The resource-monitoring layer was installed 2026-09-01, eighteen days after the
crash, and the box rebooted between the crash and the current boot. No
monitoring-layer record of this window can exist. What survives is
contemporaneous telemetry captured inside the window by other systems.

### Resource timeline, crash #1 through resolution

Sources: `crashes/bf-4x12ec/attempt-index.tsv` (kill instants/durations/exit
codes), `crash-logs/transcript-midstorm-9539f3b2.jsonl` (in-window host
readings), `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl`
(`fleet.cpu_saturated` load series, 383 events that day).

| Time (2026-08-14) | Event | Resource reading |
|-------------------|-------|------------------|
| 08:23:02Z | — | day's 1-minute load maximum **55.98** — no crash or kill follows it (2 h before the storm) |
| 09:53:11–10:21:06Z | pre-storm | 5 load samples, 8.77–22.51, mean 15.18 |
| 10:21:06Z | attempt 1 dispatched | load 13.13 |
| 10:21:23.336Z | attempt 1 runs `git count-objects -vH` | **4,649 objects / 17.20 GiB loose**, size-pack 9.60 MiB |
| 10:23:02.958Z | **kill #1** — exit −1 after **115,797 ms** | (host gauges: none captured) |
| 10:23:02 → 11:27:26Z | **44 kills**, exit −1 each, cadence one per ~90–100 s | durations 38,882–115,797 ms, **mean 64,645 ms**; load samples n=44, **10.37–30.92, mean 14.47** |
| 10:43:38.687Z | attempt 15 dispatched | load 19.66 |
| 10:43:49.108Z | attempt 15 re-runs `count-objects` | **byte-identical** to 10:21:23 — no gc had completed; repo still 17.20 GiB |
| 10:43:59.438Z | attempt 15 runs `df -h / && free -h` — **the only in-window host reading** | disk `/` 444G total, 355G used, **67G free (85 %)**; mem 62Gi total, 16Gi used, 24Gi free, 22Gi buff/cache, **45Gi available**; **swap 0 B used** |
| 10:44:07.643Z | attempt 15 issues `git gc --aggressive --prune=now` | — |
| 10:44:53.202Z | **attempt 15 killed**, exit −1 after 74,285 ms | 46 s into the gc, 54 s after the healthy host reading |
| 11:27:26Z | kill #44 — last exit −1 | storm's max load 30.92 recorded 11:21:25Z, six minutes earlier |
| 11:38–12:50Z | 8 × exit 124, mean 600,020 ms | the 600 s timeout cap — the gc no longer died fast, it hung |
| 12:50:33Z | attempt 53 dispatched | exits **0** at 12:58:45Z after 491,784 ms — needle's auto-split (`SPLIT_COMPLETE`, Addendum 4 §3), not the gc |

### Readings against the repo CLAUDE.md safe-operating-limits table

| Resource | Healthy / Warning / Critical | Crash-window reading | Verdict |
|----------|------------------------------|----------------------|---------|
| Available memory | 20 GB / 10 GB / 5 GB | **45 Gi available**, 0 B swap (10:43:59Z, mid-storm) | **healthy** — 2.25× the healthy floor, 9× the critical line |
| Disk space | 50 GB / 30 GB / 20 GB free | **67 G free** (85 % used on 444 G) | **healthy** — above even the healthy minimum |
| CPU load (1 min) | < 5 / < 10 / > 15 | 44 storm samples 10.37–30.92, mean 14.47; pre-storm mean 15.18 | **exceeds the warning and critical lines — but uncorrelated with the kills** (see below) |
| Git GC memory | 1 GB / 2 GB / 4 GB | killed `git` processes at **11.73–11.97 GiB anon-rss against the 12 GiB dispatch-scope `MemoryMax`** (Aug-16 same-regime kernel records, below) | the binding limit was the *scope cap*; the gc-script bounds postdate the crash |

On load: the series is threshold-gated (`fleet.cpu_saturated` emits only above
0.8 × 9 reported cores = 7.2), so all 44 samples being well above the floor
proves load was *persistently* elevated and flat — 14.47 storm mean vs 15.18
pre-storm mean is no ramp, and the largest excursion of the day (55.98) came
two hours before the storm and killed nothing. A host entering swap-death or
allocator thrash shows escalating load; this one never trends. The kills are
one per ~90–100 s with 39–116 s lifetimes — a deterministic, per-attempt
cadence, not the irregular signature of system-wide resource collapse.

### Kernel evidence for the mechanism (post-window, same regime)

No Aug-14 kernel record survives, but the current boot holds the same kill
under the same conditions: **257** `oom-kill:constraint=CONSTRAINT_MEMCG …
task=git` events on Aug-16 (first-hand recount; Addendum 6's figure confirmed),
with killed `git` processes at `anon-rss` 12,301,364–12,555,188 kB
(**11.73–11.97 GiB**, i.e. pressed against the scope's 12 GiB cap) and
`oom_memcg` naming a `user@1001.service/app.slice/run-*.scope` needle dispatch
scope. (The Sep-02/06/07 memcg kills in the same grep are synthetic
test-scope events, not live agent work.) This is corroboration by regime
match, not observation of the Aug-14 window — consistent with Addendum 2 §
Evidence-window limitation.

### Explicit resource verdict

**OOM — cgroup-local, not host. Disk normal. Load elevated but uncorrelated.**
The crash correlates with memcg exhaustion *inside the 12 GiB dispatch scope*:
each attempt's `git gc --aggressive --prune=now` drove its own cgroup over the
scope cap in 39–116 s, while the host itself held 45 Gi available, untouched
swap, 67 G disk free, and flat chronically-elevated load. No host-level gauge
in the CLAUDE.md limits table was in a state that predicts a kill; a scope-local
memcg kill is invisible to them, which is exactly why the host readings look
healthy through 44 consecutive deaths. Disk did not contribute — the 17.20 GiB
of loose objects was repository bloat (the very thing the gc was meant to fix),
not disk exhaustion. Load did not contribute — flat, chronic, and uncorrelated
with kill instants.

Limits of this verdict: no per-cgroup telemetry existed on Aug-14, so the
scope watermark is not directly measured for this window; the verdict rests on
excluding host exhaustion (the only in-window reading), the deterministic kill
cadence, and the Aug-16 kernel records of the same mechanism at the same cap.

**Addendum 8 Sources:** `.beads/logs/resource-monitor.log` + `resource-metrics.log` (absence, greps run this dispatch); journalctl (boot list, epoch-bounded window query, `journalctl -k` memcg recount); `docs/crashes/bf-4x12ec/attempt-index.tsv`; `docs/crash-investigations/evidence/bf-4x12ec/crash-logs/transcript-midstorm-9539f3b2.jsonl`; `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl`; `docs/crash-investigations/evidence/bf-4x12ec/system-state.md` (independent derivation, agreeing); repo CLAUDE.md safe-operating-limits table
**Addendum 8 version note:** appended at HEAD `e0b70dab` (version line still 1.9/1.10 in flight — the v1.10 body-harmonization of bead domchk-8c78ae8b was uncommitted in the worktree and is intentionally not carried by this addendum); no version bump claimed by this addendum.

---

## Addendum 9 — Workload Contribution and Reproducibility Assessment (2026-09-08, bead domchk-78d89c6b)

Split child 3 of 4 of umbrella `domchk-4adc1a55`; scope: what bf-4x12ec was executing at
crash time, whether that workload contributed to the crash, and whether the crash is
reproducible or a one-time event. Depends on child 2's resource verdict (Addendum 8) for
the resource conclusions used below; nothing here re-opens them. All times UTC.

### What bf-4x12ec was executing at crash time

**The bead's own deliverable command — `git gc --aggressive --prune=now`, mid-flight at
every kill.** Not bead operations, not post-completion cleanup, not an incidental
background job: the worker died inside the task it was dispatched to do. Re-verified
first-hand this dispatch from the retrieved bundles:

- Both surviving crash-era session transcripts end at that exact unanswered `Bash`
  `tool_use` with no following `tool_result` — attempt 1 (`transcript-attempt1-crash-8b2a5b0d.jsonl`, where the gc is preceded by a `git config --unset gc.aggressivewindow` adjustment) and attempt 15 mid-storm (`transcript-midstorm-9539f3b2.jsonl`, where it is preceded by attempt 15's `df -h / && free -h` host reading). The operation-summary census (`docs/crash-investigations/evidence/bf-4x12ec/operation-summary.md`, bead domchk-dfce2360) extends this to **44/44 exit −1 attempts** dying inside the same command, killed 18.6–80.6 s (mean 30.8 s) after issuing it (per-attempt lifetimes 38.9–115.8 s, `attempt-index.tsv`).
- **No attempt ever completed the gc.** `git count-objects -vH` re-run at 10:43:49.108Z
  returned **byte-identical** output to 10:21:23.336Z — 4,649 objects / 17.20 GiB loose
  (Addendum 8's timeline) — so the store was untouched by everything the storm threw at
  it. At every kill the work was still to do; that is the opposite of a
  post-completion kill.
- The eventual completion belongs to a different bead four days later: auto-split child
  **bf-173o7e**, closed 2026-08-17T17:12:09Z ("17.20GB loose objects packed into 444MB
  pack file, repository valid"). bf-4x12ec's own completion notes ("Git cleanup completed
  successfully despite agent crash") compress this into one sentence and are easy to
  misread as "the crash came after the work" — the addenda above (2 §3, 4, 7) correct
  that reading; this one states it plainly: **at all 44 kills, nothing had completed.**

### Comparison against the documented patterns

Against `docs/crash-response-guide.md` and
`docs/research/root-cause-analysis-signal-minus-one-crashes.md` (with its dated
correction of 2026-09-07, bead domchk-15999f2c):

| Documented pattern | Match? | Reading for bf-4x12ec |
|---|---|---|
| Pattern 1 — Post-completion false positive (~40 %) | **No** | No commit, no deliverable, no completion within 30 s — or at all — before any kill (byte-identical `count-objects` above). The 30-second gap heuristic has nothing to latch onto. |
| Pattern 2 — Git GC operations (~15 %) | **Exact match** | `git gc --aggressive` in progress, exit −1. Note the pattern's verification step ("repository valid and compressed → gc succeeded, termination was cleanup") mis-leads if applied across the storm: the repository became valid and compressed only via bf-173o7e four days later. No in-storm attempt got there. |
| Pattern 3 — Infrastructure: repository bloat (~15 %) | **Exact match** | 17.20 GiB loose objects; fixed ~90–100 s re-dispatch cadence; zero exit-code variation across 44 deaths; routine git operation OOM. |
| FP Rule 1 (commit < 30 s before crash) | No | No commit existed anywhere in the storm window. |
| FP Rule 2 (crash → retry → success = self-healed transient) | **Surface match only** | The guide's own bf-1s6c3 caveat predicts this case: attempt 53's exit 0 was needle's **auto-split** (`SPLIT_COMPLETE`, Addendum 4 §3) changing the task shape, not the gc completing, and the environment did not change on Aug-14 — the gc child bf-173o7e's own attempts died the same way before its eventual success. A persistent cause outlasting the retry loop is Infrastructure, not transient. |
| Research doc: exit −1 semantics | Consistent | −1 is needle's writer-side sentinel for a worker dead with no wait status; it identifies no signal number, and the doc's "SIGHUP cascade" rank has zero log support (its own dated correction). Classification: memory-pressure/OOM (Type 1) over repository bloat (Type 5) — infrastructure, not code, not workflow, not service. |

**Kill layer vs alert layer.** The 44 kills are real INFRASTRUCTURE events. The 44
auto-minted one-alert-per-kill beads are the FALSE_POSITIVE layer — the target bead
closed rev 4 on 2026-08-17, so every alert on it is stale by the guide's classification
table (44 storm alerts + 16 regeneration beads: `docs/crash-investigations/bf-4x12ec-alert-inventory.md`). A real kill and a false-positive alert are not contradictory
here; they are different layers of the same storm.

### Verdict: did the workload contribute?

**Yes — it is the proximate trigger and the sole memory consumer, and none of that is a
defect in the work performed.** Three conditions were jointly necessary; remove any one
and no crash occurs:

1. **17.20 GiB of loose objects** — the precondition, which is itself the thing the task
   existed to fix (Phase 1.2 emergency stabilization "to eliminate the OOM risk").
2. **No pack-memory bound** — `pack.windowMemory` was not set anywhere until 2026-09-02,
   so `pack-objects` was free to build its delta window and delta cache without limit.
3. **The 12 GiB dispatch-scope `MemoryMax`** — the kill boundary (Addendum 8; the
   Aug-16 kernel records show the same `git` deaths at 11.73–11.97 GiB anon-rss against
   that cap, and the surviving-journal recount in
   `docs/crash-investigations/evidence/bf-4x12ec/kernel-systemd-messages.md`, bead
   domchk-ad80e265, finds 375 kills at exactly `usage==limit==12582912kB`).

The workload's contribution is therefore **condition interaction, not workmanship**: a
legitimate, necessary operation whose unbounded memory profile — over a bloated store,
inside a capped scope — was the crash mechanism. The classification stays
INFRASTRUCTURE with no code defect, consistent with this report's standing finding and
with the repo-wide rule that domain-check code is never in the causal path (this bead
touches git object storage only).

### Reproducible or one-time?

**Under the 2026-08-14 configuration: deterministically reproducible.** 44 consecutive
identical kills of the same command, then 8 further attempts at the 600 s timeout cap,
is a deterministic environmental kill, not a fluke. Nor was it one-time in fleet terms —
the same regime killed bf-1ea4g's push-side variant (56/57), bf-4k2ws (55 kills), and
bf-173o7e's own gc storm on this same operation, in the bloat era the guide's Pattern 3
describes.

**Today: not reproducible.** Both of the crash's own preconditions changed, verified
live this dispatch (2026-09-08), all first-hand:

| Guard | Live check this dispatch | Result |
|---|---|---|
| Precondition removed | `git count-objects -vH`; `du -sh .git` | **252 loose objects / 1.67 MiB** (was 4,649 / 17.20 GiB — a ~10⁴× smaller loose mass), 1 pack / 100.25 MiB, 0 garbage, `.git` 106 MB |
| Memory bound in place | `./scripts/setup-git-gc-config.sh --verify` | exit 0 — effective chain system → global → local: `windowMemory=2g` / `deltaCacheSize=1g` / `threads=1` → worst case **≈3072 MiB** against the 12 GiB scope |
| Death-command replay | `./scripts/test-gc-memory-bounds.sh` | **17/17 pass** — the exact crash command `git gc --aggressive --prune=now` exits 0 under `MemoryMax=768M` (1/16 of the dispatch scope), pack-objects peak RSS **320,556 KB** vs the >12 GiB the unbounded run consumed |

Corroboration: bf-5jhvpk's repack at full `--depth=250 --window=250` executed
2026-09-08T00:28:54Z exited 0 at a 350.4 MB scope peak under its 4G cap (Addendum 7's
close-time re-verification) — the heavy-pack operation class now completes under bounds
on this repository.

**Return condition:** the crash comes back only if **both** the bloat precondition
recurs **and** the pack-memory bound is removed — exactly the two conditions Pattern 3's
prevention layers (repo-wide `.beads/` gitignore with 0 tracked bead state, the 10 MB
pre-commit gate, the persistent pack-memory config repo-local and global, the daily
02:00 health check) exist to block. Retry safety itself is child 4's remit
(domchk-dba1e0bb); what this addendum contributes to it is that a re-run of the bare
command is no longer the death operation it was in August.

**Limits:** no per-cgroup telemetry existed on Aug-14 (Addendum 8), so "deterministic
then" rests on the 44/44 census and the byte-identical `count-objects` readings, not on
a measured scope watermark. The replay is scaled (8 × 64 MiB blobs, not 17.20 GiB); the
deployed-config worst case (≈3072 MiB) is bound arithmetic, not a bloat-scale re-run,
which would be unsafe on the shared repository and is deliberately not attempted. The
kernel-systemd evidence file cited above is a sibling bead's deliverable in flight
(domchk-ad80e265) and was unpushed at this dispatch; its figures are corroboration, not
load-bearing here.

**Addendum 9 Sources:** `docs/crash-investigations/evidence/bf-4x12ec/operation-summary.md` (44/44 census); `docs/crash-investigations/evidence/bf-4x12ec/crash-logs/transcript-attempt1-crash-8b2a5b0d.jsonl` + `transcript-midstorm-9539f3b2.jsonl` (fatal `tool_use`, re-read this dispatch); `docs/crashes/bf-4x12ec/attempt-index.tsv` (53-attempt exit-code census); `docs/crash-response-guide.md` (Patterns 1–3, FP Rules 1–3, bf-1ea4g Pattern 6); `docs/research/root-cause-analysis-signal-minus-one-crashes.md` (+ its 2026-09-07 dated correction); live bead records bf-4x12ec (Closed rev 4) and bf-173o7e (Closed rev 19); live `git count-objects -vH` / `du -sh .git` / `setup-git-gc-config.sh --verify` / `test-gc-memory-bounds.sh` (this dispatch); `docs/crash-investigations/bf-4x12ec-alert-inventory.md`; Addenda 4, 7 and 8 of this report
**Addendum 9 version note:** appended on top of origin/main `a00d02bb` as v1.11; the pre-edit worktree copy was byte-identical to local HEAD `c3de56b` (file blob `d517ec00`, verified by hash-object), and this file's staged 290-line deletion in the shared index is a co-tenant's in-flight state, not carried by the commit that publishes this addendum.
