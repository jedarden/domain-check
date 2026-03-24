package domain

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParse_ValidDomains(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string // expected ParsedDomain.Domain
		wantTLD string // expected ParsedDomain.TLD
		wantUni string // expected ParsedDomain.Unicode (empty = no IDN)
	}{
		{
			name:    "simple com",
			input:   "example.com",
			want:    "example.com",
			wantTLD: "com",
		},
		{
			name:    "uppercase input",
			input:   "EXAMPLE.COM",
			want:    "example.com",
			wantTLD: "com",
		},
		{
			name:    "mixed case",
			input:   "ExAmPlE.CoM",
			want:    "example.com",
			wantTLD: "com",
		},
		{
			name:    "with www subdomain",
			input:   "www.example.com",
			want:    "example.com",
			wantTLD: "com",
		},
		{
			name:    "multi-level subdomain",
			input:   "a.b.c.example.com",
			want:    "example.com",
			wantTLD: "com",
		},
		{
			name:    "co.uk multi-level TLD",
			input:   "example.co.uk",
			want:    "example.co.uk",
			wantTLD: "co.uk",
		},
		{
			name:    "co.uk with subdomain",
			input:   "www.example.co.uk",
			want:    "example.co.uk",
			wantTLD: "co.uk",
		},
		{
			name:    "com.au multi-level TLD",
			input:   "example.com.au",
			want:    "example.com.au",
			wantTLD: "com.au",
		},
		{
			name:    "dev TLD",
			input:   "myapp.dev",
			want:    "myapp.dev",
			wantTLD: "dev",
		},
		{
			name:    "org TLD",
			input:   "opensource.org",
			want:    "opensource.org",
			wantTLD: "org",
		},
		{
			name:    "with trailing dot",
			input:   "example.com.",
			want:    "example.com",
			wantTLD: "com",
		},
		{
			name:    "with leading/trailing whitespace",
			input:   "  example.com  ",
			want:    "example.com",
			wantTLD: "com",
		},
		{
			name:    "with tab and newline whitespace",
			input:   "\texample.com\n",
			want:    "example.com",
			wantTLD: "com",
		},
		{
			name:    "hyphen in label",
			input:   "my-domain.com",
			want:    "my-domain.com",
			wantTLD: "com",
		},
		{
			name:    "numeric labels",
			input:   "123.com",
			want:    "123.com",
			wantTLD: "com",
		},
		{
			name:    "punycode input",
			input:   "xn--mnchen-3ya.de",
			want:    "xn--mnchen-3ya.de",
			wantTLD: "de",
			wantUni: "münchen.de",
		},
		{
			name:    "unicode input (German)",
			input:   "münchen.de",
			want:    "xn--mnchen-3ya.de",
			wantTLD: "de",
			wantUni: "münchen.de",
		},
		{
			name:    "net TLD",
			input:   "example.net",
			want:    "example.net",
			wantTLD: "net",
		},
		{
			name:    "io TLD",
			input:   "example.io",
			want:    "example.io",
			wantTLD: "io",
		},
		{
			name:    "63-char label (max)",
			input:   strings.Repeat("a", 63) + ".com",
			want:    strings.Repeat("a", 63) + ".com",
			wantTLD: "com",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result.Domain)
			assert.Equal(t, tt.wantTLD, result.TLD)
			assert.Equal(t, tt.wantUni, result.Unicode)
			assert.Equal(t, tt.input, result.Original)
		})
	}
}

