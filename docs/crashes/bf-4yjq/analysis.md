# bf-4yjq — Crash Evidence Analysis

**Analyze leg:** domchk-ec2cbd03 ("Analyze crash evidence and determine root cause"),
executed 2026-09-09 at HEAD `af94e85`.
**Subject bead:** `bf-4yjq` — *"Git origin remote points to GitHub directly; Forgejo mirror has
diverged/gone stale"* (P2, type `task`, **Closed** rev 2, 2026-08-17T00:14:14Z — re-read live
this dispatch).
**Workspace:** `/home/coding/domain-check`, worker `claude-code-glm-4.7-lab-domain-check`,
needle session `8446529e`, agent `claude-code-glm-4.7`.

This file is the dispatch's tasked deliverable at its named path. It derives **no new cause
claim** — the classification and root cause are canon
(`docs/crashes/bf-4yjq-evidence.md` §9–§10, `docs/crashes/bf-4yjq-report.md`), and this
analysis re-checks them against the primary sources rather than restating from memory. Every
load-bearing figure below was **re-derived first-hand at this dispatch**; where a figure
agrees with the prior legs, that agreement is the verification.

---

## 1. Verdict

| Question | Answer |
|---|---|
| Crash classification | **INFRASTRUCTURE — repository-bloat sub-type** |
| Root cause | Unbounded `git push` (pack-objects) over the bloat-era 18 GB store, inside needle's 12 GiB memcg dispatch scope → memory-cgroup OOM kill → needle's `exit_code=-1` sentinel |
| Real crash or false positive? | **Real crash** — 50 genuine mid-task kills of one bead. The *alert layer* those kills minted is the false-positive surface (stale alerts against a bead that closed 2026-08-17), dispositioned in §5 |
| Work lost | **None.** The task completed and the bead closed five days after the storm (split child `bf-2xygo` finished the remote work the same evening) |
| Code defect | **None.** No panic, stack trace, core dump, or application error anywhere in the chain — consistent with the corpus-wide zero-defect finding for domain-check |

## 2. The four candidate classes, checked

The dispatch's classification checklist, each row answered from evidence re-derived this
dispatch:

| Candidate | Verdict | Basis (this dispatch) |
|---|---|---|
| **Infrastructure event** (memory pressure / OOM / SIGHUP) | **YES — this is the class** | 50/56 claims died `exit_code=-1` with zero exit-code variation (worker-log slot `.log.2`, re-parsed live); `.git` was **18 GB** at crash time (4,594 loose objects / 17.20 GiB vs 9.60 MiB packed — contemporaneous snapshot); every crash run's last recorded command is the task's own `git push` (full 56-session transcript census, §4). Guide Quick Reference row: −1 + fixed-cadence re-dispatch deaths + `.git` > 5 GB → **Repository bloat** |
| Agent workflow failure (max turns / bead-closing) | No | Exactly one workflow-class event in the whole window — attempt 2's `exit_code=1` `Failure` (18:00:17Z), whose transcript ends at `git add -A && git commit …`, not an `error_max_turns` signature. The 4 `exit 124` dispatch-cap timeouts (20:40:47 → 21:11:27Z) are the storm's *tail*, downstream of it |
| Service failure (inference gateway 503/502) | No | No HTTP 5xx signature anywhere in the surviving telemetry; the guide's exit-1 service row never occurs in the census |
| Code defect | No | No stack traces or core dumps exist (SIGKILL-class deaths produce none; `coredumpctl` floor 2026-08-17, unrelated). The killed work is a git operation, not domain-check code |

## 3. Root cause

