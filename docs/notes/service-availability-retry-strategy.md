# Service Availability, Failure Modes and Retry Strategy

How domain-check handles failures of its external dependencies — RDAP registries,
the IANA bootstrap service and the inference gateway — and what an operator does
when one of them is down.

Every value below was verified against the implemented code (`internal/resilience/`,
`internal/rdap/rdap.go`, `internal/bootstrap/bootstrap.go`, `internal/cli/healthcheck.go`,
`cmd/domain-check/main.go`), not against the plan. If this document and the code
disagree, the code wins and this document needs updating.

The three building blocks live in `internal/resilience`:

- `retry.go` — exponential-backoff retry loop (`resilience.Do`)
- `errors.go` — transient/permanent error classification
- `circuitbreaker.go` — per-dependency circuit breakers (`CircuitBreaker`, `BreakerSet`)

They compose: an open breaker fails before any network I/O, so it never consumes
retry budget; a retry loop retries only transient errors.

## Retry policy (`internal/resilience/retry.go`)

The wait before retry *N* (1-based) is `BaseDelay * 2^(N-1)`, capped at `MaxDelay`.
`Delay()` doubles in a loop rather than by bit-shift so a large attempt number
saturates at the cap instead of overflowing `time.Duration`.

### Standard policy — `DefaultRetryConfig`

| Field | Value |
|---|---|
| `MaxRetries` | `DefaultMaxRetries` = **5** (retries after the initial attempt → 6 attempts total) |
| `BaseDelay` | `DefaultBaseDelay` = **1s** |
| `MaxDelay` | `DefaultMaxDelay` = **32s** (cap on a single wait) |
| `MaxElapsedTime` | `DefaultMaxElapsedTime` = **0** — no cumulative budget; the caller's context deadline is the only wall-clock bound |

The five waits are **1s, 2s, 4s, 8s, 16s** — a cumulative **31s** of backoff on
top of six attempts.

Used by the `healthcheck` CLI subcommand (the only production caller), where
there is no server request timeout to fit inside.

### Interactive policy — `DefaultInteractiveRetryConfig`

Same base delay and retry count, but tightened for request paths that must answer
inside the server's `DefaultRequestTimeout` of **30s** (`internal/server/safeguards.go`):

| Field | Value |
|---|---|
| `MaxDelay` | **8s** |
| `MaxElapsedTime` | **10s** cumulative backoff budget |

Before each wait, `Do` stops the loop if `elapsed + nextDelay` would exceed the
budget. The waits actually taken are 1s, 2s and 4s (7s cumulative); a 4th retry's
8s wait would reach 15s, so the loop stops after **at most 4 attempts** (1 initial
+ 3 retries), leaving the rest of the 30s request timeout for the request itself.
Retries never widen the caller's deadline — a context cancellation cuts the loop
off mid-backoff.

Used by the RDAP client (`rdap.go`) and the bootstrap fetcher (`bootstrap.go`).

### Loop exit conditions (`resilience.Do`)

`fn` is called with the 1-based attempt number. The loop stops on: success,
a **permanent** error (returned unwrapped, immediately), context cancellation
(the context error is returned), retries exhausted, or the elapsed-time budget.
When every attempt failed with a transient error the result is
`*AttemptsExhaustedError`, which reports the attempt count and the last underlying
error — and is itself classified **transient**, because whatever failed may still
clear. Treat it as an outage, not a client mistake.

## Error classification (`internal/resilience/errors.go`)

Errors carry a class via the `ErrTransient` / `ErrPermanent` sentinels
(`WrapTransient` / `WrapPermanent`, tested with `IsTransient` / `IsPermanent`).

`Classify(err)`:

- `ErrTransient` → transient
- `ErrPermanent`, `ErrCircuitOpen`, `context.Canceled`, `context.DeadlineExceeded` → permanent
- anything else → `ClassUnknown` — `Do` treats unknown as **transient** (a dropped
  connection or refused dial is usually a blip)

`TransientHTTPStatuses` — the status codes worth retrying:

| Status | Reason recorded |
|---|---|
| 408 | request timeout |
| 429 | rate limited |
| 502 | bad gateway |
| 503 | service unavailable |
| 504 | gateway timeout |

