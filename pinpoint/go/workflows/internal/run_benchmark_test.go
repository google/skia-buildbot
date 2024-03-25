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

func TestRunBenchmark_GivenSuccessfulRun_ShouldReturnCas(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	var rba *RunBenchmarkActivity
	const fakeTaskID = "fake-task"
	const state = run_benchmark.State(swarming.TASK_STATE_COMPLETED)
	cas := &swarmingV1.SwarmingRpcsCASReference{
		CasInstance: "fake-instance",
	}

	env.OnActivity(rba.ScheduleTaskActivity, mock.Anything, mock.Anything).Return(fakeTaskID, nil).Once()
	env.OnActivity(rba.WaitTaskFinishedActivity, mock.Anything, fakeTaskID).Return(state, nil).Once()
	env.OnActivity(rba.RetrieveTestCASActivity, mock.Anything, fakeTaskID).Return(cas, nil).Once()

	env.ExecuteWorkflow(RunBenchmarkWorkflow, &RunBenchmarkParams{})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result *workflows.TestRun
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, &workflows.TestRun{
		TaskID: fakeTaskID,
		Status: state,
		CAS:    cas,
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
