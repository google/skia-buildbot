package internal

import (
	"context"
	"fmt"
	"time"

	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/backends"
	"go.skia.org/infra/pinpoint/go/run_benchmark"
	"go.skia.org/infra/pinpoint/go/workflows"
	ppb "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/temporal"
	"go.temporal.io/sdk/workflow"
)

// RunBenchmarkActivity wraps RunBenchmarkWorkflow in Activities
type RunBenchmarkActivity struct {
}

var runBenchmarkActivityOption = workflow.ActivityOptions{
	ScheduleToCloseTimeout: 24 * time.Hour, // swarming tasks can be pending for a long time
	// The default gRPC timeout is 5 minutes, longer than that so it can capture grpc errors.
	HeartbeatTimeout: 6 * time.Minute,
	RetryPolicy: &temporal.RetryPolicy{
		InitialInterval:    15 * time.Second,
		BackoffCoefficient: 2.0,
		MaximumInterval:    1 * time.Minute,
		MaximumAttempts:    3,
	},
}

// RunBenchmarkWorkflow is a Workflow definition that schedules a single task,
// polls and retrieves the CAS for the RunBenchmarkParams defined.
func RunBenchmarkWorkflow(ctx workflow.Context, params workflows.RunBenchmarkParams) (*workflows.TestRun, error) {
	ctx = workflow.WithActivityOptions(ctx, runBenchmarkActivityOption)
	logger := workflow.GetLogger(ctx)

	var rba RunBenchmarkActivity
	var taskID string
	if err := workflow.ExecuteActivity(ctx, rba.ScheduleTaskActivity, params).Get(ctx, &taskID); err != nil {
		logger.Error("Failed to schedule task:", err)
		return nil, skerr.Wrap(err)
	}

	var state string
	if err := workflow.ExecuteActivity(ctx, rba.WaitTaskFinishedActivity, taskID).Get(ctx, &state); err != nil {
		logger.Error("Failed to poll task ID:", err)
		return nil, skerr.Wrap(err)
	}

	success := run_benchmark.IsTaskStateSuccess(state)

	resp := workflows.TestRun{
		TaskID: taskID,
		Status: state,
	}

	var cas *swarmingV1.SwarmingRpcsCASReference
	if !success {
		return &resp, nil
	}

	if err := workflow.ExecuteActivity(ctx, rba.RetrieveCASActivity, taskID).Get(ctx, &cas); err != nil {
		logger.Error("Failed to retrieve CAS reference:", err)
		return nil, skerr.Wrap(err)
	}

	resp.CAS = cas
	return &resp, nil
}

// ScheduleTaskActivity wraps BuildChromeClient.SearchOrBuild
func (rba *RunBenchmarkActivity) ScheduleTaskActivity(ctx context.Context, params workflows.RunBenchmarkParams) (string, error) {
	logger := activity.GetLogger(ctx)

	sc, err := backends.NewSwarmingClient(ctx, backends.DefaultSwarmingServiceAddress)
	if err != nil {
		logger.Error("Failed to connect to swarming client:", err)
		return "", skerr.Wrap(err)
	}

	// TODO(jeffyoon@) - this is a workaround to generate the request object s.t. the refactor
	// does not obstruct the current run_benchmark workflow. Once the request spec defined
	// by the proto is in full use, this can be deprecated.
	//
	// This only fills in the minimum set required to run the benchmark.
	req := &ppb.ScheduleBisectRequest{
		// run_benchmark.RunBenchmarkRequest doesn't support story tags
		Configuration: params.Request.Config.Bot,
		Benchmark:     params.Request.Benchmark,
		Story:         params.Request.Story,
	}

	taskIds, err := run_benchmark.Run(ctx, sc, req, params.Request.Commit, params.Request.JobID, params.Request.Build, 1)
	if err != nil {
		return "", err
	}
	return taskIds[0].TaskId, nil
}

// WaitTaskFinishedActivity polls the task until it finishes or errors. Returns the status
// if the task finishes regardless of task success
func (rba *RunBenchmarkActivity) WaitTaskFinishedActivity(ctx context.Context, taskID string) (string, error) {
	logger := activity.GetLogger(ctx)

	sc, err := backends.NewSwarmingClient(ctx, backends.DefaultSwarmingServiceAddress)
	if err != nil {
		logger.Error("Failed to connect to swarming client:", err)
		return "", skerr.Wrap(err)
	}

	activity.RecordHeartbeat(ctx, "begin run_benchmark task polling")
	failureRetries := 5
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
			state, err := sc.GetStatus(ctx, taskID)
			if err != nil {
				logger.Error("Failed to get task status:", err, "remaining retries:", failureRetries)
				failureRetries -= 1
				if failureRetries <= 0 {
					return "", skerr.Wrapf(err, "Failed to wait for task to complete")
				}
			}
			fin, err := run_benchmark.IsTaskStateFinished(state)
			if err != nil {
				return "", skerr.Wrapf(err, "Failed to check task state")
			}
			if fin {
				return state, nil
			}
			time.Sleep(15 * time.Second)
			activity.RecordHeartbeat(ctx, fmt.Sprintf("waiting on test %v with state %s", taskID, state))
		}
	}
}

// RetrieveCASActivity wraps retrieves task artifacts from CAS
func (rba *RunBenchmarkActivity) RetrieveCASActivity(ctx context.Context, taskID string) (*swarmingV1.SwarmingRpcsCASReference, error) {
	logger := activity.GetLogger(ctx)

	sc, err := backends.NewSwarmingClient(ctx, backends.DefaultSwarmingServiceAddress)
	if err != nil {
		logger.Error("Failed to connect to swarming client:", err)
		return nil, skerr.Wrap(err)
	}

	cas, err := sc.GetCASOutput(ctx, taskID)
	if err != nil {
		logger.Error("Failed to retrieve CAS:", err)
		return nil, err
	}

	return cas, nil
}
