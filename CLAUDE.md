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

### Current Repository Health (verified live 2026-09-06)

The repository is **repaired and healthy**. The ~18GB loose-object bloat that
caused the memcg-OOM crashes (bf-1s6c3, bf-4yjq, 2026-08-12) was packed down
and cannot recur through `.beads/` — the whole directory is gitignored, 0
tracked files.

- **`.git`:** 94 MB (was ~18 GB); 169 loose objects / 1.27 MiB (normal churn,
  packed by the daily 03:00 gc), one consolidated pack (90.43 MiB), 0 garbage
- **Integrity:** `git fsck --full` clean; `./scripts/check-repo-health.sh` passes.
  **`--full` is the integrity gate on this box:** `git fsck --no-full` on a
  packed repo (git 2.50.1) exits 2 with ~1,008 false `invalid reflog entry`
  errors — flag noise on packed repos, not corruption (every flagged OID
  verifies; an unrelated healthy repo fails identically). Never "repair" the
  reflog (`reflog expire`/`delete`) in response — that destroys real history.
  Evidence: [bf-4x12ec report, fsck caveat](docs/crash-reports/bf-4x12ec-git-gc-crash.md)
- **Effective gc bounds:** `./scripts/setup-git-gc-config.sh --verify` → ✅
  (worst case ≈3GiB per pack run, within the 12GiB dispatch scope)
- **Verification record:** [bf-4yjq cleanup verification](docs/crashes/bf-4yjq-cleanup-verification.md) —
  re-run 2026-09-06, five days after the original cleanup, and holding

Everything below is the standing procedure that keeps it that way.

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
- ✅ Memory-limited operations — soft `SAFE_GC_MEMORY_MAX` (pack.windowMemory) under a hard `SAFE_GC_CGROUP_MAX` ceiling, with a `ulimit -v` fallback
- ✅ Fail-fast resource validation: invalid config or insufficient memory/disk/load exits **2** before any git work (`./scripts/safe-git-gc.sh --check-only` reports the same verdict without touching anything)
- ✅ Checkpoint/resume capability after each stage
- ✅ Progress tracking and monitoring
- ✅ Pre-flight integrity checks
- ✅ Proven safety: this repo's verified cleanups (18GB → 92MB on 2026-09-01, re-verified at 93MB on 2026-09-06) ran under these bounds — `docs/crashes/bf-173o7e-cleanup-verification.md`

Tested by `scripts/test-safe-git-gc-limits.sh` (33 assertions, seconds to run; the disruptive real-gc cases gate on `DOMCHECK_RUN_LONG_TESTS=1`). Details: [safe-git-gc.sh Run-Time Safeguards](docs/maintenance/repository-maintenance-guide.md).

**Evidence (corrected 2026-09-06):** The early investigation docs (`docs/crash-investigation-bf-4x12ec.md`, `docs/investigation-summary-bf-173o7e-2026-09-01.md`) recorded the Aug-14 gc deaths as "gc completed successfully, ~1.1GB peak, no OOM events" — that conclusion is **superseded**. Those runs were memcg-OOM SIGKILLs at the dispatch scope's 12GiB bound (the mechanical guard below exists because of them); the kernel records proving the mechanism were only recovered later, for the push-side variant bf-198ne. What *is* proven: bounded runs complete cleanly, and this repo now sits at ~94MB after the verified 18GB cleanup. Do not run bare `git gc --aggressive` on the strength of the older docs.

