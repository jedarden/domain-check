# Infrastructure status — bf-4yjq crash storm (2026-08-12)

**Subject bead:** bf-4yjq — "Git origin remote points to GitHub directly; Forgejo mirror has
diverged/gone stale" (P2, closed 2026-08-17)
**Dispatch bead:** domchk-7aa2f7bd · **Written:** 2026-09-06 · **Worker:**
`claude-code-glm-5.3-flash-lab-roam-6`
**Storm window:** 2026-08-12T17:50:23Z (first dispatch) → 21:14:56Z (last dispatch); 50
crash deaths 17:53:53.875Z → 20:30:38.310Z, then 4 timeouts and one exit-0 (timeline:
[raw-logs/README.md](raw-logs/README.md)). Host-local equivalent: 13:50–17:14 EDT.

Every figure in this document was re-derived live on 2026-09-06 from the primary sources
listed in §2 — chiefly the six needle structured event logs for 2026-08-12 (all workers,
all beads, not just bf-4yjq), the needle plaintext worker log (`…log.2`, covering
2026-08-11 → 2026-08-15), and the 56 crash-session transcripts. Figures inherited from the
investigation docs rather than re-derived here are marked *standing record*.

Sibling documents: [error-analysis.md](error-analysis.md) (error-level evidence) ·
[version-info.md](version-info.md) (code/build metadata) ·
[raw-logs/](raw-logs/README.md) (primary logs).

---

## 1. Answer up front

| Acceptance criterion | Finding |
|---|---|
| **Memory / CPU / disk at crash time** | **CPU: recorded — saturated all day** (§4): every hour of 2026-08-12 contains above-threshold load samples; within the storm window load₁ was 7.6–15.3 (median 10.06) against a 12-core host — the *lowest*-saturation stretch of the day, so CPU pressure did not cause the storm. **Memory: no direct reading exists anywhere** (§3) — no telemetry source on this host recorded memory in August 2026 and no crash attempt ever ran `free`; what exists is the 12 GiB dispatch-scope bound plus the kernel-proven Aug-16 analogue. **Disk: no reading was ever taken** (§5) — no `df`/`du` in any transcript, no monitoring yet — but zero ENOSPC signals excludes disk exhaustion as the mechanism. |
| **OOM killer events in system logs** | **None survive for Aug 12, and none can** (§6): the system journal begins 2026-08-15 19:46:33 EDT and the storm ran under a boot that ended at the Aug-14 16:39 reboot, so the Aug-12 kernel records were lost with the volatile journal. The journal *does* capture memcg OOM kills — 916 lines since it began, first one being `git` at anon-RSS 11.7 GiB on Aug 16 — which is exactly the record Aug 12 would have left. |
| **Inference gateway availability** | **Up, at crash time and now** (§7). At crash time: every attempt completed many LLM turns right up to its death, and there are zero message-shaped gateway/unavailable/5xx hits in 14.76 MB of transcripts plus all Aug-12 needle telemetry. Live 2026-09-06: `/health` → `ok`. |
| **SIGHUP / other system-level events** | **Zero SIGHUP evidence anywhere** (§8) — transcripts, structured events, plaintext worker log. No reboot or host event inside the storm window (the boot running the storm began Aug 12 01:15 EDT and survived until Aug 14 16:39). The system-level event that *was* occurring is day-long CPU saturation with needle throttling worker launches — outside the storm window. |
| **Infrastructure anomalies** | Day-long fleet CPU saturation with launch deferrals (§4); a **rolling sequence of three single-bead crash loops** over the same bloated repo — bf-31mno (348 deaths) → bf-4yjq (50) → bf-1s6c3 (49) — with **zero hour-overlap** (§9); a self-amplifying loop in which each crashed attempt committed another ~237 MB snapshot, growing the store it died on (§9). Host-level OOM: none (§6). |

---

## 2. What telemetry exists for 2026-08-12

This is the governing constraint for every criterion: the host had **no memory or disk
telemetry at all** in August 2026, and its kernel-level logging did not yet persist.

