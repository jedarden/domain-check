# Release Workflow Test Results

**Date:** 2026-07-02
**Template:** `domain-check-build` (WorkflowTemplate)
**Namespace:** `argo-workflows` (iad-ci cluster)

---

## Test 1: Entrypoint Routing Confirmation

**Workflow:** `domain-check-release-test-bxcg6`
**Entrypoint:** `release`
**Tag:** `v0.0.0-test`
**Result:** ❌ quality-gate failed (exit code 127 — git not installed)

```
[Failed] domain-check-release-test-bxcg6 (template=release)
  └─ [Failed] quality-gate (template=quality-gate) — Error (exit code 127)
```

The `golang:1.26-alpine` image did not include `git`, causing all shell commands to fail. Entrypoint routing was confirmed correct — the failure was an image issue.

---

## Test 2: Git Fix Verification

**Workflow:** `domain-check-release-test-258wv`
**Entrypoint:** `release`
**Tag:** `v0.0.0-test`
**Result:** ❌ quality-gate failed (exit code 2 — git installed, tag not found)

```
[Failed] domain-check-release-test-258wv (template=release)
  └─ [Failed] quality-gate (template=quality-gate) — Error (exit code 2)
```

### What Changed

| Aspect | Test 1 | Test 2 |
|--------|--------|--------|
| Exit code | 127 (command not found) | 2 (git clone failed) |
| Git installed? | No | Yes (`apk add git` now runs) |
| Failure point | `git` binary missing | `git clone --branch v0.0.0-test` fails (tag doesn't exist) |
| goreleaser-release reached? | No | No |

### Analysis

The `apk --no-cache add git ca-certificates` fix in the `quality-gate` template works. Git is now installed. The failure at exit code 2 is because `git clone --branch v0.0.0-test` cannot find the tag on the remote — this is expected for a test tag that was never pushed.

The `quality-gate` script runs `set -ex`, so the first failed command (`git clone --branch v0.0.0-test`) terminates the script before reaching `go vet` and `go test`. Similarly, `goreleaser-release` is never reached because it depends on `quality-gate` passing.

### Full Release Path Confirmation

To fully validate the `quality-gate → goreleaser-release` path, a real tag needs to exist on the remote. The entrypoint routing is confirmed from both tests, and the git fix is confirmed from the exit code change (127 → 2).

To do a complete end-to-end test:
1. Push a real tag (e.g., `git tag v0.0.0-test && git push origin v0.0.0-test`)
2. Resubmit the same workflow
3. quality-gate should clone successfully, then run `go vet ./...` and `go test -race ./...`
4. goreleaser-release should clone, checkout the tag, and attempt goreleaser release

### Workflow Submission Command

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

## Key Takeaways

1. **Entrypoint routing works** — `entrypoint: release` correctly routes to the `release` template's step sequence (`quality-gate → goreleaser-release`)
2. **Git fix works** — `apk add git` in the `quality-gate` container resolves the exit code 127 issue
3. **Full path requires a real tag** — `git clone --branch $TAG` fails without a tag on the remote, preventing quality-gate from running `go vet`/`go test` and preventing goreleaser-release from being reached
4. **goreleaser-release also needs the tag** — its script does `git checkout $TAG` which would also fail for a non-existent tag, but the step is gated behind quality-gate

---

## Most Recent domain-check-build Workflow (2026-07-03)

**Date queried:** 2026-07-03
**Namespace:** `argo-workflows` (iad-ci cluster)
**Template:** `domain-check-build` (exists, created 2026-05-27T02:17:56Z, age 37d)

### Finding: No domain-check-build workflow runs present

All `domain-check-build` workflow instances have been cleaned up from the cluster (podGC policy deletes pods on completion, and workflows are garbage-collected). No active, succeeded, or failed workflow instances with this template name exist in the namespace.

The most recent domain-check-related workflows were the `domain-check-release-test-*` manual runs documented above (Test 1 and Test 2), which have also been cleaned up.

### Workflow Template Status

| Field | Value |
|-------|-------|
| Name | `domain-check-build` |
| Namespace | `argo-workflows` |
| Created | 2026-05-27T02:17:56Z |
| Age | 37 days |
| ResourceVersion | 40239012 |
| Generation | 4 (updated 3 times since creation) |

### Summary

- **Most recent domain-check-build workflow run:** None present (cleaned up)
- **Status:** N/A — no workflow instances found
- **Template exists:** Yes, ready for new submissions
