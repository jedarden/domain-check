# Crash Resolution Record — bf-1ea4g (final)

**Date:** 2026-09-07
**Bead:** domchk-e2c1e79e ("Verify crash prevention improvements and document learnings")
**Target:** bf-1ea4g — "Document local main branch state" (closed 2026-08-13T09:10:16Z)
**Status:** ✅ RESOLVED — kill mechanism prevented and replay-proven; alert layer false-positive and draining

> **What this document is.** The resolution ledger this bead's dispatch asked for: the
> closing verdict, the live verification battery that supports it, and the
> lessons-learned / future-investigation checklist. It is a **pointer-and-verification**
> record, not a competing root-cause narrative — every factual claim about the crash
> itself is cited to the canonical record and its evidence bundle (§7), not re-derived.
>
> **Dispatch premises corrected up front** (both already corrected by sibling beads):
> 1. `docs/crash-prevention-guide.md` **does not exist**; the live file answering to it
>    is [`docs/comprehensive-crash-prevention-guide.md`](comprehensive-crash-prevention-guide.md)
>    (premise correction registered by domchk-29af544b in 38db68a, which also flags that
>    guide's overclaimed metrics — M-4). Learnings were added there, not at the
>    nonexistent path.
> 2. "Final report" documents for this bead already circulate
>    (`docs/investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md`,
>    plus the 2026-09-02 same-day corpus). **The canonical record is and stays**
>    [`docs/crash-inventory-bf-1ea4g-summary.md`](crash-inventory-bf-1ea4g-summary.md);
>    this file exists to satisfy a named deliverable and to timestamp the 2026-09-07
>    ~16:30Z verification pass, and is cross-registered there as §5.4.

---

## 1. Resolution verdict

| Question | Answer | Basis |
|---|---|---|
| Is the crash resolved? | **Yes.** Target closed 2026-08-13T09:10:16Z by its own attempt 57; deliverable on origin/main; zero data loss | Canonical §1/§5, re-verified via `bead show` by three sibling beads 2026-09-07 |
| Root cause (final) | Unbounded `git push` (pack-objects) materializing a **422-commit unpushed backlog** still carrying retired bead-forge object mass, inside the dispatch scope's 12 GiB `MemoryMax` → memcg-OOM-class SIGKILL → needle's `exit_code: -1` sentinel | Canonical §4 (HIGH on operation: 54/57 transcripts die inside push; MEDIUM on the exact instant's killer — no Aug-13 kernel record can exist) |
| Classification | **Two layers, separately:** the kill = INFRASTRUCTURE; the alerts = FALSE_POSITIVE (the bead self-recovered the same morning) | Canonical §1, §3 |
| Code defect | **NONE** — the killed process was `git` | Canonical §3, consistent with every investigation in this corpus |
| Is the mechanism prevented today? | **Yes, and replay-proven** — memory bound on gc *and* push (≈3 GiB worst case), `test-gc-memory-bounds.sh` reconstructs the death operation and asserts it survives a 768 MiB scope | §2, this session |
| Is the *precondition* monitored? | **Yes as of 2026-09-07** — the M-1 unpushed-backlog monitor landed in `check-repo-health.sh` (8d326cc, domchk-f239e178) and reports `0 ahead, CLEAR` | §2, §3 |
| Is the alert layer fixed? | **Partially — this is the open half.** The six 2026-09-02 fixes exist and their suites pass, but the pipeline has never fired in production (D-1..D-10); the duplicate-alert backlog is draining (56 ALERT-shaped beads, 10 unresolved in the bf-titled pool as of this session) | Gap analysis §6; §2 census |

## 2. Verification battery — live, this session (2026-09-07 16:29–16:45 UTC)

Every line below is first-hand output from this session, run in the shared worktree at
HEAD 8d326cc (origin/main synchronized 0/0).

| # | Check | Result |
|---|---|---|
| 1 | `./scripts/test-closed-bead-filter.sh` — the false-positive functional test (fabricated exit −1 trace for closed bead bf-2vtzg through the alert manager in a sandbox) | **7/7, exit 0** — closed bead skipped by the FIX 1 gate, no alert logged |
| 2 | `./scripts/test-gc-memory-bounds.sh` — includes the **bf-1ea4g death-operation replay**: bounded `git push` over a 192 MiB unpacked backlog under `MemoryMax=768M` | **16/16, exit 0** — push exits 0, remote receives the backlog, store stays loose, peak push RSS **232,504 KB** vs the >12 GiB the unbounded push consumed on 2026-08-13 (4th independent pass; byte-identical to §5.1's run — the workload is deterministic); gc-side replay pack-objects 320,436 KB |
| 3 | `./scripts/test-crash-alert-fixes.sh` — the six-fix suite | **13/13, exit 0** — *run against the worktree copy, which carries a co-tenant's uncommitted +38-line Test 13* (a second closed-bead functional check; in-flight, disclosed, not attributed as landed) |
| 4 | `./scripts/check-repo-health.sh` | **exit 0** — `.git` 106 MB, 372 loose objects (3.50 MiB), 1 pack 99.11 MiB, **0 garbage**; pack-memory-bound section ✅ (≈3072 MiB worst case); **Unpushed Commit Backlog: 0 ahead of origin/main, CLEAR (warn ≥50)** |
| 5 | `./scripts/setup-git-gc-config.sh --verify` — the direct fix | **exit 0** — effective bound resolved system → global → local, ≈3072 MiB worst case (windowMemory 2g / deltaCache 1g / threads 1), covers gc *and* push |
| 6 | `./scripts/preflight-health-check.sh` | **4/4 passed, exit 0** |
| 7 | Scheduled maintenance | **8/8** `domain-check-*` systemd user timers present |
| 8 | Structural guards | `git ls-files .beads \| wc -l` = **0**; `setup-git-hooks.sh --check` exit 0 (10 MB pre-commit gate current); `origin/main..HEAD` / `HEAD..origin/main` = **0 / 0** |
| 9 | Alert pool census (`bead list --limit 20000 --json`) | **88** beads title-mention bf-1ea4g; **56 ALERT-shaped** (one per kill); bf-titled pool unresolved **10** (7 open + 3 in_progress) — down from the same-day census's 12; across all 88 title-mentions incl. domchk-* investigation beads, **24** unresolved, of which the last open split child **domchk-cb9eb4de** is in flight |
| 10 | False positives generated by this testing | **Zero.** No ALERT-shaped bead created in the 2 h window spanning the runs; the suites write their alert state to per-run sandboxes, and no live `alert-state.json` / `processed-alerts.txt` was created |

## 3. Will a similar crash be detected and prevented?

**Prevented (repo side — the strongest layer in the corpus):**

1. **The kill operation is bounded.** `pack.windowMemory=2g` + `deltaCacheSize=1g` +
   `pack.threads=1` (the window limit is per-thread) bound every pack-objects run —
   including `git push`'s — to ≈3 GiB worst case, applied repo-locally *and* globally so
   a bare gc/push sees it, with `--verify` checking the effective chain daily.
2. **The death operation itself has a recurrence test.** `test-gc-memory-bounds.sh`
   rebuilds a near-identical-shape unpacked backlog and asserts the push survives a
   1/16th-scale dispatch scope. This is the assertion that did not exist when
   bf-1ea4g died.
3. **The precondition now has telemetry.** M-1 (the one detection rule this crash
   motivates) landed 2026-09-07 in `check-repo-health.sh`: `rev-list --count` against
   upstream, warn ≥50 / CRITICAL ≥200, run daily by the 02:00 timer. On Aug-13 nothing
   in the workspace counted commit-ahead; the 422-commit condition accumulated
   silently across ~30 killed attempts.
4. **Re-entry is blocked:** `.beads/` wholly gitignored (0 tracked files), 10 MB
   pre-commit gate, repo at 106 MB (was 18 GB era).

**Not yet detected — the honest residue (all registered, none new here):**

- **H-1, the multiplier, is still open (NEEDLE-side).** Nothing checks, before
  re-claiming a bead, whether prior attempts died identically on the same operation.
  bf-1ea4g's loop re-claimed every ~2 min for 110 min, and each attempt committed
  before pushing, so the loop grew its own kill condition: 1 kill → 56. Until a
  per-bead consecutive-failure stop-condition exists, prevention (1)–(3) bound the
  *damage* per attempt, not the *count* of attempts.
- **The alert pipeline remains outside the creation path** (D-1..D-10): the suites
  pass, but no production caller invokes the manager, and four load-bearing parses do
  not match the real alert shape. The load-bearing false-positive prevention today is
  procedural — verify the target bead's actual state before investigating — which is
  what drained 46 of these 56 alerts. The hourly `alert-triage-sweep.sh` timer is the
  carrier for the backlog half; judge repair by the ledger flipping, not by suites.
- **Dispatch-scope memory telemetry (M-2) and transcript retention (M-3)** remain
  external asks; without them the next memcg kill is again provable only by
  inference from a sentinel.

## 4. Lessons learned

The canonical record's §8 carries the ten investigation-technique learnings; the four
below are the ones this resolution pass adds or re-proves:

1. **A crash can be fully resolved and still leave work open.** The kill was prevented
   within days; the alert beads it minted were still being drained 25 days later, and
   the detection rule for its precondition landed 25 days later. "Resolved" must state
   *which layer* — otherwise a reader inherits all three clocks as one.
2. **In-flight sibling work is a status, not a gap.** The gap analysis (10:23Z today)
   recorded M-1 as an uncommitted co-tenant edit and declined to attribute it; five
   hours later it was on origin/main (8d326cc). Re-verify landed-vs-in-flight at the
   moment you write, and timestamp the check — a verification record that does not
   carry its own time is how anachronistic premises spread (canonical learning #7).
3. **Suite counts drift as sibling owners grow suites** (12/12 → 13/13 here; the
   closed-bead functional check now exists in two suites). Cite the count you measured
   with the hour you measured it, never a remembered one.
4. **Dispatch-named deliverables deserve an existence check before a creation.** This
   dispatch named a guide path that does not exist and a doc path nothing had ever
   written. `git log --all -- <path>` first: creating a file a sibling deleted on
   purpose, or "fixing" a premise a sibling already corrected, is how this corpus grew
   four circulating crash instants for one bead.

## 5. Checklist for future crash investigations

**Before investigating (false-positive gate):**

- [ ] `bead show <target>` — is the bead actually open? (Most alerts point at work
      another worker already closed.)
- [ ] `git log --all --grep <bead-id>` and `git log origin/main..HEAD` — does a dead
      prior attempt's deliverable already exist, unpushed or on a local branch?
- [ ] Read the target bead's *own* deliverable; near-identical artifact **titles**
      across beads are the main wrong-report source — check the Related Bead field.
- [ ] Trust closure only from the store's own close events
      (`.beads/checkpoint/forensic.jsonl`), never from a note or doc claiming it.

**Classifying:**

- [ ] Treat `exit_code: -1` as a sentinel with **no** signal number; never read SIGHUP
      or SIGKILL-9 out of it.
- [ ] Resolve the alert's timestamp to the `agent.completed` record in the worker log
      (alert stamps are heartbeats seconds after the real kill).
- [ ] Find where in each attempt the death landed (last-tool-call / bracket analysis).
      Operation-correlated deaths ⇒ the operation's own resource profile;
      time-correlated ⇒ fleet event. This is the mechanism discriminator.
- [ ] Classify the **kill** and the **alert** as two separate layers (here:
      INFRASTRUCTURE and FALSE_POSITIVE, both true).
- [ ] Check which day any health/state premise was measured on — premises can be
      anachronistic by weeks.

**Attributing and evidence:**

- [ ] Establish the evidence classes that exist *for that date* before concluding
      (journald starts 2026-08-15, health collector 2026-08-15 23:53 EDT, traces are
      single-slot; worker transcripts and `attempt-index.tsv` are the durable ones).
- [ ] Prefer kernel records for *that* a kill happened; transcripts for *what the
      agent was doing* — the latter is what settled this crash.
- [ ] Disclose co-tenant in-flight work separately; never attribute, sweep, or commit it.

**Closing out:**

- [ ] Append dated subsections to the canonical record; if the dispatch names a new
      path, banner the file as a pointer and cross-register it.
- [ ] Re-run every live claim you cite, this session, and stamp the times.
- [ ] Record what remains open *per layer*, with owners, so "resolved" cannot be read
      as "nothing left".

## 6. What this bead changed

- Created this resolution record (verification battery §2, prevention/residue §3,
  lessons §4, checklist §5).
- Added the bf-1ea4g final-learnings section to the live
  [`comprehensive-crash-prevention-guide.md`](comprehensive-crash-prevention-guide.md)
  (the dispatch's `crash-prevention-guide.md` path does not exist).
- Cross-registered this pass as §5.4 of the canonical
  [`crash-inventory-bf-1ea4g-summary.md`](crash-inventory-bf-1ea4g-summary.md).
- No code change: every detection improvement the dispatch asked to verify already
  existed and passed live (§2). M-1 had landed hours before this session (8d326cc).

## 7. References

- **Canonical record:** [`docs/crash-inventory-bf-1ea4g-summary.md`](crash-inventory-bf-1ea4g-summary.md) (364f187) → root-cause determination `docs/investigations/bf-1ea4g-root-cause-determination-2026-09-02.md`; evidence bundle `docs/crashes/bf-1ea4g/` (2ce9cd9: README, attempt-index.tsv, attempt-30 bracket + transcript)
- **Gap analysis:** [`docs/crash-prevention-gaps-bf-1ea4g.md`](crash-prevention-gaps-bf-1ea4g.md) (38db68a) — seven-stage decomposition, H-1/H-2/M-1..M-4/L-1/L-2, the FP-measure-by-measure evaluation
- **Gap registers:** [`docs/crash-prevention-requirements.md`](crash-prevention-requirements.md) (G-1..G-13); [`docs/alert-deduplication-gap-analysis-2026-09-07.md`](alert-deduplication-gap-analysis-2026-09-07.md) (D-1..D-10)
- **Prevention commits:** a4c8ffa (preventive measures + operator verification battery, domchk-87ef5683); 2e8ce7a/99306f5 (death-operation replay test, domchk-9d840579); 8d326cc (M-1 unpushed-backlog monitor, domchk-f239e178)
- **Mechanism corroboration:** `docs/crashes/bf-198ne-crash-report.md` (push-side variant, kernel-proven); `docs/crash-artifacts-bf-3561g.md` §4/§6; `docs/branch-divergence-analysis.md` (backlog 660 → 422 → 0)
- **Standing procedure:** `CLAUDE.md` ("Crash Prevention and Investigation"), `docs/crash-response-guide.md`

---

*Verification figures in §2 and the census in §2#9 were measured 2026-09-07 16:29–16:45 UTC by domchk-e2c1e79e; historical figures are cited, not restated as first-hand.*
