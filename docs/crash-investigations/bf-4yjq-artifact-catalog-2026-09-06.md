# Crash Artifact Catalog: bf-4yjq — live-verified inventory (2026-09-06)

**Subject bead:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo mirror has diverged/gone stale" (P2, closed 2026-08-17)
**Dispatch bead:** domchk-e92faa40
**Catalog date:** 2026-09-06
**Scope:** every file that holds bf-4yjq crash evidence, with full paths, file types, and the
confirmed absences. Every path below was checked live on this date; nothing is inherited from
prior reports.

**Relationship to existing reports.** The crash itself is analyzed in the canonical report
[`bf-4yjq-crash-investigation.md`](bf-4yjq-crash-investigation.md) (domchk-4eab7c59). This
catalog is the evidence *inventory* that report lacked: it re-derives the canonical 50-crash
count from a source that report did not use (§2), corrects two of its evidence statements
(§2, §4), and lists exactly which paths cited by the earlier artifact catalogs no longer exist
(§6). Older path lists: [`docs/crash-artifacts-bf-4yjq.md`](../crash-artifacts-bf-4yjq.md) and
[`docs/crash-artifacts-bf-4yjq-raw.md`](../crash-artifacts-bf-4yjq-raw.md) — both now carry a
banner pointing here; their inventories are superseded.

---

## 1. Workspace where bf-4yjq ran

**`/home/coding/domain-check`** — attested by primary evidence, not inferred: every one of the
225 bf-4yjq worker-log records carries `needle.workspace=/home/coding/domain-check` on worker
`claude-code-glm-4.7-lab-domain-check` (§2), and the bead store holding all 50 alert beads is
this workspace's `.beads/`.

## 2. Primary crash-era evidence — needle fleet log slot `.log.2` ⚠ last surviving Aug-12 source

**Path:** `/home/coding/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2`
**Type:** plaintext structured worker log (one tracing event per line) — **not** a session
trace, core dump, or kernel log.
**Size / coverage:** 134,207,883 bytes; first line 2026-08-11T14:12:54Z, last 2026-08-15T20:35:19Z.
**bf-4yjq records:** 225 lines, 2026-08-12T17:50:23Z → 2026-08-12T21:14:59Z.

Event breakdown (re-counted live 2026-09-06):

| Records | Event | Detail |
|---------|-------|--------|
| 56 | `claimed bead via claim_auto` | 50 crash runs + 4 timeout runs + 1 exit-1 run + 1 exit-0 run |
| 50 | `handling agent outcome … exit_code=-1 outcome=Crash(-1)` | deaths 17:53:53.875 → 20:30:38.310 UTC (full list below) |
| 50 | `agent crashed — releasing bead and creating alert … signal_code=-1` | one per crash |
| 50 | `crash alert bead created …` | one per crash |
| 1 | `exit_code=1 outcome=Failure` @ 18:00:17Z | mid-storm dispatch that *errored* rather than being killed |
| 4 | `exit_code=124 outcome=Timeout` @ 20:40:47 / 20:51:01 / 21:01:14 / 21:11:27Z | "agent timed out — releasing bead as deferred" |
| 1 | `exit_code=0 outcome=Success` @ 21:14:56Z | **did not complete the task** — "agent exited successfully but bead is still open (orphaned) status=blocked" |
| 3 | auto-split triggers (`failure_count=3/4/5`, threshold=3) | the origin of the alert-umbrella/split debris |

Two consequences for the record:

1. **The canonical 50-crash count is independently confirmed** from worker telemetry, a source
   the canonical report did not use (it reconstructed from the alert-bead corpus alone). The
   two sources agree to the second: worker-log deaths 17:53:53.875 → 20:30:38.310 vs
   alert-bead creations 17:54:00.249 → 20:30:43.716 UTC.
2. **The disruption window extends past the crash window.** Canonical §3 records crashes
   ending 20:30:43; the worker log shows what happened next — 4 dispatch timeouts (~10 min
   cadence, the runs now surviving long enough to time out instead of dying instantly) and one
   exit-0 run that still failed to close the bead, ending 21:14:59Z. The bead was finally
   completed and closed 2026-08-17 00:14 UTC. Canonical §3's "No raw session evidence survives"
   remains true for *session* evidence (no core dumps, no stack traces, no heartbeats), but
   should not be read as "no telemetry survives" — worker outcome records do.

