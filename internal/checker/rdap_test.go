package checker

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
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

// TestRDAPFixtureParsing tests parsing of RDAP fixtures from testdata/rdap/.
// Covers 8 registered domain fixtures from various registries.
func TestRDAPFixtureParsing(t *testing.T) {
	// Registered domain fixtures with expected values
	registeredTests := []struct {
		fixture    string
		wantReg    *domain.Registration
	}{
		// Test 1: Parse Verisign registered - UPPERCASE ldhName, no fractional seconds
		{
			fixture: "verisign-google.com.json",
			wantReg: &domain.Registration{
				Registrar: "MARKMONITOR INC.",
				Created:   "1997-09-15T04:00:00Z",
				Expires:   "2028-09-14T04:00:00Z",
				Nameservers: []string{"ns1.google.com", "ns2.google.com", "ns3.google.com", "ns4.google.com"},
				Status:    []string{"client delete prohibited", "client transfer prohibited", "client update prohibited"},
			},
		},
		// Test 2: Parse PIR registered - redacted array, millisecond dates
		{
			fixture: "pir-wikipedia.org.json",
			wantReg: &domain.Registration{
				Registrar: "MarkMonitor Inc.",
				Created:   "2001-01-13T00:00:00Z",
				Expires:   "2025-01-13T00:00:00Z",
				Nameservers: []string{"ns0.wikimedia.org", "ns1.wikimedia.org", "ns2.wikimedia.org"},
			},
		},
		// Test 3: Parse Google registered - reregistration event type
		{
			fixture: "google-web.dev.json",
			wantReg: &domain.Registration{
				Registrar: "Google Domains LLC",
				Created:   "2018-12-10T00:00:00Z", // reregistration event
				Expires:   "2025-12-10T00:00:00Z",
				Nameservers: []string{"ns1.google.com", "ns2.google.com", "ns3.google.com", "ns4.google.com"},
			},
		},
		// Test 4: Parse CentralNic - 1-digit fractional second
		{
			fixture: "centralnic-example.xyz.json",
			wantReg: &domain.Registration{
				Registrar: "NameCheap, Inc.",
				Created:   "2014-03-20T12:59:17Z",
				Expires:   "2025-03-20T23:59:59Z",
				Nameservers: []string{"ns1.example.com", "ns2.example.com"},
			},
		},
		// Test 5: Parse Identity Digital - self-referencing link
		{
			fixture: "identity-digital-nic.live.json",
			wantReg: &domain.Registration{
				Registrar: "Identity Digital Inc.",
				Created:   "2015-07-16T14:55:48Z",
				Expires:   "2025-07-16T14:55:48Z",
				Nameservers: []string{"ns1.identitydigital.com", "ns2.identitydigital.com"},
			},
		},
		// Test 6: Parse Nominet - trailing dots on nameservers, microsecond dates
		{
			fixture: "nominet-bbc.uk.json",
			wantReg: &domain.Registration{
				Registrar: "Nominet UK",
				Created:   "1996-08-01T00:00:00Z",
				Expires:   "2025-08-01T00:00:00Z",
				Nameservers: []string{"ns1.bbc.co.uk", "ns2.bbc.co.uk", "ns1.thc.bbc.co.uk", "ns2.thc.bbc.co.uk"},
			},
		},
		// Test 7: Parse DENIC - minimal response, timezone offset dates
		{
			fixture: "denic-example.de.json",
			wantReg: &domain.Registration{
				Created: "1998-03-12T20:44:25Z", // +01:00 timezone offset normalized to UTC
				Nameservers: []string{"ns1.example.de", "ns2.example.de"},
			},
		},
		// Test 8: Parse Registro.br - custom event types, timezone offset
		{
			fixture: "registrobr-example.br.json",
			wantReg: &domain.Registration{
				Registrar: "Registro BR",
				Created:   "1995-01-01T03:00:00Z", // -03:00 offset normalized to UTC
				Expires:   "2025-01-01T03:00:00Z",
				Nameservers: []string{"ns1.example.com.br", "ns2.example.com.br"},
			},
		},
	}

	for _, tt := range registeredTests {
		t.Run(tt.fixture, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "rdap", tt.fixture))
			if err != nil {
				t.Fatalf("failed to read fixture: %v", err)
			}

			reg := parseRDAPBody(data)
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
			} else {
				for i := range reg.Nameservers {
					if reg.Nameservers[i] != tt.wantReg.Nameservers[i] {
						t.Errorf("nameserver[%d]: got %q, want %q", i, reg.Nameservers[i], tt.wantReg.Nameservers[i])
					}
				}
			}
		})
	}
}

