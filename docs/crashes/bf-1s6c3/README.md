# Crash Artifact Summary: Bead bf-1s6c3 (2026-08-12)

**Collection bead:** domchk-fcac734a
**Collection date:** 2026-09-06
**Subject bead:** bf-1s6c3 — "Create merge commit reconciling Forgejo and GitHub histories" (P2, task; created 2026-08-12T21:12:09Z, status Closed)
**Sources:** raw needle event log + Claude Code session transcripts, both still on disk (fabric-prune has been broken since 2026-08-17, so Aug-12 dispatch logs were never deleted)
**Sibling extractions from the same event:** `docs/crash/bf-4yjq/`, `docs/crash/bf-2xygo/`
**Sibling write-up:** `docs/crashes/bf-1s6c3-crash-storm-timeline-2026-09-06.md`
(domchk-1fb4ad35, committed the same day) — its primary-source findings (real merge
`42a7b07`, Aug-16 close) are incorporated in §2 and §6.

Everything below is read from the raw artifacts in this directory, not from prior
summaries. Where prior docs contradict the raw log, the contradiction is stated
explicitly (§6). Every figure was re-verified against the raw logs and live `git`
on 2026-09-06 before commit; four errors in this document's first draft (written by
an earlier attempt of the same bead, uncommitted at its death) are fixed in §4, §5.2
and §6.

---

## 1. Crash log location and timestamp

| Source | Path | Size | SHA-256 (first 16) |
|---|---|---|---|
| Needle events, Aug-12 | `/home/coding/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl` | 3,589,712 B | `3a487dc3139a2785…` |
| Needle events, Aug-13 | `/home/coding/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` | 3,111,314 B | `f76959bb0542e2b5…` |
| Session transcripts | `~/.claude/projects/-home-coding-domain-check/*.jsonl` | 76 files, 22,254,104 B | per-file in `sessions-index.tsv` |

Extracted copies (byte-exact `grep` lines, original order preserved) are committed here:

- `needle-events-2026-08-12-bf-1s6c3.jsonl` — 945 events, 257,002 B
- `needle-events-2026-08-13-bf-1s6c3.jsonl` — 513 events, 139,129 B
- `bf-1s6c3-crash-sessions-2026-08-12_13.tar.gz` — all 76 crash-window session transcripts (5.0 MB compressed)
- `sessions-index.tsv` — one row per attempt: session start, UUID, bytes, last-issued command

SHA-256 for every committed file: `MANIFEST.sha256`.

**Timeline of record:**

- **2026-08-12T21:31:27.663Z** — first `bead.claim.succeeded` (seq 4641). The claim follows
  bf-2xygo's closing success (21:31:21) by six seconds; bf-1s6c3 is that analysis bead's
  follow-up task.
- **2026-08-12T21:31:27.674Z** — first `agent.dispatched` (seq 4650), agent
  `claude-code-glm-4.7`, template `pluck-default`, `prompt_len=70670`.
- **2026-08-12T21:36:44.519Z** — attempt 1 `agent.completed` `exit_code=-1` (duration 316,572 ms),
  classified `crash`, handled `alerted`.
- **2026-08-12T21:31:27Z → 2026-08-13T02:01:27Z** — **76 dispatch attempts over ~4.5 hours**
  (49 on Aug-12, 27 on Aug-13).
- **2026-08-13T02:01:22.561Z** — attempt 76 `agent.completed` `exit_code=0` (duration 384,204 ms)
  → `outcome=success` → `verification.passed` (gates_run 1) → **`bead.orphaned`** (02:01:27.732).

**The "2026-08-12T21:36:51.240046999Z" crash timestamp carried by the Sep-1 docs is attempt 1 of
76** — it falls inside attempt 1's post-crash handling window (`agent.completed` 21:36:44.519 →
`bead.released` 21:36:53.890). It is a real instant, but it is not "the" crash; it is the first
of 71 identical deaths.

## 2. Exit codes, agent identity, and the death mechanism

Agent/model for every attempt: `claude-code-glm-4.7` / `glm-4.7`, worker slot
`claude-code-glm-4.7-lab-domain-check`, routing rule `default`.

