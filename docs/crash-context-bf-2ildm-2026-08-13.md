# Crash context — bf-2ildm ("Extract GitHub-specific commits", 2026-08-13 alert storm)

Assembled 2026-09-07 by **domchk-fe9f7c5b**, the summary child of
**domchk-b049ea9e** ("Gather context about crashed bead bf-2ildm", Closed rev 8,
notes dated 2026-09-02; itself a child of alert bead **bf-o6vbwl**). This is a
context/handoff document, not a new investigation: it relays what
domchk-b049ea9e found, corrects it against the records that supersede it, and
re-verifies the load-bearing figures live. Artifacts live in
`docs/crashes/bf-2ildm/` (domchk-ea755548); the current root-cause record is
`docs/investigations/bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md`.

**Bottom line — read the FALSE POSITIVE label at the level where it is true.**
The 2026-09-02 context investigation (domchk-b049ea9e, with
`docs/investigations/bf-2ildm-final-investigation-report-2026-09-02.md`)
concluded "bead bf-2ildm DID NOT CRASH — FALSE POSITIVE." The **kill** half of
that is superseded (2026-09-07): bf-2ildm was killed **38 times** on 2026-08-13
(exit −1, 13:37:24Z → 15:53:28Z) by the workspace's repository-bloat regime —
memcg-OOM inside needle's 12 GiB per-dispatch scope. The **alert** half stands:
every alert was FALSE_POSITIVE *at the alert layer* — each fired at a real kill,
but implied lost work, and no work was lost (the target succeeded 2026-08-16,
exit 0, 85,327 ms, and closed 22:44:38.873Z). The dedup gate confirms the
alert-level verdict: `./scripts/alert-deduplication.sh check bf-66sw7c` →
"DUPLICATE: crash target bf-2ildm is already resolved" (re-run live 2026-09-07).

## 1. Work context — what bf-2ildm was

| Field | Value |
|---|---|
| Title | Extract GitHub-specific commits |
| Type / priority | task / P2 |
| Created | 2026-08-13T11:12:57.942289666Z |
| Status now | **Closed** (rev 7; classification appended by domchk-e05fa5a8 @ e3a8820) — verified live 2026-09-07 |
| Task | Third step of a GitHub/Forgejo branch-divergence analysis chain: `git log <common-ancestor>..<github-branch>` to list commits unique to GitHub; capture the count plus each commit's SHA, author, date and message; save to a temporary state file for the subsequent beads |
| Scope | ONLY GitHub-specific extraction — no Forgejo commits, no final analysis; depended on the second child bead completing first |
| Agent / model | `claude-code-glm-4.7` (provider `zai`, model `glm-4.7`) |
| Needle worker / session | `claude-code-glm-4.7-lab-domain-check` / session `e29942f7` |
| Workspace | `/home/coding/domain-check` (.) |
| Dispatch template | `pluck` / `pluck-default`, prompt_len 70,745 |

The task itself — a `git log` between two remotes — has no relationship to any
crash mechanism. bf-2ildm is the *victim* of the same Aug-13 kill loop as
bf-1ea4g (56 kills), bf-4k2ws (55) and bf-1s6c3 (22), not its cause; no code
defect has ever been found in this workspace's application code.

domchk-b049ea9e's notes also relay the 09-02 corpus narrative that the agent
"created 4 focused child beads" and that all were properly configured — carried
here as-recorded, not independently re-derived.

## 2. The FALSE POSITIVE determination — as recorded, and as it stands

**What domchk-b049ea9e recorded (2026-09-02):** bead bf-2ildm "DID NOT CRASH";
it completed successfully with exit 0; the reported exit −1 was "placeholder
data, never validated"; status CLOSED (2026-08-16); all acceptance criteria
met. Cause claimed: five systematic bugs in crash-alert generation — (1)
premature alert generation, (2) placeholder exit code −1, (3) no bead-status
validation, (4) no timestamp validation, (5) no duplicate prevention (21+
alerts).

**How that reads after the 2026-09-07 correction** (superseding RCA §3–4,
retrieval bundle):

