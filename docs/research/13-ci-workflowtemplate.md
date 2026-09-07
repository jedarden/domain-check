# CI WorkflowTemplate — domain-check-build

Documents the Argo Workflows WorkflowTemplate that builds and publishes the Docker image for domain-check.

**Source:** `jedarden/declarative-config` → `k8s/iad-ci/argo-workflows/domain-check-workflowtemplate.yml`

## Pipeline Flow

```
Workflow trigger (manual or EventSource)
  │
  ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 1: resolve-version                                     │
│     Clone repo from GitHub (main branch)                     │
│     Read/auto-bump VERSION file (semver patch bump)         │
│     Commit + push version bump if changed                    │
│     Output: version string (e.g. "1.3.2")                    │
└──────────────────────────┬──────────────────────────────────┘
                           │
                           ▼
┌─────────────────────────────────────────────────────────────┐
│  Step 2: docker-build                                        │
│     Kaniko builds Docker image from repo Dockerfile         │
│     Tags: ronaldraygun/domain-check:<version> + :latest      │
│     Pushes to Docker Hub                                     │
│     Retries: up to 2 on error (30s backoff, 2× factor)       │
└─────────────────────────────────────────────────────────────┘
```

## Template Structure

### Entrypoint: `build`

Two-step sequential pipeline:

1. `resolve-version` → resolves/creates a semver version string
2. `docker-build` → builds and pushes Docker image using the resolved version

### Step 1: `resolve-version`

| Property | Value |
|----------|-------|
| Image | `alpine/git` |
| Timeout | 120 seconds |
| Service account | `argo-workflow` |

**Logic:**

1. Clone `jedarden/domain-check` at the specified branch (default: `main`)
2. Check if `VERSION` file exists:
   - **No VERSION file:** Creates it with `0.1.0`, commits and pushes
   - **VERSION file exists, changed in last commit:** Reads the new version from the file
   - **VERSION file exists, unchanged:** Auto-bumps the patch version (e.g. `1.3.1` → `1.3.2`), commits and pushes the bump
3. Writes the resolved version to `/tmp/version` as an output parameter

**Git identity for auto-bumps:**
```
user.email = github@jedarden.com
user.name  = jedarden
```

**Output:** `parameters.version` — the resolved semver string.

### Step 2: `docker-build`

| Property | Value |
|----------|-------|
| Image | `gcr.io/kaniko-project/executor:latest` |
| Timeout | 1800 seconds (30 min) |
| Retries | 2 (on error, 30s backoff, 2× exponential) |
| CPU request/limit | 1000m / 4000m |
| Memory request/limit | 2Gi / 8Gi |

**Kaniko arguments:**

| Arg | Value | Purpose |
|-----|-------|---------|
| `--context` | `git://github.com/jedarden/domain-check.git#refs/heads/main` | Clone source from GitHub |
| `--dockerfile` | `Dockerfile` | Use the Dockerfile at repo root |
| `--destination` | `ronaldraygun/domain-check:<version>` | Version-tagged image |
| `--destination` | `ronaldraygun/domain-check:latest` | Rolling latest tag |
| `--build-arg` | `VERSION=<version>` | Pass version as build arg |
| `--cache` | `true` | Enable Kaniko layer caching |
| `--cache-repo` | `ronaldraygun/cache` | Docker Hub repo for cache layers |

**Context source:** Clones directly from GitHub via Kaniko's built-in git support (no artifact passing from step 1).

## Secrets and Environment Variables

| Name | Used In | Source | Purpose |
|------|---------|--------|---------|
| `GH_TOKEN` | `resolve-version` | `secret/github-webhook-secret.key:token` | GitHub API token for cloning/pushing to the repo |
| `GIT_TOKEN` | `docker-build` | `secret/github-webhook-secret.key:token` | GitHub token for Kaniko git context clone |
| Docker Hub credentials | `docker-build` | `secret/docker-hub-registry` (volume mount at `/kaniko/.docker`) | Authentication for pushing to Docker Hub |
| Service account | Both steps | `argo-workflow` (cluster-level) | RBAC permissions for pod creation, secret access |

## Secrets Required

The WorkflowTemplate expects two Kubernetes secrets in the `argo-workflows` namespace:

1. **`github-webhook-secret`** — GitHub personal access token
   - Key: `token`
   - Permissions needed: `repo` (read/write) — for cloning, committing VERSION bumps, and pushing
   - Used by both steps (as `GH_TOKEN` and `GIT_TOKEN`)

