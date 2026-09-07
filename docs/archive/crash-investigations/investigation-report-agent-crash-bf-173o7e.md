# Investigation Report (Final): Agent Crash bf-173o7e

**Report date:** 2026-09-06 (finalization bead `domchk-7880ada7`)
**Subject bead:** `bf-173o7e` — "Execute git gc --aggressive with pruning" (created 2026-08-14T12:57:54Z, closed 2026-08-17T17:15:23Z)
**Crash events:** 2026-08-14 (retry storm, primary) and 2026-08-17 (post-completion turn exhaustion, secondary)
**Classification:** **INFRASTRUCTURE** — cgroup-scoped memcg-OOM SIGKILL of an unbounded `git gc --aggressive --prune=now` inside a 12 GiB dispatch scope; plus a separate post-completion workflow failure
**Confidence:** HIGH for the storm mechanism (live re-derived counts) and for the Aug-17 event (trace still on disk); the Aug-14 kernel-side record itself has rotated and is attested-only
**Domain-check code:** ✅ zero defects — unchanged by this revision and re-confirmed below

> **This revision supersedes the 2026-09-01 version of this same file** (commit `232bc73`).
> That version analyzed the Aug-17 termination only, reported its root cause as
> "FALSE POSITIVE — administrative workflow failure," and attributed an "18 GB → 445 MB
> (97.5 % reduction)" to that attempt. The classification was right for the event it
> described, but the bead's primary crash event is the **Aug-14 memcg-OOM storm**, and
> several of that version's quantitative claims were corrected against the preserved trace
> on 2026-09-05 (`domchk-60407475`, commit `73176de`). §6 lists every corrected claim.
> Corpus-wide context: `docs/investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md`.

---

## 1. Executive Summary

`bf-173o7e` is the largest single crash event in the domain-check corpus, and it produced
**two unrelated terminations** that early investigations conflated. The primary event is the
**2026-08-14 retry storm**: the bead's own text prescribed bare `git gc --aggressive --prune=now`
against a workspace holding **17.20 GiB of loose objects**, needle re-dispatched the identical
doomed task **132 times**, and **129 attempts died `exit -1` over ~10.5 hours** (12:58:58Z →
23:25:35Z). The mechanism is kernel **memory-cgroup OOM SIGKILL** — pack-objects allocating
without bound inside the per-dispatch scope's `MemoryMax=12 GiB` — with the highest-badness
victim being the agent process itself, so needle recorded its `unwrap_or(-1)` sentinel rather
than a signal number. Nothing bounded the retry loop, and the one exit-0 attempt's success went
unrecognized (`bead.orphaned`), so the bead was left for manual closure. The secondary event is
the **2026-08-17 termination** the previous version of this report documented: a post-completion
`error_max_turns` (exit 1) that exhausted its remaining turns retrying `bead close` — a real
event, correctly classified FALSE POSITIVE at the alert level, but not the bead's crash.

