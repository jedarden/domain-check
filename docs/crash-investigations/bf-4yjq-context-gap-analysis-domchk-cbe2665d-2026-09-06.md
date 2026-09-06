# Gap analysis: bf-4yjq crash context against the critical-information checklist (2026-09-06)

**Dispatch:** domchk-cbe2665d
**Subject bead:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo mirror has diverged/gone stale" (P2, closed 2026-08-17T00:14:14Z)
**Chain position:** domchk-221cb3aa (locate report) → domchk-4950dc16 (extract details) → **this bead (identify gaps)**

**Subject of the analysis.** The crash-context report the chain located and extracted:
[`docs/crash-context-report-bf-4yjq-comprehensive.md`](../crash-context-report-bf-4yjq-comprehensive.md)
(commit `b694065`, blob `2862c16a`, 346 lines / 13,574 bytes — re-verified byte-identical and
working-tree-clean today; provenance §7). Its §7 (absences) and §8 (corrections/staleness) are the
handoff this page consumes; nothing here re-litigates those.

**What this page adds.** The extraction was report-bounded; this page is *checklist-bounded*: it
scores the report against the critical crash-information checklist, then triages every gap into
closed-elsewhere / partially-filled / permanently-unrecoverable, and names the one gap class whose
loss is why this crash's root cause is capped at MEDIUM-HIGH confidence rather than proven.

**Companions consumed:** record-level extraction
[`bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md`](bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md),
artifact catalog
[`bf-4yjq-artifact-catalog-2026-09-06.md`](bf-4yjq-artifact-catalog-2026-09-06.md),
canonical investigation
[`bf-4yjq-crash-investigation.md`](bf-4yjq-crash-investigation.md), and the contemporaneous metrics
compilation `.beads/crash-bf-4yjq-summary.txt` (14,498 B, 2026-09-01 — see §4, gap G-3).

---

## 1. Checklist verdict

| # | Checklist item | Required | Report's coverage | Verdict |
|---|----------------|----------|-------------------|---------|
| 1 | Exact exit code and signal | ✓ | "-1 (SIGKILL)" in three shapes (exec summary, alert JSON, memory section) | **PARTIAL** — exit code `-1` is documented and correct on every crash; the "SIGKILL / Signal 9" identification is the report's gloss on needle's *unrecorded-signal sentinel* (extraction §8.1). Exit code: present. Signal: not knowable from the report, and not knowable from any surviving source (§5, G-2) |
| 2 | Timestamp and workspace state | ✓ | 9-row timeline (3 rows approximate), repo metrics, remote/branch state, bead status | **PARTIAL** — present but materially wrong in scale: 9 crashes at ~17-min intervals vs the verified **50 at ~3.1-min** (canonical §3); window end 20:24 matches no table row; the dispatch-cited 20:12:37 crash is absent. *Workspace* state is well covered for the repository, entirely absent for the host (row 5) |
| 3 | What work was being attempted | ✓ | Bead mission, the six-step remediation plan, "crashes incidental to the task" | **PRESENT** — the report's strongest section; consistent with the canonical report and the record-level survival data |
| 4 | Agent version | ✓ | `claude-code-glm-4.7`, assignee `claude-code-glm-4.7-lab-domain-check`, investigator `…-2` | **PRESENT** at report level. Session-slot and claim-mechanism context (`8446529e`, `claim_auto`) come from the record-level extraction, not the report |
| 5 | System resource state at crash time | — | **None.** The report's "Memory and Process Conditions" section (line 110) carries only kill *characteristics* — no numeric figure anywhere | **MISSING from the report.** Host-level figures exist only in the Sep-1 contemporaneous compilation (§4, G-3); per-crash figures exist nowhere |
| 6 | Error logs or stack traces | — | Correctly states none exist; quotes only the alert body | **ABSENT, and the absence is now evidenced** (record-level §3): no stack traces, no agent error text, no stderr coverage of the storm, with the would-have-held inventory checked |
| 7 | Recent commits or workspace changes | — | No such section — a full-text search finds nothing on commit history or pre-crash diffs | **MISSING from the report, and unrecoverable**: crash-era commits are gone from the DAG (log jumps `00117cb` Aug-09 → `8373e5d` Aug-15; the Aug-16 `c27899f` squashed catch-up removed them — canonical §3) |
| 8 | Other RCA context | — | Alert labels/priorities, related-bead crash pattern, dependency chain, artifact list, resolution record | **MIXED** — correlation context is rich; *mechanism* context is absent: per-crash in-flight command, process identity (worker vs git child), dispatch-scope name/limit, host load at death |

