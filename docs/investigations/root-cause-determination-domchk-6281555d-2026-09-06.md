# Root Cause Determination — 2026-09-01 Crash Investigation

**Bead:** domchk-6281555d (RCA; step 2 of the 4-way auto-split chain created 2026-09-01T19:38:42Z)
**Chain:** domchk-20dc36b4 (parent report) → domchk-59e1f1d5 (evidence compilation, closed) →
**domchk-6281555d (this document)** → domchk-ece81e17 (solutions) → domchk-7880ada7 (finalize)
**Evidence base:** [`evidence-compilation-2026-09-01-crash-investigation.md`](evidence-compilation-2026-09-01-crash-investigation.md)
(domchk-59e1f1d5, commit a7aefac) — every figure below re-verified live on 2026-09-06 (§7)
**Investigation period:** 2026-08-12 → 2026-09-05
**Classification:** INFRASTRUCTURE (cgroup-scoped OOM) + TOOL/DETECTION defect — **not a code defect**
**Confidence:** HIGH for the mechanism ([LIVE] kernel + needle records); MEDIUM only for the
2026-08-14 event's internal decomposition, whose primary logs are [GONE] and rest on the committed corpus

---

## Executive Summary

**Confirmed root cause: unbounded-memory processes running inside per-dispatch systemd scopes whose
`MemoryMax=12 GiB` is the binding constraint.** When the process set inside a dispatch scope —
`git pack-objects` under a bare `git gc --aggressive --prune=now`, or a `node`/vitest run — allocates
past 12 GiB, the kernel's memory-cgroup OOM killer delivers an **uncatchable SIGKILL**
(`constraint=CONSTRAINT_MEMCG`). The agent dies with no Go panic and no stack trace, and needle
records the death through `code().unwrap_or(-1)` as `exit_code=-1` — a **sentinel for an unrecorded
signal death**, not a signal number and not SIGHUP.

Two conditions made the bound reachable: **repository bloat** (18 GB, 17.16 GB loose objects at
bf-1s6c3, 2026-08-12 — 36× a healthy ~500 MB) supplied the unbounded input, and **naive redelivery**
(a duplicate gc bead dispatched 132 times on Aug-14, 129 attempts dying `exit -1` with flat kill
durations) supplied the repetition. Every kill record that survives reads `CONSTRAINT_MEMCG` inside a
`needle.slice/run-p*.scope` or `app.slice/run-p*.scope` cgroup — **the host was never out of memory**,
which is why host-wide memory alerting caught none of it.

A second, independent cause — **needle's crash detection lacking completion detection, self-healing
awareness, and deduplication** — did not kill anything, but converted a bounded set of real
infrastructure events into 200+ alerts, ~60% of them duplicates, and spawned the very auto-split chain
this document belongs to.

**Bottom line:** no domain-check defect exists anywhere in the evidence (157+ investigations, zero
code findings, no Go panic ever recorded). The signal-death era ended 2026-08-26; since then the
failure class is synchronized `exit_code=1` service waves, a different mechanism entirely.

---

## 1. Confirmed root cause

### 1.1 The kill path (data flow)

```
needle dispatch
  └─ systemd scope  user@1001.service/needle.slice/run-p<id>.scope     MemoryMax = 12 GiB
       └─ agent process (claude-code worker)
            └─ shell: git gc --aggressive --prune=now                  ← unbounded command
                 └─ git pack-objects                                   ← RSS ∝ object set
                      ▲  repo = 18 GB total / 17.16 GB loose objects   ← the amplifier (bf-1s6c3)
                      │
                      │  anon-rss climbs to ≈12.0 GiB  (= the scope bound; largest observed
                      │  victim 12,555,188 kB)
                      ▼
             kernel memcg OOM:  oom_kill_process → SIGKILL
                 constraint=CONSTRAINT_MEMCG, oom_memcg=…/run-p<id>.scope
                      ▼
             SIGKILL is uncatchable → agent dies with NO Go panic, NO stack trace
                      ▼
             needle: exit status unavailable → code().unwrap_or(-1)
                 ⇒ bead.outcome: exit_code=-1  outcome=Crash(-1)         ← the sentinel
                      ▼
             needle crash alert (no completion check, no dedup)
                 ⇒ 129 duplicate alerts for one storm; retries re-enter the same path
                      ▼
             retry: same command, same 17 GB object set ⇒ identical death (flat kill durations)
```

