# Domain Check Crash Response Guide

**Created:** 2026-09-01  
**Purpose:** Agent guide for investigating and responding to crash alerts  
**Related:** `docs/crash-mitigation-strategies.md`, `docs/crash-analysis-bf-1s6c3-2026-09-06.md` (bf-1s6c3 canonical report — the 2026-09-01 corpus it supersedes is listed under Related Documentation below), `docs/maintenance/repository-maintenance-guide.md`

---

## Quick Reference: Crash Classification

When investigating a crash, first classify the type:

| Exit Code | Pattern | Classification | Action |
|-----------|---------|----------------|--------|
| **-1** | Signal death, code unrecorded (needle sentinel — see note 2) | Infrastructure event | Check system resources, verify work completion |
| **-1** | Fixed-cadence re-dispatch deaths, `.git` > 5GB | **Infrastructure: Repository bloat** | Check repo size → `safe-git-gc.sh` cleanup (Pattern 3) |
| **1** | error_max_turns | Workflow failure | Verify task completed, check bead closing issues |
| **1** | HTTP 503/502 | Service unavailability | Check inference gateway status, retry with backoff |
| **137** | SIGKILL (128+9) | OOM killer | Check memory pressure, verify git gc safety |
| **124** | Needle's 600 s dispatch cap | Workflow: timeout | Read the transcript — a timeout with **no tool calls** in the window means the agent never started work (4 of bf-1s6c3's 76 attempts; the terminal ones issued none) |
| **Other** | Application error | Code/task issue | Standard debugging |

> **1. Repository bloat is a distinct infrastructure sub-type**, not generic memory pressure: the
> OOM trigger is the repository's own size, so it recurs on every dispatch until the repo is
> cleaned — and unlike memory pressure, it is detectable *before* any crash (see
> Pattern 3 below for detection heuristics and cleanup).

> **2. `exit -1` is a sentinel, not a signal number.** Needle writes `-1` for any signal death
> with no recorded code (`code().unwrap_or(-1)`); the correct Unix encoding of a SIGKILL death
> is 137, and a SIGHUP death would surface as 129. Never assert a specific signal from `-1`
> alone — read kernel/journald records first. bf-4yjq (2026-08-12) recorded all 50 deaths as
> `-1`, and the mechanism was a memcg OOM SIGKILL, not the SIGHUP the first reports claimed.

---

## Automated Crash Alert System (Implemented 2026-09-02)

> **2026-09-07 status — read this first.** The alerting layer changed after this
> section was written. Duplicate-alert defense now lives in a dedicated gate,
> `scripts/alert-deduplication.sh check <alert-bead-id>` (exit 0 = DUPLICATE →
> suppress, 1 = UNIQUE → proceed, 2 = usage, 3 = INDETERMINATE → fail open),
> backed by `scripts/crash-resolution-tracker.sh`, whose `check` now evaluates
> live bead closure and work-completion markers instead of the 0-record ledger
> it used to read (gaps D-1..D-10 in
> [alert-deduplication-gap-analysis-2026-09-07.md](alert-deduplication-gap-analysis-2026-09-07.md),
> shipped in `48aafce`). Run the gate **before** starting any investigation of an
> ALERT bead — most alerts today point at work another worker already finished.
> Caveats on the subsections below: nothing in production invokes
> `crash-alert-manager.sh` (no timer, no hook), and at HEAD its
> FALSE_POSITIVE / SERVICE_FAILURE branches are inert — its classification
> variable is the classifier's `=====` banner (gap D-3). Architecture, usage,
> testing, and troubleshooting: **[alerting-system-guide.md](alerting-system-guide.md)**.

### Quick Start: Automated Crash Processing

**NEW:** Use the automated crash alert system before manual investigation:

```bash
# Process a crash alert with full automation
./scripts/crash-alert-manager.sh <bead-id>

# Auto-process recent crashes
./scripts/crash-alert-manager.sh --auto-process

# Classify a crash type
./scripts/crash-classifier.sh <bead-id>

# Test crash alert fixes
./scripts/test-crash-alert-fixes.sh
```

### What the Automated System Does

The crash alert manager automatically implements all 6 critical fixes:

1. **Closed Bead Filtering** - Skips alerts for beads that already completed successfully
2. **Duplicate Detection** - Prevents multiple investigation beads for same crash
3. **Exit Code Validation** - Checks exit code 0 (success) vs actual crash
4. **Completion Awareness** - Detects post-completion termination vs crash during task
5. **Alert Cooldown** - 5-minute cooldown prevents alert spam during system-wide events
6. **Crash Classification** - Categorizes crashes as FALSE_POSITIVE, SERVICE_FAILURE, INFRASTRUCTURE, or CODE_DEFECT

### Classification Types

| Classification | Description | Action Required |
|----------------|-------------|-----------------|
| **FALSE_POSITIVE** | Post-completion cleanup failure, max_turns, or completed bead | No action - close bead |
| **SERVICE_FAILURE** | External service unavailable (HTTP 503/502) | Retry with backoff when service restored |
| **INFRASTRUCTURE** | memcg-OOM inside the dispatch scope, resource exhaustion, repository bloat | Check the cgroup boundary and repo size, verify work completion |
| **CODE_DEFECT** | Actual application error | Standard investigation required |
| **UNKNOWN** | Unable to classify | Manual investigation required |

### Example Usage

