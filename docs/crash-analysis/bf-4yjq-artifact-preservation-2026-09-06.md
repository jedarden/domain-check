# bf-4yjq crash artifacts — preservation pass (2026-09-06)

**Dispatch bead:** domchk-b6a900e4 — "Gather crash information and system state"
**Subject bead:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo mirror has
diverged/gone stale" (P2, **closed** 2026-08-17). The bead itself is a completed task, not a
crash investigation; the crash record attached to it is a storm of 50 worker deaths during
2026-08-12, each of which spawned an "ALERT: Agent crash on bead bf-4yjq" alert bead.

**This pass preserves raw artifacts. It derives no new analysis.** The canonical analysis is
[`docs/crash-investigations/bf-4yjq-crash-investigation.md`](../crash-investigations/bf-4yjq-crash-investigation.md)
(domchk-4eab7c59) and the live-verified evidence inventory is
[`docs/crash-investigations/bf-4yjq-artifact-catalog-2026-09-06.md`](../crash-investigations/bf-4yjq-artifact-catalog-2026-09-06.md)
(domchk-e92faa40). Where older reports disagree with the canonical record, the canonical record
wins — see "Supersessions" below before quoting either preserved compilation.

## Files preserved here

| File | Bytes | Source | What it is |
|------|-------|--------|------------|
| `bf-4yjq-needle-worker-log-extract.log` | 94,212 | `~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2` | **Primary crash-era telemetry** — all 225 `bf-4yjq` worker-log records of 2026-08-12, verbatim. The only surviving Aug-12 source for this storm |
| `bf-4yjq-system-state-snapshot-2026-09-01.txt` | 14,498 | `.beads/crash-bf-4yjq-summary.txt` | Contemporaneous metrics compilation: repo/git state, memory, load, disk, error formats. Superseded on crash count |
| `bf-4yjq-resolution-record-2026-09-01.md` | 10,152 | `.beads/crash-bf-4yjq-resolution.md` | Resolution record (cleanup + mitigations). Superseded on crash count and on the gc-peak claim |

Both `.beads/` sources are **outside git** (`.beads/` is gitignored) and would be lost to a
workspace rebuild; the worker log is outside git entirely.

## Why now — the primary source is one rotation from erasure

`…lab-domain-check.log.2` is rotation slot 2 of a size-based (~128 MiB) scheme that keeps two
slots. Re-verified live 2026-09-06:

- exists, 134,207,883 bytes, mtime 2026-08-15 16:35 EDT; covers
  2026-08-11T14:12:54Z → 2026-08-15T20:35:19Z (mtime is EDT; the coverage stamps are UTC —
  consistent, 16:35 EDT = 20:35 UTC)
- the active slot `…lab-domain-check.log` was already at 90,732,929 bytes on 2026-09-05

The catalog (written 2026-09-06, earlier the same day) warned "two more rotations erase it";
with the active slot past 90 MB, **the next single rotation shifts `.1`→`.2` and overwrites the
Aug-12 evidence**. Only the 50 crash-death timestamps (catalog §2.1) were preserved in git
before this pass; the full 225 records now are.

## Extraction method and integrity

```
grep 'bf-4yjq' ~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2 \
  > docs/crash-analysis/bf-4yjq-needle-worker-log-extract.log
```

- 225 records, 2026-08-12T17:50:23Z (first claim) → 2026-08-12T21:14:59Z (last record)
- Event breakdown re-counted live from the extract on 2026-09-06 — **matches catalog §2
  exactly**, an independent re-derivation: 56 `claim_auto` claims; 50 `exit_code=-1
  outcome=Crash(-1)` deaths (17:53:53.875 → 20:30:38.310 UTC); 50 `agent crashed — releasing
  bead and creating alert` lines; 50 `crash alert bead created` lines; 1 `exit_code=1` failure
  (18:00:17); 4 `exit_code=124` timeouts (20:40:47, 20:51:01, 21:01:14, 21:11:27); 1
  `exit_code=0` success that still left the bead open (21:14:56, "orphaned … status=blocked")
- sha256:
  - extract `95d7f713ceea166e6e5c2e67b5f1a0c5c423f58610cce6e74b5726d54a393255`
  - snapshot `631983ab0cedf861e2e942b9b5d107948654e1d6851841244d3b5b4376dc0dc6`
  - resolution `5776c08a8eb22d726e4c7c5f840eeaa7d5b8f19f4e335f6fcab251eff80b134b`

