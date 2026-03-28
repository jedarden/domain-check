package checker

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const sampleBootstrap = `{
  "version": "1.0",
  "publication": "2026-03-17T18:19:24Z",
  "services": [
    [["com", "net"], ["https://rdap.verisign.com/com/v1/"]],
    [["org"], ["https://rdap.publicinterestregistry.org/rdap/"]],
    [["dev", "app"], ["https://pubapi.registry.google/rdap/"]],
    [["co.uk", "org.uk"], ["https://rdap.nominet.uk/rdap/"]],
    [["io"], ["https://rdap.centralnic.com/rdap/", "https://rdap.centralnic.com/io/rdap/"]],
    [["xyz"], []]
  ]
}`

func TestParseBootstrap(t *testing.T) {
	servers, err := parseBootstrap([]byte(sampleBootstrap))
	require.NoError(t, err)

	// Basic TLD lookups.
	assert.Equal(t, "https://rdap.verisign.com/com/v1/", servers["com"])
	assert.Equal(t, "https://rdap.verisign.com/com/v1/", servers["net"])
	assert.Equal(t, "https://rdap.publicinterestregistry.org/rdap/", servers["org"])

	// Multiple TLDs mapped to same server.
	assert.Equal(t, "https://pubapi.registry.google/rdap/", servers["dev"])
	assert.Equal(t, "https://pubapi.registry.google/rdap/", servers["app"])

	// Two-part TLDs.
	assert.Equal(t, "https://rdap.nominet.uk/rdap/", servers["co.uk"])
	assert.Equal(t, "https://rdap.nominet.uk/rdap/", servers["org.uk"])

	// Multiple URLs — should use the first.
	assert.Equal(t, "https://rdap.centralnic.com/rdap/", servers["io"])

	// Empty URLs entry — should be skipped.
	_, ok := servers["xyz"]
	assert.False(t, ok, "TLD with empty URLs should not be in map")
}

func TestParseBootstrap_MalformedJSON(t *testing.T) {
	_, err := parseBootstrap([]byte("not json"))
	require.Error(t, err)
}

func TestParseBootstrap_Empty(t *testing.T) {
	data, _ := json.Marshal(map[string]interface{}{
		"version":    "1.0",
		"publication": "2026-01-01T00:00:00Z",
		"services":   []interface{}{},
	})
	servers, err := parseBootstrap(data)
	require.NoError(t, err)
	assert.Empty(t, servers)
}

func TestParseBootstrap_ShortEntries(t *testing.T) {
	data, _ := json.Marshal(map[string]interface{}{
		"version":    "1.0",
		"publication": "2026-01-01T00:00:00Z",
		"services": []interface{}{
			[]interface{}{[]string{"com"}},           // Missing URLs
			[]interface{}{[]string{"org"}, "notarr"}, // URLs not an array
		},
	})
	servers, err := parseBootstrap(data)
	require.NoError(t, err)
	assert.Empty(t, servers)
}

func TestBootstrapManager_Lookup(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleBootstrap))
	}))
	defer srv.Close()

	b, err := NewBootstrapManager(context.Background(), srv.URL)
	require.NoError(t, err)
	defer b.Stop()

	url, err := b.Lookup("com")
	require.NoError(t, err)
	assert.Equal(t, "https://rdap.verisign.com/com/v1/", url)

	url, err = b.Lookup("dev")
	require.NoError(t, err)
	assert.Equal(t, "https://pubapi.registry.google/rdap/", url)
}

func TestBootstrapManager_LookupNotFound(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleBootstrap))
	}))
	defer srv.Close()

	b, err := NewBootstrapManager(context.Background(), srv.URL)
	require.NoError(t, err)
	defer b.Stop()

	_, err = b.Lookup("notexist")
	assert.ErrorIs(t, err, ErrTLDNotFound)
}

func TestBootstrapManager_FallbackOnFetchFailure(t *testing.T) {
	// Server that always fails.
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	b, err := NewBootstrapManager(context.Background(), srv.URL)
	require.NoError(t, err)
	defer b.Stop()

	// Fallback servers should be available.
	url, err := b.Lookup("com")
	require.NoError(t, err)
	assert.Equal(t, "https://rdap.verisign.com/com/v1/", url)

	url, err = b.Lookup("org")
	require.NoError(t, err)
	assert.Equal(t, "https://rdap.publicinterestregistry.org/rdap/", url)

	// Non-fallback TLD should fail.
	_, err = b.Lookup("dev")
	assert.ErrorIs(t, err, ErrTLDNotFound)
}

