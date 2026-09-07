# BR Claim Exclusion Rules

**Investigation Date:** 2026-08-10  
**Scope:** Determine exactly which labels, statuses, and conditions cause `bf claim` (formerly `br claim`) to skip beads.

## Summary

`bf claim` does NOT filter by labels at all. Labels like `deferred`, `umbrella`, `split-child`, `failure-count:*` are purely informational and have **no effect** on claim eligibility. The only exclusions are based on status, flags, and dependency blocking.

## Complete Exclusion Rules

A bead is excluded from `bf claim` results if ANY of the following conditions are true:

### 1. Status Exclusions

The claim query filters for `status = 'open'` only. Beads with any other status are excluded:

- `in_progress` — **Excluded** (already claimed/being worked on)
- `closed` — **Excluded** (terminal status)
- `tombstone` — **Excluded** (terminal status)
- `blocked` — **Excluded** (not an open bead)
- `Deferred` — **Excluded** (custom status, not `open`)
- Any other custom status that is not exactly `open`

**Terminal statuses that unblock dependencies:** `closed`, `tombstone`, `done`, `completed` (these satisfy blocking dependencies, but the beads themselves are not claimable).

### 2. Flag Exclusions (on the `issues` table)

- `ephemeral = 1` — **Excluded** (temporary/meta-beads not intended for execution)
- `pinned = 1` — **Excluded** (pinned beads are held for manual assignment)
- `is_template = 1` — **Excluded** (template beads are not executable)
- `deleted_at IS NOT NULL` — **Excluded** (soft-deleted beads)

### 3. Dependency Blocking (via `blocked_issues_cache`)

A bead is excluded if it has unresolved blocking dependencies. The `blocked_issues_cache` table is populated with beads that have:

```sql
-- A bead is blocked if it has a dependency where:
-- 1. The dependency type is one of: blocks, parent-child, conditional-blocks, waits-for
-- 2. The dependency target's status is NOT IN ('closed', 'tombstone', 'done', 'completed')

INSERT INTO blocked_issues_cache (issue_id, blocked_by, blocked_at)
SELECT d.issue_id, '[' || GROUP_CONCAT('\"' || d.depends_on_id || '\"') || ']' AS blocked_by, ?1
FROM dependencies d
INNER JOIN issues i ON i.id = d.depends_on_id
WHERE d.type IN ('blocks', 'parent-child', 'conditional-blocks', 'waits-for')
AND i.status NOT IN ('closed', 'tombstone', 'done', 'completed')
```

**Example:** If bead `bf-A` depends on bead `bf-B` with type `blocks`, and `bf-B` has status `open`, then `bf-A` is excluded from claim results until `bf-B` reaches a terminal status.

### 4. Migration Lock

If a migration is in progress (`migration_lock` table has an unexpired row), `bf claim` returns `None` (no beads available) rather than risking concurrent access during schema changes.

## What Does NOT Cause Exclusion

### Labels (No Effect)

The following labels have **zero impact** on claim eligibility:

- `deferred` — Informational only
- `umbrella` — Informational only  
- `split-child` — Informational only
- `failure-count:N` — Informational only (e.g., `failure-count:4`)
- Any other label — Labels are NOT queried in the claim logic at all

**Evidence:** The claim SQL in `src/claim.rs` never joins to the `labels` or `bead_labels` tables. Labels exist only for categorization and display purposes.

### Config-Driven Filters (None Exist)

There is NO configuration-driven filtering in `bf claim`. The following do NOT exist:

- `.beads/config.yaml` setting `exclude_labels` — Does not exist
- `.beads/config.yaml` setting `claim_filters` — Does not exist
- Any other config file that affects claim eligibility

**Evidence:** No config parsing in `src/claim.rs`, and grepping the entire codebase for `exclude_labels` and `claim_filters` returns zero results.

### `in_progress` Status Reclamation

`bf claim` DOES reclaim stale `in_progress` beads back to `open` before selecting candidates. If a bead has `status = 'in_progress'` AND `updated_at < (NOW - claim_ttl_minutes)`, it is reclaimed to `open` and becomes eligible for claiming.

- Default TTL: 30 minutes (configurable via `--claim-ttl` flag)
- Purpose: Prevents abandoned claims from permanently blocking beads
- Count: The number of reclaimed beads is returned in `ClaimResult.reclaimed`

## Claim Query (Core WHERE Clause)

```sql
SELECT i.id
FROM issues i
LEFT JOIN dependencies d ON d.depends_on_id = i.id
    AND d.type IN ('blocks', 'parent-child', 'conditional-blocks', 'waits-for')
LEFT JOIN critical_path_cache c ON c.bead_id = i.id
WHERE i.status = 'open'                    -- ONLY open beads
  AND i.ephemeral = 0                       -- Non-ephemeral
  AND i.pinned = 0                          -- Non-pinned
  AND i.is_template = 0                     -- Non-template
  AND i.deleted_at IS NULL                  -- Not soft-deleted
  AND i.id NOT IN (                          -- Not blocked by unresolved dependencies
      SELECT issue_id FROM blocked_issues_cache
  )
GROUP BY i.id
ORDER BY (downstream_impact, critical_path_bonus, priority, created_at) DESC
LIMIT 1
```

## Key Findings

1. **`failure-count:4` causes neither exclusion nor deprioritization** — It's just a label, not part of the scoring formula or exclusion logic. The claim scoring formula uses: downstream_impact (count of blocking dependencies), critical_path_bonus (1000/(float+1)), priority (lower is better), and created_at (older is better).

2. **`split-child` is informational only** — It does not exclude a bead from claiming. It's simply a label for organizational purposes.

3. **`deferred` status IS excluded** — But not because of the label. If a bead's `status` field is set to `Deferred` (a custom status), it fails the `status = 'open'` check. The label `deferred` is irrelevant; the status field is what matters.

4. **No config-driven claim filters exist** — All filtering is hardcoded in the SQL query. There's no user-facing way to add custom exclusion rules without modifying the source code.

## References

- **Claim implementation:** `/home/coding/bead-forge-source/src/claim.rs` (lines 218-322)
- **Blocked issues cache:** `/home/coding/bead-forge-source/src/storage/sqlite.rs` (lines with `INSERT INTO blocked_issues_cache`)
- **Schema definition:** `/home/coding/bead-forge-source/src/storage/schema.rs` (full table definitions)
- **Terminal status list:** `closed`, `tombstone`, `done`, `completed` (from `blocked_issues_cache` population query)
