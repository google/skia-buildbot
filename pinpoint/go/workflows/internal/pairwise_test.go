package internal

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/go/skerr"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/compare"
	"go.skia.org/infra/pinpoint/go/workflows"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

const mockChart = "chart"

func TestPairwiseWorkflow_GivenUnsuccessfulWorkflow_ReturnsError(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(PairwiseCommitsRunnerWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseCommitsRunner})
	env.OnWorkflow(workflows.PairwiseCommitsRunner, mock.Anything, mock.Anything).Return(nil, skerr.Fmt("some error")).Once()

	env.ExecuteWorkflow(PairwiseWorkflow, &workflows.PairwiseParams{
		Request: &pinpoint_proto.SchedulePairwiseRequest{
			Chart: mockChart,
		},
	})
	require.True(t, env.IsWorkflowCompleted())
	require.Error(t, env.GetWorkflowError())

	var pe *pinpoint_proto.PairwiseExecution
	require.Error(t, env.GetWorkflowResult(&pe))
	assert.Nil(t, pe)
	env.AssertExpectations(t)
}

func TestPairwiseWorkflow_GivenSuccessfulWorkflow_ReturnsCorrectPValues(t *testing.T) {
	mockedValuesA := []float64{1.2, 2.2, 1.6, 2.0}
	mockedValuesB := []float64{5.5, 6.6, 5.8, 6.3}
	mockResult := &PairwiseRun{
		Left: CommitRun{
			Runs: []*workflows.TestRun{
				{
					Values: map[string][]float64{
						mockChart: mockedValuesA,
					},
				},
			},
		},
		Right: CommitRun{
			Runs: []*workflows.TestRun{
				{
					Values: map[string][]float64{
						mockChart: mockedValuesB,
					},
				},
			},
		},
	}
	statResults, err := compare.ComparePairwise(mockedValuesA, mockedValuesB, compare.UnknownDir)
	require.NoError(t, err)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(PairwiseCommitsRunnerWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseCommitsRunner})
	env.OnWorkflow(workflows.PairwiseCommitsRunner, mock.Anything, mock.Anything).Return(mockResult, nil).Once()
	env.OnActivity(ComparePairwiseActivity, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(statResults, nil).Once()

	env.ExecuteWorkflow(PairwiseWorkflow, &workflows.PairwiseParams{
		Request: &pinpoint_proto.SchedulePairwiseRequest{
			Chart: mockChart,
		},
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var pe *pinpoint_proto.PairwiseExecution
	require.NoError(t, env.GetWorkflowResult(&pe))
	assert.NotNil(t, pe)
	assert.NotEmpty(t, pe.JobId)
	assert.Equal(t, statResults.PValue, pe.Statistic.PValue)
	assert.Nil(t, pe.Culprit)
	env.AssertExpectations(t)
}

func TestPairwiseWorkflow_GivenSuccessfulWorkflowWithCulprit_ReturnsCulprit(t *testing.T) {
	mockedValuesA := []float64{1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 1.7, 1.8, 1.9}
	mockedValuesB := []float64{10.1, 10.2, 10.3, 10.4, 10.5, 10.6, 10.7, 10.8, 10.9}
	mockResult := &PairwiseRun{
		Left: CommitRun{
			Runs: []*workflows.TestRun{
				{
					Values: map[string][]float64{
						mockChart: mockedValuesA,
					},
				},
			},
		},
		Right: CommitRun{
			Runs: []*workflows.TestRun{
				{
					Values: map[string][]float64{
						mockChart: mockedValuesB,
					},
				},
			},
		},
	}
	statResults, err := compare.ComparePairwise(mockedValuesA, mockedValuesB, compare.UnknownDir)
	require.NoError(t, err)

	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(PairwiseCommitsRunnerWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseCommitsRunner})
	env.OnWorkflow(workflows.PairwiseCommitsRunner, mock.Anything, mock.Anything).Return(mockResult, nil).Once()
	env.OnActivity(ComparePairwiseActivity, mock.Anything, mock.Anything, mock.Anything, mock.Anything).Return(statResults, nil).Once()

	env.ExecuteWorkflow(PairwiseWorkflow, &workflows.PairwiseParams{
		Request: &pinpoint_proto.SchedulePairwiseRequest{
			Chart: mockChart,
			EndCommit: &pinpoint_proto.CombinedCommit{
				Main: common.NewChromiumCommit("fake-commit"),
			},
		},
		CulpritVerify: true,
	})
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var pe *pinpoint_proto.PairwiseExecution
	require.NoError(t, env.GetWorkflowResult(&pe))
	assert.NotNil(t, pe)
	assert.NotEmpty(t, pe.JobId)
	assert.Equal(t, statResults.PValue, pe.Statistic.PValue)
	assert.Equal(t, "fake-commit", pe.Culprit.Main.GitHash)
	env.AssertExpectations(t)
}
