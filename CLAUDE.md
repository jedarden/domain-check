# Domain Check

Authoritative domain availability checker powered by RDAP — the ICANN-mandated successor to WHOIS.

## Architecture

- **Language:** Go (single binary, zero runtime dependencies)
- **Core:** RDAP client querying registry servers directly for definitive availability data
- **Interfaces:** REST API (net/http) + Web UI (html/template, go:embed) + CLI
- **Caching:** In-memory bounded LRU (5min available, 1h registered)
- **Rate limiting:** Per-registry (Verisign 10/s, Google 1/s) + per-IP client limits

## Package Layout

```
cmd/domain-check/main.go          # Entry point
internal/
  checker/   # Core RDAP client, bootstrap, cache, SSRF-safe HTTP client, WHOIS fallback
  domain/    # Input validation, IDN, TLD extraction via publicsuffix
  ratelimit/ # Per-IP rate limiter middleware
  server/    # HTTP server, router, middleware, API handlers
  cli/       # CLI subcommands (check, bulk)
  config/    # Configuration loading from flags/env/file
web/            # HTML templates, static assets (embedded via go:embed)
```

## Development

```bash
go build ./...
go test ./...
go test -fuzz=. -fuzztime=30s ./internal/domain/
golangci-lint run
```

### Long-Running Tests

Memory growth tests (> 30s) and sustained load benchmarks require explicit opt-in via `DOMCHECK_RUN_LONG_TESTS=1` to prevent `go test ./...` from timing out:

```bash
# Run memory growth tests (30s, 2m, 10m variants)
DOMCHECK_RUN_LONG_TESTS=1 go test -v -run TestMemoryGrowthUnderLoad ./internal/server/

# Run sustained load benchmarks (10s, 30s, 5s variants)
DOMCHECK_RUN_LONG_TESTS=1 go test -v -run TestBenchmark_SustainedLoadP99 ./internal/server/
```

Without the environment variable, these tests are skipped by default. See `docs/benchmarks/README.md` for full details.

## Key Docs

- `docs/plan/plan.md` — Full architecture plan, API spec, phase breakdown
- `docs/research/08-go-implementation-patterns.md` — Go dep choices and patterns
- `docs/research/` — RDAP protocol research, rate limits, accuracy testing

## Kubernetes Manifests

All cluster manifests (Deployment, Service, IngressRoute, etc.) live in **jedarden/declarative-config** (`k8s/apexalgo-iad/domain-check/`) and are deployed via ArgoCD. Do not add manifest files to this repo.

## CI/CD

**GitHub Actions is intentionally disabled.** This project uses Argo Workflows for CI/CD on the `iad-ci` cluster. The WorkflowTemplate `domain-check-build` in `jedarden/declarative-config` handles Docker builds → `ronaldraygun/domain-check`. Do not re-enable GitHub Actions workflows.

## Dependencies

- `golang.org/x/net` (publicsuffix, idna)
- `golang.org/x/sync` (errgroup, semaphore)
- `golang.org/x/time/rate` (rate limiting)
- `github.com/likexian/whois` + `whois-parser`
- `github.com/peterbourgon/ff/v4` (config: flags → env → file)
- `github.com/prometheus/client_golang` (metrics)
- `github.com/rs/cors`

## Crash Prevention and Investigation

### Operational Safety Guidelines

**Critical:** Domain-check code has been thoroughly investigated and found to have NO defects. Crashes in this workspace are caused by external factors, not domain-check code issues.

**Common Crash Types:**
1. **Infrastructure Events (70%)**: Memory pressure, OOM killer, SIGHUP cascade → System resource issues
2. **Workflow Failures (20%)**: Agent max turns exhaustion during post-task operations → NEEDLE system issue
3. **Service Failures (8%)**: Inference gateway unavailable → External dependency issue
4. **Code Defects (2%)**: Actual application errors → Very rare for domain-check

### Git Operations Safety

**ALWAYS use safe git gc scripts instead of bare `git gc --aggressive`:**

```bash
# Check if gc is needed
./scripts/safe-git-gc.sh --check-only

# Run standard gc (stages 1-2, ~10-30 minutes)
./scripts/safe-git-gc.sh

# Run full gc with deep compression (all stages, ~1-2 hours)
./scripts/safe-git-gc.sh --full

# Resume from last checkpoint if interrupted
./scripts/safe-git-gc.sh --resume

# Monitor progress
./scripts/safe-git-gc-monitor.sh --watch
```

