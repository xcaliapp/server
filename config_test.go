package main

import (
	"testing"

	"github.com/stretchr/testify/suite"
)

type readConfigurationTestSuite struct {
	suite.Suite
}

func TestReadConfiguration(t *testing.T) {
	suite.Run(t, &readConfigurationTestSuite{})
}

func (t *readConfigurationTestSuite) TestGetDrawingSets() {
	wsgwAlias := "wsgw"
	wsgwRoot := "wsgw-root"
	wsgwPath := "wsgw_path"
	xcaliAlias := "xcali"
	xcaliRoot := "xcali-root"
	xcaliPath := "xcali_path"
	t.T().Setenv("XCALIAPP_DRAWINGSET_wsgw_ROOT", wsgwRoot)
	t.T().Setenv("XCALIAPP_DRAWINGSET_wsgw_PATH", wsgwPath)
	t.T().Setenv("XCALIAPP_DRAWINGSET_xcali_ROOT", xcaliRoot)
	t.T().Setenv("XCALIAPP_DRAWINGSET_xcali_PATH", xcaliPath)

	expectedSets := drawingSets{
		wsgwAlias: drawingSet{
			root: wsgwRoot,
			path: wsgwPath,
		},
		xcaliAlias: drawingSet{
			root: xcaliRoot,
			path: xcaliPath,
		},
	}

	drawingSets, err := getDrawingSets()

	t.NoError(err)
	t.Equal(expectedSets, drawingSets)
}
