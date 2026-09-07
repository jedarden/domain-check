# bf-4yjq Crash Investigation Chain — Findings and Resolution

**Dispatch bead:** domchk-a6059a67 — "Document crash findings and resolution"
**Chain documented:** bf-2o7nlw (context) → bf-1ziy13 (root cause) → bf-mje3pd (fix + verify)
**Subject of the chain:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo
mirror has diverged/gone stale" (P2, closed 2026-08-17)
**Report date:** 2026-09-07
**Purpose:** the single chain-level record the bead's acceptance criteria call for —
classification, root cause, resolution, prevention — consolidating the companion
per-dispatch docs without repeating their derivations. Load-bearing figures were
re-verified first-hand on 2026-09-07 (see §7).

**Position in the documentation set.** This record does not replace the deeper docs it
consolidates:

| Document | Scope |
|---|---|
| [bf-4yjq-crash-investigation.md](bf-4yjq-crash-investigation.md) | **Canonical bf-4yjq report** — 50-kill Aug-12 storm, corrected counts |
| [bf-4yjq-root-cause-determination-domchk-54bc57df-2026-09-06.md](bf-4yjq-root-cause-determination-domchk-54bc57df-2026-09-06.md) | Root-cause mechanism + contributing factors, corrected framing |
| [crashes/bf-mje3pd-crash-artifacts-analysis-domchk-a4cc1326-2026-09-07.md](../crashes/bf-mje3pd-crash-artifacts-analysis-domchk-a4cc1326-2026-09-07.md) | bf-mje3pd's own 14-dispatch crash loop, first-hand re-extraction |
| [crashes/bf-mje3pd-crash-classification-domchk-bf8c4fd3-2026-09-07.md](../crashes/bf-mje3pd-crash-classification-domchk-bf8c4fd3-2026-09-07.md) | bf-mje3pd classification = INFRASTRUCTURE, heartbeat-instant correction |
| [../verification/bf-mje3pd-crash-analysis.md](../verification/bf-mje3pd-crash-analysis.md) | Fix-verification report (domchk-9bc6579f) |
| [../notes/incident-resolution-bf-1y1d0g-bf-mje3pd-crash-2026-09-02.md](../notes/incident-resolution-bf-1y1d0g-bf-mje3pd-crash-2026-09-02.md) | Alert bf-1y1d0g incident resolution |

---

## 1. Executive Summary

bf-4yjq was a **git remote reconciliation task** (repoint `origin` at Forgejo, reconcile the
Forgejo/GitHub divergence with a merge commit, configure the Forgejo→GitHub server-side push
mirror). On **2026-08-12 the agent dispatched against it was killed 50 times, every one with
exit code -1**, between 17:54:00 and 20:30:43 UTC (~one death every 3.1 minutes), inside a
workspace-wide crash storm. The needle auto-split machinery spun up a three-bead
investigation chain on 2026-08-13:

1. **bf-2o7nlw** — crash context and log gathering. Closed the same hour (18:37Z).
2. **bf-1ziy13** — root-cause determination. Closed 4 minutes later (18:41Z).
3. **bf-mje3pd** — implement and verify the fix. **Itself crashed 7 × exit -1 that same
   evening** while shipping the prevention scripts into the still-bloated repository, then
   completed on its 14th dispatch (21:18:23Z) and closed 2026-08-17T00:15:35Z.

**Outcome: resolved, zero work lost.** The task content (remote reconciliation) was completed
and is correct on the live repo today (§5.1); the crash class (repository bloat → memcg-OOM
during git operations) was eliminated (§5.2); the prevention scripts bf-mje3pd shipped in its
final attempts (`ea23bd1`, `164b62d`) are the ancestors — in content — of the layered defense
this repo still runs (§6). The only loose end is alert hygiene: **bf-1cezsk (ALERT: Agent
crash on bead bf-mje3pd) is still open at rev 23** because nothing in the alert layer retires
an alert when its target bead resolves (§5.3).

## 2. Crash Classification

**INFRASTRUCTURE — resource exhaustion (memcg-OOM regime) during git operations on a bloated
repository. Not a code defect. Not a workflow failure.**

- **Exit code -1** is needle's sentinel for death by signal (infrastructure-level process
  termination), not a signal number and not an application exit —
  [docs/signal-analysis-exit-code-negative-one.md](../signal-analysis-exit-code-negative-one.md);
  classification table in [docs/crash-response-guide.md](../crash-response-guide.md).
