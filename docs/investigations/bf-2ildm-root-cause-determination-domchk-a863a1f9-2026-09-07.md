# Root Cause Determination: bf-2ildm (2026-08-13 alert storm)

**Investigation dispatch:** domchk-a863a1f9 (2026-09-07)
**Crash target:** bf-2ildm — "Extract GitHub-specific commits" (CLOSED 2026-08-16T22:44:38.873Z, rev 7)
**Alert bead carrying the investigated stamp:** bf-66sw7c (created 2026-08-13T14:40:42.642Z)
**Classification basis (previous step):** domchk-2c792cc9, `docs/crash-classification-bf-2ildm-2026-08-13-14-40.md` (commit `d82b6a2`)
**Supersedes:** `docs/investigations/root-cause-determination-bf-2ildm-2026-09-02.md` (domchk-b672deb9) — see §4

## Verdict

| Question | Answer | Confidence |
|---|---|---|
| What killed bf-2ildm's Aug-13 attempts? | **INFRASTRUCTURE — repository-bloat regime**: kernel memcg-OOM SIGKILL inside needle's 12 GiB per-dispatch scope | Kill regime **HIGH**; precise mechanism **MODERATE** (no Aug-13 kernel record survives) |
| Was work lost? | **No.** The target closed successfully 2026-08-16 after a retry (trace `exit_code: 0`, 85,327 ms, captured 2026-08-16T22:28:44Z) | HIGH |
| Was alert bf-66sw7c correct? | **FALSE_POSITIVE at the alert level** — it fired at a real kill, but the work it implies was lost was completed three days later | HIGH |
| Root cause in the domain-check codebase? | **None.** No code defect in any of the 157+ investigations of this workspace | HIGH |

Two levels, and they answer different questions. The 38 kills were real; the
alarm's implication (work lost, investigation owed) was not. Both the previous
step's classification and this determination hold that structure.

## 1. Scope decision — applying this task's acceptance criteria

The task says "Skip this if classification was FALSE_POSITIVE." The previous
step's classification is **two-level**, so the skip rule is applied per level:

- **Alert level — FALSE_POSITIVE → skipped.** No alert-side investigation is
  owed. bf-66sw7c was already closed by the classification chain (rev 29,
  2026-09-07) with the bf-26r8bi disposition ("target resolved, no work lost").
  This dispatch does not re-open that work; §5 only appends an investigation
  summary to the closed bead's notes, per the task's output requirement.
- **Kill level — INFRASTRUCTURE → completed here.** The type-appropriate
  investigation steps (system resources, repository health, memory pressure)
  are run **live** in §5, and the kill's root cause is stated in §2.

What remained genuinely open for an RCA step after the classification docs:
(a) one consolidated kill-level root-cause statement (the classification docs
deliberately deferred regime detail to the maintenance guide), (b) superseding
the wrong 2026-09-02 RCA that still sat in `docs/investigations/` (§4), and
(c) live verification that the mitigating layer holds (§5). That is this
document.

## 2. Kill-level root cause

**Statement:** The Aug-13 attempts of bf-2ildm were killed by the workspace's
repository-bloat regime — git/bead-state operations whose memory footprint
exceeded the 12 GiB `MemoryMax` of needle's per-dispatch systemd scope, so the
kernel's memory-cgroup OOM killer SIGKILLed the worker (exit −1 is needle's
died-without-exit-code sentinel, `code().unwrap_or(-1)`; a true SIGKILL encodes
137). The regime's preconditions on 2026-08-13: ~18 GB of loose objects from
17+ identical 237 MB `.beads/*.jsonl` snapshots still tracked in git, and a
422-commit unpushed backlog. Every dispatch re-entered the same regime, which
is why the deaths recurred at a fixed cadence until the repo was cleaned.

**Confidence split, stated plainly:**

- **Regime — HIGH.** 38 of 43 attempts died exit −1, all mid-task (durations
  98–309 s), at a fixed ~2–5 min re-dispatch cadence across 2 h 16 m — the
  signature the crash-response guide assigns to its "Infrastructure:
  repository bloat" sub-type. The same-day census (`d9c4622`, R5) attributes 38
  exit-minus-one kills to bf-2ildm alongside bf-65lsdu 127, bf-1ea4g 56,
  bf-4k2ws 55, bf-1s6c3 22 — one regime, many beads, same morning.
