# Crash Prevention Validation Guide

**Created:** 2026-09-08
**Bead:** domchk-82c1ff9a
**Purpose:** Step-by-step procedures for validating that each crash-prevention safeguard
in this workspace actually works — run before citing any "prevention in force" claim in a
report, bead note, or CLAUDE.md section.

**Related:**
[`docs/crash-prevention-requirements.md`](crash-prevention-requirements.md) (the safeguard
inventory, G-1..G-13) ·
[`docs/crash-response-guide.md`](crash-response-guide.md) (triage + per-alert-type
runbooks) · [`docs/alerting-system-guide.md`](alerting-system-guide.md) (alert-layer
architecture) · [`docs/maintenance/repository-maintenance-guide.md`](maintenance/repository-maintenance-guide.md)

---

## Why this exists

Prevention status in this workspace has repeatedly been documented as *ahead of the
mechanisms it describes* — `crash-prevention-requirements.md` §6 records five competing
crash distributions none of which was derived from a counted population, and the M-4
correction in [`docs/comprehensive-crash-prevention-guide.md`](comprehensive-crash-prevention-guide.md)
retracts three status claims that were design targets, not measurements. The rule this
guide encodes:

> **A safeguard is "in force" only on the day you ran its validation.** Cite the command,
> its exit code, and the date. Never cite a previous document's verification as if it
> were current — re-run the smallest applicable check instead.

---

## How to use this guide

- **Layer 1–3 commands are read-only** against the live repo and bead store; safe to run
  anytime, including from a dispatch.
- **Layer 4 suites are hermetic** (staged fakes, temp dirs, private `bead` shims) with the
  two exceptions noted inline: `test-gc-memory-bounds.sh` runs real git gc/push inside a
  disposable 768 MiB cgroup (heavier, ~minutes), and `test-closed-bead-filter.sh` reads the
  real bead store — **run it from the repo cwd**, where the workspace's own `.beads`
  answers (from `/tmp` the ancestor `/home/coding/.beads` workspace answers instead and the
  suite fails 3/7 on environment, not code).
- Exit-code convention across the gate scripts: `0` = clear/pass, `1` = failed/elevated,
  `2` = usage, `4` = blocked-by-breaker, `75` = defer (EX_TEMPFAIL). A suite exit of 0
  with a `Results: N passed, 0 failed` line is the pass signal.
