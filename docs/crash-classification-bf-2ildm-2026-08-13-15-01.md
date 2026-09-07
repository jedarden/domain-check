# Crash Classification: bf-2ildm @ 2026-08-13T15:01:52.450373520+00:00

**Classification dispatch:** domchk-e05fa5a8 (2026-09-07)
**Crash target:** bf-2ildm — "Extract GitHub-specific commits" (CLOSED 2026-08-16T22:44:38Z)
**Alert bead carrying this stamp:** bf-z15pix (created 2026-08-13T15:01:52.460Z)
**Reported exit code:** −1
**Alert-level verdict:** FALSE_POSITIVE (resolved target; no work lost)
**Kill-level verdict:** INFRASTRUCTURE — repository-bloat-era kill regime
**Confidence:** HIGH (per-attempt event index; no surviving kernel record for Aug-13)

This is the per-instant classification record for the alert whose carried stamp is
`2026-08-13T15:01:52.450373520+00:00`, companion to
`docs/archive/crash-investigations/crash-summary-bf-2ildm-timestamp-2026-08-13-13-44.md`
(the 13:44:20 instant). It also dates the correction the 2026-09-02-era bf-2ildm
documents need: their conclusion "no actual crash occurred" is true at the **alert**
level and false at the **kill** level.

## Verdict

Two levels, and they answer different questions:

