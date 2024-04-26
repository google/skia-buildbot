package internal

import (
	"context"
	"errors"
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
	// additional dimensions for bot selection
	Dimensions map[string]string
	// iteration for the benchmark run. A few workflows have multiple iterations of
	// benchmark runs and this param comes in handy to get additional info of a specific run.
	// This is for debugging/informational purposes only.
	IterationIdx int32
}

// RunBenchmarkActivity wraps RunBenchmarkWorkflow in Activities
type RunBenchmarkActivity struct {
}

// RunBenchmarkWorkflow is a Workflow definition that schedules a single task,
// polls and retrieves the CAS for the RunBenchmarkParams defined.
func RunBenchmarkWorkflow(ctx workflow.Context, p *RunBenchmarkParams) (*workflows.TestRun, error) {
	ctx = workflow.WithActivityOptions(ctx, runBenchmarkActivityOption)
	pendingCtx := workflow.WithActivityOptions(ctx, runBenchmarkPendingActivityOption)
	logger := workflow.GetLogger(ctx)

	var rba RunBenchmarkActivity
	var taskID string
	var state run_benchmark.State
	defer func() {
		// ErrCanceled is the error returned by Context.Err when the context is canceled
		// This logic ensures cleanup only happens if there is a Cancellation error
		if !errors.Is(ctx.Err(), workflow.ErrCanceled) {
			return
		}
		// For the Workflow to execute an Activity after it receives a Cancellation Request
		// It has to get a new disconnected context
		newCtx, _ := workflow.NewDisconnectedContext(ctx)

		err := workflow.ExecuteActivity(newCtx, rba.CleanupBenchmarkRunActivity, taskID, state).Get(ctx, nil)
		if err != nil {
			logger.Error("CleanupBenchmarkRunActivity failed", err)
		}
	}()

	if err := workflow.ExecuteActivity(ctx, rba.ScheduleTaskActivity, p).Get(ctx, &taskID); err != nil {
		logger.Error("Failed to schedule task:", err)
		return nil, skerr.Wrap(err)
	}

	// polling pending and polling running are two different activities
	// because swarming tasks can be pending for hours while swarming tasks
	// generally finish in ~10 min
	// TODO(sunxiaodi@): handle NO_RESOURCE retry case in WaitTaskPendingActivity
	if err := workflow.ExecuteActivity(pendingCtx, rba.WaitTaskPendingActivity, taskID).Get(pendingCtx, &state); err != nil {
		logger.Error("Failed to poll pending task ID:", err)
		return nil, skerr.Wrap(err)
	}

	if err := workflow.ExecuteActivity(ctx, rba.WaitTaskFinishedActivity, taskID).Get(ctx, &state); err != nil {
		logger.Error("Failed to poll running task ID:", err)
		return nil, skerr.Wrap(err)
	}

	if !state.IsTaskSuccessful() {
		return &workflows.TestRun{
			TaskID: taskID,
			Status: state,
		}, nil
	}

	var cas *swarmingV1.SwarmingRpcsCASReference
	if err := workflow.ExecuteActivity(ctx, rba.RetrieveTestCASActivity, taskID).Get(ctx, &cas); err != nil {
		logger.Error("Failed to retrieve CAS reference:", err)
		return nil, skerr.Wrap(err)
	}

	return &workflows.TestRun{
		TaskID: taskID,
		Status: state,
		CAS:    cas,
	}, nil
}

// ScheduleTaskActivity wraps run_benchmark.Run
func (rba *RunBenchmarkActivity) ScheduleTaskActivity(ctx context.Context, rbp *RunBenchmarkParams) (string, error) {
	logger := activity.GetLogger(ctx)

	sc, err := backends.NewSwarmingClient(ctx, backends.DefaultSwarmingServiceAddress)
	if err != nil {
		logger.Error("Failed to connect to swarming client:", err)
		return "", skerr.Wrap(err)
	}

	taskIds, err := run_benchmark.Run(ctx, sc, rbp.Commit.GetMainGitHash(), rbp.BotConfig, rbp.Benchmark, rbp.Story, rbp.StoryTags, rbp.JobID, rbp.BuildCAS, 1, rbp.Dimensions)
	if err != nil {
		return "", err
	}
	return taskIds[0].TaskId, nil
}

// WaitTaskPendingActivity polls the task until it is not longer pending. Returns the status
// if the task stops pending regardless of task success
func (rba *RunBenchmarkActivity) WaitTaskPendingActivity(ctx context.Context, taskID string) (run_benchmark.State, error) {
	logger := activity.GetLogger(ctx)

	sc, err := backends.NewSwarmingClient(ctx, backends.DefaultSwarmingServiceAddress)
	if err != nil {
		logger.Error("Failed to connect to swarming client:", err)
		return "", skerr.Wrap(err)
	}

	activity.RecordHeartbeat(ctx, "begin pending run_benchmark task polling")
	failureRetries := 5
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
			s, err := sc.GetStatus(ctx, taskID)
			state := run_benchmark.State(s)
			if err != nil {
				logger.Error("Failed to get task status:", err, "remaining retries:", failureRetries)
				failureRetries -= 1
				if failureRetries <= 0 {
					return "", skerr.Wrapf(err, "Failed to wait for task to start")
				}
			}
			if !state.IsTaskPending() {
				return state, nil
			}
			time.Sleep(15 * time.Second)
			activity.RecordHeartbeat(ctx, fmt.Sprintf("waiting on test %v with state %s", taskID, state))
		}
	}
}

// WaitTaskFinishedActivity polls the task until it finishes or errors. Returns the status
// if the task finishes regardless of task success
func (rba *RunBenchmarkActivity) WaitTaskFinishedActivity(ctx context.Context, taskID string) (run_benchmark.State, error) {
	logger := activity.GetLogger(ctx)

	sc, err := backends.NewSwarmingClient(ctx, backends.DefaultSwarmingServiceAddress)
	if err != nil {
		logger.Error("Failed to connect to swarming client:", err)
		return "", skerr.Wrap(err)
	}

	activity.RecordHeartbeat(ctx, "begin run_benchmark task running polling")
	failureRetries := 5
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
			s, err := sc.GetStatus(ctx, taskID)
			state := run_benchmark.State(s)
			if err != nil {
				logger.Error("Failed to get task status:", err, "remaining retries:", failureRetries)
				failureRetries -= 1
				if failureRetries <= 0 {
					return "", skerr.Wrapf(err, "Failed to wait for task to complete")
				}
			}
			if state.IsTaskFinished() {
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

// CleanupActivity wraps run_benchmark.Cancel
func (rba *RunBenchmarkActivity) CleanupBenchmarkRunActivity(ctx context.Context, taskID string, state run_benchmark.State) error {
	if len(taskID) == 0 || state.IsTaskFinished() {
		return nil
	}

	logger := activity.GetLogger(ctx)
	sc, err := backends.NewSwarmingClient(ctx, backends.DefaultSwarmingServiceAddress)
	if err != nil {
		logger.Error("Failed to connect to swarming client:", err)
		return skerr.Wrap(err)
	}

	err = run_benchmark.Cancel(ctx, sc, taskID)
	if err != nil {
		return err
	}
	return nil
}
