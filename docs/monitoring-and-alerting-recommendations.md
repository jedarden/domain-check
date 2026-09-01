# Monitoring and Alerting Recommendations for Domain-Check

**Created:** 2026-09-01  
**Purpose:** Comprehensive monitoring strategy to prevent and detect crash conditions  
**Related:** `docs/crash-mitigation-strategies.md`, `docs/crash-response-guide.md`

---

## Executive Summary

Based on comprehensive crash investigation (200+ crashes analyzed), the primary causes are infrastructure events and service failures, not code defects. This document defines a monitoring strategy to detect these conditions early and prevent crashes through proactive alerts.

**Key Findings:**
- 70% of crashes: Infrastructure events (memory pressure, OOM, SIGHUP cascade)
- 20% of crashes: Agent workflow failures (max turns exhaustion)
- 8% of crashes: Service availability failures (inference gateway down)
- 2% of crashes: Actual code defects (very rare for domain-check)

---

## Alert Severity Levels

| Severity | Description | Response Time | Example |
|----------|-------------|---------------|---------|
| **P0 - Critical** | System down, data loss risk | Immediate | OOM imminent, repository corruption |
| **P1 - High** | Service unavailable, crashes occurring | 5 minutes | Inference gateway down, crash surge |
| **P2 - Medium** | Resource pressure, degraded performance | 30 minutes | High memory pressure, disk space low |
| **P3 - Low** | Early warning, preventive action | 1 hour | Memory trending upward, GC needed |

---

## System-Level Monitoring

### 1. Memory Pressure Monitoring

**Purpose:** Detect memory pressure before OOM killer activates (80% threshold)

**Metrics:**
```yaml
- name: node_memory_pressure_percentage
  type: gauge
  description: "Memory pressure percentage from systemd-oomd"
  labels: [host]
```

**Alerts:**
```yaml
# P2: Warning - Memory pressure elevated
- alert: HighMemoryPressureWarning
  expr: node_memory_pressure_percentage > 70
  for: 1m
  labels:
    severity: P2
  annotations:
    summary: "Memory pressure elevated ({{ $value }}%)"
    description: "Memory pressure is {{ $value }}%, approaching OOM threshold of 80%."
    action: "Investigate memory usage, prepare for potential OOM"

# P0: Critical - OOM imminent
- alert: OomImminent
  expr: node_memory_pressure_percentage > 75
  for: 30s
  labels:
    severity: P0
  annotations:
    summary: "OOM IMMINENT - Memory pressure {{ $value }}%"
    description: "Memory pressure is {{ $value }}%, OOM threshold is 80%. Act immediately to prevent system crash."
    action: "Kill non-essential processes, clear caches, abort heavy operations"
```

**Dashboard Panels:**
- Memory pressure percentage (current, 5m avg, 1h avg)
- Available memory (GB)
- Memory usage by top 10 processes
- Trend: Memory pressure over last 24h

### 2. Disk Space Monitoring

**Purpose:** Prevent disk exhaustion that could crash git operations

**Metrics:**
```yaml
- name: node_filesystem_avail_bytes
  type: gauge
  description: "Available disk space in bytes"
  labels: [mountpoint, device]

- name: node_filesystem_size_bytes
  type: gauge
  description: "Total disk size in bytes"
  labels: [mountpoint, device]
```

**Alerts:**
```yaml
# P2: Disk space low
- alert: DiskSpaceLow
  expr: (node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"}) * 100 < 20
  for: 5m
  labels:
    severity: P2
  annotations:
    summary: "Disk space low ({{ $value }}% available)"
    description: "Only {{ $value }}% disk space remaining on root filesystem."
    action: "Clean up old files, clear package caches, remove unnecessary data"

# P1: Disk space critical
- alert: DiskSpaceCritical
  expr: (node_filesystem_avail_bytes{mountpoint="/"} / node_filesystem_size_bytes{mountpoint="/"}) * 100 < 10
  for: 1m
  labels:
    severity: P1
  annotations:
    summary: "DISK SPACE CRITICAL - Only {{ $value }}% remaining"
    description: "Critical disk space: {{ $value }}% available. Immediate action required."
    action: "URGENT: Clean up immediately, abort write-heavy operations"
```

**Dashboard Panels:**
- Disk usage percentage (current, trend)
- Available disk space (GB)
- Disk usage by directory (top 10)
- I/O operations and throughput

### 3. CPU Load Monitoring

**Purpose:** Detect CPU saturation that causes system unresponsiveness

**Metrics:**
```yaml
- name: node_load1
  type: gauge
  description: "1-minute load average"

- name: node_load5
  type: gauge
  description: "5-minute load average"

- name: node_load15
  type: gauge
  description: "15-minute load average"

- name: node_cpu_seconds_total
  type: counter
  description: "Total CPU seconds spent"
  labels: [mode, cpu]
```

