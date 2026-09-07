# bf-1s6c3 Investigation Data Bundle

**Producing bead:** domchk-a2f6aabd (claude-code-glm-5.3-flash-lab-roam-9), 2026-09-06
**Subject bead:** bf-1s6c3 — "Create merge commit reconciling Forgejo and GitHub histories" (CLOSED)
**Purpose:** the data package for the analysis/classification phase (downstream bead
domchk-63805e42, which this bead blocks). Every figure below was re-run live on 2026-09-06;
nothing is cited from a prior worker's notes without re-verification.

Primary catalog: [`docs/crashes/bf-1s6c3/README.md`](bf-1s6c3/README.md) (extraction bundle,
committed af5ea6b). This file adds what the dispatch specifically asked for and the catalog
does not carry: the provenance of the **named** crash timestamp, the byte-exact window around
the real kill, and the log-source availability matrix for 2026-08-12.

## 1. The named timestamp is an alert time, not a kill time

The dispatch (and every downstream consumer) names **2026-08-12T22:04:12.524613796+00:00**.
That string is the `Timestamp` field of the crash-report description in alert bead
**bf-l3t8x** ("ALERT: Agent crash on bead bf-1s6c3") — re-verified live today via `bead show
bf-l3t8x`. It does **not** appear anywhere in the needle event log; the nearest needle record
is 9,430 ns earlier. Provenance chain, all UTC:

| Event | Timestamp | Source |
|---|---|---|
| Real kill — `agent.completed`, `exit_code=-1`, `duration_ms=172839` | 22:04:06.124743603Z | needle log line 12928, seq 4907 |
| `outcome.classified` → `crash` | 22:04:06.125925844Z | line 12931, seq 4910 |
| heartbeat `HANDLING_RELEASE_DONE` | 22:04:12.524604366Z | line 12937, seq 4916 |
| **Named timestamp (alert description `Timestamp` field)** | **22:04:12.524613796** | bf-l3t8x description; **kill + 6.40 s** |
| Alert bead `created_at` | 22:04:12.531629767Z | bf-l3t8x metadata; named ts + 7.0 ms |
| `bead.released` (`release_success`) + `outcome.handled` (alerted) | 22:04:14.880194935Z | line 12938, seq 4917 |

So the named timestamp is the crash handler's post-release bookkeeping clock reading
**6.40 s after the actual kill** — the same alert-after-kill offset documented for bf-173o7e
(8–120 s) and bf-4x12ec. It is one of the 71 identical `exit_code=-1` deaths, not a distinct
crash event. The dispatch's "exit code -1 signal" and "workspace ." fields come from the same
alert description (`Agent: claude-code-glm-4.7`, `Workspace: .` = `/home/coding/domain-check`).

## 2. Exit code classification

- **exit_code −1** is needle's sentinel for an agent process killed by a signal, not a signal
  number. Needle's own `outcome.classified` events record it explicitly for Aug-12, so no
  death-event bracketing is needed for this date.
- Per `docs/crash-response-guide.md`: exit −1 → **INFRASTRUCTURE EVENT**.
- Attributed mechanism (established RCA, not re-derived here): memcg-OOM during git operations
  on the then-18 GB repository (17.16 GB loose objects from 17+ committed 237 MB
  `.beads/*.jsonl` snapshots). Caveat the classifier should carry: **no kernel record exists
  for 2026-08-12 itself** — journald starts 2026-08-15 19:46 EDT and the Aug-12 kernel records
  were lost to the 2026-08-14 16:39 EDT reboot — so the OOM attribution rests on the needle
  exit codes, the repo state, and later kernel-proven identical mechanisms (bf-4x12ec
  2026-08-14, bf-198ne 2026-08-16).

## 3. Byte-exact window around the kill (attempt 10)

Raw source: `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl`
(3,589,712 B, sha256 `3a487dc3139a2785f3ec2917f29d00121abbe109a95793163ad0a3c27352b28e` —
matches extraction-bundle README §1, re-checked today). Line numbers are from that file.

```
22:01:13.039248450Z  agent.dispatched        seq 4904  prompt_len=70670 prompt_hash=sha256:aaa143d4…  (attempt 10)
22:04:06.124743603Z  agent.completed         seq 4907  exit_code=-1 duration_ms=172839   <- THE KILL
22:04:06.125925844Z  outcome.classified      seq 4910  exit_code=-1 outcome=crash
22:04:10.037265359Z  heartbeat HANDLING_FLUSH_DONE    seq 4913
22:04:12.524604366Z  heartbeat HANDLING_RELEASE_DONE  seq 4916   <- named ts lands 9,430 ns after this
22:04:12.524613796   [bf-l3t8x alert description Timestamp — no needle event]
22:04:12.531629767Z  [bf-l3t8x created_at]
22:04:14.880194935Z  bead.released           seq 4917  reason=release_success
22:04:14.880202059Z  outcome.handled         seq 4918  action=alerted outcome=crash
22:04:17.345125710Z  bead.claim.succeeded    seq 4923  -> attempt 11
22:04:17.349403098Z  fleet.cpu_saturated     seq 4929  load_average=11.0 core_count=9 threshold=0.8
22:04:17.358828679Z  agent.dispatched        seq 4932  prompt_len=70670  (attempt 11)
```

