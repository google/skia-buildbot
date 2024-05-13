package swarmingv2

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	apipb "go.chromium.org/luci/swarming/proto/api_v2"
	"go.skia.org/infra/go/cipd"
	"go.skia.org/infra/go/common"
	"go.skia.org/infra/go/now"
	"go.skia.org/infra/go/swarming"
	"go.skia.org/infra/go/swarming/v2/mocks"
	"go.skia.org/infra/go/testutils"
	"go.skia.org/infra/task_scheduler/go/types"
	"google.golang.org/protobuf/types/known/timestamppb"
)

const (
	fakeDigest     = "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234/987"
	fakeDigestHash = "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234"
	fakeDigestSize = 987
)

func TestGetFreeMachines_CombinesPagedResponses(t *testing.T) {
	ctx := context.Background()
	client := &mocks.SwarmingV2Client{}
	s := &SwarmingV2TaskExecutor{
		casInstance: "fake-cas-instance",
		pubSubTopic: "fake-pubsub-topic",
		client:      client,
	}

	req1 := &apipb.BotsRequest{
		Limit: 1000,
		Dimensions: []*apipb.StringPair{
			{Key: "pool", Value: "fake-pool"},
		},
		IsBusy:        apipb.NullableBool_FALSE,
		IsDead:        apipb.NullableBool_FALSE,
		InMaintenance: apipb.NullableBool_FALSE,
		Quarantined:   apipb.NullableBool_FALSE,
	}
	client.On("ListBots", testutils.AnyContext, req1).Return(&apipb.BotInfoListResponse{
		Cursor: "cursor1",
		Items: []*apipb.BotInfo{
			{BotId: "1"},
			{BotId: "2"},
		},
	}, nil)
	req2 := &apipb.BotsRequest{
		Limit: 1000,
		Dimensions: []*apipb.StringPair{
			{Key: "pool", Value: "fake-pool"},
		},
		IsBusy:        apipb.NullableBool_FALSE,
		IsDead:        apipb.NullableBool_FALSE,
		InMaintenance: apipb.NullableBool_FALSE,
		Quarantined:   apipb.NullableBool_FALSE,
		Cursor:        "cursor1",
	}
	client.On("ListBots", testutils.AnyContext, req2).Return(&apipb.BotInfoListResponse{
		Items: []*apipb.BotInfo{
			{BotId: "3"},
			{BotId: "4"},
		},
	}, nil)

	machines, err := s.GetFreeMachines(ctx, "fake-pool")
	require.NoError(t, err)
	require.Equal(t, []*types.Machine{
		{ID: "1", Dimensions: []string{}},
		{ID: "2", Dimensions: []string{}},
		{ID: "3", Dimensions: []string{}},
		{ID: "4", Dimensions: []string{}},
	}, machines)
}

func TestGetPendingTasks_CombinesPagedResponses(t *testing.T) {
	ts := time.Unix(1715176877, 0) // Arbitrary time.
	ctx := now.TimeTravelingContext(ts)
	client := &mocks.SwarmingV2Client{}
	s := &SwarmingV2TaskExecutor{
		casInstance: "fake-cas-instance",
		pubSubTopic: "fake-pubsub-topic",
		client:      client,
	}

	req1 := &apipb.TasksWithPerfRequest{
		Limit:                   1000,
		Start:                   timestamppb.New(ts.Add(-2 * 24 * time.Hour)),
		End:                     timestamppb.New(ts),
		State:                   apipb.StateQuery_QUERY_PENDING,
		Tags:                    []string{"pool:fake-pool"},
		IncludePerformanceStats: false,
	}
	client.On("ListTasks", testutils.AnyContext, req1).Return(&apipb.TaskListResponse{
		Cursor: "cursor1",
		Items: []*apipb.TaskResultResponse{
			{TaskId: "1", State: apipb.TaskState_PENDING},
			{TaskId: "2", State: apipb.TaskState_PENDING},
		},
	}, nil)
	req2 := &apipb.TasksWithPerfRequest{
		Limit:                   1000,
		Start:                   timestamppb.New(ts.Add(-2 * 24 * time.Hour)),
		End:                     timestamppb.New(ts),
		State:                   apipb.StateQuery_QUERY_PENDING,
		Tags:                    []string{"pool:fake-pool"},
		IncludePerformanceStats: false,
		Cursor:                  "cursor1",
	}
	client.On("ListTasks", testutils.AnyContext, req2).Return(&apipb.TaskListResponse{
		Items: []*apipb.TaskResultResponse{
			{TaskId: "3", State: apipb.TaskState_PENDING},
			{TaskId: "4", State: apipb.TaskState_PENDING},
		},
	}, nil)

	tasks, err := s.GetPendingTasks(ctx, "fake-pool")
	require.NoError(t, err)
	require.Equal(t, []*types.TaskResult{
		{ID: "1", Status: types.TASK_STATUS_PENDING, Tags: map[string][]string{}},
		{ID: "2", Status: types.TASK_STATUS_PENDING, Tags: map[string][]string{}},
		{ID: "3", Status: types.TASK_STATUS_PENDING, Tags: map[string][]string{}},
		{ID: "4", Status: types.TASK_STATUS_PENDING, Tags: map[string][]string{}},
	}, tasks)
}