**Retention risk:** this file is rotation slot `.2` of a size-based (~128 MiB) rotation. Two
more rotations of the worker log erase it. It is the only primary source for this storm; §2.1
preserves the extract.

### 2.1 Preserved extract — the 50 crash-death timestamps (UTC, 2026-08-12)

```
17:53:53.875682  18:03:30.710053  18:06:05.303174  18:11:59.571601  18:14:43.825651  18:18:13.430332
18:19:44.011796  18:22:09.786227  18:25:21.962973  18:26:56.097018  18:28:32.311394  18:34:00.295945
18:38:03.981623  18:41:23.839455  18:43:18.976726  18:49:45.806773  18:52:01.838547  18:54:12.740376
18:56:13.176438  18:58:57.531893  19:02:20.135447  19:04:05.473758  19:05:38.406013  19:07:48.522799
19:11:23.767222  19:13:28.518894  19:15:55.126946  19:21:05.984457  19:24:52.238482  19:29:19.580158
19:31:12.646296  19:35:48.434578  19:40:05.963318  19:42:41.509575  19:44:22.893241  19:50:04.302016
19:53:23.400905  19:54:38.951525  19:58:33.352761  20:04:52.564574  20:06:33.824617  20:10:15.288642
20:12:31.602783  20:14:17.489742  20:16:47.736554  20:18:37.904674  20:20:43.604092  20:24:01.573430
20:25:59.204840  20:30:38.310348
```

Non-crash outcome timestamps (same source): exit-1 18:00:17; timeouts 20:40:47, 20:51:01,
21:01:14, 21:11:27; orphaned exit-0 21:14:56.

## 3. Bead-store artifacts (`/home/coding/domain-check/.beads/`)

| Path | Type | Holds | Status |
|------|------|-------|--------|
| `.beads/checkpoint/forensic.jsonl` | JSONL checkpoint (13.3 MB) | The 50 alert beads below, with descriptions, labels (`alert`, `crash`, `signal--1`, `failure-count:N`, `umbrella`, `verification-failed`, `split-child`), deps | **Live but not durable — `.beads/` is gitignored in this repo** |
| `.beads/beads.db` | SQLite | Live bead store | Live |
| `.beads/traces/bf-4yjq/` | — | — | **ABSENT.** bf-4yjq's own 56 crash-era dispatches predate trace capture; it was never re-dispatched after capture began |
| `.beads/crash-bf-4yjq-summary.txt` | text (14,498 B, 2026-09-01 18:52 EDT) | Contemporaneous metrics compilation | Superseded on crash count (canonical §10) |
| `.beads/crash-bf-4yjq-resolution.md` | Markdown (10,152 B, 2026-09-01 19:10 EDT) | Resolution record | Retained |
| `.beads/crash-reports/` | directory | — | Exists, **empty** |
| `.beads/receipts/` | directory | — | Exists, **empty** |
| `.beads/state/crash-resolutions.json` | JSON | Crash-resolution tracking | **No bf-4yjq entry** (2 entries, other beads) |
| `.beads/logs/*` (`crash-monitor.log`, `crash-alert-manager.log`, …) | text | Monitoring stack | Starts 2026-09-01; **zero bf-4yjq lines** — postdates the storm |
| `.beads/heartbeats.jsonl` | JSONL (425 lines) | Worker heartbeats | Earliest 2026-08-15T12:06Z — **nothing from Aug 12** |
| `.beads/events.jsonl` | JSONL (2.3 MB) | Bead event stream | **Zero bf-4yjq records** |

**Task work products** (context, not crash evidence): `.beads/divergence-ancestor.json`,
`.beads/divergence-point.json`, `.beads/github_commits_analysis.json` — all 2026-08-13, the
sessions that finally ran once the storm ended.

### 3.1 The 50 alert beads (crash-alert corpus)

All titled "ALERT: Agent crash on bead bf-4yjq", created 17:54:00.249 → 20:30:43.716 UTC on
2026-08-12, in creation order:

```
bf-276uk bf-3dq63 bf-59bwz bf-3ssnm bf-2fiyo bf-29rca bf-uoyie bf-2weev bf-2ftau bf-44x3a
bf-64hxa bf-3b9rv bf-1dxk7 bf-hw4i5 bf-1ygk6 bf-2j99a bf-9b8oe bf-d7j07 bf-46ttc bf-2dj1g
bf-bkpuh bf-x5ynu bf-4tl4v bf-1dzwv bf-aruwg bf-2o8p2 bf-2t7xh bf-4wi3v bf-1fvk2 bf-22514
bf-35bhc bf-3f6ue bf-mlv3u bf-5egrf bf-bykl0 bf-4tnae bf-3k3ya bf-5966o bf-vcsxj bf-19qh7
bf-mus1k bf-50zoz bf-47ugw bf-3pee6 bf-1o4ag bf-6awu2 bf-gz3r6 bf-1jxy8 bf-66h5p bf-2n3ve
```

Current statuses (from the checkpoint's latest per-bead state, 2026-09-06):
**27 open, 13 closed, 9 in_progress, 1 deferred** — the alert-hygiene debt canonical §9.2
recommends sweeping.

## 4. Per-alert-bead traces — `.beads/traces/<alert-id>/`

36 of the 50 alert beads have a trace directory. Each holds exactly four files (verified
identical across sampled dirs):

| File | Type | Content |
|------|------|---------|
| `metadata.json` | JSON | exit_code, outcome, duration_ms, token counts, `captured_at`, trace_format |
| `trace.jsonl` | JSONL | structured agent events (messages, tool calls) |
| `stdout.txt` | JSONL session transcript (0.7–1.4 MB) | full session stream |
| `stderr.txt` | text | systemd scope invocation line + hook warnings |

**None contain Aug-12 crash-era session data.** Traces are single-slot (last dispatch only),
and every surviving capture is a post-storm re-dispatch of the *alert bead*:

- `captured_at`: 4 on 2026-08-17, 20 on 2026-08-26, 6 on 2026-09-01, 6 on 2026-09-02
- exit codes: 32 × exit 0 (success), 4 × exit 1 (failure: `bf-2t7xh`, `bf-gz3r6`, `bf-mus1k`, `bf-x5ynu`)
- **14 alert beads with no trace dir at all:** bf-1dxk7, bf-1o4ag, bf-1ygk6, bf-2fiyo, bf-2n3ve,
  bf-3b9rv, bf-3dq63, bf-3ssnm, bf-44x3a, bf-59bwz, bf-64hxa, bf-66h5p, bf-9b8oe, bf-uoyie

*Correction to canonical §3:* it states `.beads/traces/` "holds entries only for later re-runs
(Aug 26 / Sep 1, exit 0)". Actually 36 of the 50 exist, including an Aug-17 wave, and 4 hold
exit-1 failures.

## 5. Documentation artifacts (`docs/`)

183 files mention bf-4yjq; 27 have it in the filename. Roles per canonical §10:

**Canonical / verified primary**
- `docs/crash-investigations/bf-4yjq-crash-investigation.md` — canonical report (commit db3f1f2)
- `docs/crash-circumstances-bf-4yjq-domchk-d5dd1b33-2026-09-02.md` — 50-crash verification (commit 55dab07)

**Resolution / status records**
- `docs/crash-investigation-bf-4yjq-summary-2026-08-26.md`, `…-summary-2026-09-01.md`, `…-final-summary.md`,
  `docs/crash-investigation-bf-4yjq-2026-08-12.md`, `docs/crash-investigation-report-bf-4yjq-comprehensive.md`,
  `docs/crash-investigation-report-bf-4yjq-final.md`, `docs/notes/agent-crash-bf-4yjq-investigation-summary.md`,
  `docs/notes/crash-bf-4yjq-investigation.md`, `docs/crashes/bf-4yjq-crash-evidence-summary.md`

**Superseded on crash count/cadence (9-crash era) — retained for other facets**
- `docs/crashes/bf-4yjq-crash-report.md` (banner-linked to canonical), `docs/reports/bf-4yjq-comprehensive-crash-report.md` (banner-linked),
  `docs/crash-artifacts-bf-4yjq.md` + `docs/crash-artifacts-bf-4yjq-raw.md` (banner-linked here),
  `docs/crash-root-cause-bf-4yjq.md`, `docs/crash-pattern-analysis-bf-4yjq.md`, `docs/crash-data-extraction-bf-4yjq.md`,
  `docs/research/crash-data-extraction-bf-4yjq.md`, `docs/research/crash-context-analysis-bf-4yjq-2026-09-01.md`,
  `docs/crash-context-bf-4yjq-comprehensive.md`, `docs/crash-context-report-bf-4yjq-comprehensive.md`,
  `docs/remediation-strategy-bf-4yjq.md`,
  `docs/crash-reports/bf-4yjq-crash-investigation.md` (added 2026-09-06, domchk-479d7eaf — the only
  doc holding the Aug-25 single-event analysis; banner-linked here, and its §"Timestamp
  Discrepancy" is resolved in that banner: `18:27:01.995975627Z` = alert bf-44x3a after death
  `18:26:56.097018Z`, `19:04:11.819822892+00:00` = alert bf-x5ynu after death `19:04:05.473758Z`)

**Duplicate-alert verification reports** (the auto-split/re-dispatch cycle's paper trail)
- `docs/verification-report-bf-1vuk2-duplicate-alert-resolved-bf-4yjq-crash.md`,
  `docs/verification-report-bf-1ygk6-duplicate-alert-resolved-bf-4yjq-crash.md`,
  `docs/verification-report-bf-uoyie-duplicate-alert-resolved-bf-4yjq.md`

**Post-storm worker-log coverage** (re-dispatch waves of the alert beads): current
`needle-claude-code-glm-4_7-lab-domain-check.log` (5 records 2026-08-17, then 2026-08-25 → 09-02),
`…lab-domain-check-2.log` (2026-08-26), `…lab-roam-1.log` (44 alert-bead claims on 2026-08-16 + 1
"Gather crash artifacts and context for bf-4yjq" claim), `…lab-test-fix.log` (2026-08-16),
`…lab-roam-5.log` (2026-09-01/02). None of these hold crash-era data; they document the
duplicate-alert cycle.

## 6. Confirmed absences

| What | Why it does not exist |
|------|----------------------|
| Core dumps | `coredumpctl` inventory's earliest entry is 2026-08-17 16:01 EDT (SIGABRT, pdftract, COREFILE missing); nothing from Aug 12, nothing needle/agent-related. Consistent with SIGKILL-class deaths |
| Kernel OOM / journald records | System journal starts 2026-08-15 19:46:33 EDT; user journal 2026-08-17 15:33:14 EDT (single boot, re-verified today). The Aug-12 kernel-level step of the root-cause chain is therefore unrecoverable — the reason canonical §6 caps mechanism confidence at MEDIUM-HIGH |
| `.beads/traces/bf-4yjq/` | See §3 — dispatches predate trace capture |
| Crash-era session transcripts | Per-attempt agent JSONL is deleted; trace capture did not yet exist |
| Stale paths cited by `docs/crash-artifacts-bf-4yjq.md` | `.beads/issues.jsonl` (retired bf-shaped store; this workspace is bead-rs with `beads.db` + `checkpoint/`), `.beads/traces/bf-4yjq/`, `.beads/traces/bf-3b9rv/`, and `bf-5e1jao-investigation-summary.md` (project root) — **none exist today** |

## 7. Root cause — by reference only

This catalog locates evidence; it does not re-derive the analysis. Canonical determination:
INFRASTRUCTURE — resource exhaustion (OOM) during git operations on the bloated 18 GB
repository; HIGH confidence on the bloat correlation, MEDIUM-HIGH on the mechanism. Note the
corpus-level mechanism correction of 2026-09-06 (domchk-e843c4f1, commit c700252): the verified
mechanism class for needle dispatch deaths is kernel **memcg** OOM SIGKILL inside each
dispatch's `MemoryMax=12 GiB` scope, and `exit_code=-1` is needle's sentinel for an unrecorded
signal death. For the Aug-12 storm specifically the kernel-level step remains unverifiable (§6),
so neither formulation should be restated as directly proven for this event. The §2 extension
(4 timeouts + 1 orphaned success after the last crash) is consistent with the environment
degrading then recovering and does not change the classification.

## 8. Provenance

All counts, timestamps, statuses, and absences in this catalog were re-derived live on
2026-09-06 by dispatch domchk-e92faa40 from: `~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2`
(grep on `bf-4yjq` + event-shape tally), `.beads/checkpoint/forensic.jsonl` (parsed alert-bead
corpus), `.beads/traces/*/metadata.json`, `coredumpctl list`, `journalctl` / `journalctl --user`
first lines, and per-path `stat`/`ls`. No figure is inherited from a prior report; where this
catalog differs from one, the difference is stated explicitly (§2, §4, §6).
