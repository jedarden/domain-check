# Fix Recommendations: Crash Prevention and Mitigation

**Document Date:** 2026-09-01  
**Based On:** Root cause analysis investigations (domchk-c7176067, domchk-a1c9d590)  
**Confidence Level:** HIGH  
**Classification:** INFRASTRUCTURE FIXES (primary) + TOOL IMPROVEMENTS (secondary)

---

## Executive Summary

**Critical Finding:** The exit code -1 crashes are **NOT caused by defects in the domain-check codebase**. The code is robust with proper signal handling, bounded resources, and graceful shutdown. The crashes are caused by **infrastructure-level resource exhaustion** and **system-level process termination**.

**Root Causes Identified:**
1. **Primary:** Memory pressure triggering systemd-oomd (94.71% pressure vs 80% threshold)
2. **Secondary:** CPU saturation (4.46x load on 7 cores)
3. **Tertiary:** System-wide SIGHUP cascade from external events

**Fix Strategy:** Three-tiered approach focusing on infrastructure resilience, monitoring/alerting, and tool improvements. No code changes required for domain-check.

---

## Part 1: Infrastructure Fixes (High Priority)

### Fix 1.1: Systemd-oomd Threshold Adjustment

**Problem:** Memory pressure threshold (80%) is too aggressive relative to available capacity (62GB total, 52GB available after cleanup).

**Current Configuration:**
```
Memory Pressure: 94.71% > 80.00% threshold → OOM kill triggered
Total Memory: 62GB
Available After Cleanup: 52GB (83% free)
```

**Recommended Fix:**

Adjust systemd-oomd configuration to use absolute memory limits instead of pressure percentage:

```bash
# /etc/systemd/system.conf.d/oomd.conf
[Manager]
# Use absolute memory limit (leave 20GB free)
DefaultMemoryAccounting=yes
DefaultMemoryMax=80%

# For user slice
[Service]
MemoryAccounting=yes
MemoryMax=50G  # Leave 12GB headroom
MemoryMaxSwap=0 # Disable swap to prevent thrashing

# Pressure-based fallback (less aggressive)
ManagedOOMMemoryPressure=limit
ManagedOOMMemoryPressureLimit=90%  # Increase from 80%
```

**Implementation Steps:**
1. Create systemd drop-in configuration
2. Test with memory stress workload
3. Monitor for 48 hours under normal load
4. Validate OOM behavior under controlled memory pressure

**Effort:** 2 hours  
**Risk Level:** LOW (reversible, no data loss)  
**Testing Strategy:** 
- Simulate memory pressure with `stress-ng --vm 2 --vm-bytes 20G`
- Monitor systemd-oomd logs: `journalctl -u systemd-oomd`
- Validate domain-check continues under 45GB memory pressure

---

### Fix 1.2: Memory Pressure Alerting (Preemptive)

**Problem:** No alerting before memory pressure triggers OOM killer. The system reaches 94.71% before any intervention.

**Recommended Fix:**

Implement memory pressure alerting at 70% threshold (14% headroom before OOM):

```yaml
# Prometheus alerting rules
groups:
  - name: memory_pressure
    rules:
      - alert: HighMemoryPressure
        expr: node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes < 0.30
        for: 5m
        labels:
          severity: warning
          component: infrastructure
        annotations:
          summary: "Memory pressure above 70% on {{ $labels.instance }}"
          description: "Memory available is {{ $value }}% - investigate potential memory leaks"
          
      - alert: CriticalMemoryPressure
        expr: node_memory_MemAvailable_bytes / node_memory_MemTotal_bytes < 0.20
        for: 2m
        labels:
          severity: critical
          component: infrastructure
        annotations:
          summary: "Memory pressure above 80% on {{ $labels.instance }}"
          description: "Immediate action required - risk of OOM kill"
```

**Implementation Steps:**
1. Add memory pressure metrics to node_exporter (if not present)
2. Deploy alerting rules to Prometheus
3. Configure AlertManager routing to operations channel
4. Create runbook for memory pressure response

**Effort:** 4 hours  
**Risk Level:** LOW (monitoring only)  
**Testing Strategy:**
- Simulate gradual memory pressure increase
- Validate alert fires at 70% threshold
- Confirm no OOM trigger at 70% (only at 90%)

