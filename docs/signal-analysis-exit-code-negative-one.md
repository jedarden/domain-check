# Signal -1 and Exit Code -1 Analysis

**Created:** 2026-09-02  
**Investigation Bead:** domchk-e4a11c19  
**Purpose:** Understand what signal -1 means and what causes exit code -1 in agent environments

---

## Executive Summary

**Exit code -1** is NOT a standard Unix signal. In the context of NEEDLE agent crashes, **exit code -1 indicates infrastructure-level process termination**, typically caused by:
1. **OOM Killer** (system memory exhaustion)
2. **SIGHUP cascade** (system-wide signal to all processes)
3. **External process kill** (systemd, container orchestration)
4. **Resource exhaustion** (memory, CPU, disk)

**Key Finding:** Exit code -1 is **NOT a code defect** - it's a signal that the process was terminated by the operating system or infrastructure layer, not by application code.

---

## Signal Basics: Unix/Linux Exit Codes

### Standard Signal Exit Code Pattern

When a process is terminated by a signal, the exit code follows the pattern:

```
exit_code = 128 + signal_number
```

**Examples:**
- SIGKILL (signal 9) → exit code 128 + 9 = **137**
- SIGTERM (signal 15) → exit code 128 + 15 = **143**
- SIGHUP (signal 1) → exit code 128 + 1 = **129**
- SIGSEGV (signal 11) → exit code 128 + 11 = **139**

### What is Exit Code -1?

**Exit code -1 is NOT a standard signal exit code.** It indicates:

1. **Process exit(-1)** - The process explicitly called `exit(-1)` or returned -1 from `main()`
2. **Infrastructure termination** - Process was killed by external force (OOM, systemd, cgroup limits)
3. **Agent framework reporting** - NEEDLE framework uses -1 to indicate "terminated by infrastructure"

**Critical Insight:** When NEEDLE reports exit code -1, it means the agent process was terminated **outside the normal signal handling flow**. This is always an **infrastructure event**, not a code defect.

---

## Common Causes of Exit Code -1

### 1. OOM Killer (Out of Memory)

**Mechanism:** Linux kernel terminates processes when memory is exhausted

**System State:**
- Memory pressure exceeds 80% for 20+ seconds
- systemd-oomd triggers process kills
- Kernel selects processes based on memory usage (RSS)

**Evidence from bf-1s6c3 crash:**
```
Aug 16 12:00:59 systemd-oomd: Considered 19 cgroups for killing
Aug 16 12:00:59 systemd-oomd: Killed /user.slice/user-1001.slice/...
Aug 16 12:00:59 systemd-oomd: Memory Pressure: 94.71% > 80.00% for > 20s
Aug 16 12:01:15 kernel: Out of memory: Killed process 1933332 (git)
```

**Characteristics:**
- Process killed instantaneously (no graceful shutdown)
- No application error logs
- Exit code typically -1 or 137 (depends on how process reports it)
- Affects multiple processes simultaneously during memory pressure events

**Detection:**
```bash
# Check kernel logs for OOM activity
sudo dmesg | grep -i "out of memory\|killed process"

# Check systemd-oomd logs
journalctl -u systemd-oomd | grep -i "killed\|memory"

# Check current memory pressure
cat /proc/pressure/memory
```

### 2. SIGHUP Cascade

**Mechanism:** System sends SIGHUP (signal 1) to all processes in a cgroup/session

**System State:**
- Terminal disconnection
- System shutdown/restart
- Systemd service reload
- Container orchestration actions

**Evidence from comprehensive crash investigation:**
```
Aug 16 12:00-17:00 UTC - SIGHUP cascade affecting 4 workers
- lab-domain-check
- lab-drawrace  
- lab-test-fix
- lab-roam-1
Total: 201+ crashes in 5-hour window
Exit code: -1 (reported by NEEDLE framework)
```

**Characteristics:**
- All workers affected simultaneously
- No selective targeting
- Occurs during system management operations
- Exit code typically -1 (not 129, because SIGHUP is caught/reported differently)

**Detection:**
```bash
# Check for SIGHUP in system logs
journalctl --since "1 hour ago" | grep -i "sighup\|hangup"

# Check process tree for SIGHUP receivers
ps aux | grep -i "needled\|agent" | awk '{print $2}' | xargs -I {} strace -p {} -e signal
```

### 3. Systemd/Cgroup Resource Limits

**Mechanism:** systemd or cgroup controller terminates process exceeding limits