| Source | Covers Aug 12? | What it gives | Memory data? |
|---|---|---|---|
| Needle structured event logs, 6 workers (`~/.needle/logs/claude-code-glm-4.7-lab-*-2026-08-12.jsonl`, ≈24 MB) | ✅ full day | outcomes/exit codes, `fleet.cpu_saturated` with `load_average`, launch deferrals, dispatch cadence | **No** — 0 of 100 523 events carries a memory field (checked programmatically) |
| Needle plaintext worker log `needle-claude-code-glm-4_7-lab-domain-check.log.2` (2026-08-11 → 08-15) | ✅ full day | per-dispatch `load_1min` WARN readings, ERROR/WARN lines | No |
| 56 crash-session transcripts (`raw-logs/*.tar.gz`, 14.76 MB) | ✅ the storm itself | tool-level agent activity, death truncation points | No — no attempt ran `free`, `du`, `df`, or `git count-objects` (also noted in [version-info.md](version-info.md) §5) |
| System journal (`journalctl`) | ❌ first entry 2026-08-15 19:46:33 EDT | — | — |
| Kernel ring buffer (`dmesg`) | ❌ current boot only (begins Aug 15 09:48 per wtmp) | — | — |
| `coredumpctl` | ❌ earliest entry 2026-08-17 16:01:44 EDT | — | — |
| `sar`/sysstat, atop | not installed | — | — |
| `lab-health-collector.service` (host metrics → lab-health dashboard) | ❌ first invocation 2026-08-15 23:53 EDT, current boot | collects load/CPU/mem every ~30 s — **but only from Aug 15 onward** | Yes, but starts 3 days after the storm |
| `.beads/logs/*` monitoring logs | ❌ begin 2026-09-01 | — | — |
| wtmp (`last reboot`) | ✅ | boot history (§6) | — |

## 3. Memory at crash time — a bound and an analogue, not a reading

**No direct measurement of memory exists for 2026-08-12.** Nothing on the host sampled
memory that day, and the crash-era sessions never ran a memory command. What can be
established:

- **The container that died was bounded at 12 GiB.** Every agent runs inside a
  `systemd-run --user --scope` unit under `needle.slice`. Live-verified 2026-09-06:
  dispatch scope `MemoryMax=12884901888` (**exactly 12 GiB**), `MemoryHigh=infinity`;
  parent `needle.slice` `MemoryHigh=24 GiB`, `MemoryMax=32 GiB`; host 62 GiB. Caveat: the
  crash-era needle binary was 0.3.1-era (per [version-info.md](version-info.md) §5;
  current is 0.6.0), so the Aug-12 value of the bound is *documented*, not kernel-attested
  — no scope record from that date survives.
- **The kernel-proven analogue two days later shows what the bound did.** First journalled
  OOM kill, 2026-08-16 00:27:35 EDT (bf-198ne, the push-side variant of this same crash):

  ```
  kernel: git invoked oom-killer: gfp_mask=0xcc0(GFP_KERNEL), order=0, oom_score_adj=200
  kernel: oom-kill:constraint=CONSTRAINT_MEMCG,...,oom_memcg=/user.slice/user-1001...
  kernel: Memory cgroup out of memory: Killed process 3322486 (git)
          total-vm:13847248kB, anon-rss:12301364kB, ...
  ```

  `git` reached **anon-RSS ≈ 11.7 GiB** and was SIGKILLed by its memcg — the dispatch
  scope's 12 GiB. `git push`'s pack-objects was unbounded then: `pack.windowMemory` was
  not installed until 2026-09-02. This is the mechanism bf-4yjq's 50 deaths are inferred
  to share (per-run corroboration in [error-analysis.md](error-analysis.md) §5; the
  Aug-12 kernel step itself is unrecoverable, §6 below).
- **Scope of the pressure: the cgroup, not the host.** Every OOM-kill constraint line in
  the entire journal (458 records since it begins) is `CONSTRAINT_MEMCG`; there is not one
  host-level OOM. The 62 GiB host was never the exhausted resource — the 12 GiB dispatch
  scope was. Nothing from August suggests Aug 12 differed.
- **The store being packed was the pressure source** (*standing record*, verified by the
  later cleanup): ≈18 GB `.git` with 17.2 GiB of loose objects, of which 17+ identical
  237 MB `.beads/*.jsonl` snapshots.

**Verdict:** consistent with memcg exhaustion inside the 12 GiB dispatch scope during
`git push` pack-objects over the bloated store; **not** consistent with host-wide memory
exhaustion. Confidence in the mechanism: MEDIUM-HIGH (as rated in the canonical report) —
inferred, because the kernel record for this date does not exist.

