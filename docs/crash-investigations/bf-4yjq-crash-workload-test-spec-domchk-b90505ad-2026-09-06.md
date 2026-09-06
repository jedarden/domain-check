# bf-4yjq Failing Workload — Reproducible Test Specification

**Subject bead:** bf-4yjq ("Git origin remote points to GitHub directly; Forgejo mirror has diverged/gone stale", P2, closed 2026-08-17)
**Parent alert:** bf-hw4i5 ("ALERT: Agent crash on bead bf-4yjq", 2026-08-12T18:41:29Z)
**Dispatch bead:** domchk-b90505ad (split-child chain: identify workload → run baseline test → regression suite → test report → verify fix)
**Date:** 2026-09-06
**Canonical crash record:** [`bf-4yjq-crash-investigation.md`](bf-4yjq-crash-investigation.md) (domchk-4eab7c59, 2026-09-02)
**Harness implementing this spec:** [`scripts/test-bf-4yjq-crash-condition.sh`](../../scripts/test-bf-4yjq-crash-condition.sh)

---

## 1. The failing workload

**Any substantive git operation against a repository whose object store is dominated by
loose objects, executed inside a memory-bounded agent scope.**

Concretely — the scenario that killed 50/50 dispatches of bf-4yjq on 2026-08-12: an agent
process starts in this repository while it held **~18 GB of `.git`, of which 17.2 GiB was
loose objects vs 9.6 MiB packed (≈1,800:1 inverted ratio, ~4,594 objects)**, and issues its
first substantive git command (status/fetch on a cold store, packing, fsck — the canonical
report §6 list). The command's working set exceeds the dispatch scope's memory bound, the
kernel's memcg OOM killer SIGKILLs the process, and the needle-visible result is
`exit code: -1`. The agent dies on *whichever git operation it touched first*; the crash is
incidental to bf-4yjq's content (a remote reconciliation), a function of when it was
scheduled relative to the bloat window.

The workload type is therefore **not a specific command** — it is the pairing
`{bloated loose-object store} × {bounded memory scope} × {any store-walking git operation}`.
The harness pins `git gc` as the representative operation because it is the
highest-memory store-walking command and the one whose modern variant (Aug-14, bf-173o7e)
is directly measured.

## 2. Crash parameters

| Parameter | Value | Provenance |
|---|---|---|
| Workload type | Store-walking git ops (status / fetch / pack / fsck) by a dispatched agent | Canonical §2, §6 |
| Repository size at crash | ~18 GB `.git` | Contemporaneous metrics (`.beads/crash-bf-4yjq-summary.txt`) |
| Loose vs packed | **17.2 GiB loose (~4,594 objects) vs 9.6 MiB packed** | Canonical §6 |
| Memory limit | **Not recoverable for Aug-12** — kernel logs before 2026-08-15 19:46 EDT are gone (no journald retention). The known later bound for identical dispatch scopes is `MemoryMax=12GiB` (Aug-14, bf-173o7e/bf-4x12ec). This spec does **not** claim a figure for Aug-12. | Fleet telemetry records |
| Timing — crash window | 2026-08-12 **17:54:00 → 20:30:43 UTC**, 2h37m | Verified from forensic checkpoint |
| Timing — cadence | 50 deaths, **mean interval 188 s (~3.1 min)**; each dispatch died within ~1–3 min of starting | Canonical §3 |
| Timing — per-kill | Kill lands seconds-to-minutes into the first git operation (harness: **~4.7–5.0 s** at 1/17th scale) | This spec §6 |
| Exit signature | `exit code: -1` on **50/50** events (zero variation) — needle's sentinel for a signal death with no recorded code | Canonical §3, §5 |
| Kill class | memcg OOM (inferred MEDIUM-HIGH for Aug-12: contemporaneous telemetry, raw kernel logs lost; **directly observed** in the harness reproduction) | Canonical §6 |
| Host state at crash | load average 15–17 on 12 cores, memory effectively exhausted during git ops, disk 84% full | Canonical §6 |
| Scope | 6 beads, 455 exit-code −1 events, 05:36–23:57 UTC — workspace-wide regime | Canonical §4 |