Attempt 10 = transcript `cdcdf9a9-96cd-43a2-ba9b-7f345e00338c` (row 10 of
`docs/crashes/bf-1s6c3/sessions-index.tsv`, session start 22:01:14.329Z, last issued command
`git push origin main`). Attempt 11 = `6ce3a44c-8220-412b-9fd0-e9526a32c720`, started
22:04:18.565Z. Crash→retry gap: **11.2 s**; the storm did not self-heal — 65 more dispatches
followed.

## 4. Log-source availability for 2026-08-12 (absence re-checked live today)

| Source | Oldest data | For Aug-12 |
|---|---|---|
| `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl` | 2026-08-12 | **available** — the only primary witness; still on disk (fabric-prune broken since 2026-08-17) |
| `.beads/logs/` | 2026-09-01 (`crash-pattern-alerts.log`, `repo-health.log`) | **absent** — the monitoring stack that writes these did not exist yet |
| system journald | 2026-08-15 19:46 EDT | **absent** — Aug-12 kernel records lost to the 2026-08-14 reboot |
| `coredumpctl list --since 2026-08-12 --until 2026-08-13` | — | **"No coredumps found."** |
| Aug-12/13 session transcripts (76) | 2026-08-12 17:43–20:45 local | **available** — in the committed tarball |

## 5. Artifact catalog

**Committed extraction bundle** — `docs/crashes/bf-1s6c3/` (af5ea6b, produced by
domchk-fcac734a), re-verified today: `sha256sum -c MANIFEST.sha256` → all 5 OK; extracts
945 + 513 events (0 unparseable per the sibling verification); `sessions-index.tsv` 76
attempts; `bf-1s6c3-crash-sessions-2026-08-12_13.tar.gz` → 76 members.

**Archived investigation docs** — `docs/archive/crash-investigations/` (moved there by
a883044; **the path in bf-1s6c3's closure notes, `docs/crash-investigation-summary-bf-1s6c3-2026-09-01.md`,
is stale** — the file is at `archive/crash-investigations/crash-investigation-summary-bf-1s6c3-2026-09-01.md`).
Seven bf-1s6c3 docs live there, including `crash-context-bf-1s6c3-complete.md` (quotes the
bf-l3t8x alert JSON; source of the named timestamp) and `crash-root-cause-analysis-bf-1s6c3-final.md`.

**Untracked on this clone** — `docs/crashes/bf-1s6c3-artifact-extraction-verification-2026-09-06.md`
is domchk-60c12286's deliverable (its bead is closed); it is that bead's verification record,
left untracked — not committed canon.

**Live bead state** — alert bead bf-l3t8x still `InProgress` (created 2026-08-12T22:04:12.531629767Z).
Its Notes carry two **known-stale** claims a classifier should not absorb: "663 commits ahead of
origin/main" (phantom divergence — 0/0 everywhere as of 2026-09-06, see
`docs/branch-divergence-analysis.md`) and "bf-1s6c3 is still open" (it closed 2026-08-16).
bf-l3t8x itself is left untouched; alert closure belongs to its own closure chain.

## 6. Storm context — where this death sits

76 dispatches to bf-1s6c3, 21:31:27.663Z Aug-12 → 02:01:27.732Z Aug-13: **71 × exit −1,
4 × 124 (600 s timeouts), 1 × 0**. Every attempt's last issued command was `git push origin
main` (or `git push github main`) — the deaths occurred during git push/merge on the bloated
repo, i.e. the same push-side pack-objects mechanism later kernel-proven as bf-198ne, four
days before `pack.windowMemory` bounds existed (added 2026-09-02). A `fleet.cpu_saturated`
event (load 11.0 on 9 cores) fired 0.2 s before the attempt-11 dispatch. The one `exit 0` was
the auto-split run, not a completed merge; the real merge commit 42a7b07 was created mid-storm
and orphaned by the 2026-08-16 pre-squash history reset, and bf-1s6c3 closed 2026-08-16
(citing the now-dead SHA 7dd79eb). Remediation layers (gitignore, 10 MB pre-commit gate,
pack.windowMemory/deltaCache/threads bounds, daily timers) are in place and re-verified
2026-09-06 (63c3125).

## 7. Initial classification hints (for domchk-63805e42)

1. **INFRASTRUCTURE EVENT** — exit −1, one of 71 identical deaths in a rolling single-bead
   crash loop; mechanism = memcg-OOM during git operations on repo bloat. Not a domain-check
   code defect (no investigation has ever found one), not a service failure, not
   max-turns exhaustion.
2. **Not a false positive at the bead level in the alert's own terms** (the agent really was
   killed mid-task), but the *task* was completed after the storm: repo cleaned (18 GB →
   ~94 MB, holding), bead closed. Post-closure alert churn on bf-1s6c3 (the 131-bead alert
   pool) is the known duplicate-alert pattern.
3. Use the **kill time 22:04:06.124Z** for any windowing, not the alert-descriptor timestamp
   22:04:12.524 (kill + 6.40 s).
4. Treat as unreliable: the closure-note path `docs/crash-investigation-summary-bf-1s6c3-2026-09-01.md`
   (stale — archived), bf-l3t8x Notes (stale divergence + open-state claims), and
   `crash-context-bf-1s6c3-complete.md`'s "actual crash 21:36:51" row (that is the
   artifacts-summary bead's metadata timestamp; needle deaths in the window are explicit) and
   its unqualified "SIGKILL (9) by OOM killer" (no kernel record survives for Aug-12).
