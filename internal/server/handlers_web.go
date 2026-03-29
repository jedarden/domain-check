package server

import (
	"context"
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
