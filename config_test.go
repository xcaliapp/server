package main

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/suite"
)

type readConfigurationTestSuite struct {
	suite.Suite
}

func TestReadConfiguration(t *testing.T) {
	suite.Run(t, &readConfigurationTestSuite{})
}

func (t *readConfigurationTestSuite) TestGetDrawingRepoConfig() {
	wsgwName := "wsgw"
	wsgwLabel := "WebSocket Gateway"
	wsgwRoot := "wsgw-root"
	wsgwPath := "wsgw_path"
	xcaliName := "xcali"
	xcaliLabel := "XCalidraw Application"
	xcaliRoot := "xcali-root"
	xcaliPath := "xcali_path"
	t.T().Setenv("XCALIAPP_DRAWINGREPO_LIST", fmt.Sprintf("%s:%s,%s:%s", wsgwName, wsgwLabel, xcaliName, xcaliLabel))
	t.T().Setenv("XCALIAPP_DRAWINGREPO_wsgw_ROOT", wsgwRoot)
	t.T().Setenv("XCALIAPP_DRAWINGREPO_wsgw_PATH", wsgwPath)
	t.T().Setenv("XCALIAPP_DRAWINGREPO_xcali_ROOT", xcaliRoot)
	t.T().Setenv("XCALIAPP_DRAWINGREPO_xcali_PATH", xcaliPath)

	expectedSets := drawingReposConfigs{
		wsgwName: drawingRepoConfig{
			name:  wsgwName,
			label: wsgwLabel,
			root:  wsgwRoot,
			path:  wsgwPath,
		},
		xcaliName: drawingRepoConfig{
			name:  xcaliName,
			label: xcaliLabel,
			root:  xcaliRoot,
			path:  xcaliPath,
		},
	}

	drawingRepos, err := getDrawingRepoConfigs()

	t.NoError(err)
	t.Equal(expectedSets, drawingRepos)
}
