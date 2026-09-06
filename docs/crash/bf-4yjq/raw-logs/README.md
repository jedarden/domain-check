# Raw logs — bf-4yjq crash storm (2026-08-12)

**Subject bead:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo mirror has
diverged/gone stale" (P2, closed 2026-08-17)
**Extraction bead:** domchk-495041ac · **Extracted:** 2026-09-06 · **Workspace:**
`/home/coding/domain-check`, worker `claude-code-glm-4.7-lab-domain-check`, needle session
`8446529e`

This directory holds the raw needle output and the per-run agent session streams for the
56-dispatch storm of 2026-08-12 (50 crashes exit −1, 1 failure exit 1, 4 timeouts exit 124,
1 exit-0 that still orphaned the bead). Nothing here is summarized or paraphrased except
where a file is explicitly labeled as an index.

---

## 1. Crash timestamp verification — which run

The correct run was identified from two independent primary sources, then cross-checked
against each other to the second:

| Source | File | Coverage |
|---|---|---|
| Needle plaintext worker log | `~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2` | first line 2026-08-11T14:12:54Z → last 2026-08-15T20:35:19Z |
| Needle structured event log | `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl` | 2026-08-12 only |

Both agree: first dispatch 2026-08-12T17:50:23Z, last dispatch 21:11:40Z, first death
(classified `crash` exit −1) **2026-08-12T17:53:53.875Z**, last crash death
**2026-08-12T20:30:38.310Z**, then 4 timeouts (20:40:47 / 20:51:01 / 21:01:14 / 21:11:27Z)
and one exit-0 run at 21:14:56Z that failed to close the bead (`bead.orphaned`). The 50
deaths are listed to the millisecond in
[`docs/crash-investigations/bf-4yjq-artifact-catalog-2026-09-06.md`](../../crash-investigations/bf-4yjq-artifact-catalog-2026-09-06.md)
§2.1 and re-derived from `needle-events-2026-08-12-bf-4yjq.jsonl` in this directory.

**Local-time note:** these are UTC. Host file mtimes are EDT (−4h), so the storm window is
13:50–17:14 local on 2026-08-12.

## 2. Contents

| File | Size | What it is |
|---|---|---|
| `bf-4yjq-crash-sessions-2026-08-12.tar.gz` | 3.5 MB | **The 56 per-run agent session transcripts** (14.76 MB uncompressed), byte-exact, one per dispatch. See §3. |
| `sessions-index.tsv` | — | One row per dispatch: outcome, exit code, dispatch/classified UTC, duration, transcript last-event UTC, kill gap, lines, bytes, sha256, original filename. 56 rows + header. |
| `needle-events-2026-08-12-bf-4yjq.jsonl` | 283 KB | All 1,071 bf-4yjq records from the structured needle event log, verbatim (event types: `bead.claim.succeeded`, `agent.dispatched`, `transform.*`, `agent.completed`, `outcome.classified`, `outcome.handled`, `bead.released`, `heartbeat.emitted`, `verification.passed`, `bead.orphaned`). This is a second primary source the artifact catalog did not use. |
| `needle-worker-log-bf-4yjq-slot2.log` | 92 KB | Copy of the 225 raw lines from worker-log rotation slot `.log.2` (committed copy: `docs/crash-analysis/bf-4yjq-needle-worker-log-extract.log`). Duplicate included so this bundle is self-contained. |
| `MANIFEST.sha256` | — | sha256 of every other file in this directory. |

Extract the transcripts with:

```bash
tar -xzf bf-4yjq-crash-sessions-2026-08-12.tar.gz   # → sessions/attempt-NN-<outcome>-exit<code>--<sessionId>.jsonl
sha256sum -c MANIFEST.sha256                        # run from inside this directory
```

**Git note:** `needle-events-2026-08-12-bf-4yjq.jsonl` and
`needle-worker-log-bf-4yjq-slot2.log` are matched by this repo's blanket `*.jsonl` /
`*.log` ignore rules and are **force-added on purpose** (`git add -f`) — they are the
deliverable, not scratch output. A fresh clone keeps them; `git clean -X` in this
directory would delete them.

## 3. The session transcripts survive — correction to the standing record

Every prior report states that no per-run session evidence survives for this storm:

- canonical report §3: "No raw session evidence survives" (no core dumps / stack traces /
  heartbeats) — the artifact catalog re-affirmed this for *session* evidence;
- artifact catalog §6: "Crash-era session transcripts — Per-attempt agent JSONL is deleted;
  trace capture did not yet exist."

That claim is **wrong for the crash-era dispatch transcripts**. All 56 per-run Claude Code
session files survive under `~/.claude/projects/-home-coding-domain-check/<sessionId>.jsonl`
— a location none of the prior inventories checked (they only looked at needle's own
`~/.needle/` and `.beads/traces/` capture, which did not exist until later, and at the
per-alert re-dispatch traces).

