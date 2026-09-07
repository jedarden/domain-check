# Stepwise Git GC Strategy — Research and Design

**Bead:** domchk-adb15fe5
**Date:** 2026-09-05
**Status:** Design. Informs the execution/monitoring implementation (domchk-9029e178); no
script changes are made by this document.
**Verified against:** git 2.50.1 as installed on this box. Every argv, flag semantic and
default below was either **measured** with `GIT_TRACE=1` / a scratch repo, or **cited from
this installation's man pages** — see [Appendix A](#appendix-a-verification-performed-for-this-document).
**Supersedes:** the parameter choices in [`../safer-git-gc-strategy.md`](../safer-git-gc-strategy.md)
(2026-09-01), whose memory figures are asserted rather than measured and whose stage
commands do not do what they claim (§7).
**Re-verified:** 2026-09-06 by the retry dispatch after the first (01:40) worker died
before committing. Every load-bearing claim re-checked independently against the current
tree: the three repack argvs re-traced on git 2.50.1 with `GIT_TRACE` (§2.1, identical),
the effective config re-read (§2.3, identical), repo state re-measured (§3 — now 1 pack,
90.43 MiB, 10,712 in-pack, 11 loose), and the §7.1 gap analysis re-read against
`scripts/safe-git-gc.sh` line-by-line (all eight gaps still present).

---

## 0. Summary

The August crashes were never "git gc is dangerous." They were three unbounded multipliers
landing in a 12 GiB cgroup:

| Multiplier | Value on 2026-08-14 | Bounded value today |
|---|---|---|
| Delta-search window memory | **unbounded** (`pack.windowMemory` unset = no limit) | 2 g per thread |
| Threads | **12** (`pack.threads` default = nproc) | 1 |
| Window size | **250** (what `--aggressive` sets) | 5 (`pack.window`, used by plain gc) |

Bounded, the identical crash command peaks at **≈313 MiB** and completes in minutes instead
of being SIGKILLed 129 times in a row.

`git gc --aggressive` is not a different algorithm. Measured on this box, it is plain
`git gc` plus three repack flags:

```text
git gc                → git repack -d -l -q --cruft --cruft-expiration=2.weeks.ago
git gc --aggressive   → git repack -d -l -f --depth=50 --window=250 -q --cruft --cruft-expiration=2.weeks.ago
```

The *entire* difference is `-f --depth=50 --window=250`: a 25–50× wider delta-search window
(the depth default is 50 either way — `--aggressive` does not deepen it) plus forced
recomputation of every existing delta. Those two flags are exactly where the memory and the
hours go, and neither has ever been shown to help this repository: every measured size win
here came from packing loose objects at all (17.20 GiB → 90.34 MiB), which plain `git gc`
does.

**The stepwise strategy is therefore not "aggressive in smaller pieces." It is: plain
`git gc` by default; widen the window in a bounded full repack only when size or clone
performance justifies it; force delta recomputation almost never.**

---

## 1. Failure history this design answers

| Date | Bead | What actually happened |
|---|---|---|
| 2026-08-14 | bf-173o7e | 129 of 132 dispatch attempts SIGKILLed by kernel memcg OOM while running bare `git gc --aggressive --prune=now` over 17.20 GiB of loose objects inside needle's 12 GiB `run-p*.scope`; pack-objects RSS exceeded the scope ceiling each time. Zero-backoff re-claim turned one deterministic kill into a 10.5 h storm. |
| 2026-08-14 | bf-4x12ec | Same mechanism, earlier in the day; the 53rd "attempt" was an auto-split, not a completed gc. Also surfaced a config-type trap: `gc.aggressiveWindow 1.hour` made `git gc --aggressive` exit 128 (`bad numeric config value`) until unset. |
| 2026-08-12 | bf-1s6c3 | 18 GB repo, 17 GB loose objects → OOM during git reconciliation. Fixed by packing at all: 18 GB → 138 MB. Plain packing did the work; no aggressive flag involved. |
| 2026-08-13 | bf-65lsdu | OOM amplified by *several* gc runs executing concurrently, each >4 GB. Box-wide lock now serializes them. |
| 2026-09-02 | (post-fix) | `safe-git-gc.sh` under the bounds completed in ~6 min; repo left as 1 pack, 90.19 MiB. |

