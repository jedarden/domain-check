# Code version and build metadata — bf-4yjq crash storm (2026-08-12)

**Subject bead:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo mirror has
diverged/gone stale" (closed 2026-08-17)
**Dispatch bead:** domchk-55effc6d · **Written:** 2026-09-06
**Storm window:** 2026-08-12T17:50:23Z (first dispatch) → 21:14:56Z (last dispatch); 56
dispatches — 50 crashes exit −1, 1 failure exit 1, 4 timeouts exit 124, 1 exit-0 that still
orphaned the bead (see [raw-logs/README.md](raw-logs/README.md)).

**Source of record:** the 56 preserved per-run agent session transcripts
(`raw-logs/bf-4yjq-crash-sessions-2026-08-12.tar.gz`), re-extracted and grepped for this
report. Every figure below marked `transcripts` is a tool input/output captured *inside* the
crash-era sessions — primary evidence, not a later summary. Figures marked *standing record*
come from the investigation docs and could not be re-derived here (see §6).

---

## 1. Headline

| Item | Value | Source |
|---|---|---|
| Workspace HEAD at first crash | `199b70cdc8efa36689f84df7bc3d5ac95893300c` — "fix: improve timeout error detection and logging", **ahead of origin/main by 305 commits** | transcripts (att-1 `git branch -vv`, 17:51Z) |
| origin/main = github/main at crash time | `63ba02474c9b6bc339388adb3a44542e10755a10` — "fix: remove unused time import and update bootstrap test initialization", 2026-08-09 13:00:56 −0400 | transcripts (att-20 `git log -1 --format="%H %ai %s" origin/main`) |
| Same change in today's repo | `00117cb879ecba7b1a819d80f1e4980ccb5d2881` (identical subject + author/commit timestamps) — **SHA rewritten, tree not byte-verifiable** | local `git show -s 00117cb` |
| Crash-era objects surviving today | **None.** Every SHA in this report is absent from the current object store | `git cat-file -t` on each |
| What actually died | `git` operations under the needle dispatch scope (exit −1) — **no domain-check binary was built or executed during the storm** | needle worker log; transcripts |

The single most important caveat: **the exact crash-time tree cannot be checked out.** The
whole Aug-12 branch state (the 318-commit local backlog) was discarded when the repo was
repaired — the 237 MB `.beads/*.jsonl` blobs lived in those commits, and purging them
rewrote history (`c27899f` "chore: catch up lab work onto origin (squashed)", 2026-08-16).
Everything below is reconstructed from what the transcripts recorded.

## 2. Git state, dispatch by dispatch

Local `main` was **305–318 commits ahead of origin/main** and moved under the storm's feet:
each attempt committed a fresh `.beads/` tracking-file snapshot, then died before pushing, so
the next attempt started from a new HEAD. Observed HEADs (all `transcripts`, full 40-hex
unless marked short-form):

| HEAD (short) | First seen in | Ahead of origin | Note |
|---|---|---|---|
| `199b70c` | att-1 (first crash, killed 17:53:53.875Z, 3 m 30 s) | 305 | full SHA `199b70cdc8efa36689f84df7bc3d5ac95893300c` |
| `dc90487` | att-2 | 306 | "chore: update bead tracking files before git reconciliation" |
| `9196623` | att-6 … att-10 | 307 | full `91966237f2e8d2bb39280d1da7c96676860ce141` |
| `14babd7` | att-11 … att-17 | 308 | full `14babd7f0259c2d16792802a6c631d0a5f32ea51` |
| `3415f38` | att-20 | — | `3415f38754a3390de8455eff495d4f254002c3ea`, 2026-08-12 14:48:45 −0400 — "…and clean up history" |
| `7e09cb4` | att-22 | 310 | full `7e09cb4417a2d375aa9a4318f26e59d3c65c7297` |
| `fa3499e` | att-24 … att-25 | — | full `fa3499eb7b74066952e7f2c699b84265f12e6924` |
| `ba03e8f` | att-30 … att-31 | 314 | full `ba03e8fa0f334b2da42f6f08869f93e67d3a58c3` |
| `d5bf038` | att-33 | — | full `d5bf0387a52c5bb5217566880cb7a7ced2e39f00` |
| `5afeeb8` | att-35 | — | full `5afeeb8bb2777e239148c482a7c57e8aaceba728` |
| `a17b791` | att-36 … att-37 | 318 | full `a17b79165ce651ba7210a48326df9553246e3b2b` |
| `19cc74f` / `afc68c7` / `0abadf7` / `28ababd` | att-40 … att-50 | — | full SHAs in transcripts |

