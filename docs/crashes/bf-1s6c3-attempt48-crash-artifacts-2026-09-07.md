# bf-1s6c3 Attempt-48 Crash Artifact Bundle (alert bead bf-1wz2w)

**Producing bead:** domchk-02f84337 (claude-code-glm-5.3-flash-lab-roam-5), 2026-09-07
**Subject bead:** bf-1s6c3 — "Create merge commit reconciling Forgejo and GitHub histories" (CLOSED 2026-08-16T14:00:13Z, actor `system`)
**Named crash timestamp:** 2026-08-12T23:53:11.034551744+00:00 (from alert bead **bf-1wz2w**)
**Downstream consumer:** domchk-4e8821ca ("Verify crash prevention and document learnings", open)

This file fulfills the dispatch's stated output (`docs/crash-artifacts-bf-1s6c3.md`), placed
here under the chain's naming convention — each attempt-specific bundle names its attempt and
alert, as `bf-1s6c3-investigation-data-bundle-2026-09-06.md` (attempt 10 / alert bf-l3t8x) and
`bf-1s6c3-attempt16-crash-artifacts-2026-09-06.md` (attempt 16 / alert bf-2oq9d) did for
theirs. No committed doc had rendered this attempt. Every figure below was re-derived live on
2026-09-07 from the primary log and the committed extraction bundle; nothing is cited from a
prior worker's notes without re-verification. Storm-wide analysis lives in
[`docs/crash-analysis-bf-1s6c3-2026-09-06.md`](../../crash-analysis-bf-1s6c3-2026-09-06.md)
(canonical report); this bundle adds only what that report does not carry.

## 1. The named timestamp is an alert time, not a kill time

Same provenance shape as bf-l3t8x (kill + 6.40 s) and bf-2oq9d (kill + 6.26 s): the string
`2026-08-12T23:53:11.034551744+00:00` is the `Timestamp` field of the crash-report
description in alert bead **bf-1wz2w** ("ALERT: Agent crash on bead bf-1s6c3", re-verified
live via `bead show bf-1wz2w` and in `.beads/checkpoint/forensic.jsonl`). The nearest needle
event is 8.4 µs earlier. All UTC:

| Event | Timestamp | Source |
|---|---|---|
| Claim — `bead.claim.succeeded` | 23:51:16.438997488Z | extract line 901 (seq 5964) |
| Dispatch-time CPU: `fleet.cpu_saturated` load **10.86** | 23:51:16.444651300Z | full worker log |
| Dispatch — attempt **48**, `prompt_len=70670`, same `prompt_hash` as every attempt | 23:51:16.451086395Z | extract line 905 (seq 5973) |
| **Real kill — `agent.completed`, `exit_code=-1`, `duration_ms=108759`** | **23:53:05.431962513Z** | extract line 908 (seq 5976) |
| `outcome.classified` → `crash` | 23:53:05.439604166Z | extract line 910 (seq 5979) |
| heartbeat `HANDLING_RELEASE_DONE` | 23:53:11.034543302Z | extract line 916 (seq 5985) |
| **Named timestamp (bf-1wz2w description `Timestamp`)** | **23:53:11.034551744** | kill + **5.6026 s** |
| Alert bead `created_at` | 23:53:11.040235535Z | named ts + 5.7 ms |
| `bead.released` (`release_success`) + `outcome.handled` (alerted) | 23:53:13.173810038Z / .173815546Z | extract lines 917–918 (seqs 5986–5987) |
| Retry — attempt **49** claimed + dispatched | 23:53:15.412709118Z / 23:53:15.422610752Z | extract line 920 (seq 5992) |

Named timestamp = crash-handler post-release bookkeeping, **kill + 5.60 s**; crash→retry gap
**10.0 s**, no backoff. The storm did not pause: attempt 49 followed in 10 s and itself died
(exit −1, 237,174 ms) at 23:57:13.220787155Z. Use **23:53:05.431Z** for any windowing, not
23:53:11.034.

## 2. Crash log file — identified and reviewed

**Primary:** `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl` —
3,589,712 B, sha256 `3a487dc3139a2785f3ec2917f29d00121abbe109a95793163ad0a3c27352b28e`
(re-hashed today; unchanged since extraction). Byte-exact window for this attempt = extract
lines **901–920** (claim → next claim), of which the events in §1 are the load-bearing rows;
`agent.completed` sits on line 908.