```bash
# Investigate crash alert bf-3561g
./scripts/crash-alert-manager.sh bf-3561g

# Output:
# INFO: Checking bead closure status for: bf-3561g
# INFO: Checking for existing alert beads for target: bf-4k2ws
# INFO: Validating exit code before generating alert
# INFO: Classifying crash type...
# Classification: FALSE_POSITIVE
# Reason: Bead bf-4k2ws already closed (completed successfully)
# Action: No alert generated - false positive filtered
```

### Monitoring System Integration

The automated crash system integrates with continuous monitoring:

```bash
# Install continuous monitoring (systemd user timers — this box has no crontab)
./scripts/setup-repo-maintenance.sh
systemctl --user list-timers 'domain-check-*' --all

# Monitor logs
tail -f .beads/logs/crash-alert-manager.log
tail -f .beads/logs/crash-monitor.log
tail -f .beads/logs/resource-alerts.log
```

### When to Use Manual vs Automated

**Use Automated System (crash-alert-manager.sh) for:**
- All standard crash alerts
- Post-completion cleanup failures
- System-wide event detection
- Duplicate alert filtering

**Use Manual Investigation (this guide) for:**
- Unusual crash patterns not classified by automation
- Code defects requiring debugging
- Complex multi-factor crashes
- Verification of automated classification

---

## Work Completion Verification (Pre-Close Gate)

Before closing any bead — and especially after any long-running operation
(git gc, bulk checks, large refactors) — verify the work actually landed:

```bash
./scripts/verify-work-completion.sh <bead-id> \
  --require-path <artifact-the-task-was-supposed-to-produce> \
  --summary "what was done, committed in <sha>"

# Exit 0 → safe: bead close <bead-id> --reason "work verified"
# Exit 1 → do NOT close: commits unpushed / artifacts missing / box unhealthy
```

The gate fails when commits are not pushed, expected artifacts are missing, or
the box is under memory/disk/load pressure. It records the outcome to
`.beads/state/work-completion/<bead-id>.json` either way.

**Using markers during crash triage:** when a worker died with exit code -1,
check for a marker before investigating:

```bash
cat .beads/state/work-completion/<bead-id>.json 2>/dev/null
```

- `"result": "VERIFIED"` — the task itself was complete when the worker died;
  treat the crash as post-completion (usually a false positive), close or re-verify the bead
- `"result": "FAILED"` — verification found real gaps (unpushed commits,
  missing artifacts); the work needs finishing, not just re-dispatch
- no marker — the worker died before reaching verification; fall through to
  the Investigation Checklist below

Full usage: `scripts/README.md` → Work Completion Verification. Tests:
`./scripts/test-verify-work-completion.sh`.

---

## Investigation Checklist

### Phase 1: Immediate Classification (2 minutes)

**Automated First Step:**
```bash
# Try automated classification first
./scripts/crash-classifier.sh <bead-id>

# If classification is CODE_DEFECT or UNKNOWN, proceed with manual investigation
```

**Manual Classification (if automated system insufficient):**

```bash
# 1. Get bead metadata
bead show <id> --json

# 2. Check exit code and outcome
# Exit code -1 → Infrastructure event (skip to Phase 2A)
# Exit code 1 with "error_max_turns" → Workflow failure (skip to Phase 2B)
# Exit code 1 with HTTP 5xx → Service failure (skip to Phase 2C)
# Other → Standard investigation (Phase 2D)

# 3. Check current system state
free -h                    # Memory availability
df -h /                    # Disk space
uptime                     # Load average
```

**Crash-timestamp caveat (learned from bf-1s6c3, added 2026-09-06):** the `Timestamp` in a
crash-alert bead's description is the crash handler's `HANDLING_RELEASE_DONE` **heartbeat,
not the kill** — it trails the real death by 6.4 s (bf-1s6c3: alert 22:04:12.524613796 vs.
`agent.completed` 22:04:06.124743603Z), 7.2 s (its 22:24:04 event), and 8–120 s (bf-173o7e).
An investigation window built on the alert time can miss the death entirely. Get the real
time from the needle worker log's `agent.completed` record for the bead/agent, and widen
`<crash_timestamp>` windows below (and any journalctl `--since`) to absorb the drift.

### Phase 2A: Infrastructure Event (Exit Code -1)

**Pattern:** SIGKILL, SIGHUP, OOM killer → System-wide resource pressure

**Checklist:**
- [ ] Verify task completed successfully before crash
  ```bash
  git log --since="<crash_timestamp>" --until="<crash_timestamp+30min>" --oneline
  ```
- [ ] Check for system-wide events
  ```bash
  journalctl --since "<crash_timestamp-1hour>" --until "<crash_timestamp+1hour>" | grep -E "oom|kill|memory"
  ```
- [ ] Check the **cgroup boundary, not just the host** — the binding limit for agent work is
  the dispatch scope's `MemoryMax` (12 GiB), and the kill can land while the host has memory
  to spare (bf-4yjq class: 50 kills inside the scope on a healthy host)
  ```bash
  # Real memcg kills carry kernel scope/task/rss lines. A systemd "killed by the OOM
  # killer" notice with NO kernel oom-kill line in the same seconds is a NixOS
  # switch-to-configuration re-execution replaying stale memory.events counters,
  # not a kill.
  journalctl --since "@<epoch>" | grep -E "oom-kill|constraint=CONSTRAINT_MEMCG|memory peak"
  ```
