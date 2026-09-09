# Agent Crash — bf-2vtzg: what it was doing, the crash scenario, and the conditions that produced it

**Dispatch:** `domchk-aa4817c9` (2026-09-09) — documentation deliverable of an alert-layer re-fire.
**Target bead:** `bf-2vtzg` — "Document remote Forgejo origin state" (task, P2, created 2026-08-13T07:14:57.322Z, **closed 2026-08-13T09:42:58.663Z, revision 1, never reopened**).
**Standing:** the target work shipped the same day the crash happened. This record documents the crash for the research corpus; nothing is open against bf-2vtzg. It is **subordinate to the canon** — `docs/crash-diagnostics-bf-2vtzg-2026-09-07.md` (§1–§12, the five-stage investigation chain `domchk-c45df846` → `domchk-0befc321` → `domchk-80860fb2` → `domchk-f27cf324` → `domchk-1ec90d5e`) — and makes **no new cause claim**. Where the raw alert text carries stale premises, they are corrected inline (§2.1); where the canon's load-bearing figures are reused, they were re-verified first-hand this session (§6).

## 1. What bf-2vtzg was attempting to do

From the bead's own description (re-read live from the store this session):

- **Step 2 of the branch-divergence analysis chain**: capture the current state of the Forgejo `origin` remote, as the second half of a local-vs-remote divergence snapshot. Its dependency `bf-1ea4g` (local state documentation) blocked it; that bead closed 09:10:16.731Z and bf-2vtzg's first claim came 28.5 s later.
- **Acceptance criteria:** document the remote `origin/main` commit SHA, the branch-tip message and author, the commit timestamp, and the fetch URL; append the data to a temporary state file for later analysis.
- **Explicit scope limit:** *"This bead ONLY reads and documents the Forgejo remote state. It does NOT touch GitHub or perform any comparisons."* The work is read-only git/remote inspection (`ls-remote` / `fetch` / `rev-list` against Forgejo) plus a small doc + JSON write. Nothing in the task touches domain-check code.
- **What it produced:** `forgejo_remote_state_bf-2vtzg.json` (blob `9cf6ee23`, repo root) and `forgejo-origin-state-bf-2vtzg.md` (blob `01bac556`, now at `docs/archive/crash-investigations/` after the `a883044` archive move). Both verified on `origin/main` this session by `hash-object` against the cited hashes.

The task itself is small and safe. It crashed because of *where and when* it ran, not what it did (§3).

## 2. The exact crash scenario

### 2.1 What "exit code -1 (signal -1)" actually means — two template premises corrected

The alert beads for this crash (e.g. `bf-37jbh`, `bf-39xem`) carry the template line `- **Exit code**: -1 (signal -1)`. Both halves need correcting before the scenario is meaningful:

1. **`exit_code = -1` is needle's abnormal-child-death sentinel, not a signal number.** No signal has value −1, and the worker log never records which signal fired — the agent process simply never returned an exit status. The underlying mechanism is the kernel's **uncatchable SIGKILL**, chain-inferred to be a **memcg-OOM kill** inside the dispatch scope (§3.2). `exit −1` is the *shape* of an external kill; the *mechanism* comes from the sibling evidence, not from the number.
2. **The alert's "Timestamp:" is not the kill instant.** Alert-bead timestamps are `heartbeat.emitted` stamps written seconds *after* the kill, when the alert bead is created. Verified example from this very crash: the archived FP report's "crash timestamp" `09:35:19.810714905Z` is a heartbeat emitted **6.31 s after** attempt 7's kill at `09:35:13.495776901Z` (re-measured from the primary log: 6.3146 s). The actual kill instants are the `agent.completed` stamps in §2.2.

### 2.2 The crash itself: a 10-attempt kill loop, 09:10:45–09:43:26 UTC on 2026-08-13

