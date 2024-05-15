package internal

import (
	"context"
	"errors"
	"fmt"
	"time"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/build_chrome"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

// BuildChromeActivity wraps BuildChrome in Activities.
type BuildChromeActivity struct {
}

// BuildChrome is a Workflow definition that builds Chrome.
func BuildChrome(ctx workflow.Context, params workflows.BuildChromeParams) (*workflows.Build, error) {
	ctx = workflow.WithActivityOptions(ctx, buildActivityOption)
	logger := workflow.GetLogger(ctx)

	bca := &BuildChromeActivity{}
	var buildID int64
	var status buildbucketpb.Status
	defer func() {
		// ErrCanceled is the error returned by Context.Err when the context is canceled
		// This logic ensures cleanup only happens if there is a Cancellation error
		if !errors.Is(ctx.Err(), workflow.ErrCanceled) {
			return
		}
		// For the Workflow to execute an Activity after it receives a Cancellation Request
		// It has to get a new disconnected context
		newCtx, _ := workflow.NewDisconnectedContext(ctx)

		err := workflow.ExecuteActivity(newCtx, bca.CleanupBuildActivity, buildID, status).Get(ctx, nil)
		if err != nil {
			logger.Error("CleanupBuildActivity failed", err)
		}
	}()

	if err := workflow.ExecuteActivity(ctx, bca.SearchOrBuildActivity, params).Get(ctx, &buildID); err != nil {
		logger.Error("Failed to wait for SearchOrBuildActivity:", err)
		return nil, err
	}

	if err := workflow.ExecuteActivity(ctx, bca.WaitBuildCompletionActivity, buildID).Get(ctx, &status); err != nil {
		logger.Error("Failed to wait for WaitBuildCompletionActivity:", err)
		return nil, err
	}

	if status != buildbucketpb.Status_SUCCESS {
		return &workflows.Build{
			BuildChromeParams: params,
			ID:                buildID,
			Status:            status,
			CAS:               nil,
		}, nil
	}

	var cas *apipb.CASReference
	if err := workflow.ExecuteActivity(ctx, bca.RetrieveCASActivity, buildID, params.Target).Get(ctx, &cas); err != nil {
		logger.Error("Failed to wait for RetrieveCASActivity:", err)
		return nil, err
	}

	return &workflows.Build{
		BuildChromeParams: params,
		ID:                buildID,
		Status:            status,
		CAS:               cas,
	}, nil
}

// SearchOrBuildActivity wraps BuildChromeClient.SearchOrBuild
func (bca *BuildChromeActivity) SearchOrBuildActivity(ctx context.Context, params workflows.BuildChromeParams) (int64, error) {
	logger := activity.GetLogger(ctx)

	bc, err := build_chrome.New(ctx)
	if err != nil {
		logger.Error("Failed to new build_chrome:", err)
		return 0, err
	}

	activity.RecordHeartbeat(ctx, "kicking off the build.")
	buildID, err := bc.SearchOrBuild(ctx, params.WorkflowID, params.Commit.GetMainGitHash(), params.Device, params.Commit.DepsToMap(), params.Patch)
	if err != nil {
		logger.Error("Failed to build chrome:", err)
		return 0, err
	}
	return buildID, nil
}

// WaitBuildCompletionActivity wraps BuildChromeClient.GetStatus and waits until it is completed or errors.
func (bca *BuildChromeActivity) WaitBuildCompletionActivity(ctx context.Context, buildID int64) (buildbucketpb.Status, error) {
	logger := activity.GetLogger(ctx)

	bc, err := build_chrome.New(ctx)
	if err != nil {
		logger.Error("Failed to new build_chrome:", err)
		return buildbucketpb.Status_STATUS_UNSPECIFIED, err
	}
	failureRetries := 10
	for {
		select {
		case <-ctx.Done():
			return buildbucketpb.Status_STATUS_UNSPECIFIED, ctx.Err()
		default:
			status, err := bc.GetStatus(ctx, buildID)
			if err != nil {
				logger.Error("Failed to get build status:", err, "remaining retries:", failureRetries)
				failureRetries -= 1
				if failureRetries <= 0 {
					return buildbucketpb.Status_STATUS_UNSPECIFIED, skerr.Wrapf(err, "Failed to wait for build to complete")
				}
			}
			if status&buildbucketpb.Status_ENDED_MASK == buildbucketpb.Status_ENDED_MASK {
				return status, nil
			}
		}
		time.Sleep(5 * time.Second)
		activity.RecordHeartbeat(ctx, fmt.Sprintf("waiting on build to complete: %v", buildID))
	}
}

// RetrieveCASActivity wraps BuildChromeClient.RetrieveCAS and gets build artifacts in CAS.
func (bca *BuildChromeActivity) RetrieveCASActivity(ctx context.Context, buildID int64, target string) (*apipb.CASReference, error) {
	logger := activity.GetLogger(ctx)

	bc, err := build_chrome.New(ctx)
	if err != nil {
		logger.Error("Failed to new build_chrome:", err)
		return nil, err
	}

	activity.RecordHeartbeat(ctx, fmt.Sprintf("start retrieving CAS for: (%v, %v)", buildID, target))
	cas, err := bc.RetrieveCAS(ctx, buildID, target)
	if err != nil {
		logger.Error("Failed to retrieve CAS:", err)
		return nil, err
	}
	return cas, nil
}

// CleanupBuildActivity wraps BuildChromeClient.CancelBuild
func (bca *BuildChromeActivity) CleanupBuildActivity(ctx context.Context, buildID int64, status buildbucketpb.Status) error {
	if buildID == 0 || !(status == buildbucketpb.Status_SCHEDULED || status == buildbucketpb.Status_STARTED) {
		return nil
	}

	logger := activity.GetLogger(ctx)
	bc, err := build_chrome.New(ctx)
	if err != nil {
		logger.Error("Failed to new build_chrome:", err)
		return err
	}

	activity.RecordHeartbeat(ctx, "cancelling the build.")
	err = bc.CancelBuild(ctx, buildID, "Pinpoint job cancelled")
	if err != nil {
		logger.Error("Failed to cancel build:", err)
		return err
	}
	return nil
}
