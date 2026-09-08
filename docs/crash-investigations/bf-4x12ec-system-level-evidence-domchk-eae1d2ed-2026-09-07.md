# bf-4x12ec — System-Level Cause Class: host-wide OOM vs memcg OOM vs timeout

Investigator bead: **domchk-eae1d2ed** (child 3 of 4 of the `domchk-2ff261ce`
split) · Crash bead: **bf-4x12ec** ("Execute aggressive git garbage collection
to eliminate OOM risk", task, P2) · Worker `claude-code-glm-4.7-lab-domain-check`
· Needle session `a6dbb1fc` · All timestamps UTC (box local = UTC−4).

Written 2026-09-08T01:4xZ (2026-09-07 evening local). Consumes the timeline
delivered by child 2
([`bf-4x12ec-crash-timeline-domchk-ba8584a1-2026-09-07.md`](bf-4x12ec-crash-timeline-domchk-ba8584a1-2026-09-07.md))
and the surviving-source inventory from child 1
([`bf-4x12ec-log-source-inventory-domchk-a3f1f8f5-2026-09-07.md`](bf-4x12ec-log-source-inventory-domchk-a3f1f8f5-2026-09-07.md)).
Every figure was re-derived this attempt from a primary source — the committed
evidence bundle `docs/crashes/bf-4x12ec/` (53 attempt transcripts, census.tsv,
attempt-index.tsv, needle-events JSONL), the live bead store, `journalctl`, and
live systemd scope properties — not carried from the prior documents. The
mechanism itself is the canonical report's
([`bf-4x12ec-crash-investigation.md`](bf-4x12ec-crash-investigation.md),
Addenda 2/3/6); this document re-verifies it at the system level and answers
one question the split posed: **which resource boundary killed the 44
`exit −1` attempts?**

## 1. Verdict

**Cgroup/memcg OOM inside the agent's transient dispatch scope
(`run-p*.scope`, `MemoryMax` = 12 GiB) — not host-wide OOM, not a timeout.**

- Host-wide OOM is excluded by captures taken *inside* the crash window: the
  host held 45 Gi available with 0 B swap used mid-storm (§3).
- Timeout is excluded by exit-code arithmetic: needle's own deadline kill is
  recorded as `124`, and all 8 of those are cleanly separated; the 44 deaths
  landed at 38.9–115.8 s, nowhere near any 600 s ceiling (§6).
- Memcg OOM is the only candidate consistent with every measurement
  (§5). For Aug-14 it remains **regime-matched, not kernel-proven** — the
  kernel journal for the crash window is unrecoverable — and the kernel-proven
  same-mechanism twin is the Aug-16 `CONSTRAINT_MEMCG` git-kill census,
  re-extracted live this attempt (§5.3).

## 2. Method

| Candidate | Test applied this attempt | Result |
|---|---|---|
| Host-wide OOM | Read the mid-storm host capture from attempt 15's transcript record-by-record; corroborate with attempt 2 | §3 — host not memory-constrained |
| Cgroup/memcg OOM | Live scope properties; live `oom_score_adj`; aggressive-gc memory shape vs 17.20 GiB loose; Aug-16 kernel census re-extraction; Aug-14 journal bounds | §5 — supported |
| Timeout | Exit-code/duration census re-derived from the committed needle-events bundle; per-attempt durations from attempt-index.tsv | §6 — excluded |

Primary sources re-read this attempt:

- `docs/crashes/bf-4x12ec/transcripts/transcript-attempt15-9539f3b2.jsonl` (mid-storm capture)
- `docs/crashes/bf-4x12ec/transcripts/transcript-attempt02-971486ad.jsonl` (corroboration)
- `docs/crashes/bf-4x12ec/transcripts/` — all 53 transcripts, last-record census (§4)
- `docs/crashes/bf-4x12ec/transcripts/census.tsv` + `docs/crashes/bf-4x12ec/attempt-index.tsv` (join keys)
- `docs/crashes/bf-4x12ec/needle-events-2026-08-14-bf-4x12ec.jsonl.gz` (1,146 events; exit census)
- `journalctl --list-boots`, `journalctl --since @<epoch>` (Aug-16 window), `systemctl --user show` on a live `run-p*.scope`

## 3. Host-wide OOM: excluded (mid-storm host state)

Attempt 15 is the attempt that captured host state seconds before dying.
Re-read record-by-record from its transcript this attempt:

| Time (UTC) | Record | Reading |
|---|---|---|
| 10:43:47.915 | `tool_use` `git count-objects -vH` | — |
| 10:43:49.108 | `tool_result` | **4,649 loose objects, 17.20 GiB** (in-pack 4,081, 1 pack 9.60 MiB, 0 garbage) |
| 10:43:58.860 | `tool_use` `df -h / && echo --- && free -h` | — |
| **10:43:59.438** | `tool_result` | disk **444G total / 355G used / 67G avail (85 %)**; Mem 62Gi total, 16Gi used, 24Gi free, 17Mi shared, 22Gi buff/cache, **45Gi available**; **Swap 24Gi, 0 B used** |
| 10:44:07.088 | assistant text | "Now I'll start the aggressive garbage collection…" |
| 10:44:07.643 | `tool_use` `git gc --aggressive --prune=now` (timeout 600000 ms) | **no `tool_result` ever returned** |
| 10:44:53.202 | `agent.completed` (worker log) | **exit −1**, 74.3 s attempt duration |

**Reading: the host was not memory-constrained when the gc was launched.**
45 Gi available, 24 Gi of completely unused swap, and disk not yet the
constraint (67 G free; the 85 % figure is the repo-bloat symptom, not a kill
mechanism). Corroborated earlier in the same window by attempt 2
(`free -h` at 10:23:56.490Z, re-read this attempt): **50 Gi available**,
swap 24 Gi / 0 B used, load 10.86 — and attempt 2 was killed 65 s later.

Two further host-side observations from the same records:

- **Zero packing progress across the storm.** `git count-objects -vH` reads
  identically at 10:21:23 (attempt 1) and 10:43:49 (attempt 15) — 4,649 loose
  / 17.20 GiB, 22.4 minutes apart. Aggressive gc builds delta chains in
  memory *before* writing any pack, so 44 attempts × ~30 s of gc each left no
  on-disk trace. The repo the 44th attempt measured was the repo the 1st
  died on.
- **The host outlived the storm.** Phase 1 ran 64 m 23 s of continuous
  kill/retry on one host that stayed up, and phase 2's eight 600 s attempts
  (11:28–12:50Z) and attempt 53's 491.8 s success ran on the same host
  afterward. A host-wide OOM condition does not selectively kill only the
  process currently running `git gc`, 44 times in a row, while the same
  machine keeps serving 14 other needle workers.

Host-wide OOM is therefore **ruled out** as the cause class of the
`exit −1` kills.

## 4. Every kill lands inside its own gc launch

The child-1 census established that all 44 phase-1 transcripts end in an
unanswered gc `tool_use`; this attempt re-derived it directly (last
conversation record of each of the 44 crash transcripts: 43 ×
`Bash: git gc --aggressive --prune=now`, 1 × attempt 8's
`echo "Starting aggressive git garbage collection at $(date)" && git gc …`
variant; the 8 timeout transcripts contain no assistant records at all and
attempt 53 ends in `SPLIT_COMPLETE` text).

New this attempt — **the per-kill correlation table**, joining each
transcript's fatal gc record to its worker-log kill from
attempt-index.tsv:

| # | fatal gc `tool_use` record (UTC) | `agent.completed` kill (UTC) | gap (s) | note |
|---|---|---|---|---|
| 1 | 10:22:36.260 | 10:23:02.958 | 26.7 |  |
| 2 | 10:24:08.660 | 10:25:01.512 | 52.9 |  |
| 3 | 10:26:24.201 | 10:26:47.738 | 23.5 |  |
| 4 | 10:28:03.262 | 10:28:26.319 | 23.1 |  |
| 5 | 10:29:14.511 | 10:29:33.156 | 18.6 |  |
| 6 | 10:30:24.696 | 10:31:05.871 | 41.2 |  |
| 7 | 10:31:47.377 | 10:32:08.823 | 21.4 |  |
| 8 | 10:32:50.279 | 10:33:11.442 | 21.2 | gc via `echo … &&` wrapper |
| 9 | 10:34:19.552 | 10:34:40.898 | 21.3 |  |
| 10 | 10:35:42.546 | 10:36:05.465 | 22.9 |  |
| 11 | 10:37:13.874 | 10:37:54.324 | 40.5 |  |
| 12 | 10:39:02.604 | 10:39:27.018 | 24.4 |  |
| 13 | 10:40:25.677 | 10:40:53.801 | 28.1 |  |
| 14 | 10:42:23.551 | 10:42:58.570 | 35.0 |  |
| 15 | 10:44:07.643 | 10:44:53.202 | 45.6 |  |
| 16 | 10:45:36.609 | 10:46:16.579 | 40.0 |  |
| 17 | 10:47:14.336 | 10:48:14.021 | 59.7 |  |
| 18 | 10:49:10.071 | 10:49:35.988 | 25.9 |  |
| 19 | 10:50:22.159 | 10:50:43.362 | 21.2 |  |
| 20 | 10:51:29.244 | 10:51:49.607 | 20.4 |  |
| 21 | 10:52:46.049 | 10:53:06.749 | 20.7 |  |
| 22 | 10:54:36.927 | 10:55:21.576 | 44.6 |  |
| 23 | 10:56:26.477 | 10:57:03.172 | 36.7 |  |
| 24 | 10:57:48.576 | 10:58:12.166 | 23.6 |  |
| 25 | 10:59:24.136 | 10:59:48.535 | 24.4 |  |
| 26 | 11:00:58.085 | 11:01:31.948 | 33.9 |  |
| 27 | 11:02:17.197 | 11:02:59.658 | 42.5 |  |
| 28 | 11:04:32.859 | 11:04:55.025 | 22.2 |  |
| 29 | 11:05:38.657 | 11:05:58.444 | 19.8 |  |
| 30 | 11:06:48.057 | 11:07:07.394 | 19.3 |  |
| 31 | 11:07:52.855 | 11:08:26.236 | 33.4 |  |
| 32 | 11:09:24.654 | 11:09:44.290 | 19.6 |  |
| 33 | 11:10:42.664 | 11:11:03.458 | 20.8 |  |
| 34 | 11:11:39.645 | 11:12:12.344 | 32.7 |  |
| 35 | 11:12:45.542 | 11:13:05.688 | 20.1 |  |
| 36 | 11:13:59.270 | 11:14:21.326 | 22.1 |  |
| 37 | 11:15:12.468 | 11:15:34.379 | 21.9 |  |
| 38 | 11:16:30.661 | 11:16:58.674 | 28.0 |  |
| 39 | 11:18:04.558 | 11:18:40.715 | 36.2 |  |
| 40 | 11:19:35.716 | 11:20:56.353 | 80.6 |  |
| 41 | 11:21:58.960 | 11:22:28.529 | 29.6 |  |
| 42 | 11:23:29.884 | 11:24:19.158 | 49.3 |  |
| 43 | 11:25:15.642 | 11:25:36.847 | 21.2 |  |
| 44 | 11:26:49.882 | 11:27:26.173 | 36.3 |  |

*44 crash attempts (attempts 1–44). Gap = `agent.completed` minus the fatal gc `tool_use` record timestamp: **min 18.6 s · median 25.9 s · mean 30.7 s · max 80.6 s**. Attempt 8's last record is the same gc behind an `echo` header. Sub-second tails truncated (source timestamps are ns-precision; the last three decimals are omitted for width).*

Gap statistics: **44/44 killed 18.6–80.6 s after the fatal gc record**
(mean 30.7 s, median 25.9 s). Basis note: the left column is the timestamp of
the assistant `tool_use` *record* (model emission); the actual `git` process
spawn happens later, so each true run-time-before-kill is somewhat *shorter*
than the gap shown. No attempt's gc ever returned a `tool_result`.

**Dated correction to the Sep-2 docs.**
`bf-4x12ec-crash-artifacts-2026-09-02.md` §4 and
`bf-4x12ec-final-crash-report.md` say attempt 15 was "killed ~8 s after
launching gc". From the transcript timeline, 8.2 s is the gap between the
`free -h` **tool_result** (10:43:59.438) and the gc **tool_use record**
(10:44:07.643) — i.e. the pre-launch pause, not a launch→kill interval. The
verifiable launch→kill gap for attempt 15 is 45.6 s (record →
`agent.completed`). The correction does not weaken the conclusion — the
direction of the error runs the other way (the gc ran ~45 s, not ~8 s, with
45 Gi host-available throughout) — but the "~8 s" phrasing should not be
cited as a launch→kill measurement.

## 5. The memcg/cgroup OOM hypothesis

### 5.1 The boundary, verified live (2026-09-07)

```
systemctl --user show run-p<dispatch>.scope
  MemoryMax=12884901888        # exactly 12 GiB
  MemoryHigh=infinity
  Slice=needle.slice
~/.config/systemd/user/needle.slice.d/limits.conf
  MemoryMax=32G                # outer slice cap — the scope cap binds first
  MemoryHigh=24G
```

Every needle dispatch runs in its own transient `run-p<pid>-i<id>.scope`
(`systemd-run`), and this attempt confirmed on a live dispatch scope that the
per-scope `MemoryMax` is **12 GiB** — the same figure the canonical report's
Addendum 3 identified from Aug-14-era scopes. The outer `needle.slice` cap
(32 G) is larger, so a single runaway process inside a dispatch scope hits
the 12 GiB scope boundary first, while the host still has gigabytes free.

`oom_score_adj` was also re-verified live: **every dispatched `claude`
process (and the needle worker itself) runs with `oom_score_adj=200`** —
preferred OOM victims. The kernel lines in §5.3 carry the same
`oom_score_adj:200` marking, confirming the setting is long-standing, not
recent.

### 5.2 Why this gc crosses that boundary

`git gc --aggressive` recomputes full delta chains across the entire object
set in memory before emitting a pack. The repo measured **17.20 GiB of loose
objects / 18 G `.git`** inside the window (attempts 1, 2, 15 — identical
readings), i.e. an object set larger than the 12 GiB scope cap it was being
packed under. `pack.windowMemory` was unset on 2026-08-14 (attempt 1 had
*burned 36.5 s* discovering `gc.aggressivewindow` was malformed and unset it;
the window/delta/threads bounds that today cap pack-objects at ≈313 MiB peak
in the repo's own regression test — `scripts/test-gc-memory-bounds.sh` — did
not exist until 2026-09-02). An unbounded aggressive gc over a 17–18 GiB
loose set is exactly the workload the canonical report concludes cannot fit
under a 12 GiB cap (Addendum 3 §"the death"; the later bf-198ne push-side
twin exceeded the same cap the same way).

### 5.3 Kernel evidence: Aug-14 missing, Aug-16 proves the mechanism

**Aug-14 kernel journal is unrecoverable.** Re-verified live this attempt:

```
journalctl --list-boots
  0  52309698918d4ec1b8cf2680af8cbcb8  Sat 2026-08-15 19:56:33 EDT  Mon 2026-09-07 21:2x EDT
```

Exactly **one** surviving boot, whose first entry is
**2026-08-15T23:56:33Z** — ~36.5 h *after* the last bf-4x12ec kill
(2026-08-14T11:27:26Z). No kernel OOM record for the crash window can exist.
(This is the value child 1 re-verified; the older "19:26" reading in the
notes doc's confidence row is superseded.) Per the crash-response-guide's own
caveat, a systemd OOM *notice* without a kernel line in the same seconds is a
NixOS re-exec replaying stale counters, so the classification below rests on
corroboration and is stated as such — the canonical report's framing:
**regime-matched, not kernel-proven for Aug-14**.

**The Aug-16 twin is kernel-proven, and this attempt re-derived it live**
(window 2026-08-16T00:00:00Z–24:00Z, inside the surviving boot):

| Measurement | This attempt (live `journalctl`) | Prior record |
|---|---|---|
| `oom-kill:constraint=CONSTRAINT_MEMCG` kernel lines | **414** | Addendum 3/6 (257 git + others) |
| with `task=git` | **257** | "257 git OOM-kills" (Addendum 3, count corrected by Addendum 6) — **matches exactly** |
| git victims' `anon-rss` (257 `Killed process (git)` lines) | min 1.20 GiB · **median 11.73 GiB** · **max 11.97 GiB**; 163 of 257 ≥ 11 GiB | "11–12 GB ceiling — dying at a hard ceiling" |
| `oom_memcg`/`task_memcg` | `…/app.slice/run-p<pid>-i<id>.scope` | same transient dispatch-scope shape |
| victim `oom_score_adj` | `200` (in the kernel lines themselves) | preferred-victim marking |

The Aug-16 git processes died **inside dispatch-scope memcgs at a hard
~12 GiB RSS ceiling** (max 11.97 GiB = 99.8 % of 12884901888 bytes) — the
same scopes, the same memory class of git operation over the same
unrepaired 17–18 GiB repository, one kernel-recorded day after bf-4x12ec's
44 unrecorded deaths. That is the regime match. Nothing about bf-4x12ec's
window distinguishes it from the Aug-16 kills except the survival of the
kernel lines.

### 5.4 Alternative boundaries considered and rejected

- **`needle.slice` (32 G)** — larger than the scope cap; and the Aug-16
  kernel lines name `run-p*.scope` memcgs, not the slice.
- **systemd-oomd host pressure (94.71 % threshold)** — would kill across
  scopes on host pressure, contradicted by §3's 45–50 Gi available and by
  the kills being scoped to the gc's own memcg.
- **Agent `--max-turns 30`** — a turns cap exits 1 (`error_max_turns`
  class); the stream contains zero such records for this bead, and 43 of 44
  deaths landed mid-first-turn inside one outstanding tool call.
- **Disk exhaustion** — 67 G free at the capture; and `ENOSPC` exits 1 with
  stderr text, not a signal-death sentinel.

## 6. Timeout: excluded

Re-derived this attempt from the committed bundle (1,146 events, 53
`agent.completed`): **44 × exit −1** (crash, durations 38.9–115.8 s), **8 ×
exit 124** (timeout, durations 600,018–600,024 ms), **1 × exit 0** (attempt
53). Three independent reasons the −1s are not timeouts:

1. **Needle records its own deadline kill as 124** (GNU timeout convention),
   and all 8 of those are present and cleanly separated — the timeout
   machinery demonstrably reports through a different code than the 44.
2. The 44 died at **38.9–115.8 s**, 5–15× below the 600 s dispatch cap; none
   approached any ceiling.
3. The gc `tool_use` itself carried `timeout: 600000 ms` — the agent-side
   tool cap that *would* have produced a tool error, not a process death;
   no `tool_result` of any kind ever returned.

Phase 2 (attempts 45–52) *is* timeout-shaped, but it is a different
phenomenon — eight 600 s attempts with zero assistant records on a
CPU-saturated box — already characterized by the child-2 timeline §4, and it
produced no `exit −1` and no alert beads.

## 7. Classification per docs/crash-response-guide.md

**INFRASTRUCTURE — memcg-OOM SIGKILL inside the 12 GiB dispatch scope. Not a
domain-check code defect.**

- **Entry rule:** guide Phase 1 — "Exit code -1 → Infrastructure event (skip
  to Phase 2A)". All 44 deaths match.
- **Class definition:** the guide's classification table row —
  "INFRASTRUCTURE | memcg-OOM inside the dispatch scope, resource
  exhaustion, repository bloat | Check the cgroup boundary and repo size,
  verify work completion" — matches every clause, and the guide's
  Common-Infrastructure-Events list names bf-4x12ec explicitly as a memcg
  case "kernel-verified" by class (the per-instance kernel proof lives in
  the Aug-16 census, §5.3).
- **Not the other classes:**
  - *SERVICE_FAILURE* — no 5xx, no external service in the death path; the
    inference gateway answered throughout (the agents kept producing tool
    calls until the kill).
  - *FALSE_POSITIVE-by-completion* — at each death instant the bead's task
    was mid-flight inside the gc (§4); the work completed only later
    (12:58:45Z attempt-53 auto-split, and the actual gc under child
    bf-173o7e, closed 2026-08-17). The 44 *alert beads* minted one-per-kill
    are the false-positive layer, already dispositioned by the alert
    inventory; the *crashes* were real.
  - *CODE_DEFECT* — no application error in any of the 53 attempts; the
    crash trigger was the task body's own prescribed command (the bead was
    created to run this gc), and domain-check code is not in the death path.
    This matches the repo-wide finding that domain-check code is
    defect-free.
