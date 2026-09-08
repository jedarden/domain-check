# Crash Prevention Runbook — Daily/Weekly Operator Procedure

**Scope:** the domain-check workspace on the lab box. Every command below was verified to exist
with these flags on 2026-09-08 (domchk-a61cc009); the verified test evidence behind each layer is
[`docs/crash-prevention-testing.md`](../crash-prevention-testing.md).
**Companion docs:** design [`docs/crash-prevention-design.md`](../crash-prevention-design.md),
monitoring [`docs/crash-prevention-monitoring-design.md`](../crash-prevention-monitoring-design.md),
requirements [`docs/crash-prevention-requirements.md`](../crash-prevention-requirements.md),
maintenance [`docs/maintenance/repository-maintenance-guide.md`](../maintenance/repository-maintenance-guide.md).

**Environment facts that shape this runbook:** this box is NixOS — there is **no `crontab`**; all
scheduling is systemd **user timers**. `/usr/bin/time` does not exist (use
`/run/current-system/sw/bin/time -v`). `journald` timestamps are **local time**; needle crash
stamps are UTC.

---

## 1. Daily preflight (before starting agent tasks)

```bash
cd ~/domain-check
./scripts/preflight-health-check.sh          # exit 0 healthy, 1 unhealthy, 75 = deferred (system event active)
```

Exit **75** is a load-shedding directive (a system event is active via
`scripts/system-event-mode.sh`), **not a failed check**, and `--warn-only` does not relax it.
Add `--verbose` for per-check detail.

Fast one-shot variants of the same layers (all verified exit 0 on a healthy box):

```bash
./scripts/resource-monitor.sh --once         # memory / disk / load / PSI pressure / unsafe-gc config
./scripts/service-monitor.sh --once          # inference gateway health + resources
./scripts/check-repo-health.sh               # .git size, loose objects, packs, unpushed backlog, pack-memory bound
./scripts/crash-pattern-detection.sh         # last-24h crash census + surge detection
```

Resource floors enforced at preflight (see §6 for the full threshold table): abort a dispatch when
available memory < 10 GB, disk < 20 GB, or 1-min load ≥ 10.

Dispatch entry point: start agent dispatches through `./scripts/needle-with-limiter.sh` — it gates
`needle run`/`needle supervise` through the crash-storm circuit breaker
(`scripts/crash-circuit-breaker.sh`) and the concurrency limiter, so a bead in storm backoff is
deferred instead of re-dispatched into the same crash. `preflight-health-check.sh` Check 4
surfaces any OPEN breaker.

## 2. The six systemd timers (and what each does)

```bash
systemctl --user list-timers 'domain-check-*' --all   # every row must have a future NEXT time
./scripts/setup-repo-maintenance.sh                   # install/refresh the timers (daemon-reload + enable --now)
```

Verified live 2026-09-08 (8 timers listed; the six core ones plus two extras this box runs):

| Timer | Schedule | What it does | Output |
|---|---|---|---|
| `domain-check-service-monitor` | `*:00/2` (every 2 min) | inference gateway + resource health | `.beads/logs/service-monitor.log` |
| `domain-check-resource-monitor` | `*:00/5` (every 5 min) | memory / disk / load / PSI thresholds | `.beads/logs/resource-{alerts,metrics}.log` |
| `domain-check-monitoring` | `*:00/10` (every 10 min) | crash pattern + surge detection over the last 24 h | `.beads/logs/crash-monitor.log` |
| `domain-check-repo-health` | daily 02:00 | repo size / loose objects / gc-need report (`auto-gc-trigger.sh --dry-run`) | `.beads/logs/git-gc-check.log` |
| `domain-check-git-gc` | daily 03:00 | bounded incremental gc | `.beads/logs/git-gc.log` |
| `domain-check-git-gc-full` | Sun 04:00 (`MemoryMax=4G`) | bounded full gc | `.beads/logs/git-gc-full.log` |
| `domain-check-auto-gc` *(extra)* | daily 02:30 | auto-gc trigger pass | — |
| `domain-check-alert-triage` *(extra)* | hourly | alert triage sweep | — |

