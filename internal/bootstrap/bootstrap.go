// Package bootstrap loads, caches, and refreshes the IANA RDAP bootstrap file.
package bootstrap

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/jedarden/domain-check/internal/resilience"
)

// defaultBootstrapURL is the IANA RDAP bootstrap file URL.
const defaultBootstrapURL = "https://data.iana.org/rdap/dns.json"

// defaultRefreshInterval is how often the bootstrap file is re-fetched.
const defaultRefreshInterval = 24 * time.Hour

// fallbackServers are used when the IANA bootstrap fetch fails.
var fallbackServers = map[string]string{
	"com": "https://rdap.verisign.com/com/v1/",
	"net": "https://rdap.verisign.com/net/v1/",
	"org": "https://rdap.publicinterestregistry.org/rdap/",
}

// ErrTLDNotFound is returned when no RDAP server is known for a TLD.
var ErrTLDNotFound = errors.New("no RDAP server found for TLD")

// Manager loads, caches, and refreshes the IANA RDAP bootstrap file.
type Manager struct {
	mu       sync.RWMutex
	servers  map[string]string // TLD → RDAP server base URL
	updated  time.Time
	url      string
	client   *http.Client
	stopCh   chan struct{}
	stopped  chan struct{}
	breakers *resilience.BreakerSet // circuit breaker for the bootstrap host
	retryCfg resilience.RetryConfig // policy for the outbound bootstrap fetch
}

// NewManager creates a Manager that fetches the IANA bootstrap
// file from the given URL. If url is empty, the default IANA URL is used.
// It performs an initial fetch synchronously and starts a background refresh goroutine.
func NewManager(ctx context.Context, url string) (*Manager, error) {
	return newManagerWithRetry(ctx, url, resilience.DefaultInteractiveRetryConfig())
}

// newManagerWithRetry builds a Manager with an explicit retry policy. It is
// the seam tests use to observe or shorten the retry schedule; production
// callers go through NewManager.
func newManagerWithRetry(ctx context.Context, url string, cfg resilience.RetryConfig) (*Manager, error) {
	if url == "" {
		url = defaultBootstrapURL
	}

	b := &Manager{
		servers:  make(map[string]string),
		url:      url,
		client:   &http.Client{Timeout: 30 * time.Second},
		stopCh:   make(chan struct{}),
		stopped:  make(chan struct{}),
		breakers: resilience.NewBreakerSet(resilience.BreakerConfig{}),
		retryCfg: cfg,
	}

	// Initial fetch — use fallbacks on failure.
	if err := b.Refresh(ctx); err != nil {
		b.loadFallbacks()
	}

	go b.refreshLoop()

	return b, nil
}

// retryConfig returns the manager's retry policy, falling back to the
// interactive default when unset (a Manager not built by newManagerWithRetry).
func (b *Manager) retryConfig() resilience.RetryConfig {
	if b.retryCfg.BaseDelay <= 0 {
		return resilience.DefaultInteractiveRetryConfig()
	}
	return b.retryCfg
}

// breakerFor returns the circuit breaker for the bootstrap host, creating the
// set on first use so any Manager is gated.
func (b *Manager) breakerFor(host string) *resilience.CircuitBreaker {
	if b.breakers == nil {
		b.breakers = resilience.NewBreakerSet(resilience.BreakerConfig{})
	}
	return b.breakers.Get(host)
}

// Refresh fetches and parses the IANA bootstrap file, updating the TLD→URL map.
func (b *Manager) Refresh(ctx context.Context) error {
	body, err := b.fetch(ctx)
	if err != nil {
		return err
	}

	servers, err := parseBootstrap(body)
	if err != nil {
		return fmt.Errorf("parse bootstrap: %w", err)
	}

	b.mu.Lock()
	b.servers = servers
	b.updated = time.Now()
	b.mu.Unlock()

	return nil
}

// fetch performs the bootstrap HTTP GET with exponential-backoff retry and a
// circuit breaker on the bootstrap host. Only transient conditions are
// retried — transport errors and 408/429/502/503/504 answers; any other
// status surfaces on the first attempt. Once the breaker is open the fetch
// fails fast with an error wrapping ErrCircuitOpen and no network I/O, which
// keeps a persistently unreachable IANA from stalling startup or the 24h
// refresh loop: the caller keeps serving the last known mapping (or the
// built-in fallbacks).
func (b *Manager) fetch(ctx context.Context) ([]byte, error) {
	host := bootstrapHost(b.url)
	breaker := b.breakerFor(host)

	var body []byte
	err := resilience.Do(ctx, b.retryConfig(), func(int) error {
		// An open circuit fails before any network I/O; its message names
		// the bootstrap host. Classified permanent so the retry loop does
		// not burn its budget on a source we have stopped asking.
		if err := breaker.Allow(); err != nil {
			return resilience.WrapPermanent(err)
		}

		data, err := b.fetchOnce(ctx)
		if err != nil {
			breaker.Record(err)
			return err // already classified by fetchOnce
		}
		breaker.Record(nil)
		body = data
		return nil
	})
	if err != nil {
		return nil, err
	}
	return body, nil
}

