# Crash Diagnostics Package — bf-2vtzg

**Collected:** 2026-09-07 (data-collection bead `domchk-0befc321`)
**Target bead:** bf-2vtzg — "Document remote Forgejo origin state" (created 2026-08-13T07:14:57Z, closed 2026-08-13T09:42:58Z, revision 1, never reopened)
**Chain:** `domchk-c45df846` (classify — CLOSED: INFRASTRUCTURE, confidence HIGH, depth "NONE beyond this classification") → **this package** → `domchk-80860fb2` (analyze root cause, open, blocked by the collector)
**Raw artifacts:** `.beads/state/domchk-0befc321/` (gitignored, per chain convention) — `bf-2vtzg-events-domain-check-2026-08-13.jsonl` (the complete per-event extraction, 54.5 KB / 178 events)

This package renders the five data-collection acceptance criteria against the surviving evidence. Every figure below was re-extracted first-hand on 2026-09-07 from the primary sources named, not copied from prior corpus docs.

## 1. Evidence availability at crash time (what can and cannot exist)

The crash window is 2026-08-13 09:10–09:43 UTC. Evidence availability for that window, verified live:

| Source | Status for 2026-08-13 |
|---|---|
| Needle worker JSONL log (`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl`, 12,131 events) | ✅ Complete — all 10 attempts present |
| Kernel / journald memcg records | ❌ Cannot exist — single journald boot begins **2026-08-15 19:56:33 EDT** (`journalctl --list-boots` = boot 0 only); independently stated by today's bf-4k2ws determination |
| Memory/disk telemetry | ❌ Not collected pre-journal (lab-health-collector starts 2026-08-15); no memory/disk event type exists anywhere in the day's logs |
| Coredumps | ❌ None before 2026-08-25 |
| Monitoring logs (`.beads/logs/`) | ❌ Begin 2026-09-01 |
| CPU/load | ✅ Reconstructable from `fleet.cpu_saturated` samples in the needle logs |

Consequence: crash-time **memory and disk are unrecoverable**, and the memcg-OOM mechanism is **chain-inferred from kernel-proven siblings** (bf-4x12ec gc-side, bf-198ne push-side), not kernel-proven for this bead — the same evidentiary posture the bf-4k2ws determination took for the same day.

## 2. Crash-time system state (what survives)

