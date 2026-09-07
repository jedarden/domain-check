# Signal -1 and Exit Code -1 Analysis

**Created:** 2026-09-02  
**Investigation Bead:** domchk-e4a11c19  
**Updated:** 2026-09-07 (bead domchk-15999f2c — §1–§6 below the 2026-09-02 summary are the verified technical record; every load-bearing figure re-executed a second time the same day by the bead's closing attempt — deviations found are corrected inline and summarized at the end of §6)  
**Purpose:** Understand what signal -1 means and what causes exit code -1 in agent environments

---

## Executive Summary

**Exit code -1** is NOT a standard Unix signal. In the context of NEEDLE agent crashes, **exit code -1 indicates infrastructure-level process termination**, typically caused by:
1. **OOM Killer** (system memory exhaustion)
2. **SIGHUP cascade** (system-wide signal to all processes)
3. **External process kill** (systemd, container orchestration)
4. **Resource exhaustion** (memory, CPU, disk)

**Key Finding:** Exit code -1 is **NOT a code defect** - it's a signal that the process was terminated by the operating system or infrastructure layer, not by application code.

---

# 2026-09-07 update — what exit -1 technically denotes (bead domchk-15999f2c)

Everything in §1–§6 was re-executed first-hand on 2026-09-07 (commands run, source
read, records parsed). The 2026-09-02 analysis below is retained unchanged; claims it
makes that later evidence contradicts are listed in [§5](#5-corrections-to-the-2026-09-02-sections-below).

**TL;DR**

- `-1` **cannot be a real exit status** — a POSIX exit status is 8 bits (0–255). It is
  a *writer-side sentinel* meaning "the child did not exit normally".
- In NEEDLE (Rust) the sentinel comes from `std::process::ExitStatus::code()`, which
  returns `None` when the child died by signal; needle normalizes `None → -1`
  (`src/dispatch/mod.rs:1861,1905,2082,2579`). **Every signal flattens to -1** —
  SIGHUP, SIGKILL and SIGTERM are indistinguishable in the recorded value.
- **"signal -1" is not a signal.** It is needle's alert renderer applying
  `signal_num = code > 128 ? code − 128 : code` (`src/outcome/mod.rs:1502`) to the
  sentinel. The alert body literally reads `- **Exit code**: -1 (signal -1)` and the
  alert bead gets the literal label `signal--1` — verified on alert beads
  bf-56kmlk, bf-3dxljn, bf-x88dnf, bf-1pidqn, bf-1cezsk (the first three Open at
  citation time; bf-1pidqn Open; bf-1cezsk Closed — the records persist either
  way). Linux signals number 1–64;
  there is no signal −1.
- The **true signal number is not persisted anywhere numeric** in needle's records.
- Measured across this workspace's authoritative event log (`.beads/events.jsonl` —
  a **live** log: 30,808 events when first measured, 31,101 on the same-day
  re-verification, 31,468 on a third same-day pass, the crash/timeout/sighup counts
  unchanged throughout): **all 247 `crash` records
  carry `exit_code: -1`**, all 22 `timeout` records carry `124`, failures carry
  `1` (1,195 records as of the third pass — a live figure, growing only with the
  service-class `exit 1` waves; 209 `0`-exit records — verification-gate failures).
- The dominant **proven** cause on this box is a **memcg OOM SIGKILL inside a
  12 GiB dispatch scope**: all 525 kernel OOM events in the journal's retained
  window (2026-08-16 → 2026-09-07) are `constraint=CONSTRAINT_MEMCG` — not one
  system-wide OOM. Victims named by the kernel: `task=git` (307), `task=node`
  (157 — the agent runtime), `task=bash` (60), `task=python3` (1). The window's
  first kernel kill (Aug 16 00:27:35 EDT = 04:27:35Z — journalctl stamps local,
  needle stamps UTC) and events.jsonl's first crash record (04:27:36Z) are one
  second apart: two independent clocks agreeing on the same event.
- **Timeout kills never surface as -1.** Needle kills the process group on its
  hard/idle deadline but deliberately records **124** ("GNU timeout convention",
  `src/dispatch/mod.rs:103`) plus a `timeout_reason` object in the trace metadata.
- The historically claimed **"SIGHUP cascade" has zero log support**: 0
  sighup/hangup records in `.beads/events.jsonl`. The 2026-08-16 wave the 2026-09-02
  text attributes to SIGHUP was kernel memcg OOM — the classifier says so itself
  (`scripts/crash-classifier.sh:466` at HEAD e1e5bf0; the same numbers held at the
  earlier HEAD 14e301d).

## 1. What a negative exit code represents

A process's exit status as seen by its parent is **8 bits, 0–255**. The kernel's
`wait` status is a different encoding that distinguishes two death modes:

- `WIFEXITED` — the process called `exit(n)`; the parent sees `WEXITSTATUS = n` (0–255)
- `WIFSIGNALED` — the process was killed by signal N; the parent sees `WTERMSIG = N`
  and **no exit code at all**

Shells bridge the two by reporting a signal death as `128 + N`. Verified live on
this box:

```console
$ bash -c 'exit -1'; echo $?        # → 255   (a negative value wraps; -1 is never stored)
$ bash -c 'kill -9 $$'; echo $?     # → 137   (SIGKILL = 128 + 9)
$ bash -c 'kill -1 $$'; echo $?     # → 129   (SIGHUP  = 128 + 1)
```

So `-1` never crosses the kernel/shell boundary. When a harness *record* shows `-1`,
it is the recording language's convention for "no exit code" — or, for one family of
writers, "signal N" with N negated. All four relevant conventions, verified on this
box (Rust from the source, the other three by running them):

| Recorder | Child died by signal | Child exited `exit 1` | Note |
|---|---|---|---|
| **Rust** `ExitStatus::code()` (NEEDLE) | `None` → needle writes **-1** | `Some(1)` | real signal available via `.signal()` but dropped |
| **Go** `os.ProcessState.ExitCode()` | **-1** (verified: SIGKILL → -1) | `1` | same shape as needle's, by API contract |
| **Python** `subprocess` `.returncode` | **−N**: SIGHUP → **-1**, SIGKILL → **-9** (verified) | `1` | a Python-written `-1` means SIGHUP *specifically* |
| **shell** `$?` | `128+N` (129 / 137) | `1` | `exit -1` wraps to 255 |

**Consequence: the same printed `-1` means different things depending on the writer.**
In needle records it means "died by signal, or was killed before it could exit —
signal unknown". A `-9` in a record can only come from a Python-convention writer;
needle can never produce it. (A pair of hand-authored fixtures dropped into
`.beads/traces/` at 2026-09-07T17:54Z — `bf-test-oom-001` with `exit_code: -9` and
`bf-test-sighup-001` with `exit_code: -1`, minimal schema, no `agent`/`captured_at`
fields — is exactly this Python-convention shape, and is **test data, not fleet
data**; exclude both from any census.)

## 2. How the NEEDLE harness records it

Source pinned at `~/NEEDLE` commit `9d09220` (clean tree; installed binary
`needle 0.6.0`, built 2026-09-01T13:47:48Z). Needle spawns the agent process
(e.g. claude-code on Node) as its child and reaps it; the recorded `exit_code`
describes **that child**.

**Where -1 comes from**

- `src/dispatch/mod.rs:1861,1905,2082,2579` — `status.code().unwrap_or(-1)`.
  `ExitStatus::code()` is `None` for a signal death (`src/test_runner.rs:432`:
  *"exit_code: Process exit code (None if killed by signal)"*), so the recorded
  value is -1 for **any** signal.
- `src/dispatch/mod.rs:1305,1698,1711,2111,2125` — literal `-1` written when needle
  never reaped a status at all: dispatch spawn error, transform feeder killed after
  drain / without drain. So `-1` covers "killed" *and* "no status was ever obtained".
- The one kill needle does **not** record as -1 is its own deadline kill:
  *"When a timeout fires, the process is killed with exit code 124 (GNU timeout
  convention)"* (`src/dispatch/mod.rs:103`; the `killpg(SIGKILL)` sites are
  `:1923-1926` idle and `:1952-1955` hard).