| Level | Classification | Meaning |
|-------|----------------|---------|
| **Kill** (what killed attempt 24) | **INFRASTRUCTURE** — repository-bloat sub-type | Real mid-task death, exit −1, in the Aug-13 bloat-era kill regime |
| **Alert** (bf-z15pix's claim that work was lost) | **FALSE_POSITIVE** | Target bf-2ildm completed on 2026-08-16 and closed successfully; nothing needed re-doing |

The kill was real; the alarm was not. Both findings can hold at once, and for
bf-2ildm they do. (Same two-level structure as bf-1ea4g: the alert-level
FALSE_POSITIVE is about the alert, not the kill.)

## 1. The carried stamp is a handler heartbeat, not the death instant

Per the per-attempt index (docs/crashes/bf-2ildm/attempt-index.tsv,
crash-alert-ledger.tsv, extracted 2026-09-07 from the needle worker log):

- **Real death of the attempt this alert names: 2026-08-13T15:01:35.775155479Z**
  (attempt 24 — claimed 14:56:25.960Z, killed after 309,461 ms ≈ 5 m 09 s, mid-task).
- **Carried stamp 15:01:52.450Z is ≈16.7 s AFTER that kill**, and 10 ms before the
  alert bead's creation (15:01:52.460Z) — the crash handler's post-kill heartbeat.
- This is uniform across the bead: all 38 alert stamps sit 7.7–29.4 s after their
  attempt's kill. No dispatch-named "crash instant" on this bead is a death time.

## 2. Exit code −1: what it does and does not tell you

Per `docs/crash-response-guide.md` (Quick Reference note 2): **exit −1 is needle's
sentinel for a signal death with no recorded exit code** (`code().unwrap_or(-1)`) —
it is not a signal number, and the alert text's "signal -1" merely echoes that
sentinel. A genuine SIGKILL death encodes 137 (128+9); SIGHUP would encode 129.

No kernel record can upgrade the sentinel for this bead: journald on this host has a
single boot beginning 2026-08-15 19:56:33 EDT, dmesg is restricted, and the box
rebooted twice on Aug-14 — **no Aug-13 kernel line survives for any bead**. The
memcg-OOM mechanism (SIGKILL inside the 12 GiB per-dispatch scope, kernel
`CONSTRAINT_MEMCG`, git pack-objects the largest victims) is *proven* for the same
morning's sibling beads and is assigned here **by regime match**, not by a direct
kernel record for bf-2ildm.

## 3. The kill regime: 43 attempts, 38 signal deaths, fixed cadence

bf-2ildm's Aug-13 attempt series (all times UTC, from the attempt index):

| Attempts | Exit | Window | Character |
|----------|------|--------|-----------|
| 1–38 | **−1** | 13:35:34 → 15:53:48 | Signal deaths, fixed ~2–5 min re-dispatch cadence, durations 98–309 s (mid-task, never instant) |
| 39–42 | 124 | 16:03:48 → 16:35:06 | Needle's 600 s dispatch cap |
| 43 | 1 | 16:35:26 (19 ms) | Immediate failure, quarantined — dispatching stopped |

Fixed-cadence exit −1 re-dispatch deaths are the guide's **"Infrastructure:
repository bloat"** sub-type (Pattern 3): the trigger is the repository's own size,
so the kill recurs on every dispatch until the repo is cleaned. Aug-13 was the
bloat era's peak day — 18 GB of loose objects, `.beads/` snapshots still tracked, a
422-commit unpushed backlog. The same-day census (commit `d9c4622`, R5) attributes
**38 exit-minus-one kills to bf-2ildm** alongside bf-65lsdu 127, bf-1ea4g 56,
bf-4k2ws 55, and bf-1s6c3 22 — five beads dying in the same regime on the same day.

Attempt 1's surviving transcript (docs/crashes/bf-2ildm/session-transcript-attempt01-81bc97cc.jsonl)
ends mid-task: the last assistant turn reasons "Good! Now I need to push and then
close the bead" and dies at the Bash call — the same work-then-die-at-git shape as
the era's proven push/pack kills. Per-attempt transcript coverage beyond attempt 1
was not re-derived for this classification; the attempt index is the authoritative
count.

## 4. Why the alert is still a false positive

- **Target state:** bf-2ildm is CLOSED (2026-08-16T22:44:38Z, revision 6). The
  successful retry's trace metadata shows `exit_code: 0, outcome: success`,
  captured 2026-08-16T22:28:44Z (85.3 s). All acceptance criteria were met; the
  deliverable is in the chain's committed analysis.
- **Dedup gate:** `./scripts/alert-deduplication.sh check bf-z15pix` →
  `DUPLICATE: crash target bf-2ildm is already resolved` (exit 0).
- **Nothing was lost:** 38 killed attempts cost ~2.3 hours of wall time, not work —
  the bead's task completed on the Aug-16 retry. The alert's premise (work to
  recover) is false even though the kills were real.

## 5. Dated correction to the earlier bf-2ildm record

The 2026-08-26 / 2026-09-02 verification corpus (including
`docs/archive/crash-investigations/verification-report-bf-z15pix-false-positive-crash-alert-resolved-bf-2ildm.md`
and the target bead's own notes) concluded "No actual crash occurred" and framed the
Aug-13 alerts as fabricated placeholder data generated "3+ days BEFORE completion."
Two parts of that framing need dating:

1. **The alert stamps are not fabricated and not premature** — each is the handler's
   heartbeat seconds after a real kill. The alerts were misdirected (aimed at a bead
   whose work would later succeed), not invented.
2. **The kills happened.** "No crash" holds only at the alert level. The per-attempt
   index (2026-09-07) is the evidence the earlier corpus lacked; before it existed,
   the single-slot trace archive (which retains only the last attempt — the
   successful Aug-16 one, exit 0) made the Aug-13 deaths look impossible. That is an
   artifact-retention blind spot, not a false alarm.

The alert-level FALSE_POSITIVE verdict and its fixes (closed-bead filtering, dedup,
7-day history window) stand unchanged.

## 6. Sources

- `docs/crashes/bf-2ildm/attempt-index.tsv`, `crash-alert-ledger.tsv`,
  `needle-events-2026-08-13-bf-2ildm.jsonl.gz`, `session-transcript-attempt01-81bc97cc.jsonl`,
  `trace-archive-current-state.json` — extracted 2026-09-07 (untracked sibling
  bundle at classification time; its owning chain commits it)
- `docs/crash-response-guide.md` — classification table, exit −1 sentinel note (2),
  repository-bloat sub-type, 124/timeout semantics
- `docs/archive/crash-investigations/crash-summary-bf-2ildm-timestamp-2026-08-13-13-44.md`
  — companion instant record (13:44:20 = attempt 3's alert, bf-1wkda)
- Commit `d9c4622` (domchk-508e54c0) — Aug-13 per-bead kill census (bf-2ildm 38) and
  the Aug-13 telemetry coverage gap
- Commit `4963782` (domchk-1ec90d5e) §12.5 — the alert-family artifact index and the
  two-level alert/kill framing
- Live store: `bead show bf-2ildm` (closed, rev 6), `bead show bf-z15pix` (open),
  `scripts/alert-deduplication.sh check bf-z15pix` → DUPLICATE

## 7. Signposts (not this dispatch's scope)

- **bf-z15pix** (open, carries this dispatch's exact stamp) already has a committed
  verification report (archived, 2026-08-26) and a DUPLICATE gate result; it belongs
  to its own verify-and-close chain. Eight more alert beads from this storm are
  open and three in progress — same disposition.
- The sibling artifact bundle under `docs/crashes/bf-2ildm/` is that chain's
  deliverable; this doc cites its figures rather than duplicating the bundle.
