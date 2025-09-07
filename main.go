package main

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/xid"
)

func main() {
	draRepoConfigs, err := getDrawingRepoConfigs()
	if err != nil {
		panic(err)
	}

	s, err := newServer(draRepoConfigs)
	if err != nil {
		panic(err)
	}

	logger := getLogger()
	logger.Info().Interface("drawingRepos", draRepoConfigs).Int("port", s.config.port).Msg("starting server...")

	s.start()
}

func RequestLogger(g *gin.Context) {
	start := time.Now()

	l := getLogger().With().Str("req_xid", xid.New().String()).Logger()

	r := g.Request
	g.Request = r.WithContext(l.WithContext(r.Context()))

	lrw := newLoggingResponseWriter(g.Writer)

	defer func() {
		panicVal := recover()
		if panicVal != nil {
			lrw.statusCode = http.StatusInternalServerError // ensure that the status code is updated
			panic(panicVal)                                 // continue panicking
		}
		l.
			Info().
			Str("method", g.Request.Method).
			Str("url", g.Request.URL.RequestURI()).
			Str("user_agent", g.Request.UserAgent()).
			Int("status_code", lrw.statusCode).
			Dur("elapsed_ms", time.Since(start)).
			Msg("incoming request")
	}()

	g.Next()
}

type loggingResponseWriter struct {
	http.ResponseWriter
	statusCode int
}

func newLoggingResponseWriter(w http.ResponseWriter) *loggingResponseWriter {
	return &loggingResponseWriter{w, http.StatusOK}
}

func (lrw *loggingResponseWriter) WriteHeader(code int) {
	lrw.statusCode = code
	lrw.ResponseWriter.WriteHeader(code)
}
