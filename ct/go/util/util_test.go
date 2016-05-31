package util

import (
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

import (
	assert "github.com/stretchr/testify/require"
)

const (
	TEST_FILE_NAME          = "testingtesting"
	GS_TEST_TIMESTAMP_VALUE = "123"
)

func TestGetCTBareMetalWorkers(t *testing.T) {
	workers := GetCTBareMetalWorkers()
	for i := 0; i < NUM_BARE_METAL_MACHINES; i++ {
		assert.Equal(t, fmt.Sprintf(BARE_METAL_NAME_TEMPLATE, i+1), workers[i])
	}
}

func TestGetMasterLogLink(t *testing.T) {
	expectedLink := fmt.Sprintf("%s/util.test.%s.%s.log.INFO.rmistry-1440425450.02", MASTER_LOGSERVER_LINK, MASTER_NAME, CtUser)
	actualLink := GetMasterLogLink("rmistry-1440425450.02")
	assert.Equal(t, expectedLink, actualLink)
}

func TestGetStartRange(t *testing.T) {
	assert.Equal(t, 1, GetStartRange(1, 1000))
	assert.Equal(t, 2001, GetStartRange(3, 1000))
	assert.Equal(t, 41, GetStartRange(3, 20))
}

func TestGetPathToPyFiles(t *testing.T) {
	swarmingPath := GetPathToPyFiles(true)
	assert.True(t, strings.HasSuffix(swarmingPath, filepath.Join("src", "go.skia.org", "infra", "ct", "py")))
	nonSwarmingPath := GetPathToPyFiles(false)
	assert.True(t, strings.HasSuffix(nonSwarmingPath, filepath.Join("src", "go.skia.org", "infra", "ct", "py")))
}
