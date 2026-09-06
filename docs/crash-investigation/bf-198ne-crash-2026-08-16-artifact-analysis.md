# Crash Artifact Analysis — the 2026-08-16T13:30:50Z death on bead bf-198ne

**Bead:** domchk-507d18c4 (step 1 of 4: gather/analyze → domchk-9d269c9c classify → domchk-99f184b6 mitigate → domchk-f3c5f61d document)
**Parent alert:** domchk-d46ec441 "ALERT: Agent crash on bead bf-198ne" (still open; this chain exists to close it)
**Companion report:** [`bf-198ne-context.md`](bf-198ne-context.md) covers the **different, earlier** event — the 2026-08-12 bf-2xygo crash that bf-198ne was created to alert on. This document covers the **2026-08-16T13:30:50Z** crash of the agent that was dispatched to *work* bf-198ne.

All times UTC. Provenance per the corpus convention: **[LIVE]** = re-derived here from surviving primary sources; **[COMMIT]** = attested by a committed document whose primary sources have rotated; **[REPORTED]** = volumetric claim not reproducible today.

---

## 1. Executive summary

Two consecutive agents dispatched to bf-198ne on 2026-08-16 died the same way, ~50 seconds after
each issued `git push`. Both had already committed their work. They were killed by the kernel
under `CONSTRAINT_MEMCG` inside their 12 GiB dispatch scope, during the 12:00–17:00Z wave of the
Aug-16 storm — a wave in which 257 of 414 victims were `git` processes. The push was trying to
upload a 720-commit backlog whose tree still contained **5.6 GB of retired bead-forge state**
(`.beads/.bf_history`, 4.7 GB, plus 508 stale checkpoint shards); that state was dropped 22 seconds
after the wave's last kill, and the crashes stopped.

**Classification: INFRASTRUCTURE** (memory-pressure memcg OOM), per `docs/crash-response-guide.md`
§Phase 2A. No code defect. Work was not lost.

**[LIVE]** throughout, except where tagged.

---

## 2. The `exit -1` sentinel

`exit_code=-1` recorded by needle is **not a signal number**. Signals are 1–31; there is no signal
−1. Needle records `code().unwrap_or(-1)` when a child dies without a usable exit status — i.e.
killed by a signal. It is therefore *not* SIGHUP (a catchable signal, recordable as signal 1) and
*not* literally "SIGKILL signal 9".

Two committed documents misread it:

| Document | Claim | Correction |
|---|---|---|
| `docs/crash-investigation/bf-198ne-context.md` §"Signal -1 Technical Analysis" | "Signal -1 = SIGKILL (signal 9) … delivered exclusively by the Linux OOM killer" | −1 is needle's sentinel; the kill is memcg-constrained SIGKILL **inside the 12 GiB dispatch scope**, not the host OOM killer — the host was not out of memory **[LIVE, corpus c700252]** |
| `docs/notes/agent-crash-investigation-domchk-d46ec441.md` header | "exit -1 (signal -1, SIGHUP)" | Same correction; SIGHUP is signal 1 and would be recorded as such |

The conclusion each document reaches (infrastructure, not code) survives; the mechanism lines do not.

---

## 3. Artifact inventory

What survives for the 13:30:50Z crash, and what does not.

### Primary artifacts found (all [LIVE])

