package domain

import "time"

// Source indicates how the result was obtained.
type Source string

const (
	SourceRDAP  Source = "rdap"
	SourceDNS   Source = "dns"
	SourceWHOIS Source = "whois"
	SourceCache Source = "cache"
)

// Registration holds registration details for an unavailable domain.
type Registration struct {
	Registrar   string   `json:"registrar,omitempty"`
	Created     string   `json:"created,omitempty"`
	Expires     string   `json:"expires,omitempty"`
	Nameservers []string `json:"nameservers,omitempty"`
	Status      []string `json:"status,omitempty"`
}

// DomainResult is the result of a domain availability check.
type DomainResult struct {
	Domain      string       `json:"domain"`
	Available   bool         `json:"available"`
	TLD         string       `json:"tld"`
	CheckedAt   time.Time    `json:"checked_at"`
	Source      Source       `json:"source"`
	Cached      bool         `json:"cached"`
	DurationMs  int64        `json:"duration_ms"`
	Registration *Registration `json:"registration,omitempty"`
	Error       string       `json:"error,omitempty"`
}
