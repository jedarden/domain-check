# bf-65lsdu Crash Investigation — Verification Report

**Date:** 2026-09-09
**Deliverable bead:** domchk-8114156d (document leg of the bf-65lsdu investigation chain)
**Inputs synthesized:** gather leg [domchk-4511fe51](bead://domchk-4511fe51) (Closed rev 5) · analyze leg [domchk-79863691](bead://domchk-79863691) (Closed rev 4)
**Incident bead:** `bf-65lsdu` — "Run repository cleanup to eliminate 17GB bloat" (Closed 2026-08-17, rev 5)
**Classification:** INFRASTRUCTURE EVENT — repository-bloat-era memcg OOM kill regime (carried; **no new cause claim** in this document)
**Domain-check code impact:** **NONE** — zero defects found by this chain and across the 157+ investigation corpus
**Standing canon:** [RCA — bf-65lsdu "signal −1"](../research/root-cause-analysis-bf-65lsdu-signal-minus-one-2026-09-02.md) · [chain retrospective (domchk-754711b5, commit 0ed3e2a)](../crash-investigations/bf-65lsdu-chain-retrospective-domchk-754711b5-2026-09-09.md)

> **Where this document sits.** The path above is this dispatch's task-named output
> location (`docs/benchmarks/`); the bf-65lsdu document family lives in
> `docs/crash-investigations/` and the two documents above are the authoritative
> ones. This report renders the same findings at operator/developer readability
> from the two child legs named in its task, subordinated to that canon. Where an
> older bf-65lsdu document disagrees, §2.5 and §3.5 give the correction.

---

## 1. Executive summary

`bf-65lsdu` was the remediation task for this workspace's 18 GB repository bloat:
run `git gc --aggressive --prune=now` over **17.20 GiB of loose objects (4,515
objects, ~18 GB `.git`)** using the era's `scripts/cleanup-bloat.sh`. The cleanup
of the bloat was itself the most memory-hungry operation the bloat ever produced,
and it ran unbounded (no `pack.windowMemory` existed until 2026-09-02) inside a
needle dispatch scope capped at **12 GiB MemoryMax**.

**What happened:** the bead was not crashed once — it was killed **163 times**
(exit code −1) between 2026-08-13T21:22:23.216Z and 2026-08-14T01:10:58.163Z,
followed by 6 exit-1 failures, the last of which tripped the harness's quarantine
(failure_count 5/5, 2026-08-14T01:11:11.824Z). Each kill produced an alert bead
(**163 alert beads, one per kill — all 163 now Closed**), a release, and a
re-claim ~60–95 s later that re-ran the byte-identical doomed operation.

**Why:** the kernel's memory-cgroup OOM killer killed `git` (pack-objects) as its
anonymous memory crossed the 12 GiB scope limit. `exit −1` is needle's
abnormal-child-death **sentinel**, not a signal number — a real SIGKILL encodes
137, a timeout 124. No domain-check code was involved in the failure.

**How it ended:** retry could never work (the memory demand is a function of the
repository contents, which did not change between attempts). What worked was
**decomposition** — on 2026-08-17 the bead was split into three sequential
children (baseline → execute → verify, all Closed), the actual gc ran under them
in **90.3 seconds** (exit 0), and the parent closed the same hour. The repository
was fully repaired by the verified 2026-09-01 cleanup and holds today at
**105 MB `.git` / 1 pack / 34 loose objects / 0 garbage**.

**State now:** the chain is fully closed — target bead, all three split children,
both template-chain legs, and all 163 alert beads. Prevention is layered and was
re-verified live for this report (§5). Nothing is owed on this chain except one
unpicked sibling analysis bead, flagged in §4.6.

---

## 2. Timeline and context

### 2.1 Background — the bloat era

Committed bead state did it: repeated ~237 MB `.beads/*.jsonl` snapshots took the
repository to ~18 GB (17.20 GiB loose / 4,515 objects). Every significant git
operation in the workspace became a memory hazard, and agent dispatches — which
run inside a 12 GiB cgroup scope — were the first casualties.

### 2.2 The bead's own window (all times UTC)

| Time | Event |
|---|---|
| 2026-08-13 21:16:00 | `bf-65lsdu` created (P2): "Execute git gc --aggressive to pack the 17GB of loose objects" |
| 2026-08-13 21:22:23.216 | **First kill** — exit −1, 149.4 s into the attempt |
| 2026-08-13 (rest of day) | **127 kills**; last at 23:59:58.067. One `worker.handling.timeout` (23:41:35) |
| 2026-08-14 00:01:47 → 00:40:11 | **36 more kills** (exit −1) |
| 2026-08-14 00:48:35 → 01:10:58.163 | 6 exit-1 failures; last completion of the storm |
| 2026-08-14 01:11:11.824 | **Quarantined** (failure_count 5/5) — the only loop-breaker of the era |
| 2026-08-17 00:32 | Bead split into 3 sequential children (`domchk-bdb1fedf` → `domchk-af4b5ef4` → `domchk-87be56d8`) |
| 2026-08-17 00:34:00.391 | Cleanup attempt records **exit 0 / 90.267 s** (trace metadata) — this was the *split orchestration*, not the gc itself; the gc ran under the children |
| 2026-08-17 00:45:33.228 | Parent `bf-65lsdu` **Closed** (rev 5). Work never lost |

### 2.3 Shape of the storm

- 174 dispatches, 169 completions: **163 × exit −1 (classified `crash`)** + 6 ×
  exit 1 (5 classified `failure`; the 6th lost its classification to the handling
  watchdog that fired 2 ms later).
- **Kill latency:** 29.7 s – 369.7 s; **158 of 163 kills under 120 s** — deaths in
  agent startup and early git work. The workers never reached the packing they
  were dispatched to perform.
- **Alerts:** exactly one ALERT bead per kill ("ALERT: Agent crash on bead
  bf-65lsdu"). Every alert's `Timestamp` is the crash-handler **heartbeat seconds
  after the real kill** (first alert `bf-ncs0ev` stamped 11.94 s after the first
  kill) — alert timestamps are not kill instants.

### 2.4 Workspace-local, not fleet-wide

Same worker, same day (Aug-13): 395 agent completions = **344 × exit −1** / 22 ×
timeout-124 / 18 × exit 0 / 11 × exit 1 — with bf-65lsdu the day's largest single
victim (127, ahead of bf-1ea4g 56, bf-4k2ws 55, bf-2ildm 38, bf-1s6c3 22). Five
peer workers on healthy repositories, same box and day, lost **≤2** exit −1 each.
The trigger was this workspace's bloat × this task's unbounded git operation.

### 2.5 Where older documents are superseded

| Earlier document | Superseded figure | Current finding |
|---|---|---|
| Canon RCA (2026-09-02) §1.3 | "~123 alert beads" | **163** — the Aug-14 continuation postdates that RCA's Aug-13-only window |
| First investigation (2026-08-17) | one crash at 21:27:56Z; "exit −1 (signal -1, SIGKILL)" | 163-kill storm; 21:27:56Z is attempt 5's heartbeat; −1 is a sentinel, not a signal |
| First investigation (2026-08-17) | completion "commit 5bf23b7 (752M, 22 loose)" | **5bf23b7 is absent from the object store** (pre-squash era) — the verifiable records are the resolution doc and the live 105 MB state |
| Retrospective crash report (2026-09-02) | "11 SIGKILL events / 13 alert beads / 11 wasted dispatches" | 163 kills, 163 alert beads, 169 failed dispatches |
| Retrospective crash report (2026-09-02) | R7 "THE OPEN ITEM" (daily gc timer dying `status=127`) | **Resolved** — `domain-check-git-gc.service` `Result=success` exit 0, last fired 2026-09-09 03:00:10 EDT |

---

## 3. Root cause analysis

### 3.1 Verdict

**RESOURCE ISSUE — memory-cgroup (memcg) OOM kill regime.** Not a process error,
not max-turns, not a caught signal, not SIGHUP, not a service outage, and **not a
domain-check code defect**.

### 3.2 Causal chain (four layers)

1. **Ultimate cause — committed bead state.** Repeated ~237 MB `.beads/*.jsonl`
   snapshots bloated the repo to ~18 GB. Every git operation became memory-heavy.
2. **Proximate cause — the task was the death operation.** Bare
   `git gc --aggressive --prune=now` over a 17.2 GiB loose set maximizes
   pack-objects memory, and no `pack.windowMemory` bound existed until
   2026-09-02. It ran uncapped inside the 12 GiB dispatch scope (re-read live
   from an in-flight scope: `memory.max = 12884901888` exactly).
3. **Re-dispatch amplifier.** crash → alert bead → release → re-claim re-fired
   the byte-identical prompt into the identical constraint 163 times until
   quarantine stopped it. Retry was structurally futile: nothing changed between
   attempts.
4. **Kill latency locates the deaths.** 158/163 kills under 120 s — memory
   pressure crossed the limit during startup/early git work, before aggressive
   packing could ever finish.

### 3.3 Mechanism evidence and confidence

- **No kernel record can exist for the kill window:** journald holds a single
  boot whose first entry is 2026-08-15 19:56:33 EDT; the kills span Aug-13/14.
  For *these* kills the mechanism is therefore **chain-inferred, MEDIUM-HIGH
  confidence** (regime match), not kernel-proven.
- **Kernel-proven for the era class** from 2026-08-16 onward: journal records
  `Memory cgroup out of memory: Killed process (git) ... anon-rss:12301364kB`
  (~12.3 GB) inside a needle `run-p*.scope` — the same standing condition, same
  store, same scope class, two days later.
- **Exit-code semantics:** `−1` is the harness's abnormal-child-death sentinel.
  A caught signal would encode 129/130/143; SIGKILL 137; timeout 124 (22 of the
  latter on this worker the same day). The silent, traceless death signature —
  no stack, no stderr beyond the dispatch-scope line — corroborates SIGKILL-class
  death.

### 3.4 Reproducible or transient?

**Systemic to the bloat era** (163 identical kills of one bead is the opposite of
transient), and **mechanically closed today**: the store is repaired and the
death operation is now a tested negative — `test-gc-memory-bounds.sh` re-runs the
bare `git gc --aggressive --prune=now` inside a 768 MiB cgroup and it completes
bounded. Live re-check for this report: repository healthy, bounds verified (§5).

### 3.5 Record-integrity footnote

`bf-65lsdu`'s close reason cites commit `5bf23b7`, which is **absent from the
object store** (pruned in the pre-squash era). Nothing to fix in the closed bead;
noted so nobody chases the SHA. The verifiable outcome records are
[cleanup-resolution-2026-08-17.md](../cleanup-resolution-2026-08-17.md)
(17.20 GB → 753 MB / 118 loose) and the live 105 MB state, with the verified
2026-09-01 cleanup as the canonical final state
([bf-65lsdu-cleanup-verification.md](../verification/bf-65lsdu-cleanup-verification.md)).

---

## 4. Recommendations and next steps

Nothing on this chain is owed beyond verification-and-close behavior on re-fires.
In priority order:

1. **Do not re-run bare gc, ever.** Use `./scripts/safe-git-gc.sh` (staged,
   checkpointed, memory-bounded). The bare `git gc --aggressive --prune=now` path
   is bounded by persistent config today, but the safe script remains the
   sanctioned entry point.
2. **Keep `.beads/` untracked.** The whole directory is gitignored plus a 10 MB
   pre-commit gate; never re-track bead state — that is how the 18 GB era began.
3. **Treat a deterministic failure as a shape problem, not a retry problem.** If
   a dispatch fails identically twice, change the work's shape (decompose) or
   bound the resource. The crash-storm breaker (trip at crash #3 vs the
   historical 163) now enforces the stop mechanically.
4. **Re-validate prevention on a schedule, not from its landing commit.** The
   2026-09-02 retrospective found the daily gc timer dying `status=127` in 3 ms
   while everything *looked* installed. Re-run the battery in
   [docs/crash-prevention-validation.md](../crash-prevention-validation.md)
   before citing any "prevention in force" claim, and remember the
   `daemon-reload` gotcha after editing any systemd user unit.
5. **Alert hygiene for sibling storms.** Other bloat-era alert families (bf-31mno
   was larger at 434) should keep being dispositioned through their own
   verify-then-close chains — never re-investigated — using the per-alert-type
   runbooks in [docs/crash-response-guide.md](../crash-response-guide.md).
   Start from target-bead state and `git log --all --grep <bead-id>`, not from
   the alert's Timestamp.
6. **One open bead remains in this chain's namespace (flagged, not touched):**
   `domchk-e42fe292` — "Rank signal −1 root causes for the bf-65lsdu crash by
   probability" (Open since 2026-09-02). Its research question is answered by the
   canon RCA (OOM kill regime ranked first; SIGHUP cascade ruled out). Its own
   chain should verify-then-close it; this document leg does not own it.
7. **Parent bead status:** verified, no update needed — `bf-65lsdu` is Closed
   (rev 5, 2026-08-17T00:45:33.228Z), as are all three split children
   (`domchk-bdb1fedf` rev 7, `domchk-af4b5ef4` rev 4, `domchk-87be56d8` rev 3).

---

## 5. Preventive measures — status verified live 2026-09-09 (this dispatch)

| # | Layer | Blocks which cause | Live verification (this dispatch) |
|---|---|---|---|
| 1 | `.beads/` gitignored + 10 MB pre-commit hook | Ultimate cause — bloat accumulation | `git ls-files .beads` = **0 tracked**; hook current |
| 2 | Persistent pack-memory bounds: `pack.windowMemory=2g`, `deltaCacheSize=1g`, `threads=1` (repo-local **and** global) + `gc.auto=0` | Proximate cause — uncapped pack-objects on bare gc **and** push | `setup-git-gc-config.sh --verify` **exit 0** — worst-case pack memory ≈3072 MiB, within the 12 GiB scope |
| 3 | `safe-git-gc.sh` + crash-safe `cleanup-bloat.sh` at HEAD | The death operation itself | No bare aggressive-gc path at HEAD; bounded replay test green |
| 4 | Crash-storm circuit breaker + `needle-with-limiter.sh` dispatch gate | Re-dispatch amplifier — the ×163 loop | Breaker status `{}` (no beads in backoff); its regression replay converts this shape into 3 dispatches / 1 alert |
| 5 | Monitoring: 8 `domain-check-*` systemd user timers | Silent re-accumulation | **8/8 future-triggered**; `domain-check-git-gc.service` `Result=success` exit 0 (2026-09-09 03:00 EDT) |
| 6 | Crash-alert hardening (closed-bead filter, dedup, 300 s cooldown, classification) | Aftermath false-positive storm | 163/163 alert beads closed, **0 open** |

Live repository state at writing: `.git` **105 MB** · 34 loose objects / 228 KiB ·
1 pack / 100.70 MiB · 0 garbage · `check-repo-health.sh` **exit 0** · **0/0**
divergence from origin.

---

## 6. Verification record (2026-09-09, dispatch domchk-8114156d)

Every load-bearing figure above was re-verified first-hand this dispatch from
primary sources, not cited from any leg's notes:

- **Kill ledger re-counted** from the raw worker logs
  (`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-1{3,4}.jsonl`,
  JSON-parsed `agent.completed` events): Aug-13 = **127 × exit −1** (first
  21:22:23.216Z, last 23:59:58.067Z); Aug-14 = **36 × exit −1 + 6 × exit 1**
  (last completion 01:10:58.163Z) → **163 kills + 6 failures**; 169 completions,
  168 classified (163 `crash` + 5 `failure`).
- **Alert census re-counted live** from the bead store: **163** ALERT beads titled
  for bf-65lsdu, **163 closed, 0 open**; plus 14 non-alert chain/doc beads — 13
  closed, 1 open (`domchk-e42fe292`, flagged in §4.6).
- **Target and children re-read:** `bf-65lsdu` Closed rev 5 (2026-08-17T00:45:33Z);
  split children `domchk-bdb1fedf` (rev 7) / `domchk-af4b5ef4` (rev 4) /
  `domchk-87be56d8` (rev 3) all Closed; trace metadata re-read — exit 0 /
  outcome success / 90,267 ms / captured 2026-08-17T00:34:00.391Z.
- **Input legs re-read:** `domchk-4511fe51` (gather, Closed rev 5) and
  `domchk-79863691` (analyze, Closed rev 4) — this document's §2/§3 synthesize
  their notes and agree with them on every figure.
- **Live health:** `check-repo-health.sh` exit 0; `setup-git-gc-config.sh
  --verify` exit 0; breaker `{}`; 8/8 timers future-triggered; git-gc service
  exit 0; 0 tracked `.beads` files; 0/0 unpushed.
- **Docs-only deliverable:** no Go code touched (`go build`/`go test` baseline is
  unaffected; this commit adds one markdown file).

## Sources

- Raw worker logs Aug-13/14 (kill ledger, re-parsed this dispatch) · live bead
  store (`bf-65lsdu`, split children, 163 alert beads, legs `domchk-4511fe51` /
  `domchk-79863691` / `domchk-754711b5`) · `.beads/traces/bf-65lsdu/metadata.json`
- Canon: [RCA signal −1 (2026-09-02)](../research/root-cause-analysis-bf-65lsdu-signal-minus-one-2026-09-02.md) ·
  [chain retrospective (2026-09-09)](../crash-investigations/bf-65lsdu-chain-retrospective-domchk-754711b5-2026-09-09.md)
- Operating: [repository maintenance guide](../maintenance/repository-maintenance-guide.md) ·
  [crash response guide](../crash-response-guide.md) ·
  [crash-prevention-validation.md](../crash-prevention-validation.md)
