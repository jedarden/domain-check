# Investigation Report (Final): Domain-Check Crash Corpus

**Bead:** domchk-e843c4f1 — step 3 of 4 (gather → draft → **recommendations** → commit)
**Chain:** domchk-65afcc88 (compiled findings, closed) → domchk-d6871df1 (draft, closed, 9338e2b) → **domchk-e843c4f1 (this final)** → domchk-0d3c11c9 (commit + link)
**Umbrella:** domchk-11c26b24 "Document investigation results and recommendations" (parent bead bf-4829x8)
**Report date:** 2026-09-06 (all `[LIVE 09-06]` figures re-derived 2026-09-06, twice — 02:25 UTC in the draft, 23:07 UTC here, §11)
**Supersedes:** the draft of this report at
`investigation-report-draft-2026-09-06-domchk-d6871df1.md` (commit 9338e2b). Same verified content;
this version adds §9 (recommendations), adopts §7 as the canonical superseded-claims list, and
renumbers the handoff (§10) and re-derivation (§11) sections.
**Report type:** **Delta report** — corrects and extends the anchor; deliberately does not re-derive it
**Baseline:** [`docs/investigations/final-investigation-report-2026-09-01.md`](final-investigation-report-2026-09-01.md), commit 383241f
**Primary input:** [`docs/investigations/findings-compilation-2026-09-05-domchk-65afcc88.md`](findings-compilation-2026-09-05-domchk-65afcc88.md)
**Classification:** INFRASTRUCTURE (cgroup-scoped OOM) + TOOL ISSUE (NEEDLE alerting) — **not a code defect**
**Confidence:** HIGH for L1/L5 and every event 2026-08-16 onward; MEDIUM for 2026-08-12/13/14 (attested only, §8)
**Status:** FINAL — recommendations delivered (§9); remaining work is commit/link only (§10)

---

## 1. Executive Summary

Over 2026-08-12 → 2026-09-06 the domain-check workspace generated 200+ crash alerts. The anchor
investigation (383241f) concluded correctly that **domain-check code is defect-free** and that the
workload's failures were external. That conclusion stands. What has changed since is the *mechanism*:
the anchor's stated cause — "systemd-oomd at 94.71% memory pressure → SIGHUP cascade" — is
**superseded**. Surviving primary evidence shows a different and narrower mechanism.

**The corrected finding in one line:** agents were killed by **kernel memory-cgroup OOM inside their
per-dispatch `MemoryMax=12 GiB` scopes**, triggered by unbounded-memory processes (bare
`git gc --aggressive`, `node`/vitest) — never by host-wide memory exhaustion, and never by SIGHUP.
`exit_code=-1` is needle's `unwrap_or(-1)` **sentinel for an unrecorded signal death**, not a signal
number; reading it as "SIGHUP" is what produced the original misdiagnosis.

**Five causal layers** (§3) account for the whole corpus: scoped-OOM kills (L1), NEEDLE alerting
defects that turned ~200 real events into 200+ alerts of which ~60% were duplicates (L2), a needle
crash-handler defect that was itself a kill source (L3), service-class `exit_code=1` waves that are
the *current* dominant failure mode (L4), and explicitly **not** domain-check code (L5).

**Current state — all re-derived 2026-09-06:** zero `exit_code=-1` in this worker's log since
2026-08-17T16:00:26Z; zero real kernel memcg kills since 2026-08-17 (all 15 since then are the
2026-09-02 synthetic-test and bounded-gc-scope kills); repository healthy and optimal (92 MB `.git`,
1 pack / 90.34 MiB, 0 garbage) with the gc memory bounds proven in production; host far inside safe
operating limits (44 G mem available, 87 G disk free).

**The dominant ongoing impact is not crashes — it is wasted effort** (§6): an investigation that
concluded on 2026-09-01 has since been re-synthesized at least three times by parallel auto-split
chains, including this one. What to do about it is §9.

---

## 2. Scope and Relationship to Prior Reports

This report exists because the corpus already contains three near-duplicate syntheses of the same
concluded investigation. Re-deriving a fourth would add noise, not information. Accordingly:

| Document | Commit | Relationship to this report |
|----------|--------|-----------------------------|
| `docs/investigations/final-investigation-report-2026-09-01.md` | 383241f | **Anchor.** Conclusions stand; several mechanism details superseded (§7). Its opening sections still state the corrected-away mechanism — a correction banner has been added to its header. |
| `docs/investigations/signal-minus-one-final-investigation-report-2026-09-02.md` | b1ae579 | Final report for the exit −1 / bf-173o7e chain. Consistent with §3 here. |
| `docs/investigations/bf-173o7e-root-cause-determination-domchk-2e371a2c-2026-09-02.md` | 07ab240 | Highest-confidence RCA of the largest single event. Subsumed into §3 L1 and §4. |
| `docs/investigations/findings-compilation-2026-09-05-domchk-65afcc88.md` | 5b4be43 | This report's input. Verification tags and the superseded-claims table originate there. |
| `docs/investigations/evidence-compilation-2026-09-01-crash-investigation.md` | **untracked** | Sibling chain's artifact — the corpus's best provenance-tagged source. **Still uncommitted** (gap 2, §10; recommendation R8). |

