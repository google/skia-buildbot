package bot_configs

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetIsolateTarget_WithConfigDefinedBot_ReturnsTarget(t *testing.T) {
	target, err := GetIsolateTarget("android-pixel4-perf", "benchmark")
	assert.Equal(t, "performance_test_suite_android_trichrome_chrome_google_64_32_bundle", target)
	assert.NoError(t, err)
}

func TestGetIsolateTarget_WithPixel10Perf_ReturnsTarget(t *testing.T) {
	target, err := GetIsolateTarget("android-pixel10-perf", "benchmark")
	assert.Equal(t, "performance_test_suite_android_trichrome_chrome_google_64_32_bundle", target)
	assert.NoError(t, err)
}

func TestGetIsolateTarget_WithPixel10CbbBot_ReturnsTarget(t *testing.T) {
	target, err := GetIsolateTarget("android-pixel10-perf-cbb", "benchmark")
	assert.Equal(t, "performance_test_suite_android_trichrome_chrome_google_64_32_bundle", target)
	assert.NoError(t, err)
}

func TestGetIsolateTarget_WithRegexMatching_ReturnsTarget(t *testing.T) {
	target, err := GetIsolateTarget("android-pixel4_webview-perf", "benchmark")
	assert.Equal(t, "performance_webview_test_suite", target)
	assert.NoError(t, err)
}

func TestGetIsolateTarget_WithConfigUnlistedBot_ReturnsTarget(t *testing.T) {
	target, err := GetIsolateTarget("linux-perf", "benchmark")
	assert.Equal(t, "performance_test_suite", target)
	assert.NoError(t, err)
}

func TestGetIsolateTarget_WithWebRTCBenchmark_ReturnsTarget(t *testing.T) {
	target, err := GetIsolateTarget("linux-perf", "webrtc_perf_tests")
	assert.Equal(t, "webrtc_perf_tests", target)
	assert.NoError(t, err)
}

func TestGetIsolateTarget_BotNotListedInBotConfigs_ReturnsError(t *testing.T) {
	_, err := GetIsolateTarget("fake device", "benchmark")
	require.Error(t, err)
}