| Exit code | Attempts | Needle classification | Handling |
|---|---|---|---|
| −1 | 71 (49 Aug-12 + 22 Aug-13) | `crash` | `alerted` |
| 124 | 4 (Aug-13 01:11:08.765, 01:34:19.519, 01:44:32.389, 01:54:45.320) | `timeout` | `deferred` |
| 0 | 1 (Aug-13 02:01:22.561) | `success` | `none` |

All four timeouts ran exactly the 600 s cap (durations 600,018–600,024 ms).
Crash attempts lasted 62.5 s–431.0 s (median 160.9 s) — each one did real work
before dying. `exit_code=-1` is needle's wait-status sentinel for "died by
signal", not a signal number; no signal −1 exists.

**Where the 71 signal deaths happened.** Each row of `sessions-index.tsv` records the last
tool call the session issued before its transcript ends mid-flight (no tool result, no further
records):

| Last-issued command | Attempts |
|---|---|
| `git push origin main` | 60 |
| `git push github main` | 10 |
| `git push` | 1 |
| `git fetch origin && git fetch github` | 1 |
| `git commit -m "chore: update bead tracking state before merge reconciliation…"` | 1 |
| `bf show bf-4k2ws && …` (the successful split) | 1 |
| (no tool use recorded) | 2 |

**71 of 76 attempts died with `git push` as their last issued command** — 60 against Forgejo
`origin`, 10 against `github` (attempts 69–70 show the agent switching remotes trying to get
through), one bare. Worked example (attempt 1, session `5b194820`): 18 Bash calls, a real
commit (`[main 7ad8d15] chore: update bead tracking files before git reconciliation`, 8 files,
1802 insertions), then `git push origin main` issued at 21:36:29.421 — transcript ends with no
result, needle records `exit_code=-1` 15 s later.

**The deliverable itself landed mid-storm.** Attempt 4 (session `217a276f`, started
21:43:21.712Z) ran `git merge origin/main -m "Merge reconciliation: Forgejo and GitHub remote
histories"`, producing merge commit `42a7b07` (parents `47e7758` + `00117cb`) at **21:47:07Z** —
then exited −1 at 21:48:06.650Z like every other attempt, 59 s after committing. The commit
survived on disk, but it is **not an ancestor of `main`**: the 2026-08-16 history squash moved
it onto branch `pre-squash-history-20260816`, and no push of it ever landed (every push died,
per the table above). The storm's remaining 72 attempts re-did work against an
already-locally-merged repo because the bead's completion path still required the push.

This is the pack-objects memcg-OOM mechanism already proven for this repo: `git push`'s
pack-objects on the bloated repository exceeded the dispatch scope's memory limit and was
SIGKILLed — the same mechanism as bf-4x12ec (gc variant) and bf-198ne (push variant),
`docs/maintenance/repository-maintenance-guide.md:158`.

## 3. System resource state at crash time

**Direct samples do not exist.** System `journald` on this box starts 2026-08-15 19:46 EDT —
three days after this crash — so there are no kernel memcg records, load averages, or disk
readings for Aug-12. `.beads/logs/` monitoring starts 2026-09-01. Needle OTLP traces are
single-slot (last dispatch only). Any document claiming memory/disk/CPU figures *for the crash
moment* is reconstructed, not measured.

What *is* established, with committed provenance:

- **Repository state:** `.git` ≈ 18 GB, ~17 GB of loose objects — 17+ identical ~237 MB
  `.beads/*.jsonl` snapshots committed on 2026-08-12. Provenance:
  `docs/crashes/bf-4yjq-cleanup-verification.md` (sizes at :74-75, snapshot description at :84;
  re-measured during the verified cleanup); the pack-down to 93–94 MB held at every re-check
  since 2026-09-01.
- **Dispatch memory bound:** the needle dispatch scope runs `MemoryMax=12GiB`
  (`docs/maintenance/repository-maintenance-guide.md:158`). A `git push` of an 18 GB repository
  must materialize a pack well past that bound — which is what the 71 identical mid-push
  deaths (§2) show happening.
