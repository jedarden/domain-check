# bf-65lsdu Chain Retrospective — crash findings and resolution

**Date:** 2026-09-09
**Deliverable bead:** domchk-754711b5 (document leg, child 4 of 4 of alert bead `bf-1mcxco`)
**Incident bead:** bf-65lsdu — "Run repository cleanup to eliminate 17GB bloat"
**Chain legs reviewed:** gather domchk-db5410d4 · analyze domchk-6f62651a · implement-fix domchk-97ed35b5
**Standing canon:** [docs/research/root-cause-analysis-bf-65lsdu-signal-minus-one-2026-09-02.md](../research/root-cause-analysis-bf-65lsdu-signal-minus-one-2026-09-02.md) (domchk-f853408b)
**Classification:** INFRASTRUCTURE EVENT — repository-bloat-era memcg/oomd kill regime (carried; no new cause claim)
**Domain-check code impact:** **NONE** — zero defects found by every investigation in this chain and across the corpus

This is the document leg of the bf-1mcxco investigation chain. It renders the four
task criteria — what happened, why it happened, how it was fixed, lessons learned —
by consolidating the chain's three sibling legs (closed 2026-09-09) with the prior
bf-65lsdu document family, and records where the older documents are superseded.
It adds no new cause claim; every figure below was re-verified first-hand from
primary sources on 2026-09-09 (verification record in §6).

---

## 1. What happened

### 1.1 The task

Bead `bf-65lsdu` (P2, created 2026-08-13T21:16:00.660Z) was the remediation task
for the workspace's 18GB repository bloat: run `git gc --aggressive --prune=now`
over **17.20 GiB of loose objects (4,515 objects)** — ~99% of the whole repository —
using the era's `scripts/cleanup-bloat.sh`. The bead itself was the hazard: the
cleanup of the bloat was the single most memory-hungry operation the bloat ever
produced, dispatched into a needle agent scope capped at **12 GiB MemoryMax**
(12884901888, re-read live 2026-09-09 from an in-flight dispatch scope).

### 1.2 The storm — 163 kills, not one crash

The bead did not crash once. It was killed **163 times** (JSON-parsed first-hand
from `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-1{3,4}.jsonl`,
re-counted byte-exact 2026-09-09):

| Window | Dispatches | Completions | exit −1 (kills) | Other |
|---|---|---|---|---|
| Aug-13 21:19:53 → 23:59:58 UTC | 128 | 127 | **127** | 1 `worker.handling.timeout` (23:41:35.460Z) |
| Aug-14 00:01:47 → 01:10:58 UTC | 46 | 42 | **36** | 6 exit 1 (5 classified failure) |
| **Total** | | | **163** | |

After the last failure the harness **quarantined** the bead
(2026-08-14T01:11:11.824Z, failure_count 5/5) — the only thing that finally
stopped the loop. Kill latency: Aug-13 min 29.7 s / mean 51.6 s / median 44.5 s /
max 369.7 s; Aug-14 mean 46.1 s — **158 of 163 kills under 120 s**, i.e. deaths in
agent startup and early git work, not deep in gc packing. Each kill was followed by
an alert bead, a release, and a re-claim ~60–95 s later that re-ran the identical
operation against the identical repository.

**163 distinct ALERT beads** titled "ALERT: Agent crash on bead bf-65lsdu" were
created — exactly one per kill — and **all 163 are Closed** (re-counted live
2026-09-09). `bf-1mcxco`, this chain's parent, is the storm's 5th alert: its report
"Timestamp" 21:27:56.401750288Z is the crash-handler **heartbeat 13.04 s after**
attempt 5's kill (agent.completed 21:27:43.363Z) — alert timestamps across this
family are heartbeat provenance, not kill instants.

### 1.3 Resolution — decomposition, not retry

