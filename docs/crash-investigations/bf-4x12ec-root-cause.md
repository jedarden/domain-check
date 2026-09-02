# Root Cause Determination: bf-4x12ec

| Field | Value |
|---|---|
| **Crash bead** | `bf-4x12ec` — "Execute aggressive git garbage collection to eliminate OOM risk" (task / P2, created 2026-08-14T10:17:26Z, closed 2026-08-17T14:50:41Z) |
| **Crash window** | 2026-08-14, 10:23:02–11:27:26 UTC (phase 1) — 53 attempts total until 12:58:45 UTC |
| **Exit signature** | `exit_code = -1` ×44 (39–116 s each), `124` ×8 (600 s cap), `0` ×1 |
| **Determination date** | 2026-09-02 · bead `domchk-9e2aa740` |
| **Confidence** | **HIGH** for the mechanism; **MEDIUM-HIGH** for the Aug-14 kill itself (kernel logs for that date are rotated away — see Evidence Limits) |
| **Canonical companion** | [`bf-4x12ec-final-crash-report.md`](bf-4x12ec-final-crash-report.md) (consolidated narrative; this file is the formal root-cause statement) |

---

## Root Cause Statement

> **`bf-4x12ec`'s agents were killed by the kernel's memory-cgroup (memcg) OOM
> killer: `git gc --aggressive --prune=now`, executed inside needle's
> transient memory-limited `run-*.scope` against a repository holding
> 4,649 loose objects totalling 17.20 GiB, drove git's in-memory delta
> compression past the scope's memory budget, and the kernel SIGKILLed the
> `git` process. The agent child therefore died on a signal with no wait
> status, which needle records as `exit_code = -1` and classifies `crash`.
> The trigger was deterministic — same repository state, same command, same
> cgroup budget — so the worker's retry loop reproduced the identical death
> 44 times over 64 minutes with zero packing progress, until needle's
> auto-split template decomposed the bead (11:59:06Z) into gc / repack /
> verify children that each fit inside the scope budget; the 53rd attempt
> exited 0 at 12:58:45Z.**

Classification against the four candidate categories the task poses:

| Candidate | Verdict |
|---|---|
| **OOM** | ✅ **Yes** — cgroup-scoped (`CONSTRAINT_MEMCG`), *not* host-wide |
| **Signal** | ✅ **Yes** — SIGKILL (9) delivered by the kernel OOM killer; the proximate cause of `exit -1` |
| **Panic** | ❌ No — no Go/runtime panic, no core dump, no application error output on any of 53 attempts |
| **domain-check code defect** | ❌ No — the crashing process was `git`, not the domain-check binary; the agent's own transcript shows correct tool use throughout |

OOM and signal are not alternatives here: the memcg OOM kill *is* the signal
source. "Resource issue" is the accurate single-word answer.

---

## Verification Performed by This Dispatch

The consolidated report's figures were re-derived independently from primary
sources rather than cited. All checks agree.

### 1. Retry storm — primary needle event log, re-parsed

Source: `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl`
(10,138 lines). Events for `bead_id=bf-4x12ec`:

| Event | Count |
|---|---|
| `bead.claim.succeeded` / `agent.dispatched` / `agent.completed` / `outcome.classified` | 53 / 53 / 53 / 53 |
| `agent.completed` exit `−1` (classified `crash`) | 44 |
| `agent.completed` exit `124` (classified `timeout`) | 8 |
| `agent.completed` exit `0` (classified `success`) | 1 |
| `bead.released` / `bead.orphaned` / `verification.passed` / `worker.handling.timeout` | 52 / 1 / 1 / 1 |

Dispatch→completion durations computed from the same log:

- Phase 1 (`−1`): **44 kills, 39.0 s – 116.0 s** — never enough time to write a pack
- Phase 2 (`124`): **8 runs, all exactly 600.1 s** — the timeout cap, i.e. runs that *survived* long enough to hit it
- Phase 3 (`0`): **491.9 s**, completing at `2026-08-14T12:58:45.113Z`

(Exit-code tallies in the raw log read 88/16/2 because each completion emits
both `agent.completed` and `outcome.classified` with the same code: 44×2, 8×2, 1×2.)

The three-phase shape is itself the strongest causal evidence: fast kills
while memory pressure was highest → runs surviving to the 600 s cap as it
eased → the decomposed workload finally completing in 491.9 s. That gradient
is what a memory-budget failure produces and what neither a pure timeout nor
a code defect produces.

