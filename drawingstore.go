package main

import (
	"context"
	"fmt"
	"gitstore"
	"s3store"
)

func newDrawingStore(ctx context.Context, repoConfig drawingRepoConfig) drawingRepo {
	var repo drawingRepo

	switch repoConfig.storeType {
	case LOCAL_GIT:
		logger := getLogger().With().Str("drawingRepoConfig", fmt.Sprintf("%v", repoConfig)).Logger()
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
		panic(fmt.Errorf("invalid drawingRepoConfig: %v", repoConfig))
	}

	return repo
}