- **Mechanism — MODERATE.** Assigned **by regime match**, not direct record:
  journald on this host has a single boot beginning 2026-08-15 19:56:33 EDT, so
  no Aug-13 kernel line survives for any bead. The mechanism is the era's
  proven one (recovered later for bf-4x12ec, bf-198ne, bf-1s6c3), but for this
  bead specifically no `CONSTRAINT_MEMCG` line can be quoted. This is an
  evidence-retention limit, not a competing hypothesis.

**Ruled out:**

| Alternate | Why not |
|---|---|
| CODE_DEFECT | Zero panics/stack traces in any attempt; the task (git log extraction between two remotes) is the same shape that succeeded on 2026-08-16 in 85 s |
| SERVICE_FAILURE | Deaths are mid-task signal losses, not HTTP 5xx/timeout exits; no gateway involvement in the attempt profile |
| WORKFLOW_FAILURE | Not max-turns (exit would be 1 `error_max_turns`, not −1); 4 later attempts hit the 600 s cap (exit 124), a different, non-crash class |
| Wrong-tool/bead-store corruption | No schema errors in any record; deaths precede any store mutation |

## 3. Alert-level root cause (why bf-66sw7c is a false positive)

The alert's carried stamp `2026-08-13T14:40:42.628685942+00:00` is the crash
handler's post-kill heartbeat, **13.1 s after** the real death it names, and
14 ms before the alert bead's creation. Per the retrieval bundle's
`attempt-index.tsv`, the attempt this alert maps to is **attempt 19**:

| Field | Value |
|---|---|
| claim → kill | 2026-08-13T14:38:24.455725579Z (log line 6299) → **14:40:29.551762239Z** |
| exit / duration | −1 / 124,759 ms (mid-task) |
| released / handled | 14:40:46.615684833Z / `alerted` |
| alert bead | bf-66sw7c, created 14:40:42.642439194Z (stamp +13.1 s) |

The alert fired correctly at a real kill while the bead was open. It became a
false positive retroactively: the target succeeded on 2026-08-16 (exit 0,
85.3 s) and closed at 22:44:38.873Z, so the implied "work was lost" never held.
Dedup gate confirms: `alert-deduplication.sh check bf-66sw7c` → DUPLICATE,
target resolved. Cost of the storm: ~2.3 h of wall time across 38 killed
attempts — not work.

**Ordinal correction (dated 2026-09-07).** The classification doc (§2 of
`crash-classification-bf-2ildm-2026-08-13-14-40.md`) and the notes it appended
to bf-66sw7c label this kill "attempt 20" and call bf-26r8bi "attempt 21".
Against the retrieval bundle's authoritative per-attempt index, the ordinals
are **19** (bf-66sw7c) and **20** (bf-26r8bi). Every timestamp and duration the
classification doc cites matches attempt 19's row exactly, so the finding is
unchanged — only the label was off by one for this pair. The companion instants
are correct as published: bf-1wkda = attempt 3 (kill 13:44:10.765Z), bf-z15pix
= attempt 24 (kill 15:01:35.775Z). Cite the bundle's ordinals.

**Storm shape (from the bundle, re-verified 2026-09-07):** 43 attempts — 38 ×
exit −1 (kills 13:37:24.838573229Z → 15:53:28.914636262Z), 4 × exit-124
(exactly 600,000 ms caps, attempts 39–42, no alert), 1 × exit-1 at 19 ms
(attempt 43, quarantined 16:35:40.675Z at `failure_count: 5`). The 14:00Z hour
alone holds 15 consecutive kills (attempts 9–23). All 38 alerts map 1:1 to
their kills (7.7–29.4 s gap), no duplicates, no orphans.

**Documented absences** (why the mechanism cannot be proven directly): no
kernel/OOM record for any of the 38 kills; no `.beads/logs/` telemetry exists
for Aug-13 (collectors start 2026-09-02); the single-slot trace archive was
overwritten by the successful Aug-16 run — which is also what broke the
classifier (`crash-classifier.sh` → UNKNOWN, with a trace-provenance warning)
and, more consequentially, misled the 2026-09-02 investigation (§4).

