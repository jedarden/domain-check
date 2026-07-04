# Quality-Gate Logs — 2026-07-04

Captured from pod `domain-check-qg-capture-v4nsl-build-quality-gate-3069676470`
Workflow: `domain-check-qg-capture-v4nsl` (Failed, exit code 2)
Node message: `main: Error (exit code 2)`
Image: `golang:1.26-alpine`

## Raw Output

```
+ apk --no-cache add git ca-certificates
time=2026-07-04T04:44:54.937Z level=INFO msg="waiting for signals" argo=true signalPath=/var/run/argo/ctr/main/signal
( 1/12) Installing brotli-libs (1.2.0-r1)
( 2/12) Installing c-ares (1.34.6-r0)
( 3/12) Installing libunistring (1.4.2-r0)
( 4/12) Installing libidn2 (2.3.8-r0)
( 5/12) Installing nghttp2-libs (1.69.0-r0)
( 6/12) Installing libpsl (0.21.5-r3)
( 7/12) Installing zstd-libs (1.5.7-r2)
( 8/12) Installing libcurl (8.21.0-r0)
( 9/12) Installing libexpat (2.8.2-r0)
(10/12) Installing pcre2 (10.47-r0)
(11/12) Installing git (2.54.0-r0)
(12/12) Installing git-init-template (2.54.0-r0)
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
go: downloading github.com/likexian/whois v1.15.7
go: downloading github.com/likexian/whois-parser v1.24.21
go: downloading golang.org/x/sync v0.20.0
go: downloading golang.org/x/time v0.15.0
go: downloading github.com/prometheus/client_golang v1.23.2
go: downloading github.com/davecgh/go-spew v1.1.1
go: downloading github.com/pmezard/go-difflib v1.0.0
go: downloading gopkg.in/yaml.v2 v2.4.0
go: downloading github.com/likexian/gokit v0.25.16
go: downloading github.com/beorn7/perks v1.0.1
go: downloading github.com/cespare/xxhash/v2 v2.3.0
go: downloading github.com/prometheus/client_model v0.6.2
go: downloading github.com/prometheus/common v0.66.1
go: downloading github.com/prometheus/procfs v0.16.1
go: downloading google.golang.org/protobuf v1.36.8
go: downloading gopkg.in/yaml.v3 v3.0.1
go: downloading golang.org/x/text v0.35.0
go: downloading github.com/munnerz/goautoneg v0.0.0-20191010083416-a7dc8b61c822
go: downloading go.yaml.in/yaml/v2 v2.4.2
go: downloading golang.org/x/sys v0.42.0
+ go test -race ./...
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
time=2026-07-04T04:45:31.971Z level=INFO msg="sub-process exited" argo=true error="exit status 2"
time=2026-07-04T04:45:31.971Z level=INFO msg="file signal handler exiting due to context cancellation" argo=true
Error: exit status 2
```

## Root Cause

The `golang:1.26-alpine` image has CGO disabled by default. The `-race` flag in `go test -race` requires CGO (`CGO_ENABLED=1`) because the race detector is implemented in C.

`go vet ./...` passed successfully — the failure is solely from `go test -race`.

## Fix Required

The WorkflowTemplate `domain-check-build` in `declarative-config` needs one of:
1. Add `CGO_ENABLED=1` env var to the quality-gate container (requires `apk add build-base` for gcc/musl-dev)
2. Switch from `golang:1.26-alpine` to `golang:1.26` (Debian-based, CGO works out of the box)
3. Drop `-race` from the quality-gate step (not recommended per plan.md CI guarantees)
