# Workflow Entrypoint Test Plan

## Status: BLOCKED by Expired Credentials

**Date:** 2026-08-10  
**Blocker:** iad-ci cluster ServiceAccount token expired August 10, 2026  
**Impact:** Cannot submit manual workflows to verify entrypoint routing

The workflow template is correctly configured with two entrypoints, but actual testing requires refreshed cluster credentials. This document provides the complete test procedure to execute once credentials are renewed.

## Workflow Template Structure

**Template:** `domain-check-build` in `argoproj.io/v1alpha1`  
**Namespace:** `argo-workflows`  
**ServiceAccount:** `argo-workflow`  

### Entrypoint 1: `build` (Default)

**Steps:**
1. `build-quality-gate` — golangci-lint + go test -race + fuzz tests (15 min timeout)
2. `resolve-version` — Clone repo, bump VERSION file, commit & push (2 min timeout)
3. `docker-build` — Kaniko builds image → ronaldraygun/domain-check:{version}+latest (30 min timeout, 2 retries)

**When it runs:** Every push to main (no tag required)

**Expected result:** Success — Docker image pushed to Docker Hub

### Entrypoint 2: `release`

**Steps:**
1. `quality-gate` — go vet + go test -race on checked-out tag (10 min timeout)
2. `goreleaser-release` — Run goreleaser to build binaries + GitHub Release (30 min timeout)

**When it runs:** Tag push triggers workflow with `entrypoint: release` + `tag: v*`

**Expected result:** Success for real tags; failure for test tags (tag doesn't exist on remote)

## Test Procedure

### Test 1: Build Entrypoint (Default)

**Purpose:** Verify the default entrypoint runs quality-gate + resolve-version + docker-build ONLY

**Submission command:**
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
  # No entrypoint specified = uses default "build"
  # No tag parameter = empty string (builds from branch)
EOF
```

**Expected workflow graph:**
```
domain-check-build-test-xxxxx
├── build-quality-gate (container, golang:1.26)
├── resolve-version (script, alpine/git)
└── docker-build (container, gcr.io/kaniko-project/executor)
```

**Verification steps:**
1. Watch workflow start: `kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get workflows -n argo-workflows -l workflows.argoproj.io/workflow-template=domain-check-build -w`
2. Check pod names: `kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get pods -n argo-workflows -l workflows.argoproj.io/workflow=domain-check-build-test-xxxxx`
3. Verify NO `goreleaser-release` pod exists
4. Stream logs from each step:
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig logs -n argo-workflows <pod-name> -c main -f
   ```
5. Confirm final status: `kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get workflow domain-check-build-test-xxxxx -n argo-workflows -o jsonpath='{.status.phase}'`

**Expected result:** `Succeeded` with all 3 steps completed

**Record:** Run ID, start time, completion time, final status

---

### Test 2: Release Entrypoint with Test Tag

**Purpose:** Verify the release entrypoint runs quality-gate → goreleaser-release (proves entrypoint routing works)

**Submission command:**
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
  entrypoint: release  # Override default entrypoint
  arguments:
    parameters:
      - name: tag
        value: "v0.0.0-test"  # Test tag that doesn't exist on remote
EOF
```

**Expected workflow graph:**
```
domain-check-release-test-xxxxx
├── quality-gate (container, golang:1.26)
└── goreleaser-release (container, golang:1.26-alpine)
```

**Verification steps:**
1. Watch workflow start: `kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get workflows -n argo-workflows -l workflows.argoproj.io/workflow-template=domain-check-build -w`
2. Check pod names: `kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get pods -n argo-workflows -l workflows.argoproj.io/workflow=domain-check-release-test-xxxxx`
3. Verify NO `resolve-version` or `docker-build` pods exist
4. Verify `goreleaser-release` pod exists (this proves entrypoint routing)
5. Stream logs:
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig logs -n argo-workflows <pod-name> -c main -f
   ```
6. Watch for goreleaser failure (expected):
   - Error: `git clone --branch v0.0.0-test` fails
   - Or `git checkout v0.0.0-test` fails
   - This is CORRECT behavior — proves it reached the goreleaser step

**Expected result:** `Failed` at `goreleaser-release` step (tag doesn't exist), but `quality-gate` step should succeed

**Record:** Run ID, start time, failure point, error message (should be tag-not-found)

---

## Success Criteria

### Test 1 (Build Entrypoint)
- ✅ Workflow completes with status `Succeeded`
- ✅ Three pods ran: `build-quality-gate`, `resolve-version`, `docker-build`
- ✅ NO `goreleaser-release` pod executed
- ✅ Docker image `ronaldraygun/domain-check:<version>` pushed to Hub
- ✅ Docker image `ronaldraygun/domain-check:latest` pushed to Hub

### Test 2 (Release Entrypoint)
- ✅ `quality-gate` step completes successfully
- ✅ `goreleaser-release` step executes (proves entrypoint routing)
- ✅ NO `resolve-version` or `docker-build` pods executed
- ✅ Workflow fails at goreleaser with tag-not-found error (expected for test tag)
- ✅ Failure is NOT at entrypoint resolution (workflow starts correctly)

## What This Proves

Once both tests pass:

1. **Entrypoint routing works:** The `entrypoint: release` override correctly switches from build to release steps
2. **Build workflow is complete:** Quality gate + version bump + Docker build all execute in sequence
3. **Release workflow is complete:** Quality gate + goreleaser execute in sequence
4. **Tag-triggered execution is ready:** Real tag pushes will successfully create GitHub releases

## Next Steps After Credentials Refreshed

1. Renew iad-ci cluster ServiceAccount token
2. Execute Test 1 (build entrypoint) — should succeed
3. Execute Test 2 (release entrypoint) — should fail at goreleaser (expected)
4. Document run IDs and results in this file
5. Update `docs/notes/release-workflow-status-2026-08-10.md` with final test results
6. Close bead bf-4wvi with test summary

## Current Blocker Details

**Error when attempting kubectl operations:**
```
error: the server has asked the client to provide credentials
```

**Root cause:** iad-ci cluster ServiceAccount token expired (documented in git commit 4a3dd95)

**Resolution required:** Renew OIDC token from Rackspace Spot UI for `cloudspace-admin` group, update `/home/coding/.kube/iad-ci.kubeconfig`

**Reference:** `docs/notes/release-workflow-status-2026-08-10.md` for complete workflow status history
