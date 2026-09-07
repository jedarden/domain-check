# bf-4yjq Crash Investigation — Consolidated Summary

**Dispatch bead:** domchk-ea5c6a63
**Date:** 2026-09-06
**Source artifacts reviewed:**
[`docs/crash-artifacts-bf-4yjq.md`](../crash-artifacts-bf-4yjq.md) (artifact catalog, 2026-08-16)
and [`docs/reports/bf-4yjq-comprehensive-crash-report.md`](../reports/bf-4yjq-comprehensive-crash-report.md) (2026-08-14)

**Canonical report this summary defers to:**
[`docs/crash-investigations/bf-4yjq-crash-investigation.md`](bf-4yjq-crash-investigation.md) (domchk-4eab7c59, 2026-09-02)

Both source artifacts predate the canonical verification and carry supersession banners; this
summary consolidates what they and the canonical report together establish, corrected to the
verified figures.

---

## What the incident was

Bead **bf-4yjq** ("Git origin remote points to GitHub directly; Forgejo mirror has
diverged/gone stale", P2) dispatched agents that were killed **50 times, all exit code -1**,
between **17:54:00 and 20:30:43 UTC on 2026-08-12** — one death every ~3.1 minutes for 2h37m.
The bead sat in the middle shift of a **workspace-wide storm: 455 exit-code -1 events across
6 beads over ~18.5 hours** the same day. The bead's own work completed after the storm
(closed 2026-08-17) and is verified correct on the live repo.

## Key findings

1. **Root cause: repository bloat, not the task and not a code defect.** The bf-2ildm-era
   workflow had repeatedly committed ~237 MB `.beads/` JSONL files, inflating the repo to
   ~18 GB with **17.2 GiB of loose objects vs 9.6 MiB packed** (inverted ratio ≈1,800:1).
   Substantive git operations on that object store pulled multi-GB working sets and were
   OOM-killed. Classification: **INFRASTRUCTURE — resource exhaustion**.

2. **The historical record undercounted the event.** Both reviewed artifacts record
   **9 crashes at ~17-minute intervals**; verification against
   `.beads/checkpoint/forensic.jsonl` established **50 at ~3.1-minute intervals**, and the
   350-event bf-31mno storm earlier that day was recorded nowhere. Per-bead histories built
   from alert sampling cannot be trusted for scale — the forensic checkpoint is the source
   of truth.

3. **"BLOCKED, not actively executing" is unverifiable.** The old reports' claim that the
   bead was blocked at crash time does not survive against the ~3-minute re-dispatch kill
   cadence. The defensible statement — which all waves agree on — is that the crashes were
   **incidental to the task's content**: any agent touching the bloated repo died on
   whichever git operation it hit first.

4. **The evidence base has hard limits.** No core dumps, no kernel OOM logs, and no Aug-12
   heartbeats survive. Bloat correlation is HIGH-confidence; the OOM mechanism specifically
   is MEDIUM-HIGH (inferred from contemporaneous telemetry, not re-verifiable raw logs).

5. **Triage signature worth keeping:** a spike of sub-3-minute exit-code -1 re-dispatch
   deaths across multiple beads is an *environmental regime*, not per-bead failures —
   triage repo size / memory / load first. This crash is a condition you can re-create, not
   an event you can re-run.

## What was fixed

| Fix | Detail |
|-----|--------|
| Repository cleanup (Aug 13–14) | 18 GB → ~91–92 MB (97.5% reduction); crashes stopped exactly when the trigger was removed |
| The task itself | `origin` → Forgejo, server-side push mirror to GitHub; closed 2026-08-17, re-verified 2026-09-02 |
| `.gitignore` exclusion of `.beads/` | Stops the commit pattern that created the bloat |
| Pre-commit hooks | Block files >10 MB |
| Repo-health monitoring | `scripts/check-repo-health.sh` + systemd timers; alert >1 GB total / >500 MB loose |
| Bounded gc | `scripts/safe-git-gc.sh` (checkpoint/resume, memory limits), daily incremental + weekly full timers, `pack.windowMemory`/`deltaCacheSize`/`threads=1` config guarding the bare-gc path |
| Crash-alert system | Closed-bead filtering, dedup, cooldown, classification (2026-09-02) |

## Live re-verification (this dispatch, 2026-09-06)

Commands run today against the working repo — all green:

```
git count-objects -vH → 98 loose objects (788 KiB), 10,712 in-pack, 1 pack (90.43 MiB)
du -sh .git           → 93 MB   (healthy limit: 500 MB)
git remote -v         → origin → git.ardenone.com/jedarden/domain-check (Forgejo-primary);
                        github-mirror remote also present (flagged in canonical §8 as
                        off-convention; mirroring should be server-side on Forgejo)
host                  → 79 GB disk free, 44 GB memory available
```

The failure class remains non-reproducible: a healthy object store plus a healthy host leaves
the mechanism nothing to act on. Recurrence is guarded by the prevention stack above, which
should not be relaxed.

## Open items (not this dispatch's scope)

- `docs/crash-investigation-bf-4yjq-summary-2026-08-26.md` still repeats the superseded
  **9-crash** figure with no banner — a candidate for the documentation-refresh bead
  (domchk-7625a5cc) that this summary blocks.
- Hundreds of Aug-12 alert beads, including most of bf-4yjq's 50, remain open — the known
  alert-hygiene debt (canonical report, §9 recommendation 2).

---

**Status:** ✅ CONSOLIDATED — investigation closed; both source artifacts reconciled to the
canonical record; current state independently verified live on 2026-09-06.
