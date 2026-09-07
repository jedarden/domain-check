# Release Workflow Entrypoint Test Plan

**Status:** ⏳ BLOCKED - Expired iad-ci credentials (as of 2026-08-10)

This document captures the test plan for verifying both workflow entrypoints work correctly. Once credentials are refreshed, these manual workflow submissions will confirm the tag-triggered execution logic.

## Workflow Structure

The `domain-check-build` WorkflowTemplate has two entrypoints:

### 1. `build` (default entrypoint)
**Steps:**
- `build-quality-gate` → golangci-lint + go test -race + fuzz tests (15 min activeDeadlineSeconds)
- `resolve-version` → clone, check VERSION file, auto-bump patch if needed, commit + push (2 min activeDeadlineSeconds)
- `docker-build` → Kaniko builds `ronaldraygun/domain-check:{version}` + `:latest` (30 min activeDeadlineSeconds, 2 retries)

### 2. `release` entrypoint
**Steps:**
- `quality-gate` → go vet + go test -race on tag checkout (10 min activeDeadlineSeconds)
- `goreleaser-release` → Goreleaser builds release assets + GitHub Release (30 min activeDeadlineSeconds)

## Test Plan

### Test 1: Default Build Entrypoint (No Special Args)

**Purpose:** Verify the default entrypoint runs build-quality-gate + resolve-version + docker-build only (no goreleaser step).

**Submission Command:**
```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: domain-check-build-test1-
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

**Expected Results:**
- ✅ Workflow starts successfully
- ✅ Step sequence:
  1. `build-quality-gate` completes (golangci-lint, go test -race, fuzz tests all pass)
  2. `resolve-version` completes (VERSION file checked/incremented, committed, pushed)
  3. `docker-build` completes (Docker image pushed to `ronaldraygun/domain-check:{version}` + `:latest`)
- ✅ **No goreleaser step runs** (release entrypoint not invoked)
- ✅ Final workflow status: `Succeeded`
- ✅ Docker image available on Docker Hub

**Verification Commands:**
```bash
# Watch workflow progress
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get workflows -n argo-workflows -l workflows.argoproj.io/workflow-template=domain-check-build --watch

# Get workflow status
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get workflow <workflow-name> -n argo-workflows -o jsonpath='{.status.phase}'

# Get per-node details
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get workflow <workflow-name> -n argo-workflows -o json | python3 -c "
import json,sys
w = json.load(sys.stdin)
for node in w['status'].get('nodes',{}).values():
    if node.get('phase') in ('Failed','Error','Succeeded'):
        print(node['displayName'], '-', node['phase'])
        if 'message' in node:
            print('  msg:', node['message'])
"

# Check Docker Hub for the new image (requires docker login)
docker pull ronaldraygun/domain-check:latest
docker image inspect ronaldraygun/domain-check:latest | jq '.[0].Created'
```

**Expected Failure Modes (graceful):**
- If golangci-lint fails → workflow fails at build-quality-gate step
- If tests fail → workflow fails at build-quality-gate step
- If git push fails → workflow fails at resolve-version step
- If docker build fails → workflow retries up to 2 times (30s → 60s exponential backoff)

---

### Test 2: Release Entrypoint with Test Tag

**Purpose:** Verify the release entrypoint runs quality-gate + goreleaser-release. The goreleaser step is expected to fail gracefully because `v0.0.0-test` doesn't exist on the remote — this confirms the entrypoint routing works.

**Submission Command:**
```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: domain-check-release-test2-
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
        value: "v0.0.0-test"
EOF
```

**Expected Results:**
- ✅ Workflow starts successfully
- ✅ Step sequence:
  1. `quality-gate` completes (go vet + go test -race pass on tag checkout)
  2. `goreleaser-release` runs (installs goreleaser, clones repo, checks out tag)
- ⚠️ `goreleaser-release` **fails gracefully** with error indicating tag `v0.0.0-test` doesn't exist on remote
- ✅ The failure proves the release entrypoint was invoked (not the build entrypoint)
- ✅ Final workflow status: `Failed` (expected, due to non-existent tag)
- ✅ Build entrypoint steps (resolve-version, docker-build) do NOT run

**Expected goreleaser failure output (similar to):**
```
+ goreleaser release --clean
  • releasing...
  • loading config file                               .goreleaser.yml
  • loading environment variables
  ERROR: context deadline exceeded
  or
  ERROR: git clone --branch v0.0.0-test failed (remote tag not found)
```

**Verification Commands:**
```bash
# Watch workflow progress
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get workflows -n argo-workflows -l workflows.argoproj.io/workflow-template=domain-check-build --watch