2. **`docker-hub-registry`** — Docker Hub registry credentials
   - Key: `.dockerconfigjson`
   - Contains: Docker Hub login credentials for the `ronaldraygun` account
   - Mounted as a volume at `/kaniko/.docker/config.json` for Kaniko authentication

## Artifacts CI Expects from the Repo

The CI pipeline does **not** consume any artifacts from goreleaser. The two systems are independent:

| System | Trigger | Output |
|--------|---------|--------|
| **GoReleaser** | Git tag push (`v1.2.3`) | 9 binary archives + checksums.txt → GitHub Release |
| **CI WorkflowTemplate** | Manual or EventSource trigger | 1 Docker image → Docker Hub (`ronaldraygun/domain-check`) |

The CI only needs these files to exist in the repo:

| File | Required | Purpose |
|------|----------|---------|
| `Dockerfile` | Yes | Multi-stage Docker build (golang:1.22-alpine → alpine:3.19) |
| `VERSION` | Optional (auto-created) | Semver version string for image tagging |
| Source code (go.mod, cmd/, internal/, web/) | Yes | Compiled by Dockerfile's build stage |

The CI does **not** run `go test`, `golangci-lint`, or any quality checks — it only resolves a version number and builds the Docker image.

## Gaps and Observations

### 1. No testing or quality gates in CI

The WorkflowTemplate has **no test step**. It jumps straight from version resolution to Docker build. GoReleaser's `before.hooks` run `go mod tidy` and `go generate ./...`, but neither system runs `go test ./...` or `golangci-lint`. If a commit introduces a compilation error, the Docker build step will fail (Kaniko build fails), but logic bugs, failing tests, or lint issues will ship to `:latest`.

### 2. No goreleaser integration

The CI does not invoke goreleaser at all. goreleaser runs separately (presumably manually or via a different trigger). This means:
- goreleaser produces 9 binary archives + checksums → GitHub Release (on tag push)
- CI produces 1 Docker image → Docker Hub (on workflow trigger)

There is no coordination between the two. The version resolved by `resolve-version` (from the `VERSION` file) is independent of the git tag used by goreleaser.

### 3. VERSION file vs git tag version drift

The `resolve-version` step bumps a `VERSION` file in the repo's `main` branch. goreleaser uses git tags (`v1.2.3`). If these are not kept in sync manually, the Docker image tag and the goreleaser release tag could diverge — e.g., Docker Hub has `ronaldraygun/domain-check:1.5.3` while the GitHub release is `v1.5.2`.

### 4. Kaniko clones the repo independently

Kaniko's `--context=git://...` clones the repo fresh from GitHub, independent of the `resolve-version` step's clone. The version bump commit from step 1 may not be reflected in step 2's clone if the push hasn't propagated yet (race condition). The `--build-arg=VERSION=<version>` partially mitigates this by passing the version explicitly, but the Dockerfile doesn't use a `VERSION` build arg — it hardcodes `./cmd/domain-check` without version injection.

### 5. Dockerfile version injection mismatch

The Dockerfile uses:
```dockerfile
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags='-s -w -extldflags "-static"' \
    -trimpath \
    -o domain-check \
    ./cmd/domain-check
```

It does **not** use the `--build-arg=VERSION` that Kaniko passes. goreleaser's ldflags inject `-X main.version={{.Version}}`, but the Dockerfile's ldflags do not. The binary built inside Docker will have empty/zero version vars unless the Dockerfile is updated to accept and use the `VERSION` arg.

### 6. Fixed `latest` tag on every build

Every CI run pushes `ronaldraygun/domain-check:latest`, regardless of whether the build is from `main` or a feature branch. If the workflow is triggered on a non-main branch, `latest` gets overwritten with potentially unstable code.

### 7. `gcr.io/kaniko-project/executor:latest`

The Kaniko image uses the `:latest` tag, which violates the CLAUDE.md rule: "Never use `:latest` image tags — always pin to a specific digest or version tag in manifests and Workflow templates." This could break if the Kaniko image changes unexpectedly.

## Workflow Parameters

| Parameter | Default | Description |
|-----------|---------|-------------|
| `git-repo` | `jedarden/domain-check` | GitHub repo to clone |
| `branch` | `main` | Branch to build from |
