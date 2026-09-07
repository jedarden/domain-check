# Crash Root Cause Determination — Corpus Synthesis (domchk-4f0b8b43, 2026-09-07)

**Bead:** domchk-4f0b8b43 ("Document root cause with supporting evidence")
**Chain:** consumes [`docs/crash-classification-domchk-4857bed2-2026-09-07.md`](crash-classification-domchk-4857bed2-2026-09-07.md)
(classification, `5ede627`) ← [`docs/crash-data-gathering-domchk-990ef135-2026-09-07.md`](crash-data-gathering-domchk-990ef135-2026-09-07.md)
(data gathering); feeds domchk-3b605127 ("Develop mitigation strategy based on root cause")
**Provenance:** authored 2026-09-07 ~19:00–19:45Z. Every load-bearing figure in §2–§4 was
**re-derived first-hand from primary artifacts by this bead** (not copied from the upstream
docs) — `journalctl` reads byte-exact, `events.jsonl` re-parsed, all 173 exit=1 trace bodies
re-census'd, bead store re-read, and HEAD built in isolation from a dirty co-tenant tree.
Repo at HEAD `8cc1172` (a co-tenant's commit ahead of `origin/main` `5ede627`; this doc is
the only content change by this bead). Method notes are inline so each figure can be re-run.

---

## 1. Root cause determination (the verdict)

**The domain-check codebase has no defect in any captured failure record, and no captured
kill is a memory-exhaustion kill.** The corpus's crash and failure population resolves into
three mechanisms, each external to the application:

| # | Mechanism | Population | Root cause (one layer deeper than the classification) |
|---|---|---|---|
| M1 | **External userspace SIGKILL-class kill of needle workers**, taking in-flight dispatch scopes with them | 2 dispatch deaths (bf-57nao4, bf-12gb0r, Aug 26) + 1 early-death (domchk-e761cbd5, Sep 2) | A chronic needle-worker churn regime on the box (§4-B: 9 worker deaths in 11 min, restart counters in the hundreds/thousands, `status=72/OSFILE` while idle) — neither kernel OOM nor systemd-oomd is involved (both verified absent). The actor is needle/fleet-side and remains **unidentified**; the damage profile is fully characterized |
| M2 | **Service-class API unavailability** | 129 of 173 exit=1 (74.6%) — terminal `503 no available server` ×105, `Connection refused` ×12, can't reach API server ×7, 429 ×5 | Upstream gateway/relay capacity, landing in synchronized waves (117 of 173 on Sep 1–2; 76 inside the Sep 1 19:00Z–Sep 2 10:59Z band). Not a workspace defect; classify the wave, not the bead |
| M3 | **Turn-budget exhaustion** | 43 of 173 exit=1 (24.9%), every one at exactly `num_turns: 31` | The dispatch cap being reached mid-work, **with no service-failure signature in any of the 43 bodies** (§4). The classification's retraction stands: the "turn-cap secondary to service degradation" cascade is unproven and is not carried forward |
| — | Hard time-caps (exit=124, `timeout_reason: hard`) | 7 | Not crashes at all — pinned-to-the-cap workload-bound runs (5× 3600 s on Sep 6 alone) |

For the two Aug-26 dispatch deaths specifically — the only crashes in the corpus — the
determination is **FALSE_POSITIVE (post-completion)**, and the causal chain behind them is
now closed end-to-end (§3): an alert bead left unclosed by its own exit=0 runs stayed in the
ready frontier, was re-claimed by the same worker 0.25 s after finishing, closed correctly
in-run on the third attempt, and was then killed in the worker-churn wave — costing
bookkeeping only. **No work was lost. No investigation bead should be spawned for either
Aug-26 bead** (their verdicts are final at this layer).

The one *actionable defect this corpus exposes* is in the observability plumbing, not the
application: bf-57nao4's kill is the only one of the corpus's two captured crashes with **no
crash record in `.beads/events.jsonl`** — because the needle worker that would have written
it died in the same wave first (§4-C). The alert layer consequently cannot see that crash at
all, and the known `domchk-f6fff20f` banner defect would swallow the verdicts that do exist.

---

