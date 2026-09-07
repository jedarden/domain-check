# Root Cause Analysis — bf-1ea4g (kill storm 2026-08-13)

**Written:** 2026-09-07 by investigation bead **domchk-e940d850** ("Determine root cause and recommend mitigation for bf-1ea4g")
**Target bead:** bf-1ea4g — "Document local main branch state" (created 2026-08-13T07:14:47Z, **Closed 2026-08-13T09:10:16.731Z** — re-verified via `bead show` this session)
**Status of this document:** a *rendering* — it exists because this dispatch names this exact path. It re-derives nothing. The **canonical bf-1ea4g record** is
[`docs/investigations/bf-1ea4g-root-cause-determination-2026-09-02.md`](investigations/bf-1ea4g-root-cause-determination-2026-09-02.md) (root-cause re-determination, domchk-c2b8c832), and the **one-stop corpus inventory** is
[`docs/crash-inventory-bf-1ea4g-summary.md`](crash-inventory-bf-1ea4g-summary.md) (domchk-6a5f4207, commit 364f187), which cross-registers this file. Correct evidence belongs in those, not here.

**Classification (both layers):** kill = **INFRASTRUCTURE**; alert = **FALSE_POSITIVE** (self-recovered).
**Root cause (one sentence):** unbounded `git push` — pack-objects materializing a **422-commit unpushed backlog** still carrying retired bead-forge object mass on a repo still tracking `.beads/` state, inside the needle dispatch scope's **12 GiB `MemoryMax`**, with no `pack.windowMemory` bound in effect → memcg-OOM-class SIGKILL → needle's `exit_code: -1` sentinel, retried **56 times** because nothing bounded the retry.

---

## 1. Determination at a glance

| Field | Determination | Confidence / basis |
|---|---|---|
| Event | **Not one crash — 56 kills + 1 success.** 57 attempts 07:17:49Z → 09:08:30Z, `56 × exit_code −1`, then attempt 57 exits 0 and closes the bead | HIGH — `docs/crashes/bf-1ea4g/attempt-index.tsv`, corroborated by 57 surviving first-line-tagged transcripts |
| Any single named instant | One of those 56 kills (e.g. alert bf-1nb5u's 08:23:51.806Z = attempt 30's post-kill `HANDLING_RELEASE_DONE` heartbeat; the kill is 08:23:44.918Z) | HIGH — resolved byte-exact from the raw worker log |
| Killer operation | **`git push`** — 54 of 57 attempt transcripts end mid-push, 2 mid-`git commit`, 1 at `bf close` (the success) | HIGH — last-tool-call analysis over all surviving transcripts |
| Mechanism | memcg-OOM-class SIGKILL at the dispatch scope's 12 GiB `MemoryMax` (kernel-proven for the era and the sibling instants: bf-198ne, Aug-16 — `task=git`, `CONSTRAINT_MEMCG`, 720 commits / 5.6 GB tree) | HIGH on the class; **MEDIUM** it was this exact instant's killer |
| Alert layer | **FALSE_POSITIVE** — attempt 57 closed the bead 88 min after the first kill, deliverable intact (`main_branch_state_bf-1ea4g.json`, blob `e77648c7` on origin/main), zero data loss | HIGH |
| Code defect | **NONE** — the killed process was `git`; the workload a docs snapshot. Consistent with every investigation in this corpus | HIGH |

**Why MEDIUM on the instant's specific killer:** no Aug-13 kernel record can exist. journald's single boot begins 2026-08-15 19:56:33 EDT; `kernel.dmesg_restrict` blocks the ring buffer past the Aug-14 reboots; the earliest surviving OOM line is Aug-16 00:27:35 EDT. That is a *coverage gap*, not evidence for any alternative mechanism — the operation correlation (54/57 on one heavy git operation scattered over 1 h 50 min, not time- or fleet-event-correlated) is what retires the SIGHUP-cascade and other era readings.

## 2. Root cause and its amplifiers

**Primary:** the bound that turned "big" into "dead" — no `pack.windowMemory`/`deltaCacheSize`/`threads` config existed until 2026-09-02, so `git push`'s pack-objects was free to materialize every object the remote lacked over a multi-hundred-commit backlog inside a 12 GiB ceiling.

