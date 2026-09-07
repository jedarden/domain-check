# Root Cause Determination: bf-173o7e crash — investigation bead `domchk-2e371a2c`

| Field | Value |
|---|---|
| **Subject bead** | `bf-173o7e` — "Execute git gc --aggressive with pruning" (created 2026-08-14T12:57:54Z, closed 2026-08-17T17:15:23Z) |
| **Crash event** | Aug-14 2026 `exit −1` retry storm: 12:58:58Z → 23:25:35Z |
| **Root cause** | Kernel **memcg OOM SIGKILL**: bead-prescribed `git gc --aggressive --prune=now` over 17.20 GiB of loose objects exhausted needle's 12 GiB `run-*.scope` (`MemoryMax=12 GiB`, `oom_score_adj=200`); the kernel killed the highest-badness task — the agent process itself — and needle recorded the sentinel `exit −1` |
| **Failure mode** | INFRASTRUCTURE — memory-cgroup resource exhaustion. Not a code defect, not a timeout, not a service failure |
| **Confidence** | **HIGH** (see §5) |
| **Determination by** | `domchk-2e371a2c`, 2026-09-02 — consolidates and re-verifies `domchk-17ca8b7d` (storm RCA, commit 46f0360) and `domchk-858fecb1` (verification report, commit db1acb3) |

---

## 1. Causal chain

