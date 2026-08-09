package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaults(t *testing.T) {
	cfg := Defaults()

	assert.Equal(t, ":8080", cfg.Addr)
	assert.Empty(t, cfg.ConfigFile)
	assert.Equal(t, 10000, cfg.CacheSize)
	assert.Equal(t, 5*time.Minute, cfg.CacheTTLAvailable)
	assert.Equal(t, 1*time.Hour, cfg.CacheTTLRegistered)
	assert.Equal(t, 24*time.Hour, cfg.BootstrapRefresh)
	assert.False(t, cfg.TrustProxy)
	assert.Equal(t, "*", cfg.CorsOrigins)
	assert.True(t, cfg.Metrics)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "info", cfg.LogLevel)
}

func TestLoad_Defaults(t *testing.T) {
	cfg, err := Load([]string{})
	require.NoError(t, err)

	assert.Equal(t, ":8080", cfg.Addr)
	assert.Empty(t, cfg.ConfigFile)
	assert.Equal(t, 10000, cfg.CacheSize)
	assert.Equal(t, 5*time.Minute, cfg.CacheTTLAvailable)
	assert.Equal(t, 1*time.Hour, cfg.CacheTTLRegistered)
	assert.Equal(t, 24*time.Hour, cfg.BootstrapRefresh)
	assert.False(t, cfg.TrustProxy)
	assert.Equal(t, "*", cfg.CorsOrigins)
	assert.True(t, cfg.Metrics)
	assert.Equal(t, "json", cfg.LogFormat)
	assert.Equal(t, "info", cfg.LogLevel)
}

func TestLoad_CLIOverrides(t *testing.T) {
	cfg, err := Load([]string{
		"--addr", ":9090",
		"--cache-size", "5000",
		"--cache-ttl-available", "10m",
		"--cache-ttl-registered", "2h",
		"--bootstrap-refresh", "12h",
		"--trust-proxy",
		"--cors-origins", "https://example.com",
		"--metrics=false",
		"--log-format", "text",
		"--log-level", "debug",
	})
	require.NoError(t, err)

	assert.Equal(t, ":9090", cfg.Addr)
	assert.Equal(t, 5000, cfg.CacheSize)
	assert.Equal(t, 10*time.Minute, cfg.CacheTTLAvailable)
	assert.Equal(t, 2*time.Hour, cfg.CacheTTLRegistered)
	assert.Equal(t, 12*time.Hour, cfg.BootstrapRefresh)
	assert.True(t, cfg.TrustProxy)
	assert.Equal(t, "https://example.com", cfg.CorsOrigins)
	assert.False(t, cfg.Metrics)
	assert.Equal(t, "text", cfg.LogFormat)
	assert.Equal(t, "debug", cfg.LogLevel)
}

func TestLoad_EnvVarOverrides(t *testing.T) {
	t.Setenv("DOMCHECK_ADDR", ":7070")
	t.Setenv("DOMCHECK_CACHE_SIZE", "2000")
	t.Setenv("DOMCHECK_TRUST_PROXY", "true")
	t.Setenv("DOMCHECK_LOG_FORMAT", "text")
	t.Setenv("DOMCHECK_LOG_LEVEL", "warn")

	cfg, err := Load([]string{})
	require.NoError(t, err)

	assert.Equal(t, ":7070", cfg.Addr)
	assert.Equal(t, 2000, cfg.CacheSize)
	assert.True(t, cfg.TrustProxy)
	assert.Equal(t, "text", cfg.LogFormat)
	assert.Equal(t, "warn", cfg.LogLevel)
}

func TestLoad_CLIOverridesEnv(t *testing.T) {
	t.Setenv("DOMCHECK_ADDR", ":7070")
	t.Setenv("DOMCHECK_LOG_LEVEL", "debug")

	cfg, err := Load([]string{"--addr", ":9090"})
	require.NoError(t, err)

	assert.Equal(t, ":9090", cfg.Addr, "CLI flag should override env var")
	assert.Equal(t, "debug", cfg.LogLevel, "env var should still apply for unset flags")
}

