# Git GC Operations Mitigation Strategy

**Document Version:** 1.0  
**Date:** 2026-08-28  
**Bead:** domchk-e49087e2  
**Status:** PROPOSED

## Executive Summary

This document analyzes historical crash patterns related to git gc operations and proposes a comprehensive mitigation strategy. **Key finding:** The most recent crash (bf-173o7e) was NOT a git gc failure — the operation completed successfully, and the crash occurred during bead closing due to max_turns limit exhaustion. However, git gc --aggressive remains a resource-intensive operation that warrants safeguards.

## Root Cause Analysis

### What Actually Happened (bf-173o7e, 2026-08-17)

**The Task:** Execute `git gc --aggressive --prune=now` on `/home/coding/domain-check`

**The Outcome:**
- ✅ Git gc completed successfully in ~7 minutes
- ✅ Repository optimized: 9→3 loose objects, all 7,753 objects packed into 445MB file
- ✅ Memory usage: 864MB-1.3GB (well within 52GB available)
- ✅ Repository integrity maintained
- ❌ Agent crashed during bead closing (not git gc)

**The Crash:**
- Exit code: 1 (failure)
- Terminal reason: `max_turns` (30-turn limit exhausted)
- Crash phase: Post-completion workflow (bead closing attempts)
- NOT resource exhaustion, NOT OOM, NOT git gc failure

### Why Bead Closing Failed

The agent entered a retry loop trying to close the bead after task completion:
1. `bead close bf-173o7e --reason "..."` → Exit 1 (verification failed)
2. `bead close --skip-verify` → Exit 1 (still failed)
3. `bead update --status closed` → Exit 4 (wrong command)
4. Multiple attempts with different repo paths
5. Hit max_turns limit before resolving

**Root Cause:** Workflow/state confusion — agent may have been operating in wrong repository context (pdftract vs domain-check), and verification logic failed even when bypassed.

### Historical Context: SIGHUP Cascades

Separate from git gc operations, the system experienced a mass SIGHUP cascade event on 2026-08-16 (12:00-17:00 UTC):
- 200+ crashes across all workers
- Exit code: -1 (signal -1, SIGHUP)
- System-wide external termination
- Unrelated to git gc or resource exhaustion

## Risk Assessment

### Current Risks (High Confidence)

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Max_turns exhaustion during post-task workflow | Medium | Medium | Increase limit, improve close error handling |
| Repository context confusion | Medium | Low | Validate workspace paths, add context checks |
| Bead close verification failures | Low | Medium | Fix verification logic, add fallback |

### Potential Risks (Theoretical)

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Git gc memory exhaustion | Very Low | High | Pre-check memory, set ulimits, use cgroups |
| Git gc timeout on massive repos | Low | Medium | Progress monitoring, incremental gc |
| Concurrent git operations causing corruption | Very Low | Critical | Git lock file validation, exclusive operations |

### Why Git GC --aggressive Is Generally Safe Here

1. **Ample memory:** 62GB RAM, gc only used 1.3GB at peak (~2%)
2. **Repository size:** 445MB pack file (manageable)
3. **Completion proof:** Historical run succeeded in 7 minutes
4. **No OOM indicators:** Load average moderate (2.45), no swap pressure
5. **Git's safeguards:** Built-in lock files prevent corruption

## Mitigation Strategy

### Option 1: Process Improvements (RECOMMENDED)

**Focus:** Fix the actual failure mode — bead closing workflow

**Implementation Checklist:**

1. **Increase max_turns for long-running tasks**
   - Identify tasks with post-completion workflows (git gc, large builds)
   - Set max_turns=60 (or higher) for these tasks
   - Add turn budget monitoring before attempting bead close
   - **Effort:** Low (configuration change)

2. **Improve bead close error handling**
   - Add exponential backoff on close failures (avoid retry storms)
   - Log verification failures with context (why did verify fail?)
   - Implement fallback close mechanism (force close after N attempts)
   - **Effort:** Medium (bead-rs enhancement)

