# Remediation & Prevention Plan: bf-4x12ec

| Field | Value |
|---|---|
| **Crash bead** | `bf-4x12ec` — "Execute aggressive git garbage collection to eliminate OOM risk" (Closed 2026-08-17, work complete) |
| **Root cause** | [`bf-4x12ec-root-cause.md`](bf-4x12ec-root-cause.md) (bead `domchk-9e2aa740`); independently re-derived and **Closed** by `domchk-93526393` on 2026-09-08 |
| **This bead** | `domchk-0fcaef88` — "Propose remediation and prevention measures" (remediation child of the `domchk-be3cf290` diagnostics chain) |
| **Date** | 2026-09-08 |
| **Verification basis** | Every "verified live" claim below was re-run in this workspace during this dispatch, not cited from an earlier document |
| **Classification** | INFRASTRUCTURE — memcg OOM of bare `git gc --aggressive --prune=now` inside the 12 GiB dispatch scope. No domain-check code defect. |

---

## 0. Answers to the four acceptance criteria, up front

1. **Fix for the immediate issue** → already in place and holding; verified live
   2026-09-08 in §2. The OOM mechanism has three independent layers now
   (bounded pack memory, bounded staged gc, healthy repo), each re-verified
   below.
2. **Patterns / safeguards against similar crashes** → §4. Six gaps remain;
   three are minutes-scale commits in this repo, one is a new pre-execution
   guard, two are adoption/behavior items.
3. **Retry bf-4x12ec?** → **No. Do not retry, do not re-dispatch.** §3.
4. **Process / tooling changes and beads needing updates** → §5 and §6.

---

## 1. The causal chain being remediated

From the RCA (both the 2026-09-02 determination and the 2026-09-08
re-derivation), the crash decomposes into four conditions and four
amplifiers:

| # | Element | What it was on 2026-08-14 |
|---|---------|---------------------------|
| **C1** | Repo precondition | 4,649 loose objects / **17.20 GiB** unpacked |
| **C2** | Trigger | the **bead body itself prescribed** bare `git gc --aggressive --prune=now` (+ `repack --depth=250 --window=250`) |
| **C3** | No memory bound | `pack.windowMemory` / `pack.threads` / `pack.deltaCacheSize` did not exist yet (added 2026-09-02) |
| **C4** | Constraint | needle's transient dispatch scope, `MemoryMax=12 GiB`, `oom_score_adj=200` |
| **A1** | Amplifier | naive retry: **44 identical deterministic kills**, zero packing progress |
| **A2** | Amplifier | auto-split engaged only at 11:59:06Z, ~96 min in |
| **A3** | Amplifier | stale `gc.aggressivewindow='1.hour'` consumed the first 2 attempts (git exit 128) |
| **A4** | Amplifier | `bead.orphaned` at 12:58:55Z on a *successful* bead → 9-day false-positive alert tail |

C4 is not changeable from this repo (dispatch-scope sizing is NEEDLE-owned;
canon gap G-10). Everything else is.

---

## 2. Immediate-issue remediation — IN PLACE, verified live 2026-09-08

| Element | Remediation | Verified live this dispatch |
|---|---|---|
| **C1** | `.beads/` fully gitignored + 10 MB pre-commit gate | `git count-objects -vH` → **150 loose objects / 1,012 KiB, one 100.25 MiB pack, 0 garbage**; `.git` ≈ 105 MB. The 17.20 GiB precondition cannot rebuild through bead state (0 tracked `.beads/` files), and `./scripts/setup-git-hooks.sh --check` → "installed and byte-identical to tracked source" |
| **C2** | Bare aggressive gc prohibited by repo CLAUDE.md; replacement is staged, checkpoint/resumable `scripts/safe-git-gc.sh` | Nightly bounded remediation runs regardless: `.beads/logs/git-gc.log` ends "Safe Git GC Completed Successfully … repository size: 101M"; all 8 `domain-check-*` timers show future trigger times (`systemctl --user list-timers`) |
| **C3** | Persistent pack-memory bounds, repo-local **and** global | `./scripts/setup-git-gc-config.sh --verify` → effective worst case **≈3072 MiB** per pack run (`windowMemory=2g`, `deltaCache=1g`, `threads=1`) against the 6 GiB ceiling for a 12 GiB scope. Covers `git push`'s pack-objects too — the bf-198ne variant of this same kill |
| **A3** | Stale-config preflight | `safe-git-gc.sh` validates config and resource headroom and exits 2 before any git work; the `gc.aggressivewindow` key remains unset |

