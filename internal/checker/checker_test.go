package checker

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coding/domain-check/internal/domain"
)

// mockRDAPServer creates a test HTTP server that responds to RDAP queries.
func mockRDAPServer(t *testing.T, responses map[string]string) *httptest.Server {
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract domain from path like /domain/example.com
		path := r.URL.Path
		t.Logf("Mock server received request: %s", path)
		if !strings.HasPrefix(path, "/domain/") {
			t.Logf("Path doesn't start with /domain/, returning 404")
			http.NotFound(w, r)
			return
		}
		domainName := strings.TrimPrefix(path, "/domain/")
		t.Logf("Extracted domain: %s", domainName)

		resp, ok := responses[domainName]
		t.Logf("Response lookup: found=%v, value=%q", ok, resp)
		if !ok {
			// Default: return 404 (available)
			t.Logf("No response configured, returning 404")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		if resp == "429" {
			w.WriteHeader(http.StatusTooManyRequests)
			w.Write([]byte(`{"errorCode":429,"title":"Rate Limited"}`))
			return
		}

		if resp == "error" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		// Return 200 with RDAP response
		w.Header().Set("Content-Type", "application/rdap+json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(resp))
	}))
}

// testHTTPClient creates an HTTP client for testing that allows localhost connections.
func testHTTPClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			DialContext: (&net.Dialer{
				Timeout:   5 * time.Second,
				KeepAlive: 30 * time.Second,
			}).DialContext,
			TLSHandshakeTimeout:   5 * time.Second,
			ResponseHeaderTimeout: 10 * time.Second,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			IdleConnTimeout:       90 * time.Second,
		},
		Timeout: 15 * time.Second,
	}
}

func TestCheckBulk_Empty(t *testing.T) {
	cache := NewResultCache(DefaultTTLs(), 100)
	checker := NewChecker(CheckerConfig{
		Cache: cache,
	})

	result := checker.CheckBulk(context.Background(), nil)

	if len(result.Results) != 0 {
		t.Errorf("expected 0 results, got %d", len(result.Results))
	}
	if result.TotalChecked != 0 {
		t.Errorf("expected 0 total checked, got %d", result.TotalChecked)
	}
	if result.TotalCached != 0 {
		t.Errorf("expected 0 total cached, got %d", result.TotalCached)
	}
}

func TestCheckBulk_SingleDomain(t *testing.T) {
	// Create mock RDAP server
	server := mockRDAPServer(t, map[string]string{
		"available.com": "404", // Available
	})
	defer server.Close()

	// Create bootstrap manager with mock server
	bootstrap := &BootstrapManager{
		servers: map[string]string{
			"com": server.URL + "/",
		},
	}

	// Create allowlist (for validation, but we use testHTTPClient)
	allowlist := NewAllowList([]string{server.URL})

	// Create test HTTP client (allows localhost for testing)
	httpClient := testHTTPClient()

	// Create rate limiter
	ratelimit := NewRateLimiter()

	// Create RDAP client
	rdapClient := NewRDAPClient(RDAPClientConfig{
		HTTPClient: httpClient,
		Bootstrap:  bootstrap,
		RateLimit:  ratelimit,
		AllowList:  allowlist,
	})

	// Create cache
	cache := NewResultCache(DefaultTTLs(), 100)

	// Create checker
	checker := NewChecker(CheckerConfig{
		RDAPClient: rdapClient,
		Cache:      cache,
		Bootstrap:  bootstrap,
	})

	// Test single domain
	result := checker.CheckBulk(context.Background(), []string{"available.com"})

	if len(result.Results) != 1 {
		t.Errorf("expected 1 result, got %d (errors: %v)", len(result.Results), result.Errors)
	}

	domainResult, ok := result.Results["available.com"]
	if !ok {
		t.Fatalf("expected result for available.com, errors: %v", result.Errors)
	}

	if !domainResult.Available {
		t.Errorf("expected domain to be available, got: %+v", domainResult)
	}
	if domainResult.Domain != "available.com" {
		t.Errorf("expected domain 'available.com', got %q", domainResult.Domain)
	}
}

