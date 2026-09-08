# bf-4yjq crash evidence — source inventory and reconciliation

**Verifying bead:** domchk-7a34eb37 · **Date:** 2026-09-07
**Canonical report under verification:** [bf-4yjq-crash-investigation.md](bf-4yjq-crash-investigation.md)
(verified byte-identical to the cited commit `db3f1f2` — tracked copy at HEAD is unchanged)
**Named crash instant in the task:** 2026-08-12T18:18:20Z = alert bead `bf-29rca`
(created 18:18:20.472Z) — the 6th of the 50 crashes, one bead per kill; not a distinct event.

## Verdict

| Canonical claim | Status after this pass |
|---|---|
| 50 exit-code −1 crashes | **CONFIRMED** — three independent sources agree, alert IDs match 1:1 |
| Window 17:54:00–20:30:43 UTC | **CONFIRMED** for alert-bead creation; needle's own death stamps run 17:53:53.875–20:30:38.310 (~6 s earlier) |
| Cadence "~3.1-min (188 s)" | **CORRECTED to 191.9 s (~3.2 min)** — arithmetic denominator slip, conclusion unchanged |
| "No raw session evidence survives" (§3) | **SUPERSEDED for dispatch transcripts** — 56 per-run transcripts survive (archived 2026-09-06); still true for kernel logs / core dumps / trace capture |
| Storm table §4 (455 events, 6 beads) | **CONFIRMED exactly under its stated Aug-12-dated basis** — every bead count and window reproduces; but it is a one-day *slice*, not whole-storm totals (bf-31mno 434, bf-1s6c3 71) — see §1-A |
| Root cause: OOM during "git operations" | **REFINED: uniformly push-side** — all 50 crash runs' last Bash call is `git push`; bf-198ne is not the first push-side memcg OOM, it is the same mechanism already operating Aug 12 |

Every figure in this doc was re-derived from the raw sources by this bead (not copied from an
earlier report) and then re-verified end-to-end by the closing pass on 2026-09-07 — counts,
windows, the push census, the storm table, and the absence claims (`journalctl -k` for
Aug-12 → "-- No entries --"; oldest boot `52309698` first entry 2026-08-15 19:56:33 EDT;
`coredumpctl` earliest 2026-08-17; bf-4yjq Closed, store `updated 2026-08-17T00:14:14Z`).
The committing pass re-ran all of it live and confirmed: the 50-bead chronology (bf-29rca
6th at 18:18:20.472Z), live-log counts 225 / 1,071, the 50/4/1/1 `outcome.classified` split,
death stamps 17:53:53.875700862 → 20:30:38.310367540Z, all five archive files `sha256sum -c`
OK, 56 index rows, the 50/50 push census with all six description variants summing to 50,
§4's slices (455 = 350+50+49+4+1+1), byte-identity of the committed worker-log extract, and
the canonical report's HEAD blob `39418f1` = its `db3f1f2` blob.

## 1. Source inventory

### A. Bead store — `.beads/checkpoint/forensic.jsonl`

- ~25 MB, ~60,000 lines — a **live store that grows on every `bead sync flush-only`**, so
  line counts drift between passes (817 lines mention `bf-4yjq` at this pass; an earlier
  attempt recorded 58,792/815). The 50-bead set below is the invariant.
- **50 distinct alert beads** titled `ALERT: Agent crash on bead bf-4yjq`, all exit −1,
  `created_at` 17:54:00.249108Z (`bf-276uk`) → 20:30:43.715574Z (`bf-2n3ve`).
  Full 50-bead chronology in **Appendix A** below (re-derived by JSON parse, not grep).
- Also in the store: the storm table — re-derived 2026-08-12 crash-report beads:
  bf-31mno 350 (05:36–16:31), **bf-4yjq 50 (17:54–20:30)**, bf-2xygo 4 (21:18–21:28),
  bf-1s6c3 49 (21:36–23:57), bf-23n 1 (17:08), bf-5d18 1 (17:23) = **455 events, 6 beads,
  100 % exit −1** — matches canonical §4 bead-for-bead. **Scope note (added on the closing
  pass):** §4's stated basis is "records dated 2026-08-12", so its figures are a one-day
  slice of each storm, not whole-storm totals. Alert beads with the same title exist on
  adjacent days: **bf-31mno's full storm is 434** alerts (2026-08-11T15:55:59 →
  2026-08-12T16:31:52 — 84 of them on Aug-11), and **bf-1s6c3's full storm is 71**
  (2026-08-12T21:36:51 → 2026-08-13T01:24:13 — 22 more on Aug-13). **bf-4yjq's 50 is its
  complete storm** — no alert bead for it exists on any other day, so the headline count is
  not a slice. For whole-storm counts prefer 434 / 71 (corroborated by the bf-1s6c3
  corrected record: 76 dispatches / 71 kills); for "the Aug-12 day", §4's slices are exact.
