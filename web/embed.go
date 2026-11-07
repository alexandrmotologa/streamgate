package web

import (
	"embed"
	"io/fs"
)

//go:embed dist/*
var distFS embed.FS

// Assets returns an fs.FS referencing the built single-page application assets.
func Assets() (fs.FS, error) {
	return fs.Sub(distFS, "dist")
}