## 3. Expected vs actual behavior

| | Expected | Actual (2026-08-12) |
|---|---|---|
| Agent dispatch | Agent runs the remote-reconciliation steps to completion | **Killed by SIGKILL-class signal** on the first substantive git operation; needle records `exit -1` |
| Git operation | `git status`/`fetch`/`pack` completes in seconds | Working set exceeds the scope bound → memcg OOM kill; **loose object set left exactly as it was** — nothing the agent did could shrink it, so every retry hit the same wall (50×) |
| Retry loop | Transient failures recover | Deterministic re-kill every ~3.1 min for 2h37m; `failure-count` escalators (1→4) fired while crashes continued |
| Task outcome | Merge commit + repoint `origin` + push mirror | Task progressed **only after the trigger was removed** (repo packed/cleaned Aug 13–14, 18 GB → ~91–138 MB); bf-4yjq then completed normally and closed 2026-08-17 |

**Reproducibility verdict carried over from the canonical report:** deterministic *while the
trigger existed* (50/50), not reproducible against today's healthy repo. Hence the spec
below re-creates the **condition** at reduced scale rather than re-running the **event**.

## 4. Why reproduction is scaled — and why scaling is sound

Re-creating 17.2 GiB of loose objects on the live repo is exactly what the repo-health
guardrails (`.gitignore` for `.beads/`, >10 MB pre-commit block, 1 GB/500 MB alert
thresholds) exist to prevent, and would itself be the hazard. The scaling rests on one
measured law:

> **The git process's peak RSS scales with the loose-object byte volume.**
> Measured: >12 GiB pack working set on 17.2 GiB loose (bf-173o7e storm, 12 GiB scope);
> ~510 MiB RSS on 1.08 GiB loose (harness, §6).

The kill fires when `peak RSS > scope bound`, so a scaled repo (1.08 GiB loose) inside a
proportionally scaled bound (`MemoryMax=512M`) reproduces the same kernel decision —
`CONSTRAINT_MEMCG` OOM kill — through the same mechanism, at 1/17th the bytes and ~2 min
wall clock. The `512M` bound is a **stand-in, not a claim about the Aug-12 scope**, which
is unrecoverable (§2).

## 5. Test specification

**Harness:** `scripts/test-bf-4yjq-crash-condition.sh` — self-contained, scratch repo under
`/tmp/bf4yjq-spec.XXXXXX` (never touches the live repo), removed on exit unless
`DOMCHECK_KEEP_BF4YJQ=1`.

**Setup (performed by the harness):**

1. `git init` a scratch repo; commit **16 blobs × 64 MiB of `/dev/urandom`** (incompressible,
   so loose-object bytes ≈ working-set bytes) → **48 loose objects, ~1.08 GiB**.
2. Establish the bound: `systemd-run --user --scope -p MemoryMax=512M -p MemorySwapMax=0`.

**Assertion A — the crash re-created.** Run bare `git gc` in the bloated repo inside the bound.

- *Expected:* the process dies by signal at the memory peak; scope surfaces a signal exit
  (143/SIGTERM client-side, after systemd tears the scope down post-OOM-kill);
  the journal attributes it to the **memcg OOM killer** (user journal: *"killed by the OOM
  killer"* / *"Failed with result 'oom-kill'"*; kernel: `oom-kill:constraint=CONSTRAINT_MEMCG`,
  `Memory cgroup out of memory: Killed process … (git)`, anon-rss ≈ the bound);
  **the loose set survives untouched** — kill mid-operation, no cleanup, the property that
  made Aug-12 a loop.
- *Fail if:* gc completes, dies without OOM attribution, or mutates the object store.

**Assertion B — deployed mitigation (memory bounds).** Same repo, same `git gc`, with the
deployed bare-gc bounds (`pack.windowMemory=128m`, `pack.deltaCacheSize=64m`, `pack.threads=1`
— what `scripts/setup-git-gc-config.sh` sets).