**Committed extract:** `docs/crashes/bf-1s6c3/needle-events-2026-08-12-bf-1s6c3.jsonl` —
945 events, all for bf-1s6c3; attempt 48 is the 48th of the day's 49 dispatches. The extract
carries **no system-state events** (it is bead-filtered), which is why §3 reads the full
worker log. `sha256sum -c MANIFEST.sha256` re-run today: **all 5 files OK**.

**Transcript:** attempt 48 = `f818432d-d779-4d16-8ca1-b7fed172e7e3.jsonl`, row 48 of
`docs/crashes/bf-1s6c3/sessions-index.tsv` (start 23:51:17.520Z, **297,530 B — byte-count
matches the index**), member of the committed tarball
`bf-1s6c3-crash-sessions-2026-08-12_13.tar.gz` (76 members). 90 rows, 17 tool calls.

## 3. System state at crash time

| Signal | Reading | Status |
|---|---|---|
| **CPU** | `fleet.cpu_saturated` load **10.86** at dispatch; **10.16** at 23:53:15.416 (10 s after the kill); bracketed by 10.44–16.78 across 23:41–23:57 (14.0 at 23:35, 12.93 at 23:38, 16.78 at 23:46, 15.42 at 23:48, 16.65 at 23:57) | **documented** — sustained saturation on the 12-core host across the whole window |
| **Memory** | not recorded anywhere for Aug-12 | **unrecoverable** — structural, see below |
| **Disk** | not recorded anywhere for Aug-12 | **unrecoverable** — same |
| Coredumps | `coredumpctl --since "2026-08-12" --until "2026-08-14"` → **No coredumps found** (re-run today) | absent |
| Kernel OOM records | none can exist — `journalctl --list-boots` first entry **2026-08-15 19:46:33 EDT** (boot `52309698…`, re-verified today) | absent |

Memory/disk absence is structural, not a lost file: `.beads/logs/` monitoring did not exist
yet (starts 2026-09-01), system journald starts 2026-08-15 19:46 EDT (so the Aug-12 kernel
records were lost to the 2026-08-14 16:39 EDT reboot), and lab-health-collector starts
Aug-15 23:53 EDT, so memory/disk series begin there. Note also that the axis this event
violated was **repository size** (≈18 GB object store vs the 12 GiB dispatch scope), not host
memory — the canonical report's §4.3/§5.2 and the classification hold without a host-memory
reading.

## 4. What attempt 48 was doing when it was killed

From the transcript (17 tool calls — 16 read-only git/bead verification, then the push):

| # | Command (23:51:26 → 23:52:52) | Result |
|---|---|---|
| 1–6 | `git log --graph --all -20`, `git remote -v`, `git branch -a`, `git log origin/main -10`, `git log github/main -10`, `git merge-base origin/main github/main` | merge-base → `63ba024` (dead pre-squash SHA today) |
| 7–9 | `git rev-parse origin/main github/main`, `git diff` between them, `git rev-parse main` | local `main` → `412582c` (dead pre-squash SHA today) |
| 10 | `git log --oneline main..origin/main` | **empty — Forgejo had 0 commits local main lacked** |
| 11 | `git log --oneline origin/main..main` | bead-state "chore:" commits (`412582c`, `e2d87ab`, `5ea6f47`, …) |
| 12 | `bf show bf-2xygo` | re-read the prerequisite divergence analysis |
| 13 | `git show 7dd79eb --stat` | `Merge: 43bf4c3 63ba024`, 2026-08-12 17:47:07 −0400 (= 21:47:07Z), "Merge reconciliation: Forgejo and GitHub remote histories" — the pre-squash name of **`42a7b07`** |
| 14 | `git log origin/main..7dd79eb --oneline \| wc -l` | **332** |
| 15 | `git branch --contains 7dd79eb` | **`* main`** — the deliverable was already on local `main` at 23:52:40.614Z |
| 16 | — | — |
| 17 | **`git push origin main` at 23:52:52.390Z** | **no tool result ever returned** |