**Amplifiers (why 56 times, and why ~90 alert beads):**
1. **Self-amplifying retry loop** — each killed attempt had already committed its snapshot onto main, so every retry's push was bigger than the last (main advanced one commit per attempt). Nothing bounded the retry (gap **H-1**, still open).
2. **No memory bound on git transport** — landed 2026-09-02, covers gc *and* push (§5).
3. **Pre-dedup alerting** — one alert bead per kill (pre-0.4.2 needle): 56 ALERT-shaped beads against a target that closed the same morning (§6).
4. **Fleet CPU saturation** all morning (71/71 samples 07:00–10:00Z, peak 19.87 on 9 recorded cores) — context that slows pushes and raises per-operation pressure, not the mechanism.

**Alternatives ruled out:** network/remote error (would exit 1 *with* an error result; these pushes return *no result*); needle timeout (records as exit 124 — 5 same-day bf-4k2ws specimens, none here); disk exhaustion (no signature in any attempt); SIGHUP cascade (the sentinel carries no signal number, and deaths are operation-correlated); code defect (uninvolved).

## 3. The dispatch taxonomy, branch by branch

| Branch | Verdict | What the branch asked | Answer |
|---|---|---|---|
| INFRASTRUCTURE | ✅ **the kill** | Specific failure; OOM logs; repo health | Failure = memcg-OOM-class SIGKILL of `git push` inside the 12 GiB dispatch `MemoryMax`. **No Aug-13 OOM log can exist** (§1, evidence-retention gap M-3/G-8). Repo health re-verified live 2026-09-07: `.git` **104 MB**, 88 loose objects / 668 KiB, 2 packs / 99.78 MiB, 0 garbage, `origin/main...HEAD` **0/0**, `check-repo-health.sh` exit 0 — the 18 GB bloat era is repaired and holding |
| FALSE_POSITIVE | ✅ **the alerts** | Why triggered; suppression logic; was work completed | Triggered because pre-0.4.2 needle minted one alert per kill with no closed-bead/duplicate/completion awareness. Work **was** completed — bead closed 09:10:16.731Z, deliverable on origin/main. The six 2026-09-02 fixes (closed-bead filter, duplicate detection, processed-alert tracking, completion awareness, exit-code validation, cooldown) are implemented and tested (`test-crash-alert-fixes.sh`; `test-closed-bead-filter.sh`) |
| SERVICE_FAILURE | ❌ | gateway 5xx / retry patterns | Ruled out — no 5xx class; the deaths are local, mid-operation, 13.8 s into the push, with no result rather than an error result |
| CODE_DEFECT | ❌ | code issues | Ruled out — killed process was `git`; zero domain-check defects across the whole corpus |

## 4. Evidence

- **Evidence bundle:** [`docs/crashes/bf-1ea4g/`](crashes/bf-1ea4g/README.md) (commit 2ce9cd9) — `attempt-index.tsv` (1 header + 57 rows with source line numbers), verbatim attempt-30 event bracket (worker-log lines 3283–3361), the 1,093-record Aug-13 worker-log extract (gzipped), the attempt-30 session transcript (ends on an unanswered `tool_use` 13.8 s before the kill record = **mid-task, mid-push**, refuting the older "post-completion cleanup" framing), system-state doc. `sha256sum -c MANIFEST.sha256` → **6/6 OK** this session (run from inside the bundle directory; the worktree README/MANIFEST carry a co-tenant's uncommitted +59-line addition, not attributed here).
- **Technique that settled it:** last-tool-call analysis over a bead's surviving attempt transcripts (enumerate by first-line dispatch tag + mtime, read each transcript's final `tool_use`). The operation the deaths cluster on is the killer.
- **Era vs. live:** the crash-era repo (18 GB Aug-12 / `.beads/` tracked + 422-commit backlog Aug-13 / 5.6 GB mass Aug-16) no longer exists; do not feed today's 104 MB repo into the era's bloat premise. Backlog arc 660 → 422 → 0: `docs/branch-divergence-analysis.md`.
- **Kernel-proven sibling:** bf-198ne (2026-08-16) — the push-side variant, `task=git`, `CONSTRAINT_MEMCG` at 100 % of the 12 GiB `MemoryMax`; report at `docs/crashes/bf-198ne-crash-report.md`.

## 5. Mitigation — recommended then, live-verified now