**Alerts:**
```yaml
# P2: CPU saturation
- alert: CpuSaturation
  expr: node_load1 / count(node_cpu_seconds_total{mode="idle"}) by (instance) > 5
  for: 5m
  labels:
    severity: P2
  annotations:
    summary: "CPU saturation detected (load {{ $value }}x cores)"
    description: "1-minute load average is {{ $value }}x the number of CPU cores."
    action: "Investigate CPU-intensive processes, consider throttling"

# P1: Severe CPU saturation
- alert: CpuSaturationSevere
  expr: node_load1 / count(node_cpu_seconds_total{mode="idle"}) by (instance) > 10
  for: 2m
  labels:
    severity: P1
  annotations:
    summary: "SEVERE CPU SATURATION - load {{ $value }}x cores"
    description: "System is heavily overloaded, processes may be unresponsive."
    action: "URGENT: Kill or throttle CPU-intensive processes"
```

**Dashboard Panels:**
- Load averages (1min, 5min, 15min)
- CPU usage percentage by mode (user, system, idle, iowait)
- Top CPU-consuming processes
- CPU trend over last 24h

---

## Service-Level Monitoring

### 4. Inference Gateway Health

**Purpose:** Detect inference gateway unavailability before agents fail

**Metrics:**
```yaml
- name: inference_gateway_up
  type: gauge
  description: "Inference gateway health check status (1=up, 0=down)"
  labels: [gateway]

- name: inference_gateway_response_time_seconds
  type: histogram
  description: "Inference gateway response time in seconds"
  labels: [gateway]
  buckets: [0.1, 0.5, 1.0, 2.0, 5.0, 10.0]
```

**Health Check Configuration:**
```yaml
health_checks:
  - name: inference_gateway
    url: https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health
    interval: 30s
    timeout: 5s
    expected_status: 200
```

**Alerts:**
```yaml
# P1: Gateway down
- alert: InferenceGatewayDown
  expr: inference_gateway_up == 0
  for: 1m
  labels:
    severity: P1
  annotations:
    summary: "Inference gateway is DOWN"
    description: "Inference gateway has been down for >1 minute. Agents will fail until restored."
    action: "Check gateway service status, restart if needed, verify network connectivity"

# P2: Gateway degraded
- alert: InferenceGatewayDegraded
  expr: inference_gateway_response_time_seconds > 5
  for: 5m
  labels:
    severity: P2
  annotations:
    summary: "Inference gateway degraded (slow response)"
    description: "Gateway response time is {{ $value }}s, expected <5s."
    action: "Investigate gateway performance, check for overload"

# P2: Gateway error rate high
- alert: InferenceGatewayErrorRateHigh
  expr: (rate(inference_gateway_requests_total{status=~"5.."}[5m]) / rate(inference_gateway_requests_total[5m])) > 0.05
  for: 5m
  labels:
    severity: P2
  annotations:
    summary: "Gateway error rate elevated ({{ $value | humanizePercentage }})"
    description: "Gateway is returning 5xx errors for {{ $value | humanizePercentage }} of requests."
    action: "Check gateway logs, investigate service degradation"
```

**Dashboard Panels:**
- Gateway up/down status
- Response time percentiles (p50, p95, p99)
- Request rate (requests/sec)
- Error rate by status code

---

## Application-Level Monitoring

### 5. Git Operation Monitoring

**Purpose:** Monitor git gc operations for memory usage and duration

**Metrics:**
```yaml
- name: git_gc_operations_total
  type: counter
  description: "Total git gc operations"
  labels: [repo, mode, outcome]

- name: git_gc_duration_seconds
  type: histogram
  description: "Git gc operation duration in seconds"
  labels: [repo, mode]
  buckets: [60, 300, 600, 1800, 3600, 7200]

- name: git_gc_memory_bytes
  type: gauge
  description: "Current git gc memory usage in bytes"
  labels: [repo, pid]

- name: git_gc_loose_objects
  type: gauge
  description: "Number of loose git objects"
  labels: [repo]

- name: git_gc_pack_files
  type: gauge
  description: "Number of git pack files"
  labels: [repo]
```