- **Phase 2A checklist disposition:**

  | Checklist item | Result |
  |---|---|
  | Verify task completion before classifying | ✅ completed by retry — `verification.passed` 12:58:45.126649351Z; bead closed 2026-08-17 rev 4 |
  | System-wide event check (`journalctl` oom/kill/memory) | ⚠️ **unrecoverable for Aug-14** (single surviving boot begins 2026-08-15T23:56:33Z, §5.3) — class rests on corroboration, stated as such |
  | **Check the cgroup boundary, not just the host** | ✅ the guide's own named check: host 45–50 Gi available in-window while the 12 GiB scope cap bound (§3, §5.1) — the guide's text cites this very bead |
  | 30-second-rule FALSE_POSITIVE check | ✅ n/a — every kill landed inside the gc, not post-commit cleanup |

- **Action per the guide: no code changes.** The mitigations are
  configuration and tooling, all already landed: `pack.windowMemory=2g` /
  `pack.deltaCacheSize=1g` / `pack.threads=1` (repo + global — bounds
  pack-objects under both gc and push), `scripts/safe-git-gc.sh` with
  check-only preflight and checkpoint/resume, the 10 MB pre-commit hook, and
  `.beads/` fully gitignored so the bloat that made the object set
  pack-hostile cannot recur. Verified in repo CLAUDE.md ("Mechanical guard
  for the bare-gc path"), including the bound bare-gc rerun of these exact
  death commands peaking at ≈313 MiB.

## 8. What this attempt adds

1. The **44-row gc-launch → kill correlation table** (§4) — previously only
   attempt 2 had a recorded gap (52.9 s, in the notes doc's resource-state
   section); the gap is now measured for every kill (18.6–80.6 s, mean
   30.7 s).
2. Live re-verification (2026-09-07) of the dispatch-scope `MemoryMax`
   (12884901888 B = 12 GiB), the outer slice caps, and `oom_score_adj=200`
   on dispatched agents (§5.1).
3. Live re-extraction of the Aug-16 kernel census with the git-only
   `anon-rss` distribution (median 11.73 GiB, max 11.97 GiB, 163/257 ≥
   11 GiB) — first-hand confirmation of the "hard ceiling" claim (§5.3).
4. The journal bound re-verified from `journalctl --list-boots` (one boot,
   first entry 2026-08-15T23:56:33Z), closing the "19:26 vs 19:56:33"
   discrepancy in the notes doc's confidence row in favor of 19:56:33 EDT
   (§5.3).
5. The dated correction to the "~8 s after launching gc" phrasing (§4).
6. Rejection of the alternative boundaries (slice cap, systemd-oomd,
   max-turns, disk) with the specific record each would have left (§5.4).

## Cross-references

- Canonical report (mechanism, Addenda 2/3/6):
  [`bf-4x12ec-crash-investigation.md`](bf-4x12ec-crash-investigation.md)
- Timeline, all 53 attempts (child 2 of this split):
  [`bf-4x12ec-crash-timeline-domchk-ba8584a1-2026-09-07.md`](bf-4x12ec-crash-timeline-domchk-ba8584a1-2026-09-07.md)
- Surviving log-source inventory (child 1):
  [`bf-4x12ec-log-source-inventory-domchk-a3f1f8f5-2026-09-07.md`](bf-4x12ec-log-source-inventory-domchk-a3f1f8f5-2026-09-07.md)
- Signal-sentinel semantics (−1 / "signal −1"):
  [`bf-4x12ec-evidence-signal-semantics-domchk-15854355-2026-09-07.md`](bf-4x12ec-evidence-signal-semantics-domchk-15854355-2026-09-07.md)
- Exit-code census + classification (prior split's exit/signal child):
  `docs/notes/bf-4x12ec-crash-investigation.md` §"Exit code and signal
  analysis" and §"Resource-state correlation" (`domchk-0e707410`)
- Evidence bundle: `docs/crashes/bf-4x12ec/` (README, MANIFEST.sha256,
  attempt-index.tsv, transcripts/, needle-events JSONL)
