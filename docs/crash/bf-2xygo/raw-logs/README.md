# Crash Artifact Extraction: bf-2xygo (2026-08-12)

**Extraction bead:** domchk-cd364e0a
**Extraction date:** 2026-09-06
**Source file:** `/home/coding/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl`
(3,589,712 bytes, mtime Aug 12 19:57 EDT = 23:57 UTC — covers the crash window)
**Extracted events:** 91 (byte-exact `grep` lines, original order preserved)
**SHA-256:** see `MANIFEST.sha256`

## Task that crashed

> Fetch and analyze divergence between Forgejo and GitHub remotes
> — compare tips, list commits unique to each side, identify merge base.

Bead bf-2xygo (P2, type task). Created 2026-08-12T21:12Z, closed 2026-08-12T21:30Z.
**Subject bead status: CLOSED** — task completed on the 5th dispatch attempt.

## Verified crash timeline (from the raw events, not from prior summaries)

All five attempts used an **identical dispatch**: `prompt_len=70650`,
`prompt_hash=sha256:41e63f99…5b07`, template `pluck-default`, agent
`claude-code-glm-4.7` (`agent.dispatched`, seq 4514/4542/4570/4598/4626).

| Attempt | Dispatched (UTC) | agent.completed | Exit | Duration (ms) | transform events_written |
|---|---|---|---|---|---|
| 1 | 21:15:05.484 | 21:18:21.891 | −1 | 196,226 | 29 |
| 2 | 21:18:31.034 | 21:21:25.960 | −1 | 174,755 | 29 |
| 3 | 21:21:35.874 | 21:24:44.890 | −1 | 188,795 | 28 |
| 4 | 21:24:54.121 | 21:28:24.006 | −1 | 209,674 | 35 |
| 5 | 21:28:33.650 | 21:31:21.362 | **0** | 167,541 | 25 |

Each crash attempt produced the event chain
`agent.completed exit_code=-1` → `outcome.classified outcome=crash` →
`bead.released reason=release_success` → `outcome.handled action=alerted`.
Attempt 5 produced `outcome=success` → `verification.passed` → `bead.completed`.

## Exit code −1 — meaning verified

`exit_code=-1` is needle's record of the **`wait()` status sentinel for
"process died by signal"** — it is **not a signal number**; no signal `-1`
exists. Each such event was classified `outcome=crash` and handled as an
alert. This matches the repo canon
(`docs/crashes/signal-minus1-root-cause-analysis-verified-2026-09-02.md`,
`docs/crashes/crash-analysis-exit-code-signal-1-2026-09-02.md`): "exit code −1
is NOT a signal number … exit code −1 is process exit status, not signal
number."

## Repository state at crash time

- **`.git` ≈ 18 GB, ~17 GB loose objects** (17+ identical 237MB `.beads/*.jsonl`
  snapshots committed) — this is the bf-1s6c3 / bf-4yjq bloat storm evening.
  Evidence: `docs/crashes/repository-bloat-crash-bf-1s6c3-2026-08-12.md`;
  canon re-verified 2026-09-06 in the repo CLAUDE.md.
- **Zero commits on main from 2026-08-09T13:00Z to 2026-08-12T22:00Z**
  (`git log --until`). Attempt 5's `outcome=success` therefore committed
  nothing; the divergence analysis surfaced in git only later, as
  `docs/notes/forgejo-github-sync-analysis-2026-08-26.md` (commit 3838a30,
  2026-08-26).
- The crashed task's own operation class — `git fetch` of both remotes on the
  bloated repo — is the same operation class that memcg-OOM'd elsewhere in the
  same storm window (bf-4yjq 19:21, bf-1s6c3 from 21:36; bf-2xygo's four kills
  at 21:18–21:28 fall between them).

## Root-cause attribution — two competing claims in the record

1. **Repository bloat (18 GB / 17 GB loose objects).** Contemporaneous: bf-2igib's
   close reason of 2026-08-16 states the crash "was caused by repository bloat
   (18GB with 17GB loose objects)". Consistent with the corrected Aug-12 canon
   and with the crash window sitting inside the bloat storm.
2. **CPU saturation (load 8.21–9.4, "91–104% of 9 cores").** Claimed by the
   2026-08-25 investigation (`docs/archive/crash-investigations/crash-investigation-bf-2xygo-2026-08-12.md`)
   and repeated by the 2026-09-01 duplicate-alert verification.

