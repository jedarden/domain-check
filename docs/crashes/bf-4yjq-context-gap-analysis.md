# bf-4yjq crash context — gap analysis and documentation-completeness verdict (2026-09-06)

**Dispatch:** domchk-9d9a0294 ("Document crash context gaps for bead bf-4yjq")
**Subject bead:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo mirror has
diverged/gone stale" (P2, **closed** 2026-08-17T00:14:14Z). The bead is a completed task, not a
crash: what "crashed" was the 2026-08-12 storm of 50 consecutive agent dispatches for it, each
killed with exit −1 on the pre-cleanup ~18 GB repository.
**Classification:** INFRASTRUCTURE (`docs/crash-response-guide.md` exit −1 row; committed verdict in
[`crash-investigations/bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md`](../crash-investigations/bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md))
**Chain position:** domchk-221cb3aa (locate report) → domchk-4950dc16 (extract report details) →
domchk-e5404cd7 (extract log/record details) → domchk-cbe2665d (identify gaps) → **this bead
(synthesize: are the docs sufficient?)**

**What this page adds that the companions do not.** The gap analysis
([`crash-investigations/bf-4yjq-context-gap-analysis-domchk-cbe2665d-2026-09-06.md`](../crash-investigations/bf-4yjq-context-gap-analysis-domchk-cbe2665d-2026-09-06.md))
is checklist-bounded: it scores the crash-context report and triages its gaps. This page is the
chain's synthesis step, written after a same-day evidence commit (`77fac01`) changed part of the
gap picture. Its three inputs: (1) it resolves **this dispatch's own crash timestamp** —
`2026-08-12T20:12:37.375433456+00:00`, different from every sibling dispatch's — to the exact
death event it belongs to (§1.2, new); (2) it restates the gap list **with post-`77fac01` status**
— the rotation risk is closed and G-4 is substantially answered (§3); (3) it renders the final
completeness verdict the chain was building toward (§4/§6).

---

## 1. Crash summary

### 1.1 The event

| Field | Value | Source |
|-------|-------|--------|
| Bead | bf-4yjq (git-remotes reconciliation task — Forgejo-primary origin + server-side push mirror) | bead record |
| Event class | 50 consecutive dispatch deaths, all `exit_code=-1`, `outcome=Crash(-1)`, `signal_code=-1` | worker-log extract (225 records), independently re-derived twice |
| Death window | 2026-08-12T17:53:53.875Z → 2026-08-12T20:30:38.310Z (~3.1 min mean cadence) | [`crash-investigations/bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md`](../crash-investigations/bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md) §5 |
| Per-run survival | 65–375 s of real work before each kill (median 149 s) — mid-task kills, far under the 600 s dispatch timeout | same, §4 |
| Environment at death | repo ~18 GB / 17.16 GiB loose objects (ratio ≈1,800:1 inverted); load 15–17 on 12 cores; disk 84% | [`crash-analysis/bf-4yjq-system-state-snapshot-2026-09-01.txt`](../crash-analysis/bf-4yjq-system-state-snapshot-2026-09-01.txt) |
| Outcome | Environment repaired Aug 13 (repo packed 18 GB → ~93 MB); bead completed and closed 2026-08-17; zero recurrences since | [`bf-4yjq-cleanup-verification.md`](bf-4yjq-cleanup-verification.md) |

**Exit code −1 / "signal −1" semantics (read this before quoting either).** `exit_code=-1` is the
recorded needle field; `signal_code=-1` is the crash-handler field quoted in every alert body
("`**Exit code**: -1 (signal -1)", verbatim). **−1 is needle's sentinel for a death whose signal was
not recorded — it is not a signal number and no signal name survives anywhere in the telemetry**
(extraction §1). Older documents' "signal −1 = SIGKILL (Signal 9)" is a gloss, not a record. The
mechanism the fleet later verified for this death class — kernel memcg OOM SIGKILL inside the
dispatch scope's `MemoryMax=12 GiB` — holds at **MEDIUM-HIGH confidence for this date**; the kernel
line that would prove it for Aug-12 cannot exist (§3, G-1).

### 1.2 This dispatch's timestamp, resolved to its death event (new)

The dispatch supplied `2026-08-12T20:12:37.375433456+00:00` as the crash time. That is a real
record, but it is **not the death timestamp** — it is the "Timestamp" field of the alert-bead body
needle wrote for the death, **5.77 s after it**:

