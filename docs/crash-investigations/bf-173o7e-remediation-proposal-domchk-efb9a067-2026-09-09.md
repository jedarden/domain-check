# Remediation Proposal: bf-173o7e Aug-14 `exit −1` storm — bead `domchk-efb9a067`

| Field | Value |
|---|---|
| **Bead** | `domchk-efb9a067` — "Propose remediation for crash" (recommend leg of the bf-173o7e template chain) |
| **Upstream leg** | Root cause: `domchk-79a68be3` (Closed rev 4, 2026-09-09) — memcg OOM SIGKILL, subordinate to the closed investigation leg `domchk-4b924241` |
| **Downstream leg** | `domchk-fc2d8600` — "Implement crash remediation" (open; see §7 for its disposition) |
| **Target** | `bf-173o7e` — Closed rev 20 (2026-08-17); work never lost |
| **Date** | 2026-09-09 · HEAD `a55d67e` |
| **Disposition** | **The remediation already exists, is implemented, and is verified — recommend adoption-as-is, zero functional work.** This doc is the proposal record: approach, complexity, trade-offs, approval basis, and corrections to the superseded 2026-08-28 mitigation-strategy doc. |

---

## 1. Root cause restated (no new claim)

From the canon determination
([bf-173o7e-aug14-storm-root-cause-2026-09-02.md](bf-173o7e-aug14-storm-root-cause-2026-09-02.md),
re-affirmed by `domchk-79a68be3` on 2026-09-09): the bead-prescribed bare
`git gc --aggressive --prune=now` ran inside the dispatch scope's 12 GiB memcg
(`run-*.scope`, `memory.max=12884901888`) over a store carrying **17.20 GiB of
loose objects** (committed `.beads/*.jsonl` snapshots). Unbounded `git
pack-objects` RSS exceeded the cgroup budget and the kernel's memory-cgroup OOM
killer SIGKILLed the agent — recorded by needle as the `exit −1`
abnormal-child-death sentinel, not a signal number. Needle re-dispatched the
identical task 131 times (129 kills, 10.5 h). INFRASTRUCTURE, systemic, and now
**resolved**.

So a remediation has to close four things: the bloat source, the unbounded
memory of the operation itself, the absence of a safe maintenance path, and the
retry loop that turned one kill into a 131-attempt storm.

## 2. Remediation approach — the defense in force, re-verified live this dispatch

All seven layers below **predate this dispatch** (landed by earlier closed
beads; inventory and live evidence per the committed fix-verification record
[`docs/crash-fix-verification-bf-173o7e-domchk-5355a695-2026-09-08.md`](../crash-fix-verification-bf-173o7e-domchk-5355a695-2026-09-08.md)).
Every figure in the right-hand column was **re-run first-hand on 2026-09-09 for
this proposal** (HEAD `a55d67e`), not quoted:

| # | Causal link closed | Control | Landed in | Live evidence, 2026-09-09 (this dispatch) |
|---|---|---|---|---|
| 1 | Bloat source (bead snapshots committed to git) | `.beads/` + `*.db` + `*.jsonl` gitignored | `4e169ee` (2026-08-17) | `git ls-files .beads` → **0** tracked files |
| 2 | Bloat source (any large file) | 10 MB pre-commit size gate | hook source `2ec91ec`; installer shipped 2026-09-06 | `setup-git-hooks.sh --check` → **rc 0** (installed, byte-identical to tracked source) |
| 3 | **The kill locus** — unbounded pack memory vs the 12 GiB scope | Persistent git config `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` (repo-local **and** global, so bare gc/push are bounded too) | `2ec91ec` | `setup-git-gc-config.sh --verify` → **rc 0**, effective worst case **≈3072 MiB** within the ceiling for the 12 GiB scope |
| 4 | **Death-op replay** — the exact command that killed 129 attempts | `scripts/test-gc-memory-bounds.sh` replays it under a 768 MiB cgroup | `scripts/test-gc-memory-bounds.sh` | **17/17 passed** — bare `git gc --aggressive --prune=now` **exited 0 under MemoryMax=768M**, pack-objects **peak RSS 320,536 KB** < 700 MiB cap (the crash run exceeded 12 GiB) |
| 5 | Bounded maintenance path so no agent needs bare gc | `scripts/safe-git-gc.sh` (fail-fast preflight exit 2, staged stages, checkpoint/resume, monitor) | `2ec91ec` | `--check-only` preflight passes (**rc 1 = "GC not needed" verdict**, not a failure); `test-safe-git-gc-limits.sh` **33/33 passed** |
| 6 | Continuous enforcement without agent-run gc | systemd **user** timers: repo-health 02:00, auto-gc check 02:30, incremental gc 03:00, weekly full gc `MemoryMax=4G`, plus monitoring timers | `setup-repo-maintenance.sh` | `systemctl --user list-timers 'domain-check-*'` → **8/8 present** |
| 7 | The retry storm itself (1 kill → 131 attempts) | per-bead crash circuit breaker + `needle-with-limiter.sh` pre-dispatch gate (storm backoff defers, never re-fires) | `ad73b42` / `7f8af4d` (2026-09-07) | `crash-circuit-breaker.sh status` → **`"beads": {}`** (no open breakers); dispatch entry point documented in CLAUDE.md |