**Gotchas:** after editing any `~/.config/systemd/user/domain-check-*` unit file, run
`systemctl --user daemon-reload` or the timer silently never fires (this bit the weekly full-gc on
2026-09-02). The cron-based `scripts/monitoring-setup.sh` **does not work on this box** — do not
use it to schedule anything. Neither `check-repo-health.sh` nor `auto-gc-trigger.sh` self-appends;
the 02:00 record exists only because the timer's unit uses `StandardOutput=append:`.

## 3. Safe git gc — the only sanctioned gc path

Never run bare `git gc --aggressive` (and never `--prune=now`): that exact command memcg-OOM'd the
12 GiB dispatch scope in the bf-4x12ec/bf-173o7e storms. The bare path is *also* bounded now by
persistent git config (§4), but the scripted path adds preflight gates, checkpointing, and a
monitor.

```bash
./scripts/safe-git-gc.sh --check-only   # verdict only: exit 1 = "GC not needed" (NOT a failure); exit 0 = gc IS needed
./scripts/safe-git-gc.sh                # standard: stages 1-2, ~10-30 min
./scripts/safe-git-gc.sh --full         # all stages incl. deep compression, ~1-2 h
./scripts/safe-git-gc.sh --resume       # resume from last checkpoint after an interruption
./scripts/safe-git-gc.sh --auto-when-needed   # no-op (exit 0) unless the repo crosses a bloat threshold; safe from monitors
```

Reading the results:

- **Exit 1 from `--check-only` is the normal healthy verdict.** The check path itself ran clean —
  config validated, resource gates passed — it just concluded gc is not needed.
- Preflight gates (memory / disk / load) make the script exit **2** *before* touching anything if
  the box is unfit.
- Forensics: `.git/safe-gc.log`, `.git/safe-gc-checkpoint.json`, and the systemd scope lines in
  `journalctl --user`.
- Watch a long run from a second terminal: `./scripts/safe-git-gc-monitor.sh --watch`.

Emergency sequence if the repo ever re-bloats (symptoms: `.git` > 1 GB, exit −1 during git ops):

```bash
git count-objects -vH && du -sh .git
./scripts/safe-git-gc.sh --full
./scripts/safe-git-gc-monitor.sh --watch      # second terminal
du -sh .git && git fsck --full
```

Use `git fsck --full` as the integrity gate. `git fsck --no-full` on this packed repo (git 2.50.1)
exits 2 with ~1,008 false `invalid reflog entry` errors — never "repair" the reflog in response.

## 4. GC memory bound verification

```bash
./scripts/setup-git-gc-config.sh --verify     # exit 0 = bounded; exit 1 = unbounded or threads unpinned
./scripts/setup-git-gc-config.sh --global     # apply bounds box-wide (~/.gitconfig)
./scripts/setup-git-gc-config.sh              # apply bounds to this repo (local config)
```

`--verify` resolves the **effective** chain a bare gc actually sees (system → global → local) and
reports which scope supplies each key. Verified live 2026-09-08:

```
✅ Verified — effective (system -> global -> local); scope: windowMemory=local
   deltaCacheSize=local threads=local; worst-case pack memory ≈ 3072MiB
   (windowMemory=2147483648, threads=1, deltaCache=1073741824) — within the
   6442450944 ceiling for a 12GiB dispatch scope.
```

The bound also covers `git push`'s pack-objects (the bf-198ne variant of the same OOM). The
regression harness is `./scripts/test-gc-memory-bounds.sh` (17 assertions; verified 17/17 on
2026-09-08 — both historical death operations replay exit 0 under a 768M cgroup at peaks ≈227 MiB
push / ≈313 MiB gc).

## 5. Alert triage

```bash
./scripts/crash-classifier.sh <bead-id>       # classify one crash
./scripts/crash-alert-manager.sh <bead-id>    # classify + dedup + (maybe) create an investigation bead
./scripts/crash-alert-manager.sh --auto-process   # process recent crashes in bulk
```

