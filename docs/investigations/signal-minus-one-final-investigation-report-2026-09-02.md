# Final Investigation Report: signal −1 crash of 2026-08-14T13:55:36Z (gc storm, cycle 4)

**Deliverable bead:** `domchk-4c4a6163` ("Document findings and determine if action needed")
**Investigation chain:** `domchk-31e43626` (context) → `domchk-430bc424` (gc completion
verification) → `domchk-760530a8` (root cause) → `domchk-4c4a6163` (this report)
**Subject crash:** `exit_code = −1`, assigned timestamp 2026-08-14T13:55:36.566Z
**Report date:** 2026-09-02
**Verdict:** memcg OOM SIGKILL of the agent, mid-gc, inside its own 12 GiB dispatch
scope — **INFRASTRUCTURE**, not a domain-check code defect. Work was not lost; the
interrupted gc succeeded on retry. **No further code fixes required.**

This report consolidates the chain's findings. Deep dives are not repeated here:

- Root cause detail: [`docs/crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md`](../crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md)
- Signal semantics + crash/completion timing: [`signal-minus-one-root-cause-domchk-5b35d60e-2026-09-02.md`](signal-minus-one-root-cause-domchk-5b35d60e-2026-09-02.md)
- Mechanism (sibling bead): [`docs/crash-investigations/bf-4x12ec-root-cause.md`](../crash-investigations/bf-4x12ec-root-cause.md)

---

## 1. Crash context and timeline

The subject crash is one attempt of the **bf-173o7e storm**: 131 dispatch attempts for a
single gc bead ("Execute git gc --aggressive with pruning", created 2026-08-14T12:57:54Z,
a duplicate of sibling `bf-4x12ec`'s gc task), of which **129 exited −1** mid-gc, one hit
the 600 s cap (exit 124, 22:19:04Z), and one succeeded (exit 0, 23:25:35Z).

| Time (UTC, 2026-08-14) | Event |
|---|---|
| 12:57:54 | `bf-173o7e` created — prescribes `git gc --aggressive --prune=now` over **17.20 GiB loose objects** |
| 12:58:45 | Sibling `bf-4x12ec`'s 53rd and final attempt exits 0 — the storm opens 13 s later on the same lethal workload |
| 13:54:12.622 | Subject attempt claimed (dispatched 13:54:12.637Z) — **cycle 4 of 131** |
| 13:54:13 → 13:55:13 | Agent transcript `4a6daee4-aa55-4a1b-…jsonl` runs; it launches the gc and ends with *"The git gc process is running successfully. Let me wait a bit more"* + `sleep 10 && tail -50 /tmp/git-gc-domain-check.log` |
| **13:55:24.357** | **Kernel memcg OOM SIGKILLs the agent** (71.7 s run). Needle records `exit −1` — killed 10.7 s into its wait, while the gc child ran inside the same scope |
| 13:55:36.566 | `HANDLING_RELEASE_DONE` heartbeat — serialized into the alert as the (misleading) assigned crash timestamp, 12.2 s after the real kill |
| 15Z–19Z | Storm pauses (0 attempts) |
| 20Z–23Z | 78 further attempts (45 at 21Z, 15 at 22Z, 17 at 23Z + the 20Z one) |
| 22:19:04 | Sole exit 124 — needle's own 600 s timeout, proving timeouts were not the −1 cause |
| **23:25:35** | **Final attempt exits 0 (40.1 s) — the gc completes; repo fully packed** |
| 23:25:49 | `bead.orphaned` — released unassigned on success (worker-side defect, documented for bf-4x12ec) |
| 08-17 17:15 | `bf-173o7e` closed |
| 08-26 | Stale-alert regeneration (no bead-state checks) spawns investigation beads 12 days post-crash — including this chain |

**Why the assigned timestamp lies:** alert-family timestamps serialize release
bookkeeping heartbeats, not the kill. Always re-derive the kill from `agent.completed`
in `~/.needle/logs/claude-code-glm-4.7-lab-<worker>-<date>.jsonl`. Four such mappings
are now tabulated in the storm root-cause doc.

## 2. Verification that the gc completed successfully

**Verified.** Two independent verifications, 2026-09-02:

1. **`domchk-430bc424` (chain sibling):** `git fsck --full` exit 0; single pack
   `pack-98054595` (10,478 objects, 90.18 MiB) with an intact completion signature —
   `.bitmap` + `.rev` + `info/packs` all finalized 11:09:09, no `tmp_*`, no `.keep`, no
   `gc.log`; `.git` 93 MiB. (Its first fsck attempt exited 2, but that was an
   fsck-vs-concurrent-commit race, not corruption: another agent committed `eb04e23`
   mid-scan; the immediate re-run was clean.)
2. **This bead, fresh at report time:** `git count-objects -vH` → 104 loose objects /
   840 KiB, 10,478 in-pack, 1 pack, 90.18 MiB, zero garbage; `git fsck --full
   --no-reflogs` exit 0 (3 dangling commits = normal residue); `.git` 93 MB. `git
   status`, `log`, `cat-file`, and `ls-remote origin` all healthy.

Loose-object counts quoted across this investigation (58, 63, 73, 93, 104) are live
snapshots of a repo under concurrent docs commits, not growth — the pack is byte-stable
at 10,478 objects / 90.18 MiB, far under the 500 MB healthy threshold, and the daily
03:00 gc timer collects residue.

## 3. Root cause

**memcg OOM → SIGKILL inside the dispatch scope → needle `exit −1` sentinel.**

- **What −1 is:** not a signal number. Needle 0.6.0 records
  `ExitStatus::code().unwrap_or(-1)` — the sentinel for "died by a signal needle did not
  send". Needle's own timeout path returns 124 (this storm produced exactly one).
- **The mechanism:** `git gc --aggressive` computes delta chains across the whole object
  set in memory before writing pack bytes. Inside the transient `run-*.scope`
  (`MemoryMax = 12,884,901,888` = exactly 12 GiB, `memory.high = max`,
  `oom_score_adj = 200` — a *preferred* OOM victim, `memory.oom.group = 0`) the budget
  is exhausted in tens of seconds over 17.20 GiB of loose objects, and the kernel kills
  the highest-badness task — the agent itself, waiting on its own gc child. Limits were
  re-verified live on the investigator's own dispatch scope (pitfall: query with
  `systemctl --user`; the system manager reports `MemoryMax=infinity`).
