# Quality Gate Log — 2026-07-04

## Debug Workflow: `domain-check-build-debug-t68mf`

- **Phase:** Failed
- **Step:** `build-quality-gate`
- **Pods:** Already deleted (podGC override expired before log capture)

## Captured From: `domain-check-qg-capture-v4nsl`

Pod `domain-check-qg-capture-v4nsl-build-quality-gate-3069676470` survived with `0/2 Error` state. Logs retrieved from `main` container.

- **Phase:** Failed
- **Step:** `build-quality-gate`
- **Go version:** go1.26.4 linux/amd64
- **Image:** golang:1.26-alpine
- **Branch:** main
- **Argo Executor:** v4.0.3 (go1.25.7)

## Full Log Output

### Setup
- Alpine packages installed (git, ca-certificates, etc.)
- Repository cloned: `jedarden/domain-check` (branch `main`)

### `go vet ./...`
**Status: PASSED** (no output = no issues)

Dependencies downloaded successfully:
- github.com/peterbourgon/ff/v4, stretchr/testify, golang.org/x/net, likexian/whois, likexian/whois-parser, golang.org/x/sync, golang.org/x/time, prometheus/client_golang, and transitive deps

### `go test -race ./...`
**Status: FAILED**

```
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
```

## Root Cause

The CI image is `golang:1.26-alpine`. Alpine uses musl libc, not glibc. The `go test -race` flag requires CGO (the race detector is implemented in C). On Alpine, CGO is disabled by default because the base system lacks the C toolchain (gcc, musl-dev).

## Fix Required

The `build-quality-gate` step script needs to either:

1. **Install the C toolchain before running tests:**
   ```sh
   apk add --no-cache gcc musl-dev
   ```
2. **Or set `CGO_ENABLED=1` explicitly** (but this alone won't work without gcc)

The simplest fix is adding `gcc musl-dev` to the `apk add` line in the quality-gate template, alongside `git ca-certificates`.

## Template Location

The `build-quality-gate` template is in the `domain-check-build` WorkflowTemplate at:
```
jedarden/declarative-config → k8s/iad-ci/argo-workflows/domain-check-build.yaml
```
