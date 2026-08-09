# Quality-Gate Root Cause Analysis

**Date:** 2026-07-03
**Source:** Debug workflow `domain-check-build-debug-podgc2-5dxln` logs captured by bf-5t1u
**Original failing workflow:** `domain-check-build-94972`

## Summary

The quality-gate step fails because `go test -race` requires CGO, but the `golang:1.26-alpine` base image disables CGO by default. No Go tests actually run — the toolchain rejects the command before compilation begins.

## Exit Code

**2** — returned by `go test -race ./...` when CGO is disabled. This is Go's own exit code for a command-line usage error, not a test failure (which would be exit code 1).

## Failing Command

```
go test -race ./...
```

**Error message:**
```
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
```

## Passing Command

```
go vet ./...
```

Passed successfully with zero errors. All dependencies were downloaded and vet completed cleanly.

## Step-by-Step Execution Sequence

| Step | Command | Result |
|------|---------|--------|
| 1 | `apk --no-cache add git ca-certificates` | ✅ OK — 29 packages, 20.6 MiB |
| 2 | `git clone --branch main` | ✅ OK — source cloned |
| 3 | `go version` | ✅ OK — go1.26.4 linux/amd64 |
| 4 | `go vet ./...` | ✅ OK — no vet errors |
| 5 | `go test -race ./...` | ❌ **FAILED** — CGO not enabled |

## Test Failures

**There are no test failures.** No tests were executed. The failure occurs at the toolchain level before any test code is compiled or run. The `-race` flag is rejected by the `go` command itself because `CGO_ENABLED=0` is the default on Alpine images.

## Root Cause

On Alpine-based images (`golang:*-alpine`), the `CGO_ENABLED` environment variable defaults to `0` (disabled). The Go race detector (`-race` build flag) requires CGO because it instruments memory accesses using C-level atomics. Without CGO, the race detector cannot link, so `go test -race` exits immediately with:

```
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
```

Alpine uses **musl libc** instead of glibc, which is why CGO is disabled by default — musl-based toolchains require explicit CGO enablement.

## Fix

Add `CGO_ENABLED=1` to the quality-gate container's `env` list in the `domain-check-build` WorkflowTemplate in `declarative-config`. This must be paired with installing a C compiler and musl-dev on the Alpine image, or alternatively switching the base image to Debian-based (`golang:1.26-bookworm`) where CGO is enabled by default.

**Option A (minimal — stay on Alpine):**
```yaml
env:
  - name: CGO_ENABLED
    value: "1"
# Also add to the apk install step:
#   apk add gcc musl-dev
```

**Option B (switch to Debian):**
```yaml
image: golang:1.26-bookworm  # instead of golang:1.26-alpine
# CGO_ENABLED defaults to 1 on Debian, gcc already available
```

## Audit Trail

- Original workflow `domain-check-build-94972` — pods garbage-collected, logs unrecoverable
- Debug workflow `domain-check-build-debug-podgc2-5dxln` — logs captured with `podGC: OnWorkflowSuccess`
- Prior debug workflows (bf-3mbz, podgc-hqwdk) — pods cleaned up before logs captured
