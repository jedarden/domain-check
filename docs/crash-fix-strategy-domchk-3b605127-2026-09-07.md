# Crash Fix Strategy — Corpus Mitigation (domchk-3b605127, 2026-09-07)

**Bead:** domchk-3b605127 ("Propose and document fix strategy")
**Chain:** consumes [`docs/crash-root-cause-domchk-4f0b8b43-2026-09-07.md`](crash-root-cause-domchk-4f0b8b43-2026-09-07.md)
(root-cause determination, `339b696`+`304d2ed`) ← classification (`domchk-4857bed2`) ←
data gathering (`domchk-990ef135`); feeds domchk-ac6f3e1f (the parent crash-investigation
bead this chain was split from)
**Provenance:** authored 2026-09-07 ~20:00–20:30Z against HEAD `7e34c2f` (= `origin/main`,
0 unpushed). Every claim about current tool behavior below was **re-verified live by this
bead at that HEAD** — `crash-classifier.sh` / `crash-alert-manager.sh` read directly,
bf-57nao4's event stream re-parsed from `.beads/events.jsonl`, `check-repo-health.sh`
re-run (exit 0), and the `domchk-f6fff20f` fix re-read at its landing site. Nothing here
rests on the upstream documents' say-so alone.

---

## 1. Strategy at a glance

The root cause is **three external mechanisms and two observability gaps, with zero
application defects** (RCA §1). The strategy therefore makes **no domain-check code
change**. Its honest scope splits by authority:

| # | Item | Fixes | Authority | Effort | Status in this leg |
|---|---|---|---|---|---|
| I1 | Close the stale-open `domchk-f6fff20f` tracking bead (its defect is fixed and twice-verified at HEAD) | RCA gap 2 residual | workspace (bookkeeping) | 1 min | **Executed by this bead** — see §3.1 |
| I2 | Name the **crash-record capture race** in `crash-classifier.sh` (emit `INFRASTRUCTURE` with the mechanism, not bare `UNKNOWN`, when the events layer is silent under a provenance-ok crash trace) | RCA gap 1 | workspace | small | **Proposed with exact patch spec** — §3.2; needs a follow-up implementer bead |
| I3 | Record the manual classification rule — "trace says crash, events layer silent → classify from the trace" — where the next consumer looks | RCA gap 1 (stopgap) | workspace | 1 min | **Landed in this leg** — CLAUDE.md dated correction + this document |
| I4 | Escalate the **needle worker-churn actor** (9 deaths/11 min, restart counters 2704/335, idle workers killed) to the fleet owner | M1 | needle/fleet — **outside this repo** | — | Evidence package assembled — §3.4 |
| I5 | Supervisor-level crash recording on the needle side (write the crash record from the supervisor, not the worker, so a wave cannot erase it) | RCA gap 1 (durable fix) | needle/fleet | medium | Recommended — §3.5 |
| I6 | Needle queue-policy question: hold/cool an exit=0-without-close bead before re-offer | M1 amplifier, RCA gap 3 | needle/fleet | small | Recommended with quantified instance — §3.6 |

Service waves (M2) and turn-cap exhaustion (M3) need **no new work at all** — every layer
that absorbs them already exists and is verified firing (§4).

---

## 2. Ground rules the strategy obeys

These follow directly from the root cause and from CLAUDE.md; each is a constraint the
implementation steps are checked against in §5.

1. **No application change.** Zero panics, zero application errors, zero real goroutine
   dumps across all 176 failure/crash/timeout records (RCA §6, re-verified). Any
   "fix" that edits `internal/` would be fixing a defect that does not exist.
2. **No re-investigation of bf-57nao4 / bf-12gb0r.** Both are final FALSE_POSITIVE
   (closed before their kills, deliverables in tree); the RCA's verdict is terminal.
3. **No repo-maintenance churn.** The bloat-era safeguards are verified holding —
   `check-repo-health.sh` exit 0 re-run by this bead today; 105 MB repo, effective
   pack-memory bound ≈3 GiB worst case. Nothing in this strategy touches them.