**How it is classified** (`src/types/mod.rs:415-432`, `Outcome::classify`):

| exit code | Outcome |
|---|---|
| 0 | Success |
| 1 | Failure |
| 124 | Timeout |
| 127 | AgentNotFound |
| 2–123 and 125–128 | Failure |
| ≥ 129 (i.e. 128+N) | Crash(N) |
| **negative (`i32::MIN..=-1`)** | **Crash(exit_code)** |

Unit tests pin both shapes: `classify(137) → Crash(137)` (a shell-style 128+N value)
and `classify(-1) → Crash(-1)`, `classify(-9) → Crash(-9)`
(`src/types/mod.rs:1462-1468`, `src/outcome/mod.rs:2280-2281,2930-2940`).

**How the alert renders it** (`src/outcome/mod.rs:343 → 1486 → 1502-1569`):
`Outcome::Crash(code)` calls `handle_crash(store, bead, code)`, which computes
`signal_num = if code > 128 { code − 128 } else { code }`, then fills the template
`- **Exit code**: {code} (signal {signal_num})` and labels the alert
`signal-{signal_num}`. With the sentinel this produces, verbatim, on the live
alert beads:

```
- **Exit code**: -1 (signal -1)
labels: alert, crash, signal--1, ...
```

**Where the real signal *is* visible — and where it is not.** `ExitStatus::signal()`
is read only to compose human-readable log text ("terminated by signal {sig} (not
initiated by needle)", transform path, `src/dispatch/mod.rs:2084-2090`), and needle's
*own* received signals are logged by name on graceful shutdown
(`src/worker/mod.rs:1345-1358`: SIGHUP / SIGINT / SIGTERM). Neither lands in the
numeric fields of `metadata.json`, `events.jsonl` or the alert body.

