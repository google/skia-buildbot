package backends

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"go.skia.org/infra/go/swarming/mocks"

	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
)

func TestNewSwarmingClient_Default_SwarmingClient(t *testing.T) {
	ctx := context.Background()
	sc, err := NewSwarmingClient(ctx, DefaultSwarmingServiceAddress)
	assert.NoError(t, err)
	assert.NotNil(t, sc)
}

func TestListPinpointTasks_ValidInput_TasksFound(t *testing.T) {
	ctx := context.Background()
	mockClient := mocks.NewApiClient(t)

	bA := &swarmingV1.SwarmingRpcsCASReference{
		CasInstance: "instance",
		Digest: &swarmingV1.SwarmingRpcsDigest{
			Hash:      "hash",
			SizeBytes: 0,
		},
	}

	mockClient.On("ListTasks", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]*swarmingV1.SwarmingRpcsTaskRequestMetadata{
			{
				TaskId: "123",
			},
			{
				TaskId: "456",
			},
		}, nil).Once()

	sc := &SwarmingClientImpl{
		ApiClient: mockClient,
	}
	taskIds, err := sc.ListPinpointTasks(ctx, "id", bA)
	assert.NoError(t, err)
	assert.Equal(t, []string{"123", "456"}, taskIds)
}

func TestListPinpointTasks_ValidInput_NoTasksFound(t *testing.T) {
	ctx := context.Background()
	mockClient := mocks.NewApiClient(t)

	bA := &swarmingV1.SwarmingRpcsCASReference{
		CasInstance: "instance",
		Digest: &swarmingV1.SwarmingRpcsDigest{
			Hash:      "hash",
			SizeBytes: 0,
		},
	}

	mockClient.On("ListTasks", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return([]*swarmingV1.SwarmingRpcsTaskRequestMetadata{}, nil).Once()

	sc := &SwarmingClientImpl{
		ApiClient: mockClient,
	}
	taskIds, err := sc.ListPinpointTasks(ctx, "id", bA)

	assert.NoError(t, err)
	assert.Empty(t, taskIds)
}

func TestListPinpointTasks_InvalidInputs_Error(t *testing.T) {
	ctx := context.Background()
	mockClient := mocks.NewApiClient(t)
	sc := &SwarmingClientImpl{
		ApiClient: mockClient,
	}

	taskIds, err := sc.ListPinpointTasks(ctx, "", &swarmingV1.SwarmingRpcsCASReference{})
	assert.Nil(t, taskIds)
	assert.ErrorContains(t, err, "Cannot list tasks because request is missing JobID")

	taskIds, err = sc.ListPinpointTasks(ctx, "id", nil)
	assert.Nil(t, taskIds)
	assert.ErrorContains(t, err, "Cannot list tasks because request is missing cas isolate")
}

func TestGetCASOutput_ValidInput_SwarmingRBECasRef(t *testing.T) {
	ctx := context.Background()
	mockClient := mocks.NewApiClient(t)
	sc := &SwarmingClientImpl{
		ApiClient: mockClient,
	}

	mockClient.On("GetTask", ctx, mock.Anything, mock.Anything).
		Return(&swarmingV1.SwarmingRpcsTaskResult{
			State: "COMPLETED",
			CasOutputRoot: &swarmingV1.SwarmingRpcsCASReference{
				CasInstance: "instance",
				Digest: &swarmingV1.SwarmingRpcsDigest{
					Hash:      "hash",
					SizeBytes: 0,
				},
			},
		}, nil).Once()

	rbe, err := sc.GetCASOutput(ctx, "taskId")
	assert.NoError(t, err)
	assert.Equal(t, "instance", rbe.CasInstance)
	assert.Equal(t, "hash", rbe.Digest.Hash)
	assert.Equal(t, int64(0), rbe.Digest.SizeBytes)
}

func TestGasCASOutput_IncompleteTask_Error(t *testing.T) {
	ctx := context.Background()
	mockClient := mocks.NewApiClient(t)
	sc := &SwarmingClientImpl{
		ApiClient: mockClient,
	}

	mockClient.On("GetTask", ctx, mock.Anything, mock.Anything).
		Return(&swarmingV1.SwarmingRpcsTaskResult{
			State: "Not_Completed",
		}, nil).Once()

	rbe, err := sc.GetCASOutput(ctx, "taskId")
	assert.Nil(t, rbe)
	assert.ErrorContains(t, err, "cannot get result of task")
}
