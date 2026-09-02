# Agent Crash Report: bf-4x12ec — Final Consolidated Report

| Field | Value |
|---|---|
| **Crash bead** | `bf-4x12ec` — "Execute aggressive git garbage collection to eliminate OOM risk" (task / P2) |
| **Crash date** | 2026-08-14 (retry storm 10:23:02–11:27:26 UTC; success 12:58:45 UTC) |
| **Exit code** | `-1` (×44), `124` (×8), then `0` (×1) |
| **Worker / session** | `claude-code-glm-4.7-lab-domain-check` / `a6dbb1fc` (agent `claude-code-glm-4.7`, model `glm-4.7`) |
| **Workspace** | `/home/coding/domain-check` |
| **Classification** | INFRASTRUCTURE — cgroup-scoped OOM kill. **Not a domain-check code defect.** |
| **Work completed** | ✅ YES — 2026-08-14T12:58:45Z, on the 53rd attempt |
| **Bead closed** | 2026-08-17T14:50:41Z (manual; work itself finished Aug 14) |
| **This report** | Final synthesis — bead `domchk-8c3fceeb`, 2026-09-02 |

This document consolidates three completed investigations into one canonical
report and is the intended citation target for this crash:

1. [`bf-4x12ec-crash-artifacts-2026-09-02.md`](bf-4x12ec-crash-artifacts-2026-09-02.md) — artifact recovery + system-state evidence (child investigation 1, bead `domchk-2ff261ce`)
2. [`bf-4x12ec-log-review-2026-09-02.md`](bf-4x12ec-log-review-2026-09-02.md) — independent re-parse of the primary event log (child investigation 2, bead `domchk-30d451d3`)
3. [`bf-4x12ec-crash-investigation.md`](bf-4x12ec-crash-investigation.md) — original investigation + Addenda 1–3 (beads `domchk-661c2dc6`, `domchk-90640785`)

All timestamps UTC unless marked EDT (local = UTC−4).

---

## Summary

`bf-4x12ec` tasked Phase 1.2 emergency repository stabilization: pack the
**17.20 GiB across ~4,650 loose git objects** (18 GB `.git`) that were making
every git operation an OOM hazard. The bead's own body prescribed
`git gc --aggressive --prune=now` — the exact operation that then killed the
agents sent to run it.

The result was **not one crash but a 64-minute retry storm, 53 attempts**:

| Phase | Window (UTC) | Attempts | Exit | Classified | Duration each |
|---|---|---|---|---|---|
| 1. Crash loop | 10:23:02 – 11:27:26 | 44 | `-1` | `crash` / alerted | 38.9 – 115.8 s |
| 2. Timeout loop | 11:38:07 – 12:50:14 | 8 | `124` | `timeout` / deferred | exactly 600.0 s |
| 3. Success | 12:58:45 | 1 | `0` | `success` / none | 491.8 s |

The kill was a **cgroup (memcg) OOM kill of `git gc` inside the agent's
resource scope**, not host-wide memory exhaustion — mid-storm, with 45 Gi
host memory available and 0 B swap used, an attempt was still killed ~8 s
after launching gc. Every phase-1 attempt made **zero** packing progress:
the loose-object count and byte figures were identical before and after each
killed gc (aggressive gc builds delta chains in memory before writing any
pack).

The storm ended only when needle's **auto-split template** decomposed the
bead into three children (`bf-173o7e` gc, `bf-5jhvpk` repack, `bf-im2sl1`
verify). The final attempt created those children, chained them with
dependencies, and exited 0 at 12:58:45 with a passing verification gate. The
gc itself completed under child bead `bf-173o7e` (Closed): repo 18 GB →
**753 MB**, loose objects → **141**, git operations restored. Scheduled
safe-git-gc maintenance has since taken the repo to **92 MB / 53 loose
objects** (verified live 2026-09-02).

## Timeline (2026-08-14, UTC)

