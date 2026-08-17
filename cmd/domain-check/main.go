package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/jedarden/domain-check/internal/bootstrap"
	"github.com/jedarden/domain-check/internal/cache"
	"github.com/jedarden/domain-check/internal/checker"
	"github.com/jedarden/domain-check/internal/cli"
	"github.com/jedarden/domain-check/internal/config"
	"github.com/jedarden/domain-check/internal/httpclient"
	"github.com/jedarden/domain-check/internal/rdap"
	"github.com/jedarden/domain-check/internal/ratelimit"
	"github.com/jedarden/domain-check/internal/server"
	"github.com/jedarden/domain-check/internal/watch"
	"github.com/jedarden/domain-check/internal/whois"
)

// Build information set by goreleaser during release.
var (
	version = "1.78.0-goreleaser-e2e-test-2026-08-11"
	commit  = "unknown"
	date    = "unknown"
)

func main() {
	// Determine subcommand.
	if len(os.Args) < 2 {
		// No subcommand - run server with defaults.
		runServer(os.Args[1:])
		return
	}

	subcommand := os.Args[1]
	switch subcommand {
	case "check":
		runCheck(os.Args[2:])
	case "bulk":
		runBulk(os.Args[2:])
	case "serve":
		runServer(os.Args[2:])
	case "help", "-h", "--help":
		printUsage()
	default:
		// Unknown subcommand - try parsing as server config for backwards compatibility.
		// If it starts with "-", treat as server flags.
		if len(subcommand) > 0 && subcommand[0] == '-' {
			runServer(os.Args[1:])
		} else {
			fmt.Fprintf(os.Stderr, "unknown subcommand: %s\n\n", subcommand)
			printUsage()
			os.Exit(1)
		}
	}
}

func printUsage() {
	fmt.Fprint(os.Stderr, `domain-check - Authoritative domain availability checker

Usage:
  domain-check [serve] [flags]     Start the HTTP server (default)
  domain-check check <domain> [flags]  Check domain availability
  domain-check bulk <file> [flags]     Bulk check domains from file

Serve flags:
  --addr string           HTTP listen address (default ":8080")
  --config string         Path to YAML config file
  --cache-size int        LRU cache max entries (default 10000)
  --cache-ttl-available   TTL for available domain results (default 5m)
  --cache-ttl-registered  TTL for registered domain results (default 1h)
  --bootstrap-refresh     IANA RDAP bootstrap refresh interval (default 24h)
  --trust-proxy           Trust X-Forwarded-For headers
  --cors-origins string   Allowed CORS origins (default "*")
  --metrics               Enable /metrics Prometheus endpoint (default true)
  --log-format string     Log output format: json or text (default "json")
  --log-level string      Minimum log level: debug, info, warn, error (default "info")
  --enable-watch          Enable watch feature (webhook notifications for domain availability changes)
  --watch-db-path string  Path to watch database (default "data/watches.db")
  --watch-poll-interval   Poll interval for watched domains (default 15m)
  --watch-max-ttl         Maximum TTL for a watch (default 2160h = 90 days)
  --watch-max-per-ip      Maximum watches per IP per 24h (default 10)

Check flags:
  <domain>                Domain name to check
  --tlds string           Comma-separated list of TLDs to expand (e.g., "com,org,dev")
  --format string         Output format: text, json, csv (default "text")
  --timeout duration      HTTP timeout for RDAP queries (default 30s)

Bulk flags:
  <file>                  Path to file containing domains (one per line)
  --concurrency int       Number of concurrent checks (default 20)
  --format string         Output format: text, json, csv (default "text")
  --timeout duration      HTTP timeout for RDAP queries (default 30s)
  --progress              Show progress indicator

Exit codes (check/bulk):
  0  All checked domains are available
  1  At least one domain is taken/registered
  2  Error occurred

Examples:
  domain-check serve --addr :3000
  domain-check serve --enable-watch
  domain-check check example.com
  domain-check check example --tlds com,org,dev --format json
  domain-check bulk domains.txt --concurrency 30 --format csv
  domain-check bulk domains.txt --progress
`)
}

