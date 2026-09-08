# bf-173o7e crash-context report — `domchk-114d6172` (2026-09-08)

Crash-information gathering pass for the agent crash on bead `bf-173o7e`
("Execute git gc --aggressive with pruning"). Every figure below was
re-derived first-hand on 2026-09-08 from the primary sources named in place —
nothing is copied forward unverified. This doc consolidates; it does not
replace the canonical determinations listed in §7.

**Verdict (matches the canon):** INFRASTRUCTURE — kernel **memcg OOM
SIGKILL** of the bead-prescribed, then-unbounded `git gc --aggressive
--prune=now` (17.20 GiB loose objects) inside needle's 12 GiB dispatch scope,
repeated **129 × `exit −1`** across a 10.5-hour retry storm. Work objective
was achieved by the storm's own exit-0 attempt; the bead closed 2026-08-17.
**Zero domain-check code involvement.** Confidence HIGH.

| Acceptance criterion | Result |
|---|---|
| Crash logs + timestamps retrieved | §1 — primary worker log re-derived live |
| Agent type + workspace identified | §2 — `claude-code-glm-4.7`, `/home/coding/domain-check` |
| Exit code (−1) + signal documented | §3 — sentinel, SIGKILL via memcg OOM |
| System logs / error patterns checked | §4 — journald window limits, kernel corroboration, alert-family census |
| Findings summarized | this document |

## 1. Crash logs and timestamps

**Primary source:** `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl`
(2,598,910 bytes; 2,857 lines carry `bf-173o7e`). `.beads/traces/bf-173o7e/`
holds only the **Aug-17** run (see §4.3); `.beads/checkpoint/forensic.jsonl`
begins 2026-08-16, so it has **zero Aug-14 events** — the storm survives there
only as issue snapshots of the alert beads it spawned.

### 1.1 Storm census (re-derived 2026-09-08 from the primary log)

**132 `agent.dispatched` / 131 `agent.completed`** for `bf-173o7e` — exit
histogram **129 × −1, 1 × 124, 1 × 0**. One dispatch never recorded a
completion (session-continuity note in the storm RCA §3).

| Hour (UTC) | exit −1 kills | duration range |
|---|---|---|
| 12Z | 1 | 50.2 s |
| 13Z | 35 | 40.4 – 214.3 s |
| 14Z | 17 | 37.5 – 123.9 s |
| 15Z–19Z | 0 | *(storm paused)* |
| 20Z | 1 | 58.9 s |
| 21Z | 45 | 28.2 – 128.5 s |
| 22Z | 14 | 37.6 – 216.6 s |
| 23Z | 16 | 38.7 – 198.9 s |

- First dispatch **2026-08-14T12:58:58.215Z**; first kill
  **12:59:48.629Z** (50.2 s run); last kill **23:24:21.728Z** (71.1 s run);
  overall kill durations **28.2 s (21:52:03.540Z) → 216.6 s (22:08:25.031Z)**.
- The exit-124 outlier: **22:19:04.950Z, 607.6 s** — the 600 s cap, not a kill.
- The success: **23:25:35.038Z, exit 0, 40.1 s** — the repository was packed
  by the storm's *own* attempt; per the storm RCA the bead was then
  `bead.orphaned` at 23:25:49 (success went unrecognized), leaving manual
  closure for Aug-17.

### 1.2 Bead lifecycle timestamps

| Event | Timestamp (UTC) | Source |
|---|---|---|
| Bead created | 2026-08-14T12:57:54.528Z | live bead record |
| First storm dispatch | 2026-08-14T12:58:58.215Z | primary log |
| Final kill (129th) | 2026-08-14T23:24:21.728Z | primary log |
| Exit-0 completion | 2026-08-14T23:25:35.038Z | primary log |
| Bead closed | **2026-08-17T17:12:09.406Z** | checkpoint snapshot |
| Close reason | "Git gc completed successfully - 17.20GB loose objects packed into 444MB pack file, repository valid" | checkpoint snapshot |
| Last updated | 2026-09-06T08:17:48.901Z (verification notes) | live bead record |

**Timestamp provenance trap (applies to every alert bead of this storm):**
the `Timestamp:` in a crash-report bead is the *heartbeat/release* instant,
not the kill. Four assigned→actual mappings are tabulated in the storm RCA §1
(e.g. assigned 14:06:16.551Z = `HANDLING_RELEASE_DONE` heartbeat, actual kill
14:06:08.828Z). Always re-derive from `agent.completed`. Example read from the
checkpoint today: alert snapshot `bf-115a60` says "Timestamp:
2026-08-14T21:59:00.194Z" — that is release bookkeeping, not a kill instant.

