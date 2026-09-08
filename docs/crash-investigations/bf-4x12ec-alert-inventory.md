# bf-4x12ec — Crash Alert Bead Inventory

**Bead:** domchk-f05d6f91 · Child 1 of the domchk-bcdd4b4f split (umbrella: *Check for duplicate crash alerts on bf-4x12ec*)
**Generated:** 2026-09-07, live from `bead list --json --limit 100000` (3,443 beads scanned)

## Scope and method

Two queries define the inventory, both run against the live bead store:

1. **Exact title** `ALERT: Agent crash on bead bf-4x12ec` — **44 beads**, all created 2026-08-14.
2. **Substring** `bf-4x12ec` in the title *or* body of every `domchk-*` bead — **125 beads**.

A substring match overstates the set: most body hits are incidental (a bead-template line such as
`Start with: bead show bf-4x12ec`, or a comparison list alongside bf-1s6c3/bf-173o7e). The inventory
therefore separates beads that are *substantively about* bf-4x12ec from those that merely name it.

**Core inventory** = 44 wave-1 alerts + 55 title-referencing `domchk-*` + 3 genuine body-only
`domchk-*` = **102 beads**.
**Excluded as incidental** = 67 `domchk-*` body-only mentions + 2 `bf-*` alerts for *other* targets
that cite bf-4x12ec in passing (`bf-5wxej` → bf-4k2ws, `bf-4ifshb` → bf-173o7e; both closed).

## Headline result

The prior verification's two figures are **confirmed exactly**:

| Claim | Prior | This inventory |
|-------|-------|----------------|
| Wave-1 storm alerts | 44 | **44** (`bf-fmg2cw` 10:23:11Z → `bf-5x69lm` 11:28:02Z, span 3891 s, mean gap **90.5 s**, min 60, max 155) |
| Wave-2 regeneration beads | ~16 | **16** (all 2026-08-26, 20:17:41Z → 21:13:57Z) |

**Correction to the wave-2 date:** the regeneration wave is entirely **2026-08-26** — zero beads on
2026-08-25. The "08-25/26" framing in the split description is not borne out by creation timestamps.
A single earlier bead, `domchk-c95117c0` (2026-08-17), sits between the two waves.

**Headline concern — 21 of 44 wave-1 alerts remain unresolved**, 24 days after the storm and 20 days
after bf-4x12ec itself closed (2026-08-17) with its work verified complete. These are stale alert
beads, not open work. Sibling bead `domchk-e9ed52aa` owns closing them as false positives.

## Status tally

### Status tally

**Wave 1 — 2026-08-14 storm ALERT beads** — 44 beads: closed 23, open 15, in_progress 5, deferred 1 → **21 NOT closed**
**Pre-wave — 2026-08-17** — 1 beads: closed 0, open 1, in_progress 0, deferred 0 → **1 NOT closed**
**Wave 2 — 2026-08-26 regeneration** — 16 beads: closed 6, open 10, in_progress 0, deferred 0 → **10 NOT closed**
**Wave 3 — 2026-09-02 split/investigation** — 38 beads: closed 10, open 22, in_progress 6, deferred 0 → **28 NOT closed**
**Scope C — genuine body-only** — 3 beads: closed 3, open 0, in_progress 0, deferred 0 → **0 NOT closed**

**CORE INVENTORY (waves 1–3 + pre-wave + scope C)** — 102 beads: closed 42, open 48, in_progress 11, deferred 1 → **60 NOT closed**
**Excluded incidental body-only** — 67 beads: closed 46, open 18, in_progress 3, deferred 0 → **21 NOT closed**

Grand total beads touching bf-4x12ec in title or body: 169 (core 102 + incidental 67)

### Beads not closed (flag list)

#### Wave 1 ALERT beads not closed — 21

All are alert beads for a target (`bf-4x12ec`) that closed 2026-08-17 with its work verified complete — these are stale alerts, not open work. Listed in creation order:

