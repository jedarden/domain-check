# bf-4x12ec — Classification and Retry-Safety Verdict

**Date:** 2026-09-08
**Bead:** `domchk-dba1e0bb` (split child 4 of 4, final) of umbrella `domchk-4adc1a55` ("Analyze root cause of signal -1 crash on bf-4x12ec")
**Status:** Verdict doc. Synthesizes children 1–3; adds no new primary evidence except where marked **re-verified this dispatch**. Drafted by a prior attempt of this bead and left uncommitted at its death (04:15Z), then re-verified first-hand, corrected where that snapshot had drifted, and committed by this attempt — see §7.
**Canonical record:** [`bf-4x12ec-crash-investigation.md`](bf-4x12ec-crash-investigation.md) — Addenda 7/8/9 carry children 1–3's findings. This is a separate file because that report's shared index holds a co-tenant's staged 407-line deletion and its deliverable addenda are already at HEAD; appending here would have collided with in-flight work it isn't mine to resolve.

---

## 1. Inputs

| Child | Bead | Status | Deliverable |
|---|---|---|---|
| 1 — evidence + signal −1 semantics | `domchk-15854355` | Closed | `bf-4x12ec-evidence-signal-semantics-domchk-15854355-2026-09-07.md` + report §5 (commit `76ad268`) |
| 2 — crash-window resources | `domchk-5f3ec6e1` | Closed | report Addendum 8 (commit `0a2dbb0`) |
| 3 — workload + reproducibility | `domchk-78d89c6b` | Closed | report Addendum 9 (commit `5cebf89`) |

All three commits are ancestors of HEAD; HEAD == origin/main (0/0 divergence) when this
dispatch started. The attempt census and every figure relied on below were re-derived
first-hand for this verdict rather than copied.

## 2. Classification method

`./scripts/crash-classifier.sh bf-4x12ec` → stdout
`ERROR: Bead trace not found: .beads/traces/bf-4x12ec/trace.jsonl`, **exit 2** ("missing
artifacts"). This is expected, not a tooling failure: `.beads/traces/<id>/` holds only the
most recent run of a bead id, and bf-4x12ec last ran 2026-08-14 — 25 days ago. The
automated path is therefore unavailable, and per the acceptance criteria the
[`docs/crash-response-guide.md`](../crash-response-guide.md) criteria are used instead.
(Consistent with the standing caveat that the classifier sees only `events.jsonl` crash
records, so an automated `UNKNOWN` would not have meant "no crash" either.)

## 3. Classification — **INFRASTRUCTURE** (sub-type: repository bloat)

One of the four permitted values: `INFRASTRUCTURE`.

The guide's classification table row for this crash matches on **all three** of its
columns, not just the exit code:

| Guide row | Guide expects | bf-4x12ec (re-derived this dispatch) |
|---|---|---|
| Exit code | **−1**, zero variation | 44 × exit −1, `0..0` variation across the storm |
| Pattern | **fixed-cadence re-dispatch deaths** | 10:23:02Z → 11:27:26Z, one kill per ~90–100 s (mean attempt lifetime 64,645 ms + ~26 s handling) |
| Repo state | **`.git` > 5 GB** | 17.20 GiB loose / 4,649 objects, `size-pack` 9.60 MiB — 36× the 500 MB healthy ceiling |

Mechanism, per child 2's resource verdict and child 3's workload verdict, agreeing with
the independent derivation in [`evidence/bf-4x12ec/system-state.md`](evidence/bf-4x12ec/system-state.md):

> `git gc --aggressive --prune=now` over **17.20 GiB of loose objects** × **no
> `pack.windowMemory`** (installed only 2026-09-02) × the dispatch scope's **12 GiB
> `MemoryMax`** → pack-objects RSS hit the cgroup ceiling → **memcg OOM SIGKILL** →
> needle recorded the signal death as its `exit −1` sentinel.

Causal chain of the classification, each step carried by a child:

- **Not CODE_DEFECT.** domain-check code is not in the causal path (child 3 §1): the
  killed operation is git plumbing, and the same kills reproduce with no application
  involved. The repo-wide finding — no domain-check defect in any investigation — holds.
- **Not SERVICE_FAILURE.** No HTTP 5xx, no gateway involvement; the deaths are
  synchronous with a local memory ceiling, not an external dependency.
- **Not FALSE_POSITIVE.** Child 3's pattern comparison closes this explicitly: guide
  Pattern 1 (post-completion) does **not** match — both surviving crash-era transcripts
  end at the *unanswered* `git gc` `tool_use`, and `count-objects` is byte-identical at
  10:21:23Z and 10:43:49Z, so no attempt ever completed the gc, let alone finished 30 s
  before a kill. FP Rule 2 ("crash → retry → success") is a **surface match only**:
  attempt 53's exit 0 was needle's auto-split (`SPLIT_COMPLETE`) changing the task shape,
  the guide's own bf-1s6c3 caveat, and the environment did not change on Aug-14 — the
  gc's own child bf-173o7e died the same way before succeeding. A persistent cause that
  outlives the retry loop is Infrastructure, not a transient.

**Layer separation (this is the sentence the umbrella should quote).** The crash is
INFRASTRUCTURE at the kill layer. The 44 one-alert-bead-per-kill artifacts minted during
the storm are the FALSE_POSITIVE layer — genuine kills, spurious *alerts* — and are
handled separately by the alert-layer fixes and the
[duplicate-alert-waves analysis](bf-4x12ec-duplicate-alert-waves.md). Calling the crash a
false positive because its alerts were is the error this split exists to prevent.

**Why `exit −1` says nothing about the signal.** Child 1's semantics finding (evidence
doc §1, with report Addendum 2 as the in-report carrier; the evidence doc's own §5 dated
addendum tracks the body-wording history):
`−1` is needle's writer-side sentinel for a worker dead with no recorded wait status
(`code().unwrap_or(-1)`). A SIGKILL death correctly encodes as 137, SIGHUP as 129. The
sentinel alone therefore identifies no signal; the memcg-OOM mechanism is established by
kernel records, not by the `-1`. The body's three over-claims ("53rd attempt" in the
Summary, "completed on retry" in Resolution step 2, "Signal -1 = SIGKILL" in Signal
Analysis) were still bare at v1.8 — **they are now annotated in place at HEAD v1.10/v1.11**
(softened heading + dated Correction block beneath the Signal Analysis claim; inline
"corrected 2026-09-08, bead domchk-8c78ae8b, from …" notes on the other two), so the body
is safe to cite again; the addenda remain the primary-source carriers.

