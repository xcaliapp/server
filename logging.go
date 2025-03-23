package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"sync"
	"time"

	"github.com/rs/xid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/pkgerrors"
)

type LogLevel = string

const (
	DebugLevel LogLevel = "debug"
	InfoLevel  LogLevel = "info"
)

type LogFormat = string

const (
	JSONFormat    LogFormat = "json"
	ColoredFormat LogFormat = "colored"
)

func parseLevel() zerolog.Level {
	logLevel := os.Getenv("LOG_LEVEL")
	var level zerolog.Level
	switch logLevel {
	case InfoLevel:
		level = zerolog.InfoLevel
	case DebugLevel:
		level = zerolog.DebugLevel
	default:
		level = zerolog.InfoLevel
	}
	fmt.Printf("Log level: %v\n", level)
	return level
}

var once sync.Once

var log zerolog.Logger

func getLogger() zerolog.Logger {
	once.Do(func() {
		zerolog.ErrorStackMarshaler = pkgerrors.MarshalStack
		zerolog.TimeFieldFormat = time.RFC3339Nano

		var output io.Writer = zerolog.ConsoleWriter{
			Out:        os.Stdout,
			TimeFormat: time.RFC3339,
		}

		isDevelopmentEnv := func() bool {
			return os.Getenv("APP_ENV") == "development"
		}

		// if !isDevelopmentEnv() {
		// 	fileLogger := &lumberjack.Logger{
		// 		Filename:   "iconrepo.log",
		// 		MaxSize:    5,
		// 		MaxBackups: 10,
		// 		MaxAge:     14,
		// 		Compress:   true,
		// 	}

		// 	output = zerolog.MultiLevelWriter(os.Stderr, fileLogger)
		// }

		var gitRevision string

		buildInfo, ok := debug.ReadBuildInfo()
		if ok {
			for _, v := range buildInfo.Settings {
				if v.Key == "vcs.revision" {
					gitRevision = v.Value
					break
				}
			}
		}

		logLevel := parseLevel()

		fmt.Printf("Log level: %v\n", logLevel)

		logContext := zerolog.New(output).
			Level(zerolog.Level(logLevel)).
			With().
			Timestamp().
			Str("git_revision", gitRevision).
			Str("go_version", buildInfo.GoVersion).
			Str("app_xid", xid.New().String())
		if isDevelopmentEnv() {
			logContext = logContext.Caller()
		}

		log = logContext.Logger()
	})

	return log
}

const (
	HandlerLogger string = "handler"
	ServiceLogger string = "service"
	UnitLogger    string = "unit"
	MethodLogger  string = "method"
)

func CreateUnitLogger(logger zerolog.Logger, unitName string) zerolog.Logger {
	return logger.With().Str(UnitLogger, unitName).Logger()
}

func CreateMethodLogger(logger zerolog.Logger, unitName string) zerolog.Logger {
	return logger.With().Str(MethodLogger, unitName).Logger()
}
