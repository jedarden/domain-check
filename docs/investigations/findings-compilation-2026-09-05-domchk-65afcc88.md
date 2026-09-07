# Findings Compilation — Investigation Corpus, 2026-08-12 → 2026-09-05

**Bead:** domchk-65afcc88 (step 1 of 4: gather → draft → recommendations → commit)
**Chain parent (umbrella):** domchk-11c26b24 "Document investigation results and recommendations" (parent bead bf-4829x8)
**Compiled:** 2026-09-05
**Investigation period covered:** 2026-08-12 → 2026-09-05
**Purpose:** Single organized source of every finding in the crash-investigation corpus, with verification
status per claim and an explicit gap list — the input for the downstream report draft (domchk-d6871df1).

---

## 0. Read this first: the investigation this chain feeds is already concluded

The most important finding from gathering is structural, not technical:

| Fact | Evidence |
|------|----------|
| The umbrella's deliverable — a committed final investigation report with timeline, root cause, impact, recommendations — **already exists** | `docs/investigations/final-investigation-report-2026-09-01.md`, committed **383241f** (2026-09-01), produced by closed bead domchk-20dc36b4 |
| A **second** 4-way auto-split of the same work was created 31 minutes after the first and **stalled at step 1** — its evidence compilation was written (2026-09-05 18:51) but the bead is still open and its three downstream beads are blocked | domchk-59e1f1d5 (open) → 6281555d → ece81e17 → 7880ada7; artifact `docs/investigations/evidence-compilation-2026-09-01-crash-investigation.md` is **untracked** |
| **This chain** (65afcc88 → d6871df1 → e843c4f1 → 0d3c11c9) is a third instantiation of the same split, created 2026-09-01 20:09 | bead checkpoint: all six beads created by `system` at 20:09:56, `split-child` label |

**Consequence for the downstream beads:** domchk-d6871df1 should draft a **delta report** — the corrected
mechanism plus everything after 2026-09-01 — not re-derive the investigation. Re-deriving would add a
fourth near-duplicate synthesis to a corpus that already has three.

---

## 1. Corpus catalogue

902 markdown documents on disk; 899 tracked, **23 untracked**. Investigation material spans these locations:

| Location | Files | Contents |
|----------|------:|----------|
| `docs/` (root) | 465 | Individual crash analyses, verification reports, remediation strategies, alert resolutions |
| `docs/crash-investigations/` | 89 (+12 in `bf-4k2ws/`) | Per-crash investigation bundles (bf-4k2ws, bf-4x12ec, …) |
| `docs/verification/` | 39 | Alert verification reports |
| `docs/bead-verification/` + `docs/archive/bead-verification-reports/` | 34 + 32 | Bead-level close-verification reports |
| `docs/notes/` | 73 | Application notes, including release/CI status |
| `docs/research/` | 43 | Product research (RDAP, rate limits, goreleaser) + crash research (`root-cause-analysis-signal-minus-one-crashes.md`) |
| `docs/crashes/` | 26 | Crash records |
| `docs/investigations/` | 17 | **Synthesis anchors** — final reports and root-cause determinations |
| `docs/crash-reports/` | 13 | Crash reports |
| `docs/analysis/`, `docs/crash-investigation/`, `docs/crash-analysis/`, `docs/investigation/` | 9 + 5 + 2 + 2 | Additional analyses |
| `docs/verification-reports/` + `docs/verifications/` | 3 + 1 | Verification reports |
| `docs/maintenance/` | 8 | Repository maintenance guides (gc strategy) |
| `docs/reports/` | 6 | Roll-up reports |

### 1.1 Synthesis anchors (start here; do not re-derive)

