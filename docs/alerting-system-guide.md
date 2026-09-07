# Alerting System Guide

**Purpose:** reference for the crash-alerting stack in `scripts/` — architecture,
the duplicate-alert defenses, day-to-day usage, testing, and troubleshooting.
**Written:** 2026-09-07 · **Bead:** domchk-33565a46 · **Verified live at HEAD `48aafce`** (every
command in §7 was run this session; outputs quoted are this session's).

Related: [alert deduplication gap analysis](alert-deduplication-gap-analysis-2026-09-07.md)
(the D-1..D-10 findings this guide's dedup layer implements) ·
[crash response guide](crash-response-guide.md) (investigation workflow) ·
[crash alert fix implementation 2026-09-02](crash-alert-fix-implementation-2026-09-02.md)
(the original manager/classifier fixes).

---

## 1. Where alerts come from (and what this repo owns)

Alert beads are created by the **needle harness** at kill time, upstream of this
repo — the title shape is `ALERT: Agent crash on bead bf-XXXX`, and September-era
alert bead IDs are `domchk-*`. Nothing in this repo creates or suppresses those
beads (see the repo-boundary note in the gap analysis, §5 P1). What this repo
owns is everything *after* creation:

- **classify** what kind of failure it was (`crash-classifier.sh`)
- **decide whether the crash is already resolved** (`crash-resolution-tracker.sh`)
- **decide whether the alert is a duplicate of work already covered**
  (`alert-deduplication.sh` — the gate)
- **feed an investigation verdict** to whoever acts (an agent, a closure-bead
  blocker, a human)

The scale of the problem this addresses, measured live on 2026-09-07: of 1,714
alert-shaped beads, **176 open alerts point at a crash target that is already
CLOSED** (re-confirmed this session) — each one is investigation work somebody
could waste. bf-65lsdu has 166 alerts against a closed target, bf-173o7e 133,
bf-1s6c3 74 (gap analysis §1 has the full concentration table).

**Wiring status (be honest with yourself here):** no systemd timer and no
production caller invokes any of these scripts today. The only timer-driven
script in this space, `crash-pattern-detection.sh` (`domain-check-monitoring.timer`,
every 10 min), **reports** — it creates and suppresses nothing. The dedup gate is
a tool you invoke (§6); putting it in an automated path is open work
(gap analysis §5, P1).

---

## 2. Architecture

```
 needle harness (upstream, owns creation)
   └─ creates ALERT bead "ALERT: Agent crash on bead X"
        │
        ▼
 ┌─ this repo ─────────────────────────────────────────────────────────┐
 │                                                                     │
 │  alert-deduplication.sh check <alert-bead>      ← THE GATE          │
 │    ├─ extracts the crash TARGET from the bead title                 │
 │    ├─ 1. VERIFIED work-completion marker for the target?  → dup     │
 │    ├─ 2. crash-resolution-tracker.sh <target> check       → dup     │
 │    └─ 3. another open alert already covers the target?   → dup     │
 │         none of the above                                 → proceed │
 │                                                                     │
 │  crash-resolution-tracker.sh <target> check     ← RESOLUTION AUTH   │
 │    ├─ 1. live bead closure (bead show)                              │
 │    ├─ 2. VERIFIED work-completion marker                            │
 │    └─ 3. crash-resolutions.json (cache — manual marks + auto-marks) │
 │                                                                     │
 │  crash-classifier.sh <bead>                     ← CLASSIFIER        │
 │  crash-alert-manager.sh <bead>                  ← LEGACY PIPELINE   │
 │       (wraps classifier + tracker; §5 for what still works)         │
 └─────────────────────────────────────────────────────────────────────┘
```

### Components

| Script | Role | Contract |
|---|---|---|
| `scripts/alert-deduplication.sh` | per-alert duplicate gate | `check <bead-id>` → exit 0/1/2/3 (§3) |
| `scripts/crash-resolution-tracker.sh` | single authority for "is this crash resolved" | `check` → 0 resolved / 1 not / 2 error (§4) |
| `scripts/crash-classifier.sh` | crash categorization | prints FALSE_POSITIVE / SERVICE_FAILURE / INFRASTRUCTURE / CODE_DEFECT / UNKNOWN |
| `scripts/crash-alert-manager.sh` | 2026-09-02 pipeline wrapping the above | exit 0 no alert / 1 alert / 2 classification failed / 3 error (§5) |
| `scripts/test-alert-dedup-check.sh` | hermetic suite for the gate + tracker | 41 assertions, exit 0/1 |
| `scripts/test-crash-alert-fixes.sh` | suite for the 2026-09-02 manager fixes | 12 assertions, exit 0/1 |

### State and ledgers — what is truth and what is cache

| Path | Written by | Status |
|---|---|---|
| `.beads/state/work-completion/<id>.json` | `verify-work-completion.sh` (pre-close gate) | **source of truth** for "the task held up" (`result`: VERIFIED \| FAILED) |
| bead store (`.beads/beads.db`) | `bead` CLI | **source of truth** for closure — `check` reads it live on every run |
| `.beads/state/crash-resolutions.json` | tracker's `mark-resolved` + live-evaluation backfill | **cache only** — manual marks and FALSE_POSITIVE auto-marks; non-closure records expire after 30 days |
| `.beads/logs/processed-alerts.txt` | manager fix 3 | instance-keyed ledger (superseded by target-keyed dedup) |
| `.beads/logs/alert-state.json` | manager fix 5 | cooldown state, keyed on classification |
| `.beads/events.jsonl` | needle (append-only crash record) | **source of truth** for crash *history*; `report` reads this |
| `.beads/logs/alert-deduplication.log`, `crash-resolution-tracker.log`, `crash-alert-manager.log` | their scripts | audit trails |

The cache-vs-truth split is deliberate and is the D-1 fix: before 2026-09-07 the
tracker's `check` read **only** `crash-resolutions.json`, a ledger whose only
writer was an unreachable FALSE_POSITIVE branch — it held 0 records and reported
NOT_RESOLVED for beads whose `Status:` was `Closed`.

---

## 3. The dedup gate — `alert-deduplication.sh check <bead-id>`

Shipped 2026-09-07 (`48aafce`, domchk-b3de301f), implementing gap-analysis
D-1/D-2/D-4/D-5/D-6/D-8/D-10.

### Exit contract

| Exit | Verdict | Meaning |
|---|---|---|
| **0** | DUPLICATE | suppress — target resolved, or an open alert already covers it |
| **1** | UNIQUE | legitimate alert — proceed |
| **2** | USAGE | bad arguments |
| **3** | INDETERMINATE | bead store unreadable, or the bead is not in it — **fail open** |

Fail-open is a design decision, not an accident: crash alerting must never depend
on this gate being runnable. An unreadable store suppresses nothing and blocks
nothing — exit 3 means "proceed, and know you're proceeding unverified".

### How the target is derived (D-2/D-4)

Every verdict is keyed on the **crash target**, never on the alert-bead
instance. Duplicates arrive as fresh alert beads for the same target, so an
instance-keyed ledger is structurally blind to them. The target comes from the
bead title — `ALERT: Agent crash on bead bf-XXXX` and
`Investigate agent crash on bead X` both resolve; `domchk-*` alert beads are
accepted (there is no `^bf-` gate on the alert bead's own ID). Dependency edges
are deliberately *not* used: in this workspace they are prerequisite "blocker"
edges, not target pointers.

### Verdict order inside `check`

1. **VERIFIED work-completion marker for the target** (`G-9`) — local and cheap;
   a marker the pre-close gate wrote means the task held up, so the crash is
   post-completion and the alert is a duplicate.
2. **Resolution tracker live check** on the target — closure, then marker, then
   cache (§4).
3. **Open-alert coverage** — any *other* open/in-progress alert bead aimed at
   the same target: the fan-out shape that produced the 176-alert backlog.
4. Otherwise **PROCEED** (exit 1).

### `report` mode (fleet-wide)

`alert-deduplication.sh report [--window-hours N]` (default 24) reads
`.beads/events.jsonl` — the append-only crash record, not the single-slot traces
(one file per bead dir, overwritten every dispatch, so they are the wrong source
for history). Line discipline, which is the D-6 fix and matters if you grep its
output: lines beginning `DUPLICATE` are **per-target repeats only**; fleet-wide
coincidences are labeled `FLEET-WIDE CRASH OBSERVATION` and never use the word
"duplicate". The previous version's all-clear line contained "duplicate", so a
consumer grepping for it suppressed genuine alerts precisely when no duplicate
existed.

Environment knobs: `BEAD_SCAN_LIMIT` (bead list --limit, default 999999),
`REPORT_WINDOW_HOURS`.

---

## 4. Resolution authority — `crash-resolution-tracker.sh`

`<target> check` evaluates **live state first**, then the cache:

1. **Bead closure** via the bead store — closure never expires (D-10: expiry is
   meaningful for environment-level resolutions like "repo was bloated, now it
   isn't", not for "the bead was closed"). A cached closure cannot suppress a
   target that the live store says is open again — reopened-for-another-pass is
   a shape this fleet produces. Only an unreadable store defers to the last
   observed closure.
2. **VERIFIED work-completion marker** — `result: FAILED` vetoes a cached
   `task_completion` record: the cache entry was backfilled from a then-VERIFIED
   marker, and a live negative signal must outrank a stale derived one.
3. **The cache** — manual `mark-resolved` records and auto-backfilled closures;
   non-closure records expire after `RESOLUTION_AGE_DAYS` (30).

Actions: `check` (default), `mark-resolved` (auto-detects resolution type),
`mark-unresolved`, `show-state`, `list-resolved`, `cleanup`.

`mark-resolved`'s auto-detection also consults the bead's trace
(exit code 0, completion indicators, commit-before-crash within 30 s) and — for
OOM-shaped traces — repository health (`git count-objects -v` parsed by key name,
the D-8 fix; the old `-vH` awk field grabbed the unit token, making the
loose-object test inert).

---

## 5. The 2026-09-02 manager pipeline — what still works

`crash-alert-manager.sh` wraps the classifier and the tracker behind the six
"critical fixes" documented in
[crash-alert-fix-implementation-2026-09-02.md](crash-alert-fix-implementation-2026-09-02.md):
closed-bead filtering, duplicate detection, processed-alert tracking, exit-code
validation, completion awareness, and a 5-minute cooldown keyed on
classification. `test-crash-alert-fixes.sh` (12 assertions) covers them and
passes at HEAD.

Status at HEAD `48aafce`, per the gap analysis (do not trust the section's prose
unqualified — the doc above predates the findings):

- **Inert — D-3:** the manager takes `head -1` of the classifier's stdout as the
  classification, and the classifier prints its `=====` banner first. So
  `CLASSIFICATION` is literally `=============================` and the
  `FALSE_POSITIVE` / `SERVICE_FAILURE` branches never fire (verified in HEAD
  source this session; the manager's own log shows
  `Classification: ==================================`).
- **Inert for 99.7% of traces — D-5 (manager side):** its exit-code extraction
  greps compact JSON (`"exit_code":-1`) while 1,850 of 1,856 trace metadata
  files are pretty-printed. (The *tracker's* copy of this bug is fixed; the
  manager's remains.)
- **D-7:** the cooldown drops a suppressed alert without recording it anywhere —
  coalescing that loses the event.
- **D-9:** nothing consumes the manager's outputs, and no production caller
  invokes the manager at all (§1).

Also: the working tree carries **uncommitted sibling work** in
`crash-alert-manager.sh` / `crash-classifier.sh` — the bf-3561g system-event
gate, the bf-65lsdu circuit breaker, and domchk-0c601026's monitor windows.
`48aafce`'s provenance note assigns those to their owning beads. This guide
describes HEAD; do not cite working-tree behavior of those two scripts.

---

## 6. Usage guide

### You were dispatched to an ALERT bead (the common case)

Before writing a single line of investigation, ask the gate whether the crash is
already handled:

```bash
./scripts/alert-deduplication.sh check <alert-bead-id>
```

- **exit 0 (DUPLICATE)** — read the verdict line: it names the target and the
  reason (resolved / VERIFIED marker / covered by open alerts). Update the bead
  and close it as a duplicate instead of investigating. Leave final closure of
  the *alert* to its own closure-bead blocker if one exists (standing
  convention).
- **exit 1 (UNIQUE)** — proceed with the crash response guide.
- **exit 3 (INDETERMINATE)** — proceed, but you are unverified: the store was
  unreadable or the bead unknown. Fix the store issue and re-run if it matters.

To find the resolution evidence yourself (the gate's step 2, unwrapped):

```bash
./scripts/crash-resolution-tracker.sh <target-bead-id> check   # 0 = resolved
bead show <target-bead-id> | grep -i '^Status'                 # closure, live
cat .beads/state/work-completion/<target-bead-id>.json         # VERIFIED/FAILED
```

### Processing / reporting

```bash
# Classify a crash (works best on a bead that has a trace slot)
./scripts/crash-classifier.sh <bead-id>

# Legacy pipeline — see §5 for which branches are inert at HEAD
./scripts/crash-alert-manager.sh <bead-id>            # process one
./scripts/crash-alert-manager.sh --auto-process       # recent traces, last hour
./scripts/crash-alert-manager.sh <bead-id> --force-alert   # bypass dedup + cooldown

# Fleet-wide repeat-crash report (24 h default)
./scripts/alert-deduplication.sh report
./scripts/alert-deduplication.sh report --window-hours 72
```

### Recording resolution state by hand

```bash
./scripts/crash-resolution-tracker.sh <bead-id> mark-resolved     # auto-detects type
./scripts/crash-resolution-tracker.sh <bead-id> mark-unresolved
./scripts/crash-resolution-tracker.sh list-resolved
./scripts/crash-resolution-tracker.sh <bead-id> show-state
./scripts/crash-resolution-tracker.sh cleanup                     # drop expired records
```

`mark-resolved` writes the cache; it never overrides the live store — a closed
bead is resolved because the store says so, not because of the cache entry.

### Audit trails

```bash
tail -f .beads/logs/alert-deduplication.log      # every check verdict, with reasons
tail -f .beads/logs/crash-resolution-tracker.log
tail -f .beads/logs/crash-alert-manager.log
```

---

## 7. Testing and verification

### Suites

```bash
bash scripts/test-alert-dedup-check.sh    # 41 assertions — gate + tracker
bash scripts/test-crash-alert-fixes.sh    # 12 assertions — manager fixes
```

`test-alert-dedup-check.sh` is **hermetic**: it copies the scripts under test
into a temp sandbox, puts a fake `bead` CLI on PATH, and derives nothing from
the real store — so it is fast, offline, and safe to run any time. Scenarios
map 1:1 to the gap analysis: D-1 repro inverted (closed bead resolves), target-
keyed dedup, real-title extraction, domchk-* acceptance, reopen/expiry
semantics, VERIFIED-vs-FAILED marker handling, fail-open on an unreadable store,
caller-CWD independence, and report semantics (the word "duplicate" never
appears when there is no duplicate). Live-store scenarios exist but run only
under `DEDUP_TEST_LIVE=1`, so default runs stay hermetic.

### Live verification record — 2026-09-07, HEAD `48aafce`

Run this session while writing this guide; re-run these to re-verify after any
change to the four scripts.

```bash
# Suites: 41/41 and 12/12, both exit 0
bash scripts/test-alert-dedup-check.sh; echo $?
bash scripts/test-crash-alert-fixes.sh; echo $?

# D-1 inverted: a CLOSED target now resolves (was NOT_RESOLVED/exit 1 pre-48aafce)
./scripts/crash-resolution-tracker.sh bf-173o7e check; echo $?
#  → RESOLVED, exit 0

# The headline duplicate shape: OPEN alert bead, CLOSED target, real needle title
./scripts/alert-deduplication.sh check bf-2ftau; echo $?      # bf-4yjq target
#  → DUPLICATE: crash target bf-4yjq is already resolved, exit 0

# A genuine UNIQUE live specimen: open target, no covering alert
./scripts/alert-deduplication.sh check domchk-468b69b8; echo $?
#  → PROCEED: no resolution and no open alert for target bf-hw4i5, exit 1

# Contract edges
./scripts/alert-deduplication.sh check            # → usage, exit 2
./scripts/alert-deduplication.sh check bf-nope99  # → INDETERMINATE fail-open, exit 3

# Report separates per-target repeats from fleet-wide coincidence
./scripts/alert-deduplication.sh report --window-hours 24
#  → "DUPLICATE ALERT PATTERN: bead <X> had 3 crash-class events ..." (per-target)
#  → "FLEET-WIDE CRASH OBSERVATION: 101 distinct beads exited 1 in the last 24h
#     - infrastructure event, not an alert-level repeat"
```

Census commands for the backlog the gate exists to stop growing (count open
alerts whose target is closed) are in the gap analysis, Appendix A — 176 as of
this session.

---

## 8. Troubleshooting

| Symptom | Cause | Action |
|---|---|---|
| Gate exits 3 ("not found in store") on a bead that exists | `bead list --json` returned empty or the ID is mistyped; also fires for beads outside the current workspace's store | Check `bead show <id>` in this workspace; re-run. Exit 3 is fail-open — treat the alert as unverified, not as unique |
| Gate exits 0 but you believe the crash is live | Verdict cites a resolution: check which one. `VERIFIED` marker stale? `cat .beads/state/work-completion/<target>.json`. Target reopened after a cached closure? `bead show <target>` — live closure wins, so an open target would not have produced exit 0 via closure | Re-run with the marker corrected (`verify-work-completion.sh` re-run) or `mark-unresolved` the cache entry; the store is always authoritative |
| Tracker says NOT_RESOLVED for a bead you know is closed | Pre-2026-09-07 behavior (D-1) — or you are in a different workspace than the bead's store | Confirm the tracker at HEAD reads live state (§4); confirm the bead is in *this* workspace's store |
| Report prints "No crash event record found" | `.beads/events.jsonl` missing — the append-only needle record is not present in this clone | Report mode is informational only; nothing to fix for the gate |
| Manager says "Trace file not found" (exit 3) | Manager is trace-slot-based; alert beads (domchk-*) have no trace slot for the *crash* — the crash trace belongs to the target bead | Run the gate on the alert bead instead (§6); the manager needs the crashed bead's own trace |
| Manager classification reads `=====` | D-3 — still open at HEAD (§5) | Do not rely on the manager's FALSE_POSITIVE / SERVICE_FAILURE branches; use the classifier directly or the gate |
| Gate is never consulted in production | By design, nothing calls it yet — no timer, no harness hook (§1) | Run it manually per §6; wiring it into an automated path is gap analysis §5 P1, unowned |
| Tests fail after editing the gate | Suite is hermetic — a failure is real, not environmental | `bash scripts/test-alert-dedup-check.sh` output names the scenario; map it back to the D-# in the gap analysis |

---

## 9. Known open gaps

Owned by the gap analysis priorities; none blocks using the gate today.

- **P1 — automated path.** The gate is callable, not wired: needle owns bead
  creation, so the in-repo lever is a post-creation triage pass or a hook
  callers invoke before starting investigation work (§5 P1 in the gap analysis).
- **D-3 / manager-side D-5 / D-7 / D-9** — the manager's classification
  extraction, its exit-code parse, its silent cooldown, and its missing
  consumers (§5). Fixing these means touching `crash-alert-manager.sh`, which
  currently carries sibling beads' uncommitted work — coordinate first.
- **P4 — backlog sweep.** ~176 open alerts against closed targets already in
  the pool. The gate stops new ones; draining the existing pool is a
  report-only pass that feeds each alert's closure-bead blocker (closure stays
  with the blockers, per the standing convention).

---

## 10. References

- `docs/alert-deduplication-gap-analysis-2026-09-07.md` — D-1..D-10, with
  reproduction commands (Appendix A)
- `docs/crash-alert-fix-implementation-2026-09-02.md` — the six manager fixes
- `docs/crash-response-guide.md` — investigation workflow, work-completion
  markers during triage
- `scripts/README.md` — script quick reference
- Commits: `7d34f8c` (gap analysis, domchk-b5448b6a) · `48aafce` (gate + tracker
  rewrite + suite, domchk-b3de301f)
