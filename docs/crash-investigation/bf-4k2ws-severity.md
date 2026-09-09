# bf-4k2ws — Severity Determination

**Dispatched leg:** severity determination (`domchk-5a0a456a`, template-chain
"document" step)
**Target bead:** `bf-4k2ws` — "Analyze divergent Forgejo and GitHub branch states" —
**Closed** (rev 2, 2026-08-16T15:35:42Z, 8/8 acceptance criteria met)
**Source alert bead:** `bf-4xlwo` (Open, rev 20, re-read live this session) ↔
**attempt 57 of 62**, death 2026-08-13T06:34:29.204954348Z
**Severity:** **REAL problem (INFRASTRUCTURE, repository-bloat-era kill regime) —
with the remediation already landed and verified holding; the alert itself is a
STALE FALSE POSITIVE.** No new remediation actions are owed.
**Determination date:** 2026-09-09

> **What this document is.** The severity-leg deliverable, filed at the
> dispatch-named path. It is **subordinate to the canonical record**:
> `docs/investigations/bf-4k2ws-root-cause-determination-domchk-7f838f36-2026-09-07.md`
> (§§1–17) and `docs/crash-investigations/bf-4k2ws/README.md`. The determination
> was already made and re-verified by the prior legs
> (`domchk-6ba3b2af` gather → `domchk-541f1089` analyze); this document does not
> re-open it — it converts it into a severity verdict and a remediation ledger,
> re-verifying the current state first-hand this session (§2, §5).
>
> **Per the canon's lesson 10** ("a dispatch template is a stale-premise
> vector"), the dispatch's own three-way choice is corrected in §0 before it is
> used.

---

## 0. Template-premise corrections (read first)

| Template premise | As dispatched | Verified fact |
|---|---|---|
| Three-way severity choice — "**False positive**: Agent killed externally but no actual problem" | One label must cover the whole event | **The event has three layers, and the three labels land on different ones** (canon §8.3): the kills were real (55 genuine deaths — "no actual problem" is false at the kill layer); the underlying cause was a real infrastructure defect (≈18 GB store); the *alert* is false in two specific senses — it fired per-kill with no dedup (multiplication), and attempt 57 was **post-completion re-work of already-verified work**, so this alert's target was done before the alert existed (stale). §1 |
| "If false positive … how to prevent future false alerts" | Implies the false-positive question is open | Already answered and implemented: the alert-layer fixes (closed-bead filtering, dedup + processed-alerts tracking, completion awareness, exit-code validation, cooldown) are committed with suites, and the specific mechanism that made *this* alert false (orphaned verified success → post-completion kill) is documented at canon §9.2. §4 |
| "Based on the root cause analysis from domchk-541f1089" | Treats that document as the root-cause source | Correct and current — `docs/crash-investigation/bf-4k2ws-root-cause.md` carries the determination; this leg builds on it and adds no new cause claim |
| Deliverable path `docs/crash-investigation/` (singular) | Implies the corpus home | The corpus home is `docs/crash-investigations/` (plural); created at the named path anyway so the criterion resolves there — subordinate to the canon, same as the two prior legs |

---

## 1. Determination — severity against the three-way choice

### 1.1 The crash regime: **REAL problem — remediation required, landed, verified**

This is not "minor". The night produced **55 genuine mid-run deaths of one bead
in ~5 h 14 min** (62 attempts, one worker session), and the bead is one instance
of a regime that killed across the era: bf-1s6c3 (71), bf-4yjq (50),
bf-173o7e (129), bf-4x12ec (44, kernel-proven), bf-198ne (kernel-proven). The
cause — git-remote-heavy dispatches against the then-**≈18 GB** object store
inside the 12 GiB dispatch scope (`memory.max = 12884901888`), killed by memcg
OOM — is a real infrastructure defect, not noise: it killed *verified,
repeating* work deterministically until the store was repaired.

**But "requires remediation" is satisfied in the past tense.** Every component
of the fix (canon §10.1, R1–R4) was already committed to `origin/main` before
this leg dispatched, and the five-check battery re-ran **green this session**
(§2). The severity of an already-remediated, verified-holding infrastructure
defect is: **real problem — remediated and holding.**

