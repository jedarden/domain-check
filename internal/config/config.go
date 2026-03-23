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

	// TrustProxy enables trusting X-Forwarded-For headers for client IP (default false).
	TrustProxy bool

	// CorsOrigins is the comma-separated list of allowed CORS origins (default "*").
	CorsOrigins string

	// Metrics enables the /metrics Prometheus endpoint (default true).
	Metrics bool

	// LogFormat is the log output format: "json" or "text" (default "json").
	LogFormat string

	// LogLevel is the minimum log level: "debug", "info", "warn", or "error" (default "info").
	LogLevel string
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
		Metrics:            true,
		LogFormat:          "json",
		LogLevel:           "info",
	}
}

// Load parses configuration from the provided args (typically os.Args[1:])
// using the layered priority: CLI flags > env vars (DOMCHECK_ prefix) >
// YAML config file > defaults.
func Load(args []string) (*Config, error) {
	cfg := Defaults()

	fs := ff.NewFlagSet("domain-check")
	fs.StringVar(&cfg.Addr, 0, "addr", cfg.Addr, "HTTP listen address")
	fs.StringVar(&cfg.ConfigFile, 0, "config", "", "path to YAML config file")
	fs.IntVar(&cfg.CacheSize, 0, "cache-size", cfg.CacheSize, "LRU cache max entries")
	fs.DurationVar(&cfg.CacheTTLAvailable, 0, "cache-ttl-available", cfg.CacheTTLAvailable, "TTL for available domain results")
	fs.DurationVar(&cfg.CacheTTLRegistered, 0, "cache-ttl-registered", cfg.CacheTTLRegistered, "TTL for registered domain results")
	fs.DurationVar(&cfg.BootstrapRefresh, 0, "bootstrap-refresh", cfg.BootstrapRefresh, "IANA RDAP bootstrap refresh interval")
	fs.BoolVar(&cfg.TrustProxy, 0, "trust-proxy", "trust X-Forwarded-For headers")
	fs.StringVar(&cfg.CorsOrigins, 0, "cors-origins", cfg.CorsOrigins, "allowed CORS origins (comma-separated)")
	fs.BoolVarDefault(&cfg.Metrics, 0, "metrics", true, "enable /metrics Prometheus endpoint")
	fs.StringEnumVar(&cfg.LogFormat, 0, "log-format", "log output format (json or text)", "json", "text")
	fs.StringEnumVar(&cfg.LogLevel, 0, "log-level", "minimum log level", "info", "debug", "warn", "error")

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
