# Quality-Gate Log Capture Attempt

**Date:** 2026-07-03
**Workflow:** `domain-check-build-c5c8n` (most recent at time of check)

## Result: Pods deleted by podGC — no logs capturable

All pods from all recent `domain-check-build-*` workflows have been deleted by the time this check ran. The Argo Workflows controller's default `podGC` policy (`OnPodCompletion`) deletes pods the moment any step finishes — before logs can be streamed.

### Evidence

- **Pod list query:** `kubectl get pods -n argo-workflows -l workflows.argoproj.io/workflow=domain-check-build-c5c8n` → `No resources found`
- **Checked 3 most recent workflows** (`c5c8n`, `795vw`, `94972`) — all had zero surviving pods
- **All namespace pods listed** — none belong to any domain-check-build workflow

### Workflow details (domain-check-build-c5c8n)

| Field | Value |
|-------|-------|
| Phase | Failed |
| Failed step | `build-quality-gate` |
| Exit code | 2 |
| Started | 2026-07-03T13:40:39Z |
| Finished | 2026-07-03T13:41:29Z |
| Duration | ~50s |
| PodGC in spec | `null` (controller default applies) |
| Host node | `prod-instance-17819273493130218` |

### Root cause (from previous analysis, commit db35597)

Exit code 2 from `go test -race` is caused by CGO being unavailable on Alpine (`golang:1.24-alpine`). The `-race` detector requires CGO, and Alpine's `gcc`/`musl-dev` packages may not be installed in the builder image.

### How to capture logs next time

1. **Submit a debug workflow** with `podGC: OnWorkflowCompletion` override so pods survive until the workflow is deleted
2. **Stream logs while running** — pods only exist while the step executes
3. **Use Argo UI** — `https://argo-ci.ardenone.com` retains logs for 2h after failure

### Debug workflow template

```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: domain-check-build-debug-
  namespace: argo-workflows
spec:
  podGC: OnWorkflowCompletion
  workflowTemplateRef:
    name: domain-check-build
EOF
```
