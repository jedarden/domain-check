# Evidence Compilation — 2026-09-01 Crash Investigation

**Bead:** domchk-59e1f1d5 (Gather Evidence; split-child of the 2026-09-01 crash investigation, parent report `domchk-20dc36b4`)
**Compiled:** 2026-09-05 (committed 2026-09-06 — the compilation sat untracked for a day,
which is the stall recorded as gap 2 in
`docs/investigations/findings-compilation-2026-09-05-domchk-65afcc88.md` and step 4 of the
recommendations in `docs/investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md`)
**Investigation period:** 2026-08-12 → 2026-09-05
**Scope:** Structured compilation of all evidence behind the 2026-09-01 final investigation report
(`docs/investigations/final-investigation-report-2026-09-01.md`), re-verified against primary sources where they survive.

---

## 1. Verification method

Every claim below is tagged with its strongest available verification level:

| Tag | Meaning |
|-----|---------|
| **[LIVE]** | Re-derived from a primary source on 2026-09-05 during this compilation (journal, needle log, `git config`, repo state) |
| **[COMMIT]** | Established in a committed investigation document; cite hash given |
| **[REPORTED]** | Appears in the 2026-09-01 final report but could **not** be re-verified against a surviving primary source |
| **[GONE]** | Primary source has been rotated/expunged; only the committed record remains |

Primary sources used:

- journald (`journalctl`), retention starting **2026-08-15T19:46:33-04:00** (host local time = UTC−4) — **[LIVE]**
- Needle primary worker log `~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log` (87 MB), spanning
  **2026-08-16T03:09:46Z → 2026-09-05T09:06:18Z** — **[LIVE]**
- Git repository state of this workspace — **[LIVE]**
- Committed investigation corpus under `docs/` — **[COMMIT]**

> **Timezone note.** journald prints host-local time (UTC−4). All timestamps in this document are **UTC**
> unless suffixed `-04:00`. First journal entry: `2026-08-15T19:46:33-04:00` = `2026-08-15T23:46:33Z`.

---

## 2. Evidence inventory and retention status

| Artifact | Retention | Status |
|----------|-----------|--------|
| Kernel memcg-OOM records (journald) | since 2026-08-15T23:46:33Z | **[LIVE]** — Aug-16 storm fully present; Aug-14 **not** covered |
| `systemd-oomd` kill records | since 2026-08-15T23:46:33Z | **[LIVE]** |
| Needle primary log (domain-check worker) | 2026-08-16 → 2026-09-05 | **[LIVE]** — Aug-16 onward; Aug-14 storm records **[GONE]** |
| Needle log archives (`~/.needle/logs/archive/`) | 2 files, 2026-07-30 / 07-31 only | **[GONE]** — no Aug-14 coverage (single-slot/snapshot retention) |
| Kernel logs for 2026-08-14 (bf-173o7e storm) | boot record begins Aug 15 | **[GONE]** — established in d283576 |
| Application stack traces (Go) | n/a | **None exist** — deaths were external SIGKILLs of the agent host process; no Go panic was ever recorded. See §5.3 |
| Reflogs covering 2026-08-14 | truncated to 2026-09-01 | **[GONE]** — established in d283576 |
| Committed investigation corpus | 157+ reports under `docs/` | **[COMMIT]** |

**Retention conclusion:** every event from 2026-08-16 onward is re-verifiable from primary sources today.
The 2026-08-14 bf-173o7e storm (132 dispatches / 129 exit −1) rests on the needle log that has since been
rotated; its surviving evidence base is the committed corpus (07ab240, db1acb3, d283576, b1ae579), which
recorded the raw counts while the log was still present.

---

## 3. Crash timeline (chronological, UTC)

