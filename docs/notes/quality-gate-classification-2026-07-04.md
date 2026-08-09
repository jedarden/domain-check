# Quality-Gate Failure Classification — 2026-07-04

## Classification: Configuration issue (category 4)

The failure is a **configuration issue** — the CI quality-gate container uses `golang:1.26-alpine`, which ships with `CGO_ENABLED=0`. The test step passes `-race` to `go test`, but the race detector requires CGO.

## Verbatim error message

```
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
```

## File and line number

N/A — this is not a code defect. The error originates from the Go toolchain itself, not from any source file in the repository.

## Evidence

- `go vet ./...` passes successfully (no compilation errors, no vet issues)
- `go test -race ./...` fails before any test executes — the Go compiler refuses to compile the race-instrumented binary
- All dependencies download successfully (no missing modules)
- Git clone succeeds, `go version` reports `go1.26.4 linux/amd64`
- The container image `golang:1.26-alpine` is Alpine-based, which defaults to `CGO_ENABLED=0`

## Why this is NOT the other categories

1. **Compilation error** — No. `go vet` passes; the code compiles cleanly.
2. **Test failure** — No. Tests never ran. The `go test` invocation fails at link time, before any test binary executes.
3. **go vet issue** — No. `go vet ./...` completes with no output (clean pass).
4. **Configuration issue** — **Yes.** The WorkflowTemplate selects an Alpine image without CGO support, but the test command requires CGO via the `-race` flag.
5. **Infrastructure issue** — No. All tools are present (go, git), network works (deps download, git clone succeeds), no permission errors.

## Source of the misconfiguration

The quality-gate step in the `domain-check-build` WorkflowTemplate (in `jedarden/declarative-config`, path `k8s/iad-ci/argo-workflows/`) uses the Alpine image without enabling CGO. The fix is to either:
- Add `CGO_ENABLED=1` + `apk add build-base` to the quality-gate container, or
- Switch to `golang:1.26` (Debian-based, CGO works out of the box)