// runCheck executes the check subcommand.
func runCheck(args []string) {
	// Default configuration.
	cfg := cli.CheckConfig{
		Format:    "text",
		Timeout:   30 * time.Second,
		UserAgent: "domain-check/1.0",
	}

	var domain string
	var tldsStr string

	// Simple flag parsing.
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--tlds":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --tlds requires a value")
				os.Exit(2)
			}
			tldsStr = args[i+1]
			i++
		case "--format":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --format requires a value")
				os.Exit(2)
			}
			cfg.Format = args[i+1]
			i++
		case "--timeout":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --timeout requires a value")
				os.Exit(2)
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: invalid timeout: %v\n", err)
				os.Exit(2)
			}
			cfg.Timeout = d
			i++
		case "-h", "--help":
			printUsage()
			os.Exit(0)
		default:
			if len(arg) > 0 && arg[0] != '-' {
				if domain == "" {
					domain = arg
				} else {
					fmt.Fprintf(os.Stderr, "error: unexpected argument: %s\n", arg)
					os.Exit(2)
				}
			} else {
				fmt.Fprintf(os.Stderr, "error: unknown flag: %s\n", arg)
				os.Exit(2)
			}
		}
	}

	if domain == "" {
		fmt.Fprintln(os.Stderr, "error: domain argument is required")
		fmt.Fprintln(os.Stderr, "Usage: domain-check check <domain> [--tlds com,org,dev] [--format text|json|csv]")
		os.Exit(2)
	}

	cfg.Domain = domain
	if tldsStr != "" {
		cfg.TLDs = splitAndTrim(tldsStr)
	}

	// Validate format.
	switch cfg.Format {
	case "text", "json", "csv":
		// Valid.
	default:
		fmt.Fprintf(os.Stderr, "error: invalid format: %s (must be text, json, or csv)\n", cfg.Format)
		os.Exit(2)
	}

	exitCode := cli.Check(context.Background(), cfg)
	os.Exit(exitCode)
}

// runBulk executes the bulk subcommand.
func runBulk(args []string) {
	// Default configuration.
	cfg := cli.BulkConfig{
		Format:      "text",
		Concurrency: 20,
		Timeout:     30 * time.Second,
		UserAgent:   "domain-check/1.0",
		ShowProgress: false,
	}

	var file string

	// Simple flag parsing.
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--concurrency":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --concurrency requires a value")
				os.Exit(2)
			}
			var err error
			cfg.Concurrency, err = parseInt(args[i+1])
			if err != nil || cfg.Concurrency < 1 {
				fmt.Fprintf(os.Stderr, "error: invalid concurrency: must be a positive integer\n")
				os.Exit(2)
			}
			i++
		case "--format":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --format requires a value")
				os.Exit(2)
			}
			cfg.Format = args[i+1]
			i++
		case "--timeout":
			if i+1 >= len(args) {
				fmt.Fprintln(os.Stderr, "error: --timeout requires a value")
				os.Exit(2)
			}
			d, err := time.ParseDuration(args[i+1])
			if err != nil {
				fmt.Fprintf(os.Stderr, "error: invalid timeout: %v\n", err)
				os.Exit(2)
			}
			cfg.Timeout = d
			i++
		case "--progress":
			cfg.ShowProgress = true
		case "-h", "--help":
			printUsage()
			os.Exit(0)
		default:
			if len(arg) > 0 && arg[0] != '-' {
				if file == "" {
					file = arg
				} else {
					fmt.Fprintf(os.Stderr, "error: unexpected argument: %s\n", arg)
					os.Exit(2)
				}
			} else {
				fmt.Fprintf(os.Stderr, "error: unknown flag: %s\n", arg)
				os.Exit(2)
			}
		}
	}

	if file == "" {
		fmt.Fprintln(os.Stderr, "error: file argument is required")
		fmt.Fprintln(os.Stderr, "Usage: domain-check bulk <file> [--concurrency 20] [--format text|json|csv] [--progress]")
		os.Exit(2)
	}

	cfg.File = file

	// Validate format.
	switch cfg.Format {
	case "text", "json", "csv":
		// Valid.
	default:
		fmt.Fprintf(os.Stderr, "error: invalid format: %s (must be text, json, or csv)\n", cfg.Format)
		os.Exit(2)
	}

	exitCode := cli.Bulk(context.Background(), cfg)
	os.Exit(exitCode)
}