// TestRDAPAvailableFixtures tests parsing of 404/error response fixtures.
// Covers 4 available domain fixtures (Verisign empty, PIR JSON, Google JSON, CentralNic error object).
func TestRDAPAvailableFixtures(t *testing.T) {
	fixtures := []string{
		"verisign-404.txt",
		"pir-404.json",
		"google-404.json",
		"centralnic-404.json",
	}

	for _, fixture := range fixtures {
		t.Run(fixture, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "rdap", fixture))
			if err != nil {
				t.Fatalf("failed to read fixture: %v", err)
			}

			reg := parseRDAPBody(data)
			if reg == nil {
				t.Fatal("expected non-nil registration")
			}

			// All 404 fixtures should result in empty registration
			// (the availability is determined by HTTP status code, not body parsing)
			if reg.Registrar != "" {
				t.Errorf("expected empty registrar for 404 response, got %q", reg.Registrar)
			}
		})
	}
}

// TestRDAPErrorFixtures tests parsing of error response fixtures.
// Covers 3 error fixtures (429 rate limit, 400 bad request, empty body).
func TestRDAPErrorFixtures(t *testing.T) {
	tests := []struct {
		fixture    string
		wantEmpty  bool
	}{
		{
			fixture:   "google-429.json",
			wantEmpty: true, // error response, no registration data
		},
		{
			fixture:   "pir-400.json",
			wantEmpty: true,
		},
		{
			fixture:   "verisign-400-empty.txt",
			wantEmpty: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.fixture, func(t *testing.T) {
			data, err := os.ReadFile(filepath.Join("testdata", "rdap", tt.fixture))
			if err != nil {
				t.Fatalf("failed to read fixture: %v", err)
			}

			reg := parseRDAPBody(data)
			if reg == nil {
				t.Fatal("expected non-nil registration")
			}

			if tt.wantEmpty && reg.Registrar != "" {
				t.Errorf("expected empty registration for error response, got registrar %q", reg.Registrar)
			}
		})
	}
}

