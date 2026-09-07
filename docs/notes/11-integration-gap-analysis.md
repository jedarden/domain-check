# Integration Gap Analysis: GoReleaser vs CI WorkflowTemplate

Cross-references GoReleaser output (`.goreleaser.yml`) against CI expectations (`domain-check-build` WorkflowTemplate) and the plan (`docs/plan/plan.md`).

**Last updated:** 2026-07-02

## Critical Gaps (builds fail or produce incorrect output)

### C1. Missing `main.version`/`main.commit`/`main.date` variables in main.go

- **GoReleaser injects:** `-X main.version={{.Version}}`, `-X main.commit={{.Commit}}`, `-X main.date={{.Date}}` via ldflags
- **Actual:** No `var version`, `var commit`, or `var date` declared anywhere in `cmd/domain-check/main.go` (or any Go file in the repo)
- **Impact:** GoReleaser builds will either fail at link time (undefined symbol) or silently produce binaries with no version info. Running `--version` won't show build metadata. Both GoReleaser and Docker builds are affected.
- **Fix:** Add `var (version, commit, date string)` to `main.go`, use them in a `--version` flag handler, and in the hardcoded `UserAgent: "domain-check/1.0"` strings.

### C2. Dockerfile ignores the VERSION build arg

- **CI passes:** `--build-arg=VERSION={{inputs.parameters.version}}` to Kaniko (line 108 of WorkflowTemplate)
- **Dockerfile:** No `ARG VERSION` declaration, no `-X main.version=${VERSION}` in the `go build` ldflags (line 20-24 of Dockerfile)
- **Impact:** Docker image binary has no version info at all. The `--build-arg` is silently ignored.
- **Fix:** Add `ARG VERSION=dev` to Dockerfile, then include `-X main.version=${VERSION}` in ldflags. Also requires C1 to be fixed first.

## High-Priority Gaps (broken CI guarantees)

### H1. No test, lint, or fuzz step in CI pipeline

- **plan.md expects:** `golangci-lint` → `go test -race` → fuzz (30s) → `go build`
- **CI actually does:** `resolve-version` → `docker-build` (no quality gates)
- **GoReleaser before.hooks:** `go mod tidy` + `go generate ./...` (no tests or lint)
- **Impact:** Bugs, failing tests, or lint issues can ship to `ronaldraygun/domain-check:latest`. GoReleaser will release binaries to GitHub even if tests fail. Other Go CI templates in this infra (e.g., `agentscribe-ci`) run full checks before releasing.
- **Fix:** Add a `test` step between `resolve-version` and `docker-build` that runs `golangci-lint` and `go test -race ./...`. Also add `go vet ./...` to goreleaser's `before.hooks` (or better: make goreleaser part of the CI pipeline so it runs after tests pass).

### H2. plan.md references GitHub Actions, not Argo Workflows

- **plan.md (line 996-1014):** Shows a full GitHub Actions YAML workflow with matrix Go versions, codecov, etc.
- **Actual CI:** Argo Workflows on iad-ci (per CLAUDE.md: "GitHub Actions are disabled")
- **Impact:** Misleading for anyone reading the plan. The plan describes a CI pipeline that doesn't exist and violates the infra rule.
- **Fix:** Replace the plan.md CI section with the actual Argo Workflows pipeline description.

## Medium-Priority Gaps (version confusion, missing integration)

### M1. VERSION file vs git tag — dual source of truth for version

- **CI uses:** `VERSION` file in repo root (auto-bumps patch on every run)
- **GoReleaser uses:** git tags (e.g., `v1.2.3`)
- **Impact:** Docker image tag and GitHub release tag can diverge. E.g., Docker Hub has `1.5.3` but GitHub release is `v1.5.2`. No single action produces both artifacts with matching versions.
- **Fix:** Either (a) have CI create a git tag from the VERSION file and trigger goreleaser from that tag, or (b) replace VERSION file with tag-based versioning and have CI derive the version from `git describe`, or (c) add goreleaser as a CI step so both artifacts are produced in one pipeline.

### M2. ~~No goreleaser step in CI WorkflowTemplate~~ RESOLVED

- **CI currently:** Resolves version → builds Docker image → **runs goreleaser** (build pipeline). Release pipeline runs quality gate → goreleaser release.
- **Resolution:** Commit `362233f` in declarative-config added `goreleaser` container step (using `goreleaser/goreleaser:v2`) to the build pipeline and `goreleaser-release` step to the release pipeline. Both use `goreleaser release --clean`.

### M3. ~~GITHUB_TOKEN not configured for goreleaser use in CI~~ RESOLVED

- **Resolution:** Both the `goreleaser` and `goreleaser-release` templates now set `GITHUB_TOKEN` and `GH_TOKEN` env vars via `secretKeyRef` to `github-webhook-secret` (key: `token`). The secret exists in the `argo-workflows` namespace on iad-ci. Argo Workflows automatically masks `secretKeyRef` values from logs and the UI.