| Artifact | Path | What it proves |
|---|---|---|
| Needle dispatch log | `~/.needle/logs/claude-code-glm-4.7-lab-drawrace-70478ac5-2026-08-16.jsonl` (1,178 events, 13:05:37–15:22:10Z) | Complete dispatch history of bf-198ne: 7 dispatches, exit codes, outcome classifications, alert-creation timestamps |
| Crashed session transcript | `~/.claude/projects/-home-coding-domain-check/522efe9a-e68c-4f2e-9274-5ad8ec299b82.jsonl` (159 records; first 13:24:39.630Z, last write 13:30:49.450Z) | The agent's actual final act: commit, then `git push`, then nothing |
| Earlier crashed session transcript | `~/.claude/projects/-home-coding-domain-check/cdb2b90a-2a40-4439-b86e-bafff202f2c1.jsonl` (133 records, ends 13:24:33.724Z) | Same pattern on the previous attempt — makes the mechanism reproducible, not incidental |
| Pre-crash commit | `7a50353` "chore: update needle predispatch sha after Domain Watch verification" (13:29:49Z) | Work committed 61 s before death; preserved on branch `pre-squash-history-20260816` |
| Companion commit (attempt 3) | `26dab61` "feat: complete Domain Watch implementation (ADR-001)" (13:23:39Z) | The substantive work of the crashed task |
| Pre-squash lineage | branch `pre-squash-history-20260816`, tip `7e4edf6` (17:18:49Z local) | The 720-commit backlog the fatal pushes were uploading |
| Squash commit | `c27899f` "chore: catch up lab work onto origin (squashed)" (22:20:34Z) | Captured the day's work onto main; today `HEAD` and `origin/main` are identical (0 / 0 divergence) |
| Bloat-drop commit | `b2d8233` ".beads: 5.6G → 16M" (17:40:54Z) | Removed the object mass the wave was killing over; 22 s after the wave's last kill |
| Fleet crash census | re-derived from all `*2026-08-16.jsonl` in `~/.needle/logs/` | 409 `agent.completed` events with `exit_code=-1`; 26 in 13:18–13:36Z across 4 worker types |

### Negative findings — evidence that does not exist

| Source | Status |
|---|---|
| journald for Aug 16 | **Absent** — earliest surviving entry is 2026-08-17 15:33:14 [LIVE]. The Aug-16 kernel kill records were captured by the sibling corpus before rotation and are cited from there **[COMMIT c700252]** |
| `.beads/traces/bf-198ne/` | Absent — no trace directory for this bead (traces exist for Aug-17+ beads) [LIVE] |
| Per-agent event log | `…drawrace-bf-198ne.agent.jsonl` was zero-activity-cleaned at 15:25:31Z the same day [LIVE, `mend.zero_activity_log_cleaned` in the roam-1 log] |
| Coredumps | None before Aug 25 [COMMIT, corpus] |
| sar/sysstat | Not installed [LIVE] |

The per-agent and trace losses mean we cannot quote the agent's own tool results for the push —
but the transcript's final tool call, the dispatch log's exit codes, and the git object database
independently establish the sequence.

---

## 4. Verified timeline of bf-198ne on 2026-08-16

From the drawrace dispatch log [LIVE]. Every dispatch used template `pluck` with the identical
prompt hash `c96fcfcd…` (64,223 bytes) unless noted.

| # | Dispatched | transform.completed | `agent.completed` exit | Outcome | Result |
|---|---|---|---|---|---|
| 1 | 13:06:25 | 13:12:13 (348 s, 66 ev) | 1 | failure | released |
| 2 | 13:12:16 | 13:20:20 (484 s, 96 ev) | 1 | failure | released; mitosis: not splittable |
| 3 | 13:20:41 | 13:24:33 (232 s, 59 ev) | **−1** | **crash** | alerted → `domchk-3042abf1` (13:24:35.094) |
| 4 | 13:24:36 | 13:30:49 (373 s, 62 ev) | **−1** | **crash** | alerted → **`domchk-d46ec441` (13:30:50.879)** ← this investigation |
| 5 | 13:30:52 | 13:38:15 (444 s, 65 ev) | 1 | failure | released |
| 6 | — | 13:38:27 (9 s) | 0 | mitosis eval | not splittable |
| 7 | 13:38:29 (`split`) | 13:40:24 (114 s, 41 ev) | 0 | success | `bead.orphaned` |

The alert timestamp is the `HANDLING_RELEASE_DONE` heartbeat (13:30:50.879210Z), 0.44 s after the
real `agent.completed` (13:30:50.436803Z) — for this event the alert time *is* the death time,
unlike the bf-173o7e pattern where alerts lagged 8–120 s.