- `bf-fmg2cw` — **open** (created 2026-08-14T10:23:11, rev 16, last updated 2026-08-26T20:18:30)
- `bf-191ch8` — **open** (created 2026-08-14T10:28:37, rev 16, last updated 2026-08-26T20:21:36)
- `bf-12ad85` — **open** (created 2026-08-14T10:29:46, rev 16, last updated 2026-08-26T20:26:47)
- `bf-438934` — **open** (created 2026-08-14T10:33:19, rev 16, last updated 2026-08-26T20:30:16)
- `bf-3yv2jn` — **open** (created 2026-08-14T10:39:42, rev 16, last updated 2026-08-26T20:41:33)
- `bf-22w69c` — **open** (created 2026-08-14T10:45:01, rev 16, last updated 2026-08-26T20:47:24)
- `bf-qz9mov` — **open** (created 2026-08-14T10:49:44, rev 17, last updated 2026-08-26T20:50:52)
- `bf-48vwac` — **open** (created 2026-08-14T10:52:14, rev 21, last updated 2026-08-26T20:55:26)
- `bf-c1sthq` — **open** (created 2026-08-14T10:57:10, rev 17, last updated 2026-08-26T20:59:59)
- `bf-2m532x` — **open** (created 2026-08-14T10:58:19, rev 22, last updated 2026-08-26T21:06:10)
- `bf-4oblul` — **open** (created 2026-08-14T11:01:40, rev 18, last updated 2026-08-26T21:04:54)
- `bf-44upi7` — **open** (created 2026-08-14T11:08:40, rev 17, last updated 2026-08-26T21:11:48)
- `bf-22h8jj` — **open** (created 2026-08-14T11:09:54, rev 26, last updated 2026-08-26T21:15:27)
- `bf-drsdsn` — **in_progress** (created 2026-08-14T11:11:10, rev 13, last updated 2026-08-17T15:40:03)
- `bf-68u9bl` — **in_progress** (created 2026-08-14T11:12:19, rev 2, last updated 2026-08-17T15:39:03)
- `bf-2aa8vo` — **in_progress** (created 2026-08-14T11:13:29, rev 8, last updated 2026-08-17T15:59:10)
- `bf-4833lh` — **open** (created 2026-08-14T11:14:39, rev 17, last updated 2026-08-17T16:11:34)
- `bf-2ozrew` — **in_progress** (created 2026-08-14T11:15:47, rev 3, last updated 2026-08-17T15:44:22)
- `bf-2yruum` — **in_progress** (created 2026-08-14T11:24:40, rev 10, last updated 2026-08-17T16:11:14)
- `bf-25uq3d` — **open** (created 2026-08-14T11:25:52, rev 15, last updated 2026-08-26T21:19:41)
- `bf-5x69lm` — **deferred** (created 2026-08-14T11:28:02, rev 5, last updated 2026-08-17T16:06:19)

#### domchk-* beads not closed — 39 of 58

