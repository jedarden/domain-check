# Mitigation Implementation — bf-2ildm (kill storm 2026-08-13)

**Written:** 2026-09-07 by implementation bead **domchk-fb01f52d** ("Implement crash prevention fixes for bf-2ildm")
**Target bead:** bf-2ildm — "Extract GitHub-specific commits" (**Closed 2026-08-16T22:44:38.873Z**, rev 7, work intact)
**Status of this document:** the *implementation record* for the mitigation dispatch — a pointer-and-verification file, not a root-cause narrative. The root cause is **not re-derived here**; the canonical record is the RCA
[`docs/investigations/bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md`](investigations/bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md) (ef960d2), built on the classification
[`docs/crash-classification-bf-2ildm-2026-08-13-14-40.md`](crash-classification-bf-2ildm-2026-08-13-14-40.md) (d82b6a2) and the retrieval bundle
[`docs/crashes/bf-2ildm/`](crashes/bf-2ildm/README.md) (1f56d9c, 43-row per-attempt index). Every figure below was measured first-hand in this dispatch (2026-09-07 ~14:38–14:45 EDT), not cited forward.

**Finding in one line:** the two-level verdict (kill INFRASTRUCTURE / alert FALSE_POSITIVE) means the fix work this dispatch owes is the **repository-bloat layer**, and every repo-side measure that layer calls for was already implemented and landed by earlier beads; this dispatch **re-proved each one first-hand**, carried forward two dispatch-premise corrections (§4), and signposts what is genuinely still open and who owns it (§5). Nothing new was owed on the mechanism — the honest disposition is *"verified, not skipped"* (§2).

---

## 1. The root cause being prevented