**Mechanical guard for the bare-gc path (2026-09-02):** safe-git-gc.sh bounds only its own invocations — the Aug-14 crash storm happened when an agent ran bare `git gc --aggressive --prune=now`, whose pack-objects RSS exceeded the 12GiB `MemoryMax` of needle's dispatch scope (memcg OOM SIGKILL → exit code -1, 129 attempts). That path is now defended by persistent git config, not convention: `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` (the window limit is per-thread, so threads must be pinned) → worst case ≈3GiB per pack run. Applied repo-locally **and** globally (`./scripts/setup-git-gc-config.sh --global`). Check anytime with `./scripts/setup-git-gc-config.sh --verify` — it resolves the **effective** bound (system → global → local, the chain a bare gc actually sees) and reports which scope supplies each key, so a repo protected only by the box-wide global config verifies clean; exit 1 = no effective bound or threads unpinned. Tests in `scripts/test-gc-memory-bounds.sh` rerun both memcg-OOM death commands under a 768MiB cgroup: the bare `git gc --aggressive --prune=now` (peak pack-objects RSS ≈ 313MiB) and bf-1ea4g's death operation, a bounded `git push` over an unpacked multi-snapshot backlog (peak push RSS ≈ 227MiB vs the >12GiB its unbounded push consumed). The same bound covers `git push`'s pack-objects too: **bf-198ne (2026-08-16) was the push-side variant of the same memcg OOM** (720-commit backlog still carrying 5.6GB of retired bead-forge state), re-verified resolved 2026-09-06 — `docs/crashes/bf-198ne-crash-report.md`. See `docs/maintenance/repository-maintenance-guide.md`.

### Service Availability Checks

Before starting tasks that depend on external services:

```bash
# Check inference gateway availability
# NOTE: `-k` is required — the gateway serves a self-signed cert, so plain `-sf`
# always fails with curl 60 ("Gateway down") while the gateway is actually fine.
curl -skf --max-time 5 https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health || echo "Gateway down"

# Check system resources
free -h                    # Memory: Need 10GB+ available
df -h /                    # Disk: Need 20GB+ free
uptime                     # Load: Should be < 10 on 1min average
```

**Retry Strategy for Transient Failures:**

> The shell sketch below is the operator-side convention. domain-check implements
> this in code — exponential backoff, circuit breakers, transient/permanent error
> classes and the `healthcheck` subcommand — see
> [docs/notes/service-availability-retry-strategy.md](docs/notes/service-availability-retry-strategy.md)
> for the implemented policy and operator recovery procedures.

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

### Prevention Validation and Per-Alert-Type Runbooks (2026-09-08)

**A prevention claim is only as current as its last validation.** Before citing any
"prevention in force" statement in a report or bead note, re-run the applicable layer from
[docs/crash-prevention-validation.md](docs/crash-prevention-validation.md) — per-layer
procedures (repo/gc bounds → monitoring timers → alert pipeline → surge/dispatch gates →
integration wiring), the one-command battery, failure-attribution rules for this shared
worktree, and the dated verification record. Full battery all-green 2026-09-08
(domchk-82c1ff9a): 13 live checks + 19 tracked suites, 0 failures.

```bash
# Smallest useful re-check (read-only, ~1 min)
./scripts/check-repo-health.sh && ./scripts/setup-git-gc-config.sh --verify \
  && ./scripts/preflight-health-check.sh \
  && ./scripts/system-event-mode.sh check && ./scripts/crash-circuit-breaker.sh status
```

**Responding to an alert:** `docs/crash-response-guide.md` → "Operational Runbooks by
Alert Type" — one runbook per class (A: crash/exit −1, B: max-turns, C: service 503/502,
D: resource, E: repo-health/bloat, F: crash surge, G: breaker/deferral), each with the
commands, close criteria, and escalation threshold. Start every crash alert with the
target-resolution + dedup gate, not the investigation: most alerts point at work another
worker already finished. Only **tracked** suites are canon when validating
(26 of 52 on-disk `scripts/test-*.sh` at 2026-09-08 — the rest are in-flight sibling work).

### Crash Investigation Guidance

When investigating crashes, follow the classification guide in `docs/crash-response-guide.md`:

**Quick Classification:**
- **Exit Code -1**: Infrastructure event → Check system resources, verify work completion
- **Exit Code 1 (error_max_turns)**: Workflow failure → Verify task completed, check bead closing
- **Exit Code 1 (HTTP 503/502)**: Service failure → Check gateway status, retry with backoff
- **Other**: Standard investigation → Check crash artifacts

