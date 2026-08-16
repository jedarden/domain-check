# Workflow Entrypoint Test Commands - Ready for Execution

**Status:** Ready to execute once iad-ci credentials are refreshed

## Prerequisites

- Valid iad-ci cluster kubeconfig at `/home/coding/.kube/iad-ci.kubeconfig`
- ServiceAccount token for argo-workflow namespace (not expired)

## Test 1: Build Entrypoint (Default)

**Purpose:** Verify build workflow runs quality-gate → resolve-version → docker-build (no goreleaser)

```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<'EOF'
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: domain-check-build-test-
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
EOF
```

**Expected Result:**
- Workflow ID: `domain-check-build-test-XXXXX`
- Steps executed: `build-quality-gate` → `resolve-version` → `docker-build`
- Docker images pushed: `ronaldraygun/domain-check:{version}` and `ronaldraygun/domain-check:latest`
- NO `goreleaser-release` step (verifies entrypoint isolation)

## Test 2: Release Entrypoint

**Purpose:** Verify release workflow routes to quality-gate → goreleaser-release

```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<'EOF'
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
      - name: tag
        value: v0.0.0-test
EOF
```

**Expected Result:**
- Workflow ID: `domain-check-release-test-XXXXX`
- Steps executed: `quality-gate` → `goreleaser-release`
- `goreleaser-release` step fails at `git checkout v0.0.0-test` or `git describe --tags --exact-match` (expected — test tag doesn't exist)
- NO `docker-build` step (verifies entrypoint routing)

## Monitoring Commands

```bash
# List recent workflows
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
  get workflows -n argo-workflows --sort-by=.metadata.creationTimestamp | tail -20

# Get workflow status
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
  get workflow <workflow-name> -n argo-workflows \
  -o jsonpath='{.status.phase} - {.status.message}'

# Get per-node failure details
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
  get workflow <workflow-name> -n argo-workflows -o json | python3 -c "
import json,sys
w = json.load(sys.stdin)
for node in w['status'].get('nodes',{}).values():
    if node.get('phase') in ('Failed','Error'):
        print(node['displayName'], '-', node['phase'])
        print('  msg:', node.get('message',''))
"

# Stream logs from a running pod (must be caught WHILE running)
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
  get pods -n argo-workflows -l workflows.argoproj.io/workflow=<workflow-name>

kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
  logs -n argo-workflows <pod-name> -c main -f
```

## Current Blocker

**Error:** `the server has asked the client to provide credentials`

**Root Cause:** iad-ci cluster ServiceAccount token expired (2026-08-10)

**Impact:** Cannot submit workflows or monitor existing runs

**Resolution Required:** Refresh ServiceAccount token for iad-ci cluster

## Template Verification (Completed)

From `~/declarative-config/k8s/iad-ci/argo-workflows/domain-check-workflowtemplate.yml`:

- ✓ Build entrypoint exists (line 34-45)
- ✓ Release entrypoint exists (line 47-52)
- ✓ Build steps: `build-quality-gate` → `resolve-version` → `docker-build`
- ✓ Release steps: `quality-gate` → `goreleaser-release`
- ✓ No goreleaser in build entrypoint
- ✓ No docker-build in release entrypoint
- ✓ Local quality gate tests pass

**Ready to execute:** Once credentials are refreshed, run the two submission commands above to verify entrypoint routing.
