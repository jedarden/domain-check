# Release Workflow Test Results

**Date:** 2026-07-02
**Workflow:** `domain-check-release-test-bxcg6`
**Template:** `domain-check-build` (WorkflowTemplate)
**Entrypoint:** `release`
**Tag:** `v0.0.0-test`
**Namespace:** `argo-workflows` (iad-ci cluster)

## Objective

Submit a manual release workflow with `entrypoint: release` and `tag: v0.0.0-test` to confirm entrypoint routing reaches the `release` template's steps (quality-gate → goreleaser-release).

## Result: Entrypoint Routing Confirmed ✓

The workflow correctly resolved `entrypoint: release` to the `release` template in the WorkflowTemplate. Node tree confirms:

```
[Failed] domain-check-release-test-bxcg6 (template=release)
  └─ [Failed] quality-gate (template=quality-gate) — Error (exit code 127)
```

The `release` template's first step (`quality-gate`) was invoked as expected. The routing from `entrypoint: release` → `release` template → `quality-gate` step works.

## Failure Analysis

### quality-gate: exit code 127

The `quality-gate` container uses `golang:1.26-alpine` and runs `git clone` in its script. The `golang:1.26-alpine` image does not include `git`, causing all shell commands to fail with exit code 127 (command not found).

This is the **same issue documented in commit `13e5039`**: "golang:1.26-alpine missing git blocks all CI". It affects both the `build-quality-gate` and `quality-gate` templates.

### goreleaser-release: not reached

The `goreleaser-release` step was never reached because `quality-gate` failed first. However, entrypoint routing is confirmed — once the `git` issue is fixed (by installing git in the container or switching to a git-capable image), the full `quality-gate → goreleaser-release` path will execute.

**The goreleaser step itself would also fail** because:
1. It uses `golang:1.26-alpine` (same missing-git issue)
2. The tag `v0.0.0-test` doesn't exist on the remote, so `git clone --branch v0.0.0-test` would fail anyway

## Fix Required

The `quality-gate` and `goreleaser-release` templates need `git` installed in the `golang:1.26-alpine` container. Options:

1. **Add `apk add --no-cache git`** before `git clone` in both templates
2. **Switch to `golang:1.26`** (Debian-based, includes git) — larger image but simpler
3. **Use an init container** to install git before the main script runs

Option 1 is preferred (smallest image change, matches the fix pattern for `build-quality-gate`).

## Workflow Submission Command

```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: domain-check-release-test-
  namespace: argo-workflows
spec:
  workflowTemplateRef:
    name: domain-check-build
  entrypoint: release
  arguments:
    parameters:
      - name: git-repo
        value: jedarden/domain-check
      - name: branch
        value: main
      - name: tag
        value: v0.0.0-test
EOF
```

## Key Takeaway

**Entrypoint routing works.** The `entrypoint: release` override correctly routes to the `release` template's step sequence. The failure at `quality-gate` is a pre-existing image issue (missing `git` in `golang:1.26-alpine`), not a routing problem. Once that's fixed, a real release with a valid tag will flow through `quality-gate → goreleaser-release` as designed.