- `domchk-2ff261ce` — **open** (created 2026-08-26T20:17:41) — Gather crash artifacts for bead bf-4x12ec
- `domchk-9aa5f0a8` — **open** (created 2026-08-26T20:17:43) — Analyze root cause of agent crash on bf-4x12ec
- `domchk-46a00141` — **open** (created 2026-08-26T20:21:16) — Investigate agent crash logs for bf-4x12ec
- `domchk-c59d96ac` — **open** (created 2026-08-26T20:21:19) — Re-run failed bead bf-4x12ec
- `domchk-4adc1a55` — **open** (created 2026-08-26T20:44:44) — Analyze root cause of signal -1 crash on bf-4x12ec
- `domchk-0e428c71` — **open** (created 2026-08-26T20:44:50) — Document crash findings and preventive recommendations for bf-4x12ec
- `domchk-c99cdf80` — **open** (created 2026-08-26T20:57:49) — Investigate crashed bead bf-4x12ec context and goal
- `domchk-fcbaefea` — **open** (created 2026-08-26T21:11:08) — Verify crash fix and re-run bf-4x12ec
- `domchk-bcdd4b4f` — **open** (created 2026-08-26T21:13:57) — Check for duplicate crash alerts on bf-4x12ec
- `domchk-fe10456e` — **open** (created 2026-09-02T14:09:17) — Write bf-4x12ec crash investigation summary
- `domchk-a745eefe` — **in_progress** (created 2026-09-02T14:14:53) — Verify memory and resource constraints for the bf-4x12ec kill
- `domchk-ce556de0` — **open** (created 2026-09-02T14:14:53) — Compile bf-4x12ec diagnostic findings into parent bead
- `domchk-e48b5e1b` — **in_progress** (created 2026-09-02T14:16:58) — Verify no-code-defect finding documented in bf-4x12ec report
- `domchk-8c78ae8b` — **open** (created 2026-09-02T14:16:58) — Fill gaps and finalize bf-4x12ec investigation report
- `domchk-c1c0afd8` — **in_progress** (created 2026-09-02T14:28:58) — Extract bf-4x12ec bead record and crash facts
- `domchk-dfce2360` — **in_progress** (created 2026-09-02T14:29:06) — Document the operation in progress when bf-4x12ec was killed
- `domchk-ad80e265` — **open** (created 2026-09-02T14:29:11) — Extract kernel and systemd messages for the bf-4x12ec signal -1 event
- `domchk-d7241598` — **open** (created 2026-09-02T14:29:14) — Document workspace state during the bf-4x12ec crash window
- `domchk-6df39087` — **open** (created 2026-09-02T14:29:21) — Identify files and systems involved in bf-4x12ec
- `domchk-e3ecf2a6` — **open** (created 2026-09-02T14:29:27) — Write consolidated bf-4x12ec findings to docs/notes deliverable
- `domchk-ba8584a1` — **open** (created 2026-09-02T14:52:20) — Document bf-4x12ec crash timeline and bead context
- `domchk-eae1d2ed` — **open** (created 2026-09-02T14:52:21) — Check system-level evidence for bf-4x12ec crash
- `domchk-10640b7f` — **open** (created 2026-09-02T14:52:22) — Write bf-4x12ec crash artifacts summary file
- `domchk-15854355` — **in_progress** (created 2026-09-02T15:08:12) — Collect bf-4x12ec crash evidence and document signal -1 semantics
- `domchk-5f3ec6e1` — **open** (created 2026-09-02T15:08:13) — Check system resources around the bf-4x12ec crash window
- `domchk-78d89c6b` — **open** (created 2026-09-02T15:08:14) — Assess bf-4x12ec workload contribution and crash reproducibility
- `domchk-dba1e0bb` — **open** (created 2026-09-02T15:08:14) — Classify the bf-4x12ec crash and recommend retry safety
- `domchk-f05d6f91` — **in_progress** (created 2026-09-02T15:34:41) — Inventory all bf-4x12ec crash alert beads and their statuses
- `domchk-b2b77f12` — **open** (created 2026-09-02T15:34:42) — Verify bf-4x12ec alert beads against false-positive evidence
- `domchk-1e38a7c2` — **open** (created 2026-09-02T15:34:44) — Document the two-wave bf-4x12ec duplicate-alert pattern
- `domchk-a3690174` — **open** (created 2026-09-02T15:34:46) — Confirm crash-alert mitigations prevent bf-4x12ec regeneration
- `domchk-e9ed52aa` — **open** (created 2026-09-02T15:34:48) — Close residual open bf-4x12ec wave-1 alert beads as false positives
- `domchk-779d1180` — **open** (created 2026-09-02T18:35:01) — Write bf-4x12ec incident timeline report
- `domchk-08bdde8d` — **open** (created 2026-09-02T18:35:02) — Add root cause analysis to bf-4x12ec incident report
- `domchk-0936d2db` — **open** (created 2026-09-02T18:35:03) — Add repository state verification to bf-4x12ec incident report
- `domchk-1ef6b252` — **open** (created 2026-09-02T18:35:04) — Document bf-4x12ec task outcome and resolution
- `domchk-6f771e64` — **open** (created 2026-09-02T18:35:04) — Update CLAUDE.md procedures from bf-4x12ec findings and finalize report
- `domchk-c95117c0` — **open** (created 2026-08-17T15:59:21) — Gather crash information for bead bf-4x12ec
- `domchk-c0077666` — **open** (created 2026-08-26T21:05:38) — Verify bf-4x12ec crash resolution status

---

## Ordered inventory

### Wave 1 — 2026-08-14 storm (44 exact-title ALERT beads)

One `ALERT: Agent crash on bead bf-4x12ec` bead per memcg-OOM kill. Pre-0.4.2 needle emitted a distinct alert bead per kill, so these 44 map 1:1 onto the 44 exit -1 deaths.