## 2. Evidence chain — what was re-verified first-hand, and how

Every row below was re-executed by this bead on 2026-09-07 (pass-3). The pass-2 figures in
[`crash-classification-domchk-4857bed2-2026-09-07.md`](crash-classification-domchk-4857bed2-2026-09-07.md)
§4 are reproduced exactly, with two additions (§5).

| # | Claim | Primary artifact | First-hand result |
|---|---|---|---|
| E1 | bf-57nao4 closed 7 s before its kill | `bead show bf-57nao4` → Closed, Updated `2026-08-26T22:54:09.727937885Z`; kill at `22:54:16.033Z` (trace metadata) | ✅ byte-exact |
| E2 | Completed exit=0 twice, re-claimed +0.25 s, killed 50.6 s into attempt 3 | `.beads/events.jsonl`: complete `22:50:00.769Z` (79.5 s, `lab-drawrace`) → complete `22:53:01.967Z` (108.1 s, `lab-domain-check-2`) → claim `22:53:02.213Z` (**+0.25 s, same worker, same strand**) → dispatch `22:53:02.216Z` | ✅ exact |
| E3 | bf-12gb0r's deliverable is in the tree | `internal/watch/`, `internal/server/handlers_watch.go`, `docs/adr/001-domain-watch-webhook-notifications.md` @ `0d64373` | ✅ all present on disk |
| E4 | Scope memory peaks 231 M / 366.5 M — memory exhaustion excluded | `journalctl --user` teardown lines, epoch-bracketed `--since @…` (LOCAL-time trap avoided): `run-p2868370-i233525862.scope: Consumed 2.995s CPU time, 231M memory peak` @ 18:54:15 EDT; `run-p2866338-i233523830.scope: Consumed 10.638s CPU time, 366.5M memory peak` @ 18:54:48 EDT | ✅ byte-exact; peaks are ~35× below the 12 GiB dispatch ceiling |
| E5 | No kernel OOM, no systemd-oomd on Aug 26 | `journalctl -k` in the 22:45–22:58Z window → 0 `oom-kill` lines; `journalctl -u systemd-oomd` for Aug 26 → `-- No entries --` | ✅ both empty — the kill actor is **userspace** |
| E6 | domchk-e761cbd5 early-death kill | stray `.beads/traces/domchk-e761cbd5.jsonl` → `{"exit_code": -1, "signal": "SIGKILL", "uptime_seconds": 45, "turns_completed": 0}` | ✅ byte-exact; zero turns = nothing to lose |
| E8 | 129/173 exit=1 terminate on a service-class error | terminal-`API Error` census over all 173 `stdout.txt` bodies: 503×105, refused×12, reach×7, 429×5 = **129** | ✅ exact reproduction |
| E10 | 43/43 turn-cap bodies carry **no** genuine service signature | `error_max_turns` census over the remaining 44 bodies (1 undetermined, `domchk-6129d0c7`); re-run with the §3.5-specific error strings → 0 signatures | ✅ 43/43 clean — and see §6 for the one false hit a looser grep produced |
| E11 | Zero code defects in any captured record | `panic:` in 0/173 bodies; `goroutine \d+` real dumps 0; crash records carry application errors 0 | ✅ exact |
| E12 | All kernel memcg kills are synthetic harness scopes; **0 dispatch scopes** | `task_memcg` census, Sep 6→present: 96 lines = replay-harness 50 + `safe-git-gc-*` 43 + `mw-*` 3 + `run-p*` dispatch **0** | ✅ exact (scope must be basename-matched after splitting the value — paths otherwise mis-bin) |
| — | Corpus population | catalog `docs/crash-data-traces-catalog-2026-09-07.csv`: 1925 rows = 1726 success / 173 failure / 2 crash / 7 timeout / 17 in-flight | ✅ exact |
| — | Repo-side exclusions hold live | `check-repo-health.sh` → exit 0; `.git` 105 M; 167 loose objects / 1.27 MiB; 43 G mem available (19:40Z) | ✅ the bf-1s6c3/bf-4yjq repo-bloat mechanism and the bf-198ne push variant remain repaired and out of scope for this corpus |

---

