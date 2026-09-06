# bf-4yjq Crash Evidence Summary

**Dispatch bead:** domchk-eb60ed60 ("Document crash evidence for bead bf-4yjq")
**Subject bead:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo mirror has diverged/gone stale" (P2, closed 2026-08-17T00:14:14Z)
**Compiled:** 2026-09-06
**This document supersedes** [`docs/crashes/bf-4yjq-crash-evidence-summary.md`](../crashes/bf-4yjq-crash-evidence-summary.md)
(same subject, wrong figures — see §8) as the compiled evidence summary for this crash.

This is the **compiled entry point** for the bf-4yjq crash evidence: every headline figure in
§1–§4 was re-derived live on 2026-09-06 (§7 provenance), and each section points at the chain
document that carries the underlying detail. It exists because the two older bf-4yjq documents
in `docs/crashes/` carry crash counts (4 and 9) that verification has replaced with 50, and the
older evidence summary had no supersession banner.

---

## 1. Headline evidence

| Item | Verified value |
|------|----------------|
| **Crash timestamp (window)** | **50 deaths, 2026-08-12T17:53:53.875Z → 20:30:38.310Z UTC** (13:53:53 → 16:30:38 EDT); mean death→death interval ≈192 s (~3.2 min) — the canonical report's "188 s" divides the same 9,404 s window by 50 instead of by the 49 gaps |
| **Exit code** | **`-1` on all 50** — needle's sentinel `outcome=Crash(-1)` / `signal_code=-1`: a process death whose signal was not recorded, **not** a signal number |
| **Error messages** | No stack traces exist. Complete inventory: the 50 identical ERROR handler lines + the machine-written alert body (§4) |
| **Agent context** | `claude-code-glm-4.7` on worker `claude-code-glm-4.7-lab-domain-check`, workspace `/home/coding/domain-check`, working the **git-remotes task**; each run survived **65–375 s (median 149 s)** of real work before dying |
| **Root cause** | Repository bloat (~18 GB `.git`, 17.2 GiB loose objects) from the bf-2ildm-era `.beads/` commit pattern; git operations on that object store were killed by memory exhaustion |
| **Classification** | **INFRASTRUCTURE — resource exhaustion.** Not a code defect; no investigation of this workspace has ever found one |
| **Resolution** | Repo cleaned Aug 13–14 (18 GB → ~91–92 MB); crashes stopped exactly when the trigger was removed. Subject task completed and the bead closed 2026-08-17 |
| **Current state (2026-09-06)** | `.git` **93 MB** (114 loose / 10,712 in-pack / 1 pack); `origin` → `git.ardenone.com/jedarden/domain-check`; crash class not reproducible |

## 2. Timeline (all UTC, 2026-08-12)

| Event | Count | First | Last |
|-------|-------|-------|------|
| First claim of bf-4yjq | 1 | 17:50:23.048 | — |
| **Crash deaths (exit −1)** | **50** | **17:53:53.875** | **20:30:38.310** |
| Crash-alert beads created | 50 | 17:54:00.249 | 20:30:43.716 |
| Exit-1 failure | 1 | 18:00:17.683 | — |
| Timeouts (exit 124, each 600.1 s) | 4 | 20:40:47.985 | 21:11:27.500 |
| Orphaned exit-0 (bead left open) | 1 | 21:14:56.747 | — |

Two timestamp conventions exist and differ by ~6 s: the **death** timestamps above (worker-log
outcome lines) and the **alert-creation** timestamps baked into the 50 alert-bead bodies
(median 6.0 s later). Alert-bead timestamps must not be quoted as death times. The bead was
completed and closed 2026-08-17T00:14:14Z. Per-run pairing for all 50 deaths (alert bead id +
survival seconds) is tabulated in the
[crash-details extraction](bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md) §5.

## 3. Exit code