### 2. Kernel corroboration — memcg kills in the current journal

Re-queried live 2026-09-02 (`journalctl -k`):

| Measurement | Value |
|---|---|
| `oom-kill:constraint=CONSTRAINT_MEMCG` lines | **420** (414 on Aug 16, 6 on Sep 02) — **zero host-wide** |
| Constraint lines naming `task=git` | **257, all on 2026-08-16** |
| `oom_score_adj` of every git kill | **200** (preferred victim — these scopes are flagged OOM-able by design) |
| `anon-rss` at kill, 257 events | median **12,301,364 kB**, max **12,555,188 kB** — a hard ~12 GiB memcg ceiling |
| `oom_memcg` path | a **distinct** `…/app.slice/run-p*.scope` per kill — one fresh needle-dispatched scope per retry |

Sample (first Aug-16 git kill):

```
Aug 16 00:27:35 lab kernel: oom-kill:constraint=CONSTRAINT_MEMCG,nodemask=(null),
  cpuset=user.slice,...,oom_memcg=…/app.slice/run-p3295453-i208789885.scope,
  task=git,pid=3322486,uid=1001
Aug 16 00:27:35 lab kernel: Memory cgroup out of memory: Killed process 3322486 (git)
  total-vm:13847248kB, anon-rss:12301364kB, … oom_score_adj:200
```

This is the **Aug-14 mechanism replayed with kernel logging intact** (during
the bf-4x12ec cleanup window, before the Aug-17 closure): same command class,
same transient-scope isolation, same ~12 GiB kill ceiling, same
`oom_score_adj=200`. Minor reconciliation with the canonical report's count:
that report cites "419 `Killed process` events, every one `CONSTRAINT_MEMCG`";
the live journal has 420 constraint lines (414 Aug-16) vs 419 "Killed process"
lines (413 Aug-16) — one Aug-16 memcg OOM's kill line did not survive. The
**257 `task=git`** figure matches exactly, and is the number that matters.

### 3. Host memory does not contradict the OOM verdict

Mid-storm capture from inside a killed attempt's transcript (10:43:59 UTC,
8 s before its gc was killed): **45 Gi host memory available, 0 B swap used**
— and the next attempt died anyway. That combination is only possible when
the kill is scoped to a cgroup budget, which is exactly what every kill in
the journal shows (`oom_memcg=…run-*.scope`). "51 GB was free" style
objections are non sequiturs for memcg kills.

### 4. Specific condition — the code path that died

- The bead **body itself prescribed the lethal command**
  (`git gc --aggressive --prune=now`, plus `git repack -a -d --depth=250
  --window=250`) — the hazard was embedded in the task text, not introduced
  by an agent.
- `--aggressive` forces git to compute delta chains across the *entire*
  object set (window/depth 250) **in memory before writing any pack
  bytes**. That is why every killed attempt left the repo byte-identical:
  4,649 loose objects / 17.20 GiB before *and* after each kill — zero
  progress per attempt, 44 times.
- The process ran under needle's transient `run-*.scope` with a memory
  ceiling (same class as the observed ~12 GiB kills) and
  `oom_score_adj=200`. When git's anon-rss reached the ceiling, the kernel
  killed `git` — the agent child then lost its process and exited on a
  signal, producing needle's `exit_code = -1` (`crash`), not `137`, because
  the CLI wrapper died with the killed child rather than reporting its wait
  status.
- Two further attempts (10:21:41, 10:22:09) were lost *before* the crash
  loop to a stale `gc.aggressivewindow='1.hour'` config (git exit 128 ×2),
  fixed at 10:22:18 with `git config --unset` (still unset today — verified).

### 5. Current repository state — remediation holding (verified live 2026-09-02)

`git count-objects -vH` in this workspace: **25 loose objects / 224 KiB,
10,478 in-pack, single 90.18 MiB pack, 0 garbage; `.git` = 92 MB.** The
17.20-GiB hazard that caused the crash no longer exists, and bare
`git gc --aggressive` is prohibited in favour of
`scripts/safe-git-gc.sh` (repo CLAUDE.md; proven 1.1 GB peak RSS).

---

## Why It Took 53 Attempts (contributing causes)

The OOM kill is the root cause of *a* crash; these turned one deterministic
failure into a 64-minute storm and a nine-day alert tail:

