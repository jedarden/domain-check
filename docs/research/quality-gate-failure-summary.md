# Quality-Gate Failure Summary

**Date:** 2026-07-04
**WorkflowTemplate:** `domain-check-build` (in `jedarden/declarative-config`)
**Step:** `build-quality-gate`

## Classification

| Field | Value |
|-------|-------|
| **Failed command** | `go test -race ./...` |
| **Passed command** | `go vet ./...` |
| **Exit code** | 2 (propagated by `set -ex` from the failing `go test` invocation) |
| **Error message** | `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1` |
| **Classification** | Toolchain misconfiguration — CGO unavailable on Alpine |
| **Severity** | CI pipeline failure (non-code) — all builds blocked |
| **Determinism** | Deterministic — fails on every run with `golang:1.26-alpine` |

## Environment

| Setting | Value |
|---------|-------|
| **Base image** | `golang:1.26-alpine` |
| **Go version** | go1.26.4 linux/amd64 |
| **C library** | musl (Alpine) |
| **CGO_ENABLED** | 0 (default on Alpine — no gcc/musl-dev installed) |

## Root Cause

The Go race detector (`-race` flag) requires CGO because it links against the thread sanitizer C runtime. The `golang:1.26-alpine` image uses musl libc and does not ship with `gcc` or `musl-dev`, so CGO cannot be enabled. The quality-gate step runs both `go vet ./...` (passes — no CGO needed) and `go test -race ./...` (fails — CGO unavailable).

## File/Line References

No source file or line references — this is a CI environment/toolchain issue, not a code defect. The failure originates from Go's toolchain itself (the `-race` flag's CGO dependency check) before any project code is compiled.

## Reproduction

Captured via debug workflows with `podGC: OnWorkflowCompletion` override:
- `domain-check-build-debug-mjwllw` (earliest capture)
- `domain-check-qg-capture-rlkxt` (canonical reproduction)
- `domain-check-build-debug-qg-x8bx6` (latest confirmation)

All three produced identical failure. Original failed runs' logs are irretrievable — `podGC: OnPodCompletion` deleted pods immediately on step completion.

## Recommended Fix

Switch the quality-gate container image from `golang:1.26-alpine` to `golang:1.26` (Debian-based). Debian includes glibc and gcc, enabling CGO for the race detector without additional packages. See [15-quality-gate-failure-analysis.md](15-quality-gate-failure-analysis.md) for full analysis and alternative fix options.