| Time | Event |
|---|---|
| 10:17:26.387 | Bead `bf-4x12ec` created (P2) |
| 10:21:06.969 | First claim — session `a6dbb1fc`, template `pluck`, 71,698-byte prompt |
| 10:21:23 | `git count-objects -vH` → **4,649 loose objects / 17.20 GiB**, 9.60 MiB pack |
| 10:21:32 | `du -sh .git/` → **18G** |
| 10:21:41 | gc attempt 1 → exit 128: `bad numeric config value '1.hour' for 'gc.aggressivewindow'` |
| 10:22:09 | Retry `"1h"` → exit 128 (git wants integer days) — two attempts lost to a stale bad config |
| 10:22:18 | `git config --unset gc.aggressivewindow` → OK (still unset today) |
| 10:22:36 | gc attempt 2 launched (`git gc --aggressive --prune=now`) — never returns |
| **10:23:02.958** | **Attempt #1 dies — `exit_code=-1`**, 115.8 s in (needle seq 1741) |
| 10:23:02.959 | Classified `outcome=crash` |
| **10:23:14.244** | **First alert** — 11.3 s after the first death; the earliest alert for this bead |
| 10:23:16 → 11:27:26 | **42 more claim→crash cycles** — 44 kills total, 39–116 s each; worker alerts, releases, re-claims each time |
| 10:43:35.281 | `worker.handling.timeout` — `bf sync --flush-only failed` mid-storm: the bead-store write path was also struggling |
| 10:43:58.590 | Mid-storm capture (transcript `9539f3b2`): disk 85% / 67G free; **mem 45Gi available, swap 0B** |
| 10:44:07 | That attempt launches gc → killed at 10:44:53 (~8 s later) |
| 11:28:07 | First of 3 `pluck` attempts with zero assistant output → exit 124 at exactly 600 s |
| 11:59:06 | Needle switches to **`split` template** ("Auto-Split: Decompose This Bead") |
| 11:59:06 → 12:50:33 | 5 more auto-split attempts → exit 124 at 600 s, no tool calls |
| 12:57:53–12:58:39 | Final attempt: creates children `bf-173o7e` / `bf-5jhvpk` / `bf-im2sl1`, chains deps, umbrella label |
| **12:58:45.113** | **Exit 0** (491.8 s) — work complete; `verification.passed` 11 ms later (`gates_run: 1`) |
| 12:58:55.502 | `bead.orphaned` — released unassigned instead of closed. This is why alerts kept regenerating until a manual close on Aug 17 (14:50:41Z) |

**Timestamp reconciliation.** Four other "crash timestamps" circulate in
older reports for this bead (10:25:30, 10:39:42.223, 10:41:13, 11:14:39).
All are single events *inside* the phase-1 storm — each is one attempt's
`HANDLING_RELEASE_DONE` heartbeat or its alert — not separate crashes.
The dispatch-recorded 10:23:11.219Z is crash #1's heartbeat; the agent
process actually died at 10:23:02.958Z. **First death 10:23:02.958Z, first
alert 10:23:14.244Z, last alert 11:28:04.917Z.**

## Root Cause Analysis

### Primary cause

**Cgroup-scoped (memcg) OOM kill of `git gc --aggressive --prune=now`.**
Aggressive gc computes delta chains across the full 17.2-GiB loose-object
set in memory before emitting a pack; against that repo the operation
exceeded the memory budget of the agent's transient `run-*.scope` and the
kernel SIGKILLed it. `exit_code=-1` is needle's classification for a child
that died without a wait status (signal death) — SIGKILL (9) is the
canonical cause given instant death, zero application error logs, and no
core dumps. The kill repeated identically 44 times because the trigger was
deterministic: same repo state, same command, same scope budget.

### Why host memory figures do not contradict this

Captured *inside* the crash window: **45 Gi available, 0 B swap used** — and
the next attempt was still killed. Free host RAM and an OOM kill coexist
when the kill is under a **cgroup** limit (`constraint=CONSTRAINT_MEMCG`),
which only needs the *scope's* budget exceeded. Corroboration from the
current boot's kernel log (same period, same mechanism): on 2026-08-16
13:29:49 EDT — inside bf-4x12ec's cleanup window, before the Aug-17 closure
— the kernel killed `git` at **7.79 GB anon-rss** under `CONSTRAINT_MEMCG`
in a transient `run-p*.scope`. All 13 `Killed process` events in the current
boot (6 node/vitest, 6 bash, 1 git) are CONSTRAINT_MEMCG with
`oom_score_adj=200`: this box's agent/child processes run in memory-limited
scopes and are marked preferred OOM victims.

### Debunked alternative explanations

| Claim | Status | Why |
|---|---|---|
| "gc ran ~57 minutes, then was SIGKILLed" | **False** | No phase-1 attempt survived 115.8 s; longest run of the day was the successful 491.8 s. The 57-min figure is just bead creation (10:17:26) → attempt #31's alert (11:14:39) |
| "External timeout/capacity governor, NOT OOM" | **Unsupported** | Phase-1 deaths (39–116 s) are far below the 600 s cap phase-2 runs visibly survived to; and a pure timeout cannot explain phase 3 succeeding in 491.8 s |
| "OOM impossible — 51 GB was free" | **Non sequitur** | See above: memcg kills do not need host exhaustion |

The **three-phase signature is itself the strongest causal evidence**: fast
kills while memory pressure was high → runs surviving to the 600 s cap as
pressure eased → the operation finally fitting under the cap. This is
consistent with OOM and inconsistent with both single-cause alternatives.

