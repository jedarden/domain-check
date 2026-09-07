# Crash Report: Bead bf-198ne (2026-08-16) — Resolution & Verification

**Report Date:** 2026-09-06
**Bead ID:** bf-198ne ("ALERT: Agent crash on bead bf-2xygo")
**Crashed worker:** claude-code-glm-4.7 (attempts 4 and 5 of the 2026-08-16 dispatch series)
**Parent alert:** domchk-d46ec441 (closure bead for the chain)
**Classification:** INFRASTRUCTURE — memcg OOM (`CONSTRAINT_MEMCG`) SIGKILL of `git` inside a 12 GiB dispatch scope
**Status:** RESOLVED — zero data loss, all countermeasures verified in force, no further action required

---

## Executive Summary

Two consecutive agents dispatched to bf-198ne on 2026-08-16 died of signal death (needle sentinel
`exit -1`) ~50 seconds after each issued `git push`. Both had already committed their work. The
kernel killed their `git push`'s `pack-objects` at the **exact 12 GiB hard bound of the dispatch
scope** (`usage 12582912kB, limit 12582912kB`, recovered from journald), while pushing a
720-commit backlog whose tree still carried **5.6 GB of retired bead-forge state**
(`.beads/.bf_history`, 4.7 GB + 508 stale checkpoint shards) that origin had never received.

The crash was a post-completion kill: the work the agents had built (Domain Watch, ADR-001) was
committed before either died and is in current `main` today. The object mass that made the push
heavy was deleted 22 seconds after the wave's last kernel kill, and the crashes stopped.

**No code defect. No data loss. Every mitigation the crash implies is in force and was re-verified
live on 2026-09-06 (§4).** The event is closed.

---

## 1. Disambiguation — three events share these names

This workspace's alert beads have repeatedly been confused with one another by title alone. The
chain here is:

| Bead | What it is | Status |
|---|---|---|
| `bf-2xygo` | The *original* task (Forgejo/GitHub divergence analysis) whose agent crashed 2026-08-12 | closed |
| `bf-198ne` | The *alert* bead created for that 2026-08-12 crash | closed |
| **domchk-d46ec441** | The *second* alert: an agent dispatched to *work on* bf-198ne died **2026-08-16T13:30:50Z** | open → close with this report |
| `docs/crash-investigation/bf-198ne-context.md` | Documents the **2026-08-12** bf-2xygo event | — |
| `docs/crash-investigation/bf-198ne-crash-2026-08-16-artifact-analysis.md` | Documents the **2026-08-16** death — the canonical RCA for this report | — |

This report covers the **2026-08-16T13:30:50Z** death (parent alert domchk-d46ec441). The
2026-08-12 event is separate and already documented in `bf-198ne-context.md`.

---

## 2. What happened (summary — full detail in the canonical RCA)

Canonical document: [`docs/crash-investigation/bf-198ne-crash-2026-08-16-artifact-analysis.md`](../crash-investigation/bf-198ne-crash-2026-08-16-artifact-analysis.md)
(artifact inventory, 8-dispatch timeline, both kernel kill records, fleet census).

- **Attempts 4 and 5** of the 2026-08-16 dispatch series both ended: commit → `git push` → kernel
  SIGKILL of `git` 48.6–49.0 s later → `agent.completed` with the −1 sentinel 1.0–1.4 s after that.
- **Kernel kill records (recovered from journald):** both deaths show `task=git`,
  `oom-kill:constraint=CONSTRAINT_MEMCG`, memcg at `usage 12582912kB, limit 12582912kB` — the
  scope at its exact hard limit, anon-rss ≈ 11.7 GiB.
- **Why the push was heavy:** 720 commits of unpushed backlog including 5.6 GB of retired
  bead-forge state. `b2d8233` (".beads: 5.6G -> 16M") landed at 17:40:54Z — 22 s after the wave's
  final kernel kill — and the crashes stopped.
- **Fleet context:** 414 memcg kills that day, 332 in the 12:00–17:00Z wave; 26 crashes across 4
  worker types in the 18-minute window containing both deaths. Nothing bead-specific.