| # | Bead | Created (UTC) | Status | Inter-alert gap |
|---|------|---------------|--------|-----------------|
| 1 | `bf-fmg2cw` | 2026-08-14T10:23:11 | **open** | — |
| 2 | `bf-3m9m1v` | 2026-08-14T10:25:30 | **closed** | 139 s |
| 3 | `bf-msui35` | 2026-08-14T10:27:04 | **closed** | 94 s |
| 4 | `bf-191ch8` | 2026-08-14T10:28:37 | **open** | 93 s |
| 5 | `bf-12ad85` | 2026-08-14T10:29:46 | **open** | 69 s |
| 6 | `bf-438k7j` | 2026-08-14T10:31:14 | **closed** | 88 s |
| 7 | `bf-30pdr8` | 2026-08-14T10:32:17 | **closed** | 63 s |
| 8 | `bf-438934` | 2026-08-14T10:33:19 | **open** | 62 s |
| 9 | `bf-2vepdz` | 2026-08-14T10:35:02 | **closed** | 103 s |
| 10 | `bf-5a3q4w` | 2026-08-14T10:36:34 | **closed** | 92 s |
| 11 | `bf-4nmj66` | 2026-08-14T10:38:09 | **closed** | 95 s |
| 12 | `bf-3yv2jn` | 2026-08-14T10:39:42 | **open** | 93 s |
| 13 | `bf-lntjyq` | 2026-08-14T10:41:13 | **closed** | 91 s |
| 14 | `bf-whzeuf` | 2026-08-14T10:43:31 | **closed** | 138 s |
| 15 | `bf-22w69c` | 2026-08-14T10:45:01 | **open** | 90 s |
| 16 | `bf-2oov1x` | 2026-08-14T10:46:38 | **closed** | 97 s |
| 17 | `bf-4qj2rz` | 2026-08-14T10:48:23 | **closed** | 105 s |
| 18 | `bf-qz9mov` | 2026-08-14T10:49:44 | **open** | 81 s |
| 19 | `bf-4h2mqq` | 2026-08-14T10:50:58 | **closed** | 74 s |
| 20 | `bf-48vwac` | 2026-08-14T10:52:14 | **open** | 76 s |
| 21 | `bf-4xbt4g` | 2026-08-14T10:53:14 | **closed** | 60 s |
| 22 | `bf-1uh46l` | 2026-08-14T10:55:49 | **closed** | 155 s |
| 23 | `bf-c1sthq` | 2026-08-14T10:57:10 | **open** | 81 s |
| 24 | `bf-2m532x` | 2026-08-14T10:58:19 | **open** | 69 s |
| 25 | `bf-67jjlg` | 2026-08-14T10:59:58 | **closed** | 99 s |
| 26 | `bf-4oblul` | 2026-08-14T11:01:40 | **open** | 102 s |
| 27 | `bf-3cy3vk` | 2026-08-14T11:03:25 | **closed** | 105 s |
| 28 | `bf-353z15` | 2026-08-14T11:05:05 | **closed** | 100 s |
| 29 | `bf-bm3x3s` | 2026-08-14T11:06:06 | **closed** | 61 s |
| 30 | `bf-2804g8` | 2026-08-14T11:07:21 | **closed** | 75 s |
| 31 | `bf-44upi7` | 2026-08-14T11:08:40 | **open** | 79 s |
| 32 | `bf-22h8jj` | 2026-08-14T11:09:54 | **open** | 74 s |
| 33 | `bf-drsdsn` | 2026-08-14T11:11:10 | **in_progress** | 76 s |
| 34 | `bf-68u9bl` | 2026-08-14T11:12:19 | **in_progress** | 69 s |
| 35 | `bf-2aa8vo` | 2026-08-14T11:13:29 | **in_progress** | 70 s |
| 36 | `bf-4833lh` | 2026-08-14T11:14:39 | **open** | 70 s |
| 37 | `bf-2ozrew` | 2026-08-14T11:15:47 | **in_progress** | 68 s |
| 38 | `bf-2u3dzu` | 2026-08-14T11:17:23 | **closed** | 96 s |
| 39 | `bf-10jhaa` | 2026-08-14T11:18:53 | **closed** | 90 s |
| 40 | `bf-5f9xqg` | 2026-08-14T11:21:10 | **closed** | 137 s |
| 41 | `bf-f49g6z` | 2026-08-14T11:22:46 | **closed** | 96 s |
| 42 | `bf-2yruum` | 2026-08-14T11:24:40 | **in_progress** | 114 s |
| 43 | `bf-25uq3d` | 2026-08-14T11:25:52 | **open** | 72 s |
| 44 | `bf-5x69lm` | 2026-08-14T11:28:02 | **deferred** | 130 s |