// parseInt parses a string to an int.
func parseInt(s string) (int, error) {
	var result int
	var negative bool
	i := 0

	if len(s) > 0 && s[0] == '-' {
		negative = true
		i = 1
	}

	for ; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return 0, fmt.Errorf("invalid integer")
		}
		result = result*10 + int(s[i]-'0')
	}

	if negative {
		result = -result
	}
	return result, nil
}

// splitAndTrim splits a comma-separated string and trims whitespace from each element.
func splitAndTrim(s string) []string {
	parts := splitString(s, ',')
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		p = trimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// splitString splits a string by the given separator.
func splitString(s string, sep rune) []string {
	var result []string
	var current string
	for _, r := range s {
		if r == sep {
			result = append(result, current)
			current = ""
		} else {
			current += string(r)
		}
	}
	result = append(result, current)
	return result
}

// trimSpace removes leading and trailing whitespace.
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && isSpace(rune(s[start])) {
		start++
	}
	for end > start && isSpace(rune(s[end-1])) {
		end--
	}
	return s[start:end]
}

// isSpace reports whether r is a whitespace character.
func isSpace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

// runServer starts the HTTP server.
func runServer(args []string) {
	cfg, err := config.Load(args)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger.
	log := server.DefaultLogger(cfg.LogFormat, cfg.LogLevel)

	// Create rate limiter for server (per-IP rate limiting).
	rateLimiter := server.NewRateLimiter(log)

	// Start periodic cleanup of stale IP entries (every 10 minutes).
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rateLimiter.Cleanup()
		}
	}()

	// Initialize Prometheus metrics.
	metrics := server.GetMetrics()

	// Initialize the domain checker with all its dependencies.
	ctx := context.Background()
	domainChecker, bootstrap, err := setupDomainChecker(ctx, cfg, log, metrics)
	if err != nil {
		log.Error("failed to initialize domain checker", "error", err)
		os.Exit(1)
	}

	// Create service monitor for uptime and check counting.
	monitor := server.NewServiceMonitor()

	// Initialize watch manager if enabled.
	var watchHandler server.WatchHandlerInterface
	var watchManager *watch.Manager
	if cfg.EnableWatch() {
		watchManager, err = setupWatchManager(ctx, cfg, domainChecker, log)
		if err != nil {
			log.Error("failed to initialize watch manager", "error", err)
			os.Exit(1)
		}

		// Start the watch manager in background.
		if err := watchManager.Start(ctx); err != nil {
			log.Error("failed to start watch manager", "error", err)
			os.Exit(1)
		}

		watchHandler = server.NewWatchHandler(watchManager, log)
		log.Info("watch feature enabled", "db_path", cfg.WatchDBPath, "poll_interval", cfg.WatchPollInterval)
	}

	// Create router with all routes and middleware.
	handler := server.Router(cfg, log, rateLimiter, domainChecker, bootstrap, monitor, metrics, watchHandler)

	// Start periodic metrics update (bootstrap age every minute).
	go func() {
		ticker := time.NewTicker(1 * time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if bootstrap != nil {
					age := time.Since(bootstrap.Updated())
					metrics.SetBootstrapAge(age.Seconds())
				}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Create and run the HTTP server.
	srv := server.New(cfg, handler, log)

	// Defer watch manager cleanup
	if watchManager != nil {
		defer watchManager.Stop()
	}

	if err := srv.Run(ctx); err != nil {
		log.Error("server error", "error", err)
		os.Exit(1)
	}
}

// setupDomainChecker creates and initializes a fully configured domain checker.
// It returns the checker, bootstrap manager, and any error.
func setupDomainChecker(ctx context.Context, cfg *config.Config, log *slog.Logger, metrics *server.Metrics) (*checker.Checker, *bootstrap.Manager, error) {
	// Create bootstrap manager for IANA RDAP bootstrap.
	bs, err := bootstrap.NewManager(ctx, "")
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create bootstrap manager: %w", err)
	}

	// Create allowlist for RDAP servers (populated from bootstrap).
	// Get the current bootstrap URLs to seed the allowlist.
	bootstrapURLs := bs.URLs()
	allowlist := httpclient.NewAllowList(bootstrapURLs)

	// Create safe HTTP client with SSRF protection and per-registry connection pools.
	safeClient := httpclient.NewSafeClient(httpclient.ClientConfig{
		AllowList: allowlist,
		UserAgent: "domain-check/1.0",
		Transport: httpclient.NewPerRegistryRoundTripper(),
	})

	// Start background bootstrap refresh and allowlist update.
	go func() {
		ticker := time.NewTicker(cfg.BootstrapRefresh)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				if err := bs.Refresh(ctx); err != nil {
					log.Warn("bootstrap refresh failed", "error", err)
					continue
				}
				// Update allowlist with new URLs.
				urls := bs.URLs()
				for _, u := range urls {
					allowlist.Allow(u)
				}
				log.Debug("bootstrap refresh completed", "urls", len(urls))
			case <-ctx.Done():
				return
			}
		}
	}()

	// Create per-registry rate limiter.
	registryRateLimit := ratelimit.NewRateLimiter()

	// Create RDAP client.
	rdapClient := rdap.NewRDAPClient(rdap.RDAPClientConfig{
		HTTPClient: safeClient,
		Bootstrap: bs,
		RateLimit:  registryRateLimit,
		AllowList:  allowlist,
		UserAgent:  "domain-check/1.0",
		Metrics:    metrics,
	})

	// Create WHOIS client for ccTLD fallback.
	whoisClient := whois.NewWHOISClient(whois.WHOISClientConfig{
		UserAgent: "domain-check/1.0",
	})

	// Create DNS pre-filter (optional optimization).
	dnsPreFilter := checker.NewDNSPreFilter()

	// Create result cache with configured TTLs.
	resultCache := cache.NewResultCache(cache.CacheTTLs{
		Available:  cfg.CacheTTLAvailable,
		Registered: cfg.CacheTTLRegistered,
		Error:      30 * time.Second,
	}, cfg.CacheSize, metrics)

	// Create the main checker with all components.
	domainChecker := checker.NewChecker(checker.CheckerConfig{
		RDAPClient:      rdapClient,
		WHOISClient:     whoisClient,
		DNSPreFilter:    dnsPreFilter,
		Cache:           resultCache,
		Bootstrap: bs,
		UseDNSPrefilter: false, // Disabled by default - can be enabled via config
		BulkConfig:      checker.DefaultBulkCheckConfig(),
		ActiveMetrics:   metrics,
	})

	return domainChecker, bs, nil
}

// setupWatchManager creates and initializes a fully configured watch manager.
func setupWatchManager(ctx context.Context, cfg *config.Config, domainChecker *checker.Checker, log *slog.Logger) (*watch.Manager, error) {
	// Create watch store.
	store, err := watch.NewStore(cfg.WatchDBPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create watch store: %w", err)
	}

	// Create webhook client.
	webhookClient := watch.NewWebhookClient()

	// Create watch manager configuration.
	watchCfg := watch.ManagerConfig{
		Store:           store,
		WebhookClient:   webhookClient,
		Checker:         domainChecker,
		PollInterval:    cfg.WatchPollInterval,
		MaxTTL:          cfg.WatchMaxTTL,
		MaxWatchesPerIP: cfg.WatchMaxPerIP,
		Logger:          log,
	}

	// Create watch manager.
	watchManager := watch.NewManager(watchCfg)

	return watchManager, nil
}
