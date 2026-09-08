# Crash-Safeguards Verification Report

**Report Date:** 2026-09-08 (results consolidated; umbrella bead opened 2026-09-06)
**Verification Scope:** Every in-process crash-prevention safeguard shipped in `internal/server` and the checker's concurrency-limit layer
**Parent Task:** domchk-c13a55b8 — *Verify crash prevention safeguards with testing*
**Execution:** 5-step split, all steps executed first-hand on 2026-09-08:

| Step | Bead | Scope | Result |
|---|---|---|---|
| 1 | domchk-6999f0fe | Test-suite / regression baseline | PASS (working tree) — see Finding F1 |
| 2 | domchk-69b9ebe8 | Signals, panic recovery, timeouts | PASS |
| 3 | domchk-4dbf80ca | Sustained load, concurrency/connection limits | PASS |
| 4 | domchk-77c89450 | Memory growth / leaks (`DOMCHECK_RUN_LONG_TESTS=1`) | PASS — leak-free |
| 5 | domchk-ec8c8faf | This report | this document |

**Confidence Level:** HIGH
**Conclusion:** 🟢 ALL SAFEGUARDS VERIFIED WORKING — 12 disclosed findings, none a safeguard defect
**Raw evidence:** `.beads/state/safeguard-verification-2026-09-06/` (gitignored — deliberate; test logs are not committed)

---

## Executive Summary

All eight parent acceptance criteria were executed to completion with real measured results, and every in-process crash safeguard in the codebase held under both unit-test and live-process verification:

- **Standard suite:** `go build` / `go vet` / `go test ./...` all exit 0 — 13/13 test packages pass, 0 failures, re-confirmed uncached (`-count=1`).
- **Signals:** SIGTERM, SIGINT and SIGHUP were each delivered to a freshly built live process; all three took the graceful-drain path and exited 0. A held active connection produced a genuine 4.24 s drain inside the 15 s budget.
- **Panic recovery:** 7/7 tests pass — panics become logged 500s with crash-recorder feed, `ErrAbortHandler` re-panics pass through, and a response already started aborts rather than corrupting.
- **Timeouts:** the 30 s request-timeout middleware returns 503 on overrun, propagates deadlines to handlers, drops late writes, and does not misclassify client disconnects; server-level Read/ReadHeader/Write/Idle timeouts are correct against that budget. 6/6 + 2/2 pass.
- **Concurrency/connection limits:** per-registry semaphores (Verisign 10 / PIR 10 / Google 2 / default 3), the global errgroup cap (50) and the 100-per-registry queue-depth cap all queue or reject cleanly — exhaustion never errors the request path, OOMs, or crashes. 76 PASS / 0 FAIL across the five recorded run groups.
- **Sustained load:** 152,661 requests across the three sustained/bulk scenarios at 0.00 % error; p99 4.83 ms (10 s), 53.9 ms (30 s), 5.24 ms (concurrent bulk).
- **Memory:** 30 s / 2 m / 10 m load variants plus a 50,000-entry IP-limiter test — **leak-free**. The 10-minute run ends *below* its heap baseline; OS-reserved memory flat; goroutines return to baseline; cache and limiter maps bounded.

Twelve findings are disclosed below (F1–F12). **None is a defect in a safeguard.** The most consequential (F1) is a repository-integrity issue, not a runtime one: **pure `main` does not compile** because a co-tenant's `resource_monitor{,_test}.go` were never committed, so every green run in this campaign rode on untracked working-tree files. F5/F6 concern the sustained-load test harness driving ~41× its nominal rate.

> **Relationship to the earlier report.** [docs/archive/crash-investigations/safeguards-verification-report-2026-09-01.md](archive/crash-investigations/safeguards-verification-report-2026-09-01.md) (historically referenced as `docs/safeguards-verification-report-2026-09-01.md`; since moved under `docs/archive/crash-investigations/`) is **narrower than this document**: it covers only the original SIGHUP-handling fix (bead domchk-b10b5ec9, commit `7bed0a3`) that added `syscall.SIGHUP` to `signal.Notify`. It does not cover the test suite, panic recovery, timeouts, concurrency limits, sustained load, or memory growth — those are verified here for the first time against HEAD. The SIGHUP behavior it established is re-confirmed live in §2 below.

---

## Provenance