### 2026-08-12 — bf-1s6c3: repository-bloat OOM (origin event)
- Repository reached **18 GB with 17.16 GB loose objects**; routine git operations pushed `git` past its scope limit → memcg OOM SIGKILL; exit −1.
- Cleanup reduced the repo 18 GB → 138 MB (−99.2%); task completed after cleanup.
- **[COMMIT]** c4d2b29, a7b1347, 2f1a9b2, 76a9c1d; summary in `docs/bf-1s6c3-investigation-summary.md`.
- Classification: INFRASTRUCTURE (self-inflicted resource exhaustion), no code defect.

### 2026-08-13 — bf-1ea4g false-positive cycle begins
- Original crash 2026-08-13 07:42:34Z assessed false positive (work completed before the kill);
  **9+ duplicate investigation beads** were subsequently created for the same event.
- **[COMMIT]** 383241f §Pattern 3; `docs/fix-proposal-bf-1ea4g-crash-pattern-2026-09-02.md`.

### 2026-08-14 — bf-173o7e storm (largest single event; kernel logs do not survive)
- Duplicate gc bead dispatched **132 times**; **129 attempts died `exit_code=-1`** over ~10.5 h —
  flat kill durations prove the object set never shrank between attempts (no attempt ever got far enough to pack).
- Mechanism: bare `git gc --aggressive --prune=now` drove `git pack-objects` RSS past the **`MemoryMax=12GiB`**
  of the needle dispatch scope (`run-p*.scope`) → kernel memcg OOM SIGKILL per attempt.
- Actual packing completed later (Aug-14 23:25 → Aug-17) by a non-attempt process (bf-4833lh note).
- **[COMMIT]** 07ab240 (counts re-verified live from the then-present log), db1acb3, d283576 (timing
  decomposition and Aug-14 git-side forensics), 5d501a8 / 227a15c (heartbeat-vs-kill timestamp mapping).
- Kernel-side records for this day **[GONE]** (journal begins Aug 15); needle-side records **[GONE]** (rotated);
  committed values **[COMMIT]**.

### 2026-08-16 — the Aug-16 storm (fully re-verified from primary sources, 2026-09-05)
- **Kernel memcg OOM kills: 414**, first `2026-08-16T04:27:35Z` (journal `Aug 16 00:27:35 -04:00`),
  last `2026-08-16T17:40:32Z` — a **13.2-hour** window, **[LIVE]**.
- Killed-process breakdown of the 414: **`git` 257**, **`node (vitest …)` 156**, other 1 — **[LIVE]**.
- Largest single `git` victim: **anon-rss 12,555,188 kB ≈ 12.0 GiB** (matches the 12 GiB scope bound;
  three top kills all ≈12.3–12.55 GB) — **[LIVE]**.
- All kills `constraint=CONSTRAINT_MEMCG` inside per-dispatch scopes (`needle.slice/run-p*.scope` and
  `app.slice/run-p*.scope`) — **cgroup-scoped, not host-wide** — **[LIVE]**.
- Needle crash outcomes for this worker: **461 on Aug-16 + 3 on Aug-17**, hourly spread 04Z…17Z
  (peak hours 16Z=85, 13Z=81, 14Z=57) — **[LIVE]**, distribution in §5.4.
- Verbatim first crash record — **[LIVE]**:
  ```
  2026-08-16T04:27:36.062598Z INFO worker.session{needle.worker_id=claude-code-glm-4.7-lab-domain-check
    needle.session_id=90a8ac84 needle.agent=claude-code-glm-4.7 needle.workspace=/home/coding/domain-check}:
    bead.outcome{needle.bead.id=bf-uoyie}: needle::outcome: handling agent outcome
    bead_id=bf-uoyie exit_code=-1 outcome=Crash(-1)
  ```
  It fires **0.9 s** after the kernel's first memcg kill of the day (04:27:35Z) — the needle-side view of the
  same SIGKILL.
- This worker's `exit_code=-1` count by day: **Aug-16: 157, Aug-17: 1, thereafter 0** — **[LIVE]**.

### 2026-08-17 → 2026-08-25 — deceptively quiet; detection flaws surface
- Signal deaths stop for this worker; alerts continue anyway. 200+ alerts accumulate, ~60% duplicates;
  post-completion kills (bf-5tgsk: completed 16:35:54, killed 16:36:24, bead closed 16:36:51) and
  self-healed retries (bf-6bio4g: crash → retry exit 0) both generate alerts — **[COMMIT]** 383241f.
