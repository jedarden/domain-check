# bf-1s6c3 Crash Condition — Reproducible Test Specification

**Subject bead:** bf-1s6c3 ("bead-state snapshot bloat → memcg-OOM kill storm", P2, closed 2026-09-02)
**Dispatch bead:** domchk-f921ef45 ("Create test case reproducing bf-1s6c3 crash scenario")
**Harness provenance:** `scripts/test-bf-1s6c3-crash-condition.sh` was authored and
test-run by the parallel chain's dispatch (domchk-2125075e) and committed by it as
`4f58755` with a §12 provenance record in the canonical crash document; **this bead
independently verified the harness live (7/7) and documents it here**
**Date:** 2026-09-07
**Canonical crash record:** [`docs/crash-analysis-bf-1s6c3-2026-09-06.md`](../crash-analysis-bf-1s6c3-2026-09-06.md)
(§12, "Crash-reproduction harness 2026-09-07" — the committing bead's own record)
**Harness implementing this spec:** [`scripts/test-bf-1s6c3-crash-condition.sh`](../../scripts/test-bf-1s6c3-crash-condition.sh)
**Precedent:** the bf-4yjq harness and its spec
([`bf-4yjq-crash-workload-test-spec-domchk-b90505ad-2026-09-06.md`](bf-4yjq-crash-workload-test-spec-domchk-b90505ad-2026-09-06.md))
established the scaled-reproduction convention this test follows.

> **Dispatch-pointer correction.** This bead's dispatched task says to reference
> `docs/crash-analysis-domchk-c9641ac5-2026-09-01.md` for the exact crash conditions. That
> document analyses a *different* bead (`domchk-c9641ac5`, a 2026-09-01 inference-gateway
> 503 service crash — exit 1, no OOM, no bloat). It contains no bf-1s6c3 crash conditions.
> The exact conditions come from the canonical 2026-09-06 record cited above; see §11 there
> for how the older bf-1s6c3 corpus diverges from it.

---

## 1. The failing workload

**A repository whose object store is dominated by loose, near-identical large blobs, paired
with a memory-bounded agent scope and a store-walking git operation.**

bf-1s6c3's concrete shape (canonical record §1, §4, §6): an automated bead-state writer had
committed **17+ near-identical ~237 MB `.beads/*.jsonl` snapshots** to git, growing the repo
to **≈18 GB with ≈17 GB loose objects**. Every substantive git operation then ran
git-pack-objects with an unbounded working set inside the dispatch scope's **`MemoryMax=12GiB`**, the kernel's memcg
OOM killer SIGKILLed it, and needle recorded the
sentinel `exit code: -1`. Over 265 minutes the retry loop produced **76 dispatch attempts:
71 × exit −1 (memcg OOM), 4 × exit 124 (timeout), 1 × exit 0** — 71/76 died at **`git
push`'s pack-objects**; the Aug-14 variant (bf-173o7e / bf-4x12ec) died at bare
`git gc --aggressive --prune=now`, the same mechanism one command earlier.

The workload type is therefore the pairing
`{17 GB loose near-identical blobs} × {bounded scope} × {push | aggressive gc}` —
not a specific repository. That is what makes it reproducible: the loose store only has to
be *big relative to the scope's memory bound*, and the payloads only have to be
*individually incompressible but mutually near-identical* (so zlib cannot collapse them,
yet delta search still walks them all). Both properties are constructed, not copied.

## 2. Crash parameters

| Parameter | Value | Provenance |
|---|---|---|
| Storm span | 2026-08-12T21:31:27Z → 2026-08-13T02:01:27Z (265 min) | Canonical §1 |
| Attempts / kills | 76 attempts: 71 × exit −1, 4 × exit 124, 1 × exit 0 | Canonical §1 |
| Death step | `git push` → git-pack-objects (71/76); `git gc --aggressive --prune=now` (Aug-14 variant) | Canonical §4, §6 |
| Kill mechanism | kernel memcg OOM SIGKILL, `constraint=CONSTRAINT_MEMCG`, inside the dispatch scope | Canonical §6; kernel records recovered for the push-side variant bf-198ne |
| Needle-visible code | `exit code: -1` — a *sentinel* for a signal death with no recorded code, not a signal number | Canonical §11 |
| Repository size | ≈18 GB `.git`, ≈17 GB loose | Canonical §1, §4 (canon-sourced; not re-measurable — repo repaired 2026-09-01) |
| Bloat composition | 17+ near-identical ~237 MB `.beads/*.jsonl` snapshots committed | Canonical §4 |
| Scope bound | `MemoryMax=12GiB` (`docs/maintenance/repository-maintenance-guide.md:158`) | Canonical §6 |
| Amplifying cause | retry loop re-entered the same bloat 76×; each kill emitted another alert | Canonical §6, §7 |

## 3. What the harness asserts

The harness rebuilds the crash precondition, re-creates both death steps, then proves each
deployed prevention layer stops one link of the chain. Seven assertions, all required:

| # | Assertion | Expected |
|---|---|---|
| A1 | **Bloat forms** — N commits of one near-identical JSONL snapshot path | ≥30% of raw bytes land as loose objects (measured ≈55%), 0 packs |
| A2 | **Crash re-created: `git push`** inside `MemoryMax=512M` | signal death, loose set intact, kill attributed to memcg OOM (kernel `oom_memcg=` line or systemd's OOM notice) |
| A3 | **Crash re-created: `git gc --aggressive --prune=now`** in the same bound | same, with the loose set unpruned — the bf-173o7e/bf-4x12ec variant |
| B1 | **Bounds mitigate: A3 + `pack.windowMemory` / `pack.deltaCacheSize` / `pack.threads`** | exit 0, ≥1 pack, inside the same 512M that just killed it |
| B2 | **Bounds mitigate: A2 over the packed store** | exit 0, remote receives the branch |
| C | **gitignore prevents** — the live repo's rules refuse `.beads/**`, `*.jsonl`, `*.db` | `check-ignore` matches all four payload shapes |
| D | **Hook prevents** — the installed pre-commit hook | blocks an 11 MB file *and* a force-added `.beads/` snapshot |

A2/A3 are the acceptance criterion "trigger git operations that caused OOM in the original
crash": they execute the two exact command shapes that killed the fleet and assert the same
kernel mechanism.

## 4. Why reproduction is scaled — and why the 18 GB figure is not built

The dispatched acceptance criterion reads "creates an 18GB repository with 17GB loose
objects", and the harness deliberately does **not** do that — it builds the same condition
at ~1/15th scale. This is the design, not a fallback, and this section is this bead's
disposition of that criterion. The same reason the bf-4yjq spec gives applies, plus three
bf-1s6c3-specific ones:

1. **The full condition is what this repo's guardrails exist to prevent.** The bloat table
   in `CLAUDE.md` marks >1 GB total as *Critical — immediate cleanup*; a test that built
   18 GB to satisfy the criterion's letter would trip every detector the previous beads
   installed, on a shared box with a single 444 GB disk.
2. **It would not fit safely.** 18 GB of loose objects plus pack copies plus margin needs
   ~55 GB of scratch against the box's free space at dispatch time (52 GB) — and the
   harness's own preflight refuses any requested scale above a 4 GiB hard cap
   (`BF1S6C3_SNAPSHOT_MB × BF1S6C3_SNAPSHOTS`), exiting 2 before any git work.
3. **Scale is not the mechanism — the ratio is.** pack-objects' peak RSS scales with the
   loose-object set, and the crash turns on `RSS(bounded) > MemoryMax`. Holding that
   inequality across a scale change reproduces the kill deterministically; holding the
   absolute size adds only I/O time. The harness scales both sides: defaults build
   ~2 GiB raw → ~1.1 GiB loose (≈1/15th of the original's 17 GB) and test inside a
   512 MiB scope (≈1/24th of the 12 GiB dispatch scope) — the unbounded run overflows the
   bound, and the bounded run (B1/B2) fits, which is exactly the invariant the original
   crash turned on.

This matches the sanctioned precedent: the bf-4yjq harness was committed at **1/17th scale**
(described in its own commit as "memcg OOM kill reproduced at 1/17th scale, both
mitigations verified") and is the repo's accepted pattern for crash-condition tests.

**Determinism** comes from the construction, not from the scale: the snapshot generator
emits one fixed random field per record and moves only a round counter, so every round is
byte-identical except one integer — maximally delta-able, while the random field keeps zlib
from collapsing the blob. Given the same kernel OOM behaviour, the same assertions pass on
every run; three independent runs (§6) produced the same verdicts.

## 5. How to run

```bash
# defaults: 32 × 64 MiB snapshots → ~1.1 GiB loose, 512 MiB scope, ~2–4 min
./scripts/test-bf-1s6c3-crash-condition.sh

# keep the scratch repo for inspection (default: removed on exit)
DOMCHECK_KEEP_BF1S6C3=1 ./scripts/test-bf-1s6c3-crash-condition.sh

# scale overrides (product hard-capped at 4096 MiB; preflight exits 2 above it)
BF1S6C3_SNAPSHOT_MB=192 BF1S6C3_SNAPSHOTS=16 BF1S6C3_MEMORY_MAX=512M \
  ./scripts/test-bf-1s6c3-crash-condition.sh
```

| Variable | Default | Purpose |
|---|---|---|
| `BF1S6C3_SNAPSHOT_MB` | 64 | raw MiB per commit's snapshot |
| `BF1S6C3_SNAPSHOTS` | 32 | commits of the one snapshot path |
| `BF1S6C3_MEMORY_MAX` | 512M | the stand-in for the 12 GiB dispatch scope |
| `BF1S6C3_TIMEOUT` | 1800 | wall-clock guard per bounded operation |
| `BF1S6C3_WINDOW_MEMORY` / `_DELTA_CACHE` / `_THREADS` | 128m / 64m / 1 | B1/B2 pack bounds, scaled to the 512 M scope (deployed values are 2g / 1g / 1) |
| `BF1S6C3_MIN_AVAIL_GB` | 2 | preflight MemAvailable floor |

**Exit codes:** `0` all seven assertions passed · `1` one or more failed ·
`2` preflight refusal (short disk/memory, scale above the cap, or missing
`systemd-run`/`python3`/`journalctl`) — nothing ran.

**Safety envelope:** every git/push/gc execution runs inside a
`systemd-run --user --scope` hard-capped at `MemoryMax=512M` with `MemorySwapMax=0` and a
wall-clock timeout — the kill assertions are themselves the proof the cap contains the
blast radius. All scratch state lives in one `mktemp` dir under `/tmp`. The live
repository is only *read* (its `.gitignore` and hook installer are copied into the scratch
repo, never written). The push target is a local bare remote — no network.

## 6. Measured validation

**This bead's run, 2026-09-07 06:08Z, defaults (32 × 64 MiB), 7/7 passed** — raw log
`.beads/state/bf1s6c3-verify-060814.log` (gitignored; figures below are its own):

| Step | Measured |
|---|---|
| A1 | 46 s to build **1122 MiB loose across 128 objects, 0 packs** (2048 MiB raw → 0.55 deflate ratio) |
| A2 | bare `git push` died in **2 s**, scope exit 143, OOM records user=2 kernel=1, loose set intact |
| A3 | bare `git gc --aggressive --prune=now` died in **2 s**, same attribution, 128 loose / 0 packs — unpruned |
| B1 | the same aggressive gc with the bounds: **61 s, exit 0, 1 pack** inside the same 512 MiB |
| B2 | the same push over the packed store: **16 s, exit 0**, remote HEAD present |
| C | `.gitignore:66` (`.beads/`) and `.gitignore:70` (`*.jsonl`) match all four payload shapes |
| D | hook blocked both the 11 MB file and the force-added `.beads/issues.jsonl` |

**Cross-scale determinism:** two further independent runs corroborate the verdicts. The
harness-authoring dispatch's earlier invocation (domchk-2125075e, 2026-09-07 05:11Z, 1.5×
scale — 16 × 192 MiB) built **1683 MiB loose across 64 objects, 0 packs** and passed A1
with the same verdict; the committing dispatch's own full run (recorded in the canonical
document's §12 harness appendix) passed **7/7 at the same defaults with identical bloat
figures (1122 MiB / 128 objects)**. Same construction across 1× and 1.5× payload — the §4
scaling claim holds in practice, and the defaults' figures reproduce byte-for-byte.

**Fourth independent run — this bead's closing attempt (2026-09-07 07:03Z, defaults):**
7/7 passed in 133 s (raw log `.beads/state/bf1s6c3-verify-f921ef45-070306.log`,
gitignored): A1 **1122 MiB across 128 loose objects, 0 packs**, byte-identical to the
committed run's figures; A2 push killed in 2 s (scope exit 143, OOM user=2 kernel=1); A3
aggressive gc killed in 2 s, loose set unpruned; B1 bounded gc 67 s, exit 0, 1 pack
inside the same 512 MiB; B2 bounded push 14 s, exit 0, remote HEAD present; C and D as
tabled. Same construction, fourth verdict — the §4 scaling and determinism claims hold
across four runs by three separate dispatches in one morning.

**Two header inaccuracies observed (recorded here rather than edited in the sibling's
file):** the harness header quotes the deflate ratio as "measured 0.34" (both runs measured
≈0.55) and describes A1 as "16 commits of one ~192 MiB snapshot" (that was the sibling's
run; the committed defaults are 32 × 64 MiB). Neither affects any assertion — A1's floor is
30% and the defaults' "~1 GiB loose" claim is correct.

## 7. Limits — what this test does not prove

- **It does not re-measure the original event.** The 18 GB / 17 GB figures are canon-sourced
  from cleanup-era documentation and are not re-measurable — the repo was repaired to
  ~100 MB on 2026-09-01 and re-verified 2026-09-06 (canonical §4, §12).
- **It does not prove the 12 GiB scope would have been exceeded at 18 GB.** That inequality
  is the historical record; the harness proves the *mechanism* (unbounded pack-objects over
  a loose-dominated store kills inside a bound; the deployed bounds make it complete).
- **Kill attribution depends on journald.** A2/A3 accept either a kernel
  `oom_memcg=` line or systemd's user-journal OOM notice, because a notice with no kernel
  line can also be a NixOS switch-to-configuration replay of stale counters.
- **Scope-name teardown race.** `systemd-run --user --scope` exits 1 client-side when a
  same-named predecessor is still tearing down; the harness derives every unit name from
  the run's PID (the fix the bf-4yjq harness needed in `f05cbe3`), so parallel runs do not
  collide.
- **It is not a regression gate for domain-check code.** Nothing here exercises the
  application; it exercises the repository-maintenance layers that keep the crash
  precondition from forming.

## 8. Acceptance-criteria disposition (dispatch bead domchk-f921ef45)

| Dispatched criterion | Disposition |
|---|---|
| "Write a test case that creates an 18GB repository with 17GB loose objects" | Discharged as unsafe by design — §4. The harness builds the identical *condition* (loose-dominated store of near-identical incompressible snapshots) at ~1/15th scale, hard-capped at 4 GiB with preflight refusal; a literal 18 GB build would trip the repo's own Critical bloat threshold and would not fit this box's free disk. |
| "Test should trigger git operations that caused OOM in original crash" | Met — A2 runs bare `git push` and A3 runs bare `git gc --aggressive --prune=now` (the two exact death steps, §2) and asserts the memcg-OOM kill with kernel attribution. |
| "Test must be runnable and deterministic" | Met — three independent runs (§6), two at defaults with identical bloat figures, one at 1.5× scale, all 7/7. |
| "Include test documentation explaining what it reproduces" | Met — the harness header (mechanism, assertions, safety, usage) plus this specification. |

**Signpost for this chain's follow-up bead** `domchk-27d451e9` ("Verify no crash under
original crash conditions"): its criteria — the reproduction test completes without an OOM
kill, git operations finish with normal resource usage, memory stays well inside the bound —
are exactly what this harness's **B1/B2** assertions measure. §6 already records them
passing live (bounded gc 61 s / exit 0 inside 512 MiB; bounded push 16 s / exit 0); peak
scope memory cannot exceed `MemoryMax=512M` by construction, which satisfies the <4 GB
peak requirement with a wide margin. Re-run the harness for a fresh timestamp rather than
re-deriving the mechanism.
