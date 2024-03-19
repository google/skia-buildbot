package run_benchmark

import (
	"slices"
	"strings"

	"go.skia.org/infra/go/skerr"

	bc "go.skia.org/infra/pinpoint/go/bot_configs"
)

type BenchmarkTest interface {
	GetCommand() []string
}

// NewBenchmarkTest returns a BenchmarkTest based on the request parameters.
// The Configuration (bot) is used alongside the benchmark to determine the
// isolate target for that combination. Based on the isolate target,
func NewBenchmarkTest(commit, botConfig, browser, benchmark, story, storyTags string) (BenchmarkTest, error) {
	config, err := bc.GetBotConfig(botConfig, false)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to fetch bot configs to create benchmark test")
	}
	target, err := bc.GetIsolateTarget(botConfig, benchmark)
	if err != nil {
		return nil, skerr.Wrapf(err, "Failed to get isolate target to create the benchmark test")
	}

	switch {
	// The following targets are specific to lacros telemetry test
	case slices.Contains([]string{"performance_test_suite_eve", "performance_test_suite_octopus"}, target):
		return NewLacrosTest(target, benchmark, config.Browser, commit, story, storyTags), nil
	// Few targets could have suffixes, especially for Android.
	// For example, 'performance_test_suite_android_clank_monochrome_64_32_bundle'
	case strings.Contains(target, "performance_test_suite") || strings.Contains(target, "telemetry_perf_tests") || target == "performance_webview_test_suite":
		return &telemetryTest{
			benchmark: benchmark,
			browser:   config.Browser,
			commit:    commit,
			story:     story,
			storyTags: storyTags,
		}, nil
	default:
		return nil, skerr.Fmt("Unsupported test target %s", target)
	}
}
