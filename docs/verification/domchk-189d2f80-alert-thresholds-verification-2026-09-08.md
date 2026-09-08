# Crash-alert thresholds, cooldown and dedup — first-hand validation (domchk-189d2f80)

**Bead:** domchk-189d2f80 (child 3 of 5, split of domchk-b1626933 — "Alert thresholds validated")
**Date:** 2026-09-08, runs at 10:03:22–10:04:44Z (06:03–06:04 EDT)
**Code state:** HEAD `efb1603250a44e6cb747d059442621d7bcabcb60`; shared worktree (dirt disclosed in §6-F4)
**Method:** attempt 2. Attempt 1 (same bead, earlier today ~08:55–09:03Z) left uncommitted raw logs tagged
`*-189d2f80-fresh.log`; every figure below was **re-derived first-hand** in attempt 2 and logged under
`*-189d2f80-r2.log`. Attempt 1's files are retained alongside as corroboration.
**Verdict:** all three acceptance criteria met. Two findings disclosed (§6) — both pre-existing and
documented, neither a regression introduced by this bead.

## 1. Acceptance criteria mapping

| Criterion | Result |
|---|---|
| `test-crash-alert-fixes.sh` 12/12 passing | **13/13, exit 0** — task text was stale by one day: test 13 (functional closed-bead check) was added at `e4fcbec` 2026-09-07; CLAUDE.md already records 13/13 as of 2026-09-07. Count drift = suite growth, not staleness. Passed **twice**: worktree copy and a fresh `git archive HEAD` extract (run from the repo cwd, per the known cwd-sensitivity of the real-`bead show` premise). |
| ≥1 synthetic observation per mechanism (threshold crossing, cooldown, dedup) | Thresholds: §3 (six PSI crossings incl. both exact boundaries, plus MEM/DISK/CPU crossings, plus a live all-OK run). Cooldown: §4 A2/A3b/A3/A5. Dedup: §4 A4/A7/A7b/A6. |
| Evidence committed and pushed (own paths only) | This file is the committed evidence summary. The task named `.beads/state/crash-prevention-testing/alert-thresholds.md` as the record location — that tree is **gitignored by the repo's bloat-prevention layer** (`.gitignore:66`, whole `.beads/`), and CLAUDE.md's prevention rule is that `.beads/` tracked-file count stays 0, so the record was written there *untracked* and this identical summary committed at a tracked path, matching the repo convention (`docs/verification/domchk-6a227734-gc-bounds-verification-2026-09-08.md`, `...-e0e1120e-monitoring-stack-verification-2026-09-08.md`). See §6-F3. |

## 2. Suite results

```
worktree:      Total tests: 13  Passed: 13  Failed: 0   exit 0   (10:03:22Z)
HEAD extract:  Total tests: 13  Passed: 13  Failed: 0   exit 0   (10:03:22Z, extract of efb1603, repo cwd)
```

Tests 1–12 are marker greps over the fix set (FIX 1–6, cooldown, processed-alerts tracking, FP
detection); test 13 runs the manager in a `.beads/state` sandbox against a synthetic exit −1 trace for
the really-Closed bead bf-2vtzg and asserts it is skipped cleanly (rc 0, "already CLOSED", no alert, no
state file). The suite is read-only toward the live store except its own `bead show bf-2vtzg` premise.

## 3. Threshold crossings (resource-monitor.sh, 70/80 PSI + MEM/DISK/CPU)

PSI is read from `/proc/pressure/memory`, which is not env-overridable, so crossings were exercised
against **sandboxed copies** with only the pressure-file path re-pointed at a fixture (one sed line);
a second copy lowered the plain-assignment thresholds to force crossings against the box's live
values. Alert output went to the sandbox's own `.beads/logs/` — nothing wrote to the live alert log
except the real `--once` run in step 4.

```
stock thresholds, PSI avg60 fixture:
  0.00 -> PRESSURE: 0%  [OK]          79.90 -> PRESSURE: 79% [WARNING]
 69.90 -> PRESSURE: 69% [OK]           80.00 -> PRESSURE: 80% [CRITICAL]
 70.00 -> PRESSURE: 70% [WARNING]      85.00 -> PRESSURE: 85% [CRITICAL]
alert lines: "[WARNING] Memory pressure elevated: 70% (>= 70% threshold)"
             "[CRITICAL] Memory pressure critical: 80% (>= 80% OOM threshold)"
boundary semantics: percent = printf "%d" of avg60 (fraction truncates), compared -ge — so 70.00
crosses, 69.90 does not; 80.00 crosses, 79.90 does not. Exact `-ge` at both published thresholds.
```

```
lowered thresholds vs live box values (PSI fixture 0.00 held OK):
  MEMORY_CRITICAL_GB=5->55 : MEMORY: 54GB available [CRITICAL]  (< 55 threshold, -le)
  DISK_WARNING_GB=30->80   : DISK:   62GB free      [WARNING]   (< 80 threshold, -le)
  CPU_WARNING=10->3        : CPU:    8.55 load      [WARNING]   (>= 3 threshold, -ge)
```

