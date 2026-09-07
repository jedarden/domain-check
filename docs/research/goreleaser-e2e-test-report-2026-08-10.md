# GoReleaser Release Pipeline End-to-End Test Report

**Date:** 2026-08-10  
**Test scope:** Full goreleaser release pipeline from tag push to published GitHub release  
**Status:** ✅ **Configuration verified** | ⚠️ **CI workflow blocked by credentials**

## Test Summary

### ✅ Verified Components

1. **GoReleaser configuration** - Valid and functional
2. **Local build test** - All platform binaries built successfully
3. **Binary functionality** - Built binary executes correctly
4. **Archive generation** - Proper tar.gz/zip formats with LICENSE/README.md
5. **Checksums** - SHA256 checksums generated correctly
6. **Build metadata injection** - version, commit, date ldflags configured

### ⚠️ Blocked Components

1. **Argo Workflow execution** - iad-ci cluster ServiceAccount token expired (2026-08-10)
2. **GitHub release creation** - Cannot verify actual release publication
3. **Release notes formatting** - Cannot verify changelog generation on GitHub
4. **Tag-triggered automation** - Cannot verify webhook triggers workflow

## Test Results

### 1. Local GoReleaser Configuration Test

**Command:**
```bash
goreleaser check --config .goreleaser.yml
```

**Result:** ✅ PASSED
- Configuration file is valid
- No syntax or schema errors

### 2. Local Snapshot Build Test

**Command:**
```bash
goreleaser release --snapshot --clean --config .goreleaser.yml --skip=announce
```

**Result:** ✅ PASSED (5 seconds execution time)

**Build Matrix:**
| OS | Arch | Binary | Archive |
|---|------|--------|---------|
| Linux | amd64 (x86_64) | ✅ 6.2M | ✅ tar.gz |
| Linux | arm64 | ✅ 5.8M | ✅ tar.gz |
| Linux | arm (v7) | ✅ 6.0M | ✅ tar.gz |
| Darwin (macOS) | amd64 (x86_64) | ✅ 6.3M | ✅ tar.gz |
| Darwin (macOS) | arm64 (Apple Silicon) | ✅ 6.0M | ✅ tar.gz |
| Windows | amd64 (x86_64) | ✅ 6.3M | ✅ zip |
| FreeBSD | amd64 (x86_64) | ✅ 6.2M | ✅ tar.gz |
| FreeBSD | arm64 | ✅ 5.8M | ✅ tar.gz |
| FreeBSD | arm (v7) | ✅ 6.0M | ✅ tar.gz |

**Artifacts Generated:**
- ✅ 9 platform-specific binaries
- ✅ 8 tar.gz archives (Unix platforms)
- ✅ 1 zip archive (Windows)
- ✅ `checksums.txt` with SHA256 hashes for all artifacts
- ✅ `metadata.json` with build information
- ✅ Archives include LICENSE and README.md as configured

### 3. Binary Functionality Test

**Command:**
```bash
./dist/domain-check_linux_amd64_v1/domain-check help
```

**Result:** ✅ PASSED
- Binary executes without errors
- Help text displays correctly
- All subcommands documented (serve, check, bulk)
- Version injection configured (via ldflags)

### 4. Archive Content Verification

**Archive:** `domain-check_Linux_x86_64.tar.gz`

**Contents:**
```
LICENSE
README.md
domain-check
```

**Result:** ✅ PASSED
- Includes LICENSE file
- Includes README.md
- Binary present and executable

## CI/CD Pipeline Configuration

### Argo Workflow Template

**File:** `declarative-config/k8s/iad-ci/argo-workflows/domain-check-workflowtemplate.yml`

**Release Entry Point:**
```yaml
- name: release
  steps:
    - - name: quality-gate
        template: quality-gate
    - - name: goreleaser-release
        template: goreleaser-release
```

**GoReleaser Step Configuration:**
- Uses `golang:1.26-alpine` image
- Installs GoReleaser v2.5.0
- Clones repository with full history (required for tag metadata)
- Checks out the specific tag
- Runs `goreleaser release --clean`
- Passes both `GH_TOKEN` and `GITHUB_TOKEN` from `github-webhook-secret` Secret

**Quality Gate:**
- Runs `golangci-lint v1.64.8`
- Runs `go test -race` with coverage
- Runs fuzz tests (30s each for `FuzzValidateDomain` and `FuzzParseRDAPResponse`)

**Triggering:**
The workflow has a conditional entrypoint:
```yaml
- name: choose-entrypoint
  steps:
    - - name: run-release
        template: release
        when: "{{workflow.parameters.tag}} != \"\""
    - - name: run-build
        template: build
        when: "{{workflow.parameters.tag}} == \"\""
```

When `tag` parameter is provided (e.g., `"v1.0.0"`), it runs the release workflow. When empty, it runs the Docker build workflow.

## Current State

### GitHub Releases
**Status:** No releases exist despite multiple tags

**Verified:** GitHub API confirms 0 releases for `jedarden/domain-check`

