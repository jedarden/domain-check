# Domain Check — Architecture Plan

## Overview

Domain Check is an authoritative domain availability checker with two interfaces:
1. **Web UI** — clean, fast, no-signup interface for humans
2. **REST API** — JSON API for programmatic/machine consumption

Both are backed by the same core engine that queries RDAP registry servers directly for definitive domain availability data.

## Design Principles

1. **Authoritative** — RDAP direct to registries, not WHOIS guesswork
2. **Zero accounts** — no signup, no API keys for basic use (rate-limited by IP)
3. **Client-side first** — web UI works without JavaScript for basic checks
4. **Self-hostable** — single binary or Docker container, zero external dependencies
5. **Privacy-respecting** — no analytics, no tracking, no data retention beyond operational caching
6. **Open source** — MIT license

## System Architecture

```
┌─────────────────────────────────────────────────────────┐
│                        Clients                          │
│                                                         │
│  ┌──────────────┐    ┌──────────────┐    ┌───────────┐  │
│  │   Browser     │    │  curl / SDK  │    │  CI/CD    │  │
│  │   (Web UI)    │    │  (REST API)  │    │  Scripts  │  │
│  └──────┬───────┘    └──────┬───────┘    └─────┬─────┘  │
└─────────┼───────────────────┼──────────────────┼────────┘
          │                   │                  │
          ▼                   ▼                  ▼
┌─────────────────────────────────────────────────────────┐
│                     HTTP Server                         │
│                                                         │
│  ┌──────────────────────────────────────────────────┐   │
│  │                  Router                           │   │
│  │                                                   │   │
│  │  GET  /                    → Web UI (HTML)        │   │
│  │  GET  /check?d=example.com → Web UI result        │   │
│  │  GET  /api/v1/check?d=...  → JSON single check   │   │
│  │  POST /api/v1/bulk         → JSON bulk check      │   │
│  │  GET  /api/v1/tlds         → Supported TLD list   │   │
│  │  GET  /health              → Health check         │   │
│  └──────────────────┬───────────────────────────────┘   │
│                     │                                    │
│  ┌──────────────────▼───────────────────────────────┐   │
│  │              Rate Limiter                         │   │
│  │                                                   │   │
│  │  Per-IP:  10 checks/minute (web), 60/min (API)   │   │
│  │  Global:  Protects against abuse                  │   │
│  │  Burst:   Allow small bursts, sliding window      │   │
│  └──────────────────┬───────────────────────────────┘   │
│                     │                                    │
│  ┌──────────────────▼───────────────────────────────┐   │
│  │              Domain Check Engine                  │   │
│  │                                                   │   │
│  │  1. Parse & validate domain input                 │   │
│  │  2. Extract TLD                                   │   │
│  │  3. Optional: DNS pre-filter (fast path)          │   │
│  │  4. Look up RDAP server from bootstrap cache      │   │
│  │  5. Query RDAP with per-registry rate limiting    │   │
│  │  6. Interpret response (200/404/429)              │   │
│  │  7. Return structured result                      │   │
│  └──────────────────┬───────────────────────────────┘   │
│                     │                                    │
│  ┌──────────────────▼───────────────────────────────┐   │
│  │           RDAP Client                             │   │
│  │                                                   │   │
│  │  ┌─────────────────────────────────────────────┐  │   │
│  │  │         Bootstrap Cache                     │  │   │
│  │  │                                             │  │   │
│  │  │  Source: data.iana.org/rdap/dns.json        │  │   │
│  │  │  Refresh: every 24 hours                    │  │   │
│  │  │  Maps: TLD → RDAP server URL               │  │   │
│  │  │  591 TLD mappings (as of 2026-03-17)        │  │   │
│  │  └─────────────────────────────────────────────┘  │   │
│  │                                                   │   │
│  │  ┌─────────────────────────────────────────────┐  │   │
│  │  │      Per-Registry Rate Limiter              │  │   │
│  │  │                                             │  │   │
│  │  │  Verisign (.com/.net):  10 req/sec          │  │   │
│  │  │  PIR (.org):            10 req/sec          │  │   │
│  │  │  Google (.dev/.app):    1 req/sec           │  │   │
│  │  │  Default (unknown):     1 req/sec           │  │   │
│  │  │                                             │  │   │
│  │  │  On HTTP 429: exponential backoff           │  │   │
│  │  │  (1s, 2s, 4s, 8s, max 30s, 3 retries)      │  │   │
│  │  └─────────────────────────────────────────────┘  │   │
│  │                                                   │   │
│  │  ┌─────────────────────────────────────────────┐  │   │
│  │  │         Result Cache                        │  │   │
│  │  │                                             │  │   │
│  │  │  TTL: 5 minutes (available domains)         │  │   │
│  │  │  TTL: 1 hour (registered domains)           │  │   │
│  │  │  Backend: in-memory (bounded LRU)           │  │   │
│  │  │  Max entries: 10,000                        │  │   │
│  │  └─────────────────────────────────────────────┘  │   │
│  └───────────────────────────────────────────────────┘   │
└─────────────────────────────────────────────────────────┘
```