```
live, unmodified script, repo cwd (10:03:32Z): rc=0
  MEMORY: 54GB available [OK] · DISK: 62GB free [OK] · CPU: 8.50 load [OK]
  PRESSURE: 0% [OK] · UNSAFE_GC: none [OK]
```

## 4. Cooldown + duplicate detection (crash-alert-manager.sh, sandboxed, `bead` stubbed)

Driver checks **16/16, exit 0**. All state sandbox-local; the stub answers every `bead` call, so the
live store was never touched (create-ledger empty — §7).

| # | Synthetic condition | Observed behavior |
|---|---|---|
| A1 | Genuine exit −1 crash, MemAvailable fixture 4 GiB < 5 GiB floor | Alert generated (rc 1); classification **INFRASTRUCTURE**, reason "Memory exhausted — MemAvailable 4194304kB below 5242880kB floor"; classification read as a bare token (post-8cc1172 anchored-token wiring verified live, not the old banner-string bug); crash history recorded; bead marked processed |
| A2 | Second alert, 0 s later, same classification | Suppressed rc 0 — "Alert cooldown active for INFRASTRUCTURE (0s elapsed, 300s required)": the 5-minute cooldown is **per classification**, not global-only |
| A3b | Per-classification window backdated 400 s, global window fresh | Still suppressed rc 0 — "Reason: global alert cooldown active": a second, independent global cooldown layer sits behind the per-classification one |
| A3 | Both windows backdated 400 s | Alert fires rc 1 — the cooldown is a **window, not a wall** (matches the 2026-09-07 replay lesson: backdating state by more than the window legitimately lets the next alert through) |
| A4 | Re-process an already-processed alert bead | Suppressed rc 0 — "Reason: Already processed this alert bead" (FIX 2/3 self-leg) |
| A5 | `--force-alert` inside the still-open global window | Alert fires rc 1 — force bypasses the cooldown gates |
| A6 | Alert bead (Open) whose crash **target** is Closed | Worktree manager suppresses rc 0 — "Crash target domchk-tgtclosed is already CLOSED". **HEAD `efb1603` suppresses the same input** ("already closed (nothing left to investigate)") — see §6-F2 |
| A7 | Target-keyed dedup ledger: fresh line for target | Suppressed rc 0 — "Duplicate alert for already-processed crash target domchk-tgtopen" |
| A7b | Same ledger line backdated 8 days | No longer suppresses (7-day window expired); proceeds past target dedup into the next gate (cooldown) |

Dedup **fail-open** behavior observed first-hand (full gate trace:
`alert-manager-gate-trace-189d2f80-r2.log`): with the bead store unreadable, the
`alert-deduplication.sh` gate returns 3 → `WARN Duplicate gate returned 3 … failing open` →
`INDETERMINATE: bead store unreadable - failing open (proceed)` → the system-event gate allows →
alert generated. Store-keyed dedup deliberately degrades open; the ledger-based legs (A4/A7) and the
cooldowns are what still hold without the store.

## 5. Crash-classifier categories (sandboxed, stubbed bead, fixture inside a caller-supplied window → PROVENANCE ok)

| Category | Synthetic input | Verdict |
|---|---|---|
| FALSE_POSITIVE | exit 1 + `error_max_turns` trace | FALSE_POSITIVE ✓ |
| FALSE_POSITIVE | exit −1 + bead already Closed (bf-4k2ws pattern) | FALSE_POSITIVE ✓ |
| SERVICE_FAILURE | 503 / "no available server" trace | SERVICE_FAILURE ✓ |
| INFRASTRUCTURE | exit −1 + MemAvailable 4 GiB < 5 GiB floor | INFRASTRUCTURE ✓ (memory reason) |
| INFRASTRUCTURE | exit −1 + .git fixture 600 MiB > 500 MiB | INFRASTRUCTURE ✓ (repo-bloat reason) |
| INFRASTRUCTURE | exit −1, all overrides healthy | INFRASTRUCTURE ✓ ("Abnormal child death (exit -1 sentinel) - bead not recovered") |
| CODE_DEFECT | exit 2 + `panic: runtime error: invalid memory address…` trace | **UNKNOWN** — the classifier cannot emit CODE_DEFECT at all (see §6-F1) |
| UNKNOWN | benign "finished dispatch normally" trace, exit 0 | UNKNOWN ✓ |
| benign real id | `./scripts/crash-classifier.sh bf-2vtzg`, repo cwd | rc **2**, "ERROR: Bead trace not found", no classification emitted, nothing written |

Emitter census over `crash-classifier.sh` at this content: FALSE_POSITIVE 5, SERVICE_FAILURE 1,
INFRASTRUCTURE 6, **CODE_DEFECT 0**, UNKNOWN 2 emitting branches.