### 1.2 This specific alert (`bf-4xlwo` ↔ attempt 57): **STALE FALSE POSITIVE**

The alert is false in both documented senses, neither of which impugns the kill:

1. **Multiplication** — one of 55 ALERT beads for one cause: pre-dedup needle
   (< 0.4.2) filed one bead per kill, no fingerprint/cooldown.
2. **Stale target** — attempt 57 fell in the post-completion window
   (attempts 34–61): it re-ran work whose **first success had already passed
   verification** (04:48:09.546Z, orphaned 5.8 s later). The target closed
   **2026-08-16, three days after the storm**, with all 8 acceptance criteria
   met and four deliverable docs on `origin/main` (presence re-verified via
   `git cat-file -e` this session:
   `divergence-analysis-bf-4k2ws-2026-08-13-pre-merge.md`,
   `branch-divergence-bf-4k2ws-2026-08-13.md`,
   `branch-divergence-analysis-bf-4k2ws-current.md`,
   `branch-divergence-analysis.md`). Branch divergence is 0/0 and holding
   (canon §8.3). **No work was lost**; the alert fired for work that was
   already done and verified once.

`bf-4xlwo` itself remains Open (rev 20, re-read live this session) and unowned —
stale by the standard disposition (target closed, work shipped, canon §7.1):
**it owes a disposition, not an investigation.**

### 1.3 Why "minor issue" fits neither layer

"Recoverable, no action needed" describes neither: the kill regime needed
substantial action (and got it — repo repair, bounded gc, gates), and the alert
layer needed its own fixes (and got them). What *is* left is neither a severity
nor a remediation: a short list of **open items with named owners** (§3.2) and
a disposition for the stale alert bead.

---

## 2. Required remediation steps — **none new; the ledger, verified live 2026-09-09**

Every kill-mechanism layer was re-run green by this bead this session
(`bf-4k2ws` worktree, HEAD `975cc31` = `origin/main`, 0 unpushed):

| # | Layer (canon §10.1) | Live verification this session |
|---|---|---|
| R1 | Pack memory bounded inside the 12 GiB scope (gc **and** push-side pack-objects) | `setup-git-gc-config.sh --verify` → exit 0; effective chain system→global→local, worst case **≈3072 MiB** within the ceiling |
| R2 | Object store must not re-bloat (`.beads/` gitignored, 10 MB pre-commit gate) | `check-repo-health.sh` → exit 0; **`.git` 106 MB**, 0 unpushed backlog |
| R3 | Remediation bounded and unconditional (safe-gc + daily/weekly timers) | health check reports no unmanaged aggressive gc running |
| R4 | Heavy work gated on environment | `preflight-health-check.sh` → exit 0 (4/4); `system-event-mode.sh check` → exit 0 "clear"; `crash-circuit-breaker.sh status` → exit 0, no open entries |

The regime's two kill legs are covered by different halves: the 55 mid-run
kills by R1+R2 (bound the allocation, keep the store small), the 5 × exit 124
600 s-cap deaths by nothing repo-side — that cap is a NEEDLE work-time knob
(G-11/G-12), and the correct repo-side response is the R4 gate.

**Actions this determination itself requires: none.** Specifically **not** owed:
no repository action (healthy at 106 MB), no gc re-tuning (anti-requirement 2,
canon §10.3), no Go/code change (zero domain-check defects across 157+
investigations — the storm never touched the application), no new
bf-4k2ws-scope document beyond this one, and no re-opening of the closed target.

## 3. Prevention recommendations for future alerts

### 3.1 In force (prevents recurrence of *this* regime and alert class)

- **Regime:** R1–R4 above, holding; dispatch entry point
  `scripts/needle-with-limiter.sh` so a bead in storm backoff is deferred out of
  the ready frontier instead of re-dispatched into the same crash.
- **Alert layer:** closed-bead filtering (fixes 1 & 5, including the
  `bf-29rca`-shape target-closure gate, `8cc1172`), duplicate detection +
  processed-alerts tracking (2 & 3), completion awareness + exit-code
  validation (4 & 6), 5-minute cooldown; suites
  `test-crash-alert-fixes.sh` / `test-closed-bead-filter.sh` green at recent
  HEADs per CLAUDE.md's verification record.

### 3.2 Open items — with owners; **not this chain's work** (canon §10.2)