| Step | HEAD when run | Working tree notes |
|---|---|---|
| 1 (05:08–05:40Z) | `4f5d984` | 0 unpushed; co-tenant-untracked `internal/server/resource_monitor{,_test}.go` present (required to compile — F1) |
| 2 (05:36–05:45Z) | `76bf33c` | 0 unpushed; `internal/server/` byte-identical to HEAD |
| 3 (07:33–07:55Z) | `1b27762` | 0 unpushed; all runs `-count=1` (forced fresh execution) |
| 4 (04:02–04:16Z) | `23d83f3` | 0 relevant skips; `-timeout 20m` added (see F11) |

All steps ran in the shared worktree on the lab box, each after a passing preflight (memory ≥ 10 G, disk ≥ 20 G, load < 10). The safeguard source files under test were HEAD-identical at each step. Step 5 re-verified the standing build failure first-hand at current HEAD `0266871` before writing this report.

> **F12 — every number in this report is a single run on a shared box.** Steps 1–4 each executed their measurements once (load averages during runs ranged ≈ 1.6–6). The two independent sustained-load observations available (prior attempt: 10 s p99 3.80 ms, 30 s p99 7.55 ms; this run: 4.83 ms and 53.9 ms) show p99 varies several-fold with box load while error rates stay at 0.00 %. Thresholds in every gate were missed by wide margins (p99 gates ×2–100; heap-growth threshold ×~500), so no verdict here is load-fragile — but individual *latency* figures should be quoted as "measured on a loaded shared box," not as intrinsic properties of the server. A quiet-box re-run would tighten them; it would not plausibly flip a verdict.

---

## 1. Standard Test Suite / Regressions (step 1)

**Criterion:** *"Run existing test suite to ensure no regressions."*
**Verdict: 🟢 PASS** — 13/13 test packages, 0 failures (working tree). One repository-integrity finding disclosed (F1).

| Command | Exit | Outcome |
|---|---|---|
| `go build ./...` | 0 | clean |
| `go vet ./...` | 0 | clean |
| `go test ./...` | 0 | 13/13 packages `ok` |
| `go test -count=1 ./...` (uncached re-run) | 0 | 13/13 `ok`, 0 FAIL |

Per-package (uncached): bootstrap 0.4s · cache 0.3s · **checker 23.8s** · cli 1.1s · config 0.005s · domain 0.03s · **httpclient 20.1s** · ratelimit 6.8s · **rdap 14.2s** · resilience 0.1s · **server 4.5s** · whois 0.3s. No-test-file packages: `cmd/calculate-divergence-stats`, `cmd/domain-check`, `cmd/extract-github-commits`, `internal/watch`, `web`.

**Failing packages: NONE** in the working tree. Long tests (`DOMCHECK_RUN_LONG_TESTS`) were excluded here by design — steps 3 and 4 own them, and both ran them.