4. **Everything workspace-side flows through the existing crash-alert system** — the
   classifier, alert manager, dedup, cooldown, and surge detector — rather than adding a
   parallel pipeline. I2 is one new signal inside the existing classifier, not a new tool.
5. **Needle-side items are escalated, not improvised.** In particular `needle cleanup`
   is prohibited by CLAUDE.md (it SIGHUPs live workers) and must not be reached for as a
   "fix" for the churn actor.

---

## 3. The items in detail

### 3.1 I1 — close `domchk-f6fff20f` (bookkeeping; EXECUTED in this leg)

The banner defect is fixed: at HEAD `7e34c2f`, `crash-alert-manager.sh` extracts the
classification with

```
grep -m1 -E '^(FALSE_POSITIVE|SERVICE_FAILURE|INFRASTRUCTURE|CODE_DEFECT|UNKNOWN)[[:space:]]*$'
```

with a `head -1` fallback only when no verdict line exists (the `====` banner cannot match
the anchored pattern), and `crash-classifier.sh` prints the verdict as line 1 of stdout
before any human context. Two independent verifications already exist in the record
(`domchk-81938e89`'s cascade replay at HEAD `e9ab3d4`; RCA §5 gap 2 at HEAD `339b696`);
this bead re-read both scripts at HEAD `7e34c2f` as a third. The bead itself sat **Open,
unassigned, rev 3**, its "NOT yet fixed" note dated against older HEAD `ba23173`.

**Action taken by this bead:** verified the fix live, then
`bead close domchk-f6fff20f --if-revision 3 --reason "…"` with the evidence above.
This is exactly the "bookkeeping owed" the RCA §9.2 hands to this bead, and it makes the
defect register reflect the tree. If the original reporter wants it reopened for their own
verification protocol, `bead reopen` is available and nothing in the close reason prevents
it.

### 3.2 I2 — name the capture race in `crash-classifier.sh` (proposed patch)

**The gap, re-verified first-hand.** bf-57nao4's event stream in `.beads/events.jsonl`
ends at its attempt-4 `dispatch` record (`2026-08-26T22:53:02.216Z`) — no `complete`, no
`fail`, no `crash` ever lands, because the worker that would have written them died in the
same wave (RCA §4-C). For such a bead the classifier's crash-instant resolver
(`resolve_crash_epoch`) finds no events window and (the trace slot having been overwritten
by later runs) no provenance-ok `captured_at`, so classification falls through to bare
`UNKNOWN` — indistinguishable, to every downstream consumer, from "we looked and there was
no crash". The RCA's manual rule covers humans; nothing covers the automated path.

**Proposed behavior.** One new signal, `signal_capture_race()`, consulted in
`classify_crash()` only when the existing signals have not fired and the classification
would otherwise be `UNKNOWN`:

- **Events leg (provenance-independent, safe on its own):** the bead has ≥1 `dispatch`
  record in `.beads/events.jsonl` whose ts is the bead's **last** event of any kind — no
  subsequent `complete` / `fail` / `crash` / `timeout` for that dispatch — **and** at
  least one earlier run for the same bead ended `complete` exit=0 (the signature of a
  worker dying before it can record anything, rather than a bead that was never picked
  up). → classify `INFRASTRUCTURE`, mechanism line
  `crash-record capture race (worker died before writing the crash record); classify from the trace`.
- **Trace leg (only when `PROVENANCE=ok`, per the classifier's existing evidence rule):**
  trace metadata shows `exit_code: -1` / `SIGKILL` → attach it as the mechanism's
  supporting evidence line. Never quote an unprovenanced slot — that is the bf-3561g
  fabricated-evidence failure the classifier already guards against.

**Implementation approach.**

- Insertion point: the `UNKNOWN` fallback tail of `classify_crash()`
  (`scripts/crash-classifier.sh`, currently the `echo "UNKNOWN"` at ~line 532). The
  resolver `resolve_crash_epoch()` stays untouched — the signal keys on event-kind shape,
  not on a crash instant, so it cannot poison the window derivation.
- Reuse the file's own parsing convention (the `crash_window_from_events()` python block
  already reads `.beads/events.jsonl`; add event-kind extraction to the same pattern) and
  its fail-open contract (§ header comment: every helper returns nonzero for "signal not
  asserted" and never aborts the run).
- Output contract: verdict stays line 1 of stdout (the alert manager greps an anchored
  token line — `crash-alert-manager.sh:382-383`); the mechanism line follows it, so
  `Reason:` extraction in the manager's FALSE_POSITIVE branch picks it up unchanged.
- Manager side: no change required — `INFRASTRUCTURE` already flows through the
  per-classification cooldown and the suppression log. Optionally log the mechanism line
  via `log_alert` for the crash-history record.
- **Tests:** extend `scripts/test-crash-alert-fixes.sh` with a grep-marker test for the
  new signal, and — the lesson that produced the f6fff20f defect — a **functional**
  case: replay a fabricated events tail (`…dispatch`, nothing after) plus a
  provenance-ok crash trace through the classifier and assert the verdict line is
  `INFRASTRUCTURE`, not `UNKNOWN`. The bf-6d3d6 replay harness
  (`domchk-81938e89`'s sandboxed replay, stubbed `bead`) is the template.
- **Provenance hazard for the implementer:** `crash-classifier.sh`,
  `crash-alert-manager.sh`, and `test-crash-alert-fixes.sh` all carry **co-tenant dirt
  right now** (staged and unstaged, in this shared worktree). Land this patch from a
  clean extract (`git archive HEAD | tar -x -C <tmp>`, develop, then commit via the
  private-index recipe) or wait until the siblings' in-flight edits are pushed — do not
  build on the dirty tree, and do not sweep co-tenant hunks into the commit.
- **Owner:** this is implementation, not strategy — it needs its own bead. Nothing in the
  current chain is dispatched to write it.

**What this buys, honestly:** detection and correct labeling only. The crashes
themselves are needle-side (I4/I5); no workspace change reduces their frequency.

### 3.3 I3 — the stopgap rule, recorded where consumers look (LANDED in this leg)

Until I2 exists, the RCA §5 gap-1 rule is the working mitigation: **when a trace says
`outcome: crash` but `events.jsonl` has no crash record, classify from the trace, not
from the events layer** — and an automated `UNKNOWN` from the classifier is never, by
itself, evidence that no crash occurred (the classifier window trap is already in the
canon). This bead landed the rule in CLAUDE.md's crash-alert section as a dated
correction (the one place the next investigation actually starts) and it is restated
here with its mechanism. The same dated correction records I1's closure.

### 3.4 I4 — escalate the worker-churn actor (needle/fleet; evidence package = RCA §4-B)

The actor is userspace, chronic, and kills mostly-idle workers (7 of 9 dead in
`state=Selecting` with `beads_processed=0`; restart counters 2704/335 at that moment;
kernel OOM and systemd-oomd both excluded by absence; dispatch scope peaks 231–366 MB,
~35× under the ceiling). Nothing in domain-check can see it, let alone fix it — the
needle supervisor's logs are fleet-level.

**Recommended escalation package** (hand to the operator / file on the NEEDLE side;
`~/NEEDLE` is the fleet repo, `needle-ci` builds it):

- RCA §4-B verbatim as the finding — actor unidentified, damage profile fully
  characterized.
- The two live signatures to grep for on the fleet side: `status=72/OSFILE` +
  "stopped unexpectedly … killed by an external process (e.g., SIGKILL, OOM, capacity
  governor)" in the user journal; needle's own "capacity governor" hypothesis is the
  leading candidate.
- The ask: name the actor, and decide whether idle-worker recycling is intended
  behavior (in which case the fix is to make it gentle — see I5/I6) or a defect.
- **Explicit non-remedy to record:** `needle cleanup` must not be run against the fleet
  to "stabilize" worker counts — CLAUDE.md prohibits it because it SIGHUPs live
  workers, i.e. it would convert the churn problem into guaranteed in-flight
  dispatch kills.

**Workspace-side absorption meanwhile (already live, no change):** the classifier's
INFRASTRUCTURE clustering signal (`CLUSTER_MIN_BEADS=10` other beads within 600 s) and
the alert manager's surge detector catch the wave shape; `crash-pattern-detection.sh`
(10-min systemd timer) reads `events.jsonl` directly and is the ongoing tripwire.

### 3.5 I5 — supervisor-level crash recording (needle/fleet; durable fix for the capture race)

The crash record is written today by the **worker** after it observes its dispatch's
death — so any wave that kills worker and dispatch together erases the record (RCA §4-C:
247 all-time records prove the layer works when the worker survives 5 s; bf-57nao4 shows
it failing when it doesn't). The durable fix is needle-side: the **supervisor** writes a
crash record (or the worker's crash handler writes it to a path that survives the worker)
the moment a dispatch's scope tears down abnormally, independent of the worker's own
liveness. I2 is the workspace-side compensating detection until this lands; when it
lands, I2's signal degrades gracefully (the events leg simply starts finding records).

### 3.6 I6 — exit=0-without-close release cycling (needle/fleet queue policy)

One alert bead cycled across nine days, four dispatches and two exit=0 runs before
closing (RCA §4-A) — each exit=0-without-close run returned it to the ready frontier
immediately, and the worker's routine complete→claim cadence (~0.25 s) re-claimed it
before any human look. The queue did nothing wrong; the policy question is whether an
exit=0 run that closes nothing should be held/cooled before re-offer. Escalate with the
quantified instance. **Workspace-side absorption (already live):** closed-bead filtering
(fixes 1/5), dedup + processed-alerts tracking (fixes 2/3), completion awareness +
exit-code validation (fixes 4/6), and the per-classification cooldown — all six verified
green against the current suites, so the cycling costs bookkeeping, not alert spam.

---

## 4. Integration with the existing crash-prevention system

Where each mechanism lands in the stack that already exists and is verified firing
(timers re-verified 2026-09-07 per CLAUDE.md; suites 13/13 and 7/7 green; replay 10/10):

| Mechanism / signal | Detected by | Absorbed by | Alert outcome | Residual gap → item |
|---|---|---|---|---|
| M1 worker-churn kill, worker survives | events-layer crash record (247 all-time) | classifier provenance gating + FP/INFRASTRUCTURE signals; manager closed-bead filter | classified, deduped, cooled | actor unnamed → I4; record erasure → I5/I2 |
| M1 worker-churn kill, worker dies too | **nothing automated** | manual rule (I3) until I2 lands | bare `UNKNOWN` today → `INFRASTRUCTURE` + mechanism under I2 | I2 |
| M2 service wave (129/173 exit=1, 74.6%) | `service-monitor.sh` (2-min timer, `-sk` gateway check, 503 branch) | classifier `SERVICE_FAILURE` + per-classification cooldown (live since the banner fix) + surge detector → `INFRASTRUCTURE EVENT` | wave classified, not per-bead | none — classify the wave, not the bead |
| M3 turn-cap (`num_turns: 31`, 43/173, 0 service signatures) | classifier `error_max_turns` → `FALSE_POSITIVE` (administrative) | guide's "verify the bead's actual state first" | no investigation bead | turn budget is a dispatch-template choice → needle-side, folded into I4's escalation conversation |
| Repo bloat (retired mechanism) | `check-repo-health.sh` (exit 0 re-run today), 02:00 timer + `auto-gc-trigger.sh --dry-run` | gitignore + 10 MB pre-commit gate + `pack.windowMemory` bounds + bounded gc scripts | health alert | none — verified holding; do not touch (§2.3) |
| Alert spam during any surge | `crash-alert-manager.sh` cooldown + suppression log | `alert-deduplication.sh` (4 legs incl. processed-alerts) | coalesced | none |

The design principle the table encodes: **every new fact this corpus produced is a
signal inside an existing layer, never a new pipeline.** I2 is the only code change
proposed anywhere in this strategy, and it is one function in a script that already
exists for exactly this purpose.

---

## 5. Validation against CLAUDE.md guidelines

| Guideline | How this strategy complies |
|---|---|
| Hard prohibitions (no `.github/workflows/*`, no `kind: Job`/`CronJob`, no mutating `kubectl`, no `:latest`/bare-SHA images, no force-push, no `.beads/` hand-edits, no `needle cleanup`, no bare NATO tmux) | **None is touched.** No CI change (the alert system is local scripts + systemd user timers); I1/I3 are `bead` CLI + doc edits only — `.beads/` is written through the CLI as required; I4 explicitly forbids the `needle cleanup` non-remedy (§3.4) |
| "NixOS box — systemd user timers, no crontab" | Every detection layer referenced is an existing `domain-check-*` timer; nothing here proposes cron or a new scheduler |
| "Domain-check code has NO defects — focus crash investigation on infrastructure/service" | §2.1: zero application edits proposed anywhere; the single code change (I2) is in the crash-observability scripts, not `internal/` |
| "Verify the target bead's actual state before investigating" / FP rules | The strategy's first executed action (I1) is itself a bead-store verification, and I2's events-leg signature keys on verified event shape rather than trace strings |
| Crash alert system fixes 1–6 + cooldown + classification | Unchanged and load-bearing (§4); I2 plugs into the manager's anchored-token grep contract with verdict-first stdout, the exact contract the f6fff20f defect violated |
| Git safety (safe-gc scripts, pack-memory bounds, no bare gc, private-index commits in this shared worktree) | §2.3 leaves the maintenance layer untouched; I2's implementer notes mandate the clean-extract + private-index path because the target scripts are co-tenant-dirty today |
| Bead-rs discipline (`bead` only, close via `bead close`, `--if-revision` for contended beads) | I1 used `bead close --if-revision 3`; no bf-shaped command anywhere |
| Escalation honesty | I4/I5/I6 are labeled needle/fleet-authority with the honest caveat that the churn actor is unidentified — the strategy does not pretend a workspace fix exists where none does |

**Acceptance-criteria mapping:** mitigation strategy from root cause → §1/§3 (one item
per RCA mechanism/gap); immediate fixes → I1 (executed), I3 (landed), I2 (spec'd, small);
preventive measures → I4/I5/I6 escalations + §4 absorption table; existing mitigation
scripts referenced → §4 table + §3.6; implementation approach → §3.2 (patch spec,
insertion point, tests, ownership); CLAUDE.md validation → this section.

---

## 6. Out of scope (explicit)

1. Any change to `internal/` or `web/` — no defect exists to fix (RCA §6).
2. Re-investigating bf-57nao4 / bf-12gb0r / domchk-e761cbd5 — final per RCA §1/§3.
3. Repo-maintenance, gc, or hook changes — the layer is verified holding (§2.3).
4. Closing domchk-ac6f3e1f (this chain's parent) — that bead's owner decides what this
   strategy's adoption means for it; this document is its input, not its verdict.
5. Naming the churn actor — outside this workspace's authority (RCA §4-B).

---

## 7. Re-run / verification instructions

- **I1 evidence:** `git show HEAD:scripts/crash-alert-manager.sh | sed -n '380,385p'`
  (anchored-token grep), `grep -n "verdict" scripts/crash-classifier.sh | head`
  (verdict-first comment block), `bead show domchk-f6fff20f` (Status: Closed).
- **I2 gap, live:** `grep '"bf-57nao4"' .beads/events.jsonl | tail -1` — last record is
  a `dispatch`, nothing terminal follows; then
  `./scripts/crash-classifier.sh bf-57nao4` — today this prints `UNKNOWN`; under I2 it
  should print `INFRASTRUCTURE` + the capture-race mechanism line. (Note: the classifier
  run needs the bead's trace-window env or provenance-ok slot per its header; the
  events-leg signature under I2 is what removes that dependency.)
- **Absorption layers, live:** `./scripts/check-repo-health.sh` (exit 0),
  `systemctl --user list-timers 'domain-check-*' --all` (all future-triggered),
  `./scripts/test-crash-alert-fixes.sh` and `./scripts/test-closed-bead-filter.sh`
  (13/13, 7/7 as of 2026-09-07).
- **journald reads for the churn signature:** epoch brackets only —
  `journalctl --user --since @<epoch> --until @<epoch> | grep -E "status=72/OSFILE|killed by an external"`.
  ISO `--since` is LOCAL time on this box; that trap has bitten two investigations
  already.