### 1.2 Why the crash occurred at the technical level

1. **The constraint was the cgroup, not the host.** Every surviving kernel record reads
   `oom-kill:constraint=CONSTRAINT_MEMCG` with `oom_memcg=` pointing at the dispatch scope
   ([LIVE], evidence §5.1). The kernel kills the largest task *inside the charged memcg* when that
   memcg hits its hard limit — host free memory is irrelevant. `systemd-oomd` merely corroborates
   ("A process of this unit has been killed by the OOM killer"); it was not the actor.
2. **`git pack-objects` memory is proportional to the object set and unbounded by default.** With
   17.16 GB of loose objects, `--aggressive` re-walks and re-deltas everything; `pack.windowMemory`
   (the delta-search window) and `pack.deltaCacheSize` were unset at the time, so the only ceiling was
   the scope's `MemoryMax`. The kernel's own accounting shows the three largest `git` victims at
   ≈12.0–12.55 GiB anon-rss — *at* the bound, which is the signature of a hard-limit kill rather than
   a host shortfall.
3. **SIGKILL leaves no application trace.** The victim was `git` (257 of 414 kills on Aug-16) or
   `node` (156), not a domain-check goroutine; the agent host process dies with the scope's process
   group. There is therefore **no Go stack trace to analyse — the kernel OOM record is the stack
   trace** (evidence §5.3).
4. **`exit_code=-1` is needle's sentinel, not a signal.** When the child is killed by a signal, Go's
   `cmd.ProcessState.ExitCode()` cannot return a signal-based code through the path needle uses, so
   `code().unwrap_or(-1)` records −1 (commit 5d501a8). A genuine SIGHUP is catchable and would surface
   as a recorded signal — the misreading of −1 as "SIGHUP cascade" is the single error that produced
   the anchor report's superseded mechanism (evidence §8, correction 1).
5. **Repetition without progress.** On Aug-14 the kill durations across 129 attempts are *flat*
   — proof the object set never shrank, i.e. no attempt ever reached the packing phase. Retries were
   deterministic re-entries into a deterministic failure.

### 1.3 The two magnitude events

| Event | Kernel record | Needle record | Mechanism detail |
|---|---|---|---|
| **bf-173o7e, 2026-08-14** | **[GONE]** (journal begins Aug-15) | **[GONE]** (log rotated) — **[COMMIT]** 07ab240, db1acb3, d283576 | Duplicate gc bead dispatched **132×**, **129 attempts `exit -1` over ~10.5 h**; flat durations ⇒ zero packing progress; repo actually packed Aug-14 23:25 → Aug-17 by a **non-attempt** process |
| **Aug-16 storm** | **414 memcg kills**, 04:27:35Z → 17:40:32Z (13.2 h, two waves: 04–07Z and 12–17Z) — **[LIVE]** | 461 crash outcomes Aug-16 + 3 Aug-17; first crash record 0.9 s after the kernel's first kill — **[LIVE]** | Victims: **257 `git`, 156 `node (vitest…)`, 1 other**; all `CONSTRAINT_MEMCG` in dispatch scopes |

The Aug-16 record is the evidentiary anchor: it is fully re-derivable from journald today, and its
per-process breakdown (git + node) is what ties both events to one mechanism.

---

## 2. Triggering conditions

All five must hold simultaneously; each historical kill satisfies all five.

