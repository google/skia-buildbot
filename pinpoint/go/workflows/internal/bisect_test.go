package internal

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/pinpoint/go/workflows"
	pb "go.skia.org/infra/pinpoint/proto/v1"

	"github.com/stretchr/testify/require"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

// TODO(b/327019543): More tests and test data should be added here
//
//	This is only to validate the dependent workflow signature and the workflow can connect.
func Test_Bisect_SimpleNoDiffCommits_ShouldReturnEmptyCommit(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(SingleCommitRunner, workflow.RegisterOptions{Name: workflows.SingleCommitRunner})

	env.OnWorkflow(workflows.SingleCommitRunner, mock.Anything, mock.Anything).Return(&CommitRun{}, nil).Times(2)
	env.OnActivity(CompareResults, mock.Anything, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(false, nil).Once()

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
	require.Empty(t, be.Commit)
	env.AssertExpectations(t)
}
