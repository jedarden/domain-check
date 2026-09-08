# bf-4x12ec — Crash Artifacts Summary (2026-09-02)

Investigator bead: **domchk-2ff261ce** · Crash bead: **bf-4x12ec**
Agent: `claude-code-glm-4.7` · Worker: `claude-code-glm-4.7-lab-domain-check`
Needle session: `a6dbb1fc` · Workspace: `/home/coding/domain-check`
First recorded crash: **2026-08-14T10:23:11.219513632Z** · Exit code: **-1**

All timestamps UTC unless marked EDT (local = UTC−4).

> **2026-09-08 — consolidated.** This is the split parent's artifact survey,
> now the summary document for umbrella `domchk-2ff261ce`'s four-child split.
> §1–§8 are the 2026-09-02 survey, retained as written; **§9** (added by child
> 4, `domchk-10640b7f`) folds in the three sibling children's deliverables,
> applies their corrections to the superseded figures below, and verifies the
> parent bead's four acceptance criteria with pointers.

## 1. Headline findings

1. **This was not one crash — it was the first of 44.** Between 10:21 and 11:27
   UTC the worker claimed bf-4x12ec **53 times**: 44 attempts died with exit -1,
   8 more hit the 600 s agent timeout (exit 124), and the 53rd succeeded (exit 0,
   12:58:45). Each of the 44 kills occurred seconds after the agent launched
   `git gc --aggressive --prune=now` against a repo holding **17.20 GiB across
   4,649 loose objects** (18G `.git`).
2. **The kill was not host-wide OOM.** Contemporaneous tool output captured
   mid-storm (10:43:59 UTC) shows the host with **45Gi memory available and
   0B swap used** — and that attempt was still killed ~8 s after launching gc.
   The kill is consistent with a **cgroup/memcg-scoped OOM** on the agent scope
   (the same pattern visible in today's kernel journal for `safe-git-gc-*.scope`
   memcg kills), not exhaustion of the host's 62G.
   *(Dated correction 2026-09-08: the "~8 s" is the `free -h` result → gc
   launch pause, not a launch→kill interval; the verifiable launch→kill gap for
   attempt 15 is 45.6 s, and for all 44 kills 18.6–80.6 s — see §9.2.)*
3. **The kernel record for the crash window is unrecoverable.** The systemd
   journal's only boot starts 2026-08-15 19:26 EDT — after the crash. No kernel
   OOM lines for Aug 14 survive; the classification rests on the needle-side
   exit codes plus the source-level command correlation above.
   *(Dated correction 2026-09-08: the boot's first entry is
   2026-08-15T23:56:33Z = 19:56:33 EDT — "unrecoverable" stands, the timestamp
   is corrected, and the mechanism is now kernel-proven for the same scopes one
   day later via the Aug-16 `CONSTRAINT_MEMCG` census — see §9.2.)*
4. **Resolution path:** needle's auto-split template kicked in; the final
   attempt decomposed the bead into three children — `bf-173o7e` (gc, **Closed**),
   `bf-5jhvpk` (repack, still **Open**), `bf-im2sl1` (verify, still **Open**) —
   chained them with dependencies, and closed the parent as an umbrella.
5. **The bead's own prescribed command was the crash trigger.** The task body
   instructed `git gc --aggressive --prune=now` on the 18GB repo — the exact
   operation that OOM-killed the previous agents. The crash was incidental to
   agent behavior; the work itself was hazardous as specified.

## 2. Crash timeline (2026-08-14, UTC)

