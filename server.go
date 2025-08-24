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
	CopyDrawing(ctx context.Context, sourceTitle string, destinationTitle string, modifiedBy string) error
	ListDrawingTitles(ctx context.Context) ([]string, error)
	GetDrawing(ctx context.Context, title string) (string, error)
	DeleteDrawing(ctx context.Context, newTitle string, modifiedBy string) error
}

type putDrawingRequest struct {
	Content string `json:"content"`
}

type patchDrawingRequest struct {
	NewTitle string `json:"newTitle"`
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
			c.AbortWithError(http.StatusInternalServerError, listErr)
			return
		}

		c.JSON(http.StatusOK, titles)
	})

	api.PUT("/drawing/:id", func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())

		id := c.Param("id")

		body, readBodyErr := io.ReadAll(c.Request.Body)
		if readBodyErr != nil {
			logger.Error().Err(readBodyErr).Msg("failed to read request body")
			c.AbortWithError(http.StatusInternalServerError, readBodyErr)
			return
		}
		var requestData putDrawingRequest
		requestBodyUnmarshalErr := json.Unmarshal(body, &requestData)
		if requestBodyUnmarshalErr != nil {
			logger.Error().Err(requestBodyUnmarshalErr).Msg("failed to unmarshal request body")
			c.AbortWithError(http.StatusInternalServerError, requestBodyUnmarshalErr)
			return
		}
		logger.Debug().Str("content", requestData.Content).Send()
		contentReader := strings.NewReader(requestData.Content)

		user, userExtractErr := getUserFromContext(c)
		if userExtractErr != nil {
			logger.Error().Err(userExtractErr).Str("title", id).Msg("failed to extract user from context")
			c.AbortWithError(http.StatusInternalServerError, userExtractErr)
			return
		}

		putDrawingErr := store.PutDrawing(c, id, contentReader, user.Username)
		if putDrawingErr != nil {
			logger.Error().Err(putDrawingErr).Str("title", id).Msg("failed to store drawing %s: %w")
			c.AbortWithError(http.StatusInternalServerError, putDrawingErr)
			return
		}
		c.Status(http.StatusOK)
	})

	api.GET("/drawing/:id", func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())

		id := c.Param("id")
		content, getContentErr := store.GetDrawing(c, id)
		if getContentErr != nil {
			logger.Error().Err(getContentErr).Str("title", id).Msg("failed to get drawing content")
			c.AbortWithError(http.StatusInternalServerError, getContentErr)
			return
		}
		logger.Debug().Str("title", id).Int("content length", len(content)).Msg("content found")
		c.JSON(http.StatusOK, content)
	})

	api.DELETE("/drawing/:title", func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())
		title := c.Param("title")
		if len(title) == 0 {
			logger.Debug().Msg("Missing 'title' path parameter")
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		user, userExtractErr := getUserFromContext(c)
		if userExtractErr != nil {
			logger.Error().Err(userExtractErr).Str("title", title).Msg("failed to extract user from context")
			c.AbortWithError(http.StatusInternalServerError, userExtractErr)
			return
		}
		err = store.DeleteDrawing(c, title, user.Username)
		if err != nil {
			logger.Error().Err(err).Msg("failed to delete the object with the old name")
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		c.Status(http.StatusOK)
	})

	api.PATCH("/drawing/:id", func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())

		id := c.Param("id")

		body, readBodyErr := io.ReadAll(c.Request.Body)
		if readBodyErr != nil {
			logger.Error().Err(readBodyErr).Msg("failed to read request body")
			c.AbortWithError(http.StatusInternalServerError, readBodyErr)
			return
		}
		var requestData patchDrawingRequest
		requestBodyUnmarshalErr := json.Unmarshal(body, &requestData)
		if requestBodyUnmarshalErr != nil {
			logger.Error().Err(requestBodyUnmarshalErr).Msg("failed to unmarshal request body")
			c.AbortWithError(http.StatusInternalServerError, requestBodyUnmarshalErr)
			return
		}
		logger.Debug().Interface("requestData", requestData).Send()
		switch {
		case len(requestData.NewTitle) > 0:
			logger := logger.With().Str("title", id).Str("newTitle", requestData.NewTitle).Logger()

			user, userExtractErr := getUserFromContext(c)
			if userExtractErr != nil {
				logger.Error().Err(userExtractErr).Str("newTitle", requestData.NewTitle).Msg("failed to extract user from context")
				c.AbortWithError(http.StatusInternalServerError, userExtractErr)
				return
			}
			err = store.CopyDrawing(c, id, requestData.NewTitle, user.Username)
			if err != nil {
				logger.Error().Err(err).Msg("failed to create the copy to subsist as the renamed object")
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			err = store.DeleteDrawing(c, id, user.Username)
			if err != nil {
				logger.Error().Err(err).Msg("failed to delete the object with the old name")
				c.AbortWithError(http.StatusInternalServerError, err)
				return
			}
			c.Status(http.StatusOK)
		}
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
