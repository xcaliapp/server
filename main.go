package main

import (
	"context"
	"fmt"
	"gitstore"
	"net"
	"net/http"
	"s3store"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/xid"
)

type server struct {
	ctx      context.Context
	listener net.Listener
	config   options
}

var s = server{
	ctx:      context.Background(),
	listener: nil,
	config: options{
		8080,
		[]passwordCredentials{{
			Username: "peter.dunay.kovacs@gmail.com",
			Password: "pass",
		}},
		LOCAL_GIT,
		getDrawingStoreLocation(),
	},
}

func main() {
	var err error
	s.listener, err = net.Listen("tcp", fmt.Sprintf(":%d", s.config.port))
	if err != nil {
		portSpec := fmt.Sprintf("port %d", s.config.port)
		if s.config.port == 0 {
			portSpec = "an ephemeral port"
		}
		panic(fmt.Sprintf("Error while starting to listen at %s: %v", portSpec, err))
	}

	logger := getLogger()
	logger.Info().Int("port", s.config.port).Msg("starting server...")
	startServer(newDrawingStore(s))
}

func newDrawingStore(s server) drawingStore {
	switch s.config.drawingStoreTyp {
	case LOCAL_GIT:
		logger := getLogger().With().Str("repoType", "LOCAL_GIT").Str("repoPath", s.config.drawingStoreKey).Logger()
		repo, repoErr := gitstore.NewLocalGitStore(s.config.drawingStoreKey, &logger)
		if repoErr != nil {
			panic(repoErr)
		}
		return repo
	case GITLAB:
		panic("Not yet supported")
	case S3:
		repo, repoErr := s3store.NewDrawingStore(s.ctx, "test-xcali-backend")
		if repoErr != nil {
			panic(fmt.Sprintf("failed to created S3 store: %v", repoErr))
		}
		return repo
	}
	panic(fmt.Errorf("invalid drawingStoreTyp: %v", s.config.drawingStoreTyp))
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
