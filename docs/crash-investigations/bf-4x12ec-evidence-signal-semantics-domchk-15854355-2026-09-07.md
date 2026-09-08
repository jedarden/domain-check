# bf-4x12ec — Evidence Collection & Signal −1 Semantics (domchk-15854355, 2026-09-07)

Split child 1 of 4 of umbrella `domchk-4adc1a55`. Scope per the bead: **gather
primary-source evidence only — no synthesis, no conclusions.** Root-cause
determination, resource correlation, workload/reproducibility assessment and
retry-safety classification belong to the sibling children (`domchk-5f3ec6e1`,
`domchk-78d89c6b`, `domchk-dba1e0bb`) and the umbrella.

Everything below was executed or read first-hand on 2026-09-07 against repo
HEAD `8ae57ea`. **Line numbers are HEAD line numbers**, taken with
`git show HEAD:<path> | grep -n` — not worktree line numbers. That distinction
matters here: the shared worktree copy of the canonical report carries a
co-tenant's uncommitted edits (§3.2), and one earlier check this attempt
(`git diff --quiet HEAD -- <path>`) reported the file clean while the blob
differed from HEAD — the known `git diff HEAD` hazard on a mass-staged tree.
The blob-id check (`git rev-parse HEAD:<path>` vs `git hash-object <path>`)
is what actually caught it. Where a figure comes from a bundle file rather
than a live re-run, that is stated.

---

## 1. What `signal -1` / `exit code -1` means in the NEEDLE harness

**The finding:** `-1` is a *writer-side sentinel* meaning "the child process
produced no exit status". It is **not a POSIX signal** — Linux signals number
1–64 (`signal(7)`), so no signal −1 exists. The string `signal -1` seen in
alert bodies is needle's alert renderer applying `code − 128 if code > 128`
arithmetic to that sentinel; it identifies no signal.

### 1.1 Where it is defined / observed (all primary)

| Source | What it establishes | Verified this attempt |
|---|---|---|
| `docs/signal-analysis-exit-code-negative-one.md` (updated 2026-09-07, bead `domchk-15999f2c`) | The authoritative technical record: §1 negative-exit encodings per recorder language, §2 needle's recording sites + outcome classification table, §3 the eight termination causes that surface as −1 with the evidence that decides each, §4 recorded-convention vs real signal number, §5 corrections to this doc's own 2026-09-02 body, §6 evidence-source commands | Read in full; §1–§6 figures re-executed independently by its own bead the same day (three same-day passes recorded in the file) |
| `docs/crash-response-guide.md:15` (exit-code table) + `:28` (note 2) | Triage-facing statement: "`exit -1` is a sentinel, not a signal number. Needle writes `-1` for any signal death with no recorded code (`code().unwrap_or(-1)`); the correct Unix encoding of a SIGKILL death is 137 … Never assert a specific signal from `-1` alone" | Read at HEAD `8ae57ea` |
| NEEDLE source, `~/NEEDLE` HEAD `01ecf05` (clean tree) | `src/dispatch/mod.rs:1861,1905,2082,2579` each read `status.code().unwrap_or(-1)` — `ExitStatus::code()` is `None` on a signal death, so **every signal flattens to −1**; `src/dispatch/mod.rs:103` pins needle's own deadline kill to **124** (GNU timeout convention), so a −1 record is never a needle timeout; `src/outcome/mod.rs:1500–1504` holds the `signal_code > 128 → signal_code − 128` arithmetic that mints the `signal -1` label | Re-read this attempt (all four `unwrap_or(-1)` sites, both other pins). The sibling doc pins `9d09220`; the lines are unchanged at the current HEAD `01ecf05` |
| Live bead store | Alert bead `bf-3m9m1v` (Closed, rev 3) body carries, verbatim: `- **Exit code**: -1 (signal -1)` and `Timestamp: 2026-08-14T10:25:30.457683731+00:00` | `bead show bf-3m9m1v` run this attempt |

Consequences recorded by those sources (restated here as their content, not as
new analysis): a −1 record cannot name the signal that killed the child; the
real signal is recoverable only from kernel/journald records or needle's
human-readable `terminated by signal {sig}` log lines; a −1 is never a needle
timeout (that is 124); and `classify()` maps any negative code to
`Outcome::Crash(code)` (`src/types/mod.rs:415-432`, per the sibling doc).

