# RCA — "kubeconfig not found" bead-close verification failures (domchk-0a31de73)

**Date:** 2026-09-05
**Bead:** domchk-0a31de73
**Target symptom:** bead-close verification for bf-173o7e reported
`kubeconfig not found: /home/coding/.kube/iad-ci.kubeconfig` although that
path exists today.

## Verdict

**No script bug, no wrong path, no permission problem.** The kubeconfig
genuinely did not exist when every failure occurred. All 64 real executions of
the check happened on **2026-08-17 between 15:26:37Z and 20:48:04Z**; the file
currently at `/home/coding/.kube/iad-ci.kubeconfig` was **created
2026-08-25 08:57:26 EDT** (`stat` birth time == modify time, unmodified since).
The bead's premise — "exists but reported missing, so the check must be broken"
— projects the file's *present* existence onto a *historical* failure.

Answering the bead's three-way question directly:

| Hypothesis | Verdict | Evidence |
|---|---|---|
| Wrong path | Rejected | 261 mentions across all traces contain exactly **one** path variant, and it is the documented one (`docs/notes/release-workflow-status-2026-08-10.md:103` uses the same path). No trace ever shows `$HOME/.kube/config`, a relative path, or any other variant. |
| Permission issue | Rejected | Absence is proven outright (below), so permission was never reached; the file is and was owned `coding:users`. |
| Script bug | Rejected | The check reported the truth. The path it printed is the path it tested, and that path was empty on Aug 17. |

Classification: **environment / stale-premise issue** — the correct category
is "credential file temporarily absent," not a code defect.

## The mechanism (now retired)

`bead close` was shimmed by `~/.local/bin/bead-wrapper.sh` (retained as
`~/.local/bin/bead-wrapper.sh.bak-20260825`), which intercepted `close` and
exec'd `/home/coding/pdftract/.cli/bead-close-with-verify.sh` — a verify gate
built by pdftract bead bf-17ycgu, whose notes record "Error handling tested
(**kubeconfig validation**, input validation)". That gate validated the iad-ci
kubeconfig because it submits `rust-verify` workflows to the iad-ci cluster
before allowing a close.

Two facts about that mechanism:

1. **Hardcoded wrong-workspace paths** — the wrapper pointed at
   `/home/coding/pdftract/...` and the failure banner reads
   `Repo: /home/coding/pdftract` even for beads in other workspaces. This is a
   real defect of the mechanism, already identified and documented by
   domchk-60407475 (commit `73176de`, the current HEAD).
2. **Retired 2026-08-25** — only the `.bak-20260825` file remains
   (mtime 07:23 EDT). The kubeconfig was recreated 94 minutes later
   (08:57 EDT), consistent with a single remediation session: kill the broken
   gate, then refresh credentials. The current close gate in this repo,
   `scripts/verify-work-completion.sh`, contains **zero** references to
   kube/kubectl/cluster (grep verified 2026-09-05), so this failure mode
   cannot recur through any live path.

## Primary evidence

| # | Fact | Source |
|---|---|---|
| 1 | Failure window **15:26:37.962Z → 20:48:04.554Z, 2026-08-17**, across **34 distinct beads** (64 real `tool_result` executions; 97 transcript lines total, the remainder being later agents quoting the historical text) | `.beads/traces/bf-353z15/stdout.txt:6307` (first) … `.beads/traces/bf-gwibon/stdout.txt:4343` (last) |
| 2 | Independent proof of absence: `ls` on the path returned `ls: cannot access '/home/coding/.kube/iad-ci.kubeconfig': No such file or directory` at **2026-08-17T19:35:25.131Z** — mid-window | `.beads/traces/bf-50ehit/stdout.txt:7165` |
| 3 | Current file **born 2026-08-25 08:57:26.315 EDT**, birth == modify (never rewritten) | `stat /home/coding/.kube/iad-ci.kubeconfig` |
| 4 | File existed Aug 7–10 with **expired credentials** (server 401, "asks the client to provide credentials"; OIDC expiry ~3 days per `~/CLAUDE.md`) — so it was present, then removed, before Aug 17 | pdftract bead store (multiple beads, e.g. the sealed-secret verification beads) + `docs/notes/release-workflow-status-2026-08-10.md` |
| 5 | Verify gate went live **2026-08-17T15:09:36Z** (bf-17ycgu closed); first observed failure **17 minutes later** | `bead show bf-17ycgu` (pdftract workspace) + evidence #1 |

## Timeline

| When (UTC unless noted) | Event |
|---|---|
| 2026-08-07 → 08-10 | `iad-ci.kubeconfig` present, token expired — kubectl reaches the server and gets 401 (evidence #4) |
| between 08-10 and 08-17 | kubeconfig removed from `/home/coding/.kube/` (no surviving record of the deletion; the Aug-25 inode birth proves the path was not continuously occupied) |
| 2026-08-17 15:09:36Z | Verify gate (bf-17ycgu) goes live |
| 2026-08-17 15:26:37Z | First `kubeconfig not found` close failure (bf-353z15) |
| 2026-08-17 17:06:02Z | bf-173o7e's own close attempt fails the same way (per domchk-60407475's corrected timeline at `73176de`); bead closed 17:12:09Z by a later attempt |
| 2026-08-17 19:35:25Z | `ls` proves the path absent (bf-50ehit) |
| 2026-08-17 20:48:04Z | Last failure in the window (bf-gwibon) |
| 2026-08-25 07:23 EDT | Wrapper retired (`~/.local/bin/bead-wrapper.sh.bak-20260825`) |
| 2026-08-25 08:57:26 EDT | kubeconfig recreated (birth time), unmodified since |
| 2026-09-05 | Zero live executions since Aug 25; every post-Aug-25 mention of the error in traces is an agent *quoting* Aug-17 evidence, not re-running the check |

## Relation to prior work

This doc does not duplicate domchk-60407475 (commit `73176de`), which corrected
the bf-173o7e RCA and attributed the close loop to "wrong-cwd repo resolution +
a missing kubeconfig." What this investigation adds:

- **Filesystem proof** of the missing-kubeconfig half: birth time 2026-08-25,
  plus an independent in-trace `ls` from inside the failure window.
- **The exact failure window** (34 beads, 5h22m on Aug 17) and the fact that
  the gate went live 17 minutes before the first failure.
- **The resolution of the bead's own paradox**: "exists" is an Aug-25-onward
  state; "reported missing" was an Aug-17 true statement. Both observations are
  correct; they describe different days.
- **Non-recurrence**: the check no longer exists in any live path.

## Action taken

None required — there is nothing to fix. The defective mechanism was retired
on 2026-08-25; the replacement gate has no cluster dependency. The only
durable deliverable is this record, so future agents hitting the string
`kubeconfig not found` in old traces classify it as a closed environmental
event (2026-08-17, iad-ci kubeconfig absent) rather than re-investigating it as
a live script bug.
