# bf-2xygo Crash Investigation — Consolidated Findings Report

**Investigation bead:** domchk-49962f7e ("Document investigation findings for bf-2xygo crash")
**Date:** 2026-09-06
**Subject bead:** bf-2xygo — "Fetch and analyze divergence between Forgejo and GitHub remotes" (P2, task)
**Crash date:** 2026-08-12, 21:18–21:31 UTC
**Method:** `docs/crash-response-guide.md` (exit-code table, False-Positive Rules 1–3, Crash
Patterns 1 & 3), applied to the committed raw-event extract
`docs/crash/bf-2xygo/raw-logs/` (91 events, commit fb7bedd, manifest-verified 2026-09-06)
**Role of this document:** the narrative write-up deferred to this bead by
`docs/crashes/bf-2xygo-crash-classification-2026-09-06.md` (domchk-fe1e4a60). The extraction
(domchk-cd364e0a) and classification (domchk-fe1e4a60) layers are cited, not re-derived.
Everything marked **[LIVE]** below was re-executed by this bead on 2026-09-06 rather than
copied from a prior report.

---

## 1. Quick reference

| Field | Value |
|---|---|
| Bead | bf-2xygo — Forgejo/GitHub remote divergence analysis |
| Deaths | 4 × `exit_code=-1` (21:18:21, 21:21:25, 21:24:44, 21:28:24 UTC) |
| Resolution | Attempt 5 exited **0** at 21:31:21 UTC; `verification.passed`; bead closed same minute-window |
| Crash type | **Infrastructure event — repository-bloat sub-type** (Pattern 3) |
| Alert disposition | **False-positive / no action** — self-healed transient; subject completed and closed 2026-08-12 |
| Root cause | `git fetch` of both remotes against the ~18 GB bloated repo, inside the bf-1s6c3/bf-4yjq bloat storm evening |
| Domain-check code involved | None — the task never executed application code |
| Kernel proof | None possible for Aug-12 (journald begins 2026-08-15); mechanism proven for the later same-mechanism bf-198ne |

## 2. Crash summary and timeline

bf-2xygo was dispatched five times with a **byte-identical prompt** (`prompt_len=70650`,
`prompt_hash=sha256:41e63f99…5b07`, template `pluck-default`, agent `claude-code-glm-4.7`).
Four consecutive attempts died by signal; the fifth succeeded. All timestamps UTC.

| Attempt | Dispatched | agent.completed | Exit | Duration (ms) | Events written |
|---|---|---|---|---|---|
| 1 | 21:15:05.484 | 21:18:21.891 | −1 | 196,226 | 29 |
| 2 | 21:18:31.034 | 21:21:25.960 | −1 | 174,755 | 29 |
| 3 | 21:21:35.874 | 21:24:44.890 | −1 | 188,795 | 28 |
| 4 | 21:24:54.121 | 21:28:24.006 | −1 | 209,674 | 35 |
| 5 | 21:28:33.650 | 21:31:21.362 | **0** | 167,541 | 25 |

Each failed attempt produced the identical event chain — **[LIVE]** re-read from the
committed extract, all four instances confirmed:

```
agent.completed (exit_code=-1) → outcome.classified (outcome=crash)
  → bead.released (reason=release_success) → outcome.handled (action=alerted)
```

Attempt 5 produced `outcome=success` → `verification.passed` → `bead.completed`
(needle event 21:31:25.337). The bead store itself records the close at
**21:30:57.461** — 24 s *before* the worker process exited: the worker finished, closed its
bead, and shut down cleanly. This is the opposite of the release-conflict death pattern
(bead closed then killed by signal); here the kill sequence had already stopped.

Total loss to the crash: ~13 minutes of wall clock and four alert records. Zero data loss —
attempt 5 completed the analysis, and the deliverable exists on main as
`docs/notes/forgejo-github-sync-analysis-2026-08-26.md` (commit 3838a30;
**[LIVE]** `git ls-tree origin/main` confirms the blob on the pushed tip). Attempt 5 itself
committed nothing — **[LIVE]** `git log --until` shows zero main-branch commits from
2026-08-09T13:00Z to 2026-08-12T22:00Z — so the analysis reached git only later, from a
2026-08-26 session.