## 4. Supersession of the 2026-09-02 root cause determination

`root-cause-determination-bf-2ildm-2026-09-02.md` (domchk-b672deb9) concluded:
"**Bead bf-2ildm did NOT crash** … the reported crash was a FALSE POSITIVE
caused by systematic bugs in the crash alert generation system," root cause
"placeholder exit code −1 instead of actual trace data." That conclusion is
**superseded on the kill level** by the per-attempt index assembled 2026-09-07
(domchk-ea755548, `docs/crashes/bf-2ildm/`): the bead was killed 38 times. The
09-02 report's central inference error was **evidence selection**: the
single-slot trace archive held only the successful Aug-16 retry (exit 0), and
the report read that surviving record as if it were the record *of the
reported crash* — so the alert's exit −1 looked fabricated next to the trace's
0. In fact the −1s came from 38 real kills, and the trace slot had simply been
overwritten by the later successful attempt (the retention blind spot §3
documents).

Its "physical impossibility" argument dissolves under the corrected timeline:
an alert generated at a kill, while the bead is open and mid-task, is exactly
what the alert system exists to produce. Nothing was generated "3+ days before
completion"; the alert was generated 3 days *before the completion that later
made it stale*. The same reversal applies to the 2026-08-26 verification
report's "generated after completion" (both directions corrected in the
classification doc §5). Precise ordering: kill 08-13T14:40:29.551Z → heartbeat
14:40:42.628Z → target closes 08-16T22:44:38Z.

**What survives from the 09-02 report** (and from bf-2ildm's bead note, which
carries its summary): the five alert-layer defects it listed were real defects
of the alert layer regardless of the kill question — no closed-bead filtering,
no dedup, no cooldown, no exit-code validation — and the fixes implemented
2026-09-02 stand (`test-crash-alert-fixes.sh` 13/13, `test-closed-bead-filter.sh`
7/7, re-verified 2026-09-07). The task-completion finding (target closed, work
intact) also stands. Note the report was internally tense on exactly this
point: its §4 already listed "6.0G `.beads/` … OOM when loaded" as the original
crash conditions while its §1 claimed no crash occurred. A dated correction
banner now sits at the top of that file pointing here. bf-2ildm's bead record
needs no further correction — the classification chain already appended the
current verdict there (rev 7, 2026-09-07).

## 5. Live infrastructure verification (INFRASTRUCTURE_EVENT steps, run 2026-09-07)

The kill regime's preconditions were re-checked live today; the regime is
repaired and holding:

| Check (live command) | Result today | Era (2026-08-13) |
|---|---|---|
| `.git` size (`du -sh .git`) | **104 MB** | ~18 GB |
| Loose objects (`git count-objects -vH`) | 129 / 1.01 MiB | ~17 GB |
| Packed | 11,700 objects, 2 packs, 99.78 MiB, 0 garbage | fragmented |
| Repo health (`check-repo-health.sh`) | **exit 0** — fragmentation OK, no unmanaged aggressive gc, backlog CLEAR | failing regime |
| Pack-memory bound (`setup-git-gc-config.sh --verify`) | **exit 0** — effective (system→global→local), worst case ≈3072 MiB within the 6 GiB ceiling of the 12 GiB dispatch scope | unbounded bare gc/push |
| Unpushed backlog | **0 commits** (`origin/main..HEAD`) | 422 commits |
| `.beads/` tracked files (`git ls-files .beads \| wc -l`) | **0** (gitignored, `.gitignore` lines 66/68–70) | 237 MB snapshots committed |
| Pre-commit 10 MB gate (`setup-git-hooks.sh --check`) | exit 0, installed and current | absent |
| Maintenance timers (`systemctl --user list-timers 'domain-check-*'`) | **8/8 with future trigger times** | absent |
| System memory (`free -g`) | 44 G available | pressure |
| Disk (`df -h /`) | 43 G free (above the 30 G warning line) | — |
| Load (`uptime`) | 4.67 / 5.73 / 7.49 (1/5/15 min) | — |

Footnote for completeness: `.beads/` is 5.2 G **on disk** today, but with 0
tracked files it cannot bloat the repository — bead-state size is a store
retention concern, not the repo-side OOM mechanism that killed this bead.
Reproduction attempt is deliberately not performed: re-creating an 18 GB
loose-object repository to re-test a memcg kill would risk the box for a
mechanism already proven three times over (bf-4x12ec, bf-198ne, bf-1s6c3).

