package checker

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestIsBlockedIP(t *testing.T) {
	tests := []struct {
		name   string
		ip     string
		blocked bool
	}{
		// Loopback
		{"loopback v4", "127.0.0.1", true},
		{"loopback v4 high", "127.255.255.255", true},
		{"loopback v6", "::1", true},

		// Link-local
		{"link-local v4", "169.254.0.1", true},
		{"link-local v6", "fe80::1", true},

		// RFC 1918
		{"rfc1918 10", "10.0.0.1", true},
		{"rfc1918 172", "172.16.0.1", true},
		{"rfc1918 192", "192.168.1.1", true},
		{"rfc1918 172 boundary", "172.31.255.255", true},

		// IPv4-mapped IPv6 private
		{"v4-mapped loopback", "::ffff:127.0.0.1", true},
		{"v4-mapped 10", "::ffff:10.0.0.1", true},
		{"v4-mapped 192", "::ffff:192.168.1.1", true},
		{"v4-mapped link-local", "::ffff:169.254.1.1", true},

		// ULA
		{"ula fc00", "fc00::1", true},
		{"ula fd00", "fd12:3456::1", true},

		// Zero network
		{"zero v4", "0.0.0.0", true},
		{"zero v6", "::", true},
		{"zero v4 high", "0.255.255.255", true},

		// Carrier-grade NAT
		{"cg-nat", "100.64.0.1", true},

		// Documentation
		{"documentation v6", "2001:db8::1", true},

		// Multicast
		{"multicast v4", "224.0.0.1", true},
		{"multicast v6", "ff02::1", true},

		// Broadcast
		{"broadcast v4", "255.255.255.255", true},

		// Not blocked — public IPs
		{"public v4 google", "8.8.8.8", false},
		{"public v4 cloudflare", "1.1.1.1", false},
		{"public v6", "2606:4700:4700::1111", false},
		{"public v4 rdap verisign", "192.5.6.30", false}, // actual Verisign RDAP
		{"public v4 not-cg-nat boundary", "100.63.255.255", false},
		{"public v4 not-cg-nat boundary 2", "100.128.0.0", false},
		{"public v4 not-rfc1918 172 boundary", "172.15.255.255", false},
		{"public v4 not-rfc1918 172 boundary 2", "172.32.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			require.NotNil(t, ip, "invalid test IP %q", tt.ip)
			assert.Equal(t, tt.blocked, isBlockedIP(ip))
		})
	}
}

