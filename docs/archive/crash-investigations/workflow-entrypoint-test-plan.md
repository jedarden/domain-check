# Workflow Entrypoint Test Plan — domain-check-build

**Status:** BLOCKED — Expired iad-ci credentials (as of 2026-08-10)

**Purpose:** Verify both the `build` and `release` entrypoints work correctly before any real tag is pushed.

## Background

The `domain-check-build` WorkflowTemplate has two entrypoints:

1. **`build`** (default): Runs on every push to main
   - Steps: `build-quality-gate` → `resolve-version` → `docker-build`
   - Used for: Continuous integration, Docker image builds

2. **`release`**: Runs on git tag push
   - Steps: `quality-gate` → `goreleaser-release`
   - Used for: GitHub releases with binaries

## Test Strategy

### Test 1: Build Entrypoint (Default)

**Submission:**
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: domain-check-build-manual-
  namespace: argo-workflows
spec:
  workflowTemplateRef:
    name: domain-check-build
```

**Command:**
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
EOF
```

**Expected Behavior:**
1. Workflow uses default `entrypoint: build`
2. Runs three steps in sequence:
   - `build-quality-gate` (900s limit): golangci-lint, go test -race, fuzz tests
   - `resolve-version` (120s limit): Auto-bump VERSION file or initialize
   - `docker-build` (1800s limit, retry 2×): Kaniko build → ronaldraygun/domain-check
3. Should NOT run `goreleaser-release` step
4. Should complete successfully (assuming quality gate passes)

**Verification Commands:**
```bash
# List recent workflows
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get workflows -n argo-workflows --sort-by=.metadata.creationTimestamp | tail -5

# Get workflow status
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get workflow <workflow-name> -n argo-workflows

# Get detailed status with node results
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get workflow <workflow-name> -n argo-workflows -o json | python3 -c "
import json,sys
w = json.load(sys.stdin)
print('Phase:', w['status'].get('phase'))
print('Message:', w['status'].get('message'))
print()
for node in w['status'].get('nodes',{}).values():
    if node.get('phase') in ('Failed','Error'):
        print(node['displayName'], '-', node['phase'])
        print('  msg:', node.get('message',''))
"

# Stream logs while running (must catch it while pod is alive)
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig get pods -n argo-workflows -l workflows.argoproj.io/workflow=<workflow-name>
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig logs -n argo-workflows <pod-name> -c main -f
```

**Expected Result:**
- Phase: `Succeeded`
- Steps executed: `build-quality-gate`, `resolve-version`, `docker-build`
- Docker image pushed: `ronaldraygun/domain-check:<version>` and `ronaldraygun/domain-check:latest`
- No `goreleaser-release` step in execution graph

---

### Test 2: Release Entrypoint (Override)

**Submission:**
```yaml
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: domain-check-release-manual-
  namespace: argo-workflows
spec:
  entrypoint: release  # Override default
  arguments:
    parameters:
      - name: tag
        value: "v0.0.0-test"  # Non-existent tag for testing
  workflowTemplateRef:
    name: domain-check-build
```

**Command:**
```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<'EOF'
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

**Expected Behavior:**
1. Workflow uses overridden `entrypoint: release`
2. Sets `workflow.parameters.tag = "v0.0.0-test"`
3. Runs two steps in sequence:
   - `quality-gate` (600s limit): go vet, go test -race (clones at tag)
   - `goreleaser-release` (1800s limit): Runs goreleaser release
4. `goreleaser-release` will fail (expected): Tag `v0.0.0-test` doesn't exist on remote
5. **Critical:** The workflow should **reach** the `goreleaser-release` step, proving entrypoint routing works

**Expected Failure Mode:**
The `goreleaser-release` step should fail with:
```
error: git clone: fatal: couldn't find remote ref refs/tags/v0.0.0-test
```
or from goreleaser:
```
ERROR  tag 'v0.0.0-test' not found
```

This is **expected and acceptable** — we're testing the entrypoint routing, not the release itself.

**Verification:**
Same commands as Test 1, plus checking that:
- Phase: `Error` or `Failed`
- Failed step: `goreleaser-release` (not earlier steps)
- `quality-gate` step: `Succeeded`
- No `resolve-version` or `docker-build` steps in execution graph

**Expected Result:**
- Phase: `Error`
- Steps executed: `quality-gate` (succeeded), `goreleaser-release` (failed)
- Failure in: `goreleaser-release` step only
- No `resolve-version` or `docker-build` steps
- **This proves entrypoint routing works:**
  - ✅ Release entrypoint was used (not build)
  - ✅ Quality gate ran (tag was passed correctly)
  - ✅ goreleaser step was reached (entrypoint routing successful)

---

## Success Criteria

### Test 1 (Build Entrypoint)
- [ ] Workflow completes successfully
- [ ] Runs only: `build-quality-gate` → `resolve-version` → `docker-build`
- [ ] Does NOT run `goreleaser-release`
- [ ] Docker image pushed to `ronaldraygun/domain-check`

### Test 2 (Release Entrypoint)
- [ ] Workflow uses `release` entrypoint (verified by execution graph)
- [ ] Runs only: `quality-gate` → `goreleaser-release`
- [ ] Does NOT run `resolve-version` or `docker-build`
- [ ] Reaches `goreleaser-release` step (proves routing works)
- [ ] Fails in `goreleaser-release` with tag-not-found error (expected)

## Documented Results

**Workflow Run IDs:** (To be filled after submission)
- Build workflow: `domain-check-build-manual-<xxxxxxxx>`
- Release workflow: `domain-check-release-manual-<xxxxxxxx>`

**Results:** (To be filled after execution completes)
- Build workflow phase: `<Succeeded|Failed|Error>`
- Release workflow phase: `<Succeeded|Failed|Error>`
- Release workflow failed at: `<step-name>`

## Notes

- **Credential Issue:** As of 2026-08-10, iad-ci cluster credentials are expired. These submissions will fail until credentials are refreshed.
- **Argo UI:** Workflows can be monitored at https://argo-ci.ardenone.com (VPN only, Google SSO)
- **Pod Cleanup:** Argo Workflows uses `podGC: OnPodCompletion` — pods are deleted immediately after completion. Logs must be captured while running or via Argo UI.
- **TTL:** Successful workflows expire after 30min, failed after 2h. Results disappear after TTL — document promptly.

## References

- WorkflowTemplate: `/home/coding/declarative-config/k8s/iad-ci/argo-workflows/domain-check-workflowtemplate.yml`
- CI/CD Documentation: `docs/plan/plan.md` section "CI/CD — Argo Workflows (iad-ci)"
- Manual Submissions: `docs/workflow-test-manifests.yaml`
