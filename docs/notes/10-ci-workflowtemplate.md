# CI WorkflowTemplate: domain-check-build

Documents the Argo Workflows WorkflowTemplate that handles Docker image builds and GitHub releases for domain-check. The template lives in `declarative-config` at `k8s/iad-ci/argo-workflows/domain-check-workflowtemplate.yml` and is synced to the `iad-ci` cluster via ArgoCD.

## Template Overview

| Field | Value |
|-------|-------|
| **Name** | `domain-check-build` |
| **Namespace** | `argo-workflows` |
| **ServiceAccount** | `argo-workflow` |
| **Entrypoints** | `build` (default, push to main) and `release` (tag push v*) |
| **Source repo** | `github.com/jedarden/domain-check` (parameterized, default: `main` branch) |

## Automated Triggering

The `domain-check-sensor` (in `declarative-config` at `k8s/iad-ci/argo-events/domain-check-sensor.yml`) watches for push events via the `github-webhooks` EventSource and routes them to the correct entrypoint:

| Event | Filter | Entrypoint | What Runs |
|-------|--------|------------|-----------|
| Push to `main` | `body.ref == refs/heads/main` | `build` | quality gate → version resolve → Docker build |
| Tag push `v*` | `body.ref =~ ^refs/tags/v[0-9]+\.[0-9]+\.[0-9]+` | `release` | quality gate → goreleaser → GitHub Release |

CI auto-bump commits (author `Argo Workflows CI`) are filtered out to prevent cascade loops.

## Step Pipeline

The template has two entrypoints:

### Build pipeline (entrypoint: `build`)

Triggered on push to main. Three sequential steps:

```
build (entrypoint)
├── Step 1: build-quality-gate — go vet + go test -race (clones by branch)
├── Step 2: resolve-version     — Auto-bump or read VERSION file
└── Step 3: docker-build       — Kaniko Docker build → Docker Hub
```

**Note:** GoReleaser does NOT run in the build pipeline. It runs only via the `release` entrypoint on tag pushes (v* tags).

### Release pipeline (entrypoint: `release`)

Triggered on tag push (v*). Two sequential steps:

```
release
├── Step 1: quality-gate       — go vet + go test -race (clones by tag)
└── Step 2: goreleaser-release — Cross-compile + publish GitHub Release
```

### Step 1: build-quality-gate

**Purpose:** Validate code quality before building the Docker image. Clones the repo by branch and runs vet + test.

**Image:** `golang:1.26-alpine`

**Logic:**
1. Clone the repo using the branch from `workflow.parameters.branch`
2. Run `go vet ./...`
3. Run `go test -race ./...`

**Resources:**

| | Request | Limit |
|--|---------|-------|
| CPU | 1000m | 4000m |
| Memory | 2Gi | 4Gi |

**Deadline:** 600s (10 minutes).

**Secrets:**
- `GH_TOKEN` from `github-webhook-secret` (key: `token`) — used for git clone

**Difference from `quality-gate`:** The `quality-gate` template (used by the release pipeline) clones by tag (`workflow.parameters.tag`), while `build-quality-gate` clones by branch. This distinction is necessary because branch pushes don't have a tag reference.

### Step 2: resolve-version

**Purpose:** Determine the image version string by reading or auto-bumping the `VERSION` file in the repo.

**Image:** `alpine/git`

**Logic:**
1. Clone the repo (branch from `workflow.parameters.branch`) using `GH_TOKEN` for auth
2. If `VERSION` file exists:
   - Check if `VERSION` was changed in the latest commit (`git diff --name-only HEAD~1 HEAD -- VERSION`)
   - If **changed** (manual version bump by a developer): read and use that version as-is
   - If **unchanged**: auto-increment the patch version (e.g. `0.1.0` → `0.1.1`), commit, and push
3. If `VERSION` does not exist: create it with `0.1.0`, commit, and push
4. Write the resolved version to `/tmp/version` (output parameter)

**Output parameter:** `version` — the resolved semver string, passed to the next step.

**Secrets:**
- `GH_TOKEN` from `github-webhook-secret` (key: `token`) — used for git clone + push

**Git identity:** `github@jedarden.com` / `jedarden` (per CLAUDE.md convention)

### Step 3: docker-build

**Purpose:** Build a Docker image from the repo's `Dockerfile` and push to Docker Hub.

**Image:** `gcr.io/kaniko-project/executor:latest`

**Kaniko build arguments:**

| Arg | Value | Description |
|-----|-------|-------------|
| `--context` | `git://github.com/jedarden/domain-check.git#refs/heads/main` | Source context (git-based, no local checkout) |
| `--dockerfile` | `Dockerfile` | Dockerfile location in the repo root |
| `--destination` | `ronaldraygun/domain-check:<version>` | Version-tagged image |
| `--destination` | `ronaldraygun/domain-check:latest` | Rolling latest tag |
| `--build-arg` | `VERSION=<version>` | Injects version into the build |
| `--cache=true` | — | Enable layer caching |
| `--cache-repo` | `ronaldraygun/cache` | Cache storage on Docker Hub |

