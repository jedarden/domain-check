# Archived Crash Investigation Documents

**Archived:** 2026-09-06 (bead domchk-87a7bb2a)
**Count:** 387 files — 372 from the `docs/` root, 15 from `docs/research/`
**Method:** `git mv` (history preserved; nothing deleted)

---

## What this is

One-off working documents produced during the 2026-08-12 → 2026-09-06 crash
investigation period: per-bead investigation reports, verification reports,
fix proposals, root-cause analyses, divergence snapshots, and evidence
extractions. They were accumulating flat in `docs/` (469 markdown files at the
root) and in `docs/research/`, burying the documents that are actually
load-bearing.

**The entry point for crash prevention is not in here.** Start at
[`docs/crash-prevention-requirements.md`](../../crash-prevention-requirements.md)
— the consolidated, live-verified inventory of safeguards and gaps. It
supersedes the conclusions scattered across these files.

## ⚠️ Reading these files

Claims in this archive are **frozen as written and often wrong**. The
consolidated requirements doc records the corrections; the recurring ones:

- **`exit -1` is a needle sentinel for "died by signal", not a signal number.**
  Files here claiming SIGKILL or SIGHUP from the exit code alone are
  guessing. Kernel records (`docs/crashes/bf-198ne-crash-report.md`) are the
  authority.
- **The 70/20/8/2 crash distribution** appears here in five mutually
  inconsistent variants. None is derived from a counted population.
- **`bf-4x12ec` was first reported as "NOT OOM"** in these files; the memcg
  evidence later inverted that conclusion.
- Repository-size figures predate the 2026-09-01 cleanup (18 GB → 94 MB) and
  do not describe the current repo.

Treat any single file here as one worker's contemporaneous notes, not as
settled fact.

## Layout

- `./` — documents moved from the `docs/` root (paths otherwise unchanged)
- `research/` — crash-flavoured documents moved from `docs/research/`

## What was deliberately NOT archived

- **Everything referenced from outside `docs/`** — code, scripts,
  `CLAUDE.md` (both levels), `README.md`, and the repo-root `MEMORY.md`.
- **The reference closure of those files** — any document they link to stays
  put, so no active document has a link that moved out from under it.
  106 documents were protected in total.
- **`docs/research/01`–`14` numbered series** and the other RDAP / build
  pipeline research — that is the project's research corpus, not crash
  output.
- Organized subdirectories (`docs/crashes/`, `docs/maintenance/`,
  `docs/benchmarks/`, `docs/plan/`, `docs/notes/`, `docs/adr/`,
  `docs/crash-investigations/`, …) and everything under `docs/archive/`
  already.

## Why archived rather than deleted

No file here was byte-identical to another (checked by content hash), so
nothing met a removal bar that would not also lose information. Archiving
gets the clutter out of the working documentation set at zero information
cost; the canon requirements doc's P6 finding is that evidence in this
workspace has repeatedly turned out to matter only after it was nearly lost.
