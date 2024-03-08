package bot_configs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetIsolateTarget_WithConfigDefinedBot_ReturnsTarget(t *testing.T) {
	target, err := GetIsolateTarget("android-pixel4-perf", "benchmark")
	assert.Equal(t, target, "performance_test_suite_android_clank_trichrome_chrome_google_64_32_bundle")
	assert.NoError(t, err)
}

func TestGetIsolateTarget_WithRegexMatching_ReturnsTarget(t *testing.T) {
	target, err := GetIsolateTarget("android-pixel4_webview-perf", "benchmark")
	assert.Equal(t, target, "performance_webview_test_suite")
	assert.NoError(t, err)
}

func TestGetIsolateTarget_WithConfigUnlistedBot_ReturnsTarget(t *testing.T) {
	target, err := GetIsolateTarget("linux-perf", "benchmark")
	assert.Equal(t, target, "performance_test_suite")
	assert.NoError(t, err)
}

func TestGetIsolateTarget_WithWebRTCBenchmark_ReturnsTarget(t *testing.T) {
	target, err := GetIsolateTarget("linux-perf", "webrtc_perf_tests")
	assert.Equal(t, target, "webrtc_perf_tests")
	assert.NoError(t, err)
}

func TestGetIsolateTarget_BotNotListedInBotConfigs_ReturnsError(t *testing.T) {
	_, err := GetIsolateTarget("fake device", "benchmark")
	require.Error(t, err)
}
