# Fix Test Results: agent-crash fix — test child `domchk-19f39731`

| Field | Value |
|---|---|
| **Bead** | `domchk-19f39731` — "Test fix and verify agent stability" (test child of the bf-173o7e split family; blocked-by the fix child `domchk-5355a695`, **Closed** rev 4) |
| **Task** | Test the implemented fix and verify that the agent crash issue is resolved |
| **Test date** | 2026-09-08 (18:17–18:21 EDT), run first-hand for this record |
| **HEAD at test** | `89b9179` (the fix child's verification commit), 0 unpushed |
| **Fix under test** | The seven committed causal-link layers of the bf-173o7e fix — inventory and layer→commit map in [`docs/crash-fix-verification-bf-173o7e-domchk-5355a695-2026-09-08.md`](crash-fix-verification-bf-173o7e-domchk-5355a695-2026-09-08.md) |
| **Conclusion** | **Tests pass — the fix holds.** The exact crash workload that SIGKILLed 129 attempts on 2026-08-14 now completes exit 0 inside a 768 MiB cgroup (1/16th of the 12 GiB dispatch scope) at 320,524 KB peak pack-objects RSS, with no OOM kill outside a deliberate negative test. All suites green; no crashes observed; the split chain is complete (umbrella already Closed). |

---

## 1. Acceptance criteria → live evidence

| Acceptance criterion | Result | Evidence (this date, first-hand) |
|---|---|---|
| Fix deployed to test environment | ✅ | The test environment *is* the fix's deployment target — this workspace. Every layer verified live and in force: dispatch scope `run-p1662857-i253288857.scope` at cgroupfs `memory.max=12884901888` / `oom.group=0`; persistent pack bound effective (`windowMemory=2g`, `deltaCacheSize=1g`, `threads=1`, all local — worst case ≈3072 MiB); `.gitignore` rules at `:66,68-70` with `git ls-files .beads` → **0**; 10 MB pre-commit hook installed and **byte-identical** (`setup-git-hooks.sh --check` rc 0); `safe-git-gc.sh` + breaker (`crash-circuit-breaker.sh status` → no open breakers) + dispatch gate `needle-with-limiter.sh` present; **8/8** `domain-check-*` systemd user timers future-triggered |
| Ran agent through similar workload that caused the original crash | ✅ | `scripts/test-gc-memory-bounds.sh` — the death-op replay: the **verbatim crash command** `git gc --aggressive --prune=now` over 8×64 MiB incompressible blobs, and the bf-1ea4g death operation (bounded `git push` over a 6×32 MiB unpacked backlog), each under `MemoryMax=768M` |
| Verified no crashes occur with the fix in place | ✅ | Replay **17/17, rc 0** — both historical kill operations **exited 0** (no SIGKILL). Journal census for the test window (18:16–18:21 EDT): the **only** OOM kill is the safe-gc suite's *own deliberate negative test* (see §3) — no unintended kill anywhere |
| Monitored resource usage (memory/CPU) during testing | ✅ | Cgroup-accounted peaks: pack-objects **320,524 KB**, push **232,124 KB** (caps: 700 MiB). Box during the run: MemAvailable 55.2 → 55.7 GB, load 1-min 10.4 → 7.3 — no memory pressure (floor 20 GB), the replay is too small to move box-level numbers, which is the point |
| Documented test results and confirmed stability | ✅ | This document + the bead note on `domchk-19f39731` |
| Ready to close parent bead if tests pass | ✅ (vacuous) | The parent/umbrella `domchk-8156d915` ("Analyze root cause of agent crash") is **already Closed** (rev 4, RCA delivered at HEAD `3a6f3d1`). Nothing is owed to it; this bead's closure completes the split chain: RCA `domchk-8156d915` → fix `domchk-5355a695` → **test `domchk-19f39731`** |

## 2. Full battery, this date

| Check | Result |
|---|---|
| `scripts/test-gc-memory-bounds.sh` (death-op replay) | **17/17 passed, rc 0** — crash command exit 0 under `MemoryMax=768M`, peak pack-objects RSS **320,524 KB < 700 MiB**, repo fully packed after gc; push leg exit 0, peak **232,124 KB**, bare remote received the backlog, backlog stayed loose (18 objects) |
| `scripts/test-safe-git-gc-limits.sh` | **33/33 passed, rc 0** |
| `scripts/check-repo-health.sh` | rc 0 — repo **105 MB**, 156 loose objects, 1 pack (**100.49 MiB**), 0 garbage, fragmentation acceptable, effective pack bound verified within the 6 GiB ceiling, **0 unpushed backlog** |
| `scripts/setup-git-gc-config.sh --verify` | rc 0 — effective (system → global → local) worst case **≈3072 MiB** vs scope `memory.max=12884901888` |
| `scripts/safe-git-gc.sh --check-only` | preflight resource checks pass; **"GC not needed"** verdict (exit 1 = verdict, not failure) — repo has stayed below gc-worthy size since the 2026-09-01 repair |
| `scripts/preflight-health-check.sh` | **5/5** — no system event, gateway available (checked `-skf`), repo healthy, cgroup headroom healthy, no open breakers |
| Timers | `systemctl --user list-timers 'domain-check-*'` → **8/8 present, all future-triggered** |
| Box pre-flight | 55 GB MemAvailable, 22 G free disk, load 10.4 (within the ≤15 limit; co-tenant noise, settled to 7.3 during the run) |

**Scale of the fix, one number:** the Aug-14 crash run of `git gc --aggressive --prune=now` exceeded the
12 GiB dispatch scope and was memcg-OOM SIGKILLed **129 times across 131 attempts**. The same command now
peaks at **320,524 KB ≈ 313 MiB — 2.4% of that scope (~38× headroom)**, and passes with 3× margin even
inside a 768 MiB cgroup.

## 3. The one OOM kill in the window is the guard working, not a failure

`journalctl` for 18:16–18:21 EDT records exactly one memcg OOM kill:
`safe-git-gc-run-1687730-*.scope`, `task=bash`, anon-rss 63,488 kB. That is **test [2] of
`test-safe-git-gc-limits.sh` doing its job**: it runs a 300 MB allocation under a deliberate **64 M**
ceiling and asserts the over-limit process is SIGKILLed (exit 137) — proving the hard ceiling enforces
rather than being decorative. Whole-day census (2026-09-08): **7 OOM kills, all `bash`, all inside
`safe-git-gc-run-*` scopes** — one per suite invocation (each verification pass today fires it once).
**Zero** kills in any `run-p*.scope` dispatch scope, **zero** in the `gcmb-*` replay scopes. This mirrors
the standing fleet signature (kernel memcg kills occur only inside synthetic repro scopes).

## 4. Build-state flag (pre-existing; out of scope for this docs-only bead)

Worktree `go build ./...` fails at `internal/watch/manager.go:491:24: domain.Parse undefined` — a
co-tenant's **in-flight uncommitted** `internal/watch` edit, byte-for-byte the same failure the fix child
flagged at 18:06 EDT four hours earlier (`89b9179` message, §4). Separately, HEAD itself does not compile
(`e4fcbec` committed the `NewResourceMonitor` call site without the untracked
`internal/server/resource_monitor{,_test}.go`). Neither involves this bead, which adds one `docs/*.md`
file; both are surfaced for their owners.

## 5. How to re-verify

```bash
./scripts/test-gc-memory-bounds.sh            # death-op replay, 17 assertions (the crash workload)
./scripts/test-safe-git-gc-limits.sh          # safe-gc suite, 33 assertions (incl. the deliberate-kill negative test)
./scripts/check-repo-health.sh                # bloat + bound + backlog
./scripts/setup-git-gc-config.sh --verify     # effective pack-memory bound
./scripts/preflight-health-check.sh           # 5 checks incl. breaker + cgroup headroom
systemctl --user list-timers 'domain-check-*' --all
git ls-files .beads | wc -l                   # must stay 0
# journal proof that the only OOM kills are the suite's own negative test:
journalctl --since today --no-pager | grep -oE "oom_memcg=[^,]*scope" | sort | uniq -c
```

## Related

- Fix verification (the bead this one tests): [`docs/crash-fix-verification-bf-173o7e-domchk-5355a695-2026-09-08.md`](crash-fix-verification-bf-173o7e-domchk-5355a695-2026-09-08.md)
- RCA (umbrella deliverable): [`docs/crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md`](crash-investigations/bf-173o7e-aug14-storm-root-cause-2026-09-02.md)
- Crash-context pass: [`docs/crash-investigations/bf-173o7e-crash-context-domchk-114d6172-2026-09-08.md`](crash-investigations/bf-173o7e-crash-context-domchk-114d6172-2026-09-08.md)
- Prevention requirements (G-1..G-13): [`docs/crash-prevention-requirements.md`](crash-prevention-requirements.md)
- Per-layer validation procedures: [`docs/crash-prevention-validation.md`](crash-prevention-validation.md)
- Maintenance guide: [`docs/maintenance/repository-maintenance-guide.md`](maintenance/repository-maintenance-guide.md)
