# Domain Check

Authoritative domain availability checker powered by RDAP — the ICANN-mandated successor to WHOIS.

## Architecture

- **Language:** Go (single binary, zero runtime dependencies)
- **Core:** RDAP client querying registry servers directly for definitive availability data
- **Interfaces:** REST API (net/http) + Web UI (html/template, go:embed) + CLI
- **Caching:** In-memory bounded LRU (5min available, 1h registered)
- **Rate limiting:** Per-registry (Verisign 10/s, Google 1/s) + per-IP client limits

## Package Layout

```
cmd/domain-check/main.go          # Entry point
internal/
  domain/     # Input validation, IDN, TLD extraction via publicsuffix
  bootstrap/  # IANA RDAP bootstrap loader + 24h cache refresh
  rdap/       # RDAP client, response parser, per-registry rate limiting
  whois/      # WHOIS fallback for ccTLDs without RDAP
  cache/      # In-memory LRU with per-status TTLs
  httpclient/ # SSRF-safe HTTP client (private IP blocking)
  ratelimit/  # Per-IP rate limiter middleware
  server/     # HTTP server, router, middleware, API handlers
  web/        # HTML templates, static assets, template handlers
  cli/        # CLI subcommands (check, bulk)
```

## Development

```bash
go build ./...
go test ./...
go test -fuzz=. -fuzztime=30s ./internal/domain/
golangci-lint run
```

## Key Docs

- `docs/plan/plan.md` — Full architecture plan, API spec, phase breakdown
- `docs/research/08-go-implementation-patterns.md` — Go dep choices and patterns
- `docs/research/` — RDAP protocol research, rate limits, accuracy testing

## Dependencies

- `golang.org/x/net` (publicsuffix, idna)
- `golang.org/x/sync` (errgroup, semaphore)
- `golang.org/x/time/rate` (rate limiting)
- `github.com/likexian/whois` + `whois-parser`
- `github.com/peterbourgon/ff/v4` (config: flags → env → file)
- `github.com/prometheus/client_golang` (metrics)
- `github.com/rs/cors`