| Recorded claim | Standing |
|---|---|
| "Alert was a FALSE POSITIVE" | **Stands at the alert layer** — every alert implied lost work and none was lost; dedup → DUPLICATE, target resolved |
| "bead bf-2ildm DID NOT CRASH" | **Superseded** — 38 real exit −1 kills, per `docs/crashes/bf-2ildm/attempt-index.tsv` |
| "exit −1 is placeholder data, never validated" | **Superseded** — exit −1 is needle's died-without-exit-code sentinel; 38 real kills produced it |
| "exit 0 is the actual exit code of the reported run" | **Superseded provenance** — the trace's exit 0 (85,327 ms) is the 2026-08-16 retry; the single-slot trace was overwritten, so no crash-era trace survives |
| "alerts fired 3+ days before completion (physically impossible)" | **Reversed** — alerts fired 7.7–29.4 s after real kills, mid-task, while the bead was open; they became stale three days later when the target resolved |
| "no closed-bead filter / no dedup / no cooldown / no exit-code validation" | **Stands as alert-layer defects** — real regardless of the kill question; fixed 2026-09-02, re-verified 2026-09-07 (`test-crash-alert-fixes.sh` 13/13, `test-closed-bead-filter.sh` 7/7) |
| "21+ duplicate alerts for the same resolved crash" | **Reframed** — the storm's 38 alert beads map 1:1 to 38 distinct kills (no duplicates or orphans *within* the storm); what made them feel duplicated is re-processing after the target resolved |

The 09-02 report was internally tense on exactly the decisive point: its §4
already listed "6.0G `.beads/` … OOM when loaded" among the original crash
conditions while its §1 claimed no crash occurred. That tension is what the
superseding RCA resolved in favor of the kills.

## 3. Evidence for the false-positive conclusion — corrected reading

Each evidence item domchk-b049ea9e cited, with its current standing:

1. **"Exit code 0 confirmed from trace metadata" vs the reported −1.** The
   record is real; its provenance was misread. `.beads/traces/bf-2ildm/` is a
   single-slot archive, and the slot holds only the successful 2026-08-16
   retry (`exit_code: 0`, `outcome: "success"`, `captured_at:
   2026-08-16T22:28:44Z`, 85,327 ms). Reading that surviving record as the
   record *of the reported crash* is what made the −1s look fabricated next to
   a 0. `crash-classifier.sh bf-2ildm` still reports exactly this
   ("trace slot does not describe the incident run", provenance unverified,
   classification UNKNOWN) — re-run live 2026-09-07.
2. **"All acceptance criteria met; child beads created; bead closed
   successfully."** Stands. The work was not lost: the 2026-08-16 retry
   succeeded and the target closed at 22:44:38.873Z (rev 7 since carries the
   classification append). This is the durable core of the false-positive
   finding.
3. **"Repository clean, no uncommitted changes."** Stands for the eventual
   successful attempt's outcome.
4. **"NO crash logs, NO core dumps, NO stack traces."** True but
   non-probative — a documented absence, not proof of no crash. journald on
   this host has a single boot beginning 2026-08-15 19:56:33 EDT, so no
   Aug-13 kernel record survives for any bead; `.beads/logs/` telemetry starts
   2026-09-02. The crash record that does survive is the worker log's 38
   exit −1 `agent.completed` rows (919 bf-2ildm records, bundled verbatim).
5. **"Alert generated 3+ days BEFORE completion — physically impossible."**
   Reversed by the corrected timeline (§4): kill 08-13T14:40:29.551762Z →
   post-kill heartbeat 14:40:42.628685Z (+13.1 s) → alert bead bf-66sw7c
   created 14:40:42.642439Z (+14 ms after the stamp) → target closes
   08-16T22:44:38Z. An alert generated at a kill, while the bead is open and
   mid-task, is what the alert system exists to produce; it became a false
   positive retroactively.

**Kill-regime context (why the kills happened, one paragraph):** 38 of 43
attempts died exit −1, all mid-task (98–309 s), on a fixed ~2–5 min
re-dispatch cadence across 2 h 16 m — the signature of the era's
repository-bloat regime (~18 GB loose objects from 17+ identical 237 MB
`.beads/*.jsonl` snapshots still tracked, plus a 422-commit unpushed backlog).
Kill regime confidence HIGH; precise mechanism MODERATE (assigned by regime
match — no Aug-13 kernel record survives). The regime is repaired and holding
(live 2026-09-07: `.git` 104 MB, `check-repo-health.sh` exit 0, pack-memory
bound ≈3072 MiB worst case within the ceiling, 0 unpushed, 0 tracked `.beads/`
files — superseding RCA §5).