Repo state under these layers, live this dispatch: `.git` **105 MB** (39 loose
objects / 260 KiB, **one** pack / 12,607 objects / 100.70 MiB, 0 garbage),
`check-repo-health.sh` **exit 0**, 0 commits unpushed — vs the crash-era
17.20 GiB loose-object store. The bloat that made aggressive gc attractive
cannot recur through `.beads/`, and the operation that killed 129 attempts now
fits the scope with ~38× headroom.

## 3. Implementation complexity assessment

| Layer | Status | Remaining effort |
|---|---|---|
| 1–3 (gitignore, hook, pack-memory config) | **Landed, committed, verified** | None — config-level, already applied repo-local *and* global |
| 4 (death-op replay) | **Landed, committed, verified** | None — 17-assertion suite; runs in minutes |
| 5 (safe-git-gc.sh) | **Landed, committed, verified** | None — 33-assertion suite; `--check-only` for cheap preflight |
| 6 (timers) | **Landed, verified** | None — per-clone/per-user install via `setup-repo-maintenance.sh`; keep `daemon-reload` after unit edits |
| 7 (breaker + dispatch gate) | **Landed, verified** | None — behavioral: dispatches must enter via `needle-with-limiter.sh` |

**Complexity verdict: zero new functional work.** Everything is configuration,
scripts, and timers at HEAD. The only "implementation" this chain still owes is
*validation cadence*, which already exists:
[docs/crash-prevention-validation.md](../crash-prevention-validation.md)
defines the per-layer procedures and the dated verification record (last full
battery green 2026-09-09, `280708f`); this dispatch re-ran the decisive layers
again. A remediation claim is only as current as its last validation — that is
the ongoing cost, and it is a scheduled one, not a build.

## 4. Trade-offs considered

| Decision | Alternative rejected | What it costs | What it buys |
|---|---|---|---|
| **Bound the operation in persistent git config** (windowMemory/deltaCache/threads, global + local) | Ban aggressive gc outright; or rely on per-invocation cgroup wrappers only | `pack.threads=1` makes big packs slower; worst case still ≈3 GiB per pack run | Every gc/push path — including a bare invocation by a future agent — is bounded *by default*, with no wrapper to forget. Wrapper-only defenses fail exactly when an agent skips them (the bf-4x12ec / bf-173o7e shape) |
| **safe-git-gc.sh as the operating path** (staged, checkpointed, fail-fast) | One-shot `git gc --aggressive --prune=now` | Longer wall-clock; checkpoint bookkeeping; agents must be told (CLAUDE.md mandate) | Resumable after interruption, monitorable, preflight refuses insufficient memory/disk/load *before* touching git, and each stage is bounded |
| **Keep `--aggressive` available but bounded** (canon position) | The superseded doc's "no evidence it's unsafe" or a hard prohibition with no substitute | Aggressive compression is rarely needed once the bloat source is closed | The death-op replay (layer 4) proves the historical kill command is now survivable at 1/16th of the scope — defense does not depend on agents obeying the ban |
| **Storm breaker + defer** (layer 7) | Let needle retry indefinitely | A deferred bead waits (latency) instead of killing 131 dispatches | Converts a deterministic per-attempt failure into backoff; the fix-then-retry lesson (decompose-or-bound, never re-dispatch the same death) |
| **`.beads/` untracked, checkpoint-flush discipline** | Track bead state in git (the crash-era practice) | Bead state is not in git history; durability rides on the explicit `bead sync flush-only` checkpoint | Structurally closes the 17+ × 237 MB snapshot mechanism that caused the era's bloat — the one fix that makes the others rarely needed |
| **Timer-driven maintenance** (layer 6) | Agent-initiated gc inside dispatch scopes | A scheduled daily run; unit edits need `daemon-reload` | No agent ever needs to run gc inside a 12 GiB dispatch scope again |