**False Positive Detection:**
- Verify the target bead's actual state **before** investigating: a large share of crash alerts target beads another worker already closed or completed. Read the bead's own deliverable and `git log --grep <bead-id>` first — near-identical artifact *titles* across beads are the main false-positive source (a report can match a title word-for-word and belong to a different bead; check its Related Bead field)
- If work committed < 30 seconds before crash → FALSE POSITIVE (post-completion cleanup)
- If crash → retry → success pattern → SELF-HEALED TRANSIENT FAILURE
- If 10+ crashes in 10 minutes → INFRASTRUCTURE EVENT (system-wide)

**Key Documentation:**
- Crash Response Guide: `docs/crash-response-guide.md`
- Comprehensive Investigation: `docs/comprehensive-crash-investigation-report-2026-09-01.md`
- Mitigation Strategies: `docs/crash-mitigation-strategies.md`
- Specific Crashes: `docs/crash-analysis-domchk-c9641ac5-2026-09-01.md`, `docs/investigation-summary-bf-173o7e-2026-09-01.md`

**Pre-Close Work Verification:** Run `./scripts/verify-work-completion.sh <bead-id> --summary "..."` before `bead close` — it fails if commits are unpushed or expected artifacts are missing, and writes `.beads/state/work-completion/<bead-id>.json` so crash triage can tell post-completion crashes from mid-task ones. Usage: `scripts/README.md`.

### Crash Alert System (2026-09-02 Implementation)

**Implemented Fixes:** Comprehensive crash alert system improvements prevent false positives and duplicate alerts (all six critical fixes, grouped by mechanism):

1. **Closed bead filtering** (fixes 1 & 5): checks if target bead is CLOSED before creating alerts (prevents false positives like bf-3561g investigating completed bead bf-4k2ws). For **ALERT beads** (`$BEAD_ID` is the alert bead itself, Open; the crash target is a different bead named in its title) the FIX 1 target-closure gate also consults the *target's* status ahead of classification and the breaker record, failing open when that status is unreadable — before domchk-cd8ec29e (2026-09-08) suppression for that shape came only from `alert-deduplication.sh`'s later target-resolution leg, so an Open ALERT bead against a Closed target generated an alert whenever that downstream gate was absent (the bf-29rca shape). Regression coverage: `scripts/test-closed-bead-filter.sh` phase 2, scenarios A–D
2. **Duplicate detection + processed-alerts tracking** (fixes 2 & 3): prevents multiple investigation beads for the same crash event
3. **Completion awareness + exit-code validation** (fixes 4 & 6): detects post-completion cleanup termination vs. crash during task
4. **Alert cooldown:** 5-minute cooldown prevents alert spam during system-wide events
5. **Crash classification:** accurate categorization (FALSE_POSITIVE, SERVICE_FAILURE, INFRASTRUCTURE, CODE_DEFECT)

**Scripts:**
- `scripts/crash-alert-manager.sh` - Main alert processing with all 6 critical fixes
- `scripts/crash-classifier.sh` - Crash categorization
- `scripts/alert-deduplication.sh` - Duplicate detection
- `scripts/test-crash-alert-fixes.sh` - Test suite (re-verified 2026-09-07: 13/13 passing; the 2026-09-06 record said 12/12 — the suite grows as checks are added)
- `scripts/test-closed-bead-filter.sh` - Functional closed-bead filter test: runs a fabricated trace for closed bead bf-2vtzg through crash-alert-manager.sh in a sandbox and asserts no alert is generated (7 assertions; `test-crash-alert-fixes.sh` only checks the FIX 1/5 markers are present)

**Usage:**
```bash
# Process a crash alert
./scripts/crash-alert-manager.sh <bead-id>

# Auto-process recent crashes
./scripts/crash-alert-manager.sh --auto-process

# Classify a crash
./scripts/crash-classifier.sh <bead-id>

# Test alert fixes
./scripts/test-crash-alert-fixes.sh
```

**Documentation:** `docs/crash-alert-fix-implementation-2026-09-02.md`