## API Specification

### Single Domain Check

```
GET /api/v1/check?d=example.com
```

Response (available):
```json
{
  "domain": "example.com",
  "available": true,
  "tld": "com",
  "checked_at": "2026-03-22T14:30:00Z",
  "source": "rdap",
  "cached": false
}
```

Response (taken):
```json
{
  "domain": "google.com",
  "available": false,
  "tld": "com",
  "checked_at": "2026-03-22T14:30:00Z",
  "source": "rdap",
  "cached": false,
  "registration": {
    "registrar": "MarkMonitor Inc.",
    "created": "1997-09-15T04:00:00Z",
    "expires": "2028-09-14T04:00:00Z",
    "nameservers": ["ns1.google.com", "ns2.google.com"],
    "status": ["client delete prohibited", "client transfer prohibited"]
  }
}
```

### Multi-TLD Check (check a name across TLDs)

```
GET /api/v1/check?d=example&tlds=com,org,dev,net,io
```

Response:
```json
{
  "name": "example",
  "results": [
    { "domain": "example.com", "available": false, "tld": "com" },
    { "domain": "example.org", "available": false, "tld": "org" },
    { "domain": "example.dev", "available": true, "tld": "dev" },
    { "domain": "example.net", "available": false, "tld": "net" },
    { "domain": "example.io", "available": true, "tld": "io" }
  ],
  "checked_at": "2026-03-22T14:30:00Z"
}
```

### Bulk Check

```
POST /api/v1/bulk
Content-Type: application/json

{
  "domains": [
    "numcrunch.com",
    "dimecalc.com",
    "publiccalc.com",
    "publiccalc.org"
  ]
}
```

Response:
```json
{
  "results": [
    { "domain": "numcrunch.com", "available": false },
    { "domain": "dimecalc.com", "available": true },
    { "domain": "publiccalc.com", "available": true },
    { "domain": "publiccalc.org", "available": true }
  ],
  "checked_at": "2026-03-22T14:30:00Z",
  "duration_ms": 342
}
```

Limits: max 50 domains per request.

### Supported TLDs

```
GET /api/v1/tlds
```

Response:
```json
{
  "count": 591,
  "tlds": ["aaa", "aarp", "abarth", "abb", "abbott", "..."],
  "bootstrap_updated": "2026-03-17T18:19:24Z"
}
```

### Error Responses

```json
{
  "error": "rate_limited",
  "message": "Rate limit exceeded. Try again in 42 seconds.",
  "retry_after": 42
}
```

```json
{
  "error": "invalid_domain",
  "message": "Invalid domain format: 'not a domain'"
}
```

```json
{
  "error": "unsupported_tld",
  "message": "TLD '.xyz123' is not in the RDAP bootstrap registry"
}
```

## Web UI

### Design Goals
- **Instant feel** — results appear as fast as RDAP responds (~100-200ms)
- **No JavaScript required** — basic form submission works without JS
- **Progressive enhancement** — JS adds live results, multi-TLD expansion, keyboard shortcuts
- **Mobile-first** — single input field, clear results
- **No clutter** — no ads, no signup prompts, no tracking banners