| # | Condition | Attested by |
|---|---|---|
| T1 | A process whose allocation is proportional to an **unbounded input** runs inside a dispatch scope | bare `git gc --aggressive` over a bloated object set; `node`/vitest suite |
| T2 | The **dispatch scope's `MemoryMax=12 GiB`** is the binding constraint (not host memory) | `CONSTRAINT_MEMCG` on 100% of surviving records **[LIVE]** |
| T3 | The input is large enough to reach the bound — **repository bloat** | 18 GB / 17.16 GB loose at bf-1s6c3 **[COMMIT]**; repo now 92 MB **[LIVE] §7** |
| T4 | The command is **retried/redelivered** rather than decomposed | 132 dispatches / 129 identical deaths (bf-173o7e); 44 identical deaths in bf-4x12ec before auto-split engaged at 11:59:06Z (`bf-4x12ec-root-cause.md`, committed) |
| T5 | No bound on the bare path (`pack.windowMemory`/`pack.deltaCacheSize` unset; no staged gc script) | bounds absent pre-533cb46, present and verified today **[LIVE] §7** |

**Post-fix status:** T3 and T5 are eliminated in this workspace (repo 92 MB; `pack.windowMemory=2g`,
`pack.deltaCacheSize=1g`, `pack.threads=1` pinned repo-local **and** global ⇒ worst case ≈3 GiB per
pack run), which is why the signal-death era ended. T1/T2 remain structural: any future unbounded
process inside a dispatch scope can still hit the bound — the *general* pattern is mitigated by
decomposition (auto-split) and the staged `safe-git-gc.sh`, not eliminated.

---

## 3. Edge cases identified

1. **Post-completion kill ≠ task failure.** bf-5tgsk: completed 16:35:54, killed 16:36:24, bead
   closed 16:36:51 — the kill landed after the work, yet generated a crash alert
   (no completion detection, evidence §3).
2. **Self-healed retry still alerts.** bf-6bio4g: crash → retry exit 0; the alert fired for the
   transient, not for an outcome.
3. **`exit 0` does not mean the intended operation happened.** bf-4x12ec's 53rd attempt exited 0 at
   12:58:45Z only because needle's auto-split had decomposed the bead (11:59:06Z) into gc/repack/verify
   children that each fit the scope budget — the exit-0 belongs to the decomposed child, not to the
   original monolithic command succeeding (`docs/crash-investigations/bf-4x12ec-root-cause.md`).
   Corollary: an exit-0 + `verification.passed` record proves that *run's* outcome only.
4. **`exit_code=1` waves are a different failure class.** Since Aug-26 the fleet sees synchronized
   exit-1 waves (13–15 workers/min, cross-workspace) = inference-service class — no signal, no OOM.
   Classify before investigating (f9af254).
5. **Cgroup-scoped OOM is invisible to host alerting.** Host memory stayed healthy throughout, so
   host-wide thresholds (70% pressure) could never have caught any of these kills; only per-scope
   observation works (evidence §8, correction 4).
6. **Health-check false alarm by construction.** The documented gateway check `curl -sf …` fails with
   curl 60 (self-signed cert) while the gateway answers `200 ok`; `-skf` is correct. A doc-level bug
   that manufactures "service down" conclusions.
7. **The alert system can manufacture its own work.** The 19:38:42Z 4-way auto-split that produced
   this chain is itself an L2 artifact — dispatched children onto a report whose mechanism was later
   corrected; one sibling dispatch in the wider corpus carried a wholly fabricated premise
   (nonexistent `--skip-verify` flag, nonexistent CI check, 6369467).
8. **A needle defect can be the kill source.** 2026-08-26: agent closes bead → dies by signal → crash
   handler runs `bead release` against the closed bead → exit 4 → unhandled error kills the worker
   (18 deaths). Not OOM, not service — a tool bug (eba6c2a).

---

## 4. Code and configuration references