The unifying trade-off: **structural/automatic bounds over behavioral
instructions.** Each layer assumes an agent *will* someday run the wrong
command, and makes that survivable — the bf-48wvu leg's P1 recommendation
("keep `.beads/` untracked + never bare gc") is enforced by gitignore + config
rather than by convention.

## 5. Recommended approach and its approval basis

**Recommendation: adopt the seven-layer defense as-is; no functional changes;
keep the validation cadence.** Concretely:

1. Never run bare `git gc --aggressive` as an agent task — use
   `scripts/safe-git-gc.sh` (canon mandate, CLAUDE.md "Git Operations Safety").
2. Keep `.beads/` untracked and the 10 MB pre-commit hook installed per clone.
3. Keep the persistent pack-memory config applied repo-local **and** global.
4. Enter dispatches through `./scripts/needle-with-limiter.sh` so the breaker
   can defer a storming bead.
5. Re-run the prevention battery before citing any "prevention in force"
   claim (per [docs/crash-prevention-validation.md](../crash-prevention-validation.md)).

**Approval basis:** this is not a new decision requiring sign-off — it is the
already-adopted, operator-mandated standing procedure
([docs/maintenance/repository-maintenance-guide.md](../maintenance/repository-maintenance-guide.md)),
implemented by closed beads, proven by the death-op replay, and re-verified
live by this dispatch (§2). What this leg *does* decide is the open question
left by the only competing committed proposal: `docs/git-gc-mitigation-strategy.md`
sits at "Status: PROPOSED" with "Review and approve this mitigation strategy"
as its next step. That approval question is **closed by this document** — with
the corrections in §6, because that doc's premise is superseded.

## 6. Corrections: the 2026-08-28 mitigation-strategy doc is superseded on root cause

[`docs/git-gc-mitigation-strategy.md`](../git-gc-mitigation-strategy.md)
(2026-08-28, bead `domchk-e49087e2`) analyzed this same target bead. Its
root-cause premise predates the recovered kernel evidence and is superseded by
the canon RCA (whose §5 already marks the two events as distinct):