- `.beads/crash-bf-4yjq-summary.txt` (14.5 KB, Sep-1-era file): records the **superseded**
  9-crash / ~17-min / "17:54–20:24" count. Four of its five sample timestamps match verified
  alert beads exactly (bf-2weev, bf-3b9rv, bf-1dzwv, bf-19qh7); its "1st: 17:54:33" matches
  none of the 50 exactly (nearest is bf-276uk at 17:54:00). Count superseded; per-stamp
  agreement retained.

### B. NEEDLE session logs (live sources + committed archive)

**Live** (still on disk, counts verified this pass):

| File | bf-4yjq content |
|---|---|
| `~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2` (134 MB, rotation slot `.2`) | 225 lines; 50 × `exit_code=-1 outcome=Crash(-1)`, 50 × `agent crashed`, 50 × `crash alert bead created`, 56 × `claim_auto`; plus 1 × Failure(1), 4 × Timeout(124), 1 × Success(0) |
| `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl` (3.6 MB) | 1,071 structured records; `outcome.classified` = **50 crash/−1, 4 timeout/124, 1 failure/1, 1 success/0** |

**Committed archive** — `docs/crash/bf-4yjq/raw-logs/` (extracted 2026-09-06 by
domchk-495041ac; `sha256sum -c MANIFEST.sha256` passes on all 5 files this pass):

| File | What it holds |
|---|---|
| `bf-4yjq-crash-sessions-2026-08-12.tar.gz` (3.5 MB) | **56 per-run agent session transcripts** (one per dispatch), recovered from `~/.claude/projects/-home-coding-domain-check/` — the location the canonical report's inventories never checked. All 56 first lines carry the `[needle:…:bf-4yjq:auto]` dispatch tag. |
| `needle-events-2026-08-12-bf-4yjq.jsonl` (288 KB) | Verbatim copy of all 1,071 structured events |
| `needle-worker-log-bf-4yjq-slot2.log` (94 KB) | Verbatim copy of the 225 worker-log lines (force-added past the repo-wide `*.log` ignore rule) |
| `sessions-index.tsv` | 56 rows — per-attempt outcome, exit code, dispatch/classified UTC, kill gap, transcript hash |
| `MANIFEST.sha256` | Pins all of the above |

Committed extract `docs/crash-analysis/bf-4yjq-needle-worker-log-extract.log` (225 lines) is
**byte-identical** to the bundle's worker-log copy (`cmp` clean). Live-vs-archive counts match
exactly (225 lines / 1,071 records), so the archive is faithful to the live sources.

### C. Resource / repo-health monitor logs — **NO Aug-12 coverage**

Every monitor log under `.beads/logs/` **postdates the storm by ~3 weeks**. Earliest entries:

| Log | First entry |
|---|---|
| `resource-metrics.log` | 2026-09-01T22:49:42Z |
| `repo-health.log` | 2026-09-01 |
| `crash-pattern-alerts.log` / `crash-monitor.log` / `resource-alerts.log` | 2026-09-02 |
| `git-gc*.log`, `service-*.log` | 2026-09-02 / 09-06 era |

There is **no instrument reading of 2026-08-12 anywhere in `.beads/logs/`**. The canonical
report's "contemporaneous telemetry records load average 15–17 … disk 84 % full" (§6 step 2)
traces to `docs/crash-analysis/bf-4yjq-system-state-snapshot-2026-09-01.txt` — a **Sep-1
capture**, not an Aug-12 reading. Downstream docs should cite it as post-hoc, not
contemporaneous.

### D. Confirmed absences (re-checked live 2026-09-07)

