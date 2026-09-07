# Crash Circumstances Analysis: bf-4yjq

**Analysis bead:** domchk-d5dd1b33
**Date:** 2026-09-02
**Subject bead:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo mirror has diverged/gone stale"
**Method:** Independent re-analysis of `.beads/checkpoint/forensic.jsonl`, alert-bead records, surviving workspace artifacts, and current repository state. Prior bf-4yjq docs were treated as claims and re-verified against raw evidence, not assumed correct.

---

## Executive Summary

bf-4yjq is a **git remote reconciliation task** (repoint `origin` at Forgejo, reconcile
Forgejo/GitHub divergence, configure the server-side push mirror). On **2026-08-12** the agent
dispatched against it was killed **50 times**, every one with exit code -1, between
**17:54:00 and 20:30:43 UTC** — one death every ~3.1 minutes for 2h37m.

The headline correction: prior documentation records **9** crashes. The bead store holds **50
distinct alert beads** for bf-4yjq crashes in that window, and bf-4yjq was only the
second-largest victim of a **workspace-wide crash storm on the same day: 455 exit-code -1
crash reports across 6 beads, running from 05:36 to 23:57 UTC (~18.5 hours)**.

Assessment: **infrastructure failure (OOM from the 18 GB repository bloat), not a code defect** —
consistent with the existing root-cause consensus — but the storm's scale was previously
undercounted by ~5x for this bead and essentially unrecorded for the others.

---

## 1. Task in flight at crash time

| Field | Value |
|-------|-------|
| Bead | bf-4yjq (P2, assignee `claude-code-glm-4.7-lab-domain-check`) |
| Created | 2026-07-20 |
| Crash window | 2026-08-12 17:54:00 → 20:30:43 UTC |
| Closed | 2026-08-17 00:14 UTC — "Git remote configuration successfully fixed and verified" |

The task: fetch both remotes, diff the divergent tips, create a merge commit (no force-push),
repoint `origin` to `git.ardenone.com/jedarden/domain-check`, set up the Forgejo→GitHub
server-side push mirror, verify end-to-end.

Crash-era state files show the reconciliation analysis itself is what the crashed sessions
were feeding: `.beads/divergence-ancestor.json`, `.beads/divergence-point.json`,
`.beads/github_commits_analysis.json`, `.beads/.branch_divergence_state.json` (all dated
2026-08-13, i.e. produced by the sessions that finally ran after the storm).

**Every crash was incidental to the task's content.** The agent was not killed mid-merge or
mid-push by anything about git remotes; it was killed by whatever generic git operation each
short session touched first, on a repository that could not survive git operations.

## 2. The crash storm bf-4yjq sat inside (new finding)

Full-file scan of `.beads/checkpoint/forensic.jsonl` for crash-report descriptions
(`**Bead ID**: ...` / `**Exit code**: ...` template) dated 2026-08-12, deduplicated by alert-bead ID:

| Target bead | Distinct crash events | Window (UTC) | Notes |
|-------------|----------------------|--------------|-------|
| bf-31mno | **350** | 05:36:21 – 16:31:52 | Not previously tallied in any bf-4yjq-era doc |
| **bf-4yjq** | **50** | 17:54:00 – 20:30:43 | Subject of this analysis |
| bf-1s6c3 | 49 | 21:36:51 – 23:57:21 | The bloat crash already documented |
| bf-2xygo | 4 | 21:18:27 – 21:28:29 | |
| bf-23n | 1 | 17:08:40 | |
| bf-5d18 | 1 | 17:23:54 | |
| **Total** | **455** | 05:36 – 23:57 (~18.5 h) | 100% exit code -1 |

Gaps between beads (≈1h22m before bf-4yjq, ≈1h06m after) look like cleanup/cooldown periods
rather than recovery — the storm resumes on the next retried bead. bf-4yjq's 50 events are not
an isolated failure; they are one bead's share of a single day-long OOM regime, with bf-4yjq
absorbing the middle shift after bf-31mno burned out and before bf-1s6c3 took over.

## 3. bf-4yjq crash-sequence detail

- **50 alert beads** (`ALERT: Agent crash on bead bf-4yjq`), all `exit code: -1 (signal -1)`,
  created 17:54:00.249 → 20:30:43.716 UTC — **mean interval 188 s (~3.1 min)**, so each
  re-dispatch died within roughly one to three minutes of starting.
- Labels on the alert beads: `alert`, `crash`, `signal--1`, plus `failure-count:N` escalators
  (`failure-count:1` at 18:38, `failure-count:4` by 20:04) and `umbrella` /
  `verification-failed` on later ones — the alert system was escalating while the retry loop
  kept losing agents.
- Zero variation in exit code across all 50 — a deterministic environmental kill, not a
  flaky code path.
- **No trace of the original crashed sessions survives.** `.beads/traces/` holds entries only
  for later re-runs of the alert beads themselves (captured Aug 26 / Sep 1, exit 0). No core
  dumps, no stack traces, no heartbeats from Aug 12 (`heartbeats.jsonl` retains only recent
  entries). Reconstructing the sessions is only possible from the alert beads and the
  surviving state files.

## 4. Workspace state at crash time

Contemporaneous `git count-objects -vH` recorded in `.beads/crash-bf-4yjq-summary.txt`
(produced during the original investigation, corroborated by the parallel bf-1s6c3 and
bf-1ea4g investigations of the same day):

