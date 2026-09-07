# Mitigation — the 2026-08-16T13:30:50Z death on bead bf-198ne

**Bead:** domchk-99f184b6 (step 3 of 4: ~~gather/analyze domchk-507d18c4~~ → ~~classify domchk-9d269c9c~~ → **mitigate (this bead)** → document domchk-f3c5f61d)
**Parent alert:** domchk-d46ec441 "ALERT: Agent crash on bead bf-198ne" — closed by **domchk-f3c5f61d**, not by this bead
**Companion documents:** [`bf-198ne-crash-2026-08-16-artifact-analysis.md`](bf-198ne-crash-2026-08-16-artifact-analysis.md) (root cause, §8 classification, §10 handoff), [`bf-198ne-context.md`](bf-198ne-context.md) (the earlier 2026-08-12 bf-2xygo event the alert originally concerned)
**Classification being mitigated:** **INFRASTRUCTURE** — memcg OOM (`CONSTRAINT_MEMCG`) SIGKILL of `git push` inside a 12 GiB dispatch scope, per the analysis's §8

All `[LIVE]` claims in this document were re-derived on **2026-09-06** by domchk-99f184b6 using the
same primary-source conventions as the analysis document — nothing here is cited from prior
reports without re-checking.

---

## 1. What this bead had to decide

The analysis's §10 handoff states the mitigation step should treat the crash as historical rather
than actionable, because every mitigation it implies was already in place and mechanically
enforced. Before accepting that, this bead re-verified each claim live rather than trusting the
handoff. **The result: every claimed mitigation is in force today, and the crash's specific
mechanism is structurally impossible to reproduce in this repository now.** No new code or
configuration change is warranted — inventing one would add surface without closing any gap.

The deliverable of an INFRASTRUCTURE-class mitigation per the acceptance criteria is: document
resource state, verify no data loss, and record the mitigation action. All three are below.

## 2. Resource state at mitigation time [LIVE, 2026-09-06]

