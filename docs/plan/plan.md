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
| Web UI | 10 checks/minute | Sliding window |
| API (no key) | 60 checks/minute | Sliding window |
| Bulk endpoint | 5 requests/minute (50 domains each = 250 domains/min) | Sliding window |

### Registry-Facing (per upstream)

| Registry | Max Rate | Burst | Backoff on 429 |
|----------|----------|-------|----------------|
| Verisign (.com/.net) | 10/sec | 20 | 1s, 2s, 4s (3 retries) |
| PIR (.org) | 10/sec | 20 | 1s, 2s, 4s (3 retries) |
| Google (.dev/.app) | 1/sec | 5 | 5s, 10s, 20s (3 retries) |
| Default (unknown registry) | 2/sec | 5 | 2s, 4s, 8s (3 retries) |

## Project Structure

```
domain-check/
├── cmd/
│   └── domain-check/
│       └── main.go              # CLI entry point
├── internal/
│   ├── server/
│   │   ├── server.go            # HTTP server setup
│   │   ├── routes.go            # Route definitions
│   │   ├── handlers_web.go      # Web UI handlers
│   │   ├── handlers_api.go      # API handlers
│   │   └── middleware.go        # Rate limiting, logging, CORS
│   ├── checker/
│   │   ├── checker.go           # Domain check engine
│   │   ├── rdap.go              # RDAP client
│   │   ├── bootstrap.go         # IANA bootstrap manager
│   │   ├── dns.go               # DNS pre-filter
│   │   ├── cache.go             # Result cache
│   │   └── ratelimit.go         # Per-registry rate limiting
│   └── domain/
│       ├── parse.go             # Domain parsing & validation
│       └── result.go            # Result types
├── web/
│   ├── templates/
│   │   ├── index.html           # Home page
│   │   ├── result.html          # Check result page
│   │   └── layout.html          # Base layout
│   └── static/
│       ├── style.css            # Minimal CSS
│       └── app.js               # Progressive enhancement JS
├── docs/
│   ├── research/                # Research documents
│   └── plan/                    # This file
├── Dockerfile
├── go.mod
├── go.sum
├── LICENSE                      # MIT
└── README.md
```

## Implementation Phases

### Phase 1: Core Engine
- IANA bootstrap loader and cacher
- RDAP client with per-registry rate limiting
- Domain parsing and validation
- Single domain check with full RDAP response parsing
- In-memory result cache
- Unit tests for all core logic

### Phase 2: API Server
- HTTP server with router
- `GET /api/v1/check` — single domain check
- `GET /api/v1/check?tlds=` — multi-TLD check
- `POST /api/v1/bulk` — bulk check
- `GET /api/v1/tlds` — supported TLD list
- `GET /health` — health check
- Per-IP rate limiting middleware
- JSON error responses
- Integration tests

### Phase 3: Web UI
- Server-rendered HTML templates
- Home page with search form
- Results page with styled output
- Progressive enhancement JS (live search, multi-TLD expansion)
- Mobile-responsive CSS
- Shareable result URLs

### Phase 4: DNS Pre-Filter + Optimization
- DNS NS lookup as fast path
- Parallel bulk execution with goroutine pool
- Connection pooling for RDAP HTTP client
- Graceful shutdown

### Phase 5: Deployment
- Dockerfile (multi-stage build)
- GitHub Actions CI (lint, test, build, release)
- Cloudflare Pages deployment (optional)
- README with usage examples
