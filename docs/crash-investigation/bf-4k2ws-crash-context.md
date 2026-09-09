# bf-4k2ws — Crash Context

**Collected:** 2026-09-09
**Author bead:** `domchk-6ba3b2af` (gather leg)
**Target bead:** `bf-4k2ws` — "Analyze divergent Forgejo and GitHub branch states" — **Closed** (rev 2, 2026-08-16T15:35:42.024203483Z)
**Classification (current):** **INFRASTRUCTURE** — repository-bloat-era kill regime; the false-positive element is confined to the alert layer

> **What this document is.** A context extract fulfilling the gather-leg acceptance
> criteria, filed at the dispatch-named path. It is **subordinate to the canonical
> record**: `docs/investigations/bf-4k2ws-root-cause-determination-domchk-7f838f36-2026-09-07.md`
> (§§1–17) and `docs/crash-investigations/bf-4k2ws/README.md`. No figure here
> contradicts the canon; census-level numbers are quoted from it with provenance.
>
> **What this document adds beyond the canon:** the alert-bead ↔ death mapping for
> this specific dispatch (`bf-4xlwo` ↔ attempt 57), which the evidence bundle's
> mapping does not name, and the exact log record behind the template's timestamp.

---

## 0. Template-premise corrections (read first)

The dispatch template that commissioned this leg carried fields from the source
alert bead. Two of them are stale premises — exactly the pattern the canon's
lesson 10 names ("a dispatch template is a stale-premise vector"). Both were
re-derived from the primary log before being recorded here:

| Template field | As dispatched | Verified fact |
|---|---|---|
| Crash timestamp | `2026-08-13T06:34:35.805234687+00:00` | **A heartbeat stamp, not the death.** It matches the worker log's `heartbeat.emitted` record `2026-08-13T06:34:35.805222217Z` (sequence 8376) to within **12.47 µs**, i.e. the crash-report writer's clock read microseconds after that heartbeat. The actual kill is `agent.completed` at **2026-08-13T06:34:29.204954348Z** — the heartbeat is death **+ 6.600 s**, inside the canon's 5.1–9.8 s heartbeat window (§3.6) |
| Exit code | `-1 (signal -1)` | **−1 is needle's unrecorded-signal sentinel — never a signal number.** The era's mechanism (chain-inferred for this bead; kernel-proven for siblings `bf-4x12ec`/`bf-198ne`) is a memcg-OOM SIGKILL inside the 12 GiB dispatch scope. "signal -1" is the alert template's wording, not a signal that exists |
| Deliverable location | `docs/crash-investigation/` (singular) | Not the corpus home — that directory holds `bf-198ne`/`bf-1s6c3` material. This file is created here anyway so the acceptance item resolves at the named path; the canon remains authoritative |

---

## 1. Target bead — full description and status

- **ID:** `bf-4k2ws` · **Title:** "Analyze divergent Forgejo and GitHub branch states" · **Type:** task · **Priority:** P2
- **Status:** **Closed** (rev 2; closed 2026-08-16T15:35:42.024203483Z — three days after the storm)
- **Created:** 2026-08-13T01:57:53.592871267Z (a bead-existence instant, not a crash instant)
- **Assignee:** `claude-code-glm-4.7-lab-domain-check`
- **Task (verbatim summary):** pre-merge, **read-only** analysis of Forgejo vs GitHub branch divergence — document local/Forgejo/GitHub tips, list commits unique to each side, identify the divergence point, write the analysis to a file. "No merge operations are performed in this bead."
- **Outcome:** all 8 acceptance criteria met; four deliverable docs on `origin/main` (presence re-verified via `git cat-file -e`, canon §4). **No work was lost.** The divergence answer (0/0) still holds.

---

## 2. Source alert bead — `bf-4xlwo`

Every field in this leg's dispatch template comes from this alert bead's body.
It is one of the **55 ALERT beads** (`ALERT: Agent crash on bead bf-4k2ws`) that
pre-dedup needle (< 0.4.2) filed — one per kill, no fingerprint/cooldown.

Verbatim body (re-read live from the checkpoint snapshot, 2026-09-09):

```
## Agent Crash Report

- **Bead ID**: bf-4k2ws
- **Agent**: claude-code-glm-4.7
- **Exit code**: -1 (signal -1)
- **Workspace**: .
- **Timestamp**: 2026-08-13T06:34:35.805234687+00:00

The agent process was killed. This bead has been released for retry.
```

- **Created:** 2026-08-13T06:34:35.811234404Z — **5.9 ms after** the heartbeat whose stamp it carries
- **Current status:** **Open**, rev 20 (labels: `alert`, `crash`, `deferred`, `failure-count:4`, `signal--1`, `split-child`, `umbrella`, `verification-failed`; blocked by `domchk-bc924f96`). One of the 32 still-open ALERT beads at the canon's 2026-09-09 recount (21 closed / 32 open / 2 in progress)
- **New mapping (not in the evidence bundle):** `bf-4xlwo` ↔ **attempt 57**, death `2026-08-13T06:34:29.204954348Z`. Stale relative to its target: the alert fired against work that had already passed verification once (04:48:09.546Z) and whose bead closed three days later

