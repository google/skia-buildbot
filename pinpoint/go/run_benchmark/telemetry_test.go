package run_benchmark

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetCommand_WaterfallGTest_TestCommand(t *testing.T) {
	c := "01bfa421eee3c76bbbf32510343e074060051c9f"
	b, err := NewBenchmarkTest(c, "android-pixel6-perf", "", "components_perftests", "", "")

	assert.NoError(t, err)

	cmd := b.GetCommand()
	// stories w/o story tags should have -d set
	assert.Contains(t, cmd, "-d")
	assert.NotContains(t, cmd, "--story-tag-filter")

	// Should not contain run_benchmark because it's a waterfall
	// enabled GTest.
	assert.NotContains(t, cmd, "../../tools/perf/run_benchmark")
}

func TestGetCommand_PerfBrowserTestWithStory_StoryTestCommand(t *testing.T) {
	c := "01bfa421eee3c76bbbf32510343e074060051c9f"
	b, err := NewBenchmarkTest(c, "android-pixel2_webview-perf", "", "performance_browser_tests", "story", "all")
	assert.NoError(t, err)

	cmd := b.GetCommand()
	// stories w/o story tags should have -d set
	assert.Contains(t, cmd, "--story-filter")
	assert.Contains(t, cmd, "--story-tag-filter")

	// Special gtest filter for performance_browser_tests
	assert.Contains(t, cmd, "--gtest_filter=*/TabCapturePerformanceTest.*:*/CastV2PerformanceTest.*")
}

func TestGetCommand_NonWaterfallEnabledGTest_TestCommand(t *testing.T) {
	c := "01bfa421eee3c76bbbf32510343e074060051c9f"
	b, err := NewBenchmarkTest(c, "android-pixel2_webview-perf", "", "random_test", "", "")
	assert.NoError(t, err)

	cmd := b.GetCommand()

	// Non waterfall enabled gtest should have run_benchmark
	assert.Contains(t, cmd, "../../tools/perf/run_benchmark")
	assert.Contains(t, cmd, "--benchmarks")
	assert.Contains(t, cmd, "random_test")
}
