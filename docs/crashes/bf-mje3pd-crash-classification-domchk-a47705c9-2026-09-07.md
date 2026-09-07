# Crash Classification: bf-mje3pd — split step 2 of alert bead bf-3dxljn

**Classification bead:** domchk-a47705c9 (2026-09-07)
**Alert bead being split:** bf-3dxljn ("ALERT: Agent crash on bead bf-mje3pd", exit code −1,
created 2026-08-13T19:46:57Z)
**Evidence input:** child 1's compilation —
`docs/investigations/bf-mje3pd-evidence-compilation-domchk-916a1e66-2026-09-07.md`
(domchk-916a1e66, bfef974)
**Prior classification this record cross-checks:** domchk-bf8c4fd3 (63ca904) —
`docs/crashes/bf-mje3pd-crash-classification-domchk-bf8c4fd3-2026-09-07.md`
**Method:** both classifier scripts re-run first-hand against the crash record and their
output captured verbatim; verdict mapped through the `docs/crash-response-guide.md` exit-code
decision tree; consistency checked against the bf-mje3pd verification reports at HEAD
(f5e6377). No new log extraction — the first-hand census in 63ca904 is cited, not re-derived.

---

## Verdict

**INFRASTRUCTURE** — repository-bloat regime sub-type; mechanism **regime-matched, not
kernel-proven** (no surviving kernel record; journald floor is 2026-08-15). Crashes genuine
and mid-task — not a false positive *at the time* and not a domain-check code defect. The
alert's disposition *today* is **stale, not false**: bf-mje3pd closed 2026-08-17 (rev 2) with
its deliverable landed on the 14th dispatch (`verification.passed`), so bf-3dxljn warrants no
action. This is the same verdict 63ca904 recorded; this bead's contribution is the captured
classifier output and the consistency check below.

---

## Acceptance criterion 1 — classifier runs, output captured

### `scripts/crash-classifier.sh bf-mje3pd` — record-based classifier, cannot see this crash

```
$ ./scripts/crash-classifier.sh bf-mje3pd
ERROR: Bead trace not found: .beads/traces/bf-mje3pd/trace.jsonl
$ echo $?
2
```

Exit **2**, stderr line above, no classification emitted. Re-run live 2026-09-07 from the
repo cwd; identical output to child 1's first-hand run (domchk-916a1e66 §3), which recorded
the line but not the exit code — **exit 2 is new information from this dispatch**.

Why it cannot classify: the classifier reads only the per-bead trace, and per-bead traces are
single-slot — bf-mje3pd's is gone. The replacement sources start too late to help:
`.beads/logs/` earliest entries 2026-09-02, journald floor 2026-08-15 (single boot begins
2026-08-15 19:56:33 EDT). **No automated classifier can see a 2026-08-13 crash.** Per the
guide's own Phase 1 ("If classification is CODE_DEFECT or UNKNOWN, proceed with manual
investigation", `docs/crash-response-guide.md:196`) and the corpus capture-race rule
(`docs/crash-root-cause-domchk-4f0b8b43-2026-09-07.md` §4-C), a failed/absent automated
classification is **not** evidence that no crash occurred. Classification therefore comes
from the raw fleet worker log — which 63ca904 already did first-hand (281 events,
JSON-parsed, 14-dispatch table).

### `scripts/classify-signal-crash.sh` — state-based classifier, inadmissible here

```
$ ./scripts/classify-signal-crash.sh
/run/current-system/sw/bin/bash: line 7: ./scripts/classify-signal-crash.sh:
cannot execute: required file not found
$ bash scripts/classify-signal-crash.sh
=== Signal -1 Crash Classification ===
...
Repository Size: 105MB
Loose Objects: 303
Available Memory: 45262MB (70.7%)
Load Average (1min): 2.97 (24.8%)
CLASSIFICATION: LIKELY SIGHUP CASCADE (Signal 1)
$ echo $?
0
```

Three findings, all first-hand 2026-09-07:

1. **Not directly executable on this box** — its shebang is `#!/bin/bash` and NixOS has no
   `/bin/bash` (the repo's other scripts use `#!/usr/bin/env bash`). Run it as
   `bash scripts/classify-signal-crash.sh`. This is why the task's "or" alternative fails
   with a confusing exec error before any classification.
2. **It classifies the workspace's *current* state, not a bead's crash record** — its inputs
   are `du -sk .git`, `free -m`, `/proc/loadavg` at run time. It has no bead argument and no
   timestamp input, so its output carries **zero information about the 2026-08-13 crash**.
   On today's repaired repo (105 MB, healthy memory, normal load) it necessarily returns the
   healthy branch. Running it after a *future* bloat would return `OOM_SIGKILL` for the same
   crash — the verdict follows the clock, not the evidence.
3. **Its SIGHUP label is the framing the guide supersedes.** "LIKELY SIGHUP CASCADE
   (Signal 1)" is asserted from *absence* of bloat/memory/CPU pressure, with no signal
   evidence. Guide note 2 (`docs/crash-response-guide.md:28`): exit −1 is a sentinel, never
   assert a specific signal from it — a SIGHUP claim needs a 129 exit code or other signal
   evidence, which does not exist for this crash. **Do not cite this script's output as
   corroboration for any bf-mje3pd classification, in either direction.**

---

## Acceptance criterion 2 — matches the crash-response-guide for exit code −1

Mapping bf-mje3pd through the guide:

| Guide rule | Where | Application to bf-mje3pd |
|---|---|---|
| exit −1 → **Infrastructure event** | `docs/crash-response-guide.md:15`, `:205`, decision tree `:1024` | Applies — 7 of the bead's 12 classified outcomes are exit −1 |
| exit −1 + `.git` > 5GB → **Infrastructure: Repository bloat** (Pattern 3) | `:16` | Applies — `.git` was ~18 GB / ~17.16 GB loose on 2026-08-13 (repaired since) |
| "Work completed within 30s? → FALSE POSITIVE" branch | `:1026` | **Does not apply** — at bf-3dxljn's alert instant the bead was 4 attempts into a 14-attempt loop; the deliverable landed on attempt 14 (21:18:23Z, `verification.passed`). The two in-loop commits (ea23bd1, 164b62d) each preceded a *kill*, not a completion |
| exit −1 is a sentinel — never a specific signal | `:28` | Honored — the signal is not recoverable; "SIGHUP" and "SIGKILL asserted from −1" are both out of bounds |
| "Other → Code/task issue" (CODE_DEFECT) | `:22` | Not reachable — exit −1 routes to Infrastructure before this branch; no investigation of this crash ever found a domain-check defect |

So the guide yields exactly one classification for this record: **INFRASTRUCTURE**, refined to
the repository-bloat sub-type. That satisfies the acceptance criterion ("infrastructure event
or false positive, not a domain-check code defect") on the infrastructure leg.

---

## Acceptance criterion 3 — consistency with the existing verification report

Cross-checked `docs/verification/bf-mje3pd-crash-analysis.md` (domchk-9bc6579f, 81614ac
2026-09-02; blob since re-landed by the fleet tree-restore 2ec91ec) and, via child 1's
inventory, the earliest archived report
`docs/archive/crash-investigations/verification-report-bf-3za7vh-crash-analysis-bf-mje3pd-2026-08-26.md`
(624b2d2). Paths verified live at HEAD this dispatch.

**Concur — the classification is consistent across all four records:**

| Record | Verdict |
|---|---|
| `docs/crash-response-guide.md` exit-code tree | Infrastructure (repository-bloat sub-type) |
| domchk-bf8c4fd3 classification (63ca904) — authoritative | INFRASTRUCTURE, regime-matched |
| `docs/verification/bf-mje3pd-crash-analysis.md` (81614ac) | "Infrastructure crash - repository bloat → OOM (not false positive)" |
| archived bf-3za7vh report (624b2d2) | "NOT a simple false positive … persistent crashes with eventual success" |

**Mismatches found — all in the verification report's *supporting figures*, none in its
classification.** These are the corrections 63ca904 already established first-hand; this
dispatch re-verified each against the report text and confirms they stand:

| Verification report says | First-hand (63ca904) | Effect on classification |
|---|---|---|
| "Exit code -1 indicates SIGKILL (signal 9)" (line 97) | Signal not recoverable — journal floor 2026-08-15; −1 is a sentinel (`crash-response-guide.md:28`) | Strengthens the guide fit: the report asserts a signal the evidence cannot support |
| 9 × exit −1, 2 successes, **13** attempts | **7** × exit −1, 1 classified exit 0, **14** dispatches (attempt 10 = unclassified mitosis eval) | Counts only — 7 ≠ 9 changes nothing about the verdict |
| Crash duration 2h15m | 2h24m46s (18:53:50Z → 21:18:36Z); kill burst 43m22s | Counts only |
| "pack-objects consumed 3-6GB RAM" | Regime inference — no surviving kernel record for this bead | Downgrades mechanism to regime-matched, not kernel-proven |
| "592 commits ahead of origin/main" | Phantom divergence — describes the resolved pre-squash state | Counts only |
| "Add to crontab" for maintenance (line 228) | This box is NixOS — no crontab; superseded by systemd user timers (`setup-repo-maintenance.sh`) | Remediation advice only |

The archived bf-3za7vh report adds one claim 63ca904 likewise corrected: its figures predate
the first-hand census, and its "marked orphaned despite successful outcome" observation is
real but is alert-layer debris, not a second failure mode.

**No mismatch was found between this classification and child 1's evidence note**
(domchk-916a1e66). Its §3 classifier caveat is reproduced exactly by this dispatch's re-run,
and its §4 conclusion matches the verdict above. A dated addendum recording this
classification has been appended to that note, as the task requires.

---

## Recorded classification (the deliverable)

> **bf-mje3pd, 2026-08-13 — CLASSIFICATION: INFRASTRUCTURE** (repository-bloat regime
> sub-type; mechanism regime-matched, not kernel-proven — no surviving kernel record).
> Crashes genuine and mid-task (7 × exit −1 within a 14-dispatch loop, 18:53:50Z–21:18:36Z).
> Not a false positive at the time; not a domain-check code defect. No work lost —
> deliverable landed on the 14th dispatch with `verification.passed`; bead Closed 2026-08-17
> (rev 2). Alert disposition today: **stale** — bf-3dxljn (this split's parent) documents
> attempt 11's release heartbeat, not a 14th crash, and warrants no action. Prior art for the
> close reason: the bf-6d3d6 chain's *"crashes genuine / work never lost / alert stale."*

Automated classification was unavailable for this record for a structural reason (single-slot
trace gone; both log sources post-date the crash) — documented above with the captured output
rather than asserted. The classification is recorded here and in the child-1 evidence note's
dated addendum.

---

*Classified 2026-09-07 by domchk-a47705c9 against HEAD f5e6377. Classifier runs executed
first-hand from the repo cwd; exit codes captured directly (no pipeline); every cited guide
line number and doc path verified against HEAD this dispatch. Doc paths in this file that
live under `docs/archive/` are the frozen archive paths (a883044) — cite them as written.*