**Verification finding (this extraction):** the load-average table in claim 2 is
**unsourced from any surviving artifact**. The cited primary log contains **no
load or CPU metrics of any kind** (`grep` for load-bearing event types returns
nothing), and journald on this box only begins 2026-08-15 19:46 EDT, so no
kernel or sar data exists for Aug 12. Worse, the table's four timestamps
(21:15:05, 21:18:31, 21:21:35, 21:24:54) are **exactly bf-2xygo's own claim /
re-claim event timestamps** — the signature of load values attached to real
events after the fact, not measured at the time. Treat claim 1 (repository
bloat) as the better-supported attribution; claim 2 should not be cited as
measured fact.

Also corrected against the raw events: the 2026-08-25 doc's "29 events written
per attempt" is wrong — the true sequence is 29 / 29 / 28 / 35 / 25. Its
attempt table (timestamps, durations, exit codes) and prompt size (70,650)
**do** verify byte-exactly.

### Correction (2026-09-06, domchk-fe1e4a60)

The claim above that "the cited primary log contains **no load or CPU metrics
of any kind**" is **wrong**. The Aug-12 log contains 545 `fleet.cpu_saturated`
events carrying `{load_average, core_count, threshold}` (e.g. `9.11 / 9 cores /
0.8`) and 82 `worker.launch.deferred` events whose reason strings quote measured
1-minute load ("system saturated: CPU load saturated: 10.79 (1-minute average)
/ 9 cores = 1.20 > threshold 0.80"). The four load values in the 2026-08-25
doc's table (9.11 / 9.4 / 8.47 / 8.21) are therefore **real recorded
launch-gate samples, not unsourced numbers** — each was emitted within ~6 ms of
one of bf-2xygo's own dispatch events, which is why this README read them as
"attached after the fact". The circularity observation still stands: the samples
coincide with dispatch evaluations, so the Aug-25 doc's load-vs-crash
"correlation" is partly mechanical, and saturation was chronic all day (events
in every hour 05–23), so it cannot by itself discriminate bf-2xygo's deaths.
Full treatment: `docs/crashes/bf-2xygo-crash-classification-2026-09-06.md`.

## What is preserved vs. unrecoverable

**Preserved here (2026-09-06):** all 91 bf-2xygo events from the dated
dispatch log — the complete needle-side record of the crash, including exit
codes, durations, prompt hash, and per-attempt transform output counts.

**Known unrecoverable (checked 2026-09-06):**
- Kernel / OOM records for 2026-08-12 — journald begins 2026-08-15 19:46 EDT.
- Any load-average or system-metric samples from the crash window — never
  recorded in the surviving logs (see above).
- Session transcripts for these five attempts — the bf-4yjq extraction
  (`docs/crash/bf-4yjq/raw-logs/`, commit 77fac01) bundled crash-era session
  transcripts from the same evening; no bf-2xygo session transcript was
  included there and the single-slot trace has long since rotated.
- `~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check-2.log` contains
  one bf-2xygo mention, but it is a 2026-08-26 commit-hook warning from a
  later session (bf-2igib's), not crash-era evidence — not bundled.

## Prior investigation trail (from `.beads/checkpoint/forensic.jsonl`)

bf-2xygo's crash has been investigated **at least 8 times** across 6 beads:
bf-36tp5 (2026-08-16), bf-2igib (closed 4×: 08-16, 08-17, 08-26×2), bf-2lxwt
(closed 5×: 08-16 → 08-26), domchk-71e698fe (2026-08-25, the original
investigation), domchk-3042abf1 (2026-09-01, duplicate-alert verification).
bf-2xygo itself closed 2026-08-12 with the task completed on attempt 5.

**Open sibling:** `domchk-49962f7e` ("Document investigation findings for
bf-2xygo crash", created 2026-09-02, still open) owns the narrative crash
analysis write-up. This bundle deliberately covers only raw-artifact
extraction and claim verification, not a fresh investigation.

## Files

- `needle-events-2026-08-12-bf-2xygo.jsonl` — 91 raw events, byte-exact
- `MANIFEST.sha256` — checksums
- `README.md` — this provenance and verification record
