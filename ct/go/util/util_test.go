package util

import (
	"fmt"
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

func TestGetIntFlagValue(t *testing.T) {
	testutils.SmallTest(t)
	assert.Equal(t, 4, GetIntFlagValue("--pageset-repeat=4", PAGESET_REPEAT_FLAG, 1))
	assert.Equal(t, 4, GetIntFlagValue("--pageset-repeat 4", PAGESET_REPEAT_FLAG, 1))
	// Use first value if multiple are specified.
	assert.Equal(t, 4, GetIntFlagValue("--pageset-repeat=4 --pageset-repeat=3", PAGESET_REPEAT_FLAG, 1))
	// Test that default value gets returned.
	assert.Equal(t, 2, GetIntFlagValue("", PAGESET_REPEAT_FLAG, 2))
	assert.Equal(t, 2, GetIntFlagValue("--pageset-repeatsssss=4", PAGESET_REPEAT_FLAG, 2))
	assert.Equal(t, 2, GetIntFlagValue("--somethingelse", PAGESET_REPEAT_FLAG, 2))
}

func TestGetBasePixelDiffRemoteDir(t *testing.T) {
	testutils.SmallTest(t)
	// Test valid runID.
	remoteDir, err := GetBasePixelDiffRemoteDir("rmistry-20170510163703")
	assert.NoError(t, err)
	assert.Equal(t, fmt.Sprintf("%s/2017/05/10/16/rmistry-20170510163703", PixelDiffRunsDir), remoteDir)
	// Test invalid runID.
	remoteDir, err = GetBasePixelDiffRemoteDir("blahblahblah")
	assert.Error(t, err)
	assert.Equal(t, "", remoteDir)
}

func TestRemoveFlagsFromArgs(t *testing.T) {
	testutils.SmallTest(t)
	assert.Equal(t, "", RemoveFlagsFromArgs("--pageset-repeat=4", PAGESET_REPEAT_FLAG))
	assert.Equal(t, "", RemoveFlagsFromArgs("--pageset-repeat=4 --run-benchmark-timeout=400", PAGESET_REPEAT_FLAG, RUN_BENCHMARK_TIMEOUT_FLAG))
	assert.Equal(t, "--abc", RemoveFlagsFromArgs("--pageset-repeat=4 --pageset-repeat=abc --pageset-repeat --abc", PAGESET_REPEAT_FLAG))
	assert.Equal(t, "", RemoveFlagsFromArgs("", PAGESET_REPEAT_FLAG))
	assert.Equal(t, "--abc", RemoveFlagsFromArgs("--abc", PAGESET_REPEAT_FLAG))
	assert.Equal(t, "--output-format=csv --traffic-setting=Regular-3G", RemoveFlagsFromArgs("--output-format=csv --run-benchmark-timeout=900 --traffic-setting=Regular-3G", RUN_BENCHMARK_TIMEOUT_FLAG))
}