| Time (UTC) | Event |
|---|---|
| 2026-08-14 01:11:11 | Bead quarantined (failure_count 5/5) — loop stops |
| 2026-08-17 00:32 | Bead split into three sequential children: `domchk-bdb1fedf` (baseline) → `domchk-af4b5ef4` (execute) → `domchk-87be56d8` (verify); parent converted to umbrella |
| 2026-08-17 00:34:00 | Cleanup attempt succeeds: **exit 0 in 90.267 s** (trace metadata re-read 2026-09-09); the actual gc ran under the children |
| 2026-08-17 00:45:33 | Parent `bf-65lsdu` closed (rev 5) |

`docs/cleanup-resolution-2026-08-17.md` records the result: **17.20 GB → 753 MB
`.git` / 118 loose objects**. No single decomposed step needed the memory footprint
of a monolithic aggressive repack. The canon final state is the verified
2026-09-01 cleanup, live and re-verified 2026-09-09: `.git` 105M, 29 loose objects
/ 192 KiB, 1 pack 100.70 MiB, 0 garbage, `check-repo-health.sh` exit 0, 0/0
divergence from origin. **Work was never lost.**

### 1.4 Where the earlier documents are superseded

Corrections carried forward from the 2026-09-09 legs (flagged, not silently
rewritten — the originals stand as history):

| Earlier document | Superseded figure | Current finding |
|---|---|---|
| [canon RCA 2026-09-02](../research/root-cause-analysis-bf-65lsdu-signal-minus-one-2026-09-02.md) §1.3 | "~123 alert beads" | **163** alert beads, one per kill — the Aug-14 continuation (36 kills + 6 exit-1, quarantine) postdates that RCA's Aug-13-only window |
| [crash investigation 2026-08-17](bf-65lsdu-crash-investigation.md) | one crash at 21:27:56Z; exit −1 "(signal -1, SIGKILL)" | 163-kill storm; 21:27:56Z is attempt 5's heartbeat; −1 is the harness's abnormal-child-death **sentinel**, not a signal number |
| crash investigation 2026-08-17 | completion "commit 5bf23b7 (752M, 22 loose)" | **5bf23b7 is absent from the object store** (era objects pruned) — that citation is unverifiable; the verifiable record is the resolution doc (753 MB / 118 loose) and the live 105 MB state |
| [retrospective crash report 2026-09-02](../reports/bf-65lsdu-retrospective-crash-report.md) | "11 SIGKILL events", "thirteen crash-alert beads", "11 wasted dispatches" | 163 kills, 163 alert beads, 169 failed dispatches (163 + 6 exit-1); that report's own R2 decompose rule is exactly what worked |
| retrospective 2026-09-02 | R7 "THE OPEN ITEM" — daily git-gc timer `status=127` | **RESOLVED** (see §3.3) — `domain-check-git-gc.service` last fired 2026-09-09 03:00:10 EDT, `Result=success`, exit 0 |

---

## 2. Why it happened

Four causal layers (analyze leg domchk-6f62651a; carried from canon, mechanism
**regime-matched**, not kernel-proven for this bead):

1. **Ultimate cause — committed bead state.** ~237 MB `.beads/*.jsonl` snapshots
   committed repeatedly bloated the repository to ~18 GB with 17.20 GiB loose
   objects. Every git operation in the workspace became memory-hungry.
2. **Proximate cause — the task was the death operation.** Bare
   `git gc --aggressive --prune=now` over a 17.2 GiB loose set maximizes
   pack-objects memory, and **no `pack.windowMemory` bound existed until
   2026-09-02** — the operation ran uncapped inside a 12 GiB dispatch scope.
3. **Re-dispatch amplifier.** crash → alert bead → release → immediate re-claim
   re-ran the same deterministic failure 163 times into the same constraint;
   quarantine (failure_count 5/5) was the only loop-breaker of the era.
4. **Kill latency locates the deaths.** 158/163 kills under 120 s: memory pressure
   crossed the kill threshold during startup/early git work, long before
   aggressive packing could finish — the victims never reached the operation they
   were dispatched to perform.

