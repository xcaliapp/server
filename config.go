package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
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

const DefaultServerPort = 8080
const defaultUsername = "peter.dunay.kovacs@gmail.com"

func getServerPort() int {
	envvar := os.Getenv("SERVER_PORT")
	if len(envvar) > 0 {
		port, err := strconv.Atoi(envvar)
		if err != nil {
			panic(fmt.Sprintf("failed to parse SERVER_PORT %s: %#v", envvar, err))
		}
		return port
	}
	return DefaultServerPort
}

func getUsername() string {
	envvar := os.Getenv("XCALIAPP_USERNAME")
	if len(envvar) > 0 {
		return envvar
	}
	return defaultUsername
}
