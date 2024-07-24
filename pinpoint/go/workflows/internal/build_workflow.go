package internal

import (
	"context"
	"errors"
	"fmt"
	"time"

	buildbucketpb "go.chromium.org/luci/buildbucket/proto"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/clients/build"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

// BuildWorkflow is a Workflow definition that creates a build artifact.
func BuildWorkflow(ctx workflow.Context, params workflows.BuildParams) (*workflows.Build, error) {
	ctx = workflow.WithActivityOptions(ctx, buildActivityOption)
	logger := workflow.GetLogger(ctx)

	project := params.Project
	if params.Project == "" {
		project = "chromium"
	}

	buildClient, err := build.NewBuildClient(context.Background(), project)
	if err != nil {
		logger.Error("Failed to instantiate build client: ", err)
		return nil, err
	}
	ctx = workflow.WithValue(ctx, build.BuildClientContextKey, buildClient)

	bca := &BuildActivity{}
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

		err := workflow.ExecuteActivity(newCtx, bca.CleanupBuildActivity, buildID, status, project).Get(ctx, nil)
		if err != nil {
			logger.Error("CleanupBuildActivity failed", err)
		}
	}()

	if err := workflow.ExecuteActivity(ctx, bca.SearchOrBuildActivity, params).Get(ctx, &buildID); err != nil {
		logger.Error("Failed to wait for SearchOrBuildActivity:", err)
		return nil, err
	}

	if err := workflow.ExecuteActivity(ctx, bca.WaitBuildCompletionActivity, buildID, project).Get(ctx, &status); err != nil {
		logger.Error("Failed to wait for WaitBuildCompletionActivity:", err)
		return nil, err
	}

	if status != buildbucketpb.Status_SUCCESS {
		return &workflows.Build{
			BuildParams: params,
			ID:          buildID,
			Status:      status,
			CAS:         nil,
		}, nil
	}

	var cas *apipb.CASReference
	if err := workflow.ExecuteActivity(ctx, bca.RetrieveBuildArtifactActivity, buildID, params.Target, params.Project).Get(ctx, &cas); err != nil {
		logger.Error("Failed to wait for RetrieveBuildArtifactActivity:", err)
		return nil, err
	}

	return &workflows.Build{
		BuildParams: params,
		ID:          buildID,
		Status:      status,
		CAS:         cas,
	}, nil
}

// BuildActivity wraps Build Activities.
type BuildActivity struct{}

// SearchOrBuildActivity searches for an existing build to reuse, or triggers a new one.
func (bca *BuildActivity) SearchOrBuildActivity(ctx context.Context, params workflows.BuildParams) (int64, error) {
	logger := activity.GetLogger(ctx)

	buildClient, err := build.NewBuildClient(ctx, "chrome")
	if err != nil {
		logger.Error("Failed to instantiate build client: ", err)
		return 0, skerr.Wrapf(err, "failed to instantiate a build client")
	}

	activity.RecordHeartbeat(ctx, "kicking off the build.")
	findReq, err := buildClient.CreateFindBuildRequest(params)
	if err != nil {
		logger.Error("Failed to create find build request: ", err)
		return -1, skerr.Wrapf(err, "failed to create find build request")
	}

	logger.Debug("Request for finding a build: ", findReq)
	findResp, err := buildClient.FindBuild(ctx, findReq)
	if err != nil {
		logger.Error("Failed to search for an existing build: ", err)
		return -1, skerr.Wrapf(err, "failed to search for an exsting build")
	}

	if findResp.BuildID != 0 {
		return findResp.BuildID, nil
	}

	// Not found, trigger a new build
	buildReq, err := buildClient.CreateStartBuildRequest(params)
	if err != nil {
		logger.Error("Failed to generate build request: ", err)
		return -1, skerr.Wrapf(err, "failed to generate build request")
	}

	logger.Debug("Request for a new build: ", buildReq)
	buildResp, err := buildClient.StartBuild(ctx, buildReq)
	if err != nil {
		logger.Error("Failed to start new build: ", err)
		return -1, skerr.Wrapf(err, "failed to start new build")
	}

	logger.Info("New build started. Creation response: ", buildResp.Response.(*buildbucketpb.Build))
	return buildResp.Response.(*buildbucketpb.Build).Id, nil
}

// WaitBuildCompletionActivity polls the build and waits until it is completed or errors.
func (bca *BuildActivity) WaitBuildCompletionActivity(ctx context.Context, buildID int64, project string) (buildbucketpb.Status, error) {
	logger := activity.GetLogger(ctx)

	buildClient, ok := ctx.Value(build.BuildClientContextKey).(build.BuildClient)
	if !ok {
		newBuildClient, err := build.NewBuildClient(ctx, project)
		if err != nil {
			logger.Error("Failed to instantiate build client missing from context: ", err)
			return buildbucketpb.Status_STATUS_UNSPECIFIED, err
		}
		buildClient = newBuildClient
	}

	failureRetries := 10
	for {
		select {
		case <-ctx.Done():
			return buildbucketpb.Status_STATUS_UNSPECIFIED, ctx.Err()
		default:
			status, err := buildClient.GetStatus(ctx, buildID)
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

// RetrieveBuildArtifactActivity gets build artifacts.
func (bca *BuildActivity) RetrieveBuildArtifactActivity(ctx context.Context, buildID int64, target, project string) (*apipb.CASReference, error) {
	logger := activity.GetLogger(ctx)

	buildClient, ok := ctx.Value(build.BuildClientContextKey).(build.BuildClient)
	if !ok {
		newBuildClient, err := build.NewBuildClient(ctx, project)
		if err != nil {
			logger.Error("Failed to instantiate build client missing from context: ", err)
			return nil, err
		}
		buildClient = newBuildClient
	}

	activity.RecordHeartbeat(ctx, fmt.Sprintf("start retrieving CAS for: (%v, %v)", buildID, target))

	getArtifactReq := &build.GetBuildArtifactRequest{
		BuildID: buildID,
		Target:  target,
	}
	resp, err := buildClient.GetBuildArtifact(ctx, getArtifactReq)
	if err != nil {
		logger.Error("Failed to fetch build artifacts: ", err)
		return nil, err
	}

	return resp.Response.(*apipb.CASReference), nil
}

// CleanupBuildActivity wraps BuildChromeClient.CancelBuild
func (bca *BuildActivity) CleanupBuildActivity(ctx context.Context, buildID int64, status buildbucketpb.Status, project string) error {
	if buildID == 0 || !(status == buildbucketpb.Status_SCHEDULED || status == buildbucketpb.Status_STARTED) {
		return nil
	}

	logger := activity.GetLogger(ctx)

	buildClient, ok := ctx.Value(build.BuildClientContextKey).(build.BuildClient)
	if !ok {
		newBuildClient, err := build.NewBuildClient(ctx, project)
		if err != nil {
			logger.Error("Failed to instantiate build client missing from context: ", err)
			return err
		}
		buildClient = newBuildClient
	}

	activity.RecordHeartbeat(ctx, "cancelling the build.")

	cancelReq := &build.CancelBuildRequest{
		BuildID: buildID,
		Reason:  "Pinpoint job cancelled",
	}
	return buildClient.CancelBuild(ctx, cancelReq)
}
