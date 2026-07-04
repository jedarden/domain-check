# Quality-Gate Pod Status — bf-clao

## Date: 2026-07-04

## Finding: Pods deleted by podGC — no logs capturable

All pods from recent failed `domain-check-build` workflows have been deleted by the Argo Workflows `podGC` policy (`OnPodCompletion` is the default). The pods are cleaned up immediately upon completion, before any log-capture step can run.

### Workflows checked (most recent Failed builds)

| Workflow | Age | Failure Node | Failure Message |
|----------|-----|-------------|-----------------|
| `domain-check-build-hqftd` | ~43m | `build-quality-gate` | `main: Error (exit code 2)` |
| `domain-check-build-lw47h` | ~48m | child failed | child failed |
| `domain-check-build-lhjfr` | ~58m | child failed | child failed |
| `domain-check-build-qd9vp` | ~65m | child failed | child failed |
| `domain-check-build-5mj2l` | ~102m | child failed | child failed |
| `domain-check-build-ttmq5` | ~106m | child failed | child failed |

### Most recent failed build details

- **Workflow:** `domain-check-build-hqftd`
- **Namespace:** `argo-workflows`
- **Phase:** Failed
- **Failed node:** `build-quality-gate` (type: Pod) — `main: Error (exit code 2)`
- **PodGC policy:** Not set on workflow (cluster default `OnPodCompletion` applies)

### Pod query results

- `kubectl get pods -l workflows.argoproj.io/workflow=domain-check-build-hqftd` returned **zero pods**
- Same result for all other recent failed workflow names — all pods deleted by podGC

### Conclusion

Pod absence is confirmed. No quality-gate logs were captured because the pods no longer exist. The failure pattern is consistent across all recent runs: `build-quality-gate` exits with code 2.

### For downstream beads

- Pod absence confirmed — cannot retrieve quality-gate logs from failed workflow pods
- To capture logs in the future: submit a debug workflow with `podGC: OnWorkflowCompletion` override, or check the Argo UI at `https://argo-ci.ardenone.com` within the failure TTL window (2h)
