# Domain Check Crash Logs Catalog

## Generated: 2026-09-01

## Summary
Total crash events found: **247** (all exit code -1)
Date range: 2026-08-16 to 2026-08-26
Primary crash signal: -1 (typically SIGKILL or OOM)

---

## Crash Log Locations

### 1. Primary Crash Event Log
**File:** `/home/coding/domain-check/.beads/events.jsonl`
- **Format:** JSONL (one JSON object per line)
- **Total crashes:** 247 events
- **Exit codes:** All -1 (signal -1)
- **Key fields:** bead, duration_ms, event, exit_code, outcome, strand, ts, worker

**Sample crash event:**
```json
{
  "bead": "bf-uoyie",
  "duration_ms": 186505,
  "event": "crash",
  "exit_code": -1,
  "outcome": "crash",
  "strand": "auto",
  "ts": "2026-08-16T04:27:36.261347993+00:00",
  "worker": "lab-domain-check"
}
```

### 2. Forensic Investigation Records
**File:** `/home/coding/domain-check/.beads/checkpoint/forensic.jsonl`
- **Format:** JSONL with full bead issue records
- **Size:** 9MB (9,028,196 bytes)
- **Contents:** Detailed crash investigation reports with:
  - Crash alerts with full investigation notes
  - Root cause analysis
  - Repository health status
  - Resolution status

**Sample forensic record:**
```json
{
  "issue": {
    "assignee": "claude-code-glm-4.7-lab-roam-2",
    "base_status": "closed",
    "close_reason": "Investigation complete: FALSE POSITIVE...",
    "closed_at": "2026-08-26T21:10:37.6640833Z",
    "description": "## Agent Crash Report\n\n- **Bead ID**: bf-4x12ec...",
    "id": "bf-10jhaa",
    "issue_type": "task",
    "labels": ["alert", "crash", "signal--1", "split-child"],
    "notes": "## Investigation Complete: FALSE POSITIVE...",
    "priority": 2,
    "title": "ALERT: Agent crash on bead bf-4x12ec"
  },
  "record_type": "issue"
}
```

### 3. Individual Crash Traces
**Directory:** `/home/coding/domain-check/.beads/traces/`
- **Total trace directories:** 913
- **Format:** Each bead has its own directory with 4 files:
  - `metadata.json` - Execution metadata
  - `stderr.txt` - Standard error output
  - `stdout.txt` - Standard output (can be large)
  - `trace.jsonl` - Full execution trace

**Example trace directory structure:**
```
/home/coding/domain-check/.beads/traces/bf-173o7e/
├── metadata.json       (398 bytes)
├── stderr.txt          (457 bytes)
├── stdout.txt          (1.5MB)
└── trace.jsonl         (21KB)
```

**Metadata format:**
```json
{
  "bead_id": "bf-173o7e",
  "agent": "claude-code-glm-4.7",
  "provider": "zai",
  "model": "glm-4.7",
  "exit_code": 1,
  "outcome": "failure",
  "duration_ms": 444317,
  "captured_at": "2026-08-17T17:06:59.953876423Z"
}
```

### 4. Diagnostic Data
**File:** `/home/coding/domain-check/.beads/diagnostics/pluck-diagnostics.json`
- **Format:** JSON
- **Size:** 291KB
- **Purpose:** Contains diagnostic information about bead operations

### 5. Repository Health Logs
**File:** `/home/coding/domain-check/.beads/logs/repo-health.log`
- **Format:** Text log
- **Purpose:** Git repository health monitoring
- **Sample content:**
```
[2026-09-01T11:22:28-04:00] Repository Health Check
  .git size: 89MB (91600KB)
  Loose objects: 8877
  Pack files: 2
  Prune-packable: 0
  Garbage: 0
```

### 6. Empty Crash Reports Directory
**Directory:** `/home/coding/domain-check/.beads/crash-reports/`
- **Status:** Empty (no files stored)
- **Purpose:** Appears to be reserved for future crash report storage

---

## Crash Statistics