The kill landed **13.0 s into the push** (23:53:05.432 − 23:52:52.390), with local main
already containing the reconciliation merge. This is the push-side pack-objects memcg-OOM
mechanism later kernel-proven as bf-198ne, and the storm-wide finding holds for this attempt:
its last issued command was a `git push`.

Two first-hand data points this attempt adds to the record:

- **Direct proof the deliverable was on local main mid-storm** — `git branch --contains
  7dd79eb` → `main` at 23:52:40Z, ~2 h 5 m after the merge landed. The canonical report's
  "72 of 76 dispatches ran against an already-satisfied task" is here confirmed by an
  attempt's own tool output, not only inferred from the timeline.
- **A third divergent "divergence" figure** — attempt 48 measured **332** (`origin/main..7dd79eb`),
  against attempt 16's live **340** and bf-1wz2w's claimed **"685+"**. None is 685; all three
  are the phantom-divergence family catalogued in `docs/branch-divergence-analysis.md`
  (0/0 everywhere since 2026-08-17). Do not absorb any of them into a write-up.

## 5. Bead bf-1wz2w — alert state and four stale claims in its Notes

bf-1wz2w is **Open** (labels `alert, crash, signal--1, split-child, umbrella,
verification-failed`), blocked by domchk-4e8821ca (Open — verification layer, this bundle's
consumer) and domchk-e36b5da6 (Closed — documentation layer). Leave bf-1wz2w itself
untouched; alert closure belongs to its own closure chain. Its Notes, and the 2026-08-26
verification report `docs/bead-verification/bf-1wz2w.md` that repeats them, are **unreliable
on five counts**:

| Claim | Reality (re-verified 2026-09-07) |
|---|---|
| "Root cause: agent timeout (600s) exceeded" | Attempt 48 died at **108,759 ms** by **signal** (`exit_code=-1`, `outcome.classified=crash`) — nowhere near the 600 s cap. The four 600 s timeouts are the **exit 124** attempts on Aug-13 01:11–01:54Z, a different failure class 27 attempts later |
| "685+ commit divergence" | Attempt 48's own measurements: `main..origin/main` = **0**; `origin/main..7dd79eb` = **332**. No 685 figure appears in its transcript; divergence today is **0/0** |
| "remotes synchronized at `63ba024`" | Dead pre-squash SHA (`git cat-file` fatal today); both remotes == local `HEAD` == **`5d29b47`** (verified live) |
| "Full investigation report in `bf-4hp9p-crash-investigation.md`" | That bead investigated a **different alert** — bf-4hp9p's own report is titled "826 crashes on 2026-08-16"; the file was relocated to `docs/crash-investigations/` by the `a883044` archive consolidation. It is not attempt 48's investigation |
| "bf-1s6c3 closed 2026-08-16T14:36:03Z" (Aug-26 doc) | The forensic closed event reads **2026-08-16T14:00:13.240405326Z**, actor `system` — matching the canonical report |

The correct disposition is the canonical report's, unchanged by any of this: **Infrastructure
— repository bloat (Pattern 3)**, alert disposition "no further action" — the bead is closed,
the deliverable is represented on `main` by `46293c5`, and the repository condition is
repaired and holding.

## 6. Artifact catalog

**Committed — `docs/crashes/bf-1s6c3/`** (bundle af5ea6b, domchk-fcac734a; manifest
re-verified today, 5/5 OK):

| File | Role for this attempt |
|---|---|
| `needle-events-2026-08-12-bf-1s6c3.jsonl` | 945 events; attempt 48 window = lines 901–920 |
| `bf-1s6c3-crash-sessions-2026-08-12_13.tar.gz` | 76 transcripts incl. `f818432d-…jsonl` (attempt 48) |
| `sessions-index.tsv` | row 48 = attempt 48: start 23:51:17.520Z, 297,530 B, last cmd `git push origin main` |
| `MANIFEST.sha256`, `README.md` | integrity + bundle provenance |