## 6. Findings and supersessions

- **F1 — CODE_DEFECT is unreachable (pre-existing, documented, not fixed here).** No branch of
  `crash-classifier.sh` emits CODE_DEFECT, so a panic-shaped trace classifies UNKNOWN and the manager
  treats it accordingly. This is the same UNKNOWN gap CLAUDE.md records ("automated UNKNOWN ≠ no
  crash"; capture races can leave the events log silent), and the proposed automated fix (classifier
  emits INFRASTRUCTURE + mechanism for the silent-events shape) is §3.2 of
  `docs/crash-fix-strategy-domchk-3b605127-2026-09-07.md`. Citation caveat: that strategy doc is
  **untracked in the current worktree and absent from HEAD** (deleted at `e4fcbec`); CLAUDE.md at HEAD
  still references it. Recorded here as an observation for the parent bead's criterion; the fix
  itself is out of this bead's scope.
- **F2 — attempt 1's "gate absent at HEAD" contrast is superseded.** Attempt 1's HEAD extract
  (04:41 EDT) predated `efb1603` (committed 05:43 EDT), so its A6-head check showed the HEAD manager
  fanning out on a closed-target alert. `efb1603` is exactly the FIX 1 target-closure fix, and my
  fresh extract of it suppresses the same input. Worktree and HEAD now agree on this shape; nothing
  to fix. The divergence that remains between them is the co-tenant's in-flight work in F4.
- **F3 — evidence path vs gitignore.** `.beads/state/crash-prevention-testing/alert-thresholds.md`
  was written as tasked but cannot be committed without force-adding into the gitignored `.beads/`
  tree, which would break the repo's standing prevention invariant (`git ls-files .beads | wc -l`
  = 0; the gitignore of `.beads/` is the fix that ended bf-4yjq). The committed deliverable is
  therefore this file; the state-dir record is byte-identical minus this section's pointer.
- **F4 — shared-worktree dirt disclosure (tested as-is).** `crash-alert-manager.sh` in the worktree
  carries 54 uncommitted lines vs HEAD (domchk-* alert-bead target extraction per gap D-4, and a
  time-bounded 7-day target-suppression window — another worker's in-flight domchk-7e5a9cc7 work);
  `crash-pattern-detection.sh` +448, `verify-work-completion.sh` +9, `domain-check-monitoring.service`
  +7 are also dirty. The scripts this bead's criteria actually exercise — `test-crash-alert-fixes.sh`,
  `crash-classifier.sh`, `resource-monitor.sh`, `alert-deduplication.sh` — are **identical to HEAD**.
  Every result above is reported against the content that was actually executed.

## 7. Safety statement (no live bead or store mutation)

- The only live-store reads anywhere: the suite's `bead show bf-2vtzg` premise check (read-only,
  twice — worktree and HEAD-extract runs) and the benign-id classifier run (§5, exits 2 before any
  write).
- No `bead create`, `close`, `update`, `reopen`, or `release` was executed by this bead, and no alert
  bead was created or closed. Every synthetic manager/classifier run used a PATH-stubbed `bead`
  inside a `/tmp` sandbox; the stub's create-ledger is empty (the manager marks existing beads
  processed in its ledger rather than creating anything). Sandboxes were removed after the runs.
- The live `resource-monitor.sh --once` appended its normal one-shot report to
  `.beads/logs/resource-{alerts,metrics}.log` — the same operational logs the 5-minute timer writes.

## 8. Raw evidence index (all under the gitignored `.beads/state/crash-prevention-testing/`)

| File | Content |
|---|---|
| `suite-worktree-189d2f80-r2.log` | Suite 13/13, worktree copy |
| `suite-head-extract-repo-cwd-189d2f80-r2.log` | Suite 13/13, `git archive HEAD` extract run from repo cwd |
| `pressure-thresholds-189d2f80-r2.log` | PSI 70/80 crossings + lowered-threshold MEM/DISK/CPU crossings |
| `resource-monitor-once-189d2f80-r2.log` | Live `--once`, all OK, rc 0 |
| `cooldown-dedup-189d2f80-r2.log` | A1–A7b driver, 16/16, sandbox ledgers (zero creates) |
| `alert-manager-gate-trace-189d2f80-r2.log` | Full-output A1 gate trace incl. dedup fail-open |
| `classifier-categories-189d2f80-r2.log` | Category exercise + emitter census (CODE_DEFECT 0) |
| `classifier-benign-id-189d2f80-r2.log` | Benign-id rc 2 trace-not-found |
| `alert-threshold-driver-189d2f80-r2.sh` | The attempt-2 driver that produced all of the above |
| `*-189d2f80-fresh.log` (attempt 1) | Prior attempt's corroborating logs, same code content |

Key figures are inlined in §2–§5 so this summary stands alone if the gitignored raw logs are
recycled.