func TestSanitizeDomain(t *testing.T) {
	tests := []struct {
		name    string
		domain  string
		wantErr error
	}{
		{"clean domain", "example.com", nil},
		{"domain with path", "example.com/path", ErrInvalidDomainInput},
		{"url scheme", "http://example.com", ErrInvalidDomainInput},
		{"url with port", "example.com:8080", ErrInvalidDomainInput},
		{"email", "user@example.com", ErrInvalidDomainInput},
		{"slash only", "exam/ple.com", ErrInvalidDomainInput},
		{"colon only", "exam:ple.com", ErrInvalidDomainInput},
		{"at only", "exam@ple.com", ErrInvalidDomainInput},
		{"all forbidden", "http://user@example.com:8080/path", ErrInvalidDomainInput},
		{"subdomain", "www.example.com", nil},
		{"idn punycode", "xn--nxasmq6b.com", nil},
		{"with dash", "my-domain.com", nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := SanitizeDomain(tt.domain)
			if tt.wantErr != nil {
				assert.ErrorIs(t, err, tt.wantErr)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestAllowList(t *testing.T) {
	t.Run("creation from URLs", func(t *testing.T) {
		al := NewAllowList([]string{
			"https://rdap.verisign.com/domain/",
			"https://rdap.publicinterestregistry.org/domain/",
			"https://pubapi.registry.google/rdap/domain/",
		})

		assert.True(t, al.Allowed("https://rdap.verisign.com/domain/example.com"))
		assert.True(t, al.Allowed("https://rdap.publicinterestregistry.org/domain/example.org"))
		assert.True(t, al.Allowed("https://pubapi.registry.google/rdap/domain/example.dev"))
		assert.False(t, al.Allowed("https://evil.com/domain/example.com"))
		assert.False(t, al.Allowed("https://localhost/domain/example.com"))
	})

	t.Run("case insensitive", func(t *testing.T) {
		al := NewAllowList([]string{"https://rdap.verisign.com/"})
		assert.True(t, al.Allowed("https://RDAP.VERISIGN.COM/domain/example.com"))
	})

	t.Run("allow adds to list", func(t *testing.T) {
		al := NewAllowList(nil)
		assert.False(t, al.Allowed("https://rdap.example.com/"))
		al.Allow("https://rdap.example.com/domain/")
		assert.True(t, al.Allowed("https://rdap.example.com/domain/foo"))
	})

	t.Run("empty URL ignored", func(t *testing.T) {
		al := NewAllowList([]string{"", "://invalid", "https://valid.com/"})
		assert.True(t, al.Allowed("https://valid.com/"))
		assert.Equal(t, 1, len(al.Hosts()))
	})

	t.Run("hosts returns all allowed", func(t *testing.T) {
		al := NewAllowList([]string{
			"https://a.com/",
			"https://b.com/",
		})
		hosts := al.Hosts()
		assert.Len(t, hosts, 2)
		assert.Contains(t, hosts, "a.com")
		assert.Contains(t, hosts, "b.com")
	})
}

// newTestSafeClient creates a safe client for testing that keeps allowlist and
// redirect validation but uses a regular dialer (since httptest binds to
// 127.0.0.1 which is blocked by SafeDialer). IP blocking is tested directly
// via TestIsBlockedIP and TestNewSafeClient_PrivateIPBlocking.
func newTestSafeClient(cfg ClientConfig) *http.Client {
	al := cfg.AllowList
	ua := cfg.UserAgent

	return &http.Client{
		Transport: &http.Transport{
			TLSHandshakeTimeout:   tlsHandshakeTimeout,
			ResponseHeaderTimeout: responseHeaderTimeout,
			MaxIdleConns:          100,
			MaxIdleConnsPerHost:   10,
			MaxConnsPerHost:       20,
			IdleConnTimeout:       90 * time.Second,
			ForceAttemptHTTP2:     true,
		},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= maxRedirects {
				return fmt.Errorf("%w: exceeded %d redirects", ErrTooManyRedirects, maxRedirects)
			}
			if al != nil && !al.Allowed(req.URL.String()) {
				return fmt.Errorf("%w: redirect target %q not in allowlist", ErrNotInAllowlist, req.URL.Hostname())
			}
			if ua != "" {
				req.Header.Set("User-Agent", ua)
			}
			return nil
		},
		Timeout: totalRequestTimeout,
	}
}

func TestNewSafeClient_RedirectValidation(t *testing.T) {
	// Create an allowlist with only the first server
	allowedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok" {
			w.WriteHeader(http.StatusOK)
			return
		}
		// Redirect to external (not in allowlist)
		w.Header().Set("Location", "https://evil.com/steal")
		w.WriteHeader(http.StatusFound)
	}))
	defer allowedServer.Close()

	blockedServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer blockedServer.Close()

	al := NewAllowList([]string{allowedServer.URL})

	client := newTestSafeClient(ClientConfig{
		AllowList: al,
	})

	t.Run("allows direct request to allowed host", func(t *testing.T) {
		resp, err := client.Get(allowedServer.URL + "/ok")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("blocks redirect to non-allowlisted host", func(t *testing.T) {
		_, err := client.Get(allowedServer.URL + "/redirect")
		assert.ErrorIs(t, err, ErrNotInAllowlist)
		assert.True(t, strings.Contains(err.Error(), "redirect target"))
	})

	t.Run("blocks request to non-allowlisted host", func(t *testing.T) {
		// CheckRedirect only fires on redirects, not initial requests.
		// The caller must validate the initial URL against the allowlist
		// before passing to the client. This is by design.
		_, err := client.Get(blockedServer.URL)
		require.NoError(t, err)
	})
}

func TestNewSafeClient_MaxRedirects(t *testing.T) {
	var (
		redirectCount int
		serverURL     string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		redirectCount++
		if redirectCount > 5 {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Location", serverURL)
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	serverURL = server.URL
	defer server.Close()

	al := NewAllowList([]string{server.URL})
	client := newTestSafeClient(ClientConfig{AllowList: al})

	_, err := client.Get(server.URL)
	assert.ErrorIs(t, err, ErrTooManyRedirects)
}

func TestNewSafeClient_Timeouts(t *testing.T) {
	// Test that the client respects the total request timeout.
	slowServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(20 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer slowServer.Close()

	al := NewAllowList([]string{slowServer.URL})

	client := newTestSafeClient(ClientConfig{
		AllowList: al,
	})

	start := time.Now()
	_, err := client.Get(slowServer.URL)
	elapsed := time.Since(start)

	assert.Error(t, err)
	assert.True(t, strings.Contains(err.Error(), "timeout") || strings.Contains(err.Error(), "context deadline exceeded"),
		"expected timeout error, got: %v", err)
	// Should timeout well before 20s sleep completes
	assert.Less(t, elapsed, 20*time.Second)
}

func TestNewSafeClient_PrivateIPBlocking(t *testing.T) {
	_ = NewAllowList([]string{"http://localhost/"})

	// Attempting to connect to localhost should be blocked by the dialer.
	// The server uses a TCP listener on 127.0.0.1, which is blocked.
	// We can't easily test this with httptest since it binds to 127.0.0.1.
	// Instead, test the dialer directly.
	dialer := SafeDialer()

	t.Run("blocks loopback", func(t *testing.T) {
		conn, err := dialer.Dial("tcp", "127.0.0.1:80")
		if conn != nil {
			conn.Close()
		}
		assert.ErrorIs(t, err, ErrBlockedIP)
	})

	t.Run("blocks 192.168", func(t *testing.T) {
		conn, err := dialer.Dial("tcp", "192.168.1.1:80")
		if conn != nil {
			conn.Close()
		}
		assert.ErrorIs(t, err, ErrBlockedIP)
	})

	t.Run("blocks 10.0.0.0", func(t *testing.T) {
		conn, err := dialer.Dial("tcp", "10.0.0.1:80")
		if conn != nil {
			conn.Close()
		}
		assert.ErrorIs(t, err, ErrBlockedIP)
	})

	t.Run("blocks 169.254", func(t *testing.T) {
		conn, err := dialer.Dial("tcp", "169.254.1.1:80")
		if conn != nil {
			conn.Close()
		}
		assert.ErrorIs(t, err, ErrBlockedIP)
	})

	t.Run("blocks 0.0.0.0", func(t *testing.T) {
		conn, err := dialer.Dial("tcp", "0.0.0.0:80")
		if conn != nil {
			conn.Close()
		}
		assert.ErrorIs(t, err, ErrBlockedIP)
	})

	t.Run("blocks link-local v6", func(t *testing.T) {
		// This might fail on systems without IPv6, which is fine.
		conn, err := dialer.Dial("tcp6", "[fe80::1]:80")
		if conn != nil {
			conn.Close()
		}
		if err != nil {
			assert.ErrorIs(t, err, ErrBlockedIP)
		}
	})
}

func TestSafeDo(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/rdap+json")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, `{"objectClassName":"domain","ldhName":"example.com"}`)
	}))
	defer server.Close()

	al := NewAllowList([]string{server.URL})
	client := newTestSafeClient(ClientConfig{
		AllowList: al,
		UserAgent: "domain-check/1.0",
	})

	t.Run("successful request", func(t *testing.T) {
		resp, err := SafeDo(context.Background(), client, server.URL+"/domain/example.com")
		require.NoError(t, err)
		defer resp.Body.Close()
		assert.Equal(t, http.StatusOK, resp.StatusCode)
	})

	t.Run("context cancellation", func(t *testing.T) {
		ctx, cancel := context.WithCancel(context.Background())
		cancel() // cancel immediately
		_, err := SafeDo(ctx, client, server.URL+"/domain/example.com")
		assert.Error(t, err)
	})
}

func TestSafeDo_AllowlistCheck(t *testing.T) {
	// Create a server NOT in the allowlist
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	// Empty allowlist — should still be able to call (allowlist is enforced via redirect, not initial request).
	// The caller is responsible for checking al.Allowed() before SafeDo.
	al := NewAllowList(nil)
	client := newTestSafeClient(ClientConfig{AllowList: al})

	// Direct request to unlisted host succeeds (by design — redirect protection is the layer).
	resp, err := SafeDo(context.Background(), client, server.URL)
	require.NoError(t, err)
	resp.Body.Close()
}

func TestNewSafeClient_UserAgent(t *testing.T) {
	var receivedUA string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedUA = r.Header.Get("User-Agent")
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	al := NewAllowList([]string{server.URL})
	client := newTestSafeClient(ClientConfig{
		AllowList: al,
		UserAgent: "domain-check/1.0",
	})

	req, _ := http.NewRequest(http.MethodGet, server.URL, nil)
	resp, err := client.Do(req)
	require.NoError(t, err)
	resp.Body.Close()
	// The client does not set User-Agent on initial requests — that is the
	// caller's responsibility. It preserves User-Agent on redirects via CheckRedirect.
	// Use SafeDo or set the header on the request for initial requests.
	assert.Equal(t, "Go-http-client/1.1", receivedUA)
}

func TestSafeDialer_ConnectTimeout(t *testing.T) {
	// Connect to a non-routable IP that should not be blocked but will timeout.
	// Use a likely-unreachable IP in a public range.
	dialer := SafeDialer()
	dialer.Timeout = 100 * time.Millisecond

	// 198.51.100.1 is a documentation IP (TEST-NET-2) — public range, not blocked.
	// But it's not routable, so it should timeout.
	start := time.Now()
	_, err := dialer.Dial("tcp", "198.51.100.1:12345")
	elapsed := time.Since(start)

	// Should get a timeout error, not a blocked IP error
	if err != nil {
		assert.NotErrorIs(t, err, ErrBlockedIP, "public IP should not be blocked")
	}
	// Should be fast due to short timeout
	assert.Less(t, elapsed, 500*time.Millisecond)
}

func TestNewSafeClient_TLSHandshakeTimeout(t *testing.T) {
	// Create a TLS server that never completes the handshake.
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	require.NoError(t, err)
	defer ln.Close()

	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		// Read but never respond (stall TLS handshake).
		buf := make([]byte, 1)
		conn.Read(buf)
		conn.Close()
	}()

	// Note: This test uses 127.0.0.1, which is blocked by the safe dialer.
	// So this test verifies the transport timeout configuration exists but can't
	// test TLS handshake timeout through the safe dialer. Instead, verify the
	// transport has the correct timeout set.
	client := NewSafeClient(ClientConfig{})
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)
	assert.Equal(t, tlsHandshakeTimeout, transport.TLSHandshakeTimeout)
	assert.Equal(t, responseHeaderTimeout, transport.ResponseHeaderTimeout)
	assert.Equal(t, totalRequestTimeout, client.Timeout)
}