| Layer | Reference | Role in the root cause |
|---|---|---|
| Kernel | `oom_kill_process.cold+0x8/0x87`; `Memory cgroup out of memory: Killed process …` (evidence §5.1, verbatim **[LIVE]**) | The only "stack trace" that exists for any of these deaths |
| needle (upstream) | `code().unwrap_or(-1)` in the agent-outcome path (commit 5d501a8) | Produces the `exit_code=-1` sentinel; the source of the SIGHUP misread |
| git pack | `pack.windowMemory` (⚠ **per-thread** — hence `pack.threads=1`), `pack.deltaCacheSize` | The knobs whose absence made the bare path unbounded; both now pinned at `2g`/`1g`/`1` repo-local + global (533cb46) |
| Repo guard | `scripts/setup-git-gc-config.sh --verify` | Resolves the **effective** bound (system → global → local) and exits 1 if unbounded or threads unpinned |
| Bounded gc path | `scripts/safe-git-gc.sh` (`--full`, `--resume`, `--check-only`), `scripts/safe-git-gc-monitor.sh`, `scripts/test-gc-memory-bounds.sh` (re-runs the exact crash command under a 768 MiB cgroup; peak pack-objects RSS ≈312 MiB) | Mechanically proves the same operation fits when bounded |
| Scheduling | `scripts/domain-check-git-gc.service`, `scripts/domain-check-git-gc-full.service` (`MemoryMax=4G`), systemd user timers (daily 03:00 incremental, Sun 04:00 full) | Maintenance without unbounded memory; ⚠ `daemon-reload` required after unit edits or timers silently never fire |
| Detection (repo-side) | `scripts/crash-alert-manager.sh` (+ `crash-classifier.sh`, `alert-deduplication.sh`, 5-min cooldown), `scripts/verify-work-completion.sh` pre-close gate | Mitigations for L2 — completion awareness, classification, dedup |
| Application (null result) | `internal/server/safeguards.go` — `Recover()` and `DefaultRequestTimeout` (commit 98ab63e) | Defense-in-depth for request-path panics/timeouts. **No domain-check defect appears anywhere in the evidence** — these safeguards address a class of failure that never occurred here |

---

## 5. Evidence chain

1. **Kill records exist and are cgroup-scoped** → journald Aug-16: 414 × `Memory cgroup out of
   memory`, all `CONSTRAINT_MEMCG`, `oom_memcg=…/run-p*.scope` **[LIVE]** (re-derived 2026-09-06, §7).
2. **Victims are git and node, at the bound** → 257 `git` / 156 `node`; largest `git` anon-rss
   12,555,188 kB ≈ 12.0 GiB vs `MemoryMax=12GiB` **[LIVE]**.
3. **The agent death is the same event seen from userspace** → first needle crash record fires
   **0.9 s** after the kernel's first kill of the day (04:27:35Z → 04:27:36.062598Z, bead bf-uoyie)
   **[LIVE]**.
4. **−1 is a sentinel, not a signal** → needle records it via `code().unwrap_or(-1)`; SIGHUP is
   catchable and recordable **[COMMIT]** 5d501a8. All 158 of this worker's exit-(-1) outcomes fall on
   Aug-16/Aug-17, then zero forever **[LIVE]** — consistent with a closed era, not an ongoing mechanism.
5. **Bloat supplied the unbounded input** → 18 GB / 17.16 GB loose at bf-1s6c3; cleanup to 138 MB
   restored function **[COMMIT]** c4d2b29/a7b1347/2f1a9b2/76a9c1d.
6. **Repetition without progress** → 132 dispatches / 129 `exit -1` with flat kill durations
   (bf-173o7e) **[COMMIT]**; independently, 44 identical deaths in bf-4x12ec before decomposition
   **[COMMIT]**.
7. **The bound is the fix and it works** → `pack.*` keys pinned repo+global (533cb46); bounded run
   proven in production 2026-09-02 (completed, 1 pack 90.19 MiB) **[COMMIT]** f9af254; synthetic
   reproduction under a 768 MiB cgroup peaks ≈312 MiB **[COMMIT]**.
