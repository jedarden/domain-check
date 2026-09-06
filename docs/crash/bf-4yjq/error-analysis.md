# Error Analysis — bf-4yjq crash storm (2026-08-12)

**Subject bead:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo mirror has
diverged/gone stale" (P2, closed 2026-08-17)
**Dispatch bead:** domchk-940a5fac · **Written:** 2026-09-06 · **Worker:**
`claude-code-glm-5.3-flash-lab-roam-6`
**Storm window:** 2026-08-12T17:50:23Z (first dispatch) → 21:14:56Z (last dispatch); 56
dispatches of the same bead to one worker session (`8446529e`).

Every figure in this document was **re-derived live on 2026-09-06** from the committed
primary sources in [`raw-logs/`](raw-logs/README.md) — the 56 per-run session transcripts
(14.76 MB uncompressed), the structured needle event log (1,071 records), and the plaintext
worker-log extract (225 lines) — not copied from an earlier report. Where this analysis
sharpens or corrects the standing record, §7 says so explicitly.

---

## 1. Answer up front

| Question | Answer |
|---|---|
| **Exact failure mode** | Needle recorded `exit_code=-1 outcome=Crash(-1)` / `signal_code=-1` on **50 of 56 dispatches**. −1 is needle's **sentinel for "agent process died by unhandled signal / unrecoverable status"** — it is not a signal number. The killing signal itself is unrecorded for this date (no surviving kernel record, §4); SIGKILL-class external kill is the only mechanism consistent with the evidence. |
| **Agent-visible error text** | **None at the death points.** 49 of the 50 crash transcripts truncate immediately after a `git push origin main` tool call with **no tool result ever returning**; attempt 12's sole push was denied in-band by a permission prompt and its transcript ends right after that denial. Net: no crash attempt ever received a real git result from an executed push (§4). No panic, no stack trace, no OOM message, nothing. |
| **Error type** | **INFRASTRUCTURE** — resource exhaustion during git operations on the ~18 GB repository, per `docs/crash-response-guide.md`'s taxonomy (§6). |
| **Stack traces / panic dumps** | **Zero exist, in any source** (§4). This is a verified absence, not an extraction gap. |

---

## 2. Sources searched

| Source | Size | Searched for |
|---|---|---|
| `raw-logs/bf-4yjq-crash-sessions-2026-08-12.tar.gz` → 56 transcripts | 14.76 MB | panic/goroutine/backtrace text, OOM/kill messages, git stderr, error-flagged tool results, per-run truncation point |
| `raw-logs/needle-worker-log-bf-4yjq-slot2.log` | 225 lines | every ERROR/WARN/INFO line needle emitted for this bead |
| `raw-logs/needle-events-2026-08-12-bf-4yjq.jsonl` | 1,071 records | any record carrying an `error` field or non-{exit_code,outcome} diagnostic |
| `raw-logs/sessions-index.tsv` | 56 rows | per-attempt outcome, exit code, kill gap |
| `.beads/checkpoint/forensic.jsonl` (the 50 alert beads) | 50 issues | alert titles, description templates, labels |
| live: `coredumpctl list`, `journalctl` | — | any kernel/crash record covering 2026-08-12 |

---

## 3. Complete error-message inventory

### 3.1 needle worker log — the only crash-era error lines that exist

225 lines total: **50 ERROR / 6 WARN / 169 INFO**. The non-INFO lines, exhaustively:

| Level | Count | Message (verbatim shape) | Meaning |
|---|---|---|---|
| ERROR | 50 | `agent crashed — releasing bead and creating alert bead_id=bf-4yjq signal_code=-1 agent=claude-code-glm-4.7` | the crash signal itself |
| WARN | 4 | `agent timed out — releasing bead as deferred bead_id=bf-4yjq` | attempts 52–55 (exit 124) |
| WARN | 1 | `agent failure — releasing bead bead_id=bf-4yjq` | attempt 2 (exit 1) |
| WARN | 1 | `agent exited successfully but bead is still open (orphaned) bead_id=bf-4yjq status=blocked` | attempt 56, 21:14:59Z |

Each ERROR is preceded by its paired outcome line, which carries the only exit-code data:

```
2026-08-12T17:53:53.875682Z  INFO ...: needle::outcome: handling agent outcome bead_id=bf-4yjq exit_code=-1 outcome=Crash(-1)
2026-08-12T17:53:53.875703Z ERROR ...: needle::outcome: agent crashed — releasing bead and creating alert bead_id=bf-4yjq signal_code=-1 agent=claude-code-glm-4.7
2026-08-12T17:54:02.643609Z  INFO ...: needle::outcome: crash alert bead created bead_id=bf-4yjq alert_id=bf-276uk
```

