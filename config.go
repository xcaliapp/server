package main

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type drawingStoreType string

const (
	LOCAL_GIT drawingStoreType = "LOCAL_GIT"
	GITLAB    drawingStoreType = "GITLAB"
	S3        drawingStoreType = "S3"
)

type passwordCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type drawingSet struct {
	storeType drawingStoreType
	root      string
	path      string
}

type drawingSets map[string]drawingSet

type options struct {
	port            int
	passwordCreds   []passwordCredentials
	drawingStoreTyp drawingStoreType
}

const (
	drawingsetEnvvarNameAppPart     = "XCALIAPP"
	drawingsetEnvvarNameFeaturePart = "DRAWINGSET"
	drawingsetEnvvarNameTypePart    = "TYPE"
	drawingsetEnvvarNameRootPart    = "ROOT"
	drawingsetEnvvarNamePathPart    = "PATH"
)

func getDrawingSets() (drawingSets, error) {
	logger := CreateFunctionLogger(getLogger(), "getDrawingSets")

	const (
		appPart         = 0
		featurePart     = 1
		setAliasPart    = 2
		featurePropPart = 3
	)

	byAliasMap := drawingSets{}

	for _, env := range os.Environ() {
		envNameAndValue := strings.Split(env, "=")
		envName := envNameAndValue[0]
		envValue := envNameAndValue[1]
		if envName == drawingsetEnvvarNameAppPart+"_"+drawingsetEnvvarNameFeaturePart {
			logger.Info().Str("drawingSet.path", envValue).Msg("No drawing sets proper, only a single set at")
			byAliasMap[""] = drawingSet{root: envValue}
			return byAliasMap, nil
		}
		envvarNameParts := strings.Split(envName, "_")
		app := envvarNameParts[appPart]
		if app == drawingsetEnvvarNameAppPart {
			feature := envvarNameParts[featurePart]
			if feature == drawingsetEnvvarNameFeaturePart {
				alias := envvarNameParts[setAliasPart]
				property := envvarNameParts[featurePropPart]
				setWithAlias, ok := byAliasMap[alias]
				if !ok {
					setWithAlias = drawingSet{}
				}
				switch property {
				case drawingsetEnvvarNameTypePart:
					setWithAlias.storeType = drawingStoreType(envValue)
				case drawingsetEnvvarNameRootPart:
					setWithAlias.root = envValue
				case drawingsetEnvvarNamePathPart:
					setWithAlias.path = envValue
				}
				byAliasMap[alias] = setWithAlias
			}
		}
	}

	for _, value := range byAliasMap {
		if value.root == "" {
			logger.Warn().Interface("drawingSet", value).Msg("No root")
		}
		if value.path == "" {
			logger.Info().Interface("drawingSet", value).Msg("No path")
		}
	}

	return byAliasMap, nil
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