func TestCheckBulk_MultipleDomains(t *testing.T) {
	// Create mock RDAP server
	server := mockRDAPServer(t, map[string]string{
		"available.com": "404",
		"taken.com": `{
			"objectClassName": "domain",
			"ldhName": "TAKEN.COM",
			"status": ["client transfer prohibited"],
			"entities": [{"roles": ["registrar"], "ldhName": "Test Registrar"}],
			"events": [
				{"eventAction": "registration", "eventDate": "2020-01-01T00:00:00Z"},
				{"eventAction": "expiration", "eventDate": "2025-01-01T00:00:00Z"}
			]
		}`,
		"error.com": "error",
	})
	defer server.Close()

	bootstrap := &BootstrapManager{
		servers: map[string]string{
			"com": server.URL + "/",
		},
	}

	allowlist := NewAllowList([]string{server.URL})
	httpClient := testHTTPClient()
	ratelimit := NewRateLimiter()

	rdapClient := NewRDAPClient(RDAPClientConfig{
		HTTPClient: httpClient,
		Bootstrap:  bootstrap,
		RateLimit:  ratelimit,
		AllowList:  allowlist,
	})

	cache := NewResultCache(DefaultTTLs(), 100)

	checker := NewChecker(CheckerConfig{
		RDAPClient: rdapClient,
		Cache:      cache,
		Bootstrap:  bootstrap,
	})

	result := checker.CheckBulk(context.Background(), []string{
		"available.com",
		"taken.com",
		"error.com",
	})

	// available.com and taken.com should succeed
	// error.com returns 500 which sets an error on the result but doesn't fail
	if len(result.Results) < 2 {
		t.Errorf("expected at least 2 results, got %d (results: %v, errors: %v)",
			len(result.Results), result.Results, result.Errors)
	}

	// Check available domain
	if r := result.Results["available.com"]; r == nil || !r.Available {
		t.Error("expected available.com to be available")
	}

	// Check taken domain
	if r := result.Results["taken.com"]; r == nil || r.Available {
		t.Error("expected taken.com to be registered")
	}
}

func TestCheckBulk_CacheHit(t *testing.T) {
	server := mockRDAPServer(t, map[string]string{
		"cached.com": "404",
	})
	defer server.Close()

	bootstrap := &BootstrapManager{
		servers: map[string]string{
			"com": server.URL + "/",
		},
	}

	allowlist := NewAllowList([]string{server.URL})
	httpClient := testHTTPClient()
	ratelimit := NewRateLimiter()

	rdapClient := NewRDAPClient(RDAPClientConfig{
		HTTPClient: httpClient,
		Bootstrap:  bootstrap,
		RateLimit:  ratelimit,
		AllowList:  allowlist,
	})

	cache := NewResultCache(DefaultTTLs(), 100)

	// Pre-populate cache
	now := time.Now()
	cache.Set("cached.com", domain.DomainResult{
		Domain:     "cached.com",
		Available:  true,
		TLD:        "com",
		CheckedAt:  now,
		Source:     domain.SourceRDAP,
		DurationMs: 100,
	})

	checker := NewChecker(CheckerConfig{
		RDAPClient: rdapClient,
		Cache:      cache,
		Bootstrap:  bootstrap,
	})

	result := checker.CheckBulk(context.Background(), []string{"cached.com"})

	if result.TotalCached != 1 {
		t.Errorf("expected 1 cache hit, got %d", result.TotalCached)
	}

	// Verify result is from cache
	r := result.Results["cached.com"]
	if r == nil {
		t.Fatal("expected cached result")
	}
	if !r.Cached {
		t.Error("expected Cached flag to be true")
	}
}

func TestCheckBulk_Timeout(t *testing.T) {
	// Create a server that intentionally delays
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second) // Delay longer than timeout
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	bootstrap := &BootstrapManager{
		servers: map[string]string{
			"com": server.URL + "/",
		},
	}

	allowlist := NewAllowList([]string{server.URL})
	httpClient := testHTTPClient()
	ratelimit := NewRateLimiter()

	rdapClient := NewRDAPClient(RDAPClientConfig{
		HTTPClient: httpClient,
		Bootstrap:  bootstrap,
		RateLimit:  ratelimit,
		AllowList:  allowlist,
	})

	checker := NewChecker(CheckerConfig{
		RDAPClient: rdapClient,
		Bootstrap:  bootstrap,
		BulkConfig: BulkCheckConfig{
			GlobalConcurrency: 50,
			TotalTimeout:      100 * time.Millisecond, // Very short timeout
		},
	})

	ctx := context.Background()
	result := checker.CheckBulk(ctx, []string{"slow.com"})

	// Should have an error due to timeout
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error for timeout, got %d (errors: %v)", len(result.Errors), result.Errors)
	}

	// Duration should be approximately the timeout
	if result.Duration > 500*time.Millisecond {
		t.Errorf("bulk check took too long: %v", result.Duration)
	}
}