## 4. CPU — saturated all day; the storm window was the calmest stretch

Needle records `fleet.cpu_saturated` (load₁ > 0.8 × core_count, needle seeing 9 cores on
the 12-core host) and emits a `load_1min` WARN per dispatch above the same threshold.

- **3,065 `fleet.cpu_saturated` events across all 6 workers, 05:37 → 23:54 UTC** — every
  hour of the day contains above-threshold samples. By construction these are a *floor*,
  not a distribution (they exist only above the threshold), but the floor holds all day.
- All-day dispatch-time `load_1min` series (plaintext log, n=1 254): min 7.23, **median
  11.84, max 55.98**, mean 12.85.
- **Inside the storm window (17:45–21:15 UTC):** dispatch-time series n=51 — min 7.61,
  **median 10.06, max 15.33**, mean 10.31; structured-log samples n=241 — mean 9.8,
  max 15.7. Normalized against needle's 9-core view that is ≈1.1× — overloaded, but
  **the mildest saturation of the day** (the midday peak reached load₁ ≈ 84.5).
- Needle actively throttled the fleet: 82 `worker.launch.deferred` events with reason
  `system saturated: CPU load saturated …` on the domain-check worker alone that day —
  **all outside the storm window** (0 in-window).

**Verdict:** CPU pressure was a real, day-long infrastructure condition on this host, but
it did **not** cause the storm: the storm ran through the day's least-saturated hours, and
the death mechanism (§3) is a per-cgroup memory bound, not a load effect. Also note
`core_count: 9` in all 3,065 events — needle's view of a 12-core host (cgroup-visible
CPU), the denominator for every normalized figure above.

## 5. Disk — no reading was ever taken; exhaustion excluded as the mechanism

- No source records disk usage for Aug 12: no `df`/`du` output exists in any of the 56
  transcripts, the repo-health monitoring log begins 2026-09-01, and no system disk
  monitor was running.
- **Zero** ENOSPC / `No space left` signals in 14.76 MB of transcripts and in all Aug-12
  needle telemetry (message-shaped search, §11) — every git command that ran before the
  deaths returned normal output.
- Bounding only (*standing record*): the repository was ≈18 GB on a 444 GB single root
  disk — under 5 % of capacity. Plausible, and consistent with the total absence of
  space errors.

**Verdict:** disk exhaustion is excluded as a crash mechanism by signal absence; the
actual disk headroom at crash time is unknowable.

## 6. OOM killer / kernel records — why the Aug-12 record does not exist

- **The journal begins 2026-08-15 19:46:33 EDT** (`journalctl` first entry; single boot id
  `52309698` in the journal). wtmp shows the *current* boot began Aug 15 09:48 — journal
  persistence was enabled ~10 h into that boot, and every earlier boot's journal was
  volatile and is gone.
- **The storm ran on the boot begun 2026-08-12 01:15 EDT.** wtmp boot sequence around the
  storm: Aug 11 10:43 → Aug 12 01:15 → **Aug 14 16:39** → Aug 14 21:41 → Aug 15 09:48
  (current). **No reboot occurred inside the storm window**; the Aug-12 boot's kernel
  records were destroyed by the Aug-14 16:39 reboot. The reboot cluster of Aug 14–15 —
  two reboots on the day of the bf-173o7e/bf-4x12ec gc storm — is itself an anomaly
  worth noting, but it postdates this bead's storm by two days.
- `coredumpctl` earliest entry 2026-08-17 16:01:44 EDT (pdftract, COREFILE missing) —
  nothing from Aug 12. `dmesg` covers only the current boot.
- **What the journal records when it does cover an event** — i.e. what Aug 12 lost:

  | Day (since journal start) | Kernel OOM lines | Killed processes |
  |---|---|---|
  | Aug 16 | 828 | 257× `git` (the bf-198ne/bf-173o7e memcg kills) + several `node (vitest)` at 4.7–8.6 GiB anon |
  | Sep 02 | 30 | `bash`/`python3` — synthetic test/gc scopes |
  | Sep 06 | 58 | `bash`/`git` — synthetic test/gc scope replays |

  All 458 `oom-kill:constraint=` records are `CONSTRAINT_MEMCG`; **zero host-level OOM
  has ever been recorded on this host.**

