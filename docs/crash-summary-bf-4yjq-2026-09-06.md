# bf-4yjq — Final Crash Summary and Lessons Learned

**Dispatch bead:** domchk-be242408 (documentation and knowledge capture)
**Date:** 2026-09-06
**Subject bead:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo mirror has
diverged/gone stale" (P2, **closed 2026-08-17**, verified live this dispatch)
**Classification:** **INFRASTRUCTURE — repository bloat** (kernel memcg OOM SIGKILL inside the
12 GiB dispatch scope; not a code defect, not a service failure, not a false positive)

This is the closing summary of the bf-4yjq investigation. It consolidates; it does not
re-derive. Every figure below is either carried from the canonical sources in §7 — each of
which was itself live-verified — or was re-executed live for this dispatch on 2026-09-06 and
marked **[LIVE]**. Where an older bf-4yjq document disagrees with this summary, the canonical
sources win; the known disagreements (9 vs 50 crashes, SIGHUP vs memcg OOM) are catalogued in
§7 of `docs/investigations/investigation-report-final-2026-09-06-domchk-e843c4f1.md`.

---

## 1. What happened, in one table

| | |
|---|---|
| **The task** | Repoint `origin` at Forgejo, reconcile the Forgejo/GitHub divergence with a merge commit (no force-push), configure the Forgejo→GitHub server-side push mirror. A real task, not a crash-investigation bead |
| **The event** | 50 consecutive agent deaths, 100 % exit code −1, zero variation, 17:54:00 → 20:30:43 UTC on 2026-08-12 — one every ~3.1 min for 2 h 37 m |
| **Context** | Middle shift of a same-day workspace storm: 455 exit −1 events across 6 beads over ~18.5 h (bf-31mno 350, bf-4yjq 50, bf-1s6c3 49, …) |
| **Root cause** | ~18 GB repository with 17.2 GiB loose objects (≈1,800:1 loose:packed — committed ~237 MB `.beads/*.jsonl` snapshots); each dispatch's first store-walking git operation exceeded the dispatch scope's `MemoryMax=12GiB` → cgroup-scoped memcg OOM SIGKILL. The host was not out of memory |
| **Why 50** | One deterministic environmental condition × a zero-backoff re-dispatch loop. No retry could shrink the trigger — the kill left the loose set untouched |
| **The two co-requisite defects** | (1) unbounded *accumulation* (`.beads/` tracked, no size gate); (2) unbounded *operation* (no memory ceiling on pack-objects). Fixing either alone leaves the crash reachable |
| **The fix** | Both layers, already deployed — see §2 |
| **Task outcome** | Completed after the Aug 13–14 cleanup (18 GB → ~94 MB), closed 2026-08-17, still verified correct |

---

## 2. Remediation review (child bead domchk-f49dfe47)