### Pages

**`GET /`** — Home page with search input
- Single text input: "Enter a domain name"
- Optional TLD checkboxes (.com, .org, .net, .dev, .io, .app)
- Submit button
- Works as a plain HTML form (no JS needed)

**`GET /check?d=example.com`** — Results page
- Shows availability result with color coding (green = available, red = taken)
- For taken domains: shows registrar, registration date, expiry, nameservers
- "Also check" section: same name across other TLDs
- Shareable URL (bookmarkable results)
- JSON link for API access to same result

### Progressive Enhancement (with JS)
- Live results as you type (debounced, after 3+ characters + TLD)
- Multi-TLD results expand inline without page reload
- Keyboard shortcut: Enter to check, Tab to cycle TLDs
- Copy-to-clipboard for available domains
- History of recent checks (localStorage, never sent to server)

### HTML Structure
```
┌─────────────────────────────────────┐
│  Domain Check                       │
│  Authoritative availability lookup  │
│                                     │
│  ┌─────────────────────────┐ ┌───┐  │
│  │ example.com             │ │ → │  │
│  └─────────────────────────┘ └───┘  │
│                                     │
│  ☐ .com  ☐ .org  ☐ .net            │
│  ☐ .dev  ☐ .io   ☐ .app            │
│                                     │
│  ─────────────────────────────────  │
│                                     │
│  ● example.com — Available          │
│    (checked via RDAP, 112ms)        │
│                                     │
│  ● example.org — Taken              │
│    Registrar: PIR                   │
│    Registered: 1995-08-14           │
│    Expires: 2025-08-13              │
│                                     │
│  ─────────────────────────────────  │
│  Powered by RDAP · Open Source      │
│  github.com/jedarden/domain-check   │
└─────────────────────────────────────┘
```

## Technology Stack

### Backend

**Language: Go**

Rationale:
- Single binary deployment (no runtime dependencies)
- Excellent HTTP server stdlib (`net/http`)
- Built-in concurrency (goroutines for parallel RDAP queries)
- Fast startup, low memory footprint
- Easy cross-compilation for Docker
- Strong ecosystem for HTTP clients, JSON parsing, rate limiting

**Key packages:**
- `net/http` — HTTP server and RDAP client
- `golang.org/x/time/rate` — per-registry rate limiting
- `sync` — concurrent query coordination
- `html/template` — server-side HTML rendering
- `encoding/json` — API responses and RDAP parsing

### Frontend

**Approach: Server-rendered HTML + vanilla JS progressive enhancement**

- HTML templates rendered server-side (Go `html/template`)
- CSS: minimal, inline or single file, no framework
- JS: vanilla ES6, no build step, no framework
  - Debounced live search
  - Fetch API for async checks
  - DOM manipulation for result display
- Total JS payload target: < 5 KB

Rationale: This is a single-purpose utility. A React/Next.js app would be massive overkill. Server-rendered HTML with a sprinkle of JS gives the best performance, smallest payload, and simplest deployment.

### Deployment

**Primary: Single Go binary**
```bash
./domain-check serve --port 8080
```

**Docker:**
```dockerfile
FROM golang:1.22-alpine AS build
WORKDIR /app
COPY . .
RUN go build -o domain-check .

FROM alpine:3.19
COPY --from=build /app/domain-check /usr/local/bin/
EXPOSE 8080
CMD ["domain-check", "serve"]
```

**Cloudflare Pages + Workers (alternative):**
The web UI could be static HTML on Pages with a Worker handling API requests. This gives global edge deployment with zero infrastructure management. However, Workers have CPU time limits that may constrain bulk checks.

## Core Engine Design

### Bootstrap Manager

```go
type BootstrapManager struct {
    mu       sync.RWMutex
    servers  map[string]string // TLD → RDAP server URL
    updated  time.Time
}

// Refresh downloads the IANA bootstrap file
func (b *BootstrapManager) Refresh() error
// Lookup returns the RDAP server for a TLD
func (b *BootstrapManager) Lookup(tld string) (string, error)
```

