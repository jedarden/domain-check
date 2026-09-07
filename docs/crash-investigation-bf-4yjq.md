# Crash Evidence: bf-4yjq

**Dispatch:** domchk-4e32a3a4 (evidence-collection task)
**Evidence date:** 2026-09-06
**Subject:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo mirror has diverged/gone stale" (P2, **closed** 2026-08-17T00:14:14Z)
**Dispatch-supplied crash timestamp:** 2026-08-12T19:54:44
**Classification:** **INFRASTRUCTURE** (per `docs/crash-response-guide.md`)

**Scope of this record.** bf-4yjq already has a canonical investigation
([`crash-investigations/bf-4yjq-crash-investigation.md`](crash-investigations/bf-4yjq-crash-investigation.md)), a
committed classification record
([`crash-investigations/bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md`](crash-investigations/bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md)),
and a live-verified artifact catalog
([`crash-investigations/bf-4yjq-artifact-catalog-2026-09-06.md`](crash-investigations/bf-4yjq-artifact-catalog-2026-09-06.md)).
This page does not overturn any of them. What it adds: (1) it resolves the **exact timestamp this
dispatch was dispatched with** — `2026-08-12T19:54:44` — to the specific death event it belongs to,
at second precision, from the preserved raw event log; (2) it re-verifies every claim live on
2026-09-06 rather than inheriting it; (3) it records, as checked absences, which log sources cannot
exist for this event and why.

---

## 1. What actually crashed — and what did not

bf-4yjq **is not a crash task**. It is a git-remote reconciliation task (Forgejo-primary origin +
server-side push mirror), created 2026-07-20 and **closed successfully** on 2026-08-17 after the
environment that was killing its agents was repaired. The "crash on bf-4yjq" is the name the
alert system attached to the 2026-08-12 infrastructure storm in which **50 consecutive agent
dispatches for this bead died with exit code −1** on a repository bloated to ~18 GB (17.16 GiB of
loose objects). The bead's task itself was completed and verified; see §7.

Per the response guide's quick-reference table, **exit code −1 → Infrastructure event**. The
other three categories are excluded in the committed classification record and summarized in §6.

## 2. Crash timestamp and context

### 2.1 Resolving the dispatch timestamp `2026-08-12T19:54:44`

The dispatch prompt supplied `2026-08-12T19:54:44` as the crash time. That timestamp **is a real
record**, but it is not a death timestamp. It is a `HANDLING_RELEASE_DONE` **heartbeat** in the
preserved needle event log, emitted **5.7 seconds after** the death it belongs to
([`crash/bf-4yjq/raw-logs/needle-events-2026-08-12-bf-4yjq.jsonl`](crash/bf-4yjq/raw-logs/needle-events-2026-08-12-bf-4yjq.jsonl),
line 744, seq 4025).

The full cycle around it, from the same log (all times UTC, 2026-08-12):

| Time | Event | Meaning |
|------|-------|---------|
| 19:53:23.399 | `agent.completed` exit_code=−1 | prior run's death (37th of 50) |
| 19:53:34.329 | `agent.dispatched` | automatic retry |
| **19:54:38.950** | **`agent.completed` exit_code=−1** | **the death this dispatch's timestamp belongs to** (38th of 50) |
| 19:54:38.951 | `outcome.classified` → Crash(−1) | needle classifies the kill |
| 19:54:44.676 | `heartbeat.emitted` HANDLING_RELEASE_DONE | **← the timestamp in the dispatch** |
| 19:54:46.810 | `bead.released` | bead handed back for the next retry |
| 19:54:48.902 | `bead.claim.succeeded` → dispatched | 39th run begins |

**Consequence for triage:** treat alert/heartbeat timestamps as upper bounds. This dispatch's
timestamp trails its death by 5.7 s; the same storm's alert-bead creation timestamps trail by
up to ~7 s (17:53:53.875 death → 17:54:00.249 alert). The death itself is the
`agent.completed exit_code=−1` record, not the heartbeat.

### 2.2 The storm