// fetchOnce issues a single bootstrap GET and reads the body — one attempt of
// fetch's retry loop. Returned errors carry their transient/permanent class.
func (b *Manager) fetchOnce(ctx context.Context) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.url, nil)
	if err != nil {
		// A URL that cannot be parsed will not parse any better on retry.
		return nil, resilience.WrapPermanent(fmt.Errorf("create bootstrap request: %w", err))
	}

	resp, err := b.client.Do(req)
	if err != nil {
		// Unclassified transport errors are treated as transient: a dropped
		// connection or a refused dial is usually a blip.
		return nil, resilience.WrapTransient(fmt.Errorf("fetch bootstrap: %w", err))
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		err := fmt.Errorf("fetch bootstrap: HTTP %d", resp.StatusCode)
		if resilience.IsTransientStatus(resp.StatusCode) {
			return nil, resilience.WrapTransient(err)
		}
		return nil, resilience.WrapPermanent(err)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return nil, resilience.WrapTransient(fmt.Errorf("read bootstrap body: %w", err))
	}
	return body, nil
}

// bootstrapHost extracts the host from a bootstrap URL for the breaker name.
func bootstrapHost(raw string) string {
	if u, err := url.Parse(raw); err == nil && u.Host != "" {
		return u.Host
	}
	return raw
}

// Lookup returns the RDAP server base URL for the given TLD.
// It returns ErrTLDNotFound if no server is known for the TLD.
func (b *Manager) Lookup(tld string) (string, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	url, ok := b.servers[tld]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrTLDNotFound, tld)
	}
	return url, nil
}

// Updated returns the time of the last successful bootstrap refresh.
func (b *Manager) Updated() time.Time {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.updated
}

// ServerCount returns the number of TLDs currently mapped.
func (b *Manager) ServerCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.servers)
}

// URLs returns all RDAP server base URLs currently mapped.
// The returned slice is a copy and is safe for the caller to modify.
func (b *Manager) URLs() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	// Use a map to deduplicate URLs (multiple TLDs may share the same server).
	seen := make(map[string]bool, len(b.servers))
	urls := make([]string, 0, len(b.servers))
	for _, url := range b.servers {
		if !seen[url] {
			seen[url] = true
			urls = append(urls, url)
		}
	}
	return urls
}

// TLDs returns all TLDs currently mapped.
// The returned slice is a copy and is safe for the caller to modify.
func (b *Manager) TLDs() []string {
	b.mu.RLock()
	defer b.mu.RUnlock()

	tlds := make([]string, 0, len(b.servers))
	for tld := range b.servers {
		tlds = append(tlds, tld)
	}
	return tlds
}

// InjectServers replaces the server map; intended for testing only.
func (b *Manager) InjectServers(servers map[string]string) {
	b.mu.Lock()
	b.servers = servers
	b.mu.Unlock()
}

// Stop terminates the background refresh goroutine.
func (b *Manager) Stop() {
	close(b.stopCh)
	<-b.stopped
}

// refreshLoop periodically refreshes the bootstrap file until Stop is called.
func (b *Manager) refreshLoop() {
	defer close(b.stopped)

	ticker := time.NewTicker(defaultRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-b.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			_ = b.Refresh(ctx) // Keep serving stale on failure.
			cancel()
		}
	}
}

// loadFallbacks populates the server map with hardcoded fallback entries.
func (b *Manager) loadFallbacks() {
	b.mu.Lock()
	for tld, url := range fallbackServers {
		b.servers[tld] = url
	}
	b.mu.Unlock()
}

// ianaBootstrap is the decoded IANA RDAP bootstrap JSON structure.
// The "services" field contains arrays of [TLDs..., URLs...] as raw JSON arrays,
// so we decode it as [][]interface{}.
type ianaBootstrap struct {
	Version     string          `json:"version"`
	Publication string          `json:"publication"`
	Services    [][]interface{} `json:"services"`
}

// parseBootstrap decodes the raw IANA bootstrap JSON into a TLD→URL map.
func parseBootstrap(data []byte) (map[string]string, error) {
	var raw ianaBootstrap
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil, err
	}

	servers := make(map[string]string)
	for _, svc := range raw.Services {
		if len(svc) < 2 {
			continue
		}

		var tlds []string
		if arr, ok := svc[0].([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					tlds = append(tlds, s)
				}
			}
		}

		var urls []string
		if arr, ok := svc[1].([]interface{}); ok {
			for _, v := range arr {
				if s, ok := v.(string); ok {
					urls = append(urls, s)
				}
			}
		}

		if len(urls) == 0 {
			continue
		}
		for _, tld := range tlds {
			servers[tld] = urls[0]
		}
	}

	return servers, nil
}