## 3. Timeline — the full causal chain of the corpus's one real crash pair

### 3.1 bf-57nao4 ("ALERT: Agent crash on bead bf-173o7e", created 2026-08-14 14:26:50Z)

| When (UTC) | Event | Source |
|---|---|---|
| Aug 17 18:13:15.839 | claim (`lab-domain-check`) → 18:16:27 complete **exit=0** — bead **not closed** | `.beads/events.jsonl` |
| Aug 26 22:48:40.969 | claim attempt 2 (`lab-drawrace`, strand explore) | events layer |
| Aug 26 22:50:00.769 | complete **exit=0** — bead **still not closed**; the worker's death line lands the same second (`18:50:00 EDT`, `state=Selecting`, uptime 80 s) | events layer + user journal |
| Aug 26 22:51:13.801 | claim attempt 3 (`lab-domain-check-2`) — **0.24 s after that worker's previous completion** (`bf-4ifshb` at 22:51:13.560) | events layer |
| Aug 26 22:53:01.967 | complete **exit=0** — bead still open | events layer |
| Aug 26 22:53:02.213 | **re-claim of the same bead, +0.25 s, same worker, same strand** → dispatch attempt 4 at .216 | events layer |
| Aug 26 22:52:37 – 22:57:53 | surrounding window: 9 needle-worker deaths in 11 min (§4-B) — `lab-s1` 22:52:37, `lab-roam-2` 22:52:55, `lab-test-fix` 22:53:24, `lab-domain-check` 22:53:48, `lab-drawrace` 22:54:53, `lab-domain-check` again 22:57:53, `lab-roam-2` 22:58:09, + `lab-bead-forge` ×3 and `aggregator-monitor` ×3 | user journal |
| Aug 26 22:54:09.727 | **bead closed in-run** ("Duplicate false positive — original git gc work (bf-173o7e) completed successfully") | bead store |
| Aug 26 22:54:15 EDT | dispatch scope torn down: `Consumed 2.995s CPU time, 231M memory peak` | user journal |
| Aug 26 22:54:16.033 | **killed** — 6.3 s after closing, 50.6 s into the attempt. **No crash record is ever written**: this dispatch is `lab-domain-check-2`'s last event ever (2776 events Aug 17 15:06Z → Aug 26 22:53:02Z, then silence; the name never appears in the user journal) | trace metadata + events layer |

### 3.2 bf-12gb0r (same alert generation, same wave, 32 s later)

| When (UTC) | Event | Source |
|---|---|---|
| Aug 17 18:16:27.708 | claim → 18:21:17 complete **exit=0**, not closed | events layer |
| Aug 26 22:52:03.918 | claim (`lab-drawrace`, restarted after its 22:50:00 death) → dispatch | events layer |
| Aug 26 22:54:40.730 | **bead closed in-run** (ADR-001 Domain Watch complete) | bead store |
| Aug 26 22:54:48.546 | killed at 366.5 M scope peak — deliverable verified in tree (E3) | trace metadata + journal |
| Aug 26 22:54:48.794 | **crash record written** to events layer by `lab-drawrace` — which then itself died at 22:54:53 EDT (`state=Handling`, `status=1/FAILURE`) | events layer + user journal |

### 3.3 Reading the timeline

Both beads are crash-**alert** beads from the Aug-14 generation that ran to exit=0 on
Aug 17 without closing, sat back in the ready frontier, and were re-dispatched nine days
later. The Aug-26 re-claims are the documented alert-bead release-cycling shape: an agent
turn that ends on anything other than `bead close` returns the bead to the frontier, and
the queue correctly re-offers it. Both Aug-26 attempts that finally closed their beads
succeeded in doing so **before** the kills landed — which is exactly why the crash-response
guide's "verify the target bead's actual state first" rule classifies both as
FALSE_POSITIVE, and why the loss here is two stale alert beads' bookkeeping and nothing
else. The third exit=−1 (domchk-e761cbd5) died at 45 s uptime with 0 turns — nothing
executed, nothing to lose.

---

## 4. The three handoff leads — advanced

### A. Why did the queue re-claim a closed bead within 0.25 s? — **ANSWERED (it didn't)**

