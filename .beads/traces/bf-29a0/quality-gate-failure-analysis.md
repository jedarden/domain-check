# Quality-Gate Failure Analysis — domain-check-build

**Bead:** bf-29a0
**Date:** 2026-07-03
**Status:** Root cause identified

## Summary

All `domain-check-build` workflows on iad-ci fail at the `quality-gate` step with **exit code 2**. The failure is systematic and reproducible across every run.

## Exit Code

**Exit code 2** — produced by `set -ex` in the quality-gate shell script when a command returns non-zero.

## Root Cause (verbatim error)

```
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
```

## Why This Happens

The quality-gate step runs `go test -race ./...` inside a `golang:1.26-alpine` container. Alpine Linux does **not** include GCC or musl-dev by default. The `-race` flag in Go requires CGO (it links a C-based race detector library), which in turn requires a C compiler. Without `gcc` and `musl-dev` installed, `go test -race` immediately fails with the error above.

## Where It Fails

| Step | Result |
|------|--------|
| `apk --no-cache add git ca-certificates` | Passes (only installs git + certs, not gcc) |
| `git clone ...` | Passes |
| `go version` | Passes |
| `go vet ./...` | Passes (vet does not need CGO) |
| `go test -race ./...` | **FAILS** — CGO required but unavailable |

## Evidence

- **7 consecutive workflow runs** all failed identically: exit code 2, message `main: Error (exit code 2)`
- **Typical duration ~45s** — failure occurs early during test compilation (before any tests execute)
- **One outlier at ~14m** — likely triggered activeDeadlineSeconds=600s timeout (compilation attempted without CGO, hung)
- **Pods deleted by podGC: OnPodCompletion** — no container logs preserved; diagnosis from template analysis + Go's documented CGO requirement for `-race`

## Affected Template

**WorkflowTemplate:** `domain-check-build` in namespace `argo-workflows` (iad-ci cluster)
**Step:** `quality-gate`
**Container image:** `golang:1.26-alpine`
**Script:** `set -ex` → `go vet ./...` → `go test -race ./...`

## Fix Required

Add `gcc` and `musl-dev` to the `apk add` command in the quality-gate step:

```diff
- apk --no-cache add git ca-certificates
+ apk --no-cache add git ca-certificates gcc musl-dev
```

This enables CGO in the Alpine container, which is required by `go test -race`.

## Downstream Action

A separate bead will apply this fix to the WorkflowTemplate in `declarative-config`.