The remediation link of this bead's chain —
domchk-f49dfe47, "Implement remediation for crash bf-4yjq" — closed 2026-09-06 with
the verdict **AUTO-SPLIT REFUSED, BEAD SATISFIED**: the split premise ("failed 3×, too
complex") was worker deaths in a fleet exit=1 wave, not task complexity, and the remediation
it was asked to implement was already deployed, committed and pushed. Its closing note
re-verified every Infrastructure-branch acceptance criterion live (6 systemd timers firing;
mitigation docs on origin/main; infra changes documented in CLAUDE.md) and recorded the
mechanism bounds (`.git` 97 M, `--verify` exit 0).

This dispatch independently re-executed the load-bearing checks rather than quoting that note
— all green:

| Check (run 2026-09-06) | Result |
|---|---|
| `./scripts/setup-git-gc-config.sh --verify` | **exit 0** — effective bound resolves local, worst case ≈3072 MiB per pack run, within the ceiling for a 12 GiB dispatch scope |
| Repository state `git count-objects -vH` + `du -sh .git` | **[LIVE]** 136 loose objects, 10,980 in-pack, 1 pack (90.93 MiB), 0 garbage, `.git` **97 M** |
| Monitoring | **[LIVE]** all 6 `domain-check-*` user timers armed with future trigger times |
| Crash-pattern detector | **[LIVE]** `./scripts/crash-pattern-detection.sh --quiet` → exit 0 (stable) |
| Surge threshold (committed, HEAD) | **[LIVE]** `CRASH_SURGE_THRESHOLD=3` — 3 crashes in 5 min = infrastructure event (see §5, gap 1) |

**Verdict: remediation is complete and holding.** No new technical work is required or
performed here; this bead is documentation only, matching its scope.

### The two fix layers (deployed)

- **Layer A — stop the accumulation:** whole-`.beads/` gitignore (+ `*.db`/`*.jsonl`
  repo-wide, 0 tracked `.beads/` files); 10 MB pre-commit gate; daily repo-health timer with
  >500 MB loose-object alerts; the standing bloat itself repaired.
- **Layer B — bound the operation:** `pack.windowMemory=2g`, `pack.deltaCacheSize=1g`,
  `pack.threads=1` repo-local **and** box-global — covering bare `git gc` *and* the
  `git push` pack path (bf-198ne was the push-side variant) — plus `safe-git-gc.sh` as the
  sanctioned cleanup path and `--verify` as the fail-closed check.

---

## 3. Consolidated findings

1. **Root cause: repository bloat; mechanism: cgroup-scoped memcg OOM.** Confidence HIGH on
   the bloat correlation, MEDIUM-HIGH on the Aug-12 mechanism itself — Aug-12's kernel logs
   do not survive, so the mechanism rests on uniform exit signatures, the crashes stopping
   exactly when the trigger was removed, kernel-record proof of the same kill class for the
   Aug-14 (bf-4x12ec) and Aug-16 (bf-198ne) variants, and an end-to-end reproduction at
   1/17th scale (`scripts/test-bf-4yjq-crash-condition.sh`, 6/6 assertions × 3 runs).
2. **The historical record undercounted the event.** Alert-bead sampling recorded **9**
   crashes; the forensic checkpoint shows **50**, plus a 350-kill storm (bf-31mno) recorded
   nowhere. Scale must be derived from `.beads/checkpoint/forensic.jsonl`, never from the
   alerts an investigator happened to find.
3. **`exit -1` is needle's sentinel for a signal death with no recorded code** — not a
   signal number, and never evidence of SIGHUP specifically (a SIGHUP death would surface as
   129). This single misreading was the corpus's largest source of misclassification.
4. **The binding memory constraint was the dispatch scope, not the host.** Host-wide
   memory reasoning ("62 GB total") and host-wide alerting both miss the boundary that
   actually kills.
5. **The crashes were incidental to the task's content** — bf-4yjq was a git-remote task
   whose *scheduling* inside the bloat window killed it. The older "BLOCKED, not actively
   executing" claim does not survive the ~3-minute re-dispatch-kill cadence.
6. **The wider cost was the duplicate-alert wake**, not the kills: bf-4yjq attracted
   132 title-matching beads over the following weeks (45 open, 7 in_progress, 80 closed —
   **[LIVE]** recount, titles matching `bf-4yjq`, 2026-09-06), several generating
   investigation dispatches for an already-resolved event.

---

## 4. What worked well in the investigation process

These are the process behaviors that produced a correct answer out of a corrupted evidence
base; they are worth repeating on the next storm.

1. **The chain decomposition held.** Analyze artifacts → classify → remediate → document,
   one bead per link, each producing a durable cited deliverable. The final link (this one)
   could verify instead of redo because the earlier links committed their evidence rather
   than leaving conclusions in bead notes.
2. **Re-derivation over quotation.** Every load-bearing figure in the canonical record was
   re-counted from the forensic checkpoint, not copied forward. That discipline is what
   caught the 9 → 50 undercount and the mis-dated SIGHUP window.
3. **Supersession banners on superseded documents.** Older reports keep their telemetry but
   carry banners pointing at the corrections list, so stale figures are visible as stale
   instead of silently competing with current ones.
4. **The dedup protocol worked in both directions.** `git log --grep <bead-id>` plus reading
   the bead's own deliverable before starting prevented duplicate classification docs
   (domchk-4075ffa8 refused to write a second one) — and equally proved that *this* bead's
   deliverable, a final summary at its literal requested path, was genuinely missing.
5. **Mechanical guards over convention.** The durable fixes are gitignore rules, a pre-commit
   size gate, and persistent git config — things that cannot be forgotten under pressure,
   rather than procedures that must be remembered.
6. **Evidence was extracted before it rotated away.** The surviving Aug-12 primary source was
   a single log-rotation slot; the raw logs, session transcripts and manifests were committed
   (77fac01) while they still existed, and the artifact catalog records what did and did not
   survive.
7. **The mechanism was re-created as a condition, not an event.** A 1/17th-scale harness
   reproduced the memcg OOM and proved both mitigations neutralize it — without rebuilding
   17 GiB of loose objects on the live repo, which would itself be the hazard the guardrails
   exist to prevent.

---

## 5. Gaps in monitoring and alerting that bf-4yjq exposed

Full inventory with priorities lives in `docs/crash-prevention-requirements.md` (gaps
G-1…G-13); the items this crash specifically demonstrated:

1. **Surge detection fired too late — the threshold was wrong for slow-burn storms.**
   bf-4yjq's cadence peaked near 5 crashes/10 min, sliding under the then-documented
   10-in-10-minutes rule while it killed agents for 2.5 hours. The committed detector now
   fires at **3 crashes in 5 minutes** (`CRASH_SURGE_THRESHOLD=3`), and the working tree adds
   the canonical report's recommended workspace-aggregate signal (≥10 crashes/hour sustained)
   — **that improvement is still uncommitted as of 2026-09-06** (see §6).
2. **Host-wide monitoring cannot see the kill boundary.** The deaths happened inside
   per-dispatch `MemoryMax=12GiB` cgroups while the host had memory to spare
   (gap **G-10**). Scope-level memory pressure is the signal that matters.
3. **The retry loop amplified one kill into 50** (gap **G-11**, needle-side): zero-backoff
   re-dispatch turned a deterministic environmental kill into a storm and then into dozens of
   phantom investigations.
4. **Evidence retention is inadequate for the next investigation** (gap **G-8**): no journald
   before Aug-15, single-slot session traces, and UTC/EDT timestamp mixing meant the Aug-12
   mechanism had to be established by inference. Retain kernel OOM + journald ≥30 days, rotate
   rather than overwrite traces, stamp UTC.
5. **Detection without remediation** (gap **G-2**): repo-health monitoring could see bloat
   approaching before any crash; nothing acted on it. Detection that does not close the loop
   only documents the failure in advance.
6. **Alert hygiene:** the duplicate-alert wake is still draining attention (45 open /
   7 in_progress title-matching beads). A one-time bulk-close of alerts whose targets are
   already resolved is warranted; until then every new bf-4yjq dispatch pays the dedup tax.

---

## 6. Open items (not this dispatch's scope)

- **Uncommitted detector improvements** (`scripts/crash-pattern-detection.sh`: window
  filtering, ≥10/h storm-rate signal, DEGRADED semantics, `--quiet` fixes) — the file's own
  fix-history attributes them to closed beads domchk-0c601026 and domchk-f49dfe47, but no
  commit carries them; they are live on this box and run under the monitoring timer, but a
  fresh clone would not have them. They need a commit owner.
- **Stale documents still repeating the 9-crash figure** without a banner — refresh owned by
  bead domchk-7625a5cc.
- **Alert backlog** bulk-close (§5.6), and the open duplicate sibling
  **domchk-a960d30b** ("Implement remediation for crash bf-4yjq", open since 2026-08-26) —
  identical in title to the closed domchk-f49dfe47 and satisfied by the same deployed
  remediation; a candidate to verify-and-close rather than re-execute.
- **Scaled harness chain:** domchk-30e8aab9 / domchk-10404857 / domchk-3e443d56 /
  domchk-fbb7bbbd own the recurring execution of the crash-condition harness.

---

## 7. Source index

| Document | Role |
|---|---|
| `docs/crash-investigations/bf-4yjq-crash-investigation.md` | **Canonical crash record** — verified 50-crash count, storm table, task outcome, recommendations |
| `docs/crashes/bf-4yjq-consolidated-findings-domchk-4ed0544b-2026-09-06.md` | Consolidated findings: quick reference, evidence table, two-layer fix, lessons, source index |
| `docs/crash-investigations/bf-4yjq-consolidated-summary-domchk-ea5c6a63-2026-09-06.md` | Consolidated summary reconciling the two pre-verification artifacts |
| `docs/crash-investigations/bf-4yjq-crash-classification-domchk-48e02d6f-2026-09-06.md` | INFRASTRUCTURE verdict under the response guide's four-way taxonomy |
| `docs/crash-investigations/bf-4yjq-root-cause-determination-domchk-54bc57df-2026-09-06.md` | Root-cause determination, incl. SIGHUP ruled out on primary evidence |
| `docs/crashes/bf-4yjq-fix-proposal-verification-2026-09-06.md` | Fix description, trade-offs, residual risks |
| `docs/crashes/bf-4yjq-cleanup-verification.md` | Repository repair record (18 GB → ~94 MB, holding) |
| `docs/crash-investigations/bf-4yjq-artifact-catalog-2026-09-06.md` | Live-verified evidence inventory — what survives and where |
| `docs/crash/bf-4yjq/raw-logs/` | Preserved primary sources (committed 77fac01): worker log, needle events, 56 session transcripts |
| `docs/crashes/bf-198ne-crash-report.md` | Kernel-record proof of the mechanism class (push-side variant) |
| `docs/crash-prevention-requirements.md` | Live-verified safeguard inventory and the G-1…G-13 gap list |
| `docs/crash-response-guide.md` | Triage procedure — updated this dispatch (exit −1 semantics, scope-level memory check, surge threshold) |

---

*Consolidated for dispatch domchk-be242408, 2026-09-06. Checks marked **[LIVE]** were
executed for this summary on the working repo (`.git` 97 M, 90.93 MiB single pack, 0 garbage,
6 timers armed, detector stable) — not quoted from an earlier record.*
