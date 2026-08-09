# GoReleaser Full Pipeline Summary

Concise overview of the entire `.goreleaser.yml` pipeline — how all sections connect from pre-build hooks through GitHub Release publication.

Detailed per-section docs: [09-builds](09-goreleaser-builds.md) → [10-archives/checksum/changelog](10-goreleaser-archives-checksum-changelog.md) → [11-release](11-goreleaser-release.md).

## Pipeline Flow

```
Tag push (e.g. git tag v1.2.3 && git push --tags)
  │
  ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  1. Before Hooks                                                            │
│     go mod tidy         → ensure go.mod/go.sum are clean                    │
│     go generate ./...   → regenerate all //go:generate directives          │
└───────────────────────────────┬───────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  2. Builds                                                                  │
│     9 binaries across 4 OSes × 3 architectures (3 combos excluded)           │
│     • CGO_ENABLED=0 (fully static, zero runtime deps)                       │
│     • -s -w (strip symbols, ~25-30% size reduction)                        │
│     • -X main.version/commit/date (version injection from git tag/SHA)     │
│     Matrix: linux/{amd64,arm64,armv7}, darwin/{amd64,arm64},               │
│             windows/amd64, freebsd/{amd64,arm64,armv7}                     │
└───────────────────────────────┬───────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  3. Archives                                                                 │
│     9 archives: tar.gz (linux, darwin, freebsd) + zip (windows)             │
│     Each contains: binary + LICENSE + README.md                             │
│     Naming: domain-check_{Os}_{Arch} (e.g. domain-check_Linux_x86_64)      │
└───────────────────────────────┬───────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  4. Checksum                                                                 │
│     Single checksums.txt with SHA256 digests for all 9 archives             │
│     Users verify with: sha256sum -c checksums.txt --ignore-missing         │
└───────────────────────────────┬───────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  5. Changelog                                                                │
│     Generated from git commits since last tag, sorted ascending             │
│     Excludes: docs:, test:, ci:, chore:, build: (non-user-facing changes)   │
│     Keeps: feat:, fix:, perf:, breaking changes                             │
└───────────────────────────────┬───────────────────────────────────────────┘
                                │
                                ▼
┌─────────────────────────────────────────────────────────────────────────────┐
│  6. GitHub Release                                                            │
│     Published immediately (draft: false)                                     │
│     Prerelease flag inferred from tag (auto): v1.2.3 → stable,              │
│       v1.2.3-rc.1 → prerelease                                              │
│     Mode: replace — deletes existing release, creates clean one            │
│     Release contains: 9 archives + checksums.txt + changelog                │
└─────────────────────────────────────────────────────────────────────────────┘
```

## Release Artifacts per Tag

A single `git tag v1.2.3` produces:

| # | Artifact                                | Format  |
|---|-----------------------------------------|---------|
| 1 | `domain-check_Linux_x86_64`            | tar.gz  |
| 2 | `domain-check_Linux_arm64`             | tar.gz  |
| 3 | `domain-check_Linux_armv7`              | tar.gz  |
| 4 | `domain-check_Darwin_x86_64`           | tar.gz  |
| 5 | `domain-check_Darwin_arm64`            | tar.gz  |
| 6 | `domain-check_Windows_x86_64`          | zip     |
| 7 | `domain-check_FreeBSD_x86_64`          | tar.gz  |
| 8 | `domain-check_FreeBSD_arm64`           | tar.gz  |
| 9 | `domain-check_FreeBSD_armv7`           | tar.gz  |
| 10| `checksums.txt`                         | text    |
| + | Changelog body (embedded in release)    | —       |

## Key Design Decisions

| Decision                     | Rationale                                                                  |
|------------------------------|-----------------------------------------------------------------------------|
| `CGO_ENABLED=0`              | Static binary, no runtime deps, reliable cross-compilation                  |
| `-s -w` ldflags              | ~25-30% binary size reduction; debug info not needed in releases           |
| `-X main.*` version vars    | `--version` reports exact git tag, commit SHA, and build timestamp        |
| `tar.gz` for Unix, `zip` for Windows | Platform-native archive formats                                          |
| Ascending changelog sort     | Chronological reading order, conventional for changelogs                    |
| Exclude non-user prefixes    | Keep changelog focused on features/fixes, not housekeeping                  |
| `prerelease: auto`          | Tag convention (`-rc.N`, `-beta.N`) controls release visibility            |
| `mode: replace`             | Clean slate on re-run — no stale artifacts from previous attempts            |
| Before hooks (`tidy`+`generate`) | Build-time correctness gates — fail fast if deps or generated code are stale |