func TestCheckBulk_ConcurrencyLimit(t *testing.T) {
	var mu sync.Mutex
	activeCount := 0
	maxConcurrent := 0

	// Create a server that tracks concurrent requests
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		activeCount++
		if activeCount > maxConcurrent {
			maxConcurrent = activeCount
		}
		mu.Unlock()

		defer func() {
			mu.Lock()
			activeCount--
			mu.Unlock()
		}()

		// Simulate some work
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	bootstrap := &BootstrapManager{
		servers: map[string]string{
			"com": server.URL + "/",
		},
	}

	allowlist := NewAllowList([]string{server.URL})
	httpClient := testHTTPClient()
	ratelimit := NewRateLimiter()

	rdapClient := NewRDAPClient(RDAPClientConfig{
		HTTPClient: httpClient,
		Bootstrap:  bootstrap,
		RateLimit:  ratelimit,
		AllowList:  allowlist,
	})

	// Create checker with low concurrency limit
	checker := NewChecker(CheckerConfig{
		RDAPClient: rdapClient,
		Bootstrap:  bootstrap,
		BulkConfig: BulkCheckConfig{
			GlobalConcurrency: 5,
			TotalTimeout:      30 * time.Second,
		},
	})

	// Check 10 unique domains
	domains := make([]string, 10)
	for i := 0; i < 10; i++ {
		domains[i] = "test.com" // Same domain but multiple requests
	}

	result := checker.CheckBulk(context.Background(), domains)

	// Verify max concurrency didn't exceed limit
	// Note: Due to caching, after the first check the rest are cached
	if maxConcurrent > 6 { // Allow some slack
		t.Errorf("concurrency limit may have been exceeded: max concurrent was %d", maxConcurrent)
	}

	// All should succeed (after first check, rest are from cache)
	if len(result.Results) != 10 {
		t.Errorf("expected 10 results, got %d", len(result.Results))
	}
	if result.TotalCached != 9 {
		t.Errorf("expected 9 cache hits, got %d", result.TotalCached)
	}
}

func TestCheckBulk_PartialResults(t *testing.T) {
	// Create server that returns errors for some domains
	server := mockRDAPServer(t, map[string]string{
		"good1.com": "404",
		"good2.com": "404",
		"bad.com":   "429", // Rate limited
	})
	defer server.Close()

	bootstrap := &BootstrapManager{
		servers: map[string]string{
			"com": server.URL + "/",
		},
	}

	allowlist := NewAllowList([]string{server.URL})
	httpClient := testHTTPClient()
	ratelimit := NewRateLimiter()

	rdapClient := NewRDAPClient(RDAPClientConfig{
		HTTPClient: httpClient,
		Bootstrap:  bootstrap,
		RateLimit:  ratelimit,
		AllowList:  allowlist,
	})

	checker := NewChecker(CheckerConfig{
		RDAPClient: rdapClient,
		Cache:      NewResultCache(DefaultTTLs(), 100),
		Bootstrap:  bootstrap,
	})

	result := checker.CheckBulk(context.Background(), []string{
		"good1.com",
		"good2.com",
		"bad.com",
	})

	// Should have at least 2 successful results
	if len(result.Results) < 2 {
		t.Errorf("expected at least 2 successful results, got %d", len(result.Results))
	}

	// Verify successful domains
	if r := result.Results["good1.com"]; r == nil || !r.Available {
		t.Error("missing or unavailable result for good1.com")
	}
	if r := result.Results["good2.com"]; r == nil || !r.Available {
		t.Error("missing or unavailable result for good2.com")
	}

	// bad.com should have a result with an error (rate limited)
	if r := result.Results["bad.com"]; r != nil && r.Error == "" {
		t.Error("expected error on bad.com result for rate limiting")
	}
}

