# bf-4k2ws — Root Cause Analysis

**Dispatched leg:** analyze (`domchk-541f1089`, template-chain "analyze" step)
**Target bead:** `bf-4k2ws` — "Analyze divergent Forgejo and GitHub branch states" —
**Closed** (rev 2, 2026-08-16T15:35:42Z, 8/8 acceptance criteria met)
**Source alert bead:** `bf-4xlwo` (Open, rev 20) ↔ **attempt 57 of 62**, death
2026-08-13T06:34:29.204954348Z
**Classification:** **INFRASTRUCTURE — resource limit** (memory-cgroup OOM kill
inside the 12 GiB dispatch scope). **Not an agent bug, not a domain-check code
defect.**
**Determination date:** 2026-09-09

> **What this document is.** The analyze-leg deliverable, filed at the
> dispatch-named path. It is **subordinate to the canonical record**:
> `docs/investigations/bf-4k2ws-root-cause-determination-domchk-7f838f36-2026-09-07.md`
> (§§1–17) and `docs/crash-investigations/bf-4k2ws/README.md`. The determination
> was already made and re-verified; this document does not re-open it — it
> answers the dispatch's six acceptance criteria against the current
> determination, quoting census figures from the canon with provenance and
> re-verifying the attempt-57 evidence first-hand this session.
>
> **Per the canon's lesson 10** ("a dispatch template is a stale-premise
> vector"), three premises in this dispatch's own template are corrected in
> §0 before the analysis uses them.

---

## 0. Template-premise corrections (read first)

| Template premise | As dispatched | Verified fact |
|---|---|---|
| "Investigate what signal −1 means (SIGTERM? SIGKILL? OOM?)" | −1 treated as a signal number | **−1 is needle's unrecorded-signal sentinel — no such signal exists.** The process died by signal with the status unrecorded. The underlying question is still answered: the mechanism of that signal death is a memcg-OOM **SIGKILL** (chain-inferred for this bead, kernel-proven for the siblings `bf-4x12ec`/`bf-198ne`). §3 |
| "Check system logs for memory issues" | Kernel/journald records assumed to exist for the crash night | **None survive.** The box rebooted 2026-08-14 and the single current boot begins 2026-08-15 19:56:33 EDT (re-verified this session via `journalctl --list-boots`). Aug-13 kernel lines, journald records, and memory samples cannot exist; `coredumpctl` has zero 2026-08-13 entries. The surviving record is the worker log. §4 |
| Deliverable path `docs/crash-investigation/` (singular) | Implies the corpus home | The corpus home is `docs/crash-investigations/` (plural); the singular directory holds `bf-198ne`/`bf-1s6c3` material. This file is created at the named path anyway so the criterion resolves there — it is subordinate to the canon, not a competing report |

A fourth premise — the dispatch's "Timestamp:
2026-08-13T06:34:35.805234687Z" inherited from `bf-4xlwo` — was already
corrected by the gather leg (`domchk-6ba3b2af`): it is a **heartbeat stamp**
(log seq 8376, `HANDLING_RELEASE_DONE` at 06:34:35.805222217Z, 12.47 µs delta),
not the death. The kill is `agent.completed` at 06:34:29.204954348Z, 6.600 s
earlier.

---

## 1. Determination

**Root cause (carried from the canon, not re-derived here):** dispatches
running git-remote-heavy work against the then-**≈18 GB** object store (the
bf-1s6c3/bf-4yjq bloat-era repository, ~17 GB of loose objects) inside needle's
**12 GiB dispatch scope** exhausted the scope's memory budget; the kernel's
memory-cgroup OOM killer SIGKILLed the attempt — uncatchable, recorded by
needle only as the sentinel `exit_code = −1`. The retry layer released the bead
and re-claimed it with **nothing bounding the loop**: 62 attempts, one worker
session, ~5 h 14 min.

**Classification against the dispatch's four-way choice:**

| Dispatch category | Verdict |
|---|---|
| **Resource limit** | **✅ This one.** The constraint was the cgroup (`memory.max = 12884901888`, re-read live from a running dispatch scope this session), not the host — the host was never out of memory |
| Agent bug | ❌ No defect: the task was read-only git analysis; no panic, no stderr, no stack; zero `max_turn` mentions in the day's 395 completions (canon §1) |
| External factor | Partially — co-factor only: host CPU saturation (59 `fleet.cpu_saturated` samples in-window, load 7.63–18.51 on 9 cores, threshold > 7.2). Not independently sufficient; the era's kill mechanism is cgroup-scoped (canon §3.4) |
| Unknown | ❌ Mechanism confidence is **MEDIUM-HIGH**, not unknown — chain-inferred for this bead (no Aug-13 kernel record survives), kernel-proven for the same-repo same-scope gc/push siblings `bf-4x12ec` / `bf-198ne` |

