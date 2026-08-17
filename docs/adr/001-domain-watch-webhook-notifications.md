# ADR-001: 2026-07-20 — Domain Watch: Webhook-Based Notifications for Availability Changes

## Status

Accepted

## Context

As shipped, domain-check answers exactly one question: "is this domain available right now?" Every check is a single stateless RDAP round-trip; the process holds no memory of what a caller has asked before beyond the 5-minute/1-hour result cache. That fits the tool's original brief (fast, zero-tracking, no-signup lookup) and is reflected in Design Principle #5 ("no data retention beyond operational caching").

In practice, though, the single highest-value action a domain-availability tool can enable is not "tell me now" but "tell me the moment it changes." Catchy short domains and expiring registrations are almost never caught by someone polling manually — they're caught by whoever has an automated watcher running. Every popular tool in this space (whois watchers, expired-domain trackers, drop-catching services) exists primarily to serve that use case, and domain-check currently has no answer for it beyond "run the CLI in a cron job yourself" — which isn't even documented in the README.

This ADR is scoped to the architecture of that next feature. It does not fix the separate finding (filed as beads alongside this ADR) that the service currently has no live deployment at all — no ArgoCD `Application` is registered for `k8s/apexalgo-iad/domain-check/` in declarative-config and the `ronaldraygun/domain-check` Docker Hub repository has never had an image pushed to it. Getting a watch feature shipped presupposes the base service is actually running somewhere first; that's a prerequisite, not part of this decision.

## Decision

Add a **Domain Watch** capability: a caller registers a `(domain, webhook_url)` pair; a background poller re-checks watched domains on a schedule and POSTs a signed JSON payload to the webhook the moment a domain's availability flips from taken to available.

Concretely:
- A new `internal/watch` package owns a small embedded store (`etcd-io/bbolt` — pure Go, single file, no separate server process, consistent with the existing "single binary, zero external dependencies" principle) holding `{domain, webhook_url, created_at, expires_at, last_checked, last_status}`.
- A background goroutine (following the existing `BootstrapManager` 24h-refresh pattern in `internal/bootstrap/`) walks the store on a fixed interval (e.g. every 15 minutes) and re-checks each watched domain through the *existing* `checker.Checker.Check` — reusing the current per-registry rate limiters (`internal/ratelimit/`) and result cache (`internal/checker/cache.go`) rather than adding a second RDAP code path.
- Watched-domain polling runs under its own low-priority semaphore, separate from live interactive traffic, so a large watch list can never starve real-time user requests.
- On a taken→available transition, POST `{domain, available: true, checked_at}` to the stored webhook URL with an HMAC-SHA256 signature (shared secret returned at registration time), 3 retries with backoff, then mark the watch delivered-and-expired — single-fire, not an ongoing subscription, so it can't become a spam vector.
- Hard caps bound the privacy/abuse surface: max N watches per IP per day, max TTL of 90 days per watch (auto-expires — operational state with a hard ceiling, not indefinite tracking, so it stays in the spirit of "no data retention"), and only a domain name + webhook URL are ever stored — no email, no account.
- New endpoints: `POST /api/v1/watch` (register, returns id + secret), `DELETE /api/v1/watch/{id}` (cancel, requires secret).
- The outbound webhook POST must go through a dedicated SSRF-safe dialer (reusing the private-IP-blocking logic already in `internal/checker/ssrf.go` / `internal/httpclient/`), since this is the project's first *outbound-to-arbitrary-URL* request path — distinct from the existing inbound RDAP allowlist.

## Alternatives Considered

1. **Document a CLI cron pattern instead of building a server feature.** Zero code, ships today. Rejected as the primary answer: it only serves CLI/self-hosted users who already run their own infra. The Web UI and hosted-API audience — the majority of traffic — get nothing, and they're the users most likely to want this.
2. **Long-lived SSE/WebSocket connection per watched domain** (client holds a connection open, server pushes on change). Avoids persistent storage and webhook retry/delivery logic entirely. Rejected as the *primary* mechanism because it requires the caller's process or browser tab to stay connected for days-to-weeks, which doesn't match a "check and walk away" usage pattern. Worth revisiting later as a *secondary*, lower-effort option specifically for the Web UI (a user actively staring at a results page).
3. **Outsource scheduling/storage to a Cloudflare Worker + KV cron**, no change to the Go binary. Rejected: breaks the "self-hostable single binary/Docker container, zero external dependencies" principle. A self-hoster running their own instance would get a feature that silently no-ops without a separate Cloudflare account — a confusing, non-obvious dependency bolted onto an otherwise self-contained tool.
4. **Email digests via SMTP instead of webhooks.** More approachable for non-technical users. Rejected for v1: requires storing a real email address (a materially different privacy posture than a webhook URL) plus an SMTP relay dependency/credential. Webhook-first keeps the "no PII" property intact, and is trivially bridgeable to email later via a webhook-to-email relay the user opts into themselves (e.g. their own Zapier/n8n).

## Consequences

- **Positive:** turns domain-check from a lookup tool into the thing people actually want a domain checker for (catching drops); reuses the existing RDAP client, cache, and rate limiter instead of duplicating logic; bounded TTL + single-fire delivery keeps it consistent with the project's zero-tracking ethos rather than becoming a silent long-term surveillance feature.
- **Negative / new surface:** introduces the project's first persistent state and its first outbound-to-arbitrary-URL network path (webhook delivery) — a new SSRF surface distinct from the already-hardened inbound RDAP allowlist, requiring its own private-IP-blocking dialer.
- **Operational:** the embedded store needs a persistent volume. The current Deployment mounts only an `emptyDir` (see `k8s/apexalgo-iad/domain-check/deployment.yaml` in declarative-config), which loses all watches on every pod restart or reschedule — unacceptable for a feature whose entire value is surviving over days-to-weeks. This is a real infra change (PVC) to land in declarative-config alongside the code, not an afterthought.
- **Scope:** background poller + webhook delivery/retry + abuse caps is a genuine multi-week feature, not a quick patch. It should land behind a feature flag (`--enable-watch`, default off) so self-hosters who want the simple stateless tool aren't forced into the new persistence/PVC requirement.

## Implementation Plan

This ADR will be implemented across multiple phases:

1. **Phase 1:** Core watch infrastructure (`internal/watch` package with bbolt store)
2. **Phase 2:** Background poller with rate-limited RDAP checks
3. **Phase 3:** Webhook delivery with SSRF-safe dialer and retry logic
4. **Phase 4:** API endpoints (`POST /api/v1/watch`, `DELETE /api/v1/watch/{id}`)
5. **Phase 5:** Abuse caps and feature flag integration
6. **Phase 6:** PVC for persistent volume in Kubernetes manifests

## References

- Original plan.md: ADR-001 section
- Related beads: Implementation tracking beads filed alongside this ADR
- Design Principles: Zero-tracking, self-hostable single binary
