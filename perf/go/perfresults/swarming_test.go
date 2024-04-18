package perfresults

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	swarmingv2 "go.chromium.org/luci/swarming/proto/api_v2"
)

func makeCAS(hash string, size int64) *swarmingv2.CASReference {
	return &swarmingv2.CASReference{
		Digest: &swarmingv2.Digest{
			Hash:      hash,
			SizeBytes: size,
		},
		CasInstance: "projects/chrome-swarming/instances/default_instance",
	}
}

func Test_SwarmingClient_FindChildTaskIds_ReturnsChildIds(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	httpc := setupReplay(t, "SwarmingClient_FindChildTaskIds_ReturnsChildIds.json")

	expect := func(instance, parentID string, count int, childIDs ...string) {
		sc, err := newSwarmingClient(ctx, instance+".appspot.com", httpc)
		assert.NoError(t, err)

		ids, err := sc.findChildTaskIds(ctx, parentID)
		assert.NoError(t, err)
		assert.Len(t, ids, count)
		assert.Subset(t, ids, childIDs)
	}

	// taskId and runId are diff'd by 1 and should return the same child Ids
	// from: https://chrome-swarming.appspot.com/task?id=68f6bf166ca58810
	expect("chrome-swarming", "68f6bf166ca58810", 15, "68f6c580c2e5d710", "68f6c578b8c15110", "68f6c5778932e210")
	expect("chrome-swarming", "68f6bf166ca58811", 15, "68f6c580c2e5d710", "68f6c578b8c15110", "68f6c5778932e210")

	// from: https://chrome-swarming.appspot.com/task?id=68f70a9ed3e8b610
	expect("chrome-swarming", "68f70a9ed3e8b610", 4, "68f70cd5327c9410")

	// no child Id shouldn't panic
	expect("chrome-swarming", "68f6c580c2e5d710", 0)
	expect("chromium-swarm", "68f751b0f38b3810", 0)
}

func Test_SwarmingClient_FindChildTaskIds_ErrorOnNonExisting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	httpc := setupReplay(t, "SwarmingClient_FindChildTaskIds_ErrorOnNonExisting.json")
	sc, err := newSwarmingClient(ctx, "chrome-swarming.appspot.com", httpc)
	assert.NoError(t, err)

	_, err = sc.findChildTaskIds(ctx, "not-existing-task-id")
	require.ErrorContains(t, err, "unable to get parent task details")
}

func Test_SwarmingClient_FindTaskCASOutputs_ReturnCAS(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	httpc := setupReplay(t, "SwarmingClient_FindTaskCASOutputs_ReturnCAS.json")
	sc, err := newSwarmingClient(ctx, "chrome-swarming.appspot.com", httpc)
	assert.NoError(t, err)

	expect := func(taskIDs []string, expected ...*swarmingv2.CASReference) {
		cas, err := sc.findTaskCASOutputs(ctx, taskIDs...)
		assert.NoError(t, err)
		assert.EqualValues(t, cas, expected)
	}

	// CAS Output from: https://chrome-swarming.appspot.com/task?id=68f6c580c2e5d711
	cas752 := makeCAS("d127f8323a5016001b6d44bdc784a41aacb982909f721878589258d3dfc30616", 752)

	// CAS Output from: https://chrome-swarming.appspot.com/task?id=68fc991978265810
	cas730 := makeCAS("33680ccf87dfb334f0f2296efff6a7d10d78f757ed0b6aec88026fc9fd76713a", 730)

	// runId and taskId diff'd by 1 but points to the same CAS.
	expect([]string{"68f6c580c2e5d710"}, cas752)
	expect([]string{"68f6c580c2e5d711"}, cas752)

	expect([]string{"68fc991978265810"}, cas730)

	expect([]string{"68fc991978265810", "68f6c580c2e5d711"}, cas730, cas752)

	// some task like builder task doesn't have CAS output
	expect([]string{"68f6bf166ca58810"}, nil)
	expect([]string{"68f6bf166ca58810", "68fc991978265810"}, nil, cas730)
}

func Test_SwarmingClient_FindTaskCASOutputs_ErrorOnNonExisting(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	httpc := setupReplay(t, "SwarmingClient_FindTaskCASOutputs_ErrorOnNonExisting.json")
	sc, err := newSwarmingClient(ctx, "chrome-swarming.appspot.com", httpc)
	assert.NoError(t, err)

	check_error := func(taskIds ...string) {
		_, err := sc.findTaskCASOutputs(ctx, taskIds...)
		assert.ErrorContains(t, err, "unable to get the cas output")
	}

	check_error("non-existing-task")
	check_error("68f6c578b8c15110", "non-existing-task")
}