All three refs converged at `63ba024` as merge-base: `git rev-parse origin/main github/main`
returned it identically in every attempt that checked, and `git merge-base origin/main
github/main` = `git merge-base HEAD origin/main` = `63ba024…` throughout — the divergence
the bead was tasked to fix was *ahead-of*, not *forked-from*: local main was a strict
superset of both remotes.

## 3. What the unpushed backlog contained (att-37 measurement, 317–318 commits)

```
git log --pretty=format:"%s" origin/main..HEAD | sort | uniq -c | sort -rn | head
     35  chore: update bead tracking files before test tag
     10  chore: update bead tracking files
      7  chore: update bead tracking state before git reconciliation
      3  docs: add goreleaser release pipeline verification report
      3  chore: update bead tracking files before git reconciliation
      2  fix: improve timeout error detection and logging
      ...
git log --oneline origin/main..HEAD -- .beads/ | wc -l        → 161
git log --oneline origin/main..HEAD -- "**/*.go" "**/go.mod"…  → only 199b70c, 70b8aab (+docs)
```

- **161 of ~317 commits touched `.beads/`** — bead-forge tracking snapshots, the substance
  that made the repo bloat (*standing record:* 17+ identical 237 MB `issues.jsonl`
  snapshots, ~18 GB repo). Notably **`git log … -- ":!.beads" -- ":!.git"` returned empty**
  — once `.beads` churn is excluded, the tail of the backlog is entirely docs/verify/VERSION
  noise.
- **The only unpushed Go code delta over origin was `199b70c` + `70b8aab`** (same title,
  two commits: "fix: improve timeout error detection and logging"), plus `610388f` "verify:
  module path already correct". These are candidates for "recent changes" only in the sense
  of *what was in the tree* — nothing ties them to the kill mechanism, which was repo size.
- The rest: package extractions now on origin (`internal/httpclient`, `internal/whois`,
  `internal/rdap`, `internal/cache`, bootstrap), goreleaser verification docs, iad-ci
  workflow-attempt docs, and `VERSION` bumps (`54cec3d` 4.95.0-test → `2df777d` 5.8.0-test).

## 4. Build metadata

**No domain-check binary is part of this crash.** The dying processes were `git` (index/
pack-objects work on an 18 GB object store) inside the needle dispatch scope. There is no
build timestamp to capture for the crashed code, and no artifact was produced. What can be
captured is the *source* metadata:

| Item | At crash time (2026-08-12) | On today's origin tip (`00117cb`) |
|---|---|---|
| Module path | `github.com/jedarden/domain-check` (transcripts, att-2 era + `610388f`) | same |
| `VERSION` file | **`5.8.0-test`** — unpushed local bump (`git diff 63ba024 HEAD -- VERSION`: `0.1.7` → `5.8.0-test`, transcripts) | `0.1.7` |
| `go` directive | not separately recorded; backlog carried no `go.mod` bump after the origin tip → **`go 1.26.1`** | `go 1.26.1` |
| Deps | as at `00117cb` (whois v1.15.7, whois-parser v1.24.21, ff/v4 v4.0.0-beta.1, prometheus/client_golang v1.23.2, x/net v0.52.0, x/sync v0.20.0, x/time v0.15.0) | same (bbolt + x/sys v0.45.0 arrived later) |
| Go toolchain binary | **not captured** — no attempt ran `go version` | — |

## 5. Runtime / environment configuration

**Agent runtime** (`transcripts`, every record carries these fields):
- Claude Code CLI **2.1.227** (4 707 transcript records), model **`glm-4.7`**,
  branch `main`, template `pluck` / `pluck-default`, prompt 71 289 bytes
  (`sha256:ae84d6617aa9ac00a1ac32058a952c4adc8a93b04d4970d872f19d51af3d1139`)
- needle worker `claude-code-glm-4.7-lab-domain-check`, session `8446529e`,
  workspace `/home/coding/domain-check` (worker log line 1)

**needle version:** the crash-era binary was **not preserved**, so only a bound is possible
from `~/.needle/bin/` backups (memory-note practice): upgrades on 2026-08-11 10:02 left
`needle-stable.pre-0.3.1.bak` (**0.2.19**) and `needle-stable.pre-assignee-fix.bak`
(**0.2.16**); the next preserved snapshot, `needle-stable.pre-0.4.2-20260819`, is
**0.4.0**. So the Aug-12 binary was **≥0.2.19-era, ≤0.4.0**, most plausibly the 0.3.1
installed 2026-08-11. Neither the worker log nor the event log prints a version string.
(Current needle is 0.6.0, 2026-09-01.)

