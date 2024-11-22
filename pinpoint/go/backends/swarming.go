package backends

import (
	"context"

	"go.skia.org/infra/go/auth"
	"go.skia.org/infra/go/httputils"
	"go.skia.org/infra/go/skerr"
	swarmingv2 "go.skia.org/infra/go/swarming/v2"
	"go.skia.org/infra/pinpoint/go/bot_configs"
	"google.golang.org/protobuf/types/known/timestamppb"

	"golang.org/x/oauth2/google"

	apipb "go.chromium.org/luci/swarming/proto/api_v2"
)

const (
	DefaultSwarmingServiceAddress = "chrome-swarming.appspot.com:443"
	RunBenchmarkFailure           = "BENCHMARK_FAILURE" // the task completed but benchmark run failed
)

// SwarmingClient
type SwarmingClient interface {
	// CancelTasks tells Swarming to cancel the given tasks.
	CancelTasks(ctx context.Context, taskIDs []string) error

	// GetCASOutput returns the CAS output of a swarming task.
	GetCASOutput(ctx context.Context, taskID string) (*apipb.CASReference, error)

	// GetStartTime returns the starting time of the swarming task.
	GetStartTime(ctx context.Context, taskID string) (*timestamppb.Timestamp, error)

	// GetStatus gets the current status of a swarming task.
	GetStatus(ctx context.Context, taskID string) (string, error)

	// TriggerTask is a wrapper around swarming.ApiClient TriggerTask
	TriggerTask(ctx context.Context, req *apipb.NewTaskRequest) (*apipb.TaskRequestMetadataResponse, error)

	// FetchFreeBots gets a list of available bots per specified builder configuration.
	FetchFreeBots(ctx context.Context, builder string) ([]*apipb.BotInfo, error)

	// GetBotTasksBetweenTwoTasks generates a list of tasks that started in between two tasks.
	// This function is primarily used by Pairwise jobs to assess if another swarming task
	// executed in between a pair of tasks.
	GetBotTasksBetweenTwoTasks(ctx context.Context, botID, taskID1, taskID2 string) (*apipb.TaskListResponse, error)
}

// SwarmingClientImpl
// TODO(jeffyoon@) make this private once run_benchmark doesn't rely on this in testing.
type SwarmingClientImpl struct {
	swarmingv2.SwarmingV2Client
}

func NewSwarmingClient(ctx context.Context, server string) (*SwarmingClientImpl, error) {
	httpClientTokenSource, err := google.DefaultTokenSource(ctx, auth.ScopeReadOnly)
	if err != nil {
		return nil, skerr.Wrapf(err, "Problem setting up default token source")
	}
	c := httputils.DefaultClientConfig().WithTokenSource(httpClientTokenSource).With2xxOnly().Client()
	sc := swarmingv2.NewDefaultClient(c, server)
	return &SwarmingClientImpl{
		SwarmingV2Client: sc,
	}, nil
}

// CancelTasks tells Swarming to cancel the given tasks.
func (s *SwarmingClientImpl) CancelTasks(ctx context.Context, taskIDs []string) error {
	for _, id := range taskIDs {
		_, err := s.CancelTask(ctx, &apipb.TaskCancelRequest{
			TaskId:      id,
			KillRunning: true,
		})
		if err != nil {
			return skerr.Fmt("Could not cancel task %s due to %s", id, err)
		}
	}
	return nil
}

// GetCASOutput returns the CAS output of a swarming task in the form of a RBE CAS hash.
// This function assumes the task is finished, or it throws an error.
func (s *SwarmingClientImpl) GetCASOutput(ctx context.Context, taskID string) (*apipb.CASReference, error) {
	task, err := s.GetResult(ctx, &apipb.TaskIdWithPerfRequest{
		TaskId:                  taskID,
		IncludePerformanceStats: false,
	})
	if err != nil {
		return nil, skerr.Wrapf(err, "could not retrieve CAS of task %s", taskID)
	}
	if task.State != apipb.TaskState_COMPLETED {
		return nil, skerr.Fmt("cannot get result of task %s because it is %s and not COMPLETED", taskID, task.State)
	}
	return task.CasOutputRoot, nil
}

// GetStartTime returns the starting time of the swarming task.
func (s *SwarmingClientImpl) GetStartTime(ctx context.Context, taskID string) (*timestamppb.Timestamp, error) {
	res, err := s.GetResult(ctx, &apipb.TaskIdWithPerfRequest{TaskId: taskID})
	if err != nil {
		return nil, skerr.Wrapf(err, "could not get result of swarming task %s", taskID)
	}
	return res.StartedTs, skerr.Wrap(err)
}

// GetStatus gets the current status of a swarming task.
func (s *SwarmingClientImpl) GetStatus(ctx context.Context, taskID string) (string, error) {
	res, err := s.GetResult(ctx, &apipb.TaskIdWithPerfRequest{
		TaskId:                  taskID,
		IncludePerformanceStats: false,
	})
	if err != nil {
		return "", skerr.Fmt("failed to get swarming task ID %s due to err: %v", taskID, err)
	}
	// Swarming tasks can COMPLETE but fail. In this case, we need to
	// differentiate a successful task from an unsuccessful task.
	if res.State == apipb.TaskState_COMPLETED && res.Failure {
		return RunBenchmarkFailure, nil
	}
	return res.State.String(), nil
}

func (s *SwarmingClientImpl) TriggerTask(ctx context.Context, req *apipb.NewTaskRequest) (*apipb.TaskRequestMetadataResponse, error) {
	return s.SwarmingV2Client.NewTask(ctx, req)
}

// FetchFreeBots gets a list of available bots per specified builder configuration.
func (s *SwarmingClientImpl) FetchFreeBots(ctx context.Context, builder string) ([]*apipb.BotInfo, error) {
	var botConfig bot_configs.BotConfig
	botConfig, err := bot_configs.GetBotConfig(builder, false)
	if err != nil {
		return nil, skerr.Wrapf(err, "error retrieving bots")
	}

	dims := make([]*apipb.StringPair, 0, len(botConfig.Dimensions))

	for _, d := range botConfig.Dimensions {
		dims = append(dims, &apipb.StringPair{
			Key:   d["key"],
			Value: d["value"],
		})
	}

	bots, err := swarmingv2.ListBotsHelper(ctx, s.SwarmingV2Client, &apipb.BotsRequest{
		Dimensions:    dims,
		Quarantined:   apipb.NullableBool_FALSE,
		IsDead:        apipb.NullableBool_FALSE,
		InMaintenance: apipb.NullableBool_FALSE,
	})

	return bots, skerr.Wrap(err)
}

// GetBotTasksBetweenTwoTasks generates a list of tasks that started in between two tasks.
func (s *SwarmingClientImpl) GetBotTasksBetweenTwoTasks(ctx context.Context, botID, taskID1, taskID2 string) (*apipb.TaskListResponse, error) {
	taskStart1, err := s.GetStartTime(ctx, taskID1)
	if err != nil {
		return nil, skerr.Wrap(err)
	}
	taskStart2, err := s.GetStartTime(ctx, taskID2)
	if err != nil {
		return nil, skerr.Wrap(err)
	}

	botReq := &apipb.BotTasksRequest{
		BotId: botID,
		State: apipb.StateQuery_QUERY_ALL,
		Start: taskStart1,
		End:   taskStart2,
		Sort:  apipb.SortQuery_QUERY_STARTED_TS,
	}

	tasks, err := s.SwarmingV2Client.ListBotTasks(ctx, botReq)
	return tasks, skerr.Wrap(err)
}
