# Crash Analysis Catalog

**Purpose:** Per-incident crash evidence and analysis artifacts for the domain-check workspace.
**Scope:** This directory holds preserved artifacts and incident analyses. Procedures and
status live one level up — start at [`../crash-documentation-index.md`](../crash-documentation-index.md).

**Last Updated:** 2026-09-06 (added bf-1s6c3 crash analysis and this catalog)

---

## Standing Finding

Domain-check code has **no defects**. Every investigated crash in this catalog is
infrastructure-class — the dominant mechanism being **repository bloat → memory-cgroup OOM
SIGKILL during git operations** (2026-08-12 → 2026-08-16), repaired as of 2026-09-01 and
re-verified repeatedly since.

---

## Catalog

### Incident analyses

| Document | Subject | Summary |
|----------|---------|---------|
| [`bf-1s6c3-crash-analysis-2026-08-12.md`](bf-1s6c3-crash-analysis-2026-08-12.md) | **bf-1s6c3** (Forgejo/GitHub history merge) | Full crash analysis: 76-dispatch kill storm on the bloated 18 GB repository (71 × exit −1, 4 × timeout, 1 × success); deliverable `42a7b07` landed mid-storm; classification **infrastructure — repository bloat → memcg OOM**, not a code defect. Reconciles the dispatch's cited `2026-08-12T22:24:04` timestamp (a `HANDLING_RELEASE_DONE` heartbeat 7.2 s after the real kill) and corrects the dead-SHA citations (`7dd79eb`, `2832106`). |
| [`repository-bloat-root-cause-analysis-2026-08-12.md`](repository-bloat-root-cause-analysis-2026-08-12.md) | **2026-08-12 bloat crisis** (period-level) | Root cause analysis for the whole episode: 17+ identical ~237 MB `.beads/*.jsonl` snapshots committed → 18 GB repo / 17 GB loose objects → systematic OOM SIGKILL of git operations. Cleanup (18 GB → 138 MB) and the preventive layers that followed. |
| [`bf-4yjq-resolution-record-2026-09-01.md`](bf-4yjq-resolution-record-2026-09-01.md) | **bf-4yjq** (git remote/mirror divergence; 50-kill storm 2026-08-12) | Resolution record: mitigation implemented and verified. |

### Preserved evidence

| Document | Subject | Contents |
|----------|---------|----------|
| [`bf-1s6c3-agent-logs-preserved.md`](bf-1s6c3-agent-logs-preserved.md) | bf-1s6c3 | Preserved needle worker-log extract: first crash (2026-08-12T21:36:44Z) and retry events, agent metadata (worker `claude-code-glm-4.7-lab-domain-check`, session `8446529e`), raw JSONL events. |
| [`bf-4yjq-artifact-preservation-2026-09-06.md`](bf-4yjq-artifact-preservation-2026-09-06.md) | bf-4yjq | Artifact-preservation pass — derives no new analysis; points at the canonical investigation and evidence inventory in `docs/crash-investigations/`. |
| [`bf-4yjq-needle-worker-log-extract.log`](bf-4yjq-needle-worker-log-extract.log) | bf-4yjq | Raw worker-log extract (tracing format), starting 2026-08-12T17:50:23Z. |
| [`bf-4yjq-system-state-snapshot-2026-09-01.txt`](bf-4yjq-system-state-snapshot-2026-09-01.txt) | bf-4yjq | Raw system-state snapshot captured at investigation time. |

---

## Notes on Using This Catalog

- **Figures can be stale.** The older documents in this directory predate the 2026-09-06
  primary-source recount. Where they disagree with a per-incident classification in
  `docs/crashes/<bead>-crash-classification-*.md`, the classification (and the
  `<bead>-crash-storm-timeline-*.md` it cites) is authoritative. For bf-1s6c3 specifically:
  "one crash" → 76 dispatches; merge SHA `7dd79eb`/`2832106` → `42a7b07` (branch
  `pre-squash-history-20260816`); main's reconciliation → `46293c5`.
- **Alert timestamps are not kill timestamps.** Crash-alert records often cite a
  `HANDLING_RELEASE_DONE` heartbeat seconds after the actual `agent.completed` exit −1.
- **Dead SHAs are common.** The 2026-08-16 history squash renamed commits; verify every SHA
  with `git cat-file -e` before citing it, and check which ref contains it.
- **Adjacent directories:** `docs/crashes/` (per-incident classifications, timelines,
  remediation), `docs/crash-investigations/` (investigation records),
  [`../crash-response-guide.md`](../crash-response-guide.md) (classification procedure).