- [ ] Classify as false positive if work completed
  ```bash
  # If commit exists within 30 seconds before crash → FALSE POSITIVE
  # If no commit found → Proceed to Phase 2D
  ```

**Common Infrastructure Events:**
- **memcg OOM inside the dispatch scope (dominant, kernel-verified):** cgroup-scoped SIGKILL
  when a git operation exceeds the scope's `MemoryMax` (12 GiB) — the host can have memory to
  spare (bf-4yjq, bf-4x12ec, bf-198ne). See note 2 above and the cgroup check in this checklist
- **Memory Pressure:** systemd-oomd activation (94.71% pressure threshold)
- **OOM Killer (host-wide):** Process termination (exit code 137)
- **SIGHUP Cascade:** System-wide signal to all workers — historically asserted for the
  Aug-12 storms but never kernel-confirmed; do not claim it without a 129 exit code or
  other signal evidence

**Action Required:**
- ✅ NO CODE CHANGES NEEDED
- ⚠️ Document in bead notes as "false positive - infrastructure event"
- ⚠️ Close bead with clear notes

### Phase 2B: Workflow Failure (error_max_turns)

**Pattern:** Agent exhausted 30-turn limit during post-task operations

**Checklist:**
- [ ] Verify main task completed successfully
  ```bash
  # Check for task completion markers:
  # - Git commits with task-related changes
  # - Test results showing success
  # - Documentation indicating completion
  ```
- [ ] Identify where agent got stuck
  ```bash
  # Read trace file for last actions
  jq -r '.[] | select(.type == "tool_call") | .tool' .beads/traces/<id>/trace.jsonl | tail -20
  ```
- [ ] Classify as false positive if task succeeded
  ```bash
  # If task evidence exists → FALSE POSITIVE - workflow issue only
  # If no evidence of task completion → Proceed to Phase 2D
  ```

**Common Workflow Failures:**
- **Bead closing loops:** Agent retrying close operations
- **Post-completion troubleshooting:** Agent trying to resolve non-issues
- **Verification loops:** Excessive checking of completed work

**Action Required:**
- ✅ NO CODE CHANGES NEEDED
- ⚠️ Document main task outcome in bead notes
- ⚠️ Close bead with task completion status

### Phase 2C: Service Failure (HTTP 503/502)

**Pattern:** Inference gateway or external service unavailable

**Checklist:**
- [ ] **Check if preflight health check was run**
  ```bash
  # Review health check log
  cat /tmp/preflight-health-check.log | tail -50
  
  # If preflight check was skipped → Preventable crash
  # If preflight check failed → Expected deferral (not a crash)
  # If preflight check passed → Unexpected service failure
  ```
- [ ] Check service availability
  ```bash
  # For inference gateway (zai provider):
  curl -f --max-time 5 https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health || echo "Gateway down"
  
  # Or use the preflight check script:
  ./scripts/preflight-health-check.sh --verbose
  ```
- [ ] Identify failure point
  ```bash
  # Check trace for service error messages
  grep -i "503\|502\|unavailable" .beads/traces/<id>/trace.jsonl
  ```
- [ ] Verify no domain-check code involved
  ```bash
  # Service failures are external to domain-check code
  # Agent framework issue, not application issue
  ```

**Common Service Failures:**
- **Inference Gateway 503:** "no available server" (traefik-apexalgo-iad)
- **Network timeouts:** Temporary connectivity issues
- **Rate limiting:** External API limits exceeded

**Action Required:**
- ✅ NO CODE CHANGES NEEDED
- ⚠️ Retry task with backoff if service is now available
- ⚠️ If service still down, defer task until restored

### Phase 2D: Standard Investigation (Other exit codes)

**Pattern:** Possible application-level error

**Checklist:**
- [ ] Examine crash artifacts
  ```bash
  ls -la .beads/traces/<id>/
  # - trace.jsonl (conversation trace)
  # - metadata.json (session metadata)
  # - stdout.txt (agent output)
  # - stderr.txt (agent errors)
  ```
- [ ] Review error messages
  ```bash
  jq -r '.[] | select(.type == "error") | .message' .beads/traces/<id>/trace.jsonl
  ```
- [ ] Check for domain-check code involvement
  ```bash
  # If crash involved domain-check operations → Investigate code
  # If crash was in agent framework → Infrastructure/workflow issue
  ```
- [ ] Verify repository integrity
  ```bash
  git fsck --full
  git status
  ```

---

## Common Crash Patterns

### Pattern 1: Post-Completion False Positive (~40% of crashes)

**Symptoms:**
- Exit code -1 (SIGKILL)
- Work committed successfully before crash
- 30-second gap between completion and termination

**Example Timeline:**
```
16:35:54 UTC - Task completed, commit 549aa42 created
16:36:24 UTC - Agent terminated (SIGKILL)
16:36:51 UTC - Bead closed successfully
```

**Action:** Classify as FALSE POSITIVE, close with notes

### Pattern 2: Git GC Operations (~15% of crashes)

**Symptoms:**
- Exit code 137 (OOM killer) or -1 (SIGKILL)
- `git gc --aggressive` in progress
- High memory usage during git operation

**Verification:**
```bash
# Check if git gc completed successfully
git count-objects -vH
git fsck --full

# If repository valid and compressed → Git gc succeeded, termination was cleanup
```