**Verified working (2026-09-07, domchk-81938e89 — the bf-5npjj closure leg):**
- Suites: `test-crash-alert-fixes.sh` 13/13, `test-closed-bead-filter.sh` 7/7, both exit 0.
- Cascade replay: a sandboxed replay of the bf-6d3d6 shape — six genuine exit -1 kills of one
  bead, each packaged as its own alert bead, compressed into one minute — produced exactly
  **one** alert. The 300s cooldown absorbed the other five; FIX 3 stopped the immediate re-run
  ("Already processed this alert bead"); the surge detector fired `INFRASTRUCTURE EVENT
  DETECTED`; zero bead mutations (the replay stubbed `bead`, so nothing touched the live store).
- Health layer: `check-repo-health.sh` clean (105MB repo, effective pack-memory bound ≈3072MiB
  worst case within the 6GiB ceiling, 0 unpushed backlog), `auto-gc-trigger.sh --dry-run`
  → "GC not needed", `safe-git-gc.sh --check-only` passes preflight (its exit 1 there is the
  "GC not needed" *verdict*, not a failure), and all 8 `domain-check-*` timers had future
  trigger times.
- Re-executed first-hand against HEAD c04f017 by this bead's closing attempt (2026-09-07
  18:37Z): replay 10/10 assertions — the extra one proves the cooldown is a *window*, not a
  wall (backdating the state by the real 6080s gap between kills lets a 7th alert fire) —
  both suites green, health exit 0, 0 unpushed.
