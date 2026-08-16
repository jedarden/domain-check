# WorkflowTemplate Validation Report

**Date:** 2026-08-10  
**Template:** domain-check-build  
**File:** declarative-config/k8s/iad-ci/argo-workflows/domain-check-workflowtemplate.yml  
**Last Commit:** 2cad678b "ci: configure tag-triggered goreleaser execution"

---

## LOCAL VALIDATION: PASSED ✓

### YAML Structure
- Valid YAML syntax (no parse errors)
- All required fields present:
  - `apiVersion: argoproj.io/v1alpha1`
  - `kind: WorkflowTemplate`
  - `metadata.name: domain-check-build`
  - `metadata.namespace: argo-workflows`
  - `spec.templates: 8 templates defined`

### Templates Defined (8 total)
1. `choose-entrypoint` — Router based on `tag` parameter presence
2. `build` — Main build pipeline (quality-gate → resolve-version → docker-build)
3. `release` — Release pipeline (quality-gate → goreleaser-release)
4. `resolve-version` — VERSION file logic with auto-bump
5. `docker-build` — Kaniko executor image build
6. `build-quality-gate` — Tests for build (lint, race, fuzz)
7. `quality-gate` — Tests for release (lint, race, fuzz)
8. `goreleaser-release` — GoReleaser execution for GitHub releases

### Template References: VALID ✓
All referenced templates exist in the spec:
- `choose-entrypoint` → references `release`, `build`
- `build` → references `build-quality-gate`, `resolve-version`, `docker-build`
- `release` → references `quality-gate`, `goreleaser-release`

### Parameter Flow: VALID ✓
- `tag` parameter controls entrypoint routing (empty = build, non-empty = release)
- `version` output parameter flows from `resolve-version` → `docker-build`
- GitHub token mounted from `github-webhook-secret` secret
- Docker config mounted from `docker-hub-registry` secret

### Resource Limits: CONFIGURED ✓
All templates have explicit CPU/memory requests and limits

### Active Deadlines: CONFIGURED ✓
- Workflow-level: 12900s (prevents hung runs before pod starts)
- Step-level: 120s (resolve-version) to 1800s (docker-build, goreleaser-release)

### Security: VALID ✓
- Non-root containers not enforced (acceptable for build containers)
- Secrets mounted via `secretKeyRef` (no plaintext credentials)
- Git tokens passed via environment variables from secrets

---

## CLUSTER VERIFICATION: BLOCKED ❌

**BLOCKING ISSUE:** Expired iad-ci cluster ServiceAccount token (since 2026-08-10)

The following verification steps cannot complete until the iad-ci cluster credentials are refreshed:

### 1. Check WorkflowTemplate exists on cluster

```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
  get workflowtemplate domain-check-build -n argo-workflows
```

**Expected result:**
```
NAME                    AGE
domain-check-build      Xd
```

**Current status:** Cannot verify — credentials expired

---

### 2. Verify ArgoCD sync status

```bash
kubectl --kubeconfig=/home/coding/.kube/ardenone-manager.kubeconfig \
  get application argo-workflows-ns-iad-ci -n argocd \
  -o jsonpath='{.status.health.status} {.status.sync.status}'
```

**Expected result:** `Healthy Synced`

**Current status:** Cannot verify — requires access to ardenone-manager cluster

---

### 3. Check for ArgoCD sync errors

```bash
kubectl --kubeconfig=/home/coding/.kube/ardenone-manager.kubeconfig \
  get application argo-workflows-ns-iad-ci -n argocd \
  -o jsonpath='{.status.operationState.message}'
```

**Expected result:** No errors, sync successful

**Current status:** Cannot verify — requires access to ardenone-manager cluster

---

### 4. Verify no ArgoCD reconcile errors

Check ArgoCD application controller logs for errors related to this WorkflowTemplate:

```bash
kubectl --kubeconfig=/home/coding/.kube/ardenone-manager.kubeconfig \
  logs -n argocd $(kubectl --kubeconfig=/home/coding/.kube/ardenone-manager.kubeconfig \
    get pods -n argocd -l app.kubernetes.io/name=argocd-application-controller \
    -o jsonpath='{.items[0].metadata.name}') --tail=1000 | \
  grep -i "domain-check" | tail -20
```

**Expected result:** No error messages, successful sync logs

**Current status:** Cannot verify — requires access to ardenone-manager cluster

---

## NEXT STEPS (Once Credentials Refreshed)

1. **Refresh iad-ci ServiceAccount token** via Rackspace Spot UI
2. **Run cluster verification commands** above
3. **If WorkflowTemplate is missing** from cluster, check ArgoCD logs
4. **If sync errors exist**, fix manifest in declarative-config and re-sync
5. **Submit test workflow** to verify execution:

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
  arguments:
    parameters:
      - name: tag
        value: ""  # Empty tag = build entrypoint
EOF
```

6. **Monitor test execution:**

```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig \
  get workflows -n argo-workflows -l workflows.argoproj.io/workflow-template=domain-check-build
```

---

## CREDENTIAL ISSUE REFERENCE

See: `docs/notes/release-workflow-status-2026-08-10.md`

The workflow submission and validation have been blocked since August 10, 2026 due to an expired ServiceAccount token for the iad-ci cluster. This is a known infrastructure issue and does **not** indicate a problem with the WorkflowTemplate itself.

Local quality gate tests all pass successfully (`go vet`, `go test -race`, fuzz tests), indicating that the workflow should complete successfully once credentials are refreshed.

---

## ACCEPTANCE CRITERIA STATUS

| Criterion | Status | Notes |
|-----------|--------|-------|
| YAML validation | ✓ PASSED | Syntax valid, all fields present |
| Commit and push changes | ✓ ALREADY DONE | Commit 2cad678b |
| Verify ArgoCD sync | ❌ BLOCKED | Expired credentials |
| Confirm WorkflowTemplate on cluster | ❌ BLOCKED | Expired credentials |
| No ArgoCD sync errors | ❌ BLOCKED | Expired credentials |

---

## CONCLUSION

**Local validation:** PASSED ✓

**Cluster verification:** BLOCKED by expired iad-ci credentials ❌

The WorkflowTemplate YAML is syntactically valid and structurally sound. All template references are correct, parameter flow is logical, and resource limits are configured. However, cluster verification cannot complete until the iad-ci ServiceAccount token is refreshed.

Once credentials are refreshed, run the verification commands in the "Next Steps" section to confirm ArgoCD sync and WorkflowTemplate presence on the cluster.