`exit_code=-1` is needle's **sentinel for a process death whose signal was not recorded** — not
the numeric value of any POSIX signal, and not evidence of any specific signal name. `was_interrupted=false`
on every record: needle did not interrupt these runs. The fleet-wide mechanism for this death
class is the kernel **memcg OOM killer SIGKILLing the dispatch inside its 12 GiB scope**, but for
the Aug-12 storm that kernel-level step is unverifiable (system journal starts 2026-08-15, earliest
coredump 2026-08-17), so the memcg attribution stays at MEDIUM-HIGH confidence for this event.
What is directly proven: every run died 1–6 minutes into real work, far under the 600 s dispatch
timeout — kills mid-task, not timeouts, not startup failures.

## 4. Error messages

**There are no stack traces and no agent-side error text anywhere in the surviving corpus. Any
report quoting a trace or panic for this event is fabricating it.** The complete inventory:

1. **50 ERROR handler lines** (needle's reaction to the death, no diagnostic content from the
   dying process): `agent crashed — releasing bead and creating alert bead_id=bf-4yjq signal_code=-1`
2. **The machine-written alert body**, identical on all 50 alert beads — `Exit code: -1 (signal -1)`,
   `Workspace: .`, and the dispatch's death-adjacent timestamp
3. **Six WARN lines** for the non-crash outcomes (1 × agent failure, 4 × timeout, 1 × orphaned success)

Absence evidence (verified live 2026-09-06): no core dumps before Aug 17, no journal coverage of
Aug 12, no session transcripts or traces (dispatches predate trace capture), and the three stderr
rotation slots bracket but do not cover the storm.

## 5. What the agent was working on

bf-4yjq was a **git remote reconciliation task, not a crash task**: repoint `origin` at
Forgejo-primary convention, reconcile the Forgejo/GitHub divergence with a merge commit (no
force-push), and configure the Forgejo→GitHub server-side push mirror. The crashes were
**incidental to the task's content** — each short-lived session died on whatever git operation it
touched first, on an object store that could not survive git operations. The task completed only
after the storm: its work products (`.beads/divergence-*.json`, `.beads/github_commits_analysis.json`)
are dated Aug 13, and the bead closed Aug 17.

Storm context: bf-4yjq absorbed the middle shift of a **workspace-wide 455-event exit-code −1
storm across 6 beads, 05:36–23:57 UTC (~18.5 h)** the same day. A spike of sub-3-minute exit −1
re-dispatch deaths across multiple beads is an *environmental regime*, not per-bead failure —
triage repo size / memory / load first.

## 6. Resolution, fixes, and live state

The trigger was removed on Aug 13–14 (gc; 18 GB → ~91–92 MB, 97.5 % reduction) and the crashes
stopped exactly then. Deployed prevention: `.beads/` gitignored, >10 MB pre-commit block,
repo-health monitoring timers, `safe-git-gc.sh` with checkpoint/resume, `pack.windowMemory` /
`deltaCacheSize` / `threads=1` guarding the bare-gc path, and the 2026-09-02 crash-alert system.

**Live re-verification for this summary (2026-09-06, this dispatch):**

```
grep -c 'handling agent outcome bead_id=bf-4yjq exit_code=-1' <worker log.2>  → 50
distinct alert beads titled 'ALERT: Agent crash on bead bf-4yjq' (forensic.jsonl) → 50
ls -l ~/.needle/logs/…log.2 → 134,207,883 B (byte-identical to the extraction's count — still present)
git count-objects -vH → 114 loose (920 KiB), 10,712 in-pack, 1 pack
du -sh .git → 93 MB        (healthy limit 500 MB; critical 1 GB)
git remote -v → origin = git.ardenone.com/jedarden/domain-check  ✓ (github-mirror client remote also present, flagged off-convention in canonical §8)
bead show bf-4yjq → Closed 2026-08-17T00:14:14Z
```

A healthy object store plus a healthy host leaves the mechanism nothing to act on — the class is
not reproducible today, and recurrence is guarded by the prevention stack, which should not be relaxed.

## 7. Evidence map

