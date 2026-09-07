# Crash Analysis: Bead bf-1s6c3 (2026-08-12 → 08-13 kill storm)

**Analysis Date:** 2026-09-06
**Subject Bead:** bf-1s6c3 — "Create merge commit reconciling Forgejo and GitHub histories"
**Documentation Bead:** domchk-ed3ed12b
**Status:** ✅ RESOLVED — subject bead closed 2026-08-16, repository repaired and verified holding
**Classification:** Infrastructure — repository-bloat sub-type (crash-response-guide Pattern 3)
**Confidence:** High (~95% class; ~90% sub-type, epistemic caveat in §5)
**Format template:** `docs/crashes/bf-173o7e-report.md`, `docs/crash-analysis-domchk-c9641ac5-2026-09-01.md`

> **Authority note.** This report is the write-up layer of an investigation chain that worked
> from raw artifacts. Where earlier bf-1s6c3 docs (the 2026-08-26 / 2026-09-01 corpus)
> contradict it, the earlier docs are wrong — §11 lists each correction. Every load-bearing
> figure here was re-verified first-hand on 2026-09-06, both by the predecessor beads in the
> chain and again by this bead (§12).

---

## 1. Executive Summary

**CRITICAL FINDING:** This was an **infrastructure event** — a kernel memcg-OOM SIGKILL of
`git push`'s pack-objects on an ≈18 GB repository, **re-executed 71 times by an automated
re-dispatch loop over 4 hours 30 minutes**. It was NOT a code defect, NOT a single crash, and
NOT the "post-completion false positive" the 2026-09-01 docs classified it as.

### Key Facts

| Attribute | Value |
|---|---|
| Exit codes | **−1 × 71** (signal death, code unrecorded) · 124 × 4 (600 s timeout) · **0 × 1** (success) |
| Storm span | 2026-08-12T21:31:27Z → 2026-08-13T02:01:27Z (265 min, 76 dispatch attempts) |
| Death cadence | median inter-completion gap 177 s; ~10 s claim→dispatch re-dispatch cycle |
| Kill density | 2.68 kills / 10 min (16.1/hour) |
| Agent / model | `claude-code-glm-4.7` / glm-4.7, worker slot `claude-code-glm-4.7-lab-domain-check`, session `8446529e` |
| Repository at crash time | ≈18 GB `.git`, ≈17 GB loose objects (canon-sourced; not re-measurable — see §4) |
| Mechanism | memcg OOM SIGKILL of pack-objects inside the 12 GiB dispatch scope (`MemoryMax`) |
| Deliverable | merge `42a7b07` landed mid-storm (attempt 4, 21:47:07Z) and survived on disk |
| Work lost | **None** — remotes reconciled by the later merge `46293c5`; zero divergence today |

### Resolution Status

| Aspect | Status | Notes |
|---|---|---|
| Root cause identified | ✅ Complete | Repository bloat → memcg OOM at the push step (§6) |
| Code defects found | ✅ None | Task was a git history reconciliation; domain-check code never touched |
| Work loss | ✅ None | Deliverable on `main` via `46293c5`; remotes converged (verified 2026-09-06) |
| Repository repaired | ✅ Holding | 18 GB → ~100 MB on 2026-09-01; re-verified 2026-09-06 (§12) |
| Prevention layers | ✅ 6 of 6 in force | In-repo gap (§8 item 3) **closed 2026-09-07** — installer committed as `dfa60a9`; the NEEDLE-side lever (§8 item 6) lives outside this repository |

---

## 2. What Bead bf-1s6c3 Was Trying to Accomplish

- **Title:** Create merge commit reconciling Forgejo and GitHub histories
- **Created:** 2026-08-12T21:12:09Z · **Priority:** P2 · **Type:** task
- **Origin:** follow-up execution bead to bf-2xygo (divergence analysis), which exited 0 at
  21:31:21.362Z — bf-1s6c3's first claim followed six seconds later
- **Task:** merge the diverged Forgejo (`origin`) and GitHub (`github`) histories locally and
  push the reconciled result, per the workspace's "merge, never force-push" rule

### Task Outcome: ✅ DELIVERED, ❌ DELIVERY BLOCKED — by the repository, not the agent

| Acceptance criterion | Outcome |
|---|---|
| A merge commit combining both histories | ✅ `42a7b07` (attempt 4, 21:47:07Z, parents `47e7758` + `00117cb`) |
| Merge message explains what was merged | ✅ "Merge reconciliation: Forgejo and GitHub remote histories" |
| Local main contains the reconciled history | ✅ today — via the **later** merge `46293c5` (2026-08-17); `42a7b07` itself was orphaned onto `pre-squash-history-20260816` by the 2026-08-16 history squash and is **not** an ancestor of `main` |
| Both remote histories reconciled | ✅ today — Forgejo and GitHub both at the same commit as local (`c8dc3cf`, verified live 2026-09-06) |
| Push of the merge (implicit) | ❌ never achieved during the storm — **71 of 76 attempts died at the push step**; the remotes were reconciled afterwards by other work |

---

## 3. Crash Timeline

All timestamps UTC. Sources: committed raw needle-event extracts
(`docs/crashes/bf-1s6c3/needle-events-2026-08-1{2,3}-bf-1s6c3.jsonl`), session transcripts
(`bf-1s6c3-crash-sessions-2026-08-12_13.tar.gz`, indexed in `sessions-index.tsv`), live `git`.

| Time (UTC) | Event |
|---|---|
| 08-12 21:12:09 | Bead bf-1s6c3 created |
| 08-12 21:31:21.362 | Prerequisite bf-2xygo's 5th attempt succeeds (exit 0) |
| 08-12 21:31:27.663 | **First claim** (`bead.claim.succeeded`, seq 4641) — 6 s later |
| 08-12 21:31:27.674 | First `agent.dispatched` (template `pluck-default`, `prompt_len=70670`) |
| 08-12 21:36:44.519 | **Attempt 1 dies** — `exit_code=-1`, duration 316,572 ms; classified `crash`, handled `alerted`. Last tool call: `git push origin main` at 21:36:29.421, transcript ends with no result |
| 08-12 21:43:21.712 | Attempt 4 dispatched (session `217a276f`) |
| 08-12 21:47:07 | **Attempt 4 creates the deliverable** — merge `42a7b07` |
| 08-12 21:48:06.650 | **Attempt 4 killed** — `exit_code=-1`, duration 285,151 ms, **59.6 s after committing**. Commit survives on disk; the agent did not |
| 08-12 21:48:16.519 | `bead.released` — crash handler re-queues the bead |
| 08-12 21:48:18.804 | `bead.claim.succeeded` — re-claimed 2.3 s later |
| 08-12 21:48:18.815 | `agent.dispatched` — **~10 s re-dispatch cycle, no backoff, no resource gate** |
| 08-12 21:48 → 08-13 02:01 | **72 further dispatches** against an already-satisfied task. 49 attempts on Aug-12 (all exit −1), 27 on Aug-13 (22 × −1, 4 × 124, 1 × 0) |
| 08-13 01:11:08 → 01:54:45 | The four 600 s timeouts (durations 600,018–600,024 ms — exactly the cap) |
| 08-13 ~01:55–02:00 | Attempts 74–75 issue **no tool calls at all** inside the 600 s window; attempt 75 is the first **auto-split** dispatch (`prompt_len=2868`, transcript opens `## Auto-Split: Decompose This…`) — the task shape changes |
| 08-13 02:01:22.561 | **Attempt 76 exits 0** (384,204 ms) — an auto-split that decomposed the bead into four child beads using only `bf` commands, never touching git. `verification.passed` → `bead.orphaned` 02:01:27.732 |
| 08-16 14:00:13 | Bead **closed** (actor `system`); close reason cites the merge as `7dd79eb` — a dead pre-squash SHA of `42a7b07` (§11) |

The "crash timestamp" carried by the 2026-09-01 docs (`2026-08-12T21:36:51.240Z`) is a real
instant but not "the" crash — it falls inside attempt 1's post-crash handling window and is
merely the first of 71 identical deaths.

---

## 4. Artifact Analysis

### 4.1 Artifacts examined