---

### Fix 1.3: CPU Saturation Detection and Throttling

**Problem:** No detection or prevention of CPU saturation (4.46x load caused 826 crashes on 2026-08-16).

**Recommended Fix:**

Implement per-worker CPU monitoring with automatic throttling:

```bash
# /usr/local/bin/cpu-guard.sh
#!/bin/bash
THRESHOLD=3.0  # 3x load average
CHECK_INTERVAL=60

while true; do
    LOAD_1MIN=$(awk '{print $1}' /proc/loadavg)
    CORES=$(nproc)
    LOAD_RATIO=$(echo "$LOAD_1MIN / $CORES" | bc -l)
    
    if (( $(echo "$LOAD_RATIO > $THRESHOLD" | bc -l) )); then
        echo "CPU saturation detected: ${LOAD_RATIO}x load"
        # Pause new work dispatch
        systemctl stop needle-dispatch@*.service
        # Signal active workers to drain gracefully
        pkill -SIGUSR1 needle-worker
        break
    fi
    
    sleep $CHECK_INTERVAL
done
```

**Integration with Needle:**
Add CPU-aware work dispatch:
```python
def can_dispatch_new_work():
    load_avg = os.getloadavg()[0]
    cores = os.cpu_count()
    load_ratio = load_avg / cores
    
    if load_ratio > 3.0:
        logger.warning(f"CPU saturation {load_ratio:.2f}x - pausing dispatch")
        return False
    return True
```

**Implementation Steps:**
1. Deploy cpu-guard.sh as systemd service
2. Add CPU checks to Needle dispatch logic
3. Configure graceful worker drain on SIGUSR1
4. Monitor load average during normal operations

**Effort:** 8 hours  
**Risk Level:** MEDIUM (affects work scheduling)  
**Testing Strategy:**
- Simulate CPU load with `stress-ng --cpu 7 --cpu-method fft`
- Validate dispatch pauses at 3.0x load
- Confirm active workers complete before new dispatch resumes

---

## Part 2: Monitoring and Detection Improvements (Medium Priority)

### Fix 2.1: Crash Pattern Recognition

**Problem:** 40% of crashes are post-completion false positives, but system generates duplicate investigations.

**Recommended Fix:**

Implement crash pattern classification before alert generation:

```python
def classify_crash(bead_id, metadata, repo_state):
    """Classify crash type to reduce false positives."""
    
    # Pattern 1: Post-completion false positive
    last_commit = get_last_commit_time(repo_state)
    crash_time = metadata['crash_timestamp']
    
    if crash_time > last_commit + timedelta(minutes=1):
        if has_deliverables(repo_state):
            return "POST_COMPLETION_FALSE_POSITIVE"
    
    # Pattern 2: Transient crash with self-healing
    if has_successful_retry(bead_id):
        return "TRANSIENT_SELF_HEALED"
    
    # Pattern 3: System-wide event
    if is_concurrent_crash_window(crash_time):
        return "SYSTEM_WIDE_EVENT"
    
    # Pattern 4: Genuine crash
    return "GENUINE_CRASH"

def generate_alert(bead_id, classification):
    """Only alert for genuine crashes."""
    if classification == "GENUINE_CRASH":
        create_investigation_bead(bead_id)
    else:
        log_crash_pattern(bead_id, classification)
        update_metrics(crash_type=classification)
```

**Implementation Steps:**
1. Add classification logic to Needle crash handler
2. Implement commit-time comparison (git log)
3. Add concurrent crash detection (time-window clustering)
4. Update metrics to track classification distribution

**Effort:** 12 hours  
**Risk Level:** LOW (reduces alert noise)  
**Testing Strategy:**
- Replay historical crashes (2026-08-16 dataset)
- Validate classification accuracy against known false positives
- Confirm genuine crashes still trigger alerts

---

### Fix 2.2: Alert Deduplication

**Problem:** 60% of crash alerts are duplicates (30+ verification reports for same crash).

**Recommended Fix:**

Implement crash fingerprinting and deduplication:

```python
def crash_fingerprint(bead_id, exit_code, signal, workspace):
    """Generate unique fingerprint for crash."""
    
    # Normalized components
    workspace_norm = os.path.basename(workspace)
    exit_norm = exit_code if exit_code != -1 else "signal_-1"
    
    # Time-based bucket (hourly windows)
    crash_time = get_crash_timestamp(bead_id)
    time_bucket = crash_time.replace(minute=0, second=0, microsecond=0)
    
    return f"{workspace_norm}:{exit_norm}:{time_bucket.isoformat()}"

def should_create_alert(bead_id):
    """Check if crash already investigated."""
    fingerprint = crash_fingerprint(bead_id, ...)
    
    if redis.exists(f"crack_fp:{fingerprint}"):
        logger.info(f"Duplicate crash {fingerprint} - skipping alert")
        return False
    
    # Mark as investigated (TTL 7 days)
    redis.setex(f"crack_fp:{fingerprint}", 86400*7, "1")
    return True
```

**Implementation Steps:**
1. Add fingerprinting to crash detection
2. Deploy Redis for deduplication state (24hr TTL)
3. Add metrics for duplicate rate tracking
4. Create dashboard showing crash uniqueness

**Effort:** 8 hours  
**Risk Level:** LOW (improves efficiency)  
**Testing Strategy:**
- Replay 2026-08-16 crash dataset
- Verify duplicate detection reduces alerts by 60%
- Validate unique crashes still create alerts

---

### Fix 2.3: Work Completion Detection

**Problem:** No detection of successful task completion before crash time.

**Recommended Fix:**

Add work completion validation before crash alert generation:

```python
def validate_task_failure(bead_id, workspace, crash_time):
    """Verify task was not completed before crash."""
    
    repo = git.Repo(workspace)
    
    # Check for commits after bead creation and before crash
    bead_created = get_bead_created_time(bead_id)
    commits = list(repo.iter_commits(since=bead_created, until=crash_time))
    
    if commits:
        # Work was committed - likely post-completion false positive
        latest_commit = commits[0]
        logger.info(
            f"Bead {bead_id} has commits before crash: "
            f"{latest_commit.hexsha[:8]} @ {latest_commit.committed_datetime}"
        )
        return False  # Not a genuine crash
    
    # Check for expected deliverables
    deliverables = get_expected_deliverables(bead_id)
    if deliverables:
        if all(os.path.exists(os.path.join(workspace, d)) for d in deliverables):
            logger.info(f"Bead {bead_id} deliverables exist - likely false positive")
            return False
    
    return True  # No evidence of completion - genuine crash
```

**Implementation Steps:**
1. Extract deliverable expectations from task descriptions (NLP or manual tagging)
2. Add git commit history check to crash validation
3. Integrate into crash alert generation flow
4. Track false positive rate metrics

**Effort:** 16 hours  
**Risk Level:** LOW (improves accuracy)  
**Testing Strategy:**
- Validate against bf-5tgsk (known post-completion false positive)
- Confirm genuine crashes still pass validation
- Measure reduction in false positive rate

---

## Part 3: Tool Improvements (Low Priority)

### Fix 3.1: Bead Closing Workflow Resilience

**Problem:** Bead close command can fail even with --skip-verify (bf-173o7e experienced multiple failures).

**Recommended Fix:**

Improve bead-rs close command resilience:

```go
func (b *Bead) Close(opts CloseOptions) error {
    // Attempt standard close
    err := b.closeStandard(opts)
    if err == nil {
        return nil
    }
    
    // Fallback 1: Force close with state override
    if opts.SkipVerify {
        err = b.closeForce(opts)
        if err == nil {
            return nil
        }
    }
    
    // Fallback 2: Direct database update
    if opts.AllowDirectDB {
        err = b.closeDirectDB(opts)
        if err == nil {
            return nil
        }
    }
    
    // All methods failed - return detailed error
    return fmt.Errorf("close failed after all fallbacks: %w", err)
}

func (b *Bead) closeDirectDB(opts CloseOptions) error {
    // Emergency fallback: update DB directly
    stmt := `UPDATE beads SET status = 'closed', closed_at = NOW() 
             WHERE id = $1 AND status != 'closed'`
    
    result, err := b.db.Exec(stmt, b.ID)
    if err != nil {
        return fmt.Errorf("direct DB close failed: %w", err)
    }
    
    rows, _ := result.RowsAffected()
    if rows == 0 {
        return fmt.Errorf("bead %s already closed or not found", b.ID)
    }
    
    log.Warn("Closed bead via direct DB fallback", "bead", b.ID)
    return nil
}
```