**Mechanism evidence.** No kernel record can exist for the kill window: journald
holds a single boot whose first entry is 2026-08-15 19:56:33 EDT, and the kills
span 2026-08-13/14. The mechanism is anchored by the same standing condition two
days later, in the retained journal: 2026-08-16 00:27:35 EDT kernel
"Memory cgroup out of memory: Killed process 3322486 (git) anon-rss:12301364kB"
inside a needle `run-p*.scope` (git at ~12.3 GB anon RSS on the same bloated
store), plus a 2026-08-15 23:25:00 systemd-oomd session kill at 90.60% memory
pressure.

**"signal −1" is not a signal.** Exit −1 is needle's abnormal-child-death sentinel
(died without exiting). True SIGKILL encodes 137; timeouts are separately coded
124 (22× on this worker that day). No caught-signal exits, no panic traces — the
retained attempt stderr holds only a SessionEnd-hook warning and the dispatch
scope name.

**Workspace-local trigger, not fleet-wide.** Aug-13 on this worker: 395 agent
completions = 344× exit −1 / 22× 124 / 18× 0 / 11× 1 (re-counted byte-exact
2026-09-09); bf-65lsdu's 127 is the day's largest share, ahead of bf-1ea4g (56),
bf-4k2ws (55), bf-2ildm (38), bf-1s6c3 (22). Five peer workers on healthy repos,
same box and day, lost ≤2 exit −1 each. The day's top victim being the
17GB-cleanup bead itself is precisely what a bloat regime predicts.

**Not:** a domain-check code defect (the crash was inside a git subprocess; the
codebase has zero defects across 157+ investigations), not SIGHUP, not
max-turns/timeout.

---

## 3. How it was fixed

### 3.1 Immediate resolution

Task decomposition (the 2026-09-02 retrospective's R2 rule, proven by this very
incident): split into baseline → execute → verify children, each small enough to
stay under the resource ceiling and independently verifiable. The monolithic form
died 163 times; the decomposed form succeeded once, in **90.3 seconds**.

### 3.2 Preventive layers now in force (each verified live 2026-09-09)

