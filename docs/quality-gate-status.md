# Quality-Gate Node Status — domain-check-build

**Query Time:** 2026-07-04

## Most Recent Failed Run

| Field | Value |
|-------|-------|
| **Workflow Name** | `domain-check-build-v5pfz` |
| **Namespace** | `argo-workflows` (iad-ci) |
| **Workflow Phase** | Failed |
| **Overall Message** | `child 'domain-check-build-v5pfz-3363020603' failed` |
| **Created** | `2026-07-04T05:32:49Z` |

## Quality-Gate Node Details

| Field | Value |
|-------|-------|
| **Node ID** | `domain-check-build-v5pfz-3363020603` |
| **Node Name** | `build-quality-gate` |
| **Template Name** | `build-quality-gate` |
| **Node Type** | Pod |
| **Phase** | **Failed** |
| **Exit Code** | `2` |
| **Message** | `main: Error (exit code 2)` |
| **Started** | `2026-07-04T05:32:50Z` |
| **Finished** | `2026-07-04T05:33:34Z` |
| **Duration** | ~44 seconds |
| **Host Node** | `prod-instance-17817844549640125` |
| **Resources** | CPU: 44 (core·s), Memory: 862 (MiB·s) |

## Node Tree

```
domain-check-build-v5pfz (Steps) — Failed
  └── [0] (StepGroup) — Failed
      └── build-quality-gate (Pod) — Failed
          Phase: Failed
          Message: main: Error (exit code 2)
          Exit Code: 2
          Started: 2026-07-04T05:32:50Z
          Finished: 2026-07-04T05:33:34Z
```

## Context

Exit code 2 indicates the quality-gate step itself failed (not a pod-level crash). Per prior analysis (commit `0703367`), this is caused by `go test -race` failing on Alpine due to missing CGO support — Alpine musl libc does not support the race detector which requires CGO.

## Prior Runs (for reference)

| Workflow | Phase | Quality-Gate Message | Date |
|----------|-------|---------------------|------|
| `domain-check-build-v5pfz` | Failed | `main: Error (exit code 2)` | 2026-07-04 |
| `domain-check-build-ttmq5` | Failed | `main: Error (exit code 2)` | 2026-07-03 |
| `domain-check-build-debug-qg-jz8k9` | Failed | `main: Error (exit code 2)` | 2026-07-03 |
| `domain-check-build-debug-qg-hkst6` | Error | `pods ... is forbidden: error looking up service account argo-workflows/github-auth: serviceaccount "github-auth" not found` | earlier |
| `domain-check-build-debug-qg-wl5sw` | Failed | child failed | earlier |
| `domain-check-build-jxwhw` | Failed | child failed | earlier |
| `domain-check-build-hsrx2` | Failed | child failed | earlier |
| `domain-check-build-debug-qg-h89zn` | Failed | child failed | earlier |
| `domain-check-build-45dk7` | Failed | child failed | earlier |
| `domain-check-build-debug-qg-78gw7` | Succeeded | — | earlier |

## Notes

- Pod GC cleaned up the pod before logs could be captured (podGC: OnPodCompletion).
- The failure is in the `build-quality-gate` step, which runs lint/test/build checks.
- Exit code 2 from the `main` container indicates a check failure (lint or test).
- The root cause is identified as `go test -race` requiring CGO on Alpine; fix is to switch to Debian base or add `CGO_ENABLED=1` + gcc/musl-dev.
- To get detailed logs, submit a debug workflow with `podGC: OnWorkflowCompletion` override, or check the Argo UI at https://argo-ci.ardenone.com within the 2-hour TTL window for failed workflows.