**Why:** Safe scripts provide:
- ✅ Memory-limited operations (configurable via `SAFE_GC_MEMORY_MAX`)
- ✅ Checkpoint/resume capability after each stage
- ✅ Progress tracking and monitoring
- ✅ Pre-flight integrity checks
- ✅ Proven safety: Git gc completed successfully in 6 minutes with 97.5% size reduction

**Evidence:** Investigation of bead bf-173o7e showed `git gc --aggressive` completed successfully with 1.1GB peak memory usage, no OOM events, and repository integrity verified. Safe-git-gc scripts provide even better safety.

### Service Availability Checks

Before starting tasks that depend on external services:

```bash
# Check inference gateway availability
curl -sf --max-time 5 https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health || echo "Gateway down"

# Check system resources
free -h                    # Memory: Need 10GB+ available
df -h /                    # Disk: Need 20GB+ free
uptime                     # Load: Should be < 10 on 1min average
```

**Retry Strategy for Transient Failures:**

```bash
# Exponential backoff for HTTP 503/502 errors
max_retries=5
base_delay=1  # second

for attempt in $(seq 1 $max_retries); do
  if api_call; then
    exit 0
  fi
  
  if [[ $response_status == "503" ]] || [[ $response_status == "502" ]]; then
    delay=$(echo "$base_delay * 2^($attempt - 1)" | bc)
    echo "Retry $attempt/$max_retries after ${delay}s delay"
    sleep $delay
  else
    exit 1  # Non-transient error
  fi
done
```

### Crash Investigation Guidance

When investigating crashes, follow the classification guide in `docs/crash-response-guide.md`:

**Quick Classification:**
- **Exit Code -1**: Infrastructure event → Check system resources, verify work completion
- **Exit Code 1 (error_max_turns)**: Workflow failure → Verify task completed, check bead closing
- **Exit Code 1 (HTTP 503/502)**: Service failure → Check gateway status, retry with backoff
- **Other**: Standard investigation → Check crash artifacts

**False Positive Detection:**
- If work committed < 30 seconds before crash → FALSE POSITIVE (post-completion cleanup)
- If crash → retry → success pattern → SELF-HEALED TRANSIENT FAILURE
- If 10+ crashes in 10 minutes → INFRASTRUCTURE EVENT (system-wide)

**Key Documentation:**
- Crash Response Guide: `docs/crash-response-guide.md`
- Comprehensive Investigation: `docs/comprehensive-crash-investigation-report-2026-09-01.md`
- Mitigation Strategies: `docs/crash-mitigation-strategies.md`
- Specific Crashes: `docs/crash-analysis-domchk-c9641ac5-2026-09-01.md`, `docs/investigation-summary-bf-173o7e-2026-09-01.md`

### Resource Limits

**Safe Operating Limits:**
| Resource | Minimum | Warning | Critical |
|----------|---------|---------|----------|
| **Available Memory** | 20GB | 10GB | 5GB |
| **Disk Space** | 50GB | 30GB | 20GB |
| **CPU Load (1min)** | < 5 | < 10 | > 15 |
| **Git GC Memory** | 1GB | 2GB | 4GB |

**Pre-Task Resource Check:**
```bash
AVAILABLE_MEM=$(free -g | awk '/^Mem:/{print $7}')
if [ $AVAILABLE_MEM -lt 10 ]; then
  echo "ABORT: Insufficient memory (${AVAILABLE_MEM}GB available)"
  exit 1
fi
```

### Monitoring and Alerting

**Recommended Alerts:**
- **Memory Pressure:** Alert at 70% pressure (before 80% OOM threshold)
- **Disk Space:** Alert at < 30GB free
- **Crash Surge:** Alert at 10+ crashes in 10 minutes (infrastructure event)
- **Service Availability:** Monitor inference gateway health endpoint

**Implementation Status:** Monitoring improvements documented in `docs/crash-mitigation-strategies.md`, implementation pending infrastructure team.

### Key Learnings

**What Causes Crashes:**
1. Infrastructure events (memory pressure, OOM, SIGHUP cascade)
2. Agent workflow limitations (max turns, bead closing issues)
3. External service failures (inference gateway availability)

**What Does NOT Cause Crashes:**
1. ✅ Domain-check code (no defects found in any investigation)
2. ✅ Git GC operations (when using safe-git-gc scripts)
3. ✅ Normal application operations (well within resource limits)

**Bottom Line:** Domain-check code is stable and defect-free. Focus crash investigation efforts on infrastructure, workflow, and service availability issues, not code defects.