**Verdict: the immediate issue is fixed and has stayed fixed for 25 days**
(repo has been ≤ ~107 MB since 2026-09-01 across five independent
verification passes). No further code change is needed to close the original
OOM; the remaining work below is about the *amplifiers* and the *next*
memory-heavy command.

---

## 3. Retry assessment

**bf-4x12ec: NO RETRY.**

- The bead is **Closed** with its own acceptance criteria met (its Notes
  record the compaction completing; the crash happened *around* the work, not
  instead of it — attempt 53 exited 0 at 12:58:45Z).
- The repo state it targeted is not merely repaired but *maintained*: 1 pack /
  100.25 MiB today, nightly bounded gc keeping it there. Re-running its
  literal command today would be simultaneously **unnecessary** (nothing to
  pack) and **prohibited** (repo CLAUDE.md).
- The 44-kill / 8-timeout retry tail is fully explained by the RCA; its alert
  tail is handled by closed-bead filtering + dedup + cooldown (verified
  2026-09-07, see repo CLAUDE.md "Crash Alert System").

**If an equivalent task ever appears again** (repo genuinely bloated), the
correct approach is different from the one the bead prescribed:

```bash
./scripts/safe-git-gc.sh --check-only     # preflight: verdict, not failure
./scripts/safe-git-gc.sh                  # staged, bounded; --resume if interrupted
./scripts/safe-git-gc-monitor.sh --watch  # separate terminal
./scripts/safe-git-gc.sh --full           # only if the standard pass is insufficient
```

Never the bare `git gc --aggressive --prune=now` — that is the exact command
whose memcg-OOM produced this crash, bf-173o7e, bf-4yjq, and the bf-198ne
push-side variant.

---

## 4. Remaining prevention gaps

Ordered by leverage. Items 1–2 are commits, not engineering; item 3 is the
only new mechanism proposed here.

### GAP-1 (P1) — the aggressive-gc detector is untracked while its callers are tracked

`scripts/detect-unsafe-gc.sh` (flags any `--aggressive` gc that
`safe-git-gc.sh` did not launch) exists on disk and is wired into two
**tracked** callers:

- `scripts/check-repo-health.sh:96` (daily 02:00 repo-health timer)
- `scripts/resource-monitor.sh:291` (every-5-minutes resource monitor)

Both degrade to a *silent skip* when the script is missing
("detect-unsafe-gc.sh not available, skipping"). `git ls-files` confirms it
and its self-test `scripts/test-detect-unsafe-gc.sh` are **untracked** — a
fresh clone loses the detection leg with no error anywhere.

*Change:* commit both files (they are another worker's in-flight deliverable
— verify ownership via `git log --all --grep detect-unsafe-gc` / the owning
bead before taking them; do not duplicate the work). *Effort:* minutes.

### GAP-2 (P1) — a tracked wrapper depends on an untracked script

`scripts/needle-with-limiter.sh` (tracked, self-tested by
`scripts/test-needle-with-limiter-gate.sh`) sets
`LIMITER_SCRIPT="$SCRIPT_DIR/agent-concurrency-limiter.sh"` — and
`agent-concurrency-limiter.sh` is **untracked**. The tracked dispatch wrapper
is broken on a fresh clone, and the limiter has no self-test of its own.