**Alerts:**
```yaml
# P2: Git GC running long
- alert: GitGcRunningLong
  expr: time() - git_gc_start_timestamp > 7200
  for: 1m
  labels:
    severity: P2
  annotations:
    summary: "Git GC operation running > 2 hours"
    description: "Git GC for {{ $labels.repo }} has been running for >2 hours."
    action: "Check if operation is hung, monitor memory usage"

# P1: Git GC memory high
- alert: GitGcMemoryHigh
  expr: git_gc_memory_bytes > 4e9  # 4GB
  for: 1m
  labels:
    severity: P1
  annotations:
    summary: "Git GC memory usage high ({{ $value | humanize }}B)"
    description: "Git GC is using {{ $value | humanize }}B, risk of OOM."
    action: "Consider aborting GC, use safe-git-gc with memory limits"

# P3: Repository needs GC
- alert: RepositoryNeedsGc
  expr: git_gc_loose_objects > 1000
  for: 24h
  labels:
    severity: P3
  annotations:
    summary: "Repository {{ $labels.repo }} has {{ $value }} loose objects"
    description: "Repository has accumulated {{ $value }} loose objects, consider running git gc."
    action: "Schedule safe-git-gc run during off-hours"
```

**Dashboard Panels:**
- Active git gc operations (count, duration)
- Git GC memory usage
- Repository health (loose objects, pack files)
- GC operation success rate

### 6. Agent Task Monitoring

**Purpose:** Detect stuck or failing agent tasks

**Metrics:**
```yaml
- name: needle_agent_tasks_total
  type: counter
  description: "Total agent tasks"
  labels: [worker, outcome]

- name: needle_agent_task_duration_seconds
  type: histogram
  description: "Agent task duration in seconds"
  labels: [worker, task_type]
  buckets: [60, 300, 600, 1800, 3600, 7200]

- name: needle_agent_turns_total
  type: histogram
  description: "Number of agent conversation turns"
  labels: [worker, task_type, outcome]
  buckets: [10, 20, 30, 40, 50, 100]

- name: needle_crash_rate_5m
  type: gauge
  description: "Crash rate per 5 minutes"
  labels: [worker]
```

**Alerts:**
```yaml
# P2: Agent task stuck
- alert: NeedleAgentTaskStuck
  expr: needle_agent_task_duration_seconds{outcome="running"} > 7200
  for: 10m
  labels:
    severity: P2
  annotations:
    summary: "Agent task running > 2 hours"
    description: "Task {{ $labels.task_type }} on {{ $labels.worker }} has been running >2 hours."
    action: "Investigate task status, check for hang or infinite loop"

# P1: Crash surge detected
- alert: CrashSurgeDetected
  expr: sum(rate(needle_agent_tasks_total{outcome="failed"}[5m])) by (worker) > 10
  for: 2m
  labels:
    severity: P1
  annotations:
    summary: "CRASH SURGE - {{ $value | humanize }} crashes/5m on {{ $labels.worker }}"
    description: "Infrastructure event likely: {{ $value }} crashes in 5 minutes."
    action: "Check system resources (memory, disk, CPU), investigate infrastructure event"

# P2: High crash rate
- alert: HighCrashRate
  expr: sum(rate(needle_agent_tasks_total{outcome="failed"}[1h])) by (worker) > 0.1
  for: 30m
  labels:
    severity: P2
  annotations:
    summary: "Elevated crash rate ({{ $value }} crashes/hour)"
    description: "Worker {{ $labels.worker }} experiencing {{ $value }} crashes/hour."
    action: "Investigate crash patterns, check for systematic issues"

# P3: Max turns exhaustion
- alert: MaxTurnsExhaustion
  expr: sum(rate(needle_agent_turns_total{outcome="max_turns"}[1h])) by (worker) > 0.05
  for: 1h
  labels:
    severity: P3
  annotations:
    summary: "Agent max turns exhaustion occurring"
    description: "Agents hitting turn limits ({{ $value }}/hour), indicates workflow issues."
    action: "Investigate workflow patterns, consider increasing max turns for complex tasks"
```

**Dashboard Panels:**
- Active tasks by worker
- Task duration distribution (p50, p95, p99)
- Crash rate (1m, 5m, 1h averages)
- Turn count distribution
- Task success rate

---

## Custom Metrics Collection

### Collection Script Setup

```bash
#!/bin/bash
# metrics-collector.sh - Custom metrics for domain-check

# Memory pressure (requires systemd-oomd query)
MEMORY_PRESSURE=$(systemctl show systemd-oomd | grep MemoryPressureLimit | cut -d= -f2)
echo "node_memory_pressure_percentage $MEMORY_PRESSURE"

# Inference gateway health
if curl -sf --max-time 5 https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health > /dev/null; then
  echo "inference_gateway_up 1"
else
  echo "inference_gateway_up 0"
fi

# Git GC operations
for pid in $(pgrep -f "git.*gc\|git.*repack"); do
  memory=$(ps -o rss= -p $pid | awk '{print $1*1024}')  # Convert to bytes
  echo "git_gc_memory_bytes{repo=\"domain-check\",pid=\"$pid\"} $memory"
done

# Repository health
cd /home/coding/domain-check
loose_objects=$(git count-objects -v | grep loose_objects | cut -d= -f2 | tr -d ' ')
echo "git_gc_loose_objects{repo=\"domain-check\"} $loose_objects"

pack_files=$(git count-objects -v | grep pack | cut -d= -f2 | tr -d ' ')
echo "git_gc_pack_files{repo=\"domain-check\"} $pack_files"
```