**What this report adds that no prior document contains in one place:** the corrected mechanism
(L1–L5), the verified timeline with per-claim provenance, the additions since the anchor report, an
impact assessment that separates *verified* from *attested-only* figures, and a prioritized,
owner-assigned recommendation set (§9).

---

## 3. Root Cause Analysis (Corrected)

The corpus resolves to five independent causal layers. Only L1 kills agents by signal; L2 and L3
generate alerts and deaths of their own; L4 is the present-day failure mode; L5 is the null result
that the whole investigation exists to establish.

### L1 — Unbounded memory inside a bounded cgroup (produces `exit -1`)

**Mechanism.** Each needle dispatch runs inside a systemd scope with `MemoryMax=12 GiB`. A process
inside it that allocates without bound — bare `git gc --aggressive --prune=now` (its
`git pack-objects` child in particular) or a `node`/vitest run — crosses that line, and the kernel
delivers SIGKILL under `CONSTRAINT_MEMCG`. The agent dies with no Go panic and no stack trace; needle
records `exit_code=-1` via `code().unwrap_or(-1)`.

**The sentinel.** `exit_code=-1` is therefore *not* a signal number and *not* SIGHUP. A real SIGHUP
is catchable and would be recorded as signal 1. This single misreading is the root of the anchor
report's superseded mechanism.

**Amplifier.** Repository bloat. At bf-1s6c3 (2026-08-12) the repo stood at **18 GB with 17.16 GB of
loose objects** — 36× a healthy ~500 MB — so routine git operations were enough to breach the bound.

**The decisive constraint evidence.** Every surviving kill record reads `CONSTRAINT_MEMCG` inside
`needle.slice/run-p*.scope` or `app.slice/run-p*.scope`. The host was *not* out of memory — which is
why host-wide memory alerting never caught any of this.

**Largest single event — bf-173o7e, 2026-08-14.** A duplicate gc bead was dispatched **132 times**;
**129 attempts died `exit -1` over ~10.5 h**. The kill durations are flat across all 129, which proves
the object set never shrank between attempts — each attempt re-read the same 17 GB and died at the
same point. Actual packing completed **Aug-14 23:25 → Aug-17 by a non-attempt process**, not by any
of the 129.

**Status: mechanically fixed and proven in production.** Persistent git config bounds the bare path
(`pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1`; the window limit is per-thread so
threads must be pinned) → worst case ≈3 GiB per pack run, applied repo-locally *and* globally
(533cb46). Safe-gc scripts plus systemd user timers bound the scripted paths. The bound fired as
designed on 2026-09-02 and the gc completed cleanly. `[LIVE 09-06]` all three keys set; repo 92 MB
`.git`, 1 pack / 90.34 MiB, 32 loose / 292 KiB, **0 garbage**. The bound is *enforced* but not yet
*observed* — no monitoring script verifies it (§9, R6).

### L2 — Detection and alerting defects (produce alert volume, not crashes)

NEEDLE's crash detection lacks four capabilities, and each omission has an observable consequence:

| Missing capability | Observable consequence |
|--------------------|------------------------|
| Work-completion detection | Post-completion kills alert as crashes (bf-5tgsk: completed 16:35:54, killed 16:36:24, bead closed 16:36:51 — all work landed) |
| Self-healing awareness | crash → retry-exit-0 sequences alert (bf-6bio4g) |
| Deduplication | ~60% of the 200+ alerts were duplicates; bf-173o7e alone drew **129 duplicate alerts** |
| Event-pattern recognition | 10+ crashes in 10 minutes read as 10 separate events rather than one infrastructure event |

**The auto-split loop compounds L2.** It re-dispatches onto already-resolved beads (bf-4ifshb,
bf-1cd5v6 closed 3×) and, at least once, generated a **wholly fabricated premise** — a claimed
bf-173o7e "verification blocker" requiring a script fix and a `--skip-verify` flag, neither of which
exists (6369467: `verify-work-completion.sh` has no cluster logic; `bead close` has no such flag).

**Status:** repo-side mitigations implemented (`scripts/crash-alert-manager.sh` + classifier + dedup +
5-minute cooldown; `scripts/verify-work-completion.sh` pre-close gate). **Upstream NEEDLE fixes
pending** — the loop that produced this very chain is itself unremediated (§9, R1–R4; §10, gap 5).

### L3 — Needle crash-handler defect (a kill source in its own right)

