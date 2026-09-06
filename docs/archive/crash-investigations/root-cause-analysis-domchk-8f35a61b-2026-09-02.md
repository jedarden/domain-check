# Root Cause Analysis — domchk-8f35a61b (2026-09-02)

**Bead:** `domchk-8f35a61b` — "Analyze crash logs and root cause" (pluck task, created 2026-08-27, no
specific target crash; claimed by worker `claude-code-glm-4.7-lab-domain-check` at
2026-09-02T20:04:53Z)
**Scope:** fleet-wide crash-log analysis for 2026-09-02 (UTC), all 19 needle workers on this box
**Method:** primary evidence only — fleet needle logs (`~/.needle/logs/`), kernel log
(`journalctl -k`), user journal (`journalctl --user`), bead logs (`.beads/logs/`), live cluster state
via the read-only kubectl proxy, and live resource/curl checks. Every claim below is reproducible
from the cited command.

## Verdict

**Nothing killed any agent with a signal today.** There are **zero `exit=-1` events on 2026-09-02**
across the entire fleet; the last signal death anywhere in the logs is **2026-08-26T22:54:48Z**.
What today's "crash" volume actually is:

1. **432 exit-code-1 agent failures** (vs 1025 successes, 1 timeout) that cluster into **synchronized
   fleet-wide waves** — 13–15 *different* workers failing in the *same minute*, ~12 times across the
   day. Simultaneous failure across independent workers and workspaces is a service-level failure
   (model/backend availability), not per-task defects and not resource exhaustion.
2. **A verified live telemetry outage**: both `needle-otel-collector` replicas in apexalgo-iad are in
   CrashLoopBackOff (52 and 72 restarts) on a **config schema mismatch**, so every needle OTLP
   log/trace export has failed with 503 all day. This corrupts observability (traces/logs lost) but
   does not kill agents.
3. **All kernel memcg OOM kills today are synthetic** — test scopes (`mw-oomdbg-*`, `probe-hog-*`,
   `mw-oom-N`) from this repo's own crash-detection test scripts, plus short-lived `safe-git-gc-*`
   scopes where the memory bound fired **as designed**. No agent process was OOM-killed.
4. **One new false-alarm vector**: the health-check command documented in CLAUDE.md fails with
   `curl: (60) SSL certificate problem: self-signed certificate` — the gateway itself is healthy
   (HTTP 200 `ok`). An agent following the runbook verbatim will wrongly conclude "Gateway down".

The standing repo finding — **no defects in domain-check application code** — is unchallenged by
today's evidence: no failure traces to application behavior.

## 1. Exit-code census (2026-09-02, all workers)

```
$ grep -h "handling agent outcome" ~/.needle/logs/needle-claude-code-glm-4_7-lab-*.log \
    | grep -E '^2026-09-02' …
   1025 exit=0 outcome=Success
    432 exit=1 outcome=Failure
      1 exit=124 outcome=Timeout
      0 exit=-1                        # grep -c 'exit=-1' → 0
```

- **`exit=-1` (SIGKILL sentinel): zero today.** Last seen fleet-wide:
  `2026-08-26T22:54:48Z`; before that 2026-08-17 and 2026-08-16. The Aug-14 memcg-OOM storm class
  (bf-173o7e, bf-4x12ec) has not recurred since the `pack.windowMemory=2g` /
  `pack.deltaCacheSize=1g` / `pack.threads=1` bounds were applied (repo-local + global).
- **`exit=1`: 432 events**, ~29.6% failure rate. Per CLAUDE.md classification this is the
  workflow/service-failure class (max-turns or upstream HTTP failure), not the signal class.

## 2. The exit=1 failures are synchronized fleet waves, not task failures

Clustering today's exit=1 events by minute (all workers):

```
15 fails @ 06:06   15 @ 03:21   14 @ 09:09   14 @ 04:39   14 @ 01:22
13 @ 09:48         13 @ 05:09   13 @ 01:06   12 @ 01:57   11 @ 09:57   11 @ 03:09
```

Example wave, 01:06:27–01:06:41Z — **11 distinct workers** (`roam-4`, `roam-8`, `test-fix`,
`roam-1`, `roam-3`, `drawrace`, `roam-6`, `roam-5`, `domain-check`, `roam-10`, `roam-7`) failing
within 14 seconds of each other, on unrelated beads across unrelated workspaces (`coding`,
`domain-check`, `sigil`, `pdftract`, `bf-*`). Agents do not share task content, but they do share
the inference gateway. Simultaneity across 11+ workers is the signature of upstream service
flakiness (the "If 10+ crashes in 10 minutes → INFRASTRUCTURE EVENT" rule from
`docs/crash-response-guide.md`, here in its exit=1/service form).