| Time | Event | Source |
|------|-------|--------|
| 10:17:26.387 | Bead bf-4x12ec created (P2, auto strand) | bead record |
| 10:21:06.969 | `bead.claim.succeeded` — session `a6dbb1fc`, prompt_len 71698, template `pluck` | needle log seq 1729 |
| 10:21:07.995 | Prompt enqueued → Claude session `8b2a5b0d` | transcript |
| 10:21:23.336 | `git count-objects -vH` → 4,649 loose, 17.20 GiB, 9.60 MiB pack | transcript |
| 10:21:32.406 | `du -sh .git/` → 18G | transcript |
| 10:21:41.782 | gc attempt 1 → **exit 128**: `bad numeric config value '1.hour' for 'gc.aggressivewindow'` | transcript |
| 10:22:02.982 | Fix try `git config gc.aggressivewindow "1h"` → **exit 128** (still non-numeric) | transcript |
| 10:22:18.314 | `git config --unset gc.aggressivewindow` → OK (unset persists to today) | transcript |
| 10:22:36.260 | **gc attempt 2 launched** (`git gc --aggressive --prune=now`) — no result ever returned | transcript |
| 10:23:02.958 | `agent.completed` — **exit_code -1**, dur 115 797 ms | needle log seq 1741 |
| 10:23:02.959 | `outcome.classified` — exit -1 → outcome **crash** | needle log seq 1744 |
| 10:23:11.219 | `HANDLING_RELEASE_DONE` heartbeat — **the dispatch's recorded crash timestamp** | needle log seq 1750 |
| 10:23:14.244 | `bead.released` (release_success); `outcome.handled` — action **alerted** | needle log seq 1752–1753 |
| 10:23:16 → 11:27 | **42 more claim→crash cycles**, all exit -1, 39–116 s each (44 total) | needle log |
| 10:43:58.590 | Mid-storm evidence capture (transcript `9539f3b2`): `df` 85% used / 67G free; `free` **45Gi available**, swap 0B | transcript |
| 10:44:07 | That attempt launches gc → killed (exit -1 at 10:44:53) | transcript + needle log |
| 11:28:07 | First of 3 `pluck` attempts producing **zero assistant output** → exit 124 at exactly 600 s (11:38:07, 11:48:28, 11:58:51) | needle log |
| 11:59:06 | Needle switches to **`split` template** ("Auto-Split: Decompose This Bead", prompt_len 3896) | needle log |
| 11:59:06 → 12:50:33 | 5 more auto-split attempts → all exit 124 at 600 s, no tool calls | needle log |
| 12:57:53–12:58:39 | Final attempt (transcript `31800ee3`): `bf create` ×3 children, `bf dep add` chain, umbrella label | transcript |
| 12:58:45.113 | **exit 0**, `verification.passed`, outcome success — bead closed | needle log |

## 3. Artifact inventory

### Primary artifacts (recovered)

| Artifact | Path | Mtime (EDT) | Notes |
|----------|------|-------------|-------|
| Crashed attempt #1 transcript | `~/.claude/projects/-home-coding-domain-check/8b2a5b0d-0aae-4226-8ff8-e9d263e84045.jsonl` | 2026-08-14 06:23:02 | 50 lines; dispatch marker `[needle:claude-code-glm-4.7-lab-domain-check:bf-4x12ec:auto]`; ends mid-gc with no tool_result |
| Needle worker log (Aug 14) | `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl` | current | 10 138 lines; bf-4x12ec events at seq 1729–2003+; 53 claims, exit codes −1×44, 124×8, 0×1 |
| Mid-storm attempt w/ system state | `~/.claude/projects/-home-coding-domain-check/9539f3b2-eabe-432b-8d9f-7b5b0abc931d.jsonl` | 2026-08-14 06:44:53 | Contains `df -h` + `free -h` output captured 8 s before its own kill |
| Auto-split timeout attempt | `~/.claude/projects/-home-coding-domain-check/c48ec3f3-0420-455f-9e48-7c728cc914fe.jsonl` | 2026-08-14 08:09:22 | `split` template prompt (3 896 chars) with zero assistant events |
| Successful split attempt | `~/.claude/projects/-home-coding-domain-check/31800ee3-de7c-4619-abe8-07468fb7de32.jsonl` | 2026-08-14 08:58:44 | The bead-splitting session; exit 0 |
| Bead record | `bead show bf-4x12ec` (live, post-migration) | updated 2026-08-17 | Includes final metrics in notes: `.git` 753 MB, 141 loose, 10 265 in-pack |
| Sub-beads | `bf-173o7e` (Closed), `bf-5jhvpk` (Open), `bf-im2sl1` (Open) | created 2026-08-14 12:57–12:58 | gc / repack / verify split |

