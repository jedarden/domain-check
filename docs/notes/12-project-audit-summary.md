# Project Audit: GoReleaser + CI Pipeline

**Last updated:** 2026-07-02

## Scope

Audited three components and their integration:
1. `.goreleaser.yml` — multi-platform binary release configuration
2. `domain-check-build` WorkflowTemplate — Argo Workflows CI pipeline
3. `Dockerfile` — container build for Docker Hub

## Current State

### .goreleaser.yml

Fully configured for multi-platform GitHub releases:

| Aspect | Status |
|--------|--------|
| Build targets | 4 OSes × 3 arches = 9 binaries (linux/darwin/windows/freebsd) |
| Static builds | `CGO_ENABLED=0`, ldflags strip (`-s -w`) |
| Version injection | ldflags: `-X main.version/commit/date` |
| Archives | tar.gz (unix), zip (windows), includes LICENSE + README |
| Checksums | `checksums.txt` with SHA256 hashes |
| Changelog | Auto-generated, excludes `docs:`/`test:`/`ci:`/`chore:`/`build:` prefixes |
| Release mode | `replace` (overwrites existing), auto-detects prereleases |

**Pre-build hooks:** `go mod tidy` + `go generate ./...` (no test/lint gate).

### domain-check-build WorkflowTemplate

Two-step Argo Workflow on `iad-ci`:

| Step | Tool | Purpose |
|------|------|---------|
| `resolve-version` | `alpine/git` | Read/auto-bump `VERSION` file, push to repo |
| `docker-build` | Kaniko | Build Docker image → `ronaldraygun/domain-check:<version>` + `:latest` |

**Output:** Docker image on Docker Hub only. No binary releases, no tests, no lint.

### Dockerfile

Multi-stage build: `golang:1.22-alpine` (builder) → `alpine:3.19` (runtime). Non-root user, health check, static binary. Does NOT accept or use the `VERSION` build arg passed by CI.

## Findings (prioritized)

### Critical — builds produce incorrect output

| ID | Issue | Location |
|----|-------|----------|
| C1 | No `var version/commit/date` declared in Go code — GoReleaser ldflags target nonexistent symbols | `cmd/domain-check/main.go` |
| C2 | Dockerfile ignores `--build-arg=VERSION` — binary in Docker has no version info | `Dockerfile` lines 20-24 |

### High — broken CI guarantees

| ID | Issue | Location |
|----|-------|----------|
| H1 | No test/lint/fuzz step in CI — untested code can ship to `:latest` | WorkflowTemplate |
| H2 | plan.md describes GitHub Actions CI (disabled), not the actual Argo pipeline | `docs/plan/plan.md` |

### Medium — missing integration

| ID | Issue |
|----|-------|
| M1 | VERSION file (CI) vs git tags (GoReleaser) — dual source of truth for version |
| M2 | No GoReleaser step in CI — binary releases require manual execution |
| M3 | `GITHUB_TOKEN` env not configured for GoReleaser in CI context |
| M4 | GoReleaser `before.hooks` skip `go vet`/`go test` |
| M5 | `:latest` Docker tag pushed regardless of branch |

### Low — policy violations and optimizations

| ID | Issue |
|----|-------|
| L1 | Kaniko uses `:latest` tag — violates CLAUDE.md pinning rule |
| L2 | `alpine/git` uses implicit `:latest` tag — same violation |
| L3 | Hardcoded `UserAgent: "domain-check/1.0"` in 4 places — should use version var |
| L4 | Kaniko clones repo independently — theoretical race with VERSION push |
| L5 | No `--snapshot-mode=redo` for faster Kaniko layer caching |

## What Needs to Be Added/Fixed

### Must-fix (before first production release)

1. **Add version variables to main.go** — declare `var (version, commit, date string)`, wire to `--version` flag and UserAgent strings
2. **Dockerfile: accept VERSION build arg** — `ARG VERSION=dev` + `-X main.version=${VERSION}` in ldflags
3. **Add CI quality gate** — `golangci-lint run` + `go test -race ./...` step between version resolution and Docker build
4. **Pin CI images** — Kaniko and alpine/git to specific version tags

### Should-add (for complete CI/CD)

5. **Unify versioning** — choose VERSION file or git tags as single source; derive the other from it
6. **Add GoReleaser to CI** — conditional step (on tag or after Docker build) to produce binary releases alongside Docker image
7. **Protect `:latest`** — only push on main branch builds
8. **Update plan.md** — replace GitHub Actions CI section with actual Argo Workflows description

### Nice-to-have

9. Kaniko `--snapshot-mode=redo` for faster caching
10. Pass git artifacts between steps instead of re-cloning

## Detailed Analysis

- [GoReleaser configuration](09-goreleaser-configuration.md) — full breakdown of `.goreleaser.yml`
- [CI WorkflowTemplate structure](10-ci-workflowtemplate.md) — full breakdown of the Argo pipeline
- [Integration gap analysis](11-integration-gap-analysis.md) — cross-referenced findings with fix suggestions

## Related Open Beads

| Bead | Topic | Status |
|------|-------|--------|
| `bf-3g3` | Add CI gate for tests/lint/fuzz before Docker build | Open |
| `bf-5oq` | Add goreleaser step to Argo WorkflowTemplate | Open |
| `bf-2ru` | Verify goreleaser release pipeline wired to Argo CI | Open |