- 2026-08-26 — last fleet-wide signal deaths: **18 worker deaths** from a needle defect (crash handler
  runs `bead release` against an already-closed bead → exit 4 → unhandled error kills the worker) — **[COMMIT]** eba6c2a.

### 2026-08-26 → 2026-09-05 — no signal deaths; failure class shifts to service
- **Zero `exit_code=-1` since Aug-26** fleet-wide; the day's failures are synchronized `exit_code=1` waves
  (13–15 workers/min, cross-workspace) = inference-service class, not signals; needle OTLP 503s traced to a
  live `needle-otel-collector` CrashLoopBackOff (config schema drift) — **[COMMIT]** f9af254 (domchk-8f35a61b RCA).
- All kernel memcg kills after Aug-16 are synthetic test/gc scopes (mw-oomdbg, probe-hog) or the bounded
  safe-git-gc run firing **as designed** (completed 07:55, 1 pack, 90.19 MiB) — **[COMMIT]** f9af254; **[LIVE]** repo state §6.1.
- 2026-09-01 19:10Z — final investigation report committed (383241f); 19:38:42Z — this bead's 4-way
  auto-split created (59e1f1d5 / 6281555d / ece81e17 / 7880ada7) — **[LIVE]** bead checkpoint.

---

## 4. Reproduction steps (re-derive every live number)

```bash
# Kernel memcg kill count on Aug-16 + window bounds + per-process breakdown
journalctl --no-pager --since "2026-08-16 00:00" --until "2026-08-17 00:00" \
  | grep -c "Memory cgroup out of memory"                       # 414
journalctl --no-pager --since "2026-08-16 00:00" --until "2026-08-17 00:00" \
  | grep "Memory cgroup out of memory" | head -1                # 00:27:35 -04:00
journalctl --no-pager --since "2026-08-16 00:00" --until "2026-08-17 00:00" \
  | grep "Memory cgroup out of memory" \
  | grep -oP 'Killed process \d+ \(\K[^)]+' | sort | uniq -c    # 257 git, 156 node (vitest …

# Constraint class (cgroup vs host)
journalctl --no-pager --since "2026-08-16 00:00" --until "2026-08-17 00:00" \
  | grep -oP 'constraint=\K[^,]+' | sort | uniq -c              # all CONSTRAINT_MEMCG

# Largest git victim
journalctl --no-pager --since "2026-08-16 00:00" --until "2026-08-17 00:00" \
  | grep "Memory cgroup out of memory" | grep "(git)" \
  | grep -oP 'anon-rss:\K\d+' | sort -rn | head -1              # 12555188

# Needle-side crash outcomes + exit codes (log spans 2026-08-16 → 2026-09-05)
grep 'needle.outcome="crash"' ~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log \
  | cut -c1-13 | sort | uniq -c
grep -oP "exit_code=\K-?\d+" ~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log \
  | sort | uniq -c                                              # 1023×0, 304×1, 158×-1, 17×124

# Repo health + gc bounds (see §6)
du -sh .git; git count-objects -vH
git config --get pack.windowMemory; git config --get pack.deltaCacheSize; git config --get pack.threads
```

---

## 5. Crash signatures, logs, and traces

### 5.1 Kernel memcg-OOM record (the closest thing to a "stack trace" — there are no Go stack traces)

