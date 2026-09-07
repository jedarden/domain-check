# Section Inventory — bf-4x12ec Crash Investigation Report (domchk-f6aba211, 2026-09-07)

Present/missing inventory of the four required sections in
`docs/crash-investigations/bf-4x12ec-crash-investigation.md`, produced for the
follow-up gap-filling bead.

**Basis:** full read of the report at HEAD `14e301d` (593 lines, report v1.6 =
body + Addenda 1–6). The worktree copy was verified byte-identical to HEAD
before citation (`git diff HEAD -- <path>` empty), so every line number below
is a HEAD line number in that file. This inventory covers the named report
only; the sibling docs `bf-4x12ec-root-cause.md` and
`bf-4x12ec-final-crash-report.md` were located (both carry their own RCA
sections) but were not inventoried.

## Verdict: 4 / 4 required sections PRESENT — nothing is missing

| Required section | Status | Lives at | Quality |
|---|---|---|---|
| Root cause analysis | ✅ PRESENT | `## Root Cause Analysis` L92–108 + `## Crash Classification` L110–117 + Addendum 2 §Corrections (L229) + Addendum 3 §Root-cause determination (L370) + Addendum 6 bottom line (L586) | Authoritative version is in Addenda 3/6; body's v1.x version is superseded in three specifics (below) |
| Crash details | ✅ PRESENT | `## Crashed Bead Details` L6–19 + `## Crash Context and Timeline` L21–41 + `## Signal Analysis` L43–50 + Addendum 2 three-phase & four-timestamp tables (L198–227) + Addendum 4 transcripts | Primary-source version is Addendum 2; body retains superseded single-crash framing |
| Resolution steps | ✅ PRESENT | `### Resolution Steps` L62–67 (under `## Deliverable Verification`) | 5 explicit steps; executor attribution corrected by Addendum 4 |
| Verification metrics | ✅ PRESENT | `### Final Verified Metrics` table L69–79 + `### Success Evidence` L86–90 + Addendum 1 (L155) + Addendum 3 (L390) + Addendum 4 (L495) + Addendum 5 (L520) | Before/after/target/result table, re-verified live four times after Aug-17 |

The gap-filling bead's work is **in-place harmonization of the body with the
addenda**, not adding sections. Every contradiction below is already *resolved
somewhere in the document* — the addenda correct the body correctly — but the
body text was never annotated, so a reader who stops at the body (or cites the
Summary) gets the pre-correction story.

## Contradictions and thin spots (all internal to this report)

### C1 — Summary says the gc "completed on the 53rd attempt"; Addendum 4 corrects it (P1)

- **Summary L4:** "…before the `git gc --aggressive --prune=now` operation
  **completed on the 53rd attempt** at 12:58:45Z."
- **Addendum 4 L425–436:** the 53rd attempt did *not* run the gc — it executed
  needle's auto-split (`SPLIT_COMPLETE`, children bf-173o7e / bf-5jhvpk /
  bf-im2sl1); "the gc completed under bf-173o7e," which recorded the final
  metrics. Addendum 4 explicitly names this Summary sentence as corrected.
- Addendum 5's restoration (L511) still repeats the "on the 53rd attempt"
  phrasing, so the correction never reached the Summary or Addendum 5's line.

### C2 — Resolution step 2 says "completed on retry"; same Addendum 4 correction (P1)

- **L64:** "Executed aggressive garbage collection … completed on retry."
- No bf-4x12ec attempt ever completed the gc (44 × exit -1, 8 × exit 124);
  the completion was under child bead bf-173o7e after the attempt-53
  auto-split (Addendum 4 L425–443).

### C3 — Body root-cause mechanism contradicted by Addendum 3's kernel evidence (P2)

- **L97:** "`git pack-objects` process consumed 3-6GB RAM per operation" — vs
  Addendum 3 L351: Aug-16 kernel-recorded git kills at **1.2–11.97 GB, mean
  10.14 GB**, 163/257 hugging the 11–12 GB ceiling.
- **L98:** "**Multiple concurrent git operations** exhausted available memory"
  — vs Addendum 3 L372: "**Cgroup** memory exhaustion, not system OOM"; one
  `git gc --aggressive --prune=now` per attempt against the dispatch scope's
  `MemoryMax=12GiB` (Addendum 3 L357–363). No concurrency evidence exists;
  each phase-1 transcript ends mid-single-gc (Addendum 4 L407–415).

### C4 — Signal Analysis states as "Definitive" what Addendum 2 downgrades to inference (P2)

- **L45–50:** "**Signal -1 = SIGKILL (Signal 9)** in Linux … Delivered by:
  Linux OOM killer" under the heading "Signal -1 Definitive Identification."