// TestParseRDAPDate tests date parsing with various formats.
// Covers: RFC3339, fractional seconds (0-6 digits), timezone offsets, invalid dates.
func TestParseRDAPDate(t *testing.T) {
	tests := []struct {
		input    string
		want     string
	}{
		// Test 11: Parse timezone offset date
		{"2018-03-12T21:44:25+01:00", "2018-03-12T20:44:25Z"},
		// Test 12: Parse zero-fraction date (CentralNic format)
		{"2014-03-20T12:59:17.0Z", "2014-03-20T12:59:17Z"},
		// Test 13: Parse microsecond date (Nominet format)
		{"2025-10-29T03:51:11.009091Z", "2025-10-29T03:51:11Z"},
		// Test 14: Parse +00:00 date (MarkMonitor format)
		{"2028-09-14T07:00:00.000+00:00", "2028-09-14T07:00:00Z"},
		// Additional date format tests
		{"", ""},
		{"1997-09-15T04:00:00Z", "1997-09-15T04:00:00Z"},
		{"2024-08-14T09:15:03Z", "2024-08-14T09:15:03Z"},
		{"1995-08-14T00:00:00.000Z", "1995-08-14T00:00:00Z"},
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
// Covers: trailing dots stripped, case normalization, empty entries skipped.
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
		// Test 5: Strip trailing dots from nameserver names (DENIC, Nominet)
		{
			name: "with trailing dots",
			input: []rdapNameserver{
				{LDHName: "NS1.EXAMPLE.COM."},
				{LDHName: "NS2.EXAMPLE.COM."},
			},
			want: []string{"ns1.example.com", "ns2.example.com"},
		},
		// Test 1: Normalize ldhName to lowercase (Verisign returns UPPERCASE)
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

// TestExtractRegistrar tests registrar extraction from entity hierarchy.
func TestExtractRegistrar(t *testing.T) {
	tests := []struct {
		name     string
		entities []rdapEntity
		want     string
	}{
		{
			name:     "empty entities",
			entities: nil,
			want:     "",
		},
		{
			name: "direct registrar",
			entities: []rdapEntity{
				{LDHName: "MarkMonitor Inc.", Roles: []string{"registrar"}},
			},
			want: "MarkMonitor Inc.",
		},
		{
			name: "nested registrar",
			entities: []rdapEntity{
				{
					Roles: []string{"registrant"},
					Entities: []rdapEntity{
						{LDHName: "CentralNic", Roles: []string{"registrar"}},
					},
				},
			},
			want: "CentralNic",
		},
		{
			name: "prefers LDHName over Handle",
			entities: []rdapEntity{
				{Handle: "123", LDHName: "Test Registrar", Roles: []string{"registrar"}},
			},
			want: "Test Registrar",
		},
		{
			name: "falls back to Handle",
			entities: []rdapEntity{
				{Handle: "REG-123", Roles: []string{"registrar"}},
			},
			want: "REG-123",
		},
		// Test 16: Missing handle field - parse successfully with empty
		{
			name: "no handle or ldhName",
			entities: []rdapEntity{
				{Roles: []string{"registrar"}},
			},
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRegistrar(tt.entities)
			if got != tt.want {
				t.Errorf("extractRegistrar() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestExtractDates tests date extraction from events.
func TestExtractDates(t *testing.T) {
	tests := []struct {
		name        string
		events      []rdapEvent
		wantCreated string
		wantExpires string
	}{
		{
			name:        "empty events",
			events:      nil,
			wantCreated: "",
			wantExpires: "",
		},
		{
			name: "registration and expiration",
			events: []rdapEvent{
				{Action: "registration", Date: "2020-01-01T00:00:00Z"},
				{Action: "expiration", Date: "2025-01-01T00:00:00Z"},
			},
			wantCreated: "2020-01-01T00:00:00Z",
			wantExpires: "2025-01-01T00:00:00Z",
		},
		// Test 3: Handle reregistration event without error (Google)
		{
			name: "reregistration fallback",
			events: []rdapEvent{
				{Action: "reregistration", Date: "2018-12-10T00:00:00Z"},
				{Action: "expiration", Date: "2025-12-10T00:00:00Z"},
			},
			wantCreated: "2018-12-10T00:00:00Z",
			wantExpires: "2025-12-10T00:00:00Z",
		},
		// Test 15: Unknown event types ignored
		{
			name: "unknown events ignored",
			events: []rdapEvent{
				{Action: "delegation check", Date: "2024-01-01T00:00:00Z"},
				{Action: "registration", Date: "2020-01-01T00:00:00Z"},
				{Action: "custom event", Date: "2024-01-01T00:00:00Z"},
			},
			wantCreated: "2020-01-01T00:00:00Z",
			wantExpires: "",
		},
		{
			name: "case insensitive event actions",
			events: []rdapEvent{
				{Action: "REGISTRATION", Date: "2020-01-01T00:00:00Z"},
				{Action: "Expiration", Date: "2025-01-01T00:00:00Z"},
			},
			wantCreated: "2020-01-01T00:00:00Z",
			wantExpires: "2025-01-01T00:00:00Z",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gotCreated, gotExpires := extractDates(tt.events)
			if gotCreated != tt.wantCreated {
				t.Errorf("created: got %q, want %q", gotCreated, tt.wantCreated)
			}
			if gotExpires != tt.wantExpires {
				t.Errorf("expires: got %q, want %q", gotExpires, tt.wantExpires)
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

// Test 17: CentralNic error object - don't treat as domain object
func TestCentralNicErrorObject(t *testing.T) {
	data := []byte(`{
		"rdapConformance": ["rdap_level_0"],
		"objectClassName": "error",
		"errorCode": 404,
		"title": "Not Found",
		"description": ["The requested domain was not found."]
	}`)

	reg := parseRDAPBody(data)
	if reg == nil {
		t.Fatal("expected non-nil registration")
	}

	// Should not extract any data from error object
	if reg.Registrar != "" {
		t.Errorf("expected empty registrar for error object, got %q", reg.Registrar)
	}
}