`exit_code=-1` is needle's `wait()` sentinel for **died by signal** — not a signal number
(repo canon: `docs/crashes/signal-minus1-root-cause-analysis-verified-2026-09-02.md`).

## 3. Evidence gathered — and what did not survive

**Preserved and used:**

- `docs/crash/bf-2xygo/raw-logs/` — all 91 needle events from the dated dispatch log
  `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl` (3,589,712 bytes,
  mtime Aug 12 19:57 EDT — **[LIVE]** unchanged since the crash). Manifest sha256
  re-verified **[LIVE]** 2026-09-06: OK.
- Live recounts across all six Aug-12 agent logs (`domain-check`, `drawrace`, `roam-1`,
  `roam-2`, `s1`, `test-fix`), by the classification bead.
- Contemporaneous repo-state records: bf-2igib's 2026-08-16 close reason (≈18 GB `.git`,
  ≈17 GB loose objects) and `docs/crashes/repository-bloat-crash-bf-1s6c3-2026-08-12.md`.

**Known unrecoverable (checked 2026-09-06, restated from the extract README):**

- **Kernel/OOM records for Aug-12.** journald on this box begins 2026-08-15 19:46 EDT.
  The signal-level mechanism (memcg `CONSTRAINT_MEMCG` SIGKILL at the 12 GiB dispatch-scope
  bound) is *proven* for the later push-side variant bf-198ne — same operation class, same
  storm lineage — but is **not provable for Aug-12 specifically**. No finding in this report
  depends on it.
- **Session transcripts** for the five attempts (single-slot trace long rotated; the
  bf-4yjq extraction did not include any bf-2xygo transcript).
- **Load samples inside the crash window** as a *discriminating* signal — see §7: samples
  exist but only at dispatch-evaluation moments.

## 4. Root cause analysis

**Primary attribution: repository bloat.** At crash time the repo carried ≈18 GB of `.git`
with ≈17 GB loose objects (17+ identical 237 MB `.beads/*.jsonl` snapshots committed), and
the evening was the bf-1s6c3/bf-4yjq bloat storm: bf-4yjq's 50 kills ended 20:30, bf-1s6c3's
49 began 21:36 — bf-2xygo's four kills at 21:18–21:28 fall exactly between them. The task's
own operation class was `git fetch` of both remotes on that repo — the same
git-pack-operations class that memcg-OOM'd its neighbours, and later proven lethal for
bf-198ne's `git push` on 2026-08-16.

**Why the kills were bead-local, not a fleet wave.** The classification bead's live recount
found exactly **4** exit-−1 deaths in 21:15–21:32 across all six Aug-12 agent logs — all
bf-2xygo's own. The day's 455 kills were concentrated, not synchronized (bf-31mno 350,
bf-4yjq 50, bf-1s6c3 49, bf-2xygo 4, two singles). The fixed ~3.2-minute re-dispatch cadence
(needle re-claimed within ~10 s of each release) against a repo that could not complete a
pack operation is Pattern 3's signature.

**Excluded alternates** (per `docs/crash-response-guide.md`):

- **workflow-failure** — requires exit 1 + `error_max_turns`. All four deaths were signal
  deaths; `transform.completed` succeeded on every attempt.
- **service-failure** — requires HTTP 503/502 to the inference gateway. No 5xx in any
  attempt; no gateway-failure signature in the day log.
- **code-defect** — the task never executed domain-check code, and no application error
  appears in any attempt. Consistent with 157+ investigations of this workspace finding
  zero domain-check defects.

**Epistemic status.** The 18 GB figure is contemporaneous but not re-measurable (the repo
was repaired to ~94 MB, re-verified 2026-09-06). The Aug-12 kill mechanism is unverifiable
(§3). The classification therefore rests on the Pattern-3 signature — which is exactly the
detection heuristic the crash-response guide provides for the pre-journald era. Confidence
in the attribution is high; confidence in any *specific* syscall-level narrative for these
four kills should stay modest.

