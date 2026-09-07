package web

import (
	"fmt"
	"html/template"
	"io"
	"io/fs"
)

// TemplateData holds data passed to templates.
type TemplateData struct {
	Title       string
	Domain      string
	Available   bool
	TLD         string
	Source      string
	Cached      bool
	DurationMs  int64
	Error       string
	ErrorDetail string
	Registration *Registration
	AltTLDs     []AltTLDResult
	Results     []MultiTLDResult // For multi-TLD results from form submission
}

// MultiTLDResult holds a result for a single TLD in a multi-TLD check.
type MultiTLDResult struct {
	Domain       string
	TLD          string
	Available    bool
	Source       string
	Cached       bool
	DurationMs   int64
	Error        string
	Registration *Registration
}

// AltTLDResult holds a result for an alternative TLD check.
type AltTLDResult struct {
	TLD       string
	Domain    string
	Available bool
	Error     string
}

// Registration holds domain registration details.
type Registration struct {
	Registrar   string
	Created     string
	Expires     string
	Nameservers []string
	Status      []string
}

// TemplateRenderer holds parsed HTML templates.
type TemplateRenderer struct {
	index  *template.Template
	result *template.Template
}

// LoadTemplates parses all templates from the embedded filesystem.
func LoadTemplates() (*TemplateRenderer, error) {
	tmplFS, err := Templates()
	if err != nil {
		return nil, fmt.Errorf("failed to get templates FS: %w", err)
	}

	// Parse index page templates (layout + index content)
	indexTmpl, err := template.ParseFS(tmplFS, "layout.html", "index.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse index templates: %w", err)
	}

	// Parse result page templates (layout + result content)
	resultTmpl, err := template.ParseFS(tmplFS, "layout.html", "result.html")
	if err != nil {
		return nil, fmt.Errorf("failed to parse result templates: %w", err)
	}

	return &TemplateRenderer{
		index:  indexTmpl,
		result: resultTmpl,
	}, nil
}

// RenderIndex renders the index (home) page.
func (t *TemplateRenderer) RenderIndex(w io.Writer, data *TemplateData) error {
	return t.index.ExecuteTemplate(w, "index", data)
}

// RenderResult renders the result page.
func (t *TemplateRenderer) RenderResult(w io.Writer, data *TemplateData) error {
	return t.result.ExecuteTemplate(w, "result", data)
}

// StaticFS returns the filesystem for static assets.
func StaticFS() (fs.FS, error) {
	return Static()
}
