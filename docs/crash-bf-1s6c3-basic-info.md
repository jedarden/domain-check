# bf-1s6c3 — Basic Crash Information (gather step)

**Compiled:** 2026-09-07 by domchk-7dc70ccd — the "Gather crash artifacts and basic crash
information" step of the parallel bf-1s6c3 investigation chain
(`domchk-7dc70ccd` → classification → investigation; sibling gather close: domchk-2f8e3f15).
**Committed by the bead's later attempt after a full re-verification pass:** every §1–§3
and §5–§6 figure was re-checked first-hand against the committed extracts, the bundle
manifest (5/5 OK), the session transcripts, and live `git`/`bead show`; §4's repository
figures were refreshed at commit time.
**Subject bead:** bf-1s6c3 — "Create merge commit reconciling Forgejo and GitHub histories"
**Status of the incident:** ✅ RESOLVED — subject bead closed 2026-08-16; repository repaired
and holding (see canonical report header)

**Canonical sources — cited, not duplicated:**

| Layer | Document |
|---|---|
| Analysis (root cause, classification, corrections) | `docs/crash-analysis-bf-1s6c3-2026-09-06.md` (domchk-ed3ed12b) — §12 carries each chain bead's dated re-verification |
| Raw-artifact bundle + per-artifact guide | `docs/crashes/bf-1s6c3/` (collection bead domchk-fcac734a; read its `README.md` first; `MANIFEST.sha256` covers 5/5 files) |
| Prior gather close (same spec) | domchk-2f8e3f15, closed 2026-09-07 with all live claims re-verified |

Every figure below was re-verified first-hand on 2026-09-07 against the committed extracts
and live `git`; where this file and the 2026-09-01 corpus disagree, this file follows the
canonical report (its §11 lists the corrections).

---

## 1. Crash timestamp

| Event | Timestamp (UTC) | Source |
|---|---|---|
| First `bead.claim.succeeded` | **2026-08-12T21:31:27.663Z** | extract seq 4641 |
| First dispatch (`agent.dispatched`) | 2026-08-12T21:31:27.674Z | extract seq 4650 |
| **First kill** (`agent.completed` exit −1) | **2026-08-12T21:36:44.519Z** (316,572 ms attempt) | extract |
| Last kill | 2026-08-13 (71st of 76 attempts) | extract |
| Final attempt exit 0 → `bead.orphaned` | **2026-08-13T02:01:22.561Z** → 02:01:27.732Z | extract |

**Storm span: 2026-08-12T21:31:27Z → 2026-08-13T02:01:27Z — 265 minutes, 76 dispatch
attempts, 71 kills** (49 on Aug-12, 27 on Aug-13).

> **Stale-timestamp warning.** The "2026-08-12T21:36:51Z" crash date carried by the Sep-1
> docs is attempt **1**'s post-crash handling instant (`agent.completed` 21:36:44.519 →
> `bead.released` 21:36:53.890), not "the" crash. Same rule for any other single instant
> named in a dispatch: it is one of 71 identical deaths unless stated otherwise.

## 2. Exit code and signal

`exit_code = −1` is **needle's wait-status sentinel for "died by signal" — it is not a
signal number** (no signal −1 exists). The kill signal was **SIGKILL** (uncatchable), and
the mechanism is the repo-bloat memcg-OOM path: `git push`'s pack-objects exceeding the
12 GiB `MemoryMax` of the per-dispatch systemd scope. Kernel records for Aug-12 itself do
not exist (journald starts Aug-15); the memcg mechanism is kernel-proven for the sibling
events bf-4x12ec (gc variant) and bf-198ne (push variant), and this event carries the same
complete Pattern-3 signature — see canonical report §4.3/§5.2.

Census re-counted first-hand from the committed extracts (945 + 513 lines):

| Exit code | Attempts | `outcome.classified` | `outcome.handled` |
|---|---|---|---|
| **−1** (SIGKILL) | **71** (60 died at `git push origin main`, 10 at `git push github main`, 1 bare `git push`) | crash × 71 | alerted × 71 |
| 124 (600 s timeout) | 4 | — | deferred × 4 |
| 0 (success) | 1 (auto-split only, no git) | — | none × 1 |

Each of the 71 crashes emitted one alert — 71 alerts for one undrained cause. No attempt
ever pushed successfully.

## 3. Agent version

