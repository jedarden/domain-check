# Comprehensive Crash Context — bf-4yjq

**Subject bead:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo mirror has
diverged/gone stale" (P2, closed 2026-08-17, task verified complete)
**Synthesis dispatch:** domchk-577a6273 · **Written:** 2026-09-06
**Crash date:** 2026-08-12 — storm window 17:50:23 → 21:14:56 UTC (13:50–17:14 EDT)
**Classification:** **INFRASTRUCTURE — repository bloat → cgroup-scoped memcg OOM during
`git push`** (not a service failure, not a workflow failure, not a code defect)

This document consolidates the four gather layers of the bf-4yjq investigation chain into one
crash-context reference:

| Layer | Document | What it contributes |
|---|---|---|
| Raw logs | [`bf-4yjq/raw-logs/`](bf-4yjq/raw-logs/README.md) | 56 per-run session transcripts, needle event log (1,071 records), worker-log extract (225 lines), per-attempt index — the primary sources |
| Errors | [`bf-4yjq/error-analysis.md`](bf-4yjq/error-analysis.md) | Complete error-message inventory, death-point analysis, verified absences |
| Version info | [`bf-4yjq/version-info.md`](bf-4yjq/version-info.md) | Code/build/agent-runtime metadata, git state dispatch by dispatch |
| Infrastructure | [`bf-4yjq/infrastructure-status.md`](bf-4yjq/infrastructure-status.md) | Memory/CPU/disk/kernel/gateway/system-event findings at crash time |

Those four documents were each derived live on 2026-09-06 from the committed primary sources,
not copied from earlier reports. This synthesis carries their figures forward, marks what was
re-verified for *this* dispatch, and reconciles them with the standing record (the canonical
report and its supersession list). Where this document disagrees with an older bf-4yjq doc,
the siblings and the canonical report win — §10 catalogues the disagreements.

---

## 1. Answer up front

| Question | Answer |
|---|---|
| **What crashed** | 50 consecutive agent dispatches against bead bf-4yjq, all killed with needle's `exit_code=-1` sentinel, between 17:53:53.875Z and 20:30:38.310Z on 2026-08-12 — one death every ~3.1 min for 2 h 37 m. Followed by 4 command timeouts (exit 124) and one exit-0 run that still failed to close the bead. |
| **What the bead was** | A real git task — repoint `origin` at Forgejo, reconcile the Forgejo/GitHub divergence with a merge commit (no force-push), configure the Forgejo→GitHub server-side push mirror. Not a crash-investigation bead. |
| **Root cause** | ~18 GB repository with 17.2 GiB of loose objects (17+ identical ~237 MB `.beads/*.jsonl` snapshots, loose:packed ≈ 1,800:1). Each attempt committed another snapshot, then died at `git push origin main` — `git pack-objects` walking the loose set exceeded the dispatch scope's documented `MemoryMax=12GiB` → memcg OOM SIGKILL. The host was never out of memory. |
| **Why 50 times** | One deterministic environmental condition × a zero-backoff re-dispatch loop. No retry could shrink the trigger — the kill left the loose set untouched, and each attempt's own snapshot commit grew it. |
| **Classification** | INFRASTRUCTURE under `docs/crash-response-guide.md`'s taxonomy. SERVICE_FAILURE, WORKFLOW, and CODE_DEFECT are each excluded on primary evidence (§7). |
| **Mechanism confidence** | **MEDIUM-HIGH.** The Aug-12 kernel record does not exist (system journald begins 2026-08-15 19:46 EDT), so the kill signal is never named for this date. The mechanism rests on: uniform exit signatures, the abrupt-truncation death shape, the crashes stopping exactly when the bloat was removed, kernel-proven proof of the same kill class on Aug-14 (bf-4x12ec) and Aug-16 (bf-198ne), and a 1/17th-scale reproduction harness. |
| **Task outcome** | Completed after the Aug-13/14 cleanup removed the trigger; closed 2026-08-17; remotes re-verified correct on the live repo (canonical report §8, final summary §2). |
| **Current state** | Trigger removed and held: `.git` **103 M** with a 90.93 MiB single pack, 0 garbage (**[LIVE]** re-check for this dispatch, 2026-09-06). All prevention layers deployed. |

---

## 2. What was running — version, build, and runtime context

