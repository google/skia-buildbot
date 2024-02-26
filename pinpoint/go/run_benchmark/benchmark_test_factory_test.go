package run_benchmark

import (
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"

	ppb "go.skia.org/infra/pinpoint/proto/v1"
)

func TestGetBenchmark_TelemetryTestBuilder_TelemetryTests(t *testing.T) {
	expect := func(req *ppb.ScheduleBisectRequest, commit, want string) {
		benchmark, _ := NewBenchmarkTest(req, commit)
		assert.Equal(t, want, reflect.TypeOf(benchmark).String())
	}

	commit := "01bfa421eee3c76bbbf32510343e074060051c9f"
	req := &ppb.ScheduleBisectRequest{
		Configuration: "android-go-perf",
		Benchmark:     "components_perftests",
	}
	expect(req, commit, "*run_benchmark.telemetryTest")

	req = &ppb.ScheduleBisectRequest{
		Story:         "story",
		StoryTags:     "all",
		Configuration: "android-pixel2_webview-perf",
		Benchmark:     "performance_browser_tests",
	}
	expect(req, commit, "*run_benchmark.telemetryTest")
}
