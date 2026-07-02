# GoReleaser Release and Before Hooks Configuration

Documents the `before` and `release` sections of `.goreleaser.yml` — pre-build hooks and GitHub release behavior.

See [09-goreleaser-builds.md](09-goreleaser-builds.md) for builds, [10-goreleaser-archives-checksum-changelog.md](10-goreleaser-archives-checksum-changelog.md) for archives/checksum/changelog.

## Before Hooks

```
before:
  hooks:
    - go mod tidy
    - go generate ./...
```

The `before` section runs commands **before every build**, sequentially, in the project root directory. If any hook exits non-zero, the entire release is aborted. These hooks enforce build-time invariants:

| Hook                | Purpose                                                                                                        |
|---------------------|----------------------------------------------------------------------------------------------------------------|
| `go mod tidy`       | Syncs `go.mod` and `go.sum` — ensures no stray dependencies are listed and all required ones are recorded     |
| `go generate ./...`| Runs all `//go:generate` directives across the project — regenerates embedded assets, parsers, or stubs as needed |

Together they guarantee the build starts from a clean dependency state and all generated code is up to date.

## Release

```
release:
  draft: false
  prerelease: auto
  mode: replace
```

Controls how GoReleaser creates (or updates) the GitHub Release that holds the built artifacts.

### `draft: false`

The release is published **immediately** as a public, non-draft GitHub Release. If set to `true`, GoReleaser would create a draft release that requires manual promotion from the GitHub web UI — useful for review workflows but unnecessary for an automated pipeline.

### `prerelease: auto`

GoReleaser **infers** the prerelease flag from the git tag:

| Tag Pattern              | GitHub Release Prerelease Flag | Example                    |
|--------------------------|-------------------------------|----------------------------|
| `v1.2.3`                 | **false** — stable release    | Final release              |
| `v1.2.3-rc.1`           | **true** — prerelease        | Release candidate          |
| `v1.2.3-beta.4`         | **true** — prerelease        | Beta                       |
| `v1.2.3-alpha.1`        | **true** — prerelease        | Alpha                      |

Any tag containing a hyphen after the semver core (`-rc.N`, `-beta.N`, `-alpha.N`, etc.) is marked as a prerelease. Clean semver tags are stable. This eliminates manual toggling — the tag convention determines the release visibility.

### `mode: replace`

Controls how GoReleaser handles an **existing** GitHub Release with the same tag:

| Mode       | Behavior                                                                                                |
|------------|---------------------------------------------------------------------------------------------------------|
| `replace`  | **Delete** the existing release and all its assets, then create a new one from scratch                  |
| `append`   | Keep the existing release, add new assets alongside existing ones                                         |
| `update`   | Keep the existing release, replace assets whose filenames match, leave non-matching assets untouched     |

`replace` ensures a clean slate on every release run — stale binaries from a previous attempt are never left behind. This is the safest default for a single-maintainer project where re-running a release is a deliberate action (not a concurrent publish). The trade-off: if a release is replaced while users are actively downloading, in-flight downloads may break. Acceptable here since releases are infrequent and re-runs are manual.

## Summary

- **Before hooks:** `go mod tidy` + `go generate ./...` enforce clean deps and fresh generated code before every build
- **Release:** Published immediately (`draft: false`), prerelease inferred from tag (`auto`), existing releases replaced on re-run (`mode: replace`)
