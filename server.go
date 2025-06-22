package main

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/memstore"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

type drawingStore interface {
	PutDrawing(ctx context.Context, title string, contentReader io.Reader, modifiedBy string) error
	ListDrawingTitles(ctx context.Context) ([]string, error)
	GetDrawing(ctx context.Context, title string) (string, error)
}

type putDrawingRequest struct {
	Title   string `json:"title"`
	Content string `json:"content"`
}

func startServer(port int, passwordCreds []passwordCredentials, store drawingStore) {
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		portSpec := fmt.Sprintf("port %d", port)
		if port == 0 {
			portSpec = "an ephemeral port"
		}
		panic(fmt.Sprintf("Error while starting to listen at %s: %v", portSpec, err))
	}

	rootEngine := gin.Default()
	rootEngine.Use(RequestLogger)
	sessionStore := memstore.NewStore([]byte("secret"))
	rootEngine.Use(sessions.Sessions("mysession", sessionStore))
	rootEngine.NoRoute(gin.WrapH(AssetHandler("/", "webclient_dist", getLogger())))
	gob.Register(User{})
	rootEngine.Use(checkBasicAuthentication(basicConfig{passwordCreds: passwordCreds}))

	api := rootEngine.Group("/api")
	api.GET("/drawings", func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())

		titles, listErr := store.ListDrawingTitles(c)
		if listErr != nil {
			logger.Error().Err(listErr).Msg("failed to list drawing titles")
			c.AbortWithError(500, listErr)
			return
		}
		c.JSON(200, titles)
	})

	api.PUT("/drawing", func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())

		body, readBodyErr := io.ReadAll(c.Request.Body)
		if readBodyErr != nil {
			logger.Error().Err(readBodyErr).Msg("failed to read request body")
			c.AbortWithError(500, readBodyErr)
			return
		}
		var requestData putDrawingRequest
		requestBodyUnmarshalErr := json.Unmarshal(body, &requestData)
		if requestBodyUnmarshalErr != nil {
			logger.Error().Err(requestBodyUnmarshalErr).Msg("failed to unmarshal request body")
			c.AbortWithError(500, requestBodyUnmarshalErr)
			return
		}
		logger.Debug().Str("content", requestData.Content).Send()
		contentReader := strings.NewReader(requestData.Content)

		user, userExtractErr := getUserFromContext(c)
		if userExtractErr != nil {
			logger.Error().Err(userExtractErr).Str("title", requestData.Title).Msg("failed to extract user from context")
			c.AbortWithError(500, userExtractErr)
			return
		}

		putDrawingErr := store.PutDrawing(c, requestData.Title, contentReader, user.Username)
		if putDrawingErr != nil {
			logger.Error().Err(putDrawingErr).Str("title", requestData.Title).Msg("failed to store drawing %s: %w")
			c.AbortWithError(500, putDrawingErr)
			return
		}
		c.Status(200)
	})

	api.GET("/drawing", func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())

		title := c.Query("title")
		content, getContentErr := store.GetDrawing(c, title)
		if getContentErr != nil {
			logger.Error().Err(getContentErr).Str("title", title).Msg("failed to get drawing content")
			c.AbortWithError(500, getContentErr)
			return
		}
		logger.Debug().Str("title", title).Int("content length", len(content)).Msg("content found")
		c.JSON(200, content)
	})

	http.Serve(listener, rootEngine)
}

func getUserFromContext(c *gin.Context) (*User, error) {
	session := sessions.Default(c)
	untypedUser := session.Get(userKey)

	if untypedUser == nil {
		return nil, fmt.Errorf("no user in session")
	}

	user, userTypeOk := untypedUser.(User)
	if !userTypeOk {
		return nil, fmt.Errorf("failed to cast user in session from %T to %T", user, User{})
	}

	return &user, nil
}