func TestGetTaskResult(t *testing.T) {
	ctx := context.Background()
	client := &mocks.SwarmingV2Client{}
	s := &SwarmingV2TaskExecutor{
		casInstance: "fake-cas-instance",
		pubSubTopic: "fake-pubsub-topic",
		client:      client,
	}
	client.On("GetResult", testutils.AnyContext, &apipb.TaskIdWithPerfRequest{
		TaskId:                  "task-id",
		IncludePerformanceStats: false,
	}).Return(&apipb.TaskResultResponse{
		TaskId: "task-id",
		State:  apipb.TaskState_COMPLETED,
	}, nil)

	task, err := s.GetTaskResult(ctx, "task-id")
	require.NoError(t, err)
	require.Equal(t, &types.TaskResult{
		ID:     "task-id",
		Status: types.TASK_STATUS_SUCCESS,
		Tags:   map[string][]string{},
	}, task)
}

func TestGetTaskCompletionStatuses(t *testing.T) {
	ctx := context.Background()
	client := &mocks.SwarmingV2Client{}
	s := &SwarmingV2TaskExecutor{
		casInstance: "fake-cas-instance",
		pubSubTopic: "fake-pubsub-topic",
		client:      client,
	}

	var taskIds []string
	var taskStates []apipb.TaskState
	var expectResult []bool
	addTask := func(id string, state apipb.TaskState, isFinished bool) {
		taskIds = append(taskIds, id)
		taskStates = append(taskStates, state)
		expectResult = append(expectResult, isFinished)
	}

	addTask("1", apipb.TaskState_BOT_DIED, true)
	addTask("2", apipb.TaskState_CANCELED, true)
	addTask("3", apipb.TaskState_CLIENT_ERROR, true)
	addTask("4", apipb.TaskState_COMPLETED, true)
	addTask("5", apipb.TaskState_EXPIRED, true)
	addTask("6", apipb.TaskState_INVALID, true)
	addTask("7", apipb.TaskState_KILLED, true)
	addTask("8", apipb.TaskState_NO_RESOURCE, true)
	addTask("9", apipb.TaskState_TIMED_OUT, true)
	addTask("10", apipb.TaskState_PENDING, false)
	addTask("11", apipb.TaskState_RUNNING, false)

	client.On("ListTaskStates", testutils.AnyContext, &apipb.TaskStatesRequest{
		TaskId: taskIds,
	}).Return(&apipb.TaskStates{
		States: taskStates,
	}, nil)

	actual, err := s.GetTaskCompletionStatuses(ctx, taskIds)
	require.NoError(t, err)
	require.Len(t, actual, len(expectResult))
	for idx, expect := range expectResult {
		require.Equal(t, expect, actual[idx], taskStates[idx])
	}
}

