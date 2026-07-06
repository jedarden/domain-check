// Package bootstrap manages the IANA RDAP bootstrap file, which maps TLDs to
// their RDAP server base URLs. It loads the file on startup and refreshes it
// every 24 hours in the background.
package bootstrap

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

// Manager loads, caches, and refreshes the IANA RDAP bootstrap file.
type Manager struct {
	mu      sync.RWMutex
	servers map[string]string // TLD → RDAP server base URL
	updated time.Time
	url     string
	client  *http.Client
	stopCh  chan struct{}
	stopped chan struct{}
}

// NewManager creates a Manager that fetches the IANA bootstrap
// file from the given URL. If url is empty, the default IANA URL is used.
// It performs an initial fetch synchronously and starts a background refresh goroutine.
func NewManager(ctx context.Context, url string) (*Manager, error) {
	if url == "" {
		url = defaultBootstrapURL
	}

	m := &Manager{
		servers: make(map[string]string),
		url:     url,
		client:  &http.Client{Timeout: 30 * time.Second},
		stopCh:  make(chan struct{}),
		stopped: make(chan struct{}),
	}

	// Initial fetch — use fallbacks on failure.
	if err := m.Refresh(ctx); err != nil {
		m.loadFallbacks()
	}

	go m.refreshLoop()

	return m, nil
}

// Refresh fetches and parses the IANA bootstrap file, updating the TLD→URL map.
func (m *Manager) Refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, m.url, nil)
	if err != nil {
		return fmt.Errorf("create bootstrap request: %w", err)
	}

	resp, err := m.client.Do(req)
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

	m.mu.Lock()
	m.servers = servers
	m.updated = time.Now()
	m.mu.Unlock()

	return nil
}

// Lookup returns the RDAP server base URL for the given TLD.
// It returns ErrTLDNotFound if no server is known for the TLD.
func (m *Manager) Lookup(tld string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	url, ok := m.servers[tld]
	if !ok {
		return "", fmt.Errorf("%w: %s", ErrTLDNotFound, tld)
	}
	return url, nil
}

// Updated returns the time of the last successful bootstrap refresh.
func (m *Manager) Updated() time.Time {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.updated
}

// ServerCount returns the number of TLDs currently mapped.
func (m *Manager) ServerCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.servers)
}

// URLs returns all RDAP server base URLs currently mapped.
// The returned slice is a copy and is safe for the caller to modify.
func (m *Manager) URLs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// Use a map to deduplicate URLs (multiple TLDs may share the same server).
	seen := make(map[string]bool, len(m.servers))
	urls := make([]string, 0, len(m.servers))
	for _, url := range m.servers {
		if !seen[url] {
			seen[url] = true
			urls = append(urls, url)
		}
	}
	return urls
}

// TLDs returns all TLDs currently mapped.
// The returned slice is a copy and is safe for the caller to modify.
func (m *Manager) TLDs() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tlds := make([]string, 0, len(m.servers))
	for tld := range m.servers {
		tlds = append(tlds, tld)
	}
	return tlds
}

// InjectServers replaces the server map; intended for testing only.
func (m *Manager) InjectServers(servers map[string]string) {
	m.mu.Lock()
	m.servers = servers
	m.mu.Unlock()
}

// Stop terminates the background refresh goroutine.
func (m *Manager) Stop() {
	close(m.stopCh)
	<-m.stopped
}

// refreshLoop periodically refreshes the bootstrap file until Stop is called.
func (m *Manager) refreshLoop() {
	defer close(m.stopped)

	ticker := time.NewTicker(defaultRefreshInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			_ = m.Refresh(ctx) // Keep serving stale on failure.
			cancel()
		}
	}
}

// loadFallbacks populates the server map with hardcoded fallback entries.
func (m *Manager) loadFallbacks() {
	m.mu.Lock()
	for tld, url := range fallbackServers {
		m.servers[tld] = url
	}
	m.mu.Unlock()
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