**Resources:**

| | Request | Limit |
|--|---------|-------|
| CPU | 1000m | 4000m |
| Memory | 2Gi | 8Gi |

**Retry strategy:** Up to 2 retries on error, exponential backoff (30s × 2^n).

**Deadline:** 1800s (30 minutes).

**Secrets:**
- `GIT_TOKEN` from `github-webhook-secret` (key: `token`) — used by Kaniko to clone the git context
- Docker Hub credentials from `docker-hub-registry` Secret (`.dockerconfigjson` key), mounted at `/kaniko/.docker`

**Volume:** `docker-config` — a volume from the `docker-hub-registry` Secret providing Docker Hub authentication.

## Artifacts CI Produces

The build pipeline produces **two artifact types**: a Docker image and cross-compiled binaries.

| Tag | Example | Purpose |
|-----|---------|---------|
| `<version>` | `ronaldraygun/domain-check:0.1.5` | Immutable, version-pinned deployment target |
| `latest` | `ronaldraygun/domain-check:latest` | Rolling tag for convenience |

The release pipeline produces GitHub Releases with binaries and checksums.

## Build Pipeline: GoReleaser Step (Removed)

The `goreleaser` template was previously part of the build pipeline (Step 3), running `goreleaser release --clean` on every push to main. This was removed because:

1. GoReleaser requires a git tag to produce a release — running without a tag is incorrect
2. Cross-compiled binaries should only be published on intentional releases (tag pushes)
3. The `release` entrypoint already handles this correctly with its own `goreleaser-release` template

The `release` entrypoint (triggered on `v*` tag pushes) runs `quality-gate → goreleaser-release`, which properly validates code quality and publishes GitHub Releases.

## Release Pipeline: GoReleaser Release Step

**Template name:** `goreleaser-release`

**Image:** `golang:1.26-alpine` (installs goreleaser v2.5.0 via wget)

**Logic:**
1. Install goreleaser binary
2. Clone repo with full history (goreleaser needs tag metadata)
3. Checkout the tag from `workflow.parameters.tag`
4. Run `goreleaser release --clean`

**Secrets:**
- `GH_TOKEN` from `github-webhook-secret` (key: `token`) — used for git clone
- `GITHUB_TOKEN` from `github-webhook-secret` (key: `token`) — used by goreleaser for GitHub Release creation

## Secrets Required

| Secret Name | Key | Used By | Purpose |
|-------------|-----|---------|---------|
| `github-webhook-secret` | `token` | `resolve-version` (`GH_TOKEN`), `docker-build` (`GIT_TOKEN`), `quality-gate` (`GH_TOKEN`), `goreleaser-release` (`GH_TOKEN` + `GITHUB_TOKEN`) | GitHub API access for git clone/push, Kaniko git context, and goreleaser release creation |
| `docker-hub-registry` | `.dockerconfigjson` | `docker-build` (volume mount) | Docker Hub push authentication |

Both secrets must exist in the `argo-workflows` namespace on the `iad-ci` cluster.

## Gaps Between CI and GoReleaser

### 1. VERSION vs Git Tags — Dual Source of Truth

The CI WorkflowTemplate uses a `VERSION` file in the repo root as the version source. GoReleaser uses git tags (e.g. `v0.1.0`). These are **not automatically synchronized**:

- CI auto-increments the `VERSION` file on every run (patch bump)
- GoReleaser requires a `git tag` to trigger a release
- If a developer runs CI but forgets to tag, the Docker image gets a version that has no corresponding GoReleaser release
- If a developer tags without updating `VERSION`, the CI auto-bump will skip (detects VERSION change) but the Docker tag will use the tag version, which may diverge from the auto-bumped VERSION

**Impact:** Docker image tags and GitHub release tags may not match. This is manageable but requires awareness.

### 2. VERSION Build Arg Unused in Dockerfile

The CI passes `--build-arg=VERSION={{inputs.parameters.version}}` to Kaniko, but the current `Dockerfile` does not use the `VERSION` ARG in its `go build` command. GoReleaser, by contrast, injects version metadata via ldflags (`-X main.version={{.Version}}`, `main.commit={{.Commit}}`, `main.date={{.Date}}`).

**Impact:** The Docker image binary does not report its version via `main.version` — the binary built inside Docker has no version info injected, unlike the GoReleaser-built binaries.

### 3. No Kaniko `--snapshot-mode` Configured

Kaniko defaults to `full` snapshot mode which hashes the full file contents for layer caching. For Go projects, `--snapshot-mode=redo` (only re-hashes changed files based on git diff) could significantly speed up rebuilds. Not a gap per se, but an optimization opportunity.

### 4. No `--no-push` / Validation Step