| Event | Time (UTC) | Source |
|-------|-----------|--------|
| **Death — `agent.completed exit_code=-1` (43rd of 50), alert `bf-47ugw`** | **2026-08-12T20:12:31.602783Z** | preserved worker-log extract, read live for this page |
| Alert-body "Timestamp" — **the value this dispatch was dispatched with** | 2026-08-12T20:12:37.375433456+00:00 | supplied; **+5.773 s** after the death |
| Alert bead `bf-47ugw` `created_at` | 2026-08-12T20:12:37.381899600Z | `.beads/checkpoint/forensic.jsonl`, read live; **+6.5 ms** after the body timestamp |
| Needle's `crash alert bead created alert_id=bf-47ugw` log line | 2026-08-12T20:12:39.840943Z | preserved extract; +8.24 s after the death |

All three offsets sit inside the skews the record-level extraction had already quantified
(death→alert-body 5.1–9.0 s, median 6.0 s; body→`created_at` single-digit ms; death→log-line
7.0–12.6 s) — this death is a textbook member of its own distribution, which is the cross-check.
Sibling dispatch domchk-4e32a3a4 resolved *its* timestamp (`19:54:44`, a
`HANDLING_RELEASE_DONE` heartbeat 5.7 s after death #38) the same way; the two resolutions together
confirm the standing rule: **treat every dispatch- or alert-supplied timestamp as a ~6 s upper
bound; the death is the `agent.completed exit_code=−1` record.** The report-level extraction
(domchk-4950dc16 §2) had already tied `bf-47ugw` to this timestamp but left the death unresolved;
that gap is now closed, here.

## 2. What the documentation records (the documented-details inventory)

Full inventories live in the two extraction documents — report-level
([domchk-4950dc16](../crash-investigations/bf-4yjq-context-report-extraction-domchk-4950dc16-2026-09-06.md))
and record-level ([domchk-e5404cd7](../crash-investigations/bf-4yjq-crash-details-extraction-domchk-e5404cd7-2026-09-06.md)).
Condensed, the record as it stands covers:

- **Death record:** all 50 death timestamps to the microsecond, each paired to its alert bead and
  survival time (extraction §5); uniform `Crash(-1)`/`signal_code=-1`; `was_interrupted=false`.
- **Non-crash tail:** 1 exit-1 failure (18:00:17), 4 × exit-124 at exactly 600.1 s (20:40–21:11),
  1 orphaned exit-0 (21:14:56), 3 auto-splits — disruption window ends 21:14:59Z, not 20:30.
- **Agent identity:** worker `claude-code-glm-4.7-lab-domain-check`, model `claude-code-glm-4.7`,
  session `8446529e`, workspace `/home/coding/domain-check`, `claim_auto` × 56.
- **Task context:** the six-step git-remotes remediation plan; "crashes incidental to the task";
  zero commits in the storm window; bead closed 2026-08-17 and re-verified live.
- **Environment:** crash-era repo metrics, load/disk/inode figures; today's healthy repo state
  (93 MB, fsck clean, Forgejo origin, timers armed).
- **Error text:** the complete inventory is the 50 identical ERROR handler lines and the
  machine-written alert body — no stack traces, no agent stderr, no application error text exist
  anywhere in the corpus (with the would-have-held sources checked and enumerated, §3).
- **Correlation context:** bloat origin (bf-2ildm's 17+ × ~237 MB `.beads/*.jsonl` commits), the
  sibling-bead signal−1 pattern across 2026-08-11→17, the fleet-wide 455-event storm.
- **Raw evidence, now in git:** worker-log extract (225 records, sha-verified), 1,071-event needle
  events jsonl, 56 crash-era session transcripts (tarball + index), `MANIFEST.sha256` —
  [`docs/crash/bf-4yjq/raw-logs/`](../crash/bf-4yjq/raw-logs/README.md), committed in `77fac01`.

## 3. The missing-context items (from the previous task), with status as of today

### 3.1 The eight gaps (domchk-cbe2665d), restated with post-`77fac01` status

| # | Gap | Why it cannot be recovered | Status today |
|---|-----|----------------------------|--------------|
| G-1 | **Kernel OOM record** — which process the kernel selected, its badness score, host-wide vs in-memcg scope | System journal begins 2026-08-15 19:46:33 EDT; user journal 2026-08-17. Aug-12 kernel output does not exist on this box. **Load-bearing:** the reason mechanism confidence is capped at MEDIUM-HIGH | **Closed permanently** |
| G-2 | Signal identity per death | −1 is the unrecorded-signal sentinel; no signal name in any of the 225 records | **Closed permanently** |
| G-3 | Per-crash memory figures (dying process RSS, scope limit, free RAM at the death instant) | Never captured — no monitoring stack existed on Aug-12 (resource-monitor timers date from 2026-09-02) | **Closed permanently** |
| G-4 | The in-flight command at each death (fetch / merge / repoint / mirror-config) | Former reason (superseded same day): "no session transcripts, no traces, stderr slots bracket the storm" — the transcripts did survive, in a location no inventory had checked | **Answered by `77fac01`** — see §3.2 |
| G-5 | Process identity at death — needle worker vs git child | The distinction later kernel-recoverable crashes resolved stays inferential for this one | **Open, bounded** — the transcript evidence points at `git push`'s pack-objects (§3.2) but "last recorded tool call" is not the kernel's oom-select decision; only G-1's lost kernel line could have settled it |
| G-6 | Crash-era commits and workspace diffs | Removed from the DAG by the Aug-16 squash `c27899f`; unreachable by any git operation | **Closed permanently** |
| G-7 | Core dumps | `coredumpctl` earliest entry 2026-08-17 16:01 EDT; SIGKILL-class deaths produce none by design | **Closed permanently** |
| G-8 | Aug-12 heartbeats | `.beads/heartbeats.jsonl` earliest entry 2026-08-15T12:06Z | **Closed permanently** |

### 3.2 What changed the same day: `77fac01` closed the rotation risk and answered G-4

Two same-day passes landed after the gap page was written, both now in git as part of
[`docs/crash/bf-4yjq/raw-logs/`](../crash/bf-4yjq/raw-logs/README.md) (commit `77fac01`,
domchk-495041ac, pushed — the branch tip this page is written on):

1. **The rotation risk is gone.** The gap page's one still-actionable item was that `.log.2` — the
   storm's only primary source — sat in rotation slot 2 with the live slot already past 90 MB.
   All 225 worker-log records, the 1,071-event structured events jsonl, and the 56 per-run agent
   session transcripts (tarball + `sessions-index.tsv` + `MANIFEST.sha256`) are now committed;
   the worker-log extract copy in
   [`docs/crash-analysis/`](../crash-analysis/bf-4yjq-needle-worker-log-extract.log) is
   sha-verified byte-identical (`95d7f713…393255`). `.log.2` itself was re-checked live for this
   page — still on disk, still 134,207,883 B — but it is now a convenience copy, not the only one.
2. **G-4 is answered per-run.** The standing "no session evidence survives" claim was wrong: all
   56 per-run transcripts survive under `~/.claude/projects/`, a location no prior inventory had
   checked. The raw-log pass matched them 1:1 to dispatches (zero unmatched) and found **all 50
   crashed runs end at `git push origin main` as their final recorded Bash tool call**, at 7–32
   git commands into each retry; the lone exit-1 failure ends instead at `git add -A && git
   commit`. This page re-verified the claim by spot-extraction (attempt-01 at 17:53:38.489Z —
   the exact timestamp the README quotes — attempt-25, and attempt-02 all reproduce it). The
   reading it forces: the Aug-12 storm was itself **push-side** — `git push`'s pack-objects over
   the ~17 GB of loose objects inside the dispatch scope's 12 GiB `MemoryMax` — so bf-198ne
   (Aug-16) was not a later "push-side variant" of a gc-side event; it was the same mechanism
   already operating on Aug 12, which is also why the `pack.windowMemory`/`pack.threads` bounds
   installed 2026-09-02 cover push as well as gc.

**No other gap changed status.** G-1, G-2, G-3, G-6, G-7, G-8 are permanently unrecoverable — they
are properties of what Aug-12's box did not record, not of what later work failed to save. The
future-facing half of the gap page's §5 stands as written: the monitoring stack deployed
2026-09-02 records exactly the categories G-1–G-8 lost, so the *next* storm of this class is
attributable directly from raw logs.

## 4. Sufficiency assessment — is the documentation enough for root-cause analysis?

**Verdict: yes, sufficient — with one stated confidence ceiling. The record is as complete as it
can ever be for this event; nothing further is obtainable, and no further investigation of the
event itself is warranted.**

| Question a reader brings | Can the docs answer it? | Basis |
|--------------------------|------------------------|-------|
| What died, when, how often? | **Yes — definitively** | 50 death timestamps to the microsecond, survival per run, alert pairing, all re-derived twice and now git-preserved |
| Why did it die? | **Yes at class level (HIGH); mechanism at MEDIUM-HIGH** | Bloat correlation is proven — crashes ceased exactly when the repo was cleaned and never returned across ~3 weeks. The kernel-OOM step (G-1) is an attribution from later, kernel-recoverable members of the same death class, not a record for this date. Since `77fac01` the per-run record corroborates the mechanism from inside the storm: 50/50 deaths at the `git push` step over the bloated object store |
| Which process/command died? | **Command: yes, per-run. Process identity: inference, permanently** | G-4 answered (§3.2): every crashed run's final recorded Bash call is `git push origin main`. G-5 remains open in the strict sense — the agent is not sampled between tool calls, so the transcript pins the workflow step, not the kernel's oom-select decision; only G-1's lost kernel line could have settled it |
| Was it the task's fault? | **Yes — excluded, definitively** | Zero application error text in the corpus; zero commits in the window; the killed process was the dispatch agent, not domain-check — consistent with the repo-wide zero-defect finding |
| Was the alert a false positive? | **No — excluded on all three guide rules** | Crashes were mid-task (not post-completion); retry never succeeded inside the storm (recovery came from an environment change 5 days later); recorded caveat: the guide's ≥10-crashes/10-min surge detector would *not* even have fired (peak 5/10 min) — itself a documented gap (G-1 of `docs/crash-prevention-requirements.md`) |
| Could it happen again? | **Yes — answered** | Trigger condition removed and guarded (gitignored `.beads/`, 10 MB pre-commit gate, `pack.windowMemory` bounds, gc timers); reproducibility verdict: "not reproducible today; was deterministically reproducible while the trigger existed" (canonical §7) |

The confidence ceiling is *already* correctly stated in the documents a future reader hits first
(canonical §6: "HIGH on the bloat correlation, MEDIUM-HIGH on the OOM mechanism"; evidence record
§2.3; gap page §4). Nothing in the corpus overstates it except the 2026-08-26
crash-context report's own "Gaps: NONE IDENTIFIED" line, which the gap page §6 corrects and which
is superseded-banner territory, not new investigation.

**Is the documentation *complete*?** Two different answers, and the distinction matters:

- **Evidence record: complete — permanently.** Every surviving source has been read, extracted,
  cross-checked, and (as of `77fac01`) committed. The remaining unknowns are unrecoverable *in
  principle*, not pending work. "Requires additional context" is answered **no**.
- **Document set: one class of residual debt, already owned.** Older documents still carry
  superseded claims (the 9-crash figure, "gc completed ~1.1 GB peak, no OOM events", "signal −1 =
  Signal 9"). These are hygiene items on beads that already exist — the stale-doc refresh
  (domchk-7625a5cc) and the supersession banners noted in the consolidated findings §7 — not gaps
  this chain can close by investigating further.

## 5. Recommendations

### 5.1 For this event — nothing left to investigate

1. **Do not attempt further recovery.** G-1, G-2, G-3, G-6, G-7, G-8 are closed permanently
   (§3.1); G-4 was closed by `77fac01`'s transcript pass, and G-5 is closed *as a question* in the
   only sense that matters for mitigation — the death point is the `git push` step, and the
   mitigation (`pack.windowMemory`/`pack.threads` bounds on push, not just gc) is already
   installed and verified. Whether the killed PID was the worker or its pack-objects child
   changes no action a future operator would take. Further digging produces restatements, not
   facts — the failure mode this repo's crash corpus has repeatedly documented.
2. **Carry one narrative correction forward:** documents that date the *push-side* memcg-OOM
   variant to bf-198ne (Aug-16) predate `77fac01` and should be read as superseded on that
   point — the Aug-12 storm was itself uniformly push-side (§3.2). This affects framing, not
   conclusions: the mitigation set is unchanged.
3. **Cite the committed evidence paths, not `.log.2`.** Documents that carry a "re-derive with
   `grep 'bf-4yjq' …log.2`" instruction (extraction §6, gap page §5) go stale the moment
   rotation fires. The reproducible instruction is now: read
   [`docs/crash/bf-4yjq/raw-logs/`](../crash/bf-4yjq/raw-logs/README.md) and verify against its
   `MANIFEST.sha256` — noting the README's own git note that the `*.jsonl`/`*.log` copies there
   were force-added past the repo's ignore rules and that `git clean -X` in that directory
   would delete them.

### 5.2 For the record as a whole — point, don't duplicate

The open items that remain are systemic and already tracked elsewhere; restating them here would
only create another doc to keep in sync. In priority order, they live in:

- `docs/crash-prevention-requirements.md` G-1–G-13 — the prevention gaps (storm-level detection,
  evidence retention, dispatch-scope sizing, work-completion detection at the alert source), the
  closure of which is what makes the *next* Aug-12 fully attributable;
- canonical report §9 recommendation 2 / consolidated findings §7 — the Aug-12 alert-bead backlog
  (most of this storm's 50 alerts are still open, distorting counting);
- domchk-7625a5cc — the stale-doc refresh for documents still repeating superseded figures (§4).

## 6. Answer to the dispatch's closing question

**Is the crash documentation complete, or does it require additional context?**

**Complete for root-cause purposes; no additional context is obtainable.** What happened, at what
scale, to whom, in what environment, at which workflow step, with what task impact, and what ended
it — all definitively recorded, live-verified, and now git-preserved. Why it died is recorded at
HIGH confidence for the correlation and MEDIUM-HIGH for the kernel-level mechanism, with the
ceiling's cause (G-1) itself documented and permanent; the per-run transcript evidence places all
50 deaths at the `git push` step, which is mechanism corroboration the gap page did not have when
it was written. The two facts this dispatch contributes are §1.2 — the dispatch-supplied timestamp
`20:12:37.375433456+00:00` is alert-body time for death #43 (`bf-47ugw`), 5.77 s after the actual
death at `20:12:31.602783Z`, recorded here so the next dispatch carrying it does not re-derive it —
and the post-`77fac01` gap-status restatement in §3. The remaining work in this area is prevention
implementation and documentation hygiene, both owned by beads that already exist; nothing about
bf-4yjq itself is open.

## 7. Provenance — checked live for this dispatch, 2026-09-06 ~15:45–15:56 EDT

Nothing inherited unverified. Commands and results:

- Death event: `grep '20:12:' docs/crash-analysis/bf-4yjq-needle-worker-log-extract.log` →
  `handling agent outcome … exit_code=-1 outcome=Crash(-1)` at `20:12:31.602783Z`, the ERROR
  handler line 30 µs later, `crash alert bead created alert_id=bf-47ugw` at `20:12:39.840943Z`,
  next claim at `20:12:42.350523Z`.
- Alert bead: `bf-47ugw` in `.beads/checkpoint/forensic.jsonl` → `created_at`
  `2026-08-12T20:12:37.381899600Z`, title "ALERT: Agent crash on bead bf-4yjq".
- Arithmetic: 37.375433456 − 31.602783 = **5.773 s** (death→body); 6,466,144 ns = **6.5 ms**
  (body→`created_at`); 8.24 s (death→log line) — all inside the extraction's quantified skews.
- Ordinal: death #43 of 50 by position in the extraction's §5 table (agreeing with sibling
  domchk-4e32a3a4's independent numbering of #37/#38 in the same list).
- Raw evidence: `sha256sum docs/crash-analysis/bf-4yjq-needle-worker-log-extract.log` →
  `95d7f713ceea166e6e5c2e67b5f1a0c5c423f58610cce6e74b5726d54a393255`, matching the preservation
  pass's recorded hash; `git log -- docs/crash/` → `77fac01` (6 files, 1,513 insertions),
  `git rev-parse HEAD origin/main` → identical (`77fac01`), i.e. committed **and pushed**.
- Transcript spot-check (G-4 corroboration): extracted the `77fac01` tarball to a scratch
  directory and read the final `Bash` `tool_use` of three transcripts — attempt-01 (crash)
  `git push origin main` at `2026-08-12T17:53:38.489Z` (the exact timestamp the raw-log README
  quotes), attempt-25 (crash) `git push origin main`, attempt-02 (failure, exit 1)
  `git add -A && git commit …` — all three reproduce the README's §4 claims.
- `.log.2`: present, 134,207,883 B — unchanged from every earlier same-day check, now a redundant
  convenience copy.
- Subject bead: `bead show bf-4yjq` → **Closed**, git-remotes task, closed 2026-08-17T00:14:14Z.
- This bead: `bead show domchk-9d9a0294` → InProgress, no prior worker commits
  (`git log --all --grep domchk-9d9a0294` → empty) — deliverable path
  `docs/crashes/bf-4yjq-context-gap-analysis.md` did not exist before this page.