**500 is deliberately absent**: registries return it for malformed queries, so the
same answer would come back on every retry and only burn the request budget.
`StatusClass()` maps 2xx → not an error, the five codes above → transient, every
other non-2xx → permanent.

## Circuit breaker (`internal/resilience/circuitbreaker.go`)

State machine:

```
closed --- 5 consecutive failures ---> open
open   --- 30s elapses ---> half-open
half-open --- 2 consecutive successes ---> closed
half-open --- any failure ---> open (for another 30s)
```

Defaults: `DefaultFailureThreshold` = **5**, `DefaultOpenDuration` = **30s**,
`DefaultSuccessThreshold` = **2**. Five failures is one full standard retry
cycle's worth.

Semantics that matter operationally:

- **Fail fast.** While open, `Allow`/`Execute` return an error wrapping
  `ErrCircuitOpen` — message `circuit open: dependency is unavailable: <name>
  (retry after <remaining>)` — *without calling the dependency*. No network I/O,
  no wasted request budget.
- **No retry burn.** Callers in the RDAP/bootstrap paths wrap `ErrCircuitOpen`
  in `WrapPermanent` before returning it to `Do`, so the retry loop aborts
  immediately instead of backing off five times against a breaker that will not
  change its mind.
- **One probe at a time in half-open.** A recovering service is probed by a
  single request, not hit with the whole queued backlog.
- **Successes heal.** Any success in the closed state resets the consecutive
  failure counter; two consecutive successes in half-open close the circuit.
- **`BreakerSet`** hands out one breaker per dependency name — the registry
  hostname for RDAP, the bootstrap host for the bootstrap fetch — created on
  first use with a shared config. A Verisign outage therefore cannot trip the
  Google Registry's circuit. `States()` snapshots every breaker for diagnostics.
- **What counts as a failure.** In `rdap.doRequest`, a transport error and a
  transient status (408/429/502/503/504) each record a failure; a definitive
  registry answer — 200 registered, 404 available, 400 malformed — records a
  *success*. An "available" verdict can never trip a breaker.
- **Reset.** `CircuitBreaker.Reset()` closes the circuit and clears counters, for
  an operator who has fixed the dependency and does not want to wait out the open
  period. There is no CLI flag for it — state is in-process memory, so a server
  restart also clears it. Normally nothing needs doing: the breaker self-heals
  through the half-open probe.

## Where it is wired

| Call path | Retry policy | Breaker |
|---|---|---|
| RDAP registry query (`internal/rdap/rdap.go` `doRequest`) | `DefaultInteractiveRetryConfig` (8s cap, 10s budget) | one per registry hostname, via `BreakerSet` |
| IANA bootstrap fetch (`internal/bootstrap/bootstrap.go` `fetch`) | `DefaultInteractiveRetryConfig` | one per bootstrap host |
| `healthcheck` subcommand (`internal/cli/healthcheck.go`) | `DefaultRetryConfig` with `MaxRetries` from `--retries` | none (it is a probe, not a dependency call) |
| `serve` startup pre-flight (`cmd/domain-check/main.go`) | none — a single `CheckHealth` attempt, logged; startup must not wait on a dependency that is already down | none |

## Failure-mode table

| Failure | Detected as | Class | Retried? | Caller-visible outcome |
|---|---|---|---|---|
| Transient registry status | 408 / 429 / 502 / 503 / 504 | transient | yes — exponential backoff | retried in-process; if it persists, see the next rows |
| Transport failure | connection refused/reset, DNS, TLS error | unknown → treated transient | yes | retried; each attempt counts toward the breaker |
| Rate-limit exhaustion | still `HTTP 429` after the budget, or the registry queue depth limit hit (`ratelimit.ErrServiceBusy`) | transient | budget exhausted | per-domain result with error `RDAP rate limited` |
| Registry outage (breaker open) | `ErrCircuitOpen` from the registry's breaker | permanent wrapper, fail fast | no | immediate per-domain result with error `RDAP registry temporarily unavailable: …` (`rdap.ErrRegistryUnavailable`) |
| Attempts exhausted, breaker still closed | `AttemptsExhaustedError` | transient | no attempts left | same `RDAP registry temporarily unavailable` result |
| Definitive registry answer | 200 registered, 404 available, 400 malformed | not an error / permanent | no | normal availability answer; records a breaker success |
| Registry 500 | status outside `TransientHTTPStatuses` | permanent | no | returned on the first attempt; never trips the breaker |
| Invalid input / allowlist rejection | validation error, URL not in the RDAP allowlist, unparsable URL | permanent | no | `400 invalid_domain` / `unsupported_tld` / allowlist error |
| Caller timeout or cancellation | context deadline (30s request timeout) or cancel | permanent | no | propagated: the API layer answers `504 upstream_timeout` |