When an agent closes its bead and then dies by signal, the crash handler runs `bead release` against
the now-closed bead, gets **exit 4**, and the unhandled error kills the whole worker. **18 worker
deaths on 2026-08-26** — the last fleet-wide signal-death day.

**Status: upstream fix needed.** No evidence of an upstream fix anywhere in the corpus (§9, R3).

### L4 — Service-class failures (the current dominant failure class)

Since 2026-08-26 the failure mode has shifted from signal deaths to **synchronized `exit_code=1`
waves — 13–15 workers per minute, cross-workspace** — which are inference-service-class, not signal
deaths. Two additional items in this layer:

- **`needle-otel-collector` CrashLoopBackOff** on config schema drift (`filterprocessor`
  `log_statements`, `awss3` exporter keys) — all-day telemetry 503s, unrelated to workload health.
- **A documentation-induced false alarm:** the documented gateway health check `curl -sf …` fails
  with **curl 60 (self-signed certificate)** while the gateway answers `200 ok`. `[LIVE 09-06]`
  re-verified twice: `-sf` → exit 60; `-skf` → `ok`, exit 0. The repo's CLAUDE.md showed the `-sf`
  form; corrected alongside the draft (§5).

**Status:** classified. Remediation is fleet/service-side except the health-check doc fix, which is
in this repo.

### L5 — Explicitly not a cause: domain-check code

**Zero defects across 157+ investigations.** No Go panic and no stack trace was ever recorded for any
investigated death. Every SIGKILL victim was `git`, `node`, or the agent host process — never a
domain-check fault. This null result is the investigation's most consequential finding: it means
crash-response effort belongs on infrastructure and workflow, not on this codebase (§9, R5).

---

## 4. Timeline of Events

All times UTC. Provenance per §8: **[LIVE]** = re-derived from surviving primary sources;
**[COMMIT]** = attested by a committed investigation document whose primary sources have rotated;
**[REPORTED]** = anchor-report volumetric, not reproducible today.

| Date | Event | Provenance |
|------|-------|------------|
| **2026-08-12** | **bf-1s6c3 — origin event.** Repo at 18 GB / 17.16 GB loose objects; routine git operations drove `git` past the 12 GiB dispatch scope → memcg OOM SIGKILL, exit −1. Cleanup 18 GB → 138 MB (−99.2%); the task completed afterward. | [COMMIT] c4d2b29, a7b1347, 2f1a9b2, 76a9c1d |
| **2026-08-13** | **bf-1ea4g false-positive cycle begins.** Original crash assessed false positive (work completed before the kill); 9+ duplicate investigation beads follow. | [COMMIT] 383241f |
| **2026-08-14** | **bf-173o7e storm — largest single event.** Duplicate gc bead dispatched 132×; **129 attempts die `exit -1` over ~10.5 h** on bare `git gc --aggressive --prune=now`. Flat kill durations prove the object set never shrank between attempts. Real packing completed Aug-14 23:25 → Aug-17 by a non-attempt process. Kernel- and needle-side records for this day have since rotated. | [COMMIT] 07ab240, db1acb3, d283576, 5d501a8, 227a15c, 6b4aa4c; raw sources gone |
| **2026-08-16** | **Aug-16 storm — the fully re-verifiable day.** **414 kernel memcg kills in 04:27:35Z → 17:40:32Z** (13.2 h, two waves: 04–07Z = 129, 12–17Z = 332). Victims: **257 `git`, 156 `node` (vitest), 1 other**; largest `git` anon-rss 12,555,188 kB ≈ 12.0 GiB — exactly at the scope bound. Every kill `CONSTRAINT_MEMCG` inside a dispatch scope. Needle side: **461** crash outcomes for this worker, with `exit_code=-1` recorded **157** times. | [LIVE] journald + needle log |
| **2026-08-17** | **Signal deaths end.** 3 crash outcomes, **1** `exit_code=-1` — the last at **16:00:26.884570Z** (last crash outcome 16:00:27.014301Z). 64 "kubeconfig not found" close-verify failures across 34 beads are **correct behavior** — the file genuinely did not exist until 2026-08-25. | [LIVE] needle log; [COMMIT] 5a0f127 |
| **2026-08-17 → 08-25** | **Alerts outlive their causes.** 200+ alerts accumulate, ~60% duplicates. Post-completion kills and self-healed retries both alert. | [COMMIT] 383241f |
| **2026-08-25** | `iad-ci.kubeconfig` created (birth == mtime); wrong-workspace close-verify wrapper retired. Retroactively explains the Aug-17 failures. | [COMMIT] 5a0f127 |
| **2026-08-26** | **Last fleet-wide signal deaths: 18 workers** — needle defect L3: crash handler's `bead release` exits 4 against an already-closed bead, unhandled error kills the worker. | [COMMIT] eba6c2a |
| **2026-08-26 → 09-06** | **Zero `exit_code=-1` fleet-wide** — 19 consecutive days at the time of writing. Failure class shifts to synchronized `exit_code=1` waves (13–15 workers/min, cross-workspace) = service class. Needle OTLP 503s root-caused to the otel-collector CrashLoopBackOff. Gateway health-check false alarm identified (curl 60 vs `200 ok`). | [COMMIT] f9af254 + [LIVE 09-06] |
| **2026-09-01** | **Anchor final report committed** (383241f, bead domchk-20dc36b4, closed). **Auto-splits proliferate:** sibling chain created 19:38, this chain 20:09 — three parallel syntheses of a concluded investigation. | [LIVE] bead checkpoint |
| **2026-09-02** | **All 15 kernel memcg kills since Aug-17 happen on this day** — 14 `bash` + 1 `python3`, in synthetic test scopes (`mw-oomdbg`, probe-hog) plus the bounded safe-git-gc run firing as designed; the gc completed 07:55 leaving 1 pack / 90.19 MiB. GC memory bounds proven in production. | [LIVE] journald; [COMMIT] f9af254, 533cb46 |
| **2026-09-05** | **Verification day.** Repo integrity proven end-to-end (7e57bb8: `fsck --full` and `--strict` exit 0, 0 dangling / 0 orphaned / 0 garbage). Corpus validated against primary evidence (73176de). Findings compilation committed (5b4be43). A wholly fabricated dispatch premise documented (6369467). Close-verify kubeconfig RCA (5a0f127). | [COMMIT] |
| **2026-09-06** | **The draft** (domchk-d6871df1, 9338e2b). All figures re-derived: repo 92 MB `.git`, 1 pack / 90.34 MiB, 25 loose / 220 KiB, 0 garbage; 15 memcg kills since Aug-17 all on Sep-02; last `exit_code=-1` 2026-08-17T16:00:26Z; host 45 G mem available / 87 G disk / load 0.41; gateway `-sf` exit 60 vs `-skf` `ok`. | [LIVE 09-06] |
| **2026-09-06 (23:07 UTC)** | **This final.** Every `[LIVE]` figure re-derived a second time and reproduced exactly (§11): same 15 memcg kills (14 `bash` + 1 `python3`, all Sep-02), same last `exit_code=-1`, same 1 pack / 90.34 MiB / 0 garbage; loose objects drifted 25 → 32 (220 → 292 KiB) from ordinary fleet work — immaterial; host 44 G mem available, load 1.9/1.0/0.9 (fleet active). | [LIVE 09-06] |