**Action:**
- ✅ Use `scripts/safe-git-gc.sh` instead of bare `git gc --aggressive`
- ✅ Verify repository integrity
- ⚠️ If OOM occurred, document memory limits used

### Pattern 3: Infrastructure — Repository Bloat Crashes (~15% of infrastructure crashes)

**Symptoms:**
- Exit code -1 (SIGKILL from memcg OOM), zero exit-code variation across events
- Crashes recur at a fixed cadence on every re-dispatch — agents die minutes apart, for hours
- Repository size > 5GB (should be <500MB)
- Loose objects > 1GB (should be packed)
- **`.git/objects` > 10GB = high risk → preemptive cleanup required**
- Routine git operations (clone, fetch, checkout, gc, fsck, push) trigger OOM
- Multiple crashes over a short period (all exit code -1)

**Evidence from bf-4yjq (2026-08-12):**
- **50 crashes** between 17:54:00 and 20:30:43 UTC, every one exit code -1 — one death every
  ~3.1 minutes for 2.5 hours. (The "9 crashes at ~17-minute intervals" figure in the earlier
  comprehensive report is superseded — see its banner and the canonical investigation.)
- Repository: 18GB with 17GB loose objects; trigger was 17+ identical ~237MB
  `.beads/*.jsonl` bead-state snapshots that had been committed to git
- Deterministic environmental kill, not a code defect: domain-check code was never involved
- **Repaired:** packed to ~94MB, `.beads/` fully gitignored (0 tracked files), `git fsck` clean —
  re-verified 2026-09-06 (`docs/crashes/bf-4yjq-cleanup-verification.md`)

