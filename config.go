package main

import (
	"os"
	"path/filepath"
)

type drawingStoreType int

const (
	LOCAL_GIT drawingStoreType = iota
	GITLAB
	S3
)

type passwordCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type options struct {
	port            int
	passwordCreds   []passwordCredentials
	drawingStoreTyp drawingStoreType
	drawingStoreKey string
}

func getDrawingStoreLocation() string {
	envvar := os.Getenv("DRAWINGSTORE_LOCATION")
	if len(envvar) > 0 {
		return envvar
	}
	return filepath.Join(os.Getenv("HOME"), ".xcaliapp", "drawing-store")
}
