package backends

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/timestamppb"

	"go.skia.org/infra/go/swarming/v2/mocks"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
)

func TestNewSwarmingClient_Default_SwarmingClient(t *testing.T) {
	ctx := context.Background()
	sc, err := NewSwarmingClient(ctx, DefaultSwarmingServiceAddress)
	assert.NoError(t, err)
	assert.NotNil(t, sc)
}

func TestGetStatus_CompletedAndSuccess_ReturnsCompleted(t *testing.T) {
	ctx := context.Background()
	mockClient := &mocks.SwarmingV2Client{}

	mockClient.On("GetResult", ctx, mock.Anything, mock.Anything).
		Return(&apipb.TaskResultResponse{
			State:   apipb.TaskState_COMPLETED,
			Failure: false,
		}, nil).Once()

	sc := &SwarmingClientImpl{
		SwarmingV2Client: mockClient,
	}
	status, err := sc.GetStatus(ctx, "task")
	require.NoError(t, err)
	assert.Equal(t, apipb.TaskState_COMPLETED.String(), status)
}

func TestGetStatus_CompletedAndFailure_ReturnsRunBenchmarkFailure(t *testing.T) {
	ctx := context.Background()
	mockClient := &mocks.SwarmingV2Client{}

	mockClient.On("GetResult", ctx, mock.Anything, mock.Anything).
		Return(&apipb.TaskResultResponse{
			State:   apipb.TaskState_COMPLETED,
			Failure: true,
		}, nil).Once()

	sc := &SwarmingClientImpl{
		SwarmingV2Client: mockClient,
	}
	status, err := sc.GetStatus(ctx, "task")
	require.NoError(t, err)
	assert.Equal(t, RunBenchmarkFailure, status)
}

func TestGetStatus_NotCompleted_ReturnsState(t *testing.T) {
	ctx := context.Background()
	mockClient := &mocks.SwarmingV2Client{}

	mockClient.On("GetResult", ctx, mock.Anything, mock.Anything).
		Return(&apipb.TaskResultResponse{
			State:   apipb.TaskState_BOT_DIED,
			Failure: true,
		}, nil).Once()

	sc := &SwarmingClientImpl{
		SwarmingV2Client: mockClient,
	}
	status, err := sc.GetStatus(ctx, "task")
	require.NoError(t, err)
	assert.Equal(t, apipb.TaskState_BOT_DIED.String(), status)
}

func TestListPinpointTasks_ValidInput_TasksFound(t *testing.T) {
	ctx := context.Background()
	mockClient := &mocks.SwarmingV2Client{}

	bA := &apipb.CASReference{
		CasInstance: "instance",
		Digest: &apipb.Digest{
			Hash:      "hash",
			SizeBytes: 0,
		},
	}

	mockClient.On("ListTasks", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&apipb.TaskListResponse{
			Items: []*apipb.TaskResultResponse{
				{
					TaskId: "123",
				},
				{
					TaskId: "456",
				},
			},
		}, nil).Once()

	sc := &SwarmingClientImpl{
		SwarmingV2Client: mockClient,
	}
	taskIds, err := sc.ListPinpointTasks(ctx, "id", bA)
	assert.NoError(t, err)
	assert.Equal(t, []string{"123", "456"}, taskIds)
}

func TestListPinpointTasks_ValidInput_NoTasksFound(t *testing.T) {
	ctx := context.Background()
	mockClient := &mocks.SwarmingV2Client{}

	bA := &apipb.CASReference{
		CasInstance: "instance",
		Digest: &apipb.Digest{
			Hash:      "hash",
			SizeBytes: 0,
		},
	}

	mockClient.On("ListTasks", ctx, mock.Anything, mock.Anything, mock.Anything, mock.Anything).
		Return(&apipb.TaskListResponse{}, nil).Once()

	sc := &SwarmingClientImpl{
		SwarmingV2Client: mockClient,
	}
	taskIds, err := sc.ListPinpointTasks(ctx, "id", bA)

	assert.NoError(t, err)
	assert.Empty(t, taskIds)
}

func TestListPinpointTasks_InvalidInputs_Error(t *testing.T) {
	ctx := context.Background()
	mockClient := &mocks.SwarmingV2Client{}
	sc := &SwarmingClientImpl{
		SwarmingV2Client: mockClient,
	}

	taskIds, err := sc.ListPinpointTasks(ctx, "", &apipb.CASReference{})
	assert.Nil(t, taskIds)
	assert.ErrorContains(t, err, "Cannot list tasks because request is missing JobID")

	taskIds, err = sc.ListPinpointTasks(ctx, "id", nil)
	assert.Nil(t, taskIds)
	assert.ErrorContains(t, err, "Cannot list tasks because request is missing cas isolate")
}

