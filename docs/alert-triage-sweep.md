# Alert Triage Sweep

The hourly, **report-only** pass that walks every open crash alert in the bead
store, resolves each alert's crash target through the single resolution
authority, and writes a triage queue that closure-bead blockers, agents, and
humans drain. Implements §5 P1(a)/P4 of
[alert-deduplication-gap-analysis-2026-09-07.md](alert-deduplication-gap-analysis-2026-09-07.md).

Part of the bf-mje3pd prevention stack — research bead `domchk-f7865662`
("Research preventive measures for bf-mje3pd crash type", Closed 2026-09-07),
whose action 2 was "land the alert-triage layer in git … so the running
prevention state has a tracked source". (The brief's own file,
`docs/crashes/bf-mje3pd-prevention-research-domchk-f7865662-2026-09-07.md`,
was untracked from git in the 2026-09-07 evening cleanup sweep — commit
`e4fcbec` deleted ~30 bf-mje3pd/bf-2ildm/bf-4x12ec investigation docs from the
index; the text survives in the bead record and as untracked on-disk copies.)
The layer ran on this box from 2026-09-07 onward before it was tracked; this
commit is that tracked source, not a behavior change.

## Components

| File | Role |
|---|---|
| `scripts/alert-triage-sweep.sh` | The sweep itself |
| `scripts/setup-alert-triage-timer.sh` | Install / `--status` / `--remove` the systemd user timer |
| `scripts/domain-check-alert-triage.service` | Oneshot unit (report-only, `MemoryMax=512M`, pinned PATH) |
| `scripts/domain-check-alert-triage.timer` | Hourly trigger (`OnBootSec=10min`, `Persistent=true`) |
| `scripts/test-alert-triage-sweep.sh` | Hermetic suite (20 assertions; fake `bead` on PATH, real store untouched) |

## What it does

For every **open / in_progress** alert bead (title starts `ALERT:`, or carries
an `alert` label), the sweep derives the crash target with the gate's own
predicate (`alert-deduplication.sh`'s scan), so the two tools cannot disagree
about what an alert is or which crash it points at. Each **distinct** target is
resolved once through `crash-resolution-tracker.sh check` and the alert gets a
verdict:

| Verdict | Meaning |
|---|---|
| `RESOLVED_TARGET` | target is resolved (closure / VERIFIED marker / ledger) — close candidate for its closure-bead blocker |
| `ORPHANED_TARGET` | target no longer exists in the store |
| `FANOUT_KEEPER` | target unresolved; oldest open alert for it — the one that should carry the investigation |
| `FANOUT_DUPLICATE` | target unresolved; an older open alert already carries it |
| `NEEDS_REVIEW` | target unresolved and this is its only open alert — the only verdict that deserves fresh investigation effort |

**It never closes, updates, or creates a bead.** Closure stays with each
alert's closure-bead blocker (the standing convention). Every `bead` invocation
the sweep makes is read-only, and the test suite asserts that contract.

Outputs:

- Queue (atomic replace, one JSON record per alert):
  `.beads/state/alert-triage/queue.jsonl`
- Log (one summary line per run): `.beads/logs/alert-triage.log`
- Exit codes: `0` swept, `2` usage, `3` store unreadable — **fail open**:
  a broken store writes nothing rather than manufacturing close candidates.

## Usage

```bash
./scripts/alert-triage-sweep.sh            # summary to stdout
./scripts/alert-triage-sweep.sh --json     # queue records on stdout
./scripts/setup-alert-triage-timer.sh      # install/refresh + enable --now
./scripts/setup-alert-triage-timer.sh --status
./scripts/setup-alert-triage-timer.sh --remove
bash scripts/test-alert-triage-sweep.sh    # hermetic suite, safe anywhere
```

`BEAD_SCAN_LIMIT` overrides the `bead list --limit` scan bound (default
`999999` — the store must be scanned whole; a partial scan would under-report
fan-out groups).

## Relationship to the rest of the alert layer

- `crash-alert-manager.sh` decides whether a **new** alert bead is created.
- `alert-deduplication.sh check <bead>` answers one alert at a time, on demand.
- The sweep is the automated fleet-wide pass over the whole open-alert pool —
  the piece the gap analysis found missing (D-9: signals existed with no
  consumer).
- Its queue is what a **closure bead** drains: the bf-mje3pd chain's five stale
  alerts (bf-1cezsk, bf-56kmlk, bf-1pidqn, bf-3dxljn, bf-x88dnf) all read
  `RESOLVED_TARGET` — their target closed 2026-08-17 and every prior
  verification converged on *crashes genuine, work complete, alerts stale*.
  The queue holds the same verdict for ~220 resolved-target alerts fleet-wide.

## Verification record (2026-09-07, domchk-97a4d354)

Re-executed in full at landing time (HEAD `9b32085`) — every figure below is
this attempt's own run, not the authoring attempt's:

- `bash scripts/test-alert-triage-sweep.sh` → **20/20, exit 0** (re-run).
- `systemd-analyze verify` on both unit files → clean (exit 0).
- Installed units in `~/.config/systemd/user/` byte-identical to the repo
  copies (`cmp` clean both), timer `enabled`, last fired 18:00:57 EDT, next
  trigger 19:00:57 EDT — a live future trigger at commit time.
- `bash -n` clean on all three scripts.
- Live sweep 2026-09-07T22:56:40Z (~1.6s): alerts=233 across 29 targets,
  RESOLVED_TARGET=215, ORPHANED=0, FANOUT_KEEPER=2, FANOUT_DUPLICATE=14,
  NEEDS_REVIEW=2, new_close_candidates=0; queue rewritten atomically
  (233 records).
- No regressions: `test-crash-alert-fixes.sh` and `test-closed-bead-filter.sh`
  both exit 0 at the same HEAD. The landing adds six files and changes no
  existing path's behavior.

## Fresh-clone reproducibility — what is and is not reproducible yet

A fresh clone plus `scripts/setup-alert-triage-timer.sh` now reproduces the
running triage layer; previously the layer existed only on this worktree's
disk. One path caveat: both unit files pin this clone's absolute path
(`WorkingDirectory=/home/coding/domain-check`, the `ExecStart` and the
`append:` log paths), so a clone must live at that path — true on this box —
or the installer needs path templating first.

Still disk-only (deliberately **not** landed here):

- The two `domain-check-auto-gc.{service,timer}` units → the threshold-
  triggered auto-gc layer, owned by the safer-gc bead (domchk-9029e178, open).
  The original reason to hold them — a dangling `ExecStart` on an untracked
  `--auto-when-needed` flag — is gone as of `e4fcbec`, which landed the flag
  in tracked `safe-git-gc.sh` together with the `crash-classifier.sh`
  capture-race verdict, the `cleanup-bloat.sh` safe rewrite, the
  `repo-health-monitor.sh` pack-mass fix, and the `setup-repo-maintenance.sh`
  wiring for both new timers. That leaves one real fresh-clone gap this
  landing does not close: `setup-repo-maintenance.sh` (tracked) now wires
  `domain-check-auto-gc.timer`, but the two unit files it names are still not
  in git. Landing them is the owning bead's call, not this one's.

(`e4fcbec` was a commit of the shared worktree's dirty *tracked* files, so it
swept up several beads' uncommitted edits at once and — untracked files being
invisible to `git commit` — none of the new files. That is why the six files
above and the two auto-gc units survived on disk only.)
