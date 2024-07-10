package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/pinpoint/go/run_benchmark"
	"go.skia.org/infra/pinpoint/go/workflows"

	"github.com/stretchr/testify/require"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.temporal.io/sdk/testsuite"
)

var mockCas = &apipb.CASReference{
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
	require.EqualExportedValues(t, workflows.TestRun{
		TaskID: fakeTaskID,
		Status: state,
		CAS:    mockCas,
	}, *result)
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
	require.EqualExportedValues(t, workflows.TestRun{
		TaskID: fakeTaskID,
		Status: state,
		CAS:    nil,
	}, *result)
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
	require.EqualExportedValues(t, workflows.TestRun{
		TaskID: fakeTaskID2,
		Status: state_completed,
		CAS:    mockCas,
	}, *result)
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
	require.EqualExportedValues(t, workflows.TestRun{
		TaskID: fakeTaskID,
		Status: state_no_resource,
		CAS:    nil,
	}, *result)
	env.AssertExpectations(t)
}

func TestRunBenchmarkPairwise_HappyPath_ReturnsCAS(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	var rba *RunBenchmarkActivity
	p1 := &RunBenchmarkParams{Dimensions: map[string]string{"value": "bot-123"}}
	p2 := &RunBenchmarkParams{Dimensions: map[string]string{"value": "bot-123"}}
	mockTaskID1, mockTaskID2 := "fake-task1", "fake-task2"
	state := run_benchmark.State(swarming.TASK_STATE_COMPLETED)

	env.OnActivity(rba.ScheduleTaskActivity, mock.Anything, mock.Anything).Return(mockTaskID1, nil).Once()
	env.OnActivity(rba.WaitTaskAcceptedActivity, mock.Anything, mockTaskID1).Return(state, nil).Once()
	env.OnActivity(rba.ScheduleTaskActivity, mock.Anything, mock.Anything).Return(mockTaskID2, nil).Once()
	env.OnActivity(rba.WaitTaskPendingActivity, mock.Anything, mockTaskID1).Return(state, nil).Once()
	env.OnActivity(rba.WaitTaskPendingActivity, mock.Anything, mockTaskID2).Return(state, nil).Once()
	env.OnActivity(rba.WaitTaskFinishedActivity, mock.Anything, mockTaskID1).Return(state, nil).Once()
	env.OnActivity(rba.WaitTaskFinishedActivity, mock.Anything, mockTaskID2).Return(state, nil).Once()
	env.OnActivity(rba.RetrieveTestCASActivity, mock.Anything, mockTaskID1).Return(mockCas, nil).Once()
	env.OnActivity(rba.RetrieveTestCASActivity, mock.Anything, mockTaskID2).Return(mockCas, nil).Once()

	env.ExecuteWorkflow(RunBenchmarkPairwiseWorkflow, p1, p2, workflows.LeftThenRight)

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result *workflows.PairwiseTestRun
	require.NoError(t, env.GetWorkflowResult(&result))
	assert.EqualExportedValues(t, workflows.PairwiseTestRun{
		FirstTestRun: &workflows.TestRun{
			TaskID: mockTaskID1,
			Status: state,
			CAS:    mockCas,
		},
		SecondTestRun: &workflows.TestRun{
			TaskID: mockTaskID2,
			Status: state,
			CAS:    mockCas,
		},
	}, *result)
	env.AssertExpectations(t)
}

func TestRunBenchmarkPairwise_NoResources_ReturnsError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	var rba *RunBenchmarkActivity
	p1 := &RunBenchmarkParams{Dimensions: map[string]string{"value": "bot-123"}}
	p2 := &RunBenchmarkParams{Dimensions: map[string]string{"value": "bot-123"}}
	mockTaskID1 := "fake-task1"
	noResourceState := run_benchmark.State(swarming.TASK_STATE_NO_RESOURCE)
	expectedErr := skerr.Fmt("Failed to wait for task %s to be accepted", mockTaskID1)

	env.OnActivity(rba.ScheduleTaskActivity, mock.Anything, mock.Anything).Return(mockTaskID1, nil).Once()
	env.OnActivity(rba.WaitTaskAcceptedActivity, mock.Anything, mockTaskID1).Return(noResourceState, expectedErr).Times(int(runBenchmarkPendingActivityOption.RetryPolicy.MaximumAttempts))

	env.ExecuteWorkflow(RunBenchmarkPairwiseWorkflow, p1, p2, workflows.RightThenLeft)

	require.True(t, env.IsWorkflowCompleted())
	assert.Error(t, env.GetWorkflowError())
	var result *workflows.PairwiseTestRun
	assert.Error(t, env.GetWorkflowResult(&result))
	assert.Nil(t, result)
	env.AssertExpectations(t)
}
