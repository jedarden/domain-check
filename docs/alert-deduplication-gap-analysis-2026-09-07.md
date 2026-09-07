# Alert Deduplication Gap Analysis

**Date:** 2026-09-07 · **Bead:** domchk-b5448b6a · **Scope:** why duplicate alerts
for already-resolved crashes still get generated, and what the deduplication
logic is missing.

Every claim below was verified live against this workspace on 2026-09-07
(reproduction commands in [Appendix A](#appendix-a--reproduction-commands)).
Line numbers refer to the working tree as of that date — note that
`scripts/crash-alert-manager.sh` carries **uncommitted** changes (the
bf-3561g system-event gate and bf-65lsdu circuit-breaker integration) from
another worker; HEAD-relative numbers differ slightly. None of those changes
are affected by the findings here.

---

## 1. Headline finding

**The in-repo deduplication pipeline has never once fired in production, and
it is not in the path that creates alerts.** Everything duplicate-shaped about
the current alert pool comes from that combination.

Two independent facts establish it:

1. **No production caller.** Nothing outside test/verify scripts invokes
   `crash-alert-manager.sh`. The only timer-driven piece of this stack,
   `domain-check-monitoring.timer`, runs `crash-pattern-detection.sh`, which
   *reports* — it creates, suppresses, and closes nothing.
2. **Every dedup ledger is empty while the alert pool is huge.** Across 296
   recorded invocations (19 distinct beads, all synthetic test replays —
   identical timestamps), the manager has generated **zero** alerts, and:

   | Ledger | State on 2026-09-07 | Written by |
   |---|---|---|
   | `.beads/logs/processed-alerts.txt` | **0 lines** | alert generation (fix 3) |
   | `.beads/state/crash-resolutions.json` | **0 records** | FALSE_POSITIVE auto-mark |
   | `.beads/logs/alert-state.json` | **absent** | alert generation |
   | `.alert-processed` trace markers | 12 of 1,876 trace slots | `--auto-process` |

Alert beads are created by the **needle harness** at kill time, upstream of
this repo. `docs/crash-prevention-requirements.md` already scopes
"alert dedup at source" to the NEEDLE repository (§ note at the mitigation
table). That boundary is real, but it does not excuse the state of the
in-repo gate: **the gate this repo owns is inert even when it is run**, for
the concrete reasons in §3.

### The scale of the duplicate problem (live census, 2026-09-07)

`bead list --json` (3,434 beads) filtered to agent-crash-titled alert beads:

- **1,714 alert-shaped beads**, of which **217 open / 72 in_progress** today.
- **176 open alert beads point at a target bead that is already CLOSED**
  (re-counted within the hour at 177 — the live store drifts as closure-bead
  workers act) — live duplicates of resolved issues, 216 of the 217 created
  ≥7 days ago.
- Concentration per target: bf-65lsdu **166** alerts (target closed),
  bf-173o7e **133** (closed), bf-1s6c3 **74** (closed), bf-4k2ws **68**
  (closed), bf-1ea4g **58** (closed), bf-4yjq **55** (closed),
  bf-4x12ec **47** (closed), bf-2ildm **41** (closed).
- Only bf-31mno (434) and bf-5vp (153) have open targets — those are
  retry-storms of an unresolved bead, a different (also undeduped) failure
  mode.

This matches the canon in `docs/crash-prevention-requirements.md`
("bf-173o7e alone generated 129 duplicate investigations") and the
bottom line in `CLAUDE.md`: most crash alerts today point at work another
worker already finished.

---

## 2. Where deduplication is *supposed* to happen

Four mechanisms in the manager, plus two satellite tools:

| # | Mechanism | Where | Keyed on |
|---|---|---|---|
| 1 | Closed-bead filter (fix 1) | `crash-alert-manager.sh:277-294` | the bead passed in |
| 2 | Target already processed (fix 2) | `crash-alert-manager.sh:298-319` | extracted target bead |
| 3 | This alert bead already processed (fix 3) | `crash-alert-manager.sh:322-326` | the bead passed in |
| 4 | Resolution status check | `crash-alert-manager.sh:260-273` → `crash-resolution-tracker.sh check` | the bead passed in |
| 5 | 5-minute cooldown | `crash-alert-manager.sh:409-422` | classification type only |
| 6 | `alert-deduplication.sh` (SERVICE_FAILURE only) | `crash-alert-manager.sh:394-406` | nothing — fleet-wide report |

What none of them is keyed on: **the target bead's live resolution state**.

---

## 3. Verified gaps

Ordered by how directly they explain "resolved crashes generate new alerts".

### D-1 — Resolution status is checked, but only against a ledger that nothing writes

`crash-alert-manager.sh:260-273` does consult resolution status before
alerting — the AC-3 question is technically "yes". But
`crash-resolution-tracker.sh check` (`:210-233`) reads **only**
`.beads/state/crash-resolutions.json`. The live evaluation functions
(`check_bead_closure`, `check_task_completion`, `check_repository_health`)
are reachable solely through `determine_resolution_type`, which is called
only from `mark-resolved`. The `check` action never looks at the bead store.

Verified: `./scripts/crash-resolution-tracker.sh bf-173o7e check` →
**NOT_RESOLVED (exit 1)** for a bead whose `Status:` is `Closed`.

And the ledger it does read has **0 records** (§1), because its only writer
is the FALSE_POSITIVE branch that D-3 shows is unreachable. A resolution
memory that is never populated, consulted instead of the live store, cannot
suppress anything.

**Fix:** make `check` perform the live evaluation (bead closure via
`bead show`, plus the `.beads/state/work-completion/<id>.json` markers that
`verify-work-completion.sh` already writes — this is gap **G-9** in
`docs/crash-prevention-requirements.md`). Keep the ledger as a cache, not as
the source of truth.

### D-2 — Dedup is keyed on the alert-bead instance, never on the crash

Even when the pipeline works, it records/suppresses by the ID it was handed.
Duplicates arrive as **fresh alert beads** for the same underlying crash, so
an instance-keyed ledger is structurally blind to them:

- fix 3 (`:322`) — same alert bead processed twice → matches, but that is not
  the duplicate shape the fleet produces.
- fix 2 (`:303-315`) — extracts the target and checks it, which *is* the right
  key, but see D-4: extraction never succeeds.
- resolution ledger — `mark-resolved` writes under the passed-in bead ID. A
  crash resolved once is "resolved" only under that one ID.

**Fix:** key all dedup state on `(target_bead_id, crash_fingerprint)` where
the fingerprint comes from `.beads/events.jsonl` (event window + exit code),
the authoritative crash record that `crash-classifier.sh` already parses.
Never key on the alert-bead instance alone.

### D-3 — The classification variable is the classifier's banner line

`crash-alert-manager.sh:376` takes `head -1` of the classifier's stdout.
`crash-classifier.sh main()` (`:318-320`) prints `=====` banner lines
**first**, so `CLASSIFICATION` is literally `=============================`.

Consequences, all silent:

- `[[ "$CLASSIFICATION" == "FALSE_POSITIVE" ]]` (`:380`) — never true →
  `mark-resolved` never runs → the ledger stays empty (feeds D-1).
- `[[ "$CLASSIFICATION" == "SERVICE_FAILURE" ]]` (`:394`) — never true → the
  only call to `alert-deduplication.sh` is unreachable.
- The cooldown and `alert-state.json` record the banner as the classification.

The manager's own log proves it:
`[INFO] Classification: ==================================`.

**Fix:** match the classification with
`grep -m1 -E '^(FALSE_POSITIVE|SERVICE_FAILURE|INFRASTRUCTURE|CODE_DEFECT|UNKNOWN)$'`,
or better, give the classifier a machine-readable mode (`--format=json` or
classification-on-first-line) and have callers consume that.

### D-4 — Target extraction never matches the real alert titles

`crash-alert-manager.sh:307` requires the title to match
`[Aa]gent[[:space:]-][Cc]rash[[:space:]-][Oo]n[[:space:]-]bf-…` — exactly one
space/hyphen between the words. The actual needle title is
**`ALERT: Agent crash on bead bf-XXXX`**; the word `bead` breaks the match.
Verified against real titles:

```
NO MATCH: ALERT: Agent crash on bead bf-22mb      (the real shape)
MATCH   : ALERT: Agent crash on bf-22mb           (assumed shape)
NO MATCH: Investigate agent crash on bead bf-4yjq
```

Worse, the gate at `:303` only attempts extraction when the *alert bead's own
ID* matches `^bf-[a-z0-9]+$`. Current-generation alert beads are `domchk-*`,
so for the entire September fleet, `TARGET_BEAD_ID` is empty and the
per-target dedup check (`:315`) is skipped before the regex even gets a
chance.

**Fix:** extract the target from the bead's dependency/ref edges first
(titles are the least reliable carrier — the canon's own "near-identical
titles" false-positive trap), falling back to
`(agent[ -]crash[^a-z0-9]+on[^a-z0-9]+(bead[ ]+)?)(bf-|domchk-)[a-z0-9]+`,
and drop the `^bf-` prefix gate on the alert-bead ID entirely.

### D-5 — The exit-code guards parse a JSON shape the traces don't use

`crash-alert-manager.sh:283` and `:337` extract with
`grep -o '"exit_code":[0-9-]*'` — compact JSON only. Census of all 1,856
trace metadata files: **1,850 (99.7%) are pretty-printed**
(`"exit_code": -1`, with a space); 6 are compact. Sampling 400 real traces,
the manager's parse returned empty for **all of them**, while the
classifier's own space-tolerant `grep -oP '"exit_code":\s*-?\d+'` (`:69`)
returns the value.

Consequence: fix 4's "exit code 0 = successful completion, not a crash"
guard (`:339`) is inert for 99.7% of traces — a post-retry *success* run
sitting in the single-slot trace dir can still be alerted as a crash. Fix 1
(`:285`) survives only because both branches exit 0.

(The uncommitted bf-65lsdu block at `:350-353` added a space-tolerant
fallback **for the circuit breaker only** — the same one-line fix applied
there proves how small the repair is, but fix 1/fix 4 still lack it.)

**Fix:** one shared, space-tolerant extractor (or `jq -r '.exit_code'`) used
by every consumer of `metadata.json`.

### D-6 — `alert-deduplication.sh` is a fleet-wide report wired in as a per-alert gate

This is the file the task names, and it is the weakest link:

- **It cannot suppress anything on its own.** It takes no bead ID, prints
  prose, and always exits 0. The manager greps its stdout for
  `duplicate|same.*pattern` (`:399`) — string-matching a report as if it
  were a verdict.
- **The grep is inverted.** The "all clear" line
  `✅ No duplicate alert patterns detected` contains the word *duplicate*, so
  it matches: the manager concludes "Duplicate service failure detected" and
  suppresses a genuine alert **precisely when no duplicate exists**.
  Verified live.
- **The ≥3-per-bead branch is unreachable.** `bead_crash_counts` is
  incremented once per `trace.jsonl` file (`:39-43`), and the traces layout
  is one file per bead dir (1,824 files / 1,876 dirs) — the count is
  structurally 1, never 3. (Single-slot traces are also the wrong source for
  crash *history*; `.beads/events.jsonl` is the append-only record.)
- **The ≥5 signature branch flags the wrong thing.** The signature is
  `date-of-first-line : unique-exit-codes` (`:46-51`). Verified in a
  sandbox: **six different beads crashing once each on the same day with the
  same exit code → "REPEATING CRASH SIGNATURE: 2026-09-07:-1 (6
  occurrences)"** — a fleet-wide coincidence, not a duplicate. Meanwhile a
  real duplicate (same unresolved bead re-alerted across days) never shares a
  signature, because the date differs.
- **CWD-dependent.** It resolves `.beads` relative to the *caller's*
  directory (`:8`), unlike every other script here, which derive
  `PROJECT_ROOT` from `BASH_SOURCE`. Invoked from anywhere but the repo root
  it prints "No traces directory found" — which does not match the manager's
  grep, so the alert then proceeds. Dedup effectiveness depends on the
  invoking shell's CWD.

**Fix:** give it a real contract — `alert-deduplication.sh check <bead-id>`
→ exit 0 (dedup) / 1 (no dup) keyed on target + fingerprint over
`events.jsonl` in a window; drop stdout prose-matching entirely; derive
paths from `BASH_SOURCE`.

### D-7 — The cooldown suppresses without recording, and is keyed on classification only

`crash-alert-manager.sh:409-422` drops the alert entirely (exit 0, "no alert
needed") when another alert of the same *classification type* fired within
300 s — and writes nothing, so the suppressed crash is not deferred, queued,
or logged as pending. The information is destroyed, and the next occurrence
re-attempts from scratch. Since `alert-state.json` doesn't exist (§1), this
has never fired — but if D-3 is fixed and alerts start flowing, this becomes
live immediately.

**Fix:** record suppressed alerts in a pending/coalesced queue (the
uncommitted system-event-gate block already builds exactly this pattern with
`system-event-suppressions.jsonl` — reuse it), and include bead ID in the
cooldown key.

### D-8 — The resolution tracker's own checks have parse bugs

- `check_repository_health` (`:153-156`) greps `git count-objects -vH |
  grep size | awk '{print $3}'` — with `-vH` that is the **unit token**
  (`MiB`/`bytes`), not the value, so the `*"garbage"*` test compares unit
  strings. It returns "healthy" today only because the `du -s .git` half of
  the condition passes; the loose-object test is inert.
- `check_task_completion` (`:99`) greps the trace for
  `work.*complete|task.*done` — the same over-broad string match the
  classifier's trace-provenance work (bf-3561g) already showed produces
  FALSE_POSITIVE verdicts from unrelated runs' text.

**Fix:** parse `count-objects --verbose` (no `-H`) with `jq`/awk on the
key names; gate completion heuristics behind the provenance check the
classifier already implements.

### D-9 — Nothing downstream consumes the signals that ARE produced

`crash-pattern-detection.sh` (the one timer-driven script) does detect
per-bead repeat crashes (`:285-290`, ≥3 in window → "DUPLICATE ALERT
PATTERN") but only prints to `crash-monitor.log`, conflates *crashes* with
*alerts*, and has no consumer. The manager's exit 1 ("alert generated") has
no consumer either. Detection without an acting agent is indistinguishable
from no detection.

### D-10 — Resolution expiry re-arms old resolved crashes

`crash-resolution-tracker.sh:25` expires resolution records after 30 days
(`RESOLUTION_AGE_DAYS`), after which `check` reports NOT_RESOLVED again. For
a *resolved-by-closure* bead that is wrong permanently — closure does not
stale out. Expiry is only meaningful for environment-level resolutions
("repo was bloated, now it isn't").

---

## 4. Direct answers to the task's questions

**Why can resolved crashes generate new alerts?** Four stacked reasons, in
causal order: (1) the only gate in the alert-creation path belongs to needle,
upstream, and consults none of this repo's state; (2) the in-repo pipeline,
when run by hand, cannot classify an alert bead at all (no trace slot exists
for it → classifier exit 2 → manager exit 2) and cannot find the target bead
in a crashed worker's record either (D-4); (3) the resolution check consults
a 0-record ledger instead of the live bead store (D-1, D-3); (4) the
deduplication script produces fleet-wide coincidences, not per-target
duplicate verdicts, and its one wiring in is both unreachable and inverted
(D-6).

**Is resolution status checked before alert generation?** Yes —
`crash-alert-manager.sh:260-273` — but the check is a no-op in practice: it
reads a ledger nothing has ever written (0 records), it never evaluates live
bead state on the `check` path, and it is keyed to the passed-in bead rather
than the crash's target. The live closure check that *does* work (fix 1) is
likewise keyed to the wrong bead for alert-shaped inputs.

---

## 5. Recommended improvements

Priority order; P1 items are the ones that change the duplicate rate.

1. **P1 — Put a working gate in the real path.** Since needle owns bead
   creation, the in-repo levers are: (a) a **post-creation triage pass** —
   the manager re-targeted at *alert beads*, whose first two checks are "is
   the target bead closed?" and "does a work-completion marker exist for the
   target?" (G-9), emitting close/defer verdicts for a human or closure-bead
   blocker; and (b) a single `dedup-check <bead-id>` subcommand other
   tooling can call before starting investigation work. Both key on the
   target bead (D-2).
2. **P1 — Key all dedup state on `(target_bead_id, crash_fingerprint)`** from
   `.beads/events.jsonl`, not on alert-bead instances (D-2, D-6).
3. **P2 — Make the resolution check live**: `check` runs the closure +
   work-completion evaluation directly; the JSONL ledger becomes a cache
   (D-1). Drop expiry for closure-based resolutions (D-10).
4. **P2 — Fix the classification extraction** (D-3) and **unify exit-code
   parsing** (D-5) — both are one-line fixes that un-break the FALSE_POSITIVE
   and SERVICE_FAILURE paths, i.e. they make the existing design function.
5. **P2 — Give `alert-deduplication.sh` a real contract** (D-6): arguments,
   exit codes, target-keyed windowed analysis over `events.jsonl`,
   `BASH_SOURCE`-derived paths. Stop grepping its prose.
6. **P3 — Fix target extraction** (D-4): dependency/ref edges first, tolerant
   title regex second, no `^bf-` gate.
7. **P3 — Fix the title/ID substring greps** in the processed-alerts ledger
   (`grep -q "$TARGET_BEAD_ID"` matches `bf-31p3g` inside longer IDs); use
   exact-field matching and add a TTL.
8. **P3 — Record suppressed alerts** (D-7) so cooldown/gate suppression is
   coalescing, not loss.
9. **P4 — Sweep the existing backlog**: a report-only pass over the ~177 open
   alerts whose targets are closed. Closure itself should stay with each
   alert's closure-bead blocker (the standing convention), so this is a
   queue for those blockers, not an auto-close.
10. **P4 — Fix the resolution tracker's parse bugs** (D-8) so its verdicts
    mean what they say.

---

## Appendix A — Reproduction commands

```bash
# D-1: resolution check ignores live closure
bead show bf-173o7e | grep -i '^Status'          # -> Status: Closed
./scripts/crash-resolution-tracker.sh bf-173o7e check   # -> NOT_RESOLVED, exit 1

# §1: empty ledgers vs alert pool
wc -l .beads/logs/processed-alerts.txt
python3 -c "import json;print(len(json.load(open('.beads/state/crash-resolutions.json'))['resolutions']))"
ls .beads/logs/alert-state.json 2>&1
bead list --limit 20000 --json | python3 -c "  # count open alerts w/ closed targets
import json,sys,re
rows=[json.loads(l) for l in sys.stdin if l.strip()]
by={b['id']:b for b in rows}
pat=re.compile(r'agent[\s-]*crash',re.I); tgt=re.compile(r'(?:bf-|domchk-)[a-z0-9]+')
n=0
for b in rows:
    if b['status']=='open' and pat.search(b['title']):
        m=tgt.findall(b['title'])
        if m and m[-1] in by and by[m[-1]]['status']=='closed': n+=1
print(n)"                                              # -> ~177 (drifts as closures land)

# D-4: title regex vs real titles
[[ "ALERT: Agent crash on bead bf-22mb" =~ [Aa]gent[[:space:]-][Cc]rash[[:space:]-][Oo]n[[:space:]-]bf-[a-z0-9]+ ]] \
  && echo MATCH || echo NO-MATCH

# D-5: exit-code parse census
for f in .beads/traces/*/metadata.json; do
  grep -oq '"exit_code":[0-9-]' "$f" && echo compact || echo pretty
done | sort | uniq -c

# D-3: classification is the banner
grep "Classification:" .beads/logs/crash-alert-manager.log | tail -1

# D-6: six one-crash beads on one day -> REPEATING CRASH SIGNATURE
S=$(mktemp -d); cd "$S"; mkdir -p .beads/traces/b{1..6}
for i in 1 2 3 4 5 6; do printf '{"timestamp":"2026-09-07T10:0%d:00Z"}\n{"exit_code":-1}\n' $i > .beads/traces/b$i/trace.jsonl; done
/home/coding/domain-check/scripts/alert-deduplication.sh | grep SIGNATURE
cd /tmp && rm -rf "$S"
```