| Item | Owner / vehicle |
|---|---|
| G-3 adoption half — actually call `system-event-mode.sh` from `crash-alert-manager.sh` / `preflight-health-check.sh` | **`domchk-6951fe0c`** — do not duplicate |
| Alert-layer D-1..D-10 (dedup knobs exist; pipeline never fired in production) | `domchk-b5448b6a`'s prioritized fix list; repo-boundary caveat: needle owns bead creation |
| G-9 work-completion detection at the alert source, G-10 dispatch-scope sizing, G-11 retry/backoff, G-12 turn budgets, G-13 CPU throttling | NEEDLE repo — Phase 3 of `docs/crash-prevention-requirements.md` |

### 3.3 How to prevent future alerts of this shape (the task's false-positive ask)

The reason *this* alert was false is a **capture-then-stale** pattern: the kill
was real, but the work had already succeeded and the loop re-ran it. The
standing countermeasures, in the order a future responder should apply them:

1. **Verify the target bead's actual state before investigating** — read the
   bead, its deliverable, and `git log --all --grep <bead-id>` first; most
   alerts point at work another worker already finished.
   For this event the entire downstream alert pool (~180 beads) is stale by
   exactly this test.
2. **Read the alert timestamp as a heartbeat, never a death** — `bf-4xlwo`'s
   "Timestamp" is `HANDLING_RELEASE_DONE` (death + 6.600 s); the kill is the
   preceding `agent.completed`.
3. **Treat an exit-−1 sentinel as a class, not a signal** — and never from it
   name SIGTERM/SIGHUP; classify from the worker log (canon §3.6).
4. **Never re-fire a template chain at a closed target** — the classification
   (`docs/crash-response-guide.md` FALSE_POSITIVE, INFRASTRUCTURE) is already
   made; re-fired legs verify and disposition, they do not re-investigate.

---

## 4. Acceptance-criteria mapping

| Criterion | Answer | Where |
|---|---|---|
| Review the root cause analysis from domchk-541f1089 | Reviewed and adopted — `docs/crash-investigation/bf-4k2ws-root-cause.md`; its determination (INFRASTRUCTURE resource limit, memcg-OOM SIGKILL, chain-inferred MEDIUM-HIGH) is carried, not re-derived | §1.1 |
| False positive / minor / real | **Real problem (INFRASTRUCTURE regime) — already remediated and verified holding; the alert itself is a STALE FALSE POSITIVE (post-completion kill, target closed 8/8 three days later, zero work lost)** | §1 |
| If remediation needed, identify specific actions | None new — R1–R4 all landed pre-dispatch and re-verified green this session; genuinely open items have owners (domchk-6951fe0c, domchk-b5448b6a, NEEDLE G-9..G-13) | §2, §3.2 |
| If false positive, document why + prevention | Why: 55-per-cause multiplication (pre-dedup needle) + stale target (post-completion re-work of a verified success); prevention: alert-layer fixes in force, four-step responder order | §1.2, §3.1, §3.3 |

**Deliverable:** this document — severity classification, remediation ledger,
prevention recommendations.

---

## 5. Sources

- Prior legs: `docs/crash-investigation/bf-4k2ws-root-cause.md` (`domchk-541f1089`), `docs/crash-investigation/bf-4k2ws-crash-context.md` (`domchk-6ba3b2af`)
- Canon: `docs/investigations/bf-4k2ws-root-cause-determination-domchk-7f838f36-2026-09-07.md` (§8.3 three-layer semantics, §10.1 R1–R5 + §10.2 open items, §10.3 anti-requirements); `docs/crash-investigations/bf-4k2ws/README.md`
- Live reads this session: `bead show bf-4k2ws` (Closed rev 2), `bead show bf-4xlwo` (Open rev 20), `git cat-file -e origin/main:<doc>` ×4 (all four deliverables present), five-check battery (repo health exit 0, `.git` 106 MB / 0 unpushed, gc bounds ≈3072 MiB exit 0, preflight 4/4, surge gate clear, breaker no open entries)
- Alert-layer verification record: repo `CLAUDE.md` "Crash Alert System" (suites 13/13, 7/7; cascade replay 10/10 at HEAD `d23f24d`)