| Field | Value | Evidence |
|---|---|---|
| Adapter / model | `claude-code-glm-4.7` / `glm-4.7` — identical on all 76 attempts | `agent.routing_decision` / `agent.dispatched` events |
| Worker slot | `claude-code-glm-4.7-lab-domain-check`, session `8446529e` | events |
| Template | `pluck` / `pluck-default`, `prompt_len=70670` (74 pluck attempts byte-identical, `prompt_hash sha256:aaa143d4…`; attempts 75–76 auto-split, `prompt_len=2868`) | `agent.dispatched` events |
| Claude Code CLI | **2.1.227** — verified identical across attempts 1, 19, 39, 75, 76 | `version` field in each crash-session transcript header |
| Needle worker binary at crash time | **0.2.19** (last confirmed install before the storm: 2026-08-11 10:02 EDT; `~/.needle/bin/needle-stable.pre-0.3.1.bak` reports `needle 0.2.19`) | binary backups. Caveat: the dispatch events record no needle version, the user journal starts 2026-08-17, and the 0.3.1 install date is unrecorded — the next dated binary evidence is 0.4.0 (mtime 2026-08-17 15:26 EDT, later renamed `needle-stable.pre-0.4.2-20260819`). Read "0.2.19, possibly superseded by 0.3.1 inside the Aug-11→17 gap" — exact crash-window daemon version is unrecoverable. |

## 4. Workspace path

`/home/coding/domain-check` (git repo, `lab.ardenone.com`, the fleet's domain-check slot).

Repository size at crash time: **≈18 GB `.git`, ≈17 GB loose objects** (17+ identical
~237 MB `.beads/*.jsonl` snapshots committed that day) — canon-sourced from the verified
cleanup, not re-measurable post-repair (canonical report §4).

Repository today (re-verified 2026-09-07 by this bead's committing attempt): `.git`
**101 MB**, 24 loose objects, pack 99.11 MiB, garbage 0 — the loose count is ordinary daily
churn (it read 16 earlier the same day). `git rev-list --left-right --count
HEAD...origin/main` read **0 / 0** at local HEAD `003af27`; the investigation chain's later
§12-append commits (`1547173`…`6178a01`) were stacked unpushed on local main at that moment
and land with this bead's push, restoring `origin/main` == local HEAD.

## 5. Bead being worked on (task context)

```
ID:       bf-1s6c3
Title:    Create merge commit reconciling Forgejo and GitHub histories
Created:  2026-08-12T21:12:09Z · P2 · task
Claimed:  2026-08-12T21:31:27Z (six seconds after its analysis predecessor bf-2xygo exited 0)
Closed:   2026-08-16T14:00:13Z (close reason cites dead SHA 7dd79eb — see canonical report §6/§11)
```

**What the agent was doing:** merging the diverged Forgejo (`origin`) and GitHub (`github`)
histories and pushing the reconciled result — never force-pushing. The task's git half
succeeded mid-storm: attempt 4 produced merge `42a7b07` (parents `47e7758` + `00117cb`) at
21:47:07Z, then died 59 s later like every other attempt. The commit survived on disk but is
**not an ancestor of `main`** (the 2026-08-16 squash moved it to `pre-squash-history-20260816`),
and no push ever landed — **71 of 76 attempts died with `git push` as the last command
issued**. The remotes were reconciled later by other work (merge `46293c5`, now an ancestor
of `main`; zero divergence today). No work was lost; domain-check code was never touched.

## 6. Artifact inventory

**Committed bundle — `docs/crashes/bf-1s6c3/`** (tracked; `MANIFEST.sha256` 5/5 OK re-verified 2026-09-07):

| File | Size | Contents |
|---|---|---|
| `needle-events-2026-08-12-bf-1s6c3.jsonl` | 257,002 B | 945 extracted needle events (byte-exact grep lines, original order) |
| `needle-events-2026-08-13-bf-1s6c3.jsonl` | 139,129 B | 513 extracted needle events |
| `bf-1s6c3-crash-sessions-2026-08-12_13.tar.gz` | 5.0 MB | all 76 crash-window Claude Code session transcripts |
| `sessions-index.tsv` | 7,192 B | 77 lines = header + one row per attempt (start, UUID, bytes, last-issued command) |
| `README.md` | 15,013 B | the collection bead's artifact-by-artifact guide (§1–7) |
| `MANIFEST.sha256` | 484 B | SHA-256 for the five payload files |

**Raw sources still on disk** (fabric-prune broken since 2026-08-17, so nothing was deleted):

| Source | Size | SHA-256 (first 16) |
|---|---|---|
| `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl` | 3,589,712 B | `3a487dc3139a2785` (re-verified 2026-09-07 — matches bundle README) |
| `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` | 3,111,314 B | `f76959bb0542e2b5` (re-verified 2026-09-07) |
| `~/.claude/projects/-home-coding-domain-check/*.jsonl` | 76 files, 22,254,104 B | per-file in `sessions-index.tsv` |

**Artifacts that do NOT exist for this event** (so no investigation should wait on them):

- Kernel/journald memcg records — system journal starts 2026-08-15 19:46 EDT, user journal 2026-08-17
- Core dumps — none before 2026-08-25 (all present are unrelated pdftract builds)
- System resource samples at crash time (memory/disk/load) — `.beads/logs/` monitoring starts 2026-09-01; needle OTLP traces are single-slot (last dispatch only)
- The needle daemon's own version string inside the Aug-12 events (see §3 caveat)

## 7. Acceptance-criteria mapping

| Criterion | Where |
|---|---|
| Crash timestamp and exit code documented | §1, §2 |
| Agent version and workspace identified | §3, §4 |
| Task context captured (bead being worked on) | §5 |
| List of all available crash artifacts compiled | §6 (including what does not exist) |

**Classification-step criteria (domchk-2a28452d):** crash type classified, pattern analysis,
false-positive determination, next steps → all four in **§8** below.

---

## 8. Crash classification, pattern analysis, false-positive determination

**Added 2026-09-07 by domchk-2a28452d — this chain's classification step** (the step after the
gather step that compiled §§1–7). Everything below was re-derived first-hand for this bead:
the census, cadence and density figures are recounted directly from this bundle's extracts
(945 + 513 lines), the git ancestry and repository figures re-measured live, then
cross-checked against the canonical report (§5) and the parallel chain's committed
classification deliverable
(`docs/crashes/bf-1s6c3-crash-classification-2026-09-06.md`, domchk-56b5ba67, commit `9b92cd9`)
— which it matches. This section adds the classification layer to the basic-facts sheet; the
canonical report's §5 (rationale), §6 (root-cause chain) and §12 (each chain bead's dated
re-verification) remain the authoritative record and are cited, not duplicated.