Consolidated from [`bf-4yjq/version-info.md`](bf-4yjq/version-info.md) (every figure there is
a tool input/output captured inside the crash-era transcripts, unless marked otherwise).

**The dying process was `git`, not domain-check.** No domain-check binary was built or
executed during the storm — there is no build timestamp to capture and no artifact was
produced. The crash-era metadata that does exist:

| Item | Value at crash time (2026-08-12) |
|---|---|
| Workspace HEAD at first crash | `199b70c` "fix: improve timeout error detection and logging" — **305 commits ahead of origin/main** |
| origin/main = github/main | `63ba024` ("fix: remove unused time import…", 2026-08-09) — the merge-base of all three refs throughout |
| Divergence shape | *Ahead-of*, not forked-from: local `main` was a strict superset of both remotes |
| Backlog composition | 161 of ~317 unpushed commits touched `.beads/`; the only unpushed Go code delta was `199b70c` + `70b8aab` |
| `VERSION` | `5.8.0-test` (unpushed local bump; origin still at `0.1.7`) |
| Agent runtime | Claude Code CLI **2.1.227**, model **`glm-4.7`**, needle worker `claude-code-glm-4.7-lab-domain-check`, session `8446529e` |
| needle version | **0.3.1-era** (bound: ≥0.2.19, ≤0.4.0, from `.pre-*` backups; binary not preserved) |
| Remotes | `origin` **already pointed at Forgejo** by the first crash attempt — the remaining work was the backlog push and mirror config, not the remote URL |

**Critical caveat — the crash-time tree is unrecoverable.** The whole Aug-12 branch state was
discarded when the repo was repaired: purging the 237 MB `.beads/` blobs rewrote history
(`c27899f` squash, 2026-08-16). Every SHA above is absent from today's object store; the
figures survive only because the session transcripts recorded them.

---

## 3. The errors — complete inventory

Consolidated from [`bf-4yjq/error-analysis.md`](bf-4yjq/error-analysis.md), which searched
all 14.76 MB of transcripts, the structured event log, the worker log, and the 50 alert beads.

### 3.1 What needle recorded

225 worker-log lines: **50 ERROR / 6 WARN / 169 INFO**, the same shape every time:

```
2026-08-12T17:53:53.875682Z  INFO … exit_code=-1 outcome=Crash(-1)
2026-08-12T17:53:53.875703Z ERROR … agent crashed — releasing bead and creating alert
2026-08-12T17:54:02.643609Z  INFO … crash alert bead created alert_id=bf-276uk
```

| Outcome | Attempts | Record |
|---|---|---|
| crash | **50** | `exit_code=-1`, `outcome=Crash(-1)`, `signal_code=-1` |
| failure | 1 (attempt 2, 18:00:17Z) | `exit_code=1` — ended at `git add -A && git commit`, not a push |
| timeout | 4 (attempts 52–55, 20:40–21:11Z) | `exit_code=124` |
| success | 1 (attempt 56, 21:14:56Z) | `exit_code=0` — but the bead stayed open → `bead.orphaned`; closed five days later, 2026-08-17 |

(Outcome counts **[LIVE]**-re-tallied from `raw-logs/sessions-index.tsv` for this dispatch:
50 crash / 4 timeout / 1 success / 1 failure; 50 ERROR lines; 0 `"error"` fields in the
structured log.)

### 3.2 What the agent saw

**Nothing at any death point.** 49 of the 50 crash transcripts truncate immediately after an
unanswered `git push origin main` tool call; attempt 12's single push was denied in-band by a
permission prompt before git ran, and its transcript ends there. **No crash attempt ever
received a git result back from a push.** There is no panic, no stack trace, no OOM message,
no shell text — the transcript's truncation point *is* the death record, sitting 0–23 s
(median 14 s) before needle's classification line. Crash-attempt durations were 1.1–6.2 min
(median 2.5); each retry redid the whole task from claim to push before dying.

The only error text in the entire corpus is three recovered mid-task errors (missing `gh`
CLI ×2, a rejected push prompt, two benign git flag/path errors) — none related to the
deaths. Zero of the 1,071 structured records carries an `error` field.

> **Grep caveat:** naive `grep 503\|502` over the transcripts returns ~700 false positives —
> UUID/tool-call-ID substrings and the repo's own doc prose. Match on message shapes, not
> bare numbers.

### 3.3 Verified absences (the negative is the finding)