In the workspace's own taxonomy (`docs/crash-response-guide.md`) this is
**INFRASTRUCTURE**, matching the classification `outcome.classified` assigned
at the kill (crash class, exit −1).

**Amplifiers (why one cause produced 55 kills):**

1. **Unbounded retry** — every kill released the bead back to the ready
   frontier and the dispatch layer re-claimed it, ~4.5 min median cycle, no
   storm breaker or backoff in that era.
2. **Verify-then-close debt** — the task *succeeded* at 04:48:09.546Z and
   07:17:41.039Z (both `verification.passed`), and each success was
   `bead.orphaned` ~6 s later instead of closing. The terminal condition was
   unreachable from the success side, so **25 of the 55 kills (45 %) were
   post-completion** — including the one this alert represents.

The kills were real. The false positives live entirely in the **alert layer**:
pre-dedup needle (< 0.4.2) filed one ALERT bead per kill — 55 beads for one
cause, every one after 2026-08-16 fired against an already-closed bead.

---

## 2. The specific kill this alert represents — attempt 57 of 62

Re-derived first-hand this session from the untouched primary log
`~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl`
(3,111,314 bytes, mtime 2026-08-13 19:59); matches the gather leg's table.
All times UTC.

| Instant | Event (log seq) |
|---|---|
| 06:29:08.909680204Z | `bead.claim.succeeded` — **attempt 57** (8355) |
| 06:29:08.913416631Z | `fleet.cpu_saturated` — load **11.65**, 9 cores (8361) — see §6 |
| 06:29:08.919987765Z | `agent.dispatched` — agent `claude-code-glm-4.7`, session `8446529e`, template `pluck`, prompt_len 70,932 (8364) |
| 06:29:08.939617828Z | `transform.started` (8365) |
| 06:34:28.911379687Z | `transform.completed` — 319,951 ms (8366) |
| **06:34:29.204954348Z** | **`agent.completed` — exit −1, 319,831 ms (5 m 19.8 s). The kill.** (8367) |
| 06:34:29.207023780Z | `outcome.classified` — crash, exit −1 (8370) |
| 06:34:33.461946764Z → 06:34:35.805222217Z | `heartbeat.emitted` ×4 — the last, `HANDLING_RELEASE_DONE` (8376), is the record behind the alert's "Timestamp" |
| 06:34:35.811234404Z | alert bead `bf-4xlwo` written (+5.9 ms; body re-read live from the checkpoint this session — verbatim match) |
| 06:34:38.234212909Z | `bead.released` (8377) |
| 06:34:38.236100365Z | `outcome.handled` — `{"action":"alerted"}` (8378) |
| 06:34:40.411374321Z | next claim — **attempt 58**, which then hit the 600 s dispatch cap (exit **124**, 600,020 ms — canon's cap attempts 58/59/61) |

**Position in the storm:** attempt 57 falls in the post-completion window
(attempts 34–61) — it ran *after* the first verified success (04:48:09.546Z)
had been orphaned (04:48:15.306Z), i.e. it was killed **re-doing work that was
already done and verified**. Its 319.8 s duration sits mid-distribution for the
kill class (overall 123.6–528.9 s, median 252.9 s; buckets
[0, 15, 22, 15, 2, 1, 0] with **zero** kills within 70 s of the 600 s cap —
the scope-budget signature, canon §3.1).

**No work was lost.** The target closed 2026-08-16 with all 8 criteria met;
four deliverable docs on `origin/main`; the divergence answer (0/0) still holds
(canon §4).

---

## 3. What "exit −1 (signal −1)" means

Exit codes in this record are **classes, not signals** (canon §3.6):

| Exit | Night's count | Meaning |
|---|---|---|
| **−1** | 55 | Needle's **unrecorded-signal sentinel**: the process died by signal and the exit status was not recorded. **Never name a signal from it** — "(signal −1)" in the `bf-4xlwo` body is template wording, not a signal that exists (no signal −1 exists in POSIX) |
| 124 | 5 | The **600 s dispatch cap** (attempts 16, 17, 58, 59, 61) — *not* max-turns |
| 0 | 2 | Success (both `verification.passed`, both then orphaned) |

Answering the template's actual question — SIGTERM, SIGKILL, or OOM:

- **Not SIGTERM:** SIGTERM is catchable and would have produced a recorded exit
  status (143 or a handler exit), not the sentinel. Nothing in the loop's 55
  kills shows a recorded status.
- **Not SIGHUP:** the superseded 2026-09-02 corpus claimed exactly this and is
  wrong — **zero exit-129s anywhere in the loop** (canon §3.5).
- **SIGKILL from the memory cgroup's OOM killer (memcg OOM)** is the mechanism:
  uncatchable, so the status is unrecorded → sentinel −1 → `outcome.classified:
  crash`. **Chain-inferred for this bead** (the Aug-14 reboot destroyed the
  Aug-13 kernel records), **kernel-proven** (`CONSTRAINT_MEMCG`) for the
  same-repository, same-scope siblings `bf-4x12ec` (bare `git gc --aggressive`)
  and `bf-198ne` (unbounded `git push` pack-objects).

---

## 4. System logs — what was checked and what survives

Re-verified live this session:

| Check | Result |
|---|---|
| `journalctl --list-boots` | **One boot**, beginning 2026-08-15 19:56:33 EDT → no kernel or journald record of 2026-08-13 can exist |
| `coredumpctl list` | **Zero 2026-08-13 entries** (earliest 2026-08-17, an unrelated `pdftract` SIGABRT). A memcg SIGKILL produces no core in any case |
| Memory/disk samples at kill time | None — health collection starts 2026-08-15; only `fleet.cpu_saturated` load samples exist for Aug-13 (59 in-window: 7.63–18.51) |
| Dispatch scope `memory.max` | **12884901888 (12 GiB)**, read live from the in-flight needle dispatch scope cgroup |
| Primary worker log | Intact, 3.1 MB, untouched since 2026-08-13 19:59 — the census source; attempt-57 window re-derived from it this session |
| Application error text | **None exists.** Needle recorded no stderr, no stack, no panic — the per-kill record is the sentinel −1 plus `outcome.classified: crash` |

The absence of kernel lines is a **retention** fact (boot boundary), not
evidence of absence of kills — the kills are in the worker log.

---

## 5. Environmental or code-related?

**Environmental (resource limit), with no code defect anywhere in the chain.**

- The dying process was needle's agent worker doing **git-remote-heavy** work
  (fetch / ls-remote / rev-list against Forgejo and GitHub) — the exact
  operation class the ≈18 GB bloat-era store turned into deterministic kills
  inside the 12 GiB scope. Domain-check code had no role; the task was
  read-only analysis.
- The workspace's standing finding holds: across 157+ investigations of this
  repo, **no domain-check code defect was ever found** — zero panics, zero
  stack traces (repo CLAUDE.md, "Crash Incident Investigation").
- The killed agent is not defective either: the identical task, agent, session,
  and template **succeeded twice** the same night (exit 0, both
  `verification.passed`). A deterministic environment constraint that kills a
  run partway — while identical runs complete when memory pressure timing
  allows — is a resource limit signature, not a bug signature.
- The kill-duration distribution (123.6–528.9 s, median 252.9 s, zero
  cap-adjacent) matches an allocation failure at an unpredictable point in a
  memory-hungry operation, and matches no timeout or fixed boundary (canon
  §3.1).

---

## 6. Preceding error messages or warnings

**Application-level errors: none.** No stderr, no panic, no stack, no
degraded-service record precedes the kill; the log goes straight from
`transform.completed` to the fatal `agent.completed`.

**Warnings — three classes existed, and the era's tooling consumed none of
them:**

1. **Host CPU saturation** — a `fleet.cpu_saturated` record at
   **06:29:08.913416631Z (load 11.65 on 9 cores)** sits **6.5 ms before
   attempt 57's dispatch** (seq 8361), and another (10.7) immediately precedes
   attempt 58's. 59 such samples fall inside the loop window (7.63–18.51).
   The co-factor was measurable at dispatch time and nothing gated on it —
   exactly the gap `scripts/preflight-health-check.sh` /
   `system-event-mode.sh` now close (canon §5.1 R4, re-run green this
   session).
2. **The loop's own history** — by attempt 57, the same bead had already been
   killed 32 times and capped twice. The loudest possible warning that the
   dispatch was re-entering a failing regime, and no breaker existed to
   consume it (the `crash-circuit-breaker.sh` / `needle-with-limiter.sh` gate
   is the fix).
3. **The orphaned success** — `verification.passed` at 04:48:09.546Z followed
   by `bead.orphaned` 5.8 s later. Every kill after that instant was provably
   wasted work; the dispatch layer had no post-verification terminal check.

---

## 7. Similar crashes with this agent/worker type before — yes, the whole era

Same agent (`claude-code-glm-4.7`), same worker family
(`…-lab-domain-check`), same repository, same 12 GiB dispatch scope:

| Bead | Date | Kills | Mechanism status |
|---|---|---|---|
| `bf-1s6c3` (predecessor) | 2026-08-12 → 08-13 | 71 (of 76 dispatches) | chain-inferred (`docs/crash-analysis-bf-1s6c3-2026-09-06.md`); completed exit 0 on this very session **7.15 s before** `bf-4k2ws` was first claimed |
| **`bf-4k2ws`** | **2026-08-13** | **55** (of 62 attempts) | chain-inferred — this document |
| `bf-4yjq` | 2026-08-12 | 50 | chain-inferred; repo repaired & verified (`docs/crashes/bf-4yjq-cleanup-verification.md`) |
| `bf-173o7e` | 2026-08-14 | 129 of 131 attempts | chain-inferred |
| `bf-4x12ec` | 2026-08-14 | 44 identical kills | **kernel-proven** memcg OOM of bare `git gc --aggressive` |
| `bf-198ne` | 2026-08-16 | push-side variant | **kernel-proven** memcg OOM of `git push` pack-objects |
| `bf-1ea4g` | 2026-08-13 | closed after 57 attempts | unbounded `git push` pack-objects over the commit backlog |

One regime, one root cause, many instances — which is why the canon's lesson 7
is "classify the cause once; close the swarm as instances," and why this leg
does not re-investigate.

---

## 8. Ruled out

| Alternate | Why excluded |
|---|---|
| SIGHUP cascade (2026-09-02 corpus) | Zero exit-129s in the loop; −1 is a sentinel, not signal 1; the claimed window was also the wrong day |
| Agent bug / domain-check code defect | Task read-only; no error text; zero `max_turn` mentions in 395 completions; identical task+agent+session succeeded twice the same night |
| Max-turns exhaustion | Same zero-`max_turns` evidence; the 5 × 124 are the dispatch cap |
| Service failure (gateway 503/502) | No service-class signature; the failure mode is uniform exit −1 |
| Host OOM / disk exhaustion | Host was never out of memory — the constraint was the cgroup; the "clean repo / 52 GB free" readings in the superseded report were captured 2026-09-02, three weeks post-repair, describing the wrong night |

---

## 9. Acceptance-criteria mapping

| Criterion | Answer | Where |
|---|---|---|
| Review the crash context document from `domchk-6ba3b2af` | Reviewed — `docs/crash-investigation/bf-4k2ws-crash-context.md` (on `origin/main`); its attempt-57 chronology re-verified first-hand this session and its alert↔death mapping (`bf-4xlwo` ↔ attempt 57) adopted | §2 |
| What does −1 mean (SIGTERM? SIGKILL? OOM?) | Premise corrected: −1 is an unrecorded-signal sentinel, not a signal; the mechanism is memcg-OOM SIGKILL (chain-inferred here, kernel-proven for siblings) | §0, §3 |
| System logs for memory issues / forced termination | Checked live: no kernel/journald/coredump/memory-sample record survives the Aug-14 reboot; the worker log is the surviving record; 12 GiB `memory.max` read live | §4 |
| Environmental (resource limits) or code-related (agent bug) | **Environmental — resource limit** (cgroup memory cap). No agent bug, no domain-check code defect | §1, §5 |
| Preceding error messages or warnings | No app-level errors; three warning classes existed unconsumed: cpu_saturated at dispatch (11.65, 6.5 ms pre-dispatch), the loop's own 32 prior kills, the orphaned verified success | §6 |
| Similar crashes with this agent type before | Yes — the repo-bloat-era kill-regime family (bf-1s6c3, bf-4yjq, bf-173o7e, bf-4x12ec, bf-198ne, bf-1ea4g), same agent/worker/repo/scope | §7 |

**Deliverable content:** root cause §1; evidence §§2–7; classification §1
(resource limit / INFRASTRUCTURE).

---

## 10. Sources

- Primary (re-derived this session): `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` — attempt-57 window quoted at seqs 8355–8393
- Live reads this session: `bead show bf-4k2ws` (Closed rev 2), `bead show bf-4xlwo` (Open rev 20, body verbatim match), `journalctl --list-boots`, `coredumpctl list`, dispatch-scope `memory.max` from `/proc/self/cgroup`, five-check prevention battery (all exit 0 — `check-repo-health.sh`, `setup-git-gc-config.sh --verify` ≈3072 MiB worst case, `preflight-health-check.sh` 4/4, `system-event-mode.sh check` → clear, `crash-circuit-breaker.sh status` → no open entries)
- Canon (quoted with provenance, not re-derived): `docs/investigations/bf-4k2ws-root-cause-determination-domchk-7f838f36-2026-09-07.md` (§§1–17; §17 re-derived the census byte-exact 2026-09-09); `docs/crash-investigations/bf-4k2ws/README.md`
- Gather leg: `docs/crash-investigation/bf-4k2ws-crash-context.md` (`domchk-6ba3b2af`)
- Kernel-proven siblings: `docs/crash-reports/bf-4x12ec-git-gc-crash.md`, `docs/crashes/bf-198ne-crash-report.md`; predecessor: `docs/crash-analysis-bf-1s6c3-2026-09-06.md`
- Evidence bundle: `docs/crash-artifacts-bf-4k2ws/README.md`
