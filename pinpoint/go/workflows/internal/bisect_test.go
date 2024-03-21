package internal

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"

	pb "go.skia.org/infra/pinpoint/proto/v1"
)

func mockedSingleCommitRun(ctx workflow.Context, p *SingleCommitRunnerParams) (*CommitRun, error) {
	return &CommitRun{
		Commit: p.CombinedCommit,
		Runs:   make([]*workflows.TestRun, p.Iterations),
	}, nil
}

// TODO(b/327019543): More tests and test data should be added here
//
//	This is only to validate the dependent workflow signature and the workflow can connect.
func TestBisect_SimpleNoDiffCommits_ShouldReturnEmptyCommit(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(SingleCommitRunner, workflow.RegisterOptions{Name: workflows.SingleCommitRunner})

	env.OnWorkflow(workflows.SingleCommitRunner, mock.Anything, mock.Anything).Return(mockedSingleCommitRun, nil).Times(2)
	env.OnActivity(GetAllValuesLocalActivity, mock.Anything, mock.Anything, mock.Anything).Return(&CommitValues{}, nil).Twice()
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
