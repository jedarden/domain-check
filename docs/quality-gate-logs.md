# Quality-Gate Container Logs

Captured from debug workflow `domain-check-build-debug-podgc2-5dxln` on 2026-07-03.
Pod: `domain-check-build-debug-podgc2-5dxln-build-quality-gate-1082504947`
Namespace: `argo-workflows` (iad-ci cluster)
podGC strategy: `OnWorkflowSuccess` (pods retained on failure for debugging)

## Full Container Output

```
time=2026-07-03T21:55:34.024Z level=INFO msg="waiting for signals" argo=true signalPath=/var/run/argo/ctr/main/signal
+ apk --no-cache add git ca-certificates
( 1/12) Installing brotli-libs (1.2.0-r1)
( 2/12) Installing c-ares (1.34.6-r0)
( 3/12) Installing libunistring (1.4.2-r0)
( 4/12) Installing libidn2 (2.3.8-r0)
( 5/12) Installing nghttp2-libs (1.69.0-r0)
( 6/12) Installing libpsl (0.21.5-r3)
( 7/12) Installing zstd-libs (1.5.7-r2)
( 8/12) Installing libcurl (8.21.0-r0)
( 9/12) Installing libexpat (2.8.2-r0)
(10/12) Installing pcre2 (10.47-r1)
(11/12) Installing git (2.54.0-r0)
(12/12) Installing git-init-template (2.54.0-r31)
Executing busybox-1.37.0-r31.trigger
OK: 20.6 MiB in 29 packages
+ BRANCH=main
+ git clone --branch main https://x-access-token:***@github.com/jedarden/domain-check.git /workspace
Cloning into '/workspace'...
+ cd /workspace
+ go version
go version go1.26.4 linux/amd64
+ echo 'Running quality gate on branch: main'
+ go vet ./...
Running quality gate on branch: main
go: downloading github.com/peterbourgon/ff/v4 v4.0.0-beta.1
go: downloading github.com/stretchr/testify v1.11.1
go: downloading golang.org/x/net v0.52.0
go: downloading github.com/prometheus/client_golang v1.23.2
go: downloading golang.org/x/time v0.15.0
go: downloading github.com/likexian/whois v1.15.7
go: downloading github.com/likexian/whois-parser v1.24.21
go: downloading golang.org/x/sync v0.20.0
go: downloading github.com/davecgh/go-spew v1.1.1
go: downloading github.com/pmezard/go-difflib v1.0.0
go: downloading gopkg.in/yaml.v2 v2.4.0
go: downloading github.com/beorn7/perks v1.0.1
go: downloading github.com/cespare/xxhash/v2 v2.3.0
go: downloading github.com/prometheus/client_model v0.6.2
go: downloading github.com/prometheus/common v0.66.1
go: downloading github.com/prometheus/procfs v0.16.1
go: downloading google.golang.org/protobuf v1.36.8
go: downloading gopkg.in/yaml.v3 v3.4.1
go: downloading github.com/likexian/gokit v0.25.16
go: downloading golang.org/x/text v0.35.0
go: downloading github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822
go: downloading go.yaml.in/yaml/v2 v2.4.2
go: downloading golang.org/x/sys v0.42.0
+ go test -race ./...
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
time=2026-07-03T21:56:13.054Z level=INFO msg="file signal handler exiting due to context cancellation" argo=true
time=2026-07-03T21:56:13.054Z level=INFO msg="sub-process exited" argo=true error="exit status 2"
Error: exit status 2
```

## Analysis

### Root Cause

The quality-gate step fails at `go test -race ./...` because the `golang:1.26-alpine` base image disables CGO by default. The Go race detector (`-race` flag) requires CGO to be enabled (`CGO_ENABLED=1`).

### Error Detail

```
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
```

### Steps That Succeeded

1. `apk --no-cache add git ca-certificates` — dependencies installed
2. `git clone --branch main` — source cloned successfully
3. `go version` — Go 1.26.4 confirmed
4. `go vet ./...` — passed (no vet errors)
5. `go test -race ./...` — **FAILED** (CGO not enabled)

### Fix

Add `CGO_ENABLED=1` to the quality-gate container's `env` list in the `domain-check-build` WorkflowTemplate in `declarative-config`. Note that Alpine uses musl libc, so CGO is possible but the resulting binary links against musl rather than glibc.

### Debugging Notes

- The original debug workflow (bf-3mbz) pods were cleaned up by podGC before logs could be captured.
- A second debug workflow (`domain-check-build-debug-podgc-hqwdk`) with `podGC: OnWorkflowCompletion` also had pods cleaned up on failure.
- A third debug workflow (`domain-check-build-debug-podgc2-5dxln`) with `podGC: OnWorkflowSuccess` successfully retained pods after failure, enabling this log capture.
- **Lesson:** Use `podGC: OnWorkflowSuccess` for debugging — pods persist on failure but clean up on success.