**Verdict:** the acceptance-criterion search was performed and resolves to a verified
absence: the Aug-12 kernel OOM record — the one artifact that would name the signal and
the cgroup limit — was never written to any store that survives. The journalled Aug-16
`git` kill at the same 12 GiB bound (§3) is the standing kernel-level proof of the
mechanism inferred for this storm.

## 7. Inference gateway — up at crash time and now

- **At crash time (indirect but strong):** all 56 attempts performed sustained multi-turn
  LLM work (1.1–6.2 min, 7–32 git commands each) and 49 of the 50 crash transcripts
  truncate mid-work, seconds before needle classified the death — the gateway was serving
  the model continuously through 17:50–21:14 UTC. Routing events show
  `gen_ai.system=zai / model=glm-4.7` throughout the day.
- **Message-shaped search for service failure: zero hits.** Across all Aug-12 needle
  events and all transcripts there is no `gateway`, `unavailable`, `refused`,
  `503`/`502`-as-a-message, or timeout-from-network line. (Naive numeric greps find
  hundreds of `503`/`502` — all substrings of sequence numbers and tool-call IDs, plus
  the repo's own doc prose quoting CLAUDE.md's retry recipe; same caveat as
  [error-analysis.md](error-analysis.md) §3.3. The only real `timeout` outcomes are
  needle's exit-124 command timeouts.)
- **Live check 2026-09-06 18:4x EDT:**
  `curl -skf --max-time 5 https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health`
  → `ok`. (`-k` is required — the gateway serves a self-signed cert; a plain `-sf` check
  fails with curl 60 while the gateway is fine.)

**Verdict:** SERVICE_FAILURE is excluded for this storm, on both crash-time and live
evidence — matching [error-analysis.md](error-analysis.md) §6.

## 8. SIGHUP and other system-level events

- **SIGHUP: zero evidence.** Message-shaped searches for `sighup`/`hangup` across the 56
  transcripts, all six structured event logs, and the plaintext worker log return 0 hits.
  A SIGHUP-class death would also contradict the evidence shape (no shell-level text,
  sub-15-s kill-to-classification latency — [error-analysis.md](error-analysis.md) §5).
  The "SIGHUP cascade" entry in the repo's generic crash taxonomy is **excluded** for
  this storm.
- **No reboot, restart, or host event inside the storm window** (§6 boot sequence).
- **System-level events that were happening:** day-long CPU saturation with needle
  throttling worker launches (§4), and the rolling bead-level crash loops of §9. Neither
  occurs *inside* the window beyond bf-4yjq's own loop.

## 9. Anomalies — what the day actually looked like

**The day's crash activity was a rolling handoff between three single-bead loops, not one
concurrent storm.** Exit −1 events on 2026-08-12, all workers (1 102 classified outcomes
total: 457 crash [41 %] / 208 timeout / 158 failure / 279 success):

| Hour (UTC) | bf-31mno | bf-4yjq | bf-1s6c3 | other beads |
|---|---|---|---|---|
| 05:00–16:59 | **348** | 0 | 0 | 4 |
| 17:00–20:59 | 0 | **50** | 0 | 2 (bf-23n ×1 at 17:08, bf-2xygo et al.) |
| 21:00–23:59 | 0 | 0 | **49** | 4 |

- The three loops **never share an hour**: bf-31mno stops the hour before bf-4yjq's first
  death (17:53:53Z); bf-1s6c3 begins the hour after bf-4yjq's last (20:30:38Z). Each
  poison bead monopolizes a worker slot in a ~3-min retry cadence until the loop breaks
  (timeouts / auto-split), then the next one is picked up.
- **Within bf-4yjq's own window (17:50:23–21:14:56Z), its 50 deaths are the only exit −1
  events fleet-wide**, while the fleet remained busy: 236 `agent.dispatched` events across
  the 6 workers in that window. The canonical report's "455-event, 6-bead workspace-wide
  storm" counts the whole day across beads (this re-derivation: 457 events, 9 distinct
  beads — bf-31mno 348, bf-4yjq 50, bf-1s6c3 49, bf-2xygo 4, bf-4tciy 2, bf-28p 1, and
  3 more singletons); the window-level view above is the sharper statement.
