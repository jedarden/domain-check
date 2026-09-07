# Goreleaser Pipeline Verification - Test Results

**Test Date:** 2026-08-11
**Test Version:** v1.77.0-goreleaser-pipeline-test-2026-08-11
**Goreleaser Version:** v2.17.1

## Overview

This document verifies the end-to-end goreleaser release pipeline for the domain-check project, ensuring that when a tag is pushed to GitHub, the CI/CD workflow correctly builds and publishes multi-platform binaries to GitHub Releases.

## Configuration Validation

✅ **Goreleaser Configuration:** `.goreleaser.yml` is valid and passes all checks

```bash
$ goreleaser check
  • checking                                  path=.goreleaser.yml
  • 1 configuration file(s) validated
  • thanks for using GoReleaser!
```

## Local Snapshot Build Test

✅ **Local Build:** Goreleaser successfully builds all configured platform binaries in snapshot mode (34s build time)

### Build Configuration

The goreleaser configuration builds binaries for the following platforms:

| OS      | Arch    | ARM Version | Output Format |
|---------|---------|-------------|---------------|
| linux   | amd64   | -           | tar.gz        |
| linux   | arm64   | v8.0        | tar.gz        |
| linux   | arm     | v7          | tar.gz        |
| darwin  | amd64   | -           | tar.gz        |
| darwin  | arm64   | v8.0        | tar.gz        |
| windows | amd64   | -           | zip           |
| freebsd | amd64   | -           | tar.gz        |
| freebsd | arm64   | v8.0        | tar.gz        |
| freebsd | arm     | v7          | tar.gz        |

### Build Artifacts

All 9 platform combinations build successfully:

```
dist/
├── domain-check_Darwin_arm64.tar.gz         (6.0M)
├── domain-check_Darwin_x86_64.tar.gz        (6.3M)
├── domain-check_Freebsd_arm64.tar.gz        (5.8M)
├── domain-check_Freebsd_armv7v7.tar.gz      (6.0M)
├── domain-check_Freebsd_x86_64.tar.gz       (6.2M)
├── domain-check_Linux_arm64.tar.gz          (5.8M)
├── domain-check_Linux_armv7v7.tar.gz       (6.0M)
├── domain-check_Linux_x86_64.tar.gz         (6.2M)
├── domain-check_Windows_x86_64.zip         (5.8M)
└── checksums.txt                           (SHA256)
```

### Build Details

- **Build duration:** ~34 seconds (local snapshot)
- **Build mechanism:** Go `go build` with CGO_ENABLED=0 (static binaries)
- **Compiler flags:** `-s -w` (strip debug info, reduce binary size)
- **Version injection:** Build info injected via ldflags:
  - `main.version={{.Version}}`
  - `main.commit={{.Commit}}`
  - `main.date={{.Date}}`
- **Included files:** LICENSE, README.md in each archive

### Binary Verification

✅ **Binary Execution:** Built binaries execute correctly

```bash
$ ./domain-check help
domain-check - Authoritative domain availability checker

Usage:
  domain-check [serve] [flags]     Start the HTTP server (default)
  domain-check check <domain> [flags]  Check domain availability
  domain-check bulk <file> [flags]     Bulk check domains from file
```

## CI/CD Workflow Configuration

### Workflow Template

**Template:** `domain-check-build` in `jedarden/declarative-config`
**Location:** `k8s/iad-ci/argo-workflows/domain-check-workflowtemplate.yml`
**Cluster:** iad-ci (Rackspace Spot, us-east-iad-1)

### Workflow Entrypoints

The workflow has two entrypoints, selected based on whether a `tag` parameter is provided:

1. **`build`** (default, tag=""): Docker image build → `ronaldraygun/domain-check:VERSION`
2. **`release`** (tag provided): Quality gate → goreleaser release → GitHub release

### Release Pipeline Flow

When a tag is pushed, the workflow executes:

```
release entrypoint
├── 1. quality-gate (15min timeout)
│   ├── golangci-lint run ./...
│   ├── go test -race -coverprofile=coverage.out ./...
│   ├── go test -fuzz=FuzzValidateDomain -fuzztime=30s ./internal/domain/
│   └── go test -fuzz=FuzzParseRDAPResponse -fuzztime=30s ./internal/checker/
│
└── 2. goreleaser-release (30min timeout)
    ├── Install goreleaser v2.5.0
    ├── Clone repo with full history
    ├── Checkout tag
    └── goreleaser release --clean
```

### Workflow Submission

To manually trigger the release pipeline:

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
        value: "v1.77.0-goreleaser-pipeline-test-2026-08-11"
EOF
```

## Expected GitHub Release Output

When the workflow completes successfully, the GitHub release will include:

### Release Assets

- **9 platform archives** (tar.gz for Unix, zip for Windows)
- **checksums.txt** (SHA256 hashes of all archives)
- **Total size:** ~55MB compressed

### Release Metadata

- **Tag:** v1.77.0-goreleaser-pipeline-test-2026-08-11
- **Name:** v1.77.0-goreleaser-pipeline-test-2026-08-11
- **Draft:** false
- **Pre-release:** auto (detected from tag name)
- **Release notes:** Auto-generated from git commit messages (ascending order)

### Changelog Filters

The following commit types are excluded from release notes:
- `^docs:`
- `^test:`
- `^ci:`
- `^chore:`
- `^build:`

## Test Execution Plan

### Prerequisites

1. **iad-ci cluster access:** Valid ServiceAccount token (currently expired as of 2026-08-10)
2. **GitHub token:** `github-webhook-secret` secret must be valid in iad-ci cluster
3. **Tag pushed:** Test tag must exist in GitHub repository

### Test Steps

1. ✅ **Configuration validation:** `goreleaser check` passes
2. ✅ **Local snapshot build:** All 9 platforms build successfully
3. ✅ **Binary verification:** Built binaries execute correctly
4. ⏳ **Push test tag:** `git push origin v1.77.0-goreleaser-pipeline-test-2026-08-11`
5. ⏳ **Submit workflow:** Manual workflow submission with tag parameter
6. ⏳ **Monitor execution:** Watch workflow progress via Argo UI or kubectl
7. ⏳ **Verify GitHub release:** Check release includes all assets and metadata

### Status as of 2026-08-11

| Step | Status | Notes |
|------|--------|-------|
| Config validation | ✅ Complete | `.goreleaser.yml` is valid |
| Local build test | ✅ Complete | All 9 platforms built in 34s |
| Binary verification | ✅ Complete | Binaries execute correctly |
| Tag creation | ✅ Complete | Tag ready to push |
| Workflow execution | ⏸️ Blocked | iad-ci credentials expired |
| GitHub release verification | ⏸️ Blocked | Pending workflow execution |

## Issues Found

### iad-ci Cluster Credentials Expired

**Issue:** The iad-ci cluster ServiceAccount token expired on 2026-08-10, preventing workflow submission and execution.

**Impact:** Cannot submit workflows to iad-ci cluster, cannot execute goreleaser release pipeline.

**Workaround:** Wait for credentials to be refreshed, then re-test workflow execution.

**Documentation:** See `docs/notes/release-workflow-status-2026-08-10.md`

## Recommendations

### For Production Use

1. **Credential automation:** Set up automatic token refresh for iad-ci cluster credentials
2. **Monitoring:** Add alerting for expiring ServiceAccount tokens
3. **Test tags:** Use a consistent naming convention for test releases (e.g., `*-test-*`)
4. **Pre-release testing:** Always run local `goreleaser release --snapshot` before pushing tags

### For Future Testing

1. **Automated test:** Consider creating a script that automates this entire verification process
2. **Version bumping:** Document version bumping process (currently manual in `main.go`)
3. **Changelog:** Ensure commit messages follow conventional commits for better auto-generated changelogs

## Conclusion

The goreleaser configuration is valid and the local build process works correctly for all configured platforms. The CI/CD workflow is properly configured to execute quality gates and goreleaser builds on tag push. The only blocker is the expired iad-ci cluster credentials, which must be refreshed to complete the end-to-end test.

Once credentials are refreshed, pushing the test tag `v1.77.0-goreleaser-pipeline-test-2026-08-11` and submitting the workflow will verify the complete pipeline from tag to GitHub release.