| Artifact | Status |
|---|---|
| Stack traces / panic dumps | **Zero exist in any source** — positive search of all 56 transcripts, not an extraction gap |
| Core dumps | None — `coredumpctl` earliest entry 2026-08-17 16:01:44 EDT, nothing from Aug-12 |
| Kernel / journald records | None — system journal first entry **2026-08-15 19:46:33 EDT**; the Aug-12 boot's records died with the Aug-14 16:39 reboot |
| `.beads/traces/bf-4yjq/` | Absent — the dispatches predate needle trace capture |
| stderr capture | Did not exist in this era; transcripts are stdout only |

### 3.4 The "signal −1" confusion — origin identified

All 50 alert beads carry the identical needle-generated body: `Exit code: -1 (signal -1)`.
Needle rendered its **unrecorded-signal sentinel** as though it were a signal number, in the
very artifact an investigator reads first. `exit −1` proves only "the agent process was
killed"; it is never SIGHUP (that would surface as 129) and never evidence of signal 9
specifically. This template wording — not any observation — seeded the early corpus's
SIGHUP-vs-SIGKILL debate. Alert-bead timestamps (~6.4 s after the classification line) are
creation times, not death times.

---

## 4. Infrastructure at crash time

Consolidated from [`bf-4yjq/infrastructure-status.md`](bf-4yjq/infrastructure-status.md).
Governing constraint: **the host had no memory or disk telemetry at all in August 2026**, and
its kernel logging did not yet persist.

### 4.1 What telemetry exists for 2026-08-12

| Source | Covers Aug 12? | Memory data? |
|---|---|---|
| Needle structured event logs, 6 workers (≈24 MB) | ✅ full day | **No** — 0 of 100,523 events carries a memory field |
| Needle plaintext worker log (`.log.2`, Aug-11→15) | ✅ full day | No |
| 56 crash-session transcripts | ✅ the storm itself | No — no attempt ran `free`, `du`, `df`, or `git count-objects` |
| System journal / dmesg / coredumpctl | ❌ all begin Aug-15 or later | — |
| `lab-health-collector` (mem/load every ~30 s) | ❌ starts 2026-08-15 23:53 EDT | 3 days too late |
| `.beads/logs/*` monitoring | ❌ begins 2026-09-01 | — |

### 4.2 Memory — a bound and an analogue, not a reading

- **The container that died was bounded at 12 GiB.** Dispatch scopes run under
  `needle.slice` with `MemoryMax=12GiB` (live-verified value; the crash-era needle was
  0.3.1-era, so the Aug-12 value is *documented*, not kernel-attested). Parent slice:
  `MemoryHigh=24 GiB`, `MemoryMax=32 GiB`; host 62 GiB.
- **The kernel-proven analogue two days later** (bf-198ne, 2026-08-16 00:27:35 EDT, the
  first journalled OOM): `git` reached **anon-RSS ≈ 11.7 GiB** and was SIGKILLed by its
  memcg — `oom-kill:constraint=CONSTRAINT_MEMCG … Killed process 3322486 (git)`.
  `git push`'s pack-objects was unbounded then; `pack.windowMemory` did not exist until
  2026-09-02. This is the mechanism bf-4yjq's 50 deaths are inferred to share.
- **The pressure was scoped to the cgroup, not the host.** Every OOM-kill constraint line in
  the entire journal (458 records) is `CONSTRAINT_MEMCG`; there is not one host-level OOM on
  this host. The 62 GiB host was never the exhausted resource.
- **The pressure source was the store being packed:** ≈18 GB `.git`, 17.2 GiB loose objects,
  17+ identical ~237 MB `.beads/*.jsonl` snapshots.

### 4.3 CPU — saturated all day; the storm ran in the calmest stretch

3,065 `fleet.cpu_saturated` events across all 6 workers, 05:37→23:54 UTC — every hour of the
day contains above-threshold samples. Inside the storm window, dispatch-time load₁ was
7.6–15.3 (median 10.06) against needle's 9-core view (≈1.1×) — **the mildest saturation of
the day** (midday peaked at load₁ ≈ 84.5). Needle's 82 `worker.launch.deferred` throttling
events that day all fall *outside* the window. **CPU pressure was a real all-day background
condition but did not cause the storm** — the death mechanism is a per-cgroup memory bound.

### 4.4 Disk, gateway, and system events — all excluded