**Persisted surfaces (all read live, 2026-09-07):**

| Surface | Shape | Example |
|---|---|---|
| `.beads/traces/<bead>/metadata.json` | `{"exit_code": -1, "outcome": "crash", "timeout_reason": null, "captured_at": ...}` | bf-12gb0r, bf-57nao4 (both 2026-08-26T22:54Z) |
| `.beads/events.jsonl` — authoritative; this is what the crash classifier reads | `{"event": "crash", "exit_code": -1, "outcome": "crash", "worker": ..., "ts": ...}` | 247/247 crash records |
| alert bead body + label | `- **Exit code**: -1 (signal -1)` / `signal--1` | bf-56kmlk et al. |

**Timestamp caveat:** the alert body's **Timestamp** is `Utc::now()` inside
`handle_crash` — when needle *created the alert* (post-reap), not the kill instant.
Corpus work has measured gaps of seconds to ~2 minutes between the kill and that
stamp, so it brackets the death but does not equal it. A crash *task's* quoted
"Timestamp:" is the source alert bead's creation stamp, one more indirection out.

## 3. Termination causes that surface as exit -1, and the evidence that decides

| # | Cause | Surfaces as | Confirming evidence | Rules it out |
|---|---|---|---|---|
| 1 | **memcg OOM SIGKILL** (cgroup memory limit) | -1, Crash | `journalctl -k`: `Memory cgroup out of memory` + `oom-kill:constraint=CONSTRAINT_MEMCG … oom_memcg=…run-<id>.scope … task=<name>,pid=<n>` | no kernel record in the death window |
| 2 | **systemd-oomd** userspace kill | -1, Crash | `journalctl -u systemd-oomd`: "Killed /user.slice/… due to memory pressure … for > Xs"; PSI at `cat /proc/pressure/memory` (oomd is present and enabled on this box) | oomd log silent in the window |
| 3 | **System-wide OOM** (global, not memcg) | -1, Crash | `constraint=CONSTRAINT_NONE` in the oom-kill line | **none in the retained window — 525/525 events are CONSTRAINT_MEMCG** |
| 4 | **Needle hard/idle timeout kill** | **124**, Timeout — never -1 | `timeout_reason: {"hard":{"timeout_secs":N}}` in trace metadata; `event:"timeout"` in events.jsonl | a -1 record ⇒ *not* a needle timeout |
| 5 | **Parent teardown, graceful** (needle itself receives SIGHUP/SIGINT/SIGTERM) | **no crash record** — claimed bead released with reason `signal received (<NAME>)` | worker-log shutdown line; bead release event | a -1 crash record exists ⇒ needle survived to reap its child |
| 6 | **Worker SIGKILL / reboot / `needle cleanup`** (capture race) | **no record at all**, or -1 in metadata.json with no crash event in events.jsonl | events.jsonl ends at the fatal `dispatch`; bracket the silent gap between surrounding claims | a crash record stamped inside the death window |
| 7 | **SIGHUP cascade** (historic claim) | claimed -1 | **none found: 0 sighup/hangup records in events.jsonl** | classifier attributes the 2026-08-16 wave to memcg OOM (`scripts/crash-classifier.sh:466`) |
| 8 | **Manual / external kill** (operator, orchestrator, scope stop) | -1, Crash | journal audit of scope stop / killpg around the instant; no kernel OOM record | kernel OOM record present (⇒ row 1–3) |

The two most-confused rows, in detail:

- **MemoryMax kills; CPUQuota does not.** A cgroup `MemoryMax` breach triggers the
  kernel memcg OOM killer → SIGKILL → -1. `CPUQuota` only **throttles** CPU
  bandwidth; it never terminates a process, so it cannot produce -1 — at worst it
  starves the child into needle's deadline, which then records **124**. Verified
  live on my own dispatch: scope `run-p3916213-i247154771.scope` carries
  `MemoryMax=12884901888` (12 GiB) with `CPUQuotaPerSecUSec=infinity`; the enclosing
  `needle.slice` carries `MemoryHigh=25769803776` (24 GiB) /
  `MemoryMax=34359738368` (32 GiB). Re-verified the same day on a *different*
  attempt's scope (`run-p4061388-i247299946.scope`) — identical values, so the
  12 GiB bound is the fleet-wide dispatch shape, not one attempt's setting. A memcg
  kill is **not** evidence the box was out
  of memory — system RAM can be free while one scope blows its own limit (that is
  exactly the 2026-08 bloat-era signature: 17 GB of loose objects inside a 12 GiB
  scope).
- **Which victim died matters.** The kernel line names the killed task:
  `task=node` means the **agent process itself** (claude-code runs on Node) was
  killed → needle records -1 → `crash`. `task=git` / `task=bash` mean a **child of
  the agent** was killed — the agent usually absorbs that and exits 1, which is a
  `fail` record, not a `crash`. Of the 525 events: git 307 (the bloat era's
  repository operations), node 157, bash 60, python3 1. **Since 2026-09-01 every
  memcg kill sits in a named synthetic test/repro scope, and none in a live
  dispatch scope** (`run-p*-i*.scope`: zero since 2026-09-01): `safe-git-gc-run-*`
  / `safe-git-gc-*` (49, `task=bash` — the deliberate bound-verification test
  scopes), the bf-1s6c3 / bf-4yjq OOM repro scopes `bf1s6c3-push-*` `bf1s6c3-gc-*`
  `bf4yjq-crash-*` (49, `task=git`), the memory-watch tests `mw-oom*` `mw-diag`
  `mw-abort-test` `run-isolated` `probe-hog` (12), and `gcmb-bare-aggressive`
  (1, `task=git` — the re-run of the Aug-14 death command). Post-repair kills are
  verification harnesses re-creating known deaths, not live agent work.
- **SIGHUP is the anti-cause here.** When *needle* receives SIGHUP it shuts down
  gracefully (`src/worker/mod.rs:264` installs the handlers; `:1345-1390` releases
  the claimed bead with reason `signal received (SIGHUP)` and stops) — no -1 crash
  record. And the fleet log contains zero SIGHUP records for the whole crash era.

## 4. "signal -1": recorded convention vs. real signal number

- `signal(7)`: standard + real-time signals number **1–64**; 0 is only the `kill(2)`
  liveness-probe convention; **no negative signal exists**.
- needle's `(signal {num})` body field and `signal-{num}` label are *arithmetic on
  the recorded exit code*, so the -1 sentinel flows through untouched →
  `signal--1`. The label is cosmetically broken (double dash) and semantically
  empty: it identifies no signal.
- **The true signal is not recoverable from needle's records.** To name it:
  1. `journalctl -k` around the death — an OOM record ⇒ SIGKILL (memcg or global
     per the `constraint=` field);
  2. the worker log's human-readable lines (`"terminated by signal {sig}"`,
     transform path) — the only place needle writes the real number;
  3. context: a synchronized -1 wave across workers ⇒ OOM regime; a single bead in a
     long git operation ⇒ scope limit; `timeout_reason` set ⇒ it would have been 124.
- **Never** read needle's -1 as "SIGHUP". That reading is valid only for a
  Python-subprocess writer (§1) — and the 2026-09-02 sibling doc makes exactly that
  mistake (see §5).

## 5. Corrections to the 2026-09-02 sections below

1. **"Exit code typically -1 or 137"** (OOM section below) — in needle records it is
   **always -1**. `137` can appear only inside captured *stderr text* (a shell's own
   job report), never in the `exit_code` field.
2. **"SIGHUP Cascade (~20% of cases)"** (below, and in
   `docs/research/root-cause-analysis-signal-minus-one-crashes.md` whose signal table
   maps `-1 → SIGHUP`) — no log support: 0 sighup/hangup records. The 2026-08-16
   wave was kernel memcg OOM. The `-1 → SIGHUP` mapping is the Python-subprocess
   convention; needle is Rust and records -1 for every signal.
