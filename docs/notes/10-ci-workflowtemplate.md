# CI WorkflowTemplate: domain-check-build

Documents the Argo Workflows WorkflowTemplate that handles Docker image builds for domain-check. The template lives in `declarative-config` at `k8s/iad-ci/argo-workflows/domain-check-workflowtemplate.yml` and is synced to the `iad-ci` cluster via ArgoCD.

## Template Overview

| Field | Value |
|-------|-------|
| **Name** | `domain-check-build` |
| **Namespace** | `argo-workflows` |
| **ServiceAccount** | `argo-workflow` |
| **Entrypoint** | `build` (steps-based DAG) |
| **Source repo** | `github.com/jedarden/domain-check` (parameterized, default: `main` branch) |

## Step Pipeline

The template runs two sequential steps:

```
build (entrypoint)
├── Step 1: resolve-version    — Auto-bump or read VERSION file
└── Step 2: docker-build      — Kaniko Docker build → Docker Hub
```

### Step 1: resolve-version

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

### Step 2: docker-build

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

## Secrets Required

| Secret Name | Key | Used By | Purpose |
|-------------|-----|---------|---------|
| `github-webhook-secret` | `token` | `resolve-version` (as `GH_TOKEN`), `docker-build` (as `GIT_TOKEN`) | GitHub API access for git clone/push and Kaniko git context |
| `docker-hub-registry` | `.dockerconfigjson` | `docker-build` (volume mount) | Docker Hub push authentication |

Both secrets must exist in the `argo-workflows` namespace on the `iad-ci` cluster.

## Artifacts CI Produces

The CI pipeline produces exactly **one artifact**: a Docker image pushed to Docker Hub.

| Tag | Example | Purpose |
|-----|---------|---------|
| `<version>` | `ronaldraygun/domain-check:0.1.5` | Immutable, version-pinned deployment target |
| `latest` | `ronaldraygun/domain-check:latest` | Rolling tag for convenience |

The image is built from the repo's `Dockerfile` (multi-stage: `golang:1.22-alpine` builder → `alpine:3.19` runtime). The `VERSION` build arg is available but the current Dockerfile does not use it — it only uses the `CGO_ENABLED=0` static build with ldflags.

## Artifacts CI Does NOT Produce

The CI pipeline does **not** run GoReleaser. GoReleaser is a separate release tool for GitHub binary distributions. The CI pipeline and GoReleaser have distinct responsibilities:

| Concern | CI WorkflowTemplate | GoReleaser |
|---------|-------------------|------------|
| Docker image build | Yes | No |
| Binary releases (tar.gz/zip) | No | Yes (9 platform binaries) |
| Checksums file | No | Yes (`checksums.txt`) |
| Changelog generation | No | Yes (from git commits) |
| GitHub Release creation | No | Yes |
| Version bumping | Yes (auto-bump patch) | No (reads tag) |
| Docker Hub push | Yes (`ronaldraygun`) | No |

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
