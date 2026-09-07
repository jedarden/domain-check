# Recommendations & Prevention Verification: bf-1s6c3 (alert bf-1atrl)

**Date:** 2026-09-07
**Bead:** domchk-a18b2c06 — "Generate recommendations and preventive measures"
**Chain position:** domchk-0d35f6e5 (root cause, closed) → **this bead** → bf-1atrl (alert umbrella)
**Subject:** bf-1s6c3 — "Create merge commit reconciling Forgejo and GitHub histories"
**Status:** ✅ COMPLETE — subject closed 2026-08-16, all six in-repo prevention layers verified in force today
**Classification:** Infrastructure — repository bloat (crash-response-guide Pattern 3)

> **Authority note.** This report does not re-derive the analysis. The root cause and its
> evidence live in `docs/crash-analysis-bf-1s6c3-2026-09-06.md` (§5 classification, §6 RCA,
> §8 prevention layers, §11 corrections to prior docs); the workspace-wide requirements and
> gap list live in `docs/crash-prevention-requirements.md`. This report verifies — first-hand,
> today — that those layers are actually in force, states the recommendations in priority
> order, maps prevention to crash type, and records which older documents must not be cited
> forward. Every check in §5 was re-run by this bead on 2026-09-07, not copied.

---

## 1. Investigation summary

The bf-1s6c3 "crash" was **one undrainable infrastructure condition executed 76 times**: a
kernel memcg-OOM SIGKILL of `git push`'s pack-objects on an ≈18 GB repository, re-run by an
automated ~10 s re-dispatch loop for 4 h 30 min.

| Fact | Value | Source |
|---|---|---|
| Dispatch attempts | 76 (2026-08-12T21:31:27Z → 08-13T02:01:22Z) | §3 of the canonical report |
| Exit codes | **−1 × 71** · 124 × 4 · **0 × 1** | recounted first-hand today from `docs/crashes/bf-1s6c3/needle-events-2026-08-1{2,3}-bf-1s6c3.jsonl` (945 + 513 lines) |
| Where the deaths happened | **71 of 76 died with a `git push` as their last issued command** | `sessions-index.tsv` last-command table, §4.2 |
| Alerts raised | **71** — one per kill, for one cause | `outcome.handled` in the extracts |
| Work lost | **None** — merge `42a7b07` landed mid-storm; `main`'s reconciliation is `46293c5`; remotes converged | §2, §4.4 |
| Compute wasted | ≈5.5 h of agent retries producing zero pushed bytes | §7 |

---

## 2. Root cause and contributing factors

Three stacked layers (canonical §6) — prevention must address each one, not just the first:

1. **Immediate** — pack-objects for an ≈18 GB object store inside the dispatch scope's
   `MemoryMax=12GiB`. The kernel killed the worker mid-attempt; needle recorded the
   unrecorded-code deaths as `exit -1`.
2. **Amplifying** — the crash handler released and re-claimed the bead on a ~10 s cycle:
   **no backoff, no resource gate, no stop-condition for satisfied work.** The deliverable
   had already landed 59.6 s into attempt 4, yet 72 further dispatches re-ran a memory-doomed
   push. This layer is what turned one kill into 71 kills and 71 alerts.
3. **Underlying** — 17+ identical ~237 MB `.beads/*.jsonl` bead-state snapshots committed to
   git, bloating the object store 36× past the healthy band.

Contributing factors worth naming because each maps to a control below: bead state was
ever trackable in the first place; no pre-flight checked repository size before significant
git work (the host `free -g` gate would not have caught this — the violated axis was
repository size, not host memory); nothing bounded pack memory for bare `git push`, only
the safe-gc wrapper path; and four 600 s timeouts at the storm's tail show the loop also
ground on under load.

---

## 3. Recommendations with priorities

### 3.1 In force and re-verified today — keep, and keep verifying

| Layer | Control | Verified 2026-09-07 |
|---|---|---|
| Object store | Packed and holding: `.git` 102 MB, 3 packs / 99.13 MiB, 78 loose objects / 544 KiB, **0 garbage** | ✅ this bead |
| Re-entry block | `.gitignore` `.beads/` + `*.db` + `*.jsonl`; `git ls-files .beads` → **0** | ✅ this bead |
| Commit-time backstop | Pre-commit hook (10 MB per-file / 50 MB per-commit caps, hard block on staged `.beads/`); `setup-git-hooks.sh --check` → installed hook byte-identical to tracked source | ✅ this bead (closed G-1 as `dfa60a9`) |
| Pack memory bound | `pack.windowMemory=2g`, `pack.threads=1`, `pack.deltaCacheSize=1g` — effective chain resolves clean; worst case ≈3072 MiB vs the 12 GiB scope | ✅ this bead (`--verify` exit 0) |
| Scheduled maintenance | 6 daily/weekly systemd user timers incl. the 03:00 bounded gc under `MemoryMax=4G` | ✅ this bead (§5 — now **7**, see below) |
| Bounded remediation path | `scripts/safe-git-gc.sh` (never bare `git gc --aggressive`) | ✅ standing procedure |