3. **The percentage ranks (40/30/20/5/5)** — never measured; treat them as the
   2026-09-02 author's estimate. Measured 2026-09-07: 247 crash records, 100% at
   exit -1, spanning 2026-08-16T04:27Z → 2026-08-26T22:54Z, and **zero -1 records
   since** — the live failure signature is synchronized `exit 1` waves
   (service-class), not kills.
4. **"Resource Limits"** arm — split it: `MemoryMax` kills (memcg OOM → -1);
   `CPUQuota` throttles only (→ at most a 124 timeout); `RuntimeMaxSec`/`TasksMax`
   are not how this fleet's kills fire — the observed deadline kills are needle's
   own hard/idle timeouts (→ 124, with `timeout_reason`).
5. **Repository bloat** — a *historical* cause (17 GB loose objects, repaired
   2026-09-01, re-verified 2026-09-06; `.beads/` gitignored, 10 MB pre-commit gate,
   `pack.windowMemory` bounds, daily timers). Keep it as background, not a live
   hypothesis.
6. **The 30-second FALSE_POSITIVE rule** — keep, with the corpus caveat: per-attempt
   committing against a 3–12 min kill cadence lands commit/kill pairs within seconds
   *by construction*, so proximity alone is not completion evidence; decide from the
   bead's state at the instant (delivered? closed? deliverable at HEAD?).
7. **Decision tree** — add the two arms it lacks: a **timeout arm**
   (`timeout_reason` set / exit 124 ⇒ not a crash), and an **UNKNOWN / capture-race
   arm** (a crash-shaped trace against a silent `events.jsonl` is classified *from
   the trace*; the kill wave can take needle before it writes the record, so
   classifier UNKNOWN is never by itself evidence that no crash occurred).

## 6. Evidence-source quick reference (all verified on this box, 2026-09-07)

```bash
# Kernel OOM (memcg vs global) — the decisive record; works unprivileged:
journalctl -k --since "<death - 5min>" --until "<death + 5min>" \
  | grep -iE "out of memory|oom-kill|killed process"
#   525 events in the retained window, ALL 'Memory cgroup out of memory'
#   (CONSTRAINT_MEMCG), each naming oom_memcg=...run-<id>.scope, task=, pid=

# systemd-oomd (userspace, PSI-driven):
journalctl -u systemd-oomd --since "<window>" | grep -i killed

# cgroup limits actually in force (12 GiB per-dispatch scope):
systemctl --user show run-<pid>-<n>.scope -p MemoryMax -p CPUQuotaPerSecUSec
systemctl --user show needle.slice -p MemoryHigh -p MemoryMax
cat /proc/self/cgroup        # which scope/slice you are inside right now

# Needle's own record of the death:
cat .beads/traces/<bead>/metadata.json      # exit_code/outcome/timeout_reason/captured_at
# events.jsonl is JSONL — parse it; bare grepping for -1 hits UUID substrings:
python3 -c "import json;[print(json.loads(l)) for l in open('.beads/events.jsonl') \
  if json.loads(l).get('event')=='crash']"

# Graceful-shutdown evidence (needle received a signal itself):
grep -h "signal received" ~/.needle/logs/needle-<worker>.log
```

Cross-links: the sibling analysis this section correctates is
[docs/research/root-cause-analysis-signal-minus-one-crashes.md](research/root-cause-analysis-signal-minus-one-crashes.md)
(whose own header now carries a dated banner pointing here); operational
classification lives in `docs/crash-response-guide.md` and
`scripts/crash-classifier.sh` (its `-1` handling and Aug-16 attribution note are at
`:455` and `:466` at HEAD e1e5bf0 — the same numbers held at the earlier HEAD
14e301d, but the working-tree copy carries co-tenant edits, so line numbers can
drift).

**Re-verification note (same day, the bead's closing attempt).** Every §1–§6
claim above was re-executed from scratch; all source pins (NEEDLE 9d09220 clean,
needle 0.6.0), record counts (247 crash / -1, 22 timeout / 124, 0 sighup), kernel
tally (525/525 `CONSTRAINT_MEMCG`; git 307 / node 157 / bash 60 / python3 1), the
five live alert beads' `-1 (signal -1)` bodies, the `signal--1` label (1,585
checkpoint occurrences), both trace fixtures, and the cgroup limits all reproduced
exactly. Three deviations found and corrected inline: (1) `.beads/events.jsonl` is
a live log — 31,101 events on re-verification vs 30,808 first pass, crash/timeout
counts unchanged; (2) the post-2026-09-01 kill-scope claim was too narrow — the
kills sit in *several* synthetic scope families (§3), the invariant that survives
is **zero kills in live dispatch scopes**; (3) the pinned HEAD for the classifier
citation moved 14e301d → e1e5bf0 with the cited lines intact.