The pattern across every entry: the damage came from *unbounded* pack-objects, and the fix
that worked was *bounding* it — not retrying, and not a different gc algorithm.

---

## 2. Research: what git actually does, and where the memory goes

### 2.1 The gc pipeline (argv measured on git 2.50.1)

`git gc` is an orchestrator. Measured on git 2.50.1 it runs, in order:

```text
git pack-refs --all --prune
git reflog expire --all
git repack -d -l -q --cruft --cruft-expiration=2.weeks.ago      ← the expensive step
    └─ two git pack-objects runs: one for the main pack (--all --reflog
       --indexed-objects --keep-true-parents), one for the cruft pack (--cruft)
git prune --expire 2.weeks.ago --no-progress
git worktree prune --expire 3.months.ago
git rerere gc
```

plus a `git commit-graph write` when `gc.writeCommitGraph` (default true) has work to do.
The repack argv it builds is the whole story:

| Invocation | Effective repack argv (measured) |
|---|---|
| `git gc` | `git repack -d -l -q --cruft --cruft-expiration=2.weeks.ago` |
| `git gc --aggressive` | `git repack -d -l -f --depth=50 --window=250 -q --cruft --cruft-expiration=2.weeks.ago` |
| `git gc --aggressive --prune=now` | `git repack -d -l -f --depth=50 --window=250 -q -a` (+ `git prune --expire=now`) |
| `git gc --auto` (nothing to do) | no repack at all |

Three consequences that drive the whole design:

1. **`--aggressive` only widens the window and forces delta recompute.** `--depth` is 50
   either way (`gc.aggressiveDepth` defaults to 50, the same as the non-aggressive
   default), so the "aggressive" work is `-f` + window 10 → 250.
2. **`--aggressive` overrides this repo's conservative config.** `git repack` receives
   `--window`/`--depth` *on the command line*, and a CLI flag beats `pack.window`/`pack.depth`
   config. This repo sets `pack.window 5` and `pack.depth 20` — those protect plain gc only.
   The aggressive path ran at window 250 regardless. There is no config key that can
   restrain the aggressive path; it has to be not-invoked, or cgroup-bounded.
3. **`--prune=now` changes the repack shape, not just the prune date.** With a grace period
   gc repacks with `--cruft` (unreachable objects preserved in a cruft pack); with
   `--prune=now` it repacks with `-a`, which **deletes** unreachable pack contents. Measured:
   after `repack -a -d` an unreachable object count of 3 became 0; after
   `repack -d --cruft --cruft-expiration=2.weeks.ago` the same 3 survived
   (`.mtimes` present). The grace period is not an efficiency knob — it is git's guard
   against deleting objects a concurrent writer created while gc was enumerating, which in a
   fleet-worked repo with 10+ agents is the normal condition, not the exception
   (git-gc(1) "NOTES").

### 2.2 Memory model of the expensive step (`git pack-objects` delta search)

All gc memory risk lives in one subprocess. Its RSS is approximately:

```text
RSS(pack-objects) ≈ threads × windowMemory        ← the delta-search window
                  + pack.deltaCacheSize           ← write-out delta cache
                  + fixed(object count)           ← object tables + name hash
                  + mapped object data            ← blobs read during comparison
```

Verified facts for each term (git-pack-objects(1), git-config(1) on this installation):

- **`windowMemory` is per thread.** Both man pages state it explicitly: "the actual memory
  usage will be the limit multiplied by the number of threads," and `pack.threads` "specifies
  0 … auto-detect the number of CPUs." So an unset `pack.windowMemory` on a 12-core box is
  not "a limit we didn't set" — it is 12 concurrent unlimited searches. This single
  sentence is the root cause of both Aug-14 storms, and `pack.threads=1` is therefore the
  one bound that may never be dropped.
- **`windowMemory` is soft.** It scales the window down *dynamically*; it is a heuristic the
  delta search consults, not an allocator cap. Large blobs are still read whole. The only
  hard bound on the process is the cgroup ceiling.
- **`core.bigFileThreshold`** (default 512 MiB) objects are stored without any delta
  attempt, so big files do not enter the window search at all.
- **Fixed overhead scales with object count.** This repo has 10,653 in-pack objects, so the
  fixed term is single-digit MiB — negligible here. A monorepo with millions of objects
  would see hundreds of MiB. Any memory budget table should carry the object count that
  produced it.