**Mechanism chain:** repository bloat (17+ identical 237 MB `.beads/*.jsonl` snapshots
committed by bf-2ildm → 18 GB store, loose:packed ratio inverted) → each re-dispatch re-ran the
remote-reconciliation task to its terminal `git push origin main` step, whose pack-objects is
the memory-hungry process over that store → the worker runs inside a systemd dispatch scope
capped at 12 GiB → memory-cgroup OOM kill (`CONSTRAINT_MEMCG`, uncatchable SIGKILL) → needle
records its no-wait-status sentinel `exit_code=-1`, releases the bead, mints an alert →
re-dispatch into the unchanged environment → repeat 50 times.

Two provenance qualifiers, stated the way the canon states them:

- **`-1` is a sentinel, not a signal number.** It is needle's value for a child that died
  without reporting a wait status.
- **Regime-matched, not kernel-proven, for Aug-12 specifically.** The system journal's single
  boot (`52309698`) begins **2026-08-15 20:01:33 EDT** — re-verified live this dispatch — so the
  Aug-12 kernel OOM records are unrecoverable. The mechanism is pinned by uniform per-run
  evidence (50/50 death command, fixed cadence, the 18 GB store) plus the kernel-proven
  siblings: **bf-198ne** (2026-08-16) is the *push-side* variant of this exact OOM, and
  bf-4x12ec the gc-side variant.

**Amplifier, not cause:** needle's per-kill alert minting (one alert bead per death, 50/50,
created 7–13 s after each kill — first `bf-276uk` at 17:54:02.643Z, 8.8 s after kill 1; last
`bf-2n3ve` at 20:30:45.743Z) turned one undrainable repository condition into a 50-bead alert
wave. The kill cadence itself was fixed: kill→kill gap min **75.6 s** / median **155.5 s** /
mean **191.9 s** / max **576.8 s** (n=49) over a **9,404 s** window (17:53:53.875Z →
20:30:38.310Z).

## 4. Evidence (re-derived this dispatch, 2026-09-09, HEAD `af94e85`)

1. **Kill census — primary worker-log slot, re-parsed live.**
   `~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2` (134 MB, still present,
   one rotation from erasure): **56 `claim_auto` claims; 50 × `Crash(-1)`; 1 × `Failure(1)`;
   4 × `Timeout(124)`; 1 × `Success(0)`** — byte-identical to the canon census.
2. **Death point — full transcript census, re-run.** All 56 per-run transcripts from the
   committed bundle `docs/crash/bf-4yjq/raw-logs/bf-4yjq-crash-sessions-2026-08-12.tar.gz`
   were walked this dispatch for each session's last Bash tool call: **50/50 crash sessions end
   at `git push`**; the 6 exceptions are exactly the non-crash outcomes (exit-1 →
   `git add -A && git commit`; exit-0 → `bf show bf-2xygo`; the four exit-124 timeouts end at
   two no-Bash sessions, one `git log`, one `bf create`). Uniformity across 50 runs pins the
   death point to the push step — the guide's Pattern 3 symptom "routine git operations trigger
   OOM."
3. **Target bead state — re-read live.** `bf-4yjq` **Closed rev 2**, updated
   2026-08-17T00:14:14Z, close reason recording the reconciled remotes and the Forgejo push
   mirror's 2026-08-17T00:11:34Z successful sync. **No work was lost** to the storm.
4. **Environment, then vs now.** Crash time: 18 GB `.git` / 4,594 loose objects / 17.20 GiB
   (contemporaneous snapshot — the only contemporaneous figures that survive; its crash count
   is superseded by this census). Now: `.git` **106 MB**, 109 loose objects / 736 KiB, one pack
   (12,607 objects), 0 garbage, 0 unpushed; `check-repo-health.sh` **exit 0**;
   `setup-git-gc-config.sh --verify` **exit 0** (effective worst-case pack memory ≈3072 MiB,
   within the 12 GiB dispatch scope); `crash-pattern-detection.sh` **STABLE, 0 crashes/24 h**.
   The regime that produced these kills is gone, and the layered prevention (`.beads/`
   gitignored, 10 MB pre-commit gate, persistent `pack.windowMemory`/`pack.threads` bounds,
   daily timers) is what keeps it gone.
