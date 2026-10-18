package pdswarm

import (
	"embed"
	"io/fs"
)

// DistFS embeds all frontend build assets from web/dist.
//
//go:embed web/dist/*
var DistFS embed.FS

// WebFS returns a sub-filesystem rooted at web/dist.
func WebFS() fs.FS {
	sub, err := fs.Sub(DistFS, "web/dist")
	if err != nil {
		return DistFS
	}
	return sub
}