**Kill level (INFRASTRUCTURE — repository-bloat regime):** 38 of bf-2ildm's 43 Aug-13 attempts died `exit_code: -1` — kernel memcg-OOM SIGKILL of git/bead-state operations whose footprint exceeded the 12 GiB `MemoryMax` of needle's per-dispatch scope, against a repo still carrying ~18 GB of loose objects (17+ committed 237 MB `.beads/*.jsonl` snapshots) — re-entered at a fixed ~2–5 min re-dispatch cadence for 2 h 16 m. Regime confidence **HIGH**; precise mechanism **MODERATE** (no Aug-13 kernel record survives this host's single Aug-15 boot; assigned by regime match to the bf-4x12ec / bf-198ne / bf-1s6c3 family).

**Alert level (FALSE_POSITIVE):** alert bf-66sw7c fired at a real kill (attempt **19**, kill 14:40:29.551762239Z, 124,759 ms, mid-task; the 14:40:42.628Z stamp is the handler heartbeat 13.1 s later), but the work it implies was lost was completed 2026-08-16 (trace `exit_code: 0`, 85,327 ms; bead closed 22:44:38.873Z). bf-66sw7c is closed (rev 29); no alert-side investigation is owed.

## 2. The dispatch's taxonomy branch, task by task

The template's fix types key off root cause; the skip rule ("Skip if no fixes needed (FALSE_POSITIVE or self-healed)") is applied **per level**, exactly as the RCA applied it (§1 of that document): FALSE_POSITIVE → the alert level is skipped; INFRASTRUCTURE → the kill level's fix layer is dispositioned here.

| Dispatch fix type | Applies? | What this dispatch did | Verdict |
|---|---|---|---|
| *Repository bloat:* run safe-git-gc, update .gitignore | **Yes — this is the branch** | Ran the safe path's `--check-only` rather than a real gc (premise correction 1, §4.1): resources passed, verdict **"GC not needed"** (exit 1 is the verdict, not a failure); `auto-gc-trigger.sh --dry-run` agrees. `.gitignore:66` `.beads/` + `:68–70` `*.db` / `*.db.backup.*` / `*.jsonl` repo-wide; `git ls-files .beads \| wc -l` → **0** tracked files. Size reduction arc: ~18 GB (Aug-12/13 era) → 104 MB (RCA, this morning) → **105 MB** (this dispatch), 0 garbage, `fsck` clean via the health check | ✅ in place — verified, not skipped |
| *Resource pressure:* pre-flight checks, resource limits | Yes, as guard | The limit that matters for this regime is the **pack-memory bound** (`pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1`, repo-local *and* global) — verified effective, worst case ≈ **3072 MiB** inside the 6 GiB ceiling of the 12 GiB scope. Pre-flight entry points live and passing (`preflight-health-check.sh` layer: `resource-monitor.sh --once`, `service-monitor.sh --once`); installed via **`setup-repo-maintenance.sh`**, not the retired cron installer (premise correction 2, §4.2) | ✅ in place — verified |
| *Workflow issues:* error handling, bead closing / alert suppression | Alert layer only | Six 2026-09-02 fixes present; both suites green this dispatch (§3 #8). **One defect remained in the FP branch** — the CLASSIFICATION wiring (D-3) — owned by open bead `domchk-f6fff20f`; its fix landed **locally, unpushed** mid-dispatch (8cc1172, domchk-701bcfa5, InProgress) and its new wiring test passes 13/13 — **not redone here** (§5.1). **Trued ~15:20 EDT:** 8cc1172 is since **pushed** (origin/main e9ab3d4, 0 unpushed) and domchk-701bcfa5 **Closed** (rev 10); `domchk-f6fff20f` still **Open** (rev 3) (§5.1). Note the repo scripts are the *response* layer; alert *creation* is needle-side (dedup gap analysis D-4/D-5) | ⚠️ partial, ownership signed |
| *Service failures:* retry with backoff | No | SERVICE_FAILURE ruled out upstream (RCA §2): deaths are mid-task signal losses, not HTTP 5xx/timeout exits; no gateway involvement in the attempt profile. Gateway re-checked healthy this dispatch (§3 #7) | n/a — ruled out |
| *Code defects:* fix identified bugs | No | None found — zero defects in 157+ investigations of this workspace (RCA §2, ruled out) | n/a — none exist |

## 3. First-hand verification battery (this dispatch)

| # | Check | Result |
|---|---|---|
| 1 | `./scripts/setup-git-gc-config.sh --verify` — the direct fix: `pack.windowMemory=2g` / `pack.deltaCacheSize=1g` / `pack.threads=1`, repo-local **and** global | **exit 0** — effective chain system → global → local, all three keys local; worst-case pack memory ≈ **3072 MiB** (window 2 GiB × threads 1 + deltaCache 1 GiB) inside the 6 GiB ceiling for a 12 GiB dispatch scope |
| 2 | `./scripts/test-gc-memory-bounds.sh` — replays the era's two proven death operations (bf-1ea4g's bounded `git push` over an unpacked backlog; bf-173o7e's bare `git gc --aggressive --prune=now`) under a 768 MiB cgroup | **16/16, exit 0** — both death operations complete bounded (bare gc peak pack-objects RSS **320,312 KB** vs the >12 GiB its unbounded ancestor consumed). bf-2ildm's own death operation is not identified (mechanism MODERATE — no Aug-13 record), so the *family* proof is what is testable; the family is the regime that killed it |
| 3 | `./scripts/check-repo-health.sh` | **exit 0** — pack-memory bound verified, no unmanaged aggressive gc/repack running, unpushed backlog **0** (< 50 warn threshold) |
| 4 | `.gitignore` / tracked bead state | `.beads/` (line 66) + `*.db` / `*.db.backup.*` / `*.jsonl` (68–70) ignored; **0** tracked `.beads/` files — the bloat mechanism cannot re-enter through bead state |
| 5 | `./scripts/setup-git-hooks.sh --check` — the 10 MB pre-commit gate that would have blocked the era's 237 MB snapshot commits | **exit 0** — installed, byte-identical to tracked source (`scripts/pre-commit-repo-size-hook`, shipped dfa60a9) |
| 6 | Timers: `systemctl --user list-timers 'domain-check-*' --all` | **8/8 future-scheduled** — monitoring 10 min, resource-monitor 5 min, service-monitor 2 min, alert-triage hourly, repo-health daily 02:00, auto-gc daily 02:30, incremental gc daily 03:00, full gc Sun 04:00 (MemoryMax=4G) |
| 7 | `./scripts/resource-monitor.sh --once` / `./scripts/service-monitor.sh --once` | both **PASS** — memory 43 GB avail, disk 43 GB free, load 6.76, pressure 0%, unsafe-gc none; inference gateway HEALTHY (HTTP 200, via `-skf` — the plain-`-sf` cert false alarm), "All services healthy" |
| 8 | Alert-layer suites: `test-crash-alert-fixes.sh`, `test-closed-bead-filter.sh` | both **exit 0**, run twice — first against the pre-8cc1172 scripts (~14:39 EDT), then re-run after the co-tenant wiring commit landed (~14:55 EDT), still green (13-check fix suite incl. the FALSE_POSITIVE branch markers; 7/7 functional closed-bead replay) |
| 9 | Size / objects | `.git` **105 MB**; 147 loose objects / 1.14 MiB; 11,700 packed in 2 packs / 99.78 MiB; **0 garbage** |
| 10 | Divergence / backlog | `git log origin/main..HEAD` **empty** at first check and re-verified immediately before commit — the 422-commit backlog precondition does not exist today |
| 11 | Host resources (`free -g`, `df -h /`, `uptime`) | 44 G available; 43 G disk free (above the 30 G warning line); load **5.86–6.89** 1-min — above the CLAUDE.md "<5" ideal because of co-tenant work on this shared box, but inside every operative guard (safe-git-gc max 15, resource monitor OK, gc preflight passed) |

**Size-reduction arc (crash precondition → today):** **~18 GB** `.git` with ~17 GB loose objects / 17+ committed 237 MB `.beads/*.jsonl` snapshots (2026-08-12/13 era) → **93 MB** after the verified 2026-09-01 cleanup → **94 MB** re-verified 2026-09-06 → **104 MB** (RCA §5, this morning) → **105 MB** this dispatch (normal churn, 0 garbage). The regime that killed bf-2ildm 38 times is gone and has held for five days past the last scheduled re-verification — two independent same-day measurements agree.

### 3.1 Re-executed first-hand by the closing attempt (~15:00–15:20 EDT / 19:00–19:20Z, HEAD e9ab3d4, `origin/main..HEAD` = 0)

The first attempt of this bead wrote §3 and died before committing; this attempt re-ran **every** check above live before committing. All green — no regression. Figures that moved, all normal churn: loose objects 147 → **175** (1.14 → 1.32 MiB), disk free 43 G → **34 G** (still above the 30 G warning line — the one number worth watching), load 4.67–6.55 (co-tenant work on this shared box), and safe-gc printed its worst case as ≈3584 MiB on this pass (its own overhead allowance; still inside the 6 GiB ceiling). The gc-bounds replay's peaks moved within noise (push **232,432 KB**, bare-gc pack-objects **320,548 KB**). Additions this pass:

- `test-crash-alert-classification-wiring.sh` re-run at HEAD: **13/13, exit 0** — the D-3 branch fires on the pushed state too.
- `safe-git-gc.sh --check-only` re-run: preflight passed (45,633 M avail, 34 G disk, load 4.67 — all inside limits) → **"GC not needed", rc=1** — the verdict, per §4.1.
- The gateway "unavailable" line `service-monitor.sh --once` prints was re-classified first-hand: plain `-sf` fails **curl 60** on the self-signed cert while `-skf` answers **HTTP 200 "ok" in 55 ms** — the known cert false alarm (CLAUDE.md documents it), not an outage. Report the monitor's gateway line only after a `-skf` re-check.

Attribution note for future re-runs: `test-closed-bead-filter.sh` must be run **from inside the bead workspace**. Run from a bare `git archive HEAD scripts/` extract it fails 4/7 on its own premise line — its `bead show` status probe (line 82) finds no workspace, so the suite correctly reports "test premise broken … not a filter result". Same script, real worktree: **7/7**.

## 4. Dispatch-premise corrections

Carried forward from the bf-1ea4g implementation record (domchk-c68ae9bb) and **re-verified live this dispatch**, so the next dispatch inherits the correction, not the mistake.

### 4.1 "Repository bloat: Run safe-git-gc"

**Running a real gc would have been the wrong action.** The repo is 105 MB with 0 garbage; a gc here is pure churn. The correct INFRASTRUCTURE-branch behavior when the repo is healthy is the **check**: `./scripts/safe-git-gc.sh --check-only`, which runs the full fail-fast preflight (invalid config / insufficient memory / disk / load → exit 2 *before* any git work) and then returns a verdict. Its exit 1 on "GC not needed" is the **verdict, not a failure** — the same shape as `auto-gc-trigger.sh --dry-run`. Both ran clean this dispatch (§3 #1, #3; check-only run at 14:39:14 EDT: last gc size 101M, 45,587M avail, load 6.89, all inside limits).

### 4.2 Resource limits and the monitoring installer

The template's "add resource limits / pre-flight checks" is **already satisfied by config, not by new code**: the operative limit for this regime is the persistent pack-memory bound (§3 #1) — adding more would duplicate it — and pre-flight checks exist as `preflight-health-check.sh` plus the two one-shot monitors (§3 #7), installed as systemd **user** timers by **`./scripts/setup-repo-maintenance.sh`**. Do **not** reach for `./scripts/monitoring-setup.sh`: at HEAD it is still the retired **cron** installer (crontab writes at lines 15–22), a silent no-op on this cron-less NixOS box (gap G-7). Re-confirmed at HEAD this dispatch.

## 5. What this dispatch did not implement, and who owns it

### 5.1 FALSE_POSITIVE branch — the CLASSIFICATION wiring defect (owned: `domchk-f6fff20f`)

- **Was live on `origin/main` at 25eda96; fixed there since:** `scripts/crash-alert-manager.sh:279` captured `CLASSIFICATION=$(echo "$CLASSIFICATION_OUTPUT" | head -1)` while `crash-classifier.sh main()` prints its `====` banner first — so `CLASSIFICATION` was the banner string, the manager's FALSE_POSITIVE branch was dead code, the 300 s cooldown keyed on a constant, and `crash-history.jsonl` recorded a garbage classification field. Verified this dispatch against the pre-fix state; **trued ~15:20 EDT:** with 8cc1172 now pushed, `origin/main` (e9ab3d4) carries the **fix**, not the defect. Gap-analysis ID **D-3** (`docs/alert-deduplication-gap-analysis-2026-09-07.md` §134).
- **The fix landed mid-dispatch, locally and unpushed** at **14:51:30 EDT**: the worktree HEAD advanced 25eda96 → **8cc1172** ("crash classification signals + classifier-to-alert wiring", bead **domchk-701bcfa5**, InProgress at the time) — robust token extraction with a head-1 fallback at manager lines **285–286**, plus three new tests (`test-crash-alert-classification-wiring.sh`, `test-crash-classifier-signals.sh`, `test-trace-slot-provenance.sh`). That dispatch ran the new wiring test: **13/13, exit 0** — "line 1 is FALSE_POSITIVE", i.e. the branch that D-3 showed unreachable now fires. **Trued ~15:20 EDT:** 8cc1172 is **pushed** (`origin/main..HEAD` = 0, origin at e9ab3d4), domchk-701bcfa5 **Closed** (rev 10, 18:57:05Z), and the wiring test re-ran **13/13** at that state (§3.1); **`domchk-f6fff20f` remains Open** (rev 3, 19:02:12Z) — closing the defect bead is its owner's call, not this dispatch's. A **further** co-tenant rewrite of the manager remains uncommitted on top of 8cc1172 (+137/−1 at both ~14:55 and ~15:20 EDT), and the three new wiring tests were observed **staged for deletion** while still present on disk — in-flight co-tenant state, recorded as observation only, not a verdict on the fix. Per the shared-worktree sweep hazard, this dispatch **touched neither script** — committing or re-editing them would collide with the owner's in-flight work.
- **Not broken by it:** closed-bead filtering, dedup, completion awareness, cooldown *effect* and cascade collapse (6 kills → 1 alert in the bf-6d3d6-shaped replay, domchk-81938e89) all verified working, and both suites pass this dispatch before *and* after 8cc1172 (§3 #8). The corpus lesson stands: grep-marker suite tests cannot see wiring bugs — a functional replay can; the new wiring test is exactly that.

### 5.2 Single-slot trace retention (no owner) — the gap that made bf-2ildm expensive

The last attempt's trace overwrites all prior ones. This is the blind spot that produced the wrong 2026-09-02 RCA (which read the surviving Aug-16 *success* trace as the record of the reported crash) and the classifier's UNKNOWN verdict with its trace-provenance warning. Compensating control in place: retrieval bundles with per-attempt indexes (`docs/crashes/bf-2ildm/`, domchk-ea755548) — built by hand, after the fact. Candidate improvement, **unowned**: multi-slot retention keyed by (bead, attempt). Related: worker transcripts have no retention requirement at all (gap M-4 in `docs/crash-prevention-gaps-bf-1ea4g.md`) — and transcripts are the evidence class that actually settled bf-1ea4g. Not repo-fixable in this dispatch.

### 5.3 Alert-stamp provenance (no owner)

Alert beads carry the crash handler's post-kill heartbeat, not the kill instant — bf-66sw7c's 14:40:42.628Z stamp is 13.1 s after attempt 19's real kill 14:40:29.551762239Z. Documented control: cite kills from the per-attempt index, never the bead stamp (the ordinals correction in the RCA §3 is the same lesson). Candidate: stamp the kill instant on the alert bead — which would live in the same co-tenant-contested alert scripts as §5.1, so it must wait for that owner's landing regardless.

### 5.4 Per-clone hook protection (documented control, no automation gap closed here)

The 10 MB pre-commit gate is per-clone; a fresh clone is unprotected until `./scripts/setup-git-hooks.sh install` (idempotent; `--check` exit 0 = installed and current, verified §3 #5). The protection is the documented procedure plus the daily health check; this dispatch adds no code to it.

### 5.5 NEEDLE-side residuals (not repo-fixable)

- **H-1 — the self-amplifying retry stop-condition** (`docs/crash-prevention-gaps-bf-1ea4g.md` §4): the single amplifier that turned bf-2ildm's first kill into **38** (each re-claim re-entered the same doomed regime at a 2–5 min cadence for 2 h 16 m). Policy change on the NEEDLE side; tracked in the workspace gap register `docs/crash-prevention-requirements.md` (G-1..G-13). Must not be filed as a print-only detection rule.
- **M-2 — dispatch-scope memory telemetry:** nothing observes the 12 GiB memcg that did the killing; resource monitoring here is host-wide (§3 #7 measures the host, not the scope). Requirements P1 in the same register.

## 6. Acceptance-criteria map

| Criterion | Where |
|---|---|
| Implement fixes appropriate to root cause | §2 — every repo-side measure the repository-bloat branch calls for was already landed (commits: dfa60a9 pre-commit gate, 8d326cc backlog monitor M-1, 7160e6a its daily wiring, pack-memory bound tracked at HEAD); this dispatch verified rather than duplicated, and signed the one owned defect it must not touch (§5.1) |
| Test fix effectiveness | §3 — the 16/16 death-operation-family replay under a bounded cgroup, plus the 10-check live battery, all green 2026-09-07 |
| Document changes | This file + the dated addendum appended to the RCA's §6 (mitigation status) pointing here |
| No regressions introduced | **Zero code changes** — this dispatch's commit is documentation only; both alert suites and the full battery were run after the worktree state was confirmed, and the commit touches exactly its two doc paths via a private index (no co-tenant files swept) |

## 7. References

- Canonical bf-2ildm RCA: `docs/investigations/bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md` (ef960d2) — this dispatch appends a dated pointer to its §6
- Classification: `docs/crash-classification-bf-2ildm-2026-08-13-14-40.md` (d82b6a2) and `docs/crash-classification-bf-2ildm-2026-08-13-15-01.md` (e3a8820); bundle: `docs/crashes/bf-2ildm/` (1f56d9c, `attempt-index.tsv` attempt 19)
- Superseded: `docs/investigations/root-cause-determination-bf-2ildm-2026-09-02.md` (domchk-b672deb9 — banner at top points to the RCA)
- Mechanism family: `docs/crashes/bf-198ne-crash-report.md` (push-side memcg OOM), `docs/crash-analysis-bf-1s6c3-2026-09-06.md` (bloat mechanism), `docs/maintenance/repository-maintenance-guide.md` (mitigation layer)
- Gap registers: `docs/crash-prevention-requirements.md` (G-1..G-13), `docs/crash-prevention-gaps-bf-1ea4g.md` (H-1, M-2, M-4), `docs/alert-deduplication-gap-analysis-2026-09-07.md` (D-3)
- Sibling implementation record with the same shape: `docs/mitigation-implementation-bf-1ea4g-2026-08-13.md` (domchk-c68ae9bb, c04f017) — §4's premise corrections originated there
- Fix ownership (trued 2026-09-07 ~15:20 EDT): the D-3 fix 8cc1172 (domchk-701bcfa5, **Closed** rev 10) is **pushed** — origin/main at e9ab3d4, `origin/main..HEAD` = 0; the defect bead `domchk-f6fff20f` (D-3 CLASSIFICATION wiring, P2) remains **Open** (rev 3) — closing it is its owner's call

*Written by domchk-fb01f52d attempt 1, 2026-09-07 (~14:38–14:57 EDT / 18:38–18:57Z); that attempt died before committing. Re-executed in full, trued up (§3.1, §5.1, §7) and committed by the closing attempt, ~15:00–15:20 EDT / 19:00–19:20Z, against HEAD e9ab3d4. All live figures first-hand this session; historical figures cited to the documents that established them.*
