package internal

import (
	"context"
	"fmt"
	"time"

	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/backends"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/run_benchmark"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

// RunBenchmarkParams are the Temporal Workflow params
// for the RunBenchmarkWorkflow.
type RunBenchmarkParams struct {
	// the Pinpoint job id
	JobID string
	// the swarming instance and cas digest hash and bytes location for the build
	BuildCAS *swarmingV1.SwarmingRpcsCASReference
	// commit hash
	Commit *midpoint.CombinedCommit
	// device configuration
	BotConfig string
	// benchmark to test
	Benchmark string
	// story to test
	Story string
	// story tags for the test
	StoryTags string
}

// RunBenchmarkActivity wraps RunBenchmarkWorkflow in Activities
type RunBenchmarkActivity struct {
}

// RunBenchmarkWorkflow is a Workflow definition that schedules a single task,
// polls and retrieves the CAS for the RunBenchmarkParams defined.
func RunBenchmarkWorkflow(ctx workflow.Context, p *RunBenchmarkParams) (*workflows.TestRun, error) {
	ctx = workflow.WithActivityOptions(ctx, runBenchmarkActivityOption)
	logger := workflow.GetLogger(ctx)

	var rba RunBenchmarkActivity
	var taskID string
	if err := workflow.ExecuteActivity(ctx, rba.ScheduleTaskActivity, p).Get(ctx, &taskID); err != nil {
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

	if err := workflow.ExecuteActivity(ctx, rba.RetrieveTestCASActivity, taskID).Get(ctx, &cas); err != nil {
		logger.Error("Failed to retrieve CAS reference:", err)
		return nil, skerr.Wrap(err)
	}

	resp.CAS = cas
	return &resp, nil
}

// ScheduleTaskActivity wraps BuildChromeClient.SearchOrBuild
func (rba *RunBenchmarkActivity) ScheduleTaskActivity(ctx context.Context, rbp *RunBenchmarkParams) (string, error) {
	logger := activity.GetLogger(ctx)

	sc, err := backends.NewSwarmingClient(ctx, backends.DefaultSwarmingServiceAddress)
	if err != nil {
		logger.Error("Failed to connect to swarming client:", err)
		return "", skerr.Wrap(err)
	}

	taskIds, err := run_benchmark.Run(ctx, sc, rbp.Commit.GetMainGitHash(), rbp.BotConfig, rbp.Benchmark, rbp.Story, rbp.StoryTags, rbp.JobID, rbp.BuildCAS, 1)
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

// RetrieveTestCASActivity wraps retrieves task artifacts from CAS
func (rba *RunBenchmarkActivity) RetrieveTestCASActivity(ctx context.Context, taskID string) (*swarmingV1.SwarmingRpcsCASReference, error) {
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
