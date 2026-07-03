# Quality-Gate Node Status — domain-check-build-94972

**Query Time:** 2026-07-03

## Most Recent Workflow

| Field | Value |
|-------|-------|
| Workflow | `domain-check-build-94972` |
| Phase | Failed |
| Created | 2026-07-03T13:16:19Z |
| Overall Message | `child 'domain-check-build-94972-2895068185' failed` |

## Quality-Gate Node

| Field | Value |
|-------|-------|
| Node ID | `domain-check-build-94972-2895068185` |
| Display Name | `build-quality-gate` |
| Phase | **Failed** |
| Exit Code | **2** |
| Message | `main: Error (exit code 2)` |
| Started | 2026-07-03T13:16:20Z |
| Finished | 2026-07-03T13:22:01Z |
| Duration | ~5m 41s |
| Template | `build-quality-gate` (local/) |

## Context

Exit code 2 indicates the quality-gate step itself failed (not a pod-level crash). Per prior analysis (commit `0703367`), this is caused by `go test -race` failing on Alpine due to missing CGO support — Alpine musl libc does not support the race detector which requires CGO.

## Final Verification — All Parent Acceptance Criteria Met (bf-2ibq)

**Date:** 2026-07-03
**Status:** ✅ **Complete — no outstanding gaps**

| # | Acceptance Criterion | Status | Evidence |
|---|---------------------|--------|----------|
| 1 | Raw logs captured | ✅ Met | Full container output in `docs/quality-gate-logs.md` — debug workflow `domain-check-build-debug-podgc2-5dxln`, pod `domain-check-build-debug-podgc2-5dxln-build-quality-gate-1082504947` |
| 2 | Exit code identified | ✅ Met | Exit code **2** — documented in `quality-gate-node-status.md` (line 22), `quality-gate-root-cause.md` (line 13), `quality-gate-logs.md` (line 63). Go toolchain error, not a test failure. |
| 3 | Debug workflow logs obtained | ✅ Met | Complete step-by-step execution captured in `docs/quality-gate-logs.md` (all 5 steps: apk, clone, go version, go vet, go test -race) and corroborated in `docs/research/15-quality-gate-failure-analysis.md` |

### Cross-Document Consistency

All four quality-gate documents agree on the root cause, exit code, error message, and fix:

| Document | Exit Code | Error Message | Fix |
|----------|----------|---------------|-----|
| `quality-gate-status.md` | 2 | CGO disabled on Alpine | (references other docs) |
| `quality-gate-logs.md` | 2 | `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1` | Add `CGO_ENABLED=1` + gcc/musl-dev |
| `quality-gate-node-status.md` | 2 | CGO disabled on Alpine | Add `CGO_ENABLED=1` to env |
| `quality-gate-root-cause.md` | 2 | `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1` | Option A (Debian) or Option B (Alpine+gcc) |
| `docs/research/15-quality-gate-failure-analysis.md` | 2 | `go: -race requires cgo; enable cgo by setting CGO_ENABLED=1` | Option A (Debian, recommended) or B (Alpine+gcc) or C (drop -race) |

### Outstanding Items

None. The root cause is identified, logs are captured, and the fix is documented. The remaining work is applying the fix to the `domain-check-build` WorkflowTemplate in `declarative-config` — that is a separate task outside the scope of this quality-gate investigation.
