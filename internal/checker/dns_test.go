package checker

import (
	"context"
	"net"
	"testing"
	"time"
)

func TestDNSPreFilter_Check(t *testing.T) {
	t.Run("registered domain with NS records", func(t *testing.T) {
		// Use a domain that definitely has NS records.
		// google.com is a safe bet for a registered domain.
		pf := NewDNSPreFilter()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result := pf.Check(ctx, "google.com")

		if result.Error != nil {
			t.Logf("DNS lookup error (may be network-related): %v", result.Error)
			// Don't fail the test on network errors
			t.Skip("network error")
		}

		if !result.HasNS {
			t.Error("expected HasNS=true for google.com, got false")
		}

		if len(result.Nameservers) == 0 {
			t.Error("expected nameservers for google.com, got none")
		}

		t.Logf("google.com nameservers: %v", result.Nameservers)
	})

	t.Run("non-existent domain returns no NS", func(t *testing.T) {
		pf := NewDNSPreFilter()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		// Use a domain that definitely doesn't exist.
		result := pf.Check(ctx, "this-domain-definitely-does-not-exist-12345.invalid")

		if result.Error != nil {
			// Network errors are acceptable for this test
			t.Logf("DNS lookup error (may be network-related): %v", result.Error)
			t.Skip("network error")
		}

		if result.HasNS {
			t.Error("expected HasNS=false for non-existent domain, got true")
		}
	})

	t.Run("context cancellation", func(t *testing.T) {
		pf := NewDNSPreFilter()

		ctx, cancel := context.WithCancel(context.Background())
		cancel() // Cancel immediately

		result := pf.Check(ctx, "google.com")

		if result.Error == nil {
			t.Error("expected error for cancelled context")
		}
	})
}

func TestDNSPreFilter_CheckDomain(t *testing.T) {
	t.Run("registered domain returns unavailable result", func(t *testing.T) {
		pf := NewDNSPreFilter()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result := pf.CheckDomain(ctx, "google.com")

		if result == nil {
			t.Log("DNS lookup returned nil (may be network-related)")
			t.Skip("network error")
		}

		if result.Available {
			t.Error("expected Available=false for google.com")
		}

		if result.Source != "dns" {
			t.Errorf("expected Source=dns, got %s", result.Source)
		}

		if result.Registration == nil || len(result.Registration.Nameservers) == 0 {
			t.Error("expected nameservers in registration")
		}

		t.Logf("result: domain=%s, available=%v, source=%s, duration=%dms",
			result.Domain, result.Available, result.Source, result.DurationMs)
	})

	t.Run("non-existent domain returns nil", func(t *testing.T) {
		pf := NewDNSPreFilter()

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		result := pf.CheckDomain(ctx, "this-domain-definitely-does-not-exist-12345.invalid")

		// For non-existent domains, we expect nil (inconclusive, needs RDAP)
		// Or an error result if there was a network issue
		if result != nil && result.Error != "" {
			t.Logf("DNS lookup error: %s", result.Error)
			t.Skip("network error")
		}

		if result != nil {
			t.Errorf("expected nil result for non-existent domain, got %+v", result)
		}
	})
}

func TestIsNXDomain(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{
			name:     "nil error",
			err:      nil,
			expected: false,
		},
		{
			name:     "NXDOMAIN DNSError",
			err:      &net.DNSError{Err: "no such host", IsNotFound: true},
			expected: true,
		},
		{
			name:     "non-NXDOMAIN DNSError",
			err:      &net.DNSError{Err: "timeout", IsTimeout: true},
			expected: false,
		},
		{
			name:     "error with 'no such host' string",
			err:      context.DeadlineExceeded, // Not NXDOMAIN
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isNXDomain(tt.err)
			if got != tt.expected {
				t.Errorf("isNXDomain() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestNewDNSPreFilterWithResolver(t *testing.T) {
	// Create a custom resolver pointing to Google DNS
	resolver := &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, network, "8.8.8.8:53")
		},
	}

	pf := NewDNSPreFilterWithResolver(resolver)

	if pf == nil {
		t.Fatal("NewDNSPreFilterWithResolver returned nil")
	}

	if pf.resolver != resolver {
		t.Error("resolver not set correctly")
	}
}

// BenchmarkDNSPreFilter_Check benchmarks DNS lookups.
func BenchmarkDNSPreFilter_Check(b *testing.B) {
	pf := NewDNSPreFilter()
	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		pf.Check(ctx, "google.com")
	}
}
