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

type drawingId = string
type drawingTitle = string

type drawingRepo interface {
	PutDrawing(ctx context.Context, key string, contentReader io.Reader, modifiedBy string) error
	CopyDrawing(ctx context.Context, sourceId string, destinationId string, modifiedBy string) error
	ListDrawings(ctx context.Context) (map[drawingId]drawingTitle, error)
	GetDrawing(ctx context.Context, key string) (string, error)
	DeleteDrawing(ctx context.Context, key string, modifiedBy string) error
}

type drawingRepoName string
type drawingRepoLabel string
type drawingRepoRef struct {
	Name  drawingRepoName  `json:"name"`
	Label drawingRepoLabel `json:"label"`
}
type drawingRepos map[drawingRepoRef]drawingRepo

func (repos drawingRepos) getRepo(repoName drawingRepoName) (drawingRepo, bool) {
	for repoRef, repo := range repos {
		if repoRef.Name == repoName {
			return repo, true
		}
	}
	return nil, false
}

type drawingRepoItem struct {
	Id    drawingId    `json:"id"`
	Title drawingTitle `json:"title"`
}
type drawingRepoContent struct {
	RepoRef drawingRepoRef    `json:"repoRef"`
	Items   []drawingRepoItem `json:"items"`
}

type drawingLists map[drawingRepoName]drawingRepoContent

type server struct {
	ctx    context.Context
	config options
	repos  drawingRepos
}

type putDrawingRequest struct {
	Content string `json:"content"`
}

func (s *server) start() {
	h := handlerFactory{
		s.repos,
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
	api.GET("/drawings", h.getDrawingListsHandler())
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

func addListFromStoreToFullList(repoRef drawingRepoRef, list map[drawingId]drawingTitle, fullList drawingLists) {
	content := drawingRepoContent{RepoRef: repoRef}
	for key, title := range list {
		content.Items = append(content.Items, drawingRepoItem{
			Id:    key,
			Title: title,
		})
	}
	fullList[repoRef.Name] = content
}

func (hf *handlerFactory) getDrawingListsHandler() func(c *gin.Context) {
	return func(c *gin.Context) {
		logger := zerolog.Ctx(c.Request.Context())

		fullList := drawingLists{}

		for repoRef, store := range hf.repos {
			list, listErr := store.ListDrawings(c)
			addListFromStoreToFullList(repoRef, list, fullList)
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

func splitRepoNameFromId(aliasKey string) (string, string) {
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

	repoName, drawingId := splitRepoNameFromId(idParam)

	repo, hasRepo := hf.repos.getRepo(drawingRepoName(repoName))
	if !hasRepo {
		logger.Error().Str("id", idParam).Str("repoName", repoName).Msg("failed to find repo")
		c.AbortWithStatus(http.StatusInternalServerError)
		return
	}

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
		repoName, drawingId := splitRepoNameFromId(idParam)

		repo, hasRepo := hf.repos.getRepo(drawingRepoName(repoName))
		if !hasRepo {
			logger.Error().Str("id", idParam).Str("repoName", repoName).Msg("failed to find repo")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

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

		repoName, drawingId := splitRepoNameFromId(idParam)
		repo, hasRepo := hf.repos.getRepo(drawingRepoName(repoName))
		if !hasRepo {
			logger.Error().Str("id", idParam).Str("repoName", repoName).Msg("failed to find repo")
			c.AbortWithStatus(http.StatusInternalServerError)
			return
		}

		err := repo.DeleteDrawing(c, drawingId, user.Username)
		if err != nil {
			logger.Error().Err(err).Msg("failed to delete the object with the old name")
			c.AbortWithError(http.StatusInternalServerError, err)
			return
		}
		c.Status(http.StatusOK)
	}
}

func newDrawingStore(ctx context.Context, repoConfig drawingRepoConfig) drawingRepo {
	var repo drawingRepo

	switch repoConfig.storeType {
	case LOCAL_GIT:
		logger := getLogger().With().Interface("drawingRepoDescriptor", repoConfig).Logger()
		blobStore, repoErr := gitstore.NewLocalGitStore(repoConfig.root, repoConfig.path, &logger)
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
		panic(fmt.Errorf("invalid drawingRepoDescriptor: %v", repoConfig))
	}

	return repo
}

func newServer(repoConfigs drawingReposConfigs) (*server, error) {
	ctx := context.Background()

	repos := drawingRepos{}
	for name, repoConfig := range repoConfigs {
		repos[drawingRepoRef{drawingRepoName(name), drawingRepoLabel(repoConfig.label)}] = newDrawingStore(ctx, repoConfig)
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
		repos: repos,
	}, nil
}
