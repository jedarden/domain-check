package checker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coding/domain-check/internal/domain"
)

// TestRDAPResponseParsing tests parsing of various RDAP response formats.
func TestRDAPResponseParsing(t *testing.T) {
	tests := []struct {
		name       string
		body       string
		wantReg    *domain.Registration
		wantAvail  bool
	}{
		{
			name: "empty body",
			body: "",
			wantReg: &domain.Registration{},
			wantAvail: false,
		},
		{
			name: "error response object",
			body: `{"objectClassName": "error", "errorCode": 404}`,
			wantReg: &domain.Registration{},
			wantAvail: false,
		},
		{
			name: "minimal registered domain",
			body: `{"ldhName": "EXAMPLE.COM", "status": ["client transfer prohibited"]}`,
			wantReg: &domain.Registration{
				Status: []string{"client transfer prohibited"},
			},
			wantAvail: false,
		},
		{
			name: "full verisign format",
			body: `{
				"rdapConformance": ["rdap_level_0"],
				"objectClassName": "domain",
				"handle": "2138514_DOMAIN_COM-VRSN",
				"ldhName": "EXAMPLE.COM",
				"status": ["client delete prohibited", "client transfer prohibited", "client update prohibited"],
				"entities": [
					{
						"handle": "292",
						"ldhName": "MARKMONITOR INC.",
						"roles": ["registrar"],
						"publicIds": [{"type": "IANA Registrar ID", "identifier": "292"}]
					}
				],
				"events": [
					{"eventAction": "registration", "eventDate": "1997-09-15T04:00:00Z"},
					{"eventAction": "expiration", "eventDate": "2028-09-14T04:00:00Z"},
					{"eventAction": "last changed", "eventDate": "2024-08-14T09:15:03Z"}
				],
				"nameservers": [
					{"ldhName": "NS1.EXAMPLE.COM."},
					{"ldhName": "NS2.EXAMPLE.COM."}
				],
				"secureDNS": {"delegationSigned": false}
			}`,
			wantReg: &domain.Registration{
				Registrar: "MARKMONITOR INC.",
				Created:   "1997-09-15T04:00:00Z",
				Expires:   "2028-09-14T04:00:00Z",
				Nameservers: []string{"ns1.example.com", "ns2.example.com"},
				Status:    []string{"client delete prohibited", "client transfer prohibited", "client update prohibited"},
			},
			wantAvail: false,
		},
		{
			name: "PIR format with redacted",
			body: `{
				"rdapConformance": ["rdap_level_0", "icann_rdap_response_profile_0", "icann_rdap_technical_implementation_guide_0"],
				"objectClassName": "domain",
				"ldhName": "example.org",
				"entities": [
					{
						"handle": "1234",
						"ldhName": "Example Registrar LLC",
						"roles": ["registrar"]
					}
				],
				"events": [
					{"eventAction": "registration", "eventDate": "1995-08-14T00:00:00.000Z"},
					{"eventAction": "expiration", "eventDate": "2025-08-13T00:00:00.000Z"}
				],
				"redacted": [
					{"name": "Registry Registrant ID", "description": "Registry Registrant ID is redacted"}
				]
			}`,
			wantReg: &domain.Registration{
				Registrar: "Example Registrar LLC",
				Created:   "1995-08-14T00:00:00Z",
				Expires:   "2025-08-13T00:00:00Z",
			},
			wantAvail: false,
		},
		{
			name: "Google format with reregistration event",
			body: `{
				"rdapConformance": ["rdap_level_0"],
				"objectClassName": "domain",
				"ldhName": "example.dev",
				"entities": [
					{
						"handle": "1234",
						"ldhName": "Google Registrar",
						"roles": ["registrar"]
					}
				],
				"events": [
					{"eventAction": "reregistration", "eventDate": "2020-02-12T00:00:00Z"},
					{"eventAction": "expiration", "eventDate": "2025-02-12T00:00:00Z"}
				],
				"secureDNS": {"zoneSigned": true, "delegationSigned": true}
			}`,
			wantReg: &domain.Registration{
				Registrar: "Google Registrar",
				Created:   "2020-02-12T00:00:00Z",
				Expires:   "2025-02-12T00:00:00Z",
			},
			wantAvail: false,
		},
		{
			name: "Nominet format with trailing dots and microsecond dates",
			body: `{
				"rdapConformance": ["rdap_level_0", "nominet_rdap_profile_0"],
				"objectClassName": "domain",
				"ldhName": "example.uk",
				"entities": [
					{
						"handle": "5678",
						"ldhName": "Nominet UK",
						"roles": ["registrar"]
					}
				],
				"events": [
					{"eventAction": "registration", "eventDate": "2025-10-29T03:51:11.009091Z"},
					{"eventAction": "expiration", "eventDate": "2026-10-29T03:51:11.009091Z"}
				],
				"nameservers": [
					{"ldhName": "NS1.NIC.UK."},
					{"ldhName": "NS2.NIC.UK."}
				]
			}`,
			wantReg: &domain.Registration{
				Registrar: "Nominet UK",
				Created:   "2025-10-29T03:51:11Z",
				Expires:   "2026-10-29T03:51:11Z",
				Nameservers: []string{"ns1.nic.uk", "ns2.nic.uk"},
			},
			wantAvail: false,
		},
		{
			name: "DENIC minimal format",
			body: `{
				"rdapConformance": ["rdap_level_0", "deNIC_rdap_profile_0"],
				"objectClassName": "domain",
				"ldhName": "example.de",
				"entities": [],
				"nameservers": []
			}`,
			wantReg: &domain.Registration{},
			wantAvail: false,
		},
		{
			name: "timezone offset date",
			body: `{
				"objectClassName": "domain",
				"ldhName": "example.de",
				"events": [
					{"eventAction": "registration", "eventDate": "2018-03-12T21:44:25+01:00"},
					{"eventAction": "expiration", "eventDate": "2028-09-14T07:00:00.000+00:00"}
				]
			}`,
			wantReg: &domain.Registration{
				Created: "2018-03-12T20:44:25Z",
				Expires: "2028-09-14T07:00:00Z",
			},
			wantAvail: false,
		},
		{
			name: "unknown event types ignored",
			body: `{
				"objectClassName": "domain",
				"ldhName": "example.br",
				"events": [
					{"eventAction": "delegation check", "eventDate": "2024-01-01T00:00:00Z"},
					{"eventAction": "registration", "eventDate": "2020-01-01T00:00:00Z"},
					{"eventAction": "custom event", "eventDate": "2024-01-01T00:00:00Z"}
				]
			}`,
			wantReg: &domain.Registration{
				Created: "2020-01-01T00:00:00Z",
			},
			wantAvail: false,
		},
		{
			name: "nested registrar entity",
			body: `{
				"objectClassName": "domain",
				"ldhName": "example.xyz",
				"entities": [
					{
						"handle": "registrant1",
						"roles": ["registrant"],
						"entities": [
							{
								"handle": "reg1",
								"ldhName": "CentralNic",
								"roles": ["registrar"]
							}
						]
					}
				]
			}`,
			wantReg: &domain.Registration{
				Registrar: "CentralNic",
			},
			wantAvail: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reg := parseRDAPBody([]byte(tt.body))
			if reg == nil {
				t.Fatal("expected non-nil registration")
			}

			if reg.Registrar != tt.wantReg.Registrar {
				t.Errorf("registrar: got %q, want %q", reg.Registrar, tt.wantReg.Registrar)
			}
			if reg.Created != tt.wantReg.Created {
				t.Errorf("created: got %q, want %q", reg.Created, tt.wantReg.Created)
			}
			if reg.Expires != tt.wantReg.Expires {
				t.Errorf("expires: got %q, want %q", reg.Expires, tt.wantReg.Expires)
			}
			if len(reg.Nameservers) != len(tt.wantReg.Nameservers) {
				t.Errorf("nameservers: got %v, want %v", reg.Nameservers, tt.wantReg.Nameservers)
			}
			for i := range reg.Nameservers {
				if i >= len(tt.wantReg.Nameservers) {
					break
				}
				if reg.Nameservers[i] != tt.wantReg.Nameservers[i] {
					t.Errorf("nameserver[%d]: got %q, want %q", i, reg.Nameservers[i], tt.wantReg.Nameservers[i])
				}
			}
		})
	}
}