- *Expected:* **exit 0** — the bound caps the working set below 512 M on the same
  repository that just died.
- *Fail if:* any exit ≠ 0.

**Assertion C — historical mitigation (packed store).** Unset the pack bounds, fully pack the
repo (`git gc` unbounded once), then rerun the agents' ordinary operations — `git status`,
`git log --oneline -5`, `git fsck --full` — inside the same 512 M bound.

- *Expected:* **all exit 0**; store ends `1 pack, 0 loose` — the state the Aug-13/14 cleanup
  produced, after which the real bf-4yjq work proceeded normally.
- *Fail if:* any operation dies, or the store does not fully pack.

**Pass criterion:** 6/6 assertions (1 + 1 + 3 operation checks + store-state check).
**Runtime:** ~2–3 min; peak disk ~2.5 GiB in `/tmp`; peak host RSS ~512 MiB (cgroup-capped).

## 6. Measured validation (this dispatch, 2026-09-06)

Three live runs on this box (62 G RAM, load <1): **6/6 assertions each run, harness exit 0,
identical kill signature every time** — deterministic, as the 50/50 historical record predicts.

Representative kill (run 3, unit `bf4yjq-crash-2387704.scope`):

```
10:14:18.771  systemd: Started [systemd-run] git gc
10:14:23.465  systemd: bf4yjq-crash-2387704.scope: A process of this unit has been killed by the OOM killer.
10:14:23.472  systemd: bf4yjq-crash-2387704.scope: Failed with result 'oom-kill'.
10:14:23.472  systemd: Consumed 4.669s CPU time, 512M memory peak.        # 4.7 s dispatch-to-kill
kernel:
  git invoked oom-killer: gfp_mask=0xcc0(GFP_KERNEL), order=0, oom_score_adj=200
  oom-kill:constraint=CONSTRAINT_MEMCG,…oom_memcg=…/bf4yjq-crash-2387704.scope,task=git,pid=2388175
  Memory cgroup out of memory: Killed process 2388175 (git) … anon-rss:522368kB   # 510 MiB ≈ the 512M bound
```

| Assertion | Expected | Measured |
|---|---|---|
| A: bare `git gc`, 1.08 GiB loose @ 512 M | memcg OOM kill, store untouched | **Killed in ~4.7–5.0 s**, `CONSTRAINT_MEMCG`, anon-rss 510 MiB, 48 loose objects intact |
| B: same gc + deployed pack bounds | exit 0 | **exit 0** |
| C: packed store, status / log / fsck @ 512 M | all exit 0 | **all exit 0**, final store `1 pack, 0 loose` |

## 7. Signature mapping and limits

- **`exit -1` ↔ the harness's signal exit.** Needle's `-1` is its sentinel for a signal death
  with no recorded exit code; the harness's client-side 143 is systemd-run's surface for the
  same event (kernel SIGKILL of the hog task, then scope teardown). Assertion A therefore
  accepts any signal death *and requires* journal OOM attribution, rather than matching a
  specific number.
- **The harness asserts the mechanism, not the Aug-12 magnitude.** Scope bound, repo size and
  per-kill duration differ from Aug-12 by the scaling factor (§4); the kill class, the
  untouched-store property, and both mitigations match the historical record.
- **What this spec does not settle:** the exact Aug-12 scope bound and the exact command each
  of the 50 victims was running — unrecoverable (no kernel logs, no heartbeats, no traces).
  The canonical report's MEDIUM-HIGH mechanism confidence for Aug-12 stands; what this spec
  adds is a *directly observable* instance of that mechanism.
- **Re-run conditions:** safe on any host with `systemd-run --user`, ~3 GB free in `/tmp`,
  and ≥2 GiB memory headroom. All state is scratch; nothing persists.

---

**Deliverable of domchk-b90505ad.** Chain successors (domchk-30e8aab9 "run baseline crash
test", domchk-10404857 regression suite, domchk-3e443d56 test report, domchk-fbb7bbbd verify
fix) can execute this harness as their baseline: green = fix in force; any red assertion
identifies which mitigation regressed.