*Change:* track `agent-concurrency-limiter.sh`; add a small self-test in the
style of the existing `test-*` scripts. *Effort:* < 1 h.

### GAP-3 (P1) — nothing blocks the lethal command *before* it runs

The detection layered in GAP-1 is observe-only and polled every 5 minutes;
bf-4x12ec's kills landed **39–116 s** after the gc was issued, so a monitor
tick can straddle an entire death. The durable guard is pre-execution, and
the mechanism already exists on this box:
`~/.claude/hooks/org-rule-guard.py` is a `PreToolUse` hook that already
blocks mutating `kubectl` verbs, `.github/workflows/*`, `kind: Job/CronJob`,
and `:latest` image tags — it has **no git rule** (verified by grep,
2026-09-08).

*Proposed rule* (insert in `check_bash`, `org-rule-guard.py:178`): deny any
Bash command invoking `git gc` or `git repack` with `--aggressive`, with a
deny message pointing at `scripts/safe-git-gc.sh`. This mirrors the
detector's structural invariant exactly — `safe-git-gc.sh` never passes
`--aggressive`, so *any* `--aggressive` request is by definition outside the
sanctioned path — and it is fail-open like the rest of the hook, with the
same "the rule binds regardless of whether the hook catches it" clause the
other prohibitions carry. Scope it to `--aggressive` only: non-aggressive gc
and push are already memory-bounded by C3's persistent config, and a broader
git-verb block would fight legitimate work.

*Effort:* < 1 h including a replay test against the bf-4x12ec command line.
This closes the *task-text* hazard too (C2): it makes a bead body that
prescribes the command un-executable rather than merely inadvisable.

### GAP-4 (P2) — the retry circuit breaker exists and is tested, but is not in the dispatch path

`scripts/crash-circuit-breaker.sh` (per-bead retry-storm breaker, A1) and
`scripts/needle-with-limiter.sh` (the wrapper that gates `needle run` /
`needle supervise` through the breaker + concurrency limiter, A1+A2) are both
tracked and tested. Nothing in the live path invokes the wrapper: no systemd
unit, no CLAUDE.md line, no doc outside its own header references it, and no
breaker state file exists under `.beads/state/` — the breaker has never
fired outside its tests. NEEDLE's internal release-and-retry loop — the thing
that re-dispatched bf-4x12ec 44 times — remains ungated at the source
(canon G-13 / design doc §9.2 Phase 4, external ask).

*Change (repo-side stopgap):* name `needle-with-limiter.sh` the sanctioned
dispatch entry point — one line in repo CLAUDE.md and one in
`docs/maintenance/repository-maintenance-guide.md` — and/or fold a breaker
status check into `scripts/preflight-health-check.sh` so every manual
preflight at least sees an OPEN breaker. *Effort:* < 1 h. The real fix stays
with NEEDLE.

### GAP-5 (P2) — the worker-side half of the orphan fix

A4's alert-side half is done (closed-bead filter, dedup, cooldown —
re-verified 2026-09-07). The worker-side half — **close the bead with
completion notes on success; never leave it to release/orphan** — is still
only a lesson-learned sentence. `scripts/verify-work-completion.sh` already
writes `.beads/state/work-completion/<bead>.json` before close.

*Change:* have `verify-work-completion.sh` emit a warning (not a failure)
when the target bead's store state is `released`/`orphaned` while a
deliverable commit exists — that is the bf-4x12ec 12:58:55Z shape, and it is
the cheapest place to catch it. Plus one line in the crash-response guide's
worker checklist. *Effort:* < 1 h.

### GAP-6 (P3) — fold these into the canon requirements doc

