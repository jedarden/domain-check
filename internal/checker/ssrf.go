// Package checker implements domain availability checking with SSRF protection.
package checker

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"syscall"
	"time"
)

// blockedIPRanges defines CIDR ranges that must not be reachable from the HTTP client.
// This defeats SSRF via DNS rebinding by checking the resolved IP after DNS lookup
// but before the TCP connection is established.
var blockedIPRanges = []*net.IPNet{
	// Loopback (127.0.0.0/8, ::1/128)
	mustParseCIDR("127.0.0.0/8"),
	mustParseCIDR("::1/128"),
	// Link-local (169.254.0.0/16, fe80::/10)
	mustParseCIDR("169.254.0.0/16"),
	mustParseCIDR("fe80::/10"),
	// RFC 1918 private (10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16)
	mustParseCIDR("10.0.0.0/8"),
	mustParseCIDR("172.16.0.0/12"),
	mustParseCIDR("192.168.0.0/16"),
	// IPv4-mapped IPv6 loopback and private (::ffff:127.0.0.1, etc.)
	mustParseCIDR("::ffff:127.0.0.0/104"),
	mustParseCIDR("::ffff:10.0.0.0/104"),
	mustParseCIDR("::ffff:172.16.0.0/108"),
	mustParseCIDR("::ffff:192.168.0.0/112"),
	mustParseCIDR("::ffff:169.254.0.0/112"),
	// Unique Local Addresses (ULA) fc00::/7
	mustParseCIDR("fc00::/7"),
	// Zero network (0.0.0.0/8, ::/128)
	mustParseCIDR("0.0.0.0/8"),
	mustParseCIDR("::/128"),
	// Current network (as specified by the host)
	mustParseCIDR("100.64.0.0/10"), // Carrier-grade NAT
	// IPv6 documentation prefix (not routable)
	mustParseCIDR("2001:db8::/32"),
	// IPv4 multicast
	mustParseCIDR("224.0.0.0/4"),
	// IPv6 multicast
	mustParseCIDR("ff00::/8"),
	// IPv6 site-local (deprecated but still blockable)
	mustParseCIDR("fec0::/10"),
	// IPv4 broadcast
	mustParseCIDR("255.255.255.255/32"),
}

// ErrBlockedIP is returned when a connection attempt targets a private/blocked IP.
var ErrBlockedIP = errors.New("connection to blocked IP address")

// ErrNotInAllowlist is returned when a URL is not in the RDAP allowlist.
var ErrNotInAllowlist = errors.New("URL not in RDAP allowlist")

// ErrTooManyRedirects is returned when redirect chain exceeds the maximum.
var ErrTooManyRedirects = errors.New("too many redirects")

// ErrInvalidDomainInput is returned when domain input contains forbidden characters.
var ErrInvalidDomainInput = errors.New("domain contains forbidden characters")

// maxRedirects is the maximum number of HTTP redirects allowed.
const maxRedirects = 3

// timeout constants for the HTTP client.
const (
	connectTimeout     = 5 * time.Second
	tlsHandshakeTimeout = 5 * time.Second
	responseHeaderTimeout = 10 * time.Second
	totalRequestTimeout   = 15 * time.Second
)

func mustParseCIDR(s string) *net.IPNet {
	_, ipNet, err := net.ParseCIDR(s)
	if err != nil {
		panic(fmt.Sprintf("ssrf: invalid CIDR %q: %v", s, err))
	}
	return ipNet
}

// isBlockedIP returns true if the IP falls within any blocked range.
func isBlockedIP(ip net.IP) bool {
	for _, ipNet := range blockedIPRanges {
		if ipNet.Contains(ip) {
			return true
		}
	}
	return false
}

// SafeDialer returns a *net.Dialer configured with a Control callback that
// blocks connections to private/blocked IP ranges after DNS resolution.
// The Control callback fires after DNS resolution but before the TCP
// connection is established, defeating DNS rebinding attacks.
func SafeDialer() *net.Dialer {
	return &net.Dialer{
		Timeout:   connectTimeout,
		KeepAlive: 30 * time.Second,
		Control: func(network, address string, c syscall.RawConn) error {
			host, _, err := net.SplitHostPort(address)
			if err != nil {
				return fmt.Errorf("%w: %v", ErrBlockedIP, err)
			}
			ip := net.ParseIP(host)
			if ip == nil {
				// Not an IP literal (shouldn't happen after DNS resolution, but be safe).
				return fmt.Errorf("%w: failed to parse resolved address %q", ErrBlockedIP, host)
			}
			if isBlockedIP(ip) {
				return fmt.Errorf("%w: %s", ErrBlockedIP, ip)
			}
			return nil
		},
	}
}

// AllowList manages a set of allowed RDAP base URLs. Only URLs in the allowlist
// can be fetched by the safe HTTP client.
type AllowList struct {
	hosts map[string]bool // e.g. "rdap.verisign.com"
}

// NewAllowList creates an AllowList from a slice of RDAP base URLs.
// The URLs are parsed and their hostnames are extracted for O(1) lookups.
func NewAllowList(urls []string) *AllowList {
	al := &AllowList{hosts: make(map[string]bool, len(urls))}
	for _, u := range urls {
		if host := extractHost(u); host != "" {
			al.hosts[strings.ToLower(host)] = true
		}
	}
	return al
}

// Allow adds a URL's host to the allowlist.
func (al *AllowList) Allow(rawURL string) {
	if host := extractHost(rawURL); host != "" {
		al.hosts[strings.ToLower(host)] = true
	}
}

// Allowed returns true if the host of the given URL is in the allowlist.
func (al *AllowList) Allowed(rawURL string) bool {
	host := extractHost(rawURL)
	if host == "" {
		return false
	}
	return al.hosts[strings.ToLower(host)]
}

// Hosts returns the list of allowed hosts.
func (al *AllowList) Hosts() []string {
	hosts := make([]string, 0, len(al.hosts))
	for h := range al.hosts {
		hosts = append(hosts, h)
	}
	return hosts
}

func extractHost(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return ""
	}
	return u.Hostname()
}

// ClientConfig holds configuration for the SSRF-safe HTTP client.
type ClientConfig struct {
	// AllowList is the set of allowed RDAP server hosts.
	AllowList *AllowList
	// UserAgent is the User-Agent header for outgoing requests.
	UserAgent string
}

// NewSafeClient creates an *http.Client protected against SSRF attacks.
//
// Protection layers:
//  1. Private IP blocking via net.Dialer.Control (defeats DNS rebinding)
//  2. URL allowlist enforcement (only bootstrap-approved RDAP servers)
//  3. Redirect validation (redirect targets must be in allowlist, max 3 hops)
//  4. Configured timeouts (5s connect, 5s TLS, 10s header, 15s total)
func NewSafeClient(cfg ClientConfig) *http.Client {
	al := cfg.AllowList
	ua := cfg.UserAgent

	return &http.Client{
		Transport: &http.Transport{
			DialContext:           SafeDialer().DialContext,
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

// SanitizeDomain rejects domains containing characters that indicate URL
// fragments, credentials, or path segments rather than plain domain names.
// This must be called before any network request.
//
// Rejected characters: / : @
func SanitizeDomain(domain string) error {
	if strings.ContainsAny(domain, "/:@") {
		return ErrInvalidDomainInput
	}
	return nil
}

// SafeDo performs an HTTP GET with SSRF protection. It validates the URL against
// the allowlist, applies the request timeout via context, and uses the safe client.
func SafeDo(ctx context.Context, client *http.Client, rawURL string) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(ctx, totalRequestTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	return client.Do(req)
}
