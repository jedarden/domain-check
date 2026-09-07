# Pod Absence Confirmation — bead bf-allu

**Date:** 2026-07-04

## Status: Pod absence confirmed — quality-gate logs already captured

### Pod check (most recent failed workflow)

| Field | Value |
|-------|-------|
| Workflow | `domain-check-build-qd9vp` |
| Phase | Failed |
| Failed step | `build-quality-gate` — exit code 2 |
| Finished at | 2026-07-04T03:11:03Z |
| Pod query | `No resources found` |
| Reason | podGC `OnPodCompletion` (cluster-level default) |

All pods from `domain-check-build-qd9vp` were deleted immediately on completion by the Argo Workflows controller's default `podGC: OnPodCompletion` strategy. No pods exist for any recent domain-check-build workflow runs.

### Raw quality-gate logs: already captured

The quality-gate logs were previously captured from debug workflow `domain-check-build-debug-qg-wl5sw` (which used `podGC: OnWorkflowCompletion` override):

**File:** `docs/research/quality-gate-logs-2026-07-04.txt`

Key output from the captured logs:
```
go: downloading github.com/peterbourgon/ff/v4 v4.0.0-beta.1
go: downloading github.com/stretchr/testify v1.11.1
...
+ go vet ./...       ← succeeded
+ go test -race ./... ← FAILED: exit code 2
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
```

### Root cause (confirmed)

`golang:1.26-alpine` lacks `gcc`/`musl-dev` required by `go test -race` (CGO). Fix: add `gcc musl-dev` to the `apk add` line in the `domain-check-build` WorkflowTemplate, or switch to Debian-based `golang:1.26`.

### Record for downstream beads

- ✅ Raw quality-gate logs saved at: `docs/research/quality-gate-logs-2026-07-04.txt`
- ✅ Pod absence confirmed (podGC OnPodCompletion)
- ✅ Root cause documented at: `docs/notes/quality-gate-failure-analysis.md`
- ✅ Fix documented at: `docs/notes/quality-gate-failure-analysis.md` (Options A and B)
