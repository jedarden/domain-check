# Post-Repack Repository Verification — bf-5jhvpk

**Verified:** 2026-09-08T00:49Z
**Bead:** domchk-3d5f740f (verification leg; repack itself executed by
domchk-371e54d8, preflight baseline by domchk-2971874f)
**Baseline:** `/tmp/bf-5jhvpk-baseline.txt` (recorded 2026-09-08T00:08:46Z at
HEAD `3b2bdf9`)
**Repack:** 2026-09-08T00:28:54Z, `git repack -a -d --depth=250 --window=250
--no-write-bitmap-index` under `systemd-run --user -p MemoryMax=4G -p
MemorySwapMax=0 -p CPUQuota=300%`, HEAD at run time `7f8af4d` — exit 0, 3s,
scope peak 350.4M (~11x headroom), no OOM in journalctl/dmesg

## Verdict

**PASS.** The repack consolidated 2 packs into 1 with no integrity damage and
no growth in the on-disk footprint. Every acceptance criterion is met; the one
number that looks like a miss (`size-pack` +0.47 MiB) is an artifact of the
baseline splitting its object store across two buckets and is explained below.

## Acceptance Criteria

### 1. `git fsck --full` reports no errors — PASS

```
git fsck --full   →  exit 0
```

Dangling objects only (28 entries: stale needle-worker WIP commits, trees and
blobs from today's co-tenant churn — including a superseded earlier copy of
what is now HEAD). Same shape as the pre-flight gate's
`fsck --connectivity-only` result; no corruption, no missing objects, no
errors.

### 2. Post-repack size-pack vs pre-repack — PASS (with explanation)

| Metric | Baseline 00:08Z | Now (00:49Z) | Delta |
|---|---|---|---|
| packs | 2 (99M + 679K) | **1** (104,777,374 B) | 2 → 1 |
| `size-pack` | 99.78 MiB | 100.25 MiB | **+0.47 MiB (+0.5%)** |
| loose objects | 485 / 3.48 MiB | 101 / 656 KiB | −384 / −2.85 MiB |
| **packed + loose** | **103.26 MiB** | **100.89 MiB** | **−2.37 MiB (−2.3%)** |
| in-pack objects | 11,700 | 12,174 | +474 (formerly loose) |
| `du -sh .git` | 107M | **104M** | −3M (−2.8%) |
| garbage | 0 | 0 | — |
| `tmp_*` pack files | — | **0** | — |

Read the `size-pack` line together with the loose line: the baseline's
99.78 MiB covered **two** packs and **excluded** 3.48 MiB of loose objects,
while the single post-repack pack **absorbed** 474 of those formerly-loose
objects (and 7 co-tenant commits landed after it). Raw `size-pack` therefore
overstates by construction. On the like-for-like measure — total object store,
packed plus loose — the repository **shrank by 2.37 MiB**, and the whole `.git`
directory shrank 107M → 104M.

Size-pack has not moved at all since the repack: the current pack
(`pack-2b881867`) is byte-identical to the executor's immediate post-repack
measurement (104,777,374 bytes), so the 7 subsequent commits added only loose
objects (88 right after → 101 now; still 4.8x below the 485 baseline).

Delta compression is confirmed effective: `git verify-pack -v` puts the pack's
uncompressed content at 146.70 MiB vs 100.25 MiB on disk (−32%). Its object
inventory sums to exactly the `count-objects` figure — 2,671 commits + 4,420
trees + 4,884 blobs + 199 tags = 12,174.

Two non-integrity notes:
- The old pack's `.bitmap` is gone — expected under `--no-write-bitmap-index`
  per the run spec (non-bare repo; the weekly full-gc regenerates one). A
  missing bitmap slows local fetch/push delta search; it stores no object data.
- A `.rev` reverse-index file is new alongside the pack — normal modern-git
  output.

### 3. No leftover `tmp_*`; loose count not grown — PASS

`find .git/objects/pack -name 'tmp_*' -o -name '*.tmp'` → 0 files. Old packs
and their indexes were pruned; only `pack-2b881867{.idx,.pack,.rev}` remain.
Loose objects fell 485 → 101 (the baseline's loose population is what the
repack packed in).

### 4. HEAD unchanged and `go build ./...` passes — PASS

- HEAD is `fea1a62` (2026-09-08T00:35:08Z, co-tenant doc commit). `HEAD~7` is
  **exactly** the baseline commit `3b2bdf9`, and `git merge-base --is-ancestor`
  confirms it — the repack-time HEAD `7f8af4d` is `HEAD~3`. History advanced
  only by 7 ordinary co-tenant commits after the repack; no reset, amend, or
  HEAD movement from the repack itself.
- `go build ./...` → exit 0.
- `git status`: only the pre-existing co-tenant modifications and untracked
  investigation docs present at dispatch time — nothing created by this
  verification.
- `scripts/check-repo-health.sh` → exit 0: 1 pack, effective pack-memory bound
  ≈3072 MiB worst case (within the 6GiB ceiling for the dispatch scope), no
  unmanaged aggressive gc, **0 unpushed commits** vs `origin/main`.

## Run Notes Worth Keeping

- Attempt 1 of the repack (00:26:36Z) exited 1 **client-side before git ran**:
  this box's `systemd-run` rejects `-p Nice=15` ("Unknown assignment"). The fix
  was a `nice -n 15` command prefix instead of the property. Any future run
  copied from this spec needs the same substitution.
- Preflight baseline artifacts: `/tmp/bf-5jhvpk-baseline.txt`; executor run
  notes: `/tmp/bf-5jhvpk-repack-bead-notes.txt` and
  `.beads/logs/bf-5jhvpk-repack.log`. `/tmp/repack-before.txt` and
  `/tmp/repack-run.log` are **older, unrelated** Sept-2 measurements — do not
  cite them for this run.

## Bottom Line

The aggressive repack achieved its goal — one consolidated pack, 2.37 MiB less
object store, `.git` at 104M — inside its 4G memory cap with ~11x headroom, and
left the repository fully consistent: fsck clean, HEAD lineage intact, build
green, health gate green.