The `exit 1` outcomes (#1, #2, #5) are ordinary workflow failures — the agent ran to completion and
failed; only #3 and #4 are signal deaths.

---

## 5. What the crashed agents were doing when they died

Both crashed attempts ended identically [LIVE, both transcripts]:

| Attempt | Last commit issued | Last tool call | Process died |
|---|---|---|---|
| 3 | `26dab61` (13:23:39Z) — "feat: complete Domain Watch implementation (ADR-001)" | `git push` at 13:23:44.525Z | 13:24:34.436Z — **49.9 s later** |
| 4 | `7a50353` (13:29:49Z) — "chore: update needle predispatch sha after Domain Watch verification" | `git push` at 13:30:00.374Z | 13:30:50.436Z — **50.1 s later** |

Attempt 4's own last reasoning, recorded at 13:30:00.346Z, immediately before the fatal call:

> "Good, I've committed the predispatch SHA update. Now I need to push to origin and then close the
> bead. The branch is 661 commits ahead of origin/main now."

No tool result ever returned; the transcript ends there. Two independent processes, same task shape,
same object set, dying 49.9 s and 50.1 s after the same command — that regularity is a memory ramp
reaching a fixed bound, not a random fault.

### Why the push was heavy

`git rev-list --count pre-squash-history-20260816 ^8373e5d` = **720 commits** [LIVE] — everything
since the 2026-08-15 bead-forge→bead-rs rehydrate (`8373e5d`), which origin had never received.
Among them, `b2d8233` records in its own message:

> "removed .beads/.bf_history (4.7G of bead-forge rolling snapshots) and pruned 508 stale
> checkpoint generation shards … **.beads: 5.6G -> 16M**"

Until 17:40:54Z the tree therefore carried ~5.6 GB of blob data that origin did not have. Every
`git push` in the interim had to enumerate and pack that mass — `git pack-objects` being precisely
the process class the corpus identifies as the wave's dominant victim (257 of 414 kills were `git`,
largest anon-rss 12,555,188 kB ≈ 12.0 GiB, exactly at the scope bound) **[COMMIT c700252]**.

### Closure of the causal loop

The corpus dates the Aug-16 wave's final kernel kill to **17:40:32Z** [COMMIT c700252]. `b2d8233`,
which removed the 5.6 GB, is timestamped **17:40:54Z** — 22 seconds later. The crashes ended when
the object mass they were choking on was deleted. The fleet census supports the same boundary:
exit −1 events stop by 17:44Z in the surviving logs [LIVE].

---

## 6. Fleet context — this was not bead-specific

`agent.completed` events with `exit_code=-1`, bucketed by minute across all surviving Aug-16 logs
[LIVE]:

- **409** such events across the day (the corpus reports 826 *crash alerts* for the same day at
  alert granularity; the two figures measure different layers [REPORTED])
- Hourly: peaks at 12Z (43), **13Z (71)**, 14Z (61), 16Z (72)
- In the 18-minute window 13:18–13:36Z containing both bf-198ne deaths: **26 crashes across 4
  worker types** (`screenferry`, `test-fix`, `domain-check`, `drawrace`), hitting nearly every
  minute

`docs/crash-response-guide.md` sets "10+ crashes in 10 minutes" as the INFRASTRUCTURE EVENT
threshold; this window is 26 in 18 minutes with no shared bead, workspace, or code path. Nothing
about bf-198ne itself was failing — unrelated workers were dying simultaneously.

---

## 7. Pre-crash work completion status — verified, not assumed

The guide's false-positive test ("commit exists within 30 s before crash") is just missed here —
the commit is **61 s** before death — but the substance is the same and stronger, because the
commit content survives in current main:

| Check | Result [LIVE] |
|---|---|
| Attempt 4's commit exists | `7a50353`, 13:29:49Z, message matches the transcript's `git commit` invocation |
| Attempt 3's substantive commit exists | `26dab61`, 13:23:39Z — "feat: complete Domain Watch implementation (ADR-001)" |
| That work is in current HEAD | `internal/server/handlers_watch.go` (162 lines) + watch routes; `docs/adr/001-domain-watch-webhook-notifications.md` present |
| Both commits preserved | reachable only via `pre-squash-history-20260816` (original hashes superseded by the same-day squash) |
| The backlog reached origin | `c27899f` squash (22:20:34Z) captured the day's work; today `git rev-list --count origin/main..HEAD` = 0 and `HEAD..origin/main` = 0 |
| The alert's *subject* work also complete | bf-2xygo's divergence analysis is `20e4c3c` (2026-08-17 12:56:10Z, `docs/notes/divergence-statistics.json`) |

