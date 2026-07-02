# Domain Check — Audit Summary

**Date:** 2026-07-02
**Scope:** Full project audit — implementation completeness, CI/CD integrity, code quality, and deployment readiness.

## Current State

The project is **feature-complete and building cleanly** (`go build ./...` passes, 37 Go source files). All five planned phases have working implementations:

| Phase | Status | Key Deliverables |
|-------|--------|-----------------|
| 1 — Core Engine | ✅ Complete | RDAP client, bootstrap loader, WHOIS fallback, domain parsing/validation, cache, SSRF protection, per-registry rate limiting |
| 2 — API Server | ✅ Complete | `/api/v1/check` (single + multi-TLD), `/api/v1/bulk`, `/api/v1/tlds`, `/health`, `/metrics`; rate limiting, CORS, security headers, request ID middleware |
| 3 — Web UI | ✅ Complete | `go:embed` templates + static assets, server-rendered HTML, progressive enhancement JS, mobile-first CSS, shareable result URLs |
| 4 — CLI + Optimization | ✅ Complete | `domain-check check` (single + multi-TLD), `domain-check bulk` (file input, JSON/CSV/text output), DNS pre-filter, parallel execution |
| 5 — Deployment + CI | ⚠️ Partial | Dockerfile exists and works; CI/CD has integration gaps (see below) |

### Test Coverage

- **16 test files**, ~11,700 lines of test code across all packages
- **1,500+ testdata files** — recorded RDAP fixtures from 8+ registries, WHOIS fixtures, fuzz corpora
- **Fuzz targets:** `FuzzValidateDomain`, `FuzzParseRDAPResponse`
- **E2E tests:** 2 Playwright smoke tests in `tests/e2e/`

### Research & Documentation

- **13 research documents** covering RDAP protocol, rate limits, accuracy comparison, ccTLD support, edge cases, Go patterns, GoReleaser, CI
- **10 benchmark documents** with performance baselines
- **Comprehensive README** (10 KB) with usage examples for all interfaces

## What's Broken

### Critical (builds produce incorrect output)

1. **C1 — Missing version injection variables.** GoReleaser injects `-X main.version/commit/date` via ldflags, but no `var version string` etc. are declared in `main.go`. Running `--version` won't work. Both GoReleaser and Docker builds are affected. (Tracked: part of bf-5vp)

2. **C2 — Dockerfile ignores VERSION build arg.** CI passes `--build-arg=VERSION=...` but the Dockerfile has no `ARG VERSION` declaration. The binary ships with no version info. (Tracked: part of bf-5vp)

### High Priority (CI guarantees not enforced)

3. **H1 — No test/lint/fuzz step in CI.** The Argo WorkflowTemplate goes straight from `resolve-version` → `docker-build`. Bugs, failing tests, or lint issues can ship to `ronaldraygun/domain-check:latest`. (Tracked: bf-3g3)

4. **H2 — plan.md describes GitHub Actions CI that doesn't exist.** The plan shows a full GH Actions YAML with matrix Go versions and codecov, but actual CI runs on Argo Workflows per project policy. Misleading for contributors. (Tracked: part of bf-3g3 scope)

5. **Go toolchain version drift.** Dockerfile uses `golang:1.22-alpine`, go.mod declares `go 1.26.1`, README references "Go 1.23+". These should align. (Tracked: bf-i0l)

## What's Missing

### Deployment & CI Gaps

| Item | Impact | Tracked By |
|------|--------|-----------|
| ~~No goreleaser step in CI WorkflowTemplate~~ ✅ Resolved | Docker and GitHub releases are now coordinated in one pipeline | — |
| VERSION file vs git tag dual source of truth | Docker image tag and GitHub release tag can diverge | bf-5oq |
| No branch protection for `:latest` Docker tag | Non-main branch builds could overwrite `:latest` with unstable code | Untracked |
| Kaniko + alpine/git images use `:latest` | Violates infra pinning policy; could break on upstream change | Untracked |
| GoReleaser `before.hooks` skip tests/vet | Releases can ship with failing tests | bf-3g3 |
| ~~GITHUB_TOKEN not configured for goreleaser in CI~~ ✅ Resolved | Both goreleaser templates wire GITHUB_TOKEN via secretKeyRef | — |