- Repository total **~18 GB**; loose objects **17.2 GiB across ~4,594 objects** vs **9.6 MiB** packed —
  a severely inverted loose:packed ratio.
- Bloat source: repeated commits of ~237 MB `.beads/` JSONL files during the bf-2ildm era,
  before `.beads/` exclusion was in place.
- System conditions recorded at the time: load average 15–17 on 12 cores, memory effectively
  exhausted during git operations, disk 84% full.

Any substantive git operation on that object store (status/fetch on a cold repo, packing,
fsck) pulled multi-GB working sets; agents died almost immediately on dispatch, which is
exactly what the ~3-minute crash cadence shows.

**Caveat on attribution:** the OOM-killer mechanism rests on the contemporaneous
investigations' system telemetry. Kernel OOM logs were not retained (journalctl access was
limited even then), so the signal-9-from-OOM step cannot be re-verified from raw logs today.
What *is* independently verifiable now: the uniform exit -1 pattern, the scale of the storm,
the bloat metrics recorded at the time, and the fact that the crashes stopped once the
repository was cleaned.

## 5. Inputs and processing context (what survives)

- Divergence-analysis artifacts (listed in §1) — the actual work products of the task, all
  post-storm (Aug 13).
- Git history from the crash window is **gone from the current DAG**: the log jumps from
  2026-08-09 (`00117cb`) to 2026-08-15 (`8373e5d`, bead-forge→bead-rs migration), with the
  crash-era commits removed by the Aug 16 `c27899f` "catch up lab work onto origin (squashed)".
  The 237 MB commits cannot be inspected directly anymore; only their recorded metrics remain.
- Alert-bead corpus: 50 bf-4yjq crash reports + hundreds of sibling storm alerts, still in the
  store (most never individually closed — the known alert-hygiene debt).

## 6. Current state verification (2026-09-02)

The task bf-4yjq set out to do is confirmed done on the live repo:

- `origin` → `https://git.ardenone.com/jedarden/domain-check.git` ✅ (Forgejo-primary)
- local `main` (`994c589`) is identical to `origin/main` ✅
- repo: `.git` 92 MB, 12 loose objects (100 KiB), 1 pack (90.18 MiB) ✅ — all thresholds green
- host: 49 GB memory available, no pressure

Note: a local remote named `github-mirror` (github.com) exists alongside `origin`. The
workspace convention calls for mirroring to be *server-side on Forgejo*, not a client-side
remote. Today's `ab63992` divergence analysis covers Forgejo↔GitHub tip state; the presence
and use of this client-side remote may deserve a separate look, but is outside this crash
analysis's scope.

## 7. Preliminary assessment

**Classification: INFRASTRUCTURE — resource exhaustion (OOM) during git operations on a
bloated repository. Confidence: HIGH on bloat correlation, MEDIUM-HIGH on the OOM mechanism
specifically** (kernel logs not retained; mechanism taken from contemporaneous telemetry).

Chain of events:

1. Pre-Aug-12: bf-2ildm-era workflow commits ~237 MB `.beads/` JSONL files repeatedly →
   repository reaches 18 GB, 17.2 GB loose.
2. 2026-08-12 05:36 UTC onward: git operations on the bloated repo begin OOM-killing agents;
   the storm rolls across six beads for ~18.5 hours (455 kills).
3. 17:54–20:30 UTC: bf-4yjq's retry loop loses 50 consecutive agents, ~1 every 3 minutes.
4. Between 20:30 Aug 12 and the morning of Aug 13 the repository is packed/cleaned by
   bf-173o7e's `git gc` (18 GB → ~91–138 MB, documented elsewhere); divergence-analysis
   artifacts dated Aug 13 show bf-4yjq work then proceeding normally.
5. 2026-08-17: bf-4yjq closed as completed; remotes verified correct (re-verified above).

The bead's own task (git remotes) was neither cause nor casualty of the mechanism — the crash
loop was purely a function of *when* the bead was scheduled relative to the bloat window.

## 8. Corrections to the prior record

Prior docs (`docs/crash-investigation-bf-4yjq-summary-2026-08-26.md`,
`.beads/crash-bf-4yjq-summary.txt`) state **9 crashes at ~17-minute intervals**. Verified from
the bead store: **50 distinct crash events at ~3.1-minute intervals**, 17:54–20:30 UTC. The
nine timestamps listed in the old docs are all present in the verified set of 50 — the earlier
investigation sampled only the alert beads it happened to find. The old docs also do not
record that bf-4yjq's crashes were part of a same-day 455-event storm (bf-31mno's 350 crashes
appear in no bf-4yjq-era document). Downstream counts derived from "9" (e.g. cadence claims,
severity ranking) should be treated as superseded.

## 9. Recommendations carried forward

1. **Storm-level detection, not per-bead.** The existing crash-pattern detection keys on
   repeated crashes of one bead; on Aug 12 no single bead exceeded thresholds until dozens of
   siblings had already died. Aggregate exit-1 rates per workspace/hour is the signal that
   fires earliest here.
2. **Close the alert backlog.** Hundreds of Aug-12 alert beads are still open, which both
   distorts any future counting and created the duplicate-alert noise documented elsewhere.
3. **Retain storm telemetry.** This analysis could not re-verify the OOM mechanism from raw
   logs because none were kept; resource-monitor snapshots (now running via systemd timers)
   should be archived with timestamps so future signal-1 storms are attributable directly.