- Loads `https://data.iana.org/rdap/dns.json` on startup
- Refreshes every 24 hours in background
- Thread-safe reads via RWMutex
- Falls back to hardcoded servers for .com, .net, .org if bootstrap fetch fails

### RDAP Client

```go
type RDAPClient struct {
    httpClient  *http.Client
    limiters    map[string]*rate.Limiter // per-registry rate limiters
    bootstrap   *BootstrapManager
    cache       *ResultCache
}

// Check queries RDAP for a single domain
func (r *RDAPClient) Check(ctx context.Context, domain string) (*DomainResult, error)
// CheckBulk queries multiple domains with parallel execution
func (r *RDAPClient) CheckBulk(ctx context.Context, domains []string) ([]*DomainResult, error)
```

Rate limiter configuration:
```go
var registryLimits = map[string]rate.Limit{
    "rdap.verisign.com":                     rate.Limit(10),  // 10/sec
    "rdap.publicinterestregistry.org":       rate.Limit(10),  // 10/sec
    "pubapi.registry.google":                rate.Limit(1),   // 1/sec
    "default":                               rate.Limit(2),   // 2/sec conservative
}
```

### DNS Pre-Filter (Optional Optimization)

```go
// FastCheck uses DNS to skip RDAP for domains with active nameservers
func (r *RDAPClient) FastCheck(ctx context.Context, domain string) (*DomainResult, error) {
    ns, err := net.LookupNS(domain)
    if err == nil && len(ns) > 0 {
        // NS records exist → definitely registered, skip RDAP
        return &DomainResult{Domain: domain, Available: false, Source: "dns"}, nil
    }
    // NXDOMAIN or error → must verify with RDAP
    return r.Check(ctx, domain)
}
```

This saves RDAP queries for domains that are obviously registered (have active DNS). The DNS check is nearly instant and has no rate limits.

### Result Cache

```go
type ResultCache struct {
    mu      sync.RWMutex
    entries map[string]*CacheEntry
    maxSize int
}

type CacheEntry struct {
    Result    *DomainResult
    ExpiresAt time.Time
}
```

TTL strategy:
- **Available domains: 5 minutes** — short TTL because availability can change quickly (someone might register it)
- **Registered domains: 1 hour** — longer TTL because registered domains rarely become available suddenly
- **Errors: 30 seconds** — retry soon on transient failures

### Domain Result

```go
type DomainResult struct {
    Domain      string        `json:"domain"`
    Available   bool          `json:"available"`
    TLD         string        `json:"tld"`
    CheckedAt   time.Time     `json:"checked_at"`
    Source      string        `json:"source"` // "rdap", "dns", "cache"
    Cached      bool          `json:"cached"`
    DurationMs  int64         `json:"duration_ms"`
    Registration *Registration `json:"registration,omitempty"`
    Error       string        `json:"error,omitempty"`
}

type Registration struct {
    Registrar   string   `json:"registrar,omitempty"`
    Created     string   `json:"created,omitempty"`
    Expires     string   `json:"expires,omitempty"`
    Nameservers []string `json:"nameservers,omitempty"`
    Status      []string `json:"status,omitempty"`
}
```

## Rate Limiting Strategy

### Client-Facing (per IP)

| Interface | Limit | Window |
|-----------|-------|--------|
| Web UI | 10 checks/minute | Sliding window (token bucket) |
| API (no key) | 60 checks/minute | Sliding window (token bucket) |
| Bulk endpoint | 5 requests/minute (50 domains each = 250 domains/min) | Sliding window |

Implementation: `golang.org/x/time/rate` with a per-IP map. Stale entries evicted every 10 minutes to prevent memory growth.

IP extraction priority: `CF-Connecting-IP` → `X-Real-IP` → first entry of `X-Forwarded-For` → `r.RemoteAddr`. Configurable via `--trust-proxy` flag (when false, only `r.RemoteAddr` is used).

### Registry-Facing (per upstream)