## 5. Classification and alert disposition

Per `docs/crashes/bf-2xygo-crash-classification-2026-09-06.md`, which this report adopts:

- **Crash type:** `infrastructure-event`, repository-bloat sub-type (Pattern 3 signature
  satisfied on every still-checkable criterion).
- **Alert disposition:** **false-positive / no action.** False-positive **Rule 2 fires**
  (crash → retry → success; the fleet's own fifth attempt resolved it — a self-healed
  transient failure). Rules 1 and 3 were checked live and do not fire: no commit within four
  days of the deaths (so these were mid-task kills, not post-completion cleanup), and no
  surge in the window (§4). Today the closed-bead filter also fires — bf-2xygo is CLOSED
  with its deliverable on main.
- These are two different questions answered separately: what *killed* the worker
  (infrastructure), versus what the *alert warrants* (nothing — already healed, four days
  stale by the time the alert chain reached an investigation bead on 2026-09-02).

The crash has now been independently examined at least **9 times across 7 beads**: bf-36tp5,
bf-2igib (closed 4×), bf-2lxwt (closed 5×), domchk-71e698fe (the original 2026-08-25
investigation), domchk-3042abf1 (2026-09-01 duplicate-alert verification),
domchk-cd364e0a (raw-log extraction), domchk-fe1e4a60 (classification), plus this bead's
consolidation. Every record converges on "no domain-check defect"; the mechanism narrative
matured from CPU saturation → repository bloat as evidence accumulated.

## 6. What this incident contributed beyond its own resolution

bf-2xygo is a small instance (4 kills) of the storm that produced this repo's permanent
defences, and its record supplied three corrections of lasting value:

1. **The pre-journald evidence standard.** bf-2xygo demonstrates that Aug-12-era crash
   attributions cannot cite kernel records — they don't exist — and must rest on the
   Pattern-3 signature. Records that claim otherwise (kernel lines, measured load at kill
   time) for Aug-12 are anachronisms.
2. **Launch-gate load samples are not crash telemetry.** The Aug-12 log *does* contain CPU
   metrics — **[LIVE]** recount: 545 `fleet.cpu_saturated` events and 82
   `worker.launch.deferred` events in the source log — but they are emitted at
   dispatch-evaluation moments. Any per-crash "load caused the kill" reading built on them
   is partly mechanical correlation.
3. **A duplicate-alert cost data point.** Eight investigation beads before any doc landed;
   the 2026-08-25 investigation's mechanism claim then propagated to a 2026-09-01
   duplicate-alert verification before being corrected. Cheaper closure at the first
   "already resolved" check would have saved seven dispatches.

## 7. Corrections to prior records

1. **The 2026-08-25 investigation's mechanism claim (`CPU saturation killed the workers`)
   is superseded.** Its four load samples (9.11 / 9.4 / 8.47 / 8.21) are real recorded
   launch-gate values — the correction chain that established this is in
   `docs/crashes/bf-2xygo-crash-classification-2026-09-06.md` — but saturation was chronic
   all day (events in every hour 05–23), so load alone cannot discriminate bf-2xygo's four
   deaths from the day's other 451 kills, and the established kill mechanism for git
   operations on the bloated repo is memcg OOM at the dispatch-scope bound. A dated
   superseded-banner now heads that document.
2. **The raw-logs README's original "no load or CPU metrics of any kind" claim was wrong**
   and is already corrected in place by domchk-fe1e4a60's appended note. This report
   re-verified the counts live (§6) rather than citing either version.
3. **"29 events written per attempt" (Aug-25 doc) is wrong** — the true sequence is
   29/29/28/35/25. The Aug-25 doc's attempt table (timestamps, durations, exit codes) and
   prompt size (70,650) verify byte-exactly against the raw extract.

## 8. Mitigation strategies already in place

The bloat that killed these workers is repaired and structurally closed. **[LIVE]** — every
item below re-executed 2026-09-06 by this bead, not copied:

