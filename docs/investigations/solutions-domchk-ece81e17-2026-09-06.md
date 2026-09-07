# Solutions and preventative measures — 2026-09-01 crash corpus

**Deliverable:** solutions step of the 2026-09-01 chain (`domchk-ece81e17`). Upstream:
root-cause determination `domchk-6281555d` (commit 1592bf5,
`docs/investigations/root-cause-determination-domchk-6281555d-2026-09-06.md`, §9 handoff).
Downstream: report finalization `domchk-7880ada7`.
Corpus-wide context: `docs/investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md`
(§9 recommendations R1–R9 — this document implements none of R1–R5; it documents what is already
implemented and makes R6/R7 implementation-ready).

**Root cause being solved** (from the RCA): cgroup-scoped memcg-OOM SIGKILL of unbounded-memory
processes (bare `git gc --aggressive --prune=now` → pack-objects; node/vitest) inside per-dispatch
scopes with `MemoryMax=12 GiB`, amplified by repository bloat (18 GB / 17.16 GB loose at bf-1s6c3)
and multiplied by naive redelivery (bf-173o7e: 129 attempts). `exit_code=-1` is needle's
`unwrap_or(-1)` sentinel, not a signal number. **No domain-check defect appears anywhere in the
evidence.**

---

## Executive summary

The solution set for this root cause is **already largely implemented** in this workspace, and every
control below was re-executed live on 2026-09-06 before being documented here. The L1 kill
(unbounded pack-objects) is closed mechanically: the bare-gc path now carries hard memory bounds in
git config at both repo and global scope, the bounded `safe-git-gc.sh` path plus `MemoryMax`-limited
systemd services handle all scheduled maintenance, and a reproduction test re-runs the *exact* crash
command under a 768 MiB cgroup — it exits 0 with pack-objects peaking at ≈313 MiB (the original run
exceeded 12 GiB). The L2 amplifier (bloat) is guarded by `.gitignore`, a >10 MB pre-commit hook, and
daily repo-health checks. Detection-side (L2 alerts) has classification, deduplication, completion
awareness, cooldown, and a crash-storm circuit breaker, all tested 12/12.

What remains is deliberately **not** implemented here: two small repo-side observability gaps (§2.2 —
the bound is *enforced but unobserved*; cgroup-scoped OOM counts are invisible to host-wide
monitors) and four upstream NEEDLE fixes (§5.2) that no workspace change can substitute for.

---

## 1. Fix implementation details

### 1.1 L1 — bound the bare-gc path (the kill mechanism)

| What | Reference | Live verification (2026-09-06) |
|---|---|---|
| Pack memory knobs | commit **533cb46** — `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` in `~/.gitconfig` **and** repo `.git/config` | `git config --show-origin` shows all three from both scopes; `gc.autoPackLimit` 10 (global) / 50 (local) |
| Effective-bound verifier | `scripts/setup-git-gc-config.sh --verify` — resolves the chain a bare gc actually sees (system → global → local), exits 1 if unbounded or threads unpinned | **exit 0**; reports worst case ≈**3072 MiB** (2 GiB window × 1 thread + 1 GiB delta cache), within the script's 6 GiB ceiling for a 12 GiB dispatch scope |

⚠ `pack.windowMemory` is a **per-thread** limit. Pinning it without `pack.threads=1` multiplies the
bound by the thread count — which is why the verifier exits non-zero on an unpinned repo, and why
the test suite asserts that state is rejected (§3.1).

### 1.2 L1 — bounded maintenance path and scheduling

| What | Reference | Live verification (2026-09-06) |
|---|---|---|
| Bounded gc script | `scripts/safe-git-gc.sh` — default stages 1–2; `--full` adds deep compression; `--resume` continues from checkpoint (`.git/safe-gc-checkpoint.json`); `--check-only`; memory via `SAFE_GC_MEMORY_MAX` | modes/checkpoint confirmed in source; pre-flight integrity checks |
| Progress monitoring | `scripts/safe-git-gc-monitor.sh --watch` | — |
| Scheduled services | `scripts/domain-check-git-gc.{service,timer}` (daily 03:00), `scripts/domain-check-git-gc-full.{service,timer}` (Sun 04:00, **`MemoryMax=4G`**), `scripts/domain-check-repo-health.{service,timer}` (daily 02:00) | all 6 `domain-check-*` user timers listed with future trigger times (`systemctl --user list-timers`) |