| Hypothesis | Verdict |
|---|---|
| Disk exhaustion | **Excluded** — zero ENOSPC / "No space left" signals anywhere; no reading was ever taken, but the 18 GB repo sat under 5 % of the 444 GB disk |
| Inference gateway failure | **Excluded** — all 56 attempts sustained multi-turn LLM work right up to their deaths; zero message-shaped gateway/5xx hits in any source; `/health` → `ok` live |
| SIGHUP | **Excluded** — zero `sighup`/`hangup` hits in any source; a SIGHUP death would surface as 129 and leave shell-level text |
| Reboot / host event mid-storm | **Excluded** — the boot running the storm began 2026-08-12 01:15 EDT and survived until the Aug-14 16:39 reboot |

---

## 5. Timeline

### 5.1 Lead-up

| When (UTC) | Event |
|---|---|
| 2026-07-20 13:59 | Bead bf-4yjq created (P2) — git remote reconciliation task |
| pre-Aug-12 | `.beads/` tracking snapshots (~237 MB each) repeatedly committed — `.beads/` not yet gitignored, no size gate, no `pack.windowMemory` → repo grows to ~18 GB, 17.2 GiB loose |
| 2026-08-09 13:00 | Last real code change lands on origin (`63ba024`) — nothing code-side in the run-up |
| 2026-08-12 01:15 EDT | Boot begins — this boot hosts the entire storm and survives past it |
| 2026-08-12 05:36 → 16:31 | bf-31mno's crash loop: **348 deaths** on the same bloated repo |

### 5.2 The bf-4yjq storm (2026-08-12)

| When (UTC) | Event |
|---|---|
| 17:50:23 | First dispatch of bf-4yjq (attempt 1, HEAD `199b70c`, 305 ahead) |
| 17:53:53.875 | **First death** — `exit_code=-1`; alert `bf-276uk` created ~8.8 s later |
| 17:54 → 20:30 | **50 consecutive deaths**, all exit −1, mean interval ~188 s. HEAD advances 305 → 318 *during* the storm (each attempt commits another `.beads/` snapshot before dying at the push). Alert labels escalate `failure-count:1 → 4` |
| 18:00:17 | Attempt 2 fails with exit 1 (ended at `git add -A && git commit`) — the only non-signal failure |
| 20:30:38.310 | **Last crash death** (attempt 50) — the crash run ends |
| 20:40 → 21:11 | 4 command timeouts (exit 124, attempts 52–55); needle fires 3 `auto-split` escalations at failure counts 3/4/5 |
| 21:14:56 | Attempt 56 exits 0 — but the bead stays open (`bead.orphaned`, `status=blocked`); the *post-storm* orphan check, not a crash-time state |

### 5.3 Aftermath

| When (UTC) | Event |
|---|---|
| 2026-08-12 21:00 → 23:57 | bf-1s6c3's crash loop: **49 deaths** — the next shift of the same storm |
| 2026-08-13 → 08-14 | Repository packed/cleaned (18 GB → ~91–94 MB); divergence-analysis artifacts dated Aug-13 show bf-4yjq work proceeding normally |
| 2026-08-14 / 08-16 | Kernel-proven variants of the same mechanism: bf-4x12ec (gc-side), bf-198ne (push-side, `git` at 11.7 GiB anon-RSS, `CONSTRAINT_MEMCG`) |
| 2026-08-17 00:14 | bf-4yjq closed — "Git remote configuration successfully fixed and verified" |
| 2026-09-01 → 09-06 | Prevention stack hardened and re-verified (safe-git-gc bounds, `pack.windowMemory` repo+global, six systemd timers); repo holding at ~94–103 MB |

**Storm context.** The day's crash activity was a **rolling handoff between three
single-bead loops, not one concurrent storm**: bf-31mno (348) stops the hour before
bf-4yjq's first death; bf-1s6c3 (49) begins the hour after its last. The three loops never
share an hour. Fleet-wide 2026-08-12: 455–457 exit-−1 events across 9 distinct beads over
~18.5 h. Within bf-4yjq's own window its 50 deaths are the only exit-−1 events fleet-wide,
while the fleet stayed busy (236 dispatches across 6 workers).

---

## 6. Root cause analysis

### 6.1 Chain of events