### M4. GoReleaser before.hooks don't validate code quality

- **Current hooks:** `go mod tidy` + `go generate ./...`
- **Missing:** `go vet ./...`, `go test ./...`
- **Impact:** GoReleaser will build and release binaries even if tests fail, as long as the code compiles.
- **Fix:** Either add test commands to `before.hooks` (goreleaser will abort if they fail) or rely on CI test gates running before goreleaser is invoked.

### M5. No branch protection for `:latest` Docker tag

- **CI always pushes:** `ronaldraygun/domain-check:latest` regardless of branch parameter
- **Impact:** If the workflow is triggered on a non-main branch, `:latest` gets overwritten with potentially unstable code.
- **Fix:** Only push `:latest` when `branch == "main"`. For other branches, only push the version tag.

## Low-Priority Gaps (policy violations, minor improvements)

### L1. Kaniko image uses `:latest` tag

- **WorkflowTemplate line 102:** `image: gcr.io/kaniko-project/executor:latest`
- **CLAUDE.md rule:** "Never use `:latest` image tags — always pin to a specific digest or version tag"
- **Impact:** Could break if the Kaniko image changes unexpectedly.
- **Fix:** Pin to a specific version, e.g., `gcr.io/kaniko-project/executor:v1.23.0` or a digest.

### L2. `alpine/git` image uses `:latest` tag (implicit)

- **WorkflowTemplate line 39:** `image: alpine/git`
- **CLAUDE.md rule:** Same as L1.
- **Impact:** Lower risk than Kaniko, but violates the pinning policy.
- **Fix:** Pin to specific version, e.g., `alpine/git:v2.45.0`.

### L3. Hardcoded UserAgent string

- **main.go line 101:** `UserAgent: "domain-check/1.0"`
- **main.go line 422:** `UserAgent: "domain-check/1.0"`
- **main.go line 458:** `UserAgent: "domain-check/1.0"`
- **main.go line 464:** `UserAgent: "domain-check/1.0"`
- **Should use:** The version variable (after C1 is fixed), e.g., `fmt.Sprintf("domain-check/%s", version)`
- **Impact:** UserAgent in RDAP/WHOIS requests always reports `1.0` regardless of actual version.

### L4. Kaniko clones repo independently — potential race condition

- **Step 1:** Clones repo, bumps VERSION, pushes to GitHub
- **Step 2 (Kaniko):** Clones from GitHub independently via `--context=git://github.com/...`
- **Race:** If push from step 1 hasn't propagated to GitHub's API by the time Kaniko clones, step 2 builds from stale code
- **Mitigation:** VERSION is passed as build arg (partially mitigates), but the Dockerfile doesn't use it yet
- **Impact:** Unlikely in practice (GitHub propagates fast), but theoretically possible.

### L5. No `--snapshot-mode=redo` for Kaniko

- **Kaniko defaults:** Full snapshot mode (hashes all files)
- **Optimization:** `--snapshot-mode=redo` uses git diff for more efficient layer caching in Go projects
- **Impact:** Slower builds than necessary. Not a gap, just an optimization opportunity.

## Summary: Prioritized Fix Order

| Priority | ID | Description | Effort |
|----------|----|-------------|--------|
| **CRITICAL** | C1 | Add `var version/commit/date` to main.go | Small |
| **CRITICAL** | C2 | Dockerfile: accept + use VERSION build arg | Small |
| **HIGH** | H1 | Add test/lint/fuzz step to CI pipeline | Medium |
| **HIGH** | H2 | Update plan.md CI section to reflect Argo (not GH Actions) | Small |
| **MEDIUM** | M1 | Unify version source (VERSION file vs git tag) | Medium |
| **MEDIUM** | M2 | ~~Add goreleaser step to CI WorkflowTemplate~~ ✅ Resolved | — |
| **MEDIUM** | M3 | ~~Configure GITHUB_TOKEN for goreleaser in CI~~ ✅ Resolved | — |
| **MEDIUM** | M4 | Add `go vet`/`go test` to goreleaser before.hooks | Small |
| **MEDIUM** | M5 | Only push `:latest` for main branch builds | Small |
| **LOW** | L1 | Pin Kaniko image to specific version | Trivial |
| **LOW** | L2 | Pin alpine/git image to specific version | Trivial |
| **LOW** | L3 | Use version var in UserAgent strings | Small |
| **LOW** | L4 | Address Kaniko race condition (artifact passing) | Small |
| **LOW** | L5 | Add `--snapshot-mode=redo` to Kaniko | Trivial |

## Existing Beads Tracking Related Work

| Bead | Topic | Status |
|------|-------|--------|
| `bf-3g3` | Add CI gate for tests/lint/fuzz before Docker build | Open |
| `bf-5oq` | Add goreleaser step to Argo WorkflowTemplate | Open |
| `bf-2ru` | Verify goreleaser release pipeline wired to Argo CI | Open |