- **Mechanism is regime-matched, not kernel-proven.** The kernel/journald records for both
  crash windows were lost to the 2026-08-14/15 reboot (current boot's journal starts
  2026-08-15 19:56:33 EDT). The classification rests on the regime: the 18 GB /
  17.16 GB-loose-object store documented for bf-1s6c3, bf-4yjq, bf-1ea4g and bf-4k2ws on
  Aug-12/13, plus exit -1 bursts at 3–6 minute intervals matching that regime. Earlier
  documents that state "SIGKILL (signal 9) from the OOM killer" or "pack-objects consumed
  3–6 GB" as observed fact are rendering inference; the specific signal is not recoverable.
- **Reproducibility:** deterministic while the trigger existed (50/50 deaths for bf-4yjq,
  7/7 exit -1 within bf-mje3pd's kill burst), and the class is **not reproducible today** —
  the bloated object store no longer exists. It recurs only if the *condition* recurs, which
  the prevention stack (§6) guards.
- **False-positive reading, precisely scoped:** the crashes were real; the *alerts* that are
  still arriving against this chain are stale. bf-mje3pd closed 2026-08-17, so every alert
  raised after that (bf-3za7vh, bf-1cezsk, and the 2026-08-26 wave) targets work that is
  already done — the dominant alert failure mode in this workspace.

## 3. Timeline

All times UTC.

**Phase 1 — the bf-4yjq crash storm (2026-08-12).** 50 dispatches killed with exit -1,
17:54:00 → 20:30:43Z, ~3.1-minute cadence, inside a workspace-wide storm the canonical report
census puts at 455 exit -1 events across 6 beads that day. No chain existed yet; the alert
bead bf-276uk ("ALERT: Agent crash on bead bf-4yjq") accumulated revisions and eventually
auto-split into the chain below.

**Phase 2 — chain dispatched and two legs complete (2026-08-13, evening).**

| Bead | Created | Closed | Deliverable |
|---|---|---|---|
| bf-2o7nlw (context) | 18:24:32 | 18:37:18 (rev 1) | crash context + log evidence for the exit -1 |
| bf-1ziy13 (root cause) | 18:24:47 | 18:41:30 (rev 1) | root cause: OOM-class resource exhaustion during git operations |
| bf-mje3pd (fix + verify) | 18:25:38 | 2026-08-17 00:15:35 (rev 2) | prevention scripts + verification (§5) |

**Phase 3 — bf-mje3pd's own crash loop (2026-08-13 18:53:50 → 21:18:36Z, 2h24m46s).**
First-hand census from the raw fleet log (domchk-a4cc1326, re-verified by domchk-bf8c4fd3):
**14 dispatches / 13 completions — 7 × exit -1, 3 × exit 1, 1 × exit 124 (600 s timeout),
2 × exit 0**, one dispatch (attempt 12) with no completion record, its death bracketed by the
lab-drawrace worker's `peer.crashed` at 20:36:55Z, 49m52s in.

- Kill burst: attempts 1–11 minus two exit-1s, 19:03:11 → 19:46:33Z (**43m22s**, 3–6 min gaps).
- The two fix commits landed seconds before kills:
  `ea23bd1` 19:02:32Z (**39 s** before attempt 1's kill), `164b62d` 19:21:41Z (**14 s**
  before attempt 5's kill).
- First alert bead bf-1y1d0g created 19:03:21Z, ~10 s after the first crash classification.
- Attempt 14 (split template) reached `outcome: success` at 21:18:23Z with
  `verification.passed`; `bead.orphaned` 21:18:36Z. Bead formally closed 2026-08-17T00:15:35Z.

**Phase 4 — aftermath (2026-08-16 → today).** Aug-16 gc consolidated the store (~756 MB);
the verified 18 GB → ~92 MB cleanup landed 2026-09-01 and was re-verified holding
2026-09-06 ([docs/crashes/bf-4yjq-cleanup-verification.md](../crashes/bf-4yjq-cleanup-verification.md)).
bf-4yjq, bf-29h1yy, bf-276uk (rev 18), bf-1y1d0g, bf-3za7vh all closed; stale-alert
investigations on 2026-08-26 and 2026-09-02 re-confirmed the same conclusion each time.
**bf-1cezsk remains open** — the final-verification leg (domchk-40cd5fde, blocked by this
document's bead) is what closes it.

## 4. Root Cause Analysis

**Primary cause — repository bloat.** The object store stood at **~18 GB total, 17.16 GB
loose** (≈4,482 loose objects vs 9.6 MB packed, an ≈1,800:1 inverted loose:packed ratio),
produced by **17+ identical 237 MB `.beads/*.jsonl` snapshots committed to git** (the
`.beads/` directory was tracked at the time; bf-2ildm's state snapshots are the named
contributor in the Aug-17 record). On that store, every substantive git operation pulled a
working set larger than the needle dispatch scope's cgroup memory limit could hold, and the
kernel's memory-cgroup OOM killer ended the process. **The constraint was the dispatch
scope's cgroup limit, not host memory** — the box has 62 GB; the kills happened inside the
worker's own memory ceiling.

**The task content was not causal.** bf-4yjq's remote reconciliation was not what killed its
workers — each short-lived session died on whatever generic git operation it touched first,
on an object store that could not survive git operations. Crash exposure was a function of
*when a bead was scheduled* inside the bloat window, not what the bead was doing. This is why
the chain itself became a victim: bf-mje3pd was *implementing the prevention scripts* on the
still-bloated repo the same evening, and was killed on post-commit work — the earlier
"catch-22: fixing the bloat crashed on the bloat" framing is directionally right but
overstated, because both in-loop fix commits survived and their content is live at HEAD.

**Why the chain's own fix leg crashed 7 times before succeeding.** Same regime, same
mechanism, same 3–6-minute cadence — bf-mje3pd was simply scheduled into the bloat window's
last hours. Needle's retry loop released and re-claimed within 2–18 s each time; the kill
loop ended when the bloat-era store finally yielded to the shipped scripts and the worker
session changed (attempts 13–14 ran under a new session).

**Contributing factor — no alert lifecycle.** The chain resolved the *crash*, but nothing in
the alert layer retired the alerts generated *about* the chain. bf-mje3pd's closure did not
close bf-1y1d0g (took until 2026-09-02), and bf-1cezsk is still open 21 days after its
target closed. This is the residual defect the 2026-09-02 alert-system work (closed-bead
filter, dedup, cooldown) shrinks but does not eliminate.

**Corrections this record carries forward** (older docs are not wrong about the outcome, but
several of their figures are):

| Older claim | Corrected |
|---|---|
| "signal -1 = SIGKILL (signal 9) from OOM killer" (notes/bf-1pidqn.md, docs/verification/bf-mje3pd-crash-analysis.md) | exit -1 is a death-by-signal sentinel; OOM mechanism is regime-matched inference — kernel records lost to the Aug-15 journal floor |
| "exit -1 is SIGHUP" ([docs/crashes/signal-minus1-root-cause-analysis-verified-2026-09-02.md](../crashes/signal-minus1-root-cause-analysis-verified-2026-09-02.md)) | death by signal, unspecified; SIGHUP-cascade framing resolved against by the bf-4yjq RCA (§3.2 there) |
| "9 crashes / 11+ attempts over 2+ hours" (bf-1y1d0g resolution note, verification doc) | 14 dispatches / 13 completions; 7 × exit -1; success on the 14th dispatch |
| "2 hours 15 minutes" crash-to-success (verification doc) | first dispatch → terminal: 2h24m46s; first *crash* → success: 2h15m12s |
| "18 GB → 756 MB" as the resolution (bf-1pidqn.md) | interim state; the verified cleanup to ~92 MB is 2026-09-01 |

## 5. Resolution

### 5.1 The task itself — complete and verified live

bf-4yjq's remote reconciliation is correct on this repo today (re-run first-hand
2026-09-07): `origin` → `https://git.ardenone.com/jedarden/domain-check.git` (Forgejo, the
sole push target), `github-mirror` → `https://github.com/jedarden/domain-check.git`, with the
Forgejo **server-side** push mirror syncing on commit — no client-side dual-push. The mirror
was re-verified end-to-end on 2026-09-07 (commit 5f0de50): all three refs identical, 0/0
divergence, `last_error` empty, two consecutive zero-error syncs.

### 5.2 The crash class — eliminated

18 GB / 17.16 GB loose → Aug-16 consolidation (~756 MB) → **verified 18 GB → ~92 MB cleanup
2026-09-01**, re-verified 2026-09-06, and measured again today: **`.git` = 106 MB, 278 loose
objects / 1.94 MiB, 2 packs / 99.78 MiB, 0 garbage**. No bloat-era crash has recurred since
2026-08-17; live-fleet exit -1 is near-zero in steady state (September census).

### 5.3 The chain — closed except its alert

bf-2o7nlw ✅ closed · bf-1ziy13 ✅ closed · bf-mje3pd ✅ closed (rev 2) · bf-29h1yy (state
assessment sibling) ✅ closed · bf-276uk (parent alert) ✅ closed rev 18 · bf-1y1d0g (first
bf-mje3pd alert) ✅ closed · bf-3za7vh (repeat alert) ✅ closed. **bf-1cezsk (repeat alert,
rev 23) remains open** pending its final-verification leg (domchk-40cd5fde, which this
document's bead unblocks); its correct resolution note is the one every prior verification
converged on: *crashes genuine, work complete and pushed, alerts stale against a closed bead
— no remediation required.*

### 5.4 Work survival — nothing was lost to the kills

`ea23bd1` and `164b62d` are reachable today **only** from the `pre-squash-history-20260816`
branch (the Aug-16 history squash dropped them from the main line), but their content is
live at HEAD: `scripts/check-repo-health.sh`, `scripts/cleanup-repo-bloat.sh`,
`scripts/pre-commit-repo-size-hook` and the `.beads/` gitignore rule all exist in their
maintained forms. Verified first-hand 2026-09-07 (`git branch --contains`, `git ls-files`).

## 6. Prevention Measures

The stack bf-mje3pd started and later beads completed, with each layer's live status at
2026-09-07:

| Layer | Mechanism | Live status |
|---|---|---|
| 1. Source elimination | whole `.beads/` dir + `*.db` + `*.jsonl` gitignored | ✅ `.gitignore:66-70`, 0 tracked `.beads/` files |
| 2. Commit gate | pre-commit repo-size hook, 10 MB per-file block | ✅ installed in this clone (`scripts/pre-commit-repo-size-hook`; per-clone — fresh clones need `scripts/setup-git-hooks.sh install`) |
| 3. Pack-memory bounds | `pack.windowMemory=2g`, `deltaCacheSize=1g`, `threads=1` — bounds bare `git gc` **and** `git push` pack-objects | ✅ repo-local + global; `setup-git-gc-config.sh --verify` checks the effective chain |
| 4. Bounded maintenance | `safe-git-gc.sh` (staged, checkpoint/resumable), `cleanup-repo-bloat.sh`, `check-repo-health.sh` | ✅ all at HEAD; bare `git gc --aggressive` stays prohibited |
| 5. Scheduled monitoring | systemd **user timers** (NixOS — no crontab): crash-pattern 10 min, resource 5 min, service 2 min, repo-health+auto-gc daily 02:00, gc daily 03:00, full gc weekly Sun 04:00 | ✅ installed; verify with `systemctl --user list-timers 'domain-check-*'` |
| 6. Alert hygiene | closed-bead filter, duplicate detection, 5-min cooldown, crash classification (`crash-alert-manager.sh` + `crash-classifier.sh`) | ✅ at HEAD; this chain is the canonical example of what the closed-bead filter prevents |

**Open gap (owned by the alert layer, not by this chain):** alert lifecycle. Nothing
automatically retires an alert when its target bead resolves — the reason bf-1cezsk needed
human chain-closure 21 days after bf-mje3pd closed. Until that lands, crash triage must keep
checking the target bead's live state first (`bead show` + `git log --grep <bead-id>`), per
the crash-response guide.

## 7. Verification record (domchk-a6059a67, this attempt)

Re-executed 2026-09-07 ~20:4xZ against HEAD `63ca904` (== origin/main, 0 unpushed at write
time):

- **Bead states** re-read live from the store: bf-2o7nlw Closed rev 1, bf-1ziy13 Closed rev 1,
  bf-mje3pd Closed rev 2 (closed 2026-08-17T00:15:35Z), bf-4yjq Closed rev 2, bf-29h1yy
  Closed rev 1, bf-276uk Closed rev 18, bf-1y1d0g Closed rev 21, bf-3za7vh Closed rev 12,
  **bf-1cezsk Open rev 23**. Close reasons read from `.beads/checkpoint/forensic.jsonl`.
- **Census figures** for bf-mje3pd's crash loop are cited from
  domchk-a4cc1326's first-hand extraction (committed 9b83c81) cross-checked against
  domchk-bf8c4fd3's independent classification (committed 63ca904) — the two agree on every
  figure used here.
- **Repo health** measured live: `du -sh .git` → 106M; `git count-objects -vH` → 278 loose /
  1.94 MiB, 99.78 MiB in 2 packs, 0 garbage.
- **Remotes** verified live via `git remote -v` (Forgejo origin, GitHub github-mirror).
- **Prevention layers** verified at HEAD: `.gitignore` rules at lines 66–70 with
  `git ls-files .beads` → 0; pre-commit hook present; all four named scripts tracked
  (`git ls-files scripts/`).
- **Fix-commit claims** verified: `ea23bd1` / `164b62d` timestamps and containing branch
  (`pre-squash-history-20260816` only; `git merge-base --is-ancestor` → not on main), and
  their script filenames present at HEAD.
- **Every cited path** checked to exist at HEAD before citation; archived documents are
  cited at their `docs/archive/` locations.
