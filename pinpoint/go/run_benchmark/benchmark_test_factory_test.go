package run_benchmark

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetBenchmark_TelemetryTestBuilder_TelemetryTests(t *testing.T) {
	expect := func(commit, botConfig, browser, benchmark, story, storyTags string, want string) {
		bt, _ := NewBenchmarkTest(commit, botConfig, browser, benchmark, story, storyTags)
		assert.Equal(t, want, reflect.TypeOf(bt).String())
	}

	c, b := "fake-commit", "fake-browser"
	expect(c, "android-pixel6-perf", b, "components_perftests", "", "", "*run_benchmark.telemetryTest")
	expect(c, "android-pixel2_webview-perf", b, "performance_browser_tests", "story", "all", "*run_benchmark.telemetryTest")
}