func TestTriggerTask(t *testing.T) {
	ctx := context.Background()
	client := &mocks.SwarmingV2Client{}
	s := &SwarmingV2TaskExecutor{
		casInstance: "fake-cas-instance",
		pubSubTopic: "fake-pubsub-topic",
		client:      client,
	}
	expectRequest := &apipb.NewTaskRequest{
		Name:        "task-name",
		Priority:    swarming.RECOMMENDED_PRIORITY,
		PubsubTopic: fmt.Sprintf(swarming.PUBSUB_FULLY_QUALIFIED_TOPIC_TMPL, common.PROJECT_ID, s.pubSubTopic),
		TaskSlices: []*apipb.TaskSlice{
			{
				ExpirationSecs: int32(swarming.RECOMMENDED_EXPIRATION.Seconds()),
				Properties: &apipb.TaskProperties{
					CasInputRoot: &apipb.CASReference{
						CasInstance: "fake-cas-instance",
						Digest: &apipb.Digest{
							Hash:      "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
							SizeBytes: fakeDigestSize,
						},
					},
					Dimensions: []*apipb.StringPair{
						{Key: "os", Value: "Linux"},
					},
					ExecutionTimeoutSecs: int32(swarming.RECOMMENDED_HARD_TIMEOUT.Seconds()),
					IoTimeoutSecs:        int32(swarming.RECOMMENDED_IO_TIMEOUT.Seconds()),
				},
			},
		},
		User: swarmingUser,
	}
	client.On("NewTask", testutils.AnyContext, expectRequest).Return(&apipb.TaskRequestMetadataResponse{
		TaskId:  "task-id",
		Request: &apipb.TaskRequestResponse{},
	}, nil)

	res, err := s.TriggerTask(ctx, &types.TaskRequest{
		Name:       "task-name",
		CasInput:   fakeDigest,
		Dimensions: []string{"os:Linux"},
	})
	require.NoError(t, err)
	require.Equal(t, &types.TaskResult{
		ID:     "task-id",
		Status: types.TASK_STATUS_PENDING,
	}, res)
}

func TestTriggerTask_Minimal(t *testing.T) {
	ctx := context.Background()
	client := &mocks.SwarmingV2Client{}
	s := &SwarmingV2TaskExecutor{
		casInstance: "fake-cas-instance",
		pubSubTopic: "fake-pubsub-topic",
		client:      client,
	}
	expectRequest := &apipb.NewTaskRequest{
		Name:        "task-name",
		Priority:    swarming.RECOMMENDED_PRIORITY,
		PubsubTopic: fmt.Sprintf(swarming.PUBSUB_FULLY_QUALIFIED_TOPIC_TMPL, common.PROJECT_ID, s.pubSubTopic),
		TaskSlices: []*apipb.TaskSlice{
			{
				ExpirationSecs: int32(swarming.RECOMMENDED_EXPIRATION.Seconds()),
				Properties: &apipb.TaskProperties{
					CasInputRoot: &apipb.CASReference{
						CasInstance: "fake-cas-instance",
						Digest: &apipb.Digest{
							Hash:      "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
							SizeBytes: fakeDigestSize,
						},
					},
					Dimensions: []*apipb.StringPair{
						{Key: "os", Value: "Linux"},
					},
					ExecutionTimeoutSecs: int32(swarming.RECOMMENDED_HARD_TIMEOUT.Seconds()),
					IoTimeoutSecs:        int32(swarming.RECOMMENDED_IO_TIMEOUT.Seconds()),
				},
			},
		},
		User: swarmingUser,
	}
	client.On("NewTask", testutils.AnyContext, expectRequest).Return(&apipb.TaskRequestMetadataResponse{
		TaskId:  "task-id",
		Request: &apipb.TaskRequestResponse{},
	}, nil)

	res, err := s.TriggerTask(ctx, &types.TaskRequest{
		Name:       "task-name",
		CasInput:   fakeDigest,
		Dimensions: []string{"os:Linux"},
	})
	require.NoError(t, err)
	require.Equal(t, &types.TaskResult{
		ID:     "task-id",
		Status: types.TASK_STATUS_PENDING,
	}, res)
}

func TestConvertTaskRequest_Minimal(t *testing.T) {
	s := &SwarmingV2TaskExecutor{
		casInstance: "fake-cas-instance",
		pubSubTopic: "fake-pubsub-topic",
		client:      nil, // Unused in this test.
	}
	input := &types.TaskRequest{
		CasInput: fakeDigest,
	}
	expect := &apipb.NewTaskRequest{
		Priority: swarming.RECOMMENDED_PRIORITY,
		TaskSlices: []*apipb.TaskSlice{
			{
				ExpirationSecs: int32(swarming.RECOMMENDED_EXPIRATION.Seconds()),
				Properties: &apipb.TaskProperties{
					CasInputRoot: &apipb.CASReference{
						CasInstance: s.casInstance,
						Digest: &apipb.Digest{
							Hash:      "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
							SizeBytes: fakeDigestSize,
						},
					},
					ExecutionTimeoutSecs: int32(swarming.RECOMMENDED_HARD_TIMEOUT.Seconds()),
					IoTimeoutSecs:        int32(swarming.RECOMMENDED_IO_TIMEOUT.Seconds()),
				},
			},
		},
		User:        swarmingUser,
		PubsubTopic: fmt.Sprintf(swarming.PUBSUB_FULLY_QUALIFIED_TOPIC_TMPL, common.PROJECT_ID, s.pubSubTopic),
	}
	actual, err := s.convertTaskRequest(input)
	require.NoError(t, err)
	require.Equal(t, expect, actual)
}