---

## 5. Additions Since the Anchor Report (2026-09-01 → 2026-09-06)

The anchor report closed the investigation. Everything below happened *after* it and is not reflected
in it:

1. **The L1 fix landed and was proven in production** — 533cb46 (bare-path git config bounds) and
   f9af254 (the 2026-09-02 bounded run completing cleanly under the 768 MiB cgroup test).
2. **Repository integrity proven end-to-end** — 7e57bb8: `fsck --full` and `--strict` both exit 0
   with zero output; all 455 refs and 10,636 ref-reachable objects resolve; 17 non-ref-reachable
   objects all covered by reflogs; repo already optimal.
3. **The corpus itself was validated** — 73176de traced three Sep-1 documents against the preserved
   bf-173o7e trace; headline findings stand, five mechanism details corrected (§7).
4. **The Aug-17 "kubeconfig not found" wave resolved as correct behavior** — 5a0f127.
5. **A fabricated dispatch premise was documented** — 6369467.
6. **Three parallel synthesis chains were created** for the already-concluded investigation, including
   this one — the auto-split loop operating on a closed question (§10, gaps 2/5).

---

## 6. Impact Assessment

| Dimension | Finding | Severity | Provenance |
|-----------|---------|----------|------------|
| **Domain-check code** | Zero defects found in any investigation | None — null result | [COMMIT] corpus-wide |
| **Data loss** | Zero. Repo integrity proven: `fsck --full`/`--strict` clean, 0 dangling, 0 orphaned, 0 garbage | None | [COMMIT] 7e57bb8 + [LIVE 09-06] |
| **Repository** | Healthy and optimal: 92 MB `.git`, 1 pack / 90.34 MiB, 32 loose / 292 KiB, 0 garbage; gc bounds holding | None — within the <500 MB healthy band | [LIVE 09-06] |
| **Fleet signal deaths** | Zero `exit_code=-1` since 2026-08-26 (19 days); zero real kernel memcg kills since 2026-08-17 (all 15 since are 2026-09-02 synthetic/bounded-scope kills) | None currently | [LIVE 09-06] |
| **Host resources** | 44–45 G mem available / 62 G, 87 G disk free, load < 2 — all far inside documented safe limits | None | [LIVE 09-06] |
| **Availability** | Service degraded during the Aug-14 and Aug-16 storms (repeated agent deaths); no incorrect availability data was ever served — RDAP results were unaffected | Moderate, historical | [COMMIT] |
| **Upstream NEEDLE** | L2 and L3 remain unremediated upstream: no completion detection, no dedup, and the release-conflict kill (18 deaths) | **High** — remains a live kill/noise source | [COMMIT] eba6c2a, 383241f |
| **Wasted effort** | **The dominant ongoing impact.** 200+ alerts (~60% duplicates), 157+ verification reports, ~110 verification-doc files across eight directory naming schemes, and at least **three parallel synthesis chains** for one concluded investigation | **High — ongoing** | [COMMIT] + [LIVE] catalogue |
| **Misdiagnosis risk** | The anchor report's opening still states the superseded mechanism; any reader starting there quotes "SIGHUP cascade / 94.71% oomd" | Medium — mitigated by the correction banner added to its header, which now points here | [COMMIT] 383241f |

