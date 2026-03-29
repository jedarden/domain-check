// Package web provides embedded templates and static assets for the web UI.
package web

import (
	"embed"
	"io/fs"
)

//go:embed templates/* static/*
var content embed.FS

// Templates returns the embedded templates filesystem.
func Templates() (fs.FS, error) {
	return fs.Sub(content, "templates")
}

// Static returns the embedded static assets filesystem.
func Static() (fs.FS, error) {
	return fs.Sub(content, "static")
}
