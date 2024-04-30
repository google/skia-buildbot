package internal

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/pinpoint/go/run_benchmark"
	"go.skia.org/infra/pinpoint/go/workflows"

	"github.com/stretchr/testify/require"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.temporal.io/sdk/testsuite"
)

var mockCas = &swarmingV1.SwarmingRpcsCASReference{
	CasInstance: "fake-instance",
}

func TestRunBenchmark_GivenSuccessfulRun_ShouldReturnCas(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	var rba *RunBenchmarkActivity
	const fakeTaskID = "fake-task"
	const state = run_benchmark.State(swarming.TASK_STATE_COMPLETED)

	env.OnActivity(rba.ScheduleTaskActivity, mock.Anything, mock.Anything).Return(fakeTaskID, nil).Once()
	env.OnActivity(rba.WaitTaskPendingActivity, mock.Anything, fakeTaskID).Return(state, nil).Once()
	env.OnActivity(rba.WaitTaskFinishedActivity, mock.Anything, fakeTaskID).Return(state, nil).Once()
	env.OnActivity(rba.RetrieveTestCASActivity, mock.Anything, fakeTaskID).Return(mockCas, nil).Once()

	env.ExecuteWorkflow(RunBenchmarkWorkflow, &RunBenchmarkParams{})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result *workflows.TestRun
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, &workflows.TestRun{
		TaskID: fakeTaskID,
		Status: state,
		CAS:    mockCas,
	}, result)
	env.AssertExpectations(t)
}

func TestRunBenchmark_GivenUnsuccessfulRun_ShouldNotReturnCas(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	var rba *RunBenchmarkActivity
	const fakeTaskID = "fake-task"
	const state = run_benchmark.State(swarming.TASK_STATE_BOT_DIED)

	env.OnActivity(rba.ScheduleTaskActivity, mock.Anything, mock.Anything).Return(fakeTaskID, nil).Once()
	env.OnActivity(rba.WaitTaskPendingActivity, mock.Anything, fakeTaskID).Return(state, nil).Once()
	env.OnActivity(rba.WaitTaskFinishedActivity, mock.Anything, fakeTaskID).Return(state, nil).Once()

	env.ExecuteWorkflow(RunBenchmarkWorkflow, &RunBenchmarkParams{})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result *workflows.TestRun
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, &workflows.TestRun{
		TaskID: fakeTaskID,
		Status: state,
		CAS:    nil,
	}, result)
	env.AssertExpectations(t)
}

func TestRunBenchmark_ReturnsNoResource_TriesAgain(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	var rba *RunBenchmarkActivity
	const (
		fakeTaskID1       = "fake-task"
		fakeTaskID2       = "fake-task2"
		state_no_resource = run_benchmark.State(swarming.TASK_STATE_NO_RESOURCE)
		state_completed   = run_benchmark.State(swarming.TASK_STATE_COMPLETED)
	)

	env.OnActivity(rba.ScheduleTaskActivity, mock.Anything, mock.Anything).Return(fakeTaskID1, nil).Once()
	env.OnActivity(rba.WaitTaskPendingActivity, mock.Anything, fakeTaskID1).Return(state_no_resource, nil).Once()
	env.OnActivity(rba.ScheduleTaskActivity, mock.Anything, mock.Anything).Return(fakeTaskID2, nil).Once()
	env.OnActivity(rba.WaitTaskPendingActivity, mock.Anything, fakeTaskID2).Return(state_completed, nil).Once()
	env.OnActivity(rba.WaitTaskFinishedActivity, mock.Anything, fakeTaskID2).Return(state_completed, nil).Once()
	env.OnActivity(rba.RetrieveTestCASActivity, mock.Anything, fakeTaskID2).Return(mockCas, nil).Once()

	env.ExecuteWorkflow(RunBenchmarkWorkflow, &RunBenchmarkParams{})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result *workflows.TestRun
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, &workflows.TestRun{
		TaskID: fakeTaskID2,
		Status: state_completed,
		CAS:    mockCas,
	}, result)
	env.AssertExpectations(t)
}

func TestRunBenchmark_ReturnsNoResourceTooManyTimes_ErrorsOut(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	var rba *RunBenchmarkActivity
	const (
		fakeTaskID        = "fake-task"
		state_no_resource = run_benchmark.State(swarming.TASK_STATE_NO_RESOURCE)
	)

	env.OnActivity(rba.ScheduleTaskActivity, mock.Anything, mock.Anything).Return(fakeTaskID, nil).Times(maxRetry)
	env.OnActivity(rba.WaitTaskPendingActivity, mock.Anything, fakeTaskID).Return(state_no_resource, nil).Times(maxRetry)
	env.OnActivity(rba.WaitTaskFinishedActivity, mock.Anything, fakeTaskID).Return(state_no_resource, nil).Once()

	env.ExecuteWorkflow(RunBenchmarkWorkflow, &RunBenchmarkParams{})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result *workflows.TestRun
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, &workflows.TestRun{
		TaskID: fakeTaskID,
		Status: state_no_resource,
		CAS:    nil,
	}, result)
	env.AssertExpectations(t)
}