Span: 10:23:11Z → 11:28:02Z = 3891 s; mean gap 90.5 s; min 60 s, max 155 s.

### Pre-wave — 2026-08-17 (1 bead)

| # | Bead | Created (UTC) | Status | Note |
|---|------|---------------|--------|------|
| 1 | `domchk-c95117c0` | 2026-08-17T15:59:21 | **open** | Earliest bf-4x12ec investigation bead; target bead closed this day |

### Wave 2 — 2026-08-26 regeneration (16 beads)

Re-derived investigation beads, emitted 20:17–21:19Z. All 16 name bf-4x12ec in the title.

| # | Bead | Created (UTC) | Status | Note |
|---|------|---------------|--------|------|
| 1 | `domchk-2ff261ce` | 2026-08-26T20:17:41 | **open** | |
| 2 | `domchk-9aa5f0a8` | 2026-08-26T20:17:43 | **open** | |
| 3 | `domchk-46a00141` | 2026-08-26T20:21:16 | **open** | |
| 4 | `domchk-c59d96ac` | 2026-08-26T20:21:19 | **open** | |
| 5 | `domchk-4009b661` | 2026-08-26T20:24:15 | **closed** | |
| 6 | `domchk-661c2dc6` | 2026-08-26T20:39:11 | **closed** | |
| 7 | `domchk-ff1dfcfc` | 2026-08-26T20:44:40 | **closed** | |
| 8 | `domchk-4adc1a55` | 2026-08-26T20:44:44 | **open** | |
| 9 | `domchk-0e428c71` | 2026-08-26T20:44:50 | **open** | |
| 10 | `domchk-d986ce54` | 2026-08-26T20:54:16 | **closed** | |
| 11 | `domchk-9e2aa740` | 2026-08-26T20:54:20 | **closed** | |
| 12 | `domchk-c99cdf80` | 2026-08-26T20:57:49 | **open** | |
| 13 | `domchk-c0077666` | 2026-08-26T21:05:38 | **open** | |
| 14 | `domchk-fcbaefea` | 2026-08-26T21:11:08 | **open** | |
| 15 | `domchk-90640785` | 2026-08-26T21:13:53 | **closed** | |
| 16 | `domchk-bcdd4b4f` | 2026-08-26T21:13:57 | **open** | |

### Wave 3 — 2026-09-02 split/investigation wave (38 beads)

