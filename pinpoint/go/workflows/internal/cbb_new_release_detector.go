package internal

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bazelbuild/remote-apis-sdks/go/pkg/digest"
	repb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"go.skia.org/infra/cabe/go/backends"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// NewReleaseWorkflowTimeout is the overall workflow timeout.
// Large timeout implemented due to crbug.com/428723126
const NewReleaseWorkflowTimeout time.Duration = 240 * time.Minute

// ClSubmissionTimeout is waiting time to submit a CL.
// It should be about 10 minutes less than the overall timeout.
const ClSubmissionTimeout time.Duration = 230 * time.Minute

var (
	// Activity options for detecting new releases and committing build info to
	// the chromium/src repository.
	releaseDetectorActivityOptions = workflow.ActivityOptions{
		StartToCloseTimeout: NewReleaseWorkflowTimeout,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	}

	httpClient *http.Client = httputils.NewTimeoutClient()
)

// ChromeReleaseInfo contains the detected new Chrome release info.
type ChromeReleaseInfo struct {
	CommitPosition string
	CommitHash     string
	Builds         []BuildInfo
}

// Bot configs that are available in each platform. This should eventually be
// put in a config file, but is currently stored here for simplicity.
var platformBots = map[string][]string{
	"mac": {"mac-m3-pro-perf-cbb"},
	"windows": {
		"win-victus-perf-cbb",
		"win-arm64-snapdragon-elite-perf-cbb",
	},
	"android": {"android-pixel-tangor-perf-cbb"},
}

const (
	// Pseudo benchmark and filename to retrieve Edge and Safari versions from devices.
	// Must match the strings used in
	// https://source.chromium.org/chromium/chromium/src/+/main:testing/scripts/run_performance_tests.py.
	browserVersionsBenchmark = "browser_versions"
	browserVersionsFilename  = "browser_versions.json"
)

// All Mac bot config names, with the number of devices in each config.
var safariConfigs = map[string]int32{
	"mac-m3-pro-perf-cbb": 5,
}

// All Windows bot config names, with the number of devices in each config.
var edgeConfigs = map[string]int32{
	"win-victus-perf-cbb":                 4,
	"win-arm64-snapdragon-elite-perf-cbb": 4,
}

// CbbNewReleaseDetectorWorkflow is the most basic Workflow Defintion.
func CbbNewReleaseDetectorWorkflow(ctx workflow.Context) (*ChromeReleaseInfo, error) {
	ctx = workflow.WithActivityOptions(ctx, releaseDetectorActivityOptions)

	// Is this workflow running locally on a dev machine?
	isDev := strings.HasPrefix(workflow.GetActivityOptions(ctx).TaskQueue, "localhost.")

	var bucket string
	if isDev {
		bucket = "chrome-perf-experiment-non-public"
	} else {
		bucket = "chrome-perf-non-public"
	}

	ctx = workflow.WithChildOptions(ctx, getBrowerVersionsWorkflowOptions)
	var safariVersions []BuildInfo
	if err := workflow.ExecuteChildWorkflow(ctx, workflows.CbbGetBrowserVersions, "safari").Get(ctx, &safariVersions); err != nil {
		// Log and ignore the error. This child workflow is expected to fail
		// occasionally, when the browsers are in the middle of an update.
		// Simply ignore that browser and try again next time. No need to fail
		// the entire workflow.
		sklog.Errorf("Unable to get current Safari versions: %v", err)
	}

	var edgeVersions []BuildInfo
	if err := workflow.ExecuteChildWorkflow(ctx, workflows.CbbGetBrowserVersions, "edge").Get(ctx, &edgeVersions); err != nil {
		sklog.Errorf("Unable to get current Edge versions: %v", err)
	}

	otherBrowsers := append(safariVersions, edgeVersions...)

	var commitInfo ChromeReleaseInfo
	if err := workflow.ExecuteActivity(ctx, GetChromeReleasesInfoActivity, otherBrowsers, isDev).Get(ctx, &commitInfo); err != nil {
		return nil, skerr.Wrap(err)
	}
	log.Printf("commitInfo:%v", commitInfo)

	if len(commitInfo.Builds) > 0 {
		// Trigger CBB benchmark runs.
		ctx = workflow.WithChildOptions(ctx, runBenchmarkWorkflowOptions)
		commit := common.NewCombinedCommit(common.NewChromiumCommit(commitInfo.CommitHash))
		cp, err := strconv.ParseInt(commitInfo.CommitPosition, 10, 32)
		if err != nil {
			return nil, skerr.Wrapf(err, "Invalid commit position %s", commitInfo.CommitPosition)
		}
		commit.Main.CommitPosition = int32(cp)
		wg := workflow.NewWaitGroup(ctx)
		for _, build := range commitInfo.Builds {
			for _, bot := range platformBots[build.Platform] {
				wg.Add(1)
				workflow.Go(ctx, func(gCtx workflow.Context) {
					defer wg.Done()
					p := &CbbRunnerParams{
						BotConfig:  bot,
						Commit:     commit,
						Browser:    build.Browser,
						Channel:    build.Channel,
						SkipFinch:  false,
						Benchmarks: nil, // nil means run the standard set of benchmarks
						Bucket:     bucket,
					}
					var cr *CommitRun
					if err := workflow.ExecuteChildWorkflow(ctx, workflows.CbbRunner, p).Get(gCtx, &cr); err != nil {
						sklog.Errorf("Error in CBB runner %#v: %v", p, err)
					}

					if build.Browser == "chrome" && build.Platform != "android" {
						// With Chrome on desktop, re-run all benchmarks with Finch control disabled.
						p.SkipFinch = true
						if err = workflow.ExecuteChildWorkflow(ctx, workflows.CbbRunner, p).Get(gCtx, &cr); err != nil {
							sklog.Errorf("Error in CBB runner %#v: %v", p, err)
						}
					}
				})
			}
		}

		wg.Wait(ctx)
	}

	// Check for new Safari Technology Preview and download it. This is
	// intentionally done at the end of the workflow, to avoid new Safari TP
	// being installed on test devices while we're running CBB benchmarks.
	var stpVersion string
	if err := workflow.ExecuteActivity(ctx, DownloadSafariTPActivity, isDev).Get(ctx, &stpVersion); err != nil {
		return nil, skerr.Wrap(err)
	}

	return &commitInfo, nil
}

