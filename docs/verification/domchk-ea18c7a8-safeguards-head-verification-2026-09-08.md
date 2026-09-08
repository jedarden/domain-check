# Server safeguards on committed HEAD — verification (domchk-ea18c7a8)

**Bead:** domchk-ea18c7a8 (child 4 of 5, split of domchk-b1626933 — "All safeguards tested and working")
**Date:** 2026-09-08, run at ~10:17–10:23Z (06:17–06:23 EDT)
**Code state:** HEAD `017452a8a07ef7bf39736baca8a4400b52de1af1`, tested in a throwaway
`git archive HEAD` extract at `/tmp/dc-head-verify` — deliberately **not** the shared worktree,
which carries a co-tenant's in-flight work.
**Verdict:** every safeguard test passes on HEAD once the one missing file is present; the pure-HEAD
build fails on exactly that missing file, fully attributed (§3) and not fixed, per the task's
gotcha. One finding disclosed (§4).

## 1. Acceptance criteria mapping

| Criterion | Result |
|---|---|
| `go build ./...` succeeds on HEAD | **❌ on pure HEAD → ✅ with the co-tenant's two untracked files added.** Failure is `internal/server/server.go:117:13: undefined: NewResourceMonitor`, i.e. committed code referencing a file that has never been committed. Attributed in §3; no fix applied. |
| Named safeguards/resource-monitor test pattern passes on HEAD | **✅ rc 0, 6/6 PASS** (`-run 'TestSafeguard\|TestServer\|TestResourceMonitor' -count=1`, 0.205s) — on HEAD + the two untracked files; the pattern cannot compile without them because HEAD's `server_safeguards_test.go` also references `NewResourceMonitor`. Coverage nuance in §5. |
| Any failure attributed: committed-HEAD vs co-tenant diff | **✅** §3 — md5-attested A/B: pure HEAD fails; HEAD + exactly the two untracked files builds and passes everything. |
| Evidence committed and pushed (own paths only) | This file is the committed summary. The task named `.beads/state/crash-prevention-testing/safeguards.md` as the record location — that tree is **gitignored by the repo's bloat-prevention layer** (`.gitignore:66`, whole `.beads/`), and CLAUDE.md's rule is that `.beads/` tracked-file count stays 0, so the record was written there *untracked* and this identical summary committed at a tracked path, matching the repo convention (`docs/verification/domchk-189d2f80-...-2026-09-08.md`, `domchk-6a227734-gc-bounds-...`). |

## 2. Run results

All runs inside the HEAD extract; `DOMCHECK_RUN_LONG_TESTS` unset throughout (task step 4 — the
10m memory-growth test stays out of scope):

```
preflight: 54G mem avail · 61G disk free · load 5.38        -> gates pass
go build ./...                                  (pure HEAD) -> rc 1
  internal/server/server.go:117:13: undefined: NewResourceMonitor
go build ./...   (+ resource_monitor{,_test}.go copied in)  -> rc 0
go test ./internal/server/ -run 'TestSafeguard|TestServer|TestResourceMonitor' -count=1
                                                            -> rc 0, 6/6 PASS  (0.205s)
    TestResourceMonitorDefaults                    PASS
    TestResourceMonitorHysteresis                  PASS
    TestResourceMonitorRunStopsOnContextCancel     PASS
    TestResourceMonitorSampleFeedsRealMemoryStats  PASS
    TestServerStartsAndStopsResourceMonitor        PASS
    TestServerShutdown                             PASS
go test ./internal/server/ -count=1  (whole package, supplementary)
                                                            -> rc 0, 151/151 PASS (4.704s)
```

The whole-package run is the bead's real subject: it contains every panic-recovery
(`TestRecover*`), request-timeout (`TestTimeout*`), graceful-shutdown (`TestSignalHandling`,
`TestGracefulShutdownWithActiveConnections`), HTTP-timeout and resource-monitor test in the
package, all green on HEAD+files.