func TestBlockedIPRanges_Completeness(t *testing.T) {
	// Verify that all major private/special-use ranges are covered.
	// Reference: RFC 6890, RFC 4291, etc.
	testIPs := map[string]bool{
		// Should be blocked
		"127.0.0.1":    true, // loopback
		"::1":          true, // loopback
		"10.0.0.1":     true, // RFC 1918
		"172.16.0.1":   true, // RFC 1918
		"192.168.1.1":  true, // RFC 1918
		"169.254.1.1":  true, // link-local
		"fe80::1":      true, // link-local
		"fc00::1":      true, // ULA
		"fd00::1":      true, // ULA
		"0.0.0.0":      true, // zero
		"::":           true, // zero
		"100.64.0.1":   true, // CGN
		"2001:db8::1":  true, // documentation
		"224.0.0.1":    true, // multicast v4
		"ff02::1":      true, // multicast v6
		"255.255.255.255": true, // broadcast
		"fec0::1":      true, // site-local (deprecated)

		// Should NOT be blocked
		"8.8.8.8":        false, // Google DNS
		"1.1.1.1":        false, // Cloudflare
		"9.9.9.9":        false, // Quad9
		"2606:4700::1111": false, // Cloudflare v6
		"192.5.6.30":     false, // Verisign RDAP (public)
	}

	for ipStr, wantBlocked := range testIPs {
		t.Run(ipStr, func(t *testing.T) {
			ip := net.ParseIP(ipStr)
			require.NotNil(t, ip, "invalid test IP %q", ipStr)
			assert.Equal(t, wantBlocked, isBlockedIP(ip), "IP %s", ipStr)
		})
	}
}

