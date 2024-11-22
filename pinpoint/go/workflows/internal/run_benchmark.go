package internal

import (
	"context"
	"errors"
	"fmt"
	"time"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/backends"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/run_benchmark"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.temporal.io/sdk/activity"
	"go.temporal.io/sdk/workflow"
)

const maxRetry = 3

// RunBenchmarkParams are the Temporal Workflow params
// for the RunBenchmarkWorkflow.
type RunBenchmarkParams struct {
	// the Pinpoint job id
	JobID string
	// the swarming instance and cas digest hash and bytes location for the build
	BuildCAS *apipb.CASReference
	// commit hash
	Commit *common.CombinedCommit
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
	// Chart is a story histogram in a Benchmark.
	Chart string
	// AggregationMethod is method to aggregate sampled values.
	// If empty, then the original values are returned.
	AggregationMethod string
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

	// sometimes bots can die in the middle of a Pinpoint job. If a task is scheduled
	// onto a dead bot, the swarming task will return NO_RESOURCE. In that case, reschedule
	// the run on any other bot.
	// TODO(sunxiaodi@): Monitor how often tasks fail with NO_RESOURCE. We want to maintain this
	// occurence below a threshold i.e. 5%.
	for attempt := 1; canRetry(state, attempt); attempt++ {
		if err := workflow.ExecuteActivity(ctx, rba.ScheduleTaskActivity, p).Get(ctx, &taskID); err != nil {
			logger.Error("Failed to schedule task:", err)
			return nil, skerr.Wrap(err)
		}
		// polling pending and polling running are two different activities
		// because swarming tasks can be pending for hours while swarming tasks
		// generally finish in ~10 min
		if err := workflow.ExecuteActivity(pendingCtx, rba.WaitTaskPendingActivity, taskID).Get(pendingCtx, &state); err != nil {
			logger.Error("Failed to poll pending task ID:", err)
			return nil, skerr.Wrap(err)
		}
		// remove the bot ID from the swarming task request so that the task can
		// schedule on all bots in the pool for future retries
		p.Dimensions = nil
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

	var cas *apipb.CASReference
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

func canRetry(state run_benchmark.State, attempt int) bool {
	return (state == run_benchmark.State("") || state.IsNoResource()) && attempt <= maxRetry
}

// RunBenchmarkPairwiseWorkflow is a Workflow definition that schedules a pairwise of tasks,
// polls and retrieves the CAS for the RunBenchmarkParams defined.
// TODO(b/340247044): connect mutex lock to this workflow and lock the swarming resource
// from the same pinpoint job and other pinpoint jobs. After swarming tasks have scheduled,
// the mutex lock can be released and the rest of the workflow can proceed. This workflow
// will also not schedule swarming tasks until it obtains the lock on the swarming resource.
// TODO(sunxiaodi@): Convert this workflow to accept slice and replace RunBenchmarkWorkflow
// with this workflow.
func RunBenchmarkPairwiseWorkflow(ctx workflow.Context, firstRBP, secondRBP *RunBenchmarkParams, first workflows.PairwiseOrder) (*workflows.PairwiseTestRun, error) {
	if firstRBP.Dimensions["value"] == "" || secondRBP.Dimensions["value"] == "" {
		return nil, skerr.Fmt("no bot ID provided to either first params: %s or second params: %s in pairwise run benchmark workflow", firstRBP.Dimensions["value"], secondRBP.Dimensions["value"])
	}

	ctx = workflow.WithActivityOptions(ctx, runBenchmarkActivityOption)
	pendingCtx := workflow.WithActivityOptions(ctx, runBenchmarkPendingActivityOption)
	logger := workflow.GetLogger(ctx)

	var rba RunBenchmarkActivity
	var firstTaskID, secondTaskID string
	var firstState, secondState run_benchmark.State
	// defer activity cleanup if workflow is cancelled
	defer func() {
		// ErrCanceled is the error returned by Context.Err when the context is canceled
		// This logic ensures cleanup only happens if there is a Cancellation error
		if !errors.Is(ctx.Err(), workflow.ErrCanceled) {
			return
		}
		// For the Workflow to execute an Activity after it receives a Cancellation Request
		// It has to get a new disconnected context
		newCtx, _ := workflow.NewDisconnectedContext(ctx)

		err := workflow.ExecuteActivity(newCtx, rba.CleanupBenchmarkRunActivity, firstTaskID, firstState).Get(ctx, nil)
		if err != nil {
			logger.Error("CleanupBenchmarkRunActivity failed", err)
		}

		err = workflow.ExecuteActivity(newCtx, rba.CleanupBenchmarkRunActivity, secondTaskID, secondState).Get(ctx, nil)
		if err != nil {
			logger.Error("CleanupBenchmarkRunActivity failed", err)
		}
	}()

	// monitor task interception and ordering
	var isTaskContinous, isTaskOrdered bool
	var taskContError, taskOrderError error
	mh := workflow.GetMetricsHandler(ctx).WithTags(map[string]string{
		"job_id":    firstRBP.JobID,
		"benchmark": firstRBP.Benchmark,
		"config":    firstRBP.BotConfig,
		"story":     firstRBP.Story,
		"bot_id":    firstRBP.Dimensions["value"],
		"task1":     firstTaskID,
		"task2":     secondTaskID,
	})
	mh.Counter("pairwise_task_count").Inc(1)
	defer func() {
		if taskContError == nil && isTaskContinous {
			mh.Counter("pairwise_task_continuous_true").Inc(1)
		} else if taskContError == nil && !isTaskContinous {
			mh.Counter("pairwise_task_continuous_false").Inc(1)
		} else {
			mh.Counter("pairwise_task_continuous_error").Inc(1)
		}

		if taskOrderError == nil && isTaskOrdered {
			mh.Counter("pairwise_task_order_true").Inc(1)
		} else if taskOrderError == nil && !isTaskOrdered {
			mh.Counter("pairwise_task_order_false").Inc(1)
		} else {
			mh.Counter("pairwise_task_order_error").Inc(1)
		}

		if errors.Is(ctx.Err(), workflow.ErrCanceled) || errors.Is(ctx.Err(), workflow.ErrDeadlineExceeded) {
			mh.Counter("pairwise_task_timeout_count").Inc(1)
		}
	}()

	if err := workflow.ExecuteActivity(ctx, rba.ScheduleTaskActivity, firstRBP).Get(ctx, &firstTaskID); err != nil {
		logger.Error("Failed to schedule first task:", err)
		return nil, skerr.Wrap(err)
	}

	if err := workflow.ExecuteActivity(pendingCtx, rba.WaitTaskAcceptedActivity, firstTaskID).Get(pendingCtx, &firstState); err != nil {
		logger.Error("Failed to poll accepted first task ID:", err)
		return nil, skerr.Wrap(err)
	}

	if err := workflow.ExecuteActivity(ctx, rba.ScheduleTaskActivity, secondRBP).Get(ctx, &secondTaskID); err != nil {
		logger.Error("Failed to schedule second task:", err)
		return nil, skerr.Wrap(err)
	}

	if err := workflow.ExecuteActivity(pendingCtx, rba.WaitTaskPendingActivity, firstTaskID).Get(pendingCtx, &firstState); err != nil {
		logger.Error("Failed to poll pending first task ID:", err)
		return nil, skerr.Wrap(err)
	}

	if err := workflow.ExecuteActivity(pendingCtx, rba.WaitTaskPendingActivity, secondTaskID).Get(pendingCtx, &secondState); err != nil {
		logger.Error("Failed to poll pending second task ID:", err)
		return nil, skerr.Wrap(err)
	}

	if err := workflow.ExecuteActivity(ctx, rba.WaitTaskFinishedActivity, firstTaskID).Get(ctx, &firstState); err != nil {
		logger.Error("Failed to poll running first task ID:", err)
		return nil, skerr.Wrap(err)
	}

	if err := workflow.ExecuteActivity(ctx, rba.WaitTaskFinishedActivity, secondTaskID).Get(ctx, &secondState); err != nil {
		logger.Error("Failed to poll running second task ID:", err)
		return nil, skerr.Wrap(err)
	}

	// We do not handle the error because they do not affect the overall workflow's function.
	// The error will be counted and monitored.
	taskOrderError = workflow.ExecuteActivity(ctx, rba.IsTaskPairOrderedActivity, firstTaskID, secondTaskID).Get(ctx, &isTaskOrdered)
	taskContError = workflow.ExecuteActivity(ctx, rba.IsTaskPairContinuousActivity, firstRBP.Dimensions["value"], firstTaskID, secondTaskID).Get(ctx, &isTaskContinous)

	if !firstState.IsTaskSuccessful() || !secondState.IsTaskSuccessful() {
		return &workflows.PairwiseTestRun{
			FirstTestRun: &workflows.TestRun{
				TaskID: firstTaskID,
				Status: firstState,
			},
			SecondTestRun: &workflows.TestRun{
				TaskID: secondTaskID,
				Status: secondState,
			},
			Permutation: workflows.PairwiseOrder(first),
		}, nil
	}

	var firstCAS, secondCAS *apipb.CASReference
	if err := workflow.ExecuteActivity(ctx, rba.RetrieveTestCASActivity, firstTaskID).Get(ctx, &firstCAS); err != nil {
		logger.Error("Failed to retrieve first CAS reference:", err)
		return nil, skerr.Wrap(err)
	}
	if err := workflow.ExecuteActivity(ctx, rba.RetrieveTestCASActivity, secondTaskID).Get(ctx, &secondCAS); err != nil {
		logger.Error("Failed to retrieve second CAS reference:", err)
		return nil, skerr.Wrap(err)
	}

	return &workflows.PairwiseTestRun{
		FirstTestRun: &workflows.TestRun{
			TaskID: firstTaskID,
			Status: firstState,
			CAS:    firstCAS,
		},
		SecondTestRun: &workflows.TestRun{
			TaskID: secondTaskID,
			Status: secondState,
			CAS:    secondCAS,
		},
		Permutation: workflows.PairwiseOrder(first),
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

// WaitTaskAcceptedActivity polls the task until Swarming schedules the task.
// If the task is not scheduled, then it returns NO_RESOURCE.
// Note that there are other causes for NO_RESOURCE, but the solution is generally
// the same: schedule the run on a different, available bot.
// This activity is intended to only be used by pairwise workflow.
func (rba *RunBenchmarkActivity) WaitTaskAcceptedActivity(ctx context.Context, taskID string) (run_benchmark.State, error) {
	logger := activity.GetLogger(ctx)

	sc, err := backends.NewSwarmingClient(ctx, backends.DefaultSwarmingServiceAddress)
	if err != nil {
		logger.Error("Failed to connect to swarming client:", err)
		return "", skerr.Wrap(err)
	}

	activity.RecordHeartbeat(ctx, "begin accepted run_benchmark task polling")
	// TODO(b/327224992): Investigate if it is possible to consolidate activity retry logic
	// for all run_benchmark activities.
	failureRetries := 5
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
			s, err := sc.GetStatus(ctx, taskID)
			if err != nil {
				logger.Error("Failed to get task status:", err, "remaining retries:", failureRetries)
				failureRetries -= 1
				if failureRetries <= 0 {
					return "", skerr.Wrapf(err, "Failed to wait for task to be accepted")
				}
				time.Sleep(3 * time.Second) // duration is shorter as swarming should schedule the task fast
				activity.RecordHeartbeat(ctx, fmt.Sprintf("waiting on test %v with state %s", taskID, s))
				continue
			}
			// Swarming state in no resource implies bot is not available or the task
			// is not yet scheduled.
			if run_benchmark.State(s).IsNoResource() {
				logger.Warn("swarming task:", taskID, "had status:", s, "remaining retries:", failureRetries)
				failureRetries -= 1
				if failureRetries <= 0 {
					return run_benchmark.State(s), skerr.Wrapf(err, "Failed to wait for task %s to be accepted", taskID)
				}
				time.Sleep(3 * time.Second) // duration is shorter as swarming should schedule the task fast
				activity.RecordHeartbeat(ctx, fmt.Sprintf("waiting on test %v with state %s", taskID, s))
				continue
			}
			return run_benchmark.State(s), nil
		}
	}
}

// WaitTaskPendingActivity polls the task until it is no longer pending. Returns the status
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
			} else if !state.IsTaskPending() {
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
func (rba *RunBenchmarkActivity) RetrieveTestCASActivity(ctx context.Context, taskID string) (*apipb.CASReference, error) {
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

func (rba *RunBenchmarkActivity) IsTaskPairContinuousActivity(ctx context.Context, botID, taskID1, taskID2 string) (bool, error) {
	sc, err := backends.NewSwarmingClient(ctx, backends.DefaultSwarmingServiceAddress)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	tasks, err := sc.GetBotTasksBetweenTwoTasks(ctx, botID, taskID1, taskID2)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	// We expect one swarming task between the two time stamps.
	// If there are < 1 items, then either one task did not start (i.e. no resource) or they occured out of order.
	// More than one implies that a task intercepted task1 and task2.
	switch len(tasks.Items) {
	case 0:
		return false, skerr.Fmt("no tasks reported for bot %s given tasks %s and %s", botID, taskID1, taskID2)
	case 1:
		return true, nil
	}
	return false, nil
}

func (rba *RunBenchmarkActivity) IsTaskPairOrderedActivity(ctx context.Context, taskID1, taskID2 string) (bool, error) {
	sc, err := backends.NewSwarmingClient(ctx, backends.DefaultSwarmingServiceAddress)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	t1Start, err := sc.GetStartTime(ctx, taskID1)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	t2Start, err := sc.GetStartTime(ctx, taskID2)
	if err != nil {
		return false, skerr.Wrap(err)
	}
	return t1Start.AsTime().Before(t2Start.AsTime()), nil
}
