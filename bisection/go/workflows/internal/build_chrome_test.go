package internal

import (
	"testing"

	"github.com/stretchr/testify/mock"
	"go.skia.org/infra/bisection/go/workflows"

	"github.com/stretchr/testify/require"
	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
	"go.temporal.io/sdk/testsuite"
)

func Test_BuildChrome_ShouldReturnCAS(t *testing.T) {
	testSuite := &testsuite.WorkflowTestSuite{}
	env := testSuite.NewTestWorkflowEnvironment()

	var bca *BuildChromeActivity
	buildID := int64(1234)
	cas := &swarmingV1.SwarmingRpcsCASReference{
		CasInstance: "fake-instance",
	}

	env.OnActivity(bca.SearchOrBuildActivity, mock.Anything, mock.Anything).Return(buildID, nil)
	env.OnActivity(bca.WaitBuildCompletionActivity, mock.Anything, mock.Anything).Return(true, nil)
	env.OnActivity(bca.RetrieveCASActivity, mock.Anything, mock.Anything, mock.Anything).Return(cas, nil)

	env.ExecuteWorkflow(BuildChrome, workflows.BuildChromeParams{})

	require.True(t, env.IsWorkflowCompleted())
	require.NoError(t, env.GetWorkflowError())
	var result *swarmingV1.SwarmingRpcsCASReference
	require.NoError(t, env.GetWorkflowResult(&result))
	require.Equal(t, cas, result)
}
