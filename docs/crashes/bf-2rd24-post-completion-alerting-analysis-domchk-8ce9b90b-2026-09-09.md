# Post-Completion Crash Alerting — Analysis and Recommendation

**Bead:** domchk-8ce9b90b · **Parent ALERT bead:** bf-2rd24 (target bf-1ea4g)
**Date:** 2026-09-09 · **Verified at HEAD `d6a1f2f`, every command run live this session**
**Task:** review whether alerting should distinguish crashes-during-execution from
crashes-after-completion; determine whether the resulting false positive can be
prevented; state whether configuration changes are needed.

---

## 1. Verdict

**Preventable — and already prevented at HEAD by four independent in-repo layers.**
No new alert-configuration change is required for this false-positive class. The
residual work is not configuration: it is draining the backlog of already-minted
alert beads through the closure layer (owned elsewhere, see §8), plus measuring
the suppression rate so "prevention works" becomes quantified.

One premise in the task is corrected inline: a blanket "post-completion ⇒ no
alert" rule would have been wrong. bf-1ea4g — this very alert's target — is the
counterexample: it was killed *after* its deliverable commit, during `git push`,
by the repository-bloat memcg-OOM regime. Its post-completion crash was a real
infrastructure event, and the alert it raised is the signal that led to the
root-cause investigation. The correct policy is what is now implemented: **alert
on every kill, then classify and suppress automatically** (§5), keeping the
suppressed record.

## 2. How alerts are generated (reviewed)

Alert beads are created by the **needle harness upstream of this repo** at kill
time — title shape `ALERT: Agent crash on bead bf-XXXX`. Nothing in this repo
creates or gates their creation (`docs/alerting-system-guide.md` §1, the
repo-boundary finding of the gap analysis §5 P1). Pre-dedup needle minted one
alert bead *per kill* (bf-4k2ws: 55 alert beads for 55 kills). What this repo
owns is everything after creation: classification, resolution, dedup,
suppression, and triage.

Two properties of the upstream shape matter for this task:

- **The alert body's `Timestamp:` is the bead-creation heartbeat, not the kill
  instant.** The kill instant lives in the worker log / `events.jsonl` and is
  typically seconds-to-minutes earlier.
- **`exit code: -1 (signal -1)` is needle's sentinel for abnormal child death**,
  not a signal number (see §3). The alert body therefore cannot say *which*
  mechanism killed the process.

## 3. Can "signal −1" be detected separately? (yes — it already is)

`exit_code = -1` **is** the detection: every kill-shaped needle record carries
it, so the kill class is separated from ordinary failures by that field alone.
What `-1` does *not* carry is a signal number — SIGKILL is uncatchable and
leaves no stack trace, so mechanism attribution needs a second source:

- `scripts/crash-classifier.sh` reads the exit code from the trace metadata
  (provenance-gated — a single-slot trace may hold a different run) or falls
  back to the event stream, then branches on `-1`.
- `scripts/classify-signal-crash.sh` (Layer 0) distinguishes the historical
  mechanisms at host level; kernel `oom-kill` lines in `journalctl` pin memcg
  OOM (the verified mechanism for the Aug-2026 kill regime, incl. bf-1ea4g's
  push-side kill — `CONSTRAINT_MEMCG` in the 12 GiB dispatch scope).
- Known limit (documented, domchk-3b605127): a kill wave can take the worker
  *before it writes the crash record* (bf-57nao4's stream ends at its fatal
  dispatch), so a missing record is not evidence of no crash — the classifier
  answers `UNKNOWN` there, classified from the trace instead.

## 4. Pattern or isolated incident? (pattern — and it dominates the alert load)

Live census this session from the hourly triage sweep
(`.beads/logs/alert-triage.log`, sweep of 2026-09-09T06:07:32Z):

| Verdict | Count | Meaning |
|---|---|---|
| **RESOLVED_TARGET** | **198** | crash target already closed — post-completion/stale class, no investigation owed |
| FANOUT_DUPLICATE | 14 | older open alert already carries the unresolved target |
| FANOUT_KEEPER | 2 | carries the investigation for its target |
| NEEDS_REVIEW | 2 | the only verdict deserving fresh investigation |
| **Total open alerts** | **216** | across 28 targets |

