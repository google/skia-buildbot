package run_benchmark

import (
	"fmt"
	"regexp"

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

// Map from benchmark display name (as shown on the dashboard) to actual
// benchmark name used by crossbench.
// Please keep it in sync with benchmarks defined on perf waterfall:
// https://chromium.googlesource.com/chromium/src/+/main/tools/perf/core/bot_platforms.py
// Also keep it in sync with legacy Pinpoint while it still exists:
// https://chromium.googlesource.com/catapult/+/HEAD/dashboard/dashboard/pinpoint/models/quest/run_telemetry_test.py
var crossbenchBenchmarks = map[string]string{
	"jetstream2.crossbench":      "jetstream_2.2",
	"jetstream3.crossbench":      "jetstream_main",
	"jetstream-main.crossbench":  "jetstream_main",
	"motionmark1.3.crossbench":   "motionmark_1.3",
	"speedometer2.crossbench":    "speedometer_2",
	"speedometer2.0.crossbench":  "speedometer_2.0",
	"speedometer2.1.crossbench":  "speedometer_2.1",
	"speedometer3.crossbench":    "speedometer_3",
	"speedometer3.0.crossbench":  "speedometer_3.0",
	"speedometer3.1.crossbench":  "speedometer_3.1",
	"loadline_phone.crossbench":  "loadline-phone-fast",
	"loadline_tablet.crossbench": "loadline-tablet-fast",
}

type telemetryTest struct {
	benchmark string
	browser   string
	commit    string
	story     string
	storyTags string
	extraArgs []string
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
	} else if benchmark, ok := crossbenchBenchmarks[t.benchmark]; ok {
		cmd = append(cmd, "../../third_party/crossbench/cb.py")
		cmd = append(cmd, t.GetCrossbenchExtraArgs(benchmark)...)
		return cmd
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
		// story-filter is preferred over --story since stories can sometimes use
		// mismatched characters like ":" or "_"
		cmd = append(cmd, "--story-filter", fmt.Sprintf("^%s$", replaceNonAlphaNumeric(t.story)))
	}

	if t.storyTags != "" {
		cmd = append(cmd, "--story-tag-filter", t.storyTags)
	}

	cmd = append(cmd, "--pageset-repeat", "1", "--browser", t.browser)
	cmd = append(cmd, defaultExtraArgs...)

	// For results2 to differentiate between runs, we need to add the
	// Telemetry parameter `--results-label <change>` to the runs.
	// If the commit is less than 7 char, that implies the results2 are not used
	// TODO(b/411136326) deprecate this label once the UI is no longer dependant on this.
	if len(t.commit) >= 7 {
		cmd = append(cmd, "--results-label", t.commit[:7])
	}

	cmd = append(cmd, t.extraArgs...)

	return cmd
}

func (t *telemetryTest) GetCrossbenchExtraArgs(benchmark string) []string {
	cmd := []string{}
	cmd = append(cmd, "--benchmark-display-name", t.benchmark)
	cmd = append(cmd, "--benchmarks", benchmark)
	cmd = append(cmd, "--browser="+t.browser)
	cmd = append(cmd, "-v")
	cmd = append(cmd, "--isolated-script-test-output", "${ISOLATED_OUTDIR}/output.json")
	cmd = append(cmd, t.extraArgs...)

	return cmd
}

// replaceNonAlphaNumeric replaces all non alpha-numeric characters
// in a string with ".". In the story-filter arg, the string passed
// needs to be a regex. Periods are treated as any character in regex.
// Story names can sometimes have ambiguous names like browse:media or
// browse_media so this removes this ambiguity.
// This replacement matches behavior in legacy Pinpoint
func replaceNonAlphaNumeric(s string) string {
	var nonAlphanumericRegex = regexp.MustCompile(`[^a-zA-Z0-9 ]`)
	return nonAlphanumericRegex.ReplaceAllString(s, ".")
}