---

## 3. The crash — attempt 57 of 62 (first-hand from the primary log)

Source: the untouched worker log
`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl`
(3,111,314 bytes, mtime 2026-08-13 19:59 — untouched), cross-checked against the
committed extract `docs/crash-artifacts-bf-4k2ws/2026-09-07T0823Z-needle-log-extract-bf-4k2ws-aug13.txt`.
All times UTC.

| Instant | Event |
|---|---|
| 06:29:08.909680204Z | `bead.claim.succeeded` — **attempt 57** (57th claim of the loop; mapping validated: claim #33 = 04:41:50.301Z is the attempt that succeeded at 04:48:09.546Z, 379 s later ✓) |
| 06:29:08.919987765Z | `agent.dispatched` (seq 8364) — agent `claude-code-glm-4.7`, session `8446529e`, template `pluck`/`pluck-default`, prompt_len 70,932 |
| 06:29:08.939617828Z | `transform.started` |
| 06:34:28.911379687Z | `transform.completed` — 319,951 ms |
| **06:34:29.204954348Z** | **`agent.completed` — exit −1, 319,831 ms (5 m 19.8 s). The kill.** |
| 06:34:29.207023780Z | `outcome.classified` — exit −1, `crash` |
| 06:34:33.461946764Z → 06:34:35.805222217Z | `heartbeat.emitted` ×4 (`HANDLING_FLUSH…` → `HANDLING_RELEAS…`, seqs 8373–8376) — **the last of these is the record behind the template's "crash timestamp"** |
| 06:34:35.811234404Z | alert bead `bf-4xlwo` written (+5.9 ms) |
| 06:34:38.234212909Z | `bead.released` (`release_success`) |
| 06:34:38.236100365Z | `outcome.handled` — `{"action":"alerted"}` (verified live in the log) |
| 06:34:40.411374321Z | next claim — attempt 58 |

**Position in the storm.** Attempt 57 falls in the **post-completion window** (attempts 34–61): it ran *after* the first verified success (04:48:09.546Z) had been orphaned (04:48:15.306Z), so it was killed re-doing already-verified work — 25 of the night's 55 kills were of this class. Its successor, attempt 58, hit the 600 s dispatch cap (exit **124**, 600,020 ms) — matching the canon's cap attempts (58, 59, 61). Attempt 57's 319.8 s duration sits mid-distribution for the kill class (overall 123.6–528.9 s, median 252.9 s — quoted from the canon's twice-byte-exact-re-derived census).

---

## 4. Exit code semantics

Exit codes in this record are **classes, not signals** (canon §3.6):

| Exit | Night's count | Meaning |
|---|---|---|
| **−1** | 55 | Needle's **unrecorded-signal sentinel**: the process died by signal and the code was not recorded. **Never name a signal from it** — the "(signal -1)" in the alert body is template wording, not a signal. For this era the mechanism is memcg-OOM SIGKILL (chain-inferred here; kernel-proven for the gc/push siblings) |
| 124 | 5 | The **600 s dispatch cap** (attempts 16, 17, 58, 59, 61) — *not* max-turns |
| 0 | 2 | Success (both `verification.passed`, both then orphaned instead of closing) |

---

## 5. Workspace and agent identity

- **Workspace:** `.` (alert body) = the dispatch's workspace root, i.e. **`/home/coding/domain-check`** — the repo the bead belongs to and the worker (`…-lab-domain-check`) serves. The worker log records **no explicit cwd field**, so the absolute-path rendering is interpretation; the `.` itself is verbatim from the alert.
- **Agent:** `claude-code-glm-4.7` (dispatched-event field, verified live)
- **Worker:** `claude-code-glm-4.7-lab-domain-check` · **Session:** `8446529e` — all 62 attempts ran on this one session
- **Dispatch template:** `pluck` / `pluck-default` · prompt 70,932 bytes, `sha256:894ca1279376b01747c03529443bac74c413e627da898173cecbbd7eb58d5a74`

---

## 6. Crash logs, core dumps, error messages — what survives

**Surviving:**

| Artifact | Where | State |
|---|---|---|
| Primary worker log (the census source) | `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` | Intact, 3.1 MB, mtime 2026-08-13 19:59 |
| Committed extract of it | `docs/crash-artifacts-bf-4k2ws/2026-09-07T0823Z-needle-log-extract-bf-4k2ws-aug13.txt` | 436 bf-4k2ws lifecycle events |
| Evidence bundle + README | `docs/crash-artifacts-bf-4k2ws/` | Collected 2026-09-07 by `domchk-8e0fc94d` |
| Alert-bead bodies | `.beads/checkpoint/` snapshots (e.g. `bf-4xlwo`) | Re-read live for this document |

