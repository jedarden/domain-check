# Crash Response Playbook

**Document Version:** 1.0  
**Last Updated:** 2026-09-01  
**Purpose:** Operational procedures for responding to agent crashes with exit code -1

---

## Overview

This playbook provides step-by-step procedures for responding to agent crashes that result in exit code -1. Exit code -1 can represent **two distinct signal types**:

1. **Signal 9 (SIGKILL)**: Linux OOM (Out Of Memory) killer intervention
2. **Signal 1 (SIGHUP)**: External system process termination

**Critical First Step**: Always run the classification script before taking any action:

```bash
./scripts/classify-signal-crash.sh
```

This script will automatically:
- Check repository health (size, loose objects)
- Check system memory availability
- Classify the crash as OOM or SIGHUP
- Recommend appropriate actions

---

## Alert: Agent Crash with Exit Code -1

### Immediate Actions (0-15 minutes)

#### Step 1: Classify the Crash

```bash
cd /home/coding/domain-check
./scripts/classify-signal-crash.sh
```

**Expected Outputs:**

- **If OOM SIGKILL**: Script exits with code 1, recommends repository recovery
- **If SIGHUP Cascade**: Script exits with code 0, recommends documenting as fleet event

#### Step 2: Verify Crash Pattern

Check if this is a systematic pattern (>3 crashes on same bead):

```bash
# List recent crash alert beads for this workspace
bead list --json | jq -r '.[] | select(.title | contains("ALERT")) | select(.title | contains("crash")) | "\(.id) \(.title)"' | tail -20
```

Look for:
- Multiple alerts for same bead within short time window
- Pattern of systematic retries with same exit code
- Cross-worker correlation (check other workspaces)

#### Step 3: Check Repository Health (if OOM classified)

```bash
# Quick health check
du -sh .git
git count-objects -vH

# Thresholds:
# - Repository size should be < 500MB
# - Loose objects should be < 1000
```

#### Step 4: Check System Memory (if OOM classified)

```bash
# Check current memory state
free -h

# Check kernel logs for OOM events
sudo dmesg | grep -i "out of memory" | tail -20

# Check system journal for recent OOM kills
journalctl -xe --grep="OOM" | tail -20
```

---

## Investigation (15-60 minutes)

### For OOM SIGKILL Crashes

#### Root Cause Identification

**Repository Bloat (Most Common)**

```bash
# Check for large files in git history
git rev-list --objects --all |
git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)' |
awk '/^blob/ {print substr($0,6)}' |
sort -nk2 |
tail -20
```

**Memory Exhaustion During Git Operations**

```bash
# Review memory monitoring logs (if available)
cat .beads/logs/git-memory-*.log | tail -50

# Check for memory-intensive git operations near crash time
git log --since="2 days ago" --until="1 day ago" --oneline
```

**Systematic Pattern Recognition**

If >3 crashes on same bead:
- Root cause is environmental, not task-specific
- Any memory-intensive operation would trigger same result
- Focus on repository/system health, not bead task

#### Impact Assessment

```bash
# Check affected bead count
bead list --json | jq -r '[.[] | select(.title | contains("crash")) | .id] | length'

# Check timeline
bead list --json | jq -r '.[] | select(.title | contains("crash")) | "\(.created) \(.id)"' | sort

# Check cross-worker impact
# (Manually check other workspace directories for similar crashes)
```

### For SIGHUP Cascade Crashes

#### Root Cause Identification

**External System Event**

```bash
# Check for systemd service restarts
journalctl -xe --since="1 day ago" | grep -i "systemd\|restart\|reload" | tail -50

# Check for fleet manager activity
# (Depends on your fleet management system)

# Check TTY/terminal hangups
journalctl -xe --since="1 day ago" | grep -i "sighup\|hangup" | tail -50
```

**Temporal Clustering**

```bash
# Group crash alerts by hour
bead list --json | jq -r '.[] | select(.title | contains("crash")) | .created' |
cut -c1-13 | sort | uniq -c
```

Look for clustering of crashes within 5-hour windows (characteristic of SIGHUP cascades).

#### Cross-Worker Verification

Check other bead workspaces for crashes in same time window:

```bash
# For each workspace under /home/coding/*/:
# cd /path/to/workspace
# bead list --json | jq -r '.[] | select(.title | contains("crash")) | .created'
```

If multiple workspaces show crashes in same time window → Fleet-wide event confirmed.

