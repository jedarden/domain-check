package main

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/coding/domain-check/internal/cli"
	"github.com/coding/domain-check/internal/config"
	"github.com/coding/domain-check/internal/server"
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

	// Create rate limiter.
	rateLimiter := server.NewRateLimiter(log)

	// Start periodic cleanup of stale IP entries (every 10 minutes).
	go func() {
		ticker := time.NewTicker(10 * time.Minute)
		defer ticker.Stop()
		for range ticker.C {
			rateLimiter.Cleanup()
		}
	}()

	// Create router with all routes and middleware.
	// Note: DomainChecker will be passed when full implementation is ready.
	handler := server.Router(cfg, log, rateLimiter, nil)

	// Create and run the HTTP server.
	srv := server.New(cfg, handler, log)

	if err := srv.Run(context.Background()); err != nil {
		log.Error("server error", "error", err)
		os.Exit(1)
	}
}