### Negative findings (checked, absent)

- **Kernel journal for the crash window:** gone. `journalctl --list-boots` shows a
  single boot starting 2026-08-15 19:26:03 EDT; nothing from Aug 14 survives.
  *(Dated correction 2026-09-08: first entry is 2026-08-15T23:56:33Z =
  **19:56:33 EDT** — see §9.2.)*
- **`.beads/logs/*` monitoring logs:** earliest entries 2026-09-01 — the
  resource/crash monitors did not exist on Aug 14.
- **`.git/gc.log`:** absent (no failed-gc residue).
- **Git reflog:** no entries surviving from Aug 14–15 (expired/rewritten).
- **Aug-14 pack file:** the single current pack is dated Sep 2 08:33 — the Aug-14
  pack was since rewritten by scheduled maintenance, so pack mtimes no longer
  evidence the original cleanup.
- **Needle log itself:** contains no "oom"/memory events — only exit codes.

*(Dated correction 2026-09-08: the "primary artifacts" table above lists the
**five** transcripts that had been captured as of 2026-09-02. Child 1 of the
split later established that **all 53** attempt transcripts survive and
committed them byte-verified — see §9.1 and §9.2.)*

## 4. System state evidence

**Captured from inside the crash window (transcript `9539f3b2`, 10:43:59 UTC,
~20 min after the first crash):**

```
Filesystem  Size  Used Avail Use%   → /dev/...  444G  355G  67G  85% /
            total  used  free  buff/cache  available
Mem:        62Gi   16Gi  24Gi  22Gi        45Gi
Swap:       24Gi   0B    24Gi
```

Disk was at 85% (67G free — adequate); **memory was not host-constrained**
(45Gi available, zero swap pressure) — yet the kill still happened. This rules
out the earlier investigations' framing of host-wide memory exhaustion at this
moment and points at a **scope-limited (memcg) kill**. Supporting parallel from
today's journal (different events, same mechanism): repeated
`Memory cgroup out of memory: Killed process ... oom_memcg=...safe-git-gc-*.scope`
kills on 2026-09-02 — i.e. this box demonstrably kills git processes that
exceed their cgroup budget while the host has memory to spare.

**Repository state across the window:** 4,649 loose objects / 17.20 GiB at
10:21:23 and *identical* figures at 10:43:49 — the killed gc attempts made
**zero** packing progress (aggressive gc builds delta chains in memory before
writing any pack). All actual progress happened after the split, when
bf-173o7e's gc ran to completion.

## 5. Error messages

No stack traces exist — SIGKILL-style kills leave none, and no core dumps were
generated. The recoverable error output:

1. `fatal: bad numeric config value '1.hour' for 'gc.aggressivewindow' in file .git/config` (exit 128, 10:21:41)
2. `fatal: bad numeric config value '1h' for 'gc.aggressivewindow' in file .git/config` (exit 128, 10:22:09 — git expects an integer number of *days*, not a duration)

The invalid `gc.aggressivewindow=1.hour` setting (author unknown; predates this
bead) cost the agent two of its attempts before the fatal one; the agent's
`git config --unset` fix persisted and the key is still unset today.

Needle-side classification (verbatim): `{"exit_code":-1,"outcome":"crash"}` →
`{"action":"alerted","bead_id":"bf-4x12ec","outcome":"crash"}`.

## 6. Timestamp reconciliation

