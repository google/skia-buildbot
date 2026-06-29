// This file defines CBB New Release Detector workflow, which is the top-level
// workflow used by CBB. This workflow is intended to be scheduled to run
// regularly on the perf infra Temporal server. Currently, the schdule runs
// every weekday at 22:00 UTC time (2 pm PST or 3 pm PDT on US west coast).
//
// To create the schedule using Temporal UI: Go to
// https://skia-temporal-ui.corp.goog/namespaces/perf-internal/schedules/create
// and enter the following information:
// * Name: CBB Schedule
// * Workflow Type: perf.cbb_new_release_detector
// * Workflow Id: perf.cbb_new_release_detector
// * Task Queue: perf.perf-chrome-public.bisect
// * Schedule Spec: Click on "Days of the Week" tab, and then select Weekdays
// * Time: Enter 22 hrs, 00 min
// And then click Create Schedule button.
//
// Alternatively, you can create the schedule using Temporal CLI, if you have
// Temporal CLI installed locally. You need to follow steps 1 through 3 at
// https://skia.googlesource.com/buildbot/+/refs/heads/main/temporal/README.md#locally-trigger-production-workflow-follow-these-steps-with-care
// and then run the following command:
//	temporal schedule create --schedule-id 'CBB Schedule' --workflow-type perf.cbb_new_release_detector --workflow-id perf.cbb_new_release_detector --cron '0 22 * * 1-5' --task-queue perf.perf-chrome-public.bisect --namespace perf-internal
//
// Regardless of how you created it, you can view the schedule at
// https://skia-temporal-ui.corp.goog/namespaces/perf-internal/schedules,
// or run the Temporal CLI command `temporal schedule list`.

package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/bazelbuild/remote-apis-sdks/go/pkg/digest"
	repb "github.com/bazelbuild/remote-apis/build/bazel/remote/execution/v2"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"

	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/sklog"
	"go.skia.org/infra/pinpoint/go/backends"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/workflows"
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
	"mac": {
		"mac-m3-pro-perf-cbb",
		"mac-m4-mini-perf-cbb",
	},
	"windows": {
		"win-victus-perf-cbb",
		"win-arm64-snapdragon-elite-perf-cbb",
	},
	"android": {
		"android-pixel-tangor-perf-cbb",
		"android-pixel10-perf-cbb",
	},
}

const (
	// Pseudo benchmark and filename to retrieve Edge and Safari versions from devices.
	// Must match the strings used in
	// https://source.chromium.org/chromium/chromium/src/+/main:testing/scripts/run_performance_tests.py.
	browserVersionsBenchmark = "browser_versions"
	browserVersionsFilename  = "browser_versions.json"

	// Email address to send notification when a CBB job fails.
	errorNotificationEmail = "chrome-cbb@google.com"
)

// All Mac bot config names, with the number of devices in each config.
var safariConfigs = map[string]int32{
	"mac-m3-pro-perf-cbb":  5,
	"mac-m4-mini-perf-cbb": 5,
}

// All Windows bot config names, with the number of devices in each config.
var edgeConfigs = map[string]int32{
	"win-victus-perf-cbb":                 4,
	"win-arm64-snapdragon-elite-perf-cbb": 4,
}