**Unrecoverable (recorded so nobody re-searches):**

| Artifact | Why absent |
|---|---|
| Dispatch trace | Needle traces are single-slot; the `bf-4k2ws` slot holds nothing and the oldest surviving trace dir postdates the crash by three days (bundle `2026-09-07T0824Z-traces-check.txt`) |
| Kernel memcg-OOM records | The box rebooted 2026-08-14; the single current boot begins **2026-08-15 19:56:33 EDT** (re-verified via `journalctl --list-boots` for this document) — Aug-12/13 kernel lines are gone. Mechanism is therefore **chain-inferred** for this bead (MEDIUM-HIGH), kernel-proven only for siblings `bf-4x12ec`/`bf-198ne` |
| journald records | Same boot boundary — no journald record for the storm can exist |
| Core dumps | `coredumpctl` has **zero 2026-08-13 entries** (earliest: 2026-08-17, an unrelated `pdftract` SIGABRT); the cores under `/var/lib/systemd/coredump/` are later, other-process SIGABRTs. A memcg SIGKILL produces no core |
| Memory/disk samples at kill time | The health collector starts 2026-08-15; only `fleet.cpu_saturated` load samples exist for 2026-08-13 (59 in-window, load 7.63–18.51 — canon §3.4) |

**Error messages:** none exist in the application sense. Needle recorded no
stderr, no stack, no panic — the per-kill record is the sentinel −1 plus
`outcome.classified: crash`. Zero `max_turn` mentions in the day's 395
completions (canon §1), ruling out the workflow-exhaustion class.

---

## 7. Classification and current state

- **Kills were real; the false positives live in the alert layer** — of two kinds: multiplication (55 alert beads for one cause) and stale targets (45 % of kills were post-completion; every alert after 2026-08-16 fired against a closed bead).
- Root cause: dispatches running git-remote-heavy work against the then-≈18 GB object store inside the 12 GiB dispatch scope (`memory.max = 12884901888`), killed by memcg OOM; amplified by unbounded retry and verify-then-close debt. Full analysis, ruled-out alternates, and the fix inventory: canon §§1–5.
- **Target resolved:** closed 2026-08-16 with 8/8 criteria; deliverables on `origin/main`.
- **Prevention:** kill-mechanism layers R1–R4 deployed and the five-check battery green 2026-09-09 (canon §5.4); alert-layer fixes committed with suites, with the "knobs, not yet a pipeline" caveat (§5.3).
- `bf-4xlwo` itself remains Open and unowned — stale by the standard disposition (target closed, work shipped, canon §7.1): it owes a disposition, not an investigation.

---

## 8. Acceptance-criteria mapping

| Criterion | Answer | Verified by |
|---|---|---|
| Full bead description and status of bf-4k2ws | §1 — Closed rev 2, read-only divergence analysis, 8/8 met | `bead show bf-4k2ws` live |
| Capture crash timestamp 2026-08-13T06:34:35.805234687+00:00 | §0/§3 — captured **and corrected**: heartbeat stamp (matches log record to 12.47 µs); death = 06:34:29.204954348Z | Live worker log, seq 8376 |
| Exit code −1 and what "signal −1" means | §4 — unrecorded-signal sentinel; no such signal number; mechanism memcg-OOM SIGKILL (chain-inferred) | Canon §3.6 + live `outcome.classified` |
| Workspace the agent worked in (`.`) | §5 — `.` = `/home/coding/domain-check`; no cwd field in the log (interpretation labeled) | Alert body + worker identity |
| Crash logs / core dumps / error messages | §6 — worker log + extract + bundle survive; traces, kernel, journald, cores, samples do not; no app-level error text exists | `ls`/`coredumpctl`/`journalctl --list-boots` live |
| Agent type (claude-code-glm-4.7-lab-domain-check) | §5 — worker `claude-code-glm-4.7-lab-domain-check`, agent `claude-code-glm-4.7`, session `8446529e` | Live `agent.dispatched` record |

---

## 9. Sources

- Primary: `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` (untouched; events quoted at seqs 8364, 8373–8382)
- `docs/crash-artifacts-bf-4k2ws/2026-09-07T0823Z-needle-log-extract-bf-4k2ws-aug13.txt`, `…0824Z-traces-check.txt`, `README.md`
- Canon: `docs/investigations/bf-4k2ws-root-cause-determination-domchk-7f838f36-2026-09-07.md`; `docs/crash-investigations/bf-4k2ws/README.md` (§ references)
- Live reads this session: `bead show bf-4k2ws`, `bead show bf-4xlwo`, `.beads/checkpoint/objects/*` snapshots, `coredumpctl list`, `journalctl --list-boots`