1. `bf-173o7e`'s body prescribed the exact command `git gc --aggressive
   --prune=now` against a workspace holding **17.20 GiB loose objects**
   (9.6 MiB packed) — verified from the bead text (`bead show bf-173o7e`).
2. `git gc --aggressive` computes delta chains across the whole object set
   in memory before writing pack bytes; with no `pack.windowMemory` bound in
   place at the time, pack-objects RSS grew unbounded.
3. Each dispatch ran inside needle's transient `run-*.scope` capped at
   **12 GiB** with `oom_score_adj=200` — the scope budget was exhausted in
   tens of seconds to ~3.5 minutes.
4. The kernel's memory-cgroup OOM killer SIGKILLed the highest-badness task
   — on Aug-14 typically the **agent process**, not the git child, which is
   why needle recorded `exit −1` instead of a git exit code.
5. The retry layer re-dispatched the identical, identically-doomed task
   **132 times** (12:58:58Z–23:25:35Z): 131 completions = **129 × exit −1**
   (21.6–216.6 s) + **1 × exit 124** (607.6 s — the 600 s cap) + **1 × exit 0**
   (40.1 s, 23:25:35Z), then `bead.orphaned` — success went unrecognized and
   the bead was left for manual closure. Nothing bounded the loop.

## 2. Supporting evidence (primary sources; re-verified live 2026-09-02 by this bead)

- **Storm counts re-derived from the primary needle log** this session:
  `grep -c agent.dispatched | grep bf-173o7e` on
  `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl` →
  **132 dispatches / 131 completions**, exit-code histogram
  **129 × `exit_code:-1`, 1 × `0`, 1 × `124`** — matches
  [`../crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md`](../crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md)
  exactly (including the one dispatch with no recorded completion — session
  a6dbb1fc died at `transform.started`, resumed under a6eaf955).
- **Prescribed command is in the bead body** — "Run `git gc --aggressive
  --prune=now` to pack 17.20GB of loose objects" (`bead show bf-173o7e`).
  The crashing process was the agent scope running that command: **zero
  domain-check code involvement.**
- **`exit −1` is a sentinel, not a signal number** — needle records
  `ExitStatus::code().unwrap_or(-1)`, i.e. "died on a signal needle did not
  send" ([`../analysis/signal-analysis.md`](../analysis/signal-analysis.md)).
  The signal was SIGKILL from the memcg OOM killer.
- **Mid-gc death proof** — the 13:55:24.357Z kill's agent transcript ends
  with *"The git gc process is running successfully. Let me wait a bit
  more"* + a `sleep 10 && tail` progress check; SIGKILL arrived 10.7 s later
  (commit 9d374d1). Kills landed 21.6–216.6 s into runs — mid-operation,
  not at any timeout boundary.
- **Kernel-verified identical mechanism in the same period** — 257
  `task=git` `CONSTRAINT_MEMCG` kills on Aug-16 with anon-rss median
  12,301,364 kB hugging the 12 GiB cap; **all 424 OOM kills this boot are
  `CONSTRAINT_MEMCG`, zero host-OOM kills** (bf-4x12ec-root-cause.md §2).
- **Alert timestamps are `HANDLING_RELEASE_DONE` heartbeats**, trailing the
  real `agent.completed` kills by ~7.7–120 s (five mapped for this storm,
  e.g. assigned 14:21:34.162Z ← real kill 14:21:15.779Z, attempt #50). Every
  bf-173o7e alert must be re-derived from `agent.completed`, never from the
  assigned timestamp.

## 3. Triggering conditions

1. **Unbounded gc command in bead text** — no `pack.windowMemory` /
   `pack.threads` bound existed (fixed 2026-09-02, see §6).
2. **Pathological repository state** — 17.20 GiB loose objects from the
   earlier crash debris (bf-1s6c3 lineage).
3. **Constrained dispatch scope** — 12 GiB memcg far below the worst-case
   footprint of an unbounded `--aggressive` repack of that object set.
4. **Retry loop with no circuit breaker and broken success handling** —
   exit-0 went unrecognized (`bead.orphaned`), so a doomed task was retried
   131 times, producing the alert storm that then spawned duplicate/stale
   alert beads for the next two weeks.

## 4. Alternatives ruled out (false positives)

| Candidate | Verdict | Basis |
|---|---|---|
| SIGHUP cascade / fleet-wide event | ❌ | At each kill exactly one agent died; nearest sibling completions exited 0 |
| Timeout governor | ❌ | 129 kills at 21.6–216.6 s sit far below the 600 s cap one surviving run visibly hit (exit 124 at 607.6 s). Supersedes the "timeout monitor" guess in `bf-173o7e-false-positive.md` |
| "git finished in background, agent killed after completion" | ❌ (superseded) | Transcript proof shows gc actively running seconds before kills; completion came later from the exit-0 storm attempt |
| Host-wide OOM | ❌ | ~45 Gi host RAM free mid-storm; all boot OOM kills are `CONSTRAINT_MEMCG` |
| domain-check code defect | ❌ | Crashing process was the agent scope running the bead's git command |
| Aug-17 `exit 1` / `max_turns` event | different event | Post-completion bead-close exhaustion, analyzed separately in `bf-173o7e-root-cause.md`; its "exit 1, NOT −1" statements apply only to Aug-17 |

**At the alerting level** this chain *is* a false positive in the narrow
sense that the work objective was achieved (repo fully packed by storm end,
bead closed 2026-08-17, repo healthy since) — but the kills were real. This
investigation bead (`domchk-2e371a2c`, created 2026-08-26T22:38Z) is itself
an artifact of that stale-alert regeneration: twelve days post-crash, nine
days post-closure, against an already-closed bead. Fixed by closed-bead
filtering, dedup, and cooldown in `scripts/crash-alert-manager.sh`.

## 5. Confidence: HIGH

- The event-log signature was re-derived from the primary needle log this
  session and matches the prior determination exactly.
- The exit-code pattern (dozens of kills at 21.6–216.6 s + one 124 at
  exactly the 600 s cap + one 0) is deterministic evidence of a
  resource-bound kill loop, not transient blips.
- The identical mechanism is kernel-confirmed in the same period (Aug-16
  `task=git` `CONSTRAINT_MEMCG` kills hugging the 12 GiB cap).
- **Residual uncertainty:** Aug-14 kernel logs are rotated away (current
  boot began 2026-08-15 19:26 EDT), so the specific Aug-14 kills carry no
  direct kernel OOM record — the mechanism is established by corroboration,
  not by a per-event kernel line. SIGKILLed dispatches also leave no
  `.beads/traces/` entries. Neither gap can close retroactively; both are
  documented in the storm RCA §6.

## 6. Disposition and mitigations (verified live 2026-09-02 by this bead)

- **Work: complete; no retry needed.** Repository healthy — 10,478 in-pack
  objects, single complete 90.18 MiB pack, `.git` 93 MB, `git fsck` exit 0.
  (vs 17.20 GiB loose / 9.6 MiB pack at crash time.)
- **Mechanical fix shipped and verified:** `pack.windowMemory=2g`,
  `pack.deltaCacheSize=1g`, `pack.threads=1` bound worst-case pack memory to
  ≈3 GiB — `./scripts/setup-git-gc-config.sh --verify` → ✅ within the
  ceiling for a 12 GiB scope (exit 0). Bare `git gc --aggressive` is banned
  in favour of `scripts/safe-git-gc.sh`; `scripts/crash-circuit-breaker.sh`
  covers same-cause kill loops.
- **Zero git memcg kills since Aug-16.**

## References

- Storm RCA: [`../crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md`](../crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md)
- Verification report: [`../crash-reports/bf-173o7e.md`](../crash-reports/bf-173o7e.md)
- Mechanism detail (shared with bf-4x12ec): [`../crash-investigations/bf-4x12ec-root-cause.md`](../crash-investigations/bf-4x12ec-root-cause.md)
- Attempt #38 exit-state analysis: [`../crash-investigations/bf-173o7e-attempt-38-log-exit-state-analysis-domchk-a0f2c805-2026-08-14.md`](../crash-investigations/bf-173o7e-attempt-38-log-exit-state-analysis-domchk-a0f2c805-2026-08-14.md)
- Signal sentinel decode: [`../analysis/signal-analysis.md`](../analysis/signal-analysis.md)
- Maintenance guide (memory bounds): [`../maintenance/repository-maintenance-guide.md`](../maintenance/repository-maintenance-guide.md)

---

**Determination (bead `domchk-2e371a2c`, 2026-09-02):** memcg OOM → SIGKILL
inside the 12 GiB dispatch scope → needle `exit −1` sentinel, ×129 attempts
of a duplicate-gc-bead retry storm. INFRASTRUCTURE. Confidence HIGH. Work
complete, bead correctly closed 2026-08-17, no further action.