| Registry | Max Rate | Concurrency (semaphore) | Backoff on 429 |
|----------|----------|------------------------|----------------|
| Verisign (.com/.net) | 10/sec | 10 concurrent | 1s, 2s, 4s (3 retries) |
| PIR (.org) | 10/sec | 10 concurrent | 1s, 2s, 4s (3 retries) |
| Google (.dev/.app) | 1/sec | 2 concurrent | 5s, 10s, 20s (3 retries) |
| Default (unknown registry) | 2/sec | 3 concurrent | 2s, 4s, 8s (3 retries) |

Implementation: `golang.org/x/time/rate` for rate limiting, `golang.org/x/sync/semaphore` for concurrency. Combined in bulk operations: `errgroup.SetLimit(50)` global cap + per-registry semaphores.

## Input Validation

### Domain Name Rules (RFC 1035, RFC 1123, RFC 5891)

- Total FQDN length: 1–253 characters
- Label length: 1–63 characters
- Allowed characters: `[a-zA-Z0-9-]` (LDH rule)
- Labels must not start or end with a hyphen
- Labels must not have hyphens at positions 3–4 unless starting with `xn--` (punycode)
- At least two labels required (name + TLD)
- Case-insensitive (normalize to lowercase)

### Processing Pipeline

1. Trim whitespace, strip trailing dot, lowercase
2. Reject if contains `/:@` (URL fragments, not domains)
3. IDN conversion via `golang.org/x/net/idna` (Lookup profile) — handles punycode encoding
4. Extract TLD via `golang.org/x/net/publicsuffix` — handles multi-level TLDs (`.co.uk`, `.com.au`)
5. Validate each label against LDH regex: `^[a-zA-Z0-9]([a-zA-Z0-9-]{0,61}[a-zA-Z0-9])?$`
6. Verify TLD exists in IANA bootstrap or has known WHOIS server
7. Strip subdomains: `www.example.co.uk` → `example.co.uk` via `EffectiveTLDPlusOne()`

## ccTLD Support and WHOIS Fallback

### Lookup Flow

```
Input domain
  → Extract TLD via PSL (golang.org/x/net/publicsuffix)
  → Check IANA RDAP bootstrap for TLD
  → If RDAP server exists: query RDAP (structured JSON)
  → If no RDAP server: fall back to WHOIS port 43 (text)
  → Parse response to determine availability
```

### ccTLD Coverage

~314M domains under TLDs with RDAP. ~45M under TLDs without (requiring WHOIS fallback). Major ccTLDs without RDAP: `.de`, `.co`, `.jp`, `.kr`, `.cn`, `.ru`, `.se`, `.ch`, `.at`, `.be`, `.nz`, `.gg`. Full list in [research/06-cctld-and-psl.md](../research/06-cctld-and-psl.md).

### WHOIS Fallback

Library: `github.com/likexian/whois` + `github.com/likexian/whois-parser`

The WHOIS path returns a simplified result (available/taken + registrar if parseable). Full RDAP-quality metadata is not available via WHOIS due to unstructured text format.

## RDAP Response Parsing

### Robustness Rules (from empirical testing of 8+ registries)

1. **Normalize `ldhName` to lowercase** — Verisign returns UPPERCASE
2. **Handle empty body on 404** — Verisign returns 0 bytes
3. **Optional access for all fields** — `handle`, `unicodeName`, `links`, `secureDNS`, `port43`, `redacted` may be absent
4. **Strip trailing dots from nameserver names** — DENIC and Nominet append them
5. **Lenient date parsing** — handle 0–6 fractional digits, `Z` and `±HH:MM` offsets
6. **Ignore unknown event types** — registries use non-standard types (`reregistration`, `delegation check`)
7. **Do not validate `rdapConformance` strictly** — registries use custom extensions
8. **Do not distinguish 400 vs 404** — some registries return 404 for invalid format

See full edge case matrix in [research/07-rdap-response-edge-cases.md](../research/07-rdap-response-edge-cases.md).

### Timeout Configuration