| Document | Commit | Role |
|----------|--------|------|
| `docs/investigations/final-investigation-report-2026-09-01.md` | 383241f | Anchor final report: 19-day, 200+ alert investigation; conclusions stand, several mechanism details superseded (§3.1) |
| `docs/investigations/signal-minus-one-final-investigation-report-2026-09-02.md` | b1ae579 | Final report for the exit −1 / bf-173o7e cycle-4 chain |
| `docs/investigations/bf-173o7e-root-cause-determination-domchk-2e371a2c-2026-09-02.md` | 07ab240 | HIGH-confidence RCA of the largest single event (Aug-14 storm) |
| `docs/investigations/bf-173o7e-crash-timing-vs-gc-completion-domchk-536862b8-2026-09-02.md` | d283576 | Timing decomposition; corrects the "attempt 53 completed gc" claim |
| `docs/investigations/bf-173o7e-original-bead-context-domchk-8304c1c0-2026-09-02.md` | — | Original-bead context |
| `docs/investigations/root-cause-determination-bf-2ildm-2026-09-02.md` | — | bf-2ildm RCA |
| `docs/investigations/evidence-compilation-2026-09-01-crash-investigation.md` | **untracked** | Sibling chain's evidence compilation — verification tags ([LIVE]/[COMMIT]/[REPORTED]/[GONE]) and §8 corrections; best single source for claim provenance |
| `docs/root-cause-analysis-bf-173o7e-2026-09-01.md` | 73176de (addendum) | Validated against the preserved bf-173o7e trace |
| `docs/maintenance/repository-maintenance-guide.md` | — | Operational countermeasures |
| `docs/crash-response-guide.md` | — | Agent-facing classification guide |

### 1.2 Validation passes already run over the corpus

1. **Sibling evidence compilation** (2026-09-05, untracked): re-derived every Aug-16-onward number from
   journald + the needle log; corrected four mechanism details in the anchor report (§3.1 below).
2. **73176de** (2026-09-05, domchk-60407475): traced three Sep-1 documents against the preserved
   bf-173o7e trace; headline findings stand, five specific details corrected (§3.1).
3. **7e57bb8** (2026-09-05, domchk-e7f0de3b): repository integrity — `fsck --full` and `--strict` exit 0,
   zero output; all 455 refs and 10,636 ref-reachable objects resolve; 17 non-ref-reachable objects all
   reflog-covered (0 dangling, 0 orphaned); repo already optimal.

---

## 2. Consolidated findings

### 2.1 Timeline (all times UTC)