8. **The era closed** → zero fleet signal deaths since 2026-08-26 **[COMMIT]** f9af254; repo at
   92 MB / 1 pack / 0 garbage today **[LIVE]**.

Steps 1–3 establish the mechanism directly from primary sources; 4 fixes the interpretation of the
needle-side record; 5–7 supply cause and remedy; 8 confirms remediation held.

---

## 6. Ruled-out alternate causes

| # | Alternate hypothesis | Verdict | Decisive evidence |
|---|---|---|---|
| A1 | **A domain-check code defect** (panic, leak, runaway allocation) | **REJECTED** | No Go panic or stack trace in any event; SIGKILL victims are `git`/`node`/agent-host, never a domain-check fault; zero findings across 157+ investigations **[COMMIT]** 383241f. App code never ran in the killing scope's memory profile |
| A2 | **Host-wide memory exhaustion / systemd-oomd at 94.71% pressure** | **REJECTED** | 100% of surviving kills are `CONSTRAINT_MEMCG` **[LIVE]**; host memory healthy. The literal `94.71` string in journald is a substring of Tailscale IPs in socat lines, not an oomd record **[LIVE]** (evidence §8, correction 3) |
| A3 | **SIGHUP cascade** (the anchor report's original claim) | **REJECTED** | `exit -1` is the `unwrap_or(-1)` sentinel; a real SIGHUP is catchable and would be recorded as signal 1; kernel records show SIGKILL from the OOM killer **[COMMIT]** 5d501a8 + **[LIVE]** §5.1 |
| A4 | **Inference-gateway outage killed the workers** | **REJECTED (for the signal deaths)** | Service problems manifest as `exit_code=1` waves (13–15 workers/min, post-Aug-26) — a different signature, a different era. Gateway outages never produced an `exit -1` |
| A5 | **Git gc is inherently unsafe on this host** | **REVISED → REJECTED** | The *bounded* path is proven: 2026-09-02 run completed under its MemoryMax (1 pack, 90.19 MiB) **[COMMIT]** f9af254; synthetic rerun of the exact crash command peaks ≈312 MiB under the pinned config **[COMMIT]** 533cb46. The hazard was exclusively the **unbounded bare** invocation |
| A6 | **One of the 129 attempts eventually packed the repo** (i.e. the storm "worked") | **REJECTED** | Flat kill durations across all 129 prove no attempt shrank the object set; packing completed Aug-14 23:25 → Aug-17 by a **non-attempt** process. bf-4x12ec's lone exit-0 belongs to an auto-split *child*, not to the monolithic command **[COMMIT]** |
| A7 | **Manual/external kill or tampering** | **REJECTED** | Every record is a kernel memcg kill, uid 1001, inside a dispatch scope, correlated to a dispatch's own command; no `kill(1)`/signal-sending actor appears in any journal or needle record |
| A8 | **NEEDLE reaped or actively killed the agents** | **REJECTED (for exit −1)** | Needle is the *observer* here — it records the outcome 0.9 s after the kernel kill. Where needle *does* kill (L3 `bead release` exit-4 defect, Aug-26, 18 deaths), the records show a tool error path, not an OOM signature — that is a separate confirmed cause, listed in §3.8, not an alternative to this one |

**Explicitly retained as co-causes (not alternates):** L2 detection/alerting defects (alert volume,
duplicate chains, fabricated premises) and L3 the bead-release crash-handler defect (a genuine kill
source on Aug-26). Both are independent of the OOM mechanism and both remain partially unremediated
upstream — see [`investigation-report-final-2026-09-06-domchk-e843c4f1.md`](investigation-report-final-2026-09-06-domchk-e843c4f1.md) §3 (L2/L3) and §9 (R1–R4).

---

## 7. Live re-derivation — 2026-09-06

Every load-bearing figure re-run during this RCA; all matched the evidence compilation exactly:

```
$ du -sh .git && git count-objects -vH
92M      .git
count: 11                      size: 88.00 KiB        # loose
in-pack: 10712                 size-pack: 90.43 MiB
garbage: 0

$ git config --show-origin --get-all pack.windowMemory | sort -u
file:/home/coding/.gitconfig   2g
file:.git/config               2g                     # deltaCacheSize=1g, threads=1 likewise in BOTH scopes

$ systemctl --user show needle.slice -p MemoryHigh -p MemoryMax
MemoryHigh=25769803776                                # 24 GiB
MemoryMax=34359738368                                 # 32 GiB (dispatch scopes carry their own 12 GiB)

$ grep -oP "exit_code=\K-?\d+" ~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log | sort | uniq -c
   1023 0        # success
    304 1        # task/workflow failure (incl. service waves)
    158 -1       # sentinel — ALL on Aug-16/Aug-17
     17 124      # timeout

$ grep -c 'needle.outcome="crash"' <same log>
464                                                   # 461 Aug-16 + 3 Aug-17
```

---

## 8. Provenance limits

- **[LIVE]** — everything from 2026-08-16 onward: journald (retention from 2026-08-15T23:46:33Z), the
  needle primary log (2026-08-16 → 2026-09-05), repo state, git config, slice limits.
- **[COMMIT]** — the 2026-08-12/13/14 events: their kernel and needle logs are **[GONE]** (rotation),
  so counts rest on the committed corpus (07ab240, db1acb3, d283576, b1ae579, c4d2b29 et al.), which
  recorded the raw values while the logs existed.
- **No [REPORTED] figure is load-bearing here.** The anchor report's "201+ crashes", "826 crashes"
  and "94.71%" volumetrics are attested-only and are *not* used in this determination; where they
  conflicted with primary sources, the corrected values appear above (evidence §8).
- **Relationship to the corpus:** this document is the RCA deliverable of the 2026-09-01 chain. It is
  consistent with, and defers to, the corpus-wide corrected analysis in
  `investigation-report-final-2026-09-06-domchk-e843c4f1.md` (five causal layers L1–L5) — the layering
  here corresponds to **L1** (root cause), **L2/L3** (co-causes) and **L5** (the null result), with L4
  (service class) out of scope for this determination beyond the classification edge case in §3.4.

---

## 9. Handoff to downstream beads

For **domchk-ece81e17** (solutions documentation), the determination above implies the solution set is
already largely *implemented* and should be documented as such, with these residual gaps called out:

1. Documented and verified: git-config memory bounds (repo + global), safe-gc scripts, systemd timers,
   crash-alert manager/classifier/dedup, `verify-work-completion.sh` pre-close gate.
2. Open in this repo: no monitoring script *observes* the effective gc bound on a schedule (it is
   enforced but unobserved) — cf. e843c4f1 §9 R6.
3. Open upstream (NEEDLE, P0): completion detection, alert deduplication, the L3 `bead release`
   exit-4 crash-handler fix, and an early circuit-breaker on same-cause/same-command kill signatures.
4. Anti-recommendations to carry forward: do not re-enable host-wide memory alerting as the control
   for this failure mode (it is structurally blind to cgroup-scoped kills — §3.5); do not "fix" the
   gateway health check by loosening TLS verification silently — use `-skf` with the self-signed-cert
   caveat documented (§3.6).

For **domchk-7880ada7** (finalize): the corrected mechanism in this document and in the evidence
compilation supersedes the anchor report's §"Primary Root Cause" line; the anchor report already
carries the 2026-09-06 correction banner pointing at the corpus.

---

**Determination complete.** Root cause: cgroup-scoped memcg-OOM SIGKILL of unbounded-memory processes
inside `MemoryMax=12 GiB` dispatch scopes, amplified by repository bloat and multiplied by naive
redelivery; misrecorded as `exit_code=-1` and misread as a SIGHUP cascade. No domain-check defect.
Remediated in this workspace since 2026-09-02; detection-side fixes partially implemented, partially
pending upstream.