- **The delta cache** (`pack.deltaCacheSize`, default 256 MiB) holds finished deltas before
  write-out; `pack.deltaCacheLimit` (default 1000 bytes) caps a *single* cached delta. This
  cache is a flat, predictable term — bounded by config, unaffected by window size.

### 2.3 Knob reference

| Knob | Default | This repo (effective) | Effect |
|---|---|---|---|
| `pack.window` | 10 | **5** | candidates compared per object; main size/effort dial |
| `pack.depth` | 50 (max 4095) | **20** | max delta chain length; deep chains cost unpack-side CPU, not pack-side RAM |
| `pack.windowMemory` | **unlimited** | 2 g | per-thread window cap (soft) |
| `pack.deltaCacheSize` | 256 MiB | 1 g | write-out delta cache (flat) |
| `pack.deltaCacheLimit` | 1000 | (unset) | per-delta cache cap in bytes |
| `pack.threads` | **nproc (12 here)** | 1 | multiplies the window term |
| `pack.packSizeLimit` / `--max-pack-size` | unlimited | unset | **splits the pack**; *not* a memory control (§7) |
| `core.bigFileThreshold` | 512 MiB | (unset) | no delta attempt above this |
| `repack.writeBitmaps` | false | **true** | bitmap index on full repacks; incompatible with incremental repacks |
| `gc.aggressiveWindow` | 250 | (unset) | window used **only** by `gc --aggressive`, passed on the CLI |
| `gc.aggressiveDepth` | 50 | 50 (no-op) | depth used only by `gc --aggressive` |
| `gc.cruftPacks` | true | (unset) | unreachable objects → cruft pack, not loose |
| `gc.pruneExpire` | 2.weeks.ago | 2.weeks.ago | unreachable-object grace period |
| `gc.auto` | 6700 | 100 (local) | loose-object count that triggers background auto-gc |
| `gc.autoPackLimit` | 50 | 50 (local; global 10) | pack count that triggers auto-gc consolidation |
| `gc.bigPackThreshold` | 0 | (unset) | keep packs above this out of the repack; gc's own "don't repack the big pack" escape hatch — it also self-limits when its *memory estimate* says repacking won't fit |

Two things worth flagging from this table:

- `gc.aggressiveWindow` is an **integer**, and git does not validate it until
  `git gc --aggressive` runs — `1.hour` (set by an earlier fix attempt here) produced
  `exit 128: bad numeric config value` on every aggressive invocation until it was unset.
  Any implementation that touches this key must write an integer.
- `gc.auto=100` (local) makes git start background auto-gc at just 100 loose objects. In a
  repo worked by 10+ agents this fires constantly, and auto-gc does **not** go through
  `safe-git-gc.sh`'s box-wide lock — only git's own per-repo `gc.pid` lock applies. It is
  bounded by the pack config (so it cannot OOM the box the way Aug-14 did), but two gcs in
  two different repos at once can still stack 2 × soft-limit. See §5's concurrency rule.

### 2.4 What `--aggressive` actually buys

Nothing measurable here. The wins recorded in this repo all came from *packing loose
objects at all*:

| Run | Before | After | Flag used |
|---|---|---|---|
| bf-1s6c3 cleanup | 18 GB (17 GB loose) | 138 MB | packing only |
| bf-173o7e eventual completion | 17.20 GiB loose | 90.34 MiB, 1 pack | packing only |
| 2026-09-02 07:55 scheduled run | bloated | 90.19 MiB, 1 pack | `safe-git-gc.sh` (plain gc) |

A wider window improves delta *selection* for churny histories; on a 10.6k-object,
single-pack, mostly-docs repository the realistic upside is a few percent of 90 MiB, against
a measured cost of hours and >12 GiB. Compression comes from the window; if you want more of
it, widen the window *with a memory cap* — don't reach for `--aggressive`, which widens the
window *and* discards all existing deltas *and* rewrites the whole pack.

---

## 3. Measured evidence from this repository

| Measurement | Value | Source |
|---|---|---|
| pack-objects peak RSS, aggressive argv, pre-bound | >12 GiB (exceeded the scope ceiling) | kernel memcg kill records, bf-173o7e/bf-4x12ec |
| pack-objects peak RSS, aggressive argv, post-bound | **≈313 MiB** (exit 0 in a 768 MiB cgroup) | `scripts/test-gc-memory-bounds.sh`, 8×64 MiB blobs |
| Current repo state | 1 pack, 90.34 MiB, 10,653 objects, 20 loose (172 KiB) | `git count-objects -vH`, 2026-09-05 |
| Box | 12 CPUs, 62 GiB RAM, 42 GiB available | `nproc`, `free -g` |
| Repo is already converged | `gc` has nothing to do right now | — |

