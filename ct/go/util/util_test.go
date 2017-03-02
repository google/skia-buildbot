package util

import (
	"path/filepath"
	"strings"
	"testing"

	"go.skia.org/infra/go/testutils"

	assert "github.com/stretchr/testify/require"
)

const (
	TEST_FILE_NAME          = "testingtesting"
	GS_TEST_TIMESTAMP_VALUE = "123"
)

func TestGetStartRange(t *testing.T) {
	testutils.SmallTest(t)
	assert.Equal(t, 1, GetStartRange(1, 1000))
	assert.Equal(t, 2001, GetStartRange(3, 1000))
	assert.Equal(t, 41, GetStartRange(3, 20))
}

func TestGetPathToPyFiles(t *testing.T) {
	testutils.SmallTest(t)
	swarmingPath := GetPathToPyFiles(true)
	assert.True(t, strings.HasSuffix(swarmingPath, filepath.Join("src", "go.skia.org", "infra", "ct", "py")))
	nonSwarmingPath := GetPathToPyFiles(false)
	assert.True(t, strings.HasSuffix(nonSwarmingPath, filepath.Join("src", "go.skia.org", "infra", "ct", "py")))
}

func TestGetRepeatValue(t *testing.T) {
	testutils.SmallTest(t)
	assert.Equal(t, 4, GetRepeatValue("--pageset-repeat=4", 1))
	assert.Equal(t, 4, GetRepeatValue("--pageset-repeat 4", 1))
	// Use first value if multiple are specified.
	assert.Equal(t, 4, GetRepeatValue("--pageset-repeat=4 --pageset-repeat=3", 1))
	// Test that default value gets returned.
	assert.Equal(t, 2, GetRepeatValue("", 2))
	assert.Equal(t, 2, GetRepeatValue("--pageset-repeatsssss=4", 2))
	assert.Equal(t, 2, GetRepeatValue("--somethingelse", 2))
}
