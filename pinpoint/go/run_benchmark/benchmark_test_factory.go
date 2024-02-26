package run_benchmark

import (
	"strings"

	"go.skia.org/infra/go/skerr"

	bc "go.skia.org/infra/pinpoint/go/bot_configs"
	ppb "go.skia.org/infra/pinpoint/proto/v1"
)

type BenchmarkTest interface {
	GetCommand() []string
}

// NewBenchmarkTest returns a BenchmarkTest based on the request parameters.
// The Configuration (bot) is used alongside the benchmark to determine the
// isolate target for that combination. Based on the isolate target,
func NewBenchmarkTest(req *ppb.ScheduleBisectRequest, commit string) (BenchmarkTest, error) {
	bot, benchmark := req.Configuration, req.Benchmark
	config, err := bc.GetBotConfig(bot, false)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to fetch bot configs to create benchmark test")
	}
	target, err := bc.GetIsolateTarget(bot, benchmark)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get isolate target to create the benchmark test")
	}

	// These targets could have suffixes, especially for Android.
	// For example, 'performance_test_suite_android_clank_monochrome_64_32_bundle'
	if strings.Contains(target, "performance_test_suite") || strings.Contains(target, "telemetry_perf_tests") {
		return &telemetryTest{
			benchmark: req.Benchmark,
			browser:   config.Browser,
			commit:    commit,
			story:     req.Story,
			storyTags: req.StoryTags,
		}, nil
	}

	switch target {
	case "performance_webview_test_suite":
		return &telemetryTest{
			benchmark: req.Benchmark,
			browser:   config.Browser,
			commit:    commit,
			story:     req.Story,
			storyTags: req.StoryTags,
		}, nil
	default:
		return nil, skerr.Fmt("Unsupported test target %s", target)
	}
}