⚠ Editing any `~/.config/systemd/user/domain-check-*` unit without `systemctl --user daemon-reload`
silently leaves the stale unit running — this bit the weekly full-gc on 2026-09-02.

### 1.3 L2 — repository bloat prevention (the amplifier)

- `.gitignore:66` — `.beads/` (the bf-1s6c3/bf-4yjq bloat was 17 × 237 MB `.beads/*.jsonl` files →
  18 GB repo where *any* git operation OOM-killed).
- `.git/hooks/pre-commit` (from `scripts/pre-commit-repo-size-hook`) — blocks files > 10 MB.
- Repo-size thresholds and detection commands in repo `CLAUDE.md` ("Repository Bloat Prevention");
  `scripts/check-repo-health.sh`, `scripts/repo-health-monitor.sh`, `scripts/cleanup-*.sh`,
  `scripts/recover-repo-bloat.sh`; daily repo-health timer (§1.2).
- Proven result: the same bloat went 18 GB → 138 MB (99.2 %) after cleanup with the task completing.

### 1.4 L2 — detection-side alert fixes

`scripts/crash-alert-manager.sh` (commit-era 2026-09-02, since extended) integrates:

- **CRITICAL FIX 1** (`:237`) — check target-bead closure *before* generating an alert (kills the
  "investigate a finished bead" class, e.g. bf-3561g → closed bf-4k2ws).
- **CRITICAL FIX 2** (`:260`) — duplicate alert-bead detection per target bead.
- **CRITICAL FIX 4** (`:297`) — exit-code validation before alerting.
- Crash-storm **circuit breaker** (`:204`; breaker logic in `scripts/crash-circuit-breaker.sh`,
  `BREAKER_THRESHOLD=3` consecutive same-bead crashes → OPEN, from the bf-65lsdu RCA §7) — converts
  a 127-alert storm into one alert and one deferral.
- Resolution tracking (`:222`), 5-minute cooldown, `--auto-process` mode.
- `scripts/crash-classifier.sh` — FALSE_POSITIVE / SERVICE_FAILURE / INFRASTRUCTURE / CODE_DEFECT,
  with the classification rules from `docs/crash-response-guide.md`.
- `scripts/alert-deduplication.sh` — duplicate detection primitives.

### 1.5 Pre-close completion gate

`scripts/verify-work-completion.sh <bead-id> --summary "..."` — run as the last step before
`bead close`. Verifies commits are pushed, expected artifacts exist, the bead carries a completion
note, and the box is healthy enough to finish; writes a durable marker at
`.beads/state/work-completion/<bead-id>.json` so post-completion kills are distinguishable from
mid-task ones at triage. Usage: `scripts/README.md`. This is the repo-side half of upstream
recommendation R1 (§5.2) — the alert *source* upstream still lacks it.

### 1.6 Application hardening — defense in depth, **not** the fix

`internal/server/safeguards.go` (commit **98ab63e**): `Recover()` (`:40`) converts request-path
panics into 500s with logging/metrics; `Timeout()` (`:106`) with `timeoutWriter` (`:161–224`) enforces
`DefaultRequestTimeout` without double-writing responses; graceful drain in `internal/server/server.go`
(`:137–161`, `Server.Shutdown` `:181`) records shutdown outcomes.

These safeguards address a failure class that **never occurred** in this corpus — every
investigation found zero domain-check defects. They are hardening, and the RCA's null result (§4
"Application (null result)") must not be read as "these fixed the crashes".

### 1.7 In-flight, uncommitted — explicitly *not* counted as implemented

The shared worktree holds uncommitted work from other beads: `internal/server/crashrecorder.go`,
`internal/server/resource_monitor.go` (+ tests), and edits to `cmd/domain-check/main.go`,
`internal/config/config.go` (its `:138` comment references the recorder),
`internal/server/{metrics,routes,safeguards,server}.go` and several `scripts/*.sh`. `go build ./...`
passes with them present, but none are committed, none are covered by this document, and any bead
claiming them must verify their owning bead's status first.

