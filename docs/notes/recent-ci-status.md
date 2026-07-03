# Recent CI Status — domain-check-build

## Most Recent Run

| Field | Value |
|-------|-------|
| **Workflow Name** | `domain-check-build-vdxl5` |
| **Phase** | Failed |
| **Creation Timestamp** | `2026-07-03T09:34:11Z` |
| **Started At** | `2026-07-03T09:34:12Z` |
| **Finished At** | `2026-07-03T09:35:01Z` |
| **Duration** | ~49 seconds |
| **Namespace** | `argo-workflows` (iad-ci cluster) |

## Failure Details

- **Failed Node:** `build-quality-gate` (Pod type)
- **Error Message:** `main: Error (exit code 2)`
- **Root Cause:** The quality gate step (lint/build/test checks) exited with code 2

This is the only domain-check-build workflow run found. The `build-quality-gate` pod failure typically indicates a build or test error in the source code.