## 3. Failure attribution (the A/B that settles it)

- HEAD's committed `internal/server/server.go` line 117 calls `NewResourceMonitor(...)`. Its
  definition lives in `internal/server/resource_monitor.go`, which is **untracked** in the shared
  worktree (another worker's bead, mtimes 2026-09-06 02:54 / test 03:04). HEAD's
  `server_safeguards_test.go` references it too. A `git archive HEAD` extract therefore contains
  the references but not the definition: pure HEAD cannot compile.
- Contrast run: the two untracked files were **copied** into the extract (md5s recorded identical
  to the worktree originals: `52cf304480d1ef8e3fc5958617c50c5b`, `baf5c78df828da1840eb6314b98dd424`)
  and nothing else changed — build rc 0, named pattern 6/6, whole package 151/151. So the single
  missing file is the entire delta: there is no defect in any committed byte.
- Classification: **committed-HEAD failure caused by uncommitted co-tenant work** — the
  already-documented F1 finding (commit `7879715`; CLAUDE.md notes main unbuildable in isolation
  since `e4fcbec` 2026-09-07 for exactly this reason). Pre-existing and disclosed; per the task's
  gotcha the co-tenant's in-flight files were neither staged, edited, nor reverted, and the fix
  belongs to their bead (commit the two files).

## 4. Findings

- **F1 (pre-existing, documented):** `go build ./...` fails on pure HEAD (`017452a`) with
  `undefined: NewResourceMonitor` at `internal/server/server.go:117`. Any baseline verified on a
  HEAD extract fails the same way until `resource_monitor.go` + `resource_monitor_test.go` are
  committed by their owner. Green worktree runs ride on the untracked files (verified here: with
  them, 151/151).
- **F2 (task-text drift, informational):** the task described `server_safeguards_test.go` as
  MODIFIED by the co-tenant; at run time it was **clean vs HEAD** — only the two
  `resource_monitor*` files remained untracked. Also disclosed: the worktree's other staged dirt
  (docs moves, `internal/resilience/*` deletions) is the co-tenant's;
  `git grep internal/resilience HEAD` finds no HEAD reference, so it is orthogonal to this
  verification.

## 5. Pattern-coverage nuance (for the next verifier)

The tasked pattern matches only **6** tests — there is **no `TestSafeguard*` prefix anywhere** in
the package. The safeguards live under other names: `TestRecover*` (10), `TestTimeout*` (6),
`TestSignalHandling`, `TestGracefulShutdownWithActiveConnections`, `TestHTTPTimeouts`,
`TestRequestTimeoutAccommodatesBulkChecker`, `TestContextCancellation`,
`TestRouterChainsRecoverAndTimeout`, `TestRateLimiterRunCleanupStopsOnContextCancel`
(`internal/server/safeguards_test.go`, `server_safeguards_test.go`). A run keyed on the name
pattern alone therefore under-covers the safeguards by 145 tests; the whole-package run in §2 is
the number to cite for "all safeguards tested and working".

## 6. Raw evidence index (gitignored `.beads/state/crash-prevention-testing/`)

- `safeguards-build-head-FAIL-ea18c7a8.log` — the pure-HEAD undefined-symbol failure
- `safeguards-build-head-plus-ea18c7a8.log` — empty output, rc 0, with the two files added
- `safeguards-suite-head-plus-ea18c7a8.log` — verbose named-pattern run (6 PASS)
- `safeguards-fullpkg-head-plus-ea18c7a8.log` — whole-package `ok ... 4.704s`
- `safeguards.md` — the tasked record (same content as this file, untracked per gitignore)

## 7. Safety statement

The shared worktree's dirty and untracked files were never staged, edited, or reverted (verified
copies only, in `/tmp`); zero live bead mutations beyond this bead's own close; the `/tmp` extract
is disposable. Own paths only in the commit carrying this file.