## 4. Timestamps and verification data

All times UTC. Bundle figures re-read from `docs/crashes/bf-2ildm/` by this
bead; live checks re-run 2026-09-07.

| Timestamp (UTC) | Event |
|---|---|
| 2026-08-13T11:12:57.942Z | bf-2ildm created ("Extract GitHub-specific commits") |
| 13:35:34.306Z (log L5766) | attempt-1 claim — the storm's first dispatch |
| **13:37:24.838Z** | **first kill** (exit −1, 110,309 ms); handled 13:37:37.298Z (L5790) |
| 13:37:24.838Z → 15:53:28.914Z | the **38 exit −1 kills** (attempts 1–38); the 14:00Z hour alone holds 15 consecutive kills (attempts 9–23) |
| 14:38:24.455Z (L6299) | attempt-19 claim |
| 14:40:29.551762Z | **attempt-19 kill** (exit −1, 124,759 ms) — the kill behind alert bf-66sw7c (ordinal correction: attempt **19**, not 20; bf-26r8bi is 20, not 21) |
| 14:40:42.628685Z | bf-66sw7c's carried crash stamp = post-kill handling heartbeat, **+13.1 s** after the kill |
| 14:40:42.642439Z | alert bead **bf-66sw7c** created (+14 ms after the stamp) |
| 14:40:46.615684Z | attempt-19 released, `handled: alerted` |
| 15:36:14.415Z / .423Z | **bf-o6vbwl** carried stamp / created — ledger row 34 of 38, the alert bead this context chain hangs off |
| 16:03:48Z → 16:35:06Z | attempts 39–42: exit-124, each exactly 600,000 ms (600 s cap) — `handled: deferred`, **no alerts** |
| 16:35:26Z / 16:35:40.675Z | attempt 43: exit 1 at 19 ms (died before work started) → bead **quarantined** (`failure_count: 5`, threshold 5) |
| 2026-08-16T22:28:44.172Z | **successful retry: exit 0, 85,327 ms** — the only record the single-slot trace retains |
| 2026-08-16T22:44:38.873Z | bf-2ildm **closed** — the fact that makes every alert stale |

**Re-verified live by domchk-fe9f7c5b, 2026-09-07:**

| Check | Result |
|---|---|
| `bead show bf-2ildm` | Closed, rev 7 (updated 2026-09-07T17:47:17Z by the classification append) |
| `./scripts/alert-deduplication.sh check bf-66sw7c` | "DUPLICATE: crash target bf-2ildm is already resolved" |
| `./scripts/crash-classifier.sh bf-2ildm` | UNKNOWN — "Insufficient data to classify"; "trace slot does not describe the incident run"; trace provenance unverified; slot holds the 2026-08-16T22:28:44.172164374Z run, `exit_code 0` |
| `attempt-index.tsv` row 19 | claim 14:38:24.455725579Z (L6299) → kill 14:40:29.551762239Z, exit −1, 124,759 ms → alerted → bf-66sw7c, gap 13.1 s |
| `crash-alert-ledger.tsv` | bf-66sw7c and bf-o6vbwl rows match the stamps/creation times above; 38 rows + header, 1:1 with the kills |

## 5. Sources

- domchk-b049ea9e notes (`bead show domchk-b049ea9e`, Closed rev 8) — the
  findings summarized here
- `docs/investigations/bf-2ildm-final-investigation-report-2026-09-02.md`
  (domchk-efbd3a4d chain) — the evidence base domchk-b049ea9e cited
- `docs/investigations/bf-2ildm-root-cause-determination-domchk-a863a1f9-2026-09-07.md`
  (ef960d2) — the superseding two-level verdict; §4 documents why the 09-02
  conclusion was wrong and what survives
- `docs/crashes/bf-2ildm/` retrieval bundle (domchk-ea755548, 1f56d9c):
  `attempt-index.tsv`, `crash-alert-ledger.tsv`, `README.md`,
  `trace-archive-current-state.json`
- Live store and scripts re-run 2026-09-07: `bead show bf-2ildm`,
  `scripts/alert-deduplication.sh check bf-66sw7c`,
  `scripts/crash-classifier.sh bf-2ildm`
- Sibling context precedent: `docs/crash-context-bf-1ea4g-2026-08-13.md`
  (domchk-cf6855ad)
