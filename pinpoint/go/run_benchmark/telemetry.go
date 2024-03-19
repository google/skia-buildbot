package run_benchmark

import (
	"fmt"

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

// Please keep this executable-argument mapping synced with perf waterfall:
// https://chromium.googlesource.com/chromium/src/+/main/tools/perf/core/bot_platforms.py
var waterfallEnabledGtests = util.NewStringSet([]string{
	"base_perftests",
	"components_perftests",
	"dawn_perf_tests",
	"load_library_perf_tests",
	"performance_browser_tests",
	"sync_performance_tests",
	"tracing_perftests",
	"views_perftests",
})

type telemetryTest struct {
	benchmark string
	browser   string
	commit    string
	story     string
	storyTags string
}

// getCommand generates the command needed to execute Telemetry benchmark tests.
// This function assumes that the request object has been validated, and that
// required fields have been verified for.
// TODO(b/318863812): support user submitted extra_args.
// This support is needed for pairwise executions, not bisection.
func (t *telemetryTest) GetCommand() []string {
	cmd := []string{
		"luci-auth",
		"context",
		"--",
		"vpython3",
		"../../testing/test_env.py",
		"../../testing/scripts/run_performance_tests.py",
	}

	// Note(jeffyoon@): The logic below can likely be made more efficient, but
	// for parity sake we retain the same order for arguments as Catapult's Pinpoint.
	// Ref: https://source.chromium.org/chromium/chromium/src/+/main:third_party/catapult/dashboard/dashboard/pinpoint/models/quest/run_telemetry_test.py;drc=10f23074bdab2b425cb29a8224b63298fdac42b7;l=127
	if _, ok := waterfallEnabledGtests[t.benchmark]; ok {
		if t.benchmark == "performance_browser_tests" {
			cmd = append(cmd, "browser_tests")
		} else {
			cmd = append(cmd, t.benchmark)
		}
	} else {
		cmd = append(cmd, "../../tools/perf/run_benchmark")
	}

	cmd = append(cmd, t.GetTelemetryExtraArgs()...)

	return cmd
}

func (t *telemetryTest) GetTelemetryExtraArgs() []string {
	cmd := []string{}
	if t.storyTags == "" {
		cmd = append(cmd, "-d")
	}

	if _, ok := waterfallEnabledGtests[t.benchmark]; ok {
		cmd = append(cmd, "--gtest-benchmark-name", t.benchmark, "--non-telemetry", "true")
		switch t.benchmark {
		case "base_perftests", "dawn_perf_tests", "sync_performance_tests":
			cmd = append(cmd, "--test-launcher-jobs=1", "--test-launcher-retry-limit=0")
		case "components_perftests", "views_perftests":
			cmd = append(cmd, "--xvfb")
		case "performance_browser_tests":
			// Allow the full performance runs to take up to 60 seconds (rather
			// than the default of 30 for normal CQ browser test runs).
			cmd = append(cmd, "--full-performance-run", "--test-launcher-jobs=1",
				"--test-launcher-retry-limit=0", "--ui-test-action-timeout=60000",
				"--ui-test-action-max-timeout=60000", "--test-launcher-timeout=60000",
				"--gtest_filter=*/TabCapturePerformanceTest.*:*/CastV2PerformanceTest.*")
		default:
			break
		}
	} else {
		cmd = append(cmd, "--benchmarks", t.benchmark)
	}

	if t.story != "" {
		// TODO(b/40635221): Note that usage of "--run-full-story-set" and "--story-filter"
		// can be replaced with --story=<story> (no regex needed). See crrev/c/1869800.
		cmd = append(cmd, "--story-filter", fmt.Sprintf("^%s$", t.story))
	}

	if t.storyTags != "" {
		cmd = append(cmd, "--story-tag-filter", t.storyTags)
	}

	cmd = append(cmd, "--pageset-repeat", "1", "--browser", t.browser)
	cmd = append(cmd, defaultExtraArgs...)

	// For results2 to differentiate between runs, we need to add the
	// Telemetry parameter `--results-label <change>` to the runs.
	// TODO(jeffyoon@) deprecate this label once the UI is no longer dependant on this.
	cmd = append(cmd, "--results-label", t.commit[:7])

	// Note: Appending "--run-full-story-set" last per comment above to retain
	// argument order. This is always appended in catapult regardless of whether
	// a story is defined or not.
	cmd = append(cmd, "--run-full-story-set")

	return cmd
}
