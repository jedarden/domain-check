// Package config provides layered configuration loading for domain-check.
// Priority order: CLI flags > environment variables > config file > defaults.
package config

import (
	"fmt"
	"time"

	"github.com/peterbourgon/ff/v4"
	"github.com/peterbourgon/ff/v4/ffyaml"
)

// Config holds all runtime configuration for domain-check.
type Config struct {
	// Addr is the HTTP listen address (default ":8080").
	Addr string

	// ConfigFile is the path to a YAML config file.
	ConfigFile string

	// CacheSize is the maximum number of entries in the LRU cache (default 10000).
	CacheSize int

	// CacheTTLAvailable is the TTL for "available" domain results (default 5m).
	CacheTTLAvailable time.Duration

	// CacheTTLRegistered is the TTL for "registered" domain results (default 1h).
	CacheTTLRegistered time.Duration

	// BootstrapRefresh is the IANA RDAP bootstrap refresh interval (default 24h).
	BootstrapRefresh time.Duration

	// CorsOrigins is the comma-separated list of allowed CORS origins (default "*").
	CorsOrigins string

	// LogFormat is the log output format: "json" or "text" (default "json").
	LogFormat string

	// LogLevel is the minimum log level: "debug", "info", "warn", or "error" (default "info").
	LogLevel string

	// Watch features
	WatchDBPath       string        // Path to watch database (default "data/watches.db")
	WatchPollInterval time.Duration // Poll interval for watched domains (default 15m)
	WatchMaxTTL       time.Duration // Maximum TTL for a watch (default 2160h = 90 days)
	WatchMaxPerIP     int           // Maximum watches per IP per 24h (default 10)

	// Boolean flags (parsed from strings)
	TrustProxyVal string // Internal string value for TrustProxy
	MetricsVal     string // Internal string value for Metrics
	EnableWatchVal string // Internal string value for EnableWatch
}

// TrustProxy returns the boolean value for TrustProxy.
func (c *Config) TrustProxy() bool {
	return c.TrustProxyVal == "true"
}

// Metrics returns the boolean value for Metrics.
func (c *Config) Metrics() bool {
	return c.MetricsVal != "false" // Default is true
}

// EnableWatch returns the boolean value for EnableWatch.
func (c *Config) EnableWatch() bool {
	return c.EnableWatchVal == "true"
}

// Defaults returns a Config populated with default values.
func Defaults() Config {
	return Config{
		Addr:               ":8080",
		CacheSize:          10000,
		CacheTTLAvailable:  5 * time.Minute,
		CacheTTLRegistered: 1 * time.Hour,
		BootstrapRefresh:   24 * time.Hour,
		CorsOrigins:        "*",
		LogFormat:          "json",
		LogLevel:           "info",
		WatchDBPath:        "data/watches.db",
		WatchPollInterval:  15 * time.Minute,
		WatchMaxTTL:        90 * 24 * time.Hour, // 90 days
		WatchMaxPerIP:      10,
		// Boolean defaults (as strings)
		TrustProxyVal:  "false",
		MetricsVal:     "true", // Default is true
		EnableWatchVal: "false",
	}
}

// Load parses configuration from the provided args (typically os.Args[1:])
// using the layered priority: CLI flags > env vars (DOMCHECK_ prefix) >
// YAML config file > defaults.
func Load(args []string) (*Config, error) {
	cfg := Defaults()

	// Pre-process boolean flags to support both --flag and --flag=value syntax
	args = normalizeBoolArgs(args, "trust-proxy", "metrics", "enable-watch")

	fs := ff.NewFlagSet("domain-check")
	fs.StringVar(&cfg.Addr, 0, "addr", cfg.Addr, "HTTP listen address")
	fs.StringVar(&cfg.ConfigFile, 0, "config", "", "path to YAML config file")
	fs.IntVar(&cfg.CacheSize, 0, "cache-size", cfg.CacheSize, "LRU cache max entries")
	fs.DurationVar(&cfg.CacheTTLAvailable, 0, "cache-ttl-available", cfg.CacheTTLAvailable, "TTL for available domain results")
	fs.DurationVar(&cfg.CacheTTLRegistered, 0, "cache-ttl-registered", cfg.CacheTTLRegistered, "TTL for registered domain results")
	fs.DurationVar(&cfg.BootstrapRefresh, 0, "bootstrap-refresh", cfg.BootstrapRefresh, "IANA RDAP bootstrap refresh interval")

	// Boolean flags using StringVar with manual parsing to support true/false defaults
	// ff's BoolVar always defaults to false, so we need custom handling for true defaults
	fs.StringVar(&cfg.TrustProxyVal, 0, "trust-proxy", cfg.TrustProxyVal, "trust X-Forwarded-For headers")
	fs.StringVar(&cfg.CorsOrigins, 0, "cors-origins", cfg.CorsOrigins, "allowed CORS origins (comma-separated)")
	fs.StringVar(&cfg.MetricsVal, 0, "metrics", cfg.MetricsVal, "enable /metrics Prometheus endpoint")
	fs.StringEnumVar(&cfg.LogFormat, 0, "log-format", "log output format (json or text)", "json", "text")
	fs.StringEnumVar(&cfg.LogLevel, 0, "log-level", "minimum log level", "info", "debug", "warn", "error")

	// Watch feature flags (all in the same flagset)
	fs.StringVar(&cfg.EnableWatchVal, 0, "enable-watch", cfg.EnableWatchVal, "enable watch feature (webhook notifications)")
	fs.StringVar(&cfg.WatchDBPath, 0, "watch-db-path", cfg.WatchDBPath, "path to watch database")
	fs.DurationVar(&cfg.WatchPollInterval, 0, "watch-poll-interval", cfg.WatchPollInterval, "poll interval for watched domains")
	fs.DurationVar(&cfg.WatchMaxTTL, 0, "watch-max-ttl", cfg.WatchMaxTTL, "maximum TTL for a watch")
	fs.IntVar(&cfg.WatchMaxPerIP, 0, "watch-max-per-ip", cfg.WatchMaxPerIP, "maximum watches per IP per 24h")

	if err := ff.Parse(fs, args,
		ff.WithEnvVarPrefix("DOMCHECK"),
		ff.WithConfigFileFlag("config"),
		ff.WithConfigFileParser(ffyaml.Parse),
		ff.WithConfigAllowMissingFile(),
	); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}

	return &cfg, nil
}

// normalizeBoolArgs converts boolean flag shorthands to --flag=value format.
// --flag becomes --flag=true, --no-flag becomes --flag=false.
// This allows StringVar to be used for boolean flags while supporting
// both the long form (--flag=true) and the shorthand (--flag).
func normalizeBoolArgs(args []string, boolFlags ...string) []string {
	result := make([]string, 0, len(args))

	for _, arg := range args {
		matched := false

		for _, flag := range boolFlags {
			// Check for --flag (convert to --flag=true)
			if arg == "--"+flag {
				result = append(result, "--"+flag+"=true")
				matched = true
				break
			}
			// Check for --no-flag (convert to --flag=false)
			if arg == "--no-"+flag {
				result = append(result, "--"+flag+"=false")
				matched = true
				break
			}
		}

		if !matched {
			result = append(result, arg)
		}
	}

	return result
}