- **10:23:11.219513632Z** (this dispatch's crash timestamp) = the
  `HANDLING_RELEASE_DONE` heartbeat of crash #1 (seq 1750, 10:23:11.219506142Z).
  The agent process actually died at **10:23:02.958Z**.
- **10:25:30.457683731Z** (used by `docs/crash-investigations/bf-4x12ec-crash-investigation.md`)
  = the same heartbeat of **crash #2** — the worker re-claimed the bead at
  10:23:16 and crashed again. Both refer to the same incident chain; neither is
  "the" crash, because there were 44.

## 7. Corrections to the prior investigation report

1. **Scale:** prior report frames a single crash; the needle log shows a
   44-crash retry storm (10:21–11:27) followed by 8 timeouts — part of the
   Aug-12/14 infrastructure storm documented for bf-4yjq et al.
2. **Mechanism:** "<2GB available, OOM killer" is not supported for this
   window — captured host memory at 10:43:59 was 45Gi available. Refine to:
   cgroup-scoped kill of git gc inside the agent's resource scope (exact
   kernel record lost to journal rotation).
3. **Attribution of the cleanup:** the successful gc ran under child bead
   **bf-173o7e** (now Closed); children `bf-5jhvpk` (repack) and `bf-im2sl1`
   (verify) were **left Open** when the parent umbrella closed. They are now
   redundant — scheduled safe-git-gc maintenance has since driven the repo to
   92 MB `.git` / 20 loose objects (verified 2026-09-02) — but they should be
   closed rather than re-dispatched.

## 8. Conclusion

Exit -1 on bf-4x12ec = a cgroup-scoped kill of `git gc --aggressive` on an
17.2-GiB-loose-object repository, repeated 44 times by automatic retry until
needle's auto-split template decomposed the bead into executable units. The
crash was environmental (repository bloat + per-scope memory limits), the task
specification itself prescribed the memory-hazardous command, and no
domain-check code was involved. Work was completed via the split; the parent
bead is Closed with final metrics recorded (18 GB → 753 MB, 4,649 → 141 loose).

## 9. Consolidation addendum (2026-09-08 — child 4 of the `domchk-2ff261ce` split)

Written by `domchk-10640b7f`, the final child of the auto-split this bead's
umbrella underwent on 2026-09-02T14:52Z. It consolidates the three sibling
children's deliverables into this summary, applies their corrections to the
2026-09-02 figures above (originals retained, each marked in place), and
verifies the parent bead's four acceptance criteria. Nothing in §1–§8 is
rewritten; where this section disagrees with them, this section is the
correction.

### 9.1 Deliverables consolidated (all committed and pushed)

| Child | Bead | Deliverable | Commit |
|---|---|---|---|
| 1 | `domchk-a3f1f8f5` | Surviving log-source inventory — every source, its window, recoverability; captured **all 53** attempt transcripts (`docs/crashes/bf-4x12ec/transcripts/`, 53/53 `cmp`-verified + `census.tsv` + `MANIFEST.sha256`) | `e1d9477` |
| 2 | `domchk-ba8584a1` | Crash timeline — 53 attempts in 3 phases (44 × exit −1 → 8 × 600 s timeout → 1 × exit 0 auto-split), per-attempt table re-derived from the primary worker log | `66a8e09` |
| 3 | `domchk-eae1d2ed` | System-level cause class — host-wide OOM vs memcg OOM vs timeout, each tested against primaries; 44-row gc-launch → kill correlation table | `9edb0b4` |

Companion documents already in the tree that this summary sits between: the
canonical report [`bf-4x12ec-crash-investigation.md`](bf-4x12ec-crash-investigation.md)
(`b366d86`, mechanism + Addenda), the signal-sentinel semantics note
[`bf-4x12ec-evidence-signal-semantics-domchk-15854355-2026-09-07.md`](bf-4x12ec-evidence-signal-semantics-domchk-15854355-2026-09-07.md)
(`50ae4a6`), the alert inventory
[`bf-4x12ec-alert-inventory.md`](bf-4x12ec-alert-inventory.md) (`fea1a62`, the
44 storm alert beads `bf-fmg2cw` → `bf-5x69lm` plus 16 regeneration beads), and
the kernel/systemd extraction
[`evidence/bf-4x12ec/kernel-systemd-messages.md`](evidence/bf-4x12ec/kernel-systemd-messages.md)
(`55f23db`, `domchk-ad80e265` — landed while this addendum was in flight).