**Severity summary.** Historical severity was **high** (sustained agent loss on Aug-14/Aug-16,
18 deaths on Aug-26) but is now **low** for the workload itself. Remaining severity is concentrated
in *organizational* impact: unremediated upstream defects and an alert pipeline that keeps
re-investigating closed questions.

---

## 7. Superseded Claims — Do Not Propagate

> **Canonical pointer list (adopted 2026-09-06, §9 R10 — closes §10 gap 9).** This section is the
> corpus's single authoritative list of claims that are known-wrong but still reachable from
> committed documents. Any new corpus document must either cite it or extend it; none may restate a
> claim below as fact. The root cause of the anchor's stale mechanism was that no such list existed.

These appear in committed corpus documents. Each is corrected below; none should be repeated in any
future report.

| Superseded claim | Corrected to | Correction source |
|------------------|--------------|-------------------|
| "SIGHUP cascade" killed workers | Kernel **memcg-OOM SIGKILL**; `exit -1` is the unrecorded-signal sentinel — SIGHUP is catchable and would have been recorded as signal 1 | 5d501a8; sibling evidence compilation §8 |
| Aug-16 window "12:00–17:00 (5 h), 201+ crashes" | **04:27:35Z–17:40:32Z (13.2 h), two waves; 461 crash outcomes for this worker** | [LIVE] journal + needle log |
| "systemd-oomd Memory Pressure: 94.71%" | **Not a surviving record.** The only `94.71` strings in journald are Tailscale-IP substrings in socat lines. Verified mechanism: kernel memcg OOM at the 12 GiB scope bound | [LIVE] |
| "System-wide OOM" | **Cgroup-scoped OOM** — every kill `CONSTRAINT_MEMCG` in a dispatch scope; the host was not out of memory, so host-wide alerting could never have caught it | [LIVE] |
| "bf-4x12ec attempt 53 (12:58:45Z) completed the gc" | That run was the **`template=split` auto-split** executing the bead mitosis; real packing completed Aug-14 23:25 → Aug-17 by a non-attempt process | d283576 |
| "18 GB → 445 MB (97.5% reduction) in that attempt" | The 444.24 MiB pack was **already present** pre-gc; that attempt shrank nothing | 73176de |
| "Crash came 4 hours after completion" | UTC/EDT mixup — real gap **~90 s** (gc 17:00:14Z, first close attempt 17:06:02Z, death 17:06:59Z) | 73176de |
| "No clear error message" | The failure was **explicit** (close script resolution error) | 73176de |
| bf-173o7e has an unresolved "verification blocker" needing a script fix / `--skip-verify` bypass | **The blocker does not exist**; `verify-work-completion.sh` has no cluster logic and `bead close` has no such flag | 6369467 |
| Aug-17 close-verify "kubeconfig not found" failures = tooling bug | **Correct behavior** — the file genuinely did not exist until Aug-25 | 5a0f127 |

---

## 8. Evidence Provenance and Limitations

Every claim in §3–§6 carries one of three tags. The distinction is not cosmetic — it bounds what can
ever be re-verified again.

- **[LIVE]** — re-derived from surviving primary sources. Journald retains from 2026-08-15T23:46:33Z;
  the needle primary log covers 2026-08-16T03:09:46Z → 2026-09-05T09:06:18Z; repo state, git config,
  and the bead checkpoint are current. **Every event from 2026-08-16 onward is in this class.**
- **[COMMIT]** — attested by a committed investigation document whose primary sources have rotated.
  The kernel and needle logs for 2026-08-12/13/14 are gone and reflogs were truncated to Sep-1.
  **These claims cannot be re-verified again; the committed record is final.**
- **[REPORTED]** — anchor-report volumetrics that are not reproducible from any surviving source:
  "201+ crashes", "826 crashes (worst day)", "16+ consecutive days stable", "94.71% pressure". Quote
  only as attested figures, never as live facts.

**Limitations.**

1. **Aug-12/13/14 are permanently attested-only.** This report labels them as such rather than citing
   those numbers as if re-derived.
