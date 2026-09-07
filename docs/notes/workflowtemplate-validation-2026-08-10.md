# WorkflowTemplate Validation Report

**Date**: 2026-08-10  
**WorkflowTemplate**: domain-check-build  
**Location**: `k8s/iad-ci/argo-workflows/domain-check-workflowtemplate.yml` in declarative-config

## Summary

✅ **YAML Validation**: PASSED  
✅ **Git Status**: Clean, up to date with origin  
❌ **Cluster Verification**: BLOCKED (expired credentials)  
❌ **ArgoCD Sync Verification**: BLOCKED (expired credentials)

## YAML Structure Validation

### Entry Points

1. **build** (default entrypoint)
   - Used for: Regular Docker builds on main branch pushes
   - Steps: `build-quality-gate` → `resolve-version` → `docker-build`

2. **release** (newly added for goreleaser)
   - Used for: GitHub releases triggered by tags
   - Steps: `quality-gate` → `goreleaser-release`

### Template Definitions

| Template | Purpose | Timeout | Key Features |
|----------|---------|---------|--------------|
| `build-quality-gate` | CI quality checks | 900s | golangci-lint, tests, fuzz tests |
| `quality-gate` | Release quality checks | 600s | go vet, go test -race |
| `resolve-version` | Auto-bump VERSION file | 120s | Git-based versioning |
| `docker-build` | Build Docker image | 1800s | Kaniko, multi-tag (version + latest) |
| `goreleaser-release` | GitHub release | 1800s | GoReleaser v2.5.0, cross-compile |

### Goreleaser Release Template

```yaml
- name: goreleaser-release
  activeDeadlineSeconds: 1800
  container:
    image: golang:1.26-alpine
    args:
      - |
        # Installs GoReleaser v2.5.0
        # Clones repo at specified tag
        # Runs goreleaser release --clean
        # Uses .goreleaser.yml from repo root
```

**Configuration**:
- Uses `GH_TOKEN` from `secret: github-webhook-secret`
- Tag parameter passed via `workflow.parameters.tag`
- Reads `.goreleaser.yml` from repository root

## Git Status

```bash
$ cd /home/coding/jedarden/declarative-config
$ git status
On branch main
Your branch is up to date with 'origin/main'.
nothing to commit, working tree clean
```

**Latest commit**: `9f7a14d6 feat(dashboard): complete authentik cutover -- provider is live`

The WorkflowTemplate changes are already committed and pushed to GitHub.

## Cluster Verification

### Status: ❌ BLOCKED by expired iad-ci credentials

Attempting to verify the WorkflowTemplate on the cluster:

```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
  get workflowtemplate domain-check-build -n argo-workflows

error: You must be logged in to the server (the server has asked for the client to provide credentials)
```

**Issue**: The iad-ci cluster ServiceAccount token expired on 2026-08-10.

**Impact**:
- Cannot verify WorkflowTemplate is synced to cluster
- Cannot check workflow submission status
- Cannot verify ArgoCD Application health

## ArgoCD Sync Verification

### Status: ❌ BLOCKED

The ArgoCD Application `argo-workflows-ns-iad-ci` should automatically sync the WorkflowTemplate from declarative-config, but verification is blocked by:

1. Expired iad-ci kubeconfig credentials
2. ArgoCD API connectivity issues

## Expected Verification Steps (Post-Credential Refresh)

Once iad-ci credentials are refreshed, execute:

```bash
# 1. Verify WorkflowTemplate exists on cluster
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
  get workflowtemplate domain-check-build -n argo-workflows -o yaml

# Expected: Full WorkflowTemplate YAML with both entrypoints

# 2. Check ArgoCD Application sync status
kubectl --kubeconfig=/home/coding/.kube/ardenone-manager.kubeconfig \
  get application argo-workflows-ns-iad-ci -n argocd -o yaml

# Expected: status.sync.status: "Synced", health.status: "Healthy"

# 3. List all WorkflowTemplates in namespace
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
  get workflowtemplates -n argo-workflows

# Expected: domain-check-build listed
```

## Test Workflow Submission (Optional)

Once credentials are refreshed, test the release workflow:

```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: domain-check-build-test-
  namespace: argo-workflows
spec:
  workflowTemplateRef:
    name: domain-check-build
  entrypoint: release
  arguments:
    parameters:
      - name: tag
        value: v0.1.0-test
EOF
```

## Blocking Issues

1. **Expired iad-ci credentials**: ServiceAccount token needs refresh from Rackspace Spot UI
2. **ArgoCD API unreachable**: Network or credential issue preventing ArgoCD API verification

## Conclusion

The WorkflowTemplate YAML is valid, properly structured with both `build` and `release` entrypoints, and committed to declarative-config. The `release` entrypoint correctly implements GoReleaser v2.5.0 for GitHub releases.

**Next steps**:
1. Refresh iad-ci cluster credentials
2. Verify WorkflowTemplate appears on cluster
3. Confirm ArgoCD sync status
4. Test workflow submission if needed

The workflow should automatically sync once ArgoCD can connect to iad-ci with valid credentials.
