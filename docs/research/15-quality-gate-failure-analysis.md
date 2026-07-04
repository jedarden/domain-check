# Quality-Gate Failure Analysis

**Date:** 2026-07-03
**Workflow:** `domain-check-build` (8 consecutive failures: `4ztt8` through `ls9lk`)
**WorkflowTemplate:** `domain-check-build` in `jedarden/declarative-config`

## Raw Logs

Full quality-gate pod output captured via reproduction workflow with `podGC: OnWorkflowCompletion` override. See [quality-gate-logs.txt](quality-gate-logs.txt) for the definitive capture (includes raw output, retrieval attempts, and exit code analysis). Earlier captures in [quality-gate-debug-logs.txt](quality-gate-debug-logs.txt).

## Failure Classification

| Category | Value |
|----------|-------|
| **Failure type** | Toolchain/environment misconfiguration — CGO unavailable |
| **Severity** | CI pipeline failure (non-code) — all builds blocked |
| **Determinism** | Deterministic — fails on every run on `golang:1.26-alpine` |
| **Impact scope** | Entire `domain-check-build` WorkflowTemplate |
| **Affected step** | `build-quality-gate` |
| **Exit code** | 2 (from `set -ex` propagating the `go test` failure) |

## Failure Summary

| Field | Value |
|-------|-------|
| **Failed node** | `build-quality-gate` |
| **Step that failed** | `go test -race ./...` |
| **Step that passed** | `go vet ./...` |
| **Error message** | `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1` |
| **Go version** | go1.26.4 linux/amd64 |
| **Base image** | `golang:1.26-alpine` |

## Root Cause

**CGO is unavailable on Alpine Linux.** The quality-gate container uses `golang:1.26-alpine` (musl libc). Go's race detector (`-race` flag) requires CGO because it links against the thread sanitizer runtime. Alpine lacks `gcc` and `musl-dev`, so CGO cannot be enabled.

The quality-gate template runs:
```sh
go vet ./...       # ✓ succeeds (no CGO needed)
go test -race ./...  # ✗ fails — CGO not available on Alpine
```

`go vet` passes cleanly. Only `go test -race` fails because of the CGO dependency.

## Fix Options

### Option A: Switch quality-gate image to Debian-based (recommended)
Replace `golang:1.26-alpine` with `golang:1.26` or `golang:1.26-bookworm` in the quality-gate container spec. Debian images include glibc and gcc by default, so CGO works out of the box.

### Option B: Install gcc on Alpine
Add `gcc musl-dev` to the `apk add` line:
```sh
apk --no-cache add git ca-certificates gcc musl-dev
```
This keeps the smaller Alpine image but adds the toolchain needed for CGO.

### Option C: Drop `-race` from quality gate
Use `go test ./...` without `-race`. This loses race detection in CI, which is a significant trade-off for a concurrent Go application.

**Recommendation:** Option A is cleanest — the quality-gate step doesn't need Alpine's minimal footprint.

## Log Capture Notes

The initial attempt to capture logs failed because all pods were deleted by Argo's `podGC: OnPodCompletion` policy. Pods are removed the instant they complete, making retroactive log capture impossible. Two independent debug workflows with `podGC: OnWorkflowCompletion` override were submitted:
- First: `domain-check-build-debug-mjwllw` (captured in `quality-gate-debug-logs.txt`)
- Second: `domain-check-build-debug-qg-s8qhl` (confirmed identical root cause)

Both produced identical failure: `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1`.

Third confirmation captured from workflow `domain-check-build-debug-podgc2-5dxln` (pod `domain-check-build-debug-podgc2-5dxln-build-quality-gate-1082504947`), which was still running when checked. The pod was in `Error` state with the same `go test -race ./...` failure. Go version `go1.26.4 linux/amd64` on `golang:1.26-alpine`. `go vet ./...` passed; only `go test -race` failed.

Note: `OnWorkflowCompletion` podGC still deletes pods once the entire workflow finishes (not just the individual step), so logs must be captured while the workflow is still in a terminal state but before the podGC controller runs. In this case the pod survived because the workflow was still in a terminal `Failed` state and podGC had not yet cleaned up.
