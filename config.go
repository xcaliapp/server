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

type drawingRepoConfig struct {
	name      string
	label     string
	storeType drawingStoreType
	root      string
	path      string
}

type drawingReposConfigs map[string]drawingRepoConfig

type options struct {
	port            int
	passwordCreds   []passwordCredentials
	drawingStoreTyp drawingStoreType
}

const (
	drawingRepoEnvvarNameAppPart         = "XCALIAPP"
	drawingRepoEnvvarNameFeaturePart     = "DRAWINGREPO"
	drawingRepoEnvvarNameListPart        = "LIST" // maps names to labels; sample value: "wsgw:WebSocket Gateway,xcaliapp:Xcalidraw App"
	drawingRepoEnvvarNameStorageTypePart = "STORETYPE"
	drawingRepoEnvvarNameRootPart        = "ROOT"
	drawingRepoEnvvarNamePathPart        = "PATH"
)

func getDrawingRepoConfigs() (drawingReposConfigs, error) {
	logger := CreateFunctionLogger(getLogger(), "getDrawingRepos")

	nameToConfigMap := drawingReposConfigs{}

	envVarFeatureBaseName := drawingRepoEnvvarNameAppPart + "_" + drawingRepoEnvvarNameFeaturePart

	listEnvVarName := envVarFeatureBaseName + "_" + drawingRepoEnvvarNameListPart
	for _, env := range os.Environ() {
		envNameAndValue := strings.Split(env, "=")
		envName := envNameAndValue[0]
		envValue := envNameAndValue[1]
		if envName == listEnvVarName {
			namesAndLabels := strings.SplitSeq(envValue, ",") // e.g. "WebSocket Gateway:wsgw"
			for nameAndLabel := range namesAndLabels {
				nl := strings.Split(nameAndLabel, ":")
				nameToConfigMap[nl[0]] = drawingRepoConfig{
					name:  nl[0],
					label: nl[1],
				}
			}
		}
	}

	setupConfigForName := func(config drawingRepoConfig) drawingRepoConfig {
		for _, env := range os.Environ() {
			envVarNamePrefix := envVarFeatureBaseName + "_" + config.name + "_"
			envNameAndValue := strings.Split(env, "=")
			envName := envNameAndValue[0]
			envValue := envNameAndValue[1]
			if len(envName) > len(envVarNamePrefix) && envName[0:len(envVarNamePrefix)] == envVarNamePrefix {
				property := envName[len(envVarNamePrefix):]
				switch property {
				case drawingRepoEnvvarNameStorageTypePart:
					config.storeType = drawingStoreType(envValue)
				case drawingRepoEnvvarNameRootPart:
					config.root = envValue
				case drawingRepoEnvvarNamePathPart:
					config.path = envValue
				}

				if envName == envVarNamePrefix+"_"+drawingRepoEnvvarNameStorageTypePart {
					config.storeType = drawingStoreType(envValue)
				}
				if envName == envVarNamePrefix+"_"+drawingRepoEnvvarNameRootPart {
					config.root = envValue
				}
				if envName == envVarNamePrefix+"_"+drawingRepoEnvvarNamePathPart {
					config.path = envValue
				}
			}
		}
		return config
	}

	for _, config := range nameToConfigMap {
		nameToConfigMap[config.name] = setupConfigForName(config)
	}

	for _, value := range nameToConfigMap {
		if value.root == "" {
			logger.Warn().Interface("drawingRepoConfig", value).Msg("No root")
		}
		if value.path == "" {
			logger.Info().Interface("drawingRepoConfig", value).Msg("No path")
		}
	}

	return nameToConfigMap, nil
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