One worker (`claude-code-glm-4.7-lab-domain-check`, session `8446529e`) claimed the bead ten times. Re-extracted first-hand from the primary worker log this session (`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl`; figures match canon §3 to within rounding):

| # | Dispatched (Z) | Died / exited (Z) | exit_code | Lived | Notes |
|---|---|---|---|---|---|
| 1 | 09:10:45.244 | 09:13:17.645 | −1 | 152.4 s | committed `97576e0` ~24 s before death |
| 2 | 09:13:30.330 | 09:17:40.929 | −1 | 250.6 s | committed `4fbebd0` ~56 s before death |
| 3 | 09:17:57.074 | 09:22:11.662 | −1 | 254.6 s | committed `ad88d53` ~27 s before death |
| 4 | 09:22:21.206 | 09:25:05.670 | −1 | 164.5 s | committed `506cffb` ~26 s before death |
| 5 | 09:25:15.379 | 09:29:19.432 | −1 | 244.1 s | committed `1ec2354` ~22 s before death |
| 6 | 09:29:28.784 | 09:32:42.019 | −1 | 193.2 s | **the only kill with no commit** |
| 7 | 09:32:51.376 | 09:35:13.496 | −1 | 142.1 s | committed `bb4c14b` ~25 s before death |
| 8 | 09:35:23.933 | 09:36:51.931 | −1 | 88.0 s | committed `7b596cf` ~25 s before death |
| 9 | 09:37:01.029 | 09:40:22.180 | −1 | 201.2 s | committed `59f89f5` ~27 s before death |
| 10 | 09:40:31.518 | 09:43:23.562 | **0** | 172.0 s | success — closed the bead in-task |

Every one of the nine kills followed the identical event chain in the log:

```
agent.completed  {exit_code: -1}
→ outcome.classified {exit_code: -1, outcome: "crash"}
→ bead.released            (released for retry)
→ outcome.handled  {action: "alerted"}
```

That last step is the pre-0.4.2 **one-alert-per-kill** behavior that seeded this crash's ~12-bead alert family — nine alerts for one target, every one of which investigated work the retry loop itself had already finished.

Attempt 10 — dispatched 9 s after its predecessor's kill, into an **unchanged repository** — ran 172 s, exited 0, recorded `verification.passed {gates_run: 1}`, closed the bead in-task at 09:42:58.663Z (`close_reason "Completed"`), and its `outcome.handled` recorded `action: "none"`.

### 2.3 Where the deaths sat in each attempt

Eight of the nine killed attempts had **already finished the task's substance and committed it** 22–56 s before they died. All eight commits (`97576e0`, `4fbebd0`, `ad88d53`, `506cffb`, `1ec2354`, `bb4c14b`, `7b596cf`, `59f89f5`) are reachable **only** from the local-only branch `pre-squash-history-20260816` — verified this session that none is on `origin/main`. The deaths therefore land in **follow-on git work after the commit** (status/fetch/push against the bloated object store), exactly where the sibling mechanisms live. The task was never the thing failing; git operations over the bloated store were.

Two consequences worth stating precisely:

- **Attempt 9's write is the shipped deliverable.** `59f89f5`'s `docs/forgejo-origin-state-bf-2vtzg.md` is byte-identical to `origin/main`'s blob today (`01bac556…`; verified by `hash-object` this session). Attempt 10 committed nothing — so what the later squash `c27899f` carried to origin is the *last killed attempt's* bytes, and attempt 10's verified close validated **attempt 9's** work, 3.5 minutes after attempt 9 died. (This corrects the collector package's earlier "22 minutes / attempt 3" statement — canon §11.2.)
- **The "503 commits ahead" figure in the deliverable is a loop artifact**, not a standing condition: each attempt re-measured a local tip that its predecessors' deaths had advanced (attempt 5's own commit message says "500 commits ahead"). Divergence is 0/0 today.

### 2.4 Reproduction status — this crash is not input-reproducible