| Artifact | Path | Contents |
|---|---|---|
| Needle event extracts | `docs/crashes/bf-1s6c3/needle-events-2026-08-12-bf-1s6c3.jsonl` (945 events), `…-2026-08-13-bf-1s6c3.jsonl` (513) | Byte-exact `grep` lines from the worker logs; `MANIFEST.sha256` covers all files |
| Session transcripts | `docs/crashes/bf-1s6c3/bf-1s6c3-crash-sessions-2026-08-12_13.tar.gz` | All 76 crash-window Claude Code transcripts (5.0 MB compressed) |
| Attempt index | `docs/crashes/bf-1s6c3/sessions-index.tsv` | One row per attempt: session start, UUID, bytes, **last-issued command** |
| Bead close record | `.beads/checkpoint/forensic.jsonl` | The 2026-08-16 `closed` event and its close reason |
| Collection bead | domchk-fcac734a (`docs/crashes/bf-1s6c3/README.md`) | Provenance for everything above |

The location named in the original dispatch (`.beads/crashes/`) **does not exist**; the real
primary sources are `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-1{2,3}.jsonl`
(still on disk — fabric-prune has been broken since 2026-08-17, so Aug-12 logs were never
deleted).

### 4.2 What the raw log shows (recounted first-hand from the committed extracts, 2026-09-06)

- `agent.completed` × 76: **exit −1 × 71, exit 124 × 4, exit 0 × 1** — matches the committed
  timeline exactly
- `outcome.classified`: `crash` × 71, `timeout` × 4, `success` × 1. Every attempt produced an
  explicit classification — **no silent-death gaps**, so no log bracketing was needed
- `outcome.handled`: **`alerted` × 71** — one alert per kill, 71 alerts for one undrainable cause
- `transform.completed` succeeded on all 76 attempts — template rendering was never the
  failure point

**Where the 71 signal deaths happened.** `sessions-index.tsv` records the last tool call each
session issued before its transcript ends mid-flight:

| Last-issued command | Attempts |
|---|---|
| `git push origin main` | 60 |
| `git push github main` | 10 |
| `git push` | 1 |
| `git fetch origin && git fetch github` | 1 |
| `git commit -m "chore: update bead tracking state before merge reconciliation…"` | 1 |
| `bf show bf-4k2ws && …` (the successful auto-split) | 1 |
| (no tool use recorded) | 2 |

**71 of 76 attempts died with a `git push` as their last issued command.** Attempts 69–70 show
the agent switching remotes trying to get through. This is the signature of the mechanism:
the kill lands inside `git push`'s pack-objects, not inside the merge itself.

### 4.3 What the artifacts *cannot* show