| # | Layer | Blocks which cause | Live verification (2026-09-09) |
|---|---|---|---|
| 1 | `.beads/` gitignored (`.gitignore:66`) + 10 MB pre-commit hook | Ultimate cause — bloat accumulation | `git ls-files .beads` = 0 tracked; hook installed and byte-identical to tracked source (`setup-git-hooks.sh --check` exit 0) |
| 2 | Persistent pack-memory bounds: `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` (repo-local **and** global) + `gc.auto=0` (GAP-2, commit 3a14ffe) | Proximate cause — uncapped pack-objects on bare gc **and** push | `setup-git-gc-config.sh --verify` exit 0 — worst-case pack memory ≈3072 MiB, within the ceiling for a 12 GiB scope |
| 3 | `safe-git-gc.sh` (staged, checkpointed, bounded) + crash-safe `cleanup-bloat.sh` rewrite at HEAD | The death operation itself | No bare `gc --aggressive --prune=now` at HEAD; the rewritten script's header names the one-liner it replaced |
| 4 | Crash-storm circuit breaker + `needle-with-limiter.sh` dispatch gate (commit ad73b42) | Re-dispatch amplifier — the ×163 loop | `test-crash-storm-regression.sh` (this chain's own replay): pre-breaker control **UNBOUNDED** (assertions fail there, as history did); post-fix trips at crash #3 → 3 dispatches, 1 alert, bead deferred — 18/18 assertions green |
| 5 | Memory/health monitoring: 8 `domain-check-*` systemd user timers | Silent re-accumulation | All 8 timers future-triggered; `check-repo-health.sh` exit 0; 0 unpushed backlog |
| 6 | Crash-alert hardening (closed-bead filter, dedup, cooldown, classification) | Aftermath false-positive storm (163 stale alerts all needed triage) | Suites green per the fix leg; `test-gc-memory-bounds.sh --unit` 10/10 re-run this dispatch |

This closes the canon RCA §7's then-residual gap ("no crash-storm breaker") with
ad73b42, and closes the bf-65lsdu death operation as a tested negative:
`test-gc-memory-bounds.sh` re-runs it under a 768 MiB cgroup and it completes
bounded (the crash-era run exceeded 12 GiB).

### 3.3 Status of the 2026-09-02 retrospective's open items

All four are now closed — verified live 2026-09-09:

- **R7 (open item: timers failing `status=127` on a PATH problem):** ✅ RESOLVED.
  `domain-check-git-gc.service` last fired 2026-09-09 03:00:10 EDT with
  `Result=success` / `ExecMainStatus=0`; its log shows a full safe-gc cycle
  completing at 105 M / 1 pack / 0 loose. `domain-check-repo-health.service`
  likewise `Result=success`.
- **R8 (commit repo copies of unit files):** ✅ RESOLVED — all 14
  `scripts/domain-check-*.{service,timer}` unit files are tracked in git.
- **R9 (docs assume cron):** ✅ RESOLVED — CLAUDE.md and the maintenance guide
  document the systemd user timers (this box has no `crontab`).
- **R10 (verify one clean timer cycle):** ✅ VERIFIED by the 2026-09-09 morning
  cycle — repo-health 02:00:06 success → auto-gc 02:30 → bounded incremental gc
  03:00:10 exit 0, repository holding at 105 MB, "GC not needed".

---

## 4. Lessons learned (for future bead execution)

1. **Deterministic resource exhaustion never yields to retry.** Memory demand is a
   function of repository contents; between retries nothing changed, so all 163
   attempts were guaranteed identical. Change the *shape* of the work (decompose)
   or bound the resource — never re-dispatch the same doomed operation. The
   circuit breaker now enforces the stop mechanically (trip at crash #3 vs the
   historical 163).
2. **The remediation can be the hazard.** An uncapped aggressive repack over a
   bloated loose set is the most memory-hungry operation available; the cleanup
   task became the bloat's largest memory consumer. Bound heavy operations in
   persistent config (not convention), and prefer decomposed forms whose steps
   each fit the resource budget.
3. **An alert's "Timestamp" is a heartbeat, not the kill instant.** bf-1mcxco's
   21:27:56.401Z stamp postdates its attempt's kill by 13.04 s. Derive forensics
   from the worker log / trace bracket, not from the alert bead's header.
4. **Read the exit-code table before classifying.** −1 is the harness's death
   sentinel (not a signal number, not SIGKILL=137, not timeout=124). A silent,
   traceless death is the SIGKILL/OOM signature; a caught signal or panic leaves
   a trace.
5. **Alert storms out-cost incidents.** 163 alert beads consumed triage sessions
   far exceeding the 90 seconds the actual cleanup needed, and retrospective
   false positives for the resolved bead kept arriving for weeks. Completion
   awareness, dedup, and the target-closure gate are load-bearing, not niceties.
6. **A prevention loop that fails silently is the durable risk.** The 2026-09-02
   retrospective found the daily gc timer dying `status=127` in 3 ms while
   everything looked installed — confidence without coverage. Prevention claims
   must be re-validated on a schedule
   ([docs/crash-prevention-validation.md](../crash-prevention-validation.md)),
   not cited from their landing commit.
7. **Workspace-local triggers masquerade as fleet-wide events.** The control-group
   comparison is the discriminator: 344 kills on the bloated workspace's worker vs
   ≤2 per peer worker on the same box and day. Check a peer cohort before
   declaring an infrastructure event host-wide.
8. **Record-retention gaps are coverage gaps, not proof of absence.** The journal
   begins 2026-08-15, after these kills — the mechanism is regime-matched from
   era anchors, and reports should say exactly that rather than overclaiming
   "kernel-proven".

---

## 5. Document family

| Document | Role | Status |
|---|---|---|
| [root-cause-analysis-bf-65lsdu-signal-minus-one-2026-09-02.md](../research/root-cause-analysis-bf-65lsdu-signal-minus-one-2026-09-02.md) | **Canon RCA** (domchk-f853408b) | Authoritative for cause/mechanism; §1.3 alert count superseded (163, not ~123) |
| [bf-65lsdu-crash-investigation.md](bf-65lsdu-crash-investigation.md) | First investigation (2026-08-17) | Historical — single-crash framing and 5bf23b7/752M citations superseded (§1.4) |
| [../reports/bf-65lsdu-retrospective-crash-report.md](../reports/bf-65lsdu-retrospective-crash-report.md) | Incident retrospective (2026-09-02, domchk-1d947f1e) | Historical — kill/alert counts superseded; R7–R10 since resolved (§3.3) |
| [../research/root-cause-analysis-bf-65lsdu-crash-2026-08-13.md](../research/root-cause-analysis-bf-65lsdu-crash-2026-08-13.md) | First RCA (domchk-2ab71440) | Superseded by the 09-02 canon (which corrects its crash mechanics) |
| [../cleanup-resolution-2026-08-17.md](../cleanup-resolution-2026-08-17.md) | Resolution record | Valid — 17.20 GB → 753 MB / 118 loose |
| [../fix-proposal-bf-65lsdu-oom-git-gc-2026-09-02.md](../fix-proposal-bf-65lsdu-oom-git-gc-2026-09-02.md) | Fix proposal | Superseded by implemented state (§3.2) |
| [../crash-information-bf-65lsdu.md](../crash-information-bf-65lsdu.md) | Information gather (2026-09-02) | Historical |
| [../maintenance/repository-maintenance-guide.md](../maintenance/repository-maintenance-guide.md) | Operating procedures | Current |
| This document | Chain retrospective (document leg) | Current |

---

## 6. Verification record (2026-09-09, this dispatch)

Re-run first-hand while writing this document; not cited from any leg's notes:

- Kill ledger re-counted byte-exact from the raw Aug-13/14 worker logs:
  127 + 36 exit −1, +6 exit 1; first kill 2026-08-13T21:22:23.216Z, last
  2026-08-14T01:10:58.163Z; Aug-13 worker-wide census 344/22/18/11 over 395 completions.
- Alert census re-counted from the live bead store: **163** beads titled
  "ALERT: Agent crash on bead bf-65lsdu", **163 closed**, 0 open.
- Target bead re-read: closed 2026-08-17T00:45:33.228Z rev 5; split children
  `domchk-bdb1fedf` / `domchk-af4b5ef4` / `domchk-87be56d8` all Closed.
- Success trace re-read: `.beads/traces/bf-65lsdu/metadata.json` — exit 0,
  outcome success, duration_ms 90267, captured 2026-08-17T00:34:00.391Z.
- Live repo state: `.git` 105M; 29 loose / 192 KiB; 1 pack 100.70 MiB; 0 garbage;
  `check-repo-health.sh` exit 0 (0 unpushed); `setup-git-gc-config.sh --verify`
  exit 0 (≈3072 MiB worst case).
- Timer cycle: all 8 `domain-check-*` timers future-triggered;
  `domain-check-git-gc.service` Result=success exit 0 at 2026-09-09 03:00:10 EDT;
  02:00 repo-health and 02:30 auto-gc likewise successful ("GC not needed").
- `test-crash-storm-regression.sh` 18/18 (pre-breaker control UNBOUNDED, post-fix
  trips at crash #3); `test-gc-memory-bounds.sh --unit` 10/10.
- Docs-only deliverable: no Go code touched.

## Sources

- `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-1{3,4}.jsonl` (kill ledger, re-parsed)
- Bead store: `bf-65lsdu`, `bf-1mcxco`, legs `domchk-db5410d4` / `domchk-6f62651a` /
  `domchk-97ed35b5`, split children, 163 alert beads (re-counted via `bead list --json`)
- `.beads/traces/bf-65lsdu/{metadata.json,stderr.txt}`
- `journalctl` (single boot from 2026-08-15 19:56:33 EDT; Aug-16 memcg-OOM anchors)
- `systemctl --user` timer/service state and `.beads/logs/git-gc{,-check}.log`
