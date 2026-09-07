# podGC Analysis — domain-check-build on iad-ci

**Date:** 2026-07-03
**Cluster:** iad-ci (namespace: argo-workflows)
**Workflow:** domain-check-build-t98cn (most recent as of analysis)

## Finding: No pods exist — deleted by podGC OnPodCompletion

### Evidence

1. **Cluster-level podGC configuration:** The Argo Workflows controller configmap
   (`argo-workflows-iad-ci-workflow-controller-configmap`) sets:
   ```yaml
   podGC:
     strategy: OnPodCompletion
   ```
   This applies to **all** workflows on the cluster unless overridden at the
   workflow or template level.

2. **WorkflowTemplate:** `domain-check-build` does **not** set a `podGC` field in
   its own spec, so it inherits the cluster-level `OnPodCompletion` strategy.

3. **Workflow status:** `domain-check-build-t98cn` is in `Failed` phase, finished
   at `2026-07-03T11:15:27Z`. All 3 nodes (the workflow itself, the
   `build-quality-gate` step, and its container `[0]`) show `finishedAt` set.

4. **Pod query result:** `No resources found` — zero pods matching
   `workflows.argoproj.io/workflow=domain-check-build-t98cn`.

### Conclusion

The absence of pods is **expected and conclusive**. The cluster-level
`podGC.strategy: OnPodCompletion` policy deletes every pod the moment its
container completes (regardless of success or failure). Since workflow
`t98cn` finished at 11:15:27Z, its pods were deleted immediately
thereafter. This is consistent with the same result observed across
all recent domain-check-build workflows (`x2f2d`, `tqkkl`, `tv2nz`,
`hxcs5`, `ztx4z`, `vdxl5`).

### Implication

To capture logs from a failed workflow step, logs must be streamed **while
the pod is still running**, or the workflow must be submitted with a
`podGC: OnWorkflowCompletion` override (which preserves pods until the
entire workflow finishes). The Argo UI retains archived logs for a TTL
window (success: 30min, failure: 2h).
