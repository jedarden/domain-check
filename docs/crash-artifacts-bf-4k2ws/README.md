# Crash artifact bundle — bf-4k2ws

Collected 2026-09-07 by bead `domchk-8e0fc94d` (collection split-child of alert
`bf-15k67`). All live captures are timestamped in their filenames; everything
else is quoted verbatim from surviving sources with the source path named.

## Files

| File | What it is |
|---|---|
| `2026-09-07T0823Z-needle-log-extract-bf-4k2ws-aug13.txt` | **Primary evidence.** Full attempt loop (436 lifecycle events), the kill bracket matching the alert instant, and `fleet.cpu_saturated` samples — extracted from the surviving needle worker log for 2026-08-13 |
| `2026-09-07T0825Z-crash-record-bf-15k67.md` | The alert bead's verbatim crash report, the alert-vs-death timestamp mapping, and the target bead's state |
| `2026-09-07T0819Z-system-state-current.txt` | Live memory / disk / load capture (2026-09-07, **not** at crash time — see Unrecoverable below) |
| `2026-09-07T0820Z-git-repo-state-current.txt` | Live git repo size/health metrics (repaired state) |
| `2026-09-07T0820Z-crash-classifier-run.txt` | `scripts/crash-classifier.sh bf-4k2ws` run → exit 2, no trace artifact to classify from |
| `2026-09-07T0824Z-traces-check.txt` | Why nothing can be copied from `.beads/traces/`, and which era artifacts are unrecoverable |

## Findings

1. **The dispatch's "crash timestamp" is the alert time, not the death time.**
   `2026-08-13T02:33:47.409682217Z` is the `HANDLING_RELEASE_DONE` heartbeat at
   `02:33:47.409670765Z` (equal to 11.5 µs), **6.025 s after** the real kill:
   `agent.completed exit_code=-1` at `2026-08-13T02:33:41.384776124Z`, an
   attempt that ran 173.9 s (dispatched 02:30:47.256Z). Session `8446529e`,
   worker `claude-code-glm-4.7-lab-domain-check`, agent `claude-code-glm-4.7`.

2. **That kill was one of a 62-attempt crash loop on bf-4k2ws** (2026-08-13
   02:01:29Z → 07:17:41Z, single worker session): **55 crash (exit −1) + 5
   timeout (exit 124) + 2 success (exit 0)**. The successes at 04:48:09.546Z
   (378.9 s) and 07:17:41.039Z (193.4 s) each ran 1 verification gate and were
   followed ~6 s later by `bead.orphaned`. So the task **did complete during
   the loop**, twice — the exit −1 alert is the post-completion /
   mid-storm pattern, matching the committed FALSE_POSITIVE classification.

3. **Classification: FALSE_POSITIVE / infrastructure-era kill, not a code
   defect.** Target bead bf-4k2ws is Closed with its deliverables committed
   (`c27899f`, 2026-08-16 squash). The night's kill mechanism is the
   already-documented repo-bloat memcg-OOM storm (bf-1s6c3 / bf-4yjq era) —
   see `docs/crash-analysis-bf-1s6c3-2026-09-06.md`. This bundle adds the
   alert↔death mapping and the attempt census; it does not change that
   classification and does not supersede the existing bf-4k2ws corpus
   (`docs/crashes/bf-4k2ws-crash-report.md`, `docs/crash-investigations/bf-4k2ws/`).

4. **CPU state at the kill is recoverable; memory/disk is not.** The loop
   window holds 59 `fleet.cpu_saturated` samples, load 7.63–18.51 on 9 cores
   (saturation threshold 0.8 ⇒ load > 7.2): the box was saturated the entire
   time, including across the mapped kill. No memory/disk samples exist for
   2026-08-13 (health collection starts 2026-08-15; kernel kill records for
   Aug-12/13 were lost to the Aug-14 reboot).

5. **The classifier run is an honest empty result.** `crash-classifier.sh`
   needs `.beads/traces/<id>/trace.jsonl`; no bf-4k2ws trace survives (trace
   slots are single-run and the oldest surviving trace dir is 2026-08-16), so
   it exits 2. The classification above rests on the log + bead-state evidence
   instead.

## Current-state caveat

The system-state and git-state files capture 2026-09-07 health (repo 102 MB,
one 99.11 MiB pack, 31 loose objects, fsck clean; 46 Gi RAM available) — they
document that the repo-bloat preconditions of 2026-08-13 are gone, **not** the
crash-time state, which is unrecoverable and described in the traces-check
file instead.