Two misdiagnoses from the earlier round are worth naming because they shaped a week of wasted
effort. First, `exit_code=-1` was read as a signal ("SIGHUP cascade", "systemd-oomd at 94.71 %
memory pressure") — it is neither; it is needle's sentinel for an unrecorded signal death, and
the only surviving kill records for the period read `CONSTRAINT_MEMCG` inside dispatch scopes
while the host itself was not out of memory. Second, the Aug-17 attempt was credited with the
"18 GB → 445 MB, 97.5 % reduction" — the preserved trace shows the 444.24 MiB pack **already
present** at that attempt's first command; its actual work was consolidating 9 loose objects to
3. The real packing of the 17.20 GiB loose set was completed **Aug-14 23:25 → Aug-17 by a
non-attempt process**. What does survive every correction is the null result: **zero
domain-check code defects** across the entire corpus — every SIGKILL victim was `git`, `node`,
or the agent host process.

The kill mechanism is now **fixed mechanically and proven in production**: persistent git config
bounds the bare path (`pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` → worst
case ≈3 GiB per pack run), the scheduled path runs through `safe-git-gc.sh` under
`MemoryMax`-limited systemd user timers, and the exact crash command re-run under a 768 MiB
cgroup exits 0 with pack-objects peaking at ≈313 MiB. `[LIVE 2026-09-06]` the bound verifies
clean (`setup-git-gc-config.sh --verify` → exit 0, worst case ≈3072 MiB) and the repository is
healthy and optimal (93 MB `.git`, 1 pack / 90.43 MiB, 0 garbage). Zero signal deaths
fleet-wide since 2026-08-26. What remains is not this fix set but its **observability** (the
bound is enforced but unmonitored; host-wide monitors are structurally blind to cgroup-scoped
kills), four upstream NEEDLE defects (no completion detection, no dedup, the release-conflict
kill, unvalidated auto-split premises), and the discipline problem this report itself closes:
an investigation concluded on 2026-09-01 was re-synthesized at least three times afterward.

---

## 2. The Two Terminations

| | **Event 1 — Aug-14 storm (primary)** | **Event 2 — Aug-17 close failure (secondary)** |
|---|---|---|
| Time | 2026-08-14 12:58:58Z → 23:25:35Z (~10.5 h) | 2026-08-17 16:59:45Z → 17:06:59Z (~7.4 min) |
| Exit | **−1** ×129, 124 ×1, 0 ×1 (of 132 dispatches) | **1** (`error_max_turns`, 30-turn limit) |
| Cause | memcg-OOM SIGKILL mid-gc | Turn exhaustion retrying `bead close` |
| Class | INFRASTRUCTURE | Workflow (FALSE POSITIVE at alert level) |
| Work outcome | Packing completed Aug-14 23:25 → Aug-17 by a **non-attempt process** | gc had completed ~90 s earlier; bead closed 17:12:09Z by a later attempt |
| Determination | `domchk-2e371a2c` (07ab240), HIGH confidence | `domchk-60407475` (73176de), trace-validated |

### 2.1 Event 1 — the Aug-14 memcg-OOM storm

1. The bead body prescribed the exact command: *"Run `git gc --aggressive --prune=now` to pack
   17.20 GB of loose objects"* — with no `pack.windowMemory` bound in place.
2. `git gc --aggressive` computes delta chains across the whole object set in memory before
   writing pack bytes; pack-objects RSS grew unbounded.
3. Each dispatch ran in a transient `run-*.scope` capped at **12 GiB** with
   `oom_score_adj=200`; the budget was exhausted in 21.6–216.6 s.
4. The kernel's memcg OOM killer SIGKILLed the highest-badness task — on Aug-14 typically the
   **agent process**, which is why needle recorded `exit −1` instead of a git exit code.
5. The retry layer re-dispatched the identical task 132 times: **129 × exit −1** + **1 × exit
   124** (607.6 s, the 600 s cap) + **1 × exit 0** (40.1 s at 23:25:35Z), then `bead.orphaned`.

**The kill durations are flat across all 129 attempts.** That is the decisive observation: the
object set never shrank between attempts, so every attempt re-read the same 17 GB and died at
the same point. A surviving transcript proves mid-gc death — the 13:55:24.357Z attempt ends
*"The git gc process is running successfully. Let me wait a bit more"* plus a `sleep 10 && tail`
progress check, killed 10.7 s later.

### 2.2 Event 2 — the Aug-17 post-completion turn exhaustion

Corrected timeline, all UTC, from the preserved trace (`73176de` §11.6):

```
16:59:45  session start
16:59:52  count-objects  →  444.24 MiB pack ALREADY present, 9 loose objects
17:00:14  git gc --aggressive --prune=now launched (PID 1112553)
17:05:33  git fsck --full  → times out, exit 143
17:05:52  count-objects  →  3 loose, 7753 in-pack, pack size unchanged
17:06:02  first bead close attempt   ── completion → death ≈ 90 seconds
17:06:59  error_max_turns
17:12:09  bead bf-173o7e closed by a later attempt
```

The close attempts failed **diagnosably, not opaquely** (`73176de` §11.7): the close script
resolved the repo from the shell's cwd and printed `Repo: /home/coding/pdftract` — the wrong
workspace — and its kubeconfig prerequisite was absent, so verification could not run. Two
environment problems were visible in the very first attempt's output; the remaining turn budget
went to retries (4 × `bead close`, 1 × `bead update --status closed` → exit 4, `bead close
--help`, `which`/`type`) instead of the error text. The banner's suggested `--skip-verify`
bypass is not a `bead close` flag (`6369467`), so that path could not have succeeded either.
Two later findings complete this picture:

- The missing `iad-ci.kubeconfig` was **correct behavior**, not a tooling bug — the file
  genuinely did not exist until 2026-08-25 (`5a0f127`).
- The in-attempt "repository verified" claim does not hold: `git fsck --full` timed out
  (exit 143); integrity is established by the later 2026-09-02 fsck-clean re-verification
  (`db1acb3`, `9f9930d`).

---

## 3. Evidence and Findings

### 3.1 Surviving primary evidence (re-verified live 2026-09-06)

`.beads/traces/bf-173o7e/` still holds the Aug-17 dispatch (traces are **single-slot** — last
dispatch only):

```json
{
  "bead_id": "bf-173o7e", "agent": "claude-code-glm-4.7", "provider": "zai",
  "exit_code": 1, "outcome": "failure", "duration_ms": 444317,
  "captured_at": "2026-08-17T17:06:59.953876423Z"
}
```

The trace's final tool call is the close attempt —
`bead close bf-173o7e --reason "Git gc completed successfully" --repo /home/coding/domain-check
--skip-verify` — followed immediately by the terminal `{"type":"error","message":"error_max_turns"}`.

### 3.2 Evidence that has rotated (permanently attested-only)

| Class | Status |
|---|---|
| Aug-12/13/14 **kernel** OOM records | Gone — current boot began 2026-08-15. The storm's mechanism is established by corroboration (identical `CONSTRAINT_MEMCG` kills hugging the 12 GiB cap on Aug-16), not by a per-event kernel line |
| Aug-14 **git-side** forensics | Gone — reflog truncated to Sep-1, no `gc.pid`/`gc.log`, sole current pack written Sep-2 11:09 (`d283576`) |
| Aug-14 needle counts | Attested by the committed storm RCA (`07ab240`), re-derived from the primary log on 2026-09-02 while the Aug-14 log still existed; that log has since rotated |
| `94.71 %` oomd pressure, "826 crashes", "201+ crashes", "40 % / 60 %" rates | **[REPORTED]** volumetrics — not reproducible from any surviving source; quote only as attested figures |

**Consequence:** no further re-derivation of Event 1 is possible. The committed record is
final; future documents should cite it, not re-derive it.

### 3.3 Current state `[LIVE 2026-09-06]`

```bash
du -sh .git                                   # 93M
git count-objects -vH                         # 84 loose; 1 pack, 90.43 MiB; 0 garbage
./scripts/setup-git-gc-config.sh --verify     # exit 0 — worst case ≈3072 MiB, within the
                                              # 6 GiB ceiling for a 12 GiB dispatch scope
bead show bf-173o7e                           # Closed
```

Fleet-wide: zero `exit_code=-1` since 2026-08-26; zero real kernel memcg kills since
2026-08-17 (all 15 since then are 2026-09-02 synthetic-test and bounded-gc-scope kills). The
current dominant failure class is synchronized `exit_code=1` waves — service-class, not signal
deaths.

---

## 4. Root Cause Analysis

### 4.1 Triggering conditions (Event 1)

1. **Unbounded gc command in the bead text** — no `pack.windowMemory`/`pack.threads` bound
   existed (fixed 2026-09-02, `533cb46`).
2. **Pathological repository state** — 17.20 GiB loose objects (crash debris of the
   bf-1s6c3 lineage), 36× a healthy repo.
3. **Constrained dispatch scope** — 12 GiB memcg, far below the worst-case footprint of an
   unbounded `--aggressive` repack of that object set.
4. **Redelivery with no circuit breaker and broken success handling** — exit-0 went
   unrecognized, so a doomed task was retried 131 times. This is what turned one crash into a
   storm and the storm into ~129 duplicate alerts.

### 4.2 Alternatives ruled out

| Candidate | Verdict | Basis |
|---|---|---|
| SIGHUP cascade / fleet-wide event | ❌ | At each kill exactly one agent died; nearest sibling completions exited 0. SIGHUP is catchable and would record as signal 1 |
| Timeout governor | ❌ | 129 kills at 21.6–216.6 s sit far below the 600 s cap one surviving run visibly hit (exit 124 at 607.6 s) |
| "gc finished in background, agent killed after" | ❌ | Transcript proof: gc actively running seconds before kills; completion came from the exit-0 storm attempt and a later non-attempt process |
| Host-wide OOM | ❌ | ~45 GiB host RAM free mid-storm; all boot OOM kills are `CONSTRAINT_MEMCG` |
| domain-check code defect | ❌ | Crashing process was the agent scope running the bead's git command; zero panics or stack traces corpus-wide |
| Aug-17 `exit 1` as "the" crash | ❌ (different event) | Post-completion close exhaustion; its "exit 1, NOT −1" statements apply only to Aug-17 |

### 4.3 Where this bead sits in the corpus's five layers

`bf-173o7e` is the canonical instance of **L1** (unbounded memory inside a bounded cgroup) and,
through its 129 duplicate alerts, of **L2** (detection/alerting defects). It is *not* an
instance of L3 (needle crash-handler release-conflict kill), L4 (service-class exit-1 waves),
or L5 (domain-check code — the corpus-wide null result). Full layering:
`docs/investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md` §3.

---

## 5. Superseded Claims — Do Not Propagate

This section extends the corpus's canonical superseded-claims list (corpus report §7). Claims
below appeared in the 2026-09-01 version of this file and remain reachable from it and its
siblings; none should be repeated as fact.

| Superseded claim (2026-09-01 version) | Corrected to | Source |
|---|---|---|
| "Crash came 4 hours after completion" | UTC/EDT mixup — real gap **≈90 s** (gc done ~17:05:30Z, first close attempt 17:06:02Z, death 17:06:59Z) | `73176de` §11.6 |
| "18 GB → 445 MB (97.5 % reduction) by this attempt" | The 444.24 MiB pack was **already present** at this attempt's first command; its delta was 9 → 3 loose objects, pack size unchanged | `73176de` §11.3 |
| "systemd-oomd at 94.71 % pressure → SIGHUP cascade" | Not a surviving record; mechanism is kernel **memcg-OOM SIGKILL** in a 12 GiB dispatch scope. `exit −1` is needle's sentinel for an unrecorded signal death, not a signal number | `5d501a8`, corpus §7 |
| "CPU saturation caused 826 crashes on 2026-08-16" | [REPORTED] volumetric, unreproducible. Live re-derivation: 414 kernel memcg kills on Aug-16 (two waves), 257 `git` + 156 `node` victims, all `CONSTRAINT_MEMCG` | corpus §4, [LIVE] |
| "Repository integrity verified via fsck during the attempt" | In-attempt `git fsck --full` **timed out** (exit 143); integrity rests on the 2026-09-02 re-verification | `73176de` §11.5 |
| "No clear error message" from the close attempts | The first attempt's failure was **explicit** (wrong-cwd repo resolution + missing kubeconfig prerequisite) | `73176de` §11.7 |
| "`--skip-verify` is the documented bypass" | Not a `bead close` flag; the banner naming it belongs to the close *wrapper* | `6369467` |
| "Aug-17 kubeconfig failure = tooling bug" | **Correct behavior** — the file genuinely did not exist until 2026-08-25 | `5a0f127` |
| "SIGHUP handling remediation fixed the crashes" | `7bed0a3` is defensive hardening against a failure mode that never occurred; the kill mechanism was bounded-cgroup OOM | solutions doc §1.6 |
| "bf-173o7e has an unresolved verification blocker" | The blocker does not exist; `verify-work-completion.sh` has no cluster logic and `bead close` has no such flag | `6369467` |

---

## 6. Solutions and Preventative Measures

Merged from the solutions deliverable (`docs/investigations/solutions-domchk-ece81e17-2026-09-06.md`,
parent bead `domchk-ece81e17`, closed). Every control below was re-executed live on 2026-09-06.

### 6.1 Implemented — the kill mechanism

| Control | Reference | Live verification |
|---|---|---|
| Pack memory bounds on the bare path | `533cb46` — `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1`, repo-local **and** global | `--verify` exit 0, worst case ≈3072 MiB |
| Effective-bound verifier | `scripts/setup-git-gc-config.sh --verify` — resolves system → global → local, exits non-zero if unbounded or threads unpinned | exit 0 `[LIVE 09-06]` |
| Bounded maintenance path | `scripts/safe-git-gc.sh` (stages, checkpoint/resume, `SAFE_GC_MEMORY_MAX`), `safe-git-gc-monitor.sh --watch` | in tree |
| Scheduled, memory-limited maintenance | `domain-check-git-gc{,-full}` timers (daily 03:00 / Sun 04:00, **`MemoryMax=4G`**), repo-health timer 02:00 | 6 `domain-check-*` user timers with future trigger times |
| Reproduction test | `scripts/test-gc-memory-bounds.sh` — re-runs the *exact* crash command under `MemoryMax=768M`: exit 0, pack-objects peak RSS ≈313 MiB | **12/12 passed** 2026-09-06 |
| Bloat guards (amplifier) | `.gitignore:66` excludes `.beads/`; pre-commit hook blocks >10 MB files; daily repo-health check | in tree |

⚠ `pack.windowMemory` is **per-thread** — pinning it without `pack.threads=1` multiplies the
bound by core count. The verifier rejects that state, and the test suite asserts it.

### 6.2 Implemented — detection and alerting

`scripts/crash-alert-manager.sh` + `crash-classifier.sh` + `alert-deduplication.sh` +
`crash-circuit-breaker.sh`: closed-bead filtering before alerting, duplicate detection,
exit-code validation, FALSE_POSITIVE/SERVICE_FAILURE/INFRASTRUCTURE/CODE_DEFECT classification,
crash-storm circuit breaker, 5-minute cooldown. `scripts/test-crash-alert-fixes.sh` → **12/12**
(2026-09-06). Plus `scripts/verify-work-completion.sh` as the pre-close gate — the repo-side
half of upstream R1, and the artifact that makes post-completion kills distinguishable from
mid-task ones at triage.

Application hardening (`internal/server/safeguards.go` panic recovery and request timeouts,
`98ab63e`; SIGHUP in the signal set, `7bed0a3`) is **defense in depth, not the fix** — it
addresses a failure class that never occurred in this corpus.

### 6.3 Open — the two repo-side gaps (implementation-ready, unowned)

1. **R6 — make the bound observable.** No scheduled script calls `setup-git-gc-config.sh
   --verify`; a config regression would be rediscovered, not alerted. Wire it into
   `check-repo-health.sh` and `preflight-health-check.sh` (≈5 lines each).
2. **R7 — cgroup-scoped OOM detection.** `resource-monitor.sh` watches host-wide signals only,
   which is exactly the blind spot that produced the original SIGHUP misdiagnosis and let 414
   Aug-16 kills happen under a green dashboard. Alert on a non-zero delta of `memory.events`
   `oom_kill` for `needle.slice` dispatch scopes, or on `Memory cgroup out of memory` journald
   lines per interval. **Highest-value detection fix open in this repo.**

Do **not** restore host-wide memory alerting as the OOM control — it structurally cannot see a
cgroup-scoped kill.

### 6.4 Open — upstream NEEDLE (P0; no workspace substitute exists)

| # | Fix | Evidence |
|---|---|---|
| R1 | Work-completion detection in the alert source (bead-closed event + `.beads/state/work-completion/` marker) | Post-completion kills alert as crashes (bf-5tgsk: completed 16:35:54, killed 16:36:24, closed 16:36:51) |
| R2 | One event → one alert, with cooldown | bf-173o7e drew **129 duplicate alerts** |
| R3 | Crash handler must treat `bead release` exit 4 (already closed) as "nothing to release", not fatal | **18 worker deaths on 2026-08-26** |
| R4 | Validate auto-split premises against live bead state; refuse splits onto closed beads or citing nonexistent files/flags | bf-4ifshb, bf-1cd5v6 (closed 3×), the fabricated bf-173o7e "verification blocker" (`6369467`) |

---

## 7. Lessons Learned and Recommendations

1. **Read `exit_code=-1` as a sentinel, never as a signal number.** It is needle's
   `code().unwrap_or(-1)` for an unrecorded signal death. The −1-as-SIGHUP reading is what
   produced the original misdiagnosis and a week of wrong remediation.
2. **Classify before investigating — and establish which event you are investigating.** A bead
   with 132 dispatches has many terminations; this bead's two were conflated for a week, and
   the wrong one got the root-cause title.
3. **Never reconstruct event order from needle alert timestamps.** They are
   `HANDLING_RELEASE_DONE` heartbeats 8–120 s after the real `agent.completed` kill. Derive
   kill boundaries from the session event log.
4. **Scope every quantitative claim to its attempt.** Attribution without the attempt's own
   first-command output is how "97.5 % reduction" got attached to an attempt that moved 6
   objects. The trace's opening state is the baseline.
5. **A retry loop without completion detection turns one crash into a storm.** 129 flat kill
   durations over 10.5 h is the signature of a fixed, unretried-into-success failure — and the
   failure to recognize the one exit-0 is why the bead needed manual closure.
6. **Commit evidence while the primary sources exist.** The Aug-14 kernel and needle logs have
   rotated; this event can never be re-derived again, only re-attested. Uncommitted evidence is
   how that loss happens.
7. **Close scripts must resolve the bead's workspace, not the shell's cwd**, and verification
   prerequisites should be preflighted before the close attempt (Event 2's proximate cause).
8. **Before writing any corpus document:** `git log --grep <bead-id>`, search
   `docs/investigations/`, check the target bead's status. If the question is closed, close the
   bead with a pointer instead of a new document — this report exists because three parallel
   chains re-synthesized a concluded investigation.
9. **Provenance-tag every figure** `[LIVE]` / `[COMMIT]` / `[REPORTED]`, and cite the canonical
   superseded-claims list (§5 above, corpus report §7) rather than restating corrected claims.

---

## 8. Actionable Next Steps

In priority order. Items 1–2 are repo-side and small; 3–4 are the decision items; 5 is the
upstream ask.

1. **Wire the bound into the health checks (R6).** Call `scripts/setup-git-gc-config.sh
   --verify` from `scripts/check-repo-health.sh` and `scripts/preflight-health-check.sh`; on
   non-zero exit, emit the existing critical-severity alert. ≈5 lines per script.
2. **Add cgroup-scoped OOM detection to `scripts/resource-monitor.sh` (R7).** Alert on non-zero
   `memory.events` `oom_kill` delta for `needle.slice` dispatch scopes. Create beads for 1–2
   rather than leaving them in prose.
3. **Do not re-investigate this bead.** The Aug-14 mechanism is determined (HIGH confidence,
   `07ab240`), the Aug-17 event is validated (`73176de`), the work is complete, the bead is
   closed, and the repo is healthy. Duplicate alerts on bf-173o7e should be closed with a
   pointer to this report — the corpus records 129 of them already.
4. **Do not run bare `git gc --aggressive`, and do not loosen the bounds.** Use
   `scripts/safe-git-gc.sh`. The bound firing as designed on 2026-09-02 *was* the fix working.
5. **Escalate R1–R4 upstream to NEEDLE** — completion detection, dedup, the release-conflict
   kill, auto-split premise validation. Repo-side wrappers only absorb these after the fact.
6. **Housekeeping (corpus R8/R9):** commit the untracked investigation documents (including the
   sibling chain's evidence compilation, which has three beads blocked behind it); for bf-31mno
   (434 kills, no RCA), cross-reference only if its primary evidence has rotated — do not open
   a chain.

### Parent acceptance criteria — verification

Parent bead `domchk-ece81e17` (closed) required: fix implemented with code references
(§6.1–§6.2 — commits `533cb46`, `98ab63e`, `7bed0a3`, script paths and line references);
monitoring improvements (§6.2–§6.3 — operating timers plus the two named gaps); testing
strategies (§6.1 reproduction test, §6.2 alert-logic suites, both 12/12 on 2026-09-06); code
patterns/practices to avoid (§7 items 1, 4, 5, 6, 7 and §8.4); process changes for future
investigations (§7 items 2, 3, 8, 9). All five are covered above and in the parent's own
deliverable; this report adds the merge, the corrections, and the actionable ordering.

---

## 9. Provenance

- **[LIVE 2026-09-06]** — trace metadata and terminal trace lines (`.beads/traces/bf-173o7e/`);
  `du -sh .git` (93M); `git count-objects -vH` (84 loose, 1 pack 90.43 MiB, 0 garbage);
  `setup-git-gc-config.sh --verify` (exit 0, ≈3072 MiB worst case); `bead show bf-173o7e`
  (closed); `bead show domchk-7880ada7` (this task).
- **[COMMIT]** — storm counts and mechanism (`07ab240`, `46f0360`, `db1acb3`); Aug-17 trace
  validation and corrections (`73176de`); crash-timing verdict DURING (`d283576`); original-bead
  context (`9d7e810`); nonexistent-blocker finding (`6369467`); kubeconfig correctness
  (`5a0f127`); pack bounds (`533cb46`); safeguards (`98ab63e`); SIGHUP handling (`7bed0a3`);
  solutions deliverable (`3e85864`); corpus final (`c700252`).
- **[REPORTED], quote-only** — "94.71 % oomd pressure", "826 crashes in a day", "201+ crashes",
  "40 % false-positive / 60 % duplicate" rates. None is load-bearing in this report.
- **Limitations.** Aug-12/13/14 are permanently attested-only (§3.2). Scope is
  single-workspace; fleet-wide figures rest on the corpus's cross-checks. The in-attempt
  16:21:24Z Aug-17 dispatch has no transcript (single-slot retention) and cannot be reconciled.

### Related documents

| Document | Path |
|---|---|
| Corpus-wide final report (L1–L5, R1–R12) | `docs/investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md` |
| Storm root-cause determination | `docs/investigations/bf-173o7e-root-cause-determination-domchk-2e371a2c-2026-09-02.md` |
| Storm RCA (full) | `docs/crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md` |
| Crash-timing vs gc completion | `docs/investigations/bf-173o7e-crash-timing-vs-gc-completion-domchk-536862b8-2026-09-02.md` |
| Original-bead context | `docs/investigations/bf-173o7e-original-bead-context-domchk-8304c1c0-2026-09-02.md` |
| Solutions and preventative measures | `docs/investigations/solutions-domchk-ece81e17-2026-09-06.md` |
| Sept-1 RCA + validation addendum §11 | `docs/root-cause-analysis-bf-173o7e-2026-09-01.md` |
| Signal sentinel decode | `docs/analysis/signal-analysis.md` |
| Maintenance guide (gc bounds) | `docs/maintenance/repository-maintenance-guide.md` |
| Crash response guide | `docs/crash-response-guide.md` |

---

*Final — `domchk-7880ada7`, 2026-09-06. Supersedes the 2026-09-01 version of this file
(`232bc73`). Upstream asks: R1–R4. Repo-side follow-ups: R6, R7.*
