package server

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/coding/domain-check/internal/domain"
	"github.com/coding/domain-check/web"
)

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

		h.renderResult(w, data)
	}
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