The bead was **not closed** when it was re-claimed — it was open, after two exit=0 runs
that had each ended without a `bead close`. The 0.25 s interval is not a needle anomaly at
all: it is that worker's routine complete→claim-next cadence, visible five times in the
preceding hour (`bf-4cxa1d` 22:40:56, `bf-28su5u` 22:43:14/22:46:09/22:47:33, `bf-4ifshb`
22:49:42/22:51:13 — every completion followed by the next claim in 0.2–0.3 s). The queue
did nothing wrong; the *upstream* condition is exit-0-without-close on alert beads, which
is already in the canon (the NEEDLE auto-split alert-loop finding: template do-not-close
lines + release cycling). **Residual needle-side lead for the mitigation bead:** whether an
exit=0 run that closes nothing should be *released back to ready immediately* (today) or
held/cooled before re-offer — that policy choice is what turns one unclosed run into a
three-attempt loop.

### B. What external actor killed workers in the 22:52–22:58Z wave? — **NARROWED, actor unidentified**

Proven by absence and by shape:

- **Not the kernel OOM killer** — 0 `oom-kill` lines in the window (E5).
- **Not systemd-oomd** — no entries for all of Aug 26 (E5); this also permanently retires the
  `crash-artifacts-bf-3561g.md` oomd attribution for this era (that doc is about Aug 16 and
  says 94.29 %, not 94.71 % — see the classification §6.5).
- **Not memory pressure anywhere** — the two dispatch scopes peaked at 231 M / 366.5 M
  (E4), orders of magnitude under every boundary on the box.
- **It is chronic, not a one-off** — restart counters read 2704 (`lab-roam-2`) and 335
  (`lab-s1`) *at that moment*; the same `status=72/OSFILE` + "stopped unexpectedly …
  killed by an external process (e.g., SIGKILL, OOM, capacity governor)" signature repeats
  across nine worker deaths in 11 minutes, and 7 of the 9 died in `state=Selecting` with
  `beads_processed=0` — **idle** workers being killed, which is why only two of them cost a
  dispatch.
- **The fleet kept working through it** — `lab-drawrace`, `lab-roam-2`, and
  `lab-domain-check` all claimed and completed beads after their own neighbors died;
  `lab-domain-check-2` is the only worker that vanished permanently.

So: a userspace actor, needle/fleet-side, killing mostly-idle worker processes
intermittently and chronically — needle's own "capacity governor" hypothesis is the
leading candidate and is **outside this workspace's authority to identify further** (the
needle supervisor's logs are fleet-level; nothing in domain-check can see the actor).
Handed to domchk-3b605127 as an infra-side mitigation target, with the honest caveat that
the actor is not yet named.

### C. Why did the events layer capture bf-12gb0r's crash but not bf-57nao4's? — **ANSWERED**

The crash record is written by the needle **worker** after it observes its dispatch's
death. bf-12gb0r's worker (`lab-drawrace`) survived 5 s past the kill and wrote the record
before dying itself; bf-57nao4's worker (`lab-domain-check-2`) was already dead — its
event stream ends at the attempt-4 dispatch, and nothing terminal was ever written for
that dispatch. The layer demonstrably works: **247 crash records across ~74 beads all-time
in `events.jsonl`**, including the whole Aug-16 cluster. The gap is therefore not a
classifier bug but a **capture race: a kill wave that takes the worker before it can
record its child's death leaves that crash invisible to every downstream consumer**
(classifier window derivation, alert manager, dedup). That is the needle-side finding the
classification's §7.2 suspected, now with the mechanism named.

---

## 5. Cross-reference against CLAUDE.md crash-prevention guidelines

