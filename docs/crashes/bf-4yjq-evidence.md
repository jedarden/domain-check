# bf-4yjq — Crash Evidence File

**Gather leg:** domchk-b06e87d4 ("Extract crash details and context from logs" / "Gather Crash
Evidence") — executed 2026-09-09.
**Subject bead:** `bf-4yjq` — *"Git origin remote points to GitHub directly; Forgejo mirror has
diverged/gone stale"* (P2, task, **Closed** rev 2).
**Workspace:** `/home/coding/domain-check`, worker `claude-code-glm-4.7-lab-domain-check`,
needle session `8446529e`, agent `claude-code-glm-4.7`.

This file is the dispatch's tasked deliverable: one self-contained evidence record. It derives
**no new cause claim** — the classification (INFRASTRUCTURE / repository-bloat regime) and root
cause are already canon; see §9 for the canonical pointers. Every load-bearing figure below was
re-derived first-hand at HEAD `3efa6a3` on 2026-09-09 from the primary sources named in §6; the
two figures this dispatch could *not* re-derive (because their sources are gone) are marked as
such.

---

## 1. Crash metadata

| Field | Value | Source |
|---|---|---|
| Bead | `bf-4yjq` (P2, type `task`, labels `deferred`, `umbrella`; blocks `bf-1h6rk`) | checkpoint issue snapshot, re-read 2026-09-09 |
| First death | **2026-08-12T17:53:53.875Z** (17:53:53 UTC = 13:53:53 EDT) | worker-log slot `.log.2`, live re-parse |
| Last death | **2026-08-12T20:30:38.310Z** | same |
| Storm window | 17:53:53Z → 20:30:38Z = **9,404 s** (2 h 36 m 44 s) | computed from the 50 kill instants |
| Deaths | **50 × `exit_code=-1` `outcome=Crash(-1)`** | live re-count: 50/50 |
| Other outcomes | 1 × `exit_code=1` `Failure` (18:00:17Z); 4 × `exit_code=124` `Timeout` (20:40:47 / 20:51:01 / 21:01:14 / 21:11:27Z); 1 × `exit_code=0` `Success` (21:14:56.748Z) | live re-count |
| Attempts | **56 claims** (`claim_auto`), 17:50:23.048Z → 21:11:40.837Z | live re-count |
| Kill cadence | kill→kill gap: min 75.6 s, **median 155.5 s**, max 576.8 s, **mean 191.9 s** (n=49 gaps) | computed this dispatch; see the supersession note in §8 on the older 149 s figure |
| Alert beads minted | **50** — one per death, created 7.0–12.6 s after each kill (first `bf-276uk`, last `bf-2n3ve`) | live re-parse |
| Agent | `claude-code-glm-4.7` on worker `claude-code-glm-4.7-lab-domain-check`, session `8446529e` | every log line's span fields |
| Workspace | `/home/coding/domain-check` (`needle.workspace=…` in every record) | same |
| Process identity of the killed work | `git push`'s pack-objects over the bloated store — see §4 | regime-matched; §2 for proof status |

**Exit-code semantics (read this before quoting "SIGKILL").** `-1` is needle's
no-wait-status sentinel for a child that died without reporting an exit status — it is not a
signal number. SIGKILL-via-memcg-OOM is the *regime-matched* mechanism: the repo-bloat
precondition (18 GB store) is documented, the death point (§4) is uniform across all 50 runs,
and sibling storms of the same era are kernel-proven the same way — but **the Aug-12 kernel
records themselves are gone**: the system journal's first entry is 2026-08-15T19:46:33 EDT
(single boot `52309698`, re-verified live this dispatch), and `.beads/checkpoint/forensic.jsonl`
retains zero Aug-12 events. So bf-4yjq's mechanism is corroborated per-run, not kernel-proven.
Supersessions of older docs on exactly this point are tabulated in §8.

## 2. What the agent was doing

The bead's task: reconcile this checkout's git remotes with the Forgejo-primary convention —
fetch both remotes, merge the diverged histories (explicitly *without* force-push), repoint
`origin` to `git.ardenone.com/jedarden/domain-check`, set up Forgejo's server-side push mirror
to GitHub, verify convergence.

Each retry re-ran the whole task from the top and then died at the same terminal workflow step:

- **All 50 crashed runs have `git push origin main` as their final recorded Bash tool call.**
  Re-verified first-hand this dispatch on the extracted transcripts of attempt 1
  (`e0be4878…`) and attempt 50: last Bash call = `git push origin main`. Attempt 56 (the
  exit-0 run) ends instead at a `bf show bf-2xygo …` call.
- Attempt 2 — the one `exit_code=1` Failure (18:00:17Z) — ended at
  `git add -A && git commit …`, not at a push.
- 18 of the 50 show a `git commit` earlier in the same session; 32 issued the push with no
  prior commit call. Per-run git activity before death: 7–32 git commands (mean 15.6).

Caveat, stated plainly (as the raw-logs README states it): "last recorded tool call" is not
proof of the process the kernel killed — the agent is not sampled between tool calls. But
50/50 uniformity on the identical command pins the death point to the push step. With the
store carrying ~17 GiB of loose objects, `git push`'s pack-objects is the memory-hungry
process — the same push-side memcg-OOM mechanism later kernel-proven at bf-198ne (2026-08-16).

**No work was lost to the storm:** the task itself was completed and the bead closed five days
later (§5); the crash exposure was scheduling inside the bloat window, not the task content.

## 3. Timeline of bead state changes

| Instant (UTC) | Event | Source |
|---|---|---|
| 2026-07-20T13:59:43.129Z | Bead `bf-4yjq` created (P2, task) | checkpoint issue snapshot |
| 2026-08-12T17:50:23.048Z | First `claim_auto` — storm begins (56 claims through 21:11:40.837Z) | worker log |
| 2026-08-12T17:53:53.875Z | Death 1/50, `Crash(-1)`; alert `bf-276uk` minted 17:54:02.644Z | worker log |
| 2026-08-12T18:00:17Z | Attempt 2 → `exit_code=1` `Failure`, bead released | worker log |
| 2026-08-12T20:30:38.310Z | Death 50/50; alert `bf-2n3ve` minted 20:30:45.743Z | worker log |
| 2026-08-12T20:40:47 / 20:51:01 / 21:01:14 / 21:11:27Z | 4 × `exit_code=124` `Timeout`, each released as **deferred** | worker log |
| 2026-08-12T20:51:14 / 21:01:27 / 21:11:40Z | Needle `auto-split triggered` (SPLIT template) at `failure_count` 3, 4, 5 (threshold 3) — no child beads resulted in the store | worker log |
| 2026-08-12T21:14:56.748Z | Attempt 56 → `exit_code=0` `Success`; `verification.passed`; all validation gates ran | worker log + event log |
| 2026-08-12T21:14:59.207Z | `bead.orphaned` — "agent exited successfully but bead is still open (orphaned) … status=blocked" | worker log |
| 2026-08-17T00:11:34Z | Forgejo push mirror last-sync success recorded | close reason |
| 2026-08-17T00:14:14.580Z | **Bead CLOSED**, rev 2 — close reason: *"Git remote configuration successfully fixed and verified. Origin now points to Forgejo (git.ardenone.com), GitHub mirror is working via server-side push mirror, both repositories are in sync (a245b38), and push mirror last synced successfully at 2026-08-17T00:11:34Z with no errors."* | checkpoint issue snapshot |

### 3.1 The 50 deaths — instants and the alert beads they minted

Kill instants are needle's `outcome.classified` times for `Crash(-1)`; each alert bead was
created 7.0–12.6 s later. Attempt numbers are the crash ordinal, not the claim ordinal
(claims 1–56 interleave the non-crash outcomes listed in §1).

| # | Death instant (UTC) | Alert bead |
|---|---|---|
| 1 | `2026-08-12T17:53:53.875Z` | `bf-276uk` |
| 2 | `2026-08-12T18:03:30.710Z` | `bf-3dq63` |
| 3 | `2026-08-12T18:06:05.303Z` | `bf-59bwz` |
| 4 | `2026-08-12T18:11:59.571Z` | `bf-3ssnm` |
| 5 | `2026-08-12T18:14:43.825Z` | `bf-2fiyo` |
| 6 | `2026-08-12T18:18:13.430Z` | `bf-29rca` |
| 7 | `2026-08-12T18:19:44.011Z` | `bf-uoyie` |
| 8 | `2026-08-12T18:22:09.786Z` | `bf-2weev` |
| 9 | `2026-08-12T18:25:21.962Z` | `bf-2ftau` |
| 10 | `2026-08-12T18:26:56.097Z` | `bf-44x3a` |
| 11 | `2026-08-12T18:28:32.311Z` | `bf-64hxa` |
| 12 | `2026-08-12T18:34:00.295Z` | `bf-3b9rv` |
| 13 | `2026-08-12T18:38:03.981Z` | `bf-1dxk7` |
| 14 | `2026-08-12T18:41:23.839Z` | `bf-hw4i5` |
| 15 | `2026-08-12T18:43:18.976Z` | `bf-1ygk6` |
| 16 | `2026-08-12T18:49:45.806Z` | `bf-2j99a` |
| 17 | `2026-08-12T18:52:01.838Z` | `bf-9b8oe` |
| 18 | `2026-08-12T18:54:12.740Z` | `bf-d7j07` |
| 19 | `2026-08-12T18:56:13.176Z` | `bf-46ttc` |
| 20 | `2026-08-12T18:58:57.531Z` | `bf-2dj1g` |
| 21 | `2026-08-12T19:02:20.135Z` | `bf-bkpuh` |
| 22 | `2026-08-12T19:04:05.473Z` | `bf-x5ynu` |
| 23 | `2026-08-12T19:05:38.406Z` | `bf-4tl4v` |
| 24 | `2026-08-12T19:07:48.522Z` | `bf-1dzwv` |
| 25 | `2026-08-12T19:11:23.767Z` | `bf-aruwg` |
| 26 | `2026-08-12T19:13:28.518Z` | `bf-2o8p2` |
| 27 | `2026-08-12T19:15:55.126Z` | `bf-2t7xh` |
| 28 | `2026-08-12T19:21:05.984Z` | `bf-4wi3v` |
| 29 | `2026-08-12T19:24:52.238Z` | `bf-1fvk2` |
| 30 | `2026-08-12T19:29:19.580Z` | `bf-22514` |
| 31 | `2026-08-12T19:31:12.646Z` | `bf-35bhc` |
| 32 | `2026-08-12T19:35:48.434Z` | `bf-3f6ue` |
| 33 | `2026-08-12T19:40:05.963Z` | `bf-mlv3u` |
| 34 | `2026-08-12T19:42:41.509Z` | `bf-5egrf` |
| 35 | `2026-08-12T19:44:22.893Z` | `bf-bykl0` |
| 36 | `2026-08-12T19:50:04.302Z` | `bf-4tnae` |
| 37 | `2026-08-12T19:53:23.400Z` | `bf-3k3ya` |
| 38 | `2026-08-12T19:54:38.951Z` | `bf-5966o` |
| 39 | `2026-08-12T19:58:33.352Z` | `bf-vcsxj` |
| 40 | `2026-08-12T20:04:52.564Z` | `bf-19qh7` |
| 41 | `2026-08-12T20:06:33.824Z` | `bf-mus1k` |
| 42 | `2026-08-12T20:10:15.288Z` | `bf-50zoz` |
| 43 | `2026-08-12T20:12:31.602Z` | `bf-47ugw` |
| 44 | `2026-08-12T20:14:17.489Z` | `bf-3pee6` |
| 45 | `2026-08-12T20:16:47.736Z` | `bf-1o4ag` |
| 46 | `2026-08-12T20:18:37.904Z` | `bf-6awu2` |
| 47 | `2026-08-12T20:20:43.604Z` | `bf-gz3r6` |
| 48 | `2026-08-12T20:24:01.573Z` | `bf-1jxy8` |
| 49 | `2026-08-12T20:25:59.204Z` | `bf-66h5p` |
| 50 | `2026-08-12T20:30:38.310Z` | `bf-2n3ve` |

### 3.2 Alert-wave disposition as of this dispatch (2026-09-09)

Re-read live from the bead store (`bead list --json --status open --limit 5000`, 432 open
beads): **38 of the 50 alert beads are Closed; 12 remain Open** against a target that closed
2026-08-17 — `bf-2ftau`, `bf-hw4i5`, `bf-2dj1g`, `bf-1dzwv`, `bf-1fvk2`, `bf-22514`,
`bf-35bhc`, `bf-mlv3u`, `bf-5egrf`, `bf-4tnae`, `bf-47ugw`, `bf-1jxy8`. Also still Open:
`bf-1rucqm` ("Verify repository health and resume bf-4yjq work") and ten `domchk-*` beads of
this storm's template chain (including two sibling gather legs, domchk-e8cc9d7c /
domchk-cc71d6c0). Recorded here as current state only — alert closure belongs to the
alert-lifecycle owners, not to this gather leg. Per the corpus's target-resolution + dedup
gate, all of these point at completed work and are stale by the bf-3dxljn disposition.