**Third same-day pass (split-refusal dispatch).** The auto-split dispatcher read this
bead's three release cycles as "too big, decompose it"; in fact each attempt had finished
the research and died before committing, so the split was refused and the completed
deliverable committed instead. This pass re-ran the decisive figures first-hand — NEEDLE
source pins, 247/247 crash −1, 22 timeout 124, 0 sighup, 525/525 `CONSTRAINT_MEMCG`
(git 307 / node 157 / bash 60 / python3 1, 0 in live dispatch scopes since 2026-09-01),
shell 255/137/129, the five alert beads' `-1 (signal -1)` bodies, the `signal--1` label
(1,585 live occurrences), both test fixtures, and the `classify` table — all reproduced;
the only delta was the live-log growth already flagged above (31,468 events, fail=1
1,186 → 1,195).

---

## Signal Basics: Unix/Linux Exit Codes

### Standard Signal Exit Code Pattern

When a process is terminated by a signal, the exit code follows the pattern:

```
exit_code = 128 + signal_number
```

**Examples:**
- SIGKILL (signal 9) → exit code 128 + 9 = **137**
- SIGTERM (signal 15) → exit code 128 + 15 = **143**
- SIGHUP (signal 1) → exit code 128 + 1 = **129**
- SIGSEGV (signal 11) → exit code 128 + 11 = **139**

### What is Exit Code -1?

**Exit code -1 is NOT a standard signal exit code.** It indicates:

1. **Process exit(-1)** - The process explicitly called `exit(-1)` or returned -1 from `main()`
2. **Infrastructure termination** - Process was killed by external force (OOM, systemd, cgroup limits)
3. **Agent framework reporting** - NEEDLE framework uses -1 to indicate "terminated by infrastructure"

**Critical Insight:** When NEEDLE reports exit code -1, it means the agent process was terminated **outside the normal signal handling flow**. This is always an **infrastructure event**, not a code defect.

---

## Common Causes of Exit Code -1

### 1. OOM Killer (Out of Memory)

**Mechanism:** Linux kernel terminates processes when memory is exhausted

**System State:**
- Memory pressure exceeds 80% for 20+ seconds
- systemd-oomd triggers process kills
- Kernel selects processes based on memory usage (RSS)

**Evidence from bf-1s6c3 crash:**
```
Aug 16 12:00:59 systemd-oomd: Considered 19 cgroups for killing
Aug 16 12:00:59 systemd-oomd: Killed /user.slice/user-1001.slice/...
Aug 16 12:00:59 systemd-oomd: Memory Pressure: 94.71% > 80.00% for > 20s
Aug 16 12:01:15 kernel: Out of memory: Killed process 1933332 (git)
```

**Characteristics:**
- Process killed instantaneously (no graceful shutdown)
- No application error logs
- Exit code typically -1 or 137 (depends on how process reports it)
- Affects multiple processes simultaneously during memory pressure events

**Detection:**
```bash
# Check kernel logs for OOM activity
sudo dmesg | grep -i "out of memory\|killed process"

# Check systemd-oomd logs
journalctl -u systemd-oomd | grep -i "killed\|memory"

# Check current memory pressure
cat /proc/pressure/memory
```

### 2. SIGHUP Cascade

**Mechanism:** System sends SIGHUP (signal 1) to all processes in a cgroup/session

**System State:**
- Terminal disconnection
- System shutdown/restart
- Systemd service reload
- Container orchestration actions

**Evidence from comprehensive crash investigation:**
```
Aug 16 12:00-17:00 UTC - SIGHUP cascade affecting 4 workers
- lab-domain-check
- lab-drawrace  
- lab-test-fix
- lab-roam-1
Total: 201+ crashes in 5-hour window
Exit code: -1 (reported by NEEDLE framework)
```

**Characteristics:**
- All workers affected simultaneously
- No selective targeting
- Occurs during system management operations
- Exit code typically -1 (not 129, because SIGHUP is caught/reported differently)

**Detection:**
```bash
# Check for SIGHUP in system logs
journalctl --since "1 hour ago" | grep -i "sighup\|hangup"

# Check process tree for SIGHUP receivers
ps aux | grep -i "needled\|agent" | awk '{print $2}' | xargs -I {} strace -p {} -e signal
```

### 3. Systemd/Cgroup Resource Limits

**Mechanism:** systemd or cgroup controller terminates process exceeding limits

