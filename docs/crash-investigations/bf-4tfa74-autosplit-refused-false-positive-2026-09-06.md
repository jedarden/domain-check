# Alert bf-4tfa74 — auto-split refused (duplicate false positive), 2026-09-06

**Dispatch:** claude-code-glm-5.3-flash-lab-roam-1
**Alert bead:** bf-4tfa74 ("ALERT: Agent crash on bead bf-173o7e", created 2026-08-14T14:17:18Z)
**Disposition:** split refused — no work to decompose; target already closed and root-caused; bead closed.

## Why the split was refused

The auto-split template fired on a **crash alert**, not a task. Its premise — a
parent bead "too big or complex, failed 3×" — does not survive verification:

1. **Target bead is CLOSED.** bf-173o7e ("Execute git gc --aggressive with
   pruning") closed 2026-08-17T17:12:09Z with reason "Git gc completed
   successfully — 17.20GB loose objects packed into 444MB pack file, repository
   valid."
2. **Root cause is settled and committed many times over.** The Aug-14
   `exit -1` was a kernel **memcg OOM SIGKILL** mid-gc (129× exit −1 across the
   132-dispatch Aug-14 storm; 12 GiB dispatch scope). Consolidated in
   `077ab240`-lineage reports and `docs/research/root-cause-analysis-signal-minus-one-crashes.md`.
   The alert's own notes ("max turns, exit 1") carry the superseded Aug-17
   mechanism attribution — corrected by domchk-e1792d54 (4c888b7) and
   domchk-6f3e2601, both appended to bf-173o7e.
3. **The fix is already shipped.** `533cb46 fix: bound the bare git-gc path
   that memcg-OOM-killed bf-173o7e` set `pack.windowMemory=2g`,
   `pack.deltaCacheSize=1g`, `pack.threads=1`; verified via
   `scripts/setup-git-gc-config.sh --verify` and
   `scripts/test-gc-memory-bounds.sh`.
4. **bf-173o7e's latest note (2026-09-06, domchk-6f3e2601) already says:**
   "No action required; do not re-dispatch."
5. **A prior split attempt already left residue.** bf-4tfa74 carried
   `split-child` + `umbrella` labels with **zero children** (verified:
   no bead in the workspace references bf-4tfa74 besides itself) — labels from
   an attempt that never completed mitosis. Creating 3–5 children now would
   manufacture maintenance work where none exists, and add to the 129
   duplicate alerts already generated for bf-173o7e.

## Blocker state

bf-4tfa74's single dependency edge is `domchk-6f3e2601` ("Close original bead
bf-173o7e with success status") — **closed**, so nothing blocks closure.

## Actions taken

- Removed stale `split-child` and `umbrella` labels from bf-4tfa74 (never
  earned; no children exist).
- Appended this disposition to bf-4tfa74 notes.
- Closed bf-4tfa74 as a duplicate false positive of resolved bf-173o7e.
- Left the system-managed alert labels (`alert`, `crash`, `failure-count:3`,
  `signal--1`, `verification-failed`) untouched.

## Loop context

This is the same failure class documented in
`docs/verification-report-bf-28su5u-duplicate-alert-resolved-bf-173o7e.md`
(bf-28su5u) and the bf-1cd5v6 rewrite (c2abb56): NEEDLE re-dispatches alert
beads for an event whose RCA, fix, and closure all landed weeks earlier. The
correct handling is verification → refuse → clean → close, not bead mitosis.

## Verification commands (all run live 2026-09-06)

```bash
bead show bf-173o7e        # Status: Closed, close reason in checkpoint
bead show domchk-6f3e2601  # Status: Closed
bead show bf-4tfa74 --json # labels incl. stale split-child/umbrella; 1 dep
bead list --json --limit 5000 | grep bf-4tfa74   # self-reference only — no children
git log --oneline --grep bf-173o7e | head -20    # RCA + fix already committed
```