| Timeout | Value | Purpose |
|---------|-------|---------|
| TCP connect | 5s | Detect unreachable registries |
| TLS handshake | 5s | Detect TLS issues |
| Response header | 10s | Detect slow/hung registries |
| Total request | 15s | Hard safety net |
| Bulk operation | 30s | Max wall time for a bulk request |

Connection failures (HTTP 000) treated as a separate error class, not as "available."

## Security

### SSRF Prevention

1. **RDAP URL allowlist** — only query URLs present in the IANA bootstrap file. Reject any URL not in the allowlist.
2. **Private IP blocking** — custom `net.Dialer.Control` callback blocks connections to loopback, link-local, RFC 1918, and ULA addresses after DNS resolution (defeats DNS rebinding).
3. **Redirect validation** — `CheckRedirect` on `http.Client` verifies redirect targets are in the RDAP allowlist. Max 3 redirects.
4. **Input sanitization** — reject domains containing `/:@` before any network request.

### XSS Prevention

Go's `html/template` auto-escapes by context (HTML body, attributes, URLs, JS, CSS). Rules:
- Never use `template.HTML` type for user-supplied data
- Always use quoted attributes in templates
- Pass data via `data-` attributes for JS consumption, not inline `<script>` blocks

### Content Security Policy

```
default-src 'none';
script-src 'self';
style-src 'self';
img-src 'self';
form-action 'self';
base-uri 'self';
frame-ancestors 'none'
```

All JS in external files (`/static/app.js`), no inline scripts, no nonce needed.

Additional headers: `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, `Referrer-Policy: strict-origin-when-cross-origin`.

### Request Body Limits

`http.MaxBytesReader` on all handlers reading a body. Bulk endpoint: 64 KB max body + 50 domain item limit.

### CORS

`Access-Control-Allow-Origin: *` for API endpoints (public API, same as RDAP registries themselves). Configurable via `--cors-origins` flag for self-hosted instances.

## Observability

### Structured Logging

`log/slog` (stdlib, Go 1.21+). JSON handler for production, text handler for development.

Per-request fields: `request_id`, `method`, `path`, `remote_ip`, `status`, `duration_ms`.
Per-RDAP-call fields: `registry`, `domain`, `status_code`, `duration_ms`.

### Metrics (Prometheus)

```
domcheck_requests_total{method, path, status}        — counter
domcheck_request_duration_seconds{method, path}       — histogram
domcheck_rdap_requests_total{registry, status}        — counter (success/error/timeout)
domcheck_rdap_duration_seconds{registry}              — histogram
domcheck_cache_hits_total{result}                     — counter (hit/miss)
domcheck_active_checks                                — gauge (in-flight goroutines)
domcheck_bulk_check_size                              — histogram (batch size)
```

Exposed at `GET /metrics`. Go runtime metrics included automatically.

### Health Check

`GET /health` returns:
- `status: ok|degraded|unhealthy`
- `bootstrap_age`: time since last IANA bootstrap refresh
- `uptime`: process uptime
- `checks_served`: total checks since startup

Degraded: bootstrap older than 48 hours. Unhealthy: bootstrap older than 7 days or failed to load.

### Request Tracing

Lightweight request ID middleware (16 hex chars from `crypto/rand`). Accepts `X-Request-Id` from upstream proxy, generates if absent. Set on response header and propagated to upstream RDAP calls. Thread-safe via `context.WithValue`.

## Configuration

### Mechanism

CLI flags > environment variables > config file > defaults. Using `github.com/peterbourgon/ff/v4`.

Env var prefix: `DOMCHECK_`. Config file: YAML, path set via `--config` flag.

### Parameters

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `--addr` | `DOMCHECK_ADDR` | `:8080` | Listen address |
| `--config` | — | — | Path to YAML config file |
| `--cache-size` | `DOMCHECK_CACHE_SIZE` | `10000` | LRU cache max entries |
| `--cache-ttl-available` | `DOMCHECK_CACHE_TTL_AVAILABLE` | `5m` | TTL for available results |
| `--cache-ttl-registered` | `DOMCHECK_CACHE_TTL_REGISTERED` | `1h` | TTL for registered results |
| `--bootstrap-refresh` | `DOMCHECK_BOOTSTRAP_REFRESH` | `24h` | IANA bootstrap refresh interval |
| `--trust-proxy` | `DOMCHECK_TRUST_PROXY` | `false` | Trust X-Forwarded-For headers |
| `--cors-origins` | `DOMCHECK_CORS_ORIGINS` | `*` | Allowed CORS origins |
| `--metrics` | `DOMCHECK_METRICS` | `true` | Enable /metrics endpoint |
| `--log-format` | `DOMCHECK_LOG_FORMAT` | `json` | Log format: json or text |
| `--log-level` | `DOMCHECK_LOG_LEVEL` | `info` | Log level: debug, info, warn, error |

### TLS

The server does not terminate TLS itself. Deploy behind a reverse proxy (Caddy, nginx, Cloudflare) for TLS. This keeps the binary simple and avoids certificate management.

## CLI Mode

In addition to `serve`, the binary supports direct CLI usage:

```bash
# Single domain
domain-check check example.com