## 6. Mitigation status and residual gaps

**In force, verified live today (§5):** the layered defense that ends this
regime — gitignore (`.beads/`, `*.db`, `*.jsonl`) → 10 MB pre-commit gate
(`dfa60a9`) → persistent pack-memory bounds applied repo-locally *and* globally
(so bare `git gc`/`git push` are bounded too) → `safe-git-gc.sh` bounded path
with checkpoint/resume → daily repo-health + incremental gc + weekly full gc
timers. The repository-side cause of bf-2ildm's kills is **closed**; no new
repo-side mitigation work is opened by this investigation.

Residual gaps, each already owned elsewhere — listed so this report is honest
about what the investigation does *not* close:

1. **Single-slot trace retention** (the blind spot that produced the wrong
   09-02 RCA and the classifier's UNKNOWN): the last attempt overwrites all
   prior ones. Compensating control in place — retrieval bundles with
   per-attempt indexes (domchk-ea755548). Candidate improvement, no owner yet:
   multi-slot retention keyed by (bead, attempt).
2. **`crash-alert-manager.sh` classification wiring** (open bead
   `domchk-f6fff20f`): the manager reads the classifier's banner line as
   `CLASSIFICATION`, so the FALSE_POSITIVE branch never fires. Cascade
   prevention is unaffected (verified by replay, 2026-09-07). Owner bead open;
   lesson recorded — grep-marker suite tests cannot see wiring bugs.
3. **Alert-stamp provenance**: alert beads carry the handler heartbeat, not the
   kill instant. Documented control: cite kills from the per-attempt index,
   never the bead stamp. Candidate: stamp the kill instant on the alert bead.
4. **Per-clone hook protection**: the 10 MB gate is per-clone; a fresh clone is
   unprotected until `setup-git-hooks.sh install`. Documented in CLAUDE.md.

## 7. Sources (all cited as of 2026-09-07)

- `docs/crashes/bf-2ildm/` retrieval bundle (domchk-ea755548): `attempt-index.tsv`
  (attempt 19 row; 43 rows), `crash-alert-ledger.tsv` (bf-66sw7c row — its
  `status_now` column is a ~14:00Z snapshot, predating the classification
  chain's closes), `README.md`, `trace-archive-current-state.json`
- Classification records: `docs/crash-classification-bf-2ildm-2026-08-13-14-40.md`
  (`d82b6a2`), `docs/crash-classification-bf-2ildm-2026-08-13-15-01.md`
  (`e3a8820`), `docs/archive/crash-investigations/crash-summary-bf-2ildm-timestamp-2026-08-13-13-44.md`
- Superseded: `docs/investigations/root-cause-determination-bf-2ildm-2026-09-02.md`
  (domchk-b672deb9), `docs/investigations/bf-2ildm-final-investigation-report-2026-09-02.md`
  (2026-09-02 corpus), `docs/verification/verification-report-bf-66sw7c-false-positive-alert-resolved-bf-2ildm-crash.md`
  (2026-08-26; timing claim corrected)
- Live store: `bead show bf-2ildm` (closed, rev 7, classification appended),
  `bead show bf-66sw7c` (closed, rev 29), `alert-deduplication.sh check bf-66sw7c`
  (DUPLICATE), `crash-classifier.sh bf-2ildm` (UNKNOWN + provenance warning)
- Live infrastructure runs (§5): `check-repo-health.sh`, `setup-git-gc-config.sh --verify`,
  `setup-git-hooks.sh --check`, `git count-objects -vH`, `git ls-files .beads`,
  `systemctl --user list-timers`, `free -g`, `df -h /`, `uptime`
- Era evidence: commit `d9c4622` (Aug-13 per-bead kill census),
  `docs/crash-analysis-bf-1s6c3-2026-09-06.md` (bloat mechanism),
  `docs/maintenance/repository-maintenance-guide.md` (mitigation layer)
- Related open item: `domchk-f6fff20f` (alert-manager classification wiring);
  CLAUDE.md "Crash Alert System" (suite counts, replay verification)
