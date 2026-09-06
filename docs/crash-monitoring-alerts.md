# Crash Monitoring and Alert Response

How domain-check records crashes, what it exposes, and what to do when each
alert fires. Alert rules: [`monitoring/alerts-domain-check-crashes.yml`](../monitoring/alerts-domain-check-crashes.yml).
Companion background: [`docs/crash-safeguards-and-monitoring.md`](crash-safeguards-and-monitoring.md).

## What is monitored

| Surface | What it holds |
|---------|---------------|
| `/metrics` | Prometheus counters/histograms, including the crash family (table below) |
| `/health` | Status (`ok` / `degraded` when a crash loop is active), uptime, and `crashes: {total, loop_detected, recent}` — recent events have stacks stripped |
| `/api/v1/crashes?limit=N` | Full crash history, stacks included, newest first (default limit 20, `limit=0` for all retained) |
| `data/crash-dumps/` | On-disk dump ring: `crash-<unixnano>-<kind>.json`, newest `--crash-dump-max` kept (default 10) |

A crash loop degrades `/health` but **keeps it 200** on purpose: restarting a
crash-looping process from a health probe would multiply the problem. The
alert rules own the restart/rollback decision.

## Metrics reference

| Metric | Labels | Meaning |
|--------|--------|---------|
| `domcheck_crash_loop_detected` | — | 1 while ≥ threshold crashes occurred within the window |
| `domcheck_crashes_total` | `kind` | Crash events: `panic`, `shutdown_error`. Signals are deliberately excluded so a deploy SIGTERM cannot trip an alert |
| `domcheck_panics_recovered_total` | — | Recovered request panics (predates the crash family, still incremented) |
| `domcheck_signal_receptions_total` | `signal` | OS signals caught: `SIGINT`, `SIGTERM`, `SIGHUP` |
| `domcheck_signal_handler_executions_total` | `signal`, `outcome` | Completed signal→drain paths; a SIGHUP cascade is visible here as `signal="SIGHUP"` |
| `domcheck_signal_handler_duration_seconds` | `signal` | Signal receipt → end of the graceful shutdown it triggered |
| `domcheck_graceful_shutdowns_total` | `outcome` | `graceful` (drained inside budget), `forced` (drain timed out), `error` (Shutdown failed) |
| `domcheck_shutdown_duration_seconds` | — | Time spent draining connections |

## Alerts and response procedures

### DomcheckCrashLoop

**Severity:** critical · **Expression:** `domcheck_crash_loop_detected == 1`

The process recorded ≥ `--crash-loop-threshold` (default 3) crashes of kind
`panic` or `shutdown_error` within `--crash-loop-window` (default 5m).

1. `GET /api/v1/crashes?limit=10` — read the newest stacks; the panic message
   and request path usually name the failing handler immediately.
2. Cross-check the dump ring: `ls -t data/crash-dumps/ | head` and read the
   newest JSON dumps (full stacks, request IDs, client IPs).
3. Identify the trigger: a single path (one bad handler — mitigate by rolling
   back the last deploy) or many paths (dependency returning garbage — check
   upstream RDAP reachability before blaming the code).
4. Recover by rollback, not restart: a restart clears the in-memory history
   and the loop re-detects only after another `threshold` crashes, so a
   restart masks the evidence. Capture `/api/v1/crashes` output **before**
   touching the process.
5. The gauge self-clears when the events age out of the window, so after a fix
   confirm `domcheck_crash_loop_detected == 0` within ~`window`.

### DomcheckPanicRecovered

**Severity:** warning · **Expression:** any `domcheck_crashes_total{kind="panic"}` increase in 10m

One recovered panic = one client got a 500 and the process survived. Treat
every occurrence as a defect report:

1. Pull the stack from `/api/v1/crashes` (or the newest dump file) and note the
   `method`, `path`, `request_id`, `client_ip`.
2. Reproduce against that path with a similar input; IDN and odd-domain inputs
   are the usual suspects — `go test -fuzz=. -fuzztime=30s ./internal/domain/`
   covers the parser layer.
3. File the fix against the handler named at the top of the stack. No ticket
   needed if the fix is trivial, but the stack must end up in a doc or commit
   message.

### DomcheckPanicStorm

**Severity:** critical · **Expression:** `increase(domcheck_panics_recovered_total[5m]) > 5`

Requests are failing at rate. Shed or roll back first, investigate second —
the stacks stay available in the dump ring after rollback.

### DomcheckShutdownError

**Severity:** critical · **Expression:** any `domcheck_graceful_shutdowns_total{outcome="error"}` increase in 10m

`Shutdown()` failed with something other than the drain timeout; connections
were neither drained nor closed cleanly, and the event counts toward the loop
verdict. Expect this alongside a pod teardown — check `kubectl logs` for the
`server shutdown error` line and whether the process was already wedged
(deadline misses on the listener, fd exhaustion). Two in a row means the
process is not shutting down cleanly and the next deploy may hang.

### DomcheckShutdownForced

**Severity:** warning · **Expression:** `increase(domcheck_graceful_shutdowns_total{outcome="forced"}[30m]) > 2`

The 15s drain budget expired and remaining connections were cut. Ordinary
during a hard stop; a smell when it repeats:

1. Check `domcheck_shutdown_duration_seconds` — is the drain consuming the
   full budget?
2. Correlate with `domcheck_request_duration_seconds` — slow in-flight RDAP
   requests with no upstream deadline are the usual cause. Request timeouts
   (`domcheck_request_timeouts_total`) should be catching these; a rising
   timeout counter with forced shutdowns points at the timeout budget rather
   than the drain budget.

### DomcheckSighupCascade

**Severity:** warning · **Expression:** `increase(domcheck_signal_receptions_total{signal="SIGHUP"}[15m]) > 3`

The recurring fleet failure mode. domain-check handles SIGHUP as a graceful
shutdown and survives a single one; a burst is **infrastructure, not the
application** — needle dispatch scopes, systemd unit reloads, or a terminal
multiplexer teardown. Investigate the host: check the needle worker logs
(`~/.needle/logs/`) for a dispatch wave at the same timestamps before opening
any application-side ticket. Do not roll back a deploy over this alert.

### DomcheckSignalHandlerGap

**Severity:** warning · **Expression:** receptions exceed completed handler runs per signal over 30m

The attribution path itself broke: a signal was received but its handler never
published an execution, so the signal metrics can no longer be trusted. This
is a defect in `internal/server/server.go` (`Run`), not an operational event —
capture logs and treat it as a code bug.

## Configuration

| Flag | Env | Default | Effect |
|------|-----|---------|--------|
| `--crash-dump-dir` | `DOMCHECK_CRASH_DUMP_DIR` | `data/crash-dumps` | Dump directory; empty disables dumps |
| `--crash-dump-max` | `DOMCHECK_CRASH_DUMP_MAX` | `10` | Dumps kept on disk, oldest pruned |
| `--crash-loop-threshold` | `DOMCHECK_CRASH_LOOP_THRESHOLD` | `3` | Crashes in the window that count as a loop |
| `--crash-loop-window` | `DOMCHECK_CRASH_LOOP_WINDOW` | `5m` | Crash-loop detection window |

In-memory history is capped at 50 events (stacks at 16 KiB each) so a panic
storm cannot grow the process that is reporting on itself.

## Wiring the rules

The rules file is loaded by Prometheus via `rule_files` and the scrape target
is configured on the cluster side. Per repo policy, that wiring lives in
**jedarden/declarative-config** (`k8s/apexalgo-iad/domain-check/`) — do not
add manifests to this repo.