**Evidence from bf-1s6c3 (same evening, 21:31Z → 08-13 02:01Z) — the push-side sibling:**
- **76 dispatches: 71 × exit −1, 4 × 124, 1 × 0** over 4 h 30 m, median 177 s between deaths —
  and **71 crash alerts, one per kill**. (The 2026-09-01 corpus's "9 crashes / FALSE POSITIVE"
  reading is superseded; the canonical report's §11 lists each correction.)
- **71 of 76 attempts died with a `git push` as their last issued command** — the kill lands
  inside pack-objects, not the merge. The last-command distribution across attempts is a
  detection signature in its own right: when many attempts of one bead end mid-`push`/`gc`/`fsck`,
  suspect the repository, not the task.
- The deliverable (merge `42a7b07`) landed at attempt 4 — **59.6 s before its worker died** — so
  72 later dispatches re-ran expensive git work against an already-satisfied task on a ~10 s
  re-claim cycle, until attempt 76 survived *by changing the task shape* (auto-split into
  bead-only children), not because the environment improved.
- Canonical record: `docs/crash-analysis-bf-1s6c3-2026-09-06.md` — its §12 carries first-hand
  re-verifications (repository at ~102 MB, `42a7b07` a commit but not an ancestor of `main`,
  on-`main` reconciliation `46293c5`).

**Detection heuristics (run before any significant git operation):**
```bash
du -sh .git                # < 1GB healthy · 1-5GB warning · > 5GB critical
du -sh .git/objects        # > 10GB = HIGH RISK → preemptive cleanup required
git count-objects -vH      # loose count/size vs packed

# Scripted equivalents:
./scripts/check-repo-health.sh         # full diagnostic pass
./scripts/preflight-health-check.sh    # fails when .git exceeds 1GB
```

**Cleanup — `scripts/safe-git-gc.sh` is the prescribed method. Never run bare
`git gc --aggressive`:**
```bash
# 1. Confirm cleanup is actually needed
./scripts/safe-git-gc.sh --check-only

# 2. Run it (stages 1-2, or --full for deep compression; --resume after interruption)
./scripts/safe-git-gc.sh --full

# 3. Monitor from another terminal
./scripts/safe-git-gc-monitor.sh --watch

# 4. Verify
du -sh .git && git fsck --full
```

The bare-gc path is also defended mechanically, by persistent git config rather than
convention: `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` bound any
pack-objects run — including bare `git gc` and `git push` — to ≈3GiB worst case. Check the
*effective* bound (system → global → local) with `./scripts/setup-git-gc-config.sh --verify`;
exit 1 means no bound is in force or threads are unpinned.

**Prevention:**
- ✅ Keep the **whole `.beads/` directory** (plus `*.db` and `*.jsonl` repo-wide) in
  `.gitignore` — never re-track bead state. This is the fix that ended bf-4yjq; ignoring only
  `.beads/*.jsonl` patterns is not sufficient. Verify: `git ls-files .beads | wc -l` → 0
- ✅ 10MB pre-commit hook — install/verify per clone with `./scripts/setup-git-hooks.sh install`
  / `--check` (committed 2026-09-07 as `dfa60a9`; per-clone, so a fresh clone is unprotected
  until installed). Source: `scripts/pre-commit-repo-size-hook` + `.githooks/pre-commit`;
  self-test `scripts/test-setup-git-hooks.sh`
- ✅ Scheduled maintenance via **systemd user timers**: `./scripts/setup-repo-maintenance.sh`
  (repo health + auto-gc check daily 02:00, incremental gc daily 03:00, full gc Sun 04:00).
  Do **not** use `./scripts/monitoring-setup.sh` — it is cron-based and this box (NixOS) has no
  crontab
- ✅ Size limits and the full maintenance procedure:
  `docs/maintenance/repository-maintenance-guide.md`

**Reference artifacts:**
- `docs/reports/bf-4yjq-comprehensive-crash-report.md` — comprehensive crash report
  (root-cause analysis and telemetry valid; crash count/cadence superseded — see its banner)
- `docs/crash-investigations/bf-4yjq-crash-investigation.md` — canonical investigation
  (verified 50-crash count)
- `docs/crash-artifacts-bf-4yjq.md` — preserved crash artifacts
- `docs/crashes/bf-4yjq-cleanup-verification.md` — post-cleanup verification record
- `docs/crash-mitigation-strategies.md` — mitigation strategies

### Pattern 4: Service Availability Failure (~8% of crashes)

**Symptoms:**
- Exit code 1 with HTTP 503/502 errors
- "no available server" message
- Inference gateway unavailable

**Action:**
- ⚠️ Check gateway status: `curl https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health`
- ⚠️ Retry task when service restored
- ✅ NO code changes needed

### Pattern 5: Max Turns Exhaustion (~20% of crashes)

**Symptoms:**
- Exit code 1 with "error_max_turns"
- Agent spent turns on post-task troubleshooting
- Main task completed successfully

**Action:**
- ✅ Verify main task completion
- ⚠️ Document workflow issue
- ✅ NO code changes needed if task succeeded

---

## False Positive Detection Heuristics

Use these rules to quickly identify false positives:

### Rule 1: Time Gap Check
```bash
# If commit exists < 30 seconds before crash → FALSE POSITIVE
commit_time=$(git log -1 --format=%ct <commit_hash>)
crash_time=$(date -d "<crash_timestamp>" +%s)
gap=$((crash_time - commit_time))

if [ $gap -lt 30 ]; then
  echo "FALSE POSITIVE: Work completed $gap seconds before crash"
fi
```

> **Rule 1 across a storm (bf-1s6c3, 2026-09-06):** the 30-second check compares the *last*
> commit with the *last* crash. When a re-dispatch loop is involved, also ask whether the
> **deliverable** landed anywhere in the storm window — bf-1s6c3's merge landed at attempt 4 and
> the loop then ran 72 further dispatches against satisfied work. Deliverable-present +
> bead-still-open is workflow debt (verify-then-close, don't re-run the task), whatever killed
> the workers.

### Rule 2: Success Pattern Check
```bash
# If crash → retry → success pattern → SELF-HEALED TRANSIENT FAILURE
# Check bead event history for successful retries
bead show <id> --json | jq '.history[] | select(.outcome == "success")'
```

> **Rule 2 caveat (bf-1s6c3, 2026-09-06):** an exit-0 terminal attempt after a kill storm is
> only a *surface* match for "self-healed". bf-1s6c3's attempt 76 exited 0 because the
> auto-split changed the task shape (bead-only children, no `git push`) — the 18 GB cause was
> untouched. A persistent cause outlasting the retry loop is Infrastructure, not transient:
> confirm the environment actually changed before classifying the storm self-healed.

### Rule 3: System-Wide Event Check

The committed detector (`scripts/crash-pattern-detection.sh`) fires an infrastructure event
at **3 crashes within 5 minutes** (`CRASH_SURGE_THRESHOLD=3`) — lowered from the older
10-in-10-minutes rule after bf-4yjq (2026-08-12) lost agents every ~3.1 minutes for 2.5 hours
while peaking near 5 per 10 minutes, i.e. under any per-bead threshold until the whole
workspace is measured:

```bash
./scripts/crash-pattern-detection.sh --since 1hour
# exit 0 = stable (or degraded: stale source) · 1 = elevated · 2 = infrastructure event
```

Two corollaries from bf-4yjq:

- **Derive scale from the forensic checkpoint, not from alert beads.** Alert-bead sampling
  recorded 9 crashes for bf-4yjq; the checkpoint scan shows 50, plus a 350-kill storm
  (bf-31mno) recorded nowhere. Scan `.beads/checkpoint/forensic.jsonl` for Crash(-1) records
  in the window — the canonical investigation documents the exact filter.
- **A sustained low-and-slow cadence is still an environmental regime.** Fixed-cadence
  exit −1 re-dispatch deaths across multiple beads means triage repo size / memory / load at
  the workspace level before any per-bead debugging.
- **Watch the re-dispatch loop, not just the kills (bf-1s6c3).** A kill → release → re-claim
  cycle measured in seconds (~10 s there) with no backoff and no resource gate converts one
  undrainable kill into a storm: 71 kills → 71 alerts at 1:1. When you see fixed-cadence
  deaths, compare the `outcome.handled action=alerted` count against the number of *distinct
  causes* — many alerts, one cause means the loop is the amplifier, and fixing the repo (not
  the alerts) drains the whole pool.

---

## Git GC Safety Procedures

### When to Use Safe Git GC Scripts

**ALWAYS use `scripts/safe-git-gc.sh` instead of bare `git gc --aggressive`:**

```bash
# Check if gc needed
./scripts/safe-git-gc.sh --check-only

# Run standard gc (stages 1-2)
./scripts/safe-git-gc.sh

# Run full gc with deep compression
./scripts/safe-git-gc.sh --full

# Monitor progress
./scripts/safe-git-gc-monitor.sh --watch
```

**Why Safe Scripts?**
- ✅ Memory-limited operations (configurable via `SAFE_GC_MEMORY_MAX`)
- ✅ Three-stage strategy (standard → incremental → deep compression)
- ✅ Checkpoint/resume capability after each stage
- ✅ Progress tracking and monitoring
- ✅ Pre-flight integrity checks

**Mechanical guard on the bare-gc path (2026-09-02):** the August 2026 crash storm was caused
by bare `git gc --aggressive` exceeding the dispatch scope's 12GiB `MemoryMax` — memcg OOM
SIGKILL, 129 attempts. That path is now defended by persistent git config, not convention:
`pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` (the window limit is
per-thread, so threads must be pinned) → worst case ≈3GiB per pack run. Applied repo-locally
**and** globally, and it bounds `git push`'s pack-objects too (bf-198ne was the push-side
variant of the same OOM). Verify anytime:
`./scripts/setup-git-gc-config.sh --verify` — exit 1 = no effective bound or unpinned threads.

**When `git gc --aggressive` is Acceptable:**
- On dedicated systems with > 16GB free RAM
- With cgroup memory limits in place
- For one-time optimization of large repos
- When safe-git-gc scripts are unavailable

**When NOT to Use --aggressive:**
- On systems with < 8GB RAM
- On repos > 5GB without memory limits
- Without monitoring/resumability
- On shared systems where OOM affects others

### Memory-Limited Git GC

```bash
# Run git gc under systemd slice with memory limit
systemd-run --scope --quiet \
  -p MemoryMax=2g \
  -p MemorySwapMax=0 \
  -p CPUQuota=200% \
  scripts/safe-git-gc.sh --full
```

### Monitoring Git GC Progress

```bash
# Watch gc progress in real-time
./scripts/safe-git-gc-monitor.sh --watch

# Check gc status
./scripts/safe-git-gc-monitor.sh
```

---

## Service Availability Procedures

### Pre-Flight Health Checks

**IMPLEMENTED:** Pre-flight health check script available at `scripts/preflight-health-check.sh`

**Before starting agent tasks that depend on external services, run:**

```bash
# Standard pre-flight check
./scripts/preflight-health-check.sh

# Verbose mode for detailed diagnostics
./scripts/preflight-health-check.sh --verbose

# Warn-only mode for monitoring
./scripts/preflight-health-check.sh --warn-only
```

**What the script checks:**
- Inference gateway availability
- Memory availability (configurable, default 10GB)
- Disk space (configurable, default 20GB)
- CPU load (configurable, default <10 on 1min average)
- Git repository health — **repository size** (fails above 1GB; this gate previously
  failed open and passed repos of any size, fixed 2026-09-06), large files in history,
  loose-object counts

**Exit codes:**
- `0` - All checks passed
- `1` - One or more checks failed
- `2` - Invalid arguments

**Usage pattern:**
```bash
# Before starting agent task
if ! ./scripts/preflight-health-check.sh; then
  echo "ERROR: System health check failed"
  echo "Task deferred until system is healthy"
  exit 1
fi

# Task proceeds knowing resources are sufficient
./agent-task.sh
```

**See also:** `docs/crash-prevention-preflight-checks.md` (implementation documentation)

---

**Alternative: Manual Health Checks**

If you need to customize the checks, here's the manual approach:

```bash
#!/bin/bash
# Manual pre-flight health check

# Check inference gateway
GATEWAY_URL="https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health"
if ! curl -sf --max-time 5 "$GATEWAY_URL" > /dev/null; then
  echo "ERROR: Inference gateway unavailable"
  echo "Deferring task until service is healthy"
  exit 1
fi

# Check memory availability
AVAILABLE_MEM=$(free -g | awk '/^Mem:/{print $7}')
if [ $AVAILABLE_MEM -lt 10 ]; then
  echo "ERROR: Insufficient memory (${AVAILABLE_MEM}GB available)"
  exit 1
fi

# Check disk space
DISK_FREE=$(df -BG / | awk 'NR==2 {print $4}' | tr -d 'G')
if [ $DISK_FREE -lt 20 ]; then
  echo "ERROR: Insufficient disk space (${DISK_FREE}GB free)"
  exit 1
fi

echo "All health checks passed - proceeding with task"
```

### Repository Size Pre-Flight (before git operations)

Every significant git operation — `gc`, `push`, `clone`, `fetch`, `checkout`, `fsck` —
scales with repository size, and the 2026-08-12 bloat storm (Pattern 3) turned routine
operations into deterministic OOM kills. Check before running one:

```bash
du -sh .git            # < 1GB healthy · 1-5GB warning · > 5GB critical
du -sh .git/objects    # > 10GB = HIGH RISK → preemptive cleanup required
git count-objects -vH  # loose vs packed breakdown

# Scripted gate — exits 1 when .git exceeds 1GB:
./scripts/preflight-health-check.sh
```

Over threshold: run `./scripts/safe-git-gc.sh --full` (see Pattern 3) and re-check before
proceeding. Never work around it with bare `git gc --aggressive`.

### Retry with Exponential Backoff

For transient service failures:

```bash
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
    # Non-transient error - fail immediately
    exit 1
  fi
done

exit 1  # All retries exhausted
```

---

## Crash Documentation Template

When documenting a crash investigation, use this template:

```markdown
# Crash Investigation: <bead_id>

**Investigation Date:** <date>
**Bead ID:** <id>
**Agent:** <agent_name>
**Exit Code:** <code>
**Classification:** <FALSE POSITIVE | INFRASTRUCTURE | SERVICE | CODE>

## Executive Summary
<One-paragraph summary of crash type and classification>

## Crash Timeline
- <timestamp>: Event 1
- <timestamp>: Event 2
- <timestamp>: Crash

## Root Cause
<Primary cause classification>

## Evidence
- System resources at crash time
- Relevant log entries
- Work completion verification

## Action Required
- ✅ NO ACTION or ⚠️ SPECIFIC ACTION

## Classification
<FALSE POSITIVE / INFRASTRUCTURE ISSUE / SERVICE FAILURE / CODE DEFECT>
```

---

## Resource Limits and Monitoring

### Safe Operating Limits

| Resource | Minimum | Warning | Critical |
|----------|---------|---------|----------|
| **Available Memory** | 20GB | 10GB | 5GB |
| **Disk Space** | 50GB | 30GB | 20GB |
| **CPU Load (1min)** | < 5 | < 10 | > 15 |
| **Git GC Memory** | 1GB | 2GB | 4GB |

### Pre-Task Resource Check

```bash
# Check system resources before starting task
echo "=== System Resources ==="
free -h
df -h /
uptime

# Abort if resources insufficient
AVAILABLE_MEM=$(free -g | awk '/^Mem:/{print $7}')
if [ $AVAILABLE_MEM -lt 10 ]; then
  echo "ABORT: Insufficient memory (${AVAILABLE_MEM}GB available)"
  exit 1
fi
```

> **The host-memory gate does not cover the repository-bloat class (bf-1s6c3, 2026-09-06).**
> There the violated axis was the **repository-size** axis — an ≈18 GB object store against the
> dispatch scope's 12 GiB `MemoryMax` — not the host axis: host memory read healthy for all
> 76 dispatches and this gate would have passed every one of them. The operative pre-flight for
> git-heavy work is the **Repository Size Pre-Flight** above (`du -sh .git`,
> `git count-objects -vH`, `./scripts/preflight-health-check.sh`), not host memory alone.

---

## When to Escalate

Escalate to human operator if:

1. **Repository Corruption Suspected**
   - `git fsck` shows errors
   - Objects missing or corrupted
   - Repo size unexpectedly large

2. **Persistent Service Failures**
   - Service down for > 30 minutes
   - Multiple retries fail with same error
   - No service status information available

3. **Unknown Exit Codes**
   - Exit code not in classification table
   - No recognizable error pattern
   - Multiple unexplained crashes

4. **Data Loss Suspected**
   - Work artifacts missing
   - Expected commits not found
   - Test results inconsistent

---

## Monitoring Recommendations

### System-Level Monitoring

```yaml
# Example Prometheus alerts
monitoring:
  alerts:
    - name: HighMemoryPressure
      expr: node_memory_pressure_percentage > 70
      for: 1m
      annotations:
        summary: "Memory pressure above 70% - OOM risk"
    
    - name: DiskSpaceLow
      expr: node_filesystem_avail_bytes{mountpoint="/"} < 20GB
      for: 5m
      annotations:
        summary: "Less than 20GB disk space available"
    
    - name: CrashSurgeDetected
      expr: increase(needle_crashes_total{outcome="failed"}[5m]) >= 3
      for: 5m
      annotations:
        summary: "Infrastructure event: 3+ crashes in 5 minutes"
        description: "Matches the committed detector threshold —
          scripts/crash-pattern-detection.sh CRASH_SURGE_THRESHOLD=3 over a 5-minute window."
```

The alerting lesson from the bloat storms: **alerts scale with kills, not with causes.**
bf-1s6c3 produced 71 alerts for one undrainable repository condition; bf-173o7e produced 131;
bf-31mno 350. Deduplication and the 5-minute cooldown in `scripts/crash-alert-manager.sh`
absorb part of this, but the durable fix is always the underlying condition — drain the cause
and the alert pool empties with it.

### Application-Level Monitoring

```yaml
  - name: InferenceGatewayDown
      expr: up{job="inference_gateway"} == 0
      for: 1m
      annotations:
        summary: "Inference gateway is down"
        description: "Agents will fail until gateway is restored"
    
    - name: NeedleAgentTaskStuck
      expr: needle_agent_task_duration_seconds{outcome="running"} > 7200
      for: 10m
      annotations:
        summary: "Agent task running > 2 hours"
```

---

## Key Learnings Summary

### What Causes Crashes

1. **Infrastructure Events (70%)**: memcg-OOM inside the dispatch scope (kernel-verified for the
   Aug-2026 git gc/push storms), memory pressure, OOM killer, **repository bloat**. The
   "SIGHUP cascade" framing of the 2026-09-01 corpus is superseded — never kernel-confirmed and
   excluded by the exit-code record (no 129 anywhere)
2. **Workflow Failures (20%)**: Max turns exhaustion, bead closing loops
3. **Service Failures (8%)**: Inference gateway unavailable, network issues
4. **Code Defects (2%)**: Actual application errors

### What Does NOT Cause Crashes

1. ✅ **Git GC** - When using safe-git-gc scripts
2. ✅ **Domain-Check Code** - No defects found in any crash investigation
3. ✅ **Normal Operations** - Well within resource limits

### Quick Decision Tree

```
Exit Code -1?
├─ Yes → Infrastructure Event
│  ├─ Work completed within 30s? → FALSE POSITIVE
│  ├─ Deliverable landed earlier in a re-dispatch storm? → Satisfied work; verify-then-close
│  ├─ Final attempt exited 0 after a storm? → Surface match only — confirm the cause was
│  │   removed, not that the task shape changed
│  └─ No completion evidence? → Check system logs
│
Exit Code 1 with error_max_turns?
├─ Yes → Workflow Failure
│  ├─ Main task completed? → FALSE POSITIVE
│  └─ Task incomplete? → Max turns issue
│
Exit Code 1 with HTTP 503/502?
├─ Yes → Service Failure
│  └─ Check gateway status, retry with backoff
│
Exit Code 124?
├─ Yes → Dispatch Timeout (600 s cap)
│  ├─ Tool calls in the window? → Task too slow for the cap; consider decomposition
│  └─ No tool calls? → Agent never started (environment/template issue)
│
Other Exit Code?
└─ Standard Investigation
   ├─ Domain-check code involved? → Debug code
   └─ Agent framework issue? → Workflow/infrastructure
```

---

## Related Documentation

- **Automated Crash Alert System (2026-09-02):**
  - `docs/crash-alert-fix-implementation-2026-09-02.md` - Complete crash alert system documentation
  - `docs/monitoring-implementation-summary-2026-09-02.md` - Monitoring system implementation
  - `docs/verification-report-bf-4k2ws-crash-investigation-2026-09-02.md` - bf-4k2ws crash verification

- **bf-1s6c3 canonical report (2026-09-06):** `docs/crash-analysis-bf-1s6c3-2026-09-06.md` —
  the write-up layer of the same-evening push-side storm (71 kills in 76 dispatches). Its §11
  lists every way the 2026-09-01 corpus is wrong ("9 crashes" → 76 dispatches/71 kills; the
  FALSE-POSITIVE premise; the SIGHUP mechanism; dead SHAs `2832106`/`7dd79eb` → real merge
  `42a7b07`, on-`main` reconciliation `46293c5`). Cite it, **not**
  `docs/comprehensive-crash-investigation-report-2026-09-01.md` (superseded for bf-1s6c3)

- **bf-1ea4g canonical record (2026-09-07):**
  `docs/investigations/bf-1ea4g-root-cause-determination-2026-09-02.md` — 57 attempts /
  56 `exit -1` kills on 2026-08-13; kill **INFRASTRUCTURE** (unbounded `git push`
  pack-objects over a 422-commit unpushed backlog in the repo-bloat era, inside the
  12 GiB dispatch scope) / alert **FALSE_POSITIVE** (the bead self-recovered and closed
  the same morning). The consolidated one-stop view is
  `docs/crash-inventory-bf-1ea4g-summary.md`; raw artifacts live in
  `docs/crashes/bf-1ea4g/`. Cite these, **not** the 2026-09-02-era SIGHUP /
  healthy-repo mechanism (superseded by that doc's dated banner and its §2026-09-07
  re-determination)
- **Mitigation Strategies:** `docs/crash-mitigation-strategies.md`

- **Specific Crashes:** 
  - `docs/crash-analysis-domchk-c9641ac5-2026-09-01.md` (Service availability)
  - `docs/investigation-summary-bf-173o7e-2026-09-01.md` (False positive)
  - `docs/crash-artifacts-bf-4yjq.md` (Repository bloat - 50 OOM crashes from 18GB repo, see below)

- **Repository Bloat (bf-4yjq, 2026-08-12 — repaired and re-verified 2026-09-06):**
  - `docs/reports/bf-4yjq-comprehensive-crash-report.md` — comprehensive report
    (root cause and telemetry valid; crash count/cadence superseded — see its banner)
  - `docs/crash-investigations/bf-4yjq-crash-investigation.md` — canonical investigation
    (verified 50 crashes at ~3.1-minute intervals)
  - `docs/crashes/bf-4yjq-cleanup-verification.md` — cleanup verified, repo at ~94MB
  - `docs/maintenance/repository-maintenance-guide.md` — size limits, daily maintenance,
    emergency cleanup steps
  - `docs/crash-mitigation-strategies.md` — mitigation strategies

- **Git GC Safety:** `docs/safe-git-gc-implementation.md`, `docs/safer-git-gc-strategy.md`

---

**Guide Status:** ✅ Complete  
**Last Updated:** 2026-09-07 (fourth pass — bf-1ea4g canonical-record block added to
Related Documentation, domchk-0a6edc46); third pass, from the bf-1s6c3 canonical report
`docs/crash-analysis-bf-1s6c3-2026-09-06.md`: exit 124 added to the classification table,
INFRASTRUCTURE row and "What Causes Crashes" moved off the superseded SIGHUP framing,
Rule 1/Rule 2 false-positive caveats for re-dispatch storms, re-dispatch-amplifier corollary,
bf-1s6c3 push-side evidence block in Pattern 3, surge example aligned to the committed
3-in-5-minutes threshold, host-memory-gate limitation note, pre-commit hook bullet pointed at
the committed installer) — second pass 2026-09-06 (bf-4yjq closing summary
`docs/crash-summary-bf-4yjq-2026-09-06.md`: `exit -1` documented as needle's unrecorded-signal
sentinel rather than a signal number, cgroup/dispatch-scope memory check added to Phase 2A,
surge threshold corrected to the detector's committed 3-in-5-minutes with the slow-burn
corollaries)  
**Target Audience:** Agents investigating crash alerts  
**Purpose:** Fast crash classification and response decisions