**Implementation Steps:**
1. Add fallback close methods to bead-rs
2. Add --allow-direct-db flag for emergency use
3. Improve error messages with specific failure reasons
4. Add retry logic for transient failures

**Effort:** 6 hours  
**Risk Level:** MEDIUM (modifies bead state management)  
**Testing Strategy:**
- Test all fallback paths in controlled environment
- Verify DB integrity after fallback close
- Validate no bead data corruption

---

### Fix 3.2: Max Turns Adjustment for Complex Workflows

**Problem:** 30-turn limit too low for complex post-task workflows (bf-173o7e hit limit during bead closing).

**Recommended Fix:**

Implement dynamic turn limits based on task complexity:

```python
def calculate_turn_limit(task_type, estimated_duration):
    """Calculate appropriate turn limit for task."""
    
    BASE_LIMIT = 30
    COMPLEX_MULTIPLIER = {
        'simple': 1.0,
        'standard': 1.5,
        'complex': 2.0,
        'multi_phase': 3.0,
    }
    
    # Estimate complexity from task description
    complexity = classify_task_complexity(task_type, estimated_duration)
    multiplier = COMPLEX_MULTIPLIER.get(complexity, 1.0)
    
    return int(BASE_LIMIT * multiplier)

# Task classification heuristics
def classify_task_complexity(task_type, estimated_duration):
    if estimated_duration > timedelta(hours=2):
        return 'multi_phase'
    elif 'investigation' in task_type or 'debug' in task_type:
        return 'complex'
    elif estimated_duration > timedelta(minutes=30):
        return 'standard'
    else:
        return 'simple'
```

**Implementation Steps:**
1. Add task complexity classifier to Needle
2. Implement dynamic turn limit calculation
3. Add turn limit logging for monitoring
4. Adjust based on historical turn usage patterns

**Effort:** 4 hours  
**Risk Level:** LOW (increases limits, no breaking changes)  
**Testing Strategy:**
- Validate simple tasks still use 30 turns
- Confirm complex tasks get adequate turns
- Monitor turn exhaustion rate after deployment

---

## Part 4: Operational Procedures (Zero Code Changes)

### Fix 4.1: Memory Pressure Response Runbook

**Recommended Fix:**

Create operational runbook for memory pressure incidents:

```markdown
# Memory Pressure Response Runbook

## Detection
- Alert: HighMemoryPressure (70% threshold)
- Symptom: Memory available < 30% of total

## Immediate Actions (5 min)
1. Check memory usage: `free -h`
2. Identify top consumers: `ps aux --sort=-%mem | head -20`
3. Check for git gc operations: `ps aux | grep git`
4. Review Needle worker count: `systemctl status needle-*`

## Escalation Actions (15 min)
1. Pause new work dispatch: `systemctl stop needle-dispatch@*.service`
2. Allow active workers to drain gracefully
3. Consider terminating large git processes if safe

## Recovery Actions (30 min)
1. Monitor memory pressure drop
2. Resume work dispatch when < 60% pressure
3. Investigate root cause (memory leak? workload spike?)

## Prevention
1. Review workload patterns
2. Consider resource limits for large operations
3. Schedule heavy operations during low-traffic periods
```

**Implementation Steps:**
1. Document runbook in operations handbook
2. Train on-call team on procedures
3. Add runbook link to alert annotations
4. Quarterly review and update

**Effort:** 4 hours  
**Risk Level:** NONE (operational procedure)  
**Testing Strategy:**
- Tabletop exercise with ops team
- Validate procedures against historical incidents

---

### Fix 4.2: Scheduled Maintenance for Large Git Operations

**Problem:** Git gc operations can consume significant memory (bf-173o7e used 1.3GB, can scale higher for larger repos).

**Recommended Fix:**

