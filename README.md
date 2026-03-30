# Domain Check

Authoritative domain availability checker powered by [RDAP](https://about.rdap.org/) — the ICANN-mandated successor to WHOIS.

## Features

- **Authoritative** — queries registry RDAP servers directly (Verisign, PIR, Google, etc.)
- **Web UI** — clean, fast, no-signup interface for humans
- **REST API** — JSON API for programmatic access
- **CLI** — check domains from the terminal
- **Bulk checking** — check up to 50 domains in a single request
- **Multi-TLD** — check a name across multiple TLDs at once
- **Self-hostable** — single Go binary or Docker container
- **Zero tracking** — no analytics, no cookies, no data retention

## Why RDAP over WHOIS?

| | RDAP | WHOIS |
|---|---|---|
| Protocol | HTTPS (JSON) | TCP port 43 (plaintext) |
| Accuracy | Definitive | Unreliable (missed registered domains in testing) |
| .dev/.app support | Yes | No (Google disabled port 43) |
| Response format | Structured JSON | Unstructured text (varies by registry) |
| Parsing | Trivial | Fragile regex per-registry |

In testing, `whois` reported a registered domain as available. RDAP was 100% accurate across all test cases. See [docs/research/05-accuracy-comparison.md](docs/research/05-accuracy-comparison.md).

## Quick Start

### Binary (Linux/macOS)

Download the latest release, then run:

```bash
./domain-check serve
# Listening on :8080
```

Open `http://localhost:8080` in your browser.

### Docker

```bash
docker run -p 8080:8080 ghcr.io/jedarden/domain-check
```

### From Source

Requires [Go 1.23+](https://go.dev/dl/).

```bash
git clone https://github.com/jedarden/domain-check.git
cd domain-check
go build -o domain-check ./cmd/domain-check
./domain-check serve
```

## API Usage

All API endpoints are prefixed with `/api/v1/`.

### Check a Single Domain

```bash
curl -s 'http://localhost:8080/api/v1/check?d=example.com' | jq
```

```json
{
  "domain": "example.com",
  "available": true,
  "tld": "com",
  "checked_at": "2026-03-29T14:30:00Z",
  "source": "rdap",
  "cached": false,
  "duration_ms": 112
}
```

For registered domains, the response includes registration details:

```json
{
  "domain": "google.com",
  "available": false,
  "tld": "com",
  "checked_at": "2026-03-29T14:30:00Z",
  "source": "rdap",
  "cached": false,
  "duration_ms": 108,
  "registration": {
    "registrar": "MarkMonitor Inc.",
    "created": "1997-09-15T04:00:00Z",
    "expires": "2028-09-14T04:00:00Z",
    "nameservers": ["ns1.google.com", "ns2.google.com"],
    "status": ["client delete prohibited", "client transfer prohibited"]
  }
}
```

### Multi-TLD Check

Check a name across multiple TLDs in one request:

```bash
curl -s 'http://localhost:8080/api/v1/check?d=example&tlds=com,org,dev,net,io' | jq
```

```json
{
  "name": "example",
  "total": 5,
  "succeeded": 5,
  "failed": 0,
  "duration": "234ms",
  "results": [
    { "domain": "example.com", "tld": "com", "result": { "available": false, "registration": { "registrar": "..." } } },
    { "domain": "example.org", "tld": "org", "result": { "available": false, "registration": { "registrar": "..." } } },
    { "domain": "example.dev", "tld": "dev", "result": { "available": true } },
    { "domain": "example.net", "tld": "net", "result": { "available": false, "registration": { "registrar": "..." } } },
    { "domain": "example.io", "tld": "io", "result": { "available": true } }
  ]
}
```

### Bulk Check

Check up to 50 domains in a single POST request (max 64 KB body):

```bash
curl -s -X POST 'http://localhost:8080/api/v1/bulk' \
  -H 'Content-Type: application/json' \
  -d '{"domains":["example.com","example.org","example.dev"]}' | jq
```

```json
{
  "total": 3,
  "succeeded": 3,
  "failed": 0,
  "duration": "342ms",
  "results": [
    { "domain": "example.com", "result": { "available": false, "registration": { "registrar": "..." } } },
    { "domain": "example.org", "result": { "available": false, "registration": { "registrar": "..." } } },
    { "domain": "example.dev", "result": { "available": true } }
  ]
}
```

### Health Check

```bash
curl -s http://localhost:8080/health
```

```json
{"status":"ok"}
```

### Rate Limits

| Endpoint | Limit |
|----------|-------|
| Web UI (`/check`) | 10 req/min per IP |
| API (`/api/v1/check`) | 60 req/min per IP |
| Bulk (`/api/v1/bulk`) | 5 req/min per IP |

Rate-limited responses return `429 Too Many Requests` with a `Retry-After` header.

## CLI Usage

### Check a Single Domain

```bash
domain-check check example.com
```

Check across multiple TLDs:

```bash
domain-check check example --tlds com,org,dev
```

JSON output:

```bash
domain-check check example.com --format json
```

### Bulk Check from File

Create a file with one domain per line:

```
# domains.txt
example.com
example.org
example.dev
```

Run the bulk check:

```bash
domain-check bulk domains.txt
```

With progress indicator and CSV output:

```bash
domain-check bulk domains.txt --progress --format csv
```

Adjust concurrency (default: 20):

```bash
domain-check bulk domains.txt --concurrency 30
```

### CLI Exit Codes

| Code | Meaning |
|------|---------|
| 0 | All checked domains are available |
| 1 | At least one domain is taken |
| 2 | Error occurred |

## Self-Hosting

### Binary

```bash
# Download and run
./domain-check serve --addr :3000

# Run behind a reverse proxy
./domain-check serve --trust-proxy --addr :8080
```

### Docker

```bash
# Build from source
docker build -t domain-check .

# Run
docker run -p 8080:8080 domain-check

# With environment variables
docker run -p 8080:8080 \
  -e DOMCHECK_CACHE_SIZE=50000 \
  -e DOMCHECK_TRUST_PROXY=true \
  domain-check
```

### Reverse Proxy (Caddy)

```
domain-check.example.com {
    reverse_proxy localhost:8080
}
```

### Reverse Proxy (nginx)

```nginx
server {
    listen 443 ssl;
    server_name domain-check.example.com;

    location / {
        proxy_pass http://127.0.0.1:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

## Configuration

All settings can be configured via CLI flags, environment variables (prefixed with `DOMCHECK_`), or a YAML config file. Priority: flags > env vars > config file > defaults.

### Config File

Create a `config.yaml`:

```yaml
addr: ":3000"
cache-size: 50000
cache-ttl-available: 10m
cache-ttl-registered: 2h
trust-proxy: true
cors-origins: "https://example.com"
log-format: text
log-level: info
```

Load it with `--config config.yaml` or `DOMCHECK_CONFIG=config.yaml`.

### All Parameters

| Flag | Env Variable | Default | Description |
|------|-------------|---------|-------------|
| `--addr` | `DOMCHECK_ADDR` | `:8080` | HTTP listen address |
| `--config` | `DOMCHECK_CONFIG` | — | Path to YAML config file |
| `--cache-size` | `DOMCHECK_CACHE_SIZE` | `10000` | LRU cache max entries |
| `--cache-ttl-available` | `DOMCHECK_CACHE_TTL_AVAILABLE` | `5m` | TTL for available domain results |
| `--cache-ttl-registered` | `DOMCHECK_CACHE_TTL_REGISTERED` | `1h` | TTL for registered domain results |
| `--bootstrap-refresh` | `DOMCHECK_BOOTSTRAP_REFRESH` | `24h` | IANA RDAP bootstrap refresh interval |
| `--trust-proxy` | `DOMCHECK_TRUST_PROXY` | `false` | Trust `X-Forwarded-For` for client IP |
| `--cors-origins` | `DOMCHECK_CORS_ORIGINS` | `*` | Allowed CORS origins (comma-separated) |
| `--metrics` | `DOMCHECK_METRICS` | `true` | Enable `/metrics` Prometheus endpoint |
| `--log-format` | `DOMCHECK_LOG_FORMAT` | `json` | Log format: `json` or `text` |
| `--log-level` | `DOMCHECK_LOG_LEVEL` | `info` | Log level: `debug`, `info`, `warn`, `error` |

## Architecture

Domain Check is a single Go binary with zero runtime dependencies. Templates and static assets are embedded via `go:embed`.

```
cmd/domain-check/main.go          # Entry point
internal/
  config/      # Layered config: flags → env → YAML → defaults
  domain/      # Input validation, IDN, TLD extraction via publicsuffix
  bootstrap/   # IANA RDAP bootstrap loader with 24h cache refresh
  rdap/        # RDAP client, response parser, per-registry rate limiting
  whois/       # WHOIS fallback for ccTLDs without RDAP
  cache/       # In-memory bounded LRU with per-status TTLs
  httpclient/  # SSRF-safe HTTP client (private IP blocking)
  ratelimit/   # Per-IP rate limiter middleware
  server/      # HTTP server, router, middleware, API handlers
  web/         # HTML templates, static assets, template handlers
  cli/         # CLI subcommands (check, bulk)
web/
  templates/   # HTML templates (layout, index, result)
  static/      # CSS, JS, favicon (embedded)
```

### Request Flow

1. **Validate** — normalize input, convert IDN to punycode, extract TLD via publicsuffix
2. **Cache check** — return cached result if fresh
3. **Rate limit** — enforce per-registry limits (Verisign 10/s, Google 1/s)
4. **RDAP query** — query the authoritative registry server over HTTPS
5. **Parse** — extract availability, registration details, nameservers
6. **Cache store** — cache result with per-status TTL (5min available, 1h registered)
7. **Respond** — return structured JSON

### Caching

| Result Type | TTL | Rationale |
|-------------|-----|-----------|
| Available | 5 min | Availability can change quickly |
| Registered | 1 hour | Registered domains rarely become available |
| Error | 30 sec | Retry soon on transient failures |

### Security

- **SSRF protection** — private IP blocking, URL allowlist, redirect validation
- **Security headers** — CSP, X-Frame-Options, X-Content-Type-Options, Referrer-Policy
- **Non-root Docker** — runs as `appuser` (UID 1000)
- **No tracking** — no analytics, no cookies, no data retention

## Development

```bash
# Build
go build -o domain-check ./cmd/domain-check

# Run tests
go test ./...

# Run tests with verbose output
go test -v ./internal/domain/

# Fuzz testing
go test -fuzz=. -fuzztime=30s ./internal/domain/

# Lint
golangci-lint run
```

## Documentation

- [docs/research/](docs/research/) — Research on domain checking methods, RDAP protocol, rate limits, accuracy testing
- [docs/plan/plan.md](docs/plan/plan.md) — Complete architecture plan

## Contributing

1. Fork the repository
2. Create a feature branch: `git checkout -b my-feature`
3. Make your changes
4. Run tests: `go test ./...`
5. Lint: `golangci-lint run`
6. Commit with a descriptive message
7. Push and open a pull request

Please ensure all tests pass and the code is properly formatted before submitting.

## License

MIT
