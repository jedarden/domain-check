# GoReleaser Build Configuration

Documents the `builds` section of `.goreleaser.yml` — the cross-compilation matrix, linker flags, and ignored platform combinations.

## Environment

```
CGO_ENABLED=0
```

Disables cgo entirely, producing a **statically-linked binary** with zero C dependencies. Required for cross-compilation to work cleanly and ensures the binary runs on any target system without shared libraries. This is the standard setting for Go CLI tools that only use stdlib and pure-Go packages.

## Main Package

```
./cmd/domain-check
```

The entry point is `cmd/domain-check/main.go`, following the standard Go project layout convention of placing the executable package under `cmd/`.

## Target Platforms

### Operating Systems (`goos`)

| Value    | Platform         |
|----------|-----------------|
| `linux`  | Linux            |
| `darwin` | macOS            |
| `windows`| Windows          |
| `freebsd`| FreeBSD          |

### Architectures (`goarch`)

| Value   | Architecture |
|---------|-------------|
| `amd64` | 64-bit x86 (x86_64) |
| `arm64` | 64-bit ARM (AArch64) |
| `arm`   | 32-bit ARM |

### ARM Version (`goarm`)

| Value | Target              |
|-------|--------------------|
| `7`   | ARMv7 (used when `goarch=arm`) |

ARMv7 supports hardware floating point and is the minimum version for modern ARM devices (Raspberry Pi 2+, most ARM servers). Older ARMv5/v6 devices are excluded.

### Full Matrix (before ignores)

|                | amd64 | arm64 | arm (v7) |
|----------------|-------|-------|----------|
| **linux**      | ✅    | ✅    | ✅        |
| **darwin**     | ✅    | ✅    | ✅*       |
| **windows**    | ✅    | ✅*   | ✅*       |
| **freebsd**    | ✅    | ✅    | ✅        |

## Ignored Combinations

Three entries in the `ignore` matrix exclude platforms where Go toolchain support is incomplete or the target is obsolete:

| goos      | goarch | goarm | Reason                                           |
|-----------|--------|-------|--------------------------------------------------|
| `windows` | `arm64`| —     | Windows ARM64 support in Go is experimental/imperfect; excludes to avoid shipping a binary that may have runtime issues |
| `windows` | `arm`  | —     | Windows on 32-bit ARM is effectively dead (Windows RT era); Go toolchain support is minimal |
| `darwin`  | `arm`  | —     | Apple Silicon Macs are arm64 only; no 32-bit ARM macOS hardware exists |

After exclusions, the **actual build matrix is 13 binaries**:

|                | amd64 | arm64 | arm (v7) |
|----------------|-------|-------|----------|
| **linux**      | ✅    | ✅    | ✅        |
| **darwin**     | ✅    | ✅    | ❌        |
| **windows**    | ✅    | ❌    | ❌        |
| **freebsd**    | ✅    | ✅    | ✅        |

## Linker Flags (`ldflags`)

```
-s -w -X main.version={{.Version}} -X main.commit={{.Commit}} -X main.date={{.Date}}
```

### Strip flags

| Flag | Purpose |
|------|---------|
| `-s` | Strip symbol table — removes debug symbols, reduces binary size |
| `-w` | Strip DWARF debug info — removes line number tables and type info |

Together `-s -w` typically reduce binary size by ~25–30%. Trade-off: no stack traces with line numbers in panics (acceptable for a release binary; debug builds omit these flags).

### Version injection (`-X`)

Each `-X` flag sets a Go package-level variable at link time by rewriting its initializer. The `main` package must declare these as `var` (not `const`) for injection to work:

| Flag                                | Variable        | Value              | Purpose |
|-------------------------------------|-----------------|--------------------|---------|
| `-X main.version={{.Version}}`       | `main.version`  | Git tag (e.g. `v1.2.3`) or `snapshot` for untagged builds | Semver version reported by `domain-check --version` |
| `-X main.commit={{.Commit}}`       | `main.commit`   | Full git SHA (e.g. `a1b2c3d4`) | Exact commit for auditability — `--version` shows which source produced the binary |
| `-X main.date={{.Date}}`           | `main.date`     | Build timestamp in RFC3339 (e.g. `2026-07-02T14:30:00Z`) | When the binary was built, useful for diagnosing "which build is running?" |

GoReleaser populates `{{.Version}}`, `{{.Commit}}`, and `{{.Date}}` from the git tag and commit at release time. For non-release builds (e.g. `goreleaser release --snapshot`), `.Version` defaults to the `snapshot` template value.

## Summary

- **13 release binaries** across 4 OSes and 3 architectures
- **Fully static** (`CGO_ENABLED=0`) — no runtime dependencies
- **Stripped** (`-s -w`) for minimal binary size
- **Version-injected** via ldflags — `--version` reports exact git tag, commit SHA, and build date