Crash→alert handling took **~8–9 s** per death (alert bead created 50/50 times — the full
50 alert IDs are in Appendix A).

**Exit-code / outcome pairs observed** (worker log, cross-checked against the structured
log's `outcome.classified`/`outcome.handled` pairs, which tally 100 crash / 2 failure /
8 timeout / 2 success occurrences = the same 50/1/4/1 attempts × 2 events each):

| Outcome | Attempts | Needle record |
|---|---|---|
| crash | **50** | `exit_code=-1`, `outcome=Crash(-1)`, `signal_code=-1` |
| failure | 1 (attempt 2, 18:00:17Z) | `exit_code=1`, `outcome=Failure` |
| timeout | 4 (attempts 52–55: 20:40:47 / 20:51:01 / 21:01:14 / 21:11:27Z) | `exit_code=124` |
| success | 1 (attempt 56, 21:14:56Z) | `exit_code=0` — but the bead stayed open → `bead.orphaned` |

**Retry-loop escalation indicators** (workflow-system *consequence* of the crashes, not a
separate failure): three `auto-split triggered: using SPLIT template bead_id=bf-4yjq
failure_count=N threshold=3` lines at 20:51:14Z (N=3), 21:01:27Z (N=4), 21:11:40Z (N=5).

### 3.2 Structured event log — carries no error fields

All 1,071 records were parsed. **Zero records contain an `error` key** (`grep -c '"error"'
` → 0). The only diagnostic payload anywhere in the structured log is
`{"exit_code": -1, "outcome": "crash"}` on `outcome.classified`, and
`{"action": "alerted", "outcome": "crash"}` on `outcome.handled`. No signal number, no
stderr excerpt, no stack frames — needle's structured telemetry for this era records the
*fact* of a death, never its content.

### 3.3 Agent-visible errors inside the 56 transcripts — the only error text the runs ever produced

Scanning every tool result in all 56 transcripts for `is_error: true` yields **3 results**,
plus **4 git stderr strings** that arrived as normal (non-flagged) tool output. None is
related to the deaths; all are mid-task errors the agent recovered from:

| Attempt | Error text | Nature |
|---|---|---|
| 30, 34 | `Exit code 127` — `/run/current-system/sw/bin/bash: line 1: gh: command not found` | environment: the `gh` CLI was not installed; agent switched to plain git/curl |
| 12 | `The user doesn't want to proceed with this tool use. The tool use was rejected …` | in-band rejection of a `git push` permission prompt — see §5 |
| 2× (2 attempts) | `fatal: unrecognized argument: --local` | benign git flag misuse, recovered |
| 2× (2 attempts) | `fatal: path 'head.go' does not exist in '63ba024'` | benign `git show` path error, recovered |

There is **no other error text anywhere in 14.76 MB of crash-era agent output** — and in
particular no error, warning, or diagnostic at any of the 50 death points.

