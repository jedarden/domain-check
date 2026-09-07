# Crash context — bf-1ea4g (2026-08-13)

Assembled 2026-09-07 by **domchk-cf6855ad**, the crash-context child of alert
bf-1nb5u's 2026-09-02 split. This is a context/handoff document, not a new
investigation: the artifacts live in `docs/crashes/bf-1ea4g/` (sibling
domchk-93ac565b) and the corpus record is
`docs/crash-inventory-bf-1ea4g-summary.md`. What this file adds is the
resolution of the **other** alert instant this split's children were
dispatched with (08:45:05, vs. the bundle's 08:23:51), the era-vs-live state
split a classifier needs, and the handoff in §7. It duplicates no bundle
content.

**Bottom line.** bf-1ea4g itself was never a crash — it is a 2026-08-13
"document local main branch state" task that completed and closed at
09:10:16.731Z after 57 dispatches, 56 of which were killed exit −1. Both alert
instants named by this split (08:23:51.806 and 08:45:05) are post-kill
`HANDLING_RELEASE_DONE` heartbeats of that single loop (attempts 30 and 42),
~6–7 s after the real kills. Work self-completed 47 min after the first kill,
so the alerts were false positives at the alert layer; the kill mechanism
(INFRASTRUCTURE-era) is documented and remediated — unbounded `git push`
pack-objects over the 422-commit unpushed backlog, bounded by
`pack.windowMemory` since.

## 1. Work context — what bf-1ea4g was

| Field | Value |
|---|---|
| Title | Document local main branch state |
| Type / priority | task / P2 |
| Created | 2026-08-13T07:14:47.400Z |
| Closed | 2026-08-13T09:10:16.731Z (`bead show`: Status closed, Updated == close; never reopened per bf-1nb5u's resolution note, verified live 2026-09-07) |
| Deliverable | `main_branch_state_bf-1ea4g.json` (repo root) + `docs/archive/crash-investigations/local-main-state-bf-1ea4g.md` |
| Dispatches | 57 on Aug-13 (07:17:49.943Z → 09:10:39.562Z) — 56 × exit −1, 1 × exit 0 (attempt 57) |

The bead's own scope (snapshot local main's SHA/subject/timestamp to a temp
file) has no relationship to any crash mechanism; it is the *victim* of the
worker-kill loop, not its cause. The task was small — every killed attempt ran
76–134 s — which is why the loop re-dispatched every ~2 min all morning.

## 2. The instant this dispatch names — 08:45:05 — resolved

The dispatch prompt carries "exit code −1 at 2026-08-13T08:45:05". Re-derived
first-hand from `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl`
(3,111,314 bytes, mtime Aug-13 19:59 local — still on disk):

| Event | Timestamp (UTC) | Worker log line |
|---|---|---|
| Attempt-42 dispatch | 08:43:42.572992078Z | 3661 |
| **Kill: `agent.completed` `exit_code: -1` (duration 76,159 ms)** | **08:44:58.908131320Z** | 3664 |
| `outcome.classified` | 08:44:58.908647037Z | 3667 |
| `HANDLING_RELEASE_DONE` heartbeat — **the named instant** | **08:45:05.908552183Z** | 3673 |
| `bead.released` (`release_success`) | 08:45:08.202791905Z | 3674 |

The named instant is attempt 42's post-kill handling heartbeat, **6.10 s after
the actual kill** — the same alert-stamp pattern the bundle documents for
attempt 30 (heartbeat 08:23:51.806 vs kill 08:23:44.918, 6.89 s). Attempts 30
and 42 are two kills of one loop, not two crashes; any bead or dispatch naming
an Aug-13 bf-1ea4g instant resolves the same way (grep the named second against
`docs/crashes/bf-1ea4g/attempt-index.tsv`, then read back to the preceding
`agent.completed`).

## 3. Crash artifacts — where they live

All collected and hash-pinned by domchk-93ac565b (`docs/crashes/bf-1ea4g/MANIFEST.sha256`):

| Artifact | File |
|---|---|
| All 57 attempts (dispatch/completion/exit/duration, source line numbers) | `docs/crashes/bf-1ea4g/attempt-index.tsv` |
| Raw event bracket around attempt 30 (L3283–3361) | `docs/crashes/bf-1ea4g/needle-events-2026-08-13-bf-1ea4g-attempt30.jsonl` |
| Every bf-1ea4g record in the Aug-13 worker log (1,093 records) | `docs/crashes/bf-1ea4g/needle-events-2026-08-13-bf-1ea4g.jsonl.gz` |
| Attempt-30 agent session transcript (dies mid-tool-call 13.8 s before the kill) | `docs/crashes/bf-1ea4g/session-transcript-attempt30-449405f5.jsonl` |
| CPU/load evidence + documented absences | `docs/crashes/bf-1ea4g/system-state-2026-08-13T082344Z.md` |
| Durable originals | `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` and `~/.claude/projects/-home-coding-domain-check/449405f5-….jsonl` (both still on disk, re-verified 2026-09-07) |

`.beads/traces/` is a single-slot buffer emptied long ago — nothing there for
this crash (documented in the system-state file).

## 4. System resource state at the kill

**CPU/load — recorded directly** (needle `fleet.cpu_saturated`, the era's only
system-state telemetry):

| Sample | Load (9 cores) |
|---|---|
| Attempt-42 dispatch, 08:43:42.564Z | 11.18 |
| Attempt-43 dispatch, 11.5 s after the kill, 08:45:10.406Z | 11.38 |
| Window peak, 08:37:03.142Z | 19.87 |
| 07:00–10:00Z window | 71/71 samples saturated (every sample > 0.8 threshold) |

**Memory / disk at the kill — not recoverable, and no figure should be
invented.** The first memory/disk sampler on this box (lab-health-collector)
starts 2026-08-15 23:53 EDT and system journald starts 2026-08-15 19:46 EDT, so
Aug-13 has neither; needle emitted no memory or disk event types that day, and
no coredump infrastructure existed before Aug-25. `exit_code: -1` is needle's
died-without-exit-code sentinel, not a signal number. The bundle's
system-state file documents each absence with the collector start dates that
prove it; the kill's mechanism is therefore established at era level (§5), not
from this instant.

## 5. Repository bloat indicators — era vs. live

Read these as two different answers to the same checklist item:

- **At the crash (Aug-13):** inside the documented bloat era — Aug-12 measured
  ~18 GB `.git` / 17.16 GB loose objects (bf-1s6c3 / bf-4yjq), `.beads/` still
  tracked, and a **422-commit unpushed backlog** (660 on Aug-12 → 0 only after
  later plain pushes). No Aug-13 disk measurement exists (§4). The kill
  family's mechanism — unbounded `git push` pack-objects over that backlog
  inside the dispatch scope's 12 GiB `MemoryMax` — is kernel-proven for its
  Aug-16 twin (bf-198ne, 720 commits / 5.6 GB) and recorded in the inventory's
  §4.
- **Live (2026-09-07 16:34Z, this session):** `.git` **106 MB**;
  `git count-objects -vH` → 372 loose / 3.50 MiB, 1 pack / 99.11 MiB, 0
  garbage, 0 prune-packable; local↔origin divergence **0 / 0**. Healthy — the
  repaired state re-verified in repo `CLAUDE.md`. **A classifier that evaluates
  "repository bloat > 1 GB" against the live repo will wrongly clear the era's
  mechanism; era-level records are the correct input.**

## 6. Crash pattern — last 24 hours, and the Aug-13 loop it came from

- **Last 24 h (2026-09-07):** the repo's own detector, run by the monitoring
  timer at 16:30:12Z, reports **"No crashes detected in the last 24hours —
  System Status: STABLE"** (`.beads/logs/crash-monitor.log`). Nothing in the
  current fleet resembles the Aug-13 pattern.
- **The Aug-13 pattern itself:** one bead, 57 dispatches at ~2-min cadence over
  ~2 h, 56 kills exit −1, terminal attempt exit 0 → `verification.passed`
  09:10:39.573Z → bead completed 09:10:43.080Z. Single-bead retry loop under
  box-wide CPU saturation, not a workspace-wide storm.

## 7. Handoff — classification (for domchk-6b123791, blocked on this bead)

The classification is already established by closed siblings and by the parent
alert's own resolution (bf-1nb5u closed RESOLVED 2026-09-07T16:20Z, ~35 min
before this file was assembled); a dispatched classifier should verify, not
re-derive:

- **Alert layer: FALSE_POSITIVE.** Work self-completed — the bead closed
  09:10:16.731Z, 47 min after the first kill and 22 min after attempt 42's —
  so no investigation action was ever needed from the alerts the loop minted.
- **Mechanism layer: INFRASTRUCTURE-era kill.** Unbounded `git push`
  pack-objects materializing the 422-commit unpushed backlog inside the
  dispatch scope's 12 GiB `MemoryMax`; bounded since by `pack.windowMemory` /
  `pack.deltaCacheSize` / `pack.threads=1` (repo-local + global), verified by
  `./scripts/setup-git-gc-config.sh --verify` and
  `scripts/test-gc-memory-bounds.sh`.
- **Not** CODE_DEFECT (no domain-check code involvement) and not
  SERVICE_FAILURE (no gateway involvement in any record).

Citations: `docs/crash-inventory-bf-1ea4g-summary.md` (§4 mechanism, §5 alert
family of 88, line 16/43 for the 08:23 instant),
`docs/crash-analysis-bf-1ea4g-signal-minus-one-2026-09-02.md`,
`docs/investigations/bf-1ea4g-root-cause-determination-2026-09-02.md`,
`docs/crash-prevention-gaps-bf-1ea4g.md`, and the bundle README's
discrepancy list (the Sep-2 "repo healthy → not bloat" premise was anachronistic
for Aug-13).

## 8. Related open residue (as of 2026-09-07)

- Open, unassigned duplicate children of this split still in the pool:
  domchk-e940d850, domchk-c68ae9bb, domchk-05b8d837, domchk-39ada6b8,
  domchk-45b3a31c, domchk-b40518bd, domchk-d0520d3e, domchk-f8240cbe,
  domchk-0c8558e4, domchk-b1d9c1cb, domchk-de0315fe — each should resolve
  against this file and the inventory without new investigation.
- In flight (other owners): domchk-cb9eb4de, domchk-e2c1e79e.
