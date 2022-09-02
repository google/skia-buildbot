package util

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	TEST_FILE_NAME          = "testingtesting"
	GS_TEST_TIMESTAMP_VALUE = "123"
)

func TestGetStartRange(t *testing.T) {
	require.Equal(t, 1, GetStartRange(1, 1000))
	require.Equal(t, 2001, GetStartRange(3, 1000))
	require.Equal(t, 41, GetStartRange(3, 20))
}

func TestGetPathToPyFiles(t *testing.T) {
	expectedLocalPathSuffix := filepath.Join("ct", "py")
	expectedSwarmingPathSuffix := "py"

	// Test local path.
	pathToPyFiles, err := GetPathToPyFiles(true /* local */)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(pathToPyFiles, expectedLocalPathSuffix))

	// Test swarming path.
	pathToPyFiles, err = GetPathToPyFiles(false /* local */)
	require.NoError(t, err)
	require.True(t, strings.HasSuffix(pathToPyFiles, expectedSwarmingPathSuffix))
}

func TestGetStrFlagValue(t *testing.T) {
	require.Equal(t, "desktop", GetStrFlagValue("--user-agent=desktop", USER_AGENT_FLAG, "default-value"))
	require.Equal(t, "desktop", GetStrFlagValue("--user-agent desktop", USER_AGENT_FLAG, "default-value"))
	// Use first value if multiple are specified.
	require.Equal(t, "desktop", GetStrFlagValue("--user-agent=desktop --user-agent=mobile", USER_AGENT_FLAG, "default-value"))
	// Test that default value gets returned.
	require.Equal(t, "default-value", GetStrFlagValue("", USER_AGENT_FLAG, "default-value"))
	require.Equal(t, "default-value", GetStrFlagValue("--user-agentsssss=desktop", USER_AGENT_FLAG, "default-value"))
	require.Equal(t, "default-value", GetStrFlagValue("--somethingelse", USER_AGENT_FLAG, "default-value"))
}

func TestGetIntFlagValue(t *testing.T) {
	require.Equal(t, 4, GetIntFlagValue("--pageset-repeat=4", PAGESET_REPEAT_FLAG, 1))
	require.Equal(t, 4, GetIntFlagValue("--pageset-repeat 4", PAGESET_REPEAT_FLAG, 1))
	// Use first value if multiple are specified.
	require.Equal(t, 4, GetIntFlagValue("--pageset-repeat=4 --pageset-repeat=3", PAGESET_REPEAT_FLAG, 1))
	// Test that default value gets returned.
	require.Equal(t, 2, GetIntFlagValue("", PAGESET_REPEAT_FLAG, 2))
	require.Equal(t, 2, GetIntFlagValue("--pageset-repeatsssss=4", PAGESET_REPEAT_FLAG, 2))
	require.Equal(t, 2, GetIntFlagValue("--somethingelse", PAGESET_REPEAT_FLAG, 2))
}

func TestRemoveFlagsFromArgs(t *testing.T) {
	require.Equal(t, "", RemoveFlagsFromArgs("--pageset-repeat=4", PAGESET_REPEAT_FLAG))
	require.Equal(t, "", RemoveFlagsFromArgs("--pageset-repeat=4 --run-benchmark-timeout=400", PAGESET_REPEAT_FLAG, RUN_BENCHMARK_TIMEOUT_FLAG))
	require.Equal(t, "--abc", RemoveFlagsFromArgs("--pageset-repeat=4 --pageset-repeat=abc --pageset-repeat --abc", PAGESET_REPEAT_FLAG))
	require.Equal(t, "", RemoveFlagsFromArgs("", PAGESET_REPEAT_FLAG))
	require.Equal(t, "--abc", RemoveFlagsFromArgs("--abc", PAGESET_REPEAT_FLAG))
	require.Equal(t, "--output-format=csv --traffic-setting=Regular-3G", RemoveFlagsFromArgs("--output-format=csv --run-benchmark-timeout=900 --traffic-setting=Regular-3G", RUN_BENCHMARK_TIMEOUT_FLAG))
}
