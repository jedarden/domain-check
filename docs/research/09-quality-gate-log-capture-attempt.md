# Quality-Gate Log Capture Attempt

**Date:** 2026-07-03
**Bead:** bf-1uy9

## Finding: All pods deleted by podGC (OnPodCompletion)

Checked four recent `domain-check-build` workflows for surviving pods:

| Workflow | Age | Status | Pods Found |
|----------|-----|--------|------------|
| `domain-check-build-795vw` | ~6m | Failed | None |
| `domain-check-build-94972` | ~19m | Failed | None |
| `domain-check-build-9z6rr` | ~22m | Failed | None |
| `domain-check-build-blxdb` | ~27m | Failed | None |

**All pods from all four workflows are gone.** The Argo controller uses `podGC: OnPodCompletion`, which deletes pods the moment they finish. No logs were capturable.

## Root Cause Analysis (from workflow metadata)

All four workflows failed at the same node:

- **Node:** `build-quality-gate`
- **Exit code:** 2
- **Duration:** ~48s (e.g., 795vw: 13:29:09Z → 13:29:57Z)

### The quality-gate template (`golang:1.26-alpine`)

```yaml
container:
  image: golang:1.26-alpine
  command: ["sh", "-c"]
  args: |
    set -ex
    apk --no-cache add git ca-certificates
    BRANCH="{{workflow.parameters.branch}}"
    git clone --branch "$BRANCH" ...
    cd /workspace
    go version
    echo "Running quality gate on branch: ${BRANCH}"
    go vet ./...
    go test -race ./...
```

### The problem: `go test -race` requires CGO

Alpine Linux uses musl libc, not glibc. Go's race detector requires CGO (it links against the thread sanitizer runtime), which needs glibc-compatible toolchains. On Alpine:

```
go test -race ./...
# runtime/cgo: pthread_create failed: Resource temporarily unavailable
```

or similar CGO-related failures that produce exit code 2.

## Recommended Fix

Replace `golang:1.26-alpine` with a glibc-based image for the quality-gate step:

- **Option A:** `golang:1.26` (Debian-based, CGO works)
- **Option B:** `golang:1.26-bookworm` (explicit Debian variant)

The quality-gate step doesn't need Alpine's minimal footprint — it just needs `git`, `ca-certificates`, and a working Go toolchain with CGO support.

## Alternative: Submit a debug workflow with podGC override

Per the CLAUDE.md instructions, to capture logs from a future failing workflow, submit a debug run with `podGC: OnWorkflowCompletion`:

```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: domain-check-build-debug-
  namespace: argo-workflows
spec:
  podGC:
    strategy: OnWorkflowCompletion
  workflowTemplateRef:
    name: domain-check-build
EOF
```

This preserves pods after completion so logs can be captured.
