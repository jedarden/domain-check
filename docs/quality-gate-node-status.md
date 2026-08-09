# Quality-Gate Node Status — Recent domain-check-build Workflows

**Query Time:** 2026-07-03 (updated)

## Most Recent Failed Run (non-debug)

| Field | Value |
|-------|-------|
| Workflow | `domain-check-build-jxwhw` |
| Node ID | `domain-check-build-jxwhw-4035286674` |
| Node Display Name | `build-quality-gate` |
| Phase | **Failed** |
| Message | `main: Error (exit code 2)` |
| Exit Code | null (not recorded at workflow node level) |
| Type | Pod |
| Started | 2026-07-04T01:46:35Z |
| Finished | 2026-07-04T01:47:23Z |
| Duration | ~48s |

## All Recent Failed Runs

| Workflow | Node ID | Phase | Message | Started | Finished |
|----------|---------|-------|---------|---------|----------|
| `domain-check-build-jxwhw` | `domain-check-build-jxwhw-4035286674` | Failed | `main: Error (exit code 2)` | 2026-07-04T01:46:35Z | 2026-07-04T01:47:23Z |
| `domain-check-build-hsrx2` | `domain-check-build-hsrx2-3270610851` | Failed | `main: Error (exit code 2)` | 2026-07-04T01:33:23Z | 2026-07-04T01:34:12Z |
| `domain-check-build-debug-qg-wl5sw` | `domain-check-build-debug-qg-wl5sw-3572223479` | Failed | `main: Error (exit code 2)` | 2026-07-04T02:00:03Z | 2026-07-04T02:01:11Z |
| `domain-check-build-debug-qg-h89zn` | `domain-check-build-debug-qg-h89zn-2378505076` | Failed | `main: Error (exit code 2)` | 2026-07-04T00:59:32Z | 2026-07-04T01:00:18Z |
| `domain-check-build-45dk7` | `domain-check-build-45dk7-2865606903` | Failed | `main: Error (exit code 2)` | 2026-07-04T00:51:47Z | 2026-07-04T00:53:58Z |

## Most Recent Run Overall

`domain-check-build-debug-qg-78gw7` — **Succeeded** (6m53s ago).

## Root Cause (from prior investigation)

All quality-gate failures share the same generic message: `main: Error (exit code 2)`. Prior investigation (via debug workflow `domain-check-build-debug-podgc2-5dxln` with `podGC: OnWorkflowSuccess`) confirmed the root cause:

**The `golang:1.26-alpine` base image disables CGO by default. The Go race detector (`-race` flag) requires `CGO_ENABLED=1`.** The quality-gate step sets no `CGO_ENABLED` environment variable, so CGO is off (`0`), and `go test -race` fails immediately.

Key log excerpt:
```
+ go test -race ./...
go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
time=2026-07-03T21:56:13.054Z level=INFO msg="sub-process exited" argo=true error="exit status 2"
Error: exit status 2
```

**Fix:** Add `CGO_ENABLED=1` to the quality-gate container's `env` list in the `domain-check-build` WorkflowTemplate in `declarative-config`. Alpine uses musl libc, so CGO is functional but the resulting binary links against musl rather than glibc.

## Notes

- Pod GC policy (`OnPodCompletion`) means container-level logs and exit codes are not retrievable after the pod finishes.
- The `exitCode` field is null at the workflow node level — only the message string indicates the code.
- To capture the actual quality-gate failure reason, the workflow must be submitted with `podGC: OnWorkflowCompletion` override, or logs must be streamed while the pod is running.
- `domain-check-build-debug-qg-hkst6` failed at the entry template level (service account `github-auth` not found), not at the quality-gate step.

## Audit Trail

- Original workflow `domain-check-build-94972` — no longer present in cluster (garbage collected), all pod logs unrecoverable
- Debug workflow `domain-check-build-debug-podgc2-5dxln` — full logs captured in `docs/quality-gate-logs.md`
- Prior debug workflows (`bf-3mbz`, `podgc-hqwdk`) — pods cleaned up before logs captured; led to the `OnWorkflowSuccess` strategy that succeeded on the third attempt
- This update: queried cluster 2026-07-03, confirmed consistent failure pattern across 5 consecutive failed runs