| CLAUDE.md guidance | How this corpus exercises it |
|---|---|
| Quick classification: "Exit −1 → infrastructure event; verify work completion first" | Vindicated exactly: both exit=−1 dispatches are post-completion external kills (E1–E4); both beads closed and delivered before dying |
| "Verify the target bead's actual state **before** investigating" | The deciding rule here — both Aug-26 verdicts fall out of the bead store alone. Also the reason no new investigation bead is owed for either |
| "If 10+ crashes in 10 minutes → INFRASTRUCTURE EVENT" | The wave is real but its dispatch-level casualty count is 2; the *worker*-level count is 9/11 min. The guide's threshold correctly points at infrastructure — the refinement this corpus adds is that the event is worker churn, not dispatch failure |
| "Domain-check code has NO defects; crashes are external" | Holds for the corpus: 0 panics, 0 application errors, 0 real goroutine dumps across all 176 failure/crash/timeout records (E11), HEAD builds and vets clean in isolation (§7) |
| Repository-bloat prevention layer (gitignore, 10 MB pre-commit gate, pack-memory bound, daily health timer) | Unexercised by this corpus by design — the era it guards against (bf-1s6c3/bf-4yjq, GB-scale memcg kills) is absent: all Sep-6→present kernel kills are the bound-verification harnesses' own scopes (E12), and the repo is 105 M with health exit 0 today |
| Monitoring stack (resource/service monitors, crash-pattern detection) | Correctly silent on Aug 26 (`.beads/logs/` starts Sep 1 — journald is the only surviving Aug source); the 50,488-line crash-monitor log against 3 real crash records confirms the guide's alert-noise caveat |
| Known defect `domchk-f6fff20f` (alert manager reads the classifier's `====` banner as `CLASSIFICATION`) | Compounds the §4-C capture gap: even the verdicts that exist never reach the alert layer. Both defects are alert-plumbing, not application, and both are already owned |

**Genuine gaps this corpus adds to the prevention register** (for domchk-3b605127):

1. **Crash-record capture race** (§4-C): a crash whose worker dies in the same wave is
   invisible to the events layer, and therefore to the classifier and alert manager. No
   workspace-side mitigation exists; the fix is needle-side (supervisor-level crash
   recording), and until then the manual rule stands: **when a trace says `outcome: crash`
   but `events.jsonl` has no crash record, classify from the trace, not from the events
   layer.** This is the same window trap recorded in the classification §5 for
   `crash-classifier.sh`.
2. **The `domchk-f6fff20f` banner defect** (pre-existing, open) — reconfirmed as live by
   this chain; the FALSE_POSITIVE branch of the alert manager remains dead code until it
   lands.
3. **Exit-0-without-close release cycling** (§4-A residual) — a queue-policy question for
   needle, already documented as the alert-loop root; this corpus adds a quantified
   instance (one alert bead cycling across nine days, four dispatches, two exit=0 runs
   before closing).

---

## 6. No-code-defect verification (acceptance criterion)

Three independent legs:

1. **Corpus census (primary):** across all 173 exit=1 bodies, 2 crash captures, and 1
   stray crash record — `panic:` 0, real goroutine dumps 0 (`goroutine \d+` pattern),
   application error messages 0. The 129 service-terminal records end on the harness's own
   `is_error: true` API-failure shape (`model: "<synthetic>"`), not on anything the
   application emitted.
2. **Build/vet (supporting):** HEAD `8cc1172` extracted via `git archive` to `/tmp` (the
   working tree is co-tenant-dirty and is *not* a valid build surface) — `go build ./...`
   exit 0, `go vet ./...` exit 0.
3. **Panic-site audit (supporting):** 5 non-test `panic(` sites, all legitimate: three
   startup-time invariants (embedded template/static-FS load, unparseable CIDR config at
   init) and two recover-and-repanic middleware idioms (`safeguards.go:54,82`, the
   standard `http.ErrAbortHandler` pattern). None is reachable from a request path as an
   unhandled defect.

Method note for future re-runs (the §7.6 hazard, reproduced live): a *loose* service-marker
grep over the 43 turn-cap bodies produced exactly 1 false hit — the word `overloaded`
inside a **tool-call argument** (`"partial_json":" overloaded"`), not an API error. With
the full §3.5 error-string forms, the count is 0/43, as claimed. Match full error strings
only; spot-check every hit's context.

---

## 7. Pattern comparison with the existing canon

- **The bf-1ea4g shape** ([`docs/crash-inventory-bf-1ea4g-summary.md`](crash-inventory-bf-1ea4g-summary.md)):
  post-completion external kills dominating an exit=−1 alert stream; bf-57nao4/bf-12gb0r
  are the same class with the bead-store close proven to the second (E1).
- **NOT the repo-bloat memcg-OOM shape** (bf-1s6c3 / bf-4yjq,
  [`docs/crash-analysis-bf-1s6c3-2026-09-06.md`](crash-analysis-bf-1s6c3-2026-09-06.md)):
  those were GB-scale `CONSTRAINT_MEMCG` kills against an 18 GB repo; this corpus's peaks
  are 231–366 M and the repo is 105 M, bound verified, health exit 0 (§2, last row).
- **NOT the push-side variant** (bf-198ne, `docs/crashes/bf-198ne-crash-report.md`): no
  `git push` appears in any crash record and the unpushed backlog is 0.
- **The service-wave signature** (fleet crash census, Sep 2026): independently reproduced
  from primary transcripts — 117 of 173 exit=1 land on Sep 1–2 with terminal 503/429
  strings; per-bead investigation of these is the wrong unit.
- **The bf-3561g oomd attribution** (`docs/crash-artifacts-bf-3561g.md`): not reproducible
  for Aug 26 and misdated besides — treat that doc's oomd mechanism as Aug-16-only.

---

## 8. What this document adds beyond its sources

1. **The crash-record capture race named and proven** (§4-C): bf-57nao4's missing
   events-layer record is explained by its worker's own death — the worker's event stream
   ends at the fatal dispatch (2776 events, then nothing), the first direct evidence for
   the classification's §7.2 suspicion. All-time coverage (247 records) proves the layer
   itself works.
2. **The 0.25 s re-claim demystified** (§4-A): it is the worker's normal claim cadence,
   demonstrated five times in the preceding hour on *other* beads; the anomaly was never
   the interval, it was the bead's open state after exit=0 runs.
3. **The wave sized and classified** (§4-B): nine worker deaths in 11 minutes, seven of
   them idle (`state=Selecting`, `beads_processed=0`), restart counters 2704/335 —
   chronic churn with a userspace actor, kernel and oomd both excluded by absence.
4. **A pass-3 reproduction of every load-bearing census** (§2), including the one false
   hit a looser grep produces in the turn-cap bucket (§6) — recorded as a live
   demonstration of the classification's §7.6 method hazard.

---

## 9. Handoff to domchk-3b605127 (mitigation)

Nothing in this corpus warrants an application-side change. The mitigation bead's honest
scope is:

1. **Needle-side (primary, outside this repo):** the worker-churn actor (§4-B) and the
   crash-record capture race (§4-C). Both need fleet-level authority; this document is the
   evidence package for escalating them.
2. **Workspace-side, already-owned:** land `domchk-f6fff20f` (the CLASSIFICATION banner
   fix) so the verdicts that do exist reach the alert layer.
3. **Workspace-side, cheap and new:** the manual classification rule from §5 gap 1 —
   "trace says crash, events layer silent → classify from the trace" — is already how this
   chain worked; the residual is recording it where the next classifier consumer will find
   it (it is in this document and the classification §5).
4. **Explicitly out of scope:** re-investigating bf-57nao4/bf-12gb0r (final
   FALSE_POSITIVE), any domain-check code change (no defect exists to fix), and any
   repo-maintenance change (the bloat-era safeguards are verified holding — §2 last row).

## 10. Re-run instructions

All figures re-derive from: the catalog
`docs/crash-data-traces-catalog-2026-09-07.csv` + `.beads/traces/*/stdout.txt` bodies
(terminal-error/turn-cap/panic censuses, §6 method note); `.beads/events.jsonl` filtered by
bead/worker/window (timelines, §3–§4); `bead show <id>` (close instants); `journalctl
--user` / `-k` / `-u systemd-oomd` with **epoch** `--since @<ts>` brackets (journald's ISO
`--since` is LOCAL time — the recurring trap) for scope teardowns, worker deaths, and
oom absence; `task_memcg` basename-matched after splitting the comma-joined value for the
kernel-kill census; `git archive HEAD | tar -x -C <tmp>` for a clean-tree build on this
shared, co-tenant-dirty worktree.
