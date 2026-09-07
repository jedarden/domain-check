# Repository Bloat Remediation Plan

**Created:** 2026-09-01  
**Repository:** domain-check  
**Current Size:** 1.7 GB (not 18 GB as initially reported)

## Executive Summary

The domain-check repository is **actually quite healthy at 1.7 GB total**. The initial report of 18 GB appears to be incorrect or referring to a different repository. However, this document provides a comprehensive cleanup plan for the current state and establishes baseline metrics for future reference.

## Current Repository State

### Total Size Breakdown (1.7 GB)

| Directory | Size | Description | Status |
|-----------|------|-------------|--------|
| `dist/` | 124 MB | GoReleaser build artifacts (9 platform binaries) | ⚠️ Should be gitignored |
| `node_modules/` | 22 MB | Node.js dependencies (Playwright testing) | ✅ Gitignored, but present in workspace |
| `domain-check/` | 21 MB | Local build binary | ⚠️ Should be gitignored |
| `internal/` | 14 MB | Go source code | ✅ Tracked in git |
| `docs/` | 6.2 MB | Documentation | ✅ Tracked in git |
| `web/` | 1.7 MB | Frontend assets (templates, static files) | ✅ Tracked in git |
| `.git/` | 92 MB | Git repository metadata | ✅ Healthy |

### Git Repository Health

```
count: 240
size: 1.61 MiB
in-pack: 9,164
packs: 1
size-pack: 88.70 MiB
prune-packable: 0
garbage: 0
size-garbage: 0 bytes
```

**Assessment:** Git repository is in excellent condition. Only one pack file, no garbage, no loose objects needing pruning.

## Cleanup Opportunities

### 1. Build Artifacts (145 MB recoverable)

#### `dist/` Directory (124 MB)

**Contents:**
- 9 platform-specific binaries (~14 MB each)
- `artifacts.json`, `config.yaml`, `metadata.json` (GoReleaser metadata)

**Issue:** These are GoReleaser build outputs that should NOT be in the repository.

**Fix:** Already gitignored (`*.exe` and `dist/` in `.gitignore`), but directory exists from a prior build.

**Cleanup Command:**
```bash
rm -rf dist/
```

**Prerequisites:** None (build artifacts are reproducible via `goreleaser release`)

---

#### Local Binary `domain-check` (21 MB)

**Issue:** Local build binary in repository root.

**Fix:** Should be gitignored but isn't explicitly listed.

**Cleanup Command:**
```bash
rm -f domain-check
```

**Prerequisites:** None (reproducible via `go build ./cmd/domain-check`)

---

### 2. Node Dependencies (22 MB recoverable)

#### `node_modules/` Directory

**Purpose:** Playwright end-to-end testing dependencies for web UI.

**Status:** Already gitignored (correctly), but present in workspace for testing.

**Cleanup Command:**
```bash
rm -rf node_modules/
npm install  # To restore when needed for testing
```

**Prerequisites:** Note `npm install` command in documentation for future testing setup.

---

## Git Cleanup Operations

### Current State: Excellent (No Action Needed)

The git repository is already optimized:
- Single pack file (88.70 MB)
- No garbage objects
- No loose objects requiring pruning
- Reflog is minimal (dry-run showed nothing to expire)

### Maintenance Operations (If Needed in Future)

#### Standard Garbage Collection

```bash
# Safe, conservative gc (10-30 minutes)
git gc

# With aggressive compression (1-2 hours, higher memory)
git gc --aggressive --prune=now
```

**Memory Requirements:**
- Standard gc: ~1-2 GB peak
- Aggressive gc: ~4 GB peak

**Risks:** None. Git gc is safe and idempotent.

---

#### Reflog Expiration

```bash
# Expire reflog entries older than 90 days
git reflog expire --expire=90days --all

# Expire all reflog entries (advanced, loses recovery safety)
git reflog expire --expire=now --all
```

**Memory Impact:** Minimal (reflog is text-based, ~KB)

**Recovery Loss:** Reflog entries provide a 90-day safety net for recovering from accidental deletions. Expiring them reduces this safety net.

**Recommendation:** Keep default 90-day expiry for safety.

---

#### Loose Object Pruning

```bash
# Prune loose objects older than 2 weeks (default)
git prune

# Prune all loose objects (advanced)
git prune --expire=now
```

**Current State:** No loose objects to prune (`prune-packable: 0`)

**Recommendation:** Not needed currently.

---

## Disk Recovery Potential

### Immediate Recovery: 145 MB