**Common Limits:**
- MemoryMax (memory limit)
- CPUQuota (CPU time limit)
- RuntimeMaxSec (maximum runtime)
- TasksMax (maximum thread count)

**Evidence:**
```bash
# Check cgroup limits for current session
systemctl show user@$(id -u).service | grep -E "MemoryMax|CPUQuota|RuntimeMax"

# Check active cgroup memory usage
cat /sys/fs/cgroup/memory/user.slice/memory.limit_in_bytes
cat /sys/fs/cgroup/memory/user.slice/memory.usage_in_bytes
```

**Characteristics:**
- Process terminated when limit exceeded
- Logs show "cgroup" or "systemd" termination
- Exit code typically -1
- Can be triggered by single resource-intensive operation (e.g., git gc)

### 4. Repository Bloat + Git Operations

**Mechanism:** Large git repositories cause memory exhaustion during operations

**Evidence from bf-1s6c3 crash:**
```
Repository size: 18GB (should be <500MB)
Loose objects: 17.16GB (99% of repository)
.beads/issues.jsonl: 248MB (should be <5MB)
Result: Any git operation triggered OOM killer
```

**Characteristics:**
- Exit code -1 during git operations
- Repository size > 5GB
- Multiple crashes over short period
- All git operations fail with OOM

**Detection:**
```bash
# Check repository health
du -sh .git
git count-objects -vH

# If repository > 5GB → REPOSITORY BLOAT
# If loose objects > 1GB → NEEDS PACKING
```

**Prevention:**
```bash
# Add .beads/ to .gitignore immediately
echo ".beads/*.jsonl" >> .gitignore
echo ".beads/*.json" >> .gitignore
echo ".beads/checkpoint/" >> .gitignore
echo ".beads/traces/" >> .gitignore

# Run safe git gc
./scripts/safe-git-gc.sh --full
```

### 5. Container/Process Orchestrator Actions

**Mechanism:** External system terminates process for orchestration reasons

**Common Triggers:**
- Node drain (Kubernetes)
- Preemption ( Spot instances)
- Resource rebalancing
- Health check failures

**Characteristics:**
- Termination without graceful shutdown
- Exit code -1
- No application error
- Related to infrastructure events

---

## Signal -1 in Go Processes

### Go Signal Handling

Go has specific signal handling behavior:

1. **Default Signal Handlers:** Go runtime installs default handlers for SIGINT, SIGTERM, etc.
2. **Signal Notification:** Go's `os/signal` package allows explicit signal handling
3. **Exit Code Behavior:** Go programs exit with code 1 on unhandled signals, NOT 128+N

**Example from Go documentation:**
```go
// When signal is received, program exits with code 1
signal.Notify(sigchan, syscall.SIGINT, syscall.SIGTERM)
```

**This means:** In Go processes, exit code -1 is even MORE likely to be infrastructure termination, not signal delivery.

### Agent-Specific Context

In the NEEDLE agent framework:

1. **Exit Code -1** is used by the framework to indicate "terminated by infrastructure"
2. **Not caught by signal handlers** - Process killed before handlers can run
3. **Reported as -1** - Framework's way of saying "I didn't exit, I was killed"

**Evidence from crash logs:**
```
Exit code: -1
Signal: unknown
Outcome: terminated by infrastructure
```

---

## System State Checks

### When Investigating Exit Code -1

**Always check these system states:**

1. **Memory Pressure**
   ```bash
   free -h
   cat /proc/pressure/memory
   ```

2. **OOM Killer Activity**
   ```bash
   sudo dmesg | grep -i "killed process\|out of memory"
   journalctl -k | grep -i "oom"
   ```

3. **Systemd Activity**
   ```bash
   journalctl --since "1 hour ago" | grep -i "systemd-oomd\|cgroup\|slice"
   ```

4. **Resource Limits**
   ```bash
   systemctl show user@$(id -u).service | grep -E "MemoryMax|CPUQuota"
   ```

5. **Recent SIGHUP Events**
   ```bash
   journalctl --since "1 hour ago" | grep -i "sighup"
   ```

---

## False Positive Detection

### The 30-Second Rule

**Most exit code -1 crashes are false positives:**

If work was committed within 30 seconds before crash, it's a **post-completion termination**, not a task crash:

```bash
# Check if work completed before crash
git log --since="<crash_timestamp-60sec>" --until="<crash_timestamp+30sec>" --oneline

# If commit exists → FALSE POSITIVE (work completed, cleanup terminated)
# If no commit → Check system logs for infrastructure event
```