func TestLoad_ConfigFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`
addr: ":9090"
cache-size: 5000
cache-ttl-available: 10m
cache-ttl-registered: 2h
bootstrap-refresh: 12h
trust-proxy: true
cors-origins: "https://example.com"
metrics: false
log-format: text
log-level: debug
`)
	require.NoError(t, os.WriteFile(configPath, content, 0644))

	cfg, err := Load([]string{"--config", configPath})
	require.NoError(t, err)

	assert.Equal(t, ":9090", cfg.Addr)
	assert.Equal(t, 5000, cfg.CacheSize)
	assert.Equal(t, 10*time.Minute, cfg.CacheTTLAvailable)
	assert.Equal(t, 2*time.Hour, cfg.CacheTTLRegistered)
	assert.Equal(t, 12*time.Hour, cfg.BootstrapRefresh)
	assert.True(t, cfg.TrustProxy)
	assert.Equal(t, "https://example.com", cfg.CorsOrigins)
	assert.False(t, cfg.Metrics)
	assert.Equal(t, "text", cfg.LogFormat)
	assert.Equal(t, "debug", cfg.LogLevel)
}

func TestLoad_ConfigFileOverriddenByEnv(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`
addr: ":9090"
log-level: debug
`)
	require.NoError(t, os.WriteFile(configPath, content, 0644))

	t.Setenv("DOMCHECK_ADDR", ":7070")

	cfg, err := Load([]string{"--config", configPath})
	require.NoError(t, err)

	assert.Equal(t, ":7070", cfg.Addr, "env var should override config file")
	assert.Equal(t, "debug", cfg.LogLevel, "config file should set value when no env var")
}

func TestLoad_MissingConfigFileAllowed(t *testing.T) {
	_, err := Load([]string{"--config", "/nonexistent/config.yaml"})
	require.NoError(t, err, "missing config file should not error")
}

func TestLoad_ConfigFileOverridesDefaults(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`addr: ":9999"`)
	require.NoError(t, os.WriteFile(configPath, content, 0644))

	cfg, err := Load([]string{"--config", configPath})
	require.NoError(t, err)

	assert.Equal(t, ":9999", cfg.Addr)
	assert.Equal(t, 10000, cfg.CacheSize, "unset config values should use defaults")
}

func TestLoad_CLIOverridesConfigFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	content := []byte(`
addr: ":9090"
log-level: debug
`)
	require.NoError(t, os.WriteFile(configPath, content, 0644))

	cfg, err := Load([]string{"--config", configPath, "--addr", ":7070"})
	require.NoError(t, err)

	assert.Equal(t, ":7070", cfg.Addr, "CLI flag should override config file")
	assert.Equal(t, "debug", cfg.LogLevel, "config file should still apply")
}

func TestLoad_InvalidLogFormat(t *testing.T) {
	_, err := Load([]string{"--log-format", "xml"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse config")
}

func TestLoad_InvalidLogLevel(t *testing.T) {
	_, err := Load([]string{"--log-level", "trace"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse config")
}

func TestLoad_InvalidFlag(t *testing.T) {
	_, err := Load([]string{"--bogus-flag", "value"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parse config")
}

func TestLoad_DurationParsing(t *testing.T) {
	cfg, err := Load([]string{
		"--cache-ttl-available", "30s",
		"--cache-ttl-registered", "45m",
		"--bootstrap-refresh", "168h",
	})
	require.NoError(t, err)

	assert.Equal(t, 30*time.Second, cfg.CacheTTLAvailable)
	assert.Equal(t, 45*time.Minute, cfg.CacheTTLRegistered)
	assert.Equal(t, 168*time.Hour, cfg.BootstrapRefresh)
}

func TestLoad_Help(t *testing.T) {
	_, err := Load([]string{"--help"})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "help")
}

func TestLoad_UnknownFlag(t *testing.T) {
	_, err := Load([]string{"--unknown"})
	require.Error(t, err)
}

func TestLoad_BoolFlags(t *testing.T) {
	// Test --trust-proxy sets to true
	cfg, err := Load([]string{"--trust-proxy"})
	require.NoError(t, err)
	assert.True(t, cfg.TrustProxy)

	// Test --metrics=false sets to false
	cfg, err = Load([]string{"--metrics=false"})
	require.NoError(t, err)
	assert.False(t, cfg.Metrics)

	// Test --metrics=true explicit
	cfg, err = Load([]string{"--metrics=true"})
	require.NoError(t, err)
	assert.True(t, cfg.Metrics)
}