```
Aug 16 00:27:35 lab kernel: git invoked oom-killer: gfp_mask=0xcc0(GFP_KERNEL), order=0, oom_score_adj=200
Aug 16 00:27:35 lab kernel:  oom_kill_process.cold+0x8/0x87
Aug 16 00:27:35 lab kernel: oom-kill:constraint=CONSTRAINT_MEMCG,nodemask=(null),cpuset=user.slice,
  mems_allowed=0,oom_memcg=/user.slice/user-1001.slice/user@1001.service/needle.slice/run-p1830008-i211518363.scope,
  task_memcg=…/run-p1830008-i211518363.scope,task=git,pid=1851349,uid=1001
Aug 16 00:27:35 lab kernel: Memory cgroup out of memory: Killed process 1851349 (git)
  total-vm:9729496kB, anon-rss:8452204kB, file-rss:5308kB, shmem-rss:0kB, UID:1001
Aug 16 00:27:35 lab systemd[1]: user@1001.service: A process of this unit has been killed by the OOM killer.
```
**[LIVE]** (verbatim from journald; the largest victim recorded anon-rss 12,555,188 kB).

### 5.2 Needle-side crash record

```
… bead.outcome{needle.bead.id=bf-uoyie needle.outcome="crash"}: needle::outcome:
  handling agent outcome bead_id=bf-uoyie exit_code=-1 outcome=Crash(-1)
```
**[LIVE]** — see §3 (2026-08-16) for the full line.

### 5.3 Why no application stack trace exists

The killed process was never domain-check Go code under distress — the SIGKILL target was `git`
(257 of 414) or `node` test runners (156) inside an agent's dispatch scope. SIGKILL is uncatchable, so the
agent dies without any Go panic/trace; needle records `exit_code=-1`, which per 5d501a8 is the
`code().unwrap_or(-1)` **sentinel for an unrecorded signal death** — not a signal number and not SIGHUP.

### 5.4 Exit-code and outcome distribution (this worker's log, 2026-08-16 → 2026-09-05) — **[LIVE]**

| Exit code | Count | Meaning |
|-----------|-------|---------|
| 0 | 1,023 | success |
| 1 | 304 | task/workflow failure (incl. service-class waves) |
| −1 | 158 | sentinel for unrecorded signal death (**all 157+1 on Aug-16/Aug-17**) |
| 124 | 17 | timeout |

Crash outcomes by hour, Aug-16 (UTC): 04Z 33, 05Z 6, 06Z 51, 07Z 6, 10Z 33, 12Z 39, 13Z 81,
14Z 57, 15Z 34, 16Z 85, 17Z 36 — plus Aug-17 16Z 3. Two waves: a morning wave (04–07Z, 129) and the
dominant midday/evening wave (12–17Z, 332).

---

## 6. Metrics and performance data

### 6.1 Repository health — **[LIVE] 2026-09-05** (re-confirmed 2026-09-06 at commit time)

| Metric | Crash-era (Aug-12) | Current |
|--------|--------------------|---------|
| Total repo size | 18 GB | **92 MB** (`.git`) |
| Loose objects | 17.16 GB | **11 objects / 104 KiB** (0 at commit — all since packed) |
| Packs / pack size | — | **1 pack / 90.34 MiB** (90.43 MiB at commit) |
| In-pack objects | — | 10,653 (10,712 at commit) |
| Garbage | — | **0** |

> The only movement between 2026-09-05 and the 2026-09-06 commit is the handful of
> investigation commits themselves landing in the pack. Every other [LIVE] figure in this
> document was re-run at commit time and matched exactly: 414 Aug-16 memcg kills
> (257 `git` / 156 `node`), needle exit-code distribution 1023×0 / 304×1 / 158×−1 / 17×124,
> and the gc bounds `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1`.

### 6.2 GC memory bounds (the mechanical fix for the Aug-14 mechanism) — **[LIVE]**

```
pack.windowMemory    = 2g
pack.deltaCacheSize  = 1g
pack.threads         = 1        # window limit is per-thread → threads must be pinned
```
Worst case ≈3 GiB per pack run vs the 12 GiB scope bound. Applied repo-locally and globally in 533cb46;
verified via `./scripts/setup-git-gc-config.sh --verify`; the bounded path was proven in production on
2026-09-02 (safe-git-gc bound fired as designed, gc completed, 1 pack 90.19 MiB) **[COMMIT]** f9af254.
Current `needle.slice`: MemoryHigh=24 GiB, MemoryMax=32 GiB (dispatch scopes carry their own
`MemoryMax=12GiB`) — **[LIVE]**.