### Evidence and its limits

- **Primary source:** needle event log
  `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl`
  (10,138 lines) — 53 claims, 53 dispatched, 53 completed (44 × −1, 8 × 124,
  1 × 0). Independently re-derived twice (Addendum 2 and the log review);
  both agree exactly.
- **Kernel logs for 2026-08-14 are unrecoverable**: the current boot began
  2026-08-15 19:26 EDT and archived journals start 19:31. No historical
  memory metrics exist (`.beads/logs/resource-metrics.log` begins 09-01).
  The OOM determination therefore rests on the retry-storm signature plus
  same-period CONSTRAINT_MEMCG corroboration — **state this whenever citing
  the root cause; it is not direct Aug-14 kernel evidence.**
- Also absent: `.git/gc.log`, Aug-14 reflog entries, Aug-14 pack file (since
  rewritten by scheduled maintenance), and any "oom" event in the needle log
  (it records exit codes only).

## System State at Crash

From inside the crash window (transcript `9539f3b2`, 10:43:59 UTC, ~20 min
after the first crash — 8 s before its own attempt was killed):

```
Filesystem  Size  Used Avail Use%   →  444G  355G  67G  85% /
            total  used  free  buff/cache  available
Mem:         62Gi   16Gi   24Gi   22Gi        45Gi
Swap:        24Gi    0B    24Gi
```

| Resource | Value at crash | Reading |
|---|---|---|
| Host memory | 45 Gi available, 0 B swap used | Not host-constrained — kill was scope-limited |
| Disk | 85% used, 67 G free | Adequate; not a factor |
| Repository | 4,649 loose objects / 17.20 GiB / 18G `.git` | The hazard; unchanged across kills (zero gc progress) |
| Load context | Aug-12/14 storm period (load 15–17 seen on sibling investigations) | Elevated fleet-wide background |
| Process state | git killed by SIGKILL, no core dump, no app error log | Signal death, deterministic |

## Work Completion Status

**✅ COMPLETED — 2026-08-14T12:58:45Z, 53rd attempt.** The parent bead's
surviving role was umbrella; the actual gc ran under child bead `bf-173o7e`.