**Committed — analysis layer:** `docs/crash-analysis-bf-1s6c3-2026-09-06.md` (canonical
report — storm census, §12 verification appendix); `docs/crashes/bf-1s6c3-investigation-data-bundle-2026-09-06.md`
(attempt 10 counterpart; log-source availability matrix reused in §3);
`docs/crashes/bf-1s6c3-attempt16-crash-artifacts-2026-09-06.md` (attempt 16 counterpart);
`docs/crashes/bf-1s6c3-crash-storm-timeline-2026-09-06.md` (76-dispatch census, this attempt
is row 48 of); `docs/branch-divergence-analysis.md` (phantom-divergence reference for §4).

**Live on disk:** `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl`
(sha256 §2) — the only primary witness for this attempt's `fleet.cpu_saturated` samples; dated
needle dispatch logs are never pruned (fabric-prune broken since 2026-08-17).

**Live verification re-run by this bead (2026-09-07):** repository `.git` 102 MB · 82 loose
objects / 560 KiB · 3 packs / 99.13 MiB · garbage 0 · `git fsck --full` exit 0 (two dangling
trees, normal churn) · `git ls-files .beads` → 0 · `.gitignore:66` `.beads/` (`:68` `*.db`,
`:70` `*.jsonl`) · `HEAD...origin/main` → 0 / 0 · GitHub mirror `main` == local `HEAD` ==
`5d29b47` · host today: 46 Gi memory available, 53 G disk free, load 6.07 — healthy. Dead-SHA
re-check: `412582c`, `63ba024`, `7dd79eb` all fail `git cat-file`.

## 7. Classification hints (for domchk-4e8821ca)

1. **INFRASTRUCTURE EVENT** — attempt 48 of 76 (71 × −1, 4 × 124, 1 × 0); mechanism =
   push-side pack-objects memcg-OOM on the then-≈18 GB repository, four days before
   `pack.windowMemory` bounds existed (added 2026-09-02, now covering gc *and* push;
   `./scripts/setup-git-gc-config.sh --verify` passes).
2. Window on **kill 23:53:05.431Z**, not the named 23:53:11.034Z (kill + 5.60 s).
3. Not a domain-check code defect; not a service failure; not max-turns exhaustion
   (108.9 s runtime, signal death).
4. Task already satisfied when this attempt died — **proven by its own transcript**
   (`git branch --contains 7dd79eb` → `main`) — but the death itself was real and mid-push,
   so this is not a false positive at the alert's own level.
5. Treat as unreliable: bf-1wz2w's Notes and `docs/bead-verification/bf-1wz2w.md` (five
   counts in §5) — and any "685+", "666", "340" or "332" divergence figure, which are all
   stale pre-squash snapshots of a divergence that is 0/0 today.

## 8. Classification and root-cause determination (domchk-00e228b9, 2026-09-07)

Applied per [`docs/crash-response-guide.md`](../../crash-response-guide.md): Quick Reference
exit-code table row 2 + note 2, the crash-classifier table's INFRASTRUCTURE row, False-Positive
Rules 1–3 (with the domchk-64e1461a storm caveats), and **Pattern 3 (Infrastructure —
Repository Bloat)**. Chain context: this is the gather → **classify** → fix chain's middle link
(child 1 = domchk-02f84337, this document; child 3 = domchk-e9234a0d, apply the matched fix).
Every load-bearing figure in §1–§5 was re-verified live before classifying — extract lines
901–920 (claim seq 5964 → retry claim seq 5992, `agent.completed` seq 5976 = `exit_code=-1`,
`duration_ms=108759`), alert bead bf-1wz2w's `Timestamp` = 23:53:11.034551744Z, transcript
f818432d (90 rows / 17 tool calls / 297,530 B, `git branch --contains 7dd79eb` → `* main` at
23:52:40.614Z, `wc -l` = 332, push issued 23:52:52.390Z with **no tool result ever returned**),
bf-1s6c3 closed event 2026-08-16T14:00:13.240405326Z actor `system`, and today's repaired
state (`.git` 102 MB · 134 loose objects · in-pack 11,182 · garbage 0 · `fsck --full` exit 0,
dangling trees only · 0/0 divergence · gc-bounds `--verify` exit 0).

### 8.1 Verdict