### 3.2 Open — this repository

| Priority | Item | Gap | Note |
|---|---|---|---|
| P1 | Retire or shim cron-based `scripts/monitoring-setup.sh` — this box is NixOS with no crontab; it is a documented silent no-op | G-7 | 0.5 h; removes a trap, adds no behavior |
| P1 | Failover-aware gateway check + an **enforced** preflight-before-dispatch point (the convention is documented; nothing enforces it) | G-4 | `scripts/gateway-failover.sh` confirmed absent today |
| P2 | Weekly prevention feedback loop (threshold review against monitoring logs) | G-5 | `scripts/crash-prevention-feedback.sh` confirmed absent today |
| P2 | Evidence retention: ≥30-day kernel/journald retention, rotated traces, UTC-only timestamps, keep `gc.log` | G-8 | Cheap and prevents the next investigation from reconstructing |

~~G-3 (system-event surge gate)~~ — **closed** since the requirements doc was written:
`scripts/system-event-mode.sh` landed as `e0fab45` (domchk-84d48411, 32/32 tests) and was
exercised live by this bead (STATE clear, event source fresh, exit 0). Dated note appended
to `docs/crash-prevention-requirements.md` §4 G-3 by this bead.

### 3.3 Open — NEEDLE / infrastructure side (outside this repo)

| Priority | Item | Why it matters for *this* crash |
|---|---|---|
| **P0** | **Re-dispatch stop-condition for satisfied work** — "deliverable present, bead still open" must drain before the next dispatch, plus backoff and a resource gate | This is the amplifier (§2 layer 2): it converted one kill into 71 and 71 alerts. It is the single highest-leverage fix for this crash type and lives entirely outside this repository |
| P0 | Alert producer consults work-completion markers + a post-completion grace period before raising an alert | 72 of 76 dispatches ran against already-satisfied work; `verify-work-completion.sh` writes the marker today, but nothing that raises alerts reads it |
| P1 | Retry with backoff on 502/503; complexity-aware turn budgets | Service-failure and max-turns classes (G-11/G-12) |
| P2 | Dispatch-scope sizing policy for memory-heavy work (G-10); load-based throttling (G-13) | CPU-saturation class (bf-xumcu: 826 crashes in a day) |

---

## 4. Preventive measures mapped to crash type

From the crash-type table in `docs/crash-prevention-requirements.md` §1; status verified live
2026-09-07 unless noted.

| Crash type | Preventive measure | Status |
|---|---|---|
| **Repository bloat → OOM** (this crash, bf-4yjq) | gitignore re-entry block + pre-commit size gate + repo-size pre-flight table + safe-gc path | ✅ all four verified today; no recurrence since 2026-08-16 |
| **memcg OOM of a git process in the dispatch scope** (bf-4x12ec, bf-198ne) | `pack.windowMemory`/`threads`/`deltaCache` bounds on gc **and** push; nightly gc under `MemoryMax=4G` | ✅ verified today |
| **Crash-surge / system-wide event** (2026-08-16 cascade) | `crash-pattern-detection.sh` (3-in-5-min) **+ `system-event-mode.sh` gate with exit-75 deferral convention** | ✅ gate live today (G-3 closed) |
| **Post-completion agent death** (bf-173o7e) | `verify-work-completion.sh` marker files for triage | ⚠️ marker exists; the *alert producer* still doesn't consult it (G-9, open) |
| **`error_max_turns` workflow failure** | `bead-split-recommender.sh` guidance; turn budgets are NEEDLE config | ⚠️ partly (G-12 open) |
| **Inference gateway unavailable** (domchk-c9641ac5) | `service-monitor.sh` every 2 min (use `curl -skf` — self-signed cert); retry snippet in CLAUDE.md | ⚠️ detection yes; failover/enforcement no (G-4 open) |
| **CPU saturation mass-crash** (bf-xumcu) | `resource-monitor.sh` warnings at 2.0× normalized load | ⚠️ warnings only, no throttling (G-13 open) |
| **Duplicate / false-positive alerts** (bf-173o7e: 129 dups) | `crash-alert-manager.sh` closed-bead filter, dedup, 5-min cooldown; `crash-classifier.sh` | ✅ **all tests passing today**; source-side suppression still NEEDLE-side (G-9) |

**Coverage reading:** the two classes with hard kernel kills (bloat, in-scope OOM) are the
two this repository fully owns, and both are closed-loop verified. The remaining exposure is
concentrated in alert quality and NEEDLE dispatch policy, not in anything this repo's code
or config can reach.

---

## 5. Monitoring and alerting improvements