1. **Accumulation (pre-Aug-12):** the bead-forge-era workflow repeatedly commits ~237 MB
   `.beads/*.jsonl` snapshots — `.beads/` is not gitignored, no pre-commit size gate exists
   → ~18 GB repository, 17.2 GiB loose objects vs 9.6 MiB packed (≈1,800:1 inverted ratio).
2. **Bound (constant):** every agent dispatch runs in a `systemd-run --user --scope` unit
   with `MemoryMax=12GiB`. Git operations that walk the whole object set must fit in it.
3. **Trigger (each attempt):** the retry loop commits another snapshot, then runs
   `git push origin main` → `git pack-objects` walks the 17.2 GiB loose set inside the
   12 GiB scope → memcg OOM SIGKILL. The agent never sees an error; the transcript truncates
   mid-call; needle records the sentinel −1 and releases the bead.
4. **Amplification (the loop):** zero-backoff re-dispatch re-enters step 3 ~3 minutes later.
   The kill leaves the loose set untouched and each attempt's snapshot commit grows it —
   **the storm grew the exact store its deaths were caused by.** Nothing opposed it.
5. **Terminus:** the bloat is removed Aug-13/14; the crash class stops instantly and never
   returns. The task completes and the bead closes 2026-08-17.

### 6.2 The two co-requisite defects

Neither alone was sufficient; both had to exist:

1. **Unbounded accumulation** — `.beads/` tracked, no size gate on what enters history.
2. **Unbounded operation** — no memory ceiling on git's packing path, inside a finite
   dispatch scope.

Fixing either alone leaves the crash reachable: a bounded gc over an unbounded store still
dies on the push; an unbounded gc over a clean store is merely slow. Both are now fixed
(§8).

### 6.3 Confidence

| Step | Confidence | Basis |
|---|---|---|
| Repository bloat was the trigger | HIGH | Contemporaneous metrics, uniform death signature, crashes stopping exactly when the bloat was removed, no recurrence since |
| The kill was a memcg OOM inside the dispatch scope | MEDIUM-HIGH | Inferred — the Aug-12 kernel record is unrecoverable. Rests on: uniform exit signatures, abrupt-truncation death shape, sub-15 s kill-to-classification latency, kernel-proven same-class kills on Aug-14/16, and a 1/17th-scale reproduction (`scripts/test-bf-4yjq-crash-condition.sh`, 6/6 assertions × 3 runs) |
| Death point = the push step | HIGH (per-run corroboration) | 50/50 uniformity on the identical command across sessions of varying length; caveat: "last recorded tool call" is not kernel attribution — the agent is not sampled between tool calls |

### 6.4 What this crash was not

- **Not a domain-check code defect** — no domain-check binary was built or executed during
  the storm; the last real code change predated it by 3 days. No investigation of this
  workspace has ever found a domain-check code defect.
- **Not a service failure** — the inference gateway served every attempt to its death.
- **Not a workflow failure** — each attempt was actively and correctly executing the bead's
  task when killed. The workflow system's *responses* (3 auto-splits, 50 alert beads) were
  consequences of the crash loop, not its cause.
- **Not a false positive** — the bead was genuinely open and the work genuinely unfinished
  at every death.
- **Not a host-wide OOM** — the host had memory to spare; the 12 GiB dispatch scope was the
  boundary that killed.
- **Not a task-content failure** — the crash exposure was a function of *when the bead was
  scheduled* relative to the bloat window, not of what git-remote work it was doing.

Note: the older reports' "BLOCKED, not actively executing its work" claim is **contradicted
by the per-run evidence** — all 50 crash attempts were mid-`git push` on the bead's own
task. The `status=blocked` reading comes from the post-storm orphan check at 21:14:59Z.

---

## 7. Crash classification — four-way framework applied

Framework: `docs/crash-response-guide.md` (FALSE_POSITIVE / SERVICE_FAILURE /
INFRASTRUCTURE / CODE_DEFECT), with repository bloat explicitly a member of the
INFRASTRUCTURE row since domchk-a54cc245.

**Verdict: INFRASTRUCTURE** — matching the guide's repository-bloat criteria: `exit_code -1`
+ fixed-cadence re-dispatch deaths + `.git` ≫ 5 GB (≈18 GB, ≈1,800:1 loose:packed). The
same verdict is recorded independently in the classification deliverable
([`crash-investigations/bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md`](../crash-investigations/bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md))
and the error inventory (§3 above).

