// Package web embeds the built React SPA. If the dist subdirectory doesn't
// exist at build time, FS() returns fs.ErrNotExist and the server falls back
// to a placeholder page (useful for backend-only development).
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var distFS embed.FS

func FS() (fs.FS, error) {
	sub, err := fs.Sub(distFS, "dist")
	if err != nil {
		return nil, err
	}
	// Sanity check that index.html exists; if not, treat as missing bundle.
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil, fs.ErrNotExist
	}
	return sub, nil
}