Establish scheduled maintenance windows for large git operations:

```yaml
# Maintenance schedule
maintenances:
  - name: git_gc_aggressive
    schedule: "0 3 * * 0"  # 3 AM Sundays
    repositories:
      - /home/coding/domain-check
      - /home/coding/other-large-repos
    operation: git gc --aggressive --prune=now
    max_runtime: 6h
    memory_limit: 4G
    cpu_limit: 4
    
  - name: git_gc_standard
    schedule: "0 2 * * 1-6"  # 2 AM Mon-Sat
    repositories:
      - /home/coding/all-repos
    operation: git gc
    max_runtime: 1h
    memory_limit: 2G
    cpu_limit: 2
```

**Implementation Steps:**
1. Create systemd timers for scheduled gc operations
2. Add resource limits (cgroups) to prevent memory exhaustion
3. Configure alerting for maintenance operations
4. Document maintenance schedule in ops handbook

**Effort:** 4 hours  
**Risk Level:** LOW (improves predictability)  
**Testing Strategy:**
- Test maintenance schedule on staging repos
- Validate resource limits prevent OOM
- Confirm no impact to daytime operations

---

## Part 5: Validation and Testing Strategy

### Validation Plan

**Phase 1: Infrastructure Fixes (Week 1)**
1. Deploy systemd-oomd configuration
2. Enable memory pressure alerting
3. Test with controlled memory pressure (stress-ng)
4. Validate no OOM triggers below 90% threshold

**Phase 2: Monitoring Improvements (Week 2-3)**
1. Deploy crash classification logic
2. Enable alert deduplication
3. Implement work completion detection
4. Test against historical crash dataset (2026-08-16)

**Phase 3: Tool Improvements (Week 4)**
1. Update bead-rs with fallback close methods
2. Implement dynamic turn limits
3. Test with complex workflow scenarios
4. Validate no regression in normal operations

**Phase 4: Operational Procedures (Ongoing)**
1. Train ops team on runbooks
2. Schedule maintenance windows
3. Monitor metrics and adjust thresholds
4. Quarterly review of crash patterns

### Success Criteria

**Infrastructure Metrics:**
- Zero OOM kills below 90% memory pressure
- CPU saturation detection within 2 minutes
- Memory pressure alerts trigger at 70% threshold

**Alert Quality Metrics:**
- False positive rate < 10% (currently ~40%)
- Duplicate alert rate < 5% (currently ~60%)
- Genuine crash alert rate > 95% accuracy

**Operational Metrics:**
- Mean time to detection < 5 minutes for saturation
- Mean time to response < 15 minutes for alerts
- Zero data loss during crash events

**Code Quality Metrics:**
- No code defects introduced (maintain current code quality)
- All tests passing (go test ./...)
- No regression in signal handling or resource management

---

## Part 6: Risk Assessment and Mitigation

### Risk Matrix

| Fix | Risk Level | Impact | Mitigation |
|-----|------------|--------|------------|
| systemd-oomd config | LOW | High system stability | Reversible, test under controlled load |
| Memory alerting | LOW | Early warning | Monitoring only, no system changes |
| CPU throttling | MEDIUM | Work scheduling delays | Gradual dispatch, graceful drain |
| Crash classification | LOW | Reduced alert noise | Replay historical data, validate accuracy |
| Alert deduplication | LOW | Improved efficiency | TTL-based state, auto-recovery |
| Work completion detection | LOW | Better accuracy | Conservative validation, manual review |
| Bead close fallbacks | MEDIUM | Improved reliability | Multiple fallbacks, extensive testing |
| Dynamic turn limits | LOW | Adequate turns for complexity | Conservative multipliers, monitor usage |

### Rollback Plan

All fixes are designed to be reversible:

1. **Infrastructure changes**: Configuration files can be reverted
2. **Monitoring changes**: Disable alerting rules, metrics remain
3. **Code changes**: Git revert, bead-rs backward compatible
4. **Operational procedures**: Document previous procedures, gradual rollout

### Testing Requirements

**Pre-deployment:**
- All fixes tested in staging environment
- Memory pressure simulation tests pass
- CPU saturation handling validated
- Crash classification accuracy > 95% on historical data