## 4. Systemic or isolated?

**Systemic as a mechanism class during the repo-bloat era; isolated — in fact
defended — since 2026-09-02.** Neither half of that sentence is optional; dropping
either produces the wrong operational conclusion.

### 4.1 Systemic then — one mechanism, six beads, three days

The identical interaction (unbounded git pack operation × repository bloat × 12 GiB
dispatch scope) is documented across the Aug-12 → Aug-16 window:

| Bead | Variant | Scale |
|---|---|---|
| `bf-1s6c3` | gc | 76 dispatches / 71 kills, 2026-08-12 |
| `bf-4yjq` | gc | 50 kills, same evening |
| `bf-4x12ec` | gc | 44 kills, 2026-08-14 (this crash) |
| `bf-173o7e` | gc (bf-4x12ec's auto-split child) | 131 attempts / 129 kills, 2026-08-14 |
| `bf-1ea4g` | **push** | 57 attempts, closed 2026-08-13 |
| `bf-198ne` | **push** | 720-commit backlog, 2026-08-16 |

Kernel corroboration that this is one mechanism and not six coincidences — **re-verified
this dispatch:** all **257** `oom-kill: constraint=CONSTRAINT_MEMCG … task=git` records in
the journal fall on **Aug 16 alone**, every one inside a
`user@1001.service/*/run-p*.scope` transient dispatch scope — **220 `app.slice` / 37
`needle.slice`**, split from the records' own `task_memcg=` field. Both slices are
needle dispatch scopes of the same user session, not synthetic test scopes. That is the
same death, under the same constraint, in the same scope class, across unrelated beads —
the definition of a systemic configuration fault rather than a one-off. (No Aug-14 kernel
record survives; child 2 establishes the Aug-14 mechanism from the same-regime Aug-16
records plus the in-window host reading.)

### 4.2 Isolated now — the precondition and the amplifier are both gone

Also **re-verified first-hand this dispatch**:

| Guard | Live check | Result |
|---|---|---|
| Loose mass | `git count-objects -vH` | **332 objects / 2.30 MiB** (310 / 2.10 at the 04:15Z draft run — normal churn), 1 pack / 100.25 MiB, garbage 0 (was 4,649 / 17.20 GiB) |
| Repo size | `du -sh .git` | **107 MB** (106 at 04:15Z) |
| Memory bound | `./scripts/setup-git-gc-config.sh --verify` | exit 0 — `windowMemory=2g` / `deltaCacheSize=1g` / `threads=1`, all local scope → worst case **≈3072 MiB** against the 12 GiB scope |
| Death-command replay | `./scripts/test-gc-memory-bounds.sh` | **17/17, exit 0** — the exact `git gc --aggressive --prune=now` exits 0 under `MemoryMax=768M` (1/16 of the dispatch scope), pack-objects peak RSS **320,520 KB** (320,488 at 04:15Z) vs the >12 GiB the unbounded run consumed |
| Host headroom | `free -g`, `uptime` | 52 Gi available of 62; load 3.10 (2.01 at 04:15Z) |

The decisive systemic-vs-isolated discriminator is in the kernel record, and it is a
scope-name split:

- **Zero** memcg kills inside `needle.slice` since Sep 1 (`journalctl -k --since
  "2026-09-01" | grep CONSTRAINT_MEMCG | grep -c needle.slice` → **0**). The last
  needle-scope memcg kill of any task is **Aug 16 13:21:31**.
- The **50** September `task=git` memcg kills all sit inside *deliberately named*
  test/probe scopes, counted from the records' `task_memcg=` field this dispatch:
  `bf1s6c3-push-*` 17, `bf4yjq-crash-*` 17, `bf1s6c3-gc-*` 15 (historical-storm
  replays), `gcmb-bare-aggressive` 1 (the memory-bounds suite) = 50. Those are the guard
  suites proving the bound holds under artificially tiny ceilings. That is the defense
  working, not the disease recurring.

So: the *mechanism* is systemic and was the dominant crash cause of its era, but the
*configuration* that realized it no longer exists, and the live dispatch path has not
produced a memcg kill in three weeks. Reading §4.1 alone would argue for treating every
future gc as dangerous; reading §4.2 alone would erase the reason the guards exist. Both
are the finding.

## 5. Retry recommendation

**Two answers, because "can it retry" conflates them. Answer them separately.**

### 5.1 Safety — **GO.** A retry would not crash.

The crash was deterministic under the Aug-14 configuration and is not reproducible under
today's. Every precondition is measured, not inferred: the loose mass is ~10⁴× smaller,
the bound caps the worst case at ~3 GiB against a 12 GiB scope, and the precise death
command now exits 0 under a ceiling 16× tighter than the one that killed it. A re-run
would have nothing to pack anyway — the repo is already fully packed.

### 5.2 Necessity — **NO-GO. Do not re-dispatch bf-4x12ec.**

Not because it is unsafe, but because there is nothing to retry. **bf-4x12ec is Closed**
(rev 4, re-shown live this dispatch), and its deliverable — the aggressive gc — was completed by its auto-split child
**`bf-173o7e` (Closed)**, verified live this dispatch. No attempt in the 53-attempt storm
ever completed the gc; the eventual completion belongs to that child four days later.
This also corrects the v1.6 body's own claims: bf-4x12ec's notes say "Git cleanup
completed successfully despite agent crash" and its Resolution says "completed on retry"
— both wrong about *which* attempt finished it, both superseded by report Addendum 7's
completion verification.

Re-dispatching a closed bead whose work is done would re-run an already-satisfied
operation against an already-packed repository, and would mint a fresh wave of per-kill
alert beads if anything hiccupped — exactly the alert-layer failure mode this split
documented.

### 5.3 Conditions (binding if any gc is ever re-run here)

1. **Never bare `git gc --aggressive --prune=now`.** Use `./scripts/safe-git-gc.sh`
   (staged, checkpointed, memory-bounded, monitorable; `--check-only` for the verdict).
   Note the persistent `pack.windowMemory` config does bound the bare path too, but that
   is a backstop, not a license — the bare command has no checkpoint/resume, no staging,
   and no monitoring, and it is the command that killed 44 attempts here.
2. **Re-verify before relying on the bound:**
   `./scripts/setup-git-gc-config.sh --verify` must exit 0. The bound is repo-local +
   global config, so a fresh clone is unprotected until `setup-git-gc-config.sh` is run
   there.
3. **Detection is already wired** — `auto-gc-trigger.sh --dry-run` runs daily at 02:00.
   Re-evaluate this verdict if loose objects exceed ~1 GB or `.git` exceeds 500 MB
   *with* the bound verified absent.
4. **Never re-open bf-4x12ec.** New gc work gets a fresh bead. Re-opening would
   misrepresent a completed deliverable as outstanding.

## 6. What the umbrella can close on

`domchk-4adc1a55`'s acceptance criteria, each with its carrier:

| AC | Verdict | Carried by |
|---|---|---|
| What `signal -1` means | needle's died-without-exit-code sentinel, not a POSIX signal; mechanism established by kernel records | child 1, report §5 |
| Cause: OOM / timeout / signal / other | **memcg OOM** in the 12 GiB dispatch scope; the 8 × exit 124 are the separate 600 s cap, not the crash cause | child 2 (Addendum 8), child 3 (Addendum 9) |
| System resources at crash time | host healthy (45 Gi avail, 67 G disk); binding limit was the **scope cap**; load elevated but uncorrelated | child 2 (Addendum 8) |
| Agent-type behavior | not an agent/model behavior effect — deterministic config interaction; 44/44 identical kills | child 3 (Addendum 9) |
| Reproducible or one-time | deterministic under Aug-14 config, **not reproducible today** | child 3 (Addendum 9) + §4.2 here |
| Workload contribution | **yes, as necessary trigger** via condition interaction — not a workmanship defect | child 3 (Addendum 9) |
| Systemic or isolated | systemic mechanism class (6 beads, 257 kernel records), defended since 2026-09-02 | **§4 here** |
| Retry recommendation | safety GO / necessity NO-GO + 4 conditions | **§5 here** |

Residual, owned elsewhere and **not** owed by this umbrella: the open sibling
`domchk-0e428c71` ("Document crash findings and preventive recommendations"), which
predates the split and is not one of its four children (re-shown Open, rev 1, this
dispatch). The report body's three over-claims were the previous residual; they are now
annotated in place at HEAD v1.10/v1.11 (§3) and so no longer carry — the
[duplicate-alert-waves analysis](bf-4x12ec-duplicate-alert-waves.md) and the alert-layer
fixes remain the standing owners of that lesson.

---

## 7. Provenance and verification log

**Two attempts produced this file.** A prior attempt of `domchk-dba1e0bb` wrote §§1–6 and
died before committing; the deliverable survived only as an untracked worktree file
(mtime 2026-09-08 04:15Z). It is attributed to this bead by its filename, its bead-id
heading, and an mtime inside this bead's release window — `git log --all --grep
domchk-dba1e0bb` is empty, so there is no committed prior version to reconcile with.
This attempt re-verified every load-bearing claim first-hand before committing, and
corrected the places where the 04:15Z snapshot had drifted or over-stated:

- §3: the report body's three over-claims were "still uncorrected" only at v1.8; at HEAD
  v1.10/v1.11 they are annotated in place (re-grepped live), so the doc now says that
  instead of pointing later readers at bare superseded wording. The semantics citation
  now names its real carriers (evidence doc §1 + report Addendum 2), not a "report §5"
  that does not exist.
- §4.1: the 257 Aug-16 records are **not** all in `needle.slice` — the split is
  220 `app.slice` / 37 `needle.slice`, read from each record's `task_memcg=` field.
  The systemic argument (same death, same constraint, same scope class) is unchanged.
- §4.2: September scope families given with exact counts (the draft's `safe-git-gc-*`,
  `mw-oom*`, `probe-hog`, `run-isolated-*` examples are real September scopes for
  *other* tasks, but not for `task=git`).
- §4.2 table: churned live numbers refreshed (both runs agree within normal churn).

What this attempt actually ran (all live, ~05:05Z):

| Check | Result |
|---|---|
| `./scripts/crash-classifier.sh bf-4x12ec` | exit 2, "Bead trace not found" — matches §2; automated path unavailable |
| Children 1–3 | all Closed (rev 10 / 10 / 4); commits `76ad268`, `0a2dbb0`, `5cebf89` all ancestors of HEAD `159a6cc` |
| HEAD vs origin/main at dispatch start | identical (0/0) |
| Attempt census at HEAD | Summary + Addendum 2 table: 44 × `exit_code=-1` (38.9–115.8 s), 8 × `exit 124` (600.0 s), attempt 53 = auto-split — matches §3/§5.2 |
| Guide row | `crash-response-guide.md` L16: exit **-1** / fixed-cadence re-dispatch deaths / `.git` > 5GB → **Infrastructure: Repository bloat** |
| Kernel census | 307 `CONSTRAINT_MEMCG … task=git` oom-kills total: **257 on Aug 16** (220 app.slice / 37 needle.slice), **50 in September**, all in named test scopes (§4.2) |
| needle.slice since Sep 1 | **0** memcg kills; last needle-scope kill of any task Aug 16 13:21:31 |
| Repo state | 332 objects / 2.30 MiB loose, 1 pack 100.25 MiB, garbage 0, `.git` 107 MB |
| `./scripts/setup-git-gc-config.sh --verify` | exit 0 — worst case ≈3072 MiB vs the 12 GiB dispatch scope |
| `./scripts/test-gc-memory-bounds.sh` | **17 passed, 0 failed** — the exact death command exits 0 under `MemoryMax=768M`, pack-objects peak 320,520 KB |
| Bead states | `bf-4x12ec` Closed rev 4 (notes verbatim: "Git cleanup completed successfully despite agent crash" — the claim §5.2 corrects); `bf-173o7e` Closed rev 19; `domchk-0e428c71` Open rev 1 |
| Host | 52 Gi available of 62; load 3.10 |
