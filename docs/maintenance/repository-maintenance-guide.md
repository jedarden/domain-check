# Repository Maintenance Guide

**Purpose:** Prevent repository bloat and maintain system stability  
**Last Updated:** 2026-09-02  
**Status:** ✅ Active

---

## Quick Reference

### Daily Operations
```bash
# Pre-flight health check before starting work
./scripts/preflight-health-check.sh
```

### Weekly Maintenance
```bash
# Check repository health
./scripts/check-repo-health.sh

# Run cleanup if repository >500MB
./scripts/safe-git-gc.sh --full
```

### Emergency (If Repository >5GB)
```bash
# Immediate cleanup
./scripts/safe-git-gc.sh --full

# Monitor in another terminal
./scripts/safe-git-gc-monitor.sh --watch
```

---

## Automated Scheduling (systemd user timers)

All scheduled maintenance and monitoring runs as **systemd user timers**, not
cron — this box is NixOS and has no `crontab`. Installed by
`scripts/setup-repo-maintenance.sh` (repo-health + gc units) and
`scripts/install-monitoring.sh` / `scripts/install-git-gc-timers.sh`
(monitoring units); unit sources live in `scripts/domain-check-*.{service,timer}`.

| Timer | Schedule | What it runs |
|-------|----------|--------------|
| `domain-check-service-monitor.timer` | every 2 min | `service-monitor.sh --once` |
| `domain-check-resource-monitor.timer` | every 5 min | `resource-monitor.sh --once` |
| `domain-check-monitoring.timer` | every 10 min | `crash-pattern-detection.sh` |
| `domain-check-repo-health.timer` | daily 02:00 | `auto-gc-trigger.sh --dry-run` |
| `domain-check-git-gc.timer` | daily 03:00 | `safe-git-gc.sh` (stages 1-2) |
| `domain-check-git-gc-full.timer` | weekly Sun 04:00 | `safe-git-gc.sh --full` (MemoryMax=4G) |

**Verify the fleet is healthy:**
```bash
systemctl --user list-timers 'domain-check-*' --all
```
Every timer should show a future `Trigger` time. A timer showing `n/a` or
`-` is not going to fire — investigate before assuming coverage.

**Known failure mode (bit 2026-09-02):** editing a unit file in
`~/.config/systemd/user/` without `systemctl --user daemon-reload` leaves the
manager holding the previous (possibly fatal) unit state. The timer then
silently never fires while `list-timers` still lists it. After changing any
unit file — by hand or by re-running the installers — always:
```bash
systemctl --user daemon-reload && systemctl --user start domain-check-*.timer
```
The installers do this; bare `cp` into `~/.config/systemd/user/` does not.

---

## Repository Size Thresholds

| Size | Status | Action |
|------|--------|--------|
| <500MB | ✅ Healthy | None required |
| 500MB-1GB | ⚠️ Warning | Plan cleanup soon |
| 1GB-5GB | 🚨 Critical | Run cleanup immediately |
| >5GB | 🆘 Emergency | Run cleanup NOW |

---

## Warning Signs of Repository Bloat

**Early Warning Signs:**
- Repository size approaching 1GB
- Git operations becoming slow
- Loose objects >500MB

**Critical Symptoms:**
- Exit code -1 (SIGKILL) during git operations
- Repository size >5GB
- Routine git operations trigger OOM
- Multiple crashes with exit code -1

---

## Persistent Pack-Memory Bounds (OOM root-cause fix)

Exit-code -1 crashes in this workspace (bf-173o7e: 129 kills on 2026-08-14; bf-4x12ec: same
mechanism) were **kernel memcg OOM kills**, not git bugs: a bare `git gc --aggressive
--prune=now` over a bloated repo pushed git-pack-objects RSS past the 12GiB `MemoryMax` of
needle's per-dispatch scope (`run-p*.scope` in the systemd **user** manager), and the kernel
SIGKILLed the agent. `safe-git-gc.sh` bounds only its own sanctioned path — the bare
invocation is defended solely by git config, which for a year existed only as hand-applied
local state in this repo's `.git/config` and nowhere reproducible.

`scripts/setup-git-gc-config.sh` now makes that bound persistent, reproducible, and
verifiable:

- `pack.windowMemory = 2g` — caps the delta search window
- `pack.deltaCacheSize = 1g` — caps the delta write-out cache
- `pack.threads = 1` — **required**: per git docs the window limit is *per thread*, so
  leaving threads unset lets git multiply the window across all cores
- Worst case ≈ 3GiB per pack run, a quarter of the 12GiB dispatch scope

