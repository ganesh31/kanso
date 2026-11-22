// Package webui embeds the built web app in the server binary. `make
// build` copies web/dist into dist/ before compiling; without that the
// binary carries only the placeholder and serves the API alone.
package webui

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

// FS returns the embedded web app with index.html at its root, or nil if
// the binary was built without one.
func FS() fs.FS {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil
	}
	if _, err := fs.Stat(sub, "index.html"); err != nil {
		return nil
	}
	return sub
}