// TestParseRDAPDate tests date parsing with various formats.
func TestParseRDAPDate(t *testing.T) {
	tests := []struct {
		input    string
		want     string
	}{
		{"", ""},
		{"1997-09-15T04:00:00Z", "1997-09-15T04:00:00Z"},
		{"2024-08-14T09:15:03Z", "2024-08-14T09:15:03Z"},
		{"1995-08-14T00:00:00.000Z", "1995-08-14T00:00:00Z"},
		{"2025-10-29T03:51:11.009091Z", "2025-10-29T03:51:11Z"},
		{"2018-03-12T21:44:25+01:00", "2018-03-12T20:44:25Z"},
		{"2028-09-14T07:00:00.000+00:00", "2028-09-14T07:00:00Z"},
		{"2014-03-20T12:59:17.0Z", "2014-03-20T12:59:17Z"},
		{"invalid-date", "invalid-date"}, // Falls through to original
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := parseRDAPDate(tt.input)
			if got != tt.want {
				t.Errorf("parseRDAPDate(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestExtractNameservers tests nameserver extraction and normalization.
func TestExtractNameservers(t *testing.T) {
	tests := []struct {
		name string
		input []rdapNameserver
		want  []string
	}{
		{
			name:  "empty",
			input: nil,
			want:  nil,
		},
		{
			name: "with trailing dots",
			input: []rdapNameserver{
				{LDHName: "NS1.EXAMPLE.COM."},
				{LDHName: "NS2.EXAMPLE.COM."},
			},
			want: []string{"ns1.example.com", "ns2.example.com"},
		},
		{
			name: "uppercase normalized",
			input: []rdapNameserver{
				{LDHName: "NS1.GOOGLE.COM"},
				{LDHName: "NS2.GOOGLE.COM"},
			},
			want: []string{"ns1.google.com", "ns2.google.com"},
		},
		{
			name: "already lowercase",
			input: []rdapNameserver{
				{LDHName: "ns1.example.com"},
			},
			want: []string{"ns1.example.com"},
		},
		{
			name: "empty ldhName skipped",
			input: []rdapNameserver{
				{LDHName: "ns1.example.com"},
				{LDHName: ""},
				{LDHName: "ns2.example.com"},
			},
			want: []string{"ns1.example.com", "ns2.example.com"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractNameservers(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("got %v, want %v", got, tt.want)
				return
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("got[%d] = %q, want %q", i, got[i], tt.want[i])
				}
			}
		})
	}
}

// TestBuildRDAPURL tests URL construction.
func TestBuildRDAPURL(t *testing.T) {
	tests := []struct {
		base   string
		domain string
		want   string
	}{
		{"https://rdap.verisign.com/com/v1/", "example.com", "https://rdap.verisign.com/com/v1/domain/example.com"},
		{"https://rdap.verisign.com/com/v1", "example.com", "https://rdap.verisign.com/com/v1/domain/example.com"},
		{"https://rdap.publicinterestregistry.org/rdap/", "example.org", "https://rdap.publicinterestregistry.org/rdap/domain/example.org"},
		{"https://pubapi.registry.google/registration/v1/", "example.dev", "https://pubapi.registry.google/registration/v1/domain/example.dev"},
	}

	for _, tt := range tests {
		t.Run(tt.base, func(t *testing.T) {
			got := buildRDAPURL(tt.base, tt.domain)
			if got != tt.want {
				t.Errorf("buildRDAPURL(%q, %q) = %q, want %q", tt.base, tt.domain, got, tt.want)
			}
		})
	}
}

// TestExtractRegistryHost tests host extraction for rate limiting.
func TestExtractRegistryHost(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"https://rdap.verisign.com/com/v1/", "rdap.verisign.com"},
		{"https://rdap.publicinterestregistry.org/rdap/", "rdap.publicinterestregistry.org"},
		{"https://pubapi.registry.google/registration/v1/", "pubapi.registry.google"},
		{"http://localhost:8080/", "localhost:8080"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractRegistryHost(tt.input)
			if got != tt.want {
				t.Errorf("extractRegistryHost(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// TestRDAPClientCheck tests the full Check flow with a mock server.
func TestRDAPClientCheck(t *testing.T) {
	ctx := context.Background()

	t.Run("registered domain", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			if !strings.HasSuffix(r.URL.Path, "/domain/example.com") {
				t.Errorf("unexpected path: %s", r.URL.Path)
			}
			w.Header().Set("Content-Type", "application/rdap+json")
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"objectClassName": "domain",
				"ldhName": "example.com",
				"entities": [{"ldhName": "Test Registrar", "roles": ["registrar"]}],
				"events": [
					{"eventAction": "registration", "eventDate": "2020-01-01T00:00:00Z"},
					{"eventAction": "expiration", "eventDate": "2025-01-01T00:00:00Z"}
				]
			}`))
		}))
		defer server.Close()

		client := newTestRDAPClient(server)
		result, err := client.Check(ctx, "example.com")
		if err != nil {
			t.Fatalf("Check failed: %v", err)
		}

		if result.Available {
			t.Error("expected domain to be registered")
		}
		if result.Registration == nil || result.Registration.Registrar != "Test Registrar" {
			t.Errorf("expected registrar, got: %+v", result.Registration)
		}
	})

	t.Run("available domain", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
		}))
		defer server.Close()

		client := newTestRDAPClient(server)
		result, err := client.Check(ctx, "available123.com")
		if err != nil {
			t.Fatalf("Check failed: %v", err)
		}

		if !result.Available {
			t.Error("expected domain to be available")
		}
		if result.Registration != nil && result.Registration.Registrar != "" {
			t.Errorf("expected no registrar for available domain, got: %s", result.Registration.Registrar)
		}
	})

	t.Run("rate limited", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusTooManyRequests)
		}))
		defer server.Close()

		client := newTestRDAPClient(server)
		result, err := client.Check(ctx, "ratelimited.com")
		if err != nil {
			t.Fatalf("Check failed: %v", err)
		}

		if result.Error == "" {
			t.Error("expected error for rate limited response")
		}
	})

	t.Run("empty body on 404", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusNotFound)
			// No body, like Verisign
		}))
		defer server.Close()

		client := newTestRDAPClient(server)
		result, err := client.Check(ctx, "empty404.com")
		if err != nil {
			t.Fatalf("Check failed: %v", err)
		}

		if !result.Available {
			t.Error("expected domain to be available (empty 404)")
		}
	})

	t.Run("invalid domain input", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			t.Error("should not make request for invalid domain")
		}))
		defer server.Close()

		client := newTestRDAPClient(server)
		_, err := client.Check(ctx, "http://evil.com")
		if err == nil {
			t.Error("expected error for domain with URL characters")
		}
	})
}

// newTestRDAPClient creates an RDAPClient configured for testing with a mock server.
func newTestRDAPClient(server *httptest.Server) *RDAPClient {
	// Create bootstrap that points to test server
	bootstrap := &BootstrapManager{
		servers: map[string]string{
			"com": server.URL + "/",
		},
		updated: time.Now(),
	}

	allowlist := NewAllowList([]string{server.URL})

	return NewRDAPClient(RDAPClientConfig{
		HTTPClient: server.Client(),
		Bootstrap:  bootstrap,
		RateLimit:  NewRateLimiter(),
		AllowList:  allowlist,
		UserAgent:  "domain-check-test/1.0",
	})
}

// TestFuzzRDAPResponse tests that parsing never panics on random input.
func FuzzParseRDAPBody(f *testing.F) {
	// Seed with known fixtures
	f.Add([]byte(""))
	f.Add([]byte(`{"objectClassName": "domain", "ldhName": "example.com"}`))
	f.Add([]byte(`{"objectClassName": "error"}`))
	f.Add([]byte("not json at all"))

	f.Fuzz(func(t *testing.T, data []byte) {
		// Should never panic
		reg := parseRDAPBody(data)
		if reg == nil {
			t.Error("expected non-nil registration even on invalid input")
		}
	})
}

// TestJSONUnmarshalLenience tests that unknown fields don't cause errors.
func TestJSONUnmarshalLenience(t *testing.T) {
	// Response with many unknown/custom fields
	data := []byte(`{
		"rdapConformance": ["rdap_level_0", "custom_extension_1"],
		"objectClassName": "domain",
		"ldhName": "example.com",
		"customField": "some value",
		"anotherCustom": {"nested": "object"},
		"entities": [
			{
				"handle": "123",
				"roles": ["registrar"],
				"customEntityField": true
			}
		],
		"events": [
			{"eventAction": "registration", "eventDate": "2020-01-01T00:00:00Z", "customEventField": 42}
		],
		"customTopLevel": [1, 2, 3]
	}`)

	var resp rdapResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		t.Fatalf("failed to parse lenient response: %v", err)
	}

	if resp.LDHName != "example.com" {
		t.Errorf("ldhName: got %q, want %q", resp.LDHName, "example.com")
	}
}