### 1.2 Terminology note for later children

This fleet's artifacts use three spellings interchangeably — `exit code -1`,
`signal -1`, and "SIGKILL"-as-inference. Only the first two are recorded facts;
any specific signal number (SIGKILL=9 or otherwise) is **inference** layered on
top, and the canonical report's own Addendum 2 downgrades its body's "Signal -1
= SIGKILL" wording to exactly that (see §3.1, C4 below).

---

## 2. Primary-source records for bf-4x12ec: timestamps and attempt counts

### 2.1 The attempt census — 53 dispatches on 2026-08-14

Primary record: `docs/crashes/bf-4x12ec/attempt-index.tsv` (54 lines = header +
53 attempts, with claim/dispatch/completion timestamps, exit codes, durations,
classifications, release reasons and the **absolute line number of each event
in the original worker log**). Worker:
`claude-code-glm-4.7-lab-domain-check`, needle session `a6dbb1fc`, template
`pluck/pluck-default` (prompt 71,698 bytes) for attempts 1–47,
`split/split-default` (3,896 bytes) for attempts 48–53.

Counted from that TSV this attempt (`awk -F'\t'` on column 5):

| Class | Attempts | Count | First completion (UTC) | Last completion (UTC) |
|---|---|---|---|---|
| `crash`, exit **−1** | 1–44 | **44** | `2026-08-14T10:23:02.958717335Z` (attempt 1) | `2026-08-14T11:27:26.173929030Z` (attempt 44) |
| `timeout`, exit **124** (600 s dispatch cap) | 45–52 | **8** | `2026-08-14T11:38:07.866513606Z` | `2026-08-14T12:50:14.282827911Z` |
| `success`, exit **0** (auto-split template) | 53 | **1** | `2026-08-14T12:58:45.113834930Z` | — |

Every one of the 44 −1 attempts was classified `crash` and released
`release_success` with `handled_action: alerted`; the 8 timeouts were
classified `timeout` and released `deferred`. Attempt 53 ran needle's
auto-split (created children bf-173o7e / bf-5jhvpk / bf-im2sl1 and closed the
parent), then `verification.passed` at 12:58:45.126649351Z and
`bead.orphaned` at 12:58:55.502272008Z.

### 2.2 The named crash instant — and what it actually stamps

The crash this split's umbrella was dispatched on is the one named in alert
bead **bf-3m9m1v** ("ALERT: Agent crash on bead bf-4x12ec", created
`2026-08-14T10:25:30.464340624Z`). Its `Timestamp:` field —
`2026-08-14T10:25:30.457683731+00:00` — is **not the kill**. Resolution from
`docs/crashes/bf-4x12ec/README.md` (bracket table) and
`bracket-source-lines.tsv`:

| Event | Timestamp (UTC) | Worker-log line |
|---|---|---|
| Attempt-2 claim | 10:23:16.800695191Z | 4440 |
| Attempt-2 dispatch | 10:23:16.810887451Z | 4449 |
| **Kill: `agent.completed` `exit_code: −1`** (duration 104,481 ms) | **10:25:01.512001992Z** | 4452 |
| `outcome.classified` → crash | 10:25:01.515989118Z | 4455 |
| `HANDLING_RELEASE_DONE` heartbeat | 10:25:30.457670958Z | 4465 |
| `Timestamp:` in bf-3m9m1v (the named instant) | 10:25:30.457683731+00:00 | bead store |
| bf-3m9m1v created | 10:25:30.464340624Z | bead store |

The named instant is the post-kill handling heartbeat, **28.95 s after the
actual kill** (this kill's handling held a 23 s release window between flush
completion at 10:25:07.328Z and release at 10:25:30.457Z). Needle pre-0.4.2
minted **one alert bead per kill — 44 in total**, first `bf-fmg2cw`
(10:23:11.219513632Z), last `bf-5x69lm` (11:28:02.194277764Z). The full
per-kill table is
`docs/crash-investigations/evidence/bf-4x12ec/crash-logs/alert-beads-exit-timestamps.txt`
(44 rows, keyed to the forensic checkpoint), and its header states the same
heartbeat rule.

