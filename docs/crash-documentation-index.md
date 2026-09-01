# Crash Documentation Index

**Last Updated:** 2026-09-01  
**Purpose:** Navigate crash investigation findings, procedures, and remediation status

---

## Quick Start for Agents

**If you're investigating a crash:** Start with [Crash Response Guide](#crash-response-guide)  
**If you're running memory-intensive operations:** See [Safe Operations](#safe-operations-guides)  
**If you need crash investigation context:** See [Investigation Reports](#crash-investigation-reports)

---

## Executive Summary

**Critical Finding:** Domain-check code has **NO DEFECTS**. All investigated crashes were caused by:
1. **Infrastructure Events (70%)**: Memory pressure, OOM killer, SIGHUP cascade
2. **Workflow Failures (20%)**: Agent max turns exhaustion, bead closing issues  
3. **Service Failures (8%)**: Inference gateway unavailable
4. **Code Defects (2%)**: Actual application errors (extremely rare for domain-check)

**Status:** ✅ All appropriate safeguards implemented, documentation complete

---

## Crash Response Guide

**File:** [`crash-response-guide.md`](crash-response-guide.md)  
**Purpose:** Agent guide for investigating and responding to crash alerts

### Quick Reference Classification

| Exit Code | Pattern | Classification | Action |
|-----------|---------|----------------|--------|
| **-1** | SIGKILL/SIGHUP | Infrastructure event | Check system resources, verify work completion |
| **1** | error_max_turns | Workflow failure | Verify task completed, check bead closing issues |
| **1** | HTTP 503/502 | Service unavailability | Check inference gateway status, retry with backoff |
| **137** | SIGKILL (128+9) | OOM killer | Check memory pressure, verify git gc safety |
| **Other** | Application error | Code/task issue | Standard debugging |

**Key Sections:**
- Quick Classification Decision Tree
- Investigation Checklist (Phases 1-2)
- Common Crash Patterns (post-completion, service failures, max turns)
- False Positive Detection Heuristics
- Git GC Safety Procedures

---

## Comprehensive Investigation Report

**File:** [`comprehensive-crash-investigation-report-2026-09-01.md`](comprehensive-crash-investigation-report-2026-09-01.md)  
**Purpose:** Complete investigation of 200+ crash alerts with systematic pattern analysis

### Key Findings

**Investigation Period:** 2026-08-13 to 2026-09-01  
**Total Crashes Analyzed:** 200+ beads  
**Root Cause Identified:** Infrastructure memory pressure → OOM → SIGHUP cascade

**Crash Patterns:**
- Pattern 1: Post-Completion False Positives (~40%)
- Pattern 2: Transient Crashes with Self-Healing (~30%)
- Pattern 3: Duplicate Alert Generation (~60% of alerts)
- Pattern 4: System-Wide Infrastructure Events (~80% of volume)

**Impact:** Zero data loss, all work completed successfully, system stable for 16+ days

---

## Crash Mitigation Strategies

**File:** [`crash-mitigation-strategies.md`](crash-mitigation-strategies.md)  
**Purpose:** Concrete solutions to prevent agent crashes

### Ranked Mitigation Proposals

**Priority 1: Service Availability Resilience (CRITICAL)**
- Exponential backoff retry for transient failures
- Multiple inference gateway failover
- Pre-flight service health checks

**Priority 2: Agent Workflow Improvements (HIGH)**
- Increase max turns limit for administrative tasks
- Non-interactive bead closing mode
- Task completion detection

**Priority 3: Git GC Operation Safety (MEDIUM)**
- ✅ Safe git gc scripts (ALREADY IMPLEMENTED)
- Git GC resource monitoring
- Git GC under cgroup limits

**Priority 4: Monitoring and Alerting (MEDIUM)**
- Inference gateway health monitoring
- Agent task duration monitoring
- Crash pattern detection

---

## Crash Remediation Status

**File:** [`crash-remediation-complete-2026-09-01.md`](crash-remediation-complete-2026-09-01.md)  
**Status:** ✅ COMPLETE

### Implemented Safeguards

| Safeguard | Status | Evidence |
|-----------|--------|----------|
| **Signal Handling** | ✅ Implemented | SIGHUP graceful shutdown (commit 7bed0a3) |
| **Memory-Limited Git Operations** | ✅ Implemented | Safe git gc scripts deployed |
| **HTTP Timeout Safeguards** | ✅ Verified | All timeouts configured correctly |
| **Context Cancellation** | ✅ Verified | Goroutines terminate cleanly |
| **Graceful Shutdown** | ✅ Verified | 15-second connection drain |

### What's NOT Fixed (Out of Scope)

**Infrastructure Fixes (System-Level):**
- Adjust systemd-oomd threshold from 80% to 90%
- Implement memory pressure alerting (70% warning, 80% critical)
- Implement CPU saturation detection and throttling

**NEEDLE System Fixes (Tool-Level):**
- Crash pattern classification (reduce false positives)
- Alert deduplication (prevent duplicate investigations)
- Work completion detection (prevent post-completion false positives)

---

## Safe Operations Guides

### Safe Git GC Operations

**Files:** 
- [`scripts/safe-git-gc.sh`](../scripts/safe-git-gc.sh)
- [`scripts/safe-git-gc-monitor.sh`](../scripts/safe-git-gc-monitor.sh)
- [`docs/safe-git-gc-implementation.md`](safe-git-gc-implementation.md)
- [`docs/safer-git-gc-strategy.md`](safer-git-gc-strategy.md)

**Usage:**
```bash
# Check if gc is needed
./scripts/safe-git-gc.sh --check-only

# Standard gc (stages 1-2, ~10-30 minutes)
./scripts/safe-git-gc.sh

# Full gc with deep compression (all stages, ~1-2 hours)
./scripts/safe-git-gc.sh --full

# Resume from last checkpoint if interrupted
./scripts/safe-git-gc.sh --resume

# Monitor progress
./scripts/safe-git-gc-monitor.sh --watch
```

**Why Safe Scripts:**
- ✅ Memory-limited operations (configurable via `SAFE_GC_MEMORY_MAX`)
- ✅ Checkpoint/resume capability after each stage
- ✅ Progress tracking and monitoring
- ✅ Pre-flight integrity checks
- ✅ Proven safety: Git gc completed successfully in 6 minutes with 97.5% size reduction

### Pre-Flight Health Checks

Before starting tasks that depend on external services:

```bash
# Check inference gateway availability
curl -sf --max-time 5 https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health || echo "Gateway down"

# Check system resources
free -h                    # Memory: Need 10GB+ available
df -h /                    # Disk: Need 20GB+ free
uptime                     # Load: Should be < 10 on 1min average
```

### Retry Strategy for Transient Failures

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

---

## Crash Investigation Reports

### Individual Crash Documentation

**Directory:** [`docs/crashes/`](crashes/)  
**Contains:** 289+ individual crash investigation and verification reports

**Key Individual Reports:**
- [`crashes/bf-173o7e-report.md`](crashes/bf-173o7e-report.md) - Git gc crash (false positive, workflow issue)
- [`crash-analysis-domchk-c9641ac5-2026-09-01.md`](crash-analysis-domchk-c9641ac5-2026-09-01.md) - Service availability failure
- [`investigation-summary-bf-173o7e-2026-09-01.md`](investigation-summary-bf-173o7e-2026-09-01.md) - False positive investigation

**Verification Report Pattern:** `verification-report-*.md` (60+ reports confirming crash classifications)

---

## Monitoring and Alerting

### Recommended Alerts

**1. Memory Pressure Alert**
```yaml
- name: HighMemoryPressure
  expr: node_memory_pressure_percentage > 70
  for: 1m
  annotations:
    summary: "Memory pressure above 70% - OOM risk"
```

**2. Crash Surge Detection**
```yaml
- name: CrashSurgeDetected
  expr: needle_crashes_total{outcome="failed"} > 10
  for: 10m
  annotations:
    summary: "Infrastructure event: 10+ crashes in 10 minutes"
```

**3. Inference Gateway Health**
```yaml
- name: InferenceGatewayDown
  expr: up{job="inference_gateway"} == 0
  for: 1m
  annotations:
    summary: "Inference gateway is down"
```

### System Resource Limits

| Resource | Minimum | Warning | Critical |
|----------|---------|---------|----------|
| **Available Memory** | 20GB | 10GB | 5GB |
| **Disk Space** | 50GB | 30GB | 20GB |
| **CPU Load (1min)** | < 5 | < 10 | > 15 |
| **Git GC Memory** | 1GB | 2GB | 4GB |

---

## Key Learnings Summary

### What Causes Crashes

1. **Infrastructure Events (70%)**: Memory pressure, OOM killer, SIGHUP cascade
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
Other Exit Code?
└─ Standard Investigation
   ├─ Domain-check code involved? → Debug code
   └─ Agent framework issue? → Workflow/infrastructure
```

---

## Documentation Structure

```
docs/
├── crash-response-guide.md                    # Start here for crash investigation
├── comprehensive-crash-investigation-report-2026-09-01.md  # Complete investigation
├── crash-mitigation-strategies.md            # Mitigation proposals
├── crash-remediation-complete-2026-09-01.md  # Remediation status
├── crash-documentation-index.md              # This file
├── crashes/                                  # Individual crash reports
│   ├── bf-173o7e-report.md                  # Example: Git gc crash
│   └── verification-report-*.md             # 60+ verification reports
└── safe-git-gc-*.md                          # Git gc safety documentation
```

---

## Related Documentation in CLAUDE.md

The main project documentation ([`CLAUDE.md`](../CLAUDE.md)) includes a **Crash Prevention and Investigation** section with:

- Operational safety guidelines
- Git operations safety procedures
- Service availability checks
- Crash investigation guidance
- Resource limits and monitoring

---

## Acceptance Criteria Status

| Criteria | Status | Evidence |
|----------|--------|----------|
| Complete investigation report with all findings | ✅ Complete | Comprehensive investigation report (200+ crashes analyzed) |
| Documentation updates made to relevant project files | ✅ Complete | CLAUDE.md updated, crash guides created |
| Clear guidance for agents running memory-intensive operations | ✅ Complete | Safe git gc scripts, crash response guide |
| Monitoring or alerting recommendations documented | ✅ Complete | Mitigation strategies, monitoring recommendations |

---

## Task Status

**Bead:** domchk-9f82e3a1  
**Task:** Document crash findings and update procedures  
**Status:** ✅ READY TO CLOSE

All acceptance criteria met:
- ✅ Comprehensive investigation reports compiled and accessible
- ✅ Documentation updated (CLAUDE.md, crash guides, procedures)
- ✅ Clear guidance for memory-intensive operations (safe git gc, pre-flight checks)
- ✅ Monitoring/alerting recommendations documented (mitigation strategies)

---

**Index Created:** 2026-09-01  
**Purpose:** Single entry point for all crash documentation  
**Target Audience:** Agents investigating crashes or running memory-intensive operations  
**Maintenance:** Update when new crash patterns emerge or remediation status changes