func TestBootstrapManager_FallbackOnNetworkError(t *testing.T) {
	b, err := NewBootstrapManager(context.Background(), "http://127.0.0.1:1")
	require.NoError(t, err)
	defer b.Stop()

	url, err := b.Lookup("net")
	require.NoError(t, err)
	assert.Equal(t, "https://rdap.verisign.com/net/v1/", url)
}

func TestBootstrapManager_FallbackOnMalformedJSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"version": "1.0", "services": "oops"}`))
	}))
	defer srv.Close()

	b, err := NewBootstrapManager(context.Background(), srv.URL)
	require.NoError(t, err)
	defer b.Stop()

	url, err := b.Lookup("com")
	require.NoError(t, err)
	assert.Equal(t, "https://rdap.verisign.com/com/v1/", url)
}

func TestBootstrapManager_Updated(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleBootstrap))
	}))
	defer srv.Close()

	before := time.Now()
	b, err := NewBootstrapManager(context.Background(), srv.URL)
	require.NoError(t, err)
	defer b.Stop()

	assert.False(t, b.Updated().Before(before))
}

func TestBootstrapManager_ServerCount(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleBootstrap))
	}))
	defer srv.Close()

	b, err := NewBootstrapManager(context.Background(), srv.URL)
	require.NoError(t, err)
	defer b.Stop()

	// com, net, org, dev, app, co.uk, org.uk, io = 8 TLDs
	assert.Equal(t, 8, b.ServerCount())
}

func TestBootstrapManager_RefreshReplacesData(t *testing.T) {
	var count int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count++
		w.Header().Set("Content-Type", "application/json")
		if count == 1 {
			w.Write([]byte(sampleBootstrap))
		} else {
			// Return only .info on second fetch.
			w.Write([]byte(`{
				"version": "1.0",
				"publication": "2026-03-18T00:00:00Z",
				"services": [[["info"], ["https://rdap.donuts.co/rdap/"]]]
			}`))
		}
	}))
	defer srv.Close()

	b, err := NewBootstrapManager(context.Background(), srv.URL)
	require.NoError(t, err)
	defer b.Stop()

	assert.Equal(t, 8, b.ServerCount())

	err = b.Refresh(context.Background())
	require.NoError(t, err)

	assert.Equal(t, 1, b.ServerCount())
	url, err := b.Lookup("info")
	require.NoError(t, err)
	assert.Equal(t, "https://rdap.donuts.co/rdap/", url)

	// Old TLDs should be gone.
	_, err = b.Lookup("com")
	assert.ErrorIs(t, err, ErrTLDNotFound)
}

func TestBootstrapManager_ContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately.

	_, err := NewBootstrapManager(ctx, "http://127.0.0.1:1")
	require.NoError(t, err) // Should fall back gracefully.
}

func TestBootstrapManager_ConcurrentReads(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleBootstrap))
	}))
	defer srv.Close()

	b, err := NewBootstrapManager(context.Background(), srv.URL)
	require.NoError(t, err)
	defer b.Stop()

	var wg sync.WaitGroup
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			url, err := b.Lookup("com")
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}
			if url != "https://rdap.verisign.com/com/v1/" {
				t.Errorf("wrong URL: %s", url)
			}
		}()
	}
	wg.Wait()
}

func TestBootstrapManager_ConcurrentReadWrite(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleBootstrap))
	}))
	defer srv.Close()

	b, err := NewBootstrapManager(context.Background(), srv.URL)
	require.NoError(t, err)
	defer b.Stop()

	var wg sync.WaitGroup

	// Writers refreshing concurrently.
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_ = b.Refresh(context.Background())
		}()
	}

	// Readers looking up concurrently.
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, _ = b.Lookup("com")
			_ = b.ServerCount()
			_ = b.Updated()
		}()
	}

	wg.Wait()
}

func TestBootstrapManager_Stop(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleBootstrap))
	}))
	defer srv.Close()

	b, err := NewBootstrapManager(context.Background(), srv.URL)
	require.NoError(t, err)

	// Stop should not block.
	done := make(chan struct{})
	go func() {
		b.Stop()
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Stop blocked too long")
	}
}

func TestBootstrapManager_HTTPErrors(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{"NotFound", http.StatusNotFound},
		{"ServerError", http.StatusInternalServerError},
		{"Forbidden", http.StatusForbidden},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			defer srv.Close()

			b, err := NewBootstrapManager(context.Background(), srv.URL)
			require.NoError(t, err) // Falls back.
			defer b.Stop()

			// Fallback servers should work.
			url, err := b.Lookup("com")
			require.NoError(t, err)
			assert.Equal(t, fallbackServers["com"], url)
		})
	}
}

func TestBootstrapManager_RefreshLoopStopsOnContextCancel(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(sampleBootstrap))
	}))
	defer srv.Close()

	b, err := NewBootstrapManager(context.Background(), srv.URL)
	require.NoError(t, err)

	// Verify the manager is running.
	assert.Greater(t, b.ServerCount(), 0)

	// Stop should terminate the refresh loop.
	start := time.Now()
	b.Stop()
	elapsed := time.Since(start)

	// Stop should complete quickly (not waiting for ticker).
	assert.Less(t, elapsed, 100*time.Millisecond)
}

func TestBootstrapManager_RefreshOnErrorKeepsOldData(t *testing.T) {
	var callCount int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&callCount, 1) == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.Write([]byte(sampleBootstrap))
		} else {
			w.WriteHeader(http.StatusInternalServerError)
		}
	}))
	defer srv.Close()

	b, err := NewBootstrapManager(context.Background(), srv.URL)
	require.NoError(t, err)
	defer b.Stop()

	// Initial data loaded.
	assert.Equal(t, 8, b.ServerCount())

	// Refresh fails, but data should remain.
	err = b.Refresh(context.Background())
	require.Error(t, err)
	assert.Equal(t, 8, b.ServerCount(), "data should remain after failed refresh")

	// Lookup should still work with old data.
	url, err := b.Lookup("com")
	require.NoError(t, err)
	assert.Equal(t, "https://rdap.verisign.com/com/v1/", url)
}

func TestParseBootstrap_EdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr bool
	}{
		{
			name:    "empty_object",
			input:   `{}`,
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name:    "missing_services",
			input:   `{"version": "1.0"}`,
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name: "service_with_non_string_tld",
			input: `{
				"version": "1.0",
				"services": [
					[[123, "com"], ["https://example.com/"]]
				]
			}`,
			want:    map[string]string{"com": "https://example.com/"},
			wantErr: false,
		},
		{
			name: "service_with_non_string_url",
			input: `{
				"version": "1.0",
				"services": [
					[["com"], [123, "https://example.com/"]]
				]
			}`,
			want:    map[string]string{"com": "https://example.com/"},
			wantErr: false,
		},
		{
			name: "service_with_single_element_array",
			input: `{
				"version": "1.0",
				"services": [
					[["com"]]
				]
			}`,
			want:    map[string]string{},
			wantErr: false,
		},
		{
			name:    "invalid_json",
			input:   `{invalid`,
			want:    nil,
			wantErr: true,
		},
		{
			name: "services_not_array",
			input: `{
				"version": "1.0",
				"services": "not an array"
			}`,
			want:    nil,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseBootstrap([]byte(tt.input))
			if tt.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, tt.want, got)
		})
	}
}

func TestBootstrapManager_LargeBootstrap(t *testing.T) {
	// Generate a large bootstrap file to test size limits.
	var services []string
	for i := 0; i < 1000; i++ {
		services = append(services, fmt.Sprintf(`[["tld%d"], ["https://registry%d.example/"]]`, i, i))
	}
	largeBootstrap := fmt.Sprintf(`{
		"version": "1.0",
		"publication": "2026-03-17T00:00:00Z",
		"services": [%s]
	}`, strings.Join(services, ", "))

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(largeBootstrap))
	}))
	defer srv.Close()

	b, err := NewBootstrapManager(context.Background(), srv.URL)
	require.NoError(t, err)
	defer b.Stop()

	assert.Equal(t, 1000, b.ServerCount())

	url, err := b.Lookup("tld500")
	require.NoError(t, err)
	assert.Equal(t, "https://registry500.example/", url)
}

func TestBootstrapManager_ContextTimeoutDuringRefresh(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond) // Slow response
		w.Write([]byte(sampleBootstrap))
	}))
	defer srv.Close()

	b, err := NewBootstrapManager(context.Background(), srv.URL)
	require.NoError(t, err) // Initial fetch succeeds (30s timeout)
	defer b.Stop()

	// Now try a refresh with a short timeout.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()

	err = b.Refresh(ctx)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "context deadline exceeded")
}