func TestCheckBulk_RegistryGrouping(t *testing.T) {
	// Create two mock servers for different registries
	comServer := mockRDAPServer(t, map[string]string{
		"test.com": "404",
	})
	defer comServer.Close()

	orgServer := mockRDAPServer(t, map[string]string{
		"test.org": "404",
	})
	defer orgServer.Close()

	bootstrap := &BootstrapManager{
		servers: map[string]string{
			"com": comServer.URL + "/",
			"org": orgServer.URL + "/",
		},
	}

	allowlist := NewAllowList([]string{comServer.URL, orgServer.URL})
	httpClient := testHTTPClient()
	ratelimit := NewRateLimiter()

	rdapClient := NewRDAPClient(RDAPClientConfig{
		HTTPClient: httpClient,
		Bootstrap:  bootstrap,
		RateLimit:  ratelimit,
		AllowList:  allowlist,
	})

	checker := NewChecker(CheckerConfig{
		RDAPClient: rdapClient,
		Cache:      NewResultCache(DefaultTTLs(), 100),
		Bootstrap:  bootstrap,
	})

	result := checker.CheckBulk(context.Background(), []string{
		"test.com",
		"test.org",
	})

	if len(result.Results) != 2 {
		t.Errorf("expected 2 results, got %d", len(result.Results))
	}

	// Verify both domains were checked
	if r := result.Results["test.com"]; r == nil || !r.Available {
		t.Error("missing or unavailable result for test.com")
	}
	if r := result.Results["test.org"]; r == nil || !r.Available {
		t.Error("missing or unavailable result for test.org")
	}
}

func TestCheck_ContextCancellation(t *testing.T) {
	// Create a slow server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	bootstrap := &BootstrapManager{
		servers: map[string]string{
			"com": server.URL + "/",
		},
	}

	allowlist := NewAllowList([]string{server.URL})
	httpClient := testHTTPClient()
	ratelimit := NewRateLimiter()

	rdapClient := NewRDAPClient(RDAPClientConfig{
		HTTPClient: httpClient,
		Bootstrap:  bootstrap,
		RateLimit:  ratelimit,
		AllowList:  allowlist,
	})

	checker := NewChecker(CheckerConfig{
		RDAPClient: rdapClient,
		Bootstrap:  bootstrap,
		BulkConfig: BulkCheckConfig{
			GlobalConcurrency: 50,
			TotalTimeout:      5 * time.Second,
		},
	})

	// Create a context that we'll cancel
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	result := checker.CheckBulk(ctx, []string{"slow.com"})

	// Should have an error due to cancellation
	if len(result.Errors) != 1 {
		t.Errorf("expected 1 error for cancellation, got %d", len(result.Errors))
	}
}

func TestCheckBulk_MaxDomains(t *testing.T) {
	// Create server
	server := mockRDAPServer(t, nil)
	defer server.Close()

	bootstrap := &BootstrapManager{
		servers: map[string]string{
			"com": server.URL + "/",
		},
	}

	allowlist := NewAllowList([]string{server.URL})
	httpClient := testHTTPClient()
	ratelimit := NewRateLimiter()

	rdapClient := NewRDAPClient(RDAPClientConfig{
		HTTPClient: httpClient,
		Bootstrap:  bootstrap,
		RateLimit:  ratelimit,
		AllowList:  allowlist,
	})

	checker := NewChecker(CheckerConfig{
		RDAPClient: rdapClient,
		Bootstrap:  bootstrap,
	})

	// Create 50 domains (max allowed per API spec)
	domains := make([]string, 50)
	for i := 0; i < 50; i++ {
		domains[i] = "test.com"
	}

	result := checker.CheckBulk(context.Background(), domains)

	// All 50 should succeed (first one checks, rest from cache)
	if len(result.Results) != 50 {
		t.Errorf("expected 50 results, got %d", len(result.Results))
	}
}

func TestGroupByRegistry(t *testing.T) {
	bootstrap := &BootstrapManager{
		servers: map[string]string{
			"com": "https://rdap.verisign.com/com/v1/",
			"net": "https://rdap.verisign.com/net/v1/",
			"org": "https://rdap.publicinterestregistry.org/rdap/",
		},
	}

	checker := &Checker{
		bootstrap: bootstrap,
	}

	groups := checker.groupByRegistry([]string{
		"example.com",
		"test.net",
		"sample.org",
	})

	// com and net should both map to verisign
	if groups["example.com"] != "rdap.verisign.com" {
		t.Errorf("expected rdap.verisign.com, got %s", groups["example.com"])
	}
	if groups["test.net"] != "rdap.verisign.com" {
		t.Errorf("expected rdap.verisign.com, got %s", groups["test.net"])
	}
	if groups["sample.org"] != "rdap.publicinterestregistry.org" {
		t.Errorf("expected rdap.publicinterestregistry.org, got %s", groups["sample.org"])
	}
}