The dispatch asks for reproduction of the crash conditions. The honest statement: **the crash is not deterministically reproducible from the task input, and that non-determinism is itself diagnostic.** Ten byte-identical dispatches ran against an unchanged tree; nine died at uncorrelated lifetimes (88–255 s, no cap-adjacent clustering against the 600 s dispatch cap) and one survived. A code defect fails the same input the same way every time; shared-resource contention kills probabilistically. Reproducing this crash requires reconstructing the 2026-08-13 environmental conjunction (§3) — an ~18 GB object store inside a 12 GiB cgroup — not re-running the bead. Re-running the bead today, in the repaired repo, succeeds (that is precisely what attempt 10 did).

## 3. Conditions that led to the crash

Six conditions, in causal order. (1)–(3) are the mechanism; (4)–(6) are the amplifiers that turned one kill into a nine-kill loop plus an alert family.

1. **The bloat-era object store.** At crash time the repository was ≈18 GB `.git` with ≈17 GB of loose objects — 17+ identical 237 MB `.beads/*.jsonl` snapshots committed to history (canon-sourced from the cleanup record; not re-measurable now, since repaired). Every significant git operation over that store was memory-hungry. bf-2vtzg sat squarely in the bloat era (2026-08-12 → 09-01), and its git-remote-heavy task class is exactly the operation class the era turned into kills.
2. **The 12 GiB dispatch-scope ceiling.** Each attempt ran inside a per-dispatch systemd scope with `MemoryMax` = 12 GiB. Re-verified live this session from the writer's own in-flight dispatch scope: `systemctl --user show <scope> -p MemoryMax` → `12884901888` (= 12 GiB exactly; parent `needle.slice` = 32 GiB). This was the only binding ceiling between attempt and host — the host was never out of memory; the cgroup was. When git's pack-objects exceeded 12 GiB inside the scope, the kernel's `CONSTRAINT_MEMCG` OOM killer delivered SIGKILL.
3. **Unbounded pack-objects.** No `pack.windowMemory` bound existed on 2026-08-13; the `pack.windowMemory=2g` / `pack.threads=1` / `pack.deltaCacheSize=1g` config that caps a gc or push at ≈3 GiB worst-case was applied 2026-09-02, three weeks later.
4. **A fleet-wide kill regime.** This loop was 9 of **344** exit −1 kills on this one worker on 2026-08-13 (re-counted first-hand from the primary log: bf-65lsdu 127, bf-1ea4g 56, bf-4k2ws 55, bf-2ildm 38, bf-1s6c3 22, bf-ncxbt 11, **bf-2vtzg 9**, bf-mje3pd 7, …). The window is bracketed by the same mechanism: bf-1ea4g — this bead's own dependency — died ×6 in the 33 minutes before bf-2vtzg's first claim, and bf-ncxbt, *an alert bead for this very crash*, died ×4 to the same mechanism afterward. CPU was saturated throughout the window (51 `fleet.cpu_saturated` samples 09:00–10:00Z, load 7.23–16.52 on 9 cores — canon §2, not re-derived this session).
5. **The retry loop had no stop condition.** Nine re-dispatches re-did identical work with no retry cap that fired (H-1) and no commit-ahead counter to notice the loop was advancing a local tip without ever pushing (M-1). The loop ended only because attempt 10 happened to survive — nothing about the repository changed between kill 9 and success.
6. **Evidence constraint on the mechanism claim.** No Aug-13 kernel record can exist (the single journald boot begins 2026-08-15 19:56:33 EDT; no telemetry predates 2026-09-01), so the memcg-OOM mechanism is **chain-inferred via kernel-proven same-day, same-worker, same-operation-class siblings** — bf-4k2ws (55 `CONSTRAINT_MEMCG` git kills), bf-1ea4g (push-side), bf-4x12ec (gc-side) — rather than kernel-proven for this bead. This is the same evidentiary posture the bf-4k2ws determination took for the same day. Confidence: classification INFRASTRUCTURE / no code defect **HIGH**; mechanism **MEDIUM-HIGH** (bounded by evidence non-existence, not by contradiction).

