package catapult

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/worker"
	"go.temporal.io/sdk/workflow"

	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/internal"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
)

func TestUpdateStatesWithComparisons_LessThanOneState_Error(t *testing.T) {
	states := []*pinpoint_proto.LegacyJobResponse_State{{}}
	err := updateStatesWithComparisons(states, 0.0, compare.Down)
	assert.Error(t, err)
}

func TestUpdateStatesWithComparisons_OneComparison_Same(t *testing.T) {
	states := []*pinpoint_proto.LegacyJobResponse_State{
		{
			Values: []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		},
		{
			Values: []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		},
	}
	err := updateStatesWithComparisons(states, 0.0, compare.Down)
	require.NoError(t, err)
	assert.Empty(t, states[0].Comparisons.Prev)
	assert.Equal(t, string(compare.Same), states[0].Comparisons.Next)
	assert.Equal(t, string(compare.Same), states[1].Comparisons.Prev)
	assert.Empty(t, states[1].Comparisons.Next)
}

func TestUpdateStatesWithComparisons_MultiComparison_Different(t *testing.T) {
	states := []*pinpoint_proto.LegacyJobResponse_State{
		{
			Values: []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		},
		{
			Values: []float64{0, 1, 2, 3, 4, 5, 6, 7, 8, 9},
		},
		{
			Values: []float64{7, 8, 9, 10, 11, 12, 13, 14, 15, 16},
		},
	}
	err := updateStatesWithComparisons(states, 0.0, compare.Down)
	require.NoError(t, err)
	assert.Empty(t, states[0].Comparisons.Prev)
	assert.Equal(t, string(compare.Same), states[0].Comparisons.Next)
	assert.Equal(t, string(compare.Same), states[1].Comparisons.Prev)
	assert.Equal(t, string(compare.Different), states[1].Comparisons.Next)
	assert.Equal(t, string(compare.Different), states[2].Comparisons.Prev)
	assert.Empty(t, states[2].Comparisons.Next)
}

func TestCatapultBisectWorkflow_HappyPath_ReturnsDatastoreResponse(t *testing.T) {
	mockBisectExecution := &internal.BisectExecution{
		JobId: mockJobId,
	}
	mockDSResp, err := unmarshalMockDatastoreResp(mockDatastoreResp)
	require.NoError(t, err)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()
	env.RegisterWorkflowWithOptions(internal.BisectWorkflow, workflow.RegisterOptions{Name: workflows.Bisect})
	env.RegisterWorkflowWithOptions(ConvertToCatapultResponseWorkflow, workflow.RegisterOptions{Name: workflows.ConvertToCatapultResponseWorkflow})

	env.OnWorkflow(workflows.Bisect, mock.Anything, mock.Anything).Return(mockBisectExecution, nil).Once()
	env.OnWorkflow(workflows.ConvertToCatapultResponseWorkflow, mock.Anything, mock.Anything, mockBisectExecution).Return(mockPinpointLegacyJobResp, nil).Once()
	env.OnActivity(WriteBisectToCatapultActivity, mock.Anything, mockPinpointLegacyJobResp, false).Return(mockDSResp, nil).Once()

	env.ExecuteWorkflow(CatapultBisectWorkflow, &workflows.BisectParams{
		Request: &pinpoint_proto.ScheduleBisectRequest{},
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var actual *pinpoint_proto.BisectExecution
	require.NoError(t, env.GetWorkflowResult(&actual))
	assert.NotNil(t, actual)
	assert.Equal(t, mockJobId, actual.JobId)
	assert.Empty(t, actual.Culprits)
	env.AssertExpectations(t)
}

func TestCatapultBisectWorkflow_ReplayEvents_ShouldAlwaysPass(t *testing.T) {
	replayer := worker.NewWorkflowReplayer()

	replayer.RegisterWorkflowWithOptions(CatapultBisectWorkflow, workflow.RegisterOptions{Name: workflows.CatapultBisect})
	replayer.RegisterWorkflowWithOptions(internal.BisectWorkflow, workflow.RegisterOptions{Name: workflows.Bisect})
	replayer.RegisterWorkflowWithOptions(ConvertToCatapultResponseWorkflow, workflow.RegisterOptions{Name: workflows.ConvertToCatapultResponseWorkflow})

	err := replayer.ReplayWorkflowHistoryFromJSONFile(nil, "testdata/catapult_bisect_event_history_20240627.json")
	assert.NoError(t, err)
}