- **Ruled out:** SIGHUP cascade / fleet event (only one agent died; nearest other
  completion was exit 0), timeout governor (129 kills at 21–217 s vs. one surviving
  607.6 s run), host-wide OOM (~45 GiB free mid-storm; exhaustion was memcg-scoped),
  domain-check code defect (the killed process was the agent scope running a git
  command embedded in the bead text).
- **Evidence limits:** Aug-14 kernel logs are rotated away (current boot begins
  2026-08-15); the mechanism is confirmed via the same-period Aug-16 journal (257
  `task=git` `CONSTRAINT_MEMCG` kills) and the mid-gc transcript proof in §1.

**Fleet context:** 247 signal crashes family-wide (245 on Aug-16, 1 on Aug-17, 1 on
Aug-26) — re-derived from `.beads/events.jsonl` at report time — and **zero since
2026-08-26 (7 days clean)**. The zero-backoff release→re-claim loop amplified one
hazardous workload into hundreds of deaths across workers; all sibling workers on Aug-14
were unaffected, confirming a scoped, not host-wide, kill.

## 4. Impact assessment — did the crash cause any problems?

**No lasting problems.** Explicitly:

- **No data loss.** The repo was fully packed by the storm's end and is healthy today
  (§2). No commits, refs, or working-tree files were damaged; fsck is clean.
- **No domain-check impact.** The service, code, and tests were never implicated — the
  crashing process was the agent scope, not domain-check. Zero code defects found.
- **No silent failure.** The gc's eventual success was verified, not assumed.

Real but bounded costs, all process-level:

| Cost | Magnitude |
|---|---|
| gc completion delay | ~10.5 h (12:58Z first claim → 23:25Z success) |
| Wasted agent attempts | 131 for bf-173o7e (~2.3 h agent-time) + 53 for sibling bf-4x12ec + 245 fleet-wide on Aug-16 |
| Alert/investigation noise | Stale-alert regeneration (no bead-state checks) spawned investigation beads 12 days after the crash, including this chain |

## 5. Recommendations for prevention

Already in place (all verified on this box, 2026-09-02):

1. **No bare `git gc --aggressive`** — policy since bf-1s6c3; use
   `scripts/safe-git-gc.sh` (staged, checkpoint/resume, `SAFE_GC_MEMORY_MAX` ceiling,
   pre-flight integrity checks). Enforced by repo CLAUDE.md and monitoring timers.
2. **Cgroup memory guard** (shipped 2026-09-02, commit `f0a7a81`):
   `scripts/cgroup-memory-guard.sh` + preflight Check 3 read the *dispatch scope's own*
   memcg headroom before memory-heavy work — the exact blind spot this crash exposed
   (`free -g` looks fine while the tightest bounded cgroup has none).
3. **Crash-alert hygiene** (2026-09-02): closed-bead filtering, duplicate detection,
   completion awareness, 5-min cooldown, classification in
   `scripts/crash-alert-manager.sh` — stops the stale-alert bead storm this chain grew
   from.
4. **Monitoring timers** (systemd user units): repo health daily 02:00, incremental gc
   03:00, full gc Sun 04:00 (MemoryMax=4G), crash-pattern/resource/service monitors —
   with `daemon-reload` after any unit edit.
5. **Work-completion marking** (`scripts/verify-work-completion.sh`) so crash triage can
   distinguish mid-task kills from post-completion false positives.

Open follow-ups (recommended, not required for this crash):

6. **Commit the untracked mitigation scripts.** `scripts/crash-circuit-breaker.sh`,
   `agent-concurrency-limiter.sh`, `needle-with-limiter.sh`, `memory-watch.sh` and their
   tests are load-bearing mitigations that currently exist only in the working tree — a
   workspace reset loses them. They are other workers' in-flight files, so this report
   does not commit them; each owner should.
7. **Remaining `exit 1` (max_turns) class is a separate, still-elevated failure mode**
   (44.2% Sep 01, 28.1% Sep 02) — not a signal crash, tracked separately; do not fold it
   into this mitigation set.

## 6. Decision: are fixes or mitigations required?

**No new fixes are required for this crash.** The root cause is external infrastructure
(memcg-scoped OOM under needle's 12 GiB dispatch scope), every mitigation for it already
exists and is holding — zero signal crashes in the 7 days since Aug-26, the repository
is 93 MB with a stable 90.18 MiB pack, and domain-check code remains defect-free. The
only recommended follow-up is housekeeping (item 6: commit the untracked mitigation
scripts) plus continued monitoring of the separate exit-1 class (item 7). Nothing in
this investigation warrants a change to domain-check source, its dependencies, or its
cluster manifests.

---

**Disposition:** findings consolidated; crash impact nil on work and service;
prevention measures in place and effective; no further action required beyond the
housekeeping follow-ups above. Report closes bead `domchk-4c4a6163`. 2026-09-02.