### 6.3 Kill/pressure metrics — **[LIVE]**

| Measure | Value |
|---------|-------|
| Kernel memcg kills, Aug-16 | 414 (257 `git`, 156 `node`, 1 other) |
| Kernel memcg kills, Aug-15 / Aug-17 / Aug-18 | 0 / 0 / 0 |
| bf-173o7e storm (Aug-14, log since rotated) | 132 dispatches, 129 exit −1 **[COMMIT]** |
| Largest `git` victim RSS | 12,555,188 kB anon (≈12.0 GiB, at the scope bound) |
| Needle dispatch scope bound | `MemoryMax=12GiB` **[COMMIT]** 533cb46-era docs |
| Fleet signal deaths since Aug-26 | 0 **[COMMIT]** f9af254 |

---

## 7. Hypotheses tested and outcomes

| # | Hypothesis | Outcome | Decisive evidence |
|---|------------|---------|-------------------|
| H1 | A domain-check code defect causes the crashes | **REJECTED** | No Go panic/stack trace in any event; SIGKILL victims are `git`/`node`, never a domain-check fault; zero defects across 157+ investigations **[COMMIT]** 383241f |
| H2 | Host-wide memory exhaustion / systemd-oomd 94.71% pressure kills workers | **REVISED** | All Aug-16 kills are `constraint=CONSTRAINT_MEMCG` inside per-dispatch scopes **[LIVE]**; the literal `94.71` string in the journal is a substring of Tailscale IPs in socat lines, **not** an oomd record **[LIVE]** — see §8 |
| H3 | Exit −1 means SIGHUP cascade | **REJECTED** | Exit −1 is the `unwrap_or(-1)` sentinel for an unrecorded signal death (SIGKILL from memcg OOM); SIGHUP would be catchable and recordable **[COMMIT]** 5d501a8 |
| H4 | Repository bloat is the memory consumer that triggers the kills | **CONFIRMED** | 18 GB/17 GB-loose repo (bf-1s6c3) **[COMMIT]**; `git` victims at 12.0 GiB anon-rss = scope bound **[LIVE]**; bare `gc --aggressive` reproduces (bounded-path test peaks ≈312 MiB under the new config) **[COMMIT]** 533cb46 |
| H5 | Inference-gateway outages cause the crash alerts | **REVISED** | Gateway/service problems manifest as `exit_code=1` waves (13–15 workers/min), a different signature from exit −1; also: the documented `curl -sf` health check fails on the self-signed cert (curl 60) while the gateway answers 200 — a false-alarm vector **[COMMIT]** f9af254 |
| H6 | NEEDLE crash detection generates false/duplicate alerts | **CONFIRMED** | ~60% duplicate alerts; post-completion and self-healed crashes alerted; auto-split re-dispatches onto resolved beads (bf-4ifshb, bf-1cd5v6 ×3, 129 dups for bf-173o7e) **[COMMIT]** 383241f + f9af254 |
| H7 | GC operations are inherently unsafe here | **REVISED** | Bounded gc is safe and proven (Sep-02 run completed under its MemoryMax); the hazard was the **unbounded bare** `git gc --aggressive --prune=now` path, now defended by pinned git config **[COMMIT]** 533cb46, **[LIVE]** §6.2 |

---

## 8. Corrections to the 2026-09-01 final report

Re-verification against primary sources surfaced four material corrections. The report's **conclusions**
(no code defects; infrastructure + tool issue; zero data loss) stand; several of its **mechanism details** do not:

1. **"SIGHUP cascade" → memcg-OOM SIGKILL.** Exit −1 is a sentinel, not SIGHUP; the kernel records show
   `Memory cgroup out of memory` SIGKILLs of `git`/`node` (§5.1). **[LIVE]**
2. **Event window "12:00–17:00 (5 h)" → 04:27:35Z–17:40:32Z (13.2 h),** with two waves (04–07Z and
   12–17Z). Kill counts for this worker: 461 crashes on Aug-16, not 201+. **[LIVE]**
