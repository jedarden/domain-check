# Quality-Gate Log Capture Attempt

**Date:** 2026-07-03
**Workflow:** `domain-check-build-ls9lk` (most recent of 8 failed runs)

## Finding

All pods from all `domain-check-build` workflows have been deleted by the Argo Workflows `podGC: OnPodCompletion` policy. No quality-gate pods remain in the `argo-workflows` namespace — logs cannot be captured retroactively.

## Evidence

Scanned all 8 recent workflows (from `ls9lk` to `4ztt8`):

```
domain-check-build-4ztt8    Failed  52m
domain-check-build-debug-tdrrf  Failed  49m
domain-check-build-blxdb    Failed  47m
domain-check-build-9z6rr    Failed  42m
domain-check-build-94972    Failed  39m
domain-check-build-795vw    Failed  26m
domain-check-build-c5c8n    Failed  15m
domain-check-build-ls9lk    Failed  4m25s
```

For `ls9lk` (latest), the failure node is:

```
build-quality-gate - Failed
  msg: main: Error (exit code 2)
  type: Pod
```

`kubectl get pods -l workflows.argoproj.io/workflow=<name>` returns empty for every workflow.

## Root Cause

The exit code 2 from `go test -race` on Alpine is due to CGO being unavailable on `alpine:3.19` (no `gcc`/`musl-dev`). The `-race` flag requires CGO, causing the quality-gate step to fail immediately.

## For Next Attempt

To capture logs from a failing step, either:
1. Stream logs **while the pod is running** (catch it before completion triggers podGC)
2. Submit a debug workflow with `podGC: OnWorkflowCompletion` override
3. Use the Argo UI at `https://argo-ci.ardenone.com` (success: 30min, failure: 2h TTL)