**92% of currently open alerts point at an already-resolved target.** This is a
structural property of the alert layer, not an isolated incident:

- bf-2rd24 itself is one of **7 sibling alert beads on bf-1ea4g**, all swept
  `RESOLVED_TARGET` (`.beads/state/alert-triage/queue.jsonl`, swept live today).
- The parent bead's own notes (2026-08-26) already dispositioned it: work
  committed at e19739a *before* the crash; bf-1ea4g Closed; no corrective
  action.
- bf-4k2ws's storm shows the same shape at higher amplitude: 55 genuine kills,
  25 of them *post-completion* (after `verification.passed` had landed).
- The corpus census behind the guide found 1,714 alert-shaped beads with 176
  open alerts against closed targets as of 2026-09-07 — the queue drain since
  (216 open today) shows the triage layer + closure blockers working it down.

## 5. Prevention — the layers in force (all verified live this session)

A fresh alert for a resolved/post-completion target is suppressed by **four
independent mechanisms**; a fresh post-completion kill on an *open* target is
suppressed by the classifier's FALSE_POSITIVE branches. Verified today:

1. **Target-closure gate** — `crash-alert-manager.sh` FIX 1/5 checks the crash
   *target's* status before generating anything, including for ALERT beads
   whose own ID is passed in (extended to that shape by efb1603,
   domchk-cd8ec29e; regression coverage `test-closed-bead-filter.sh` — **16/16,
   exit 0** this session, from the repo cwd per its known cwd sensitivity).
2. **Dedup gate keyed on the target** — `alert-deduplication.sh check
   <alert-bead>` resolves the target and returns DUPLICATE when the target is
   closed, has a VERIFIED work-completion marker (G-9), or is already covered
   by an open alert; a 7-day crash-history ledger backs it up
   (`ALERT_TARGET_SUPPRESS_DAYS=7`). **Run live on this alert's own bead:**
   `./scripts/alert-deduplication.sh check bf-2rd24` → `DUPLICATE: crash target
   bf-1ea4g is already resolved`, exit 0.
3. **Classifier FALSE_POSITIVE branches** for kills on open targets —
   deliverable commit within `COMMIT_WINDOW_SEC=30` of the kill (bf-2vtzg
   pattern), bead-closed-after-crash recovery (bf-4k2ws pattern),
   `error_max_turns`, and exit-0 validation (FIX 6). `test-crash-alert-fixes.sh`
   **exit 0** this session; classification wiring fixed at 8cc1172 (anchored
   token match, superseding the banner-string bug domchk-f6fff20f).
4. **Fleet-level coalescing** — 300 s per-classification cooldown + global
   system-event window (f21e381, suppressed alerts now *recorded*), crash-storm
   circuit breaker wired into the dispatch entry point
   `scripts/needle-with-limiter.sh` (23d83f3; defer instead of re-dispatch), and
   the **hourly `domain-check-alert-triage.timer`** running the report-only
   sweep that produced the §4 census (8 timers listed, all future-triggered).

## 6. Are configuration changes needed? (no — knobs already exist and are sane)

No new configuration is required for this false-positive class. The tunables
that exist, all env-overridable with the current defaults verified in the
scripts at HEAD:

| Knob | Default | Where | Effect |
|---|---|---|---|
| `COMMIT_WINDOW_SEC` | 30 | crash-classifier.sh | commit-this-close-to-the-kill ⇒ FALSE_POSITIVE |
| `ALERT_TARGET_SUPPRESS_DAYS` | 7 | crash-alert-manager.sh | fresh alert for same target suppressed within window |
| cooldown window | 300 s | alert-cooldown.sh | per-classification, behind an independent global window |
| breaker threshold / PSI gates | see script | crash-circuit-breaker.sh / system-event-mode.sh | storm deferral at dispatch time |

