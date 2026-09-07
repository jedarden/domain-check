# Workflow Entrypoint Test Results - 2026-08-10

## Objective

Test both the `build` and `release` entrypoints of the `domain-check-build` WorkflowTemplate to verify:
1. Build entrypoint runs: quality-gate → resolve-version → docker-build (no goreleaser)
2. Release entrypoint runs: quality-gate → goreleaser-release (reaches goreleaser step)
3. Entrypoint routing works correctly via `entrypoint:` parameter

## Test Date

2026-08-10 19:46 UTC

## Status: ❌ BLOCKED - Expired Credentials

Both workflow submission attempts failed with identical credential errors. The iad-ci cluster ServiceAccount token is expired/revoked, preventing all workflow submissions.

## Test Environment

- **Cluster:** iad-ci (Rackspace Spot, us-east-iad-1)
- **Namespace:** argo-workflows
- **Kubeconfig:** `/home/coding/.kube/iad-ci.kubeconfig`
- **WorkflowTemplate:** `domain-check-build`
- **Location:** `~/declarative-config/k8s/iad-ci/argo-workflows/domain-check-workflowtemplate.yml`

## Test Attempt 1: Build Entrypoint (Default)

### Submission Command
```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<'EOF'
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: domain-check-build-manual-
  namespace: argo-workflows
spec:
  workflowTemplateRef:
    name: domain-check-build
  arguments:
    parameters:
      - name: git-repo
        value: jedarden/domain-check
      - name: branch
        value: main
      - name: tag
        value: ""
EOF
```

### Result: ❌ FAILED - Credential Error
```
error: error validating "STDIN": error validating data: failed to download openapi: the server has asked the client to provide credentials; if you choose to ignore these errors, turn validation off with --validate=false
```

### Retry with --validate=false: ❌ SAME ERROR
```
E0810 19:46:36.995668 3004969 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: the server has asked the client to provide credentials"
error: unable to recognize "STDIN": the server has asked the client to provide credentials
```

### Expected Behavior (If Credentials Were Valid)
The `build` entrypoint would execute:

1. **build-quality-gate** (15 minutes)
   - Clone from `main` branch
   - Run golangci-lint, go test -race, fuzz tests
   - Should PASS (all local tests pass)

2. **resolve-version** (120s)
   - Bump patch version if needed
   - Output version parameter for docker-build

3. **docker-build** (1800s, 2 retries)
   - Build and push Docker image to ronaldraygun/domain-check

**Important:** The goreleaser-release step is NOT part of the build entrypoint.

## Test Attempt 2: Release Entrypoint

### Submission Command
```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create --validate=false -f - <<'EOF'
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

### Result: ❌ FAILED - Credential Error
```
E0810 19:46:41.898981 3026490 memcache.go:265] "Unhandled Error" err="couldn't get current server API group list: the server has asked the client to provide credentials"
error: unable to recognize "STDIN": the server has asked the client to provide credentials
```

### Expected Behavior (If Credentials Were Valid and Tag Existed)

The `release` entrypoint would execute:

1. **quality-gate** (600s)
   - Clone from tag `v0.0.0-test`
   - Run go vet and go test -race
   - Should PASS (all local tests pass)
   - Would FAIL if tag doesn't exist on remote

2. **goreleaser-release** (1800s)
   - Install goreleaser v2.5.0
   - Build 10 platform binaries
   - Create GitHub Release with binaries
   - Would FAIL gracefully if v0.0.0-test tag doesn't exist

**Important:** The docker-build step is NOT part of the release entrypoint.

## Workflow Template Structure Confirmed

### Entrypoint: `build` (default)
Steps: build-quality-gate → resolve-version → docker-build

### Entrypoint: `release`
Steps: quality-gate → goreleaser-release

### Key Differences
| Aspect | Build | Release |
|--------|-------|---------|
| Quality gate | build-quality-gate (full) | quality-gate (basic) |
| Version resolution | Yes | No (uses tag) |
| Docker build | Yes | No |
| GoReleaser | No | Yes |
| Source | Branch (main) | Tag (v0.0.0-test) |

## Acceptance Criteria Status

| Criteria | Status | Evidence |
|----------|--------|----------|
| Submit manual build workflow | ❌ Blocked | Credential error |
| Confirm build no goreleaser | ⏸️ Not tested | Cannot submit |
| Submit manual release workflow | ❌ Blocked | Credential error |
| Confirm release reaches goreleaser | ⏸️ Not tested | Cannot submit |
| Release fails gracefully on missing tag | ⏸️ Not tested | Cannot submit |
| Capture workflow run IDs | ❌ Blocked | No submissions |

## What Was Verified

1. ✅ WorkflowTemplate structure is correct
2. ✅ Both entrypoints exist and are properly configured
3. ✅ Build entrypoint excludes goreleaser-release
4. ✅ Release entrypoint includes goreleaser-release
5. ✅ Entrypoint routing parameter is defined

## What Could NOT Be Verified

1. ❌ Actual workflow execution (blocked by credentials)
2. ❌ Runtime entrypoint routing behavior
3. ❌ goreleaser-release step execution
4. ❌ Error handling for missing tags

## Next Steps

To complete this testing:

1. **Refresh iad-ci credentials** (primary blocker)
   - Regenerate ServiceAccount token for argocd-manager
   - Update kubeconfig
   - Verify access

2. **Push test tag**
   ```bash
   git tag v0.0.0-test && git push origin v0.0.0-test
   ```

3. **Resubmit both workflows** (commands documented above)

4. **Monitor and document results**

## Related Documentation

- `docs/notes/release-workflow-status-2026-08-10.md` - Full credential analysis
- `docs/notes/release-workflow-test-results.md` - July 2026 test results
- `~/declarative-config/k8s/iad-ci/argo-workflows/domain-check-workflowtemplate.yml` - Template

## Summary

The workflow entrypoint structure is correctly configured. Build runs quality-gate → resolve-version → docker-build (no goreleaser). Release runs quality-gate → goreleaser-release (no docker-build).

However, actual runtime behavior could not be verified due to expired iad-ci cluster credentials blocking all submissions since August 10, 2026.

The submission commands are correct and ready to execute once credentials are refreshed.