3. **"systemd-oomd Memory Pressure: 94.71%" is not a surviving record.** The only `94.71` matches in
   journald are substrings of Tailscale IP addresses in socat log lines. The verified mechanism is
   kernel memcg OOM at the 12 GiB dispatch-scope bound, with systemd-oomd corroborating
   ("A process of this unit has been killed by the OOM killer"), not the 94.71%-pressure figure. **[LIVE]**
4. **"System-wide OOM" → cgroup-scoped OOM.** Every kill is `CONSTRAINT_MEMCG` in a
   `needle.slice/run-p*.scope` or `app.slice/run-p*.scope` cgroup; the host as a whole was not out of
   memory. Consequence: host-wide memory alerting (70% pressure) would **not** have caught these —
   per-scope bounds (now pinned via git config + safe-gc scripts) are the effective control. **[LIVE]**

---

## 9. Configuration and code changes related to the crashes

| Change | Purpose | Reference | Verified |
|--------|---------|-----------|----------|
| `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` (repo + global) | Bound the bare-gc path that killed bf-173o7e | 533cb46; `scripts/setup-git-gc-config.sh --verify` | **[LIVE]** §6.2 |
| `scripts/safe-git-gc.sh` (+ `--full`, `--resume`, `--check-only`) and monitor | Staged, memory-limited, checkpointed gc | CLAUDE.md; test `scripts/test-gc-memory-bounds.sh` | **[COMMIT]** |
| systemd user timers (`domain-check-*`): repo-health daily 02:00, incremental gc 03:00, full gc Sun 04:00 (`MemoryMax=4G`) | Scheduled maintenance without unbounded memory | `scripts/setup-repo-maintenance.sh` | **[COMMIT]** + timers listed in CLAUDE.md |
| `scripts/crash-alert-manager.sh` (+ classifier, dedup, 5-min cooldown) | Suppress false positives / duplicate alerts | b7c0c21; `docs/crash-alert-fix-implementation-2026-09-02.md` | **[COMMIT]** |
| `scripts/verify-work-completion.sh <bead-id>` pre-close gate | Distinguish post-completion kills from mid-task crashes | CLAUDE.md, `scripts/README.md` | **[COMMIT]** |
| `scripts/preflight-health-check.sh`, `crash-pattern-detection.sh`, resource/service monitors | Pre-task and continuous monitoring | a672f17, 60f3fa2, 46fbba7 | **[COMMIT]** |
| `bead release` crash-handler defect (needle) | Kill source on Aug-26 (18 worker deaths) — upstream fix needed | eba6c2a | **[COMMIT]** |

---

## 10. Source index

**Primary (live) sources** — journald since 2026-08-15T23:46:33Z; needle log
`needle-claude-code-glm-4_7-lab-domain-check.log` (2026-08-16T03:09:46Z → 2026-09-05T09:06:18Z);
git repo state; `git config`; `systemctl --user show needle.slice`; bead checkpoint
(`.beads/checkpoint/forensic.jsonl`).

**Committed corpus (key documents)** — final report 383241f
(`docs/investigations/final-investigation-report-2026-09-01.md`); bf-173o7e RCA 07ab240, db1acb3,
d283576, b1ae579; timestamp mapping 5d501a8, 227a15c, 6b4aa4c; exit −1 sentinel 5d501a8;
gc bounds fix 533cb46; bf-1s6c3 corpus c4d2b29/a7b1347/2f1a9b2/76a9c1d; needle release-conflict
defect eba6c2a; Sep-2 service-class + otel RCA f9af254; repo integrity 7e57bb8.

---

**Compilation complete.** Evidence status: primary-verified for all events from 2026-08-16 onward;
commit-attested for 2026-08-12/13/14 (primary logs rotated). Downstream beads:
domchk-6281555d (RCA) blocked on this bead; ece81e17 (solutions) and 7880ada7 (finalize) follow.