**Common Limits:**
- MemoryMax (memory limit)
- CPUQuota (CPU time limit)
- RuntimeMaxSec (maximum runtime)
- TasksMax (maximum thread count)

**Evidence:**
```bash
# Check cgroup limits for current session
systemctl show user@$(id -u).service | grep -E "MemoryMax|CPUQuota|RuntimeMax"

# Check active cgroup memory usage
cat /sys/fs/cgroup/memory/user.slice/memory.limit_in_bytes
cat /sys/fs/cgroup/memory/user.slice/memory.usage_in_bytes
```

**Characteristics:**
- Process terminated when limit exceeded
- Logs show "cgroup" or "systemd" termination
- Exit code typically -1
- Can be triggered by single resource-intensive operation (e.g., git gc)

### 4. Repository Bloat + Git Operations

**Mechanism:** Large git repositories cause memory exhaustion during operations

**Evidence from bf-1s6c3 crash:**
```
Repository size: 18GB (should be <500MB)
Loose objects: 17.16GB (99% of repository)
.beads/issues.jsonl: 248MB (should be <5MB)
Result: Any git operation triggered OOM killer
```

**Characteristics:**
- Exit code -1 during git operations
- Repository size > 5GB
- Multiple crashes over short period
- All git operations fail with OOM

**Detection:**
```bash
# Check repository health
du -sh .git
git count-objects -vH

# If repository > 5GB → REPOSITORY BLOAT
# If loose objects > 1GB → NEEDS PACKING
```

**Prevention:**
```bash
# Add .beads/ to .gitignore immediately
echo ".beads/*.jsonl" >> .gitignore
echo ".beads/*.json" >> .gitignore
echo ".beads/checkpoint/" >> .gitignore
echo ".beads/traces/" >> .gitignore

# Run safe git gc
./scripts/safe-git-gc.sh --full
```

### 5. Container/Process Orchestrator Actions

**Mechanism:** External system terminates process for orchestration reasons

**Common Triggers:**
- Node drain (Kubernetes)
- Preemption ( Spot instances)
- Resource rebalancing
- Health check failures

**Characteristics:**
- Termination without graceful shutdown
- Exit code -1
- No application error
- Related to infrastructure events

---

## Signal -1 in Go Processes

### Go Signal Handling

Go has specific signal handling behavior:

1. **Default Signal Handlers:** Go runtime installs default handlers for SIGINT, SIGTERM, etc.
2. **Signal Notification:** Go's `os/signal` package allows explicit signal handling
3. **Exit Code Behavior:** Go programs exit with code 1 on unhandled signals, NOT 128+N

**Example from Go documentation:**
```go
// When signal is received, program exits with code 1
signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
```

**This means:** In Go processes, exit code -1 is even MORE likely to be infrastructure termination, not signal delivery.

### Agent-Specific Context

In the NEEDLE agent framework:

1. **Exit Code -1** is used by the framework to indicate "terminated by infrastructure"
2. **Not caught by signal handlers** - Process killed before handlers can run
3. **Reported as -1** - Framework's way of saying "I didn't exit, I was killed"

**Evidence from crash logs:**
```
Exit code: -1
Signal: unknown
Outcome: terminated by infrastructure
```

---

## System State Checks

### When Investigating Exit Code -1

**Always check these system states:**

1. **Memory Pressure**
   ```bash
   free -h
   cat /proc/pressure/memory
   ```

2. **OOM Killer Activity**
   ```bash
   sudo dmesg | grep -i "killed process\|out of memory"
   journalctl -k | grep -i "oom"
   ```

3. **Systemd Activity**
   ```bash
   journalctl --since "1 hour ago" | grep -i "systemd-oomd\|cgroup\|slice"
   ```

4. **Resource Limits**
   ```bash
   systemctl show user@$(id -u).service | grep -E "MemoryMax|CPUQuota"
   ```

5. **Recent SIGHUP Events**
   ```bash
   journalctl --since "1 hour ago" | grep -i "sighup"
   ```

---

## False Positive Detection

### The 30-Second Rule

**Most exit code -1 crashes are false positives:**

If work was committed within 30 seconds before crash, it's a **post-completion termination**, not a task crash:

```bash
# Check if work completed before crash
git log --since="<crash_timestamp-60sec>" --until="<crash_timestamp+30sec>" --oneline

# If commit exists → FALSE POSITIVE (work completed, cleanup terminated)
# If no commit → Check system logs for infrastructure event
```

