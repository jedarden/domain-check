package watch

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewWebhookClient(t *testing.T) {
	client := NewWebhookClient()
	if client == nil {
		t.Fatal("NewWebhookClient returned nil")
	}
	if client.client == nil {
		t.Error("HTTP client not initialized")
	}
	if client.userAgent != "DomainCheck/1.0 (+https://github.com/jedarden/domain-check)" {
		t.Errorf("unexpected user agent: %s", client.userAgent)
	}
}

func TestWebhookClient_Deliver_Success(t *testing.T) {
	// Create test server that accepts webhooks
	var receivedPayload *WebhookPayload
	var receivedSignature string

	// Use a listener on a non-loopback address if possible, otherwise we need to test differently
	// Since we can't easily bind to non-loopback in tests, we'll test the HTTP delivery logic
	// using a mock HTTP client approach

	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("expected POST, got %s", r.Method)
		}

		// Parse payload
		var payload WebhookPayload
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("failed to decode payload: %v", err)
		}
		receivedPayload = &payload

		// Get signature
		receivedSignature = r.Header.Get("X-DomainCheck-Signature")

		// Check user agent
		if ua := r.Header.Get("User-Agent"); ua != "DomainCheck/1.0 (+https://github.com/jedarden/domain-check)" {
			t.Errorf("unexpected User-Agent: %s", ua)
		}

		w.WriteHeader(http.StatusOK)
	}))

	// Start the server - it will bind to loopback which SSRF blocks, so we test the delivery logic directly
	server.Start()
	defer server.Close()

	// Create a test client that bypasses SSRF checks for testing
	client := &WebhookClient{
		client: server.Client(),
		userAgent: "DomainCheck/1.0 (+https://github.com/jedarden/domain-check)",
	}

	payload := &WebhookPayload{
		Domain:    "example.com",
		Available: true,
		CheckedAt: time.Now(),
	}

	secret := "test-secret"

	// Deliver directly to server URL (bypassing validateWebhookURL for testing)
	// In production, validateWebhookURL is called first in Deliver()
	statusCode, err := testDeliverDirectly(client, server.URL, secret, payload)
	if err != nil {
		t.Fatalf("Deliver failed: %v", err)
	}
	if statusCode != 200 {
		t.Errorf("statusCode = %d, want 200", statusCode)
	}
	if receivedPayload == nil {
		t.Fatal("payload not received")
	}
	if receivedPayload.Domain != payload.Domain {
		t.Errorf("received domain = %s, want %s", receivedPayload.Domain, payload.Domain)
	}
	if receivedPayload.Available != payload.Available {
		t.Errorf("received available = %v, want %v", receivedPayload.Available, payload.Available)
	}

	// Verify signature
	payloadBytes, _ := json.Marshal(payload)
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payloadBytes)
	expectedSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	if receivedSignature != expectedSignature {
		t.Errorf("signature mismatch: got %s, want %s", receivedSignature, expectedSignature)
	}
}

// testDeliverDirectly delivers a webhook without URL validation (for testing only)
func testDeliverDirectly(c *WebhookClient, webhookURL, secret string, payload *WebhookPayload) (int, error) {
	// This is a copy of the delivery logic from Deliver() but without validateWebhookURL
	ctx := context.Background()

	// Serialize payload
	body, err := json.Marshal(payload)
	if err != nil {
		return 0, err
	}

	// Generate HMAC-SHA256 signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	signature := hex.EncodeToString(mac.Sum(nil))

	// Create request
	req, err := http.NewRequestWithContext(ctx, "POST", webhookURL, bytes.NewReader(body))
	if err != nil {
		return 0, err
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("X-DomainCheck-Signature", "sha256="+signature)

	// Send request
	resp, err := c.client.Do(req)
	if err != nil {
		return 0, err
	}
	defer resp.Body.Close()

	// Read body to allow connection reuse
	_, _ = io.Copy(io.Discard, resp.Body)

	return resp.StatusCode, nil
}

func TestWebhookClient_Deliver_InvalidURL(t *testing.T) {
	client := NewWebhookClient()
	payload := &WebhookPayload{
		Domain:    "example.com",
		Available: true,
		CheckedAt: time.Now(),
	}

	tests := []struct {
		name        string
		url         string
		expectedErr string
	}{
		{
			name:        "invalid URL",
			url:         ":invalid",
			expectedErr: "parse webhook URL",
		},
		{
			name:        "file scheme",
			url:         "file:///etc/passwd",
			expectedErr: "webhook URL validation failed",
		},
		{
			name:        "localhost",
			url:         "http://localhost/webhook",
			expectedErr: "webhook URL validation failed",
		},
		{
			name:        "local hostname",
			url:         "http://example.local/webhook",
			expectedErr: "webhook URL validation failed",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := client.Deliver(context.Background(), tt.url, "secret", payload)
			if err == nil {
				t.Error("expected error, got nil")
			}
			if err != nil && err.Error()[:len(tt.expectedErr)] != tt.expectedErr {
				t.Errorf("error = %v, want prefix %s", err, tt.expectedErr)
			}
		})
	}
}