| # | Bead | Created (UTC) | Status | Note |
|---|------|---------------|--------|------|
| 1 | `domchk-4bad8e94` | 2026-09-02T14:09:15 | **closed** | |
| 2 | `domchk-520b8682` | 2026-09-02T14:09:16 | **closed** | |
| 3 | `domchk-0e707410` | 2026-09-02T14:09:17 | **closed** | |
| 4 | `domchk-fe10456e` | 2026-09-02T14:09:17 | **open** | |
| 5 | `domchk-4fceff67` | 2026-09-02T14:14:51 | **closed** | |
| 6 | `domchk-e68956b1` | 2026-09-02T14:14:52 | **closed** | |
| 7 | `domchk-a745eefe` | 2026-09-02T14:14:53 | **in_progress** | |
| 8 | `domchk-ce556de0` | 2026-09-02T14:14:53 | **open** | |
| 9 | `domchk-f6aba211` | 2026-09-02T14:16:56 | **closed** | |
| 10 | `domchk-791bfb2e` | 2026-09-02T14:16:57 | **closed** | |
| 11 | `domchk-e48b5e1b` | 2026-09-02T14:16:58 | **in_progress** | |
| 12 | `domchk-8c78ae8b` | 2026-09-02T14:16:58 | **open** | |
| 13 | `domchk-48f3e34d` | 2026-09-02T14:28:58 | **closed** | |
| 14 | `domchk-c1c0afd8` | 2026-09-02T14:28:58 | **in_progress** | |
| 15 | `domchk-40c9c99a` | 2026-09-02T14:29:02 | **closed** | |
| 16 | `domchk-dfce2360` | 2026-09-02T14:29:06 | **in_progress** | |
| 17 | `domchk-ad80e265` | 2026-09-02T14:29:11 | **open** | |
| 18 | `domchk-d7241598` | 2026-09-02T14:29:14 | **open** | |
| 19 | `domchk-6df39087` | 2026-09-02T14:29:21 | **open** | |
| 20 | `domchk-e3ecf2a6` | 2026-09-02T14:29:27 | **open** | |
| 21 | `domchk-a3f1f8f5` | 2026-09-02T14:52:19 | **closed** | |
| 22 | `domchk-ba8584a1` | 2026-09-02T14:52:20 | **open** | |
| 23 | `domchk-eae1d2ed` | 2026-09-02T14:52:21 | **open** | |
| 24 | `domchk-10640b7f` | 2026-09-02T14:52:22 | **open** | |
| 25 | `domchk-15854355` | 2026-09-02T15:08:12 | **in_progress** | |
| 26 | `domchk-5f3ec6e1` | 2026-09-02T15:08:13 | **open** | |
| 27 | `domchk-78d89c6b` | 2026-09-02T15:08:14 | **open** | |
| 28 | `domchk-dba1e0bb` | 2026-09-02T15:08:14 | **open** | |
| 29 | `domchk-f05d6f91` | 2026-09-02T15:34:41 | **in_progress** | ← **this bead** |
| 30 | `domchk-b2b77f12` | 2026-09-02T15:34:42 | **open** | |
| 31 | `domchk-1e38a7c2` | 2026-09-02T15:34:44 | **open** | |
| 32 | `domchk-a3690174` | 2026-09-02T15:34:46 | **open** | |
| 33 | `domchk-e9ed52aa` | 2026-09-02T15:34:48 | **open** | |
| 34 | `domchk-779d1180` | 2026-09-02T18:35:01 | **open** | |
| 35 | `domchk-08bdde8d` | 2026-09-02T18:35:02 | **open** | |
| 36 | `domchk-0936d2db` | 2026-09-02T18:35:03 | **open** | |
| 37 | `domchk-1ef6b252` | 2026-09-02T18:35:04 | **open** | |
| 38 | `domchk-6f771e64` | 2026-09-02T18:35:04 | **open** | |

### Scope C — genuine bf-4x12ec investigations found in body only (3 beads)

These do not name bf-4x12ec in the title but are substantively about it (5–7 body mentions each, bf-4x12ec as their stated subject).

| Bead | Created (UTC) | Status | Why included |
|------|---------------|--------|--------------|
| `domchk-862d95d1` | 2026-08-26T20:39:17 | **closed** | Verifies bf-4x12ec git-gc work completion against its acceptance criteria |
| `domchk-30d451d3` | 2026-08-26T21:10:47 | **closed** | "Review and document the crash for bead bf-4x12ec" |
| `domchk-0bda808c` | 2026-08-26T21:10:56 | **closed** | "Determine why the agent crashed during bf-4x12ec" — documents the memcg root cause |

### Excluded — incidental body-only mentions (67 domchk beads)

Match the body-reference query but are not bf-4x12ec investigations. Two density classes:

- **67 beads** with 1–4 body mentions — generic crash-investigation beads from the same era that reference bf-4x12ec in passing (most often inside a bead-template line like `Start with: bead show bf-4x12ec`, or in a comparison list alongside bf-1s6c3/bf-173o7e). Status: 46 closed, 18 open, 3 in_progress.
- **2 beads** with 5–7 mentions whose actual subject is **bf-173o7e**, citing bf-4x12ec comparatively: `domchk-c438170f` (closed), `domchk-31e43626` (closed). domchk-31e43626 states explicitly that the 13:55:36Z death belongs to bf-173o7e, *not* bf-4x12ec.

Excluded separately: 2 `bf-*` ALERT beads for other targets that mention bf-4x12ec in their body — `bf-5wxej` (target bf-4k2ws, closed) and `bf-4ifshb` (target bf-173o7e, closed). Neither is a bf-4x12ec alert.

<details><summary>Incidental body-only beads (full list)</summary>

