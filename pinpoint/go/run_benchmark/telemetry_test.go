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
	b, err := NewBenchmarkTest(c, "android-pixel4_webview-perf", "", "performance_browser_tests", "story", "all")
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
	b, err := NewBenchmarkTest(c, "android-pixel4_webview-perf", "", "random_test", "", "")
	assert.NoError(t, err)

	cmd := b.GetCommand()

	// Non waterfall enabled gtest should have run_benchmark
	assert.Contains(t, cmd, "../../tools/perf/run_benchmark")
	assert.Contains(t, cmd, "--benchmarks")
	assert.Contains(t, cmd, "random_test")
}

func TestGetCommand_Crossbench_TestCommand(t *testing.T) {
	c := "01bfa421eee3c76bbbf32510343e074060051c9f"
	b, err := NewBenchmarkTest(c, "win-11-perf", "release", "speedometer3.1.crossbench", "default", "")
	assert.NoError(t, err)

	cmd := b.GetCommand()

	assert.Contains(t, cmd, "../../third_party/crossbench/cb.py")
	assert.NotContains(t, cmd, "../../tools/perf/run_benchmark")
	assert.Contains(t, cmd, "speedometer3.1.crossbench")
	assert.Contains(t, cmd, "speedometer_3.1")
}

func TestReplaceNonAlphaNumeric_WorksAsIntended(t *testing.T) {
	test := func(name, story string, expected string) {
		t.Run(story, func(t *testing.T) {
			storyRegex := replaceNonAlphaNumeric(story)
			assert.Equal(t, expected, storyRegex)
		})
	}
	story := "Speedometer2"
	test("alpha numeric only stays the same", story, story)

	story = "browse:media:tiktok_infinite_scroll:2021"
	expected := "browse.media.tiktok.infinite.scroll.2021"
	test("example story - non alpha numeric characters replaced", story, expected)

	story = "mse.html?media=aac_audio.mp4"
	expected = "mse.html.media.aac.audio.mp4"
	test("example story 2 - non alpha numeric characters replaced", story, expected)

	story = "._:/?=&"
	expected = "......."
	test("known non alpha numeric characters that appear in stories are replaced", story, expected)

	story = "!@#$%^&*()-=+[]{}./,`~_:?"
	expected = "........................."
	test("random non alpha numberic characters are covered", story, expected)
}
