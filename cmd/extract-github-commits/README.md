# GitHub-Specific Commits Extraction Tool

## Purpose

This tool extracts and analyzes commits that exist on the GitHub mirror but not on the Forgejo origin repository. It is part of the bead tracking system for monitoring repository synchronization status.

## Usage

```bash
# Basic usage (default branches: github/main vs origin/main)
go run cmd/extract-github-commits/main.go

# Custom branches
go run cmd/extract-github-commits/main.go github/main origin/main

# Custom branches and output file
go run cmd/extract-github-commits/main.go github/main origin/main custom-state.json
```

## Output

The tool generates a JSON state file (`.github-commits-state.json` by default) containing:

```json
{
  "common_ancestor": "63ba02474c9b6bc339388adb3a44542e10755a10",
  "github_branch": "github/main",
  "forgejo_branch": "origin/main",
  "extracted_at": "2026-08-13T10:59:23-04:00",
  "total_commits": 0,
  "github_commits": [
    {
      "sha": "commit SHA",
      "author": "Author Name",
      "date": "2026-08-13T10:00:00-04:00",
      "message": "Commit message"
    }
  ]
}
```

## How It Works

1. **Fetches both remotes** - Ensures local refs are up-to-date
2. **Finds common ancestor** - Uses `git merge-base` to find the divergence point
3. **Extracts GitHub-specific commits** - Uses `git log <common-ancestor>..<github-branch>`
4. **Captures commit details** - SHA, author, date, and message
5. **Saves to state file** - JSON format for use by subsequent beads

## Acceptance Criteria Met

✅ List of commits unique to GitHub is generated using `git log <common-ancestor>..<github-branch>`
✅ Count of GitHub-specific commits is calculated  
✅ Commit SHAs, authors, dates, and messages are captured
✅ Data is saved to temporary state file for use by subsequent beads

## Current Status

As of the latest extraction, there are **0 GitHub-specific commits**. The Forgejo and GitHub branches are in sync at commit `63ba02474c9b6bc339388adb3a44542e10755a10`.