**CPU/load — saturated for the entire window.** 51 `fleet.cpu_saturated` events in 09:00–10:00Z across the fleet's logs, `load_average` ranging **7.23–16.52 on 9 cores** (saturation threshold 0.8 → ~7.2). Peaks coincide with the loop's dispatch moments (16.52 at 09:17:57Z, right at attempt 3's dispatch). Day-wide for this worker, the committed bf-4k2ws determination counts 59 samples at 7.63–18.51 — this window sits inside that envelope, at nothing like its extreme.

**Memory/disk at crash time:** not recoverable (§1). The only memory statement that can be made is mechanistic: each dispatch ran inside the 12 GiB `MemoryMax` dispatch scope, and the bloat-era object store (§5) was the load that consumed it.

**Current system snapshot (2026-09-07 10:44 EDT), for contrast:** 46 Gi available of 62 Gi, load 5.98, mem Pressure PSIs ~0. One current observation worth flagging: disk free is **22 GB (95% used)** — below the repo's own 30 GB warning threshold, unrelated to this crash but noted since the package captures disk state.

## 3. Crash artifacts — the 10-attempt kill loop

All 10 attempts came from one worker (`claude-code-glm-4.7-lab-domain-check`, session `8446529e`), 09:10:45Z → 09:43:26Z:

| # | Dispatched (Z) | Died / exited (Z) | exit_code | Lived |
|---|---|---|---|---|
| 1 | 09:10:45.25 | 09:13:17.64 | −1 | 152.1 s |
| 2 | 09:13:30.35 | 09:17:40.93 | −1 | 250.2 s |
| 3 | 09:17:57.09 | 09:22:11.66 | −1 | 254.3 s |
| 4 | 09:22:21.22 | 09:25:05.67 | −1 | 164.3 s |
| 5 | 09:25:15.38 | 09:29:19.43 | −1 | 243.8 s |
| 6 | 09:29:28.79 | 09:32:42.02 | −1 | 193.0 s |
| 7 | 09:32:51.39 | 09:35:13.50 | −1 | 141.9 s |
| 8 | 09:35:23.94 | 09:36:51.93 | −1 | 87.8 s |
| 9 | 09:37:01.04 | 09:40:22.18 | −1 | 200.9 s |
| 10 | 09:40:31.53 | 09:43:23.56 | **0** | 171.9 s |

Structure of every one of the nine kills, identical each time: `agent.completed exit_code=-1` → `outcome.classified {exit_code: -1, outcome: "crash"}` → `bead.released release_success` → `outcome.handled {action: "alerted"}` — the pre-0.4.2 **one-alert-per-kill** behavior that seeded this bead family. Attempt lifetimes 88–254 s with no cap-adjacent clustering (dispatch cap was 600 s) — deaths mid-run, not timeouts.

Corroborations and extensions of the classification sibling's record:

- Kill timestamps match `domchk-c45df846`'s nine exactly, first-hand.
- The archived FP report's "crash timestamp" **09:35:19.810714905Z is a `heartbeat.emitted` 6.31 s after attempt 7's 09:35:13.495Z kill** — re-measured here (6.31 s), and it is followed by the release at 09:35:21.896Z. The classification's heartbeat reading is confirmed.
- **Attempt 3 committed the deliverable (`ad88d53`, author+commit 09:21:44Z) 27 s before its own kill** at 09:22:11Z. The loop was dying *after* committing — the deliverable survived the loop even though five more attempts died after it.
- Attempt 10's exit 0 + `verification.passed {gates_run: 1}` + `bead.completed` (09:43:26.714Z) therefore validated work **already committed 22 minutes earlier by a dead attempt** — the same verify-then-close amplifier shape the bf-4k2ws determination documents. Note the bead's own `Updated` stamp (09:42:58.663Z, the `bead close`) precedes the agent's exit (09:43:23.561Z): the agent closed the bead in-task, then exited, then needle recorded completion.
- The dispatch-level retry loop terminated by behavior change (attempt 10 succeeding), not by remediation — nothing about the repository changed between attempts 9 and 10.

## 4. Fleet context — this loop sat inside a same-day kill regime

Exit-−1 kills on the `domain-check` worker, 09:00–10:00Z, chronological:

- 09:00:12–09:08:30Z — **bf-1ea4g ×6** (bf-2vtzg's dependency bead, which closed 09:10:16.731Z; bf-2vtzg's first claim came 28.5 s later)
- 09:13:17–09:40:22Z — **bf-2vtzg ×9** (this event)
- 09:46:15–09:58:20Z — **bf-ncxbt ×4** (bf-ncxbt is *itself* one of the 11 prior sibling alert beads for bf-2vtzg — the alert-investigation dispatches were dying to the same mechanism that generated them; 11 bf-ncxbt kills on the day)

Day-wide, this worker recorded **344 exit-−1 kills on 2026-08-13**: bf-65lsdu ×127, bf-1ea4g ×56, bf-4k2ws ×55 (independently reproducing the canon's "55" for bf-4k2ws — a check that this extraction method matches the committed record), bf-2ildm ×38, bf-1s6c3 ×22, bf-ncxbt ×11, bf-2vtzg ×9, bf-mje3pd ×7, bf-6d3d6 ×6, bf-574w1 ×5, bf-3hivb ×5, bf-29h1yy ×2, bf-2o7nlw ×1. The bf-2vtzg loop is a 9-kill instance of a 344-kill day, not an isolated failure.

## 5. Repository state

**At crash time:** ≈18 GB `.git`, ≈17 GB loose objects — canon-sourced from the cleanup record per the committed bf-4k2ws determination (§2/§3.2 there), not re-measurable now. bf-2vtzg sat squarely in the bloat era (2026-08-12 → 09-01), and its task class — git-remote-heavy reads (ls-remote / fetch / rev-list against Forgejo) — is exactly the operation class the bloat era turned into kills.

**Collected 2026-09-07 (live):**

| Metric | Value |
|---|---|
| `.git` size | 106 MB |
| Loose objects | 319 / 3.10 MiB |
| Packs | 1, 99.11 MiB |
| Garbage | 0 |
| `git fsck --full` | exit 0 (3 dangling objects — normal churn) |
| `check-repo-health.sh` | exit 0 |
| Effective pack-memory bound | verified, worst case ≈3072 MiB (windowMemory=2g, threads=1, deltaCache=1g) |
| Divergence origin/main…HEAD | 0 / 0 |

The bloat precondition cannot recur through bead state (`.gitignore:66 .beads/`, 0 tracked `.beads` files).

## 6. Service availability (inference gateway)

- **At crash time:** unrecoverable — no monitoring logs predate 2026-09-01 and journald begins 2026-08-15. The classification already excludes SERVICE_FAILURE on exit-code classes alone: all nine kills are exit −1, and no 1/503-shaped completion appears anywhere in the loop.
- **Collected 2026-09-07:** `curl -skf https://traefik-apexalgo-iad...:8444/health` → `ok`, rc 0.

## 7. Git operations history

- **Deliverable produced and verified on origin/main:** `forgejo_remote_state_bf-2vtzg.json` (blob `9cf6ee23`) and the analysis doc (now `docs/archive/crash-investigations/forgejo-origin-state-bf-2vtzg.md`, blob `01bac556`, after the a883044 archive move). Both present on origin/main today.
- **But the original commit never reached origin by its own attempt's push.** `ad88d53` ("docs: document Forgejo remote origin state for bead bf-2vtzg", 09:21:44Z, touching the JSON + `docs/branch-divergence-analysis-bf-2vtzg.md` + `.needle-predispatch-sha`) is reachable **only** from the local-only branch `pre-squash-history-20260816` — not from origin/main. The content reached origin/main via the later squash `c27899f` ("catch up lab work onto origin (squashed)"). This is direct evidence for how the loop's work actually landed: commits accumulated locally through the kills, and a later fleet-level reconciliation — not any bf-2vtzg attempt — carried them to the remote.
- **Crash-era divergence snapshots (the phantom-N family):** bf-4k2ws's doc records "438 local commits ahead" (tip `b2a71f7`, 07:03:23Z); bf-1ea4g's records 724 total commits (05:26:41Z); bf-2vtzg's deliverable records **"Local branch ahead by 503 commits"** (local tip `6f0c76fc` @ 08:12:36Z vs remote `63ba0247` @ 2026-08-09 13:00:56 −0400). The local tip advanced 07:03 → 07:32 → 08:12 while its attempts kept dying and committing — the loop grew its own ahead-count. All of these figures describe the pre-squash state, since resolved: 0/0 divergence everywhere (2026-09-06), never actual divergence. No weight should be given to "503 commits ahead" as a standing condition.

## 8. Timeline (UTC, 2026-08-13)

| Time | Event |
|---|---|
| 07:14:57 | bf-2vtzg created — step 2 of the branch-divergence chain, "depends on bf-1ea4g (local state documentation)" |
| 05:26–09:10 | bf-1ea4g's own 57-attempt loop runs against the same repo (56 exit-−1 kills this day on this worker) |
| 09:10:16.731 | bf-1ea4g closed (dependency satisfied) |
| 09:10:45.24 | bf-2vtzg attempt 1 claimed, 28.5 s after the dependency closed |
| 09:13:17 → 09:40:22 | attempts 1–9 killed, exit −1, each released + alerted |
| 09:21:44 | attempt 3 commits the deliverable (`ad88d53`), 27 s before its own kill |
| 09:40:31.53 | attempt 10 dispatched |
| 09:42:58.663 | attempt 10 closes bf-2vtzg in-task (`bead close`) |
| 09:43:23.561 | attempt 10 exits 0; `verification.passed` (1 gate) |
| 09:43:26.714 | needle records `bead.completed`; outcome handled, action none |
| 09:46–09:58 | bf-ncxbt (an alert bead for this very crash) killed ×4 by the same mechanism |

## 9. Acceptance-criteria mapping

| Criterion | Where |
|---|---|
| System state at crash time (memory, load, disk) | §1–§2 — load reconstructed; memory/disk proven unrecoverable, with the availability argument |
| Crash logs and artifacts collected | §3 + raw extract in `.beads/state/domchk-0befc321/` |
| Repository health assessed (size, loose objects) | §5 |
| Service availability checked (inference gateway) | §6 |
| Git operations history reviewed | §7 |
| Timeline of events | §8 |

## 10. Notes for the analyzer (`domchk-80860fb2`)

- Nothing collected contradicts the classification: INFRASTRUCTURE, HIGH confidence. Mechanism is chain-inferred (12 GiB dispatch-scope memcg OOM over the bloat-era object store) via kernel-proven siblings, since no Aug-13 kernel record can exist.
- Keep the two layers separate, as the classification does: the **kills** are INFRASTRUCTURE; the **alert layer** is the false-positive surface (the target was closed by its own attempt 10, and the alert family — 11 prior siblings plus this chain — investigated finished work; bf-ncxbt itself died to the same mechanism).
- No code-defect evidence anywhere in the artifacts; the deliverable itself is intact and correct on origin/main.
- The "503 commits ahead" figure in bf-2vtzg's deliverable is a resolved pre-squash snapshot, not an open condition (§7).
