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
		logger := CreateFunctionLogger(log, "AssetHandler").With().Str("asset", name).Logger()

		logger.Debug().Msg("asset requested")

		assetPath := path.Join(root, name)

		// If we can't find the asset, return the default index.html
		// content
		f, err := Assets.Open(assetPath)
		if err != nil {
			logger.Debug().Err(err).Msg("asset retrieval result")
		}
		if os.IsNotExist(err) {
			logger.Debug().Msg("return the default index.html")
			f, err = Assets.Open(path.Join(root, "index.html"))
			if err != nil {
				logger.Error().Err(err).Msg("failed to open index.html")
			}
			return f, err
		}

		// Otherwise assume this is a legitimate request routed
		// correctly
		return f, err
	})

	return http.StripPrefix(prefix, http.FileServer(http.FS(handler)))
}
