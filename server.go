package main

import (
	"context"
	"crypto/rand"
	"encoding/gob"
	"encoding/json"
	"fmt"
	"gitstore"
	"io"
	"net"
	"net/http"
	"s3store"
	"strings"

	"github.com/gin-contrib/sessions"
	"github.com/gin-contrib/sessions/memstore"
	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
)

type drawingList map[string]string

type drawingRepo interface {
	PutDrawing(ctx context.Context, key string, contentReader io.Reader, modifiedBy string) error
	CopyDrawing(ctx context.Context, sourceId string, destinationId string, modifiedBy string) error
	ListDrawings(ctx context.Context) (map[string]string, error)
	GetDrawing(ctx context.Context, key string) (string, error)
	DeleteDrawing(ctx context.Context, key string, modifiedBy string) error
}

type drawingRepos map[string]drawingRepo

func (repo drawingRepos) repoWithAlias(alias string) drawingRepo {
	return repo[alias]
}

type server struct {
	ctx    context.Context
	config options
	stores drawingRepos
}

type putDrawingRequest struct {
	Content string `json:"content"`
}

func (s *server) start() {
	h := handlerFactory{
		s.stores,
	}

	port := s.config.port

	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		portSpec := fmt.Sprintf("port %d", port)
		if s.config.port == 0 {
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
	rootEngine.Use(checkBasicAuthentication(basicConfig{passwordCreds: s.config.passwordCreds}))

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
	repos drawingRepos
}

func addListFromStoreToFullList(sourceAlias string, list drawingList, fullList drawingList) {
	for key, title := range list {
		fullList[fmt.Sprintf("%s-%s", sourceAlias, key)] = title
	}
}

func (hf *handlerFactory) getDrawingListHandler() func(c *gin.Context) {
	return func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())

		fullList := drawingList{}

		for alias, store := range hf.repos {
			list, listErr := store.ListDrawings(c)
			addListFromStoreToFullList(alias, list, fullList)
			if listErr != nil {
				logger.Error().Err(listErr).Msg("failed to list drawing titles")
				c.AbortWithError(http.StatusInternalServerError, listErr)
				return
			}
		}

		c.JSON(http.StatusOK, fullList)
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

func splitAliasId(aliasKey string) (string, string) {
	split := strings.Split(aliasKey, "-")
	return split[0], split[1]
}

func (hf *handlerFactory) putDrawing(c *gin.Context, idParam string) {
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
		logger.Error().Err(userExtractErr).Str("id", idParam).Msg("failed to extract user from context")
		c.AbortWithError(http.StatusInternalServerError, userExtractErr)
		return
	}

	alias, drawingId := splitAliasId(idParam)

	repo := hf.repos.repoWithAlias(alias)
	putDrawingErr := repo.PutDrawing(c, drawingId, contentReader, user.Username)
	if putDrawingErr != nil {
		logger.Error().Err(putDrawingErr).Str("id", idParam).Msg("failed to store drawing %s: %w")
		c.AbortWithError(http.StatusInternalServerError, putDrawingErr)
		return
	}
}

func (hf *handlerFactory) getDrawingContent() func(c *gin.Context) {
	return func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())

		idParam := c.Param("id")
		alias, drawingId := splitAliasId(idParam)

		repo := hf.repos.repoWithAlias(alias)
		content, getContentErr := repo.GetDrawing(c, drawingId)
		if getContentErr != nil {
			logger.Error().Err(getContentErr).Str("id", idParam).Msg("failed to get drawing content")
			c.AbortWithError(http.StatusInternalServerError, getContentErr)
			return
		}
		logger.Debug().Str("id", idParam).Int("content length", len(content)).Msg("content found")
		c.JSON(http.StatusOK, content)
	}
}

func (hf *handlerFactory) deleteDrawing() func(c *gin.Context) {
	return func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())
		idParam := c.Param("id")
		if len(idParam) == 0 {
			logger.Debug().Msg("Missing 'id' path parameter")
			c.AbortWithStatus(http.StatusBadRequest)
			return
		}
		user, userExtractErr := getUserFromContext(c)
		if userExtractErr != nil {
			logger.Error().Err(userExtractErr).Str("id", idParam).Msg("failed to extract user from context")
			c.AbortWithError(http.StatusInternalServerError, userExtractErr)
			return
		}

		alias, drawingId := splitAliasId(idParam)
		repo := hf.repos.repoWithAlias(alias)

		err := repo.DeleteDrawing(c, drawingId, user.Username)
		if err != nil {
			logger.Error().Err(err).Msg("failed to delete the object with the old name")
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		c.Status(http.StatusOK)
	}
}

func newDrawingStore(ctx context.Context, drRepo drawingRepoConfig) drawingRepo {
	var repo drawingRepo

	switch drRepo.storeType {
	case LOCAL_GIT:
		logger := getLogger().With().Interface("drawingRepoDescriptor", drRepo).Logger()
		blobStore, repoErr := gitstore.NewLocalGitStore(drRepo.root, drRepo.path, &logger)
		if repoErr != nil {
			panic(repoErr)
		}
		repo = blobStore
	case GITLAB:
		panic("Not yet supported")
	case S3:
		blobStore, blobStoreErr := s3store.NewDrawingStore(ctx, "test-xcali-backend")
		if blobStoreErr != nil {
			panic(fmt.Sprintf("failed to created S3 store: %v", blobStoreErr))
		}
		repo = blobStore
	default:
		panic(fmt.Errorf("invalid drawingRepoDescriptor: %v", drRepo))
	}

	return repo
}

func newServer(drRepos drawingReposConfigs) (*server, error) {
	ctx := context.Background()

	stores := drawingRepos{}
	for alias, drRepo := range drRepos {
		stores[alias] = newDrawingStore(ctx, drRepo)
	}

	return &server{
		ctx: ctx,
		config: options{
			getServerPort(),
			[]passwordCredentials{{
				Username: getUsername(),
				Password: "pass",
			}},
			LOCAL_GIT,
		},
		stores: stores,
	}, nil
}