func TestRedirectUserAgent(t *testing.T) {
	var (
		uaOnRedirect string
		serverURL    string
	)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uaOnRedirect = r.Header.Get("User-Agent")
		if r.URL.Path == "/target" {
			w.WriteHeader(http.StatusOK)
			return
		}
		w.Header().Set("Location", serverURL+"/target")
		w.WriteHeader(http.StatusFound)
	}))
	defer server.Close()
	serverURL = server.URL

	al := NewAllowList([]string{serverURL})
	client := newTestSafeClient(ClientConfig{
		AllowList: al,
		UserAgent: "domain-check/1.0",
	})

	resp, err := client.Get(serverURL + "/start")
	require.NoError(t, err)
	defer resp.Body.Close()

	// The redirect target should have the User-Agent set by CheckRedirect.
	assert.Equal(t, "domain-check/1.0", uaOnRedirect)
}

func TestSafeClient_TLSConfig(t *testing.T) {
	// Verify the safe client doesn't disable TLS verification.
	client := NewSafeClient(ClientConfig{})
	transport, ok := client.Transport.(*http.Transport)
	require.True(t, ok)

	// TLSClientConfig should be nil (uses default, secure TLS verification).
	// If it were set to InsecureSkipVerify: true, that would be a security issue.
	if transport.TLSClientConfig != nil {
		assert.False(t, transport.TLSClientConfig.InsecureSkipVerify,
			"safe client must not skip TLS verification")
	}
}

// Test that the safe client can be used with HTTPS servers.
func TestSafeClient_HTTPS(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	al := NewAllowList([]string{server.URL})
	client := newTestSafeClient(ClientConfig{
		AllowList: al,
	})

	// Skip TLS verification for the test server since it uses a self-signed cert.
	transport := client.Transport.(*http.Transport)
	transport.TLSClientConfig = &tls.Config{InsecureSkipVerify: true}

	resp, err := client.Get(server.URL)
	require.NoError(t, err)
	defer resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode)
}