1. **Task text prescribed the memory-hazardous command.** The bead body was
   authored as a mitigation *for* repository bloat but mandated the very
   operation that OOMs on a bloated repo.
2. **Naive retry against a deterministic failure.** 44 identical kills
   bought zero progress; auto-split (the decomposition path) only engaged at
   11:59:06Z, ~96 minutes in. A same-cause/same-command kill signature
   should trip a circuit breaker (now: `scripts/crash-circuit-breaker.sh`)
   or split early.
3. **Stale git config consumed the first two attempts** before the storm
   even began.
4. **`bead.orphaned` at 12:58:55Z** released the completed bead unassigned
   instead of closing it, so alert generation kept regenerating — 44
   redundant alert beads, ~20 still open days later. Closed-bead filtering,
   dedup, and cooldown now exist in `scripts/crash-alert-manager.sh`; the
   worker-side half (close with completion notes on success) is the cheap
   remaining fix.

## Debunked Alternatives (carried forward, re-checked)

| Claim | Verdict | Basis |
|---|---|---|
| "gc ran ~57 minutes then was killed" | **False** | No phase-1 attempt survived 116 s (re-measured: max 116.0 s); the 57-min figure is bead creation → attempt #31's alert |
| "External timeout/capacity governor, not OOM" | **Unsupported** | Phase-1 deaths (39–116 s) sit far below the 600 s cap that phase-2 runs visibly survived; a timeout cannot explain the 491.9 s success |
| "OOM impossible — host had 45 Gi free" | **Non sequitur** | Memcg kills need only the scope budget exceeded; all 257 same-period git kills are `CONSTRAINT_MEMCG` with host RAM to spare |
| SIGHUP cascade | **Ruled out** | No `needle cleanup` ran in the window; SIGHUP deaths are sibling-worker-wide, whereas 0/44 kills had any other bead completing within ±3 s across all six same-day worker logs (980 completions scanned) |

## Reproducibility

**Was reproducible: 44/44** — deterministic given (17.20 GiB loose objects +
bare `git gc --aggressive --prune=now` + scope memory budget). Every
identical re-run died identically, in 39–116 s.

**Is not reproducible today**: the repo is 92 MB (the failure precondition
is gone) and the bare command is banned in favour of the staged,
memory-limited, checkpoint/resume `scripts/safe-git-gc.sh`. The *general*
pattern — a monolithic memory-heavy operation inside a `run-*.scope` —
remains live and is the thing decomposition (auto-split) and staged scripts
mitigate.

## Evidence Limits (state these when citing the root cause)

- **No kernel logs survive for 2026-08-14** (current boot began 2026-08-15;
  archived journals start 19:31 EDT that day), no historical memory metrics
  exist (`.beads/logs/resource-metrics.log` begins 09-01), and there is no
  core dump, `.git/gc.log`, or in-log OOM event (needle logs exit codes
  only). There are **no stack traces to analyze** — the process was
  SIGKILLed, which by definition leaves none.
- The Aug-14 root cause therefore rests on: (a) the retry-storm signature
  from the primary event log, triple-derived (Addendum 2, the log review,
  and this dispatch — all concordant); (b) direct kernel evidence of the
  identical mechanism 257 times in the same period (Aug-16 cleanup window);
  and (c) host memory figures that *exclude* host-wide OOM while memcg kills
  demonstrably proceeded. That is inference-plus-corroboration, and it is
  high-confidence — but it is not an Aug-14 `dmesg` line, and this document
  will not pretend otherwise.
- **Capture `journalctl -k` immediately** in future crashes; it does not
  survive rotation.

## Disposition

- **Work: complete.** Repo 18 GB → 753 MB at the time, → 92 MB now; git
  operations restored; verified live 2026-09-02.
- **Residual cleanup (not this crash's cause):** children `bf-5jhvpk`
  (repack) and `bf-im2sl1` (verify) remain Open though subsumed — close
  them, do not re-dispatch.

---

**Determination:** cgroup-scoped (memcg) OOM → SIGKILL of `git gc
--aggressive --prune=now` inside needle's transient memory-limited scope —
INFRASTRUCTURE. No panic, no timeout governor, no domain-check code defect.
Determined 2026-09-02 · bead `domchk-9e2aa740` · all figures in §Verification
independently re-derived for this document.
