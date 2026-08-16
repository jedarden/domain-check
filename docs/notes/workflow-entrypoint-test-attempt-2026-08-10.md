# Workflow Entrypoint Test Attempt - 2026-08-10

## Task Objective

Test both build and release entrypoints of the domain-check-build WorkflowTemplate via manual Argo Workflow submissions to verify entrypoint routing works correctly before any real tag is pushed.

## Workflow Template Verification

### Template Location
~/declarative-config/k8s/iad-ci/argo-workflows/domain-check-workflowtemplate.yml

### Entrypoints Confirmed

#### 1. build Entrypoint (Default)
Purpose: Build Docker images for main branch commits
Triggers: Push to main branch, or manual submission
Outputs: Docker images ronaldraygun/domain-check:{version,latest}

Steps:
- build-quality-gate (900s timeout)
- resolve-version (120s timeout)  
- docker-build (1800s timeout, 2 retries)

#### 2. release Entrypoint
Purpose: Create GitHub releases with goreleaser
Triggers: Git tag push with entrypoint: release parameter
Outputs: Multi-platform binaries + GitHub Release

Steps:
- quality-gate (600s timeout)
- goreleaser-release (1800s timeout)

## Test Attempts

### Attempt 1: Build Entrypoint Submission
Date: 2026-08-10 19:42 UTC
Result: BLOCKED by expired credentials
Error: the server has asked the client to provide credentials

### Attempt 2: Build with Validation Disabled
Retry: 2026-08-10 19:42 UTC
Result: BLOCKED by expired credentials
Error: unable to recognize STDIN: the server has asked the client to provide credentials

### Attempt 3: Check Recent Workflows
Result: BLOCKED by expired credentials
Error: You must be logged in to the server

## Blocker Analysis

The iad-ci cluster ServiceAccount token has expired or been revoked. This is the same blocker documented in docs/notes/release-workflow-status-2026-08-10.md.

Impact:
- Cannot submit any workflows to iad-ci cluster
- Cannot monitor existing workflow runs
- Cannot verify entrypoint routing in live environment
- Cannot test goreleaser-release step

## Expected Behavior (Once Credentials Fixed)

### Build Entrypoint Test
Expected Steps:
1. build-quality-gate runs golangci-lint, go test -race, and fuzz tests (should pass)
2. resolve-version determines the VERSION from the repo
3. docker-build builds and pushes Docker images to Docker Hub

Expected Outcome:
- Workflow completes successfully
- Two Docker images appear: ronaldraygun/domain-check:{version} and latest
- No goreleaser step is executed (build entrypoint does not include it)

### Release Entrypoint Test
Expected Steps:
1. quality-gate runs go vet and go test -race (should pass)
2. goreleaser-release installs goreleaser v2.5.0 and runs release process

Expected Outcome:
- Workflow reaches goreleaser-release step (proving entrypoint routing works)
- goreleaser fails gracefully because v0.0.0-test tag won't exist on remote
- This proves the entrypoint parameter correctly routes to the release steps
- No docker-build step is executed (release entrypoint does not include it)

## Verification Status

### Workflow Structure Verification - COMPLETE
- Confirmed build entrypoint exists in template
- Confirmed release entrypoint exists in template
- Verified build entrypoint steps: quality-gate → resolve-version → docker-build
- Verified release entrypoint steps: quality-gate → goreleaser-release
- Confirmed no goreleaser step in build entrypoint
- Confirmed no docker-build step in release entrypoint

### Runtime Verification - BLOCKED
- Cannot submit build workflow to verify default entrypoint
- Cannot submit release workflow to verify entrypoint routing parameter
- Cannot capture workflow run IDs
- Cannot monitor workflow progress
- Cannot confirm goreleaser step execution

## Acceptance Criteria Status

Criteria status:
- Submit manual build workflow: BLOCKED (Credentials expired)
- Build workflow completes with quality-gate + resolve-version + docker-build: BLOCKED (Cannot submit)
- Build workflow does NOT run goreleaser step: VERIFIED (Template structure confirmed)
- Submit manual release workflow with entrypoint: release: BLOCKED (Credentials expired)
- Release workflow reaches goreleaser step: BLOCKED (Cannot submit)
- Release workflow fails gracefully on goreleaser: BLOCKED (Cannot submit)
- Capture and document workflow run IDs: BLOCKED (Cannot list workflows)

## Conclusion

Status: Testing blocked by expired iad-ci cluster credentials

Findings:
1. Workflow template structure is correctly configured with both entrypoints
2. Build entrypoint contains: build-quality-gate → resolve-version → docker-build
3. Release entrypoint contains: quality-gate → goreleaser-release
4. No cross-contamination between entrypoints
5. Local quality gate tests all pass successfully
6. Cannot verify runtime behavior without valid credentials

Next Action:
Refresh iad-ci cluster credentials to unblock workflow testing.

Risk Level: Low (template structure is correct, local tests pass, only credential issue prevents testing)

Timeline: Unknown (awaiting cluster admin access for credential regeneration)
