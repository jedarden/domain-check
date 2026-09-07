# Domain Check Project Memory

This directory stores persistent project memory for the domain-check workspace.

## Memory Files

- [Crash Fix Implementation Report - bf-1s6c3](docs/crash-fix-implementation-report-bf-1s6c3-2026-09-01.md) — Complete implementation of Phase 1 crash mitigation strategies (repository monitoring, safe git GC, resource monitoring, pre-flight checks). Domain-check code is defect-free; all crashes are infrastructure/workflow/service-related.
- [Repository Bloat OOM Crashes](docs/crash-artifacts-bf-4yjq.md) — Incident bf-4yjq: 9 crashes over 2.5 hours from 18GB repository with 17GB loose objects causing OOM during routine git operations
- [SIGHUP Git Merge Crash Pattern](memory/crash-pattern-sighup-git-merge.md) — Exit code -1 during long-running git operations with 661+ divergent commits under high load

## Memory Format

Each memory file contains:
- **Frontmatter:** name, description, metadata (type: user|feedback|project|reference)
- **Body:** The fact being documented
- **Links:** Connections to related memories via [[name]] syntax

## Memory Types

- **user:** Who the user is (role, expertise, preferences)
- **feedback:** Guidance on how agents should work (corrections, confirmed approaches)
- **project:** Ongoing work, goals, or constraints not derivable from code/git history
- **reference:** Pointers to external resources (URLs, dashboards, tickets)
