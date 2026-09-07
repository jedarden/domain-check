# bf-4yjq — Fix Proposal and Verification Record

**Date:** 2026-09-06
**Bead:** domchk-0c601026 (chain: [root cause analysis](../crash-root-cause-bf-4yjq.md) → **this fix proposal** → consolidated findings report)
**Crash:** bf-4yjq, 2026-08-12 — 50 verified exit-code −1 deaths in 2h37m, within the 455-event Aug-12 storm ([canonical crash investigation](../crash-investigations/bf-4yjq-crash-investigation.md))
**Status:** ✅ Fix already deployed; verified live on this date (no new code required)

> **Superseded-claims compliance:** this document restates no entry on the
> corpus's canonical corrections list (§7 of
> [`../investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md`](../investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md)).
> The mechanism given below is the corrected one — **cgroup-scoped kernel
> memcg-OOM SIGKILL**, not host-wide exhaustion and not SIGHUP.

---

## 1. Root cause being fixed

The crash mechanism (from the RCA, re-confirmed by later forensics on
bf-173o7e / bf-4x12ec / bf-198ne): git operations over an enormous loose-object
set — 17.16 GiB across 4,482 objects, produced by bf-2ildm committing 237 MB
`.beads/*.jsonl` files 17+ times — pulled a `git-pack-objects` working set
larger than the 12 GiB `MemoryMax` of the needle dispatch scope. The kernel's
**cgroup-scoped** memcg OOM killer SIGKILLed the agent (the host was not out of
memory, which is why host-wide alerting could never have caught it); needle
recorded the sentinel `exit -1`.

Two independent defects had to co-exist for the crash:

1. **Unbounded accumulation** — the repo held 18 GiB it should never have held
   (`.beads/` was tracked; no size gate on commits).
2. **Unbounded git operation** — a bare `git gc --aggressive --prune=now` (and
   the packing side of `git push`) had no memory ceiling, so repo size mapped
   directly to agent death.

A complete fix must address both; fixing either one alone leaves the crash
reachable.

## 2. The fix (both layers, all deployed)

### Layer A — stop the accumulation (kills the precondition)

| Control | Where | Effect |
|---|---|---|
| `.gitignore` → `.beads/` | `.gitignore:66` | the bloat source (bead-store JSONL) can no longer be committed; 0 tracked `.beads/` files |
| Pre-commit large-file gate | `.git/hooks/pre-commit` (`MAX_SIZE_MB=10`) | rejects any >10 MB file — the 237 MB jsonl commits are now impossible without `--no-verify` |
| Repo-health monitoring | `scripts/check-repo-health.sh`, daily `domain-check-repo-health.timer` 02:00 | alerts at the >500 MB / >500 MB-loose thresholds long before git operations become lethal |
| Repair of the standing bloat | this repo, packed down during Aug/Sep 2026 cleanup | 18 GB → 94 MB now, verified live (`.git`: 90.43 MiB pack, 1.17 MiB loose, `fsck` clean — [cleanup verification](bf-4yjq-cleanup-verification.md)) |

### Layer B — bound the operation (kills the mechanism)

| Control | Where | Effect |
|---|---|---|
| `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` | repo-local **and** box-global git config (`scripts/setup-git-gc-config.sh`, applied `--global` 2026-09-02) | worst case ≈3 GiB per pack run, a quarter of the 12 GiB dispatch scope. `threads=1` is mandatory — the window limit is per thread, so unset threads multiply the window across all cores |
| Effective-bound verification | `scripts/setup-git-gc-config.sh --verify` | exit 1 unless the *effective* system→global→local chain carries a safe bound; this is the check to run whenever a repo reports fresh `exit -1`s |
| Sanctioned gc path | `scripts/safe-git-gc.sh` (memory-limited, checkpoint/resume, pre-flight integrity) | routine maintenance never needs the bare command at all |
| Bounded scheduled gc | `domain-check-git-gc-full.timer` → unit `MemoryMax=4G` | even the weekly full gc is cgroup-confined |

This covers **both** observed kill paths: bare `git gc` (bf-4yjq, bf-173o7e,
bf-4x12ec) *and* the `git push` variant (bf-198ne) — `pack.windowMemory`
bounds pack-objects under push as well (verified in the bf-198ne resolution,
docs/crashes/bf-198ne-crash-report.md).

## 3. Verification (executed live 2026-09-06)

Every claim below was re-run for this record, not quoted from an earlier report.