---

## 2. Monitoring and alerting improvements

### 2.1 Operating now (verified firing 2026-09-06)

| Timer | Cadence | Log |
|---|---|---|
| `domain-check-service-monitor` | 2 min | `.beads/logs/service-monitor.log` |
| `domain-check-resource-monitor` | 5 min | `.beads/logs/resource-monitor.log` |
| `domain-check-monitoring` (crash patterns) | 10 min | `.beads/logs/crash-monitor.log` |
| `domain-check-repo-health` | daily 02:00 | `.beads/logs/repo-health.log` |
| `domain-check-git-gc` (incremental) | daily 03:00 | — |
| `domain-check-git-gc-full` (`MemoryMax=4G`) | Sun 04:00 | — |

Install/refresh with `./scripts/setup-repo-maintenance.sh`; this box is NixOS — the crontab-based
`scripts/monitoring-setup.sh` does **not** work here.

### 2.2 Gap A — make the L1 bound *observable* (e843c4f1 R6; verified absent today)

`scripts/setup-git-gc-config.sh --verify` exists and is correct, but **no scheduled script calls
it**: neither `scripts/check-repo-health.sh` nor `scripts/preflight-health-check.sh` invokes it, and
`scripts/repo-health-check.sh:81` only *prints* a suggestion. A config regression (a tool clobbering
`pack.windowMemory`, a repo without the local keys and no global fallback) would be rediscovered, not
alerted. **Implementation (≈5 lines × 2 scripts):** call `scripts/setup-git-gc-config.sh --verify`
in `check-repo-health.sh` and `preflight-health-check.sh`; on non-zero exit, emit the existing
critical-severity alert (`repo_health.log` + threshold table in CLAUDE.md). The bound is today
*enforced* (§1.1) but *unobserved*.

### 2.3 Gap B — cgroup-scoped OOM detection (e843c4f1 R7; highest-value detection fix)

`scripts/resource-monitor.sh` watches `MemAvailable` and PSI memory pressure — **host-wide signals
only**. Every kill in this corpus was `CONSTRAINT_MEMCG` *inside* a dispatch scope while the host was
nowhere near exhaustion; that blind spot produced the original SIGHUP misdiagnosis and let the
2026-08-16 kills (414) happen under a green dashboard. **Implementation:** alert on a non-zero delta
of `memory.events` `oom_kill` for the dispatch scopes (`/sys/fs/cgroup/needle.slice/*/memory.events`),
or count `Memory cgroup out of memory` journald lines per interval. Either converts the corpus's
founding blind spot into a signal; the journald variant is one `journalctl --since` grep away.

### 2.4 Anti-recommendation — do **not** restore host-wide memory alerting as the control

Host-wide pressure alerts are structurally blind to cgroup-scoped kills (RCA §3.5): the memcg
accounting that kills these processes is invisible to host-wide `MemAvailable` until long after. The
correct instruments are §2.3's scoped counters and §2.2's bound verification.

---

## 3. Testing strategies to validate the fix

All of the following were **executed 2026-09-06** for this document; figures below are from those runs.

### 3.1 Reproduction under the bound — the decisive test

`scripts/test-gc-memory-bounds.sh` — **12/12 passed**:

- unit: all three pack keys land in a fresh repo; `--verify` passes in a bounded repo;
- unit: the safety core overrides a stale bound *without clobbering* unrelated gc policy
  (`gc.auto=7` preserved, `pack.windowMemory=128m` corrected);
- unit: `--verify` **rejects** an unbounded repo and **rejects** unset `pack.threads`
  (the per-thread multiplication trap);
- unit: `--verify` recognizes the box-wide global bound when a repo has none;
- integration: the *exact* bf-173o7e crash command, `git gc --aggressive --prune=now`, re-run under
  `MemoryMax=768M` on 24 loose objects of incompressible data (8 × 64 MiB) — **exit 0**, pack-objects
  peak RSS **320 400 KB (≈313 MiB)** vs > 12 GiB unbounded, repo fully packed.

This is the model for regression-proofing any future bound: re-run the crash command itself inside a
deliberately smaller cgroup and assert both exit status and peak RSS.

