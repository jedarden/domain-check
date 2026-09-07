# Verification Report: domchk-bde0c832 — bf-173o7e alert timestamp mapped to storm attempt #81

| Field | Value |
|---|---|
| **Dispatch bead** | `domchk-bde0c832` — "Investigate agent crash on bead bf-173o7e" (created 2026-08-27T00:16:24Z; dispatched 2026-09-02) |
| **Subject bead** | `bf-173o7e` — "Execute git gc --aggressive with pruning" (closed 2026-08-17, work complete) |
| **Alert timestamp investigated** | `2026-08-14T21:33:47.099214956Z` (first carried by alert `bf-5r72xi`; never before correlated to its kill) |
| **Disposition** | **RESOLVED — INFRASTRUCTURE** (kernel memcg OOM SIGKILL); this dispatch is a stale duplicate; **new evidence: sixth timestamp→attempt mapping for the storm** |
| **Root cause** | Attempt **#81 of the 132-dispatch Aug-14 storm**: `agent.completed` **21:33:20.417010Z, `exit_code:-1`, 65.76 s in** — kernel memcg OOM SIGKILL of the agent scope running the bead's `git gc --aggressive --prune=now` on 17.20 GiB loose objects in the 12 GiB dispatch scope. Canonical RCA: [`investigations/bf-173o7e-root-cause-determination-domchk-2e371a2c-2026-09-02.md`](investigations/bf-173o7e-root-cause-determination-domchk-2e371a2c-2026-09-02.md) (07ab240) |

## 1. Why this bead existed

`domchk-bde0c832` was created 2026-08-27 by the auto-split chain investigating
bf-173o7e and re-dispatched 2026-09-02 — ten days after the subject bead
closed (2026-08-17) and after the definitive RCA (07ab240) was already
committed. It is itself an artifact of the stale-alert regeneration loop the
canonical RCA §4 documents. This report discharges its acceptance criteria by
correlating the one storm timestamp no prior dispatch had mapped, verifying
the resolution still holds, and closing.

## 2. Evidence package (all from the primary needle log)

Source: `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl`,
worker `claude-code-glm-4.7-lab-domain-check`, session `a6eaf955`
(lines 8487–8545). Kernel logs do not survive Aug 14 (boot begins Aug 15) and
journald begins Aug 17, so this log is the primary surviving record.

### 2.1 The alert timestamp is a release heartbeat, not the kill

| T (2026-08-14, UTC) | Event | Seq |
|---|---|---|
| 21:32:14.449004Z | `agent.dispatched` — attempt **#81 of 132**, `template=pluck`, `prompt_len=70842`, `prompt_hash=sha256:c995c129…` | 845 |
| 21:33:20.387994Z | `transform.completed` (65 899 ms, 11 events) | 847 |
| **21:33:20.417010Z** | **`agent.completed` — `exit_code:-1`, `duration_ms=65763` → the real kill** | 848 |
| 21:33:20.420208Z | `outcome.classified` — `exit_code:-1 → crash` | 851 |
| 21:33:44.324692Z | `HANDLING_FLUSH_DONE` → `HANDLING_RELEASE` | 858–859 |
| **21:33:47.099201Z** | **`HANDLING_RELEASE_DONE` ← the alert timestamp** (13.6 µs offset, same precision artifact as the 14:02:25 mapping) | 861 |
| 21:33:50.038903Z | `bead.released` (`release_success`) → `outcome.handled` `action=alerted outcome=crash` — the alert that spawned bf-5r72xi and this dispatch | 862–863 |

**Alert lag: 26.68 s** — inside the established 7.7–120 s heartbeat window.
The standing rule holds for the sixth time: **alert timestamps are never kill
times; re-derive from `agent.completed`.**

### 2.2 Attempt #81 fits every storm signature

- Kill at **65.76 s** sits inside the flat 21.6–216.6 s kill band whose 129
  identical durations prove the object set never shrank across 10.5 h —
  mid-gc, pre-pack-write, not at any timeout boundary.
- Full-size prompt (70 842 B, pluck template) — a genuine re-investigation
  attempt, not a 3 896 B auto-split.
- Immediate sibling **attempt #82** re-claimed at 21:33:53.256Z, same prompt
  hash, and died identically (`exit_code:-1`, 74.1 s) at 21:35:07.601Z;
  attempt #83 dispatched 21:35:42.838Z. The retry loop simply advanced.
- `fleet.cpu_saturated` context at dispatch: load 15.45–16.89 on 9 cores —
  ambient fleet load, not the cause (host had ~45 Gi free; all boot OOM kills
  are `CONSTRAINT_MEMCG`).

### 2.3 Workspace state at crash time

From the committed original-bead-context report (9d7e810): HEAD `00117cb`,
**17.20 GiB loose / 9.6 MiB packed**, zero Aug-14 commits, no files modified
by the bead (killed attempts write no traces). The Aug-14 git-side forensics
(reflogs, gc.pid) have since been expunged (d283576), so workspace state is
cited from that committed derivation, not re-derived.

### 2.4 Bead being processed

`bf-173o7e` — "Execute git gc --aggressive with pruning" (P2 task, created
2026-08-14T12:57:54Z, prescribed command in bead body). **Now CLOSED** (rev
18); the objective was achieved at storm end — repo consolidated to a single
90.18 MiB pack, fsck clean, re-verified 2026-09-02 (9f9930d, 6210dbf) — so at
the alerting level this is a false positive (real kill, already-resolved
work), exactly the class the canonical RCA labels "alert-level false
positive only."

## 3. Disposition

- **No retry needed** — work complete, repo healthy, memory bounds on the
  bare-gc path now mechanically enforced (`pack.windowMemory=2g`,
  `pack.deltaCacheSize=1g`, `pack.threads=1`; verified exit-0).
- **Canonical RCA updated** — §2 "five mapped" → **six mapped** for the
  storm, adding `assigned 21:33:47.099Z ← real kill 21:33:20.417Z, attempt
  #81`.
- Supersedes the only prior doc touching this timestamp
  ([`crash-reports/bf-5r72xi-verification.md`](crash-reports/bf-5r72xi-verification.md)),
  whose "gc completed before the crash" claim the canonical RCA already
  corrected (completion came via the storm's exit-0 attempt at 23:25:35Z).