### By Bead (Top 20)
| Bead ID | Crash Count |
|---------|-------------|
| bf-44x3a | 18 |
| bf-1vuk2 | 18 |
| bf-9b8oe | 14 |
| bf-3riuu | 14 |
| bf-uoyie | 11 |
| bf-dzntf | 10 |
| bf-3lwth | 10 |
| bf-3b9rv | 10 |
| bf-1rsa6 | 10 |
| bf-687r6 | 9 |
| bf-3561g | 9 |
| bf-2jr19 | 7 |
| bf-w4fwe | 5 |
| bf-4hp9p | 5 |
| bf-1zt5b | 5 |
| bf-5cd2d | 4 |
| bf-2d9p3 | 4 |
| bf-1ui56 | 4 |
| bf-1jcbg | 4 |
| bf-xumcu | 3 |

### By Worker
| Worker | Crash Count |
|--------|-------------|
| lab-domain-check | 154 |
| lab-drawrace | 41 |
| lab-test-fix | 32 |
| lab-roam-1 | 20 |

### By Exit Code
| Exit Code | Count | Interpretation |
|-----------|-------|----------------|
| -1 | 247 | Signal -1 (typically SIGKILL/OOM) |

---

## Crash Investigation Findings

### Common Patterns
1. **Git gc operations** - Several crashes related to `git gc --aggressive` operations
2. **Resource exhaustion** - Exit code -1 typically indicates SIGKILL (likely OOM)
3. **Subprocess survival** - Some crashes occurred while long-running subprocesses (git gc) survived and completed successfully

### False Positives
Several crash alerts were determined to be FALSE POSITIVES:
- Agent crashed but subprocess completed successfully
- Repository remained healthy despite crash
- No actual data loss or corruption

### Notable Crashes
- **bf-173o7e** - Git gc --aggressive caused agent crash, but operation eventually completed
- **bf-4x12ec** - False positive: git gc completed despite agent crash
- Multiple crashes on beads bf-44x3a, bf-1vuk2 (18 crashes each)

---

## Log File Types Summary

| File Type | Location | Format | Purpose | Size |
|-----------|----------|--------|---------|------|
| Event Log | `.beads/events.jsonl` | JSONL | All events including crashes | 1.6MB |
| Forensic Log | `.beads/checkpoint/forensic.jsonl` | JSONL | Detailed crash investigations | 9MB |
| Trace Files | `.beads/traces/{bead-id}/` | Multiple | Per-bead execution traces | Variable |
| Diagnostics | `.beads/diagnostics/pluck-diagnostics.json` | JSON | Operational diagnostics | 291KB |
| Health Log | `.beads/logs/repo-health.log` | Text | Git repository health | 910B |
| Crash Reports | `.beads/crash-reports/` | Empty | Reserved for future reports | 0B |

---

## Access Patterns

### Quick Crash Search
```bash
# Find all crash events
grep -i "\"event\":\"crash\"" /home/coding/domain-check/.beads/events.jsonl

# Count crashes by bead
grep -i "\"event\":\"crash\"" /home/coding/domain-check/.beads/events.jsonl | \
  jq -r '.bead' | sort | uniq -c | sort -rn

# Find crashes for specific bead
grep "\"bead\":\"bf-173o7e\"" /home/coding/domain-check/.beads/events.jsonl
```

### Detailed Investigation
```bash
# Check trace for specific crashed bead
ls /home/coding/domain-check/.beads/traces/bf-173o7e/

# View crash metadata
cat /home/coding/domain-check/.beads/traces/bf-173o7e/metadata.json

# Check stderr for error messages
cat /home/coding/domain-check/.beads/traces/bf-173o7e/stderr.txt

# View full investigation notes
grep "bf-173o7e" /home/coding/domain-check/.beads/checkpoint/forensic.jsonl | \
  jq -r '.issue.notes'
```

---

## Recommendations

1. **Monitor events.jsonl** - Real-time crash detection via grep
2. **Check traces directory** - Detailed per-crash analysis
3. **Review forensic.jsonl** - Complete investigation history
4. **Empty crash-reports directory** - Consider implementing crash report generation
5. **Resource monitoring** - Most crashes are exit code -1, suggesting memory/CPU limits

---

## Conclusion

The .beads directory contains comprehensive crash logging across multiple files:
- **247 crash events** tracked in events.jsonl (all exit code -1)
- **913 trace directories** with detailed execution logs
- **9MB forensic database** with investigation notes
- **Active monitoring** via heartbeats.jsonl and repo-health.log

Most crashes appear to be infrastructure-related (memory/CPU limits) rather than application defects, with several false positives where subprocesses survived agent termination.
