package util

import (
	"path/filepath"
	"strings"
	"testing"

	assert "github.com/stretchr/testify/require"
	"go.skia.org/infra/go/testutils/unittest"
)

const (
	TEST_FILE_NAME          = "testingtesting"
	GS_TEST_TIMESTAMP_VALUE = "123"
)

func TestGetStartRange(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, 1, GetStartRange(1, 1000))
	assert.Equal(t, 2001, GetStartRange(3, 1000))
	assert.Equal(t, 41, GetStartRange(3, 20))
}

func TestGetPathToPyFiles(t *testing.T) {
	unittest.SmallTest(t)
	expectedLocalPathSuffix := filepath.Join("ct", "py")
	expectedSwarmingPathSuffix := "py"

	// Test local path.
	pathToPyFiles := GetPathToPyFiles(true /* local */)
	assert.True(t, strings.HasSuffix(pathToPyFiles, expectedLocalPathSuffix))

	// Test swarming path.
	pathToPyFiles = GetPathToPyFiles(false /* local */)
	assert.True(t, strings.HasSuffix(pathToPyFiles, expectedSwarmingPathSuffix))
}

func TestGetIntFlagValue(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, 4, GetIntFlagValue("--pageset-repeat=4", PAGESET_REPEAT_FLAG, 1))
	assert.Equal(t, 4, GetIntFlagValue("--pageset-repeat 4", PAGESET_REPEAT_FLAG, 1))
	// Use first value if multiple are specified.
	assert.Equal(t, 4, GetIntFlagValue("--pageset-repeat=4 --pageset-repeat=3", PAGESET_REPEAT_FLAG, 1))
	// Test that default value gets returned.
	assert.Equal(t, 2, GetIntFlagValue("", PAGESET_REPEAT_FLAG, 2))
	assert.Equal(t, 2, GetIntFlagValue("--pageset-repeatsssss=4", PAGESET_REPEAT_FLAG, 2))
	assert.Equal(t, 2, GetIntFlagValue("--somethingelse", PAGESET_REPEAT_FLAG, 2))
}

func TestRemoveFlagsFromArgs(t *testing.T) {
	unittest.SmallTest(t)
	assert.Equal(t, "", RemoveFlagsFromArgs("--pageset-repeat=4", PAGESET_REPEAT_FLAG))
	assert.Equal(t, "", RemoveFlagsFromArgs("--pageset-repeat=4 --run-benchmark-timeout=400", PAGESET_REPEAT_FLAG, RUN_BENCHMARK_TIMEOUT_FLAG))
	assert.Equal(t, "--abc", RemoveFlagsFromArgs("--pageset-repeat=4 --pageset-repeat=abc --pageset-repeat --abc", PAGESET_REPEAT_FLAG))
	assert.Equal(t, "", RemoveFlagsFromArgs("", PAGESET_REPEAT_FLAG))
	assert.Equal(t, "--abc", RemoveFlagsFromArgs("--abc", PAGESET_REPEAT_FLAG))
	assert.Equal(t, "--output-format=csv --traffic-setting=Regular-3G", RemoveFlagsFromArgs("--output-format=csv --run-benchmark-timeout=900 --traffic-setting=Regular-3G", RUN_BENCHMARK_TIMEOUT_FLAG))
}