**Correction to prior documentation:** `docs/notes/agent-crash-investigation-domchk-d46ec441.md`
cites commit **`a3e2981`** as the divergence-statistics commit. That object does not exist
(`git cat-file -t a3e2981` → "Not a valid object name") [LIVE]. The real commit, same timestamp and
message, is **`20e4c3c`** — the Aug-16 squash rewrote hashes, and that document was written against
pre-squash references without re-resolving them.

**Bottom line:** the crash cost two agents their sessions and produced two alert beads; it cost the
work nothing. What the agents had built was committed before either died and is in main today.

---

## 8. Classification

Per `docs/crash-response-guide.md` §Quick Reference and §Phase 2A:

- **Exit code −1 → Infrastructure event.** Confirmed at kernel level by the corpus's journald
  re-derivation: 414 memcg kills on Aug-16, every one `CONSTRAINT_MEMCG` inside a dispatch scope,
  332 of them in the 12:00–17:00Z wave that contains both deaths.
- **Not a workflow failure** — the two `exit 1` attempts on the same bead are that class, and they
  look entirely different (agent ran to completion; no signal).
- **Not a service failure** — no HTTP 5xx; the inference gateway is not in the causal path.
- **Not a code defect** — the failing operation was a stock `git push`; domain-check code was not
  executing in a way that contributed. Consistent with the corpus's null result on domain-check
  code.
- **False-positive nuance.** The death was real, so "false positive" would be the wrong label —
  but this is a **post-completion kill**: work committed, crash during the push. The retry chain
  self-healed (attempt 5 completed the push generation at 13:37:55Z / `f8f8457`; the split
  dispatch orphaned the bead at 13:40:24Z; the squash carried everything to origin at 22:20:34Z).

**Severity: low.** Transient, self-healed within 10 minutes of dispatches, zero data loss.

---

## 9. Acceptance criteria

- [x] **Exit code −1 understood and documented** — §2 (sentinel semantics) and §5 (actual kill
      mechanism), with corrections to two misreading documents.
- [x] **Crash artifacts collected** — §3, with paths, sizes, and the negative findings that bound
      what can still be known.
- [x] **Pre-crash work completion verified** — §7, by object lookup rather than by trusting prior
      reports (which is how the `a3e2981` error propagated).
- [x] **Classified per crash-response-guide.md** — §8: infrastructure (memcg OOM under
      `CONSTRAINT_MEMCG` in the 12 GiB dispatch scope), post-completion, self-healed.

## 10. Handoff to domchk-9d269c9c (classify) and domchk-99f184b6 (mitigate)

The classification work is effectively done above; the mitigation step should treat this as
historical rather than actionable: the 5.6 GB object mass is gone (`.beads/` untracked in HEAD,
`.gitignore` covers it, repo `.git` is ~92 MB with 1 pack), the memory bounds that make bare
`git` safe are now mechanically enforced (`pack.windowMemory=2g`, `pack.deltaCacheSize=1g`,
`pack.threads=1`, verified by `scripts/setup-git-gc-config.sh --verify` and the 768 MiB cgroup
tests in `scripts/test-gc-memory-bounds.sh`), and the fleet has recorded **zero `exit_code=-1`
since 2026-08-17** [COMMIT c700252 + LIVE 09-06]. No further mitigation is required for this bead;
the remaining value of this chain is closing the stale alert `domchk-d46ec441` accurately.

---

*Report generated 2026-09-06 by domchk-507d18c4 (claude-code-glm-5.3-flash). Sources re-derived
this session: needle dispatch log, two Claude session transcripts, git object database, fleet log
census; Aug-16 kernel kill records cited from the committed corpus (c700252) because journald for
that day has rotated.*
