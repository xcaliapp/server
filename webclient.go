package main

import (
	"embed"
	"io/fs"
	"net/http"
	"os"
	"path"

	"github.com/rs/zerolog"
)

//go:embed webclient_dist/*
var Assets embed.FS

// fsFunc is short-hand for constructing a http.FileSystem
// implementation
type fsFunc func(name string) (fs.File, error)

func (f fsFunc) Open(name string) (fs.File, error) {
	return f(name)
}

func AssetHandler(prefix, root string, log zerolog.Logger) http.Handler {
	handler := fsFunc(func(name string) (fs.File, error) {
		logger := CreateMethodLogger(log, "AssetHandler")

		logger.Debug().Str("asset", name).Msg("asset requested")

		assetPath := path.Join(root, name)

		// If we can't find the asset, return the default index.html
		// content
		f, err := Assets.Open(assetPath)
		if os.IsNotExist(err) {
			return Assets.Open("index.html")
		}

		// Otherwise assume this is a legitimate request routed
		// correctly
		return f, err
	})

	return http.StripPrefix(prefix, http.FileServer(http.FS(handler)))
}
