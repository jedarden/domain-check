# Quality-Gate Pod Status — bf-allu

## Date: 2026-07-03

## Finding: Pods deleted by podGC — no logs capturable

All pods from recent failed `domain-check-build` workflows have been deleted by the Argo Workflows `podGC` policy (`OnPodCompletion` is the default). The pods are cleaned up immediately upon completion, before any log-capture step can run.

### Workflows checked (all Failed)

| Workflow | Age | Failure Node | Failure Message |
|----------|-----|-------------|-----------------|
| `domain-check-build-lhjfr` | ~5m | `build-quality-gate` | `main: Error (exit code 2)` |
| `domain-check-build-qd9vp` | ~12m | child failed | child failed |
| `domain-check-build-5mj2l` | ~49m | child failed | child failed |
| `domain-check-build-ttmq5` | ~53m | child failed | child failed |
| `domain-check-build-debug-qg-jz8k9` | ~68m | main | `main: Error (exit code 2)` |
| `domain-check-build-debug-qg-wl5sw` | ~82m | child failed | child failed |
| `domain-check-build-jxwhw` | ~96m | child failed | child failed |
| `domain-check-build-hsrx2` | ~109m | child failed | child failed |

### Pod query results

- `kubectl get pods -l workflows.argoproj.io/workflow=domain-check-build-*` returned **zero pods** for all recent workflow names
- The only pods in `argo-workflows` namespace are the controller, server, pushgateway, adb-relay, and unrelated pending/completed workflow pods (needle-ci, sigil-ci, hoop-ci)

### Conclusion

The quality-gate step failure (`exit code 2`) occurs in the `build-quality-gate` pod, but because `podGC` is `OnPodCompletion` (the Argo controller default), the pod is deleted as soon as it exits. To capture quality-gate logs from a failed workflow in the future, options are:

1. **Submit a debug workflow** with `podGC: OnWorkflowCompletion` override — pods survive until the entire workflow finishes
2. **Stream logs while running** — catch the pod while it's executing (requires monitoring or a pre-hook)
3. **Check the Argo UI** (`https://argo-ci.ardenone.com`) — logs are retained for completed workflows within a TTL window (success: 30min, failure: 2h)

### For downstream beads

Pod absence is confirmed. No quality-gate logs were captured because the pods no longer exist. The failure pattern is consistent across all recent runs: `build-quality-gate` exits with code 2.