`docs/crash-prevention-requirements.md` is the single requirements list
(G-1…G-13). GAP-1/2 here are instances of a general gap that doc does not yet
name: **"a tracked caller with an untracked dependency silently degrades."**
GAP-3 generalizes as: **"detection is not prevention; hazardous verbs need a
pre-execution gate."** Recommend the canon's owner add both as gap items with
dated corrections, rather than this plan editing a doc another bead
actively maintains — same policy the corpus applies to frozen docs, applied
prudently to a hot one.

---

## 5. Process and tooling recommendations (canonical mapping)

Nothing in this plan reopens a requirement the canon already withdrew — in
particular **G-2 stays withdrawn** (unconditional nightly bounded gc
dominates any conditional variant), and **G-6 stays reclassified** (service
hygiene, not fleet crash prevention). The bf-4x12ec-specific deltas:

| Recommendation | Maps to | New here? |
|---|---|---|
| Pre-execution block of `--aggressive` gc (GAP-3) | new — extends the org-rule-guard mechanism to the crash corpus's top killer | ✅ |
| Track the detector + limiter scripts (GAP-1/2) | new — "untracked dependency of a tracked caller" | ✅ |
| Wire the breaker gate into the dispatch path (GAP-4) | canon G-13 (external), repo-side stopgap | half |
| Worker closes on success (GAP-5) | canon G-9's worker-side twin | half |
| Evidence retention (kernel/journald ≥30 d, transcripts ≥30 d — the Aug-14 kernel logs that would have settled this crash in one query are gone) | canon G-8, already ranked | re-affirmed |
| Read `exit -1` as "died by signal, mechanism unknown" | canon R-DOC-1 | re-affirmed |

## 6. Beads needing updates

| Bead | State observed live 2026-09-08 | Needed action |
|---|---|---|
| `bf-5jhvpk` (repack child) | **Open** rev 18, **Notes empty** — yet the repack is recorded as executed 2026-09-08T00:28Z by `domchk-371e54d8`, and live state matches it (1 pack / 100.25 MiB) | Owner should append a dated note with the verification figures and close; the empty-Notes-plus-executed-work shape is consistent with a gate-reverted close — re-close, do **not** re-run anything |
| `bf-im2sl1` (verify child) | **Open** rev 1, untouched since 2026-08-14 | Close as subsumed — its verification criteria are met by current repo state (§2); cite this doc's live figures |
| `domchk-30b53d74` (umbrella, `failure-count:4`, `verification-failed`) | Open | Disposition stale: root cause determined (`domchk-93526393`, Closed) and remediated (§2) — refuse + close per the auto-split alert-loop policy |
| `domchk-3152117c` ("Document crash resolution") | Open | Its deliverable is substantially satisfied by [`bf-4x12ec-final-crash-report.md`](bf-4x12ec-final-crash-report.md) + this plan — close with pointers, no new report |
| `docs/crash-prevention-requirements.md` owner (`domchk-d7c086d6`) | — | Fold GAP-1/2/3 generalizations into §4 (GAP-6) |

## 7. Sources

- [`bf-4x12ec-root-cause.md`](bf-4x12ec-root-cause.md) — formal RCA (domchk-9e2aa740)
- Bead `domchk-93526393` — 2026-09-08 independent re-derivation, Closed (this plan's prerequisite)
- [`bf-4x12ec-final-crash-report.md`](bf-4x12ec-final-crash-report.md) — consolidated narrative + lessons
- [`docs/crash-prevention-requirements.md`](../../crash-prevention-requirements.md) — G-1…G-13 canon
- [`docs/crashes/bf-198ne-crash-report.md`](../crashes/bf-198ne-crash-report.md) — the push-side memcg variant
- Live verification commands this dispatch: `git count-objects -vH`, `git ls-files`,
  `scripts/setup-git-gc-config.sh --verify`, `scripts/setup-git-hooks.sh --check`,
  `systemctl --user list-timers 'domain-check-*'`, `.beads/logs/git-gc.log`,
  `grep -n check_bash ~/.claude/hooks/org-rule-guard.py`