---

## Resolution (1-4 hours)

### For OOM SIGKILL Crashes

#### Automated Repository Recovery

```bash
cd /home/coding/domain-check
./scripts/recover-repo-bloat.sh
```

**What this does:**
1. Checks repository integrity (git fsck)
2. Runs conservative git gc
3. Runs aggressive git gc if still needed
4. Reports size reduction achieved
5. Lists largest files if still problematic

**Expected Results:**
- Repository size < 500MB
- Loose objects < 100
- Improved git operation performance

#### Manual Recovery (If Automated Fails)

**Step 1: Investigate Large Files**

```bash
# Find largest blobs in history
git rev-list --objects --all |
git cat-file --batch-check='%(objecttype) %(objectname) %(objectsize) %(rest)' |
awk '/^blob/ {print substr($0,6)}' |
sort -nk2 |
tail -20
```

**Step 2: Remove Large Files from History**

```bash
# Use git-filter-repo (preferred method)
# First, install if needed:
# pip install git-filter-repo

# Create a backup branch
git branch backup-before-cleanup

# Remove large files (example: removing *.jsonl files)
git filter-repo --path .beads/ --invert-paths

# Force cleanup (be careful - this rewrites history)
git reflog expire --expire=now --all
git gc --prune=now --aggressive

# Force push to update remote (ONLY if you're sure!)
# git push origin --force --all
```

**Step 3: Verify Recovery**

```bash
# Check repository health
./scripts/classify-signal-crash.sh

# Should report: "Repository is healthy"
```

#### Update .gitignore (If Needed)

If large files were accidentally committed, add them to .gitignore:

```bash
# Edit .gitignore to add patterns for large files
# Example:
# *.jsonl
# *.db
# *.sqlite
# .beads/

# Apply to future commits
git add .gitignore
git commit -m "Add large file patterns to .gitignore"
```

### For SIGHUP Cascade Crashes

#### Documentation (No Repository Action Needed)

**Step 1: Document the Event**

Create crash investigation document:

```bash
# Use existing template
cp docs/crash-investigation-template.md docs/crash-investigation-sighup-$(date +%Y%m%d).md
```

Document:
- Timestamp range of crashes
- Number of affected workers
- Types of tasks interrupted
- External event source (if identifiable)

**Step 2: Update Bead Notes**

For each crash alert bead:

```bash
bead update <alert-bead-id> --notes "
SIGHUP cascade event - fleet-wide external termination.
Timestamp: <timestamp-range>
Affected workers: <list>
Root cause: External system process (systemd/fleet manager)
Classification: Signal 1 (SIGHUP) - exit code -1
Action taken: Documented as fleet event, no repository action needed
"
```

**Step 3: Close Alert Beads**

```bash
bead close <alert-bead-id> --reason "SIGHUP cascade event - documented as fleet-wide external termination, no repository issue"
```

---

## Prevention (Ongoing)

### Layer 0: Classification

- ✅ Classification script: `./scripts/classify-signal-crash.sh`
- Run immediately on any exit code -1 crash
- Distinguishes OOM from SIGHUP automatically

### Layer 1: Prevention

**Pre-commit Hook (Installed)**

```bash
# Verify pre-commit hook is active
test -x .git/hooks/pre-commit && echo "✅ Pre-commit hook installed" || echo "❌ Pre-commit hook missing"
```

**Git GC Configuration**

```bash
# Verify automatic GC is configured
git config --local --get gc.auto        # Should be 256
git config --local --get gc.autoPackLimit  # Should be 10
```

**.gitignore Protection**

```bash
# Verify .gitignore has large file patterns
grep -E "(\.db|\.jsonl|\.sqlite)" .gitignore
```

### Layer 2: Monitoring

**Daily Repository Health Checks**

```bash
# Run health monitoring script
./scripts/monitor-repo-health.sh

# Check logs
cat .beads/logs/repo-health.log | tail -50
```

**CI/CD Integration**

The `domain-check-build` Argo Workflow includes a repository health check step. Verify it's running:

```bash
# Check recent workflow runs
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
  get workflows -n argo-workflows \
  -l workflows.argoproj.io/workflow-template=domain-check-build \
  --sort-by=.metadata.creationTimestamp | tail -5
```

### Layer 3: Early Detection

**Pattern Recognition**

Review crash alert bead creation logic for systematic pattern detection:

- >3 crashes on same bead in 1 hour → Escalate as systematic pattern
- Multiple workers with crashes in same time window → Fleet-wide event
- Repository bloat detected → Immediate recovery action

### Layer 4: Response

**Automated Recovery**

```bash
# Run recovery script (detects and fixes bloat automatically)
./scripts/recover-repo-bloat.sh
```

**Fleet-Level Coordination**

For SIGHUP cascade events:
- Document in central incident log
- Coordinate with fleet manager
- Avoid duplicate investigations across workers

---

## Post-Incident (24-48 hours)

### Review Lessons Learned

**For OOM Crashes:**

1. **What caused the bloat?**
   - Large file commits from bead tasks?
   - Git operations on already-bloated repo?
   - System memory exhaustion?

2. **Was prevention effective?**
   - Did pre-commit hook block large files?
   - Did .gitignore cover the file type?
   - Was automatic GC running?

3. **Was detection timely?**
   - Was repository growing before crash?
   - Were health check alerts firing?
   - Was classification immediate?

**For SIGHUP Crashes:**

1. **Was the event documented correctly?**
   - Was classification accurate?
   - Were fleet-wide impacts identified?
   - Was root cause traceable?

2. **Was response appropriate?**
   - Were no repository actions taken?
   - Was fleet coordination effective?
   - Were duplicate investigations avoided?

### Adjust Thresholds

If bloat recurred or detection was late:

```bash
# Lower repository size threshold in scripts
# Edit: scripts/monitor-repo-health.sh
# Change: REPO_SIZE_WARNING=$((500 * 1024 * 1024))
# To:     REPO_SIZE_WARNING=$((300 * 1024 * 1024))  # 300MB

# Lower loose objects threshold in scripts
# Edit: scripts/classify-signal-crash.sh
# Change: if [ "$LOOSE_OBJECTS" -gt 1000 ]; then
# To:     if [ "$LOOSE_OBJECTS" -gt 500 ]; then
```

### Update Documentation

Record findings in:

1. Crash investigation documents (`docs/crash-investigation-*.md`)
2. This playbook (update procedures based on lessons learned)
3. Bead notes (document root cause and resolution)

### Monitor for Recurrence

Track metrics for 30 days:

```bash
# Check for new exit code -1 crashes
bead list --json | jq -r '.[] | select(.title | contains("crash")) | select(.title | contains("exit code -1")) | "\(.created) \(.id)"' | grep $(date +%Y-%m)

# Check repository size trend
tail -30 .beads/logs/repo-health.log | grep "Repository Health"
```

---

## Success Criteria

### For OOM SIGKILL Crashes

1. ✅ Repository size < 500MB (sustained for 30 days)
2. ✅ Loose objects < 100
3. ✅ No OOM crashes for 30 days
4. ✅ All large files blocked by pre-commit hook
5. ✅ Health checks running daily

### For SIGHUP Cascade Crashes

1. ✅ All signal -1 crashes classified within 5 minutes
2. ✅ SIGHUP events documented as fleet-wide
3. ✅ No manual repository recovery for SIGHUP events
4. ✅ Fleet-level coordination procedures established
5. ✅ Zero misattribution to task failures

---

## Emergency Contacts

**For System-Level Issues:**
- Infrastructure: System administrator
- Fleet Manager: Fleet operations team

**For Bead/Agent Issues:**
- Domain-check workspace: /home/coding/domain-check
- Bead documentation: `docs/beads/`
- Crash investigations: `docs/crash-investigation-*.md`

---

## Appendix: Quick Reference

### Classification Script

```bash
# Always run first!
./scripts/classify-signal-crash.sh
# Exit code 0 = SIGHUP (external event)
# Exit code 1 = OOM (repository issue)
```

### Recovery Script

```bash
# For OOM crashes only
./scripts/recover-repo-bloat.sh
# Automatically detects and fixes repository bloat
```

### Health Monitoring

```bash
# Check current repository health
./scripts/monitor-repo-health.sh

# Review historical health data
cat .beads/logs/repo-health.log
```

### Bead Commands

```bash
# Update bead with investigation findings
bead update <bead-id> --notes "Findings..."

# Close crash alert bead
bead close <alert-bead-id> --reason "Resolution summary..."
```

---

**Playbook Status:** ✅ ACTIVE  
**Next Review Date:** 2026-10-01  
**Maintained By:** domain-check workspace
