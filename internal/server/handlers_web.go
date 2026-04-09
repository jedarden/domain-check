package server

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"

	"github.com/coding/domain-check/internal/domain"
	"github.com/coding/domain-check/web"
)

// Default alternative TLDs to check for "Also check" section.
var defaultAltTLDs = []string{"com", "org", "net", "dev", "io", "app"}

// WebHandlers handles web UI requests.
type WebHandlers struct {
	templates *web.TemplateRenderer
	checker   DomainChecker
	log       *slog.Logger
}

// NewWebHandlers creates a new WebHandlers instance.
func NewWebHandlers(checker DomainChecker, log *slog.Logger) *WebHandlers {
	templates, err := web.LoadTemplates()
	if err != nil {
		panic("failed to load templates: " + err.Error())
	}
	return &WebHandlers{
		templates: templates,
		checker:   checker,
		log:       log,
	}
}

// IndexHandler renders the home page.
func (h *WebHandlers) IndexHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if err := h.templates.RenderIndex(w, &web.TemplateData{}); err != nil {
			h.log.Error("failed to render index", "error", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}
	}
}

// CheckHandler handles domain check requests from the web UI.
func (h *WebHandlers) CheckHandler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		query := r.URL.Query().Get("d")
		if query == "" {
			http.Redirect(w, r, "/", http.StatusFound)
			return
		}

		// Check if multiple TLDs are selected (from form checkboxes)
		tlds := r.URL.Query()["tlds"]
		if len(tlds) > 0 {
			h.handleMultiTLDCheck(w, r, query, tlds)
			return
		}

		// Parse domain
		domainName, err := domain.Parse(query)
		if err != nil {
			h.renderResult(w, &web.TemplateData{
				Domain:      query,
				Error:       "Invalid domain",
				ErrorDetail: err.Error(),
			})
			return
		}

		// Check availability
		result, err := h.checker.Check(r.Context(), domainName.Domain)
		if err != nil {
			h.renderResult(w, &web.TemplateData{
				Domain:      domainName.Domain,
				Error:       "Check failed",
				ErrorDetail: err.Error(),
			})
			return
		}

		// Prepare template data
		data := &web.TemplateData{
			Domain:     result.Domain,
			Available:  result.Available,
			TLD:        result.TLD,
			Source:     string(result.Source),
			Cached:     result.Cached,
			DurationMs: result.DurationMs,
		}

		if result.Error != "" {
			data.Error = result.Error
		} else if !result.Available && result.Registration != nil {
			data.Registration = &web.Registration{
				Registrar:   result.Registration.Registrar,
				Created:     result.Registration.Created,
				Expires:     result.Registration.Expires,
				Nameservers: result.Registration.Nameservers,
				Status:      result.Registration.Status,
			}
		}

		// Fetch alternative TLD results
		// Extract the name part (domain without TLD)
		name := strings.TrimSuffix(domainName.Domain, "."+domainName.TLD)
		data.AltTLDs = h.getAltTLDResults(r.Context(), name, domainName.TLD)

		h.renderResult(w, data)
	}
}

// handleMultiTLDCheck handles checking a domain name across multiple TLDs.
func (h *WebHandlers) handleMultiTLDCheck(w http.ResponseWriter, r *http.Request, query string, tlds []string) {
	// Normalize the domain name (remove TLD if present, or use as-is)
	name := normalizeName(query)

	// Validate the name
	if err := validateName(name); err != nil {
		h.renderResult(w, &web.TemplateData{
			Domain:      query,
			Error:       "Invalid domain name",
			ErrorDetail: err.Error(),
		})
		return
	}

	// Build list of full domain names to check
	var domains []string
	for _, tld := range tlds {
		tld = strings.TrimPrefix(tld, ".")
		tld = strings.ToLower(tld)
		if tld != "" {
			domains = append(domains, name+"."+tld)
		}
	}

	if len(domains) == 0 {
		h.renderResult(w, &web.TemplateData{
			Domain: query,
			Error:  "No valid TLDs selected",
		})
		return
	}

	// Check if we have a bulk checker available
	bulkChecker, ok := h.checker.(BulkChecker)
	if !ok {
		// Fallback: render error (shouldn't happen in production)
		h.renderResult(w, &web.TemplateData{
			Domain:      query,
			Error:       "Bulk checking not available",
			ErrorDetail: "The server does not support bulk domain checking",
		})
		return
	}

	// Perform bulk check
	bulkResult := bulkChecker.CheckBulk(r.Context(), domains)

	// Build multi-TLD results
	results := make([]web.MultiTLDResult, 0, len(domains))
	for _, fullDomain := range domains {
		result := web.MultiTLDResult{Domain: fullDomain}

		// Extract TLD from full domain
		parts := strings.SplitN(fullDomain, ".", 2)
		if len(parts) == 2 {
			result.TLD = parts[1]
		}

		if errStr, hasErr := bulkResult.Errors[fullDomain]; hasErr {
			result.Error = errStr
		} else if res := bulkResult.Results[fullDomain]; res != nil {
			result.Available = res.Available
			result.Source = string(res.Source)
			result.Cached = res.Cached
			result.DurationMs = res.DurationMs
			if res.Error != "" {
				result.Error = res.Error
			}
			if !res.Available && res.Registration != nil {
				result.Registration = &web.Registration{
					Registrar:   res.Registration.Registrar,
					Created:     res.Registration.Created,
					Expires:     res.Registration.Expires,
					Nameservers: res.Registration.Nameservers,
					Status:      res.Registration.Status,
				}
			}
		} else {
			result.Error = "no result returned"
		}

		results = append(results, result)
	}

	// Render multi-TLD results
	data := &web.TemplateData{
		Domain:   name,
		Results:  results,
		AltTLDs:  h.getAltTLDResultsForName(r.Context(), name, tlds),
	}
	h.renderResult(w, data)
}

