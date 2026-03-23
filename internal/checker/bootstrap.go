package checker

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"
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

// BootstrapManager loads, caches, and refreshes the IANA RDAP bootstrap file.
type BootstrapManager struct {
	mu      sync.RWMutex
	servers map[string]string // TLD → RDAP server base URL
	updated time.Time
	url     string
	client  *http.Client
	stopCh  chan struct{}
	stopped chan struct{}
}

// NewBootstrapManager creates a BootstrapManager that fetches the IANA bootstrap
// file from the given URL. If url is empty, the default IANA URL is used.
// It performs an initial fetch synchronously and starts a background refresh goroutine.
func NewBootstrapManager(ctx context.Context, url string) (*BootstrapManager, error) {
	if url == "" {
		url = defaultBootstrapURL
	}

	b := &BootstrapManager{
		servers: make(map[string]string),
		url:     url,
		client:  &http.Client{Timeout: 30 * time.Second},
		stopCh:  make(chan struct{}),
		stopped: make(chan struct{}),
	}

	// Initial fetch — use fallbacks on failure.
	if err := b.Refresh(ctx); err != nil {
		b.loadFallbacks()
	}

	go b.refreshLoop()

	return b, nil
}

// Refresh fetches and parses the IANA bootstrap file, updating the TLD→URL map.
func (b *BootstrapManager) Refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, b.url, nil)
	if err != nil {
		return fmt.Errorf("create bootstrap request: %w", err)
	}

	resp, err := b.client.Do(req)
	if err != nil {
		return fmt.Errorf("fetch bootstrap: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("fetch bootstrap: HTTP %d", resp.StatusCode)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return fmt.Errorf("read bootstrap body: %w", err)
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

// Lookup returns the RDAP server base URL for the given TLD.
// It returns ErrTLDNotFound if no server is known for the TLD.
func (b *BootstrapManager) Lookup(tld string) (string, error) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	url, ok := b.servers[tld]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrTLDNotFound, tld)
	}
	return url, nil
}

// Updated returns the time of the last successful bootstrap refresh.
func (b *BootstrapManager) Updated() time.Time {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return b.updated
}

// ServerCount returns the number of TLDs currently mapped.
func (b *BootstrapManager) ServerCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.servers)
}

// Stop terminates the background refresh goroutine.
func (b *BootstrapManager) Stop() {
	close(b.stopCh)
	<-b.stopped
}

// refreshLoop periodically refreshes the bootstrap file until Stop is called.
func (b *BootstrapManager) refreshLoop() {
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
func (b *BootstrapManager) loadFallbacks() {
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
