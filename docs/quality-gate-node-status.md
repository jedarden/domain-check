# Quality-Gate Node Status — domain-check-build-94972

**Query Time:** 2026-07-03

## Most Recent Failed Workflow

| Field | Value |
|-------|-------|
| Workflow | `domain-check-build-94972` |
| Phase | Failed |
| Created | 2026-07-03T13:16:19Z |
| Overall Message | `child 'domain-check-build-94972-2895068185' failed` |
| Workflow in Cluster | No — garbage collected |

## Quality-Gate Node

| Field | Value |
|-------|-------|
| Node ID | `domain-check-build-94972-2895068185` |
| Display Name | `build-quality-gate` |
| Phase | **Failed** |
| Exit Code | **2** |
| Message | `main: Error (exit code 2)` |
| Started | 2026-07-03T13:16:20Z |
| Finished | 2026-07-03T13:22:01Z |
| Duration | ~5m 41s |
| Template | `build-quality-gate` (local/) |

## Pod Status

| Field | Value |
|-------|-------|
| Pods Exist | **No** |
| Reason | Deleted by `podGC: OnPodCompletion` — the controller deletes pods the moment they finish. |
| Logs Available | No — cannot be retrieved after podGC deletion. To capture logs from a failed step, either stream them while running or submit a debug workflow with `podGC: OnWorkflowCompletion` override. |

## Root Cause Analysis

Exit code 2 indicates the quality-gate step itself failed (not a pod-level crash). Per prior analysis (commit `0703367`), this is caused by `go test -race` failing on Alpine due to missing CGO support — Alpine musl libc does not support the race detector which requires CGO.

## Audit Trail

- Workflow no longer present in cluster (garbage collected)
- `kubectl get pods -l workflows.argoproj.io/workflow=domain-check-build-94972` → No resources found
- All pod logs unrecoverable — podGC deletes pods on completion
