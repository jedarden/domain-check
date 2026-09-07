# bf-4yjq log extraction — working-directory corpus (domchk-dd389d34)

**Bead:** domchk-dd389d34 — "Extract bf-4yjq agent logs from storage" (P2)
**Deliverable:** `/home/coding/domain-check/.beads/state/bf-4yjq-log-extraction/` (≈300 MB)
**Completed:** 2026-09-06 · built cumulatively across four dispatches of the bead (12:19–15:28 EDT)

`.beads/state/` is gitignored by design — 300 MB of raw logs must never enter git
(that is the exact mechanism that caused the bf-1s6c3 / bf-4yjq repository bloat).
This document is the durable pointer to the corpus; the corpus itself lives on disk.

## What is in the corpus

| Directory | Size | Files | Contents |
|---|---|---|---|
| `needle-worker-logs/` | 132M | 2 | Crash-era primary worker log rotation slot `.log.2` (134 MB, coverage 2026-08-11T14:12Z → 2026-08-15T20:35Z) and the structured event JSONL `claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl` |
| `traces/` | 139M | 271 (72 bead dirs) | Per-alert-bead trace dirs for the 50 crash-alert beads — `stdout.txt`, `stderr.txt`, `trace.jsonl`, `metadata.json` (captured with `cp -a`) |
| `agent-sessions/` | 16M | 59 | Crash-era per-run agent transcripts (Aug-12 mtimes preserved) |
| `bead-store/` | 14M | 17 | Bead-store snapshots: work-completion records, crash summaries, divergence points |
| `extracts/` | 668K | 67 | Line extracts of every bf-4yjq-matching source log + `INDEX.txt` + `ZERO-HIT-SOURCES.txt` |
| `investigation-sessions/` | 576K | 13 | Session streams of prior bf-4yjq investigation dispatches |
| `predispatch/` | 8K | 1 | Pre-dispatch state record |
| `integrity/` | 96K | 2 | `SHA256SUMS` (436 files) + `integrity-report.txt` |

Tooling in the same directory: `extract.sh` (full copies), `extract-lines.sh` /
`finalize-extracts.sh` / `refresh-extracts.sh` (grep extracts),
`extract-predispatch.sh`, `verify-integrity.sh` (regenerates `integrity/`).

## Integrity status

`integrity/integrity-report.txt` (generated 2026-09-06 15:28 EDT): **611 checks
passed, 22 failed, 8 extracts whose source grew post-capture.** Every failure is
live-source drift, not a capture defect:

- 8 trace HASH-MISMATCHes — the live single-slot trace for `bf-aruwg` was rewritten
  by a later dispatch (14:33 EDT) and the `domchk-dd389d34` trace belongs to the
  dispatch running at verification time; both sources changed *after* capture. The
  captured copies are the point-in-time record.
- 6 EXTRACT-COUNT-MISMATCHes — the source `agent.jsonl` rotated away after capture
  ("source now 0": the captured extract is now the *only* copy) or is the live
  dispatch log of an in-flight worker.
- 8 "grew post-capture" — the live log store kept appending; the extract is a valid
  snapshot.

Independent re-verification at close time: **436/436 files in `integrity/SHA256SUMS`
verify OK** (copy-side `sha256sum -c`, no live-source comparison — immune to drift).

`verify-integrity.sh` compares against *live* sources, so re-running it later will
report new drift lines by design; the 2026-09-06 15:28 report plus the SHA256SUMS
check above are the authoritative record for this extraction.

## Acceptance criteria — how each was met

- **Copy bf-4yjq log files to working directory** — 300 MB corpus across the eight
  directories above, derived live from `~/.needle/logs`, `.beads/traces/`, and the
  bead store (see `extract.sh` for the exact source list).
- **Capture stdout/stderr output** — `traces/*/stdout.txt` + `stderr.txt` for the
  alert-bead trace dirs; worker-log and stderr rotation slots in
  `needle-worker-logs/` and `extracts/`.
- **Preserve file timestamps and metadata** — all copies via `cp -p` / `cp -a`;
  mtime equality is checked per file in the integrity report ("mtime preserved").
- **Verify log file integrity** — 611-check report + SHA256SUMS, independently
  re-verified 436/436 OK at close.

## Cross-references

- Sibling raw-log bundle (56 storm session transcripts, needle events, worker-log
  slot 2) committed at `docs/crash/bf-4yjq/raw-logs/` — bead domchk-495041ac. The
  two extractions cross-verify: 56/56 storm transcripts byte-identical to
  `sessions-index.tsv` sha256.
- Catalog of the 50 crash-era deaths: `docs/crash-investigations/bf-4yjq-artifact-catalog-2026-09-06.md`.
- Consumer of this corpus: **domchk-db992d55** ("Parse and identify failure patterns
  in bf-4yjq logs") — unblocked by this bead's closure. Note that substantial
  pattern analysis already exists (see `docs/crashes/bf-4yjq-crash-evidence-summary.md`
  and the domchk-b9513e0b pattern-analysis commit); that bead should reconcile
  against this corpus rather than re-derive from scratch.

## Why the auto-split was refused (2026-09-06)

The bead accumulated `failure-count:3` across four dispatches, but each attempt made
*cumulative* progress on the same on-disk corpus (extract.sh 12:19 → traces 13:21 →
extracts+integrity 13:30–13:35 → predispatch 14:40 → agent-sessions 15:18 → final
integrity report 15:28). The failures were per-dispatch turn exhaustion on a
nearly-finished task, not a task too large to hold — so splitting it into children
would have fragmented finished work rather than enabled it. The correct resolution
was to verify the deliverable, write this record, and close.