## 4. What the crash did not involve

- **No domain-check code.** The task is read-only git/remote documentation; the bead's own scope note excludes anything else. No defect was found in any of the corpus's 157+ investigations, and no artifact of this loop contains a panic, stack trace, or application error — SIGKILL leaves none. This is consistent with the repo's standing "2% code defects, none found here" classification.
- **Not a service failure.** All nine kills are exit −1; no 503/502-shaped completion exists anywhere in the loop, which excludes the service class on exit-code shape alone.
- **Not a workflow/max-turns failure.** No max-turns signature exists in any attempt; lifetimes are far from the dispatch cap.
- **Not a timeout.** 88–255 s against a 600 s cap.

## 5. Resolution state and recurrence status

- **Target:** closed by its own attempt 10 at 2026-08-13T09:42:58.663Z, `close_reason "Completed"`, revision 1 — re-verified live from the store *and* `forensic.jsonl` this session; never reopened in the 27 days since.
- **Deliverable:** both blobs on `origin/main` (`9cf6ee23`, `01bac556`), content correct; the crash cost the loop nothing but time and the alert family it seeded.
- **Recurrence:** the precondition is mechanically closed. `.beads/` fully gitignored (0 tracked files), the 10 MB pre-commit gate installed, pack-objects bounded on both the gc and push paths (≈3072 MiB worst case, repo-local and global — canon §11.5/§12.2 re-verified 2026-09-07; `check-repo-health.sh` exit 0 and 0/0 divergence re-run this session). The bloat cannot regrow through bead state, and the alert layer that kept this crash alive in the queue for weeks is fixed and tested — `test-closed-bead-filter.sh`'s sandbox case **is this very bead**: a fabricated bf-2vtzg trace produces no alert.
- **Stale-premise correction:** sibling alert `bf-39xem`'s August note reads *"Repository bloat issue remains UNRESOLVED"* with recommendations (gitignore, aggressive gc, pre-commit hooks) — **every one of those was subsequently implemented**, and the repo was packed 18 GB → ~94–107 MB. That note's live-state claims are superseded; the archived FP report's two timestamp premises are corrected in §2.1 (freeze policy: corrected here and in the canon, not in the archived files).

## 6. Verification record — what was re-run first-hand this session (2026-09-09)

| Claim | How verified | Result |
|---|---|---|
| Target state | `bead show bf-2vtzg` | Closed, rev 1, created 07:14:57.322Z, closed 09:42:58.663Z, `bf-1ea4g` blocks it |
| 10-attempt loop, exit codes, lifetimes | re-extraction from `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` (187 bf-2vtzg events) | 10 claims / 10 dispatches / 10 completions; 9 × exit −1 + 1 × exit 0; lifetimes 88.0–254.6 s — matches canon §3 |
| Per-kill event chain | `outcome.classified` / `outcome.handled` payloads | 9 × `{exit_code:-1, outcome:"crash"}` → `action:"alerted"`; attempt 10 `{exit_code:0, outcome:"success"}` → `action:"none"` + `verification.passed` |
| Heartbeat, not kill instant | nearest `heartbeat.emitted` after attempt 7's kill | `09:35:19.810708191Z` = 6.3146 s after `09:35:13.495776901Z` |
| Deliverable on origin/main | `git cat-file -e` + `hash-object` | json `9cf6ee23…` ✅, md `01bac556…` ✅, archived FP report ✅ |
| Attempt 9 byte-identity | `git show 59f89f5:<path> \| git hash-object --stdin` | `01bac556…` — identical to origin/main's blob |
| Loop commits not on main | `git branch --contains` + `merge-base --is-ancestor` ×8 | all 8 reachable only from `pre-squash-history-20260816` |
| Day-wide kill regime | re-count of all `outcome.classified exit_code:-1` in the day's log | 344 kills; bf-2vtzg = 9 |
| 12 GiB dispatch scope | `systemctl --user show <own in-flight scope> -p MemoryMax` | `12884901888` = 12 GiB exactly (needle.slice parent 32 GiB) |
| Repo health today | `./scripts/check-repo-health.sh`; divergence | exit 0; `origin/main…HEAD` 0/0 |