| Bead | Created (UTC) | Status | Mentions |
|------|---------------|--------|----------|
| `domchk-9dab657d` | 2026-08-17T16:06:26 | **closed** | 2 |
| `domchk-88677565` | 2026-08-17T16:06:34 | **closed** | 4 |
| `domchk-0c601026` | 2026-08-26T01:55:49 | **closed** | 1 |
| `domchk-6dcfc824` | 2026-08-26T03:10:42 | **closed** | 1 |
| `domchk-5a3581c6` | 2026-08-26T07:44:11 | **closed** | 1 |
| `domchk-b9a8d893` | 2026-08-26T08:03:20 | **closed** | 1 |
| `domchk-aa7e861a` | 2026-08-26T11:50:19 | **closed** | 1 |
| `domchk-308e95bf` | 2026-08-26T12:54:57 | **closed** | 1 |
| `domchk-4eab7c59` | 2026-08-26T20:08:32 | **closed** | 1 |
| `domchk-9d85dc9f` | 2026-08-26T20:21:17 | **open** | 1 |
| `domchk-be3cf290` | 2026-08-26T20:24:24 | **open** | 2 |
| `domchk-0fcaef88` | 2026-08-26T20:24:39 | **open** | 1 |
| `domchk-a8b0e719` | 2026-08-26T20:28:47 | **open** | 2 |
| `domchk-7b316e8e` | 2026-08-26T20:28:56 | **open** | 1 |
| `domchk-8c3fceeb` | 2026-08-26T20:39:23 | **closed** | 2 |
| `domchk-6c363433` | 2026-08-26T20:49:56 | **open** | 2 |
| `domchk-bf20ed27` | 2026-08-26T20:50:07 | **open** | 2 |
| `domchk-3e723f18` | 2026-08-26T20:54:29 | **open** | 1 |
| `domchk-0c1beda9` | 2026-08-26T20:57:56 | **open** | 2 |
| `domchk-2400c0aa` | 2026-08-26T20:58:02 | **open** | 1 |
| `domchk-0f9eb93a` | 2026-08-26T21:03:35 | **closed** | 4 |
| `domchk-b037ca90` | 2026-08-26T21:03:42 | **closed** | 1 |
| `domchk-f6757c18` | 2026-08-26T21:03:51 | **open** | 3 |
| `domchk-fd8a13ae` | 2026-08-26T21:05:39 | **open** | 1 |
| `domchk-4d934d6f` | 2026-08-26T21:05:40 | **open** | 1 |
| `domchk-c475e20e` | 2026-08-26T21:11:00 | **in_progress** | 1 |
| `domchk-6a9ca138` | 2026-08-26T21:14:15 | **open** | 2 |
| `domchk-d55c4004` | 2026-08-26T21:14:23 | **open** | 2 |
| `domchk-cdce02d0` | 2026-08-26T21:18:00 | **closed** | 1 |
| `domchk-30b53d74` | 2026-08-26T21:18:06 | **open** | 1 |
| `domchk-3152117c` | 2026-08-26T21:18:20 | **open** | 1 |
| `domchk-2539cf8c` | 2026-08-26T21:28:22 | **in_progress** | 2 |
| `domchk-a49ea1fb` | 2026-08-26T21:29:12 | **in_progress** | 2 |
| `domchk-17ca8b7d` | 2026-08-26T21:37:01 | **closed** | 3 |
| `domchk-c438170f` | 2026-08-26T22:19:21 | **closed** | 5 |
| `domchk-ef456bde` | 2026-08-26T22:19:30 | **closed** | 2 |
| `domchk-fe8edf7a` | 2026-08-26T22:26:29 | **closed** | 1 |
| `domchk-c67caf80` | 2026-08-26T22:26:36 | **closed** | 1 |
| `domchk-31e43626` | 2026-08-26T22:34:46 | **closed** | 6 |
| `domchk-760530a8` | 2026-08-26T22:34:59 | **closed** | 2 |
| `domchk-5443effb` | 2026-08-26T22:38:29 | **closed** | 1 |
| `domchk-fe489733` | 2026-08-26T22:38:30 | **closed** | 3 |
| `domchk-0eea1a4b` | 2026-08-26T22:41:25 | **closed** | 1 |
| `domchk-536862b8` | 2026-08-26T22:41:34 | **closed** | 3 |
| `domchk-673bbad9` | 2026-08-26T22:50:00 | **closed** | 1 |
| `domchk-28b9a6e1` | 2026-08-26T23:25:16 | **closed** | 2 |
| `domchk-dc8633f9` | 2026-08-27T00:40:08 | **closed** | 2 |
| `domchk-f3c5f61d` | 2026-09-01T20:13:01 | **closed** | 1 |
| `domchk-a39caced` | 2026-09-01T21:56:39 | **closed** | 1 |
| `domchk-87a1f33b` | 2026-09-01T22:45:11 | **closed** | 1 |
| `domchk-45da5280` | 2026-09-01T23:08:30 | **closed** | 1 |
| `domchk-b9513e0b` | 2026-09-02T00:21:02 | **closed** | 1 |
| `domchk-aa204718` | 2026-09-02T00:21:35 | **closed** | 1 |
| `domchk-c4883c72` | 2026-09-02T01:30:47 | **closed** | 1 |
| `domchk-1fb4ad35` | 2026-09-02T01:40:59 | **closed** | 1 |
| `domchk-9fe9f087` | 2026-09-02T01:47:18 | **closed** | 1 |
| `domchk-9fe7fba1` | 2026-09-02T02:22:06 | **closed** | 1 |
| `domchk-0d43def4` | 2026-09-02T02:25:09 | **closed** | 1 |
| `domchk-6abaa850` | 2026-09-02T05:31:00 | **closed** | 2 |
| `domchk-ebf9c1f7` | 2026-09-02T05:31:13 | **closed** | 1 |
| `domchk-2222ea44` | 2026-09-02T06:41:34 | **closed** | 1 |
| `domchk-ecf47b49` | 2026-09-02T06:41:57 | **closed** | 3 |
| `domchk-26ccd69b` | 2026-09-02T06:42:21 | **closed** | 1 |
| `domchk-aa986d4b` | 2026-09-02T06:42:23 | **closed** | 1 |
| `domchk-80860fb2` | 2026-09-02T07:56:30 | **closed** | 1 |
| `domchk-f7865662` | 2026-09-02T10:11:56 | **closed** | 1 |
| `domchk-6a227734` | 2026-09-06T17:56:16 | **open** | 2 |