- Suites are added per-bead and often land uncommitted. **Only tracked suites are canon**
 — see [Attribution rules](#attribution-rules-for-failures) before treating a red suite
  as a prevention regression.

---

## Layer 1 — Repository & git-memory bounds

*Guards the crash class that produced bf-4yjq, bf-1s6c3, bf-4x12ec, bf-198ne, bf-1ea4g:
memory exhaustion inside git operations.*

| # | Check | Command | Expected | Live 2026-09-08 |
|---|-------|---------|----------|-----------------|
| 1.1 | Repo health (size, loose objects, packs, backlog) | `./scripts/check-repo-health.sh` | exit 0, `✅ Comprehensive health check complete!` | ✅ exit 0 |
| 1.2 | Effective pack-memory bound (system→global→local) | `./scripts/setup-git-gc-config.sh --verify` | exit 0; worst case ≈3072 MiB, inside the ceiling | ✅ exit 0, ≈3072 MiB |
| 1.3 | Pre-commit hook installed and current | `./scripts/setup-git-hooks.sh --check` | exit 0, `byte-identical to tracked source` | ✅ exit 0 |
| 1.4 | Bead state untracked (the bloat source) | `git ls-files .beads \| wc -l` | `0` | ✅ 0 |
| 1.5 | Bloat absent → gc not needed | `./scripts/auto-gc-trigger.sh --dry-run` | exit 0, `GC not needed` | ✅ exit 0 |
| 1.6 | Unpushed backlog below warning | `./scripts/check-unpushed-backlog.sh` | exit 0, `CLEAR` (0 < 50) | ✅ exit 0, CLEAR |

**Step-by-step (bf-4yjq recurrence drill).** The bf-4yjq trigger was an ~18 GB object
store with an ≈1,800:1 loose:packed ratio. To prove the trigger is absent today:

```bash
du -sh .git .git/objects          # .git < 1GB; objects ≪ 10GB HIGH-RISK line
git count-objects -vH             # loose count/size small vs one consolidated pack
./scripts/check-repo-health.sh    # scripted pass/fail on the same thresholds
```

2026-09-08: `.git` ≈ 104–106 MB, one pack, hundreds (not thousands) of loose objects —
three orders of magnitude below the kill threshold.

**Death-operation replay (the strongest layer-1 evidence).** `test-gc-memory-bounds.sh`
re-runs the actual kill operations of bf-1ea4g (bounded push over an unpacked backlog) and
bf-4x12ec (pack-objects) inside a 768 MiB cgroup — the mechanism that killed them cannot
recur while the pack bounds hold:

```bash
bash scripts/test-gc-memory-bounds.sh      # → 17 passed, 0 failed
```

2026-09-08: push peak RSS 232,416 KB, pack-objects peak RSS 320,532 KB — both under the
700 MiB assertion cap and ~40× below the 12 GiB dispatch scope.

---

## Layer 2 — Monitoring & timers

*Guards detection: without firing timers, every threshold in this workspace is dormant.*

| # | Check | Command | Expected | Live 2026-09-08 |
|---|-------|---------|----------|-----------------|
| 2.1 | All repo timers enabled with future triggers | `systemctl --user list-timers 'domain-check-*' --all` | every row's `NEXT` in the future | ✅ 8/8 future |
| 2.2 | Service monitor (gateway + resources) | `./scripts/service-monitor.sh --once` | exit 0, `PRE-FLIGHT CHECK PASSED` | ✅ exit 0 |
| 2.3 | Resource monitor (pressure / unsafe-gc) | `./scripts/resource-monitor.sh --once` | exit 0, `PRESSURE: … [OK]`, `UNSAFE_GC: none` | ✅ exit 0, pressure 0% |
| 2.4 | Crash pattern detector (surge window) | `./scripts/crash-pattern-detection.sh` | exit 0 (stable) / 1 (elevated) / 2 (event) | ✅ exit 0, 0 actionable |

Notes:
- The timer set is **8**, not the 6 documented pre-2026-09-07: `alert-triage` and
  `auto-gc` timers were added since. Count drift upward = suite/timer growth, not drift
  from the documented baseline; check each row's `NEXT` rather than the count.
- After editing any `~/.config/systemd/user/domain-check-*` unit:
  `systemctl --user daemon-reload` — otherwise the timer silently never fires (this bit
  the weekly full-gc on 2026-09-02).

---

## Layer 3 — Alert pipeline (classification, dedup, cooldown, closed-bead filtering)

*Guards the dominant cost: alerts that point at work another worker already finished.*

| # | Suite | What it proves | Live 2026-09-08 |
|---|-------|----------------|-----------------|
| 3.1 | `bash scripts/test-crash-alert-fixes.sh` | the 2026-09-02 six-fix alert system (closed-bead gate, dedup, cooldown, completion awareness, classification) | ✅ 13/13 |
| 3.2 | `bash scripts/test-closed-bead-filter.sh` | functional replay: a closed target generates **no** alert (repo cwd required) | ✅ 7/7 |
| 3.3 | `bash scripts/test-alert-dedup-check.sh` | dedup gate contract (0 dup / 1 unique / 2 usage / 3 indeterminate-fail-open) | ✅ 41/41 |
| 3.4 | `bash scripts/test-alert-dedup-history.sh` | 7-day crash-history window: record ledger, suppress + reference, expiry, fail-open, manager wiring | ✅ 13/13 |
| 3.5 | `bash scripts/test-alert-triage-sweep.sh` | alert-triage sweep semantics (invoke via `bash` — the tracked file is 100644 by intent; its doc prescribes `bash`) | ✅ 20/20 |

**Live-commands variant** (what the suites assert, exercised against the real store):

```bash
./scripts/crash-classifier.sh <bead-id>            # prints verdict token first, then context
./scripts/alert-deduplication.sh check <alert-id>  # 0=DUP 1=UNIQUE 2=usage 3=INDETERMINATE(fail open)
./scripts/crash-circuit-breaker.sh status          # {"beads": {}} = no open breakers
```

2026-09-08: breaker status `{"beads": {}}` — nothing latched.

**Known ceiling of this layer (do not re-discover it as a bug):** the suite greps check
marker *presence*, not wiring. The `=====` banner/classification wiring bug (domchk-f6fff20f,
fixed by `8cc1172`) passed every marker test while the FALSE_POSITIVE branch was dead code
in production. When a classification claim matters, replay it functionally
(`test-closed-bead-filter.sh` is that replay for FIX 1/5) — and note that nothing in
production invokes `crash-alert-manager.sh` on a timer; the alert *producer* is NEEDLE-side.

---

## Layer 4 — Surge & dispatch gates

*Guards the amplifier: re-dispatching into the same crash (the bf-4x12ec shape — 44
identical kills → 44 alert beads).*

| # | Suite / check | What it proves | Live 2026-09-08 |
|---|---------------|----------------|-----------------|
| 4.1 | `bash scripts/test-system-event-mode.sh` | surge gate: crash-burst + synchronized-exit-wave + PSI signals, exit contract 0/75/4/2, latch + hold | ✅ 32/32 |
| 4.2 | `bash scripts/test-crash-storm-regression.sh` | storm regression: pre-breaker control asserts the unbounded shape fails, breaker bounds it | ✅ 18/18 |
| 4.3 | `bash scripts/test-crash-circuit-breaker.sh` | breaker trip at threshold, dispatch blocked (exit 4), half-open probe, backoff cap, defer, rebuild, 24 h decay | ✅ all pass |
| 4.4 | `bash scripts/test-concurrency-limiter.sh` | dispatch concurrency limiter | ✅ 13/13 |
| 4.5 | `bash scripts/test-needle-with-limiter-gate.sh` | `needle-with-limiter.sh` gates `needle run`/`supervise` through breaker + limiter | ✅ 25/25 |
| 4.6 | `bash scripts/test-preflight-breaker-check.sh` | preflight Check 4 surfaces an OPEN breaker | ✅ 22/22 |

**Live gate state:**

```bash
./scripts/system-event-mode.sh status   # STATE clear = no event latched
./scripts/crash-circuit-breaker.sh status
```

2026-09-08: `STATE clear`, breaker `{}`.

**Deferral drill (what an operator does when a storm *is* active):**

```bash
./scripts/system-event-mode.sh check || ec=$?
# ec=0  → proceed; ec=75 → defer new heavy work until the hold expires
# ec=4  → alert-suppressed (gate says a suppression window owns this alert)
```

---

## Layer 5 — Integration verification (the wiring between safeguards)

Individual scripts passing is necessary, not sufficient: the safeguards only compose if
the *call edges* exist. Each edge below has a tracked suite that fails if the wiring is
removed — run them together and any integration regression localizes:

| Edge | Wiring | Asserted by | Live 2026-09-08 |
|------|--------|-------------|-----------------|
| dispatch → breaker | `needle-with-limiter.sh` consults `crash-circuit-breaker.sh` before `needle run`/`supervise` | `test-needle-with-limiter-gate.sh` 25/25 | ✅ |
| dispatch → concurrency cap | same script wraps the limiter | `test-concurrency-limiter.sh` 13/13 | ✅ |
| preflight → breaker | `preflight-health-check.sh` Check 4 queries the breaker | `test-preflight-breaker-check.sh` 22/22 | ✅ |
| preflight → surge gate | `preflight-health-check.sh` defers (75) on an active event | `test-system-event-mode.sh` 32/32 | ✅ |
| repo-health → backlog telemetry | daily 02:00 `check-repo-health.sh` runs `check-unpushed-backlog.sh` | `test-unpushed-backlog-wiring.sh` 11/11 | ✅ |
| alert manager → dedup gate → ledger | `crash-alert-manager.sh` checks then records via `alert-deduplication.sh` | `test-alert-dedup-history.sh` 13/13 | ✅ |
| gc/push → memory bound | persistent git config bounds *every* pack-objects, bare or scripted | `test-gc-memory-bounds.sh` 17/17 + `test-setup-git-gc-config.sh` 34/34 | ✅ |
| nightly remediation → bound | 03:00 unit runs `safe-git-gc.sh` under `MemoryMax=4G` | `test-safe-git-gc-limits.sh` 33/33 | ✅ |

**End-to-end sequence drill** (read-only; proves the layers line up in one pass):

```bash
./scripts/preflight-health-check.sh          # 5/5 — box + repo + breaker
./scripts/system-event-mode.sh check         # 0 — dispatch permitted
./scripts/crash-circuit-breaker.sh status    # {} — nothing latched
./scripts/check-repo-health.sh               # 0 — no bloat, no backlog
systemctl --user list-timers 'domain-check-*' --all   # all NEXT future
```

2026-09-08: all five green in sequence — a dispatch admitted at Layer 4 would land on a
repo that Layer 1 measures healthy, with Layer 2 monitors armed and Layer 3 dedup live.

---

## The one-command battery

Everything above, in run order (tracked suites only; ~5–10 min, dominated by
`test-gc-memory-bounds.sh`):

```bash
cd /home/coding/domain-check

# Layers 1–2: live health (read-only)
./scripts/check-repo-health.sh && ./scripts/setup-git-gc-config.sh --verify \
  && ./scripts/setup-git-hooks.sh --check && ./scripts/auto-gc-trigger.sh --dry-run \
  && ./scripts/check-unpushed-backlog.sh && [ "$(git ls-files .beads | wc -l)" = 0 ] \
  && ./scripts/service-monitor.sh --once && ./scripts/resource-monitor.sh --once

# Layers 3–5: hermetic suites
# NOTE: `test-crash-pattern-detection` was listed here on the first 2026-09-08 run but
# is UNTRACKED (not canon — attribution rule 1); it was dropped from the battery by the
# closing-attempt re-run rather than silently adding an untracked suite to the count.
for t in test-crash-alert-fixes test-alert-dedup-check test-alert-dedup-history \
         test-system-event-mode test-crash-storm-regression \
         test-crash-circuit-breaker test-concurrency-limiter test-needle-with-limiter-gate \
         test-preflight-breaker-check test-safe-git-gc-limits test-setup-git-gc-config \
         test-setup-git-hooks test-check-unpushed-backlog test-unpushed-backlog-wiring \
         test-verify-work-completion test-cgroup-memory-guard; do
  bash scripts/$t.sh >/tmp/val-$t.log 2>&1 && echo "PASS $t" || { echo "FAIL $t"; tail -5 /tmp/val-$t.log; }
done

# Heavy but decisive: the death-operation replay
bash scripts/test-gc-memory-bounds.sh

# Repo-cwd-sensitive functional replay (run LAST, from the repo root)
bash scripts/test-closed-bead-filter.sh
bash scripts/test-alert-triage-sweep.sh
```

2026-09-08 result: **every command exit 0** — 13 live checks + 19 suites, 0 failures
(first run; the 19 included the then-untracked `test-crash-pattern-detection`). The
closing-attempt re-run (see verification record) keeps 19 suites with that one replaced
by a tracked-only count. **Run the suites on an idle box**: `test-safe-git-gc-limits`
and the e2e scenarios in it execute the real `safe-git-gc.sh`, whose fail-fast preflight
defers (exit 1/2) when 1-minute load exceeds 15 — a batch of suites running concurrently
can push load past that and fail scenarios that pass on an idle box.

---

## Attribution rules for failures

A red result is a prevention regression only after attribution. In this shared worktree:

1. **Tracked vs on-disk.** At 2026-09-08, 26 of the 52 `scripts/test-*.sh` files on disk
   were tracked at HEAD; the other 26 are in-flight sibling work, uncommitted. Validate
   with `git ls-files scripts/test-*.sh | wc -l` vs `ls scripts/test-*.sh | wc -l`, and
   only treat **tracked** suites as canon. (Worked example: `test-system-event-consumers.sh`
   was untracked on 2026-09-08 and failed 4/36 on its own stale assertions — e.g. expecting
   `test-crash-alert-fixes.sh` to still report "12 tests" after that suite grew to 13.
   Not a HEAD regression; excluded from the battery.)
2. **Worktree vs HEAD.** Scripts under active edit (`git status`) may differ from HEAD.
   Prove attribution with a HEAD extract:
   `git archive HEAD scripts | tar -x -C /tmp/h && bash /tmp/h/scripts/<suite>.sh`.
   If HEAD passes and the worktree fails, the failure belongs to the in-flight edit, not
   to prevention.
3. **Environment vs code.** `test-closed-bead-filter.sh` from a `/tmp` extract fails 3/7
   by construction (the ancestor `~/.beads` workspace answers instead). Re-run from the
   repo cwd before investigating. Suites also grow over time — a count mismatch against an
   old document ("said 12/12, got 13/13") is growth, not staleness. Two live-counter
   sensitivities are known and are not code defects:
   - **Load floor.** `test-safe-git-gc-limits.sh`'s e2e scenarios run the real
     `safe-git-gc.sh`, which defers at 1-minute load > 15. Batch-running many suites at
     once can trip that (observed 2026-09-08: 7 e2e failures under a load spike; 33/33 on
     an idle-box re-run minutes later). Check `uptime` before attributing.
   - **Margin-of-1 thresholds.** Its threshold-floor cases set
     `SAFE_GC_MIN_AVAIL_MEM` to live `MemAvailable` **+1 MiB**; if availability rises
     between the test's read and the script's check (page-cache churn on a busy box), the
     floor lands *below* availability and the script correctly proceeds (rc=0). Observed
     once 2026-09-08, sandwiched between two 33/33 runs. Re-run before attributing.
4. **Gate exit codes are verdicts, not crashes.** `safe-git-gc.sh --check-only` exit 1 can
   be the "GC not needed" verdict; `alert-deduplication.sh check` exit 0 means
   *duplicate → suppress*. Read the contract before paging anyone.

---

## What this battery does NOT validate

Honest scope, so nobody cites it for more than it proves:

- **NEEDLE-side behavior** — the retry-loop stop-condition (H-1: does anything check
  whether the last N attempts died identically *before* re-claiming?), per-scope memory
  telemetry (M-2), and complexity-aware turn budgets (G-12) live outside this repo and have
  no suite here. Detection without an actor is not prevention.
- **Production invocation of the alert manager.** Nothing in production calls
  `crash-alert-manager.sh` on a timer or hook; the suites prove it works, not that it runs.
- **Gateway failover (G-4) and the prevention feedback loop (G-5)** — still open
  requirements in `crash-prevention-requirements.md` §4; there is nothing to validate yet.
- **Real storm conditions.** `test-crash-storm-regression.sh` and the death-op replays
  are reconstructions under synthetic cgroups. They prove the bound, not fleet behavior
  during an actual event.
- **Timestamps in logs** are not validated to be UTC; the journald/`git log` local-time
  trap (EDT vs UTC crash stamps) remains a manual discipline.

---

## Verification record

| Date | Scope | Result |
|------|-------|--------|
| 2026-09-08 | Full battery — 13 live checks + 19 tracked suites (Layer 1–5) by domchk-82c1ff9a at HEAD `b07ac79` | **all green**; details per layer above |
| 2026-09-08 | Full battery re-run by domchk-82c1ff9a's closing attempt at HEAD `60ecf28` (dispatch base; the bead's docs commit ships on top) — 13 live checks + 19 tracked suites, `test-crash-pattern-detection` dropped from the battery as untracked | **all green after attribution**: 7 `test-safe-git-gc-limits` e2e failures were a >15 load spike from the batch itself (33/33 on idle-box re-run); one later 32/33 run hit the margin-of-1 MemAvailable race (attribution rule 3); preflight exit 1 once immediately post-heavy-suite, 5/5 on two subsequent runs. Everything else first-pass green. |
| 2026-09-09 | Full battery re-run by domchk-309f49bd (verify leg of the bf-2vtzg memcg-OOM chain; sibling implement-fix domchk-32c54537 closed 08:07Z the same morning) at HEAD `c28345d` (dispatch base; this row's commit ships on top) — 13 live checks + 19 tracked suites, sequential single-batch run at load 5–6 | **all green after attribution**: `test-safe-git-gc-limits` first pass 32/33 on the documented margin-of-1 MemAvailable race (rule 3, scenario [8] memory threshold), 33/33 on immediate re-run. Death-op replay 17/17 — push peak RSS 232,472 KB, pack-objects peak RSS 320,524 KB, both < the 700 MiB assertion cap (~40× under the 12 GiB scope that killed bf-2vtzg). `test-closed-bead-filter` now 16/16 (was 7 in the 2026-09-07 record — suite growth, not drift). Live state: repo 104 MB / 1 pack / 14 loose / 0 unpushed, bound ≈3072 MiB, breaker `{}`, event-mode `clear`, preflight all-pass, 8/8 timers future. |

Re-run the battery (or the smallest applicable layer) before citing prevention status
anywhere. Update the row above with your date, scope, and bead.