5. **Storm-shape corroboration.** Zero exit-code variation across 50 kills plus a fixed cadence
   plus `.git` > 5 GB is the guide's Pattern 3 signature checklist at **6/6** (formally derived
   in `bf-4yjq-evidence.md` §10.2, which also recomputes the surge windows: a 3-kill 300 s
   window from 18:18:13Z and a 5-kill 600 s window against the committed detector's
   `CRASH_SURGE_THRESHOLD=3` — an infrastructure-event verdict would have fired).

## 5. Real crash vs false positive

**The crashes are real.** Applying the guide's False Positive Detection Heuristics
(re-checked this dispatch; formally derived in `bf-4yjq-evidence.md` §10.3):

- **Rule 1 (post-completion gap): negative, both readings.** Every kill is mid-task — the last
  recorded command in all 50 crash sessions is the task's own push step, not cleanup. The
  deliverable-in-storm-window corollary is also negative: the remote reconciliation was not
  satisfied anywhere inside the 9,404 s window, so no retry ran against already-completed work
  (the contrast with bf-1s6c3, where it did).
- **Rule 2 (crash → retry → success = self-healed?): surface match only — the caveat bites.**
  The exit-0 attempt (56) issued **zero git operations**; its 14 substantive commands are all
  `bf`-CLI split bookkeeping. It survived by *abandoning* the death operation (task-shape
  change), not because the environment improved — the store was still 18 GB on 2026-08-12; the
  repair landed 2026-09-01. The storm must **not** be recorded as self-healed.
- **Rule 3 (system-wide event): positive.** Fixed-cadence death waves are an environmental
  regime by the guide's own definition; bf-1s6c3 ran the same regime the same evening.

**The alert layer is where the false positives live.** Each of the 50 kills minted an
`ALERT: Agent crash on bead bf-4yjq` bead, and the target bead closed five days later — so the
surviving alerts are stale-by-closure. Live census this dispatch: **50 ALERT beads — 36 Closed
/ 12 Open / 2 InProgress** (`bf-2j99a`, `bf-vcsxj` held by concurrent workers) — identical to
the classify leg's count hours earlier, i.e. no drift. Retirement of those alerts belongs to
the alert-lifecycle owners (bf-3dxljn disposition: target-resolution + dedup gate first); it
does not un-kill the workers, and it is not this leg's action.

## 6. Artifacts reviewed

The task's named artifact directory `docs/crashes/bf-4yjq/` did not exist before this
deliverable — the bf-4yjq record lives as flat files. Reviewed this dispatch:

| Artifact | Role |
|---|---|
| `docs/crashes/bf-4yjq-evidence.md` | Gather + classify legs (§1–§9 evidence, §10 classification) — canon |
| `docs/crashes/bf-4yjq-report.md` | Report leg (domchk-b6e16f96, commit `ad0018c`) — canon synthesis |
| `docs/crashes/bf-4yjq-cleanup-verification.md` | Repository repair record (18 GB → 92 MB, re-verified) |
| `docs/crash/bf-4yjq/raw-logs/` | Primary sources: worker-log copies, structured event log, the 56-session transcript bundle |
| `docs/crash-response-guide.md` | Classification framework, FP heuristics, Runbook A |
| `~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2` | Live primary worker-log slot (225 `bf-4yjq` records) |

## 7. Chain position

This leg is the analyze child of the bf-4yjq template chain: gather re-fire `domchk-cc71d6c0`
(**Closed** rev 4 — re-read live) → **this bead** → report child `domchk-52b232c5` (still Open,
a duplicate of the already-landed report leg `domchk-b6e16f96`; its disposition belongs to its
own close dispatch, not this one). The recommend leg landed as `af94e85`
(mitigation-strategies v2.4 erratum). Nothing in this analysis alters the canon
classification, the root cause, or any prior leg's figures — every figure re-derived here
matched them.