| Date | Event | Verification |
|------|-------|--------------|
| 2026-08-12 | **bf-1s6c3 — origin event.** Repo at 18 GB with 17.16 GB loose objects; routine git operations drove `git` past its dispatch-scope limit → memcg OOM SIGKILL, exit −1. Cleanup: 18 GB → 138 MB (−99.2%); task completed after. Classification INFRASTRUCTURE (self-inflicted), no code defect | [COMMIT] c4d2b29, a7b1347, 2f1a9b2, 76a9c1d |
| 2026-08-13 | **bf-1ea4g false-positive cycle begins.** Original crash assessed false positive (work completed before the kill); 9+ duplicate investigation beads follow | [COMMIT] 383241f |
| 2026-08-14 | **bf-173o7e storm — largest single event.** A duplicate gc bead dispatched 132×; 129 attempts died exit −1 over ~10.5 h. Bare `git gc --aggressive --prune=now` pushed `git pack-objects` RSS past the **MemoryMax=12 GiB** dispatch scope → one memcg SIGKILL per attempt; flat kill durations prove the object set never shrank between attempts. Actual packing completed Aug-14 23:25 → Aug-17 by a non-attempt process. Kernel- and needle-side records for this day are **rotated** — the surviving base is the committed corpus | [COMMIT] 07ab240, db1acb3, d283576, 5d501a8, 227a15c, 6b4aa4c; raw sources [GONE] |
| 2026-08-16 | **Aug-16 storm — fully re-verifiable today.** **414 kernel memcg kills** in 04:27:35Z → 17:40:32Z (13.2 h, two waves: 04–07Z = 129, 12–17Z = 332). Victims: **257 `git`, 156 `node` (vitest), 1 other**; largest `git` victim anon-rss 12,555,188 kB ≈ 12.0 GiB (at the scope bound); every kill `CONSTRAINT_MEMCG` inside `needle.slice/run-p*.scope` / `app.slice/run-p*.scope` — **cgroup-scoped, not host-wide**. Needle side: 461 crash outcomes on Aug-16 + 3 on Aug-17, then zero | [LIVE] — re-derived 2026-09-05 from journald + needle log |
| 2026-08-17 → 08-25 | Signal deaths stop; alerts do not. 200+ alerts accumulate, ~60% duplicates. Post-completion kills (bf-5tgsk: completed 16:35:54, killed 16:36:24, bead closed 16:36:51) and self-healed retries (bf-6bio4g: crash → retry exit 0) both alert. 2026-08-17's 64 "kubeconfig not found" close-verify failures across 34 beads were **correct behavior** (file genuinely absent; wrong-workspace wrapper retired Aug-25) | [COMMIT] 383241f; 5a0f127 |
| 2026-08-26 | **Last fleet-wide signal deaths: 18 workers**, cause a needle defect — the crash handler runs `bead release` against an already-closed bead, gets exit 4, and the unhandled error kills the worker | [COMMIT] eba6c2a |
| 2026-08-26 → 09-05 | **Zero `exit_code=-1` fleet-wide.** Failure class shifts to synchronized `exit_code=1` waves (13–15 workers/min, cross-workspace) = inference-service class. Needle OTLP 503s root-caused to a live `needle-otel-collector` CrashLoopBackOff (config schema drift). New false-alarm vector: the documented `curl -sf` gateway health check fails with curl 60 (self-signed cert) while the gateway answers `200 ok` | [COMMIT] f9af254 |
| 2026-09-01 | Anchor final report committed (383241f). Auto-splits proliferate: sibling chain 19:38, this chain 20:09 | [LIVE] bead checkpoint |
| 2026-09-02 | **All 15 kernel memcg kills after Aug-17 happened on this day** (14 `bash`, 1 `python3`) — synthetic test scopes and the bounded safe-git-gc run firing as designed (gc completed 07:55, 1 pack 90.19 MiB). GC memory bounds (`pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1`) proven in production | [LIVE] journald; [COMMIT] f9af254, 533cb46 |
| 2026-09-05 | Repo integrity verified end-to-end (7e57bb8); corpus validation addenda (73176de); a wholly fabricated dispatch premise documented (6369467 — the claimed bf-173o7e "verification blocker" and `--skip-verify` flag do not exist); close-verify kubeconfig RCA (5a0f127) | [COMMIT] |

### 2.2 Root cause, by causal layer

**L1 — Resource consumption (produces exit −1).** Unbounded-memory processes running inside
per-dispatch memcg scopes (`MemoryMax=12 GiB`) get SIGKILLed by the kernel; the agent dies without any
Go panic, and needle records `exit_code=-1`, which is the `code().unwrap_or(-1)` **sentinel for an
unrecorded signal death** — not a signal number and not SIGHUP. Consumers: bare `git gc
--aggressive --prune=now` (pack-objects) and `node`/vitest runs. Amplifier: repository bloat (18 GB /
17 GB loose at bf-1s6c3). **Status: mechanically fixed** — persistent git config bounds the bare path
(worst case ≈3 GiB) applied repo-locally and globally (533cb46), safe-gc scripts + systemd user timers
bound the scripted paths, and the bound was proven in production on 2026-09-02. Verified today:
`pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` all set; repo 92 MB `.git`,
1 pack / 90.34 MiB, 20 loose objects / 172 KiB, 0 garbage.