### 8.1 Crash type: INFRASTRUCTURE — repository-bloat sub-type (guide Pattern 3)

**Primary class: Infrastructure event. Confidence: high (~95%).**
**Sub-type: repository bloat. Confidence: high (~90%)** — the two legs that rest on
contemporaneous documentation rather than re-measurable state are (a) the ≈18 GB crash-time
repo size, which survives only in cleanup-era docs because the offending objects were packed
away 2026-09-01, and (b) the memcg kill mechanism, which has no Aug-12 kernel record (journald
starts Aug-15) and is kernel-proven only for the better-instrumented later siblings bf-4x12ec
(gc variant) and bf-198ne (push variant). Per the guide's own design, the classification rests
on the Pattern-3 detection signature — which is fully present — not on a kernel record that
cannot exist for this date.

Guide mapping applied (`docs/crash-response-guide.md` Quick Reference table):

| Guide row | Requires | bf-1s6c3 (recounted) | Verdict |
|---|---|---|---|
| Exit −1 → Infrastructure event | signal death, code unrecorded | 71 of 76 completions exit −1; **zero** `129`/`137` signal encodings anywhere in 1,458 extract lines (−1 is needle's sentinel, not a signal number — guide note 2) | ✅ |
| Exit −1 + fixed-cadence re-dispatch deaths + `.git` > 5 GB → **Infrastructure: Repository bloat** (Pattern 3) | all three | 71 deaths at median 173.7 s over a 227 min kill window; ≈18 GB `.git` at crash time; the task itself was the routine git operation (`git push`) that triggered each kill | ✅ |

Excluded alternates — each re-checked in the extracts for this classification, not copied:

- **Workflow failure** (exit 1 + `error_max_turns`): the string `max_turns` appears **0** times
  in either extract, and `transform.completed` succeeded on **all 76** attempts — template
  rendering was never the failure point.
- **Service failure** (exit 1 + HTTP 503/502): **0** occurrences of a 502/503 status to the
  inference gateway in either extract.
- **Code defect**: no application error in any attempt; the task never touched domain-check
  code (it was a two-parent git history merge). Consistent with the workspace's standing
  finding that no domain-check code defect has ever been confirmed.

### 8.2 Pattern analysis: repeated, not isolated — a deterministic self-sustaining storm

| Measure | Value (recounted from the committed extracts) |
|---|---|
| Shape | **Repeated** — 76 dispatch attempts against one bead; 71 identical signal deaths |
| Kill window | 2026-08-12T21:36:44.519Z → 2026-08-13T01:24:06.842Z = **227.4 min** |
| Storm span (first kill → final exit 0) | **265 min** (§1's table gives the same window as claim → orphan, 270 min) |
| Inter-kill gap | median **173.7 s**, range 74–731 s |
| Inter-completion gap (all 76 completions) | median **176.7 s** |
| Kill density | **3.12 kills / 10 min** over the kill window (2.68 / 10 min over the full 265 min span) |
| Exit-code variation | **zero** among the 71 deaths — plus 4 × 124 (needle's 600 s cap) and 1 × 0 |
| Attempt duration (the 71 kills) | median 160,886 ms, range 62,523–431,048 ms — **mid-attempt deaths, not post-completion cleanup** |
| Outcome bookkeeping | `outcome.classified` crash × 71 / timeout × 4 / success × 1; `outcome.handled` **alerted × 71** — one undrained cause raised 71 alerts |
| Re-dispatch loop | release → re-claim in ~10 s each death (measured by the parallel chain and re-confirmed at attempt boundaries in canonical report §12) |
| Last-command signature | 71 of 76 attempts ended with a `git push` issued (60 `origin`, 10 `github`, 1 bare) — the kill lands inside pack-objects |
| Same-mechanism context | follows bf-4yjq's 50 kills (17:54–20:30Z) the same evening on the same ≈18 GB object store |

This is the guide's Pattern-3 signature in full: fixed-cadence re-dispatch deaths for hours,
zero exit-code variation, repository > 5 GB, routine git operations triggering the kill. The
guide's own bf-1s6c3 evidence block (Pattern 3 section) carries the same shape.

### 8.3 False-positive determination: not a false positive as to cause; alert already discharged

| Rule (guide §False Positive Detection) | Test | bf-1s6c3 | Verdict |
|---|---|---|---|
| 1 — time gap | work committed < 30 s before crash | attempt 4 committed merge `42a7b07` at 21:47:07Z and was killed at 21:48:06.650Z — **59.6 s**, and the deaths were mid-attempt (median run 2.7 min) | **Not triggered** |
| 2 — success pattern | crash → retry → success (self-healed transient) | the 76th attempt did exit 0 — but via the auto-split *changing the task shape* (bead-only children, no `git push`), with the 18 GB cause untouched. A persistent cause outlasting the retry loop is Infrastructure, not transient (guide Rule 2 caveat, written for this storm) | **Surface match only — does not downgrade the classification** |
| 3 — system-wide event | 10+ crashes / 10 min | **3.12 / 10 min** (kill window) | **Not triggered** |

**Determination.** The crash is **not a false positive as to cause**: workers died mid-task of
an environmental kill for the entire storm (guide's FALSE_POSITIVE class requires a
post-completion-cleanup / completed-bead premise, which does not hold). **The alert is,
however, already discharged:** the subject bead bf-1s6c3 closed 2026-08-16, and its deliverable
is represented on `main` by the later reconciliation `46293c5` — re-verified live for this
section as an ancestor of `main`, while `42a7b07` itself is **not** (it survives only on
`pre-squash-history-20260816`), and the close reason's `7dd79eb` plus the older `2832106` both
fail `git cat-file` (dead pre-squash SHAs). The two-layer reading, per canonical report §5.3:
*what killed the workers* = infrastructure / repository bloat; *what the alert warrants today* =
nothing further — a verification-only disposition.

### 8.4 Next steps based on the classification

Pattern 3's remediation ladder, live status re-checked 2026-09-07 for this classification
(supersedes the parallel classification doc's 2026-09-06 snapshot where they differ):

| # | Action | Status (re-verified 2026-09-07) |
|---|---|---|
| 1 | Pack the object store (`safe-git-gc.sh`; never bare `git gc --aggressive`) | ✅ done 2026-09-01 — today: `.git` **102 MB**, 83 loose objects (daily churn), pack 99.11 MiB, garbage 0 |
| 2 | Keep bead state out of git | ✅ `.beads/` → 0 tracked files (`.gitignore:66` `.beads/`, `:70` `*.jsonl`) |
| 3 | Pre-commit > 10 MB gate **with an installer** | ✅ shipped 2026-09-06 (`dfa60a9`); `./scripts/setup-git-hooks.sh --check` exits 0 today — **supersedes the parallel classification doc's "item 3 = open gap" note** |
| 4 | Bound the bare-gc/push pack-objects path | ✅ `./scripts/setup-git-gc-config.sh --verify` passes today |
| 5 | Scheduled repo-health + bounded gc | ✅ the `domain-check-*` systemd user timers are installed and holding future triggers |
| 6 | Re-dispatch stop-condition for satisfied work (the amplifier that turned 1 kill into 71) | ❌ NEEDLE-side, outside this repository — remains the systemic finding |

**Net next steps: none for this event in this repository.** Items 1–5 are in place and were
re-verified live for this section; the only open lever is item 6, which lives in the NEEDLE
fleet. Full remediation rationale: canonical report §8/§9. Divergence check at write time:
`HEAD...origin/main` = 0/0 at `9ae17f2`.