- **Self-amplification loop (the key anomaly).** Each crashed attempt committed another
  bead-tracking snapshot before dying: HEAD moved from 305 to 318 commits ahead of origin
  *during* the storm ([version-info.md](version-info.md) §2), i.e. up to ≈13 × 237 MB of
  new loose objects added while the loop was running. Nothing opposed it — `.beads/` was
  not gitignored, the 10 MB pre-commit gate and the `pack.windowMemory` bounds did not
  exist yet. The storm was therefore growing the exact store its deaths were caused by.
- **Current-state contrast (2026-09-06, 18:4x EDT):** 45 GiB memory available of 62;
  58 GB disk free; `.git` 98 MB. The repo-side cause is repaired and holding
  ([bf-4yjq-cleanup-verification.md](../../crashes/bf-4yjq-cleanup-verification.md)).

## 10. Classification input

| Hypothesis | Verdict |
|---|---|
| Memory pressure (memcg) at the dispatch scope | **Consistent — primary hypothesis** (bound + analogue + uniform death point; kernel step unrecoverable for this date) |
| Host-wide memory exhaustion | Excluded (no host-level OOM ever recorded; 62 GiB host) |
| CPU saturation as cause | Excluded as cause (storm ran in the day's calmest stretch; mechanism is a memory bound) — recorded as background condition |
| Disk exhaustion | Excluded (zero ENOSPC; no space errors in any source) |
| Gateway / service failure | Excluded (§7) |
| SIGHUP / graceful shutdown | Excluded (§8; [error-analysis.md](error-analysis.md) §5) |
| Reboot / host event mid-storm | Excluded (§6) |

## 11. Provenance — re-derive everything

```bash
cd ~/.needle/logs

# CPU saturation: count + hourly spread, all workers
python3 - <<'EOF'
import glob, json, collections
sat=collections.Counter(); beads=collections.defaultdict(collections.Counter)
for f in glob.glob('claude-code-glm-4.7-*2026-08-12.jsonl'):
    for line in open(f):
        try: r=json.loads(line)
        except: continue
        et=r['event_type']; d=r.get('data') or {}
        if et=='fleet.cpu_saturated': sat[r['timestamp'][:13]]+=1
        elif et=='outcome.classified' and d.get('exit_code')==-1:
            beads[r['timestamp'][:13]][d.get('bead_id')]+=1
print(sum(sat.values()), dict(sat))
for h in sorted(beads): print(h, dict(beads[h]))
EOF

# dispatch-time load readings (threshold-gated WARNs)
grep -oE '2026-08-12T[0-9:]+.*load_1min=[0-9.]+' \
  needle-claude-code-glm-4_7-lab-domain-check.log.2 | grep -oE 'T[0-9:]+|load_1min=[0-9.]+'

# zero memory fields in needle telemetry; zero SIGHUP/ENOSPC/gateway-as-message
grep -c '"mem' claude-code-glm-4.7-*2026-08-12.jsonl            # → 0 per file
grep -cE '"(sighup|hangup|enospc|no space left|out of memory)"' \
  claude-code-glm-4.7-*2026-08-12.jsonl                         # → 0 per file

# journal coverage + boot history
journalctl --output=short-iso --no-pager | head -1    # → 2026-08-15T19:46:33-04:00
journalctl --list-boots --no-pager                    # → single boot, 2026-08-15
last reboot | head -6                                 # → Aug 12 01:15 … Aug 15 09:48
coredumpctl list --no-pager | head -2                 # → earliest 2026-08-17

# the Aug-16 kernel analogue (git at the 12 GiB memcg bound)
journalctl -k --no-pager -S '2026-08-16 00:27:30' -U '2026-08-16 00:27:40' | grep -E 'oom-kill|Killed process'
journalctl -k --no-pager | grep -oE 'constraint=CONSTRAINT_[A-Z]+' | sort | uniq -c   # → all MEMCG

# dispatch scope memory bound, live
systemctl --user show "$(awk -F0:: '/0::/{print $2}' /proc/self/cgroup)" -p MemoryMax   # → 12884901888

# transcripts: no resource command was ever run
tar -xzf ~/domain-check/docs/crash/bf-4yjq/raw-logs/bf-4yjq-crash-sessions-2026-08-12.tar.gz -C /tmp
grep -licE 'free -|df -h|du -sh|count-objects|Filesystem' /tmp/sessions/*.jsonl | grep -v ':0' | wc -l  # → 0

# gateway, live (self-signed cert → -k is required)
curl -skf --max-time 5 https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health     # → ok
```