**Existing Tags:**
- v1.4.0-goreleaser-final-test-2026-08-10 (current HEAD)
- v1.3.0-goreleaser-verification-test-2026-08-10
- v1.2.0-goreleaser-e2e-test-2026-08-10
- (and several earlier test tags)

**Root Cause:** The Argo workflow has not been executed for these tags, likely due to:
1. iad-ci cluster ServiceAccount token expiration (2026-08-10)
2. No manual workflow submission with tag parameter
3. No argo-events sensor triggering on tag push (or sensor not configured)

### Blocking Issue: Expired CI Credentials

**Reported in:** `docs/notes/release-workflow-status-2026-08-10.md`

**Issue:**
> Current Status: ❌ BLOCKED by expired iad-ci cluster credentials
> The workflow submission has been blocked since August 10, 2026 due to an expired ServiceAccount token for the iad-ci cluster.

**Impact:**
- Cannot submit workflows to `iad-ci` cluster
- Cannot verify Docker builds via Argo Workflows
- Cannot verify GitHub release creation via goreleaser
- Cannot test webhook triggers from GitHub tag pushes

## Expected Workflow Behavior (Not Yet Verified)

### Manual Submission

```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: domain-check-release-
  namespace: argo-workflows
spec:
  workflowTemplateRef:
    name: domain-check-build
  arguments:
    parameters:
      - name: tag
        value: "v1.5.0-test"
EOF
```

**Expected Result:**
1. Workflow starts with `tag=v1.5.0-test`
2. `choose-entrypoint` routes to `release` path (tag is non-empty)
3. `quality-gate` runs (lint, test, fuzz)
4. `goreleaser-release` executes
5. GitHub release created with tag `v1.5.0-test`
6. Release includes all 9 platform binaries
7. Release includes checksums.txt
8. Release includes auto-generated changelog

### Automated Trigger (argo-events)

**Sensor:** `declarative-config/k8s/iad-ci/argo-events/domain-check-sensor.yml`

**Expected Behavior:**
1. User pushes tag to GitHub
2. GitHub webhook triggers argo-events sensor
3. Sensor submits workflow with `tag` parameter
4. Workflow executes as above
5. Release appears on GitHub automatically

**Status:** ⚠️ NOT VERIFIED (requires cluster access)

## Recommendations

### Immediate (to unblock CI)

1. **Renew iad-ci credentials**
   - Refresh the ServiceAccount token for iad-ci cluster
   - Update `~/.kube/iad-ci.kubeconfig`
   - Test connectivity: `kubectl get workflows -n argo-workflows`

2. **Manual workflow submission test**
   - Create a test tag: `git tag v1.5.0-goreleaser-manual-test && git push origin v1.5.0-goreleaser-manual-test`
   - Submit workflow manually with the tag
   - Monitor execution: `kubectl get workflow -n argo-workflows -w`
   - Verify GitHub release creation

### Short-term (improve release process)

1. **Add version file**
   - The workflow includes a `resolve-version` step that reads from a VERSION file
   - Add `VERSION` file to repo root with current version
   - Enables auto-bumping on non-tag commits

2. **Verify argo-events sensor**
   - Ensure GitHub webhook is configured in repository settings
   - Verify sensor deployment is healthy
   - Test automatic triggering by pushing a tag

3. **Add release notes template**
   - Current `.goreleaser.yml` uses auto-changelog
   - Consider adding `release_notes_template.md` for custom formatting
   - Document what commits are excluded (docs:, test:, ci:, chore:, build:)

### Long-term (release hygiene)

1. **Tagging convention**
   - Use semantic versioning: `vX.Y.Z`
   - Avoid test suffixes for production releases
   - Keep test tags clearly marked (as currently done)

2. **Pre-release validation**
   - Run full test suite before tagging
   - Verify Docker image builds locally
   - Test release candidate binaries on target platforms

3. **Documentation**
   - Document release process in README.md
   - Add troubleshooting guide for CI failures
   - Include instruction for manual release workflow submission

## Conclusion

### ✅ Successfully Verified

- GoReleaser configuration is valid and complete
- All 9 platform binaries build successfully
- Archives include proper metadata (LICENSE, README.md)
- Checksums are generated correctly
- Binary executes and responds to commands
- Build time is fast (~5 seconds for full matrix)

### ⚠️ Requires Investigation

- Why no GitHub releases exist despite multiple tags
- argo-events sensor status and webhook configuration
- iad-ci cluster credential expiration

### ❌ Blocked by CI Access

- Full end-to-end workflow execution
- GitHub release creation
- Changelog generation
- Automated tag-triggered releases

### Next Steps

1. **Unblock CI:** Renew iad-ci cluster credentials
2. **Manual Test:** Submit workflow with test tag to verify full pipeline
3. **Document:** Update README.md with release process once verified
4. **Automate:** Ensure argo-events webhook triggers on tag push

---

**Tested by:** Claude (glm-4.7-lab-domain-check agent)  
**Commit:** b981c70  
**Tag:** v1.4.0-goreleaser-final-test-2026-08-10