So: **bf-4x12ec's crash record is 44 distinct kills with 44 distinct alert
timestamps, not one crash** — and the umbrella's "signal -1 crash on
bf-4x12ec" resolves to attempt 2 of that 53-attempt sequence.

### 2.3 Where the primary records live

| Location | Contents | Status this attempt |
|---|---|---|
| `docs/crashes/bf-4x12ec/` | Attempt index, attempt-2 bracket (raw worker-log lines 4411–4473), full 1,146-record event extract (`needle-events-2026-08-14-bf-4x12ec.jsonl.gz`), attempt-2 session transcript (`session-transcript-attempt2-971486ad.jsonl`, 28 records), `MANIFEST.sha256` | `sha256sum -c MANIFEST.sha256` → **all 6 files OK**; `zcat … \| wc -l` → **1,146 lines**; exit distribution recounted from the TSV → **44 / 8 / 1** as above |
| `docs/crash-investigations/evidence/bf-4x12ec/crash-logs/` | Sibling bundle (`domchk-48f3e34d`, commit `9b32085`): crash-window worker-log segment, four session transcripts (attempt-1 crash, mid-storm, autosplit-timeout, split-success), the 44 alert-bead records + timestamp table, per-attempt outcome timeline | Both bundles independently pulled **exactly 1,146 records** from the same source log with matching per-attempt counts |
| `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-14.jsonl` | The durable original (2,598,910 bytes / 10,138 lines). Only Aug-14 log carrying bf-4x12ec events (drawrace / roam-1 / roam-2 / s1 / test-fix: 0) | Per bundle README's negative-findings table; not re-opened this attempt (bundle hashes verified instead) |
| `.beads/logs/crash-monitor.log` | **No bf-4x12ec coverage at all** | `grep -c 4x12ec` → **0 hits**; earliest entries across every `.beads/logs/` file are 2026-09-01/02 — the monitors postdate the crash by ~18 days. **The acceptance criterion's `.beads/logs/crash-monitor.log` step is answered by absence**: the Aug-14 crash log survives only in `~/.needle/logs` (+ the session transcripts in `~/.claude/projects`) |
| `.beads/traces/` | No `bf-4x12ec` slot (traces are single-slot per dispatch; Aug-14 slots reclaimed) | Per bundle README; consistent with `docs/signal-analysis-exit-code-negative-one.md` §2's list of surviving surfaces |
| Kernel journal (`journalctl -k`) | **Unrecoverable for this bead** — the single surviving boot starts 2026-08-15 19:56:33 EDT, after the crash | Recorded by the bundle README; the mechanism therefore stays *regime-matched, not kernel-proven* for bf-4x12ec specifically (kernel memcg evidence exists for the family: Addendum 3 of the canonical report, and the 525/525 `CONSTRAINT_MEMCG` journal tally in `docs/signal-analysis-exit-code-negative-one.md` §3) |

### 2.4 What the killed attempts were doing (transcript evidence)

From `session-transcript-attempt2-971486ad.jsonl` (read for this attempt's
summary; the file is hash-verified): `git count-objects -vH` → 4,649 loose
objects / 17.20 GiB at 10:23:26.221Z; `du -sh .git/` → 18G at 10:23:43.210Z;
`free -h` → 62Gi total / **50Gi available** at 10:23:56.490Z; **final record at
10:24:08.660Z is the `Bash` `tool_use` for `git gc --aggressive --prune=now`
(timeout 600000) with no matching `tool_result`** — i.e. the transcript ends
mid-tool-call, ≥52.9 s before the recorded kill instant. The same
fatal-command shape is documented for attempt 1 in the sibling bundle's
`transcript-attempt1-crash-8b2a5b0d.jsonl`. (Host RAM being plentiful at the
last reading is recorded here because it is in the transcript; what that
implies is child 2's question, not this doc's.)

---

## 3. State of the existing bf-4x12ec investigation docs

So later steps do not re-derive them. All paths under `docs/` unless noted;
all clean vs HEAD `8ae57ea`.