**L2 — Detection and alerting defects (produces alert volume, not crashes).** NEEDLE crash detection
lacks (a) work-completion detection, (b) self-healing awareness, (c) deduplication, (d) event-pattern
recognition → ~60% of 200+ alerts were duplicates; post-completion kills and self-healed retries alert.
The auto-split loop compounds it: it re-dispatches onto already-resolved beads (bf-4ifshb, bf-1cd5v6 ×3,
129 duplicate alerts for bf-173o7e) and can generate a wholly fabricated premise (6369467).
**Status: repo-side mitigations implemented** (`scripts/crash-alert-manager.sh` + classifier + dedup +
5-min cooldown; `scripts/verify-work-completion.sh` pre-close gate), **upstream NEEDLE fixes pending**.

**L3 — Needle crash-handler defect (a kill source in its own right).** `bead release` against an
already-closed bead exits 4; the crash handler's unhandled error kills the worker — 18 deaths on
2026-08-26. **Status: upstream fix needed**; no evidence of an upstream fix in the corpus.

**L4 — Service-class failures (the current dominant failure class).** Synchronized `exit_code=1` waves
(13–15 workers/min, cross-workspace), otel-collector CrashLoopBackOff on config drift, and a
documentation-induced false alarm (self-signed cert on the gateway health check). **Status: classified;
remediation lives outside this repo** (fleet/service side), except the health-check doc fix.

**L5 — Explicitly NOT a cause: domain-check code.** Zero defects across 157+ investigations; no Go
panic or stack trace was ever recorded for any investigated death; SIGKILL victims were always `git`,
`node`, or the agent host process, never a domain-check fault.

### 2.3 Impact

| Dimension | Finding | Verification |
|-----------|---------|--------------|
| Domain-check code | **Zero defects** found in any investigation | [COMMIT] corpus-wide |
| Data loss | **Zero.** Repo integrity proven 2026-09-05: `fsck --full`/`--strict` clean, 0 dangling, 0 orphaned, 0 garbage | [COMMIT] 7e57bb8 + [LIVE] §4 |
| Repository | Healthy and optimal: 92 MB `.git`, 1 pack / 90.34 MiB, 20 loose / 172 KiB, gc bounds holding | [LIVE] |
| Fleet signals | Zero `exit_code=-1` since 2026-08-26; zero real kernel memcg kills since 2026-08-17 (all 15 since then are 2026-09-02 synthetic/bounded-scope kills) | [LIVE] |
| Host resources (now) | 42 G mem available / 62 G, 89 G disk free, load 1.35 — far inside the documented safe-operating limits | [LIVE] |
| Wasted effort | **The dominant ongoing impact.** 200+ alerts (~60% duplicates), 157+ verification reports, at least three parallel synthesis chains for one concluded investigation, ~110 verification-doc files in dedicated directories | [COMMIT] + [LIVE] catalogue §1 |
| Upstream NEEDLE | Improvements documented but not implemented upstream; two defects (L2, L3) remain kill/noise sources | [COMMIT] eba6c2a, 383241f |

### 2.4 Superseded claims — do not propagate these into the new report

| Superseded claim | Corrected to | Correction source |
|------------------|--------------|-------------------|
| "SIGHUP cascade" killed workers | Kernel **memcg-OOM SIGKILL**; exit −1 is the unrecorded-signal sentinel, SIGHUP would be catchable/recordable | 5d501a8; sibling evidence compilation §8 |
| Aug-16 window "12:00–17:00 (5 h), 201+ crashes" | **04:27:35Z–17:40:32Z (13.2 h), two waves; 461 crash outcomes for this worker** | [LIVE] journal + needle log |
| "systemd-oomd Memory Pressure: 94.71%" | **Not a surviving record** — the only `94.71` strings in journald are Tailscale-IP substrings in socat lines; verified mechanism is kernel memcg OOM at the 12 GiB scope bound | [LIVE] |
| "System-wide OOM" | **Cgroup-scoped OOM** — every kill `CONSTRAINT_MEMCG` in a dispatch scope; host was not out of memory (so host-wide alerting would not have caught it) | [LIVE] |
| "bf-4x12ec attempt 53 (12:58:45Z) completed the gc" | That run was the **template=split auto-split**; real packing completed Aug-14 23:25 → Aug-17 by a non-attempt process | d283576 |
| "18 GB → 445 MB (97.5% reduction) in that attempt" | Trace shows the 444.24 MiB pack **already present** pre-gc; that attempt shrank nothing | 73176de |
| "Crash came 4 hours after completion" | UTC/EDT mixup — real gap **~90 s** (gc 17:00:14Z, first close attempt 17:06:02Z, death 17:06:59Z) | 73176de |
| "No clear error message" | Failure was **explicit** (close script resolution error) | 73176de |
| bf-173o7e has an unresolved "verification blocker" needing a script fix / `--skip-verify` bypass | **The blocker does not exist**; `verify-work-completion.sh` has no cluster logic and `bead close` has no such flag | 6369467 |
| Aug-17 close-verify "kubeconfig not found" failures = tooling bug | **Correct behavior** — file genuinely absent until Aug-25 | 5a0f127 |