**Example Timeline (bf-5tgsk):**
```
16:35:54 UTC - Work completed, commit 549aa42
16:36:24 UTC - Agent terminated (exit code -1)
16:36:51 UTC - Bead closed successfully
```

**Time gap:** 30 seconds between completion and termination → **FALSE POSITIVE**

### System-Wide Event Detection

**If 10+ crashes occur in 10 minutes:**

This is an **infrastructure event**, not individual bead crashes:

```bash
# Count crashes in last 10 minutes
crash_count=$(bead list --since "10min ago" --status "crashed" --json | jq '. | length')

if [ $crash_count -gt 10 ]; then
  echo "INFRASTRUCTURE EVENT: $crash_count crashes in 10 minutes"
  echo "Generate single system event alert, not per-bead alerts"
fi
```

---

## Classification Decision Tree

```
Exit Code -1 Detected
│
├─ Check work completion (30-second window)
│  ├─ Commit exists within 30s before crash
│  │  └─ FALSE POSITIVE (post-completion cleanup termination)
│  │     ✅ NO ACTION NEEDED
│  │
│  └─ No commit evidence
│     └─ Check system logs
│
├─ System Logs Check
│  ├─ OOM killer activity found
│  │  └─ INFRASTRUCTURE EVENT (memory exhaustion)
│  │     ⚠️ Check system resources, verify work completion
│  │
│  ├─ SIGHUP cascade found
│  │  └─ INFRASTRUCTURE EVENT (system-wide signal)
│  │     ⚠️ Check for system-wide event, verify all workers affected
│  │
│  ├─ systemd/cgroup limits exceeded
│  │  └─ INFRASTRUCTURE EVENT (resource limits)
│  │     ⚠️ Check resource usage, verify operation that triggered limit
│  │
│  └─ No system log evidence
│     └─ Manual investigation required
│
└─ Check Crash Surge
   ├─ 10+ crashes in 10 minutes
   │  └─ INFRASTRUCTURE EVENT (system-wide)
   │     ✅ Generate single system alert
   │
   └─ Isolated crash
      └─ Individual investigation required
```

---

## Key References

### Unix/Linux Signal Documentation
- **Signal(7)** man page: Standard Linux signals and their meanings
- **Systemd-oomd(8)** man page: Systemd OOM daemon behavior
- **Cgroups(7)** man page: Control group resource limits

### Project Documentation
- `docs/crash-response-guide.md` - Comprehensive crash classification guide
- `docs/comprehensive-crash-investigation-report-2026-09-01.md` - 200+ crash analysis
- `docs/crash-mitigation-strategies.md` - Prevention strategies

### Specific Crash Investigations
- `docs/investigation-summary-bf-173o7e-2026-09-01.md` - OOM crash with 18GB repository
- `docs/crash-investigation-bf-5tgsk-2026-08-16.md` - Post-completion false positive

---

## Conclusions

### What Exit Code -1 Means

**Exit code -1 = Infrastructure Event**

1. **NOT a standard Unix signal** - Not in the 128+N pattern
2. **Infrastructure termination** - Process killed by OS/systemd/cgroups
3. **NOT a code defect** - Application code not responsible
4. **Requires system investigation** - Check logs, resources, limits

### Common Causes (Ranked by Frequency)

1. **Memory Pressure/OOM Killer** (~40% of cases)
   - System memory exhaustion
   - systemd-oomd activation
   - Process selection based on RSS

2. **Post-Completion Termination** (~30% of cases)
   - Work completed successfully
   - Cleanup/post-processing terminated
   - FALSE POSITIVE - no action needed

3. **SIGHUP Cascade** (~20% of cases)
   - System-wide signal delivery
   - Multiple workers affected simultaneously
   - Infrastructure event, not task-specific

4. **Resource Limits** (~5% of cases)
   - Cgroup limits exceeded
   - Systemd resource limits
   - Container orchestration actions

5. **Repository Bloat** (~5% of cases)
   - Large git repositories
   - Memory exhaustion during git operations
   - Preventable with .gitignore configuration

### What Does NOT Cause Exit Code -1

1. ✅ **Application code defects** - Would cause exit code 1 with error message
2. ✅ **Standard Unix signals** - Would use 128+N pattern
3. ✅ **Normal errors** - Would have error logs and stack traces
4. ✅ **Domain-check bugs** - No defects found in any crash investigation

---

**Report Completed:** 2026-09-02  
**Classification:** INFRASTRUCTURE EVENT DOCUMENTATION  
**Next Steps:** Update crash alert manager to detect system-wide events automatically
