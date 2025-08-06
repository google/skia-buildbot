package internal

import (
	"log"
	"strings"
	"time"

	"go.skia.org/infra/go/skerr"
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
}

// CbbNewReleaseDetectorWorkflow is the most basic Workflow Defintion.
func CbbNewReleaseDetectorWorkflow(ctx workflow.Context) (*ChromeReleaseInfo, error) {
	ctx = workflow.WithActivityOptions(ctx, releaseDetectorActivityOptions)

	// Is this workflow running locally on a dev machine?
	isDev := strings.HasPrefix(workflow.GetActivityOptions(ctx).TaskQueue, "localhost.")

	var commitInfo ChromeReleaseInfo
	if err := workflow.ExecuteActivity(ctx, GetChromeReleasesInfoActivity, isDev).Get(ctx, &commitInfo); err != nil {
		return nil, skerr.Wrap(err)
	}
	log.Printf("commitInfo:%v", commitInfo)
	workflowResult := &ChromeReleaseInfo{
		CommitPosition: commitInfo.CommitPosition,
		CommitHash:     commitInfo.CommitHash,
	}
	return workflowResult, nil
}