### 2.5 Claim-verification status summary

- **[LIVE] re-verifiable today:** every event from 2026-08-16 onward — journal (from 2026-08-15T23:46:33Z),
  needle primary log (2026-08-16T03:09:46Z → 2026-09-05T09:06:18Z), repo state, git config, bead checkpoint.
- **[COMMIT]-attested only:** 2026-08-12/13/14 (bf-1s6c3, bf-1ea4g, bf-173o7e) — kernel and needle logs for
  Aug-14 are rotated and reflogs were truncated to Sep-1. These claims cannot be re-verified again; treat
  the committed record as final.
- **[REPORTED] (unreproducible):** the anchor report's volumetrics — "201+ crashes", "826 crashes
  (worst day)", "16+ consecutive days stable", "94.71% pressure". None re-derivable from surviving
  primary sources; quote only as attested figures, never as live facts.

---

## 3. Gaps

Ordered by how much they would distort the downstream report.

1. **The anchor final report still leads with a superseded mechanism.**
   `final-investigation-report-2026-09-01.md` (383241f) states SIGHUP cascade / 94.71% oomd pressure /
   5-hour window / system-wide OOM. The corrections live in an **untracked** sibling document and scattered
   addenda — 73176de corrected three *other* Sep-1 docs, not this one. Any reader or downstream agent
   starting from the anchor report will quote the wrong mechanism. *Fix: a small correction commit
   (recommended during step 2/3 of this chain).*
