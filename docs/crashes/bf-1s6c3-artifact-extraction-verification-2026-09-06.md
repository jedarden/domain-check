# bf-1s6c3 Crash Artifact Extraction — Independent Verification

**Verifying bead:** domchk-60c12286 (claude-code-glm-5.3-flash, roam-4)
**Verified 2026-09-06, 18:50–19:10 EDT**
**Subject bead:** bf-1s6c3 — "Create merge commit reconciling Forgejo and GitHub histories"
**Extraction verified:** `docs/crashes/bf-1s6c3/`, produced by the concurrent dispatch
**domchk-fcac734a** (worker `claude-code-glm-5.3-flash-lab-roam-5`). This dispatch found that
extraction already on disk and **verified it rather than re-extracting it** — all claims below
were re-run live by the verifying bead; nothing here is cited from the producing worker's notes.

Primary catalog: [`docs/crashes/bf-1s6c3/README.md`](bf-1s6c3/README.md). This file records the
second dispatch's verification plus four items the catalog does not cover (§2, §5, §6, §7).

## 1. Bundle integrity — all checks pass

| Check | Result |
|---|---|
| `sha256sum -c MANIFEST.sha256` (5 files) | **all OK** |
| `needle-events-2026-08-12-bf-1s6c3.jsonl` | 945 lines, **0 unparseable**, matches README count |
| `needle-events-2026-08-13-bf-1s6c3.jsonl` | 513 lines, **0 unparseable**, matches README count |
| Event spans | 21:31:27.663Z Aug-12 → 02:01:27.732Z Aug-13 — matches README §1 timeline |
| `bf-1s6c3-crash-sessions-2026-08-12_13.tar.gz` | `tar -tzf` clean (exit 0, no errors), **76 members** |
| Member modes | **76 × `-rw-------`** — original transcript permissions preserved (0600, matching `~/.claude/projects/`) |
| Member dates | 2026-08-12 17:43–20:45 local = the 21:31Z–02:01Z UTC crash window |

**Source provenance re-verified live** (source logs still on disk; fabric-prune broken since
2026-08-17 so Aug-12 dispatch logs were never deleted):

| Source | Size | SHA-256 |
|---|---|---|
| `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-12.jsonl` | 3,589,712 B | `3a487dc3139a2785f3ec2917f29d00121abbe109a95793163ad0a3c27352b28e` |
| `~/.needle/logs/claude-code-glm-4.7-lab-domain-check-2026-08-13.jsonl` | 3,111,314 B | `f76959bb0542e2b5cee982e5b6610614985c77ad8f20349527d04f6a551b21ce` |

Both match the README §1 table byte-exactly — independent proof the producing worker read these
files and that they have not changed since extraction.

## 2. The "2026-08-12T22:06:23" crash timestamp — mapped to its attempt

The dispatch that requested this extraction names **2026-08-12T22:06:23** as the crash
timestamp. In the raw log that instant is **not a death** — it is the post-crash handling
heartbeat of one mid-storm attempt:

```
22:06:17.301Z  agent.completed   exit_code=-1, duration_ms=119743   <- the real death
22:06:22.303Z  heartbeat.emitted state=HANDLING
22:06:23.396Z  heartbeat.emitted state=HANDLING_RELEASE_DONE       <- the named timestamp
22:06:25.521Z  bead.released
22:06:27.703Z  bead.claim.succeeded  -> next attempt dispatched 22:06:27.714Z
```

So the named timestamp is **6.1 s after** the actual kill — the same alert-timestamp-after-the-
kill offset documented for bf-173o7e (8–120 s) and bf-4x12ec. It is one of the 71 identical
`exit_code=-1` deaths, not a distinct crash event.

## 3. Absent artifact classes — absence re-verified live 2026-09-06

The dispatch asked for journalctl, and memory/CPU state from system logs, at 2026-08-12T22:06Z.
These **do not exist and cannot be collected**; each boundary below was re-checked live:

| Source | Oldest entry | Consequence for Aug-12 |
|---|---|---|
| System journal (`journalctl --list-boots`) | **2026-08-15 19:46:33 EDT** | no kernel memcg records for this crash |
| User journal | 2026-08-17 15:33 EDT | none |
| `coredumpctl list` | 2026-08-17 16:01:44 EDT | no coredump for this crash |
| `.beads/logs/` | 2026-09-01 11:22 (`crash-pattern-alerts.log`) | no monitoring logs from the crash |
| Needle OTLP traces | no trace store exists under `~/.needle/` at all | nothing to be single-slot-overwritten |

This confirms README §3: any document claiming memory/disk/load figures *for the crash moment*
is reconstructed, not measured. The recoverable proxies are the bundle's raw needle events and
session transcripts, plus the repository-size provenance the README cites.

## 4. Git state

- **Current (2026-09-06, at verification):** branch `main`; crash-era SHAs confirmed dead —
  `git cat-file -t 2832106` and `7ad8d15` (the commit attempt 1's session recorded creating)
  both fail with "Not a valid object name"; current history contains **no commits dated
  Aug-12/13** (post-reconciliation history). Forgejo/GitHub divergence is zero per `7906efc`.
- **At crash time (reconstructed, from the bundle's session transcripts):** the repo carried
  ~18 GB `.git` with ~17 GB loose objects; every attempt died at `git push` (71 of 76), which
  is the pack-objects memcg-OOM mechanism proven for bf-4x12ec (gc) and bf-198ne (push).

## 5. Commit hazard for the bundle — `*.jsonl` ignore rule

`.gitignore:70` is a repo-wide `*.jsonl` rule (the bf-4yjq bloat fix). It matches **both**
`needle-events-*.jsonl` files in the bundle, so a plain `git add docs/crashes/bf-1s6c3/`
**silently drops the two primary artifacts** and commits only the tarball, index, manifest and
README. Verified with `git check-ignore -v` (both hit `.gitignore:70:*.jsonl`). Whoever commits
this bundle must `git add -f` those two files — the same trap that silently dropped a prior
extraction bead's deliverables.

## 6. Dispatch acceptance-criteria mapping (domchk-60c12286)

| Criterion | Outcome |
|---|---|
| Locate and copy artifacts | done by domchk-fcac734a into the durable `docs/crashes/bf-1s6c3/` (supersedes a temp dir) |
| Artifacts readable / not corrupted | §1 — checksums, JSONL parse, tar integrity all pass |
| List with timestamps and checksums | bundle `MANIFEST.sha256` + `sessions-index.tsv`; source sizes/hashes re-verified §1 |
| Preserve original permissions | §1 — tarball stores all 76 transcripts at their original `0600` |
| Needle crash logs | bundle event JSONLs, re-verified §1 |
| Agent stdout/stderr | 76 session transcripts in the tarball, indexed per attempt |
| journalctl at 2026-08-12T22:06Z | does not exist (§3); real death was 22:06:17.301Z (§2) |
| Memory/CPU state from system logs | does not exist (§3); documented proxies in README §3 |
| Git state | §4 |
| Summary file with sizes/checksums/timestamps | bundle README + MANIFEST, plus this verification record |

## 7. Duplication note

Two dispatches of this extraction task ran concurrently on 2026-09-06: domchk-fcac734a
(produced the bundle) and domchk-60c12286 (this verification). One bundle is the intended
outcome; this file is the second dispatch's verification record and commit-hazard flag, not a
second extraction.