2. **Scope is single-workspace.** Fleet-wide figures (the `exit_code=1` waves, the 18 deaths on
   Aug-26) come from needle fleet logs, not this repo's corpus; the cross-workspace conclusions rest
   on f9af254 alone (§9, R12).
3. **bf-31mno — the largest single kill storm (434 kills) — has no dedicated RCA.** It is mentioned
   only inside bf-4yjq documents (§9, R9).
4. **Timing caveat on alert timestamps.** Needle alert timestamps are *handling* heartbeats
   (8–120 s after the real kill), not death times — do not use them to reconstruct event order.

---

## 9. Recommendations

Prioritized by harm reduction (§6), and — because the corpus's most expensive mistake was applying
repo-side fixes to upstream problems and vice versa — each carries an **owner**. "Verified absent"
means checked in this repo on 2026-09-06 *before* being recommended here, so none of this duplicates
work that already exists. Nothing in this section has been implemented as part of this step: this
bead's deliverable is the recommendation set, and the repo's remaining housekeeping belongs to step 4
(§10).

### P0 — Upstream NEEDLE (largest remaining harm; §10 gap 5)

**R1. Add work-completion detection to crash alerting.** Before raising a crash alert, needle should
ask whether the work actually landed: the bead-closed event, plus the pre-close verification record
`.beads/state/work-completion/<bead-id>.json` that `scripts/verify-work-completion.sh` already writes
repo-side. A post-completion kill (bf-5tgsk: completed 16:35:54, killed 16:36:24, bead closed
16:36:51 — all work landed) must not alert as a crash. *Completion awareness exists repo-side in
`scripts/crash-alert-manager.sh`; the alert source is upstream and has none.*

**R2. Deduplicate alerts per underlying event.** One event → one alert, with a cooldown. bf-173o7e
drew **129 duplicate alerts** and ~60% of the 200+ total were duplicates. Repo-side equivalents exist
(`scripts/alert-deduplication.sh`, 5-minute cooldown in `scripts/crash-alert-manager.sh`); the
duplication originates upstream, so the repo scripts can only absorb it after the fact.

**R3. Fix the L3 release-conflict kill.** The crash handler treats `bead release` exit 4 (bead
already closed) as fatal and kills the whole worker — **18 deaths on 2026-08-26**, the last
fleet-wide signal-death day. Exit 4 on a release against a closed bead means "nothing to release",
not "handler error". This is the only remaining *kill source* in the corpus and it lives in needle,
not in any workspace.

**R4. Validate auto-split premises against live bead state before dispatching.** The auto-splitter
re-dispatches onto already-resolved beads (bf-4ifshb; bf-1cd5v6 closed 3×) and at least once
dispatched a **wholly fabricated premise** — a bf-173o7e "verification blocker" requiring a script
fix and a `--skip-verify` flag, neither of which exists (6369467). Minimum rule: refuse a split whose
target bead is already closed, and refuse a premise citing a file, flag, or behavior that does not
exist. This loop produced the chain that produced this report.

### P0 — Decision, not work

**R5. Stop re-synthesizing a concluded investigation.** Three parallel chains re-synthesized the same
closed question after the anchor report (2026-09-01), this chain included. Before writing any corpus
document: `git log --grep <bead-id>`, search `docs/investigations/`, and check the target bead's
status. If the question is closed, close the bead with a pointer to the existing report instead of a
new document. This is the dominant ongoing impact (§6) and costs discipline, not effort.

### P1 — This repo (small, verified gaps)

**R6. Make the L1 bound observable in the health checks.** `scripts/setup-git-gc-config.sh --verify`
already resolves the *effective* bound (system → global → local, the chain a bare gc actually sees)
and exits non-zero if the bare-gc path is unbounded or threads are unpinned — but no monitoring
script calls it. **Verified absent:** neither `scripts/check-repo-health.sh` nor
`scripts/preflight-health-check.sh` invokes it, and `scripts/repo-health-check.sh:81` only prints a
suggestion to run setup. Wire the verify call into both the health check and the pre-flight, so a
config regression becomes an alert rather than a rediscovery. The bound is today *enforced* (all
three keys present, `[LIVE 09-06]`) but *unobserved*.

**R7. Add cgroup-scoped OOM detection to resource monitoring.** `scripts/resource-monitor.sh` watches
`MemAvailable` and PSI memory pressure — **host-wide signals only**. Every kill in this corpus was
`CONSTRAINT_MEMCG` *inside* a dispatch scope with the host nowhere near exhaustion; that is exactly
the blind spot that produced the original SIGHUP misdiagnosis and let the 414 kills of 2026-08-16
happen under a green dashboard. Watch `memory.events` `oom_kill` for the dispatch scopes (or count
`Memory cgroup out of memory` journald lines) and alert on a non-zero delta. **This is the
highest-value detection fix in this report** — it converts the corpus's founding blind spot into a
signal.