**Bottom line:** the report satisfies the four *required* checklist items in substance (one of them,
row 1, with a precision caveat), and fails the three *optional-but-decisive* ones (rows 5–7) — which
are exactly the items that separate "correlated with bloat" (proven) from "killed by the OOM
mechanism named in the report" (inferred).

## 2. Gaps closed by companion work — not open, but do not cite the report for them

| Gap in the report | Where it is actually answered |
|-------------------|-------------------------------|
| Crash count and cadence (9 / ~17 min) | Canonical §3 + record-level extraction: **50 deaths**, 17:53:53.875 → 20:30:38.310 UTC, mean interval 188 s |
| Exact per-crash timestamps (3 of 9 approximate; 20:24 window end unrepresented; 20:12:37 missing) | Record-level §2/§5: all 50 death timestamps to the microsecond, plus the death↔alert skew quantified (5.1–9.0 s, median 6.0 s) — alert-bead timestamps are not death times |
| Signal identity | Record-level §1: `-1` is the sentinel for an unrecorded signal; no signal name survives anywhere in the telemetry |
| Per-run lifetimes | Record-level §4: survival 65–375 s (median 149 s) — real work before each death, far under the 600 s timeout |
| What happened after the last crash | Catalog §2 + record-level §2: 4 timeouts at exactly 600.1 s, one orphaned exit-0, three auto-splits — the disruption window ends 21:14:59, not 20:24 |
| Workspace identity | Catalog §1: `/home/coding/domain-check` attested on all 225 records (the report's `Workspace: .` is the literal CLI argument, not a different workspace) |

These belong on this list because the report is the document a future reader hits first, and its
self-assessment (§6 below) claims none of this is missing.

## 3. Gaps partially filled — the surviving record covers them at one level only

- **Host resource state.** `.beads/crash-bf-4yjq-summary.txt` (Sep-1 compilation) carries the only
  numeric figures in the corpus: 62 GB RAM, swap 0, load average 15–17 on 12 cores, disk 84% full
  (350/444 GB), inodes 80%. Canonical §6 carries the load/disk figures in git. What no source
  carries: *per-crash* figures — the dying process's RSS, its cgroup/scope, available RAM at the
  death instant, or the kernel's oom-select decision. The compilation concedes this itself
  ("No memory profiling at crash time", "No memory usage snapshots during actual crashes").
- **Mechanism class.** The kernel-memcg-OOM-in-a-`MemoryMax=12 GiB`-scope mechanism is *verified
  fleet-wide* for needle dispatch deaths (domchk-e843c4f1, commit `c700252`) — from later,
  kernel-recoverable events (bf-4x12ec Aug-14, bf-198ne Aug-16). For the Aug-12 storm it remains an
  attribution, not a record. The one genuinely unattested link: whether the Aug-12 dispatch scope
  carried the same 12 GiB `MemoryMax` — the figure is documented for later events, never for this one.

## 4. Permanently unrecoverable gaps (with the verified reason)

| # | Gap | Why it cannot be recovered |
|---|-----|----------------------------|
| G-1 | **The kernel OOM record** — which process the kernel selected, its badness score, and whether the kill was host-wide or inside the dispatch's memcg scope | System journal begins 2026-08-15 19:46:33 EDT; user journal 2026-08-17 (re-verified live today, §7). Aug-12 kernel output does not exist on this box |
| G-2 | **Signal identity** for each death | `-1` is needle's sentinel for a death whose signal was not recorded; no signal name appears anywhere in the 225-record telemetry |
| G-3 | **Per-crash memory figures** (RSS, scope limit, free RAM at death) | Never captured; `.beads/crash-bf-4yjq-summary.txt` states so explicitly. No monitoring stack existed on Aug-12 — the resource-monitor timers date from 2026-09-02 |
| G-4 | **The in-flight command at each death** (which of fetch/merge/repoint/mirror-config was running) | No session transcripts (per-attempt JSONL deleted), no traces (bf-4yjq's 56 dispatches predate trace capture), and the three stderr slots bracket the storm without covering it (record-level §3) |
| G-5 | **Process identity at death** — the needle worker itself vs a git child process | Same evidence set as G-4. This is the distinction the later kernel-recoverable crashes resolved; for this storm it stays open |
| G-6 | **Crash-era commits and workspace diffs** | Removed from the DAG by the Aug-16 squash commit `c27899f`; unreachable by any git operation |
| G-7 | **Core dumps** | `coredumpctl` earliest entry 2026-08-17 16:01:44 EDT (pdftract, SIGABRT) — nothing from Aug-12, consistent with SIGKILL-class deaths |
| G-8 | **Aug-12 heartbeats** | `.beads/heartbeats.jsonl` earliest entry 2026-08-15T12:06Z |

G-1 is the load-bearing one: it is the specific reason the canonical report caps mechanism
confidence at MEDIUM-HIGH, and everything else in this table is downstream of the same
evidence-retention regime that produced it.

**The one question G-1 would have settled, still open for this event:** the contemporaneous
compilation frames the death as host-level exhaustion ("62 GB total … memory exhausted … OOM killer
delivered SIGKILL"), while the class-verified mechanism is *scope-level* memcg OOM inside a 12 GiB
dispatch. Those are different mechanisms with different mitigations (host memory headroom vs
dispatch-scope bounds), and the kernel line that would have distinguished them is gone. This is why
neither formulation may be restated as directly proven for the Aug-12 storm — the bloat
*correlation* is HIGH-confidence and proven by the crash class ending when the repo was cleaned;
the OOM *mechanism* is not.

## 5. Gaps still actionable today (one live item)

- **`.log.2` rotation risk.** The storm's only primary source is rotation slot 2 of a ~128 MiB
  size-based rotation; re-verified live today it still exists, byte-count unchanged at 134,207,883 B,
  with the live slot at 90,732,929 B — one more rotation erases it. The *extracted figures* are
  already preserved in git (record-level §5 and catalog §2.1 carry all 50 timestamps and the full
  event tally); what would be lost is the raw 225-record slice itself, i.e. the ability to re-derive
  anything those extractions did not anticipate. Re-derivable until then with
  `grep 'bf-4yjq' ~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2`.
  This is the last gap on this page that action can still close.
- **Every other gap** is a *future*-facing one, and it is already closed: the monitoring stack
  deployed 2026-09-02 (resource-monitor snapshots, journald retention, trace capture, crash-alert
  dedup, `pack.windowMemory` bounds) records exactly the categories G-1–G-8 lost. The canonical
  report's recommendation 3 ("retain storm telemetry") is implemented, not pending.

## 6. The report's self-assessment, corrected

`docs/crash-context-report-bf-4yjq-comprehensive.md` line 336 states: **"Gaps: NONE IDENTIFIED."**
Its own contents contradict that — three approximate timestamps, a crash count verified wrong by 5×,
no numeric resource figures, no commit-level workspace history, and a signal identification its own
telemetry cannot support. The extraction's §8.5 flagged the claim; this page is the corrected list
that replaces it.

## 7. Provenance

Checked live 2026-09-06 by dispatch domchk-cbe2665d (nothing inherited unverified):

- Source report: `git status` clean; tracked blob `2862c16a1cfefbab5602f16bfcca8da6a23f1b4a` ==
  worktree `git hash-object`; 346 lines / 13,574 bytes; "Gaps: NONE IDENTIFIED" at line 336; full-text
  greps confirm zero numeric memory figures and no commit-history content.
- `~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2`: present, 134,207,883 B;
  live slot 90,732,929 B; `.log.1` 134,217,317 B.
- `journalctl --list-boots`: earliest boot 2026-08-15 19:46:33 EDT.
- `coredumpctl list`: earliest entry 2026-08-17 16:01:44 EDT (pdftract, SIGABRT, COREFILE missing).
- `.beads/crash-bf-4yjq-summary.txt`: 14,498 B; resource figures quoted in §3/G-3 read from it
  directly (62 GB / swap 0 / load 15–17 / disk 84% / inodes 80%, and its own "no memory profiling"
  admissions).
- All other figures are cited from the companion documents named in the header, each of which
  carries its own live-verification provenance.
