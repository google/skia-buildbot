package internal

import (
	"log"
	"strconv"
	"strings"
	"time"

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
		"win-arm64-snapdragon-plus-perf-cbb",
		"win-arm64-snapdragon-elite-perf-cbb",
	},
	"android": {"android-pixel-tangor-perf-cbb"},
}

// CbbNewReleaseDetectorWorkflow is the most basic Workflow Defintion.
func CbbNewReleaseDetectorWorkflow(ctx workflow.Context) (*ChromeReleaseInfo, error) {
	ctx = workflow.WithActivityOptions(ctx, releaseDetectorActivityOptions)
	ctx = workflow.WithChildOptions(ctx, runBenchmarkWorkflowOptions)

	// Is this workflow running locally on a dev machine?
	isDev := strings.HasPrefix(workflow.GetActivityOptions(ctx).TaskQueue, "localhost.")

	var commitInfo ChromeReleaseInfo
	if err := workflow.ExecuteActivity(ctx, GetChromeReleasesInfoActivity, isDev).Get(ctx, &commitInfo); err != nil {
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
						Benchmarks: nil, // nil means run the standard set of benchmarks
						Bucket:     "chrome-perf-non-public",
					}
					var cr *CommitRun
					if err := workflow.ExecuteChildWorkflow(ctx, workflows.CbbRunner, p).Get(gCtx, &cr); err != nil {
						sklog.Errorf("Error in CBB runner %#v: %v", p, err)
					}
				})
			}
		}

		wg.Wait(ctx)
	}

	return &commitInfo, nil
}