// CbbNewReleaseDetectorWorkflow is the most basic Workflow Definition.
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
		commit := common.NewCombinedCommit(common.NewChromiumCommit(commitInfo.CommitHash))
		cp, err := strconv.ParseInt(commitInfo.CommitPosition, 10, 32)
		if err != nil {
			return nil, skerr.Wrapf(err, "Invalid commit position %s", commitInfo.CommitPosition)
		}
		commit.Main.CommitPosition = int32(cp)
		wg := workflow.NewWaitGroup(ctx)

		// Runs can be triggered in parallel on different platforms (Mac, Windows, etc),
		// and also on different bot configs in the same platform (e.g., Intel Windows
		// and ARM Windows), but runs on the same bot config should not be triggered in
		// parallel, to avoid unnecessary comptition of bot resources. To make this
		// possible, we first group the builds based on their platforms.
		platformBuilds := make(map[string][]BuildInfo)
		for _, build := range commitInfo.Builds {
			platformBuilds[build.Platform] = append(platformBuilds[build.Platform], build)
		}
		for platform, builds := range platformBuilds {
			for _, bot := range platformBots[platform] {
				wg.Add(1)
				// Starting of the parallelism boundary. We kick off the following code
				// block in parallel on all bot configs, but the logic inside the code
				// block runs sequentially on a particular bot config.
				workflow.Go(ctx, func(ctx workflow.Context) {
					defer wg.Done()
					for _, build := range builds {
						p := &CbbRunnerParams{
							BotConfig:  bot,
							Commit:     commit,
							Browser:    build.Browser,
							Channel:    build.Channel,
							SkipFinch:  false,
							Benchmarks: nil, // nil means run the standard set of benchmarks
							Bucket:     bucket,
						}
						workflowID := fmt.Sprintf(
							"cbb_runner-%s-%s-%s",
							strings.ReplaceAll(getShortBrowserName(build.Browser, build.Channel), " ", "-"),
							getShortBrowserVersion(build.Version, build.Browser, build.Channel),
							getShortBotName(bot))
						callCbbRunner(ctx, p, workflowID)

						if build.Browser == "chrome" && build.Platform != "android" {
							// With Chrome on desktop, re-run all benchmarks with Finch control disabled.
							p.SkipFinch = true
							workflowID += "-no-finch"
							callCbbRunner(ctx, p, workflowID)
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
			PinpointJobID:  fmt.Sprintf("CBB get %s version on %s", browser, getShortBotName(config)),
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

func CollectBrowserVersionsActivity(ctx context.Context, browser, platform string, cr *CommitRun) ([]BuildInfo, error) {
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
			return nil, fmt.Errorf("failed to parse digest %v: %w", run.CAS.Digest, err)
		}

		rootDir := &repb.Directory{}
		if _, err := cl.ReadProto(ctx, d, rootDir); err != nil {
			return nil, fmt.Errorf("failed to read root directory proto: %w", err)
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
			return nil, fmt.Errorf("failed to parse digest %v: %w", run.CAS.Digest, err)
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

func callCbbRunner(ctx workflow.Context, p *CbbRunnerParams, workflowID string) {
	options := cbbRunnerWorkflowOptions
	options.WorkflowID = workflowID
	ctx = workflow.WithChildOptions(ctx, options)
	var cr *CommitRun
	if err := workflow.ExecuteChildWorkflow(ctx, workflows.CbbRunner, p).Get(ctx, &cr); err != nil {
		sklog.Errorf("Error in CBB runner %#v: %v", p, err)
		// Check if we have received a custom error object (CbbRunnerError) with additional details
		// of what went wrong. The custom error has been wrapped by Temporal, and needs to be unwrapped.
		var workflowErr *temporal.ChildWorkflowExecutionError
		if errors.As(err, &workflowErr) {
			err = workflowErr.Unwrap()
			var appErr *temporal.ApplicationError
			if errors.As(err, &appErr) && appErr.Type() == "CbbRuntimeError" {
				sklog.Error("Found a CbbRuntimeError inside ApplicationError, trying to retrieve it")
				var cbbErr CbbRunnerError
				err2 := appErr.Details(&cbbErr)
				if err2 == nil {
					err = &cbbErr
				} else {
					sklog.Errorf("Unable to retrieve CbbRunnerError: %v", err2)
				}
			}
		}
		sendCbbRunnerErrorEmail(ctx, options.WorkflowID, p, err)
	}
}

func sendCbbRunnerErrorEmail(ctx workflow.Context, id string, p *CbbRunnerParams, err error) {
	var cbbErr *CbbRunnerError
	hasCbbErr := errors.As(err, &cbbErr)

	subject := fmt.Sprintf("CBB runner \"%s\" failed", id)
	body := fmt.Sprintf("CBB runner \"%s\" failed:\n<ul>\n", html.EscapeString(id))
	if hasCbbErr {
		body += fmt.Sprintf("<li> Failed workflow details: <a href=\"%s\">%s</a></li>\n", cbbErr.WorkflowLink, cbbErr.WorkflowLink)
	}
	body += fmt.Sprintf(
		`<li> Bot: %s</li>
<li> Browser: %s</li>
<li> Channel: %s</li>
<li> Commit: %s</li>
<li> Skip Finch: %v</li>
<li> Error: %v</li>
</ul>`,
		html.EscapeString(p.BotConfig),
		html.EscapeString(p.Browser),
		html.EscapeString(p.Channel),
		html.EscapeString(p.Commit.GetMainGitHash()),
		p.SkipFinch,
		html.EscapeString(err.Error()))

	if hasCbbErr && len(cbbErr.SwarmingLinks) > 0 {
		if len(cbbErr.SwarmingLinks) == cbbErr.TotalBenchmarkCount {
			body += fmt.Sprintf("<p>All %d benchmarks have failed</p>\n", cbbErr.TotalBenchmarkCount)
		} else {
			body += fmt.Sprintf("<p>%d benchmarks have failed (out of a total of %d)</p>\n", len(cbbErr.SwarmingLinks), cbbErr.TotalBenchmarkCount)
		}

		body += "<p>Failed benchmarks:</p>\n<ul>\n"
		for b, l := range cbbErr.SwarmingLinks {
			body += fmt.Sprintf("<li> %s: <a href=\"%s\">swarming tasks</a> </li>\n", html.EscapeString(b), l)
		}
		body += "</ul>\n"

		isDev := strings.HasPrefix(workflow.GetActivityOptions(ctx).TaskQueue, "localhost.")
		var temporalHost, monitorLink string
		if isDev {
			temporalHost = "temporal-ui-dev.corp.goog"
			monitorUrl := fmt.Sprintf(
				"https://%s/namespaces/perf-internal/workflows?query=%%60WorkflowType%%60%%3D%%22perf.cbb_runner%%22",
				temporalHost)
			monitorLink = fmt.Sprintf("<a href=\"%s\">this page</a>", monitorUrl)
		} else {
			temporalHost = "skia-temporal-ui.corp.goog"
			monitorLink = "<a href=\"http://go/cbb-runner\">go/cbb-runner</a>"
		}

		inputJson, _ := json.MarshalIndent(p, "", "  ")

		body += fmt.Sprintf(`
<p> Please use the information provided above to investigate the failures.
After you have corrected any underlying issues, follow these steps to rerun the failed benchmarks: </p>
<ul>
  <li> Go to the <a href="https://%s/namespaces/perf-internal/schedules/create">Create Schedule</a> page
       on Temporal UI, and enter the following entries: </li>
  <ul>
    <li> <b>Name</b>: <code>%s</code> </li>
	<li> <b>Workflow Type</b>: <code>%s</code> </li>
	<li> <b>Workflow Id</b>: <code>%s</code> </li>
	<li> <b>Task Queue</b>: <code>perf.perf-chrome-public.bisect</code> </li>
  </ul>
  <li> Copy and paste the following jSON into the <b>Input</b> field:
<pre>
%s
</pre>
  </li>
  <li> Under <b>Schedule Spec</b>, select the date/time you want to rerun the benchmarks.
       It is recommended that you select the <b>Days of the Month</b> tab,
	   and then select either today or another day in the near future.
	   Select a convenient time to run the benchmarks, keeping in mind that you are
	   entering UTC time. Also keep in mind that CBB automatcailly runs at 22:00 UTC time
	   on each weekday, and it is best not to schedule your rerun too close to that time. </li>
  <li> Double check your entries, and then click the <b>Create Schedule</b>
</ul>
<p> After the scheduled time, go to %s to monitor the status of the rerun,
look for Workflow ID %s among the list. </p>
<p> After your scheduled rerun has finished, please delete your schedule by going to the
<a href="https://%s/namespaces/perf-internal/schedules/%s">schedule page</a>,
Click on the down arrow inside the Pause button at the upper-right corner of the page,
and then select the Delete command. Be sure you clicked on the right link,
and don't delete the wrong schedule.
`,
			temporalHost,
			html.EscapeString(id),
			workflows.CbbRunner,
			html.EscapeString(id),
			string(inputJson),
			monitorLink,
			html.EscapeString(id),
			temporalHost,
			html.EscapeString(id),
		)
	}

	ctx = workflow.WithActivityOptions(ctx, workflow.ActivityOptions{
		StartToCloseTimeout: 1 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			MaximumAttempts: 1,
		},
	})
	_ = workflow.ExecuteActivity(ctx, SendEmailActivity, []string{errorNotificationEmail}, subject, body).Get(ctx, nil)
}
