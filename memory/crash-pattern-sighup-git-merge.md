---
name: crash-pattern-sighup-git-merge
description: SIGHUP crash pattern during long-running git merge operations
metadata:
  type: project
---

## SIGHUP Crash Pattern

**Pattern Identifier:** Exit code -1 (SIGHUP) during long-running git operations

### Trigger Conditions

- **Operation:** Complex git merge reconciliation with 661+ divergent commits
- **Duration:** 20-30 minute stalls before crash
- **System State:** High load (17-20 average) despite adequate memory (52GB free)

### Investigation Summary

**Agent:** claude-code-glm-4.7-lab-roam-3
**Bead:** bf-1s6c3 (git merge reconciliation task)
**Parent Bead:** bf-81074 (extensive investigation notes)
**Crash Timestamp:** 2026-08-12 21:50:33 UTC (last activity: 21:21:42 UTC)

**Timeline:**
- Last agent activity: 21:21:42 UTC
- Crash: 21:50:33 UTC
- Stall duration: 29 minutes

**System State at Crash:**
- Load average: 17-20 (high)
- Available memory: 52GB (adequate)
- Operation: Creating merge commit for 661+ divergent commits

### Pattern Evidence

- **43 similar crash alert beads** exist for parent task bf-4yjq
- Multiple crashes on 2026-08-12 between 17:54-21:50 UTC
- All involve long-running git operations under high load

### Root Cause

Long-running git merge operations with complex commit histories (661+ divergent commits) appear to trigger SIGHUP signals when:
1. System load is elevated (17-20 range)
2. Operation duration exceeds 20-30 minutes
3. Merge involves significant conflict resolution

### Mitigation Strategies

**For Long-Running Git Operations:**

1. **Break into smaller steps:**
   - Avoid single merge commits with 661+ divergent commits
   - Consider incremental merges (e.g., merge 50-100 commits at a time)
   - Use rebasing or cherry-picking as alternatives to mega-merges

2. **Monitor system state:**
   - Check load average before starting complex operations
   - Consider deferring heavy merges when load > 10

3. **Alternative approaches:**
   - Use `git merge --squash` for consolidating histories
   - Consider `git rebase -i` for linearizing commit history
   - Break reconciliation into multiple smaller merge commits

### Related Beads

- **Parent Investigation:** bf-81074 (extensive investigation notes)
- **Task Bead:** bf-1s6c3 (git merge reconciliation)
- **Pattern Bead:** bf-4yjq (43 similar crash alerts)

### Exit Code Reference

- **Exit code -1:** SIGHUP signal received (infrastructure event)
- **Classification:** Infrastructure event, not code defect
- **Action:** Review operation complexity and system state

### When This Pattern Applies

Use this pattern when investigating crashes with:
- Exit code -1
- Long-running git operations (>20 minutes)
- High system load (15+)
- Complex merge/reconciliation tasks

### Future Investigation Steps

If similar SIGHUP crashes occur:
1. Check system load and memory state
2. Identify the git operation being performed
3. Count number of commits being merged
4. Review parent bead bf-81074 for full investigation methodology
5. Consider breaking operation into smaller steps
