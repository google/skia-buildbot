package backends

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"

	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/swarming/mocks"

	swarmingV1 "go.chromium.org/luci/common/api/swarming/swarming/v1"
)

func TestNewSwarmingClient_Default_SwarmingClient(t *testing.T) {
	ctx := context.Background()
	sc, err := NewSwarmingClient(ctx, DefaultSwarmingServiceAddress)
	assert.NoError(t, err)
	assert.NotNil(t, sc)
}

func TestGetStatus_CompletedAndSuccess_ReturnsCompleted(t *testing.T) {
	ctx := context.Background()
	mockClient := mocks.NewApiClient(t)

	mockClient.On("GetTask", ctx, mock.Anything, mock.Anything).
		Return(&swarmingV1.SwarmingRpcsTaskResult{
			State:   swarming.TASK_STATE_COMPLETED,
			Failure: false,
		}, nil).Once()

	sc := &SwarmingClientImpl{
		ApiClient: mockClient,
	}
	status, err := sc.GetStatus(ctx, "task")
	require.NoError(t, err)
	assert.Equal(t, swarming.TASK_STATE_COMPLETED, status)
}

func TestGetStatus_CompletedAndFailure_ReturnsRunBenchmarkFailure(t *testing.T) {
	ctx := context.Background()
	mockClient := mocks.NewApiClient(t)

	mockClient.On("GetTask", ctx, mock.Anything, mock.Anything).
		Return(&swarmingV1.SwarmingRpcsTaskResult{
			State:   swarming.TASK_STATE_COMPLETED,
			Failure: true,
		}, nil).Once()

	sc := &SwarmingClientImpl{
		ApiClient: mockClient,
	}
	status, err := sc.GetStatus(ctx, "task")
	require.NoError(t, err)
	assert.Equal(t, RunBenchmarkFailure, status)
}

func TestGetStatus_NotCompleted_ReturnsState(t *testing.T) {
	ctx := context.Background()
	mockClient := mocks.NewApiClient(t)

	mockClient.On("GetTask", ctx, mock.Anything, mock.Anything).
		Return(&swarmingV1.SwarmingRpcsTaskResult{
			State:   swarming.TASK_STATE_BOT_DIED,
			Failure: true,
		}, nil).Once()

	sc := &SwarmingClientImpl{
		ApiClient: mockClient,
	}
	status, err := sc.GetStatus(ctx, "task")
	require.NoError(t, err)
	assert.Equal(t, swarming.TASK_STATE_BOT_DIED, status)
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

func TestFetchFreeBots_NoBuildConfig_ReturnsError(t *testing.T) {
	const fakeBuilder = "fake_builder"

	ctx := context.Background()
	mockClient := mocks.NewApiClient(t)
	sc := &SwarmingClientImpl{
		ApiClient: mockClient,
	}
	_, err := sc.FetchFreeBots(ctx, fakeBuilder)
	require.Error(t, err)
}

func TestFetchFreeBots_ForBuilder_ReturnsFreeBots(t *testing.T) {
	const builder = "android-pixel2-perf"

	ctx := context.Background()
	mockClient := mocks.NewApiClient(t)
	sc := &SwarmingClientImpl{
		ApiClient: mockClient,
	}

	mockClient.On("ListBotsForDimensions", ctx, mock.Anything).Return(
		func(_ context.Context, dims map[string]string) ([]*swarmingV1.SwarmingRpcsBotInfo, error) {
			assert.Len(t, dims, 3)
			// The expected dimensions asserted below are from bot_configs
			// For the builder in question, these dimensions will
			// be fetched from here: https://source.corp.google.com/h/skia/buildbot/+/main:pinpoint/go/bot_configs/external.json;l=23#:~:text=%22-,android,-%2Dpixel2%2Dperf%22
			assert.Equal(t, "chrome.tests.pinpoint", dims["pool"])
			assert.Equal(t, "walleye", dims["device_type"])
			assert.Equal(t, "OPM1.171019.021", dims["device_os"])

			return []*swarmingV1.SwarmingRpcsBotInfo{{BotId: "b1"}, {BotId: "b2"}}, nil
		},
	)

	resp, err := sc.FetchFreeBots(ctx, builder)
	require.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, "b1", resp[0].BotId)
	assert.Equal(t, "b2", resp[1].BotId)
}