All three copies are byte-identical to their sources (verified by hash on 2026-09-06). The two
`.beads/` copies are preserved unmodified — including their now-superseded claims — because
editing a preserved artifact would destroy its evidentiary value; corrections live here.

## Acceptance criteria — evidence mapping

| Criterion | Result |
|-----------|--------|
| Locate crash logs for bf-4yjq | **Located and preserved.** `bf-4yjq-needle-worker-log-extract.log` (primary). No session traces, core dumps, or kernel records survive for Aug-12 — see below |
| System state at crash time (memory, load, disk) | **Preserved as compiled 2026-09-01**: `bf-4yjq-system-state-snapshot-2026-09-01.txt` — 18 GB repo / 17.16 GB loose objects / 9.60 MiB packed; load 15–17 on 12 cores; disk 84 % (~71 GB free), inodes 80 %; I/O 43 MB/s read / 18 MB/s write. Not a live `free`/`vmstat` capture — a post-hoc compilation. The binding memory limit was later corrected to the dispatch cgroup, not the host (see Supersessions) |
| Agent process information | **From telemetry**: worker `claude-code-glm-4.7-lab-domain-check`, agent/model `claude-code-glm-4.7`, needle session `8446529e`, workspace `/home/coding/domain-check`, 56 dispatch attempts. Per-attempt survival is in `docs/crash-investigations/bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md`: min 65 s, **median 149 s**, p75 216 s, max 375 s, mean 174 s. **Per-PID and RSS-at-death are unrecoverable** — see below |
| Save crash artifacts to `docs/crash-analysis/` | This directory previously held bf-1s6c3 artifacts only; the three bf-4yjq files above are new here |

## Confirmed absences (not recoverable, per catalog §6, re-checked 2026-09-06)

| What | Why |
|------|-----|
| Per-PID / per-attempt RSS at death | No coredumps exist before 2026-08-25, and SIGKILL-class deaths produce none |
| Kernel OOM / journald records for Aug-12 | System journal starts 2026-08-15 19:46:33 EDT; user journal 2026-08-17 15:33:14 EDT (single boot) |
| Crash-era session transcripts / stack traces | Per-attempt agent JSONL deleted; trace capture did not yet exist (`.beads/traces/bf-4yjq/` absent) |

The kill mechanism itself is not in doubt — it is established from later, better-instrumented
reproductions (kernel `CONSTRAINT_MEMCG` kills inside the `MemoryMax=12 GiB` dispatch scope;
crash command re-run under a bound exits 0 at ~313 MiB peak) — see
`docs/crashes/bf-4yjq-consolidated-findings-domchk-4ed0544b-2026-09-06.md`.

## Supersessions — read the preserved copies with these corrections

1. **Crash count.** Both 2026-09-01 compilations say **9 crashes**. The canonical record is
   **50 crashes** (17:53:53.875 → 20:30:38.310 UTC), independently re-derived from the same
   worker-log slot by catalog §2 and re-counted from the extract preserved here. The "9"
   figure came from the alert-bead corpus subset examined at the time.
2. **"gc completed successfully, ~1.1 GB peak"** (resolution record, citing the Aug-14
   cleanup). Superseded per repo CLAUDE.md: those Aug-14 `git gc --aggressive` runs were
   **memcg-OOM SIGKILLs** at the dispatch scope's 12 GiB bound; the kernel records proving the
   mechanism were recovered later (bf-4x12ec / bf-198ne). The repo was packed to ~93 MB by a
   later non-attempt process, and the claim's "no OOM events" conclusion does not hold.
3. **"Multiple concurrent git operations exhausted available memory"** (snapshot's crash
   mechanism). The corrected mechanism is a single operation's working set exceeding the
   dispatch cgroup bound while the 62 GB host had memory to spare — which is why host-wide
   monitoring missed it.

## Rest of the corpus (pointers, not duplicates)

- Canonical report: `docs/crash-investigations/bf-4yjq-crash-investigation.md`
- Evidence inventory (live-verified, supersedes older catalogs):
  `docs/crash-investigations/bf-4yjq-artifact-catalog-2026-09-06.md`
- Consolidated findings + fix verification:
  `docs/crashes/bf-4yjq-consolidated-findings-domchk-4ed0544b-2026-09-06.md`,
  `docs/crashes/bf-4yjq-fix-proposal-verification-2026-09-06.md`
- Per-crash-death detail (50 timestamps + survival stats):
  `docs/crash-investigations/bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md`
- Cleanup verification (still holding 2026-09-06): `docs/crashes/bf-4yjq-cleanup-verification.md`
