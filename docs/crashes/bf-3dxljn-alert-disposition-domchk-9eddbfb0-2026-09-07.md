# Alert disposition: bf-3dxljn ("ALERT: Agent crash on bead bf-mje3pd")

**Disposition bead:** domchk-9eddbfb0 (2026-09-07) — final step of splitting alert bead
bf-3dxljn
**Alert bead:** bf-3dxljn — created 2026-08-13T19:46:57Z, exit code −1, still **Open**
**Target bead:** bf-mje3pd — "Implement fix and verify agent crash prevention"

---

## Disposition: **STALE — close as stale, no investigation owed**

Not a false positive, and not an open crash. The crash was genuine and
mid-task; the target bead's work was delivered in full; the alert simply
outlived its target.

| Question | Answer | Evidence |
|---|---|---|
| Is the target bead closed? | **Yes** — Closed rev 2, 2026-08-17T00:15:35Z | re-read live from the bead store this dispatch |
| Was the crash real? | **Yes** — 7 × exit −1 across 2h24m on 2026-08-13, mid-task at every named instant | `docs/crashes/bf-mje3pd-crash-classification-domchk-bf8c4fd3-2026-09-07.md` |
| Classification | **INFRASTRUCTURE**, repository-bloat regime sub-type; mechanism regime-matched, not kernel-proven | `docs/crashes/bf-mje3pd-crash-classification-domchk-a47705c9-2026-09-07.md` (§ Verdict) |
| Was any work lost? | **No** — deliverable landed on the 14th dispatch with `verification.passed` | same two classification docs |
| Is the alert false? | **No** — it was accurate when minted; it is **stale** because the target closed 2026-08-17, four days later | `docs/crashes/bf-mje3pd-alert-suppression-domchk-8bec8a51-2026-09-07.md` (§ 5) |
| Would today's tooling suppress it? | **Yes, at every layer** — `alert-deduplication.sh check bf-3dxljn` → `DUPLICATE` exit 0, "crash target bf-mje3pd is already resolved"; suites 12/12 and 7/7 at HEAD | suppression doc §§ 2–4, 7 |
| Is bf-3dxljn's own timestamp a distinct crash? | **No** — 19:46:57Z is a release heartbeat 24 s after attempt 11's kill (19:46:33Z) and 3 s before `outcome.handled action=alerted` | evidence compilation § 4 |
| Is a domain-check code defect involved? | **No** — exit −1 routes to Infrastructure before the CODE_DEFECT branch; no investigation of this crash ever found a domain-check defect | classification doc, guide-mapping table |

## Why the alert still exists

bf-3dxljn was created 2026-08-13, weeks before the 2026-09-02 alert fixes, and
**no component of the stack retires an already-created alert bead once its
target closes** — detection landed, alert-lifecycle closure has not
(domchk-f7865662 lists that closure layer as an open gap). The fixes are
verified working; this alert simply predates them.

## Alert family (target bf-mje3pd, read live 2026-09-07)

Seven alert beads, one per kill under the pre-0.4.2 needle behaviour:

| Bead | Created (UTC) | Status |
|---|---|---|
| bf-1y1d0g | 2026-08-13T19:03:21 | closed |
| bf-1pidqn | 2026-08-13T19:15:29 | **open** — same disposition |
| bf-56kmlk | 2026-08-13T19:18:56 | **open** — same disposition |
| bf-3za7vh | 2026-08-13T19:22:02 | closed |
| bf-1cezsk | 2026-08-13T19:32:58 | closed |
| bf-x88dnf | 2026-08-13T19:37:19 | **open** — same disposition |
| bf-3dxljn | 2026-08-13T19:46:57 | **open** — this document |

The three still-open beads besides bf-3dxljn are the same shape and need no
investigation either; they wait on the same alert-lifecycle-closure layer.

## Split record (children of bf-3dxljn)

| Child | Step | Deliverable |
|---|---|---|
| domchk-916a1e66 | 1 — evidence compilation | `docs/investigations/bf-mje3pd-evidence-compilation-domchk-916a1e66-2026-09-07.md` (bfef974) |
| domchk-a47705c9 | 2 — classification | `docs/crashes/bf-mje3pd-crash-classification-domchk-a47705c9-2026-09-07.md` (e7f5b97) |
| domchk-8bec8a51 | 3 — suppression verification | `docs/crashes/bf-mje3pd-alert-suppression-domchk-8bec8a51-2026-09-07.md` (ab9042f, 14e301d) |
| domchk-9eddbfb0 | 4 — this disposition | this file |

Investigation record outside the split: domchk-bf8c4fd3 (63ca904) is the
authoritative classification of bf-mje3pd itself; `docs/verification/bf-mje3pd-crash-analysis.md`
(81614ac) is the original verification report.

## Action

Close **bf-3dxljn** as stale. Suggested close reason, following the settled
wording of the bf-6d3d6 chain: *"crashes genuine / work never lost / alert
stale — target bf-mje3pd closed 2026-08-17 (rev 2) with verification.passed;
classification INFRASTRUCTURE (repository-bloat regime), not a code defect;
suppression verified at every layer for fresh alerts."*

This document records the disposition only; closing the parent bead is the
alert-lifecycle step and is deliberately left to its owner.

---

*Written 2026-09-07 by domchk-9eddbfb0 at HEAD `14e301d`. Bead statuses re-read
live from the bead-rs store at write time; every cited path verified present and
byte-identical at HEAD.*