Nothing new is owed on the mechanism: every measure below was already landed by earlier beads in the chain; this session **re-ran** the battery first-hand.

| Measure | Status, verified 2026-09-07 (this session) |
|---|---|
| **Git-transport memory bound** — `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` (window is per-thread, so threads pinned), repo-local + global | ✅ `./scripts/setup-git-gc-config.sh --verify` **exit 0** — effective chain system → global → local, worst case ≈ **3072 MiB** inside the 6 GiB ceiling for a 12 GiB scope |
| **Death-operation recurrence test** — bounded push over an unpacked backlog | ✅ `./scripts/test-gc-memory-bounds.sh` **16/16, exit 0** — the bf-1ea4g case: bounded `git push` over a 192 MiB unpacked backlog under `MemoryMax=768M` → exit 0, remote receives the backlog, store stays loose, peak push RSS **231,468 KB** (vs >12 GiB unbounded on 2026-08-13; fifth independent pass in the 231–233 MB band) |
| **Repo de-bloat held** | ✅ `.git` 104 MB, 0 garbage, `fsck`/health exit 0, divergence 0/0 |
| **`.beads/` re-bloat path closed** | ✅ `.gitignore:66` `.beads/` (plus `*.db`, `*.jsonl`), **0** tracked `.beads/` files |
| **10 MB pre-commit gate** | ✅ `./scripts/setup-git-hooks.sh --check` exit 0 (hook byte-identical to tracked source) |
| **Scheduled maintenance** | ✅ **8/8** `domain-check-*` systemd user timers future-scheduled |
| **Unpushed-backlog monitor (M-1)** — the rule this crash uniquely motivates | ✅ landed (8d326cc) + daily wiring (7160e6a); health check reports `ahead of upstream: 0` — CLEAR |
| **Alert-system fixes** (the FALSE_POSITIVE layer) | ✅ implemented 2026-09-02; suite re-verified by sibling runs today |
| **Evidence retention** (bundle + transcripts) | ✅ the artifact class that actually settled this crash is now committed and manifest-pinned |

## 6. Residual gaps and implementation path

| Gap | Why it matters here | Path |
|---|---|---|
| **H-1 — NEEDLE retry stop-condition** | The single amplifier that turned 1 kill into 56; each retry grew its own kill condition | NEEDLE-side policy change, not a repo fix — must not be filed as a print-only detection rule. Tracked in `docs/crash-prevention-gaps-bf-1ea4g.md` and `docs/crash-prevention-requirements.md` |
| **M-2 — dispatch-scope memory telemetry** | Nothing observes the 12 GiB memcg that did the killing; resource monitoring is host-wide | NEEDLE/host collector addition; requirements P1 in `docs/crash-prevention-requirements.md` |
| **Alert-pool churn** | Census this session (filter: `bead list --json --limit 5000`, title substring `bf-1ea4g`; ALERT-shape = `^ALERT`): **90** title-mentions — 69 closed / 15 open / 4 in_progress / 2 deferred; ALERT-shaped **56** (one per kill) — 48 closed / 5 open / 3 in_progress. The pool grows only by minting new dup children (88 → 90 since the 2026-09-07 inventory) while ALERT-shaped stays pinned at 56 | Verify target state, resolve against the canonical record + `docs/crash-context-bf-1ea4g-2026-08-13.md`, close no-commit — no re-derivation. This dispatch (domchk-e940d850) is itself one of the 90 and resolves the same way, with this file as its named deliverable |

## 7. Acceptance-criteria map (this dispatch)

| Criterion | Where |
|---|---|
| Root cause identified with evidence | §1–§2, evidence in §4 (canonical: re-determination §2–§8) |
| Specific mitigation recommended | §5 — the direct fix is the pack-memory bound; every layer live-verified this session |
| Preventive measures documented | §5 table + §6 residual gaps |
| Implementation path clear | §5 (landed, with commits/tests named) + §6 (the two open items are NEEDLE-side, with their tracking docs) |

*Similar crashes:* bf-198ne (push-side, kernel-proven), bf-4x12ec / bf-173o7e (gc-side), bf-4k2ws (same-day 55-kill storm), bf-1s6c3 / bf-4yjq (the bloat era itself) — all one mechanism family, all closed by the same bound.