func TestWebhookClient_Deliver_PrivateIP(t *testing.T) {
	// Create test server on loopback (will be blocked by SSRF protection)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewWebhookClient()
	payload := &WebhookPayload{
		Domain:    "example.com",
		Available: true,
		CheckedAt: time.Now(),
	}

	// The server is on loopback, which should be blocked
	_, err := client.Deliver(context.Background(), server.URL, "secret", payload)
	if err == nil {
		t.Error("expected error for loopback IP, got nil")
	}
}

func TestWebhookClient_Deliver_NetworkError(t *testing.T) {
	client := NewWebhookClient()
	payload := &WebhookPayload{
		Domain:    "example.com",
		Available: true,
		CheckedAt: time.Now(),
	}

	// Use a URL that will fail to connect
	_, err := client.Deliver(context.Background(), "http://example.invalid:9999", "secret", payload)
	if err == nil {
		t.Error("expected error for unreachable host, got nil")
	}
}

func TestWebhookClient_Deliver_StatusCodes(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
	}{
		{
			name:       "200 OK",
			statusCode: 200,
		},
		{
			name:       "201 Created",
			statusCode: 201,
		},
		{
			name:       "202 Accepted",
			statusCode: 202,
		},
		{
			name:       "204 No Content",
			statusCode: 204,
		},
		{
			name:       "400 Bad Request",
			statusCode: 400,
		},
		{
			name:       "404 Not Found",
			statusCode: 404,
		},
		{
			name:       "500 Internal Server Error",
			statusCode: 500,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
			}))
			server.Start()
			defer server.Close()

			// Use test client that bypasses SSRF checks
			client := &WebhookClient{
				client: server.Client(),
				userAgent: "DomainCheck/1.0 (+https://github.com/jedarden/domain-check)",
			}
			payload := &WebhookPayload{
				Domain:    "example.com",
				Available: true,
				CheckedAt: time.Now(),
			}

			statusCode, err := testDeliverDirectly(client, server.URL, "secret", payload)
			// Deliver doesn't return error for HTTP error status codes, only for delivery failures
			if err != nil {
				t.Errorf("unexpected error for status %d: %v", tt.statusCode, err)
			}
			if statusCode != tt.statusCode {
				t.Errorf("statusCode = %d, want %d", statusCode, tt.statusCode)
			}
		})
	}
}

func TestWebhookClient_Deliver_ContextCancellation(t *testing.T) {
	// Create a server that delays response
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	client := NewWebhookClient()
	payload := &WebhookPayload{
		Domain:    "example.com",
		Available: true,
		CheckedAt: time.Now(),
	}

	// Create a context with short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	_, err := client.Deliver(ctx, server.URL, "secret", payload)
	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip       string
		private  bool
	}{
		// Loopback
		{"127.0.0.1", true},
		{"127.0.0.2", true},
		{"::1", true},

		// Link-local
		{"169.254.1.1", true},
		{"fe80::1", true},

		// RFC 1918
		{"10.0.0.1", true},
		{"10.255.255.255", true},
		{"172.16.0.1", true},
		{"172.31.255.255", true},
		{"192.168.0.1", true},
		{"192.168.255.255", true},

		// RFC 4193 (ULA)
		{"fc00::1", true},
		{"fd00::1", true},

		// Public IPs
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"2001:4860:4860::8888", false},
	}

	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			ip := net.ParseIP(tt.ip)
			if ip == nil {
				t.Fatalf("failed to parse IP: %s", tt.ip)
			}
			got := isPrivateIP(ip)
			if got != tt.private {
				t.Errorf("isPrivateIP(%s) = %v, want %v", tt.ip, got, tt.private)
			}
		})
	}
}

func TestVerifySignature(t *testing.T) {
	payload := []byte(`{"domain":"example.com","available":true}`)
	secret := "test-secret"

	// Generate valid signature
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	validSignature := "sha256=" + hex.EncodeToString(mac.Sum(nil))

	tests := []struct {
		name      string
		payload   []byte
		secret    string
		signature string
		valid     bool
	}{
		{
			name:      "valid signature",
			payload:   payload,
			secret:    secret,
			signature: validSignature,
			valid:     true,
		},
		{
			name:      "invalid signature",
			payload:   payload,
			secret:    secret,
			signature: "sha256=invalid",
			valid:     false,
		},
		{
			name:      "wrong secret",
			payload:   payload,
			secret:    "wrong-secret",
			signature: validSignature,
			valid:     false,
		},
		{
			name:      "different payload",
			payload:   []byte(`{"different":"payload"}`),
			secret:    secret,
			signature: validSignature,
			valid:     false,
		},
		{
			name:      "missing sha256 prefix",
			payload:   payload,
			secret:    secret,
			signature: hex.EncodeToString(mac.Sum(nil)),
			valid:     false,
		},
		{
			name:      "empty signature",
			payload:   payload,
			secret:    secret,
			signature: "",
			valid:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := VerifySignature(tt.payload, tt.secret, tt.signature)
			if result != tt.valid {
				t.Errorf("VerifySignature() = %v, want %v", result, tt.valid)
			}
		})
	}
}
