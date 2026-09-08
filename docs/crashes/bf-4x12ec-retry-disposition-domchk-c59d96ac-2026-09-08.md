# Retry disposition: bf-4x12ec ("Re-run failed bead bf-4x12ec" = domchk-c59d96ac)

**Disposition bead:** domchk-c59d96ac (2026-09-08) — the verification-and-retry leg of the
bf-4x12ec wave-2 chain
**Task:** "Re-run failed bead bf-4x12ec" — re-queue, monitor, verify the fix prevents the
crash, document the outcome
**Target bead:** bf-4x12ec — "Execute aggressive git garbage collection to eliminate OOM risk"
**Alert parent:** bf-191ch8 — "ALERT: Agent crash on bead bf-4x12ec" (2026-08-14T10:28:37Z)

---

## Disposition: **RETRY NOT REQUIRED — target already closed with the deliverable verified; fix verified live this dispatch; close as done**

The re-run this task asks for was never needed: **bf-4x12ec is Closed (rev 4,
2026-08-17T14:50:41Z)** and its deliverable — the aggressive gc — is verified in the
store's own close notes (18 GB → 753 MB, loose objects 4,627 → 141, fsck clean, git
operations OOM-free) and later re-verified by report Addendum 7 (`domchk-fe10456e`), which
attributes the actual completion to the auto-split child **bf-173o7e (Closed)**, not to any
of the 53 storm attempts. Re-dispatching a closed bead whose work is done would re-run an
already-satisfied operation against an already-packed repository and mint a fresh wave of
per-kill alert beads — the exact alert-layer failure mode the 2026-08-26 regeneration wave
demonstrated. The standing verdict doc is explicit on both halves:
[`bf-4x12ec-classification-retry-verdict-domchk-dba1e0bb-2026-09-08.md`](../crash-investigations/bf-4x12ec-classification-retry-verdict-domchk-dba1e0bb-2026-09-08.md)
§5 — safety **GO**, necessity **NO-GO**, and §5.3 condition 4, "Never re-open bf-4x12ec."
This bead's immediate predecessor in the same chain closed 30 s before this dispatch and
left the same instruction in its close notes (domchk-548dc4e4): "Close it on the same
verify-then-close basis; do NOT re-run any gc."

Accordingly no gc was run. The "verify the fix prevents the crash" half of the task **was**
executed first-hand, by replaying the precise death command under a bounded ceiling.

## Acceptance criteria

| AC | Verdict | Carrier |
|---|---|---|
| Bead bf-4x12ec completes successfully | **Satisfied** — Closed rev 4 (2026-08-17), deliverable verified; completion via split child bf-173o7e, per Addendum 7 | bead store; `docs/crash-investigations/bf-4x12ec-crash-investigation.md` Addendum 7 |
| If still failing, new crash report created | N/A — not failing; no crash occurred during this dispatch | this document |
| Outcome documented in docs/crashes/ | **This document** | — |
| Parent bead bf-191ch8 ready to close | **Closed this dispatch** as stale, immediately after this bead | bead store |

## Fix verification (first-hand, 2026-09-08, HEAD `df36180`)

| Check | Command | Result |
|---|---|---|
| Memory bound effective | `./scripts/setup-git-gc-config.sh --verify` | **exit 0** — effective chain system→global→local; `windowMemory=2g` / `deltaCacheSize=1g` / `threads=1`; worst-case pack memory ≈3072 MiB within the ceiling for the 12 GiB dispatch scope |
| Death-command replay | `./scripts/test-gc-memory-bounds.sh` | **17/17, exit 0** — the exact bf-4x12ec crash command `git gc --aggressive --prune=now` exits 0 under `MemoryMax=768M` (1/16 of the dispatch scope), pack-objects peak RSS **320,396 KB** vs the >12 GiB its unbounded Aug-14 run consumed; bf-1ea4g's bounded push also exits 0 (peak RSS 232,504 KB) |
| Crash precondition gone | `git count-objects -vH`, `du -sh .git` | 332 loose / **2.30 MiB** (was 4,649 / 17.20 GiB), 1 pack / 100.25 MiB, garbage 0, `.git` 107 MB |
| Repo health | `./scripts/check-repo-health.sh` | **exit 0**, 0 unpushed commits (HEAD `df36180` == origin/main) |

The mechanism the fix defeats is documented in the canonical investigation
([`bf-4x12ec-crash-investigation.md`](../crash-investigations/bf-4x12ec-crash-investigation.md)):
kernel memcg OOM SIGKILL of unbounded `git gc --aggressive --prune=now` over 17.20 GiB of
loose objects inside the 12 GiB dispatch scope — INFRASTRUCTURE, repository-bloat regime,
no domain-check code defect (repo-wide finding, unchanged). Both preconditions are now
impossible: the loose mass cannot re-form through `.beads/` (fully gitignored, 10 MB
pre-commit gate), and the pack path is memory-bounded even when invoked bare.

## Alert parent bf-191ch8

bf-191ch8 is wave-1 alert #4 of the 2026-08-14 storm — 93 s after the target bead was
created, one of 44 one-alert-per-kill artifacts minted by pre-0.4.2 needle. Its disposition
is already settled and frozen:
[`docs/archive/crash-investigations/crash-investigation-bf-191ch8.md`](../archive/crash-investigations/crash-investigation-bf-191ch8.md)
(a081775) — "false positive alert for successfully completed git cleanup" — and the alert
inventory ([`bf-4x12ec-alert-inventory.md`](../crash-investigations/bf-4x12ec-alert-inventory.md)
line 426) records it as **verified-FP (doc)**: a genuine kill at creation, stale once the
target closed on 2026-08-17. Its only dependency edge was this bead (the inventory lists
`bf-191ch8 ← domchk-c59d96ac` as an unresolved blocker pair); with this bead closed the
parent is unblocked, and it is closed as stale in the same hour rather than left open to
regenerate another wave — the standing pattern of the bf-mje3pd/bf-3dxljn alert families.

## Chain record

| Bead | Leg | Status |
|---|---|---|
| bf-4x12ec | target — gc deliverable | Closed 2026-08-17 (work by split child bf-173o7e) |
| bf-191ch8 | wave-1 storm alert | Closed 2026-09-08 as stale (this dispatch) |
| domchk-9d85dc9f | wave-2 root-cause leg | Closed |
| domchk-548dc4e4 | wave-2 fix leg | Closed 2026-09-08 (verify-only, no commit needed) |
| domchk-c59d96ac | wave-2 verification/retry leg | Closed 2026-09-08 (this document) |

Nothing in this chain remains open. The one binding condition for any future gc work stands
(verdict doc §5.3): use `./scripts/safe-git-gc.sh`, never bare aggressive gc, and give new
gc work a fresh bead — never re-open bf-4x12ec.

---

*Written 2026-09-08 by domchk-c59d96ac at HEAD `df36180`. Every status re-read live from
the bead-rs store this dispatch; every command above executed this dispatch; every cited
path verified present at HEAD.*
