package main

import (
	"context"
	"crypto/rand"
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
	CopyDrawing(ctx context.Context, sourceId string, destinationId string, modifiedBy string) error
	ListDrawings(ctx context.Context) (map[string]string, error)
	GetDrawing(ctx context.Context, title string) (string, error)
	DeleteDrawing(ctx context.Context, newTitle string, modifiedBy string) error
}

type putDrawingRequest struct {
	Content string `json:"content"`
}

func startServer(port int, passwordCreds []passwordCredentials, store drawingStore) {
	h := handlerFactory{store}

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

	rootEngine.GET("/drawings", gin.WrapH(AssetHandler("/", "webclient_dist", getLogger())))

	api := rootEngine.Group("/api")
	api.GET("/drawings", h.getDrawingListHandler())
	api.POST("/drawing", h.createNewDrawing())
	api.PUT("/drawing/:id", h.updateDrawing())
	api.GET("/drawing/:id", h.getDrawingContent())
	api.DELETE("/drawing/:id", h.deleteDrawing())

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

type handlerFactory struct {
	store drawingStore
}

func (hf *handlerFactory) getDrawingListHandler() func(c *gin.Context) {
	return func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())

		titles, listErr := hf.store.ListDrawings(c)
		if listErr != nil {
			logger.Error().Err(listErr).Msg("failed to list drawing titles")
			c.AbortWithError(http.StatusInternalServerError, listErr)
			return
		}
		c.JSON(http.StatusOK, titles)
	}
}

func (hf *handlerFactory) createNewDrawing() func(c *gin.Context) {
	return func(c *gin.Context) {
		id := rand.Text()
		hf.putDrawing(c, id)
		c.JSON(200, id)
	}
}

func (hf *handlerFactory) updateDrawing() func(c *gin.Context) {
	return func(c *gin.Context) {
		id := c.Param("id")
		hf.putDrawing(c, id)
		c.JSON(200, id)
	}
}

func (hf *handlerFactory) putDrawing(c *gin.Context, drawingId string) {
	logger := zerolog.Ctx(c.Request.Context())

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
		logger.Error().Err(userExtractErr).Str("id", drawingId).Msg("failed to extract user from context")
		c.AbortWithError(http.StatusInternalServerError, userExtractErr)
		return
	}

	putDrawingErr := hf.store.PutDrawing(c, drawingId, contentReader, user.Username)
	if putDrawingErr != nil {
		logger.Error().Err(putDrawingErr).Str("id", drawingId).Msg("failed to store drawing %s: %w")
		c.AbortWithError(http.StatusInternalServerError, putDrawingErr)
		return
	}
}

func (hf *handlerFactory) getDrawingContent() func(c *gin.Context) {
	return func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())

		id := c.Param("id")
		content, getContentErr := hf.store.GetDrawing(c, id)
		if getContentErr != nil {
			logger.Error().Err(getContentErr).Str("id", id).Msg("failed to get drawing content")
			c.AbortWithError(http.StatusInternalServerError, getContentErr)
			return
		}
		logger.Debug().Str("id", id).Int("content length", len(content)).Msg("content found")
		c.JSON(http.StatusOK, content)
	}
}

func (hf *handlerFactory) deleteDrawing() func(c *gin.Context) {
	return func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())
		id := c.Param("id")
		if len(id) == 0 {
			logger.Debug().Msg("Missing 'id' path parameter")
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		user, userExtractErr := getUserFromContext(c)
		if userExtractErr != nil {
			logger.Error().Err(userExtractErr).Str("id", id).Msg("failed to extract user from context")
			c.AbortWithError(http.StatusInternalServerError, userExtractErr)
			return
		}
		err := hf.store.DeleteDrawing(c, id, user.Username)
		if err != nil {
			logger.Error().Err(err).Msg("failed to delete the object with the old name")
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		c.Status(http.StatusOK)
	}
}
