// Package web embeds the built frontend (web/dist copied to internal/web/dist
// by `make web`). The .gitkeep keeps the directory present for `go build`
// before the frontend has been built.
package web

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS returns the frontend rooted at index.html.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		panic(err)
	}
	return sub
}