**Git remotes as they stood inside att-1** (the bead's premise, already half-fixed):

```
github  https://github.com/jedarden/domain-check.git   (fetch/push)
origin  https://git.ardenone.com/jedarden/domain-check.git   (fetch/push)
* main 199b70c [origin/main: ahead 305]
```

`origin` already pointed at Forgejo by the first crash attempt; the divergence was the
305-commit unpushed backlog, not the remote URL.

**Kill mechanism, with its evidence limit:** exit −1 is the sentinel for a signal death
needle could not classify (worker log: `exit_code=-1 outcome=Crash(-1)`). For the
**Aug-14 and Aug-16** variants of this crash the kernel records survive and prove memcg-OOM
SIGKILL at the dispatch scope's documented 12 GiB `MemoryMax`; for **Aug-12 no kernel record
exists** (system journald begins 2026-08-15 19:46 EDT), so the same mechanism is *inferred*
here from exit −1 + the ~18 GB object store (*standing record*) + the kernel-proven
analogues two and four days later. Not one of the 56 transcripts ran `du`, `git
count-objects`, or `free` — the bloat figures rest on the investigation docs, never on these
sessions.

## 6. Recent changes that may have contributed

1. **The `.beads/` snapshot commit storm — the contributor, and a feedback loop.** Each of
   the 50 crashed attempts committed another bead-tracking snapshot (HEAD advanced 305 →
   318 *during the storm itself*), then died before pushing. The crash therefore grew the
   very bloat that caused it. Nothing at the time opposed this: `.beads/` was **not**
   gitignored, the 10 MB pre-commit gate **did not exist yet**, and no `pack.windowMemory`
   bound was in place — all three were added afterwards as direct results of this storm
   (see repo CLAUDE.md, "Repository Bloat Prevention").
2. **Last real code change predates the crash by 3 days** (origin tip `63ba024`/`00117cb`,
   2026-08-09) — no code change landed in the run-up; the crash is not downstream of any
   recent commit. Consistent with the standing finding that domain-check code is not the
   crash source.
3. **Unpushed `199b70c`/`70b8aab` timeout-error-detection fix** — present in the working
   tree, never pushed, lost in the rewrite. No evidence it influenced the kill; recorded
   because it is the one piece of *code* that existed at crash time and exists nowhere
   today.

## 7. Not reconstructable (and why)

| Lost | Reason |
|---|---|
| Exact crash-time tree / any Aug-12 commit object | history rewritten 2026-08-16 (`c27899f` squash) to purge the `.beads` blobs; reflog begins 2026-09-02 |
| Whether `63ba024`'s tree is byte-identical to `00117cb`'s | its tree hash was never printed in the transcripts; identity rests on subject + identical author/commit timestamps |
| Go toolchain binary version | no attempt ran `go version` |
| Exact needle patch version on Aug-12 | binary overwritten; only `.pre-*` backups survive |
| Kernel OOM records for Aug-12 | system journald starts 2026-08-15 19:46 EDT |

## 8. Re-derivation

```bash
cd /tmp && tar xzf docs/crash/bf-4yjq/raw-logs/bf-4yjq-crash-sessions-2026-08-12.tar.gz
# HEAD at first crash:
grep -oh '199b70c[0-9a-f]*' /tmp/sessions/*.jsonl | sort -u        # → 199b70cdc8efa36689f84df7bc3d5ac95893300c
# origin tip at crash time + its date/subject:
grep -oh '63ba02474c9b6bc339388adb3a44542e10755a10 2026[^"]*' /tmp/sessions/*.jsonl | head -1
# CLI/model (per-record fields):
python3 -c "…" # collect d['version'] / message.model across sessions → 2.1.227 / glm-4.7
# backlog composition (att-37): git log --oneline origin/main..HEAD -- .beads/ | wc -l → 161
```

Cross-references: [raw-logs/README.md](raw-logs/README.md) (dispatch/kill timeline, to the
millisecond) · [../crashes/bf-4yjq-crash-report.md](../../crashes/bf-4yjq-crash-report.md) ·
[../crashes/bf-4yjq-cleanup-verification.md](../../crashes/bf-4yjq-cleanup-verification.md)
(repo repaired; 94 MB) · `docs/crashes/bf-198ne-crash-report.md` (push-side variant of the
same memcg mechanism, kernel-proven).