| | |
|---|---|
| **Category** | **INFRASTRUCTURE EVENT** |
| **Specific mechanism** | **Push-side pack-objects memcg-OOM** — `git push origin main` was 13.0 s in flight (23:52:52.390 → 23:53:05.432Z) when the kill landed, pack-objects inside the 12 GiB dispatch scope against the then-≈18 GB object store. The bf-198ne mechanism, four days before `pack.windowMemory` bounds existed (added 2026-09-02, now covering gc *and* push) |
| **False positive?** | **No** — see §8.3; the death was real and mid-task |
| **Alert disposition** | No further action for the crash event itself: subject closed, deliverable on `main` (`46293c5`), repository condition repaired and verified holding. Alert closure belongs to bf-1wz2w's own closure chain |

### 8.2 Exit-code mapping and Pattern-3 criteria

`exit_code = -1` with `outcome.classified = crash` maps to **Infrastructure** via guide note 2
(`-1` is needle's `wait()` sentinel for *died by signal*, not a signal number). Pattern 3,
criterion by criterion for this attempt:

| Pattern 3 criterion | Attempt 48 | Status |
|---|---|---|
| Signal death, `exit -1` | seq 5976, `exit_code=-1`, classified `crash` | ✅ re-verified from extract |
| Repository > 5 GB | ≈18 GB at crash time (canon-sourced); 102 MB today | ✅ |
| Routine git operation triggers the kill | The kill's command **is** the push — 16 of 17 tool calls were read-only verification that completed cleanly; the first mutating git call is what died | ✅ from transcript |
| Zero exit-code variation | Storm-wide 71 × −1; this attempt is one of them, no variation | ✅ |
| Fixed-cadence re-dispatch | Retry claimed **10.0 s** after the kill (23:53:15.412Z); attempt 49 died the same way at 23:57:13.220Z | ✅ |

**Excluded alternates** (guide Phase 2B/2C/2D):

- **Workflow failure** — requires `exit 1` + `error_max_turns`. Attempt 48 was signal-killed at
  108.9 s, 18% of the 600 s dispatch cap; the four genuine timeouts are the exit-124 attempts
  on Aug-13, a different class 27 attempts later. This also refutes bf-1wz2w's Notes claim
  "agent timeout (600s) exceeded" (§5).
- **Service failure** — no gateway 5xx at any point; the failure was local, instantaneous, and
  precisely coincident with the push.
- **Code defect** — the killed process was git's own pack-objects; the task never touched
  domain-check code. Consistent with the standing no-defects finding.

### 8.3 False-positive rules — applied

| Rule | bf-1wz2w / attempt 48 | Verdict |
|---|---|---|
| **1. Work committed < 30 s before crash** | Last commit was `7dd79eb` at 21:47:07Z — **2 h 6 m** before the kill — and the kill landed mid-push, mid-attempt | **Not triggered** |
| **2. Crash → retry → success** | Attempt 49 died the same way 4 min later; the eventual attempt-76 exit 0 is the guide's documented *surface* match (persistent cause outlasted the kills, not healed) | **Surface match only — no downgrade** |
| **3. 10+ crashes / 10 min** | 1 kill in this window (storm-wide 2.68/10 min) | **Not triggered** — its absence is part of Pattern 3's signature (bloat is per-repository and persistent, not system-wide and instantaneous) |

**The two-layer reading, resolved for this attempt.** The alert does carry the
deliverable-landed-mid-storm component — `git branch --contains 7dd79eb` → `* main` at
23:52:40.614Z proves from the attempt's own transcript that the task was already satisfied
~12 s before it issued the push — but under the guide's Rule-1 storm caveat that is
verify-then-close debt on the *subject*, not a downgrade of the *death*: the kill was real, it
was mid-task, and its mechanism is the same one that killed 70 other attempts. **Classification:
INFRASTRUCTURE — repository bloat (Pattern 3); not FALSE_POSITIVE, not Workflow, not Service,
not Code defect.**

### 8.4 Canonical-record sync

Per the workspace dedup rule, a dated re-verification subsection resolving the **bf-1wz2w /
attempt-48** instant (the last of the storm's named instants without one) was appended to
[`docs/crash-analysis-bf-1s6c3-2026-09-06.md`](../../crash-analysis-bf-1s6c3-2026-09-06.md) §12.
The verdict above is §5's, unchanged — this section renders it for this chain's specific
attempt and adds no new mechanism.
