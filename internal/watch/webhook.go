package watch

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"syscall"
	"time"
)

// WebhookPayload is the JSON payload sent to webhook URLs.
type WebhookPayload struct {
	Domain    string    `json:"domain"`
	Available bool      `json:"available"`
	CheckedAt time.Time `json:"checked_at"`
}

// WebhookClient handles outbound webhook delivery with SSRF protection.
type WebhookClient struct {
	client    *http.Client
	userAgent string
}

// NewWebhookClient creates a new webhook client with SSRF-safe defaults.
func NewWebhookClient() *WebhookClient {
	return &WebhookClient{
		client: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 30 * time.Second,
					// Control callback for SSRF protection
					Control: func(network, address string, conn syscall.RawConn) error {
						host, _, err := net.SplitHostPort(address)
						if err != nil {
							// If we can't parse, assume it's a hostname (not :port)
							host = address
						}

						ip := net.ParseIP(host)
						if ip == nil {
							// Not an IP address - will be resolved, can't check here
							return nil
						}

						// Block private IPs after DNS resolution (defeats DNS rebinding)
						if isPrivateIP(ip) {
							return fmt.Errorf("webhook blocked: private IP not allowed (%s)", ip)
						}

						return nil
					},
				}).DialContext,
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     90 * time.Second,
			},
			// Prevent following redirects to private IPs
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 3 {
					return fmt.Errorf("too many redirects")
				}

				// Check redirect target
				if req.URL != nil {
					if err := validateWebhookURL(req.URL); err != nil {
						return fmt.Errorf("redirect blocked: %w", err)
					}
				}

				return nil
			},
		},
		userAgent: "DomainCheck/1.0 (+https://github.com/jedarden/domain-check)",
	}
}

// isPrivateIP checks if an IP address is in a private range.
func isPrivateIP(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return true
	}

	if ip4 := ip.To4(); ip4 != nil {
		// RFC 1918: 10.0.0.0/8, 172.16.0.0/12, 192.168.0.0/16
		switch {
		case ip4[0] == 10:
			return true
		case ip4[0] == 172 && ip4[1] >= 16 && ip4[1] <= 31:
			return true
		case ip4[0] == 192 && ip4[1] == 168:
			return true
		}
	}

	if ip16 := ip.To16(); ip16 != nil {
		// RFC 4193: fc00::/7 (Unique Local Addresses)
		if ip16[0]&0xfe == 0xfc {
			return true
		}
	}

	return false
}

// validateWebhookURL validates a webhook URL for SSRF safety.
func validateWebhookURL(u *url.URL) error {
	if u == nil {
		return fmt.Errorf("nil URL")
	}

	// Only allow http and https schemes
	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("invalid scheme: %s (only http/https allowed)", u.Scheme)
	}

	// Block localhost and common local hostnames
	host := u.Hostname()
	lowerHost := bytes.ToLower([]byte(host))
	if bytes.HasPrefix(lowerHost, []byte("localhost")) ||
		bytes.HasSuffix(lowerHost, []byte(".local")) {
		return fmt.Errorf("localhost hostname not allowed")
	}

	// Resolve hostname to IP and check if private
	ips, err := net.LookupIP(host)
	if err != nil {
		return fmt.Errorf("DNS resolution failed: %w", err)
	}

	for _, ip := range ips {
		if isPrivateIP(ip) {
			return fmt.Errorf("hostname resolves to private IP: %s -> %s", host, ip)
		}
	}

	return nil
}

// Deliver sends a webhook payload with HMAC signature.
// Returns HTTP status code or error if delivery failed.
func (c *WebhookClient) Deliver(ctx context.Context, webhookURL, secret string, payload *WebhookPayload) (int, error) {
	// Parse and validate webhook URL before attempting connection
	parsedURL, err := url.Parse(webhookURL)
	if err != nil {
		return 0, fmt.Errorf("parse webhook URL: %w", err)
	}
	if err := validateWebhookURL(parsedURL); err != nil {
		return 0, fmt.Errorf("webhook URL validation failed: %w", err)
	}

	// Serialize payload
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, fmt.Errorf("marshal payload: %w", err)
	}

	// Generate HMAC-SHA256 signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("X-DomainCheck-Signature", "sha256="+signature)

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read body to allow connection reuse
	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode, nil
}

// VerifySignature verifies an HMAC-SHA256 signature.
// Useful for testing and webhook receiver validation.
func VerifySignature(payload []byte, secret string, signatureHeader string) bool {
	// Extract signature from header (format: sha256=<hex>)
	sigPrefix := "sha256="
	if len(signatureHeader) <= len(sigPrefix) {
		return false
	}
	signature := signatureHeader[len(sigPrefix):]

	// Compute expected signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expected := hex.EncodeToString(mac.Sum(nil))

	// Constant-time comparison to prevent timing attacks
	return hmac.Equal([]byte(signature), []byte(expected))
}