## 4. Logs and error output — what survives and what does not

**Captured / re-verified live this dispatch:**

| Artifact | Where | Status 2026-09-09 |
|---|---|---|
| Needle plaintext worker log, 225 `bf-4yjq` records (the primary crash-era telemetry) | `~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2`, coverage 2026-08-11T14:12:54Z → 2026-08-15T20:35:19Z | **present**; still one rotation from erasure (the active slot's size is what evicts it). Committed copies: `docs/crash-analysis/bf-4yjq-needle-worker-log-extract.log` and `docs/crash/bf-4yjq/raw-logs/needle-worker-log-bf-4yjq-slot2.log` |
| Needle structured event log, 1,071 `bf-4yjq` records (56 claims, 56 dispatched/completed, 56 `outcome.classified`, 55 `bead.released`, 1 `verification.passed`, 1 `bead.orphaned`, 454 heartbeats) | committed copy `docs/crash/bf-4yjq/raw-logs/needle-events-2026-08-12-bf-4yjq.jsonl` | present (census re-run from the committed copy) |
| **All 56 per-run agent session transcripts** (3.5 MB tar.gz; the runs' stdout stream — the era's equivalent of `.beads/traces/*/stdout.txt`) | `docs/crash/bf-4yjq/raw-logs/bf-4yjq-crash-sessions-2026-08-12.tar.gz` + `sessions-index.tsv` + `MANIFEST.sha256` | present; extracted and spot-checked this dispatch (§2). This corrects every report written before 2026-09-06 that says no per-run session evidence survives — the transcripts live under `~/.claude/projects/-home-coding-domain-check/`, a location the pre-09-06 inventories never checked |
| Contemporaneous repo/system metrics compilation (repo 18 GB, loose-object stats, load, disk) | committed copy `docs/crash-analysis/bf-4yjq-system-state-snapshot-2026-09-01.txt` | present; **superseded on crash count and on mechanism provenance** (§8) |
| Bead close record incl. verbatim close reason | `.beads/checkpoint/forensic.jsonl` issue snapshot | present; re-read this dispatch |

**Confirmed absences (re-checked 2026-09-06 per the raw-logs README; the two dated checks this
dispatch could repeat were repeated):**

| What | Status |
|---|---|
| Stack traces / core dumps | **None exist.** SIGKILL-class deaths produce none; `coredumpctl` earliest entry is 2026-08-17 (unrelated). No application error output anywhere in the chain — consistent with the corpus-wide finding of zero domain-check code defects |
| Kernel / journald OOM records for Aug-12 | **Unrecoverable** — system journal's first entry is 2026-08-15T19:46:33 EDT, single boot `52309698` (re-verified live this dispatch) |
| Separate stderr capture for the era | none exists — the transcripts' truncation point *is* the death record; transcript-last-event sits 0–23 s before `outcome.classified` |
| `.beads/traces/bf-4yjq/` | absent — dispatches predate needle trace capture |
| `.beads/logs/*` monitoring logs for Aug-12 | absent — the monitoring layer starts 2026-09-01 (directory re-checked this dispatch) |

## 5. Environmental context

**At crash time (from the contemporaneous snapshot, whose crash-count figure is superseded —
its repo/system figures are the only contemporaneous numbers that survive):**

| Metric | 2026-08-12 value |
|---|---|
| Repository size | **18 GB** (should be < 500 MB) |
| Loose objects | **4,594 objects / 17.20 GiB** (`git count-objects`: count 4594, size 17.20 GiB; in-pack 4081 / 9.60 MiB — ratio inverted) |
| Cause of the bloat | 17+ identical 237 MB `.beads/*.jsonl` snapshots committed by bf-2ildm (`.beads/issues.jsonl` alone 248 MB at crash time) |
| Host memory | 62 GB total; no memory telemetry captured during the crashes |
| Load average | 15–17 on 12 cores (co-symptomatic; the kernel selects OOM victims on memory alone) |
| Disk | 84 % full, ~71 GB free |
| Affected operations | every significant git operation — clone, fetch, checkout, gc, fsck, push |

**Now (re-verified live at HEAD `3efa6a3`, 2026-09-09):**

| Metric | 2026-09-09 value | Threshold |
|---|---|---|
| `.git` size | **106 MB** | < 500 MB |
| Loose objects | 79 / 536 KiB | < 1,000 / < 100 MB |
| Packed | 100.70 MiB consolidated pack, 0 garbage | — |
| Remotes | `origin` = Forgejo (`git.ardenone.com/jedarden/domain-check`), `github-mirror` = GitHub | the bead's own acceptance criteria, met |
| Prevention | `.beads/` fully gitignored (0 tracked files), 10 MB pre-commit gate, effective pack-memory bound ≈3 GiB worst case covering bare `git gc` *and* `git push`, daily/weekly bounded-gc timers | in force; the bf-4yjq precondition (18 GB store) is structurally ruled out |

The repair record with every criterion re-run live is
`docs/crashes/bf-4yjq-cleanup-verification.md` (18 GB → 92 MB on 2026-09-01; re-verified
2026-09-06 and 2026-09-07 — see commit `2a09f34`'s dated addendum).

## 6. Where each figure came from (provenance)

All re-derived 2026-09-09 at HEAD `3efa6a3`:

```bash
# kill ledger, cadence, alert-bead pairing — from the primary worker-log slot
grep 'bf-4yjq' ~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2

# journal floor (Aug-12 kernel records unrecoverable)
journalctl --list-boots          # → single boot, first entry 2026-08-15T19:46:33 EDT

# bead state timeline + verbatim close reason
python3 -  # parse the '"bf-4yjq"' line of .beads/checkpoint/forensic.jsonl

# death command (extract the committed transcript bundle, then read last Bash call per run)
tar -xzf docs/crash/bf-4yjq/raw-logs/bf-4yjq-crash-sessions-2026-08-12.tar.gz

# current repo state / alert-wave disposition
git count-objects -vH; du -sh .git
bead list --json --status open --limit 5000
```

Pairing rule for §3.1: each alert-bead creation line follows its `Crash(-1)` outcome line by
7.0–12.6 s; the 50 pairs are order-monotonic with no interleaving.

## 7. Acceptance criteria

| Criterion | Status |
|---|---|
| Crash timestamp and exit code documented | ✅ §1 — 50 × `Crash(-1)`, window 17:53:53.875Z → 20:30:38.310Z, plus the 1 failure / 4 timeouts / 1 success |
| Workspace path confirmed | ✅ §1 — `/home/coding/domain-check`, from every record's `needle.workspace` span field |
| Available stderr/stdout logs captured | ✅ §4 — all 56 stdout-stream session transcripts survive (committed tar.gz, spot-checked); stderr for the era does not exist (confirmed absence) |
| Bead creation/modification timeline extracted | ✅ §3 — created 2026-07-20 → 56-claim storm → orphaned-blocked → closed 2026-08-17T00:14:14Z rev 2, verbatim close reason |
| Context of what the agent was doing gathered | ✅ §2 — Forgejo-primary remote reconciliation; all 50 deaths at the `git push origin main` step, verified from the transcripts |

## 8. Supersessions to observe when quoting older docs

| Older claim | Superseded by |
|---|---|
| "9 systematic crashes" / "average 1 crash every 17 minutes" (the 2026-09-01 resolution record and system-state snapshot) | **50 deaths** across a 2 h 36 m window, mean cadence 191.9 s — `docs/crash-analysis-bf-1s6c3-2026-09-06.md` and every post-09-06 re-count, including this one. CLAUDE.md flags the 9-crash figure explicitly |
| "Signal -1 = SIGKILL by the Linux OOM killer" stated as observed fact (2026-09-01-era docs) | Exit −1 is the worker's *sentinel* for a missing wait status; the memcg-OOM mechanism is **regime-matched, not kernel-proven**, because the Aug-12 kernel records are gone (journal floor 2026-08-15). The raw-logs README §5 and commit `f5e6377` both carry this correction |
| Crash time range "17:54 – 20:24 UTC" (snapshot) | 17:53:53.875Z → 20:30:38.310Z (§1, from the surviving primary log) |
| "No raw session evidence survives" (canonical report §3, artifact catalog §6) | All 56 per-run transcripts survive — `docs/crash/bf-4yjq/raw-logs/` (2026-09-06 correction, verified again this dispatch) |
| Median kill gap "149 s" | This dispatch's gap median is **155.5 s** (kill-instant → kill-instant, n=49). The 149 s figure came from a different pairing basis; cite the basis, not just the number |
| "gc-side OOM, with bf-198ne (2026-08-16) as the later push-side variant" | The Aug-12 storm was **itself uniformly push-side** (§2) — bf-198ne is the same mechanism kernel-proven later, not a variant that appeared afterward |

## 9. Canonical pointers

- Canonical investigation entry point: `docs/crashes/bf-4yjq-crash-investigation.md`
  (commit `b1140ad`); sibling copy under `docs/crash-investigations/`.
- Root cause: repository bloat → memcg-OOM regime during git operations —
  `docs/crash-analysis-bf-1s6c3-2026-09-06.md` (covers bf-4yjq's 50 deaths the same evening as
  sibling bf-1s6c3's storm) and `docs/crash-investigations/bf-4yjq-root-cause-determination-domchk-54bc57df-2026-09-06.md`.
- Raw-evidence bundle (transcripts, event log, worker-log extract, README with absences and
  pairing rules): `docs/crash/bf-4yjq/raw-logs/README.md`.
- Evidence inventory: `docs/crash-investigations/bf-4yjq-artifact-catalog-2026-09-06.md`.
- Repair verification: `docs/crashes/bf-4yjq-cleanup-verification.md`.
- Classification record: `docs/crashes/bf-4yjq-crash-investigation.md` §11 /
  commit `5d8f474` (INFRASTRUCTURE, repository-bloat regime).

*Gather leg domchk-b06e87d4, 2026-09-09 — docs-only; no code touched; no new cause claim.*