func TestConvertTaskRequest_Full(t *testing.T) {
	s := &SwarmingV2TaskExecutor{
		casInstance: "fake-cas-instance",
		pubSubTopic: "fake-pubsub-topic",
		client:      nil, // Unused in this test.
	}
	input := &types.TaskRequest{
		Caches: []*types.CacheRequest{
			{
				Name: "go",
				Path: "/cache/go",
			},
		},
		CasInput: fakeDigest,
		CipdPackages: []*cipd.Package{
			{
				Name:    "my-pkg",
				Path:    "/path/to/my-pkg",
				Version: "version:1",
			},
		},
		Command:    []string{"echo", "hello world"},
		Dimensions: []string{"os:Linux"},
		Env: map[string]string{
			"KEY1": "VALUE",
		},
		EnvPrefixes: map[string][]string{
			"KEY2": {"VALUE1", "VALUE2"},
		},
		ExecutionTimeout:    5 * time.Minute,
		Expiration:          10 * time.Minute,
		Idempotent:          true,
		IoTimeout:           2 * time.Minute,
		Name:                "task-name",
		Outputs:             []string{"output"},
		ServiceAccount:      "my-service-account",
		Tags:                []string{"task-tag:123"},
		TaskSchedulerTaskID: "ts-task-id",
	}
	expect := &apipb.NewTaskRequest{
		Name:     "task-name",
		Priority: swarming.RECOMMENDED_PRIORITY,
		TaskSlices: []*apipb.TaskSlice{
			{
				ExpirationSecs: int32((10 * time.Minute).Seconds()),
				Properties: &apipb.TaskProperties{
					Caches: []*apipb.CacheEntry{
						{
							Name: "go",
							Path: "/cache/go",
						},
					},
					CipdInput: &apipb.CipdInput{
						Packages: []*apipb.CipdPackage{
							{
								PackageName: "my-pkg",
								Version:     "version:1",
								Path:        "/path/to/my-pkg",
							},
						},
					},
					Command: []string{"echo", "hello world"},
					Dimensions: []*apipb.StringPair{
						{Key: "os", Value: "Linux"},
					},
					Env: []*apipb.StringPair{
						{Key: "KEY1", Value: "VALUE"},
					},
					EnvPrefixes: []*apipb.StringListPair{
						{Key: "KEY2", Value: []string{"VALUE1", "VALUE2"}},
					},
					ExecutionTimeoutSecs: int32((5 * time.Minute).Seconds()),
					Idempotent:           true,
					CasInputRoot: &apipb.CASReference{
						CasInstance: s.casInstance,
						Digest: &apipb.Digest{
							Hash:      "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
							SizeBytes: fakeDigestSize,
						},
					},
					IoTimeoutSecs: int32((2 * time.Minute).Seconds()),
					Outputs:       []string{"output"},
				},
			},
		},
		Tags:           []string{"task-tag:123"},
		User:           swarmingUser,
		ServiceAccount: "my-service-account",
		PubsubTopic:    fmt.Sprintf(swarming.PUBSUB_FULLY_QUALIFIED_TOPIC_TMPL, common.PROJECT_ID, s.pubSubTopic),
		PubsubUserdata: "ts-task-id",
	}
	actual, err := s.convertTaskRequest(input)
	require.NoError(t, err)
	require.Equal(t, expect, actual)
}

func TestConvertTaskResult_Minimal(t *testing.T) {
	input := &apipb.TaskResultResponse{}
	expect := &types.TaskResult{
		Status: types.TASK_STATUS_MISHAP,
		Tags:   map[string][]string{},
	}
	actual, err := convertTaskResult(input)
	require.NoError(t, err)
	require.Equal(t, expect, actual)
}