| Quantity | Value | Source |
|----------|-------|--------|
| Dispatches in preserved window | 56 (session `8446529e`) | raw events jsonl, counted live 2026-09-06 |
| Deaths, exit −1 | **50** | same |
| Other outcomes | 4 × exit 124 (600 s timeouts, 20:40–21:11Z), 1 × exit 1, 1 × exit 0 (orphaned — bead still open afterward) | same |
| First death | 2026-08-12T17:53:53.875Z | same; worker log `.log.2` agrees to the second |
| Last exit−1 death | 2026-08-12T20:30:38.310Z | same |
| Disruption window end | 21:14:59Z (last timeout/orphaned run) | worker log `.log.2` |
| Median dispatch→death survival | ~184 s (min 65 s, max 600 s) | raw events jsonl, 56 runs |
| Median gap between deaths | ~156 s (peak 5 deaths in any 10-min window, ≈19/hour) | classification record §False Positive |
| Crash cadence | ~3.1 min between deaths | canonical report §3 |

The event log shows the retry loop plainly: claim → dispatch → 65–375 s of real work → kill →
release → redispatch ~10 s later. Each run got far enough to be mid-task; none lasted to the
600 s dispatch timeout until late in the window, when runs began surviving long enough to time
out instead (4 × 124 after 20:40Z) — the same environmental pressure, a longer tail.

### 2.3 Log-source coverage for this timestamp — checked absences

| Source | Coverage of 2026-08-12 | Result |
|--------|------------------------|--------|
| System journal (`journalctl`) | **starts 2026-08-15T19:46:33 EDT** (boot 523096…) | **No kernel/OOM records for Aug 12 exist.** Checked live 2026-09-06. |
| coredumpctl | earliest entry 2026-08-17 16:01 EDT | No core dumps for this event (SIGKILL prevents them by design, and none exist) |
| `.beads/logs/` | starts 2026-09-01 | Not applicable |
| Needle worker log slot `.log.2` | 2026-08-11T14:12Z → 08-15T20:35Z | **Exists** — 128 MiB, 225 bf-4yjq lines, the storm's only primary source (rotation risk, §3) |
| Preserved extract `crash/bf-4yjq/raw-logs/` | events jsonl + worker-log extract + session tarball, `MANIFEST.sha256` | Exists, committed |

The kernel-OOM step of the kill chain is therefore **not directly provable for this event** —
the records that would prove it postdate the storm. The committed classification record holds
the OOM attribution at **MEDIUM-HIGH confidence**, on the strength of: exit −1 across 50/50
runs, contemporaneous repo telemetry (18 GB / 17.16 GiB loose), load 15–17 on 12 cores, and the
fleet-wide verification of this exact death class (memcg OOM inside the 12 GiB dispatch scope)
for later events with surviving kernel records. Older documents stating "Signal −1 = SIGKILL
from the OOM killer" as fact overstate the evidence for this specific date; downstream readers
should not restate it as directly proven.

## 3. Available logs and artifacts

All paths checked live 2026-09-06.

**Primary (crash-era) evidence:**

| Artifact | Path | Notes |
|----------|------|-------|
| Needle worker log, slot `.log.2` | `/home/coding/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2` | **Still on disk** (128 MiB, 225 bf-4yjq lines). Only surviving primary source; size-based rotation (~128 MiB/slot) could erase it. |
| Preserved events extract | [`crash/bf-4yjq/raw-logs/needle-events-2026-08-12-bf-4yjq.jsonl`](crash/bf-4yjq/raw-logs/needle-events-2026-08-12-bf-4yjq.jsonl) | 1,071 events, 17:50:23Z→21:15:03Z, session 8446529e — the source for §2 |
| Preserved worker-log extract | [`crash/bf-4yjq/raw-logs/needle-worker-log-bf-4yjq-slot2.log`](crash/bf-4yjq/raw-logs/needle-worker-log-bf-4yjq-slot2.log) | 225 lines, one per dispatch |
| Crash-session tarball + index | [`crash/bf-4yjq/raw-logs/bf-4yjq-crash-sessions-2026-08-12.tar.gz`](crash/bf-4yjq/raw-logs/bf-4yjq-crash-sessions-2026-08-12.tar.gz), `sessions-index.tsv` | with `MANIFEST.sha256` and `README.md` |
| 50 death timestamps | [`crash-investigations/bf-4yjq-artifact-catalog-2026-09-06.md`](crash-investigations/bf-4yjq-artifact-catalog-2026-09-06.md) §2.1 | re-derived from `.log.2`; agrees with the alert-bead corpus to the second |

