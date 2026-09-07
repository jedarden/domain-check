# GoReleaser Archives, Checksum, and Changelog Configuration

Documents the `archives`, `checksum`, and `changelog` sections of `.goreleaser.yml` — release packaging, integrity verification, and changelog generation.

See [09-goreleaser-builds.md](09-goreleaser-builds.md) for the `builds` section documentation.

## Archives

### Format by Operating System

| goos      | Format  | Rationale                                          |
|-----------|---------|----------------------------------------------------|
| `linux`   | `tar.gz`| Default — native Unix archive format              |
| `darwin`  | `tar.gz`| macOS is Unix-based, tar.gz is conventional        |
| `freebsd` | `tar.gz`| FreeBSD is Unix-based, tar.gz is conventional     |
| `windows` | `zip`   | Override — Windows users expect zip, not tar.gz   |

The default format is `tar.gz` (line 40). The `format_overrides` block (line 49) switches Windows to `zip`, since Windows has no built-in tar.gz support prior to Windows 10 1803 and zip remains the expected format on that platform.

### Archive Naming Template

```
{{ .ProjectName }}_{{ title .Os }}_{{ arch }}
```

The template produces names like:

| Build Target                        | Archive Name                              |
|-------------------------------------|------------------------------------------|
| linux/amd64                         | `domain-check_Linux_x86_64`              |
| linux/arm64                         | `domain-check_Linux_arm64`                |
| linux/arm (v7)                      | `domain-check_Linux_armv7`               |
| darwin/amd64                        | `domain-check_Darwin_x86_64`             |
| darwin/arm64                        | `domain-check_Darwin_arm64`              |
| windows/amd64                       | `domain-check_Windows_x86_64.zip`        |
| freebsd/amd64                       | `domain-check_FreeBSD_x86_64`            |
| freebsd/arm64                       | `domain-check_FreeBSD_arm64`             |
| freebsd/arm (v7)                    | `domain-check_FreeBSD_armv7`             |

#### Architecture Name Mapping

GoReleaser uses Go toolchain architecture names (e.g. `amd64`), but the template renames them to the conventional names users expect:

| Go Arch | Renamed To  | Rule                                         |
|---------|-------------|----------------------------------------------|
| `amd64` | `x86_64`    | `eq .Arch "amd64"` → `x86_64`              |
| `386`   | `i386`      | `eq .Arch "386"` → `i386`                  |
| `arm`   | `armv{ver}` | `eq .Arch "arm"` → `armv{{ .Arm }}`        |
| `arm64` | `arm64`     | Falls through to `{{ .Arch }}` (no rename) |

Note: `386` is listed in the template even though the current `builds.goarch` matrix does not include it (only `amd64`, `arm64`, `arm`). The mapping is retained as a guard if `386` is added to the build matrix in the future.

The OS name is title-cased via `title .Os` (e.g. `linux` → `Linux`).

### Included Files

Each archive contains the binary plus two extra files:

| File       | Purpose                                      |
|------------|----------------------------------------------|
| `LICENSE`  | MIT license — required for legal compliance   |
| `README.md`| Usage instructions and quick-start guide      |

These are specified in the `files` list (line 52). GoReleaser bundles them alongside the compiled binary inside each archive.

## Checksum

```
checksums.txt
```

GoReleaser generates a SHA256 checksum file named `checksums.txt` containing one line per release artifact:

```
<sha256>  domain-check_Linux_x86_64.tar.gz
<sha256>  domain-check_Linux_arm64.tar.gz
<sha256>  domain-check_Windows_x86_64.zip
...
```

Users verify download integrity by comparing:

```bash
sha256sum -c checksums.txt --ignore-missing
```

The filename `checksums.txt` is set by `name_template` in the `checksum` section (line 57).

## Changelog

### Sort Order

```
sort: asc
```

Changelog entries are sorted in **ascending** order — oldest commits appear first, newest last. This is the conventional reading order for changelogs (chronological top-to-bottom) and matches the format expected by most changelog parsers.

### Exclude Filters

The `filters.exclude` list (line 63) strips conventional commit prefixes that don't represent user-facing changes. Excluded prefixes:

| Prefix          | Typical Use                                  |
|-----------------|----------------------------------------------|
| `^docs:`        | Documentation-only changes (README, research docs, inline comments) |
| `^test:`        | Test additions, fixes, and test infrastructure |
| `^ci:`          | CI/CD configuration (GoReleaser, Argo Workflows, Dockerfile) |
| `^chore:`       | Maintenance tasks (dependency bumps, .gitignore, tooling) |
| `^build:`       | Build system changes (Makefile, ldflags, cross-compilation) |

All filters use anchored regex (`^`) so they match only at the start of the commit message's first line. Only commits whose first line starts with one of these prefixes are excluded.

**Effect:** The generated changelog contains only user- and operator-relevant changes: features (`feat:`), bug fixes (`fix:`), performance improvements (`perf:`), and breaking changes (`!` or `BREAKING`). Housekeeping commits that would clutter the changelog are filtered out.

## Summary

- **Archives:** `tar.gz` for Unix (linux, darwin, freebsd), `zip` for Windows; binary + `LICENSE` + `README.md`; arch names rewritten (amd64 → x86_64)
- **Checksum:** Single `checksums.txt` with SHA256 digests for all release artifacts
- **Changelog:** Ascending chronological order, excluding `docs:`, `test:`, `ci:`, `chore:`, and `build:` prefixed commits
