package checker

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestWHOISRateLimiter(t *testing.T) {
	rl := NewWHOISRateLimiter()

	// Test that we can get a limiter for an unknown server.
	lim := rl.getOrCreate("whois.example.com")
	if lim == nil {
		t.Fatal("expected limiter to be created")
	}

	// Test that we get the same limiter for the same server.
	lim2 := rl.getOrCreate("whois.example.com")
	if lim != lim2 {
		t.Error("expected same limiter for same server")
	}

	// Test that a known server has its config.
	denicLim := rl.getOrCreate("whois.denic.de")
	if denicLim == nil {
		t.Fatal("expected DENIC limiter to be created")
	}
	if denicLim.config.MinInterval != 5*time.Second {
		t.Errorf("expected DENIC min interval 5s, got %v", denicLim.config.MinInterval)
	}
}

func TestWHOISRateLimiter_Wait(t *testing.T) {
	rl := NewWHOISRateLimiter()
	ctx := context.Background()

	// Test waiting for an unknown server (should be fast).
	start := time.Now()
	if err := rl.Wait(ctx, "whois.example.com"); err != nil {
		t.Fatalf("wait failed: %v", err)
	}
	elapsed := time.Since(start)
	if elapsed > 100*time.Millisecond {
		t.Errorf("expected fast wait, took %v", elapsed)
	}
}

func TestWHOISRateLimiter_WaitContextCancellation(t *testing.T) {
	rl := NewWHOISRateLimiter()

	// Create a context that's already cancelled.
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := rl.Wait(ctx, "whois.example.com")
	if err == nil {
		t.Error("expected error from cancelled context")
	}
	// The error is wrapped, so we check for the underlying error.
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("expected context canceled error, got %v", err)
	}
}

func TestWHOISClientConfig(t *testing.T) {
	// Test default config.
	client := NewWHOISClient(WHOISClientConfig{})
	if client.timeout != 15*time.Second {
		t.Errorf("expected default timeout 15s, got %v", client.timeout)
	}
	if client.ratelimit == nil {
		t.Error("expected rate limiter to be created")
	}

	// Test custom config.
	customTimeout := 30 * time.Second
	rl := NewWHOISRateLimiter()
	client = NewWHOISClient(WHOISClientConfig{
		Timeout:   customTimeout,
		RateLimit: rl,
		UserAgent: "test-agent",
	})
	if client.timeout != customTimeout {
		t.Errorf("expected timeout %v, got %v", customTimeout, client.timeout)
	}
	if client.ratelimit != rl {
		t.Error("expected custom rate limiter")
	}
}

func TestWhoisServerForTLD(t *testing.T) {
	tests := []struct {
		tld      string
		expected string
	}{
		{"de", "whois.denic.de"},
		{"jp", "whois.jprs.jp"},
		{"ru", "whois.tcin.ru"},
		{"ch", "whois.nic.ch"},
		{"gg", "whois.gg"},
		{"unknown", "whois.iana.org"},
		{"", "whois.iana.org"},
	}

	for _, tt := range tests {
		t.Run(tt.tld, func(t *testing.T) {
			result := whoisServerForTLD(tt.tld)
			if result != tt.expected {
				t.Errorf("whoisServerForTLD(%q) = %q, want %q", tt.tld, result, tt.expected)
			}
		})
	}
}

func TestIsAvailableFromRaw(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		tld      string
		expected bool
	}{
		{
			name:     "no match",
			raw:      "No match for domain example.com",
			tld:      "com",
			expected: true,
		},
		{
			name:     "not found",
			raw:      "Domain not found",
			tld:      "com",
			expected: true,
		},
		{
			name:     "status free (de)",
			raw:      "Domain: example.de\nStatus: free",
			tld:      "de",
			expected: true,
		},
		{
			name:     "jprs no data",
			raw:      "[ No Matching Data ]",
			tld:      "jp",
			expected: true,
		},
		{
			name:     "registered with registrar",
			raw:      "Domain Name: EXAMPLE.COM\nRegistrar: Example Registrar Inc.",
			tld:      "com",
			expected: false,
		},
		{
			name:     "registered with creation date",
			raw:      "Domain: example.de\nCreated: 2020-01-01",
			tld:      "de",
			expected: false,
		},
		{
			name:     "ru no entries",
			raw:      "No entries found for the selected source(s).",
			tld:      "ru",
			expected: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isAvailableFromRaw(tt.raw, tt.tld)
			if result != tt.expected {
				t.Errorf("isAvailableFromRaw() = %v, want %v", result, tt.expected)
			}
		})
	}
}