| Mitigation | Live check | Result |
|---|---|---|
| Repository repair | `du -sh .git` / `git count-objects -vH` | **97 MB** (was ~18 GB); 130 loose objects / 4.22 MiB; 10,980 packed / 90.93 MiB; **0 garbage** |
| Integrity | `git fsck --connectivity-only` | exit 0 (two dangling trees — normal churn) |
| Cannot recur via `.beads/` | `.gitignore` | whole-directory ignore, 0 tracked files |
| Pre-commit backstop | `.git/hooks/pre-commit` | blocks staged files > 10 MB (would have blocked the 237 MB snapshots) |
| Pack-memory bounds on bare gc **and** push | `./scripts/setup-git-gc-config.sh --verify` | ✅ effective (system → global → local); worst case ≈3 GiB per pack run, within the 6 GiB ceiling for a 12 GiB dispatch scope |
| Bounded maintenance path | `scripts/safe-git-gc.sh` | staged, checkpointed, memory-limited gc; bare `git gc --aggressive` remains prohibited |
| Detection | systemd user timers | all six firing: service-monitor (2 min), resource-monitor (5 min), crash-pattern (10 min), repo-health (daily 02:00), incremental gc (daily 03:00), full gc (weekly Sun 04:00) |

Response-side procedures this incident's records now feed: `docs/crash-response-guide.md`
(classification + false-positive rules applied here), `docs/crash-mitigation-strategies.md`,
and `docs/maintenance/repository-maintenance-guide.md` (operating procedures and thresholds).

## 9. Recommended actions

**None required.** The cause is repaired and held (97 MB, fsck clean), prevention is layered
(gitignore → pre-commit gate → pack-memory bounds → daily automated health check), and the
self-healing retry that resolved the incident on the evening of 2026-08-12 needed no help.

Explicit non-actions, so the next reader doesn't re-open them:

- **No code change** — domain-check code is not in this failure path.
- **No re-dispatch** — the subject bead is closed with its deliverable on main; the
  analysis it produced (`docs/notes/forgejo-github-sync-analysis-2026-08-26.md`) is the
  record of record for the Forgejo/GitHub divergence question.
- **No new mitigation** — the Aug-12 gap (no kernel records) is closed permanently only by
  time: journald exists since 2026-08-15, and every crash since then is kernel-attributable.
- **Do not cite** the Aug-25 doc's load table as measured kill-time telemetry, or its
  CPU-saturation mechanism as current (§7).

## 10. Source index

**bf-2xygo-specific**

- `docs/crash/bf-2xygo/raw-logs/` — 91 raw needle events + provenance README + manifest (domchk-cd364e0a, fb7bedd)
- `docs/crashes/bf-2xygo-crash-classification-2026-09-06.md` — classification layer (domchk-fe1e4a60, 40952f5)
- `docs/archive/crash-investigations/crash-investigation-bf-2xygo-2026-08-12.md` — original 2026-08-25 investigation (mechanism superseded §7)
- `docs/archive/crash-investigations/verification-report-domchk-3042abf1-duplicate-alert-resolved-bf-2xygo-crash.md` — 2026-09-01 duplicate-alert verification
- `docs/notes/forgejo-github-sync-analysis-2026-08-26.md` — the crashed task's deliverable (3838a30)

**Canon and related incidents**

- `docs/crash-response-guide.md` — classification method used here
- `docs/crash-mitigation-strategies.md`, `docs/maintenance/repository-maintenance-guide.md` — mitigation and operating procedure
- `docs/investigations/root-cause-determination-domchk-6281555d-2026-09-06.md` — canonical memcg-OOM root-cause determination
- `docs/crashes/repository-bloat-crash-bf-1s6c3-2026-08-12.md` — the storm bf-2xygo sits inside
- `docs/crashes/bf-198ne-crash-report.md` — kernel-proven push-side variant of the same mechanism (2026-08-16)
- `docs/crashes/bf-4yjq-consolidated-findings-domchk-4ed0544b-2026-09-06.md` — sibling consolidated findings for the same evening's largest bead-local storm
- `docs/crashes/signal-minus1-root-cause-analysis-verified-2026-09-02.md` — exit −1 semantics
- `docs/crashes/crash-documentation-index-2026-09-02.md` — index entry for this report