func TestParse_InvalidDomains(t *testing.T) {
	tests := []struct {
		name       string
		input      string
		wantPhase  string // expected failed phase
		wantErrStr string // substring expected in error message
	}{
		{
			name:       "empty string",
			input:      "",
			wantPhase:  "trim",
			wantErrStr: "empty after trimming",
		},
		{
			name:       "whitespace only",
			input:      "   ",
			wantPhase:  "trim",
			wantErrStr: "empty after trimming",
		},
		{
			name:       "contains slash (URL)",
			input:      "example.com/path",
			wantPhase:  "url-chars",
			wantErrStr: "URL characters",
		},
		{
			name:       "contains colon (URL scheme)",
			input:      "http://example.com",
			wantPhase:  "url-chars",
			wantErrStr: "URL characters",
		},
		{
			name:       "contains at-sign (email)",
			input:      "user@example.com",
			wantPhase:  "url-chars",
			wantErrStr: "URL characters",
		},
		{
			name:       "label starts with hyphen",
			input:      "-example.com",
			wantPhase:  "idna",
			wantErrStr: "invalid label",
		},
		{
			name:       "label ends with hyphen",
			input:      "example-.com",
			wantPhase:  "idna",
			wantErrStr: "invalid label",
		},
		{
			name:       "TLD starts with hyphen",
			input:      "example.-com",
			wantPhase:  "idna",
			wantErrStr: "invalid label",
		},
		{
			name:       "single label (no TLD)",
			input:      "example",
			wantPhase:  "tld-extract",
			wantErrStr: "no registrable domain found",
		},
		{
			name:       "label too long (64 chars)",
			input:      strings.Repeat("a", 64) + ".com",
			wantPhase:  "ldh",
			wantErrStr: "exceeds 63 characters",
		},
		{
			name:       "domain too long (254 chars)",
			input:      strings.Repeat("a", 250) + ".com",
			wantPhase:  "ldh",
			wantErrStr: "exceeds 253 characters",
		},
		{
			name:       "dot only",
			input:      ".",
			wantPhase:  "trim",
			wantErrStr: "empty after trimming",
		},
		{
			name:       "consecutive dots",
			input:      "example..com",
			wantPhase:  "ldh",
			wantErrStr: "label 2 is empty",
		},
		{
			name:       "underscore in label",
			input:      "my_domain.com",
			wantPhase:  "idna",
			wantErrStr: "",
		},
		{
			name:       "space in label",
			input:      "my domain.com",
			wantPhase:  "idna",
			wantErrStr: "",
		},
		{
			name:       "just a TLD",
			input:      "com",
			wantPhase:  "tld-extract",
			wantErrStr: "no registrable domain found",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.input)
			require.Error(t, err)
			assert.Nil(t, result)

			require.IsType(t, &ParseError{}, err)
			pe := err.(*ParseError)
			assert.Equal(t, tt.input, pe.Input)
			assert.Equal(t, tt.wantPhase, pe.Phase)
			if tt.wantErrStr != "" {
				assert.Contains(t, pe.Err, tt.wantErrStr)
			}
		})
	}
}

func TestParse_IDNAEdgeCases(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantErr   bool
		wantDomain string
	}{
		{
			name:      "Chinese characters",
			input:     "测试.com",
			wantErr:   false,
			wantDomain: "xn--0zwm56d.com",
		},
		{
			name:      "Japanese hiragana",
			input:     "にほんご.jp",
			wantErr:   false,
			wantDomain: "xn--38j2b6b6e.jp",
		},
		{
			name:      "already punycode",
			input:     "xn--example-6q4a.com",
			wantErr:   false,
			wantDomain: "xn--example-6q4a.com",
		},
		{
			name:      "punycode with subdomain",
			input:     "www.xn--mnchen-3ya.de",
			wantErr:   false,
			wantDomain: "xn--mnchen-3ya.de",
		},
		{
			name:    "invalid punycode",
			input:   "xn--invalid-.com",
			wantErr: true,
		},
		{
			name:    "mixed script (rejected by IDNA 2008)",
			input:   "xn--mnchen-3ya.xn--0zwm56d",
			wantErr: false, // Different TLD labels are fine
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.input)
			if tt.wantErr {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tt.wantDomain, result.Domain)
			}
		})
	}
}

func TestParse_ParseError_Error(t *testing.T) {
	err := &ParseError{
		Input: "bad input",
		Phase: "ldh",
		Err:   "starts with hyphen",
	}
	assert.Equal(t, `domain "bad input": ldh (starts with hyphen)`, err.Error())
}

func TestParse_EffectiveTLDPlusOne(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{name: "com", input: "sub.www.example.com", want: "example.com"},
		{name: "co.uk", input: "a.b.example.co.uk", want: "example.co.uk"},
		{name: "org", input: "deep.nested.example.org", want: "example.org"},
		{name: "com.au", input: "www.example.com.au", want: "example.com.au"},
		{name: "gov.uk", input: "www.direct.gov.uk", want: "direct.gov.uk"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := Parse(tt.input)
			require.NoError(t, err)
			assert.Equal(t, tt.want, result.Domain)
		})
	}
}