Match proven: each of the 56 attempts was paired 1:1 with exactly one transcript whose
first line carries the dispatch tag
`[needle:claude-code-glm-4.7-lab-domain-check:bf-4yjq:auto]` and whose last event timestamp
sits 0–23 s before needle's `outcome.classified` for that attempt (the gap is the kill-to-
classification latency; ~13 s is typical for a SIGKILL death, 0 s for the exits that
recorded their own status). 56 attempts → 56 transcripts, zero unmatched, zero orphans.
The transcript of attempt 1 ends at 17:53:38.489Z and needle records its death at
17:53:53.875Z. The mapping, with hashes, is `sessions-index.tsv`.

These transcripts are the runs' stdout stream — the same artifact needle's later trace
capture stores as `stdout.txt` in `.beads/traces/`. There is no separate stderr capture for
this era; the transcripts' truncation point *is* the death record.

**Retention warning:** `~/.claude/projects/` is outside needle's rotation but equally
unmanaged. This tar.gz is now the only copy of attempt-level evidence for all 56 runs.

## 4. What the raw logs show at the death point

Derived from the transcripts (stated here as an observation, not a re-analysis):

- **All 50 crashed runs have `git push origin main` as their final recorded Bash tool call**
  (29× described "Push local commits to Forgejo origin", 7× "…to Forgejo", 6× "Push to
  Forgejo origin", 6× "Push commits to Forgejo origin", 1× "Push 307 commits to Forgejo
  origin", 1× "…to Forgejo (origin)"). 18 of the 50 show a `git commit` earlier in the same
  session; the other 32 issued the push without a prior `git commit` tool call. Per-run git
  activity before the death: 7–32 git commands (mean 15.6) — each retry redid the whole
  task and then died at the same terminal workflow step.
- The one exit-1 failure (attempt 2, 18:00:17Z) ended instead at
  `git add -A && git commit …` — not at a push.
- The 4 timeouts and the exit-0 run ended elsewhere (`git log`, two with no Bash call yet,
  `bead create`, `bead show` — note `bf`, the pre-migration CLI).

Caveat, stated plainly: "last recorded tool call" is not proof of the process the kernel
killed; the agent is not sampled between tool calls. But 50/50 uniformity on the identical
command, across sessions of varying length and content, pins the death point to the push
step of the workflow.

**Mechanism significance.** The canonical account attributes the storm to OOM "during git
operations on the bloated 18 GB repository", and treats the push-side memcg OOM as first
observed at bf-198ne (2026-08-16, "the push-side variant"). The raw logs show the Aug-12
storm was itself uniformly push-side — `git push`'s pack-objects over the 17 GB of loose
objects, inside the dispatch scope's 12 GiB `MemoryMax`. bf-198ne is therefore not a later
variant of a gc-side event; it is the same mechanism already operating on Aug 12, which is
also why `pack.windowMemory`/`pack.threads` bounds (installed 2026-09-02) cover `git push`
as well as gc. The kernel-level step for Aug 12 itself remains unverifiable (§5), so this
is per-run corroboration of the mechanism, not kernel proof of it.

## 5. Confirmed absences (re-checked live 2026-09-06)

| What | Status |
|---|---|
| Core dumps | `coredumpctl` earliest entry 2026-08-17 16:01:44 EDT (pdftract, COREFILE missing) — nothing from Aug 12 |
| Kernel / journald records | system journal first entry 2026-08-15 19:46:33 EDT (boot `52309698`, single boot) — Aug-12 kernel kills unrecoverable |
| `.beads/traces/bf-4yjq/` | absent — dispatches predate needle trace capture |
| Needle stderr capture for this era | none exists |
| `~/.needle/snapshots/heap-1785742906779-manual.heapsnapshot` | 2026-08-03 — predates the storm, unrelated, not included |

## 6. Provenance — how to re-derive everything

Extracted 2026-09-06 from live sources by dispatch domchk-495041ac:

```bash
# needle plaintext worker log (225 bf-4yjq lines)
grep 'bf-4yjq' ~/.needle/logs/needle-claude-code-glm-4_7-lab-domain-check.log.2

# needle structured event log (1071 bf-4yjq records)
grep 'bf-4yjq' ~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl

# per-run session transcripts (the tar.gz above) — match the dispatch tag on the
# FIRST line only. A whole-file `grep -l` over-matches: any later session that
# quotes the tag as tool output (e.g. an agent running this very grep) also hits —
# 59 whole-file matches vs 56 first-line matches when re-checked 2026-09-06.
for f in ~/.claude/projects/-home-coding-domain-check/*.jsonl; do
  head -c 400 "$f" | grep -q '\[needle:claude-code-glm-4\.7-lab-domain-check:bf-4yjq:' \
    && echo "$f"
done   # → 56 files
```

Attempt↔transcript pairing rule: match each `outcome.classified` timestamp (from the event
log) to the transcript whose last event is ≤5 s before it and closest; verify uniqueness.
`sessions-index.tsv` is the result; `MANIFEST.sha256` pins the bytes committed here.
