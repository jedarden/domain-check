# Quality-Gate Failure Analysis — domain-check-build

**Bead:** bf-29a0
**Date:** 2026-07-03
**Status:** Root cause identified

## Summary

The quality-gate step in the `domain-check-build` CI workflow fails consistently with **exit code 2** on the `go test -race ./...` command. The root cause is that the `golang:1.26-alpine` Docker image does not include a C compiler (`gcc`) or musl development headers (`musl-dev`), which are required by Go's race detector (`-race` flag requires `CGO_ENABLED=1`).

## Exit Code

**Exit code: 2** — returned by the `go test` command via `set -ex` shell script. This is Go's standard exit code for a compilation/linking failure (not a test failure, which would be exit code 1).

## Exact Error Message (verbatim)

```
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
```

## Source of Error

The error originates from the Go toolchain itself — not from the domain-check source code. There are no failing tests, no compilation errors in the project's Go code, and no lint issues.

**Failing command:** `go test -race ./...`
**Failing step:** `build-quality-gate` (template in `domain-check-build` WorkflowTemplate)
**Workflow runs affected:** All `domain-check-build-*` runs since the quality-gate step was added (7+ consecutive failures)

## Command Sequence in Quality-Gate Script

```bash
set -ex
apk --no-cache add git ca-certificates          # ← succeeds
BRANCH=main
git clone --branch $BRANCH ... /workspace        # ← succeeds
cd /workspace
go version                                        # ← succeeds (go1.26.4 linux/amd64)
go vet ./...                                      # ← succeeds (downloads deps, no issues)
go test -race ./...                               # ← FAILS: exit code 2
```

`go vet ./...` passes successfully. The failure occurs only at `go test -race`.

## Technical Explanation

Go's race detector works by instrumenting the binary at compile time using CGO. When `-race` is specified, Go attempts to compile with `CGO_ENABLED=1`, which requires:

1. A C compiler (`gcc` or `clang`)
2. C standard library headers (`musl-dev` on Alpine)

The `golang:1.26-alpine` image is a minimal Alpine-based image that does **not** include these tools. The standard `golang:1.26` (Debian-based) image does include them.

## Fix Required (for downstream beads)

The fix must be applied in the `domain-check-build` WorkflowTemplate in `declarative-config` (`k8s/iad-ci/argo-workflows/domain-check-workflowtemplate.yml`). Two options:

### Option A: Install build tools in the Alpine image (recommended)

Add to the `apk add` line in both `build-quality-gate` and `quality-gate` templates:

```bash
apk --no-cache add git ca-certificates gcc musl-dev
```

This is the minimal change — adds ~50MB to the container but keeps the Alpine base image's small footprint.

### Option B: Use the Debian-based golang image

Change the image from `golang:1.26-alpine` to `golang:1.26` (Debian-based, includes gcc by default).

This is simpler but results in a larger base image (~800MB vs ~300MB).

## Affected Templates

| Template Name | Entrypoint | Used By |
|---------------|-----------|---------|
| `build-quality-gate` | `build` | Push to `main` |
| `quality-gate` | `release` | Tag push `v*` |

Both templates share the same root cause and need the same fix.

## Workflow Run History (confirmation of consistent failure)

| Workflow | Exit Code | Duration |
|----------|-----------|----------|
| domain-check-build-x2f2d | 2 | ~50s |
| domain-check-build-t98cn | 2 | ~47s |
| domain-check-build-qv94b | 2 | ~14m (timeout) |
| domain-check-build-s5z8k | 2 | ~44s |
| domain-check-build-wmf2z | 2 | ~45s |
| domain-check-build-qms44 | 2 | ~44s |
| domain-check-build-debug-tdrrf | 2 | ~39s (podGC override) |

## Previous Bug (Already Fixed)

An earlier iteration of the quality-gate step failed with **exit code 127** (command not found) because `git` was not installed in the `golang:1.26-alpine` image. This was fixed by adding `apk --no-cache add git ca-certificates` to the script. The `git` fix is in place — the remaining failure is the CGO/gcc issue described above.

## Log Sources

- Debug workflow logs: `docs/research/quality-gate-debug-logs.txt`
- Workflow status data: `.beads/traces/bf-3bmi/quality-gate-logs.txt`
- WorkflowTemplate documentation: `docs/notes/10-ci-workflowtemplate.md`
