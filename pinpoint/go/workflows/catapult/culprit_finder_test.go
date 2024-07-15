package catapult

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"go.skia.org/infra/pinpoint/go/common"
	"go.skia.org/infra/pinpoint/go/workflows"
	"go.skia.org/infra/pinpoint/go/workflows/internal"
	pinpoint_proto "go.skia.org/infra/pinpoint/proto/v1"
	"go.temporal.io/sdk/testsuite"
	"go.temporal.io/sdk/workflow"
)

func generateCulpritFinderParams() *workflows.CulpritFinderParams {
	return &workflows.CulpritFinderParams{
		Request: &pinpoint_proto.ScheduleCulpritFinderRequest{
			StartGitHash:         "8f2037564966f83e53701d157622dd42b931a13f",
			EndGitHash:           "049ab03450dd980d3afc27f13edfef9f510ed819",
			Configuration:        "win-11-perf",
			Benchmark:            "speedometer2",
			Story:                "Speedometer2",
			Chart:                "RunsPerMinute",
			Statistic:            "",
			ComparisonMagnitude:  "1.0",
			ImprovementDirection: "Up",
		},
	}
}

func TestCulpritFinder_NoRegression_ReturnsEarly(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(internal.PairwiseWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseWorkflow})

	env.OnWorkflow(workflows.PairwiseWorkflow, mock.Anything, mock.Anything).Return(&pinpoint_proto.PairwiseExecution{
		Significant: false,
	}, nil).Once()

	env.ExecuteWorkflow(CulpritFinderWorkflow, generateCulpritFinderParams())
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var cfe *pinpoint_proto.CulpritFinderExecution
	require.NoError(t, env.GetWorkflowResult(&cfe))
	assert.NotNil(t, cfe)
	assert.False(t, cfe.RegressionVerified)
	assert.Nil(t, cfe.Culprits)
	env.AssertExpectations(t)
}

func TestCulpritFinder_NoCulpritsAfterBisect_ReturnsRegression(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(internal.PairwiseWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseWorkflow})
	env.RegisterWorkflowWithOptions(CatapultBisectWorkflow, workflow.RegisterOptions{Name: workflows.CatapultBisect})

	env.OnWorkflow(workflows.PairwiseWorkflow, mock.Anything, mock.Anything).Return(
		&pinpoint_proto.PairwiseExecution{
			Significant: true,
			Statistic: &pinpoint_proto.PairwiseExecution_WilcoxonResult{
				ControlMedian:   0.1, // arbitrary values
				TreatmentMedian: 0.2,
			},
		}, nil).Once()
	env.OnWorkflow(workflows.CatapultBisect, mock.Anything, mock.Anything).Return(&pinpoint_proto.BisectExecution{}, nil).Once()

	env.ExecuteWorkflow(CulpritFinderWorkflow, generateCulpritFinderParams())
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var cfe *pinpoint_proto.CulpritFinderExecution
	require.NoError(t, env.GetWorkflowResult(&cfe))
	assert.NotNil(t, cfe)
	assert.True(t, cfe.RegressionVerified)
	assert.Nil(t, cfe.Culprits)
	env.AssertExpectations(t)
}

func TestCulpritFinder_CulpritsVerified_ReturnsCulprits(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	env.RegisterWorkflowWithOptions(internal.PairwiseWorkflow, workflow.RegisterOptions{Name: workflows.PairwiseWorkflow})
	env.RegisterWorkflowWithOptions(CatapultBisectWorkflow, workflow.RegisterOptions{Name: workflows.CatapultBisect})

	fakeCulprits := []*pinpoint_proto.CombinedCommit{
		{Main: common.NewChromiumCommit("commit2")},
		{Main: common.NewChromiumCommit("commit4")},
		{Main: common.NewChromiumCommit("commit6")},
	}

	fakeCulpritPairs := []*pinpoint_proto.Culprit{
		{
			Prior:   &pinpoint_proto.CombinedCommit{Main: common.NewChromiumCommit("commit1")},
			Culprit: &pinpoint_proto.CombinedCommit{Main: common.NewChromiumCommit("commit2")},
		},
		{
			Prior:   &pinpoint_proto.CombinedCommit{Main: common.NewChromiumCommit("commit3")},
			Culprit: &pinpoint_proto.CombinedCommit{Main: common.NewChromiumCommit("commit4")},
		},
		{
			Prior:   &pinpoint_proto.CombinedCommit{Main: common.NewChromiumCommit("commit5")},
			Culprit: &pinpoint_proto.CombinedCommit{Main: common.NewChromiumCommit("commit6")},
		},
	}

	rc := make(chan *pinpoint_proto.PairwiseExecution, len(fakeCulprits))
	pairwiseExecutions := make([]*pinpoint_proto.PairwiseExecution, len(fakeCulprits))
	pairwiseExecutions[0] = &pinpoint_proto.PairwiseExecution{}
	rc <- &pinpoint_proto.PairwiseExecution{}
	for i := 1; i < len(fakeCulprits); i++ {
		pairwiseExecutions[i] = &pinpoint_proto.PairwiseExecution{
			Culprit: fakeCulprits[i],
		}
		rc <- &pinpoint_proto.PairwiseExecution{
			Culprit: fakeCulprits[i],
		}
	}
	env.OnWorkflow(workflows.PairwiseWorkflow, mock.Anything, mock.Anything).Return(
		&pinpoint_proto.PairwiseExecution{
			Significant: true,
			Statistic: &pinpoint_proto.PairwiseExecution_WilcoxonResult{
				ControlMedian:   0.1, // arbitrary values
				TreatmentMedian: 0.2,
			},
		}, nil).Once()
	env.OnWorkflow(workflows.CatapultBisect, mock.Anything, mock.Anything).Return(&pinpoint_proto.BisectExecution{
		Culprits:         fakeCulprits,
		DetailedCulprits: fakeCulpritPairs,
	}, nil).Once()
	env.OnWorkflow(workflows.PairwiseWorkflow, mock.Anything, mock.Anything).Return(func(ctx workflow.Context, pp *workflows.PairwiseParams) (*pinpoint_proto.PairwiseExecution, error) {
		return <-rc, nil
	}).Times(len(fakeCulprits))

	env.ExecuteWorkflow(CulpritFinderWorkflow, generateCulpritFinderParams())
	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())

	var cfe *pinpoint_proto.CulpritFinderExecution
	require.NoError(t, env.GetWorkflowResult(&cfe))
	assert.NotNil(t, cfe)
	assert.True(t, cfe.RegressionVerified)
	assert.EqualValues(t, fakeCulprits[1:], cfe.Culprits)
	env.AssertExpectations(t)
}