## 2. Agent type and workspace

- **Agent:** `claude-code-glm-4.7` — present on all 132 dispatch records;
  `model: glm-4.7`; provider `zai` (trace `metadata.json`).
- **Worker:** `claude-code-glm-4.7-lab-domain-check`; **dispatch template**
  `pluck` × 132.
- **Workspace:** `/home/coding/domain-check` (the worker's repo). The
  `agent.completed` records carry no workspace field (verified — extraction
  returns none), so workspace attribution is via the worker id + bead repo.
- **Repository state at crash time:** HEAD `00117cb` (SHA verified to exist:
  `git cat-file -t` → commit), **17.20 GiB loose objects / 9.6 MiB packed**,
  zero Aug-14 commits (per `domchk-8304c1c0`); the bead body itself prescribed
  the lethal command against exactly that state.

## 3. Exit code −1 and signal details

The kill records look like this (first kill, verbatim):

```json
{"timestamp": "2026-08-14T12:59:48.629746268Z",
 "event_type": "agent.completed",
 "worker_id": "claude-code-glm-4.7-lab-domain-check",
 "session_id": "a6dbb1fc", "sequence": 3361, "bead_id": "bf-173o7e",
 "data": {"agent": "claude-code-glm-4.7", "bead_id": "bf-173o7e",
          "duration_ms": 50216, "exit_code": -1, "model": "glm-4.7"}}
```

- **`exit_code: −1` is a sentinel, not a signal number.** Needle records
  `ExitStatus::code().unwrap_or(-1)` — "died on a signal needle did not send"
  (decode: `docs/analysis/signal-analysis.md`). There is **no signal /
  wait-status field** in the record — the death mechanism is not directly
  recorded, which is why the kernel line is corroborative rather than
  per-event (§4.1).
- **The signal was SIGKILL from the kernel's memory-cgroup OOM killer**,
  established by corroboration: mid-gc transcript proof (the 13:55:24.357Z
  attempt's transcript ends *"The git gc process is running successfully…"*
  + a `sleep 10 && tail` progress check; SIGKILL arrived 10.7 s later),
  deterministic duration distribution (129 kills at 28.2–216.6 s, far below
  the 600 s cap), and the kernel-confirmed identical mechanism on Aug-16
  (§4.1).
- **Alert-layer mislabel to ignore:** auto-generated crash-report beads phrase
  it "Exit code: -1 (signal -1)". "signal −1" is meaningless as a signal
  number — it is the sentinel leaking into prose.

## 4. System logs and error patterns

### 4.1 journald / kernel

- **No kernel records survive for Aug-14.** This machine has a single boot in
  journald; its first entry is **2026-08-15 19:56:33 EDT** — Aug-14 rotated
  away with the reboot. This is the RCA's residual-uncertainty gap; it cannot
  close retroactively.
- **Surviving corroboration (re-queried today):** **257
  `constraint=CONSTRAINT_MEMCG` `task=git` kills, all on 2026-08-16**, in
  needle-shaped dispatch scopes (220 `app.slice/run-p-*.scope` + 37
  `needle.slice/run-p-*.scope` under `user@1001`), anon-rss hugging the
  12 GiB `MemoryMax` (median 12,301,364 kB per the canon). They predate that
  day's first agent completion (04:23Z), so they are cleanup-window gc runs —
  they prove the *mechanism*, not these events.
- **Post-Aug-16 git memcg kills (new census this pass):** 50 more
  (`task=git`: 16 on Sep-06, 34 on Sep-07) — **all in named synthetic repro
  scopes** (`bf1s6c3-gc-*`, `bf1s6c3-push-*`, `bf4yjq-crash-*`,
  `gcmb-bare-aggressive-*`): the death-op replay / memory-bounds tests
  working as designed. **Zero `run-p` dispatch-scope git kills since
  Aug-17** (queried Aug-18 → present). So the canon's "zero git memcg kills
  since Aug-16" holds for live dispatch scopes; the 50 synthetic kills are
  expected, not a regression.
- **Host-wide OOM ruled out:** ~45 Gi host RAM free mid-storm (per
  `bf-4x12ec-root-cause.md` §3); memcg kills are scope-local by construction.
  Boot-wide today: 1,060 `oom-kill` lines, 530 `CONSTRAINT_MEMCG`, 307
  `task=git` — no host-OOM kills.

### 4.2 Error pattern — the alert-family aftermath

**159 live beads carry `bf-173o7e` in their title** (census today via
`bead list --json`): **106 closed, 37 in_progress, 14 open, 2 deferred.**
The storm's 129 kills + broken success handling produced an alert bead family
that regenerated stale duplicates for weeks after the 2026-08-17 closure —
dozens of `verification-report-bf-XXXX-duplicate-alert-resolved-bf-173o7e`
docs in `docs/archive/crash-investigations/` are the record of that cleanup.
This census is informational; the open/in-progress beads belong to their own
dispatch chains and are not this bead's action item.

### 4.3 Error pattern — the single-slot trace trap (the recurring misread)

`.beads/traces/bf-173o7e/` (metadata captured **2026-08-17T17:06:59Z**,
exit_code **1**, outcome `failure`, 444,317 ms) holds the **Aug-17** run, not
the Aug-14 storm: its final events are repeated `bead close bf-173o7e
--reason "Git gc completed successfully" --skip-verify` attempts ending in
`{"type": "error", "code": "error_max_turns"}`. Every doc that reports
"exit code was 1 (error_max_turns), NOT −1" for bf-173o7e — e.g. the archived
2026-08-26 crash-data bundle — is reading this trace and describing the
**post-completion bead-close exhaustion event**, a different event from the
Aug-14 kills. Both are real; they are 3 days apart. A SIGKILLed dispatch
writes no trace at all.

## 5. Root cause (one paragraph, canon unchanged)

The bead's own body prescribed `git gc --aggressive --prune=now` over 17.20
GiB of loose objects with no `pack.windowMemory` bound in place; `--aggressive`
computes delta chains across the whole object set in memory, pack-objects RSS
grew unbounded inside needle's transient `run-*.scope` (`MemoryMax=12 GiB`,
`oom_score_adj=200`), and the kernel's memcg OOM killer SIGKILLed the
highest-badness task — usually the agent process itself, hence the `exit −1`
sentinel. The retry layer re-dispatched the identical, identically-doomed task
131 more times because nothing bounded the loop and success went unrecognized.
Full chain + alternatives ruled out: §7 references.

