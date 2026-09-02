# Pre-Repack Repository Verification — 2026-09-02

Bead: domchk-ed360874
Repository: `/home/coding/domain-check` (branch `main` at `db3f1f2`)
Prerequisite: parent git gc operation complete — **confirmed** (`domain-check-git-gc.service` finished 2026-09-02 07:55:08 EDT)

## Result: ✓ Repository verified ready for repack

All three acceptance criteria pass. Gate script: `scripts/verify-pre-repack.sh` (exit 0).

## Acceptance Criteria

| Criterion | Status | Evidence |
|-----------|--------|----------|
| Repository passes `git fsck` validation | ✅ PASS | `git fsck --full --no-progress` exit 0, zero output — no corruption, no dangling objects |
| At least 2GB free disk space | ✅ PASS | 94G available on `/` (47x requirement) |
| No git processes holding locks | ✅ PASS | No `index.lock`, `gc.pid`, ref/pack `.lock` files; no `git gc/repack/index-pack/pack-objects` processes running |

## Verification Detail

### 1. Parent git gc complete

- `domain-check-git-gc.service` ran at 07:54:53 and finished successfully at 07:55:08 on 2026-09-02 (systemd user journal)
- No `.git/gc.pid` present; no gc or multi-pack-index process running

Note: the scheduled 03:00:10 run the same morning failed with exit 127 (unit
configuration error — "Invalid user/group name" logged at 2026-09-01 21:39:48).
The 07:54 manual/triggered run completed, so the prerequisite holds, but the
unit file defect is worth a separate fix.

### 2. Repository consistency

```
$ git fsck --full --no-progress
(exit 0, no output)

$ git count-objects -vH
count: 35
size: 300.00 KiB
in-pack: 10408
packs: 1
size-pack: 90.18 MiB
prune-packable: 0
garbage: 0
```

Repository is compact and consistent: 92MB total `.git`, single pack, only 35
loose objects (well inside the "healthy" band from the repository size limits
table — <500MB total, <100MB loose, <100 loose objects). No consistency issue
carried over from the gc operation.

### 3. Disk space

94G free on the root filesystem against a 2GB minimum for repack headroom.

### 4. Concurrency / locks

- No git processes matching `gc|repack|index-pack|pack-objects|fsck|prune|commit|merge|rebase` were running
- No git-internal lock files: `index.lock`, `gc.pid`, `config.lock`, `shallow.lock`, `packed-refs.lock`, ref/log/pack `*.lock` all absent
- One **non-git** custom lock exists: `.git/needle-trailer.lock` (0 bytes, dated 2026-08-09, not held by any process per `lsof`). It is created by the needle trailer hook for its own serialization, is not consulted by git, and does not block repack. Reported as informational by the gate script.

## Gate Script

`scripts/verify-pre-repack.sh` reproduces this verification and is intended to
be invoked immediately before any `git repack`:

```bash
./scripts/verify-pre-repack.sh            # full output
./scripts/verify-pre-repack.sh --quiet    # suppress statistics
```

Checks performed (any failure exits 1):
1. Parent gc complete — `.git/gc.pid` absent, no gc/multi-pack-index process
2. `git fsck --full` passes within `FSCK_TIMEOUT` (default 300s); dangling objects are informational, not failures
3. Free disk space ≥ `MIN_DISK_GB` (default 2)
4. No git processes running and no git-internal lock files held

Tunables: `MIN_DISK_GB`, `FSCK_TIMEOUT` environment variables.

Negative paths tested 2026-09-02: `--bogus` arg → exit 2; `MIN_DISK_GB=10000` →
exit 1 with disk failure; simulated `.git/index.lock` → exit 1 with lock
reported (removed afterward; no git operation ran while the test lock existed).

## Conclusion

The repository is in a consistent post-gc state with ample disk and no
concurrent git activity. It is verified ready for the repack operation.
