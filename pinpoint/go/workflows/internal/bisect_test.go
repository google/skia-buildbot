package internal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/midpoint"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	pb "go.skia.org/infra/pinpoint/proto/v1"
)

func mockedSingleCommitRun(ctx workflow.Context, p *SingleCommitRunnerParams) (*CommitRun, error) {
	return &CommitRun{
		Build: &workflows.Build{
			BuildChromeParams: workflows.BuildChromeParams{
				Commit: *p.CombinedCommit,
			},
		},
		Runs: make([]*workflows.TestRun, p.Iterations),
	}, nil
}

func mockedGetAllValuesLocalActivity(ctx context.Context, cr *BisectRun, chart string) (*CommitValues, error) {
	return &CommitValues{
		Commit: cr.Build.Commit,
		Values: make([]float64, 0),
	}, nil
}

func mockedGetErrorValuesLocalActivity(ctx context.Context, cr *BisectRun, chart string) (*CommitValues, error) {
	return &CommitValues{
		Commit: cr.Build.Commit,
		Values: make([]float64, 0),
	}, nil
}

// TODO(b/327019543): More tests and test data should be added here
//
//	This is only to validate the dependent workflow signature and the workflow can connect.
func TestBisectWorkflow_SimpleNoDiffCommits_ShouldReturnEmptyCommit(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(SingleCommitRunner, workflow.RegisterOptions{Name: workflows.SingleCommitRunner})

	env.OnWorkflow(workflows.SingleCommitRunner, mock.Anything, mock.Anything).Return(mockedSingleCommitRun).Times(2)
	env.OnActivity(GetErrorValuesLocalActivity, mock.Anything, mock.Anything, mock.Anything).Return(mockedGetErrorValuesLocalActivity).Twice()
	env.OnActivity(CompareFunctionalActivity, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&compare.CompareResults{Verdict: compare.Same}, nil).Once()
	env.OnActivity(GetAllValuesLocalActivity, mock.Anything, mock.Anything, mock.Anything).Return(mockedGetAllValuesLocalActivity).Twice()
	env.OnActivity(ComparePerformanceActivity, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(&compare.CompareResults{Verdict: compare.Same}, nil).Once()

	env.ExecuteWorkflow(BisectWorkflow, &workflows.BisectParams{
		Request: &pb.ScheduleBisectRequest{
			ComparisonMagnitude: "1",
		},
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var be *pb.BisectExecution
	require.NoError(t, env.GetWorkflowResult(&be))
	require.NotNil(t, be)
	require.NotEmpty(t, be.JobId)
	require.Empty(t, be.Culprits)
	env.AssertExpectations(t)
}

func TestBisectRunTracker_NewIdx_ReturnSameRun(t *testing.T) {
	tracker := bisectRunTracker{}
	idx, run := tracker.newRun(&midpoint.CombinedCommit{})
	require.Same(t, run, tracker.get(idx), "should be exact same addresses")
}

func TestBisectRunTracker_TwoRuns_ReturnDiffIndex(t *testing.T) {
	tracker := bisectRunTracker{}
	idx1, run1 := tracker.newRun(&midpoint.CombinedCommit{})
	idx2, run2 := tracker.newRun(&midpoint.CombinedCommit{})
	require.NotEqualValues(t, idx1, idx2)
	require.NotSame(t, run1, run2, "pointers should be different")
	require.NotSame(t, tracker.get(idx1), tracker.get(idx2), "pointers should be different")
}

func TestBisectRunTracker_NonExistIndex_ReturnNil(t *testing.T) {
	nonExist := BisectRunIndex(1000)
	tracker := bisectRunTracker{}
	require.Nil(t, tracker.get(nonExist))
	_, _ = tracker.newRun(&midpoint.CombinedCommit{})
	require.Nil(t, tracker.get(nonExist))
}

func TestBisectRunTracker_ManyRuns_ReturnIndex(t *testing.T) {
	tracker := bisectRunTracker{}
	for i := 0; i < 100; i++ {
		idx, run := tracker.newRun(&midpoint.CombinedCommit{})
		require.Same(t, run, tracker.get(idx), "should be exact same addresses")
	}
}
