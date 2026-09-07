# Audit: scripts/cleanup-bloat.sh vs crash-prevention acceptance criteria

**Bead:** domchk-7de40513 (read-only child of domchk-f411b534, "Review and update
cleanup scripts for crash prevention"). **Date:** 2026-09-07.
**Scope:** `scripts/cleanup-bloat.sh` (809 lines, working tree) and
`scripts/test-cleanup-bloat.sh` (260 lines). No code changed in this child.

## Provenance of the audited code — read this first

The audit target is **uncommitted working-tree state**:

- HEAD (`f21e381`) ships a **21-line** `scripts/cleanup-bloat.sh` whose body is
  exactly `git gc --aggressive --prune=now` — the death command from bf-1s6c3
  (18 GB repo / 17 GB loose objects → memcg-OOM SIGKILL mid-gc).
- The 809-line crash-safe rewrite exists only as uncommitted changes
  (`git status`: ` M scripts/cleanup-bloat.sh`, +802/−14 vs HEAD), and
  `scripts/test-cleanup-bloat.sh` is **untracked** (`??`).
- `docs/repository-health.md:110-119` still documents the script as "Runs
  `git gc --aggressive --prune=now`" and recommends it quarterly and in the
  "Repository is Too Large" troubleshooting path — accurate for HEAD, dangerous
  if anyone follows it before the rewrite lands.

Every finding below refers to the **working-tree** version. Until the rewrite
(and its test) are committed, HEAD behavior and documented behavior are two
different programs. Committing the rewrite is the parent bead's core deliverable
and is gap 1 below.

## Verdict per parent criterion

| Parent criterion | Verdict | Basis |
|---|---|---|
| Memory usage monitoring | **partial** | Process-tree RSS sampler + kill-guard verified live; but pre/post fsck runs unmonitored, guard kills only the root pid, no hard cgroup bound (gaps 3, 5, 6) |
| Checkpoint/resume | **met** | Per-stage checkpoint + `--resume` verified by suite T6/T7 (49/49 pass across two runs); completed/failed checkpoint semantics correct |
| Pre-flight checks | **met** (disk / memory / load) **partial** (integrity) | All three resource gates verified live on this repo; the fsck integrity gate has no memory bound (gap 3) |
| Progress indicators + verbose logging | **met** | Timestamped leveled log, per-stage elapsed/size/peak, heartbeat, dry-run plan, log rotation — verified live; minor nits (gaps 8-10) |
| Safe testing | **partial** | Suite is safe by construction (throwaway `/tmp` repos) and went green on re-run (49 pass / 0 fail / 1 skip, exit 0), but the first run failed 9 assertions from a proven fixture race, the self-test is not hermetic, and neither file is committed (gaps 1, 2, 4, 7) |

## Ordered, actionable gaps

Ordered most-important first. Severity: **HIGH** = blocks the criterion's
intent; MED = real defect, bounded risk; LOW = polish.

### 1. HIGH — Commit the rewrite; HEAD still ships the bare-gc one-liner

`scripts/cleanup-bloat.sh` is ` M` (+802/−14, uncommitted) and
`scripts/test-cleanup-bloat.sh` is `??`. At HEAD the script *is* the bf-1s6c3
death command, and `docs/repository-health.md:81,113,118,199,212` still instruct
running it. **Action (implementation child):** commit the 809-line script + its
test together, and update `docs/repository-health.md` to describe the staged,
monitored behavior and point routine maintenance at `safe-git-gc.sh`
(the script's own header, `scripts/cleanup-bloat.sh:55-57`, already says this).

### 2. HIGH — Test fixture races the box-global `gc.auto`; suite is flaky here

`make_bloated_repo` (`scripts/test-cleanup-bloat.sh:67-88`) does not pin git
config in the throwaway repos, so they inherit `~/.gitconfig`'s
`gc.auto=256` / `gc.autoPackLimit=10` (resolved via
`git config --show-origin --get gc.auto` → `file:/home/coding/.gitconfig 256`).
A 120-commit fixture produces ~363 loose objects, so `git commit`'s built-in
`gc --auto` can pack it mid-build.

Proven twice on this box, same code:
- Suite run 1: fixture reported **50 loose objects** → heuristics correctly said
  "not needed" → 9 assertions cascade-failed (T4/T5), 40 pass / 9 fail.
- Suite run 2 (minutes later): fixture held **364 loose** → **49 pass / 0 fail /
  1 skip, exit 0**.
- Standalone repro: 120 commits without `gc.auto=0` → **68 loose / 1 pack**;
  with `git config gc.auto 0` in the fixture → **360 loose / 0 packs**.
  `cleanup-bloat.sh --check-only` returned 1 (not needed) on the former and
  0 (needed) on the latter — the script is right both times; the fixture is
  what flakes.

The 60-commit fixtures (T6-T8, 184 loose) sit under the 256 threshold, which is
why only T4/T5 ever failed. **Action:** inside `make_bloated_repo`, set
`git config gc.auto 0` (and `gc.autoDetach false`) next to the existing
`user.email`/`user.name`/`commit.gpgsign` config at
`scripts/test-cleanup-bloat.sh:72-74`.

### 3. HIGH — Pre-flight and post-cleanup fsck run with no memory bound

`preflight()` runs `timeout "$FSCK_TIMEOUT" git fsck --connectivity-only`
(`scripts/cleanup-bloat.sh:425`) and `stage_verify()` the same at
`scripts/cleanup-bloat.sh:563`. `FSCK_TIMEOUT` (default 300 s,
`scripts/cleanup-bloat.sh:71`) bounds **time, not memory**, `git fsck` does not
honor `pack.windowMemory`, and neither invocation is attached to the RSS monitor
(`start_monitor` wraps only `run_monitored` stages,
`scripts/cleanup-bloat.sh:476-528`). The script's target scenario — the exact
bloated repo it exists to clean — is where `git fsck --connectivity-only` RSS is
largest, so the *pre-flight integrity gate can itself memcg-OOM before any
monitored stage runs*, on the very workload (bf-1s6c3's 18 GB / 17 GB loose
repo) this script was written for. **Action (pick one or more):** run fsck under
`start_monitor` with the same kill cap; or reuse `safe-git-gc.sh`'s hard
`systemd-run --scope -p MemoryMax` pattern (`scripts/safe-git-gc.sh:346-361`);
or skip fsck above a repo-size threshold with an explicit warning recorded in
the checkpoint (today a timeout degrades to a warning,
`scripts/cleanup-bloat.sh:426-427`, silently dropping the corruption gate).

### 4. MED — `--selftest-monitor` is not hermetic; it mutates the real repo's `.git`

The self-test path returns before `acquire_lock`
(`scripts/cleanup-bloat.sh:661-664` vs `:683-685`) and uses the live
`$GIT_DIR` for `cleanup-bloat.log`, `cleanup-bloat-monitor.stats`, and
`cleanup-bloat-monitor.violation` (`:171-174`, written at `:248-249`,
`:256-257`). Confirmed live: running the suite from this repo's root left
`stage=selftest` stats + violation files in this repo's `.git`, and the
self-test's `MEMORY CAP EXCEEDED during 'selftest'` lines were interleaved
*inside* a concurrent `--check-only` run's log lines. Two consequences:
(a) test noise lands in a real post-mortem artifact, and (b) because
`start_monitor` does `rm -f "$VIOLATION_FILE"` (`:235`) and rewrites
`$STATS_FILE` (`:248`), a self-test running concurrently with a real cleanup can
delete a live stage's violation evidence or corrupt its peak accounting.
**Action:** run the self-test inside a throwaway `GIT_DIR` (a temp
`git init`), or take the lock for the self-test path.

### 5. MED — Guard and interrupt kill only the root pid, orphaning pack-objects

The guard sends TERM then KILL to the watched pid only
(`scripts/cleanup-bloat.sh:258-260`), and the monitor loop exits as soon as that
pid dies (`:243`); `on_interrupt` likewise signals only `$GIT_PID`
(`:640-642`). `git repack` does its work in a child `pack-objects`, so killing
the parent orphans the child — it keeps running, unmonitored and unbounded by
this script. Residual risk is bounded by `pack.windowMemory=2g` /
`pack.threads=1` (`:469-471`, worst case ≈3 GiB per the repo's gc-bounds
analysis), but the cap the operator configured is not what bounds the orphan.
**Action:** run git in its own process group and kill `-- -PGID`, or `pkill -P`
the descendants before declaring the stage dead.

### 6. MED — No hard cgroup bound; the RSS sampler is best-effort polling

The kill fires only on a *sampled* read above the cap
(`scripts/cleanup-bloat.sh:253-262`) at `MONITOR_INTERVAL=2s` default (`:72`), so
an intra-interval spike can overshoot arbitrarily before detection (the peak is
recorded, but nothing pre-emptively bounds it). The repo already has the
stronger pattern: `safe-git-gc.sh` wraps every git invocation in
`systemd-run --user --scope -p MemoryMax=…`
(`scripts/safe-git-gc.sh:317-361`), making the ceiling cumulative and
authoritative. **Action:** wrap each stage (or the whole run) in a MemoryMax
scope and keep the sampler for observability and early, gentler aborts.

### 7. MED — Test coverage gaps in the safety-critical paths

The suite (`scripts/test-cleanup-bloat.sh`) never exercises:
(a) the guard killing a **real git stage** — the rc=125 → "failed" checkpoint →
`--resume` retry path (`scripts/cleanup-bloat.sh:505-513`); only the python-hog
self-test is run (T2); (b) SIGINT/SIGTERM trap → "interrupted" checkpoint +
exit 130 (`:635-645`); (c) the stale `gc.pid` lock path (`:329-344`) — T9 covers
only `cleanup-bloat.lock`; (d) `--check-only` under a live lock, a documented
contract (`:24-25`, `:683-685`); (e) fsck-corruption abort (`:428-431`);
(f) disk and load gate aborts — only the memory gate is tested
(`scripts/test-cleanup-bloat.sh:144`); (g) `clean_stale_tmp` (`:582-590`);
(h) `--force` override (`:708-716`). **Action:** add cases in that order; (a)
and (b) are the two paths a real crash will actually hit.

### 8. LOW — Log file embeds ANSI color codes

`log*` write `echo -e` output straight to `$LOG_FILE`
(`scripts/cleanup-bloat.sh:140-150`), so `.git/cleanup-bloat.log` (confirmed
live) carries raw escapes — noisy for grep/journal append consumers and for the
post-mortem role the header claims for it (`:654-655`). **Action:** gate color
on `[ -t 1 ]` or `NO_COLOR`.

### 9. LOW — Log rotation keeps one generation; check-only mutates despite its doc

`main()` rotates `cleanup-bloat.log` → `.log.1` with `mv -f` on every run
(`scripts/cleanup-bloat.sh:656-659`), including `--check-only`, whose help
promises "change nothing" (`:18`, `:24-25`). Verified live: a check-only run
rotated the prior log; the next run would destroy it. The crashed run's log is
the primary post-mortem artifact, and one generation is thin. **Action:** skip
rotation for `--check-only`/`--selftest`, and keep a timestamped or `.log.2`
generation.

### 10. LOW — Smaller correctness notes

- `pack.deltaCacheSize` is a fixed 256 MB (`scripts/cleanup-bloat.sh:74`) while
  `--memory-max` accepts 64 MB (`:129-132`); configured git memory can exceed
  the operator's cap. Derive the delta cache from `MEMORY_MAX_MB`.
- Checkpoint `peak_rss_kb` (`:300`) reflects only the most recent stage because
  `$STATS_FILE` is removed at each stage start (`:489`).
- Resume across modes only warns (`:751-753`); resuming an aggressive
  checkpoint whose last stage was `reflog` under `--conservative` logs "Resuming
  after stage 'reflog'" but re-runs from index 0 (reflog is absent from the
  conservative stage list, `:734-737`). Harmless (idempotent) but the message
  misleads.
- `acquire_lock` liveness checks are `kill -0 $pid` (`:322`, `:332`) — PID reuse
  can make a stale lock look live. Fails safe (aborts, `--force` escape), so
  acceptable; note only.
- Tree-RSS sums shared pages once per process (`:191-223`), over-counting
  shared pages — conservative direction; acceptable.
- Suite T5's stats assertion is best-effort: it skips when stages finish before
  the first 2 s sample (`scripts/test-cleanup-bloat.sh:162-167`; the one SKIP in
  both runs). Consider `CLEANUP_BLOAT_MONITOR_INTERVAL=1` in T5 to usually
  exercise it.
- `scripts/README.md` does not mention `cleanup-bloat.sh` or its test at all
  (no grep hits), so the suite is undiscoverable from the scripts index.

## What was verified live (this audit, read-only)

- `bash -n` clean on both scripts. (shellcheck not installed on this box.)
- `./scripts/test-cleanup-bloat.sh` — run 1: 40 pass / 9 fail / 1 skip (fixture
  auto-gc race, gap 2); run 2: **49 pass / 0 fail / 1 skip, exit 0**. The suite
  builds only throwaway `/tmp` repos; the real repo is untouched by design
  (`scripts/test-cleanup-bloat.sh:4-9`) — except via the T2 self-test leakage
  (gap 4).
- `./scripts/cleanup-bloat.sh --check-only` in this repo: exits 0 ("Cleanup
  needed: loose object count 278 > 100"), logs disk 84154 MB avail / 5120 MB
  required, memory 46686 MB avail / 10240 MB required, load 2.66 / max 15 —
  gates reported, not enforced, exactly as documented (`:18-25`, `:360-364`).
- `--selftest-monitor` (via suite T2): guard fired at peak 267.1 M against a
  96 M cap and killed the hog — the memory guard demonstrably kills.

## Not audited / out of scope

- Whether `cleanup_needed`'s thresholds (loose > 100 / > 100 M / packs > 10,
  `scripts/cleanup-bloat.sh:454-457`) are the right policy — behavior verified,
  policy is the parent bead's call.
- `cleanup-repo-bloat.sh`, `recover-repo-bloat.sh`, and the other cleanup-adjacent
  scripts (separate children of domchk-f411b534).