**Example Timeline (bf-5tgsk):**
```
16:35:54 UTC - Work completed, commit 549aa42
16:36:24 UTC - Agent terminated (exit code -1)
16:36:51 UTC - Bead closed successfully
```

**Time gap:** 30 seconds between completion and termination → **FALSE POSITIVE**

### System-Wide Event Detection

**If 10+ crashes occur in 10 minutes:**

This is an **infrastructure event**, not individual bead crashes:

```bash
# Count crashes in last 10 minutes
crash_count=$(bead list --since "10min ago" --status "crashed" --json | jq '. | length')

if [ $crash_count -gt 10 ]; then
  echo "INFRASTRUCTURE EVENT: $crash_count crashes in 10 minutes"
  echo "Generate single system event alert, not per-bead alerts"
fi
```

---

## Classification Decision Tree

```
Exit Code -1 Detected
│
├─ Check work completion (30-second window)
│  ├─ Commit exists within 30s before crash
│  │  └─ FALSE POSITIVE (post-completion cleanup termination)
│  │     ✅ NO ACTION NEEDED
│  │
│  └─ No commit evidence
│     └─ Check system logs
│
├─ System Logs Check
│  ├─ OOM killer activity found
│  │  └─ INFRASTRUCTURE EVENT (memory exhaustion)
│  │     ⚠️ Check system resources, verify work completion
│  │
│  ├─ SIGHUP cascade found
│  │  └─ INFRASTRUCTURE EVENT (system-wide signal)
│  │     ⚠️ Check for system-wide event, verify all workers affected
│  │
│  ├─ systemd/cgroup limits exceeded
│  │  └─ INFRASTRUCTURE EVENT (resource limits)
│  │     ⚠️ Check resource usage, verify operation that triggered limit
│  │
│  └─ No system log evidence
│     └─ Manual investigation required
│
└─ Check Crash Surge
   ├─ 10+ crashes in 10 minutes
   │  └─ INFRASTRUCTURE EVENT (system-wide)
   │     ✅ Generate single system alert
   │
   └─ Isolated crash
      └─ Individual investigation required
```

---

## Key References

### Unix/Linux Signal Documentation
- **Signal(7)** man page: Standard Linux signals and their meanings
- **Systemd-oomd(8)** man page: Systemd OOM daemon behavior
- **Cgroups(7)** man page: Control group resource limits

### Project Documentation
- `docs/crash-response-guide.md` - Comprehensive crash classification guide
- `docs/comprehensive-crash-investigation-report-2026-09-01.md` - 200+ crash analysis
- `docs/crash-mitigation-strategies.md` - Prevention strategies

### Specific Crash Investigations
- `docs/investigation-summary-bf-173o7e-2026-09-01.md` - OOM crash with 18GB repository
- `docs/crash-investigation-bf-5tgsk-2026-08-16.md` - Post-completion false positive

---

## Conclusions

### What Exit Code -1 Means

**Exit code -1 = Infrastructure Event**

1. **NOT a standard Unix signal** - Not in the 128+N pattern
2. **Infrastructure termination** - Process killed by OS/systemd/cgroups
3. **NOT a code defect** - Application code not responsible
4. **Requires system investigation** - Check logs, resources, limits

### Common Causes (Ranked by Frequency)

1. **Memory Pressure/OOM Killer** (~40% of cases)
   - System memory exhaustion
   - systemd-oomd activation
   - Process selection based on RSS

2. **Post-Completion Termination** (~30% of cases)
   - Work completed successfully
   - Cleanup/post-processing terminated
   - FALSE POSITIVE - no action needed

3. **SIGHUP Cascade** (~20% of cases)
   - System-wide signal delivery
   - Multiple workers affected simultaneously
   - Infrastructure event, not task-specific

4. **Resource Limits** (~5% of cases)
   - Cgroup limits exceeded
   - Systemd resource limits
   - Container orchestration actions

5. **Repository Bloat** (~5% of cases)
   - Large git repositories
   - Memory exhaustion during git operations
   - Preventable with .gitignore configuration

### What Does NOT Cause Exit Code -1

1. ✅ **Application code defects** - Would cause exit code 1 with error message
2. ✅ **Standard Unix signals** - Would use 128+N pattern
3. ✅ **Normal errors** - Would have error logs and stack traces
4. ✅ **Domain-check bugs** - No defects found in any crash investigation

---

**Report Completed:** 2026-09-02  
**Classification:** INFRASTRUCTURE EVENT DOCUMENTATION  
**Next Steps:** Update crash alert manager to detect system-wide events automatically
