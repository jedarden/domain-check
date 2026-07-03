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

---

## Second Capture Attempt (2026-07-03T17:27Z)

**Workflow:** `domain-check-build-qhjzp`
**Bead:** bf-5t1u (via child bf-5hwl)

### Result: Pod stuck pending — scheduling failure, no logs available

The quality-gate pod never started, so there are no logs to capture. The pod is stuck in `Pending` because the cluster's `ch.vs1.large-iad` node pool has insufficient resources.

### Evidence

| Field | Value |
|-------|-------|
| Workflow | `domain-check-build-qhjzp` |
| Pod | `domain-check-build-qhjzp-build-quality-gate-3792994707` |
| Status | `Pending` (0/2 containers ready) |
| Age | 14+ minutes and counting |
| Node selector | `servers.ngpc.rxt.io/class=ch.vs1.large-iad` |
| Resources requested | 4 CPU / 2Gi memory (requests: 1 CPU / 2Gi) |
| Cluster nodes (ch.vs1.large-iad) | 3 nodes, each 4 CPU / 7.6Gi memory |
| Scheduler message | `0/6 nodes are available: 2 Insufficient cpu, 3 Insufficient memory, 3 node(s) didn't match Pod's node affinity/selector` |

### Why no logs

The pod's `main` container never started — `kubectl logs` returns empty output. The scheduling failure means the quality-gate script never executed.

### Scheduling constraint analysis

The `build-quality-gate` template uses `golang:1.26-alpine` with a node selector requiring `ch.vs1.large-iad` class nodes. These nodes have only 4 CPUs each, and the quality-gate requests 4 CPU at limits. With 3 such nodes in the cluster, if even one is running another workload, scheduling fails.

### Conclusion

No new quality-gate logs are obtainable from this workflow run. The root cause of quality-gate failures is already known from the previous debug run (`domain-check-build-debug-tdrrf`): CGO/gcc missing on Alpine. The fix (adding `gcc musl-dev` to the `apk add` line in the WorkflowTemplate) is documented in `quality-gate-failure-analysis.md` and needs to be applied in `declarative-config`.
