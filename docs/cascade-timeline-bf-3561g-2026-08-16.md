# Cascade Crash Timeline — 2026-08-16 12:00–17:00 UTC, and where bf-3561g sits in it

**Bead:** domchk-786e838f ("Investigate original bead bf-3561g task and cascade patterns")
**Generated:** 2026-09-06
**Upstream:** [`docs/bead-bf-3561g-scope-and-original-task.md`](bead-bf-3561g-scope-and-original-task.md)
(task/scope side) · [`docs/crash-logs-catalog-bf-3561g.md`](crash-logs-catalog-bf-3561g.md)
(log-location side) · [`docs/crash-context-bf-3561g/MANIFEST.md`](crash-context-bf-3561g/MANIFEST.md)
(crash-#4 mechanism side)

This file is the **cascade timeline and dependency map** those three do not contain.

---

## 1. What bf-3561g was

`bf-3561g` — **"ALERT: Agent crash on bead bf-4k2ws"** — is a *crash-alert bead*, not a
work bead. Created **2026-08-13T03:58:25Z** (the same second as the reported
bf-4k2ws crash it alerts on), assigned to investigate a purported exit −1 crash on
`bf-4k2ws` ("Analyze divergent Forgejo and GitHub branch states").

That target crash never happened: `bf-4k2ws` completed successfully
(exit 0, 2026-08-16T15:35:42Z), so the alert was a **false positive** from the start.
bf-3561g then sat open for three days, was dispatched into the 2026-08-16 cascade,
crashed 9 times (§5), and finally succeeded at 17:31:56Z.

---

## 2. Headline numbers for the 2026-08-16 12:00–17:00 UTC window

Measured from `.beads/events.jsonl` (append-only, UTC) and system journald.

| Metric | Value |
|---|---|
| bead `crash` events in window | **177** |
| distinct beads crashing | **59** |
| distinct workers | **4** — lab-domain-check 98, lab-drawrace 34, lab-test-fix 30, lab-roam-1 15 |
| median run duration | 120.9 s (50% under 120 s) |
| kernel memcg OOM kills, 12:00–17:05 UTC (journald 08:00–13:05 local) | **295** across **283 distinct dispatch scopes** |
| kernel OOM victims | 193 `git` + 101 `node (vitest 8)` |
| crashes with a kernel OOM **in the same second** | 70 / 177 (40%) |
| crashes with a kernel OOM **within ±2 s** | **176 / 177 (99%)** |

> **Window-boundary note.** The frequently-quoted "**201 crashes**" for this event is
> **12:00:00 → 17:29:52**, i.e. the window extended to the end of bf-3561g's own crash
> chain — not 12:00–17:00, which is **177**. Both figures are real; they are different
> windows. Total for all of 2026-08-16 is 245: **44 before 12:00** and **none after** —
> 17:29:52, the end of bf-3561g's chain, is the last crash of the day.

### The mechanism is kernel memcg OOM, not SIGHUP

`exit_code −1` / `signal_code=-1` in the needle records is the outcome classifier's
**sentinel for an abnormal child death — not a signal number**. The journald record for
the same second names the actual mechanism:

```
oom-kill:constraint=CONSTRAINT_MEMCG ... task=git,pid=485129,uid=1001
Memory cgroup out of memory: Killed process 485129 (git)
  total-vm:10496992kB, anon-rss:9161772kB ... oom_score_adj:200
```

That line (journald `08:25:24` local) is the same second as bead `bf-9b8oe`'s crash
record `12:25:24.622Z`. Every pre-2026-09-06 "SIGHUP cascade" attribution for this
window — including the cascade section of
[`docs/bead-bf-3561g-scope-and-original-task.md`](bead-bf-3561g-scope-and-original-task.md)
as originally committed (591bb1e) — was wrong on mechanism; that document's cascade
section is corrected to this reading in the same commit that adds this timeline.

Victim memory profile confirms it is the **12 GiB per-dispatch `MemoryMax`** being hit:

| Victim | Count | anon-rss min / median / max | Reading |
|---|---|---|---|
| `git` | 193 | 1.20 / **11.73** / 11.97 GiB | 123/193 at ≥11 GiB — pinning the limit |
| `node (vitest 8)` | 101 | 2.01 / 4.69 / 8.21 GiB | a *different* workspace's JS test run — collateral, not a domain-check bead |

The `node (vitest 8)` victims never correspond to a domain-check crash record: they are
other repos on the same box losing workers to the same system-wide pressure. The
`git` victims are the domain-check (and sibling-workspace) repo operations.

---

## 3. Cascade timeline (minute resolution, 12:00–17:00 UTC)

This is a **sustained low-level cascade, not a single spike**. Peak minute is 3 crashes
(13:53 and 14:28); 32 different minutes have ≥2 *distinct* beads crashing simultaneously.

```
crashes per minute, UTC  (only minutes with >=2 shown; 1-crash minutes omitted)
12:44 ##   12:46 ##   12:51 ##
13:06 ##   13:08 ##   13:12 ##   13:14 ##   13:19 ##   13:21 ##
13:24 ##   13:25 ##   13:30 ##   13:32 ##   13:46 ##   13:49 ##
13:53 ###  13:57 ##
14:08 ##   14:28 ###  14:35 ##   14:48 ##
15:36 ##   15:39 ##   15:41 ##   15:45 ##   15:52 ##
16:01 ##   16:05 ##   16:07 ##   16:27 ##   16:30 ##   16:37 ##
16:50 ##   16:52 ##   16:59 ##
```

Quiet gaps ≥5 min: only six, the longest **36 min (14:57 → 15:33)** — a lull, not an
end. The cascade is continuous across the whole five hours.

---

## 4. Dependency map — the cascade is an alert-amplification loop

The decisive structural fact: **57 of the 59 distinct beads that crashed were
themselves `ALERT:` beads.** Only two were real work beads.

Each crash makes needle create a new crash-alert bead *and* auto-retry. When the box is
under memory pressure, those alert beads are dispatched too, run git operations against
the same bloated repository, and crash as well — which creates more alert beads. The
alert system was the cascade's **amplifier**, not just its reporter.

### Root subjects and their alert fan-out

| Root subject bead | Title | ALERT beads that crashed |
|---|---|---|
| **bf-1s6c3** | Create merge commit reconciling Forgejo and GitHub histories | **25** |
| **bf-4yjq** | Git origin remote points to GitHub directly; Forgejo mirror has diverged/gone stale | **12** |
| **bf-4k2ws** | Analyze divergent Forgejo and GitHub branch states | **7** |
| **bf-1ea4g** | Document local main branch state | **6** |
| **bf-2xygo** | Fetch and analyze divergence between Forgejo and GitHub remotes | **3** |
| **bf-ncxbt** | Document remote GitHub mirror state | **3** |
| **bf-574w1** | Identify divergence and write analysis document | **1** |
| **total** | 7 subjects | **57** |

Work beads that crashed directly: `bf-1vuk2` (Analyze git remote divergence — 17 crashes,
the single most-crashing bead in the window) and `bf-31p3g` (Create merge commit
reconciling both histories).

**All seven root subjects, plus both work beads, are the same piece of work:** the
Forgejo↔GitHub divergence analysis and merge reconciliation, running against a
repository that was at that point massively bloated (the bf-1s6c3 18 GB / 17 GB-loose-
objects condition). One concentrated job, one pathological repository, one memory
ceiling — hence every dispatch of it OOMing regardless of which bead ID carried it.

### Shape

```
                    bloated repo (18 GB, 17 GB loose objects)
                                   │
        ┌──────────────────────────┴───────────────────────────┐
        │  7 divergence/merge subject beads (bf-1s6c3, bf-4yjq,│
        │  bf-4k2ws, bf-1ea4g, bf-2xygo, bf-ncxbt, bf-574w1)   │
        └──────────────────────────┬───────────────────────────┘
               git op → 11.7 GiB anon → memcg OOM (12 GiB MemoryMax)
                                   │
                        agent exit −1  (sentinel)
                                   │
                ┌──────────────────┴───────────────────┐
                │  needle: auto-retry  +  new ALERT bead │
                └──────────────────┬───────────────────┘
                                   │  57 ALERT beads dispatched
                                   ▼
                    they too run git on the same repo → OOM
                                   │
                                   └──►  more ALERT beads  (loop)
```

bf-3561g is one node of this loop — the `bf-4k2ws` branch — and is itself both a
false-positive alert *and* a crasher.

---

## 5. bf-3561g's own chain, and the cascade tail (17:00–17:32 UTC)

bf-3561g is **absent from the 12:00–17:00 set**: its 9 crashes begin at 17:13:04Z, at
the cascade's tail. Chain per `.beads/events.jsonl`, interleaved with the other workers
still crashing:

| UTC | Bead | Worker | Duration | Note |
|---|---|---|---|---|
| 17:12:03.539 | bf-saupc | lab-roam-1 | 58 s | cascade still active |
| **17:13:04.749** | **bf-3561g** | lab-domain-check | 156 s | crash #1 |
| **17:14:39.565** | **bf-3561g** | lab-domain-check | 95 s | crash #2 |
| 17:14:58.062 | bf-w4fwe | lab-drawrace | 209 s | |
| 17:15:24.772 | bf-1fy2x | lab-roam-1 | 73 s | |
| **17:16:22.735** | **bf-3561g** | lab-domain-check | 103 s | crash #3 |
| 17:17:09.583 | bf-w4fwe | lab-drawrace | 130 s | |
| 17:18:00.339 | bf-1fy2x | lab-roam-1 | 154 s | |
| **17:21:28.132** | **bf-3561g** | lab-domain-check | **305 s** | **crash #4 — the target event** |
| 17:21:31.699 | bf-6bio4g | lab-drawrace | 261 s | **3.5 s after #4**, different worker |
| **17:23:14.381** | **bf-3561g** | lab-domain-check | 106 s | crash #5 |
| **17:24:42.528** | **bf-3561g** | lab-domain-check | 88 s | crash #6 |
| **17:25:31.542** | **bf-3561g** | lab-domain-check | 49 s | crash #7 |
| **17:27:14.745** | **bf-3561g** | lab-domain-check | 103 s | crash #8 |
| **17:29:52.577** | **bf-3561g** | lab-domain-check | 158 s | crash #9 (last) |
| **17:31:56.062** | **bf-3561g** | lab-domain-check | 123 s | **exit 0 — success** |

Each crash is followed within ~15 ms by `claim` + `dispatch` (auto-retry), so these are
one continuous retry chain on one worker — not nine independent failures.

**Crash #4 specifically** was not a passive victim: that run's own agent invoked
`git gc --aggressive --prune=now` at 17:19:23Z, the gc grew to ~11.7 GiB anon inside the
12 GiB dispatch scope, and the kernel memcg OOM killed it. Five seconds earlier
(17:21:22Z) systemd-oomd had killed a *different* dispatch scope because user.slice
memory pressure was 94.29% — two concurrent scopes resident, one reaped by oomd, one by
the kernel. Full evidence: [`docs/crash-context-bf-3561g/MANIFEST.md`](crash-context-bf-3561g/MANIFEST.md).

---

## 6. Crash sequence, condensed

1. **Precondition** — repository bloated to 18 GB with 17 GB of loose objects.
2. **Trigger** — seven related Forgejo/GitHub divergence-and-merge beads dispatched;
   every `git` operation on that repo balloons toward ~11.7 GiB anon.
3. **Kill** — kernel memcg OOM inside the dispatch's 12 GiB `MemoryMax`
   (`oom_score_adj:200`, `CONSTRAINT_MEMCG`). 295 such kills in the window.
4. **Classification** — needle's classifier maps the abnormal death to the `−1`
   sentinel, recorded as `outcome: crash`.
5. **Amplification** — needle auto-retries *and* creates a new `ALERT:` bead.
6. **Re-dispatch** — the alert beads run the same kind of git work on the same repo and
   OOM too. 57 of them crash, fanning out from 7 subjects (§4).
7. **Tail** — pressure finally eases after ~17:30; bf-3561g's 10th dispatch succeeds at
   17:31:56Z.

**There is no causal dependency between the crashes** — no crash caused another through
the bead dependency graph. The "cascade" is **shared-infrastructure coupling**: every
concurrently-running dispatch was drawing from the same host memory pool against the
same bloated repository, so each kill raised pressure on the survivors. That is why the
signal shows up simultaneously on four unrelated workers (§2) and why a bead's crash
probability depended only on what *it* was doing, not on which other bead had just
died.

---

## 7. Corrections this timeline makes to earlier documents

| Earlier claim | Status | Correct reading |
|---|---|---|
| "Signal −1 (SIGHUP)" / "hangup detected on controlling terminal" | **wrong** | `−1` is needle's sentinel for abnormal child death; the kernel record shows memcg OOM |
| "caused by terminal session closure / systemd service management" | **wrong** | per-dispatch-cgroup memory exhaustion |
| "201 crashes, 2026-08-16 12:00–17:00" | **mislabelled** | 177 in 12:00–17:00; **201** is 12:00 → 17:29:52 |
| "bf-3561g … Creation Date: 2026-08-16" | **wrong** | created **2026-08-13**T03:58:25Z; its crashes are 08-16 |
| "caught in a system-wide SIGHUP cascade" (crash #4) | **incomplete** | crash #4 was its own `git gc --aggressive` hitting the 12 GiB scope limit |

The false-positive *finding* those documents report (bf-4k2ws never crashed; no work was
lost) is unaffected — only the mechanism attribution changes.

---

## 8. Verification

All figures re-derived on 2026-09-06 from primary sources:

- crash counts, durations, worker/bead splits, alert fan-out — parsed from
  `.beads/events.jsonl` (UTC) and titles from `.beads/checkpoint/forensic.jsonl`;
- kernel kills — `journalctl -k --since "2026-08-16 08:00:00" --until
  "2026-08-16 13:05:00"` (journald stamps **local**, UTC−4; 295 `oom-kill:` lines);
- correlation — crash timestamps shifted −4 h and matched against kill seconds;
- bead identities/statuses — `bead show` for all 7 root subjects, both work beads,
  and bf-3561g.

Counts are reproducible with the window predicates in §2 (`12:00:00 ≤ ts < 17:00:00`
on `event == "crash"`).