### P1 — Docs housekeeping (§10 gaps 2–3)

**R8. Commit the corpus's untracked evidence.** The sibling chain's artifact
(`docs/investigations/evidence-compilation-2026-09-01-crash-investigation.md`) — the corpus's best
provenance-tagged source — is still untracked with **three beads blocked behind it**
(domchk-59e1f1d5, open since 2026-09-01), alongside **23 further untracked investigation documents**,
including the only copies of the systemic RCA and the exit-code −1 signal analysis. Commit them,
close domchk-59e1f1d5, cascade-close its dependents. Uncommitted evidence is precisely how gap 1
happened.

### P2

**R9. bf-31mno — cross-reference it, do not open a chain.** The largest single kill storm (434
kills) has no dedicated RCA (§8, limitation 3). Before opening one, check whether its primary
evidence survived log rotation. If it did not — as with Aug-12/13/14 — record a [COMMIT]-class
cross-reference into the bf-173o7e record and stop; an investigation that can only re-attest rotated
sources is the waste this report exists to end.

**R10. Codify the provenance convention.** §7 is hereby the corpus's canonical superseded-claims
pointer list, and the [LIVE]/[COMMIT]/[REPORTED] tags (§8) are the required provenance for any new
corpus report. Every superseded claim in §7 remains reachable from committed documents; the pointer
list and the tags are what stop the next report from quoting "94.71% oomd" as a fact.

**R11. Keep the classification-first rule and the timing caveat.** Classify before investigating —
`exit_code=-1` = infrastructure; synchronized `exit_code=1` waves = service class; consult the
current fleet signature first — and never reconstruct event order from needle alert timestamps,
which are handling heartbeats 8–120 s after the kill (§8, limitation 4).

### P3 — Optional

**R12. Cross-workspace validation.** The fleet-wide figures (the `exit_code=1` waves, the 18 deaths
of Aug-26) rest on f9af254 alone (§8, limitation 2). One pass over another workspace's needle log
would confirm or bound them. Worth doing only if another fleet-level investigation is already
running; not worth opening a chain for.

### Anti-recommendations — do not do these

- **Do not** write a fourth synthesis of this corpus. Extend or correct this one.
- **Do not** run bare `git gc --aggressive`, and do not "fix" the memory bounds by raising them —
  the bound firing on 2026-09-02 *was* the fix working.
- **Do not** read `exit_code=-1` as a signal number, or as SIGHUP (§3 L1, §7).
- **Do not** add host-wide memory alerting as an OOM detector — it structurally cannot see a
  cgroup-scoped kill (R7).
- **Do not** quote [REPORTED] volumetrics ("201+ crashes", "826 crashes", "94.71%") as live facts.
- **Do not** add further repo-side alert scripts to compensate for upstream defects. The repo-side
  equivalents already exist (R1–R4); the defects are upstream, and more wrappers add the exact
  duplicate-artifact burden §6 measures.

---

## 10. Open Gaps and Handoff to domchk-0d3c11c9

The gap table from the draft, with statuses updated by this step. "Step 4" is domchk-0d3c11c9
(commit the report, update parent bead bf-4829x8 with the report reference, link related code/docs).

| # | Gap | Fix | Status after this step |
|---|-----|-----|------------------------|
| 1 | Anchor report leads with a superseded mechanism | Correction banner in its header | **Done with the draft** (banner in place; its pointer updated to this final) |
| 2 | Sibling chain stalled with its artifact uncommitted (domchk-59e1f1d5 open since 2026-09-01; three beads blocked) | Commit `evidence-compilation-2026-09-01-crash-investigation.md`, close 59e1f1d5, cascade-close | Open — **step 4, with R8** |
| 3 | 23 investigation documents untracked, incl. the only copies of the systemic RCA and the exit-code−1 signal analysis | Housekeeping commit | Open — **step 4, with R8** |
| 4 | bf-31mno (434 kills, largest single storm) has no dedicated RCA | Cross-reference only if evidence rotated (R9) | Open — P2, do **not** open a chain for it |
| 5 | **Upstream NEEDLE fixes unverified** — no completion detection, no dedup, L3 release-conflict kill unfixed, auto-split loop unremediated | R1–R4 | **Open upstream — the primary recommendation target** |
| 6 | CLAUDE.md documents the false-alarming `curl -sf` gateway health check | One-line change to `-skf` | **Done with the draft** |
| 7 | Aug-12/13/14 claims permanently attested-only | Label explicitly | Closed by the draft |
| 8 | Single-workspace evidence scope | Cross-check against another workspace's needle log | Optional — R12 (P3) |
| 9 | No corpus index / superseded-pointer convention (the root cause of gap 1) | Adopt §7 as canonical + the provenance tags (R10) | **Closed by this step** |

**Step 4 checklist (domchk-0d3c11c9).**