- **No kernel records exist for Aug-12.** System journald on this box starts
  2026-08-15 19:46 EDT — three days after this crash. There are no memcg `oom-kill` lines, load
  averages, or disk readings for the storm. Any document quoting host memory/CPU figures *at
  crash time* (the 2026-09-01 docs' "<2 GB available" claim) is **reconstruction, not
  measurement**.
- **The bloat is canon-sourced, not re-measurable.** The offending blobs (17+ identical
  ~237 MB `.beads/*.jsonl` snapshots) were packed away on 2026-09-01; the largest blob
  surviving in the object store today is 14,970,288 bytes (a Mach-O executable, 6 identical
  copies). The ≈18 GB figure rests on the cleanup-verification docs
  (`docs/crashes/bf-4yjq-cleanup-verification.md:74-84`), which measured it during the
  verified cleanup.
- **The kill mechanism is kernel-proven only for the later siblings** — bf-4x12ec (gc variant)
  and bf-198ne (push variant) have recovered kernel `oom-kill` records
  (`docs/crashes/bf-198ne-crash-report.md`). For bf-1s6c3 itself the classification rests on
  the Pattern-3 signature (§5), which is fully present.

### 4.4 Live verification (this bead, 2026-09-06)

| Check | Result |
|---|---|
| `git cat-file -t 42a7b07` | commit — "Merge reconciliation: Forgejo and GitHub remote histories", 21:47:07Z, parents `47e7758` + `00117cb` |
| `git merge-base --is-ancestor 42a7b07 main` | **not** an ancestor; contained only by `pre-squash-history-20260816` |
| `git merge-base --is-ancestor 46293c5 main` | **is** an ancestor — "Merge Forgejo and GitHub histories" (2026-08-17) |
| `git cat-file -t 2832106` / `7dd79eb` | **"Not a valid object name"** — both are dead SHAs (§11) |
| Repository | `.git` 101 MB · 5 loose objects / 36 KiB · 3 packs totaling 99.13 MiB · **0 garbage** |
| Bead-state re-entry | `git ls-files .beads` → **0**; `.gitignore:66` `.beads/` |
| Divergence | local `HEAD` == Forgejo `origin/main` == GitHub mirror == `c8dc3cf` (`rev-list --left-right --count` → 0 / 0; `git ls-remote` on the mirror) |

---

## 5. Classification Rationale

Classified per `docs/crash-response-guide.md`: Quick Reference exit-code table (row 2) and
note 2, False-Positive Detection rules 1–3, and Common Crash Patterns **Pattern 3
(Infrastructure — Repository Bloat)**.

### 5.1 Exit-code mapping → infrastructure

71 of 76 completions are `exit -1` with **zero exit-code variation among the deaths**. Per the
guide's note 2, `-1` is needle's `wait()` sentinel for *died by signal, code unrecorded* — it
is not a signal number (a SIGKILL death would encode as 137, a SIGHUP death as 129). The mapped
class is **Infrastructure event**.

### 5.2 Pattern 3 signature — every criterion present

| Pattern 3 criterion | bf-1s6c3 | Status |
|---|---|---|
| Fixed-cadence re-dispatch deaths, minutes apart, for hours | 71 deaths at median 177 s over 265 min; re-claimed within ~10 s of each kill | ✅ verified from raw log |
| Repository > 5 GB | ≈18 GB `.git`, ≈17 GB loose objects at crash time | ✅ canon-sourced |
| Routine git operations trigger the kill | The task *was* a git operation; each attempt redid expensive git work and died at its push | ✅ by task definition + last-command table |
| Zero exit-code variation | All 71 deaths exit −1 | ✅ recounted |
| Sits inside a same-mechanism same-evening storm | Between bf-4yjq's 50 kills (17:54–20:30Z) and this storm (from 21:31Z), same evening, same repo condition | ✅ per committed timeline |

**Epistemic caveat (why ~90%, not 100%, on the sub-type):** two legs rest on contemporaneous
documentation rather than re-measurable state — the 18 GB size survives only in cleanup-era
docs, and the specific kill (memcg `CONSTRAINT_MEMCG` SIGKILL) has no Aug-12 kernel record
because journald did not yet exist on this box. The classification rests on the Pattern-3
signature, which is fully present, not on a kernel record that cannot exist.

### 5.3 False-positive rules — applied, with the nuance recorded

| Rule | Threshold | bf-1s6c3 | Verdict |
|---|---|---|---|
| 1. Work committed < 30 s before crash | < 30 s | Attempt 4 committed at 21:47:07Z, killed 21:48:06.650Z — **59.6 s**, and deaths were mid-attempt (median run ~161 s) | **Not triggered** |
| 2. Crash → retry → success → self-healed transient | final retry exits 0 | Attempt 76 did exit 0 — surface match, but the cause was **persistent** (18 GB repo), not healed; the retry loop merely outlasted the kills | **Surface match only — does not downgrade** |
| 3. 10+ crashes / 10 min → system-wide event | ≥ 10 / 10 min | 2.68 kills / 10 min (bounded by each attempt's 62–431 s runtime) | **Not triggered** — recorded because its absence is part of the signature: bloat is *per-repository and persistent*, not system-wide and instantaneous |

**The two-layer reading.** Earlier docs labeled this event "FALSE POSITIVE — post-completion
infrastructure event". That premise does not hold: workers died **mid-task** for the entire
storm. There is nevertheless a genuine false-positive *component* — the deliverable had landed
at 21:47:07Z, so 72 of 76 dispatches ran against an already-satisfied task. The two questions
have different answers:

- **What killed the workers:** Infrastructure — repository bloat (Pattern 3). Not a workflow
  artifact, not a service outage, not a code defect.
- **What the alert warrants:** nothing further. The bead is closed, the deliverable is
  represented on `main`, and the repository condition is repaired and verified holding.

### 5.4 Excluded alternates

- **Workflow failure** — requires exit 1 + `error_max_turns`. None: every death is a signal
  death; `transform.completed` succeeded on all 76 attempts.
- **Service failure** — requires HTTP 503/502 to the inference gateway. No 5xx in any of the
  76 attempts; no gateway-failure signature in the day log.
- **Code defect** — no application error in any attempt; the task never touched domain-check
  code. Consistent with the standing finding that no domain-check code defect has ever been
  confirmed in this workspace's crash record.

---

## 6. Root Cause Analysis

### Immediate cause — pack-objects vs. the dispatch scope

Every dispatch executed significant git work against an ≈18 GB repository whose object store
exceeded the dispatch scope's memory budget (`MemoryMax=12GiB`,
`docs/maintenance/repository-maintenance-guide.md:158`). Pushing an 18 GB repository forces
pack-objects to materialize a pack far past that bound; the kernel killed the worker
mid-attempt each time, and needle recorded the unrecorded-code deaths as `exit -1`. The 71
deaths clustering at the push step (§4.2) — rather than at the merge — identify the specific
operation. The mechanism is kernel-proven for the better-instrumented siblings: bf-4x12ec
(`git gc` variant) and bf-198ne (`git push` variant, re-verified resolved 2026-09-06).

### Amplifying cause — why it ran for 4.5 hours

Needle's crash handler released and immediately re-claimed the bead on a ~10 s cycle with no
backoff and no resource gate. The deliverable had already landed at 21:47:07Z, but **no
stop-condition existed for "deliverable present, bead still open"** — the completion path still
required the push, and the push was memory-doomed. So 72 further dispatches re-ran an
unboundedly expensive operation against satisfied work until one attempt happened to survive —
and it survived by *changing the task shape* (auto-split into bead-only children), not because
any resource improved. The repo stayed bloated until 2026-09-01.

The same loop multiplied the alert load 1:1: each of the 71 kills emitted
`outcome.handled action=alerted`.

### Underlying cause — bead state committed to git

17+ identical ~237 MB `.beads/*.jsonl` bead-state snapshots had been committed to the
repository, bloating the object store — the same underlying cause as bf-31mno (350 kills that
day, largest single storm in the record), bf-4yjq (50), and bf-2xygo (4), all earlier the same
evening. Across all needle slots on 2026-08-12 the event recorded **460 `exit_code=-1`
completions** (455 in the domain-check slot), concentrated in five beads.

---

## 7. Impact Assessment

| Dimension | Impact |
|---|---|
| **Work loss** | **None.** The deliverable survived on disk; the on-`main` reconciliation is `46293c5`; both remotes carry identical history today (`c8dc3cf`, verified live 2026-09-06) |
| **Data integrity** | None at risk — `git fsck --full` clean; 0 garbage objects; nothing corrupted by the kills |
| **Compute** | 71 dead attempts × 62.5–431 s ≈ **5.5 hours of agent compute** producing zero pushed bytes |
| **Alert load** | **71 crash alerts** for one undrainable cause — the alert-storm shape of bf-173o7e (131 duplicate alerts) and bf-31mno (350) |
| **Workflow debt** | The auto-split left four children; the merge+push never flowed through them. bf-4k2ws closed; **bf-31p3g InProgress, bf-7d8l5 Open, bf-6b0fl Open** — now verify-then-close debt, since the remotes were reconciled by other work |
| **Record integrity** | The bead's own close reason cites a dead SHA (`7dd79eb`), and the same error propagates through the Aug-16 close wave of the alert pool — future readers must use `46293c5` for acceptance checks (§11) |

---

## 8. Prevention Recommendations

Ordered as in the classification's remediation path; every in-repo layer re-verified live on
2026-09-06.

| # | Layer | Status |
|---|---|---|
| 1 | Pack down the bloated object store — `scripts/safe-git-gc.sh`, **never** bare `git gc --aggressive` | ✅ Done 2026-09-01; verified holding: ~100 MB, `fsck` clean, 0 garbage |
| 2 | Bead state cannot re-enter git — `.gitignore` covers `.beads/`, `*.db`, `*.jsonl`; 0 tracked files | ✅ In force |
| 3 | Pre-commit backstop blocking staged files > 10 MB | ✅ **Closed 2026-09-07** (`dfa60a9`, domchk-d9117f42) — was the gap recorded below; now a tracked installer (`scripts/setup-git-hooks.sh` install/`--check`/`--uninstall`), a rewritten hook source (NUL-delimited paths, 10 MB per-file + 50 MB per-commit caps, hard block on anything staged under `.beads/`), `.githooks/pre-commit`, and a 22-assertion self-test (`scripts/test-setup-git-hooks.sh`). Re-verified live 2026-09-07: `--check` reports the installed hook byte-identical to tracked source; self-test 22/22 |
| 4 | Bound the pack-objects path for bare gc **and** push — `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` (threads pinned: the window limit is per-thread) | ✅ Applied repo-local + global; `./scripts/setup-git-gc-config.sh --verify` resolves the effective bound and passes |
| 5 | Scheduled repo-health checks + bounded gc — six systemd **user timers** (NixOS: no crontab) | ✅ Installed and firing (re-verified 2026-09-06); edit units → `systemctl --user daemon-reload` |
| 6 | Re-dispatch stop-condition for satisfied work — the amplifier | ❌ NEEDLE-fleet-side, outside this repository. Recorded as the systemic finding: it is what converted one kill into 71 |

Detection is cheap and should precede any significant git operation
(`docs/crash-response-guide.md`, Pattern 3 heuristics):

```bash
du -sh .git                      # <1GB healthy · 1–5GB warning · >5GB critical
du -sh .git/objects              # >10GB = HIGH RISK → preemptive cleanup
./scripts/check-repo-health.sh   # full diagnostic pass
```

---

## 9. Follow-up Actions

| # | Action | Owner | Status |
|---|---|---|---|
| 1 | Commit a **pre-commit hook installer** (`scripts/setup-git-hooks.sh` exists untracked; the installed hook has drifted from `scripts/pre-commit-repo-size-hook`) so fresh clones are protected — the one open in-repo gap | domain-check repo maintainers | ✅ **Closed 2026-09-07** — committed as `dfa60a9` (bead domchk-d9117f42); see §8 item 3 |
| 2 | **Re-dispatch stop-condition** for satisfied work (deliverable present, bead still open) plus dispatch-time resource gating and backoff | NEEDLE fleet (outside this repo) | Open — systemic finding |
| 3 | Verify-then-close the split children **bf-31p3g / bf-7d8l5 / bf-6b0fl** against current history — the remotes are already reconciled, so a retry would manufacture duplicate work | Next workflow-debt pass | Open |
| 4 | Close parent alert **bf-5cd2d** via its dedicated closure bead (analysis children do not close their parent alert) | Alert-closure bead owner | Open |
| 5 | Treat the 2026-09-01 bf-1s6c3 doc corpus as **superseded** (§11); cite this report and the chain below instead | Future investigators | Closed by this report |

---

## 10. References

**Guide and canon**
- Crash response guide — `docs/crash-response-guide.md` (Quick Reference exit-code rows 1–2 and note 2; False-Positive Detection rules 1–3; Pattern 3)
- Repository maintenance guide — `docs/maintenance/repository-maintenance-guide.md` (dispatch scope `MemoryMax`, safe-gc safeguards, bf-198ne push-side mechanism)
- Monitoring design canon — `docs/crash-prevention-requirements.md` and the two design docs it maps

**This investigation chain (in order)**
- Raw artifact bundle — `docs/crashes/bf-1s6c3/` (collection bead domchk-fcac734a)
- Primary-source timeline — `docs/crashes/bf-1s6c3-crash-storm-timeline-2026-09-06.md` (domchk-1fb4ad35)
- Classification — `docs/crashes/bf-1s6c3-crash-classification-2026-09-06.md` (domchk-56b5ba67)
- RCA addendum + precedent contrast — `docs/crashes/bf-1s6c3-root-cause-analysis-domchk-1c03aacb-2026-09-06.md` (domchk-1c03aacb)
- Remediation record — `docs/crashes/bf-1s6c3-remediation-2026-09-06.md` (domchk-9822e378)
- Divergence half — `docs/branch-divergence-analysis.md` (0/0 today)
- **This report** — the write-up layer

**Similar past crashes**
- **bf-4yjq** (2026-08-12, same evening, same cause, 50 kills) — `docs/crashes/bf-4yjq-cleanup-verification.md`
- **bf-4x12ec** (2026-08-14) — the `git gc` variant of the same memcg-OOM mechanism, kernel-proven
- **bf-198ne** (2026-08-16) — the `git push` variant, kernel-proven and re-verified resolved 2026-09-06 — `docs/crashes/bf-198ne-crash-report.md`
- **bf-31mno** (2026-08-12) — largest single storm (350 kills), same underlying cause
- **domchk-c9641ac5** (2026-09-01) — the canonical *service-failure* case, used in §5 as the contrast class: `docs/crash-analysis-domchk-c9641ac5-2026-09-01.md`

**Memory entries (workspace crash patterns)**
- `bf-1s6c3-crash-record-corrections` — '9 crashes' is really 76 dispatches/71 kills; the real merge is `42a7b07`, orphaned to `pre-squash-history-20260816`; Aug-12 logs carry explicit `outcome.classified` exit codes
- `agent-crash-bf-4x12ec-investigation` — `exit -1` is a sentinel, not a signal number; Aug-14 alert timestamps ≠ death timestamps
- `fleet-crash-signature-2026-09` — steady-state exit-−1 events come from synthetic scopes; the live dominant signal is synchronized exit-1 service-class waves
- `bead-rs-verification-gotchas` / `needle-autosplit-alert-loop` — verify the target bead's actual state before investigating; near-identical artifact titles are the main false-positive source
- `sibling-owned-artifact-bundles` — leave sibling bundles untracked when their collection bead is still in flight

---

## 11. Corrections to Prior bf-1s6c3 Documentation

Listed so nobody cites them forward. The raw artifacts in `docs/crashes/bf-1s6c3/` are the
authority; the 2026-09-01 corpus predates raw-log extraction and is wrong on all five points.

| Prior claim | Verified reality |
|---|---|
| "9+ OOM crashes over 2.5 hours" (also inherited into CLAUDE.md's evidence note) | **76 dispatches in 4h30m: 71 × exit −1, 4 × exit 124, 1 × exit 0.** Understates by ~8×. bf-1s6c3 alone recorded 49 crashes on Aug-12; the slot 455; the event 460 |
| Crash date `2026-08-12T21:36:51Z` (and `2026-08-13T00:38:41Z` elsewhere) | Attempt **1** and one mid-storm attempt of a continuous retry storm — neither is "the" crash |
| "FALSE POSITIVE — post-completion infrastructure event; task was already done" | Contradicted: all deaths were **mid-task**, 71 of 76 at the push step. The false-positive *component* (72 dispatches against satisfied work) is real, but the classification premise is not |
| Merge commit `2832106`; close reason's `7dd79eb` | **Both are dead SHAs** (`git cat-file` fails on each). The real merge is **`42a7b07`**, which after the 2026-08-16 squash survives only on `pre-squash-history-20260816` and is **not** an ancestor of `main`; the on-`main` reconciliation is **`46293c5`** (2026-08-17) |
| "Task completed successfully after repository cleanup (18 GB → 138 MB)" (bead note, 2026-09-01 fix-implementation report's "SIGHUP cascade / no OOM events") | The cleanup is real; the claim that *this bead's* task completed through it is not — the split children show the merge child InProgress and the push child Open. The SIGHUP mechanism is superseded by memcg OOM; the correct remediation conclusion (nothing to retry) is stated in `docs/crashes/bf-1s6c3-remediation-2026-09-06.md` |

---

## 12. Verification Appendix — this bead's own measurements (2026-09-06)

Ran directly, not copied from the chain:

- **Census recounted** from the committed extracts: `agent.completed` × 76 → exit −1 × 71,
  124 × 4, 0 × 1; `outcome.classified` crash/timeout/success = 71/4/1; `outcome.handled`
  alerted × 71; first claim 21:31:27.663Z; first crash 21:36:44.519Z (316,572 ms); final
  attempt exit 0 at 02:01:22.561Z (384,204 ms)
- **Attempt 4** read from the extract: completed 21:48:06.650826225Z, exit −1, duration
  285,151 ms → **59.6 s** after merge `42a7b07` (git: 2026-08-12T17:47:07-04:00 = 21:47:07Z,
  parents `47e7758` + `00117cb`)
- **Ancestry:** `42a7b07` not an ancestor of `main` (contained by `pre-squash-history-20260816` only);
  `46293c5` is an ancestor of `main`
- **Dead SHAs:** `2832106` and `7dd79eb` both fail `git cat-file`; `7dd79eb` present in the
  bead's own close record in `.beads/checkpoint/forensic.jsonl`
- **Repository:** `.git` 101 MB; 5 loose objects / 36 KiB; 3 packs 99.13 MiB; garbage 0;
  `git ls-files .beads` → 0; `.gitignore:66` `.beads/`
- **Convergence:** `git rev-list --left-right --count HEAD...origin/main` → 0 / 0;
  GitHub mirror `ls-remote refs/heads/main` → `c8dc3cf` = local HEAD

### Re-verification 2026-09-07 (domchk-dedc99c7 — the chain's final-documentation bead)

Every load-bearing claim above re-run first-hand, all byte-exact against the committed
extracts and live `git`:

- **Census recounted:** `agent.completed` × 76 → exit −1 × 71, 124 × 4, 0 × 1;
  `outcome.classified` crash/timeout/success = 71/4/1; `outcome.handled` alerted × 71
  (deferred × 4, none × 1); first claim 21:31:27.663Z; first crash 21:36:44.519Z; final
  attempt exit 0 at 02:01:22.561Z (extracts total 945 + 513 lines)
- **Ancestry:** `42a7b07` = commit, 2026-08-12T21:47:07Z, "Merge reconciliation: Forgejo and
  GitHub remote histories" — **not** an ancestor of `main`; `46293c5` **is**
- **Repository:** `.git` 102 MB · 62 loose objects / 408 KiB · 3 packs · garbage 0 ·
  `git ls-files .beads` → 0 · `.gitignore:66` `.beads/`, `:70` `*.jsonl`
- **Convergence:** `HEAD...origin/main` → 0 / 0; GitHub mirror `main` = local HEAD
  (`e288a6371657`)
- **Prevention layer 4:** `./scripts/setup-git-gc-config.sh --verify` exit 0 — effective
  windowMemory=2g / threads=1 / deltaCache=1g, worst case ≈3072 MiB within ceiling
- **Prevention layer 3 (correction above):** `scripts/setup-git-hooks.sh` + rewritten
  `scripts/pre-commit-repo-size-hook` + `.githooks/pre-commit` all tracked on `origin/main`
  (`dfa60a9`); `--check` → hook byte-identical to source; `scripts/test-setup-git-hooks.sh`
  → 22/22. §8 item 3 and §9 action 1 updated accordingly

**Dispatch note:** this bead (domchk-dedc99c7, "Task 4: Final Documentation" of the
bf-1s6c3 auto-split chain) found its deliverable already rendered by `af9b461` +
`fcfa79d` (domchk-ed3ed12b, closed) and therefore shipped no new summary document —
the only change here is the dated correction above.

### Re-verification 2026-09-07 (domchk-0d35f6e5 — the parallel chain's root-cause step)

This bead is the "Investigate root cause based on classification" link of the **parallel**
gather → classify → investigate chain (`domchk-2f8e3f15` → `domchk-0d43def4` →
`domchk-0d35f6e5`); its classification predecessor handed it this report's subject with
"Infrastructure — repository bloat, confidence high". The deliverable it was dispatched to
produce — root cause with supporting evidence, contributing factors, mitigation strategy —
is rendered by **§6, §5 and §8 above**, written by the sibling chain. Per the workspace
dedup rule this bead therefore shipped **no new document**; what follows is its own
first-hand re-verification, plus the explicit mapping of its dispatch's four
infrastructure-branch investigation steps onto this report.

**Live re-verification (all re-run 2026-09-07, byte-exact against this report):**

- **Census recounted** from the committed extracts (945 + 513 lines): `agent.completed` × 76
  → exit −1 × 71, 124 × 4, 0 × 1; `outcome.classified` → crash × 71, timeout × 4,
  success × 1; `outcome.handled` → alerted × 71, deferred × 4, none × 1; first claim
  21:31:27.663Z; last completion 02:01:22.561Z
- **Ancestry:** `42a7b07` = commit, 2026-08-12T21:47:07Z "Merge reconciliation: Forgejo and
  GitHub remote histories" — **not** an ancestor of `main`; `46293c5` **is**; `2832106` and
  `7dd79eb` both fail `git cat-file`
- **Repository:** `.git` 102 MB · 71 loose objects / 484 KiB · 3 packs · 99.13 MiB ·
  garbage 0 · `git fsck --full` exit 0 · `git ls-files .beads` → 0 · `.gitignore:66`
  `.beads/` (+ `*.db` / `*.jsonl` rules)
- **Convergence:** `rev-list --left-right --count HEAD...origin/main` → 0 / 0; Forgejo
  `origin/main` == local `HEAD` == `21b5a85`
- **Prevention layers 3–4:** `scripts/setup-git-gc-config.sh --verify` exit 0 (effective
  windowMemory=2g / threads=1 / deltaCache=1g, worst case ≈3072 MiB within the 12 GiB
  dispatch-scope ceiling); `scripts/setup-git-hooks.sh --check` exit 0 (installed hook
  byte-identical to tracked source)
- **Host today:** 45 G memory available, 57 G disk free, load 5.74 — healthy

**The dispatch's four infrastructure-branch steps, mapped:**

1. *Memory pressure and system resources* — the violated axis at crash time was the
   **repository-size** axis (≈18 GB object store), not the host axis; host memory/load for
   2026-08-12 remain unknowable (§4.3). The pre-task `free -g` gate would not have caught
   this event (§5's threshold review, quantified in the RCA addendum) — which is why the
   operative pre-flight for this crash type is the repo-size table (§8).
2. *OOM killer logs and system events* — re-confirmed live that none can exist for Aug-12:
   `journalctl --list-boots` first entry **2026-08-15 19:46:33 EDT**. The memcg mechanism is
   kernel-proven for the later siblings bf-4x12ec (gc) and bf-198ne (push); for this event it
   rests on the fully-present Pattern-3 signature (§5.2).
3. *SIGHUP cascade* — **excluded by the data**: all 71 deaths carry `exit_code: −1` with zero
   variation (no 129/137 signal-number encoding anywhere in the extracts), and −1 is needle's
   `wait()` sentinel for died-by-signal (§5.1). The superseded SIGHUP-cascade framing of the
   2026-09-01 corpus is catalogued in §11.
4. *Resource limits and quotas* — the binding constraint was the dispatch scope's
   `MemoryMax=12GiB` (`docs/maintenance/repository-maintenance-guide.md:158,171`) against an
   18 GB object store; the post-remediation bound (windowMemory=2g, threads pinned,
   deltaCache=1g → ≈3 GiB worst case per pack run) verifies clean today.

**Root-cause verdict (unchanged, confirmed):** Infrastructure — repository bloat
(Pattern 3). Immediate cause: memcg-OOM SIGKILL of `git push`'s pack-objects inside the
12 GiB dispatch scope (71 of 76 attempts died at the push step, §4.2). Amplifying cause: the
~10 s no-backoff re-dispatch loop with no stop-condition for satisfied work. Underlying
cause: 17+ identical ~237 MB `.beads/*.jsonl` snapshots committed to git. Mitigation: §8
layers 1–6, re-verified in force above. **Alert disposition: no further action for this
event** — the bead is closed, the deliverable is represented on `main` by `46293c5`, and the
repository condition is repaired and holding.

### Re-verification 2026-09-07 (domchk-6be24808 — the parallel chain's classification step)

This bead is the "Classify crash and collect evidence" link of a further parallel instance of
the gather → classify → investigate chain. Its dispatched deliverable — crash classification,
collected evidence, repository health metrics, work-completion check, classification report —
is rendered by **§5, §4, §4.4/§12 and §2/§5.3** above, and stands alone as
`docs/crashes/bf-1s6c3-crash-classification-2026-09-06.md` (domchk-56b5ba67). Per the
workspace dedup rule this bead shipped **no new classification document**; what follows is its
own first-hand re-verification plus the resolution of its dispatch's named crash instant.

**Live re-verification (all re-run 2026-09-07, byte-exact against this report):**

- **Census recounted** from the committed extracts (945 + 513 lines): `agent.completed` × 76
  → exit −1 × 71, 124 × 4, 0 × 1; `outcome.classified` → crash × 71, timeout × 4, success × 1;
  `outcome.handled` → alerted × 71, deferred × 4, none × 1
- **Ancestry:** `42a7b07` = commit, 2026-08-12T21:47:07Z "Merge reconciliation: Forgejo and
  GitHub remote histories", parents `47e7758` + `00117cb` — **not** an ancestor of `main`
  (contained only by `pre-squash-history-20260816`); `46293c5` **is**; `2832106` and
  `7dd79eb` both fail `git cat-file`
- **Repository:** `.git` 102 MB · 82 loose objects · in-pack 11,182 / 99.13 MiB · garbage 0 ·
  `git fsck --full` exit 0 (dangling trees only) · `git ls-files .beads` → 0 · `.gitignore:66`
  `.beads/`, `:70` `*.jsonl`
- **Convergence:** `rev-list --left-right --count HEAD...origin/main` → 0 / 0; GitHub mirror
  `main` = `5d29b47` = local HEAD
- **Health scripts (dispatch steps 1–2):** `preflight-health-check.sh` 4/4 passed;
  `check-repo-health.sh` passed — 101 MB, 3 packs, effective pack-memory bound ≈3072 MiB
  verified within the 12 GiB dispatch-scope ceiling, no unmanaged aggressive gc running
- **Host today:** 43 GiB memory available, 53 G disk free, load 6.59 — healthy

**The dispatch's named crash instant resolved:** `2026-08-12T22:25:51` is not a distinct
crash. It is the post-kill handling boundary of one of the 71 identical deaths —
`agent.completed` **22:25:44.342Z** (`exit_code: −1`, duration 94,581 ms, mid-task),
`HANDLING_RELEASE_DONE` heartbeat 22:25:51.760Z, `outcome.handled action=alerted` 22:25:53.889Z,
re-claim 22:25:56.251Z (2.4 s). Like the `21:36:51Z` and `00:38:41Z` variants catalogued in
§11, it is one mid-storm attempt boundary, not "the" crash.

**Classification verdict (unchanged, confirmed):** Infrastructure — repository bloat
(`docs/crash-response-guide.md` Pattern 3, exit-code table row 2; every Pattern-3 criterion
present per §5.2). Work completion: the deliverable landed at 21:47:07Z (attempt 4) while
deaths continued **mid-task** — 72 of 76 dispatches ran against already-satisfied work; the
on-`main` reconciliation is `46293c5` with 0/0 divergence today. Excluded alternates per
§5.4: workflow (no exit-1/max-turns signature), service failure (no 5xx to the gateway), code
defect (the task never touched application code). **Alert disposition: no further action for
this event.**

### Prevention-documentation update 2026-09-07 (domchk-64e1461a — the guide-update step)

This bead carried this report's findings into `docs/crash-response-guide.md`: exit 124 added to
the Quick Reference classification table; the INFRASTRUCTURE classification row and the
"What Causes Crashes" line moved off the superseded SIGHUP framing (§11/§12 item 3); Rule 1 and
Rule 2 false-positive caveats (deliverable-landed-mid-storm → verify-then-close debt;
exit-0-after-a-storm is a surface match when the task shape changed rather than the
environment); the re-dispatch-amplifier corollary under Rule 3 (~10 s cycle, alerts scale 1:1
with kills); the push-side bf-1s6c3 evidence block (71 of 76 deaths at `git push`) added to
Pattern 3 alongside bf-4yjq; the surge example aligned to the committed 3-in-5-minutes detector
threshold; the host-memory-gate limitation note (the `free -g` pre-task gate cannot see the
repository-size axis); and the pre-commit-hook prevention bullet pointed at the now-committed
installer (`dfa60a9`). Related-Documentation pointers now cite this report and mark the
2026-09-01 corpus superseded.

Live re-verification run by this bead before citing (all 2026-09-07, first-hand):

- Repository: `.git` 102 MB · 78 loose objects / 544 KiB · garbage 0 · `git fsck --full` clean ·
  `git ls-files .beads` → 0
- Ancestry re-checked: `42a7b07` = commit (2026-08-12T21:47:07Z, "Merge reconciliation: Forgejo
  and GitHub remote histories") and **not** an ancestor of `main`; `46293c5` **is**
- `./scripts/setup-git-gc-config.sh --verify` exit 0 — effective windowMemory=2g / threads=1 /
  deltaCache=1g, worst case ≈3072 MiB within the 12 GiB dispatch-scope ceiling
- `./scripts/setup-git-hooks.sh --check` exit 0 — installed hook byte-identical to tracked source
- `scripts/crash-pattern-detection.sh` `CRASH_SURGE_THRESHOLD=3` over a 5-minute window —
  confirmed in the committed script before aligning the guide's surge example to it

### Re-verification 2026-09-07 (domchk-42769e52 — a context-and-failure-mode bead of the bf-3laof alert pool)

This bead ("Analyze crash bf-1s6c3 context and failure mode", child of alert **bf-3laof**)
was dispatched with the alert's own timestamp — 2026-08-12T22:08:37.503833103Z — as "the
crash". The deliverable its four acceptance criteria ask for (crash context, failure mode,
timeline, artifact catalog) is rendered by **§1–§2, §5–§6, §3 and §4** above, so per the
workspace dedup rule this bead shipped no new document. Its additions are the live
re-verification below plus one fact the report did not previously carry: **which of the 71
deaths produced this particular alert.**

**New: alert bf-3laof ↔ its death, mapped from the committed extracts.** The alert's
timestamp is *not* the death instant — it is a crash-handler heartbeat (seq 4972,
22:08:37.503644680Z, 0.19 ms before the alert's recorded time; bf-3laof was created
22:08:37.515Z). The death it reports:

| Field | Value | Source |
|---|---|---|
| Attempt | **12 of 76** (dispatch ordinal 12; session start 22:06:28.504, UUID `be156451`, 249,210 transcript bytes) | extract + `sessions-index.tsv` |
| Death | seq 4963 `agent.completed` **2026-08-12T22:08:29.040179609Z**, `exit_code: -1`, duration 120,931 ms (~121 s) | extract |
| Alert latency | **8.5 s** death → alert timestamp (inside the known 8–120 s handler-heartbeat range) | extract |
| Last-issued command | `git push origin main` — the push-step kill signature shared by 60 of the 71 deaths (§4.2) | `sessions-index.tsv` |
| Handling | `outcome.classified` = `crash` (seq 4966) → `bead.released` `release_success` (22:08:41.001) → `outcome.handled` = `alerted` (seq 4975) — that alert **is** bf-3laof | extract |
| Aftermath | session 13 dispatched 22:08:45.538, ~16 s after the death — the no-backoff re-dispatch cycle of §6 | `sessions-index.tsv` |

So the failure mode behind *this specific alert* is the same as the storm's: attempt 12 died
by signal 121 s into its run, at its push, against the ≈18 GB repository. The alert is one of
the 71 `alerted` outcomes for one undrainable cause (§6); the 2026-08-12T22:08:37Z timestamp
joins 21:36:51Z (§3) as a *handling* instant, not a distinct crash.

**Live re-verification (re-run 2026-09-07, all byte-exact against this report):**

- **Census recounted** from the committed extracts: `agent.completed` × 76 → exit −1 × 71,
  124 × 4, 0 × 1; `outcome.classified` crash/timeout/success = 71/4/1; `outcome.handled`
  alerted/deferred/none = 71/4/1; first claim 21:31:27.663Z; last completion 02:01:22.561Z
- **Ancestry:** `42a7b07` = commit, 2026-08-12T21:47:07Z "Merge reconciliation: Forgejo and
  GitHub remote histories" — **not** an ancestor of `main`; `46293c5` **is**; `2832106` and
  `7dd79eb` both fail `git cat-file`
- **Repository:** `.git` 102 MB · 82 loose objects / 560 KiB · packs 99.13 MiB · garbage 0 ·
  `git fsck --full` clean · `git ls-files .beads` → 0
- **Convergence:** `rev-list --left-right --count HEAD...origin/main` → 0 / 0; Forgejo
  `origin/main` = local `HEAD` = `5d29b47` (the report's `21b5a85` was true at the 09-07
  re-verifications above; only the tip has advanced since, divergence still zero)
- **Prevention layers 3–4:** `scripts/setup-git-gc-config.sh --verify` exit 0;
  `scripts/setup-git-hooks.sh --check` exit 0 (installed hook byte-identical to source)
- **Host today:** 45 G memory available, 53 G disk free, load ~6 — healthy

**Dispositions:** context/failure-mode/timeline/artifacts → §1–§6 (verified above). Alert
bf-3laof → no further action; closure belongs to its dedicated closure bead, not this
analysis child.

### Re-verification 2026-09-07 (domchk-877f7ea9 — the documentation-consolidation step)

Dispatched to verify a five-document bf-1s6c3 corpus. Every path checked first-hand:

- **Three of the five named paths exist** as stated: `docs/crashes/bf-1s6c3-investigation.md`,
  `docs/crashes/repository-bloat-crash-bf-1s6c3-2026-08-12.md`,
  `docs/crashes/exit-code-minus-one-root-cause-analysis-final.md`.
- **A fourth exists only archived:** `docs/crash-root-cause-analysis-bf-1s6c3-final.md` was
  moved by `git mv` into `docs/archive/crash-investigations/` (commit `a883044`, on
  `origin/main`); the archived blob is byte-identical to the pre-move one (`f3544fb6`), and
  `docs/archive/crash-investigations/README.md` marks the directory as historical.
- **The fifth filename never existed:** `docs/crash-fix-verification-report-bf-1s6c3-2026-09-01.md`
  has zero git history. It conflates two real documents —
  `docs/crash-fix-implementation-report-bf-1s6c3-2026-09-01.md` (live, top level) and
  `docs/archive/crash-investigations/crash-resolution-verification-bf-1s6c3-2026-09-01.md`
  (archived). No document was fabricated to fill the name.
- **The dispatch's own acceptance figures are §5.1 supersessions:** "18 GB → 138 MB" (verified
  93–94 MB on 2026-09-01) and the "17+ days stable" point-in-time claim.
- **Consolidation shipped by this bead:** dated supersession banners added to the three live
  docs that still lacked one (`repository-bloat-crash-bf-1s6c3-2026-08-12.md`,
  `exit-code-minus-one-root-cause-analysis-final.md`,
  `crash-fix-implementation-report-bf-1s6c3-2026-09-01.md`), matching the banner already on
  `bf-1s6c3-investigation.md` — each now points at §5.1 (and, for the implementation report's
  stale "DO NOT RETRY YET" guidance, at the remediation record's supersession note).
- **CLAUDE.md crash-prevention section: present and accurate**, re-verified live 2026-09-07:
  `.git` 102 MB · 82 loose objects / 560 KiB · 3 packs 99.13 MiB · garbage 0 ·
  `git fsck --full` exit 0 · `git ls-files .beads` → 0 · `HEAD...origin/main` → 0/0 ·
  `./scripts/setup-git-gc-config.sh --verify` exit 0 (worst case ≈3072 MiB within ceiling) ·
  `./scripts/check-repo-health.sh` exit 0.

### Re-verification 2026-09-07 (domchk-d24458e6 — the bf-2oq9d documentation layer)

This bead is the open documentation layer of the bf-2oq9d alert chain (attempt 16's
alert): downstream consumer of
`docs/crashes/bf-1s6c3-attempt16-crash-artifacts-2026-09-06.md` (`e288a63`), blocked by
its gatherer domchk-873f89f1 (closed), itself blocking bf-2oq9d. Its dispatch spec —
"crash investigation report in `docs/crash-analysis/`, findings cross-referenced with
existing crash docs, remediation steps documented, related procedures updated" — is
already rendered by committed sibling work, each criterion checked in `HEAD` first-hand:

- **Report in `docs/crash-analysis/`:** `bf-1s6c3-crash-analysis-2026-08-12.md` plus the
  directory catalog `README.md` (`9890c8a`, domchk-671f228f, closed).
- **Cross-referencing:** the catalog's links into the `docs/crashes/` classifications and
  both crash indexes (`fcfa79d`, domchk-ed3ed12b, closed); the attempt-16 artifact doc
  names this bead as its consumer and its §7 carries the classification hints for it.
- **Remediation for future reference:** `docs/maintenance/repository-maintenance-guide.md`
  and `docs/verification/bf-1s6c3-recommendations-verification-2026-09-07.md` (`635bb21`,
  domchk-a18b2c06), plus the pack-memory-bound rollback path
  (`499d44d`, domchk-9fe7fba1 — this bead's blocker, closed).
- **Related procedures:** `docs/crash-response-guide.md` third pass from this report
  (`ef842bf`, domchk-64e1461a — exit-124 branch, push-side Pattern-3 evidence block,
  re-dispatch-amplifier corollary).

Per the attempt-16 doc's §7 hints, all adopted: **INFRASTRUCTURE EVENT**, exit −1,
attempt 16 of 76, mechanism = push-side pack-objects memcg-OOM on the bloated repo four
days before the `pack.windowMemory` bounds existed; window = kill 22:17:51.025831684Z
(extract line 296, seq 5076, 154632 ms), the dispatch's named 22:17:57.286Z being
kill + 6.26 s; not a code defect, not a service failure, not max-turns exhaustion; task
already satisfied when the attempt died (`42a7b07` landed 21:47:07Z) but the death was
real and mid-push. bf-2oq9d's Notes (`7dd79eb`, "666 ahead", `63ba024`) treated as
unreliable per hint 5.

Live re-verification (this bead's own runs, 2026-09-07):

- **Census recounted from both committed extracts** (945 + 513 lines): `agent.completed`
  × 76 → exit −1 × 71, 124 × 4, 0 × 1; classified crash/timeout/success = 71/4/1; first
  crash 21:36:44.519246181Z, final exit 0 at 2026-08-13T02:01:22.561923995Z.
- **Ancestry:** `42a7b07` ("Merge reconciliation: Forgejo and GitHub remote histories",
  2026-08-12T21:47:07Z) is **not** an ancestor of `main`; `46293c5` **is**.
- **Repository:** `.git` 102 MB · 120 loose objects / 856 KiB · 3 packs · garbage 0 ·
  `git fsck --full` exit 0 (dangling trees only) · `git ls-files .beads` → 0 ·
  `.gitignore:66` `.beads/`, `:70` `*.jsonl`.
- **Convergence:** `HEAD...origin/main` → 0 / 0 (`93da1c1`).
- **Prevention:** `./scripts/setup-git-gc-config.sh --verify` exit 0 — effective
  windowMemory=2g / deltaCacheSize=1g / threads=1 (local scope), worst case ≈3072 MiB
  within the 12 GiB dispatch-scope ceiling.

**Dispatch note:** no new summary document shipped — the four acceptance criteria are
satisfied by the committed files above, and the workspace already holds ~460 crash docs;
another near-duplicate would add noise, not record. This dated subsection is the bead's
only change.

### Re-verification 2026-09-07 (domchk-c3955b52 — a further parallel chain's root-cause step)

This bead is the "Analyze root cause of bf-1s6c3 crash" link of another parallel
gather → classify → investigate instance (`domchk-6be24808` → `domchk-c3955b52`); its
classification predecessor closed with "Infrastructure — repository bloat". The deliverable
its acceptance criteria ask for — proximate cause, contributing factors, evidence citations,
one-time-vs-systemic determination, code-defect check — is rendered by **§6 (root cause), §5
(classification it builds on) and §8 (mitigation)** above. Per the workspace dedup rule this
bead shipped **no new document**; what follows is its own first-hand re-verification plus the
three items this report previously carried only in fragments.

**Live re-verification (all re-run 2026-09-07, byte-exact against this report):**

- **Census recounted** from the committed extracts (945 + 513 lines): `agent.completed` × 76
  → exit −1 × 71, 124 × 4, 0 × 1; `outcome.classified` (`data.outcome`) → crash × 71,
  timeout × 4, success × 1; `outcome.handled` → alerted × 71, deferred × 4, none × 1;
  `transform.completed` × 76; first claim 21:31:27.663203161Z; last completion
  02:01:22.561923995Z
- **Push-step concentration recounted** from `sessions-index.tsv` (76 rows): 60 ×
  `git push origin main`, 10 × `git push github main` — the §4.2 table confirmed at source
- **Ancestry:** `42a7b07` = commit, 2026-08-12T21:47:07Z "Merge reconciliation: Forgejo and
  GitHub remote histories", parents `47e7758` + `00117cb` — **not** an ancestor of `main`
  (contained only by `pre-squash-history-20260816`); `46293c5` **is**; `2832106` and
  `7dd79eb` both fail `git cat-file`
- **Repository:** `.git` 102 MB · 120 loose objects / 856 KiB · in-pack 11,182 / 99.13 MiB ·
  garbage 0 · `git fsck --full` exit 0 (dangling trees only) · `git ls-files .beads` → 0 ·
  `.gitignore:66` `.beads/`, `:68` `*.db`, `:70` `*.jsonl`
- **Convergence:** `rev-list --left-right --count HEAD...origin/main` → 0 / 0
- **Prevention layers 3–4:** `scripts/setup-git-gc-config.sh --verify` exit 0 (effective
  windowMemory=2g / threads=1 / deltaCache=1g, worst case ≈3072 MiB within the 12 GiB
  dispatch-scope ceiling); `scripts/setup-git-hooks.sh --check` exit 0 (installed hook
  byte-identical to tracked source)
- **Host today:** 45 GiB memory available, 53 GB disk free, load 7.22 — healthy. Host
  memory/load *at crash time* remain unknowable (§4.3); the violated axis was
  repository size (§6), so the dispatched resource-pressure step resolves to the repo-size
  axis, not the host axis
- **Journald boundary re-checked:** first boot entry **2026-08-15 19:56:33 EDT** — no kernel
  record for 2026-08-12 can exist, as §4.3 states

**New: the checkpoint-pattern leg is now closed at source.** §4.3 records that the offending
blobs "were packed away" and the bloat is canon-sourced. This bead verified that directly:
`git log --all --oneline -- .beads/` → **0 commits across every ref** — the 17+ identical
~237 MB `.beads/*.jsonl` snapshot commits are unreachable from *any* surviving ref, and the
largest blob in the entire current object store is 14,970,288 bytes (§4.3's figure,
re-confirmed). The offending objects are therefore confirmed gone, not merely unreferenced:
the bloat figures remain canon-sourced (`docs/crashes/bf-4yjq-cleanup-verification.md`) and
cannot be re-measured.

**New: the code-defect exclusion strengthened from tree identity.** The task's own statement
(`bead show bf-1s6c3`) is "Create merge commit reconciling Forgejo and GitHub histories" —
all five acceptance criteria are merge mechanics; none names application behaviour. The two
deliverable-level merges are **tree-preserving**:

| Commit | tree vs parent 1 | tree vs parent 2 | Reading |
|---|---|---|---|
| `42a7b07` (deliverable) | **identical** | differs | Reconciliation kept side 1's entire content and grafted side 2's *ancestry* only — zero content delta |
| `46293c5` (on-`main`) | **identical** | **identical** | Pure history join — zero content delta |

So the storm's attempts authored, modified and executed **no application code**: the
102-file parent-vs-parent divergence (which does include `internal/server/handlers_api.go`)
was resolved by adopting side 1 wholesale, not by editing source. This is stronger than
§5.4's "the task never touched domain-check code" — the deliverable *could not* have
introduced a code defect, because its trees introduce no change.

**New: the one-time-vs-systemic determination.** Not a one-time event — a three-layer
systemic pattern, each layer with its own status:

1. **Underlying cause — bead state committed to git** (§6): systemic across the fleet on
   2026-08-12 (same evening: bf-31mno 350 kills, bf-4yjq 50, bf-2xygo 4; 460 `exit_code=-1`
   event-wide). **Repaired and structurally prevented** — `.gitignore` + 0 tracked `.beads`
   files + the committed pre-commit hook (layer 3, `dfa60a9`), all re-verified above.
2. **Kill mechanism — unbounded pack-objects vs. the dispatch scope** (§6 immediate): systemic
   on 2026-08-12 and again Aug-14/16 (bf-173o7e gc-side, bf-198ne push-side — kernel-proven).
   **Bounded** since the `pack.windowMemory`/`threads`/`deltaCacheSize` config (layer 4,
   verified above at ≈3 GiB worst case); bf-198ne re-verified resolved.
3. **Amplifier — ~10 s no-backoff re-dispatch with no stop-condition for satisfied work** (§6
   amplifying): systemic, NEEDLE-fleet-side, **still open** (§8 item 6 / §9 action 2). This is
   the layer that converted one kill into 71 deaths and 71 alerts; until it ships, a
   satisfied-work storm remains reachable from any future persistent kill cause.

**Root-cause verdict (unchanged, confirmed):** Infrastructure — repository bloat (Pattern 3).
Proximate: memcg-OOM SIGKILL of `git push`'s pack-objects inside the 12 GiB dispatch scope
against an ≈18 GB object store (71 of 76 attempts died at the push step). Contributing:
repo-side — bead-state snapshots in git + no pre-commit backstop at the time; fleet-side —
the re-dispatch amplifier. **Alert disposition: no further action for this event** — the bead
is closed, the deliverable is represented on `main` by `46293c5`, and the repository
condition is repaired and verified holding. Closure of parent alert bf-5cd2d belongs to its
dedicated closure bead (§9 action 4).

### Re-verification 2026-09-07 (bf-1atrl — the alert bead for attempt 17's kill)

bf-1atrl is one of the 71 `outcome.handled → alerted` alert beads: its description names
`2026-08-12T22:20:07.308826347+00:00` against `exit code -1`. This bead mapped that instant
first-hand from the committed extracts — it is **attempt 17 of 76**, the retry dispatched
immediately after attempt 16 (bf-2oq9d's subject, subsection above):

| Event | Timestamp (Z) | Source |
|---|---|---|
| Attempt 16 released (`release_success`) | 22:17:59.717015390 | seq 5086 (attempt-16 window) |
| Attempt 17 claimed + dispatched | 22:18:01.915918516 / 22:18:01.927504266 | seqs 5092, 5101 |
| **Real kill — `agent.completed`, `exit_code=-1`, `duration_ms=119768`** | **22:20:01.902289324** | seq 5104 |
| `outcome.classified` → `crash` | 22:20:01.903069901 | seq 5107 |
| heartbeat `HANDLING_RELEASE_DONE` | 22:20:07.308817515 | seq 5113 |
| **Named timestamp (bf-1atrl description `Timestamp`)** | **22:20:07.308826347** | kill + **5.406537023 s** |
| Alert bead `created_at` | 22:20:07.315370608 | named ts + 6.5 ms |

So bf-1atrl is **not a distinct crash**: it is the alert generated by attempt 17's
memcg-OOM death, ~120 s into the attempt (119,768 ms — not a 600 s timeout; the
"pure timeout issue" attribution in `docs/crash-investigations/bf-1atrl-crash-investigation.md`
was already superseded by domchk-a18b2c06's banner, commit `635bb21`). Attempt 17's
*dispatch* was already recorded in the attempt-16 window above; its *outcome* was not, and
this subsection supplies it.

**The alert's deliverable was already shipped (2026-08-26, on `origin/main`):**
`docs/crash-investigations/bf-1atrl-crash-investigation.md` (`9fa2060`, plus same-day
duplicate `6b39d0a` and companion verification report `918fdac`) — and the target bead
itself is Closed (2026-09-02T03:26:02Z, rev 4). Alert disposition: no further action.

Live re-verification (this bead's own runs, 2026-09-07):

- **Census recounted from both committed extracts** (945 + 513 lines): `agent.completed`
  × 76 → exit −1 × 71, 124 × 4, 0 × 1; classified crash/timeout/success = 71/4/1; the
  kill above is the 17th of those 76.
- **Ancestry:** `42a7b07` = commit, **not** an ancestor of `main`; `46293c5` **is**.
- **Repository:** `.git` 102 MB · 130 loose objects / 948 KiB · garbage 0 ·
  `git ls-files .beads` → 0.
- **Convergence:** `HEAD...origin/main` → 0 / 0.
- **Prior deliverable:** `git merge-base --is-ancestor` → `9fa2060`, `6b39d0a`, `918fdac`
  all on `origin/main`.

**Dispatch note:** no new summary document shipped — this bead's deliverable (an
investigation of the 22:20:07Z alert) has existed since 2026-08-26 and was corrected
2026-09-07 by `635bb21`; the workspace already holds ~460 crash docs. This dated
subsection is the bead's only change.

### Re-verification 2026-09-07 (domchk-00e228b9 — the gather→classify→fix chain's classification step, alert bf-1wz2w / attempt 48)

This bead is the middle link of a further parallel instance of the chain (child 1
domchk-02f84337 = the [attempt-48 artifact bundle](docs/crashes/bf-1s6c3-attempt48-crash-artifacts-2026-09-07.md),
committed as `93da1c1`; child 3 domchk-e9234a0d = apply the matched fix). Its dispatched
deliverable — a classification section appended to child 1's crash-artifacts document — is
that document's new §8, which applies this report's §5 verdict to attempt 48 specifically.
This subsection records its first-hand re-verification and resolves the **bf-1wz2w** instant,
which until now was the one named storm instant (after 21:36:51Z, 22:20:07Z, 22:25:51Z,
00:38:41Z) with no §12 entry.

**The named instant resolved:** `2026-08-12T23:53:11.034551744Z` (bf-1wz2w's `Timestamp`)
is the `HANDLING_RELEASE_DONE` heartbeat — **kill + 5.6026 s**, the widest of the storm's
alert lags. The real kill is `agent.completed` **23:53:05.431962513Z** (`exit_code = −1`,
`duration_ms = 108759`, extract line 908 seq 5976), `outcome.handled action=alerted`
23:53:13.173Z, retry claim **10.0 s** after the kill (23:53:15.412Z). It is one mid-storm
attempt boundary — attempt 48 of 49 that day — not a distinct crash.

**Live re-verification (all re-run 2026-09-07):**

- **Attempt-48 transcript** (tarball member `f818432d-…jsonl`, 90 rows / 17 tool calls /
  297,530 B, matching `sessions-index.tsv` row 48): 16 read-only git/bead verifications, then
  `git push origin main` at 23:52:52.390Z with **no tool result ever returned** — the kill
  landed 13.0 s into the push. `git branch --contains 7dd79eb` → `* main` at 23:52:40.614Z
  (first-hand proof the deliverable was on local `main` mid-storm); `origin/main..7dd79eb`
  measured **332** (the phantom-divergence family; 0/0 today).
- **Ancestry re-checked:** `42a7b07` = commit, **not** an ancestor of `main`; `46293c5` **is**;
  `412582c`, `63ba024`, `7dd79eb` all fail `git cat-file`.
- **Repository:** `.git` 102 MB · 134 loose objects · in-pack 11,182 · garbage 0 ·
  `git fsck --full` exit 0 (dangling trees only) · `HEAD...origin/main` → 0 / 0.
- **Safeguard layer:** `./scripts/setup-git-gc-config.sh --verify` exit 0 (the bound covering
  exactly this attempt's push path). Subject `bf-1s6c3` closed 2026-08-16T14:00:13.240405326Z,
  actor `system`, re-read from the forensic closed event. Host: 45 G memory available,
  52 G disk free, load 4.76 — healthy.

**Classification verdict (unchanged, confirmed):** Infrastructure — repository bloat
(Pattern 3, exit-code table row 2). Mechanism for this attempt: **push-side pack-objects
memcg-OOM**, the bf-198ne mechanism, four days before `pack.windowMemory` bounds existed.
False-positive rules: Rule 1 not triggered (last commit 2 h 6 m before the kill; death
mid-task), Rule 2 surface match only, Rule 3 not triggered; the deliverable-landed-mid-storm
component is verify-then-close debt on the subject, not a downgrade of the death. Excluded
alternates per §5.4 — including bf-1wz2w's Notes' "600 s timeout" claim, which the 108.9 s
signal death refutes. **Alert disposition: no further action for this event.**

---

**Analysis Status:** ✅ COMPLETE
**Classification:** Infrastructure — repository bloat (Pattern 3); alert disposition: no further action
**No domain-check code defects:** confirmed — the task never touched application code
**Report completed:** 2026-09-06 · domchk-ed3ed12b
