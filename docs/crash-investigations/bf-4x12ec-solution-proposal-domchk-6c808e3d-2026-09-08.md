# Solution Proposal — bf-4x12ec agent crash

> **Bead:** `domchk-6c808e3d` ("Propose solution for agent crash") — solution leg
> of the bf-4x12ec family. **Date:** 2026-09-08. **Input:** the verified root
> cause analysis, closed as `domchk-00fec118` (commit `382b976`), whose
> deliverables are [`docs/crash-reports/bf-4x12ec-git-gc-crash.md`](../crash-reports/bf-4x12ec-git-gc-crash.md)
> (§Root Cause), `docs/notes/bf-4x12ec-crash-investigation.md`, and
> [`docs/research/root-cause-analysis-signal-minus-one-crashes.md`](../research/root-cause-analysis-signal-minus-one-crashes.md).
> This document proposes the solution; it does not re-derive the RCA.

---

## 1. What we are preventing (RCA review)

**Verified mechanism:** `git gc --aggressive --prune=now`, run by an agent
dispatch against a repository holding 4,649 loose objects / 17.20 GiB, drove
the anonymous memory of needle's transient `run-p*.scope` past its
`MemoryMax=12 GiB`, and the kernel's cgroup OOM killer (`CONSTRAINT_MEMCG`)
SIGKILLed a task in the scope — on Aug-14 the agent dispatch task itself, which
is why needle recorded the harness sentinel `exit_code=-1` rather than a git
exit code. 44 identical kills over 64 minutes bought zero packing progress
(aggressive gc builds delta chains in memory before writing any pack byte);
the work finished only when needle's auto-split decomposed it (~96 min in) and
the 53rd attempt exited 0.

The RCA's causal chain has five distinct links. A solution must be judged
against all five, because fixing only the first still leaves the shape that
re-fired this incident as ~60 alert beads:

| # | Causal link | Evidence anchor |
|---|---|---|
| F1 | Repository bloat armed the trap: 17.20 GiB loose objects (committed `.beads/*.jsonl` snapshots) made any aggressive git operation memory-lethal | crash-window transcript, 10:21:23Z |
| F2 | The hazard was **in the task text** — the bead body itself prescribed the bare lethal command, authored as a mitigation *for* bloat | bead body |
| F3 | The binding memory constraint was the **dispatch scope**, not the host: 12 GiB `MemoryMax`, while the host held ~45 Gi free throughout | scope verified live; mid-storm capture 10:43:59Z |
| F4 | `memory.oom.group=0` → per-task victim selection, so *which* task died was nondeterministic (agent task Aug-14, `git` Aug-16) while *why* stayed constant | commit `89c66af` |
| F5 | **Retry storm amplifier:** needle re-claimed and re-ran the identical deterministic kill 44×, then released the finished bead *orphaned* instead of closing it — so false-positive alerts regenerated for days | needle event log; `bead.orphaned` 12:58:55Z |

Evidence limit carried over from the RCA (and respected here): no kernel line
survives for Aug-14 (boot began Aug-15). The verdict rests on the retry-storm
signature plus direct kernel evidence of the identical mechanism 257× in the
Aug-16 window. That perished-evidence problem is itself a solution gap — §4,
item 1.

## 2. Solution approach — one control per causal link

The strategy is **defense in depth with one owner per link**, not a single
fix: F1 is owned by repository hygiene (nothing may re-arm the bloat), F2 by
bounding the command itself so the hazard is inert even when re-prescribed,
F3/F4 by sizing work to the scope rather than the scope to the work, and F5 by
gates that stop re-dispatch into a deterministic kill. Decomposition — the
auto-split that actually ended the incident — remains the preferred escape:
fit each piece inside an individual scope budget rather than raising limits.

| Link | Control | Owner | Status |
|---|---|---|---|
| F1 bloat | `.beads/` + `*.jsonl` + `*.db` gitignored, 0 tracked; 10 MB pre-commit size gate (tracked installer + self-test); repo-health thresholds + daily 02:00 check | this repo | **In force** — verified live §3 |
| F2 lethal command | `scripts/safe-git-gc.sh` (soft `SAFE_GC_MEMORY_MAX` → `pack.windowMemory`, hard `SAFE_GC_CGROUP_MAX` ceiling, `ulimit -v` fallback, fail-fast preflight exit 2, checkpoint/resume) **plus** persistent git config `pack.windowMemory=2g` / `pack.deltaCacheSize=1g` / `pack.threads=1` applied repo-locally **and** globally, so the *bare* gc/push path is bounded too — worst case ≈3,072 MiB against the 12 GiB scope | this repo | **In force** — verified live §3, death-op replay §3 row 12 |
| F3/F4 scope budget | Size work to the scope: bounded pack memory (row above), staged multi-pass cleanup (the `bf-173o7e`/`bf-5jhvpk`/`bf-im2sl1` decomposition), never read host free RAM as scope headroom; dispatches enter through `scripts/needle-with-limiter.sh` (breaker + concurrency gate) and `preflight-health-check.sh` | this repo (convention) + NEEDLE (scope policy, §4 item 4) | Partially in force — the repo-side convention is live; uniform 12 GiB scopes remain a NEEDLE-side gap (G-10) |
| F5 retry storm | `scripts/crash-circuit-breaker.sh` (trip at threshold, dispatch blocked exit 4, half-open probe, 24 h decay) + `scripts/system-event-mode.sh` surge gate (crash-burst / synchronized-exit-wave / PSI; 0/75/4/2 contract) + alert layer (closed-bead filter, dedup + 7-day ledger, cooldown, classifier) + close gate `verify-work-completion.sh` so finished work is never orphaned | this repo | **In force** — verified live §3; production *invocation* gap noted §4 item 5 |
| F5 alert regeneration | Closed-bead filtering + target-resolution gate; dedup ledger; surge suppression | this repo | **In force** — verified live §3 |

