package main

import (
	"encoding/json"
	"io"
	"net/http"
	"strings"
	xcalistores3 "xcalistore-s3"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/memstore"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

func startServer(store *xcalistores3.DrawingStore) {
	rootEngine := gin.Default()
	rootEngine.Use(RequestLogger)
	sessionStore := memstore.NewStore([]byte("secret"))
	rootEngine.Use(sessions.Sessions("mysession", sessionStore))
	rootEngine.Use(checkBasicAuthentication(basicConfig{passwordCreds: s.config.passwordCreds}))

	rootEngine.GET("/drawing", func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())

		titles, listErr := store.ListDrawingTitles(c)
		if listErr != nil {
			logger.Error().Err(listErr).Msg("failed to list drawing titles")
			c.AbortWithError(500, listErr)
			return
		}
		c.JSON(200, titles)
	})

	rootEngine.PUT("/drawing/:title", func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())

		title := c.Param("title")
		body, readBodyErr := io.ReadAll(c.Request.Body)
		if readBodyErr != nil {
			logger.Error().Err(readBodyErr).Msg("failed to read request body")
			c.AbortWithError(500, readBodyErr)
			return
		}
		var content string
		requestBodyUnmarshalErr := json.Unmarshal(body, &content)
		if requestBodyUnmarshalErr != nil {
			logger.Error().Err(requestBodyUnmarshalErr).Msg("failed to unmarshal request body")
			c.AbortWithError(500, requestBodyUnmarshalErr)
			return
		}
		logger.Debug().Str("content", content).Send()
		contentReader := strings.NewReader(content)
		putDrawingErr := store.PutDrawing(c, title, contentReader)
		if putDrawingErr != nil {
			logger.Error().Err(putDrawingErr).Str("title", title).Msg("failed to store drawing %s: %w")
			c.AbortWithError(500, putDrawingErr)
			return
		}
		c.Status(200)
	})

	rootEngine.GET("/drawing/:title", func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())

		title := c.Param("title")
		content, getContentErr := store.GetDrawing(c, title)
		if getContentErr != nil {
			logger.Error().Err(getContentErr).Str("title", title).Msg("failed to get drawing content")
			c.AbortWithError(500, getContentErr)
			return
		}
		logger.Debug().Str("title", title).Int("content length", len(content)).Msg("content found")
		c.JSON(200, content)
	})

	http.Serve(s.listener, rootEngine)
}