**Analysis records (consolidating, in reading order):**

| Document | Role |
|----------|------|
| [`crash-investigations/bf-4yjq-crash-investigation.md`](crash-investigations/bf-4yjq-crash-investigation.md) | Canonical analysis — root cause, storm table |
| [`crash-investigations/bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md`](crash-investigations/bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md) | Guide-taxonomy verdict, false-positive rules applied |
| [`crash-investigations/bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md`](crash-investigations/bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md) | Per-run deaths/alerts/survival; exit-code semantics |
| [`crash-analysis/bf-4yjq-system-state-snapshot-2026-09-01.txt`](crash-analysis/bf-4yjq-system-state-snapshot-2026-09-01.txt) | Crash-era telemetry transcription (crash counts superseded — see §5) |
| [`crashes/bf-4yjq-crash-report.md`](crashes/bf-4yjq-crash-report.md) | 2026-09-01 report; carries a supersession banner (9-crash count → 50) |
| [`crashes/bf-4yjq-cleanup-verification.md`](crashes/bf-4yjq-cleanup-verification.md) | Repository repair record, criteria re-run 2026-09-06 |

**Stack traces, core dumps, stderr from the killed runs: none exist.** SIGKILL-class termination
produces none, and the 50 alert beads' `.beads/traces/` payloads carry only the standard
needle crash notice ("The agent process was killed. This bead has been released for retry.")
with the uniform exit −1. No domain-check application error text appears anywhere in the corpus
— the killed process was the dispatch agent, not the application.

## 4. What work was being attempted

Two layers, which is the source of most confusion around this event:

1. **The agent dispatches that died** were retries of bf-4yjq: reconcile this checkout's git
   remotes to the workspace's Forgejo-primary convention (fetch both remotes, diff the tips,
   create a merge commit — no force-push — repoint `origin` at
   `git.ardenone.com/jedarden/domain-check.git`, configure the Forgejo server-side push mirror,
   verify convergence). Each died 65–375 s into real work, **producing no commits**: the DAG has
   no commits anywhere in the 17:53–21:15Z storm window. The era's last commits are the
   2026-08-12 ~03:00–07:42 UTC "chore: update bead tracking files before test tag" series — the
   very commits that grew the bloat (17+ commits each carrying a ~237–248 MB `.beads/*.jsonl`
   snapshot from bead bf-2ildm's extraction).
2. **The bead itself** was closed successfully on 2026-08-17T00:14:14Z by its closure-bead
   blocker (domchk-ab442ed9), after the repository was cleaned. Live re-verification
   2026-09-06: `origin` → `https://git.ardenone.com/jedarden/domain-check.git` ✅, local `main`
   == `origin/main` (`88846703`) ✅, `bead show bf-4yjq` → Closed ✅. (A local `github-mirror`
   remote remains configured — a leftover noted by the canonical report §8; mirroring is meant
   to be server-side on Forgejo. Out of scope here.)

So: the crashes were **mid-task** (no commit, no completion, killed during work), and the task
they belonged to **did complete** once the environment was repaired. An older snapshot
(`crash-analysis/bf-4yjq-system-state-snapshot-2026-09-01.txt`) describes the bead as "95%
complete / BLOCKED at crash time"; the verified record is that no run in the storm completed
anything, and the completion came from the later, post-cleanup pass.

## 5. System state

**At crash time** (contemporaneous telemetry preserved in
`crash-analysis/bf-4yjq-system-state-snapshot-2026-09-01.txt` and the canonical report §6):

| Metric | Value at crash time | Healthy threshold |
|--------|--------------------|-------------------|
| Repository size | ~18 GB | < 500 MB |
| Loose objects | 17.16–17.20 GiB, ~4,594 objects | < 100 MB |
| Packed | 9.60 MiB (ratio ≈ 1,800:1, inverted) | — |
| Committed bloat | ~237–248 MB `.beads/*.jsonl` × 17+ commits | `.beads/` untracked |
| Load average | 15–17 on 12 cores | < 5 |
| Memory | effectively exhausted during git operations | ≥ 20 GB available |
| Disk | 84% full, ~71 GB free | > 50 GB free |
| `git fsck` | timed out after 2 minutes | completes |

**Today** (live check 2026-09-06, this dispatch):

| Metric | Value | Verdict |
|--------|-------|---------|
| `.git` size | 93 MB (15 loose objects / 96 KiB; 10,980 objects in 2 packs / 90.93 MiB; 0 garbage) | ✅ |
| Memory available | 47 GiB of 62 GiB | ✅ |
| Load average | 0.38 (12 cores) | ✅ |
| Disk | 86% used, 61 GB free | ✅ |
| `git remote -v` / branch sync | Forgejo origin; HEAD == origin/main | ✅ |

The trigger condition — a multi-gigabyte loose-object store that every significant git
operation had to touch — no longer exists, which is why this crash class has not recurred.
Prevention in force: `.beads/` gitignored (`0` tracked files), 10 MB pre-commit gate,
`pack.windowMemory=2g`/`pack.threads=1` bounding bare gc and push, daily repo-health + gc
timers.

## 6. Classification per docs/crash-response-guide.md

| Category | Verdict | Basis |
|----------|---------|-------|
| **INFRASTRUCTURE** | ✅ **This event** | Exit −1 on 50/50 runs → guide's "-1 = Infrastructure event" row. Contemporaneous resource telemetry at exhaustion; deaths ceased only when the repository was cleaned and never returned. |
| CODE_DEFECT | ✗ Excluded | No application error text anywhere in the corpus; the killed process was the agent, not domain-check. Consistent with the repo-wide zero-defect finding across 157+ investigations. |
| SERVICE_FAILURE | ✗ Excluded | No HTTP 503/502, no gateway-unavailable signature anywhere in the window. |
| FALSE_POSITIVE | ✗ Excluded | All three guide rules negative: no commit within 30 s of any crash (no commits at all in the window); retry never succeeded inside the storm (50 consecutive deaths — recovery came from an environment change 5 days later, not retry luck); and the ≥10-crashes/10-min surge rule would *not* even have fired (peak 5 per 10 min), which is itself a recorded gap in the guide's detector. |

**Caveat carried forward:** exit −1 is needle's sentinel for an unrecorded signal, and the
kernel OOM record for Aug 12 cannot exist (§2.3). INFRASTRUCTURE is certain; the specific
memcg-OOM mechanism is MEDIUM-HIGH confidence for this date.

## 7. Work completion

| When | State |
|------|-------|
| At crash time | Not complete — killed mid-task, zero commits in the storm window |
| 2026-08-17 | Bead closed by its closure-bead blocker after the repo cleanup |
| 2026-09-06 (this dispatch) | Re-verified live: Forgejo origin ✅, branch sync ✅, bead Closed ✅, repository healthy ✅ |

No further action is required on the subject bead. The remaining open item in this area is
alert-hygiene debt (the storm's ~50 alert beads and the wider Aug-12 pool), which is tracked in
the canonical report §9 and is not this dispatch's deliverable.

## 8. Sources for this record

Primary evidence was read directly for this dispatch: the preserved events jsonl (§2.1 cycle,
death/survival census), the live needle worker log slot `.log.2` (existence + 225-line count),
live `journalctl`/`coredumpctl` coverage checks (§2.3), live repo/system state (§5), `bead show
bf-4yjq`, and git history for the storm window. Everything else is consolidated from the
committed record listed in §3, with superseded claims flagged where encountered (§2.3 signal
statement, §4 "95% complete" framing, and the 9-crash count superseded by 50 in
`crashes/bf-4yjq-crash-report.md`'s own banner).
