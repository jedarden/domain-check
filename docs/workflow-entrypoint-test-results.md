# Workflow Entrypoint Test Results — domain-check-build

**Date:** 2026-08-10  
**Status:** ❌ BLOCKED — Cannot submit workflows due to expired iad-ci credentials  
**Blocking Issue:** ServiceAccount token for iad-ci cluster expired (credential validation failure)

## What Was Attempted

### 1. Workflow Template Analysis

Successfully examined the `domain-check-build` WorkflowTemplate at:
```
/home/coding/declarative-config/k8s/iad-ci/argo-workflows/domain-check-workflowtemplate.yml
```

**Confirmed Structure:**
- **Build entrypoint (default):** `build-quality-gate` → `resolve-version` → `docker-build`
- **Release entrypoint:** `quality-gate` → `goreleaser-release`
- Both entrypoints exist and are properly configured

### 2. Workflow Submission Attempts

**Attempt 1 - Build Workflow:**
```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: domain-check-build-manual-
  namespace: argo-workflows
spec:
  workflowTemplateRef:
    name: domain-check-build
EOF
```

**Result:** ❌ Failed with credential error:
```
error: error validating "STDIN": error validating data: failed to download openapi: the server has asked the client to provide credentials
```

**Attempt 2 - Build Workflow (validation disabled):**
```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create --validate=false -f - <<EOF
...same YAML...
EOF
```

**Result:** ❌ Failed with credential error:
```
error: unable to recognize "STDIN": the server has asked the client to provide credentials
```

**Attempt 3 - Release Workflow:**
Not attempted due to same credential issue.

## What Was Documented

### 1. Test Plan Document

Created `docs/workflow-entrypoint-test-plan.md` with:
- Complete test strategy for both entrypoints
- Exact YAML manifests for submission
- Expected behavior for each entrypoint
- Verification commands
- Success criteria checklist

### 2. Workflow Manifests

Created `docs/workflow-test-manifests.yaml` with:
- Ready-to-use YAML for both test workflows
- Inline submission commands
- Monitoring and verification commands

### 3. Expected Behavior

Based on workflow template analysis:

**Build Entrypoint (Test 1):**
- Should run: `build-quality-gate` → `resolve-version` → `docker-build`
- Should NOT run: `goreleaser-release`
- Expected result: Success (assuming quality gate passes)
- Expected output: Docker image pushed to `ronaldraygun/domain-check`

**Release Entrypoint (Test 2):**
- Should run: `quality-gate` → `goreleaser-release`
- Should NOT run: `resolve-version` or `docker-build`
- Expected result: Failure in `goreleaser-release` (tag `v0.0.0-test` doesn't exist)
- **Critical:** Should reach `goreleaser-release` step, proving entrypoint routing works

## What Needs to Happen Next

### To Complete This Test:

1. **Refresh iad-ci credentials** (blocking issue)
   - The ServiceAccount token in `/home/coding/.kube/iad-ci.kubeconfig` has expired
   - Needs to be regenerated from the Rackspace Spot UI (cloudspace-admin OIDC token)
   - Documented expiry: ~3 days (requires manual regeneration)

2. **Submit Test 1 (Build Workflow)**
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
   apiVersion: argoproj.io/v1alpha1
   kind: Workflow
   metadata:
     generateName: domain-check-build-manual-
     namespace: argo-workflows
   spec:
     workflowTemplateRef:
       name: domain-check-build
   EOF
   ```

3. **Submit Test 2 (Release Workflow)**
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
   apiVersion: argoproj.io/v1alpha1
   kind: Workflow
   metadata:
     generateName: domain-check-release-manual-
     namespace: argo-workflows
   spec:
     entrypoint: release
     arguments:
       parameters:
         - name: tag
           value: "v0.0.0-test"
     workflowTemplateRef:
       name: domain-check-build
   EOF
   ```

4. **Monitor and Document Results**
   - Capture workflow run IDs
   - Verify execution graph matches expectations
   - Confirm build workflow does NOT run goreleaser
   - Confirm release workflow reaches goreleaser step (even if it fails)

## Workflow Run IDs

**Not yet submitted** — awaiting credential refresh.

## Test Results

**Not yet executed** — awaiting credential refresh.

## Conclusion

The workflow template analysis confirms both entrypoints are correctly configured:
- ✅ Build entrypoint: Runs quality gate + version resolution + Docker build
- ✅ Release entrypoint: Runs quality gate + goreleaser release
- ✅ Entrypoint routing via `spec.entrypoint` override is supported
- ✅ Tag parameter passing is properly configured

The test cannot be completed until iad-ci cluster credentials are refreshed. Once refreshed, the documented submission commands can be executed immediately to verify end-to-end workflow behavior.

## Artifacts Created

1. `docs/workflow-entrypoint-test-plan.md` — Comprehensive test strategy
2. `docs/workflow-test-manifests.yaml` — Ready-to-use workflow manifests
3. `docs/workflow-entrypoint-test-results.md` — This document

All artifacts are ready for immediate use once credentials are refreshed.