# Multi-TLD
domain-check check example --tlds com,org,dev,net,io

# Bulk from file
domain-check bulk domains.txt --json --concurrency 20

# Output formats: text (default), json, csv
domain-check check example.com --format json
```

This enables scripting and CI/CD usage without starting a server.

## Graceful Degradation

| Failure | Behavior |
|---------|----------|
| IANA bootstrap unreachable | Fall back to hardcoded servers for .com, .net, .org. Other TLDs return `unsupported_tld` error until bootstrap recovers. |
| Specific registry down | Return per-domain error in bulk results. Other domains in the batch still succeed. |
| Registry rate limiting (429) | Exponential backoff (3 retries). If still 429, return `upstream_rate_limited` error for that domain. |
| Cache full | LRU eviction — oldest entries removed to make space. |
| Rate limit queue full | Domains queued for a rate-limited registry wait up to 10s. If the queue exceeds 100 items, new requests get `service_busy` error. |
| WHOIS fallback failure | Return `whois_unavailable` error for that TLD. |

Bulk requests always return partial results — each domain has its own `error` field. The overall request succeeds (HTTP 200) with per-domain success/failure.

## Testing Strategy

### Unit Tests
- Domain validation: 30+ table-driven test cases (valid, invalid, IDN, edge cases)
- Bootstrap parser: test with real and malformed bootstrap JSON
- RDAP response parser: fixtures from 8+ registries in `testdata/rdap/`
- Cache: TTL expiration, LRU eviction, concurrent access
- Rate limiter: token bucket behavior, per-IP isolation

### Integration Tests
- Full middleware chain via `httptest.NewRecorder`
- Mock RDAP server via `httptest.NewServer` with recorded fixtures
- Rate limiting triggers correctly
- Body size limits enforced
- Security headers present

### Fuzz Testing
- Go 1.18+ native fuzzing on `ValidateDomain()` input
- Properties checked: no panics, valid outputs have dots, labels ≤ 63 chars

### Load Testing
- `hey` for quick smoke tests during development
- `vegeta` for precise rate-based characterization
- Target: p99 < 500ms for cached responses, p99 < 2s for uncached

### Fixture Recording
```bash
go test -tags=record  # captures live RDAP responses as testdata fixtures
```

Fixtures committed to repo. CI runs against fixtures (never hits real registries).

## Project Structure

```
domain-check/
├── cmd/
│   └── domain-check/
│       └── main.go              # CLI entry point (serve + check + bulk subcommands)
├── internal/
│   ├── server/
│   │   ├── server.go            # HTTP server setup, graceful shutdown
│   │   ├── routes.go            # Route definitions
│   │   ├── handlers_web.go      # Web UI handlers
│   │   ├── handlers_api.go      # API handlers
│   │   └── middleware.go        # Rate limiting, logging, CORS, security headers, request ID
│   ├── checker/
│   │   ├── checker.go           # Domain check engine (orchestrates RDAP/WHOIS/DNS)
│   │   ├── rdap.go              # RDAP client with response parsing
│   │   ├── whois.go             # WHOIS fallback for ccTLDs without RDAP
│   │   ├── bootstrap.go         # IANA bootstrap manager (refresh, lookup, fallback)
│   │   ├── dns.go               # DNS pre-filter (fast path for domains with NS records)
│   │   ├── cache.go             # LRU result cache with per-status TTLs
│   │   ├── ratelimit.go         # Per-registry rate limiting + concurrency semaphores
│   │   └── ssrf.go              # Safe dialer with private IP blocking
│   └── domain/
│       ├── parse.go             # Domain parsing, validation, IDN conversion, PSL lookup
│       └── result.go            # Result types (DomainResult, Registration, etc.)
├── web/
│   ├── templates/
│   │   ├── index.html           # Home page with search form
│   │   ├── result.html          # Check result page
│   │   └── layout.html          # Base layout with security headers, CSP
│   ├── static/
│   │   ├── style.css            # Minimal CSS (mobile-first)
│   │   └── app.js               # Progressive enhancement (debounced search, multi-TLD)
│   └── embed.go                 # //go:embed directives for templates/ and static/
├── testdata/
│   └── rdap/                    # Recorded RDAP response fixtures (8+ registries)
├── docs/
│   ├── research/                # Research documents (01-08)
│   └── plan/                    # This file
├── Dockerfile
├── go.mod
├── go.sum
├── LICENSE                      # MIT
└── README.md
```

## Implementation Phases

### Phase 1: Core Engine
- IANA bootstrap loader and cacher with 24h refresh
- RDAP client with per-registry rate limiting and concurrency semaphores
- WHOIS fallback client for ccTLDs without RDAP
- Domain parsing: validation, IDN conversion, PSL-based TLD extraction
- Lenient RDAP response parser (handles all edge cases from 8+ registries)
- In-memory LRU cache with per-status TTLs
- SSRF-safe HTTP client (private IP blocking, URL allowlist)
- Unit tests with recorded fixtures, table-driven validation tests, fuzz tests

### Phase 2: API Server
- HTTP server with `net/http` router
- `GET /api/v1/check` — single domain check
- `GET /api/v1/check?tlds=` — multi-TLD check
- `POST /api/v1/bulk` — bulk check (max 50 domains, 64 KB body)
- `GET /api/v1/tlds` — supported TLD list
- `GET /health` — health check (bootstrap age, uptime, status)
- `GET /metrics` — Prometheus metrics
- Per-IP rate limiting middleware with configurable trust-proxy
- CORS middleware (configurable origins)
- Security headers middleware (CSP, X-Frame-Options, etc.)
- Request ID middleware
- Request body size limits
- JSON error responses with proper HTTP status codes
- Integration tests via `httptest.NewRecorder`

### Phase 3: Web UI
- `//go:embed` for templates and static assets (single binary)
- Server-rendered HTML templates (Go `html/template`)
- Home page with search form (works without JS)
- Results page with styled output and registration details
- Progressive enhancement JS: debounced live search, multi-TLD expansion, copy-to-clipboard
- Mobile-first responsive CSS
- Shareable result URLs
- No inline JS (CSP-compliant)

### Phase 4: CLI Mode + Optimization
- `domain-check check` subcommand (single and multi-TLD)
- `domain-check bulk` subcommand (file input, JSON/CSV/text output)
- DNS NS pre-filter (fast path skipping RDAP for domains with active nameservers)
- Parallel bulk execution: `errgroup` with per-registry semaphores
- HTTP client connection pooling tuned per registry
- Graceful shutdown with 15s drain
- Configuration via ff/v4 (flags → env → config file → defaults)

### Phase 5: Deployment + CI
- Dockerfile (multi-stage: golang:alpine → alpine, single binary + embedded assets)
- GitHub Actions CI: lint (`golangci-lint`), test, fuzz (30s), build, release binaries
- Load testing baseline with `vegeta`
- README with usage examples for all interfaces (web, API, CLI)
- robots.txt (noindex result pages to prevent search engines caching availability data)
- Favicon and OG social card