## 7. Related artifacts

| Artifact | Where |
|---|---|
| Canon investigation (§1–§12: collection, classification, RCA, fix verification, resolution verification) | `docs/crash-diagnostics-bf-2vtzg-2026-09-07.md` |
| Target deliverables | `forgejo_remote_state_bf-2vtzg.json` (root); `docs/archive/crash-investigations/forgejo-origin-state-bf-2vtzg.md` |
| Archived FP report (premises corrected in §2.1 above) | `docs/archive/crash-investigations/crash-investigation-bf-2vtzg-false-positive-2026-09-02.md` |
| Primary worker log | `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` (12,131 events; 187 for bf-2vtzg) |
| Raw per-event extract (collector's) | `.beads/state/domchk-0befc321/bf-2vtzg-events-domain-check-2026-08-13.jsonl` (gitignored) |
| Loop branch (8 killed attempts' commits) | local-only `pre-squash-history-20260816` |
| Alert family | umbrella `bf-39xem` + siblings bf-37jbh, bf-3uawn, bf-4fvi9h, bf-4nyp7, bf-58j3z, bf-5ami9, bf-5o8ey, bf-ive13, bf-ncxbt, bf-xg2gg |
| Mechanism canon (kernel-proven siblings) | bf-4k2ws (55 memcg kills), bf-1ea4g (`docs/crash-inventory-bf-1ea4g-summary.md`), bf-4x12ec (gc-side), bf-198ne (push variant) |

**Bottom line:** bf-2vtzg was a small, read-only Forgejo-state documentation task that ran into the bloat-era repository and the 12 GiB dispatch scope at the worst hour of the fleet's worst day. Nine identical dispatches died mid-run in post-commit git work over an ~18 GB object store — `exit −1` being needle's sentinel for the kernel's uncatchable memcg-OOM SIGKILL — and the tenth, into an unchanged repo, succeeded and closed the bead. Every condition that produced the kills has since been mechanically removed, the deliverable shipped, and the alert surface it seeded is tested closed.

## 8. Root cause determination (`domchk-cf22b1af`, 2026-09-09)

Renders this chain's second dispatch — *"Identify the root cause of the signal −1 crash"* — whose four acceptance criteria are phrased in signal terms. The verdict is the canon's (§11.1), restated at the precision the signal phrasing demands; **no new cause claim**. Every load-bearing figure below was re-verified first-hand this session (§8.5).

### 8.1 The specific signal: SIGKILL, delivered by the kernel's memcg OOM killer

**SIGKILL — signal 9 — kernel-issued, uncatchable, therefore never observable by the dying process or its harness.** The alert template's `signal -1` is a rendering of needle's sentinel, not a signal identifier:

- No signal has value −1. `exit_code = -1` is needle's abnormal-child-death sentinel, emitted because the agent process never returned an exit status (§2.1). The alert beads (e.g. `bf-37jbh`) carry the line `**Exit code**: -1 (signal -1)` verbatim — re-read from the store this session.
- The primary log records **no signal at all**: `grep -ci '"signal'` across all 12,131 events of the day's worker log returns **0** (re-verified this session). The harness never observed which signal fired — with SIGKILL it cannot.
- The signal is therefore identified **by mechanism, not by number**: the kernel's memory-cgroup OOM killer. The kernel's own record of the identical kill shape, from the journal-covered push-storm era: `Memory cgroup out of memory: Killed process 3322486 (git) total-vm:13847248kB, anon-rss:12301364kB, … oom_score_adj:200` with `oom-kill:constraint=CONSTRAINT_MEMCG`, inside `run-p*.scope` (first of **540** surviving records, 2026-08-16 00:27:35 EDT).

### 8.2 The code path / condition that triggered it

**No domain-check code is in the path** (§4). The triggering condition chain, in code-path terms — each link verified first-hand this session:

```
git (post-commit follow-on work: status/fetch/push)          ← 8 of 9 kills landed 22–56 s after committing the deliverable (§11.2)
  → git pack-objects, UNBOUNDED                              ← no pack.windowMemory on 2026-08-13; bound applied 2026-09-02 (§3 item 3)
    → over the bloat-era object store (~18 GB .git, ~17 GB loose, §3 item 1)
      → RSS exceeds the dispatch scope ceiling               ← run-p*.scope MemoryMax = 12 GiB (re-read live: 12884901888)
        → kernel CONSTRAINT_MEMCG OOM kill → SIGKILL         ← kernel-proven shape: git at anon-rss 12.30 GB pinned at the cap, oom_score_adj 200
          → process dies with no exit status
            → needle sentinel exit_code = -1                 ← agent.completed {exit_code: -1} (re-extracted, all 10 attempts)
              → alert template renders "signal -1"           ← the string that named this crash
```

The kill instant is therefore not any single line of code but the *conjunction*: the loop's post-commit git work supplied the load, the bloat-era store made that load ~18 GB-shaped, the unbounded pack-objects converted it into RSS, and the 12 GiB cgroup converted the RSS into a kill. Remove any one link and attempt 10's survival is what all ten attempts would have looked like — which is what the repaired repo delivers today.

### 8.3 Resource issue, deadlock, or external signal?

**A resource issue — cgroup-constrained memory (memcg OOM).** The three-way template choice collapses once the terms are fixed:

- **Not a deadlock.** Every attempt made forward progress — eight of the nine killed attempts committed a finished deliverable before dying (§11.2); lifetimes 88–255 s are uncorrelated and far under the 600 s dispatch cap (a deadlocked dispatch pins at its cap or hangs silently); and attempt 10 succeeded against an *unchanged* repository. Deterministic hangs fail identically; this failed probabilistically — the signature of shared-resource contention (§11.3 step 7).
- **Not an administratively issued external signal.** No surviving record shows anything signalling this worker: the primary log contains **zero** signal references (verified), and no Aug-13 kernel record exists at all (§8.4's boot boundary). The exit-code shape also discriminates: across the day's 395 `agent.completed` events (344 × −1, 22 × 124, 18 × 0, 11 × 1) there is **not one** caught-signal-shaped status (129/SIGHUP, 143/SIGTERM, 130/SIGINT) — whatever killed 344 dispatches was never observed or handled, which is SIGKILL-class, not a delivered-and-caught signal. And the kills arrive at nine uncorrelated mid-run instants on one bead inside a 344-kill day whose same-worker regime is kernel-proven as memcg OOM for the operation class — not at a boundary an operator action or fleet trim would mark. (The August corpus's SIGHUP-cascade framing is explicitly superseded by the September kernel records.)
- **The precise rendering.** SIGKILL *is* externally delivered — its sender, the kernel's OOM killer, sits outside the dying process, which is exactly why no handler saw it and why the sentinel exists. But the *cause class* is resource exhaustion: the kernel killed the process because the cgroup's memory was spent, not because anyone chose to signal it. "External signal" in the template's sense — a deliberate act by an operator or supervisor — is excluded; "resource issue" is the answer, with the kernel as its delivery mechanism.

### 8.4 Precision note on "kernel-proven siblings" (§3 item 6 / canon §11.3)

§3 item 6 and canon §11.3 call the same-day siblings (bf-4k2ws, bf-1ea4g) "kernel-proven". The wording is looser than the evidence and this dispatch's signal-phrased criteria force the fix:

- The journal has a **single boot beginning 2026-08-15 19:56:33 EDT** (re-run: `journalctl --list-boots`), and the **first surviving `CONSTRAINT_MEMCG` record is 2026-08-16 00:27:35 EDT**. No Aug-13 kernel record can exist for bf-2vtzg **or** for bf-4k2ws/bf-1ea4g — canon §1 and the committed bf-4k2ws determination both say exactly this.
- The **kernel-proven element is the mechanism class** — `git` inside `run-p*.scope` dispatch scopes pinned at the 12 GiB cap, killed by `CONSTRAINT_MEMCG` — established by the journal-covered 2026-08-16 push-storm era (540 records re-counted this session; the era census — 414 on Aug 16, 257 of them git kills at anon-rss 12.30–12.56 GB, `oom_score_adj` 200, `oom_memcg=run-p*.scope` — per the committed bf-4k2ws determination). The same-day siblings are **log-proven** (exit −1 plus the identical four-event chain, 344 kills re-counted from the primary log), not kernel-proven.
- This tightens the wording to what canon §11.6 already asserts as the confidence basis ("the only reason it is not HIGH is that Aug-13 kernel proof is *impossible to exist*"). Classification INFRASTRUCTURE and mechanism confidence **MEDIUM-HIGH are unchanged**; append-only here per the freeze policy — the closed sections' text stands.

### 8.5 Verification record — re-run first-hand this session (2026-09-09)

| Claim | How verified | Result |
|---|---|---|
| Target state | `bead show bf-2vtzg` | Closed, rev 1, closed 09:42:58.663Z, never reopened |
| Alert wording | `bead show bf-37jbh` | `**Exit code**: -1 (signal -1)` verbatim |
| 10-attempt loop | re-extraction from the primary worker log (187 bf-2vtzg events) | 10 dispatches / 10 completions; 9 × exit −1 (88.0–254.6 s) + 1 × exit 0 (172.0 s) |
| Per-kill event chain | events within 30 s of each kill | `agent.completed {exit_code:-1}` → `outcome.classified {outcome:"crash"}` → `bead.released` → `outcome.handled {action:"alerted"}` |
| No signal ever recorded | `grep -ci '"signal'` over all 12,131 events | **0** |
| No memory telemetry exists | distinct event types in the day's log | 28 types; **no memory-named type** (only `fleet.cpu_saturated`, 602) — memory-at-kill unverifiable, which is *why* the mechanism stays chain-inferred |
| Exit-code shape day-wide | all 395 `agent.completed` events | 344 × −1, 22 × 124, 18 × 0, 11 × 1 — **zero** caught-signal statuses (129/143/130) |
| Journal boot boundary | `journalctl --list-boots` | single boot, first entry 2026-08-15 19:56:33 EDT |
| First surviving kernel memcg record | `journalctl -k \| grep -c CONSTRAINT_MEMCG` + first lines | **540**; first 2026-08-16 00:27:35 EDT — `Killed process 3322486 (git) … anon-rss:12301364kB … oom_score_adj:200`, `oom_memcg=run-p*.scope` |
| Dispatch scope ceiling | `cat /sys/fs/cgroup/<own in-flight scope>/memory.max` | `12884901888` = 12 GiB exactly, `needle.slice/run-p*.scope` |
| Recurrence closed today | `./scripts/check-repo-health.sh`; `git rev-list --count` both directions | exit 0, no unpushed backlog; `origin/main…HEAD` 0/0 |

**Root cause, one line:** the kernel's memcg OOM killer delivered SIGKILL (signal 9) to unbounded `git pack-objects` over the bloat-era ~18 GB object store inside the 12 GiB `MemoryMax` dispatch scope; the process died without a status, needle recorded the `exit_code = -1` sentinel, and the alert template rendered it as "signal -1" — a resource issue, not a deadlock and not a signalled process, with the mechanism chain-inferred for this bead (no Aug-13 kernel record can exist) and kernel-proven for the operation class via the 2026-08-16 journal records.