> **Grep caveat for anyone re-searching this corpus:** a naive `grep 503\|502` over the
> transcripts returns ~700 hits that are **all false positives** — substrings of UUIDs and
> tool-call IDs (`call_9b268b48350341c6a1dcd493`), and prose from the repo's own docs the
> agents read (domain-check's plan/tests tables contain literal `503`/`502` figures, and the
> dispatch prompt embeds CLAUDE.md's retry-strategy sample, which mentions them). No
> transcript line is an HTTP error emitted by a service. Match on message-shaped patterns,
> not bare numbers.

### 3.4 The 50 crash-alert beads — needle's own error report, and the source of the "signal −1" confusion

All 50 alert beads survive in `.beads/checkpoint/forensic.jsonl` with **one identical
title and one identical description template**:

```
ALERT: Agent crash on bead bf-4yjq

## Agent Crash Report

- **Bead ID**: bf-4yjq
- **Agent**: claude-code-glm-4.7
- **Exit code**: -1 (signal -1)
- **Workspace**: .
- **Timestamp**: 2026-08-12T17:54:00.242078980+00:00

The agent process was killed. This bead has been released for retry.
```

Labels: all 50 carry `alert` + `crash` + `signal--1`; 34 also carry a `failure-count:N`
escalator whose surviving values are `1`×4, `2`×3, `4`×15, `5`×11 and `12`×1 (labels are
mutated in place as retries accumulate, so the distribution is a snapshot, not a log).

Two things follow from these beads:

- **The alert template itself prints `-1 (signal -1)`.** Needle rendered its *sentinel* as
  though it were a signal number, in the very artifact meant to be read first. That wording
  is the origin of the "exit −1 = SIGHUP? = signal 9?" confusion that runs through the
  early investigation docs — the sentinel was mistaken for a signal at the source. The
  alert's own text proves only "the agent process was killed"; it names no signal.
- The alert timestamp (`17:54:00.242Z`) is ~6.4 s after needle's classification line
  (17:53:53.875Z) — it is the alert-bead creation time, not the death time. Alert-bead
  timestamps must not be used as death timestamps (a discrepancy that also bit the
  bf-4x12ec investigation).

---

## 4. Stack traces and panic dumps — none exist (verified absence)

This is the acceptance criterion that resolves to a negative, and the negative is itself
the finding. Four independent lines of evidence:

1. **Content search of the transcripts:** 0 hits across all 56 files for `panic:`,
   `goroutine N [`, `fatal error:`, `out of memory`, `cannot allocate`, `memory exhausted`,
   `Killed`, or any Go/Rust backtrace frame pattern.
2. **No stderr capture existed in this era.** The transcripts are the runs' stdout stream —
   the artifact needle's later trace capture stores as `stdout.txt` in `.beads/traces/`.
   There is no separate stderr artifact for August 2026, and `.beads/traces/bf-4yjq/` is
   absent (the dispatches predate trace capture).
3. **No core dumps.** Live re-checked 2026-09-06: `coredumpctl list` earliest entry is
   **2026-08-17 16:01:44 EDT** (pdftract, COREFILE `missing`) — nothing from Aug-12, and
   nothing retained even for the entries that do exist.
4. **No kernel records.** Live re-checked 2026-09-06: the system journal's first entry is
   **2026-08-15 19:46:33 EDT** (single boot). The Aug-12 kernel-side kill records — the one
   artifact that would name the signal and the cgroup limit — were never written to any
   store that survives.

**What the transcripts do record instead is the death itself, by truncation.** Re-verified
across all 50 crash transcripts (not taken from the README's earlier count):

- **49 of 50** end with an unanswered `git push origin main` `tool_use` — the agent issued
  the push and the transcript simply stops. No `tool_result` ever arrived.
- **Attempt 12** is the one exception, and it is not a git result: it issued exactly one
  `git push origin main`, which was denied in-band by a permission prompt before git ran
  (the `is_error` row in §3.3). Its transcript ends immediately after that denial — **no
  later push was issued** (the denial result is the last tool event in the transcript, and
  it is the attempt's only push).

So, precisely: **no crash attempt ever received a git result back from a push.** The last
event in each transcript sits **0–23 s (median 14 s)** before needle's `outcome.classified`
for that attempt — that gap is the kill-to-classification latency, and its shortness is
itself evidence of an abrupt external kill rather than a graceful exit. Crash-attempt
durations were 1.1–6.2 min (median 2.5) — each retry redid the whole task from claim to
push before dying.

---

## 5. Exact failure mode

| Property | Value |
|---|---|
| Exit code recorded | `-1` — **needle's sentinel**, not a signal number and not a shell status. `outcome=Crash(-1)` is needle's enum rendering of it; `signal_code=-1` repeats the sentinel because the signal was not observed, only its absence of a normal exit. |
| Killing signal | **Not recorded for this date** (§4.4). SIGKILL-class external kill is the only mechanism consistent with: no agent-visible error, no core dump, abrupt transcript truncation, sub-15-s classification latency, and 100 % reproducibility at the same command. A SIGHUP or graceful shutdown would have left shell-level text; a panic would have left a trace. |
| Death point | Uniform in shape: `git push origin main` is the final tool call in all 50 — 49 killed mid-push with no result, attempt 12 denied pre-execution (§4). Description variants: "Push local commits to Forgejo origin" ×29, "…to Forgejo" ×7, "Push to Forgejo origin" ×6, "Push commits to Forgejo origin" ×6, "Push 307 commits to Forgejo origin" ×1, "…to Forgejo (origin)" ×1. |
| Mechanism | `git push`'s `git pack-objects` walking the repo's **17.2 GiB of loose objects** inside the needle dispatch scope's **12 GiB `MemoryMax`** → memcg OOM kill. For Aug-12 this is per-run corroboration, not kernel proof (the kernel step is unrecoverable, §4.4); the same push-side mechanism was later **kernel-proven** for bf-198ne (2026-08-16, `docs/crashes/bf-198ne-crash-report.md`), and the `pack.windowMemory`/`pack.threads` bounds installed 2026-09-02 cover `git push` as well as gc precisely because of this. |
| Scale / cadence | 50 consecutive losses 17:54–20:30 UTC, ~3.1-min mean interval — inside a same-day **455-event, 6-bead** workspace-wide storm, of which this bead's window is one slice (canonical report §4). |
| Handling after each death | bead released → crash alert bead created (8–9 s later). 50 alert beads total (Appendix A); the 9 listed in the superseded comprehensive report are a verified subset. |

The 50th crash at 20:30:38.310Z ended the crash run; the four timeouts and the exit-0 run
that follow are tail behaviour of the same retry loop, not a different failure mode. The
exit-0 run's own outcome line is the last error indicator in the window: `agent exited
successfully but bead is still open (orphaned) bead_id=bf-4yjq status=blocked` — it passed
needle's command gate but did not close the bead; the bead was closed five days later, on
2026-08-17.

---

## 6. Error-type categorization

**Classification: INFRASTRUCTURE** — per `docs/crash-response-guide.md`'s taxonomy, and
specifically its repository-bloat row: `exit_code -1` + fixed-cadence re-dispatch deaths +
`.git` ≫ 5 GB (the repo was ~18 GB with a ≈1,800:1 loose:packed object ratio at crash
time). This matches the already-recorded verdict in `docs/crash-root-cause-bf-4yjq.md`;
this analysis adds the error-level evidence beneath it.

Exclusions, each with its basis from the error inventory:

| Category | Excluded because |
|---|---|
| **SERVICE_FAILURE** | No HTTP 503/502, no gateway/unreachable text, no network error string anywhere in 14.76 MB of transcripts or in either needle log. (Naive numeric greps *appear* to find 503/502 — they are UUID/tool-call-ID substrings and the repo's own doc prose; see the caveat in §3.3.) The crash is a local resource event, not a dependency. |
| **CODE_DEFECT** | No domain-check binary was built or executed during the storm (`docs/crash/bf-4yjq/version-info.md` §1) — the deaths are in git plumbing under the agent's dispatch scope. No panic, no non-zero application exit on the crash attempts: the agent process never got to report anything. |
| **WORKFLOW / FALSE_POSITIVE** | The bead was genuinely open and the work genuinely unfinished at every death — each attempt was actively executing the bead's own task (commit + push to Forgejo) when killed. The workflow system's *responses* (3 auto-split escalations, 50 alert beads) are consequences of the crash loop, not its cause. |

One standing claim needs a correction flag: `docs/crash-root-cause-bf-4yjq.md` §"Critical
Finding" states the bead "was BLOCKED at crash time, not actively executing its git remote
configuration work." The per-run error evidence contradicts the second half — all 50 crash
attempts were mid-`git push` on the bead's task at death. The `status=blocked` reading
comes from the **post-storm** orphan check at 21:14:59Z (after the exit-0 run), not from
any crash-time record.

---

## 7. Reconciliation with the standing record

| Prior claim | Status after this analysis |
|---|---|
| Comprehensive report §3 / artifact catalog §6: "No raw session evidence survives … no stack traces" | **Half-corrected already** by `raw-logs/README.md` §3 (the 56 transcripts do survive). This analysis completes it: the transcripts were searched for error content, and there is none — so the no-stack-traces conclusion now rests on a positive search of the surviving evidence, not just on the absence of artifacts. |
| Comprehensive report §2: "`signal -1` … represents SIGKILL (Signal 9) from the Linux OOM killer" | **Sharpened.** −1 is needle's sentinel; the signal identity was never recorded for Aug-12. "SIGKILL-class" is correct as inference (canonical report §6 rates the OOM step MEDIUM-HIGH confidence for exactly this reason). Do not cite "signal −1 = signal 9" as an observed fact. |
| Comprehensive report: 9 crashes at ~17-min intervals | **Superseded** (50 crashes at ~3.1-min intervals) — already banner-flagged in that report; its 9 alert IDs are confirmed here as a subset of the 50. |
| `raw-logs/README.md` §4: "All 50 crashed runs have `git push origin main` as their final recorded Bash tool call" | **Confirmed and sharpened**: 49/50 got no result at all; attempt 12's single push was denied in-band by a permission prompt before git ran and its transcript ends there — no later push was issued. Net: zero pushes ever returned a git result in a crash attempt. |
| Canonical report §5: "Raw telemetry: none survives (no core dumps, no kernel OOM logs, no heartbeats)" | **Confirmed live 2026-09-06** (coredumps, journald coverage), with one refinement: 454 `heartbeat.emitted` records *do* survive in the structured log — what is absent is kernel/process-level telemetry, not all heartbeats. |
| Early docs debating whether "signal −1" means SIGHUP or SIGKILL | **Origin identified** (§3.4): needle's own crash-alert template prints `Exit code: -1 (signal -1)`, rendering the sentinel as a signal number in the artifact agents read first. The −1 was never a signal; the template wording — not any observation — seeded the debate. |

---

## 8. Provenance — re-derive everything

```bash
cd /home/coding/domain-check/docs/crash/bf-4yjq/raw-logs

# 50 crash / 1 failure / 4 timeout / 1 success, with kill gaps
awk -F'\t' 'NR>1{print $2" exit="$3}' sessions-index.tsv | sort | uniq -c | sort -rn

# the ERROR/WARN lines (50 + 6)
grep -cE ' ERROR ' needle-worker-log-bf-4yjq-slot2.log
grep -E ' ERROR | WARN ' needle-worker-log-bf-4yjq-slot2.log

# zero error fields in the structured log
grep -c '"error"' needle-events-2026-08-12-bf-4yjq.jsonl   # → 0

# the 56 transcripts, then: no panic/trace/OOM text anywhere
tar -xzf bf-4yjq-crash-sessions-2026-08-12.tar.gz
grep -l -i -E '"panic:|goroutine [0-9]+ \[|fatal error:|out of memory|cannot allocate|Killed$' \
  sessions/*.jsonl | wc -l                                  # → 0

# every error-flagged tool result (no space after the colon in these transcripts)
grep -c '"is_error":true' sessions/*.jsonl | grep -v ':0'   # → 3 results in 2 files
                                                            #   (attempts 12, 30, 34)

# the 50 alert beads' labels and bodies (checkpoint, not `bead show` — it hides labels)
grep -h -A6 '"id": "bf-276uk"' /home/coding/domain-check/.beads/checkpoint/forensic.jsonl | head -8
# or per-bead: python3 -c "import json;[print(json.loads(l)['issue']['labels']) for l in open('/home/coding/domain-check/.beads/checkpoint/forensic.jsonl') if '\"bf-276uk\"' in l]"

# death-point truncation, per attempt (last tool_use vs any following result)
# — script in this dispatch's session; summary in §4–§5 above

# live absences (2026-09-06)
coredumpctl list --no-pager | head -2       # earliest 2026-08-17 16:01:44 EDT
journalctl --output=short-iso --no-pager | head -1   # first entry 2026-08-15 19:46:33 EDT
```

---

## Appendix A — the 50 crash-alert beads created by this storm

In creation order (worker log, `alert_id=` on each `crash alert bead created` line):

```
bf-276uk bf-3dq63 bf-59bwz bf-3ssnm bf-2fiyo bf-29rca bf-uoyie bf-2weev bf-2ftau bf-44x3a
bf-64hxa bf-3b9rv bf-1dxk7 bf-hw4i5 bf-1ygk6 bf-2j99a bf-9b8oe bf-d7j07 bf-46ttc bf-2dj1g
bf-bkpuh bf-x5ynu bf-4tl4v bf-1dzwv bf-aruwg bf-2o8p2 bf-2t7xh bf-4wi3v bf-1fvk2 bf-22514
bf-35bhc bf-3f6ue bf-mlv3u bf-5egrf bf-bykl0 bf-4tnae bf-3k3ya bf-5966o bf-vcsxj bf-19qh7
bf-mus1k bf-50zoz bf-47ugw bf-3pee6 bf-1o4ag bf-6awu2 bf-gz3r6 bf-1jxy8 bf-66h5p bf-2n3ve
```

Every alert carries needle's `alert` / `crash` / `signal--1` labels (verified in
`forensic.jsonl` for all 50); 34 also carry a `failure-count:N` escalator whose surviving
values are `1`×4, `2`×3, `4`×15, `5`×11, `12`×1. All 50 have the identical title
`ALERT: Agent crash on bead bf-4yjq` and the identical body quoted in §3.4. These 50 beads
are the reason the bf-4yjq alert pool kept re-dispatching long after the storm — the
deduplication history for them lives in `docs/crash-root-cause-bf-4yjq.md` and the
canonical report.