**Post-deployment:**
- Monitor crash rate for 30 days
- Validate false positive reduction
- Confirm no regression in genuine crash detection
- Review alert volume and response times

---

## Part 7: Implementation Timeline

### Week 1: Critical Infrastructure
- Deploy systemd-oomd configuration (2h)
- Enable memory pressure alerting (4h)
- Document and train on runbooks (4h)
- **Total: 10 hours**

### Week 2-3: Monitoring Improvements
- Implement crash classification (12h)
- Add alert deduplication (8h)
- Implement work completion detection (16h)
- **Total: 36 hours**

### Week 4: Tool Improvements
- Bead close fallbacks (6h)
- Dynamic turn limits (4h)
- Testing and validation (8h)
- **Total: 18 hours**

### Ongoing: Operations and Monitoring
- Scheduled maintenance (4h setup)
- Quarterly runbook reviews (2h)
- Continuous monitoring (ongoing)
- **Total: 6 hours setup + ongoing**

**Total Effort: 70 hours (1.75 weeks @ 40h/week)**

---

## Part 8: Long-term Considerations

### Future Enhancements

**1. Predictive Memory Management**
- Machine learning model to predict memory usage patterns
- Preemptive work throttling before OOM threshold
- Automated scaling decisions based on workload

**2. Crash Prevention**
- Pre-flight checks before large operations (git gc, builds)
- Resource reservation for critical operations
- Worker isolation via cgroups or containers

**3. Enhanced Monitoring**
- Real-time crash pattern dashboard
- Automated classification confidence scores
- Integration with incident management systems

### Monitoring Dashboard Metrics

**Recommended Prometheus Queries:**

```promql
# Memory pressure trend
rate(node_memory_MemAvailable_bytes[5m])

# CPU saturation
100 * (avg(rate(node_cpu_seconds_total{mode!="idle"}[5m])) by (instance) / count by (instance) (node_cpu_seconds_total))

# Crash rate by classification
rate(needle_crashes_total[1h]) by (classification)

# False positive rate
rate(needle_crashes_total{classification="false_positive"}[24h]) / rate(needle_crashes_total[24h])

# Duplicate alert rate
rate(needle_duplicate_alerts_total[24h]) / rate(needle_alerts_total[24h])
```

---

## Conclusions

### Summary of Recommendations

**NO CODE CHANGES REQUIRED FOR DOMAIN-CHECK**

The domain-check codebase is robust and properly implemented:
- ✅ Proper signal handling (SIGINT/SIGTERM)
- ✅ Graceful shutdown with 15s drain timeout
- ✅ Bounded LRU cache with TTL eviction
- ✅ Per-registry concurrency limits
- ✅ HTTP timeouts on all connections
- ✅ No unbounded goroutines or resource leaks

**REQUIRED FIXES: INFRASTRUCTURE AND MONITORING**

1. **Infrastructure Fixes (High Priority):**
   - Adjust systemd-oomd thresholds (70% alert, 90% OOM)
   - Implement CPU saturation detection and throttling
   - Schedule large operations during maintenance windows

2. **Monitoring Improvements (Medium Priority):**
   - Crash pattern classification before alert generation
   - Alert deduplication to prevent duplicate investigations
   - Work completion detection to reduce false positives

3. **Tool Improvements (Low Priority):**
   - Bead closing workflow resilience with fallbacks
   - Dynamic turn limits for complex workflows

4. **Operational Procedures (Zero Code Changes):**
   - Memory pressure response runbook
   - Scheduled maintenance for large git operations

### Expected Outcomes

**After Implementation:**
- False positive rate: 40% → < 10%
- Duplicate alert rate: 60% → < 5%
- OOM kills: Eliminated below 90% memory pressure
- CPU saturation: Detected and mitigated within 2 minutes
- Investigation efficiency: 70% reduction in false positive workload

**System Stability:**
- Zero crashes caused by infrastructure resource exhaustion
- Preemptive detection of memory pressure and CPU saturation
- Graceful handling of system-wide signal cascades
- Improved resilience for large git operations

**Operational Efficiency:**
- Reduced alert noise allows focus on genuine issues
- Automated classification reduces manual triage
- Deduplication prevents redundant investigations
- Clear runbooks enable faster incident response

