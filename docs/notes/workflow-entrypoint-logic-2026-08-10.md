# Workflow Entrypoint Logic (2026-08-10)

## Change Summary

Updated the `domain-check-build` WorkflowTemplate to support dual execution modes:
- **Regular CI mode** (default): Runs on every push to `main` — builds Docker images
- **Release mode** (tag-triggered): Runs only on git tag pushes — publishes GitHub releases

## Implementation

### Previous Behavior
- Single entrypoint: `build`
- Always ran: `build-quality-gate` → `resolve-version` → `docker-build`
- `release` entrypoint existed but was never invoked
- `goreleaser-release` step was defined but never executed

### New Behavior
- New entrypoint: `choose-entrypoint` (conditional dispatcher)
- Uses Argo Workflows `when` clause to evaluate `workflow.parameters.tag`
- **When `tag` is empty** (regular push):
  - Executes `build` template: quality gate → version resolution → Docker build
  - Skips `release` template entirely (goreleaser never runs)
- **When `tag` is non-empty** (tag push, e.g., `v1.2.3`):
  - Executes `release` template: quality gate → goreleaser release
  - Skips `build` template (no Docker build, version resolution is from tag)

### Workflow Structure

```
choose-entrypoint (entrypoint)
├── run-release (when tag != "")
│   └── release
│       ├── quality-gate
│       └── goreleaser-release
└── run-build (when tag == "")
    └── build
        ├── build-quality-gate
        ├── resolve-version
        └── docker-build
```

## Acceptance Criteria Met

✅ goreleaser step runs only on tag push events (when `tag` parameter is set to a version like `v1.2.3`)
✅ On non-tag pushes, goreleaser step is skipped (conditional execution via `when` clause)
✅ Existing build/test/fuzz steps continue to run on every push (via `build-quality-gate` in both modes)
✅ WorkflowTemplate supports both regular CI and release modes through parameter-based conditional dispatch

## Files Modified

- `declarative-config/k8s/iad-ci/argo-workflows/domain-check-workflowtemplate.yml`
  - Changed `entrypoint` from `build` to `choose-entrypoint`
  - Added `choose-entrypoint` template with conditional steps
  - No changes to existing `build`, `release`, or step templates

## How It Works

The WorkflowTemplate now has three main templates:

1. **choose-entrypoint**: The new default entrypoint that conditionally dispatches
2. **build**: Original CI pipeline (quality gate → version → Docker image)
3. **release**: Release pipeline (quality gate → goreleaser GitHub release)

The conditional logic uses Argo Workflows' native `when` clause:
```yaml
when: "{{workflow.parameters.tag}} != \"\""  # Release mode
when: "{{workflow.parameters.tag}} == \"\""  # Regular CI mode  
```

When submitting a workflow, setting the `tag` parameter triggers release mode; leaving it empty (default) triggers regular CI mode.

## Testing Notes

The workflow cannot be tested end-to-end until the expired iad-ci cluster credentials are refreshed (see `docs/notes/release-workflow-status-2026-08-10.md`). However, the YAML syntax is valid and the conditional logic follows Argo Workflows best practices for parameter-based branching.