// normalizeName normalizes a domain name for multi-TLD checking.
func normalizeName(input string) string {
	name := strings.TrimSpace(input)
	name = strings.ToLower(name)

	// Remove TLD if present (e.g., "example.com" -> "example")
	if strings.Contains(name, ".") {
		parts := strings.SplitN(name, ".", 2)
		name = parts[0]
	}

	// Remove trailing dot
	name = strings.TrimSuffix(name, ".")

	return name
}

// validateName validates a domain name label (without TLD).
func validateName(name string) error {
	if len(name) < 1 {
		return errors.New("domain name cannot be empty")
	}
	if len(name) > 63 {
		return errors.New("domain name exceeds 63 characters")
	}
	if name[0] == '-' {
		return errors.New("domain name cannot start with a hyphen")
	}
	if name[len(name)-1] == '-' {
		return errors.New("domain name cannot end with a hyphen")
	}
	for i, c := range name {
		if !isLDH(c) {
			return fmt.Errorf("invalid character '%c' at position %d", c, i)
		}
	}
	return nil
}

// isLDH reports whether c is a valid LDH character: a-z, 0-9, or hyphen.
func isLDH(c rune) bool {
	return (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-'
}

// getAltTLDResultsForName fetches alternative TLD results for a name, excluding the checked TLDs.
func (h *WebHandlers) getAltTLDResultsForName(ctx context.Context, name string, checkedTLDs []string) []web.AltTLDResult {
	bulkChecker, ok := h.checker.(BulkChecker)
	if !ok {
		return nil
	}

	// Create a map of checked TLDs for quick lookup
	checked := make(map[string]bool)
	for _, tld := range checkedTLDs {
		checked[strings.ToLower(strings.TrimPrefix(tld, "."))] = true
	}

	// Build list of alternative domains to check
	var domains []string
	var tlds []string
	for _, tld := range defaultAltTLDs {
		if checked[tld] {
			continue
		}
		domains = append(domains, name+"."+tld)
		tlds = append(tlds, tld)
	}

	if len(domains) == 0 {
		return nil
	}

	// Perform bulk check
	bulkResult := bulkChecker.CheckBulk(ctx, domains)
	if bulkResult == nil {
		return nil
	}

	// Build results
	var results []web.AltTLDResult
	for i, fullDomain := range domains {
		alt := web.AltTLDResult{
			TLD:    tlds[i],
			Domain: fullDomain,
		}

		if errStr, hasErr := bulkResult.Errors[fullDomain]; hasErr {
			alt.Error = errStr
		} else if res := bulkResult.Results[fullDomain]; res != nil {
			alt.Available = res.Available
		} else {
			alt.Error = "no result"
		}

		results = append(results, alt)
	}

	return results
}

// getAltTLDResults fetches availability for the same name across other TLDs.
func (h *WebHandlers) getAltTLDResults(ctx context.Context, name, currentTLD string) []web.AltTLDResult {
	// Try to use bulk checker for parallel requests
	bulkChecker, ok := h.checker.(BulkChecker)
	if !ok {
		return nil
	}

	// Build list of alternative domains to check (excluding current TLD)
	var domains []string
	var tlds []string
	for _, tld := range defaultAltTLDs {
		if strings.EqualFold(tld, currentTLD) {
			continue
		}
		domains = append(domains, name+"."+tld)
		tlds = append(tlds, tld)
	}

	if len(domains) == 0 {
		return nil
	}

	// Perform bulk check
	bulkResult := bulkChecker.CheckBulk(ctx, domains)
	if bulkResult == nil {
		return nil
	}

	// Build results
	var results []web.AltTLDResult
	for i, fullDomain := range domains {
		alt := web.AltTLDResult{
			TLD:    tlds[i],
			Domain: fullDomain,
		}

		if errStr, hasErr := bulkResult.Errors[fullDomain]; hasErr {
			alt.Error = errStr
		} else if res := bulkResult.Results[fullDomain]; res != nil {
			alt.Available = res.Available
		} else {
			alt.Error = "no result"
		}

		results = append(results, alt)
	}

	return results
}

// renderResult renders the result page.
func (h *WebHandlers) renderResult(w http.ResponseWriter, data *web.TemplateData) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := h.templates.RenderResult(w, data); err != nil {
		h.log.Error("failed to render result", "error", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// StaticHandler returns an HTTP handler for serving static assets.
func StaticHandler() http.Handler {
	staticFS, err := web.StaticFS()
	if err != nil {
		panic("failed to get static FS: " + err.Error())
	}
	return http.FileServer(http.FS(staticFS))
}

// escapeHTML escapes special HTML characters.
func escapeHTML(s string) string {
	return url.PathEscape(s)
}