- **Addendum 2 L258–265:** `exit_code=-1` is the worker's sentinel for "child
  terminated without a wait status"; "It is not itself a POSIX signal."
  SIGKILL(9)-via-OOM is the *canonical inference*, not an identification.
  (L13 in Crashed Bead Details already hedges correctly — the section body
  does not.)

### C5 — "System State During Crash Period" cites the superseded corpus (P2)

- **L36–41:** secondhand figures with no Aug-14 primary source — "OOM killer
  active, <2GB available," "Load Average: 15-17," "Disk 84% full," and
  "**9 systematic crashes in 2.5 hours on bf-4yjq alone**."
- Addendum 2 L277–283 records that *no* kernel logs or memory telemetry
  survive for Aug-14 (current boot began 2026-08-15), so none of these
  contemporaneous numbers are verifiable; Addendum 3 L327–336 supplies the
  sourced version (direct telemetry: load 10.4–30.92, mean ~13.8).
- The `<2GB available ⇒ OOM` premise is expressly corrected at Addendum 2
  L244–255 (ample free RAM does not rule out a memcg kill).
- The bf-4yjq line cites exactly the count the corrected record supersedes:
  `docs/crash-analysis-bf-1s6c3-2026-09-06.md` gives bf-4yjq **50 kills**
  (17:54–20:30Z) that evening, and CLAUDE.md's crash section states the 2026-09-06
  record "supersedes the 2026-09-01 corpus's '9 crashes in 2.5 hours' count."
  No addendum to this report corrects L41.

### C6 — Loose-object "before" count unreconciled: 4,627 vs 4,649 (P3)

- **Body** says **4,627** (L73, L88, L124, L167 — "4,627 → 141").
- **Addendum 4** says **4,649** from primary sources: the phase-1 transcripts'
  `git count-objects` (`count: 4649`, L411) and bf-173o7e's recorded final
  metrics ("4,649 → 141", L433, L476).
- A 22-object gap between two same-day measurements is plausible (objects
  created between readings), but the report never reconciles them, and the
  body's number is the one a gap-filler would "correct" wrongly if it picked
  either unilaterally. Needs a dated note stating which figure came from
  which measurement, not a silent overwrite.

### Non-gaps (verified present, no action)

- **Pack-figure drift** (10,265 in 750.67 MiB at L74 vs 10,408 in 90.18 MiB in
  the Sep-2 readings) — expected drift from scheduled gc, already annotated in
  Addendum 1 L160–161.
- **"⚠️ close" target rows** (753MB vs <500MB; 141 vs <100) — subsequently
  satisfied (92MB, 20–43 loose objects), annotated in Addenda 1/3/5.
- **Migration conflation** — self-corrected in-body (`## Original Work
  Context` L52–56); the task was repository cleanup, not the bead-rs
  migration.

## Checklist for the gap-filling bead

All edits in place in `docs/crash-investigations/bf-4x12ec-crash-investigation.md`.
Body sections are historical record — **annotate, don't rewrite** (Addendum 5
L529–534 sets that precedent for this exact file).

- [ ] **P1 — Summary L4:** replace "completed on the 53rd attempt at 12:58:45Z"
      with the Addendum 4 phrasing ("the bead was decomposed on the 53rd
      attempt; the gc completed under child bf-173o7e") or append a
      "(see Addendum 4)" pointer.
- [ ] **P1 — Resolution step 2, L64:** correct "completed on retry" → executed
      under child bead bf-173o7e after the attempt-53 auto-split (Addendum 4).
- [ ] **P2 — Root Cause body L97–98:** annotate the 3–6GB / "multiple
      concurrent operations" claims as superseded by Addendum 3 (single gc per
      attempt; 12GiB memcg cap; 1.2–11.97 GB mean 10.14 GB kernel-recorded).
- [ ] **P2 — Signal Analysis L45–50:** soften "Definitive Identification" to
      the Addendum 2 framing (sentinel value; SIGKILL-via-OOM as inference);
      L13 already reads correctly.
- [ ] **P2 — System State L36–41:** mark the secondhand figures as
      unverifiable for Aug-14 per the Addendum 2 evidence-window limitation,
      point at Addendum 3's telemetry, and replace/annotate the "9 systematic
      crashes on bf-4yjq" line with the corrected 50-kill record
      (`docs/crash-analysis-bf-1s6c3-2026-09-06.md`).
- [ ] **P3 — Metrics L73/88/124/167:** add a dated reconciliation note for
      4,627 vs 4,649 (body count vs transcript/bf-173o7e measurement) rather
      than overwriting either.
- [ ] Out of scope, no action: resolution steps exist and match the corrected
      story once Addendum 4 is read; verification metrics are complete and
      were re-verified live four times post-cleanup; pack-figure drift and the
      ⚠️ target rows are already annotated.