Two design rules from the RCA are load-bearing for everything above and are
restated here because they are the parts most likely to be violated under
pressure:

1. **Bound the command; do not raise the scope.** The host had 45 Gi free for
   the entire storm and every attempt died anyway. A larger `MemoryMax` would
   have moved the kill threshold, not removed it, and `oom.group=0` means the
   victim would still be arbitrary.
2. **Stop retrying deterministic failures — decompose.** Same command + same
   repo state + kill inside ~2 minutes is a signature, not bad luck. The
   breaker exists because needle's own auto-split took ~96 minutes to engage.

## 3. Live re-verification (this bead, 2026-09-08)

Every "in force" claim above was re-executed today per
[`docs/crash-prevention-validation.md`](../crash-prevention-validation.md) ("a
safeguard is in force only on the day you ran its validation"). Scope budget
read from cgroupfs (kernel-enforced ground truth — note `systemctl show`
reported `MemoryMax=infinity` for the same scope; trust `memory.max`):

```
run-p1222298-i252848300.scope: memory.max=12884901888 (12 GiB)  memory.oom.group=0
needle.slice: max=34359738368 high=25769803776   user-1001.slice: max=51539607552
```

| # | Check | Result |
|---|---|---|
| 1 | `./scripts/check-repo-health.sh` | exit 0 — comprehensive check complete, 0 unpushed |
| 2 | `./scripts/setup-git-gc-config.sh --verify` | exit 0 — effective local bound, worst case ≈3,072 MiB within the 6 GiB ceiling for a 12 GiB scope |
| 3 | `./scripts/setup-git-hooks.sh --check` | exit 0 — installed, byte-identical to tracked source |
| 4 | `git ls-files .beads \| wc -l` | `0` — the bloat source stays untracked |
| 5 | `./scripts/auto-gc-trigger.sh --dry-run` | exit 0 — GC not needed |
| 6 | `./scripts/check-unpushed-backlog.sh` | exit 0 — CLEAR (0 < 50) |
| 7 | `systemctl --user list-timers 'domain-check-*'` | 8/8 with future `NEXT` |
| 8 | `./scripts/system-event-mode.sh check` | exit 0 — `STATE clear` |
| 9 | `./scripts/crash-circuit-breaker.sh status` | `{"beads": {}}` — nothing latched |
| 10 | `./scripts/resource-monitor.sh --once` / `preflight-health-check.sh` | exit 0 / exit 0 — pressure 0%, `UNSAFE_GC: none`; disk 24 GB free flagged `[WARNING]` (above the 20 GB critical floor, below the 30 GB warn line — headroom worth reclaiming before any full-gc window) |
| 11 | `test-crash-alert-fixes` / `test-system-event-mode` / `test-crash-circuit-breaker` / `test-needle-with-limiter-gate` / `test-concurrency-limiter` | 13/13, 32/32, 18/18, 25/25, 13/13 — all exit 0 |
| 12 | **Death-operation replay** `bash scripts/test-gc-memory-bounds.sh` | **17/17, exit 0** — both historical kill operations re-run inside a 768 MiB cgroup: bare `git gc --aggressive --prune=now` exits 0 (pack-objects peak RSS 320,432 KB) and the bounded `git push` over an unpacked backlog exits 0 (peak 232,428 KB). The mechanism that killed 44 attempts cannot recur while the pack bounds hold |

## 4. Residual gaps and implementation steps

The crash class itself is closed repo-side: all four prevention
recommendations in the RCA report are live (§3). What remains open is the
*amplifier* and the *economics* — detection, evidence, and NEEDLE-side
retry policy — ordered here by how much future agent-time each one costs.

1. **Evidence retention (G-8, P6).** The Aug-14 kernel lines did not survive
   boot rotation; this incident's RCA is inference-plus-corroboration because
   of it. *Steps:* retain journald/kernel records ≥30 days; retain
   worker/session transcripts ≥30 days (the highest-value line — transcripts
   prove what the agent was doing when it died); rotate `.beads/traces/`
   instead of overwriting; UTC-only timestamps. Requirement text:
   [`docs/crash-prevention-requirements.md`](../crash-prevention-requirements.md) §4 G-8 + M-3 extension.
2. **Retry-loop stop condition (NEEDLE-side, H-1).** Nothing in the *retry
   path* asks "did the last N attempts die identically?" before re-claiming —
   the exact behavior that converted one kill into 44. The repo's breaker
   bounds the blast radius from outside; the fix belongs where the re-claim
   decision is made. *Steps:* in NEEDLE's dispatch loop, fingerprint each
   failure (exit sentinel + duration bucket + bead) and stop/defer after N
   identical fingerprints; expose the fingerprint to the alert producer so the
   44 clones collapse to one.
3. **Consult work-completion state at the alert source (G-9, NEEDLE-side).**
   The orphaned release at 12:58:55Z kept alerts regenerating until the manual
   close. `verify-work-completion.sh` writes the marker; the alert *producer*
   never reads it. *Steps:* before raising an alert, consult the marker + a
   30 s post-completion grace period.
4. **Dispatch-scope sizing policy (G-10, NEEDLE-side).** Uniform 12 GiB scopes
   are survivable *only* because this repo pre-bounds pack memory; any
   non-git heavy workload (large test runs) remains unprotected. *Steps:*
   scope-size by workload class, or expose a heavier scope opt-in gated on the
   preflight — keep preferring decomposition over bigger scopes.
5. **Production invocation of the alert manager.** Nothing in this repo calls
   `crash-alert-manager.sh` on a timer or hook; the suites prove it works, not
   that it runs (the alert producer is NEEDLE-side). *Steps:* either wire a
   systemd user timer to sweep open crash alerts through the manager, or
   record the decision that alert handling stays operator/agent-initiated —
   the current undefined state is what lets a dead code path look like
   prevention.
6. **Documentation debt, this crash class.** `docs/git-gc-mitigation-strategy.md`
   (2026-08-28, status "PROPOSED") still frames `exit -1` as SIGHUP and treats
   the pack-memory safeguards as proposals; both are superseded by the RCA
   (`-1` is a harness sentinel; the safeguards are live per §3). *Steps:* add
   a superseded-banner pointing at the RCA and
   `crash-prevention-requirements.md`, or fold its still-valid content into
   the maintenance guide and archive it. (Not edited by this bead — separate
   ownership; recorded here so it does not survive another sweep.)
7. **Still-open requirements for *other* crash classes** (not this RCA's
   scope, listed to keep the gap list in one place): G-4 gateway failover +
   enforced preflight, G-5 prevention feedback loop,
   G-13 CPU-saturation throttling —
   [`docs/crash-prevention-requirements.md`](../crash-prevention-requirements.md) §4–5.

## 5. Monitoring and prevention measures (standing)

**Detection of a recurrence** would flow through the already-installed layer
chain: resource monitor every 5 min (pressure / unsafe-gc) → crash-pattern
detector every 10 min (surge = 3 crashes in 5 min) → surge gate + breaker
(defer new dispatch; block re-dispatch of latched beads) → alert pipeline
(classify, dedup, closed-bead filter). An operator or agent responds per
[`docs/crash-response-guide.md`](../crash-response-guide.md) runbook A/D/F.

**The standing prevention check** is the per-layer battery in
[`docs/crash-prevention-validation.md`](../crash-prevention-validation.md) —
re-run the applicable layer before citing any "in force" claim (the smallest
form, ~1 min):

```bash
./scripts/check-repo-health.sh && ./scripts/setup-git-gc-config.sh --verify \
  && ./scripts/preflight-health-check.sh \
  && ./scripts/system-event-mode.sh check && ./scripts/crash-circuit-breaker.sh status
```

**Honest limits of this solution** (inherited from the validation guide's
scope note): the battery proves bounds and wiring, not fleet behavior during a
real storm; NEEDLE-side behavior (retry stop-condition, scope policy) has no
suite here; and detection without an actor is not prevention — hence §4
items 2, 3, and 5.

## 6. Acceptance criteria mapping

| Bead criterion | Where satisfied |
|---|---|
| Review the root cause analysis | §1 (five-link causal chain + evidence limit), input = closed `domchk-00fec118` |
| Propose a concrete fix or mitigation strategy | §2 (control-per-link table + two design rules) |
| Document the solution approach | §2 + §5 (this document) |
| Outline implementation steps | §4 (ordered, with owners; repo-side class closed, NEEDLE-side steps enumerated) |
| Consider monitoring/prevention measures | §3 (live re-verification) + §5 (detection chain, standing battery, honest limits) |