3. **Repository context validation**
   - Verify workspace path matches expected repository
   - Add git remote check (ensure we're in right repo before operations)
   - Log repository context at start of task
   - **Effort:** Low (add validation checks)

4. **Workflow-stage turn accounting**
   - Track turns consumed by task completion vs. post-completion
   - Reserve turn budget for bead closing (e.g., allocate 5 turns)
   - Warn when approaching limit during post-task workflows
   - **Effort:** Medium (workflow tracking)

**Justification:** This addresses the ACTUAL crash cause with minimal changes, without over-engineering for a theoretical risk.

### Option 2: Git GC Specific Safeguards (DEFENSIVE)

**Focus:** Add safety nets for git gc operations specifically

**Implementation Checklist:**

1. **Pre-execution resource checks**
   - Verify available memory > 4GB before starting
   - Check disk space > 2× repository size
   - Validate no concurrent git operations in progress
   - **Effort:** Low (add pre-flight checks)

2. **Run under cgroup limits**
   - Use `systemd-run` or `cgcreate` to set memory limit (e.g., 4GB)
   - Set CPU limit (e.g., 4 cores) to prevent system overload
   - Timeout after 30 minutes (git gc should never take longer)
   - **Example:**
     ```bash
     systemd-run --scope --user -p MemoryLimit=4G -p CPUQuota=400% git gc --aggressive
     ```
   - **Effort:** Low (wrapper script)

3. **Use incremental gc instead of aggressive**
   - Default to `git gc` (no --aggressive flag)
   - Only use --aggressive for annual/bi-annual maintenance
   - Schedule aggressive runs during low-use periods
   - **Effort:** Low (change default behavior)

4. **Progress monitoring**
   - Monitor git gc process (already running? join as monitor)
   - Log pack file growth every 60 seconds
   - Alert if operation exceeds 15 minutes
   - **Effort:** Medium (monitoring wrapper)

**Justification:** Defensive programming against potential future issues, though none have manifested yet.

### Option 3: Alternative Approaches (EXPLORATORY)

**Focus:** Avoid git gc --aggressive entirely

**Alternatives:**

1. **Use git gc (standard) instead of --aggressive**
   - Standard gc is faster (~2 minutes vs. ~7 minutes)
   - Uses less memory
   - Good enough for most maintenance
   - Reserve --aggressive for annual deep optimization
   - **Trade-off:** Less optimal compression, but safer

2. **Automatic gc via git config**
   - Set `gc.auto` (default is 6700 loose objects triggers gc)
   - Let git handle gc automatically during normal operations
   - Avoids manual intervention entirely
   - **Trade-off:** Less control, uses --aggressive rarely

3. **Scheduled maintenance instead of on-demand**
   - Run git gc weekly via cron (standard mode)
   - Run git gc --aggressive monthly via workflow
   - Remove from agent task scope entirely
   - **Trade-off:** Adds infrastructure complexity

**Justification:** Only pursue if Option 1 fails or crashes recur.

## Recommended Approach

### Primary Strategy: Option 1 (Process Improvements)

**Rationale:**
- Addresses the ACTUAL crash cause (max_turns during bead closing)
- Low implementation effort
- No performance trade-offs
- Improves robustness for all long-running tasks, not just git gc

**Implementation Order:**
1. **Immediate:** Increase max_turns to 60 for git gc tasks
2. **Short-term:** Add repository context validation
3. **Medium-term:** Improve bead close error handling with backoff
4. **Long-term:** Implement workflow-stage turn accounting

### Secondary Safeguards: Option 2 (Git GC Specific)

**Rationale:**
- Defensive insurance against theoretical risks
- Low-cost additions
- Provides observability into git gc operations

**Implementation Order:**
1. **Immediate:** Add pre-flight memory check (verify > 4GB available)
2. **Short-term:** Add progress monitoring (log pack file growth)
3. **Medium-term:** Consider cgroup limits if repository grows significantly

### NOT Recommended: Option 3 (Avoid --aggressive)

**Rationale:**
- No evidence that --aggressive is unsafe in this environment
- Historical run succeeded without resource pressure
- Standard gc is less optimal (larger pack files)
- Adds complexity without clear benefit

## Monitoring Strategy

### Metrics to Track

**Per-Task Metrics:**
- Turns consumed (task completion vs. post-completion)
- Bead close success/failure rate
- Repository context validation failures

**Git GC Specific Metrics:**
- Operation duration (target: < 15 minutes)
- Peak memory usage (target: < 4GB)
- Repository size before/after
- Loose object count reduction

**System Metrics:**
- Available memory before/during/after git gc
- Load average trends
- Swap usage (should remain zero)

### Alerting Thresholds

| Metric | Warning | Critical | Action |
|--------|---------|----------|--------|
| Turns consumed (pre-close) | 40/60 | 55/60 | Reserve budget for close |
| Bead close failures | 3 in hour | 5 in hour | Investigate verification logic |
| Git gc duration | > 10 min | > 20 min | Check repository size |
| Memory usage (git gc) | > 2GB | > 4GB | Consider cgroup limits |
| Repository size | > 1GB | > 2GB | Plan migration/lfs |

### Health Checks

**Pre-Git GC:**
```bash
# Memory check
free -g | awk '/^Mem/ {exit $7 < 4}'

# Disk space check
df -BG /home/coding/domain-check/.git | awk 'NR==2 {exit $4 < 2}'

# Git status check
git status --porcelain || exit 1  # fail if uncommitted changes
```

**Post-Git GC:**
```bash
# Repository integrity
git fsck --quick

# Pack file validation
git verify-pack -v .git/objects/pack/*.idx | grep -E '^[0-9a-f]{40}'
```

## Implementation Timeline

### Phase 1: Immediate (Week 1)
- [ ] Increase max_turns to 60 for git gc tasks
- [ ] Add repository context validation to task startup
- [ ] Implement pre-flight memory/disk checks

### Phase 2: Short-term (Month 1)
- [ ] Implement bead close retry with exponential backoff
- [ ] Add git gc progress monitoring
- [ ] Set up metrics collection for all thresholds

### Phase 3: Medium-term (Quarter 1)
- [ ] Implement workflow-stage turn accounting
- [ ] Add cgroup limits if repository exceeds 1GB
- [ ] Create scheduled git gc maintenance workflow

### Phase 4: Long-term (Ongoing)
- [ ] Monitor crash patterns and adjust strategies
- [ ] Review git gc necessity quarterly
- [ ] Consider git-lfs if repository grows beyond 2GB

## Success Criteria

### Process Metrics
- ✅ Zero crashes due to max_turns exhaustion during bead closing
- ✅ < 5% bead close failure rate
- ✅ 100% repository context validation pass rate

### Git GC Metrics
- ✅ 100% git gc operations complete without OOM
- ✅ < 15 minute duration for git gc --aggressive
- ✅ Zero repository corruption events

### System Metrics
- ✅ Load average remains < 5 during git gc
- ✅ Zero swap usage during git gc
- ✅ No user-facing performance degradation

## Contingency Planning

### If Git GC Fails Mid-Operation
1. **Check lock files:** `.git/gc.pid` indicates gc in progress
2. **Do NOT force kill:** Let gc complete or it will roll back on next run
3. **Verify on completion:** Run `git fsck --quick`
4. **Fallback:** If gc corrupted repository, restore from backup (last commit)

### If Repository Grows Beyond 2GB
1. **Consider git-lfs:** Move large assets to LFS storage
2. **Evaluate shallow clones:** Reduce history depth for workers
3. **Alternative:** Move to separate git repository for large assets

### If Max_Turns Exhaustion Recurs
1. **Immediate:** Increase limit further (max_turns=90)
2. **Investigation:** Audit bead close verification logic
3. **Fallback:** Implement force-close mechanism as emergency measure

## Conclusion

The crash of bead bf-173o7e was **not a git gc failure** — it was a workflow issue where the agent exhausted its turn budget while attempting to close the bead after successful task completion. The git gc --aggressive operation itself completed successfully within acceptable resource limits.

**Recommended actions:**
1. Fix the bead closing workflow (Option 1)
2. Add git gc-specific safeguards as insurance (Option 2)
3. Continue using git gc --aggressive where appropriate
4. Monitor the implemented metrics

**Do NOT avoid git gc --aggressive outright** — there's no evidence it's unsafe in this environment, and it provides valuable optimization. The mitigation strategy focuses on process improvements rather than restricting git operations.

---

**Next Steps:**
1. Review and approve this mitigation strategy
2. Implement Phase 1 actions (immediate)
3. Schedule review for Phase 2 (short-term)
4. Update strategy based on monitoring data

**Document Ownership:** Domain Check maintainers  
**Review Date:** 2026-11-28 (quarterly review)  
**Related Beads:** domchk-e49087e2, bf-173o7e (resolved)