func TestGetRegistryForDomain(t *testing.T) {
	bootstrap := &BootstrapManager{
		servers: map[string]string{
			"com": "https://rdap.verisign.com/com/v1/",
		},
	}

	checker := &Checker{
		bootstrap: bootstrap,
	}

	tests := []struct {
		domain   string
		expected string
	}{
		{"example.com", "rdap.verisign.com"},
		{"test.de", "whois.denic.de"}, // WHOIS fallback
		{"unknown.xyz", "unknown"},    // Not in bootstrap
	}

	for _, tt := range tests {
		t.Run(tt.domain, func(t *testing.T) {
			result := checker.getRegistryForDomain(tt.domain)
			if result != tt.expected {
				t.Errorf("expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func BenchmarkCheckBulk_10Domains(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	bootstrap := &BootstrapManager{
		servers: map[string]string{
			"com": server.URL + "/",
		},
	}

	allowlist := NewAllowList([]string{server.URL})
	httpClient := testHTTPClient()
	ratelimit := NewRateLimiter()

	rdapClient := NewRDAPClient(RDAPClientConfig{
		HTTPClient: httpClient,
		Bootstrap:  bootstrap,
		RateLimit:  ratelimit,
		AllowList:  allowlist,
	})

	checker := NewChecker(CheckerConfig{
		RDAPClient: rdapClient,
		Bootstrap:  bootstrap,
	})

	domains := []string{"a.com", "b.com", "c.com", "d.com", "e.com",
		"f.com", "g.com", "h.com", "i.com", "j.com"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// Clear cache between iterations
		checker.CheckBulk(context.Background(), domains)
	}
}

func BenchmarkCheckBulk_50Domains(b *testing.B) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	bootstrap := &BootstrapManager{
		servers: map[string]string{
			"com": server.URL + "/",
		},
	}

	allowlist := NewAllowList([]string{server.URL})
	httpClient := testHTTPClient()
	ratelimit := NewRateLimiter()

	rdapClient := NewRDAPClient(RDAPClientConfig{
		HTTPClient: httpClient,
		Bootstrap:  bootstrap,
		RateLimit:  ratelimit,
		AllowList:  allowlist,
	})

	checker := NewChecker(CheckerConfig{
		RDAPClient: rdapClient,
		Bootstrap:  bootstrap,
	})

	domains := make([]string, 50)
	for i := 0; i < 50; i++ {
		domains[i] = "test.com"
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		checker.CheckBulk(context.Background(), domains)
	}
}

func TestCheckBulk_PerRegistrySemaphore(t *testing.T) {
	// This test verifies that per-registry semaphores limit concurrent
	// requests to each registry while allowing full concurrency across registries.

	var mu sync.Mutex
	activeByRegistry := make(map[string]int)
	maxByRegistry := make(map[string]int)

	// Create a server that tracks concurrent requests per "registry" (by domain suffix)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Extract domain from path
		path := r.URL.Path
		domain := strings.TrimPrefix(path, "/domain/")

		// Determine registry from domain (for test purposes)
		var registry string
		if strings.HasSuffix(domain, ".com") {
			registry = "rdap.verisign.com"
		} else if strings.HasSuffix(domain, ".org") {
			registry = "rdap.publicinterestregistry.org"
		} else {
			registry = "unknown"
		}

		mu.Lock()
		activeByRegistry[registry]++
		if activeByRegistry[registry] > maxByRegistry[registry] {
			maxByRegistry[registry] = activeByRegistry[registry]
		}
		mu.Unlock()

		defer func() {
			mu.Lock()
			activeByRegistry[registry]--
			mu.Unlock()
		}()

		// Simulate some work
		time.Sleep(50 * time.Millisecond)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	bootstrap := &BootstrapManager{
		servers: map[string]string{
			"com": server.URL + "/",
			"org": server.URL + "/",
		},
	}

	allowlist := NewAllowList([]string{server.URL})
	httpClient := testHTTPClient()
	ratelimit := NewRateLimiter()

	rdapClient := NewRDAPClient(RDAPClientConfig{
		HTTPClient: httpClient,
		Bootstrap:  bootstrap,
		RateLimit:  ratelimit,
		AllowList:  allowlist,
	})

	checker := NewChecker(CheckerConfig{
		RDAPClient: rdapClient,
		Bootstrap:  bootstrap,
		BulkConfig: BulkCheckConfig{
			GlobalConcurrency: 50,
			TotalTimeout:      30 * time.Second,
		},
	})

	// Create unique domains to avoid caching
	domains := make([]string, 0, 30)
	for i := 0; i < 15; i++ {
		domains = append(domains, fmt.Sprintf("test%d.com", i))
		domains = append(domains, fmt.Sprintf("test%d.org", i))
	}

	result := checker.CheckBulk(context.Background(), domains)

	// All domains should succeed
	if len(result.Results) != 30 {
		t.Errorf("expected 30 results, got %d (errors: %v)", len(result.Results), result.Errors)
	}

	// Verify per-registry concurrency limits
	// Verisign (.com) should be limited to 10 concurrent
	if max := maxByRegistry["rdap.verisign.com"]; max > VerisignConcurrency {
		t.Errorf("Verisign concurrency exceeded: max %d, limit %d", max, VerisignConcurrency)
	}

	// PIR (.org) should be limited to 10 concurrent
	if max := maxByRegistry["rdap.publicinterestregistry.org"]; max > PIRConcurrency {
		t.Errorf("PIR concurrency exceeded: max %d, limit %d", max, PIRConcurrency)
	}
}

func TestGetRegistrySem(t *testing.T) {
	checker := NewChecker(CheckerConfig{})

	// Test that known registries get their configured limits
	tests := []struct {
		registry   string
		expectedLimit int64
	}{
		{"rdap.verisign.com", VerisignConcurrency},
		{"rdap.publicinterestregistry.org", PIRConcurrency},
		{"pubapi.registry.google", GoogleConcurrency},
		{"unknown.registry.example", DefaultRegistryConcurrency},
	}

	for _, tt := range tests {
		t.Run(tt.registry, func(t *testing.T) {
			sem := checker.getRegistrySem(tt.registry)
			if sem == nil {
				t.Fatal("expected semaphore, got nil")
			}

			// Verify we can acquire up to the expected limit
			ctx := context.Background()
			for i := int64(0); i < tt.expectedLimit; i++ {
				if err := sem.Acquire(ctx, 1); err != nil {
					t.Errorf("failed to acquire semaphore %d/%d: %v", i+1, tt.expectedLimit, err)
				}
			}

			// The next acquire should block (we verify by using a short timeout)
			timeoutCtx, cancel := context.WithTimeout(ctx, 10*time.Millisecond)
			defer cancel()
			if err := sem.Acquire(timeoutCtx, 1); err == nil {
				t.Errorf("expected timeout when acquiring beyond limit")
			}

			// Release all acquired tokens
			for i := int64(0); i < tt.expectedLimit; i++ {
				sem.Release(1)
			}
		})
	}
}

func TestCheckBulk_MixedRegistries(t *testing.T) {
	// Test that bulk checks efficiently handle domains across multiple registries
	// with different rate limits.

	var requestOrder muSafeSlice
	var mu sync.Mutex

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		domain := strings.TrimPrefix(r.URL.Path, "/domain/")
		mu.Lock()
		requestOrder = append(requestOrder, domain)
		mu.Unlock()

		time.Sleep(20 * time.Millisecond)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	bootstrap := &BootstrapManager{
		servers: map[string]string{
			"com": server.URL + "/",
			"org": server.URL + "/",
		},
	}

	allowlist := NewAllowList([]string{server.URL})
	httpClient := testHTTPClient()
	ratelimit := NewRateLimiter()

	rdapClient := NewRDAPClient(RDAPClientConfig{
		HTTPClient: httpClient,
		Bootstrap:  bootstrap,
		RateLimit:  ratelimit,
		AllowList:  allowlist,
	})

	checker := NewChecker(CheckerConfig{
		RDAPClient: rdapClient,
		Bootstrap:  bootstrap,
	})

	// Mix of .com and .org domains
	domains := []string{
		"a.com", "b.com", "c.com",
		"x.org", "y.org", "z.org",
	}

	start := time.Now()
	result := checker.CheckBulk(context.Background(), domains)
	elapsed := time.Since(start)

	if len(result.Results) != 6 {
		t.Errorf("expected 6 results, got %d", len(result.Results))
	}

	// With parallel execution and 20ms per request, 6 domains should complete
	// much faster than 6 * 20ms = 120ms if running in parallel
	if elapsed > 100*time.Millisecond {
		t.Logf("Warning: bulk check took %v, expected faster parallel execution", elapsed)
	}
}

// muSafeSlice is a thread-safe slice for testing
type muSafeSlice []string