| Source | Size | Safety |
|--------|------|--------|
| `dist/` binaries | 124 MB | ✅ Safe (reproducible) |
| `domain-check` binary | 21 MB | ✅ Safe (reproducible) |

### With `node_modules/` Removal: 167 MB Total

**Note:** Only remove `node_modules/` if not actively running Playwright tests.

---

## Prerequisites and Safety Checks

### Pre-Cleanup Checklist

- [ ] Confirm no uncommitted changes: `git status`
- [ ] Verify tests pass: `go test ./...`
- [ ] Check available disk space: `df -h /` (need 10+ GB free for safety)
- [ ] Backup critical data (optional but recommended for first-time gc)

### Pre-Git-GC Checklist

- [ ] Close any IDE or git GUI tools
- [ ] Check available memory: `free -h` (need 2+ GB free for aggressive gc)
- [ ] Verify repository integrity: `git fsck` (optional, for peace of mind)

---

## Risks and Mitigations

### Risk 1: Accidental Deletion of Needed Files

**Risk Level:** Low  
**Mitigation:** All identified cleanup items are build artifacts reproducible from source.

**Recovery:**
- `dist/` → Run `goreleaser release`
- `domain-check` → Run `go build ./cmd/domain-check`
- `node_modules/` → Run `npm install`

---

### Risk 2: Git GC Data Loss

**Risk Level:** Very Low  
**Mitigation:** Git gc never deletes reachable objects. Only unreachable (garbage) objects are pruned.

**Recovery:** If reflog is intact, you can recover commits from the last 90 days even after aggressive gc.

---

### Risk 3: Memory Pressure During Git GC

**Risk Level:** Low on this system (62 GB RAM)  
**Mitigation:** Use staged gc from `scripts/safe-git-gc.sh` instead of manual `git gc --aggressive`.

**Alternative:** Run standard `git gc` (lower memory) instead of aggressive mode.

---

## Implementation Plan

### Phase 1: Build Artifact Cleanup (Immediate)

```bash
# Remove build artifacts (145 MB recovered)
rm -rf dist/
rm -f domain-check

# Verify cleanup
du -sh .
git status
```

**Expected Result:** Repository shrinks from 1.7 GB to ~1.55 GB

---

### Phase 2: Git Maintenance (Optional)

**Note:** Current git state is excellent. Skip unless repository grows significantly.

```bash
# Standard maintenance (safe, fast)
git gc

# Verify integrity
git fsck
```

**Expected Result:** Git pack size may reduce slightly, repository integrity verified.

---

### Phase 3: Monitoring (Ongoing)

Track repository size over time with baseline metrics:

```bash
# Record baseline
echo "$(date): Repository size $(du -sh . | cut -f1)" >> docs/metrics/repo-size.log

# Record git metrics
git count-objects -vH >> docs/metrics/git-objects.log
```

**Alert Threshold:** Investigate if repository exceeds 5 GB.

---

## Post-Cleanup Verification

### Size Verification

```bash
# Confirm cleanup
du -sh .                    # Should show ~1.55 GB
du -sh .git                 # Should remain ~92 MB
git count-objects -vH       # Should show no garbage
```

### Functionality Verification

```bash
# Ensure project still builds
go build ./cmd/domain-check

# Ensure tests pass
go test ./...

# Ensure git operations work
git log --oneline | head -5
git status
```

---

## Baseline Metrics (2026-09-01)

| Metric | Value | Target |
|--------|-------|--------|
| Total repository size | 1.7 GB | < 5 GB |
| Git metadata size | 92 MB | < 500 MB |
| Git pack files | 1 | Minimize |
| Loose objects | 0 | 0 |
| Garbage objects | 0 | 0 |
| Reflog entries | Minimal | < 1000 |

---

## Related Documentation

- **Git GC Safety:** `docs/research/crash-incident-summary-domain-check-2026-08-26.md`
- **Safe GC Scripts:** `scripts/safe-git-gc.sh`
- **GC Monitoring:** `scripts/safe-git-gc-monitor.sh`
- **Crash Prevention:** `docs/crash-mitigation-strategies.md`

---

## Summary

The domain-check repository is **healthy and well-maintained** at 1.7 GB. The initial 18 GB figure appears to be incorrect. Immediate cleanup of build artifacts can recover 145 MB (167 MB with `node_modules/`). Git metadata is optimized and requires no maintenance at this time.

**Recommendation:** Execute Phase 1 (build artifact cleanup) and monitor repository size quarterly. Git gc operations are not currently needed.