> ⚠️ **F1 (the one genuine baseline item): pure `main` does not compile.** A `git archive HEAD` extract of `4f5d984` fails `go build` / `go vet` / `go test` on `internal/server` with 5 undefined symbols (`server.go:117: undefined: NewResourceMonitor`, plus `ResourceMonitor`, `resourceCaptureHandler`, `newResourceMonitorWithThresholds`, `ResourceThresholds` in `server_safeguards_test.go`). Commit `e4fcbec` (2026-09-07 22:48Z) added the +115 lines of *use-sites* but the defining files `internal/server/resource_monitor{,_test}.go` were never committed (`git log --all` on them → empty) — they exist only as untracked working-tree files. **Re-verified first-hand at HEAD `0266871` on 2026-09-08: still broken, identically.** Every commit since `e4fcbec` is unbuildable from a fresh clone; each green run in this campaign compiled only because the shared worktree carries the untracked files. Not fixed here (no-source-change constraint; committing a co-tenant's files is forbidden). **Fix owner: whoever owns that work — commit the two files or revert the use-sites.** The safeguard verdicts in this report are unaffected: every safeguard file tested is HEAD content.

**F2 — golangci-lint not runnable (environment, not code).** Installed at `~/.local/bin/golangci-lint` v2.2.0 but built with go1.24.4 against a repo targeting go1.26.1 → `can't load config: the Go language version (go1.24) … is lower than the targeted Go version (1.26.1)`, exit 3. Rebuild the linter with go1.26 to restore the lint layer; `go vet ./...` (exit 0) covers the vet layer meanwhile.

---

## 2. Signal Handling — SIGTERM / SIGINT / SIGHUP (step 2)

**Criterion:** *"Test signal handling: send SIGTERM/SIGINT and verify graceful shutdown."* (SIGHUP added per the 2026-09-01 fix's scope.)
**Verdict: 🟢 PASS** — all three signals verified against a live process; each drained gracefully and exited 0.

### Unit tests (4 PASS / 1 SKIP / 0 FAIL, exit 0)

`TestSignalHandling` (context-cancellation shutdown), `TestHTTPTimeouts`, `TestRequestTimeoutAccommodatesBulkChecker`, `TestContextCancellation` all PASS. `TestGracefulShutdownWithActiveConnections` is skipped **by its own first line** (`t.Skip("Skipping network-dependent test in CI environment")`, `internal/server/server_safeguards_test.go:80`) — see F3.

### Live process verification (built binary, loopback ports, fresh process per signal)

Each leg: start server on a free port → confirm `/health` returns HTTP 200 → deliver signal → capture exit code via `wait` → scan logs for the drain trail and for panics.

| Leg | Signal | Condition | Exit code | Drain time | Log trail |
|---|---|---|---|---|---|
| A | SIGTERM | idle, health 200 | **0** | 0.00 s | `signal received: terminated` → `draining connections gracefully` → `server stopped` |
| B | SIGINT | **active connection held open** (incomplete request) | **0** | **4.24 s** | `signal received: interrupt` → `draining connections gracefully` → `server stopped` 4.2 s later |
| C | SIGHUP | health 200, liveness verified pre-signal | **0** — *not* 129 | 0.00 s | `signal received: hangup` → `draining connections gracefully` → `server stopped` |

No panics in any log; no leftover processes.

**LEG B is the load-bearing result.** It directly compensates the suite-skipped drain test (F3): a socket was held with an *incomplete* HTTP request, and `http.Server.Shutdown` demonstrably **waited for that active connection** (which `ReadHeaderTimeout` = 5 s then closed) before finishing — 4.24 s post-signal, inside the 15 s drain budget, versus 0.00 s on idle legs. The drain is real, not instant-return.

### SIGHUP semantics — read before quoting the criterion

`internal/server/server.go:95` registers `SIGINT, SIGTERM, SIGHUP` on one `signal.Notify` channel (the `7bed0a3` fix verified in the 2026-09-01 report). **SIGHUP is handled through the same graceful drain as SIGINT/SIGTERM and exits cleanly with code 0 — it is neither ignored nor abruptly fatal.** What the acceptance criterion rules out — an unhandled default-disposition death (exit 129) or a crash — did not occur: exit 0, drain trail logged, no panic. The 2026-09-01 report's own live-verification expectation is exactly this ("*draining connections gracefully and exits cleanly*"). If product intent is for SIGHUP to **keep the process alive** (the classic daemon-reload semantic), that is a design change against that report, not a safeguard defect — recorded as F4, not fixed.

> **F3 — the drain safeguard has no standing unit coverage.** `TestGracefulShutdownWithActiveConnections` is permanently skipped in the suite (network-dependent skip). Coverage exists only via live process tests (this campaign's LEG B, and the 2026-09-01 report's procedure). A CI-safe `httptest`-based rewrite would close the gap.

> **F4 — SIGHUP terminates the process (gracefully).** "Must NOT kill" is satisfied in the sense that matters for the historical SIGHUP-cascade crash class — no abrupt kill, no panic, clean drain — but the process does exit. This is the shipped design's documented intent, not a regression.

---

## 3. Panic Recovery (step 2)

**Criterion:** *"Test panic recovery: trigger controlled panics and verify recovery."*
**Verdict: 🟢 PASS** — 7/7 (`TestRecover*`, exit 0).

| Test | What it proves |
|---|---|
| `TestRecoverFeedsCrashRecorder` | A panicking handler is recovered; the panic value + full stack land in the structured ERROR log (`panic recovered in request handler`) with request id/method/path, and feed the crash recorder |
| `TestRecoverReturns500AndKeepsServing` | Client gets 500; the server keeps serving subsequent requests |
| `TestRecoverRecordsMetric` | Recovery increments the panic metric; **the crash-loop detector fires at 3 crashes / 5 min** (`crash loop detected crashes_in_window=3 threshold=3`) |
| `TestRecoverPassesThroughCleanHandlers` | Zero overhead / no interference on non-panicking handlers |
| `TestRecoverRepanicsErrAbortHandler` | `http.ErrAbortHandler` is deliberately *not* swallowed (re-panicked, per net/http contract) |
| `TestRecoverAbortsWhenResponseAlreadyStarted` | A panic after headers are written aborts the connection instead of writing a corrupt 500 |
| `TestRecoverAbortsAfterImplicitWrite` | Same protection when the write was implicit (e.g. header snapshot) |

Crash-loop detection — the safeguard that converts a panic storm into a bounded, observable event — is exercised by `TestRecoverRecordsMetric`.

---

## 4. Request Timeout Guards (step 2)

**Criterion:** *"Test timeout guards: verify long-running requests are terminated."*
**Verdict: 🟢 PASS** — 6/6 timeout-middleware tests + the server-timeout invariants.

Configuration under test (`internal/server/safeguards.go:23`): `DefaultRequestTimeout = 30s`, applied by the `Timeout` middleware (`safeguards.go:106`); server socket timeouts sized to it (`internal/server/server.go:43-50`): `ReadTimeout` 15s, `ReadHeaderTimeout` 5s, `WriteTimeout` 45s (comment: must exceed the 30s request timeout), `IdleTimeout` 120s.

| Test | What it proves |
|---|---|
| `TestTimeoutReturns503WhenHandlerOverruns` | A handler exceeding its budget is terminated; client receives **503** |
| `TestTimeoutRecordsMetric` | Timeout events are counted in metrics |
| `TestTimeoutDoesNotAffectFastHandlers` | Fast handlers are unimpeded |
| `TestTimeoutPropagatesDeadlineToHandler` | The deadline reaches the handler's context (so the bulk checker's RDAP calls inherit it) |
| `TestTimeoutDropsLateHandlerWrites` | A handler writing after its deadline cannot corrupt the response |
| `TestTimeoutClientDisconnectIsNotATimeout` | Client disconnects are not misclassified as server timeouts |
| `TestHTTPTimeouts` | The server-level Read/ReadHeader/Write/Idle invariants hold against the 30 s request budget |
| `TestRequestTimeoutAccommodatesBulkChecker` | The bulk checker's own 30 s `TotalTimeout` fits inside the request budget |

Long-running requests are terminated server-side with a clean 503 — no hung goroutine pile-up, no partial-body corruption.

---

## 5. Connection / Concurrency Limit Exhaustion (step 3)

**Criterion:** *"Test connection limits: verify pool exhaustion is handled gracefully."*
**Verdict: 🟢 PASS** — 76 PASS / 0 FAIL across the five recorded run groups (`step3-conn-limits.txt`: [2a] 28, [2b] 2, [3a] 20, [3b] 24, [2c] 2; zero FAIL); exhaustion queues or rejects cleanly, never crashes. *(Note: the step-3 bead notes tally this as "74 PASS" and "18/18" for the ratelimit package; the raw file counts 76 and 24. This report uses the raw file — the notes undercounted, the direction is conservative and no FAIL exists in either reading.)*

### Layered limits under test

| Layer | Limit | Where | Observed behavior at exhaustion |
|---|---|---|---|
| Global bulk concurrency | errgroup cap 50 (`GlobalConcurrency`) | `internal/checker/checker.go:29-46` | `TestCheckBulk_ConcurrencyLimit`: cap 5 with 10 domains → max in-flight ≤ 6 (test slack), all 10 results returned |
| Per-registry concurrency | Verisign 10 · PIR 10 · Google 2 · default 3 (`semaphore.Weighted`) | `checker.go:36-45` | `TestCheckBulk_PerRegistrySemaphore` (env-gated, run with the flag): 30 domains / 15 per registry through the real semaphores — max in-flight stayed within limits and **all 30 succeeded**; excess **queues**, none error |
| Per-registry waiters | Semaphore waiters block | `TestGetRegistrySem` | The (limit+1)-th acquire under a 10 ms timeout fails **cleanly** with a context deadline — blocked, not errored/OOM |
| Registry queue depth | 100 waiting per registry | `internal/ratelimit/ratelimit.go:70` | `TestAcquire_QueueDepthLimit` / `TestQueueDepth_ActiveEntries`: beyond the cap `Acquire` returns a clean error (the plan's `service_busy` path) instead of queueing without bound; cancellation while queued unwinds cleanly |
| Per-IP rate limits | web 10/min · API 60/min · bulk 5/min (burst = limit) | `internal/server/middleware.go:330-334` | see §6 below |

### Bulk correctness at the limits

`TestCheckBulk_MaxDomains` (50-domain max, through the real limiter, 23.6 s) PASS; `TestBenchmark_Bulk50DomainsP99` 99 req / 0 errors / p99 6.35 ms PASS; partial-failure semantics (`TestCheckBulk_PartialResults` — a 429-throwing domain retries with backoff while the rest of the batch completes) PASS.

### Per-IP burst behavior (20/20 PASS, 3.65 s)

`TestRateLimiter_HighRateTriggers429`: **200 simultaneous requests from ONE IP → ≥100 receive 429 and ≤70 succeed** — the token bucket absorbs its small burst allowance and *rejects* the rest (with `Retry-After` + JSON error body, `TestRateLimiter_429Response`), rather than queueing them into memory. Isolation and hygiene all pass: per-IP isolation, per-endpoint-type separation, `RemoteAddr` fallback, token refill, stale-entry eviction, concurrent cleanup/creation races, and `TestIPLimiterMemoryGrowth` (the per-IP map does not grow without bound).

### Registry-facing limiter (`internal/ratelimit/`, 24/24 PASS, 6.83 s)

Per-registry token buckets (Verisign/PIR 10/s burst 10 · Google 1/s burst 2 · default 2/s burst 3) with 429 exponential backoff {1,2,4}s Verisign/PIR, {5,10,20}s Google, {2,4,8}s default, `MaxRetries` 3 (clamped at the last step, `TestAcquire_BackoffBeyondSteps`).

> **F7 — terminology trap for future runs.** `internal/ratelimit` is the **registry-facing** limiter (upstream 429 backoff + queue depth). The **per-IP** limiter lives in `internal/server` (`middleware.go`). The step-3 task text pointed the per-IP burst check at the former; both were run and recorded, so nothing is missing — but the two layers protect different boundaries and should not be conflated in future beads.

> **F8 — the flagship semaphore test silently skips without the env var.** `TestCheckBulk_PerRegistrySemaphore` (and `TestCheckBulk_MaxDomains`, `TestBenchmark_ConcurrentBulkP99`) are gated on `DOMCHECK_RUN_LONG_TESTS=1`. The step's literal `-run 'Semaphore|Concurrency|Bulk'` pattern alone produced SKIPs that would have read as a false pass; the env var was required and used. Any future "-run" verification of these must pair the pattern with the env var.

---

## 6. Sustained Load Stability + p99 (step 3)

**Criterion:** *"Run load tests to verify server stability under sustained load."*
**Verdict: 🟢 PASS** — 0.00 % error across all scenarios; p99 within every configured gate.

| Scenario | Requests | Errors | p50 | p95 | p99 | max | Gate | Verdict |
|---|---|---|---|---|---|---|---|---|
| `TestBenchmark_SustainedLoadP99` (10 s) | 40,960 | 0 (0.00 %) | 1.98 ms | 3.89 ms | **4.83 ms** | 15.9 ms | err ≤ 0.1 %, p99 ≤ 100 ms | PASS |
| `TestBenchmark_SustainedLoadP99_Long` (30 s) | 109,702 | 0 (0.00 %) | 2.18 ms | 21.5 ms | **53.9 ms** | 118 ms | err ≤ 0.1 %, p99 ≤ 100 ms | PASS |
| `TestBenchmark_ConcurrentBulkP99` (5 s) | 1,999 | 0 (0.00 %) | 3.40 ms | 4.81 ms | **5.24 ms** | 5.80 ms | err ≤ 2 %, p99 ≤ 500 ms | PASS |
| `TestBenchmark_Bulk50DomainsP99` | 99 | 0 (0.00 %) | 2.35 ms | 4.98 ms | **6.35 ms** | 6.35 ms | err ≤ 2 %, p99 ≤ 500 ms | PASS |

All runs first-hand with `-count=1`. A prior attempt's (overwritten) files recorded the same 10 s p99 3.80 ms / 30 s p99 7.55 ms / 0 errors — consistent.

> **F5 — the load runner drives ~41× its nominal rate.** `runBenchSustained` (`internal/server/benchmark_regression_test.go:307-311`) computes `concurrency = min(rate, 64)` then `interval := time.Second / concurrency` **per goroutine**, so 64 workers each firing 64×/s offer **4,096 req/s**, not the parameter's 100 req/s. (The adjacent comment describes `1/(rate/concurrency)` per goroutine = 100/s, which the code does not implement; the same formula makes ConcurrentBulkP99's nominal 20 bulk/s an actual ~400 bulk/s.) Consequences, both disclosed rather than fixed here:
> 1. The safeguards were exercised at a **far harsher** load than the plan's "sustained 100 req/s cached" scenario and held — so this is over-coverage, not under-coverage.
> 2. But the **plan document's** targets (cached p99 < 10 ms; sustained 100 req/s p99 < 50 ms) were never literally measured as named. If that scenario is wanted as a contract, fix the runner's interval first.

> **F6 — 30 s p99 (53.9 ms) exceeds the plan doc's 50 ms figure.** Under the ~41× real load of F5 and a busy box (load avg ≈ 4.7), and still inside the test's own 100 ms gate. The prior attempt recorded 7.5 ms on a quieter box. This is load-variance on a shared machine, **not a regression** — but it is the one number in this campaign above a documented plan figure, so it is disclosed rather than smoothed over.

---

## 7. Memory Growth / Leak Verdict (step 4)

**Criterion:** *"Run memory growth tests to verify no leaks (DOMCHECK_RUN_LONG_TESTS=1)."*
**Verdict: 🟢 LEAK-FREE** — all four tests executed to completion, none skipped (16 PASS / 0 FAIL; the only `--- SKIP` string in the file is the summary quoting the pattern itself).

Executed 2026-09-08 04:02–04:16Z, `DOMCHECK_RUN_LONG_TESTS=1`, batch 756 s (12.6 min), `-timeout 20m` added (F11). HeapAlloc figures are post-forced-GC.

| Test | Runtime | Load | HeapAlloc baseline → final | Growth | Sys (OS-reserved) | Goroutines | Half-trend |
|---|---|---|---|---|---|---|---|
| `TestMemoryGrowthUnderLoad` (30 s) | 32.03 s PASS | 1,500 req @ 50/s | 0.39 → 0.57 MB | +0.17 MB (threshold 100 MB) | 12.28 → 17.37 MB | 4 → 2 | −0.43 MB (declining) |
| `TestMemoryGrowthUnderLoadExtended` (2 m) | 122.02 s PASS | 5,950 req @ 50/s, **0 errors** | 0.60 → 0.85 MB | +0.25 MB | 17.37 → 17.37 MB (flat) | 4 → 2 | −0.03 MB (flat) |
| `TestMemoryGrowthUnderLoadFull` (10 m) | 602.02 s PASS | 30,000 req @ 50/s | 0.60 → **0.59 MB** | **−0.00 MB — final below baseline** | 17.37 → 17.37 MB (flat); HeapSys 10.72 → 10.59 MB | 4 → 2 | −0.05 MB; final slope −0.32 MB/interval (declining) |
| `TestIPLimiterMemoryGrowth` | 0.02 s PASS | 50,000 unique-IP limiters | +6,355 KB heap | **130 bytes/entry** (< 500 threshold) | — | — | `cleanup()` leaves **0 entries** (< 100 threshold) |

**Leak basis:** no monotonic growth in any variant; every half-trend ≤ 0; the 10-minute run ends *below* its baseline; Sys does not ratchet upward across variants; goroutines return to baseline; the result cache stays bounded (0 entries against its 10,000 cap); the IP-limiter map is bounded at 130 B/entry and fully evicts. The plan's "memory growth > 100 MB indicates a leak" threshold is missed by three orders of magnitude.

> **F9 — the in-run `ip_limiter_growth` subtest is vacuous.** Inside the three load variants, that subtest's middleware chain (`RequestID → ClientIP → Logging → SecurityHeaders`) never wires the `RateLimiter` middleware in, so `IPLimiterCount` reads 0 throughout despite ~29,950 distinct `client_ip=` values in the run log. It passes for the wrong reason. **Real** limiter-map coverage comes from the direct 50,000-entry `TestIPLimiterMemoryGrowth` (above) plus `TestIPLimiterCleanup`. This is a test-wiring gap, not a product defect — but the subtest as wired proves nothing and should either be fixed to include the middleware or removed.

> **F10 — RSS is not instrumented.** The tests read `runtime.MemStats` (HeapAlloc/HeapSys/Sys), not process RSS. Flat `Sys` at 17.37 MB across the 2 m and 10 m variants is the closest OS-reservation proxy and did not grow. An RSS-sampling pass would make the verdict airtight against allocator retention outside the Go heap.

> **F11 — the "Errors: 50" in the 30 s and 10 m variants are teardown races, not server errors.** They are client-side `client.Do` failures racing `srv.Close()` at teardown; every logged request is status=200 (37,350/37,350; zero non-200), and the 2 m variant recorded 0 errors. Also note the `-timeout 20m` addition: go's default 10 m test timeout would have killed the batch mid-`Full`-variant — `memory_test.go`'s own doc comments prescribe generous timeouts for the Extended/Full variants.

---

## 8. Remaining Edge Cases and Open Risks

### Covered and holding

| Edge case | Where verified | Outcome |
|---|---|---|
| Signal arrives while a connection is mid-request | step 2 LEG B | drains 4.24 s, exit 0 |
| Panic after response headers already written | step 2 (`TestRecoverAbortsWhenResponseAlreadyStarted`) | connection aborted, no corrupt body |
| `http.ErrAbortHandler` internal abort | step 2 (`TestRecoverRepanicsErrAbortHandler`) | passes through per net/http contract |
| Client disconnect vs. server timeout | step 2 (`TestTimeoutClientDisconnectIsNotATimeout`) | not misclassified |
| Handler writes after its deadline | step 2 (`TestTimeoutDropsLateHandlerWrites`) | late writes dropped |
| Bulk batch with a persistently-429 domain | step 3 (`TestCheckBulk_PartialResults`) | retries with backoff; rest of batch completes |
| More bulk domains than a registry's semaphore weight | step 3 (`TestCheckBulk_PerRegistrySemaphore`) | queue → all succeed |
| More waiters than the 100/registry queue cap | step 3 (`TestAcquire_QueueDepthLimit`) | clean `service_busy`-class error |
| Cancellation while queued on a full semaphore | step 3 (`TestAcquire_ContextCanceledDuringSemaphoreWait`) | unwinds cleanly |
| Single-IP flood across endpoint types | step 3 (`TestRateLimiter_HighRateTriggers429`) | 429 + `Retry-After`, not queueing |
| ~30 k distinct client IPs under sustained load | step 4 (30 s / 2 m / 10 m variants) | limiter map bounded, heap flat |

### Open risks — ranked

1. **F1 — `main` is unbuildable from a fresh clone** (open since `e4fcbec`, 2026-09-07; re-confirmed at `0266871`). This is the only finding that blocks *other* work: CI image builds, fresh clones, and any HEAD-extract verification all fail until `resource_monitor{,_test}.go` are committed or the use-sites reverted. Highest-priority follow-up, owned outside this bead chain.
2. **F5/F6 — the sustained-load harness measures a scenario nobody specified.** 4,096 req/s instead of 100. The results are valid (and flattering) but not the plan's named contract; fix the interval formula, then re-baseline p99 against the plan's figures on a quiet box.
3. **F3 — the graceful-drain safeguard has no standing unit test.** Its only coverage is live-process verification. An `httptest`-based rewrite of `TestGracefulShutdownWithActiveConnections` would make it CI-safe and permanent.
4. **F9 — vacuous `ip_limiter_growth` subtest** inside the memory variants (wired without the limiter middleware). Fix the chain or drop the subtest so it cannot be cited as coverage.
5. **F2 — golangci-lint pinned to a go1.24 binary** cannot lint the go1.26 repo at all. The lint layer is dark until the linter is rebuilt.
6. **F4 — SIGHUP terminates (gracefully).** Confirm this is the intended semantic for the deployment environment; if a keep-alive/reload semantic is wanted, it is a deliberate design change.
7. **F10 — no RSS instrumentation** in the memory tests; heap-side verdict only.
8. **Environment caveats, no action:** F7 (ratelimit naming), F8 (env-gated tests silently skip), F11 (teardown-race errors + required `-timeout 20m`), F12 (single-run/shared-box measurements).

### Out of scope here

Process-level and infrastructure crash safeguards (git-gc memory bounds, repository-bloat prevention, crash-alert classification, circuit breaker) are separate layers with their own verification records — see `docs/crash-prevention-requirements.md` (G-1..G-13) and `docs/maintenance/repository-maintenance-guide.md`. This report covers the in-process server/checker safeguards only.

---

## Acceptance-Criteria Scorecard (parent domchk-c13a55b8)

| # | Parent criterion | Result | Evidence |
|---|---|---|---|
| 1 | Test suite, no regressions | 🟢 PASS (working tree) — F1 disclosed | §1 |
| 2 | Load tests, sustained stability | 🟢 PASS — F5/F6 disclosed | §6 |
| 3 | Signal handling SIGTERM/SIGINT graceful shutdown | 🟢 PASS (live, exit 0 ×3) | §2 |
| 4 | Panic recovery | 🟢 PASS 7/7 | §3 |
| 5 | Timeout guards terminate long requests | 🟢 PASS 8/8 relevant | §4 |
| 6 | Connection limits, graceful exhaustion | 🟢 PASS 76/0 | §5 |
| 7 | Memory growth tests, no leaks (`DOMCHECK_RUN_LONG_TESTS=1`) | 🟢 LEAK-FREE | §7 |
| 8 | Document results + remaining edge cases | 🟢 this report | §8, F1–F12 |

---

## Conclusions

**Every in-process crash safeguard works as designed.** Signals drain gracefully on all three catchable termination signals; panics are converted into logged, counted, crash-loop-bounded 500s; long requests are terminated at their 30 s budget with clean 503s; concurrency layers queue and then reject cleanly instead of erroring or crashing; the server sustained ~153 k requests (152,661 across the three sustained/bulk scenarios) at 0.00 % error; and ten-plus minutes of sustained load left the heap *below* its starting size.

**Nothing was silently passed.** Twelve findings are on the record above. None is a safeguard defect; F1 (unbuildable `main`) is the only one that gates other work, and it predates this campaign.

**Recommended next actions** (none blocking this bead): land `resource_monitor{,_test}.go` or revert its use-sites (F1); fix the sustained-load interval formula and re-baseline (F5/F6); restore standing drain coverage via `httptest` (F3); rewire or remove the vacuous limiter subtest (F9); rebuild golangci-lint against go1.26 (F2).

---

## Metadata

| Field | Value |
|---|---|
| Report bead | domchk-ec8c8faf (step 5/5) |
| Parent task | domchk-c13a55b8 |
| Predecessor beads | domchk-6999f0fe, domchk-69b9ebe8, domchk-4dbf80ca, domchk-77c89450 (all Closed) |
| Execution window | 2026-09-08 (steps at 4f5d984 / 76bf33c / 1b27762 / 23d83f3; findings re-verified at 0266871) |
| Raw outputs | `.beads/state/safeguard-verification-2026-09-06/` — step 1: `step1-testsuite.txt`, `step1-golangci-lint.txt`, `step1-head-extract-check.txt`, `step1-summary.md`; step 2: `step2-signal-timeout.txt`, `step2-panic-recovery.txt`, `step2-timeout-mw.txt`, `step2-live-signals.txt`, `step2-summary.md`; step 3: `step3-sustained-load.txt`, `step3-conn-limits.txt`, `step3-summary.md`; step 4: `step4-memory-growth.txt` |
| Committed artifacts | this file only (raw test logs stay gitignored to prevent repository bloat — the bf-1s6c3/bf-4yjq failure mode) |
| Findings disclosed | 12 (F1–F12); 0 safeguard defects |
| Code modified by this verification | none |

## Related Documentation

- [docs/archive/crash-investigations/safeguards-verification-report-2026-09-01.md](archive/crash-investigations/safeguards-verification-report-2026-09-01.md) — **earlier, narrower report: SIGHUP fix only** (bead domchk-b10b5ec9, commit `7bed0a3`); everything else is covered here
- [docs/benchmarks/verification-reports/crash-investigation-verification-2026-09-02.md](benchmarks/verification-reports/crash-investigation-verification-2026-09-02.md) — style/precedent for this report
- `docs/plan/plan.md` — load-testing targets and timeout configuration the results were read against
- `docs/benchmarks/README.md` — the `DOMCHECK_RUN_LONG_TESTS` gating contract
- `docs/crash-prevention-requirements.md`, `docs/maintenance/repository-maintenance-guide.md` — process/infrastructure safeguard layers (separate scope)