### 3.2 Alert-logic tests

`scripts/test-crash-alert-fixes.sh` — **12/12 passed** (closed-bead filtering, duplicate detection,
completion awareness, cooldown, processed-alerts tracking, FALSE_POSITIVE classification).
Companions: `scripts/test-crash-circuit-breaker.sh`, `scripts/test-alert-suppression*.sh`,
`scripts/test-preventive-measures.sh`, `scripts/test-mitigation-*.sh` (artifacts of parallel chains —
run the ones covering the control you touch).

### 3.3 Application tests

`go build ./...` and `go test ./...` (`internal/server`, `internal/config` green today). Fuzz
convention per `docs/plan/plan.md`: crash cases land in `testdata/fuzz/` and are committed, so every
plain `go test` replays them as regression tests.

### 3.4 Verification discipline

The corpus's most reliable failure mode was *documented figures that outlived their evidence*. Every
load-bearing number in this document was re-derived live (§7), matching the RCA §7 and the evidence
compilation exactly. Rule for future documents: **if a figure cannot be re-derived, tag it
[COMMIT]/[REPORTED] and say what would need to exist to upgrade it.**

---

## 4. Code patterns and practices to avoid

1. **Unbounded-memory operations inside bounded cgroup scopes.** Bare `git gc --aggressive
   --prune=now` (and node/vitest) under a 12 GiB `MemoryMax` is how agents died. Use the bounded path
   (§1.2) or confirm the effective bound first (§1.1).
2. **Treating `pack.windowMemory` as a process-wide cap.** It is per-thread; without
   `pack.threads=1` the "bound" scales with cores.
3. **Host-wide signals as proxies for scoped failures.** A green `MemAvailable`/PSI dashboard said
   nothing about 414 memcg kills (§2.3/§2.4).
4. **Reading `exit_code=-1` as a signal number or SIGHUP evidence.** It is needle's
   `unwrap_or(-1)` sentinel (RCA §1.1); "−1 storm" reasoning sent multiple chains down a wrong path.
5. **Committing bulk mutable state stores.** `.beads/*.jsonl` at 237 MB × 17 produced the 18 GB repo;
   keep `.beads/` ignored and the >10 MB hook active (§1.3).
6. **Applying repo-side fixes to upstream problems and vice versa.** Dedup/completion-awareness
   exist repo-side, but the *alert source* is upstream — repo scripts only absorb duplicates after
   the fact. Tag every recommendation with an owner (§5.2 does).
7. **Redelivery without completion detection.** bf-173o7e took 129 attempts with flat kill durations;
   naive retry multiplies a fixed crash into a storm.
8. **Silently loosening TLS verification.** The gateway health check needs `curl -skf` (self-signed
   cert); plain `-sf` reports "gateway down" while the gateway answers 200. Loosen explicitly and
   document why.
9. **Editing systemd units without `daemon-reload`.** The stale-unit trap silently disabled the
   weekly full-gc (2026-09-02).
10. **Trusting alert timestamps as death timestamps.** They are `HANDLING_RELEASE_DONE` heartbeats
    8–120 s after the real `agent.completed` kill; reconstruct kill boundaries from the session event
    log instead.

---

## 5. Lessons learned, process changes, and open work

### 5.1 Process changes (repo-side, adopt now)

- **Before writing any corpus document:** `git log --grep <bead-id>`, search `docs/investigations/`,
  and check the target bead's status (e843c4f1 R5). Three parallel chains re-synthesized this same
  concluded question after the anchor report; if the question is closed, close the bead with a
  pointer instead of a new document.
- **Run `scripts/verify-work-completion.sh` before every `bead close`** (§1.5) — it is what makes
  post-completion kills classifiable at triage.
- **Commit evidence while the primary sources exist.** Journald retention starts 2026-08-15; the
  Aug-12/13/14 kernel logs are gone and can only be re-attested from committed corpus documents
  (RCA §8). Uncommitted evidence is how the gap happens (e843c4f1 R8).
- **Alert handling order:** verify target-bead state → check for an existing committed report →
  refuse duplicate splits → clean stale labels → close (per the auto-split loop findings).