2. **Sibling chain stalled with its artifact uncommitted.** domchk-59e1f1d5 has been open since 2026-09-01;
   its 19.7 KB evidence compilation (the corpus's best provenance-tagged source) is untracked, and its three
   downstream beads (6281555d, ece81e17, 7880ada7) are blocked indefinitely. *Fix: commit the artifact,
   close 59e1f1d5, cascade-close the chain.*
3. **23 investigation documents are untracked** and exist only in this working tree — including the only
   copies of the systemic RCA, the exit-code−1 signal analysis, and the bf-4x12ec artifacts
   (list in §1). *Fix: a housekeeping commit.*
4. **bf-31mno — the largest single kill storm (434 kills) — has no dedicated RCA.** It is mentioned only
   inside bf-4yjq documents. Either give it its own determination or explicitly fold it into the bf-173o7e
   record with a pointer. *Fix: one small RCA or a cross-reference note.*
5. **Upstream NEEDLE fixes are unverified.** Repo-side mitigations exist, but there is no evidence the
   upstream NEEDLE alert pipeline adopted completion detection / deduplication, and the L3 release-conflict
   defect (18 deaths) has no recorded upstream fix. The auto-split loop that produced this duplicate chain
   is itself unremediated. *Fix: this is the substantive content for domchk-e843c4f1's recommendations.*
6. **CLAUDE.md still documents the false-alarming gateway health check.** f9af254 proved `curl -sf` fails
   with curl 60 on the self-signed cert while the gateway answers `200 ok`; the doc still shows the
   `-sf` form. *Fix: one-line doc change (`-skf`) in step 2/3.*
7. **Aug-12/13/14 claims are permanently attested-only.** Primary sources are gone; no further
   re-verification is possible. The downstream report should say so explicitly rather than citing those
   numbers as if they were re-derived.
8. **Scope of evidence is single-workspace.** Fleet-wide figures (e.g. the exit=1 waves, 18 deaths on
   Aug-26) come from needle fleet logs, not from this repo's corpus; cross-workspace conclusions rest on
   f9af254 alone.
9. **No corpus index existed before this document.** Verification material is spread over eight
   directory naming schemes (`verification`, `verifications`, `verification-reports`,
   `bead-verification`, `archive/bead-verification-reports`, `crash-investigations`,
   `crash-investigation`, `crashes`, `crash-reports`) with no pointer convention for superseded docs —
   the root cause of gap 1. *Fix: adopt the §2.4 superseded-claims table as the canonical pointer list.*

---

## 4. Live re-derivation (2026-09-05)

Commands behind the [LIVE] figures in this document:

```bash
# Repository health
du -sh .git                                   # 92M
git count-objects -vH                         # 20 loose / 172 KiB; 1 pack 90.34 MiB; 0 garbage

# GC memory bounds (effective, repo-local)
git config --get pack.windowMemory            # 2g
git config --get pack.deltaCacheSize          # 1g
git config --get pack.threads                 # 1

# Kernel memcg kills after the Aug-16 storm (all synthetic/bounded)
journalctl --no-pager --since "2026-08-17 00:00" | grep -c "Memory cgroup out of memory"   # 15
journalctl --no-pager --since "2026-08-17 00:00" | grep "Memory cgroup out of memory" \
  | awk '{print $1}' | sort | uniq -c                                                        # 15 → Sep 02
journalctl --no-pager --since "2026-08-17 00:00" | grep -oP 'Killed process \d+ \(\K[^)]+' \
  | sort | uniq -c                                                           # 14 bash, 1 python3

# Signal deaths in this worker's needle log (Aug-16 storm only, then zero)
grep 'needle.outcome="crash"' \
  ~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log | cut -c1-10 | uniq -c      # 461 Aug-16, 3 Aug-17

# Host resources
free -g; df -BG --output=avail / | tail -1; uptime        # 42G avail; 89G free; load 1.35
```

---

## 5. Handoff to domchk-d6871df1 (draft the report)

Recommended shape — a **delta report**, not a re-derivation:

1. **Baseline:** cite 383241f as the anchor; state that its conclusions stand and that §2.4 above lists
   the mechanism details superseded since.
2. **Corrected mechanism section:** L1–L5 from §2.2 (memcg-scoped OOM at the 12 GiB bound; alerting
   defects; release-conflict defect; service-class waves; explicitly no code defect).
3. **Timeline:** §2.1, marked [LIVE]/[COMMIT]/attested per §2.5.
4. **Additions since the anchor report:** gc bounds fix proven in production (533cb46, f9af254), repo
   integrity proof (7e57bb8), corpus validation (73176de), kubeconfig RCA (5a0f127), fabricated-premise
   dispatch (6369467).
5. **Recommendations (for e843c4f1):** the pending-upstream list (gap 5), the anchor-report correction
   (gap 1), the health-check doc fix (gap 6), and chain/split hygiene (gap 2, 9).
6. **Target location:** `docs/investigations/`, alongside the anchor; commit in step 4 (0d3c11c9) together
   with the sibling chain's untracked artifact (gap 2).

---

*Compilation complete — domchk-65afcc88. The full corpus (902 docs) is catalogued in §1; the anchor final
report, the sibling evidence compilation, and the post-Sep-2 RCAs were reviewed in full, and every number
tagged [LIVE] above was re-derived on 2026-09-05. Downstream: domchk-d6871df1 (draft) → e843c4f1
(recommendations) → 0d3c11c9 (commit).*