- **Per-attempt cost:** 71 dead attempts × 62–431 s each ≈ 5.5 hours of agent compute consumed
  by the retry loop, producing zero pushed bytes.

## 4. Surrounding bead states

**Same-slot beads crashing on Aug-12 (exit −1 / total completions, Aug-12 only):**

| Bead | Crashes | Notes |
|---|---|---|
| bf-31mno | 350/356 | largest single storm of the event in this slot |
| bf-4yjq | 50/56 | sibling; raw-log extraction at `docs/crash/bf-4yjq/` |
| **bf-1s6c3** | **49/49** | this bead — every Aug-12 attempt died |
| bf-2xygo | 4/5 | divergence analysis; 5th attempt succeeded 21:31:21.362Z, immediately before this bead's first claim |
| bf-23n, bf-5d18 | 1/3 each | same slot |

Cross-slot stragglers the same day (one crash each unless noted): bf-4tciy **2**
(roam-1 + roam-2), bf-28p 1 (roam-2), bf-kc3wh 1 (s1), bf-x8mluq 1 (test-fix).

These are **day-scoped, not window-scoped**: inside bf-1s6c3's own 21:31:27Z → 02:01:27Z
window this slot ran *only bf-1s6c3* (76/76 completions) — bf-31mno's and bf-4yjq's storms
ran earlier in the day. bf-66tf, named in some alert-side listings, has **no** `exit_code=−1`
completion in any slot that day; it appears in the alert stream only.

**The four split children created by attempt 76** (chain: bf-4k2ws →blocks bf-31p3g →blocks
bf-7d8l5 →blocks bf-6b0fl →blocks bf-1s6c3; parent labeled `umbrella`):

| Child | Title | Status (2026-09-06) |
|---|---|---|
| bf-4k2ws | Analyze divergent Forgejo and GitHub branch states | **Closed** |
| bf-31p3g | Create merge commit reconciling both histories | **InProgress** |
| bf-7d8l5 | Resolve merge conflicts and verify merge completeness | **Open** |
| bf-6b0fl | Push reconciled merge to Forgejo origin | **Open** |

The chain stalled after its first child. The split succeeded as a *bead operation* (no git
push involved, hence exit 0) but the merge+push the bead existed to do was never carried
through the chain. The Forgejo/GitHub histories were later reconciled by other work — current
state is zero divergence (`7906efc`, domchk-0dcecbe7, 2026-09-06) — but that reconciliation did
not flow through bf-1s6c3's children, which are still live workflow debt.

## 5. Pattern analysis — one-off or surge?

**Part of a surge — the largest in the workspace's record.** Raw counts for 2026-08-12 across
all needle slots on this box: **460 `agent.completed` events with `exit_code=-1`**
(domain-check slot 455, roam-2 2, test-fix/roam-1/s1 1 each), concentrated in five beads.

Structure of the storm as it hit bf-1s6c3:

1. **A deterministic kill, not random flakiness.** All 74 pluck dispatches were byte-identical
   (`prompt_len=70670`, same template, same agent). Every attempt that reached a push died at
   the push; there is no attempt that pushed successfully. The dispatcher retried a
   memory-doomed operation 71 consecutive times with no backoff and no resource gate.
2. **Attempts 73–76 show the escape:** attempt 73 (still pluck, `prompt_len=70670`) timed out
   after issuing `git fetch origin && git fetch github`; attempts 74 and 75 then produced **no
   tool calls at all** inside the 600 s window (the agent evidently reasoning without acting) —
   74 was the last pluck dispatch, 75 the first **auto-split** dispatch (`prompt_len=2868`; its
   transcript opens `## Auto-Split: Decompose This…`). Attempt 76, a fresh auto-split dispatch
   (`prompt_len=2868`, prompt_hash `sha256:d3b34a57…`) — the pluck prompt's hash
   (`sha256:aaa143d4…`) is shared by all 74 pluck attempts, and only three distinct hashes
   exist across all 76 — decomposed the bead into the four children in §4 using only `bf`
   commands, completing in 384 s without touching git. The deadlock was escaped by *changing
   the task shape*, not by any resource improvement; the repo stayed bloated until 2026-09-01.