Graceful degradation: an outage never takes the server down. A single check
returns HTTP 200 with the per-domain `error` field set; bulk and multi-TLD
requests return partial results with `succeeded`/`failed` counts. Only a request
that outlives its own context returns a non-200 (`504`).

## Recovery procedures (operator)

**1. Detect.**

```bash
# Is the dependency up? Exit code says transient vs permanent (see below).
./domain-check healthcheck
# Probe something specific:
./domain-check healthcheck --url https://rdap.verisign.com/com/v1/ --timeout 3s --retries 2
```

Corroborating signals: the server's startup log line
(`pre-flight health check passed/failed`, one single attempt at boot), the
`registry_unavailable` status on `domcheck_rdap_requests_total{registry,status}`
at `/metrics`, and `/health` (see bootstrap degradation below).

**2. During an outage, do nothing to the server.** It keeps answering; the
affected registries produce per-domain errors instead of hanging. Restarting the
process does not fix a remote outage — it only clears in-memory breaker state.

**3. Let the breaker reset itself.** After 30s open, one probe goes through; two
consecutive successes close the circuit. If the breaker keeps re-opening, the
dependency is still failing — that is the signal, not a bug in domain-check.

**4. Fix the dependency, not the retry logic.** The documented shell probe for
the inference gateway is `curl -skf https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health`
— note `-k`: the gateway serves a self-signed certificate, and plain `-sf`
fails with curl 60 while the gateway is actually fine. `domain-check healthcheck`
already tolerates the self-signed certificate for its default target.

**5. Bootstrap degradation.** If the IANA bootstrap fetch
(`https://data.iana.org/rdap/dns.json`) fails, the manager installs hardcoded
fallback servers for `.com`, `.net` and `.org` only; other TLDs answer
`unsupported_tld` until the bootstrap recovers. Recovery is the background
`refreshLoop`, which re-fetches every **24h** (`defaultRefreshInterval`) and
keeps serving the previous map on failure — a failed refresh does not re-install
fallbacks mid-flight, so a server that fetched successfully once keeps its full
TLD map while stale. `/health` reports `degraded` (still HTTP 200) once the
bootstrap is older than **48h**, and `unhealthy` with **503** past **7 days**.
Because `Updated()` advances only on a *successful* refresh, a server running on
startup fallbacks reports a bootstrap age measured from the zero time — `/health`
reads `unhealthy` even though `.com`/`.net`/`.org` lookups keep working.

**6. Nothing to flush.** There is no retry/backoff state that needs clearing
after an incident — caches expire on their own TTLs (5min available, 1h
registered) and breakers self-heal.

### `healthcheck` exit codes

| Exit | Meaning |
|---|---|
| `0` | every target answered as expected (a target that recovered only after retries also exits 0, reported as `DEGRADED`) |
| `1` | at least one target is unavailable **transiently** — worth retrying later (unclassified failures count as transient, matching the retry loop's own default) |
| `2` | at least one target is unavailable **permanently** — retrying will not help, investigate |
| `3` | usage or configuration error (bad URL, no URLs) |

With several `--url` targets the exit code is the worst outcome across them
(permanent outranks transient outranks OK). Flags: `--url` (repeatable and
comma-separated; default is the inference gateway health endpoint), `--timeout`
(per attempt, default `5s`), `--retries` (default `5`, `0` disables retrying).
Worst-case wall time with defaults is six 5s attempts plus 31s of backoff ≈ 61s
— scripts should allow for that before declaring the probe itself hung.

## Cross-references

- `docs/plan/plan.md` — Graceful Degradation table and Rate Limiting Strategy
  (the per-registry rate limiters that sit in front of these retries)
- `CLAUDE.md` — "Retry Strategy for Transient Failures" (operator shell sketch)
  and "Service Availability Checks" (gateway probe)
- `internal/resilience/` — `retry.go`, `errors.go`, `circuitbreaker.go`,
  `healthcheck.go`; tests alongside each