Deliberately *not* recommended: tightening creation or suppression so that
"post-completion" kills are dropped wholesale. The 30-second commit window is
the right width — a kill seconds after the deliverable commit is almost always
administrative teardown, while a kill *minutes* later during `git push` (the
bf-1ea4g / bf-198ne shape) is real infrastructure and must keep alerting. The
current defaults preserve exactly that distinction.

## 7. Residual gaps (owned elsewhere — not owed by this analysis)

- **Closure of already-minted alerts** — no stack component retires an alert
  bead once its target resolves (alert-lifecycle gap, domchk-f7865662); the 198
  RESOLVED_TARGET queue entries wait on their closure-bead blockers. This is
  why bf-2rd24 is still Open 14 days after its own disposition — stale, not
  false.
- **FP-reduction rate never measured** (domchk-87ef5683) — the suppression
  layers are verified functionally; a counter over `system-event-suppressions`
  + triage verdicts would quantify them.
- **Classifier `UNKNOWN` in the capture-race shape** — proposed automated
  fix in `docs/crash-fix-strategy-domchk-3b605127-2026-09-07.md` §3.2.
- `CODE_DEFECT` has no emission path (documented; consistent with the
  repo-wide zero-defect record).

## 8. Recommendations

1. **Keep post-completion alerting ON; suppress at classification.** Current
   design is correct — do not gate creation on task-completion state (§1
   counterexample) and do not widen suppression past the commit window.
2. **No configuration change now.** Revisit the §6 knobs only if measurements
   (rec. 3) show under/over-suppression.
3. **Measure the suppression rate** — instrument suppressed-alert accounting
   into a weekly count (owner: domchk-87ef5683's FP-measurement item).
4. **Drain the queue** — closure-bead blockers act on the 198 RESOLVED_TARGET
   entries; the hourly sweep already keeps the queue current
   (`.beads/state/alert-triage/queue.jsonl`).
5. **For this family specifically:** bf-2rd24 and its 6 bf-1ea4g siblings are
   all swept RESOLVED_TARGET; close them as stale under the standing
   convention (no investigation owed; the target's own close + the 2026-08-26
   disposition are the citations). This bead's closure does not close them.

## 9. Corrections to the task's premises

| Task premise | Correction |
|---|---|
| "signal -1 (process kill)" | `-1` is needle's exit-code sentinel for abnormal child death, not a signal number; SIGKILL leaves none. Kill-vs-not is separated by the exit code alone (§3). |
| "Timestamp" in the alert body | heartbeat of the alert bead's creation, not the kill instant |
| "post-completion … may not need alerting" | must keep alerting, then auto-suppress — bf-1ea4g's post-commit kill was the era's real infrastructure signal (§1) |
| implicit "isolated incident" | structural: 198 of 216 open alerts are the resolved-target class (§4) |

## 10. Verification record (2026-09-09, HEAD d6a1f2f)

- `systemctl --user list-timers 'domain-check-*'` — 8 timers, incl.
  `domain-check-alert-triage.timer` hourly; sweeps logged 04:07 / 05:07 / 06:07Z.
- `.beads/logs/alert-triage.log` latest sweep — 216 open / 28 targets /
  198 RESOLVED_TARGET (§4 table).
- `.beads/state/alert-triage/queue.jsonl` — 7 bf-1ea4g alerts incl. bf-2rd24,
  all RESOLVED_TARGET.
- `./scripts/alert-deduplication.sh check bf-2rd24` → DUPLICATE, exit 0.
- `./scripts/test-crash-alert-fixes.sh` → exit 0; `./scripts/test-closed-bead-filter.sh`
  → 16 passed / 0 failed, exit 0 (repo cwd).
- Layer sources read at HEAD: `scripts/crash-classifier.sh` (commit-window,
  recovery, exit-code branches), `scripts/crash-alert-manager.sh` (FIX 1/4/5/6,
  breaker, cooldown, classification extraction), `scripts/alert-deduplication.sh`
  (work-completion + resolution legs), `scripts/alert-triage-sweep.sh`,
  `scripts/needle-with-limiter.sh`.
