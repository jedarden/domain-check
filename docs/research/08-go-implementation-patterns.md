# Go Implementation Patterns

## Recommended Dependencies

| Concern | Package | Import Path | Deps |
|---------|---------|-------------|------|
| TLD parsing | x/net publicsuffix | `golang.org/x/net/publicsuffix` | 0 (stdlib-adjacent) |
| IDN/Punycode | x/net idna | `golang.org/x/net/idna` | 0 |
| Worker pool | errgroup + semaphore | `golang.org/x/sync/errgroup`, `golang.org/x/sync/semaphore` | 0 |
| WHOIS fallback | likexian/whois | `github.com/likexian/whois` | Minimal |
| WHOIS parsing | likexian/whois-parser | `github.com/likexian/whois-parser` | Minimal |
| Config | ff/v4 | `github.com/peterbourgon/ff/v4` | 0 (stdlib `flag` only) |
| Logging | slog (stdlib) | `log/slog` | 0 |
| Metrics | prometheus client | `github.com/prometheus/client_golang` | Standard |
| Embed | stdlib | `embed` | 0 |
| Rate limiting | x/time/rate | `golang.org/x/time/rate` | 0 |
| CORS | rs/cors | `github.com/rs/cors` | 0 |

Total external deps: ~6 libraries. Everything else is stdlib.

## Asset Embedding

```go
//go:embed templates/*
var templateFS embed.FS

//go:embed static/*
var staticFS embed.FS

// Parse templates from embedded FS
tmpl, _ := template.ParseFS(templateFS, "templates/*.html")

// Serve static files
staticSub, _ := fs.Sub(staticFS, "static")
http.Handle("/static/", http.StripPrefix("/static/", http.FileServer(http.FS(staticSub))))
```

Key to single-binary deployment. `embed.FS` implements `fs.FS`.

## Configuration (ff/v4 pattern)

```go
fs := flag.NewFlagSet("domain-check", flag.ContinueOnError)
addr := fs.String("addr", ":8080", "listen address")
cacheSize := fs.Int("cache-size", 10000, "LRU cache max entries")
cacheTTL := fs.Duration("cache-ttl-available", 5*time.Minute, "TTL for available results")

ff.Parse(fs, os.Args[1:],
    ff.WithEnvVarPrefix("DOMCHECK"),
    ff.WithConfigFileFlag("config"),
    ff.WithConfigFileParser(ffyaml.Parser),
)
```

Precedence: `--addr` flag > `DOMCHECK_ADDR` env > config file > default.

## HTTP Client Tuning

```go
transport := &http.Transport{
    MaxIdleConns:        100,
    MaxIdleConnsPerHost: 10,    // Per-registry connection reuse
    MaxConnsPerHost:     20,
    IdleConnTimeout:     90 * time.Second,
    DialContext: (&net.Dialer{
        Timeout:   5 * time.Second,
        KeepAlive: 30 * time.Second,
    }).DialContext,
    TLSHandshakeTimeout:   5 * time.Second,
    ResponseHeaderTimeout: 10 * time.Second,
    ForceAttemptHTTP2:      true,
}
```

One `*http.Client` shared across goroutines. Per-request timeouts via `context.WithTimeout`.

## Bulk Execution: errgroup + per-registry semaphores

```go
g, ctx := errgroup.WithContext(ctx)
g.SetLimit(50)  // global concurrency cap

for i, domain := range domains {
    i, domain := i, domain
    registry := registryFor(domain)
    g.Go(func() error {
        if err := limiter.Acquire(ctx, registry); err != nil {
            return err
        }
        defer limiter.Release(registry)
        results[i], _ = checkDomain(ctx, domain)
        return nil
    })
}
```

Per-registry semaphores: Verisign=10, PIR=10, Google=2, default=3.

## Graceful Shutdown

```go
ctx, stop := signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)
defer stop()

srv := &http.Server{Addr: addr, Handler: mux}
go func() { errCh <- srv.ListenAndServe() }()

select {
case err := <-errCh: return err
case <-ctx.Done():
}

shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()
return srv.Shutdown(shutdownCtx)
```

## Request ID (20 lines, zero deps)

```go
func RequestIDMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        id := r.Header.Get("X-Request-Id")
        if id == "" {
            b := make([]byte, 8)
            rand.Read(b)
            id = hex.EncodeToString(b)
        }
        ctx := context.WithValue(r.Context(), requestIDKey, id)
        w.Header().Set("X-Request-Id", id)
        next.ServeHTTP(w, r.WithContext(ctx))
    })
}
```

## SSRF Prevention

Custom dialer that blocks private IPs after DNS resolution:

```go
safeDialer := &net.Dialer{
    Timeout:   5 * time.Second,
    KeepAlive: 30 * time.Second,
    Control: func(network, address string, c syscall.RawConn) error {
        host, _, _ := net.SplitHostPort(address)
        ip := net.ParseIP(host)
        if ip != nil && (ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsPrivate()) {
            return fmt.Errorf("blocked connection to private IP: %s", ip)
        }
        return nil
    },
}
```

The `Control` callback fires after DNS resolution but before connection — defeats DNS rebinding.

Additionally: allowlist RDAP base URLs from the IANA bootstrap file. Reject redirect targets not in the allowlist.

## Metrics

```go
var (
    requestsTotal   = promauto.NewCounterVec(...)   // {method, path, status}
    requestDuration = promauto.NewHistogramVec(...)  // {method, path}
    rdapRequests    = promauto.NewCounterVec(...)    // {registry, status}
    rdapDuration    = promauto.NewHistogramVec(...)  // {registry}
    cacheHits       = promauto.NewCounterVec(...)    // {result: hit|miss}
    activeChecks    = promauto.NewGauge(...)          // in-flight goroutines
)

mux.Handle("/metrics", promhttp.Handler())
```

## Security Headers

```go
w.Header().Set("Content-Security-Policy",
    "default-src 'none'; script-src 'self'; style-src 'self'; "+
    "img-src 'self'; form-action 'self'; base-uri 'self'; frame-ancestors 'none'")
w.Header().Set("X-Content-Type-Options", "nosniff")
w.Header().Set("X-Frame-Options", "DENY")
w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
```

Move inline JS to external file to avoid needing `'unsafe-inline'` or nonces.

## Body Size Limits

```go
r.Body = http.MaxBytesReader(w, r.Body, 64*1024) // 64 KB
```

Plus item count limit: max 50 domains per bulk request.

## Testing

- **Mock RDAP**: `httptest.NewServer` with recorded fixtures in `testdata/rdap/`
- **Table-driven tests**: for domain validation (30+ cases: valid, invalid, edge)
- **Integration tests**: `httptest.NewRecorder` for full middleware chain
- **Fuzz testing**: Go 1.18+ native fuzzing on domain input parsing
- **Load testing**: `hey` for quick checks, `vegeta` for precise rate characterization
