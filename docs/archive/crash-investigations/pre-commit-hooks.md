# Pre-commit Hooks

## Overview

This repository uses Git pre-commit hooks to prevent large file additions that could cause repository bloat and OOM crashes during git operations.

## Large File Protection

The pre-commit hook blocks commits containing files larger than **50MB**.

### Why This Exists

This protection was implemented as Phase 2.2 of the root cause analysis following OOM crashes caused by repeated commits of large `.beads/` files (237MB+). The hook prevents future incidents by blocking large files at commit time.

### How It Works

The `.githooks/pre-commit` script runs before every commit and checks:
- Individual file size: max 50MB per file
- Total commit size: max 100MB per commit
- Problematic file patterns: warns about `.jsonl`, `.db`, `.db.backup` files

### Error Messages

If you attempt to commit a file larger than 50MB, you'll see:

```
❌ ERROR: Large files detected in commit:
   - your-file.tmp (60MB)

Maximum file size: 50MB

Solutions:
1. Add these files to .gitignore
2. Use git-lfs for large binary files
3. Compress or reduce file sizes
```

## Configuration

The hook is configured via `core.hookspath=.githooks`, which means hooks are stored in the `.githooks/` directory instead of `.git/hooks/`. This allows hooks to be version-controlled with the repository.

### File Size Limits

Current limits (defined in `.githooks/pre-commit`):
- **MAX_FILE_SIZE**: 52428800 bytes (50MB)
- **MAX_COMMIT_SIZE**: 104857600 bytes (100MB)

## Bypassing the Hook

If you need to bypass the hook for a legitimate reason:

```bash
git commit --no-verify -m "Your message"
```

Use this sparingly and only when you're certain the large file should be committed.

## Testing

To test the hook:

```bash
# Test that it blocks large files (should fail)
dd if=/dev/zero of=large-test.tmp bs=1M count=60
git add large-test.tmp
git commit -m "test large file"  # Should be blocked

# Test that it allows normal files (should succeed)
echo "test" > normal-test.txt
git add normal-test.txt
git commit -m "test normal file"  # Should succeed
```

## Maintenance

The hook is maintained in `.githooks/pre-commit` and automatically applies to all commits in this repository. To modify the behavior, edit that file directly.