3. **Alert feedback loop:** each of the 71 crashes emitted `outcome.handled action=alerted` —
   71 alerts for one undrainable cause. This is the same alert-storm shape as bf-173o7e
   (131 duplicate alerts) and bf-31mno (350), and it is why alert-side duplicate detection
   (crash-alert-manager fixes 2/3) matters as much as the resource fix itself.

**Generalizable reading:** repo-bloat OOMs do not just kill one operation — they convert any
task that must push into an unbounded retry storm, multiply alerts 1:1 with attempts, and only
end when either the repo shrinks or the task is reshaped. This bead is the cleanest recorded
example of the second exit.

## 6. Corrections to prior bf-1s6c3 docs

The prior doc set (2026-09-01, ~20 files) predates raw-log extraction and is wrong on four
points. Listed here so nobody cites them forward; the raw artifacts in this directory are the
authority.

| Prior claim | Raw-log reality |
|---|---|
| "Crash Date: 2026-08-12T21:36:51Z" (`docs/crash-investigation/bf-1s6c3-crash-artifacts-summary.md`) | Attempt **1** of **76**. The event ran 21:31:27Z Aug-12 → 02:01:27Z Aug-13. |
| "False Positive — Post-Completion Infrastructure Event … task was already done" (same file) | Contradicted: all 49 Aug-12 sessions died **mid-task**, 71 of 76 at the push step (§2). The exit −1s were genuine work-killing memcg OOMs. |
| "Resolution Date: 2026-08-16" (same file) | Half right, for the wrong reason. The needle lifecycle ended 2026-08-13T02:01:27Z (`bead.orphaned` after the split), but the bead record itself **was** closed 2026-08-16T14:00:13.240Z (`.beads/checkpoint/forensic.jsonl`) — with a close reason citing merge `7dd79eb`, a dead SHA (next row). |
| "Merge Commit: 2832106 … successfully completed before crash" (same file) | `2832106` is not an object in this repository (`git cat-file` fails) — and neither is `7dd79eb`, which the bead's own close reason cites. The real merge is **`42a7b07`** (parents `47e7758` + `00117cb`, 2026-08-12T21:47:07Z), created mid-storm by attempt 4 (§2); after the Aug-16 squash it survives only on branch `pre-squash-history-20260816` and is **not an ancestor of `main`**. Verified live 2026-09-06; matches `docs/crashes/bf-1s6c3-crash-storm-timeline-2026-09-06.md` (domchk-1fb4ad35). |
| "9 OOM crashes over 2.5 hours" for bf-1s6c3/bf-4yjq (CLAUDE.md evidence note, inherited from the same docs) | bf-1s6c3 alone recorded **49** crashes on Aug-12; the slot recorded **455**, the event **460** (§5). The "9" figure has no counterpart in the raw log. |

Bead-note claims inherited by `bead show bf-1s6c3` ("task completed successfully after
repository cleanup 18 GB → 138 MB") carry the same error: the split children (§4) show the
merge child still InProgress and the push child still Open. The cleanup is real; the claim
that *this bead's* task completed through it is not. The same dead SHA propagates through
the crash-alert pool — the Aug-16 14:00–16:23 close wave (`bf-1rsa6`, `bf-2jd0x`,
`bf-3auz2`, `bf-3g4cp`, …) all cite merge `7dd79eb` or "post-completion"; `bf-3auz2` even
closes citing the unrelated Domain Watch feature.

## 7. Acceptance-criteria mapping

- **Crash log location and timestamp** — §1 (source paths, sizes, hashes, timeline).
- **System resource state at crash time** — §3 (what is recoverable, what is not, and why).
- **Agent type and exit code** — §2 (claude-code-glm-4.7 / glm-4.7; −1 ×71, 124 ×4, 0 ×1).
- **Crash pattern indicators** — §5 (surge of 460 exit −1s; deterministic 71× push death loop).
- **Artifacts saved to `docs/crashes/bf-1s6c3/`** — this directory (§1 file list, `MANIFEST.sha256`).