### 3.1 The four RCA-bearing reports

| Doc | Lines | What it holds | Status |
|---|---|---|---|
| `crash-investigations/bf-4x12ec-crash-investigation.md` | 592 at HEAD | **Canonical report, v1.6 at HEAD** = body + Addenda 1–6 (L155 review, L186 Add 2 primary-source retry-storm analysis, L297 Add 3 independent verification + kernel evidence, L400 Add 4 session transcripts / gc config defect / what attempt 53 actually did, L505 Add 5 re-verification + summary correction, L541 Add 6 OOM-corroboration count correction). HEAD anchors: sentinel statement L261, attempt-53 auto-split correction L425 + L434, body-retained-as-historical-record precedent L552–562 | Authoritative version is **in the addenda**; the body retains pre-correction wording in six specifics (below). **Worktree copy is 619 lines** — a co-tenant's uncommitted v1.7 edits (see §3.2) |
| `crash-investigations/bf-4x12ec-final-crash-report.md` | 297 | "Final Consolidated Report": timeline, RCA (incl. *why host memory figures do not contradict it*), debunked alternatives, work-completion status, patterns, lessons, source-document table | Post-addendum consolidation; already carries the corrected story |
| `crash-investigations/bf-4x12ec-root-cause.md` | 232 | Root-cause determination with a 5-part verification section (retry storm re-parse, kernel corroboration, host-memory reconciliation, the code path that died, current repo state) + "why it took 53 attempts", debunked alternatives, reproducibility, **evidence limits** and disposition sections | Current; its §"Evidence Limits" is the right citation when a later child needs to state what is and isn't proven |
| `crash-investigations/bf-4x12ec-section-inventory-domchk-f6aba211-2026-09-07.md` | 141 | Inventory of the canonical report against 4 required sections: **4/4 present**, plus 6 internal contradictions (C1–C6) and a gap-fill checklist | Delivered 2026-09-07. **Its line citations do not resolve at HEAD `8ae57ea`** — it says "593 lines" and cites Add 2 L258–265 / Add 4 L425–436 / Add 5 L529–534, but the HEAD file is 592 lines and those passages sit at L261, L425 and L552 respectively. It was evidently taken against a worktree state (this is the same class of drift this doc's header warns about); the *content* of all six findings C1–C6 is accurate — only the line numbers need re-deriving via `git show HEAD:` |

### 3.2 Gap-fill status: partially executed, **uncommitted**, in the shared worktree

The inventory's checklist is being worked by a sibling bead —
**`domchk-791bfb2e`** — whose edits sit **uncommitted in the worktree copy** of
the canonical report (619 lines vs HEAD's 592; +31/−4; worktree blob
`0ea13da7…` vs HEAD blob `3a5892b…`). What the uncommitted delta contains:

- a "**Metric provenance (re-verified live 2026-09-07, bead domchk-791bfb2e)**"
  block reconciling the loose-object counts — recording **4,515**
  (`docs/cleanup-resolution-2026-08-17.md`), **4,627** (bf-4x12ec's own
  description/notes) and **4,649** (the crash-time live `git count-objects`
  in the killed agents' transcripts) as three genuine readings at three
  instants. This **resolves inventory item C6**;
- a version bump to **v1.7** with an attribution correction to Addendum 4
  (the 753 MB final metrics belong to the parent bead's record, not only to
  child bf-173o7e);
- a live re-check appended: `.git` 107 MB / 467 loose / 11,700 in-pack.

What is **still uncorrected at HEAD — and also still in the worktree copy**,
checked against both:

- **C1 / Summary L4:** "…completed on the 53rd attempt at 12:58:45Z" — Addendum
  4 (HEAD L425, L434) corrects it: attempt 53 ran needle's *auto-split*; the gc
  completed under child bead bf-173o7e.
- **C2 / Resolution step 2, L64:** "completed on retry" — no bf-4x12ec attempt
  ever completed the gc (44 × −1, 8 × 124).
- **C4 / Signal Analysis L45–50:** heading "**Signal -1 Definitive
  Identification**" with "Signal -1 = **SIGKILL (Signal 9)**" — contradicted by
  §1 of this doc and by the report's own Addendum 2 (HEAD L261: "not a normal
  exit. It is not itself a POSIX signal").
- **C3 / C5** (body root-cause mechanism vs Addendum 3's kernel figures; the
  unverifiable secondhand "System State" figures incl. the "9 systematic
  crashes on bf-4yjq" line) — unaddressed in either copy.

Two cautions for whoever lands the harmonization:

1. **HEAD-vs-worktree line numbers.** The inventory's own line citations don't
   resolve at HEAD (§3.1), and once `domchk-791bfb2e`'s edits commit, +24 more
   lines shift everything after the metrics table. Cite content, or re-derive
   with `git show HEAD:<path> | grep -n` at citation time.
2. **The precedent is itself about this hazard.** Addendum 5 (HEAD L552–562)
   records that a *previous* uncommitted working-tree edit to this same report
   wrote a fabricated 57-minute gc runtime into the Summary, was refuted by
   Addendum 2's primary-source analysis, and was reverted — with the body's
   v1.0/v1.1 sections retained as historical record. Annotate, don't rewrite;
   and never treat a worktree-only reading of this file as delivered.

These edits are not this bead's scope. Recorded so the harmonizing attempt does
not re-derive them, and so no later child cites the body's superseded wording
as current.

### 3.3 Supporting and superseded docs

| Doc | Lines | Role | Status |
|---|---|---|---|
| `crash-investigations/bf-4x12ec-crash-artifacts-2026-09-02.md` | 171 | Artifact inventory of the Aug-14 storm (bead `domchk-2ff261ce`); headline "not one crash — the first of 44" | Current for its scope; superseded for counts by the 2026-09-07 bundles |
| `crash-investigations/bf-4x12ec-log-review-2026-09-02.md` | 126 | Independent re-verification of the primary worker log (bead `domchk-30d451d3`), extends Addendum 2 | Current |
| `docs/crash-investigation-bf-4x12ec.md` (repo root docs/) | 180 | **Older** investigation, dated "Crash Time 2026-08-14T11:14:39.917375296+00:00" | Superseded framing. Its "crash time" is *another* attempt's release heartbeat (the alert-stamp rule of §2.2), not a distinct crash instant — do not cite it as a second crash time |
| `docs/notes/bf-4x12ec-crash-investigation.md` | 148 | Consolidated record, reconstructed 2026-09-07; child 1 of the *other* split (`domchk-c99cdf80`, bead `domchk-c1c0afd8`), later sections appended by that split's siblings | Current; separate split lineage from this one |
| `docs/crash-reports/bf-4x12ec-verification-report.md` | 77 | 2026-08-26 verdict: **FALSE POSITIVE — work completed successfully** | Verdict still valid (target bead bf-4x12ec is Closed, rev 4, with final metrics in its notes); its era predates the 53-attempt reconciliation |
| `docs/archive/crash-investigations/` — 12 bf-4x12ec files: `crash-context-bf-4x12ec-summary.md`, `crash-summary-bf-4x12ec-comprehensive.md`, and 10 `verification-report-bf-*-resolved-bf-4x12ec-crash.md` | — | The false-positive / duplicate-alert resolution records for the other 43 kills | **Frozen archive** — cite, never edit. Note: `docs/crash-context-bf-4x12ec-summary.md` (the *un-archived* path) **does not exist** — it is cited by that filename in `crash-investigations/bf-4x12ec-log-review-2026-09-02.md` and in the final report's source table; the file lives only under `docs/archive/crash-investigations/`. Chasing the un-archived path wastes an attempt |

### 3.4 Artifact bundles (evidence, not analysis)

- `docs/crashes/bf-4x12ec/` — committed 2026-09-07 (`c781138`, bead
  `domchk-4bad8e94`, which re-verified a dead predecessor's extraction).
  **Hash-verified this attempt.**
- `docs/crash-investigations/evidence/bf-4x12ec/crash-logs/` — committed
  2026-09-07 (`9b32085`, bead `domchk-48f3e34d`). Complementary, not a
  duplicate: holds the four *other* session transcripts and the 44-bead alert
  table that the first bundle does not.

---

## 4. Verification log (what this attempt actually ran)

```
bead show domchk-15854355 / bf-4x12ec / bf-3m9m1v          # records + alert payload
bead list --json --limit 5000                               # split-sibling enumeration
sha256sum -c docs/crashes/bf-4x12ec/MANIFEST.sha256         # 6/6 OK
awk -F'\t' attempt-index.tsv (exit-code column counts)      # 44 x -1 / 8 x 124 / 1 x 0
zcat needle-events-…bf-4x12ec.jsonl.gz | wc -l              # 1,146
grep -c 4x12ec .beads/logs/crash-monitor.log                # 0 (no coverage; logs start 2026-09-01/02)
sed -n … src/dispatch/mod.rs src/outcome/mod.rs  (~/NEEDLE @ 01ecf05, clean)  # pins hold
git show HEAD:<path> | grep -n <anchor>                     # every line citation, HEAD-accurate
git rev-parse HEAD:<path> vs git hash-object <path>         # caught the dirty worktree copy
git log --all --grep domchk-15854355                        # 0 prior attempts (first run of this bead)
```

One method note worth keeping: `git diff --quiet HEAD -- <path>` reported the
canonical report clean while its worktree blob differed from HEAD (§3.2). The
blob-id comparison above is what exposed it. On this shared worktree, verify
HEAD-ness by blob id and take line numbers from `git show HEAD:` — not from a
quiet-diff exit code.

Nothing in `.beads/logs/`, no trace slot, and no kernel record exists for this
bead — the three absence findings in §2.3 are themselves the evidence answer
for any later step that plans to look there.

---

## 5. Dated addendum 2026-09-08 — §3's doc-state snapshot has moved on

This record was committed at `76ad268` against HEAD `8ae57ea`. HEAD has since
advanced (`1beeec4`, `b8f7072`, `2e34dc8`, `fea1a62`). Everything below was
re-verified live 2026-09-08T00:47Z against HEAD `fea1a62` (== `origin/main`,
divergence 0/0), with line numbers from `git show HEAD:<path> | grep -n`:

- **The canonical report is now v1.8, 639 lines at HEAD** (§3.1 said "v1.6,
  592 lines"). `1beeec4` (bead `domchk-791bfb2e`) committed the §3.2
  "uncommitted v1.7" edits (+31/−4 — the metric-provenance block resolving
  **C6**, plus the Addendum 4 attribution correction); `b8f7072` (bead
  `domchk-e48b5e1b`) added v1.8's explicit "no domain-check code defect /
  environmental-only" finding.
- **§3.2's worktree-divergence hazard is closed for that file**: the worktree
  copy is byte-identical to HEAD (blob `38935f97` both). Its §3.2 headline
  ("partially executed, **uncommitted**") is therefore historical.
- **C1 / C2 / C4 are still uncorrected in the body** — re-grepped at v1.8:
  Summary L4 still reads "completed on the 53rd attempt", Resolution step 2
  L64 still reads "completed on retry", and L45–46 still reads
  "Signal -1 = **SIGKILL (Signal 9)**". Addendum 4 (HEAD L481–482) remains the
  only correction of C1, and Addendum 2 the only correction of C4. Cite the
  addenda, not the body, on all three — unchanged guidance, now confirmed
  against v1.8.
- **Two more bf-4x12ec docs landed after this record's snapshot**, both
  2026-09-07: `crash-investigations/bf-4x12ec-alert-inventory.md` (`fea1a62`,
  bead `domchk-f05d6f91` — alert-layer census: 44 wave-1 alerts, 102-bead core
  inventory, independently corroborating §2.2's "44 alert beads" figure) and
  `crash-investigations/evidence/bf-4x12ec/operation-summary.md` (`2e34dc8`,
  bead `domchk-dfce2360` — names the killed operation as
  `git gc --aggressive --prune=now`, issued 10:24:08.660Z as the final
  `Bash` `tool_use`, kill 52.9 s later at 10:25:01.512Z, matching §2.4's
  transcript finding).

§1 and §2 — the signal semantics, the 53-attempt census, the timestamps and
the record locations — are unaffected by any of the above and stand as
written.
