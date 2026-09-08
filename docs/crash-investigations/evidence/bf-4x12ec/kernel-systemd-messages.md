# bf-4x12ec — Kernel & systemd Messages for the Signal −1 Event

Investigation bead: **domchk-ad80e265** · Extracted: **2026-09-07** (21:38–21:55 EDT)

Companion to `crash-logs/` (domchk-48f3e34d), `operation-summary.md`
(domchk-dfce2360), `../system-state.md` (domchk-40c9c99a), and
`docs/crash-investigations/bf-4x12ec-evidence-signal-semantics-domchk-15854355-2026-09-07.md`
(§1, signal semantics). Every journal figure below was queried live on
2026-09-07; the exact commands are in
[Queries executed](#queries-executed-2026-09-07) so the extraction is
reproducible.

## Answer

**No kernel or systemd record of the bf-4x12ec crash window survives.** The
window — 2026-08-14 10:21:06Z → 11:27:26Z (06:21:06 → 07:27:26 EDT) — is
~36.5 hours **before** the first entry of the only journal boot left on this
box (2026-08-15 19:56:33 EDT). Three reboots separate the crash from the
surviving journal, and no journal data from any earlier boot exists in either
the persistent (`/var/log/journal`) or volatile (`/run/log/journal`) store.
`journalctl -k` and `journalctl --user` over the window both return
`-- No entries --`.

What this document can still establish, from records that *do* survive:

1. **Which signal:** SIGKILL. `exit_code = −1` is needle's sentinel for a
   worker process that died without an exit status — not a literal signal
   number. Needle's own worker log states the interpretation
   (["Signal −1 context"](#signal--1-context)).
2. **Sender:** the kernel's memory-cgroup OOM killer inside the needle
   dispatch scope — not systemd, not a userspace process, and not the
   host-wide OOM killer. Every one of the **525** kernel OOM-kill records in
   the surviving journal is `constraint=CONSTRAINT_MEMCG`; there is not a
   single host-level (`CONSTRAINT_NONE`) OOM anywhere in it, and 375 of the
   525 sit at exactly `usage 12582912kB, limit 12582912kB` — the 12 GiB
   `MemoryMax` of the dispatch scope.
3. **Cgroup/unit:** the kernel names the victim memcg explicitly —
   `/user.slice/user-1001.slice/user@1001.service/app.slice/run-p<id>-i<id>.scope`,
   a transient `systemd-run` scope under the user manager, i.e. the per-
   dispatch scope. systemd's counterpart line comes from PID 1
   (`user@1001.service: A process of this unit has been killed by the OOM
   killer.`); the transient scope itself never logs a
   `Failed with result 'oom-kill'` line, unlike named scopes.
4. **The exact message shape the Aug-14 kill produced:** recoverable verbatim
   from the same journal's later storms of the same mechanism — 414 kills on
   Aug 16 alone, including the `task=git` kills bracketing bf-198ne's
   documented death instant. That is the regime-match used throughout the
   bf-4x12ec docs; this document is where the matching records live.

## Crash window queried

From `crash-logs/README.md` ("Crash timestamp and exit signal"). Journal
stamps on this box are **local time (EDT, UTC−4)**; needle stamps are UTC.

| Event | UTC | Local (EDT) |
|---|---|---|
| Attempt 1 claims bf-4x12ec (window start) | 2026-08-14 10:21:06.969Z | 06:21:06.969 |
| Crash #1 — `agent.completed`, exit −1 | 2026-08-14 10:23:02.958Z | 06:23:02.958 |
| First alert heartbeat (bead bf-fmg2cw's `Timestamp:`) | 2026-08-14 10:23:11.219Z | 06:23:11.219 |
| Attempt 2 kill (the alert bead bf-3m9m1v names) | 2026-08-14 10:25:01.512Z | 06:25:01.512 |
| Last of the 44 kills (window end) | 2026-08-14 11:27:26Z | 07:27:26 |

## Queries executed (2026-09-07)

```bash
# 1. kernel journal, crash window (note: --since/--until are LOCAL time here)
journalctl -k --since "2026-08-14 06:00:00" --until "2026-08-14 12:00:00" --no-pager
#   → "-- No entries --"   (exit 0)

# 2. user journal, crash window
journalctl --user --since "2026-08-14 06:00:00" --until "2026-08-14 12:00:00" --no-pager
#   → "-- No entries --"   (exit 0)

# 3. full-journal greps for the task's target patterns (both -k and --user)
#    oom-kill / "Out of memory" / "Killed signal" / SIGKILL / needle unit names
#    → results in the two sections below
```

## Journal coverage gap — stated explicitly

| Store | Coverage |
|---|---|
| System journal (`journalctl`, root view) | **Single boot** `52309698918d4ec1b8cf2680af8cbcb8`, first entry **2026-08-15 19:56:33 EDT** (23:56:33Z), last entry current. 55 journal files under `/var/log/journal/7a972da797484700baa6cb3782546e05/`, all belonging to that boot. |
| User journal (`journalctl --user`) | Same boot id, first entry **2026-08-17 15:33:14 EDT** — later still (the user manager's earliest surviving records). |
| Volatile store | `/run/log/journal/` exists but is **empty** (dir dated Aug 15 09:48 — current boot). No volatile copies of older boots. |
| `wtmp` reboot rows | Reboots recorded **Aug 14 16:39**, **Aug 14 21:41**, **Aug 15 09:48** (EDT) — all rows read "still running" because wtmp is not written at shutdown. The crash ran under the boot started **2026-08-12 01:15 EDT**, corroborated by attempt 2's own `uptime` output captured 8 s before its kill: *"up 2 days"* (`operation-summary.md` timeline, 10:23:56Z). |
| Gap | Last kill 2026-08-14 11:27:26Z → first surviving entry 2026-08-15 23:56:33Z = **36 h 29 m 07 s of missing journal**, spanning three boot boundaries. The current boot's own first ~10 h are also absent (wtmp says it began 09:48 EDT; first journal entry 19:56:33 EDT). |

> **Dated correction to `crash-logs/README.md`:** the README's "Sources
> searched" table says the single surviving boot starts "2026-08-15 19:26
> EDT". The live re-check on 2026-09-07 21:4x EDT puts it at **19:56:33 EDT**
> — the boundary has moved 30 min later since the README was written that
> morning (more of the early boot has since been rotated/vacuumed). The
> conclusion is unchanged and in fact stronger: the crash window precedes
> *all* surviving records, by an even wider margin.

Because no journal file predating the current boot exists anywhere on disk,
**no amount of journalctl options can recover the Aug-14 kernel lines** —
this is a storage-boundary gap, not a query gap. The 2026-09-02 artifacts
survey reached the same verdict (`bf-4x12ec-crash-artifacts-2026-09-02.md`);
this document re-proves it live rather than inheriting it.

## Signal −1 context

**Which signal — SIGKILL, by regime-match.** `exit_code = −1` is needle's
sentinel for "worker process died without an exit status"; the signal-semantics
doc (`bf-4x12ec-evidence-signal-semantics-domchk-15854355-2026-09-07.md` §1)
establishes it is *not* a literal signal number. Needle's own service output,
captured verbatim from the surviving user journal, states the interpretation:

```
Aug 25 07:25:34 lab needle[2952869]: This indicates the worker was killed by an
    external process (e.g., SIGKILL, OOM, capacity governor)
```

(the same line recurs at 07:26:15, 07:28:49, 07:31:29, … — the Aug-25 fleet
wave). The kernel-side records that survive from the same mechanism family all
show the sender is **the kernel's memcg OOM killer** (`oom_kill_process`,
reached via `mem_cgroup_out_of_memory` → `try_charge_memcg` — an anonymous-page
charge that crossed the cgroup limit), which kills with **SIGKILL**. systemd is
not the sender: PID 1 merely *reports* the kernel's kill, and no
`Killed process ... with signal SIGKILL` line from systemd unit-stop exists for
the crash window (unit-stop SIGKILLs of the *worker units* do appear in the
user journal — e.g. `needle-worker@lab-domain-check.service: Killing process
1143784 (needle) with signal SIGKILL.` on Aug 17 16:55:38 EDT — those are
deliberate restarts, a different thing entirely).

**Sender's cgroup/unit.** The kernel names it explicitly in every record:

```
oom_memcg=/user.slice/user-1001.slice/user@1001.service/app.slice/run-p<id>-i<id>.scope
```

That is a transient `systemd-run` scope under the user manager — the needle
**dispatch scope** (12 GiB `MemoryMax`, verified live by Addendum 3 of the
canonical RCA). Two systemd-side facts about it, both verified live:

- PID 1 attributes the kill one level up, to the parent user unit:
  `user@1001.service: A process of this unit has been killed by the OOM
  killer.` — **514** such lines in the surviving journal (839 OOM notices
  across all units). The scope-level variant `run-p*.scope: Failed with
  result 'oom-kill'` appears **0** times for transient dispatch scopes, in
  either journal — that message shape exists only for *named* scopes (49
  `safe-git-gc-*` lines in the user journal, plus the synthetic
  reproduction scopes `bf1s6c3-gc-a3-*`, `bf1s6c3-push-a2-*`,
  `bf4yjq-crash-*`, `gcmb-bare-aggressive-*`, `mw-oom-*`, `probe-hog-*`,
  `run-isolated-*`). So on this box, the per-dispatch OOM evidence shape is:
  kernel `oom-kill:constraint=…` lines + PID 1's `user@1001.service` notice —
  never a scope unit message.
- The worker unit `needle-worker@lab-domain-check.service` itself has **zero**
  OOM mentions. The worker survives every kill — it is the agent process
  inside the per-dispatch scope that dies, and the worker is what classifies
  the exit, mints the alert bead, and re-claims the bead (the 13.8 s
  claim-to-claim loop in `operation-summary.md`).

**Regime-match table (what each layer records for this mechanism):**

| Layer | Record | Survives for Aug 14? |
|---|---|---|
| Kernel | `git invoked oom-killer` / `oom-kill:constraint=CONSTRAINT_MEMCG` / `Memory cgroup out of memory: Killed process NNNN (git) …` | **No** (boot gap) |
| systemd PID 1 | `user@1001.service: A process of this unit has been killed by the OOM killer.` | **No** (boot gap) |
| User manager | (nothing — transient scopes do not log an oom-kill result) | n/a |
| Needle worker log | `agent.completed` `exit_code: −1` → `outcome.classified: crash` → alert bead | **Yes** — `crash-logs/needle-worker-log-*.jsonl` |
| Session transcript | final `tool_use` → `git gc --aggressive --prune=now`, no `tool_result` | **Yes** — `crash-logs/transcript-attempt1-crash-8b2a5b0d.jsonl` |

## What the missing Aug-14 records said — regime-matched from the same journal

The surviving journal contains **525** kernel memcg-OOM kills (all
`CONSTRAINT_MEMCG`, **0** host-level OOM). The earliest is
**Aug 16 00:27:35 EDT** — still ~41 h after the bf-4x12ec window, so none of
them is a bf-4x12ec record. But they are the same mechanism, same scope
family, same host, and they pin the exact message shape the 44 Aug-14 kills
each produced.

Verbatim kill record, Aug 16 00:27:35 EDT (earliest surviving; eliding only
the register dump and per-task table rows):

```
Aug 16 00:27:35 lab kernel: git invoked oom-killer: gfp_mask=0xcc0(GFP_KERNEL), order=0, oom_score_adj=200
Aug 16 00:27:35 lab kernel: CPU: 5 UID: 1001 PID: 3322810 Comm: git Not tainted 6.12.63 #1-NixOS
Aug 16 00:27:35 lab kernel: Hardware name: Dell Inc. OptiPlex 3000/0R7HRW, BIOS 1.9.1 01/18/2023
Aug 16 00:27:35 lab kernel: Call Trace:
Aug 16 00:27:35 lab kernel:  <TASK>
Aug 16 00:27:35 lab kernel:  dump_stack_lvl+0x5d/0x90
Aug 16 00:27:35 lab kernel:  dump_header+0x43/0x1c0
Aug 16 00:27:35 lab kernel:  oom_kill_process.cold+0x8/0x87
Aug 16 00:27:35 lab kernel:  out_of_memory+0x218/0x500
Aug 16 00:27:35 lab kernel:  mem_cgroup_out_of_memory+0x133/0x150
Aug 16 00:27:35 lab kernel:  try_charge_memcg+0x493/0x640
Aug 16 00:27:35 lab kernel:  __mem_cgroup_charge+0x42/0xe0
Aug 16 00:27:35 lab kernel:  do_anonymous_page+0x383/0x8c0
Aug 16 00:27:35 lab kernel:  __handle_mm_fault+0xb3c/0xfc0
Aug 16 00:27:35 lab kernel:  handle_mm_fault+0xe2/0x2d0
Aug 16 00:27:35 lab kernel:  do_user_addr_fault+0x227/0x640
Aug 16 00:27:35 lab kernel:  exc_page_fault+0x71/0x160
Aug 16 00:27:35 lab kernel:  asm_exc_page_fault+0x26/0x30
Aug 16 00:27:35 lab kernel: memory: usage 12582912kB, limit 12582912kB, failcnt 48499
Aug 16 00:27:35 lab kernel: Memory cgroup stats for /user.slice/user-1001.slice/user@1001.service/app.slice/run-p3295453-i208789885.scope:
Aug 16 00:27:35 lab kernel: oom-kill:constraint=CONSTRAINT_MEMCG,nodemask=(null),cpuset=user.slice,mems_allowed=0,oom_memcg=/user.slice/user-1001.slice/user@1001.service/app.slice/run-p3295453-i208789885.scope,task_memcg=/user.slice/user-1001.slice/user@1001.service/app.slice/run-p3295453-i208789885.scope,task=git,pid=3322486,uid=1001
Aug 16 00:27:35 lab kernel: Memory cgroup out of memory: Killed process 3322486 (git) total-vm:13847248kB, anon-rss:12301364kB, file-rss:4764kB, shmem-rss:0kB, UID:1001 pgtables:24432kB oom_score_adj:200
```

Paired systemd-side record, same second (PID 1, system journal):

```
Aug 16 00:27:35 lab systemd[1]: user@1001.service: A process of this unit has been killed by the OOM killer.
```

Reading: `usage == limit == 12582912 kB` = **exactly 12 GiB** — the dispatch
scope's `MemoryMax`, not host memory. `anon-rss` of the killed `git` is
~11.7–11.9 GiB across records. The call trace is an anonymous-page charge
(`do_anonymous_page` → `try_charge_memcg` → `mem_cgroup_out_of_memory`), i.e.
git was allocating heap when the charge failed. This is the precise shape the
bf-4x12ec gc kills would have produced.

### The bf-198ne correlation (same family, kernel-proven)

bf-198ne (the `git push` variant of this mechanism, 2026-08-16) is the one
family member whose kernel records **do** survive, and its canonical report
(`docs/crashes/bf-198ne-crash-report.md`) pins the second agent death at
**2026-08-16T13:30:50Z** = 09:30:50 EDT. The journal holds `task=git`
memcg kills 38 and 1 second before that instant, at the same 12 GiB bound:

```
Aug 16 09:30:12 lab kernel: oom-kill:constraint=CONSTRAINT_MEMCG,…
Aug 16 09:30:12 lab kernel: Memory cgroup out of memory: Killed process 972688 (git) total-vm:13677848kB, anon-rss:12339404kB, …
Aug 16 09:30:49 lab kernel: memory: usage 12582912kB, limit 12582912kB, failcnt 95438
Aug 16 09:30:49 lab kernel: Memory cgroup out of memory: Killed process 974979 (git) total-vm:13295044kB, anon-rss:12245348kB, …
```

(13:30:12Z / 13:30:49Z.) The bf-198ne report's §5.1 figures
(`usage 12582912kB, limit 12582912kB`, `task=git`,
`oom-kill:constraint=CONSTRAINT_MEMCG`) re-verify against the journal here,
line for line.

### Census of the 525 surviving kills

By day (EDT):

| Day | Kills | Note |
|---|---|---|
| Aug 16 | **414** | Matches the bf-198ne report's own count of "414 memcg kills that day". Hourly: 00h 12, 01h 11, 02h 39, 03h 3, 06h 17, 08h 44, 09h 71, 10h 61, 11h 34, 12h 79, 13h 43. |
| Sep 02 | 15 | |
| Sep 06 | 33 | |
| Sep 07 | 63 | |
| Aug 14 | **0** | **The bf-4x12ec window. No record survives.** |
| Aug 15 | **0** | Current boot's first 4.5 h also OOM-free (first kill overall is Aug 16 00:27:35). |

By killed task: `git` 307 · `node (vitest …)` 126 · `bash` 57 · `node` 16 ·
`esbuild` 9 · `sh` 3 · `tr` 2 · `claude` 2 · `python3` 1 · `head` 1 ·
`base64` 1. **375** of the 525 records sit at exactly
`usage 12582912kB, limit 12582912kB` (the rest are smaller synthetic test
scopes: 64 MiB ×51, 512 MiB ×49, 20 MiB ×7, 256 MiB ×2, near-limit variants
of the 12 GiB scopes). 401 distinct `run-p<id>-i<id>.scope` memcgs appear —
one OOM per dispatch scope, max 4 in any single scope.

## Negative findings (patterns the task asked for, and their counts)

| Pattern (task list) | Kernel journal | User journal |
|---|---|---|
| `oom-kill` in the crash window | **0** — window predates all records | **0** — same |
| `Out of memory` in the crash window | **0** — same | **0** — same |
| `Killed signal` (any time) | **0** — SIGKILL-by-OOM is logged as `Memory cgroup out of memory: Killed process …`, never as a signal-delivery line | 0 |
| `SIGKILL` (any time) | 0 | present but **not OOM**: systemd unit-stop lines (`needle-worker@*.service: Killing process … with signal SIGKILL`, Aug 17 restarts) and `Killed unit cgroup with SIGKILL on client request` (Sep 01, client-requested) |
| Host-level (non-memcg) OOM, any time | **0** — all 525 kills are `CONSTRAINT_MEMCG`; the host OOM killer never fired in this boot | — |
| needle/worker unit name + OOM | — | **0** for `needle-worker@lab-domain-check.service` |
| Non-OOM kernel kills, any time | 2 × `bash segfault` (Sep 06 12:56/12:57, unrelated test-harness process) | — |
| `oom`/memory events in the needle worker log itself | — | 0 (per `crash-logs/README.md` "Sources searched"; re-confirmed there) |

The zero host-level OOM count is itself evidence: it confirms the Aug-14
kills were **scope-local**, exactly as `system-state.md`'s mid-storm
`free -h` capture implies (45Gi available, 0B swap — the host was never
exhausted).

## Cross-reference — "Document the operation in progress when bf-4x12ec was killed"

`operation-summary.md` (domchk-dfce2360) establishes from the transcripts
that the operation in progress at every one of the 44 kills was
`git gc --aggressive --prune=now` (issued at 10:24:08.660Z on attempt 2,
killed 52.9 s in; 44/44 attempts end in that unanswered `tool_use`). The
kernel records in this document are consistent with that identification in
four specific ways:

1. **Task name matches the operation.** The regime-matched kills are
   `task=git`, `Comm: git` — the exact binary the transcripts show was
   running when the worker died.
2. **Victim memory matches the operation's known footprint.** Killed `git`
   processes show `anon-rss` ≈ 12.3 GB against the 12 GiB bound — and
   `docs/maintenance/repository-maintenance-guide.md`'s bounded-vs-unbounded
   measurements put unbounded aggressive repack of this repo's object
   inventory exactly in that range.
3. **Kill latency matches the storm's per-attempt profile.** The journal's
   git kills recur tens of seconds apart, the same order as the storm's
   18.6–80.6 s (mean 30.8 s) gc-runs-before-kill spread.
4. **Scope family matches.** All kills are inside
   `user@1001.service/app.slice/run-p*.scope` — the dispatch scope the
   canonical RCA's Addendum 3 verifies carries the 12 GiB `MemoryMax`.

The journal gap therefore does not weaken the operation identification —
it only means the 44 Aug-14 instances of that record were destroyed with
their boot. For bf-4x12ec specifically the mechanism remains
**regime-matched, not kernel-proven** (as `crash-logs/README.md` and
`operation-summary.md` already state); the kernel-proven member of the family
is bf-198ne, whose records this document re-verified live.

## Verification performed (2026-09-07)

- Every journal figure queried live on 2026-09-07 21:38–21:55 EDT with the
  commands shown above (plus `--list-boots` for both stores, `last -x reboot`,
  `ls /var/log/journal/*/`, `ls -la /run/log/journal/`).
- Kill counts re-derived two ways (grep `invoked oom-killer` and
  `oom-kill:constraint` → both 525); the Aug-16 daily count (414) cross-checks
  against the bf-198ne report's independent figure.
- The Aug 16 00:27:35 block and the 09:30:12/09:30:49 lines are quoted from
  `journalctl -k` output as emitted; only the register-dump and per-task table
  rows are elided, and the elision is marked.
- Journal first-entry timestamps read from `journalctl --list-boots` (both
  stores agree on boot id `52309698…`); the README's "19:26 EDT" figure is
  corrected to the live 19:56:33 EDT above.
- Local-vs-UTC handling: `timedatectl` confirms `America/New_York (EDT, -0400)`;
  all `--since/--until` windows were constructed in local time and every
  journal quote is shown with its native local stamp alongside its UTC
  equivalent.

## Files

| File | What it is |
|---|---|
| `docs/crash-investigations/evidence/bf-4x12ec/kernel-systemd-messages.md` | this document |
| `docs/crash-investigations/evidence/bf-4x12ec/crash-logs/` | raw crash-log extraction (domchk-48f3e34d) |
| `docs/crash-investigations/evidence/bf-4x12ec/operation-summary.md` | the operation in progress at kill time (domchk-dfce2360) |
| `docs/crash-investigations/evidence/bf-4x12ec/system-state.md` | window resource metrics (domchk-40c9c99a) |
| `docs/crashes/bf-198ne-crash-report.md` | the kernel-proven member of the mechanism family |