### 9.2 Dated corrections to §1–§8

1. **"Killed ~8 s after launching gc" (§1.2, §4).** 8.2 s is the gap between
   the `free -h` *tool_result* (10:43:59.438Z) and the gc *tool_use record*
   (10:44:07.643Z) — the pre-launch pause, not a launch→kill interval. The
   verifiable launch→kill gap for attempt 15 is **45.6 s**; for all 44 kills,
   **18.6–80.6 s** (mean 30.7 s, median 25.9 s). Child 3 §4 carries the full
   44-row table. The direction of the error runs against the original
   conclusion: the gc ran ~45 s, not ~8 s, with 45 Gi host-available
   throughout.
2. **Journal boot bound (§1.3, §3).** The single surviving boot's first entry
   is **2026-08-15T23:56:33Z (19:56:33 EDT)**, not 19:26 EDT. The conclusion
   is unchanged — no kernel OOM record for Aug 14 can exist — and the gap to
   the last kill (2026-08-14T11:27:26Z) is ~36.5 h.
3. **Log coverage (§3).** Five transcripts were the state of the capture on
   2026-09-02; **all 53 survive** and are now committed byte-verified in
   `docs/crashes/bf-4x12ec/transcripts/` (child 1's finding: until 2026-09-07
   a `~/.claude` retention event would have silently destroyed 48 of 53
   primary transcripts). The mid-tool-call death shape is now proven for every
   crash attempt, not just the sampled ones: 43 of 44 end at
   `git gc --aggressive --prune=now`, attempt 8 at the same command behind an
   `echo` header, all with no `tool_result`.
4. **Exit −1 semantics (§5, §8).** `exit_code −1` is needle's writer-side
   sentinel for "no exit status captured" — not a POSIX signal number (no
   signal −1 exists; the alert body's "signal −1" is a renderer applying
   `code − 128 if code > 128` to the sentinel). The *mechanism* behind the 44
   sentinel deaths is memcg-OOM SIGKILL, but the code itself identifies no
   signal. See the signal-semantics note (§9.1).
5. **Mechanism confidence (§1.2, §4).** "Consistent with a cgroup/memcg-scoped
   OOM" is now the verified verdict, not an inference from a parallel: the
   dispatch scope's `MemoryMax` is **12 GiB** (12884901888 B), verified live on
   a `run-p*.scope` on 2026-09-07, with dispatched agents at
   `oom_score_adj=200`; and the same mechanism is **kernel-proven one day
   later** — the Aug-16 census holds 257 `CONSTRAINT_MEMCG` git kills inside
   `run-p*.scope` memcgs at anon-rss median 11.73 GiB / max 11.97 GiB (99.8 %
   of the 12 GiB cap). For Aug-14 itself the class remains **regime-matched,
   not kernel-proven** (§5.3 of child 3), exactly as §1.3 states. The
   kernel/systemd extraction (`55f23db`) independently confirms both halves:
   no Aug-14 record survives (the window precedes the single surviving boot by
   ~36.5 h across three reboots), and of the 525 kernel memcg kills in the
   surviving journal, 375 sit at exactly usage == limit == 12582912 kB — the
   12 GiB dispatch cap.

### 9.3 Resolution status as of 2026-09-08

§7.3's "should be closed rather than re-dispatched" recommendation, updated
against the live bead store this attempt:

| Bead | Role | Status (2026-09-08) |
|---|---|---|
| `bf-4x12ec` | Crash target | **Closed** rev 4 (2026-08-17) |
| `bf-173o7e` | gc child | **Closed** rev 19 — closed 2026-08-17T17:12:09Z, "17.20GB loose objects packed into 444MB pack file" |
| `bf-5jhvpk` | repack child | Umbrella **Open** rev 18 — its repack child `domchk-371e54d8` is Closed rev 4 and the repack **executed 2026-09-08T00:28Z** (2 packs → 1, 100.25 MiB; peak pack-objects RSS 350.4 MiB against the 4 G cap). Awaiting re-close — do **not** re-run |
| `bf-im2sl1` | verify child | **Open** rev 1, never dispatched — redundant with the daily 02:00 repo-health timer |

Repository today (live `git count-objects -vH` / `du -sh .git` this attempt):
`.git` **106 MB**, ~250 loose objects / ~1.6 MiB (normal churn — 248 on the
first read, 252 on the second), **one pack 100.25 MiB** — against the 18 G
`.git` / 4,649 loose / 17.20 GiB the crash attempts measured.
The bloat cannot recur through `.beads/` (fully gitignored, 0 tracked files),
and the bare-gc/push path is bounded by `pack.windowMemory=2g` /
`pack.deltaCacheSize=1g` / `pack.threads=1` (repo + global).

### 9.4 Parent acceptance criteria — verification (`domchk-2ff261ce`)

Each of the four parent criteria, checked off with pointers to evidence:

- [x] **Capture any available logs from bead bf-4x12ec execution** — §3 above
      (2026-09-02 survey) as corrected by §9.2.3; full inventory: child 1's
      log-source inventory (§9.1), which captured the primary worker log
      (1,146 bf-4x12ec records), all 53 agent transcripts
      (`docs/crashes/bf-4x12ec/transcripts/`), and the 44 alert-bead records
      (`.beads/checkpoint/forensic.jsonl`, captured as the evidence bundle's
      `alert-beads-raw.jsonl`); second bundle
      `docs/crash-investigations/evidence/bf-4x12ec/crash-logs/`.
- [x] **Document the crash context (what the bead was doing)** — §2 above
      (short form) and child 2's full 53-attempt timeline (§9.1). Context: the
      bead's own task body prescribed `git gc --aggressive --prune=now` on a
      repo holding 17.20 GiB across 4,649 loose objects inside a 12 GiB
      dispatch scope; every kill landed inside that gc (child 3 §4).
- [x] **Check for system-level evidence (OOM, timeout, resource exhaustion)** —
      §4 above, now the tested verdict of child 3 (§9.1): host-wide OOM
      **excluded** (45 Gi available / 0 B swap mid-storm, attempt-15 capture),
      timeout **excluded** (deaths at 38.9–115.8 s vs the 600 s ceiling, and
      needle's own deadline kills are separately recorded as 124),
      **memcg OOM inside the 12 GiB dispatch scope confirmed** as the cause
      class — INFRASTRUCTURE per `docs/crash-response-guide.md`.
- [x] **List all artifacts found with timestamps** — §3 inventory above
      (recovered + negative findings, with mtimes), child 1's §2 source table
      (coverage windows per source), the alert inventory (44 alert beads with
      exit timestamps, `bf-fmg2cw` 10:23:11.225Z → `bf-5x69lm`
      11:28:02.199Z), and the evidence bundles' `census.tsv` /
      `attempt-index.tsv` / `MANIFEST.sha256`.

The parent's requested **output** — crash timeline (§2 + child 2), available
log sources (§3 + child 1), system state evidence (§4 + child 3), and error
messages (§5) — is carried by this document plus the three child deliverables.

---

Investigation date: 2026-09-02 · Investigator bead: domchk-2ff261ce
Consolidation addendum: 2026-09-08 · Investigator bead: domchk-10640b7f (child 4 of 4)
Artifact paths verified on this box at write time.
