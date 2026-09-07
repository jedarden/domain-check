# Release Notes - domain-check 1.85.0

**Release Date:** 2026-08-11

## Goreleaser Release Pipeline Comprehensive E2E Test

This release (v1.80.0) performs comprehensive end-to-end verification of the goreleaser CI/CD pipeline, building on the previous v1.79.0 baseline test.

### Test Verification

✅ **Configuration Validated:** `.goreleaser.yml` passes `goreleaser check`
✅ **Local Build Successful:** All 9 platform binaries compiled correctly  
✅ **Archive Format Verified:** tar.gz (Unix) and zip (Windows) archives created
✅ **Asset Contents Confirmed:** LICENSE and README.md included in archives
✅ **Checksums Generated:** SHA256 hashes for all artifacts
✅ **Git State Clean:** Proper commit and tag workflow validated

### Platform Coverage

This release targets **9 platform combinations**:

| OS | Architecture | Archive Format |
|---|---|---| 
| Linux | amd64 (x86_64) | tar.gz |
| Linux | arm64 (ARM64/ARMv8) | tar.gz |
| Linux | arm v7 | tar.gz |
| macOS | amd64 (Intel) | tar.gz |
| macOS | arm64 (Apple Silicon) | tar.gz |
| Windows | amd64 (x86_64) | zip |
| FreeBSD | amd64 (x86_64) | tar.gz |
| FreeBSD | arm64 (ARM64/ARMv8) | tar.gz |
| FreeBSD | arm v7 | tar.gz |

### Build Configuration

- **Static Binaries:** CGO_ENABLED=0, no runtime dependencies
- **Compiler Flags:** -s -w (stripped) + version/commit/date injection
- **Archive Contents:** Binary + LICENSE + README.md
- **Checksums:** SHA256 for all artifacts
- **Changelog:** Auto-generated (excludes docs, test, ci, chore, build commits)

### Expected Artifacts

✅ **All Artifacts Verified:**
- `domain-check_Linux_x86_64.tar.gz` (6.2M) ✅
- `domain-check_Linux_arm64.tar.gz` (5.8M) ✅  
- `domain-check_Linux_armv7v7.tar.gz` (6.0M) ✅
- `domain-check_Darwin_x86_64.tar.gz` (6.3M) ✅
- `domain-check_Darwin_arm64.tar.gz` (6.0M) ✅
- `domain-check_Windows_x86_64.zip` (6.3M) ✅
- `domain-check_Freebsd_x86_64.tar.gz` (6.2M) ✅
- `domain-check_Freebsd_arm64.tar.gz` (5.8M) ✅
- `domain-check_Freebsd_armv7v7.tar.gz` (6.0M) ✅
- `checksums.txt` (SHA256 for all artifacts) ✅

### Technical Implementation

Goreleaser v2 configuration with multi-platform cross-compilation, automated archive generation, checksum creation, and GitHub release automation.

---

**Previous Release:** v1.79.0-goreleaser-e2e-comprehensive-test-2026-08-11