Corroborating live state at analysis time (20:06–20:15Z):

- Gateway health endpoint answers **HTTP 200 `ok`** (with `-k`; see §5) — currently up, so the
  waves are intermittent unavailability, not a hard outage.
- Resource monitors logged `CRITICAL CPU load critical: 19.60 / 20.24` at 18:40–18:45Z
  (`.beads/logs/resource-alerts.log`) — CPU pressure on a 12-core box running ~19 concurrent
  workers, a plausible contributor to per-request latency/timeouts during waves. No kernel OOM
  followed those peaks; memory was never exhausted at the system level.

**Contributing factor, needle↔bead integration:** every pluck cycle today logs
`Failed to load full inventory … failed to parse backend 'bead-rs' operation 'list_all' as
JsonLines` (also `failed to list beads for post-dispatch audit`). Dependency-aware sorting silently
falls back to simple sort and the post-dispatch audit is skipped. Degraded, non-fatal, but it means
dispatch-time guards that depend on inventory (dependency-aware ordering, audit) are running
weakened.

## 3. Kernel OOM census — every kill is synthetic or an intentional bound

```
$ journalctl -k --since "2026-09-02" | grep "Killed process"
```

| Scope | Process killed | anon-rss | Assessment |
|---|---|---|---|
| `mw-oomdbg-*.scope` (×2), `mw-oom-N.scope` | bash | 18 MB | Deliberate OOM-debug probes (memory-watch tests) |
| `probe-hog-*.scope` | python3 | 65 MB | Deliberate hog probe (early-warning detection tests) |
| `safe-git-gc-<pid>-1.scope` (×6, 07:38–15:06Z) | bash | 63 MB | Bounded gc stage/verify runs; one scope (08:32:59Z) hit its bound → `Failed with result 'oom-kill'` |

- Every kill carries `oom_score_adj=200` and a test/gc scope name. **No needle, claude, node, or git
  maintenance process of any agent was OOM-killed.**
- The 08:32:59Z `safe-git-gc` kill is the **guardrail working as intended** — a stage exceeding its
  `MemoryMax` was killed at the bound instead of growing unbounded. Adjacent scope records show the
  normal profile: stages peaking 104 MB–1.7 GB (`safe-git-gc-aggressive-verify-2937640.scope`,
  1.7 G peak) completing cleanly.
- **Repository health verified post-gc**: the daily incremental gc completed 07:55:08Z —
  `1 pack, size-pack 90.19 MiB`, checkpoint `complete`, final verification passed
  (`.beads/logs/git-gc.log`). Repo state matches the 2026-09-02 state verified in commit 6210dbf.
- System resources at analysis time: 49 GiB RAM available, 92 GB disk free, load 8.9 (12 cores).
  Healthy; no exhaustion pattern.

## 4. Live infrastructure failures found (new, 2026-09-02)

### 4a. needle-otel-collector CrashLoopBackOff — telemetry outage (root-caused)

```
$ kubectl --server=http://traefik-apexalgo-iad:8001 get pods -n needle-observability
needle-otel-collector-5d749b8cd4-zlxr9   0/1  Running(52 restarts)  4h12m
needle-otel-collector-8d48fbc66-8x847    0/1  Running(72 restarts)  6h42m
```

Crash reason (previous-container logs), **exit=1 on every start — config rejected by the binary**:

```
'filterprocessor.Config' has invalid keys: log_statements
'exporters' error reading configuration for "awss3/ledger": decoding failed due to the following error(s):
'awss3exporter.Config' has invalid keys: bucket, credentials, endpoint, path_style, prefix, region, s3_partition, tls
```

Both replicas restart on a ~5-minute cycle and never become Ready, so the Service has no endpoints
and every needle telemetry export today failed:

```
ERROR opentelemetry_sdk: BatchSpanProcessor.ExportError … Status(503) … /v1/traces
ERROR opentelemetry_sdk: BatchLogProcessor.ExportError … Status(503) … /v1/logs
```

This is the collector image having been upgraded past its config schema (`log_statements` moved from
filterprocessor to transformprocessor; the `awss3` exporter keys were renamed). **Consequence:
needle traces/logs for all of today are largely lost** — this is exactly why crash triage has had to
rely on local text logs and kernel journal rather than traces. Note the blast radius is
observability only: workers keep running with export errors retried and dropped.

**Fix (out of scope for this repo — ArgoCD-managed):** update the collector ConfigMap in
`jedarden/declarative-config` (apexalgo-iad, `needle-observability`) to the current schema —
migrate `filterprocessor: log_statements` → `transform_processor`, and rename the `awss3/ledger`
exporter keys (`s3_partition`→`s3_partition*` formats, `endpoint`/`bucket`/`credentials` → the
current `awss3exporter` field names) — then let ArgoCD sync. Do not touch live resources.