| Check | Command | Result |
|---|---|---|
| Effective pack bound | `./scripts/setup-git-gc-config.sh --verify` | exit 0 — windowMemory=2g / deltaCache=1g / threads=1, worst case ≈3072 MiB ≤ 6 GiB ceiling |
| Box-wide coverage | `git config --global --get pack.windowMemory` / `pack.threads` | `2g`, `1` — every repo for this user is protected, no per-repo setup needed |
| Crash command under bound | `./scripts/test-gc-memory-bounds.sh` (full suite) | **12 passed, 0 failed.** The exact crash command `git gc --aggressive --prune=now` ran under `MemoryMax=768M` (1/16th of the dispatch scope): exit 0, pack-objects peak RSS 320,552 KB (313 MiB) vs >12 GiB unbounded, repo fully packed |
| Negative controls | same suite | `--verify` correctly *rejects* an unbounded repo, a repo with unset `pack.threads`, and preserves unrelated `gc.*` tuning |
| Accumulation gates | `grep .beads/ .gitignore`, hook inspection, `./scripts/check-repo-health.sh` | `.beads/` ignored (line 66); hook blocks >10 MB; health check exit 0 — 94 MB `.git`, 90.43 MiB pack, 0 garbage, 1 pack, no large files |
| Monitoring alive | `systemctl --user list-timers 'domain-check-*'` | all 6 timers armed and firing (service 2 min, resource 5 min, crash-pattern 10 min, repo-health daily 02:00, gc daily 03:00, full gc Sun 04:00) |

**Regression safety:** the bounds only cap memory; they do not change pack
correctness. The suite's negative controls confirm `--verify` fails closed on
unsafe configs, and the same run demonstrates a fully packed, fsck-clean repo.
Trade-off effects are performance-only (below).

## 4. Does this fix apply beyond bf-4yjq?

**Yes — it is a mechanism-level fix, not a bead-specific patch.**

- **Fleet-wide by construction:** Layer B lives in the *global* git config, so
  every repo on this box inherits it. That is deliberate: the identical
  mechanism killed or drove investigations in bf-1s6c3 (this repo's 18 GB
  bloat), bf-173o7e (129-kill storm, Aug-14), bf-4x12ec, bf-198ne (push
  variant, Aug-16), and crash #4 of bf-3561g (a bare gc of its own).
- **Repo-specific residue:** only the accumulated loose set itself was
  bead-specific, and it has been repaired (18 GB → 94 MB here; bf-1s6c3's repo
  likewise). Nothing in the fix assumes bf-4yjq's particular history.
- **Out of scope, and why:** needle's zero-backoff re-claim loop — which
  amplified one deterministic kill into a 129-attempt storm — is needle-side
  and cannot be changed from this repo; `scripts/crash-alert-manager.sh`
  dedup/cooldown contains the *alert*-side storm. The sibling chain
  domchk-b90505ad → domchk-30e8aab9 is separately building a scaled
  crash-condition harness (`scripts/test-bf-4yjq-crash-condition.sh`,
  untracked / in flight) that re-creates the kill at 1/17th scale; it
  complements, and does not block, this fix.

## 5. Trade-offs and residual risks

| Item | Consequence | Accepted because |
|---|---|---|
| `pack.threads=1` | packing is single-threaded — slower gc/push on large repos | the alternative is an unbounded per-thread-multiplied window; this repo's full gc now takes minutes |
| `pack.windowMemory=2g` | smaller delta search window can yield marginally larger packs / longer searches | memory safety dominates; worst case stays ≈3 GiB |
| Worst case ≈3 GiB *per pack run* | a dispatch scope under ~4 GiB could still be OOM-killed | needle scopes are 12 GiB; the weekly full-gc unit self-caps at `MemoryMax=4G` |
| Layer B covers only *packing* ops | non-packing reads over a huge loose set (`log`, `status`, `diff`) remain unbounded | the practical defense is Layer A — the loose set can no longer grow to lethal size, and repo-health monitoring alerts on drift |
| Hook can be bypassed | `git commit --no-verify` skips the 10 MB gate | the daily repo-health timer still catches the resulting bloat within 24 h |
| Alert storm containment is partial | dedup/cooldown is alert-side; the re-claim loop itself is unchanged | out of this repo's reach; documented so the amplification isn't mistaken for a repo defect |

## 6. Verdict

The fix that addresses this root cause is **already implemented, deployed to
its final scopes (repo + box-global), and verified by live execution on
2026-09-06**: the exact crash command that died 129+ times now completes inside
1/16th of the dispatch scope at 313 MiB peak RSS, and the bloat that made the
repo lethal is structurally prevented rather than periodically cleaned. No
further code change is required for bf-4yjq; the remaining open items belong to
other chains (needle re-claim loop; the sibling scaled crash-condition harness).

**Operational rule this leaves behind:** if any repo shows fresh `exit -1`s
around git operations, run
`./scripts/setup-git-gc-config.sh --verify` first — it resolves the effective
bound and names the scope supplying it.