---

**Document Status:** ✅ COMPLETE  
**Confidence Level:** HIGH  
**Recommendation:** Implement infrastructure fixes immediately (Week 1), monitoring improvements (Week 2-3), tool improvements (Week 4)  
**Risk Assessment:** LOW overall risk, HIGH value in preventing false positives  
**Success Criteria:** False positive rate < 10%, zero OOM below 90% memory pressure, genuine crash detection > 95% accuracy

---

**Document Prepared:** 2026-09-01  
**Based On Investigations:** domchk-c7176067, domchk-a1c9d590, bf-173o7e, bf-5tgsk  
**References:** 
- Crash investigation documents (2026-08-16 to 2026-09-01)
- System logs (journalctl, systemd-oomd)
- Code analysis (domain-check signal handling, resource management)
- Historical crash data (826 crashes on worst day, 201+ during SIGHUP cascade)

---

## Part 9: Implementation Validation (2026-09-05, domchk-60407475)

Each recommendation was checked against what actually ships on this box today (verified
2026-09-05). Summary: **5 implemented (mostly by different means than written), 1 partially
implemented, 2 not actionable as written, 1 must NOT be implemented, 2 still open.**

| Fix | Status as written | Verified state on 2026-09-05 |
|-----|-------------------|------------------------------|
| 1.1 oomd thresholds | ❌ Not actionable as written | Superseded: consumers bounded instead of retuning the killer |
| 1.2 memory alerting | ✅ Implemented, different mechanism | `scripts/resource-monitor.sh` + `domain-check-resource-monitor.timer` |
| 1.3 CPU guard | ⚠️ Partially implemented; details invalid | Dispatch env caps + `check-cpu-load.sh` + `crash-circuit-breaker.sh` |
| 2.1 crash classification | ✅ Implemented 2026-09-02 | `scripts/crash-classifier.sh`, `crash-alert-manager.sh` |
| 2.2 alert deduplication | ✅ Implemented, not via Redis | `scripts/alert-deduplication.sh` |
| 2.3 completion detection | ✅ Implemented 2026-09-02 | `scripts/verify-work-completion.sh` |
| 3.1 bead close fallbacks | 🚫 Must NOT be implemented as written | See 9.4 |
| 3.2 dynamic turn limits | ⬜ Open, legitimate | Not implemented; lands in needle dispatch config |
| 4.1 memory-pressure runbook | ⬜ Partially open | Crash-response + repo-maintenance guides exist; no dedicated pressure runbook |
| 4.2 scheduled gc windows | ✅ Implemented | `domain-check-git-gc.timer`, `-gc-full.timer` (MemoryMax=4G), `safe-git-gc.sh` |

### 9.1 Fix 1.1 — not actionable as written; superseded by a better mechanism

Two problems. First, the snippet is invalid systemd: `MemoryMax`, `MemoryMaxSwap`, and
`ManagedOOMMemoryPressure` are **unit-level** settings and do nothing under
`system.conf`'s `[Manager]`/`[Service]` sections. Second, this box is **NixOS** — `/etc`
drop-ins are generated/managed by the NixOS activation, so the hand-edited-file procedure
does not apply.

The shipped answer attacks the problem from the other side: bound the *consumers* so the
killer never fires. `scripts/setup-git-gc-config.sh` sets persistent
`pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` (repo-local **and**
global — the chain a bare `git gc` actually sees), worst case ≈3 GiB per pack run, against a
12 GiB dispatch scope. That is what actually prevents a repeat of the bf-173o7e/bf-4x12ec
kills. `ManagedOOMMemoryPressure` tuning remains a valid *optional* follow-up, expressed as a
NixOS declaration.

### 9.2 Fix 1.2 — implemented, different mechanism

No Prometheus/node_exporter path was deployed; alerting ships as
`scripts/resource-monitor.sh` on `domain-check-resource-monitor.timer` (every 5 min,
verified firing 2026-09-05), logging to `.beads/logs/resource-monitor.log` at the 70%
warning / 10 GB thresholds from the workspace CLAUDE.md. The two-tier alert idea is
preserved; the transport differs.