| Document | What it carries |
|----------|-----------------|
| [`bf-4yjq-crash-investigation.md`](bf-4yjq-crash-investigation.md) | **Canonical report** — analysis, root cause, storm, reproducibility (domchk-4eab7c59) |
| [`bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md`](bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md) | Record-level extraction — verbatim log lines, all 50 per-run rows, survival stats |
| [`bf-4yjq-artifact-catalog-2026-09-06.md`](bf-4yjq-artifact-catalog-2026-09-06.md) | Live-verified inventory of every artifact location; retention warning |
| [`bf-4yjq-consolidated-summary-domchk-ea5c6a63-2026-09-06.md`](bf-4yjq-consolidated-summary-domchk-ea5c6a63-2026-09-06.md) | Reconciliation of the pre-verification reports to the canonical record |
| `docs/crashes/bf-4yjq-crash-evidence-summary.md` | **Superseded (banner added)** — recorded 4 crashes / 445 MB |
| `docs/crashes/bf-4yjq-crash-report.md` | **Partially superseded (banner present)** — recorded 9 crashes / ~17-min cadence |

Retention: the storm's only primary source is needle log slot `.log.2` (134,207,883 B, confirmed
still present today). One more ~128 MiB rotation erases it; §1–§4 above and the extraction are
the durable record.

## 8. What the older documents got wrong

The superseded `docs/crashes/bf-4yjq-crash-evidence-summary.md` (2026-09-01) states **4 crash
occurrences**, asserts **`signal -1 (SIGKILL)`** as a literal signal identification, reports the
post-cleanup repo as **445 MB / 1.7 GB**, and implies the crashes were post-task. Verification
replaced all four: **50 crashes** at ~3.1-min cadence, **signal identity not recoverable** (−1 is
a sentinel), repo at **91–93 MB** today, and every death **mid-task** (median 149 s survival,
task completed only after the storm). Those wrong figures are why the umbrella alert
(domchk-e8cc9d7c) failed verification three times and was auto-split into the chain that produced
the catalog, extraction, and this summary. A supersession banner now points readers here.

## 9. Acceptance criteria

- [x] **Comprehensive crash summary document created** — this file; compiled entry point over the verified corpus (§1–§7)
- [x] **Timestamp included** — 50 deaths, first/last and window, UTC + EDT, death-vs-alert distinction (§1, §2)
- [x] **Exit code included** — `exit_code=-1` / `outcome=Crash(-1)` / `signal_code=-1`, sentinel interpretation, 50/50 consistency (§1, §3)
- [x] **Error messages included** — complete honest inventory (no stack traces exist; two line shapes + alert body, absence evidenced) (§4)
- [x] **Context of what the agent was working on** — git-remotes task, worker/model/workspace, per-run survival 65–375 s mid-task (§1, §5)
- [x] **Saved to an appropriate location** — `docs/crash-investigations/`, alongside the rest of the bf-4yjq chain deliverables

## 10. Provenance

Compiled 2026-09-06 by dispatch domchk-eb60ed60, the "document crash evidence" step of the
bf-4yjq auto-split chain (catalog → extract details → **document evidence** → umbrella). An
earlier attempt of this dispatch wrote the file but died before committing; the retry re-ran
every live check rather than trusting it. Independently re-derived live: the 50 `exit_code=-1`
outcome lines (grep count on `.log.2`), the 50 distinct alert-bead IDs and their
17:54:00.249Z → 20:30:43.715Z creation window (parsed from `.beads/checkpoint/forensic.jsonl`
over the `issue` snapshots — the same title string also appears on 3 later duplicate-investigation
beads, which are not crash alerts), the `.log.2` byte size (134,207,883 B, byte-identical),
`git count-objects` / `du`, `git remote -v`, the subject bead's closed status, and every §2
non-crash row (1 × exit 1 at 18:00:17.683Z, 4 × exit 124, 1 × exit 0). Inherited from the
verified corpus without re-derivation: per-run survival statistics and pairing (extraction §4–§5,
twice re-verified there), absence evidence (catalog §6, spot-checked), storm and root-cause
analysis (canonical §4–§7). Two deviations were introduced by this pass and are deliberate:
the supersession banner claimed in §7–§8 was found missing from
`docs/crashes/bf-4yjq-crash-evidence-summary.md` and has been added, and §1 records the
death→death interval (≈192 s) alongside the canonical report's "188 s" — the two numbers are the
same window over different divisors (49 gaps vs 50 deaths), not a factual conflict. Where the
two older `docs/crashes/` reports disagree with the verified record, §8 records the correction.
