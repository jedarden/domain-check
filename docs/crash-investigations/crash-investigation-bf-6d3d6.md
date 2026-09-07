# Crash Investigation Report: Bead bf-6d3d6

## Summary

**Bead ID**: bf-6d3d6  
**Task**: Identify common ancestor commit  
**Agent**: claude-code-glm-4.7  
**Exit Code**: -1 (signal -1)  
**Timestamp**: 2026-08-13T11:21:00.516259223+00:00  
**Status**: Agent process was killed, work was eventually completed (bead now closed)

## Root Cause Analysis

### 1. What the Agent Was Doing

The agent was executing the first step in a branch divergence analysis chain, which involved:

1. Finding the common ancestor commit between Forgejo and GitHub branches using `git merge-base`
2. Recording commit author and date
3. Capturing commit message/title
4. Saving data to a temporary state file for use by subsequent beads

The task was explicitly scoped as a **simple read-only git operation** - a single `git merge-base` command.

### 2. Crash Context

**The task WAS eventually completed.** Bead bf-6d3d6 is now **closed**, indicating that the work was successfully finished, either:
- By a retry agent that succeeded after the crash
- By a subsequent agent that took over the released bead

However, the crash itself represents another instance in the **cascading crash pattern** documented in other crash investigations.

### 3. Why It Crashed (Signal -1)

The agent crashed with **signal -1**, which indicates the process was terminated by an external signal. Based on the established pattern:

**Primary Causes:**
1. **Resource exhaustion from cascading crashes**: At the time of this crash, the local repository had accumulated hundreds of crash-recovery commits (now 741 commits ahead of origin). Even simple git operations on this bloated history consume significant resources.
2. **Timeout**: Git operations on large divergent histories can take extended time, potentially hitting system or agent timeouts.
3. **Memory pressure**: The agent environment may have hit memory limits during git operations.
4. **System resource contention**: Multiple concurrent agents may have been competing for system resources.

### 4. The "Cascading Crash" Pattern

This crash is part of a **systemic cascading crash scenario**:

1. An agent crashes while working on a task (like bf-6d3d6)
2. Recovery/cleanup agents spawn to investigate the crash  
3. Those recovery agents also crash due to the same resource constraints
4. Each crash generates a "crash recovery" commit updating `.needle-predispatch-sha`
5. This creates a feedback loop: more crashes → more commits → larger history → more crashes → more commits

**Evidence of the cascade in git history:**
```
db6cbbe docs: complete crash investigation for bead bf-574w1 signal -1
dd818f0 chore: update needle predispatch SHA after crash recovery for bf-4qxfs
3ea01d7 chore: update needle predispatch SHA after crash recovery for bf-687r6
458e0fb chore: finalize needle predispatch SHA after crash recovery for bf-687r6
fae4e34 chore: finalize needle predispatch SHA after crash recovery for bf-687r6
c195101 docs: complete crash investigation for bead bf-4k2ws signal -1
780b01f docs: complete crash investigation for bead bf-ncxbt signal -1
```

**Current repository state:**
- **741 commits ahead of origin** (and growing)
- Each crash investigation adds 1-3 commits to the history
- The problem is self-reinforcing and worsening over time

### 5. Systemic Issues

This crash reveals a fundamental architectural issue in the crash recovery workflow:

**The Crash Recovery Problem:**
- Crash recovery operations generate git commits
- These commits bloat the repository history
- Bloated history causes more crashes
- More crashes trigger more recovery operations
- The cycle repeats and worsens

**The Investigation Feedback Loop:**
- Each crash spawns investigation beads (like this one: bf-1936h)
- Investigation beads also crash under resource pressure
- Each investigation adds commits to document the crash
- This makes the underlying problem worse

## Current State Assessment

### Git Repository Status
- **Local**: 741 commits ahead of origin
- **Origin (Forgejo)**: At commit `61d27ac`  
- **GitHub mirror**: At commit `61d27ac`, in sync with Forgejo
- **No divergence**: The remotes remain in sync
- **Recovery commits**: The majority of the 741 commits are crash-recovery operations

### Bead Status
- `bf-6d3d6`: **Closed** (the crashed bead - work was eventually completed)
- `bf-1936h`: **In Progress** (this bead - crash alert investigation)

### Pattern Recognition

This crash (bf-6d3d6) is not an isolated incident. It follows a well-established pattern of crashes affecting:

- Investigation beads (bf-574w1, bf-4k2ws, bf-ncxbt)
- Recovery operations (bf-687r6, bf-4qxfs)  
- Simple git operations (bf-6d3d6 - just a `git merge-base` command)

**The fact that a simple git merge-base operation crashed indicates the resource pressure has become critical.**

## Recommendations

### Immediate Actions
1. **Acknowledge and document**: This crash is now documented for pattern analysis
2. **Close investigation bead**: Complete this investigation and move forward

### Systemic Changes Required
The cascading crash pattern cannot be resolved within the current workflow. It requires architectural changes:

1. **Move crash documentation out of git**: Crash investigations should be stored externally (e.g., a database, separate tracking system) to avoid bloating the repository
2. **Batch crash recovery commits**: Instead of one commit per crash, batch recovery operations
3. **Git history cleanup**: Once the systemic issue is fixed, perform a history cleanup to remove the accumulated crash-recovery commits
4. **Resource limits**: Implement proper resource limits and timeouts for agent operations
5. **Crash-resistant workflow**: Design crash recovery operations that don't rely on git commits for tracking

## Conclusion

Bead bf-6d3d6 crashed while performing a simple git merge-base operation due to resource exhaustion caused by the cascading crash pattern. The work was eventually completed (bead is closed), but this crash represents another data point in a systemic issue that requires architectural changes to resolve.

The crash itself is a symptom of a larger problem: the crash recovery workflow creates more crashes by bloating the git history, which creates a vicious cycle that will continue until the workflow is redesigned.

---

**Investigation completed**: 2026-08-16  
**Investigating agent**: claude-code-glm-4.7-lab-roam-1  
**Bead**: bf-1936h

---

## §2 Re-verification and Corrections (2026-09-07, bead domchk-9caed39b)

Re-dispatched on 2026-09-02 to "create" this document at its pre-consolidation
path, with premises taken from the alert corpus. The deliverable already
existed (committed 2026-08-16, moved to this canonical path the same day), and
several dispatch premises did not hold. Everything below was re-verified
first-hand on 2026-09-07; appended here per the corpus dedup-append convention
rather than forking a new file. Body sections above are preserved as the
historical 2026-08-16 record; §2.5 lists the claims they carry that are now
superseded.

### §2.1 Provenance of this document (and the path the dispatch asked for)

| Event | Commit | Date | Status in this clone |
|---|---|---|---|
| Investigation written | `12d3820` "docs: complete crash investigation for bead bf-6d3d6 signal -1" | 2026-08-16 13:02:20 −0400 | **Orphaned** — not an ancestor of HEAD |
| Content preserved | `c27899f` "chore: catch up lab work onto origin (squashed)" | 2026-08-16 18:20:34 −0400 | Ancestor of HEAD |
| Moved to this canonical path | `bb7455f` "docs: consolidate crash investigation documentation into organized directory structure" | 2026-08-16 21:18:54 −0400 | Ancestor of HEAD |

- The dispatch's requested path `docs/crash-investigation-bf-6d3d6.md` is the
  **pre-consolidation location**; it was deliberately moved under
  `docs/crash-investigations/` by `bb7455f`. Do not recreate it at the old
  path — that forks the corpus.
- The commit that "completed the investigation" is `12d3820` (orphaned by the
  same-day squash, content surviving via `c27899f`), **not** the `b6d1439`
  named by the dispatch — see §2.2.

### §2.2 Premise corrections (dispatch text vs. record)

| Dispatch premise | Verified finding (2026-09-07) |
|---|---|
| "File created at docs/crash-investigation-bf-6d3d6.md" | Already existed — see §2.1. No new file created. |
| "Document references commit b6d1439 where investigation was completed" | `b6d1439` is **not an object in this clone** (`git cat-file -t` fails). Independently searched and reported missing on 2026-08-26 in bf-14ydo's archived verification report (`docs/archive/crash-investigations/verification-report-bf-14ydo-…bf-6d3d6.md`). The real investigation commit is `12d3820` (§2.1). Fabricated SHA — consistent with the corpus-wide pattern of dispatch-template SHAs that never existed. |
| "Crash occurred during git merge-base operation" | The `git merge-base` **ran successfully in attempt 1**: the deliverable was committed at 11:16:27Z, **29 s before** attempt 1's kill at 11:16:56Z. All six kill instants (11:16:56Z–13:10:10Z) post-date the committed deliverable — post-deliverable kills, not a mid-operation death. |
| "741 crash-recovery commits" | Era measurement, valid at write time (2026-08-16, pre-squash) and now **unrecomputable**: the baseline `61d27ac` was destroyed the same day by the squash `c27899f`. Corroborated in magnitude by the surviving backup branch `pre-squash-history-20260816`: 722 commits, of which 180 (≈25%) carry crash-recovery / predispatch-SHA / crash-investigation subjects. The body's "majority of the 741" is not reproducible; ≥25% are explicitly labeled. |
| "Original bead bf-6d3d6 is now closed — work was eventually completed" | **Confirmed, with primary evidence** (§2.3, §2.4). |