1. Commit this final report together with the sibling chain's untracked artifact (R8) and the other
   untracked investigation documents, in a commit whose message names this report and its bead so
   `git log --grep domchk-e843c4f1` finds it.
2. Update parent bead **bf-4829x8** with the report reference (path + commit), per its objective.
3. Link from the anchor's correction banner (already points here) and, if a corpus index exists by
   then, add this report to it.
4. Do **not** implement R6/R7 as part of step 4 — they are recommendations for a follow-up bead, not
   report-linking work. Record them against the umbrella so they survive the chain.

---

## 11. Live Re-Derivation

### 11.1 Draft derivation — 2026-09-06 02:25 UTC (domchk-d6871df1)

Commands behind every `[LIVE 09-06]` figure first derived in the draft:

```bash
cd /home/coding/domain-check

# Repository health
du -sh .git                                   # 92M
git count-objects -vH                         # 25 loose / 220 KiB; 1 pack 90.34 MiB; 0 garbage

# GC memory bounds (effective, repo-local)
git config --get pack.windowMemory            # 2g
git config --get pack.deltaCacheSize          # 1g
git config --get pack.threads                 # 1

# Kernel memcg kills after the Aug-16 storm (all synthetic/bounded, all Sep-02)
journalctl --no-pager --since "2026-08-17 00:00" | grep -c "Memory cgroup out of memory"   # 15
journalctl --no-pager --since "2026-08-17 00:00" | grep "Memory cgroup out of memory" \
  | awk '{print $1}' | sort | uniq -c                                                       # 15 → Sep
journalctl --no-pager --since "2026-08-17 00:00" | grep -oP 'Killed process \d+ \(\K[^)]+' \
  | sort | uniq -c                                                                          # 14 bash, 1 python3

# Signal deaths in this worker's needle log
LOG=~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log
grep 'needle.outcome="crash"' $LOG | cut -c1-10 | sort | uniq -c      # 461 Aug-16, 3 Aug-17
grep 'exit_code=-1' $LOG | awk '{print substr($1,1,10)}' | sort | uniq -c   # 157 Aug-16, 1 Aug-17
grep 'exit_code=-1' $LOG | tail -1 | cut -c1-33                       # 2026-08-17T16:00:26.884570Z

# Host resources
free -g; df -BG --output=avail / | tail -1; uptime    # 45G avail; 87G free; load 0.41

# Gateway health-check false alarm (gap 6)
curl -sf  --max-time 5 https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health; echo "exit=$?"  # 60
curl -skf --max-time 5 https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health; echo "exit=$?"  # ok, 0
```

### 11.2 Final re-derivation — 2026-09-06 23:07 UTC (domchk-e843c4f1)

Every command above re-run. **All figures reproduced exactly** except the three that move with
ordinary fleet activity, as expected:

| Figure | Draft (02:25) | Final (23:07) | Reading |
|--------|---------------|---------------|---------|
| Memcg kills since Aug-17 | 15, all Sep-02, 14 `bash` + 1 `python3` | **identical** | No new kill in ~21 h |
| Last `exit_code=-1` | 2026-08-17T16:00:26.884570Z | **identical** | Still zero since Aug-17 |
| Crash outcomes by day | 461 Aug-16 / 3 Aug-17 | **identical** | No new crash outcomes |
| Pack | 1 pack / 90.34 MiB / 0 garbage | **identical** | Repo unchanged at the pack level |
| Loose objects | 25 / 220 KiB | **32 / 292 KiB** | Ordinary fleet commits; immaterial vs. the 100 MB warning band |
| GC memory bounds | `2g` / `1g` / `1` | **identical** | L1 fix still in place |
| Mem available | 45 G | **44 G** | Fleet active; far inside the 20 G minimum |
| Load (1/5/15 min) | 0.41 | **1.87 / 1.02 / 0.94** | Fleet active; inside the < 5 safe limit |
| Gateway `-sf` / `-skf` | exit 60 / `ok` exit 0 | **identical** | Doc fix (§3 L4) still correct |

Two notes for whoever re-derives next:

- The draft's needle figures come from the **glm-4_7** worker's rolling log
  (`~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log`, 90 MB). The current worker
  generation (`glm-5.3-flash`) writes **per-dispatch `.jsonl` files** instead of a rolling log, so
  the same greps do not carry forward unchanged — the rolling-log queries need an equivalent against
  the jsonl set (or whatever needle's logging converges to) before the next derivation.
- `git count-objects -vH` loose counts will keep drifting with fleet commits. Compare against the
  **band** (CLAUDE.md: <100 objects / <100 MB healthy), not against the exact number recorded here.

---

*Final — domchk-e843c4f1. Baseline 383241f; input findings-compilation 5b4be43; draft 9338e2b.
Downstream: domchk-0d3c11c9 (commit + link, §10 checklist, with R8). Upstream asks: R1–R4.*
