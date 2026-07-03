# Quality-Gate Node Status — domain-check-build-94972

**Query Time:** 2026-07-03

## Most Recent Failed Workflow

| Field | Value |
|-------|-------|
| Workflow | `domain-check-build-94972` |
| Phase | Failed |
| Created | 2026-07-03T13:16:19Z |
| Overall Message | `child 'domain-check-build-94972-2895068185' failed` |
| Workflow in Cluster | No — garbage collected |

## Quality-Gate Node

| Field | Value |
|-------|-------|
| Node ID | `domain-check-build-94972-2895068185` |
| Display Name | `build-quality-gate` |
| Phase | **Failed** |
| Exit Code | **2** |
| Message | `main: Error (exit code 2)` |
| Started | 2026-07-03T13:16:20Z |
| Finished | 2026-07-03T13:22:01Z |
| Duration | ~5m 41s |
| Template | `build-quality-gate` (local/) |

## Pod Status

| Field | Value |
|-------|-------|
| Pods Exist | **No** |
| Reason | Deleted by `podGC: OnPodCompletion` — the controller deletes pods the moment they finish. |
| Logs Available | No — cannot be retrieved after podGC deletion. To capture logs from a failed step, either stream them while running or submit a debug workflow with `podGC: OnWorkflowCompletion` override. |

## Debug Workflow Confirmation

| Field | Value |
|-------|-------|
| Debug Workflow | `domain-check-build-debug-podgc2-5dxln` |
| Phase | Failed (reproduces the original failure) |
| Quality-Gate Pod | `domain-check-build-debug-podgc2-5dxln-build-quality-gate-1082504947` |
| podGC Strategy | `OnWorkflowSuccess` (pods retained on failure) |
| Exit Code | **2** |
| Duration | ~39s (21:55:34Z → 21:56:13Z) |

### Steps Completed Before Failure

1. `apk --no-cache add git ca-certificates` — OK (20.6 MiB in 29 packages)
2. `git clone --branch main` — OK
3. `go version` — OK (go1.26.4 linux/amd64)
4. `go vet ./...` — OK (no vet errors)
5. `go test -race ./...` — **FAILED** at 2026-07-03T21:56:13Z

### Key Log Excerpt

```
+ go test -race ./...
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
time=2026-07-03T21:56:13.054Z level=INFO msg="sub-process exited" argo=true error="exit status 2"
Error: exit status 2
```

## Root Cause Analysis

**Confirmed:** The `golang:1.26-alpine` base image disables CGO by default. The Go race detector (`-race` flag) requires `CGO_ENABLED=1`. The quality-gate step sets no `CGO_ENABLED` environment variable, so CGO is off (`0`), and `go test -race` fails immediately with exit code 2 before running any tests.

The fix is to add `CGO_ENABLED=1` to the quality-gate container's `env` list in the `domain-check-build` WorkflowTemplate in `declarative-config`. Alpine uses musl libc, so CGO is functional but the resulting binary links against musl rather than glibc.

## Audit Trail

- Original workflow `domain-check-build-94972` — no longer present in cluster (garbage collected), all pod logs unrecoverable
- Debug workflow `domain-check-build-debug-podgc2-5dxln` — full logs captured in `docs/quality-gate-logs.md`
- Prior debug workflows (`bf-3mbz`, `podgc-hqwdk`) — pods cleaned up before logs captured; led to the `OnWorkflowSuccess` strategy that succeeded on the third attempt