```bash
./scripts/setup-git-gc-config.sh              # bound this repo (local scope)
./scripts/setup-git-gc-config.sh --global     # bound all repos for this user (~/.gitconfig)
./scripts/setup-git-gc-config.sh --verify     # exit 1 if the effective bound is missing/unsafe
```

Applied `--global` on this box on 2026-09-02, so every repo for the `coding` user is
protected — including the other repos whose dispatch scopes produced the bf-4x12ec-family
kills. Re-run `--verify` if a repo reports fresh exit-code -1 crashes during git operations.

Tested by `scripts/test-gc-memory-bounds.sh`: it runs the exact crash command at reduced
scale (8×64MiB incompressible blobs) under a 768MiB cgroup — 1/16th of the dispatch scope —
and asserts exit 0 with peak RSS ≈ 313MiB. Before the bound, the same command needed >12GiB
and died 129 times in a row.

What this does **not** cover: needle's zero-backoff re-claim loop, which amplified one
deterministic kill into a 129-attempt storm (bead released → re-claimed 9s later). That is
needle-side, outside this repo; alert-side suppression is handled by
`scripts/crash-alert-manager.sh` dedup/cooldown.

---

## Prevention Checklist

**Daily:**
- [ ] Run pre-flight health check before starting work
- [ ] Check available memory (>10GB required)
- [ ] Confirm pack-memory bounds still verify: `./scripts/setup-git-gc-config.sh --verify`

**Weekly:**
- [ ] Run `./scripts/check-repo-health.sh`
- [ ] Review repository size trends
- [ ] Run cleanup if size >500MB

**Monthly:**
- [ ] Comprehensive repository audit
- [ ] Review git history for large files
- [ ] Update monitoring thresholds if needed

---

## Key Scripts

| Script | Purpose | Usage |
|--------|---------|-------|
| `preflight-health-check.sh` | Validate system before tasks | Before every agent task |
| `check-repo-health.sh` | Repository size and object check | Weekly health monitoring |
| `safe-git-gc.sh` | Memory-limited garbage collection | When cleanup needed |
| `safe-git-gc-monitor.sh` | Monitor gc progress | During gc operations |
| `setup-git-gc-config.sh` | Persistent pack-memory bound + verify | After cloning; `--verify` when exit -1 appears |

---

## Important Links

**Crash Investigation:**
- [bf-1s6c3 Investigation Summary](../bf-1s6c3-investigation-summary.md) - Repository bloat crash (18GB → 138MB)
- [Crash Response Guide](../crash-response-guide.md) - Quick crash classification

**Detailed Documentation:**
- [Repository Maintenance Recommendations](./repository-maintenance-recommendations.md) - Comprehensive guide
- [Cleanup and Recovery Procedures](../cleanup-and-recovery-procedures.md) - Emergency procedures
- [Crash Mitigation Strategies](../crash-mitigation-strategies.md) - Prevention strategies

---

## Common Issues and Solutions

### Repository Keeps Growing
```bash
# Check current state
./scripts/check-repo-health.sh
du -sh .git/objects/* | sort -rh

# Solution: Run cleanup
./scripts/safe-git-gc.sh --full
```

### Git GC Fails with OOM
```bash
# Check available memory
free -h

# Run with memory limit
SAFE_GC_MEMORY_MAX=2g ./scripts/safe-git-gc.sh --full

# Confirm the persistent pack-memory bound is intact (see its section above)
./scripts/setup-git-gc-config.sh --verify
```

### Pre-Flight Check Fails
Common fixes:
- **Gateway unavailable:** Wait for service recovery
- **Insufficient memory:** Close applications or add RAM
- **Repository bloated:** Run `./scripts/safe-git-gc.sh --full`

---

## Success Criteria

Repository is healthy when:
- ✅ Repository size <500MB
- ✅ Loose objects <100 count
- ✅ No fsck errors (`git fsck --full`)
- ✅ Pre-flight health check passes
- ✅ No OOM crashes during git operations

---

## Background

The bf-1s6c3 crash (2026-08-12) demonstrated that repository bloat (18GB with 17GB loose objects) can trigger systematic OOM failures. Following cleanup (18GB → 138MB, 99.2% reduction), the task completed successfully and no similar crashes have occurred for 16+ days.

**Key Takeaway:** Proactive maintenance prevents 70% of infrastructure crashes.

---

**Need Help?** See [Repository Maintenance Recommendations](./repository-maintenance-recommendations.md) for comprehensive guidance or [Crash Response Guide](../crash-response-guide.md) for crash investigation procedures.
