package run_benchmark

import (
	"fmt"

	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/util"
)

var defaultExtraArgs = []string{
	"-v",
	"--upload-results",
	"--output-format",
	"histograms",
	"--isolated-script-test-output",
	"${ISOLATED_OUTDIR}/output.json",
}

// TODO(b/318863812): implement benchmarks in _WATERFALL_ENABLED_GTEST_NAMES
// crbug/1146949
// Please keep this executable-argument mapping synced with perf waterfall:
// https://chromium.googlesource.com/chromium/src/+/main/tools/perf/core/bot_platforms.py
var waterfallEnabledGtests = util.NewStringSet([]string{
	"base_perftests",
	"components_perftests",
	"dawn_perf_tests",
	"gpu_perftests",
	"load_library_perf_tests",
	"performance_browser_tests",
	"sync_performance_tests",
	"tracing_perftests",
	"views_perftests",
})

// A telemetryExp contains the command for the test device
// to run a telemetry benchmark on Chrome browser.
// telemetryExp will support most devices.
type telemetryExp struct{}

// createCmd creates the command for the test device.
func (t *telemetryExp) createCmd(req RunBenchmarkRequest) ([]string, error) {
	// TODO(b/318863812): implement benchmarks in _WATERFALL_ENABLED_GTEST_NAMES
	// Benchmarks in _WATERFALL_ENABLED_GTEST_NAMES are rarely used.
	// see: https://source.chromium.org/chromium/chromium/src/+/main:third_party/catapult/dashboard/dashboard/pinpoint/models/quest/run_telemetry_test.py;drc=10f23074bdab2b425cb29a8224b63298fdac42b7;l=96
	_, ok := waterfallEnabledGtests[req.Benchmark]
	if ok {
		return nil, skerr.Fmt("Benchmark %s is not yet implemented", req.Benchmark)
	}
	cmd := []string{
		"luci-auth",
		"context",
		"--",
		"vpython3",
		"../../testing/test_env.py",
		"../../testing/scripts/run_performance_tests.py",
		"../../tools/perf/run_benchmark",
		// TODO(b/318863812): Implement story_tags. Some of our entrenched users
		// are only familiar with running Pinpoint jobs with story tags. Although
		// some of this behavior is due to UI confusion, we should still support it
		// and ensure the UI makes it clear what the differences are.
		// see: https://source.chromium.org/chromium/chromium/src/+/main:third_party/catapult/dashboard/dashboard/pinpoint/models/quest/run_telemetry_test.py;drc=10f23074bdab2b425cb29a8224b63298fdac42b7;l=127
		"-d",
	}
	if req.Benchmark == "" {
		return nil, skerr.Fmt("Missing 'Benchmark' argument.")
	}
	cmd = append(cmd, "--benchmarks", req.Benchmark)
	if req.Story == "" {
		return nil, skerr.Fmt("Missing 'Story' argument.")
	}
	cmd = append(cmd, "--story-filter", fmt.Sprintf("^%s$", req.Story))
	cmd = append(cmd, "--pageset-repeat", "1")
	cmd = append(cmd, "browser", req.Config.Browser)
	cmd = append(cmd, defaultExtraArgs...)

	// TODO(b/318863812): Add support for non-chromium commits
	// append sha for results2
	if req.Commit == "" {
		return nil, skerr.Fmt("Missing 'Commit' argument.")
	}
	cmd = append(cmd,
		"--results-label",
		fmt.Sprintf("chromium@%s", req.Commit[:7]))

	// unclear what happens if you write the flags in
	// a different order
	// see: https://source.chromium.org/chromium/chromium/src/+/main:third_party/catapult/dashboard/dashboard/pinpoint/models/quest/run_telemetry_test.py;drc=10f23074bdab2b425cb29a8224b63298fdac42b7;l=80
	cmd = append(cmd, "--run-full-story-set")

	return cmd, nil
}