### 9.3 Fix 1.3 — partially implemented; the written details are invalid

- **`systemctl stop needle-dispatch@*.service`** — no such units exist (0 `needle-*` units).
  Dispatches run as `systemd-run` **scopes** under `user@1001.service/needle.slice`.
- **`pkill -SIGUSR1 needle-worker`** — no process of that name; actual comm values are
  `needle` and `needle-transfor…`. `pkill` on a wrong name is a footgun.
- **The Python dispatch hook** targets a Rust codebase (needle-rs); the check would land in
  Rust dispatch code, not Python.

Already in place, and sufficient for the stated goal: every dispatch exports per-worker caps
(`GOMAXPROCS=4`, `GOFLAGS=-p=2`, `CARGO_BUILD_JOBS=2`, `RUST_TEST_THREADS=2`,
`VITEST_MAX_THREADS=2`), and `scripts/check-cpu-load.sh` computes load against `nproc` with
80/90% thresholds, alongside `scripts/crash-circuit-breaker.sh`. The residual gap is
*dispatch gating* on load — worth doing, in the dispatcher, not as a separate
SIGUSR1-signalling shell daemon.

### 9.4 Fix 3.1 — must NOT be implemented as written

`closeDirectDB` issues `UPDATE beads SET status='closed' …` against the store. This violates
the workspace's hard rule that nothing under `.beads/` is hand-edited, and it would desync
`.beads/beads.db` from the git-tracked `.beads/checkpoint/` (nothing flushes implicitly, and
a direct write is invisible to the checkpoint model). The snippet is also doubly wrong for
this codebase: bead-rs is **Rust over SQLite**, while the sample is Go with PostgreSQL
placeholders (`$1`) and `NOW()`.

The premise is also corrected by the preserved trace (see the addendum in
`docs/root-cause-analysis-bf-173o7e-2026-09-01.md` §11.7): the bf-173o7e close failures had a
clear cause — the close script resolved the repo from the shell's cwd (`pdftract`) and the
iad-ci kubeconfig was missing so verification aborted — with the `--skip-verify` bypass
printed in the same output. bead-rs already reports actionable errors. The legitimate
residual work is: resolve the bead's workspace from the bead record rather than cwd,
preflight verification prerequisites, and improve upstream error wording.

### 9.5 Fix 3.2 — open, and the change point is needle's dispatch config

`--max-turns 30` is a per-dispatch parameter on the agent command line, so a complexity-based
limit is a dispatcher-side change, not an agent-side one. Note the practical alternative seen
since: needle's auto-split (template=split) already handles oversized tasks by bead mitosis,
which addresses the same failure without raising limits.

### 9.6 Fix 4.2 — implemented; drop the cron/YAML framing

This box is NixOS — there is no `crontab`. The shipped mechanism is systemd **user** timers:
`domain-check-git-gc.timer` (daily 03:00) and `domain-check-git-gc-full.timer` (Sun 04:00,
unit `MemoryMax=4G`), driving `scripts/safe-git-gc.sh` with checkpoint/resume and
`SAFE_GC_MEMORY_MAX`. All six `domain-check-*` timers verified active on 2026-09-05. The
YAML "maintenance schedule" block is illustrative only. Operational gotcha worth recording:
after editing a timer unit, `systemctl --user daemon-reload` is required or the timer
silently keeps stale state.

### 9.7 Premise correction

Fix 4.2's justification ("bf-173o7e used 1.3GB, can scale higher") understates the observed
failure: the Aug-14 kills were `pack-objects` exceeding the **12 GiB** dispatch scope on
17.20 GiB of loose objects, ~10× the 1.3 GB figure. Sizing guidance should start from the
git-config bounds (§9.1) and the scope `MemoryMax`, not from the 1.3 GB measurement, which
came from a much smaller consolidation.

### 9.8 Success criteria — not yet measurable here

"False positive rate < 10%, duplicate rate < 5%" has no measurement pipeline behind it; the
closest artifact is `scripts/test-crash-alert-fixes.sh` (12/12 per the workspace CLAUDE.md).
Recording classification/duplicate counters in the resource-monitor log would make Part 5's
criteria checkable.
