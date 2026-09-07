# GoReleaser Configuration

Documents the `.goreleaser.yml` configuration used for multi-platform binary releases on GitHub.

## Top-Level Sections

### `project_name: domain-check`

Sets the project name used throughout GoReleaser templates (archive naming, changelog headers, etc.). Defaults to the git repo name if omitted, but is set explicitly here for clarity.

### `before.hooks`

Commands run **before any builds start**. Hooks run sequentially and must all succeed (non-zero exit aborts the release).

| Hook | Purpose |
|------|---------|
| `go mod tidy` | Ensures `go.mod` and `go.sum` are consistent — removes unused deps, adds missing ones. Prevents a dirty tree from causing build failures. |
| `go generate ./...` | Runs all `//go:generate` directives in the project. Currently a no-op (no generate directives), but future-proofs for embedded asset generation or code generation. |

### `builds`

Defines how the Go binary is compiled. Single build configuration that produces binaries for multiple OS/arch combinations.

**Target platforms:**

| OS | Arch | Notes |
|----|------|-------|
| linux | amd64, arm64, arm (v7) | All supported |
| darwin (macOS) | amd64, arm64 | arm excluded (Apple Silicon and Intel Macs only) |
| windows | amd64 | arm64 and arm excluded |
| freebsd | amd64, arm64, arm (v7) | All supported |

**Build settings:**

- **`CGO_ENABLED=0`** — produces fully static binaries with no C dependencies, maximizing portability
- **`ldflags`** — strips debug info (`-s -w` for ~30% smaller binaries) and injects version metadata via `main.version`, `main.commit`, `main.date` (populated from git tag/commit)
- **`main: ./cmd/domain-check`** — entry point at `cmd/domain-check/main.go`

### `archives`

Configures how built binaries are packaged for distribution.

- **Format:** `tar.gz` by default, `zip` for Windows (Windows users expect zip)
- **Naming:** human-readable OS/arch names (e.g., `domain-check_Linux_x86_64`, `domain-check_Darwin_arm64`, `domain-check_Windows_x86_64`)
- **Included files:** `LICENSE` and `README.md` are bundled alongside the binary in each archive

### `checksum`

Produces a `checksums.txt` file containing SHA256 hashes of all archives. Users verify download integrity by comparing against this file.

### `changelog`

Auto-generated from git commit messages since the last tag.

- **Sort:** `asc` — oldest commits first (natural reading order)
- **Excluded prefixes:** `docs:`, `test:`, `ci:`, `chore:`, `build:` — these are housekeeping commits that don't represent user-facing changes. Only feature, fix, and perf commits appear in the published changelog.

### `release`

Controls the GitHub Release created by GoReleaser.

| Setting | Value | Meaning |
|---------|-------|---------|
| `draft` | `false` | Release is published immediately, not saved as a draft |
| `prerelease` | `auto` | GoReleaser infers from the version tag: `-rc`, `-beta`, `-alpha` suffixes → prerelease; clean semver → stable release |
| `mode` | `replace` | If a release for this tag already exists, GoReleaser replaces it rather than skipping or erroring |

## Build Matrix Summary

The combination of 4 OSes × 3 architectures × exclusions yields **10 binaries**:

1. `linux/amd64`
2. `linux/arm64`
3. `linux/arm` (v7)
4. `darwin/amd64`
5. `darwin/arm64`
6. `windows/amd64`
7. `freebsd/amd64`
8. `freebsd/arm64`
9. `freebsd/arm` (v7)

Plus checksums.txt and auto-generated changelog — all attached to the GitHub Release.