**Cron Setup:**
```cron
# Collect metrics every 30 seconds
*/30 * * * * /home/coding/domain-check/scripts/metrics-collector.sh >> /var/lib/node_exporter/textfile_collector/domain-check.metrics.prom
```

---

## Alert Response Procedures

### P0 - Critical Response

**Immediate Actions (within 1 minute):**
1. Check system resources: `free -h`, `df -h`, `uptime`
2. Identify and kill non-essential processes consuming resources
3. Abort heavy operations (git gc, large builds)
4. Clear caches if safe: `sync; echo 3 > /proc/sys/vm/drop_caches`
5. Escalate to human operator if not resolved in 5 minutes

### P1 - High Response

**Actions (within 5 minutes):**
1. Investigate root cause using crash response guide
2. Check service status for service failures
3. Review crash patterns for infrastructure events
4. Implement temporary mitigation
5. Document findings and escalate if needed

### P2 - Medium Response

**Actions (within 30 minutes):**
1. Investigate trend and root cause
2. Plan preventive action (GC, cleanup, resource adjustment)
3. Schedule during off-hours if appropriate
4. Document findings

### P3 - Low Response

**Actions (within 1 hour):**
1. Acknowledge alert
2. Plan preventive maintenance
3. Create ticket if follow-up needed

---

## Implementation Priority

### Phase 1: Immediate (Week 1)

| Alert | Priority | Effort | Impact |
|-------|----------|--------|--------|
| Memory Pressure Warning (70%) | P0 | Low | High |
| Disk Space Low (< 20%) | P1 | Low | High |
| Inference Gateway Down | P1 | Low | High |
| Crash Surge Detected | P1 | Medium | High |

### Phase 2: Short-term (Weeks 2-4)

| Alert | Priority | Effort | Impact |
|-------|----------|--------|--------|
| OOM Imminent (75%) | P0 | Low | High |
| CPU Saturation | P2 | Low | Medium |
| Git GC Memory High | P1 | Medium | High |
| Repository Needs GC | P3 | Low | Medium |

### Phase 3: Long-term (Months 2-3)

| Alert | Priority | Effort | Impact |
|-------|----------|--------|--------|
| Detailed Agent Monitoring | P2 | High | Medium |
| Custom Metrics Collection | P3 | Medium | Medium |
| Dashboard Creation | P3 | Medium | Medium |

---

## Tools and Integration

### Recommended Monitoring Stack

**Option 1: Prometheus + Grafana (Recommended)**
- **Pros:** Industry standard, powerful query language, excellent dashboards
- **Cons:** Requires setup and maintenance
- **Effort:** Medium (2-3 days setup)

**Option 2: Netdata**
- **Pros:** Easy setup, minimal configuration, good default dashboards
- **Cons:** Less flexible, limited query capability
- **Effort:** Low (1 day setup)

**Option 3: Simple Shell Script + Email**
- **Pros:** Minimal dependencies, simple
- **Cons:** Limited monitoring, poor UI
- **Effort:** Low (1 day setup)

### Alert Routing

**Email Alerts:**
```yaml
receivers:
  - name: 'critical-alerts'
    email_configs:
      - to: 'infra-alerts@ardenone.com'
        send_resolved: true
```

**Slack Integration:**
```yaml
receivers:
  - name: 'slack-alerts'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/...'
        channel: '#infrastructure'
        send_resolved: true
```

---

## Success Metrics

Monitoring implementation is successful when:

- ✅ Memory pressure detected before OOM (> 70% warning)
- ✅ Disk space exhaustion prevented (> 20% warning)
- ✅ Service failures detected within 1 minute
- ✅ Crash surges identified and investigated automatically
- ✅ False positive crashes identified without manual investigation
- ✅ Git GC operations monitored and memory-limited
- ✅ Agent task health visible in dashboards

---

## Related Documentation

- **Crash Response Guide:** `docs/crash-response-guide.md`
- **Mitigation Strategies:** `docs/crash-mitigation-strategies.md`
- **Comprehensive Investigation:** `docs/comprehensive-crash-investigation-report-2026-09-01.md`

---

**Document Status:** ✅ Complete  
**Last Updated:** 2026-09-01  
**Next Review:** 2026-10-01  
**Implementation Priority:** Phase 1 (Week 1) - High-priority alerts