func TestNeedsWHOIS(t *testing.T) {
	tests := []struct {
		tld      string
		expected bool
	}{
		{"de", true},
		{"jp", true},
		{"ru", true},
		{"com", false},
		{"net", false},
		{"org", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.tld, func(t *testing.T) {
			result := NeedsWHOIS(tt.tld)
			if result != tt.expected {
				t.Errorf("NeedsWHOIS(%q) = %v, want %v", tt.tld, result, tt.expected)
			}
		})
	}
}

func TestWHOISClient_SanitizeDomain(t *testing.T) {
	client := NewWHOISClient(WHOISClientConfig{})
	ctx := context.Background()

	tests := []struct {
		name    string
		domain  string
		wantErr bool
	}{
		{"valid domain", "example.de", false},
		{"with slash", "example.com/whois", true},
		{"with colon", "http://example.com", true},
		{"with at", "user@example.com", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Check(ctx, tt.domain)
			if (err != nil) != tt.wantErr {
				t.Errorf("Check(%q) error = %v, wantErr %v", tt.domain, err, tt.wantErr)
			}
		})
	}
}

func TestWHOISClient_Timeout(t *testing.T) {
	// Create a client with a very short timeout.
	client := NewWHOISClient(WHOISClientConfig{
		Timeout: 1 * time.Millisecond,
	})

	ctx := context.Background()
	result, err := client.Check(ctx, "example.de")

	// Should not return an error (errors are in result.Error).
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should have a timeout error in the result.
	if result.Error == "" {
		t.Error("expected timeout error in result")
	}
	if !strings.Contains(result.Error, "timeout") && !strings.Contains(result.Error, "context") {
		t.Errorf("expected timeout-related error, got: %s", result.Error)
	}
}

func TestWHOISClient_InvalidDomain(t *testing.T) {
	client := NewWHOISClient(WHOISClientConfig{})
	ctx := context.Background()

	// Test with domain that's too short (no TLD).
	_, err := client.Check(ctx, "invalid")
	if err == nil {
		t.Error("expected error for invalid domain format")
	}
}

func TestCcTLDsNeedingWHOIS(t *testing.T) {
	ccTLDs := ccTLDsNeedingWHOIS()

	expected := []string{"de", "co", "jp", "kr", "cn", "ru", "se", "ch", "at", "be", "nz", "gg"}
	for _, exp := range expected {
		found := false
		for _, tld := range ccTLDs {
			if tld == exp {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected ccTLD %q in list", exp)
		}
	}
}

func TestWHOISRateConfigDefaults(t *testing.T) {
	// Test that default config has reasonable values.
	cfg := defaultWHOISConfig

	if cfg.Rate <= 0 {
		t.Error("expected positive rate")
	}
	if cfg.Burst < 1 {
		t.Error("expected burst >= 1")
	}
	if cfg.MinInterval < 0 {
		t.Error("expected non-negative min interval")
	}
}

func TestWHOISRegistryConfigs(t *testing.T) {
	// Test that known aggressive registries have appropriate limits.
	tests := []struct {
		server       string
		minInterval  time.Duration
	}{
		{"whois.denic.de", 5 * time.Second}, // DENIC is very aggressive
		{"whois.nic.co", 2 * time.Second},
		{"whois.jprs.jp", time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.server, func(t *testing.T) {
			cfg, ok := whoisRegistryConfigs[tt.server]
			if !ok {
				t.Fatalf("config not found for %s", tt.server)
			}
			if cfg.MinInterval < tt.minInterval {
				t.Errorf("expected min interval >= %v for %s, got %v", tt.minInterval, tt.server, cfg.MinInterval)
			}
		})
	}
}