| Resource | Value | Threshold (crash-response guide) | Status |
|---|---|---|---|
| Available memory | 44 GiB of 62 GiB | ≥ 20 GiB minimum, 10 GiB warning | ✅ |
| Swap | 24 GiB, 0 B used | — | ✅ |
| Disk free on `/` | 81 GiB | ≥ 50 GiB minimum, 30 GiB warning | ✅ |
| Load average (1 min) | 2.65 | < 5 | ✅ |
| `.git` size | 93 MB (1 pack, 90.43 MiB; 88 loose objects, 708 KiB) | < 500 MB healthy; 18 GB caused bf-1s6c3 | ✅ |
| Largest object in pack | 14.97 MB (below the 10 MB pre-commit gate's relevance for new content) | — | ✅ |
| Systemd user timers | 6 `domain-check-*` timers, all with future triggers (service/resource/pattern monitors every 2–10 min; repo-health daily 02:00; incremental gc daily 03:00; full gc weekly Sun 04:00) | — | ✅ |

Every resource that was exhausted at the moment of the crash — the 12 GiB dispatch scope — is
bounded by configuration rather than by luck (§3.1).

## 3. Mitigation in force, with live verification

### 3.1 Git pack memory is bounded by persistent config — the direct countermeasure

The crash mechanism was `git push` spawning `pack-objects`, which ramped to the scope's exact
12 GiB bound (`usage 12582912kB, limit 12582912kB` on both kill records, §5.1 of the analysis).
That ramp is now impossible: pack memory is bounded by repo-local **and** global git config.

| Check [LIVE 2026-09-06] | Result |
|---|---|
| `./scripts/setup-git-gc-config.sh --verify` | exit 0 — effective chain resolves to `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`, `pack.threads=1` (threads pinned because the window limit is per-thread) |
| Worst-case pack memory from the effective bound | ≈ 3072 MiB — within the 6442450944 B (6 GiB) ceiling the script computes for a 12 GiB dispatch scope |
| `./scripts/test-gc-memory-bounds.sh` (full suite incl. integration) | **12 passed, 0 failed** |
| Integration test: the exact crash command `git gc --aggressive --prune=now` on 8 × 64 MiB incompressible blobs under `MemoryMax=768M` (1/16 of the fatal scope) | exited 0; `pack-objects` **peak RSS 320,536 KB** — against the >12 GiB anon-rss that killed both Aug-16 agents |
| Unit tests: fresh-repo bounds land; `--verify` rejects unbounded and thread-multiplied states; a repo with no local bound still verifies via the box-wide global | all pass |

`pack.windowMemory`/`pack.deltaCacheSize` are read by `pack-objects` wherever git spawns it —
`gc`, `repack`, and `push` alike — so the operation that actually died is covered, not just the
`gc` variant the bounds were originally written for.

### 3.2 The 5.6 GB object mass is gone and cannot return

| Check [LIVE 2026-09-06] | Result |
|---|---|
| `.beads/` in `.gitignore` | line 66: `.beads/` |
| Tracked files under `.beads/` | 0 (`git ls-files .beads/ | wc -l` = 0) |
| `.beads/.bf_history` (the 4.7 GB bead-forge snapshot mass) | absent |
| Repo pack | 90.43 MiB in a single pack — the `b2d8233` state held |

The push that killed both agents was heavy *only* because the worktree carried 5.6 GB of
blob data origin had never received. That vector is closed structurally: nothing under `.beads/`
can be staged again, and the pre-commit hook (§3.3) blocks any other large file from entering.

### 3.3 Large-file admission gate is installed

`.git/hooks/pre-commit` is present and executable (3,446 B), implements `MAX_SIZE_MB=10`, and
scans staged files — the same hook documented after bf-4yjq's 17 GB loose-object incident. [LIVE]

### 3.4 Fleet-wide monitoring is live

All six `domain-check-*` systemd user timers had recent past runs and future triggers on
2026-09-06 — resource monitoring every 5 min, service monitoring every 2 min, crash-pattern
detection every 10 min, plus the daily/weekly maintenance timers. A recurrence of the Aug-16
conditions (memory pressure, repo bloat, crash surge) now alerts within minutes rather than
surfacing as exit-code −1 storms. [LIVE]

## 4. No-data-loss verification [LIVE, 2026-09-06]

Re-derived by object lookup, not by trusting §7 of the analysis:

| Claim | Check | Result |
|---|---|---|
| Attempt 3's substantive commit exists | `git cat-file -t 26dab61` | commit ✅ |
| Attempt 4's final commit exists | `git cat-file -t 7a50353` | commit ✅ |
| Pre-squash lineage intact | `git rev-parse pre-squash-history-20260816` | `7e4edf6c…` — matches the analysis ✅ |
| Squash + bloat-drop + corrected divergence commits exist | `c27899f`, `b2d8233`, `20e4c3c` | all commit objects ✅ (including `20e4c3c`, the hash §7 corrected from the phantom `a3e2981`) |
| The Domain Watch work is in current HEAD | `internal/server/handlers_watch.go` | present, exactly **162 lines** as claimed ✅ |
| ADR committed | `docs/adr/001-domain-watch-webhook-notifications.md` | present (7,734 B) ✅ |
| The backlog reached origin | `git fetch` then `rev-list --count` both directions | `origin/main..HEAD` = 0, `HEAD..origin/main` = 0 ✅ |

**Zero data loss**, confirmed against the object database. The crash cost two agent sessions and
two alert beads; it cost the work nothing.

## 5. Residual risk — what remains, measured

The fleet's `exit_code=-1` rate has collapsed but not stopped. Re-derived [LIVE 2026-09-06] from
25,227 surviving log files with an spacing-agnostic scan (the naive `"exit_code": -1` pattern
misses drawrace logs, which serialize without the space):

- Last 8 days: **8 events** (Sep-01: 1, Sep-02: 4, Sep-06: 3) — all on `drawrace` / `test-fix`
  workers, all with synthetic dispatch ids (`nd-9gi8`, `nd-3wrf`, `drawrace-*`, `pdftract-*`)
- Every one of the last 3 days' events has `workspace = /home/coding/drawrace`; **none is in the
  domain-check workspace**
- This upgrades the analysis §10's fleet-tail statement with exact figures and explains its own
  counting method: the "9 scattered events Aug-26 → Sep-06" and this bead's "8 in the last 8
  days" agree within the window difference

The remaining exposure class is test probes and internal dispatch tooling in other workspaces —
not the mechanism this crash died of, and not this repository.

## 6. What this bead deliberately does not do

1. **No new code change.** Domain-check code was not executing in a way that contributed
   (§8 of the analysis), and no gap in the existing mitigations was found (§3). `go build ./...`
   and `go test ./...` pass; both were run as the standard close gate, not because the mitigation
   touches them.
2. **No closure of the parent alert** `domchk-d46ec441`. It is blocked by **two** beads —
   `domchk-ed5d9143` and `domchk-f3c5f61d` — and the document step owns writing the closure
   record that lets both be resolved accurately.
3. **No retry of the original task.** The crashed work (Domain Watch, ADR-001) is already in
   main (§4); retrying would duplicate it.

## 7. Acceptance criteria

- [x] **Infrastructure event → document resource state** — §2
- [x] **Verify no data loss** — §4, by object lookup
- [x] **Mitigation action taken and documented** — §3: four mitigations verified in force with
      live evidence; the specific crash mechanism (unbounded `pack-objects` under memcg) is
      bounded by config and proven bounded by the 768 MiB cgroup integration test
- [x] **False-positive nuance handled** — the death was real, so the classification stays
      INFRASTRUCTURE; but it was a *post-completion* kill (work committed 61 s before death,
      self-healed by attempt 5 + the same-day squash), recorded as such in §4/§6

## 8. Handoff to domchk-f3c5f61d (document/verify)

Everything needed to close the chain accurately: the alert's *subject* (bf-2xygo divergence
analysis) completed as `20e4c3c`; the crashed work completed and is in main; root cause
INFRASTRUCTURE, kernel-confirmed; mitigation verified in force by this document; residual fleet
tail is synthetic-probe noise in other workspaces. When closing `domchk-d46ec441`, cite the
analysis (§8 classification, §5.1 kill records) and this document, and note that
`bf-198ne-context.md`'s "signal −1 = SIGKILL delivered by the OOM killer" line was corrected by
the analysis's §2 — the −1 is needle's sentinel for a signal death, and the kill was memcg-constrained
inside the dispatch scope, not the host OOM killer.

---

*Written 2026-09-06 by domchk-99f184b6 (claude-code-glm-5.3-flash, lab domain-check workspace).
Every check marked [LIVE] was executed during this bead's attempt; command outputs are in the
session transcript. Scripts run: `setup-git-gc-config.sh --verify`, `test-gc-memory-bounds.sh`
(12/12), `check-repo-health.sh`, `go build ./...`, `go test ./...`, plus direct object-database
lookups and a streaming scan of 25,227 needle log files.*