### Feature Gaps (vs plan.md)

| Item | Status | Tracked By |
|------|--------|-----------|
| PSL private-suffix rejection (e.g., `example.github.io`) | Plan specifies rejection; not yet implemented | bf-4nk |
| Playwright E2E smoke tests for WHOIS ccTLD path | Only web UI E2E exists | bf-62g |
| Load testing verification against plan targets (p99 < 2s, error < 1%) | Benchmarks exist but not verified against plan spec | bf-48n |
| Copy-to-clipboard Playwright test | Progressive enhancement feature untested in E2E | bf-5sz |

### Repo Hygiene

| Item | Tracked By |
|------|-----------|
| Stale `cluster-configuration/` directory (pre-ArgoCD artifact) | bf-5re |
| Dead fuzz corpora in top-level `testdata/fuzz/` (duplicates of `internal/checker/testdata/fuzz/`) | bf-3l4 |
| Committed artifacts that should be gitignored | bf-3l4 |

## Recommended Next Steps

### Immediate (unblock CI integrity)

1. **Fix C1 + C2** — Add `var (version, commit, date string)` to `main.go`, wire them into `--version` flag and UserAgent strings. Add `ARG VERSION=dev` to Dockerfile and pass it through ldflags. (Bead: bf-5vp)

2. **Add CI quality gate** — Insert a test/lint step between `resolve-version` and `docker-build` in the Argo WorkflowTemplate. Run `go vet ./...`, `golangci-lint run`, `go test -race ./...`. Abort Docker build on failure. (Bead: bf-3g3)

### Short-Term (version and toolchain alignment)

3. **Align Go toolchain versions** — Update Dockerfile to `golang:1.26-alpine` (matching go.mod), update README accordingly. (Bead: bf-i0l)

4. **Unify versioning** — Decide on single source of truth (recommend: git tags, derive version in CI via `git describe`). Replace VERSION file with tag-based versioning. Add goreleaser as a CI step to produce both Docker image + GitHub release from one pipeline. (Beads: bf-5oq, bf-5vp)

5. **Pin CI images** — Pin Kaniko and alpine/git to specific version tags in the WorkflowTemplate. (Small, no bead needed)

### Medium-Term (hardening and coverage)

6. **Reject PSL private-suffix domains** — Implement the plan.md rule: return error for domains under private suffixes (e.g., `example.github.io`, `example.pages.dev`). (Bead: bf-4nk)

7. **Add WHOIS E2E test** — Playwright smoke test verifying the WHOIS ccTLD fallback path works end-to-end. (Bead: bf-62g)

8. **Load test verification** — Run vegeta/hey against a deployed instance and verify results meet plan targets (p99 < 2s uncached, < 10ms cached, error rate < 1%). (Bead: bf-48n)

9. **Repo cleanup** — Remove `cluster-configuration/`, deduplicate fuzz corpora, add gitignore rules for build artifacts. (Beads: bf-5re, bf-3l4)

### Low-Priority (polish)

10. **Update plan.md CI section** — Replace GitHub Actions YAML with actual Argo Workflows pipeline description to prevent contributor confusion.

11. **Branch-protect `:latest`** — Only push `ronaldraygun/domain-check:latest` when building from `main`.

12. **Kaniko artifact passing** — Pass built context between steps instead of having Kaniko clone independently (eliminates race condition).

## Cross-References

- Integration gap detail: [docs/notes/11-integration-gap-analysis.md](../notes/11-integration-gap-analysis.md)
- Full architecture plan: [docs/plan/plan.md](../plan/plan.md)
- Go implementation patterns: [docs/research/08-go-implementation-patterns.md](08-go-implementation-patterns.md)