## 6. Reconciliations found this pass (annotate, don't rewrite)

1. **Storm totals line:** the Sept-02 storm RCA says "129 × exit −1
   (21.6–216.6 s)". Re-derivation gives **min 28.2 s** (21:52:03.540Z); the
   21.6 figure is not reproduced by this pass (the RCA's own per-hour table
   bottoms at 28.2). Its max 216.6 s is confirmed (22:08:25.031Z, in the 22Z
   hour, whose published row mixed the exit-124 in — exit−1-only for 22Z is
   n=14, not 15).
2. **Close instant:** the storm RCA header says "closed 2026-08-17T17:15:23Z";
   the checkpoint snapshot and the bead's own 2026-09-06 note say
   **17:12:09.406Z**. The checkpoint value is canonical.
3. **Boot first entry:** 2026-08-15 **19:56:33** EDT measured today vs
   "19:26 EDT" in the Sept-02 docs — same fact (Aug-15 evening EDT), the
   earlier figure likely read a different line; no consequence.

## 7. References (canon)

- Storm RCA: `docs/crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md`
- Root-cause determination: `docs/investigations/bf-173o7e-root-cause-determination-domchk-2e371a2c-2026-09-02.md`
- Original-bead context: `docs/investigations/bf-173o7e-original-bead-context-domchk-8304c1c0-2026-09-02.md`
- Signal sentinel decode: `docs/analysis/signal-analysis.md`
- Shared mechanism (bf-4x12ec): `docs/crash-investigations/bf-4x12ec-root-cause.md`
- Archived single-slot-trace misread (superseded for Aug-14, valid for Aug-17):
  `docs/archive/crash-investigations/crash-data-bundle-bf-173o7e.md`

## 8. Current status (re-verified live 2026-09-08)

- Bead `bf-173o7e`: **Closed** (rev 19); work complete; no retry owed.
- Repository: `.git` 106 MB; 142 loose objects / 1.07 MiB; pack 100.49 MiB;
  0 garbage; `git fsck --full` exit 0; `./scripts/check-repo-health.sh` exit 0
  with the effective pack-memory bound verified (windowMemory=2g /
  deltaCache=1g / threads=1 → worst case ≈3,072 MiB within the ceiling for a
  12 GiB scope) and 0 unpushed commits.
- Prevention layers in force (bare gc banned → `scripts/safe-git-gc.sh`;
  persistent pack-memory config; circuit breaker; alert-manager closed-bead
  filtering/dedup/cooldown) — the applicable repo/gc-bounds layer re-ran green
  today as above.