### §2.3 Primary evidence: the full retry cycle

`~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2` records
seven dispatch attempts against bf-6d3d6 on 2026-08-13, all UTC:

| # | Claimed (claim_auto) | Outcome | Alert bead created |
|---|---|---|---|
| 1 | 11:14:14.892Z | **deliverable committed `e09cf84` 11:16:27Z** → exit −1 at 11:16:56.548Z | `bf-1qht8` (11:17:06Z) |
| 2 | 11:17:09.353Z | exit −1 at 11:20:54.151Z | `bf-1936h` (11:21:03Z; alert body timestamp 11:21:00.516Z — the instant this report documents) |
| 3 | 12:57:54.223Z | exit −1 at 13:02:11.273Z | `bf-w4fwe` |
| 4 | — | exit −1 at 13:06:16.782Z | `bf-2r30u` |
| 5 | — | exit −1 at 13:08:17.766Z | `bf-14ydo` (alert body 13:08:26.256Z) |
| 6 | — | exit −1 at 13:10:10.261Z | `bf-5npjj` |
| 7 | 13:10:24.500Z | **exit 0, `outcome=Success`** 13:12:47.884Z → `verify-changes.sh` gate passed → all validation gates passed → **"bead confirmed closed by agent"** 13:12:47.906Z | — |

Six kills, then success on the seventh attempt. The bead's own close:
`closed_at 2026-08-13T13:12:31.199Z`, close reason `Completed` — 1 h 59 m 41 s
after creation (11:12:50.305Z). Every `exit_code=-1` line is needle's
`outcome=Crash(-1)` sentinel, not a signal number.

This is the same orphaned-retry shape bf-4k2ws classified on the same day:
attempts 2–6 were re-running a bead whose deliverable was already on disk,
dying under the era's kill regime, and each death minted another alert bead.

### §2.4 The deliverable (evidence the work survived the crashes)

bf-6d3d6's acceptance criteria were: identify the Forgejo/GitHub common
ancestor via `git merge-base`, record author/date/title, save to a temp state
file for downstream beads. All three landed in attempt 1:

- Commit `e09cf84` (2026-08-13 11:16:27Z), "docs: record common ancestor
  commit for branch divergence analysis", added `docs/.branch-divergence-temp.json`.
  Content (byte-identical in the squash `c27899f`, so still retrievable from a
  surviving ancestor: `git show c27899f:docs/.branch-divergence-temp.json`;
  the working-tree copy was deleted 2026-09-01 by `079905a`):

  ```json
  {
    "common_ancestor": {
      "sha": "63ba02474c9b6bc339388adb3a44542e10755a10",
      "author": "jedarden",
      "email": "github@jedarden.com",
      "date": "Sun Aug 9 13:00:56 2026 -0400",
      "title": "fix: remove unused time import and update bootstrap test initialization"
    },
    "generated_at": "2026-08-13T10:12:00Z",
    "purpose": "Temporary state file for branch divergence analysis - common ancestor commit identification"
  }
  ```