// Verify that two BuildInfo slices have the same contents, ignoring ordering.
// This is used to check whether all CBB machines of the same OS have the same
// set of browsers installed. If there are any discrepencies, we can't start
// CBB run on those devices, and have to wait for browser updates to go through.
func compareBuildInfos(oldBis, newBis []BuildInfo) ([]BuildInfo, error) {
	if oldBis == nil {
		return newBis, nil
	}
	oldVersions := map[string]string{}
	for _, bi := range oldBis {
		oldVersions[bi.Channel] = bi.Version
	}
	for _, bi := range newBis {
		if oldVersions[bi.Channel] != bi.Version {
			return nil, fmt.Errorf(
				"browser version conflict: %s %s has both %s and %s",
				bi.Browser, bi.Channel, bi.Version, oldVersions[bi.Channel],
			)
		}
	}
	return oldBis, nil
}

func CbbGetBrowserVersionsWorkflow(ctx workflow.Context, browser string) ([]BuildInfo, error) {
	ctx = workflow.WithActivityOptions(ctx, releaseDetectorActivityOptions)
	ctx = workflow.WithChildOptions(ctx, getBrowerVersionsWorkflowOptions)

	var platform string
	var configCounts map[string]int32
	switch browser {
	case "safari":
		platform = "mac"
		configCounts = safariConfigs
	case "edge":
		platform = "windows"
		configCounts = edgeConfigs
	default:
		return nil, fmt.Errorf("unsupported browser %s", browser)
	}
	var results []BuildInfo
	for config, count := range configCounts {
		p := &SingleCommitRunnerParams{
			PinpointJobID:  fmt.Sprintf("Get %s version on %s", browser, config),
			BotConfig:      config,
			Benchmark:      browserVersionsBenchmark,
			Story:          "default",
			CombinedCommit: common.NewCombinedCommit(nil),
			Iterations:     count,
		}
		var cr *CommitRun
		if err := workflow.ExecuteChildWorkflow(ctx, workflows.SingleCommitRunner, p).Get(ctx, &cr); err != nil {
			return nil, skerr.Wrapf(err, "Error running browser_versions on %s", config)
		}
		var bi []BuildInfo
		if err := workflow.ExecuteActivity(ctx, CollectBrowserVersionsActivity, browser, platform, cr).Get(ctx, &bi); err != nil {
			return nil, skerr.Wrap(err)
		}
		var err error
		results, err = compareBuildInfos(results, bi)
		if err != nil {
			return nil, err
		}
	}
	return results, nil
}

func CollectBrowserVersionsActivity(ctx context.Context, browser string, platform string, cr *CommitRun) ([]BuildInfo, error) {
	var results []BuildInfo
	for _, run := range cr.Runs {
		clients, err := backends.DialRBECAS(ctx)
		if err != nil {
			sklog.Errorf("Failed to dial RBE CAS client due to error: %v", err)
			return nil, err
		}
		cl, ok := clients[run.CAS.CasInstance]
		if !ok {
			return nil, fmt.Errorf("swarming instance %s is not within the set of allowed instances", run.CAS.CasInstance)
		}

		d, err := digest.NewFromString(fmt.Sprintf("%s/%d", run.CAS.Digest.Hash, run.CAS.Digest.SizeBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to parse digest %v: %v", run.CAS.Digest, err)
		}

		rootDir := &repb.Directory{}
		if _, err := cl.ReadProto(ctx, d, rootDir); err != nil {
			return nil, fmt.Errorf("failed to read root directory proto: %v", err)
		}

		var versionFileDigest *repb.Digest
		for _, file := range rootDir.Files {
			if file.Name == browserVersionsFilename {
				versionFileDigest = file.Digest
			}
		}
		if versionFileDigest == nil {
			return nil, fmt.Errorf("missing browser_versions.json")
		}
		d, err = digest.NewFromString(fmt.Sprintf("%s/%d", versionFileDigest.Hash, versionFileDigest.SizeBytes))
		if err != nil {
			return nil, fmt.Errorf("failed to parse digest %v: %v", run.CAS.Digest, err)
		}
		res, _, err := cl.ReadBlob(ctx, d)
		if err != nil {
			return nil, skerr.Wrapf(err, "could not fetch results from CAS (%v)", run.CAS.Digest)
		}
		var channelVersion map[string]string
		err = json.Unmarshal(res, &channelVersion)
		if err != nil {
			return nil, skerr.Wrapf(err, "Unable to parse browser_versions.json")
		}
		sklog.Info("Installed browser versions: %v", channelVersion)

		var bi []BuildInfo
		for channel, version := range channelVersion {
			bi = append(bi, BuildInfo{
				Browser:  browser,
				Channel:  channel,
				Platform: platform,
				Version:  version,
			})
		}

		results, err = compareBuildInfos(results, bi)
		if err != nil {
			return nil, err
		}
	}

	return results, nil
}
