package internal

import (
	"context"
	"fmt"
	"time"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/bisection/go/build_chrome"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// BuildChromeActivity wraps BuildChrome in Activities.
type BuildChromeActivity struct {
}

// BuildChrome is a Workflow definition that builds Chrome.
func BuildChrome(ctx workflow.Context, params workflows.BuildChromeParams) (*swarmingV1.SwarmingRpcsCASReference, error) {
	ao := workflow.ActivityOptions{
		// Expect longer running time
		StartToCloseTimeout: 6 * time.Hour,
		// The default gRPC timeout is 5 minutes, longer than that so it can capture grpc errors.
		HeartbeatTimeout: 6 * time.Minute,
		RetryPolicy: &temporal.RetryPolicy{
			InitialInterval:    15 * time.Second,
			BackoffCoefficient: 2.0,
			MaximumInterval:    1 * time.Minute,
			MaximumAttempts:    3,
		},
	}

	ctx = workflow.WithActivityOptions(ctx, ao)
	logger := workflow.GetLogger(ctx)

	bca := &BuildChromeActivity{}
	var buildID int
	if err := workflow.ExecuteActivity(ctx, bca.SearchOrBuildActivity, params).Get(ctx, &buildID); err != nil {
		logger.Error("Failed to wait for SearchOrBuildActivity:", err)
		return nil, err
	}

	var completed bool
	if err := workflow.ExecuteActivity(ctx, bca.WaitBuildCompletionActivity, buildID).Get(ctx, &completed); err != nil {
		logger.Error("Failed to wait for WaitBuildCompletionActivity:", err)
		return nil, err
	}

	var cas *swarmingV1.SwarmingRpcsCASReference
	if err := workflow.ExecuteActivity(ctx, bca.RetrieveCASActivity, buildID, params.Target).Get(ctx, &cas); err != nil {
		logger.Error("Failed to wait for RetrieveCASActivity:", err)
		return nil, err
	}
	return cas, nil
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
	buildID, err := bc.SearchOrBuild(ctx, params.PinpointJobID, params.Commit, params.Device, params.Target, params.Deps, params.Patch)
	if err != nil {
		logger.Error("Failed to build chrome:", err)
		return 0, err
	}
	return buildID, nil
}

// WaitBuildCompletionActivity wraps BuildChromeClient.GetStatus and waits until it is completed or errors.
func (bca *BuildChromeActivity) WaitBuildCompletionActivity(ctx context.Context, buildID int64) (bool, error) {
	logger := activity.GetLogger(ctx)

	bc, err := build_chrome.New(ctx)
	if err != nil {
		logger.Error("Failed to new build_chrome:", err)
		return false, err
	}
	failureRetries := 10
	for {
		select {
		case <-ctx.Done():
			return false, ctx.Err()
		default:
			status, err := bc.GetStatus(ctx, buildID)
			if err != nil {
				logger.Error("Failed to get build status:", err, "remaining retries:", failureRetries)
				failureRetries -= 1
				if failureRetries <= 0 {
					return false, skerr.Wrapf(err, "Failed to wait for build to complete")
				}
			}
			if status == buildbucketpb.Status_SUCCESS {
				return true, nil
			}
		}
		time.Sleep(5 * time.Second)
		activity.RecordHeartbeat(ctx, fmt.Sprintf("waiting on build to complete: %v", buildID))
	}
}

// RetrieveCASActivity wraps BuildChromeClient.RetrieveCAS and gets build artifacts in CAS.
func (bca *BuildChromeActivity) RetrieveCASActivity(ctx context.Context, buildID int64, target string) (*swarmingV1.SwarmingRpcsCASReference, error) {
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