- Re-executed again against HEAD e9ab3d4 (2026-09-07 19:04Z, the bead's 5th attempt): suites
  13/13 and 7/7, replay 10/10, health exit 0, 0 unpushed, all 8 timers future-triggered.
  Process lesson from the same hour: the closing attempt's *verified* close had been
  auto-reverted by NEEDLE's `shipped_work` gate — not by any verification failure, but
  because co-tenant commit 8cc1172 (the FP-wiring fix, landed 18:51:30Z) was still unpushed
  when the gate ran at 18:52:53Z. In this shared worktree a close can be reverted by a
  *neighbor's* unpushed commit: check `git log origin/main..HEAD` and attribute before
  re-deriving work.
- Re-executed again against HEAD d23f24d (2026-09-07 19:31Z, the bead's 6th attempt): suites
  12/12 at HEAD and 13/13 in the worktree (the 13th test is a co-tenant's uncommitted
  addition — count drift = suite growth, not staleness), `test-closed-bead-filter.sh` 7/7
  both ways, replay 10/10, health exit 0, 0 unpushed, all 8 timers future-triggered.
  New lesson — **`test-closed-bead-filter.sh` is cwd-sensitive**: its premise calls the real
  `bead show bf-2vtzg` from PATH, and bead-rs resolves the workspace from cwd. Run from
  outside the repo (e.g. a `git archive HEAD` extract in /tmp), the ancestor
  `/home/coding/.beads` workspace answers with no bf-2vtzg record → empty status →
  "test premise broken" plus a downstream `Classification failed` exit 2: a 3/7 red that is
  pure environment, not a regression (the same HEAD code passes 7/7 from the repo cwd).
  When a HEAD-extract run of this suite goes red, re-run from the repo workspace before
  investigating the alert manager.

**Known defect (mechanism FIXED at HEAD 8cc1172 — bead `domchk-f6fff20f` still open,
pending its owner):** `crash-alert-manager.sh` used to read `CLASSIFICATION` as the
classifier's first stdout line, but `crash-classifier.sh main()` printed its `====` banner
first — so `CLASSIFICATION` was always the banner string. The manager's `FALSE_POSITIVE`
branch was therefore dead code (an FP-classified crash on an open bead could still generate
an alert), the cooldown keyed on a constant string (global across classifications, not
per-type), and `crash-history.jsonl` recorded a garbage classification field. Cascade
prevention was *never* affected — verified above. **Fixed by 8cc1172 (domchk-701bcfa5,
2026-09-07 18:51Z):** the classifier's `main()` now emits the verdict before any human
context, and the manager greps an anchored classification token
(`^(FALSE_POSITIVE|SERVICE_FAILURE|INFRASTRUCTURE|CODE_DEFECT|UNKNOWN)\s*$`) with a
`head -1` fallback. Verified first-hand against HEAD e9ab3d4 the same hour — the cascade
replay logs `Classification: UNKNOWN`, not a banner. The tracking bead remains open with
its owner; it tracks closure/verification, not the mechanism. Lesson: the suite's
grep-marker tests (tests 4–12) cannot see wiring bugs like this; a functional replay can.

**Dated correction 2026-09-07 (domchk-3b605127):** tracking bead `domchk-f6fff20f` is now
**Closed** (rev 4) — the fix was re-read live at HEAD `7e34c2f` before closing. **Capture-race
rule (from the corpus RCA, `docs/crash-root-cause-domchk-4f0b8b43-2026-09-07.md` §4-C):** a
kill wave can take the needle worker *before it writes the crash record* — bf-57nao4's
event stream ends at its fatal `dispatch` with no `fail`/`crash` record ever written — so a
trace that says `outcome: crash` against a silent `events.jsonl` is classified **from the
trace**, and the classifier printing `UNKNOWN` is never by itself evidence that no crash
occurred. Proposed automated fix (classifier emits `INFRASTRUCTURE` + mechanism instead of
bare `UNKNOWN` in that shape): `docs/crash-fix-strategy-domchk-3b605127-2026-09-07.md` §3.2.

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

**Dispatch entry point (remediation-plan GAP-4):** start agent dispatches through
`./scripts/needle-with-limiter.sh` — it gates `needle run`/`needle supervise` through the
crash-storm circuit breaker (`scripts/crash-circuit-breaker.sh`) and the concurrency
limiter, so a bead in storm backoff is deferred out of the ready frontier instead of being
re-dispatched into the same crash (the bf-4x12ec shape: 44 identical kills → 44 alert
beads). `./scripts/preflight-health-check.sh` Check 4 surfaces any OPEN breaker at preflight
time.

### Repository Bloat Prevention and Detection

**Critical:** Repository bloat caused the worst infrastructure crashes in this workspace: bf-1s6c3 and bf-4yjq (2026-08-12) ran an ~18GB repository with ~17GB of loose objects — **17+ identical 237MB `.beads/*.jsonl` snapshots had been committed** — so every significant git operation memcg-OOM'd (76 dispatches / 71 kills for bf-1s6c3 alone over ~4.5 h, plus 50 kills in bf-4yjq the same evening — all exit -1). It is repaired as of 2026-09-06 (see Current Repository Health above); this section is what keeps it from coming back.

**Quick Reference:** See [Repository Maintenance Guide](docs/maintenance/repository-maintenance-guide.md) for daily maintenance procedures and emergency cleanup steps.

**Repository Size Limits:**
| Metric | Healthy | Warning | Critical | Action Required |
|--------|---------|---------|----------|-----------------|
| **Total Repository Size** | <500MB | 500MB-1GB | >1GB | Immediate cleanup |
| **Loose Objects** | <100MB | 100MB-500MB | >500MB | Run git gc |
| **Loose Object Count** | <100 | 100-1000 | >1000 | Pack objects |
| **Size Ratio (Loose:Packed)** | <1:10 | 1:10 to 1:2 | >1:2 | Inverted - critical |

**Repository Bloat Symptoms:**
- Exit code -1 (SIGKILL) during git operations
- Repository size > 5GB (should be <500MB)
- Loose objects > 1GB (should be packed)
- Routine git operations trigger OOM
- Multiple crashes over short period (all exit code -1)

**Detection Commands:**
```bash
# Check repository size
du -sh .git

# Check loose vs packed objects
git count-objects -vH

# Full repository health check
./scripts/check-repo-health.sh

# Monitor repository continuously
./scripts/repo-health-monitor.sh
```

**Prevention Measures:**

1. **GitIgnore Configuration** (CRITICAL — this is the fix that ended bf-4yjq):
   ```bash
   grep -n "beads\|jsonl\|\.db" .gitignore   # → ".beads/", "*.db", "*.jsonl"
   git ls-files .beads | wc -l               # → 0 tracked files
   ```
   The **whole `.beads/` directory** is ignored (plus `*.db` and `*.jsonl`
   repo-wide) — not merely its JSONL patterns. Never re-track bead state.
   Note the repo-wide `*.jsonl` rule: scratch `.jsonl` artifacts stay
   untracked unless explicitly force-added.

2. **Pre-commit hook** (installed in this clone):
   ```bash
   ls .git/hooks/pre-commit   # blocks any staged file > 10MB
   ./scripts/setup-git-hooks.sh --check   # exit 0 = installed and current
   ```
   Per-clone, so a fresh clone is unprotected until
   `./scripts/setup-git-hooks.sh install` is run (idempotent; installs
   `scripts/pre-commit-repo-size-hook` — the canonical tracked source, and the
   file `--check` byte-compares against; `.githooks/pre-commit` is a stale
   2026-09-01-era second copy that nothing executes, `core.hooksPath` unset).
   Self-test: `scripts/test-setup-git-hooks.sh`. Shipped 2026-09-06 (`dfa60a9`)
   as the last open gap in this layer (G-1). This hook is the backstop that
   would have blocked the 237MB `.beads/*.jsonl` commits.

3. **Scheduled Maintenance** — systemd user **timers**, not crontab (this box
   is NixOS; there is no `crontab`):
   ```bash
   ./scripts/setup-repo-maintenance.sh     # install/refresh the timers
   systemctl --user list-timers 'domain-check-*' --all
   ```
   Repo health + auto-gc check daily 02:00, incremental gc daily 03:00, full gc
   weekly Sun 04:00 (`MemoryMax=4G`). Details under Monitoring and Alerting below.

**Emergency Cleanup (If Repository Bloated):**
```bash
# 1. Check repository state
git count-objects -vH
du -sh .git

# 2. Run safe git gc with monitoring
./scripts/safe-git-gc.sh --full

# 3. Monitor progress in another terminal
./scripts/safe-git-gc-monitor.sh --watch

# 4. Verify cleanup success
du -sh .git
git fsck --full
```

**Evidence from bf-1s6c3 / bf-4yjq (2026-08-12):**
- Repository: 18GB (should be <500MB) - 36x larger than normal
- Loose objects: 17.16GB (should be packed) - 99% of repository
- Root cause: 17+ identical 237MB `.beads/*.jsonl` snapshots committed to git
- 76 dispatches / 71 memcg-OOM kills for bf-1s6c3 (Aug-12 21:31Z → Aug-13 02:01Z), all exit code -1; bf-4yjq added 50 kills the same evening
- Resolution: packed down to 93MB (99.5% reduction), re-verified 2026-09-06
- No code defects found - purely infrastructure issue
- Full record: [crash analysis bf-1s6c3 (2026-09-06)](docs/crash-analysis-bf-1s6c3-2026-09-06.md) — supersedes the 2026-09-01 corpus's "9 crashes in 2.5 hours" count

### Monitoring and Alerting

**Continuous Monitoring Setup:**

Automated monitoring runs as **systemd user timers** (this box is NixOS — there is no `crontab`; the cron-based `scripts/monitoring-setup.sh` does not work here):

```bash
# Install/refresh the repo-health + git-gc timers (runs daemon-reload + enable --now)
./scripts/setup-repo-maintenance.sh

# Verify every timer has a future Trigger time
systemctl --user list-timers 'domain-check-*' --all
```

**Installed Timers (re-verified 2026-09-06 — all six present and firing):**
- Crash pattern detection: every 10 minutes (`domain-check-monitoring.timer`)
- Resource monitoring: every 5 minutes (`domain-check-resource-monitor.timer`)
- Service monitoring: every 2 minutes (`domain-check-service-monitor.timer`)
- Repo health + auto-gc check: daily 02:00 (`domain-check-repo-health.timer`)
- Incremental git gc: daily 03:00 (`domain-check-git-gc.timer`)
- Full git gc: weekly Sun 04:00 (`domain-check-git-gc-full.timer`, MemoryMax=4G)

**Gotcha:** after editing any `~/.config/systemd/user/domain-check-*` unit file, run `systemctl --user daemon-reload` — otherwise the manager keeps the stale unit state and the timer silently never fires (this bit the weekly full-gc on 2026-09-02).

**Manual Monitoring Scripts:**

```bash
# Pre-flight health check (run before starting agent tasks)
./scripts/preflight-health-check.sh

# Repository health check
./scripts/check-repo-health.sh

# Resource monitoring (one-time check)
./scripts/resource-monitor.sh --once

# Service monitoring (one-time check)
./scripts/service-monitor.sh --once

# Crash pattern detection (analyze last 24 hours)
./scripts/crash-pattern-detection.sh
```

**Monitoring Logs:**
- `.beads/logs/crash-monitor.log` - Crash pattern alerts
- `.beads/logs/resource-monitor.log` - Resource threshold alerts
- `.beads/logs/service-monitor.log` - Service availability alerts
- `.beads/logs/git-gc-check.log` - Daily 02:00 repo-size/object report (`auto-gc-trigger.sh --dry-run`, the repo-health timer's output)
- `.beads/logs/git-gc.log` / `git-gc-full.log` - Nightly / weekly bounded gc runs

(`repo-health.log` is dormant — it holds only setup-day manual runs. Neither
`check-repo-health.sh` nor `auto-gc-trigger.sh` self-appends anywhere; the timer's
output lands in `git-gc-check.log` via its `StandardOutput=append:`. Redirect
manually if you want a record of a manual health check. Verified live 2026-09-07.)

**Recommended Alerts:**
- **Memory Pressure:** Alert at 70% pressure (before 80% OOM threshold)
- **Disk Space:** Alert at < 30GB free
- **Repository Size:** Alert at >1GB (critical threshold)
- **Loose Objects:** Alert at >500MB (needs packing)
- **Crash Surge:** Alert at 10+ crashes in 10 minutes (infrastructure event)
- **Service Availability:** Monitor inference gateway health endpoint

**Implementation Status:** ✅ Monitoring operational — all six systemd timers installed and firing (verified 2026-09-06). Install/refresh with `./scripts/setup-repo-maintenance.sh`; the cron-based `scripts/monitoring-setup.sh` does not work on this NixOS box.

### Key Learnings

**What Causes Crashes:**
1. **Infrastructure events (70%)**: Memory pressure, OOM, SIGHUP cascade, **repository bloat (historical — repaired 2026-09-06)**
2. **Agent workflow limitations (20%)**: Max turns, bead closing issues
3. **External service failures (8%)**: Inference gateway availability
4. **Code defects (2%)**: Actual application errors — **NONE found in domain-check**

**Repository Bloat as a Crash Cause (repaired 2026-09-06):**
- bf-1s6c3 and bf-4yjq (2026-08-12): 18GB repository, 17GB loose objects, 17+ identical 237MB `.beads/*.jsonl` commits → memcg OOM during git operations (exit code -1)
- Resolution: packed down to 93MB and holding; `.beads/` fully gitignored, `fsck` clean
- Prevention is now layered, not manual: gitignore (bead state can't re-bloat the repo) → 10MB pre-commit gate → `pack.windowMemory` bounds on bare gc/push → daily automated health check (no need to run `check-repo-health.sh` by hand; the 02:00 timer does it)

**Live fleet crash signature (September 2026 census):**
- Exit -1 (kernel kills) is near-zero in steady state; the ones that do occur come from synthetic test/gc scopes, not live agent work
- The dominant live signal is synchronized `exit=1` waves (13–15 workers/minute across beads) — that is service-class, not a per-bead defect
- Needle OTLP 503s are usually the otel-collector crashlooping on config drift — classify before investigating

**What Does NOT Cause Crashes:**
1. ✅ Domain-check code (no defects found in any investigation)
2. ✅ Git GC operations (when using safe-git-gc scripts)
3. ✅ Normal application operations (well within resource limits)
4. ✅ Repository maintenance (with proper monitoring and pre-flight checks)

**Bottom Line:** Domain-check code is stable and defect-free. Focus crash investigation efforts on infrastructure and service availability issues, not code defects — and verify the target bead's actual state first, since most crash alerts today point at work another worker already finished.