func TestGetCASOutput_ValidInput_SwarmingRBECasRef(t *testing.T) {
	ctx := context.Background()
	mockClient := &mocks.SwarmingV2Client{}
	sc := &SwarmingClientImpl{
		SwarmingV2Client: mockClient,
	}

	mockClient.On("GetResult", ctx, mock.Anything, mock.Anything).
		Return(&apipb.TaskResultResponse{
			State: apipb.TaskState_COMPLETED,
			CasOutputRoot: &apipb.CASReference{
				CasInstance: "instance",
				Digest: &apipb.Digest{
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
	mockClient := &mocks.SwarmingV2Client{}
	sc := &SwarmingClientImpl{
		SwarmingV2Client: mockClient,
	}

	mockClient.On("GetResult", ctx, mock.Anything, mock.Anything).
		Return(&apipb.TaskResultResponse{
			State: apipb.TaskState_RUNNING,
		}, nil).Once()

	rbe, err := sc.GetCASOutput(ctx, "taskId")
	assert.Nil(t, rbe)
	assert.ErrorContains(t, err, "cannot get result of task")
}

func TestFetchFreeBots_NoBuildConfig_ReturnsError(t *testing.T) {
	const fakeBuilder = "fake_builder"

	ctx := context.Background()
	mockClient := &mocks.SwarmingV2Client{}
	sc := &SwarmingClientImpl{
		SwarmingV2Client: mockClient,
	}
	_, err := sc.FetchFreeBots(ctx, fakeBuilder)
	require.Error(t, err)
}

func TestFetchFreeBots_ForBuilder_ReturnsFreeBots(t *testing.T) {
	const builder = "android-pixel2-perf"

	ctx := context.Background()
	mockClient := &mocks.SwarmingV2Client{}
	sc := &SwarmingClientImpl{
		SwarmingV2Client: mockClient,
	}

	mockClient.On("ListBots", ctx, &apipb.BotsRequest{
		// The expected dimensions asserted below are from bot_configs
		// For the builder in question, these dimensions will
		// be fetched from here: https://source.corp.google.com/h/skia/buildbot/+/main:pinpoint/go/bot_configs/external.json;l=23#:~:text=%22-,android,-%2Dpixel2%2Dperf%22
		Dimensions: []*apipb.StringPair{
			{Key: "pool", Value: "chrome.tests.pinpoint"},
			{Key: "device_type", Value: "walleye"},
			{Key: "device_os", Value: "OPM1.171019.021"},
		},
		Limit: 1000,
	}).Return(&apipb.BotInfoListResponse{
		Items: []*apipb.BotInfo{
			{BotId: "b1"},
			{BotId: "b2"},
		},
	}, nil)

	resp, err := sc.FetchFreeBots(ctx, builder)
	require.NoError(t, err)
	assert.Len(t, resp, 2)
	assert.Equal(t, "b1", resp[0].BotId)
	assert.Equal(t, "b2", resp[1].BotId)
}

func TestGetBotTasksBetweenTwoTasks_GivenValidInputs_ReturnsTasks(t *testing.T) {
	const botID = "build132-a5"                                                 // arbitrary bot ID
	const task1, task2 = "6a7cbc697c99ed10", "6a7cbc8a22246410"                 // arbitrary task IDs
	const interruption1, interruption2 = "6a7cbc697c66ed10", "6a7cbc697c77ed10" // arbitrary task IDs

	test := func(name string, mockResp *apipb.TaskListResponse) {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			mockClient := &mocks.SwarmingV2Client{}
			sc := &SwarmingClientImpl{
				SwarmingV2Client: mockClient,
			}
			t1 := &apipb.TaskResultResponse{StartedTs: &timestamppb.Timestamp{Seconds: 1}}
			t2 := &apipb.TaskResultResponse{StartedTs: &timestamppb.Timestamp{Seconds: 2}}

			mockClient.On("GetResult", ctx, &apipb.TaskIdWithPerfRequest{TaskId: task1}).Return(t1, nil)
			mockClient.On("GetResult", ctx, &apipb.TaskIdWithPerfRequest{TaskId: task2}).Return(t2, nil)

			mockBotReq := &apipb.BotTasksRequest{
				BotId: botID,
				State: apipb.StateQuery_QUERY_ALL,
				Start: t1.StartedTs,
				End:   t2.StartedTs,
				Sort:  apipb.SortQuery_QUERY_STARTED_TS,
			}
			mockClient.On("ListBotTasks", ctx, mockBotReq).Return(mockResp, nil)

			resp, err := sc.GetBotTasksBetweenTwoTasks(ctx, botID, task1, task2)
			assert.NoError(t, err)
			assert.Equal(t, mockResp, resp)
		})
	}

	mockResp := &apipb.TaskListResponse{
		Items: []*apipb.TaskResultResponse{
			{TaskId: task1},
			{TaskId: task2},
		},
	}
	test("two task return", mockResp)

	mockResp = &apipb.TaskListResponse{
		Items: []*apipb.TaskResultResponse{
			{TaskId: task1},
			{TaskId: interruption1},
			{TaskId: interruption2},
			{TaskId: task2},
		},
	}
	test("more than two tasks return (implies task interception)", mockResp)
}

func TestGetBotTasksBetweenTwoTasks_GivenBadTask_ReturnsError(t *testing.T) {
	const botID = "build132-a5"                                 // arbitrary bot ID
	const task1, task2 = "6a7cbc697c99ed10", "6a7cbc8a22246410" // arbitrary task IDs
	ctx := context.Background()

	mockClient := &mocks.SwarmingV2Client{}
	sc := &SwarmingClientImpl{
		SwarmingV2Client: mockClient,
	}
	mockClient.On("GetResult", ctx, &apipb.TaskIdWithPerfRequest{TaskId: task1}).Return(nil, fmt.Errorf("some error"))

	resp, err := sc.GetBotTasksBetweenTwoTasks(ctx, botID, task1, task2)
	assert.Error(t, err)
	assert.Nil(t, resp)
}
