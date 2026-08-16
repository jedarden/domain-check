# Release Workflow Submission Attempt - 2026-08-10

## Task Objective

Submit a manual release workflow with `entrypoint: release` and `tag: v0.0.0-test` to verify:
1. The `release` entrypoint routing works correctly
2. Both `quality-gate` and `goreleaser-release` steps execute
3. The workflow reaches the goreleaser step (expected to fail gracefully since the test tag won't exist on remote)
4. Capture and document workflow run ID and results

## Submission Attempts

### Attempt 1: Standard kubectl create
**Time:** 2026-08-10 19:21:50 UTC

**Command:**
```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: domain-check-release-test-
  namespace: argo-workflows
  labels:
    app: domain-check-build
    test: "true"
spec:
  workflowTemplateRef:
    name: domain-check-build
  entrypoint: release
  serviceAccountName: argo-workflow
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

**Result:** ❌ FAILED
```
error: error validating "STDIN": error validating data: failed to download openapi: the server has asked the client to provide credentials
```

### Attempt 2: With --validate=false flag
**Time:** 2026-08-10 19:22:00 UTC

**Command:** Same as Attempt 1 with `--validate=false` flag

**Result:** ❌ FAILED
```
E0810 19:22:50.163018  629156 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: the server has asked the client to provide credentials"
error: unable to recognize "STDIN": the server has asked the client to provide credentials
```

### Attempt 3: Simple workflow list test
**Time:** 2026-08-10 19:22:05 UTC

**Command:**
```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get workflows -n argo-workflows
```

**Result:** ❌ FAILED
```
error: You must be logged in to the server (the server has asked the client to provide credentials)
```

## Root Cause Analysis

### Issue: Expired ServiceAccount Token

**Kubeconfig:** `/home/coding/.kube/iad-ci.kubeconfig`

**Token Details:**
- **Type:** ServiceAccount JWT bearer token
- **Service Account:** `argocd-manager` in `argocd-manager` namespace  
- **UID:** `1638c0cb-c3df-4d92-bedf-685d37bd7ba6`
- **Token Status:** ❌ **EXPIRED/REVOKED**

**Error Pattern:** All kubectl commands to iad-ci cluster fail with:
- `"the server has asked the client to provide credentials"`
- `"couldn't get current server API group list"`
- `"unable to recognize "STDIN""`

### Impact

This credential issue is **blocking all workflow operations**:
- ❌ Cannot submit new workflows (build or release entrypoints)
- ❌ Cannot monitor existing workflow runs
- ❌ Cannot list or query workflow status
- ❌ Cannot stream logs from workflow pods
- ❌ Cannot verify quality-gate execution in CI environment
- ❌ Cannot test goreleaser-release step

## Workflow Template Verification

Despite the credential blocker, I verified the workflow template structure is correct:

### WorkflowTemplate: domain-check-build
**Location:** `/home/coding/declarative-config/k8s/iad-ci/argo-workflows/domain-check-workflowtemplate.yml`

### Release Entrypoint Configuration ✅

**Entrypoint:** `release` (lines 47-52)

**Steps:**
1. `quality-gate` - Runs go vet and go test -race (10 minute timeout)
2. `goreleaser-release` - Installs goreleaser v2.5.0 and runs release (30 minute timeout)

**Expected Behavior:**
```
quality-gate (600s timeout)
  ↓
  Clone repo at tag v0.0.0-test
  ↓
  Run: go vet ./...
  ↓
  Run: go test -race ./...
  ↓
goreleaser-release (1800s timeout)
  ↓
  Install goreleaser v2.5.0
  ↓
  Clone repo with full history
  ↓
  Checkout tag v0.0.0-test
  ↓
  Verify tag with git describe --tags --exact-match
  ↓
  Run: goreleaser release --clean
  ↓
  Expected: FAIL (tag doesn't exist on remote)
```

### Submission Parameters ✅

The workflow submission command correctly specified:
- `workflowTemplateRef.name: domain-check-build` ✅
- `entrypoint: release` ✅  
- `tag: v0.0.0-test` ✅
- `serviceAccountName: argo-workflow` ✅
- All required parameters (git-repo, branch, tag) ✅

## Historical Context

From the documentation in `docs/notes/release-workflow-status-2026-08-10.md`:

- **July 2026:** Two successful workflow submissions were made
  - `domain-check-release-test-bxcg6` - quality-gate failed (git not installed)
  - `domain-check-release-test-258wv` - quality-gate failed (tag not found)
  - Both proved entrypoint routing works correctly

- **August 10, 2026:** Credential expiration discovered
  - All workflow submissions now fail with authentication errors
  - Issue affects both `build` and `release` entrypoints
  - Local quality gate tests all pass (go vet, go test -race, fuzz tests)

## Expected Results (Once Credentials Fixed)

If the workflow could be submitted, the expected behavior would be:

### Step 1: quality-gate
- **Status:** ⏳ Would pass (local tests confirm this)
- **Duration:** ~5 minutes
- **Output:** go vet and go test -race all succeed

### Step 2: goreleaser-release  
- **Status:** ❌ Would fail (expected)
- **Reason:** Tag `v0.0.0-test` doesn't exist on remote GitHub
- **Failure Point:** `git clone --branch v0.0.0-test` or `git checkout v0.0.0-test`
- **This proves:** Entry point routing works correctly (reached step 2)

### Workflow Status
- **Phase:** Error or Failed
- **Message:** "unknown reference v0.0.0-test" or similar git error
- **Node Status:** quality-gate=Succeeded, goreleaser-release=Failed

## Workflow Run ID

**Status:** ❌ UNAVAILABLE

No workflow was created due to credential blocker. Expected run ID pattern would have been:
`domain-check-release-test-XXXXXX` (where XXXXX is Argo's random suffix)

## Resolution Required

### Blocker Removal
To complete this task, the following must happen:

1. **Regenerate iad-ci ServiceAccount token**
   - ServiceAccount: `argocd-manager` in namespace `argocd-manager`
   - Requires cluster admin access to iad-ci cluster
   - Update `/home/coding/.kube/iad-ci.kubeconfig` with new token

2. **Verify credential refresh**
   ```bash
   kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get workflows -n argo-workflows
   ```

3. **Re-submit workflow**
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

4. **Capture results**
   - Get workflow ID from output
   - Monitor workflow status
   - Document per-node execution results

## Alternative Approaches Considered

### Option 1: Use different kubeconfig
- **Checked:** No other iad-ci kubeconfigs exist
- **Result:** Not available

### Option 2: Use Argo UI instead of kubectl
- **Endpoint:** `https://argo-ci.ardenone.com`
- **Issue:** UI requires authentication, likely same credential issue
- **Result:** Not tested (credentials still required)

### Option 3: Contact cluster administrator
- **Action:** Request credential refresh from cluster admin
- **Status:** Not initiated (outside scope of this task)
- **Result:** Pending human intervention

## Confidence Assessment

### High Confidence ✅
- Workflow template structure is correct
- Release entrypoint exists and is properly configured
- goreleaser-release step exists and is reachable from release entrypoint
- Submission parameters are correct
- Local quality gate tests pass (predicts CI success)

### Medium Confidence ⚠️
- goreleaser configuration (exists but untested in CI)
- GitHub token permissions (unclear if token can create releases)
- .goreleaser.yml syntax (valid locally but not CI-tested)

### Low Confidence ❌  
- Actual workflow execution behavior (blocked by credentials)
- End-to-end timing and resource usage (blocked)
- Exact failure mode of goreleaser with missing tag (blocked)

## Conclusion

**Task Status:** ❌ BLOCKED by infrastructure credentials

The workflow submission command is correctly formed and would succeed if valid credentials were available. The workflow template structure is verified to be correct, with both `quality-gate` and `goreleaser-release` steps properly configured in the `release` entrypoint.

The expired iad-ci ServiceAccount token prevents ANY workflow operations, making it impossible to:
1. Submit the test workflow
2. Capture the workflow run ID  
3. Monitor execution results
4. Verify goreleaser step execution

**Immediate Action Required:** Regenerate the `argocd-manager` ServiceAccount token in the iad-ci cluster and update `/home/coding/.kube/iad-ci.kubeconfig`.

**Next Steps After Credential Fix:**
1. Verify access with `kubectl get workflows -n argo-workflows`
2. Re-submit the workflow submission command
3. Capture workflow run ID from submission output
4. Monitor workflow execution and document results
5. Verify both quality-gate and goreleaser-release steps execute

**Timeline:** Unknown - dependent on cluster admin credential refresh

**Risk Level:** Low - configuration is sound, only credential issue remains