The last row matters for expectation-setting: the strategy below is *preventive*. Its value
is that the next 17 GiB of loose objects gets packed by a bounded plain gc in minutes
instead of by an unbounded aggressive gc that dies 129 times.

---

## 4. The stepwise design

Four phases. Phase 1 is the default and the terminal state for the overwhelming majority of
runs; phases 2 and 3 are gated escalations, not scheduled steps.

### Phase 0 — Gate (every run, before any object is touched)

```bash
./scripts/setup-git-gc-config.sh --verify     # exit 1 = no effective bound → refuse to run
flock /tmp/domain-check-safe-git-gc.lock …    # one gc box-wide (already in safe-git-gc.sh)
systemd-run --user --scope -p MemoryMax=<ceiling> git …   # hard ceiling, §5
git fsck --no-progress                        # never repack a corrupt repo
git count-objects -vH; du -sh .git            # record baseline metrics
```

Additional gate to add: **free disk ≥ 2 × size-pack.** A repack writes the new pack before
removing the old one, so the transient disk cost is roughly one extra pack. (The current
script's `< 5 GB` check is fine for this repo and wrong in general — a 5 GB pack needs ~10 GB
free regardless of the disk's total.)

### Phase 1 — Standard gc (the workhorse; daily 03:00 timer)

```bash
git gc          # no flags. No --aggressive, and no --prune=now.
```

| Property | Value |
|---|---|
| Effective repack | `git repack -d -l -q --cruft --cruft-expiration=2.weeks.ago` |
| Window / depth | `pack.window` (5) / `pack.depth` (20) — config, not overridden |
| Existing deltas | **reused** (no `-f`) |
| Unreachable objects | preserved in the cruft pack, 2-week grace |
| Soft memory worst case | ≈ windowMemory (2 g) + cache (1 g) + overhead |
| Expected peak RSS (this repo) | ≪ 1 GiB |
| Time | seconds–minutes |

**Completion gate — this is the phase that ends most runs:**

```bash
git count-objects -v
# loose count < 100 and pack files ≤ 2 (main + cruft) → stop. Do not escalate.
```

Why the defaults matter here:

- **No `--prune=now`.** It flips the repack to `-a` and deletes unreachable objects outright
  (§2.1.3), discarding both the recovery window and the concurrent-writer guard. The cost of
  keeping the grace period is a cruft pack holding at most ~2 weeks of garbage — on a
  90 MiB repo, nothing.
- **Plain gc already consolidates.** `--cruft` implies `-a` semantics for reachable objects,
  so a single `git gc` produces one main pack + one cruft pack — the observed steady state.
  Escalation is *never* needed merely to consolidate packs.

### Phase 2 — Bounded wide-window full repack (gated escalation)

```bash
git repack -d --cruft --cruft-expiration=2.weeks.ago \
     --window=50 --depth=50 --window-memory=1g --threads=1
```

| Property | Value |
|---|---|
| Window / depth | 50 / 50, **passed on the CLI** so the run is self-describing and independent of which config scope (system → global → local) the repo inherits |
| Window cap | `--window-memory=1g`, single-threaded → window term ≈ 1 g; total ≈ 1 g + 1 g cache + overhead ≈ **2.2 GiB** worst case |
| Existing deltas | reused |
| Unreachable objects | preserved (cruft). Measured, unlike `-a -d` which deleted them (§2.1.3) |
| Bitmap | inherited from `repack.writeBitmaps=true`; `--cruft -d` + bitmap verified rc=0 on 2.50.1 |

Run only when a gate fires:

```bash
packs > 2                                        # fragmentation phase 1 didn't fix
|| size-pack grew > 20% since the last phase 2   # churn worth re-deltaing
|| loose objects before phase 1 > 10_000         # a real bloat event
```

Otherwise skip: phase 1 already produced a single consolidated pack, and re-deltaing a
90 MiB pack buys kilobytes. Note the honest description of what this phase is: **a
compression pass, not a consolidation pass** — consolidation already happened in phase 1.

Explicitly rejected for this phase: `--max-pack-size`. It does not bound memory — it bounds
*pack file size*, splitting the output into multiple packs, which disables the bitmap index
and which git's own docs warn "may result in a larger and slower repository."

### Phase 3 — Forced delta recomputation (rare; quarterly at most, or after a large import)

```bash
git repack -d -f --cruft --cruft-expiration=2.weeks.ago \
     --window=50 --depth=50 --window-memory=1g --threads=1
```

`-f` (`--no-reuse-delta`) discards every existing delta and recomputes from scratch — the
only step that can beat phase 2's result, and the single most expensive thing in this
document. It is the flag that turned the Aug-14 repo into a 10.5-hour storm.

Rules if this phase is ever scheduled:

- **Keep the window wide (50).** Compression comes from the window; shrinking it to 10 to
  "save memory" defeats the purpose and leaves a *worse* pack than phase 2. Cap memory with
  `--window-memory`, not by narrowing the search.
- **Always inside the cgroup ceiling**, always holding the box-wide lock, with `git fsck
  --no-progress` and a checkpoint before *and* after.
- **Gate on a real payoff:** expected gain here is ~0–5% of 90 MiB. Justify it, or don't run
  it.
- Also rejected: `--no-reuse-object` (`-F`) — strictly more work than `-f`, no additional
  benefit.

### Phase 4 (optional, documented for completeness) — MIDX incremental repack

```bash
git multi-pack-index write --bitmap
git multi-pack-index repack --batch-size=1g
```

The modern answer to "I want incremental repacking with bounded work per run" — it repacks
only as much as fits the batch size. It is listed because it is the right tool for repos
that cannot tolerate full repacks, **not** because this repo needs it: `--batch-size`
semantics only pay off with many packs, and this repo converges to one. Both subcommands
verified rc=0 on 2.50.1.

### Abort and resume semantics

A repack is atomic at the pack level: new pack and index are written under temporary names
and swapped in, so an interrupted repack leaves the previous pack intact and the repository
valid — at worst with some duplicated objects. There is therefore **no "continue" — resume
means re-running the last uncompleted phase.** The checkpoint's job is to record which phase
to re-run, which is exactly what `.git/safe-gc-checkpoint.json` already does.

### Decision table

| After phase 1 | Do |
|---|---|
| loose < 100, packs ≤ 2 | **Stop.** (>95% of runs) |
| packs > 2, or size-pack +20% since last phase 2 | Phase 2 |
| large import / clone-serving repo / quarterly cadence reached | Phase 2, then Phase 3 |
| loose still high after phase 1, or fsck non-clean, or RSS touched the ceiling | Do **not** escalate — investigate. Escalation is not the fix for a gc that misbehaved. |

---

## 5. Memory limits and safe parameters

Two consistent profiles. Pick one per run; the fatal combination is mixing them (§7, gap 1).

| Profile | `pack.windowMemory` | `pack.deltaCacheSize` | `pack.threads` | Soft worst case | Min cgroup `MemoryMax` |
|---|---|---|---|---|---|
| **bounded** (box-wide today) | 2 g | 1 g | 1 | ≈3.2 GiB | **4 G** |
| **tight** (≤2 G ceiling, or ≥2 repos may gc concurrently) | 512 m | 256 m | 1 | ≈1 GiB | **2 G** |

`scripts/setup-git-gc-config.sh --verify` already asserts the soft worst case against a
6 GiB ceiling (its headroom allowance inside a 12 GiB dispatch scope) and prints the
computation — so the 4 G floor above is the *minimum* for that profile, and 6 G keeps a
run consistent with what `--verify` assumes.

Three rules, all derived from §2.2:

1. **`pack.threads=1` is non-negotiable.** Every other bound is per thread; drop this one and
   the window term is multiplied by nproc again.
2. **The cgroup ceiling must exceed the soft worst case.** A ceiling *below* the soft sum
   converts the safety mechanism into the killer: the run is SIGKILLed by its own protective
   cgroup once it legitimately fills its allowance.
3. **Concurrency multiplies everything.** Budget is per *box*, not per repo:
   `total ≈ concurrent_runs × soft worst case`. The box-wide lock caps concurrency at 1 for
   scripted gc; background `git gc --auto` bypasses that lock (§2.3), so either accept
   profile-tight values or accept the 2× exposure.

Explicitly listed as *unsafe* parameters, so an implementation doesn't rediscover them:
no `pack.threads` (defaults to 12), no `pack.windowMemory` (defaults to unlimited),
`--aggressive` without a cgroup ceiling, `--prune=now` in a concurrently-written repo,
`gc.aggressiveWindow` set to anything non-integer.

---

## 6. Why this is safer than `git gc --aggressive`

| Mechanism | `--aggressive` | Stepwise | Evidence |
|---|---|---|---|
| Window memory | unbounded, × 12 threads | `--window-memory` + `threads=1` | >12 GiB → ≈313 MiB measured |
| Existing deltas | discarded (`-f`) every time | reused except in phase 3 | the `-f` storm, 129 kills |
| Work done | always maximal, window 250, full rewrite | minimal by default, gated escalation | 18 GB → 138 MB was plain packing |
| Terminal state | after hours, or never | after seconds–minutes in phase 1 | 2026-09-02 07:55 run |
| Unreachable objects | `--prune=now` deletes them now | cruft pack + 2-week grace | measured 3 → 0 vs 3 → 3 |
| Concurrent writers | grace period dropped | grace period kept | git-gc(1) NOTES |
| Config override | CLI 250/50 beats `pack.window` | CLI values chosen *by us*, per run | §2.1.2 |
| Failure mode | one monolithic all-or-nothing run | phase-granular; rerun the failed phase | checkpoint/resume |
| Backstop | none | hard `MemoryMax`; git's own memory-estimate escape hatch (`gc.bigPackThreshold`, or `--keep-largest-pack`) lets gc leave the big pack un-repacked when its estimate says repacking won't fit | §2.3 |

The one-sentence version: **`--aggressive` couples three expensive decisions (widen window,
discard deltas, rewrite everything) into one unbounded command with no off switch; the
stepwise design makes each decision separately, only when a measured gate asks for it, and
caps every one of them in both soft git terms and a hard cgroup ceiling.**

---

## 7. Gaps found while writing this — for the implementation bead

Measured/read against the current tree. The two superseded gc documents are corrected in
this changeset — they were actively misleading and the fix is documentation-only — while
§7.1's script changes and the remaining rows below are the work items this design hands to
the implementation.

### 7.1 `scripts/safe-git-gc.sh`

| # | Current | Problem | Recommendation |
|---|---|---|---|
| 1 | One `MEMORY_MAX` (default `2g`) drives **both** the cgroup `MemoryMax` **and** `pack.windowMemory`, with `pack.deltaCacheSize` separately set to `1g` | Ceiling (2 G) < soft sum (2 g + 1 g ≈ 3 g). A run that fills its legitimate allowance is killed by its own protective cgroup — the failure we built the ceiling to prevent, relocated. (`setup-git-gc-config.sh --verify` already computes the soft sum and assumes a 6 GiB ceiling; the script should agree with it.) | Split into `SAFE_GC_CGROUP_MAX` (default `6g`, matching the verify script) and `SAFE_GC_WINDOW_MEMORY` (default per §5 profile), and assert `cgroup ≥ window + cache + ~512m`. |
| 2 | Stage 1 `git gc --prune=now` | Discards the 2-week grace period and flips the repack to `-a`, deleting unreachable objects outright — in a repo 10+ agents write to concurrently. | `git gc` (no flags); `gc.pruneExpire=2.weeks.ago` already supplies the policy. |
| 3 | Stage 2 `git repack -q -d --no-write-bitmap-index --max-pack-size=500m` | `--max-pack-size` is not a memory control; it splits the pack, prevents a bitmap index, and per git docs "may result in a larger and slower repository." | Drop it. Packing loose objects is already done by stage 1's gc. |
| 4 | Stage 2 `git repack -q -d -f --no-write-bitmap-index --depth=50 --window=50` and stage 3 `-d -f --depth=10 --window=10` | Both are **incremental** (`-d` without `-a`/`--cruft`), so after stage 1's gc has packed everything loose they have ~nothing to operate on; stage 3 additionally narrows the window to 10 — weaker than stage 2's own 50 — so the "deep compression" stage could only ever produce a *worse* pack. The 2026-09-02 07:55 run showed stage 2's steps failing/skipping while the stage reported complete. | Replace stage 2 with the gated phase-2 command; replace stage 3 with the phase-3 command (`--cruft`, window 50, `--window-memory`); add the §4 gates so empty work is skipped instead of reported. |
| 5 | Stages 2/3 hardcode `--no-write-bitmap-index` | Correct for incremental repacks (bitmaps genuinely are incompatible), but a full `--cruft`/`-a` repack *can* write bitmaps — and this repo wants one. | Keep the flag on any incremental repack; drop it from full repacks and let `repack.writeBitmaps=true` apply. |
| 6 | Stage 1 re-writes `git config pack.windowMemory` / `pack.deltaCacheSize` every run, sets neither `pack.threads` | Redundant with the persistent `setup-git-gc-config.sh` state; `pack.threads` is left to the global scope, so the script's own guarantee is incomplete on a box where the global config hasn't been applied. | Rely on `setup-git-gc-config.sh --verify` as the phase-0 gate; have the script pass `--threads=1` explicitly per run. |
| 7 | Preflight `git fsck --no-progress` | That's the default (shallow) fsck; the repo's own health checks use `--full`. | `--full` in the phase-0 gate (it is the stronger check; cost is seconds at this repo size). |
| 8 | `check_gc_needed` thresholds (loose > 1000, packs > 5) | Only drive `--check-only` reporting; fine, but they never gate escalation of stages 2/3. | Add the §4 phase-2 gate so wide-window work runs on evidence. |

### 7.2 Stale or wrong documentation

| File | Claim | Reality |
|---|---|---|
| `docs/git-gc-config.md` | `gc.aggressivewindow=1` is configured | Not set (was unset after the `1.hour` exit-128 failure); `1` would be an absurd window anyway. `gc.autoPackLimit=10` is documented; local value is 50. Recommended bare `git gc --aggressive` as the manual path — the exact command that caused the Aug-14 storm. **Fixed in this changeset:** the page now carries live-config values and a correction note. |
| `docs/safer-git-gc-strategy.md` | "~100-500MB" per stage, "~500MB-1GB", "capped at 1GB" | Asserted, never measured. Its stage-3 parameters (window 10) are weaker than its own stage 2 (window 50). **Fixed in this changeset:** a supersession banner now points readers at this document's measured figures. |
| `docs/remediation-strategy-bf-4yjq.md`, `docs/crash-root-cause-bf-4yjq.md` | `gc.aggressiveWindow 1.hour` / `7days` | Not integers; would make every `git gc --aggressive` exit 128 (bf-4x12ec's first crash was exactly this). |
| `CLAUDE.md` (this repo) | "Run standard gc (stages 1-2, ~10-30 minutes)" / "full gc … ~1-2 hours" | With bounds and a converged 90 MiB repo, plain gc is seconds–minutes. The estimates are inherited from the pre-bound era. |

### 7.3 Residual risks this design does *not* eliminate

- **`git gc --auto` bypasses the box-wide lock** (§2.3, §5 rule 3). Bounded, but not
  serialized. Accepted for now; revisit if two concurrent auto-gcs ever show RSS stacking.
- **needle's zero-backoff re-claim loop** amplified the Aug-14 kill into a storm. Needle-side,
  outside this repo; alert-side suppression is handled by `scripts/crash-alert-manager.sh`.
- **A 17 GiB loose-object pile-up is still the wrong operating point.** These phases make
  recovery cheap; `gc.auto`, the repo-health monitor and the pre-commit large-file hook are
  what keep the pile-up from forming.

---

## 8. Verification and monitoring hooks for the implementation

Record per run in the checkpoint JSON (extending the existing fields):

```json
{
  "phase": "phase1",
  "status": "complete",
  "repack_argv": "git repack -d -l -q --cruft ...",
  "window": 5, "depth": 20, "window_memory": "1g", "threads": 1,
  "cgroup_cap": "4G",
  "rss_peak_kb": 313344,
  "loose_before": 17212, "loose_after": 20,
  "packs_before": 1, "packs_after": 2,
  "size_pack_before": "90.34 MiB", "size_pack_after": "89.1 MiB"
}
```

- **Peak RSS sampling** during each phase (the harness in `scripts/test-gc-memory-bounds.sh`
  already does this) — it is the only way to distinguish "the ceiling was never approached"
  from "we got lucky."
- **`git fsck --no-progress` after every phase**, `--full` in phase 0. A phase that ends
  without a clean fsck is a failed phase regardless of exit code.
- **Success criteria** (carried over and tightened from the 2026-09-01 doc):
  never OOMs (ceiling ≥ soft worst case, enforced by assertion); peak RSS recorded and under
  half the ceiling; phase 1 alone reaches the terminal state on routine runs; every phase
  resumable by re-run; `git fsck --full` clean at the end; every size claim measured, not
  asserted.

---

## 9. Open questions for the implementation bead

1. Should `SAFE_GC_MEMORY_MAX` (current name) keep its meaning for callers, with a new
   `SAFE_GC_CGROUP_MAX` derived as 2× the soft sum — or is the split worth a breaking change?
   (Existing callers: `safe-git-gc.sh --full` timer units, `MemoryMax=4G` on the weekly unit.)
2. Should phase 2 run at all on this repo by default, given it converges to a single pack?
   Default answer: gate it off and let the §4 trigger fire it.
3. Does any *other* repo on this box rely on the current stage-2/3 commands? The box-wide
   global pack config makes the fix safe everywhere, but the stage commands are this repo's
   script.

---

## Appendix A — Verification performed for this document

All on git 2.50.1 (this box), 2026-09-05, in throwaway repos under `/tmp`:

| Claim | How verified |
|---|---|
| gc subprocess order (pack-refs → reflog expire → repack → prune → worktree prune → rerere gc), and that cruft packing is a *second* pack-objects run | `GIT_TRACE=1` exec lines (§2.1) |
| gc → repack argv for `git gc`, `git gc --aggressive`, `git gc --aggressive --prune=now` | `GIT_TRACE=1` exec lines (§2.1) |
| `--aggressive` = `-f` + window/depth from `gc.aggressiveWindow`/`gc.aggressiveDepth` (250/50) | traced argv; note git-gc(1)'s "AGGRESSIVE" section mentions only `-f` — the window/depth part is documented under those config keys and confirmed by the trace |
| `--aggressive` overrides `pack.window`/`pack.depth` config | CLI `--window=250 --depth=50` in the traced argv |
| cruft pack used by default; `--prune=now` switches to `-a` | traced argv |
| `repack -a -d` deletes unreachable objects; `--cruft -d` preserves them | scratch repo, 3 unreachable objects: `unreachable 3 → 0` vs `3 → 3`, `.mtimes` present |
| `repack -d --cruft --cruft-expiration=… --window=50 --depth=50 --window-memory=1g --threads=1` runs clean **with** a bitmap index (`repack.writeBitmaps=true`) | rc=0, `.bitmap` + `.rev` written, `git fsck` clean |
| `repack -a -d -f --window=50 …` and `-d -f --cruft …` run clean | rc=0, `git fsck` clean |
| `multi-pack-index write --bitmap` / `repack --batch-size=1g` exist and run clean | rc=0 on both |
| Defaults for `pack.window`(10)/`depth`(50)/`windowMemory`(unlimited)/`deltaCacheSize`(256 MiB)/`deltaCacheLimit`(1000)/`threads`(nproc)/`gc.aggressiveWindow`(250)/`gc.aggressiveDepth`(50)/`gc.cruftPacks`(true)/`gc.auto`(6700)/`gc.autoPackLimit`(50)/`core.bigFileThreshold`(512 MiB) | git-config(1), git-repack(1), git-pack-objects(1), git-gc(1) on this installation |
| "window memory × threads" multiplication | stated verbatim in both git-pack-objects(1) and git-config(1) `pack.threads` |
| Repo-local and global effective values (§2.3 column) | `git config --show-origin --show-scope --get-regexp '^(gc\|pack\|repack\|core\.compression)'` |
| >12 GiB → ≈313 MiB | `docs/maintenance/repository-maintenance-guide.md`, `scripts/test-gc-memory-bounds.sh` (pre-existing measurements, cited not re-run) |
| gc writes commit-graph by default | git-gc(1) `gc.writeCommitGraph`, default true |

Not verified (flagged where it matters): behaviour of git versions other than 2.50.1;
`pack.threads` on a box where git is compiled without pthreads (the option is then ignored
with a warning — the cgroup ceiling still applies); exact RSS figures for phases 2/3 on this
repo (budgeted from the memory model, not yet measured — §8's RSS sampling closes that gap).