**Verified in force today** (all re-run by this bead):

- 7 `domain-check-*` systemd user timers, every one with a future trigger time — the six
  long-standing units plus a **new** `domain-check-auto-gc.timer` (daily 02:30) that runs
  `safe-git-gc.sh --auto-when-needed` threshold-triggered. Its unit files are present in the
  worktree but **not yet committed** — in-flight work by a co-tenant bead; the G-2
  "deliberately not implemented" note in `docs/crash-prevention-requirements.md` therefore
  needs a dated update once that bead lands. The 02:30 conditional run complements rather
  than replaces the 03:00 unconditional bounded gc, so the G-2 objection (conditional gating
  blind to intra-pack bloat) does not apply to the pair.
- `scripts/system-event-mode.sh` — surge gate answers live, event source fresh (G-3 closed).
- `scripts/test-crash-alert-fixes.sh` — "All tests passed!" on today's run.
- Host: 45 G memory available, 56 G disk free, load 5.56 — healthy.
- Convergence: `HEAD...origin/main` = 1/0; the 1 is a co-tenant's docs-only commit
  (`d2649e8`), not divergence.

**Improvements still needed** (the open recommendations of §3, restated as monitoring asks):

1. **Close the loop from marker to alert** — the biggest measurable win. 71 alerts were
   raised for one undrainable cause here; the false-positive class is 60–75% of alert volume
   workspace-wide. The marker files already exist; the producer must read them (G-9).
2. **Make preflight a gate, not a suggestion** — preflight failure must defer the bead at
   the dispatch point, and the check must include repository size (the axis that actually
   killed bf-1s6c3), not only host memory (G-4).
3. **Feed thresholds back from data** — a weekly review job over `.beads/logs/*` so
   prevention tracks the fleet's current signature instead of August's (G-5).
4. **Retain evidence** — 30-day kernel/journald retention, rotated traces, UTC-only
   timestamps (G-8). Every correction in §11 of the canonical report exists only because
   better evidence surfaced late.

---

## 6. Documentation gaps identified and addressed

Three older documents state a root cause this investigation disproved. Each now carries a
dated supersession banner pointing at the corrected record (added by this bead); keep them
for provenance, do not cite them forward.

| Document | Superseded claim | Reality |
|---|---|---|
| `docs/verification/crash-fix-verification-report-bf-1s6c3-2026-09-01.md` | One crash at 23:41:46Z, "SIGKILL signal 9", "<2 GB available → OOM killer" | 76 dispatches / 71 kills over 4.5 h; no host memory figures exist for Aug-12 (journald starts 2026-08-15); the binding constraint was the 12 GiB dispatch scope, not host memory |
| `docs/crash-investigations/bf-1atrl-crash-investigation.md` | "Agent timeout (600s) … no OOM condition, pure timeout issue" | Only 4 of 76 attempts timed out; 71 died by signal at the push step on a bloated repo |
| `docs/crash-investigations/bf-1atrl-verification-report.md` | Same timeout attribution; single-crash framing | Same correction |

Also recorded, without editing (their owning beads are responsible):

- `docs/systemic-crash-prevention-recommendations-2026-09-02.md` — predates the 76/71 census
  correction and the raw-log extraction; its recommendation #1.3 (`--auto-when-needed`) was
  withdrawn by the G-2 correction and is now being re-implemented by an in-flight bead as a
  *complement* to the nightly unconditional gc. Read it through
  `docs/crash-prevention-requirements.md` §4's audit, not on its own.
- `docs/crash-prevention-requirements.md` §4 G-3 — its "does not exist" premise went stale
  when `e0fab45` landed; a dated closure note is appended by this bead.

---

## 7. Conclusion and alert disposition

- Root cause confirmed and unchanged: infrastructure — repository bloat (Pattern 3);
  immediate cause memcg-OOM at `git push`'s pack-objects, amplifier the no-stop-condition
  re-dispatch loop, underlying cause bead state committed to git.
- Every in-repo prevention layer is **verified in force today**; the one fully-owned cause
  has not recurred since 2026-08-16.
- **Alert disposition for bf-1atrl: no further action.** The subject bead is closed, the
  deliverable is represented on `main` by `46293c5`, and the repository condition is repaired
  and holding. The residual value of this alert is the two P0 NEEDLE-side asks in §3.3,
  which no amount of further investigation of this event will advance.
- The workspace's remaining crash-prevention debt is concentrated in alert quality
  (G-9) and dispatch policy (G-10..G-13) — NEEDLE-side — plus four small repo-side items
  (§3.2).

---

**Report complete:** 2026-09-07 · domchk-a18b2c06
**Sources:** `docs/crash-analysis-bf-1s6c3-2026-09-06.md` · `docs/crash-prevention-requirements.md` · `docs/crashes/bf-1s6c3/` (raw extracts, `MANIFEST.sha256`)