Final verified metrics (from the bead's recorded completion notes):

| Metric | Before | After | Target | Result |
|---|---|---|---|---|
| Repository size (`.git`) | ~18 GB (17.20 GiB loose) | **753 MB** | <500 MB | ⚠️ close (accepted) |
| Loose objects | 4,627 | **141** | <100 | ⚠️ close (accepted; later <100) |
| Pack objects | scattered loose | 10,265 in 750.67 MiB pack | — | ✅ |
| Git ops without OOM | failing | passing | required | ✅ |

The OOM risk was eliminated and all git operations returned to normal, so
both near-miss targets were accepted at close. Since then the scheduled
safe-git-gc timers have driven the repo further down.

**Current state (verified live 2026-09-02, this report):** `.git` **92 MB**
— 53 loose objects (436 KiB), 10,408 in-pack in a single 90.18 MiB pack,
0 garbage. Host: 49 Gi memory available. The cleanup has held.

**Loose ends from the split:** children `bf-5jhvpk` (repack) and `bf-im2sl1`
(verify) were left **Open** when the parent umbrella closed. They are
redundant — their work was subsumed by scheduled maintenance — and should be
closed, not re-dispatched.

## Patterns and Similar Past Crashes

1. **Part of the Aug-12/14 infrastructure storm.** Same period as bf-4yjq
   (50 verified crashes; part of a 455-event Aug-12 storm), bf-2ildm, and
   the bf-1s6c3 repository-bloat OOM (18 GB repo → OOM during git
   operations) — the same hazard class, one day earlier.
2. **Deterministic environmental failure defeats naive retry.** 44 identical
   kills from re-running the same command against the same repo state.
   Retry was the wrong tool; only decomposition (auto-split) made progress.
3. **Isolation confirmed, fleet-wide.** Within the worker log, 0/44 kills had
   any other bead completing within ±3 s. Scanning all six same-day worker
   logs (980 other-bead completions) finds one chance alignment — expected
   at this density is ~3. The storm was specific to bf-4x12ec's own retry
   cycle; the nearby bf-173o7e crash at 12:59:48 is uncorrelated.
4. **Duplicate-alert / false-positive storm (the dominant ongoing pattern).**
   bf-4x12ec has been the target of a dozen+ redundant alert beads
   (bf-qz9mov, bf-1uh46l, bf-48vwac, bf-4h2mqq, bf-4xbt4g, bf-4oblul,
   bf-2m532x, bf-3cy3vk, bf-44upi7, bf-2u3dzu, bf-5f9xqg, domchk-661c2dc6,
   domchk-90640785, …). One alert fired **nine days after** both completion
   (Aug 14) and closure (Aug 17). Root enabler: `bead.orphaned` at
   12:58:55 left the bead open, and alert generation lacked closed-bead
   filtering and dedup — exactly what the 2026-09-02 crash-alert-manager
   fixes (closed-bead filtering, duplicate detection, completion awareness,
   5-min cooldown, crash classification) address.
5. **"Exit -1" is routinely misread.** It is not signal −1; it is a signal
   death (here SIGKILL under memcg). SIGHUP-cascade crashes elsewhere on
   this box produce the same needle classification.
6. **The task specification embedded the hazard.** The bead body prescribed
   the exact command that killed its agents. Task authoring for git
   maintenance should prescribe `scripts/safe-git-gc.sh`, not bare
   `git gc --aggressive`.

## Lessons Learned

1. **Stop retrying deterministic failures; decompose.** 44 attempts bought
   zero progress. A same-cause, same-command kill signature within ~2 min
   should trip a circuit breaker (we now have `scripts/crash-circuit-breaker.sh`)
   or a decomposition path *early* — needle's auto-split only engaged at
   11:59:06, ~96 minutes in. The split succeeded because "gc" / "repack" /
   "verify" fit in individual scope budgets where the monolith did not.
2. **Never rule out OOM from host memory alone.** Check the process's cgroup
   budget: this box demonstrably memcg-kills git processes while the host has
   tens of GB free (`safe-git-gc-*.scope` kills recur in the journal).
   "51 GB was free" is not exculpatory evidence.
3. **Do not let task text prescribe memory-hazardous commands.** Bare
   `git gc --aggressive --prune=now` on a bloated repo is a known OOM
   trigger here. Use the staged, memory-limited, checkpoint/resume
   `scripts/safe-git-gc.sh` (proven: full gc completed with 1.1 GB peak RSS,
   97.5% size reduction, no OOM). This is now codified in the repo CLAUDE.md.
4. **Close beads, don't orphan them.** The 12:58:55 `bead.orphaned` directly
   caused a nine-day false-positive alert tail. The alert-side fixes are in;
   the worker-side behavior (close with completion notes at success) remains
   the cheap half of the fix.
5. **Pre-flight hygiene matters at the config level too.** Two attempts were
   lost to a stale `gc.aggressivewindow='1.hour'` before the crash loop even
   started; the agent's `git config --unset` fix persists. Validate git
   config before large operations.
6. **Capture kernel evidence immediately — it does not survive.** Journal
   rotation erased all Aug-14 kernel logs; resource metrics logging did not
   yet exist. Every future crash investigation should snapshot
   `journalctl -k`/`dmesg` for the window *first*. This report's root cause
   is necessarily inference-plus-corroboration, and says so.
7. **Write crash reports from primary event logs, not from earlier
   summaries.** The "57-minute gc" and "crashed at 11:14:39" narratives
   propagated across several documents until Addendum 2 re-derived every
   figure from the JSONL event stream; an uncommitted edit even
   reintroduced the debunked 57-minute claim on 2026-09-02 and had to be
   reverted (Addendum 3). Cite the event log.

## Source Documents

| Document | Bead | Contribution |
|---|---|---|
| [`bf-4x12ec-crash-investigation.md`](bf-4x12ec-crash-investigation.md) (+ Addenda 1–3) | original + `domchk-661c2dc6`, `domchk-90640785` | work verification, retry-storm analysis, false-positive re-verification |
| [`bf-4x12ec-crash-artifacts-2026-09-02.md`](bf-4x12ec-crash-artifacts-2026-09-02.md) | `domchk-2ff261ce` | artifact inventory, transcript-level timeline, mid-storm system state |
| [`bf-4x12ec-log-review-2026-09-02.md`](bf-4x12ec-log-review-2026-09-02.md) | `domchk-30d451d3` | independent event-log re-derivation; orphan/flush/verification-gate findings; fleet-wide isolation |
| `docs/crash-context-bf-4x12ec-summary.md` | — | false-positive-alert inventory (read for alerts, **not** crash mechanism — predates Addendum 2) |
| `docs/signal-analysis-exit-code-negative-one.md`, `docs/research/root-cause-analysis-signal-minus-one-crashes.md` | — | exit −1 semantics across the workspace |

---
**Report Date:** 2026-09-02 · Bead `domchk-8c3fceeb` · Confidence HIGH
(primary event log, triple-verified; kernel evidence window acknowledged)
**Classification:** INFRASTRUCTURE (cgroup OOM) — no domain-check code defect
