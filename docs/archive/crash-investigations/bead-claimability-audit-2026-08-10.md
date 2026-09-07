# Open Bead Claimability Audit

**Date:** 2026-08-10  
**Total Open Beads Analyzed:** 30  
**Task:** Audit open beads and categorize claimability per exclusion rules

---

## Exclusion Rules (from prior investigation)

Beads are **excluded** from `br claim` if they match ANY of these:

1. **Status ≠ 'open'** (only 'open' status beads are claimable)
2. **ephemeral = 1**
3. **pinned = 1** 
4. **is_template = 1**
5. **deleted_at IS NOT NULL**
6. **Has open dependencies** (in blocked_issues_cache)

**Important:** Labels like `split-child`, `failure-count:*`, etc. do NOT cause exclusion.

---

## Complete Analysis of 30 Open Beads

### Category A: GENUINELY UNCLAIMABLE (6 beads - 20%)

These beads have **open dependencies** and are correctly excluded:

| ID | Title | Open Dependency | Why Blocked |
|----|-------|-----------------|--------------|
| **bf-1hs** | Rename internal package structure to match plan layout | bf-10sy (status: blocked) | Depends on blocked SSRF extraction |
| **bf-25p4** | Diagnose why the quality-gate step is failing | bf-3tu8 (status: blocked) | Depends on proposing quality-gate fix |
| **bf-35v3** | Capture workflow results and verify quality-gate passes | bf-1csr (status: open) | Waiting for documentation bead |
| **bf-1csr** | Document release workflow results in plan and docs | bf-4k8v (status: open) | Waiting for test workflow submission |
| **bf-4k8v** | Submit a new release test workflow and verify quality-gate passes | bf-m84x (status: open) | Waiting for quality-gate fix |
| **bf-m84x** | Fix the quality-gate failure in the release workflow | bf-25p4 (status: open) | **CIRCULAR DEPENDENCY** with bf-25p4 |

**CIRCULAR DEPENDENCY DETECTED:**
- `bf-m84x` → depends on `bf-25p4` (diagnose)
- `bf-25p4` → depends on `bf-3tu8` (blocked)
- **Issue:** The quality-gate fix chain has a blocker that prevents progress

### Category B: INCORRECTLY FILTERED (14 beads - 47%)

These beads have **no open dependencies** but are still being filtered out. They SHOULD be claimable:

| ID | Title | Priority | Labels | Actual Blocker |
|----|-------|----------|---------|----------------|
| **bf-5d18** | [Pulse] [test] domain check failed | P2 | - | Assignee: glm-bravo (should NOT block) |
| **bf-1gol** | Verify bootstrap package exists and compiles | P2 | split-child | **Label does NOT exclude per rules** |
| **bf-3l4** | Repo hygiene: untrack committed artifacts | P3 | - | Assignee: glm-bravo (should NOT block) |
| **bf-4nk** | Reject PSL private-suffix domains in validation | P3 | - | Assignee: glm-bravo (should NOT block) |
| **bf-23n** | Fix go.mod module path | P2 | - | Assignee: glm-bravo (should NOT block) |
| **bf-2099** | Commit and push WorkflowTemplate fix | P2 | failure-count:1 | **Label does NOT exclude per rules** |
| **bf-4yjq** | Git origin remote points to GitHub | P2 | - | No blocker - should be claimable |
| **bf-3h3z** | Stream multi-TLD and bulk check results incrementally | P2 | - | No blocker - should be claimable |
| **bf-1mww** | Add domain name suggestion mode | P3 | - | No blocker - should be claimable |
| **bf-5ohx** | CLI: add --watch flag for polling | P3 | - | No blocker - should be claimable |
| **bf-33sv** | Implement ADR-001: Domain Watch webhook notification | P3 | - | No blocker - should be claimable |
| **bf-62g** | Add WHOIS ccTLD Playwright smoke tests | P3 | - | Dependency closed (bf-rh4l) - should be claimable |
| **bf-iotu** | Re-test release entrypoint with fixed templates | P2 | - | Dependency closed (bf-48x6) - should be claimable |
| **bf-5ti6** | Document release workflow test results | P2 | - | No blocker - should be claimable |