### 4b. acb-bots CreateContainerConfigError (×57 pods)

`kubectl get pods -A` shows 58 pods not-Ready, nearly all `acb-bots` in
`CreateContainerConfigError` (missing/invalid ConfigMap or Secret reference). Unrelated to the agent
fleet; noted for completeness. Same fix path: declarative-config, not kubectl.

### 4c. Health-check false-alarm vector (fix in this repo's docs)

CLAUDE.md's documented availability check fails even though the service is healthy:

```
$ curl -sf --max-time 5 https://traefik-apexalgo-iad.tail1b1987.ts.net:8444/health
curl: (60) SSL certificate problem: self-signed certificate        # exit 0≠ok, "Gateway down"
$ curl -sk --max-time 5 …/health
ok                                                                 # HTTP 200
```

The endpoint now serves a self-signed/default certificate, so **any agent running the documented
check concludes the gateway is down when it is up** — a plausible amplifier of spurious
"service failure" classifications and retry storms during today's waves. Recommended: change the
documented check to `curl -skf` (or pin the CA), and update `scripts/service-monitor.sh` if it uses
the same flag set.

## 5. Resource state at analysis time (acceptance-criteria checklist)

| Check | Result |
|---|---|
| Signal details (exit -1 / signal -1) | **Zero today**; last fleet-wide 2026-08-26T22:54:48Z |
| OOM / signal / timeout / other | Today's failures: **exit=1 service-class waves** (432), 1 timeout; no OOM, no signals |
| Resource exhaustion (memory/CPU/disk) | None at crash time: 49 GiB RAM avail, 92 GB disk free; CPU load peaked 20.2 at 18:40Z (pressure, not exhaustion — no OOM followed) |
| Existing crash investigation docs | CLAUDE.md crash sections; `docs/crash-response-guide.md`; 2026-09-02 RCA set (commits 6210dbf, 533cb46, eba6c2a, 07ab240, b1ae579) — none cover today's live telemetry outage or the cert false-alarm; no prior RCA mentions domchk-8f35a61b (`git log --grep` clean) |

## 6. Conclusions and contributing factors

**Direct answer — "what killed the agent process?":** For this dispatch and for the fleet on
2026-09-02: **nothing killed anything with a signal.** The failure signature of the day is
exit-code-1 service-class failures arriving in synchronized multi-worker waves — the upstream
model/backend path intermittently failing under a fleet of ~19 concurrent workers — plus a verified
telemetry-stack outage that degrades evidence quality but not agent liveness. The historical
signal-kill class (memcg OOM of git gc) remains durably fixed: its bound config held through
today's gc runs, and today's only memcg kills were the test harnesses exercising the detectors and
one guardrail firing inside its designed envelope.

Contributing factors, ranked:

1. **Model/backend service flakiness** (primary driver of the 432 exit=1) — evidenced by
   cross-workspace same-minute wave synchrony; gateway healthy at analysis time, so intermittent.
2. **CPU pressure from fleet parallelism** (load 19.6–20.2 at 18:40Z; needle's own normalized-load
   warning fired at dispatch: `load_1min=14.17 normalized=2.02 threshold=0.80`) — plausibly widens
   latency-sensitive failure windows during waves.
3. **Telemetry outage** (otel collector config drift) — no traces/logs all day; made every failure
   harder to attribute and likely inflated crash-alert volume.
4. **bead-rs `list_all` JsonLines parse failure** in every pluck cycle — dependency-aware sort and
   post-dispatch audit silently degraded.
5. **Alert-system self-noise** — pluck inventory shows 182 open candidates dominated by duplicate
   "analyze crash" investigation beads (the auto-split alert loop documented in prior RCAs); this
   bead is one of them. Each costs a full dispatch to conclude "no new crash."

**Recommendations (in priority order):**

1. Fix `needle-otel-collector` config in `jedarden/declarative-config` (schema migration, §4a) —
   restores the evidence base every future investigation depends on.
2. Fix the documented gateway health check to `-skf` and audit `scripts/service-monitor.sh` for the
   same cert-verification failure (§4c) — removes a standing false-"gateway down" signal.
3. Cap synchronized re-dispatch after waves: the crash-alert/circuit-breaker tooling already
   exists (`scripts/crash-circuit-breaker.sh`); ensure exit=1 waves (not just exit=-1) feed it.
4. Triage the bead-rs `list_all` parse failure with the bead-rs maintainers; it weakens
   dispatch-time guards on every cycle.
5. Stop auto-splitting generic "analyze crash logs" pluck beads when no new signal-class crash
   exists; today's evidence shows the signal class is at zero.