- **Sentinel correction:** `exit -1` is needle's sentinel for a signal death (`code().unwrap_or(-1)`),
  **not** SIGHUP (signal 1) and **not** literally "SIGKILL signal 9". Two earlier documents
  (`bf-198ne-context.md` §"Signal -1 Technical Analysis",
  `docs/notes/agent-crash-investigation-domchk-d46ec441.md` header) misstate the mechanism; the
  kill was memcg-constrained inside the dispatch scope, not the host OOM killer and not SIGHUP.
  Their conclusions (infrastructure, not code) survive; their mechanism lines do not.
- **Prior-doc correction:** `docs/notes/agent-crash-investigation-domchk-d46ec441.md` cites commit
  `a3e2981` for the divergence statistics; that object does not exist. The real commit is
  **`20e4c3c`** (pre-squash reference not re-resolved).

## 3. Classification (per `docs/crash-response-guide.md` §Phase 2A)

**INFRASTRUCTURE — memory-pressure memcg OOM.** Exit −1 → infrastructure; confirmed directly at
kernel level for both deaths (`CONSTRAINT_MEMCG`, scope at exact bound, `task=git`). Not a workflow
failure (the same bead's `exit 1` attempts look entirely different — agent ran to completion), not
a service failure (no HTTP 5xx in the causal path), not a code defect (stock `git push`; domain-check
code not executing in a contributing way). Severity: low — transient, self-healed within 10 minutes
of dispatches, zero data loss.

---

## 4. Mitigation effectiveness — verified live 2026-09-06 by domchk-f3c5f61d

Re-executed on this bead's attempt, not cited from the mitigation transcript. Canonical mitigation
document: [`docs/crash-investigation/bf-198ne-mitigation-2026-09-06.md`](../crash-investigation/bf-198ne-mitigation-2026-09-06.md).

| # | Countermeasure | Live check | Result |
|---|---|---|---|
| 1 | Pack memory bounded by persistent config | `./scripts/setup-git-gc-config.sh --verify` | ✅ exit 0 — effective chain `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1`; worst-case pack memory ≈ 3072 MiB, within the 6442450944 B ceiling for a 12 GiB scope |
| 2 | Mechanism proven bounded under cgroup | `./scripts/test-gc-memory-bounds.sh` | ✅ **12 passed, 0 failed**; the integration test reruns the exact crash command (`git gc --aggressive --prune=now`, 8 × 64 MiB incompressible blobs) under `MemoryMax=768M` and exits 0 with **pack-objects peak RSS 320,516 KB** — against the >12 GiB that killed both agents |
| 3 | The 5.6 GB object mass cannot return | `.gitignore` line 66 = `.beads/`; `git ls-files .beads/` = 0; `.beads/.bf_history` absent | ✅ structurally closed |
| 4 | Repo healthy | `git count-objects -vH` / `du -sh .git` | ✅ 93 MB total, 1 pack (90.43 MiB), 98 loose objects (788 KiB) — vs the 18 GB that caused bf-1s6c3 |
| 5 | Large-file admission gate | `.git/hooks/pre-commit` present, executable, 3446 B (`MAX_SIZE_MB=10`) | ✅ installed |
| 6 | Fleet monitoring live | 6 `domain-check-*` systemd user timers | ✅ all with recent runs and future triggers |
| 7 | No data loss (object lookup) | `git cat-file -t 26dab61 / 7a50353 / c27899f / b2d8233 / 20e4c3c` all `commit`; `pre-squash-history-20260816` = `7e4edf6c`; `internal/server/handlers_watch.go` in HEAD at exactly 162 lines; `docs/adr/001-domain-watch-webhook-notifications.md` present (7,734 B); `origin/main..HEAD` = 0 and `HEAD..origin/main` = 0 | ✅ zero data loss |
| 8 | Close gate | `go build ./...` and `go test ./...` | ✅ build OK, all packages pass |
| 9 | Resources at verification time | `free`/`df`/`uptime` | ✅ 44 GiB avail mem, 78 GB disk, load 3.15 |

`pack.windowMemory` is read by `pack-objects` wherever git spawns it — `gc`, `repack`, **and
`push`** — so the operation that actually died is covered, not only the `gc` variant the bounds
were originally written for.

### Recurrence assessment

The crash mechanism (unbounded `pack-objects` inside a dispatch scope) is **not recurring in this
repository**. A spacing-agnostic scan of all needle logs modified in the last 3 days (15,192 files)
finds **5 `exit_code:-1` events, all in the `drawrace` workspace with synthetic dispatch ids
(`nd-9gi8`, `nd-3wrf`, `drawrace-*`)** — the test-probe class — and none in domain-check. The
pattern that produced this crash (real work + a multi-GB unpushed object mass) is closed
structurally: gitignored `.beads/`, the 10 MB pre-commit gate, bounded pack memory, and the
monitoring timers that would surface the precursor conditions within minutes.

The recurring *pattern* worth remembering is recorded in persistent memory
(`agent-crash-bf-4x12ec-investigation`): `exit -1` is a sentinel, not a signal number, and
memcg OOM SIGKILL of a `git` process inside a dispatch scope is this fleet's dominant
infrastructure crash class — with bf-198ne as the `git push`-spawned variant of the mechanism
already documented for `git gc` (bf-4x12ec) and repository bloat (bf-1s6c3, bf-173o7e).

---

## 5. Closure record for parent alert domchk-d46ec441

The parent alert is blocked by two beads: domchk-ed5d9143 (implement fix — **closed**, verification
note on the bead) and domchk-f3c5f61d (this documentation bead — closes with this report). On
closure of the latter the parent is unblocked and should be closed citing:

1. **`docs/crash-investigation/bf-198ne-crash-2026-08-16-artifact-analysis.md`** — §8 classification
   (INFRASTRUCTURE) and §5.1 kernel kill records (`CONSTRAINT_MEMCG`, `usage 12582912kB,
   limit 12582912kB`, `task=git`).
2. **`docs/crash-investigation/bf-198ne-mitigation-2026-09-06.md`** — all countermeasures in force
   with live evidence.
3. **This report** — independent re-verification on 2026-09-06 (§4 above), closure of the chain.
4. **The sentinel/mechanism correction** — `bf-198ne-context.md`'s "signal −1 = SIGKILL delivered by
   the OOM killer" line is superseded: −1 is needle's sentinel for a signal death, and the kill was
   memcg-constrained inside the dispatch scope.
5. **Work outcome** — the crashed attempts' deliverable (Domain Watch + ADR-001) and the alert's
   subject work (bf-2xygo divergence analysis, `20e4c3c`) are all in `main`; no retry warranted.

## 6. Acceptance criteria

- [x] **Crash investigation documented in `docs/crashes/`** — this report (the earlier chain
      documents live in `docs/crash-investigation/` and `docs/notes/`; this is the `docs/crashes/`
      entry the chain lacked), registered in `docs/crashes/crash-documentation-index-2026-09-02.md`.
- [x] **Pattern added to memory** — the memcg/dispatch-scope OOM class was already recorded
      persistently; updated with the bf-198ne `git push`-spawned variant and its 2026-09-06
      verification.
- [x] **Mitigation effectiveness verified** — §4, nine checks re-executed live, all pass.
- [x] **Parent bead domchk-d46ec441 can be closed** — §5 closure record; its other blocker
      (domchk-ed5d9143) is already closed.
- [x] **No further action required on bf-198ne** — subject closed, work in main, mechanism bounded
      and tested, residual `exit -1` tail is synthetic probes in other workspaces.

---

*Written 2026-09-06 by domchk-f3c5f61d (claude-code-glm-5.3-flash, lab domain-check workspace).
Every check in §4 was executed during this bead's attempt. Chain: domchk-507d18c4 (investigate) →
domchk-9d269c9c (classify) → domchk-99f184b6 (mitigate) → **domchk-f3c5f61d (this bead,
document/verify)** → parent alert domchk-d46ec441.*