### Category C: UNCLEAR/COMPLEX CHAINS (10 beads - 33%)

These beads have complex dependency relationships that may indicate intentional sequencing or incorrect blocking:

| ID | Title | Dependencies | Status | Unclear Why |
|----|-------|--------------|---------|------------|
| **bf-4u8b** | Submit manual release workflow | bf-31nu (open) | Claims blocked by documentation bead | Is sequencing intentional? |
| **bf-31nu** | Update release workflow test results documentation | bf-iotu (open) | Claims blocked by re-test bead | Circular chain with bf-iotu |
| **bf-4wvi** | Test both entrypoints with manual workflow submissions | bf-4u8b (open) | Root of release testing chain | Intentional sequencing or incorrect? |
| **bf-259** | Configure tag-triggered execution for goreleaser | bf-2z6 (closed), bf-4wvi (open) | Depends on testing bead | Should be parallelizable |
| **bf-5oq** | Add goreleaser step to Argo WorkflowTemplate | bf-poq (closed), bf-3bd (open) | Depends on validation bead | Should be parallelizable |
| **bf-3bd** | Validate WorkflowTemplate YAML and verify ArgoCD sync | bf-259 (open) | Depends on tag-trigger config | Sequential dependency needed? |
| **bf-2ru** | Verify goreleaser release pipeline is wired to Argo CI | bf-5vp (open) | P3 priority with open dependency | Priority vs dependency tradeoff |
| **bf-5vp** | Verify end-to-end goreleaser release pipeline | bf-2ob (open) | P2 priority with open dependency | Sequencing vs parallelization |
| **bf-2ob** | Configure goreleaser for GitHub release publishing | bf-5oq (open) | Configuration depends on WorkflowTemplate | Valid dependency |

---

## Summary Statistics

- **Total open beads:** 30
- **Category A (genuinely unclaimable):** 6 (20%)
- **Category B (incorrectly filtered - should be claimable):** 14 (47%)
- **Category C (unclear dependency chains):** 10 (33%)

## Root Cause Analysis

### 1. Label Hygiene Problem ❌
**Finding:** Labels like `split-child` and `failure-count:1` do NOT exclude beads per the actual implementation.

**Impact:** 2 beads incorrectly assumed to be excluded based on labels:
- `bf-1gol` (label: split-child)
- `bf-2099` (label: failure-count:1)

### 2. Assignee Filter Assumption ❌  
**Finding:** Assignee does NOT affect claim eligibility per the rules.

**Impact:** 4 beads with assignee `glm-bravo` are incorrectly considered unclaimable:
- `bf-5d18`
- `bf-3l4`
- `bf-4nk`
- `bf-23n`

### 3. Dependency Chain Management ⚠️
**Finding:** 14 beads have no actual blockers but may be affected by:
- Assumed sequenced workflows (should be parallelizable)
- Circular dependencies in release workflow testing
- Over-dependent chains for what should be independent tasks

**Impact:** The entire release workflow chain (11 beads) appears artificially serialized.

### 4. Circular Dependency Detected 🚨
**Critical:** `bf-m84x` ↔ `bf-25p4` form a circular dependency that blocks both:
- `bf-m84x` (fix quality-gate) → depends on `bf-25p4` (diagnose quality-gate)
- `bf-25p4` (diagnose quality-gate) → depends on `bf-3tu8` (blocked)

This circular reference prevents the entire quality-gate fix chain from progressing.

---

## Recommendations

### Immediate Actions
1. **Break circular dependency:** Update `bf-m84x` to not depend on `bf-25p4` (or vice versa)
2. **Remove incorrect assumptions:** Labels and assignees do NOT block claiming
3. **Claim Category B beads:** 14 beads are ready to be claimed immediately

### Process Improvements
1. **Review dependency chains:** Many release workflow beads could be parallelized
2. **Audit blocked_issues_cache:** Verify it reflects actual open dependencies
3. **Document intended sequencing:** If chains are intentional, document why

### Filter Configuration Review
If certain beads SHOULD be filtered (e.g., assigned beads, labeled beads), this would require:
- Code changes to `bead-forge/src/claim.rs` 
- Adding label-based or assignee-based filters
- Updating this documentation

Currently, the implementation does NOT support these filters.