- **Provenance convention:** tag figures [LIVE]/[COMMIT]/[REPORTED]; name chain deliverables
  `<kind>-domchk-<bead-id>-<date>.md` under `docs/investigations/`.

### 5.2 Open upstream (NEEDLE) — P0, no workspace substitute exists

| # | Fix | Evidence |
|---|---|---|
| R1 | Work-completion detection in the alert source (bead-closed event + `.beads/state/work-completion/` marker) | bf-5tgsk: completed 16:35:54, killed 16:36:24, bead closed 16:36:51 — alerted as a crash anyway |
| R2 | One event → one alert, with cooldown | bf-173o7e drew **129 duplicate alerts**; ~60 % of the corpus's alerts were duplicates |
| R3 | Crash handler must treat `bead release` exit 4 (already closed) as "nothing to release", not fatal | **18 deaths on 2026-08-26**; the last fleet-wide signal-death day |
| R4 | Validate auto-split premises against live bead state; refuse splits onto closed beads or citing nonexistent files/flags | bf-4ifshb, bf-1cd5v6 (closed 3×), and the wholly fabricated bf-173o7e "verification blocker" (6369467) |

Also open: an early circuit-breaker on same-cause/same-command kill signatures *upstream* (the
repo-side breaker in §1.4 only absorbs what it sees); `bf-31mno` (434-kill storm, no RCA) should be
cross-referenced rather than investigated if its primary evidence has rotated (e843c4f1 R9).

### 5.3 Handoff to `domchk-7880ada7` (finalize)

- Merge order: anchor report + correction banner ← RCA `domchk-6281555d` (L1–L3, L5) ← this document
  (solutions) ← `investigation-report-final-2026-09-06-domchk-e843c4f1.md` (corpus-wide L1–L5, R1–R9).
- §2.2/§2.3 are the two remaining *repo-side* implementation items — small, scoped, and unowned; they
  should become beads rather than prose.
- §1.7 lists uncommitted worktree files that are **not** part of this corpus's implemented fix set;
  they belong to other in-flight beads and should not be folded into the final report's "implemented"
  section.

---

## 6. Acceptance criteria mapping

| Criterion | Where |
|---|---|
| Fix implemented, with code references | §1.1–§1.6 (commits 533cb46, 98ab63e; files/scripts/line numbers) |
| Monitoring improvements to detect similar issues | §2.1 (operating), §2.2–§2.3 (gaps, implementation-ready), §2.4 (anti-recommendation) |
| Testing strategies to validate the fix | §3.1–§3.4 (all suites executed 2026-09-06 with figures) |
| Code patterns / practices to avoid | §4 (10 items) |
| Process changes for future investigations | §5.1 (repo-side), §5.2 (upstream), §5.3 (handoff) |

---

## 7. Provenance

- **[LIVE 2026-09-06]** — `setup-git-gc-config.sh --verify` exit 0 (worst case ≈3072 MiB);
  `git config --show-origin` pack keys (global `~/.gitconfig` + repo-local);
  `test-gc-memory-bounds.sh` 12/12 (pack-objects peak RSS 320 400 KB under `MemoryMax=768M`);
  `test-crash-alert-fixes.sh` 12/12; `go build ./...` + `go test ./internal/server/
  ./internal/config/` green; all 6 `domain-check-*` timers with future trigger times;
  `.gitignore:66`; `.git/hooks/pre-commit` (>10 MB block).
- **[COMMIT]** — 533cb46 (pack bounds), 98ab63e (safeguards), 1592bf5 (RCA deliverable), and the
  crash-corpus documents cited throughout; source line numbers (`crash-alert-manager.sh` fixes,
  `safeguards.go`, `server.go`, `repo-health-check.sh:81`) read from the current tree.
- **[REPORTED] → resolved** — the RCA figures this document builds on were themselves re-derived live
  in RCA §7 on 2026-09-06 and matched the evidence compilation; no [REPORTED] figure is load-bearing
  here.
- Scope: this document covers the 2026-09-01 chain's solution set only. It defers to
  `root-cause-determination-domchk-6281555d-2026-09-06.md` for causality and to
  `investigation-report-final-2026-09-06-domchk-e843c4f1.md` for corpus-wide layering (L1–L5) and
  recommendation ownership.