The CI goes straight to build + push with no intermediate validation step (e.g., lint, test, vet). If the code doesn't compile or tests fail, the Docker build step will fail after consuming the full 30-minute timeout. Other WorkflowTemplates (e.g., `forge-ci`, `needle-ci`) include explicit build/test steps before the image build.

**Note:** The `Dockerfile` runs `go build` but not `go test`. Tests are expected to pass before CI runs (via developer habit or separate trigger).

### 5. Kaniko `latest` Image Tag

The `docker-build` step uses `gcr.io/kaniko-project/executor:latest` which violates the CLAUDE.md rule: "Never use `:latest` image tags — always pin to a specific digest or version tag." Should be pinned to a specific version digest for reproducibility.

## How to Submit Manually

### Build (equivalent to push to main):

```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
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

### Release (equivalent to tag push):

```bash
kubectl --kubeconfig=/home/coding/.kube/iad-ci.kubeconfig create -f - <<EOF
apiVersion: argoproj.io/v1alpha1
kind: Workflow
metadata:
  generateName: domain-check-release-manual-
  namespace: argo-workflows
spec:
  entrypoint: release
  workflowTemplateRef:
    name: domain-check-build
  arguments:
    parameters:
      - name: tag
        value: "v0.1.0"
EOF
```

## EventSource Configuration

The `domain-check` entry in `github-eventsource.yml` (declarative-config) configures a GitHub webhook:

| Field | Value |
|-------|-------|
| **EventSource** | `github-webhooks` |
| **Event name** | `domain-check` |
| **Endpoint** | `/domain-check` |
| **Events** | `push` |
| **Webhook URL** | `https://webhooks-ci.ardenone.com/domain-check` |

This single webhook endpoint receives all push events (both branch and tag pushes). The sensor applies filters to route them to the correct entrypoint.

## Sensor Configuration

The `domain-check-sensor` (in `k8s/iad-ci/argo-events/domain-check-sensor.yml`) has:

- **One dependency:** `domain-check-push` from `github-webhooks` / `domain-check` (filters for `push` events)
- **Trigger 1 (build):** Additional filter `body.ref == refs/heads/main` + exclude CI auto-bump commits → submits workflow with default `build` entrypoint
- **Trigger 2 (release):** Additional filter `body.ref =~ ^refs/tags/v[0-9]+\.[0-9]+\.[0-9]+` → submits workflow with `release` entrypoint, passing the tag (stripped of `refs/tags/` prefix) as a parameter

### Tag Push Parameter Extraction

For tag pushes, the sensor extracts:
- `tag`: `refs/tags/v0.1.0` → `v0.1.0` (strips `refs/tags/` prefix)
- Labels: `triggered-by: github-webhook`, `tag: refs/tags/v0.1.0`

### Tag Format

Only semver tags matching `vX.Y.Z` (where X, Y, Z are digits) trigger the release pipeline. Pre-release tags like `v1.0.0-rc.1` are NOT matched by this regex — they would need a separate trigger or regex adjustment if pre-release automation is desired.

## Manual Release Workflow Test (2026-07-02)

### Test: Entrypoint Routing Verification

Submitted a manual workflow with `entrypoint: release` and `tag: v0.0.0-test` to verify that the release entrypoint routes to `quality-gate → goreleaser-release` steps (not the build pipeline steps).

**Workflow runs submitted:**

| Run ID | Generated Name | Result |
|--------|---------------|--------|
| 1 | `domain-check-release-test-ns9ml` | Failed — quality-gate exit code 127 |
| 2 | `domain-check-release-test-9n6bq` | Failed — quality-gate exit code 127 (with podGC: OnWorkflowCompletion) |
| 3 | `domain-check-release-test-mk8bg` | Failed — quality-gate exit code 127 |

**Entrypoint routing: CONFIRMED ✓**

All release workflows ran the `quality-gate` template (release variant, clones by tag), not `build-quality-gate` (build variant, clones by branch). This proves the `entrypoint: release` routing works correctly — the workflow correctly selected the release pipeline steps.

**goreleaser step: NOT REACHED**

The goreleaser-release step was never reached because quality-gate failed first. This was NOT due to the missing tag (`v0.0.0-test` doesn't exist on remote) — the failure occurred before `git clone` could even be attempted.

### Pre-existing CI Bug: `golang:1.26-alpine` Missing `git`

Both `quality-gate` and `build-quality-gate` templates use `image: golang:1.26-alpine` and run `git clone` as their first command. The exit code 127 (command not found) indicates `git` is not installed in the `golang:1.26-alpine` image.

**Affected templates:** `build-quality-gate`, `quality-gate`, `goreleaser-release` (all three use `git clone`)

**Fix required:** Either:
1. Switch image to `golang:1.26-alpine` with `apk add --no-cache git` (install git at runtime)
2. Switch to a non-Alpine golang image that includes git (e.g., `golang:1.26`)
3. Use `alpine/git` for git operations and a separate golang image for build/test

This bug affects ALL domain-check CI workflows — not just the release pipeline.

### Submit Command Used

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