`crash-alert-manager.sh` flags: `<bead-id> [--classify-only] [--force-alert]` (`--force-alert`
bypasses the cooldown/dedup gates) and `--auto-process`.

Classifications emitted: `FALSE_POSITIVE`, `SERVICE_FAILURE`, `INFRASTRUCTURE`, `UNKNOWN`.
**Known gap:** `CODE_DEFECT` is currently unreachable (0 emitting branches) — a panic-shaped trace
classifies as `UNKNOWN`. And per the capture-race rule, a `outcome: crash` trace against a silent
`events.jsonl` is still a crash: **automated UNKNOWN ≠ no crash.** Classify from the worker log /
crash bundle / regime match, and verify the target bead's actual state first — most alerts today
point at work another worker already finished.

Suppression layers (all verified 2026-09-08, driver 16/16):

| Layer | Behavior |
|---|---|
| Per-classification cooldown | 300 s after an alert of the same classification |
| Global cooldown | an independent 300 s window for alerts of *any* classification, behind the per-classification one |
| Already-processed leg | a re-run of the same alert bead is skipped (FIX 2/3) |
| Target dedup | one alert per crash **target** per 7 days (expires; not a wall) |
| Closed-target gate | an alert whose crash target is CLOSED is suppressed — nothing left to investigate (`efb1603`) |
| `--force-alert` | bypasses the cooldown gates when a genuine crash must get through |
| Store unreadable | dedup **fails open** (proceeds); ledger + cooldown layers still hold |

Verify a fix claim before re-investigating: `./scripts/test-crash-alert-fixes.sh` (13/13) and
`./scripts/test-closed-bead-filter.sh` (7/7 — run it from the repo cwd; its premise reads the live
store and is cwd-sensitive). Pre-close gate: `./scripts/verify-work-completion.sh <bead-id> --summary "..."`.

## 6. Escalation thresholds

From CLAUDE.md (authoritative source); monitors already alert at these lines:

**System resources** — escalate at Warning, abort work at Critical:

| Resource | Minimum (ok) | Warning | Critical |
|---|---|---|---|
| Available memory | 20 GB | 10 GB | 5 GB |
| Disk space | 50 GB | 30 GB | 20 GB |
| CPU load (1 min) | < 5 | < 10 | > 15 |
| Git GC memory | 1 GB | 2 GB | 4 GB |

**Repository health** (daily 02:00 check):

| Metric | Healthy | Warning | Critical |
|---|---|---|---|
| Total repo size | < 500 MB | 500 MB–1 GB | > 1 GB |
| Loose objects size | < 100 MB | 100–500 MB | > 500 MB |
| Loose object count | < 100 | 100–1000 | > 1000 |
| Loose:packed ratio | < 1:10 | 1:10–1:2 | > 1:2 (inverted) |

**Alert lines fired by the monitors:** memory pressure ≥ 70 % (advisory; the OOM threshold is
80 %), disk < 30 GB, repo size > 1 GB, loose objects > 500 MB, **crash surge = 10+ crashes in
10 minutes → INFRASTRUCTURE EVENT** (stop per-bead investigation; treat as system-wide).

**Gateway:** check with `curl -skf --max-time 5 https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health`
— the `-k` is required (self-signed cert); plain `-sf` fails curl 60 while the gateway is fine.

**Escalation actions:** Warning → finish in-flight work, do not start new dispatches, re-run §1.
Critical / surge → stop dispatches, run §3 `--check-only` + `free -h` + `df -h /` + `uptime`,
classify the surge (§5), and only resume when the box is back inside Minimum.

---

## Standing rules this runbook depends on

- `.beads/` is wholly gitignored (0 tracked files) — never re-track bead state; that is the fix
  that ended bf-4yjq.
- Never force-push; never push co-tenant commits from the shared worktree; specific-path adds only.
- Domain-check cluster manifests live in `jedarden/declarative-config`, not this repo.
- Code defects are *not* the expected finding: 157+ investigations have found zero domain-check
  code defects. Expect infrastructure (memcg-OOM in dispatch scopes) and service-class failures.