| Its claim | Correction (canon basis) |
|---|---|
| "The most recent crash (bf-173o7e) was NOT a git gc failure — the operation completed successfully … max_turns limit exhaustion" | That describes the **Aug-17** `exit 1` / `max_turns` event — a *different event* from this chain's assigned crash (canon §5, last row). The Aug-14 storm was **129 × exit −1**: kernel memcg OOM SIGKILLs of pack-objects mid-gc, 21.6–216.6 s lifetimes, far below the 600 s cap one surviving run reached |
| "Memory usage: 864MB-1.3GB (well within 52GB available)" / "Why Git GC --aggressive Is Generally Safe Here" | Superseded for the Aug-14 mechanism: kills happened at the **12 GiB scope** bound, not the host's; the era's 17.20 GiB loose-object store made pack-objects RSS exceed the cgroup. CLAUDE.md's corrected-evidence note records the same supersession for the sibling early docs |
| "Continue using git gc --aggressive where appropriate" / "**Do NOT avoid git gc --aggressive outright**" | Superseded operating rule: bare aggressive gc is **the death command** for this mechanism class (it killed bf-173o7e ×129, bf-4x12ec, bf-1s6c3, bf-4yjq, bf-65lsdu). Use `safe-git-gc.sh`. Nuance this proposal adds: with layer 3's persistent bounds now in force, even a bare invocation is survivable (§2 layer 4) — the defense no longer depends on the ban being obeyed, but the safe path remains the rule |
| Its Option 2 checklist (cgroup limits, pre-flight resource checks, incremental default, progress monitoring, scheduled maintenance) | Implemented — by `safe-git-gc.sh`, `test-gc-memory-bounds.sh`, the timer fleet, and the persistent config, not by the doc's sketches. Its Option 1 (max_turns/process improvements) belongs to the Aug-17 event family and the close-gate/workflow layer (`verify-work-completion.sh`), outside this chain's mechanism |
| "Status: PROPOSED … Next Steps: Review and approve this mitigation strategy" | Answered by this document: the approved remediation for the exit −1 storm is §2's implemented defense; the 2026-08-28 doc's own risk table never contemplated memcg OOM because the kernel evidence postdates it |

The 2026-08-28 doc is **not** deleted or edited here — it remains the record of
the Aug-17 analysis and of the option space; this table is the supersession
marker. (It is also not archived, so a future reader landing on it from
`docs/plan/plan.md` should find this correction via the Related section.)

## 7. Downstream guidance — `domchk-fc2d8600` ("Implement crash remediation")

The implement-fix child should take the **verification-only disposition**
established for this same target by `domchk-5355a695` (2026-09-08, committed
record): *the fix already exists — implemented and committed by earlier beads;
re-verify live; ship no functional code.* Concretely: run §8's battery, confirm
the seven layers, append the dated row to
[docs/crash-prevention-validation.md](../crash-prevention-validation.md) per
its update protocol, and close zero-functional-commit. Re-implementing any §2
layer would duplicate closed work; writing *new* bounds would second-guess a
proven defense.

## 8. How to re-verify

```bash
./scripts/check-repo-health.sh                     # bloat + bound + backlog (exit 0)
./scripts/setup-git-gc-config.sh --verify          # effective pack-memory bound (rc 0)
./scripts/safe-git-gc.sh --check-only              # preflight (rc 1 = "GC not needed" verdict)
./scripts/test-gc-memory-bounds.sh                 # death-op replay, 17 assertions
./scripts/test-safe-git-gc-limits.sh               # safe-gc suite, 33 assertions
systemctl --user list-timers 'domain-check-*' --all  # 8 timers present
git ls-files .beads | wc -l                        # must stay 0
./scripts/setup-git-hooks.sh --check               # rc 0 = installed and current
./scripts/crash-circuit-breaker.sh status          # "beads": {} expected
```

## Related

- Canon RCA: [`docs/crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md`](bf-173o7e-aug14-storm-root-cause-2026-09-02.md) (determination bead `domchk-17ca8b7d`; re-affirmed by `domchk-79a68be3`)
- Fix-verification record (same target, 2026-09-08): [`docs/crash-fix-verification-bf-173o7e-domchk-5355a695-2026-09-08.md`](../crash-fix-verification-bf-173o7e-domchk-5355a695-2026-09-08.md)
- Superseded proposal: [`docs/git-gc-mitigation-strategy.md`](../git-gc-mitigation-strategy.md) (see §6)
- Prevention inventory + gaps: [`docs/crash-prevention-requirements.md`](../crash-prevention-requirements.md) (G-1..G-13)
- Per-layer validation procedures + dated record: [`docs/crash-prevention-validation.md`](../crash-prevention-validation.md)
- Maintenance procedures: [`docs/maintenance/repository-maintenance-guide.md`](../maintenance/repository-maintenance-guide.md)
- Cleanup record: [`docs/crashes/bf-173o7e-cleanup-verification.md`](../crashes/bf-173o7e-cleanup-verification.md)
- Sibling-family precedent for this disposition: `docs/crash-investigations/bf-48wvu-fix-recommendations-domchk-c2227cb9-2026-09-09.md`