</details>

---

## Context and disposition

**Target bead state.** `bf-4x12ec` is **Closed** (2026-08-17). Its deliverable — the git-gc repair of
the ~18 GB loose-object bloat — completed and was later exceeded (repo now ~94–107 MB). This is the
basis for treating every unresolved wave-1 alert as stale rather than actionable: the alert layer
fired once per kill before needle 0.4.2 deduplicated, and no worker ever went back to close them.

**Two-wave pattern.** The 44 kills on 2026-08-14 each produced an alert bead; the 2026-08-26
regeneration wave then re-derived 16 investigation beads against the same already-closed target —
plus 3 more found in body only (`domchk-862d95d1`, `domchk-30d451d3`, `domchk-0bda808c`, all closed).
The 2026-09-02 wave of 38 beads is a further split of the same investigation, not new crash evidence.

**Not a duplicate-alert regression.** The umbrella question — *are duplicate crash alerts being
generated for bf-4x12ec today?* — is answered by the creation dates: nothing after 2026-09-02
references bf-4x12ec as a new alert. The 2026-08-26 wave is a regeneration of *investigations*, not
of *alerts*; no `ALERT: Agent crash on bead bf-4x12ec` bead was created after 2026-08-14.

## Caveats

- Statuses are a snapshot at generation time. Beads being closed concurrently by siblings
  (`domchk-e9ed52aa` in particular) will make the not-closed lists drift immediately.
- `bead list --json` reports the base status; a `blocked` state is derived and is not reflected here.
- Revision numbers are shown for wave-1 alert beads because high revision counts (up to 26 on
  `bf-22h8jj`) indicate repeated release-cycling rather than substantive rework.

## Sibling beads in this split

- Umbrella: `domchk-bcdd4b4f` — Check for duplicate crash alerts on bf-4x12ec
- Child 1 (this bead): `domchk-f05d6f91` — Inventory all bf-4x12ec crash alert beads and their statuses
- Child 2: `domchk-b2b77f12` — Verify bf-4x12ec alert beads against false-positive evidence
- Child 3: `domchk-1e38a7c2` — Document the two-wave bf-4x12ec duplicate-alert pattern
- Child 4: `domchk-a3690174` — Confirm crash-alert mitigations prevent bf-4x12ec regeneration
- Child 5: `domchk-e9ed52aa` — Close residual open bf-4x12ec wave-1 alert beads as false positives