# Get workflow status (should be Failed)
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get workflow <workflow-name> -n argo-workflows -o jsonpath='{.status.phase}'

# Get per-node failure details
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get workflow <workflow-name> -n argo-workflows -o json | python3 -c "
import json,sys
w = json.load(sys.stdin)
for node in w['status'].get('nodes',{}).values():
    if node.get('phase') in ('Failed','Error'):
        print(node['displayName'], '-', node['phase'])
        print('  msg:', node.get('message','no message'))
"

# Check that build steps did NOT run
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get workflow <workflow-name> -n argo-workflows -o json | python3 -c "
import json,sys
w = json.load(sys.stdin)
steps = [node['displayName'] for node in w['status'].get('nodes',{}).values() if node['type'] == 'Steps']
print('Steps executed:', steps)
# Should NOT include: resolve-version, docker-build
# SHOULD include: quality-gate, goreleaser-release
"
```

**What Success Looks Like (for this test):**
- The `quality-gate` step completes successfully → proves the release entrypoint's quality gate works
- The `goreleaser-release` step runs (even though it fails) → proves the release entrypoint routing works
- No `resolve-version` or `docker-build` steps appear in the node tree → proves we didn't accidentally run the build entrypoint

---

## Test 3: Successful Release (Future Test with Real Tag)

**Purpose:** After cutting a real release tag (e.g., `v0.1.0`), verify the full release workflow succeeds end-to-end.

**Prerequisites:**
- Create and push a real tag: `git tag v0.1.0 && git push origin v0.1.0`
- Ensure `.goreleaser.yml` exists in the repo root

**Submission Command:**
```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: domain-check-release-v0.1.0-
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
        value: "v0.1.0"
EOF
```

**Expected Results:**
- ✅ Workflow starts successfully
- ✅ Step sequence:
  1. `quality-gate` completes (go vet + go test -race on tag checkout)
  2. `goreleaser-release` completes (builds binaries, creates GitHub Release, uploads assets)
- ✅ Final workflow status: `Succeeded`
- ✅ GitHub Release created at `https://github.com/jedarden/domain-check/releases/tag/v0.1.0`
- ✅ Release assets uploaded: Linux, macOS, Windows binaries, checksums file

**Verification Commands:**
```bash
# Check GitHub Release
gh release view v0.1.0 --repo jedarden/domain-check

# Download and verify an asset
wget https://github.com/jedarden/domain-check/releases/download/v0.1.0/domain-check-linux-amd64
chmod +x domain-check-linux-amd64
./domain-check-linux-amd64 --version

# Verify checksums
wget https://github.com/jedarden/domain-check/releases/download/v0.1.0/checksums.txt
sha256sum -c checksums.txt --ignore-missing
```

---

## Current Blocker (2026-08-10)

**Issue:** Expired iad-ci cluster credentials

The workflow submissions are blocked because the ServiceAccount token for the iad-ci cluster has expired. This affects all Argo Workflows submissions.

**Resolution Required:**
Refresh the iad-ci cluster credentials at `/home/coding/.kube/iad-ci.kubeconfig`.

**Evidence of Local Quality Gate Health:**

Despite the credential blocker, the workflow should succeed once credentials are refreshed, because:
- ✅ All quality gate tests pass locally (`go vet`, `go test -race`, fuzz tests)
- ✅ The WorkflowTemplate structure is valid (YAML syntax, template references)
- ✅ Required secrets exist (docker-hub-registry, github-webhook-secret) — these are cluster-managed and unaffected by the credential expiry

**Status:**
- [ ] Test 1 (build entrypoint) — awaiting credential refresh
- [ ] Test 2 (release entrypoint with test tag) — awaiting credential refresh
- [ ] Test 3 (successful release with real tag) — awaiting real release tag + credential refresh

---

## References

- **WorkflowTemplate:** `/home/coding/declarative-config/k8s/iad-ci/argo-workflows/domain-check-workflowtemplate.yml`
- **CI/CD Documentation:** `/home/coding/domain-check/docs/plan/plan.md#ci-pipeline`
- **argo Workflows UI:** `https://argo-ci.ardenone.com` (Google SSO, VPN only)
- **ArgoCD:** `https://argocd-ro-ardenone-manager-ts.ardenone.com:8444/api/v1/applications/argo-workflows-ns-iad-ci` (read-only proxy)

## Next Steps

1. **Immediate:** Refresh iad-ci cluster credentials
2. **Then:** Execute Test 1 and Test 2 above
3. **Document:** Update this file with actual workflow run IDs, timestamps, and results
4. **Future:** When ready to cut a release, execute Test 3 with a real tag