| What | Finding |
|---|---|
| Kernel / journald records for Aug-12 | **Unrecoverable** — single boot `52309698` starts 2026-08-15 19:56:33 EDT; `journalctl -k --since 2026-08-12 --until 2026-08-13` → no entries |
| Core dumps | `coredumpctl` earliest entry 2026-08-17 (pdftract) — nothing from Aug-12 |
| `.beads/traces/bf-4yjq/` | Absent — dispatches predate needle trace capture |
| Needle stderr capture for this era | None exists |

## 2. Count reconciliation — three timestamp sets for the same 50 deaths

| Set | Source | First | Last | Count |
|---|---|---|---|---|
| Death classification | needle `outcome.classified` | 17:53:53.875Z | 20:30:38.310Z | 50 |
| Alert-bead creation | forensic `created_at` (canonical report's basis) | 17:54:00.249Z | 20:30:43.716Z | 50 |
| Alert-created log line | worker log `crash alert bead created` | 17:54:02.643Z | 20:30:45.743Z | 50 |

Per kill the chain is: agent death → `outcome.classified` (worker log "handling agent
outcome") → alert bead `created_at` (~6 s later) → worker log "crash alert bead created"
(~2 s after that) → next `claim_auto` re-dispatch (~3 s later). The ~6 s alert-vs-death skew
is uniform, so every derived figure (count, span, cadence) is identical across sets. The
canonical report's window is the **alert-creation** window; needle's death stamps run ~6 s
ahead of it. Both are correct descriptions of their own event.

**Supersession order for crash counts:** forensic.jsonl / needle raw logs (**50**,
confirmed) > canonical report `db3f1f2` (50, correct on count and window) >
`.beads/crash-bf-4yjq-summary.txt` and the Aug-14–Sep-1 reports (**9 at ~17 min** —
retracted; sampled alert beads only).

## 3. Corrections and refinements

1. **Cadence: 188 s → 191.9 s.** The report's 188 s is span ÷ 50 events; 50 events have
   **49 gaps**: 9,403 s ÷ 49 = **191.9 s (~3.2 min)**. Median gap 155 s, range 75–576 s.
   Non-material — the report's own precision ("~3.1 min") still reads correctly, but the
   exact figure is 192 s.
2. **"No raw session evidence survives" is superseded for dispatch transcripts.** §3's
   statement remains true exactly as scoped (no core dumps, no stack traces, no Aug-12
   heartbeats, no trace capture) but the 56 per-run session transcripts were recovered
   2026-09-06 from `~/.claude/projects/` and archived in `docs/crash/bf-4yjq/raw-logs/`.
   Any downstream doc repeating the blanket no-survivors line should cite the bundle.
3. **Mechanism: uniformly push-side.** Independently re-verified from the transcripts this
   pass: **50/50** crash runs' last Bash tool call is a `git push` to Forgejo — and in all
   50 the last command is literally a `git push` invocation, not merely a push-described
   call (description variants 29×"Push local commits to Forgejo origin", 7×"…to Forgejo",
   6×"Push to Forgejo origin", 6×"Push commits to Forgejo origin", 1×"Push local commits to
   Forgejo (origin)", 1×"Push 307 commits to Forgejo origin" — six variants, summing to 50).
   The failure(1) run ends at a commit instead; the timeouts/success end at `git log`,
   `bf create`, `bf show` (two with no Bash call yet) — all matching the bundle README.
   So the Aug-12 storm was `git push`'s pack-objects over ~17 GB of loose objects inside the
   12 GiB dispatch scope, not generic "git operations": **bf-198ne (2026-08-16) is the same
   mechanism, not a later push-side variant** of a gc-side event. Caveat preserved from the
   bundle: last recorded tool call is not kernel proof of the killed process — it is per-run
   corroboration only (kernel records unrecoverable, §1-D).
4. **56 dispatches, not 50 attempts.** The retry loop made 56 claims: 50 crash/−1 + 4
   timeout/124 + 1 failure/1 + 1 success/0. The exit-0 run (21:14:56Z) still failed to close
   the bead (`bead.orphaned`); bf-4yjq was closed later, 2026-08-17. "50 crashes" counts
   deaths, not attempts — downstream counts should say which they mean.

## 4. Re-derivation commands

```bash
# 50 alert beads + storm table (JSON-parse, never grep counts)
python3 - <<'EOF'
import json, re
alerts = {}
with open('.beads/checkpoint/forensic.jsonl') as f:
    for line in f:
        if 'bf-4yjq' not in line: continue
        rec = json.loads(line); iss = rec.get('issue', rec); d = iss.get('description','')
        if '**Bead ID**: bf-4yjq' in d and 'Agent Crash Report' in d:
            alerts[iss['id']] = iss['created_at']
print(len(alerts), min(alerts.values()), max(alerts.values()))
EOF

# needle raw-log counts (live sources)
grep -c 'bf-4yjq' ~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2   # 225
grep -c 'bf-4yjq' ~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl  # 1071

# archive integrity
(cd docs/crash/bf-4yjq/raw-logs && sha256sum -c MANIFEST.sha256)
```

## Appendix A — the 50 alert beads, in creation order

All titled `ALERT: Agent crash on bead bf-4yjq`, all exit −1. Re-derived by JSON parse of
`.beads/checkpoint/forensic.jsonl` (never grep-count). `bf-29rca` — the instant named in the
task, 18:18:20Z — is the 6th.

| | | | | |
|---|---|---|---|---|
| `bf-276uk` 17:54:00 | `bf-3dq63` 18:03:36 | `bf-59bwz` 18:06:11 | `bf-3ssnm` 18:12:04 | `bf-2fiyo` 18:14:49 |
| `bf-29rca` 18:18:20 | `bf-uoyie` 18:19:49 | `bf-2weev` 18:22:15 | `bf-2ftau` 18:25:28 | `bf-44x3a` 18:27:02 |
| `bf-64hxa` 18:28:37 | `bf-3b9rv` 18:34:06 | `bf-1dxk7` 18:38:11 | `bf-hw4i5` 18:41:29 | `bf-1ygk6` 18:43:25 |
| `bf-2j99a` 18:49:51 | `bf-9b8oe` 18:52:09 | `bf-d7j07` 18:54:17 | `bf-46ttc` 18:56:18 | `bf-2dj1g` 18:59:03 |
| `bf-bkpuh` 19:02:26 | `bf-x5ynu` 19:04:11 | `bf-4tl4v` 19:05:45 | `bf-1dzwv` 19:07:54 | `bf-aruwg` 19:11:29 |
| `bf-2o8p2` 19:13:34 | `bf-2t7xh` 19:16:03 | `bf-4wi3v` 19:21:11 | `bf-1fvk2` 19:24:58 | `bf-22514` 19:29:25 |
| `bf-35bhc` 19:31:21 | `bf-3f6ue` 19:35:56 | `bf-mlv3u` 19:40:14 | `bf-5egrf` 19:42:49 | `bf-bykl0` 19:44:29 |
| `bf-4tnae` 19:50:11 | `bf-3k3ya` 19:53:29 | `bf-5966o` 19:54:44 | `bf-vcsxj` 19:58:38 | `bf-19qh7` 20:04:58 |
| `bf-mus1k` 20:06:39 | `bf-50zoz` 20:10:21 | `bf-47ugw` 20:12:37 | `bf-3pee6` 20:14:23 | `bf-1o4ag` 20:16:52 |
| `bf-6awu2` 20:18:43 | `bf-gz3r6` 20:20:49 | `bf-1jxy8` 20:24:06 | `bf-66h5p` 20:26:05 | `bf-2n3ve` 20:30:43 |

*(point-in-time snapshot of a live store; regenerate with §4's re-derivation command if the
store has since been flushed again — the alert-bead IDs are immutable, only surrounding
unrelated lines drift)*

## Related

- Canonical report: [bf-4yjq-crash-investigation.md](bf-4yjq-crash-investigation.md) (`db3f1f2`)
- Raw-log bundle + its own provenance: [docs/crash/bf-4yjq/raw-logs/README.md](../crash/bf-4yjq/raw-logs/README.md) (domchk-495041ac)
- Push-side mechanism, later instance: [docs/crashes/bf-198ne-crash-report.md](../crashes/bf-198ne-crash-report.md)
- Repo repair that ended the regime: [docs/crashes/bf-4yjq-cleanup-verification.md](../crashes/bf-4yjq-cleanup-verification.md)