| Alternative | Excluded because |
|---|---|
| SERVICE_FAILURE | No HTTP 503/502, no gateway/unreachable text, no network error string anywhere in 14.76 MB of transcripts or in all Aug-12 needle telemetry; gateway `/health` → `ok` live. A local resource event, not a dependency |
| CODE_DEFECT | No domain-check binary built or executed; deaths are git plumbing under the dispatch scope; the agent process never got to report anything |
| WORKFLOW / FALSE_POSITIVE | Bead genuinely open, work genuinely unfinished at every death; auto-splits and alert beads are downstream consequences |
| Host-wide memory exhaustion | No host-level OOM ever recorded on this host (all 458 journalled constraint lines are `CONSTRAINT_MEMCG`) |
| CPU saturation | Background condition all day, but the storm ran in the day's least-saturated hours and the mechanism is a memory bound, not a load effect |
| Disk exhaustion | Zero ENOSPC in every source |
| SIGHUP | Zero evidence; wrong signature (no shell text, no exit 129) |

---

## 8. Evidence inventory and provenance

**Primary sources — all committed** (extraction: domchk-495041ac, 77fac01):

| Artifact | What it is |
|---|---|
| `bf-4yjq/raw-logs/bf-4yjq-crash-sessions-2026-08-12.tar.gz` | 56 per-run agent session transcripts (14.76 MB uncompressed), byte-exact, 1:1 with dispatches. Recovered from `~/.claude/projects/` — a location no earlier inventory checked — and now the only copy |
| `bf-4yjq/raw-logs/needle-events-2026-08-12-bf-4yjq.jsonl` | All 1,071 bf-4yjq records from the structured needle event log (force-added past the repo's `*.jsonl` ignore rule, on purpose) |
| `bf-4yjq/raw-logs/needle-worker-log-bf-4yjq-slot2.log` | The 225 raw worker-log lines from rotation slot `.log.2` |
| `bf-4yjq/raw-logs/sessions-index.tsv` + `MANIFEST.sha256` | Per-attempt outcome/exit/duration/gap index and byte pinning |

**Secondary (derived, live-verified):** the four sibling documents linked in the header;
the canonical report
[`crash-investigations/bf-4yjq-crash-investigation.md`](../crash-investigations/bf-4yjq-crash-investigation.md);
the artifact catalog
[`crash-investigations/bf-4yjq-artifact-catalog-2026-09-06.md`](../crash-investigations/bf-4yjq-artifact-catalog-2026-09-06.md);
the cleanup verification
[`crashes/bf-4yjq-cleanup-verification.md`](../crashes/bf-4yjq-cleanup-verification.md).

**Unrecoverable:** the crash-time tree (history rewritten by the Aug-16 squash), any Aug-12
commit object, kernel OOM records, core dumps, stderr, the crash-era needle binary, and the
Go toolchain version (no attempt ran `go version`).

Re-derivation commands for every figure above live in each sibling's §8/§11
("Provenance — re-derive everything"); this document intentionally does not duplicate them.

---

## 9. Actionable recommendations

### 9.1 Deployed and verified (the two fix layers)

| Layer | Control | Status (**[LIVE]** re-checked 2026-09-06 unless noted) |
|---|---|---|
| **A — stop the accumulation** | Whole-`.beads/` gitignore (+ repo-wide `*.db`/`*.jsonl`); 0 tracked `.beads/` files | ✅ deployed — the standing bloat repaired, `.git` 103 M / 90.93 MiB pack / 0 garbage |
| A | 10 MB pre-commit gate (`.git/hooks/pre-commit`, per-clone) | ✅ active in this clone |
| A | Daily repo-health timer alerting >500 MB loose | ✅ among the 6 `domain-check-*` user timers, all armed |
| **B — bound the operation** | `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1`, repo-local **and** box-global — covers bare `git gc` **and** the `git push` pack path (bf-198ne) | ✅ `./scripts/setup-git-gc-config.sh --verify` → exit 0, worst case ≈3 GiB per pack run inside a 12 GiB scope |
| B | `safe-git-gc.sh` as the sanctioned cleanup path, `--verify` fail-closed | ✅ deployed (guarded since 2026-09-02) |
| Detection | Crash-pattern detector, surge threshold **3 crashes / 5 min** | ✅ `./scripts/crash-pattern-detection.sh --quiet` → exit 0 |

The recurrence risk is **conditional on the trigger class, not this event**: a clean repo
plus bounded git leaves the failure nothing to act on. Keep both layers enforced — they are
mechanical guards, not conventions.

### 9.2 Open items (owned elsewhere — do not re-implement here)

1. **Uncommitted detector improvements** — `scripts/crash-pattern-detection.sh`'s window
   filtering, ≥10/h storm-rate signal, and DEGRADED semantics are live on this box and run
   under the monitoring timer, but no commit carries them; a fresh clone would not have
   them. Needs a commit owner.
2. **Alert backlog** — bf-4yjq attracted 132 title-matching beads (45 open / 7 in_progress
   at the 2026-09-06 recount). A one-time bulk-close of alerts whose targets are already
   resolved is warranted; until then every new bf-4yjq dispatch pays the dedup tax
   (`git log --grep <bead-id>` + read the target bead before investigating).
3. **Stale documents** still repeating the superseded "9 crashes / ~17-minute intervals"
   figure without a banner — refresh owned by domchk-7625a5cc.
4. **Evidence retention** (gap G-8): retain kernel OOM events and journald ≥30 days, rotate
   rather than overwrite session traces, stamp UTC — the Aug-12 mechanism had to be
   established by inference because none of this existed. Full gap inventory:
   [`crash-prevention-requirements.md`](../crash-prevention-requirements.md) (G-1…G-13).
5. **Scope-level memory telemetry** (gap G-10): host-wide monitoring cannot see the
   per-dispatch 12 GiB boundary that actually kills; scope-level pressure is the signal to
   alert on.
6. **Retry-loop backoff** (gap G-11, needle-side): zero-backoff re-dispatch amplified one
   kill into 50 and then into dozens of phantom investigations.

### 9.3 Triage rule this crash teaches

This crash is **not an event you can re-run — it is a condition you can re-create.** Any
future spike of sub-3-minute exit-−1 re-dispatch deaths is the signature of an
environmental kill, not a code path: triage at the environment level (repo size, scope
memory, load) before any per-bead debugging, and derive scale from
`.beads/checkpoint/forensic.jsonl` — never from the alerts an investigator happens to find.

---

## 10. Superseded-claims register

Older bf-4yjq documents remain on disk (their telemetry is cited above where valid) but
these specific claims are superseded. Do not re-cite them:

| Superseded claim | Correction |
|---|---|
| 9 crashes at ~17-minute intervals | **50 crashes at ~3.1-minute intervals** (canonical report; the 9 alert IDs are a verified subset of the 50) |
| "signal −1 = SIGKILL (signal 9)" as an observed fact | −1 is needle's **sentinel**; the killing signal was never recorded for Aug-12. "SIGKILL-class" is correct only as inference |
| exit −1 indicates SIGHUP | A SIGHUP death surfaces as 129; zero SIGHUP evidence exists in any source |
| "BLOCKED, not actively executing its work" at crash time | All 50 crash attempts were mid-`git push` on the bead's own task; the `blocked` reading is the post-storm orphan check |
| "No raw session evidence survives" | The 56 per-run transcripts **do** survive (committed 77fac01) — earlier inventories never checked `~/.claude/projects/`. What remains absent is kernel/process-level telemetry, not all heartbeats (454 `heartbeat.emitted` records survive) |
| "1.7 GB" post-cleanup repo size | Intermediate state; the gc evidence and every live verification since give 91–103 MB |
| "455-event, 6-bead workspace-wide storm" | Whole-day view; the sharper statement is three zero-overlap single-bead loops (bf-31mno 348 → bf-4yjq 50 → bf-1s6c3 49), 9 distinct beads |
| bf-198ne (Aug-16) as "the push-side variant" of a gc-side event | The Aug-12 storm was itself uniformly push-side — bf-198ne is the same mechanism later kernel-proven, not a later variant |

---

*Synthesized for dispatch domchk-577a6273, 2026-09-06, from the four committed gather layers
(`docs/crash/bf-4yjq/`) plus the canonical record. Checks marked **[LIVE]** were executed for
this document on the working repo (50/4/1/1 outcome tally from `sessions-index.tsv`; 50 ERROR
lines; 0 `"error"` fields; `.git` 103 M, 90.93 MiB single pack, 0 garbage) — not quoted from
an earlier record.*