func TestConvertTaskResult_Full(t *testing.T) {
	created := time.Unix(1715190575, 0).UTC()
	started := created.Add(time.Minute)
	finished := started.Add(time.Minute)
	input := &apipb.TaskResultResponse{
		TaskId: "task-id",
		BotDimensions: []*apipb.StringListPair{
			{Key: "os", Value: []string{"Linux", "Debian"}},
		},
		BotId:       "bot-id",
		CompletedTs: timestamppb.New(finished),
		CreatedTs:   timestamppb.New(created),
		Duration:    60.0,
		ExitCode:    1,
		Failure:     true,
		CasOutputRoot: &apipb.CASReference{
			CasInstance: "fake-cas-instance",
			Digest: &apipb.Digest{
				Hash:      "abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234abcd1234",
				SizeBytes: fakeDigestSize,
			},
		},
		StartedTs: timestamppb.New(started),
		State:     apipb.TaskState_COMPLETED,
		Name:      "task-name",
		Tags:      []string{"key:value1", "key:value2"},
	}
	expect := &types.TaskResult{
		CasOutput: fakeDigest,
		Created:   created,
		Finished:  finished,
		ID:        "task-id",
		MachineID: "bot-id",
		Started:   started,
		Status:    types.TASK_STATUS_FAILURE,
		Tags: map[string][]string{
			"key": {"value1", "value2"},
		},
	}
	actual, err := convertTaskResult(input)
	require.NoError(t, err)
	require.Equal(t, expect, actual)
}

func TestConvertTaskStatus_CombinesWithFailureToProduceTaskStatus(t *testing.T) {
	check := func(state apipb.TaskState, failure bool, expect types.TaskStatus) {
		result, err := convertTaskStatus(state, failure)
		require.NoError(t, err)
		require.Equal(t, expect, result)
	}

	check(apipb.TaskState_BOT_DIED, false, types.TASK_STATUS_MISHAP)
	check(apipb.TaskState_BOT_DIED, true, types.TASK_STATUS_MISHAP)
	check(apipb.TaskState_CANCELED, false, types.TASK_STATUS_MISHAP)
	check(apipb.TaskState_CANCELED, true, types.TASK_STATUS_MISHAP)
	check(apipb.TaskState_CLIENT_ERROR, false, types.TASK_STATUS_MISHAP)
	check(apipb.TaskState_CLIENT_ERROR, true, types.TASK_STATUS_MISHAP)
	check(apipb.TaskState_EXPIRED, false, types.TASK_STATUS_MISHAP)
	check(apipb.TaskState_EXPIRED, true, types.TASK_STATUS_MISHAP)
	check(apipb.TaskState_NO_RESOURCE, false, types.TASK_STATUS_MISHAP)
	check(apipb.TaskState_NO_RESOURCE, true, types.TASK_STATUS_MISHAP)
	check(apipb.TaskState_TIMED_OUT, false, types.TASK_STATUS_MISHAP)
	check(apipb.TaskState_TIMED_OUT, true, types.TASK_STATUS_MISHAP)
	check(apipb.TaskState_KILLED, false, types.TASK_STATUS_MISHAP)
	check(apipb.TaskState_KILLED, true, types.TASK_STATUS_MISHAP)
	check(apipb.TaskState_INVALID, false, types.TASK_STATUS_MISHAP)
	check(apipb.TaskState_INVALID, true, types.TASK_STATUS_MISHAP)
	check(apipb.TaskState_PENDING, false, types.TASK_STATUS_PENDING)
	check(apipb.TaskState_PENDING, true, types.TASK_STATUS_PENDING)
	check(apipb.TaskState_RUNNING, false, types.TASK_STATUS_RUNNING)
	check(apipb.TaskState_RUNNING, true, types.TASK_STATUS_RUNNING)
	check(apipb.TaskState_COMPLETED, false, types.TASK_STATUS_SUCCESS)
	check(apipb.TaskState_COMPLETED, true, types.TASK_STATUS_FAILURE)
}

func TestConvertMachine_Minimal(t *testing.T) {
	bot := &apipb.BotInfo{}
	machine := &types.Machine{
		Dimensions: []string{},
	}
	require.Equal(t, machine, convertMachine(bot))
}

func TestConvertMachine_Full(t *testing.T) {
	bot := &apipb.BotInfo{
		BotId: "bot-id",
		Dimensions: []*apipb.StringListPair{
			{
				Key:   "os",
				Value: []string{"Linux"},
			},
			// Ensure that we handle multiple entries with the same key.
			{
				Key:   "os",
				Value: []string{"Debian", "Debian-13"},
			},
			{
				Key:   "gpu",
				Value: []string{"none"},
			},
		},
		IsDead:      true,
		Quarantined: true,
		TaskId:      "task-id",
	}
	machine := &types.Machine{
		ID: "bot-id",
		Dimensions: []string{
			"gpu:none",
			"os:Debian",
			"os:Debian-13",
			"os:Linux",
		},
		IsDead:        true,
		IsQuarantined: true,
		CurrentTaskID: "task-id",
	}
	require.Equal(t, machine, convertMachine(bot))
}