- **SHA discrepancy, stated plainly:** the state file records
  `63ba0247…`, which is **not an object in this clone** (squash-orphaned or
  mistranscribed — unresolvable), while `e09cf84`'s own commit message names
  `00117cb879ecba7b1a819d80f1e4980ccb5d2881` for identical metadata — and that
  object **exists and is an ancestor of main today** (verified
  `merge-base --is-ancestor`). The identified *change* is pinned by its
  metadata either way; the surviving name is `00117cb8…`. Later era documents
  (including `docs/branch-divergence-analysis.md`'s historical section)
  repeat `63ba0247…` and inherit the same ambiguity.
- Downstream consumption succeeded: `510bf34` (2026-08-26, ancestor of HEAD)
  computed divergence statistics from this chain's state, and the chain's
  findings are consolidated in `docs/branch-divergence-analysis.md`.
- Attempts 2–7 therefore cost six kills and six alert beads and changed
  nothing about the outcome. **Work lost: none.**

### §2.5 Root cause — 2026-09-07 framing (supersedes body §3–§5)

What the body got right: the crash is real, exit −1 is a kill-class sentinel,
the era is the repository-bloat kill regime, and the investigation/recovery
workflow did keep minting commits and alert beads as it churned.

What is superseded:

1. **Commit count was not the resource driver — object size was.** The body's
   §4 causal loop (741 commits → bloated history → more crashes) misattributes
   the mechanism. The canonical determinations for this era
   ([bf-4k2ws](bf-4k2ws-crash-investigation.md),
   [bf-1s6c3, 2026-09-06](../crash-analysis-bf-1s6c3-2026-09-06.md),
   bf-1ea4g re-determination domchk-c2b8c832) establish: ~17–18 GB of loose
   objects from **17+ identical 237 MB `.beads/*.jsonl` snapshots** committed
   to git, with a large unpushed backlog making `git push`/gc pack-objects the
   kill site. The unpushed backlog (660 on 08-12 → 422 on 08-13 → 741 by
   08-16) was an **amplifier** of pack-objects cost, not itself the memory
   consumer.
2. **The backlog was resolved, not "worsening".** The body's "self-reinforcing
   and worsening over time" state ended the day this document was written:
   the 2026-08-16 squash (`c27899f`) folded the backlog. Live on 2026-09-07:
   `origin/main..HEAD` = 0, `HEAD..origin/main` = 0,
   `git merge-base origin/main github-mirror/main` = `eb717dfe…` with both
   refs at tip. The bloat kill regime has not recurred in live agent work
   since the fix stack landed (bf-4k2ws §16 verification).
3. **The per-instant killer for these six kills is unknowable.** No Aug-13
   kernel record survives — system journald holds a single boot from
   2026-08-15 19:56 EDT, and the box rebooted twice on Aug-14. Era
   classification INFRASTRUCTURE / repo-bloat is HIGH confidence; the
   specific memcg-OOM attribution for any single Aug-13 kill is not provable.
4. **`.needle-predispatch-sha` commits are dispatch bookkeeping**, not crash
   evidence; the body's §4 evidence block cites seven SHAs that were valid at
   write time (pre-squash) and were orphaned by `c27899f` five hours later.
   The same subjects survive under different SHAs on the pre-squash backup
   branch and in the current lineage (e.g. "docs: complete crash investigation
   for bead bf-574w1 signal -1", "chore: update needle predispatch SHA after
   crash recovery for bf-4qxfs").

### §2.6 Impact assessment

- **Target work:** completed in attempt 1 (§2.4); zero work lost across six
  kills; bead closed `Completed` at 13:12:31Z the same day.
- **Alert-layer cost (the real residue):** six alert beads for one bead's six
  kills (pre-0.4.2 one-alert-per-kill). Four closed by 2026-08-26. **Two
  remain open as of 2026-09-07 — `bf-14ydo` (despite its own committed
  FALSE_POSITIVE verification report) and `bf-5npjj`.**
- **Downstream waste continued past the crash layer's fix:** this very bead
  (domchk-9caed39b, created 2026-09-02) was dispatched to re-create existing
  documentation on the strength of a fabricated SHA (`b6d1439`) and a stale
  path. The alert layer was still regenerating stale premises five weeks
  after the kill regime itself was repaired — the same dispatch-template
  stale-premise vector documented in bf-4k2ws §14.6.
- **Era context, same day:** bf-4k2ws absorbed 55 kills and bf-1ea4g 56 on
  2026-08-13; bf-6d3d6's six are a small slice of the same storm.

### §2.7 Method (reproducible)

```
git cat-file -t b6d1439                                   # fabricated SHA check
git log --all --oneline -- docs/crash-investigation-bf-6d3d6.md
git merge-base --is-ancestor <sha> HEAD                   # per-SHA survival
git show c27899f:docs/.branch-divergence-temp.json        # deliverable recovery
git log --format=%s pre-squash-history-20260816 | grep -icE 'predispatch|crash recover|crash investigation|crash alert|signal -1'
grep 'bf-6d3d6' ~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2   # retry cycle
python3 — filter .beads/checkpoint/forensic.jsonl for close_reason/closed_at
bead show bf-6d3d6 / bf-14ydo / bf-5npjj                  # current states
git rev-list --count origin/main..HEAD; git rev-list --count HEAD..origin/main
```

Snapshot at write time (2026-09-07): 267 loose objects / 2.71 MiB, 1 pack
99.11 MiB, `.git` 105 MB, tip `8b21853`.

---

**Re-verification completed**: 2026-09-07  
**Investigating bead**: domchk-9caed39b (claude-code-glm-5.3-flash-lab-roam-10)

---

## §3 Systemic analysis: the feedback loops, the failure points, and why recovery did not self-stabilize (2026-09-07, bead domchk-c382e224)

Dispatched to analyze the bf-6d3d6 crash patterns and identify the systemic
issues in the crash-recovery workflow, on the premise that "741
crash-recovery commits accumulating … created resource exhaustion." Body §4
sketched the loop and §2.5 corrected its causal weight, but neither renders
this dispatch's specific asks — named failure points, an explanation of the
missing self-stabilization, and process failures enumerated for correction —
so they are consolidated here per the dedup-append convention. Every figure
below was re-verified first-hand on 2026-09-07 (§3.5); the repo-state snapshot
is this append's own run, not §2.7's.

### §3.1 The feedback loop, stated precisely (and what actually caused the 741)

Two coupled loops ran, and they are worth separating because the body's §4
fused them:

**Loop A — the kill loop (infrastructural, not self-sustaining).** ~17–18 GB
of loose objects (17+ identical 237 MB `.beads/*.jsonl` snapshots) made
pack-objects the expensive step of every `git push` and gc; running inside a
12 GiB per-dispatch memcg scope, those operations were killed
(`CONSTRAINT_MEMCG`, needle's `exit_code=-1` sentinel). Any dispatch that
touched the repo could die. bf-6d3d6's own six kills are the era regime
acting on a bead whose real work was one `git merge-base` (§2.4: the
merge-base ran; zero work was ever lost).

**Loop B — the bookkeeping loop (the one that minted the commits).** Each
kill, under pre-0.4.2 one-alert-per-kill, minted an alert bead; each alert
and each retry re-dispatch spawned investigation work; and the workflow's
standing convention was to record dispatch bookkeeping and investigation
output as commits to the shared repo — `.needle-predispatch-sha` updates,
"docs: complete crash investigation …" reports, alert verification reports.
So kills produced commits, and commits (via the unpushed backlog they
extended) raised the pack-objects cost that produced kills. That is the
feedback loop, and it is real.

**The correction that matters for the causal story:** the 741 commits did not
*cause* the resource exhaustion — object size did; the commit backlog
*amplified* the cost of the push/gc kill site (§2.5.1). And the commits were
overwhelmingly **bookkeeping and documentation, not repair**: §2.4 shows
nothing needed repairing across all six kills (zero work lost), so the
"crash-recovery commit" population is dispatch-state churn plus write-ups,
not damage repair. The 741 figure itself stays an unrecomputable era
measurement (§2.2); its surviving corroboration is the pre-squash backup
branch — re-verified today: **722 commits, 180 (≈25%) crash-labeled
subjects**.

### §3.2 Specific workflow failure points in crash recovery

Mapped to bf-6d3d6's own retry cycle (§2.3), each point is a place the
workflow could have stopped and did not:

| # | Failure point | bf-6d3d6 instance | Evidence |
|---|---|---|---|
| F-1 | **Kill detection with no work-completion check.** A kill raised an alert without asking whether the deliverable had already landed. | Attempt 1 committed the deliverable 29 s *before* its kill; all six kills were post-deliverable, yet each minted an alert. | §2.3; needle log: 6 × `exit_code=-1 outcome=Crash(-1)` + 1 × `exit_code=0 outcome=Success` (re-verified 2026-09-07) |
| F-2 | **Retry with no stop-condition.** Re-dispatch did not check whether the bead's deliverable already existed on disk. | Attempts 2–6 re-ran a completed bead — six kills, six alert beads, and no change to the outcome. | §2.3, §2.4 ("Work lost: none") |
| F-3 | **One alert bead per kill.** The alert layer's cardinality equalled the kill count, so the loop's output grew with its input. | 6 kills → 6 alert beads (`bf-1qht8`, `bf-1936h`, `bf-w4fwe`, `bf-2r30u`, `bf-14ydo`, `bf-5npjj`). | §2.3, §2.6 |
| F-4 | **Per-dispatch state recorded as shared-main commits.** `.needle-predispatch-sha` churn put a commit on main per dispatch. | 735 commits in the current lineage touch the file; 604 carry a `predispatch` subject, with `crash investigation` (117) and `crash recover` (25) subjects overlapping the same population. | `git log -- .needle-predispatch-sha` (§3.5) |
| F-5 | **Alert premises regenerated from templates, not from state.** Alert/dispatch text carried era figures and SHAs verbatim, unvalidated against the repo. | This bead family's templates named fabricated SHA `b6d1439` (§2.2) and restated "741 crash-recovery commits" five weeks after the regime was repaired — including in this dispatch. | §2.2, §2.6; bf-4k2ws §14.6 |

### §3.3 Why the crash-recovery system did not self-stabilize

A self-stabilizing loop needs negative feedback: some signal whose magnitude
grows with the damage and which acts to reduce the loop's drive. The workflow
had none — and, worse, its only couplings were positive:

1. **Nothing measured the accumulating state.** There was no commit-ahead
   counter (M-1, 🔴 open in the bf-1ea4g determination), no dispatch-scope
   memory telemetry (M-2, 🔴 open), no work-completion check at alert time
   (G-9 — an external ask, later answered at the alert layer by the
   2026-09-02 fix stack). A loop whose participants cannot see its output
   cannot damp it.
2. **Retry and alert behavior scaled with failures, not with progress.** A
   kill produced a retry and an alert *regardless* of what the killed attempt
   had achieved (F-1, F-2, F-3). Progress — a committed deliverable —
   produced no signal at all.
3. **The loop's waste was denominated in the same currency as its cause.**
   Bookkeeping commits extended the unpushed backlog, which is exactly the
   quantity that made the kill site expensive. So Loop B fed Loop A, and
   Loop A's kills fed Loop B.

**The arc, measured** — commits per day touching `.needle-predispatch-sha`
in the current lineage (dedicated bookkeeping commits; the final two are the
2026-09-07 empty-tree accident `2e8ce7a` and its repair `2ec91ec`, incidental
touches):

| Day | 08-09 | 08-16 | 08-17 | 08-25 | 08-26 | 09-01 | 09-02 | 09-07 |
|---|---|---|---|---|---|---|---|---|
| Commits | 1 | 53 | 262 | 155 | 233 | 21 | 8 | 2 |

Two facts in this table settle the question. First, the churn **grew for two
weeks** (1 → 53 → 262): between bf-6d3d6's six kills on 08-13 and the 08-16
squash the loop ran on undamped, and the backlog itself grew (422 → 741, §2.5.2).
Second, the 08-17 wave is the **largest single day, the day after the squash**
folded the backlog — removing the amplifier alone did not stop the loop,
because the loop's drive was the alert/retry behavior, not the backlog. Decay
begins only with the fix stack landing externally: needle 0.4.2
(`needle-stable.pre-0.4.2-20260819` backup name) introduced alert dedup, and
the fuller decay to zero tracks needle 0.6.0 (built 2026-09-01) plus the
2026-09-02 crash-alert fix stack.

**What actually ended it** — six external interventions, each cutting one leg
of the loop, none generated by the loop itself:

| Intervention | Date | Leg cut |
|---|---|---|
| Squash `c27899f` folds the 741-commit backlog | 2026-08-16 | Loop A's amplifier |
| `.beads/` gitignored (repo-wide `*.jsonl`/`*.db`) + 10 MB pre-commit gate | 2026-09-01 era, hook G-1 closed 09-06 | The resource driver itself (object size) |
| `pack.windowMemory=2g` / `deltaCacheSize=1g` / `threads=1`, repo + global | 2026-09-02 | The kill site (unbounded pack-objects) |
| needle ≥0.4.2 alert dedup (end of one-alert-per-kill) | ~2026-08-19 | F-3 |
| `crash-alert-manager.sh` 6-fix stack (closed-bead filter, dedup + processed-alert tracking, completion awareness, cooldown, classification) | 2026-09-02 | F-1, F-3, part of F-5 |
| `verify-work-completion.sh` pre-close gate writing `.beads/state/work-completion/` markers | 2026-09 | F-1, F-2 (triage side) |

### §3.4 Process failures that need correction

Five, with fix status as of 2026-09-07 — the first three are the minimum the
dispatch asks for, and PF-3 is the one this analysis adds that no prior
section of the corpus records:

| ID | Process failure | Status 2026-09-07 |
|---|---|---|
| **PF-1** | **Alert cardinality equalled kill cardinality** (F-3): one bead per kill, so a single bead's bad afternoon minted six investigations of already-finished work. | ✅ Fixed at source (needle ≥0.4.2 dedup) and at response layer (2026-09-02 stack, `crash-alert-manager.sh` carries the six FIX markers; suite 12/12 per CLAUDE.md 2026-09-06). Residue: `bf-14ydo` and `bf-5npjj` still **Open** — re-verified today — 25 days after the target closed `Completed`. |
| **PF-2** | **Retry without a deliverable/stop-condition** (F-2): re-dispatch never asked whether the work was already on disk, so it paid full kill exposure to reproduce a finished deliverable. | 🔴 **Open** — bf-1ea4g's H-1 retry stop-condition remains unimplemented (bf-1ea4g root-cause determination, recommendations table). `verify-work-completion.sh` covers the triage side only, not the retry decision. |
| **PF-3** | **Dispatch bookkeeping committed to the shared repo** (F-4): `.needle-predispatch-sha` is a per-dispatch HEAD marker (never crash evidence) written as a commit on `main`, contributing 604 predispatch-subject commits to the current lineage and lengthening the backlog the kill site choked on. | 🔴 **Open and still live** — the file is **tracked and not gitignored** today (`git ls-files` hit; no `.gitignore` rule), and every dispatch still dirties it. `.beads/` was gitignored for exactly this reason; this file is the remaining un-gitignored member of the same class. |
| **PF-4** | **Alert/dispatch premises unvalidated against repo state** (F-5): era figures and SHAs propagated verbatim into new work items, dispatching agents to re-create existing deliverables (this bead family: fabricated `b6d1439`, stale pre-consolidation path, stale 741 premise). | 🟡 Partial — the vector is documented (bf-4k2ws §14.6) and the dedup-append convention is the working mitigation, but no mechanical premise validation exists; this dispatch is a live instance (it restates the 741 premise as fact — corrected here and in §2.5). |
| **PF-5** | **No feedback from the loop to its operators** — no commit-ahead counter, no dispatch-scope telemetry, so nothing could observe the churn growing (M-1, M-2 🔴 open; G-10 external). | 🔴 Open — M-1 (`rev-list --count @{upstream}..HEAD` warn ≥50 / critical ≥200 in `check-repo-health.sh`) remains the highest-leverage single detection rule and is unimplemented. |

The systemic diagnosis in one sentence: **the crash-recovery workflow was a
positive-feedback amplifier bolted onto an infrastructural kill regime, with
no measurement of its own accumulation and no stop-condition on any of its
edges — so it ran until each leg was cut externally, and the two legs still
uncut (PF-2, PF-3) remain live today.**

### §3.5 Method (reproducible, this section's own run 2026-09-07)

```
git rev-list --count origin/main..HEAD; git rev-list --count HEAD..origin/main   # 0 / 0
git rev-list --count pre-squash-history-20260816                                 # 722
git log --format=%s pre-squash-history-20260816 | grep -icE 'predispatch|crash recover|crash investigation|crash alert|signal -1'   # 180
git ls-files .needle-predispatch-sha; grep -c predispatch .gitignore             # tracked / no rule
git log --oneline -- .needle-predispatch-sha | wc -l                             # 735
git log --format=%s -- .needle-predispatch-sha | grep -icE 'predispatch'          # 604
git log --format=%ad --date=short -- .needle-predispatch-sha | sort | uniq -c    # per-day arc (§3.3)
grep 'bf-6d3d6' ~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2 | grep -o 'exit_code[^,]*' | sort | uniq -c
bead show bf-14ydo bf-5npjj                                                      # both Open
```

Snapshot at write time: 327 loose objects / 3.16 MiB, `.git` 106 MB,
divergence 0/0. Cross-references: gaps and statuses from
`docs/crash-prevention-requirements.md` (G-1 closed 2026-09-06, G-2 withdrawn
2026-09-07, G-9/G-10 open external asks); H-1/M-1/M-2 from
`docs/investigations/bf-1ea4g-root-cause-determination-2026-09-02.md`.

---

**Systemic analysis completed**: 2026-09-07  
**Investigating bead**: domchk-c382e224 (claude-code-glm-5.3-flash-lab-domain-check)
